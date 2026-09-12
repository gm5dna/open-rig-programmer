// SPDX-License-Identifier: GPL-3.0-or-later

package ic7600_test

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testdataDir is where the IC-7600's evidence lives, relative to this
// package's directory (go test's working directory).
const testdataDir = "testdata"

// frozenSHA256 is the freeze. NO TEST IN THIS PACKAGE MAY MODIFY AN
// ARTEFACT under testdataDir, and no failure here is ever fixed by editing
// one: a disagreement between an artefact and the codec is a STOP for
// orchestrator arbitration AGAINST THE PDF.
var frozenSHA256 = map[string]string{
	"ic7600-field-ledger.md":        "2b1154a5d8872132093294b540bec55f43e6abde3a90e55255d9c8663d37264f",
	"ic7600-field-ledger.csv":       "31ac06b3196558832176bc93da7180d019547b5e0da1f047af8e12f1150e8796",
	"ic7600-geometry-witness.md":    "16902b920beeec0e101362bac030cf18f1b5a97020b4696b3a61eb01be325294",
	"ic7600-geometry-witness.csv":   "3940ec88201ef934744ef11bc7a72074a2a920584b8f0372558ff9d8cf9dd865",
	"ic7600-transcription-b.md":     "d5aeb76a15043f6b2034f3ccf0f3adbd79c4464bddca6188a2cbfe6ce33d530b",
	"ic7600-transcription-b.csv":    "7643a78d277db1d21bb16e280bcb30b3a5c7cb34be1972301d0adc7abd89c414",
	"ic7600-vectors.golden":         "153dc7f70c17c3233eeb212b271e62f15a5b4417b149ad3c4a422f77bd9c7208",
	"ic7600-golden-assumptions.csv": "7bc3134e071e0c2ce03da094d16b1cfaf8a5ab426032c412832485111efce07b",
	"ic7600-golden-provenance.md":   "b98f3ed21b5d865b168e74fdf426169866c3d6aad91989ee2d0042041215416c",
}

// isFrozenClass reports whether a file under testdataDir belongs to the
// frozen set. EVERYTHING does: the predicate is deliberately total, and
// the walk below requires the map to cover the whole directory.
func isFrozenClass(name string) bool {
	return strings.HasSuffix(name, ".golden") ||
		strings.HasSuffix(name, ".csv") ||
		strings.HasSuffix(name, ".md")
}

func TestEvidenceFrozen(t *testing.T) {
	for name, want := range frozenSHA256 {
		path := filepath.Join(testdataDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading frozen artefact %s: %v", path, err)
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != want {
			t.Errorf("FREEZE BROKEN - %s has changed.\n"+
				"  recorded SHA-256 %s\n"+
				"  present  SHA-256 %s\n"+
				"This artefact is frozen evidence: it is never regenerated and never\n"+
				"edited to satisfy a test. Restore it and report the change.",
				path, want, got)
		}
	}

	var seen int
	err := filepath.WalkDir(testdataDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !isFrozenClass(d.Name()) {
			return nil
		}
		seen++
		if _, ok := frozenSHA256[d.Name()]; !ok {
			t.Errorf("%s is frozen evidence with no recorded SHA-256 - every artefact under %s "+
				"must be frozen by the commit that lands it", path, testdataDir)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", testdataDir, err)
	}
	if seen != len(frozenSHA256) {
		t.Errorf("walked %s and found %d artefacts, but the freeze covers %d - one has been moved or added",
			testdataDir, seen, len(frozenSHA256))
	}
}
