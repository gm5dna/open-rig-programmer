// SPDX-License-Identifier: GPL-3.0-or-later

package ft891_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat/ft891"
	"github.com/gm5dna/open-rig-programmer/internal/extable"
)

// TestEXInventoryGenerated_NotStale re-derives this package's generated
// inventory from its ONE source — table2.csv, the manual transcription — and
// byte-compares the result with the committed exinventory_gen.go. It is the
// CI guard for the generator: CI runs plain `go test ./...` and never
// `go generate`, so without this test an edit to table2.csv that was not
// regenerated, or a hand-edit of the generated file, would ship silently. On
// failure, run `go generate ./core/cat/ft891` and commit the result.
//
// The profile is selected FROM THE REGISTRY BY PACKAGE, not by the lookup
// name "ft891" and not from a literal in this file. A test that hardcoded its
// own profile would be re-deriving from a second copy of the facts and could
// agree with a generated file the real registration disagrees with — the
// "bound consulted from one place with its datum taken from another" shape
// that recurred four times across M9b. Selecting by Package also makes the
// test refuse rather than guess if the registry ever gains a second entry
// emitting into this package: exactly one registration may own it.
//
// Scope is deliberately package-local, not registry-wide. Profile carries no
// package-directory datum and its paths are resolved relative to the working
// directory, so no single test can verify every profile's files; core/cat's
// and the two FTdx packages' own staleness tests cover the other three
// profiles unchanged, and the four package-local tests together cover all
// four.
func TestEXInventoryGenerated_NotStale(t *testing.T) {
	var matches []extable.NamedProfile
	for _, np := range extable.RegisteredProfiles() {
		if np.Profile.Package == "ft891" {
			matches = append(matches, np)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("registry holds %d profiles emitting into package ft891, want exactly 1: %v", len(matches), names(matches))
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
		t.Errorf("%s is stale relative to %s; run `go generate ./core/cat/ft891` and commit the result (regenerated %d bytes, committed %d bytes)", p.OutFile, p.ManualCSV, len(want), len(got))
	}
}

// TestEXItems_ReturnsACopy pins the one claim exinventory.go's accessor
// makes beyond returning the inventory: that a caller cannot reach the
// generated slice through it. The accessor exists only because this package
// has no dialect.go yet, so it is the sole route to exItems and a caller
// that mutated what it got back would corrupt every later reader in the
// process.
func TestEXItems_ReturnsACopy(t *testing.T) {
	first := ft891.EXItems()
	if len(first) == 0 {
		t.Fatal("EXItems() is empty — the mutation below would prove nothing")
	}
	first[0].Name = "MUTATED"
	if again := ft891.EXItems(); again[0].Name == "MUTATED" {
		t.Error("EXItems() shares its backing array with the generated inventory")
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
