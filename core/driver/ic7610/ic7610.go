// SPDX-License-Identifier: GPL-3.0-or-later

package ic7610

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic7610 "github.com/gm5dna/open-rig-programmer/core/civ/ic7610"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// probeSlotCount is how many memory channels Open's occupied-slot search
// reads before giving up and opening UNFINGERPRINTED.
//
// BOUNDED ON PURPOSE. The fingerprint's job is to confirm a record length,
// which one occupied channel settles; walking all 99 to find a radio's
// only populated memory would put ninety-nine exchanges into every open
// for no further evidence. Ten is enough to clear a radio whose first few
// memories happen to be empty and short enough that an entirely empty
// radio opens promptly — on address evidence alone, which spec D3.2
// explicitly allows.
const probeSlotCount = 10

// New returns the IC-7610 driver built with profile.
//
// NO MODEL ENUM: this family has one member (matrix §4), so which radio a
// driver is for is fixed by the package rather than by a value a caller
// could get wrong.
func New(profile Profile, opts ...Option) driver.Driver {
	d := &ic7610Driver{profile: profile}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Option configures a driver at construction.
type Option func(*ic7610Driver)

// WithTransportLogger threads l into every session's transport.Engine at
// Open time.
//
// It appends to the driver's OWN []transport.Option, built at
// construction. A driver Option is not a transport.Option and the two must
// not be conflated: the engine is constructed inside Open, where the
// driver's opts are long out of scope, so the translated slice has to be
// carried on the driver value.
func WithTransportLogger(l transport.Logger) Option {
	return func(d *ic7610Driver) { d.transportOpts = append(d.transportOpts, transport.WithLogger(l)) }
}

// WithConsentedUnverifiedWrites records the user's consent to writes this
// project has never verified against an IC-7610.
//
// It changes the SESSION's effective capabilities and never the driver's
// static ones: internal/wiring reads the static set to decide whether to
// ASK for consent, and a driver whose static set already claimed consent
// would never be asked about. FieldErase is structurally out of reach of
// the transform (spec D4), and an unrecognised Profile is not transformed
// at all, so no value a caller can pass produces a writable session.
func WithConsentedUnverifiedWrites() Option {
	return func(d *ic7610Driver) { d.consented = true }
}

// ic7610Driver implements driver.Driver for the Icom IC-7610.
type ic7610Driver struct {
	profile   Profile
	consented bool
	// transportOpts are this driver's own options translated into the
	// transport's, ready for the transport.NewEngineWith call inside Open.
	transportOpts []transport.Option
}

// Model implements driver.Driver. It must equal Capabilities().Model and
// the Wave-4 registry key.
func (d *ic7610Driver) Model() string { return "IC-7610" }

// Capabilities implements driver.Driver: the STATIC baseline for this
// driver's profile, before any radio has been probed.
func (d *ic7610Driver) Capabilities() spec.Capabilities {
	switch d.profile {
	case Simulated:
		return capabilitiesSimulated()
	case RealHardware:
		// writeTrialsComplete is false, so there is no hardware-verified
		// profile for this arm to select and a real-hardware session gets
		// the all-Unverified fail-safe. The constant is not READ here,
		// deliberately: see its doc comment for what a flip must change,
		// and why a constant that was load-bearing on its own would let a
		// one-character edit unlock a write.
		return capabilitiesUnverified()
	default:
		// Any unrecognised Profile fails safe through its OWN explicit
		// arm rather than by sharing RealHardware's. The two return the
		// same value today, and a reader must be able to see that the
		// fail-safe is a decision rather than a coincidence of the flip's
		// state.
		return capabilitiesUnverified()
	}
}

// OpenReport is what Open observed while probing, kept on the ic7610
// Session rather than on the neutral seam: driver.SessionDiagnostics
// carries ONE aggregate counter (core/driver/optional.go) and cannot carry
// any of this, and widening the neutral seam is a tier-shared change five
// worktrees would want.
type OpenReport struct {
	// IDToken is what the radio answered to 19 00, recorded and NEVER
	// matched (D5 entry 7, lift R7).
	IDToken []byte
	// SlotsTried is how many channels the occupied-slot search read
	// before it stopped.
	SlotsTried int
	// Fingerprinted is false when every probed slot was rejected — an
	// empty radio, opened on address evidence alone (spec D3.2).
	Fingerprinted bool
	// RecordLength is the record-only length the fingerprint confirmed,
	// or 0 when Fingerprinted is false.
	RecordLength int
	// InitDrainCapExceeded records that Engine.Init hit its absolute
	// drain cap and that Open continued anyway — the NONFATAL half of
	// R9-SPLIT. It can only be true under a CONTROLLER-ADDRESSED flood:
	// a to=00 broadcast flood never reaches the engine at all.
	InitDrainCapExceeded bool
	// WireAtOpen is the adapter's own counter snapshot taken when Open
	// returned, so a broadcast-saturated line is visible even though the
	// engine saw nothing.
	WireAtOpen civ.AccumulatorStats
}

// RecordLengthMismatchError reports that a memory answer carried a record
// at a length this profile does not declare — which is the probe's
// CONTINUOUS length fingerprint failing (spec D3.2).
//
// IT NAMES NO FOUND MODEL, and driver.WrongRadioError is deliberately not
// used for that reason: that type's whole shape is a pair of CAT IDs, and
// filling it with lengths would put a made-up identity in a field callers
// render as one. Cross-model record-length distinctness is a TIER-LEVEL
// Wave-4 check and this package holds no table of other radios' lengths,
// so the honest refusal says what was measured, what was expected, and
// that the expectation is itself ASSUMED.
type RecordLengthMismatchError struct {
	// Got is the record-only length the radio's answer carried.
	Got int
	// Want is the length this profile declares.
	Want int
	// Slot is the channel the answer spoke for.
	Slot civ.ChannelAddress
}

func (e *RecordLengthMismatchError) Error() string {
	return fmt.Sprintf(
		"ic7610: %s answered a %d-byte memory record, want %d — the expected length is itself an ASSUMED derivation from one document (D5 entry 6, matrix lift R6), and this refusal names no other model because cross-model record-length distinctness is a Wave-4 tier check",
		e.Slot, e.Got, e.Want)
}

// Unwrap lets errors.Is(err, driver.ErrWrongRadio) match: whatever is on
// this port, its memory records are not the shape this driver writes.
func (e *RecordLengthMismatchError) Unwrap() error { return driver.ErrWrongRadio }

// ErrAnswerMismatch is the sentinel for tier ruling T2: a memory answer
// whose decoded channel address is not the one that was asked for.
var ErrAnswerMismatch = errors.New("ic7610: a memory answer named a channel other than the one requested")

// AnswerMismatchError reports T2's failure, naming both channels.
//
// IT EXISTS BECAUSE THE MATCHER CANNOT CATCH THIS. The landed
// civ.Profile.MemoryAnswerMatcher is deliberately ENVELOPE-ONLY — it
// checks `to`, `from`, `cn` and `sc` and nothing else — so an answer for
// channel 7 satisfies the spec for a read of channel 3 perfectly well.
// The address inside the answer is therefore the DRIVER's to check, and it
// is checked BEFORE ANY USE of the answer: before empty recognition,
// before any caching, before record mapping, before the E6 template check
// and before a write merge. A record silently mis-attributed to the wrong
// channel is the corruption this whole project refuses.
//
// core/driver/ftdx101's AnswerMismatchError is the precedent.
type AnswerMismatchError struct {
	// Want is the channel the driver asked about.
	Want civ.ChannelAddress
	// Got is the channel the answer actually named.
	Got civ.ChannelAddress
}

func (e *AnswerMismatchError) Error() string {
	return fmt.Sprintf("ic7610: asked about %s and was answered about %s — the memory-answer matcher is envelope-only, so this is the driver's check (tier ruling T2)", e.Want, e.Got)
}

// Unwrap lets errors.Is(err, ErrAnswerMismatch) match.
func (e *AnswerMismatchError) Unwrap() error { return ErrAnswerMismatch }

// Open implements driver.Driver.
//
// The choreography, and nothing else on the wire: build the CI-V framing,
// build the engine, Init (which for CI-V is a DRAIN ALONE — E1's
// InitSequence is EMPTY, so no radio mutation ever happens here), probe
// 19 00 for an address-matched reply, then read up to probeSlotCount
// memory channels for a record whose length confirms the fingerprint.
//
// Open takes ownership of port on BOTH outcomes: the Session's Close
// releases it on success, and Open itself closes it before returning an
// error.
func (d *ic7610Driver) Open(ctx context.Context, port transport.Port, id driver.Identity) (driver.Session, error) {
	// ONE ENGINE PER NewFraming VALUE (enablers fix wave X1): the adapter
	// refuses a second NewAccumulator call loudly, so the framing is built
	// HERE, per Open, and never cached on the driver or shared across
	// sessions.
	framing, err := civ.NewFraming(civic7610.Profile())
	if err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("ic7610: framing: %w", err)
	}
	// THE DIAGNOSTICS CARRIER, and why the assertion is two-result:
	// NewFraming's declared result is transport.Framing, the neutral seam
	// type, so the adapter's own counters are reachable only through the
	// house optional-capability pattern. Without them a driver would have
	// to reach for Engine.UnexpectedFrames, which on a CI-V bus answers a
	// different question and reports a healthy zero on a line saturated
	// with transceive — the accumulator swallowed those frames first.
	stats, ok := framing.(civ.AccumulatorStatsReporter)
	if !ok {
		_ = port.Close()
		return nil, fmt.Errorf("ic7610: the CI-V framing does not report accumulator stats — this driver's diagnostics require civ.AccumulatorStatsReporter")
	}
	eng, err := transport.NewEngineWith(port, framing, d.transportOpts...)
	if err != nil {
		// NewEngineWith has not taken the port on this path (it refuses
		// before touching it), so closing it here is Open's own ownership
		// obligation, not a double close.
		_ = port.Close()
		return nil, fmt.Errorf("ic7610: Open: %w", err)
	}
	sess, err := d.open(ctx, eng, stats, id)
	if err != nil {
		// eng owns port from here on, so closing eng is what releases it.
		_ = eng.Close()
		return nil, err
	}
	return sess, nil
}

