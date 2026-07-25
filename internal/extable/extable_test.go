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
			_, err := ParseCSV([]byte(tc.csv))
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
	rows, err := ParseCSV([]byte(csv))
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

	first, err := RenderGo(rows, observed)
	if err != nil {
		t.Fatalf("RenderGo (first): %v", err)
	}
	second, err := RenderGo(rows, observed)
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
	got, err := ParseObservedCSV([]byte("# provenance comment\n" + observedBody))
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
			if _, err := ParseObservedCSV([]byte(tc.csv)); err == nil {
				t.Error("ParseObservedCSV accepted a malformed artefact; want an error")
			}
		})
	}
}

// TestRenderGo_RequiresExactObservationCoverage proves the join is
// set-equal in both directions: an inventory row with no observation, and
// an observation for an address the inventory lacks, are each refused.
// Silence in either direction would leave ObservedReadWidth quietly zero.
func TestRenderGo_RequiresExactObservationCoverage(t *testing.T) {
	rows := []Row{
		{P1: 1, P2: 1, P3: 1, P1Label: "RADIO SETTING", P2Label: "MODE SSB", Name: "AF TREBLE GAIN", P4: "-20 - +10", Digits: 3, Text: false, ManualLine: 646},
	}
	t.Run("missing observation", func(t *testing.T) {
		if _, err := RenderGo(rows, map[string]Observed{}); err == nil {
			t.Error("RenderGo accepted an inventory row with no observation; want an error")
		}
	})
	t.Run("extra observation", func(t *testing.T) {
		observed := map[string]Observed{
			"010101": {ReadWidth: 3, ReadShape: "signed"},
			"999999": {ReadWidth: 1, ReadShape: "numeric"},
		}
		if _, err := RenderGo(rows, observed); err == nil {
			t.Error("RenderGo accepted an observation for an address the inventory lacks; want an error")
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
	out, err := RenderGo(rows, observed)
	if err != nil {
		t.Fatalf("RenderGo: %v", err)
	}
	for _, want := range []string{"ObservedReadWidth: 3", `ObservedReadShape: "numeric"`, "Digits: 2"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("generated output does not contain %q — both the manual's width and the observed read width must survive", want)
		}
	}
}
