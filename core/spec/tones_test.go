// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import "testing"

// TestToneHzAndString checks the two extreme, manually-verifiable entries
// of StandardCTCSSTones: index 000 (670 = 67.0 Hz, the lowest CTCSS tone)
// and index 049 (2541 = 254.1 Hz, the highest).
func TestToneHzAndString(t *testing.T) {
	cases := []struct {
		name     string
		tone     Tone
		wantHz   float64
		wantText string
	}{
		{"first (P3=000)", Tone(670), 67.0, "67.0 Hz"},
		{"last (P3=049)", Tone(2541), 254.1, "254.1 Hz"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.tone.Hz(); got != tc.wantHz {
				t.Errorf("Hz() = %v, want %v", got, tc.wantHz)
			}
			if got := tc.tone.String(); got != tc.wantText {
				t.Errorf("String() = %q, want %q", got, tc.wantText)
			}
		})
	}
}

// TestStandardCTCSSTonesShape verifies the table-level invariants: exactly
// 50 entries (the count confirmed against the FT-710 CAT manual's CN
// command Table 1), strictly ascending order, and no duplicates. Strict
// ascending order plus no duplicates also guarantees the slice index can
// safely double as the CAT tone number P3, as the manual defines it.
func TestStandardCTCSSTonesShape(t *testing.T) {
	tones := StandardCTCSSTones()
	const wantLen = 50
	if got := len(tones); got != wantLen {
		t.Fatalf("len(StandardCTCSSTones()) = %d, want %d", got, wantLen)
	}
	seen := make(map[Tone]bool, wantLen)
	for i, tone := range tones {
		if seen[tone] {
			t.Errorf("index %d: duplicate tone value %v", i, tone)
		}
		seen[tone] = true
		if i > 0 && tones[i-1] >= tone {
			t.Errorf("index %d: tone %v is not strictly greater than previous %v", i, tone, tones[i-1])
		}
	}
}

// TestStandardCTCSSTonesFirstAndLast pins the exact first and last table
// entries (and thus the P3=000 and P3=049 tone numbers) against the FT-710
// CAT manual 2306-C, CN command, Table 1.
func TestStandardCTCSSTonesFirstAndLast(t *testing.T) {
	tones := StandardCTCSSTones()
	if got := tones[0]; got != 670 {
		t.Errorf("StandardCTCSSTones()[0] (P3=000) = %v, want 670 (67.0 Hz)", got)
	}
	if got := tones[49]; got != 2541 {
		t.Errorf("StandardCTCSSTones()[49] (P3=049) = %v, want 2541 (254.1 Hz)", got)
	}
}

// TestStandardCTCSSTonesFull pins every one of the 50 entries, in index
// order, against the verified table copied from the FT-710 CAT manual
// 2306-C, CN command, Table 1 (CTCSS Tone Chart). The slice index is the
// CAT tone number P3.
func TestStandardCTCSSTonesFull(t *testing.T) {
	want := [50]Tone{
		670, 693, 719, 744, 770, 797, 825, 854, 885,
		915, 948, 974, 1000, 1035, 1072, 1109, 1148, 1188,
		1230, 1273, 1318, 1365, 1413, 1462, 1514, 1567, 1598, 1622,
		1655, 1679, 1713, 1738, 1773, 1799, 1835, 1862,
		1899, 1928, 1966, 1995, 2035, 2065, 2107, 2181, 2257,
		2291, 2336, 2418, 2503, 2541,
	}
	got := StandardCTCSSTones()
	if got != want {
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("index %d (P3=%03d): got %v, want %v", i, i, got[i], want[i])
			}
		}
	}
}

// TestStandardCTCSSTonesReturnsCopy checks that StandardCTCSSTones()
// hands back an independent array each call, not a shared, mutable
// reference: mutating one call's result must never be observable through
// a second, separate call. (Go arrays are value types, so this is
// guaranteed by the language itself — this test pins that contract
// explicitly, since a future refactor to a slice-backed implementation
// could silently break it.)
func TestStandardCTCSSTonesReturnsCopy(t *testing.T) {
	a := StandardCTCSSTones()
	a[0] = 9999
	b := StandardCTCSSTones()
	if b[0] == 9999 {
		t.Fatal("mutating one call's result changed a later call's result: StandardCTCSSTones() is not returning an independent copy")
	}
	if b[0] != 670 {
		t.Errorf("StandardCTCSSTones()[0] = %v after a prior call was mutated, want 670 (unaffected)", b[0])
	}
}

// TestValidTone covers ValidTone across every table entry (all 50 must
// report valid) plus representative invalid values: zero, a value
// between two table entries, and a value well outside the table's range.
func TestValidTone(t *testing.T) {
	for _, tone := range StandardCTCSSTones() {
		if !ValidTone(tone) {
			t.Errorf("ValidTone(%v) = false, want true (table entry)", tone)
		}
	}

	cases := []struct {
		name string
		tone Tone
	}{
		{"zero", Tone(0)},
		{"between two table entries", Tone(671)},
		{"far outside the table's range", Tone(99999)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if ValidTone(tc.tone) {
				t.Errorf("ValidTone(%v) = true, want false", tc.tone)
			}
		})
	}
}
