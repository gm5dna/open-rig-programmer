// SPDX-License-Identifier: GPL-3.0-or-later

package csvmerge

import (
	"errors"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
)

// buildBase returns a synthetic two-slot Codeplug (MYCALL-style
// fixtures, synthetic only) used across this file's tests.
func buildBase() *codeplug.Codeplug {
	return &codeplug.Codeplug{
		Schema: codeplug.CurrentSchema,
		Channels: []codeplug.Channel{
			{Slot: "001", Data: &codeplug.ChannelData{FreqHz: 7_000_000, Mode: "USB", Tag: "MYCALL1", TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false}}},
			{Slot: "002"},
		},
	}
}

// TestMergeCSV_ExactInventoryReplacesChannelsWholesale pins MergeCSV's
// core rule (task-13 brief §2, moved from cmd/rigprog/import.go's
// mergeCSV by task-15): when imported's slot inventory matches base's
// EXACTLY, base.Channels is replaced wholesale with imported.
func TestMergeCSV_ExactInventoryReplacesChannelsWholesale(t *testing.T) {
	base := buildBase()
	imported := []codeplug.Channel{
		{Slot: "001", Data: &codeplug.ChannelData{FreqHz: 14_000_000, Mode: "USB", Tag: "MYCALL2", TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false}}},
		{Slot: "002", Data: &codeplug.ChannelData{FreqHz: 14_100_000, Mode: "USB", Tag: "MYCALL3", TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false}}},
	}
	if err := MergeCSV(base, imported); err != nil {
		t.Fatalf("MergeCSV: unexpected error: %v", err)
	}
	if len(base.Channels) != 2 || base.Channels[0].Data.FreqHz != 14_000_000 || base.Channels[1].Data.Tag != "MYCALL3" {
		t.Errorf("MergeCSV: base.Channels = %+v, want it replaced wholesale by imported", base.Channels)
	}
}

// TestMergeCSV_InventoryMismatchRefusesAndNamesSlots pins MergeCSV's
// refusal path: a missing or extra slot refuses the merge outright
// (base untouched) and names every offending slot.
func TestMergeCSV_InventoryMismatchRefusesAndNamesSlots(t *testing.T) {
	base := buildBase()
	original := append([]codeplug.Channel(nil), base.Channels...)
	imported := []codeplug.Channel{
		{Slot: "001", Data: &codeplug.ChannelData{FreqHz: 7_000_000, Mode: "USB", TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false}}},
		{Slot: "003", Data: &codeplug.ChannelData{FreqHz: 7_000_000, Mode: "USB", TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false}}},
	}
	err := MergeCSV(base, imported)
	if err == nil {
		t.Fatal("MergeCSV: expected an inventory-mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "002") || !strings.Contains(err.Error(), "003") {
		t.Errorf("MergeCSV error = %q, want it to name both the missing (002) and extra (003) slots", err.Error())
	}
	if len(base.Channels) != len(original) || base.Channels[0].Slot != original[0].Slot {
		t.Errorf("MergeCSV: base.Channels mutated on refusal: %+v", base.Channels)
	}

	// A caller (cmd/rigprog) wanting its own wording (e.g. naming its
	// --csv/--into flags) needs the structured fields, not just the
	// generic Error() text — see InventoryMismatchError's doc comment.
	var mismatch *InventoryMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("MergeCSV error = %T, want *InventoryMismatchError", err)
	}
	if len(mismatch.Missing) != 1 || mismatch.Missing[0] != "002" {
		t.Errorf("InventoryMismatchError.Missing = %v, want [002]", mismatch.Missing)
	}
	if len(mismatch.Extra) != 1 || mismatch.Extra[0] != "003" {
		t.Errorf("InventoryMismatchError.Extra = %v, want [003]", mismatch.Extra)
	}
}

