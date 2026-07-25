// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// TestConnectDemo_Lifecycle exercises the REAL ConnectDemo/Disconnect
// path (via internal/wiring.OpenFakeSessionFor — task-15 brief §2's "app:
// unit tests against fakeradio via internal/wiring"). Cheap: ConnectDemo
// only opens+probes (no full ReadAll).
//
// Model/Region assertions pin task 41 (M9a-5)'s migration off the
// Task-39 compatibility wrappers: ConnectionInfo.Model now comes from
// sess.Capabilities().Model rather than the wiring.Model alias (deleted
// by task 44, once unreferenced), and Region from a driver.RegionReporter
// type assertion rather than a direct *ft710.Session.Region() call —
// both must still report exactly what they did before the migration.
func TestConnectDemo_Lifecycle(t *testing.T) {
	a, _ := newTestApp(t)

	info, err := a.ConnectDemo()
	if err != nil {
		t.Fatalf("ConnectDemo: unexpected error: %v", err)
	}
	if !info.Demo {
		t.Error("ConnectDemo: ConnectionInfo.Demo = false, want true")
	}
	if info.CATID != "0800" {
		t.Errorf("ConnectDemo: CATID = %q, want %q", info.CATID, "0800")
	}
	if info.Model != wiring.DefaultModel {
		t.Errorf("ConnectDemo: Model = %q, want %q", info.Model, wiring.DefaultModel)
	}
	// The default fakeradio image (ImageUK) is HW-CONFIRMED 2026-07-13 to
	// carry no 5xx bank at all — deriveRegion's "no-60m" case.
	if info.Region != "no-60m" {
		t.Errorf("ConnectDemo: Region = %q, want %q", info.Region, "no-60m")
	}

	if _, err := a.ConnectDemo(); !errors.Is(err, ErrAlreadyConnected) {
		t.Errorf("ConnectDemo while connected: err = %v, want ErrAlreadyConnected", err)
	}

	if err := a.Disconnect(); err != nil {
		t.Fatalf("Disconnect: unexpected error: %v", err)
	}
	if err := a.Disconnect(); !errors.Is(err, ErrNotConnected) {
		t.Errorf("Disconnect while not connected: err = %v, want ErrNotConnected", err)
	}

	// Reconnecting after a clean disconnect must work.
	if _, err := a.ConnectDemo(); err != nil {
		t.Fatalf("ConnectDemo after Disconnect: unexpected error: %v", err)
	}
}

// TestDisconnect_KeepsWorkingCopy confirms (Fix 1, adjudicated HIGH,
// Codex M6 #1 — "Verify Go's Disconnect keeps working (it does —
// confirm, don't change)") that Disconnect never touches
// working/dirty/workingPath: only a.conn and a.currentPlan are cleared.
// The frontend-side half of Fix 1 (appState.disconnectConnection) relies
// entirely on this Go contract already holding.
func TestDisconnect_KeepsWorkingCopy(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.ConnectDemo(); err != nil {
		t.Fatalf("ConnectDemo: %v", err)
	}
	if _, err := a.ReadRadio(); err != nil {
		t.Fatalf("ReadRadio: %v", err)
	}
	// An edit made while connected.
	view, err := a.GetCodeplug()
	if err != nil {
		t.Fatalf("GetCodeplug: %v", err)
	}
	edited := view.Channels[0]
	if edited.Data == nil {
		t.Fatal("test setup: slot 0's Data is nil, want a populated channel to edit")
	}
	edited.Data.Tag = "DISCONNECT-TEST"
	if _, err := a.UpdateChannel(edited); err != nil {
		t.Fatalf("UpdateChannel: %v", err)
	}
	if !a.IsDirty() {
		t.Fatal("test setup: not dirty after an edit — the rest of this test would be vacuous")
	}

	if err := a.Disconnect(); err != nil {
		t.Fatalf("Disconnect: unexpected error: %v", err)
	}

	if !a.IsDirty() {
		t.Error("IsDirty() after Disconnect = false, want true — the working copy must survive a disconnect")
	}
	got, err := a.GetCodeplug()
	if err != nil {
		t.Fatalf("GetCodeplug after Disconnect: unexpected error: %v", err)
	}
	if got.Channels[0].Data == nil || got.Channels[0].Data.Tag != "DISCONNECT-TEST" {
		t.Errorf("GetCodeplug after Disconnect: slot 0 = %+v, want the edit still present", got.Channels[0])
	}
	// Save must still be possible (SaveFile refuses only on ErrNothingLoaded/
	// busy — neither applies here).
	if err := a.SaveFile(filepath.Join(t.TempDir(), "still-saveable.json")); err != nil {
		t.Errorf("SaveFile after Disconnect: unexpected error: %v (Save must still be possible)", err)
	}
}

