// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
)

// TestGetSettingsSpec_Offline pins GetSettingsSpec's static fallback (no
// session): Live false, and the FT-710 EX descriptor's exact shape —
// 5 menus, 21 groups, 296 items (mirrors
// core/driver/ft710/settings_test.go's own pinned counts), every ID/Label/
// Display non-empty.
func TestGetSettingsSpec_Offline(t *testing.T) {
	a, _ := newTestApp(t)

	got, err := a.GetSettingsSpec()
	if err != nil {
		t.Fatalf("GetSettingsSpec: unexpected error: %v", err)
	}
	if got.Live {
		t.Error("GetSettingsSpec offline: Live = true, want false")
	}
	if got.DescriptorVersion != "ft710-ex@1" {
		t.Errorf("DescriptorVersion = %q, want \"ft710-ex@1\"", got.DescriptorVersion)
	}
	if len(got.Menus) != 5 {
		t.Fatalf("menu count = %d, want 5", len(got.Menus))
	}

	var groups, items int
	for _, m := range got.Menus {
		if m.ID == "" || m.Label == "" {
			t.Errorf("menu %+v has an empty ID/Label", m)
		}
		groups += len(m.Groups)
		for _, g := range m.Groups {
			if g.ID == "" || g.Label == "" {
				t.Errorf("group %+v has an empty ID/Label", g)
			}
			items += len(g.Items)
			for _, it := range g.Items {
				if it.ID == "" || it.Label == "" || it.Display == "" {
					t.Errorf("item %+v has an empty field", it)
				}
			}
		}
	}
	if groups != 21 {
		t.Errorf("group count = %d, want 21", groups)
	}
	if items != 296 {
		t.Errorf("item count = %d, want 296", items)
	}
}

// TestGetSettingsSpec_ConnectedSim pins the connected branch: Live true,
// and the view equals descriptorToSpecView applied to the session's own
// SettingsDescriptor().
func TestGetSettingsSpec_ConnectedSim(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.ConnectDemo(""); err != nil {
		t.Fatalf("ConnectDemo: %v", err)
	}

	got, err := a.GetSettingsSpec()
	if err != nil {
		t.Fatalf("GetSettingsSpec: unexpected error: %v", err)
	}
	if !got.Live {
		t.Error("GetSettingsSpec while connected: Live = false, want true")
	}

	a.mu.Lock()
	reader, ok := a.conn.session.(driver.SettingsReader)
	a.mu.Unlock()
	if !ok {
		t.Fatal("a.conn.session does not implement driver.SettingsReader")
	}
	want := descriptorToSpecView(reader.SettingsDescriptor(), true)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetSettingsSpec while connected != descriptorToSpecView(session descriptor, true)")
	}
}

// TestGetSettings_NothingLoaded pins the typed refusal when no working
// copy exists at all.
func TestGetSettings_NothingLoaded(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.GetSettings(); !errors.Is(err, ErrNothingLoaded) {
		t.Errorf("GetSettings (nothing loaded): err = %v, want ErrNothingLoaded", err)
	}
}

// TestGetSettings_NoSnapshot: a working copy from ReadRadio alone (never
// had its settings read) reports HasSnapshot false — an entirely ordinary
// state (settings acquisition is opt-in), not an error.
func TestGetSettings_NoSnapshot(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.ConnectDemo(""); err != nil {
		t.Fatalf("ConnectDemo: %v", err)
	}
	if _, err := a.ReadRadio(); err != nil {
		t.Fatalf("ReadRadio: %v", err)
	}

	got, err := a.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: unexpected error: %v", err)
	}
	if got.HasSnapshot {
		t.Errorf("GetSettings after ReadRadio alone: HasSnapshot = true, want false (ReadAll never touches Menus)")
	}
	if len(got.Entries) != 0 {
		t.Errorf("Entries = %+v, want empty", got.Entries)
	}
}