// open is Open's body, factored so the error path closes eng in exactly
// one place.
func (d *ic7610Driver) open(ctx context.Context, eng *transport.Engine, stats civ.AccumulatorStatsReporter, id driver.Identity) (*Session, error) {
	p := civic7610.Profile()
	var report OpenReport

	// R9-SPLIT, THE NONFATAL HALF. Init is a drain alone on CI-V, and the
	// drain is bounded by an ABSOLUTE cap precisely so it cannot fail the
	// open: a line saturated with traffic addressed to this controller
	// never yields the idle gap, and refusing to open on that basis would
	// make a busy bus indistinguishable from a broken radio. So the
	// INITIAL failure is recorded and stepped over.
	//
	// EVERY LATER DRAIN FAILURE IS FATAL, and the asymmetry is the point:
	// once the session is exchanging frames, a drain that cannot find
	// quiet means this program can no longer tell its own answers from
	// somebody else's, and continuing would be guessing. Nothing below
	// tolerates a drain-cap error, and TestOpen_LaterQuarantineFailureFailsClosed
	// holds that down.
	//
	// A BROADCAST FLOOD NEVER GETS HERE AT ALL. Frames addressed to 0x00
	// are counted and dropped by the accumulator before any engine event,
	// so they never re-arm the drain's timer; InitDrainCapExceeded can
	// only ever be true under a CONTROLLER-ADDRESSED flood.
	if err := eng.Init(ctx); err != nil {
		if !errors.Is(err, transport.ErrDrainCapExceeded) {
			return nil, fmt.Errorf("ic7610: Open: %w", err)
		}
		report.InitDrainCapExceeded = true
	}

	// THE IDENTITY PROBE (spec D3.2). What identifies the radio is that an
	// ADDRESS-MATCHED 19 00 reply arrived at all: the reply VALUE is
	// undocumented on every model in this tier (D5 entry 7, matrix lift
	// R7), so it is recorded as a diagnostic and compared against nothing.
	//
	// The address check belongs to the CODEC — the matcher comes from
	// Profile.TransceiverIDAnswerMatcher and checks both the `to` and the
	// `from` byte — and is never a rule written here (adjudication R1).
	// One retry: an identity read is idempotent, and an open should
	// survive a single swallowed reply.
	idCmd, err := p.BuildTransceiverIDRead()
	if err != nil {
		return nil, fmt.Errorf("ic7610: Open: building the 19 00 read: %w", err)
	}
	frame, err := eng.Do(ctx, idCmd, civ.CIVReadSpec(p.TransceiverIDAnswerMatcher(), 1))
	if err != nil {
		// A RADIO AT ANOTHER CI-V ADDRESS LANDS HERE, as a timeout and
		// not as a wrong-radio refusal — nothing was heard from, so
		// nothing can be attributed (spec D3.3). A radio at a CI-V baud
		// other than the assumed 19200 lands here identically, which is
		// why a wrong default-baud guess costs a clean timeout and never
		// a wrong byte (OQ2).
		return nil, fmt.Errorf("ic7610: Open: 19 00 identity probe: %w", err)
	}
	token, err := p.ParseTransceiverID(frame)
	if err != nil {
		return nil, fmt.Errorf("ic7610: Open: 19 00 identity probe: %w", err)
	}
	// hex.DecodeString rather than re-slicing the frame: the token is the
	// codec's own rendering of the answer's data bytes, and taking them
	// back from it keeps every piece of frame geometry inside core/civ.
	// An odd-length token cannot arise (the codec renders whole bytes),
	// and if one ever did the raw form is simply not recorded.
	if raw, derr := hex.DecodeString(token); derr == nil {
		report.IDToken = raw
	}
	// The static CATID is the address alone (spec D3.2); the session's is
	// that address followed by what this radio actually answered.
	id.CATID = fmt.Sprintf("%02x%s", p.RadioAddress(), token)

	// THE OCCUPIED-SLOT SEARCH, and the fingerprint it confirms. A
	// rejection means "empty, keep looking" (tier ruling T4); a record
	// confirms the length; any other error aborts the open.
	for ch := 1; ch <= probeSlotCount; ch++ {
		report.SlotsTried = ch
		raw, empty, err := probeSlot(ctx, eng, p, civ.ChannelAddress{Channel: ch})
		if err != nil {
			return nil, err
		}
		if empty {
			continue
		}
		report.Fingerprinted = true
		report.RecordLength = len(raw)
		break
	}
	// AN EMPTY RADIO OPENS ANYWAY, on address evidence alone (spec D3.2,
	// D5 entry 2(a), matrix lift R2a). Refusing here would make a radio
	// whose memories are all empty unprogrammable by this programme, which
	// is precisely the radio a user most wants to programme.

	report.WireAtOpen = stats.AccumulatorStats()

	return &Session{
		eng:    eng,
		stats:  stats,
		id:     id,
		caps:   d.sessionCapabilities(),
		report: report,
	}, nil
}

