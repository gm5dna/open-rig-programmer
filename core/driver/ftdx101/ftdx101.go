// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx101

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	// ALIASED deliberately: the dialect package's own name is also
	// "ftdx101", and an unaliased import would put a second meaning on the
	// spelling this package already answers to. catftdx101 reads as "the
	// core/cat side of the FTdx101", which is exactly what it is, and it
	// appears at TWO call sites (modelD and modelMP, below) — one per
	// model, because there are two models and therefore two dialects.
	catftdx101 "github.com/gm5dna/open-rig-programmer/core/cat/ftdx101"
	"github.com/gm5dna/open-rig-programmer/core/driver"
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
		d.consentUnverifiedWrites = true
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
	d := &ftdx101Driver{model: m, profile: profile}
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
	model   modelParams
	profile Profile
	// transportLogger, when non-nil, is threaded into every Session's
	// transport.Engine at Open time — see WithTransportLogger.
	transportLogger transport.Logger
	// consentUnverifiedWrites records the user's consent to unverified
	// writes — set only by WithConsentedUnverifiedWrites, read only by
	// sessionCapabilities. FALSE is the zero value and the default, so a
	// driver built without the option behaves exactly as it did before the
	// option existed.
	consentUnverifiedWrites bool
}

// Model implements driver.Driver.
func (d *ftdx101Driver) Model() string { return d.model.name }

