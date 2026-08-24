// SPDX-License-Identifier: GPL-3.0-or-later

package civ

import (
	"errors"
	"sync"

	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// This file is the CI-V half of spec D2's transport seam: the adapter that
// presents this package's codec to core/transport.Engine, the three answer
// MATCHERS a driver builds its CommandSpecs from, and the two spec helpers
// that keep the command CLASS explicit at every CI-V call site.
//
// IT LIVES HERE, AND core/civ IMPORTS core/transport, because spec D2 says
// so in as many words: "the CAT framing adapter lives in core/transport
// (which already imports core/cat for exactly these things — doc.go); the
// CI-V adapter lives in core/civ". The direction is the cycle-free one —
// core/transport knows nothing of core/civ — and no guard forbids it: the
// civ guards bar civ<->cat in both directions, app/ and cmd/ from reaching
// core/civ at all, and the bare core/driver seam package from importing
// it, and none of those is this import. core/civ/doc.go's "SEPARATE
// FOLLOW-UP TASK" is this task.
//
// THE CENTRAL INIT-UNDER-FLOOD RULE, stated once here and consumed by
// every CI-V driver (each radio's own probe/open task carries it):
// Engine.Init's INITIAL ErrDrainCapExceeded is NONFATAL for a CI-V driver
// — it is diagnosed and the session opens. Spec D2 is explicit that CI-V's
// "drain to quiet" is a bounded idle-gap wait that "cannot fail the open",
// and transceive is factory-ON on at least four models in this tier with
// no off-switch shipped, so a line that never goes quiet is a NORMAL
// operating state at open rather than a fault. Every LATER quarantine
// drain failure stays fail-closed exactly as the engine has it: Do's entry
// quarantine still refuses to transmit into a stream that might deliver an
// abandoned exchange's reply. The engine is UNCHANGED by this rule; it
// lives in the drivers, and it is written down here so five radio plans
// cannot each invent their own reading of it.

// DrainIdleGap and DrainCap are the CI-V DrainPolicy, and both are chosen
// against the transceive-flood reality rather than copied from CAT.
//
// IdleGap is transport.QuietPeriod. There is no CI-V reason to differ: it
// is a statement about how long a serial line must be silent before a
// previous exchange can be assumed finished, and that is a property of the
// link and the radio's turnaround, not of the framing.
//
// THE CAP IS WHERE CI-V DEPARTS, and it departs by being a HARD CEILING
// with room for exactly two postponements. transport.DrainPolicy's own
// default cap is 2*IdleGap — room for one genuine idle gap even if a
// single stale frame arrives partway through and postpones "quiet" once.
// One is too few here: a CI-V radio in transceive emits SEVERAL
// unsolicited frames for one operator action (a VFO knob turn broadcasts
// frequency, and a mode change broadcasts mode), so two postponements
// inside one drain is ordinary traffic rather than a flood. Three is not
// generous either — it is the last instant a drain may succeed at, honoured
// ahead of any queued event and inside the wait itself, so a stream that
// simply never pauses fails the drain at 600 ms instead of postponing
// itself forever. That failure is then read by the layer that knows what
// it means: nonfatal at Init (above), fail-closed everywhere after.
const (
	DrainIdleGap = transport.QuietPeriod
	DrainCap     = 3 * DrainIdleGap
)

// framing is the transport.Framing adapter for one Profile.
//
// IT OWNS A MUTEX, AND THE MUTEX IS THE WHOLE REASON THIS IS A STRUCT
// RATHER THAN A METHOD SET ON Profile. transport.Framing's contract says
// it plainly: NoteSent is called only under the engine mutex, the
// Accumulator is touched only by the reader goroutine, and "an
// implementation that shares state between the two needs its own lock".
// This one shares the most important state there is — FrameAccumulator's
// noted-sent list, written by NoteSent and consumed by Push — and
// FrameAccumulator's own doc comment says it is not safe for concurrent
// use. So every entry point that reaches the accumulator takes f.mu, and
// the accumulator is reached through no other path.
//
// THE ACCUMULATOR IS BUILT IN THE CONSTRUCTOR, not on the reader
// goroutine's NewAccumulator call. If NewAccumulator were the thing that
// ASSIGNED f.acc, then between NewEngineWith starting the reader goroutine
// and that goroutine reaching its first line there is a window in which
// the engine mutex holder can call NoteSent — Init transmits before any
// frame has to have arrived — and find a nil accumulator. Constructing it
// before the value is ever shared closes that window by construction
// rather than by a nil check that would have to guess what to do.
type framing struct {
	p Profile

	// mu guards acc and handedOut, and is the "adapter's own lock"
	// transport's framing.go requires. It is held across the whole of
	// Push, which is the only place a frame is assembled and the only
	// place a note is consumed.
	mu  sync.Mutex
	acc *FrameAccumulator
	// handedOut records that an Engine has already taken this adapter's
	// accumulator. See NewAccumulator: a second Engine is refused, not
	// served.
	handedOut bool
}

// Compile-time proof that the adapter really is the seam it claims: a
// complete transport.Framing, that civ.Command satisfies the neutral
// transport.Command interface by name rather than by the coincidence
// core/civ/command.go could only assert by shape, and that the adapter
// reports its own accumulator's counters.
var (
	_ transport.Framing        = (*framing)(nil)
	_ transport.Command        = Command{}
	_ AccumulatorStatsReporter = (*framing)(nil)
)

// AccumulatorStatsReporter is the OPTIONAL capability the value NewFraming
// returns implements: report the CI-V accumulator's own counters.
//
// IT EXISTS SO A DRIVER NEED NOT REACH FOR Engine.UnexpectedFrames. That
// counter answers a different question — "how many frames did the engine
// see that did not match the spec in force?" — and on a CI-V bus it is
// systematically the WRONG question, because the accumulator has already
// swallowed every transceive broadcast and every other station's traffic
// before the engine could count one. A driver's diagnostics want the
// numbers from THIS side of the filter: how much of the line was
// broadcast (Unexpected), how much was our own echo (Echoes), how much was
// noise (NoiseBytes). Reaching past the adapter for the engine's counter
// would report a healthy zero on a line saturated with transceive.
//
// A caller performs the two-result type assertion — the house's optional-
// capability pattern (core/driver/optional.go) — because NewFraming's
// declared result is transport.Framing, the neutral seam type, and must
// stay so.
type AccumulatorStatsReporter interface {
	// AccumulatorStats returns a snapshot of the counters the adapter's
	// own FrameAccumulator has accrued. Safe to call from any goroutine.
	AccumulatorStats() AccumulatorStats
}

// NewFraming returns the transport.Framing for p: the CI-V side of spec
// D2's seam, ready to hand to transport.NewEngineWith.
//
// AN UNCONFIGURED PROFILE IS REFUSED, and the refusal has to be here. A
// zero Profile is constructible by anyone, and the Framing built from one
// is a perfectly non-nil interface value carrying a perfectly non-nil
// Allow method — so NewEngineWith's own nil check cannot see it, and the
// engine would come up bound to a gate that speaks for no radio. core/cat
// has the same shape and the same answer (ErrUnconfiguredDialect at
// NewEngine); this is its CI-V twin, wrapping ErrInvalidProfile so a
// caller can tell a malformed model table from any other failure without
// matching message text.
//
// ONE ENGINE PER NewFraming VALUE. transport.Framing's contract says
// NewAccumulator is "called exactly once per Engine", and this adapter
// takes that literally rather than defensively: it holds ONE accumulator,
// built here (which is what closes the reader-goroutine init race), so a
// value handed to two Engines would have them share a reassembly buffer,
// share the noted-sent list echo removal depends on, and let a positive
// WithMaxFrame on the second move the first's frame bound. A driver that
// wants two Engines for one radio calls this twice. The second
// NewAccumulator call on one value is refused loudly — see NewAccumulator.
//
// The returned value additionally satisfies AccumulatorStatsReporter.
func NewFraming(p Profile) (transport.Framing, error) {
	if !p.Configured() {
		return nil, invalidProfile("NewFraming requires a configured profile: a zero Profile describes no radio, and the Framing built from one would install a gate that admits nothing while claiming to speak for a radio")
	}
	f := &framing{p: p}
	// Under the lock, before the value is shared with anyone: see the
	// type's doc comment on the NewAccumulator init race.
	f.mu.Lock()
	f.acc = p.NewAccumulator()
	f.mu.Unlock()
	return f, nil
}

// NewAccumulator returns THIS adapter's one accumulator, wrapped so every
// Push takes the adapter's lock.
//
// It does not CONSTRUCT one — the constructor did that (see the type's doc
// comment) — so the bound already in force is the PROFILE's own MaxFrame
// rather than DefaultMaxFrame. That distinction is load-bearing on a
// multi-length model: a profile whose MaxFrame is computed exactly for its
// own longest memory set would, under the package default, either buffer
// far past what its gate admits or (on a profile with a wider bound)
// discard its own answer as contamination.
//
// max > 0 — the engine's WithMaxFrame — overrides it, and does so by
// adjusting the bound on the accumulator already in hand rather than by
// replacing it, so no note and no buffered byte is lost.
//
// A SECOND CALL PANICS, and the loudness is the point. The engine calls
// this exactly once, from its reader goroutine, before that goroutine
// reads a byte — so a second call means one Framing value reached two
// Engines, which is a composition mistake with no honest recovery. The
// two would share a reassembly buffer (frames spliced across ports), share
// the noted-sent list (one port's echo suppressing the other's answer),
// and the later WithMaxFrame would silently re-bound the earlier Engine.
// The adapter's mutex prevents none of that: it removes data races, not
// cross-engine contamination.
//
// Panicking rather than returning a dead accumulator follows this
// package's own precedent for a programming error baked into a binary
// (MustNewProfile): the fault is deterministic, it surfaces the first time
// the path runs, and a caller cannot sensibly continue. Returning a
// permanently-failing accumulator instead would reach the engine as a raw
// I/O error, which handleReaderErr reads as a dead port — the misuse would
// be reported as a cable fault.
func (f *framing) NewAccumulator(max int) transport.Accumulator {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.handedOut {
		panic("civ: " + f.p.Model() + ": this Framing already gave its accumulator to an Engine — transport.Framing requires NewAccumulator be called exactly once per Engine, so build one civ.NewFraming value per Engine rather than sharing one")
	}
	f.handedOut = true
	f.reboundLocked(max)
	return lockedAccumulator{f: f}
}

// rebound adjusts this adapter's frame bound, taking the lock. It exists
// for NewAccumulator's max>0 path and for the test that pins it; a
// non-positive max leaves the profile's own bound in force.
func (f *framing) rebound(max int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reboundLocked(max)
}

// reboundLocked is rebound's body, for callers already holding f.mu.
//
// It sets the BOUND on the accumulator in hand and touches nothing else,
// which is the whole contract: notes and buffered bytes survive, because
// they belong to the same object.
func (f *framing) reboundLocked(max int) {
	if max > 0 {
		f.acc.max = max
	}
}

// IsRejection reports whether frame is a rejection OF OUR TRANSACTION: the
// CI-V FA NAK, from THIS profile's radio.
//
// THE SOURCE ADDRESS CHECK IS THE POINT. CI-V is a BUS. Another station's
// controller can be mid-exchange with another radio, and that radio's FA
// travels the same wire. The accumulator's `to` filter does not catch it —
// an FA addressed to a different controller is dropped there, but nothing
// stops a radio at another address from answering a broadcast, or a
// misconfigured second radio from sharing this profile's controller
// address. Without this check the engine would turn such a frame into
// ErrRejected and report, to the user, that THEIR radio refused THEIR
// command. It did not, and this program has no business saying it did.
func (f *framing) IsRejection(frame []byte) bool { return f.p.IsRejection(frame) }

// Allow is the profile's own outbound gate — an Engine built from this
// framing gates for the radio of the driver that built it and for no
// other. See Profile.AllowedCommand for what it refuses and why.
func (f *framing) Allow(frame []byte) bool { return f.p.AllowedCommand(frame) }

// InitSequence is EMPTY, and that is a safety property rather than an
// omission (spec D2, adjudication 3): a CI-V session opens without writing
// one byte to the radio. Transceive broadcasts are excluded STRUCTURALLY,
// by the accumulator's address filter, so this tier never quietens a bus
// by changing somebody's radio settings. Nothing outside the consent
// regime, no transceive write, no pre-identity mutation.
func (f *framing) InitSequence() []transport.Command { return nil }

// DrainPolicy is CI-V's — see DrainIdleGap and DrainCap for why the cap
// departs from transport's default and why it is a hard ceiling.
func (f *framing) DrainPolicy() transport.DrainPolicy {
	return transport.DrainPolicy{IdleGap: DrainIdleGap, Cap: DrainCap}
}

// NoteSent records a frame the engine is about to write, so the
// accumulator can recognise its echo — on a REMOTE bus and through many
// USB adapters, a frame this program writes comes straight back.
//
// It takes the adapter's lock because the note it appends is read by the
// reader goroutine inside Push. It COPIES what it needs and retains
// nothing (FrameAccumulator.NoteSent), which the seam's contract requires
// and safety obligation 1 depends on: the slice it is handed is the very
// one the gate then judges and the port then writes.
//
// SUPPRESSION IS BY RECORDED BYTES, never by position or by count — see
// FrameAccumulator.takeEcho. A position rule on a shared bus discards
// whatever arrives first, which is as likely to be the radio's real answer
// as the echo it meant to drop.
func (f *framing) NoteSent(frame []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.acc.NoteSent(frame)
}

// AccumulatorStats implements AccumulatorStatsReporter.
func (f *framing) AccumulatorStats() AccumulatorStats {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.acc.Stats()
}

// lockedAccumulator is the transport.Accumulator the engine's reader
// goroutine holds: a handle onto the adapter's ONE accumulator, taking the
// adapter's lock for the whole of every Push.
//
// A VALUE, not a second piece of state: it carries only the adapter
// pointer, so there is exactly one accumulator, one lock and one
// noted-sent list however many times NewAccumulator is called.
type lockedAccumulator struct{ f *framing }

// Push assembles frames under the adapter's lock and TRANSLATES this
// package's oversize error onto transport's.
//
// THE TRANSLATION IS NOT COSMETIC. Engine.handleReaderErr distinguishes
// exactly two outcomes by TYPE: an error that errors.As-matches
// *transport.FrameTooLongError marks the stream CONTAMINATED and leaves
// the port open for a DrainToQuiet to recover; anything else is taken for
// a dead port and CLOSES it. core/civ mints its own *FrameTooLongError
// (errors.go says why: it must never import core/cat, where transport's
// canonical value lives), and that type is NOT transport's. Handed over
// untranslated, a CI-V line that merely went noisy would tear the session
// down as an I/O failure instead of entering the recoverable state built
// for precisely this condition.
func (a lockedAccumulator) Push(chunk []byte) ([][]byte, error) {
	a.f.mu.Lock()
	defer a.f.mu.Unlock()
	frames, err := a.f.acc.Push(chunk)
	return frames, translateAccumulatorErr(err)
}

// translateAccumulatorErr maps core/civ's oversize error onto
// core/transport's, preserving DiscardedLen. Any other error passes
// through unchanged — the accumulator raises none today, and inventing a
// contamination verdict for a future one would be exactly the wrong
// direction to guess in.
func translateAccumulatorErr(err error) error {
	if err == nil {
		return nil
	}
	var tooLong *FrameTooLongError
	if errors.As(err, &tooLong) {
		return &transport.FrameTooLongError{DiscardedLen: tooLong.DiscardedLen}
	}
	return err
}

// CIVReadSpec builds the CommandSpec for one CI-V READ: an answer matched
// by match — built by one of Profile's own matcher constructors, never by
// the caller — and retryReads ADDITIONAL attempts after a timeout.
//
// Retrying a read is safe (safety obligation 2): it is idempotent, and a
// CI-V read of a memory channel changes nothing.
//
// It is the sanctioned way for a CI-V driver to build a read spec, and it
// exists for CATReadSpec's reason: D2 retired the implicit keying under
// which an unfilled spec silently became a fire-and-forget write, so the
// class is stated on every spec and no wrapper may rewrite one later.
func CIVReadSpec(match func(frame []byte) bool, retryReads int) transport.CommandSpec {
	return transport.CommandSpec{
		Class:      transport.ClassRead,
		Match:      match,
		RetryReads: retryReads,
	}
}

// CIVWriteWithAckSpec builds the CommandSpec for one CI-V WRITE: the
// memory set, whose acknowledgement is the radio's six-byte FB.
//
// ClassWriteWithAck AND RetryReads ZERO, both stated here rather than left
// to a caller. CATWriteSpec is NOT the mirror of this function and must
// not be reached for by analogy: CAT has no acknowledged write form at all
// — a Yaesu Set gets silence, and CATWriteSpec's whole content is the
// claim that silence means it worked. CI-V says FB, so a CI-V memory set
// that returned before its acknowledgement arrived would be reporting a
// success the radio never gave. RetryReads is zero because an acknowledged
// write is still a WRITE: a timeout is never resolved by resending one
// (safety obligation 2), and Engine.Do refuses a non-zero value on this
// class with ErrInvalidSpec before writing anything.
//
// match must be the profile's own six-byte address-checked ack matcher —
// Profile.AcknowledgementMatcher.
func CIVWriteWithAckSpec(match func(frame []byte) bool) transport.CommandSpec {
	return transport.CommandSpec{
		Class:      transport.ClassWriteWithAck,
		Match:      match,
		RetryReads: 0,
	}
}

// IsRejection reports whether frame is a rejection from THIS profile's
// radio: the package-level IsRejection's grammar check, plus the source
// address.
//
// THE PAIR IS DELIBERATE, and the division between them is the package's
// standing one. The package-level function knows the CI-V GRAMMAR — an FA
// body, exactly six bytes — which is the same on every Icom radio ever
// built and which no profile may relax. This method knows WHOSE refusal it
// is, which no package-level function can answer at all. The framing
// adapter calls THIS one, because "the radio refused your command" is an
// attribution and an attribution needs an address.
func (p Profile) IsRejection(frame []byte) bool {
	return IsRejection(frame) && frame[3] == p.radioAddr && frame[2] == p.controllerAddr
}

// IsAcknowledgement reports whether frame is an acknowledgement from THIS
// profile's radio, on IsRejection's terms exactly: the exact six-byte FB
// frame, addressed to this controller and from this radio.
//
// EXACT LENGTH, from the package-level function it delegates to. An
// FB-shaped frame carrying data is not a documented form, and treating one
// as an acknowledgement would report a write as accepted on the strength
// of a frame nobody can account for.
func (p Profile) IsAcknowledgement(frame []byte) bool {
	return IsAcknowledgement(frame) && frame[3] == p.radioAddr && frame[2] == p.controllerAddr
}

// TransceiverIDAnswerMatcher returns the Match for the `19 00` answer —
// the probe's first exchange (spec D3.2).
//
// IT MATCHES THE ENVELOPE AND NEVER THE VALUE. The reply's data byte is
// undocumented on all six models in this tier (spec D5 entry 7), so what
// the probe requires is that an ADDRESS-MATCHED reply arrived at all; the
// value is recorded as a diagnostic by ParseTransceiverID and compared
// against nothing. A matcher that checked it would refuse every real radio
// this tier has never seen — which is all of them.
//
// THE MATCHER COMES FROM THE CODEC, not from the driver. That is
// adjudication (a) of the Wave-2.5 review, and it is the same rule
// core/cat's PrefixLenMatcher embodies: the rule for recognising an answer
// is protocol knowledge, and a driver rebuilding one by hand is a driver
// that can get the address direction backwards.
func (p Profile) TransceiverIDAnswerMatcher() func(frame []byte) bool {
	return func(frame []byte) bool {
		_, err := p.answerBody(frame, CmdTransceiverID, SubTransceiverID)
		return err == nil
	}
}

// MemoryAnswerMatcher returns the Match for a `1A 00 <address> <record>`
// memory answer from this radio: addressed to this controller, from this
// radio, carrying 1A 00, and long enough to hold this profile's own
// address field.
//
// IT DOES NOT MATCH THE CHANNEL, and that is a decision rather than an
// oversight. Engine.Do holds ONE outstanding request at a time and the
// accumulator has already dropped every frame not addressed to this
// controller, so the only 1A 00 answer that can reach this matcher is the
// answer to the read in flight. Requiring the address field to equal the
// one we sent would add nothing against a bus hazard — an answer addressed
// to us from our radio is ours — while turning any difference in address
// ENCODING into a silent timeout on a tier whose address encoding is
// ASSUMED throughout (doc.go's register). The address that comes back is
// still decoded, validated against this profile's channel space and
// returned to the caller by ParseMemoryAnswer; a driver that reads one
// channel and is answered about another finds out there, with a diagnostic,
// rather than here, with silence.
func (p Profile) MemoryAnswerMatcher() func(frame []byte) bool {
	want := p.addressForm.addressBytes()
	return func(frame []byte) bool {
		body, err := p.answerBody(frame, CmdMemory, SubMemoryContents)
		return err == nil && len(body) >= want
	}
}

// AcknowledgementMatcher returns the Match a ClassWriteWithAck memory set
// waits on: the EXACT six-byte FB frame, address-checked in both
// directions.
//
// EXACT, because the alternative is a write reported as accepted on the
// strength of a frame that is merely FB-SHAPED — and because an FA from
// the same radio must fall through to the engine's own rejection path
// (IsRejection) rather than being mistaken for either an answer or
// unexpected traffic. Address-checked, for IsRejection's bus reason: an FB
// from another radio acknowledges another controller's write, and treating
// it as ours would report a memory set as landed when the radio never saw
// it.
func (p Profile) AcknowledgementMatcher() func(frame []byte) bool {
	return p.IsAcknowledgement
}
