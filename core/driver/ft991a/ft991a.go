// SPDX-License-Identifier: GPL-3.0-or-later

package ft991a

import (
	"context"
	"sync"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// Option configures the driver New builds — and, through it, every Session
// its Open call establishes. See WithTransportLogger.
type Option func(*ft991aDriver)

// WithTransportLogger sets the transport.Logger every Session this driver
// Opens threads into its transport.Engine. Without it, the engine's
// diagnostics — unexpected frames, quarantine drains, contamination
// (transport safety obligation 3: "surfaced, never silently discarded") —
// fall into the engine's own drop-everything default with no way for a
// caller of this driver to receive them. A nil l is ignored (the engine's
// default is kept).
func WithTransportLogger(l transport.Logger) Option {
	return func(d *ft991aDriver) {
		if l != nil {
			d.transportLogger = l
		}
	}
}

// WithConsentedUnverifiedWrites records that the USER has consented to
// writing this radio's Unverified fields, and builds a driver whose SESSIONS
// carry the consent transform: at session-capability assembly every
// write-side spec.Unverified label becomes spec.ConsentedUnverified, which
// FieldSupport.CanWrite opens (see sessionCapabilities, and
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
// idle, since its Control Command List contains no erase command at all —
// and nothing here consults writeTrialsComplete, because consent is a user
// accepting an unverified write, not evidence that the write has been
// proven.
//
// ON THIS RADIO CONSENT REACHES A VOCABULARY NO SIBLING HAS. The five-state
// P8 (caps.go's ctcssStates) is graded writable, so a consented session may
// put '3' or '4' — a DCS state — on the wire. That is the matrix's own
// [STUART] cell, ruled writable because the legend prints both values
// against the MT and MW SET charts as well as the read-side blocks; what the
// radio does with the DCS CODE it already holds for such a channel is
// doc.go's register entry A DCS-STATE CHANNEL'S CODE SURVIVES A REWRITE.
func WithConsentedUnverifiedWrites() Option {
	return func(d *ft991aDriver) {
		d.Consented = true
	}
}

// New builds the FT-991A driver for profile. RealHardware — the ZERO VALUE —
// selects the all-Unverified capability set while writeTrialsComplete is
// false (nothing writable; see that constant's doc comment), and ANY
// unrecognised Profile value deliberately selects the same fail-safe: the
// failure direction for a forged or corrupted Profile is always "nothing
// writable", never a writable set. Options: WithTransportLogger,
// WithConsentedUnverifiedWrites.
func New(profile Profile, opts ...Option) driver.Driver {
	d := &ft991aDriver{Base: driver.Base{Profile: profile}, dialect: catDialect}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// ft991aDriver implements driver.Driver for the Yaesu FT-991A.
type ft991aDriver struct {
	driver.Base
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
}

// Model implements driver.Driver.
func (d *ft991aDriver) Model() string { return modelName }

// Capabilities implements driver.Driver: the static baseline for this
// driver's profile.
//
// ON THIS RADIO IT IS ALSO THE WHOLE STORY, unlike every sibling that has a
// discovered bank: Open probes nothing (matrix §3.4), so a Session's
// EFFECTIVE capabilities are this same set, plus the consent transform when
// the user has given it. Session.Capabilities is therefore a copy of this
// rather than of something larger.
func (d *ft991aDriver) Capabilities() spec.Capabilities {
	switch d.Profile {
	case Simulated:
		return CapabilitiesSimulated()
	case RealHardware:
		// No FT-991A has ever been written to over CAT by this project —
		// no FT-991A has ever been ASKED anything at all:
		// writeTrialsComplete is false, so there is no hardware-verified
		// profile for this arm to select and a real-hardware session gets
		// the all-Unverified fail-safe — nothing writable, every write
		// refused before a frame is built UNLESS the user has consented,
		// which is the one thing that reaches past these labels and does so
		// downstream of this method (sessionCapabilities transforms what
		// this arm returns; the set returned HERE is never transformed, and
		// that is what internal/wiring.NeedsUnverifiedConsent reads). See
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
// verifies this really is an FT-991A (a typed *driver.WrongRadioError
// otherwise), and returns the Session.
//
// AI0; THEN ID; AND NOTHING ELSE. There is no discovery phase at all
// (matrix §3.4, plan P10 and task 10): the transcript is TWO FRAMES on a
// match and two on a mismatch, port closed. Nothing is probed because there
// is nothing to probe — "5xx", "5 MHz", "5MHz" and "EMG" appear in no slot
// legend of this manual, checked mechanically over the whole extraction, and
// both banks this radio does have are dense and complete at construction.
//
// THE PIN FOR THAT IS NEGATIVE, AND IT HAS TO BE. The FT-891 spends up to
// eleven MR exchanges here and can assert that they happened; an absence
// cannot be caught by watching the right thing happen, and a regression that
// added a walk would simply work, slowly, against any peer that answered. So
// TestOpen_SendsExactlyTwoFrames asserts the WHOLE transcript and
// TestOpen_NeverBuildsAnMROfANonExistentBank asserts it over a whole session.
//
// The AI0 preamble is transport.Engine.Init's and is shared by every
// registered Yaesu driver; a WRONG RADIO therefore receives exactly two
// frames — the preamble and the probe — and nothing more. Moving ID ahead of
// AI0 is a fleet seam and a roadmap item, not this driver's to take
// unilaterally.
//
// Open takes ownership of port on BOTH outcomes: the Session's Close
// releases it on success, and Open itself closes it before returning an
// error.
//
// The engine is bound to THIS driver's own dialect, passed WHOLE (d.dialect
// — see transport.NewEngine): both the outbound allowlist a session enforces
// and the AI init frame it opens with belong to the radio the session is
// for, never a package-level default that would gate every radio by whatever
// one of them permits. On this radio the gate carries a refusal no sibling's
// does — a record naming a DCS state is buildable here and refused by every
// ToneStatesCTCSS dialect — so a session gated by a sibling's dialect would
// refuse frames this one must send. transport.NewEngine refuses an
// unconfigured dialect outright, so there is no ungated path through here
// even if the field were somehow left zero.
func (d *ft991aDriver) Open(ctx context.Context, port transport.Port, id driver.Identity) (driver.Session, error) {
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
func (d *ft991aDriver) open(ctx context.Context, eng *transport.Engine, id driver.Identity) (*Session, error) {
	got, err := yaesu.Handshake(ctx, eng, d.dialect, &params, yaesu.Want{CATID: catID, Model: modelName})
	if err != nil {
		return nil, err
	}
	id.CATID = got

	return &Session{
		eng:     eng,
		dialect: d.dialect,
		id:      id,
		caps:    d.sessionCapabilities(),
	}, nil
}

// sessionCapabilities is the ONE place a session's effective capability set
// is assembled: this driver's static baseline, then — only when it was built
// with WithConsentedUnverifiedWrites AND its profile is one of the declared
// constants — the consent transform. An unrecognised profile stays
// untransformed even with the option: the fail-safe direction ("no value a
// caller can pass produces a writable session") survives consent. Applying
// the transform here, before the Session exists, keeps the set WriteChannel
// enforces (s.caps) and the set Capabilities() hands out the same value.
//
// IT TAKES NO DISCOVERED INVENTORY, where every sibling's namesake does,
// because Open discovers nothing (matrix §3.4). There is deliberately no
// effectiveCapabilities function to pass one to: a seam that existed only to
// append banks this radio cannot have would be a place for a later reader to
// add one.
func (d *ft991aDriver) sessionCapabilities() spec.Capabilities {
	return d.SessionCaps(d.Capabilities())
}

// profileRecognised reports whether this driver's profile is one of the
// declared constants — driver.Base's shared predicate, kept under the
// name this package's tests put the question by.
func (d *ft991aDriver) profileRecognised() bool { return d.Recognised() }

// Session is the FT-991A's driver.Session: one open, identity-verified
// connection. Safe for concurrent use.
//
// IT CARRIES AN OPERATION MUTEX, and the reason is not the FT-891's.
// transport.Engine serialises each individual EXCHANGE, and this radio's
// ReadChannel is ONE exchange — the combined MT read, with no cross-check to
// straddle (read.go, matrix §3.5) — so a read needs no lock against another
// read's second frame, because there is no second frame.
//
// What opMu guards is a whole DRIVER OPERATION (spec erratum S-E4, matrix
// M-E2), and this session has more than one kind of those: a read, a write
// (write.go) and a settings read (settings.go) must not interleave their
// frames even though the engine would happily serialise them one exchange at
// a time. The concurrency pin plan P12 asks for is that two racing
// ReadChannels cannot interleave two MT frames; the pin of the LOCK ITSELF is
// settings.go's readSettingGapHook, which parks a settings read inside opMu
// so a concurrent WriteChannel can be shown to be excluded by the lock and
// not merely by the engine.
//
// IT IS NOT HELD ACROSS WRITE-THEN-VERIFY: that pair belongs to core/clone,
// as the driver interface assigns it, and holding a driver lock across it
// would serialise two operations the seam deliberately keeps separate.
type Session struct {
	eng *transport.Engine
	// opMu serialises whole driver operations — see the type's doc comment.
	opMu sync.Mutex
	// dialect is the CAT dialect this session's every codec call goes
	// through — builders, parsers, the answer geometry, and the mode
	// rendering ReadChannel puts in front of the user. Copied from the
	// ft991aDriver that Opened it (which is also where the engine's gate
	// came from), so a session can never encode with one radio's dialect
	// while its transport gates with another's.
	dialect cat.Dialect
	id      driver.Identity
	caps    spec.Capabilities // effective; never mutated after Open
}

// Identity implements driver.Session.
func (s *Session) Identity() driver.Identity { return s.id }

// Capabilities implements driver.Session: the EFFECTIVE capability set — on
// this radio the profile baseline itself, plus consent when the user gave it,
// because nothing is ever discovered — as a deep copy per call (see
// spec.Capabilities.Clone for why the copy is load-bearing).
func (s *Session) Capabilities() spec.Capabilities {
	return s.caps.Clone()
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
	return driver.SessionDiagnostics{UnexpectedFrames: uint64(s.eng.UnexpectedFrames())}
}

// Close implements driver.Session. Idempotent: transport.Engine.Close
// already guarantees repeat calls return the same result.
func (s *Session) Close() error { return s.eng.Close() }

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
