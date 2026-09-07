// SPDX-License-Identifier: GPL-3.0-or-later

package fakets480

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// This file is the CI guard for the EX projection in exinventory.go: it runs
// the REAL parse over the REAL committed CSV and states, as literals, what that
// artefact structurally contains.
//
// It used to live in this package's gen/ directory and compare rendered bytes
// with a committed generated file. There is no generated file any more — the
// CSV is embedded and projected at init (06/09/2026) — so staleness cannot
// happen and the render tests went with it. What remains is the part that was
// always the real check: the printed row count, the red proofs and every
// refusal. The width perturbation, which used to compare rendered bytes,
// now compares the PROJECTION itself against the one the package holds.

// The committed artefact, read by name rather than through the embedded
// transcriptionB480 — so that reading the file the //go:embed directive names
// proves the directive points where this test thinks it does.
const (
	csvPath = "transcription-b-480.csv"

	// menus is the number of rows this chart's own book prints, written as a
	// literal from the EX block's printed domain ("000 ~ 060: Menu No.",
	// 480:401) rather than counted from the artefact — a count taken from the
	// thing it counts proves nothing.
	menus = 61
)

// parseCommitted parses the committed CSV, failing the test if it will not
// parse.
func parseCommitted(t *testing.T) []row {
	t.Helper()
	data, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("reading %s: %v", csvPath, err)
	}
	rows, err := parseB(data)
	if err != nil {
		t.Fatalf("parseB: %v", err)
	}
	return rows
}

// TestTheCommittedArtefactCarriesThePrintedRowCount pins the projection's
// length against the domain the book prints, from a literal. It is what makes
// the staleness test above impossible to satisfy with a truncated CSV.
func TestTheCommittedArtefactCarriesThePrintedRowCount(t *testing.T) {
	rows := parseCommitted(t)
	if len(rows) != menus {
		t.Errorf("%s carries %d rows, and this chart's printed domain is %d menus (480:401)", csvPath, len(rows), menus)
	}
}

// TestTheTwoDigitMenusAreTheOnesTheGridPrints pins the width distribution
// against the book, from literals — including the printed defect this
// projection carries unchanged.
//
// The EX block's prose lists "Menu No. 32, 35 and 48 ~ 52" (480:411) and OMITS
// 034, whose grid row reaches the second parameter column all the same.
// core/kw/ts480 pins the omission as an erratum of the printed block, and both
// quarantined transcriptions read the GRID. This test states the set that
// results, so a change to it is a visible, deliberate act — and so that the
// alphabet's top ('2') is proved exercised rather than assumed.
func TestTheTwoDigitMenusAreTheOnesTheGridPrints(t *testing.T) {
	want := []int{32, 34, 35, 48, 49, 50, 51, 52}
	var got []int
	for _, r := range parseCommitted(t) {
		if r.token > '0'+maxWidth {
			t.Fatalf("menu %03d carries token %q, past the alphabet's top %d", r.menu, r.token, maxWidth)
		}
		if r.token == '0'+maxWidth {
			got = append(got, r.menu)
		}
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("the two-digit menus are %v, want %v (480:411 plus the menu 034 erratum)", got, want)
	}
}

// TestRedProof_ADroppedRowIsRefused. A row lost from a transcription is the
// defect this compact form is most exposed to, because the string's index IS
// the menu number: silently indexing past a gap would renumber every menu
// after it, and every width would then be attributed to the wrong menu while
// the table still looked well-formed.
//
// So it is REFUSED at projection rather than caught downstream. The proof is
// run on the real artefact, at menu 030 — before the run of two-digit rows, so
// a renumbering would move every one of them.
func TestRedProof_ADroppedRowIsRefused(t *testing.T) {
	data, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("reading %s: %v", csvPath, err)
	}
	const prefix = "030,"
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
		t.Fatalf("no row starts %q — the perturbation did nothing and this proof would be vacuous", prefix)
	}

	rows, perr := parseB([]byte(strings.Join(kept, "\n")))
	if perr != nil {
		t.Fatalf("parseB after dropping menu 030: %v — the rows should still parse; it is the PROJECTION that must refuse", perr)
	}
	if _, err := projectWidths(rows); err == nil {
		t.Error("projectWidths accepted a chart with menu 030 missing — the index would no longer be the menu number")
	}
}

// TestRedProof_AWidthOnlyPerturbationChangesTheProjection. A dropped row is
// caught structurally; a single MIS-READ WIDTH is not, and cannot be — the
// table is still well-formed and still the right length. What catches it is
// that the projection CHANGES, and downstream of that
// core/transport/ex_crosscheck_ts480_test.go compares it against the codec's
// side, which comes from the other transcription.
//
// This proof pins the first half: change one digits cell and nothing else, and
// the projection differs from the one this package holds. If it did not, the
// cross-check would have nothing to compare.
func TestRedProof_AWidthOnlyPerturbationChangesTheProjection(t *testing.T) {
	data, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("reading %s: %v", csvPath, err)
	}
	// Menu 010 is one digit wide; widening it to two touches the digits
	// column and nothing else.
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

	rows, err := parseB([]byte(strings.Join(lines, "\n")))
	if err != nil {
		t.Fatalf("parseB after widening menu 010: %v", err)
	}
	got, err := projectWidths(rows)
	if err != nil {
		t.Fatalf("projectWidths after widening menu 010: %v", err)
	}
	if got == exWidths480 {
		t.Error("widening menu 010 from one digit to two produced an identical projection — it does not depend on the digits column")
	}
}

// TestParseB_RefusesAMalformedArtefact. Every case is a shape this committed,
// hash-frozen artefact could only acquire by being edited or mis-delivered, so
// each is a finding to report rather than a row to drop or repair.
func TestParseB_RefusesAMalformedArtefact(t *testing.T) {
	const good = "menu_number,name,digits,text\n000,Display brightness,1,0\n001,Key illumination,1,0\n"
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
		{"a width past this book's printed two", "menu_number,name,digits,text\n000,Display brightness,3,0\n"},
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

// TestParseB_ReadsTheQuotedCellThisChartActuallyCarries. Menu 035 is "CW
// keying dot, dash weight ratio" — a quoted cell with an embedded comma, which
// a naive splitter would read as five fields and refuse, or worse, misalign.
// The line number it reports is evidence, so it is checked too.
func TestParseB_ReadsTheQuotedCellThisChartActuallyCarries(t *testing.T) {
	rows := parseCommitted(t)
	var got *row
	for i := range rows {
		if rows[i].menu == 35 {
			got = &rows[i]
		}
	}
	if got == nil {
		t.Fatal("menu 035 is absent from the committed artefact")
	}
	if got.token != '2' {
		t.Errorf("menu 035's width token is %q, want '2' (480:411 lists it among the two-digit menus)", got.token)
	}
	// Row n of a gap-free chart numbered from 000 sits on physical line n+2
	// while every preceding cell is unquoted and single-line, which the
	// committed artefact happens to satisfy. Stating it here is what would
	// catch recordLines drifting.
	if want := 35 + 2; got.line != want {
		t.Errorf("menu 035 is reported on line %d, want %d", got.line, want)
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
// cells that break the obvious arithmetic. This chart's own case is menu 035;
// the newline case is the one no committed artefact exercises today, which is
// why it is here.
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
			if got := recordLines([]byte(tt.csv)); fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Errorf("recordLines(%q) = %v, want %v", tt.csv, got, tt.want)
			}
		})
	}
}
