// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft891

import (
	"sort"
	"strings"
	"testing"
)

// slotKeys returns img's slot codes, sorted, so an enumeration assertion reads
// as a set rather than as a map iteration.
func slotKeys(img map[string]MemState) []string {
	keys := make([]string, 0, len(img))
	for k := range img {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// TestDefaultImage_IsMinimalAndExactlyEnumerated states the default image's
// whole content, so that adding or dropping a slot is a decision a reviewer
// meets rather than a diff nobody reads.
//
// THE TWO CONSTRAINTS ARE THE PLAN'S, and each has a reason:
//
//   - AT LEAST ONE MEMORY CHANNEL IS POPULATED, so that the fleet's
//     read-every-registered-model pins are NON-VACUOUS against this radio (a
//     fake whose every slot was empty would let a broken read path pass them).
//   - NO 5 MHz AND NO EMG SLOT, so that the default fake exercises
//     core/driver/ft891's discovery walk in its ordinary case — eleven MR
//     probes, every one answered "?;", no discovered banks — and so that the
//     UI's one-bank-set-per-model membership pins see the same seven banks
//     every time. With5MHz and WithEMG are how a test asks for the other case.
func TestDefaultImage_IsMinimalAndExactlyEnumerated(t *testing.T) {
	want := []string{
		"001", "002",
		"P1L", "P1U", "P2L", "P2U", "P3L", "P3U", "P4L", "P4U", "P5L", "P5U",
		"P6L", "P6U", "P7L", "P7U", "P8L", "P8U", "P9L", "P9U",
	}
	got := slotKeys(DefaultImage())
	if len(got) != len(want) {
		t.Fatalf("DefaultImage has %d slots, want %d:\n got %v\nwant %v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DefaultImage slots:\n got %v\nwant %v", got, want)
		}
	}

	for _, slot := range got {
		switch parseSlotForm(slot) {
		case slotFiveMHz, slotEMG:
			t.Errorf("DefaultImage populates %q — the default image carries no 5 MHz or EMG slot", slot)
		}
	}
}

// TestDefaultImage_CarriesBothTagDisplayValues pins the one shape this radio's
// default image has that no sibling's does. Byte 28 is a LIVE flag here, so an
// image whose every channel answered '0' would leave the fleet's default-fake
// reads indistinguishable from an FTdx10's on the axis this whole milestone
// turns on.
func TestDefaultImage_CarriesBothTagDisplayValues(t *testing.T) {
	img := DefaultImage()
	var on, off int
	for _, s := range img {
		if s.TagDisplay {
			on++
		} else {
			off++
		}
	}
	if on == 0 {
		t.Error("no default channel has its TAG display ON — byte 28 is a live flag on this radio and the image should exercise both values")
	}
	if off == 0 {
		t.Error("every default channel has its TAG display ON — the image should exercise both values")
	}
}

// TestDefaultImage_EachCallIsIndependent pins the Image contract: each call
// returns a fresh map, so two radios built from one Image never share mutable
// slot state.
func TestDefaultImage_EachCallIsIndependent(t *testing.T) {
	a, b := DefaultImage(), DefaultImage()
	a["001"] = MemState{Freq: "000000000"}
	if b["001"].Freq == "000000000" {
		t.Error("mutating one DefaultImage() result changed another — the maps are shared")
	}
	delete(a, "P1L")
	if _, ok := b["P1L"]; !ok {
		t.Error("deleting from one DefaultImage() result changed another")
	}
}

// TestTwoRadiosFromOneImageDoNotAlias pins what a shared map would actually
// break: a write to one live radio showing up in another.
func TestTwoRadiosFromOneImageDoNotAlias(t *testing.T) {
	r1, conn1 := newTestRadio(t, WithFactoryImage(DefaultImage))
	r2, _ := newTestRadio(t, WithFactoryImage(DefaultImage))

	writeFrame(t, conn1, ordinaryChannel("050", '0').frame())
	assertNoReply(t, conn1)

	if _, ok := r1.SlotState("050"); !ok {
		t.Fatal("the write did not land on the first radio")
	}
	if _, ok := r2.SlotState("050"); ok {
		t.Error("a write to one radio appeared in another built from the same Image")
	}
}

func TestWithFactoryImage_ReplacesAndWithSlotOverlays(t *testing.T) {
	empty := func() map[string]MemState { return map[string]MemState{} }

	r, conn := newTestRadio(t, WithFactoryImage(empty))
	if _, ok := r.SlotState("001"); ok {
		t.Error("WithFactoryImage did not REPLACE the default image")
	}
	assertRejected(t, conn, "MT001;")

	state := ordinaryState()
	r2, _ := newTestRadio(t, WithFactoryImage(empty), WithSlot("003", state))
	got, ok := r2.SlotState("003")
	if !ok {
		t.Fatal("WithSlot after WithFactoryImage did not overlay")
	}
	if got != state {
		t.Errorf("overlaid state:\n got %+v\nwant %+v", got, state)
	}
}

// TestWith5MHz_PopulatesASparseBankVisibleViaMRReads reads the bank back over
// the wire, by MR, because MR is the only command that reaches it on this
// radio (MT's legend names memory and PMS alone).
func TestWith5MHz_PopulatesASparseBankVisibleViaMRReads(t *testing.T) {
	r, conn := newTestRadio(t, With5MHz())

	populated := map[string]bool{}
	for _, n := range sparseFiveMHzChannels {
		populated[fiveMHzSlot(n)] = true
	}
	if len(populated) < 3 {
		t.Fatalf("the 5 MHz fixture has %d channels — too few to prove a walk does not stop early", len(populated))
	}

	// Every declared 5 MHz slot of this radio, 501..510, asked in order: the
	// populated ones answer and the rest are refused. A walk that stopped at
	// the first "?;" would miss the ceiling.
	for n := fiveMHzLo; n <= fiveMHzHi; n++ {
		slot := fiveMHzSlot(n)
		reply := exchange(t, conn, "MR"+slot+";")
		if populated[slot] {
			if reply == "?;" {
				t.Errorf("MR%s; -> %q, want a record", slot, reply)
			}
			continue
		}
		if reply != "?;" {
			t.Errorf("MR%s; -> %q, want %q", slot, reply, "?;")
		}
	}

	if !populated[fiveMHzSlot(fiveMHzHi)] {
		t.Error("the fixture does not populate the declared ceiling, so a walk that stopped short would pass")
	}
	if _, ok := r.SlotState(slotEMGWire); ok {
		t.Error("With5MHz populated EMG — the two options are separate")
	}
}

func TestWithEMG_PopulatesTheEmergencyChannelOnly(t *testing.T) {
	r, conn := newTestRadio(t, WithEMG())

	if got := exchange(t, conn, "MREMG;"); got == "?;" {
		t.Error("MREMG; -> \"?;\" after WithEMG()")
	}
	for n := fiveMHzLo; n <= fiveMHzHi; n++ {
		if _, ok := r.SlotState(fiveMHzSlot(n)); ok {
			t.Errorf("WithEMG populated %s — the two options are separate", fiveMHzSlot(n))
		}
	}
}

func TestWith5MHzAndWithEMG_Compose(t *testing.T) {
	_, conn := newTestRadio(t, With5MHz(), WithEMG())
	for _, slot := range []string{fiveMHzSlot(sparseFiveMHzChannels[0]), slotEMGWire} {
		if got := exchange(t, conn, "MR"+slot+";"); got == "?;" {
			t.Errorf("MR%s; -> \"?;\" with both options given", slot)
		}
	}
	// The default image survives both overlays.
	if got := exchange(t, conn, "MT001;"); got == "?;" {
		t.Error("the bank options replaced the default image instead of overlaying it")
	}
}

// TestFiveMHzSlotSpelling pins the helper against this radio's PRINTED
// numbering, 501-510 (ft891_layout.txt:962) — not an ordinal into a bank, and
// not the FTdx10's inherited 501..599.
func TestFiveMHzSlotSpelling(t *testing.T) {
	for n, want := range map[int]string{501: "501", 505: "505", 510: "510"} {
		if got := fiveMHzSlot(n); got != want {
			t.Errorf("fiveMHzSlot(%d) = %q, want %q", n, got, want)
		}
	}
	for _, n := range sparseFiveMHzChannels {
		if parseSlotForm(fiveMHzSlot(n)) != slotFiveMHz {
			t.Errorf("the fixture channel %d spells %q, which this radio's grammar does not admit", n, fiveMHzSlot(n))
		}
	}
}

// TestDefaultImageContentIsInsideThisRadiosPrintedVocabularies is the fixture
// self-check: every default channel must be a frame this radio's own charts
// describe, or the images would be teaching the tests a vocabulary the radio
// has not got.
func TestDefaultImageContentIsInsideThisRadiosPrintedVocabularies(t *testing.T) {
	check := func(t *testing.T, img map[string]MemState) {
		t.Helper()
		for slot, s := range img {
			if parseSlotForm(slot) == slotInvalid {
				t.Errorf("%s: not a slot this radio's legends print", slot)
			}
			if len(s.Freq) != 9 {
				t.Errorf("%s: frequency %q is not the chart's nine digits", slot, s.Freq)
			}
			if !validModeWireByte(s.Mode) || s.Mode == '0' {
				t.Errorf("%s: mode %q is not a nibble this radio's legend prints", slot, s.Mode)
			}
			if !validClarSign(s.ClarSign) || !validClarMagDigits(s.ClarMag) {
				t.Errorf("%s: clarifier %q%q is outside the printed field", slot, s.ClarSign, s.ClarMag)
			}
			if !validCTCSSByte(s.CTCSS) || !validShiftByte(s.Shift) {
				t.Errorf("%s: CTCSS %q or shift %q is outside its printed legend", slot, s.CTCSS, s.Shift)
			}
			if s.Kind != kindMemory {
				t.Errorf("%s: P7 %q, want the answer kind %q", slot, s.Kind, byte(kindMemory))
			}
			if len(s.Tag) > tagFieldLen {
				t.Errorf("%s: tag %q is longer than the 12-byte field", slot, s.Tag)
			}
			if strings.ContainsRune(s.Tag, ';') {
				t.Errorf("%s: tag %q carries the frame terminator", slot, s.Tag)
			}
		}
	}

	t.Run("DefaultImage", func(t *testing.T) { check(t, DefaultImage()) })

	t.Run("with both bank options", func(t *testing.T) {
		r := New(With5MHz(), WithEMG())
		t.Cleanup(func() { _ = r.Close() })
		check(t, r.slots)
	})
}
