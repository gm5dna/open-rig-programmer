// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx10_test

import (
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/internal/extable"
)

// This file is M9c-4 task 5: the cross-check that binds the FTdx10's three
// independently derived records of Table 2 "MENU Chart" to one another.
//
// The three artefacts, and why there are three:
//
//   - TRANSCRIPTION A — table2.csv, this package's ONLY generation source
//     (task 3). Layout-text-led, PDF-checked, in the ten-column extable
//     shape, parsed here through extable.ParseCSV under the REGISTERED
//     "ftdx10" profile so that the cross-check reads the same bytes through
//     the same parser the generator does.
//   - TRANSCRIPTION B — testdata/transcription-b.csv (task 4). Derived
//     PDF-primary by a quarantined agent that never opened this repository,
//     never saw A or the ledger, and was told no row count and no address.
//   - THE GROUP-BOUNDARY LEDGER — testdata/group-ledger.csv (task 2).
//     Derived from the rendered PDF's ruled cells by a second quarantined
//     agent, BEFORE either transcription existed, and the source of the
//     profile's ExpectedRows.
//
// Agreement between three blind derivations is the evidence; this test is
// where that agreement is made mechanical rather than asserted in prose. ANY
// mismatch is a STOP for orchestrator arbitration AGAINST THE PDF, which may
// correct A, B, or the ledger — never this test, and never an artefact
// edited merely to make the test pass. That is also why every failure below
// prints the exact address or group and both sides' values: the failure
// output is the arbitration's input.
//
// # Adjudication (a): transcription B's delivered schema, and the adapter
//
// B's brief specified `p1,p2,p3,p1_label,p2_label,name,digits,text`. A
// mid-task stall/resume lost that header from the quarantined agent's
// context, and the delivered file carries
// `P1,P2,P3,Function,P4,Digits` instead: FULL printed group labels in P1/P2
// ("01 (RADIO SETTING)", "01 (MODE SSB)"), a bonus verbatim P4 column, and
// no text flag. The orchestrator accepted it VERBATIM — evidence integrity
// over format compliance — so the adaptation lives HERE, in the test, and
// the committed artefact is never rewritten. The adapter is:
//
//	p1, p2   the leading two digits of the P1 and P2 label cells
//	labels   the text inside those cells' parentheses
//	name     the Function column, verbatim
//	digits   the Digits column
//	text     digits == 12 AND P4 begins "Up to" (the chart's one text row
//	         is the only row whose P4 describes a character count rather
//	         than a value legend)
//
// # Adjudication (b): label normalisation, and why it is not a fudge
//
// The chart prints its group labels parenthesised and numbered — "01 (MODE
// SSB)". Transcription A records the BARE label, stripping that wrapper,
// because the enclosing "NN (...)" is the chart's typographic bracketing of
// the label rather than part of it — the convention the FT-710
// transcription established and A's own header documents. B and the ledger
// record the wrapper verbatim. Neither reading is wrong; they are different
// answers to a question about typography, not about the radio.
//
// The cross-check therefore normalises B's and the ledger's labels to the
// bare form before comparing. This was PRE-REGISTERED at task 3 (A's header
// states the stripping convention, written before B or this test existed),
// so it is not a difference discovered and then defined away. The verbatim
// evidence survives untouched in B and the ledger; only the SEMANTICS are
// bound here. splitWrapped refuses anything that is not exactly "NN
// (LABEL)", so a genuinely different label cannot be normalised into
// agreement by accident.
//
// # Adjudication (c): P4 is deliberately NOT bound
//
// The P4 parameter-description column is single-sourced in A per the spec —
// it is an audit trail and is not emitted into the generated Go — and B's
// copy is extra audit, not a second binding source. The two also record
// different glyph conventions for the same printed characters (A's
// extraction renders negative signs as U+2013 EN DASH, B's renders them as
// ASCII hyphen; each artefact's header documents its own), so comparing
// them would be comparing extractions, not readings. The one place P4 is
// consulted at all is the text-flag derivation above, and there only for
// its "Up to" prefix.
//
// # Scope
//
// The EX inventory models NAMES, DIGITS and TEXT only. The chart's printing
// defects in its VALUE legends (recorded in doc.go) are therefore invisible
// to every leg below, by design.

