// SPDX-License-Identifier: GPL-3.0-or-later

package cat

// DefaultMaxFrame is the maximum frame length, in bytes, FrameAccumulator
// enforces when NewFrameAccumulator is given a non-positive maxFrame.
const DefaultMaxFrame = 256

// FrameAccumulator reassembles ';'-terminated CAT frames (see SplitFrames)
// from arbitrary stream chunks — e.g. successive reads from a serial port,
// which may split a frame across reads, coalesce several frames into one
// read, or deliver any other fragmentation — while enforcing a maximum
// frame length so that a noisy or wedged serial line cannot grow its
// internal buffer without bound.
//
// The zero value is not usable; construct one with NewFrameAccumulator. A
// FrameAccumulator is not safe for concurrent use.
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
// every complete, terminator-inclusive frame now available (reusing
// SplitFrames internally). Each returned frame is an independent copy: it
// never aliases chunk, any previously-pushed chunk, or the accumulator's
// own retained buffer, so a caller is free to mutate a returned frame, or
// chunk itself, without any effect on the accumulator or on frames
// returned by earlier or later calls.
//
// Bounded accumulation: once more than maxFrame bytes have accumulated
// without ever forming a legitimate, in-bound frame, Push treats this as
// stream contamination. This covers two cases identically — a run of
// chunks that never reaches a ';' terminator (the classic "noisy or wedged
// line" case), and a single frame that DOES reach a ';' but only after
// growing past maxFrame bytes first (a legitimate frame never exceeds the
// cap, so such a frame is invalid regardless of its terminator). In either
// case Push:
//   - returns any complete, in-bound frames it found BEFORE the violation
//     (as with io.Reader, a caller must process a non-empty frames slice
//     even when err is also non-nil — do not discard frames just because
//     err != nil);
//   - stops processing at the first violation and discards everything
//     from that point to the end of the currently accumulated buffer,
//     including any further bytes in the same chunk that might otherwise
//     look like valid frames — once a stream is known contaminated,
//     resynchronising on what looks like the next frame boundary is not
//     safe, since corrupt data can coincidentally resemble a valid frame;
//   - returns a *FrameTooLongError recording exactly how many trailing
//     bytes were discarded (Unwrap()-compatible with errors.Is(err,
//     ErrFrameTooLong));
//   - resets its internal buffer to empty.
//
// Once Push returns a *FrameTooLongError, the caller (the future
// transport) must treat the underlying stream as contaminated from that
// point on and should drain it to a quiet boundary before trusting
// subsequent frames — FrameAccumulator itself starts completely fresh on
// the next Push, so it does not need to be recreated.
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
