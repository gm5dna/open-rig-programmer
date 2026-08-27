// SPDX-License-Identifier: GPL-3.0-or-later

package ic7300_test

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7300"
)

// readCSV parses one frozen artefact into header-keyed rows.
//
// encoding/csv, NEVER a regexp over prose: every one of these files
// carries quoted cells containing commas, doubled quotes and the printed
// glyphs themselves, and a regexp that got it right today would get it
// wrong on the first artefact that quotes a comma inside a note.
func readCSV(t *testing.T, name string) []map[string]string {
	t.Helper()
	f, err := os.Open(filepath.Join(evidenceDir, name))
	if err != nil {
		t.Fatalf("opening %s: %v", name, err)
	}
	defer f.Close()
	recs, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("parsing %s: %v", name, err)
	}
	if len(recs) < 2 {
		t.Fatalf("%s has %d rows — a leg with no data rows would make every assertion below vacuous", name, len(recs))
	}
	head := recs[0]
	out := make([]map[string]string, 0, len(recs)-1)
	for _, rec := range recs[1:] {
		row := make(map[string]string, len(head))
		for i, h := range head {
			if i < len(rec) {
				row[h] = rec[i]
			}
		}
		out = append(out, row)
	}
	return out
}

// onlyD1 keeps the rows of one diagram. Scope is STATED and PINNED: the
// crosscheck speaks for D1, the memory-record strip, and for nothing else.
// D2 and D3 are the two nibble insets, which the geometry test binds.
func onlyD1(rows []map[string]string) []map[string]string {
	out := make([]map[string]string, 0, len(rows))
	for _, r := range rows {
		if r["diagram_id"] == "D1" {
			out = append(out, r)
		}
	}
	return out
}

