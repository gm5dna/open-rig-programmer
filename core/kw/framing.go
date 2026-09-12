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

// Book names which of the FOUR PC-command documents a session speaks to.
//
// IT IS NOT DECORATION AND IT HAS NO DEFAULT. The books give the SAME
// stream-health token DIFFERENT causes — "O;" is "A receive buffer overrun
// error occurred" on the 590 pair (590:113), the TS-890S (890:121-123) and
// the TS-990S (990:121), and "Receive data was sent but processing was not
// completed" on the TS-480 (480:143-144), erratum E13 — so a diagnostic
// that names the wrong sentence names the wrong document. The zero value
// describes no document and NewFraming refuses it.
//
// valid() NOW ANSWERS TWO DIFFERENT QUESTIONS AND ONLY ONE OF THEM IS
// NewLayout'S. It says "a document this package has READ", which is what
// framing, the typed errors and IsFatal need: all four books print the same
// envelope and the same error-message table, so this package can speak for
// all four at the stream level. It does NOT say "a document this package's
// 50-byte RECORD describes" — that is two books, the 590 pair's and the
// TS-480's, and NewLayout tests for those two BY NAME rather than through
// valid() (layout.go). The TS-890S and TS-990S memory channel is
// core/kw/ma's, whose layout type is not a kw.Layout at all.
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
	// Book890 is the TS-890S PC control command reference (890:LINE).
	Book890
	// Book990 is the TS-990S PC control command reference (990:LINE).
	Book990
	// Book570 is the TS-570D/S/DG PC control command reference, document
	// code B62-1542-00 (evidence/ts570d.md §1). Its 28-byte memory record
	// is a true prefix of the 590 pair's/TS-480's grid (RecordLen), so it
	// is one of the three books kw.Layout describes.
	Book570
	// Book870S is the TS-870S PC control command reference, document code
	// B62-1536-00, read via the rigpix.com community mirror under the
	// 12/09/2026 provenance widening (evidence/ts870s.md §1). Its 22-byte
	// memory record is NOT a prefix of the family grid (its offsets shift
	// rather than merely stop early), so its channel is core/kw's own
	// Layout870 rather than a kw.Layout.
	//
	// NEITHER NEW BOOK HAS A TRANSCRIBED STREAM-ERROR TABLE YET. S2/S3's
	// evidence for both stops at the command-table pages (the byte-diagram
	// and Parameter Table pages this lift's RecordLen/field axes needed);
	// neither the "?;" rejection causes, the "E;"/"O;" stream tokens, nor
	// the transient-NAK warning has a citation for either document. See
	// errors.go's newStreamError and bookCitations: a live NewFraming
	// session for either book must not be wired up before a citation
	// lands there — this lift only adds the two documents to the set a
	// Layout/Layout870 may name, not to a working stream-error table.
	Book870S
)

// String renders a Book for diagnostics.
func (b Book) String() string {
	switch b {
	case Book590:
		return "TS-590S/SG PC command reference"
	case Book480:
		return "TS-480 PC command reference"
	case Book890:
		return "TS-890S PC command reference"
	case Book990:
		return "TS-990S PC command reference"
	case Book570:
		return "TS-570D/S/DG PC command reference"
	case Book870S:
		return "TS-870S PC command reference (rigpix.com mirror of B62-1536-00)"
	default:
		return "unset PC command reference"
	}
}

// valid reports whether b names a document this package has READ — see the
// type's doc comment for why that is not the same question as "a document
// this package's 50-byte record describes", which is NewLayout's.
func (b Book) valid() bool {
	switch b {
	case Book590, Book480, Book890, Book990, Book570, Book870S:
		return true
	default:
		return false
	}
}

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
// AFTER KENWOOD PAIR 2 STAGE 0 THIS CAP GATES ALL FOUR BOOKS, NOT ONLY THE
// 590 PAIR AND THE TS-480. Neither the TS-890S's nor the TS-990S's document
// states its own AI-flood arithmetic; the TS-480's 1.5 s figure is applied
// to them as the conservative bound this discipline takes on an unprinted
// fact, not because it is their own datum.
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
// Accumulator, the adapter is a complete Framing, and — the additive
// optional hook this dialect exists to use, and the whole of Q13 Option A —
// a transport.FatalFramer.
var (
	_ transport.Command     = Command{}
	_ transport.Accumulator = (*accumulator)(nil)
	_ transport.Framing     = framing{}
	_ transport.FatalFramer = framing{}
)

