// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
)

// The three artefacts of this generator, as the //go:generate directives in
// ../ex.go name them, with ".." prepended because `go test` runs here.
//
// render writes only the CSV's BASE NAME into its output, so reading the file
// through "../" still produces the bytes the directive produces from
// "transcription-b-590s.csv" — which is what makes the staleness comparison
// below a byte comparison rather than a path lottery.
var charts = []struct {
	name    string
	csvPath string
	outPath string
	varName string
	// menus is the number of rows this chart's own book prints, written as a
	// literal from the EX block's printed domain ("000 ~ 087: Menu number
	// (TS-590S)", "000 ~ 099: Menu number (TS-590SG)", 590:543-544) rather
	// than counted from the artefact — a count taken from the thing it counts
	// proves nothing.
	menus int
}{
	{"TS-590S", "../transcription-b-590s.csv", "../exinventory590s_gen.go", "exWidths590S", 88},
	{"TS-590SG", "../transcription-b-590sg.csv", "../exinventory590sg_gen.go", "exWidths590SG", 100},
}

// renderChart runs the whole projection over one chart's committed CSV and
// returns the generated file's bytes.
func renderChart(t *testing.T, c int) []byte {
	t.Helper()
	data, err := os.ReadFile(charts[c].csvPath)
	if err != nil {
		t.Fatalf("reading %s: %v", charts[c].csvPath, err)
	}
	return renderData(t, c, data)
}

// renderData is renderChart over supplied bytes, for the perturbation proofs.
func renderData(t *testing.T, c int, data []byte) []byte {
	t.Helper()
	rows, err := parseB(data)
	if err != nil {
		t.Fatalf("parseB(%s): %v", charts[c].name, err)
	}
	widths, err := projectWidths(rows)
	if err != nil {
		t.Fatalf("projectWidths(%s): %v", charts[c].name, err)
	}
	out, err := render(widths, rows, charts[c].csvPath, charts[c].varName)
	if err != nil {
		t.Fatalf("render(%s): %v", charts[c].name, err)
	}
	return out
}

// TestGeneratedFilesAreNotStale is the staleness gate: each committed
// generated file must be byte-identical to what this generator produces from
// the committed CSV beside it.
//
// It is what makes "DO NOT EDIT" true rather than merely asked for. A
// hand-edit of either generated table — or a CSV corrected without
// regenerating — would otherwise leave the fake answering one thing while the
// artefact it claims to be a projection of says another, which is exactly the
// drift the two-source cross-check assumes cannot happen on this side.
func TestGeneratedFilesAreNotStale(t *testing.T) {
	for c := range charts {
		t.Run(charts[c].name, func(t *testing.T) {
			want, err := os.ReadFile(charts[c].outPath)
			if err != nil {
				t.Fatalf("reading %s: %v", charts[c].outPath, err)
			}
			if got := renderChart(t, c); !bytes.Equal(got, want) {
				t.Errorf("%s is stale (%d bytes committed, %d bytes generated): run `go generate ./internal/fakets590`", charts[c].outPath, len(want), len(got))
			}
		})
	}
}

// TestGenerationIsIdempotent: two runs over equal input produce byte-identical
// output. Without it the staleness test above could fail intermittently for a
// reason that had nothing to do with the CSV.
func TestGenerationIsIdempotent(t *testing.T) {
	for c := range charts {
		t.Run(charts[c].name, func(t *testing.T) {
			if first, second := renderChart(t, c), renderChart(t, c); !bytes.Equal(first, second) {
				t.Error("two runs over the same CSV produced different bytes")
			}
		})
	}
}

// TestTheCommittedArtefactsCarryThePrintedRowCounts pins each projection's
// length against the domain its book prints, from a literal. It is what makes
// the staleness test above impossible to satisfy with a truncated CSV.
func TestTheCommittedArtefactsCarryThePrintedRowCounts(t *testing.T) {
	for c := range charts {
		t.Run(charts[c].name, func(t *testing.T) {
			data, err := os.ReadFile(charts[c].csvPath)
			if err != nil {
				t.Fatalf("reading %s: %v", charts[c].csvPath, err)
			}
			rows, err := parseB(data)
			if err != nil {
				t.Fatalf("parseB: %v", err)
			}
			if len(rows) != charts[c].menus {
				t.Errorf("%s carries %d rows, and this chart's printed domain is %d menus (590:543-544)", charts[c].csvPath, len(rows), charts[c].menus)
			}
		})
	}
}

