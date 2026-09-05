// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import (
	"errors"
	"fmt"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// This file is the Kenwood half of the transport seam: the adapter that
// presents this package's codec to core/transport.Engine.
//
// IT LIVES HERE, AND core/kw IMPORTS core/transport, on the core/civ side
// of the import direction — the cycle-free one, and the one core/civ's own
// framing.go already takes. core/transport knows nothing of core/kw.

// Book names which of the two PC-command documents a session speaks to.
//
// IT IS NOT DECORATION AND IT HAS NO DEFAULT. The two books give the SAME
// stream-health token DIFFERENT causes — "O;" is "A receive buffer overrun
// error occurred" on the 590 pair (590:113) and "Receive data was sent but
// processing was not completed" on the TS-480 (480:143-144), erratum E13 —
// so a diagnostic that names the wrong sentence names the wrong document.
// The zero value describes no document and NewFraming refuses it.
//
// It is a BOOK, not a registry row. The TS-590S and the TS-590SG are two
// registry rows sharing one document (590:*), and nothing in this file
// distinguishes them: the layout axes that DO differ between the siblings
// are core/kw/ts590's, per row. Naming this type after the document keeps
// the register's standing rule honest — no entry may say "a TS-590" — by
// making it impossible to write a row-shaped claim here at all.
type Book int

const (
	// BookUnset is the zero value: no document. Refused by NewFraming.
	BookUnset Book = iota
	// Book590 is "TS-590S/TS-590SG PC Control Command Reference Guide"
	// (docs cited as 590:LINE), which covers BOTH 590 registry rows.
	Book590
	// Book480 is the TS-480 PC control command reference (480:LINE).
	Book480
)

// String renders a Book for diagnostics.
func (b Book) String() string {
	switch b {
	case Book590:
		return "TS-590S/SG PC command reference"
	case Book480:
		return "TS-480 PC command reference"
	default:
		return "unset PC command reference"
	}
}

// valid reports whether b names a document this package has read.
func (b Book) valid() bool { return b == Book590 || b == Book480 }

// DrainIdleGap and DrainCap are the Kenwood DrainPolicy. The cap is a
// NAMED DECISION (plan P20), not transport's default, and the arithmetic is
// here rather than in a commit message.
//
// IdleGap is transport.QuietPeriod. There is no Kenwood reason to differ:
// it is a statement about how long a serial line must be silent before a
// previous exchange can be assumed finished, which is a property of the
// link and the radio's turnaround rather than of the framing.
//
// THE CAP IS SIZED AGAINST THE TS-480'S DOCUMENTED FLOOD, READ
// UNCONDITIONALLY. "When the old AI is ON and the IF parameters change, the
// transceiver sends the IF command every 1.5 seconds" (480:194-195). A
// session opening against a radio someone left in AI1 or AI3 sees that push
// until AI0; lands, and the conservative reading of the sentence — a push
// every 1.5 s regardless of the condition — is the one that must be
// survivable; the condition only ever makes the flood rarer, never larger.
//
// The arithmetic: a drain succeeds when it observes one IdleGap of silence.
// Pushes 1.5 s apart leave that gap open seven times over, so the only way
// the drain is postponed is a push landing INSIDE its idle window, which
// re-arms the timer. Cap must therefore cover one whole push interval
// (1.5 s) plus two idle gaps (2 x 200 ms) — the window a drain that starts
// immediately after one push and is re-armed by the next needs — which is
// 1.9 s, rounded to 2 s. transport's own default, 2*IdleGap = 400 ms, would
// fail such a drain, which is precisely why P20 refuses to leave it to the
// default.
//
// It remains a HARD CEILING honoured ahead of any queued event and inside
// the wait itself: a line that never pauses fails the drain at 2 s rather
// than holding the engine mutex indefinitely.
const (
	DrainIdleGap = transport.QuietPeriod
	DrainCap     = 2 * time.Second
)

// initFrame is the ONE frame a Kenwood session writes at open: AI0;,
// disabling Auto Information.
//
// It is byte-identical to the Yaesu family's init frame, in a different
// package, and that coincidence is not shared code. No other AI state is
// ever built: the 590 pair's legend has no AI1 and no AI3 at all — 0 OFF,
// 2 ON without backup, 4 ON with backup (590:159-162) — and AI2/AI4 push a
// response per changed parameter (590:165-167); the 480's AI1 and AI3 push
// an IF frame every 1.5 s while the IF parameters change (480:194-195).
// TestFraming_InitSequenceIsAI0AndNothingElse pins the negative.
const initFrame = "AI0;"

