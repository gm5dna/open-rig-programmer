// SPDX-License-Identifier: GPL-3.0-or-later

package ic9700

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
)

// THE CROSSCHECK: leg L's ledger, leg B's transcription and this
// package's profile are made to agree in public.
//
// Three independent readings of one printed page, and a profile built
// from them. Any two of the three can be wrong together and still look
// right; holding all three to each other is what makes the transcription
// checkable rather than merely careful.
//
// TWO TRAPS THIS FILE EXISTS TO SURVIVE.
//
//  1. THE THREE LEGS NUMBER THEIR DIAGRAMS DIFFERENTLY. Legs L and W both
//     use D1 for the 1A 00 memory-content strip, D2 for the ④ detail box,
//     D3 for the ⑬ box, D4 for the ⑭ box and D5 for the band-stacking
//     register on PDF p.16. Leg B uses D1 for the memory strip and D2 for
//     the BAND-STACKING REGISTER, and says so in its own §Diagrams: it
//     assigned no id to the three one-byte sub-diagrams and folded their
//     nibble content into the parent rows. A naive join on diagram_id
//     would compare the ④ detail box against the band-stacking register.
//     legBDiagramForLW carries the mapping, and it is ASSERTED below
//     rather than assumed.
//
//  2. THE THREE LEGS SPELL THE INDEX DIFFERENTLY. That is indexrange.go's
//     problem, tested on its own, and never a regexp buried in here.

// legBDiagramForLW maps leg L's and leg W's diagram ids onto leg B's.
var legBDiagramForLW = map[string]string{
	"D1": "D1", // the 1A 00 memory-content strip
	"D2": "",   // ④ detail box — folded into B's D1 row ④
	"D3": "",   // ⑬ detail box — folded into B's D1 row ⑬
	"D4": "",   // ⑭ detail box — folded into B's D1 row ⑭
	"D5": "D2", // the band-stacking register, PDF p.16
}

// legLFoldedInto names the D1 row of leg B that each of L's one-byte
// sub-diagrams was folded into. D2's own index is 3 rather than 4 —
// that is the printed caption defect, recorded and never repaired.
var legLFoldedInto = map[string]indexKey{
	"D2": {first: 4, last: 4},
	"D3": {first: 13, last: 13},
	"D4": {first: 14, last: 14},
}

// indexKey identifies one printed row by the index range it carries.
//
// THE FILLED FLAG IS LOAD-BEARING: {5,51} names BOTH the outlined primary
// range and the filled duplicated block, and a key without the flag would
// silently collapse them into one row.
type indexKey struct {
	first, last int
	filled      bool
}

func (k indexKey) width() int { return k.last - k.first + 1 }

func (k indexKey) String() string {
	s := fmt.Sprintf("%d~%d", k.first, k.last)
	if k.filled {
		s += " (filled)"
	}
	return s
}

func sortKeys(keys []indexKey) []indexKey {
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		switch {
		case a.first != b.first:
			return a.first < b.first
		case a.last != b.last:
			return a.last < b.last
		default:
			return !a.filled && b.filled
		}
	})
	return keys
}

// keyed is what indexKeys reads: both leg readers answer it, so the "the
// two index sets are EQUAL, not merely overlapping" assertion can be
// written once.
type keyed interface {
	keys(diagram string) []indexKey
}

func indexKeys(k keyed, diagram string) []indexKey { return k.keys(diagram) }

// readCSV parses one frozen artefact with encoding/csv — never a regexp
// over prose. The legs quote commas, embedded quotation marks and curly
// punctuation inside their prose columns, and a hand-rolled split would
// mis-read them silently.
func readCSV(t *testing.T, name string) (header []string, rows [][]string) {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	all, err := r.ReadAll()
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	if len(all) < 2 {
		t.Fatalf("%s has %d records; a leg with no rows is not evidence", name, len(all))
	}
	return all[0], all[1:]
}

// columns maps a header to column indices, failing loudly on a column the
// caller named and the file does not have.
func columns(t *testing.T, name string, header []string, want ...string) map[string]int {
	t.Helper()
	at := make(map[string]int, len(header))
	for i, h := range header {
		at[h] = i
	}
	out := make(map[string]int, len(want))
	for _, w := range want {
		i, ok := at[w]
		if !ok {
			t.Fatalf("%s has no %q column; its header is %v", name, w, header)
		}
		out[w] = i
	}
	return out
}

