// SPDX-License-Identifier: GPL-3.0-or-later

package ic7610_test

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7610"
)

// This file binds three independent readings of PDF p.12 to one another: the
// L leg's field ledger (labels and printed indices, derived first), the B
// leg's semantic transcription (widths, encodings and value lists, derived
// without sight of L), and the profile core/civ/ic7610 builds from them.
//
// THE DISCIPLINE, taken from core/cat/ftdx101/crosscheck_test.go: strict
// decomposition, no substring matching, no permissive fallback, and EVERY ROW
// CONSUMED. A row this test does not understand is a failure, because a
// silently ignored row is a piece of evidence that stopped being evidence.
//
// ANY mismatch is a STOP for orchestrator arbitration AGAINST THE PDF, which
// may correct L, B or the profile - never this test, and never an artefact
// edited merely to make the test pass. That is why every failure prints both
// sides.

// ---------------------------------------------------------------------------
// Per-leg index normalisers
// ---------------------------------------------------------------------------

// circledValue maps one circled-numeral rune to its integer, and reports
// false for anything else. TWO RANGES, and they are not contiguous:
// (1)-(20) are U+2460..U+2473 and (21)-(35) are U+3251..U+325F. A
// single-range reading would silently misnumber every index above 20 -
// which on this model is the (27) that closes the name field, i.e. the
// record length itself.
func circledValue(r rune) (int, bool) {
	switch {
	case r >= '①' && r <= '⑳':
		return int(r-'①') + 1, true
	case r >= '㉑' && r <= '㉟':
		return int(r-'㉑') + 21, true
	default:
		return 0, false
	}
}

// indexKey is the canonical form both legs normalise to: an inclusive
// printed-index range.
type indexKey struct{ lo, hi int }

func (k indexKey) String() string {
	if k.lo == k.hi {
		return strconv.Itoa(k.lo)
	}
	return fmt.Sprintf("%d..%d", k.lo, k.hi)
}

// width is the key's extent in printed indices.
func (k indexKey) width() int { return k.hi - k.lo + 1 }

// parseCircledIndex decomposes B's spelling STRICTLY: exactly "<c>",
// "<c>, <c>" or "<c> ~ <c>", every <c> a circled numeral, no leading or
// trailing text. A merely-similar cell is an error, never a pass-through,
// because a permissive fallback is precisely how a genuinely different
// index would be normalised into false agreement.
func parseCircledIndex(s string) (indexKey, error) {
	one := func(cell string) (int, error) {
		r := []rune(cell)
		if len(r) != 1 {
			return 0, fmt.Errorf("%q is not a single circled numeral", cell)
		}
		n, ok := circledValue(r[0])
		if !ok {
			return 0, fmt.Errorf("%q is not a circled numeral", cell)
		}
		return n, nil
	}
	for _, sep := range []string{", ", " ~ "} {
		lhs, rhs, found := strings.Cut(s, sep)
		if !found {
			continue
		}
		lo, err := one(lhs)
		if err != nil {
			return indexKey{}, fmt.Errorf("circled index %q: left cell: %w", s, err)
		}
		hi, err := one(rhs)
		if err != nil {
			return indexKey{}, fmt.Errorf("circled index %q: right cell: %w", s, err)
		}
		if hi <= lo {
			return indexKey{}, fmt.Errorf("circled index %q runs %d..%d, which is not ascending", s, lo, hi)
		}
		return indexKey{lo, hi}, nil
	}
	n, err := one(s)
	if err != nil {
		return indexKey{}, fmt.Errorf("circled index %q: %w", s, err)
	}
	return indexKey{n, n}, nil
}

