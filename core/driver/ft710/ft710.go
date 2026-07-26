// SPDX-License-Identifier: GPL-3.0-or-later

package ft710

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// Option configures the driver New builds — and, through it, every
// Session its Open call establishes. See WithTransportLogger.
type Option func(*ft710Driver)

// WithTransportLogger sets the transport.Logger every Session this driver
// Opens threads into its transport.Engine (M3 Codex-review fix wave, Fix
// 5, adjudicated MEDIUM). Without it, the engine's diagnostics —
// unexpected frames, quarantine drains, contamination (transport safety
// obligation 3: "surfaced, never silently discarded") — fell into the
// engine's own drop-everything default with no way for any caller of
// this driver to receive them: the driver never exposed a path to the
// engine's logger seam at all. A nil l is ignored (the engine's default
// is kept).
func WithTransportLogger(l transport.Logger) Option {
	return func(d *ft710Driver) {
		if l != nil {
			d.transportLogger = l
		}
	}
}

// New builds the FT-710 driver for profile. RealHardware selects the
// hardware-verified profile now writeTrialsComplete is true
// (CapabilitiesRealHardware — see writeTrialsComplete's doc comment for
// the M5b evidence); ANY unrecognised Profile value still deliberately
// selects the all-Unverified fail-safe profile: the failure direction
// for a forged or corrupted Profile is always "nothing writable", never
// a writable set. See Profile and writeTrialsComplete. Options:
// WithTransportLogger.
func New(profile Profile, opts ...Option) driver.Driver {
	d := &ft710Driver{profile: profile}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// ft710Driver implements driver.Driver for the Yaesu FT-710.
type ft710Driver struct {
	profile Profile
	// transportLogger, when non-nil, is threaded into every Session's
	// transport.Engine at Open time — see WithTransportLogger.
	transportLogger transport.Logger
}

// Model implements driver.Driver.
func (d *ft710Driver) Model() string { return modelName }

// Capabilities implements driver.Driver: the static baseline for this
// driver's profile — no discovered banks (see Session.Capabilities for
// the effective, per-radio set).
func (d *ft710Driver) Capabilities() spec.Capabilities {
	switch d.profile {
	case Simulated:
		return CapabilitiesSimulated()
	case RealHardware:
		if !writeTrialsComplete {
			// The pre-M5b state, kept as the guard's own mechanism: were
			// the flip ever reverted, real-hardware sessions would fall
			// straight back to nothing-writable.
			return CapabilitiesUnverified()
		}
		// The M5b PR flipped writeTrialsComplete (13/07/2026) and
		// introduced this hardware-verified profile alongside it — see
		// both doc comments in caps.go for the evidence.
		return CapabilitiesRealHardware()
	default:
		// Any unrecognised Profile value fails safe: all-Unverified,
		// nothing writable — a forged/corrupted Profile must never
		// select a writable capability set. (Pre-flip this fell out of
		// the same branch RealHardware used; post-flip it needs — and
		// has — its own explicit arm, pinned by
		// TestDriver_ModelAndBaseline's "unrecognised Profile" case.)
		return CapabilitiesUnverified()
	}
}

// StaticSettingsDescriptor implements the optional
// driver.StaticSettingsProvider capability (task 37, core/driver/
// optional.go): the driver-level, no-session-required counterpart to
// Session.SettingsDescriptor (settings.go). Identical to both: this
// driver's settings tree depends only on the static EX inventory, never
// on anything a live session discovers, so all three call sites — the
// package-level SettingsDescriptor func, the Session method, and this
// Driver method — return equal trees.
func (d *ft710Driver) StaticSettingsDescriptor() driver.SettingsDescriptor {
	return SettingsDescriptor()
}

// idSpec is the transport spec for the ID; probe: fixed 7-byte answer
// ("ID0800;"). One retry: an identity read is idempotent and Open should
// survive a single swallowed reply.
func idSpec() transport.CommandSpec {
	return transport.CommandSpec{ExpectPrefix: "ID", ExpectLen: 7, RetryReads: 1}
}

// mrSpec is the transport spec for an MR read: fixed 28-byte answer. One
// retry — reads are idempotent.
func mrSpec() transport.CommandSpec {
	return transport.CommandSpec{ExpectPrefix: "MR", ExpectLen: 28, RetryReads: 1}
}

// mtSpec is the transport spec for an MT read: variable-length answer
// ("MT" + slot + display + 0-12 byte tag + ";"), so only the prefix is
// pinned. One retry — reads are idempotent.
func mtSpec() transport.CommandSpec {
	return transport.CommandSpec{ExpectPrefix: "MT", RetryReads: 1}
}

// fnfSpec is the transport spec for a fire-and-forget Set (MW, MT set):
// no answer expected, only the bounded listen for a delayed "?;"
// rejection. All defaults; RetryReads MUST stay 0 — a write is never
// resent (transport safety obligation 2 enforces this structurally).
func fnfSpec() transport.CommandSpec {
	return transport.CommandSpec{}
}

// max60mProbe bounds 60 m discovery: the largest known regional 60 m
// channel set is the US's 15. ASSUMED (cross-ref fakeradio's register,
// items 10/12/13): both the 501.. numbering and the per-region counts
// are unverified until M5a reads a real radio.
const max60mProbe = 15

// Open implements driver.Driver: it builds a transport.Engine over port,
// establishes the session (Init: AI0 + drain-to-quiet), probes ID; and
// verifies this really is an FT-710 (typed ErrWrongRadio otherwise), then
// discovers the radio's regional 60 m/EMG channel inventory and returns a
// Session whose effective capabilities include the discovered banks as
// read-only. Open takes ownership of port: on any error the engine (and
// with it the port) is closed before returning.
func (d *ft710Driver) Open(ctx context.Context, port transport.Port, id driver.Identity) (driver.Session, error) {
	var engOpts []transport.Option
	if d.transportLogger != nil {
		// Fix 5: thread the caller's transport.Logger into the engine —
		// see WithTransportLogger.
		engOpts = append(engOpts, transport.WithLogger(d.transportLogger))
	}
	eng := transport.NewEngine(port, engOpts...)
	sess, err := d.open(ctx, eng, id)
	if err != nil {
		_ = eng.Close()
		return nil, err
	}
	return sess, nil
}

// open is Open's body, factored so the error path can close eng in
// exactly one place.
func (d *ft710Driver) open(ctx context.Context, eng *transport.Engine, id driver.Identity) (*Session, error) {
	if err := eng.Init(ctx); err != nil {
		return nil, fmt.Errorf("ft710: Open: %w", err)
	}

	// Identity probe: the ID; answer is authoritative, and anything
	// other than the FT-710's fixed "0800" means the wrong radio (or
	// something else that speaks CAT) is on this port.
	frame, err := eng.Do(ctx, cat.FT710.BuildIDRead(), idSpec())
	if err != nil {
		return nil, fmt.Errorf("ft710: Open: ID probe: %w", err)
	}
	got, err := cat.FT710.ParseIDAnswer(frame)
	if err != nil {
		return nil, fmt.Errorf("ft710: Open: ID probe: %w", err)
	}
	if got != catID {
		return nil, &driver.WrongRadioError{Want: catID, Got: got}
	}
	id.CATID = got

	logger := d.transportLogger
	if logger == nil {
		logger = nopLogger{}
	}
	slots60m, emg, overflow60m, err := discoverInventory(ctx, eng, logger)
	if err != nil {
		return nil, fmt.Errorf("ft710: Open: 60m/EMG discovery: %w", err)
	}

	return &Session{
		eng:    eng,
		id:     id,
		caps:   effectiveCapabilities(d.Capabilities(), slots60m, emg),
		region: deriveRegion(len(slots60m), emg, overflow60m),
	}, nil
}

// nopLogger is the fallback transport.Logger when no WithTransportLogger
// was supplied: it drops everything, mirroring transport's own default.
type nopLogger struct{}

func (nopLogger) Printf(string, ...any) {}

// discoverInventory probes the radio's region-dependent channel
// inventory: MR reads of "501", "502", ... stopping at the first "?;"
// rejection, capped at max60mProbe, then a single EMG probe.
//
// The overflow sentinel (M3 Codex-review fix wave, Fix 6): when ALL
// max60mProbe (15) slots answer, one further probe — "516" — checks
// whether the inventory actually continues past the largest KNOWN
// regional set. If it does, overflow60m reports true, the returned
// slots60m stays CAPPED at the 15 verified slots (only slots actually
// probed populated may be claimed in a capability bank), the anomaly is
// logged via logger, and deriveRegion reports "unknown-16plus" instead
// of ever mislabelling such a radio "US". M5a MUST extend max60mProbe
// (and this sentinel's bound with it) if real hardware or a region
// catalogue shows a legitimate >15-channel set — the sentinel exists
// precisely so that discovery FAILS LOUDLY toward "unknown" rather than
// silently reporting a truncated inventory as a complete, known one.
//
// ASSUMED, twice over (cross-ref fakeradio's register, items 2 and
// 10-13, and see doc.go): (a) 60 m slots are numbered sequentially from
// "501"; (b) a "?;" rejection of an MR read means "nothing there", and
// because factory 60 m/EMG channels are believed non-erasable, the FIRST
// rejection is taken as the end of the region's inventory — an
// empty-but-existing 60 m slot would be indistinguishable and would
// truncate discovery. Both must be checked against real hardware at M5a.
func discoverInventory(ctx context.Context, eng *transport.Engine, logger transport.Logger) (slots60m []string, emg, overflow60m bool, err error) {
	for n := 1; n <= max60mProbe; n++ {
		slot, err := cat.FT710.SixtyMSlot(n)
		if err != nil {
			return nil, false, false, err
		}
		populated, err := probeSlot(ctx, eng, slot)
		if err != nil {
			return nil, false, false, err
		}
		if !populated {
			break
		}
		slots60m = append(slots60m, slot.Wire())
	}

	if len(slots60m) == max60mProbe {
		// Every known slot answered: probe the sentinel — see this
		// function's doc comment.
		sentinel, serr := cat.FT710.SixtyMSlot(max60mProbe + 1)
		if serr != nil {
			return nil, false, false, serr
		}
		populated, perr := probeSlot(ctx, eng, sentinel)
		if perr != nil {
			return nil, false, false, perr
		}
		if populated {
			overflow60m = true
			logger.Printf("ft710: 60m discovery: sentinel slot %s answered after all %d known slots — inventory exceeds the largest known regional set; capping the bank at the %d verified slots and reporting region \"unknown-16plus\" (M5a must extend max60mProbe)", sentinel.Wire(), max60mProbe, max60mProbe)
		}
	}

	emg, err = probeSlot(ctx, eng, cat.FT710.EMGSlot())
	if err != nil {
		return nil, false, false, err
	}
	return slots60m, emg, overflow60m, nil
}

// probeSlot MR-reads one slot purely for existence: a valid answer for
// the probed slot reports populated; a "?;" rejection reports not
// populated (ASSUMED — see discoverInventory); anything else is an
// error. Unlike Session.ReadChannel it does not map fields or check
// kind: discovery only needs to know whether the slot answered.
func probeSlot(ctx context.Context, eng *transport.Engine, slot cat.Slot) (bool, error) {
	cmd, err := cat.FT710.BuildMRRead(slot)
	if err != nil {
		return false, err
	}
	frame, err := eng.Do(ctx, cmd, mrSpec())
	if errors.Is(err, cat.ErrRejected) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	m, err := cat.FT710.ParseMRAnswer(frame)
	if err != nil {
		return false, err
	}
	if m.Slot.Wire() != slot.Wire() {
		return false, &AnswerMismatchError{Requested: slot.Wire(), Answered: m.Slot.Wire()}
	}
	return true, nil
}

// deriveRegion names the regulatory region a discovered inventory
// implies: exactly 7 60 m channels and no EMG matches the UK set;
// exactly 15 plus EMG matches the US set; a ZERO-60m/zero-EMG inventory
// reports "no-60m" — HW-CONFIRMED 2026-07-13 (docs/hardware-notes.md
// §60m regional finding): Stuart's real UK FT-710 has no factory 5xx
// bank and no EMG channel at all (front-panel confirmed), so this is a
// KNOWN real variant, not an anomaly, and gets its own honest label
// rather than falling into the generic "unknown-N" bucket. Anything else
// still reports "unknown-N" (N = 60 m channel count; whether EMG was
// present is not encoded) — a genuinely unrecognised inventory shape.
// overflow60m — the 60 m sentinel having ALSO answered after all 15
// known slots (Fix 6, see discoverInventory) — overrides everything:
// such an inventory exceeds every known regional set, so it is
// "unknown-16plus", never "US", however US-like its first 15 slots look.
// The "UK"/"US" channel-count fingerprints mirror fakeradio's factory
// images and remain ASSUMED pending further hardware sessions — M5a
// characterised only the zero-60m/zero-EMG case; a real 7-60m-channel or
// 15-60m-plus-EMG radio's inventory may yet disagree with both.
func deriveRegion(count60m int, emg, overflow60m bool) string {
	switch {
	case overflow60m:
		return "unknown-16plus"
	case count60m == 7 && !emg:
		return "UK"
	case count60m == 15 && emg:
		return "US"
	case count60m == 0 && !emg:
		return "no-60m"
	default:
		return fmt.Sprintf("unknown-%d", count60m)
	}
}

// effectiveCapabilities builds a Session's capability set: a deep copy of
// the profile baseline, plus one read-only bank per discovered inventory
// (60M when any 5xx slot answered, EMG when EMG did). The discovered
// banks' field maps mirror the baseline MEM bank's READ supports with
// every Write forced to Unsupported: MW cannot target 5xx/EMG slots at
// all (cat.Slot.Writable), so no profile — not even Simulated — may
// claim them writable.
func effectiveCapabilities(base spec.Capabilities, slots60m []string, emg bool) spec.Capabilities {
	caps := cloneCapabilities(base)

	if len(slots60m) > 0 {
		caps.Banks = append(caps.Banks, spec.Bank{
			ID:    spec.Bank60m,
			Label: bank60mLabel,
			Slots: append([]string(nil), slots60m...),
			// NoBlank: these channels exist because they answered a
			// read, and CAT has no way to blank them.
			NoBlank: true,
			Fields:  readOnlyFields(base),
		})
	}
	if emg {
		caps.Banks = append(caps.Banks, spec.Bank{
			ID:      spec.BankEMG,
			Label:   bankEMGLabel,
			Slots:   []string{"EMG"},
			NoBlank: true,
			Fields:  readOnlyFields(base),
		})
	}
	return caps
}

// SynthesiseDiscoveredBanks implements the optional
// driver.DiscoveredBankSynthesizer capability (task 37, core/driver/
// optional.go): classifies an OFFLINE slot list — e.g. a working
// codeplug's own slots, with no live session at all — into the read-only
// 60M/EMG banks a live session's Open would have discovered for a radio
// whose inventory contains them.
//
// This deliberately reuses effectiveCapabilities itself, rather than
// restating its bank-shape logic (label, NoBlank, Fields): d.Capabilities()
// is the exact same "base" argument Open passes it, and slotting the
// classified slots60m/emg into the identical call guarantees this
// method's output can NEVER drift from what live discovery would have
// produced — the equivalence task 37's brief requires is structural, not
// merely tested for. Slicing off the base banks (base.Banks's own
// length, before any discovered bank is appended) yields exactly the
// newly-discovered banks, in effectiveCapabilities' own fixed
// 60M-then-EMG order.
//
// Classification: a slot already claimed by one of base's own static
// banks (MEM/PMS) is excluded before classification — this method only
// ever adds banks Open's discovery would ADD, never restates a static
// one. Every remaining slot is parsed via cat.ParseSlot (the SAME
// authoritative slot parser discoverInventory's own SixtyMSlot/EMGSlot
// builders agree with) and classified by Slot.Is60m/IsEMG, preserving
// the ORDER slots appeared in the input slice; a slot that parses to
// neither — a malformed or unrecognised wire form — is unclassifiable
// and OMITTED, never guessed into a bank.
//
// Duplicate EMG occurrences (M9a Codex-review finding 1, LOW): a live
// session probes the single physical EMG slot, so effectiveCapabilities
// represents EMG as one flag and mints an EMG bank whose Slots is exactly
// ["EMG"]. Offline classification, however, can be handed a semantically
// INVALID slot list — LoadFile accepts a file and validates it only AFTER
// loading — so the same "EMG" wire form may appear more than once. The
// pre-M9a offline app synthesis (app/uispec.go's own
// synthesiseDiscoveredBanks, since migrated onto this seam) preserved
// EVERY input occurrence; this reproduces that by collecting the EMG
// occurrences and, when there is more than one, replacing the reused
// bank's single-slot list with the full set. One or zero EMG stays
// byte-identical to effectiveCapabilities' own output — keeping the
// task-37 equivalence test meaningful — so ONLY the duplicate case,
// which live discovery can never produce, diverges from it.
func (d *ft710Driver) SynthesiseDiscoveredBanks(slots []string) []spec.Bank {
	base := d.Capabilities()
	numBaseBanks := len(base.Banks)

	claimed := make(map[string]bool)
	for _, b := range base.Banks {
		for _, s := range b.Slots {
			claimed[s] = true
		}
	}

	var slots60m []string
	var emgSlots []string
	for _, raw := range slots {
		if claimed[raw] {
			continue
		}
		s, err := cat.FT710.ParseSlot(raw)
		if err != nil {
			continue
		}
		switch {
		case s.Is60m():
			slots60m = append(slots60m, raw)
		case s.IsEMG():
			emgSlots = append(emgSlots, raw)
		}
	}

	disc := effectiveCapabilities(base, slots60m, len(emgSlots) > 0)
	banks := disc.Banks[numBaseBanks:]

	// Preserve every EMG input occurrence in the duplicate case — see the
	// doc comment above. effectiveCapabilities gave the EMG bank a single
	// "EMG" slot (all live discovery can find); offline input may carry
	// more, and the pre-M9a synthesis kept them all.
	if len(emgSlots) > 1 {
		for i := range banks {
			if banks[i].ID == spec.BankEMG {
				banks[i].Slots = append([]string(nil), emgSlots...)
			}
		}
	}
	return banks
}

// readOnlyFields derives a discovered bank's field map from base's MEM
// bank: Read supports carried over unchanged (whatever the profile says
// about reading), every Write forced to Unsupported. Each call returns a
// fresh map.
func readOnlyFields(base spec.Capabilities) map[spec.Field]spec.FieldSupport {
	mem, _ := base.Bank(spec.BankMemory) // Bank() already returns a defensive copy
	fields := make(map[spec.Field]spec.FieldSupport, len(mem.Fields))
	for f, fs := range mem.Fields {
		fs.Write = spec.Unsupported
		fields[f] = fs
	}
	return fields
}

// cloneCapabilities returns a deep copy of caps: Banks (with fresh Slots
// and Fields per bank) and every other slice independently allocated, so
// mutating the copy can never reach the original. This is load-bearing
// for the write gate: Session.Capabilities hands copies out, and a
// caller mutating one must never alter what WriteChannel enforces.
func cloneCapabilities(caps spec.Capabilities) spec.Capabilities {
	out := caps
	out.Banks = make([]spec.Bank, 0, len(caps.Banks))
	for _, b := range caps.Banks {
		// Capabilities.Bank returns a defensive copy of the bank (fresh
		// Slots and Fields) — reuse that guarantee instead of restating
		// the per-field copying here.
		cp, _ := caps.Bank(b.ID)
		out.Banks = append(out.Banks, cp)
	}
	out.Modes = append([]string(nil), caps.Modes...)
	out.CTCSSTones = append([]spec.Tone(nil), caps.CTCSSTones...)
	out.Bauds = append([]int(nil), caps.Bauds...)
	out.RequiredSlots = append([]string(nil), caps.RequiredSlots...)
	out.ShiftOptions = append([]string(nil), caps.ShiftOptions...)
	out.CTCSSStates = append([]spec.ToneState(nil), caps.CTCSSStates...)
	return out
}

// Session is the FT-710's driver.Session: one open, identity-verified,
// inventory-discovered connection. Safe for concurrent use — the
// underlying transport.Engine serialises every INDIVIDUAL exchange, and
// everything else here is immutable after Open.
//
// opMu (M3 Codex-review fix wave, Fix 2, adjudicated MEDIUM) serialises
// each LOGICAL operation's FULL multi-command sequence, not merely each
// individual wire exchange: ReadChannel is MR+MT (two separate
// transport.Engine.Do calls) and WriteChannel is MW+MT. transport.Engine's
// own mutex only guarantees at most one Do call in flight at a time — it
// says nothing about what runs in the GAP between two Do calls belonging
// to the SAME logical operation. Without opMu, a concurrent WriteChannel
// could land its entire MW+MT in the gap between a concurrent
// ReadChannel's MR and MT, producing a torn channel: frequency data from
// one moment, tag data from a later, different one. opMu closes that gap
// by holding it for the WHOLE of ReadChannel or WriteChannel, not just
// around each's individual Do calls.
type Session struct {
	eng    *transport.Engine
	id     driver.Identity
	caps   spec.Capabilities // effective; never mutated after Open
	region string

	opMu sync.Mutex
}

// Identity implements driver.Session.
func (s *Session) Identity() driver.Identity { return s.id }

// Capabilities implements driver.Session: the EFFECTIVE capability set
// (baseline + discovered read-only 60M/EMG banks), as a deep copy per
// call — see cloneCapabilities for why the copy is load-bearing.
func (s *Session) Capabilities() spec.Capabilities {
	return cloneCapabilities(s.caps)
}

// Region reports the regulatory region Open's discovery implied ("UK",
// "US", "no-60m" — HW-CONFIRMED 2026-07-13, a known real variant with no
// 5xx/EMG at all — or "unknown-N"/"unknown-16plus" for anything else —
// see deriveRegion). Callers record it in
// codeplug.RadioInfo.Region. It is an accessor on the concrete *Session,
// not part of driver.Session: region derivation is an FT-710 discovery
// quirk, not a seam-level contract.
func (s *Session) Region() string { return s.region }

// SessionDiagnostics is an alias for driver.SessionDiagnostics, moved
// there at task 37 (M9a-1, the radio-neutral core refactor — see that
// type's own doc comment in core/driver/optional.go): the shape itself
// was never FT-710-specific, only the driver producing it was, so it now
// lives on the neutral seam. Kept here, under this driver's own exported
// name, so no existing caller of ft710.SessionDiagnostics breaks.
// (*Session).Diagnostics returns this type; see driver.DiagnosticsReporter
// for the optional seam-level capability it satisfies with no signature
// change (M3 Codex-review fix wave, Fix 5, adjudicated MEDIUM, originally
// introduced this shape).
type SessionDiagnostics = driver.SessionDiagnostics

// Diagnostics reports this session's transport-level health counters as a
// point-in-time snapshot — the driver-layer surface for the engine's own
// accessors, which are otherwise unreachable (the *transport.Engine is
// buried, unexported, inside this Session). Safe for concurrent use, like
// the engine accessor it wraps. Like Region, it is an accessor on the
// concrete *Session rather than part of driver.Session: which diagnostics
// exist is a per-driver matter, not (yet) a seam-level contract.
func (s *Session) Diagnostics() SessionDiagnostics {
	n := s.eng.UnexpectedFrames()
	if n < 0 {
		// Unreachable (the engine only ever increments), but never let a
		// negative int64 wrap into an absurd uint64.
		n = 0
	}
	return SessionDiagnostics{UnexpectedFrames: uint64(n)}
}

// Close implements driver.Session. Idempotent: transport.Engine.Close
// already guarantees repeat calls return the same result.
func (s *Session) Close() error { return s.eng.Close() }

// ErrKindMismatch is the sentinel a caller should compare against (via
// errors.Is) when an MR answer's kind byte (P7) contradicts the slot it
// answered for — e.g. a memory-channel slot claiming to be a PMS entry.
// The error actually returned is a *KindMismatchError.
var ErrKindMismatch = errors.New("ft710: MR answer kind byte contradicts its slot's bank")

// KindMismatchError reports the slot and the offending kind byte. The
// full detail is carried here precisely so callers can log it: this
// driver has no logger of its own.
type KindMismatchError struct {
	// Slot is the canonical wire-form slot that was read.
	Slot string
	// Got is the P7 kind byte the answer carried.
	Got byte
	// Want lists every kind byte this slot's bank would have accepted —
	// the LENIENT, HW-CONFIRMED set acceptedKinds (read.go) returns for
	// this slot, not a single expected value: a PMS slot accepts
	// KindMemory OR KindPMS, a memory-bank slot accepts KindVFO,
	// KindMemory OR KindUnset.
	Want []byte
}

// Error implements the error interface. The single-mismatch shape
// ("carries kind '1', want '5'") this reported before M5b is preserved
// verbatim for the case a slot's acceptedKinds names exactly one byte;
// the live P1L failure this task fixed read EXACTLY this way (see
// acceptedKinds' doc comment, read.go). A slot whose bank now accepts
// more than one kind reports the full accepted set instead.
func (e *KindMismatchError) Error() string {
	if len(e.Want) == 1 {
		return fmt.Sprintf("ft710: MR answer for slot %q carries kind %q, want %q for this slot's bank", e.Slot, rune(e.Got), rune(e.Want[0]))
	}
	want := make([]string, len(e.Want))
	for i, k := range e.Want {
		want[i] = fmt.Sprintf("%q", rune(k))
	}
	return fmt.Sprintf("ft710: MR answer for slot %q carries kind %q, want one of {%s} for this slot's bank", e.Slot, rune(e.Got), strings.Join(want, ","))
}

// Unwrap lets errors.Is(err, ErrKindMismatch) match.
func (e *KindMismatchError) Unwrap() error { return ErrKindMismatch }

// ErrAnswerMismatch is the sentinel a caller should compare against (via
// errors.Is) when a slot-addressed answer (MR, MT) names a DIFFERENT
// slot than the one just requested. The transport's quarantine
// discipline makes this unlikely (a stale same-shape reply should have
// been drained), but the driver still refuses to map an answer onto the
// wrong slot. The error actually returned is an *AnswerMismatchError.
var ErrAnswerMismatch = errors.New("ft710: answer names a different slot than was requested")

// AnswerMismatchError reports the requested and the answered slot.
type AnswerMismatchError struct {
	// Requested is the slot the read asked for.
	Requested string
	// Answered is the slot the reply actually named.
	Answered string
}

// Error implements the error interface.
func (e *AnswerMismatchError) Error() string {
	return fmt.Sprintf("ft710: requested slot %q but the answer names slot %q — refusing to map a reply onto the wrong slot", e.Requested, e.Answered)
}

// Unwrap lets errors.Is(err, ErrAnswerMismatch) match.
func (e *AnswerMismatchError) Unwrap() error { return ErrAnswerMismatch }
