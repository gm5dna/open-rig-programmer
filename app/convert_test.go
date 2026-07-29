// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/clone"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/csvio"
	"github.com/gm5dna/open-rig-programmer/core/driver"
)

func TestIssuesToView(t *testing.T) {
	issues := []codeplug.Issue{
		{Slot: "001", Field: "frequency", Severity: codeplug.SeverityError, Msg: "out of range"},
		{Severity: codeplug.SeverityWarning, Msg: "no slot/field"},
	}
	got := issuesToView(issues)
	if len(got) != 2 {
		t.Fatalf("issuesToView: len = %d, want 2", len(got))
	}
	if got[0].Slot != "001" || got[0].Field != "frequency" || got[0].Severity != "error" || got[0].Msg != "out of range" {
		t.Errorf("issuesToView[0] = %+v, unexpected", got[0])
	}
	if got[1].Slot != "" || got[1].Field != "" || got[1].Severity != "warning" {
		t.Errorf("issuesToView[1] = %+v, unexpected", got[1])
	}
}

func TestBuildDiffSummary_GroupsByKindAndCounts(t *testing.T) {
	result := codeplug.DiffResult{
		Entries: []codeplug.DiffEntry{
			{Slot: "001", Bank: "MEM", Kind: codeplug.DiffAdded, After: &codeplug.ChannelData{FreqHz: 7_000_000, TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false}}},
			{Slot: "002", Bank: "MEM", Kind: codeplug.DiffModified, Before: &codeplug.ChannelData{FreqHz: 1, TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false}}, After: &codeplug.ChannelData{FreqHz: 2}},
			{Slot: "501", Bank: "60M", Kind: codeplug.DiffErased, Blocked: true, BlockReason: "erase unsupported"},
			{Slot: "003", Bank: "MEM", Kind: codeplug.DiffUnchanged},
		},
		Added: 1, Modified: 1, Erased: 1, Unchanged: 1, Blocked: 1,
	}
	got := buildDiffSummary(result)
	if len(got.Added) != 1 || got.Added[0].SlotDisplay != "M-01" {
		t.Errorf("Added = %+v", got.Added)
	}
	if len(got.Modified) != 1 || got.Modified[0].SlotDisplay != "M-02" {
		t.Errorf("Modified = %+v", got.Modified)
	}
	if len(got.Erased) != 1 || !got.Erased[0].Blocked || got.Erased[0].BlockReason != "erase unsupported" || got.Erased[0].SlotDisplay != "5-01" {
		t.Errorf("Erased = %+v", got.Erased)
	}
	if got.Counts != (DiffCounts{Added: 1, Modified: 1, Erased: 1, Blocked: 1, Unchanged: 1}) {
		t.Errorf("Counts = %+v", got.Counts)
	}
}

func TestReportToView(t *testing.T) {
	report := &clone.Report{
		FirmwareConfirmed: "V01-10",
		Written:           2,
		Verified:          2,
		SkippedBlocked:    1,
		Unchanged:         3,
		Slots: []clone.SlotResult{
			{Slot: "001", Action: "write", VerifyOK: true},
			{Slot: "002", Action: "skipped-blocked", Detail: "unsupported"},
		},
		Aborted:     false,
		AbortReason: "",
		JournalPath: "/tmp/journal.jsonl",
	}
	got := reportToView(report, "/tmp/snap.json")
	if got.FirmwareConfirmed != "V01-10" || got.Written != 2 || got.Verified != 2 || got.SkippedBlocked != 1 || got.Unchanged != 3 {
		t.Errorf("reportToView counts wrong: %+v", got)
	}
	if got.SnapshotPath != "/tmp/snap.json" || got.JournalPath != "/tmp/journal.jsonl" {
		t.Errorf("reportToView paths wrong: %+v", got)
	}
	if len(got.Slots) != 2 || got.Slots[0].SlotDisplay != "M-01" || got.Slots[1].SlotDisplay != "M-02" {
		t.Errorf("reportToView slots wrong: %+v", got.Slots)
	}
}

