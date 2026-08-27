// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

func syntheticWorking() *codeplug.Codeplug {
	return &codeplug.Codeplug{
		Schema:    codeplug.CurrentSchema,
		Generator: "test",
		Radio:     codeplug.RadioInfo{Model: "FT-710", CATID: "0800"},
		Channels: []codeplug.Channel{
			{Slot: "001", Data: &codeplug.ChannelData{FreqHz: 7_000_000, Mode: "USB", CTCSS: "OFF", Shift: "SIMPLEX", CTCSSTone: codeplug.ToneField{State: codeplug.Unknown}, ScanSkip: codeplug.BoolField{State: codeplug.Unknown}, TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false}}},
			{Slot: "002"},
		},
	}
}

// TestSaveFile_ClearsDirtyAndSetsWorkingPath pins SaveFile's direct-path
// behaviour (no dialog).
func TestSaveFile_ClearsDirtyAndSetsWorkingPath(t *testing.T) {
	a, _ := newTestApp(t)
	a.mu.Lock()
	a.working = syntheticWorking()
	a.dirty = true
	a.mu.Unlock()

	path := filepath.Join(t.TempDir(), "out.json")
	if err := a.SaveFile(path); err != nil {
		t.Fatalf("SaveFile: unexpected error: %v", err)
	}
	if a.IsDirty() {
		t.Error("IsDirty() after SaveFile = true, want false")
	}
	view, err := a.GetCodeplug()
	if err != nil {
		t.Fatalf("GetCodeplug: %v", err)
	}
	if view.WorkingPath != path {
		t.Errorf("WorkingPath = %q, want %q", view.WorkingPath, path)
	}

	reloaded, err := codeplug.Load(path)
	if err != nil {
		t.Fatalf("codeplug.Load(%s): %v", path, err)
	}
	if len(reloaded.Channels) != 2 {
		t.Errorf("reloaded Channels = %+v, want 2 entries", reloaded.Channels)
	}
}

// TestSaveFile_EmptyPathRefuses pins ErrEmptyPath.
func TestSaveFile_EmptyPathRefuses(t *testing.T) {
	a, _ := newTestApp(t)
	a.mu.Lock()
	a.working = syntheticWorking()
	a.mu.Unlock()
	if err := a.SaveFile(""); !errors.Is(err, ErrEmptyPath) {
		t.Errorf("SaveFile(\"\"): err = %v, want ErrEmptyPath", err)
	}
}

// TestSaveFile_NothingLoaded pins ErrNothingLoaded.
func TestSaveFile_NothingLoaded(t *testing.T) {
	a, _ := newTestApp(t)
	if err := a.SaveFile("/tmp/whatever.json"); !errors.Is(err, ErrNothingLoaded) {
		t.Errorf("SaveFile with nothing loaded: err = %v, want ErrNothingLoaded", err)
	}
}

// TestSaveFileAs_UsesDialogAndSaves drives SaveFileAs through the
// injected dialog fake (task-15 brief §5: "import/export via injected
// dialog fakes" — the same pattern applies to save/load).
func TestSaveFileAs_UsesDialogAndSaves(t *testing.T) {
	a, _ := newTestApp(t)
	a.mu.Lock()
	a.working = syntheticWorking()
	a.mu.Unlock()

	path := filepath.Join(t.TempDir(), "chosen.json")
	a.dialogs.(*fakeDialogs).saveFilePath = path

	got, err := a.SaveFileAs()
	if err != nil {
		t.Fatalf("SaveFileAs: unexpected error: %v", err)
	}
	if got != path {
		t.Errorf("SaveFileAs returned %q, want %q", got, path)
	}
	if _, err := codeplug.Load(path); err != nil {
		t.Errorf("SaveFileAs did not actually save to %s: %v", path, err)
	}
}

