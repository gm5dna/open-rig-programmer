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

// This file is the CI guard for fakedx10's EX generator, and it lives HERE
// rather than in package fakedx10 for one structural reason: the projection
// logic is in `package main`, which no Go package can import. A staleness test
// in fakedx10 would have to re-implement the projection to compare against it,
// and a check written against a second implementation of the thing it is
// checking is a weaker check than running the real one.
//
// So the test runs the ACTUAL generator's parse and render over the ACTUAL
// committed CSV and byte-compares the result with the committed
// exinventory_gen.go — the discipline of core/cat/ftdx10/staleness_test.go,
// which does the same thing for the dialect through internal/extable. CI runs
// plain `go test ./...` and never `go generate`, so without this a CSV edit that
// was not regenerated, or a hand-edit of the generated file, would ship
// silently.
//
// Paths are relative to this directory, which is the working directory for
// `go test` (the precedent is internal/extable/observe/main_test.go, which reads
// ../../../core/cat/table2.csv the same way). Nothing project-internal is
// imported, here or in the command itself.

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
// On failure: run `go generate ./internal/fakedx10` and commit the result. Do
// NOT hand-edit the generated file, and do not "fix" the CSV to match it: the
// CSV is a committed evidential artefact (see PROVENANCE.md).
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
		t.Errorf("%s is stale relative to %s (committed %d bytes, regenerated %d bytes); run `go generate ./internal/fakedx10` and commit the result.\nFirst divergence: %s",
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
// generator emits: 197 items across 18 (P1,P2) subgroups, exactly one of them a
// text item, none at P1=05 or P1=06.
//
// It is a TRUNCATION guard as much as a count: parseB and groupRows validate
// shape and contiguity but cannot notice that a whole trailing group is missing
// (the FT-710's own extable machinery learnt this — a jointly truncated source
// renders happily unless something checks the total). The independent binding of
// these numbers to the DIALECT's inventory is the transport cross-check's job;
// this is the local pin.
//
// The FTdx10's chart, unlike the FT-710's, populates P1 01-04 only: there is no
// P1=05 group and no EXTENSION SETTING group at all (core/cat/ftdx10/doc.go
// records that anomaly UNRESOLVED, since unlike the FT-710's it cannot be put to
// hardware).
func TestCommittedCSV_StructuralCounts(t *testing.T) {
	data, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("reading %s: %v", csvPath, err)
	}
	rows, err := parseB(data)
	if err != nil {
		t.Fatalf("parseB: %v", err)
	}
	if len(rows) != 197 {
		t.Errorf("parsed %d data rows from %s, want 197", len(rows), csvPath)
	}
	groups, err := groupRows(rows)
	if err != nil {
		t.Fatalf("groupRows: %v", err)
	}
	if len(groups) != 18 {
		t.Errorf("parsed %d (P1,P2) subgroups, want 18", len(groups))
	}

	perP1 := map[string]int{}
	textItems := 0
	for _, g := range groups {
		perP1[g.p1] += len(g.widths)
		for _, w := range g.widths {
			if w == 'T' {
				textItems++
			}
		}
	}
	want := map[string]int{"01": 99, "02": 30, "03": 57, "04": 11}
	for p1, n := range want {
		if perP1[p1] != n {
			t.Errorf("P1=%s item count = %d, want %d", p1, perP1[p1], n)
		}
	}
	for _, absent := range []string{"05", "06"} {
		if n, ok := perP1[absent]; ok {
			t.Errorf("P1=%s item count = %d, want the group to be ABSENT (the FTdx10 chart populates P1 01-04 only)", absent, n)
		}
	}
	if textItems != 1 {
		t.Errorf("text items = %d, want 1 (MY CALL. at 040101 — the FTdx10 chart's only one, where the FT-710's has six)", textItems)
	}
}

// TestParseB_TheTextItemIsTheOneAt040101 pins WHICH row is the text item, not
// merely how many there are: a projection that put the 'T' in the wrong place
// would answer 12 spaces for a numeric item and zeros for the call sign, and the
// count test above would not notice.
func TestParseB_TheTextItemIsTheOneAt040101(t *testing.T) {
	data, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("reading %s: %v", csvPath, err)
	}
	rows, err := parseB(data)
	if err != nil {
		t.Fatalf("parseB: %v", err)
	}
	for _, r := range rows {
		isText := r.token == 'T'
		wantText := r.p1 == "04" && r.p2 == "01" && r.p3 == 1
		if isText != wantText {
			t.Errorf("row at line %d (%s,%s,%02d): token %q, text=%v, want text=%v", r.line, r.p1, r.p2, r.p3, r.token, isText, wantText)
		}
	}
}

// --- Negative coverage: every refusal the projection makes ---

