// SPDX-License-Identifier: GPL-3.0-or-later

package ic7760_test

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7760"
)

func TestProfileShape(t *testing.T) {
	p := ic7760.Profile()
	if !p.Configured() {
		t.Fatal("Profile is not configured")
	}
	if got := p.Model(); got != "IC-7760" {
		t.Errorf("Model() = %q, want IC-7760", got)
	}
	if got := p.RadioAddress(); got != 0xB2 {
		t.Errorf("RadioAddress() = %#02x, want 0xB2", got)
	}
	if got := p.ControllerAddress(); got != 0xE0 {
		t.Errorf("ControllerAddress() = %#02x, want 0xE0", got)
	}
	if got := p.AddressForm(); got != civ.AddressFormFlat {
		t.Errorf("AddressForm() = %v, want flat", got)
	}
	if lo, hi := p.ChannelRange(); lo != 1 || hi != 99 {
		t.Errorf("ChannelRange() = %d..%d, want base MEM range 1..99", lo, hi)
	}
	if got := p.Discriminator(); got != civ.DiscriminatorSingleLength {
		t.Errorf("Discriminator() = %v, want single length", got)
	}
	if got := p.RecordLengths(); len(got) != 1 || got[0] != 25 {
		t.Errorf("RecordLengths() = %v, want [25]", got)
	}
	if got := p.BuildRecordLength(); got != 25 {
		t.Errorf("BuildRecordLength() = %d, want 25", got)
	}
	if p.AcceptsRecordLength(27) {
		t.Error("data-area length 27 was accepted as a record length")
	}
	if got := p.NameLength(); got != 10 {
		t.Errorf("NameLength() = %d, want 10", got)
	}
}

func TestProfileAdmitsP1P2AsOneExtraFlatRange(t *testing.T) {
	p := ic7760.Profile()
	for _, channel := range []int{1, 99, 100, 101} {
		if _, err := p.BuildMemoryRead(civ.ChannelAddress{Channel: channel}); err != nil {
			t.Errorf("BuildMemoryRead(channel %d): %v", channel, err)
		}
	}
	for _, channel := range []int{0, 102} {
		if _, err := p.BuildMemoryRead(civ.ChannelAddress{Channel: channel}); err == nil {
			t.Errorf("BuildMemoryRead(channel %d) admitted an address outside MEM plus P1/P2", channel)
		}
	}
}

func TestZeroValueProfileRefusesEverything(t *testing.T) {
	civtestRunZeroValue(t)
}

func civtestRunZeroValue(t *testing.T) {
	var p civ.Profile
	if p.Configured() {
		t.Fatal("zero Profile is configured")
	}
	if _, err := p.BuildMemoryRead(civ.ChannelAddress{Channel: 1}); err == nil {
		t.Error("zero Profile built a memory read")
	}
	if _, err := p.BuildMemorySet(civ.MemoryRecord{}); err == nil {
		t.Error("zero Profile built a memory set")
	}
}
