// SPDX-License-Identifier: GPL-3.0-or-later

package ic9700

import (
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
)

// LEG W DID NOT MEASURE BYTES, and every assertion in this file is
// written around that fact.
//
// Its first_byte/last_byte columns hold DRAWN-CELL ORDINALS counted from
// the diagram's own first cell — its own stated convention, repeated in
// its first row's notes — and it refused outright to reconcile drawn cells
// with printed indices. It counted 22 cells in the upper wrapped row, 16
// in the lower, 38 in all, and declined to publish a byte total. A test
// that read W's numbers as byte offsets would be asserting something the
// witness never claimed, and would pass or fail for reasons unrelated to
// the radio.
//
// What W CAN be held to is geometry: the ORDER its cells are drawn in, the
// WIDTH of every group it drew in full, and its own two totals. Those are
// bound to the profile here.

// dottedRunSTOPs are the STOP numbers leg W recorded against the five
// groups the strip draws as cell + dotted box + cell.
//
// STOP 2 — the index-versus-cell DIVERGENCE, which W tagged on FOURTEEN
// rows — and STOP 8 — the ④/③ caption defect, which is not a D1 row at
// all — are NOT elisions and are deliberately absent. STOP 6, the filled
// block drawn as ONE long dotted region, has its own case below.
//
// DERIVING THIS SET FROM "the notes mention a STOP" WOULD FAIL ON CORRECT
// DATA: it would flag ⑩, ⑪ — two plainly drawn cells — as an elided run,
// and the "3 cells" assertion would then fail on a witness that is right.
var dottedRunSTOPs = map[int]bool{1: true, 3: true, 4: true, 5: true, 7: true}

// stopPattern matches the `STOP <n>` tokens the witness writes in its
// notes column, including the compound form `STOP 2 | STOP 6`.
var stopPattern = regexp.MustCompile(`STOP\s+(\d+)`)

type witnessRow struct {
	fieldIndex string
	key        indexKey
	firstCell  int
	lastCell   int
	// stop is the FIRST STOP number in the row's notes that is not 2, or
	// 0 where the row carries only STOP 2 or none at all.
	stop int
	// mentionsSTOP2 records the divergence tag separately, because it
	// means nothing about elision and its COUNT is a guard against a
	// reader that silently matches nothing.
	mentionsSTOP2 bool
}

// readWitness reads leg W's D1 rows — the 1A 00 memory-content strip —
// in drawn-cell order.
func readWitness(t *testing.T) []witnessRow {
	t.Helper()
	const name = "ic9700-geometry-witness.csv"
	header, rows := readCSV(t, name)
	at := columns(t, name, header, "diagram_id", "field_index", "first_byte", "last_byte", "notes")

	var out []witnessRow
	for i, r := range rows {
		if r[at["diagram_id"]] != "D1" {
			continue
		}
		first, last, filled, err := parseIndexRange(r[at["field_index"]])
		if err != nil {
			t.Fatalf("%s row %d: %v", name, i+2, err)
		}
		firstCell, err := strconv.Atoi(r[at["first_byte"]])
		if err != nil {
			t.Fatalf("%s row %d: first_byte %q: %v", name, i+2, r[at["first_byte"]], err)
		}
		lastCell, err := strconv.Atoi(r[at["last_byte"]])
		if err != nil {
			t.Fatalf("%s row %d: last_byte %q: %v", name, i+2, r[at["last_byte"]], err)
		}
		row := witnessRow{
			fieldIndex: r[at["field_index"]],
			key:        indexKey{first: first, last: last, filled: filled},
			firstCell:  firstCell,
			lastCell:   lastCell,
		}
		for _, m := range stopPattern.FindAllStringSubmatch(r[at["notes"]], -1) {
			n, convErr := strconv.Atoi(m[1])
			if convErr != nil {
				t.Fatalf("%s row %d: STOP token %q: %v", name, i+2, m[1], convErr)
			}
			if n == 2 {
				row.mentionsSTOP2 = true
				continue
			}
			if row.stop == 0 {
				row.stop = n
			}
		}
		out = append(out, row)
	}
	if len(out) == 0 {
		t.Fatalf("%s has no D1 rows", name)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].firstCell < out[j].firstCell })
	return out
}

