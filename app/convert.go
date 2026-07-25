// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"github.com/gm5dna/open-rig-programmer/core/clone"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/csvio"
	"github.com/gm5dna/open-rig-programmer/core/driver"
)

// This file holds pure, side-effect-free conversion helpers between
// core/* return values and this package's View/Event types (types.go).
// None of these touch App state or do I/O — kept separate so they are
// trivially unit-testable (convert_test.go) independent of any session,
// dialog, or event plumbing.

// issuesToView converts codeplug.Validate's result to the display shape
// Validate/UpdateChannel(s)/ImportCSV/ImportCHIRP all return.
func issuesToView(issues []codeplug.Issue) []IssueView {
	out := make([]IssueView, len(issues))
	for i, is := range issues {
		out[i] = IssueView{Slot: is.Slot, Field: string(is.Field), Severity: string(is.Severity), Msg: is.Msg}
	}
	return out
}

// diffEntryToView converts one codeplug.DiffEntry, adding the display-
// form slot (codeplug.DisplaySlot) the frontend has no other way to
// compute.
func diffEntryToView(e codeplug.DiffEntry) DiffEntryView {
	return DiffEntryView{
		Slot:        e.Slot,
		SlotDisplay: codeplug.DisplaySlot(e.Slot),
		Bank:        string(e.Bank),
		Kind:        string(e.Kind),
		Before:      e.Before,
		After:       e.After,
		Blocked:     e.Blocked,
		BlockReason: e.BlockReason,
	}
}

// buildDiffSummary groups result's entries by kind (Added/Modified/
// Erased — Unchanged entries are counted but never listed, matching the
// CLI's writeDiffReport/writePlanSummary grouping), shared by
// DiffAgainstRadio and PrepareSend.
func buildDiffSummary(result codeplug.DiffResult) DiffSummaryView {
	summary := DiffSummaryView{
		Counts: DiffCounts{Added: result.Added, Modified: result.Modified, Erased: result.Erased, Blocked: result.Blocked, Unchanged: result.Unchanged},
	}
	for _, e := range result.Entries {
		switch e.Kind {
		case codeplug.DiffAdded:
			summary.Added = append(summary.Added, diffEntryToView(e))
		case codeplug.DiffModified:
			summary.Modified = append(summary.Modified, diffEntryToView(e))
		case codeplug.DiffErased:
			summary.Erased = append(summary.Erased, diffEntryToView(e))
		}
	}
	return summary
}

// reportToView converts a clone.Report to its display shape, attaching
// snapshotPath (which Report itself does not carry — see ReportView's
// doc comment).
func reportToView(report *clone.Report, snapshotPath string) ReportView {
	slots := make([]SlotResultView, len(report.Slots))
	for i, s := range report.Slots {
		slots[i] = SlotResultView{
			Slot:        s.Slot,
			SlotDisplay: codeplug.DisplaySlot(s.Slot),
			Action:      s.Action,
			VerifyOK:    s.VerifyOK,
			Detail:      s.Detail,
		}
	}
	return ReportView{
		FirmwareConfirmed: report.FirmwareConfirmed,
		Written:           report.Written,
		Verified:          report.Verified,
		SkippedBlocked:    report.SkippedBlocked,
		Unchanged:         report.Unchanged,
		Slots:             slots,
		Aborted:           report.Aborted,
		AbortReason:       report.AbortReason,
		JournalPath:       report.JournalPath,
		SnapshotPath:      snapshotPath,
	}
}

// lossEntriesToView converts a csvio.LossReport to its display shape.
func lossEntriesToView(report csvio.LossReport) []LossEntryView {
	out := make([]LossEntryView, len(report.Entries))
	for i, e := range report.Entries {
		out[i] = LossEntryView{Line: e.Line, Column: e.Column, Value: e.Value, Action: string(e.Action), Detail: e.Detail, Blocking: e.Blocking}
	}
	return out
}

