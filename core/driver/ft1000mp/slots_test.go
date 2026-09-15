// SPDX-License-Identifier: GPL-3.0-or-later

package ft1000mp

import "testing"

func TestParseSlot(t *testing.T) {
	cases := []struct {
		slot       string
		wantPrefix string
		wantN      int
		wantErr    bool
	}{
		{"1", "", 1, false},
		{"99", "", 99, false},
		{"100", "", 0, true},
		{"0", "", 0, true},
		{"P1", "P", 1, false},
		{"P9", "P", 9, false},
		{"P10", "P", 0, true},
		{"QMB1", "QMB", 1, false},
		{"QMB5", "QMB", 5, false},
		{"QMB6", "QMB", 0, true},
		{"bogus", "", 0, true},
	}
	for _, c := range cases {
		prefix, n, err := parseSlot(c.slot)
		if (err != nil) != c.wantErr {
			t.Errorf("parseSlot(%q) err = %v, wantErr %v", c.slot, err, c.wantErr)
			continue
		}
		if err == nil && (prefix != c.wantPrefix || n != c.wantN) {
			t.Errorf("parseSlot(%q) = (%q, %d), want (%q, %d)", c.slot, prefix, n, c.wantPrefix, c.wantN)
		}
	}
}

// TestRecordIndex_Fixed pins the dump's own fixed record positions —
// index 0-2 are current-op/VFO-A/VFO-B, memories start at 3 (matrix
// §1.6) — independent of storeArg's channel-numbering-base ASSUMPTION.
func TestRecordIndex_Fixed(t *testing.T) {
	cases := []struct {
		slot string
		want int
	}{
		{"1", 3},
		{"99", 101},
		{"P1", 102},
		{"P9", 110},
		{"QMB1", 111},
		{"QMB5", 115},
	}
	for _, c := range cases {
		got, err := recordIndex(c.slot)
		if err != nil {
			t.Fatalf("recordIndex(%q): %v", c.slot, err)
		}
		if got != c.want {
			t.Errorf("recordIndex(%q) = %d, want %d", c.slot, got, c.want)
		}
	}
	if dumpRecordCount != 116 {
		t.Fatalf("dumpRecordCount = %d, want 116", dumpRecordCount)
	}
}

// TestStoreArg_ASSUMED1Based pins the 15/09/2026 override's ASSUMED
// 1-based channel argument (matrix §1.4/§1.8): channel N -> X=N, the
// SAME 01H~71H range the Opcode Command Chart states.
func TestStoreArg_ASSUMED1Based(t *testing.T) {
	cases := []struct {
		slot string
		want byte
	}{
		{"1", 0x01},
		{"99", 0x63},
		{"P1", 0x64},
		{"P9", 0x6C},
		{"QMB1", 0x6D},
		{"QMB5", 0x71},
	}
	for _, c := range cases {
		got, err := storeArg(c.slot)
		if err != nil {
			t.Fatalf("storeArg(%q): %v", c.slot, err)
		}
		if got != c.want {
			t.Errorf("storeArg(%q) = %#02x, want %#02x", c.slot, got, c.want)
		}
	}
}
