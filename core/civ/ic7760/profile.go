// SPDX-License-Identifier: GPL-3.0-or-later

package ic7760

import "github.com/gm5dna/open-rig-programmer/core/civ"

const (
	RecordOnlyLength = 25
	DataAreaLength   = 27
	AddressBytes     = 2
)

// NameCharset is the 94 printed character codes plus the assumed ASCII
// space (ic7760-name-space-code, Stage R lift). The space is also the
// assumed pad byte (ic7760-name-pad-byte, Stage R lift).
const NameCharset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!\"#$%&'()*+,./:;<=>?@[\\]^_`{|}~ "

var modeEnum = map[byte]string{
	0x00: "LSB", 0x01: "USB", 0x02: "AM", 0x03: "CW", 0x04: "RTTY",
	0x05: "FM", 0x07: "CW-R", 0x08: "RTTY-R", 0x12: "PSK", 0x13: "PSK-R",
}

var filterEnum = map[byte]string{0x01: "FIL1", 0x02: "FIL2", 0x03: "FIL3"}
var toneModeEnum = map[byte]string{0: "OFF", 1: "TONE", 2: "TSQL"}

func layout() civ.RecordLayout {
	return civ.RecordLayout{
		Length: RecordOnlyLength,
		Fields: []civ.FieldSpan{
			{Field: civ.FieldRXFrequency, Offset: 1, Length: 5, Encoding: civ.EncodingBCDNumber, Order: civ.OrderLittleEndian, Scale: 1},
			{Field: civ.FieldMode, Offset: 6, Length: 1, Encoding: civ.EncodingEnum, Enum: modeEnum},
			{Field: civ.FieldFilter, Offset: 7, Length: 1, Encoding: civ.EncodingEnum, Enum: filterEnum},
			{Field: civ.FieldToneMode, Offset: 8, Length: 1, Nibble: civ.NibbleLow, Encoding: civ.EncodingEnum, Enum: toneModeEnum},
			{Field: civ.FieldToneTX, Offset: 9, Length: 3, Encoding: civ.EncodingBCDNumber, Order: civ.OrderBigEndian, Scale: 1},
			{Field: civ.FieldToneRX, Offset: 12, Length: 3, Encoding: civ.EncodingBCDNumber, Order: civ.OrderBigEndian, Scale: 1},
			{Field: civ.FieldName, Offset: 15, Length: 10, Encoding: civ.EncodingName},
		},
		Fixed: make([]byte, RecordOnlyLength),
	}
}

var profile = civ.MustNewProfile(civ.ProfileConfig{
	Model: "IC-7760", RadioAddress: 0xB2, AddressForm: civ.AddressFormFlat,
	ChannelLo: 1, ChannelHi: 99,
	// P1/P2 are MANUAL-EVIDENCED selectors 100/101, but are not MEM
	// channels. Keeping them in one exact extra range prevents the base
	// inventory from silently expanding; TestProfileAdmitsP1P2AsOneExtraFlatRange
	// pins both admitted endpoints and the adjacent refusals.
	ExtraRanges: []civ.AddressRange{{GroupLo: 0, GroupHi: 0, ChannelLo: 100, ChannelHi: 101}},
	NameLength:  10, NameCharset: NameCharset,
	NamePad: 0x20, Layouts: []civ.RecordLayout{layout()},
	Discriminator: civ.DiscriminatorSingleLength, BuildLength: RecordOnlyLength,
})

// Profile returns the inert, validated IC-7760 CI-V profile. Registration is
// deliberately outside this package (Wave 4); USB (B) is the Stage 1 scope.
func Profile() civ.Profile { return profile }
