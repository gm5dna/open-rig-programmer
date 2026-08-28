// SPDX-License-Identifier: GPL-3.0-or-later

package ic7300mk2_test

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
	"ic7300mk2-field-ledger.csv":       "55d860ddadb152c9ab92979926305b4f0748bf98ad7340130547ff01561cc0ad",
	"ic7300mk2-field-ledger.md":        "16cb8f79d341b6d67333bf824e19c24c749661044794ad85db824c5d824493e1",
	"ic7300mk2-geometry-witness.csv":   "0bf7f020553004429fe466db1f88f418c17e33cc9002e12dd1e0358e2362adb4",
	"ic7300mk2-geometry-witness.md":    "93e616b383f4e95281c4404d3f0c103cece0498623956a7b57fe410b1bf41516",
	"ic7300mk2-transcription-b.csv":    "8b4068d05b2960c18113944fa3ebaef6d0f4655e4376eade6a66303f0d99021a",
	"ic7300mk2-transcription-b.md":     "738b2c6e506d8cb3a5eaea0406b9a1b0b368e6a344b8ecff9cac297b2ad7ab0a",
	"ic7300mk2-vectors.golden":         "72f2e0fa54d1d49b597c8ccf0c621ec463290d79cbccf1887133879f059a8ce1",
	"ic7300mk2-golden-assumptions.csv": "71d3fa8c1d70cd1a6b162adc16144f593bfced376773b19130be1b9920609531",
	"ic7300mk2-golden-provenance.md":   "78dc014aacf9d1a3222d46ca087e8f0f288c569da262d32a3b92be927a612ec0",
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
