// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/internal/fakeradio"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// TestWriteProbeReport_ImageUK exercises writeProbeReport against the
// default fakeradio image via the production wiring constructor
// openFakeSession. HW-CONFIRMED 2026-07-13 (docs/hardware-notes.md §60m
// regional finding): ImageUK no longer synthesises a 60m/EMG bank —
// Stuart's real UK FT-710 has neither.
func TestWriteProbeReport_ImageUK(t *testing.T) {
	sess, closeAll, err := openFakeSession(testCtx(t), wiring.DefaultModel)
	if err != nil {
		t.Fatalf("openFakeSession: %v", err)
	}
	t.Cleanup(func() { _ = closeAll() })

	var stdout, stderr bytes.Buffer
	writeProbeReport(&stdout, &stderr, wiring.DefaultModel, sess)

	out := stdout.String()
	for _, want := range []string{"FT-710", "0800", "no-60m", "60 m channels: 0", "EMG channel:   no"} {
		if !strings.Contains(out, want) {
			t.Errorf("writeProbeReport stdout = %q, want it to contain %q", out, want)
		}
	}
	if stderr.Len() != 0 {
		t.Errorf("writeProbeReport stderr = %q, want empty (no unexpected frames on a fresh fake session)", stderr.String())
	}
}

// TestWriteProbeReport_ImageUS exercises writeProbeReport against a
// fakeradio built with ImageUS (15 x 60m + EMG) — task-11 brief §6's
// in-process detail check. Built via internal/wiring's own
// FakeSessionOpts test seam (task 40: this file no longer imports
// core/driver/ft710 directly — wiring.OpenFakeSessionFor is the SAME
// production code path openFakeSession itself uses, just with a
// non-default factory image layered on top for this one call) rather
// than hand-constructing an ft710.Session, since openFakeSession itself
// always uses the default (UK) image.
func TestWriteProbeReport_ImageUS(t *testing.T) {
	prevOpts := wiring.FakeSessionOpts
	wiring.FakeSessionOpts = []fakeradio.Option{fakeradio.WithFactoryImage(fakeradio.ImageUS)}
	t.Cleanup(func() { wiring.FakeSessionOpts = prevOpts })

	sess, closeAll, err := wiring.OpenFakeSessionFor(testCtx(t), wiring.DefaultModel)
	if err != nil {
		t.Fatalf("OpenFakeSessionFor (ImageUS): %v", err)
	}
	t.Cleanup(func() { _ = closeAll() })

	var stdout, stderr bytes.Buffer
	writeProbeReport(&stdout, &stderr, wiring.DefaultModel, sess)

	out := stdout.String()
	for _, want := range []string{"FT-710", "0800", "US", "60 m channels: 15", "EMG channel:   yes"} {
		if !strings.Contains(out, want) {
			t.Errorf("writeProbeReport stdout = %q, want it to contain %q", out, want)
		}
	}
}

// TestCmdProbe_NeitherPortNorFake pins task-11 brief §3/§5: probe
// requires exactly one of --port or --fake; neither -> exitUsage.
func TestCmdProbe_NeitherPortNorFake(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdProbe(testCtx(t), nil, &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdProbe(nil) = %d, want exitUsage (%d)", got, exitUsage)
	}
}

// TestCmdProbe_BothPortAndFake pins the "both -> exit 2" half of the
// same requirement.
func TestCmdProbe_BothPortAndFake(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdProbe(testCtx(t), []string{"--port", "/dev/cu.fake", "--fake"}, &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdProbe([--port ... --fake]) = %d, want exitUsage (%d)", got, exitUsage)
	}
}