func TestLossEntriesToView(t *testing.T) {
	report := csvio.LossReport{Entries: []csvio.LossEntry{
		{Line: 3, Column: "tone", Value: "88.5", Action: csvio.ActionApproximated, Detail: "rounded", Blocking: false},
		{Line: 4, Column: "location", Value: "999", Action: csvio.ActionUnsupported, Detail: "out of range", Blocking: true},
	}}
	got := lossEntriesToView(report)
	if len(got) != 2 {
		t.Fatalf("lossEntriesToView: len = %d, want 2", len(got))
	}
	if got[0].Line != 3 || got[0].Action != "approximated" || got[1].Blocking != true {
		t.Errorf("lossEntriesToView = %+v", got)
	}
}

func TestCopyChannels_IndependentDeepCopy(t *testing.T) {
	original := []codeplug.Channel{{Slot: "001", Data: &codeplug.ChannelData{FreqHz: 7_000_000, Tag: "MYCALL", TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false}}}}
	copied := copyChannels(original)
	copied[0].Data.Tag = "CHANGED"
	if original[0].Data.Tag != "MYCALL" {
		t.Errorf("copyChannels: mutating the copy changed the original: %+v", original[0])
	}
}

func TestDeepCopyCodeplug_IndependentOfSource(t *testing.T) {
	src := &codeplug.Codeplug{
		Schema:    codeplug.CurrentSchema,
		Generator: "test",
		Radio:     codeplug.RadioInfo{Model: "FT-710", CATID: "0800"},
		Channels:  []codeplug.Channel{{Slot: "001", Data: &codeplug.ChannelData{Tag: "MYCALL", TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false}}}},
	}
	dup := deepCopyCodeplug(src)
	dup.Channels[0].Data.Tag = "CHANGED"
	dup.Radio.Model = "OTHER"
	if src.Channels[0].Data.Tag != "MYCALL" || src.Radio.Model != "FT-710" {
		t.Errorf("deepCopyCodeplug: mutating dup changed src: %+v", src)
	}
}

// TestDeepCopyCodeplug_MenuSnapshotIndependence pins the schema-2
// regression: deepCopyCodeplug must clone the MenuSnapshot deeply, so
// mutating the copy's Entries or Legacy never reaches the App's working
// codeplug. A byte-copy of the old json.RawMessage field silently stopped
// covering Menus once it became a *MenuSnapshot — this catches that.
func TestDeepCopyCodeplug_MenuSnapshotIndependence(t *testing.T) {
	src := &codeplug.Codeplug{
		Schema:    codeplug.CurrentSchema,
		Generator: "test",
		Radio:     codeplug.RadioInfo{Model: "FT-710", CATID: "0800"},
		Channels:  []codeplug.Channel{{Slot: "001", Data: &codeplug.ChannelData{Tag: "MYCALL", TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false}}}},
		Menus: &codeplug.MenuSnapshot{
			Descriptor: "ft710-ex@1",
			Entries:    []codeplug.MenuEntry{{ID: "000101", Value: "3", State: codeplug.MenuKnown}},
			Legacy:     []byte(`{"leg":1}`),
		},
	}
	dup := deepCopyCodeplug(src)
	if dup.Menus == nil || dup.Menus == src.Menus {
		t.Fatalf("deepCopyCodeplug: Menus not independently allocated (dup=%p src=%p)", dup.Menus, src.Menus)
	}
	dup.Menus.Entries[0].Value = "CHANGED"
	dup.Menus.Legacy[0] = 'X'
	if src.Menus.Entries[0].Value != "3" {
		t.Errorf("deepCopyCodeplug: mutating dup.Menus.Entries changed src: %q", src.Menus.Entries[0].Value)
	}
	if src.Menus.Legacy[0] != '{' {
		t.Errorf("deepCopyCodeplug: mutating dup.Menus.Legacy changed src: %q", src.Menus.Legacy[0])
	}
}

// TestDeepCopyCodeplug_NilMenus confirms the nil-Menus path stays nil.
func TestDeepCopyCodeplug_NilMenus(t *testing.T) {
	src := &codeplug.Codeplug{Schema: codeplug.CurrentSchema, Radio: codeplug.RadioInfo{Model: "FT-710"}}
	if dup := deepCopyCodeplug(src); dup.Menus != nil {
		t.Errorf("deepCopyCodeplug: nil Menus copied to %+v, want nil", dup.Menus)
	}
}

