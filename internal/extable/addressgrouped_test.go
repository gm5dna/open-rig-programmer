// SPDX-License-Identifier: GPL-3.0-or-later

package extable

import (
	"strings"
	"testing"
)

// fixtureGroupedRequired is fixtureRequired under the FOURTH address form,
// AddressGrouped — the shape whose address is a (P1,P2,P3) triple of one, two
// and two digits. It varies exactly ONE axis from fixtureRequired, for the
// reason fixturePairRequired and fixtureSingleRequired do: a fixture that also
// moved its labels or its text rows would leave which policy refused a row an
// open question.
//
// Fixture-only, never registered, so no staleness consumer goes looking for a
// generated file that does not exist.
var fixtureGroupedRequired = func() Profile {
	p := fixtureRequired
	p.Addresses = AddressGrouped
	return p
}()

// groupedRow is a valid one-row CSV body under fixtureGroupedRequired: the
// menu-type digit in p1, a two-digit category in p2 and a two-digit item in
// p3, both label columns filled (the fixture is LabelsRequired), digits 4
// inside the fixture's 2..6 bounds.
const groupedRow = "1,00,05,RADIO SETTING,MODE SSB,AGC FAST DELAY,20 - 4000,4,false,646\n"

// groupedObservedRow is the matching observation CSV row. Its p1 column is ONE
// digit and its p2 and p3 columns two, which is what AddressGrouped requires
// of the three components its wire field carries; ALL THREE are part of the
// lookup key, which is what makes this form differ from every other.
const groupedObservedRow = "1,00,05,3,numeric\n"

// TestProfileValidate_AddressGrouped is the form's first site: Validate must
// ADMIT the new member and must still refuse an unset one on a profile of this
// shape, for the reason every other policy on this type refuses its zero value
// — an omitted semantic refuses, never defaults.
func TestProfileValidate_AddressGrouped(t *testing.T) {
	if err := fixtureGroupedRequired.Validate(); err != nil {
		t.Errorf("Validate() on the grouped-form fixture: %v", err)
	}
	p := fixtureGroupedRequired
	p.Addresses = 0
	err := p.Validate()
	if err == nil {
		t.Fatal("Validate() accepted a grouped-form-shaped profile with a zero AddressForm; want a refusal")
	}
	if !strings.Contains(err.Error(), "AddressForm") {
		t.Errorf("Validate() = %v, want it to name AddressForm", err)
	}
}

// TestParseCSV_AddressGroupedP1DomainIs0To1 is the second site, and it is the
// one place this form's bound is NOT the capacity of the field it renders
// into.
//
// P1 is a one-digit field, so a capacity argument would give it 0..9. The
// manuals enumerate the values instead — "P1 (Menu type number) 0: Menu 1:
// Advanced Menu" (ts890s_pc_rev1_layout.txt:1898-1900,
// ts990s_pc_rev2_layout.txt:1721-1723) — so the domain is the ENUMERATION and
// the refusal names the form, as AddressSingle's does, because a sentence
// quoting a number without saying which form is in force leaves the reader to
// guess which of two rules they broke.
func TestParseCSV_AddressGroupedP1DomainIs0To1(t *testing.T) {
	for _, tc := range []struct {
		name    string
		p1      string
		wantErr string // "" means the row MUST parse
	}{
		{"P1 = 0, the Menu group", "0", ""},
		{"P1 = 1, the Advanced Menu group", "1", ""},
		{"P1 = 2 is refused, though the field could carry it", "2", "must be 0..1 under AddressGrouped"},
		{"P1 = 9, the top of the one-digit field, is refused", "9", "must be 0..1 under AddressGrouped"},
		{"P1 = 10 is refused", "10", "must be 0..1 under AddressGrouped"},
		{"a negative P1 is refused", "-1", "must be 0..1 under AddressGrouped"},
	} {
		csv := tc.p1 + ",00,05,RADIO SETTING,MODE SSB,AGC FAST DELAY,20 - 4000,4,false,646\n"
		rows, err := ParseCSV(withRows(fixtureGroupedRequired, 1), []byte(csv))
		switch {
		case tc.wantErr == "" && err != nil:
			t.Errorf("%s: ParseCSV() = %v, want the row to parse", tc.name, err)
		case tc.wantErr == "" && len(rows) != 1:
			t.Errorf("%s: ParseCSV() returned %d rows, want 1", tc.name, len(rows))
		case tc.wantErr != "" && err == nil:
			t.Errorf("%s: ParseCSV() accepted the row, want a refusal naming %q", tc.name, tc.wantErr)
		case tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr):
			t.Errorf("%s: ParseCSV() = %v, want it to name %q", tc.name, err, tc.wantErr)
		}
	}
}

