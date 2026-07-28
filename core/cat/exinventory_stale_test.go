// SPDX-License-Identifier: GPL-3.0-or-later

package cat_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/gm5dna/open-rig-programmer/internal/extable"
)

// TestEXInventoryGenerated_NotStale re-derives the generated inventory from
// BOTH of its sources — table2.csv (the manual transcription) and
// table2-observed.csv (the M8c hardware READ observations) — and
// byte-compares the result with the committed exinventory_gen.go. It is the
// CI guard for the generator: CI runs plain `go test ./...` and never
// `go generate`, so without this test an edit to either source that was not
// regenerated — or a hand-edit of the generated file — would ship silently.
// On failure, run `go generate ./core/cat` and commit the result.
func TestEXInventoryGenerated_NotStale(t *testing.T) {
	csv, err := os.ReadFile("table2.csv")
	if err != nil {
		t.Fatalf("reading table2.csv: %v", err)
	}
	rows, err := extable.ParseCSV(extable.FT710Profile(), csv)
	if err != nil {
		t.Fatalf("ParseCSV(table2.csv): %v", err)
	}
	observedCSV, err := os.ReadFile("table2-observed.csv")
	if err != nil {
		t.Fatalf("reading table2-observed.csv: %v", err)
	}
	observed, err := extable.ParseObservedCSV(observedCSV)
	if err != nil {
		t.Fatalf("ParseObservedCSV(table2-observed.csv): %v", err)
	}
	want, err := extable.RenderGo(rows, observed)
	if err != nil {
		t.Fatalf("RenderGo: %v", err)
	}

	got, err := os.ReadFile("exinventory_gen.go")
	if err != nil {
		t.Fatalf("reading exinventory_gen.go: %v", err)
	}

	if !bytes.Equal(got, want) {
		t.Errorf("exinventory_gen.go is stale relative to table2.csv and/or table2-observed.csv; run `go generate ./core/cat` and commit the result (regenerated %d bytes, committed %d bytes)", len(want), len(got))
	}
}
