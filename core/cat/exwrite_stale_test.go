// SPDX-License-Identifier: GPL-3.0-or-later

package cat_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/gm5dna/open-rig-programmer/internal/extable"
)

// TestExWriteGenerated_NotStale is exinventory_stale_test.go's shape for the
// FT-710 write-descriptor table (task b2): re-derive exwrite_gen.go from
// its two sources and byte-compare. On the ft991a staleness test's own
// precedent (core/cat/ft991a/staleness_test.go:47-80) the profile is
// selected by LOOKUP NAME, "ft710write" — the same token exinventory.go's
// second `//go:generate` directive passes — rather than reassembled here,
// so this test agrees with a file the real generation actually produced.
//
// CI runs plain `go test ./...` and never `go generate`, so without this
// test a source edit that was not regenerated — or a hand-edit of
// exwrite_gen.go — would ship silently, and here that would authorise a
// live Set on stale evidence. On failure, run `go generate ./core/cat` and
// commit the result.
func TestExWriteGenerated_NotStale(t *testing.T) {
	p, ok := extable.Lookup("ft710write")
	if !ok {
		t.Fatal(`extable.Lookup("ft710write") failed; the FT-710 write-descriptor table must be registered`)
	}
	if p.Package != "cat" {
		t.Fatalf("profile %q emits into package %q, want cat — this test resolves its paths against THIS package's directory", p.Model, p.Package)
	}

	csv, err := os.ReadFile(p.ManualCSV)
	if err != nil {
		t.Fatalf("reading %s: %v", p.ManualCSV, err)
	}
	rows, err := extable.ParseCSV(p, csv)
	if err != nil {
		t.Fatalf("ParseCSV(%s): %v", p.ManualCSV, err)
	}

	writeObservedCSV, err := os.ReadFile(p.WriteObservedCSV)
	if err != nil {
		t.Fatalf("reading %s: %v", p.WriteObservedCSV, err)
	}
	observed, err := extable.ParseWriteObservedCSV(writeObservedCSV)
	if err != nil {
		t.Fatalf("ParseWriteObservedCSV(%s): %v", p.WriteObservedCSV, err)
	}

	want, err := extable.RenderWriteGo(rows, observed)
	if err != nil {
		t.Fatalf("RenderWriteGo: %v", err)
	}

	got, err := os.ReadFile(p.OutFile)
	if err != nil {
		t.Fatalf("reading %s: %v", p.OutFile, err)
	}

	if !bytes.Equal(got, want) {
		t.Errorf("%s is stale relative to %s and/or %s; run `go generate ./core/cat` and commit the result (regenerated %d bytes, committed %d bytes)", p.OutFile, p.ManualCSV, p.WriteObservedCSV, len(want), len(got))
	}
}