// TestMergeCHIRP_SparseMergeBySlot pins MergeCHIRP's core rule: every
// base slot NOT touched by imported keeps its current contents; every
// imported slot overwrites the matching base slot.
func TestMergeCHIRP_SparseMergeBySlot(t *testing.T) {
	base := buildBase()
	imported := []codeplug.Channel{
		{Slot: "002", Data: &codeplug.ChannelData{FreqHz: 14_200_000, Mode: "USB", Tag: "MYCALL4", TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false}}},
	}
	if err := MergeCHIRP(base, imported); err != nil {
		t.Fatalf("MergeCHIRP: unexpected error: %v", err)
	}
	if base.Channels[0].Data == nil || base.Channels[0].Data.Tag != "MYCALL1" {
		t.Errorf("MergeCHIRP: untouched slot 001 changed: %+v", base.Channels[0])
	}
	if base.Channels[1].Data == nil || base.Channels[1].Data.Tag != "MYCALL4" {
		t.Errorf("MergeCHIRP: slot 002 not merged: %+v", base.Channels[1])
	}
}

// TestMergeCHIRP_UnknownSlotRefusesWholesale pins MergeCHIRP's refusal
// when an imported slot does not exist in base's inventory at all:
// refused before any mutation, naming the offending slot.
func TestMergeCHIRP_UnknownSlotRefusesWholesale(t *testing.T) {
	base := buildBase()
	imported := []codeplug.Channel{
		{Slot: "099", Data: &codeplug.ChannelData{FreqHz: 7_000_000, Mode: "USB", TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false}}},
	}
	err := MergeCHIRP(base, imported)
	if err == nil {
		t.Fatal("MergeCHIRP: expected an unknown-slot error, got nil")
	}
	if !strings.Contains(err.Error(), "099") {
		t.Errorf("MergeCHIRP error = %q, want it to name slot 099", err.Error())
	}
	var unknown *UnknownSlotsError
	if !errors.As(err, &unknown) {
		t.Fatalf("MergeCHIRP error = %T, want *UnknownSlotsError", err)
	}
	if len(unknown.Slots) != 1 || unknown.Slots[0] != "099" {
		t.Errorf("UnknownSlotsError.Slots = %v, want [099]", unknown.Slots)
	}
}

// TestMergeCHIRP_DuplicateLocationRefusesWholesale pins Fix 5
// (adjudicated MEDIUM, Codex M4 #5, moved from cmd/rigprog/import.go's
// mergeCHIRP by task-15): two imported rows targeting the same slot
// refuse the merge wholesale (never last-wins), naming the duplicated
// slot in DISPLAY form.
func TestMergeCHIRP_DuplicateLocationRefusesWholesale(t *testing.T) {
	base := buildBase()
	imported := []codeplug.Channel{
		{Slot: "002", Data: &codeplug.ChannelData{FreqHz: 7_100_000, Mode: "USB", Tag: "MYCALL5", TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false}}},
		{Slot: "002", Data: &codeplug.ChannelData{FreqHz: 7_150_000, Mode: "USB", Tag: "MYCALL6", TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false}}},
	}
	err := MergeCHIRP(base, imported)
	if err == nil {
		t.Fatal("MergeCHIRP: expected a duplicate-slot error, got nil")
	}
	if !strings.Contains(err.Error(), "M-02") {
		t.Errorf("MergeCHIRP error = %q, want it to name the duplicated slot in display form (M-02)", err.Error())
	}
	if base.Channels[1].Data != nil {
		t.Errorf("MergeCHIRP: base mutated on refusal: %+v", base.Channels[1])
	}
	var duplicate *DuplicateSlotsError
	if !errors.As(err, &duplicate) {
		t.Fatalf("MergeCHIRP error = %T, want *DuplicateSlotsError", err)
	}
	if len(duplicate.Slots) != 1 || duplicate.Slots[0] != "M-02" {
		t.Errorf("DuplicateSlotsError.Slots = %v, want [M-02]", duplicate.Slots)
	}
}
