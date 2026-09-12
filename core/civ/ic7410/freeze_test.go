// SPDX-License-Identifier: GPL-3.0-or-later

package ic7410_test

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testdataDir = "testdata"

// frozenSHA256 is the freeze. testdata/ic7410-golden-vectors.md was
// authored ONCE, by this package's own implementer, directly from the
// reviewed capability matrix — SEE its own header for what kind of
// evidence it is (single-source, not an independently blind-transcribed
// leg). No test in this package may modify it, and no failure here is
// fixed by editing it: a disagreement is a bug in golden_test.go's own
// reading, to be fixed by re-reading the matrix.
var frozenSHA256 = map[string]string{
	"ic7410-golden-vectors.md": "40161937c9c6194d15dd8e372d07ce8782b1ae6df2dad39e6224da28275004f3",
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
			t.Errorf("FREEZE BROKEN - %s has changed.\n  recorded SHA-256 %s\n  present  SHA-256 %s",
				path, want, got)
		}
	}

	var seen int
	err := filepath.WalkDir(testdataDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() == "SHA256SUMS" || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		seen++
		if _, ok := frozenSHA256[d.Name()]; !ok {
			t.Errorf("%s carries no recorded SHA-256 - every artefact under %s must be frozen by the commit that lands it", path, testdataDir)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", testdataDir, err)
	}
	if seen != len(frozenSHA256) {
		t.Errorf("walked %s and found %d artefacts, but the freeze covers %d", testdataDir, seen, len(frozenSHA256))
	}
}
