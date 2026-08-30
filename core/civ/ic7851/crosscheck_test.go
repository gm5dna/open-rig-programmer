// SPDX-License-Identifier: GPL-3.0-or-later

package ic7851_test

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7851"
)

// THIS FILE CONSUMES THE EVIDENCE; IT DOES NOT MERELY STORE IT.
//
// Three legs were transcribed independently from PDF p.263 (folio 18-14)
// and the pages it cross-refers to:
//
//	L  IC-7851-field-ledger.csv       printed INDEX and printed LABEL
//	W  IC-7851-geometry-witness.csv   MEASURED drawn-cell and nibble bounds
//	B  IC-7851-transcription-b.csv    printed WIDTH, ENCODING and VALUES
//
// Every row of all three is joined on (diagram_id, normalised field index)
// and every row must be consumed: a row no assertion reaches is a row that
// proves nothing, which is the defect this file was rewritten to fix.
//
// THE ARTEFACTS ARE FROZEN (freeze_test.go's SHA-256 manifest). When a
// join fails, the fix is arbitration against the PDF by the orchestrator —
// never an edit to an artefact, and never a widened assertion here.

// ---------------------------------------------------------------- readers

// row is one CSV row with its provenance and a consumption flag.
type row struct {
	leg    string
	num    int // 1-based data row, for the failure message
	fields map[string]string
	used   bool
}

// leg is one evidence file, read into rows keyed by its header.
type leg struct {
	name string
	file string
	rows []*row
}

func readLeg(t *testing.T, name, file string) *leg {
	t.Helper()
	f, err := os.Open("testdata/" + file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	recs, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	if len(recs) < 2 {
		t.Fatalf("%s: no data rows", file)
	}
	head := recs[0]
	l := &leg{name: name, file: file}
	for i, rec := range recs[1:] {
		if len(rec) != len(head) {
			t.Fatalf("%s row %d has %d columns, header has %d", file, i+1, len(rec), len(head))
		}
		m := make(map[string]string, len(head))
		for j, h := range head {
			m[h] = rec[j]
		}
		l.rows = append(l.rows, &row{leg: name, num: i + 1, fields: m, used: false})
	}
	return l
}

// requireAllConsumed is the "no row proves nothing" rule.
func (l *leg) requireAllConsumed(t *testing.T) {
	t.Helper()
	for _, r := range l.rows {
		if !r.used {
			t.Errorf("%s (%s) row %d was never joined to anything: %v — an unconsumed evidence row is a row that proves nothing", l.name, l.file, r.num, r.fields)
		}
	}
}

// ------------------------------------------------------------ index forms

// circledToDecimal rewrites ①…㉟ as decimal numerals.
//
// The three legs spell an index range three ways — L with a tilde over
// decimal numerals, W with an en dash over circled ones, B with a tilde
// over circled ones — which is L STOP 1 and W STOP 1's typographic half.
// Normalising here is what lets the join be on NUMBERS, and the join
// passing is the arbitration: the numerals themselves never disagree.
func circledToDecimal(s string) string {
	for i, r := range []rune("①②③④⑤⑥⑦⑧⑨⑩⑪⑫⑬⑭⑮⑯⑰⑱⑲⑳㉑㉒㉓㉔㉕㉖㉗㉘㉙㉚㉛㉜㉝㉞㉟") {
		s = strings.ReplaceAll(s, string(r), strconv.Itoa(i+1)+" ")
	}
	return s
}

// parseIndex normalises a printed field index to its inclusive numeric
// span. STRICT: an unrecognised spelling is an error, never a skip.
//
// Two printed forms exist and both are exact: a COMMA PAIR ("①, ②"),
// which must be consecutive, and a RANGE ("④~⑧", "④–⑧"), which must
// ascend. Anything else stops.
func parseIndex(s string) (lo, hi int, err error) {
	t := strings.TrimSpace(circledToDecimal(s))
	if t == "" {
		return 0, 0, fmt.Errorf("empty index")
	}
	var parts []string
	switch {
	case strings.Contains(t, ","):
		parts = strings.Split(t, ",")
	case strings.ContainsAny(t, "~–—-"):
		parts = strings.FieldsFunc(t, func(r rune) bool {
			return r == '~' || r == '–' || r == '—' || r == '-'
		})
	default:
		n, e := strconv.Atoi(strings.TrimSpace(t))
		if e != nil {
			return 0, 0, fmt.Errorf("index %q is neither a numeral, a comma pair nor a range", s)
		}
		return n, n, nil
	}
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("index %q has %d members, want 2", s, len(parts))
	}
	lo, err = strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("index %q: %v", s, err)
	}
	hi, err = strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, fmt.Errorf("index %q: %v", s, err)
	}
	if hi < lo {
		return 0, 0, fmt.Errorf("index %q descends", s)
	}
	if strings.Contains(t, ",") && hi != lo+1 {
		return 0, 0, fmt.Errorf("index %q is a comma pair of non-consecutive numerals", s)
	}
	return lo, hi, nil
}

// key is a joined row's identity across the three legs.
type key struct {
	diagram string
	lo, hi  int
}

