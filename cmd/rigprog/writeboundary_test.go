// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// TestWriteBoundaryRows_234AdmittedDeniedAndHeldAbsent and
// TestWriteBoundaryRows_WidthIsSentinelZero — the row-derivation pins,
// including the 58-denied/4-held golden lists — moved to
// core/driver/ft710/writeboundary_test.go (task h2 guard fix): the
// derivation they pin moved there too, since this package may not import
// core/cat (internal/guards' composition-root discipline).

// TestWriteBoundaryCSV_RoundTripsAndDeclaresSafetyBoundary pins the CSV
// shape a human or the bench tool actually parses: the header line, one
// row per admitted address, and the safety-boundary prose stated plainly
// in the file (task-h2 brief's own wording requirement).
func TestWriteBoundaryCSV_RoundTripsAndDeclaresSafetyBoundary(t *testing.T) {
	rows := wiring.FT710WriteBoundaryRows()
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
