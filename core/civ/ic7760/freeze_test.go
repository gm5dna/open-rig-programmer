// SPDX-License-Identifier: GPL-3.0-or-later

package ic7760_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEvidenceFreeze(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	entries := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(entries) != 9 {
		t.Fatalf("SHA256SUMS has %d entries, want 9 frozen artefacts", len(entries))
	}
	for _, line := range entries {
		parts := strings.Fields(line)
		if len(parts) != 2 {
			t.Fatalf("malformed SHA256SUMS entry %q", line)
		}
		data, err := os.ReadFile(filepath.Join("testdata", parts[1]))
		if err != nil {
			t.Fatalf("reading frozen %s: %v", parts[1], err)
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != parts[0] {
			t.Errorf("frozen evidence %s changed: got %s want %s", parts[1], got, parts[0])
		}
	}
}