// parseLedgerIndex decomposes L's ASCII spelling STRICTLY: exactly "<n>",
// "<n>, <n>" or "<n> ~ <n>", with an OPTIONAL single trailing colon (the
// D4 rows print one, and the ledger's own notes say it is transcribed
// verbatim). Anything else is an error.
func parseLedgerIndex(s string) (indexKey, error) {
	body := strings.TrimSuffix(s, ":")
	if strings.HasSuffix(body, ":") {
		return indexKey{}, fmt.Errorf("ledger index %q carries more than one trailing colon", s)
	}
	one := func(cell string) (int, error) {
		if cell == "" {
			return 0, fmt.Errorf("empty cell")
		}
		for _, r := range cell {
			if r < '0' || r > '9' {
				return 0, fmt.Errorf("%q is not a decimal numeral", cell)
			}
		}
		return strconv.Atoi(cell)
	}
	for _, sep := range []string{", ", " ~ "} {
		lhs, rhs, found := strings.Cut(body, sep)
		if !found {
			continue
		}
		lo, err := one(lhs)
		if err != nil {
			return indexKey{}, fmt.Errorf("ledger index %q: left cell: %w", s, err)
		}
		hi, err := one(rhs)
		if err != nil {
			return indexKey{}, fmt.Errorf("ledger index %q: right cell: %w", s, err)
		}
		if hi <= lo {
			return indexKey{}, fmt.Errorf("ledger index %q runs %d..%d, which is not ascending", s, lo, hi)
		}
		return indexKey{lo, hi}, nil
	}
	n, err := one(body)
	if err != nil {
		return indexKey{}, fmt.Errorf("ledger index %q: %w", s, err)
	}
	return indexKey{n, n}, nil
}