// probeSlot performs ONE 1A 00 read for the occupied-slot search and
// reports the raw record, or that the slot is empty.
//
// T4 — FA IS AN ERROR, NOT A FRAME. Engine.Do consumes the FA and returns
// transport.ErrRejected with NO frame, so the empty branch keys on
// errors.Is(err, transport.ErrRejected) and never on "an FA arrived".
// Nothing in this driver calls civ.IsRejection; that stays the framing's
// internal concern.
//
// T2 — ANSWER-ADDRESS EQUALITY. The landed MemoryAnswerMatcher is
// deliberately envelope-only (to/from/cn/sc), so the DRIVER compares the
// decoded ChannelAddress against the one it asked for before making any
// use of the answer. During the probe that means before the length is
// taken as this radio's fingerprint.
func probeSlot(ctx context.Context, eng *transport.Engine, p civ.Profile, a civ.ChannelAddress) ([]byte, bool, error) {
	cmd, err := p.BuildMemoryRead(a)
	if err != nil {
		return nil, false, fmt.Errorf("ic7610: Open: building the 1A 00 read for %s: %w", a, err)
	}
	frame, err := eng.Do(ctx, cmd, civ.CIVReadSpec(p.MemoryAnswerMatcher(), 1))
	if errors.Is(err, transport.ErrRejected) {
		return nil, true, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("ic7610: Open: probing %s: %w", a, err)
	}
	got, raw, err := p.MemoryAnswerRecord(frame)
	if err != nil {
		var lengthErr *civ.RecordLengthError
		if errors.As(err, &lengthErr) {
			return nil, false, &RecordLengthMismatchError{Got: lengthErr.Got, Want: civic7610.RecordOnlyLength, Slot: a}
		}
		return nil, false, fmt.Errorf("ic7610: Open: probing %s: %w", a, err)
	}
	if got != a {
		return nil, false, &AnswerMismatchError{Want: a, Got: got}
	}
	return raw, false, nil
}

