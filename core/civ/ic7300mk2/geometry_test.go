// SPDX-License-Identifier: GPL-3.0-or-later

package ic7300mk2_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7300mk2"
)

// unreadable is the literal the W leg writes where it could measure
// nothing. It is a MEASUREMENT OUTCOME, not a missing cell.
const unreadable = "UNREADABLE"

// theNineWitnessKeys is the band's own left-to-right order — §E's field
// order with the channel address at its head — in D19's canonical form.
var theNineWitnessKeys = append([]string{theChannelAddressKey}, theEightRecordKeys...)

// TestGeometryWitnessBindsTheLayout ties the profile's record layout to the
// W leg's MEASURED drawn-cell positions.
//
// W COUNTS DRAWN CELLS, NOT BYTES. This model's band draws sixteen byte
// cells for a forty-five-byte record: three runs are drawn abbreviated —
// a shaded "..." elision cell inside ④ ~ ⑧, a wide dotted region under
// ❹ ~ ⓱ and a shaded "..." cell inside ⑱ ~ ㉝ — and the witness measured
// those elisions at roughly one, three and one cell pitches, which is
// nothing like what they elide. So drawn extent equals byte width for the
// groups drawn in full and cannot for the other three, and this test
// asserts exactly that shape.
//
// THE ❹ ~ ⓱ ROW IS MORE UNREADABLE ON THIS MODEL THAN ON THE IC-7300.
// There, the region occupies one countable drawn position and only its
// NIBBLE columns are unreadable. Here no "X:X" cell and no nibble divider
// is drawn anywhere inside the bracket, so ALL FOUR of the leg's position
// columns read UNREADABLE and no cell can be counted at all.
func TestGeometryWitnessBindsTheLayout(t *testing.T) {
	all := readCSV(t, "ic7300mk2-geometry-witness.csv")
	rows := onlyD1(all)

	// ---- 1. Nine rows, in §E's order. --------------------------------
	if len(rows) != 9 {
		t.Fatalf("W has %d D1 rows, want 9 — every later assertion indexes this run", len(rows))
	}
	for i, r := range rows {
		if got, want := normaliseKey(r["field_index"]), theNineWitnessKeys[i]; got != want {
			t.Fatalf("W D1 row %d is %q, want %q — the leg's row order IS the band's left-to-right order and the rest of this test reads it positionally", i, got, want)
		}
	}
	// This leg records D1 AND NOTHING ELSE, which is the deliberate
	// difference from the IC-7300's W leg and the reason assertion 5 below
	// binds the nibble insets to B instead.
	for _, r := range all {
		if id := r["diagram_id"]; id != "D1" {
			t.Errorf("W carries a %s row (%q) — this model's witness records only D1, and the nibble insets are bound to B", id, r["field_index"])
		}
	}

	// ---- 2. Drawn extents, and the one wholly unreadable row. --------
	num := func(row map[string]string, col string) int {
		t.Helper()
		v, err := strconv.Atoi(row[col])
		if err != nil {
			t.Fatalf("W row %q column %s is %q, which is not a number: %v", row["field_index"], col, row[col], err)
		}
		return v
	}
	const hazardKey = "4-17"
	positionCols := []string{"first_byte", "first_nibble", "last_byte", "last_nibble"}
	unreadableRows := 0
	drawn := make([]int, len(rows))     // -1 where nothing can be counted
	firstByte := make([]int, len(rows)) // -1 likewise
	for i, r := range rows {
		key := normaliseKey(r["field_index"])
		bad := 0
		for _, c := range positionCols {
			if r[c] == unreadable {
				bad++
			}
		}
		if bad > 0 {
			unreadableRows++
			if key != hazardKey {
				t.Errorf("W row %q has %d %s position columns — only the duplicated block's dotted region is unmeasurable on this model", key, bad, unreadable)
			}
			if bad != len(positionCols) {
				t.Errorf("W row %q has %d of %d position columns reading %s, want ALL FOUR — no X:X cell and no nibble divider is drawn anywhere inside this bracket, so neither a byte nor a nibble position exists to record (W STOP 6/7/9)", key, bad, len(positionCols), unreadable)
			}
			drawn[i], firstByte[i] = -1, -1
			continue
		}
		if fn, ln := num(r, "first_nibble"), num(r, "last_nibble"); fn != 1 || ln != 2 {
			t.Errorf("W row %q spans nibbles %d..%d, want 1..2 — every drawn cell on this band carries one dotted nibble divider at its centre", key, fn, ln)
		}
		firstByte[i] = num(r, "first_byte")
		drawn[i] = num(r, "last_byte") - firstByte[i] + 1
	}
	if unreadableRows != 1 {
		t.Errorf("W has %d D1 rows with an %s position column, want exactly 1 (❹ ~ ⓱)", unreadableRows, unreadable)
	}

	// ---- 3. The measured runs, pinned. -------------------------------
	//
	// -1 stands for the unreadable row in both runs, which is how the plan
	// writes them: [2, 1, 2, 2, 1, 3, 3, UNREADABLE, 2] and
	// [1, 3, 4, 6, 8, 9, 12, UNREADABLE, 15].
	wantDrawn := []int{2, 1, 2, 2, 1, 3, 3, -1, 2}
	wantFirst := []int{1, 3, 4, 6, 8, 9, 12, -1, 15}
	for i := range rows {
		if drawn[i] != wantDrawn[i] {
			t.Errorf("W row %q spans %d drawn cells, want %d (-1 meaning %s)", theNineWitnessKeys[i], drawn[i], wantDrawn[i], unreadable)
		}
		if firstByte[i] != wantFirst[i] {
			t.Errorf("W row %q begins at drawn cell %d, want %d (-1 meaning %s)", theNineWitnessKeys[i], firstByte[i], wantFirst[i], unreadable)
		}
	}
	// Contiguity across the whole band, with the dotted region occupying
	// NO drawn cell — which is the leg's own finding, not a convenience.
	// A gap anywhere else would mean a drawn cell nobody accounted for.
	prevLast := 0
	for i := range rows {
		if drawn[i] < 0 {
			continue
		}
		if firstByte[i] != prevLast+1 {
			t.Errorf("W row %q begins at drawn cell %d but the previous measured row ends at %d — the drawn run must be contiguous, the dotted region contributing no cell",
				theNineWitnessKeys[i], firstByte[i], prevLast)
		}
		prevLast = firstByte[i] + drawn[i] - 1
	}
	if prevLast != 16 {
		t.Errorf("the band's last measured drawn cell is %d, want 16 — sixteen drawn cells stand for a forty-five-byte record, which is what the three elisions carry", prevLast)
	}

	// ---- 4. Where drawn extent and byte width part company. ----------
	byteWidth := map[string]int{
		// The channel address is not part of the record and has no
		// FieldSpan: AddressFormFlat encodes it as two packed-BCD bytes.
		theChannelAddressKey: 2,
	}
	for _, g := range theGroups {
		byteWidth[g.key] = g.hi - g.lo + 1
	}
	wantDisagree := map[string]bool{"4-8": true, hazardKey: true, "18-33": true}
	for i, r := range rows {
		key := theNineWitnessKeys[i]
		bytes, ok := byteWidth[key]
		if !ok {
			t.Fatalf("no byte width is declared for key %q", key)
		}
		if wantDisagree[key] {
			if drawn[i] == bytes {
				t.Errorf("group %q: drawn extent %d EQUALS byte width %d — this is one of the three abbreviated runs and the two cannot agree", key, drawn[i], bytes)
			}
			if !strings.Contains(r["notes"], "STOP") {
				t.Errorf("group %q disagrees (drawn %d, bytes %d) but W's notes carry no STOP token — the leg must have recorded the abbreviation it could not resolve", key, drawn[i], bytes)
			}
			continue
		}
		if drawn[i] != bytes {
			t.Errorf("group %q: drawn extent %d, byte width %d — this group is drawn in full, so the two must agree exactly", key, drawn[i], bytes)
		}
	}

	// ---- 5. The two nibble insets, bound to B rather than to W. ------
	//
	// THE SUBSTITUTION IS DELIBERATE. This model's W leg records only D1 —
	// it has no D2/D3 inset rows, unlike the IC-7300's — so there is no
	// measured geometry here to bind the two split bytes to. B has read
	// the insets' own ARROW LABELS instead, which on this model is the
	// stronger evidence anyway: the IC-7300's assignment had to be traced
	// by following leaders whose printed order reverses the nibbles', and
	// this document labels each half outright.
	insetValues := map[string]string{}
	for _, r := range onlyD1(readCSV(t, "ic7300mk2-transcription-b.csv")) {
		insetValues[normaliseKey(r["field_index"])] = r["values_verbatim"]
	}
	for _, want := range []struct{ key, values string }{
		{"3", "SPLIT 0=OFF | SPLIT 1=ON | SELECT 0=OFF | SELECT 1=★1 | SELECT 2=★2 | SELECT 3=★3"},
		{"11", "DATA 0=OFF | DATA 1=ON | TONE 0=OFF | TONE 1=TONE | TONE 2=TSQL"},
	} {
		if got := insetValues[want.key]; got != want.values {
			t.Errorf("B's %s row records values_verbatim %q,\nwant %q — the column headings are prefixed to each cell precisely so the nibble each value came from is preserved", want.key, got, want.values)
		}
	}

	layout, ok := ic7300mk2.Profile().LayoutFor(45)
	if !ok {
		t.Fatal("LayoutFor(45) missing")
	}
	nibbles := func(off int) []civ.FieldSpan {
		var out []civ.FieldSpan
		for _, sp := range layout.Fields {
			if sp.Offset == off {
				out = append(out, sp)
			}
		}
		return out
	}
	// ③ at offset 0: TWO nibbles printed with their own code columns, ONE
	// of them mapped. The unmapped half is not missing evidence — B read
	// its arrow label and its two values — it is the SPLIT flag, left
	// under the Fixed template so a driver can see a Split-ON record and
	// refuse to write it back (D14, enablers E6).
	third := nibbles(0)
	if len(third) != 1 {
		t.Fatalf("the profile maps %d spans at record byte 0 (③), want exactly 1 — B transcribed TWO labelled nibbles there and the profile maps ONE of them", len(third))
	}
	if third[0].Field != civ.FieldSelect || third[0].Nibble != civ.NibbleLow || third[0].Length != 1 {
		t.Errorf("record byte 0 carries {%s len=%d nib=%v}, want {select len=1 NibbleLow} — SELECT is the RIGHT (low) nibble by the inset's own arrow label, and SPLIT is left unmapped (D14)", third[0].Field, third[0].Length, third[0].Nibble)
	}
	if len(layout.Fixed) != layout.Length {
		t.Fatalf("len(Fixed) = %d, want %d — the template is what DECLARES ③'s high nibble unmapped, and without it the asymmetry above would be an omission rather than a contract", len(layout.Fixed), layout.Length)
	}
	if layout.Fixed[0]&0xF0 != 0x00 {
		t.Errorf("Fixed[0] = %#02x — the unmapped high nibble must be 0x0, SPLIT 0=OFF: it is what each driver's E6 check compares a just-read record against", layout.Fixed[0])
	}
	// ⑪ at offset 8: BOTH nibbles mapped, DATA left and TONE right.
	eleventh := nibbles(8)
	if len(eleventh) != 2 {
		t.Fatalf("the profile maps %d spans at record byte 8 (⑪), want exactly 2", len(eleventh))
	}
	if eleventh[0].Field != civ.FieldDataMode || eleventh[0].Nibble != civ.NibbleHigh {
		t.Errorf("record byte 8's first span is {%s %v}, want {data_mode NibbleHigh} — the inset's LEFT up-arrow reads DATA", eleventh[0].Field, eleventh[0].Nibble)
	}
	if eleventh[1].Field != civ.FieldToneMode || eleventh[1].Nibble != civ.NibbleLow {
		t.Errorf("record byte 8's second span is {%s %v}, want {tone_mode NibbleLow} — the inset's RIGHT up-arrow reads TONE", eleventh[1].Field, eleventh[1].Nibble)
	}
}
