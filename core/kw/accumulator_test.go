// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import (
	"errors"
	"strings"
	"testing"
)

// TestFrameAccumulator_ReassemblesAcrossChunkBoundaries pins the whole point
// of the type: a serial read may split one frame, coalesce several, or
// fragment in any other way, and the frames that come out are the same
// either way.
func TestFrameAccumulator_ReassemblesAcrossChunkBoundaries(t *testing.T) {
	a := NewFrameAccumulator(0)
	var got []string
	for _, chunk := range []string{"MR", "0007", "00014250000", "02000000", "0000000000000", "000MEMNAME1;ID0"} {
		frames, err := a.Push([]byte(chunk))
		if err != nil {
			t.Fatalf("Push(%q): unexpected error: %v", chunk, err)
		}
		for _, f := range frames {
			got = append(got, string(f))
		}
	}
	if len(got) != 1 {
		t.Fatalf("got %d frames, want 1: %q", len(got), got)
	}
	if len(got[0]) != 50 {
		t.Errorf("frame is %d bytes, want the 50-byte MR answer: %q", len(got[0]), got[0])
	}
	frames, err := a.Push([]byte("23;"))
	if err != nil {
		t.Fatalf("Push: unexpected error: %v", err)
	}
	if len(frames) != 1 || string(frames[0]) != "ID023;" {
		t.Errorf("got %q, want one frame \"ID023;\"", frames)
	}
}

// TestFrameAccumulator_ReturnedFramesNeverAliasTheChunk pins the copy
// contract transport.Accumulator requires: a caller may mutate what it got
// back, or the chunk it pushed, with no effect on either.
func TestFrameAccumulator_ReturnedFramesNeverAliasTheChunk(t *testing.T) {
	a := NewFrameAccumulator(0)
	chunk := []byte("ID023;")
	frames, err := a.Push(chunk)
	if err != nil {
		t.Fatalf("Push: unexpected error: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("got %d frames, want 1", len(frames))
	}
	chunk[0] = 'X'
	if string(frames[0]) != "ID023;" {
		t.Errorf("mutating the pushed chunk changed the returned frame: %q", frames[0])
	}
	frames[0][0] = 'Z'
	more, err := a.Push([]byte("ID020;"))
	if err != nil {
		t.Fatalf("Push: unexpected error: %v", err)
	}
	if string(more[0]) != "ID020;" {
		t.Errorf("mutating a returned frame disturbed a later one: %q", more[0])
	}
}

// TestFrameAccumulator_BoundedAccumulation pins both violation shapes — a run
// that never terminates, and a frame that terminates only after growing past
// the bound — and the io.Reader-shaped contract that frames found BEFORE the
// violation are still returned.
func TestFrameAccumulator_BoundedAccumulation(t *testing.T) {
	t.Run("never terminates", func(t *testing.T) {
		a := NewFrameAccumulator(16)
		_, err := a.Push([]byte(strings.Repeat("x", 17)))
		var tooLong *FrameTooLongError
		if !errors.As(err, &tooLong) {
			t.Fatalf("Push error = %v, want *FrameTooLongError", err)
		}
		if !errors.Is(err, ErrFrameTooLong) {
			t.Errorf("errors.Is(err, ErrFrameTooLong) = false, want true")
		}
		if tooLong.DiscardedLen != 17 {
			t.Errorf("DiscardedLen = %d, want 17", tooLong.DiscardedLen)
		}
	})
	t.Run("terminates past the bound, frames before it survive", func(t *testing.T) {
		a := NewFrameAccumulator(8)
		frames, err := a.Push([]byte("ID023;" + strings.Repeat("y", 9) + ";"))
		if err == nil {
			t.Fatal("Push returned no error for an over-long frame")
		}
		if len(frames) != 1 || string(frames[0]) != "ID023;" {
			t.Errorf("got frames %q, want the in-bound \"ID023;\" returned alongside the error", frames)
		}
	})
	t.Run("resets itself, so the next Push starts clean", func(t *testing.T) {
		a := NewFrameAccumulator(8)
		if _, err := a.Push([]byte(strings.Repeat("z", 9))); err == nil {
			t.Fatal("Push returned no error")
		}
		frames, err := a.Push([]byte("ID023;"))
		if err != nil {
			t.Fatalf("Push after a violation: unexpected error: %v", err)
		}
		if len(frames) != 1 || string(frames[0]) != "ID023;" {
			t.Errorf("got %q, want a clean \"ID023;\"", frames)
		}
	})
}

// TestNewFrameAccumulator_NonPositiveMaxSelectsTheDefault pins the one
// defaulting rule this type has.
func TestNewFrameAccumulator_NonPositiveMaxSelectsTheDefault(t *testing.T) {
	for _, max := range []int{0, -1} {
		a := NewFrameAccumulator(max)
		if a.max != DefaultMaxFrame {
			t.Errorf("NewFrameAccumulator(%d).max = %d, want DefaultMaxFrame (%d)", max, a.max, DefaultMaxFrame)
		}
	}
}

// TestDefaultMaxFrame_ExceedsTheWidestPrintedFrame is the ONLY relation the
// bound's comment claims, pinned rather than asserted in prose: the widest
// frame either book prints is the 50-byte MR answer / MW set
// (590:1440-1461, 590:1518-1536; 480:923-943, 480:955-976), and 256 clears
// it.
//
// NO ARITHMETIC IS PINNED HERE BECAUSE NONE IS CLAIMED. 256 is the fleet's
// shared frame bound, adequate for this family rather than derived from it;
// see accumulator.go. A test asserting some multiple of 50 would be pinning
// a derivation that does not exist.
func TestDefaultMaxFrame_ExceedsTheWidestPrintedFrame(t *testing.T) {
	const widestPrintedFrame = 50
	if DefaultMaxFrame <= widestPrintedFrame {
		t.Errorf("DefaultMaxFrame = %d, want more than the 50-byte MR/MW frame", DefaultMaxFrame)
	}
}
