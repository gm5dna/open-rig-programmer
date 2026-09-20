// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// THE ERROR VALUES IN THIS FILE ARE THIS PACKAGE'S OWN, DELIBERATELY, AND
// THE FRAMING ADAPTER RECONCILES THEM AT THE SEAM.
//
// core/transport re-exports core/cat's ErrRejected and ErrFrameTooLong, so
// the canonical values live in a package core/kw must never import (the
// fence, imports_test.go). core/civ met the same wall and took the same
// route: mint local values, and translate at the adapter —
// framing.Push maps *FrameTooLongError onto transport's, which is what
// makes Engine.handleReaderErr mark the stream CONTAMINATED rather than
// close the port.
//
// There is no local ErrRejected sentinel. A rejection is a WIRE CONDITION
// this package recognises (IsRejection, frame.go); which error value a
// rejected command surfaces as belongs to the engine that issued it, and
// that is transport.ErrRejected. RejectionError WRAPS it rather than
// replacing it.

// ErrFrameTooLong is the sentinel to compare against (via errors.Is) when
// FrameAccumulator.Push reports that stream data exceeded the configured
// maximum frame length without forming an in-bound frame. Every error Push
// returns for this condition is a *FrameTooLongError.
var ErrFrameTooLong = errors.New("kw: frame exceeded maximum length")

// FrameTooLongError reports that FrameAccumulator.Push discarded
// DiscardedLen bytes because they could not form a legitimate frame within
// the accumulator's bound.
//
// It means the line is noisy, wedged, or carrying something that is not a
// Kenwood frame. The accumulator resets itself; the caller must treat
// everything from this point as contaminated and drain to a quiet boundary
// before trusting subsequent frames.
type FrameTooLongError struct {
	// DiscardedLen is the number of bytes thrown away.
	DiscardedLen int
}

func (e *FrameTooLongError) Error() string {
	return fmt.Sprintf("kw: frame exceeded maximum length, discarded %d bytes", e.DiscardedLen)
}

// Unwrap lets errors.Is(err, ErrFrameTooLong) match, alongside errors.As
// for callers that want DiscardedLen.
func (e *FrameTooLongError) Unwrap() error { return ErrFrameTooLong }

// ErrUnconfiguredBook is the sentinel NewFraming returns when it is not
// told which of the four PC-command documents the session is talking to.
//
// It fails closed for the house's standing reason — an omitted config
// semantic is REFUSED, never defaulted — and for a Kenwood-specific one:
// the book is what decides which cause sentence an "O;" carries, because
// the documents do not all give the SAME token the SAME cause (erratum
// E13). A defaulted book would quote a document the session was never told
// it was speaking to.
var ErrUnconfiguredBook = errors.New("kw: framing was given no PC-command document to speak for, refusing to construct")

// maxParseErrorFrameLen bounds how much offending input a ParseError
// retains, so malformed or hostile input cannot make a log line unbounded.
const maxParseErrorFrameLen = 64

// ErrParse is the sentinel every ParseError wraps.
var ErrParse = errors.New("kw: parse error")

// ParseError reports that some Kenwood wire value did not match any
// explicitly valid form. This package favours strictness: anything not
// explicitly valid is refused rather than guessed at.
//
// Frame holds a defensive copy of the offending input, truncated, so the
// error never aliases caller memory and never grows without bound. It is
// rendered %q-quoted: frame content is radio-supplied, not trusted.
type ParseError struct {
	Frame  []byte
	Reason string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("kw: parse error: %s (input=%q)", e.Reason, e.Frame)
}

func (e *ParseError) Unwrap() error { return ErrParse }

// newParseError builds a ParseError from the offending input, copying and
// truncating it.
func newParseError(input []byte, format string, args ...any) *ParseError {
	n := min(len(input), maxParseErrorFrameLen)
	return &ParseError{Frame: bytes.Clone(input[:n]), Reason: fmt.Sprintf(format, args...)}
}