func (k key) String() string { return fmt.Sprintf("%s %d..%d", k.diagram, k.lo, k.hi) }

// --------------------------------------------------------- STOP inventory

// arbitratedSTOPs is the decision register: every evidence row carrying a
// STOP marker, with the arbitration and its PDF anchor.
//
// A STOP IS A HALT, NOT A NOTE. The transcribers were instructed to stop
// rather than reconcile, so each one below is a decision taken ABOVE the
// artefacts, against the rendered page, and recorded here where the test
// that depends on it can be read beside it. TestSTOPsAreAllArbitrated
// fails on any STOP-bearing row this table does not name, so a re-cut
// artefact carrying a new STOP cannot pass unexamined.
var arbitratedSTOPs = map[string]string{
	// L STOP 1 — the band bracket prints an EN DASH between the numerals
	// and the explanation heading prints a TILDE. PDF p.263 (folio 18-14),
	// the ④–⑧ / ④~⑧ pair and the three like it.
	// ARBITRATION: typographic only. The NUMERALS are identical in all
	// three legs, the join below is on the numerals, and its passing is
	// the arbitration. No width, position or value depends on the glyph.
	"field-ledger STOP 1": "en dash versus tilde in an index range: the numerals agree, so the join is on numerals",
	// L STOP 2 — index ⑪ is printed twice, once as a D1 band cell and
	// again as D2's own label. PDF p.263 (folio 18-14), right column, the
	// "⑪ Data mode and tone type settings" sub-diagram.
	// ARBITRATION: D2 is an EXPANSION of the same byte, not a second one.
	// B says so in its own words ("the three D2 rows are not additive"),
	// and the width accounting below EXCLUDES D2 and then checks D2's own
	// halves sum to the D1 byte.
	"field-ledger STOP 2": "index 11 appears in D1 and again in D2: D2 expands the same byte and is excluded from the width sum",
	// W STOP 1 and W STOP 2 — the ④–⑧ and ⑱–㉗ brackets print five and
	// ten indices over three DRAWN cells each, because one of the three is
	// a printed ellipsis. PDF p.263, drawn cells 4–6 and 16–18.
	// ARBITRATION: the BYTE accounting comes from the PRINTED INDEX, which
	// L and B transcribed independently and agree on; W's drawn-cell
	// ordinals are used only to confirm that the compression happens in
	// exactly these two places and nowhere else.
	"geometry-witness STOP 1": "five printed indices over three drawn cells: byte widths come from the printed index, not from the drawn extent",
	"geometry-witness STOP 2": "ten printed indices over three drawn cells: same arbitration as STOP 1",
	// W STOP 3 — every position right of drawn cell 5 is a drawn-cell
	// ordinal and not a byte address, because nothing printed says how
	// many bytes an ellipsis cell abbreviates.
	// ARBITRATION: this test therefore NEVER reads a byte address out of
	// W's position columns. It uses W for two things only — that the
	// groups tile the band contiguously, and that exactly two of them are
	// ellipsis-compressed — and takes every byte address from the index.
	"geometry-witness STOP 3": "drawn-cell ordinals are not byte addresses: no byte offset in this test is read from W",
}

// -------------------------------------------------------------- the joins

// d1Group is one printed field group of the 1A 00 record, assembled from
// all three legs.
type d1Group struct {
	key        key
	label      string // L and B must agree
	widthBytes int    // B
	encoding   string // B
	values     string // B
	drawnFirst int    // W, a DRAWN CELL ORDINAL (W STOP 3)
	drawnCells int    // W
	byteLo     int    // derived from the printed index
	byteHi     int    // derived from the printed index
}

