// SPDX-License-Identifier: GPL-3.0-or-later

package bincat

import (
	"errors"
	"testing"
)

// buildTestRecord returns a 19-byte record laid out per testProfile()'s
// own offsets: freq 1,425,000 tens-of-Hz (14.25000 MHz), clarifier
// +100 Hz, mode USB, tone 0x0A, Minus shift, neither blanked nor split.
func buildTestRecord() []byte {
	raw := make([]byte, 19)
	raw[0] = 0x00
	raw[2], raw[3], raw[4] = 0x15, 0xbe, 0x68 // 1,425,000 tens of Hz, MSB first
	raw[5], raw[6] = 0x00, 0x64               // +100 Hz clarifier
	raw[7] = ModeUSB
	raw[8] = 0x0A
	raw[9] = 0x08 // Minus shift (bit 3)
	return raw
}

func TestParseRecord(t *testing.T) {
	p := testProfile()
	raw := buildTestRecord()

	rec, err := ParseRecord(raw, p)
	if err != nil {
		t.Fatalf("ParseRecord: %v", err)
	}
	want := Record{
		FreqTensOfHz: 1425000,
		ClarifierHz:  100,
		ModeByte:     ModeUSB,
		Mode:         "USB",
		Tone:         0x0A,
		Shift:        "MINUS",
	}
	if rec != want {
		t.Fatalf("ParseRecord(% x) = %+v, want %+v", raw, rec, want)
	}
}

func TestParseRecordPlusShiftAndFlags(t *testing.T) {
	p := testProfile()
	raw := buildTestRecord()
	raw[9] = 0x10 // Plus shift (bit 4)
	raw[0] = 0xC0 // blanked (bit7) and split (bit6)

	rec, err := ParseRecord(raw, p)
	if err != nil {
		t.Fatalf("ParseRecord: %v", err)
	}
	if rec.Shift != "PLUS" {
		t.Errorf("Shift = %q, want PLUS", rec.Shift)
	}
	if !rec.Blanked || !rec.Split {
		t.Errorf("Blanked=%v Split=%v, want both true", rec.Blanked, rec.Split)
	}
}

func TestParseRecordSimplexDefault(t *testing.T) {
	p := testProfile()
	raw := buildTestRecord()
	raw[9] = 0x00 // neither shift bit
	rec, err := ParseRecord(raw, p)
	if err != nil {
		t.Fatalf("ParseRecord: %v", err)
	}
	if rec.Shift != "SIMPLEX" {
		t.Errorf("Shift = %q, want SIMPLEX", rec.Shift)
	}
}

func TestParseRecordBothShiftBitsIsAnError(t *testing.T) {
	p := testProfile()
	raw := buildTestRecord()
	raw[9] = 0x18 // both Minus and Plus
	if _, err := ParseRecord(raw, p); !errors.Is(err, ErrRecord) {
		t.Fatalf("ParseRecord with both shift bits set: error = %v, want ErrRecord", err)
	}
}

func TestParseRecordWrongLength(t *testing.T) {
	p := testProfile()
	if _, err := ParseRecord(make([]byte, 18), p); !errors.Is(err, ErrRecord) {
		t.Fatalf("ParseRecord(18 bytes) error = %v, want ErrRecord", err)
	}
}

func TestParseRecordUnconfiguredProfile(t *testing.T) {
	if _, err := ParseRecord(buildTestRecord(), Profile{}); !errors.Is(err, ErrRecord) {
		t.Fatalf("ParseRecord with a zero Profile: error = %v, want ErrRecord", err)
	}
}

func TestParseRecordUnknownModeByteIsEmptyName(t *testing.T) {
	p := testProfile()
	raw := buildTestRecord()
	raw[7] = 0x7F // not in the 5-value legend
	rec, err := ParseRecord(raw, p)
	if err != nil {
		t.Fatalf("ParseRecord: %v", err)
	}
	if rec.Mode != "" {
		t.Errorf("Mode = %q for an unmapped byte, want empty", rec.Mode)
	}
}

func TestParseRecordNoTone(t *testing.T) {
	p := testProfile()
	p.HasTone = false
	raw := buildTestRecord()
	raw[8] = 0xFF // would be Tone if HasTone were true
	rec, err := ParseRecord(raw, p)
	if err != nil {
		t.Fatalf("ParseRecord: %v", err)
	}
	if rec.Tone != 0 {
		t.Errorf("Tone = %#x for a !HasTone profile, want 0 (unread)", rec.Tone)
	}
}
