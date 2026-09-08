// SPDX-License-Identifier: GPL-3.0-or-later

package ma_test

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/kw/ma"
	"github.com/gm5dna/open-rig-programmer/internal/extable"
)

// ts990sProfileName is this radio's own registry lookup name.
//
// SELECTION IS BY THE EXACT NAME, AND NEVER BY Package. More than one
// profile emits into package ma, so a package-only filter selects more than
// one registration and returns whichever the registry happened to sort
// first — which would check this radio's generated file against another
// chart's transcription, silently, some of the time.
// TestTS990SProfile_SelectedByNameNotByPackage below pins the equivalence of
// the two admissible selectors so the choice cannot drift.
const ts990sProfileName = "ts990s"

// profile990S returns the registered TS-990S profile, failing the test if the
// registry does not hold it. Every test in this file goes through here, so
// none of them re-states a profile fact from a literal of its own: a bound
// consulted from one place with its datum taken from another is the defect
// shape internal/extable.Profile's own doc comment says the type exists to
// prevent.
func profile990S(t *testing.T) extable.Profile {
	t.Helper()
	p, ok := extable.Lookup(ts990sProfileName)
	if !ok {
		var names []string
		for _, np := range extable.RegisteredProfiles() {
			names = append(names, np.Name)
		}
		t.Fatalf("extable.Lookup(%q) found no profile; registered: %v", ts990sProfileName, names)
	}
	return p
}

// TestTS990SProfile_SelectedByNameNotByPackage pins the two admissible
// selectors — Lookup with the exact registry name, or RegisteredProfiles
// filtered on the pair Package + VarName — as returning the same
// registration. The pair is validateRegistry's own uniqueness key, so
// exactly one match is the invariant; Package alone is not a key here at
// all, and the count check is what says so out loud.
func TestTS990SProfile_SelectedByNameNotByPackage(t *testing.T) {
	p := profile990S(t)
	var byPair []extable.NamedProfile
	for _, np := range extable.RegisteredProfiles() {
		if np.Profile.Package == p.Package && np.Profile.VarName == p.VarName {
			byPair = append(byPair, np)
		}
	}
	if len(byPair) != 1 {
		t.Fatalf("RegisteredProfiles() filtered on Package %q + VarName %q matched %d entries, want exactly 1", p.Package, p.VarName, len(byPair))
	}
	if byPair[0].Name != ts990sProfileName {
		t.Errorf("the Package+VarName filter selected %q, want %q", byPair[0].Name, ts990sProfileName)
	}
	if byPair[0].Profile.Model != p.Model {
		t.Errorf("the two selectors disagree: %q vs %q", byPair[0].Profile.Model, p.Model)
	}
}

// TestEXInventory990SGenerated_NotStale re-derives this radio's generated
// inventory from its ONE source — menu990s.csv, the manual transcription of
// this book's "EX Command Parameter Lists" — and byte-compares the result
// with the committed exinventory990s_gen.go. It is the CI guard for the
// generator: CI runs plain `go test ./...` and never `go generate`, so
// without this test an edit to menu990s.csv that was not regenerated, or a
// hand-edit of the generated file, would ship silently. On failure, run
// `go generate ./core/kw/ma` and commit the result.
func TestEXInventory990SGenerated_NotStale(t *testing.T) {
	p := profile990S(t)

	csv, err := os.ReadFile(p.ManualCSV)
	if err != nil {
		t.Fatalf("reading %s: %v", p.ManualCSV, err)
	}
	rows, err := extable.ParseCSV(p, csv)
	if err != nil {
		t.Fatalf("ParseCSV(%s): %v", p.ManualCSV, err)
	}
	// ObservationsAbsent: no TS-990S has ever been asked anything, so
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
		t.Errorf("%s is stale relative to %s; run `go generate ./core/kw/ma` and commit the result (regenerated %d bytes, committed %d bytes)", p.OutFile, p.ManualCSV, len(want), len(got))
	}
}

// TestEXItems990S_HasExpectedRows is what makes the bootstrap placeholder
// impossible to ship. That file declared exItems990S as an empty slice so
// the package compiled before this transcription existed; an ungenerated or
// half-generated tree would otherwise publish an EMPTY menu table as a valid
// one, and this package's own accessor test pins the SIGNATURES only,
// deliberately, because the values are this lane's.
//
// The count comes from the PROFILE, never from a literal here, and it is
// ExpectedRows LESS the excluded addresses: ExpectedRows counts every row
// the chart prints, and RenderGo omits the ones the profile names as
// carrying no parameter. This chart names none, so the subtraction is a
// no-op today — written out because the arithmetic, not the coincidence, is
// what this asserts.
func TestEXItems990S_HasExpectedRows(t *testing.T) {
	p := profile990S(t)
	want := p.ExpectedRows - len(p.ParameterlessAddresses)
	if got := len(ma.EXItems990S()); got != want {
		t.Errorf("EXItems990S() published %d items, want ExpectedRows (%d) less the %d excluded address(es) = %d — an empty or truncated inventory means generation has not run", got, p.ExpectedRows, len(p.ParameterlessAddresses), want)
	}
}

