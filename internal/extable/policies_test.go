// SPDX-License-Identifier: GPL-3.0-or-later

package extable

import (
	"strings"
	"testing"
)

// pairProfile is fixtureAbsent narrowed to the FT-891's shape: a four-digit
// address, no group labels, no text rows. It is the fixture the three new
// policies exist for, and it disagrees with every registered profile at all
// three of them at once.
func pairProfile() Profile {
	p := fixtureAbsent
	p.Model = "PAIR FIXTURE"
	p.Addresses = AddressPair
	p.LabelPolicy = LabelsAbsent
	p.TextRowPolicy = TextRowsAbsent
	p.TextWidth = 0
	return p
}

// pairRow is a valid one-row CSV body under pairProfile: P3 zero, both
// label columns blank, text false.
const pairRow = "08,01,00,,,AGC FAST DELAY,20 - 4000,4,false,646\n"

func TestPolicyStrings(t *testing.T) {
	for _, tc := range []struct {
		got, want string
	}{
		{AddressTriple.String(), "AddressTriple"},
		{AddressPair.String(), "AddressPair"},
		{AddressForm(0).String(), "AddressForm(0)"},
		{LabelsRequired.String(), "LabelsRequired"},
		{LabelsAbsent.String(), "LabelsAbsent"},
		{Labels(0).String(), "Labels(0)"},
		{TextRowsAllowed.String(), "TextRowsAllowed"},
		{TextRowsAbsent.String(), "TextRowsAbsent"},
		{TextRows(0).String(), "TextRows(0)"},
	} {
		if tc.got != tc.want {
			t.Errorf("String() = %q, want %q", tc.got, tc.want)
		}
	}
}

