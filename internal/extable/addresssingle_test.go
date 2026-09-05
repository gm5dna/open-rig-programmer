// SPDX-License-Identifier: GPL-3.0-or-later

package extable

import (
	"strings"
	"testing"
)

// fixtureSingleRequired is fixtureRequired under the THIRD address form,
// AddressSingle — the Kenwood shape, where the chart's printed menu number is
// the whole address and every row's p2 and p3 columns must be 0. It varies
// exactly ONE axis from fixtureRequired, for the reason fixturePairRequired
// does: a fixture that also moved its labels or its text rows would leave
// which policy refused a row an open question.
//
// Fixture-only, never registered, so no staleness consumer goes looking for a
// generated file that does not exist. The three Kenwood registrations land
// later, each with its own manual CSV.
var fixtureSingleRequired = func() Profile {
	p := fixtureRequired
	p.Addresses = AddressSingle
	return p
}()

// singleRow is a valid one-row CSV body under fixtureSingleRequired: the menu
// number in p1, p2 and p3 zero, both label columns filled (the fixture is
// LabelsRequired), digits 4 inside the fixture's 2..6 bounds.
const singleRow = "08,00,00,RADIO SETTING,MODE SSB,AGC FAST DELAY,20 - 4000,4,false,646\n"

// singleObservedRow is the matching observation CSV row. Its p2 and p3
// columns are "00" — the one value AddressSingle permits — and neither may
// appear in the lookup key.
const singleObservedRow = "08,00,00,3,numeric\n"

// TestProfileValidate_AddressSingle covers the second of the form's five
// sites: Profile.Validate must ADMIT the new member and must still refuse an
// unset one on a profile of this shape, for the reason every other policy on
// this type refuses its zero value — an omitted semantic refuses, never
// defaults.
func TestProfileValidate_AddressSingle(t *testing.T) {
	if err := fixtureSingleRequired.Validate(); err != nil {
		t.Errorf("Validate() on the single-form fixture: %v", err)
	}
	p := fixtureSingleRequired
	p.Addresses = 0
	err := p.Validate()
	if err == nil {
		t.Fatal("Validate() accepted a single-form-shaped profile with a zero AddressForm; want a refusal")
	}
	if !strings.Contains(err.Error(), "AddressForm") {
		t.Errorf("Validate() = %v, want it to name AddressForm", err)
	}
}

