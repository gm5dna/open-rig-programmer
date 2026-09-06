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

// singleObservedRow is the matching observation CSV row. Its p1 column is
// THREE digits, which is what AddressSingle requires of the component its
// wire field carries; its p2 and p3 columns are "00" — the one value the
// form permits — and neither may appear in the lookup key.
const singleObservedRow = "008,00,00,3,numeric\n"

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
// join token is P1 alone — THREE digits — because that is what RenderGo's own
// lookup renders under this form; the two sides of the join must agree on its
// shape or a complete observation CSV can never be found, however
// exhaustively it was captured (the MEDIUM-2 lesson, one form further down).
//
// REWRITTEN by the FT-991A milestone (plan §OQ3, plan review M8). The Kenwood
// lane wrote this test with the key "08", which was right while P1's domain
// was 0..99: %02d of 8 is "08". Under the three-digit domain %02d is a
// MINIMUM width — it renders 153 as "153" but 8 as "08" — so the two sides of
// the join would have agreed only by accident, and a complete capture of a
// 153-row chart would have been refused row by row on the day hardware
// arrived. The key is "008" on both sides now. The Kenwood pins are
// SUPERSEDED rather than broken: every Kenwood row still keys correctly,
// three digits wide.
func TestParseObservedCSV_AddressSingleKeysOnP1Alone(t *testing.T) {
	got, err := ParseObservedCSV(fixtureSingleRequired, []byte(singleObservedRow))
	if err != nil {
		t.Fatalf("ParseObservedCSV: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ParseObservedCSV returned %d observations, want 1", len(got))
	}
	obs, ok := got["008"]
	if !ok {
		t.Fatalf("ParseObservedCSV keyed the row as %v, want the three-digit key \"008\" under AddressSingle", got)
	}
	if obs.ReadWidth != 3 || obs.ReadShape != "numeric" {
		t.Errorf("observation = %+v, want {3 numeric}", obs)
	}
	// The whole domain, keyed: a fixed-width token means 001 and 153 are the
	// same shape, which is the property a %02d key does not have.
	for _, tc := range []struct{ p1, key string }{
		{"001", "001"},
		{"099", "099"},
		{"100", "100"},
		{"153", "153"},
	} {
		got, err := ParseObservedCSV(fixtureSingleRequired, []byte(tc.p1+",00,00,3,numeric\n"))
		if err != nil {
			t.Errorf("ParseObservedCSV(p1=%s): %v", tc.p1, err)
			continue
		}
		if _, ok := got[tc.key]; !ok {
			t.Errorf("ParseObservedCSV(p1=%s) keyed the row as %v, want %q", tc.p1, got, tc.key)
		}
	}
	// And the SHAPE is exact, in both directions: two digits is the old
	// spelling and four is one no three-digit field can carry. The refusal
	// must say "three", because a sentence that says "two" is simply false of
	// P1 under this form (plan review L9).
	for _, tc := range []struct{ name, p1 string }{
		{"the old two-digit spelling", "08"},
		{"four digits", "0153"},
		{"one digit", "8"},
	} {
		_, err := ParseObservedCSV(fixtureSingleRequired, []byte(tc.p1+",00,00,3,numeric\n"))
		if err == nil {
			t.Errorf("%s: ParseObservedCSV accepted p1 %q under AddressSingle", tc.name, tc.p1)
			continue
		}
		// It names the component P1, as parseRecord's own domain refusal
		// does ("address component P1 must be 0..999"), rather than the
		// 0-based loop index its two-digit sibling carries: a user reading
		// "component 0" beside a chart whose columns are P1/P2/P3 has to
		// guess (Stage 0 close review, seat 1 LOW-2). The two-digit
		// sentence is shipped text and keeps its index.
		if !strings.Contains(err.Error(), "address component P1 must be exactly three digits") {
			t.Errorf("%s: ParseObservedCSV refused with %q, which does not say the P1 column is three digits", tc.name, err)
		}
	}
}

// TestParseObservedCSV_TwoDigitRefusalSurvivesForTheOtherForms is the other
// side of that branch. The sentence "address component %d must be exactly two
// digits" is TRUE under AddressTriple and AddressPair and must not have moved
// when the Single arm gained its own; a single sentence composed from a
// number would have moved all three.
func TestParseObservedCSV_TwoDigitRefusalSurvivesForTheOtherForms(t *testing.T) {
	for _, tc := range []struct {
		name    string
		profile Profile
		csv     string
	}{
		{"AddressTriple", fixtureRequired, "008,00,00,3,numeric\n"},
		{"AddressPair", pairProfile(), "008,00,00,3,numeric\n"},
	} {
		_, err := ParseObservedCSV(tc.profile, []byte(tc.csv))
		if err == nil {
			t.Errorf("%s: ParseObservedCSV accepted a three-digit p1", tc.name)
			continue
		}
		if want := "address component 0 must be exactly two digits"; !strings.Contains(err.Error(), want) {
			t.Errorf("%s: ParseObservedCSV refused with %q, want it to contain %q", tc.name, err, want)
		}
	}
}

