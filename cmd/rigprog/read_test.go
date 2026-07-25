// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver/ft710"
	"github.com/gm5dna/open-rig-programmer/internal/fakeradio"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// TestApplyDefaultGenerator_FillsEmpty pins task-12 brief §1: the CLI
// sets Codeplug.Generator IF AND ONLY IF the service left it empty, using
// a stable "rigprog/" prefix.
func TestApplyDefaultGenerator_FillsEmpty(t *testing.T) {
	cp := &codeplug.Codeplug{Generator: ""}
	applyDefaultGenerator(cp)
	if cp.Generator == "" {
		t.Fatal("applyDefaultGenerator left Generator empty, want it filled")
	}
	const wantPrefix = "rigprog/"
	if len(cp.Generator) < len(wantPrefix) || cp.Generator[:len(wantPrefix)] != wantPrefix {
		t.Errorf("applyDefaultGenerator: Generator = %q, want prefix %q", cp.Generator, wantPrefix)
	}
}

// TestApplyDefaultGenerator_LeavesNonEmpty pins the "if and only if
// empty" half: an already-populated Generator (which is what
// clone.Service.ReadAll always sets today — see cmd/rigprog read.go's
// doc comment) must never be overwritten.
func TestApplyDefaultGenerator_LeavesNonEmpty(t *testing.T) {
	cp := &codeplug.Codeplug{Generator: "open-rig-programmer/core/clone"}
	applyDefaultGenerator(cp)
	if cp.Generator != "open-rig-programmer/core/clone" {
		t.Errorf("applyDefaultGenerator overwrote a non-empty Generator: got %q", cp.Generator)
	}
}

// TestWriteReadSummary pins the stdout success summary's content (task-12
// brief §1): slots read, populated count, region, truncated baseline
// digest, output path.
func TestWriteReadSummary(t *testing.T) {
	cp := &codeplug.Codeplug{
		Channels: []codeplug.Channel{
			{Slot: "001", Data: &codeplug.ChannelData{FreqHz: 7_000_000}},
			{Slot: "002"}, // empty
			{Slot: "003", Data: &codeplug.ChannelData{FreqHz: 14_000_000}},
		},
		Radio: codeplug.RadioInfo{
			Region:         "UK",
			BaselineDigest: "0123456789abcdef0123456789abcdef",
		},
	}
	var buf bytes.Buffer
	writeReadSummary(&buf, cp, "/tmp/out.json")
	out := buf.String()

	for _, want := range []string{"3", "2", "UK", "0123456789ab (truncated)", "/tmp/out.json"} {
		if !strings.Contains(out, want) {
			t.Errorf("writeReadSummary output = %q, want it to contain %q", out, want)
		}
	}
	if strings.Contains(out, "0123456789abcdef0123456789abcdef") {
		t.Errorf("writeReadSummary output = %q, want the FULL digest not to appear (only the truncated form)", out)
	}
}

