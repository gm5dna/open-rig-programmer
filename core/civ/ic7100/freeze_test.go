// SPDX-License-Identifier: GPL-3.0-or-later

package ic7100

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// frozenSHA256 includes the L/W/B/G artefacts and their supplied manifest.
// It is literal rather than derived from SHA256SUMS: deriving the expected
// digest from a file beside the subject would let both drift together.
var frozenSHA256 = map[string]string{
	"IC-7100-field-ledger.csv":       "551f60d4c6d0cd182039928c2f658d023601a5a6dba02ef9072458fb4bfef5cf",
	"IC-7100-field-ledger.md":        "371254b034b91d2991d0375d3dabd7a1b9168f9062a8ff9f3e43e5c81d4c94eb",
	"IC-7100-geometry-witness.csv":   "bc21de0debb394bf4c56db6d9e1a5d5427a2d8815414d3e0652679e93d00c713",
	"IC-7100-geometry-witness.md":    "eb5bb5549e0fa89ccd75b8f5badf1f50dbbcf534ea0cfdd7dc5d617500c87c19",
	"IC-7100-golden-assumptions.csv": "4b906981cea1d2d08f5f4af8f5e13141af5c8787945829b7fb7ac00a09bd6552",
	"IC-7100-golden-provenance.md":   "90a91367414cb69822b3788c2b04542984f0ee22364b5ab60e77589ba242b06d",
	"IC-7100-transcription-b.csv":    "55876ef1abcf58297f7fa37e689a978b044984232f2246fbafae35533e747873",
	"IC-7100-transcription-b.md":     "20bc40669a747d229c89cec363186b3b6edf2cd399bd72fbb4fb81b56ee46fbe",
	"IC-7100-vectors.golden":         "348fd3a8b24464d0152dce857bbad60069c891ae2a1144488593ec9146a0649c",
	"SHA256SUMS":                     "dabcf082c64e2d75fd9930d5b5b4fcde0d108f7983ce01d101b211aa2525a8b8",
}

func TestEvidenceFrozen(t *testing.T) {
	seen := 0
	err := filepath.WalkDir("testdata", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel("testdata", path)
		if err != nil {
			return err
		}
		want, ok := frozenSHA256[rel]
		if !ok {
			t.Errorf("unfrozen evidence file %q", rel)
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(contents)); got != want {
			t.Errorf("%s sha256 = %s, want %s — frozen evidence changed", rel, got, want)
		}
		seen++
		return nil
	})
	if err != nil {
		t.Fatalf("walk frozen evidence: %v", err)
	}
	if seen != len(frozenSHA256) {
		t.Errorf("walked %d evidence files, literal freeze has %d", seen, len(frozenSHA256))
	}
}
