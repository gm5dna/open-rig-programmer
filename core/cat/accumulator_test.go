// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"errors"
	"strings"
	"testing"
)

// assertAccumulatorFrames compares got against want, both as ordered lists
// of frame strings.
func assertAccumulatorFrames(t *testing.T, got [][]byte, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("frames = %v, want %v", framesAsStrings(got), want)
	}
	for i, w := range want {
		if string(got[i]) != w {
			t.Errorf("frames[%d] = %q, want %q", i, got[i], w)
		}
	}
}

// --- NewFrameAccumulator ---

func TestNewFrameAccumulator_NonPositiveMaxUsesDefault(t *testing.T) {
	for _, m := range []int{0, -1, -100} {
		acc := NewFrameAccumulator(m)
		if acc.max != DefaultMaxFrame {
			t.Errorf("NewFrameAccumulator(%d).max = %d, want DefaultMaxFrame (%d)", m, acc.max, DefaultMaxFrame)
		}
	}
}

func TestNewFrameAccumulator_PositiveMaxIsRespected(t *testing.T) {
	acc := NewFrameAccumulator(16)
	if acc.max != 16 {
		t.Errorf("NewFrameAccumulator(16).max = %d, want 16", acc.max)
	}
}

// --- Push: normal reassembly ---

// TestFrameAccumulator_SingleChunkMultipleFrames: several complete frames
// arriving in a single Push, mirroring SplitFrames' own multi-frame case
// (frame_test.go), but through the accumulator's Push API.
func TestFrameAccumulator_SingleChunkMultipleFrames(t *testing.T) {
	acc := NewFrameAccumulator(64)
	frames, err := acc.Push([]byte("ID0800;AI0;?;"))
	if err != nil {
		t.Fatalf("Push: unexpected error: %v", err)
	}
	assertAccumulatorFrames(t, frames, []string{"ID0800;", "AI0;", "?;"})
}

// TestFrameAccumulator_PartialThenComplete: an unterminated chunk buffers
// with no frames returned, and the following chunk that completes it
// yields exactly the reassembled frame.
func TestFrameAccumulator_PartialThenComplete(t *testing.T) {
	acc := NewFrameAccumulator(64)

	frames, err := acc.Push([]byte("ID08"))
	if err != nil {
		t.Fatalf("Push(partial): unexpected error: %v", err)
	}
	if len(frames) != 0 {
		t.Fatalf("Push(partial) frames = %v, want none", framesAsStrings(frames))
	}

	frames, err = acc.Push([]byte("00;"))
	if err != nil {
		t.Fatalf("Push(rest): unexpected error: %v", err)
	}
	assertAccumulatorFrames(t, frames, []string{"ID0800;"})
}

// TestFrameAccumulator_FramesPlusRestContinuityAcrossPushes: a frame split
// across three chunks, immediately followed in the last chunk by the start
// of the NEXT frame, must reassemble correctly across the whole sequence.
func TestFrameAccumulator_FramesPlusRestContinuityAcrossPushes(t *testing.T) {
	acc := NewFrameAccumulator(64)

	var got [][]byte
	for _, chunk := range []string{"ID08", "00;AI", "0;"} {
		frames, err := acc.Push([]byte(chunk))
		if err != nil {
			t.Fatalf("Push(%q): unexpected error: %v", chunk, err)
		}
		got = append(got, frames...)
	}
	assertAccumulatorFrames(t, got, []string{"ID0800;", "AI0;"})
}

// TestFrameAccumulator_ByteAtATimeReassembly simulates a stream arriving
// one byte at a time, mirroring frame_test.go's
// TestSplitFrames_ByteAtATimeReassembly but through the accumulator.
func TestFrameAccumulator_ByteAtATimeReassembly(t *testing.T) {
	full := []byte("ID0800;AI0;?;MC099;")
	wantFrames := []string{"ID0800;", "AI0;", "?;", "MC099;"}

	acc := NewFrameAccumulator(64)
	var got [][]byte
	for i := 0; i < len(full); i++ {
		frames, err := acc.Push(full[i : i+1])
		if err != nil {
			t.Fatalf("Push(byte %d): unexpected error: %v", i, err)
		}
		got = append(got, frames...)
	}
	assertAccumulatorFrames(t, got, wantFrames)
}