// ---------------------------------------------------------------- leg L

type ledgerRow struct {
	key   indexKey
	label string
	style string
}

type legend map[string]map[indexKey]ledgerRow

func (l legend) keys(diagram string) []indexKey {
	var out []indexKey
	for k := range l[diagram] {
		out = append(out, k)
	}
	return sortKeys(out)
}

// readLedger reads leg L, and CROSS-CHECKS the normaliser against L's own
// index_style column on every row. L spells its indices in plain ASCII
// and carries the outlined/filled distinction separately, so this is the
// one place the two statements of the same fact can be compared.
func readLedger(t *testing.T) legend {
	t.Helper()
	const name = "ic9700-field-ledger.csv"
	header, rows := readCSV(t, name)
	at := columns(t, name, header, "diagram_id", "field_index", "index_style", "label_verbatim")

	out := legend{}
	for i, r := range rows {
		first, last, filled, err := parseIndexRange(r[at["field_index"]])
		if err != nil {
			t.Fatalf("%s row %d: %v", name, i+2, err)
		}
		style := r[at["index_style"]]
		if want := style == "filled"; filled != want {
			t.Errorf("%s row %d (%q): the normaliser says filled=%v, L's own index_style column says %q",
				name, i+2, r[at["field_index"]], filled, style)
		}
		key := indexKey{first: first, last: last, filled: filled}
		diagram := r[at["diagram_id"]]
		if out[diagram] == nil {
			out[diagram] = map[indexKey]ledgerRow{}
		}
		if _, dup := out[diagram][key]; dup {
			t.Fatalf("%s row %d: %s/%v appears twice", name, i+2, diagram, key)
		}
		out[diagram][key] = ledgerRow{key: key, label: r[at["label_verbatim"]], style: style}
	}
	return out
}

// ---------------------------------------------------------------- leg B

type transRow struct {
	key        indexKey
	label      string
	widthBytes int
}

type transcription struct {
	rows  map[string]map[indexKey]transRow
	order map[string][]transRow
}

func (tr transcription) keys(diagram string) []indexKey {
	var out []indexKey
	for k := range tr.rows[diagram] {
		out = append(out, k)
	}
	return sortKeys(out)
}

// rowsInCSVOrder preserves the CSV's own row order, which on leg B is
// WIRE ORDER.
//
// THAT IS NOT THE SAME AS NUMERIC INDEX ORDER, and the difference is the
// whole accumulation. Sorted numerically the filled ❺ ~ [51] row (first
// index 5) would land between ④ and ⑤~⑨; in the CSV it sits where the
// wire puts it, between ㊹ ~ (51) and (52) ~ (67).
func (tr transcription) rowsInCSVOrder(diagram string) []transRow {
	return tr.order[diagram]
}

func (tr transcription) at(diagram string, k indexKey) (transRow, bool) {
	row, ok := tr.rows[diagram][k]
	return row, ok
}

func readTranscription(t *testing.T) transcription {
	t.Helper()
	const name = "ic9700-transcription-b.csv"
	header, rows := readCSV(t, name)
	at := columns(t, name, header, "diagram_id", "field_index", "label_verbatim", "width_bytes")

	out := transcription{
		rows:  map[string]map[indexKey]transRow{},
		order: map[string][]transRow{},
	}
	for i, r := range rows {
		first, last, filled, err := parseIndexRange(r[at["field_index"]])
		if err != nil {
			t.Fatalf("%s row %d: %v", name, i+2, err)
		}
		var width int
		if _, err := fmt.Sscanf(r[at["width_bytes"]], "%d", &width); err != nil {
			t.Fatalf("%s row %d: width_bytes %q: %v", name, i+2, r[at["width_bytes"]], err)
		}
		key := indexKey{first: first, last: last, filled: filled}
		row := transRow{key: key, label: r[at["label_verbatim"]], widthBytes: width}
		diagram := r[at["diagram_id"]]
		if out.rows[diagram] == nil {
			out.rows[diagram] = map[indexKey]transRow{}
		}
		if _, dup := out.rows[diagram][key]; dup {
			t.Fatalf("%s row %d: %s/%v appears twice", name, i+2, diagram, key)
		}
		out.rows[diagram][key] = row
		out.order[diagram] = append(out.order[diagram], row)
	}
	return out
}