// NewFraming returns the transport.Framing for book: the Kenwood side of
// the transport seam, ready to hand to transport.NewEngineWith.
//
// AN UNSET OR UNKNOWN BOOK IS REFUSED, and the refusal has to be here. A
// zero framing is constructible by anyone and the value built from one is a
// perfectly non-nil interface carrying a perfectly non-nil Allow method, so
// NewEngineWith's own nil check cannot see it. See ErrUnconfiguredBook for
// why the book is a semantic rather than a nicety.
//
// THIS CONSTRUCTOR'S Allow IS THE ENVELOPE ALONE, never the eight grammars:
// a book knows which document a session speaks (and so which cause sentence
// an "O;" carries, E13) but not which RADIO, and the per-command gate is a
// layout's. A framing built here therefore admits, among other things, the
// 42-byte erase shape of 590:1579-1581 — a frame the book really prints and
// this programme never builds. A DRIVER calls NewFramingFor(layout)
// instead (allowlist.go); NewFraming exists for NewFramingFor to build on
// and for the envelope's own tests and pins. THAT IS A GUARDED RULE AND NOT
// ONLY A SENTENCE: internal/guards' TestKenwoodDriversUseNewFramingFor fails
// on any non-test file under core/driver that calls this constructor.
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

// IsFatal implements the OPTIONAL transport.FatalFramer hook, and taking it
// is this milestone's Q13 Option A: "E;" and "O;" end the STREAM, as
// distinct from IsRejection's "this COMMAND was refused".
//
// WHY THE HOOK AND NOT ONE OF THE THREE PLACES A Framing ALREADY HAS.
// IsRejection would collapse the tokens into ErrRejected — a refusal, not a
// link failure, and indistinguishable from "?;". A driver-level matcher
// would miss them entirely on a ClassWrite, which is what MW is, and that
// is the one place the token matters most. An unrecognised frame is merely
// counted. None of the three closes the three residual windows spec
// §"Where E; and O; live" names, and only this one does.
//
// WHAT TAKING IT BUYS, at exactly the strength the engine delivers. A fatal
// frame is RECEIVED when its publication has taken and released the
// engine's fatal gate, and the guarantee is then: NO FRAME LEAVES THE HOST
// AFTER A FATAL FRAME THE ENGINE HAS RECEIVED. It is a theorem, not a hope
// — the gate totally orders the publication against the final write, so
// either the publication went first (the write's closed recheck sees the
// closure and writes nothing) or the write went first (the frame left
// BEFORE the fatal frame was received). All three windows the accumulator
// route left open are closed: the same-chunk answer, the entry purge's
// bound, and the post-purge race.
//
// The typed cause is a *StreamError carrying the token and THAT BOOK'S own
// sentence (E13), never a sentinel: the engine records the value closePort
// is given and every subsequent closed-engine error wraps it, so a driver
// recovers it with errors.As and can quote the document.
//
// NEITHER TOKEN IS EVER RETRIED and neither is ever matched as an answer.
//
// A ZERO framing WITHHOLDS THE VERDICT, on Allow's and InitSequence's
// terms and for one extra reason: this is the only one of the three that
// runs on the engine's READER GOROUTINE, which has no recover. The cause an
// "E;" or an "O;" closes a session with is that BOOK'S own sentence (E13),
// so an adapter that names no document has no honest verdict to give — and
// the M9c-1 ruling says an omitted config semantic is REFUSED, never
// defaulted, so it gives none rather than quoting a document at a venture.
// Withholding is the closed direction here: Allow already admits nothing,
// so no frame can leave whatever this returns.
// TestFraming_ZeroValueFailsClosed pins all three doors together.
//
// TestFatalTokens_FourStatesTwoTokens is the injection matrix;
// TestFatalTokens_SameChunk_SuppressesTheAnswerItArrivedWith and
// TestFatalTokens_PostPurgeRace_NoFrameLeavesAfterAReceivedFatalFrame are
// core/transport's two adversarial pins re-run through this accumulator and
// this cause. doc.go carries the commitment and the residual liveness facts.
func (f framing) IsFatal(frame []byte) error {
	if !f.book.valid() {
		return nil
	}
	token := streamErrorToken(frame)
	if token == "" {
		return nil
	}
	return newStreamError(token, f.book)
}

