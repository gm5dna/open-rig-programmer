// SPDX-License-Identifier: GPL-3.0-or-later

package ic7200_test

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7200"
)

// legalRecord is a record every mapped field of which is inside what the
// printed diagrams admit: 14.250000 MHz is inside the ④~⑧/❹~❽ digit
// domain (matrix §1 rows 13-14), and the TX-duplicate frequency is set
// EQUAL to the RX one, matching the manual's own NOTE (matrix §3.11:
// "we recommend that you set the same data as ④–⑪").
func legalRecord() civ.MemoryRecord {
	return civ.MemoryRecord{
		Address:  civ.ChannelAddress{Channel: 1},
		RXFreqHz: civ.Available(uint64(14_250_000)),
		Mode:     civ.Available("USB"),
		Filter:   civ.Available("Wide"),
		DataMode: civ.Available("ON"),
		TXFreqHz: civ.Available(uint64(14_250_000)),
	}
}

func TestGeometryAndFixedRegions(t *testing.T) {
	p := ic7200.Profile()
	cmd, err := p.BuildMemorySet(legalRecord())
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{
		0xfe, 0xfe, 0x76, 0xe0, 0x1a, 0x00, 0x00, 0x01,
		0x00,                         // ③ Split, UNMAPPED, template zero
		0x00, 0x00, 0x25, 0x14, 0x00, // ④~⑧ RX frequency, little-endian packed BCD
		0x01,                         // ⑨ mode USB
		0x01,                         // ⑩ filter Wide
		0x10,                         // ⑪ data mode ON (a FULL BYTE 0x10, not a nibble)
		0x00, 0x00, 0x25, 0x14, 0x00, // ❹~❽ TX frequency, same convention
		0x00, // ❾ TX mode mirror, UNMAPPED, template zero
		0x00, // ❿ TX filter mirror, UNMAPPED, template zero
		0x00, // ⓫ TX data-mode mirror, UNMAPPED, template zero
		0xfd,
	}
	if got := cmd.Bytes(); string(got) != string(want) {
		t.Errorf("BuildMemorySet = % X, want % X", got, want)
	}
	for _, b := range p.Layouts()[0].Fixed {
		if b != 0 {
			t.Fatalf("fixed template contains %#x", b)
		}
	}
	for _, sp := range p.Layouts()[0].Fields {
		if sp.Offset == ic7200.SplitOffset {
			t.Errorf("Split byte (offset %d) is mapped by %s; matrix §3.15(a) leaves it UNMAPPED", ic7200.SplitOffset, sp.Field)
		}
		for _, unmapped := range []int{ic7200.TXModeOffset, ic7200.TXFilterOffset, ic7200.TXDataModeOffset} {
			if sp.Offset == unmapped {
				t.Errorf("TX-duplicate mirror byte (offset %d) is mapped by %s; matrix §1b/§3.16 ADDED-1 leaves it UNMAPPED", unmapped, sp.Field)
			}
		}
	}

	// Parse the built frame back and confirm the round trip, including
	// that the four unmapped bytes decode to Unavailable — an unmapped
	// region is not decoded (the same E6-style consequence every sibling
	// Icom package in this tier documents).
	rec, err := p.ParseMemoryAnswer(append([]byte{0xfe, 0xfe, 0xe0, 0x76, 0x1a, 0x00, 0x00, 0x01}, append(want[8:len(want)-1], 0xfd)...))
	if err != nil {
		t.Fatalf("ParseMemoryAnswer: %v", err)
	}
	golden := legalRecord()
	if rec.RXFreqHz != golden.RXFreqHz || rec.TXFreqHz != golden.TXFreqHz {
		t.Errorf("parsed frequencies = rx %v tx %v, want rx %v tx %v", rec.RXFreqHz, rec.TXFreqHz, golden.RXFreqHz, golden.TXFreqHz)
	}
	if rec.Mode != golden.Mode || rec.Filter != golden.Filter || rec.DataMode != golden.DataMode {
		t.Errorf("parsed mode/filter/data_mode = %v/%v/%v, want %v/%v/%v", rec.Mode, rec.Filter, rec.DataMode, golden.Mode, golden.Filter, golden.DataMode)
	}
	if !rec.Name.Unavailable() {
		t.Errorf("parsed Name = %v, want Unavailable — this record has no name field (NoTag)", rec.Name)
	}
}

// TestGateRefusesNonZeroUnmappedBytes is this model's half of the
// tier-wide E6-style rule: the gate's re-encode leg (spec Erratum 2)
// decodes, re-validates and re-encodes byte-identically, so a set frame
// carrying a non-zero Split byte or TX-mirror byte cannot survive it.
func TestGateRefusesNonZeroUnmappedBytes(t *testing.T) {
	p := ic7200.Profile()
	cmd, err := p.BuildMemorySet(legalRecord())
	if err != nil {
		t.Fatal(err)
	}
	base := cmd.Bytes()
	if !p.AllowedCommand(base) {
		t.Fatalf("the builder's own frame % X is refused by its own gate", base)
	}
	// The record starts after FE FE <to> <from> 1A 00 and the two
	// selector bytes, so record offset N is frame index N+8.
	const recordBase = 8
	for _, tc := range []struct {
		offset int
		name   string
	}{
		{ic7200.SplitOffset, "Split (③)"},
		{ic7200.TXModeOffset, "TX mode mirror (❾)"},
		{ic7200.TXFilterOffset, "TX filter mirror (❿)"},
		{ic7200.TXDataModeOffset, "TX data-mode mirror (⓫)"},
	} {
		frame := append([]byte(nil), base...)
		frame[recordBase+tc.offset] = 0x10
		if p.AllowedCommand(frame) {
			t.Errorf("the gate admitted a set carrying 0x10 in %s: % X", tc.name, frame)
		}
	}
}