// THE TYPED ERROR FAMILY — the three outcomes every book prints, each
// carrying the document's own words.
//
// It exists because every one of these books prints an explicit
// error-message table, which is more than any registered Yaesu manual does,
// and because the table says three things of which one is alarming: "?;"
// has TWO indistinguishable causes; "E;" and "O;" are stream-health tokens
// rather than answers; and the NAK itself "may not appear due to
// microprocessor transients in the transceiver" (590:106-108, 480:136-138,
// 890:114-116, 990:114-116). A driver that reported "no answer" as "not
// present" would be inferring absence from a signal the manufacturer has
// told us is unreliable, so the timeout is a member of this family too, and
// its message says so.
//
// All three are recovered with errors.As and all three wrap a sentinel a
// caller can errors.Is against: ErrStream is this package's own (the engine
// has no equivalent — it simply closes the port with this value as the
// cause), while the rejection and the timeout wrap core/transport's, which
// is where the neutral names live.

// ErrStream is the sentinel every StreamError wraps.
var ErrStream = errors.New("kw: stream-health token")

// StreamError is the typed cause an "E;" or an "O;" closes a session with.
//
// It is what framing.IsFatal returns, so the engine records THIS VALUE
// through closePort and every subsequent closed-engine error wraps it: a
// driver recovers it with errors.As and can name the token and the
// document's own cause sentence. A sentinel would throw away the whole
// reason the FatalFramer hook exists.
//
// THE BOOK IS A FIELD BECAUSE THE BOOKS DISAGREE — erratum E13. "E;" is
// given the same cause in all four (590:110-112, 480:140-142, 890:118-120,
// 990:118-120), but "O;" is "A receive buffer overrun error occurred" in
// the 590 book (590:113), the TS-890S's (890:121-123) and the TS-990S's
// (990:121), and "Receive data was sent but processing was not completed"
// in the 480's (480:143-144) — so the divergence is the TS-480's alone,
// one document out of four. Those are different claims about the radio,
// not two phrasings of one, and a session that quoted the wrong one would
// be quoting a document it is not talking to.
//
// IT MUST NEVER WRAP A *transport.FrameTooLongError. FatalFramer's contract
// says so and the reason is mechanical: the engine delivers this cause as
// an ordinary reader event as well as closing the port, and the consumer of
// that event tests for *FrameTooLongError first, so a wrapped one would be
// reported to a caller as recoverable stream contamination rather than as
// the link having ended.
// TestStreamError_MustNotWrapAFrameTooLongError pins it.
type StreamError struct {
	// Token is the frame verbatim: "E;" or "O;".
	Token string
	// Book is the document whose sentence Cause quotes.
	Book Book
	// Cause is that document's own cause sentence, verbatim.
	Cause string
	// Citation is where the sentence is printed, in the layout-text form
	// "590:LINE", "480:LINE", "890:LINE" or "990:LINE".
	Citation string
}

func (e *StreamError) Error() string {
	return fmt.Sprintf("kw: stream-health token %q from the radio: %s (%s) — the link has ended; the port is closed and nothing further is sent", e.Token, e.Cause, e.Citation)
}

// Unwrap lets errors.Is(err, ErrStream) match, alongside errors.As for
// callers that want the token and the sentence.
func (e *StreamError) Unwrap() error { return ErrStream }