// TestCmdRead_MissingOut / NeitherPortNorFake / BothPortAndFake pin
// cmdRead's usage-error paths — all fast, no radio touched.
func TestCmdRead_MissingOut(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdRead(testCtx(t), []string{"--fake"}, &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdRead(--fake, no --out) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
}

func TestCmdRead_NeitherPortNorFake(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdRead(testCtx(t), []string{"--out", "f.json"}, &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdRead(no --port/--fake) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
}

func TestCmdRead_BothPortAndFake(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdRead(testCtx(t), []string{"--port", "/dev/cu.fake", "--fake", "--out", "f.json"}, &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdRead(both) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
}

// TestCmdRead_UnknownModel pins task 40's --model validation for read:
// an unrecognised model exits 2 (usage), naming FT-710, and never
// touches the filesystem (checked here via --out's absence, proving the
// rejection happened before checkOverwrite/snapshot-dir creation).
func TestCmdRead_UnknownModel(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "unknown-model.json")

	var stdout, stderr bytes.Buffer
	got := cmdRead(testCtx(t), []string{"--fake", "--out", out, "--model", "FTdx10"}, &stdout, &stderr)
	if got != exitUsage {
		t.Fatalf("cmdRead(--model FTdx10) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
	if !strings.Contains(stderr.String(), "FTdx10") || !strings.Contains(stderr.String(), "FT-710") {
		t.Errorf("cmdRead(--model FTdx10) stderr = %q, want it to name both the rejected and supported model", stderr.String())
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("cmdRead(--model FTdx10): --out = %s, want it to not exist (rejected before any file I/O)", out)
	}
}

// TestCmdRead_FakeExplicitModel_ByteIdentical pins task 40's headline
// requirement: "--model FT-710" (the default, spelled out explicitly)
// must be byte-identical to no --model flag at all, against --fake. Two
// full ReadAlls (~10s at current fakeradio pacing) — comparable cost to
// blackbox_test.go's own TestBlackbox_Read.
func TestCmdRead_FakeExplicitModel_ByteIdentical(t *testing.T) {
	dir := t.TempDir()
	wantOut := filepath.Join(dir, "want.json")
	gotOut := filepath.Join(dir, "got.json")

	var wantStdout, wantStderr bytes.Buffer
	wantCode := cmdRead(testCtx(t), []string{"--fake", "--out", wantOut}, &wantStdout, &wantStderr)

	var gotStdout, gotStderr bytes.Buffer
	gotCode := cmdRead(testCtx(t), []string{"--fake", "--model", "FT-710", "--out", gotOut}, &gotStdout, &gotStderr)

	if gotCode != wantCode {
		t.Fatalf("exit code = %d, want %d (flag-absent)", gotCode, wantCode)
	}
	// Both runs mention their own (different) --out path, so compare the
	// summaries with that one expected difference substituted out.
	wantSummary := strings.ReplaceAll(wantStdout.String(), wantOut, "<OUT>")
	gotSummary := strings.ReplaceAll(gotStdout.String(), gotOut, "<OUT>")
	if gotSummary != wantSummary {
		t.Errorf("stdout = %q, want byte-identical (modulo --out path) to flag-absent %q", gotSummary, wantSummary)
	}
	if !strings.Contains(gotStderr.String(), "read ") || !strings.Contains(wantStderr.String(), "read ") {
		t.Fatalf("sanity check failed: expected progress lines missing (got=%q want=%q)", gotStderr.String(), wantStderr.String())
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

// TestCmdRead_BadPort_PreservesOriginalWiringWording pins Fix 7 (Codex
// M6 #7, LOW) for the read path too — see cmdProbe's identical test for
// the full rationale. Only the "no --force needed" plumbing differs
// here: a bad --port fails before any --out file is ever touched.
func TestCmdRead_BadPort_PreservesOriginalWiringWording(t *testing.T) {
	const port = "/dev/nonexistent-rigprog-test-port"
	_, _, wiringErr := openRealSession(testCtx(t), wiring.DefaultModel, port)
	if wiringErr == nil {
		t.Fatal("openRealSession: expected an error opening a nonexistent port, got nil")
	}

	dir := t.TempDir()
	out := filepath.Join(dir, "unreachable.json")
	var stdout, stderr bytes.Buffer
	got := cmdRead(testCtx(t), []string{"--port", port, "--out", out}, &stdout, &stderr)
	if got != exitError {
		t.Fatalf("cmdRead(bad port) = %d, want exitError (%d); stderr=%q", got, exitError, stderr.String())
	}
	want := fmt.Sprintf("rigprog read: %v\n", wiringErr)
	if stderr.String() != want {
		t.Errorf("cmdRead(bad port) stderr = %q, want %q", stderr.String(), want)
	}
}

// TestCmdRead_RefuseOverwrite pins task-12 brief §1's refuse-overwrite
// rule: an existing --out without --force refuses BEFORE the radio is
// ever touched (exit 1, clear message, file left untouched) — this test
// costs no ReadAll at all, since the check happens ahead of session
// open.
func TestCmdRead_RefuseOverwrite(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "existing.json")
	const sentinel = "not a codeplug file"
	if err := os.WriteFile(out, []byte(sentinel), 0o600); err != nil {
		t.Fatalf("seeding existing --out file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	got := cmdRead(testCtx(t), []string{"--fake", "--out", out}, &stdout, &stderr)
	if got != exitError {
		t.Errorf("cmdRead(existing --out, no --force) = %d, want exitError (%d); stderr=%q", got, exitError, stderr.String())
	}
	if !strings.Contains(stderr.String(), "already exists") {
		t.Errorf("cmdRead(existing --out) stderr = %q, want it to mention the file already exists", stderr.String())
	}
	contents, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("reading %s after refused read: %v", out, err)
	}
	if string(contents) != sentinel {
		t.Error("cmdRead(existing --out, no --force) overwrote the file as a side effect, want it untouched")
	}
}

// TestCmdRead_CancelledBeforeStart pins task-12 brief §1's Ctrl-C
// requirement's observable effect: a context already cancelled before
// cmdRead does anything radio-touching yields exit 1, a "cancelled"
// message, and writes no output file — without needing to simulate an
// actual OS signal delivery (newInterruptContext itself is a thin,
// already-tested wrapper around signal.NotifyContext).
func TestCmdRead_CancelledBeforeStart(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "cancelled.json")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var stdout, stderr bytes.Buffer
	got := cmdRead(ctx, []string{"--fake", "--out", out}, &stdout, &stderr)
	if got != exitError {
		t.Errorf("cmdRead(cancelled ctx) = %d, want exitError (%d); stderr=%q", got, exitError, stderr.String())
	}
	if !strings.Contains(stderr.String(), "cancelled") {
		t.Errorf("cmdRead(cancelled ctx) stderr = %q, want it to say \"cancelled\"", stderr.String())
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("cmdRead(cancelled ctx): --out = %s, want it to not exist (err=%v)", out, err)
	}
}

// TestWriteSettingsReadSummary pins --settings' stdout summary content
// (task-34 brief): "Settings read: N" (Known entries) and "Settings
// unavailable: M" (Unavailable entries) — a fresh ReadSettings result
// only ever produces those two states (MenuUnsupported exists solely for
// MergeMenuSnapshots' carry-forward job), so this fixture covers both.
func TestWriteSettingsReadSummary(t *testing.T) {
	snap := &codeplug.MenuSnapshot{
		Entries: []codeplug.MenuEntry{
			{ID: "010101", Value: "042", State: codeplug.MenuKnown},
			{ID: "010102", Value: "050", State: codeplug.MenuKnown},
			{ID: "010103", State: codeplug.MenuUnavailable},
		},
	}
	var buf bytes.Buffer
	writeSettingsReadSummary(&buf, snap)
	out := buf.String()
	if !strings.Contains(out, "Settings read:") || !strings.Contains(out, "2") {
		t.Errorf("writeSettingsReadSummary output = %q, want a \"Settings read:\" line mentioning 2", out)
	}
	if !strings.Contains(out, "Settings unavailable:") || !strings.Contains(out, "1") {
		t.Errorf("writeSettingsReadSummary output = %q, want a \"Settings unavailable:\" line mentioning 1", out)
	}
}

// TestCmdRead_SettingsFailureLeavesOutUntouched pins task-34 brief's
// "never write half the artefact" rule: a settings-phase failure AFTER a
// successful channel ReadAll must leave --out entirely absent, exit 1 —
// cmdRead must not reach saveCodeplugNoClobber at all in this case.
//
// Exchange arithmetic (mirrors core/clone/settings_test.go's
// TestReadSettings_HardErrorAborts documented pattern): opening a
// --fake session against fakeradio's default (ImageUK) image costs 4
// exchanges (AI0, ID, MR501 rejected — no 60m, MREMG rejected — no EMG);
// the channel ReadAll that follows costs one MR per slot in the FT-710's
// static (Simulated-profile) capabilities, PLUS one further MT for every
// slot ImageUK marks Populated (an MR that succeeds always continues to
// MT — core/driver/ft710/read.go's ReadChannel) — computed here from
// ft710.CapabilitiesSimulated() and fakeradio.ImageUK() directly (not a
// hand-counted literal), so this stays correct if either ever changes.
// FaultDropReplies is scripted to drop replies starting EXACTLY ONE
// exchange past that total: the channel read completes in full — proving
// this is a genuinely GOOD channel read — and the settings phase's very
// first EX exchange gets no reply at all, forcing clone.ReadSettings to
// return an error (ErrTimeout, wrapped) after zero settings items.
func TestCmdRead_SettingsFailureLeavesOutUntouched(t *testing.T) {
	caps := ft710.CapabilitiesSimulated()
	totalSlots := 0
	for _, b := range caps.Banks {
		totalSlots += len(b.Slots)
	}
	populated := countPopulatedImage(fakeradio.ImageUK())
	const openExchanges = 4 // AI0, ID, MR501 (rejected), MREMG (rejected)
	failExchange := openExchanges + totalSlots + populated + 1

	prevOpts := wiring.FakeSessionOpts
	wiring.FakeSessionOpts = []fakeradio.Option{fakeradio.WithFault(fakeradio.FaultDropReplies(failExchange))}
	t.Cleanup(func() { wiring.FakeSessionOpts = prevOpts })

	dir := t.TempDir()
	out := filepath.Join(dir, "settings-fail.json")

	var stdout, stderr bytes.Buffer
	got := cmdRead(testCtx(t), []string{"--fake", "--settings", "--out", out}, &stdout, &stderr)
	if got != exitError {
		t.Fatalf("cmdRead(--settings, settings-phase failure) = %d, want exitError (%d); stdout=%q stderr=%q", got, exitError, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("cmdRead(--settings, settings-phase failure): --out = %s, want it to not exist (err=%v)", out, err)
	}
}
