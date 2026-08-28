// SPDX-License-Identifier: GPL-3.0-or-later

package ic7300_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7300"
)

// unreadable is the literal the W leg writes where it could measure
// nothing. It is a MEASUREMENT OUTCOME, not a missing cell, and the tests
// below assert the literal rather than treating an unparseable number as
// zero.
const unreadable = "UNREADABLE"

// theNineWitnessKeys is the strip's own left-to-right order — §E's field
// order with the channel address at its head — normalised.
var theNineWitnessKeys = append([]string{theChannelAddressKey}, theEightRecordKeys...)

// TestGeometryWitnessBindsTheLayout ties the profile's record layout to the
// W leg's MEASURED drawn-cell positions.
//
// W COUNTS DRAWN CELLS, NOT BYTES, and says so. The diagram abbreviates
// three runs — an elision cell inside ④–⑧, an undivided region under ❹–⓱
// and an elision cell inside ⑱–㉗ — so drawn extent equals byte width for
// six of the nine groups and cannot for the other three. This test asserts
// exactly that shape: the six agree, the three that disagree are exactly the
// three W stopped on, and every group's ORDER is W's order. Anything else
// would be asserting a reconciliation W declined to make.
func TestGeometryWitnessBindsTheLayout(t *testing.T) {
	all := readCSV(t, "ic7300-geometry-witness.csv")
	rows := onlyD1(all)

	// ---- 1. Nine rows, in §E's order. --------------------------------
	if len(rows) != 9 {
		t.Fatalf("W has %d D1 rows, want 9 — every later assertion indexes this run", len(rows))
	}
	for i, r := range rows {
		if got, want := normaliseKey(r["field_index"]), theNineWitnessKeys[i]; got != want {
			t.Fatalf("W D1 row %d is %q, want %q — the leg's row order IS the strip's left-to-right order and the rest of this test reads it positionally", i, got, want)
		}
	}

	// ---- 2. Drawn extents, and the one unreadable row. ---------------
	//
	// ❹–⓱ is the only D1 row whose NIBBLE columns could not be read: the
	// region is drawn undivided, with no X:X cell and no nibble rule
	// anywhere inside it. Its BYTE columns were readable — the region
	// occupies one drawn position — so drawn extent is still computable
	// there; it is the nibbles that are absent.
	num := func(row map[string]string, col string) int {
		t.Helper()
		v, err := strconv.Atoi(row[col])
		if err != nil {
			t.Fatalf("W row %q column %s is %q, which is not a number: %v", row["field_index"], col, row[col], err)
		}
		return v
	}
	unreadableRows := 0
	drawn := make([]int, len(rows))
	firstByte := make([]int, len(rows))
	for i, r := range rows {
		key := normaliseKey(r["field_index"])
		bad := r["first_nibble"] == unreadable || r["last_nibble"] == unreadable
		if bad {
			unreadableRows++
			if key != "❹-⓱" {
				t.Errorf("W row %q has an %s nibble column — only the duplicated block's undivided region is unreadable on this model", key, unreadable)
			}
			if r["first_nibble"] != unreadable || r["last_nibble"] != unreadable {
				t.Errorf("W row %q reads first_nibble %q and last_nibble %q — the undivided region has NO nibble rule at either end, so both columns must read %s", key, r["first_nibble"], r["last_nibble"], unreadable)
			}
		} else {
			if fn, ln := num(r, "first_nibble"), num(r, "last_nibble"); fn != 1 || ln != 2 {
				t.Errorf("W row %q spans nibbles %d..%d, want 1..2 — every drawn cell on this strip carries one dotted nibble rule at its centre", key, fn, ln)
			}
		}
		firstByte[i] = num(r, "first_byte")
		drawn[i] = num(r, "last_byte") - firstByte[i] + 1
	}
	if unreadableRows != 1 {
		t.Errorf("W has %d D1 rows with an %s nibble column, want exactly 1 (❹–⓱) — a second one would mean the leg could not read a cell it reports a position for", unreadableRows, unreadable)
	}

	// ---- 3. The measured runs, pinned. -------------------------------
	wantDrawn := []int{2, 1, 3, 2, 1, 3, 3, 1, 3}
	wantFirst := []int{1, 3, 4, 7, 9, 10, 13, 16, 17}
	for i := range rows {
		if drawn[i] != wantDrawn[i] {
			t.Errorf("W row %q spans %d drawn cells, want %d", theNineWitnessKeys[i], drawn[i], wantDrawn[i])
		}
		if firstByte[i] != wantFirst[i] {
			t.Errorf("W row %q begins at drawn cell %d, want %d", theNineWitnessKeys[i], firstByte[i], wantFirst[i])
		}
		// Contiguity is what proves the leg counted EVERY drawn cell
		// rather than the ones it recognised: a gap would mean a cell
		// nobody accounted for.
		if i > 0 {
			if prevLast := firstByte[i-1] + drawn[i-1] - 1; firstByte[i] != prevLast+1 {
				t.Errorf("W row %q begins at drawn cell %d but the previous row ends at %d — the run must be contiguous with no gap", theNineWitnessKeys[i], firstByte[i], prevLast)
			}
		}
	}

	// ---- 4. Where drawn extent and byte width part company. ----------
	//
	// The three abbreviations are the whole reason this test cannot simply
	// compare W's numbers to byte offsets: an elision cell stands for a
	// run of bytes nobody drew. Each of the three must carry a STOP token
	// in W's own notes, because the leg is required to have SEEN the
	// abbreviation rather than silently averaged over it; and every other
	// group must agree EXACTLY, which is what stops the three exceptions
	// widening into a general excuse.
	byteWidth := map[string]int{
		// The channel address is not part of the record and has no
		// FieldSpan: AddressFormFlat encodes it as two packed-BCD bytes.
		theChannelAddressKey: 2,
	}
	for _, g := range theGroups {
		byteWidth[g.key] = g.hi - g.lo + 1
	}
	wantDisagree := map[string]bool{"④-⑧": true, "❹-⓱": true, "⑱-㉗": true}
	for i, r := range rows {
		key := theNineWitnessKeys[i]
		bytes, ok := byteWidth[key]
		if !ok {
			t.Fatalf("no byte width is declared for key %q", key)
		}
		if wantDisagree[key] {
			if drawn[i] == bytes {
				t.Errorf("group %q: drawn extent %d EQUALS byte width %d — this is one of the three abbreviated runs and the two cannot agree; if they now do, either the layout or the leg has changed", key, drawn[i], bytes)
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

	// ---- 5. The two nibble insets. -----------------------------------
	//
	// D2 (③) and D3 (⑪) are the expansion boxes: one cell each, divided by
	// a dotted rule into two halves. The profile maps BOTH halves of ⑪ and
	// only ONE half of ③, and that asymmetry is asserted here on purpose.
	// The undrawn half of ③ is NOT missing evidence — W measured two
	// nibbles and the manual labels both — it is an UNMAPPED FIELD, the
	// split flag, which lives under the layout's Fixed template so that a
	// driver can see a Split-ON record and refuse to write it back (D14,
	// enablers E6). A reader must not mistake this test for a
	// contradiction between the leg and the layout.
	insets := map[string]map[string]string{}
	for _, r := range all {
		switch r["diagram_id"] {
		case "D2", "D3":
			insets[r["diagram_id"]] = r
		}
	}
	for _, d := range []struct{ id, key string }{{"D2", "③"}, {"D3", "⑪"}} {
		r, ok := insets[d.id]
		if !ok {
			t.Errorf("W has no %s row — the nibble inset for %s is what fixes the two halves", d.id, d.key)
			continue
		}
		if got := normaliseKey(r["field_index"]); got != d.key {
			t.Errorf("W's %s row names %q, want %q", d.id, got, d.key)
		}
		if fb, lb := num(r, "first_byte"), num(r, "last_byte"); fb != 1 || lb != 1 {
			t.Errorf("W's %s row spans bytes %d..%d, want 1..1 — an inset draws ONE cell", d.id, fb, lb)
		}
		if fn, ln := num(r, "first_nibble"), num(r, "last_nibble"); fn != 1 || ln != 2 {
			t.Errorf("W's %s row spans nibbles %d..%d, want 1..2 — the dotted mid-point rule is the only nibble division printed", d.id, fn, ln)
		}
	}

	layout, ok := ic7300.Profile().LayoutFor(39)
	if !ok {
		t.Fatal("LayoutFor(39) missing")
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
	// ③ at offset 0: ONE mapped nibble, the LOW one, carrying SELECT.
	third := nibbles(0)
	if len(third) != 1 {
		t.Fatalf("the profile maps %d spans at record byte 0 (③), want exactly 1 — W measured TWO drawn nibbles there and the profile maps ONE of them, the high half being the UNMAPPED split flag", len(third))
	}
	if third[0].Field != civ.FieldSelect || third[0].Nibble != civ.NibbleLow || third[0].Length != 1 {
		t.Errorf("record byte 0 carries {%s len=%d nib=%v}, want {select len=1 NibbleLow} — the SELECT half is mapped and the SPLIT half is not (D14; the whole-byte enum of REV 1 is overruled)", third[0].Field, third[0].Length, third[0].Nibble)
	}
	if len(layout.Fixed) != layout.Length {
		t.Fatalf("len(Fixed) = %d, want %d — the template is what DECLARES ③'s high nibble unmapped, and without it the asymmetry above would be an omission rather than a contract", len(layout.Fixed), layout.Length)
	}
	if layout.Fixed[0]&0xF0 != 0x00 {
		t.Errorf("Fixed[0] = %#02x — the unmapped high nibble must be 0x0, Split OFF: it is what each driver's E6 check compares a just-read record against", layout.Fixed[0])
	}
	// ⑪ at offset 8: BOTH nibbles mapped, DATA high and TONE low.
	eleventh := nibbles(8)
	if len(eleventh) != 2 {
		t.Fatalf("the profile maps %d spans at record byte 8 (⑪), want exactly 2 — W measured two drawn nibbles and both are mapped", len(eleventh))
	}
	if eleventh[0].Field != civ.FieldDataMode || eleventh[0].Nibble != civ.NibbleHigh {
		t.Errorf("record byte 8's first span is {%s %v}, want {data_mode NibbleHigh} — the LEFT (high) nibble reaches \"0=Data mode OFF / 1=Data mode ON\" (W hazard (c): the legends' printed top-to-bottom order is the REVERSE of the nibbles' left-to-right order)", eleventh[0].Field, eleventh[0].Nibble)
	}
	if eleventh[1].Field != civ.FieldToneMode || eleventh[1].Nibble != civ.NibbleLow {
		t.Errorf("record byte 8's second span is {%s %v}, want {tone_mode NibbleLow} — the RIGHT (low) nibble reaches \"0: OFF, 1: TONE, 2: TSQL\"", eleventh[1].Field, eleventh[1].Nibble)
	}
}
