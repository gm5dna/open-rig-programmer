// SPDX-License-Identifier: GPL-3.0-or-later

// Package ic705_test proves, in this radio's own worktree, that the
// SHARED four-byte address form enabler E4 landed on `icom-core` is the
// form the IC-705's evidence describes.
//
// IT IMPLEMENTS NOTHING SHARED. R4 routed the address form to E4, so what
// is left here is a consumption proof: a sibling's form that merely
// compiles is not the same thing as a form that reproduces THIS radio's
// frames. A failure in this file is a STOP for enabler E4, never a fix in
// this worktree.
//
// The profile below is a THROWAWAY FIXTURE, not the IC-705's — the real
// profile is Task 3's and does not exist yet — so this file can run
// before it. What it shares with the radio is only what E4 has to get
// right: the four-byte address form, a group space of 101, and the wire
// group index 100 that the IC-705 prints and sends as `01 00`.
package ic705_test

import (
	"bytes"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
)

// fixtureProfile is the smallest profile that exercises E4's form: one
// enum field in a one-byte record, and this radio's group space.
//
// GroupBase is OMITTED DELIBERATELY. It defaults to 0, and 0 is the
// IC-705's base — the radio numbers its memory groups from `0000` — so
// stating it would be noise, and E4's joint limit (GroupBase + Groups − 1
// must fit the form's index field) is checked here at 0 + 101 − 1 = 100.
func fixtureProfile(t *testing.T) civ.Profile {
	t.Helper()
	return civ.MustNewProfile(civ.ProfileConfig{
		Model:         "G2C-FIXTURE",
		RadioAddress:  0xA4,
		MaxFrame:      32,
		AddressForm:   civ.AddressFormWideGroupChannel,
		Groups:        101,
		ChannelLo:     0,
		ChannelHi:     99,
		Discriminator: civ.DiscriminatorSingleLength,
		BuildLength:   1,
		Layouts: []civ.RecordLayout{{Length: 1, Fields: []civ.FieldSpan{
			{Field: civ.FieldMode, Offset: 0, Length: 1,
				Encoding: civ.EncodingEnum, Enum: map[byte]string{0x05: "FM"}},
		}}},
	})
}

func TestE4FormPutsFourBytesOnTheWire(t *testing.T) {
	p := fixtureProfile(t)
	cmd, err := p.BuildMemoryRead(civ.ChannelAddress{Group: 0, Channel: 12})
	if err != nil {
		t.Fatalf("BuildMemoryRead: %v", err)
	}
	// FE FE A4 E0 1A 00 | 00 00 (group) 00 12 (channel) | FD — eleven
	// bytes, which is the length `ic705-vectors.golden`'s read-record
	// vector measures.
	want := []byte{0xFE, 0xFE, 0xA4, 0xE0, 0x1A, 0x00, 0x00, 0x00, 0x00, 0x12, 0xFD}
	if got := cmd.Bytes(); !bytes.Equal(got, want) {
		t.Errorf("read frame = % X, want % X", got, want)
	}
	if got := len(cmd.Bytes()); got != 11 {
		t.Errorf("read frame is %d bytes, want 11 (7 + a four-byte address)", got)
	}
}

func TestE4FormReachesTheCallGroupAtWireIndex100(t *testing.T) {
	// The hundredth-and-first group is exactly why the form exists: the
	// IC-705's CALL group is 0100 (matrix §1b), wire bytes 01 00. Group:
	// 100 is the WIRE index (E4's decided semantics — core/civ/record.go
	// on ChannelAddress: "AS THE RADIO NUMBERS IT"), so there is no base
	// arithmetic here and none anywhere else in this package.
	p := fixtureProfile(t)
	cmd, err := p.BuildMemoryRead(civ.ChannelAddress{Group: 100, Channel: 3})
	if err != nil {
		t.Fatalf("BuildMemoryRead(CALL group 100, channel 3): %v", err)
	}
	want := []byte{0xFE, 0xFE, 0xA4, 0xE0, 0x1A, 0x00, 0x01, 0x00, 0x00, 0x03, 0xFD}
	if got := cmd.Bytes(); !bytes.Equal(got, want) {
		t.Errorf("CALL read frame = % X, want % X", got, want)
	}
}

func TestE4FormRoundTripsThroughTheParser(t *testing.T) {
	// The answer direction: the two address bytes swap, and the record
	// follows the four address bytes.
	p := fixtureProfile(t)
	answer := []byte{0xFE, 0xFE, 0xE0, 0xA4, 0x1A, 0x00, 0x01, 0x00, 0x00, 0x03, 0x05, 0xFD}
	addr, rec, err := p.MemoryAnswerRecord(answer)
	if err != nil {
		t.Fatalf("MemoryAnswerRecord: %v", err)
	}
	if addr.Group != 100 || addr.Channel != 3 {
		t.Errorf("address = %+v, want {Group:100 Channel:3} — the wire index, not a display number", addr)
	}
	if len(rec) != 1 || rec[0] != 0x05 {
		t.Errorf("record = % X, want 05 — the four address bytes must not be counted into it", rec)
	}
}

func TestE4FormRefusesANonDecimalGroup(t *testing.T) {
	// A wire group of 0x0A 0x00 is not four decimal digits. Packed BCD
	// has no nibble worth ten, so the parser must REFUSE it rather than
	// read 0x0A as the decimal 10 — which would silently answer for group
	// 1000 while claiming to answer for group 10.
	p := fixtureProfile(t)
	answer := []byte{0xFE, 0xFE, 0xE0, 0xA4, 0x1A, 0x00, 0x0A, 0x00, 0x00, 0x03, 0x05, 0xFD}
	if addr, _, err := p.MemoryAnswerRecord(answer); err == nil {
		t.Errorf("MemoryAnswerRecord admitted a non-decimal group and returned %+v", addr)
	}
}

func TestE4FormRefusesAGroupPastTheDeclaredSpace(t *testing.T) {
	// 101 is one past the last group this profile has (0..100), and the
	// form could encode it perfectly well — the refusal has to come from
	// the PROFILE's declared space, not from the BCD width, or a radio
	// with 101 groups would be addressed at 9999.
	p := fixtureProfile(t)
	if cmd, err := p.BuildMemoryRead(civ.ChannelAddress{Group: 101, Channel: 0}); err == nil {
		t.Errorf("BuildMemoryRead admitted group 101 and built % X", cmd.Bytes())
	}
}
