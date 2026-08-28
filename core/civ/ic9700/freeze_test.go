// SPDX-License-Identifier: GPL-3.0-or-later

package ic9700_test

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

const evidenceDir = "testdata"

// frozenSHA256 is every file under testdata/, by name. Transcribed into
// this task's commit message; the ENFORCEMENT is this test, not the
// message.
//
// The nine files are the four evidence legs the IC-9700's Wave-1
// transcription produced — L the field ledger, W the geometry witness, B
// the independent transcription, G the golden vectors and their
// provenance — moved here from quarantine and frozen. FROM HERE ON ANY
// CHANGE TO ONE OF THEM IS A STOP, arbitrated against the PDF and
// recorded, never a quiet edit.
var frozenSHA256 = map[string]string{
	"ic9700-field-ledger.csv":       "25dda795e19e794408f4aea6ecc6921badcd1f33dff4a981c5c17ab759e2d93f",
	"ic9700-field-ledger.md":        "2a9e1b1349f0c9b834bf7fa6993931fe9e81d407072193b714d34a065b7742b8",
	"ic9700-geometry-witness.csv":   "ddabf691d996430796070097a9900d4ce82ad4d68524e99fbfa2e7d933b569c0",
	"ic9700-geometry-witness.md":    "1c77356b9082cfeb6d80bddb1a54abb93e3aa083f7859577320ac56f22d1cb70",
	"ic9700-transcription-b.csv":    "5c29e2b89cbfcea2944e2559009b12688da6084d5d0d88148d1be31942c502be",
	"ic9700-transcription-b.md":     "c8327d24618b3c9dc93c3d87741d01c61331e937fd63c457795108e2e829f417",
	"ic9700-vectors.golden":         "45b45aea27c6dbd077075a5e472d90c45b265034a2f84cbc76044fb765543aa6",
	"ic9700-golden-assumptions.csv": "b891dc59b21b42eb276f88f576189e84b50ffb751c0678d5927eb8572716e5d4",
	"ic9700-golden-provenance.md":   "76118078b4627f586839146ec1f4c4e7bb014e37289b62b47206e5d0199c6180",
}

// TestEvidenceArtefactsFrozen walks testdata/ RECURSIVELY and holds every
// file it finds to the map above.
//
// The walk is recursive rather than a glob because a glob sees one
// directory level: a subdirectory of smuggled artefacts would be
// invisible to it. And the map is checked in BOTH directions — a file
// present but unlisted fails, and a listed file that has gone missing
// fails on the count — because either alone can be satisfied by an empty
// directory.
func TestEvidenceArtefactsFrozen(t *testing.T) {
	seen := 0
	err := filepath.WalkDir(evidenceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(evidenceDir, path)
		if relErr != nil {
			return relErr
		}
		want, ok := frozenSHA256[rel]
		if !ok {
			t.Errorf("%s is under testdata/ but not in frozenSHA256 — a smuggled artefact, or a freeze that was never recorded", rel)
			return nil
		}
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(b)); got != want {
			t.Errorf("%s: sha256 = %s, want %s — a quarantined artefact has changed", rel, got, want)
		}
		seen++
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if seen != len(frozenSHA256) {
		t.Errorf("walked %d files, frozenSHA256 has %d — a frozen artefact is missing", seen, len(frozenSHA256))
	}
}
