// SPDX-License-Identifier: GPL-3.0-or-later

package ic905_test

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// evidenceDir is where the IC-905's four quarantine legs live, relative
// to this package's directory (go test's working directory).
const evidenceDir = "testdata"

// frozenEvidenceSHA256 is the freeze. Every file here was produced by a
// fresh agent working from 300/400/600 dpi renders of
// ic905_civ_2.pdf alone, which never opened this repository and never
// saw another leg's output. THE FILES ARE NEVER EDITED AND NEVER
// REGENERATED: a mismatch below is a STOP for orchestrator arbitration
// AGAINST THE PDF, which may correct the profile — never an artefact,
// and never this map.
//
// ALL NINE ARE CLAIMED HERE, deliberately, unlike core/cat/ftdx101,
// where the ledger and witness are frozen by their own tests' content
// checks. Here the ledger, the witness and transcription B are read by
// crosscheck_test.go and geometry_test.go as DATA, and a file whose
// bytes could drift under a test that parses it is not evidence any
// more. One owner, nine files, one hash apiece.
var frozenEvidenceSHA256 = map[string]string{
	"ic905-field-ledger.csv":       "20ba41601a9aed872fbe53b380de43b1dd2a5fcd536170bd79b72dd6186a7439",
	"ic905-field-ledger.md":        "0b57d8214ce2638ad97f03df4d7575b915418cbadfc22727883c95bbeeaaaa6e",
	"ic905-geometry-witness.csv":   "ce358c2d7205ea2054f49908d92af8fbabeb35f745bd4a7e9e30af0f7c7cebb1",
	"ic905-geometry-witness.md":    "39eb62102f7f52b3da5778cd4e4afe4ebe3087fbe6da95233739b754ca08c65d",
	"ic905-transcription-b.csv":    "7384dbe8458477b0f54f6e3aa6f3e8584fb3b8bccdb666d7202baa538737aea0",
	"ic905-transcription-b.md":     "f43bbbaa9b771dffb9ba85b1f0255c18eaaaa842555eac3a303c8c4ef8b9a234",
	"ic905-vectors.golden":         "58b91b6f6a30cd0d7d1435931f43b1e13ab35a798ee96a383cbd6198ad4b8202",
	"ic905-golden-assumptions.csv": "b61cbe9948b67a5ee38abb5ad0339d3b21c6b86537193f0b944f58040c00d32e",
	"ic905-golden-provenance.md":   "81fb113a257830108c7e75dade91efb094d13af0778ba3390dd3f70f8a9ebb4a",
}

func TestEvidenceFrozen(t *testing.T) {
	for name, want := range frozenEvidenceSHA256 {
		path := filepath.Join(evidenceDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading frozen artefact %s: %v", path, err)
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != want {
			t.Errorf("FREEZE BROKEN — %s has changed.\n  recorded SHA-256 %s\n  present  SHA-256 %s\n"+
				"This is quarantined evidence: it is never regenerated and never edited to satisfy a test. "+
				"Restore it and report the change.", path, want, got)
		}
	}

	var seen int
	err := filepath.WalkDir(evidenceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		seen++
		if _, ok := frozenEvidenceSHA256[d.Name()]; !ok {
			t.Errorf("%s has no recorded SHA-256: every file under %s is quarantined evidence and must be frozen by a commit that records its hash", path, evidenceDir)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", evidenceDir, err)
	}
	if seen != len(frozenEvidenceSHA256) {
		t.Errorf("walked %s and found %d files, but the freeze covers %d — an artefact has been moved or added", evidenceDir, seen, len(frozenEvidenceSHA256))
	}
}
