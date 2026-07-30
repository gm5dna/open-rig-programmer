// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/internal/fakeradio"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// TestWriteDiffReport_NoChanges pins the "no changes" path (task-12
// brief §2/§3): an all-Unchanged DiffResult renders "No changes." plus a
// count line, with no Added/Modified/Erased section headings.
func TestWriteDiffReport_NoChanges(t *testing.T) {
	result := codeplug.DiffResult{
		Entries:   []codeplug.DiffEntry{{Slot: "001", Kind: codeplug.DiffUnchanged}},
		Unchanged: 1,
	}
	var buf bytes.Buffer
	writeDiffReport(&buf, result)
	out := buf.String()

	if !strings.Contains(out, "No changes.") {
		t.Errorf("writeDiffReport (no changes) = %q, want it to contain %q", out, "No changes.")
	}
	for _, unwanted := range []string{"Added:", "Modified:", "Erased:"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("writeDiffReport (no changes) = %q, want it NOT to contain %q", out, unwanted)
		}
	}
	if !strings.Contains(out, "Unchanged 1") {
		t.Errorf("writeDiffReport (no changes) = %q, want the count line to include %q", out, "Unchanged 1")
	}
}

// TestWriteDiffReport_AddedModifiedErased pins the grouped rendering
// (task-12 brief §2): Added/Modified/Erased sections, a terse
// before->after for Modified, and the pre-M5b "UNSUPPORTED" marking plus
// BlockReason for a Blocked Erased entry.
func TestWriteDiffReport_AddedModifiedErased(t *testing.T) {
	before := codeplug.ChannelData{FreqHz: 7_000_000, Mode: "LSB", Tag: "OLD", TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false}}
	after := codeplug.ChannelData{FreqHz: 7_010_000, Mode: "USB", Tag: "NEW", TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false}}
	added := codeplug.ChannelData{FreqHz: 14_000_000, Mode: "USB", Tag: "ADDED", TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false}}

	result := codeplug.DiffResult{
		Entries: []codeplug.DiffEntry{
			{Slot: "002", Bank: spec.BankMemory, Kind: codeplug.DiffAdded, After: &added},
			{Slot: "001", Bank: spec.BankMemory, Kind: codeplug.DiffModified, Before: &before, After: &after},
			{
				Slot: "P1L", Bank: spec.BankPMS, Kind: codeplug.DiffErased, Before: &before,
				Blocked: true, BlockReason: "erase not supported on this radio",
			},
		},
		Added: 1, Modified: 1, Erased: 1, Blocked: 1, Unchanged: 114,
	}
	var buf bytes.Buffer
	writeDiffReport(&buf, result)
	out := buf.String()

	for _, want := range []string{
		"Added:", "Modified:", "Erased:",
		"M-02", // DisplaySlot("002")
		"M-01", // DisplaySlot("001")
		"P1L",  // DisplaySlot leaves PMS slots unchanged
		"14000000", "USB", `"ADDED"`,
		"7000000", "7010000", "LSB", `"OLD"`, `"NEW"`,
		"UNSUPPORTED — slot will keep its current contents",
		"erase not supported on this radio",
		"Added 1, Modified 1, Erased 1, Blocked 1, Unchanged 114",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("writeDiffReport output = %q, want it to contain %q", out, want)
		}
	}
}

