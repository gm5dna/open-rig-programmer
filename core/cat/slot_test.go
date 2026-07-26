// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"errors"
	"testing"
)

// assertSlotKind checks that exactly the named Is*() method reports true
// for s, and every other kind predicate reports false.
func assertSlotKind(t *testing.T, s Slot, kind string) {
	t.Helper()
	checks := map[string]bool{
		"memory": s.IsMemory(),
		"pms":    s.IsPMS(),
		"60m":    s.Is60m(),
		"emg":    s.IsEMG(),
		"none":   s.IsNone(),
	}
	for k, got := range checks {
		want := k == kind
		if got != want {
			t.Errorf("slot %q: %s() = %v, want %v", s.Wire(), k, got, want)
		}
	}
}

// --- MemorySlot: reference "001-099 | Memory channels M-01…M-99" ---

func TestMemorySlot(t *testing.T) {
	tests := []struct {
		name     string
		n        int
		wantWire string
		wantErr  bool
	}{
		{"n=0 rejected: below range 1-99", 0, "", true},
		{"n=1 -> 001: lower boundary", 1, "001", false},
		{"n=99 -> 099: upper boundary", 99, "099", false},
		{"n=100 rejected: above range 1-99", 100, "", true},
		{"n=-1 rejected: negative", -1, "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, err := FT710.MemorySlot(tc.n)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("MemorySlot(%d): want error, got Slot %q", tc.n, s.Wire())
				}
				return
			}
			if err != nil {
				t.Fatalf("MemorySlot(%d): unexpected error: %v", tc.n, err)
			}
			if s.Wire() != tc.wantWire {
				t.Errorf("MemorySlot(%d).Wire() = %q, want %q", tc.n, s.Wire(), tc.wantWire)
			}
			assertSlotKind(t, s, "memory")
		})
	}
}

// --- PMSSlot: reference "P1L-P9U | PMS pairs (9 lower/upper pairs)" ---
// G6 golden vector: MR answer uses slot P1L.

func TestPMSSlot(t *testing.T) {
	tests := []struct {
		name     string
		pair     int
		upper    bool
		wantWire string
		wantErr  bool
	}{
		{"pair=0 rejected: below range 1-9", 0, false, "", true},
		{"pair=1 lower -> P1L (G6 golden vector)", 1, false, "P1L", false},
		{"pair=1 upper -> P1U", 1, true, "P1U", false},
		{"pair=9 lower -> P9L: upper boundary", 9, false, "P9L", false},
		{"pair=9 upper -> P9U: upper boundary", 9, true, "P9U", false},
		{"pair=10 rejected: above range 1-9", 10, false, "", true},
		{"pair=-1 rejected: negative", -1, false, "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, err := FT710.PMSSlot(tc.pair, tc.upper)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("PMSSlot(%d,%v): want error, got Slot %q", tc.pair, tc.upper, s.Wire())
				}
				return
			}
			if err != nil {
				t.Fatalf("PMSSlot(%d,%v): unexpected error: %v", tc.pair, tc.upper, err)
			}
			if s.Wire() != tc.wantWire {
				t.Errorf("PMSSlot(%d,%v).Wire() = %q, want %q", tc.pair, tc.upper, s.Wire(), tc.wantWire)
			}
			assertSlotKind(t, s, "pms")
		})
	}
}

// --- SixtyMSlot: reference "5xx | 60m channels (region-dependent;
// ASSUMED 501… numbering)". The numbering start (501) AND the upper bound
// (599, since the wire form is fixed at 3 bytes: '5' + 2 digits) are both
// ASSUMED here, pending hardware verification (M5a/M5b).

func TestSixtyMSlot(t *testing.T) {
	tests := []struct {
		name     string
		n        int
		wantWire string
		wantErr  bool
	}{
		{"n=0 rejected: spec requires n>=1", 0, "", true},
		{"n=1 -> 501: ASSUMED numbering start per reference", 1, "501", false},
		{"n=99 -> 599: ASSUMED upper bound (3-byte wire form)", 99, "599", false},
		{"n=100 rejected: would need a 4-byte wire form", 100, "", true},
		{"n=-1 rejected: negative", -1, "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, err := FT710.SixtyMSlot(tc.n)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("SixtyMSlot(%d): want error, got Slot %q", tc.n, s.Wire())
				}
				return
			}
			if err != nil {
				t.Fatalf("SixtyMSlot(%d): unexpected error: %v", tc.n, err)
			}
			if s.Wire() != tc.wantWire {
				t.Errorf("SixtyMSlot(%d).Wire() = %q, want %q", tc.n, s.Wire(), tc.wantWire)
			}
			assertSlotKind(t, s, "60m")
		})
	}
}

// --- EMGSlot: reference "EMG | Alaska emergency channel" ---

func TestEMGSlot(t *testing.T) {
	s := FT710.EMGSlot()
	if s.Wire() != "EMG" {
		t.Errorf("EMGSlot().Wire() = %q, want %q", s.Wire(), "EMG")
	}
	assertSlotKind(t, s, "emg")
}

// --- ParseSlot accept/reject table ---
//
// Reject cases directly from the reference's "Rejection cases the builders
// MUST refuse" list: slot 100, P0L, P9X, EM, empty.

