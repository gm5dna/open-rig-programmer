// SPDX-License-Identifier: GPL-3.0-or-later

package ft1000mp

import "testing"

// TestDecodeFreqTensOfHz_ManualWorkedExample pins decodeFreqTensOfHz
// against the manual's own raw bytes for its read-side worked example
// (ft1000mpmarkv_manual layout:4884-4898, printed p.90-91): the record's
// own nibble-decimal bytes for 14.250.00 MHz are 00,05,24,10 (hex) — see
// record.go's decodeFreqTensOfHz doc comment for the re-derivation this
// pins.
func TestDecodeFreqTensOfHz_ManualWorkedExample(t *testing.T) {
	got, err := decodeFreqTensOfHz([4]byte{0x00, 0x05, 0x24, 0x10})
	if err != nil {
		t.Fatalf("decodeFreqTensOfHz: %v", err)
	}
	const want = 1_425_000 // tens-of-Hz = 14,250,000 Hz = 14.25 MHz
	if got != want {
		t.Fatalf("decodeFreqTensOfHz = %d, want %d", got, want)
	}
}

func TestSwapNibbles(t *testing.T) {
	cases := map[byte]byte{0x00: 0x00, 0x05: 0x50, 0x24: 0x42, 0x10: 0x01}
	for in, want := range cases {
		if got := swapNibbles(in); got != want {
			t.Errorf("swapNibbles(%#02x) = %#02x, want %#02x", in, got, want)
		}
	}
}

// TestParseRecord_Masked pins the memory-mask flag (Band Selection byte,
// bit 0x80) as this driver's empty-slot signal.
func TestParseRecord_Masked(t *testing.T) {
	raw := make([]byte, recordLen)
	raw[offBandSelect] = flagMemMask
	rec, err := parseRecord(raw)
	if err != nil {
		t.Fatalf("parseRecord: %v", err)
	}
	if !rec.Masked {
		t.Fatal("Masked = false, want true")
	}
}

// TestParseRecord_Populated exercises frequency, clarifier, mode and
// shift decode together on one synthetic record.
func TestParseRecord_Populated(t *testing.T) {
	raw := make([]byte, recordLen)
	// Frequency: 14.250.00 MHz, manual's own worked-example bytes.
	raw[offFreq+0], raw[offFreq+1], raw[offFreq+2], raw[offFreq+3] = 0x00, 0x05, 0x24, 0x10
	// Clarifier: +9989.375 Hz -> raw 16-bit 0x3E6F (manual's own worked example).
	raw[offClarifier], raw[offClarifier+1] = 0x3E, 0x6F
	// Mode: CW is code 2 -> bits 5-7 of byte 7 -> 2<<5 = 0x40.
	raw[offMode] = 2 << 5
	// Shift: Minus.
	raw[offFlags] = flagRptMinus

	rec, err := parseRecord(raw)
	if err != nil {
		t.Fatalf("parseRecord: %v", err)
	}
	if rec.Masked {
		t.Error("Masked = true, want false")
	}
	if rec.FreqHz != 14_250_000 {
		t.Errorf("FreqHz = %d, want 14250000", rec.FreqHz)
	}
	if rec.ClarHz != 9989 { // int(15983 * 0.625) truncates to 9989
		t.Errorf("ClarHz = %d, want 9989", rec.ClarHz)
	}
	if rec.ModeByte != 2 {
		t.Errorf("ModeByte = %d, want 2", rec.ModeByte)
	}
	if rec.Shift != "MINUS" {
		t.Errorf("Shift = %q, want MINUS", rec.Shift)
	}
}

func TestParseRecord_WrongLength(t *testing.T) {
	if _, err := parseRecord(make([]byte, 15)); err == nil {
		t.Fatal("parseRecord accepted a 15-byte record")
	}
}

// TestParseRecord_BothShiftBitsRefused pins that an operating-flags byte
// setting both Minus and Plus is a decode error, never a silent pick.
func TestParseRecord_BothShiftBitsRefused(t *testing.T) {
	raw := make([]byte, recordLen)
	raw[offFlags] = flagRptMinus | flagRptPlus
	if _, err := parseRecord(raw); err == nil {
		t.Fatal("parseRecord accepted both Minus and Plus set")
	}
}