// loadD1 joins L, W and B into the record's printed field groups, marking
// every row it consumes.
func loadD1(t *testing.T, l, w, b *leg) []d1Group {
	t.Helper()

	labels := map[key]string{}
	order := []key{}
	for _, r := range l.rows {
		if r.fields["diagram_id"] != "D1" {
			continue
		}
		lo, hi, err := parseIndex(r.fields["field_index"])
		if err != nil {
			t.Fatalf("L row %d: %v", r.num, err)
		}
		k := key{"D1", lo, hi}
		if _, dup := labels[k]; dup {
			t.Fatalf("L row %d repeats group %s", r.num, k)
		}
		if r.fields["index_style"] != "circled" {
			t.Errorf("L row %d index_style = %q, want circled — every index on this page is a circled numeral", r.num, r.fields["index_style"])
		}
		if r.fields["pdf_page"] != "263" {
			t.Errorf("L row %d cites PDF p.%s; the 1A 00 record is on PDF p.263 (folio 18-14)", r.num, r.fields["pdf_page"])
		}
		labels[k] = r.fields["label_verbatim"]
		order = append(order, k)
		r.used = true
	}
	if len(order) != 8 {
		t.Fatalf("L gives %d D1 groups, want the page's eight", len(order))
	}

	geom := map[key]*row{}
	for _, r := range w.rows {
		if r.fields["diagram_id"] != "D1" {
			continue
		}
		lo, hi, err := parseIndex(r.fields["field_index"])
		if err != nil {
			t.Fatalf("W row %d: %v", r.num, err)
		}
		geom[key{"D1", lo, hi}] = r
		r.used = true
	}
	trans := map[key]*row{}
	for _, r := range b.rows {
		if r.fields["diagram_id"] != "D1" {
			continue
		}
		lo, hi, err := parseIndex(r.fields["field_index"])
		if err != nil {
			t.Fatalf("B row %d: %v", r.num, err)
		}
		trans[key{"D1", lo, hi}] = r
		r.used = true
	}
	if len(geom) != 8 || len(trans) != 8 {
		t.Fatalf("W gives %d D1 groups and B gives %d; L gives 8 — the three legs must describe the same field groups", len(geom), len(trans))
	}

	var out []d1Group
	for _, k := range order {
		wr, ok := geom[k]
		if !ok {
			t.Fatalf("W has no row for group %s", k)
		}
		br, ok := trans[k]
		if !ok {
			t.Fatalf("B has no row for group %s", k)
		}
		if got := br.fields["label_verbatim"]; got != labels[k] {
			t.Errorf("group %s: L prints the label %q and B prints %q — two independent transcriptions of one printed heading must agree", k, labels[k], got)
		}
		width, err := strconv.Atoi(br.fields["width_bytes"])
		if err != nil {
			t.Fatalf("B row %d width_bytes %q: %v", br.num, br.fields["width_bytes"], err)
		}
		if width != k.hi-k.lo+1 {
			t.Errorf("group %s: B gives %d bytes and the printed index spans %d numerals — on this page one index is one byte", k, width, k.hi-k.lo+1)
		}
		first, err := strconv.Atoi(wr.fields["first_byte"])
		if err != nil {
			t.Fatalf("W row %d first_byte: %v", wr.num, err)
		}
		last, err := strconv.Atoi(wr.fields["last_byte"])
		if err != nil {
			t.Fatalf("W row %d last_byte: %v", wr.num, err)
		}
		// WHOLE CELLS ONLY. Every D1 bracket starts at a cell's first
		// nibble and ends at its second; a group straddling half a cell
		// would be a different geometry entirely.
		if wr.fields["first_nibble"] != "1" || wr.fields["last_nibble"] != "2" {
			t.Errorf("group %s: W measures nibbles %s..%s, want a whole-cell extent 1..2", k, wr.fields["first_nibble"], wr.fields["last_nibble"])
		}
		out = append(out, d1Group{
			key: k, label: labels[k], widthBytes: width,
			encoding: br.fields["encoding"], values: br.fields["values_verbatim"],
			drawnFirst: first, drawnCells: last - first + 1,
		})
	}

	// THE BYTE ADDRESSES, DERIVED FROM THE PRINTED INDEX AND FROM NOTHING
	// ELSE (W STOP 3). The groups must tile 1..27 exactly: a gap or an
	// overlap in the printed index is a discontinuity the page does not
	// have.
	next := 1
	for i := range out {
		if out[i].key.lo != next {
			t.Fatalf("group %s starts at printed index %d, and the previous group ended at %d — the printed indices must tile the data area with no gap and no overlap", out[i].key, out[i].key.lo, next-1)
		}
		out[i].byteLo, out[i].byteHi = out[i].key.lo, out[i].key.hi
		next = out[i].key.hi + 1
	}
	if next-1 != ic7851.DataAreaLength {
		t.Fatalf("the printed indices run to %d and DataAreaLength is %d", next-1, ic7851.DataAreaLength)
	}
	return out
}

// ------------------------------------------------------------- the tests

