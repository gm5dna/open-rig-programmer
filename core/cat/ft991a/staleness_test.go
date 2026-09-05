// SPDX-License-Identifier: GPL-3.0-or-later

package ft991a_test

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
// failure, run `go generate ./core/cat/ft991a` and commit the result.
//
// THE PROFILE IS SELECTED BY LOOKUP NAME, and that is a DEPARTURE from the
// three sibling staleness tests, which select by Package. The reason is that
// the lookup name is not a second copy of the fact here — it is the SAME
// TOKEN the generator is given. exinventory.go's directive reads
// `gen -profile ft991a`, so Lookup("ft991a") returns exactly the profile
// `go generate ./core/cat/ft991a` used; selecting by Package instead would
// let this test agree with a file the real generation never produced, if the
// two ever named different registrations. The siblings' argument against a
// name literal — that it is a second copy of the facts — applies to a
// profile assembled in the test, which this is not.
//
// The Package check below is therefore a guard, not the selection: this
// test's paths are p.ManualCSV and p.OutFile resolved against the working
// directory, which is this package's directory, so a registration named
// ft991a that emitted somewhere else would be comparing the wrong files.
//
// Scope is deliberately package-local. Profile carries no package-directory
// datum and its paths are resolved relative to the working directory, so no
// single test can verify every profile's files; core/cat's and the three
// other model packages' own staleness tests cover the rest.
func TestEXInventoryGenerated_NotStale(t *testing.T) {
	p, ok := extable.Lookup("ft991a")
	if !ok {
		t.Fatal(`extable.Lookup("ft991a") failed; the FT-991A must be registered`)
	}
	if p.Package != "ft991a" {
		t.Fatalf("profile %q emits into package %q, want ft991a — this test resolves its paths against THIS package's directory", p.Model, p.Package)
	}

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
		t.Errorf("%s is stale relative to %s; run `go generate ./core/cat/ft991a` and commit the result (regenerated %d bytes, committed %d bytes)", p.OutFile, p.ManualCSV, len(want), len(got))
	}
}
