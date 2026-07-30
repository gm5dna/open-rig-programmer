// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/internal/fakeradio"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// These black-box tests drive the compiled rigprog binary itself (via
// runBinary/TestMain, see main_test.go), pinning task-11 brief §6's
// externally observable contract — stdout/stderr/exit code — the way a
// calling script would see it, distinct from the in-process tests in
// run_test.go/ports_test.go/probe_test.go that call run()/cmdXxx
// directly.

func TestBlackbox_UnknownSubcommand(t *testing.T) {
	r := runBinary(t, "", "frobnicate")
	if r.exitCode != exitUsage {
		t.Errorf("exit code = %d, want exitUsage (%d); stderr=%q", r.exitCode, exitUsage, r.stderr)
	}
}

func TestBlackbox_NoArgs(t *testing.T) {
	r := runBinary(t, "")
	if r.exitCode != exitUsage {
		t.Errorf("exit code = %d, want exitUsage (%d); stderr=%q", r.exitCode, exitUsage, r.stderr)
	}
	if !strings.Contains(r.stderr, "ports") {
		t.Errorf("stderr = %q, want usage text listing subcommands", r.stderr)
	}
}

func TestBlackbox_ProbeNeitherPortNorFake(t *testing.T) {
	r := runBinary(t, "", "probe")
	if r.exitCode != exitUsage {
		t.Errorf("exit code = %d, want exitUsage (%d); stderr=%q", r.exitCode, exitUsage, r.stderr)
	}
}

func TestBlackbox_ProbeFake(t *testing.T) {
	// HW-CONFIRMED 2026-07-13 (docs/hardware-notes.md §60m regional
	// finding): the default fakeradio image (ImageUK) no longer
	// synthesises a 60m/EMG bank — Stuart's real UK FT-710 has neither.
	r := runBinary(t, "", "probe", "--fake")
	if r.exitCode != exitSuccess {
		t.Fatalf("exit code = %d, want exitSuccess (%d); stderr=%q", r.exitCode, exitSuccess, r.stderr)
	}
	for _, want := range []string{"FT-710", "0800", "no-60m", "60 m channels: 0", "EMG channel:   no"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("stdout = %q, want it to contain %q", r.stdout, want)
		}
	}
}

// TestBlackbox_ProbeFake_ModelFlagByteIdentical pins task 40's headline
// requirement at the compiled-binary level: "probe --fake --model
// FT-710" (the default, spelled out explicitly) is byte-identical —
// stdout, stderr, exit code — to "probe --fake" with no --model flag at
// all.
func TestBlackbox_ProbeFake_ModelFlagByteIdentical(t *testing.T) {
	want := runBinary(t, "", "probe", "--fake")
	got := runBinary(t, "", "probe", "--fake", "--model", "FT-710")

	if got.exitCode != want.exitCode {
		t.Errorf("exit code = %d, want %d (flag-absent)", got.exitCode, want.exitCode)
	}
	if got.stdout != want.stdout {
		t.Errorf("stdout = %q, want byte-identical to flag-absent %q", got.stdout, want.stdout)
	}
	if got.stderr != want.stderr {
		t.Errorf("stderr = %q, want byte-identical to flag-absent %q", got.stderr, want.stderr)
	}
}

// TestBlackbox_ProbeUnknownModel pins task 40's --model validation at the
// compiled-binary level: an unrecognised model exits 2 (usage), naming
// FT-710, without touching the (nonexistent) fake radio path at all —
// combined here with --fake to prove the rejection happens BEFORE any
// session is opened, not as a wrong-radio failure from one.
func TestBlackbox_ProbeUnknownModel(t *testing.T) {
	r := runBinary(t, "", "probe", "--fake", "--model", unknownModelSentinel)
	if r.exitCode != exitUsage {
		t.Fatalf("exit code = %d, want exitUsage (%d); stdout=%q stderr=%q", r.exitCode, exitUsage, r.stdout, r.stderr)
	}
	if !strings.Contains(r.stderr, unknownModelSentinel) || !strings.Contains(r.stderr, "FT-710") {
		t.Errorf("stderr = %q, want it to name both the rejected model and the supported one", r.stderr)
	}
}

func TestBlackbox_PortsFakeRejected(t *testing.T) {
	r := runBinary(t, "", "ports", "--fake")
	if r.exitCode != exitUsage {
		t.Errorf("exit code = %d, want exitUsage (%d); stderr=%q", r.exitCode, exitUsage, r.stderr)
	}
}

// countPopulatedImage returns how many slots in img are populated — used
// to derive the expected populated count from fakeradio.ImageUK() itself
// rather than hardcoding a number that could silently drift out of sync
// with the fixture.
func countPopulatedImage(img map[string]fakeradio.MemState) int {
	n := 0
	for _, st := range img {
		if st.Populated {
			n++
		}
	}
	return n
}

