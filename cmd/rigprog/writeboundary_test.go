// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
)

// writeBoundaryDeniedGolden and writeBoundaryHeldGolden mirror
// core/cat/exdenylist_test.go's own golden lists (58 denied, 4 held) by
// (P1,P2,P3) triple — a second, independent pin against the exact same
// milestone spec table, from the CLI side, so a future change that broke
// EITHER side's filter fails at least one of the two tests. Comments name
// the item for a human diffing a failure only; the test does not read
// them.
var writeBoundaryDeniedGolden = [][3]int{
	{1, 1, 14}, {1, 1, 17}, {1, 2, 14}, {1, 2, 17}, {1, 3, 13}, {1, 3, 16},
	{1, 4, 14}, {1, 4, 17}, {1, 5, 13}, {2, 1, 13}, {2, 1, 15},
	{3, 1, 3}, {3, 1, 4}, {3, 1, 5}, {3, 1, 6}, {3, 1, 7}, {3, 1, 8},
	{3, 1, 9}, {3, 1, 10}, {3, 1, 11}, {3, 1, 15},
	{3, 4, 1}, {3, 4, 2}, {3, 4, 3}, {3, 4, 4}, {3, 4, 6}, {3, 4, 7},
	{4, 1, 1},
	{6, 1, 1}, {6, 1, 2}, {6, 1, 3}, {6, 1, 4}, {6, 1, 15}, {6, 1, 18},
	{6, 2, 1}, {6, 2, 2}, {6, 2, 3}, {6, 2, 4}, {6, 2, 15}, {6, 2, 18},
	{6, 3, 1}, {6, 3, 2}, {6, 3, 3}, {6, 3, 4}, {6, 3, 15}, {6, 3, 18},
	{6, 4, 1}, {6, 4, 2}, {6, 4, 3}, {6, 4, 4}, {6, 4, 15}, {6, 4, 18},
	{6, 5, 1}, {6, 5, 2}, {6, 5, 3}, {6, 5, 4}, {6, 5, 15}, {6, 5, 18},
}

var writeBoundaryHeldGolden = [][3]int{
	{1, 5, 16}, {3, 1, 12}, {3, 1, 13}, {3, 1, 14},
}

func tripleID(t [3]int) string {
	return fmt.Sprintf("%02d%02d%02d", t[0], t[1], t[2])
}

// TestWriteBoundaryRows_234AdmittedDeniedAndHeldAbsent pins task-h2's own
// safety-critical counts against the shipped FT710 dialect: exactly 234
// rows, none of the 58 denied or 4 held addresses present among them, and
// every denied/held address individually confirmed absent by id (not
// just by count — a count match alone would not catch one held address
// swapped for one denied one).
func TestWriteBoundaryRows_234AdmittedDeniedAndHeldAbsent(t *testing.T) {
	rows := writeBoundaryRows(cat.FT710)
	if len(rows) != 234 {
		t.Fatalf("writeBoundaryRows(cat.FT710) returned %d rows, want 234", len(rows))
	}

	present := make(map[string]bool, len(rows))
	for _, r := range rows {
		present[r.id] = true
	}

	for _, triple := range writeBoundaryDeniedGolden {
		if present[tripleID(triple)] {
			t.Errorf("denied address %s present in write-boundary dump", tripleID(triple))
		}
	}
	for _, triple := range writeBoundaryHeldGolden {
		if present[tripleID(triple)] {
			t.Errorf("held address %s present in write-boundary dump", tripleID(triple))
		}
	}
}

// TestWriteBoundaryRows_WidthIsSentinelZero pins the "no zero sentinel
// renders as writable" fact this dump exists to carry forward (spec A1):
// every row's width is 0 — table2-write-observed.csv is empty until
// Session W actually runs — and every row's readWidth is a real,
// nonzero manual Digits value.
func TestWriteBoundaryRows_WidthIsSentinelZero(t *testing.T) {
	rows := writeBoundaryRows(cat.FT710)
	for _, r := range rows {
		if r.width != 0 {
			t.Errorf("row %s width = %d, want 0 (table2-write-observed.csv is empty)", r.id, r.width)
		}
		if r.readWidth == 0 {
			t.Errorf("row %s readWidth = 0, want the manual's real Digits value", r.id)
		}
	}
}

