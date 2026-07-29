// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx10_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/gm5dna/open-rig-programmer/internal/extable"
)

// TestEXInventoryGenerated_NotStale re-derives this package's generated
// inventory from its ONE source — table2.csv, the manual transcription — and
// byte-compares the result with the committed exinventory_gen.go. It is the
// CI guard for the generator: CI runs plain `go test ./...` and never
// `go generate`, so without this test an edit to table2.csv that was not
// regenerated, or a hand-edit of the generated file, would ship silently. On
// failure, run `go generate ./core/cat/ftdx10` and commit the result.
//
// The profile is selected FROM THE REGISTRY BY PACKAGE, not by the lookup
// name "ftdx10" and not from a literal in this file. A test that hardcoded
// its own profile would be re-deriving from a second copy of the facts and
// could agree with a generated file the real registration disagrees with —
// the "bound consulted from one place with its datum taken from another"
// shape that recurred four times across M9b. Selecting by Package also makes
// the test refuse rather than guess if the registry ever gains a second
// entry emitting into this package: exactly one registration may own it.
//
// Scope is deliberately package-local, not registry-wide. Profile carries no
// package-directory datum and its paths are resolved relative to the working
// directory, so no single test can verify every profile's files; core/cat's
// own staleness test covers the FT-710 unchanged, and the two package-local
// tests together cover both profiles.
func TestEXInventoryGenerated_NotStale(t *testing.T) {
	var matches []extable.NamedProfile
	for _, np := range extable.RegisteredProfiles() {
		if np.Profile.Package == "ftdx10" {
			matches = append(matches, np)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("registry holds %d profiles emitting into package ftdx10, want exactly 1: %v", len(matches), names(matches))
	}
	p := matches[0].Profile

	csv, err := os.ReadFile(p.ManualCSV)
	if err != nil {
		t.Fatalf("reading %s: %v", p.ManualCSV, err)
	}
	rows, err := extable.ParseCSV(p, csv)
	if err != nil {
		t.Fatalf("ParseCSV(%s): %v", p.ManualCSV, err)
	}
	// ObservationsAbsent: RenderGo requires the observation map to be EMPTY
	// rather than partial, so nil is the correct argument and not a stand-in
	// for an unread file.
	want, err := extable.RenderGo(p, rows, nil)
	if err != nil {
		t.Fatalf("RenderGo: %v", err)
	}

	got, err := os.ReadFile(p.OutFile)
	if err != nil {
		t.Fatalf("reading %s: %v", p.OutFile, err)
	}

	if !bytes.Equal(got, want) {
		t.Errorf("%s is stale relative to %s; run `go generate ./core/cat/ftdx10` and commit the result (regenerated %d bytes, committed %d bytes)", p.OutFile, p.ManualCSV, len(want), len(got))
	}
}

// names renders a profile set for a failure message.
func names(ps []extable.NamedProfile) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.Name)
	}
	return out
}