// csvBody990S returns menu990s.csv's data rows with its provenance comments
// and blank lines removed, so a test can perturb ONE row without reproducing
// the transcription's facts in this file.
func csvBody990S(t *testing.T) []string {
	t.Helper()
	p := profile990S(t)
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

// TestRedProof990S_DeletingARowIsRefused is the completeness gate fired.
// RenderGo compares the manual and observation sets against each other only,
// so neither regime can see a jointly truncated pair of sources;
// ExpectedRows is the check that catches it, and a transcription that
// silently lost a row would otherwise render happily into a smaller,
// plausible menu table.
func TestRedProof990S_DeletingARowIsRefused(t *testing.T) {
	p := profile990S(t)
	body := csvBody990S(t)
	if len(body) != p.ExpectedRows {
		t.Fatalf("menu990s.csv holds %d data rows, want %d", len(body), p.ExpectedRows)
	}
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

// TestRedProof990S_P1OutsideTheMenuTypeEnumerationIsRefused fires the
// AddressGrouped P1 rule in the direction that matters: a menu type no chart
// prints is REFUSED, not dropped or clamped. The bound is the book's
// enumeration — this chart's two tables and nothing else — and not the
// one-digit field's capacity, so 2 must fail even though the field could
// carry it.
//
// The fixture is this chart's own first row with one column edited, so the
// test cannot pass by disagreeing with the transcription about anything
// else.
func TestRedProof990S_P1OutsideTheMenuTypeEnumerationIsRefused(t *testing.T) {
	p := profile990S(t)
	first := strings.Split(csvBody990S(t)[0], ",")
	if len(first) < 1 {
		t.Fatalf("the first CSV row is empty")
	}
	for _, p1 := range []string{"2", "9"} {
		rec := append([]string(nil), first...)
		rec[0] = p1
		_, err := extable.ParseCSV(p, []byte(strings.Join(rec, ",")+"\n"))
		if err == nil {
			t.Errorf("P1 %s: ParseCSV accepted the row, want a refusal", p1)
			continue
		}
		if !strings.Contains(err.Error(), "P1") {
			t.Errorf("P1 %s: ParseCSV refused with %v, want the message to name P1", p1, err)
		}
		if !strings.Contains(err.Error(), p.Addresses.String()) {
			t.Errorf("P1 %s: ParseCSV refused with %v, want the message to name %v", p1, err, p.Addresses)
		}
	}
	// And the chart's own first row still parses, so the refusals above are
	// about the VALUE and not about the row.
	if _, err := extable.ParseCSV(p, []byte(strings.Join(first, ",")+"\n")); err != nil {
		t.Errorf("ParseCSV refused this chart's own first row: %v", err)
	}
}

// TestRedProof990S_ATextRowOfAnUndeclaredWidthIsRefused is the width half of
// the text policy. Profile carries a SET of text widths — this chart prints
// two free-text fields of different lengths — and ParseCSV refuses any text
// row whose Digits the set does not name. A transcriber who flagged a third
// field as text at some other width would be stating something about this
// chart that it does not print, and this is the refusal that says so rather
// than letting the width through as one of the two.
func TestRedProof990S_ATextRowOfAnUndeclaredWidthIsRefused(t *testing.T) {
	p := profile990S(t)
	if p.TextRowPolicy != extable.TextRowsAllowed {
		t.Fatalf("TextRowPolicy = %v, want TextRowsAllowed", p.TextRowPolicy)
	}
	// A row of this chart's own shape, flagged text at a width the profile
	// does not declare.
	const row = "0,0,0,,,Screen Saver Message,Up to 12 alphanumeric characters,12,true,1778\n"
	_, err := extable.ParseCSV(p, []byte(row))
	if err == nil {
		t.Fatalf("ParseCSV accepted a text row of width 12, want a refusal naming TextWidths %v", p.TextWidths)
	}
	if !strings.Contains(err.Error(), fmt.Sprint(p.TextWidths)) {
		t.Errorf("ParseCSV refused with %v, want the message to name the profile's TextWidths %v", err, p.TextWidths)
	}
	// And the chart's OWN text rows — both of them, at both declared widths
	// — still parse, so the refusal above is about the width and not about
	// text rows as such. They are taken from the transcription rather than
	// restated here.
	var text []string
	for _, l := range csvBody990S(t) {
		if f := strings.Split(l, ","); f[len(f)-2] == "true" {
			text = append(text, l)
		}
	}
	if len(text) != len(p.TextWidths) {
		t.Fatalf("menu990s.csv flags %d text rows, want one per declared width %v", len(text), p.TextWidths)
	}
	for _, l := range text {
		if _, err := extable.ParseCSV(p, []byte(l+"\n")); err != nil {
			t.Errorf("ParseCSV refused this chart's own text row %q at one of its declared widths %v: %v", l, p.TextWidths, err)
		}
	}
}