// TestMenuSnapshotToView_Table pins menuSnapshotToView's shape (task 35):
// nil in -> the zero SettingsView (HasSnapshot false); a Legacy-only
// snapshot -> HasSnapshot true, HasLegacy true, zero Entries; and the
// state-string mapping ("known"/"unavailable"/"unsupported") for a mixed
// snapshot, in stored order.
func TestMenuSnapshotToView_Table(t *testing.T) {
	tests := []struct {
		name string
		snap *codeplug.MenuSnapshot
		want SettingsView
	}{
		{
			name: "nil snapshot",
			snap: nil,
			want: SettingsView{},
		},
		{
			name: "legacy-only, zero entries",
			snap: &codeplug.MenuSnapshot{Legacy: []byte(`{"x":1}`)},
			want: SettingsView{HasSnapshot: true, HasLegacy: true, Entries: []SettingEntryView{}},
		},
		{
			name: "mixed states, stored order",
			snap: &codeplug.MenuSnapshot{
				Descriptor: "ft710-ex@1",
				Complete:   false,
				Entries: []codeplug.MenuEntry{
					{ID: "000101", Value: "3", State: codeplug.MenuKnown},
					{ID: "000202", State: codeplug.MenuUnavailable},
					{ID: "000303", Value: "7", State: codeplug.MenuUnsupported},
				},
			},
			want: SettingsView{
				HasSnapshot: true,
				Descriptor:  "ft710-ex@1",
				Complete:    false,
				HasLegacy:   false,
				Entries: []SettingEntryView{
					{ID: "000101", Value: "3", State: "known"},
					{ID: "000202", State: "unavailable"},
					{ID: "000303", Value: "7", State: "unsupported"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := menuSnapshotToView(tt.snap)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("menuSnapshotToView(%+v) = %+v, want %+v", tt.snap, got, tt.want)
			}
		})
	}
}

// TestDescriptorToSpecView_Table pins descriptorToSpecView's shape (task
// 35): an empty descriptor maps to an empty (non-nil) Menus slice, and a
// small hand-built two-level tree maps straight across with Live/
// DescriptorVersion carried through.
func TestDescriptorToSpecView_Table(t *testing.T) {
	tests := []struct {
		name string
		d    driver.SettingsDescriptor
		live bool
		want SettingsSpecView
	}{
		{
			name: "empty descriptor",
			d:    driver.SettingsDescriptor{Version: "v0"},
			live: false,
			want: SettingsSpecView{Live: false, DescriptorVersion: "v0", Menus: []SettingMenuView{}},
		},
		{
			name: "one menu, one group, two items, live",
			d: driver.SettingsDescriptor{
				Version: "ft710-ex@1",
				Menus: []driver.SettingMenu{
					{ID: "01", Label: "RADIO SETTING", Groups: []driver.SettingGroup{
						{ID: "0101", Label: "MODE SSB", Items: []driver.SettingItem{
							{ID: "010101", Label: "AF TREBLE GAIN", Display: "01-01-01"},
							{ID: "010102", Label: "AF BASS GAIN", Display: "01-01-02"},
						}},
					}},
				},
			},
			live: true,
			want: SettingsSpecView{
				Live:              true,
				DescriptorVersion: "ft710-ex@1",
				Menus: []SettingMenuView{
					{ID: "01", Label: "RADIO SETTING", Groups: []SettingGroupView{
						{ID: "0101", Label: "MODE SSB", Items: []SettingItemView{
							{ID: "010101", Label: "AF TREBLE GAIN", Display: "01-01-01"},
							{ID: "010102", Label: "AF BASS GAIN", Display: "01-01-02"},
						}},
					}},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := descriptorToSpecView(tt.d, tt.live)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("descriptorToSpecView(%+v, %v) = %+v, want %+v", tt.d, tt.live, got, tt.want)
			}
		})
	}
}