func printedRangesInCellOrder(w []witnessRow) []indexKey {
	out := make([]indexKey, 0, len(w))
	for _, row := range w {
		out = append(out, row.key)
	}
	return out
}

// printedRangesFromProfile derives the printed rows' WIRE ORDER from the
// profile itself, rather than restating a hand-written list.
//
// The mapped rows carry their own record offsets — the profile's spans say
// where each begins — and the unmapped rows fill the gaps between them, in
// printed-index order, at the widths their printed index ranges give. So
// the walk places every mapped row where the profile puts it and consumes
// an unmapped row wherever the profile leaves a hole. A row that did not
// fit its hole would leave the walk short of the record's end, which is a
// failure rather than a shrug.
//
// The three address bytes come first: ① is the band byte and ②,③ the
// channel pair, which is AddressFormBandChannel's whole address field and
// is not record content at all.
func printedRangesFromProfile(t *testing.T) []indexKey {
	t.Helper()
	p := Profile()

	startAt := map[int]indexKey{}
	var unmapped []indexKey
	for key, spec := range profileRowSpans {
		// The OUTLINED 5~51 block is a unit this package's duplicate test
		// needs; the strip never prints it as one row, so it takes no
		// place in the wire order.
		if key.first == 5 && key.last == 51 && !key.filled {
			continue
		}
		if len(spec.spans) == 0 {
			unmapped = append(unmapped, key)
			continue
		}
		off := spanUnionFor(t, p, key).spans[0].Offset
		if prev, dup := startAt[off]; dup {
			t.Fatalf("printed rows %v and %v both begin at record offset %d", prev, key, off)
		}
		startAt[off] = key
	}
	sortKeys(unmapped)

	out := []indexKey{{first: 1, last: 1}, {first: 2, last: 3}}
	pos, next := 0, 0
	for pos < RecordLength {
		if key, ok := startAt[pos]; ok {
			out = append(out, key)
			pos += key.width()
			continue
		}
		if next >= len(unmapped) {
			t.Fatalf("record offset %d is covered by no printed row, and every unmapped row is spent", pos)
		}
		key := unmapped[next]
		next++
		out = append(out, key)
		pos += key.width()
	}
	if pos != RecordLength {
		t.Fatalf("the printed rows overrun the record: they end at %d, want %d", pos, RecordLength)
	}
	if next != len(unmapped) {
		t.Fatalf("%d unmapped printed rows were never placed", len(unmapped)-next)
	}
	return out
}

// widthFromProfile is the byte width the PROFILE gives one printed index
// range: its spans' distinct bytes plus its declared unmapped bytes, or —
// for the two address rows, which are not record content — the widths the
// address form itself declares.
func widthFromProfile(t *testing.T, key indexKey) int {
	t.Helper()
	if AddressBytes != 3 {
		t.Fatalf("this model's address is %d bytes, and the ①/②,③ split below assumes 3", AddressBytes)
	}
	switch {
	case key.first == 1 && key.last == 1:
		return AddressBytes - 2 // ① the band byte
	case key.first == 2 && key.last == 3:
		return 2 // ②,③ the packed-BCD channel pair
	}
	u := spanUnionFor(t, Profile(), key)
	return len(coveredBytes(u)) + u.unmapped
}

func totalDrawnCells(w []witnessRow) int {
	n := 0
	for _, row := range w {
		n += row.lastCell - row.firstCell + 1
	}
	return n
}

func sumPrintedWidths(w []witnessRow) int {
	n := 0
	for _, row := range w {
		n += row.key.width()
	}
	return n
}

