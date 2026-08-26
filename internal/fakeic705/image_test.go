// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic705

import (
	"bytes"
	"testing"
)

func TestRecordLenIsTheDiagramsOwnArithmetic(t *testing.T) {
	// The memory-content diagram's byte positions run 1 to 115, of which 1-4
	// are the group and channel address. Both transcripts under
	// core/civ/ic705/testdata measure the same 115 independently. 115 - 4 = 111.
	const diagramBytes = 115
	if RecordLen != diagramBytes-addressLen {
		t.Errorf("RecordLen = %d, want %d - %d = %d", RecordLen, diagramBytes, addressLen, diagramBytes-addressLen)
	}
}

func TestBlankRecord(t *testing.T) {
	rec := BlankRecord()
	if len(rec) != RecordLen {
		t.Fatalf("BlankRecord() is %d bytes, want %d", len(rec), RecordLen)
	}
	if !bytes.Equal(rec, make([]byte, RecordLen)) {
		t.Errorf("BlankRecord() = % X, want all zero bytes", rec)
	}

	other := BlankRecord()
	rec[0] = 0xFF
	if other[0] != 0x00 {
		t.Error("BlankRecord() returns a shared slice; each call must return a fresh one")
	}
}

func TestDefaultImage_IsSparseAndCrossesGroups(t *testing.T) {
	img := DefaultImage()
	if len(img) == 0 {
		t.Fatal("DefaultImage() is empty; it exists to give a walk something to find")
	}

	groups := map[int]bool{}
	for slot, rec := range img {
		groups[slot.Group] = true
		if len(rec) != RecordLen {
			t.Errorf("slot %s holds %d bytes, want %d — a fixture's records must be acceptable ones", slot, len(rec), RecordLen)
		}
		if !inRange(slot.Group, slot.Channel) {
			t.Errorf("slot %s is outside the ranges the manual states, so nothing could ever read it", slot)
		}
	}
	if len(groups) < 2 {
		t.Errorf("DefaultImage() occupies %d group(s); a walk needs a boundary to cross", len(groups))
	}
	if !groups[callChannelGroup] {
		t.Error("DefaultImage() occupies no call channel; that group is the one with its own channel range")
	}

	// Sparse: at least one gap inside an occupied group, so a walk has
	// something to skip as well as something to find.
	if _, occupied := img[Slot{Group: 0, Channel: 2}]; occupied {
		t.Error("DefaultImage() has no gap in group 0; a dense image cannot show a skip")
	}
}

func TestDefaultImage_IsFreshEachCall(t *testing.T) {
	first := DefaultImage()
	want := len(first)

	victim := Slot{Group: 0, Channel: 0}
	if _, ok := first[victim]; !ok {
		t.Fatalf("this test's premise is wrong: DefaultImage() does not occupy %s", victim)
	}
	first[victim][0] = 0xFF
	delete(first, victim)

	second := DefaultImage()
	if len(second) != want {
		t.Errorf("the second DefaultImage() holds %d slots after the first was shortened, want %d — each call must return a fresh map", len(second), want)
	}
	if rec, ok := second[victim]; !ok {
		t.Errorf("%s was deleted from the first image and is missing from the second", victim)
	} else if rec[0] != 0x00 {
		t.Errorf("%s carries %02X in the second image after the first was mutated; each call must return fresh records", victim, rec[0])
	}
}

func TestEmptyImage(t *testing.T) {
	img := EmptyImage()
	if img == nil {
		t.Fatal("EmptyImage() is nil; it must be a usable empty map")
	}
	if len(img) != 0 {
		t.Errorf("EmptyImage() holds %d slots, want 0", len(img))
	}
	img.With(0, 0, BlankRecord()) // must not panic on a nil map
}

func TestImageClone_IsIndependent(t *testing.T) {
	original := EmptyImage().With(0, 1, BlankRecord())
	clone := original.Clone()

	clone[Slot{Group: 0, Channel: 1}][0] = 0xFF
	clone.With(0, 2, BlankRecord())

	if original[Slot{Group: 0, Channel: 1}][0] != 0x00 {
		t.Error("mutating a clone's record reached the original")
	}
	if _, present := original[Slot{Group: 0, Channel: 2}]; present {
		t.Error("adding to a clone reached the original")
	}
}

