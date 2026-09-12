// SPDX-License-Identifier: GPL-3.0-or-later

package ic7410_test

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7410"
)

// TestGeometryMatchesTheMatrix pins core/civ/ic7410's layout against
// docs/superpowers/icom-matrices/ic7410-capability-matrix.md §1b's offset
// table, byte for byte. This is a SINGLE-SOURCE check — this package's own
// implementer reading the matrix a second time — and not an independent
// blind transcription; a disagreement here is a bug in this package, to be
// fixed by re-reading the matrix, never by editing the matrix.
func TestGeometryMatchesTheMatrix(t *testing.T) {
	want := map[civ.FieldID]struct {
		offset, length int
		nibble         civ.NibbleSel
		order          civ.ByteOrder
	}{
		civ.FieldRXFrequency: {1, 5, civ.NibbleWhole, civ.OrderLittleEndian},
		civ.FieldMode:        {6, 1, civ.NibbleWhole, 0},
		civ.FieldFilter:      {7, 1, civ.NibbleWhole, 0},
		civ.FieldDataMode:    {8, 1, civ.NibbleWhole, 0},
		civ.FieldToneMode:    {9, 1, civ.NibbleHigh, 0},
		civ.FieldToneTX:      {10, 3, civ.NibbleWhole, civ.OrderBigEndian},
		civ.FieldToneRX:      {13, 3, civ.NibbleWhole, civ.OrderBigEndian},
		civ.FieldTXFrequency: {16, 5, civ.NibbleWhole, civ.OrderLittleEndian},
		civ.FieldName:        {31, 9, civ.NibbleWhole, 0},
	}

	layout := ic7410.Profile().Layouts()
	if len(layout) != 1 {
		t.Fatalf("Layouts() has %d entries, want 1 (DiscriminatorSingleLength)", len(layout))
	}
	if layout[0].Length != ic7410.RecordOnlyLength {
		t.Fatalf("layout length = %d, want %d (matrix §3.11: 1+5+2+1+1+3+3+15+9 = 40)", layout[0].Length, ic7410.RecordOnlyLength)
	}

	seen := map[civ.FieldID]bool{}
	for _, sp := range layout[0].Fields {
		w, ok := want[sp.Field]
		if !ok {
			t.Errorf("layout carries an unexpected span for %s", sp.Field)
			continue
		}
		seen[sp.Field] = true
		if sp.Offset != w.offset || sp.Length != w.length {
			t.Errorf("%s: offset/length = %d/%d, want %d/%d (matrix §1b)", sp.Field, sp.Offset, sp.Length, w.offset, w.length)
		}
		if sp.Nibble != w.nibble {
			t.Errorf("%s: nibble = %v, want %v", sp.Field, sp.Nibble, w.nibble)
		}
		if sp.Encoding == civ.EncodingBCDNumber && sp.Order != w.order {
			t.Errorf("%s: byte order = %v, want %v", sp.Field, sp.Order, w.order)
		}
	}
	for id := range want {
		if !seen[id] {
			t.Errorf("layout carries no span for %s, which the matrix maps", id)
		}
	}

	// The two UNMAPPED regions: SelectSplitOffset (record byte 0, the whole
	// byte) and the ten TX-duplicate-block bytes at TXDupUnmappedOffset.
	// Neither may be claimed by any span — matrix §1b's decomposition
	// table, "UNMAPPED — neither nibble has a neutral home" (offset 0) and
	// "UNMAPPED — no neutral field for a per-channel TX-side <x>" (offsets
	// 21-30).
	unmapped := map[int]bool{ic7410.SelectSplitOffset: true}
	for i := 0; i < ic7410.TXDupUnmappedLength; i++ {
		unmapped[ic7410.TXDupUnmappedOffset+i] = true
	}
	for _, sp := range layout[0].Fields {
		for off := sp.Offset; off < sp.Offset+sp.Length; off++ {
			if unmapped[off] {
				t.Errorf("span %s at offset %d claims record byte %d, which the matrix leaves UNMAPPED", sp.Field, sp.Offset, off)
			}
		}
	}
	if len(unmapped) != 11 {
		t.Fatalf("this test's own unmapped set has %d entries, want 11 (1 select/split byte + 10 TX-duplicate-block bytes)", len(unmapped))
	}

	// Every byte 0..39 is EITHER covered by exactly one span OR is one of
	// the eleven UNMAPPED bytes above — the record must tile completely,
	// with no gap and no double claim.
	covered := make([]int, ic7410.RecordOnlyLength)
	for _, sp := range layout[0].Fields {
		for off := sp.Offset; off < sp.Offset+sp.Length; off++ {
			covered[off]++
		}
	}
	for off := 0; off < ic7410.RecordOnlyLength; off++ {
		switch {
		case unmapped[off] && covered[off] != 0:
			t.Errorf("byte %d is both UNMAPPED and claimed by %d span(s)", off, covered[off])
		case !unmapped[off] && covered[off] != 1:
			t.Errorf("byte %d is claimed by %d spans, want exactly 1 (it is not one of the eleven UNMAPPED bytes)", off, covered[off])
		}
	}
}

// TestFixedTemplateIsAllZero pins that every unmapped byte's Fixed-template
// value is zero, as the geometry test above assumes.
func TestFixedTemplateIsAllZero(t *testing.T) {
	tmpl := ic7410.FixedTemplate()
	if len(tmpl) != ic7410.RecordOnlyLength {
		t.Fatalf("FixedTemplate() has %d bytes, want %d", len(tmpl), ic7410.RecordOnlyLength)
	}
	for i, b := range tmpl {
		if b != 0 {
			t.Errorf("FixedTemplate()[%d] = %#02x, want 0x00", i, b)
		}
	}
}

// TestModeFilterDataModeToneModeEnums pins the four enum vocabularies
// against the matrix, exactly — a code the page prints and the profile
// drops fails to decode a real record; a code the profile invents is a
// radio claim this document does not make.
func TestModeFilterDataModeToneModeEnums(t *testing.T) {
	layout := ic7410.Profile().Layouts()[0]
	byField := map[civ.FieldID]civ.FieldSpan{}
	for _, sp := range layout.Fields {
		byField[sp.Field] = sp
	}

	for _, tc := range []struct {
		field civ.FieldID
		want  map[byte]string
	}{
		{civ.FieldMode, map[byte]string{
			0x00: "LSB", 0x01: "USB", 0x02: "AM", 0x03: "CW", 0x04: "RTTY",
			0x05: "FM", 0x07: "CW-R", 0x08: "RTTY-R",
		}},
		{civ.FieldFilter, map[byte]string{0x01: "FIL1", 0x02: "FIL2", 0x03: "FIL3"}},
		{civ.FieldDataMode, map[byte]string{0x00: "OFF", 0x01: "ON"}},
		{civ.FieldToneMode, map[byte]string{0x0: "OFF", 0x1: "TONE", 0x2: "TSQL"}},
	} {
		got := byField[tc.field].Enum
		if len(got) != len(tc.want) {
			t.Errorf("%s enum has %d entries, want %d: got %v, want %v", tc.field, len(got), len(tc.want), got, tc.want)
			continue
		}
		for k, v := range tc.want {
			if got[k] != v {
				t.Errorf("%s enum[%#02x] = %q, want %q", tc.field, k, got[k], v)
			}
		}
	}
}
