// SPDX-License-Identifier: GPL-3.0-or-later

package ic7700_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFreeze_TestdataMatchesManifest hashes every evidence artefact this
// package's tests read and compares it against testdata/SHA256SUMS. No
// test in this package may modify one of these files: a disagreement
// between an artefact and the codec is a STOP for arbitration against the
// matrix/manual, never a fixed artefact — and this test is what makes a
// silent artefact edit visible instead of merely re-graded green.
func TestFreeze_TestdataMatchesManifest(t *testing.T) {
	files := []string{"IC-7700-vectors.golden"}
	sums := readManifest(t, filepath.Join("testdata", "SHA256SUMS"))
	if len(sums) != len(files) {
		t.Fatalf("SHA256SUMS lists %d entries, this test names %d — the two have drifted", len(sums), len(files))
	}
	for _, name := range files {
		want, ok := sums[name]
		if !ok {
			t.Fatalf("SHA256SUMS has no entry for %s", name)
		}
		raw, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		sum := sha256.Sum256(raw)
		got := hex.EncodeToString(sum[:])
		if got != want {
			t.Errorf("%s hashes to %s, manifest says %s — the file changed since the manifest was written", name, got, want)
		}
	}
}

func readManifest(t *testing.T, path string) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	out := map[string]string{}
	for _, line := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			t.Fatalf("%s: malformed line %q, want <sha256>  <filename>", path, line)
		}
		out[fields[1]] = fields[0]
	}
	return out
}
