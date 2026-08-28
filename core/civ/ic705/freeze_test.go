// SPDX-License-Identifier: GPL-3.0-or-later

package ic705_test

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// evidenceDir is where the four legs' quarantined artefacts live,
// relative to this package's directory (go test's working directory).
const evidenceDir = "testdata"

// frozenSHA256 is every file under testdata/, hashed at the commit that
// landed it. These artefacts are QUARANTINED EVIDENCE: they are never
// regenerated and never edited to satisfy a test. A failure here means
// the file changed — restore it from git and report the change; do not
// update the hash.
//
// # The one deliberate divergence from core/cat/ftdx101's mechanism
//
// The FTdx101 freeze covers a GOLDEN CLASS — the vector files and the
// provenance register that documents them — and lets other files sit in
// testdata/ unfrozen. This manifest covers ALL NINE files, with no class
// predicate at all, because on this model there is no other kind of file
// there: all nine are quarantined evidence, produced by four independent
// legs reading one PDF, NONE of them generated, and none of the three
// consumer tests (crosscheck, geometry, golden) pins bytes of its own.
// The CSVs are load-bearing in exactly the way the .golden file is — the
// layout's every offset is checked against `geometry-witness.csv`'s
// measurements — so a class predicate here would freeze the least of the
// evidence and leave the rest editable by whoever found a test
// inconvenient.
//
// The nine hashes were computed from the scratchpad artefacts on
// 24/08/2026 and re-verified in the worktree, byte for byte, before this
// test was written. The files were renamed on the way in (the leg prefix
// `ic705-` dropped, `ic705-golden-provenance.md` → `provenance.md`); a
// rename touches no content, and the hashes are the proof of that.
var frozenSHA256 = map[string]string{
	"field-ledger.csv":       "a0af2bcd9a758e6486c42616b1302e25b87ea1a7b991193a08dbc64a76b57033",
	"field-ledger.md":        "7b9ded6dc7e5e58cc2fd12c96d264121c49d1cb2a0dbd1fef10465e7ff457ce7",
	"geometry-witness.csv":   "49544c5a13ca39ec81b1719faaa93ad5641c489f8c38ae43b0d25e71608384f9",
	"geometry-witness.md":    "9993c1a08bbd215601fdb07c5320bd5559da7a61cfe91ec8a0413e8a9e9dd66a",
	"transcription-b.csv":    "d1bdcd77aecaeaf1c9704007290a4b9fabeefc892082cc298d7d5ece2f02a47b",
	"transcription-b.md":     "440550340427478bf2b2f7d80f049f6621c0c1f6b280e7777562d443ae6eb376",
	"vectors.golden":         "846d738ca2ebd715f8058b056cfe660a070af0bea47ec25fb02fb14b33f6e14d",
	"golden-assumptions.csv": "189791eabb194d0fb8c8f64eb342e58de5f9b3f4e4f791d64502817355a0a5e7",
	"provenance.md":          "ece9203486630977ce81b301963b9d1645485738fc20b0cb338888649e45e4e0",
}

// TestEvidenceFrozen has three clauses, and the second and third are the
// ones that catch the interesting cases.
//
//  1. Every recorded hash matches the file on disk — an EDITED artefact
//     fails.
//  2. A filepath.WalkDir over testdata/ — a WALK, not a glob, so a file
//     smuggled into a subdirectory is caught rather than sitting
//     unnoticed one level down — finds no file absent from the map.
//  3. The walked count equals len(frozenSHA256), so a frozen file that
//     was MOVED (or deleted) fails as loudly as one that was edited.
//     Clause 1 alone would not catch a move: it reads by name and would
//     merely report a missing file if the name survived, and nothing at
//     all if the map were trimmed to match.
func TestEvidenceFrozen(t *testing.T) {
	for name, want := range frozenSHA256 {
		path := filepath.Join(evidenceDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("reading frozen artefact %s: %v", path, err)
			continue
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != want {
			t.Errorf("FREEZE BROKEN — %s has changed since the commit that landed it.\n"+
				"  recorded SHA-256 %s\n"+
				"  present  SHA-256 %s\n"+
				"This artefact is QUARANTINED EVIDENCE: it is never regenerated and never\n"+
				"edited to satisfy a test. Restore it with `git checkout -- %s` and report\n"+
				"the change to the orchestrator, who arbitrates against the PDF. Do NOT\n"+
				"update the hash.",
				path, want, got, path)
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
		if _, ok := frozenSHA256[d.Name()]; !ok {
			t.Errorf("%s is under %s with no recorded SHA-256: every file there is quarantined "+
				"evidence and must be frozen by the commit that lands it",
				path, evidenceDir)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", evidenceDir, err)
	}
	if seen != len(frozenSHA256) {
		t.Errorf("walked %s and found %d files, but the freeze covers %d — a frozen artefact "+
			"has been moved, renamed or removed", evidenceDir, seen, len(frozenSHA256))
	}
}