// ------------------------------------------------- the profile's spans

// spanRef is one DECLARED (neutral field, record offset) pair. The span's
// width, nibble and encoding are NOT declared here: spanUnionFor looks the
// span up in the profile's own layout and fails if it is not there, so the
// profile remains the thing under test.
type spanRef struct {
	field  civ.FieldID
	offset int
}

// rowSpec is what one PRINTED row of leg B covers, in the profile's terms.
type rowSpec struct {
	spans    []spanRef
	unmapped int // DECLARED bytes of the row that no span covers
}

// spanUnion is the ORDERED set of profile spans a single printed row
// covers, plus the bytes it declares unmapped.
//
// LEG B GROUPS FIELDS THE PROFILE SPLITS, which is why a row cannot be
// compared against ONE FieldID: "⑩, ⑪" is one row of width 2 over TWO
// one-byte spans, "㉑ ~ ㉓" is one row of width 3 over a one-byte polarity
// span and a two-byte code span, "⑬" is one row of width 1 over TWO
// nibble spans of the SAME byte, and the filled block is one row of width
// 47 over ELEVEN spans plus twenty-six unmapped bytes.
type spanUnion struct {
	spans    []civ.FieldSpan
	unmapped int
}

func oneSpan(f civ.FieldID, off int) rowSpec {
	return rowSpec{spans: []spanRef{{f, off}}}
}

func twoSpans(f1 civ.FieldID, o1 int, f2 civ.FieldID, o2 int) rowSpec {
	return rowSpec{spans: []spanRef{{f1, o1}, {f2, o2}}}
}

// twoNibbles is two enum spans sharing ONE byte. Both are listed; the
// byte they share is counted ONCE when the coverage is totted up.
func twoNibbles(f1, f2 civ.FieldID, off int) rowSpec {
	return rowSpec{spans: []spanRef{{f1, off}, {f2, off}}}
}

// selectRow is ④: one nibble span (the low half, the select-memory tag)
// and one FIXED nibble (the high half, printed as a literal 0 with the
// leader "Fixed"). The byte is covered, so nothing is declared unmapped.
func selectRow() rowSpec { return oneSpan(civ.FieldSelect, 0) }

func unmappedRow(n int) rowSpec { return rowSpec{unmapped: n} }

// profileRowSpans keys leg B's OWN grouped ranges — not the profile's
// split ones — so every printed row matches something.
var profileRowSpans = map[indexKey]rowSpec{
	{first: 4, last: 4}:   selectRow(),
	{first: 5, last: 9}:   oneSpan(civ.FieldRXFrequency, 1),
	{first: 10, last: 11}: twoSpans(civ.FieldMode, 6, civ.FieldFilter, 7),
	{first: 12, last: 12}: oneSpan(civ.FieldDataMode, 8),
	{first: 13, last: 13}: twoNibbles(civ.FieldDuplex, civ.FieldToneMode, 9),
	{first: 14, last: 14}: unmappedRow(1),
	{first: 15, last: 17}: oneSpan(civ.FieldToneTX, 11),
	{first: 18, last: 20}: oneSpan(civ.FieldToneRX, 14),
	{first: 21, last: 23}: twoSpans(civ.FieldDTCSPolarity, 17, civ.FieldDTCSCode, 18),
	{first: 24, last: 24}: unmappedRow(1),
	{first: 25, last: 27}: oneSpan(civ.FieldOffset, 21),
	{first: 28, last: 35}: unmappedRow(8),
	{first: 36, last: 43}: unmappedRow(8),
	{first: 44, last: 51}: unmappedRow(8),
	{first: 52, last: 67}: oneSpan(civ.FieldName, 95),

	// The OUTLINED primary block, ⑤ ~ 51 taken as one range. It is not a
	// row of leg B's CSV — B prints its thirteen constituent groups
	// instead — but the duplicate test needs it as a unit.
	{first: 5, last: 51}: blockRow(0),
	// The FILLED duplicate, which IS a row of leg B's CSV: one bracket
	// over one long dotted region, 47 bytes wide.
	{first: 5, last: 51, filled: true}: blockRow(duplicateBlockShift),
}