// TestFrameAccumulator_ReturnedFramesAreIndependentCopies: neither the
// caller's original chunk slice nor a previously-returned frame may alias
// the accumulator's internal state or a later Push's output.
func TestFrameAccumulator_ReturnedFramesAreIndependentCopies(t *testing.T) {
	acc := NewFrameAccumulator(64)
	chunk := []byte("AI0;")
	frames, err := acc.Push(chunk)
	if err != nil {
		t.Fatalf("Push: unexpected error: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("frames = %v, want 1 frame", framesAsStrings(frames))
	}

	chunk[0] = 'X' // mutate the caller's original chunk after Push returns
	if string(frames[0]) != "AI0;" {
		t.Errorf("returned frame = %q, want %q (must not alias caller's chunk)", frames[0], "AI0;")
	}

	frames[0][0] = 'Z' // mutate the returned frame itself
	frames2, err := acc.Push([]byte("MC099;"))
	if err != nil {
		t.Fatalf("Push: unexpected error: %v", err)
	}
	assertAccumulatorFrames(t, frames2, []string{"MC099;"})
}

// --- Push: bounded accumulation (ErrFrameTooLong / FrameTooLongError) ---

// TestFrameAccumulator_UnterminatedExceedsLimit_MultiChunk: a stream that
// never produces a ';' must error, and reset, once the accumulated length
// exceeds maxFrame — exactly at the boundary (== max) is still tolerated,
// since the very next byte could be the terminator.
func TestFrameAccumulator_UnterminatedExceedsLimit_MultiChunk(t *testing.T) {
	acc := NewFrameAccumulator(10)

	if frames, err := acc.Push([]byte("AAAAA")); err != nil || len(frames) != 0 {
		t.Fatalf("Push(5 bytes, no term) = %v, %v, want (none, nil)", framesAsStrings(frames), err)
	}
	if frames, err := acc.Push([]byte("AAAAA")); err != nil || len(frames) != 0 {
		t.Fatalf("Push(10 bytes total, no term, at boundary) = %v, %v, want (none, nil)", framesAsStrings(frames), err)
	}

	frames, err := acc.Push([]byte("A"))
	if err == nil {
		t.Fatal("Push(11 bytes total, no term): want error")
	}
	if len(frames) != 0 {
		t.Errorf("Push(11 bytes, over limit) frames = %v, want none", framesAsStrings(frames))
	}
	var fe *FrameTooLongError
	if !errors.As(err, &fe) {
		t.Fatalf("Push error is %T, want *FrameTooLongError", err)
	}
	if fe.DiscardedLen != 11 {
		t.Errorf("FrameTooLongError.DiscardedLen = %d, want 11", fe.DiscardedLen)
	}
	if !errors.Is(err, ErrFrameTooLong) {
		t.Errorf("errors.Is(err, ErrFrameTooLong) = false, want true")
	}

	// State must have reset: a subsequent, well-formed, in-bound frame
	// parses cleanly with no leftover contamination from the overflow.
	frames, err = acc.Push([]byte("ID0800;"))
	if err != nil {
		t.Fatalf("Push after reset: unexpected error: %v", err)
	}
	assertAccumulatorFrames(t, frames, []string{"ID0800;"})
}

// TestFrameAccumulator_OversizedTerminatedFrame_SingleChunk: a single
// frame that DOES reach a ';' but is, by itself, longer than maxFrame must
// still be rejected — a legitimate frame never exceeds the cap.
func TestFrameAccumulator_OversizedTerminatedFrame_SingleChunk(t *testing.T) {
	acc := NewFrameAccumulator(8)
	frame := "TOOLONGX;" // 9 bytes > max(8)
	if len(frame) != 9 {
		t.Fatalf("test fixture is %d bytes, want 9", len(frame))
	}

	frames, err := acc.Push([]byte(frame))
	if err == nil {
		t.Fatal("Push(oversized terminated frame): want error")
	}
	if len(frames) != 0 {
		t.Errorf("frames = %v, want none", framesAsStrings(frames))
	}
	var fe *FrameTooLongError
	if !errors.As(err, &fe) {
		t.Fatalf("Push error is %T, want *FrameTooLongError", err)
	}
	if fe.DiscardedLen != 9 {
		t.Errorf("DiscardedLen = %d, want 9", fe.DiscardedLen)
	}
}

// TestFrameAccumulator_OversizedFrame_PreservesEarlierValidFrames: when a
// chunk contains a legitimate, in-bound frame followed by an oversized one,
// the legitimate frame is still returned (callers must process a non-empty
// frames slice even alongside a non-nil err, as with io.Reader), and only
// the oversized frame's bytes are counted as discarded.
func TestFrameAccumulator_OversizedFrame_PreservesEarlierValidFrames(t *testing.T) {
	acc := NewFrameAccumulator(8)
	oversized := strings.Repeat("X", 10) + ";" // 11 bytes > max(8)
	frames, err := acc.Push([]byte("AI0;" + oversized))
	if err == nil {
		t.Fatal("Push: want error (second frame exceeds max)")
	}
	assertAccumulatorFrames(t, frames, []string{"AI0;"})

	var fe *FrameTooLongError
	if !errors.As(err, &fe) {
		t.Fatalf("Push error is %T, want *FrameTooLongError", err)
	}
	if fe.DiscardedLen != len(oversized) {
		t.Errorf("DiscardedLen = %d, want %d (only the oversized frame)", fe.DiscardedLen, len(oversized))
	}
}

// TestFrameAccumulator_StopsAtFirstViolation_DiscardsRemainder: once an
// oversized frame is found, any further bytes in the SAME chunk — even a
// well-formed-looking frame after it — are discarded too. Once a stream is
// known contaminated, treating the remainder as trustworthy is unsafe: a
// resync could just as easily be a coincidental garbage byte sequence.
func TestFrameAccumulator_StopsAtFirstViolation_DiscardsRemainder(t *testing.T) {
	acc := NewFrameAccumulator(8)
	oversized := strings.Repeat("X", 10) + ";" // 11 bytes
	chunk := "AI0;" + oversized + "MC099;"

	frames, err := acc.Push([]byte(chunk))
	if err == nil {
		t.Fatal("Push: want error")
	}
	assertAccumulatorFrames(t, frames, []string{"AI0;"})

	var fe *FrameTooLongError
	if !errors.As(err, &fe) {
		t.Fatalf("Push error is %T, want *FrameTooLongError", err)
	}
	wantDiscarded := len(oversized) + len("MC099;")
	if fe.DiscardedLen != wantDiscarded {
		t.Errorf("DiscardedLen = %d, want %d (oversized frame + remainder of chunk)", fe.DiscardedLen, wantDiscarded)
	}
}

// TestFrameAccumulator_RestExceedsLimitWithPriorValidFrame: the
// "unterminated tail too long" violation path is distinct from the
// "oversized terminated frame" path — this exercises it when a valid
// leading frame is present in the same chunk.
func TestFrameAccumulator_RestExceedsLimitWithPriorValidFrame(t *testing.T) {
	acc := NewFrameAccumulator(8)
	tail := strings.Repeat("Y", 11) // 11 bytes, NO terminator
	frames, err := acc.Push([]byte("AI0;" + tail))
	if err == nil {
		t.Fatal("Push: want error (unterminated tail exceeds max)")
	}
	assertAccumulatorFrames(t, frames, []string{"AI0;"})

	var fe *FrameTooLongError
	if !errors.As(err, &fe) {
		t.Fatalf("Push error is %T, want *FrameTooLongError", err)
	}
	if fe.DiscardedLen != len(tail) {
		t.Errorf("DiscardedLen = %d, want %d", fe.DiscardedLen, len(tail))
	}
}

// FuzzFrameAccumulator requires that Push never panics, that the internal
// buffer never exceeds the configured max, that every returned frame is
// terminator-inclusive and within the size bound, and that across a whole
// sequence of Pushes, every pushed byte is accounted for exactly once as
// either part of a returned frame, part of a reported DiscardedLen, or
// still sitting in the internal buffer at the end (a prefix-consistent
// reconstruction of the pushed stream).
func FuzzFrameAccumulator(f *testing.F) {
	f.Add([]byte("ID0800;AI0;?;MC099;"), uint8(3))
	f.Add([]byte(";;;;;;"), uint8(1))
	f.Add([]byte(strings.Repeat("A", 50)), uint8(7)) // no terminator, exceeds a small cap
	f.Add([]byte("AI0;"+strings.Repeat("X", 100)+";"), uint8(11))
	f.Add([]byte(nil), uint8(0))

	f.Fuzz(func(t *testing.T, data []byte, chunkSize uint8) {
		const max = 32
		acc := NewFrameAccumulator(max)

		n := int(chunkSize)
		if n == 0 {
			n = 1
		}

		var pushed, accounted int
		for i := 0; i < len(data); i += n {
			end := i + n
			if end > len(data) {
				end = len(data)
			}
			chunk := data[i:end]
			pushed += len(chunk)

			frames, err := acc.Push(chunk)

			if len(acc.buf) > max {
				t.Fatalf("internal buffer length %d exceeds max %d after Push", len(acc.buf), max)
			}

			for _, fr := range frames {
				if len(fr) == 0 || fr[len(fr)-1] != ';' {
					t.Fatalf("Push returned frame %q not terminated with ';'", fr)
				}
				if len(fr) > max {
					t.Fatalf("Push returned frame %q longer than max %d", fr, max)
				}
				accounted += len(fr)
			}

			if err != nil {
				var fe *FrameTooLongError
				if !errors.As(err, &fe) {
					t.Fatalf("Push returned non-FrameTooLongError: %T (%v)", err, err)
				}
				if !errors.Is(err, ErrFrameTooLong) {
					t.Fatalf("errors.Is(err, ErrFrameTooLong) = false for %v", err)
				}
				accounted += fe.DiscardedLen
			}
		}
		accounted += len(acc.buf)

		if accounted != pushed {
			t.Fatalf("byte accounting mismatch: pushed %d bytes, accounted for %d (frames+discarded+buffered)", pushed, accounted)
		}
	})
}
