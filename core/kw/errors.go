// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import (
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
// told which of the two PC-command documents the session is talking to.
//
// It fails closed for the house's standing reason — an omitted config
// semantic is REFUSED, never defaulted — and for a Kenwood-specific one:
// the book is what decides which cause sentence an "O;" carries, because
// the two documents give the SAME token DIFFERENT causes (erratum E13). A
// defaulted book would quote a document the session was never told it was
// speaking to.
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
	n := len(input)
	if n > maxParseErrorFrameLen {
		n = maxParseErrorFrameLen
	}
	return &ParseError{Frame: copyBytes(input[:n]), Reason: fmt.Sprintf(format, args...)}
}

// THE TYPED ERROR FAMILY — the three outcomes both books print, each
// carrying the document's own words.
//
// It exists because this pair prints an explicit error-message table, which
// is more than any registered Yaesu manual does, and because the table says
// three things of which one is alarming: "?;" has TWO indistinguishable
// causes; "E;" and "O;" are stream-health tokens rather than answers; and
// the NAK itself "may not appear due to microprocessor transients in the
// transceiver" (590:106-108, 480:136-138). A driver that reported "no
// answer" as "not present" would be inferring absence from a signal the
// manufacturer has told us is unreliable, so the timeout is a member of
// this family too, and its message says so.
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
// THE BOOK IS A FIELD BECAUSE THE TWO BOOKS DISAGREE — erratum E13. "E;" is
// given the same cause in both (590:110-112, 480:140-142), but "O;" is "A
// receive buffer overrun error occurred" in the 590 book (590:113) and
// "Receive data was sent but processing was not completed" in the 480's
// (480:143-144). Those are different claims about the radio, not two
// phrasings of one, and a session that quoted the wrong one would be
// quoting a document it is not talking to.
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
	// Citation is where the sentence is printed, in this milestone's
	// "590:LINE" / "480:LINE" layout-text form.
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
// IT PANICS on a frame that is not one of the two tokens, and the loudness
// is deliberate: the only caller is framing.IsFatal, which has ALREADY
// recognised the token, so reaching this branch means the recogniser and
// the constructor have drifted apart. Returning nil instead would make a
// fatal frame silently ordinary — the one failure mode this whole design
// exists to prevent — and returning a placeholder sentence would put words
// in a manufacturer's mouth.
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
	default:
		panic(fmt.Sprintf("kw: newStreamError(%q, %v): not a stream-health token, or no book to quote — IsFatal is the only caller and it has already recognised the token", token, book))
	}
}

// commErrorCause is the sentence BOTH books give for "E;", printed
// identically in each (590:110-112, 480:140-142). It is written once
// because the two documents agree here, which is exactly what makes their
// disagreement about "O;" (E13) worth recording rather than smoothing over.
const commErrorCause = "A communication error occurred, such as an overrun or framing error during a serial data transmission"

// rejectionCauses is the whole of what either book says a "?;" means — TWO
// causes, printed as alternatives, with nothing anywhere to tell them apart
// (590:100-105, 480:130-135).
const rejectionCauses = "either \"Command syntax was incorrect\" or \"Command was not executed due to the current status of the transceiver (even though the command syntax was correct)\" — the two are printed as alternatives and nothing in the document distinguishes them"

// transientSentence is the sentence that makes silence uninformative, and
// it is quoted in BOTH the rejection and the timeout because it bears on
// both: a NAK that did not arrive is not evidence that none was sent
// (590:106-108, 480:136-138).
const transientSentence = "the document also warns that \"Occasionally, this message may not appear due to microprocessor transients in the transceiver\""

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
	cite, transientCite := "590:100-105", "590:106-108"
	if e.Book == Book480 {
		cite, transientCite = "480:130-135", "480:136-138"
	}
	return fmt.Sprintf("kw: the radio refused %q with \"?;\": %s (%s); %s (%s), so a refusal that did not arrive is not evidence that none was sent",
		e.Command, rejectionCauses, cite, transientSentence, transientCite)
}

// Unwrap lets errors.Is(err, transport.ErrRejected) match.
func (e *RejectionError) Unwrap() error { return transport.ErrRejected }

// NewRejectionError builds the typed refusal for command as book prints it.
func NewRejectionError(book Book, command string) *RejectionError {
	return &RejectionError{Book: book, Command: command}
}

// TimeoutError is the typed cause a read timeout produces, and it is a
// member of this family for one reason: SILENCE CARRIES NO INFORMATION ON
// THIS PAIR.
//
// Both books say the NAK is unreliable (590:106-108, 480:136-138), so a
// read that times out is neither "the thing is absent" nor "the radio
// refused". The session read fails WHOLE, with no retry and NO INFERENCE OF
// ABSENCE — and the message says that in as many words, because the
// inference is the one a reader will otherwise make. (The FT-891's
// "?;-means-absent" discovery is unavailable here for the same reason and
// one more: this family's slot space is printed, so nothing needs
// discovering.)
type TimeoutError struct {
	// Book is the document quoted.
	Book Book
	// Command is the frame that went unanswered.
	Command string
}

func (e *TimeoutError) Error() string {
	cite := "590:106-108"
	if e.Book == Book480 {
		cite = "480:136-138"
	}
	return fmt.Sprintf("kw: no answer to %q within the read timeout: this is not evidence of absence and not a refusal — %s (%s), so silence carries no information; the session read fails whole and is not retried",
		e.Command, transientSentence, cite)
}

// Unwrap lets errors.Is(err, transport.ErrTimeout) match.
func (e *TimeoutError) Unwrap() error { return transport.ErrTimeout }

// NewTimeoutError builds the typed timeout for command as book prints it.
func NewTimeoutError(book Book, command string) *TimeoutError {
	return &TimeoutError{Book: book, Command: command}
}
