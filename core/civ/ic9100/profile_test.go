// SPDX-License-Identifier: GPL-3.0-or-later

package ic9100

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
)

func TestZeroProfileIsInert(t *testing.T) {
	var zero civ.Profile
	if zero.Configured() {
		t.Fatal("zero civ.Profile is configured")
	}
	if cmd, err := zero.BuildTransceiverIDRead(); err == nil {
		t.Fatalf("zero civ.Profile built transceiver-ID read % X", cmd.Bytes())
	}
}

func TestProfilePolicy(t *testing.T) {
	p := Profile()
	if !p.Configured() {
		t.Fatal("Profile() is not configured")
	}
	if got, want := p.Model(), "IC-9100"; got != want {
		t.Errorf("Model() = %q, want %q", got, want)
	}
	// HEADLINE FINDING: matrix §3.4. 7Ch, not the 88h/E0h pair the dispatch
	// title and spec.md §1 assumed.
	if got, want := p.RadioAddress(), byte(0x7C); got != want {
		t.Errorf("RadioAddress() = %#02x, want %#02x (matrix §3.4)", got, want)
	}
	if got, want := p.ControllerAddress(), byte(0xE0); got != want {
		t.Errorf("ControllerAddress() = %#02x, want %#02x", got, want)
	}
	if got, want := p.AddressForm(), civ.AddressFormBankChannel; got != want {
		t.Errorf("AddressForm() = %v, want %v", got, want)
	}
	if got, want := p.RecordLengths(), []int{RecordLength}; !reflect.DeepEqual(got, want) {
		t.Errorf("RecordLengths() = %v, want %v", got, want)
	}
	if got, want := p.BuildRecordLength(), RecordLength; got != want {
		t.Errorf("BuildRecordLength() = %d, want %d", got, want)
	}
	if got, want := p.Discriminator(), civ.DiscriminatorSingleLength; got != want {
		t.Errorf("Discriminator() = %v, want %v", got, want)
	}
	if got, want := p.MaxFrame(), civ.DefaultMaxFrame; got != want {
		t.Errorf("MaxFrame() = %d, want default %d", got, want)
	}
	if got, want := p.Groups(), 3; got != want {
		t.Errorf("Groups() = %d, want %d (matrix §1 row 5: base radio, bands 00-02)", got, want)
	}
	if got, want := p.GroupBase(), 0; got != want {
		t.Errorf("GroupBase() = %d, want %d", got, want)
	}
	if lo, hi := p.ChannelRange(); lo != 1 || hi != 99 {
		t.Errorf("ChannelRange() = %d..%d, want 1..99", lo, hi)
	}
	if got, want := p.NameLength(), 9; got != want {
		t.Errorf("NameLength() = %d, want %d (matrix §1 row 7)", got, want)
	}
	if got, want := p.NamePad(), byte(0x20); got != want {
		t.Errorf("NamePad() = %#02x, want %#02x", got, want)
	}

	wantCharset := make([]byte, 0, 0x7f-0x20)
	for b := byte(0x20); b <= 0x7e; b++ {
		wantCharset = append(wantCharset, b)
	}
	if got := p.NameCharset(); !bytes.Equal(got, wantCharset) {
		t.Errorf("NameCharset() = % X, want printable ASCII 20..7E (matrix §3.9)", got)
	}
}

func TestProfileIsDefensive(t *testing.T) {
	p := Profile()
	layouts := p.Layouts()
	layouts[0].Fields[0].Offset = 99
	layouts[0].Fixed[DigitalSquelchOffset] = 0xff
	if got := Profile().Layouts()[0].Fields[0].Offset; got != 0 {
		t.Errorf("Profile layout was mutated through copy: offset %d", got)
	}
	if got := Profile().Layouts()[0].Fixed[DigitalSquelchOffset]; got != 0 {
		t.Errorf("Profile template was mutated through copy: %#x", got)
	}
}
