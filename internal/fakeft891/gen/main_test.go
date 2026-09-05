// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This file is the CI guard for fakeft891's EX generator, and it lives HERE
// rather than in package fakeft891 for one structural reason: the projection
// logic is in `package main`, which no Go package can import. A staleness test
// in fakeft891 would have to re-implement the projection to compare against it,
// and a check written against a second implementation of the thing it is
// checking is a weaker check than running the real one.
//
// So the test runs the ACTUAL generator's parse and render over the ACTUAL
// committed CSV and byte-compares the result with the committed
// exinventory_gen.go — the discipline of core/cat/ft891/staleness_test.go,
// which does the same thing for the dialect through internal/extable. CI runs
// plain `go test ./...` and never `go generate`, so without this a CSV edit that
// was not regenerated, or a hand-edit of the generated file, would ship
// silently.
//
// Paths are relative to this directory, which is the working directory for
// `go test` (the precedent is internal/fakedx10/gen/main_test.go, which reads
// ../transcription-b.csv the same way). Nothing project-internal is imported,
// here or in the command itself — imports_test.go's recursive fence enforces
// that for this directory too.

const (
	csvPath = "../transcription-b.csv"
	genPath = "../exinventory_gen.go"
)

// generate is the whole pipeline over a byte slice: what main() does between
// reading the CSV and writing the file.
func generate(t *testing.T, data []byte, name string) []byte {
	t.Helper()
	rows, err := parseB(data)
	if err != nil {
		t.Fatalf("parseB: %v", err)
	}
	groups, err := groupRows(rows)
	if err != nil {
		t.Fatalf("groupRows: %v", err)
	}
	out, err := render(groups, name)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return out
}

// TestGeneratedInventory_NotStale re-derives the compact inventory from its ONE
// source — this package's copy of transcription B — and byte-compares the
// rendered file with the committed exinventory_gen.go.
//
// On failure: run `go generate ./internal/fakeft891` and commit the result. Do
// NOT hand-edit the generated file, and do not "fix" the CSV to match it: the
// CSV is a committed, hash-frozen evidential artefact (see PROVENANCE.md).
func TestGeneratedInventory_NotStale(t *testing.T) {
	data, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("reading %s: %v", csvPath, err)
	}
	want := generate(t, data, csvPath)

	got, err := os.ReadFile(genPath)
	if err != nil {
		t.Fatalf("reading %s: %v", genPath, err)
	}

	if !bytes.Equal(got, want) {
		t.Errorf("%s is stale relative to %s (committed %d bytes, regenerated %d bytes); run `go generate ./internal/fakeft891` and commit the result.\nFirst divergence: %s",
			genPath, csvPath, len(got), len(want), firstDiff(got, want))
	}
}

// TestRender_IsDeterministic renders the committed CSV twice, through two
// independent parses, and requires byte equality. The staleness test's byte
// comparison is only meaningful if the generator is deterministic: a table
// emitted from a map range, or comments aligned by anything other than
// go/format, would make it fail at random.
func TestRender_IsDeterministic(t *testing.T) {
	data, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("reading %s: %v", csvPath, err)
	}
	first := generate(t, data, csvPath)
	second := generate(t, data, csvPath)
	if !bytes.Equal(first, second) {
		t.Errorf("two renders of %s differ (%d vs %d bytes) — the generator is not deterministic.\nFirst divergence: %s",
			csvPath, len(first), len(second), firstDiff(first, second))
	}
	if len(first) == 0 {
		t.Fatal("render produced no bytes — this test would pass vacuously")
	}
}

// TestRender_EmbedsOnlyTheBaseName pins the property the two tests above depend
// on: the committed bytes must not record where the generator was invoked from,
// or this test package (which reads ../transcription-b.csv) could never render
// the file the //go:generate directive produces from transcription-b.csv.
func TestRender_EmbedsOnlyTheBaseName(t *testing.T) {
	data, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("reading %s: %v", csvPath, err)
	}
	out := string(generate(t, data, csvPath))
	if strings.Contains(out, "../") {
		t.Errorf("rendered output contains a relative path component %q — only the CSV's base name may be embedded", "../")
	}
	if base := filepath.Base(csvPath); !strings.Contains(out, base) {
		t.Errorf("rendered output does not mention %q anywhere — the generated-by marker has lost its source", base)
	}
}

