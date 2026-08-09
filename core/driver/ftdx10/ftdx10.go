// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx10

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	// ALIASED deliberately: the dialect package's own name is also
	// "ftdx10", and an unaliased import would put a second meaning on the
	// spelling this package already answers to. catftdx10 reads as "the
	// core/cat side of the FTdx10", which is exactly what it is, and it
	// appears at ONE call site (catDialect, below).
	catftdx10 "github.com/gm5dna/open-rig-programmer/core/cat/ftdx10"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// catDialect is the CAT dialect this driver speaks: the ONE place
// core/driver/ftdx10 names an instance from core/cat. Everything else here
// derives from it — ftdx10Driver.dialect (and through it every Session's),
// catID, the capability data's modes and clarifier policy, the slot
// inventories, and the MT answer geometry — so the package has a single
// construction site rather than a scatter of references for a third radio
// to miss.
//
// Deliberately a package-level binding rather than a per-driver argument,
// mirroring the FT-710 driver's own reasoning: the FTdx10's dialect is
// what makes this package the FTdx10's driver, and the package-level
// construction that runs before any driver exists (catID, the bank slot
// lists) needs it too. No METHOD reaches for it: every codec call goes
// through the dialect field the driver or session carries, so a
// hand-built driver with a zero dialect fails closed rather than
// silently borrowing this one (see TestOpen_UnconfiguredDialectRefusesToOpen).
var catDialect = catftdx10.Dialect()

// Option configures the driver New builds — and, through it, every
// Session its Open call establishes. See WithTransportLogger.
type Option func(*ftdx10Driver)

// WithTransportLogger sets the transport.Logger every Session this driver
// Opens threads into its transport.Engine. Without it, the engine's
// diagnostics — unexpected frames, quarantine drains, contamination
// (transport safety obligation 3: "surfaced, never silently discarded") —
// fall into the engine's own drop-everything default with no way for a
// caller of this driver to receive them. A nil l is ignored (the engine's
// default is kept).
func WithTransportLogger(l transport.Logger) Option {
	return func(d *ftdx10Driver) {
		if l != nil {
			d.transportLogger = l
		}
	}
}

