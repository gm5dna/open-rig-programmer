// SPDX-License-Identifier: GPL-3.0-or-later

package ic905_test

// WHAT THIS FILE BINDS, AND WHAT THAT IS WORTH.
//
// The tier's A-equals-B-equals-ledger shape, with the IC-905's legs
// named: A is this package's own authored layout (the analogue of
// core/cat/ftdx101/table2.csv, which is that package's generation
// source); B is testdata/ic905-transcription-b.csv; the ledger is
// testdata/ic905-field-ledger.csv. The join key is diagram_id plus
// field_index.
//
// INDEPENDENCE IS BETWEEN L, B AND THE MATRIX, which were derived blind
// of one another from renders of one PDF. The PROFILE is authored from
// them, so this test is NOT independent evidence that the profile is
// right. Its value is as a STANDING REGRESSION INVARIANT: any future
// change to a span, a width or a printed index must re-fire this
// comparison rather than rely on somebody remembering that it was once
// done by hand. core/cat/ftdx101/geometry_test.go says the same thing
// about its own witness, and for the same reason.
//
// A MISMATCH HERE IS A STOP for orchestrator arbitration against the
// PDF. The arbitration may correct the profile, transcription B or the
// ledger. It may never correct this test, and it may never edit an
// artefact to make a test pass.

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic905"
)

// recordOffsetFor converts a PRINTED index (1...68 on PDF p.19, folio
// 18) into a 0-based offset from the start of the RECORD.
//
// Two corrections, and they are the whole of it:
//
//   - THE FOUR ADDRESS BYTES COME OFF. Printed indices 1...4 are the
//     channel address, which spec Erratum 1's convention puts OUTSIDE
//     the record; with the 1-based printed origin that is a shift of
//     five. Printed index 5 is record offset 0.
//   - THE 65-BYTE LAYOUT SHIFTS EVERY INDEX PAST 10 BY ONE. The 10 GHz
//     frequency field takes a sixth byte that the memory-content diagram
//     prints NO index for (G's hazard (d), STOP 1), so every field after
//     it still carries its printed index while sitting one byte later
//     than the diagram draws it.
//
// It is the single place both facts are written down, and Task 8's
// geometry test consumes it rather than restating either.
func recordOffsetFor(printedIndex, freqBytes int) int {
	off := printedIndex - 5
	if freqBytes == 6 && printedIndex > 10 {
		off++
	}
	return off
}

// parseFieldIndex parses a printed index cell into its first and last
// printed index. It accepts BOTH spellings the artefacts use — L and W
// write ASCII ("1, 2", "6~10", "53~68"), B writes the circled numerals
// the page prints ("(1), (2)", "(6)~(10)", "53~68", there being no
// circled form above 50) — and it is STRICT: anything it does not fully
// consume is a Fatal.
//
// The strictness is the point. A permissive fallback is precisely how
// two genuinely different indices would be normalised into false
// agreement, which is the one failure mode a cross-check cannot survive.
func parseFieldIndex(t *testing.T, where, s string) (first, last int) {
	t.Helper()
	switch {
	case strings.Contains(s, "~"):
		parts := strings.SplitN(s, "~", 2)
		first = parsePrintedIndexTerm(t, where, strings.TrimSpace(parts[0]))
		last = parsePrintedIndexTerm(t, where, strings.TrimSpace(parts[1]))
		if last <= first {
			t.Fatalf("%s: field_index %q spans %d~%d, which does not ascend", where, s, first, last)
		}
	case strings.Contains(s, ","):
		parts := strings.SplitN(s, ",", 2)
		first = parsePrintedIndexTerm(t, where, strings.TrimSpace(parts[0]))
		last = parsePrintedIndexTerm(t, where, strings.TrimSpace(parts[1]))
		if last != first+1 {
			t.Fatalf("%s: field_index %q names %d and %d, which are not the consecutive pair the comma spelling means", where, s, first, last)
		}
	default:
		first = parsePrintedIndexTerm(t, where, strings.TrimSpace(s))
		last = first
	}
	if first < 1 || last > 68 {
		t.Fatalf("%s: field_index %q resolves to %d..%d, outside the printed range 1..68", where, s, first, last)
	}
	return first, last
}

