// SPDX-License-Identifier: GPL-3.0-or-later

package ic7300_test

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

const evidenceDir = "testdata"

// frozenSHA256 is every quarantined artefact this package carries, with the
// SHA-256 it had at the commit that landed it. These are NEVER regenerated
// and NEVER edited to satisfy a test: they are the manual-derived evidence
// the profile is checked against, and a changed byte is a finding, not a fix.
var frozenSHA256 = map[string]string{
	"ic7300-field-ledger.csv":       "93521e3b460eaf4d953fb2380ccc3b6d42611f4073e3fdd7f1e2e35a4560ce2e",
	"ic7300-field-ledger.md":        "874f5431f05bbd38b2f41ea3697ddd19e8b6d623c099c65b420fe7375c16cd6d",
	"ic7300-geometry-witness.csv":   "2c47de354398b1b34bcc7ef58f6f29a86fac4cf604c13a13a15ba5a9d264eb20",
	"ic7300-geometry-witness.md":    "355bba9eb544b00af5d18a267d8f6ba3b0a91577bcdfde8a6ea989ea3e2f66d1",
	"ic7300-transcription-b.csv":    "ab03862e61add2843bd2d3499abd1bbb871de7161f23682fdc4a6648f1ec2010",
	"ic7300-transcription-b.md":     "0351bca83663819d804530cd244922e8b7b0af9890e1c5f3270938da880c7a2d",
	"ic7300-vectors.golden":         "c18ce59dcc0ecd0b1bc284437c00eea44f5f397aad001a8d51a11d0cebf0e71b",
	"ic7300-golden-assumptions.csv": "0ba2980278de312aa65366ae7007be331f0d001bfa5d7e0e321d80707c608f65",
	"ic7300-golden-provenance.md":   "3ea93962cb4e99ad17b7a7e194609ddc423c61f9ae216c67c5e2e18124640150",
}

func TestEvidenceFrozen(t *testing.T) {
	for name, want := range frozenSHA256 {
		path := filepath.Join(evidenceDir, name)
		b, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("FREEZE BROKEN — %s: %v", path, err)
			continue
		}
		sum := sha256.Sum256(b)
		if got := hex.EncodeToString(sum[:]); got != want {
			t.Errorf("FREEZE BROKEN — %s has changed since the commit that landed it.\n"+
				"  recorded SHA-256 %s\n  present  SHA-256 %s\n"+
				"This artefact is quarantined evidence: restore it with\n"+
				"  git checkout <landing-commit> -- %s\n"+
				"and report the change. It is never regenerated and never edited.", path, want, got, path)
		}
	}

	// The reverse half: a WALK, not a glob, so an artefact smuggled into a
	// subdirectory is caught rather than skipped.
	seen := 0
	err := filepath.WalkDir(evidenceDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		seen++
		if _, ok := frozenSHA256[d.Name()]; !ok {
			t.Errorf("%s is in %s but not in the freeze — a new artefact has been added without being frozen", p, evidenceDir)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", evidenceDir, err)
	}
	if seen != len(frozenSHA256) {
		t.Errorf("walked %s and found %d artefacts, but the freeze covers %d — one has been moved, renamed or added",
			evidenceDir, seen, len(frozenSHA256))
	}
}