// Capabilities implements driver.Driver: the static baseline for this
// driver's model and profile — no discovered banks (see
// Session.Capabilities for the effective, per-radio set).
func (d *ftdx101Driver) Capabilities() spec.Capabilities {
	switch d.profile {
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

// idSpec is the transport spec for the ID; probe: a fixed 7-byte answer
// ("ID0681;" for the D, "ID0682;" for the MP). The length is core/cat's
// idAnswerLen, and core/cat/ftdx101's reused-command verification checked
// this radio's own ID frame table against it (frame block at layout
// 1069-1078, availability row X O O X at layout 304: no Set, Read "ID;",
// Answer seven bytes) before the shared codec was accepted — see that
// package's doc.go. One retry: an identity read is idempotent and Open
// should survive a single swallowed reply.
func idSpec() transport.CommandSpec {
	return transport.CommandSpec{ExpectPrefix: "ID", ExpectLen: 7, RetryReads: 1}
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
	var engOpts []transport.Option
	if d.transportLogger != nil {
		engOpts = append(engOpts, transport.WithLogger(d.transportLogger))
	}
	eng, err := transport.NewEngine(port, d.model.dialect, engOpts...)
	if err != nil {
		// NewEngine has not taken the port on this path (it refuses before
		// touching it), so closing it here is Open's own ownership
		// obligation, not a double close.
		_ = port.Close()
		return nil, fmt.Errorf("ftdx101: Open: %w", err)
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
	if err := eng.Init(ctx); err != nil {
		return nil, fmt.Errorf("ftdx101: Open: %w", err)
	}

	// Identity probe: the ID; answer is authoritative (matrix §3.10,
	// MANUAL-EVIDENCED — the D answers "ID0681;", the MP "ID0682;"), and
	// anything else means the wrong radio, or something else that speaks
	// CAT, is on this port.
	//
	// THE SIBLING CASE IS THE ONE THIS PACKAGE EXISTS TO GET RIGHT: an
	// FTDX101MP answering a D driver's probe is not a foreign radio but the
	// other half of this very package, and it must be refused with BOTH
	// models named rather than with two bare four-digit IDs a user would
	// have to look up. The refusal is a CHOICE and its wording is too; the
	// IDs it compares are the manual's.
	catID := m.dialect.CATID()
	frame, err := eng.Do(ctx, m.dialect.BuildIDRead(), idSpec())
	if err != nil {
		return nil, fmt.Errorf("ftdx101: Open: ID probe: %w", err)
	}
	got, err := m.dialect.ParseIDAnswer(frame)
	if err != nil {
		return nil, fmt.Errorf("ftdx101: Open: ID probe: %w", err)
	}
	if got != catID {
		// BOTH name fields are populated when the found ID is one this
		// package knows, and BOTH must be: driver.WrongRadioError.Error()
		// renders its named form only when WantModel and GotModel are both
		// set, so a driver that filled one alone would produce the ID-only
		// text from Error() while cmd/rigprog's probe formatter — which
		// keys on GotModel alone — rendered the named one, and the same
		// refusal would read two different ways in two places. GotModel is
		// "" for an ID this package does not register, which is the
		// deliberate ID-only path (siblingNames' doc comment).
		return nil, &driver.WrongRadioError{
			Want:      catID,
			Got:       got,
			WantModel: m.name,
			GotModel:  siblingNames[got],
		}
	}
	id.CATID = got

	slots60m, emg, err := discoverInventory(ctx, m.dialect, eng)
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
	caps := effectiveCapabilities(d.model.dialect, d.Capabilities(), slots60m, emg)
	if d.consentUnverifiedWrites && d.profileRecognised() {
		caps = spec.ConsentUnverifiedWrites(caps)
	}
	return caps
}

// profileRecognised reports whether this driver's profile is one of the
// package's declared Profile constants — the same set Capabilities' switch
// names explicitly, restated here so the consent gate cannot drift open for
// a profile that switch would fail safe on. It carries no model dimension
// because Profile carries none: which radio a driver is for is fixed by
// which constructor built it (plan D1).
func (d *ftdx101Driver) profileRecognised() bool {
	switch d.profile {
	case Simulated, RealHardware:
		return true
	}
	return false
}

// discoverInventory probes this radio's 5xx and EMG channel inventory:
// EVERY slot the dialect's own 5xx space declares, in ascending order, then
// the EMG slot. It returns the wire forms that answered, in probe order,
// and whether EMG did.
//
// NO TERMINATION ASSUMPTIONS, and this is the whole design (doc.go,
// "Discovery walks the WHOLE declared range"; matrix §3.4): no contiguity
// from the first slot, no stop at the first rejection, no cap, no sentinel.
// Each of those is an FT-710 hardware fact about a radio whose factory 5xx
// channels are believed contiguous and non-erasable; nothing of the kind is
// known for either FTdx101, a populated 503 behind an empty 502 is entirely
// possible, and a walk that stopped early would report a truncated
// inventory as a complete one. The price is ~100 exchanges per Open,
// ACCEPTED and budgeted — TWICE OVER for this package, since two models
// means two models' worth of sessions in every gate. Anybody tempted to
// trim it must read doc.go's discovery section first, because the trimming
// IS the assumption.
//
// The range's extent comes from the DIALECT, by asking SixtyMSlot for
// successive ordinals until it refuses one: the last accepted ordinal is
// that dialect's declared ceiling, so no bound is written down here and a
// dialect declaring a different 5xx space would be walked correctly. The
// loop provably terminates — SixtyMSlot refuses every ordinal past
// (SixtyHi - SixtyLo + 1), and refuses ordinal 1 outright for a dialect
// with no 5xx space at all, which yields zero probes rather than a spurious
// one. The 501..599 NUMBERING itself is the DIALECT register's entry
// "SlotSpace.SixtyLo/SixtyHi = 501/599", cited not restated; what a
// REJECTION MEANS is this driver's own register entry 7.
//
// A discovery error is FATAL to Open (see open, above): a malformed answer
// or a wrong-slot echo aborts the walk and closes the port, and no bank is
// synthesised from the partial result. Discovery refuses rather than
// guesses — half a walk is not a smaller inventory, it is an unknown one.
func discoverInventory(ctx context.Context, dialect cat.Dialect, eng *transport.Engine) (slots60m []string, emg bool, err error) {
	for n := 1; ; n++ {
		slot, serr := dialect.SixtyMSlot(n)
		if serr != nil {
			// Past this dialect's declared 5xx space: the walk is complete.
			// This is the ONLY loop exit — never a rejection.
			break
		}
		populated, perr := probeSlot(ctx, dialect, eng, slot)
		if perr != nil {
			return nil, false, perr
		}
		if populated {
			slots60m = append(slots60m, slot.Wire())
		}
	}

	emgSlot := dialect.EMGSlot()
	if emgSlot.Wire() == "" {
		// A dialect with no emergency channel: nothing to probe, and no EMG
		// bank. (core/cat/ftdx101 declares "EMG" — the wire form is
		// MANUAL-EVIDENCED, matrix §1.3.4 — so this is the defensive
		// branch, not either FTdx101's path.)
		return slots60m, false, nil
	}
	emg, err = probeSlot(ctx, dialect, eng, emgSlot)
	if err != nil {
		return nil, false, err
	}
	return slots60m, emg, nil
}

// probeSlot MT-reads one slot purely for existence: a well-formed answer
// naming the probed slot reports populated, a "?;" rejection reports not
// populated (ASSUMED — register entry 7), and anything else is an error.
//
// It probes with the COMBINED MT READ, not MR: this driver never sends MR
// at all (doc.go, "MR is deliberately unused"), and a discovery path that
// quietly did would make that statement false while nothing failed.
//
// Unlike Session.ReadChannel it maps no fields — discovery only needs to
// know whether the slot answered — but it does parse the answer and check
// the slot echo, so a radio answering for a different slot raises the typed
// *AnswerMismatchError here rather than silently adding the wrong channel
// to a capability bank.
func probeSlot(ctx context.Context, dialect cat.Dialect, eng *transport.Engine, slot cat.Slot) (bool, error) {
	cmd, err := dialect.BuildMTRead(slot)
	if err != nil {
		return false, err
	}
	// cmdSpec, not spec: the package spec (core/spec) is imported here.
	cmdSpec, err := mtSpec(dialect)
	if err != nil {
		return false, err
	}
	frame, err := eng.Do(ctx, cmd, cmdSpec)
	if errors.Is(err, cat.ErrRejected) {
		// ASSUMED absent — see register entry 7.
		return false, nil
	}
	if err != nil {
		return false, err
	}
	m, _, err := dialect.ParseMTAnswerCombined(frame)
	if err != nil {
		return false, fmt.Errorf("probe %s: %w", slot.Wire(), err)
	}
	if m.Slot.Wire() != slot.Wire() {
		return false, &AnswerMismatchError{Requested: slot.Wire(), Answered: m.Slot.Wire()}
	}
	return true, nil
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
// There is NO operation mutex, and that is a consequence of the MT-only
// choreography rather than an omission: every logical operation this
// session performs is exactly ONE wire exchange (ReadChannel's combined MT
// read; the combined MT Set the write path will send), so there is no gap
// between two frames of the same operation for a concurrent operation to
// land in. The FT-710's Session holds an opMu precisely because its
// operations are two exchanges each (MR+MT, MW+MT) and a concurrent write
// landing between a read's two halves tears the channel — frequency from
// one moment, tag from another. See doc.go: a future FTdx101 operation
// needing two frames needs an opMu with it.
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
// capability; like the FT-710's and the FTdx10's, it is a method on the
// concrete *Session rather than part of driver.Session, because which
// diagnostics exist is a per-driver matter.
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

// WriteChannel — the MT-only combined Set — lives in write.go, alongside the
// refusal ladder and the frame builder it is made of. (The M9d-2 task-2
// placeholder that stood here, and the TestWriteChannel_RefusedUntilTask3
// that pinned it, were both replaced by that file at task 3.)

// ErrAnswerMismatch is the sentinel a caller should compare against (via
// errors.Is) when a slot-addressed answer names a DIFFERENT slot than the
// one just requested. The transport's quarantine discipline makes this
// unlikely (a stale same-shape reply should have been drained), but the
// driver still refuses to map an answer onto the wrong slot.  The error
// actually returned is an *AnswerMismatchError.
var ErrAnswerMismatch = errors.New("ftdx101: answer names a different slot than was requested")

// AnswerMismatchError reports the requested and the answered slot. It is
// this PACKAGE's own typed error, in this package's own namespace: the
// FT-710 and FTdx10 drivers have same-shaped ones, and none imports
// another — a caller distinguishing which radio's read went wrong needs
// distinct types, and a shared one would put a radio-specific failure on a
// seam that is meant to be neutral.
//
// ONE type for BOTH models, unlike the two capability sets: the failure it
// reports is a property of the combined MT record's slot echo, which matrix
// §2.5 shows is printed once for both radios, and the model that met it is
// already legible from the session the caller holds.
type AnswerMismatchError struct {
	// Requested is the slot the read asked for.
	Requested string
	// Answered is the slot the reply actually named.
	Answered string
}

// Error implements the error interface.
func (e *AnswerMismatchError) Error() string {
	return fmt.Sprintf("ftdx101: requested slot %q but the answer names slot %q — refusing to map a reply onto the wrong slot", e.Requested, e.Answered)
}

// Unwrap lets errors.Is(err, ErrAnswerMismatch) match.
func (e *AnswerMismatchError) Unwrap() error { return ErrAnswerMismatch }
