// SPDX-License-Identifier: GPL-3.0-or-later

package icr8600_test

import (
	"bytes"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/icr8600"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

func TestAddressSpaceIsOnlyTheNormalMemoryRectangle(t *testing.T) {
	p := icr8600.Profile()
	if got := p.AddressForm(); got != civ.AddressFormWideGroupChannel {
		t.Errorf("AddressForm() = %v, want AddressFormWideGroupChannel", got)
	}
	if icr8600.MemoryGroups != 100 {
		t.Errorf("MemoryGroups constant = %d, want 100", icr8600.MemoryGroups)
	}
	if got := p.Groups(); got != icr8600.MemoryGroups {
		t.Errorf("Groups() = %d, want 100", got)
	}
	if icr8600.MemoryGroupBase != 0 {
		t.Errorf("MemoryGroupBase constant = %d, want 0", icr8600.MemoryGroupBase)
	}
	if got := p.GroupBase(); got != icr8600.MemoryGroupBase {
		t.Errorf("GroupBase() = %d, want 0", got)
	}
	if lo, hi := p.ChannelRange(); lo != icr8600.MemoryChannelBase || hi != 99 {
		t.Errorf("ChannelRange() = (%d, %d), want (0, 99)", lo, hi)
	}

	for _, address := range []civ.ChannelAddress{
		{Group: 0, Channel: 0},
		{Group: 99, Channel: 99},
	} {
		if _, err := p.BuildMemoryRead(address); err != nil {
			t.Errorf("BuildMemoryRead(%v): %v", address, err)
		}
	}

	invalid := []struct {
		name    string
		address civ.ChannelAddress
	}{
		{"group below range", civ.ChannelAddress{Group: -1, Channel: 0}},
		{"channel below range", civ.ChannelAddress{Group: 0, Channel: -1}},
		{"channel above range", civ.ChannelAddress{Group: 0, Channel: 100}},
		{"0100 excluded", civ.ChannelAddress{Group: 100, Channel: 0}},
		{"0101 excluded", civ.ChannelAddress{Group: 101, Channel: 0}},
		// 0102 is counter-evidence for clear and unresolved for ordinary
		// memory addressing (register icr8600-scan-edge-encoding, Stage R
		// lift: capture a program-scan-edge read). It is not an ExtraRange.
		{"0102 unresolved", civ.ChannelAddress{Group: 102, Channel: 0}},
		{"rectangular closure", civ.ChannelAddress{Group: 102, Channel: 100}},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			cmd, err := p.BuildMemoryRead(tc.address)
			if err == nil {
				t.Fatalf("BuildMemoryRead(%v) succeeded with % X, want out-of-space failure", tc.address, cmd.Bytes())
			}
			if !cmd.IsZero() {
				t.Errorf("BuildMemoryRead(%v) returned non-zero command % X with error %v", tc.address, cmd.Bytes(), err)
			}
		})
	}
}

func TestAddressEncodingUsesFourPackedBCDBytes(t *testing.T) {
	cmd, err := icr8600.Profile().BuildMemoryRead(civ.ChannelAddress{Group: 99, Channel: 99})
	if err != nil {
		t.Fatalf("BuildMemoryRead: %v", err)
	}
	want := []byte{0xFE, 0xFE, 0x96, 0xE0, 0x1A, 0x00, 0x00, 0x99, 0x00, 0x99, 0xFD}
	if got := cmd.Bytes(); !bytes.Equal(got, want) {
		t.Errorf("BuildMemoryRead(99/99) = % X, want % X", got, want)
	}
}

func TestAddressSparseMemoryBankUsesZeroBasedCanonicalSlots(t *testing.T) {
	bank := icr8600.MemoryBank()
	if bank.ID != spec.BankMemory || !bank.Sparse {
		t.Errorf("MemoryBank() identity = %q Sparse=%v, want MEM sparse", bank.ID, bank.Sparse)
	}
	if bank.Groups != 100 || bank.GroupBase != 0 || bank.PerGroup != 100 || bank.ChannelBase != 0 {
		t.Errorf("MemoryBank() geometry = groups %d base %d per-group %d channel-base %d, want 100/0/100/0", bank.Groups, bank.GroupBase, bank.PerGroup, bank.ChannelBase)
	}
	if bank.Budget != 0 || !bank.BudgetUnstated {
		// The capacity is unresolved: register icr8600-budget, Stage R
		// lift: fill normal memories until the receiver refuses another.
		t.Errorf("MemoryBank() budget = %d unstated=%v, want 0/true — icr8600-budget", bank.Budget, bank.BudgetUnstated)
	}
	if len(bank.Slots) != 0 {
		t.Errorf("MemoryBank().Slots = %v, want no materialised slots in the static table", bank.Slots)
	}
	if got := spec.SparseSlot(0, 0); got != "G00-000" {
		t.Errorf("SparseSlot(0, 0) = %q, want G00-000", got)
	}
	for _, slot := range []string{"G00-000", "G99-099"} {
		if !bank.WithinSpace(slot) {
			t.Errorf("MemoryBank().WithinSpace(%q) = false", slot)
		}
	}
	for _, slot := range []string{"G100-000", "G101-000", "G102-000", "G99-100", "G102-100"} {
		if bank.WithinSpace(slot) {
			t.Errorf("MemoryBank().WithinSpace(%q) = true, want excluded", slot)
		}
	}

	again := icr8600.MemoryBank()
	bank.Slots = append(bank.Slots, "G00-000")
	if len(again.Slots) != 0 {
		t.Errorf("MemoryBank() shares mutable Slots storage: second value has %v", again.Slots)
	}
}
