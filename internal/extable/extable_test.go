// SPDX-License-Identifier: GPL-3.0-or-later

package extable

import (
	"bytes"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// goodRow is a minimal, valid one-row CSV body (no provenance header) used
// as the baseline the strictness cases mutate.
const goodRow = "01,01,01,RADIO SETTING,MODE SSB,AF TREBLE GAIN,-20 - +10,3,false,646\n"

func TestParseCSV_Strictness(t *testing.T) {
	cases := []struct {
		name    string
		csv     string
		wantErr bool
	}{
		{
			name: "valid single row with comment header",
			csv:  "# columns: p1,p2,p3,p1_label,p2_label,name,p4,digits,text,manual_line\n" + goodRow,
		},
		{
			name: "valid text row digits 12",
			csv:  "04,01,01,DISPLAY SETTING,DISPLAY,MY CALL,Up to 12 characters,12,true,879\n",
		},
		{
			name:    "duplicate triple",
			csv:     goodRow + goodRow,
			wantErr: true,
		},
		{
			name:    "digits 0 on non-text row",
			csv:     "01,01,01,RADIO SETTING,MODE SSB,AF TREBLE GAIN,x,0,false,646\n",
			wantErr: true,
		},
		{
			name:    "digits 5 on non-text row",
			csv:     "01,01,01,RADIO SETTING,MODE SSB,AF TREBLE GAIN,x,5,false,646\n",
			wantErr: true,
		},
		{
			name:    "text row with digits not 12",
			csv:     "04,01,01,DISPLAY SETTING,DISPLAY,MY CALL,Up to 12 characters,3,true,879\n",
			wantErr: true,
		},
		{
			name:    "missing column",
			csv:     "01,01,01,RADIO SETTING,MODE SSB,AF TREBLE GAIN,x,3,false\n",
			wantErr: true,
		},
		{
			name:    "non-integer P1",
			csv:     "AA,01,01,RADIO SETTING,MODE SSB,AF TREBLE GAIN,x,3,false,646\n",
			wantErr: true,
		},
		{
			name:    "non-boolean text flag",
			csv:     "01,01,01,RADIO SETTING,MODE SSB,AF TREBLE GAIN,x,3,maybe,646\n",
			wantErr: true,
		},
		{
			name:    "blank p1_label",
			csv:     "01,01,01,,MODE SSB,AF TREBLE GAIN,x,3,false,646\n",
			wantErr: true,
		},
		{
			name:    "whitespace-only p1_label",
			csv:     "01,01,01,   ,MODE SSB,AF TREBLE GAIN,x,3,false,646\n",
			wantErr: true,
		},
		{
			name:    "blank p2_label",
			csv:     "01,01,01,RADIO SETTING,,AF TREBLE GAIN,x,3,false,646\n",
			wantErr: true,
		},
		{
			name:    "whitespace-only p2_label",
			csv:     "01,01,01,RADIO SETTING,   ,AF TREBLE GAIN,x,3,false,646\n",
			wantErr: true,
		},
		{
			name:    "blank name",
			csv:     "01,01,01,RADIO SETTING,MODE SSB,,x,3,false,646\n",
			wantErr: true,
		},
		{
			name:    "whitespace-only name",
			csv:     "01,01,01,RADIO SETTING,MODE SSB,   ,x,3,false,646\n",
			wantErr: true,
		},
		{
			name:    "blank p4",
			csv:     "01,01,01,RADIO SETTING,MODE SSB,AF TREBLE GAIN,,3,false,646\n",
			wantErr: true,
		},
		{
			name:    "whitespace-only p4",
			csv:     "01,01,01,RADIO SETTING,MODE SSB,AF TREBLE GAIN,   ,3,false,646\n",
			wantErr: true,
		},
		{
			name:    "manual_line zero",
			csv:     "01,01,01,RADIO SETTING,MODE SSB,AF TREBLE GAIN,x,3,false,0\n",
			wantErr: true,
		},
		{
			name:    "manual_line negative",
			csv:     "01,01,01,RADIO SETTING,MODE SSB,AF TREBLE GAIN,x,3,false,-1\n",
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseCSV(withRows(FT710Profile(), 1), []byte(tc.csv))
			if tc.wantErr && err == nil {
				t.Fatalf("ParseCSV(%q): expected error, got nil", tc.csv)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("ParseCSV(%q): unexpected error: %v", tc.csv, err)
			}
		})
	}
}

