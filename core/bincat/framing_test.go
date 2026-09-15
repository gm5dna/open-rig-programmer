// SPDX-License-Identifier: GPL-3.0-or-later

package bincat

import (
	"bytes"
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/transport"
)

func TestNewFramingRefusesUnconfiguredProfile(t *testing.T) {
	if _, err := NewFraming(Profile{}); !errors.Is(err, ErrInvalidProfile) {
		t.Fatalf("NewFraming(zero Profile) error = %v, want ErrInvalidProfile", err)
	}
	if _, err := NewFraming(testProfile()); err != nil {
		t.Fatalf("NewFraming(testProfile()) = %v, want no error", err)
	}
}

// TestNoteSentBeforeNewAccumulator pins the constructor/reader startup
// race being closed (plan.md Phase 1, Codex #3): the accumulator is built
// inside NewFraming itself, so a NoteSent call reaching it before the
// engine's reader goroutine ever calls NewAccumulator must not panic or
// silently drop the note — the reply, once NewAccumulator IS called and
// fed bytes, is still recognised at the length NoteSent recorded.
func TestNoteSentBeforeNewAccumulator(t *testing.T) {
	p := testProfile()
	f, err := NewFraming(p)
	if err != nil {
		t.Fatalf("NewFraming: %v", err)
	}

	req := BuildFrame(OpStatusUpdate, [4]byte{UOperatingData, 0, 0, 0})
	f.NoteSent(req) // before NewAccumulator is ever called

	acc := f.NewAccumulator(0)
	reply := bytes.Repeat([]byte{0xAB}, 19)
	frames, err := acc.Push(reply)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if len(frames) != 1 || !bytes.Equal(frames[0], reply) {
		t.Fatalf("Push(% x) = %v, want one 19-byte frame", reply, frames)
	}
}

func TestNewAccumulatorCalledTwicePanics(t *testing.T) {
	f, err := NewFraming(testProfile())
	if err != nil {
		t.Fatalf("NewFraming: %v", err)
	}
	f.NewAccumulator(0)

	defer func() {
		if recover() == nil {
			t.Fatal("second NewAccumulator call did not panic")
		}
	}()
	f.NewAccumulator(0)
}

func TestAccumulatorSplitFrame(t *testing.T) {
	f, err := NewFraming(testProfile())
	if err != nil {
		t.Fatalf("NewFraming: %v", err)
	}
	f.NoteSent(BuildFrame(OpStatusUpdate, [4]byte{UOperatingData, 0, 0, 0}))
	acc := f.NewAccumulator(0)

	reply := bytes.Repeat([]byte{0x11}, 19)
	frames, err := acc.Push(reply[:10])
	if err != nil {
		t.Fatalf("first Push: %v", err)
	}
	if len(frames) != 0 {
		t.Fatalf("first Push (10 of 19 bytes) returned %d frames, want 0", len(frames))
	}

	frames, err = acc.Push(reply[10:])
	if err != nil {
		t.Fatalf("second Push: %v", err)
	}
	if len(frames) != 1 || !bytes.Equal(frames[0], reply) {
		t.Fatalf("second Push completed the split frame as %v, want one copy of % x", frames, reply)
	}
}

func TestAccumulatorCoalescedFrames(t *testing.T) {
	f, err := NewFraming(testProfile())
	if err != nil {
		t.Fatalf("NewFraming: %v", err)
	}
	// Two outstanding wants noted back to back — defensive coverage: the
	// engine holds one request at a time in normal operation, but the
	// queue must still behave correctly if more than one was noted.
	f.NoteSent(BuildFrame(OpStatusUpdate, [4]byte{UMemoryNumber, 0, 0, 0}))  // 1 byte
	f.NoteSent(BuildFrame(OpStatusUpdate, [4]byte{UOperatingData, 0, 0, 0})) // 19 bytes
	acc := f.NewAccumulator(0)

	one := []byte{0x42}
	nineteen := bytes.Repeat([]byte{0x99}, 19)
	chunk := append(append([]byte{}, one...), nineteen...)

	frames, err := acc.Push(chunk)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if len(frames) != 2 {
		t.Fatalf("Push(coalesced chunk) returned %d frames, want 2", len(frames))
	}
	if !bytes.Equal(frames[0], one) || !bytes.Equal(frames[1], nineteen) {
		t.Fatalf("Push(coalesced chunk) = %v, want [% x, % x]", frames, one, nineteen)
	}
}

