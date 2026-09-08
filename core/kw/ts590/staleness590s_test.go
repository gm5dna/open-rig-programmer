// SPDX-License-Identifier: GPL-3.0-or-later

package ts590_test

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/kw/ts590"
	"github.com/gm5dna/open-rig-programmer/internal/extable"
)

// ts590sProfileName is this radio's own registry lookup name.
//
// SELECTION IS BY THE EXACT NAME, AND NEVER BY Package. Two profiles emit
// into package ts590 — the TS-590S's and the TS-590SG's — because the book
// prints two separate menu charts over colliding addresses, so a
// package-only filter selects both and returns whichever the registry
// happened to sort first. That would silently check one radio's generated
// file against the other radio's transcription half the time, which is the
// cross-model borrowing this package's file split exists to prevent.
// core/cat/ft891's staleness test may filter by Package because that
// package holds one profile; this one may not, and
// TestTS590SProfile_SelectedByNameNotByPackage below pins the equivalence
// of the two admissible selectors so the choice cannot drift.
const ts590sProfileName = "ts590s"

// profile returns the registered TS-590S profile, failing the test if the
// registry does not hold it. Every test in this file goes through here, so
// none of them re-states a profile fact from a literal of its own: a bound
// consulted from one place with its datum taken from another is the defect
// shape internal/extable.Profile's own doc comment says the type exists to
// prevent.
func profile(t *testing.T) extable.Profile {
	t.Helper()
	p, ok := extable.Lookup(ts590sProfileName)
	if !ok {
		var names []string
		for _, np := range extable.RegisteredProfiles() {
			names = append(names, np.Name)
		}
		t.Fatalf("extable.Lookup(%q) found no profile; registered: %v", ts590sProfileName, names)
	}
	return p
}

// TestTS590SProfile_SelectedByNameNotByPackage pins the two admissible
// selectors named by the design — Lookup with the exact registry name, or
// RegisteredProfiles filtered on the pair Package + VarName — as returning
// the same registration.
//
// The pair is validateRegistry's own uniqueness key, so exactly one match is
// the invariant; Package alone is not a key at all here and the count check
// is what says so out loud. The test is not a tautology today only because
// it would fail the moment a second profile in this package took this
// VarName, or the moment the name and the pair drifted apart.
func TestTS590SProfile_SelectedByNameNotByPackage(t *testing.T) {
	p := profile(t)
	var byPair []extable.NamedProfile
	for _, np := range extable.RegisteredProfiles() {
		if np.Profile.Package == p.Package && np.Profile.VarName == p.VarName {
			byPair = append(byPair, np)
		}
	}
	if len(byPair) != 1 {
		t.Fatalf("RegisteredProfiles() filtered on Package %q + VarName %q matched %d entries, want exactly 1", p.Package, p.VarName, len(byPair))
	}
	if byPair[0].Name != ts590sProfileName {
		t.Errorf("the Package+VarName filter selected %q, want %q", byPair[0].Name, ts590sProfileName)
	}
	if byPair[0].Profile.Model != p.Model {
		t.Errorf("the two selectors disagree: %q vs %q", byPair[0].Profile.Model, p.Model)
	}
}