// TestGetSettings_LegacyOnly: a Menus snapshot carrying only a migrated
// Legacy payload (no natively-read Entries) reports HasSnapshot true,
// HasLegacy true, and zero Entries.
func TestGetSettings_LegacyOnly(t *testing.T) {
	a, _ := newTestApp(t)
	a.mu.Lock()
	a.working = &codeplug.Codeplug{
		Schema: codeplug.CurrentSchema,
		Menus:  &codeplug.MenuSnapshot{Legacy: json.RawMessage(`{"v1":true}`)},
	}
	a.mu.Unlock()

	got, err := a.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: unexpected error: %v", err)
	}
	if !got.HasSnapshot {
		t.Error("GetSettings (legacy-only): HasSnapshot = false, want true")
	}
	if !got.HasLegacy {
		t.Error("GetSettings (legacy-only): HasLegacy = false, want true")
	}
	if len(got.Entries) != 0 {
		t.Errorf("Entries = %+v, want empty", got.Entries)
	}
}

// TestReadSettingsRadio_NotConnected pins the typed refusal, with no
// partial state change, when there is no connection.
func TestReadSettingsRadio_NotConnected(t *testing.T) {
	a, _ := newTestApp(t)
	a.mu.Lock()
	a.working = &codeplug.Codeplug{Schema: codeplug.CurrentSchema}
	a.mu.Unlock()

	_, err := a.ReadSettingsRadio()
	if !errors.Is(err, ErrNotConnected) {
		t.Errorf("ReadSettingsRadio (not connected): err = %v, want ErrNotConnected", err)
	}

	a.mu.Lock()
	menusNil := a.working.Menus == nil
	dirty := a.dirty
	a.mu.Unlock()
	if !menusNil {
		t.Error("ReadSettingsRadio (refused): Menus changed, want untouched")
	}
	if dirty {
		t.Error("ReadSettingsRadio (refused): dirty = true, want false (no partial state change)")
	}
}

// TestReadSettingsRadio_NothingLoaded pins the typed refusal, with no
// partial state change, when there is a connection but no working copy.
func TestReadSettingsRadio_NothingLoaded(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.ConnectDemo(""); err != nil {
		t.Fatalf("ConnectDemo: %v", err)
	}

	_, err := a.ReadSettingsRadio()
	if !errors.Is(err, ErrNothingLoaded) {
		t.Errorf("ReadSettingsRadio (nothing loaded): err = %v, want ErrNothingLoaded", err)
	}
	if a.IsDirty() {
		t.Error("ReadSettingsRadio (refused): dirty = true, want false")
	}
}