// newStreamError builds the StreamError for token as the given book prints
// it.
//
// IT PANICS on a frame that is not one of the two tokens OR on a book that
// names no document, and the loudness is deliberate: framing.IsFatal has
// recognised the token (streamErrorToken returned non-empty) AND has
// already returned early on an INVALID book (book.valid() false), so
// neither condition can reach here from the only door there is. Reaching
// this branch therefore means the recogniser and this constructor have
// drifted apart — every VALID book now has a case, Book570 and Book870S
// included (their citations landed in the Lift K follow-up, below).
//
// The unconfigured book is refused at the CALLER rather than here because
// IsFatal runs on the engine's reader goroutine, which has no recover; see
// framing.IsFatal for why withholding the verdict is the closed direction.
// TestFraming_ZeroValueFailsClosed pins that no zero framing ever arrives.
func newStreamError(token string, book Book) *StreamError {
	switch {
	case token == communicationErrorFrame && book == Book590:
		return &StreamError{Token: token, Book: book, Cause: commErrorCause, Citation: "590:110-112"}
	case token == communicationErrorFrame && book == Book480:
		return &StreamError{Token: token, Book: book, Cause: commErrorCause, Citation: "480:140-142"}
	case token == receiveOverrunFrame && book == Book590:
		return &StreamError{Token: token, Book: book, Cause: "A receive buffer overrun error occurred", Citation: "590:113"}
	case token == receiveOverrunFrame && book == Book480:
		return &StreamError{Token: token, Book: book, Cause: "Receive data was sent but processing was not completed", Citation: "480:143-144"}
	case token == communicationErrorFrame && book == Book890:
		return &StreamError{Token: token, Book: book, Cause: commErrorCause, Citation: "890:118-120"}
	case token == communicationErrorFrame && book == Book990:
		return &StreamError{Token: token, Book: book, Cause: commErrorCause, Citation: "990:118-120"}
	// THE TWO NEW BOOKS SIDE WITH THE 590 ON "O;", WHICH NARROWS E13 rather
	// than widening it: both print "A receive buffer overrun error occurred"
	// verbatim, so the TS-480 is the one document out of four that says
	// something else. No new sentence is minted here, only citations.
	case token == receiveOverrunFrame && book == Book890:
		return &StreamError{Token: token, Book: book, Cause: "A receive buffer overrun error occurred", Citation: "890:121-123"}
	case token == receiveOverrunFrame && book == Book990:
		return &StreamError{Token: token, Book: book, Cause: "A receive buffer overrun error occurred", Citation: "990:121"}
	// Book570 AND Book870S, FOUND IN THE LIFT K FOLLOW-UP (13/09/2026), by
	// reading each document's own full manual text rather than the
	// command-table-only evidence this lift originally had: both print
	// "E;" with commErrorCauseNoComma's own wording (a genuine textual
	// variant of commErrorCause — no comma after "occurred" — not a
	// transcription slip either side of it) and "O;" with the TS-480's
	// own sentence verbatim, so BOTH new books side with the TS-480 on
	// "O;" where the 890/990 case above sides with the 590. Citations are
	// this codec's own line numbers, from
	// docs/fixtures-private/manuals/ts570_manual_00_layout.txt and
	// ts870s_manual_mirror_layout.txt.
	case token == communicationErrorFrame && book == Book570:
		return &StreamError{Token: token, Book: book, Cause: commErrorCauseNoComma, Citation: "ts570:5170-5172"}
	case token == receiveOverrunFrame && book == Book570:
		return &StreamError{Token: token, Book: book, Cause: "Receive data was sent but processing was not completed", Citation: "ts570:5174-5175"}
	case token == communicationErrorFrame && book == Book870S:
		return &StreamError{Token: token, Book: book, Cause: commErrorCauseNoComma, Citation: "ts870s:8445-8447"}
	case token == receiveOverrunFrame && book == Book870S:
		return &StreamError{Token: token, Book: book, Cause: "Receive data was sent but processing was not completed", Citation: "ts870s:8449-8450"}
	default:
		panic(fmt.Sprintf("kw: newStreamError(%q, %v): not a stream-health token, or no book to quote — IsFatal is the only caller and it has already recognised both", token, book))
	}
}

// commErrorCause is the sentence EVERY original-four book gives for "E;",
// printed identically in each (590:110-112, 480:140-142, 890:118-120,
// 990:118-120). It is written once because the documents agree here, which
// is exactly what makes the TS-480's disagreement about "O;" (E13) worth
// recording.
const commErrorCause = "A communication error occurred, such as an overrun or framing error during a serial data transmission"

// commErrorCauseNoComma is the TS-570's and the TS-870S's own wording for
// "E;": the same sentence commErrorCause carries, but printed with no
// comma after "occurred" in either document (ts570:5170-5172,
// ts870s:8445-8447) — a genuine textual variant the two new books agree
// with each other on and not with the original four, so it is its own
// constant rather than a fifth citation of commErrorCause's exact bytes.
const commErrorCauseNoComma = "A communication error occurred such as an overrun or framing error during a serial data transmission"