// TestEvidenceLegsJoinAndAreFullyConsumed is F3's structural half: the
// three legs describe ONE record, every row of every leg is reached, and
// the arithmetic of the printed widths derives the two lengths this
// package ships.
func TestEvidenceLegsJoinAndAreFullyConsumed(t *testing.T) {
	l := readLeg(t, "field-ledger", "IC-7851-field-ledger.csv")
	w := readLeg(t, "geometry-witness", "IC-7851-geometry-witness.csv")
	b := readLeg(t, "transcription-b", "IC-7851-transcription-b.csv")

	groups := loadD1(t, l, w, b)

	// THE LENGTH DERIVATION, from B's own widths. The selector is the
	// first group; the record is everything after it.
	total, selector := 0, groups[0].widthBytes
	for _, g := range groups {
		total += g.widthBytes
	}
	if total != ic7851.DataAreaLength {
		t.Errorf("B's D1 widths sum to %d and DataAreaLength is %d", total, ic7851.DataAreaLength)
	}
	if selector != ic7851.AddressBytes {
		t.Errorf("B gives the channel selector %d bytes and AddressBytes is %d", selector, ic7851.AddressBytes)
	}
	if total-selector != ic7851.RecordOnlyLength {
		t.Errorf("the derivation is incoherent: %d - %d != %d", total, selector, ic7851.RecordOnlyLength)
	}

	// W'S CONTRIBUTION, WITHIN W STOP 3's LIMIT. The drawn cells must tile
	// the band contiguously, and exactly two groups may be
	// ellipsis-compressed — the two whose own notes say so.
	drawnNext, compressed := 1, 0
	for _, g := range groups {
		if g.drawnFirst != drawnNext {
			t.Errorf("group %s is drawn from cell %d and the previous group ended at cell %d — the band's cells must be contiguous", g.key, g.drawnFirst, drawnNext-1)
		}
		drawnNext = g.drawnFirst + g.drawnCells
		switch {
		case g.drawnCells == g.widthBytes:
			// One index, one drawn cell: the page's own convention.
		case g.drawnCells < g.widthBytes:
			compressed++
			if g.drawnCells != 3 {
				t.Errorf("group %s is drawn in %d cells; every compressed group on this page is drawn as cell, ellipsis, cell", g.key, g.drawnCells)
			}
		default:
			t.Errorf("group %s is drawn in %d cells for %d printed indices — more cells than indices is a geometry this page does not have", g.key, g.drawnCells, g.widthBytes)
		}
	}
	if compressed != 2 {
		t.Errorf("%d groups are ellipsis-compressed, want exactly 2 (④~⑧ and ⑱~㉗) — a third would move every byte address to its right", compressed)
	}
	if drawn := drawnNext - 1; drawn != 18 {
		t.Errorf("the band draws %d cells, want 18", drawn)
	}
	if got := ic7851.DataAreaLength - (drawnNext - 1); got != 9 {
		t.Errorf("the printed indices exceed the drawn cells by %d, want 9 = (5-3) + (10-3)", got)
	}

	// D2 — THE ⑪ SUB-DIAGRAM, AND L STOP 2's ARBITRATION IN CODE. One
	// labelled row plus two indexless nibble rows in each of W and B; the
	// three are NOT additive, and the two halves must sum to the D1 byte.
	checkD2(t, l, w, b, groups)

	l.requireAllConsumed(t)
	w.requireAllConsumed(t)
	b.requireAllConsumed(t)
}

// checkD2 consumes the sub-diagram rows and settles the crossed-leader
// reading from the artefacts rather than from this file's opinion.
func checkD2(t *testing.T, l, w, b *leg, groups []d1Group) {
	t.Helper()

	var d1Eleven *d1Group
	for i := range groups {
		if groups[i].key.lo == 11 && groups[i].key.hi == 11 {
			d1Eleven = &groups[i]
		}
	}
	if d1Eleven == nil {
		t.Fatal("D1 has no ⑪ group; the sub-diagram expands one")
	}

	for _, r := range l.rows {
		if r.fields["diagram_id"] != "D2" {
			continue
		}
		lo, hi, err := parseIndex(r.fields["field_index"])
		if err != nil || lo != 11 || hi != 11 {
			t.Fatalf("L's D2 row indexes %q, want the ⑪ it expands", r.fields["field_index"])
		}
		if r.fields["label_verbatim"] != "" {
			t.Errorf("L's D2 row prints the label %q; the sub-diagram prints none, and the heading above it is D1 ⑪'s", r.fields["label_verbatim"])
		}
		r.used = true
	}

	// The two legends, taken from B's own nibble rows. B records them in
	// NIBBLE order (left first) while the page PRINTS them in the other
	// order — the crossed-leader hazard, which both W and B recorded
	// independently at 400 dpi and more.
	var legends []string
	halves := 0.0
	for _, r := range b.rows {
		if r.fields["diagram_id"] != "D2" {
			continue
		}
		r.used = true
		if r.fields["field_index"] != "" {
			// The labelled row: same byte, same width as D1's ⑪.
			width, err := strconv.Atoi(r.fields["width_bytes"])
			if err != nil || width != d1Eleven.widthBytes {
				t.Errorf("B's labelled D2 row is %q bytes wide and D1's ⑪ is %d — the sub-diagram expands the same one byte", r.fields["width_bytes"], d1Eleven.widthBytes)
			}
			continue
		}
		v, err := strconv.ParseFloat(r.fields["width_bytes"], 64)
		if err != nil {
			t.Fatalf("B's D2 nibble row width %q: %v", r.fields["width_bytes"], err)
		}
		halves += v
		legends = append(legends, r.fields["values_verbatim"])
	}
	if halves != 1 {
		t.Errorf("B's two D2 nibble rows sum to %v bytes, want exactly 1 — they halve one byte and do not add to the D1 sum", halves)
	}
	if len(legends) != 2 {
		t.Fatalf("B gives %d nibble legends for ⑪, want 2", len(legends))
	}
	if !strings.Contains(legends[0], "DATA 1") {
		t.Errorf("B's FIRST (left, high) nibble legend is %q, want the DATA-mode one — the crossed leaders put the data mode in the high nibble", legends[0])
	}
	if !strings.Contains(legends[1], "TSQL") {
		t.Errorf("B's SECOND (right, low) nibble legend is %q, want the tone-type one", legends[1])
	}

	// W measured the same two nibbles. Nibble 1 is the left half of the
	// byte, nibble 2 the right, and each row's own note names which
	// printed line its leader lands on.
	seen := map[string]bool{}
	for _, r := range w.rows {
		if r.fields["diagram_id"] != "D2" {
			continue
		}
		r.used = true
		if r.fields["field_index"] != "" {
			continue
		}
		n := r.fields["first_nibble"]
		if n != r.fields["last_nibble"] {
			t.Errorf("W's D2 nibble row spans nibbles %s..%s; a nibble row is one nibble", n, r.fields["last_nibble"])
		}
		if seen[n] {
			t.Errorf("W measures nibble %s of the ⑪ byte twice", n)
		}
		seen[n] = true
		switch n {
		case "1":
			if !strings.Contains(r.fields["notes"], "LOWER") {
				t.Errorf("W's nibble 1 note does not record the LOWER printed line: %q", r.fields["notes"])
			}
		case "2":
			if !strings.Contains(r.fields["notes"], "UPPER") {
				t.Errorf("W's nibble 2 note does not record the UPPER printed line: %q", r.fields["notes"])
			}
		default:
			t.Errorf("W measures nibble %q of a two-nibble byte", n)
		}
	}
	if len(seen) != 2 {
		t.Errorf("W measured %d of the ⑪ byte's two nibbles", len(seen))
	}

	// AND THE PROFILE AGREES. The tone type is the RIGHT nibble, which is
	// the LOW one; the data mode is the left/high nibble and is
	// E6-UNMAPPED, so no span may claim it.
	layout := ic7851.Profile().Layouts()[0]
	off := d1Eleven.byteLo - ic7851.AddressBytes - 1
	if off != ic7851.DataModeNibbleOffset {
		t.Fatalf("printed ⑪ derives record offset %d and DataModeNibbleOffset is %d", off, ic7851.DataModeNibbleOffset)
	}
	found := false
	for _, sp := range layout.Fields {
		if sp.Offset != off {
			continue
		}
		found = true
		if sp.Field != civ.FieldToneMode {
			t.Errorf("record byte %d is mapped by %s; only the tone type may be mapped there", off, sp.Field)
		}
		if sp.Nibble != civ.NibbleLow {
			t.Errorf("the tone type is mapped to the %v nibble; B and W both put it in the RIGHT (low) half", sp.Nibble)
		}
	}
	if !found {
		t.Errorf("no span maps record byte %d's tone-type nibble", off)
	}
}