const (
	// transcriptionBPath and groupLedgerPath are relative to the package
	// directory, which is the working directory for `go test`.
	transcriptionBPath = "testdata/transcription-b.csv"
	groupLedgerPath    = "testdata/group-ledger.csv"

	// transcriptionBHeader and groupLedgerHeader are the two artefacts'
	// exact header rows, pinned so that a schema change fails LOUDLY here
	// rather than being silently misparsed into false agreement. B's is the
	// delivered schema of adjudication (a), not the briefed one.
	transcriptionBHeader = "P1,P2,P3,Function,P4,Digits"
	groupLedgerHeader    = "p1,p2,p1_label,p2_label,first_p3,first_name,last_p3,last_name,row_count,pdf_page,visual_anchor"

	transcriptionBFields = 6
	groupLedgerFields    = 11
)

// The text-row pin, fully hardcoded. The FTdx10 chart's ONE text item, at EX
// address (04,01,01). Every component is written out here rather than
// derived from an artefact, because a pin computed from the thing it pins
// proves nothing: these literals came from the chart and must keep agreeing
// with all three derivations of it.
const (
	textRowP1      = 4
	textRowP2      = 1
	textRowP3      = 1
	textRowP1Label = "DISPLAY SETTING"
	textRowP2Label = "DISPLAY"
	// textRowName carries the chart's verbatim trailing full stop.
	textRowName = "MY CALL."
	// textRowDigits is bound to the profile's TextWidth by the pin below. It
	// is spelt as a literal here, and used by B's text derivation, so that
	// the derivation does not depend on the value the pin is checking —
	// deriving B's flag FROM TextWidth would make the pin circular.
	textRowDigits = 12
	// textRowP4Prefix is how the chart writes a character-count parameter
	// ("Up to 12 characters"), as against the "0: X 1: Y" value legends
	// every numeric row carries. It is the discriminator adjudication (a)
	// gives for reconstructing B's missing text flag.
	textRowP4Prefix = "Up to"
)

// address is one EX (P1,P2,P3) triple.
type address struct{ P1, P2, P3 int }

func (a address) String() string { return fmt.Sprintf("(%02d,%02d,%02d)", a.P1, a.P2, a.P3) }

func (a address) group() group { return group{a.P1, a.P2} }

// group is one (P1,P2) chart subgroup — the unit the ledger records.
type group struct{ P1, P2 int }

func (g group) String() string { return fmt.Sprintf("(%02d,%02d)", g.P1, g.P2) }

// entry is the semantic tuple both transcriptions carry for an address:
// bare group labels, the verbatim Function name, the Digits column, and the
// text flag. P4 is absent by adjudication (c); A's manual_line and B's P4
// are single-sourced audit columns and are not part of the comparison.
type entry struct {
	P1Label string
	P2Label string
	Name    string
	Digits  int
	Text    bool
}

func (e entry) String() string {
	return fmt.Sprintf("p1_label=%q p2_label=%q name=%q digits=%d text=%t", e.P1Label, e.P2Label, e.Name, e.Digits, e.Text)
}

// ledgerGroup is one ledger row's bindable content. pdf_page and
// visual_anchor are the ledger's own provenance and describe the printed
// page rather than the inventory, so they are read (the column count is
// pinned) but not bound.
type ledgerGroup struct {
	P1Label   string // normalised to the bare form
	P2Label   string // normalised to the bare form
	FirstP3   int
	FirstName string
	LastP3    int
	LastName  string
	RowCount  int
}

