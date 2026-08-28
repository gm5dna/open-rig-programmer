// SPDX-License-Identifier: GPL-3.0-or-later

package ic705_test

import (
	"bytes"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/civtest"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic705"
)

// TestConformance is the shared dialect conformance suite, run against
// this model's profile. It is the reason this package is data-only: every
// invariant it checks belongs to core/civ, and a model package that
// implemented any of them locally would be asserting its own opinion of a
// shared rule.
func TestConformance(t *testing.T) { civtest.Run(t, ic705.Profile()) }

// TestZeroValueProfile pins that an unconfigured civ.Profile refuses
// everything, which is what makes `var p civ.Profile` a safe zero value
// rather than a profile that quietly builds frames for radio address 0.
func TestZeroValueProfile(t *testing.T) { civtest.RunZeroValue(t) }

func TestProfilePins(t *testing.T) {
	p := ic705.Profile()
	if got := p.Model(); got != "IC-705" {
		t.Errorf("Model = %q, want %q", got, "IC-705")
	}
	if got := p.RadioAddress(); got != 0xA4 {
		t.Errorf("RadioAddress = %#02x, want 0xA4 (matrix §3.4)", got)
	}
	if got := p.ControllerAddress(); got != 0xE0 {
		t.Errorf("ControllerAddress = %#02x, want 0xE0", got)
	}
	// Record-only, per spec Erratum 1 and matrix erratum 5. 115 is the
	// DATA AREA and must never be the profile's number.
	if got := p.RecordLengths(); len(got) != 1 || got[0] != 111 {
		t.Errorf("RecordLengths = %v, want [111]", got)
	}
	if p.AcceptsRecordLength(115) {
		t.Error("AcceptsRecordLength(115) is true — 115 is the data area, not the record")
	}
	if got := p.BuildRecordLength(); got != 111 {
		t.Errorf("BuildRecordLength = %d, want 111", got)
	}
	if got := p.NameLength(); got != 16 {
		t.Errorf("NameLength = %d, want 16", got)
	}
	if got := p.NamePad(); got != 0x20 {
		t.Errorf("NamePad = %#02x, want 0x20 (ASSUMED, lift L-NAME-PAD)", got)
	}
	if got := p.Groups(); got != 101 {
		t.Errorf("Groups = %d, want 101 — 100 memory groups plus the CALL group 0100", got)
	}
	if got := p.GroupBase(); got != 0 {
		t.Errorf("GroupBase = %d, want 0 — this radio numbers its groups from 0000, so the profile omits the field and takes E4's default", got)
	}
	if lo, hi := p.ChannelRange(); lo != 0 || hi != 99 {
		t.Errorf("ChannelRange = (%d, %d), want (0, 99)", lo, hi)
	}
	if got := p.MaxFrame(); got != 128 {
		t.Errorf("MaxFrame = %d, want 128", got)
	}
	if got := p.AddressForm(); got != civ.AddressFormWideGroupChannel {
		t.Errorf("AddressForm = %v, want AddressFormWideGroupChannel — the CALL group is wire index 100, which one packed-BCD byte cannot carry", got)
	}
	if got := p.Discriminator(); got != civ.DiscriminatorSingleLength {
		t.Errorf("Discriminator = %v, want DiscriminatorSingleLength — this model declares one record length", got)
	}
}

// TestNameCharsetIsTheEnumeratedSetPlusTheSpace pins the charset as the
// printed one, and pins that it was not widened. Matrix §3.9 as corrected
// by erratum 2 enumerates A~Z, a~z, 0~9 and 32 symbols; those 32 symbols
// are exactly ASCII's punctuation, so the enumerated set plus the ASSUMED
// space is precisely 0x20..0x7E. That is an OBSERVATION about what the
// printed table adds up to, not a licence to declare a range.
func TestNameCharsetIsTheEnumeratedSetPlusTheSpace(t *testing.T) {
	got := ic705.Profile().NameCharset()
	if len(got) != 95 {
		t.Fatalf("NameCharset has %d bytes, want 95 (0x20..0x7E)", len(got))
	}
	for i, b := range got {
		if want := byte(0x20 + i); b != want {
			t.Fatalf("NameCharset[%d] = %#02x, want %#02x — the charset is the enumerated set in order", i, b, want)
		}
	}
}

func TestReadFrameMatchesTheGoldenReadVector(t *testing.T) {
	// The A1 read form, BUILT rather than transcribed: the profile must
	// produce the very frame the G leg derived by hand off PDF p.19.
	cmd, err := ic705.Profile().BuildMemoryRead(civ.ChannelAddress{Group: 0, Channel: 12})
	if err != nil {
		t.Fatalf("BuildMemoryRead: %v", err)
	}
	want := []byte{0xFE, 0xFE, 0xA4, 0xE0, 0x1A, 0x00, 0x00, 0x00, 0x00, 0x12, 0xFD}
	if got := cmd.Bytes(); !bytes.Equal(got, want) {
		t.Errorf("read frame = % X, want % X (vectors.golden read-record)", got, want)
	}
}

func TestCallGroupIsAddressable(t *testing.T) {
	// Group 100 -> 01 00; channel 3 -> 00 03 (matrix §1b: 0002, 0003 =
	// 430 C1, C2). Without this the CALL bank could not be read at all.
	cmd, err := ic705.Profile().BuildMemoryRead(civ.ChannelAddress{Group: 100, Channel: 3})
	if err != nil {
		t.Fatalf("BuildMemoryRead(CALL): %v", err)
	}
	want := []byte{0xFE, 0xFE, 0xA4, 0xE0, 0x1A, 0x00, 0x01, 0x00, 0x00, 0x03, 0xFD}
	if got := cmd.Bytes(); !bytes.Equal(got, want) {
		t.Errorf("CALL read frame = % X, want % X", got, want)
	}
}

// TestSetFrameIsTheDocumentedLength pins the two numbers spec Erratum 1
// asks every per-radio plan to state together: the profile's RECORD-ONLY
// length (111) and the `1A 00` DATA AREA (115 = 4 address bytes + 111),
// measured through a built frame rather than restated.
func TestSetFrameIsTheDocumentedLength(t *testing.T) {
	p := ic705.Profile()
	rec := civ.MemoryRecord{
		Address:      civ.ChannelAddress{Group: 0, Channel: 12},
		RXFreqHz:     civ.Available(uint64(145500000)),
		TXFreqHz:     civ.Available(uint64(145500000)),
		OffsetHz:     civ.Available(uint64(600000)),
		ToneTXDeciHz: civ.Available(uint64(885)),
		ToneRXDeciHz: civ.Available(uint64(885)),
		DTCSCode:     civ.Available(uint64(23)),
		Duplex:       civ.Available("DUP-"),
		Mode:         civ.Available("FM"),
		Filter:       civ.Available("FIL1"),
		DataMode:     civ.Available("OFF"),
		ToneMode:     civ.Available("TONE"),
		DTCSPolarity: civ.Available("NN"),
		Name:         civ.Available("MY REPEATER CH01"),
	}
	cmd, err := p.BuildMemorySet(rec)
	if err != nil {
		t.Fatalf("BuildMemorySet: %v", err)
	}
	// FE FE A4 E0 1A 00 | 4 address + 111 record | FD
	if got := len(cmd.Bytes()); got != 122 {
		t.Errorf("set frame is %d bytes, want 122 = 7 framing/command + 4 address + 111 record", got)
	}
	if !p.AllowedCommand(cmd.Bytes()) {
		t.Error("the gate refuses this profile's own builder's output")
	}
}
