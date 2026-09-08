// SPDX-License-Identifier: GPL-3.0-or-later

package ma_test

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/kw/ma"
	"github.com/gm5dna/open-rig-programmer/internal/extable"
)

// ts890sProfileName is this radio's own registry lookup name.
//
// SELECTION IS BY THE EXACT NAME, AND NEVER BY Package. This package holds
// TWO generated inventories from two different books, so a package-only
// filter selects both registrations and returns whichever the registry
// happened to sort first — which would check this radio's generated file
// against the other book's transcription half the time, the cross-model
// borrowing the package's file split exists to prevent.
// TestTS890SProfile_SelectedByNameNotByPackage below pins the equivalence of
// the two admissible selectors so the choice cannot drift.
const ts890sProfileName = "ts890s"

// profile890S returns the registered TS-890S profile, failing the test if the
// registry does not hold it. Every test in this file goes through here, so
// none of them re-states a profile fact from a literal of its own: a bound
// consulted from one place with its datum taken from another is the defect
// shape internal/extable.Profile's own doc comment says the type exists to
// prevent.
func profile890S(t *testing.T) extable.Profile {
	t.Helper()
	p, ok := extable.Lookup(ts890sProfileName)
	if !ok {
		var names []string
		for _, np := range extable.RegisteredProfiles() {
			names = append(names, np.Name)
		}
		t.Fatalf("extable.Lookup(%q) found no profile; registered: %v", ts890sProfileName, names)
	}
	return p
}

// TestTS890SProfile_SelectedByNameNotByPackage pins the two admissible
// selectors — Lookup with the exact registry name, or RegisteredProfiles
// filtered on the pair Package + VarName — as returning the same
// registration.
//
// The pair is validateRegistry's own uniqueness key, so exactly one match is
// the invariant; Package alone is not a key in this package at all, and the
// count check is what says so out loud.
func TestTS890SProfile_SelectedByNameNotByPackage(t *testing.T) {
	p := profile890S(t)
	var byPair []extable.NamedProfile
	for _, np := range extable.RegisteredProfiles() {
		if np.Profile.Package == p.Package && np.Profile.VarName == p.VarName {
			byPair = append(byPair, np)
		}
	}
	if len(byPair) != 1 {
		t.Fatalf("RegisteredProfiles() filtered on Package %q + VarName %q matched %d entries, want exactly 1", p.Package, p.VarName, len(byPair))
	}
	if byPair[0].Name != ts890sProfileName {
		t.Errorf("the Package+VarName filter selected %q, want %q", byPair[0].Name, ts890sProfileName)
	}
	if byPair[0].Profile.Model != p.Model {
		t.Errorf("the two selectors disagree: %q vs %q", byPair[0].Profile.Model, p.Model)
	}
}