// TestCrossCheckABLedger binds transcription A, transcription B and the
// group-boundary ledger to one another on every leg the milestone specifies.
func TestCrossCheckABLedger(t *testing.T) {
	// The REGISTERED profile, not a literal of this file's own: the whole
	// point is to read A exactly as the generator reads it, under the same
	// digit bounds and the same TextWidth. Lookup by name is sufficient here
	// because this test binds the ARTEFACTS rather than the ownership of the
	// generated file (which is what makes staleness_test.go select by
	// Package instead).
	p, ok := extable.Lookup("ftdx10")
	if !ok {
		t.Fatal(`extable.Lookup("ftdx10"): the profile is not registered`)
	}

	a := loadTranscriptionA(t, p)
	b := loadTranscriptionB(t)
	ledger := loadGroupLedger(t)

	t.Run("A_and_B_agree_address_for_address", func(t *testing.T) {
		// Both directions, separately, so a failure says WHICH artefact is
		// missing the address rather than merely that the sets differ.
		for _, addr := range sortedAddresses(a) {
			if _, in := b[addr]; !in {
				t.Errorf("address %s is in transcription A (%s) but NOT in transcription B (%s): A has %s", addr, p.ManualCSV, transcriptionBPath, a[addr])
			}
		}
		for _, addr := range sortedAddresses(b) {
			if _, in := a[addr]; !in {
				t.Errorf("address %s is in transcription B (%s) but NOT in transcription A (%s): B has %s", addr, transcriptionBPath, p.ManualCSV, b[addr])
			}
		}
		// The full tuple, field by field, exact strings.
		for _, addr := range sortedAddresses(a) {
			bv, in := b[addr]
			if !in {
				continue // already reported above
			}
			av := a[addr]
			if av == bv {
				continue
			}
			t.Errorf("address %s DIFFERS between the transcriptions (%s):\n  A (%s): %s\n  B (%s): %s",
				addr, strings.Join(differingFields(av, bv), ", "), p.ManualCSV, av, transcriptionBPath, bv)
		}
	})

	t.Run("A_against_the_ledger", func(t *testing.T) {
		checkAgainstLedger(t, "transcription A", p.ManualCSV, a, ledger)
	})

	t.Run("B_against_the_ledger", func(t *testing.T) {
		checkAgainstLedger(t, "transcription B", transcriptionBPath, b, ledger)
	})

	t.Run("totals", func(t *testing.T) {
		sum := 0
		for _, g := range sortedLedgerGroups(ledger) {
			sum += ledger[g].RowCount
		}
		// One four-way equality, reported whole: which of the four moved is
		// the first question arbitration asks. The value itself (197) is
		// pinned on the profile by
		// internal/extable/profile_test.go's registration test, so it is
		// deliberately not re-spelt here — this leg binds the artefacts to
		// that pinned constant rather than keeping a second copy of it.
		if len(a) != len(b) || len(a) != sum || len(a) != p.ExpectedRows {
			t.Errorf("row totals disagree: transcription A = %d, transcription B = %d, ledger row_count sum = %d, profile ExpectedRows = %d",
				len(a), len(b), sum, p.ExpectedRows)
		}
	})

	t.Run("the_text_row_pin", func(t *testing.T) {
		if textRowDigits != p.TextWidth {
			t.Errorf("the pinned text-row width %d disagrees with the profile's TextWidth %d", textRowDigits, p.TextWidth)
		}
		want := entry{
			P1Label: textRowP1Label,
			P2Label: textRowP2Label,
			Name:    textRowName,
			Digits:  textRowDigits,
			Text:    true,
		}
		wantAddr := address{textRowP1, textRowP2, textRowP3}
		for _, src := range []struct {
			label string
			path  string
			rows  map[address]entry
		}{
			{"transcription A", p.ManualCSV, a},
			{"transcription B", transcriptionBPath, b},
		} {
			var texts []address
			for _, addr := range sortedAddresses(src.rows) {
				if src.rows[addr].Text {
					texts = append(texts, addr)
				}
			}
			if len(texts) != 1 {
				t.Errorf("%s (%s) has %d text rows, want exactly 1: %v", src.label, src.path, len(texts), texts)
				continue
			}
			if texts[0] != wantAddr {
				t.Errorf("%s (%s): the text row is at %s, want %s", src.label, src.path, texts[0], wantAddr)
				continue
			}
			if got := src.rows[texts[0]]; got != want {
				t.Errorf("%s (%s): the text row at %s is %s, want %s", src.label, src.path, wantAddr, got, want)
			}
		}
	})
}

