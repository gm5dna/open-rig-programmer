// SPDX-License-Identifier: GPL-3.0-or-later

package ic7200_test

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7200"
)

func TestProfileShape(t *testing.T) {
	p := ic7200.Profile()
	if !p.Configured() {
		t.Fatal("Profile is not configured")
	}
	if got := p.Model(); got != "IC-7200" {
		t.Errorf("Model() = %q, want IC-7200", got)
	}
	if got := p.RadioAddress(); got != 0x76 {
		t.Errorf("RadioAddress() = %#x, want 0x76 (matrix §1 row 2)", got)
	}
	if got := p.ControllerAddress(); got != 0xe0 {
		t.Errorf("ControllerAddress() = %#x, want 0xe0", got)
	}
	if got := p.AddressForm(); got != civ.AddressFormFlat {
		t.Errorf("AddressForm() = %v, want flat", got)
	}
	lo, hi := p.ChannelRange()
	if lo != 1 || hi != 201 {
		t.Errorf("ChannelRange() = %d..%d, want 1..201 (matrix §1 row 4: 199 memories + P1/P2)", lo, hi)
	}
	if p.NameLength() != 0 {
		t.Errorf("NameLength() = %d, want 0 (NoTag, matrix §1 row 6)", p.NameLength())
	}
	if p.Discriminator() != civ.DiscriminatorSingleLength {
		t.Errorf("Discriminator() = %v, want single length", p.Discriminator())
	}
	if got := p.RecordLengths(); len(got) != 1 || got[0] != ic7200.RecordOnlyLength {
		t.Errorf("RecordLengths() = %v, want [%d]", got, ic7200.RecordOnlyLength)
	}
	if p.BuildRecordLength() != ic7200.RecordOnlyLength {
		t.Errorf("BuildRecordLength() = %d, want %d", p.BuildRecordLength(), ic7200.RecordOnlyLength)
	}
}

func TestProfileIsDefensive(t *testing.T) {
	p := ic7200.Profile()
	layouts := p.Layouts()
	layouts[0].Fields[0].Offset = 99
	layouts[0].Fixed[0] = 0xff
	if got := ic7200.Profile().Layouts()[0].Fields[0].Offset; got != 1 {
		t.Errorf("Profile layout was mutated through copy: offset %d", got)
	}
	if got := ic7200.Profile().Layouts()[0].Fixed[0]; got != 0 {
		t.Errorf("Profile template was mutated through copy: %#x", got)
	}
}
