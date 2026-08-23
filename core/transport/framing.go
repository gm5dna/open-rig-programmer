// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import "time"

// Command is one outbound frame the Engine may transmit: bytes, plus a
// safe rendering for diagnostics. It is deliberately the NARROWEST view of
// a codec's command type — the engine needs to write it and to name it in
// an error, and nothing else.
//
// cat.Command satisfies it, as (from the Icom tier) does civ.Command. The
// interface is what makes core/transport's state machine neutral between
// the two: before D2 the engine's Do took a cat.Command outright, which
// bound the whole transaction engine to one radio family's codec.
//
// CONTRACT ON Bytes: every call must return a FRESH, independently owned
// slice. Safety obligation 1 has the engine call Bytes() exactly once per
// transmission attempt, hand THAT slice to the gate, and write THAT SAME
// slice — an implementation that returned an internal buffer would let a
// caller (or a later attempt) mutate the bytes between the check and the
// write. cat.Command.Bytes already copies on every call; any other
// implementation must too.
type Command interface {
	// Bytes returns a fresh copy of this command's wire bytes.
	Bytes() []byte
	// String renders the command for logs and errors. It must be safe to
	// interpolate into a diagnostic — %q-quoted or equivalent — since
	// frame content is radio-supplied, not trusted.
	String() string
}

// Accumulator reassembles whole protocol frames from arbitrary stream
// chunks — successive reads from a serial port, which may split one frame
// across reads, coalesce several into one read, or fragment in any other
// way — and enforces a maximum frame length so a noisy or wedged line
// cannot grow a buffer without bound.
//
// *cat.FrameAccumulator satisfies it. The engine's reader goroutine owns
// exactly one Accumulator and is the only goroutine that touches it, so an
// implementation need not be safe for concurrent use.
//
// CONTRACT: each returned frame must be an independent copy that never
// aliases chunk or the accumulator's own retained buffer — the engine
// hands frames on to answer matchers and to a caller. A frame exceeding
// the maximum must be reported as an error satisfying errors.Is(err,
// ErrFrameTooLong) (and, for the discarded-length diagnostic,
// errors.As(err, **FrameTooLongError)); the engine treats that as stream
// CONTAMINATION (see doc.go). As with io.Reader, frames found BEFORE a
// violation are returned alongside the error and must not be discarded.
type Accumulator interface {
	Push(chunk []byte) (frames [][]byte, err error)
}

// DrainPolicy is the framing's answer to "how long may the engine spend
// waiting for this line to go quiet, and when must it give up regardless?"
// Both halves are load-bearing, and the second is the one D2 added
// (starvation deadlines): a transceive flood — factory-ON on at least four
// Icom models, with no off-switch this tier ships — is a stream that never
// goes quiet, and an idle-gap wait alone would postpone itself forever
// against one.
type DrainPolicy struct {
	// IdleGap is how long the line must be silent — no frame, no
	// accumulator error — before the engine calls it drained. Any
	// activity re-arms it. <= 0 selects QuietPeriod.
	IdleGap time.Duration
	// Cap is the ABSOLUTE ceiling on one drain, measured from when that
	// drain started and honoured ahead of any queued event AND inside
	// the wait itself, so neither an arrival rate nor a well-timed
	// single frame can extend it. A drain still short of quiet when Cap
	// elapses fails with ErrDrainCapExceeded rather than continuing;
	// Cap is the LAST instant it can succeed at, not a floor it is
	// measured from. <= 0 selects 2*IdleGap — room for one genuine
	// IdleGap of silence even if a single stale frame arrives partway
	// through and postpones "quiet" once.
	Cap time.Duration
}

// withDefaults returns a copy of p with every non-positive field replaced
// by its documented default. Engine resolves the policy ONCE, at
// construction, and holds the resolved value: a framing that returned a
// different policy per call could otherwise widen its own deadlines
// mid-drain, which is exactly what an absolute cap must not permit.
func (p DrainPolicy) withDefaults() DrainPolicy {
	if p.IdleGap <= 0 {
		p.IdleGap = QuietPeriod
	}
	if p.Cap <= 0 {
		p.Cap = 2 * p.IdleGap
	}
	return p
}

// Framing is the protocol seam Engine is generalised over (D2): everything
// about a wire protocol the transaction engine has to touch, and nothing
// about what any particular radio permits.
//
// Engine keeps its own state machine on either side of this interface —
// single outstanding request, suspect/quarantine, unexpected-frame
// counting, safety obligation 1's write-what-was-checked rule. What the
// interface supplies is the six things that differ between a ';'-terminated
// Yaesu CAT stream and Icom's binary FE FE … FD one.
//
// A Framing value is fixed at construction and read without
// synchronisation from both the reader goroutine (NewAccumulator, at
// startup) and whichever call holds the engine mutex, so an implementation
// must be immutable after construction apart from whatever NoteSent
// records — and NoteSent's own state must be safe for the engine's use
// pattern (it is called only under the engine mutex, and read only by the
// Accumulator on the reader goroutine, so an implementation that shares
// state between the two needs its own lock).
type Framing interface {
	// NewAccumulator returns a fresh frame accumulator enforcing a
	// maximum frame length of max bytes; max <= 0 selects the framing's
	// own default. Called exactly once per Engine, before its reader
	// goroutine starts.
	NewAccumulator(max int) Accumulator

	// IsRejection reports whether frame is the protocol's NAK — CAT's
	// "?;", CI-V's FA. The engine turns one into ErrRejected: a
	// definitive answer, never retried.
	IsRejection(frame []byte) bool

	// Allow is the outbound write gate: the last defence before a
	// physical radio sees these bytes (safety obligation 1). The
	// AllowFunc contract is unchanged — see AllowFunc, including its
	// obligation not to mutate or retain the frame.
	Allow(frame []byte) bool

	// InitSequence is what Engine.Init transmits, in order, before
	// draining per DrainPolicy. CAT returns the dialect's own AI0;
	// frame; CI-V returns EMPTY — nothing this tier sends to an Icom
	// radio mutates it, so a session opens without touching its
	// settings (D2, adjudication 3). Each command is sent as a
	// ClassWrite: fire-and-forget, never retransmitted.
	InitSequence() []Command

	// DrainPolicy supplies the idle gap and the absolute cap for every
	// drain, purge, quarantine and answer wait the engine performs. It
	// is consulted ONCE, at construction.
	DrainPolicy() DrainPolicy

	// NoteSent reports a frame the engine is ABOUT to write — it is
	// called BEFORE the port write, so a framing whose accumulator
	// removes bus/USB echo (CI-V) has the frame recorded before its own
	// echo can possibly arrive.
	//
	// It is called before Allow, too: the engine gives the framing the
	// last touch of the slice and THEN gates it, so that what the gate
	// approves is byte-for-byte what goes out (safety obligation 1).
	// One consequence is visible here: a frame Allow then REFUSES has
	// already been noted, though nothing was written. An implementation
	// must therefore treat NoteSent as "the engine intends to send
	// this", not "this reached the wire" — for echo removal that is the
	// right reading anyway, since an echo that never arrives is
	// superseded by the next NoteSent.
	//
	// CONTRACT: an implementation MUST copy whatever it needs and MUST
	// NOT retain OR MUTATE the passed slice — the same obligation
	// AllowFunc carries, and for the same reason. The slice is the very
	// one safety obligation 1 then gates and writes; it is live and
	// writable, and a later attempt re-derives its own.
	NoteSent(frame []byte)
}