// normaliseKey folds the two printings of one field index onto one join
// key, and folds NOTHING ELSE.
//
// Two differences, both recorded by the legs themselves. L's own notes
// record that the STRIP prints a dash where the LEGEND prints a tilde
// (L STOP 3) and that a raster cannot tell U+2013 from U+2212 from a drawn
// rule, so the two printings must be treated as ONE field and never as
// two; and the right-hand column's conditional list prints "①,②" with no
// space after the comma where the left-hand heading prints "①, ②".
//
// The GLYPHS are left alone. This model's three legs all spell their
// indices in circled and filled numerals, so no glyph-to-digit mapping is
// needed here — unlike the IC-7300MK2, whose ledger spells them in plain
// ASCII digits.
func normaliseKey(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case ' ', '\t':
			// dropped
		case '~', '-', '–', '—', '−':
			b.WriteRune('-')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// indexValue maps one printed index numeral to the number it draws.
//
// Both glyph classes, because hazard (d) is about the two of them being
// different drawings of the SAME numbers: circled (outlined) ①–⑳ and
// ㉑–㉟, and filled (white on a solid disc) ❶–❿ and ⓫–⓴.
func indexValue(r rune) (int, bool) {
	switch {
	case r >= '①' && r <= '⑳': // ① .. ⑳
		return int(r-'①') + 1, true
	case r >= '㉑' && r <= '㉟': // ㉑ .. ㉟
		return int(r-'㉑') + 21, true
	case r >= '❶' && r <= '❿': // ❶ .. ❿
		return int(r-'❶') + 1, true
	case r >= '⓫' && r <= '⓴': // ⓫ .. ⓴
		return int(r-'⓫') + 11, true
	}
	return 0, false
}

// indexRange is the numeric span a printed key covers: (4, 8) for ④–⑧,
// (3, 3) for ③, (4, 17) for ❹–⓱.
func indexRange(t *testing.T, key string) (lo, hi int) {
	t.Helper()
	var vals []int
	for _, r := range key {
		if v, ok := indexValue(r); ok {
			vals = append(vals, v)
		}
	}
	if len(vals) == 0 {
		t.Fatalf("key %q carries no index numeral this test can read", key)
	}
	lo, hi = vals[0], vals[0]
	for _, v := range vals {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	return lo, hi
}

// theEightRecordKeys is the record's own field order — every printed key
// EXCEPT ①, ②, which is the CHANNEL ADDRESS and not part of the record
// (spec Erratum 1). They are written in strip order, normalised.
var theEightRecordKeys = []string{
	"③", "④-⑧", "⑨,⑩", "⑪", "⑫-⑭", "⑮-⑰", "❹-⓱", "⑱-㉗",
}

// theChannelAddressKey is the ninth printed key, excluded from the record.
const theChannelAddressKey = "①,②"

// recordGroup is one printed row's ORDERED SPAN-UNION in the profile's
// layout: the half-open byte range [Lo, Hi] that row's spans occupy.
//
// The binding is UNION-TO-ROW, never span-to-row, because three printed
// rows cover more than one span: ⑨, ⑩ is two whole-byte spans, ⑪ is two
// NIBBLE spans over ONE byte, and ❹–⓱ is the whole duplicated TX block.
// The test asserts the BYTE width, which is what B records.
type recordGroup struct {
	key    string
	lo, hi int // inclusive record byte offsets
}

// theGroups is §E's partition: eight consecutive, non-overlapping unions
// covering every byte of the 39-byte record exactly once.
var theGroups = []recordGroup{
	{"③", 0, 0},
	{"④-⑧", 1, 5},
	{"⑨,⑩", 6, 7},
	{"⑪", 8, 8},
	{"⑫-⑭", 9, 11},
	{"⑮-⑰", 12, 14},
	{"❹-⓱", 15, 28},
	{"⑱-㉗", 29, 38},
}

// TestCrosscheckLedgerTranscriptionAndProfile holds the field ledger (L),
// the semantic transcription (B) and THE PROFILE ITSELF to each other,
// field for field.
//
// "A" here is the PROFILE. This package has no separate transcription A —
// the profile's layout IS the third leg, which is what makes this
// crosscheck bind the CODE rather than two CSVs to each other. A leg that
// disagrees with the other two is a STOP for the orchestrator to arbitrate
// against the PDF, and the fix goes to exactly ONE of the profile, the
// ledger, B or W — never to several at once, and never by editing a frozen
// artefact.
func TestCrosscheckLedgerTranscriptionAndProfile(t *testing.T) {
	ledger := onlyD1(readCSV(t, "ic7300-field-ledger.csv"))
	transcription := onlyD1(readCSV(t, "ic7300-transcription-b.csv"))
	witness := onlyD1(readCSV(t, "ic7300-geometry-witness.csv"))

	// ---- 1. Scope, stated and pinned. --------------------------------
	//
	// B carries three CONDITIONAL rows: the clear recipe printed in PDF
	// p.169's right column, "To clear the memory channel contents on
	// 1A 00". This tier ships no clear path, so those rows describe a
	// frame no builder here can name and no gate here admits, and they are
	// excluded from every key set below. The excluded set is pinned
	// EXACTLY: a fourth conditional row would be a fact about this record
	// that nothing in this package models, and is a STOP.
	type conditional struct {
		index string
		width int
	}
	wantConditional := []conditional{{"①,②", 2}, {"③", 1}, {"④", 0}}
	var gotConditional []conditional
	var bRecord []map[string]string
	for _, r := range transcription {
		if r["encoding"] == "conditional" {
			w, err := strconv.Atoi(r["width_bytes"])
			if err != nil {
				t.Fatalf("B conditional row %q has width_bytes %q, which is not a number: %v", r["field_index"], r["width_bytes"], err)
			}
			gotConditional = append(gotConditional, conditional{r["field_index"], w})
			continue
		}
		bRecord = append(bRecord, r)
	}
	if len(gotConditional) != len(wantConditional) {
		t.Fatalf("B has %d conditional D1 rows %v, want exactly %d %v — a fourth conditional row is a STOP: it would be a documented alternative form this package neither builds nor admits",
			len(gotConditional), gotConditional, len(wantConditional), wantConditional)
	}
	for i, w := range wantConditional {
		if gotConditional[i] != w {
			t.Errorf("B conditional row %d = %v, want %v — the clear recipe is ①,②: memory channel (2 bytes), ③: \"FF\" (1 byte), ④: None (0 bytes)", i, gotConditional[i], w)
		}
	}

	// ---- 2. Key sets equal, both directions. -------------------------
	//
	// Normalisation folds the strip's dash onto the legend's tilde and
	// drops the space after the comma, and does nothing else. Each leg
	// must yield EXACTLY NINE DISTINCT keys: a normaliser that collapsed
	// two real fields into one would show up here as eight.
	keysOf := func(leg string, rows []map[string]string) map[string]bool {
		t.Helper()
		out := make(map[string]bool, len(rows))
		for _, r := range rows {
			k := normaliseKey(r["field_index"])
			if out[k] {
				t.Errorf("%s: normalised key %q appears twice — the normalisation has collapsed two printed fields into one", leg, k)
			}
			out[k] = true
		}
		if len(out) != 9 {
			t.Errorf("%s: normalisation yields %d distinct keys, want 9 (the eight record fields plus the channel address ①, ②)", leg, len(out))
		}
		return out
	}
	lKeys := keysOf("L (field ledger)", ledger)
	bKeys := keysOf("B (transcription)", bRecord)
	wKeys := keysOf("W (geometry witness)", witness)
	for _, pair := range []struct {
		a, b     string
		ka, kb   map[string]bool
		wantSame string
	}{
		{"L", "B", lKeys, bKeys, ""},
		{"B", "W", bKeys, wKeys, ""},
		{"W", "L", wKeys, lKeys, ""},
	} {
		for k := range pair.ka {
			if !pair.kb[k] {
				t.Errorf("key %q is in %s but not in %s — the three legs must name the same nine fields", k, pair.a, pair.b)
			}
		}
		for k := range pair.kb {
			if !pair.ka[k] {
				t.Errorf("key %q is in %s but not in %s — the three legs must name the same nine fields", k, pair.b, pair.a)
			}
		}
	}

	// ---- 3. Labels agree. --------------------------------------------
	//
	// One exception, and it is EVIDENCE rather than a gap: no keyed
	// heading is printed for the duplicated block on this model's page, so
	// L and B must BOTH be empty there.
	lLabel := map[string]string{}
	for _, r := range ledger {
		lLabel[normaliseKey(r["field_index"])] = r["label_verbatim"]
	}
	bLabel := map[string]string{}
	for _, r := range bRecord {
		bLabel[normaliseKey(r["field_index"])] = r["label_verbatim"]
	}
	for k, want := range lLabel {
		got, ok := bLabel[k]
		if !ok {
			continue // already reported by assertion 2
		}
		if k == "❹-⓱" {
			if want != "" || got != "" {
				t.Errorf("key ❹-⓱: L label %q, B label %q — BOTH must be empty. No keyed legend is printed for the duplicated block on PDF p.169, and an empty label there is the evidence, not a gap", want, got)
			}
			continue
		}
		if got != want {
			t.Errorf("key %q: L label_verbatim %q, B label_verbatim %q — the two legs transcribed the same printed heading differently", k, want, got)
		}
	}

	// ---- 4. Widths agree with the two lengths. -----------------------
	//
	// Spec Erratum 1: the profile carries the RECORD-ONLY figure. The two
	// sums differ by exactly the channel address, and both are asserted so
	// that a later reader cannot mistake one boundary for the other.
	bWidth := map[string]int{}
	for _, r := range bRecord {
		w, err := strconv.Atoi(r["width_bytes"])
		if err != nil {
			t.Fatalf("B row %q has width_bytes %q, which is not a number: %v", r["field_index"], r["width_bytes"], err)
		}
		bWidth[normaliseKey(r["field_index"])] = w
	}
	record, dataArea := 0, 0
	for k, w := range bWidth {
		dataArea += w
		if k != theChannelAddressKey {
			record += w
		}
	}
	if record != 39 {
		t.Errorf("B's widths sum to %d over the eight keys after ①, ②, want 39 — that is the RECORD-ONLY length this profile carries (spec Erratum 1)", record)
	}
	if dataArea != 41 {
		t.Errorf("B's widths sum to %d over all nine keys, want 41 — that is the DATA-AREA length, the record plus the two-byte channel address (spec Erratum 1)", dataArea)
	}

	// ---- 5. The profile's layout agrees, as ORDERED SPAN-UNIONS. -----
	//
	// theGroups is §E's partition. It is checked against the layout before
	// it is used, so a partition that had drifted from the code could not
	// silently make the widths agree: every span must fall wholly inside
	// exactly one group, the groups must be consecutive and
	// non-overlapping, and together they must cover every byte of the
	// record exactly once.
	layout, ok := ic7300.Profile().LayoutFor(39)
	if !ok {
		t.Fatal("LayoutFor(39) missing")
	}
	next := 0
	for i, g := range theGroups {
		if g.lo != next {
			t.Fatalf("group %d (%s) starts at byte %d, want %d — §E's partition must be consecutive with no gap and no overlap", i, g.key, g.lo, next)
		}
		if g.hi < g.lo {
			t.Fatalf("group %d (%s) is empty (%d..%d)", i, g.key, g.lo, g.hi)
		}
		next = g.hi + 1
	}
	if next != layout.Length {
		t.Fatalf("§E's partition covers bytes 0..%d, but the record is %d bytes — every byte must belong to exactly one printed row", next-1, layout.Length)
	}
	groupOf := func(off int) (recordGroup, bool) {
		for _, g := range theGroups {
			if off >= g.lo && off <= g.hi {
				return g, true
			}
		}
		return recordGroup{}, false
	}
	spansPerGroup := map[string]int{}
	for i, sp := range layout.Fields {
		g, ok := groupOf(sp.Offset)
		if !ok {
			t.Fatalf("span %d (%s) at offset %d lies in no group", i, sp.Field, sp.Offset)
		}
		if last := sp.Offset + sp.Length - 1; last > g.hi {
			t.Errorf("span %d (%s) spans bytes %d..%d and straddles the boundary of group %s (%d..%d) — a span crossing two printed rows would make the union-to-row binding meaningless",
				i, sp.Field, sp.Offset, last, g.key, g.lo, g.hi)
		}
		spansPerGroup[g.key]++
	}
	for _, g := range theGroups {
		if spansPerGroup[g.key] == 0 {
			t.Errorf("group %s (bytes %d..%d) has no span at all — the record would decode with a hole in it", g.key, g.lo, g.hi)
		}
		gotBytes := g.hi - g.lo + 1
		wantBytes, ok := bWidth[g.key]
		if !ok {
			t.Errorf("group %s has no matching row in B", g.key)
			continue
		}
		if gotBytes != wantBytes {
			t.Errorf("group %s: the profile occupies %d bytes, B records width_bytes %d", g.key, gotBytes, wantBytes)
		}
	}
	// The three multi-span rows, named, so that a later edit which split
	// or merged a span shows up as a changed count rather than passing on
	// the strength of the byte width alone. ❹–⓱ is SEVEN spans — the
	// distinct TX frequency plus the six mirrored field ids — and the plan
	// text says "six" in one place and "seven" in another; seven is what
	// §E's own layout table gives, and the byte width above is the binding
	// assertion either way.
	for key, want := range map[string]int{"⑨,⑩": 2, "⑪": 2, "❹-⓱": 7} {
		if got := spansPerGroup[key]; got != want {
			t.Errorf("group %s covers %d spans, want %d", key, got, want)
		}
	}

	// ---- 6. The order agrees. ----------------------------------------
	//
	// L's row order is the strip's left-to-right order. Past the channel
	// address, each key in that order must map to a STRICTLY INCREASING
	// profile offset — which is the wire-order assumption (D5 entry 5,
	// lift ic7300-wire-order) stated as a test.
	groupStart := map[string]int{}
	for _, g := range theGroups {
		groupStart[g.key] = g.lo
	}
	prev := -1
	seenAddress := false
	order := make([]string, 0, len(ledger))
	for _, r := range ledger {
		k := normaliseKey(r["field_index"])
		order = append(order, k)
		if k == theChannelAddressKey {
			seenAddress = true
			continue
		}
		off, ok := groupStart[k]
		if !ok {
			t.Errorf("L names key %q, which §E's partition does not", k)
			continue
		}
		if off <= prev {
			t.Errorf("key %q starts at profile offset %d, which does not increase on the previous key's %d — L's row order is the strip's order and the profile's offsets must follow it", k, off, prev)
		}
		prev = off
	}
	if !seenAddress {
		t.Errorf("L never names %q — the channel address is the strip's first printed row and its absence would mean the leg was filtered wrongly", theChannelAddressKey)
	}
	if len(order) != 9 || order[0] != theChannelAddressKey {
		t.Errorf("L's D1 row order is %v, want nine rows beginning with %q", order, theChannelAddressKey)
	}

	// ---- 7. Hazard (d) is carried, not smoothed. ---------------------
	//
	// The duplicated block is drawn in the OTHER glyph class and repeats
	// indices already used earlier in the same strip. A leg that had
	// normalised the two styles to one would fail here, and that is the
	// point: this is the pair's model-specific hazard and the crosscheck
	// must carry it rather than tidy it away.
	const hazardKey = "❹-⓱"
	for _, leg := range []struct {
		name string
		keys map[string]bool
	}{{"L", lKeys}, {"B", bKeys}, {"W", wKeys}} {
		if !leg.keys[hazardKey] {
			t.Errorf("%s does not name %q — all three legs must carry the duplicated block", leg.name, hazardKey)
		}
	}
	styles := map[string]string{}
	for _, r := range ledger {
		styles[normaliseKey(r["field_index"])] = r["index_style"]
	}
	for k, style := range styles {
		want := "circled"
		if k == hazardKey {
			want = "filled"
		}
		if style != want {
			t.Errorf("L records index_style %q for key %q, want %q — the duplicated block is drawn as WHITE numerals reversed out of SOLID BLACK discs and every other D1 row is drawn as black numerals in outlined circles (matrix §3.15)", style, k, want)
		}
	}
	lo, hi := indexRange(t, hazardKey)
	if lo != 4 || hi != 17 {
		t.Fatalf("key %q reads as indices %d..%d, want 4..17", hazardKey, lo, hi)
	}
	repeats := 0
	for _, k := range theEightRecordKeys {
		if k == hazardKey {
			continue
		}
		klo, khi := indexRange(t, k)
		if klo <= hi && khi >= lo {
			repeats++
		}
	}
	if repeats == 0 {
		t.Errorf("key %q covers indices %d..%d and overlaps no earlier printed range — the whole hazard is that it REPEATS indices already used on this strip, so a run with no overlap means the keys have been renumbered", hazardKey, lo, hi)
	}

	// The profile's own consequence of hazard (d), asserted once here so
	// this file states it as well as doc.go: the duplicated block's spans
	// carry the SAME civ field ids as their RX copies (which is what makes
	// the decoder require the two copies to agree), except the transmit
	// FREQUENCY, which is distinct so that a split channel round-trips.
	txBlock := map[civ.FieldID]int{}
	for _, sp := range layout.Fields {
		if sp.Offset >= 15 && sp.Offset <= 28 {
			txBlock[sp.Field]++
		}
	}
	if txBlock[civ.FieldTXFrequency] != 1 {
		t.Errorf("the ❹–⓱ block carries %d FieldTXFrequency spans, want 1 — the transmit frequency is the one byte range of the block with a field of its own", txBlock[civ.FieldTXFrequency])
	}
	for _, id := range []civ.FieldID{civ.FieldMode, civ.FieldFilter, civ.FieldDataMode, civ.FieldToneMode, civ.FieldToneTX, civ.FieldToneRX} {
		if txBlock[id] != 1 {
			t.Errorf("the ❹–⓱ block carries %d %s spans, want 1 — the block mirrors its RX copy field for field", txBlock[id], id)
		}
	}
}
