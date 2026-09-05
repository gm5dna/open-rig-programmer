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
// guard's companion: every registered radio must SAY what shape its chart is,
// rather than inherit it.
//
// Until the FT-891 there was one population and the assertion could be a
// blanket "all three are six-digit, labelled and text-bearing". There are now
// two, so the expectations are stated PER REGISTRATION: the FT-710, FTdx10
// and FTdx101D/MP keep the values whose byte identity this guard protects,
// and the FT-891 declares the opposite of all three at once. Keeping it a
// blanket over RegisteredProfiles() would have meant weakening it to the
// intersection of two charts, which is no assertion at all.
//
// The table is keyed by lookup name and its size is compared against the
// registry's, so a FIFTH registration fails here rather than slipping through
// a sweep that never looked at it.
func TestRegisteredProfiles_DeclareTodaysBehaviourExplicitly(t *testing.T) {
	want := map[string]struct {
		addr      AddressForm
		labels    Labels
		textRows  TextRows
		textWidth int
	}{
		"ft710":   {AddressTriple, LabelsRequired, TextRowsAllowed, 12},
		"ftdx10":  {AddressTriple, LabelsRequired, TextRowsAllowed, 12},
		"ftdx101": {AddressTriple, LabelsRequired, TextRowsAllowed, 12},
		// The FT-891's chart prints a four-digit MENU Number, no group
		// labels and no free-text row: core/cat/ft891/table2.csv's
		// provenance header records all three as readings of that chart.
		"ft891": {AddressPair, LabelsAbsent, TextRowsAbsent, 0},
	}
	regs := RegisteredProfiles()
	if len(regs) != len(want) {
		t.Fatalf("RegisteredProfiles() returned %d entries, want %d — a registration this table does not name would pass vacuously", len(regs), len(want))
	}
	for _, np := range regs {
		w, ok := want[np.Name]
		if !ok {
			t.Errorf("registration %q is not named by this table; state its chart's shape here", np.Name)
			continue
		}
		if np.Profile.Addresses != w.addr {
			t.Errorf("%s: Addresses = %v, want %v", np.Name, np.Profile.Addresses, w.addr)
		}
		if np.Profile.LabelPolicy != w.labels {
			t.Errorf("%s: LabelPolicy = %v, want %v", np.Name, np.Profile.LabelPolicy, w.labels)
		}
		if np.Profile.TextRowPolicy != w.textRows {
			t.Errorf("%s: TextRowPolicy = %v, want %v", np.Name, np.Profile.TextRowPolicy, w.textRows)
		}
		if np.Profile.TextWidth != w.textWidth {
			t.Errorf("%s: TextWidth = %d, want %d", np.Name, np.Profile.TextWidth, w.textWidth)
		}
	}
}
