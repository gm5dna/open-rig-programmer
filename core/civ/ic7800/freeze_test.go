// SPDX-License-Identifier: GPL-3.0-or-later

package ic7800_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testdataDir is where the IC-7800's evidence lives, relative to this
// package's directory (go test's working directory).
const testdataDir = "testdata"

// TestEvidenceFreeze pins every evidence artefact under testdataDir against
// a manifest, testdata/SHA256SUMS, generated when each file was landed.
//
// NO TEST IN THIS PACKAGE MAY MODIFY AN ARTEFACT, and no failure here is
// ever fixed by editing one: a disagreement between an artefact and the
// codec is a STOP for orchestrator arbitration AGAINST THE PDF, never a
// fixed artefact.
//
// THREE FILES, NOT NINE. This model's matrix was authored from a single
// reading of one document (matrix §0), not the IC-7610 lineage's four
// independently-blind legs, so there is no field-ledger/geometry-witness
// pair to freeze separately from the transcription they would otherwise
// cross-check: testdata/ic7800-transcription.csv (the S1 evidence sweep's
// own field-by-field table), testdata/ic7800-vectors.golden and
// testdata/ic7800-golden-provenance.md are the whole of it.
func TestEvidenceFreeze(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(testdataDir, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	entries := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(entries) != 3 {
		t.Fatalf("SHA256SUMS has %d entries, want 3 frozen artefacts", len(entries))
	}
	for _, line := range entries {
		parts := strings.Fields(line)
		if len(parts) != 2 {
			t.Fatalf("malformed SHA256SUMS entry %q", line)
		}
		data, err := os.ReadFile(filepath.Join(testdataDir, parts[1]))
		if err != nil {
			t.Fatalf("reading frozen %s: %v", parts[1], err)
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != parts[0] {
			t.Errorf("frozen evidence %s changed: got %s want %s", parts[1], got, parts[0])
		}
	}
}