// TestReadSettingsRadio_FullCycle_Sim drives ConnectDemo -> ReadRadio ->
// ReadSettingsRadio against fakeradio's default EX state (every address
// answers Known): the returned view is Complete, GetSettings agrees with
// it, IsDirty() is true, the recorder saw >=1 transfer:progress event with
// TargetKind "setting", a 6-digit TargetID, and a non-empty TargetDisplay,
// and exactly one transfer:done{Kind:"settings",Outcome:"ok"}.
func TestReadSettingsRadio_FullCycle_Sim(t *testing.T) {
	a, rec := newTestApp(t)
	if _, err := a.ConnectDemo(""); err != nil {
		t.Fatalf("ConnectDemo: %v", err)
	}
	if _, err := a.ReadRadio(); err != nil {
		t.Fatalf("ReadRadio: %v", err)
	}

	view, err := a.ReadSettingsRadio()
	if err != nil {
		t.Fatalf("ReadSettingsRadio: unexpected error: %v", err)
	}
	if !view.HasSnapshot {
		t.Error("ReadSettingsRadio: HasSnapshot = false, want true")
	}
	if !view.Complete {
		t.Error("ReadSettingsRadio: Complete = false, want true (fakeradio answers every EX address Known by default)")
	}
	if len(view.Entries) != 296 {
		t.Errorf("len(view.Entries) = %d, want 296", len(view.Entries))
	}

	got, err := a.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if !reflect.DeepEqual(got, view) {
		t.Errorf("GetSettings() disagrees with ReadSettingsRadio's own return")
	}

	if !a.IsDirty() {
		t.Error("IsDirty() after ReadSettingsRadio = false, want true")
	}

	var sawSettingProgress bool
	for _, e := range rec.named("transfer:progress") {
		ev, ok := e.data.(ProgressEvent)
		if !ok {
			t.Fatalf("transfer:progress payload type = %T, want ProgressEvent", e.data)
		}
		if ev.TargetKind != "setting" {
			continue
		}
		sawSettingProgress = true
		if len(ev.TargetID) != 6 {
			t.Errorf("transfer:progress TargetID = %q, want 6 digits", ev.TargetID)
		}
		if ev.TargetDisplay == "" {
			t.Error("transfer:progress TargetDisplay is empty, want non-empty (looked up from the descriptor)")
		}
		if ev.Phase != "read-settings" {
			t.Errorf("transfer:progress Phase = %q, want \"read-settings\"", ev.Phase)
		}
	}
	if !sawSettingProgress {
		t.Error("no transfer:progress event with TargetKind \"setting\" was recorded")
	}

	var settingsDoneCount int
	for _, e := range rec.named("transfer:done") {
		ev, ok := e.data.(TransferDoneEvent)
		if ok && ev.Kind == "settings" {
			settingsDoneCount++
			if ev.Outcome != "ok" {
				t.Errorf("transfer:done{Kind:settings} Outcome = %q, want \"ok\"", ev.Outcome)
			}
			if ev.Report != nil {
				t.Errorf("transfer:done{Kind:settings} Report = %+v, want nil", ev.Report)
			}
		}
	}
	if settingsDoneCount != 1 {
		t.Errorf("transfer:done{Kind:settings} count = %d, want exactly 1", settingsDoneCount)
	}
}

// TestChannelProgress_TargetFieldsAndSlotCompat is the compatibility pin
// (task 35): a channel read (ReadRadio) still emits Slot exactly as
// before, AND the three new fields — TargetKind "channel", a non-empty
// TargetID whose codeplug.DisplaySlot equals both TargetDisplay and Slot.
func TestChannelProgress_TargetFieldsAndSlotCompat(t *testing.T) {
	a, rec := newTestApp(t)
	if _, err := a.ConnectDemo(""); err != nil {
		t.Fatalf("ConnectDemo: %v", err)
	}
	if _, err := a.ReadRadio(); err != nil {
		t.Fatalf("ReadRadio: %v", err)
	}

	progressed := rec.named("transfer:progress")
	if len(progressed) == 0 {
		t.Fatal("no transfer:progress events recorded")
	}
	for _, e := range progressed {
		ev, ok := e.data.(ProgressEvent)
		if !ok {
			t.Fatalf("transfer:progress payload type = %T, want ProgressEvent", e.data)
		}
		if ev.TargetKind != "channel" {
			t.Errorf("channel progress TargetKind = %q, want \"channel\"", ev.TargetKind)
		}
		if ev.Slot == "" {
			t.Error("Slot is empty, want the pre-existing DisplaySlot form (JS compatibility)")
		}
		if ev.TargetID == "" {
			t.Error("TargetID is empty, want the wire-form slot")
		}
		if got := codeplug.DisplaySlot(ev.TargetID); got != ev.TargetDisplay {
			t.Errorf("TargetDisplay = %q, want codeplug.DisplaySlot(TargetID=%q) = %q", ev.TargetDisplay, ev.TargetID, got)
		}
		if ev.TargetDisplay != ev.Slot {
			t.Errorf("TargetDisplay = %q, want it to equal Slot %q", ev.TargetDisplay, ev.Slot)
		}
	}
}

