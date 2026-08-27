// SPDX-License-Identifier: GPL-3.0-or-later

package ic7300mk2_test

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7300mk2"
)

// readCSV parses one frozen artefact into header-keyed rows, with
// encoding/csv and never a regexp over prose.
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

// onlyD1 keeps the rows of the memory-record strip. D2 and D3 are the two
// nibble insets.
func onlyD1(rows []map[string]string) []map[string]string {
	out := make([]map[string]string, 0, len(rows))
	for _, r := range rows {
		if r["diagram_id"] == "D1" {
			out = append(out, r)
		}
	}
	return out
}

// indexValue maps one printed index numeral to the number it draws:
// outlined ①–⑳ and ㉑–㉟, filled ❶–❿ and ⓫–⓴.
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

// normaliseKey is D19's normaliser, and on THIS model it is GLYPH↔DIGIT,
// not whitespace-only.
//
// The IC-7300's three legs all spell their indices in circled and filled
// numerals, so that model's crosscheck needs no such mapping. THIS
// model's do not agree with each other: `ic7300mk2-field-ledger.csv`
// spells its keys in PLAIN ASCII DIGITS — 1, 2 / 3 / 4~8 / 9, 10 / 11 /
// 12~14 / 15~17 / 4~17 / 18~33 — while B and W carry ①, ② … ❹ ~ ⓱ …
// ⑱ ~ ㉝. A whitespace-only normaliser would find nine keys in each leg
// and NO key in common.
//
// So: every circled or filled numeral becomes its decimal value,
// whitespace is dropped, and `~`, `–` and `-` are one separator.
//
// D19's canonical keys are 1,2 · 3 · 4-8 · 9,10 · 11 · 12-14 · 15-17 ·
// 4-17 · 18-33.
//
// THE FILLED/OUTLINED DISTINCTION MUST NEVER BE READ FROM THE NORMALISED
// KEY. 4-17 and the earlier 4-8 and 15-17 share their numbers BY DESIGN —
// that is hazard (d), the whole reason the duplicated block is dangerous —
// and only L's own index_style column records which glyph class was drawn.
func normaliseKey(s string) string {
	var b strings.Builder
	for _, r := range s {
		if v, ok := indexValue(r); ok {
			b.WriteString(strconv.Itoa(v))
			continue
		}
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

// theEightRecordKeys is the record's own field order — every printed key
// EXCEPT 1,2, which is the CHANNEL ADDRESS and not part of the record.
var theEightRecordKeys = []string{
	"3", "4-8", "9,10", "11", "12-14", "15-17", "4-17", "18-33",
}

// theChannelAddressKey is the ninth printed key, excluded from the record.
const theChannelAddressKey = "1,2"

// recordGroup is one printed row's ORDERED SPAN-UNION in the layout.
type recordGroup struct {
	key    string
	lo, hi int // inclusive record byte offsets
}

// theGroups is §E's MK2 partition: eight consecutive, non-overlapping
// unions covering every byte of the 45-byte record exactly once. It
// differs from the IC-7300's in its last group alone.
var theGroups = []recordGroup{
	{"3", 0, 0},
	{"4-8", 1, 5},
	{"9,10", 6, 7},
	{"11", 8, 8},
	{"12-14", 9, 11},
	{"15-17", 12, 14},
	{"4-17", 15, 28},
	{"18-33", 29, 44},
}

// TestCrosscheckLedgerTranscriptionAndProfile holds the field ledger (L),
// the semantic transcription (B) and THE PROFILE ITSELF to each other,
// field for field. "A" is the profile: this package has no separate
// transcription A, and the layout being the third leg is what makes the
// crosscheck bind the CODE.
//
// Any mismatch is a STOP for the orchestrator to arbitrate against the
// PDF; the fix goes to exactly ONE of the profile, the ledger, B or W, and
// never by editing a frozen artefact.
func TestCrosscheckLedgerTranscriptionAndProfile(t *testing.T) {
	ledger := onlyD1(readCSV(t, "ic7300mk2-field-ledger.csv"))
	transcription := onlyD1(readCSV(t, "ic7300mk2-transcription-b.csv"))
	witness := onlyD1(readCSV(t, "ic7300mk2-geometry-witness.csv"))

	// ---- 1. Scope, stated and pinned. --------------------------------
	//
	// THIS MODEL'S B LEG HAS NO CONDITIONAL ROWS. Its nine D1 rows are the
	// record and nothing else; the clear form is recorded in the matrix,
	// not in B. The count is still asserted — as ZERO — so that the two
	// siblings' crosschecks make the same statement about the same column
	// rather than one of them being silent about it.
	var bRecord []map[string]string
	conditional := 0
	for _, r := range transcription {
		if r["encoding"] == "conditional" {
			conditional++
			t.Errorf("B carries a conditional D1 row %q — this model's transcription has none, and a new one would be a documented alternative form this package neither builds nor admits", r["field_index"])
			continue
		}
		bRecord = append(bRecord, r)
	}
	if conditional != 0 {
		t.Fatalf("B has %d conditional D1 rows, want 0", conditional)
	}

	// ---- 2. Key sets equal, both directions. -------------------------
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
			t.Errorf("%s: D19's normalisation yields %d distinct keys, want 9 (the eight record fields plus the channel address)", leg, len(out))
		}
		return out
	}
	lKeys := keysOf("L (field ledger, PLAIN DIGITS)", ledger)
	bKeys := keysOf("B (transcription, GLYPHS)", bRecord)
	wKeys := keysOf("W (geometry witness, GLYPHS)", witness)
	for _, pair := range []struct {
		a, b   string
		ka, kb map[string]bool
	}{
		{"L", "B", lKeys, bKeys},
		{"B", "W", bKeys, wKeys},
		{"W", "L", wKeys, lKeys},
	} {
		for k := range pair.ka {
			if !pair.kb[k] {
				t.Errorf("key %q is in %s but not in %s — the three legs must name the same nine fields once D19's glyph↔digit mapping has been applied", k, pair.a, pair.b)
			}
		}
		for k := range pair.kb {
			if !pair.ka[k] {
				t.Errorf("key %q is in %s but not in %s — the three legs must name the same nine fields once D19's glyph↔digit mapping has been applied", k, pair.b, pair.a)
			}
		}
	}
	// The canonical set, written out, so a normaliser that had drifted
	// into agreeing with itself on the wrong keys is caught.
	for _, k := range append([]string{theChannelAddressKey}, theEightRecordKeys...) {
		if !lKeys[k] {
			t.Errorf("D19's canonical key %q is not produced by L", k)
		}
		if !bKeys[k] {
			t.Errorf("D19's canonical key %q is not produced by B", k)
		}
		if !wKeys[k] {
			t.Errorf("D19's canonical key %q is not produced by W", k)
		}
	}

	// ---- 3. Labels agree. --------------------------------------------
	//
	// The one exception is EVIDENCE: matrix Erratum 3 records that PDF
	// p.17 carries EIGHT keyed legends, not nine — the split-TX block has
	// none — so L and B must BOTH be empty there.
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
		if k == "4-17" {
			if want != "" || got != "" {
				t.Errorf("key 4-17 (❹ ~ ⓱): L label %q, B label %q — BOTH must be empty. PDF p.17 carries eight keyed legends and the duplicated block has none (matrix Erratum 3); an empty label there is the evidence, not a gap", want, got)
			}
			continue
		}
		if got != want {
			t.Errorf("key %q: L label_verbatim %q, B label_verbatim %q — the two legs transcribed the same printed heading differently", k, want, got)
		}
	}

	// ---- 4. Widths agree with the two lengths. -----------------------
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
	if record != 45 {
		t.Errorf("B's widths sum to %d over the eight keys after ①, ②, want 45 — the RECORD-ONLY length this profile carries (spec Erratum 1)", record)
	}
	if dataArea != 47 {
		t.Errorf("B's widths sum to %d over all nine keys, want 47 — the DATA-AREA length, and the figure D6's \"47 B\" row gives (spec Erratum 1)", dataArea)
	}

	// ---- 4b. The bare heading, asserted as bare. ---------------------
	//
	// §3.16 A6: `⑫ ~ ⑭ Repeater tone frequency setting` is printed with
	// NOTHING beneath it. An empty values_verbatim there is the evidence
	// that this profile's encoding for those three bytes is ASSUMED
	// (ic7300mk2-tone-tx-encoding, lift MK2-R17) — not a gap in the
	// transcription — so the emptiness is asserted rather than tolerated.
	for _, r := range bRecord {
		if normaliseKey(r["field_index"]) != "12-14" {
			continue
		}
		if got := r["values_verbatim"]; got != "" {
			t.Errorf("B's ⑫ ~ ⑭ row records values_verbatim %q, want EMPTY — the heading is printed with nothing beneath it (§3.16 A6), and an empty cell is what makes ic7300mk2-tone-tx-encoding an ASSUMED entry rather than a transcription gap", got)
		}
		if got := r["encoding"]; got != "unstated" {
			t.Errorf("B's ⑫ ~ ⑭ row records encoding %q, want \"unstated\"", got)
		}
	}

	// ---- 5. The profile's layout agrees, as ORDERED SPAN-UNIONS. -----
	layout, ok := ic7300mk2.Profile().LayoutFor(45)
	if !ok {
		t.Fatal("LayoutFor(45) missing")
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
		t.Fatalf("§E's partition covers bytes 0..%d, but the record is %d bytes", next-1, layout.Length)
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
	// The three multi-span rows: ⑨, ⑩ is a two-span union, ⑪ a
	// two-nibble-span union, and ❹ ~ ⓱ a SEVEN-span union — the distinct
	// TX frequency plus the six mirrored field ids.
	for key, want := range map[string]int{"9,10": 2, "11": 2, "4-17": 7} {
		if got := spansPerGroup[key]; got != want {
			t.Errorf("group %s covers %d spans, want %d", key, got, want)
		}
	}

	// ---- 6. The order agrees. ----------------------------------------
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
			t.Errorf("key %q starts at profile offset %d, which does not increase on the previous key's %d — L's row order is the band's order and the profile's offsets must follow it", k, off, prev)
		}
		prev = off
	}
	if !seenAddress {
		t.Errorf("L never names %q — the channel address is the band's first printed row", theChannelAddressKey)
	}
	if len(order) != 9 || order[0] != theChannelAddressKey {
		t.Errorf("L's D1 row order is %v, want nine rows beginning with %q", order, theChannelAddressKey)
	}

	// ---- 7. Hazard (d) is carried, not smoothed. ---------------------
	const hazardKey = "4-17"
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
			t.Errorf("L records index_style %q for key %q, want %q — ❹ and ⓱ are drawn as WHITE numerals reversed out of SOLID BLACK discs and every other index on the band is outlined (matrix §3.15). THE STYLE IS READ FROM THIS COLUMN AND NEVER FROM THE KEY: 4-17 shares its numbers with 4-8 and 15-17 by design", style, k, want)
		}
	}
	// The numbers themselves overlap earlier rows — which is exactly why
	// the style column, and not the key, is what tells the two classes
	// apart.
	repeats := 0
	for _, k := range theEightRecordKeys {
		if k == hazardKey {
			continue
		}
		if overlaps(t, hazardKey, k) {
			repeats++
		}
	}
	if repeats == 0 {
		t.Errorf("key %q overlaps no earlier printed range — the hazard is that it REPEATS indices already used on this band, so a run with no overlap means the keys have been renumbered", hazardKey)
	}

	// The profile's own consequence: the duplicated block mirrors its RX
	// copy field for field, except the transmit FREQUENCY, which is
	// distinct so a split channel round-trips.
	txBlock := map[civ.FieldID]int{}
	for _, sp := range layout.Fields {
		if sp.Offset >= 15 && sp.Offset <= 28 {
			txBlock[sp.Field]++
		}
	}
	if txBlock[civ.FieldTXFrequency] != 1 {
		t.Errorf("the ❹ ~ ⓱ block carries %d FieldTXFrequency spans, want 1", txBlock[civ.FieldTXFrequency])
	}
	for _, id := range []civ.FieldID{civ.FieldMode, civ.FieldFilter, civ.FieldDataMode, civ.FieldToneMode, civ.FieldToneTX, civ.FieldToneRX} {
		if txBlock[id] != 1 {
			t.Errorf("the ❹ ~ ⓱ block carries %d %s spans, want 1 — the block mirrors its RX copy field for field", txBlock[id], id)
		}
	}
}

// overlaps reports whether two canonical keys cover any index in common.
func overlaps(t *testing.T, a, b string) bool {
	t.Helper()
	alo, ahi := keyRange(t, a)
	blo, bhi := keyRange(t, b)
	return alo <= bhi && blo <= ahi
}

// keyRange is the numeric span a canonical key covers: (4, 8) for 4-8,
// (3, 3) for 3, (4, 17) for 4-17.
func keyRange(t *testing.T, key string) (lo, hi int) {
	t.Helper()
	var vals []int
	for _, part := range strings.FieldsFunc(key, func(r rune) bool { return r == '-' || r == ',' }) {
		v, err := strconv.Atoi(part)
		if err != nil {
			t.Fatalf("canonical key %q has the component %q, which is not a number: %v", key, part, err)
		}
		vals = append(vals, v)
	}
	if len(vals) == 0 {
		t.Fatalf("canonical key %q carries no index", key)
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
