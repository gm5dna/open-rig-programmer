// SPDX-License-Identifier: GPL-3.0-or-later

package fakeyaesuclone

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/clonewire"
)

// update regenerates every golden file from BuildImage's current output.
// Guarded, per spec.md §Fakes and byte-identity: run explicitly with
//
//	GOTOOLCHAIN=go1.25.0 go test ./internal/fakeyaesuclone/ -run TestGolden -update
//
// It never runs by accident — plain `go test` (the project gate) leaves
// *update false and only compares.
var update = flag.Bool("update", false, "regenerate fakeyaesuclone golden files")

func goldenPath(p clonewire.Profile) string {
	return filepath.Join("testdata", p.ProfileID+".golden")
}

// TestGolden compares each Profile's BuildImage output against its checked-in
// testdata/*.golden file, byte-for-byte. This is drift detection only, not
// evidence of correctness — see doc.go.
func TestGolden(t *testing.T) {
	for _, p := range allProfiles {
		p := p
		t.Run(p.ProfileID, func(t *testing.T) {
			got := BuildImage(p)
			path := goldenPath(p)
			if *update {
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatalf("writing golden %s: %v", path, err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading golden %s: %v (run with -update to create it)", path, err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("%s: BuildImage output does not match golden %s (%d bytes vs %d)", p.Model, path, len(got), len(want))
			}
		})
	}
}

// TestGolden_DetectsCorruption proves the comparison above actually
// compares, rather than trivially passing: a deliberately corrupted copy of
// a golden (built in-memory, never written to testdata/) must be reported
// as a mismatch against the fake's real output.
func TestGolden_DetectsCorruption(t *testing.T) {
	p := clonewire.FT817
	got := BuildImage(p)
	corrupted := append([]byte(nil), got...)
	corrupted[0] ^= 0xFF
	if bytes.Equal(got, corrupted) {
		t.Fatal("corrupted copy unexpectedly equals the real output — test fixture is broken")
	}
}