// sessionCapabilities is the ONE place a session's effective capability
// set is assembled: this driver's static set, then — only when the driver
// was built with WithConsentedUnverifiedWrites AND its profile is one of
// the declared constants — the consent transform.
//
// An unrecognised profile stays untransformed even with the option, so the
// fail-safe direction survives consent. Applying the transform here,
// before the Session exists, keeps the set WriteChannel enforces and the
// set Capabilities() hands out the same value.
func (d *ic7610Driver) sessionCapabilities() spec.Capabilities {
	caps := d.Capabilities()
	if !d.consented {
		return caps
	}
	switch d.profile {
	case RealHardware, Simulated:
		return spec.ConsentUnverifiedWrites(caps)
	default:
		return caps
	}
}

// Session is one open, probed connection to an IC-7610.
type Session struct {
	eng *transport.Engine
	// stats is the framing value Open passed to transport.NewEngineWith,
	// RETAINED for the life of the session — the DIAGNOSTICS CARRIER
	// ruling. Reaching for Engine.UnexpectedFrames instead is forbidden:
	// it would report a healthy zero on a line saturated with transceive.
	stats  civ.AccumulatorStatsReporter
	id     driver.Identity
	caps   spec.Capabilities
	report OpenReport
	// answerMismatches counts memory answers whose decoded channel address
	// was not the one requested (tier ruling T2).
	//
	// ATOMIC because it is the one piece of MUTABLE state on a Session,
	// and driver.Session's contract says implementations must be safe for
	// concurrent use. Every other field here is written once by Open and
	// only read afterwards; the transport engine serialises the exchanges
	// themselves, but nothing serialises a caller reading the diagnostic
	// while another goroutine drives a read.
	answerMismatches atomic.Uint64
}