// New builds the FTdx10 driver for profile. RealHardware — the ZERO VALUE
// — selects the all-Unverified capability set while writeTrialsComplete is
// false (nothing writable; see that constant's doc comment), and ANY
// unrecognised Profile value deliberately selects the same fail-safe: the
// failure direction for a forged or corrupted Profile is always "nothing
// writable", never a writable set. Options: WithTransportLogger.
func New(profile Profile, opts ...Option) driver.Driver {
	d := &ftdx10Driver{profile: profile, dialect: catDialect}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// ftdx10Driver implements driver.Driver for the Yaesu FTdx10.
type ftdx10Driver struct {
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
}

// Model implements driver.Driver.
func (d *ftdx10Driver) Model() string { return modelName }

// Capabilities implements driver.Driver: the static baseline for this
// driver's profile — no discovered banks (see Session.Capabilities for the
// effective, per-radio set).
func (d *ftdx10Driver) Capabilities() spec.Capabilities {
	switch d.profile {
	case Simulated:
		return CapabilitiesSimulated()
	case RealHardware:
		// No FTdx10 has ever been written to over CAT by this project:
		// writeTrialsComplete is false, so there is no hardware-verified
		// profile for this arm to select and a real-hardware session gets
		// the all-Unverified fail-safe — nothing writable, every write
		// refused before a frame is built. See writeTrialsComplete's doc
		// comment for what its flip must change, and why the constant is
		// deliberately not load-bearing on its own.
		return CapabilitiesUnverified()
	default:
		// Any unrecognised Profile value fails safe, through its own
		// explicit arm rather than by sharing RealHardware's: the two
		// happen to return the same profile today, and a reader must be
		// able to see that the fail-safe is a decision rather than a
		// coincidence of the flip's state. Pinned by
		// TestProfileMatrix_StaticPerField's invalid-profile row.
		return CapabilitiesUnverified()
	}
}

// idSpec is the transport spec for the ID; probe: a fixed 7-byte answer
// ("ID0761;"). The length is core/cat's idAnswerLen, and
// core/cat/ftdx10's reused-command verification checked this radio's own
// ID frame table against it (manual lines 976-984: no Set, Read "ID;",
// Answer seven bytes) before the shared codec was accepted — see that
// package's doc.go. One retry: an identity read is idempotent and Open
// should survive a single swallowed reply.
func idSpec() transport.CommandSpec {
	return transport.CommandSpec{ExpectPrefix: "ID", ExpectLen: 7, RetryReads: 1}
}

// Open implements driver.Driver: it builds a transport.Engine over port,
// establishes the session (Init: AI0 + drain-to-quiet), probes ID; and
// verifies this really is an FTdx10 (a typed *driver.WrongRadioError
// otherwise), then discovers this radio's 5xx/EMG channel inventory and
// returns a Session whose effective capabilities include the discovered
// banks as read-only.
//
// Open takes ownership of port on BOTH outcomes: the Session's Close
// releases it on success, and Open itself closes it before returning an
// error.
//
// The engine is bound to THIS driver's own dialect, passed WHOLE
// (d.dialect — see transport.NewEngine): both the outbound allowlist a
// session enforces and the AI init frame it opens with belong to the radio
// the session is for, never a package-level default that would gate every
// radio by whatever one of them permits. transport.NewEngine refuses an
// unconfigured dialect outright, so there is no ungated path through here
// even if the field were somehow left zero.
func (d *ftdx10Driver) Open(ctx context.Context, port transport.Port, id driver.Identity) (driver.Session, error) {
	var engOpts []transport.Option
	if d.transportLogger != nil {
		engOpts = append(engOpts, transport.WithLogger(d.transportLogger))
	}
	eng, err := transport.NewEngine(port, d.dialect, engOpts...)
	if err != nil {
		// NewEngine has not taken the port on this path (it refuses
		// before touching it), so closing it here is Open's own ownership
		// obligation, not a double close.
		_ = port.Close()
		return nil, fmt.Errorf("ftdx10: Open: %w", err)
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
func (d *ftdx10Driver) open(ctx context.Context, eng *transport.Engine, id driver.Identity) (*Session, error) {
	if err := eng.Init(ctx); err != nil {
		return nil, fmt.Errorf("ftdx10: Open: %w", err)
	}

	// Identity probe: the ID; answer is authoritative, and anything other
	// than the FTdx10's "0761" means the wrong radio (or something else
	// that speaks CAT) is on this port. An FT-710 answers "ID0800;" and
	// must be refused here rather than driven with FTdx10 frames.
	frame, err := eng.Do(ctx, d.dialect.BuildIDRead(), idSpec())
	if err != nil {
		return nil, fmt.Errorf("ftdx10: Open: ID probe: %w", err)
	}
	got, err := d.dialect.ParseIDAnswer(frame)
	if err != nil {
		return nil, fmt.Errorf("ftdx10: Open: ID probe: %w", err)
	}
	if got != catID {
		return nil, &driver.WrongRadioError{Want: catID, Got: got}
	}
	id.CATID = got

	slots60m, emg, err := discoverInventory(ctx, d.dialect, eng)
	if err != nil {
		return nil, fmt.Errorf("ftdx10: Open: 5xx/EMG discovery: %w", err)
	}

	return &Session{
		eng:     eng,
		dialect: d.dialect,
		id:      id,
		caps:    effectiveCapabilities(d.Capabilities(), slots60m, emg),
	}, nil
}

// discoverInventory probes this radio's 5xx and EMG channel inventory:
// EVERY slot the dialect's own 5xx space declares, in ascending order,
// then the EMG slot. It returns the wire forms that answered, in probe
// order, and whether EMG did.
//
// NO TERMINATION ASSUMPTIONS, and this is the whole design (doc.go,
// "Discovery walks the WHOLE declared range"): no contiguity from the
// first slot, no stop at the first rejection, no cap, no sentinel. Each of
// those is an FT-710 hardware fact about a radio whose factory 5xx
// channels are believed contiguous and non-erasable; on this radio, a
// populated 503 behind an empty 502 is entirely possible and a walk that
// stopped early would report a truncated inventory as a complete one. The
// price is ~100 exchanges per Open, accepted and budgeted; anybody
// tempted to trim it must read that doc.go section first, because the
// trimming IS the assumption.
//
// The range's extent comes from the DIALECT, by asking SixtyMSlot for
// successive ordinals until it refuses one: the last accepted ordinal is
// this dialect's declared ceiling, so no bound is written down here and a
// dialect that declared a different 5xx space would be walked correctly.
// The loop provably terminates — SixtyMSlot refuses every ordinal past
// (sixtyHi - sixtyLo + 1), and refuses ordinal 1 outright for a dialect
// with no 5xx space at all, which yields zero probes rather than a
// spurious one. The 501..599 NUMBERING itself is the DIALECT's ASSUMED
// register's SlotSpace.SixtyLo/SixtyHi entry, cited not restated; what a
// rejection MEANS is this driver's own "?;" ON A 5xx/EMG DISCOVERY PROBE
// entry.
func discoverInventory(ctx context.Context, dialect cat.Dialect, eng *transport.Engine) (slots60m []string, emg bool, err error) {
	for n := 1; ; n++ {
		slot, serr := dialect.SixtyMSlot(n)
		if serr != nil {
			// Past this dialect's declared 5xx space: the walk is
			// complete. This is the ONLY loop exit — never a rejection.
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
		// A dialect with no emergency channel: nothing to probe, and no
		// EMG bank. (core/cat/ftdx10 declares "EMG", so this is the
		// defensive branch, not the FTdx10's path.)
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
// populated (ASSUMED — the "?;" ON A 5xx/EMG DISCOVERY PROBE register
// entry), and anything else is an error.
//
// It probes with the COMBINED MT READ, not MR: this driver never sends MR
// at all (doc.go, "MR is deliberately unused"), and a discovery path that
// quietly did would make that statement false while nothing failed.
//
// Unlike Session.ReadChannel it maps no fields — discovery only needs to
// know whether the slot answered — but it does parse the answer and check
// the slot echo, so a radio answering for a different slot raises the
// typed *AnswerMismatchError here rather than silently adding the wrong
// channel to a capability bank.
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
		// ASSUMED absent — see the "?;" ON A 5xx/EMG DISCOVERY PROBE
		// register entry.
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
// driver.DiscoveredBankSynthesizer capability (core/driver/optional.go):
// it classifies an OFFLINE slot list — a working codeplug's own slots,
// with no live session anywhere — into the read-only 60M/EMG banks a live
// session's Open would have discovered for a radio whose inventory
// contained them.
//
// Implementing it is not optional in practice, whatever the interface
// says: its ABSENCE fails silently. internal/wiring's synthesis helper
// returns false for a driver that does not satisfy the interface, and the
// app then renders no discovered banks at all for an offline FTdx10
// codeplug — no error, no log, just missing rows.
//
// It deliberately reuses effectiveCapabilities itself rather than
// restating bank shape (label, NoBlank, Fields): d.Capabilities() is the
// exact same "base" argument Open passes it, so slotting the classified
// slots into the identical call makes drift between offline synthesis and
// live discovery structurally impossible rather than merely tested for.
// Slicing off the base banks — base.Banks' own length, before any
// discovered bank is appended — yields exactly the newly discovered ones,
// in effectiveCapabilities' fixed 60M-then-EMG order.
//
// Classification: a slot already claimed by one of base's static banks
// (MEM/PMS) is excluded first, because this method only ever adds banks
// discovery would ADD and never restates a static one. Every remaining
// slot is parsed by THIS DRIVER'S dialect (the same authoritative parser
// discovery's own slot builders agree with) and classified by
// Slot.Is60m/IsEMG, preserving input ORDER; a slot that parses to neither
// is unclassifiable and OMITTED, never guessed into a bank.
//
// A REPEATED "EMG" collapses to the single physical channel. Live
// discovery probes one EMG slot and can never produce a duplicate, so a
// duplicate can only come from a semantically invalid input list (LoadFile
// validates only AFTER loading); reporting one EMG row for it keeps this
// method's output identical to what a live session would carry.
// core/driver/ft710 makes the opposite choice — it preserves every
// occurrence — for a compatibility reason of its own (the pre-M9a app
// synthesis it replaced did), and that is its history, not a rule.
// TestSynthesiseDiscoveredBanks_DuplicateEMGCollapses pins this driver's
// choice so it stays a decision.
func (d *ftdx10Driver) SynthesiseDiscoveredBanks(slots []string) []spec.Bank {
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

// Session is the FTdx10's driver.Session: one open, identity-verified,
// inventory-discovered connection. Safe for concurrent use —
// transport.Engine serialises every individual exchange, and everything
// else here is immutable after Open.
//
// There is NO operation mutex, and that is a consequence of the MT-only
// choreography rather than an omission: every logical operation this
// session performs is exactly ONE wire exchange (ReadChannel's combined MT
// read; WriteChannel's combined MT Set, write.go), so there is no gap
// between two frames of the same operation for a concurrent operation to
// land in. The FT-710's Session holds an opMu precisely because its
// operations are two exchanges each (MR+MT, MW+MT) and a concurrent write
// landing between a read's two halves tears the channel — frequency from
// one moment, tag from another. See doc.go: a future FTdx10 operation
// needing two frames needs an opMu with it.
type Session struct {
	eng *transport.Engine
	// dialect is the CAT dialect this session's every codec call goes
	// through — builders, parsers, the answer geometry, and the mode
	// rendering ReadChannel puts in front of the user. Copied from the
	// ftdx10Driver that Opened it (which is also where the engine's gate
	// came from), so a session can never encode with one radio's dialect
	// while its transport gates with another's.
	dialect cat.Dialect
	id      driver.Identity
	caps    spec.Capabilities // effective; never mutated after Open
}

// Identity implements driver.Session.
func (s *Session) Identity() driver.Identity { return s.id }

// Capabilities implements driver.Session: the EFFECTIVE capability set
// (profile baseline plus the discovered read-only 60M/EMG banks), as a
// deep copy per call — see cloneCapabilities for why the copy is
// load-bearing.
func (s *Session) Capabilities() spec.Capabilities {
	return cloneCapabilities(s.caps)
}

// Diagnostics reports this session's transport-level health counters as a
// point-in-time snapshot — the driver-layer surface for the engine's own
// accessors, which are otherwise unreachable (the *transport.Engine is
// unexported inside this Session). Safe for concurrent use, like the
// accessor it wraps. It satisfies the optional
// driver.DiagnosticsReporter capability; like the FT-710's, it is a method
// on the concrete *Session rather than part of driver.Session, because
// which diagnostics exist is a per-driver matter.
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

// ErrAnswerMismatch is the sentinel a caller should compare against (via
// errors.Is) when a slot-addressed answer names a DIFFERENT slot than the
// one just requested. The transport's quarantine discipline makes this
// unlikely (a stale same-shape reply should have been drained), but the
// driver still refuses to map an answer onto the wrong slot. The error
// actually returned is an *AnswerMismatchError.
var ErrAnswerMismatch = errors.New("ftdx10: answer names a different slot than was requested")

// AnswerMismatchError reports the requested and the answered slot. It is
// this driver's OWN typed error, in this driver's own namespace: the
// FT-710 driver has a same-shaped one, and neither imports the other —
// a caller distinguishing which radio's read went wrong needs two
// distinct types, and a shared one would put a radio-specific failure on
// a seam that is meant to be neutral.
type AnswerMismatchError struct {
	// Requested is the slot the read asked for.
	Requested string
	// Answered is the slot the reply actually named.
	Answered string
}

// Error implements the error interface.
func (e *AnswerMismatchError) Error() string {
	return fmt.Sprintf("ftdx10: requested slot %q but the answer names slot %q — refusing to map a reply onto the wrong slot", e.Requested, e.Answered)
}

// Unwrap lets errors.Is(err, ErrAnswerMismatch) match.
func (e *AnswerMismatchError) Unwrap() error { return ErrAnswerMismatch }
