// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import "testing"

// Mode nibble table (reference: "Mode nibble (P6) — single ASCII char, emit
// upper case"), plus the '0' = "-" (unset) row documented immediately below
// that table.
var modeTable = []struct {
	code byte
	mode Mode
	name string
}{
	{'0', ModeUnset, "-"},
	{'1', ModeLSB, "LSB"},
	{'2', ModeUSB, "USB"},
	{'3', ModeCWU, "CW-U"},
	{'4', ModeFM, "FM"},
	{'5', ModeAM, "AM"},
	{'6', ModeRTTYL, "RTTY-L"},
	{'7', ModeCWL, "CW-L"},
	{'8', ModeDATAL, "DATA-L"},
	{'9', ModeRTTYU, "RTTY-U"},
	{'A', ModeDATAFM, "DATA-FM"},
	{'B', ModeFMN, "FM-N"},
	{'C', ModeDATAU, "DATA-U"},
	{'D', ModeAMN, "AM-N"},
	{'E', ModePSK, "PSK"},
	{'F', ModeDATAFMN, "DATA-FM-N"},
}

// TestModeRoundTrip covers all 16 wire codes ('0'-'9','A'-'F'): ParseMode
// must accept each, Wire() must return the original byte, and String()
// must return the reference's display name.
func TestModeRoundTrip(t *testing.T) {
	if len(modeTable) != 16 {
		t.Fatalf("modeTable has %d entries, want 16 (10 digits + 6 hex letters)", len(modeTable))
	}
	for _, tc := range modeTable {
		t.Run(string(tc.code), func(t *testing.T) {
			m, err := ParseMode(tc.code)
			if err != nil {
				t.Fatalf("ParseMode(%q) unexpected error: %v", tc.code, err)
			}
			if m != tc.mode {
				t.Errorf("ParseMode(%q) = %v, want %v", tc.code, m, tc.mode)
			}
			if got := m.Wire(); got != tc.code {
				t.Errorf("Wire() = %q, want %q", got, tc.code)
			}
			if got := m.String(); got != tc.name {
				t.Errorf("String() = %q, want %q", got, tc.name)
			}
		})
	}
}

// TestModeNamedConstantCount ensures exactly the 15 named modes from the
// reference table exist as distinct constants, separate from ModeUnset.
func TestModeNamedConstantCount(t *testing.T) {
	named := []Mode{
		ModeLSB, ModeUSB, ModeCWU, ModeFM, ModeAM, ModeRTTYL, ModeCWL,
		ModeDATAL, ModeRTTYU, ModeDATAFM, ModeFMN, ModeDATAU, ModeAMN,
		ModePSK, ModeDATAFMN,
	}
	if len(named) != 15 {
		t.Fatalf("expected 15 named mode constants, listed %d", len(named))
	}
	seen := make(map[Mode]bool, len(named))
	for _, m := range named {
		if seen[m] {
			t.Errorf("duplicate mode constant value %v", m)
		}
		seen[m] = true
		if m == ModeUnset {
			t.Errorf("named mode %v must not equal ModeUnset", m)
		}
	}
}

func TestParseMode_RejectsLowerCase(t *testing.T) {
	_, err := ParseMode('a')
	if err == nil {
		t.Fatal("ParseMode('a') should be rejected: only upper case comes from the radio")
	}
}

func TestParseMode_RejectsOutOfRangeLetter(t *testing.T) {
	_, err := ParseMode('G')
	if err == nil {
		t.Fatal("ParseMode('G') should be rejected: not in '0'-'9','A'-'F'")
	}
}

func TestParseMode_RejectsOtherGarbage(t *testing.T) {
	for _, c := range []byte{' ', '!', '/', ':', '@', '['} {
		if _, err := ParseMode(c); err == nil {
			t.Errorf("ParseMode(%q) should be rejected", c)
		}
	}
}

// ParseMode failures must be a typed *ParseError, per package convention.
func TestParseMode_ErrorIsParseError(t *testing.T) {
	_, err := ParseMode('Z')
	var pe *ParseError
	if err == nil {
		t.Fatal("expected error")
	}
	if pe2, ok := err.(*ParseError); !ok {
		t.Fatalf("error is %T, want *ParseError", err)
	} else {
		pe = pe2
	}
	if pe.Reason == "" {
		t.Error("ParseError.Reason should not be empty")
	}
}