// TestCommittedCSV_StructuralCounts is this package's own recount of the
// committed artefact, written as literals rather than derived from anything the
// generator emits: 159 items across 18 P1 groups, sized as the chart prints
// them.
//
// It is a TRUNCATION guard as much as a count: parseB and groupRows validate
// shape and contiguity but cannot notice that a whole trailing group is missing
// (the FT-710's own extable machinery learnt this — a jointly truncated source
// renders happily unless something checks the total). The independent binding of
// these numbers to the DIALECT's inventory is the transport cross-check's job;
// this is the local pin.
//
// THE GROUP KEY IS P1 ALONE, which is where this radio's schema parts company
// with the FTdx10's: the FT-891 chart prints a four-digit MENU Number whose two
// halves ARE the address (0803 is (P1,P2) = (08,03)) and whose P3 is always
// zero, so a group is one two-digit prefix and its items are indexed by P2.
// There is no third component to fold.
func TestCommittedCSV_StructuralCounts(t *testing.T) {
	data, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("reading %s: %v", csvPath, err)
	}
	rows, err := parseB(data)
	if err != nil {
		t.Fatalf("parseB: %v", err)
	}
	if len(rows) != 159 {
		t.Errorf("parsed %d data rows from %s, want 159", len(rows), csvPath)
	}
	groups, err := groupRows(rows)
	if err != nil {
		t.Fatalf("groupRows: %v", err)
	}
	if len(groups) != 18 {
		t.Errorf("parsed %d P1 groups, want 18", len(groups))
	}

	perP1 := map[string]int{}
	for _, g := range groups {
		perP1[g.p1] += len(g.widths)
	}
	want := map[string]int{
		"01": 3, "02": 7, "03": 2, "04": 11, "05": 20, "06": 7,
		"07": 13, "08": 12, "09": 6, "10": 11, "11": 9, "12": 4,
		"13": 2, "14": 7, "15": 18, "16": 23, "17": 1, "18": 3,
	}
	for p1, n := range want {
		if perP1[p1] != n {
			t.Errorf("P1=%s item count = %d, want %d", p1, perP1[p1], n)
		}
	}
	for p1 := range perP1 {
		if _, ok := want[p1]; !ok {
			t.Errorf("P1=%s appears in the artefact and not in this test's table — a group has been added", p1)
		}
	}
	// The chart's own EX block prints "P1 : 0101 - 1803 (MENU Number)", which
	// bounds it at exactly the first and last rows transcribed
	// (core/cat/ft891/testdata/transcription-b.md, "Source document"). So the
	// first and last groups are pinned by name as well as by size.
	if got := groups[0].p1; got != "01" {
		t.Errorf("first group is P1=%s, want 01 (the chart opens at 0101)", got)
	}
	if got := groups[len(groups)-1].p1; got != "18" {
		t.Errorf("last group is P1=%s, want 18 (the chart closes at 1803)", got)
	}
}

// TestParseB_TheOnlyFiveWideRowsAre0803And0804 is the '5' token's RED PROOF at
// the parse level, and it is the reason this generator's width alphabet runs to
// five where the FTdx10's stops at four.
//
// The FT-891 chart's Digits column runs 1..5, and the 5 comes from exactly two
// rows: 0803 OTHER DISP and 0804 OTHER SHIFT, whose signed
// "-3000 Hz - 0 - +3000 Hz" parameter counts its sign as a digit.
// core/cat/ft891/crosscheck_test.go pins the same two addresses from the A side,
// as literals, for the same reason: a pin computed from the thing it pins proves
// nothing. This is the B side's independent statement of it.
//
// WHICH rows are five wide matters as much as how many: a projection that put
// the '5' in the wrong place would answer five bytes for a one-byte item and one
// for a five-byte one, and a count alone would not notice. A generator that
// refused '5' outright — the FTdx10's alphabet, borrowed — fails this test
// rather than panicking at some later expansion, which is the whole point of
// proving the token here.
func TestParseB_TheOnlyFiveWideRowsAre0803And0804(t *testing.T) {
	data, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("reading %s: %v", csvPath, err)
	}
	rows, err := parseB(data)
	if err != nil {
		t.Fatalf("parseB: %v", err)
	}
	var five []string
	for _, r := range rows {
		if r.token < '1' || r.token > '5' {
			t.Errorf("row at line %d (%s%02d): token %q is outside '1'..'5', the whole alphabet this schema can express", r.line, r.p1, r.p2, r.token)
		}
		if r.token == '5' {
			five = append(five, fmt.Sprintf("%s%02d@%d", r.p1, r.p2, r.line))
		}
	}
	want := []string{"0803@67", "0804@68"}
	if len(five) != len(want) {
		t.Fatalf("rows with a '5' token = %v, want exactly %v", five, want)
	}
	for i := range want {
		if five[i] != want[i] {
			t.Errorf("five-wide row %d = %s, want %s", i, five[i], want[i])
		}
	}
}

