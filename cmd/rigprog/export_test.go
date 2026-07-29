// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/csvio"
)

// TestCmdExport_MissingCSV pins "--csv is required".
func TestCmdExport_MissingCSV(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdExport([]string{"somefile.json"}, &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdExport(no --csv) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
}

// TestCmdExport_MissingFileArgument / TooManyArguments pin the "exactly
// one FILE argument" requirement, matching diff's style.
func TestCmdExport_MissingFileArgument(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdExport([]string{"--csv", "out.csv"}, &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdExport(no FILE) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
}

func TestCmdExport_TooManyArguments(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdExport([]string{"--csv", "out.csv", "a.json", "b.json"}, &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdExport(2 FILEs) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
}

// TestCmdExport_RejectsPortAndFake pins task-13's OFFLINE requirement:
// export never opens a radio session, so --port/--fake are not even
// declared flags — passing either is an ordinary flag-parse failure,
// exit 2.
func TestCmdExport_RejectsPortAndFake(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdExport([]string{"--fake", "--csv", "out.csv", "somefile.json"}, &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdExport(--fake) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	got = cmdExport([]string{"--port", "/dev/cu.fake", "--csv", "out.csv", "somefile.json"}, &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdExport(--port) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
}

// TestCmdExport_RefuseOverwrite pins the shared refuse-overwrite rule
// (checkOverwrite, fileio.go): an existing --csv without --force refuses
// before FILE is ever loaded, exit 1, file left untouched.
func TestCmdExport_RefuseOverwrite(t *testing.T) {
	dir := t.TempDir()
	csvOut := filepath.Join(dir, "existing.csv")
	const sentinel = "not a csv file"
	if err := os.WriteFile(csvOut, []byte(sentinel), 0o600); err != nil {
		t.Fatalf("seeding existing --csv file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	// FILE need not even exist: the overwrite check happens first.
	got := cmdExport([]string{"--csv", csvOut, "/nonexistent/rigprog-test.json"}, &stdout, &stderr)
	if got != exitError {
		t.Errorf("cmdExport(existing --csv, no --force) = %d, want exitError (%d); stderr=%q", got, exitError, stderr.String())
	}
	if !strings.Contains(stderr.String(), "already exists") {
		t.Errorf("cmdExport(existing --csv) stderr = %q, want it to mention the file already exists", stderr.String())
	}
	contents, err := os.ReadFile(csvOut)
	if err != nil {
		t.Fatalf("reading %s after refused export: %v", csvOut, err)
	}
	if string(contents) != sentinel {
		t.Error("cmdExport(existing --csv, no --force) overwrote the file as a side effect, want it untouched")
	}
}

// TestCmdExport_LoadNonexistentFile pins a plain FILE-load failure: exit
// 1, via the shared loadCodeplugStrict helper.
func TestCmdExport_LoadNonexistentFile(t *testing.T) {
	dir := t.TempDir()
	csvOut := filepath.Join(dir, "out.csv")

	var stdout, stderr bytes.Buffer
	got := cmdExport([]string{"--csv", csvOut, "/nonexistent/path/rigprog-test.json"}, &stdout, &stderr)
	if got != exitError {
		t.Errorf("cmdExport(nonexistent FILE) = %d, want exitError (%d); stderr=%q", got, exitError, stderr.String())
	}
	if _, err := os.Stat(csvOut); !os.IsNotExist(err) {
		t.Errorf("cmdExport(nonexistent FILE): --csv = %s, want it to not exist", csvOut)
	}
}

// TestCmdExport_SchemaTooNew pins the distinct schema-too-new message.
func TestCmdExport_SchemaTooNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "too-new.json")
	tooNew := &codeplug.Codeplug{Schema: codeplug.CurrentSchema + 1}
	if err := codeplug.Save(path, tooNew); err != nil {
		t.Fatalf("Save fixture: %v", err)
	}
	csvOut := filepath.Join(dir, "out.csv")

	var stdout, stderr bytes.Buffer
	got := cmdExport([]string{"--csv", csvOut, path}, &stdout, &stderr)
	if got != exitError {
		t.Errorf("cmdExport(schema too new) = %d, want exitError (%d); stderr=%q", got, exitError, stderr.String())
	}
	if !strings.Contains(stderr.String(), "newer") {
		t.Errorf("cmdExport(schema too new) stderr = %q, want it to mention the file is newer than supported", stderr.String())
	}
}

// TestCmdExport_Success pins the happy path: rows written (one per
// slot, including empty slots — Export's own contract), exit 0, a CSV
// that reads back via csvio.Import to the same channels.
func TestCmdExport_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "in.json")
	fixture := &codeplug.Codeplug{
		Schema: codeplug.CurrentSchema,
		Channels: []codeplug.Channel{
			{Slot: "001", Data: &codeplug.ChannelData{FreqHz: 7_000_000, Mode: "USB", CTCSS: "OFF", Shift: "SIMPLEX", Tag: "MYCALL", TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false}}},
			{Slot: "002"}, // empty
		},
	}
	if err := codeplug.Save(path, fixture); err != nil {
		t.Fatalf("Save fixture: %v", err)
	}
	csvOut := filepath.Join(dir, "out.csv")

	var stdout, stderr bytes.Buffer
	got := cmdExport([]string{"--csv", csvOut, path}, &stdout, &stderr)
	if got != exitSuccess {
		t.Fatalf("cmdExport(valid) = %d, want exitSuccess (%d); stderr=%q", got, exitSuccess, stderr.String())
	}
	if !strings.Contains(stdout.String(), "2") {
		t.Errorf("cmdExport(valid) stdout = %q, want it to mention the row count (2, including the empty slot)", stdout.String())
	}
	if !strings.Contains(stdout.String(), csvOut) {
		t.Errorf("cmdExport(valid) stdout = %q, want it to mention the output path %q", stdout.String(), csvOut)
	}

	f, err := os.Open(csvOut)
	if err != nil {
		t.Fatalf("opening exported CSV: %v", err)
	}
	defer f.Close()
	channels, err := csvio.Import(f)
	if err != nil {
		t.Fatalf("csvio.Import(exported CSV): %v", err)
	}
	if len(channels) != 2 {
		t.Fatalf("csvio.Import(exported CSV): got %d channels, want 2", len(channels))
	}
	if channels[0].Slot != "001" || channels[0].Data == nil || channels[0].Data.Tag != "MYCALL" {
		t.Errorf("csvio.Import(exported CSV): channel[0] = %+v, want slot 001 tag MYCALL", channels[0])
	}
	if channels[1].Slot != "002" || channels[1].Data != nil {
		t.Errorf("csvio.Import(exported CSV): channel[1] = %+v, want empty slot 002", channels[1])
	}
}

// TestCmdExport_ForceOverwrite pins that --force allows overwriting an
// existing --csv file.
func TestCmdExport_ForceOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "in.json")
	fixture := &codeplug.Codeplug{Schema: codeplug.CurrentSchema, Channels: []codeplug.Channel{{Slot: "001"}}}
	if err := codeplug.Save(path, fixture); err != nil {
		t.Fatalf("Save fixture: %v", err)
	}
	csvOut := filepath.Join(dir, "out.csv")
	if err := os.WriteFile(csvOut, []byte("stale"), 0o600); err != nil {
		t.Fatalf("seeding existing --csv file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	got := cmdExport([]string{"--csv", csvOut, "--force", path}, &stdout, &stderr)
	if got != exitSuccess {
		t.Fatalf("cmdExport(--force) = %d, want exitSuccess (%d); stderr=%q", got, exitSuccess, stderr.String())
	}
	contents, err := os.ReadFile(csvOut)
	if err != nil {
		t.Fatalf("reading %s: %v", csvOut, err)
	}
	if string(contents) == "stale" {
		t.Error("cmdExport(--force) did not overwrite the existing file")
	}
}

// TestCmdExport_Help pins "rigprog export -h".
func TestCmdExport_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdExport([]string{"-h"}, &stdout, &stderr)
	if got != exitSuccess {
		t.Errorf("cmdExport([-h]) = %d, want exitSuccess (%d)", got, exitSuccess)
	}
	if stdout.Len() == 0 {
		t.Error("cmdExport([-h]): stdout is empty, want usage text")
	}
	for _, want := range []string{"lossless round trip", "import-only"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("cmdExport([-h]) stdout = %q, want it to contain %q (task-13 brief §1's required help note)", stdout.String(), want)
		}
	}
}
