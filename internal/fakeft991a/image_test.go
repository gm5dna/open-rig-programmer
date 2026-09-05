// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft991a

import (
	"sort"
	"testing"
)

func slotKeys(img map[string]MemState) []string {
	keys := make([]string, 0, len(img))
	for k := range img {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// TestDefaultImage_IsMinimalAndExactlyEnumerated states the whole inventory,
// so an accidental addition fails a test rather than quietly making some other
// suite's "this slot is empty" assertion a fixture accident.
//
// The shape is this milestone's plan, decision P14: at least ONE memory
// channel and at least ONE PMS slot populated (the fleet's
// read-every-registered-model pins must be non-vacuous, and the PMS bank is
// where this radio is unlike every registered sibling); NO slot outside
// 001-117, because this radio has none; and NO DCS-state channel — that lives
// in WithDCSChannels, so the default image round-trips through every fleet pin
// that predates the five-state vocabulary.
func TestDefaultImage_IsMinimalAndExactlyEnumerated(t *testing.T) {
	want := []string{
		"001", "002",
		"100", "101", // P-1L and P-1U, the numbering's floor
		"116", "117", // P-9L and P-9U, its ceiling
	}
	got := slotKeys(DefaultImage())
	if len(got) != len(want) {
		t.Fatalf("DefaultImage() holds %d slots, want %d:\n got %v\nwant %v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DefaultImage() slots:\n got %v\nwant %v", got, want)
		}
	}
}

// TestDefaultImage_MeetsP14 states the plan's three constraints as assertions
// rather than as a comment, so a later edit that satisfies the enumeration
// above while breaking one of them still fails.
func TestDefaultImage_MeetsP14(t *testing.T) {
	img := DefaultImage()

	var memories, pms int
	for slot, s := range img {
		switch parseSlotForm(slot) {
		case slotMemory:
			memories++
		case slotPMS:
			pms++
		default:
			t.Errorf("slot %q is outside the 001-117 span this radio has", slot)
		}
		if s.CTCSS == '3' || s.CTCSS == '4' {
			t.Errorf("slot %q carries a DCS P8 state (%q); P14 keeps those out of the default image", slot, s.CTCSS)
		}
	}
	if memories == 0 {
		t.Error("no memory channel is populated — the fleet's read pins would be vacuous")
	}
	if pms == 0 {
		t.Error("no PMS slot is populated — the PMS bank is where this radio is unlike every sibling")
	}
}

func TestDefaultImage_EachCallIsIndependent(t *testing.T) {
	a := DefaultImage()
	b := DefaultImage()
	a["001"] = MemState{Freq: "000000000"}
	if b["001"].Freq == "000000000" {
		t.Error("two DefaultImage() calls share a map — every Image must return an independent one")
	}
}

// TestTwoRadiosFromOneImageDoNotAlias pins what a shared map would actually
// break: a write to one live radio showing up in another.
func TestTwoRadiosFromOneImageDoNotAlias(t *testing.T) {
	_, connA := newTestRadio(t, WithFactoryImage(DefaultImage))
	rB, _ := newTestRadio(t, WithFactoryImage(DefaultImage))

	writeFrame(t, connA, ordinaryChannel("050", mtSetKindFixed).frame())
	assertNoReply(t, connA)
	if _, ok := rB.SlotState("050"); ok {
		t.Error("a write to one radio reached another built from the same Image")
	}
}

func TestWithFactoryImage_ReplacesAndWithSlotOverlays(t *testing.T) {
	empty := func() map[string]MemState { return map[string]MemState{} }
	r, _ := newTestRadio(t, WithFactoryImage(empty), WithSlot("003", ordinaryState()))

	if _, ok := r.SlotState("001"); ok {
		t.Error("WithFactoryImage did not replace the default image")
	}
	if got, ok := r.SlotState("003"); !ok || got != ordinaryState() {
		t.Errorf("WithSlot overlay: got %+v, ok=%v", got, ok)
	}
}

// TestWithDCSChannels_PopulatesTheTwoDCSStatesOnly is P14's dedicated option:
// the two states the default image deliberately lacks, reachable without a
// Set, for the layers that need a DCS channel to read.
func TestWithDCSChannels_PopulatesTheTwoDCSStatesOnly(t *testing.T) {
	base := slotKeys(DefaultImage())
	r, conn := newTestRadio(t, WithDCSChannels())

	states := map[byte]int{}
	for _, slot := range append([]string{}, dcsChannelSlots...) {
		s, ok := r.SlotState(slot)
		if !ok {
			t.Fatalf("WithDCSChannels did not populate %q", slot)
		}
		states[s.CTCSS]++
		if got := mustBeAnMRAnswer(t, exchange(t, conn, "MR"+slot+";")); got[23] != s.CTCSS {
			t.Errorf("MR%s; position 24 = %q, want %q", slot, got[23], s.CTCSS)
		}
	}
	if states['3'] != 1 || states['4'] != 1 {
		t.Errorf("the option's P8 states are %v, want one '3' (DCS ENC/DEC) and one '4' (DCS ENC)", states)
	}

	// Overlay semantics: it adds to the image already present, it does not
	// replace it.
	for _, slot := range base {
		if _, ok := r.SlotState(slot); !ok {
			t.Errorf("WithDCSChannels removed the default image's slot %q", slot)
		}
	}
}

// TestPMSSlotSpelling holds the wire numbers against the MC legend's own line,
// "100: P-1L 101: P-1U ~ 116: P-9L 117: P-9U" (ft991a_layout.txt:916).
//
// IT IS THE CELL A COPY FROM internal/fakeft891 GETS WRONG: that radio's PMS
// slots are the token "P1L"…"P9U", and eighteen of those strings would be
// eighteen non-slots here rather than a compile error.
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

// TestDefaultImageContentIsInsideThisRadiosPrintedVocabularies checks the
// FIXTURES rather than the parser: an image constant outside a printed
// vocabulary would produce an answer core/cat would rightly refuse, and it
// would do so only in whatever test happened to read that slot.
func TestDefaultImageContentIsInsideThisRadiosPrintedVocabularies(t *testing.T) {
	check := func(t *testing.T, slot string, s MemState) {
		t.Helper()
		if len(s.Freq) != 9 {
			t.Errorf("%s: frequency field %q is not the counted nine digits", slot, s.Freq)
		}
		for _, b := range []byte(s.Freq) {
			if !isDigit(b) {
				t.Errorf("%s: frequency field %q is not all digits", slot, s.Freq)
				break
			}
		}
		if !validClarSign(s.ClarSign) {
			t.Errorf("%s: clarifier sign %q is outside the printed +/-", slot, s.ClarSign)
		}
		if !validClarMagDigits(s.ClarMag) {
			t.Errorf("%s: clarifier magnitude %q is outside the printed 0000-9999", slot, s.ClarMag)
		}
		if !validModeBuildByte(s.Mode) {
			t.Errorf("%s: mode nibble %q is not one this radio's legend prints", slot, s.Mode)
		}
		if s.Kind != kindMemory {
			t.Errorf("%s: answer kind %q, want %q", slot, s.Kind, kindMemory)
		}
		if !validCTCSSByte(s.CTCSS) {
			t.Errorf("%s: P8 %q is outside the five printed states", slot, s.CTCSS)
		}
		if !validShiftByte(s.Shift) {
			t.Errorf("%s: P10 %q is outside the three printed values", slot, s.Shift)
		}
		if len(s.Tag) > tagFieldLen || !validTagField([]byte(s.Tag)) {
			t.Errorf("%s: tag %q is outside the field's width or byte rule", slot, s.Tag)
		}
	}

	for slot, s := range DefaultImage() {
		check(t, slot, s)
	}
	r := New(WithDCSChannels())
	t.Cleanup(func() { _ = r.Close() })
	for _, slot := range dcsChannelSlots {
		s, ok := r.SlotState(slot)
		if !ok {
			t.Fatalf("WithDCSChannels did not populate %q", slot)
		}
		check(t, slot, s)
	}
}
