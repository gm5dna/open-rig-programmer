// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import "testing"

// TestRecordModeName_FMNarrowIsSynthesisedFromP5AndP14 pins Q6's ruling: the
// published name of an FM record on the 590 pair is a function of TWO bytes,
// P5 and P14, because that book carries FM Narrow as a flag beside the mode
// nibble ("00: FM Normal", "01: FM Narrow", 590:1569-1571) where the Yaesu
// family folds it into the mode legend itself.
//
// THE SYNTHESIS IS THE Byte3940 AXIS'S, NOT A MODEL NAME'S. On the TS-480
// the same two bytes are a tuning step index (480:979), so P14 = "01" there
// is step 1 and says nothing about bandwidth; a synthesis keyed on the model
// rather than on the axis would publish "FM-N" for a TS-480 channel whose
// step happened to be the second one on its list.
func TestRecordModeName_FMNarrowIsSynthesisedFromP5AndP14(t *testing.T) {
	tests := []struct {
		name     string
		layout   Layout
		mode     Mode
		byte3940 string
		want     string
		wantOK   bool
	}{
		{"590SG FM Normal", layout590SG(), ModeFM, "00", "FM", true},
		{"590SG FM Narrow", layout590SG(), ModeFM, "01", "FM-N", true},
		{"590S FM Narrow", layout590S(), ModeFM, "01", "FM-N", true},
		{"590SG USB is untouched by P14", layout590SG(), ModeUSB, "01", "USB", true},
		{"590SG CW is untouched by P14", layout590SG(), ModeCW, "01", "CW", true},
		{"480 FM with step index 0", layout480(), ModeFM, "00", "FM", true},
		{"480 FM with step index 1", layout480(), ModeFM, "01", "FM", true},
		{"480 FM with step index 9", layout480(), ModeFM, "09", "FM", true},
		{"480 USB", layout480(), ModeUSB, "00", "USB", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := Record{Mode: tt.mode, Byte3940: tt.byte3940}
			got, ok := tt.layout.RecordModeName(rec)
			if ok != tt.wantOK {
				t.Fatalf("RecordModeName ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("RecordModeName = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestRecordModeName_AnFMRecordWhoseP14IsUnprintedHasNoName. On the 590 pair
// the FM name is a function of P14, and 590:1569-1571 prints exactly two
// values for it. A record carrying anything else is one this codec neither
// parses nor builds, so the honest answer is that no name can be synthesised
// — NOT "FM", which is what the spec's rejected alternative would publish and
// which would silently widen a narrow channel's name into the normal one.
func TestRecordModeName_AnFMRecordWhoseP14IsUnprintedHasNoName(t *testing.T) {
	for _, b := range []string{"", "0", "02", "99", "\x00\x00"} {
		rec := Record{Mode: ModeFM, Byte3940: b}
		if name, ok := layout590SG().RecordModeName(rec); ok {
			t.Errorf("RecordModeName with P14 %q = %q, true; want no name", b, name)
		}
	}
}

// TestRecordModeName_RefusesTheNibblesThatNameNoMode and the zero layout: a
// name published on behalf of no radio, or for a nibble both books call a
// setting failure (590:1353, 590:1362; 480:843, 480:853), would be a claim
// neither book makes.
func TestRecordModeName_RefusesTheNibblesThatNameNoMode(t *testing.T) {
	for _, m := range []Mode{ModeNone, ModeTune, Mode('x')} {
		if name, ok := layout590SG().RecordModeName(Record{Mode: m, Byte3940: "00"}); ok {
			t.Errorf("RecordModeName for nibble %q = %q, true; want no name", byte(m), name)
		}
	}
	var zero Layout
	if name, ok := zero.RecordModeName(Record{Mode: ModeFM, Byte3940: "00"}); ok {
		t.Errorf("a zero Layout named a mode: %q", name)
	}
}

// TestModeString_IsNeverADisplayName. A display name is per-LAYOUT — the 480
// prints "CWR" and "FSR" where the 590 pair print "CW-R" and "FSK-R"
// (480:852, 480:854 vs 590:1361, 590:1363; erratum E12) — and a bare Mode
// carries no layout, so a String returning a name would have to pick one
// book and be wrong on the other.
func TestModeString_IsNeverADisplayName(t *testing.T) {
	for _, m := range []Mode{ModeCWR, ModeFSKR, ModeFM} {
		if got := m.String(); got != "Mode('"+string(byte(m))+"')" {
			t.Errorf("Mode(%q).String() = %q, want the nibble form", byte(m), got)
		}
	}
}

// TestModeName_IsTheProjectsOwnSpellingOnBothBooks. E12 records what the two
// books PRINT; what a layout publishes is the programme's own consistent
// spelling, exactly as the FT-891's mode legend was treated.
func TestModeName_IsTheProjectsOwnSpellingOnBothBooks(t *testing.T) {
	for _, tt := range []struct {
		name   string
		layout Layout
	}{{"TS-590SG", layout590SG()}, {"TS-480", layout480()}} {
		t.Run(tt.name, func(t *testing.T) {
			for m, want := range map[Mode]string{ModeCWR: "CW-R", ModeFSKR: "FSK-R"} {
				got, ok := tt.layout.ModeName(m)
				if !ok {
					t.Fatalf("ModeName(%q) published nothing", byte(m))
				}
				if got != want {
					t.Errorf("ModeName(%q) = %q, want %q", byte(m), got, want)
				}
			}
		})
	}
}