// TestProfileValidate_ThreeNewPoliciesAreExplicit covers each policy's zero
// value and the one cross-field rule they carry.
//
// Zero is refused for the reason the two policies already here are: an
// omitted semantic must refuse, never default. A profile that forgot to say
// whether its chart has text rows would otherwise inherit "allowed" and
// silently transcode a text row for a radio whose chart prints none.
func TestProfileValidate_ThreeNewPoliciesAreExplicit(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mut     func(*Profile)
		wantErr string // "" means the profile MUST validate
	}{
		{"pair fixture is valid", func(*Profile) {}, ""},
		{"zero AddressForm", func(p *Profile) { p.Addresses = 0 }, "AddressForm"},
		{"unknown AddressForm", func(p *Profile) { p.Addresses = AddressForm(9) }, "AddressForm"},
		{"zero Labels", func(p *Profile) { p.LabelPolicy = 0 }, "Labels"},
		{"unknown Labels", func(p *Profile) { p.LabelPolicy = Labels(9) }, "Labels"},
		{"zero TextRows", func(p *Profile) { p.TextRowPolicy = 0 }, "TextRows"},
		{"unknown TextRows", func(p *Profile) { p.TextRowPolicy = TextRows(9) }, "TextRows"},
		{"TextRowsAbsent with a non-zero TextWidth", func(p *Profile) { p.TextWidth = 12 }, "TextWidth"},
		{"TextRowsAllowed with a zero TextWidth", func(p *Profile) {
			p.TextRowPolicy, p.TextWidth = TextRowsAllowed, 0
		}, "TextWidth"},
		{"TextRowsAllowed with a positive TextWidth", func(p *Profile) {
			p.TextRowPolicy, p.TextWidth = TextRowsAllowed, 8
		}, ""},
	} {
		p := pairProfile()
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

// TestParseCSV_PairAndAbsencePolicies is the parse half. Each refusal must
// name the FIELD and the VALUE, which is the contract every other refusal in
// this package keeps: an error that says only "bad row" leaves a
// three-hundred-row chart to be searched by hand.
func TestParseCSV_PairAndAbsencePolicies(t *testing.T) {
	for _, tc := range []struct {
		name    string
		profile Profile
		csv     string
		wantErr string // "" means the row MUST parse
	}{
		{"pair baseline parses", pairProfile(), pairRow, ""},
		{
			"AddressPair refuses a non-zero P3",
			pairProfile(),
			"08,01,07,,,AGC FAST DELAY,20 - 4000,4,false,646\n",
			"p3",
		},
		{
			"LabelsAbsent refuses a non-blank p1_label",
			pairProfile(),
			"08,01,00,RADIO SETTING,,AGC FAST DELAY,20 - 4000,4,false,646\n",
			"p1_label",
		},
		{
			"LabelsAbsent refuses a non-blank p2_label",
			pairProfile(),
			"08,01,00,,MODE SSB,AGC FAST DELAY,20 - 4000,4,false,646\n",
			"p2_label",
		},
		{
			"TextRowsAbsent refuses a text row",
			pairProfile(),
			"08,01,00,,,MY CALL,Up to 12 characters,4,true,646\n",
			"text",
		},
		{
			"LabelsRequired still refuses a blank p1_label",
			fixtureAbsent,
			"01,01,01,,MODE SSB,AF TREBLE GAIN,x,3,false,646\n",
			"p1_label",
		},
		{
			"AddressTriple still accepts a non-zero P3",
			fixtureAbsent,
			"01,01,07,RADIO SETTING,MODE SSB,AF TREBLE GAIN,x,3,false,646\n",
			"",
		},
		{
			"TextRowsAllowed still accepts a text row at TextWidth",
			fixtureAbsent,
			"01,01,01,RADIO SETTING,MODE SSB,MY CALL,Up to 8,8,true,646\n",
			"",
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
}

// TestRenderGo_LabelsAbsentEmitsEmptyLabels pins the render half of
// LabelsAbsent. ParseCSV requires the columns to be BLANK, which admits a
// whitespace-only cell; the generated file must carry "" rather than that
// whitespace, so a downstream consumer sees an absence and not a space.
func TestRenderGo_LabelsAbsentEmitsEmptyLabels(t *testing.T) {
	p := pairProfile()
	rows, err := ParseCSV(p, []byte("08,01,00,  ,\t,AGC FAST DELAY,20 - 4000,4,false,646\n"))
	if err != nil {
		t.Fatalf("ParseCSV() on whitespace-only label columns: %v", err)
	}
	out, err := RenderGo(p, rows, nil)
	if err != nil {
		t.Fatalf("RenderGo(): %v", err)
	}
	got := string(out)
	if !strings.Contains(got, `P1Label: "", P2Label: ""`) {
		t.Errorf("RenderGo() under LabelsAbsent did not emit empty labels:\n%s", got)
	}
	// And the labelled regime is untouched.
	q := fixtureAbsent
	rows, err = ParseCSV(q, []byte("01,01,01,RADIO SETTING,MODE SSB,AF TREBLE GAIN,x,3,false,646\n"))
	if err != nil {
		t.Fatalf("ParseCSV() on the labelled fixture: %v", err)
	}
	out, err = RenderGo(q, rows, nil)
	if err != nil {
		t.Fatalf("RenderGo() on the labelled fixture: %v", err)
	}
	if !strings.Contains(string(out), `P1Label: "RADIO SETTING", P2Label: "MODE SSB"`) {
		t.Errorf("RenderGo() under LabelsRequired dropped its labels:\n%s", out)
	}
}

// TestRegisteredProfiles_DeclareTodaysBehaviourExplicitly is the byte-identity
// guard's companion: the three registered radios must SAY that their charts
// are six-digit, labelled and text-bearing, rather than inherit it.
func TestRegisteredProfiles_DeclareTodaysBehaviourExplicitly(t *testing.T) {
	regs := RegisteredProfiles()
	if len(regs) != 3 {
		t.Fatalf("RegisteredProfiles() returned %d entries, want 3 — this check would pass vacuously", len(regs))
	}
	for _, np := range regs {
		if np.Profile.Addresses != AddressTriple {
			t.Errorf("%s: Addresses = %v, want AddressTriple", np.Name, np.Profile.Addresses)
		}
		if np.Profile.LabelPolicy != LabelsRequired {
			t.Errorf("%s: LabelPolicy = %v, want LabelsRequired", np.Name, np.Profile.LabelPolicy)
		}
		if np.Profile.TextRowPolicy != TextRowsAllowed {
			t.Errorf("%s: TextRowPolicy = %v, want TextRowsAllowed", np.Name, np.Profile.TextRowPolicy)
		}
		if np.Profile.TextWidth != 12 {
			t.Errorf("%s: TextWidth = %d, want 12 — the three registered charts all print a 12-byte text row", np.Name, np.Profile.TextWidth)
		}
	}
}
