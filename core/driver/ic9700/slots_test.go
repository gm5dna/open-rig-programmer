// SPDX-License-Identifier: GPL-3.0-or-later

package ic9700_test

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/driver/ic9700"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// THE SLOT SPELLING IS A CHOICE, and these tests are what make it a
// CHECKED one. Spec D4 defers per-model DisplaySlot cosmetics to the
// roadmap and no radio-facing claim rides on the spelling, but two
// properties do have to hold whatever it is: every slot this radio's
// capabilities advertise must round-trip through the address form the
// wire uses, and no two slots may share a name.
//
// The canonical forms:
//
//	MEM   <band>-<nnn>        144-001, 430-099, 1200-042   channels 1..99
//	SCAN  <band>-P<n><A|B>    144-P1A, 430-P3B             channels 100..105
//	CALL  <band>-C<n>         1200-C1, 144-C2              channels 106, 107
//
// with <band> the printed band name from PDF p.16's `①Frequency band
// codes` table — 144, 430, 1200 — mapping to the WIRE band indices 1, 2
// and 3 that E4's GroupBase makes civ.ChannelAddress.Group carry.

func TestSlotAddressRoundTripsEveryAddressableSlot(t *testing.T) {
	caps := ic9700.CapabilitiesUnverified()
	total := 0
	for _, bank := range caps.Banks {
		for _, slot := range bank.Slots {
			addr, gotBank, err := ic9700.SlotAddress(slot)
			if err != nil {
				t.Fatalf("%s: %v", slot, err)
			}
			if gotBank != bank.ID {
				t.Errorf("%s: bank %s, want %s", slot, gotBank, bank.ID)
			}
			back, backBank, err := ic9700.AddressSlot(addr)
			if err != nil || back != slot {
				t.Errorf("%s -> %v -> %q (err %v)", slot, addr, back, err)
			}
			if backBank != bank.ID {
				t.Errorf("%s -> %v -> bank %s, want %s", slot, addr, backBank, bank.ID)
			}
			total++
		}
	}
	if want := 297 + 18 + 6; total != want {
		t.Fatalf("walked %d slots, want %d (3 bands x 99 MEM, x6 SCAN, x2 CALL)", total, want)
	}
}

func TestSlotNamespacesCannotCollide(t *testing.T) {
	// R4's collision clause is written for the 705/905 sparse models, but
	// the proof is cheap here and a slot spelling that collided would put
	// two different channels at one name.
	seen := map[string]spec.BankID{}
	for _, bank := range ic9700.CapabilitiesUnverified().Banks {
		for _, slot := range bank.Slots {
			if prev, dup := seen[slot]; dup {
				t.Fatalf("slot %q is in both %s and %s", slot, prev, bank.ID)
			}
			seen[slot] = bank.ID
		}
	}
	if len(seen) != 297+18+6 {
		t.Fatalf("%d distinct slot names, want 321", len(seen))
	}
}

func TestSlotAddressRefusesWhatItCannotName(t *testing.T) {
	for _, bad := range []string{"", "144", "144-000", "144-100", "144-P4A",
		"144-C3", "222-001", "144-P1C", "M-01", "G05-012"} {
		if _, _, err := ic9700.SlotAddress(bad); err == nil {
			t.Errorf("SlotAddress(%q) succeeded; it must refuse, never guess", bad)
		}
	}
}

func TestAddressSlotRefusesAnAddressThisRadioHasNoNameFor(t *testing.T) {
	// The reverse direction's own refusal. A decoded answer carries
	// whatever the wire said, and civ's channel-space validation admits
	// 1..107 across bands 1..3 — so a band or channel outside that is a
	// frame this driver must refuse to attribute to a slot rather than
	// render into a name it invented.
	for _, bad := range []civ.ChannelAddress{
		{Group: 0, Channel: 1}, {Group: 4, Channel: 1},
		{Group: 1, Channel: 0}, {Group: 1, Channel: 108},
	} {
		if got, _, err := ic9700.AddressSlot(bad); err == nil {
			t.Errorf("AddressSlot(%v) = %q, want a refusal", bad, got)
		}
	}
}
