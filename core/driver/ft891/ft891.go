// SPDX-License-Identifier: GPL-3.0-or-later

package ft891

import (
	"context"
	"fmt"
	"sync"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// Option configures the driver New builds — and, through it, every Session
// its Open call establishes. See WithTransportLogger.
type Option func(*ft891Driver)

// WithTransportLogger sets the transport.Logger every Session this driver
// Opens threads into its transport.Engine. Without it, the engine's
// diagnostics — unexpected frames, quarantine drains, contamination
// (transport safety obligation 3: "surfaced, never silently discarded") —
// fall into the engine's own drop-everything default with no way for a
// caller of this driver to receive them. A nil l is ignored (the engine's
// default is kept).
func WithTransportLogger(l transport.Logger) Option {
	return func(d *ft891Driver) {
		if l != nil {
			d.transportLogger = l
		}
	}
}

// WithConsentedUnverifiedWrites records that the USER has consented to
// writing this radio's Unverified fields, and builds a driver whose
// SESSIONS carry the consent transform: at session-capability assembly
// every write-side spec.Unverified label becomes spec.ConsentedUnverified,
// which FieldSupport.CanWrite opens (see sessionCapabilities, and
// spec.ConsentUnverifiedWrites for the one definition of what consent
// means).
//
// Consent is a statement about a SESSION, never about the radio: this
// driver's STATIC Capabilities — what internal/wiring's registry publishes,
// what the app describes the model with, what offline synthesis classifies
// against — is untouched by the option, and only the set Open assembles
// carries the state.
//
// It is deliberately not sufficient on its own. An unrecognised Profile
// stays on the untransformed fail-safe even WITH the option
// (profileRecognised); spec.FieldErase is exempt inside the transform
// itself, so no consent can mint an erase — which on this radio is doubly
// idle, since its command list contains no erase command at all — and
// nothing here consults writeTrialsComplete, because consent is a user
// accepting an unverified write, not evidence that the write has been
// proven.
func WithConsentedUnverifiedWrites() Option {
	return func(d *ft891Driver) {
		d.consentUnverifiedWrites = true
	}
}

// New builds the FT-891 driver for profile. RealHardware — the ZERO VALUE —
// selects the all-Unverified capability set while writeTrialsComplete is
// false (nothing writable; see that constant's doc comment), and ANY
// unrecognised Profile value deliberately selects the same fail-safe: the
// failure direction for a forged or corrupted Profile is always "nothing
// writable", never a writable set. Options: WithTransportLogger,
// WithConsentedUnverifiedWrites.
func New(profile Profile, opts ...Option) driver.Driver {
	d := &ft891Driver{profile: profile, dialect: catDialect}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// ft891Driver implements driver.Driver for the Yaesu FT-891.
type ft891Driver struct {
	profile Profile
	// dialect is the CAT dialect every codec call this driver makes — and
	// every Session it Opens makes — goes through. Set from catDialect in
	// New; no Option touches it.
	//
	// It lives HERE rather than only on Session because Open builds the
	// transport.Engine — handing it this very value, from which the engine
	// takes both its outbound gate and its AI init frame — BEFORE any
	// Session exists: a dialect reachable only from Session would be
	// reachable too late to bind the engine at all.
	dialect cat.Dialect
	// transportLogger, when non-nil, is threaded into every Session's
	// transport.Engine at Open time — see WithTransportLogger.
	transportLogger transport.Logger
	// consentUnverifiedWrites records the user's consent to unverified
	// writes — set only by WithConsentedUnverifiedWrites, read only by
	// sessionCapabilities. FALSE is the zero value and the default.
	consentUnverifiedWrites bool
}

// Model implements driver.Driver.
func (d *ft891Driver) Model() string { return modelName }

// Capabilities implements driver.Driver: the static baseline for this
// driver's profile — no discovered banks (see Session.Capabilities for the
// effective, per-radio set).
func (d *ft891Driver) Capabilities() spec.Capabilities {
	switch d.profile {
	case Simulated:
		return CapabilitiesSimulated()
	case RealHardware:
		// No FT-891 has ever been written to over CAT by this project — no
		// FT-891 has ever been ASKED anything at all: writeTrialsComplete
		// is false, so there is no hardware-verified profile for this arm
		// to select and a real-hardware session gets the all-Unverified
		// fail-safe — nothing writable, every write refused before a frame
		// is built UNLESS the user has consented, which is the one thing
		// that reaches past these labels and does so downstream of this
		// method (sessionCapabilities transforms what this arm returns; the
		// set returned HERE is never transformed, and that is what
		// internal/wiring.NeedsUnverifiedConsent reads). See
		// writeTrialsComplete's doc comment for what its flip must change.
		return CapabilitiesUnverified()
	default:
		// Any unrecognised Profile value fails safe, through its own
		// explicit arm rather than by sharing RealHardware's: the two
		// happen to return the same profile today, and a reader must be
		// able to see that the fail-safe is a decision rather than a
		// coincidence of the flip's state. Pinned by
		// TestDriver_ProfileSelection's unrecognised rows.
		return CapabilitiesUnverified()
	}
}

// Open implements driver.Driver: it builds a transport.Engine over port,
// establishes the session (Init: AI0 + drain-to-quiet), probes ID; and
// verifies this really is an FT-891 (a typed *driver.WrongRadioError
// otherwise), then discovers this radio's 5xx/EMG channel inventory and
// returns a Session whose effective capabilities include the discovered
// banks as read-only.
//
// THE ORDER IS AI0; THEN ID; THEN THE ELEVEN PROBES (matrix erratum M-E1,
// spec erratum S-E5). The AI0 preamble is transport.Engine.Init's and is
// shared by every registered Yaesu driver; a WRONG RADIO therefore receives
// exactly two frames — the preamble and the probe — and nothing more. The
// spec's original "before any other frame" was withdrawn in favour of
// "before any DISCOVERY frame"; moving ID ahead of AI0 is a fleet seam and
// a roadmap item, not this driver's to take unilaterally.
//
// Open takes ownership of port on BOTH outcomes: the Session's Close
// releases it on success, and Open itself closes it before returning an
// error.
//
// The engine is bound to THIS driver's own dialect, passed WHOLE (d.dialect
// — see transport.NewEngine): both the outbound allowlist a session
// enforces and the AI init frame it opens with belong to the radio the
// session is for, never a package-level default that would gate every radio
// by whatever one of them permits. On this radio that gate is load-bearing
// in a way it is not on its siblings: the FT-891's dialect refuses to build
// an MT read of a 5xx or EMG slot, and a session gated by a sibling's
// dialect would let one through. transport.NewEngine refuses an
// unconfigured dialect outright, so there is no ungated path through here
// even if the field were somehow left zero.
func (d *ft891Driver) Open(ctx context.Context, port transport.Port, id driver.Identity) (driver.Session, error) {
	eng, err := yaesu.NewEngine(port, d.dialect, d.transportLogger, &params)
	if err != nil {
		// NewEngine has closed the port itself on this path — Open's
		// ownership obligation, discharged there in one place.
		return nil, err
	}

	sess, err := d.open(ctx, eng, id)
	if err != nil {
		_ = eng.Close()
		return nil, err
	}
	return sess, nil
}

// open is Open's body, factored so the error path can close eng in exactly
// one place.
func (d *ft891Driver) open(ctx context.Context, eng *transport.Engine, id driver.Identity) (*Session, error) {
	got, err := yaesu.Handshake(ctx, eng, d.dialect, &params, yaesu.Want{CATID: catID, Model: modelName})
	if err != nil {
		return nil, err
	}
	id.CATID = got

	slots60m, emg, err := yaesu.DiscoverInventory(ctx, eng, d.dialect, &params)
	if err != nil {
		return nil, fmt.Errorf("ft891: Open: 5xx/EMG discovery: %w", err)
	}

	return &Session{
		eng:     eng,
		dialect: d.dialect,
		id:      id,
		caps:    d.sessionCapabilities(slots60m, emg),
	}, nil
}

// sessionCapabilities is the ONE place a session's effective capability set
// is assembled: effectiveCapabilities' product, then — only when this driver
// was built with WithConsentedUnverifiedWrites AND its profile is one of the
// declared constants — the consent transform. An unrecognised profile stays
// untransformed even with the option: the fail-safe direction ("no value a
// caller can pass produces a writable session") survives consent. Applying
// the transform here, before the Session exists, keeps the set WriteChannel
// enforces (s.caps) and the set Capabilities() hands out the same value.
func (d *ft891Driver) sessionCapabilities(slots60m []string, emg bool) spec.Capabilities {
	caps := effectiveCapabilities(d.Capabilities(), slots60m, emg)
	if d.consentUnverifiedWrites && d.profileRecognised() {
		caps = spec.ConsentUnverifiedWrites(caps)
	}
	return caps
}

// profileRecognised reports whether this driver's profile is one of the
// package's declared Profile constants — the same set the capability switch
// names explicitly, restated here so the consent gate cannot drift open for
// a profile the switch would fail safe on.
func (d *ft891Driver) profileRecognised() bool {
	switch d.profile {
	case Simulated, RealHardware:
		return true
	}
	return false
}

// SynthesiseDiscoveredBanks implements the optional
// driver.DiscoveredBankSynthesizer capability (core/driver/optional.go): it
// classifies an OFFLINE slot list — a working codeplug's own slots, with no
// live session anywhere — into the read-only 60M/EMG banks a live session's
// Open would have discovered for a radio whose inventory contained them.
//
// Implementing it is not optional in practice, whatever the interface says:
// its ABSENCE fails silently. internal/wiring's synthesis helper returns
// false for a driver that does not satisfy the interface, and the app then
// renders no discovered banks at all for an offline FT-891 codeplug — no
// error, no log, just missing rows.
//
// It deliberately reuses effectiveCapabilities itself rather than restating
// bank shape (label, NoBlank, Fields): d.Capabilities() is the exact same
// "base" argument Open passes it, so slotting the classified slots into the
// identical call makes drift between offline synthesis and live discovery
// structurally impossible rather than merely tested for. Slicing off the
// base banks — base.Banks' own length, before any discovered bank is
// appended — yields exactly the newly discovered ones, in
// effectiveCapabilities' fixed 60M-then-EMG order.
//
// It calls effectiveCapabilities RAW rather than going through
// sessionCapabilities, and consent is why that changes nothing: every
// discovered bank's Write support is forced to spec.Unsupported by
// readOnlyFields (caps.go), so there is no Unverified write label in these
// banks for the consent transform to convert — and an offline codeplug's
// banks are not a session anyway.
//
// Classification: a slot already claimed by one of base's static banks
// (MEM/PMS) is excluded first, because this method only ever adds banks
// discovery would ADD and never restates a static one. Every remaining slot
// is parsed by THIS DRIVER'S dialect (the same authoritative parser
// discovery's own slot builders agree with) and classified by
// Slot.Is60m/IsEMG, preserving input ORDER; a slot that parses to neither is
// unclassifiable and OMITTED, never guessed into a bank. On this radio that
// omission bites one wire form its siblings would accept — "511" and up,
// which the FT-710's and FTdx10's 501..599 admit and this manual's printed
// 501-510 does not.
//
// A REPEATED "EMG" collapses to the single physical channel. Live discovery
// probes one EMG slot and can never produce a duplicate, so a duplicate can
// only come from a semantically invalid input list; reporting one EMG row
// for it keeps this method's output identical to what a live session would
// carry. core/driver/ft710 makes the opposite choice — it preserves every
// occurrence — for a compatibility reason of its own, and that is its
// history, not a rule.
// TestSynthesiseDiscoveredBanks_DuplicateEMGCollapses pins this driver's
// choice so it stays a decision.
func (d *ft891Driver) SynthesiseDiscoveredBanks(slots []string) []spec.Bank {
	base := d.Capabilities()
	numBaseBanks := len(base.Banks)

	claimed := make(map[string]bool)
	for _, b := range base.Banks {
		for _, s := range b.Slots {
			claimed[s] = true
		}
	}

	var slots60m []string
	var emg bool
	for _, raw := range slots {
		if claimed[raw] {
			continue
		}
		s, err := d.dialect.ParseSlot(raw)
		if err != nil {
			continue
		}
		switch {
		case s.Is60m():
			slots60m = append(slots60m, raw)
		case s.IsEMG():
			emg = true
		}
	}

	disc := effectiveCapabilities(base, slots60m, emg)
	return disc.Banks[numBaseBanks:]
}

// Session is the FT-891's driver.Session: one open, identity-verified,
// inventory-discovered connection. Safe for concurrent use.
//
// IT CARRIES AN OPERATION MUTEX, and unlike its combined-form siblings it
// needs one. transport.Engine serialises each individual EXCHANGE, not a
// pair, and a memory or PMS ReadChannel on this radio is potentially TWO
// exchanges — the combined MT read, then the cross-check's MR when MT
// answers "?;" (read.go, matrix §3.5). Without opMu a concurrent operation
// could land in that gap, and the cross-check would be interpreting an MT
// rejection against an MR answer describing a different radio state.
//
// The lock guards a SINGLE DRIVER OPERATION (spec erratum S-E4, matrix
// M-E2): the whole cross-check inside ReadChannel, and the single exchange
// inside WriteChannel (write.go). IT IS NOT HELD ACROSS
// WRITE-THEN-VERIFY: that pair belongs to core/clone, as the driver
// interface assigns it, and holding a driver lock across it would serialise
// two operations the seam deliberately keeps separate.
//
// The FTdx10's Session has no opMu and says so at length; that is that
// driver's consequence of its own one-exchange choreography and NOT a shape
// to copy here.
type Session struct {
	eng *transport.Engine
	// opMu serialises whole driver operations — see the type's doc comment.
	opMu sync.Mutex
	// dialect is the CAT dialect this session's every codec call goes
	// through — builders, parsers, the answer geometry, and the mode
	// rendering ReadChannel puts in front of the user. Copied from the
	// ft891Driver that Opened it (which is also where the engine's gate came
	// from), so a session can never encode with one radio's dialect while
	// its transport gates with another's.
	dialect cat.Dialect
	id      driver.Identity
	caps    spec.Capabilities // effective; never mutated after Open
}

// Identity implements driver.Session.
func (s *Session) Identity() driver.Identity { return s.id }

// Capabilities implements driver.Session: the EFFECTIVE capability set
// (profile baseline plus the discovered read-only 60M/EMG banks), as a deep
// copy per call — see cloneCapabilities for why the copy is load-bearing.
func (s *Session) Capabilities() spec.Capabilities {
	return cloneCapabilities(s.caps)
}

// Diagnostics reports this session's transport-level health counters as a
// point-in-time snapshot — the driver-layer surface for the engine's own
// accessors, which are otherwise unreachable (the *transport.Engine is
// unexported inside this Session). Safe for concurrent use, like the
// accessor it wraps. It satisfies the optional driver.DiagnosticsReporter
// capability; like its siblings', it is a method on the concrete *Session
// rather than part of driver.Session, because which diagnostics exist is a
// per-driver matter.
func (s *Session) Diagnostics() driver.SessionDiagnostics {
	n := s.eng.UnexpectedFrames()
	if n < 0 {
		// Unreachable (the engine only ever increments), but never let a
		// negative int64 wrap into an absurd uint64.
		n = 0
	}
	return driver.SessionDiagnostics{UnexpectedFrames: uint64(n)}
}

// Close implements driver.Session. Idempotent: transport.Engine.Close
// already guarantees repeat calls return the same result.
func (s *Session) Close() error { return s.eng.Close() }

// WriteChannel implements driver.Session and lives in write.go, beside the
// refusal ladder and the one combined MT Set it builds.

// ErrAnswerMismatch is the sentinel a caller compares against (via
// errors.Is) to ask "did a radio answer about the wrong channel?" —
// the shared one, so the question can be put once rather than once
// per driver package. The error actually returned is an
// *AnswerMismatchError naming both addresses.
var ErrAnswerMismatch = driver.ErrAnswerMismatch

// AnswerMismatchError reports that a memory answer's decoded slot was
// not the one asked for, naming both. THE CHECK IS THIS DRIVER'S
// BECAUSE NOTHING BELOW IT MAKES ONE: a NEWCAT prefix matcher checks
// the command name, so an answer for another slot satisfies the read's
// spec perfectly well, and a record mis-attributed to the wrong slot is
// the corruption this project refuses.
type AnswerMismatchError = driver.AnswerMismatchError[string]