// TestEXInventory590SGenerated_NotStale re-derives this radio's generated
// inventory from its ONE source — menu590s.csv, the manual transcription of
// "EX Command Parameter List (for TS-590S)" — and byte-compares the result
// with the committed exinventory590s_gen.go. It is the CI guard for the
// generator: CI runs plain `go test ./...` and never `go generate`, so
// without this test an edit to menu590s.csv that was not regenerated, or a
// hand-edit of the generated file, would ship silently. On failure, run
// `go generate ./core/kw/ts590` and commit the result.
func TestEXInventory590SGenerated_NotStale(t *testing.T) {
	p := profile(t)

	csv, err := os.ReadFile(p.ManualCSV)
	if err != nil {
		t.Fatalf("reading %s: %v", p.ManualCSV, err)
	}
	rows, err := extable.ParseCSV(p, csv)
	if err != nil {
		t.Fatalf("ParseCSV(%s): %v", p.ManualCSV, err)
	}
	// ObservationsAbsent: no TS-590S has ever been asked anything, so
	// RenderGo requires the observation map to be EMPTY rather than partial,
	// and nil is the correct argument — not a stand-in for an unread file.
	want, err := extable.RenderGo(p, rows, nil)
	if err != nil {
		t.Fatalf("RenderGo: %v", err)
	}

	got, err := os.ReadFile(p.OutFile)
	if err != nil {
		t.Fatalf("reading %s: %v", p.OutFile, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s is stale relative to %s; run `go generate ./core/kw/ts590` and commit the result (regenerated %d bytes, committed %d bytes)", p.OutFile, p.ManualCSV, len(want), len(got))
	}
}

// TestEXItemsS_HasExpectedRows is what makes task 5's BOOTSTRAP placeholder
// impossible to ship. That file declares exItems590S as an empty slice so
// the package compiles before this transcription exists; an ungenerated or
// half-generated tree would otherwise publish an EMPTY menu table as a valid
// one, and nothing in package ts590's own tests reads the inventory's length
// (they pin the accessor SIGNATURES, deliberately, because the values are
// this lane's).
//
// The count comes from the PROFILE, never from a literal here: ExpectedRows
// is the number the registration carries, RenderGo refuses any other row
// count against it, and this test then requires the accessor to publish
// exactly that many items.
func TestEXItemsS_HasExpectedRows(t *testing.T) {
	p := profile(t)
	if got := len(ts590.EXItemsS()); got != p.ExpectedRows {
		t.Errorf("EXItemsS() published %d items, want the profile's ExpectedRows (%d) — an empty or truncated inventory means generation has not run", got, p.ExpectedRows)
	}
}

// TestEXItemsS_AddressesAreSingleComponent pins the AddressSingle contract on
// the SHIPPED inventory rather than only on the CSV: a Kenwood EX address is
// a three-digit menu number and nothing else, so every item's P2 and P3 must
// be zero. ParseCSV refuses a non-zero column (see the red proof below), but
// this is the assertion over what consumers actually read.
func TestEXItemsS_AddressesAreSingleComponent(t *testing.T) {
	for _, it := range ts590.EXItemsS() {
		if it.Addr.P2 != 0 || it.Addr.P3 != 0 {
			t.Errorf("item %q carries Addr %v, want P2 and P3 zero", it.Name, it.Addr)
		}
	}
}

// csvBody returns menu590s.csv's data rows with its provenance comments and
// blank lines removed, so a test can perturb ONE row without reproducing the
// transcription's facts in this file.
func csvBody(t *testing.T) []string {
	t.Helper()
	p := profile(t)
	data, err := os.ReadFile(p.ManualCSV)
	if err != nil {
		t.Fatalf("reading %s: %v", p.ManualCSV, err)
	}
	var out []string
	for _, l := range strings.Split(string(data), "\n") {
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		out = append(out, l)
	}
	return out
}

// TestRedProof_DeletingARowIsRefused is the completeness gate fired. RenderGo
// compares the manual and observation sets against each other only, so
// neither regime can see a jointly truncated pair of sources; ExpectedRows is
// the check that catches it, and a transcription that silently lost a row
// would otherwise render happily into a smaller, plausible menu table.
func TestRedProof_DeletingARowIsRefused(t *testing.T) {
	p := profile(t)
	body := csvBody(t)
	if len(body) != p.ExpectedRows {
		t.Fatalf("menu590s.csv holds %d data rows, want %d", len(body), p.ExpectedRows)
	}
	// Drop the LAST row: menu 087, the one text row, so the falsification
	// also proves the gate does not depend on which kind of row went missing.
	short := strings.Join(body[:len(body)-1], "\n") + "\n"
	rows, err := extable.ParseCSV(p, []byte(short))
	if err != nil {
		t.Fatalf("ParseCSV on the shortened CSV: %v", err)
	}
	if _, err := extable.RenderGo(p, rows, nil); err == nil {
		t.Fatalf("RenderGo accepted %d rows, want a refusal naming ExpectedRows %d", len(rows), p.ExpectedRows)
	} else if !strings.Contains(err.Error(), fmt.Sprint(p.ExpectedRows)) {
		t.Errorf("RenderGo refused with %v, want the message to name ExpectedRows %d", err, p.ExpectedRows)
	}
}

// TestRedProof_NonZeroP2OrP3IsRefused fires the AddressSingle rule in the
// direction that matters: a component the wire cannot express is REFUSED, not
// dropped. A value silently discarded here would reach the generated
// inventory as a 0 that nothing recorded having changed.
//
// The fixture is this chart's own first row, edited in one column at a time,
// so the test cannot pass by disagreeing with the transcription about
// anything else.
func TestRedProof_NonZeroP2OrP3IsRefused(t *testing.T) {
	p := profile(t)
	first := strings.Split(csvBody(t)[0], ",")
	if len(first) < 3 {
		t.Fatalf("the first CSV row has %d columns, want at least 3", len(first))
	}
	for _, tc := range []struct {
		name   string
		column int
		want   string
	}{
		{"non-zero p2", 1, "p2"},
		{"non-zero p3", 2, "p3"},
	} {
		rec := append([]string(nil), first...)
		rec[tc.column] = "7"
		_, err := extable.ParseCSV(p, []byte(strings.Join(rec, ",")+"\n"))
		if err == nil {
			t.Errorf("%s: ParseCSV accepted the row, want a refusal naming %q", tc.name, tc.want)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: ParseCSV refused with %v, want the message to name %q", tc.name, err, tc.want)
		}
		if !strings.Contains(err.Error(), p.Addresses.String()) {
			t.Errorf("%s: ParseCSV refused with %v, want the message to name %v", tc.name, err, p.Addresses)
		}
	}
}

// TestRedProof_ASecondTextRowOfADifferentWidthIsRefused is the width half of
// the text policy, and it is here because this chart is one row away from
// needing a policy internal/extable does not have.
//
// Profile carries a SET of text widths and ParseCSV refuses any text row
// whose Digits the set does not name. This radio's chart prints exactly one
// free-text field — menu 087, "Power on Message (up to 8 ASCII characters)" —
// so a one-entry TextWidths of {8} is the whole truth about it. A transcriber who also flagged a
// fixed-width character field as text, at a width of 4, would be stating two
// incompatible things about one chart, and this is the refusal that says so
// rather than letting the second width through as the first.
func TestRedProof_ASecondTextRowOfADifferentWidthIsRefused(t *testing.T) {
	p := profile(t)
	if p.TextRowPolicy != extable.TextRowsAllowed {
		t.Fatalf("TextRowPolicy = %v, want TextRowsAllowed", p.TextRowPolicy)
	}
	// A row of this chart's own shape at address 000: single-component
	// address, blank labels, flagged text at a width of 4.
	const row = "0,0,0,,,Version information,4 ASCII characters,4,true,749\n"
	_, err := extable.ParseCSV(p, []byte(row))
	if err == nil {
		t.Fatalf("ParseCSV accepted a text row of width 4, want a refusal naming TextWidths %v", p.TextWidths)
	}
	if !strings.Contains(err.Error(), fmt.Sprint(p.TextWidths)) {
		t.Errorf("ParseCSV refused with %v, want the message to name the profile's TextWidths %v", err, p.TextWidths)
	}
	// And the width the chart actually prints still parses, so the refusal
	// above is about the WIDTH and not about text rows as such.
	if _, err := extable.ParseCSV(p, []byte("87,0,0,,,Power on message,Power on Message (up to 8 ASCII characters),8,true,741\n")); err != nil {
		t.Errorf("ParseCSV refused this chart's own text row at one of its declared widths %v: %v", p.TextWidths, err)
	}
}