// TestSTOPsAreAllArbitrated makes every halt in the evidence visible.
//
// A STOP means the transcriber refused to reconcile something and handed
// it up. This test enumerates them from the artefacts themselves and
// requires each to appear in arbitratedSTOPs with its decision written
// down. A re-cut artefact carrying a STOP nobody has ruled on FAILS —
// which is the whole point of a permissive skip not being available here.
func TestSTOPsAreAllArbitrated(t *testing.T) {
	want := map[string]int{
		"field-ledger STOP 1":     4, // the four ranged groups
		"field-ledger STOP 2":     2, // D1 ⑪ and the D2 row that expands it
		"geometry-witness STOP 1": 1, // ④–⑧
		"geometry-witness STOP 2": 1, // ⑱–㉗
		"geometry-witness STOP 3": 6, // every row right of the first ellipsis
	}
	got := map[string]int{}
	for _, l := range []*leg{
		readLeg(t, "field-ledger", "IC-7851-field-ledger.csv"),
		readLeg(t, "geometry-witness", "IC-7851-geometry-witness.csv"),
		readLeg(t, "transcription-b", "IC-7851-transcription-b.csv"),
	} {
		for _, r := range l.rows {
			for _, n := range []string{"1", "2", "3", "4", "5"} {
				if strings.Contains(r.fields["notes"], "STOP "+n) {
					got[l.name+" STOP "+n]++
				}
			}
		}
	}
	for name, n := range got {
		if _, ruled := arbitratedSTOPs[name]; !ruled {
			t.Errorf("%s appears on %d evidence row(s) and no arbitration is recorded for it — this is an orchestrator STOP against PDF p.263, not something to assert around", name, n)
		}
		if want[name] != n {
			t.Errorf("%s appears on %d rows, want %d — a changed STOP count means the artefact was re-cut and the arbitration needs re-taking", name, n, want[name])
		}
	}
	for name := range want {
		if got[name] == 0 {
			t.Errorf("%s is arbitrated but appears on no evidence row — the ruling has gone stale", name)
		}
	}
}