// TestIndexNormalisers tests the normalisation before anything is joined
// through it, because the normalisation is load-bearing: it is the only
// place the two legs' different spellings are made comparable, and a
// permissive one would manufacture agreement.
func TestIndexNormalisers(t *testing.T) {
	t.Run("circled", func(t *testing.T) {
		ok := map[string]indexKey{
			"①":     {1, 1},
			"①, ②":  {1, 2},
			"④ ~ ⑧": {4, 8},
			"⑱ ~ ㉗": {18, 27},
			// The exact error a single-range reading would make: U+3257 is
			// (27), not 6 and not 7.
			"㉗": {27, 27},
		}
		for in, want := range ok {
			got, err := parseCircledIndex(in)
			if err != nil {
				t.Errorf("parseCircledIndex(%q): %v", in, err)
				continue
			}
			if got != want {
				t.Errorf("parseCircledIndex(%q) = %v, want %v", in, got, want)
			}
		}
		bad := []string{
			"1, 2",   // ASCII, not circled
			"3",      // ASCII
			"④ ~ ⑧ ", // trailing space
			" ④ ~ ⑧", // leading space
			"④~⑧",    // the separator is " ~ ", spaces included
			"④, ",    // empty right cell
			"⑧ ~ ④",  // descending
			"③:",     // L's trailing colon is not B's spelling
			"",
		}
		for _, in := range bad {
			if got, err := parseCircledIndex(in); err == nil {
				t.Errorf("parseCircledIndex(%q) = %v, want an error - a permissive fallback is how a genuinely different index becomes false agreement", in, got)
			}
		}
	})

	t.Run("ledger", func(t *testing.T) {
		ok := map[string]indexKey{
			"3":       {3, 3},
			"1, 2":    {1, 2},
			"4 ~ 8":   {4, 8},
			"18 ~ 27": {18, 27},
			"3:":      {3, 3},
			"1, 2:":   {1, 2},
		}
		for in, want := range ok {
			got, err := parseLedgerIndex(in)
			if err != nil {
				t.Errorf("parseLedgerIndex(%q): %v", in, err)
				continue
			}
			if got != want {
				t.Errorf("parseLedgerIndex(%q) = %v, want %v", in, got, want)
			}
		}
		bad := []string{
			"①",      // circled, not ASCII
			"4 ~ 8 ", // trailing space
			"3::",    // two colons
			"4~8",    // the separator is " ~ "
			"8 ~ 4",  // descending
			"3 apples",
			"",
		}
		for _, in := range bad {
			if got, err := parseLedgerIndex(in); err == nil {
				t.Errorf("parseLedgerIndex(%q) = %v, want an error", in, got)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// Reading the two artefacts
// ---------------------------------------------------------------------------

// readEvidenceCSV reads one frozen artefact strictly. Comment is left at 0
// and LazyQuotes at false on purpose: nothing in a quarantined artefact may
// be skipped or repaired by the reader, and FieldsPerRecord is stated rather
// than inferred so a file that is internally consistent but the wrong shape
// still fails.
func readEvidenceCSV(t *testing.T, name, wantHeader string, fields int) [][]string {
	t.Helper()
	path := filepath.Join(testdataDir, name)
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = fields
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if len(records) < 2 {
		t.Fatalf("%s carries %d records; want a header and at least one data row", path, len(records))
	}
	if got := strings.Join(records[0], ","); got != wantHeader {
		t.Fatalf("%s header is\n  %s\nwant\n  %s", path, got, wantHeader)
	}
	return records[1:]
}

// ledgerRow is one row of the L leg, by column name.
type ledgerRow struct {
	diagram, indexRaw, indexStyle, label, page, anchor, notes string
	key                                                       indexKey
}

// transcriptionRow is one row of the B leg, by column name.
type transcriptionRow struct {
	diagram, indexRaw, label, widthRaw, encoding, values, page, anchor, notes string
	key                                                                       indexKey
	width                                                                     int
}

const (
	ledgerHeader = "diagram_id,field_index,index_style,label_verbatim,pdf_page,visual_anchor,notes"
	bHeader      = "diagram_id,field_index,label_verbatim,width_bytes,encoding,values_verbatim,pdf_page,visual_anchor,notes"
)

// loadLedger reads every L row and normalises its index. EVERY ROW IS
// CONSUMED: an unparsable index or an unknown diagram_id is a Fatal, not a
// skip.
func loadLedger(t *testing.T) []ledgerRow {
	t.Helper()
	out := make([]ledgerRow, 0, 13)
	for i, rec := range readEvidenceCSV(t, "ic7610-field-ledger.csv", ledgerHeader, 7) {
		row := ledgerRow{
			diagram: rec[0], indexRaw: rec[1], indexStyle: rec[2], label: rec[3],
			page: rec[4], anchor: rec[5], notes: rec[6],
		}
		switch row.diagram {
		case "D1", "D2", "D3", "D4":
		default:
			t.Fatalf("ledger row %d carries diagram_id %q; the ledger describes D1..D4 and nothing else", i+2, row.diagram)
		}
		key, err := parseLedgerIndex(row.indexRaw)
		if err != nil {
			t.Fatalf("ledger row %d: %v", i+2, err)
		}
		row.key = key
		out = append(out, row)
	}
	return out
}

// loadTranscription reads every B row the same way.
func loadTranscription(t *testing.T) []transcriptionRow {
	t.Helper()
	out := make([]transcriptionRow, 0, 10)
	for i, rec := range readEvidenceCSV(t, "ic7610-transcription-b.csv", bHeader, 9) {
		row := transcriptionRow{
			diagram: rec[0], indexRaw: rec[1], label: rec[2], widthRaw: rec[3],
			encoding: rec[4], values: rec[5], page: rec[6], anchor: rec[7], notes: rec[8],
		}
		switch row.diagram {
		case "D1", "D2", "D3":
		default:
			t.Fatalf("transcription row %d carries diagram_id %q; B describes D1..D3 and nothing else", i+2, row.diagram)
		}
		key, err := parseCircledIndex(row.indexRaw)
		if err != nil {
			t.Fatalf("transcription row %d: %v", i+2, err)
		}
		row.key = key
		w, err := strconv.Atoi(row.widthRaw)
		if err != nil {
			t.Fatalf("transcription row %d: width_bytes %q is not a number: %v", i+2, row.widthRaw, err)
		}
		row.width = w
		out = append(out, row)
	}
	return out
}

// ---------------------------------------------------------------------------
// The binding table: printed row -> ORDERED span-union (adjudication R8)
// ---------------------------------------------------------------------------

// rowBinding declares, for one printed D1 row, which profile spans carry it
// and what NON-SPAN content the rest of its bytes hold. Three rows map to
// something other than one span and each is named explicitly here rather
// than skipped: a skipped row is evidence that stopped being evidence.
type rowBinding struct {
	// fields are the union's member spans, IN OFFSET ORDER. Empty means the
	// row is carried by no span at all, which is a claim this table makes
	// deliberately and leg 4 then checks against the profile.
	fields []civ.FieldID
	// unmapped are the record offsets this row covers that no span claims,
	// wholly or in part - the E6 regions, carried by the Fixed template.
	unmapped []int
	// isAddress marks the one row that is not record content at all.
	isAddress bool
	// why explains the non-span content in a failure message.
	why string
}

// d1Bindings is R8's table, keyed by the canonical printed-index range.
var d1Bindings = map[indexKey]rowBinding{
	{1, 2}:   {isAddress: true, why: "the 2-byte ADDRESS, outside the record (spec Erratum 1)"},
	{3, 3}:   {unmapped: []int{0}, why: "Fixed[0] == 0x00: high nibble the printed Fixed 0, low nibble the E6-unmapped SELECT marker"},
	{4, 8}:   {fields: []civ.FieldID{civ.FieldRXFrequency}},
	{9, 10}:  {fields: []civ.FieldID{civ.FieldMode, civ.FieldFilter}},
	{11, 11}: {fields: []civ.FieldID{civ.FieldToneMode}, unmapped: []int{8}, why: "Fixed[8] high nibble 0: the E6-unmapped data mode"},
	{12, 14}: {fields: []civ.FieldID{civ.FieldToneTX}},
	{15, 17}: {fields: []civ.FieldID{civ.FieldToneRX}},
	{18, 27}: {fields: []civ.FieldID{civ.FieldName}},
}

// TestCrosscheck_LedgerAndTranscriptionAndProfile binds three independent
// readings of PDF p.12 to one another: the L leg's field ledger (labels and
// printed indices, derived first), the B leg's semantic transcription
// (widths, encodings and value lists, derived without sight of L), and the
// profile core/civ/ic7610 builds from them. Agreement between three blind
// derivations is the evidence; this test is where that agreement is made
// mechanical rather than asserted in prose.
//
// ANY mismatch is a STOP for orchestrator arbitration AGAINST THE PDF,
// which may correct L, B or the profile - never this test, and never an
// artefact edited merely to make the test pass. That is why every failure
// prints both sides.
func TestCrosscheck_LedgerAndTranscriptionAndProfile(t *testing.T) {
	ledger := loadLedger(t)
	transcription := loadTranscription(t)

	ledgerD1 := map[indexKey]ledgerRow{}
	ledgerOther := map[string][]ledgerRow{}
	for _, row := range ledger {
		if row.diagram != "D1" {
			ledgerOther[row.diagram] = append(ledgerOther[row.diagram], row)
			continue
		}
		if prev, dup := ledgerD1[row.key]; dup {
			t.Fatalf("ledger has two D1 rows for printed index %s: %q and %q", row.key, prev.label, row.label)
		}
		ledgerD1[row.key] = row
	}

	bD1 := map[indexKey]transcriptionRow{}
	bOther := map[string][]transcriptionRow{}
	for _, row := range transcription {
		if row.diagram != "D1" {
			bOther[row.diagram] = append(bOther[row.diagram], row)
			continue
		}
		if prev, dup := bD1[row.key]; dup {
			t.Fatalf("transcription has two D1 rows for printed index %s: %q and %q", row.key, prev.label, row.label)
		}
		bD1[row.key] = row
	}

	// --- Leg 1: D1 rows - L is congruent with B on label and index --------
	t.Run("leg1_ledger_equals_transcription", func(t *testing.T) {
		if len(ledgerD1) != 8 {
			t.Fatalf("ledger has %d D1 rows, want 8 - the memory-content strip prints eight bracket groups", len(ledgerD1))
		}
		if len(bD1) != 8 {
			t.Fatalf("transcription has %d D1 rows, want 8", len(bD1))
		}
		for key, l := range ledgerD1 {
			b, ok := bD1[key]
			if !ok {
				t.Errorf("ledger carries D1 printed index %s (%q) and the transcription does not - the two legs disagree about which groups the strip prints",
					key, l.label)
				continue
			}
			if l.label != b.label {
				t.Errorf("D1 printed index %s: the two legs read the label differently\n  ledger:        %q\n  transcription: %q\nThis is a STOP for arbitration against PDF p.12, not something to reconcile here.",
					key, l.label, b.label)
			}
		}
		for key, b := range bD1 {
			if _, ok := ledgerD1[key]; !ok {
				t.Errorf("transcription carries D1 printed index %s (%q) and the ledger does not", key, b.label)
			}
		}
	})

	// --- Leg 2: B's widths tile 1..27 -------------------------------------
	t.Run("leg2_transcription_widths_tile_the_data_area", func(t *testing.T) {
		keys := make([]indexKey, 0, len(bD1))
		for k := range bD1 {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i].lo < keys[j].lo })

		next, sum := 1, 0
		for _, k := range keys {
			row := bD1[k]
			if k.width() != row.width {
				t.Errorf("D1 printed index %s declares width_bytes %d, but its own index range spans %d - the row disagrees with itself",
					k, row.width, k.width())
			}
			if k.lo != next {
				t.Errorf("D1 printed index %s starts at %d, want %d - the strip's groups must tile with no gap and no overlap", k, k.lo, next)
			}
			next = k.hi + 1
			sum += row.width
		}
		if sum != ic7610.DataAreaLength {
			t.Errorf("B's widths sum to %d, want %d (DataAreaLength) - the eight-term addition of matrix S3.11", sum, ic7610.DataAreaLength)
		}
		if last := keys[len(keys)-1]; last.hi != 27 {
			t.Errorf("the last printed index is %d, want 27 - the internal check the strip itself offers is that the sum equals the last index", last.hi)
		}
	})

	// --- Leg 3: B is congruent with the profile, via the span-union table --
	layout := ic7610.Profile().Layouts()[0]
	spansByField := map[civ.FieldID]civ.FieldSpan{}
	for _, sp := range layout.Fields {
		if _, dup := spansByField[sp.Field]; dup {
			t.Fatalf("the layout carries two spans for %s; this crosscheck's union table assumes one each on this model", sp.Field)
		}
		spansByField[sp.Field] = sp
	}
	reached := map[civ.FieldID]int{}

	t.Run("leg3_transcription_equals_the_profile", func(t *testing.T) {
		for key, row := range bD1 {
			bind, ok := d1Bindings[key]
			if !ok {
				t.Errorf("printed index %s has no declared span-union; every printed row is bound explicitly, never skipped", key)
				continue
			}

			if bind.isAddress {
				if key.width() != ic7610.AddressBytes {
					t.Errorf("the address row %s spans %d printed indices, want %d (ic7610.AddressBytes)", key, key.width(), ic7610.AddressBytes)
				}
				for _, sp := range layout.Fields {
					if sp.Offset < 0 {
						t.Errorf("%s sits at a negative offset", sp.Field)
					}
				}
				// The address lies outside the record entirely, so there is
				// no record offset it could occupy: printed 1 and 2 map to
				// record offsets -2 and -1.
				continue
			}

			// Coverage: the union's member spans plus the row's named
			// non-span content must cover exactly key.lo-3 .. key.hi-3.
			covered := map[int]bool{}
			prevOffset := -1
			for _, id := range bind.fields {
				sp, ok := spansByField[id]
				if !ok {
					t.Errorf("printed index %s declares a union member %s that the layout does not carry", key, id)
					continue
				}
				if sp.Offset <= prevOffset {
					t.Errorf("printed index %s declares its union out of offset order: %s sits at %d, after an earlier member at %d",
						key, id, sp.Offset, prevOffset)
				}
				prevOffset = sp.Offset
				reached[id]++
				for i := 0; i < sp.Length; i++ {
					covered[sp.Offset+i] = true
				}
			}
			for _, off := range bind.unmapped {
				covered[off] = true
			}

			wantLo, wantHi := key.lo-3, key.hi-3
			for off := wantLo; off <= wantHi; off++ {
				if !covered[off] {
					t.Errorf("printed index %s leaves record offset %d uncovered; the union is %v and its named non-span content is %q",
						key, off, bind.fields, bind.why)
				}
			}
			for off := range covered {
				if off < wantLo || off > wantHi {
					t.Errorf("printed index %s covers record offset %d, which lies outside its own range %d..%d",
						key, off, wantLo, wantHi)
				}
			}

			// B's encoding column must agree with each member span.
			checkEncoding(t, key, row.encoding, bind, spansByField)
		}

		for _, sp := range layout.Fields {
			switch reached[sp.Field] {
			case 1:
			case 0:
				t.Errorf("the layout's %s span at offset %d is reached by NO printed row's union - a span nothing in the evidence binds is a span nobody checked",
					sp.Field, sp.Offset)
			default:
				t.Errorf("the layout's %s span is reached by %d unions, want exactly 1", sp.Field, reached[sp.Field])
			}
		}
	})

	// --- Leg 4: the unmapped rows are consumed, not skipped ----------------
	t.Run("leg4_unmapped_rows_are_consumed", func(t *testing.T) {
		fixed := ic7610.FixedTemplate()

		if _, ok := bD1[indexKey{3, 3}]; !ok {
			t.Fatal("the transcription has no D1 row for printed index 3; the unmapped SELECT byte must still be present as evidence")
		}
		if got := fixed[ic7610.SelectNibbleOffset]; got != 0x00 {
			t.Errorf("FixedTemplate()[%d] = %#02x, want 0x00 - printed (3) is UNMAPPED under ruling E6 and the template is what a write is judged against",
				ic7610.SelectNibbleOffset, got)
		}
		if ic7610.SelectNibbleOffset != 0 {
			t.Errorf("ic7610.SelectNibbleOffset = %d, want 0 (printed index 3, minus 2 address bytes, minus 1 for 0-based)", ic7610.SelectNibbleOffset)
		}
		for _, sp := range layout.Fields {
			if sp.Offset == ic7610.SelectNibbleOffset {
				t.Errorf("%s claims record offset %d. Printed (3) carries a FOUR-VALUED select-scan group marker whose neutral home is a BoolField, so ruling E6 leaves it UNMAPPED and refuses a write whose bytes differ from the template. Restoring a span here would collapse (star)1/(star)2/(star)3 to one value on every write-back.",
					sp.Field, sp.Offset)
			}
		}

		if _, ok := bD1[indexKey{11, 11}]; !ok {
			t.Fatal("the transcription has no D1 row for printed index 11")
		}
		if got := fixed[ic7610.DataModeNibbleOffset]; got != 0x00 {
			t.Errorf("FixedTemplate()[%d] = %#02x, want 0x00", ic7610.DataModeNibbleOffset, got)
		}
		if ic7610.DataModeNibbleOffset != 8 {
			t.Errorf("ic7610.DataModeNibbleOffset = %d, want 8 (printed index 11)", ic7610.DataModeNibbleOffset)
		}
		var atEight []civ.FieldSpan
		for _, sp := range layout.Fields {
			if sp.Offset == ic7610.DataModeNibbleOffset {
				atEight = append(atEight, sp)
			}
		}
		if len(atEight) != 1 || atEight[0].Field != civ.FieldToneMode || atEight[0].Nibble != civ.NibbleLow {
			t.Errorf("record offset %d carries %v; want exactly one span, tone_mode on the LOW nibble. The HIGH nibble is the four-valued data mode, UNMAPPED under ruling E6 - its neutral home is a BoolField and a 4->2 collapse would rewrite a user's data mode while readback compared equal.",
				ic7610.DataModeNibbleOffset, atEight)
		}
	})

	// --- Leg 5: D2 and D3 sub-diagram rows are NIBBLE evidence -------------
	t.Run("leg5_subdiagram_rows_are_nibble_evidence", func(t *testing.T) {
		d2 := bOther["D2"]
		if len(d2) != 1 || d2[0].key != (indexKey{3, 3}) {
			t.Fatalf("B carries %d D2 rows (%v); want exactly one, for printed index 3", len(d2), d2)
		}
		selectValues := splitPrintedList(d2[0].values, " | ")
		want := []string{"0=OFF", "1= ★1", "2= ★2", "3= ★3"}
		if strings.Join(selectValues, "|") != strings.Join(want, "|") {
			t.Errorf("B's D2 row prints the SELECT marker's values as %q, want %q - this is the evidence that the nibble is four-valued",
				selectValues, want)
		}
		// The evidence is PRESENT and deliberately UNCONSUMED: ruling E6
		// leaves the nibble unmapped, so no enum in the profile may carry
		// this vocabulary.
		names := make([]string, 0, len(selectValues))
		for _, v := range selectValues {
			_, name, ok := strings.Cut(v, "=")
			if !ok {
				t.Fatalf("B's D2 value %q is not the printed <code>=<name> form", v)
			}
			names = append(names, strings.TrimSpace(name))
		}
		for _, sp := range layout.Fields {
			if sp.Enum == nil {
				continue
			}
			have := map[string]bool{}
			for _, n := range sp.Enum {
				have[n] = true
			}
			all := true
			for _, n := range names {
				if !have[n] {
					all = false
					break
				}
			}
			if all {
				t.Errorf("%s's enum carries the SELECT-group vocabulary %v. That vocabulary is recorded evidence and deliberately UNCONSUMED: E6 leaves printed (3)'s low nibble unmapped because codeplug's ScanSkip is a BoolField.",
					sp.Field, names)
			}
		}

		d3 := bOther["D3"]
		if len(d3) != 1 || d3[0].key != (indexKey{11, 11}) {
			t.Fatalf("B carries %d D3 rows (%v); want exactly one, for printed index 11", len(d3), d3)
		}
		lists := splitPrintedList(d3[0].values, " | ")
		if len(lists) != 2 {
			t.Fatalf("B's D3 row prints %d value lists, want 2 - one per nibble of printed (11)", len(lists))
		}
		firstPrinted := parseColonList(t, lists[0])
		secondPrinted := parseColonList(t, lists[1])

		toneMode := spansByField[civ.FieldToneMode]
		if toneMode.Nibble != civ.NibbleLow {
			t.Fatalf("tone_mode sits on nibble %v, want NibbleLow", toneMode.Nibble)
		}
		if !sameEnum(toneMode.Enum, firstPrinted) {
			t.Errorf("the profile puts %v on printed (11)'s LOW nibble; B's FIRST-PRINTED list is %v.\n"+
				"THE ASSIGNMENT IS INVERTED RELATIVE TO READING ORDER AND MUST NOT BE \"CORRECTED\": the two leaders NEST rather than cross, so the upper label belongs to the RIGHT (low) nibble. See matrix Errata (rev 1) erratum 5, and the W leg's STOP 5, the B leg's rows D1,(11) and D3,(11) and the G leg's hazard (c), which traced the leaders independently and agree.\n"+
				"  second-printed list (the LEFT/high nibble, UNMAPPED under E6): %v",
				toneMode.Enum, firstPrinted, secondPrinted)
		}
		for _, sp := range layout.Fields {
			if sp.Enum != nil && sameEnum(sp.Enum, secondPrinted) {
				t.Errorf("%s's enum carries the DATA-mode vocabulary %v, which E6 leaves unmapped on the HIGH nibble of record offset %d",
					sp.Field, secondPrinted, ic7610.DataModeNibbleOffset)
			}
		}
	})

	// --- Leg 6: D4 rows are RECORDED-NOT-BUILT ----------------------------
	//
	// The clear form is evidence this tier deliberately does not act on. The
	// (3) Fixed-0 versus "FF" contradiction is recorded in both legs and
	// THIS PLAN RECONCILES NEITHER: the record diagram prints (3)'s high
	// nibble as a Fixed 0, the clear list four inches away prints (3): "FF",
	// and only a capture from a real radio can settle which describes what.
	t.Run("leg6_clear_form_is_recorded_not_built", func(t *testing.T) {
		d4 := ledgerOther["D4"]
		if len(d4) != 3 {
			t.Fatalf("the ledger carries %d D4 rows, want 3 - the clear list prints three lines and every one is evidence", len(d4))
		}
		byKey := map[indexKey]ledgerRow{}
		for _, row := range d4 {
			byKey[row.key] = row
		}
		for _, k := range []indexKey{{1, 2}, {3, 3}, {4, 4}} {
			if _, ok := byKey[k]; !ok {
				t.Errorf("the ledger's D4 rows do not include printed index %s", k)
			}
		}

		// The contradiction, both halves, present in the evidence.
		if got := byKey[indexKey{3, 3}].label; got != "“FF”" {
			t.Errorf("the ledger's D4 (3) row reads %q, want the printed “FF” - this is one half of the recorded contradiction", got)
		}
		if d2 := ledgerOther["D2"]; len(d2) != 1 || !strings.Contains(d2[0].notes, "Fixed") {
			t.Errorf("the ledger's D2 row does not record the printed \"Fixed\" leader on (3)'s left nibble - this is the other half of the contradiction, and both must survive in the evidence")
		}
		if b, ok := bD1[indexKey{3, 3}]; !ok || !strings.Contains(b.values, "“FF”") {
			t.Errorf("B's D1 (3) row does not carry the clear list's “FF”; the two legs must both record it")
		}

		// No builder makes the clear frame, and the gate refuses it for
		// every channel this profile admits.
		p := ic7610.Profile()
		idRead, err := p.BuildTransceiverIDRead()
		if err != nil {
			t.Fatalf("BuildTransceiverIDRead: %v", err)
		}
		lo, hi := p.ChannelRange()
		for ch := lo; ch <= hi; ch++ {
			clear := []byte{0xFE, 0xFE, 0x98, 0xE0, 0x1A, 0x00, bcdByte(ch / 100), bcdByte(ch % 100), 0xFF, 0xFD}

			read, err := p.BuildMemoryRead(civ.ChannelAddress{Channel: ch})
			if err != nil {
				t.Fatalf("BuildMemoryRead(%d): %v", ch, err)
			}
			set, err := p.BuildMemorySet(clearProbeRecord(ch))
			if err != nil {
				t.Fatalf("BuildMemorySet(%d): %v", ch, err)
			}
			for name, got := range map[string][]byte{
				"BuildMemoryRead":        read.Bytes(),
				"BuildMemorySet":         set.Bytes(),
				"BuildTransceiverIDRead": idRead.Bytes(),
			} {
				if string(got) == string(clear) {
					t.Fatalf("%s(channel %d) produced the CLEAR frame % X; this tier ships no clear builder", name, ch, clear)
				}
			}
			if p.AllowedCommand(clear) {
				t.Fatalf("AllowedCommand admitted the clear frame % X for channel %d; the gate admits only 19 00, a 1A 00 read and a re-validated 1A 00 set", clear, ch)
			}
		}
		// The whole-command form, PDF p.4's row 0B, likewise.
		if p.AllowedCommand([]byte{0xFE, 0xFE, 0x98, 0xE0, 0x0B, 0xFD}) {
			t.Error("AllowedCommand admitted command 0B \"Memory clear\"")
		}
	})
}

// checkEncoding binds B's encoding column to the member spans' civ encodings.
func checkEncoding(t *testing.T, key indexKey, encoding string, bind rowBinding, spans map[civ.FieldID]civ.FieldSpan) {
	t.Helper()
	switch encoding {
	case "bcd_packed":
		for _, id := range bind.fields {
			if got := spans[id].Encoding; got != civ.EncodingBCDNumber {
				t.Errorf("printed index %s: B calls it bcd_packed but %s uses %v", key, id, got)
			}
		}
	case "enum_byte":
		for _, id := range bind.fields {
			sp := spans[id]
			if sp.Encoding != civ.EncodingEnum || sp.Nibble != civ.NibbleWhole {
				t.Errorf("printed index %s: B calls it enum_byte (a WHOLE byte per code) but %s is %v on nibble %v", key, id, sp.Encoding, sp.Nibble)
			}
		}
	case "bitfield":
		if len(bind.fields) == 0 && len(bind.unmapped) == 0 {
			t.Errorf("printed index %s: B calls it bitfield, but the union declares neither a nibble-selected span nor an unmapped nibble", key)
		}
		for _, id := range bind.fields {
			sp := spans[id]
			if sp.Encoding != civ.EncodingEnum || sp.Nibble == civ.NibbleWhole {
				t.Errorf("printed index %s: B calls it bitfield, so %s must be an enum on a NIBBLE, not %v on %v", key, id, sp.Encoding, sp.Nibble)
			}
		}
	case "ascii":
		for _, id := range bind.fields {
			if got := spans[id].Encoding; got != civ.EncodingName {
				t.Errorf("printed index %s: B calls it ascii but %s uses %v", key, id, got)
			}
		}
	default:
		t.Errorf("printed index %s carries encoding %q, which this crosscheck does not understand - an unrecognised encoding is a failure, never a skip", key, encoding)
	}
}

// splitPrintedList splits a values_verbatim cell on its printed separator and
// trims nothing else: the cells are transcribed verbatim and a trimmed cell
// is a cell somebody edited.
func splitPrintedList(s, sep string) []string { return strings.Split(s, sep) }

// parseColonList decomposes one printed "<code>: <name>, <code>: <name>" list
// into the wire-value map it denotes. Strict: a malformed item is a Fatal.
func parseColonList(t *testing.T, s string) map[byte]string {
	t.Helper()
	out := map[byte]string{}
	for _, item := range strings.Split(s, ", ") {
		code, name, ok := strings.Cut(item, ": ")
		if !ok {
			t.Fatalf("printed value %q is not the <code>: <name> form", item)
		}
		n, err := strconv.Atoi(code)
		if err != nil {
			t.Fatalf("printed value %q carries a non-numeric code: %v", item, err)
		}
		out[byte(n)] = name
	}
	return out
}

// sameEnum reports whether two wire-value maps are equal.
func sameEnum(a, b map[byte]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// bcdByte packs a two-digit decimal number into one packed-BCD byte, which is
// how civ spells a channel selector.
func bcdByte(n int) byte { return byte(n/10<<4 | n%10) }

// clearProbeRecord is a minimal valid record used only to prove that the SET
// builder cannot produce the clear frame either. Its values are arbitrary and
// nothing else reads them.
func clearProbeRecord(ch int) civ.MemoryRecord {
	return civ.MemoryRecord{
		Address:      civ.ChannelAddress{Channel: ch},
		RXFreqHz:     civ.Available[uint64](14_250_000),
		Mode:         civ.Available("USB"),
		Filter:       civ.Available("FIL1"),
		ToneMode:     civ.Available("OFF"),
		ToneTXDeciHz: civ.Available[uint64](885),
		ToneRXDeciHz: civ.Available[uint64](1000),
		Name:         civ.Available("HOME QTH01"),
	}
}
