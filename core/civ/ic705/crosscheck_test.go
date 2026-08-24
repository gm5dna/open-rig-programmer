// SPDX-License-Identifier: GPL-3.0-or-later

package ic705_test

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic705"
)

// This file joins the three READING legs to each other and to the layout.
//
// L (field-ledger), B (transcription-b) and W (geometry-witness) are three
// agents who read one rendered page without sight of each other's work.
// Where they agree, the layout has three independent witnesses; where they
// disagree, that is a STOP for the orchestrator to arbitrate AGAINST THE
// PDF — never an edit to an artefact, and never an adjustment of the
// layout to match whichever leg is convenient.
//
// # Why the join needs a normaliser
//
// The three legs spell the same printed index three different ways, and
// none of them is wrong:
//
//	L: "1, 2"      "6~10"      "6~52"  (+ an index_style column)
//	B: "(1), (2)"  "(6)~(10)"  "●6 ~ ●52"
//	W: "①, ②"      "⑥~⑩"       "❻~52"   (and "㉙〜㊱", a LONGER wave dash)
//
// So the test canonicalises rather than picking a favourite. What must not
// be normalised away is the STYLE: the diagram prints the duplicated TX
// block's bracket with BLACK-FILLED numerals and every other bracket with
// outline ones, and filled 6..52 and outline 6..52 are different parts of
// the record. All three legs recorded the two styles as a STOP (L STOP
// 1/2, B STOP 2, W STOP 1); carrying the style through the canonical form
// is what turns that STOP into a checked fact.

const (
	witnessCSV    = "testdata/geometry-witness.csv"
	transcriptCSV = "testdata/transcription-b.csv"
	ledgerCSV     = "testdata/field-ledger.csv"
)

// style is whether a printed index numeral is an OUTLINE circle or a
// BLACK-FILLED one.
type style int

const (
	styleUnknown style = iota
	styleOutline
	styleFilled
)

func (s style) String() string {
	switch s {
	case styleOutline:
		return "outline"
	case styleFilled:
		return "filled"
	default:
		return "unknown"
	}
}

// indexKey is one printed index, with the style that distinguishes the
// duplicated block's ❻~❺❷ from the record's own ⑥~㊺.
type indexKey struct {
	Style style
	N     int
}

func (k indexKey) String() string { return fmt.Sprintf("%s %d", k.Style, k.N) }

// canonIndex turns a leg's printed field_index into (style, indices):
//
//	"①, ②"      -> (outline, [1 2])
//	"(6)~(10)"  -> (outline, [6 7 8 9 10])
//	"㉙〜㊱"     -> (outline, [29 … 36])   // the long wave dash
//	"●6 ~ ●52"  -> (filled,  [6 … 52])
//	"❻~52"      -> (filled,  [6 … 52])
//
// Rules, in order: map every circled numeral to its digits and mark
// outline; map every FILLED circled numeral and every literal '●' and mark
// filled; strip '(' ')' and spaces; fold '〜' to '~'; split on ','; expand
// 'a~b' inclusively.
//
// THE STYLE HINT IS THE FALLBACK, NOT THE ANSWER. A glyph in the string
// always wins. It is needed because two of the indices have no circled
// form in Unicode at all — the legs write ㊺~52 and 53~68 with bare digits
// — so a row of bare digits carries no style of its own. L is the leg that
// records the style as DATA (its index_style column), which is exactly why
// the coverage test below joins all three: L's column is the evidence for
// the style of the rows W and B can only write plainly.
func canonIndex(s, styleHint string) (style, []int) {
	st := styleUnknown
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 0x2460 && r <= 0x2473: // ①..⑳
			b.WriteString(strconv.Itoa(int(r-0x2460) + 1))
			st = markOutline(st)
		case r >= 0x3251 && r <= 0x325F: // ㉑..㉟
			b.WriteString(strconv.Itoa(int(r-0x3251) + 21))
			st = markOutline(st)
		case r >= 0x32B1 && r <= 0x32BF: // ㊱..㊿
			b.WriteString(strconv.Itoa(int(r-0x32B1) + 36))
			st = markOutline(st)
		case r >= 0x2776 && r <= 0x277F: // ❶..❿
			b.WriteString(strconv.Itoa(int(r-0x2776) + 1))
			st = styleFilled
		case r >= 0x24EB && r <= 0x24F4: // ⓫..⓴
			b.WriteString(strconv.Itoa(int(r-0x24EB) + 11))
			st = styleFilled
		case r == '●':
			st = styleFilled
		case r == '〜':
			b.WriteRune('~')
		case r == '(' || r == ')' || unicode.IsSpace(r):
			// Dropped. unicode.IsSpace rather than a literal ' ':
			// the legs transcribe from a Japanese-typeset page and the
			// separator in "●6 ~ ●52" is not always the ASCII space.
		default:
			b.WriteRune(r)
		}
	}
	if st == styleUnknown {
		switch styleHint {
		case "filled":
			st = styleFilled
		default:
			st = styleOutline
		}
	}

	var out []int
	for _, tok := range strings.Split(b.String(), ",") {
		if tok == "" {
			continue
		}
		lo, hi, isRange := strings.Cut(tok, "~")
		if !isRange {
			if n, err := strconv.Atoi(tok); err == nil {
				out = append(out, n)
			}
			continue
		}
		a, err1 := strconv.Atoi(lo)
		z, err2 := strconv.Atoi(hi)
		if err1 != nil || err2 != nil {
			continue
		}
		for n := a; n <= z; n++ {
			out = append(out, n)
		}
	}
	return st, out
}