// TestBlackbox_Read pins task-12 brief §3's black-box read contract:
// "read --fake --out f.json" -> exit 0, file exists, loads via
// codeplug.Load, populated count matches the default ImageUK factory
// image; re-run without --force -> exit 1 (no radio touched — the
// refuse-overwrite check happens before session open); with --force ->
// 0. Two ReadAlls total (~10s at current fakeradio pacing).
func TestBlackbox_Read(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "read-out.json")

	r := runBinary(t, "", "read", "--fake", "--out", out)
	if r.exitCode != exitSuccess {
		t.Fatalf("read --fake --out: exit code = %d, want exitSuccess (%d); stderr=%q", r.exitCode, exitSuccess, r.stderr)
	}
	cp, err := codeplug.Load(out)
	if err != nil {
		t.Fatalf("codeplug.Load(%s): %v", out, err)
	}
	wantPopulated := countPopulatedImage(fakeradio.ImageUK())
	gotPopulated := 0
	for _, ch := range cp.Channels {
		if !ch.Empty() {
			gotPopulated++
		}
	}
	if gotPopulated != wantPopulated {
		t.Errorf("populated channel count = %d, want %d (fakeradio.ImageUK())", gotPopulated, wantPopulated)
	}
	if !strings.Contains(r.stdout, out) {
		t.Errorf("read --fake --out stdout = %q, want it to mention the output path %q", r.stdout, out)
	}
	if !strings.Contains(r.stderr, "read ") {
		t.Errorf("read --fake stderr = %q, want it to contain progress line with 'read ' phase", r.stderr)
	}

	rNoForce := runBinary(t, "", "read", "--fake", "--out", out)
	if rNoForce.exitCode != exitError {
		t.Errorf("re-run without --force: exit code = %d, want exitError (%d); stderr=%q", rNoForce.exitCode, exitError, rNoForce.stderr)
	}

	rForce := runBinary(t, "", "read", "--fake", "--out", out, "--force")
	if rForce.exitCode != exitSuccess {
		t.Errorf("re-run with --force: exit code = %d, want exitSuccess (%d); stderr=%q", rForce.exitCode, exitSuccess, rForce.stderr)
	}
}

// TestBlackbox_Read_ModelFlagByteIdentical pins task 40's headline
// requirement at the compiled-binary level: "read --fake --model FT-710"
// (the default, spelled out explicitly) produces a byte-identical
// summary and channel digest to "read --fake" with no --model flag at
// all. Two ReadAlls total (~10s at current fakeradio pacing).
func TestBlackbox_Read_ModelFlagByteIdentical(t *testing.T) {
	dir := t.TempDir()
	wantOut := filepath.Join(dir, "want.json")
	gotOut := filepath.Join(dir, "got.json")

	want := runBinary(t, "", "read", "--fake", "--out", wantOut)
	if want.exitCode != exitSuccess {
		t.Fatalf("read --fake --out (flag-absent): exit code = %d, want exitSuccess (%d); stderr=%q", want.exitCode, exitSuccess, want.stderr)
	}
	got := runBinary(t, "", "read", "--fake", "--model", "FT-710", "--out", gotOut)
	if got.exitCode != want.exitCode {
		t.Fatalf("exit code = %d, want %d (flag-absent)", got.exitCode, want.exitCode)
	}

	// Both runs mention their own (different) --out path, so compare the
	// summaries with that one expected difference substituted out.
	wantSummary := strings.ReplaceAll(want.stdout, wantOut, "<OUT>")
	gotSummary := strings.ReplaceAll(got.stdout, gotOut, "<OUT>")
	if gotSummary != wantSummary {
		t.Errorf("stdout = %q, want byte-identical (modulo --out path) to flag-absent %q", gotSummary, wantSummary)
	}

	wantCp, err := codeplug.Load(wantOut)
	if err != nil {
		t.Fatalf("codeplug.Load(%s): %v", wantOut, err)
	}
	gotCp, err := codeplug.Load(gotOut)
	if err != nil {
		t.Fatalf("codeplug.Load(%s): %v", gotOut, err)
	}
	if got, want := codeplug.Digest(gotCp.Channels), codeplug.Digest(wantCp.Channels); got != want {
		t.Errorf("channels digest = %s, want %s (byte-identical to flag-absent)", got, want)
	}
}

// ft710SettingsDescriptorItemCount returns the total item count across
// every menu/group in the FT-710's static settings descriptor — used
// instead of a hardcoded 296, so a future descriptor change cannot
// silently desync this test from the real inventory (task-34 brief's own
// "296" figure is asserted below as a staleness guard, not baked into
// the walk itself). Sourced via wiring.StaticSettingsDescriptor (task 40:
// this file no longer imports core/driver/ft710 directly) rather than
// ft710.SettingsDescriptor() directly — both return the same tree (see
// ft710Driver.StaticSettingsDescriptor's own doc comment).
func ft710SettingsDescriptorItemCount(t *testing.T) int {
	t.Helper()
	descriptor, ok, err := wiring.StaticSettingsDescriptor(wiring.DefaultModel)
	if err != nil {
		t.Fatalf("wiring.StaticSettingsDescriptor(%s): %v", wiring.DefaultModel, err)
	}
	if !ok {
		t.Fatalf("wiring.StaticSettingsDescriptor(%s): ok = false, want true (FT-710 has a settings surface)", wiring.DefaultModel)
	}
	total := 0
	for _, m := range descriptor.Menus {
		for _, g := range m.Groups {
			total += len(g.Items)
		}
	}
	return total
}

