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

// This file is the CI guard for fakeft991a's EX generator, and it lives HERE
// rather than in package fakeft991a for one structural reason: the projection
// logic is in `package main`, which no Go package can import. A staleness test
// in fakeft991a would have to re-implement the projection to compare against
// it, and a check written against a second implementation of the thing it is
// checking is a weaker check than running the real one.
//
// So the test runs the ACTUAL generator's parse and render over the ACTUAL
// committed CSV and byte-compares the result with the committed
// exinventory_gen.go — the discipline of core/cat/ft991a/staleness_test.go,
// which does the same thing for the dialect through internal/extable. CI runs
// plain `go test ./...` and never `go generate`, so without this a CSV edit
// that was not regenerated, or a hand-edit of the generated file, would ship
// silently.
//
// Paths are relative to this directory, which is the working directory for
// `go test`. Nothing project-internal is imported, here or in the command
// itself — imports_test.go's recursive fence enforces that for this directory
// too, and TestNoCoreImports_ReachesTheGenerator asserts that the real scan
// really reaches main.go.

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
	if err := checkRun(rows); err != nil {
		t.Fatalf("checkRun: %v", err)
	}
	out, err := render(rows, name)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return out
}

// readCSV reads the committed artefact, which every test below starts from.
func readCSV(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("reading %s: %v", csvPath, err)
	}
	return data
}

// TestGeneratedInventory_NotStale re-derives the inventory from its ONE source
// — this package's copy of transcription B — and byte-compares the rendered
// file with the committed exinventory_gen.go.
//
// On failure: run `go generate ./internal/fakeft991a` and commit the result. Do
// NOT hand-edit the generated file, and do not "fix" the CSV to match it: the
// CSV is a committed, hash-frozen evidential artefact (see PROVENANCE.md).
func TestGeneratedInventory_NotStale(t *testing.T) {
	want := generate(t, readCSV(t), csvPath)

	got, err := os.ReadFile(genPath)
	if err != nil {
		t.Fatalf("reading %s: %v", genPath, err)
	}

	if !bytes.Equal(got, want) {
		t.Errorf("%s is stale relative to %s (committed %d bytes, regenerated %d bytes); run `go generate ./internal/fakeft991a` and commit the result.\nFirst divergence: %s",
			genPath, csvPath, len(got), len(want), firstDiff(got, want))
	}
}

// TestRender_IsDeterministic renders the committed CSV twice, through two
// independent parses, and requires byte equality. The staleness test's byte
// comparison is only meaningful if the generator is deterministic: a table
// emitted from a map range, or comments aligned by anything other than
// go/format, would make it fail at random.
func TestRender_IsDeterministic(t *testing.T) {
	data := readCSV(t)
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
	out := string(generate(t, readCSV(t), csvPath))
	if strings.Contains(out, "../") {
		t.Errorf("rendered output contains a relative path component %q — only the CSV's base name may be embedded", "../")
	}
	if base := filepath.Base(csvPath); !strings.Contains(out, base) {
		t.Errorf("rendered output does not mention %q anywhere — the generated-by marker has lost its source", base)
	}
}

