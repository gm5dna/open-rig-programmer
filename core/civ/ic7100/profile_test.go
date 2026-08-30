// SPDX-License-Identifier: GPL-3.0-or-later

package ic7100

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
	if got, want := p.Model(), "IC-7100"; got != want {
		t.Errorf("Model() = %q, want %q", got, want)
	}
	if got, want := p.RadioAddress(), byte(0x88); got != want {
		t.Errorf("RadioAddress() = %#02x, want %#02x", got, want)
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
	if got, want := p.Groups(), 5; got != want {
		t.Errorf("Groups() = %d, want %d", got, want)
	}
	if got, want := p.GroupBase(), 1; got != want {
		t.Errorf("GroupBase() = %d, want %d", got, want)
	}
	if lo, hi := p.ChannelRange(); lo != 1 || hi != 99 {
		t.Errorf("ChannelRange() = %d..%d, want 1..99", lo, hi)
	}
	if got, want := p.NameLength(), 16; got != want {
		t.Errorf("NameLength() = %d, want %d", got, want)
	}
	// ASSUMED: ic7100-name-pad-byte; lift: set a 3-character name and
	// read all 16 bytes back.
	if got, want := p.NamePad(), byte(0x20); got != want {
		t.Errorf("NamePad() = %#02x, want %#02x", got, want)
	}

	wantCharset := make([]byte, 0, 0x7f-0x20)
	for b := byte(0x20); b <= 0x7e; b++ {
		wantCharset = append(wantCharset, b)
	}
	// ASSUMED subclaim: ic7100-tag-charset-on-wire; lift: write ';', '\\'
	// and '~' in a name and read it back. The manual's table itself pins
	// the explicit printable-ASCII policy, including space and semicolon.
	if got := p.NameCharset(); !bytes.Equal(got, wantCharset) {
		t.Errorf("NameCharset() = % X, want printable ASCII 20..7E", got)
	}
}