// rejectionCauses is the whole of what all four books say a "?;" means —
// TWO causes, printed as alternatives, with nothing anywhere to tell them
// apart (590:100-105, 480:130-135, 890:106-112, 990:108-113).
const rejectionCauses = "either \"Command syntax was incorrect\" or \"Command was not executed due to the current status of the transceiver (even though the command syntax was correct)\" — the two are printed as alternatives and nothing in the document distinguishes them"

// transientSentence is the sentence that makes silence uninformative, and
// it is quoted in BOTH the rejection and the timeout because it bears on
// both: a NAK that did not arrive is not evidence that none was sent
// (590:106-108, 480:136-138).
const transientSentence = "the document also warns that \"Occasionally, this message may not appear due to microprocessor transients in the transceiver\""

// bookCitations gives, per book, the lines that book prints the two "?;"
// causes on and the line it prints the transient warning on. All four print
// the same two SENTENCES — which is why rejectionCauses and
// transientSentence are written once — and only the lines differ.
//
// IT IS ONE TABLE WITH TWO READERS, and that is the point. RejectionError
// quotes both entries and TimeoutError quotes the transient one alone; with
// a citation pair written out in each method, the two would be one edit from
// citing different lines for the same warning in the same book. A bound is
// consulted from the same place as its datum.
//
// A BOOK ABSENT FROM THIS TABLE GETS NO LINE AT ALL. That is what makes an
// error naming no document say so rather than fall through to the TS-590's
// numbers — TestTypedErrors_QuoteNoDocumentTheyWereNotGiven is the pin —
// and it is why the lookup is a map rather than a switch with a default arm
// that has to remember to be empty.
// Book570 AND Book870S, FOUND IN THE LIFT K FOLLOW-UP (13/09/2026): both
// print rejectionCauses' two sentences verbatim (ts570:5158-5163,
// ts870s:8434-8439). Their own transient-warning line drops the comma
// after "Occasionally" that transientSentence's quote carries ("Note:
// Occasionally this message may not appear...", ts570:5165-5167,
// ts870s:8441-8443) — the SAME single-comma variant newStreamError's
// commErrorCauseNoComma records for "E;", not re-minted as a second
// constant here because it changes no word transientSentence quotes,
// only one already-silent punctuation mark the citation line still
// points a reader at.
var bookCitations = map[Book]struct{ causes, transient string }{
	Book590:  {"590:100-105", "590:106-108"},
	Book480:  {"480:130-135", "480:136-138"},
	Book890:  {"890:106-112", "890:114-116"},
	Book990:  {"990:108-113", "990:114-116"},
	Book570:  {"ts570:5158-5163", "ts570:5165-5167"},
	Book870S: {"ts870s:8434-8439", "ts870s:8441-8443"},
}

// RejectionError is the typed refusal a "?;" produces.
//
// IT WRAPS transport.ErrRejected RATHER THAN REPLACING IT. The engine is
// what turns a frame IsRejection admits into ErrRejected, and that neutral
// value is the one every driver in this programme already tests for; this
// type adds the two things the engine cannot know — which command was
// refused, and what the radio's own document says a refusal means. A second
// sentinel would be two names for one thing.
//
// It is a DEFINITIVE answer and is never retried.
type RejectionError struct {
	// Book is the document quoted.
	Book Book
	// Command is the frame the radio refused, %q-safe for a log line.
	Command string
}