// TestCommittedCSV_StructuralCounts is this package's own recount of the
// committed artefact, written as literals rather than derived from anything the
// generator emits: 153 chart rows, of which 152 are projected and ONE — 087 — is
// excluded.
//
// It is a TRUNCATION guard as much as a count: parseB and checkRun validate
// shape and consecutiveness but cannot notice that a whole trailing stretch of
// the chart is missing (the FT-710's own extable machinery learnt this — a
// jointly truncated source renders happily unless something checks the total).
// The independent binding of these numbers to the DIALECT's inventory is the
// transport cross-check's job; this is the local pin.
//
// THE ADDRESS IS THE WHOLE MENU NUMBER, which is where this chart's schema
// parts company with the FT-891's: there is no group key to fold, so the
// structural statement is a run of numbers rather than a table of group sizes.
func TestCommittedCSV_StructuralCounts(t *testing.T) {
	rows, err := parseB(readCSV(t))
	if err != nil {
		t.Fatalf("parseB: %v", err)
	}
	if len(rows) != 153 {
		t.Errorf("parsed %d data rows from %s, want 153", len(rows), csvPath)
	}
	if err := checkRun(rows); err != nil {
		t.Fatalf("checkRun over the committed artefact: %v", err)
	}
	// The chart's own EX block prints "P1 : 001 - 153 (MENU Number)", which
	// bounds it at exactly the first and last rows transcribed
	// (core/cat/ft991a/testdata/transcription-b.md, "Pages used"). So the ends
	// are pinned by name as well as by count.
	if got := rows[0].addr; got != "001" {
		t.Errorf("first row is %s, want 001", got)
	}
	if got := rows[len(rows)-1].addr; got != "153" {
		t.Errorf("last row is %s, want 153", got)
	}

	var excluded []string
	for _, r := range rows {
		if r.excluded {
			excluded = append(excluded, r.addr)
		}
	}
	if want := []string{"087"}; len(excluded) != 1 || excluded[0] != want[0] {
		t.Errorf("excluded rows = %v, want %v — 087 RADIO ID is the only row this chart prints with no parameter", excluded, want)
	}
	if got, want := len(rows)-len(excluded), 152; got != want {
		t.Errorf("%d rows would be projected, want %d", got, want)
	}
}