// copyChannelData returns a defensive copy of d, or nil if d is nil.
// ChannelData holds only value types, so one dereference-and-copy is
// already a full deep copy — the same technique core/clone/plan.go's
// unexported copyChannelData uses, restated here since this package
// cannot reach into core/clone's unexported helpers.
func copyChannelData(d *codeplug.ChannelData) *codeplug.ChannelData {
	if d == nil {
		return nil
	}
	cp := *d
	return &cp
}

// copyChannels returns an independently-allocated deep copy of
// channels: a fresh slice, and a fresh *ChannelData per element.
func copyChannels(channels []codeplug.Channel) []codeplug.Channel {
	out := make([]codeplug.Channel, len(channels))
	for i, ch := range channels {
		out[i] = codeplug.Channel{Slot: ch.Slot, Data: copyChannelData(ch.Data)}
	}
	return out
}

// descriptorToSpecView converts d to GetSettingsSpec's display shape,
// mapping menus->groups->items straight across with no protocol fact
// invented here (see GetSettingsSpec's doc comment) — the settings-tree
// counterpart of GetUISpec's own capability mapping (uispec.go), just
// structural rather than capability-flavoured.
func descriptorToSpecView(d driver.SettingsDescriptor, live bool) SettingsSpecView {
	menus := make([]SettingMenuView, len(d.Menus))
	for i, m := range d.Menus {
		groups := make([]SettingGroupView, len(m.Groups))
		for j, g := range m.Groups {
			items := make([]SettingItemView, len(g.Items))
			for k, it := range g.Items {
				items[k] = SettingItemView{ID: it.ID, Label: it.Label, Display: it.Display}
			}
			groups[j] = SettingGroupView{ID: g.ID, Label: g.Label, Items: items}
		}
		menus[i] = SettingMenuView{ID: m.ID, Label: m.Label, Groups: groups}
	}
	return SettingsSpecView{Live: live, DescriptorVersion: d.Version, Menus: menus}
}

// menuSnapshotToView converts snap (possibly nil) to GetSettings/
// ReadSettingsRadio's display shape. HasSnapshot is false only when snap
// itself is nil — the working copy has never had its settings read at all
// (settings acquisition is opt-in, task 33); a non-nil snapshot with zero
// Entries (e.g. Legacy-only, migrated from a v1 file whose settings were
// never natively re-read) still reports HasSnapshot true. HasLegacy
// mirrors len(snap.Legacy) > 0 — the raw legacy payload itself is NEVER
// included in the view (task-35 brief's hard constraint: the raw legacy
// blob is never shipped to JS). Entries are mapped 1:1, in stored order;
// State is already one of "known"/"unavailable"/"unsupported"
// (codeplug.MenuEntryState's own string values equal SettingEntryView.
// State's documented set verbatim, so no translation table is needed).
func menuSnapshotToView(snap *codeplug.MenuSnapshot) SettingsView {
	if snap == nil {
		return SettingsView{}
	}
	entries := make([]SettingEntryView, len(snap.Entries))
	for i, e := range snap.Entries {
		entries[i] = SettingEntryView{ID: e.ID, Value: e.Value, State: string(e.State)}
	}
	return SettingsView{
		HasSnapshot: true,
		Descriptor:  snap.Descriptor,
		Complete:    snap.Complete,
		HasLegacy:   len(snap.Legacy) > 0,
		Entries:     entries,
	}
}

// deepCopyCodeplug returns an independently-allocated deep copy of cp
// (nil in, nil out): a fresh Channels slice/ChannelData per element and a
// fresh Menus snapshot via MenuSnapshot.Clone (nil-safe: a fresh Entries
// slice and Legacy bytes). RadioInfo holds no pointers, so copying the
// struct by value already isolates it. Used so ReadRadio's baseline and
// working copies (task-15 brief §2) can never alias each other, and so
// every View returned to the frontend is independent of the App's own
// in-memory state.
func deepCopyCodeplug(cp *codeplug.Codeplug) *codeplug.Codeplug {
	if cp == nil {
		return nil
	}
	out := *cp
	out.Channels = copyChannels(cp.Channels)
	out.Menus = cp.Menus.Clone()
	return &out
}
