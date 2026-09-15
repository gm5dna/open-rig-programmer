// SPDX-License-Identifier: GPL-3.0-or-later

package bincat

import "testing"

// testProfile returns an FT-890-shaped Profile for this package's own
// tests: real offsets and lengths from the evidence table (p.33's 19-byte
// VFO/Memory Data Record, p.32's Status Update table), so a test exercises
// the same geometry a real driver will configure, without this package
// shipping the value itself (that is core/driver/ft890900's job).
func testProfile() Profile {
	return Profile{
		Model:       "test-ft890",
		CATID:       "0890",
		SlotBase:    1,
		SlotCount:   32,
		RecordLen:   19,
		FullDumpLen: 649,
		Modes: map[byte]string{
			ModeLSB: "LSB",
			ModeUSB: "USB",
			ModeCW:  "CW",
			ModeAM:  "AM",
			ModeFM:  "FM",
		},
		HasTone:     true,
		FreqOffset:  2,
		ClarOffset:  5,
		ModeOffset:  7,
		ToneOffset:  8,
		FlagsOffset: 9,
	}
}

func TestProfileConfigured(t *testing.T) {
	if (Profile{}).Configured() {
		t.Fatal("zero Profile reports Configured() true")
	}
	if !testProfile().Configured() {
		t.Fatal("testProfile() reports Configured() false")
	}

	cases := []struct {
		name string
		mut  func(*Profile)
	}{
		{"no model", func(p *Profile) { p.Model = "" }},
		{"no slots", func(p *Profile) { p.SlotCount = 0 }},
		{"no record length", func(p *Profile) { p.RecordLen = 0 }},
		{"no full-dump length", func(p *Profile) { p.FullDumpLen = 0 }},
		{"no modes", func(p *Profile) { p.Modes = nil }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := testProfile()
			c.mut(&p)
			if p.Configured() {
				t.Fatalf("Configured() true after %s", c.name)
			}
		})
	}
}

func TestProfileMaxFrame(t *testing.T) {
	if got := testProfile().MaxFrame(); got != 649 {
		t.Fatalf("MaxFrame() = %d, want 649", got)
	}
	tiny := Profile{FullDumpLen: 1}
	if got := tiny.MaxFrame(); got != FrameLen {
		t.Fatalf("MaxFrame() with a 1-byte dump = %d, want FrameLen (%d)", got, FrameLen)
	}
}

func TestProfileValidChannel(t *testing.T) {
	p := testProfile()
	for _, ch := range []int{1, 2, 32} {
		if !p.ValidChannel(ch) {
			t.Errorf("ValidChannel(%d) = false, want true", ch)
		}
	}
	for _, ch := range []int{0, -1, 33, 100} {
		if p.ValidChannel(ch) {
			t.Errorf("ValidChannel(%d) = true, want false", ch)
		}
	}
}

func TestProfileReplyLength(t *testing.T) {
	p := testProfile()
	tests := []struct {
		name   string
		frame  []byte
		wantN  int
		wantOK bool
	}{
		{"full dump", BuildFrame(OpStatusUpdate, [4]byte{UFullDump, 0, 0, 0}), 649, true},
		{"memory number", BuildFrame(OpStatusUpdate, [4]byte{UMemoryNumber, 0, 0, 0}), 1, true},
		{"operating data", BuildFrame(OpStatusUpdate, [4]byte{UOperatingData, 0, 0, 0}), 19, true},
		{"both VFOs", BuildFrame(OpStatusUpdate, [4]byte{UBothVFOs, 0, 0, 0}), 18, true},
		{"memory record", BuildFrame(OpStatusUpdate, [4]byte{UMemoryRecord, 0, 0, 5}), 19, true},
		{"unknown U", BuildFrame(OpStatusUpdate, [4]byte{9, 0, 0, 0}), 0, false},
		{"a write opcode gets no reply", BuildFrame(OpSetFreq, [4]byte{0, 0, 0, 0}), 0, false},
		{"wrong length", []byte{0x10, 0, 0}, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, ok := p.ReplyLength(tt.frame)
			if n != tt.wantN || ok != tt.wantOK {
				t.Fatalf("ReplyLength(%x) = (%d, %v), want (%d, %v)", tt.frame, n, ok, tt.wantN, tt.wantOK)
			}
		})
	}
}

func TestProfileAllowedCommand(t *testing.T) {
	p := testProfile()

	allowed := [][]byte{
		BuildFrame(OpStore, [4]byte{1, 0, 0, 0}),
		BuildFrame(OpStore, [4]byte{32, 0, 0, 0}),
		BuildFrame(OpABSelect, [4]byte{0, 0, 0, 0}),
		BuildFrame(OpABSelect, [4]byte{1, 0, 0, 0}),
		BuildFrame(OpClarifier, [4]byte{0xFF, 0, 0, 0}),
		BuildFrame(OpSetFreq, [4]byte{0x00, 0x50, 0x42, 0x01}),
		BuildFrame(OpSetMode, [4]byte{ModeUSB, 0, 0, 0}),
		BuildFrame(OpStatusUpdate, [4]byte{UFullDump, 0, 0, 0}),
		BuildFrame(OpStatusUpdate, [4]byte{UMemoryRecord, 0, 0, 1}),
		BuildFrame(OpShift, [4]byte{2, 0, 0, 0}),
		BuildFrame(OpTone, [4]byte{0x20, 0, 0, 0}),
		BuildFrame(OpOffset, [4]byte{0x00, 2, 0, 0}),
	}
	for _, f := range allowed {
		if !p.AllowedCommand(f) {
			t.Errorf("AllowedCommand(%x) = false, want true", f)
		}
	}

	refused := [][]byte{
		nil,
		{0x01, 0x02, 0x03},                       // wrong length
		BuildFrame(OpStore, [4]byte{0, 0, 0, 0}), // channel 0, out of range
		BuildFrame(OpStore, [4]byte{33, 0, 0, 0}),                    // channel 33, out of range
		BuildFrame(OpABSelect, [4]byte{2, 0, 0, 0}),                  // V must be 0 or 1
		BuildFrame(OpSetFreq, [4]byte{0xFA, 0, 0, 0}),                // invalid BCD nibble
		BuildFrame(OpSetMode, [4]byte{5, 0, 0, 0}),                   // not in the 5-value legend
		BuildFrame(OpStatusUpdate, [4]byte{9, 0, 0, 0}),              // unknown U
		BuildFrame(OpStatusUpdate, [4]byte{UMemoryRecord, 0, 0, 99}), // channel out of range
		BuildFrame(OpShift, [4]byte{3, 0, 0, 0}),                     // only 0/1/2 documented
		BuildFrame(OpTone, [4]byte{0x21, 0, 0, 0}),                   // above 0x20
		BuildFrame(OpOffset, [4]byte{0x01, 0, 0, 0}),                 // first byte must be zero
		BuildFrame(0xFF, [4]byte{0, 0, 0, 0}),                        // opcode not in this milestone's set
		BuildFrame(0x02, [4]byte{1, 0, 0, 0}),                        // Recall — deliberately unshipped
	}
	for _, f := range refused {
		if p.AllowedCommand(f) {
			t.Errorf("AllowedCommand(%x) = true, want false", f)
		}
	}

	if (Profile{}).AllowedCommand(BuildFrame(OpStore, [4]byte{1, 0, 0, 0})) {
		t.Fatal("an unconfigured Profile's AllowedCommand admitted a frame")
	}
}