// Identity implements driver.Session.
func (s *Session) Identity() driver.Identity { return s.id }

// Capabilities implements driver.Session: this session's EFFECTIVE set, as
// a deep copy per call.
//
// THE COPY IS LOAD-BEARING. WriteChannel re-checks against s.caps, the
// session's own value; a caller that could reach into what Capabilities
// handed out and flip a FieldSupport would otherwise be editing the write
// gate from outside it.
func (s *Session) Capabilities() spec.Capabilities { return cloneCapabilities(s.caps) }

// cloneCapabilities deep-copies a capability set: every slice freshly
// allocated, every bank re-copied through spec.Capabilities.Bank (which
// already returns fresh Slots and Fields), and the tone RANGE — a POINTER,
// so `out := caps` would have aliased it — copied as a struct.
//
// The ok result of Bank is discarded, and what makes that safe is that b
// came out of caps.Banks and Bank scans that same slice for b.ID, so the
// lookup cannot miss; the only way it could serve the wrong bank is a
// duplicate BankID, which spec.Capabilities.Validate refuses outright and
// TestBaseline_Validate runs over both profiles.
func cloneCapabilities(caps spec.Capabilities) spec.Capabilities {
	out := caps
	out.Banks = make([]spec.Bank, 0, len(caps.Banks))
	for _, b := range caps.Banks {
		cp, _ := caps.Bank(b.ID)
		out.Banks = append(out.Banks, cp)
	}
	out.Modes = append([]string(nil), caps.Modes...)
	out.CTCSSTones = append([]spec.Tone(nil), caps.CTCSSTones...)
	out.Bauds = append([]int(nil), caps.Bauds...)
	out.RequiredSlots = append([]string(nil), caps.RequiredSlots...)
	out.ShiftOptions = append([]spec.ShiftOption(nil), caps.ShiftOptions...)
	out.CTCSSStates = append([]spec.ToneState(nil), caps.CTCSSStates...)
	out.DuplexOptions = append([]spec.DuplexOption(nil), caps.DuplexOptions...)
	out.ToneModes = append([]spec.ToneMode(nil), caps.ToneModes...)
	out.DTCSPolarities = append([]string(nil), caps.DTCSPolarities...)
	out.DTCSCodes = append([]int(nil), caps.DTCSCodes...)
	out.Filters = append([]string(nil), caps.Filters...)
	if caps.CTCSSToneRange != nil {
		r := *caps.CTCSSToneRange
		out.CTCSSToneRange = &r
	}
	return out
}

