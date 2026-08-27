// SPDX-License-Identifier: GPL-3.0-or-later

package ic705

import (
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

func TestSlotNamespacesCannotCollide(t *testing.T) {
	// THE PROOF R4 DEMANDS, over the WHOLE space and not a sample. REV 1
	// gave CALL the strings G100-000..G100-003 while MEM's 1-based mapping
	// also reached G100-xxx, so one string named two banks at once and
	// codeplug's linear bankForSlot scan would have resolved it to
	// whichever bank came first. Injectivity is not something a sample can
	// establish, so this walks all 10 004 display slots.
	caps := capabilitiesUnverified()
	mem, ok := caps.Bank(spec.BankMemory)
	if !ok {
		t.Fatal("no MEM bank")
	}
	call, ok := caps.Bank(spec.BankCall)
	if !ok {
		t.Fatal("no CALL bank")
	}

	owner := make(map[string]spec.BankID, 10004)
	seenAddr := make(map[civ.ChannelAddress]string, 10004)
	memCount, callCount := 0, 0

	check := func(slot string, wantBank spec.BankID) {
		t.Helper()
		addr, bank, err := slotToAddress(slot)
		if err != nil {
			t.Fatalf("slotToAddress(%q): %v", slot, err)
		}
		if bank != wantBank {
			t.Fatalf("slotToAddress(%q) says bank %s, want %s", slot, bank, wantBank)
		}
		if prev, dup := owner[slot]; dup {
			t.Fatalf("slot %q names bank %s AND bank %s — the namespaces collide", slot, prev, bank)
		}
		owner[slot] = bank
		if prev, dup := seenAddr[addr]; dup {
			t.Fatalf("slots %q and %q both map to wire address %v — the mapping is not injective", prev, slot, addr)
		}
		seenAddr[addr] = slot
		back, err := addressToSlot(addr)
		if err != nil {
			t.Fatalf("addressToSlot(%v): %v", addr, err)
		}
		if back != slot {
			t.Fatalf("addressToSlot(slotToAddress(%q)) = %q — the round trip renames the slot", slot, back)
		}
	}

	for g := 1; g <= 100; g++ {
		for c := 1; c <= 100; c++ {
			slot := spec.SparseSlot(g, c)
			check(slot, spec.BankMemory)
			memCount++
			if !mem.WithinSpace(slot) {
				t.Fatalf("MEM bank does not admit its own slot %q", slot)
			}
		}
	}
	for c := 1; c <= 4; c++ {
		slot := spec.SparseSlot(101, c)
		check(slot, spec.BankCall)
		callCount++
		if mem.WithinSpace(slot) {
			t.Fatalf("MEM bank admits CALL slot %q — G101 must lie outside MEM's 100-group space", slot)
		}
		if !call.WithinSpace(slot) {
			t.Fatalf("CALL bank does not list its own slot %q", slot)
		}
	}

	if memCount != 10000 || callCount != 4 {
		t.Fatalf("walked %d MEM and %d CALL slots, want 10 000 and 4 — the sweep is not the whole space", memCount, callCount)
	}
	if len(owner) != 10004 {
		t.Fatalf("the two namespaces hold %d distinct strings, want 10 004", len(owner))
	}
}

func TestMemBoundariesMapToTheRightWireIndices(t *testing.T) {
	// The off-by-one this test exists to catch reads the WRONG CHANNEL
	// silently: wire = display - 1, both group and channel.
	for _, tc := range []struct {
		slot string
		want civ.ChannelAddress
	}{
		{"G01-001", civ.ChannelAddress{Group: 0, Channel: 0}},
		{"G02-001", civ.ChannelAddress{Group: 1, Channel: 0}},
		{"G01-100", civ.ChannelAddress{Group: 0, Channel: 99}},
		{"G100-100", civ.ChannelAddress{Group: 99, Channel: 99}},
	} {
		got, bank, err := slotToAddress(tc.slot)
		if err != nil {
			t.Errorf("slotToAddress(%q): %v", tc.slot, err)
			continue
		}
		if got != tc.want || bank != spec.BankMemory {
			t.Errorf("slotToAddress(%q) = %v/%s, want %v/MEM", tc.slot, got, bank, tc.want)
		}
	}
}

func TestCallSlotsMapToWireGroup100(t *testing.T) {
	// The CALL group is the manual's 0100 (matrix §1b), wire bytes 01 00,
	// and E4's decided semantics put the WIRE index in ChannelAddress.Group.
	for c := 1; c <= 4; c++ {
		slot := spec.SparseSlot(101, c)
		got, bank, err := slotToAddress(slot)
		if err != nil {
			t.Fatalf("slotToAddress(%q): %v", slot, err)
		}
		want := civ.ChannelAddress{Group: 100, Channel: c - 1}
		if got != want || bank != spec.BankCall {
			t.Errorf("slotToAddress(%q) = %v/%s, want %v/CALL", slot, got, bank, want)
		}
	}
}

func TestCallChannelsAboveFourAreRefusedBeforeAnyBuilder(t *testing.T) {
	// O-9's FIRST line of defence and, after the 24/08/2026 ruling
	// (DEFERRED), its primary one. civ carries ONE channel range per
	// profile, so the gate itself admits CALL channels 4-99; this refusal,
	// Task 11's bank check and doc.go's recording are what keep an
	// undocumented call-channel address off the wire. SWEPT, not sampled.
	for c := 5; c <= 100; c++ {
		slot := spec.SparseSlot(101, c)
		addr, bank, err := slotToAddress(slot)
		if err == nil {
			t.Fatalf("slotToAddress(%q) returned %v/%s — the CALL group holds only channels 0000-0003", slot, addr, bank)
		}
		if !strings.Contains(err.Error(), "0000") || !strings.Contains(err.Error(), "0003") {
			t.Errorf("slotToAddress(%q) refused with %q, which does not name the CALL group's documented range 0000-0003", slot, err)
		}
	}
}

func TestMalformedSlotStringsAreRefused(t *testing.T) {
	// Each an ERROR, none a panic. G001-001 and G001- are the REV 3 cases:
	// spec.ParseSparseSlot is a strict re-render of spec.SparseSlot's
	// canonical G%02d-%03d, so a three-digit group 1 is refused and the
	// tier has exactly one spelling per address.
	for _, slot := range []string{
		"", "G", "G-1-1", "MEM01", "G00-001", "G101-000", "G01-101",
		"G001-001", "G001-", "G01-000", "G102-001", "g01-001", "G01-001 ",
	} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("slotToAddress(%q) panicked: %v", slot, r)
				}
			}()
			if addr, bank, err := slotToAddress(slot); err == nil {
				t.Errorf("slotToAddress(%q) accepted a malformed slot, returning %v/%s", slot, addr, bank)
			}
		}()
	}
}