// TestParseCSV_AddressSingleRefusesNonZeroP2AndP3 is the first site.
//
// Both extra components are REFUSED rather than dropped — the AddressPair
// discipline one component further down. A value silently discarded here
// would reach the generated inventory as a 0 that nothing recorded having
// changed, and on this radio family the discarded component would be a menu
// number's own digits.
//
// The last case is the discriminating one: AddressPair's wire field really
// does carry P2, so the new rule must not have leaked one component up into
// the form above it.
func TestParseCSV_AddressSingleRefusesNonZeroP2AndP3(t *testing.T) {
	for _, tc := range []struct {
		name    string
		profile Profile
		csv     string
		wantErr string // "" means the row MUST parse
	}{
		{"single baseline parses", fixtureSingleRequired, singleRow, ""},
		{
			"AddressSingle refuses a non-zero p2",
			fixtureSingleRequired,
			"08,01,00,RADIO SETTING,MODE SSB,AGC FAST DELAY,20 - 4000,4,false,646\n",
			"p2",
		},
		{
			"AddressSingle refuses a non-zero p3",
			fixtureSingleRequired,
			"08,00,07,RADIO SETTING,MODE SSB,AGC FAST DELAY,20 - 4000,4,false,646\n",
			"p3",
		},
		{"AddressPair still accepts a non-zero p2", pairProfile(), pairRow, ""},
	} {
		rows, err := ParseCSV(withRows(tc.profile, 1), []byte(tc.csv))
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

// TestParseObservedCSV_AddressSingleKeysOnP1Alone is the fourth site. The
// join token is P1 alone — two digits — because that is what RenderGo's own
// lookup renders under this form; the two sides of the join must agree on its
// shape or a complete observation CSV can never be found, however
// exhaustively it was captured (the MEDIUM-2 lesson, one form further down).
func TestParseObservedCSV_AddressSingleKeysOnP1Alone(t *testing.T) {
	got, err := ParseObservedCSV(fixtureSingleRequired, []byte(singleObservedRow))
	if err != nil {
		t.Fatalf("ParseObservedCSV: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ParseObservedCSV returned %d observations, want 1", len(got))
	}
	obs, ok := got["08"]
	if !ok {
		t.Fatalf("ParseObservedCSV keyed the row as %v, want the two-digit key \"08\" under AddressSingle", got)
	}
	if obs.ReadWidth != 3 || obs.ReadShape != "numeric" {
		t.Errorf("observation = %+v, want {3 numeric}", obs)
	}
}

// TestParseObservedCSV_AddressSingleRefusesNonZeroP2AndP3 is the refusal half
// of the same site: neither component is on the wire under this form
// (parseRecord enforces the identical rule for the inventory CSV), so a
// non-zero one names something the radio can never have answered with.
func TestParseObservedCSV_AddressSingleRefusesNonZeroP2AndP3(t *testing.T) {
	for _, tc := range []struct{ name, csv, want string }{
		{"non-zero p2", "08,01,00,3,numeric\n", "p2 must be 0"},
		{"non-zero p3", "08,00,01,3,numeric\n", "p3 must be 0"},
	} {
		_, err := ParseObservedCSV(fixtureSingleRequired, []byte(tc.csv))
		if err == nil {
			t.Errorf("%s: ParseObservedCSV accepted the row under AddressSingle; want a refusal", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: ParseObservedCSV refused with %q, which does not say %q", tc.name, err, tc.want)
		}
	}
}

// TestRenderGo_SingleProfileKeysObservationsByTwoDigitForm is the fifth site,
// and it goes THROUGH ParseObservedCSV on a genuine single-form CSV row for
// the reason the AddressPair version of this test does: a version that
// hand-built the map would pass even if the two sides of the join disagreed.
//
// Under the AddressPair arm this row would key "0800" and the lookup would
// refuse every observation the CSV carries.
func TestRenderGo_SingleProfileKeysObservationsByTwoDigitForm(t *testing.T) {
	profile := withRows(fixtureSingleRequired, 1)
	rows, err := ParseCSV(profile, []byte(singleRow))
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	observed, err := ParseObservedCSV(profile, []byte(singleObservedRow))
	if err != nil {
		t.Fatalf("ParseObservedCSV: %v", err)
	}
	if _, ok := observed["08"]; !ok {
		t.Fatalf("ParseObservedCSV keyed the row as %v, want the two-digit key \"08\"", observed)
	}
	out, err := RenderGo(profile, rows, observed)
	if err != nil {
		t.Fatalf("RenderGo: %v — a real single-form observation CSV, parsed by ParseObservedCSV and keyed under this profile's own AddressSingle form, must be found by RenderGo's matching lookup", err)
	}
	if !strings.Contains(string(out), "ObservedReadWidth: 3") {
		t.Errorf("generated output does not carry the two-digit-keyed observation:\n%s", out)
	}
	// The address itself still renders all three components — the generated
	// EXAddress type carries P1, P2 and P3 whatever the form — and under this
	// form the last two are the zeroes ParseCSV has already required.
	if !strings.Contains(string(out), "EXAddress{P1: 8, P2: 0, P3: 0}") {
		t.Errorf("generated output does not carry the single-form address:\n%s", out)
	}
}

// TestParseCSV_AddressSingleP1DomainIs0To99 pins the component cap
// parseRecord already enforces for every form (extable.go:119-123) under
// THIS one explicitly: a Kenwood chart prints a three-digit menu number, but
// it is numerically 0..99 (SG's own maximum is 099), so P1 fits the existing
// two-digit cap with room. Pinning it here means the T9 transcriber meets a
// stated bound rather than discovering the general cap through a failing
// parse of some future radio's row 100.
func TestParseCSV_AddressSingleP1DomainIs0To99(t *testing.T) {
	for _, tc := range []struct {
		name    string
		p1      string
		wantErr string // "" means the row MUST parse
	}{
		{"P1 = 99 is the top of the domain", "99", ""},
		{"P1 = 100 is refused", "100", "0..99"},
	} {
		csv := tc.p1 + ",00,00,RADIO SETTING,MODE SSB,AGC FAST DELAY,20 - 4000,4,false,646\n"
		rows, err := ParseCSV(withRows(fixtureSingleRequired, 1), []byte(csv))
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
