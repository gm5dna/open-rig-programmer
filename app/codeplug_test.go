// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
)

// TestReadRadio_SetsBaselineWorkingAndEvents drives ReadRadio against a
// real (default ImageUK) fakeradio session via the production
// ConnectDemo/internal/wiring path, pinning task-15 brief §5's "ReadRadio
// sets baseline+working+events".
func TestReadRadio_SetsBaselineWorkingAndEvents(t *testing.T) {
	a, rec := newTestApp(t)
	if _, err := a.ConnectDemo(); err != nil {
		t.Fatalf("ConnectDemo: %v", err)
	}

	view, err := a.ReadRadio()
	if err != nil {
		t.Fatalf("ReadRadio: unexpected error: %v", err)
	}
	if view.Radio.CATID != "0800" {
		t.Errorf("ReadRadio: Radio.CATID = %q, want %q", view.Radio.CATID, "0800")
	}
	if len(view.Channels) == 0 {
		t.Fatal("ReadRadio: zero channels — sanity check failed")
	}
	if view.Dirty || view.WorkingPath != "" || view.BaselineStale {
		t.Errorf("ReadRadio: view = %+v, want Dirty=false WorkingPath=\"\" BaselineStale=false", view)
	}
	if a.IsDirty() {
		t.Error("IsDirty() after ReadRadio = true, want false")
	}

	a.mu.Lock()
	baselineSet := a.baseline != nil
	workingSet := a.working != nil
	independentPointers := a.baseline != nil && a.working != nil && &a.baseline.Channels[0] != &a.working.Channels[0]
	a.mu.Unlock()
	if !baselineSet || !workingSet {
		t.Errorf("ReadRadio: baselineSet=%v workingSet=%v, want both true", baselineSet, workingSet)
	}
	if !independentPointers {
		t.Error("ReadRadio: baseline/working Channels share backing storage, want an independent deep copy")
	}

	if got := rec.named("transfer:progress"); len(got) == 0 {
		t.Error("ReadRadio: no transfer:progress events recorded")
	}
	doneEvents := rec.named("transfer:done")
	if len(doneEvents) != 1 {
		t.Fatalf("ReadRadio: %d transfer:done events, want exactly 1", len(doneEvents))
	}
	doneEv, ok := doneEvents[0].data.(TransferDoneEvent)
	if !ok {
		t.Fatalf("transfer:done payload type = %T, want TransferDoneEvent", doneEvents[0].data)
	}
	if doneEv.Kind != "read" || doneEv.Outcome != "ok" {
		t.Errorf("ReadRadio transfer:done = %+v, want Kind=read Outcome=ok", doneEv)
	}

	// GetCodeplug must reflect the same working copy.
	got, err := a.GetCodeplug()
	if err != nil {
		t.Fatalf("GetCodeplug: %v", err)
	}
	if got.Radio.CATID != view.Radio.CATID || len(got.Channels) != len(view.Channels) {
		t.Errorf("GetCodeplug = %+v, want it to match ReadRadio's view", got)
	}
}

// TestUpdateChannel_UnknownSlotRefuses pins "slot must exist in
// working's inventory" (task-15 brief §2) without needing a connection.
func TestUpdateChannel_UnknownSlotRefuses(t *testing.T) {
	a, _ := newTestApp(t)
	a.mu.Lock()
	a.working = &codeplug.Codeplug{Schema: codeplug.CurrentSchema, Channels: []codeplug.Channel{{Slot: "001"}}}
	a.mu.Unlock()

	_, err := a.UpdateChannel(codeplug.Channel{Slot: "999", Data: &codeplug.ChannelData{FreqHz: 7_000_000, Mode: "USB"}})
	var unknown *UnknownSlotError
	if !errors.As(err, &unknown) {
		t.Fatalf("UpdateChannel(unknown slot): err = %v, want *UnknownSlotError", err)
	}
	if unknown.Slot != "999" {
		t.Errorf("UnknownSlotError.Slot = %q, want %q", unknown.Slot, "999")
	}
	if a.IsDirty() {
		t.Error("UpdateChannel(unknown slot) set dirty, want it refused with nothing applied")
	}
}