// checkAgainstLedger binds one transcription's (P1,P2) grouping to the
// ledger: group-key sets both directions, per-group row_count, per-group
// labels, and per-group first/last (P3, name).
func checkAgainstLedger(t *testing.T, srcLabel, srcPath string, rows map[address]entry, ledger map[group]ledgerGroup) {
	t.Helper()

	byGroup := map[group][]address{}
	for _, addr := range sortedAddresses(rows) {
		g := addr.group()
		byGroup[g] = append(byGroup[g], addr)
	}

	groups := sortedGroupKeys(byGroup)
	for _, g := range groups {
		if _, in := ledger[g]; !in {
			t.Errorf("group %s is in %s (%s) but NOT in the ledger (%s): %d rows, first %s", g, srcLabel, srcPath, groupLedgerPath, len(byGroup[g]), byGroup[g][0])
		}
	}
	for _, g := range sortedLedgerGroups(ledger) {
		if _, in := byGroup[g]; !in {
			l := ledger[g]
			t.Errorf("group %s is in the ledger (%s) but NOT in %s (%s): ledger says %d rows, %q..%q", g, groupLedgerPath, srcLabel, srcPath, l.RowCount, l.FirstName, l.LastName)
		}
	}

	for _, g := range groups {
		l, in := ledger[g]
		if !in {
			continue // already reported above
		}
		// sortedAddresses already ordered these by (P1,P2,P3), so the group's
		// slice is ascending in P3. First and last are taken from that order
		// rather than from file order, so neither artefact's row ordering can
		// change what "first" and "last" mean.
		addrs := byGroup[g]
		first, last := addrs[0], addrs[len(addrs)-1]

		if len(addrs) != l.RowCount {
			t.Errorf("group %s: %s (%s) holds %d rows, the ledger (%s) records row_count %d", g, srcLabel, srcPath, len(addrs), groupLedgerPath, l.RowCount)
		}

		// The group's labels must be uniform before "the group's label" means
		// anything, so uniformity is checked first and against the group's
		// lowest-P3 row.
		ref := rows[first]
		for _, addr := range addrs {
			e := rows[addr]
			if e.P1Label != ref.P1Label || e.P2Label != ref.P2Label {
				t.Errorf("group %s: %s (%s) is not label-uniform: %s has p1_label=%q p2_label=%q, %s has p1_label=%q p2_label=%q",
					g, srcLabel, srcPath, first, ref.P1Label, ref.P2Label, addr, e.P1Label, e.P2Label)
			}
		}
		if ref.P1Label != l.P1Label {
			t.Errorf("group %s: %s (%s) has p1_label %q, the ledger (%s) has %q (both bare, per adjudication (b))", g, srcLabel, srcPath, ref.P1Label, groupLedgerPath, l.P1Label)
		}
		if ref.P2Label != l.P2Label {
			t.Errorf("group %s: %s (%s) has p2_label %q, the ledger (%s) has %q (both bare, per adjudication (b))", g, srcLabel, srcPath, ref.P2Label, groupLedgerPath, l.P2Label)
		}

		if first.P3 != l.FirstP3 || rows[first].Name != l.FirstName {
			t.Errorf("group %s: %s (%s) opens at P3 %02d %q, the ledger (%s) records P3 %02d %q", g, srcLabel, srcPath, first.P3, rows[first].Name, groupLedgerPath, l.FirstP3, l.FirstName)
		}
		if last.P3 != l.LastP3 || rows[last].Name != l.LastName {
			t.Errorf("group %s: %s (%s) closes at P3 %02d %q, the ledger (%s) records P3 %02d %q", g, srcLabel, srcPath, last.P3, rows[last].Name, groupLedgerPath, l.LastP3, l.LastName)
		}
	}
}

// loadTranscriptionA reads table2.csv through extable.ParseCSV under the
// registered profile — the same parser, the same bounds and the same
// duplicate-address refusal the generator runs. A's labels are already bare,
// so nothing is normalised on this side.
func loadTranscriptionA(t *testing.T, p extable.Profile) map[address]entry {
	t.Helper()
	data, err := os.ReadFile(p.ManualCSV)
	if err != nil {
		t.Fatalf("reading transcription A (%s): %v", p.ManualCSV, err)
	}
	rows, err := extable.ParseCSV(p, data)
	if err != nil {
		t.Fatalf("ParseCSV(%s): %v", p.ManualCSV, err)
	}
	// ParseCSV itself refuses a duplicate (P1,P2,P3), so this map cannot
	// silently absorb one.
	out := make(map[address]entry, len(rows))
	for _, r := range rows {
		out[address{r.P1, r.P2, r.P3}] = entry{
			P1Label: r.P1Label,
			P2Label: r.P2Label,
			Name:    r.Name,
			Digits:  r.Digits,
			Text:    r.Text,
		}
	}
	return out
}

