// SPDX-License-Identifier: GPL-3.0-or-later

package ic7851

import "testing"

func TestSlotMapping(t *testing.T) {
	for _, tc := range []struct {
		slot    string
		channel int
	}{{"001", 1}, {"099", 99}, {"P1", 100}, {"P2", 101}} {
		a, _, err := slotToAddress(tc.slot)
		if err != nil || a.Channel != tc.channel || a.Group != 0 {
			t.Fatalf("%s -> %#v, %v", tc.slot, a, err)
		}
	}
	for _, slot := range []string{"000", "100", "101", "CALL", "G01-001"} {
		if _, _, err := slotToAddress(slot); err == nil {
			t.Errorf("slot %q unexpectedly accepted", slot)
		}
	}
}

func TestAllFFIsEmpty(t *testing.T) {
	if recordIsAbsent(make([]byte, 25)) {
		t.Fatal("zero record treated as empty")
	}
	ff := make([]byte, 25)
	for i := range ff {
		ff[i] = 0xff
	}
	if !recordIsAbsent(ff) {
		t.Fatal("all-FF record not treated as empty")
	}
	if recordIsAbsent(nil) {
		t.Fatal("nil record treated as empty")
	}
}