func TestParseSparseSlotAcceptsAThreeDigitGroup(t *testing.T) {
	// Confirmed BY TEST rather than assumed (Task 8 step 2): the CALL
	// namespace is G101, and it only exists at all if the landed strict
	// parser renders and re-reads a three-digit group.
	g, c, ok := spec.ParseSparseSlot("G101-004")
	if !ok || g != 101 || c != 4 {
		t.Fatalf("ParseSparseSlot(\"G101-004\") = %d, %d, %v — the CALL namespace depends on this", g, c, ok)
	}
	if got := spec.SparseSlot(101, 4); got != "G101-004" {
		t.Fatalf("SparseSlot(101, 4) = %q, want \"G101-004\"", got)
	}
}

func TestAddressToSlotRefusesAnAddressOutsideBothBanks(t *testing.T) {
	// The reverse direction fails closed too: a CALL channel above 3 and a
	// group beyond the radio's own space have no display form, and
	// inventing one would name a slot no bank holds.
	for _, addr := range []civ.ChannelAddress{
		{Group: 100, Channel: 4},
		{Group: 100, Channel: 99},
		{Group: 101, Channel: 0},
		{Group: 0, Channel: 100},
		{Group: -1, Channel: 0},
	} {
		if slot, err := addressToSlot(addr); err == nil {
			t.Errorf("addressToSlot(%v) = %q, want an error", addr, slot)
		}
	}
}
