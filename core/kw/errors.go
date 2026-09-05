// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import (
	"errors"
	"fmt"
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