func TestParseSlot(t *testing.T) {
	tests := []struct {
		name    string
		wire    string
		wantErr bool
		kind    string // only checked when wantErr is false
	}{
		{"memory 001: lower boundary", "001", false, "memory"},
		{"memory 099: upper boundary", "099", false, "memory"},
		{"memory 100 rejected: reference rejection list", "100", true, ""},
		{"memory 000 handled separately (see none case)", "000", false, "none"},
		{"PMS P1L (G6 golden vector)", "P1L", false, "pms"},
		{"PMS P9U: upper boundary", "P9U", false, "pms"},
		{"PMS P0L rejected: reference rejection list", "P0L", true, ""},
		{"PMS P9X rejected: reference rejection list (bad suffix)", "P9X", true, ""},
		{"EMG", "EMG", false, "emg"},
		{"EM rejected: reference rejection list (too short)", "EM", true, ""},
		{"000 -> IsNone: MR answer VFO/MT/QMB", "000", false, "none"},
		{"60m 501: ASSUMED lower boundary", "501", false, "60m"},
		{"60m 599: ASSUMED upper boundary", "599", false, "60m"},
		{"60m 500 rejected: below ASSUMED n>=1 boundary", "500", true, ""},
		{"60m 600 rejected: not a 5xx form", "600", true, ""},
		{"lower case p1l rejected: case sensitivity", "p1l", true, ""},
		{"lower case emg rejected: case sensitivity", "emg", true, ""},
		{"empty rejected: reference rejection list", "", true, ""},
		{"too long rejected", "0011", true, ""},
		{"too short rejected", "01", true, ""},
		{"garbage rejected", "XYZ", true, ""},
		{"whitespace rejected", "   ", true, ""},
		{"P with digit out of A-Z suffix rejected", "P15", true, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, err := FT710.ParseSlot(tc.wire)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseSlot(%q): want error, got Slot %q", tc.wire, s.Wire())
				}
				var pe *ParseError
				if !errors.As(err, &pe) {
					t.Errorf("ParseSlot(%q): error is %T, want *ParseError", tc.wire, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseSlot(%q): unexpected error: %v", tc.wire, err)
			}
			if s.Wire() != tc.wire {
				t.Errorf("ParseSlot(%q).Wire() = %q, want %q", tc.wire, s.Wire(), tc.wire)
			}
			assertSlotKind(t, s, tc.kind)
		})
	}
}

// --- Writable() truth table: reference restricts MW (write) to memory and
// PMS slots only; 5xx, EMG and 000 are all excluded from MW. ---

func TestSlot_Writable(t *testing.T) {
	memory, err := FT710.MemorySlot(1)
	if err != nil {
		t.Fatal(err)
	}
	pms, err := FT710.PMSSlot(1, false)
	if err != nil {
		t.Fatal(err)
	}
	sixtym, err := FT710.SixtyMSlot(1)
	if err != nil {
		t.Fatal(err)
	}
	none, err := FT710.ParseSlot("000")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		slot Slot
		want bool
	}{
		{"memory writable", memory, true},
		{"pms writable", pms, true},
		{"60m not writable: reference MW P1 excludes 5xx", sixtym, false},
		{"emg not writable: reference MW P1 excludes EMG", FT710.EMGSlot(), false},
		{"none (000) not writable: semantics unknown, reference says do not emit", none, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.slot.Writable(); got != tc.want {
				t.Errorf("%q.Writable() = %v, want %v", tc.slot.Wire(), got, tc.want)
			}
		})
	}
}

func TestSlot_WireIsThreeBytes(t *testing.T) {
	slots := []Slot{}
	if s, err := FT710.MemorySlot(1); err == nil {
		slots = append(slots, s)
	}
	if s, err := FT710.PMSSlot(1, true); err == nil {
		slots = append(slots, s)
	}
	if s, err := FT710.SixtyMSlot(1); err == nil {
		slots = append(slots, s)
	}
	slots = append(slots, FT710.EMGSlot())
	if s, err := FT710.ParseSlot("000"); err == nil {
		slots = append(slots, s)
	}
	for _, s := range slots {
		if len(s.Wire()) != 3 {
			t.Errorf("Slot %q: Wire() length = %d, want 3", s.Wire(), len(s.Wire()))
		}
	}
}

// FuzzParseSlot seeds valid and invalid wire forms and requires that
// ParseSlot never panics and only ever returns a typed *ParseError on
// failure.
func FuzzParseSlot(f *testing.F) {
	seeds := []string{
		"001", "099", "100", "000", "P1L", "P9U", "P0L", "P9X", "P15",
		"EMG", "EM", "501", "599", "500", "600", "", "p1l", "emg",
		"0011", "01", "XYZ", "   ", "5xx", "\x00\x00\x00", ";;;",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, wire string) {
		s, err := FT710.ParseSlot(wire)
		if err != nil {
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("ParseSlot(%q) returned non-ParseError: %T (%v)", wire, err, err)
			}
			return
		}
		if s.Wire() != wire {
			t.Fatalf("ParseSlot(%q) succeeded but Wire() = %q", wire, s.Wire())
		}
	})
}
