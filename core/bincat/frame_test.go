// SPDX-License-Identifier: GPL-3.0-or-later

package bincat

import (
	"bytes"
	"errors"
	"testing"
)

func TestBuildParseFrameRoundTrip(t *testing.T) {
	args := [4]byte{0x00, 0x50, 0x42, 0x01}
	frame := BuildFrame(OpSetFreq, args)
	if len(frame) != FrameLen {
		t.Fatalf("BuildFrame returned %d bytes, want %d", len(frame), FrameLen)
	}
	want := []byte{0x00, 0x50, 0x42, 0x01, OpSetFreq}
	if !bytes.Equal(frame, want) {
		t.Fatalf("BuildFrame(%#x, %v) = % x, want % x", OpSetFreq, args, frame, want)
	}

	opcode, gotArgs, err := ParseFrame(frame)
	if err != nil {
		t.Fatalf("ParseFrame: %v", err)
	}
	if opcode != OpSetFreq || gotArgs != args {
		t.Fatalf("ParseFrame(% x) = (%#x, %v), want (%#x, %v)", frame, opcode, gotArgs, OpSetFreq, args)
	}
}

func TestParseFrameWrongLength(t *testing.T) {
	for _, frame := range [][]byte{nil, {}, {1, 2, 3}, {1, 2, 3, 4, 5, 6}} {
		if _, _, err := ParseFrame(frame); !errors.Is(err, ErrFrame) {
			t.Errorf("ParseFrame(% x) error = %v, want ErrFrame", frame, err)
		}
	}
}

func TestBuildFrameDoesNotAliasArgs(t *testing.T) {
	args := [4]byte{1, 2, 3, 4}
	frame := BuildFrame(OpSetMode, args)
	frame[0] = 0xFF
	if args[0] != 1 {
		t.Fatal("mutating the returned frame changed the caller's args array")
	}
}

func TestCommandBytesIndependentCopies(t *testing.T) {
	c := NewCommand(OpStore, [4]byte{1, 0, 0, 0})
	a := c.Bytes()
	b := c.Bytes()
	a[0] = 0xFF
	if b[0] == 0xFF {
		t.Fatal("two Bytes() calls returned aliased slices")
	}
	if got := c.Bytes(); got[0] != 1 {
		t.Fatal("mutating a returned slice affected the Command's own state")
	}
}

func TestCommandString(t *testing.T) {
	c := NewCommand(OpSetFreq, [4]byte{0x00, 0x50, 0x42, 0x01})
	want := "00 50 42 01 0a"
	if got := c.String(); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