// TestSaveFileAs_DialogCancelled pins the "user cancelled" contract:
// ("", nil) from the dialog means ("", nil) back, nothing saved.
func TestSaveFileAs_DialogCancelled(t *testing.T) {
	a, _ := newTestApp(t)
	a.mu.Lock()
	a.working = syntheticWorking()
	a.dirty = true
	a.mu.Unlock()

	got, err := a.SaveFileAs() // fakeDialogs zero value: empty path, nil error
	if err != nil || got != "" {
		t.Errorf("SaveFileAs(cancelled) = (%q, %v), want (\"\", nil)", got, err)
	}
	if !a.IsDirty() {
		t.Error("SaveFileAs(cancelled) cleared dirty, want it untouched")
	}
}

// TestLoadFilePath_ReplacesWorking pins LoadFile's direct-path variant
// (task-15 brief §2: "direct-path variants for testability").
func TestLoadFilePath_ReplacesWorking(t *testing.T) {
	a, _ := newTestApp(t)
	path := filepath.Join(t.TempDir(), "fixture.json")
	if err := codeplug.Save(path, syntheticWorking()); err != nil {
		t.Fatalf("Save fixture: %v", err)
	}

	view, err := a.loadFilePath(path)
	if err != nil {
		t.Fatalf("loadFilePath: unexpected error: %v", err)
	}
	if view.WorkingPath != path || view.Dirty {
		t.Errorf("loadFilePath view = %+v, want WorkingPath=%q Dirty=false", view, path)
	}
	if len(view.Channels) != 2 {
		t.Errorf("loadFilePath: Channels = %+v, want 2 entries", view.Channels)
	}
}

// TestLoadFile_RefusesWhileTransferRunning is also covered end-to-end
// (against a real in-flight transfer) by
// TestConfirmSend_CancelMidTransfer_AndBusyExclusion; this pins the same
// guard directly, cheaply, without a radio.
func TestLoadFile_RefusesWhileTransferRunning(t *testing.T) {
	a, _ := newTestApp(t)
	a.mu.Lock()
	a.transfer = transferState{running: true, cancel: func() {}}
	a.mu.Unlock()

	if _, err := a.LoadFile(); !errors.Is(err, ErrTransferRunning) {
		t.Errorf("LoadFile while transfer running: err = %v, want ErrTransferRunning", err)
	}
}

// TestIsDirty_IsTheDirtyLoadProtectionSurface pins task-15 brief §5's
// "dirty-load protection surface": the App itself does NOT refuse
// LoadFile just because working is dirty (task-15 brief §2: "if dirty,
// the FRONTEND asks — provide IsDirty() bool") — IsDirty() is the one
// surface a caller uses to decide whether to prompt BEFORE calling
// LoadFile. This proves both halves: IsDirty() correctly reports the
// edit, and LoadFile proceeds (is not itself a dirty gate).
func TestIsDirty_IsTheDirtyLoadProtectionSurface(t *testing.T) {
	a, _ := newTestApp(t)
	a.mu.Lock()
	a.working = syntheticWorking()
	a.mu.Unlock()

	if _, err := a.UpdateChannel(codeplug.Channel{Slot: "002", Data: &codeplug.ChannelData{FreqHz: 14_000_000, Mode: "USB", CTCSS: "OFF", Shift: "SIMPLEX", CTCSSTone: codeplug.ToneField{State: codeplug.Unknown}, ScanSkip: codeplug.BoolField{State: codeplug.Unknown}, TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false}}}); err != nil {
		t.Fatalf("UpdateChannel: %v", err)
	}
	if !a.IsDirty() {
		t.Fatal("IsDirty() after an edit = false, want true — the frontend has no other way to know to prompt")
	}

	fixturePath := filepath.Join(t.TempDir(), "other.json")
	if err := codeplug.Save(fixturePath, syntheticWorking()); err != nil {
		t.Fatalf("Save fixture: %v", err)
	}
	a.dialogs.(*fakeDialogs).openFilePath = fixturePath

	if _, err := a.LoadFile(); err != nil {
		t.Fatalf("LoadFile while dirty: unexpected error: %v (App must not itself gate on dirty)", err)
	}
	if a.IsDirty() {
		t.Error("IsDirty() after LoadFile = true, want false (the load replaced working with a clean copy)")
	}
}