// blockRow declares the eleven spans and twenty-six unmapped bytes of one
// 47-byte block. The primary block's first span is the RX frequency; the
// duplicate renames exactly that one field to the TX frequency and
// repeats every other id at +47.
func blockRow(shift int) rowSpec {
	head := civ.FieldRXFrequency
	if shift != 0 {
		head = civ.FieldTXFrequency
	}
	return rowSpec{
		spans: []spanRef{
			{head, 1 + shift},
			{civ.FieldMode, 6 + shift},
			{civ.FieldFilter, 7 + shift},
			{civ.FieldDataMode, 8 + shift},
			{civ.FieldDuplex, 9 + shift},
			{civ.FieldToneMode, 9 + shift},
			{civ.FieldToneTX, 11 + shift},
			{civ.FieldToneRX, 14 + shift},
			{civ.FieldDTCSPolarity, 17 + shift},
			{civ.FieldDTCSCode, 18 + shift},
			{civ.FieldOffset, 21 + shift},
		},
		// ⑭ (1) + ㉔ (1) + the three eight-byte call signs (24).
		unmapped: 1 + 1 + 8 + 8 + 8,
	}
}

// spanUnionFor resolves a printed row's declared span references against
// the PROFILE's own layout. A reference the profile does not carry is a
// failure here, which is what keeps the declaration honest.
func spanUnionFor(t *testing.T, p civ.Profile, key indexKey) spanUnion {
	t.Helper()
	spec, ok := profileRowSpans[key]
	if !ok {
		t.Fatalf("no profile row is declared for printed row %v", key)
	}
	layout, ok := p.LayoutFor(RecordLength)
	if !ok {
		t.Fatalf("the profile has no %d-byte layout", RecordLength)
	}
	out := spanUnion{unmapped: spec.unmapped}
	for _, ref := range spec.spans {
		found := false
		for _, sp := range layout.Fields {
			if sp.Field == ref.field && sp.Offset == ref.offset {
				out.spans = append(out.spans, sp)
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("printed row %v declares %s at record offset %d, and the profile has no such span",
				key, ref.field, ref.offset)
		}
	}
	return out
}

// spanFor returns the profile's single span for one field inside one
// printed row.
func spanFor(t *testing.T, p civ.Profile, field civ.FieldID, key indexKey) civ.FieldSpan {
	t.Helper()
	for _, sp := range spanUnionFor(t, p, key).spans {
		if sp.Field == field {
			return sp
		}
	}
	t.Fatalf("printed row %v carries no %s span", key, field)
	return civ.FieldSpan{}
}

// coveredBytes counts the DISTINCT record bytes a union's spans touch.
// Two nibble spans sharing one byte count that byte ONCE, which is the
// only way "spans + unmapped == the printed width" can hold for ⑬.
func coveredBytes(u spanUnion) map[int]bool {
	seen := map[int]bool{}
	for _, sp := range u.spans {
		for off := sp.Offset; off < sp.Offset+sp.Length; off++ {
			seen[off] = true
		}
	}
	return seen
}

// --------------------------------------------------------------- tests

func TestTheThreeLegsNumberTheirDiagramsDifferently(t *testing.T) {
	// Trap 1, asserted rather than assumed. If a later transcription
	// renumbers a diagram, this fails here instead of quietly comparing
	// the ④ detail box against the band-stacking register forty lines
	// further down.
	ledger := readLedger(t)
	trans := readTranscription(t)

	for lw, b := range legBDiagramForLW {
		if _, ok := ledger[lw]; !ok {
			t.Errorf("leg L has no diagram %s, which the alias map names", lw)
			continue
		}
		if b == "" {
			// A one-byte sub-diagram leg B gave no id to. Its content is
			// folded into B's D1 row of the same field.
			//
			// NOTE WHAT IS *NOT* ASSERTED HERE. "Leg B has no diagram
			// named D2" would be false and is exactly the trap: B DOES
			// have a D2, and it is the band-stacking register. The id is
			// shared; the diagram is not. Only the fold target below is a
			// real claim.
			key, named := legLFoldedInto[lw]
			if !named {
				t.Errorf("%s maps to no leg-B diagram and names no D1 row it folded into", lw)
				continue
			}
			if _, ok := trans.at("D1", key); !ok {
				t.Errorf("%s folds into leg B's D1 row %v, and B has no such row", lw, key)
			}
			continue
		}
		if _, ok := trans.rows[b]; !ok {
			t.Errorf("the alias map sends L's %s to B's %s, and B has no diagram %s", lw, b, b)
			continue
		}
		if got, want := indexKeys(trans, b), indexKeys(ledger, lw); !reflect.DeepEqual(got, want) {
			t.Errorf("L's %s and B's %s do not carry the same index set:\nledger %v\nB      %v", lw, b, want, got)
		}
	}

	// The band-stacking register is the alias map's whole point: prove
	// the two legs agree about it field by field, so that "D5 means D2"
	// is a checked claim.
	for _, key := range indexKeys(ledger, "D5") {
		l := ledger["D5"][key]
		b, ok := trans.at("D2", key)
		if !ok {
			t.Errorf("leg B's D2 has no row %v", key)
			continue
		}
		if l.label != b.label {
			t.Errorf("band-stacking register %v: label %q (ledger D5) vs %q (B D2)", key, l.label, b.label)
		}
	}
}

func TestLedgerAndTranscriptionAgreeOnEveryD1Field(t *testing.T) {
	ledger := readLedger(t)       // testdata/ic9700-field-ledger.csv
	trans := readTranscription(t) // testdata/ic9700-transcription-b.csv

	// Both directions: the D1 index sets must be equal, not merely
	// overlapping.
	if !reflect.DeepEqual(indexKeys(ledger, "D1"), indexKeys(trans, "D1")) {
		t.Fatalf("D1 index sets differ:\nledger %v\nB      %v",
			indexKeys(ledger, "D1"), indexKeys(trans, "D1"))
	}
	for _, key := range indexKeys(ledger, "D1") {
		l := ledger["D1"][key]
		b, _ := trans.at("D1", key)
		if l.label != b.label {
			t.Errorf("%v: label %q (ledger) vs %q (B)", key, l.label, b.label)
		}
		if want := key.last - key.first + 1; b.widthBytes != want {
			t.Errorf("%v: B says width %d, its own printed index range says %d",
				key, b.widthBytes, want)
		}
	}
}

func TestEveryPrintedRowIsCoveredBySpansOrDeclaredUnmapped(t *testing.T) {
	trans := readTranscription(t)
	p := Profile()

	// WIRE ORDER = leg B's CSV ROW ORDER, not numeric index order.
	rows := trans.rowsInCSVOrder("D1")
	if len(rows) != 18 {
		t.Fatalf("leg B's D1 has %d rows, want 18", len(rows))
	}

	pos := 1 // leg B's measured data-area position of printed index ①
	for _, row := range rows {
		key := row.key
		switch {
		case key.first == 1 && key.last == 1, key.first == 2 && key.last == 3:
			// ① band and ②,③ channel are the ADDRESS, not record content.
			if got := key.width(); got != row.widthBytes {
				t.Errorf("%v: width %d, printed range %d", key, row.widthBytes, got)
			}
			pos += row.widthBytes
			continue
		}
		u := spanUnionFor(t, p, key)
		recOff := pos - 1 - AddressBytes

		// (a) the union's spans run forward from where B's measurement
		//     says they start, and stay inside the printed row.
		prev := -1
		for _, sp := range u.spans {
			if sp.Offset < prev {
				t.Fatalf("%v: span %s at record offset %d precedes the previous span at %d",
					key, sp.Field, sp.Offset, prev)
			}
			prev = sp.Offset
			if sp.Offset < recOff || sp.Offset+sp.Length > recOff+row.widthBytes {
				t.Errorf("%v: span %s covers record offsets %d..%d, outside the row's %d..%d",
					key, sp.Field, sp.Offset, sp.Offset+sp.Length-1, recOff, recOff+row.widthBytes-1)
			}
		}
		if len(u.spans) > 0 && u.spans[0].Offset != recOff {
			t.Errorf("%v: first span at record offset %d, B's measurement implies %d",
				key, u.spans[0].Offset, recOff)
		}

		// (b) span bytes + declared unmapped bytes == the printed row
		//     width == B's stated width. Nibble spans sharing a byte
		//     count that byte ONCE.
		covered := len(coveredBytes(u))
		if got, want := covered+u.unmapped, row.widthBytes; got != want {
			t.Errorf("%v: spans cover %d + %d unmapped = %d, B says %d",
				key, covered, u.unmapped, got, want)
		}
		if got, want := row.widthBytes, key.width(); got != want {
			t.Errorf("%v: B width %d, its own printed index range %d", key, got, want)
		}
		pos += row.widthBytes
	}

	if got := pos - 1; got != DataAreaLength {
		t.Errorf("B's widths sum to %d, want %d (record %d + address %d)",
			got, DataAreaLength, RecordLength, AddressBytes)
	}
}

func TestTheFilledRowIsWalkedAtDataAreaPositionFiftyTwo(t *testing.T) {
	// "Printed order" is ambiguous exactly here, and the whole
	// accumulation depends on getting it right.
	trans := readTranscription(t)
	pos := 1
	for _, row := range trans.rowsInCSVOrder("D1") {
		if row.key.filled {
			if pos != 52 {
				t.Fatalf("the filled block is walked at position %d, want 52", pos)
			}
			if row.widthBytes != 47 {
				t.Fatalf("the filled block is %d bytes, want 47", row.widthBytes)
			}
			return
		}
		pos += row.widthBytes
	}
	t.Fatal("leg B's D1 has no filled row; the duplicated TX block has been lost")
}

func TestTheDuplicatedBlockRepeatsEverySpanAtPlusFortySeven(t *testing.T) {
	// The filled row's union is the primary block's union shifted by 47,
	// with the same unmapped runs. Asserting it here is what makes the
	// duplicate a checked claim rather than a comment.
	p := Profile()
	primary := spanUnionFor(t, p, indexKey{first: 5, last: 51})
	dup := spanUnionFor(t, p, indexKey{first: 5, last: 51, filled: true})
	if len(primary.spans) != len(dup.spans) {
		t.Fatalf("primary has %d spans, duplicate %d", len(primary.spans), len(dup.spans))
	}
	if primary.unmapped != dup.unmapped {
		t.Errorf("primary declares %d unmapped bytes, duplicate %d", primary.unmapped, dup.unmapped)
	}
	for i := range primary.spans {
		a, b := primary.spans[i], dup.spans[i]
		want := a.Field
		if b.Field == civ.FieldTXFrequency && a.Field == civ.FieldRXFrequency {
			want = civ.FieldTXFrequency // the ONE field the duplicate renames
		}
		if b.Field != want || b.Offset != a.Offset+duplicateBlockShift || b.Length != a.Length {
			t.Errorf("span %d: duplicate %s@%d+%d, want %s@%d+%d",
				i, b.Field, b.Offset, b.Length, want, a.Offset+duplicateBlockShift, a.Length)
		}
		if b.Nibble != a.Nibble || b.Encoding != a.Encoding || b.Order != a.Order || b.Scale != a.Scale {
			t.Errorf("span %d (%s): the duplicate decodes differently from its primary", i, a.Field)
		}
	}
	if got, want := len(coveredBytes(primary))+primary.unmapped, duplicateBlockShift; got != want {
		t.Errorf("the primary block covers %d bytes, want %d", got, want)
	}
}

func TestSourceIndexDefectIsPinnedNotRepaired(t *testing.T) {
	// matrix §3.16 A4; leg L STOP 1; leg W STOP 8. The one-byte detail
	// box that expands "④ Select memory setting" is captioned with a
	// circled THREE. Both legs recorded it as seen. A future
	// transcription that silently repairs it must fail here.
	ledger := readLedger(t)
	row, ok := ledger["D2"][indexKey{first: 3, last: 3}]
	if !ok {
		t.Fatal("the ledger's D2 row is no longer indexed 3 — the source defect has been repaired, which is a STOP, not a fix")
	}
	if row.label != "" {
		t.Errorf("the ledger's D2 row has acquired the label %q; the page prints none against that numeral", row.label)
	}
}

// fieldLabelFragment binds each civ.FieldID that profileRowSpans maps in
// D1 to a substring of leg B's OWN label_verbatim for the printed row it
// comes from — copied out of testdata/ic9700-transcription-b.csv, never
// invented. This is what turns "the FieldID per row is hand-written" from
// an unchecked fact into a checked one: a FieldID copied into the WRONG
// row's rowSpec now has to also match that row's PRINTED words.
//
// TWO ROWS SPLIT INTO A FieldID PAIR THE PAGE DOES NOT ITSELF NAME
// SEPARATELY: ⑩,⑪ prints only "Operating mode setting" for both
// civ.FieldMode and civ.FieldFilter, and ㉑~㉓ prints only "DTCS code
// setting" for both civ.FieldDTCSPolarity and civ.FieldDTCSCode. Both
// FieldIDs of each pair therefore share the one fragment the page
// actually prints. That is a real limit on what this test can catch
// inside those two rows — it cannot tell Mode from Filter, or Polarity
// from Code, by text alone — but it still catches either FieldID being
// copied into a DIFFERENT row, which is the failure mode profileRowSpans
// is hand-written enough to invite.
var fieldLabelFragment = map[civ.FieldID]string{
	civ.FieldSelect:       "Select memory",
	civ.FieldRXFrequency:  "Operating frequency",
	civ.FieldMode:         "Operating mode",
	civ.FieldFilter:       "Operating mode",
	civ.FieldDataMode:     "Data mode",
	civ.FieldDuplex:       "Duplex and Tone",
	civ.FieldToneMode:     "Duplex and Tone",
	civ.FieldToneTX:       "Repeater tone frequency",
	civ.FieldToneRX:       "Tone squelch frequency",
	civ.FieldDTCSPolarity: "DTCS code",
	civ.FieldDTCSCode:     "DTCS code",
	civ.FieldOffset:       "Duplex offset frequency",
	civ.FieldName:         "Memory name",
	// civ.FieldTXFrequency carries NO entry, deliberately. It appears only
	// in the outlined primary block (key {5,51}, not a leg-B CSV row — B
	// prints thirteen constituent rows instead, see profileRowSpans) and
	// the filled duplicate (key {5,51,filled:true}, a real CSV row whose
	// label_verbatim the page prints as NOTHING — STOP 2, see the block
	// comment above blockRow). Neither row offers text to bind against, so
	// the loop below skips both before any FieldID lookup is attempted.
}

// TestProfileRowSpansFieldIDsMatchTheirPrintedLabel walks every key in
// profileRowSpans and, where leg B's transcription has a printed label
// for that row, requires each mapped FieldID's fragment to appear inside
// it. A FieldID hand-copied into the wrong row's rowSpec — for example
// civ.FieldToneRX written where civ.FieldToneTX belongs, two same-length
// same-shape spans a byte-arithmetic check alone cannot tell apart —
// fails here because "Repeater tone frequency" is not a substring of
// "Tone squelch frequency setting".
func TestProfileRowSpansFieldIDsMatchTheirPrintedLabel(t *testing.T) {
	trans := readTranscription(t)

	var keys []indexKey
	for k := range profileRowSpans {
		keys = append(keys, k)
	}
	keys = sortKeys(keys)

	for _, key := range keys {
		spec := profileRowSpans[key]
		if len(spec.spans) == 0 {
			continue // an unmapped-only row (e.g. the call-sign groups) names no FieldID to bind
		}
		row, ok := trans.at("D1", key)
		if !ok {
			// The synthetic {5,51} (unfilled) key names the outlined
			// primary block as a unit for the duplicate test; B prints
			// its thirteen constituent rows instead of this one, so
			// there is no B row here to bind against.
			continue
		}
		if row.label == "" {
			// The filled ❺~[51] row: the page prints nothing as its
			// label (STOP 2), only the grey NOTE box's prose, which is
			// not the label_verbatim column.
			continue
		}
		for _, ref := range spec.spans {
			frag, ok := fieldLabelFragment[ref.field]
			if !ok {
				t.Errorf("%v: %s has no entry in fieldLabelFragment", key, ref.field)
				continue
			}
			if !strings.Contains(row.label, frag) {
				t.Errorf("%v: %s's expected fragment %q is not in B's printed label %q",
					key, ref.field, frag, row.label)
			}
		}
	}
}
