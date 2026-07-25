// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"errors"
	"testing"
)

// --- CTCSSState: reference P8, "CTCSS: 0 off, 1 ENC/DEC, 2 ENC" ---

var ctcssTable = []struct {
	code  byte
	state CTCSSState
	name  string
}{
	{'0', CTCSSOff, "off"},
	{'1', CTCSSEncDec, "ENC/DEC"},
	{'2', CTCSSEnc, "ENC"},
}

func TestCTCSSStateRoundTrip(t *testing.T) {
	for _, tc := range ctcssTable {
		t.Run(string(tc.code), func(t *testing.T) {
			c, err := ParseCTCSSState(tc.code)
			if err != nil {
				t.Fatalf("ParseCTCSSState(%q) unexpected error: %v", tc.code, err)
			}
			if c != tc.state {
				t.Errorf("ParseCTCSSState(%q) = %v, want %v", tc.code, c, tc.state)
			}
			if got := c.Wire(); got != tc.code {
				t.Errorf("Wire() = %q, want %q", got, tc.code)
			}
			if got := c.String(); got != tc.name {
				t.Errorf("String() = %q, want %q", got, tc.name)
			}
		})
	}
}

func TestParseCTCSSState_RejectsGarbage(t *testing.T) {
	for _, c := range []byte{'3', '9', 'A', ' ', 0x00} {
		if _, err := ParseCTCSSState(c); err == nil {
			t.Errorf("ParseCTCSSState(%q) should be rejected", c)
		} else {
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Errorf("ParseCTCSSState(%q): error is %T, want *ParseError", c, err)
			}
		}
	}
}

// --- Shift: reference P10, "shift: 0 simplex, 1 plus, 2 minus" ---

var shiftTable = []struct {
	code  byte
	shift Shift
	name  string
}{
	{'0', ShiftSimplex, "simplex"},
	{'1', ShiftPlus, "plus"},
	{'2', ShiftMinus, "minus"},
}

func TestShiftRoundTrip(t *testing.T) {
	for _, tc := range shiftTable {
		t.Run(string(tc.code), func(t *testing.T) {
			s, err := ParseShift(tc.code)
			if err != nil {
				t.Fatalf("ParseShift(%q) unexpected error: %v", tc.code, err)
			}
			if s != tc.shift {
				t.Errorf("ParseShift(%q) = %v, want %v", tc.code, s, tc.shift)
			}
			if got := s.Wire(); got != tc.code {
				t.Errorf("Wire() = %q, want %q", got, tc.code)
			}
			if got := s.String(); got != tc.name {
				t.Errorf("String() = %q, want %q", got, tc.name)
			}
		})
	}
}

func TestParseShift_RejectsGarbage(t *testing.T) {
	for _, c := range []byte{'3', '9', 'A', ' ', 0x00} {
		if _, err := ParseShift(c); err == nil {
			t.Errorf("ParseShift(%q) should be rejected", c)
		} else {
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Errorf("ParseShift(%q): error is %T, want *ParseError", c, err)
			}
		}
	}
}

// --- Kind: reference P7, "kind: 0 VFO, 1 Memory, 2 Memory Tune, 3 QMB, 5
// PMS (4 unused)". Kind is a plain byte (brief: "Kind byte"), not a wrapped
// type, but 4 (and anything else) must still be rejected in both
// directions per the package's strictness policy. ---

func TestValidKindByte(t *testing.T) {
	tests := []struct {
		b    byte
		want bool
	}{
		{KindVFO, true},
		{KindMemory, true},
		{KindMemTune, true},
		{KindQMB, true},
		{KindPMS, true},
		{KindUnset, true}, // documented placeholder "-": parsers must accept (mirrors ModeUnset)
		{'6', false},
		{'a', false},
		{0x00, false},
	}
	for _, tc := range tests {
		if got := validKindByte(tc.b); got != tc.want {
			t.Errorf("validKindByte(%q) = %v, want %v", tc.b, got, tc.want)
		}
	}
}

// --- ClarHz: reference P3, "clarifier: +/- then 4-digit offset 0000-9990
// Hz"; brief: "signed, -9990...+9990, 10 Hz steps". ---

func TestValidClarHz(t *testing.T) {
	tests := []struct {
		name string
		v    int16
		want bool
	}{
		{"zero", 0, true},
		{"+9990 upper boundary", 9990, true},
		{"-9990 lower boundary", -9990, true},
		{"+120 (G7)", 120, true},
		{"-120 (G7, as magnitude)", -120, true},
		{"+9995 not a 10 Hz step: rejection case", 9995, false},
		{"-9995 not a 10 Hz step: rejection case", -9995, false},
		{"+10000 out of range: rejection case", 10000, false},
		{"-10000 out of range: rejection case", -10000, false},
		{"+10 valid step", 10, true},
		{"+5 not a 10 Hz step", 5, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := validClarHz(tc.v); got != tc.want {
				t.Errorf("validClarHz(%d) = %v, want %v", tc.v, got, tc.want)
			}
		})
	}
}

// --- allDigits / parseBoolDigit / boolDigit: small shared helpers used by
// both the MR decoder and the MW encoder. ---

func TestAllDigits(t *testing.T) {
	tests := []struct {
		s    string
		want bool
	}{
		{"007000000", true},
		{"0000", true},
		{"", true}, // vacuously true
		{"00a0", false},
		{"-001", false},
		{" 001", false},
	}
	for _, tc := range tests {
		if got := allDigits([]byte(tc.s)); got != tc.want {
			t.Errorf("allDigits(%q) = %v, want %v", tc.s, got, tc.want)
		}
	}
}

func TestParseBoolDigit(t *testing.T) {
	if v, err := parseBoolDigit('0'); err != nil || v != false {
		t.Errorf("parseBoolDigit('0') = %v, %v, want false, nil", v, err)
	}
	if v, err := parseBoolDigit('1'); err != nil || v != true {
		t.Errorf("parseBoolDigit('1') = %v, %v, want true, nil", v, err)
	}
	for _, c := range []byte{'2', 'X', ' ', 0x00} {
		if _, err := parseBoolDigit(c); err == nil {
			t.Errorf("parseBoolDigit(%q) should be rejected", c)
		} else {
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Errorf("parseBoolDigit(%q): error is %T, want *ParseError", c, err)
			}
		}
	}
}

func TestBoolDigit(t *testing.T) {
	if got := boolDigit(false); got != '0' {
		t.Errorf("boolDigit(false) = %q, want '0'", got)
	}
	if got := boolDigit(true); got != '1' {
		t.Errorf("boolDigit(true) = %q, want '1'", got)
	}
}

// TestMemoryDataIsComparable ensures the struct has no fields (slices,
// maps) that would break == comparison, since tests and callers compare
// MemoryData values directly for round-trip checks.
func TestMemoryDataIsComparable(t *testing.T) {
	a := MemoryData{}
	b := MemoryData{}
	if a != b {
		t.Fatal("MemoryData zero values should be equal")
	}
}