// freqOfSlot returns the FreqHz of cp's channel with the given slot, 0 if
// present-but-empty, or -1 if the slot is absent — a helper for the Fix 4
// revision-guard tests.
func freqOfSlot(cp *codeplug.Codeplug, slot string) int64 {
	for _, c := range cp.Channels {
		if c.Slot == slot {
			if c.Data == nil {
				return 0
			}
			return int64(c.Data.FreqHz)
		}
	}
	return -1
}

// editChannelFreq builds a well-formed channel edit for slot at freq,
// matching syntheticWorking's channel shape.
func editChannelFreq(slot string, freq int64) codeplug.Channel {
	return codeplug.Channel{Slot: slot, Data: &codeplug.ChannelData{
		FreqHz: uint64(freq), Mode: "USB", CTCSS: "OFF", Shift: "SIMPLEX",
		CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
		TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false},
		ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
	}}
}

// TestWorkingRev_MutationsBumpItAndSaveDoesNot (Codex M8b #4): the
// working-copy revision counter Fix 4 relies on is bumped by every content
// mutation (a channel edit, a settings merge) but NOT by SaveFile — so a
// SaveFile in flight can tell whether the copy changed under it. The
// deterministic companion to the concurrent race test below.
func TestWorkingRev_MutationsBumpItAndSaveDoesNot(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.ConnectDemo(""); err != nil {
		t.Fatalf("ConnectDemo: %v", err)
	}
	if _, err := a.ReadRadio(); err != nil {
		t.Fatalf("ReadRadio: %v", err)
	}

	rev := func() uint64 {
		a.mu.Lock()
		defer a.mu.Unlock()
		return a.workingRev
	}
	a.mu.Lock()
	slot := a.working.Channels[0].Slot
	a.mu.Unlock()

	r0 := rev()
	if _, err := a.UpdateChannel(editChannelFreq(slot, 7_100_000)); err != nil {
		t.Fatalf("UpdateChannel: %v", err)
	}
	r1 := rev()
	if r1 <= r0 {
		t.Errorf("UpdateChannel did not bump workingRev (%d -> %d)", r0, r1)
	}

	if _, err := a.ReadSettingsRadio(); err != nil {
		t.Fatalf("ReadSettingsRadio: %v", err)
	}
	r2 := rev()
	if r2 <= r1 {
		t.Errorf("ReadSettingsRadio did not bump workingRev (%d -> %d) — a concurrent save could not detect the mid-save merge", r1, r2)
	}

	path := filepath.Join(t.TempDir(), "cp.json")
	if err := a.SaveFile(path); err != nil {
		t.Fatalf("SaveFile: %v", err)
	}
	r3 := rev()
	if r3 != r2 {
		t.Errorf("SaveFile bumped workingRev (%d -> %d), want it unchanged (Save is not a content mutation)", r2, r3)
	}
	if a.IsDirty() {
		t.Error("IsDirty() after an unraced SaveFile = true, want false (rev unchanged, so dirty must clear)")
	}
}

// TestSaveFile_RevisionGuard_NoSilentLossUnderConcurrentMutation (Codex M8b
// #4, run under -race): SaveFile snapshots the working copy under a.mu,
// writes OUTSIDE the lock, then clears dirty ONLY if the revision is
// unchanged. A mutation that lands mid-save (here an UpdateChannel — the
// same bumpWorkingRevLocked path ReadSettingsRadio uses) must therefore
// never leave the newer working copy marked clean behind the older on-disk
// snapshot. Race the two many times; on every interleaving assert the safety
// invariant: if the on-disk file disagrees with the current working copy,
// dirty is true.
func TestSaveFile_RevisionGuard_NoSilentLossUnderConcurrentMutation(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.ConnectDemo(""); err != nil {
		t.Fatalf("ConnectDemo: %v", err)
	}
	if _, err := a.ReadRadio(); err != nil {
		t.Fatalf("ReadRadio: %v", err)
	}

	a.mu.Lock()
	slot := a.working.Channels[0].Slot
	a.mu.Unlock()

	dir := t.TempDir()
	const iterations = 100
	interleavings := 0
	for i := 0; i < iterations; i++ {
		path := filepath.Join(dir, fmt.Sprintf("cp-%d.json", i))
		newFreq := int64(7_000_000 + i*1000)

		var wg sync.WaitGroup
		wg.Add(2)
		var saveErr error
		go func() {
			defer wg.Done()
			saveErr = a.SaveFile(path)
		}()
		go func() {
			defer wg.Done()
			_, _ = a.UpdateChannel(editChannelFreq(slot, newFreq))
		}()
		wg.Wait()

		if saveErr != nil {
			t.Fatalf("iter %d: SaveFile error = %v (a plain channel edit must never refuse Save)", i, saveErr)
		}

		onDisk, err := codeplug.Load(path)
		if err != nil {
			t.Fatalf("iter %d: Load(%s): %v", i, path, err)
		}
		a.mu.Lock()
		workingFreq := freqOfSlot(a.working, slot)
		dirty := a.dirty
		a.mu.Unlock()
		diskFreq := freqOfSlot(onDisk, slot)

		if diskFreq != workingFreq {
			interleavings++
			if !dirty {
				t.Fatalf("iter %d: SILENT LOSS — on-disk slot %q freq = %d but working freq = %d, yet dirty = false", i, slot, diskFreq, workingFreq)
			}
		}
	}
	// Informational: with real fsync per save the fast edit lands after the
	// snapshot on the overwhelming majority of iterations, so this is
	// essentially always > 0; the safety assertion above ran on every one
	// regardless.
	t.Logf("observed %d/%d save/mutation interleavings (disk != working)", interleavings, iterations)
}