// TestBlackbox_ReadWithSettings pins task-34 brief's black-box
// "read --fake --settings" contract: exit 0; the saved file loads as
// Schema 2 with a non-nil, Complete MenuSnapshot whose entry count
// matches the FT-710's full descriptor (296 items — fakeradio answers
// every default EX address Known, see internal/fakeradio's EXDefaults);
// stderr shows "read-settings" progress; stdout carries both settings
// summary lines. One read --fake --settings invocation (channel ReadAll
// plus a full 296-item settings read).
func TestBlackbox_ReadWithSettings(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "read-settings-out.json")

	r := runBinary(t, "", "read", "--fake", "--settings", "--out", out)
	if r.exitCode != exitSuccess {
		t.Fatalf("read --fake --settings --out: exit code = %d, want exitSuccess (%d); stdout=%q stderr=%q", r.exitCode, exitSuccess, r.stdout, r.stderr)
	}

	cp, err := codeplug.Load(out)
	if err != nil {
		t.Fatalf("codeplug.Load(%s): %v", out, err)
	}
	if cp.Schema != codeplug.CurrentSchema {
		t.Errorf("Schema = %d, want %d", cp.Schema, codeplug.CurrentSchema)
	}
	if cp.Menus == nil {
		t.Fatal("Menus is nil, want a populated MenuSnapshot")
	}
	if !cp.Menus.Complete {
		t.Error("Menus.Complete = false, want true (fakeradio answers every default EX address Known)")
	}

	wantCount := ft710SettingsDescriptorItemCount(t)
	if wantCount != 296 {
		t.Fatalf("test's own assumption about the FT-710 descriptor is stale: item count = %d, want 296", wantCount)
	}
	if len(cp.Menus.Entries) != wantCount {
		t.Errorf("len(Menus.Entries) = %d, want %d (ft710.SettingsDescriptor() item count)", len(cp.Menus.Entries), wantCount)
	}

	if !strings.Contains(r.stderr, "read-settings") {
		t.Errorf("read --fake --settings stderr = %q, want it to contain \"read-settings\" progress", r.stderr)
	}
	if !strings.Contains(r.stdout, "Settings read:") || !strings.Contains(r.stdout, "Settings unavailable:") {
		t.Errorf("read --fake --settings stdout = %q, want both settings summary lines", r.stdout)
	}
}

// TestBlackbox_ReadDefault_ChannelBehaviourUnchanged pins task-34 brief's
// core guarantee: a plain "read --fake --out" (no --settings) is
// COMPLETELY unaffected by this task — Menus stays nil, zero settings/EX
// wire traffic (proxied here by the absence of the "read-settings"
// progress phase, which ONLY clone.ReadSettings ever emits), and no
// settings summary lines. Cheap: one plain channel-only read.
func TestBlackbox_ReadDefault_ChannelBehaviourUnchanged(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "read-default-out.json")

	r := runBinary(t, "", "read", "--fake", "--out", out)
	if r.exitCode != exitSuccess {
		t.Fatalf("read --fake --out: exit code = %d, want exitSuccess (%d); stderr=%q", r.exitCode, exitSuccess, r.stderr)
	}

	cp, err := codeplug.Load(out)
	if err != nil {
		t.Fatalf("codeplug.Load(%s): %v", out, err)
	}
	if cp.Menus != nil {
		t.Errorf("Menus = %+v, want nil (default read must never touch the settings surface)", cp.Menus)
	}
	if strings.Contains(r.stderr, "read-settings") {
		t.Errorf("read --fake (default) stderr = %q, want NO \"read-settings\" progress (proves zero EX wire traffic)", r.stderr)
	}
	if strings.Contains(r.stdout, "Settings read:") || strings.Contains(r.stdout, "Settings unavailable:") {
		t.Errorf("read --fake (default) stdout = %q, want NO settings summary lines", r.stdout)
	}
}

// TestBlackbox_SettingsShow pins task-34 brief's black-box "settings"
// happy path: on a file produced by "read --fake --settings", a menu
// label ("RADIO SETTING"), a group label ("MODE SSB"), and (on the SAME
// rendered line as the known item "01-01-01 AF TREBLE GAIN") a known
// value both appear on stdout; exit 0. One settings-read setup
// invocation, reused by every assertion below (no second radio touch).
func TestBlackbox_SettingsShow(t *testing.T) {
	dir := t.TempDir()
	settingsFile := filepath.Join(dir, "settings-show.json")

	rRead := runBinary(t, "", "read", "--fake", "--settings", "--out", settingsFile)
	if rRead.exitCode != exitSuccess {
		t.Fatalf("setup read --fake --settings --out: exit code = %d, want exitSuccess (%d); stderr=%q", rRead.exitCode, exitSuccess, rRead.stderr)
	}

	r := runBinary(t, "", "settings", settingsFile)
	if r.exitCode != exitSuccess {
		t.Fatalf("settings FILE: exit code = %d, want exitSuccess (%d); stdout=%q stderr=%q", r.exitCode, exitSuccess, r.stdout, r.stderr)
	}
	for _, want := range []string{"RADIO SETTING", "MODE SSB", "AF TREBLE GAIN"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("settings FILE stdout = %q, want it to contain %q", r.stdout, want)
		}
	}

	var itemLine string
	for _, line := range strings.Split(r.stdout, "\n") {
		if strings.Contains(line, "01-01-01") {
			itemLine = line
			break
		}
	}
	if itemLine == "" {
		t.Fatalf("settings FILE stdout = %q, want a rendered line for item 01-01-01", r.stdout)
	}
	// fakeradio seeds every numeric item's placeholder as n x '0'
	// (internal/fakeradio/doc.go register item 21), with the M8c overlay
	// supplying the SHAPE observed in that session's two sweeps of one
	// radio where the answer differed. AF TREBLE GAIN is a 3-byte item (Table 2) that answered
	// with an explicit leading sign on hardware, so the fake's runtime
	// default for it is "+01" (a synthetic magnitude — see that table) — see fakeradio.EXRuntimeDefaults.
	if !strings.Contains(itemLine, "+01") {
		t.Errorf("settings FILE stdout line for 01-01-01 = %q, want it to show a known value (\"+01\", the fake's runtime EX default)", itemLine)
	}
}