// Close implements driver.Session. Idempotent, because Engine.Close is.
func (s *Session) Close() error { return s.eng.Close() }

// Fingerprint reports the record-only length Open confirmed, and whether
// it confirmed one at all.
//
// AN IC7610-PACKAGE ACCESSOR, NOT A NEUTRAL-SEAM ADDITION.
// driver.SessionDiagnostics carries only UnexpectedFrames, and widening
// the neutral seam to carry a per-tier fingerprint is a tier-shared change
// five worktrees would want a say in. A caller that needs this reaches for
// the concrete session, exactly as it would for any model-specific fact.
func (s *Session) Fingerprint() (recordLength int, confirmed bool) {
	return s.report.RecordLength, s.report.Fingerprinted
}

// OpenDiagnostics returns what Open observed while probing. See
// Fingerprint for why this is a package accessor.
func (s *Session) OpenDiagnostics() OpenReport { return s.report }

// WireStats exposes the adapter's full counter set, which the neutral
// SessionDiagnostics has no room for: echoes, noise bytes, truncations.
func (s *Session) WireStats() civ.AccumulatorStats { return s.stats.AccumulatorStats() }

// Diagnostics implements driver.DiagnosticsReporter by SUMMING the two
// sides of the filter: the engine's own unmatched-frame counter and the
// adapter's Unexpected count (the broadcasts and other stations' traffic
// the accumulator dropped before the engine could see them).
//
// NEITHER NUMBER ALONE IS THE TRUTH ABOUT THIS WIRE. The engine's counter
// answers "how many frames did the engine see that did not match the spec
// in force?", and on a CI-V bus the accumulator has already swallowed
// every transceive broadcast before the engine could count one.
func (s *Session) Diagnostics() driver.SessionDiagnostics {
	return driver.SessionDiagnostics{
		UnexpectedFrames: uint64(s.eng.UnexpectedFrames()) + uint64(s.stats.AccumulatorStats().Unexpected),
	}
}

// ReadChannel implements driver.Session; its body is in read.go, beside
// the slot map and the one read primitive it is made of.

// WriteChannel implements driver.Session; its body is in write.go,
// alongside the T5-ordered refusal ladder it is made of.

// Compile-time proof that this package really does implement the two
// neutral seams it claims.
var (
	_ driver.Driver              = (*ic7610Driver)(nil)
	_ driver.Session             = (*Session)(nil)
	_ driver.DiagnosticsReporter = (*Session)(nil)
)