func TestAccumulatorReturnsIndependentCopies(t *testing.T) {
	f, err := NewFraming(testProfile())
	if err != nil {
		t.Fatalf("NewFraming: %v", err)
	}
	f.NoteSent(BuildFrame(OpStatusUpdate, [4]byte{UMemoryNumber, 0, 0, 0}))
	acc := f.NewAccumulator(0)

	chunk := []byte{0x55}
	frames, err := acc.Push(chunk)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("got %d frames, want 1", len(frames))
	}
	chunk[0] = 0xFF // mutate the caller's own slice after Push returns
	if frames[0][0] == 0xFF {
		t.Fatal("returned frame aliased the caller's chunk slice")
	}

	frames[0][0] = 0x00 // mutate the returned frame
	frames2, err := acc.Push([]byte{0x77})
	if err != nil {
		t.Fatalf("second Push: %v", err)
	}
	if len(frames2) == 1 && frames2[0][0] == 0x00 {
		t.Fatal("mutating a previously returned frame affected the accumulator's own state")
	}
}

func TestAccumulatorFrameTooLong(t *testing.T) {
	p := testProfile()
	p.FullDumpLen = 8 // small MaxFrame, easy to exceed in a test
	f, err := NewFraming(p)
	if err != nil {
		t.Fatalf("NewFraming: %v", err)
	}
	acc := f.NewAccumulator(0)

	// No NoteSent: bytes arrive with nothing expected — noise that must
	// still be bounded rather than buffered without limit.
	_, err = acc.Push(bytes.Repeat([]byte{0x00}, 20))
	var tooLong *transport.FrameTooLongError
	if !errors.As(err, &tooLong) {
		t.Fatalf("Push(20 bytes over an 8-byte MaxFrame) error = %v, want *transport.FrameTooLongError", err)
	}
	if !errors.Is(err, transport.ErrFrameTooLong) {
		t.Fatalf("errors.Is(err, transport.ErrFrameTooLong) = false")
	}
	if tooLong.DiscardedLen != 20 {
		t.Fatalf("DiscardedLen = %d, want 20", tooLong.DiscardedLen)
	}
}

func TestFramingIsRejectionAlwaysFalse(t *testing.T) {
	f, err := NewFraming(testProfile())
	if err != nil {
		t.Fatalf("NewFraming: %v", err)
	}
	for _, frame := range [][]byte{nil, {}, BuildFrame(OpStore, [4]byte{1, 0, 0, 0})} {
		if f.IsRejection(frame) {
			t.Errorf("IsRejection(% x) = true, want false (this family has no NAK)", frame)
		}
	}
}

func TestFramingInitSequenceEmpty(t *testing.T) {
	f, err := NewFraming(testProfile())
	if err != nil {
		t.Fatalf("NewFraming: %v", err)
	}
	if seq := f.InitSequence(); len(seq) != 0 {
		t.Fatalf("InitSequence() = %v, want empty", seq)
	}
}

// TestFramingDisallowedCommandStillRefusedAfterNoteSent pins that NoteSent
// (called before Allow in core/transport's Do — engine.go:723-736) does not
// change Allow's verdict: a frame the gate would refuse is still refused
// even though it has already been noted.
func TestFramingDisallowedCommandStillRefusedAfterNoteSent(t *testing.T) {
	f, err := NewFraming(testProfile())
	if err != nil {
		t.Fatalf("NewFraming: %v", err)
	}
	disallowed := BuildFrame(OpStore, [4]byte{99, 0, 0, 0}) // channel out of range
	f.NoteSent(disallowed)
	if f.Allow(disallowed) {
		t.Fatal("Allow admitted a disallowed frame after NoteSent had already recorded it")
	}
}

func TestNoteSentDoesNotRetainOrAliasFrame(t *testing.T) {
	f, err := NewFraming(testProfile())
	if err != nil {
		t.Fatalf("NewFraming: %v", err)
	}
	frame := BuildFrame(OpStatusUpdate, [4]byte{UMemoryNumber, 0, 0, 0})
	original := append([]byte(nil), frame...)
	f.NoteSent(frame)
	frame[0] = 0xFF // mutate after NoteSent returns

	acc := f.NewAccumulator(0)
	frames, err := acc.Push([]byte{0x01})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("mutating the frame after NoteSent changed what was recorded: got %d frames, want 1 (original request was %x)", len(frames), original)
	}
}