// TestBlackbox_SettingsShow_NoSnapshot pins task-34 brief's exit-1 case:
// a plain (default, no --settings) read's file carries no settings
// snapshot at all — "settings FILE" refuses, exit 1, stderr mentioning
// --settings. Cheap: one plain channel-only read.
func TestBlackbox_SettingsShow_NoSnapshot(t *testing.T) {
	dir := t.TempDir()
	plainFile := filepath.Join(dir, "settings-no-snapshot.json")

	rRead := runBinary(t, "", "read", "--fake", "--out", plainFile)
	if rRead.exitCode != exitSuccess {
		t.Fatalf("setup read --fake --out: exit code = %d, want exitSuccess (%d); stderr=%q", rRead.exitCode, exitSuccess, rRead.stderr)
	}

	r := runBinary(t, "", "settings", plainFile)
	if r.exitCode != exitError {
		t.Errorf("settings (no snapshot): exit code = %d, want exitError (%d); stdout=%q stderr=%q", r.exitCode, exitError, r.stdout, r.stderr)
	}
	if !strings.Contains(r.stderr, "--settings") {
		t.Errorf("settings (no snapshot) stderr = %q, want it to mention --settings", r.stderr)
	}
}

// TestBlackbox_SettingsCSV_NoClobber pins task-34 brief's --csv
// no-clobber contract: an existing --csv path refuses without --force
// (exit 1, left untouched), and with --force overwrites and produces a
// parseable CSV with the exact header "id,menu,group,label,state,value".
// One settings-read setup invocation.
func TestBlackbox_SettingsCSV_NoClobber(t *testing.T) {
	dir := t.TempDir()
	settingsFile := filepath.Join(dir, "settings-csv-src.json")

	rRead := runBinary(t, "", "read", "--fake", "--settings", "--out", settingsFile)
	if rRead.exitCode != exitSuccess {
		t.Fatalf("setup read --fake --settings --out: exit code = %d, want exitSuccess (%d); stderr=%q", rRead.exitCode, exitSuccess, rRead.stderr)
	}

	csvOut := filepath.Join(dir, "settings.csv")
	const sentinel = "stale csv contents"
	if err := os.WriteFile(csvOut, []byte(sentinel), 0o600); err != nil {
		t.Fatalf("seeding existing --csv file: %v", err)
	}

	rNoForce := runBinary(t, "", "settings", "--csv", csvOut, settingsFile)
	if rNoForce.exitCode != exitError {
		t.Errorf("settings --csv (existing, no --force): exit code = %d, want exitError (%d); stderr=%q", rNoForce.exitCode, exitError, rNoForce.stderr)
	}
	contents, err := os.ReadFile(csvOut)
	if err != nil {
		t.Fatalf("reading %s after refused settings --csv: %v", csvOut, err)
	}
	if string(contents) != sentinel {
		t.Error("settings --csv (existing, no --force) overwrote the file as a side effect, want it untouched")
	}

	rForce := runBinary(t, "", "settings", "--csv", csvOut, "--force", settingsFile)
	if rForce.exitCode != exitSuccess {
		t.Fatalf("settings --csv --force: exit code = %d, want exitSuccess (%d); stdout=%q stderr=%q", rForce.exitCode, exitSuccess, rForce.stdout, rForce.stderr)
	}

	f, err := os.Open(csvOut)
	if err != nil {
		t.Fatalf("opening exported CSV: %v", err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("parsing exported CSV: %v", err)
	}
	if len(rows) < 2 {
		t.Fatalf("CSV rows = %d, want a header plus at least one data row", len(rows))
	}
	wantHeader := []string{"id", "menu", "group", "label", "state", "value"}
	if len(rows[0]) != len(wantHeader) {
		t.Fatalf("CSV header = %v, want %v", rows[0], wantHeader)
	}
	for i, col := range wantHeader {
		if rows[0][i] != col {
			t.Errorf("CSV header[%d] = %q, want %q", i, rows[0][i], col)
		}
	}
}

// TestBlackbox_SettingsUsage pins the flags-first grammar at the
// compiled-binary level (task-34 brief): no FILE, an unrecognised flag,
// and the trailing-flag form ("settings FILE --csv OUT" — stdlib flag
// parsing stops at the first positional) are all exit 2. No radio
// touched at all.
func TestBlackbox_SettingsUsage(t *testing.T) {
	t.Run("no FILE", func(t *testing.T) {
		r := runBinary(t, "", "settings")
		if r.exitCode != exitUsage {
			t.Errorf("settings (no FILE): exit code = %d, want exitUsage (%d); stderr=%q", r.exitCode, exitUsage, r.stderr)
		}
	})
	t.Run("unknown flag", func(t *testing.T) {
		r := runBinary(t, "", "settings", "--bogus", "somefile.json")
		if r.exitCode != exitUsage {
			t.Errorf("settings --bogus: exit code = %d, want exitUsage (%d); stderr=%q", r.exitCode, exitUsage, r.stderr)
		}
	})
	t.Run("trailing flag form", func(t *testing.T) {
		r := runBinary(t, "", "settings", "somefile.json", "--csv", "out.csv")
		if r.exitCode != exitUsage {
			t.Errorf("settings FILE --csv OUT (trailing flag form): exit code = %d, want exitUsage (%d); stderr=%q", r.exitCode, exitUsage, r.stderr)
		}
	})
}

// mutateForDiff loads baselinePath, applies three changes matching
// task-12 brief §3's example — change one tag (Modified), add a channel
// in an empty slot (Added), empty a populated slot (Erased, blocked:
// erase is unsupported pre-M5b on every bank) — and saves the result to
// mutatedPath.
func mutateForDiff(t *testing.T, baselinePath, mutatedPath string) {
	t.Helper()
	cp, err := codeplug.Load(baselinePath)
	if err != nil {
		t.Fatalf("codeplug.Load(%s): %v", baselinePath, err)
	}

	bySlot := make(map[string]int, len(cp.Channels))
	for i, ch := range cp.Channels {
		bySlot[ch.Slot] = i
	}

	idx001, ok := bySlot["001"]
	if !ok || cp.Channels[idx001].Data == nil {
		t.Fatal("fixture assumption broken: slot 001 must be present and populated")
	}
	modified := *cp.Channels[idx001].Data
	modified.Tag = "MUTATED"
	cp.Channels[idx001].Data = &modified

	idx002, ok := bySlot["002"]
	if !ok || cp.Channels[idx002].Data != nil {
		t.Fatal("fixture assumption broken: slot 002 must be present and empty")
	}
	added := modified
	added.Tag = "ADDEDCH"
	cp.Channels[idx002].Data = &added

	idxP1L, ok := bySlot["P1L"]
	if !ok || cp.Channels[idxP1L].Data == nil {
		t.Fatal("fixture assumption broken: slot P1L must be present and populated")
	}
	cp.Channels[idxP1L].Data = nil

	if err := codeplug.Save(mutatedPath, cp); err != nil {
		t.Fatalf("codeplug.Save(%s): %v", mutatedPath, err)
	}
}

// TestBlackbox_Diff pins task-12 brief §3's black-box diff contract: a
// pre-read file diffed against itself is "no changes"; the SAME file,
// mutated (tag change + added channel + erased channel), shows Modified/
// Added/Erased with the erased entry marked unsupported, and a correct
// count line. One shared setup read plus two diffs: three ReadAlls total
// (~15s at current fakeradio pacing).
func TestBlackbox_Diff(t *testing.T) {
	dir := t.TempDir()
	baseline := filepath.Join(dir, "diff-baseline.json")

	rSetup := runBinary(t, "", "read", "--fake", "--out", baseline)
	if rSetup.exitCode != exitSuccess {
		t.Fatalf("setup read --fake --out: exit code = %d, want exitSuccess (%d); stderr=%q", rSetup.exitCode, exitSuccess, rSetup.stderr)
	}

	t.Run("no changes", func(t *testing.T) {
		r := runBinary(t, "", "diff", "--fake", baseline)
		if r.exitCode != exitSuccess {
			t.Fatalf("diff --fake (unchanged): exit code = %d, want exitSuccess (%d); stderr=%q", r.exitCode, exitSuccess, r.stderr)
		}
		if !strings.Contains(r.stdout, "No changes.") {
			t.Errorf("diff --fake (unchanged) stdout = %q, want it to contain %q", r.stdout, "No changes.")
		}
		if !strings.Contains(r.stderr, "read ") {
			t.Errorf("diff --fake (unchanged) stderr = %q, want it to contain progress line with 'read ' phase", r.stderr)
		}
	})

	t.Run("with changes", func(t *testing.T) {
		mutated := filepath.Join(dir, "diff-mutated.json")
		mutateForDiff(t, baseline, mutated)

		r := runBinary(t, "", "diff", "--fake", mutated)
		if r.exitCode != exitSuccess {
			t.Fatalf("diff --fake (mutated): exit code = %d, want exitSuccess (%d); stderr=%q", r.exitCode, exitSuccess, r.stderr)
		}
		for _, want := range []string{
			"Added:", "Modified:", "Erased:",
			"UNSUPPORTED — slot will keep its current contents",
			"Added 1, Modified 1, Erased 1, Blocked 1,",
		} {
			if !strings.Contains(r.stdout, want) {
				t.Errorf("diff --fake (mutated) stdout = %q, want it to contain %q", r.stdout, want)
			}
		}
		if !strings.Contains(r.stderr, "read ") {
			t.Errorf("diff --fake (mutated) stderr = %q, want it to contain progress line with 'read ' phase", r.stderr)
		}
	})
}

// TestBlackbox_PortsRealMachine is environment-dependent (task-11 brief
// §6): it asserts only that ports exits 0 and produces either the ranked
// table's header or the friendly no-candidates message, never a
// specific device.
func TestBlackbox_PortsRealMachine(t *testing.T) {
	r := runBinary(t, "", "ports")
	if r.exitCode != exitSuccess {
		t.Fatalf("exit code = %d, want exitSuccess (%d); stderr=%q", r.exitCode, exitSuccess, r.stderr)
	}
	hasHeader := strings.Contains(r.stdout, "PATH")
	hasNoCandidates := strings.Contains(r.stdout, "no serial ports found")
	if !hasHeader && !hasNoCandidates {
		t.Errorf("stdout = %q, want either the ranked table's header or the no-candidates message", r.stdout)
	}
}

// TestBlackbox_ExportRejectsPortFake and TestBlackbox_ImportRejectsPortFake
// pin task-13's OFFLINE requirement at the compiled-binary level: neither
// subcommand opens a radio session, so --port/--fake are not even
// declared flags — exit 2 (usage), same class as any other unrecognised
// flag.
func TestBlackbox_ExportRejectsPortFake(t *testing.T) {
	r := runBinary(t, "", "export", "--fake", "somefile.json", "--csv", "out.csv")
	if r.exitCode != exitUsage {
		t.Errorf("export --fake: exit code = %d, want exitUsage (%d); stderr=%q", r.exitCode, exitUsage, r.stderr)
	}
}

func TestBlackbox_ImportRejectsPortFake(t *testing.T) {
	r := runBinary(t, "", "import", "--port", "/dev/cu.fake", "--csv", "a.csv", "--into", "base.json", "--out", "out.json")
	if r.exitCode != exitUsage {
		t.Errorf("import --port: exit code = %d, want exitUsage (%d); stderr=%q", r.exitCode, exitUsage, r.stderr)
	}
}

// TestBlackbox_ExportImportRoundTrip pins task-13 brief §3's black-box
// round trip — the ONE fakeradio read this task's whole test suite
// spends (every other export/import test builds its fixtures entirely
// in-process, see import_test.go's buildValidBase): read --fake -> export
// --csv -> import --csv --into the same base -> the result's channels
// are byte-identical to the original read's (codeplug.Digest equality,
// content-only, ignoring RadioInfo timestamps).
//
// The import step's own exit code IS asserted as exitSuccess (AMENDED:
// offline validation is advisory-only, never exit-gating — see
// task-13-brief.md's controller amendment). HISTORICAL CONTEXT (no
// longer reproducible with the default fixture, see below): a "read
// --fake" baseline carrying discovered 60m channels (the FORMER ImageUK:
// 501-507, before its HW-CONFIRMED 2026-07-13 regeneration — see
// docs/hardware-notes.md §60m regional finding) triggered spurious
// "not part of any bank this radio supports" notes here, because the
// ft710 driver's static OFFLINE Capabilities
// (ft710.New(ft710.RealHardware).Capabilities(), what cmdImport
// validates against per task-13 brief §2) never asserts a 60m/EMG bank
// at all — it is discovered per session, not static
// (core/driver/ft710/caps.go). Before the amendment this genuinely
// spurious noise blocked exit 0 for every 60m-region radio's files; the
// controller resolved it by making offline validation advisory (printed
// under a clear "offline validation notes" label) and reserving
// authoritative gating for write time against the live radio. The
// default image no longer has any 60m channels (Stuart's real UK
// FT-710 has none), so this exact fixture now prints "none." under that
// label instead of spurious notes — the advisory MECHANISM this test
// pins (the label always appears, gating never happens) is unchanged;
// only the specific content it happens to report has gone quiet.
func TestBlackbox_ExportImportRoundTrip(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base.json")
	csvOut := filepath.Join(dir, "roundtrip.csv")
	back := filepath.Join(dir, "back.json")

	rRead := runBinary(t, "", "read", "--fake", "--out", base)
	if rRead.exitCode != exitSuccess {
		t.Fatalf("read --fake --out: exit code = %d, want exitSuccess (%d); stderr=%q", rRead.exitCode, exitSuccess, rRead.stderr)
	}

	rExport := runBinary(t, "", "export", "--csv", csvOut, base)
	if rExport.exitCode != exitSuccess {
		t.Fatalf("export --csv: exit code = %d, want exitSuccess (%d); stderr=%q", rExport.exitCode, exitSuccess, rExport.stderr)
	}

	rImport := runBinary(t, "", "import", "--csv", csvOut, "--into", base, "--out", back)
	if rImport.exitCode != exitSuccess {
		t.Fatalf("import --csv: exit code = %d, want exitSuccess (%d) — offline validation is advisory only; stdout=%q stderr=%q", rImport.exitCode, exitSuccess, rImport.stdout, rImport.stderr)
	}
	if !strings.Contains(rImport.stdout, "offline validation notes") {
		t.Errorf("import --csv: stdout = %q, want the advisory label \"offline validation notes\" (the spurious 60m-bank notes should still appear, just non-gating)", rImport.stdout)
	}

	baseCp, err := codeplug.Load(base)
	if err != nil {
		t.Fatalf("codeplug.Load(%s): %v", base, err)
	}
	backCp, err := codeplug.Load(back)
	if err != nil {
		t.Fatalf("codeplug.Load(%s): %v", back, err)
	}
	if got, want := codeplug.Digest(backCp.Channels), codeplug.Digest(baseCp.Channels); got != want {
		t.Errorf("round trip: back.json channels digest = %s, want %s (base.json's, byte-identical channels)", got, want)
	}
}

// mutateForWrite loads baselinePath, applies task-14 brief §2's exact
// happy-path example — "change a tag + add a channel" (a Modified entry
// at "001", an Added entry at the empty "002") — and saves the result to
// mutatedPath. Deliberately narrower than blackbox's own mutateForDiff
// (no erase): write's happy-path test wants an exact, predictable
// Written/Verified == 2.
func mutateForWrite(t *testing.T, baselinePath, mutatedPath string) {
	t.Helper()
	cp, err := codeplug.Load(baselinePath)
	if err != nil {
		t.Fatalf("codeplug.Load(%s): %v", baselinePath, err)
	}

	bySlot := make(map[string]int, len(cp.Channels))
	for i, ch := range cp.Channels {
		bySlot[ch.Slot] = i
	}

	idx001, ok := bySlot["001"]
	if !ok || cp.Channels[idx001].Data == nil {
		t.Fatal("fixture assumption broken: slot 001 must be present and populated")
	}
	modified := *cp.Channels[idx001].Data
	modified.Tag = "MUTATED"
	cp.Channels[idx001].Data = &modified

	idx002, ok := bySlot["002"]
	if !ok || cp.Channels[idx002].Data != nil {
		t.Fatal("fixture assumption broken: slot 002 must be present and empty")
	}
	added := modified
	added.Tag = "ADDEDCH"
	cp.Channels[idx002].Data = &added

	if err := codeplug.Save(mutatedPath, cp); err != nil {
		t.Fatalf("codeplug.Save(%s): %v", mutatedPath, err)
	}
}

// mutateForWriteEraseOnly loads baselinePath and erases (nils) slot P1L
// ONLY — no Added/Modified entries at all — producing a "blocked-only"
// plan (task-25 brief, adjudicated remedy for the reported "i don't seem
// to be able to send deletes to the radio" defect): countSendable == 0
// (an erase never counts as sendable) but diff.Blocked > 0 (the erase IS
// pending, just unsupported), which must be reported as exitBlocked, not
// the plain "Nothing to send." exitSuccess mutateForWrite's own baseline
// self-diff exercises below.
func mutateForWriteEraseOnly(t *testing.T, baselinePath, mutatedPath string) {
	t.Helper()
	cp, err := codeplug.Load(baselinePath)
	if err != nil {
		t.Fatalf("codeplug.Load(%s): %v", baselinePath, err)
	}

	found := false
	for i, ch := range cp.Channels {
		if ch.Slot == "P1L" {
			if ch.Data == nil {
				t.Fatal("fixture assumption broken: slot P1L must be present and populated")
			}
			cp.Channels[i].Data = nil
			found = true
		}
	}
	if !found {
		t.Fatal("fixture assumption broken: slot P1L must exist")
	}

	if err := codeplug.Save(mutatedPath, cp); err != nil {
		t.Fatalf("codeplug.Save(%s): %v", mutatedPath, err)
	}
}

// buildInvalidTagFile loads baselinePath and sets slot "001"'s tag to
// contain ';' — codeplug.Validate's validTagByte excludes it (a
// command-injection defence: ';' would let a tag terminate an MT frame
// early), so this deterministically produces a SeverityError Issue
// (task-14 brief §2's own suggested example) without needing to know
// any other validation rule.
func buildInvalidTagFile(t *testing.T, baselinePath, outPath string) {
	t.Helper()
	cp, err := codeplug.Load(baselinePath)
	if err != nil {
		t.Fatalf("codeplug.Load(%s): %v", baselinePath, err)
	}
	for i, ch := range cp.Channels {
		if ch.Slot == "001" && ch.Data != nil {
			bad := *ch.Data
			bad.Tag = "BAD;TAG"
			cp.Channels[i].Data = &bad
			if err := codeplug.Save(outPath, cp); err != nil {
				t.Fatalf("codeplug.Save(%s): %v", outPath, err)
			}
			return
		}
	}
	t.Fatal("fixture assumption broken: slot 001 must be present and populated")
}

// TestBlackbox_Write pins task-14 brief §2's black-box "write" contract.
// A single shared "read --fake --out" seeds baseline.json (one ReadAll);
// every subtest below reuses it (or a cheap in-memory mutation of it) —
// --fake's factory image (fakeradio.ImageUK) is deterministic, so a
// baseline read by one process invocation still matches a LATER, freshly
// constructed --fake radio in a different process (no cross-process
// persistence is needed or assumed here: each subtest's own "write"
// PrepareSend does its own fresh ReadAll against a brand new radio that
// merely happens to start from the identical factory state). The
// genuine round-trip proof (that a write's changes actually persist and
// read back byte-identically) is NOT attempted here — see this task's
// report for why that specific assertion is covered in-process instead
// (fakeradio.Radio has no persistence across separate OS processes, so
// "write --fake" then "diff --fake" as two separate runBinary calls can
// never observe each other's radio state).
func TestBlackbox_Write(t *testing.T) {
	dir := t.TempDir()
	baseline := filepath.Join(dir, "write-baseline.json")

	rSetup := runBinary(t, "", "read", "--fake", "--out", baseline)
	if rSetup.exitCode != exitSuccess {
		t.Fatalf("setup read --fake --out: exit code = %d, want exitSuccess (%d); stderr=%q", rSetup.exitCode, exitSuccess, rSetup.stderr)
	}

	mutated := filepath.Join(dir, "write-mutated.json")
	mutateForWrite(t, baseline, mutated)

	t.Run("happy path", func(t *testing.T) {
		r := runBinary(t, "", "write", "--fake", "--yes", "--firmware", "V01-10", mutated)
		if r.exitCode != exitSuccess {
			t.Fatalf("write --fake --yes --firmware: exit code = %d, want exitSuccess (%d); stdout=%q stderr=%q", r.exitCode, exitSuccess, r.stdout, r.stderr)
		}
		for _, want := range []string{
			"Added:", "Modified:", // the plan render (step 4, reusing writeDiffReport)
			"Snapshot:", "Baseline digest:",
			"Written:        2", "Verified:       2", "SkippedBlocked: 0",
			"Journal:",
		} {
			if !strings.Contains(r.stdout, want) {
				t.Errorf("write (happy path) stdout = %q, want it to contain %q", r.stdout, want)
			}
		}
		if !strings.Contains(r.stderr, "read ") {
			t.Errorf("write (happy path) stderr = %q, want progress lines (phase \"read\")", r.stderr)
		}
	})

	t.Run("nothing to send", func(t *testing.T) {
		// No --yes: task-14 brief §2 requires this to exit 0 WITHOUT ever
		// prompting, since the nothing-to-send short-circuit (step 5)
		// happens before the confirmation gate (step 6).
		r := runBinary(t, "", "write", "--fake", baseline)
		if r.exitCode != exitSuccess {
			t.Fatalf("write --fake (nothing to send): exit code = %d, want exitSuccess (%d); stdout=%q stderr=%q", r.exitCode, exitSuccess, r.stdout, r.stderr)
		}
		if !strings.Contains(r.stdout, "Nothing to send.") {
			t.Errorf("write --fake (nothing to send) stdout = %q, want %q", r.stdout, "Nothing to send.")
		}
		if strings.Contains(r.stderr, "Type \"yes\"") {
			t.Errorf("write --fake (nothing to send) stderr = %q, want NO confirmation prompt", r.stderr)
		}
	})

	t.Run("blocked-only (erase) — exitBlocked, never a false \"Nothing to send\"", func(t *testing.T) {
		// No --yes: the blocked-only short-circuit, exactly like "nothing
		// to send" above, happens before the confirmation gate — but this
		// one has a genuinely pending change (an erase) that simply
		// cannot be honoured, so it must NOT exit 0 or print the plain
		// "Nothing to send." (task-25 brief: that message is a false
		// parity claim — the working copy does NOT match the radio here).
		eraseOnly := filepath.Join(dir, "write-erase-only.json")
		mutateForWriteEraseOnly(t, baseline, eraseOnly)

		r := runBinary(t, "", "write", "--fake", eraseOnly)
		if r.exitCode != exitBlocked {
			t.Fatalf("write --fake (blocked-only erase): exit code = %d, want exitBlocked (%d); stdout=%q stderr=%q", r.exitCode, exitBlocked, r.stdout, r.stderr)
		}
		for _, want := range []string{
			"P1L",                               // names the blocked slot
			"erase not supported on this radio", // names the reason
			"[V/M]", "[ERASE]",                  // the front-panel procedure
		} {
			if !strings.Contains(r.stdout, want) {
				t.Errorf("write --fake (blocked-only erase) stdout = %q, want it to contain %q", r.stdout, want)
			}
		}
		if strings.Contains(r.stdout, "Nothing to send.") {
			t.Errorf("write --fake (blocked-only erase) stdout = %q, want it NOT to contain the plain \"Nothing to send.\" (false parity claim)", r.stdout)
		}
		if strings.Contains(r.stderr, "Type \"yes\"") {
			t.Errorf("write --fake (blocked-only erase) stderr = %q, want NO confirmation prompt", r.stderr)
		}
	})

	t.Run("validation blocked", func(t *testing.T) {
		invalid := filepath.Join(dir, "write-invalid.json")
		buildInvalidTagFile(t, baseline, invalid)

		r := runBinary(t, "", "write", "--fake", invalid)
		if r.exitCode != exitBlocked {
			t.Fatalf("write --fake (invalid tag): exit code = %d, want exitBlocked (%d); stdout=%q stderr=%q", r.exitCode, exitBlocked, r.stdout, r.stderr)
		}
		if !strings.Contains(r.stderr, "M-01") || !strings.Contains(r.stderr, "tag") {
			t.Errorf("write --fake (invalid tag) stderr = %q, want the slot and field named", r.stderr)
		}
	})

	t.Run("piped stdin without yes (non-TTY path, not a decline)", func(t *testing.T) {
		// Piped stdin is always non-TTY (a black-box invocation's stdin is
		// always an os/exec pipe, never a character device — see
		// runBinaryStdin's doc comment), so this NEVER reaches the
		// confirmation prompt at all, regardless of what is piped in
		// ("no\n" here is otherwise-unused filler, chosen only so this
		// test cannot be confused with an empty-stdin case). This proves
		// exactly ONE task-14 brief §2 bullet — "Non-TTY without --yes" —
		// exit 2 (exitUsage). It does NOT exercise interactive decline
		// (Fix 7, adjudicated LOW, Codex M4 #7 — a piped "no\n" is never
		// actually read here); that is TestResolveConfirmation's
		// "tty, declined, cancelled" case (write_test.go), the only layer
		// that can inject isTTY=true without a real terminal.
		r := runBinary(t, "no\n", "write", "--fake", mutated)
		if r.exitCode != exitUsage {
			t.Errorf("write --fake (non-tty, no --yes) = %d, want exitUsage (%d); stdout=%q stderr=%q", r.exitCode, exitUsage, r.stdout, r.stderr)
		}
		if !strings.Contains(r.stderr, "--yes") {
			t.Errorf("write --fake (non-tty, no --yes) stderr = %q, want it to mention --yes", r.stderr)
		}
	})

	t.Run("real /dev/null stdin without yes (Fix 4 quirk, still exit 2)", func(t *testing.T) {
		// Fix 4 (adjudicated MEDIUM, Codex M4 #4): /dev/null IS itself a
		// character device on Unix, so isStdinTTY (write.go) — the
		// established, dependency-free os.ModeCharDevice idiom — actually
		// classifies `write --fake </dev/null` as INTERACTIVE, unlike the
		// piped-stdin case above. Without the EOF-based reclassification
		// in resolveConfirmation, this would reach the confirmation
		// prompt, read EOF, and be treated as a declined confirmation
		// (exit 4) instead of the contracted non-interactive exit 2. This
		// is the one black-box test in this package that needs a REAL
		// character-device stdin — runBinaryStdin (main_test.go) exists
		// precisely so this can pass an *os.File instead of runBinary's
		// pipe-backed string stdin.
		devNull, err := os.Open(os.DevNull)
		if err != nil {
			t.Skipf("cannot open %s in this environment: %v", os.DevNull, err)
		}
		defer devNull.Close()

		r := runBinaryStdin(t, devNull, "write", "--fake", mutated)
		if r.exitCode != exitUsage {
			t.Errorf("write --fake </dev/null (no --yes) = %d, want exitUsage (%d); stdout=%q stderr=%q", r.exitCode, exitUsage, r.stdout, r.stderr)
		}
		if !strings.Contains(r.stderr, "--yes") {
			t.Errorf("write --fake </dev/null (no --yes) stderr = %q, want it to mention --yes", r.stderr)
		}
	})

	t.Run("missing firmware", func(t *testing.T) {
		r := runBinary(t, "", "write", "--fake", "--yes", mutated)
		if r.exitCode != exitRefused {
			t.Errorf("write --fake --yes (no --firmware) = %d, want exitRefused (%d); stdout=%q stderr=%q", r.exitCode, exitRefused, r.stdout, r.stderr)
		}
		if !strings.Contains(r.stderr, "front panel") && !strings.Contains(r.stderr, "SD-card") {
			t.Errorf("write --fake --yes (no --firmware) stderr = %q, want front-panel/SD-card guidance", r.stderr)
		}
		if !strings.Contains(r.stderr, "--firmware") {
			t.Errorf("write --fake --yes (no --firmware) stderr = %q, want it to mention --firmware", r.stderr)
		}
	})
}
