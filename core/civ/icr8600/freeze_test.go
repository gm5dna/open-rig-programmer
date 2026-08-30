// SPDX-License-Identifier: GPL-3.0-or-later

package icr8600_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	r8600EvidenceDir        = "testdata"
	r8600ManifestName       = "SHA256SUMS"
	r8600ManifestSHA256     = "ae6bd508340ddedf37fda01a332637303a21fc6b290e1dcab547b3436e9968cf"
	r8600ManifestEntryCount = 9
)

func TestEvidenceFreezeManifest(t *testing.T) {
	manifest := loadFreezeManifest(t)
	if len(manifest) != r8600ManifestEntryCount {
		t.Fatalf("freeze manifest has %d entries, want %d", len(manifest), r8600ManifestEntryCount)
	}
	for name, want := range manifest {
		path := filepath.Join(r8600EvidenceDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("FREEZE BROKEN — read %s: %v", path, err)
			continue
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != want {
			t.Errorf("FREEZE BROKEN — %s changed: manifest %s, present %s; report and restore the evidence rather than regenerating it", path, want, got)
		}
	}

	seen := 0
	err := filepath.WalkDir(r8600EvidenceDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		seen++
		if entry.Name() == r8600ManifestName {
			return nil
		}
		if _, ok := manifest[entry.Name()]; !ok {
			t.Errorf("FREEZE BROKEN — %s is unlisted evidence", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk frozen evidence: %v", err)
	}
	if seen != len(manifest)+1 {
		t.Errorf("freeze walk found %d files, want %d manifest entries plus the manifest", seen, len(manifest))
	}
}

func loadFreezeManifest(t *testing.T) map[string]string {
	t.Helper()
	path := filepath.Join(r8600EvidenceDir, r8600ManifestName)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read freeze manifest: %v", err)
	}
	sum := sha256.Sum256(raw)
	if got := hex.EncodeToString(sum[:]); got != r8600ManifestSHA256 {
		t.Fatalf("FREEZE BROKEN — %s itself changed: pinned %s, present %s", path, r8600ManifestSHA256, got)
	}
	if bytes.ContainsRune(raw, '\r') {
		t.Fatalf("FREEZE BROKEN — %s contains a CR", path)
	}
	manifest := make(map[string]string)
	for lineNumber, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
		parts := strings.Split(line, "  ")
		if len(parts) != 2 || len(parts[0]) != sha256.Size*2 || parts[1] == "" || filepath.Base(parts[1]) != parts[1] {
			t.Fatalf("%s line %d is not '<sha256><two spaces><base name>': %q", path, lineNumber+1, line)
		}
		if _, err := hex.DecodeString(parts[0]); err != nil {
			t.Fatalf("%s line %d has invalid SHA-256 %q: %v", path, lineNumber+1, parts[0], err)
		}
		if _, exists := manifest[parts[1]]; exists {
			t.Fatalf("%s lists %q twice", path, parts[1])
		}
		manifest[parts[1]] = parts[0]
	}
	return manifest
}