// TestTheWidestTokenIsEightAndItComesFromThePowerOnMessage. Without this the
// whole projection could pass over a corpus that never exercised the top of
// the width alphabet — and 8 is the token this family's generator had to be
// widened for, against the FT-891's 5 and the FTdx10's 4.
//
// The addresses are literals from the chart: menu 087 on the S and 001 on the
// SG, both "Power on message ... up to 8 ASCII characters" (590:741, 590:750).
func TestTheWidestTokenIsEightAndItComesFromThePowerOnMessage(t *testing.T) {
	for c, wantMenu := range map[int]int{0: 87, 1: 1} {
		t.Run(charts[c].name, func(t *testing.T) {
			data, err := os.ReadFile(charts[c].csvPath)
			if err != nil {
				t.Fatalf("reading %s: %v", charts[c].csvPath, err)
			}
			rows, err := parseB(data)
			if err != nil {
				t.Fatalf("parseB: %v", err)
			}
			var eights []int
			for _, r := range rows {
				if r.token > '0'+maxWidth {
					t.Fatalf("menu %03d carries token %q, past the alphabet's top %d", r.menu, r.token, maxWidth)
				}
				if r.token == '0'+maxWidth {
					eights = append(eights, r.menu)
				}
			}
			if len(eights) != 1 || eights[0] != wantMenu {
				t.Errorf("the %d-wide rows are %v, want exactly [%d] (the Power on message, 590:741, 590:750)", maxWidth, eights, wantMenu)
			}
		})
	}
}

// --- The red proofs: what a perturbed artefact does ---

// dropRow returns the CSV with the row whose menu_number is menu removed.
func dropRow(t *testing.T, data []byte, menu int) []byte {
	t.Helper()
	prefix := fmt.Sprintf("%03d,", menu)
	var kept []string
	found := false
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, prefix) {
			found = true
			continue
		}
		kept = append(kept, line)
	}
	if !found {
		t.Fatalf("no row starts %q — the perturbation did nothing and the proof below would be vacuous", prefix)
	}
	return []byte(strings.Join(kept, "\n"))
}

// TestRedProof_ADroppedRowIsRefused. A row lost from a transcription is the
// defect this compact form is most exposed to, because the string's index IS
// the menu number: silently indexing past a gap would renumber every menu
// after it, and every width would then be attributed to the wrong menu while
// the table still looked well-formed.
//
// So it is REFUSED at projection rather than caught downstream. The proof is
// run on the real artefact, from the middle of the chart, where a renumbering
// would do the most damage.
func TestRedProof_ADroppedRowIsRefused(t *testing.T) {
	for c := range charts {
		t.Run(charts[c].name, func(t *testing.T) {
			data, err := os.ReadFile(charts[c].csvPath)
			if err != nil {
				t.Fatalf("reading %s: %v", charts[c].csvPath, err)
			}
			rows, perr := parseB(dropRow(t, data, 40))
			if perr != nil {
				t.Fatalf("parseB after dropping menu 040: %v — the row should still parse; it is the PROJECTION that must refuse", perr)
			}
			if _, err := projectWidths(rows); err == nil {
				t.Error("projectWidths accepted a chart with menu 040 missing — the index would no longer be the menu number")
			}
		})
	}
}

// TestRedProof_AWidthOnlyPerturbationChangesTheProjection. A dropped row is
// caught structurally; a single MIS-READ WIDTH is not, and cannot be — the
// table is still well-formed and still the right length. What catches it is
// that the projection CHANGES, and downstream of that
// core/transport/ex_crosscheck_ts590_test.go compares it against the codec's
// side, which comes from the other transcription.
//
// This proof pins the first half: change one digits cell and nothing else, and
// the generated bytes differ from the committed ones. If they did not, the
// cross-check would have nothing to compare.
func TestRedProof_AWidthOnlyPerturbationChangesTheProjection(t *testing.T) {
	for c := range charts {
		t.Run(charts[c].name, func(t *testing.T) {
			data, err := os.ReadFile(charts[c].csvPath)
			if err != nil {
				t.Fatalf("reading %s: %v", charts[c].csvPath, err)
			}
			// Menu 010 is one digit wide on both charts; widening it to two
			// touches the digits column and nothing else.
			const prefix = "010,"
			lines := strings.Split(string(data), "\n")
			changed := false
			for i, line := range lines {
				if !strings.HasPrefix(line, prefix) {
					continue
				}
				parts := strings.Split(line, ",")
				if got := parts[len(parts)-2]; got != "1" {
					t.Fatalf("menu 010's digits cell is %q, and this perturbation assumes 1", got)
				}
				parts[len(parts)-2] = "2"
				lines[i] = strings.Join(parts, ",")
				changed = true
			}
			if !changed {
				t.Fatalf("no row starts %q — the perturbation did nothing", prefix)
			}

			want, err := os.ReadFile(charts[c].outPath)
			if err != nil {
				t.Fatalf("reading %s: %v", charts[c].outPath, err)
			}
			if got := renderData(t, c, []byte(strings.Join(lines, "\n"))); bytes.Equal(got, want) {
				t.Error("widening menu 040 from one digit to two produced byte-identical output — the projection does not depend on the digits column")
			}
		})
	}
}