// TestProfileMatchesTheEvidenceLegs is F3's substantive half: every span,
// width, offset, encoding, byte order, enum and fixed nibble in the
// shipped profile is DERIVED from the artefacts here and compared, rather
// than being restated in a second hand-written table.
func TestProfileMatchesTheEvidenceLegs(t *testing.T) {
	l := readLeg(t, "field-ledger", "IC-7851-field-ledger.csv")
	w := readLeg(t, "geometry-witness", "IC-7851-geometry-witness.csv")
	b := readLeg(t, "transcription-b", "IC-7851-transcription-b.csv")
	groups := loadD1(t, l, w, b)

	p := ic7851.Profile()
	layout := p.Layouts()[0]

	// Record offsets are 0-based from the start of the RECORD, so printed
	// index N sits at offset N - AddressBytes - 1.
	offsetOf := func(printed int) int { return printed - ic7851.AddressBytes - 1 }

	byOffset := map[int]civ.FieldSpan{}
	for _, sp := range layout.Fields {
		byOffset[sp.Offset] = sp
	}

	for _, g := range groups {
		switch g.key.lo {
		case 1: // ①,② — the channel selector, which is the ADDRESS field
			if g.encoding != "bcd_packed" {
				t.Errorf("%s: B gives encoding %q, want bcd_packed", g.key, g.encoding)
			}
			if p.AddressForm() != civ.AddressFormFlat {
				t.Errorf("the selector is one %d-byte number with no group byte, and the profile declares %v", g.widthBytes, p.AddressForm())
			}
			lo, hi := channelRangeFrom(t, g.values)
			if plo, phi := p.ChannelRange(); plo != lo || phi != hi {
				t.Errorf("B's printed selector legend gives channels %d..%d and the profile declares %d..%d", lo, hi, plo, phi)
			}

		case 3: // ③ — select memory, E6-UNMAPPED
			if g.encoding != "enum_byte" {
				t.Errorf("%s: B gives encoding %q, want enum_byte", g.key, g.encoding)
			}
			if n := len(parseEnum(t, g.values)); n != 4 {
				t.Errorf("%s: B prints %d select-memory values, want the four printed (OFF, ★1, ★2, ★3)", g.key, n)
			}
			off := offsetOf(g.key.lo)
			if off != ic7851.SelectNibbleOffset {
				t.Errorf("printed ③ derives record offset %d and SelectNibbleOffset is %d", off, ic7851.SelectNibbleOffset)
			}
			if sp, mapped := byOffset[off]; mapped {
				t.Errorf("record byte %d is mapped by %s: the select-memory marker has FOUR states and its neutral home is a bool, so ruling E6 leaves it unmapped rather than collapsing 4 to 2", off, sp.Field)
			}

		case 4: // ④~⑧ — the frequency, little-endian, with ⑧ fixed
			checkNumericGroup(t, g, layout, offsetOf, civ.FieldRXFrequency, civ.OrderLittleEndian, "1 Hz digit", ic7851.FreqFixedOffset)

		case 9: // ⑨,⑩ — mode and filter, one enum byte each
			if g.encoding != "enum_byte" {
				t.Errorf("%s: B gives encoding %q, want enum_byte", g.key, g.encoding)
			}
			modes, filters := splitModeAndFilter(t, g.values)
			checkEnumSpan(t, byOffset[offsetOf(9)], civ.FieldMode, modes, offsetOf(9))
			checkEnumSpan(t, byOffset[offsetOf(10)], civ.FieldFilter, filters, offsetOf(10))

		case 11: // ⑪ — settled by checkD2, which the other test consumes
			if g.encoding != "enum_nibble" {
				t.Errorf("%s: B gives encoding %q, want enum_nibble", g.key, g.encoding)
			}
			toneModes := parseEnum(t, strings.Split(g.values, "|")[1])
			checkEnumSpan(t, byOffset[offsetOf(11)], civ.FieldToneMode, toneModes, offsetOf(11))

		case 12: // ⑫~⑭ — the repeater tone, big-endian, with ⑫ fixed
			checkNumericGroup(t, g, layout, offsetOf, civ.FieldToneTX, civ.OrderBigEndian, "0.1 Hz digit", ic7851.ToneTXFixedOffset)

		case 15: // ⑮~⑰ — the tone squelch, transcribed independently
			checkNumericGroup(t, g, layout, offsetOf, civ.FieldToneRX, civ.OrderBigEndian, "0.1 Hz digit", ic7851.ToneRXFixedOffset)

		case 18: // ⑱~㉗ — the name
			if g.encoding != "ascii" {
				t.Errorf("%s: B gives encoding %q, want ascii", g.key, g.encoding)
			}
			sp, mapped := byOffset[offsetOf(18)]
			if !mapped || sp.Field != civ.FieldName {
				t.Fatalf("record byte %d does not carry the name span", offsetOf(18))
			}
			if sp.Length != g.widthBytes || p.NameLength() != g.widthBytes {
				t.Errorf("B gives the name %d bytes; the span is %d and NameLength() is %d", g.widthBytes, sp.Length, p.NameLength())
			}
			if sp.Encoding != civ.EncodingName {
				t.Errorf("the name span's encoding is %v, want EncodingName", sp.Encoding)
			}

		default:
			t.Fatalf("group %s is not one this test knows how to consume", g.key)
		}
	}

	// EVERY RECORD BYTE IS ACCOUNTED FOR, either by a mapped span or by
	// one of the five bytes this package names as unmapped: the
	// E6 select-memory byte and the three printed-fixed digit pads. A byte
	// that is neither would be a byte the profile writes blind.
	named := map[int]string{
		ic7851.SelectNibbleOffset: "the E6 select-memory marker",
		ic7851.FreqFixedOffset:    "printed ⑧, a fixed-zero digit pair",
		ic7851.ToneTXFixedOffset:  "printed ⑫, a fixed-zero digit pair",
		ic7851.ToneRXFixedOffset:  "printed ⑮, a fixed-zero digit pair",
	}
	covered := map[int]bool{}
	for _, sp := range layout.Fields {
		for i := 0; i < sp.Length; i++ {
			covered[sp.Offset+i] = true
		}
	}
	for i := 0; i < ic7851.RecordOnlyLength; i++ {
		if covered[i] {
			continue
		}
		if _, ok := named[i]; !ok {
			t.Errorf("record byte %d is neither mapped nor named as an unmapped region", i)
		}
		if layout.Fixed[i] != 0 {
			t.Errorf("unmapped record byte %d (%s) has template value %#02x, want 0", i, named[i], layout.Fixed[i])
		}
	}

	// The ⑪ sub-diagram is the profile's other evidence join — which
	// nibble of that byte the tone type occupies — and consuming its rows
	// here keeps this test's own all-rows-consumed claim true.
	checkD2(t, l, w, b, groups)

	l.requireAllConsumed(t)
	w.requireAllConsumed(t)
	b.requireAllConsumed(t)
}

