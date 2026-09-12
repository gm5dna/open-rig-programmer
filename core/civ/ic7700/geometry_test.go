// SPDX-License-Identifier: GPL-3.0-or-later

package ic7700_test

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7700"
)

// TestGeometry_MatchesTheMatrixTable pins every FieldSpan's offset, length
// and nibble against matrix §3.15's byte-offset record layout table,
// mechanically rather than by re-reading the source. A drift here is a
// STOP: either this test or profile.go has the table wrong.
func TestGeometry_MatchesTheMatrixTable(t *testing.T) {
	layout, ok := ic7700.Profile().LayoutFor(ic7700.RecordOnlyLength)
	if !ok {
		t.Fatalf("no layout for length %d", ic7700.RecordOnlyLength)
	}
	want := []struct {
		field  civ.FieldID
		offset int
		length int
		nibble civ.NibbleSel
	}{
		{civ.FieldRXFrequency, 1, 5, civ.NibbleWhole},
		{civ.FieldMode, 6, 1, civ.NibbleWhole},
		{civ.FieldFilter, 7, 1, civ.NibbleWhole},
		{civ.FieldToneMode, 8, 1, civ.NibbleLow},
		{civ.FieldToneTX, 9, 3, civ.NibbleWhole},
		{civ.FieldToneRX, 12, 3, civ.NibbleWhole},
		{civ.FieldTXFrequency, 15, 5, civ.NibbleWhole},
		{civ.FieldName, 29, 10, civ.NibbleWhole},
	}
	if len(layout.Fields) != len(want) {
		t.Fatalf("layout has %d FieldSpans, want %d: %+v", len(layout.Fields), len(want), layout.Fields)
	}
	for i, w := range want {
		got := layout.Fields[i]
		if got.Field != w.field || got.Offset != w.offset || got.Length != w.length || got.Nibble != w.nibble {
			t.Errorf("Fields[%d] = %+v, want field=%s offset=%d length=%d nibble=%v", i, got, w.field, w.offset, w.length, w.nibble)
		}
	}

	// The UNMAPPED regions: idx0 (whole byte), idx8 high nibble, idx20-28
	// (9 bytes) — matrix §3.15(1), §1b, coordinator ruling 12/09/2026.
	// Every FieldSpan above must leave these bytes untouched.
	mapped := make([]bool, ic7700.RecordOnlyLength)
	for _, sp := range layout.Fields {
		for b := sp.Offset; b < sp.Offset+sp.Length; b++ {
			mapped[b] = true
		}
	}
	for _, b := range []int{ic7700.SplitSelectByteOffset} {
		if mapped[b] {
			t.Errorf("byte %d is mapped, want fully unmapped (E6)", b)
		}
	}
	// idx8 has a mapped LOW nibble (tone_mode) but must not be claimed by
	// a second, whole-byte span.
	if !mapped[ic7700.DataModeNibbleOffset] {
		t.Errorf("byte %d (tone_mode's byte) is unmapped, want its low nibble mapped", ic7700.DataModeNibbleOffset)
	}
	for b := ic7700.TXDupUnmappedOffset; b < ic7700.TXDupUnmappedOffset+ic7700.TXDupUnmappedLength; b++ {
		if mapped[b] {
			t.Errorf("byte %d is mapped, want fully unmapped (TX-dup block, no neutral field)", b)
		}
	}

	// The record tiles exactly: every byte is either mapped or one of the
	// three unmapped regions, no gap, no overlap.
	unmapped := 0
	for b := 0; b < ic7700.RecordOnlyLength; b++ {
		if !mapped[b] {
			unmapped++
		}
	}
	// idx0 (1) + idx20-28 (9) = 10 fully unmapped bytes. idx8's high
	// nibble is unmapped but the byte itself counts as "mapped" above
	// (its low nibble is), so it is not part of this count.
	if unmapped != 1+ic7700.TXDupUnmappedLength {
		t.Errorf("record has %d fully-unmapped bytes, want %d", unmapped, 1+ic7700.TXDupUnmappedLength)
	}
}