// TestWriteBoundaryCSV_RoundTripsAndDeclaresSafetyBoundary pins the CSV
// shape a human or the bench tool actually parses: the header line, one
// row per admitted address, and the safety-boundary prose stated plainly
// in the file (task-h2 brief's own wording requirement).
func TestWriteBoundaryCSV_RoundTripsAndDeclaresSafetyBoundary(t *testing.T) {
	rows := writeBoundaryRows(cat.FT710)
	var buf bytes.Buffer
	if err := writeBoundaryCSV(&buf, rows); err != nil {
		t.Fatalf("writeBoundaryCSV: unexpected error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "ONLY THING KEEPING") {
		t.Errorf("write-boundary CSV header does not state the safety-boundary fact plainly")
	}
	if !strings.Contains(out, writeBoundaryCSVHeader) {
		t.Errorf("write-boundary CSV missing its own column header %q", writeBoundaryCSVHeader)
	}

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	var dataLines int
	for _, l := range lines {
		if l == "" || strings.HasPrefix(l, "#") || l == writeBoundaryCSVHeader {
			continue
		}
		dataLines++
	}
	if dataLines != 234 {
		t.Errorf("write-boundary CSV data lines = %d, want 234", dataLines)
	}

	// One admitted address's shape, spot-checked: 010101 AF TREBLE GAIN,
	// a signed -20..+10 range, no enumerated codes.
	if !strings.Contains(out, "010101,3,0,,-20,10,1,true") {
		t.Errorf("write-boundary CSV missing/wrong row for 010101 (AF TREBLE GAIN); got:\n%s", out)
	}
}

// TestCmdSettings_WriteBoundary_Blackbox exercises the wired CLI
// dispatch end to end via cmdSettings: no --out writes CSV to stdout;
// --out writes the file and a short summary to stdout; a second run
// without --force refuses to overwrite.
func TestCmdSettings_WriteBoundary_Blackbox(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := cmdSettings([]string{"write-boundary"}, &stdout, &stderr)
	if code != exitSuccess {
		t.Fatalf("settings write-boundary (stdout): exit = %d, want %d; stderr=%q", code, exitSuccess, stderr.String())
	}
	if !strings.Contains(stdout.String(), writeBoundaryCSVHeader) {
		t.Errorf("settings write-boundary stdout missing CSV header")
	}

	dir := t.TempDir()
	out := filepath.Join(dir, "boundary.csv")
	stdout.Reset()
	stderr.Reset()
	code = cmdSettings([]string{"write-boundary", "--out", out}, &stdout, &stderr)
	if code != exitSuccess {
		t.Fatalf("settings write-boundary --out: exit = %d, want %d; stderr=%q", code, exitSuccess, stderr.String())
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("settings write-boundary --out: %s was not created: %v", out, err)
	}

	stdout.Reset()
	stderr.Reset()
	code = cmdSettings([]string{"write-boundary", "--out", out}, &stdout, &stderr)
	if code != exitError {
		t.Errorf("settings write-boundary --out (no --force, exists): exit = %d, want %d", code, exitError)
	}
	if !strings.Contains(stderr.String(), "already exists") {
		t.Errorf("settings write-boundary --out (no --force): stderr = %q, want it to mention already existing", stderr.String())
	}

	// An outer flag alongside the reserved first positional is refused,
	// same precedent as diff-observed's own refusal.
	stdout.Reset()
	stderr.Reset()
	code = cmdSettings([]string{"--csv", "x.csv", "write-boundary"}, &stdout, &stderr)
	if code != exitUsage {
		t.Errorf("settings --csv write-boundary: exit = %d, want %d", code, exitUsage)
	}
}