// --- What the parser refuses, and why each refusal is a refusal ---

// TestParseB_RefusesAMalformedArtefact. Every case is a shape this committed,
// hash-frozen artefact could only acquire by being edited or mis-delivered, so
// each is a finding to report rather than a row to drop or repair.
func TestParseB_RefusesAMalformedArtefact(t *testing.T) {
	const good = "menu_number,name,digits,text\n000,Display brightness,1,0\n001,Back light color,1,0\n"
	for _, tt := range []struct {
		name string
		csv  string
	}{
		{"empty file", ""},
		{"header only", "menu_number,name,digits,text\n"},
		{"the FT-891's three-column schema", "menu_number,name,digits\n0101,AGC FAST DELAY,4\n"},
		{"a reordered header", "menu_number,digits,name,text\n000,1,Display brightness,0\n"},
		{"a two-digit menu number", "menu_number,name,digits,text\n00,Display brightness,1,0\n"},
		{"the FT-891's four-digit MENU Number", "menu_number,name,digits,text\n0101,AGC FAST DELAY,4,0\n"},
		{"a non-digit menu number", "menu_number,name,digits,text\n0O0,Display brightness,1,0\n"},
		{"a blank name", "menu_number,name,digits,text\n000, ,1,0\n"},
		{"a zero width", "menu_number,name,digits,text\n000,Display brightness,0,0\n"},
		{"a width past the alphabet's top", "menu_number,name,digits,text\n000,Display brightness,9,0\n"},
		{"a non-numeric width", "menu_number,name,digits,text\n000,Display brightness,one,0\n"},
		{"a text flag that is neither 0 nor 1", "menu_number,name,digits,text\n000,Display brightness,1,2\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseB([]byte(tt.csv)); err == nil {
				t.Error("parseB accepted it")
			}
		})
	}
	// The control: the same shape, correct, parses. Without it every case
	// above could be passing for a reason unrelated to the defect it names.
	if _, err := parseB([]byte(good)); err != nil {
		t.Errorf("parseB refused a well-formed two-row chart: %v", err)
	}
}

// TestProjectWidths_RefusesAGapAndARepeat, directly rather than through the
// artefact: the string's index IS the menu number, so a chart that skipped one
// or repeated one cannot be modelled by this form at all.
func TestProjectWidths_RefusesAGapAndARepeat(t *testing.T) {
	for _, tt := range []struct {
		name string
		rows []row
	}{
		{"a gap", []row{{menu: 0, token: '1', line: 2}, {menu: 2, token: '1', line: 3}}},
		{"a repeat", []row{{menu: 0, token: '1', line: 2}, {menu: 0, token: '1', line: 3}}},
		{"not starting at 000", []row{{menu: 1, token: '1', line: 2}}},
		{"descending", []row{{menu: 0, token: '1', line: 2}, {menu: 1, token: '1', line: 3}, {menu: 1, token: '1', line: 4}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := projectWidths(tt.rows); err == nil {
				t.Error("projectWidths accepted it")
			}
		})
	}
}

// TestRecordLines_CountsAQuotedCellCorrectly. The line numbers this generator
// writes into its output are evidence — they are how a reader gets from a
// width token back to the transcribed row — so the counter has to survive the
// cells that break the obvious arithmetic. This family's own case is the
// TS-480's "CW keying dot, dash weight ratio"; the newline case is the one no
// committed artefact exercises today, which is why it is here.
func TestRecordLines_CountsAQuotedCellCorrectly(t *testing.T) {
	for _, tt := range []struct {
		name string
		csv  string
		want []int
	}{
		{"plain rows", "h\na\nb\n", []int{1, 2, 3}},
		{"a quoted comma", "h\n\"a,b\"\nc\n", []int{1, 2, 3}},
		{"a quoted newline", "h\n\"a\nb\"\nc\n", []int{1, 2, 4}},
		{"CRLF", "h\r\na\r\n", []int{1, 2}},
		{"no trailing newline", "h\na", []int{1, 2}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := recordLines([]byte(tt.csv))
			if fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Errorf("recordLines(%q) = %v, want %v", tt.csv, got, tt.want)
			}
		})
	}
}

// TestRender_WritesOnlyTheCSVsBaseName, which is what lets this test file read
// the artefacts through "../" and still compare bytes with the file the
// //go:generate directive produced from a bare filename.
func TestRender_WritesOnlyTheCSVsBaseName(t *testing.T) {
	out := renderChart(t, 0)
	if !bytes.Contains(out, []byte("transcription-b-590s.csv")) {
		t.Error("the generated file does not name its source CSV")
	}
	if bytes.Contains(out, []byte("../")) {
		t.Error("the generated file carries a relative path — where the generator was invoked from has leaked into the committed bytes")
	}
}