// TestReadSettingsRadio_PreservesLegacyAndUnsupported is the merge-never-
// assign pin, end to end (task 35): a working copy carrying a Legacy
// payload and an unrecognised-ID entry survives ReadSettingsRadio, then a
// SaveFile -> LoadFile round trip.
func TestReadSettingsRadio_PreservesLegacyAndUnsupported(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.ConnectDemo(""); err != nil {
		t.Fatalf("ConnectDemo: %v", err)
	}
	if _, err := a.ReadRadio(); err != nil {
		t.Fatalf("ReadRadio: %v", err)
	}

	// not a Table 2 member — see cat.Dialect.KnownEXAddress.
	const unrecognisedID = "999999"
	a.mu.Lock()
	a.working.Menus = &codeplug.MenuSnapshot{
		Descriptor: "some-older-build@1",
		Complete:   false,
		Entries:    []codeplug.MenuEntry{{ID: unrecognisedID, Value: "leftover", State: codeplug.MenuUnsupported}},
		Legacy:     json.RawMessage(`{"v1setting":42}`),
	}
	a.mu.Unlock()

	if _, err := a.ReadSettingsRadio(); err != nil {
		t.Fatalf("ReadSettingsRadio: unexpected error: %v", err)
	}

	checkPreserved := func(t *testing.T, snap *codeplug.MenuSnapshot) {
		t.Helper()
		if snap == nil {
			t.Fatal("Menus is nil, want a populated snapshot")
		}
		var found bool
		for _, e := range snap.Entries {
			if e.ID == unrecognisedID {
				found = true
				if e.State != codeplug.MenuUnsupported || e.Value != "leftover" {
					t.Errorf("carried-forward entry = %+v, want State=unsupported Value=leftover", e)
				}
			}
		}
		if !found {
			t.Error("unrecognised-ID entry lost, want it carried forward as unsupported")
		}
		if len(snap.Legacy) == 0 {
			t.Error("Legacy lost, want it carried forward")
		}
	}

	a.mu.Lock()
	checkPreserved(t, a.working.Menus)
	a.mu.Unlock()

	path := filepath.Join(t.TempDir(), "codeplug.json")
	if err := a.SaveFile(path); err != nil {
		t.Fatalf("SaveFile: %v", err)
	}
	if _, err := a.loadFilePath(path); err != nil {
		t.Fatalf("loadFilePath: %v", err)
	}

	a.mu.Lock()
	checkPreserved(t, a.working.Menus)
	a.mu.Unlock()
}

// TestSaveFile_PersistsSettings: after ReadSettingsRadio, SaveFile writes
// the settings snapshot to disk (checked by loading the file directly with
// codeplug.Load, independent of the App).
func TestSaveFile_PersistsSettings(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.ConnectDemo(""); err != nil {
		t.Fatalf("ConnectDemo: %v", err)
	}
	if _, err := a.ReadRadio(); err != nil {
		t.Fatalf("ReadRadio: %v", err)
	}
	if _, err := a.ReadSettingsRadio(); err != nil {
		t.Fatalf("ReadSettingsRadio: %v", err)
	}

	path := filepath.Join(t.TempDir(), "codeplug.json")
	if err := a.SaveFile(path); err != nil {
		t.Fatalf("SaveFile: %v", err)
	}

	onDisk, err := codeplug.Load(path)
	if err != nil {
		t.Fatalf("codeplug.Load: %v", err)
	}
	if onDisk.Menus == nil || len(onDisk.Menus.Entries) != 296 {
		t.Fatalf("on-disk Menus = %+v, want 296 persisted entries", onDisk.Menus)
	}
}

