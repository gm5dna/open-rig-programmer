// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/csvio"
)

// validChannelData returns a ChannelData that validates cleanly against
// the ft710 driver's static offline Capabilities: a plausible frequency
// within range, a supported mode, CTCSS off, simplex shift, and
// CTCSSTone/ScanSkip left Unknown (the CAT-honest state for fields no CAT
// command can read or write — see core/driver/ft710/caps.go's
// bankFields doc comment).
//
// TagDisplay is Known-false, and stays so through M9c-5 (E1d): this
// stands in for a codeplug READ FROM A RADIO, and an MT answer genuinely
// carries the display flag (core/driver/ft710/read.go), so Known is its
// honest provenance. Only CHIRP-derived channels — which no file says
// anything about the display flag for — are Unknown; see
// TestCmdImport_CHIRP_Success, which asserts both sides of that contrast
// in one merged output.
func validChannelData(freqHz uint64, mode string) *codeplug.ChannelData {
	// yaesuTierUnavailable (write_inprocess_test.go) puts the ten
	// tier-added fields where a real read, a file load and a version-1
	// CSV import all put them — see its doc comment for why a zero value
	// here would make an unchanged channel plan as modified.
	return yaesuTierUnavailable(&codeplug.ChannelData{
		FreqHz:     freqHz,
		Mode:       mode,
		CTCSS:      "OFF",
		CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
		Shift:      "SIMPLEX",
		TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false},
		ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
	})
}

// buildValidBase returns a synthetic Codeplug matching the ft710 driver's
// static offline Capabilities EXACTLY (MEM "001".."099" + PMS
// "P1L".."P9U", nothing else — see core/driver/ft710/caps.go's
// baseCapabilities, which never asserts a region-dependent 60m/EMG bank
// statically) and validating with ZERO issues: M-01 populated (required),
// every PMS pair populated (a fixture convenience — real radios ship
// all-PMS-empty and validation accepts either since the NoBlank rule was
// removed at M5b), every other MEM slot empty. This
// costs no radio I/O at all — it exists so every import test below can
// exercise merge/validate logic in isolation, without needing the one
// fakeradio read task-13's brief budgets for fixture generation (spent
// instead on the black-box round-trip test in blackbox_test.go, which
// genuinely needs a real read's shape).
func buildValidBase() *codeplug.Codeplug {
	channels := make([]codeplug.Channel, 0, 99+18)
	channels = append(channels, codeplug.Channel{Slot: "001", Data: validChannelData(7_000_000, "USB")})
	for n := 2; n <= 99; n++ {
		channels = append(channels, codeplug.Channel{Slot: fmt.Sprintf("%03d", n)})
	}
	for pair := 1; pair <= 9; pair++ {
		channels = append(channels,
			codeplug.Channel{Slot: fmt.Sprintf("P%dL", pair), Data: validChannelData(14_000_000, "USB")},
			codeplug.Channel{Slot: fmt.Sprintf("P%dU", pair), Data: validChannelData(14_100_000, "USB")},
		)
	}
	return &codeplug.Codeplug{
		Schema:   codeplug.CurrentSchema,
		Radio:    codeplug.RadioInfo{Model: "FT-710", CATID: "0800"},
		Channels: channels,
	}
}

// saveFixture saves cp to a fresh file in t.TempDir() named name and
// returns its path.
func saveFixture(t *testing.T, cp *codeplug.Codeplug, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := codeplug.Save(path, cp); err != nil {
		t.Fatalf("saving fixture %s: %v", name, err)
	}
	return path
}

// --- Flag/plumbing tests -----------------------------------------------