// TestWidthToken_TheAlphabetIsExactlyOneToFive states the FT-891's projection
// alphabet directly, in both directions, and with it the STRUCTURAL fact that
// this schema has no text item and could not describe one.
//
// The FTdx10's generator decides textness from a Digits of 12 CONFIRMED by a P4
// cell beginning "Up to". The FT-891's transcription B has three columns —
// menu_number, name, digits — and no parameter-legend column at all, so there is
// no cell a text discriminator could read. Refusing 12 here is therefore not a
// claim that this radio has no twelve-byte menu field; it is the statement that
// THIS ARTEFACT cannot express one, and that a generator inventing a 'T' from
// the width alone would be answering twelve spaces where the radio may well
// answer twelve zeros. What would catch a genuine text row is the cross-check:
// the dialect's side is generated from transcription A, which HAS a text
// column, and core/transport/ex_crosscheck_ft891_test.go compares the shapes.
func TestWidthToken_TheAlphabetIsExactlyOneToFive(t *testing.T) {
	for n := 1; n <= 5; n++ {
		tok, err := widthToken(fmt.Sprint(n))
		if err != nil {
			t.Errorf("widthToken(%d): unexpected error: %v", n, err)
			continue
		}
		if want := byte('0' + n); tok != want {
			t.Errorf("widthToken(%d) = %q, want %q", n, tok, want)
		}
	}
	for _, n := range []int{0, 6, 9, 12} {
		if _, err := widthToken(fmt.Sprint(n)); err == nil {
			t.Errorf("widthToken(%d) returned no error; want a refusal — the compact inventory has no token for it", n)
		}
	}
}

// --- Negative coverage: every refusal the projection makes ---