func TestGeometryWitnessBindsTheProfile(t *testing.T) {
	w := readWitness(t) // testdata/ic9700-geometry-witness.csv, D1 rows only

	// (a) ORDER. W's rows, in drawn-cell order, are the profile's field
	// groups in record order, with the three address bytes first.
	if got, want := printedRangesInCellOrder(w), printedRangesFromProfile(t); !reflect.DeepEqual(got, want) {
		t.Fatalf("field order differs:\nwitness %v\nprofile %v", got, want)
	}

	// (b) WIDTH. For every group W drew in full, drawn cells == printed
	// index width == the profile's byte width.
	for _, row := range w {
		printedWidth := row.key.width()
		cells := row.lastCell - row.firstCell + 1
		switch {
		case dottedRunSTOPs[row.stop]: // {1,3,4,5,7}
			if cells != 3 {
				t.Errorf("%s: dotted-box run drawn as %d cells, want 3 (cell + …… + cell)", row.fieldIndex, cells)
			}
		case row.stop == 6: // the filled block, one long dotted region
			if cells != 1 {
				t.Errorf("%s: the filled block is drawn as %d cells, want 1", row.fieldIndex, cells)
			}
		case cells != printedWidth:
			t.Errorf("%s: %d drawn cells for %d printed indices", row.fieldIndex, cells, printedWidth)
		}
		if got := widthFromProfile(t, row.key); got != printedWidth {
			t.Errorf("%s: profile width %d, printed index range %d", row.fieldIndex, got, printedWidth)
		}
	}

	// (c) TOTALS. The witness's own counts, and the sum of the printed
	// widths, are the numbers this model is easiest to confuse.
	if got, want := totalDrawnCells(w), 38; got != want {
		t.Errorf("drawn cells = %d, want %d (22 upper row + 16 lower)", got, want)
	}
	if got, want := sumPrintedWidths(w), DataAreaLength; got != want {
		t.Errorf("printed widths sum to %d, want %d", got, want)
	}
}

// TestGeometryWitnessSTOPSetsAreWhatTheWitnessWrote guards the reader
// itself. Every count below is stated in the plan from the real CSV, and a
// reader that silently matched nothing would satisfy the "3 or 1 cells"
// assertions above vacuously.
func TestGeometryWitnessSTOPSetsAreWhatTheWitnessWrote(t *testing.T) {
	w := readWitness(t)

	dotted, filled, divergence := 0, 0, 0
	for _, row := range w {
		if dottedRunSTOPs[row.stop] {
			dotted++
		}
		if row.stop == 6 {
			filled++
		}
		if row.mentionsSTOP2 {
			divergence++
		}
	}
	if want := 5; dotted != want {
		t.Errorf("%d rows carry a dotted-run STOP, want %d (⑤~⑨, ㉘~㉟, ㊱~㊸, ㊹~51, 52~67)", dotted, want)
	}
	if want := 1; filled != want {
		t.Errorf("%d rows carry STOP 6, want %d (the filled block alone)", filled, want)
	}
	// FOURTEEN, not thirteen: every row from ⑩, ⑪ through (52) ~ (67),
	// the filled row included, whose notes read "STOP 2 | STOP 6".
	if want := 14; divergence != want {
		t.Errorf("%d D1 rows mention STOP 2, want %d", divergence, want)
	}
	// And STOP 2 says nothing about elision: the rows it tags include
	// ⑩, ⑪, which is drawn as two plain cells.
	for _, row := range w {
		if row.mentionsSTOP2 && row.stop == 0 && row.lastCell-row.firstCell+1 != row.key.width() {
			t.Errorf("%s carries only STOP 2 yet its %d drawn cells differ from its %d printed indices",
				row.fieldIndex, row.lastCell-row.firstCell+1, row.key.width())
		}
	}
}

func TestGeometryWitnessFilledBlockIsTheDuplicate(t *testing.T) {
	// The ONLY filled/reversed pair in the strip is ❺ ~ ❺❶, and it is the
	// 47-byte duplicated TX block. If a later transcription loses the
	// glyph class, the block silently becomes a second copy of ⑤ ~ 51 at
	// the wrong offset — which is exactly the mistake the printed NOTE
	// invites.
	w := readWitness(t)
	filled := 0
	for _, row := range w {
		if !row.key.filled {
			continue
		}
		filled++
		if row.key.first != 5 || row.key.last != 51 {
			t.Errorf("filled group is %d~%d, want 5~51", row.key.first, row.key.last)
		}
	}
	if filled != 1 {
		t.Errorf("found %d filled groups, want exactly 1", filled)
	}
	sp := spanFor(t, Profile(), civ.FieldTXFrequency, indexKey{first: 5, last: 51, filled: true})
	if sp.Offset != 48 {
		t.Errorf("tx_frequency offset %d, want 48 (record offset 1 + the 47-byte primary block)", sp.Offset)
	}
}