// TestLoadFile_SettingsSurvive: a file written independently of the App
// (codeplug.Save) with a settings snapshot loads back through loadFilePath
// and is visible via GetSettings.
func TestLoadFile_SettingsSurvive(t *testing.T) {
	a, _ := newTestApp(t)
	path := filepath.Join(t.TempDir(), "codeplug.json")
	cp := &codeplug.Codeplug{
		Schema: codeplug.CurrentSchema,
		Radio:  codeplug.RadioInfo{Model: "FT-710", CATID: "0800"},
		Menus: &codeplug.MenuSnapshot{
			Descriptor: "ft710-ex@1",
			Complete:   true,
			Entries:    []codeplug.MenuEntry{{ID: "010101", Value: "42", State: codeplug.MenuKnown}},
		},
	}
	if err := codeplug.Save(path, cp); err != nil {
		t.Fatalf("codeplug.Save: %v", err)
	}

	if _, err := a.loadFilePath(path); err != nil {
		t.Fatalf("loadFilePath: %v", err)
	}
	got, err := a.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if !got.HasSnapshot || !got.Complete {
		t.Errorf("GetSettings after LoadFile: HasSnapshot=%v Complete=%v, want both true", got.HasSnapshot, got.Complete)
	}
	if len(got.Entries) != 1 || got.Entries[0] != (SettingEntryView{ID: "010101", Value: "42", State: "known"}) {
		t.Errorf("GetSettings after LoadFile: Entries = %+v, unexpected", got.Entries)
	}
}

// TestDisconnect_SettingsSurviveOffline: settings content survives
// Disconnect (GetSettings unchanged), and GetSettingsSpec falls back to
// Live false once offline.
func TestDisconnect_SettingsSurviveOffline(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.ConnectDemo(""); err != nil {
		t.Fatalf("ConnectDemo: %v", err)
	}
	if _, err := a.ReadRadio(); err != nil {
		t.Fatalf("ReadRadio: %v", err)
	}
	before, err := a.ReadSettingsRadio()
	if err != nil {
		t.Fatalf("ReadSettingsRadio: %v", err)
	}

	if err := a.Disconnect(); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}

	after, err := a.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings after Disconnect: %v", err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Error("GetSettings after Disconnect disagrees with ReadSettingsRadio's own return, want unchanged")
	}

	spec, err := a.GetSettingsSpec()
	if err != nil {
		t.Fatalf("GetSettingsSpec after Disconnect: %v", err)
	}
	if spec.Live {
		t.Error("GetSettingsSpec after Disconnect: Live = true, want false")
	}
}

// TestCurrentSettingsDescriptor_FollowsResolvedModel is the settings
// cluster's threading pin, and the closure of the drift M9c-5 E4 was
// written to close: this fallback used to name wiring.DefaultModel
// literally, so a model with no settings surface of its own silently
// received the FT-710's entire EX menu tree. It now follows currentModel,
// and a model wiring.StaticSettingsDescriptor cannot resolve yields the
// ZERO descriptor — an empty settings tree — never another radio's.
//
// The FT-710 leg is pinned first and asserted non-empty, so the empty
// result below is a genuine contrast rather than a vacuous pass.
func TestCurrentSettingsDescriptor_FollowsResolvedModel(t *testing.T) {
	ft710Descriptor, live := currentSettingsDescriptor(nil, nil)
	if live {
		t.Error("currentSettingsDescriptor(nil, nil): live = true, want false (no session)")
	}
	if len(ft710Descriptor.Menus) == 0 {
		t.Fatal("test setup: the default model's static descriptor has no menus — the contrast below would be vacuous")
	}

	recogniseTestModel(t)
	working := &codeplug.Codeplug{Radio: codeplug.RadioInfo{Model: testModel}}
	got, live := currentSettingsDescriptor(nil, working)
	if live {
		t.Error("currentSettingsDescriptor(nil, testModel working): live = true, want false")
	}
	if len(got.Menus) != 0 {
		t.Errorf("currentSettingsDescriptor(nil, testModel working): %d menus, want 0 — another radio's settings tree must never be served for a model that has none", len(got.Menus))
	}
}
