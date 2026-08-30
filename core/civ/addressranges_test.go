// SPDX-License-Identifier: GPL-3.0-or-later

package civ

import (
	"strings"
	"testing"
)

func bankRangeConfig() ProfileConfig {
	return ProfileConfig{
		Model:         "TEST-BANK-RANGES",
		RadioAddress:  0x88,
		AddressForm:   AddressFormBankChannel,
		Groups:        2,
		GroupBase:     1,
		ChannelLo:     10,
		ChannelHi:     11,
		ExtraRanges:   []AddressRange{{GroupLo: 4, GroupHi: 4, ChannelLo: 20, ChannelHi: 21}},
		Discriminator: DiscriminatorSingleLength,
		BuildLength:   6,
		Layouts: []RecordLayout{{
			Length: 6,
			Fields: []FieldSpan{
				{Field: FieldRXFrequency, Offset: 0, Length: 5, Encoding: EncodingBCDNumber, Order: OrderLittleEndian, Scale: 1},
				{Field: FieldMode, Offset: 5, Length: 1, Encoding: EncodingEnum, Enum: map[byte]string{0: "FM"}},
			},
		}},
	}
}

func TestAddressFormBankChannelIsDistinctAndThreeBytesWide(t *testing.T) {
	if AddressFormBankChannel == AddressFormBandChannel {
		t.Fatal("AddressFormBankChannel aliases AddressFormBandChannel — bank and band indices are different semantics")
	}
	if got := AddressFormBankChannel.String(); got != "AddressFormBankChannel" {
		t.Errorf("String() = %q, want AddressFormBankChannel", got)
	}
	if !AddressFormBankChannel.grouped() || AddressFormBankChannel.groupBytes() != 1 || AddressFormBankChannel.addressBytes() != 3 {
		t.Errorf("bank form = grouped %v, group bytes %d, address bytes %d; want true, 1, 3", AddressFormBankChannel.grouped(), AddressFormBankChannel.groupBytes(), AddressFormBankChannel.addressBytes())
	}

	p := MustNewProfile(bankRangeConfig())
	cmd, err := p.BuildMemoryRead(ChannelAddress{Group: 4, Channel: 21})
	if err != nil {
		t.Fatalf("BuildMemoryRead(extra-range address): %v", err)
	}
	want := []byte{PreambleByte, PreambleByte, 0x88, ControllerAddressDefault, CmdMemory, SubMemoryContents, 0x04, 0x00, 0x21, EndByte}
	if got := cmd.Bytes(); string(got) != string(want) {
		t.Errorf("bank/channel read = % X, want % X", got, want)
	}
}

func TestExtraRangesAreAUnionNotARectangularClosure(t *testing.T) {
	p := MustNewProfile(bankRangeConfig())
	for _, addr := range []ChannelAddress{{Group: 1, Channel: 10}, {Group: 2, Channel: 11}, {Group: 4, Channel: 20}, {Group: 4, Channel: 21}} {
		if _, err := p.BuildMemoryRead(addr); err != nil {
			t.Errorf("BuildMemoryRead(%v): %v", addr, err)
		}
	}

	hole := ChannelAddress{Group: 3, Channel: 15}
	if _, err := p.BuildMemoryRead(hole); err == nil {
		t.Errorf("BuildMemoryRead(%v) admitted the rectangular-closure hole", hole)
	}
	answer := []byte{PreambleByte, PreambleByte, ControllerAddressDefault, 0x88, CmdMemory, SubMemoryContents, 0x03, 0x00, 0x15, 0, 0, 0, 0, 0, 0, EndByte}
	if _, _, err := p.MemoryAnswerRecord(answer); err == nil {
		t.Errorf("MemoryAnswerRecord admitted the rectangular-closure hole %v", hole)
	}
	read := []byte{PreambleByte, PreambleByte, 0x88, ControllerAddressDefault, CmdMemory, SubMemoryContents, 0x03, 0x00, 0x15, EndByte}
	if p.AllowedCommand(read) {
		t.Errorf("AllowedCommand admitted the rectangular-closure hole %v", hole)
	}
}

func TestExtraRangeValidationNamesEachBrokenRule(t *testing.T) {
	tests := []struct {
		name string
		edit func(*ProfileConfig)
		want string
	}{
		{"negative group low", func(c *ProfileConfig) { c.ExtraRanges[0].GroupLo = -1 }, "GroupLo is -1"},
		{"group order", func(c *ProfileConfig) { c.ExtraRanges[0].GroupLo, c.ExtraRanges[0].GroupHi = 5, 4 }, "GroupLo..GroupHi is 5..4"},
		{"group does not fit form", func(c *ProfileConfig) { c.ExtraRanges[0].GroupHi = 100 }, "GroupHi is 100"},
		{"negative channel low", func(c *ProfileConfig) { c.ExtraRanges[0].ChannelLo = -1 }, "ChannelLo is -1"},
		{"channel order", func(c *ProfileConfig) { c.ExtraRanges[0].ChannelLo, c.ExtraRanges[0].ChannelHi = 22, 21 }, "ChannelLo..ChannelHi is 22..21"},
		{"channel does not fit form", func(c *ProfileConfig) { c.ExtraRanges[0].ChannelHi = 10000 }, "ChannelHi is 10000"},
		{"flat group low", func(c *ProfileConfig) { c.AddressForm, c.Groups, c.GroupBase = AddressFormFlat, 0, 0 }, "GroupLo is 4 under AddressFormFlat"},
		{"flat group high", func(c *ProfileConfig) {
			c.AddressForm, c.Groups, c.GroupBase, c.ExtraRanges[0].GroupLo = AddressFormFlat, 0, 0, 0
		}, "GroupHi is 4 under AddressFormFlat"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := bankRangeConfig()
			tc.edit(&cfg)
			_, err := NewProfile(cfg)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("NewProfile error = %v, want named failure containing %q", err, tc.want)
			}
		})
	}
}