// TestParseCSV_AddressGroupedCarriesAllThreeComponents is the discriminating
// half. Every form added since AddressTriple has REFUSED a component its wire
// field cannot express — P3 under Pair, P2 and P3 under Single — and this one
// refuses neither: all three are on the wire. A grouped arm that copied its
// neighbours' zero rules would refuse every row of a real chart.
//
// P2 and P3 keep the SHIPPED two-digit sentence, because their fields really
// are two digits wide; only P1's bound is the form's own.
func TestParseCSV_AddressGroupedCarriesAllThreeComponents(t *testing.T) {
	rows, err := ParseCSV(withRows(fixtureGroupedRequired, 1), []byte(groupedRow))
	if err != nil {
		t.Fatalf("ParseCSV refused a grouped row with a non-zero p3: %v", err)
	}
	if got := rows[0]; got.P1 != 1 || got.P2 != 0 || got.P3 != 5 {
		t.Errorf("Row = %+v, want P1 1 / P2 0 / P3 5 — all three components transcribed", got)
	}
	for _, tc := range []struct{ name, csv string }{
		{"P2 above the two-digit domain", "1,100,05,RADIO SETTING,MODE SSB,AGC FAST DELAY,20 - 4000,4,false,646\n"},
		{"P3 above the two-digit domain", "1,00,100,RADIO SETTING,MODE SSB,AGC FAST DELAY,20 - 4000,4,false,646\n"},
	} {
		_, err := ParseCSV(withRows(fixtureGroupedRequired, 1), []byte(tc.csv))
		if err == nil {
			t.Errorf("%s: ParseCSV accepted the row under AddressGrouped", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), "must be 0..99") {
			t.Errorf("%s: ParseCSV refused with %q, want the shipped two-digit sentence", tc.name, err)
		}
	}
}

