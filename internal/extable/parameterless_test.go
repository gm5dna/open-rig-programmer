// SPDX-License-Identifier: GPL-3.0-or-later

package extable

import (
	"strings"
	"testing"
)

// The FT-991A's chart prints ONE row with no parameter at all: menu number
// 087 RADIO ID, whose parameter column is ten hyphens and whose Digits cell
// is a single hyphen (docs/fixtures-private/manuals/ft991a_layout.txt:623,
// re-derived). The row is printed, so it is transcribed and counted; it
// names no field, so it is not an EX item. These fixtures and tests pin both
// halves.
//
// They are built on fixtureAbsent, an AddressTriple profile, DELIBERATELY:
// ParameterlessRows and AddressForm are independent policies (see the type's
// own doc comment), and a fixture that varied both at once would leave which
// of the two admitted the row an open question.

// parameterlessP4 is row 087's parameter column verbatim — the ten hyphens
// the chart draws where every other row prints a range. It is asserted
// character-for-character rather than by length, because "the manual's cells
// survive transcription verbatim" is the property, not "something hyphen-like
// arrived".
const parameterlessP4 = "----------"

// parameterlessRow is row 087 as the transcription writes it under a
// LabelsRequired, AddressTriple fixture: the menu number in p1, the Digits
// cell a single hyphen, the parameter column parameterlessP4.
const parameterlessRow = "87,00,00,RADIO SETTING,MODE SSB,RADIO ID," + parameterlessP4 + ",-,false,623\n"

// ordinaryRow is a second, ordinary row under the same fixture, so the
// exclusion tests can show that exactly ONE row leaves the inventory.
const ordinaryRow = "88,00,00,RADIO SETTING,MODE SSB,AGC FAST DELAY,20 - 4000,4,false,624\n"

// fixtureParameterless is fixtureAbsent under ParameterlessExcluded, with
// row 087's address declared. ExpectedRows is 2: the chart's printed row
// count INCLUDES the excluded row, which is the whole point of counting the
// address set separately from it.
var fixtureParameterless = func() Profile {
	p := fixtureAbsent
	p.ParameterlessPolicy = ParameterlessExcluded
	p.ParameterlessAddresses = [][3]int{{87, 0, 0}}
	p.ExpectedRows = 2
	return p
}()

// TestProfileValidate_Parameterless pins the fourth chart-shape policy's
// zero value and the cross-field rules it carries. Zero refuses for the
// reason the other three do: an omitted semantic must refuse, never default
// — a profile that forgot to say whether its chart prints a parameterless
// row would otherwise inherit one regime or the other silently.
func TestProfileValidate_Parameterless(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mut     func(*Profile)
		wantErr string // "" means the profile MUST validate
	}{
		{"the excluded fixture is valid", func(*Profile) {}, ""},
		{"zero ParameterlessRows", func(p *Profile) { p.ParameterlessPolicy = 0 }, "ParameterlessRows"},
		{"unknown ParameterlessRows", func(p *Profile) { p.ParameterlessPolicy = ParameterlessRows(9) }, "ParameterlessRows"},
		{
			"ParameterlessExcluded with no declared address",
			func(p *Profile) { p.ParameterlessAddresses = nil },
			"ParameterlessAddresses",
		},
		{
			"ParameterlessRefused with a declared address",
			func(p *Profile) { p.ParameterlessPolicy = ParameterlessRefused },
			"ParameterlessAddresses",
		},
		{
			"a duplicated address would corrupt the arithmetic",
			func(p *Profile) { p.ParameterlessAddresses = [][3]int{{87, 0, 0}, {87, 0, 0}} },
			"duplicate",
		},
		{
			"ExpectedRows equal to the excluded count leaves no inventory",
			func(p *Profile) { p.ExpectedRows = 1 },
			"ExpectedRows",
		},
	} {
		p := fixtureParameterless
		p.ParameterlessAddresses = append([][3]int(nil), p.ParameterlessAddresses...)
		tc.mut(&p)
		err := p.Validate()
		switch {
		case tc.wantErr == "" && err != nil:
			t.Errorf("%s: Validate() = %v, want accepted", tc.name, err)
		case tc.wantErr != "" && err == nil:
			t.Errorf("%s: Validate() accepted the profile, want a refusal naming %q", tc.name, tc.wantErr)
		case tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr):
			t.Errorf("%s: Validate() = %v, want it to name %q", tc.name, err, tc.wantErr)
		}
	}
}