// v4BodyNoTierKeys is a schema-4 codeplug file for model/catID with ONE
// populated channel in slot that has NO KEY for any of the ten fields the
// Icom tier added — the file shape deviation (c) is about, and the shape
// the GUI's own bankless add produces (a row built with no BankView in
// hand omits every tier key: app/frontend/src/lib/grid/columns.js's
// tierDefaults).
//
// Hand-written rather than produced by codeplug.Save, deliberately: Save
// emits the LOWEST schema that can represent the content, so a channel
// with nothing recorded in those ten fields is written as schema 3 — the
// case that was never in doubt, since the schema-3 loader migrates all
// ten to Unavailable unconditionally.
func v4BodyNoTierKeys(model, catID, slot, mode string) string {
	return fmt.Sprintf(`{"schema":4,"generator":"test","radio":{"model":%q,"cat_id":%q,"read_at":"2026-08-27T09:00:00Z"},`+
		`"channels":[{"slot":%q,"data":{"freq_hz":145500000,"mode":%q,"ctcss":"OFF","ctcss_tone":{"state":"unavailable"},`+
		`"shift":"SIMPLEX","tag":"","tag_display":{"state":"unavailable"},"scan_skip":{"state":"unavailable"}}}]}`,
		model, catID, slot, mode)
}

// writeV4Fixture writes body to a temp file and returns its path.
func writeV4Fixture(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "v4.json")
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	return path
}

// tierStatesOf returns the ten tier-added field states of the working
// copy's first channel, in codeplug.ChannelData declaration order.
func tierStatesOf(t *testing.T, a *App) []codeplug.FieldState {
	t.Helper()
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.working == nil || len(a.working.Channels) == 0 || a.working.Channels[0].Data == nil {
		t.Fatalf("working copy has no populated first channel: %+v", a.working)
	}
	d := a.working.Channels[0].Data
	return []codeplug.FieldState{
		d.TxFreqHz.State, d.Duplex.State, d.OffsetHz.State, d.ToneMode.State,
		d.ToneTx.State, d.ToneRx.State, d.DTCSCode.State, d.DTCSPolarity.State,
		d.Filter.State, d.DataMode.State,
	}
}

