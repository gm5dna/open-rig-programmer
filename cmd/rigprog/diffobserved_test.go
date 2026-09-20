// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
)

// Fixture CSVs below are TRIMMED, real rows lifted verbatim from
// core/cat/table2.csv and core/cat/table2-observed.csv (addresses 010101
// and 040101) — not invented shapes, so the fixture exercises the real
// signed/text lexicon. The codeplug JSON entries these are diffed against
// carry INVENTED values (task-h1 brief: "invented values, not captured
// ones"): "+01" is the same synthetic magnitude fakeradio's own EX
// runtime default uses for this address (see TestBlackbox_SettingsShow),
// and "TESTCALL    " is a made-up, right-space-padded 12-byte call sign,
// never a captured one.

const diffObservedManualCSVFixture = "01,01,01,RADIO SETTING,MODE SSB,AF TREBLE GAIN,-20 - -00 (or +00) - +10,3,false,646\n" +
	"04,01,01,DISPLAY SETTING,DISPLAY,MY CALL,Up to 12 characters,12,true,879\n"

const diffObservedObservedCSVFixture = "01,01,01,3,signed\n" +
	"04,01,01,12,text\n"

// writeDiffObservedFixtures writes the trimmed manual/observed CSVs to
// dir and returns their paths.
func writeDiffObservedFixtures(t *testing.T, dir string) (manualPath, observedPath string) {
	t.Helper()
	manualPath = filepath.Join(dir, "manual.csv")
	observedPath = filepath.Join(dir, "observed.csv")
	if err := os.WriteFile(manualPath, []byte(diffObservedManualCSVFixture), 0o600); err != nil {
		t.Fatalf("writing manual CSV fixture: %v", err)
	}
	if err := os.WriteFile(observedPath, []byte(diffObservedObservedCSVFixture), 0o600); err != nil {
		t.Fatalf("writing observed CSV fixture: %v", err)
	}
	return manualPath, observedPath
}

// TestCmdSettings_DiffObserved_ZeroDiff is the fixture round-trip's
// zero-diff case (task-h1 brief): both entries' width/shape match the
// trimmed observed CSV — exit 0, "Differences: 0".
func TestCmdSettings_DiffObserved_ZeroDiff(t *testing.T) {
	dir := t.TempDir()
	manualPath, observedPath := writeDiffObservedFixtures(t, dir)
	jsonPath := filepath.Join(dir, "snap.json")
	buildSettingsSnapshotFile(t, jsonPath, []codeplug.MenuEntry{
		{ID: "010101", Value: "+01", State: codeplug.MenuKnown},          // signed, 3 bytes — matches
		{ID: "040101", Value: "TESTCALL    ", State: codeplug.MenuKnown}, // text, 12 bytes — matches
	})

	var stdout, stderr bytes.Buffer
	got := cmdSettings([]string{"diff-observed", "--manual-csv", manualPath, "--observed-csv", observedPath, jsonPath}, &stdout, &stderr)
	if got != exitSuccess {
		t.Fatalf("cmdSettings(diff-observed, zero-diff) = %d, want exitSuccess (%d); stdout=%q stderr=%q", got, exitSuccess, stdout.String(), stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Addresses compared: 2") {
		t.Errorf("stdout = %q, want \"Addresses compared: 2\"", out)
	}
	if diffObservedCount(t, out) != 0 {
		t.Errorf("stdout = %q, want a \"Differences:\" line reading 0", out)
	}
}

// TestCmdSettings_DiffObserved_OneDifference is the fixture round-trip's
// one-difference case: 010101's file value no longer has a sign
// (shape becomes "numeric" against the baseline's "signed") — exit 1,
// one differing address reported, and — the privacy rule this tool
// exists to keep — neither the changed value ("042") nor the unchanged
// one ("TESTCALL") ever appears on stdout or stderr.
func TestCmdSettings_DiffObserved_OneDifference(t *testing.T) {
	dir := t.TempDir()
	manualPath, observedPath := writeDiffObservedFixtures(t, dir)
	jsonPath := filepath.Join(dir, "snap.json")
	buildSettingsSnapshotFile(t, jsonPath, []codeplug.MenuEntry{
		{ID: "010101", Value: "042", State: codeplug.MenuKnown},          // unsigned now — shape differs
		{ID: "040101", Value: "TESTCALL    ", State: codeplug.MenuKnown}, // still matches
	})

	var stdout, stderr bytes.Buffer
	got := cmdSettings([]string{"diff-observed", "--manual-csv", manualPath, "--observed-csv", observedPath, jsonPath}, &stdout, &stderr)
	if got != exitError {
		t.Fatalf("cmdSettings(diff-observed, one diff) = %d, want exitError (%d); stdout=%q stderr=%q", got, exitError, stdout.String(), stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "010101") {
		t.Errorf("stdout = %q, want the differing address 010101 named", out)
	}
	if !strings.Contains(out, "observed=3/signed") || !strings.Contains(out, "file=3/numeric") {
		t.Errorf("stdout = %q, want \"observed=3/signed\" and \"file=3/numeric\"", out)
	}
	if !strings.Contains(out, "Addresses compared: 2") {
		t.Errorf("stdout = %q, want \"Addresses compared: 2\"", out)
	}
	if diffObservedCount(t, out) != 1 {
		t.Errorf("stdout = %q, want a \"Differences:\" line reading 1", out)
	}
	for _, value := range []string{"042", "TESTCALL"} {
		if strings.Contains(out, value) || strings.Contains(stderr.String(), value) {
			t.Errorf("output leaked a setting value %q — privacy rule: this tool must diff width/shape only", value)
		}
	}
}

// diffObservedCount extracts the integer count from the "Differences: N"
// summary line, so the test does not have to hardcode
// writeDiffObservedReport's column alignment.
func diffObservedCount(t *testing.T, stdout string) int {
	t.Helper()
	for _, line := range strings.Split(stdout, "\n") {
		if rest, ok := strings.CutPrefix(line, "Differences:"); ok {
			n, err := strconv.Atoi(strings.TrimSpace(rest))
			if err != nil {
				t.Fatalf("parsing %q: %v", line, err)
			}
			return n
		}
	}
	t.Fatalf("stdout = %q, want a \"Differences:\" line", stdout)
	return -1
}

// TestCmdSettings_DiffObserved_MissingFile pins the flags-first usage
// error: no FILE argument is exit 2, no file ever touched.
func TestCmdSettings_DiffObserved_MissingFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdSettings([]string{"diff-observed"}, &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdSettings(diff-observed, no FILE) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
}

// TestCmdSettings_DiffObserved_OuterFlagRefused pins that an outer
// --csv/--model/--force flag alongside the reserved "diff-observed" word
// is refused rather than silently ignored (mirrors
// unverified-writes' own refusal).
func TestCmdSettings_DiffObserved_OuterFlagRefused(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdSettings([]string{"--model", "FT-710", "diff-observed", "somefile.json"}, &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdSettings(--model FT-710 diff-observed FILE) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
}