// framing is the transport.Framing adapter for one Book.
//
// IT HOLDS NO LOCK AND NO ACCUMULATOR, which is the shape catFraming has
// and NOT the shape core/civ's adapter has. The difference is NoteSent:
// core/civ's accumulator keeps a noted-sent list written under the engine
// mutex and read on the reader goroutine, so that adapter needs its own
// lock and must own exactly one accumulator. Kenwood assumes NO HOST ECHO
// (A25), so NoteSent records nothing, nothing is shared between the two
// goroutines, and NewAccumulator can hand out a fresh accumulator the
// reader goroutine then owns outright. A mutex here would guard nothing.
type framing struct {
	book Book
}

// The seam's claims, asserted by the COMPILER rather than left to the one
// call site that happens to exercise each: this package's Command satisfies
// the neutral Command interface, its FrameAccumulator satisfies
// Accumulator and the adapter is a complete Framing. The additive optional
// hook this dialect exists to use, transport.FatalFramer, is asserted the
// same way in fatal.go, beside the method that implements it.
var (
	_ transport.Command     = Command{}
	_ transport.Accumulator = (*accumulator)(nil)
	_ transport.Framing     = framing{}
)

// NewFraming returns the transport.Framing for book: the Kenwood side of
// the transport seam, ready to hand to transport.NewEngineWith.
//
// AN UNSET OR UNKNOWN BOOK IS REFUSED, and the refusal has to be here. A
// zero framing is constructible by anyone and the value built from one is a
// perfectly non-nil interface carrying a perfectly non-nil Allow method, so
// NewEngineWith's own nil check cannot see it. See ErrUnconfiguredBook for
// why the book is a semantic rather than a nicety.
func NewFraming(book Book) (transport.Framing, error) {
	if !book.valid() {
		return nil, fmt.Errorf("%w (got %v)", ErrUnconfiguredBook, book)
	}
	return framing{book: book}, nil
}

// NewAccumulator returns a fresh Kenwood frame accumulator. max <= 0
// selects DefaultMaxFrame.
//
// A FRESH ONE PER CALL, catFraming's shape: this adapter shares no state
// with it (see the type's doc comment), so there is nothing for a second
// Engine to corrupt and no reason to refuse one.
func (f framing) NewAccumulator(max int) transport.Accumulator {
	return &accumulator{acc: NewFrameAccumulator(max)}
}

// IsRejection reports whether frame is the radio's single unattributed NAK,
// "?;" — and never "E;" or "O;". See frame.go's IsRejection for why the
// distinction is the whole acknowledgement design.
func (f framing) IsRejection(frame []byte) bool { return IsRejection(frame) }

// Allow is the outbound write gate: the last defence before a physical
// radio sees these bytes.
//
// AT THIS TASK IT IS THE ENVELOPE ALONE — the rules both books print about
// what a frame LOOKS like — and it does not yet know which commands exist.
// Task 7 completes it with the eight grammars this milestone builds (ID
// read, AI read/set, FV read, TY read, MC read/set, MR read, MW set, EX
// read) and their field-by-field re-validation. Until then the envelope is
// what stands, and it is written so that T7's gate is an ADDITION in front
// of it rather than a rewrite of it.
//
// A zero framing admits nothing: it speaks for no document, and a gate that
// admitted a frame on behalf of no radio is the one failure this whole file
// is arranged to prevent.
func (f framing) Allow(frame []byte) bool {
	if !f.book.valid() {
		return false
	}
	return envelopeAllows(frame)
}