// markOutline keeps a FILLED marking already seen: a string mixing the two
// would be a transcription this test must not silently downgrade.
func markOutline(cur style) style {
	if cur == styleFilled {
		return styleFilled
	}
	return styleOutline
}

func keysOf(st style, ns []int) []indexKey {
	out := make([]indexKey, 0, len(ns))
	for _, n := range ns {
		out = append(out, indexKey{Style: st, N: n})
	}
	return out
}

// --- the artefacts ---------------------------------------------------

type witnessRow struct {
	Diagram   string
	Index     string
	FirstByte int
	FirstNib  int
	LastByte  int
	LastNib   int
	Notes     string
	Style     style
	Indices   []int
}

type transcriptRow struct {
	Diagram string
	Index   string
	Label   string
	Width   int
	Style   style
	Indices []int
}

type ledgerRow struct {
	Diagram    string
	Index      string
	IndexStyle string
	Label      string
	Style      style
	Indices    []int
}

func readCSV(t *testing.T, path string) [][]string {
	t.Helper()
	f, err := os.Open(filepath.FromSlash(path))
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	recs, err := r.ReadAll()
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	if len(recs) < 2 {
		t.Fatalf("%s has %d rows — the artefact is empty or unreadable, and every join below would be vacuous", path, len(recs))
	}
	return recs[1:]
}

func atoi(t *testing.T, path string, row int, s string) int {
	t.Helper()
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		t.Fatalf("%s row %d: %q is not a number: %v", path, row, s, err)
	}
	return n
}

func loadWitness(t *testing.T) []witnessRow {
	t.Helper()
	var out []witnessRow
	for i, rec := range readCSV(t, witnessCSV) {
		st, idx := canonIndex(rec[1], "")
		out = append(out, witnessRow{
			Diagram:   rec[0],
			Index:     rec[1],
			FirstByte: atoi(t, witnessCSV, i, rec[3]),
			FirstNib:  atoi(t, witnessCSV, i, rec[4]),
			LastByte:  atoi(t, witnessCSV, i, rec[5]),
			LastNib:   atoi(t, witnessCSV, i, rec[6]),
			Notes:     rec[9],
			Style:     st,
			Indices:   idx,
		})
	}
	return out
}

func loadTranscript(t *testing.T) []transcriptRow {
	t.Helper()
	var out []transcriptRow
	for i, rec := range readCSV(t, transcriptCSV) {
		st, idx := canonIndex(rec[1], "")
		out = append(out, transcriptRow{
			Diagram: rec[0],
			Index:   rec[1],
			Label:   rec[2],
			Width:   atoi(t, transcriptCSV, i, rec[3]),
			Style:   st,
			Indices: idx,
		})
	}
	return out
}