// TestCmdProbe_Fake drives cmdProbe end-to-end (in-process) with --fake,
// pinning task-11 brief §6's black-box assertions at the cmdProbe level:
// exit 0, output containing model FT-710, CAT ID 0800, and the default
// ImageUK inventory.
func TestCmdProbe_Fake(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdProbe(testCtx(t), []string{"--fake"}, &stdout, &stderr)
	if got != exitSuccess {
		t.Fatalf("cmdProbe([--fake]) = %d, want exitSuccess (%d); stderr=%q", got, exitSuccess, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"FT-710", "0800", "60 m channels: 0"} {
		if !strings.Contains(out, want) {
			t.Errorf("cmdProbe([--fake]) stdout = %q, want it to contain %q", out, want)
		}
	}
}

// TestCmdProbe_FakeExplicitModel pins task 40's headline requirement:
// "--model FT-710" (the default, spelled out explicitly) must be
// byte-identical to no --model flag at all.
func TestCmdProbe_FakeExplicitModel(t *testing.T) {
	var wantStdout, wantStderr bytes.Buffer
	wantCode := cmdProbe(testCtx(t), []string{"--fake"}, &wantStdout, &wantStderr)

	var gotStdout, gotStderr bytes.Buffer
	gotCode := cmdProbe(testCtx(t), []string{"--fake", "--model", "FT-710"}, &gotStdout, &gotStderr)

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

// TestCmdProbe_UnknownModel pins task 40's --model validation: an
// unrecognised model exits 2 (usage), naming the supported model(s), and
// never opens a session (no fakeradio invocation, no port touched).
func TestCmdProbe_UnknownModel(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdProbe(testCtx(t), []string{"--fake", "--model", unknownModelSentinel}, &stdout, &stderr)
	if got != exitUsage {
		t.Fatalf("cmdProbe(--model %s) = %d, want exitUsage (%d); stderr=%q", unknownModelSentinel, got, exitUsage, stderr.String())
	}
	if !strings.Contains(stderr.String(), unknownModelSentinel) || !strings.Contains(stderr.String(), "FT-710") {
		t.Errorf("cmdProbe(--model %s) stderr = %q, want it to name both the rejected and supported model", unknownModelSentinel, stderr.String())
	}
}

// TestWrongRadioMessage_NamesSelectedModel pins task 40's wrong-radio
// wording: the message names the SELECTED --model, not a hardcoded
// "FT-710" — a caller who asked for a specific model and got a
// different radio's CAT ID answer should be told what they asked for.
// Exercised directly against a synthetic *driver.WrongRadioError: no
// black-box or in-process path in this package can actually provoke
// Driver.Open's WrongRadioError without a real (or stub) serial port —
// --fake always answers as the correct model (see core/driver/ft710's
// own TestOpen_WrongRadio for the equivalent coverage one layer down).
func TestWrongRadioMessage_NamesSelectedModel(t *testing.T) {
	wr := &driver.WrongRadioError{Got: "0761", Want: "0800"}

	got := wrongRadioMessage("FT-710", wr)
	want := `rigprog probe: wrong radio: got CAT ID "0761", want "0800" — this port's radio does not identify as FT-710` + "\n"
	if got != want {
		t.Errorf("wrongRadioMessage(FT-710, ...) = %q, want %q", got, want)
	}

	// A different selected model — here one this build cannot open a
	// session against AT ALL (the never-registrable sentinel) — must be
	// named verbatim: the function is a pure string formatter, never
	// re-validating model itself.
	got2 := wrongRadioMessage(unknownModelSentinel, wr)
	if !strings.Contains(got2, unknownModelSentinel) {
		t.Errorf("wrongRadioMessage(%s, ...) = %q, want it to name %s", unknownModelSentinel, got2, unknownModelSentinel)
	}
	if strings.Contains(got2, "FT-710") {
		t.Errorf("wrongRadioMessage(%s, ...) = %q, want it NOT to mention FT-710", unknownModelSentinel, got2)
	}

	// M9d-2 (spec A5): when the driver could NAME the model it found —
	// the FTdx101 pair, whose two IDs belong to two models one driver
	// knows about — the diagnostic says which radio is on the port
	// instead of leaving the operator to decode a bare CAT ID. The
	// SELECTED --model is still what names the wanted side, exactly as
	// above: wrongRadioMessage never substitutes the error's own
	// WantModel for what the caller asked for.
	named := &driver.WrongRadioError{
		Got: "0682", Want: "0681",
		GotModel: "FTdx101MP", WantModel: "FTdx101D",
	}
	got3 := wrongRadioMessage("FTdx101D", named)
	want3 := `rigprog probe: wrong radio: radio identifies as FTdx101MP (CAT ID "0682"); you selected FTdx101D — this port's radio does not identify as FTdx101D` + "\n"
	if got3 != want3 {
		t.Errorf("wrongRadioMessage(FTdx101D, named) = %q, want %q", got3, want3)
	}
}

// TestCmdProbe_BadPort_PreservesOriginalWiringWording pins Fix 7 (Codex
// M6 #7, LOW): internal/wiring's extraction (task-15) changed this
// command's CLI-visible serial-open diagnostic from "cmd/rigprog: open
// serial port ...: ..." to internal/wiring's own generic "wiring: open
// serial port ...: ..." — the stated extraction contract was UNCHANGED
// user-facing wording. cmdProbe's stderr line for a serial-open failure
// must still be exactly "rigprog probe: " + openRealSession's own
// error string (constructed dynamically here, since the underlying OS
// error text is platform-dependent), and that error string must itself
// still start with the pre-extraction "cmd/rigprog: open serial port "
// wording, not "wiring: open serial port ".
func TestCmdProbe_BadPort_PreservesOriginalWiringWording(t *testing.T) {
	const port = "/dev/nonexistent-rigprog-test-port"
	_, _, wiringErr := openRealSession(testCtx(t), wiring.DefaultModel, port)
	if wiringErr == nil {
		t.Fatal("openRealSession: expected an error opening a nonexistent port, got nil")
	}

	const wantPrefix = "cmd/rigprog: open serial port "
	if !strings.HasPrefix(wiringErr.Error(), wantPrefix) {
		t.Errorf("openRealSession(bad port) error = %q, want it to start with the pre-extraction %q wording (not internal/wiring's own \"wiring: ...\" text)", wiringErr.Error(), wantPrefix)
	}

	var stdout, stderr bytes.Buffer
	got := cmdProbe(testCtx(t), []string{"--port", port}, &stdout, &stderr)
	if got != exitError {
		t.Fatalf("cmdProbe(bad port) = %d, want exitError (%d); stderr=%q", got, exitError, stderr.String())
	}
	want := fmt.Sprintf("rigprog probe: %v\n", wiringErr)
	if stderr.String() != want {
		t.Errorf("cmdProbe(bad port) stderr = %q, want %q", stderr.String(), want)
	}
}

// TestCmdProbe_Help pins the "rigprog probe -h" half of task-11 brief §1.
func TestCmdProbe_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdProbe(testCtx(t), []string{"-h"}, &stdout, &stderr)
	if got != exitSuccess {
		t.Errorf("cmdProbe([-h]) = %d, want exitSuccess (%d)", got, exitSuccess)
	}
	if stdout.Len() == 0 {
		t.Error("cmdProbe([-h]): stdout is empty, want usage text")
	}
	if stderr.Len() != 0 {
		t.Errorf("cmdProbe([-h]): stderr = %q, want empty for explicit help", stderr.String())
	}
}