// envelopeAllows reports whether frame satisfies the envelope both books
// print, and nothing more.
//
// The rules, each with its citation:
//
//   - A terminator, exactly one, and it is the LAST byte (590:87-91,
//     480:113-118). An embedded ';' is refused outright: the 590 book says
//     of the memory name that "';' cannot be used" (590:1577), and a second
//     terminator anywhere would split one frame into two on the radio's own
//     parser.
//   - At least two bytes of command name before it (590:12-13, 480:76),
//     upper case — this programme builds no lower-case opcode, and the
//     books' "either case" permission (590:62, 480:77-78) is about what the
//     radio ACCEPTS, not a licence to emit a second spelling of every frame.
//   - Every body byte printable ASCII, 0x20 to 0x7E. The 480 states the
//     rule generally — "Do not use the control characters 00 to 1Fh since
//     they are either ignored or cause a '?' answer" (480:127-129) — and
//     0x20 is admitted DELIBERATELY, not by accident: a mandatory literal
//     SPACE appears in outbound Set frames (IS P1 "Always a space",
//     590:1184; MC P1 '0' or a space below 100, 590:1334-1337; KY P1 "A
//     space must be used for the Set command", 480:768-769). 0x7F and
//     above are refused because A2's claim is bounded at 0x7E and nothing
//     in either book says what a radio would do with a higher byte.
//   - Never an ANSWER frame. "?;", "E;" and "O;" travel radio-to-host only;
//     a host that emitted one would be inventing a frame no document
//     describes.
//   - Never longer than DefaultMaxFrame — a frame this package's own
//     accumulator could not reassemble is one no answer to could be read.
func envelopeAllows(frame []byte) bool {
	if len(frame) < 3 || len(frame) > DefaultMaxFrame {
		return false
	}
	if frame[len(frame)-1] != ';' {
		return false
	}
	if IsRejection(frame) || streamErrorToken(frame) != "" {
		return false
	}
	body := frame[:len(frame)-1]
	for i, b := range body {
		if b < 0x20 || b > 0x7e || b == ';' {
			return false
		}
		if i < 2 && (b < 'A' || b > 'Z') {
			return false
		}
	}
	return true
}

// InitSequence is the one frame a fresh Kenwood session establishes: AI0;.
// See initFrame for why it is the only AI state ever built.
//
// A zero framing offers nothing, for Allow's reason.
func (f framing) InitSequence() []transport.Command {
	if !f.book.valid() {
		return nil
	}
	return []transport.Command{newCommand([]byte(initFrame))}
}

// DrainPolicy is Kenwood's — see DrainIdleGap and DrainCap for the
// arithmetic behind the cap and why it is not transport's default.
func (f framing) DrainPolicy() transport.DrainPolicy {
	return transport.DrainPolicy{IdleGap: DrainIdleGap, Cap: DrainCap}
}

// NoteSent is a NO-OP, and the emptiness is a recorded assumption rather
// than an omission: A25 assumes Kenwood radios do not echo the host's own
// frames back on the line, so there is nothing to record and nothing to
// suppress. Its lift is L-HW-19, one AI0; Set observed on each of FIVE
// (row, path) legs — the TS-590S's USB-B and RS-232C, the TS-590SG's two,
// and the TS-480's 9-pin D-sub, which is that radio's only path.
//
// If A25 is ever falsified, the repair is core/civ's shape — a noted-sent
// list matched by RECORDED BYTES, never by position or count — and it would
// bring this adapter's own lock with it, because the list is written under
// the engine mutex and read on the reader goroutine.
// TestFraming_NoteSentIsANoOp pins the behaviour, not the empty body.
func (f framing) NoteSent([]byte) {}

// accumulator is the transport.Accumulator the engine's reader goroutine
// holds: this package's FrameAccumulator, with its oversize error
// TRANSLATED onto core/transport's.
//
// THE TRANSLATION IS NOT COSMETIC. Engine.handleReaderErr distinguishes
// exactly two outcomes by TYPE: an error that errors.As-matches
// *transport.FrameTooLongError marks the stream CONTAMINATED and leaves the
// port open for a DrainToQuiet to recover; anything else is taken for a
// dead port and CLOSES it. core/kw mints its own *FrameTooLongError
// (errors.go says why), and that type is NOT transport's. Handed over
// untranslated, a Kenwood line that merely went noisy would tear the
// session down as an I/O failure instead of entering the recoverable state
// built for precisely this condition.
type accumulator struct{ acc *FrameAccumulator }

// Push assembles frames and translates the oversize error.
func (a *accumulator) Push(chunk []byte) ([][]byte, error) {
	frames, err := a.acc.Push(chunk)
	return frames, translateAccumulatorErr(err)
}

// translateAccumulatorErr maps core/kw's oversize error onto
// core/transport's, preserving DiscardedLen. Any other error passes through
// unchanged — the accumulator raises none today, and inventing a
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