// loadTranscriptionB reads B with encoding/csv and applies adjudication
// (a)'s adapter. Nothing here parses prose with a regular expression: the
// file is read as CSV, and the only structured field is the group-label
// cell, split by splitWrapped's exact "NN (LABEL)" rule.
func loadTranscriptionB(t *testing.T) map[address]entry {
	t.Helper()
	records := readCSV(t, transcriptionBPath, transcriptionBHeader, transcriptionBFields)

	out := make(map[address]entry, len(records))
	for i, rec := range records {
		where := fmt.Sprintf("%s data row %d", transcriptionBPath, i+1)
		p1, p1Label := splitWrapped(t, where+" P1", rec[0])
		p2, p2Label := splitWrapped(t, where+" P2", rec[1])
		p3, err := strconv.Atoi(rec[2])
		if err != nil {
			t.Fatalf("%s: bad P3 %q: %v", where, rec[2], err)
		}
		digits, err := strconv.Atoi(rec[5])
		if err != nil {
			t.Fatalf("%s: bad Digits %q: %v", where, rec[5], err)
		}
		addr := address{p1, p2, p3}
		if prev, dup := out[addr]; dup {
			t.Fatalf("%s: duplicate address %s (already held %s)", where, addr, prev)
		}
		out[addr] = entry{
			P1Label: p1Label,
			P2Label: p2Label,
			Name:    rec[3],
			Digits:  digits,
			// The reconstructed text flag: B carries no such column. See
			// adjudication (a).
			Text: digits == textRowDigits && strings.HasPrefix(rec[4], textRowP4Prefix),
		}
	}
	return out
}

// loadGroupLedger reads the ledger with encoding/csv and normalises its
// verbatim wrapped labels to the bare form (adjudication (b)).
func loadGroupLedger(t *testing.T) map[group]ledgerGroup {
	t.Helper()
	records := readCSV(t, groupLedgerPath, groupLedgerHeader, groupLedgerFields)

	out := make(map[group]ledgerGroup, len(records))
	for i, rec := range records {
		where := fmt.Sprintf("%s data row %d", groupLedgerPath, i+1)
		p1 := atoiOrFatal(t, where, "p1", rec[0])
		p2 := atoiOrFatal(t, where, "p2", rec[1])
		p1Num, p1Label := splitWrapped(t, where+" p1_label", rec[2])
		p2Num, p2Label := splitWrapped(t, where+" p2_label", rec[3])
		// The ledger records each group's number twice — once as a column and
		// once inside the printed label — so the two are bound to each other
		// here. Nothing else in the cross-check would notice them disagreeing.
		if p1Num != p1 {
			t.Fatalf("%s: p1 column is %02d but p1_label %q carries %02d", where, p1, rec[2], p1Num)
		}
		if p2Num != p2 {
			t.Fatalf("%s: p2 column is %02d but p2_label %q carries %02d", where, p2, rec[3], p2Num)
		}
		g := group{p1, p2}
		if _, dup := out[g]; dup {
			t.Fatalf("%s: duplicate group %s", where, g)
		}
		out[g] = ledgerGroup{
			P1Label:   p1Label,
			P2Label:   p2Label,
			FirstP3:   atoiOrFatal(t, where, "first_p3", rec[4]),
			FirstName: rec[5],
			LastP3:    atoiOrFatal(t, where, "last_p3", rec[6]),
			LastName:  rec[7],
			RowCount:  atoiOrFatal(t, where, "row_count", rec[8]),
		}
	}
	return out
}

