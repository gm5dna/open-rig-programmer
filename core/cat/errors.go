// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"errors"
	"fmt"
)

// ErrRejected is the error a caller should compare against (via
// errors.Is) when the radio replies with the literal "?;" frame.
//
// Per the reference, this is the radio's ONLY NAK: an unattributed generic
// command failure. It is returned for unknown commands, bad parameters,
// wrong radio state, an empty memory slot, and anything else that goes
// wrong — the wire protocol gives no way to tell these apart. Do not
// attempt to infer, wrap, or report a more specific cause from this error;
// there is none to infer.
var ErrRejected = errors.New(`cat: radio rejected command ("?;")`)

// ErrFrameTooLong is the sentinel a caller should compare against (via
// errors.Is) when FrameAccumulator.Push reports that stream data exceeded
// the configured maximum frame length without ever forming a legitimate,
// in-bound frame. Every error Push returns for this condition is a
// *FrameTooLongError, whose Unwrap returns this sentinel, and which
// additionally carries DiscardedLen — the number of bytes thrown away —
// for callers that want more than identity (logging, telemetry).
var ErrFrameTooLong = errors.New("cat: frame exceeded maximum length")

// FrameTooLongError reports that FrameAccumulator.Push discarded
// DiscardedLen bytes of accumulated stream data because they could not
// form a legitimate frame within the accumulator's configured maxFrame
// bound — either no terminator arrived before the bound was reached, or a
// terminator did arrive but only after the frame had already grown past
// the bound (a legitimate frame never exceeds the cap).
//
// This means the underlying serial line is noisy, wedged, or sending
// something that is not this protocol. FrameAccumulator resets its
// internal buffer when this happens: the caller (the future transport)
// must treat everything from this point as contaminated and should drain
// the line to a quiet boundary before trusting subsequent frames again —
// see FrameAccumulator.Push's doc comment.
type FrameTooLongError struct {
	// DiscardedLen is the number of bytes thrown away.
	DiscardedLen int
}

// Error implements the error interface.
func (e *FrameTooLongError) Error() string {
	return fmt.Sprintf("cat: frame exceeded maximum length, discarded %d bytes", e.DiscardedLen)
}

// Unwrap lets errors.Is(err, ErrFrameTooLong) match, alongside
// errors.As(err, &frameTooLongErr) for callers that want DiscardedLen too.
func (e *FrameTooLongError) Unwrap() error { return ErrFrameTooLong }

// maxParseErrorFrameLen bounds how much offending input a ParseError will
// retain. It exists so that malformed or hostile input (e.g. a corrupt
// stream, or a fuzzer) cannot make error messages or logs unbounded in
// size.
const maxParseErrorFrameLen = 64

// ParseError reports that some CAT wire value — a frame, a slot code, a
// mode nibble — did not match any explicitly valid form. Codecs in this
// package favour strictness: anything not explicitly valid is rejected
// with a ParseError rather than guessed at.
//
// Frame holds a defensive copy of the offending input, truncated to
// maxParseErrorFrameLen bytes. Reason is a short, human-readable
// explanation of what was wrong.
type ParseError struct {
	Frame  []byte
	Reason string
}

// Error implements the error interface.
func (e *ParseError) Error() string {
	return fmt.Sprintf("cat: parse error: %s (input=%q)", e.Reason, e.Frame)
}

// newParseError builds a ParseError from the offending input, copying and
// truncating it so the returned error never aliases caller-owned memory and
// never grows unbounded.
func newParseError(input []byte, reason string) *ParseError {
	n := len(input)
	if n > maxParseErrorFrameLen {
		n = maxParseErrorFrameLen
	}
	frame := make([]byte, n)
	copy(frame, input[:n])
	return &ParseError{Frame: frame, Reason: reason}
}