// parsePrintedIndexTerm reads ONE printed index: either all ASCII digits
// or exactly one circled numeral. Everything else is a Fatal.
func parsePrintedIndexTerm(t *testing.T, where, s string) int {
	t.Helper()
	if s == "" {
		t.Fatalf("%s: empty printed-index term", where)
	}
	runes := []rune(s)
	if len(runes) == 1 {
		if n, ok := circledNumeral(runes[0]); ok {
			return n
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatalf("%s: printed-index term %q is neither a decimal number nor a single circled numeral", where, s)
	}
	if strconv.Itoa(n) != s {
		t.Fatalf("%s: printed-index term %q does not re-render as %d — an alternative spelling is refused rather than normalised", where, s, n)
	}
	return n
}

// circledNumeral decodes the three Unicode blocks the document's index
// glyphs land in: 1-20 at U+2460, 21-35 at U+3251, 36-50 at U+32B1.
// There is NO circled form above 50, which is why the artefacts spell
// 52, 53 and 68 as plain digits and record that they do.
func circledNumeral(r rune) (int, bool) {
	switch {
	case r >= '①' && r <= '⑳':
		return int(r-'①') + 1, true
	case r >= '㉑' && r <= '㉟':
		return int(r-'㉑') + 21, true
	case r >= '㊱' && r <= '㊿':
		return int(r-'㊱') + 36, true
	default:
		return 0, false
	}
}

// spanExpectation is one field's share of a PRINTED row: which neutral
// field it is, how many bytes it takes and which nibble of them.
//
// bytes == 0 means "whatever this layout gives the whole printed row",
// which exists for exactly one row — the operating frequency, five bytes
// in one layout and six in the other.
type spanExpectation struct {
	field  civ.FieldID
	bytes  int
	nibble civ.NibbleSel
}

// expectedRowBindings is the ORDERED SPAN-UNION table ruling R8
// requires, keyed by the printed row's normalised span ("first-last").
//
// WHY A UNION AND NOT ONE FIELD PER ROW. Three printed rows carry more
// than one neutral field, and the split inside them is evidenced
// ELSEWHERE than by any D1 row — so a rule of the form "the profile's
// span for that row's field" is not merely wrong, it is arithmetically
// unsatisfiable. All three are PRE-REGISTERED here, with the evidence
// for each split named, rather than discovered at run time and defined
// away:
//
//   - (11),(12) — B records ONE row of width 2 labelled "Operating mode
//     setting"; L and W record TWO (the page prints two bare circled
//     numerals over two cells). The split is evidenced by PDF p.17
//     (folio 16), "Operating mode", whose TWO columns are "(1)Operating
//     mode" and "(2)Filter setting".
//   - (14) — all three artefacts record ONE row of width 1, encoding
//     enum_nibble. The split is evidenced by the (14) breakout box's TWO
//     leaders, followed independently by L, W, B and the matrix.
//   - (22)~(24) — all three record ONE row of width 3 labelled "DTCS
//     code setting". The split is evidenced by PDF p.24 (folio 23),
//     "DTCS code and polarity setting", and matrix section 1b's
//     dtcs_code note says so outright: the p.19 label says "DTCS code
//     setting" but the three bytes it points at carry code AND polarity.
//
// A row in the artefacts with no entry here, no entry in the unmapped
// set and no entry in the address set is a Fatal, so a future artefact
// row cannot pass unexamined.
var expectedRowBindings = map[string][]spanExpectation{
	"6-10":  {{field: civ.FieldRXFrequency}},
	"11-12": {{field: civ.FieldMode, bytes: 1}, {field: civ.FieldFilter, bytes: 1}},
	"13-13": {{field: civ.FieldDataMode, bytes: 1}},
	"14-14": {{field: civ.FieldDuplex, bytes: 1, nibble: civ.NibbleHigh}, {field: civ.FieldToneMode, bytes: 1, nibble: civ.NibbleLow}},
	"16-18": {{field: civ.FieldToneTX, bytes: 3}},
	"19-21": {{field: civ.FieldToneRX, bytes: 3}},
	"22-24": {{field: civ.FieldDTCSPolarity, bytes: 1}, {field: civ.FieldDTCSCode, bytes: 2}},
	"26-28": {{field: civ.FieldOffset, bytes: 3}},
	"53-68": {{field: civ.FieldName, bytes: 16}},
}

// unmappedRowTemplate is the six printed rows with NO civ.FieldID and no
// codeplug.ChannelData home, each with the byte the layout's Fixed
// template must carry for it. BYTE (5) IS IN THIS SET, and its presence
// is the plan's CRITICAL fix: a MEM channel carrying SELECT star 1/2/3
// on byte (5) must be REFUSED by the driver, never silently rewritten to
// OFF, and that refusal compares against this template.
var unmappedRowTemplate = map[string]byte{
	"5-5":   0x00, // Split and Select memory setting
	"15-15": 0x00, // Digital squelch setting
	"25-25": 0x00, // DV digital code squelch
	"29-36": 0x20, // UR (Destination) call sign
	"37-44": 0x20, // R1 (Access repeater) call sign
	"45-52": 0x20, // R2 (Gateway/Link repeater) call sign
}

// addressRows are the four printed indices spec Erratum 1's convention
// puts OUTSIDE the record. They appear in L, W and B and in NO layout
// span; recordOffsetFor of printed index 1 is -4, so they fall under no
// offset rule at all. They are named explicitly here — neither mapped
// nor "unmapped within the record" — and the test asserts they are
// exactly the rows with printed indices 1...4.
var addressRows = map[string]bool{"1-2": true, "3-4": true}

// unmapped reports whether a printed row is one of the six the profile
// deliberately does not map. It reads the map's PRESENCE rather than its
// value, because three of the six template bytes are 0x00 and a
// value-based test would silently drop them.
func unmapped(key string) bool {
	_, ok := unmappedRowTemplate[key]
	return ok
}

func spanKey(first, last int) string { return fmt.Sprintf("%d-%d", first, last) }

// artefactRow is one row of a quarantine CSV, normalised.
type artefactRow struct {
	key         string
	first, last int
	label       string
	widthBytes  int
	indexStyle  string
	rawIndex    string
}

// loadArtefact reads one quarantined CSV and returns its D1 rows in file
// order, with the index column normalised.
func loadArtefact(t *testing.T, name, indexCol string) []artefactRow {
	t.Helper()
	path := filepath.Join(evidenceDir, name)
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer f.Close()

	recs, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	if len(recs) < 2 {
		t.Fatalf("%s has %d rows including its header — the artefact is empty", path, len(recs))
	}
	col := func(header string) int {
		i := slices.Index(recs[0], header)
		if i < 0 {
			t.Fatalf("%s has no %q column; its header is %v", path, header, recs[0])
		}
		return i
	}
	diagram, index, label := col("diagram_id"), col(indexCol), col("label_verbatim")
	width, style := -1, -1
	if slices.Contains(recs[0], "width_bytes") {
		width = col("width_bytes")
	}
	if slices.Contains(recs[0], "index_style") {
		style = col("index_style")
	}

	var out []artefactRow
	for _, rec := range recs[1:] {
		if rec[diagram] != "D1" {
			continue
		}
		where := fmt.Sprintf("%s row %q", name, rec[index])
		first, last := parseFieldIndex(t, where, rec[index])
		row := artefactRow{
			key: spanKey(first, last), first: first, last: last,
			label: rec[label], widthBytes: -1, rawIndex: rec[index],
		}
		if width >= 0 && rec[width] != "" {
			n, err := strconv.Atoi(rec[width])
			if err != nil {
				t.Fatalf("%s: width_bytes %q is not a number", where, rec[width])
			}
			row.widthBytes = n
		}
		if style >= 0 {
			row.indexStyle = rec[style]
		}
		out = append(out, row)
	}
	return out
}

// requireTiles asserts that a side's printed spans, sorted, cover
// 1...68 with no gap and no overlap.
func requireTiles(t *testing.T, side string, rows []artefactRow) {
	t.Helper()
	sorted := slices.Clone(rows)
	slices.SortFunc(sorted, func(a, b artefactRow) int { return a.first - b.first })
	next := 1
	for _, r := range sorted {
		if r.first != next {
			t.Fatalf("%s: printed span %s starts at %d, want %d — the sides must tile 1..68 with no gap and no overlap", side, r.rawIndex, r.first, next)
		}
		next = r.last + 1
	}
	if next != 69 {
		t.Fatalf("%s: printed spans end at %d, want 68", side, next-1)
	}
}

// profileSpan finds the layout's one span for a field, and Fatals if
// there is not exactly one.
func profileSpan(t *testing.T, lay civ.RecordLayout, field civ.FieldID) civ.FieldSpan {
	t.Helper()
	var found []civ.FieldSpan
	for _, sp := range lay.Fields {
		if sp.Field == field {
			found = append(found, sp)
		}
	}
	if len(found) != 1 {
		t.Fatalf("the %d-byte layout carries %d spans for %s, want exactly 1", lay.Length, len(found), field)
	}
	return found[0]
}

// crosscheckCounters is the non-vacuity proof: every count must be
// non-zero AND equal its expected total, so a test that quietly stopped
// consuming rows fails instead of passing in silence.
type crosscheckCounters struct {
	expectations, unmappedRanges, addressRows int
}

func TestCrosscheck_ProfileAgreesWithTranscriptionBAndTheLedger(t *testing.T) {
	// 1. The ledger, filtered to the memory-content diagram.
	ledger := loadArtefact(t, "ic905-field-ledger.csv", "field_index")
	if len(ledger) != 18 {
		t.Fatalf("the field ledger has %d D1 rows, want 18 — L and W record (11) and (12) as two rows because the page prints two bare circled numerals over two cells", len(ledger))
	}

	// 2. Every ledger index is a plain circled numeral. Nothing on this
	//    page is filled, reversed, bracketed, bold or underlined (matrix
	//    section 3.15(e)) — checked independently by L, W, B and the
	//    matrix. A styled index elsewhere in the tier is the 7300's
	//    trap; here its ABSENCE is a fact to pin, not to assume.
	for _, r := range ledger {
		if r.indexStyle != "circled" {
			t.Errorf("ledger row %s has index_style %q, want \"circled\"", r.rawIndex, r.indexStyle)
		}
	}

	// 3. Transcription B, same filter.
	transB := loadArtefact(t, "ic905-transcription-b.csv", "field_index")
	if len(transB) != 17 {
		t.Fatalf("transcription B has %d D1 rows, want 17 — B records (11), (12) as ONE legend entry", len(transB))
	}

	// 4 and 5. Both sides tile the printed record with no gap and no
	//    overlap. (Step 4's normalisation happened in loadArtefact,
	//    through the strict parser.)
	requireTiles(t, "the field ledger", ledger)
	requireTiles(t, "transcription B", transB)

	// B's own width_bytes must agree with B's own printed index, which
	// is the one internal consistency check the artefact can be held to
	// without reference to anything else.
	for _, r := range transB {
		if want := r.last - r.first + 1; r.widthBytes != want {
			t.Errorf("transcription B row %s has width_bytes %d, but its printed index spans %d bytes", r.rawIndex, r.widthBytes, want)
		}
	}

	// 6. Labels, exactly — no substring matching anywhere. Every ledger
	//    row a B row covers must carry B's label verbatim, which for the
	//    (11),(12) row means BOTH ledger rows must (they are the same
	//    string, and the page prints them from one legend entry).
	splitRows := 0
	for _, b := range transB {
		var covered []artefactRow
		for _, l := range ledger {
			if l.first >= b.first && l.last <= b.last {
				covered = append(covered, l)
			}
		}
		if len(covered) == 0 {
			t.Fatalf("transcription B row %s covers no ledger row", b.rawIndex)
		}
		if len(covered) > 1 {
			splitRows++
			if b.key != "11-12" {
				t.Fatalf("transcription B row %s covers %d ledger rows — (11),(12) is the ONE pre-registered place the two sides' row counts differ, and a second divergence is an arbitration, not a reconciliation", b.rawIndex, len(covered))
			}
		}
		for _, l := range covered {
			if l.label != b.label {
				t.Errorf("printed index %s: transcription B says label %q, the ledger row %s says %q", b.rawIndex, b.label, l.rawIndex, l.label)
			}
		}
	}
	if splitRows != 1 {
		t.Fatalf("%d printed rows split differently between the ledger and transcription B, want exactly 1 ((11),(12))", splitRows)
	}

	// 7 and 8. The profile (A), through the span-union table, in BOTH
	//    layouts. The 65-byte pass is the same rule with
	//    recordOffsetFor(..., 6), which shifts every index past 10 by
	//    one — G's hazard (d) and STOP 1.
	for _, freqBytes := range []int{5, 6} {
		length := ic905.RecordLengthShort + freqBytes - 5
		lay, ok := ic905.Profile().LayoutFor(length)
		if !ok {
			t.Fatalf("LayoutFor(%d) missing", length)
		}
		var n crosscheckCounters
		for _, b := range transB {
			checkRowAgainstLayout(t, b, lay, freqBytes, &n)
		}

		// 9. Non-vacuity, per layout.
		if n.expectations != 12 {
			t.Errorf("%d-byte layout: %d span expectations checked, want 12 (the twelve mapped FieldIDs)", length, n.expectations)
		}
		if n.unmappedRanges != 6 {
			t.Errorf("%d-byte layout: %d unmapped ranges checked, want 6 ((5), (15), (25) and the three call-sign blocks)", length, n.unmappedRanges)
		}
		if n.addressRows != 2 {
			t.Errorf("%d-byte layout: %d address rows skipped, want 2", length, n.addressRows)
		}
	}

	// 9, continued: the one-off counters.
	if len(ledger) != 18 || len(transB) != 17 {
		t.Errorf("consumed %d ledger rows and %d transcription B rows, want 18 and 17", len(ledger), len(transB))
	}
}

// checkRowAgainstLayout is steps 7 and 8's rule for one printed row: its
// declared expectations, laid end to end from
// recordOffsetFor(first, freqBytes), must EXACTLY TILE the row's printed
// byte span, and each must equal the profile's actual span for that
// FieldID — offset, length AND nibble.
func checkRowAgainstLayout(t *testing.T, b artefactRow, lay civ.RecordLayout, freqBytes int, n *crosscheckCounters) {
	t.Helper()
	start := recordOffsetFor(b.first, freqBytes)
	// The row ends where the NEXT printed index starts. Written that way
	// rather than as last+1 because the 65-byte layout's sixth frequency
	// byte carries NO printed index, so the frequency row's tiling ends
	// one byte past its own last printed index there and nowhere else.
	end := recordOffsetFor(b.last+1, freqBytes)

	switch {
	case addressRows[b.key]:
		if b.last > 4 {
			t.Fatalf("printed row %s is registered as an address row but reaches index %d — only 1..4 are the channel address", b.rawIndex, b.last)
		}
		n.addressRows++

	case unmapped(b.key):
		want := unmappedRowTemplate[b.key]
		for off := start; off < end; off++ {
			for _, sp := range lay.Fields {
				if off >= sp.Offset && off < sp.Offset+sp.Length {
					t.Errorf("printed row %s (%s) is unmapped, but the %d-byte layout maps %s over record offset %d", b.rawIndex, b.label, lay.Length, sp.Field, off)
				}
			}
			if lay.Fixed[off] != want {
				t.Errorf("printed row %s: the %d-byte layout's Fixed template carries %#02x at record offset %d, want %#02x — an unmapped byte with no stated value is a byte a mutated record could re-encode identically", b.rawIndex, lay.Length, lay.Fixed[off], off, want)
			}
		}
		n.unmappedRanges++

	default:
		want, ok := expectedRowBindings[b.key]
		if !ok {
			t.Fatalf("printed row %s (%s) has no entry in expectedRowBindings, no entry in unmappedRowTemplate and no entry in addressRows — an artefact row this test does not understand is a STOP, not a skip", b.rawIndex, b.label)
		}
		cursor := start
		for _, exp := range want {
			width := exp.bytes
			if width == 0 {
				width = end - start
			}
			sp := profileSpan(t, lay, exp.field)
			if sp.Offset != cursor || sp.Length != width || sp.Nibble != exp.nibble {
				t.Errorf("printed row %s (%s), %d-byte layout: %s is expected at record offset %d, %d byte(s), nibble %v; the profile has it at offset %d, %d byte(s), nibble %v",
					b.rawIndex, b.label, lay.Length, exp.field, cursor, width, exp.nibble, sp.Offset, sp.Length, sp.Nibble)
			}
			n.expectations++
			// A HIGH-nibble expectation does not finish its byte: the
			// LOW nibble of the same byte is the next expectation, and
			// it is what advances the cursor.
			if exp.nibble != civ.NibbleHigh {
				cursor += width
			}
		}
		if cursor != end {
			t.Errorf("printed row %s (%s), %d-byte layout: its expectations tile record offsets %d..%d, but the printed row spans %d..%d — the union must tile the row exactly, with no gap and no overlap", b.rawIndex, b.label, lay.Length, start, cursor-1, start, end-1)
		}
	}
}