func loadLedger(t *testing.T) []ledgerRow {
	t.Helper()
	var out []ledgerRow
	for _, rec := range readCSV(t, ledgerCSV) {
		st, idx := canonIndex(rec[1], rec[2])
		out = append(out, ledgerRow{
			Diagram:    rec[0],
			Index:      rec[1],
			IndexStyle: rec[2],
			Label:      rec[3],
			Style:      st,
			Indices:    idx,
		})
	}
	return out
}

// --- 1. the three legs cover the same indices -------------------------

func TestLegsCoverTheSameIndices(t *testing.T) {
	want := map[indexKey]bool{}
	for n := 1; n <= 68; n++ {
		want[indexKey{styleOutline, n}] = true
	}
	for n := 6; n <= 52; n++ {
		want[indexKey{styleFilled, n}] = true
	}

	legs := map[string]map[indexKey]bool{
		"L (field-ledger)":     {},
		"B (transcription-b)":  {},
		"W (geometry-witness)": {},
	}
	rows := 0
	for _, r := range loadLedger(t) {
		if r.Diagram != "D1" {
			continue
		}
		rows++
		for _, k := range keysOf(r.Style, r.Indices) {
			legs["L (field-ledger)"][k] = true
		}
	}
	for _, r := range loadTranscript(t) {
		if r.Diagram != "D1" {
			continue
		}
		rows++
		for _, k := range keysOf(r.Style, r.Indices) {
			legs["B (transcription-b)"][k] = true
		}
	}
	for _, r := range loadWitness(t) {
		if r.Diagram != "D1" {
			continue
		}
		rows++
		for _, k := range keysOf(r.Style, r.Indices) {
			legs["W (geometry-witness)"][k] = true
		}
	}
	if rows == 0 {
		t.Fatal("parsed zero D1 rows across all three legs — the loaders or their filters are broken and this test asserted nothing")
	}

	for leg, got := range legs {
		for k := range want {
			if !got[k] {
				t.Errorf("%s does not cover printed index %s — the three legs must name the same field set, and a gap is a STOP for arbitration against the PDF", leg, k)
			}
		}
		for k := range got {
			if !want[k] {
				t.Errorf("%s names printed index %s, which is outside outline 1..68 plus filled 6..52", leg, k)
			}
		}
	}
}

// --- 2. widths agree, joined to an ORDERED SPAN-UNION -----------------