// TestUpdateChannels_UnknownSlotRefusesWholeBatch pins the bulk-edit
// wholesale-refusal rule: one bad slot in a batch refuses the WHOLE
// batch, matching internal/csvmerge's established convention — nothing
// is partially applied.
func TestUpdateChannels_UnknownSlotRefusesWholeBatch(t *testing.T) {
	a, _ := newTestApp(t)
	a.mu.Lock()
	a.working = &codeplug.Codeplug{Schema: codeplug.CurrentSchema, Channels: []codeplug.Channel{{Slot: "001"}, {Slot: "002"}}}
	a.mu.Unlock()

	chs := []codeplug.Channel{
		{Slot: "001", Data: &codeplug.ChannelData{FreqHz: 7_000_000, Mode: "USB"}},
		{Slot: "999", Data: &codeplug.ChannelData{FreqHz: 7_000_000, Mode: "USB"}},
	}
	if _, err := a.UpdateChannels(chs); err == nil {
		t.Fatal("UpdateChannels(one unknown slot): err = nil, want a refusal")
	}
	a.mu.Lock()
	slot001Untouched := a.working.Channels[0].Data == nil
	a.mu.Unlock()
	if !slot001Untouched {
		t.Error("UpdateChannels(one unknown slot): slot 001 was applied despite the batch refusal")
	}
}

// TestUpdateChannel_NothingLoaded pins the ErrNothingLoaded guard.
func TestUpdateChannel_NothingLoaded(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.UpdateChannel(codeplug.Channel{Slot: "001"}); !errors.Is(err, ErrNothingLoaded) {
		t.Errorf("UpdateChannel with nothing loaded: err = %v, want ErrNothingLoaded", err)
	}
}

// TestValidate_NothingLoaded pins the ErrNothingLoaded guard for
// Validate.
func TestValidate_NothingLoaded(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.Validate(); !errors.Is(err, ErrNothingLoaded) {
		t.Errorf("Validate with nothing loaded: err = %v, want ErrNothingLoaded", err)
	}
}

// TestValidate_DisconnectedIsAdvisory pins the disconnected-advisory
// half of task-15 brief §2's Validate bullet — no radio session
// involved at all.
func TestValidate_DisconnectedIsAdvisory(t *testing.T) {
	a, _ := newTestApp(t)
	a.mu.Lock()
	a.working = &codeplug.Codeplug{
		Schema: codeplug.CurrentSchema,
		Radio:  codeplug.RadioInfo{Model: "FT-710", CATID: "0800"},
		Channels: []codeplug.Channel{
			{Slot: "001", Data: &codeplug.ChannelData{FreqHz: 7_000_000, Mode: "USB", CTCSS: "OFF", Shift: "SIMPLEX", CTCSSTone: codeplug.ToneField{State: codeplug.Unknown}, ScanSkip: codeplug.BoolField{State: codeplug.Unknown}}},
		},
	}
	a.mu.Unlock()

	view, err := a.Validate()
	if err != nil {
		t.Fatalf("Validate: unexpected error: %v", err)
	}
	if !view.Advisory {
		t.Error("Validate while disconnected: Advisory = false, want true")
	}
}

// TestDiffAgainstRadio_NothingLoaded pins the ErrNothingLoaded guard
// (after the connection check has already passed).
func TestDiffAgainstRadio_NothingLoaded(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.ConnectDemo(); err != nil {
		t.Fatalf("ConnectDemo: %v", err)
	}
	if _, err := a.DiffAgainstRadio(); !errors.Is(err, ErrNothingLoaded) {
		t.Errorf("DiffAgainstRadio with nothing loaded: err = %v, want ErrNothingLoaded", err)
	}
}
