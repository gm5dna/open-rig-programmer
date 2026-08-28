// SPDX-License-Identifier: GPL-3.0-or-later

package ic705_test

import (
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic705"
)

// Task 4 checks that the three legs agree WITH EACH OTHER and with the
// field table. This file binds the profile to ONE leg alone: the geometry
// witness, the artefact that counted cells on a raster rather than reading
// the diagram's printed numerals.
//
// The distinction matters because the printed numerals are wrong as
// positions past the duplicated block, and a leg that read them would have
// been consistent with the other two and still put the memory name
// forty-seven bytes early. The witness is the only leg that MEASURED, so
// it is the only one that can catch that, and this file is where a future
// change to either the witness or the layout fires.

// witnessD1 returns the witness's record-band rows: the D1 diagram alone,
// never the one-byte nibble insets D2/D3/D4, whose "byte 1" is a byte of
// their own little box rather than of the record.
func witnessD1(t *testing.T) []witnessRow {
	t.Helper()
	var out []witnessRow
	for _, r := range loadWitness(t) {
		if r.Diagram == "D1" {
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		t.Fatal("no D1 rows in the geometry witness — every assertion in this file would be vacuous")
	}
	return out
}

func TestGeometryMatchesTheWitness(t *testing.T) {
	layout, ok := ic705.Profile().LayoutFor(111)
	if !ok {
		t.Fatal("the profile has no 111-byte layout")
	}

	consumed, spanned, unmapped, address := 0, 0, 0, 0
	for _, w := range witnessD1(t) {
		claims, known := layoutClaims[w.Index]
		if !known {
			t.Errorf("witness row %q (data-area %d..%d) is CONSUMED BY NOTHING — a measured row must be matched to a span or declared unmapped, or the layout has a region no one has looked at",
				w.Index, w.FirstByte, w.LastByte)
			continue
		}
		consumed++
		if claims == nil {
			// The four address bytes: measured by the witness, but not
			// part of the record at all. civ encodes them from the
			// ChannelAddress, which is why the profile's layout has
			// nothing to say about data-area bytes 1..4.
			address++
			if w.FirstByte > 4 {
				t.Errorf("witness row %q measures data-area %d..%d but is declared address-only — only bytes 1..4 are the address", w.Index, w.FirstByte, w.LastByte)
			}
			continue
		}

		// The claims must TILE what the witness measured for this printed
		// index — every nibble of the run claimed exactly once. Counted
		// per NIBBLE rather than per byte because two of these rows are a
		// single byte carrying two enums, and byte arithmetic would
		// either double-count them or leave a half-byte unaccounted for.
		wantOffset := w.FirstByte - 5
		wantEnd := w.LastByte - 5
		type half struct {
			off  int
			high bool
		}
		claimed := map[half]bool{}
		mark := func(off int, high bool) {
			k := half{off, high}
			if claimed[k] {
				t.Errorf("witness row %q: record offset %d's %s nibble is claimed twice", w.Index, off, nibbleName(high))
			}
			claimed[k] = true
			if off < wantOffset || off > wantEnd {
				t.Errorf("witness row %q measures record offsets %d..%d, but a claim reaches offset %d", w.Index, wantOffset, wantEnd, off)
			}
		}
		for _, c := range claims {
			for off := c.Offset; off < c.Offset+c.Length; off++ {
				switch c.Nibble {
				case civ.NibbleHigh:
					mark(off, true)
				case civ.NibbleLow:
					mark(off, false)
				default:
					mark(off, true)
					mark(off, false)
				}
			}
			if c.Field == "" {
				unmapped++
				continue
			}
			spanned++
			if !hasSpan(layout, c) {
				t.Errorf("witness row %q measures data-area %d..%d, so %s belongs at record offset %d length %d (%v) — the layout has no such span",
					w.Index, w.FirstByte, w.LastByte, c.Field, c.Offset, c.Length, c.Nibble)
			}
		}
		for off := wantOffset; off <= wantEnd; off++ {
			for _, high := range []bool{true, false} {
				if !claimed[half{off, high}] {
					t.Errorf("witness row %q measures record offsets %d..%d, but offset %d's %s nibble is claimed by nothing",
						w.Index, wantOffset, wantEnd, off, nibbleName(high))
				}
			}
		}
	}
	if consumed == 0 || spanned == 0 || unmapped == 0 || address == 0 {
		t.Fatalf("consumed %d rows, %d mapped claims, %d unmapped claims, %d address rows — every count must be non-zero or this test passed over an empty set",
			consumed, spanned, unmapped, address)
	}
}

func TestAddressWidthMatchesTheWitness(t *testing.T) {
	// The witness measures "①, ②" at data-area 1..2 and "③, ④" at 3..4,
	// so the address is FOUR bytes. Spec Erratum 1's number, MEASURED
	// here rather than quoted.
	var last int
	seen := map[string]bool{}
	for _, w := range witnessD1(t) {
		switch w.Index {
		case "①, ②", "③, ④":
			seen[w.Index] = true
			if w.LastByte > last {
				last = w.LastByte
			}
		}
	}
	if len(seen) != 2 {
		t.Fatalf("found %d of the two address rows in the witness — the artefact no longer carries the rows this test reads", len(seen))
	}
	if last != 4 {
		t.Fatalf("the two address rows end at data-area byte %d, want 4", last)
	}

	// A four-byte address makes the read request 7 + 4 = 11 bytes: FE FE,
	// the two address bytes, 1A 00, the four address bytes, FD.
	cmd, err := ic705.Profile().BuildMemoryRead(civ.ChannelAddress{Group: 0, Channel: 12})
	if err != nil {
		t.Fatalf("BuildMemoryRead: %v", err)
	}
	if got := len(cmd.Bytes()); got != 7+last {
		t.Errorf("the read request is %d bytes, want %d — 7 of framing and command plus the %d address bytes the witness measures", got, 7+last, last)
	}
}

func TestRecordLengthIsTheWitnessMinusTheAddress(t *testing.T) {
	// The whole of Erratum 1's convention, asserted rather than restated:
	// the data area is what the witness measures, the record is what the
	// profile declares, and the difference is the address.
	max := 0
	for _, w := range witnessD1(t) {
		if w.LastByte > max {
			max = w.LastByte
		}
	}
	if max != 115 {
		t.Fatalf("the witness's last measured data-area byte is %d, want 115", max)
	}
	lengths := ic705.Profile().RecordLengths()
	if len(lengths) != 1 {
		t.Fatalf("the profile accepts %d record lengths, want exactly 1", len(lengths))
	}
	if want := max - 4; lengths[0] != want {
		t.Errorf("the profile's accepted record length is %d, want %d = the witness's %d-byte data area less its four address bytes", lengths[0], want, max)
	}
	if ic705.Profile().AcceptsRecordLength(max) {
		t.Errorf("the profile accepts a %d-byte record — that is the DATA AREA, and a profile declaring it would give the probe's length fingerprint the wrong model's number", max)
	}
}

func TestPrintedIndicesAreNotPositions(t *testing.T) {
	// The witness's STOP 3, pinned. The diagram's final bracket prints
	// "53~68" over cells the witness measures at 100..115. A layout that
	// trusted the printed label would put the memory name forty-seven
	// bytes early — inside the duplicated TX block — and this test is
	// what stops that shipping.
	var row *witnessRow
	rows := witnessD1(t)
	for i := range rows {
		if rows[i].Index == "53~68" {
			row = &rows[i]
		}
	}
	if row == nil {
		t.Fatal(`the witness no longer carries the row printed "53~68" — this test reads that row and nothing else`)
	}
	if row.FirstByte != 100 || row.LastByte != 115 {
		t.Fatalf(`the row printed "53~68" measures data-area %d..%d, want 100..115`, row.FirstByte, row.LastByte)
	}
	if !strings.Contains(row.Notes, "STOP 3") {
		t.Errorf(`the row printed "53~68" no longer carries the witness's STOP 3 note, which is the record of WHY the printed label and the measurement disagree`)
	}

	layout, ok := ic705.Profile().LayoutFor(111)
	if !ok {
		t.Fatal("the profile has no 111-byte layout")
	}
	measured := row.FirstByte - 5 // 95
	printed := 53 - 5             // 48 — where the printed label would put it
	for _, sp := range layout.Fields {
		if sp.Field != civ.FieldName {
			continue
		}
		if sp.Offset == printed {
			t.Errorf("the name span sits at record offset %d, which is where the PRINTED index would put it — the witness measures it at %d", printed, measured)
		}
		if sp.Offset != measured {
			t.Errorf("the name span sits at record offset %d, want %d (data-area %d..%d)", sp.Offset, measured, row.FirstByte, row.LastByte)
		}
	}
}

func TestNibbleClaimsMatchTheWitness(t *testing.T) {
	layout, ok := ic705.Profile().LayoutFor(111)
	if !ok {
		t.Fatal("the profile has no 111-byte layout")
	}
	if len(layout.Fixed) != 111 {
		t.Fatalf("the layout's Fixed template is %d bytes, want 111 — the template describes the whole record or none of it", len(layout.Fixed))
	}

	// D2, D3 and D4 are one-byte nibble legends. Each measures its own
	// single box as nibbles 1..2, and its note names the D1 byte it
	// expands.
	type nibbleCase struct {
		diagram      string
		dataAreaByte int
		high, low    civ.FieldID
	}
	cases := []nibbleCase{
		{"D2", 5, "", ""},
		{"D3", 14, civ.FieldDuplex, civ.FieldToneMode},
		{"D4", 15, "", ""},
	}

	seen := 0
	for _, c := range cases {
		var row *witnessRow
		rows := loadWitness(t)
		for i := range rows {
			if rows[i].Diagram == c.diagram {
				row = &rows[i]
			}
		}
		if row == nil {
			t.Errorf("the witness no longer carries diagram %s", c.diagram)
			continue
		}
		seen++
		if row.FirstNib != 1 || row.LastNib != 2 {
			t.Errorf("%s measures nibbles %d..%d, want 1..2", c.diagram, row.FirstNib, row.LastNib)
		}
		off := c.dataAreaByte - 5
		if got := spanAt(layout, off, civ.NibbleHigh); got != c.high {
			t.Errorf("%s (record offset %d): the HIGH nibble is claimed by %q, want %q", c.diagram, off, got, c.high)
		}
		if got := spanAt(layout, off, civ.NibbleLow); got != c.low {
			t.Errorf("%s (record offset %d): the LOW nibble is claimed by %q, want %q", c.diagram, off, got, c.low)
		}
		if c.high == "" && c.low == "" && layout.Fixed[off] != 0x00 {
			t.Errorf("%s (record offset %d): neither nibble is mapped, so the template decides the byte, and it holds %#02x rather than 0x00", c.diagram, off, layout.Fixed[off])
		}
	}
	if seen != len(cases) {
		t.Fatalf("matched %d of %d nibble diagrams", seen, len(cases))
	}

	// D4's SECOND nibble is not an enum value at all: the witness records
	// that the cell prints "X 0" rather than "X X", and that the inset
	// labels that nibble "Fixed". The layout must leave it to the
	// template, which it does by leaving the WHOLE byte there — the first
	// nibble's digital-squelch codes have no neutral field to land in.
	var d4 *witnessRow
	rows := loadWitness(t)
	for i := range rows {
		if rows[i].Diagram == "D4" {
			d4 = &rows[i]
		}
	}
	if d4 == nil {
		t.Fatal("no D4 row")
	}
	if !strings.Contains(d4.Notes, "Fixed") {
		t.Errorf("the D4 row no longer records the literal 'Fixed' label on its second nibble, which is the evidence that the nibble is not an enum")
	}
}

// nibbleName is for failure messages: "high" or "low".
func nibbleName(high bool) string {
	if high {
		return "high"
	}
	return "low"
}
