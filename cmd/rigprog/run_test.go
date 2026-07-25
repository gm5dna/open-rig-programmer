// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestRun_NoArgs pins "rigprog with no args -> usage text" (task-11
// brief §1/§6). Treated as a usage error (exitUsage): a bare invocation
// is missing the required subcommand argument.
func TestRun_NoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := run(nil, strings.NewReader(""), &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("run(nil) = %d, want exitUsage (%d)", got, exitUsage)
	}
	if !strings.Contains(stderr.String(), "ports") || !strings.Contains(stderr.String(), "probe") {
		t.Errorf("run(nil) stderr = %q, want it to list subcommands", stderr.String())
	}
}

// TestRun_UnknownSubcommand pins "unknown subcommand -> 2".
func TestRun_UnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := run([]string{"frobnicate"}, strings.NewReader(""), &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("run([frobnicate]) = %d, want exitUsage (%d)", got, exitUsage)
	}
	if !strings.Contains(stderr.String(), "frobnicate") {
		t.Errorf("run([frobnicate]) stderr = %q, want it to name the unknown subcommand", stderr.String())
	}
}

// TestRun_Help pins "rigprog help -> usage text, exit 0" (explicit help
// request, task-11 brief §1).
func TestRun_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := run([]string{"help"}, strings.NewReader(""), &stdout, &stderr)
	if got != exitSuccess {
		t.Errorf("run([help]) = %d, want exitSuccess (%d)", got, exitSuccess)
	}
	if stdout.Len() == 0 {
		t.Error("run([help]): stdout is empty, want usage text")
	}
	if stderr.Len() != 0 {
		t.Errorf("run([help]) stderr = %q, want empty for explicit help", stderr.String())
	}
}

// TestRun_PortsFakeRejected and TestRun_ProbeFake exercise ports/probe
// through the full run() dispatcher, confirming the wiring above reaches
// them correctly end-to-end in-process.
func TestRun_PortsFakeRejected(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := run([]string{"ports", "--fake"}, strings.NewReader(""), &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("run([ports --fake]) = %d, want exitUsage (%d)", got, exitUsage)
	}
}

func TestRun_ProbeFake(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := run([]string{"probe", "--fake"}, strings.NewReader(""), &stdout, &stderr)
	if got != exitSuccess {
		t.Fatalf("run([probe --fake]) = %d, want exitSuccess (%d); stderr=%q", got, exitSuccess, stderr.String())
	}
	if !strings.Contains(stdout.String(), "FT-710") {
		t.Errorf("run([probe --fake]) stdout = %q, want it to mention FT-710", stdout.String())
	}
}

// TestRun_ReadDispatches and TestRun_DiffDispatches confirm run() reaches
// cmdRead/cmdDiff (task-12 brief §1/§2) rather than the notImplemented
// stub — exercised here only via a fast, no-radio usage error, so this
// stays well clear of any ReadAll cost; the radio-touching paths are
// covered by cmdRead/cmdDiff's own in-process tests and the black-box
// tests.
func TestRun_ReadDispatches(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := run([]string{"read"}, strings.NewReader(""), &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("run([read]) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
	if strings.Contains(stderr.String(), "not implemented") {
		t.Errorf("run([read]) stderr = %q, want it NOT to say \"not implemented\"", stderr.String())
	}
}

func TestRun_DiffDispatches(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := run([]string{"diff"}, strings.NewReader(""), &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("run([diff]) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
	if strings.Contains(stderr.String(), "not implemented") {
		t.Errorf("run([diff]) stderr = %q, want it NOT to say \"not implemented\"", stderr.String())
	}
}

// TestRun_WriteDispatches confirms run() reaches cmdWrite (task-14 brief
// §1) rather than the (now-empty) notImplemented stub — exercised via a
// fast, no-radio usage error (missing FILE), matching
// TestRun_ReadDispatches/TestRun_DiffDispatches.
func TestRun_WriteDispatches(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := run([]string{"write", "--fake"}, strings.NewReader(""), &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("run([write --fake]) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
	if strings.Contains(stderr.String(), "not implemented") {
		t.Errorf("run([write --fake]) stderr = %q, want it NOT to say \"not implemented\"", stderr.String())
	}
}

// TestRun_ExportDispatches confirms run() reaches cmdExport (task-13
// brief §1) rather than the notImplemented stub — exercised via a fast,
// no-radio usage error (export is OFFLINE, so there is no ReadAll cost to
// avoid here, unlike read/diff above).
func TestRun_ExportDispatches(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := run([]string{"export"}, strings.NewReader(""), &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("run([export]) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
	if strings.Contains(stderr.String(), "not implemented") {
		t.Errorf("run([export]) stderr = %q, want it NOT to say \"not implemented\"", stderr.String())
	}
}

// TestRun_ImportDispatches confirms run() reaches cmdImport (task-13
// brief §2) rather than the notImplemented stub — likewise OFFLINE, fast,
// no-radio usage error.
func TestRun_ImportDispatches(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := run([]string{"import"}, strings.NewReader(""), &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("run([import]) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
	if strings.Contains(stderr.String(), "not implemented") {
		t.Errorf("run([import]) stderr = %q, want it NOT to say \"not implemented\"", stderr.String())
	}
}

// TestRun_SettingsDispatches confirms run() reaches cmdSettings (task-34
// brief) rather than the notImplemented stub — OFFLINE, fast, no-radio
// usage error (no FILE argument).
func TestRun_SettingsDispatches(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := run([]string{"settings"}, strings.NewReader(""), &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("run([settings]) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
	if strings.Contains(stderr.String(), "not implemented") {
		t.Errorf("run([settings]) stderr = %q, want it NOT to say \"not implemented\"", stderr.String())
	}
}