// checkNumericGroup derives a packed-BCD group's byte order and its fixed
// byte from B's own printed digit leaders, then requires the profile's
// span to stop short of that byte.
//
// THIS IS WHERE THE EVIDENCE MEETS THE FIX. B prints one leader per
// NIBBLE, in the drawn left-to-right order, so leaders 2k and 2k+1 belong
// to the group's k-th byte. A byte whose BOTH leaders are printed "Fixed"
// carries no digit at all, and a span covering it would let the builder
// write one.
func checkNumericGroup(t *testing.T, g d1Group, layout civ.RecordLayout, offsetOf func(int) int, field civ.FieldID, order civ.ByteOrder, finestLeader string, wantFixedOffset int) {
	t.Helper()
	if g.encoding != "bcd_packed" {
		t.Errorf("%s: B gives encoding %q, want bcd_packed", g.key, g.encoding)
	}
	leaders := splitList(g.values)
	if len(leaders) != 2*g.widthBytes {
		t.Fatalf("%s: B prints %d digit leaders for %d bytes, want two per byte", g.key, len(leaders), g.widthBytes)
	}

	// THE FIXED BYTE, derived. Both nibbles must be fixed for the byte to
	// be one: a single fixed nibble beside a digit would still be a byte a
	// span has to cover.
	fixedBytes := []int{}
	for k := 0; k < g.widthBytes; k++ {
		a, b := isFixedLeader(leaders[2*k]), isFixedLeader(leaders[2*k+1])
		if a != b {
			t.Errorf("%s: byte %d has one fixed nibble and one digit (%q, %q) — this test's byte-level exclusion cannot express that", g.key, k, leaders[2*k], leaders[2*k+1])
		}
		if a && b {
			fixedBytes = append(fixedBytes, k)
		}
	}
	if len(fixedBytes) != 1 {
		t.Fatalf("%s: B prints %d wholly fixed bytes, want exactly 1", g.key, len(fixedBytes))
	}
	gotFixed := offsetOf(g.key.lo + fixedBytes[0])
	if gotFixed != wantFixedOffset {
		t.Errorf("%s: B's leaders put the fixed byte at record offset %d and this package names %d", g.key, gotFixed, wantFixedOffset)
	}

	// THE BYTE ORDER, derived: which end of the group carries the finest
	// printed digit.
	finest := -1
	for i, ldr := range leaders {
		if strings.Contains(ldr, finestLeader) {
			finest = i
		}
	}
	if finest < 0 {
		t.Fatalf("%s: B prints no %q leader", g.key, finestLeader)
	}
	wantOrder := civ.OrderBigEndian
	if finest < len(leaders)/2 {
		wantOrder = civ.OrderLittleEndian
	}
	if wantOrder != order {
		t.Fatalf("%s: B's leaders derive %v and this test expected %v", g.key, wantOrder, order)
	}

	// AND THE SPAN. It must carry the group's variable bytes and stop
	// short of the fixed one.
	var sp civ.FieldSpan
	for _, s := range layout.Fields {
		if s.Field == field {
			sp = s
		}
	}
	if sp.Field != field {
		t.Fatalf("the profile maps no %s span", field)
	}
	if sp.Encoding != civ.EncodingBCDNumber {
		t.Errorf("%s: the %s span's encoding is %v, want EncodingBCDNumber", g.key, field, sp.Encoding)
	}
	if sp.Order != order {
		t.Errorf("%s: the %s span's order is %v, want %v", g.key, field, sp.Order, order)
	}
	if sp.Length != g.widthBytes-1 {
		t.Errorf("%s: the %s span is %d bytes and the printed group is %d with one wholly fixed byte", g.key, field, sp.Length, g.widthBytes)
	}
	wantOffset := offsetOf(g.key.lo)
	if fixedBytes[0] == 0 {
		wantOffset++
	}
	if sp.Offset != wantOffset {
		t.Errorf("%s: the %s span starts at record offset %d, and the printed group's first VARIABLE byte is offset %d", g.key, field, sp.Offset, wantOffset)
	}
	if sp.Offset <= gotFixed && gotFixed < sp.Offset+sp.Length {
		t.Errorf("%s: the %s span covers the fixed byte at offset %d — civ.FieldSpan carries no numeric domain, so a covered fixed byte is one the builder will write a digit into", g.key, field, gotFixed)
	}
}