// TestParseCSV_FieldsParsed checks that a valid row is decoded into the
// expected Row (including leading-zero fields and the P4 comma passthrough).
func TestParseCSV_FieldsParsed(t *testing.T) {
	// P4 carries a comma, so the field must be CSV-quoted.
	csv := "03,01,05,OPERATION SETTING,GENERAL,CAT-1 RATE,\"0: 4800 bps, 1: 9600 bps\",1,false,801\n"
	rows, err := ParseCSV(withRows(FT710Profile(), 1), []byte(csv))
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	got := rows[0]
	want := Row{
		P1: 3, P2: 1, P3: 5,
		P1Label: "OPERATION SETTING", P2Label: "GENERAL", Name: "CAT-1 RATE",
		P4:     "0: 4800 bps, 1: 9600 bps",
		Digits: 1, Text: false, ManualLine: 801,
	}
	if got != want {
		t.Errorf("row = %+v, want %+v", got, want)
	}
}

func TestRenderGo_Deterministic(t *testing.T) {
	rows := []Row{
		// Deliberately out of (P1,P2,P3) order to prove RenderGo sorts.
		{P1: 6, P2: 1, P3: 1, P1Label: "EXTENSION SETTING", P2Label: "PRESET1", Name: "PRESET NAME", P4: "Up to 12 characters", Digits: 12, Text: true, ManualLine: 895},
		{P1: 1, P2: 1, P3: 1, P1Label: "RADIO SETTING", P2Label: "MODE SSB", Name: "AF TREBLE GAIN", P4: "-20 - +10", Digits: 3, Text: false, ManualLine: 646},
		{P1: 1, P2: 1, P3: 2, P1Label: "RADIO SETTING", P2Label: "MODE SSB", Name: "ENC/DEC", P4: "0: OFF", Digits: 1, Text: false, ManualLine: 647},
	}

	observed := map[string]Observed{
		"060101": {ReadWidth: 12, ReadShape: "text"},
		"010101": {ReadWidth: 3, ReadShape: "signed"},
		"010102": {ReadWidth: 1, ReadShape: "numeric"},
	}

	first, err := RenderGo(withRows(FT710Profile(), 3), rows, observed)
	if err != nil {
		t.Fatalf("RenderGo (first): %v", err)
	}
	second, err := RenderGo(withRows(FT710Profile(), 3), rows, observed)
	if err != nil {
		t.Fatalf("RenderGo (second): %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("RenderGo not deterministic: two renders of the same rows differ")
	}

	// Output must be syntactically valid Go.
	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "exinventory_gen.go", first, 0); err != nil {
		t.Fatalf("RenderGo output does not parse: %v", err)
	}

	out := string(first)
	// Sorted: RADIO SETTING (P1=1) must precede EXTENSION SETTING (P1=6).
	if i, j := strings.Index(out, "RADIO SETTING"), strings.Index(out, "EXTENSION SETTING"); i < 0 || j < 0 || i > j {
		t.Errorf("rows not sorted by (P1,P2,P3): RADIO SETTING at %d, EXTENSION SETTING at %d", i, j)
	}
	// Mandatory generated-file markers.
	for _, marker := range []string{
		"// SPDX-License-Identifier: GPL-3.0-or-later",
		"// Code generated by internal/extable/gen from table2.csv and",
		"// table2-observed.csv. DO NOT EDIT.",
		"package cat",
		"// manual line 646",
	} {
		if !strings.Contains(out, marker) {
			t.Errorf("RenderGo output missing marker %q", marker)
		}
	}
	// P4 is an audit-only CSV column and must NOT leak into the generated Go.
	if strings.Contains(out, "Up to 12 characters") {
		t.Error("RenderGo output leaked the P4 description column into generated Go")
	}
}

// --- M8c hardware READ-observation overlay (task 46) ---

// observedBody is a minimal, valid two-row observation CSV body (no
// provenance header) used as the baseline the strictness cases mutate.
const observedBody = "01,01,01,3,signed\n01,03,21,3,numeric\n"