// Allow is the outbound write gate: the last defence before a physical
// radio sees these bytes.
//
// THIS METHOD IS THE ENVELOPE ALONE — the rules all four books print about
// what a frame LOOKS like — and it does not know which commands exist,
// because a framing built by NewFraming knows the book and not the layout.
// T7 built the eight-grammar gate (ID read, AI read/set, FV read, TY read,
// MC read/set, MR read, MW set, EX read) and their field-by-field
// re-validation as a SECOND, NARROWER Allow on layoutFraming
// (allowlist.go's NewFramingFor), sitting in front of this one rather than
// replacing it — layoutFraming.Allow calls both, and the conjunction is
// free because the grammars are strictly narrower
// (TestAllowedCommand_AdmitsOnlyFramesTheEnvelopeAlsoAdmits pins it). This
// method is what a driver's gate falls back to only if it reaches for
// NewFraming instead of NewFramingFor, which is why NewFraming's own doc
// says not to and why internal/guards'
// TestKenwoodDriversUseNewFramingFor refuses it mechanically.
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

// envelopeAllows reports whether frame satisfies the envelope all four
// books print, and nothing more.
//
// The rules, each with its citation:
//
//   - A terminator, exactly one, and it is the LAST byte (590:87-91,
//     480:113-118, 890:93-96, 990:96-99). An embedded ';' is refused
//     outright: the 590 book says of the memory name that "';' cannot be
//     used" (590:1577), and a second terminator anywhere would split one
//     frame into two on the radio's own parser.
//   - At least two bytes of command name before it (590:12-13, 480:76,
//     890:76-80, 990:77-83), upper case — this programme builds no
//     lower-case opcode, and the four books' "either case" permission
//     (590:62, 480:77-78, 890:77-78, 990:80-81) is about what the
//     radio ACCEPTS, not a licence to emit a second spelling of every frame.
//   - Every body byte printable ASCII, 0x20 to 0x7E. The 480 states the
//     rule generally — "Do not use the control characters 00 to 1Fh since
//     they are either ignored or cause a '?' answer" (480:127-129) —
//     PRINTED BY THE TS-480 ALONE: neither the TS-890S's nor the TS-990S's
//     book states it, so on Book890 and Book990 this floor is an ASSUMED
//     extension of A2 (core/kw/doc.go's register), kept because refusing
//     more input is always the safe direction. 0x20 is admitted
//     DELIBERATELY, not by accident: a mandatory literal
//     SPACE appears in outbound Set frames (IS P1 "Always a space",
//     590:1184; MC P1 '0' or a space below 100, 590:1334-1337; KY P1 "A
//     space must be used for the Set command", 480:768-769). 0x7F and
//     above are refused because A2's claim is bounded at 0x7E and nothing
//     in either book says what a radio would do with a higher byte.
//   - Never an ANSWER frame. "?;", "E;" and "O;" travel radio-to-host only;
//     a host that emitted one would be inventing a frame no document
//     describes. THE RULE THAT ACTUALLY REFUSES THEM TODAY IS THE ONE
//     ABOVE, not a check of its own: all three tokens are two bytes, and
//     two bytes cannot carry an opcode and a terminator. The explicit
//     branch below is belt and braces, unreachable as the three tokens
//     stand, and no test can distinguish it — the three token rows in
//     TestEnvelopeAllows_TheDocumentedEnvelope pass on the length floor. It
//     is kept because T7 puts per-command grammars IN FRONT of this gate
//     rather than replacing it, so the day a token grows a third byte the
//     rule is already written.
//   - Never longer than DefaultMaxFrame, which is the SAME datum the
//     accumulator this adapter hands out enforces when given max <= 0:
//     TestFraming_AllowsBoundIsTheDefaultAccumulatorsBound pins the gate
//     and that accumulator to one bound in both directions. IT IS NOT THE
//     ENGINE'S WithMaxFrame(N). The seam tells NewAccumulator that number
//     and tells this gate nothing, so on a session built with
//     N < DefaultMaxFrame this gate is the WIDER of the two and would admit
//     an outbound frame that session's accumulator could not reassemble.
//     Nothing this milestone builds sets N, and T7's per-command grammars
//     bound every frame this programme builds far below either number.
func envelopeAllows(frame []byte) bool {
	if len(frame) < 3 || len(frame) > DefaultMaxFrame {
		return false
	}
	if frame[len(frame)-1] != ';' {
		return false
	}
	// Belt and braces; unreachable while every token is two bytes. See the
	// fourth bullet above.
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