// TestListPorts_NoError pins that ListPorts at least runs cleanly
// end-to-end (transport.Discover) — its content is host-dependent, so
// nothing about WHICH ports it finds is asserted.
func TestListPorts_NoError(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.ListPorts(); err != nil {
		t.Errorf("ListPorts: unexpected error: %v", err)
	}
}

// TestGetSupportedModels_ContainsDefaultModel pins the new (task 41,
// M9a-5) bound method: registry-driven (internal/wiring.SupportedModels),
// so it must at least contain wiring.DefaultModel, sorted (matching
// SupportedModels' own guarantee).
func TestGetSupportedModels_ContainsDefaultModel(t *testing.T) {
	a, _ := newTestApp(t)
	got := a.GetSupportedModels()
	if !reflect.DeepEqual(got, wiring.SupportedModels()) {
		t.Errorf("GetSupportedModels() = %v, want %v (wiring.SupportedModels())", got, wiring.SupportedModels())
	}
	found := false
	for _, m := range got {
		if m == wiring.DefaultModel {
			found = true
		}
	}
	if !found {
		t.Errorf("GetSupportedModels() = %v, want it to contain wiring.DefaultModel %q", got, wiring.DefaultModel)
	}
}

// TestReadRadio_NotConnected pins the connection-required guard shared
// by every radio-touching bound method.
func TestReadRadio_NotConnected(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.ReadRadio(); !errors.Is(err, ErrNotConnected) {
		t.Errorf("ReadRadio while not connected: err = %v, want ErrNotConnected", err)
	}
}

// TestDiffAgainstRadio_NotConnected pins the same guard for
// DiffAgainstRadio.
func TestDiffAgainstRadio_NotConnected(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.DiffAgainstRadio(); !errors.Is(err, ErrNotConnected) {
		t.Errorf("DiffAgainstRadio while not connected: err = %v, want ErrNotConnected", err)
	}
}

// TestPrepareSend_NotConnected pins the same guard for PrepareSend.
func TestPrepareSend_NotConnected(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.PrepareSend(); !errors.Is(err, ErrNotConnected) {
		t.Errorf("PrepareSend while not connected: err = %v, want ErrNotConnected", err)
	}
}

// TestConfirmSend_NoActivePlan pins ConfirmSend's synchronous
// "no plan" pre-check.
func TestConfirmSend_NoActivePlan(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.ConnectDemo(); err != nil {
		t.Fatalf("ConnectDemo: %v", err)
	}
	if err := a.ConfirmSend("anything", ""); !errors.Is(err, ErrNoActivePlan) {
		t.Errorf("ConfirmSend with no plan: err = %v, want ErrNoActivePlan", err)
	}
}

// TestCancelTransfer_NothingRunning pins CancelTransfer's guard when
// idle.
func TestCancelTransfer_NothingRunning(t *testing.T) {
	a, _ := newTestApp(t)
	if err := a.CancelTransfer(); !errors.Is(err, ErrNoTransferRunning) {
		t.Errorf("CancelTransfer while idle: err = %v, want ErrNoTransferRunning", err)
	}
}