// TestParseObservedCSV_AddressGroupedKeysOnAllThreeColumns is the third site.
// The join token is five characters — one digit, then two, then two — because
// that is what RenderGo's own lookup renders under this form; the two sides of
// the join must agree on its shape or a complete observation CSV can never be
// found, however exhaustively it was captured.
//
// The p1 column's SHAPE is exact in both directions: two digits is the other
// forms' spelling and would key this row six characters wide.
func TestParseObservedCSV_AddressGroupedKeysOnAllThreeColumns(t *testing.T) {
	got, err := ParseObservedCSV(fixtureGroupedRequired, []byte(groupedObservedRow))
	if err != nil {
		t.Fatalf("ParseObservedCSV: %v", err)
	}
	obs, ok := got["10005"]
	if !ok {
		t.Fatalf("ParseObservedCSV keyed the row as %v, want the five-character key \"10005\" under AddressGrouped", got)
	}
	if obs.ReadWidth != 3 || obs.ReadShape != "numeric" {
		t.Errorf("observation = %+v, want {3 numeric}", obs)
	}
	for _, tc := range []struct{ name, csv, want string }{
		{"a two-digit p1", "01,00,05,3,numeric\n", "address component P1 must be exactly one digit"},
		{"a three-digit p1", "001,00,05,3,numeric\n", "address component P1 must be exactly one digit"},
		{"a one-digit p2", "1,0,05,3,numeric\n", "address component 1 must be exactly two digits"},
		{"a one-digit p3", "1,00,5,3,numeric\n", "address component 2 must be exactly two digits"},
	} {
		_, err := ParseObservedCSV(fixtureGroupedRequired, []byte(tc.csv))
		if err == nil {
			t.Errorf("%s: ParseObservedCSV accepted the row under AddressGrouped", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: ParseObservedCSV refused with %q, want it to contain %q", tc.name, err, tc.want)
		}
	}
	// The one-digit sentence is this form's alone: the other three still
	// refuse a one-digit p1 with their own text, so the new arm has not moved
	// a shipped refusal.
	for _, p := range []Profile{fixtureRequired, pairProfile(), fixtureSingleRequired} {
		_, err := ParseObservedCSV(p, []byte("1,00,05,3,numeric\n"))
		if err == nil {
			t.Errorf("%v: ParseObservedCSV accepted a one-digit p1", p.Addresses)
			continue
		}
		if strings.Contains(err.Error(), "exactly one digit") {
			t.Errorf("%v: ParseObservedCSV refused with the AddressGrouped sentence: %v", p.Addresses, err)
		}
	}
}

// TestRenderGo_GroupedProfileKeysObservationsByFiveDigitForm is the fourth
// site, and it goes THROUGH ParseObservedCSV on a genuine grouped-form CSV row
// for the reason the Pair and Single versions of this test do: a version that
// hand-built the map would pass even if the two sides of the join disagreed.
//
// Under the AddressTriple arm this row would key "010005" and the lookup would
// refuse every observation the CSV carries.
//
// The emitted literal is the one thing this form does NOT change: a grouped
// address uses all three components, so kw.EXAddress{P1: n, P2: n, P3: n} is
// rendered exactly as AddressTriple renders it — the first Kenwood form for
// which that is true.
func TestRenderGo_GroupedProfileKeysObservationsByFiveDigitForm(t *testing.T) {
	profile := withRows(fixtureGroupedRequired, 1)
	rows, err := ParseCSV(profile, []byte(groupedRow))
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	observed, err := ParseObservedCSV(profile, []byte(groupedObservedRow))
	if err != nil {
		t.Fatalf("ParseObservedCSV: %v", err)
	}
	out, err := RenderGo(profile, rows, observed)
	if err != nil {
		t.Fatalf("RenderGo: %v — a real grouped-form observation CSV, parsed by ParseObservedCSV and keyed under this profile's own AddressGrouped form, must be found by RenderGo's matching lookup", err)
	}
	if !strings.Contains(string(out), "ObservedReadWidth: 3") {
		t.Errorf("generated output does not carry the five-character-keyed observation:\n%s", out)
	}
	if want := "EXAddress{P1: 1, P2: 0, P3: 5}"; !strings.Contains(string(out), want) {
		t.Errorf("generated output does not carry %s:\n%s", want, out)
	}
	// The zero-group row keys "00005", NOT "0005": the p1 column is a
	// fixed-width token on both sides, so the group digit is always present.
	zeroGroup := withRows(fixtureGroupedRequired, 1)
	rows, err = ParseCSV(zeroGroup, []byte("0,00,05,RADIO SETTING,MODE SSB,AGC FAST DELAY,20 - 4000,4,false,646\n"))
	if err != nil {
		t.Fatalf("ParseCSV on the zero group: %v", err)
	}
	observed, err = ParseObservedCSV(zeroGroup, []byte("0,00,05,3,numeric\n"))
	if err != nil {
		t.Fatalf("ParseObservedCSV on the zero group: %v", err)
	}
	if _, ok := observed["00005"]; !ok {
		t.Fatalf("ParseObservedCSV keyed the zero-group row as %v, want \"00005\"", observed)
	}
	if _, err := RenderGo(zeroGroup, rows, observed); err != nil {
		t.Errorf("RenderGo on the zero group: %v", err)
	}
}