// TestCmdDiff_NeitherPortNorFake / BothPortAndFake pin the "--port XOR
// --fake" requirement, matching probe/read.
func TestCmdDiff_NeitherPortNorFake(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdDiff(testCtx(t), []string{"somefile.json"}, &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdDiff(no --port/--fake) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
}

func TestCmdDiff_BothPortAndFake(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdDiff(testCtx(t), []string{"--port", "/dev/cu.fake", "--fake", "somefile.json"}, &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdDiff(both) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
}

// TestCmdDiff_MissingFileArgument / TooManyArguments pin the "exactly
// one FILE argument" requirement.
func TestCmdDiff_MissingFileArgument(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdDiff(testCtx(t), []string{"--fake"}, &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdDiff(--fake, no FILE) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
}

func TestCmdDiff_TooManyArguments(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdDiff(testCtx(t), []string{"--fake", "a.json", "b.json"}, &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdDiff(--fake, 2 FILEs) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
}

// TestCmdDiff_LoadNonexistentFile pins a plain file-load failure: exit 1,
// no radio ever touched (fails before session open).
func TestCmdDiff_LoadNonexistentFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdDiff(testCtx(t), []string{"--fake", "/nonexistent/path/rigprog-test.json"}, &stdout, &stderr)
	if got != exitError {
		t.Errorf("cmdDiff(nonexistent file) = %d, want exitError (%d); stderr=%q", got, exitError, stderr.String())
	}
}

// TestCmdDiff_SchemaTooNew pins task-12 brief §2's distinct message for a
// schema-too-new file — also file-load-only, no radio touched.
func TestCmdDiff_SchemaTooNew(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/too-new.json"
	tooNew := &codeplug.Codeplug{Schema: codeplug.CurrentSchema + 1}
	if err := codeplug.Save(path, tooNew); err != nil {
		t.Fatalf("Save fixture: %v", err)
	}

	var stdout, stderr bytes.Buffer
	got := cmdDiff(testCtx(t), []string{"--fake", path}, &stdout, &stderr)
	if got != exitError {
		t.Errorf("cmdDiff(schema too new) = %d, want exitError (%d); stderr=%q", got, exitError, stderr.String())
	}
	if !strings.Contains(stderr.String(), "newer") {
		t.Errorf("cmdDiff(schema too new) stderr = %q, want it to mention the file is newer than supported", stderr.String())
	}
}

// TestCmdDiff_CancelledBeforeStart mirrors
// TestCmdRead_CancelledBeforeStart: a context already cancelled before
// cmdDiff opens a session yields exit 1 and a "cancelled" message. Uses a
// minimal-but-valid fixture file so the cancellation is what's under
// test, not the load step.
func TestCmdDiff_CancelledBeforeStart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.json")
	fixture := &codeplug.Codeplug{Schema: codeplug.CurrentSchema}
	if err := codeplug.Save(path, fixture); err != nil {
		t.Fatalf("Save fixture: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var stdout, stderr bytes.Buffer
	got := cmdDiff(ctx, []string{"--fake", path}, &stdout, &stderr)
	if got != exitError {
		t.Errorf("cmdDiff(cancelled ctx) = %d, want exitError (%d); stderr=%q", got, exitError, stderr.String())
	}
	if !strings.Contains(stderr.String(), "cancelled") {
		t.Errorf("cmdDiff(cancelled ctx) stderr = %q, want it to say \"cancelled\"", stderr.String())
	}
}

// buildUKShapedCodeplug returns a Codeplug whose Channels carry MEM
// "001".."099", PMS "P1L".."P9U", plus a 60m "501".."507" set — an
// ARBITRARY, differently-shaped inventory from ImageUS's (15x60m+EMG),
// used purely to exercise codeplug.Diff's inventory-mismatch check
// below. It is named/shaped after the FORMER ImageUK factory image
// (before its HW-CONFIRMED 2026-07-13 regeneration — see
// docs/hardware-notes.md §60m regional finding — which found Stuart's
// real UK FT-710 has NO 5xx bank at all): this is now a synthetic
// fixture, NOT what a real "--fake" (default ImageUK) ReadAll produces
// any more, but it still serves its original purpose here (any
// consistently-shaped mismatch against ImageUS would do). Only Slot
// names matter for this file's purpose; Data is left nil throughout, so
// building this costs no radio I/O at all.
func buildUKShapedCodeplug() *codeplug.Codeplug {
	var channels []codeplug.Channel
	for n := 1; n <= 99; n++ {
		channels = append(channels, codeplug.Channel{Slot: fmt.Sprintf("%03d", n)})
	}
	for pair := 1; pair <= 9; pair++ {
		channels = append(channels,
			codeplug.Channel{Slot: fmt.Sprintf("P%dL", pair)},
			codeplug.Channel{Slot: fmt.Sprintf("P%dU", pair)},
		)
	}
	for n := 1; n <= 7; n++ {
		channels = append(channels, codeplug.Channel{Slot: fmt.Sprintf("5%02d", n)})
	}
	return &codeplug.Codeplug{Schema: codeplug.CurrentSchema, Channels: channels}
}

// TestCmdDiff_InventoryMismatch pins task-12 brief §3's inventory-
// mismatch error path: the buildUKShapedCodeplug candidate file (7 x
// 60m, no EMG — see its doc comment) diffed against a US-image
// fakeradio (15 x 60m + EMG) reports the mismatch plainly and exits 1.
//
// This drives cmdDiff through the SAME code path a real "rigprog diff
// --fake" invocation uses, via wiring.FakeSessionOpts — a minimal,
// test-only seam on the wiring constructor (task-12 brief §3: "if Task
// 11's wiring does not expose an image hook for tests, add a minimal
// test-only seam ... the flag parser must never populate them for real
// use"), moved to internal/wiring by task-15's extraction. See
// internal/wiring/fake.go's doc comment on FakeSessionOpts: no flag
// exists that can set it, and TestSimulatedProfileTokensConfinement
// (internal/guards) keeps passing unchanged because this seam does not
// add a second ft710.Simulated reference to any non-test file.
func TestCmdDiff_InventoryMismatch(t *testing.T) {
	candidatePath := filepath.Join(t.TempDir(), "uk-shaped.json")
	if err := codeplug.Save(candidatePath, buildUKShapedCodeplug()); err != nil {
		t.Fatalf("Save fixture: %v", err)
	}

	prevOpts := wiring.FakeSessionOpts
	wiring.FakeSessionOpts = []fakeradio.Option{fakeradio.WithFactoryImage(fakeradio.ImageUS)}
	t.Cleanup(func() { wiring.FakeSessionOpts = prevOpts })

	var stdout, stderr bytes.Buffer
	got := cmdDiff(testCtx(t), []string{"--fake", candidatePath}, &stdout, &stderr)
	if got != exitError {
		t.Errorf("cmdDiff(UK file vs US fake) = %d, want exitError (%d); stdout=%q stderr=%q", got, exitError, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "different slot inventories") {
		t.Errorf("cmdDiff(UK file vs US fake) stderr = %q, want it to report different slot inventories", stderr.String())
	}
}

// TestCmdDiff_UnknownModel pins task 40's --model validation for diff: an
// unrecognised model exits 2 (usage), naming FT-710, before the candidate
// file is even loaded (a nonexistent FILE path is used deliberately, to
// prove rejection happens before any file I/O is attempted).
func TestCmdDiff_UnknownModel(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdDiff(testCtx(t), []string{"--fake", "--model", unknownModelSentinel, "/nonexistent/rigprog-test.json"}, &stdout, &stderr)
	if got != exitUsage {
		t.Fatalf("cmdDiff(--model %s) = %d, want exitUsage (%d); stderr=%q", unknownModelSentinel, got, exitUsage, stderr.String())
	}
	if !strings.Contains(stderr.String(), unknownModelSentinel) || !strings.Contains(stderr.String(), "FT-710") {
		t.Errorf("cmdDiff(--model %s) stderr = %q, want it to name both the rejected and supported model", unknownModelSentinel, stderr.String())
	}
}

// TestCmdDiff_FakeExplicitModel_ByteIdentical pins task 40's headline
// requirement: "--model FT-710" (the default, spelled out explicitly)
// must be byte-identical to no --model flag at all, against --fake and a
// self-diffed ("no changes") candidate file.
func TestCmdDiff_FakeExplicitModel_ByteIdentical(t *testing.T) {
	dir := t.TempDir()
	baseline := filepath.Join(dir, "diff-baseline.json")

	var setupStdout, setupStderr bytes.Buffer
	setupCode := cmdRead(testCtx(t), []string{"--fake", "--out", baseline}, &setupStdout, &setupStderr)
	if setupCode != exitSuccess {
		t.Fatalf("setup cmdRead(--fake --out): exit code = %d, want exitSuccess (%d); stderr=%q", setupCode, exitSuccess, setupStderr.String())
	}

	var wantStdout, wantStderr bytes.Buffer
	wantCode := cmdDiff(testCtx(t), []string{"--fake", baseline}, &wantStdout, &wantStderr)

	var gotStdout, gotStderr bytes.Buffer
	gotCode := cmdDiff(testCtx(t), []string{"--fake", "--model", "FT-710", baseline}, &gotStdout, &gotStderr)

	if gotCode != wantCode {
		t.Errorf("exit code = %d, want %d (flag-absent)", gotCode, wantCode)
	}
	if gotStdout.String() != wantStdout.String() {
		t.Errorf("stdout = %q, want byte-identical to flag-absent %q", gotStdout.String(), wantStdout.String())
	}
	if gotStderr.String() != wantStderr.String() {
		t.Errorf("stderr = %q, want byte-identical to flag-absent %q", gotStderr.String(), wantStderr.String())
	}
}

// TestCmdDiff_Help pins "rigprog diff -h".
func TestCmdDiff_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdDiff(testCtx(t), []string{"-h"}, &stdout, &stderr)
	if got != exitSuccess {
		t.Errorf("cmdDiff([-h]) = %d, want exitSuccess (%d)", got, exitSuccess)
	}
	if stdout.Len() == 0 {
		t.Error("cmdDiff([-h]): stdout is empty, want usage text")
	}
}