func TestParseObservedCSV_Valid(t *testing.T) {
	got, err := ParseObservedCSV(FT710Profile(), []byte("# provenance comment\n"+observedBody))
	if err != nil {
		t.Fatalf("ParseObservedCSV: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("parsed %d observations, want 2", len(got))
	}
	if want := (Observed{ReadWidth: 3, ReadShape: "signed"}); got["010101"] != want {
		t.Errorf("010101 = %+v, want %+v", got["010101"], want)
	}
	if want := (Observed{ReadWidth: 3, ReadShape: "numeric"}); got["010321"] != want {
		t.Errorf("010321 = %+v, want %+v", got["010321"], want)
	}
}

// TestParseObservedCSV_Strictness pins the artefact's closed vocabulary.
// Every case here is a way a malformed or value-bearing artefact could
// otherwise reach the generated inventory.
func TestParseObservedCSV_Strictness(t *testing.T) {
	cases := []struct {
		name string
		csv  string
	}{
		{"too few fields", "01,01,01,3\n"},
		{"too many fields", "01,01,01,3,signed,extra\n"},
		{"one-digit P1", "1,01,01,3,signed\n"},
		{"three-digit P3", "01,01,001,3,signed\n"},
		{"non-numeric component", "0A,01,01,3,signed\n"},
		{"zero width", "01,01,01,0,numeric\n"},
		{"negative width", "01,01,01,-1,numeric\n"},
		{"width above the 12-byte text maximum", "01,01,01,13,numeric\n"},
		{"non-numeric width", "01,01,01,three,numeric\n"},
		{"unknown shape", "01,01,01,3,enum\n"},
		{"empty shape", "01,01,01,3,\n"},
		{"duplicate address", observedBody + "01,01,01,3,signed\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseObservedCSV(FT710Profile(), []byte(tc.csv)); err == nil {
				t.Error("ParseObservedCSV accepted a malformed artefact; want an error")
			}
		})
	}
}

// TestRenderGo_RequiresExactObservationCoverage proves the join is
// set-equal in both directions: an inventory row with no observation, and
// an observation for an address the inventory lacks, are each refused.
// Silence in either direction would leave ObservedReadWidth quietly zero.
//
// The first two subtests vary CARDINALITY, so the len(observed) != len(rows)
// gate alone refuses both and the per-address lookup inside the row loop is
// never the thing that says no. The third holds cardinality equal and moves
// the address instead: only the membership check can refuse it, so deleting
// that check now turns the suite red.
func TestRenderGo_RequiresExactObservationCoverage(t *testing.T) {
	rows := []Row{
		{P1: 1, P2: 1, P3: 1, P1Label: "RADIO SETTING", P2Label: "MODE SSB", Name: "AF TREBLE GAIN", P4: "-20 - +10", Digits: 3, Text: false, ManualLine: 646},
	}
	t.Run("missing observation", func(t *testing.T) {
		if _, err := RenderGo(withRows(FT710Profile(), 1), rows, map[string]Observed{}); err == nil {
			t.Error("RenderGo accepted an inventory row with no observation; want an error")
		}
	})
	t.Run("extra observation", func(t *testing.T) {
		observed := map[string]Observed{
			"010101": {ReadWidth: 3, ReadShape: "signed"},
			"999999": {ReadWidth: 1, ReadShape: "numeric"},
		}
		if _, err := RenderGo(withRows(FT710Profile(), 1), rows, observed); err == nil {
			t.Error("RenderGo accepted an observation for an address the inventory lacks; want an error")
		}
	})
	t.Run("wrong address at equal cardinality", func(t *testing.T) {
		// One row, one observation, ExpectedRows 1: both counting gates are
		// satisfied and the sets are still disjoint. The generated item would
		// otherwise carry the absence sentinels of a model that HAS hardware.
		observed := map[string]Observed{
			"999999": {ReadWidth: 1, ReadShape: "numeric"},
		}
		if _, err := RenderGo(withRows(FT710Profile(), 1), rows, observed); err == nil {
			t.Error("RenderGo accepted an observation set of the right size for the wrong address; want an error")
		}
	})
}

