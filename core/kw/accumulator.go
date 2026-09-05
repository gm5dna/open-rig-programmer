// SPDX-License-Identifier: GPL-3.0-or-later

package kw

// DefaultMaxFrame is the maximum frame length, in bytes, FrameAccumulator
// enforces when NewFrameAccumulator is given a non-positive maxFrame.
//
// A RESOURCE bound, not a protocol fact, and this package's OWN — the third
// independent one, beside core/cat's and core/civ's, because the fence
// forbids reading either of theirs and because the arithmetic below is
// Kenwood's.
//
// The widest frame either book prints is the 50-byte MR answer / MW set
// (590:1440-1461, 590:1518-1536; 480:923-943, 480:955-976). Every other
// frame this milestone builds or parses is far narrower: ID read 3 and
// answer 6, FV read 3 and answer 7, TY read 3 and answer 6, MC read 3 and
// Set/Answer 6, MR read 7, EX read 10. The ONE frame with no printed
// ceiling is the EX ANSWER, whose P5 is declared "variable length" with no
// upper bound anywhere (590:555-556, 480:409-411) — recorded as A19, whose
// lift is an exhaustive EX sweep. So the bound cannot be derived from the
// documents; it is chosen, at five times the widest printed frame, which
// leaves ample room for any P5 the parameter lists print while still
// refusing to buffer a wedged or noisy line without limit.
//
// It is also the datum MaxEXDigits is derived from (exdigits.go), which is
// why it is stated here once rather than in two places.
const DefaultMaxFrame = 256

// FrameAccumulator reassembles ';'-terminated Kenwood frames (see
// SplitFrames) from arbitrary stream chunks — successive reads from a
// serial port, which may split one frame across reads, coalesce several
// into one read, or fragment in any other way — while enforcing a maximum
// frame length so that a noisy or wedged serial line cannot grow its
// internal buffer without bound.
//
// The zero value is not usable; construct one with NewFrameAccumulator. A
// FrameAccumulator is not safe for concurrent use — the engine's reader
// goroutine owns exactly one and is the only goroutine that touches it.
type FrameAccumulator struct {
	buf []byte
	max int
}

// NewFrameAccumulator returns a FrameAccumulator that treats more than
// maxFrame accumulated bytes without completing a legitimate, in-bound
// frame as stream contamination (see Push). maxFrame <= 0 selects
// DefaultMaxFrame.
func NewFrameAccumulator(maxFrame int) *FrameAccumulator {
	if maxFrame <= 0 {
		maxFrame = DefaultMaxFrame
	}
	return &FrameAccumulator{max: maxFrame}
}

// Push appends chunk to the accumulator's internal buffer and extracts
// every complete, terminator-inclusive frame now available. Each returned
// frame is an independent copy: it never aliases chunk, any previously
// pushed chunk, or the accumulator's own retained buffer.
//
// Bounded accumulation, on core/cat's and core/civ's terms and for the same
// reason. Once more than maxFrame bytes have accumulated without ever
// forming a legitimate, in-bound frame — whether because no terminator
// arrived, or because one arrived only after the frame had grown past the
// bound — Push:
//   - returns any complete, in-bound frames found BEFORE the violation (as
//     with io.Reader, a caller must process a non-empty frames slice even
//     when err is also non-nil);
//   - discards everything from the violation to the end of the buffer,
//     including bytes that might look like valid frames: once a stream is
//     known contaminated, resynchronising on what merely resembles a frame
//     boundary is not safe;
//   - returns a *FrameTooLongError recording how many bytes were discarded
//     (errors.Is-compatible with ErrFrameTooLong);
//   - resets its buffer, so the next Push starts clean.
//
// THE ERROR TYPE IS THIS PACKAGE'S and the framing adapter TRANSLATES it
// onto core/transport's at the seam — see framing.go's Push. That is not
// cosmetic: Engine.handleReaderErr treats a *transport.FrameTooLongError as
// recoverable CONTAMINATION and anything else as a dead port.
func (a *FrameAccumulator) Push(chunk []byte) (frames [][]byte, err error) {
	buf := append(a.buf, chunk...)

	split, rest := SplitFrames(buf)

	consumed := 0
	for _, raw := range split {
		if len(raw) > a.max {
			discarded := len(buf) - consumed
			a.buf = nil
			return frames, &FrameTooLongError{DiscardedLen: discarded}
		}
		frames = append(frames, copyBytes(raw))
		consumed += len(raw)
	}

	if len(rest) > a.max {
		a.buf = nil
		return frames, &FrameTooLongError{DiscardedLen: len(rest)}
	}

	a.buf = copyBytes(rest)
	return frames, nil
}

// copyBytes returns an independent copy of b, so the result never aliases
// b's backing array.
func copyBytes(b []byte) []byte {
	out := make([]byte, len(b))
	copy(out, b)
	return out
}
