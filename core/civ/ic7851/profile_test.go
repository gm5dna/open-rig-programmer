// SPDX-License-Identifier: GPL-3.0-or-later

package ic7851_test

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7851"
)

func TestProfileShape(t *testing.T) {
	p := ic7851.Profile()
	if !p.Configured() {
		t.Fatal("Profile is not configured")
	}
	if got := p.Model(); got != "IC-7851" {
		t.Errorf("Model() = %q, want IC-7851", got)
	}
	if got := p.RadioAddress(); got != 0x8e {
		t.Errorf("RadioAddress() = %#x, want 0x8e", got)
	}
	if got := p.ControllerAddress(); got != 0xe0 {
		t.Errorf("ControllerAddress() = %#x, want 0xe0", got)
	}
	if got := p.AddressForm(); got != civ.AddressFormFlat {
		t.Errorf("AddressForm() = %v, want flat", got)
	}
	lo, hi := p.ChannelRange()
	if lo != 1 || hi != 101 {
		t.Errorf("ChannelRange() = %d..%d, want 1..101", lo, hi)
	}
	if p.NameLength() != 10 {
		t.Errorf("NameLength() = %d, want 10", p.NameLength())
	}
	if p.NamePad() != 0x20 {
		t.Errorf("NamePad() = %#x, want 0x20", p.NamePad())
	}
	if p.Discriminator() != civ.DiscriminatorSingleLength {
		t.Errorf("Discriminator() = %v, want single length", p.Discriminator())
	}
	if got := p.RecordLengths(); len(got) != 1 || got[0] != 25 {
		t.Errorf("RecordLengths() = %v, want [25]", got)
	}
	if p.BuildRecordLength() != 25 {
		t.Errorf("BuildRecordLength() = %d, want 25", p.BuildRecordLength())
	}
}

func TestProfileIsDefensive(t *testing.T) {
	p := ic7851.Profile()
	layouts := p.Layouts()
	layouts[0].Fields[0].Offset = 99
	layouts[0].Fixed[0] = 0xff
	if got := ic7851.Profile().Layouts()[0].Fields[0].Offset; got != 1 {
		t.Errorf("Profile layout was mutated through copy: offset %d", got)
	}
	if got := ic7851.Profile().Layouts()[0].Fixed[0]; got != 0 {
		t.Errorf("Profile template was mutated through copy: %#x", got)
	}
}