func TestImageWith_CopiesTheRecord(t *testing.T) {
	rec := BlankRecord()
	img := EmptyImage().With(0, 1, rec)
	rec[0] = 0xFF
	if img[Slot{Group: 0, Channel: 1}][0] != 0x00 {
		t.Error("With kept the caller's slice; it must copy")
	}
}

func TestImageWith_TakesAnyLength(t *testing.T) {
	img := EmptyImage().With(0, 1, bytes.Repeat([]byte{0x02}, 39))
	if got := len(img[Slot{Group: 0, Channel: 1}]); got != 39 {
		t.Errorf("With stored %d bytes, want 39 — an image is where a wrong-length record comes from", got)
	}
}

func TestWithFactoryImage_ClonesAndReplaces(t *testing.T) {
	img := DefaultImage()
	r := New(WithFactoryImage(img))
	defer r.Close()

	// The radio holds what the image held.
	for slot := range img {
		if _, occupied := r.SlotState(slot.Group, slot.Channel); !occupied {
			t.Errorf("slot %s is in the image but not in the radio", slot)
		}
	}

	// And the caller's image is untouched by the radio's own writes.
	r.storeRecord(0, 0, bytes.Repeat([]byte{0xAA}, RecordLen))
	if img[Slot{Group: 0, Channel: 0}][0] != 0x00 {
		t.Error("a write to the radio reached the caller's image; WithFactoryImage must clone")
	}
}

func TestWithFactoryImage_NilMeansEmpty(t *testing.T) {
	r := New(WithFactoryImage(nil))
	defer r.Close()
	if _, occupied := r.SlotState(0, 0); occupied {
		t.Error("WithFactoryImage(nil) left the radio holding something")
	}
}

// TestWithFactoryImage_ThenWithRecord pins the ordering the option's doc
// comment states, because getting it wrong is silent.
func TestWithFactoryImage_ThenWithRecord(t *testing.T) {
	short := bytes.Repeat([]byte{0x03}, 39)
	r := New(WithFactoryImage(DefaultImage()), WithRecord(0, 0, short))
	defer r.Close()

	rec, occupied := r.SlotState(0, 0)
	if !occupied || !bytes.Equal(rec, short) {
		t.Errorf("the later WithRecord did not win: got (% X, %v)", rec, occupied)
	}
	if _, occupied := r.SlotState(0, 1); !occupied {
		t.Error("the image's other slots were lost")
	}
}

func TestNewWithNoOptionsIsAnEmptyRadio(t *testing.T) {
	r := New()
	defer r.Close()

	for _, slot := range []Slot{{0, 0}, {0, 1}, {1, 0}, {callChannelGroup, 0}} {
		if _, occupied := r.SlotState(slot.Group, slot.Channel); occupied {
			t.Errorf("a radio built with no options holds %s; New must not seed anything", slot)
		}
	}
}

func TestSlotString(t *testing.T) {
	if got := (Slot{Group: 100, Channel: 3}).String(); got != "0100/0003" {
		t.Errorf("Slot.String() = %q, want %q", got, "0100/0003")
	}
}

func TestMustBeAddressable_PanicsOnANumberTheFieldCannotCarry(t *testing.T) {
	tests := []struct {
		name           string
		group, channel int
	}{
		{"a negative group", -1, 0},
		{"a negative channel", 0, -1},
		{"a group past four digits", 10000, 0},
		{"a channel past four digits", 0, 10000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("WithRecord(%d, %d, …) did not panic", tt.group, tt.channel)
				}
			}()
			WithRecord(tt.group, tt.channel, BlankRecord())
		})
	}
}

// TestMustBeAddressable_AllowsASlotTheWireWouldRefuse: seeding is deliberately
// wider than addressing. See WithRecord's doc comment.
func TestMustBeAddressable_AllowsASlotTheWireWouldRefuse(t *testing.T) {
	r := New(WithRecord(500, 500, BlankRecord()))
	defer r.Close()

	if _, occupied := r.SlotState(500, 500); !occupied {
		t.Fatal("a slot outside the manual's ranges could not be seeded")
	}
	if inRange(500, 500) {
		t.Fatal("this test's premise is wrong: group 500 channel 500 is in range")
	}

	w := dial(t, r)
	w.send(readFrame(0x05, 0x00, 0x05, 0x00)...)
	if got := w.next(); !bytes.Equal(got, nakBytes) {
		t.Errorf("a read of a seeded but unaddressable slot drew % X, want % X", got, nakBytes)
	}
}