// TestParseObservedCSV_AddressSingleRefusesNonZeroP2AndP3 is the refusal half
// of the same site: neither component is on the wire under this form
// (parseRecord enforces the identical rule for the inventory CSV), so a
// non-zero one names something the radio can never have answered with.
func TestParseObservedCSV_AddressSingleRefusesNonZeroP2AndP3(t *testing.T) {
	// The p1 columns are THREE digits, which is a fixture correction the
	// FT-991A milestone had to make even though this pin's rule and name are
	// untouched: the shape check now runs before the p2/p3 rule, so a
	// two-digit p1 would be refused for its width and this test would pass
	// while never reaching the refusal it exists to pin.
	for _, tc := range []struct{ name, csv, want string }{
		{"non-zero p2", "008,01,00,3,numeric\n", "p2 must be 0"},
		{"non-zero p3", "008,00,01,3,numeric\n", "p3 must be 0"},
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

// TestRenderGo_SingleProfileKeysObservationsByThreeDigitForm is the fifth
// site, and it goes THROUGH ParseObservedCSV on a genuine single-form CSV row
// for the reason the AddressPair version of this test does: a version that
// hand-built the map would pass even if the two sides of the join disagreed.
//
// Under the AddressPair arm this row would key "00800" and the lookup would
// refuse every observation the CSV carries.
//
// RENAMED by the FT-991A milestone (plan §OQ3): the Kenwood lane named it
// ...ByTwoDigitForm, and its subject is now a three-digit token. The second
// case is the one that could not exist before the widening — 153, the
// FT-991A chart's last row, whose %02d rendering happened to be right and
// whose 008 sibling's was not, which is exactly how a MINIMUM-width join key
// hides.
func TestRenderGo_SingleProfileKeysObservationsByThreeDigitForm(t *testing.T) {
	for _, tc := range []struct{ name, p1, key string }{
		{"a low address, where %02d and %03d disagree", "08", "008"},
		{"the chart's last row, where they agree", "153", "153"},
	} {
		profile := withRows(fixtureSingleRequired, 1)
		row := tc.p1 + ",00,00,RADIO SETTING,MODE SSB,AGC FAST DELAY,20 - 4000,4,false,646\n"
		rows, err := ParseCSV(profile, []byte(row))
		if err != nil {
			t.Fatalf("%s: ParseCSV: %v", tc.name, err)
		}
		observed, err := ParseObservedCSV(profile, []byte(tc.key+",00,00,3,numeric\n"))
		if err != nil {
			t.Fatalf("%s: ParseObservedCSV: %v", tc.name, err)
		}
		if _, ok := observed[tc.key]; !ok {
			t.Fatalf("%s: ParseObservedCSV keyed the row as %v, want the three-digit key %q", tc.name, observed, tc.key)
		}
		out, err := RenderGo(profile, rows, observed)
		if err != nil {
			t.Fatalf("%s: RenderGo: %v — a real single-form observation CSV, parsed by ParseObservedCSV and keyed under this profile's own AddressSingle form, must be found by RenderGo's matching lookup", tc.name, err)
		}
		if !strings.Contains(string(out), "ObservedReadWidth: 3") {
			t.Errorf("%s: generated output does not carry the three-digit-keyed observation:\n%s", tc.name, out)
		}
		// The address itself still renders all three components — the
		// generated EXAddress type carries P1, P2 and P3 whatever the form —
		// and under this form the last two are the zeroes ParseCSV has
		// already required.
		want := "EXAddress{P1: " + strings.TrimLeft(tc.p1, "0") + ", P2: 0, P3: 0}"
		if !strings.Contains(string(out), want) {
			t.Errorf("%s: generated output does not carry %s:\n%s", tc.name, want, out)
		}
	}
}

// TestParseCSV_AddressSingleP1DomainIs0To999 pins the component bound, which
// is now the FORM's rather than the package's.
//
// REWRITTEN by the FT-991A milestone (plan §OQ3). The Kenwood lane wrote this
// as ...DomainIs0To99 and pinned the narrow bound deliberately: its charts
// stop at 099, so P1 fitted the package-wide two-digit cap with room. That
// pin is SUPERSEDED, not broken — 0..99 is a subset of 0..999 and every
// Kenwood row still parses — and what replaces it is the rule that owns the
// number: the FT-991A's chart is "P1 : 001 - 153" over 153 contiguous rows,
// so 54 of them were untranscribable under the old cap and the generator
// would have failed on row 100 after the whole of Stage 0 had landed.
//
// The bound lives with the FORM because that is where the datum is: the
// three-digit wire field carries 0..999, and the four- and six-digit forms'
// two-digit components carry 0..99. The refusal names the form for the same
// reason — a sentence quoting a number without saying which form is in force
// leaves the reader to guess which of two rules they broke.
func TestParseCSV_AddressSingleP1DomainIs0To999(t *testing.T) {
	for _, tc := range []struct {
		name    string
		p1      string
		wantErr string // "" means the row MUST parse
	}{
		{"P1 = 001, the chart's first row", "001", ""},
		{"P1 = 099, the top of the OLD domain", "099", ""},
		{"P1 = 100, the first row the old cap refused", "100", ""},
		{"P1 = 153, the chart's last row", "153", ""},
		{"P1 = 999, the top of the three-digit field", "999", ""},
		{"P1 = 1000 is refused", "1000", "must be 0..999 under AddressSingle"},
		{"a negative P1 is refused", "-1", "must be 0..999 under AddressSingle"},
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

// TestParseCSV_TheOtherFormsKeepTheTwoDigitDomain is the disagreeing half,
// and it is what makes the bound FORM-dependent rather than merely wider. The
// sentence Triple and Pair refuse with is the SHIPPED one and must not have
// moved: a bound stated once and composed from whichever number is in force
// would have moved it for every model in the repository.
func TestParseCSV_TheOtherFormsKeepTheTwoDigitDomain(t *testing.T) {
	tripleRow := "100,01,01,RADIO SETTING,MODE SSB,AGC FAST DELAY,20 - 4000,4,false,646\n"
	_, err := ParseCSV(withRows(fixtureRequired, 1), []byte(tripleRow))
	if err == nil {
		t.Fatal("ParseCSV accepted a P1 of 100 under AddressTriple")
	}
	if want := "address component P1 must be 0..99, got 100"; !strings.Contains(err.Error(), want) {
		t.Errorf("the shipped Triple component refusal moved\n    want: %s\n    got:  %v", want, err)
	}
	pairOver := "100,01,00,,,AGC FAST DELAY,20 - 4000,4,false,646\n"
	_, err = ParseCSV(withRows(pairProfile(), 1), []byte(pairOver))
	if err == nil {
		t.Fatal("ParseCSV accepted a P1 of 100 under AddressPair")
	}
	if want := "address component P1 must be 0..99, got 100"; !strings.Contains(err.Error(), want) {
		t.Errorf("the Pair component refusal is not the shipped sentence\n    want: %s\n    got:  %v", want, err)
	}
	// P2 and P3 keep the narrow bound under Single too — they are not on the
	// wire at all, and the p2/p3-must-be-zero rule refuses them first, which
	// is the pin TestParseCSV_AddressSingleRefusesNonZeroP2AndP3 holds.
	_, err = ParseCSV(withRows(fixtureSingleRequired, 1), []byte("008,100,00,RADIO SETTING,MODE SSB,AGC FAST DELAY,20 - 4000,4,false,646\n"))
	if err == nil {
		t.Error("ParseCSV accepted a P2 of 100 under AddressSingle")
	}
}

// TestProfileValidate_SingleProfileMustDeclareItsDigitsCeiling pins the
// Kenwood lane's per-family width datum (Profile.DigitsCeiling) under THIS
// form specifically, because the FT-991A's own profile — Stage 1's work — is
// the first Single-form stanza this repository will carry, and a required
// field is only required if something refuses its absence on the shape that
// will omit it. It is a REQUIRED field, not one defaulting to core/cat's
// MaxDigitsCeiling: an omitted semantic refuses, never defaults.
func TestProfileValidate_SingleProfileMustDeclareItsDigitsCeiling(t *testing.T) {
	p := fixtureSingleRequired
	p.DigitsCeiling = 0
	err := p.Validate()
	if err == nil {
		t.Fatal("Validate() accepted a Single-form profile with no DigitsCeiling; want a refusal")
	}
	if !strings.Contains(err.Error(), "DigitsCeiling") {
		t.Errorf("Validate() = %v, want it to name DigitsCeiling", err)
	}
}