// TestLoadFilePath_TierFieldsNormalisedAgainstTheFileSOwnModel is
// deviation (c)'s GUI half, end to end through the real registry (Wave 4
// task R2).
//
// A schema-4 file with no key for any of the ten tier-added fields loads
// Absent — codeplug.Load has no capabilities and cannot know better — and
// loadFilePath resolves each field against the capabilities of the model
// the FILE names:
//
//   - FT-710: every one of the ten is unreachable on every bank, so all
//     ten become Unavailable — the same answer a schema-1/2/3 file
//     migrates to, and the same answer a read of that radio gives, which
//     is what stops codeplug.Diff reporting such a channel as modified.
//   - IC-7610: tone_mode, tone_tx, tone_rx and filter ARE reachable on
//     this radio's MEM bank (the four its 1A 00 record maps — see
//     ic7610TierFields), so silence about them is NOT a claim that the
//     radio lacks them and they stay Absent. The other six become
//     Unavailable.
//
// The IC-7610 half also pins what the rest of the App then does with a
// reachable-but-Absent field, which is the reason leaving it alone is
// safe: Validate REFUSES the channel, naming each field. Nothing anywhere
// invents a value for it.
func TestLoadFilePath_TierFieldsNormalisedAgainstTheFileSOwnModel(t *testing.T) {
	unavailableTen := make([]codeplug.FieldState, 10)
	for i := range unavailableTen {
		unavailableTen[i] = codeplug.Unavailable
	}

	t.Run("FT-710: all ten unreachable, so all ten become Unavailable", func(t *testing.T) {
		a, _ := newTestApp(t)
		path := writeV4Fixture(t, v4BodyNoTierKeys(wiring.DefaultModel, "0800", "001", "FM"))
		if _, err := a.loadFilePath(path); err != nil {
			t.Fatalf("loadFilePath: unexpected error: %v", err)
		}
		if got := tierStatesOf(t, a); !reflect.DeepEqual(got, unavailableTen) {
			t.Errorf("tier states = %v, want all Unavailable", got)
		}
	})

	t.Run("IC-7610: the four fields this radio HAS stay Absent", func(t *testing.T) {
		a, _ := newTestApp(t)
		path := writeV4Fixture(t, v4BodyNoTierKeys(wiring.IC7610Model, "98", "001", "FM"))
		if _, err := a.loadFilePath(path); err != nil {
			t.Fatalf("loadFilePath: unexpected error: %v", err)
		}
		want := []codeplug.FieldState{
			codeplug.Unavailable, codeplug.Unavailable, codeplug.Unavailable, // tx_frequency, duplex, offset
			codeplug.Absent, codeplug.Absent, codeplug.Absent, // tone_mode, tone_tx, tone_rx
			codeplug.Unavailable, codeplug.Unavailable, // dtcs_code, dtcs_polarity
			codeplug.Absent,      // filter
			codeplug.Unavailable, // data_mode
		}
		if got := tierStatesOf(t, a); !reflect.DeepEqual(got, want) {
			t.Errorf("tier states = %v, want %v (the four reachable ones — %v — stay Absent)", got, want, ic7610TierFields)
		}

		// The pinned consequence: Validate refuses such a channel, field
		// by field, rather than anything inventing a value for it.
		view, err := a.Validate()
		if err != nil {
			t.Fatalf("Validate: unexpected error: %v", err)
		}
		refused := make(map[string]bool)
		for _, i := range view.Issues {
			if i.Slot == "001" && i.Severity == string(codeplug.SeverityError) && strings.Contains(i.Msg, "says nothing about it") {
				refused[i.Field] = true
			}
		}
		for _, f := range ic7610TierFields {
			if !refused[f] {
				t.Errorf("Validate did not refuse the Absent %s on a radio that has the field; issues = %+v", f, view.Issues)
			}
		}
	})

	t.Run("an unrecognised model is left exactly as the file wrote it", func(t *testing.T) {
		// No capabilities to key on, so nothing is claimed: the ten stay
		// Absent rather than being normalised against some other radio's
		// baseline. currentCaps then resolves this file to the FT-710's
		// baseline, where none of the ten is reachable and Validate
		// therefore judges none of them.
		a, _ := newTestApp(t)
		path := writeV4Fixture(t, v4BodyNoTierKeys("NO-SUCH-RADIO", "ZZZZ", "001", "FM"))
		if _, err := a.loadFilePath(path); err != nil {
			t.Fatalf("loadFilePath: unexpected error: %v", err)
		}
		absentTen := make([]codeplug.FieldState, 10)
		if got := tierStatesOf(t, a); !reflect.DeepEqual(got, absentTen) {
			t.Errorf("tier states = %v, want all Absent (codeplug.Absent is the zero FieldState)", got)
		}
	})
}