func (e *RejectionError) Error() string {
	// THE SENTENCES ARE COMMON TO EVERY BOOK; ONLY THE LINES DIFFER. Each
	// document prints the same two alternative causes and the same
	// transient warning, which is why rejectionCauses and
	// transientSentence are written once and only bookCitations is per
	// book. So a value that names no document can still say WHAT a "?;"
	// means — it simply may not say WHERE, and it says that it cannot
	// rather than falling through to one book's line numbers.
	// TestTypedErrors_QuoteNoDocumentTheyWereNotGiven pins that branch.
	if c, ok := bookCitations[e.Book]; ok {
		return fmt.Sprintf("kw: the radio refused %q with \"?;\": %s (%s); %s (%s), so a refusal that did not arrive is not evidence that none was sent",
			e.Command, rejectionCauses, c.causes, transientSentence, c.transient)
	}
	return fmt.Sprintf("kw: the radio refused %q with \"?;\": %s; %s — this error names no document (%s), so no line is cited: every book prints these sentences, but naming a document this session was never told it was speaking to is what ErrUnconfiguredBook exists to prevent",
		e.Command, rejectionCauses, transientSentence, e.Book)
}

// Unwrap lets errors.Is(err, transport.ErrRejected) match.
func (e *RejectionError) Unwrap() error { return transport.ErrRejected }

// NewRejectionError builds the typed refusal for command as book prints it.
//
// IT IS FALLIBLE, ON NewFraming'S MODEL, and for NewFraming's reason: it is
// exported, a caller can hand it BookUnset, and an error that fell through
// to the TS-590's line numbers would quote a document the session was never
// told it was speaking to (ErrUnconfiguredBook). An omitted config semantic
// is REFUSED, never defaulted — the M9c-1 ruling — and a constructor is the
// place a Kenwood session can still be refused cheaply.
// TestNewRejectionError_RefusesAnUnsetBook pins it.
func NewRejectionError(book Book, command string) (*RejectionError, error) {
	if !book.valid() {
		return nil, fmt.Errorf("%w (got %v)", ErrUnconfiguredBook, book)
	}
	return &RejectionError{Book: book, Command: command}, nil
}

// TimeoutError is the typed cause a read timeout produces, and it is a
// member of this family for one reason: SILENCE CARRIES NO INFORMATION ON
// THIS FAMILY.
//
// Every book says the NAK is unreliable (590:106-108, 480:136-138,
// 890:114-116, 990:114-116), so a read that times out is neither "the thing
// is absent" nor "the radio refused". The session read fails WHOLE, with no
// retry and NO INFERENCE OF ABSENCE — and the message says that in as many
// words, because the inference is the one a reader will otherwise make.
// (The FT-891's "?;-means-absent" discovery is unavailable here for the
// same reason and one more: this family's slot space is printed, so nothing
// needs discovering.)
type TimeoutError struct {
	// Book is the document quoted.
	Book Book
	// Command is the frame that went unanswered.
	Command string
}

func (e *TimeoutError) Error() string {
	// The two branches are RejectionError.Error's exactly, for its reason:
	// the transient sentence is printed in every book, so a value that
	// names no document may still quote it and must not invent a line
	// number for it. The line itself comes from bookCitations, the same
	// table RejectionError reads, so the two cannot cite the same warning
	// in the same book at different lines.
	if c, ok := bookCitations[e.Book]; ok {
		return fmt.Sprintf("kw: no answer to %q within the read timeout: this is not evidence of absence and not a refusal — %s (%s), so silence carries no information; the session read fails whole and is not retried",
			e.Command, transientSentence, c.transient)
	}
	return fmt.Sprintf("kw: no answer to %q within the read timeout: this is not evidence of absence and not a refusal — %s; this error names no document (%s), so no line is cited; the session read fails whole and is not retried",
		e.Command, transientSentence, e.Book)
}

// Unwrap lets errors.Is(err, transport.ErrTimeout) match.
func (e *TimeoutError) Unwrap() error { return transport.ErrTimeout }

// NewTimeoutError builds the typed timeout for command as book prints it.
//
// FALLIBLE ON NewRejectionError'S TERMS EXACTLY — see there for why an
// exported constructor of a document-quoting error refuses BookUnset rather
// than defaulting to one book. TestNewTimeoutError_RefusesAnUnsetBook pins
// it.
func NewTimeoutError(book Book, command string) (*TimeoutError, error) {
	if !book.valid() {
		return nil, fmt.Errorf("%w (got %v)", ErrUnconfiguredBook, book)
	}
	return &TimeoutError{Book: book, Command: command}, nil
}
