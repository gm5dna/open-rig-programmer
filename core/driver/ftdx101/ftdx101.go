// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx101

import (
	"context"
	"fmt"
	"sync"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	// ALIASED deliberately: the dialect package's own name is also
	// "ftdx101", and an unaliased import would put a second meaning on the
	// spelling this package already answers to. catftdx101 reads as "the
	// core/cat side of the FTdx101", which is exactly what it is, and it
	// appears at TWO call sites (modelD and modelMP, below) — one per
	// model, because there are two models and therefore two dialects.
	catftdx101 "github.com/gm5dna/open-rig-programmer/core/cat/ftdx101"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// modelParams is everything that differs between the two radios this
// package drives: a display name and a CAT dialect. Matrix §4 is what makes
// a two-field struct sufficient — Yaesu prints ONE CAT manual for the
// FTDX101MP and the FTDX101D and distinguishes them in exactly three
// places, of which only the ID answer touches a capability value, so the
// sole model-conditional capability values are Model and the CAT ID (and
// the CAT ID lives inside the dialect rather than beside it).
//
// It is UNEXPORTED, and so is every function taking one, because plan D1
// fixes the exported surface as two thin constructors: an exported model
// enum's zero value would need its own fail-safe arm, and a
// registration-table closure that held a model value could hold a forged
// one. NewD and NewMP name the models instead, exactly as the dialect
// package exposes DialectD/DialectMP over one newDialect and refuses to
// offer a bare Dialect() ("there are two models, so there are two of
// them", core/cat/ftdx101/exinventory.go:19-21).
type modelParams struct {
	// name is the model's display name and driver-registry key — the
	// project's SPELLING, fixed by the M9d spec, of a manual fact (matrix
	// §1.1). It is what Model() and Capabilities().Model return.
	name string
	// dialect is that model's CAT dialect: the ONE place this package names
	// an instance from core/cat for that radio. Everything else derives
	// from it — the driver's and every session's codec, the engine's
	// outbound gate and AI init frame, the capability data's CAT ID, modes,
	// slot inventories and MT answer geometry.
	dialect cat.Dialect
}

// modelD is the FTDX101D: CAT ID 0681 (matrix §1.2, MANUAL-EVIDENCED — ID's
// P1 legend at layout 1070).
var modelD = modelParams{name: "FTdx101D", dialect: catftdx101.DialectD()}

// modelMP is the FTDX101MP: CAT ID 0682 (matrix §1.2, MANUAL-EVIDENCED —
// ID's P1 legend at layout 1072). It differs from modelD in the name and
// the CAT ID and in nothing else; core/cat/ftdx101 builds both dialects
// over ONE config for exactly that reason.
var modelMP = modelParams{name: "FTdx101MP", dialect: catftdx101.DialectMP()}

// siblingNames maps every CAT ID this PACKAGE registers to its display
// name, so a probe refusal can spell out both models involved (spec A5).
// It deliberately knows nothing beyond this package's own two radios: a
// foreign ID (an FT-710's "0800", say) resolves to "", and the error's
// ID-only text renders — naming other packages' radios is their business.
var siblingNames = map[string]string{
	modelD.dialect.CATID():  modelD.name,
	modelMP.dialect.CATID(): modelMP.name,
}

// Option configures the driver NewD or NewMP builds — and, through it,
// every Session its Open call establishes. See WithTransportLogger and
// WithConsentedUnverifiedWrites.
type Option func(*ftdx101Driver)

// WithTransportLogger sets the transport.Logger every Session this driver
// Opens threads into its transport.Engine. Without it, the engine's
// diagnostics — unexpected frames, quarantine drains, contamination
// (transport safety obligation 3: "surfaced, never silently discarded") —
// fall into the engine's own drop-everything default with no way for a
// caller of this driver to receive them. A nil l is ignored (the engine's
// default is kept).
func WithTransportLogger(l transport.Logger) Option {
	return func(d *ftdx101Driver) {
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
// PER DRIVER, therefore per MODEL: an option passed to NewD reaches the D's
// sessions and no MP's, which is the right granularity for a consent the
// user gives about a radio in front of them. This package's two radios have
// separate write guards (writeTrialsCompleteD, writeTrialsCompleteMP) for
// the same reason.
//
// It is deliberately not sufficient on its own. An unrecognised Profile
// stays on the untransformed fail-safe even WITH the option
// (profileRecognised); spec.FieldErase is exempt inside the transform
// itself, so no consent can mint an erase; and nothing here consults
// writeTrialsCompleteD or writeTrialsCompleteMP, because consent is a user
// accepting an unverified write, not evidence that the write has been
// proven.
func WithConsentedUnverifiedWrites() Option {
	return func(d *ftdx101Driver) {
		d.Consented = true
	}
}

// NewD builds the FTDX101D driver for profile. RealHardware — the ZERO
// VALUE — selects the all-Unverified capability set while
// writeTrialsCompleteD is false (nothing writable; see that constant's doc
// comment), and ANY unrecognised Profile value deliberately selects the
// same fail-safe: the failure direction for a forged or corrupted Profile
// is always "nothing writable", never a writable set. Options:
// WithTransportLogger, WithConsentedUnverifiedWrites.
func NewD(profile Profile, opts ...Option) driver.Driver {
	return newDriver(modelD, profile, opts...)
}

// NewMP builds the FTDX101MP driver for profile. Same reasoning as NewD in
// every respect, including the fail-safe profile arms; the MP's own write
// guard is writeTrialsCompleteMP, and it is false for the MP's own reasons.
func NewMP(profile Profile, opts ...Option) driver.Driver {
	return newDriver(modelMP, profile, opts...)
}

// newDriver is the ONE implementation the two exported constructors wrap
// (plan D1). The model is a parameter rather than a package-level default
// because this package drives two radios and neither is the other's
// fallback: there is no bare New, for the same reason core/cat/ftdx101
// offers no bare Dialect().
func newDriver(m modelParams, profile Profile, opts ...Option) driver.Driver {
	d := &ftdx101Driver{model: m, Base: driver.Base{Profile: profile}}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// ftdx101Driver implements driver.Driver for the Yaesu FTDX101D and
// FTDX101MP. ONE type serves both: the models differ in a name and a CAT ID
// (matrix §4), so a second type would be a copy differing in two literals.
type ftdx101Driver struct {
	// model is which of this package's two radios this driver is for, and
	// it carries the dialect every codec call goes through — see
	// modelParams. Set by newDriver; no Option touches it.
	//
	// The DIALECT lives here rather than only on Session because Open
	// builds the transport.Engine — handing it this very value, from which
	// the engine takes both its outbound gate and its AI init frame —
	// BEFORE any Session exists: a dialect reachable only from Session
	// would be reachable too late to bind the engine at all. No method
	// reaches for a package-level dialect: every codec call goes through
	// the value the driver or session carries, so a hand-built driver with
	// a zero modelParams fails closed rather than silently borrowing one
	// model's dialect (see TestOpen_UnconfiguredDialectRefusesToOpen).
	model modelParams
	driver.Base
	// transportLogger, when non-nil, is threaded into every Session's
	// transport.Engine at Open time — see WithTransportLogger.
	transportLogger transport.Logger
	// consentUnverifiedWrites records the user's consent to unverified
	// writes — set only by WithConsentedUnverifiedWrites, read only by
	// sessionCapabilities. FALSE is the zero value and the default, so a
	// driver built without the option behaves exactly as it did before the
	// option existed.
}

// Model implements driver.Driver.
func (d *ftdx101Driver) Model() string { return d.model.name }

// Capabilities implements driver.Driver: the static baseline for this
// driver's model and profile — no discovered banks (see
// Session.Capabilities for the effective, per-radio set).
func (d *ftdx101Driver) Capabilities() spec.Capabilities {
	switch d.Profile {
	case Simulated:
		return capabilitiesSimulated(d.model)
	case RealHardware:
		// No FTdx101 of EITHER model has ever been written to over CAT by
		// this project: writeTrialsCompleteD and writeTrialsCompleteMP are
		// both false, so there is no hardware-verified profile for this arm
		// to select and a real-hardware session gets the all-Unverified
		// fail-safe — nothing writable, every write refused before a frame
		// is built UNLESS the user has consented, which is the one thing
		// that reaches past these labels and does so downstream of this
		// method (sessionCapabilities transforms what this arm returns; the
		// set returned HERE is never transformed, and that is what
		// internal/wiring.NeedsUnverifiedConsent reads).
		// Neither constant is READ here, deliberately: see their
		// doc comments for what a flip must change, and why a constant that
		// was load-bearing on its own would let a one-character edit unlock
		// a write.
		return capabilitiesUnverified(d.model)
	default:
		// Any unrecognised Profile value fails safe, through its own
		// explicit arm rather than by sharing RealHardware's: the two
		// happen to return the same profile today, and a reader must be
		// able to see that the fail-safe is a decision rather than a
		// coincidence of the flips' state. Pinned by
		// TestProfileMatrix_StaticPerField's invalid-profile rows.
		return capabilitiesUnverified(d.model)
	}
}

// Open implements driver.Driver: it builds a transport.Engine over port,
// establishes the session (Init: AI0 + drain-to-quiet), probes ID; and
// verifies this really is the model this driver is for (a typed
// *driver.WrongRadioError otherwise), then discovers this radio's 5xx/EMG
// channel inventory and returns a Session whose effective capabilities
// include the discovered banks as read-only.
//
// Open takes ownership of port on BOTH outcomes: the Session's Close
// releases it on success, and Open itself closes it before returning an
// error.
//
// The engine is bound to THIS driver's own model's dialect, passed WHOLE
// (see transport.NewEngine): both the outbound allowlist a session enforces
// and the AI init frame it opens with belong to the radio the session is
// for, never a package-level default that would gate every radio by
// whatever one of them permits — which for a package holding TWO radios is
// not a hypothetical. transport.NewEngine refuses an unconfigured dialect
// outright, so there is no ungated path through here even if the field were
// somehow left zero.
//
// One note the AI0 init inherits (matrix §3.12, MANUAL-EVIDENCED): the AI
// command "is available only when PC is connected with USB cable" (layout
// 381). Over the rear RS-232C jack it is documented unavailable, and
// because the AI0 Set is sent fire-and-forget — ASSUMED, doc.go's register
// entry 9, second half, and NOT read off the availability row (that row's
// O O O X at layout 244 says AI has Set, Read and Answer FORMS, not what a
// Set draws) — a session opened on that path would not obviously fail: it
// would simply not have disarmed auto-information. This project opens
// whatever port it is given and this manual gives no way to detect which
// one, so there is no code action here; it is why every Stage R capture
// must record its port.
func (d *ftdx101Driver) Open(ctx context.Context, port transport.Port, id driver.Identity) (driver.Session, error) {
	eng, err := yaesu.NewEngine(port, d.model.dialect, d.transportLogger, &params)
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
func (d *ftdx101Driver) open(ctx context.Context, eng *transport.Engine, id driver.Identity) (*Session, error) {
	m := d.model
	got, err := yaesu.Handshake(ctx, eng, m.dialect, &params, yaesu.Want{CATID: m.dialect.CATID(), Model: m.name, Sibling: func(id string) string { return siblingNames[id] }})
	if err != nil {
		return nil, err
	}
	id.CATID = got

	slots60m, emg, err := yaesu.DiscoverInventory(ctx, eng, m.dialect, &params)
	if err != nil {
		return nil, fmt.Errorf("ftdx101: Open: 5xx/EMG discovery: %w", err)
	}

	return &Session{
		eng:     eng,
		dialect: m.dialect,
		id:      id,
		caps:    d.sessionCapabilities(slots60m, emg),
	}, nil
}

// sessionCapabilities is the ONE place a session's effective capability set
// is assembled: effectiveCapabilities' product — built with THIS DRIVER'S
// OWN MODEL'S dialect, which is what puts the right radio's EMG wire form in
// the discovered bank — then, only when this driver was built with
// WithConsentedUnverifiedWrites AND its profile is one of the declared
// constants, the consent transform. An unrecognised profile stays
// untransformed even with the option: the fail-safe direction ("no value a
// caller can pass produces a writable session") survives consent. Applying
// the transform here, before the Session exists, keeps the set WriteChannel
// enforces (s.caps) and the set Capabilities() hands out the same value.
func (d *ftdx101Driver) sessionCapabilities(slots60m []string, emg bool) spec.Capabilities {
	return d.SessionCaps(effectiveCapabilities(d.model.dialect, d.Capabilities(), slots60m, emg))
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
// renders no discovered banks at all for an offline FTdx101 codeplug — no
// error, no log, just missing rows. The compile-time assertion in
// ftdx101_test.go is what turns a renamed method or a changed receiver into
// a build failure instead.
//
// It deliberately reuses effectiveCapabilities itself rather than restating
// bank shape (label, NoBlank, Fields): d.Capabilities() is the exact same
// "base" argument Open passes it, and d.model.dialect the same dialect, so
// slotting the classified slots into the identical call makes drift between
// offline synthesis and live discovery structurally impossible rather than
// merely tested for. Slicing off the base banks — base.Banks' own length,
// before any discovered bank is appended — yields exactly the newly
// discovered ones, in effectiveCapabilities' fixed 60M-then-EMG order.
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
// is parsed by THIS DRIVER'S MODEL'S dialect (the same authoritative parser
// discovery's own slot builders agree with) and classified by
// Slot.Is60m/IsEMG, preserving input ORDER; a slot that parses to neither
// is unclassifiable and OMITTED, never guessed into a bank.
//
// A REPEATED "EMG" collapses to the single physical channel. Live discovery
// probes one EMG slot and can never produce a duplicate, so a duplicate can
// only come from a semantically invalid input list (LoadFile validates only
// AFTER loading); reporting one EMG row for it keeps this method's output
// identical to what a live session would carry. core/driver/ft710 makes the
// opposite choice — it preserves every occurrence — for a compatibility
// reason of its own, and that is its history, not a rule.
// TestSynthesiseDiscoveredBanks_DuplicateEMGCollapses pins this driver's
// choice so it stays a decision.
func (d *ftdx101Driver) SynthesiseDiscoveredBanks(slots []string) []spec.Bank {
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
		s, err := d.model.dialect.ParseSlot(raw)
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

	disc := effectiveCapabilities(d.model.dialect, base, slots60m, emg)
	return disc.Banks[numBaseBanks:]
}

// Session is an FTdx101's driver.Session: one open, identity-verified,
// inventory-discovered connection, for whichever of the two models Opened
// it. Safe for concurrent use — transport.Engine serialises every
// individual exchange, and everything else here is immutable after Open.
//
// ONE OPERATION MUTEX PER SESSION, taken by ReadSetting and by
// WriteChannel and never re-entered: a whole driver operation excludes
// another, which is a larger claim than the engine's own per-exchange
// lock because it covers the work an operation does around its
// exchanges. Every one of these radios states the rule the same way, so
// there is one rule rather than a per-radio exception that a second
// frame added to any operation would silently invalidate.
//
// This radio's operations happen to be ONE wire exchange each today
// (ReadChannel's combined MT read; WriteChannel's combined MT Set,
// write.go), so the lock costs nothing here; holding it anyway is what
// makes the FT-710's two-exchange choreography (MR+MT, MW+MT) a
// difference in the radio rather than a difference in the rule.
//
// THE SHARED BODIES TAKE NO LOCK. core/driver/internal/yaesu's
// WriteChannel and ReadSetting document that their caller holds it, so
// the mutex is taken in exactly one place per method and cannot be
// re-entered from inside.
type Session struct {
	eng *transport.Engine
	// dialect is the CAT dialect this session's every codec call goes
	// through — builders, parsers, the answer geometry, and the mode
	// rendering ReadChannel puts in front of the user. Copied from the
	// ftdx101Driver that Opened it (which is also where the engine's gate
	// came from), so a session can never encode with one radio's dialect
	// while its transport gates with another's. In THIS package that is not
	// an abstract hazard: two dialects live here, and a session holding the
	// D's while its engine gated with the MP's would differ from a correct
	// one only in what the ID probe had accepted.
	dialect cat.Dialect
	id      driver.Identity
	caps    spec.Capabilities // effective; never mutated after Open

	// opMu serialises whole DRIVER OPERATIONS on this session, which is a
	// larger claim than the engine's own per-exchange mutex: it covers the
	// work an operation does around its exchanges, not just the exchange.
	// Every operation takes it, single-frame ones included, so the rule is
	// one rule rather than a list of exceptions that a second frame added
	// to any of them would silently invalidate.
	opMu sync.Mutex
}

// Identity implements driver.Session.
func (s *Session) Identity() driver.Identity { return s.id }

// Capabilities implements driver.Session: the EFFECTIVE capability set
// (profile baseline plus the discovered read-only 60M/EMG banks), as a deep
// copy per call — see spec.Capabilities.Clone for why the copy is load-bearing.
func (s *Session) Capabilities() spec.Capabilities {
	return s.caps.Clone()
}

// Diagnostics reports this session's transport-level health counters as a
// point-in-time snapshot — the driver-layer surface for the engine's own
// accessors, which are otherwise unreachable (the *transport.Engine is
// unexported inside this Session). Safe for concurrent use, like the
// accessor it wraps. It satisfies the optional driver.DiagnosticsReporter
// capability; like the FT-710's and the FTdx10's, it is a method on the
// concrete *Session rather than part of driver.Session, because which
// diagnostics exist is a per-driver matter.
func (s *Session) Diagnostics() driver.SessionDiagnostics {
	return driver.SessionDiagnostics{UnexpectedFrames: uint64(s.eng.UnexpectedFrames())}
}

// Close implements driver.Session. Idempotent: transport.Engine.Close
// already guarantees repeat calls return the same result.
func (s *Session) Close() error { return s.eng.Close() }

// WriteChannel — the MT-only combined Set — lives in write.go, alongside the
// refusal ladder and the frame builder it is made of. (The M9d-2 task-2
// placeholder that stood here, and the TestWriteChannel_RefusedUntilTask3
// that pinned it, were both replaced by that file at task 3.)

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
