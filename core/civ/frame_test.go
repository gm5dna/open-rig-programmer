// SPDX-License-Identifier: GPL-3.0-or-later

package civ

import "testing"

func TestWellFormed(t *testing.T) {
	for _, tc := range []struct {
		name  string
		frame []byte
		want  bool
	}{
		{"the shortest legal frame", []byte{0xFE, 0xFE, 0xE0, 0x94, 0xFB, 0xFD}, true},
		{"a transceiver-ID read", []byte{0xFE, 0xFE, 0x94, 0xE0, 0x19, 0x00, 0xFD}, true},
		{"nil", nil, false},
		{"empty", []byte{}, false},
		{"one byte short of the minimum", []byte{0xFE, 0xFE, 0xE0, 0x94, 0xFD}, false},
		{"a single preamble byte", []byte{0xFE, 0xE0, 0x94, 0x19, 0x00, 0xFD}, false},
		{"no terminator", []byte{0xFE, 0xFE, 0x94, 0xE0, 0x19, 0x00}, false},
		{"an interior FD — two frames on the wire", []byte{0xFE, 0xFE, 0x94, 0xE0, 0xFD, 0x00, 0xFD}, false},
		{"an interior FE — a new frame starting inside this one", []byte{0xFE, 0xFE, 0x94, 0xE0, 0x19, 0xFE, 0xFD}, false},
		{"a third preamble byte in the addresses", []byte{0xFE, 0xFE, 0xFE, 0xE0, 0x19, 0x00, 0xFD}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := WellFormed(tc.frame); got != tc.want {
				t.Fatalf("WellFormed(% 02x) = %v, want %v", tc.frame, got, tc.want)
			}
		})
	}
}

func TestFrameAddressesAndCommand(t *testing.T) {
	frame := []byte{0xFE, 0xFE, 0xE0, 0x94, 0x1A, 0x00, 0x00, 0x12, 0xFD}

	to, ok := FrameTo(frame)
	if !ok || to != 0xE0 {
		t.Fatalf("FrameTo = %#02x, %v; want 0xe0, true", to, ok)
	}
	from, ok := FrameFrom(frame)
	if !ok || from != 0x94 {
		t.Fatalf("FrameFrom = %#02x, %v; want 0x94, true", from, ok)
	}
	cn, sc, ok := FrameCommand(frame)
	if !ok || cn != 0x1A || sc != 0x00 {
		t.Fatalf("FrameCommand = %#02x, %#02x, %v; want 0x1a, 0x00, true", cn, sc, ok)
	}

	// A frame with a command number but no sub-command byte reports the
	// command and refuses the sub-command, rather than reading the
	// terminator as data.
	short := []byte{0xFE, 0xFE, 0xE0, 0x94, 0xFB, 0xFD}
	if _, _, ok := FrameCommand(short); ok {
		t.Fatal("FrameCommand accepted a frame with no sub-command byte")
	}
	if _, ok := FrameTo(nil); ok {
		t.Fatal("FrameTo accepted nil")
	}
	if _, ok := FrameFrom([]byte{0xFE, 0xFE, 0xFD}); ok {
		t.Fatal("FrameFrom accepted a malformed frame")
	}
}

func TestIsRejectionAndAcknowledgement(t *testing.T) {
	for _, tc := range []struct {
		name    string
		frame   []byte
		reject  bool
		acknow  bool
		comment string
	}{
		{name: "FA rejection", frame: []byte{0xFE, 0xFE, 0xE0, 0x94, NakByte, 0xFD}, reject: true},
		{name: "FB acknowledgement", frame: []byte{0xFE, 0xFE, 0xE0, 0x94, AckByte, 0xFD}, acknow: true},
		{name: "an ordinary answer", frame: []byte{0xFE, 0xFE, 0xE0, 0x94, 0x19, 0x00, 0x94, 0xFD}},
		{name: "an FA-shaped frame carrying data is neither", frame: []byte{0xFE, 0xFE, 0xE0, 0x94, NakByte, 0x00, 0xFD}},
		{name: "a malformed FA", frame: []byte{0xFE, 0xE0, 0x94, NakByte, 0xFD}},
		{name: "nil", frame: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsRejection(tc.frame); got != tc.reject {
				t.Errorf("IsRejection(% 02x) = %v, want %v", tc.frame, got, tc.reject)
			}
			if got := IsAcknowledgement(tc.frame); got != tc.acknow {
				t.Errorf("IsAcknowledgement(% 02x) = %v, want %v", tc.frame, got, tc.acknow)
			}
		})
	}
}

func TestCommand_BytesIsAFreshCopyEveryCall(t *testing.T) {
	want := []byte{0xFE, 0xFE, 0x94, 0xE0, 0x19, 0x00, 0xFD}
	c := newCommand(append([]byte(nil), want...))

	first := c.Bytes()
	if string(first) != string(want) {
		t.Fatalf("Bytes() = % 02x, want % 02x", first, want)
	}
	// Mutating what a caller was handed must not reach the Command, nor
	// any copy handed out before or after: this is the TOCTOU window the
	// type exists to close.
	first[2] = 0x00
	second := c.Bytes()
	if string(second) != string(want) {
		t.Fatalf("after mutating a returned copy, Bytes() = % 02x, want % 02x", second, want)
	}
	if &first[0] == &second[0] {
		t.Fatal("two Bytes() calls returned the same backing array")
	}
}

func TestCommand_ZeroValue(t *testing.T) {
	var zero Command
	if !zero.IsZero() {
		t.Fatal("the zero Command does not report IsZero()")
	}
	if n := len(zero.Bytes()); n != 0 {
		t.Fatalf("the zero Command yielded %d bytes", n)
	}
	if s := zero.String(); s == "" {
		t.Fatal("the zero Command's String() is empty — it must still render for a diagnostic")
	}
}

func TestCommand_StringRendersHexNotText(t *testing.T) {
	c := newCommand([]byte{0xFE, 0xFE, 0x94, 0xE0, 0x19, 0x00, 0xFD})
	got := c.String()
	// CI-V frames are binary: a %q rendering of 0xFE is unreadable and a
	// raw one corrupts a log line. Hex pairs are the only useful form.
	if want := "fe fe 94 e0 19 00 fd"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