// TestProfileValidate_ParameterlessAddressesAreCopied pins that the registry
// hands out a profile whose declared exclusion set cannot be mutated back
// into it, as DocLines already is. A caller holding the registry's own slice
// could otherwise silently change which address a later generation excludes.
func TestProfileValidate_ParameterlessAddressesAreCopied(t *testing.T) {
	p := fixtureParameterless.clone()
	if len(p.ParameterlessAddresses) != 1 {
		t.Fatalf("clone() carried %d addresses, want 1", len(p.ParameterlessAddresses))
	}
	p.ParameterlessAddresses[0] = [3]int{99, 0, 0}
	if fixtureParameterless.ParameterlessAddresses[0] != [3]int{87, 0, 0} {
		t.Errorf("mutating a clone's ParameterlessAddresses changed the original to %v", fixtureParameterless.ParameterlessAddresses[0])
	}
}

// TestParseCSV_ParameterlessRow is the parse half, and it is FOUR red proofs
// and one positive one:
//
//   - a hyphen Digits cell on a row NOT in the declared set is refused — the
//     policy licenses ONE address, not any hyphen anywhere;
//   - a NUMERIC Digits cell on a row that IS in the set is refused — no
//     width may be smuggled onto an address declared to have none;
//   - a hyphen Digits cell under ParameterlessRefused is refused NAMING THE
//     ROW, which is the contract every refusal in this package keeps;
//   - the declared row parses with Parameterless true, Digits at its zero
//     value and its printed cells verbatim.
//
// Digits: 0 is deliberately NOT the representation of "no parameter" — a
// separate bool is — because a zero that means two different things is
// exactly the omitted-semantic-defaulted hazard M9c-1 exists to refuse.
// The bool is what this test reads; the zero Digits is asserted only as the
// value the field is LEFT at.
func TestParseCSV_ParameterlessRow(t *testing.T) {
	refusing := func() Profile {
		p := fixtureAbsent
		p.ParameterlessPolicy = ParameterlessRefused
		return p
	}()
	// ParseCSV does not gate on ExpectedRows — RenderGo does — so these
	// fixtures keep the count the chart would print (2) and feed it one row
	// at a time. A withRows(…, 1) here would be refused by Validate, which
	// requires ExpectedRows to EXCEED the excluded count.
	for _, tc := range []struct {
		name    string
		profile Profile
		csv     string
		wantErr string // "" means the row MUST parse
	}{
		{"the declared address parses", fixtureParameterless, parameterlessRow, ""},
		{
			"a hyphen on an undeclared address is refused",
			fixtureParameterless,
			"88,00,00,RADIO SETTING,MODE SSB,AGC FAST DELAY," + parameterlessP4 + ",-,false,624\n",
			"AGC FAST DELAY",
		},
		{
			"a numeric width on a declared address is refused",
			fixtureParameterless,
			"87,00,00,RADIO SETTING,MODE SSB,RADIO ID,20 - 4000,4,false,623\n",
			"RADIO ID",
		},
		{
			"a hyphen under ParameterlessRefused is refused, naming the row",
			refusing,
			parameterlessRow,
			"RADIO ID",
		},
	} {
		rows, err := ParseCSV(tc.profile, []byte(tc.csv))
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

	rows, err := ParseCSV(fixtureParameterless, []byte(parameterlessRow))
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	got := rows[0]
	if !got.Parameterless {
		t.Errorf("Row.Parameterless = false on the declared address, want true")
	}
	if got.Digits != 0 {
		t.Errorf("Row.Digits = %d on a parameterless row, want it left at 0", got.Digits)
	}
	if got.P4 != parameterlessP4 {
		t.Errorf("Row.P4 = %q, want the chart's ten hyphens %q verbatim", got.P4, parameterlessP4)
	}
	if got.Name != "RADIO ID" || got.ManualLine != 623 {
		t.Errorf("Row = %+v, want the printed cells transcribed unchanged", got)
	}
}

// TestParseCSV_ParameterlessIsIndependentOfAddressForm pins the sentence the
// policy's doc comment makes: a chart whose address is a PAIR may print a
// parameterless row too. The two policies are separate axes, and neither
// admits the other's row.
func TestParseCSV_ParameterlessIsIndependentOfAddressForm(t *testing.T) {
	p := fixtureParameterless
	p.Addresses = AddressPair
	p.LabelPolicy = LabelsAbsent
	p.TextRowPolicy = TextRowsAbsent
	p.TextWidth = 0
	p.ParameterlessAddresses = [][3]int{{87, 1, 0}}

	rows, err := ParseCSV(p, []byte("87,01,00,,,RADIO ID,"+parameterlessP4+",-,false,623\n"))
	if err != nil {
		t.Fatalf("ParseCSV under AddressPair: %v — the parameterless policy must not be tied to one address form", err)
	}
	if !rows[0].Parameterless {
		t.Errorf("Row.Parameterless = false under AddressPair, want true")
	}
}

// TestRenderGo_ParameterlessAddressIsAbsentByAddress is the fifth red proof.
// The generated inventory omits EXACTLY the declared address and keeps every
// other row, and the omission is keyed on the ADDRESS — the profile's own
// datum — not on a count, because a count-only gate can omit the WRONG row
// and still satisfy the arithmetic.
func TestRenderGo_ParameterlessAddressIsAbsentByAddress(t *testing.T) {
	rows, err := ParseCSV(fixtureParameterless, []byte(parameterlessRow+ordinaryRow))
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	out, err := RenderGo(fixtureParameterless, rows, nil)
	if err != nil {
		t.Fatalf("RenderGo: %v", err)
	}
	got := string(out)
	if strings.Contains(got, "RADIO ID") {
		t.Errorf("the parameterless row reached the generated inventory:\n%s", got)
	}
	if strings.Contains(got, "P1: 87") {
		t.Errorf("the excluded address reached the generated inventory:\n%s", got)
	}
	if !strings.Contains(got, "P1: 88") || !strings.Contains(got, "AGC FAST DELAY") {
		t.Errorf("the ordinary row did not survive:\n%s", got)
	}
	// The generated header records the exclusion BY ADDRESS, so a reader of
	// the artefact alone can see which row is missing and why.
	if !strings.Contains(got, "87/0/0") {
		t.Errorf("the generated header does not record the excluded address:\n%s", got)
	}
}

// TestRenderGo_ParameterlessArithmetic pins the generator's own accounting:
// ExpectedRows minus the DECLARED ADDRESS COUNT is exactly how many items are
// emitted. The discriminating case is the second: a profile declaring an
// address its CSV never carries emits one item too many, and a gate that
// compared only counts would accept it while the wrong row had been dropped.
func TestRenderGo_ParameterlessArithmetic(t *testing.T) {
	rows, err := ParseCSV(fixtureParameterless, []byte(parameterlessRow+ordinaryRow))
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	out, err := RenderGo(fixtureParameterless, rows, nil)
	if err != nil {
		t.Fatalf("RenderGo: %v", err)
	}
	if n := strings.Count(string(out), "{Addr:"); n != fixtureParameterless.ExpectedRows-len(fixtureParameterless.ParameterlessAddresses) {
		t.Errorf("generated %d items, want ExpectedRows(%d) - excluded(%d)", n, fixtureParameterless.ExpectedRows, len(fixtureParameterless.ParameterlessAddresses))
	}

	p := fixtureParameterless
	p.ParameterlessAddresses = [][3]int{{99, 0, 0}}
	absent, err := ParseCSV(p, []byte("87,00,00,RADIO SETTING,MODE SSB,RADIO ID,20 - 4000,4,false,623\n"+ordinaryRow))
	if err != nil {
		t.Fatalf("ParseCSV with an unused exclusion: %v", err)
	}
	if _, err := RenderGo(p, absent, nil); err == nil {
		t.Error("RenderGo accepted a profile whose declared excluded address is not in the inventory; want a refusal")
	}
}