// readCSV reads path as CSV, requires its first record to be exactly
// wantHeader, and returns the data records.
//
// Comment = '#' covers both files' conventions: neither artefact carries
// comment lines TODAY (A does, and A is not read here; B lost its header
// block to the stall of adjudication (a), and the ledger puts its prose in
// the .md companion), but the repository's CSV artefacts are '#'-commented
// by convention and a provenance block added later must not break this
// parser. FieldsPerRecord is set explicitly rather than inferred from the
// first record, so a file that is internally consistent but the wrong shape
// still fails.
//
// B is CRLF-terminated, the ledger LF-terminated — the two quarantined
// agents wrote on different days through different tooling. encoding/csv
// strips the CR of a CRLF terminator, so both parse identically and neither
// artefact is touched to tidy that up.
func readCSV(t *testing.T, path, wantHeader string, fields int) [][]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comment = '#'
	r.FieldsPerRecord = fields
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if len(records) == 0 {
		t.Fatalf("%s is empty", path)
	}
	if got := strings.Join(records[0], ","); got != wantHeader {
		t.Fatalf("%s header is\n  %s\nwant\n  %s", path, got, wantHeader)
	}
	if len(records) == 1 {
		t.Fatalf("%s carries a header and no data rows", path)
	}
	return records[1:]
}

// splitWrapped decodes the chart's printed group-label form "NN (LABEL)"
// into its number and its BARE label — adjudication (b)'s normalisation, and
// adjudication (a)'s source of B's P1/P2 numbers.
//
// It is deliberately exact: the cell must be two digits, a space, an opening
// parenthesis, a non-blank label, and a closing parenthesis at the very end.
// A cell that is merely similar is a Fatal, not a silent pass-through,
// because a permissive fallback is precisely how a genuinely different label
// would be normalised into false agreement.
func splitWrapped(t *testing.T, where, raw string) (int, string) {
	t.Helper()
	// "NN (LABEL)" is at minimum six bytes: two digits, " (", one label
	// byte, ")". The length is checked FIRST so the slice below cannot panic
	// on a short cell.
	const openAt = 2 // "NN" ends here; " (" follows
	if len(raw) < openAt+4 || raw[openAt:openAt+2] != " (" || !strings.HasSuffix(raw, ")") {
		t.Fatalf(`%s: %q is not the chart's printed group-label form "NN (LABEL)"`, where, raw)
	}
	n, err := strconv.Atoi(raw[:openAt])
	if err != nil {
		t.Fatalf("%s: %q does not open with a two-digit group number: %v", where, raw, err)
	}
	label := raw[openAt+2 : len(raw)-1]
	if strings.TrimSpace(label) == "" {
		t.Fatalf("%s: %q wraps a blank label", where, raw)
	}
	return n, label
}

func atoiOrFatal(t *testing.T, where, field, raw string) int {
	t.Helper()
	n, err := strconv.Atoi(raw)
	if err != nil {
		t.Fatalf("%s: bad %s %q: %v", where, field, raw, err)
	}
	return n
}

// differingFields names the tuple fields on which two entries disagree, so a
// failure message says WHAT differs before it says how.
func differingFields(a, b entry) []string {
	var out []string
	if a.P1Label != b.P1Label {
		out = append(out, "p1_label")
	}
	if a.P2Label != b.P2Label {
		out = append(out, "p2_label")
	}
	if a.Name != b.Name {
		out = append(out, "name")
	}
	if a.Digits != b.Digits {
		out = append(out, "digits")
	}
	if a.Text != b.Text {
		out = append(out, "text")
	}
	return out
}

// sortedAddresses returns m's addresses in (P1,P2,P3) order, so that every
// failure list is stable and every group slice is ascending in P3.
func sortedAddresses(m map[address]entry) []address {
	out := make([]address, 0, len(m))
	for a := range m {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].P1 != out[j].P1 {
			return out[i].P1 < out[j].P1
		}
		if out[i].P2 != out[j].P2 {
			return out[i].P2 < out[j].P2
		}
		return out[i].P3 < out[j].P3
	})
	return out
}

func sortedGroupKeys(m map[group][]address) []group {
	out := make([]group, 0, len(m))
	for g := range m {
		out = append(out, g)
	}
	sortGroups(out)
	return out
}

func sortedLedgerGroups(m map[group]ledgerGroup) []group {
	out := make([]group, 0, len(m))
	for g := range m {
		out = append(out, g)
	}
	sortGroups(out)
	return out
}

func sortGroups(gs []group) {
	sort.Slice(gs, func(i, j int) bool {
		if gs[i].P1 != gs[j].P1 {
			return gs[i].P1 < gs[j].P1
		}
		return gs[i].P2 < gs[j].P2
	})
}