// TestParseB_Refusals drives each malformed-input class through parseB over a
// minimal scratch CSV. These are the checks that make the generator refuse
// rather than emit a plausible wrong table, so each one is exercised: a
// validator nothing ever trips is a validator nobody knows works.
func TestParseB_Refusals(t *testing.T) {
	const header = "menu_number,name,digits\n"
	const goodRow = "0101,AGC FAST DELAY,4\n"

	tests := []struct {
		name    string
		csv     string
		wantErr string // substring
	}{
		{
			name:    "empty file",
			csv:     "",
			wantErr: "no header row",
		},
		{
			name:    "header only",
			csv:     header,
			wantErr: "no data rows",
		},
		{
			name:    "wrong header, right arity",
			csv:     "p1,p2,digits\n",
			wantErr: "header row is",
		},
		{
			// The FTdx10's own transcription B, header and first row. It must
			// not be readable here: its six columns would otherwise offer this
			// parser three plausible cells and a "menu number" that is a group
			// label.
			name:    "the sibling radio's six-column B",
			csv:     "P1,P2,P3,Function,P4,Digits\n01 (RADIO SETTING),01 (MODE SSB),01,AF TREBLE GAIN,-10 ~ +00 ~ +10,3\n",
			wantErr: "wrong number of fields",
		},
		{
			name:    "short record",
			csv:     header + "0101,AGC FAST DELAY\n",
			wantErr: "wrong number of fields",
		},
		{
			name:    "three-digit menu number",
			csv:     header + "101,AGC FAST DELAY,4\n",
			wantErr: "not exactly four ASCII digits",
		},
		{
			name:    "five-digit menu number — the siblings' six-digit address, truncated",
			csv:     header + "01010,AGC FAST DELAY,4\n",
			wantErr: "not exactly four ASCII digits",
		},
		{
			name:    "non-digit in the menu number",
			csv:     header + "01O1,AGC FAST DELAY,4\n",
			wantErr: "not exactly four ASCII digits",
		},
		{
			name:    "empty name cell",
			csv:     header + "0101,,4\n",
			wantErr: "empty name",
		},
		{
			name:    "non-numeric Digits",
			csv:     header + "0101,AGC FAST DELAY,four\n",
			wantErr: "is not a number",
		},
		{
			name:    "Digits 6 — one past this chart's widest",
			csv:     header + "0101,AGC FAST DELAY,6\n",
			wantErr: "no token for it",
		},
		{
			name:    "Digits 0",
			csv:     header + "0101,AGC FAST DELAY,0\n",
			wantErr: "no token for it",
		},
		{
			name:    "Digits 12 — the FTdx10's text width, which this schema cannot describe",
			csv:     header + "0101,MY CALL.,12\n",
			wantErr: "no token for it",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseB([]byte(tt.csv))
			if err == nil {
				t.Fatalf("parseB accepted the input; want an error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("parseB error = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}

	// Sanity: the same header and a well-formed row must PARSE, or every
	// refusal above could be passing for the wrong reason.
	rows, err := parseB([]byte(header + goodRow))
	if err != nil {
		t.Fatalf("parseB rejected a well-formed row: %v", err)
	}
	if len(rows) != 1 || rows[0].p1 != "01" || rows[0].p2 != 1 || rows[0].token != '4' {
		t.Fatalf("parseB(well-formed) = %+v, want one row (p1 \"01\", p2 1, token '4')", rows)
	}
}

// TestGroupRows_Refusals covers the three structural properties the compact
// widths string depends on. Each is a refusal because a repair would be a guess:
// the string's index IS the P2 item index, so a gap silently renumbers every
// item after it, and an interleaved group cannot be expressed at all.
func TestGroupRows_Refusals(t *testing.T) {
	r4 := func(p1 string, p2 int) row {
		return row{p1: p1, p2: p2, token: '4', line: p2 + 1}
	}
	tests := []struct {
		name    string
		rows    []row
		wantErr string
	}{
		{
			name:    "group does not open at P2=01",
			rows:    []row{r4("01", 2)},
			wantErr: "opens at P2=02",
		},
		{
			name:    "gap in P2",
			rows:    []row{r4("01", 1), r4("01", 3)},
			wantErr: "must run consecutively",
		},
		{
			name:    "P2 repeats",
			rows:    []row{r4("01", 1), r4("01", 1)},
			wantErr: "must run consecutively",
		},
		{
			name:    "group resumes after another intervened",
			rows:    []row{r4("01", 1), r4("02", 1), r4("01", 2)},
			wantErr: "not one contiguous block",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := groupRows(tt.rows)
			if err == nil {
				t.Fatalf("groupRows accepted the input; want an error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("groupRows error = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}

	// Sanity: two well-formed consecutive groups must fold, with the widths in
	// P2 order.
	groups, err := groupRows([]row{r4("01", 1), r4("01", 2), r4("02", 1)})
	if err != nil {
		t.Fatalf("groupRows rejected well-formed rows: %v", err)
	}
	if len(groups) != 2 || groups[0].widths != "44" || groups[1].widths != "4" {
		t.Fatalf("groupRows(well-formed) = %+v, want two groups with widths \"44\" and \"4\"", groups)
	}
}

// TestRender_RefusesAnEmptyInventory pins the last refusal: rendering nothing
// would emit a syntactically valid file declaring an empty table, and a fake
// with an empty EX inventory answers "?;" to every menu read — a silent,
// plausible-looking regression rather than a failure.
func TestRender_RefusesAnEmptyInventory(t *testing.T) {
	if _, err := render(nil, csvPath); err == nil {
		t.Error("render(nil) returned no error; want a refusal")
	}
}

// firstDiff describes where two byte slices first differ, quoting the
// surrounding line so a staleness failure names the ROW rather than an offset —
// which is what makes the failure message the input to a decision (regenerate?
// or arbitrate the CSV?) rather than a puzzle.
func firstDiff(got, want []byte) string {
	n := min(len(got), len(want))
	for i := 0; i < n; i++ {
		if got[i] != want[i] {
			return fmt.Sprintf("at byte %d:\n  committed:    %q\n  regenerated:  %q", i, lineAt(got, i), lineAt(want, i))
		}
	}
	if len(got) != len(want) {
		return fmt.Sprintf("one is a prefix of the other (committed %d bytes, regenerated %d bytes)", len(got), len(want))
	}
	return "no difference"
}

// lineAt returns the whole line containing offset i.
func lineAt(b []byte, i int) string {
	start := bytes.LastIndexByte(b[:i], '\n') + 1
	end := bytes.IndexByte(b[i:], '\n')
	if end < 0 {
		end = len(b)
	} else {
		end += i
	}
	return string(b[start:end])
}