// TestWidthsAgree joins B's widths to W's measurements.
//
// THE JOIN IS TO A SPAN-UNION, NOT TO A SINGLE ROW, and that is the whole
// point of this test's shape. The three legs do not share a row
// granularity: transcription B merges `(11), (12)` into ONE row of width 2
// ("Operating mode setting") while the witness and the ledger carry ⑪ and
// ⑫ as TWO rows of one byte each. A join that looked for "the W row with
// the same canonical index" would find nothing for that B row and assert
// nothing at all.
//
// So for every B row: collect the W rows whose canonical indices are a
// SUBSET of the B row's, require them to tile the B row's index set
// exactly, require their byte ranges to be contiguous and ordered by
// first_byte with no gap and no overlap, and require the measured total to
// equal both B's printed width_bytes and the count of printed indices. A B
// row whose union is EMPTY fails.
func TestWidthsAgree(t *testing.T) {
	// The only genuinely split row, verified against the artefacts
	// themselves rather than assumed: B merges ⑪ and ⑫, W and L split
	// them. `(1), (2)` and `(3), (4)` are ONE row each in all three legs
	// spanning two bytes, so they are NOT in this set.
	wantMultiRow := map[string]int{"(11), (12)": 2}

	wRows := loadWitness(t)
	var d1 []witnessRow
	for _, r := range wRows {
		if r.Diagram == "D1" {
			d1 = append(d1, r)
		}
	}

	checked, multiSeen := 0, 0
	for _, b := range loadTranscript(t) {
		if b.Diagram != "D1" {
			continue
		}
		bSet := map[indexKey]bool{}
		for _, k := range keysOf(b.Style, b.Indices) {
			bSet[k] = true
		}

		var matched []witnessRow
		for _, w := range d1 {
			if len(w.Indices) == 0 {
				continue
			}
			subset := true
			for _, k := range keysOf(w.Style, w.Indices) {
				if !bSet[k] {
					subset = false
					break
				}
			}
			if subset {
				matched = append(matched, w)
			}
		}
		if len(matched) == 0 {
			t.Errorf("B row %q (%s) matched NO witness row — the span-union is empty and this row asserted nothing", b.Index, b.Label)
			continue
		}
		sort.Slice(matched, func(i, j int) bool { return matched[i].FirstByte < matched[j].FirstByte })

		if want, ok := wantMultiRow[b.Index]; ok {
			multiSeen++
			if len(matched) != want {
				t.Errorf("B row %q is the known split row and should meet %d witness rows, but met %d — the named set must match the artefacts or this test asserts a fiction", b.Index, want, len(matched))
			}
		}

		// The union tiles B's index set exactly.
		union := map[indexKey]bool{}
		for _, w := range matched {
			for _, k := range keysOf(w.Style, w.Indices) {
				union[k] = true
			}
		}
		for k := range bSet {
			if !union[k] {
				t.Errorf("B row %q: printed index %s is in B but in no matched witness row", b.Index, k)
			}
		}

		// Contiguous, ordered, no gap, no overlap.
		total := 0
		for i, w := range matched {
			if w.LastByte < w.FirstByte {
				t.Errorf("witness row %q measures %d..%d — backwards", w.Index, w.FirstByte, w.LastByte)
				continue
			}
			if i > 0 && w.FirstByte != matched[i-1].LastByte+1 {
				t.Errorf("B row %q: witness rows %q (…%d) and %q (%d…) neither abut nor tile — a gap or an overlap inside one legend entry",
					b.Index, matched[i-1].Index, matched[i-1].LastByte, w.Index, w.FirstByte)
			}
			total += w.LastByte - w.FirstByte + 1
		}
		if total != b.Width {
			t.Errorf("B row %q: witness measures %d bytes but B prints width_bytes %d", b.Index, total, b.Width)
		}
		if total != len(b.Indices) {
			t.Errorf("B row %q: witness measures %d bytes but the row names %d printed indices", b.Index, total, len(b.Indices))
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("checked zero B rows — the loader or its filter is broken")
	}
	if multiSeen != len(wantMultiRow) {
		t.Errorf("saw %d of the %d named multi-row cases — a named case the artefacts no longer contain would reduce this test to nothing", multiSeen, len(wantMultiRow))
	}
}

// --- 3. the witness tiles the whole data area -------------------------

func TestWitnessTilesTheDataArea(t *testing.T) {
	var d1 []witnessRow
	for _, r := range loadWitness(t) {
		if r.Diagram == "D1" {
			d1 = append(d1, r)
		}
	}
	if len(d1) == 0 {
		t.Fatal("no D1 witness rows — this test asserted nothing")
	}
	sort.Slice(d1, func(i, j int) bool { return d1[i].FirstByte < d1[j].FirstByte })

	if d1[0].FirstByte != 1 {
		t.Errorf("the witness's first D1 row starts at data-area byte %d, want 1", d1[0].FirstByte)
	}
	for i := 1; i < len(d1); i++ {
		if d1[i].FirstByte != d1[i-1].LastByte+1 {
			t.Errorf("witness rows %q (…%d) and %q (%d…) leave a gap or overlap in the data area",
				d1[i-1].Index, d1[i-1].LastByte, d1[i].Index, d1[i].FirstByte)
		}
	}
	if last := d1[len(d1)-1].LastByte; last != 115 {
		t.Errorf("the witness's last D1 row ends at data-area byte %d, want 115 — 4 address bytes plus a 111-byte record", last)
	}
}

// --- 4. the layout matches the legs -----------------------------------

// spanExpectation is one hand-written claim about the layout: which
// neutral field sits at which RECORD offset, how wide it is, and which
// nibble it claims. An empty Field means the run is UNMAPPED.
//
// THE OFFSETS ARE LITERALS, read by hand off the witness's measurements
// and written out here. They are NOT computed from the profile: an encoder
// and a decoder sharing one wrong offset round-trip perfectly, so a test
// that derived its expectation from the thing under test would be green on
// a layout shifted wholesale by a byte.
type spanExpectation struct {
	Field  civ.FieldID
	Offset int
	Length int
	Nibble civ.NibbleSel
}

// layoutClaims maps each PRINTED index group, as the witness writes it, to
// what the layout must hold for the bytes that group measures. The comment
// on each line is transcription B's label_verbatim for the same group.
var layoutClaims = map[string][]spanExpectation{
	// "Memory group number" / "Memory channel numbers" — the four ADDRESS
	// bytes, which are not part of the record at all.
	"①, ②": nil,
	"③, ④": nil,

	// "Split and Select memory setting" — UNMAPPED, both nibbles (O-6).
	"⑤": {{Offset: 0, Length: 1}},

	// "Operating frequency setting"
	"⑥~⑩": {{Field: civ.FieldRXFrequency, Offset: 1, Length: 5}},
	// "Operating mode setting" — B merges these two; the second is the filter.
	"⑪": {{Field: civ.FieldMode, Offset: 6, Length: 1}},
	"⑫": {{Field: civ.FieldFilter, Offset: 7, Length: 1}},
	// "Data mode setting"
	"⑬": {{Field: civ.FieldDataMode, Offset: 8, Length: 1}},
	// "Duplex and Tone settings" — one byte, two nibble enums.
	"⑭": {
		{Field: civ.FieldDuplex, Offset: 9, Length: 1, Nibble: civ.NibbleHigh},
		{Field: civ.FieldToneMode, Offset: 9, Length: 1, Nibble: civ.NibbleLow},
	},
	// "Digital squelch setting" — UNMAPPED.
	"⑮": {{Offset: 10, Length: 1}},
	// "Repeater tone frequency setting" (twice, verbatim; O-5 assigns the roles)
	"⑯~⑱": {{Field: civ.FieldToneTX, Offset: 11, Length: 3}},
	"⑲~㉑": {{Field: civ.FieldToneRX, Offset: 14, Length: 3}},
	// "DTCS code setting" — B carries all three bytes as one entry; matrix
	// §1b splits polarity from code (ASSUMED, L-DTCS-POLARITY).
	"㉒~㉔": {
		{Field: civ.FieldDTCSPolarity, Offset: 17, Length: 1},
		{Field: civ.FieldDTCSCode, Offset: 18, Length: 2},
	},
	// "DV Digital code squelch setting" — UNMAPPED.
	"㉕": {{Offset: 20, Length: 1}},
	// "Duplex offset frequency setting"
	"㉖~㉘": {{Field: civ.FieldOffset, Offset: 21, Length: 3}},
	// The three 8-character DV call signs — UNMAPPED.
	"㉙〜㊱":  {{Offset: 24, Length: 8}},
	"㊲~㊹":  {{Offset: 32, Length: 8}},
	"㊺~52": {{Offset: 40, Length: 8}},

	// The duplicated TX block. No label is printed against it ANYWHERE on
	// the page; the NOTE panel alone says "The same data as ⑥ ~ 52 are
	// stored in ❻ ~ ❺❷." Its first five bytes are the TX FREQUENCY — a
	// different neutral field from the RX frequency it mirrors, because
	// with Split ON this is the frequency the radio transmits on.
	"❻~52": {
		{Field: civ.FieldTXFrequency, Offset: 48, Length: 5},
		{Field: civ.FieldMode, Offset: 53, Length: 1},
		{Field: civ.FieldFilter, Offset: 54, Length: 1},
		{Field: civ.FieldDataMode, Offset: 55, Length: 1},
		{Field: civ.FieldDuplex, Offset: 56, Length: 1, Nibble: civ.NibbleHigh},
		{Field: civ.FieldToneMode, Offset: 56, Length: 1, Nibble: civ.NibbleLow},
		{Offset: 57, Length: 1},
		{Field: civ.FieldToneTX, Offset: 58, Length: 3},
		{Field: civ.FieldToneRX, Offset: 61, Length: 3},
		{Field: civ.FieldDTCSPolarity, Offset: 64, Length: 1},
		{Field: civ.FieldDTCSCode, Offset: 65, Length: 2},
		{Offset: 67, Length: 1},
		{Field: civ.FieldOffset, Offset: 68, Length: 3},
		{Offset: 71, Length: 24},
	},

	// "Memory name setting (16 characters, fixed)" — printed 53~68,
	// MEASURED 100..115.
	"53~68": {{Field: civ.FieldName, Offset: 95, Length: 16}},
}

func TestLayoutMatchesTheLegs(t *testing.T) {
	layout, ok := ic705.Profile().LayoutFor(111)
	if !ok {
		t.Fatal("the profile has no 111-byte layout")
	}

	mapped, unmapped := 0, 0
	for _, w := range loadWitness(t) {
		if w.Diagram != "D1" {
			continue
		}
		claims, known := layoutClaims[w.Index]
		if !known {
			t.Errorf("witness row %q has no entry in layoutClaims — every measured row must be either mapped to a field or declared unmapped, or the layout has an unexamined region", w.Index)
			continue
		}
		// The hand-written offsets are bound to the WITNESS: the first
		// claim of every group must start where the witness measures it.
		if len(claims) > 0 && claims[0].Offset != w.FirstByte-5 {
			t.Errorf("layoutClaims[%q] starts at record offset %d, but the witness measures data-area byte %d = record offset %d — the hand-written table has drifted from the evidence",
				w.Index, claims[0].Offset, w.FirstByte, w.FirstByte-5)
		}
		for _, c := range claims {
			if c.Field == "" {
				unmapped++
				continue
			}
			mapped++
			if !hasSpan(layout, c) {
				t.Errorf("the layout has no %s span at record offset %d, length %d, %v — the witness measures that run under printed index %q",
					c.Field, c.Offset, c.Length, c.Nibble, w.Index)
			}
		}
	}
	if mapped == 0 || unmapped == 0 {
		t.Fatalf("checked %d mapped and %d unmapped claims — both must be non-zero or this test passed over an empty set", mapped, unmapped)
	}
}

func hasSpan(l civ.RecordLayout, c spanExpectation) bool {
	for _, sp := range l.Fields {
		if sp.Field == c.Field && sp.Offset == c.Offset && sp.Length == c.Length && sp.Nibble == c.Nibble {
			return true
		}
	}
	return false
}

// --- 5. the unmapped areas are exactly the recorded ones ---------------

// TestUnmappedAreasAreExactlyTheRecordedOnes pins O-2's inventory.
//
// # Reconciling this with matrix erratum 6
//
// Erratum 6 counts the areas no spec.Field claims as "68 bytes and one
// nibble" over the same document. This layout's set is 53 WHOLE bytes, and
// the difference is arithmetic rather than disagreement. civ permits a
// field to appear twice in one layout and requires the copies to agree, so
// SIXTEEN of the duplicated block's bytes — mode, filter, data mode, the
// duplex/tone byte, both three-byte tone fields, the DTCS polarity and
// code, and the offset — are claimed by SECOND spans and survive a write
// by construction. R6 additionally unmaps the ★n Select nibble, which
// pairs with erratum 6's loose Split nibble to make record offset 0 a
// whole byte. So 68 + 1 nibble + 1 nibble − 16 = 53.
func TestUnmappedAreasAreExactlyTheRecordedOnes(t *testing.T) {
	layout, ok := ic705.Profile().LayoutFor(111)
	if !ok {
		t.Fatal("the profile has no 111-byte layout")
	}

	want := map[int]bool{0: true, 10: true, 20: true, 57: true, 67: true}
	for i := 24; i <= 47; i++ {
		want[i] = true
	}
	for i := 71; i <= 94; i++ {
		want[i] = true
	}
	if len(want) != 53 {
		t.Fatalf("the expectation itself names %d offsets, want 53 — the test's own arithmetic is wrong", len(want))
	}

	high := make([]bool, 111)
	low := make([]bool, 111)
	for _, sp := range layout.Fields {
		for off := sp.Offset; off < sp.Offset+sp.Length; off++ {
			switch {
			case sp.Encoding == civ.EncodingEnum && sp.Nibble == civ.NibbleHigh:
				high[off] = true
			case sp.Encoding == civ.EncodingEnum && sp.Nibble == civ.NibbleLow:
				low[off] = true
			default:
				high[off], low[off] = true, true
			}
		}
	}

	got := map[int]bool{}
	for off := 0; off < 111; off++ {
		switch {
		case !high[off] && !low[off]:
			got[off] = true
		case high[off] != low[off]:
			t.Errorf("record offset %d has exactly one nibble mapped — this layout has no half-mapped byte, and a loose nibble is neither writable nor preservable", off)
		}
	}
	for off := range want {
		if !got[off] {
			t.Errorf("record offset %d should be UNMAPPED but a span claims it", off)
		}
	}
	for off := range got {
		if !want[off] {
			t.Errorf("record offset %d is unmapped but is not in O-2's recorded inventory — an unrecorded unmapped byte is a write this tier cannot account for", off)
		}
	}
	if len(got) != 53 {
		t.Errorf("the layout leaves %d unmapped bytes, want exactly 53", len(got))
	}
}

// --- 6. the nibble insets ----------------------------------------------

// TestNibbleInsetsMatchTheLayout checks the three one-byte legend insets
// the diagram draws separately: D2 over data-area byte 5, D3 over byte 14
// and D4 over byte 15. Each names a LEFT-nibble meaning and a
// RIGHT-nibble meaning, and the layout must claim exactly the nibbles it
// can carry — both of byte 14, and NEITHER of bytes 5 and 15.
func TestNibbleInsetsMatchTheLayout(t *testing.T) {
	layout, ok := ic705.Profile().LayoutFor(111)
	if !ok {
		t.Fatal("the profile has no 111-byte layout")
	}

	type insetWant struct {
		dataAreaByte int
		high, low    civ.FieldID // "" means the layout must claim nothing there
	}
	// D2 ⑤: Split (left) and ★n Select (right) — O-6 maps NEITHER.
	// D3 ⑭: Duplex (left) and Tone mode (right).
	// D4 ⑮: digital squelch (left) and a literal 0 the inset labels
	//        "Fixed" (right) — neither is a neutral field.
	wants := map[string]insetWant{
		"D2": {dataAreaByte: 5},
		"D3": {dataAreaByte: 14, high: civ.FieldDuplex, low: civ.FieldToneMode},
		"D4": {dataAreaByte: 15},
	}

	seen := 0
	for _, w := range loadWitness(t) {
		want, ok := wants[w.Diagram]
		if !ok {
			continue
		}
		seen++
		// The inset is drawn as its own one-byte box, so the witness
		// measures it at byte 1 of ITS OWN diagram; the note names the D1
		// byte it expands. Both nibbles must be present in the inset.
		if w.FirstNib != 1 || w.LastNib != 2 {
			t.Errorf("%s: the witness measures nibbles %d..%d, want 1..2 — an inset that is not a two-nibble box is not the legend this test reads", w.Diagram, w.FirstNib, w.LastNib)
		}
		off := want.dataAreaByte - 5
		gotHigh := spanAt(layout, off, civ.NibbleHigh)
		gotLow := spanAt(layout, off, civ.NibbleLow)
		if gotHigh != want.high {
			t.Errorf("%s (data-area byte %d = record offset %d): the HIGH nibble carries %q, want %q", w.Diagram, want.dataAreaByte, off, gotHigh, want.high)
		}
		if gotLow != want.low {
			t.Errorf("%s (data-area byte %d = record offset %d): the LOW nibble carries %q, want %q", w.Diagram, want.dataAreaByte, off, gotLow, want.low)
		}
	}
	if seen != len(wants) {
		t.Errorf("matched %d of the %d nibble insets — the witness no longer carries one this test names", seen, len(wants))
	}
}

// spanAt returns the field claiming one nibble of one record offset, or ""
// if none does.
func spanAt(l civ.RecordLayout, off int, half civ.NibbleSel) civ.FieldID {
	for _, sp := range l.Fields {
		if off < sp.Offset || off >= sp.Offset+sp.Length {
			continue
		}
		if sp.Encoding == civ.EncodingEnum && sp.Nibble != civ.NibbleWhole {
			if sp.Nibble == half {
				return sp.Field
			}
			continue
		}
		return sp.Field
	}
	return ""
}