// TestRenderGo_EmitsObservedFields proves the evidence reaches the
// generated file, under names that say READ — nothing here licenses a Set
// frame width (M8c is read-direction only).
func TestRenderGo_EmitsObservedFields(t *testing.T) {
	rows := []Row{
		{P1: 1, P2: 3, P3: 21, P1Label: "RADIO SETTING", P2Label: "MODE FM", Name: "TONE FREQ", P4: "00: 67.0 - 49: 254.1Hz", Digits: 2, Text: false, ManualLine: 711},
	}
	observed := map[string]Observed{"010321": {ReadWidth: 3, ReadShape: "numeric"}}
	out, err := RenderGo(withRows(FT710Profile(), 1), rows, observed)
	if err != nil {
		t.Fatalf("RenderGo: %v", err)
	}
	for _, want := range []string{"ObservedReadWidth: 3", `ObservedReadShape: "numeric"`, "Digits: 2"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("generated output does not contain %q — both the manual's width and the observed read width must survive", want)
		}
	}
}

// TestRenderGo_PairProfileKeysObservationsByFourDigitForm is the S0-close
// review's LOW-3/MEDIUM-2 fix: the observation lookup key must be rendered
// in the profile's OWN address form, not always six digits — on BOTH sides
// of the join. LOW-3 (wave 1) fixed RenderGo's lookup key alone; the
// wave-1 version of this test then bypassed ParseObservedCSV entirely,
// hand-building a four-digit-keyed map, which is exactly what let
// MEDIUM-2's defect — ParseObservedCSV itself still always keyed six
// digits, so a real Pair-form observation CSV could never actually reach
// RenderGo's fixed lookup — pass unnoticed. This version goes THROUGH
// ParseObservedCSV on a genuine Pair-form CSV row, so the two sides'
// disagreement would fail here.
//
// fixturePairRequired exists only for this test (fixture-only, no
// registered profile changes): see profile_test.go. The Triple path is
// UNCHANGED by this fix — ParseObservedCSV's AddressTriple case still
// builds the same six-digit key it always did — which is what keeps the
// FT-710's committed observation CSV and generated inventory byte-identical
// (core/cat's own staleness test re-derives and byte-compares the FT-710's
// generated file on every run, so a Triple-path regression here would fail
// there, not just here).
func TestRenderGo_PairProfileKeysObservationsByFourDigitForm(t *testing.T) {
	rows := []Row{
		{P1: 8, P2: 1, P3: 0, P1Label: "RADIO", P2Label: "GROUP", Name: "ITEM", P4: "x", Digits: 4, Text: false, ManualLine: 1},
	}
	// A genuine Pair-form observation CSV row: p3 is "00", the one value
	// AddressPair permits, and it must NOT appear in the lookup key below.
	observedCSV := "08,01,00,4,numeric\n"
	profile := withRows(fixturePairRequired, 1)
	observed, err := ParseObservedCSV(profile, []byte(observedCSV))
	if err != nil {
		t.Fatalf("ParseObservedCSV: %v", err)
	}
	if _, ok := observed["0801"]; !ok {
		t.Fatalf("ParseObservedCSV keyed the row as %v, want a four-digit key \"0801\" under AddressPair", observed)
	}
	out, err := RenderGo(profile, rows, observed)
	if err != nil {
		t.Fatalf("RenderGo: %v — a real Pair-form observation CSV, parsed by ParseObservedCSV and keyed under this profile's own AddressPair form, must be found by RenderGo's matching lookup", err)
	}
	if !strings.Contains(string(out), "ObservedReadWidth: 4") {
		t.Errorf("generated output does not carry the four-digit-keyed observation")
	}
}

// TestParseObservedCSV_PairFormRefusesNonZeroP3 pins the other half of the
// MEDIUM-2 fix: under AddressPair, p3 is not on the wire (parseRecord
// enforces the identical rule for the inventory CSV's own P3), so a
// non-zero p3 in the observation CSV is a component the radio can never
// have answered with — refused, not silently dropped from the key.
func TestParseObservedCSV_PairFormRefusesNonZeroP3(t *testing.T) {
	profile := withRows(fixturePairRequired, 1)
	if _, err := ParseObservedCSV(profile, []byte("08,01,01,4,numeric\n")); err == nil {
		t.Error("ParseObservedCSV accepted a non-zero p3 under AddressPair; want a refusal")
	} else if !strings.Contains(err.Error(), "p3 must be 0") {
		t.Errorf("ParseObservedCSV refused with %q, which does not say p3 must be 0", err)
	}
}

