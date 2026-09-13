// SPDX-License-Identifier: GPL-3.0-or-later

package fakeftdx3000

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

func TestDefaultImage_HasANonzeroToneSlot(t *testing.T) {
	// Exercises the P9 read/write asymmetry (doc.go register entry TONE
	// INDEX (P9)) from New()'s own default image, not only from a
	// test-constructed WithSlot option.
	img := DefaultImage()
	s, ok := img["002"]
	if !ok {
		t.Fatal("DefaultImage has no channel 002")
	}
	if s.Tone == "00" {
		t.Error(`channel 002's Tone = "00", want a nonzero fixture tone exercising the live-read asymmetry`)
	}
	if !validToneDigits(s.Tone) {
		t.Errorf("channel 002's Tone %q is not a valid 2-digit tone index", s.Tone)
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

func TestValidModeByte_NoAMNHole(t *testing.T) {
	if validModeByte('D') {
		t.Error("validModeByte('D') = true, want false — AM-N is absent from the MW/MR legend (matrix §1.2)")
	}
	for _, m := range []byte("123456789ABC") {
		if !validModeByte(m) {
			t.Errorf("validModeByte(%q) = false, want true — every one of the twelve legend members must pass", m)
		}
	}
}
