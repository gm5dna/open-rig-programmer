// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft2000

import "testing"

func TestDefaultImage_EachCallIsIndependent(t *testing.T) {
	a := DefaultImage()
	b := DefaultImage()
	a["001"] = MemState{Freq: "99999999"}
	if b["001"].Freq == "99999999" {
		t.Fatal("mutating one DefaultImage() call's map changed another's — the maps alias")
	}
}

func TestDefaultImage_HasAtLeastOneMemoryAndOnePMSSlot(t *testing.T) {
	img := DefaultImage()
	if _, ok := img["001"]; !ok {
		t.Error("DefaultImage has no populated plain memory channel")
	}
	if _, ok := img["100"]; !ok {
		t.Error("DefaultImage has no populated PMS pair")
	}
	for slot := range img {
		if kind := parseSlotForm(slot); kind == slotInvalid {
			t.Errorf("DefaultImage carries slot %q, outside this radio's 001-117 span", slot)
		}
	}
}

func TestPMSSlotSpelling(t *testing.T) {
	tests := []struct {
		pair int
		half byte
		want string
	}{
		{1, 'L', "100"},
		{1, 'U', "101"},
		{9, 'L', "116"},
		{9, 'U', "117"},
	}
	for _, tt := range tests {
		if got := pmsSlot(tt.pair, tt.half); got != tt.want {
			t.Errorf("pmsSlot(%d, %q) = %q, want %q", tt.pair, tt.half, got, tt.want)
		}
	}
}

func TestEncodeFreqDigits(t *testing.T) {
	got, err := encodeFreqDigits(7_000_000)
	if err != nil || got != "07000000" {
		t.Errorf("encodeFreqDigits(7_000_000) = %q, %v, want \"07000000\", nil", got, err)
	}
	if _, err := encodeFreqDigits(100_000_000); err == nil {
		t.Error("encodeFreqDigits(100_000_000) did not refuse a 9-digit value on an 8-digit field")
	}
}