// TestParseCSV_BoundsComeFromProfile proves each digit bound is READ from
// the profile rather than hardcoded, by asserting rows that are legal under
// one profile and illegal under the other — in both directions. A test that
// only checked one direction would pass with the bounds still constant.
func TestParseCSV_BoundsComeFromProfile(t *testing.T) {
	const (
		text12  = "04,01,01,DISPLAY SETTING,DISPLAY,MY CALL,Up to 12 characters,12,true,879\n"
		text8   = "04,01,01,DISPLAY SETTING,DISPLAY,MY CALL,Up to 8 characters,8,true,879\n"
		digits1 = "01,01,01,RADIO SETTING,MODE SSB,AF TREBLE GAIN,x,1,false,646\n"
		digits6 = "01,01,01,RADIO SETTING,MODE SSB,AF TREBLE GAIN,x,6,false,646\n"
	)
	ft := withRows(FT710Profile(), 1)
	fx := withRows(fixtureRequired, 1)

	cases := []struct {
		name    string
		profile Profile
		csv     string
		wantErr bool
	}{
		{"text width 12 under FT-710", ft, text12, false},
		{"text width 12 under fixture", fx, text12, true},
		{"text width 8 under fixture", fx, text8, false},
		{"text width 8 under FT-710", ft, text8, true},
		{"digits 1 under FT-710", ft, digits1, false},
		{"digits 1 under fixture", fx, digits1, true},
		{"digits 6 under fixture", fx, digits6, false},
		{"digits 6 under FT-710", ft, digits6, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseCSV(tc.profile, []byte(tc.csv))
			if tc.wantErr && err == nil {
				t.Error("ParseCSV accepted a row its profile forbids; want an error")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("ParseCSV rejected a row its profile permits: %v", err)
			}
		})
	}
}

// TestParseCSV_AddressComponentRange closes a gate that used to exist only
// by accident. ParseCSV never range-checked P1/P2/P3; the observation CSV's
// exactly-two-digits rule rejected the matching row instead. Under
// ObservationsAbsent there is no observation CSV, so a component of 100
// would render into a seven-digit wire address under the six-digit
// (AddressTriple) form — one digit wider than the field the radio's own
// EX frame carries.
func TestParseCSV_AddressComponentRange(t *testing.T) {
	cases := []struct {
		name    string
		csv     string
		wantErr bool
	}{
		{"99 accepted", "99,01,01,RADIO SETTING,MODE SSB,AF TREBLE GAIN,x,3,false,646\n", false},
		{"P1 100 rejected", "100,01,01,RADIO SETTING,MODE SSB,AF TREBLE GAIN,x,3,false,646\n", true},
		{"P2 100 rejected", "01,100,01,RADIO SETTING,MODE SSB,AF TREBLE GAIN,x,3,false,646\n", true},
		{"P3 100 rejected", "01,01,100,RADIO SETTING,MODE SSB,AF TREBLE GAIN,x,3,false,646\n", true},
		{"negative P1 rejected", "-1,01,01,RADIO SETTING,MODE SSB,AF TREBLE GAIN,x,3,false,646\n", true},
	}
	// fixtureAbsent, because the motivating exposure is the manual-only
	// regime, where no observation-side two-digit rule exists to catch a
	// bad component indirectly. (Its digit bounds 2..6 admit these rows.)
	p := withRows(fixtureAbsent, 1)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseCSV(p, []byte(tc.csv))
			if tc.wantErr && err == nil {
				t.Error("ParseCSV accepted an out-of-range address component; want an error")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("ParseCSV rejected a valid address: %v", err)
			}
		})
	}
}

// TestParseCSV_RefusesInvalidProfile pins that the API validates its own
// profile: the registry cannot vouch for a profile that never went through
// it.
//
// The invalidity is a BLANK MODEL, and the CSV is a row the fixture's own
// 2..6 digit bounds accept. ParseCSV's own logic never consults Model — it
// reads TextWidth, MinDigits and MaxDigits, and none of its error strings
// name the model — so this call succeeds the moment the p.Validate() call
// is deleted. That is the point: an earlier version passed Profile{}, whose
// zero bounds refuse every row downstream of the validation, so the test
// stayed green with the validation removed and pinned nothing.
func TestParseCSV_RefusesInvalidProfile(t *testing.T) {
	p := withRows(fixtureRequired, 1)
	p.Model = ""
	if _, err := ParseCSV(p, []byte(goodRow)); err == nil {
		t.Error("ParseCSV accepted a profile with a blank Model; want a validation error")
	}
}