func TestCmdImport_ExactlyOneOfCsvChirp(t *testing.T) {
	base := saveFixture(t, buildValidBase(), "base.json")
	out := filepath.Join(t.TempDir(), "out.json")

	var stdout, stderr bytes.Buffer
	got := cmdImport([]string{"--into", base, "--out", out}, &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdImport(neither --csv nor --chirp) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	got = cmdImport([]string{"--csv", "a.csv", "--chirp", "b.csv", "--into", base, "--out", out}, &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdImport(both --csv and --chirp) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
}

func TestCmdImport_MissingInto(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.json")
	var stdout, stderr bytes.Buffer
	got := cmdImport([]string{"--csv", "a.csv", "--out", out}, &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdImport(no --into) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
}

func TestCmdImport_MissingOut(t *testing.T) {
	base := saveFixture(t, buildValidBase(), "base.json")
	var stdout, stderr bytes.Buffer
	got := cmdImport([]string{"--csv", "a.csv", "--into", base}, &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdImport(no --out) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
}

func TestCmdImport_UnexpectedArgument(t *testing.T) {
	base := saveFixture(t, buildValidBase(), "base.json")
	out := filepath.Join(t.TempDir(), "out.json")
	var stdout, stderr bytes.Buffer
	got := cmdImport([]string{"--csv", "a.csv", "--into", base, "--out", out, "extra"}, &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdImport(extra positional) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
}

// TestCmdImport_RejectsPortAndFake pins task-13's OFFLINE requirement:
// import never opens a radio session, so --port/--fake are not even
// declared flags.
func TestCmdImport_RejectsPortAndFake(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdImport([]string{"--fake", "--csv", "a.csv", "--into", "base.json", "--out", "out.json"}, &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdImport(--fake) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	got = cmdImport([]string{"--port", "/dev/cu.fake", "--csv", "a.csv", "--into", "base.json", "--out", "out.json"}, &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdImport(--port) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
}

// TestCmdImport_RefuseOverwrite pins the shared refuse-overwrite rule: an
// existing --out without --force refuses before --into is ever loaded,
// exit 1, file left untouched.
func TestCmdImport_RefuseOverwrite(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "existing.json")
	const sentinel = "not a codeplug file"
	if err := os.WriteFile(out, []byte(sentinel), 0o600); err != nil {
		t.Fatalf("seeding existing --out file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	// --into need not even exist: the overwrite check happens first.
	got := cmdImport([]string{"--csv", "a.csv", "--into", "/nonexistent/base.json", "--out", out}, &stdout, &stderr)
	if got != exitError {
		t.Errorf("cmdImport(existing --out, no --force) = %d, want exitError (%d); stderr=%q", got, exitError, stderr.String())
	}
	if !strings.Contains(stderr.String(), "already exists") {
		t.Errorf("cmdImport(existing --out) stderr = %q, want it to mention the file already exists", stderr.String())
	}
	contents, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("reading %s after refused import: %v", out, err)
	}
	if string(contents) != sentinel {
		t.Error("cmdImport(existing --out, no --force) overwrote the file as a side effect, want it untouched")
	}
}

// TestCmdImport_LoadIntoNonexistent pins a plain --into-load failure.
func TestCmdImport_LoadIntoNonexistent(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.json")
	var stdout, stderr bytes.Buffer
	got := cmdImport([]string{"--csv", "a.csv", "--into", "/nonexistent/base.json", "--out", out}, &stdout, &stderr)
	if got != exitError {
		t.Errorf("cmdImport(nonexistent --into) = %d, want exitError (%d); stderr=%q", got, exitError, stderr.String())
	}
	if !strings.Contains(stderr.String(), "--into") {
		t.Errorf("cmdImport(nonexistent --into) stderr = %q, want it to mention --into", stderr.String())
	}
}

// TestCmdImport_LoadIntoSchemaTooNew pins the distinct schema-too-new
// message for --into.
func TestCmdImport_LoadIntoSchemaTooNew(t *testing.T) {
	base := filepath.Join(t.TempDir(), "too-new.json")
	writeTooNewCodeplug(t, base)
	out := filepath.Join(t.TempDir(), "out.json")

	var stdout, stderr bytes.Buffer
	got := cmdImport([]string{"--csv", "a.csv", "--into", base, "--out", out}, &stdout, &stderr)
	if got != exitError {
		t.Errorf("cmdImport(--into schema too new) = %d, want exitError (%d); stderr=%q", got, exitError, stderr.String())
	}
	if !strings.Contains(stderr.String(), "newer") {
		t.Errorf("cmdImport(--into schema too new) stderr = %q, want it to mention the file is newer than supported", stderr.String())
	}
}

func TestCmdImport_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdImport([]string{"-h"}, &stdout, &stderr)
	if got != exitSuccess {
		t.Errorf("cmdImport([-h]) = %d, want exitSuccess (%d)", got, exitSuccess)
	}
	if stdout.Len() == 0 {
		t.Error("cmdImport([-h]): stdout is empty, want usage text")
	}
}

// --- --csv path ----------------------------------------------------------

// writeCSVFixture exports channels as rigprog's own CSV schema to a fresh
// file in t.TempDir() and returns its path.
func writeCSVFixture(t *testing.T, channels []codeplug.Channel, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating CSV fixture %s: %v", name, err)
	}
	defer f.Close()
	if err := csvio.Export(f, channels); err != nil {
		t.Fatalf("csvio.Export(%s): %v", name, err)
	}
	return path
}

// TestCmdImport_CSV_Success pins the --csv happy path: matching slot
// inventory, clean merge, clean Validate, exit 0.
func TestCmdImport_CSV_Success(t *testing.T) {
	baseCp := buildValidBase()
	basePath := saveFixture(t, baseCp, "base.json")
	csvPath := writeCSVFixture(t, baseCp.Channels, "roundtrip.csv")
	out := filepath.Join(t.TempDir(), "out.json")

	var stdout, stderr bytes.Buffer
	got := cmdImport([]string{"--csv", csvPath, "--into", basePath, "--out", out}, &stdout, &stderr)
	if got != exitSuccess {
		t.Fatalf("cmdImport(--csv, matching inventory) = %d, want exitSuccess (%d); stdout=%q stderr=%q", got, exitSuccess, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), out) {
		t.Errorf("cmdImport(--csv) stdout = %q, want it to mention the output path", stdout.String())
	}

	result, err := codeplug.Load(out)
	if err != nil {
		t.Fatalf("codeplug.Load(%s): %v", out, err)
	}
	if codeplug.Digest(result.Channels) != codeplug.Digest(baseCp.Channels) {
		t.Error("cmdImport(--csv, matching inventory): output channels != base channels, want an identical round trip")
	}
	if !strings.HasPrefix(result.Generator, "rigprog/") {
		t.Errorf("cmdImport(--csv): Generator = %q, want the rigprog/ prefix", result.Generator)
	}
}

// TestCmdImport_UnknownModel pins task 40's --model validation for
// import: an unrecognised model exits 2 (usage), naming FT-710, before
// --into is even loaded (a nonexistent path is used deliberately).
func TestCmdImport_UnknownModel(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.json")
	var stdout, stderr bytes.Buffer
	got := cmdImport([]string{"--model", unknownModelSentinel, "--csv", "a.csv", "--into", "/nonexistent/base.json", "--out", out}, &stdout, &stderr)
	if got != exitUsage {
		t.Fatalf("cmdImport(--model %s) = %d, want exitUsage (%d); stderr=%q", unknownModelSentinel, got, exitUsage, stderr.String())
	}
	if !strings.Contains(stderr.String(), unknownModelSentinel) || !strings.Contains(stderr.String(), "FT-710") {
		t.Errorf("cmdImport(--model %s) stderr = %q, want it to name both the rejected and supported model", unknownModelSentinel, stderr.String())
	}
}

// TestCmdImport_FakeExplicitModel_ByteIdentical pins task 40's headline
// requirement: "--model FT-710" (the default, spelled out explicitly) is
// byte-identical (modulo the --out path) to no --model flag at all.
func TestCmdImport_FakeExplicitModel_ByteIdentical(t *testing.T) {
	baseCp := buildValidBase()
	basePath := saveFixture(t, baseCp, "base.json")
	csvPath := writeCSVFixture(t, baseCp.Channels, "roundtrip.csv")
	wantOut := filepath.Join(t.TempDir(), "want.json")
	gotOut := filepath.Join(t.TempDir(), "got.json")

	var wantStdout, wantStderr bytes.Buffer
	wantCode := cmdImport([]string{"--csv", csvPath, "--into", basePath, "--out", wantOut}, &wantStdout, &wantStderr)

	var gotStdout, gotStderr bytes.Buffer
	gotCode := cmdImport([]string{"--model", "FT-710", "--csv", csvPath, "--into", basePath, "--out", gotOut}, &gotStdout, &gotStderr)

	if gotCode != wantCode {
		t.Errorf("exit code = %d, want %d (flag-absent)", gotCode, wantCode)
	}
	wantOutput := strings.ReplaceAll(wantStdout.String(), wantOut, "<OUT>")
	gotOutput := strings.ReplaceAll(gotStdout.String(), gotOut, "<OUT>")
	if gotOutput != wantOutput {
		t.Errorf("stdout = %q, want byte-identical (modulo --out path) to flag-absent %q", gotOutput, wantOutput)
	}
	if gotStderr.String() != wantStderr.String() {
		t.Errorf("stderr = %q, want byte-identical to flag-absent %q", gotStderr.String(), wantStderr.String())
	}
}

// TestCmdImport_CSV_InventoryMismatch pins task-13 brief §2's --csv rule:
// a slot inventory that differs from --into's is refused, exit 1, naming
// the missing slot.
func TestCmdImport_CSV_InventoryMismatch(t *testing.T) {
	baseCp := buildValidBase()
	basePath := saveFixture(t, baseCp, "base.json")

	// Drop the last channel (P9U) before exporting, so the CSV's
	// inventory is missing exactly one slot base.json has.
	short := baseCp.Channels[:len(baseCp.Channels)-1]
	csvPath := writeCSVFixture(t, short, "short.csv")
	out := filepath.Join(t.TempDir(), "out.json")

	var stdout, stderr bytes.Buffer
	got := cmdImport([]string{"--csv", csvPath, "--into", basePath, "--out", out}, &stdout, &stderr)
	if got != exitError {
		t.Errorf("cmdImport(--csv, short inventory) = %d, want exitError (%d); stderr=%q", got, exitError, stderr.String())
	}
	if !strings.Contains(stderr.String(), "P9U") {
		t.Errorf("cmdImport(--csv, short inventory) stderr = %q, want it to name the missing slot P9U", stderr.String())
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("cmdImport(--csv, short inventory): --out = %s, want it to not exist", out)
	}
}

// TestCmdImport_CSV_OpenNonexistent pins a plain --csv-open failure.
func TestCmdImport_CSV_OpenNonexistent(t *testing.T) {
	base := saveFixture(t, buildValidBase(), "base.json")
	out := filepath.Join(t.TempDir(), "out.json")

	var stdout, stderr bytes.Buffer
	got := cmdImport([]string{"--csv", "/nonexistent/import.csv", "--into", base, "--out", out}, &stdout, &stderr)
	if got != exitError {
		t.Errorf("cmdImport(nonexistent --csv) = %d, want exitError (%d); stderr=%q", got, exitError, stderr.String())
	}
}

// TestCmdImport_CSV_ParseError pins a malformed-CSV failure (unknown
// column), exit 1.
func TestCmdImport_CSV_ParseError(t *testing.T) {
	base := saveFixture(t, buildValidBase(), "base.json")
	csvPath := filepath.Join(t.TempDir(), "malformed.csv")
	if err := os.WriteFile(csvPath, []byte("not,the,right,header\n1,2,3,4\n"), 0o600); err != nil {
		t.Fatalf("writing malformed CSV: %v", err)
	}
	out := filepath.Join(t.TempDir(), "out.json")

	var stdout, stderr bytes.Buffer
	got := cmdImport([]string{"--csv", csvPath, "--into", base, "--out", out}, &stdout, &stderr)
	if got != exitError {
		t.Errorf("cmdImport(malformed --csv) = %d, want exitError (%d); stderr=%q", got, exitError, stderr.String())
	}
}

// --- --chirp path ----------------------------------------------------------

const chirpHeaderLine = "Location,Name,Frequency,Duplex,Tone,rToneFreq,cToneFreq,Mode,Skip"

// writeCHIRPFixture writes a CHIRP CSV (header + rows) to a fresh file in
// t.TempDir() and returns its path.
func writeCHIRPFixture(t *testing.T, name string, rows ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	content := chirpHeaderLine + "\n" + strings.Join(rows, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing CHIRP fixture %s: %v", name, err)
	}
	return path
}

// TestCmdImport_CHIRP_Success pins the CHIRP happy path (task-13 brief
// §3): a row mapping cleanly into an empty MEM slot merges cleanly,
// untouched slots (including M-01 and every PMS pair) are preserved,
// exit 0.
//
// It also pins M9c-5 E1d's PROVENANCE at the CLI level (the downstream
// pin the plan names): the CHIRP-imported slot carries an Unknown
// tag_display all the way into the saved --out file, while an untouched
// radio-read slot beside it keeps its Known one. The two states surviving
// side by side in one file is what schema 3's always-present tag_display
// key exists for.
func TestCmdImport_CHIRP_Success(t *testing.T) {
	baseCp := buildValidBase()
	basePath := saveFixture(t, baseCp, "base.json")
	// Location 2 -> slot "002", currently empty in buildValidBase.
	chirpPath := writeCHIRPFixture(t, "happy.csv", `2,MYCALL,7.100000,,,,,USB,`)
	out := filepath.Join(t.TempDir(), "out.json")

	var stdout, stderr bytes.Buffer
	got := cmdImport([]string{"--chirp", chirpPath, "--into", basePath, "--out", out}, &stdout, &stderr)
	if got != exitSuccess {
		t.Fatalf("cmdImport(--chirp, happy path) = %d, want exitSuccess (%d); stdout=%q stderr=%q", got, exitSuccess, stdout.String(), stderr.String())
	}

	result, err := codeplug.Load(out)
	if err != nil {
		t.Fatalf("codeplug.Load(%s): %v", out, err)
	}
	var slot002, slot001 codeplug.Channel
	for _, ch := range result.Channels {
		switch ch.Slot {
		case "002":
			slot002 = ch
		case "001":
			slot001 = ch
		}
	}
	if slot002.Data == nil || slot002.Data.FreqHz != 7_100_000 || slot002.Data.Tag != "MYCALL" {
		t.Errorf("cmdImport(--chirp): slot 002 = %+v, want freq 145500000 tag MYCALL", slot002)
	}
	if slot001.Data == nil || slot001.Data.FreqHz != 7_000_000 {
		t.Errorf("cmdImport(--chirp): slot 001 (untouched) = %+v, want it preserved from base", slot001)
	}
	if want := (codeplug.BoolField{State: codeplug.Unknown}); slot002.Data == nil || slot002.Data.TagDisplay != want {
		t.Errorf("cmdImport(--chirp): slot 002 TagDisplay = %+v, want %+v (CHIRP carries no display flag; the imported channel must stay honestly unresolved through the merge and the save)", slot002.Data, want)
	}
	if want := (codeplug.BoolField{State: codeplug.Known, Value: false}); slot001.Data == nil || slot001.Data.TagDisplay != want {
		t.Errorf("cmdImport(--chirp): slot 001 TagDisplay = %+v, want %+v (an untouched radio-read slot keeps its Known state; the CHIRP import must not reinterpret its neighbours)", slot001.Data, want)
	}
	if len(result.Channels) != len(baseCp.Channels) {
		t.Errorf("cmdImport(--chirp): channel count = %d, want %d (inventory unchanged)", len(result.Channels), len(baseCp.Channels))
	}
	if !strings.Contains(stdout.String(), "offline validation notes") {
		t.Errorf("cmdImport(--chirp, happy path) stdout = %q, want the advisory label \"offline validation notes\" even on a clean merge", stdout.String())
	}
}

// TestCmdImport_CHIRP_Blocking pins task-13 brief §3's blocking path: an
// unrecognised Mode produces a Blocking loss entry, the report is
// printed, NOTHING is written, exit 3.
func TestCmdImport_CHIRP_Blocking(t *testing.T) {
	base := saveFixture(t, buildValidBase(), "base.json")
	// "FSK" has no chirpModeMap entry -> Blocking ActionUnsupported.
	chirpPath := writeCHIRPFixture(t, "blocking.csv", `2,MYCALL,7.100000,,,,,FSK,`)
	out := filepath.Join(t.TempDir(), "out.json")

	var stdout, stderr bytes.Buffer
	got := cmdImport([]string{"--chirp", chirpPath, "--into", base, "--out", out}, &stdout, &stderr)
	if got != exitBlocked {
		t.Fatalf("cmdImport(--chirp, blocking) = %d, want exitBlocked (%d); stdout=%q stderr=%q", got, exitBlocked, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "FSK") || !strings.Contains(stdout.String(), "BLOCKING") {
		t.Errorf("cmdImport(--chirp, blocking) stdout = %q, want the loss report to name FSK and mark it BLOCKING", stdout.String())
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("cmdImport(--chirp, blocking): --out = %s, want it to not exist (nothing written)", out)
	}
}

// TestCmdImport_CHIRP_UnknownSlot pins task-13 brief §3's unknown-slot
// path: a row targeting a slot outside --into's inventory is refused,
// exit 1, naming the slot; nothing written.
func TestCmdImport_CHIRP_UnknownSlot(t *testing.T) {
	// A deliberately small BASE: only slots 001 and 002 exist.
	small := &codeplug.Codeplug{
		Schema: codeplug.CurrentSchema,
		Channels: []codeplug.Channel{
			{Slot: "001", Data: validChannelData(7_000_000, "USB")},
			{Slot: "002"},
		},
	}
	base := saveFixture(t, small, "small-base.json")
	// Location 99 -> slot "099", not present in `small`.
	chirpPath := writeCHIRPFixture(t, "unknown-slot.csv", `99,MYCALL,7.100000,,,,,USB,`)
	out := filepath.Join(t.TempDir(), "out.json")

	var stdout, stderr bytes.Buffer
	got := cmdImport([]string{"--chirp", chirpPath, "--into", base, "--out", out}, &stdout, &stderr)
	if got != exitError {
		t.Fatalf("cmdImport(--chirp, unknown slot) = %d, want exitError (%d); stdout=%q stderr=%q", got, exitError, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "099") {
		t.Errorf("cmdImport(--chirp, unknown slot) stderr = %q, want it to name slot 099", stderr.String())
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("cmdImport(--chirp, unknown slot): --out = %s, want it to not exist (nothing written)", out)
	}
}

// TestCmdImport_CHIRP_DuplicateLocation pins Fix 5 (adjudicated MEDIUM,
// Codex M4 #5): mergeCHIRP used to be last-wins on a duplicate imported
// slot, silently discarding an earlier row's data. Two CHIRP rows both
// targeting Location 2 (-> slot "002") must instead refuse the import
// wholesale — exit 1, naming the duplicated slot in display form — with
// nothing written, rather than quietly keeping only the second row.
func TestCmdImport_CHIRP_DuplicateLocation(t *testing.T) {
	base := saveFixture(t, buildValidBase(), "base.json")
	chirpPath := writeCHIRPFixture(t, "duplicate.csv",
		`2,MYCALL1,7.100000,,,,,USB,`,
		`2,MYCALL2,7.150000,,,,,USB,`,
	)
	out := filepath.Join(t.TempDir(), "out.json")

	var stdout, stderr bytes.Buffer
	got := cmdImport([]string{"--chirp", chirpPath, "--into", base, "--out", out}, &stdout, &stderr)
	if got != exitError {
		t.Fatalf("cmdImport(--chirp, duplicate Location) = %d, want exitError (%d); stdout=%q stderr=%q", got, exitError, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "M-02") {
		t.Errorf("cmdImport(--chirp, duplicate Location) stderr = %q, want it to name the duplicated slot (M-02, display form)", stderr.String())
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("cmdImport(--chirp, duplicate Location): --out = %s, want it to not exist (nothing written)", out)
	}
}

// TestCmdImport_CHIRP_HardParseError pins a hard CHIRP parse error
// (missing a core column): exit 1, nothing written.
func TestCmdImport_CHIRP_HardParseError(t *testing.T) {
	base := saveFixture(t, buildValidBase(), "base.json")
	chirpPath := filepath.Join(t.TempDir(), "no-core-columns.csv")
	if err := os.WriteFile(chirpPath, []byte("Name,Comment\nMYCALL,hello\n"), 0o600); err != nil {
		t.Fatalf("writing CHIRP fixture: %v", err)
	}
	out := filepath.Join(t.TempDir(), "out.json")

	var stdout, stderr bytes.Buffer
	got := cmdImport([]string{"--chirp", chirpPath, "--into", base, "--out", out}, &stdout, &stderr)
	if got != exitError {
		t.Fatalf("cmdImport(--chirp, hard parse error) = %d, want exitError (%d); stdout=%q stderr=%q", got, exitError, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("cmdImport(--chirp, hard parse error): --out = %s, want it to not exist", out)
	}
}

// TestCmdImport_CHIRP_ErrorDominatesBlocking pins the AMENDED task-13
// brief §2 CHIRP precedence rule: a non-nil error from csvio.ImportCHIRP
// is handled FIRST, even when the report it returns alongside that error
// already contains a Blocking entry from an earlier, successfully-parsed
// row — the error dominates, exit 1 (not exitBlocked/3), nothing
// written. Constructed per csvio/chirp.go's own ImportCHIRP loop: row 1
// (Location 2, Mode "FSK") parses fine as a CSV record but produces a
// Blocking loss entry (unrecognised Mode) that is appended to the report
// AND still yields a non-nil Channel (see importCHIRPRow: only an
// unparseable/out-of-range Location drops the row outright); row 2 then
// contains a bare quote mid-field ("MYCALL\"2"), which is a genuine CSV
// syntax error Go's encoding/csv reader rejects (ErrBareQuote) — confirmed
// empirically against encoding/csv before writing this fixture. ImportCHIRP
// returns the channels/report accumulated so far (row 1's Blocking entry
// included) alongside that hard ParseError, exactly the "err != nil AND
// report.HasBlocking()" case the precedence rule exists for.
func TestCmdImport_CHIRP_ErrorDominatesBlocking(t *testing.T) {
	base := saveFixture(t, buildValidBase(), "base.json")
	chirpPath := writeCHIRPFixture(t, "error-dominates.csv",
		`2,MYCALL,7.100000,,,,,FSK,`,
		`3,MYCALL"2,7.200000,,,,,USB,`,
	)
	out := filepath.Join(t.TempDir(), "out.json")

	var stdout, stderr bytes.Buffer
	got := cmdImport([]string{"--chirp", chirpPath, "--into", base, "--out", out}, &stdout, &stderr)
	if got != exitError {
		t.Fatalf("cmdImport(--chirp, err+blocking report) = %d, want exitError (%d); stdout=%q stderr=%q", got, exitError, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "FSK") || !strings.Contains(stdout.String(), "BLOCKING") {
		t.Errorf("cmdImport(--chirp, err+blocking report) stdout = %q, want the (non-empty) loss report printed, naming FSK and marked BLOCKING", stdout.String())
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("cmdImport(--chirp, err+blocking report): --out = %s, want it to not exist (nothing written)", out)
	}
}

// TestCmdImport_CHIRP_OpenNonexistent pins a plain --chirp-open failure.
func TestCmdImport_CHIRP_OpenNonexistent(t *testing.T) {
	base := saveFixture(t, buildValidBase(), "base.json")
	out := filepath.Join(t.TempDir(), "out.json")

	var stdout, stderr bytes.Buffer
	got := cmdImport([]string{"--chirp", "/nonexistent/import.csv", "--into", base, "--out", out}, &stdout, &stderr)
	if got != exitError {
		t.Errorf("cmdImport(nonexistent --chirp) = %d, want exitError (%d); stderr=%q", got, exitError, stderr.String())
	}
}

// --- Validation-issues path (AMENDED: advisory only, never gates) --------

// TestCmdImport_ValidationIssues pins task-13 brief §2's AMENDED
// "offline validation is advisory" rule: a CHIRP row that parses fine
// (no Blocking loss entry) but produces a SeverityError once merged and
// Validated (a frequency below the radio's minimum) still writes --out
// AND exits 0 — the issue is printed under the advisory label, but never
// gates the exit code. Authoritative validation happens at write time
// against the connected radio.
func TestCmdImport_ValidationIssues(t *testing.T) {
	base := saveFixture(t, buildValidBase(), "base.json")
	// 20 kHz: below caps.MinFreqHz (30 kHz) but a syntactically valid
	// CHIRP frequency (parses to a whole-Hz value with no loss entry).
	chirpPath := writeCHIRPFixture(t, "too-low-freq.csv", `2,MYCALL,0.020000,,,,,USB,`)
	out := filepath.Join(t.TempDir(), "out.json")

	var stdout, stderr bytes.Buffer
	got := cmdImport([]string{"--chirp", chirpPath, "--into", base, "--out", out}, &stdout, &stderr)
	if got != exitSuccess {
		t.Fatalf("cmdImport(--chirp, validation error) = %d, want exitSuccess (%d); stdout=%q stderr=%q", got, exitSuccess, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "ERROR") {
		t.Errorf("cmdImport(--chirp, validation error) stdout = %q, want it to print the ERROR issue", stdout.String())
	}
	if !strings.Contains(stdout.String(), "offline validation notes") {
		t.Errorf("cmdImport(--chirp, validation error) stdout = %q, want the advisory label \"offline validation notes\"", stdout.String())
	}
	if !strings.Contains(stdout.String(), "authoritative validation happens at write time against the connected radio") {
		t.Errorf("cmdImport(--chirp, validation error) stdout = %q, want the advisory label's authoritative-at-write-time explanation", stdout.String())
	}
	if _, err := os.Stat(out); err != nil {
		t.Errorf("cmdImport(--chirp, validation error): --out = %s not written, want it written: %v", out, err)
	}
}

// --- --out == --into --------------------------------------------------

// TestCmdImport_OutEqualsInto_RequiresForce pins "--out may equal --into
// only with --force": without --force, the existing --into path is
// refused as any other pre-existing --out would be.
func TestCmdImport_OutEqualsInto_RequiresForce(t *testing.T) {
	baseCp := buildValidBase()
	basePath := saveFixture(t, baseCp, "base.json")
	csvPath := writeCSVFixture(t, baseCp.Channels, "roundtrip.csv")

	var stdout, stderr bytes.Buffer
	got := cmdImport([]string{"--csv", csvPath, "--into", basePath, "--out", basePath}, &stdout, &stderr)
	if got != exitError {
		t.Errorf("cmdImport(--out == --into, no --force) = %d, want exitError (%d); stderr=%q", got, exitError, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	got = cmdImport([]string{"--csv", csvPath, "--into", basePath, "--out", basePath, "--force"}, &stdout, &stderr)
	if got != exitSuccess {
		t.Errorf("cmdImport(--out == --into, --force) = %d, want exitSuccess (%d); stderr=%q", got, exitSuccess, stderr.String())
	}
}

// --- Formula-injection escaping round trip --------------------------------

// TestFormulaEscape_ExportImportRoundTrip pins task-13 brief §3's
// formula-escape spot check at the CLI level: a tag beginning '=' is
// escaped by export, and import round-trips it back to the original tag.
// Injected by editing the JSON fixture directly (task-13 brief §3's
// second suggested option) rather than a second fakeradio read.
func TestFormulaEscape_ExportImportRoundTrip(t *testing.T) {
	baseCp := buildValidBase()
	baseCp.Channels[0].Data.Tag = "=SUM(A1)"
	basePath := saveFixture(t, baseCp, "base.json")

	csvOut := filepath.Join(t.TempDir(), "out.csv")
	var exportStdout, exportStderr bytes.Buffer
	gotExport := cmdExport([]string{"--csv", csvOut, basePath}, &exportStdout, &exportStderr)
	if gotExport != exitSuccess {
		t.Fatalf("cmdExport(formula tag) = %d, want exitSuccess (%d); stderr=%q", gotExport, exitSuccess, exportStderr.String())
	}

	csvBytes, err := os.ReadFile(csvOut)
	if err != nil {
		t.Fatalf("reading exported CSV: %v", err)
	}
	firstDataLine := strings.Split(string(csvBytes), "\n")[1]
	if !strings.Contains(firstDataLine, "'=SUM(A1)") {
		t.Errorf("exported CSV first data line = %q, want the tag cell escaped to \"'=SUM(A1)\"", firstDataLine)
	}

	backPath := filepath.Join(t.TempDir(), "back.json")
	var importStdout, importStderr bytes.Buffer
	gotImport := cmdImport([]string{"--csv", csvOut, "--into", basePath, "--out", backPath}, &importStdout, &importStderr)
	if gotImport != exitSuccess {
		t.Fatalf("cmdImport(formula tag round trip) = %d, want exitSuccess (%d); stdout=%q stderr=%q", gotImport, exitSuccess, importStdout.String(), importStderr.String())
	}

	back, err := codeplug.Load(backPath)
	if err != nil {
		t.Fatalf("codeplug.Load(%s): %v", backPath, err)
	}
	if back.Channels[0].Data == nil || back.Channels[0].Data.Tag != "=SUM(A1)" {
		t.Errorf("round-tripped slot 001 tag = %+v, want \"=SUM(A1)\" restored exactly", back.Channels[0].Data)
	}
}