// checkEnumSpan compares one profile enum against the codes B transcribed.
//
// THE RADIX IS RULING OQ1's — HEXADECIMAL. The page prints every command,
// sub-command and address as a hexadecimal byte pair and this column is
// one of the same kind, so "12: PSK" is the wire byte 0x12. Reversing that
// ruling is the ORCHESTRATOR'S to do and not an editor's: it would change
// two wire bytes, and this comparison is where it would first show.
func checkEnumSpan(t *testing.T, sp civ.FieldSpan, field civ.FieldID, want map[byte]string, offset int) {
	t.Helper()
	if sp.Field != field || sp.Offset != offset {
		t.Fatalf("record byte %d carries %q, want %s", offset, sp.Field, field)
	}
	if sp.Encoding != civ.EncodingEnum {
		t.Errorf("%s's encoding is %v, want EncodingEnum", field, sp.Encoding)
	}
	if len(sp.Enum) != len(want) {
		t.Errorf("%s maps %d codes and B transcribes %d: %v vs %v", field, len(sp.Enum), len(want), sp.Enum, want)
	}
	for code, name := range want {
		got, ok := sp.Enum[code]
		if !ok {
			t.Errorf("%s does not map the printed code %#02x (%s)", field, code, name)
			continue
		}
		if got != name {
			t.Errorf("%s maps %#02x to %q and B transcribes %q", field, code, got, name)
		}
	}
}

// ------------------------------------------------------- artefact parsing

// splitList splits B's pipe-separated verbatim lists.
func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, "|") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// isFixedLeader reports whether a printed digit leader marks its nibble as
// a fixed zero. Both printed spellings are accepted and no other is:
// "1000 MHz digit: 0 (Fixed)" on the frequency strip and "Fixed digit: 0*"
// on the tone strip.
func isFixedLeader(s string) bool {
	return strings.Contains(s, "(Fixed)") || strings.HasPrefix(s, "Fixed digit")
}

// parseEnum reads "code: NAME" pairs out of one of B's value lists,
// accepting both the comma-separated nibble form and the pipe-separated
// byte form, and refusing anything else.
func parseEnum(t *testing.T, s string) map[byte]string {
	t.Helper()
	out := map[byte]string{}
	var items []string
	for _, p := range splitList(s) {
		items = append(items, strings.Split(p, ",")...)
	}
	for _, it := range items {
		it = strings.TrimSpace(it)
		if it == "" {
			continue
		}
		i := strings.Index(it, ":")
		if i < 0 {
			t.Fatalf("value %q is not a printed code:name pair", it)
		}
		code := strings.TrimSpace(it[:i])
		name := strings.TrimSpace(it[i+1:])
		v, err := strconv.ParseUint(code, 16, 8) // RULING OQ1: hexadecimal
		if err != nil {
			t.Fatalf("value %q: code %q is not a hexadecimal byte", it, code)
		}
		if prev, dup := out[byte(v)]; dup {
			t.Fatalf("code %#02x is printed twice, as %q and %q", v, prev, name)
		}
		out[byte(v)] = name
	}
	return out
}

// splitModeAndFilter divides ⑨,⑩'s single printed block at its second
// circled sub-heading. The block heads two tables — "① Operating mode" and
// "② Filter setting" — and printed index ⑨ maps to the first, ⑩ to the
// second, in the order the referenced diagram numbers them.
func splitModeAndFilter(t *testing.T, values string) (modes, filters map[byte]string) {
	t.Helper()
	parts := splitList(values)
	var cut int
	for i, p := range parts {
		if strings.Contains(p, "Filter setting") {
			cut = i
		}
	}
	if cut == 0 || !strings.Contains(parts[0], "Operating mode") {
		t.Fatalf("⑨,⑩'s printed block is not the two sub-headed tables this test expects: %q", values)
	}
	return parseEnum(t, strings.Join(parts[1:cut], "|")), parseEnum(t, strings.Join(parts[cut+1:], "|"))
}

// channelRangeFrom derives the selector's inclusive channel range from the
// three printed legend lines ("0001–0099: …", "0100: …", "0101: …").
//
// The legend is DECIMAL, and B's own note says why: the two bytes are
// drawn as four digit cells and the printed meanings are decimal ("0099 =
// channel 99", not 0x99). So the numbers are read base ten here even
// though the enum codes above are read base sixteen.
func channelRangeFrom(t *testing.T, values string) (lo, hi int) {
	t.Helper()
	lo, hi = -1, -1
	for _, part := range splitList(values) {
		i := strings.Index(part, ":")
		if i < 0 {
			t.Fatalf("selector legend %q is not a printed range:name line", part)
		}
		for _, n := range strings.FieldsFunc(part[:i], func(r rune) bool {
			return r == '–' || r == '—' || r == '~' || r == '-'
		}) {
			v, err := strconv.Atoi(strings.TrimSpace(n))
			if err != nil {
				t.Fatalf("selector legend %q: %v", part, err)
			}
			if lo < 0 || v < lo {
				lo = v
			}
			if v > hi {
				hi = v
			}
		}
	}
	if lo < 0 || hi < 0 {
		t.Fatalf("no selector range was read from %q", values)
	}
	return lo, hi
}
