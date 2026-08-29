// SPDX-License-Identifier: GPL-3.0-or-later

package ic7851_test

import (
	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7851"
	"testing"
)

func TestGeometryAndFixedRegions(t *testing.T) {
	p := ic7851.Profile()
	rec := civ.MemoryRecord{
		Address:  civ.ChannelAddress{Channel: 1},
		RXFreqHz: civ.Available(uint64(145500000)), Mode: civ.Available("USB"), Filter: civ.Available("FIL2"),
		ToneMode: civ.Available("TONE"), ToneTXDeciHz: civ.Available(uint64(885)), ToneRXDeciHz: civ.Available(uint64(1000)),
		Name: civ.Available("ALPHA 1"),
	}
	cmd, err := p.BuildMemorySet(rec)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0xfe, 0xfe, 0x8e, 0xe0, 0x1a, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x50, 0x45, 0x01, 0x01, 0x02, 0x01, 0x00, 0x08, 0x85, 0x00, 0x10, 0x00, 0x41, 0x4c, 0x50, 0x48, 0x41, 0x20, 0x31, 0x20, 0x20, 0x20, 0xfd}
	if got := cmd.Bytes(); string(got) != string(want) {
		t.Errorf("BuildMemorySet = % X, want % X", got, want)
	}
	for _, b := range p.Layouts()[0].Fixed {
		if b != 0 {
			t.Fatalf("fixed template contains %#x", b)
		}
	}
	for _, sp := range p.Layouts()[0].Fields {
		if sp.Offset == ic7851.SelectNibbleOffset {
			t.Errorf("select byte is mapped by %s", sp.Field)
		}
	}
	if p.Layouts()[0].Fields[3].Nibble != civ.NibbleLow {
		t.Error("tone mode is not the low nibble")
	}
}