// TestParseObservedCSV_CeilingComesFromProfile is the test that kills
// revision 1's derived ceiling. The fixture's MaxDigits is 6 and its
// TextWidth is 8, so a ceiling still computed as max(MaxDigits, TextWidth)
// would be 8 and would wrongly REJECT a width of 9. The rejection case
// alone passes under either implementation and proves nothing — the pair is
// the point.
func TestParseObservedCSV_CeilingComesFromProfile(t *testing.T) {
	cases := []struct {
		name    string
		profile Profile
		csv     string
		wantErr bool
	}{
		{"width 9 under the fixture's ceiling of 9", fixtureRequired, "01,01,01,9,numeric\n", false},
		{"width 10 above the fixture's ceiling of 9", fixtureRequired, "01,01,01,10,numeric\n", true},
		{"width 10 under the FT-710's ceiling of 12", FT710Profile(), "01,01,01,10,numeric\n", false},
		{"width 13 above the FT-710's ceiling of 12", FT710Profile(), "01,01,01,13,numeric\n", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseObservedCSV(tc.profile, []byte(tc.csv))
			if tc.wantErr && err == nil {
				t.Error("ParseObservedCSV accepted a width above its profile's ceiling; want an error")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("ParseObservedCSV rejected a width its profile permits: %v", err)
			}
		})
	}
}

// TestParseObservedCSV_RefusesInvalidProfile mirrors ParseCSV's pin, and
// bites for the same reason: a blank Model is invalid, yet the widths in
// observedBody are 3, comfortably inside the fixture's ceiling of 9, and
// nothing in ParseObservedCSV reads Model. Delete the p.Validate() call and
// this parse succeeds.
func TestParseObservedCSV_RefusesInvalidProfile(t *testing.T) {
	p := withRows(fixtureRequired, 1)
	p.Model = ""
	if _, err := ParseObservedCSV(p, []byte(observedBody)); err == nil {
		t.Error("ParseObservedCSV accepted a profile with a blank Model; want a validation error")
	}
}

// TestRenderGo_IdentityComesFromProfile proves the generated file's package
// clause, import, variable name and type qualification are all read from the
// profile. The FT-710 must emit none of the qualified forms; the fixture
// must emit all of them.
func TestRenderGo_IdentityComesFromProfile(t *testing.T) {
	rows := []Row{
		{P1: 1, P2: 1, P3: 1, P1Label: "RADIO SETTING", P2Label: "MODE SSB", Name: "AF TREBLE GAIN", P4: "-20 - +10", Digits: 3, Text: false, ManualLine: 646},
	}
	observed := map[string]Observed{"010101": {ReadWidth: 3, ReadShape: "signed"}}

	t.Run("fixture emits qualified identity", func(t *testing.T) {
		out, err := RenderGo(withRows(fixtureRequired, 1), rows, observed)
		if err != nil {
			t.Fatalf("RenderGo: %v", err)
		}
		for _, want := range []string{
			"package ftdx10",
			`import cat "github.com/gm5dna/open-rig-programmer/core/cat"`,
			"var exItems = []cat.EXItem{",
			"{Addr: cat.EXAddress{",
			"// exItems is the fixture inventory.",
			"// Code generated by internal/extable/gen from fixture.csv and",
			"// fixture-observed.csv. DO NOT EDIT.",
		} {
			if !strings.Contains(string(out), want) {
				t.Errorf("fixture output missing %q", want)
			}
		}
	})

	t.Run("FT-710 emits unqualified identity", func(t *testing.T) {
		out, err := RenderGo(withRows(FT710Profile(), 1), rows, observed)
		if err != nil {
			t.Fatalf("RenderGo: %v", err)
		}
		s := string(out)
		for _, want := range []string{"package cat", "var exItemsGen = []EXItem{", "{Addr: EXAddress{"} {
			if !strings.Contains(s, want) {
				t.Errorf("FT-710 output missing %q", want)
			}
		}
		for _, unwanted := range []string{"cat.EXItem", "cat.EXAddress", "import "} {
			if strings.Contains(s, unwanted) {
				t.Errorf("FT-710 output must not contain %q", unwanted)
			}
		}
	})
}

