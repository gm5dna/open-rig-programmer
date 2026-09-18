// SPDX-License-Identifier: GPL-3.0-or-later

package fakeftx1

import "testing"

func TestDefaultImage_EachCallIsIndependent(t *testing.T) {
	slotsA, tagsA := DefaultImage()
	slotsB, tagsB := DefaultImage()
	slotsA["00001"] = MemState{Freq: "999999999"}
	tagsA["00001"] = "MUTATED     "
	if slotsB["00001"].Freq == "999999999" {
		t.Fatal("mutating one DefaultImage() call's slot map changed another's — the maps alias")
	}
	if tagsB["00001"] == "MUTATED     " {
		t.Fatal("mutating one DefaultImage() call's tag map changed another's — the maps alias")
	}
}

func TestDefaultImage_HasContentInEveryBank(t *testing.T) {
	slots, _ := DefaultImage()
	banks := map[slotKind]bool{}
	for addr := range slots {
		kind := classifySlot(addr)
		if kind == slotInvalid {
			t.Errorf("DefaultImage carries address %q, outside FTX-1's own address grammar", addr)
		}
		banks[kind] = true
	}
	for _, want := range []slotKind{slotMemory, slotPMS, slotFiveMHz, slotEMG} {
		if !banks[want] {
			t.Errorf("DefaultImage has no seeded address in bank %v", want)
		}
	}
}

func TestDefaultImage_TagsReferenceRealSlots(t *testing.T) {
	slots, tags := DefaultImage()
	for addr, tag := range tags {
		if len(tag) != tagWireLen {
			t.Errorf("tag for %q is %d bytes, want the fixed %d-byte wire width", addr, len(tag), tagWireLen)
		}
		if !validTag([]byte(tag)) {
			t.Errorf("tag for %q = %q, not a valid wire tag", addr, tag)
		}
		if _, ok := slots[addr]; !ok {
			t.Errorf("DefaultImage tags address %q with no corresponding memory-block slot", addr)
		}
	}
}

func TestEncodeFreqDigits(t *testing.T) {
	got, err := encodeFreqDigits(7_000_000)
	if err != nil || got != "007000000" {
		t.Errorf("encodeFreqDigits(7_000_000) = %q, %v, want \"007000000\", nil", got, err)
	}
	if _, err := encodeFreqDigits(1_000_000_000); err == nil {
		t.Error("encodeFreqDigits(1_000_000_000) did not refuse a 10-digit value on a 9-digit field")
	}
}

func TestPadTag(t *testing.T) {
	got := padTag("HI")
	if len(got) != tagWireLen {
		t.Fatalf("padTag(%q) is %d bytes, want %d", "HI", len(got), tagWireLen)
	}
	if got != "HI          " {
		t.Errorf("padTag(%q) = %q, want %q", "HI", got, "HI          ")
	}
}
