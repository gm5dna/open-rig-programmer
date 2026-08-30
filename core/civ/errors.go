// SPDX-License-Identifier: GPL-3.0-or-later

package civ

import (
	"errors"
	"fmt"
)

// THE ERROR VALUES IN THIS FILE ARE THIS PACKAGE'S OWN, DELIBERATELY, AND
// THE FRAMING ADAPTER RECONCILES THEM LATER.
//
// Spec D2 settles the cycle-free direction for the transport seam:
// core/transport RE-EXPORTS core/cat's ErrRejected and ErrFrameTooLong, and
// neutral callers use only the transport names. core/civ cannot USE those
// values as its own, because transport's canonical ones live in core/cat —
// a package this one must NEVER import, guarded both ways. So the values
// below are local and the ADAPTER maps them onto the transport names at
// the seam: lockedAccumulator.Push (framing.go) translates
// *FrameTooLongError into transport's, which is what makes Engine's
// handleReaderErr mark the stream CONTAMINATED rather than close the port.
//
// THERE IS NO ErrRejected HERE, and that is not an omission. A rejection is
// a WIRE CONDITION this package recognises (IsRejection, frame.go), not an
// error it raises: which error value a rejected command surfaces as belongs
// to the engine that issued the command, and spec D2 puts that error in
// core/transport. Minting a second sentinel here would create exactly the
// two-names-for-one-thing drift the re-export rule exists to prevent.

// ErrFrameTooLong is the sentinel to compare against (via errors.Is) when
// FrameAccumulator.Push reports that stream data exceeded the configured
// maximum frame length without forming an in-bound frame. Every error Push
// returns for this condition is a *FrameTooLongError.
var ErrFrameTooLong = errors.New("civ: frame exceeded maximum length")

// FrameTooLongError reports that FrameAccumulator.Push discarded
// DiscardedLen bytes because they could not form a legitimate frame within
// the accumulator's bound — either no terminator arrived, or one arrived
// only after the frame had already grown past the bound.
//
// It means the line is noisy, wedged, or carrying something that is not
// CI-V. The accumulator resets itself; the caller must treat everything
// from this point as contaminated and drain to a quiet boundary before
// trusting subsequent frames.
type FrameTooLongError struct {
	// DiscardedLen is the number of bytes thrown away.
	DiscardedLen int
}

func (e *FrameTooLongError) Error() string {
	return fmt.Sprintf("civ: frame exceeded maximum length, discarded %d bytes", e.DiscardedLen)
}

// Unwrap lets errors.Is(err, ErrFrameTooLong) match, alongside errors.As
// for callers that want DiscardedLen.
func (e *FrameTooLongError) Unwrap() error { return ErrFrameTooLong }

// ErrRecordLength is the sentinel for a memory record whose length is not
// in its profile's accepted set.
//
// Spec D4 (adjudication 13) makes this an ERROR rather than a partial
// parse: the read FAILS and ReadAll aborts honestly. There is no fake
// "Unavailable" channel to fall back on — the neutral seam has no such
// result shape — and a record of an unexpected length is evidence that the
// radio is not the model this profile describes, which is precisely what
// the probe's length fingerprint (spec D3.2) reads.
var ErrRecordLength = errors.New("civ: memory record length")

// RecordLengthError reports a record length outside the profile's accepted
// set, naming both the set and what arrived.
type RecordLengthError struct {
	// Want is the profile's accepted set, in ascending order.
	Want []int
	// Got is the length that arrived.
	Got int
	// Mode is the selected mode class when a mode-keyed layout required a
	// different length. Empty for the two length discriminators.
	Mode string
}

func (e *RecordLengthError) Error() string {
	if e.Mode != "" {
		return fmt.Sprintf("civ: memory record for mode %s is %d bytes, want one of %v", e.Mode, e.Got, e.Want)
	}
	return fmt.Sprintf("civ: memory record is %d bytes, want one of %v", e.Got, e.Want)
}

func (e *RecordLengthError) Unwrap() error { return ErrRecordLength }

// maxParseErrorFrameLen bounds how much offending input a ParseError
// retains, so malformed or hostile input cannot make a log line unbounded.
const maxParseErrorFrameLen = 64

// ErrParse is the sentinel every ParseError wraps.
var ErrParse = errors.New("civ: parse error")

// ParseError reports that some CI-V wire value did not match any
// explicitly valid form. This package favours strictness: anything not
// explicitly valid is refused rather than guessed at.
//
// Frame holds a defensive copy of the offending input, truncated, so the
// error never aliases caller memory and never grows without bound.
type ParseError struct {
	Frame  []byte
	Reason string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("civ: parse error: %s (input=%s)", e.Reason, hexFrame(e.Frame))
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

// ErrInvalidProfile is the sentinel every ProfileConfig validation failure
// wraps, so a caller can tell a malformed model table from any other
// error without matching on message text.
var ErrInvalidProfile = errors.New("civ: invalid profile")

// invalidProfile builds a validation failure naming the FIELD and the
// OFFENDING VALUE, wrapping ErrInvalidProfile.
//
// Naming both is a contract the tests assert on, not a nicety: a validator
// returning a generic error from the wrong branch passes any test that
// only checks for non-nil, and core/cat has shipped exactly that kind of
// silently-correct-looking check before (dialectvalidate.go).
func invalidProfile(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidProfile, fmt.Sprintf(format, args...))
}