// TestParseB_TheOnlyEightWideRowIs151 is the '8' token's RED PROOF at the parse
// level, and it is the reason this generator's width alphabet runs to eight
// where the FT-891's stops at five and the FTdx10's at four.
//
// core/cat/ft991a/crosscheck_test.go pins the same address and the same width
// from the A side, as literals (widestRowAddr, widestRowDigits), for the same
// reason: a pin computed from the thing it pins proves nothing. This is the B
// side's independent statement of it.
//
// WHICH row is eight wide matters as much as how many: a projection that put
// the '8' in the wrong place would answer eight bytes for a one-byte item and
// one for an eight-byte one, and a count alone would not notice. A generator
// that refused '8' outright — the FT-891's alphabet, borrowed — fails this test
// rather than panicking at some later expansion, which is the whole point of
// proving the token here.
//
// The four five-wide rows are pinned in the same sweep, because the FT-891's
// alphabet would admit those and stop only at the 8: without them a reader
// could not tell whether this chart's width column exceeds 4 in one place or in
// five.
func TestParseB_TheOnlyEightWideRowIs151(t *testing.T) {
	rows, err := parseB(readCSV(t))
	if err != nil {
		t.Fatalf("parseB: %v", err)
	}
	var eight, five []string
	for _, r := range rows {
		if r.excluded {
			continue
		}
		if r.token < '1' || r.token > '8' {
			t.Errorf("row at line %d (%s): token %q is outside '1'..'8', the whole alphabet this chart declares", r.line, r.addr, r.token)
		}
		switch r.token {
		case '8':
			eight = append(eight, fmt.Sprintf("%s@%d", r.addr, r.line))
		case '5':
			five = append(five, fmt.Sprintf("%s@%d", r.addr, r.line))
		}
	}
	// 151 PRESET FREQUENCY, "00030000 ~ 47000000" — eight digits.
	if want := []string{"151@152"}; !equalStrings(eight, want) {
		t.Errorf("rows with an '8' token = %v, want exactly %v", eight, want)
	}
	// 027 TIME ZONE, 064 OTHER DISP (SSB), 065 OTHER SHIFT (SSB),
	// 083 RPT SHIFT 430MHz — transcription-b.md §1's table, minus the 8.
	if want := []string{"027@28", "064@65", "065@66", "083@84"}; !equalStrings(five, want) {
		t.Errorf("rows with a '5' token = %v, want exactly %v", five, want)
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// TestWidthToken_TheAlphabetIsExactlyOneToEight states this chart's projection
// alphabet directly, in both directions, and with it the STRUCTURAL fact that
// this schema has no text item and could not describe one.
//
// The FTdx10's generator decides textness from a Digits of 12 CONFIRMED by a P4
// cell beginning "Up to". This transcription B has three columns —
// menu_number, name, digits — and no parameter-legend column at all, so there
// is no cell a text discriminator could read. Refusing 12 here is therefore not
// a claim that this radio has no twelve-byte menu field; it is the statement
// that THIS ARTEFACT cannot express one. What would catch a genuine text row is
// the cross-check: the dialect's side is generated from transcription A, which
// HAS a text column, and core/transport/ex_crosscheck_ft991a_test.go compares
// the shapes.
//
// NOTE THE 9 AND THE 12 REFUSALS TOGETHER: 9 is one past this chart's widest,
// and 12 is the FTdx10's text width. Neither is expressible here, and the same
// refusal covers both — which is why the alphabet's ceiling is stated as a
// reading of this chart rather than as "the family's widest so far".
func TestWidthToken_TheAlphabetIsExactlyOneToEight(t *testing.T) {
	for n := 1; n <= 8; n++ {
		tok, err := widthToken(fmt.Sprint(n))
		if err != nil {
			t.Errorf("widthToken(%d): unexpected error: %v", n, err)
			continue
		}
		if want := byte('0' + n); tok != want {
			t.Errorf("widthToken(%d) = %q, want %q", n, tok, want)
		}
	}
	for _, n := range []int{0, 9, 12} {
		if _, err := widthToken(fmt.Sprint(n)); err == nil {
			t.Errorf("widthToken(%d) returned no error; want a refusal — the inventory has no token for it", n)
		}
	}
}

// TestWidthToken_RefusesByExactShape pins the same rule parseMenuNumber
// already states for the address cell onto the digits cell: a cell is
// admitted only by its EXACT SHAPE (one ASCII digit), never by what
// strconv.Atoi happens to parse. "+4" and "04" are both syntactically valid
// input to Atoi and both denote 4, but neither is the one-byte cell this
// chart's B ever prints, so both must be refused rather than silently read
// as width 4.
func TestWidthToken_RefusesByExactShape(t *testing.T) {
	for _, s := range []string{"+4", "04", "-4", "44", "", "a"} {
		if _, err := widthToken(s); err == nil {
			t.Errorf("widthToken(%q) returned no error; want a refusal — not exactly one ASCII digit", s)
		}
	}
}

// --- The parameterless token: a red proof each way (plan P18) ---

// TestParseB_TheParameterlessTokenIsExcludedOnItsOwnAddressAndRefusedElsewhere
// is the P18 pin, in both directions, on scratch CSVs rather than on the
// committed artefact — because only one of the two directions is expressible in
// an artefact that must not be edited.
//
// The token is '?' HERE and '-' in internal/extable, and both are right: one
// glyph is printed on the page and the two transcriptions spell it under
// different conventions. gen/main.go's parameterlessToken states the ruling in
// full; this test states its two consequences.
//
//   - ON 087, a '?' is EXCLUDED: parsed, counted in the chart's run, and left
//     out of the projection. So the fake's inventory is 152 for a 153-row
//     chart, exactly as the dialect's is.
//   - ON ANY OTHER ADDRESS, a '?' is REFUSED. It is not silently excluded (which
//     would shrink the inventory by a row nobody declared) and not read as a
//     width (there is none to read). Either the chart prints a row this
//     generator does not know is parameterless, or the transcription has
//     drifted; both are findings, and the arbitration is against the PDF.
//
// The third direction is here too, and it is the one neither the plan nor the
// FT-891 has a counterpart for: 087 with a NUMERIC cell is also refused. A
// declaration that no longer describes its artefact must fail loudly, or a
// later correction of the chart would silently drop a real address from the
// inventory.
func TestParseB_TheParameterlessTokenIsExcludedOnItsOwnAddressAndRefusedElsewhere(t *testing.T) {
	const header = "menu_number,name,digits\n"

	t.Run("? on 087 is excluded, not refused and not projected", func(t *testing.T) {
		rows, err := parseB([]byte(header + "086,GM DISPLAY MODE,1\n087,RADIO ID,?\n088,GM DISPLY,1\n"))
		if err != nil {
			t.Fatalf("parseB refused the declared parameterless row: %v", err)
		}
		if len(rows) != 3 {
			t.Fatalf("parsed %d rows, want 3 — the parameterless row is COUNTED, not skipped", len(rows))
		}
		if !rows[1].excluded {
			t.Errorf("row 087 is not marked excluded: %+v", rows[1])
		}
		if rows[0].excluded || rows[2].excluded {
			t.Errorf("a row other than 087 was marked excluded: %+v", rows)
		}
		// Counted in the run is the point of not skipping it: 086, 087, 088
		// are consecutive here, and dropping 087 would make them look like a
		// gap to checkRun. (checkRun itself is not called on this fragment —
		// it requires the whole chart, which opens at 001.)
		for i, want := range []int{86, 87, 88} {
			if rows[i].num != want {
				t.Errorf("row %d has num %d, want %d — the excluded row must hold its place in the chart's run", i, rows[i].num, want)
			}
		}
	})

	t.Run("? on any other address is refused", func(t *testing.T) {
		_, err := parseB([]byte(header + "001,AGC FAST DELAY,?\n"))
		if err == nil {
			t.Fatal("parseB accepted a '?' on 001; want a refusal")
		}
		for _, want := range []string{"means NO PARAMETER", "001"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("parseB error = %q, want it to contain %q", err, want)
			}
		}
	})

	t.Run("a declared parameterless address with a numeric cell is refused", func(t *testing.T) {
		_, err := parseB([]byte(header + "087,RADIO ID,4\n"))
		if err == nil {
			t.Fatal("parseB accepted a numeric cell on 087; want a refusal")
		}
		if want := "declared parameterless"; !strings.Contains(err.Error(), want) {
			t.Errorf("parseB error = %q, want it to contain %q", err, want)
		}
	})
}

// --- Negative coverage: every other refusal the projection makes ---

// TestParseB_Refusals drives each malformed-input class through parseB over a
// minimal scratch CSV. These are the checks that make the generator refuse
// rather than emit a plausible wrong table, so each one is exercised: a
// validator nothing ever trips is a validator nobody knows works.
func TestParseB_Refusals(t *testing.T) {
	const header = "menu_number,name,digits\n"
	const goodRow = "001,AGC FAST DELAY,4\n"

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
			csv:     "p1,name,digits\n",
			wantErr: "header row is",
		},
		{
			// The FTdx10's own transcription B, header and first row. Its six
			// columns would otherwise offer this parser three plausible cells
			// and a "menu number" that is a group label.
			name:    "the FTdx10's six-column B",
			csv:     "P1,P2,P3,Function,P4,Digits\n01 (RADIO SETTING),01 (MODE SSB),01,AF TREBLE GAIN,-10 ~ +00 ~ +10,3\n",
			wantErr: "wrong number of fields",
		},
		{
			// THE FT-891'S B HAS THIS FILE'S EXACT HEADER — the two radios'
			// transcriptions were delivered under one brief — so the header
			// check cannot separate them and the ADDRESS WIDTH must.
			name:    "the FT-891's identically-headed B",
			csv:     header + "0101,AGC FAST DELAY,4\n",
			wantErr: "not exactly three ASCII digits",
		},
		{
			name:    "short record",
			csv:     header + "001,AGC FAST DELAY\n",
			wantErr: "wrong number of fields",
		},
		{
			name:    "two-digit menu number — a lost leading zero",
			csv:     header + "01,AGC FAST DELAY,4\n",
			wantErr: "not exactly three ASCII digits",
		},
		{
			name:    "non-digit in the menu number",
			csv:     header + "O01,AGC FAST DELAY,4\n",
			wantErr: "not exactly three ASCII digits",
		},
		{
			name:    "empty name cell",
			csv:     header + "001,,4\n",
			wantErr: "empty name",
		},
		{
			name:    "non-numeric Digits",
			csv:     header + "001,AGC FAST DELAY,four\n",
			wantErr: "not exactly one ASCII digit",
		},
		{
			name:    "Digits 9 — one past this chart's widest",
			csv:     header + "001,AGC FAST DELAY,9\n",
			wantErr: "no token for it",
		},
		{
			name:    "Digits 0",
			csv:     header + "001,AGC FAST DELAY,0\n",
			wantErr: "no token for it",
		},
		{
			name:    "Digits 12 — the FTdx10's text width, which this schema cannot describe",
			csv:     header + "001,MY CALL.,12\n",
			wantErr: "not exactly one ASCII digit",
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
	if len(rows) != 1 || rows[0].addr != "001" || rows[0].num != 1 || rows[0].token != '4' {
		t.Fatalf("parseB(well-formed) = %+v, want one row (addr \"001\", num 1, token '4')", rows)
	}
}

// TestCheckRun_Refusals covers the structural property this flat chart has in
// place of the FT-891's group rules: one consecutive run of menu numbers from
// 001. Each is a refusal because a repair would be a guess — a gap in a
// transcription of a ruled chart is either a transcription defect or a chart
// this projection cannot model, and both are findings.
func TestCheckRun_Refusals(t *testing.T) {
	at := func(n int) row {
		return row{addr: fmt.Sprintf("%03d", n), num: n, token: '4', line: n + 1}
	}
	tests := []struct {
		name    string
		rows    []row
		wantErr string
	}{
		{
			name:    "no rows",
			rows:    nil,
			wantErr: "no rows",
		},
		{
			name:    "does not open at 001",
			rows:    []row{at(2), at(3)},
			wantErr: "the chart opens at 002",
		},
		{
			name:    "a gap",
			rows:    []row{at(1), at(3)},
			wantErr: "002 is missing or repeated",
		},
		{
			name:    "a repeat",
			rows:    []row{at(1), at(1)},
			wantErr: "002 is missing or repeated",
		},
		{
			name:    "out of order",
			rows:    []row{at(1), at(3), at(2)},
			wantErr: "002 is missing or repeated",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkRun(tt.rows)
			if err == nil {
				t.Fatalf("checkRun accepted the input; want an error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("checkRun error = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}

	// Sanity: a well-formed run must pass, or every refusal above could be
	// passing for the wrong reason.
	if err := checkRun([]row{at(1), at(2), at(3)}); err != nil {
		t.Fatalf("checkRun rejected a well-formed run: %v", err)
	}
}

// TestRender_RefusesAnEmptyInventory pins the last refusal: rendering nothing
// would emit a syntactically valid file declaring an empty table, and a fake
// with an empty EX inventory answers "?;" to every menu read — a silent,
// plausible-looking regression rather than a failure.
//
// The second case is the one this radio makes possible and the FT-891 does not:
// a chart whose every row is EXCLUDED renders no entries either, and must be
// refused by the same rule rather than by a count of parsed rows.
func TestRender_RefusesAnEmptyInventory(t *testing.T) {
	if _, err := render(nil, csvPath); err == nil {
		t.Error("render(nil) returned no error; want a refusal")
	}
	excludedOnly := []row{{addr: "087", num: 87, excluded: true, line: 88}}
	if _, err := render(excludedOnly, csvPath); err == nil {
		t.Error("render(only excluded rows) returned no error; want a refusal")
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