// TestRenderGo_RefusesInvalidProfile mirrors the parsers' pins. The inputs
// are a complete render under the required fixture — one row, one matching
// observation, ExpectedRows 1 — so every gate downstream of the validation
// is satisfied and only the blank Model stands between this call and a
// successful render. RenderGo names p.Model in error text but never reads
// it otherwise, so deleting the p.Validate() call turns this into a
// successful render and the test red.
func TestRenderGo_RefusesInvalidProfile(t *testing.T) {
	p := withRows(fixtureRequired, 1)
	p.Model = ""
	rows := []Row{
		{P1: 1, P2: 1, P3: 1, P1Label: "RADIO SETTING", P2Label: "MODE SSB", Name: "AF TREBLE GAIN", P4: "-20 - +10", Digits: 3, Text: false, ManualLine: 646},
	}
	observed := map[string]Observed{"010101": {ReadWidth: 3, ReadShape: "signed"}}
	if _, err := RenderGo(p, rows, observed); err == nil {
		t.Error("RenderGo accepted a profile with a blank Model; want a validation error")
	}
}

// TestRenderGo_ObservationRegimes pins both regimes, including the states
// revision 1 of the spec wrongly claimed were impossible.
func TestRenderGo_ObservationRegimes(t *testing.T) {
	row := Row{P1: 1, P2: 1, P3: 1, P1Label: "RADIO SETTING", P2Label: "MODE SSB", Name: "AF TREBLE GAIN", P4: "-20 - +10", Digits: 3, Text: false, ManualLine: 646}
	obs := map[string]Observed{"010101": {ReadWidth: 3, ReadShape: "signed"}}

	t.Run("absent regime refuses a non-empty map", func(t *testing.T) {
		if _, err := RenderGo(withRows(fixtureAbsent, 1), []Row{row}, obs); err == nil {
			t.Error("RenderGo accepted observations under ObservationsAbsent; want an error")
		}
	})
	t.Run("absent regime accepts an empty map", func(t *testing.T) {
		if _, err := RenderGo(withRows(fixtureAbsent, 1), []Row{row}, nil); err != nil {
			t.Errorf("RenderGo refused a valid manual-only render: %v", err)
		}
	})
	t.Run("jointly truncated sources are refused", func(t *testing.T) {
		// Both sides consistently one row short. Nothing in the set-equality
		// check can see this: the two supplied sets agree with each other.
		if _, err := RenderGo(withRows(FT710Profile(), 2), []Row{row}, obs); err == nil {
			t.Error("RenderGo accepted an inventory one row short of ExpectedRows; want an error")
		}
	})
	t.Run("both sources empty are refused", func(t *testing.T) {
		if _, err := RenderGo(FT710Profile(), nil, map[string]Observed{}); err == nil {
			t.Error("RenderGo accepted two empty sources; want an error")
		}
		if _, err := RenderGo(fixtureAbsent, nil, nil); err == nil {
			t.Error("RenderGo accepted an empty manual-only inventory; want an error")
		}
	})
	t.Run("exact row count is accepted", func(t *testing.T) {
		if _, err := RenderGo(withRows(FT710Profile(), 1), []Row{row}, obs); err != nil {
			t.Errorf("RenderGo refused a complete inventory: %v", err)
		}
	})
	t.Run("manual-only profile renders and names one source", func(t *testing.T) {
		// Moved here from Task 4 by plan review: this render can only
		// succeed once THIS task's regime switch replaces the old
		// unconditional coverage rule.
		out, err := RenderGo(withRows(fixtureAbsent, 1), []Row{row}, nil)
		if err != nil {
			t.Fatalf("RenderGo: %v", err)
		}
		s := string(out)
		if !strings.Contains(s, "// Code generated by internal/extable/gen from fixture.csv. DO NOT EDIT.") {
			t.Error("manual-only output should name exactly one source CSV")
		}
		if strings.Contains(s, "fixture-observed.csv") {
			t.Error("manual-only output must not name an observation CSV it has none of")
		}
		if !strings.Contains(s, `ObservedReadWidth: 0`) || !strings.Contains(s, `ObservedReadShape: ""`) {
			t.Error("manual-only rows must render the documented absence sentinels")
		}
	})
}
