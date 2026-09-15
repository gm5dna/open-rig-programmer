// SPDX-License-Identifier: GPL-3.0-or-later

package bincat

import (
	"sync"

	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// DrainIdleGap and DrainCap are this family's DrainPolicy. This is a
// point-to-point RS-232 link with no bus and no transceive broadcasts —
// nothing unsolicited ever arrives — so, unlike core/civ's flood-shaped
// policy, a plain idle-gap wait with core/cat's own cap ratio is enough:
// there is no flood this family's own protocol can produce to defend
// against.
const (
	DrainIdleGap = transport.QuietPeriod
	DrainCap     = 2 * DrainIdleGap
)

// framing is the transport.Framing adapter for one Profile.
//
// THE ACCUMULATOR IS BUILT IN THE CONSTRUCTOR, not on the reader
// goroutine's NewAccumulator call — core/civ/framing.go's identical
// pattern, for the identical reason (Codex #3 of this milestone's plan
// adjudication): if NewAccumulator were what ASSIGNED f.acc, the window
// between NewEngineWith starting the reader goroutine and that goroutine
// reaching its first line would let the engine mutex holder call NoteSent
// — Init transmits before any frame has to have arrived — against a nil
// accumulator. Constructing it before the value is ever shared closes that
// window by construction.
type framing struct {
	p Profile

	// mu guards acc and handedOut. NoteSent is called only under the
	// engine mutex; the accumulator's Push is called only by the reader
	// goroutine; an implementation that shares state between the two
	// needs its own lock (transport.Framing's own contract).
	mu        sync.Mutex
	acc       *accumulator
	handedOut bool
}

var _ transport.Framing = (*framing)(nil)

// NewFraming returns the transport.Framing for p, ready to hand to
// transport.NewEngineWith.
//
// AN UNCONFIGURED PROFILE IS REFUSED — core/civ's NewFraming applies the
// identical guard to a zero civ.Profile, for the identical reason: a zero
// Profile is constructible by anyone, and the Framing built from one would
// be a non-nil interface value whose Allow admits nothing while claiming
// to speak for a radio.
//
// ONE ENGINE PER NewFraming VALUE — see NewAccumulator.
func NewFraming(p Profile) (transport.Framing, error) {
	if !p.Configured() {
		return nil, invalidProfile("NewFraming requires a configured profile: a zero Profile describes no radio, and the Framing built from one would install a gate that admits nothing while claiming to speak for a radio")
	}
	f := &framing{p: p}
	f.mu.Lock()
	f.acc = newAccumulator(f.p.MaxFrame())
	f.mu.Unlock()
	return f, nil
}

// NewAccumulator returns THIS adapter's one accumulator, wrapped so every
// Push takes the adapter's lock. A SECOND CALL PANICS — one Framing value
// reaching two Engines is a composition mistake with no honest recovery
// (core/civ/framing.go's NewAccumulator carries the full argument).
func (f *framing) NewAccumulator(max int) transport.Accumulator {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.handedOut {
		panic("bincat: " + f.p.Model + ": this Framing already gave its accumulator to an Engine — transport.Framing requires NewAccumulator be called exactly once per Engine, so build one bincat.NewFraming value per Engine rather than sharing one")
	}
	f.handedOut = true
	if max > 0 {
		f.acc.max = max
	}
	return lockedAccumulator{f: f}
}

// IsRejection is always false: this family has no NAK. The FT-1000MP
// manual states plainly that an out-of-range or illegal parameter makes
// the radio "do nothing" (spec.md §Context) — a rejected write is
// SILENCE, indistinguishable on the wire from a slow, healthy one, so this
// package makes no attempt to infer a refusal from an absence of reply.
func (f *framing) IsRejection(frame []byte) bool { return false }

// Allow is the profile's own outbound gate.
func (f *framing) Allow(frame []byte) bool { return f.p.AllowedCommand(frame) }

// InitSequence is EMPTY: nothing this family sends mutates a radio setting
// just to open a session (there is no session-level command at all in
// this protocol — every command is either a read or a channel mutation).
func (f *framing) InitSequence() []transport.Command { return nil }

// DrainPolicy is this family's — see DrainIdleGap and DrainCap.
func (f *framing) DrainPolicy() transport.DrainPolicy {
	return transport.DrainPolicy{IdleGap: DrainIdleGap, Cap: DrainCap}
}

// NoteSent records the frame the engine is about to write, so the
// accumulator knows how many bytes its reply — if it gets one at all —
// will be.
//
// THIS IS NOT ECHO SUPPRESSION, unlike core/civ's NoteSent. This family's
// link is point-to-point RS-232 with nothing else on it, so there is no
// echo to remove. What it solves instead is the piece the milestone plan
// names explicitly: this protocol has NO TERMINATOR, so an untermined,
// fixed-length reply's boundary is knowable from nowhere but the request
// that solicited it (Profile.ReplyLength) — recorded here, consumed by
// the accumulator's Push.
func (f *framing) NoteSent(frame []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n, ok := f.p.ReplyLength(frame)
	if !ok {
		return
	}
	f.acc.noteWant(n)
}

// maxWantQueue bounds how many outstanding expected-reply lengths one
// accumulator remembers. The engine holds ONE outstanding request at a
// time (core/transport's single-mutex design), so in normal operation
// this queue never holds more than one entry; the bound exists only so a
// caller that noted several transmissions in a row without a Push between
// them cannot grow it without limit.
const maxWantQueue = 8

// accumulator reassembles this family's UNTERMINATED, fixed-length
// replies: each one's length is knowable only from what NoteSent recorded
// for the frame that solicited it, never from a byte on the wire. It is
// not safe for concurrent use — the adapter's mutex is what makes that
// safe in practice.
type accumulator struct {
	buf  []byte
	max  int
	want []int
}

func newAccumulator(max int) *accumulator {
	return &accumulator{max: max}
}

// noteWant records that the next reply, once one starts arriving, is n
// bytes long. n <= 0 means the frame just sent gets no reply at all (every
// write opcode in this family) and is simply not queued: nothing this
// family's radios sends unsolicited, so there is no boundary to look for.
func (a *accumulator) noteWant(n int) {
	if n <= 0 {
		return
	}
	if len(a.want) >= maxWantQueue {
		a.want = append(a.want[:0], a.want[1:]...)
	}
	a.want = append(a.want, n)
}

// push appends chunk to the buffer and slices off one complete reply for
// every queued want length the buffer can now satisfy — handling a reply
// split across several Push calls exactly as it handles several replies
// coalesced into one.
//
// Each returned frame is an independent copy, never aliasing chunk or the
// accumulator's own retained buffer. If more than max bytes accumulate
// without completing the FRONT of the want queue, Push returns whatever
// complete frames it already found alongside a *transport.FrameTooLongError
// (errors.Is-compatible with transport.ErrFrameTooLong) and resets itself
// — core/cat's and core/civ's identical contamination contract, reused
// directly here since core/transport already exports the canonical type
// (no cross-manufacturer sentinel to mint or translate).
func (a *accumulator) push(chunk []byte) ([][]byte, error) {
	a.buf = append(a.buf, chunk...)

	var frames [][]byte
	for len(a.want) > 0 && len(a.buf) >= a.want[0] {
		n := a.want[0]
		frame := make([]byte, n)
		copy(frame, a.buf[:n])
		frames = append(frames, frame)

		rest := make([]byte, len(a.buf)-n)
		copy(rest, a.buf[n:])
		a.buf = rest
		a.want = a.want[1:]
	}

	if len(a.buf) > a.max {
		discarded := len(a.buf)
		a.buf = nil
		a.want = nil
		return frames, &transport.FrameTooLongError{DiscardedLen: discarded}
	}
	return frames, nil
}

// lockedAccumulator is the transport.Accumulator the engine's reader
// goroutine holds: a handle onto the adapter's ONE accumulator, taking the
// adapter's lock for the whole of every Push.
type lockedAccumulator struct{ f *framing }

func (a lockedAccumulator) Push(chunk []byte) ([][]byte, error) {
	a.f.mu.Lock()
	defer a.f.mu.Unlock()
	return a.f.acc.push(chunk)
}

// PendingWantCanceller is an OPTIONAL capability a core/driver caller may
// use immediately after a ClassRead's own NoteSent expectation is known
// to have gone PERMANENTLY unanswered — this family's own "silence means
// no" identity probe (spec.md §Identity probe: the boundary-channel
// probe that one of FT-890/FT-900 is documented to never answer).
//
// WHY THIS EXISTS: transport.Accumulator has no cancel primitive
// (core/transport/framing.go's own interface is Push alone), and
// noteWant's queue is a plain FIFO with no timeout of its own — a want
// nothing will ever satisfy stays queued forever, and the NEXT genuine
// exchange's real reply bytes complete THAT stale want first (accepting
// bytes 1..N of a later, unrelated answer as if they were the timed-out
// probe's), corrupting every read/write after it. transport.Engine's own
// "suspect"/CONTAMINATED drains (core/transport/doc.go) solve the
// analogous problem for RAW BYTE attribution; they do not, and cannot,
// reach into a Framing's own per-request length bookkeeping.
//
// Calling this when the corresponding exchange might STILL arrive is a
// driver bug: this is not a general-purpose accumulator reset, only the
// undo half of the one NoteSent call the caller has independently
// determined timed out with zero bytes received for it (errors.Is(err,
// transport.ErrTimeout) from the Do call that sent it).
type PendingWantCanceller interface {
	// CancelPendingWant removes the most recently noted expected-reply
	// length, if any. A no-op when none is pending.
	CancelPendingWant()
}

// CancelPendingWant implements PendingWantCanceller.
func (f *framing) CancelPendingWant() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.acc.cancelLastWant()
}

// cancelLastWant undoes the most recent noteWant call, if any is still
// queued unsatisfied.
func (a *accumulator) cancelLastWant() {
	if len(a.want) == 0 {
		return
	}
	a.want = a.want[:len(a.want)-1]
}

var _ PendingWantCanceller = (*framing)(nil)