// TestEXInventory890SGenerated_NotStale re-derives this radio's generated
// inventory from its ONE source — menu890s.csv, the transcription of this
// book's "EX Command Parameter Lists" — and byte-compares the result with the
// committed exinventory890s_gen.go. It is the CI guard for the generator: CI
// runs plain `go test ./...` and never `go generate`, so without this test an
// edit to menu890s.csv that was not regenerated, or a hand-edit of the
// generated file, would ship silently. On failure, run
// `go generate ./core/kw/ma` and commit the result.
func TestEXInventory890SGenerated_NotStale(t *testing.T) {
	p := profile890S(t)

	// Named "manual" and not "csv": encoding/csv is imported by this file.
	manual, err := os.ReadFile(p.ManualCSV)
	if err != nil {
		t.Fatalf("reading %s: %v", p.ManualCSV, err)
	}
	rows, err := extable.ParseCSV(p, manual)
	if err != nil {
		t.Fatalf("ParseCSV(%s): %v", p.ManualCSV, err)
	}
	// ObservationsAbsent: no TS-890S has ever been asked anything, so
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

// TestEXItems890S_HasExpectedRowsLessTheExclusions is what makes the BOOTSTRAP
// placeholder impossible to ship. That file declared exItems890S as an empty
// slice so the package compiled before this transcription existed; an
// ungenerated or half-generated tree would otherwise publish an EMPTY menu
// table as a valid one, and this package's own accessor test reads only the
// signatures, deliberately.
//
// The count comes from the PROFILE, never from a literal here, and it is
// ExpectedRows LESS the excluded addresses rather than ExpectedRows itself:
// the chart's four "Does not correspond to a command" rows are transcribed
// and counted, and omitted from the inventory, so the published length is the
// difference. RenderGo asserts the same arithmetic from the other side.
func TestEXItems890S_HasExpectedRowsLessTheExclusions(t *testing.T) {
	p := profile890S(t)
	want := p.ExpectedRows - len(p.ParameterlessAddresses)
	if got := len(ma.EXItems890S()); got != want {
		t.Errorf("EXItems890S() published %d items, want ExpectedRows (%d) less the %d excluded address(es) = %d — an empty or truncated inventory means generation has not run", got, p.ExpectedRows, len(p.ParameterlessAddresses), want)
	}
}

// TestEXItems890S_ExcludesTheDeclaredAddressesAndNothingElse is the by-address
// half of the same gate on the SHIPPED inventory: the count above is satisfied
// by any four omissions, so it is this test that says WHICH four are gone.
func TestEXItems890S_ExcludesTheDeclaredAddressesAndNothingElse(t *testing.T) {
	p := profile890S(t)
	present := map[[3]int]bool{}
	for _, it := range ma.EXItems890S() {
		present[[3]int{int(it.Addr.P1), int(it.Addr.P2), int(it.Addr.P3)}] = true
	}
	for _, a := range p.ParameterlessAddresses {
		if present[a] {
			t.Errorf("address %d/%d/%d is in the inventory, but the profile names it parameterless — that row's chart line names no field an EX frame could read or write", a[0], a[1], a[2])
		}
	}
}

// csvBody890S returns menu890s.csv's data rows with its provenance comments
// and blank lines removed, so a test can perturb ONE row without reproducing
// the transcription's facts in this file.
func csvBody890S(t *testing.T) []string {
	t.Helper()
	p := profile890S(t)
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

// TestRedProof890S_DeletingARowIsRefused is the completeness gate fired.
// RenderGo compares the supplied sets against each other only, so no regime
// can see a jointly truncated source; ExpectedRows is the check that catches
// it, and a transcription that silently lost a row would otherwise render
// happily into a smaller, plausible menu table.
func TestRedProof890S_DeletingARowIsRefused(t *testing.T) {
	p := profile890S(t)
	body := csvBody890S(t)
	if len(body) != p.ExpectedRows {
		t.Fatalf("menu890s.csv holds %d data rows, want %d", len(body), p.ExpectedRows)
	}
	// Drop the LAST row, which is one of the four excluded ones, so the
	// falsification also proves the gate does not depend on the missing row
	// being one the inventory would have carried.
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

// TestRedProof890S_P1OutsideTheMenuTypeEnumerationIsRefused fires the
// AddressGrouped P1 rule in the direction that matters: a menu type this book
// does not print is REFUSED, not dropped or clamped. The one-digit field
// could carry 0..9; the book enumerates two values, so 2 names a group no
// chart prints and a row carrying it must not reach an inventory.
//
// The fixture is this chart's own first row with one column edited, so the
// test cannot pass by disagreeing with the transcription about anything else.
func TestRedProof890S_P1OutsideTheMenuTypeEnumerationIsRefused(t *testing.T) {
	p := profile890S(t)
	first := strings.Split(csvBody890S(t)[0], ",")
	if len(first) < 3 {
		t.Fatalf("the first CSV row has %d columns, want at least 3", len(first))
	}
	for _, p1 := range []string{"2", "9"} {
		rec := append([]string(nil), first...)
		rec[0] = p1
		rows, err := extable.ParseCSV(p, []byte(strings.Join(rec, ",")+"\n"))
		if err == nil {
			t.Errorf("P1 %s: ParseCSV accepted the row and returned %d rows, want a refusal", p1, len(rows))
			continue
		}
		if len(rows) != 0 {
			t.Errorf("P1 %s: ParseCSV refused but returned %d rows; a refused row must not reach a caller", p1, len(rows))
		}
		if !strings.Contains(err.Error(), "0..1") {
			t.Errorf("P1 %s: ParseCSV refused with %v, want the message to name the 0..1 domain", p1, err)
		}
		if !strings.Contains(err.Error(), p.Addresses.String()) {
			t.Errorf("P1 %s: ParseCSV refused with %v, want the message to name %v", p1, err, p.Addresses)
		}
	}
}

// TestRedProof890S_ATextRowOfAnUndeclaredWidthIsRefused is the width half of
// the text policy. This chart prints exactly two free-text fields and they are
// NOT the same width — a screen-saver message of up to 10 characters and a
// power-on message of up to 15 — which is why TextWidths is a set of two and
// not a single number. A transcriber who also flagged some other row as text,
// at a width neither entry names, would be stating a third thing about one
// chart, and this is the refusal that says so rather than letting it through
// as one of the two.
func TestRedProof890S_ATextRowOfAnUndeclaredWidthIsRefused(t *testing.T) {
	p := profile890S(t)
	if p.TextRowPolicy != extable.TextRowsAllowed {
		t.Fatalf("TextRowPolicy = %v, want TextRowsAllowed", p.TextRowPolicy)
	}
	// A row of this chart's own shape at 0/00/00, flagged text at a width of
	// 8 — a plausible number, and one this book never prints.
	const row = "0,00,00,,,Color Display Pattern,Up to 8 alphanumeric characters,8,true,1939\n"
	_, err := extable.ParseCSV(p, []byte(row))
	if err == nil {
		t.Fatalf("ParseCSV accepted a text row of width 8, want a refusal naming TextWidths %v", p.TextWidths)
	}
	if !strings.Contains(err.Error(), fmt.Sprint(p.TextWidths)) {
		t.Errorf("ParseCSV refused with %v, want the message to name the profile's TextWidths %v", err, p.TextWidths)
	}
	// And BOTH widths the chart actually prints still parse, so the refusal
	// above is about the width and not about text rows as such.
	for _, ok := range []string{
		"0,00,05,,,Screen Saver Message,Up to 10 alphanumeric characters,10,true,1946\n",
		"0,00,06,,,Power-on Message,Up to 15 alphanumeric characters,15,true,1947\n",
	} {
		if _, err := extable.ParseCSV(p, []byte(ok)); err != nil {
			t.Errorf("ParseCSV refused one of this chart's own text rows at a declared width %v: %v", p.TextWidths, err)
		}
	}
}

// TestRedProof890S_ExcludingTheWrongAddressesIsRefused is the falsification
// the exclusion set exists for, and the one a COUNT could never make.
//
// The perturbed CSV below still holds exactly ExpectedRows rows and still
// holds exactly four hyphen digits cells: every arithmetic gate in the chain
// is satisfied by it, and the inventory it would produce would be the right
// LENGTH with the wrong four rows missing. The licence to print a hyphen is
// per address, so ParseCSV refuses it — naming the address, in both
// directions: a hyphen on a row the profile does not name, and a width on a
// row it does.
func TestRedProof890S_ExcludingTheWrongAddressesIsRefused(t *testing.T) {
	p := profile890S(t)
	body := csvBody890S(t)

	// The declared exclusion this test moves, and an ordinary row to move it
	// on to. Both are read out of the transcription rather than restated: the
	// declared one comes from the profile, and the ordinary one is simply the
	// row before it in the CSV.
	declared := p.ParameterlessAddresses[0]
	declaredPrefix := fmt.Sprintf("%d,%02d,%02d,", declared[0], declared[1], declared[2])
	di := -1
	for i, l := range body {
		if strings.HasPrefix(l, declaredPrefix) {
			di = i
			break
		}
	}
	if di <= 0 {
		t.Fatalf("no CSV row starts %q (found at index %d) — the profile's first ParameterlessAddresses entry is not in the transcription", declaredPrefix, di)
	}
	victim := di - 1

	perturbed := append([]string(nil), body...)
	// Give the declared row a width, and give its neighbour the hyphen.
	perturbed[di] = swapDigitsCell(t, perturbed[di], "3")
	perturbed[victim] = swapDigitsCell(t, perturbed[victim], "-")

	// The arithmetic a count-only gate would check is UNCHANGED, which is the
	// whole point of this fixture.
	if len(perturbed) != p.ExpectedRows {
		t.Fatalf("the perturbed CSV holds %d rows, want %d — the fixture must not change the count", len(perturbed), p.ExpectedRows)
	}
	if got := countHyphenDigits(t, perturbed); got != len(p.ParameterlessAddresses) {
		t.Fatalf("the perturbed CSV holds %d hyphen digits cells, want %d — the fixture must not change how many rows claim to be parameterless", got, len(p.ParameterlessAddresses))
	}

	_, err := extable.ParseCSV(p, []byte(strings.Join(perturbed, "\n")+"\n"))
	if err == nil {
		t.Fatalf("ParseCSV accepted a CSV that excludes the wrong rows; the count alone cannot see it, so the by-address rule must")
	}
	if !strings.Contains(err.Error(), "ParameterlessAddresses") {
		t.Errorf("ParseCSV refused with %v, want the message to name ParameterlessAddresses", err)
	}

	// The other direction, on its own: a declared address carrying a width.
	// The hyphen count drops to three here, which is exactly why this half
	// needs its own fixture rather than riding on the one above.
	onlyDeclared := append([]string(nil), body...)
	onlyDeclared[di] = swapDigitsCell(t, onlyDeclared[di], "3")
	_, err = extable.ParseCSV(p, []byte(strings.Join(onlyDeclared, "\n")+"\n"))
	if err == nil {
		t.Fatalf("ParseCSV accepted a width on address %d/%d/%d, which the profile declares parameterless", declared[0], declared[1], declared[2])
	}
	if !strings.Contains(err.Error(), "may not carry a width") {
		t.Errorf("ParseCSV refused with %v, want the message to say a declared exclusion may not carry a width", err)
	}
}

// swapDigitsCell returns record with its digits column (index 7 of the ten
// this schema fixes) replaced by want.
func swapDigitsCell(t *testing.T, record, want string) string {
	t.Helper()
	f := strings.Split(record, ",")
	if len(f) != 10 {
		t.Fatalf("CSV record %q split into %d fields, want the schema's 10", record, len(f))
	}
	f[7] = want
	return strings.Join(f, ",")
}

// countHyphenDigits reports how many of records carry a hyphen digits cell.
// It goes through encoding/csv rather than strings.Split because one row of
// this transcription quotes a p4 cell containing a comma, and a naive split
// would miscount the columns of exactly that row.
func countHyphenDigits(t *testing.T, records []string) int {
	t.Helper()
	r := csv.NewReader(strings.NewReader(strings.Join(records, "\n") + "\n"))
	recs, err := r.ReadAll()
	if err != nil {
		t.Fatalf("re-reading the perturbed CSV: %v", err)
	}
	n := 0
	for _, f := range recs {
		if len(f) == 10 && f[7] == "-" {
			n++
		}
	}
	return n
}
