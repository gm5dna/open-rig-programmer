// SPDX-License-Identifier: GPL-3.0-or-later

package ic7200_test

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// frozenSHA256 pins this package's authored testdata: the golden vectors
// (golden_test.go), authored from matrix §3.10/§3.11/§3.12 rather than
// captured from a radio — no IC-7200 has ever been connected to this
// project (matrix §0).
var frozenSHA256 = map[string]string{
	"IC-7200-vectors.golden": "7f6ac7ca2e4edb6057331c220e4961c266d4450f1dd4d9a9bfa4ac97132beb6f",
}

func TestEvidenceFrozen(t *testing.T) {
	for name, want := range frozenSHA256 {
		data, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		got := hex.EncodeToString(sum[:])
		if got != want {
			t.Errorf("frozen evidence %s changed: got %s want %s", name, got, want)
		}
	}
	seen := 0
	err := filepath.WalkDir("testdata", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !(strings.HasSuffix(d.Name(), ".csv") || strings.HasSuffix(d.Name(), ".md") || strings.HasSuffix(d.Name(), ".golden")) {
			return nil
		}
		seen++
		if _, ok := frozenSHA256[d.Name()]; !ok {
			t.Errorf("unmanifested evidence %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen != len(frozenSHA256) {
		t.Errorf("evidence count = %d, manifest = %d", seen, len(frozenSHA256))
	}
}
