// SPDX-License-Identifier: GPL-3.0-or-later

package ic7610_test

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testdataDir is where the IC-7610's quarantined Wave-2 evidence lives,
// relative to this package's directory (go test's working directory).
const testdataDir = "testdata"

// frozenSHA256 is the freeze. Every artefact under testdataDir was produced
// by a quarantined agent that never opened this repository, from 300-500 dpi
// renders of the IC-7610 CI-V Reference Guide rev 4 alone. NO TEST IN THIS
// PACKAGE MAY MODIFY ONE, and no failure here is ever fixed by editing one:
// a disagreement between an artefact and the codec is a STOP for
// orchestrator arbitration AGAINST THE PDF.
//
// The hashes below were computed from the files exactly as the orchestrator
// landed them in the tree, and they are repeated in this commit's message so
// the freeze survives a rewritten history.
var frozenSHA256 = map[string]string{
	"ic7610-field-ledger.md":        "9e60ab49ce91b71a00bbda083da04dd5bab2d508ed7a73a0bf91b70e18688238",
	"ic7610-field-ledger.csv":       "a01b33aa18057f3339c5a7cab85a46d13411e2ed7da3bd568de338a7d83e92e7",
	"ic7610-geometry-witness.md":    "2f53ae0903a105474cab5b81b8d677bec0630c4d40deacdcfaf35566ef02e80c",
	"ic7610-geometry-witness.csv":   "46118550b89f0a4f993b772c951a835e1d11ea3cd47d6c04c287e89bc8c761cf",
	"ic7610-transcription-b.md":     "533343b369c848976c45e921f644e59fbf575dd41b1062febe3427aa80b8c84c",
	"ic7610-transcription-b.csv":    "b77d78569e8aa89ae9ac87350a31fd9790eaada0adc58ae728db48654e178cfd",
	"ic7610-vectors.golden":         "ecd1de927cef7eaa2e785d087a2e7b8f8eb7602c13b74f8bba0ceb5498febe01",
	"ic7610-golden-assumptions.csv": "86fab62070ec45557133f77b778962e9b6e6823715542afa559f9b39d3e39f62",
	"ic7610-golden-provenance.md":   "0498c7b84c6bf1d7854195ad5b26753bffec3043830b410a85256083ec1a7682",
}

// isFrozenClass reports whether a file under testdataDir belongs to the
// quarantined set. EVERYTHING does, on this model: unlike core/cat/ftdx101,
// whose testdata/ mixes quarantined vectors with a task-owned transcription A,
// every file here came out of a Wave-2 quarantine leg. So the predicate is
// deliberately total, and the walk below then requires the map to cover the
// whole directory rather than a subset somebody could quietly grow.
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
				"This artefact is quarantined evidence: it is never regenerated and never\n"+
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
			t.Errorf("%s is quarantined evidence with no recorded SHA-256 - every artefact under %s "+
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
