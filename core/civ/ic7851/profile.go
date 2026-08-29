// SPDX-License-Identifier: GPL-3.0-or-later

package ic7851

import "github.com/gm5dna/open-rig-programmer/core/civ"

// profile is the literal validated dialect. TestProfileShape pins the
// printed default address (register ic7851-moved-address is out of scope),
// while TestCrosscheckProfileAgainstLegs pins the independently measured
// layout.
var profile = civ.MustNewProfile(civ.ProfileConfig{
	Model: "IC-7851", RadioAddress: 0x8e, ControllerAddress: civ.ControllerAddressDefault,
	AddressForm: civ.AddressFormFlat, ChannelLo: 1, ChannelHi: 101,
	NameLength: 10, NameCharset: NameCharset, // ASSUMED name pad: ic7851-name-pad-byte.
	NamePad: 0x20,
	Layouts: []civ.RecordLayout{{
		Length: RecordOnlyLength,
		Fields: []civ.FieldSpan{
			// W/B pin little-endian packed BCD; it is not borrowed from a sibling.
			{Field: civ.FieldRXFrequency, Offset: 1, Length: 5, Encoding: civ.EncodingBCDNumber, Order: civ.OrderLittleEndian, Scale: 1},
			{Field: civ.FieldMode, Offset: 6, Length: 1, Encoding: civ.EncodingEnum, Enum: map[byte]string{0: "LSB", 1: "USB", 2: "AM", 3: "CW", 4: "RTTY", 5: "FM", 7: "CW-R", 8: "RTTY-R", 0x12: "PSK", 0x13: "PSK-R"}},
			{Field: civ.FieldFilter, Offset: 7, Length: 1, Encoding: civ.EncodingEnum, Enum: map[byte]string{1: "FIL1", 2: "FIL2", 3: "FIL3"}},
			{Field: civ.FieldToneMode, Offset: 8, Length: 1, Nibble: civ.NibbleLow, Encoding: civ.EncodingEnum, Enum: map[byte]string{0: "OFF", 1: "TONE", 2: "TSQL"}},
			{Field: civ.FieldToneTX, Offset: 9, Length: 3, Encoding: civ.EncodingBCDNumber, Order: civ.OrderBigEndian, Scale: 1},
			{Field: civ.FieldToneRX, Offset: 12, Length: 3, Encoding: civ.EncodingBCDNumber, Order: civ.OrderBigEndian, Scale: 1},
			{Field: civ.FieldName, Offset: 15, Length: 10, Encoding: civ.EncodingName},
		},
		Fixed: make([]byte, RecordOnlyLength),
	}},
	Discriminator: civ.DiscriminatorSingleLength, BuildLength: RecordOnlyLength,
})

func Profile() civ.Profile { return profile }