// TestParseB_Refusals drives each malformed-input class through parseB and
// groupRows over a minimal scratch CSV. These are the checks that make the
// generator refuse rather than emit a plausible wrong table, so each one is
// exercised: a validator nothing ever trips is a validator nobody knows works.
func TestParseB_Refusals(t *testing.T) {
	const header = "P1,P2,P3,Function,P4,Digits\n"
	const goodRow = "01 (RADIO SETTING),01 (MODE SSB),01,AF TREBLE GAIN,-10 ~ +00 ~ +10,3\n"

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
			name:    "wrong header",
			csv:     "p1,p2,p3,p1_label,p2_label,name\n",
			wantErr: "header row is",
		},
		{
			name:    "short record",
			csv:     header + "01 (RADIO SETTING),01 (MODE SSB),01,AF TREBLE GAIN,3\n",
			wantErr: "wrong number of fields",
		},
		{
			name:    "unwrapped P1 cell",
			csv:     header + "01,01 (MODE SSB),01,AF TREBLE GAIN,-10 ~ +10,3\n",
			wantErr: "not of the form",
		},
		{
			name:    "P1 cell with an empty label",
			csv:     header + "01 (),01 (MODE SSB),01,AF TREBLE GAIN,-10 ~ +10,3\n",
			wantErr: "empty label",
		},
		{
			name:    "one-digit P2 component",
			csv:     header + "01 (RADIO SETTING),1 (MODE SSB),01,AF TREBLE GAIN,-10 ~ +10,3\n",
			wantErr: "not exactly two ASCII digits",
		},
		{
			name:    "one-digit P3",
			csv:     header + "01 (RADIO SETTING),01 (MODE SSB),1,AF TREBLE GAIN,-10 ~ +10,3\n",
			wantErr: "not exactly two ASCII digits",
		},
		{
			name:    "non-numeric Digits",
			csv:     header + "01 (RADIO SETTING),01 (MODE SSB),01,AF TREBLE GAIN,-10 ~ +10,three\n",
			wantErr: "is not a number",
		},
		{
			name:    "Digits 5 — no token for it",
			csv:     header + "01 (RADIO SETTING),01 (MODE SSB),01,AF TREBLE GAIN,-10 ~ +10,5\n",
			wantErr: "no token for it",
		},
		{
			name:    "Digits 0",
			csv:     header + "01 (RADIO SETTING),01 (MODE SSB),01,AF TREBLE GAIN,-10 ~ +10,0\n",
			wantErr: "no token for it",
		},
		{
			name:    "Digits 12 without the text discriminator",
			csv:     header + "01 (RADIO SETTING),01 (MODE SSB),01,SOME NUMBER,000000000000 ~ 999999999999,12\n",
			wantErr: "text discriminator disagree",
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
	if len(rows) != 1 || rows[0].token != '3' {
		t.Fatalf("parseB(well-formed) = %+v, want one row with token '3'", rows)
	}
}

// TestGroupRows_Refusals covers the three structural properties the compact
// widths string depends on. Each is a refusal because a repair would be a guess:
// the string's index IS the P3 item index, so a gap silently renumbers every
// item after it, and an interleaved group cannot be expressed at all.
func TestGroupRows_Refusals(t *testing.T) {
	row3 := func(p1, p2 string, p3 int, p1Label, p2Label string) row {
		return row{p1: p1, p2: p2, p1Label: p1Label, p2Label: p2Label, p3: p3, token: '3', line: p3 + 1}
	}
	tests := []struct {
		name    string
		rows    []row
		wantErr string
	}{
		{
			name:    "group does not open at P3=01",
			rows:    []row{row3("01", "01", 2, "A", "B")},
			wantErr: "opens at P3=02",
		},
		{
			name:    "gap in P3",
			rows:    []row{row3("01", "01", 1, "A", "B"), row3("01", "01", 3, "A", "B")},
			wantErr: "must run consecutively",
		},
		{
			name:    "P3 repeats",
			rows:    []row{row3("01", "01", 1, "A", "B"), row3("01", "01", 1, "A", "B")},
			wantErr: "must run consecutively",
		},
		{
			name: "group resumes after another intervened",
			rows: []row{
				row3("01", "01", 1, "A", "B"),
				row3("01", "02", 1, "A", "C"),
				row3("01", "01", 2, "A", "B"),
			},
			wantErr: "not one contiguous block",
		},
		{
			name: "label cells disagree inside a group",
			rows: []row{
				row3("01", "01", 1, "A", "B"),
				row3("01", "01", 2, "A", "DIFFERENT"),
			},
			wantErr: "label cells disagree",
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
	// P3 order.
	groups, err := groupRows([]row{
		row3("01", "01", 1, "A", "B"),
		row3("01", "01", 2, "A", "B"),
		row3("01", "02", 1, "A", "C"),
	})
	if err != nil {
		t.Fatalf("groupRows rejected well-formed rows: %v", err)
	}
	if len(groups) != 2 || groups[0].widths != "33" || groups[1].widths != "3" {
		t.Fatalf("groupRows(well-formed) = %+v, want two groups with widths \"33\" and \"3\"", groups)
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
