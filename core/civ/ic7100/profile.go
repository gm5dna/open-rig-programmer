// SPDX-License-Identifier: GPL-3.0-or-later

package ic7100

import "github.com/gm5dna/open-rig-programmer/core/civ"

const (
	// RecordLength is record-only: it excludes the bank/channel address.
	// ASSUMED: ic7100-record-length; TestProfilePolicy pins this convention.
	RecordLength = 111
	// AddressBytes is bank 01–05 followed by the two-byte channel number.
	AddressBytes = 3
	// DataAreaLength is the complete 1A 00 data area on the wire.
	DataAreaLength = RecordLength + AddressBytes

	// duplicateBlockShift is the inclusive width of printed indices ⑤–51.
	// ASSUMED: ic7100-tx-block-mandatory; TestGeometryTXDuplicate pins the
	// arithmetic and the complete block equality used for safe writes.
	duplicateBlockShift = 47
)

// nameCharset is the manual's explicit character-code table, not the
// generic family default (which would wrongly reject semicolon).
const nameCharset = " !\"#$%&'()*+,-./0123456789:;<=>?@" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`" +
	"abcdefghijklmnopqrstuvwxyz{|}~"

var (
	selectNames = map[byte]string{0x00: "OFF", 0x01: "ON"}
	modeNames   = map[byte]string{
		0x00: "LSB", 0x01: "USB", 0x02: "AM", 0x03: "CW", 0x04: "RTTY",
		0x05: "FM", 0x06: "WFM", 0x07: "CW-R", 0x08: "RTTY-R", 0x17: "DV",
	}
	filterNames       = map[byte]string{0x01: "FIL1", 0x02: "FIL2", 0x03: "FIL3"}
	dataModeNames     = map[byte]string{0x00: "OFF", 0x01: "ON"}
	duplexNames       = map[byte]string{0x00: "OFF", 0x01: "DUP-", 0x02: "DUP+"}
	toneModeNames     = map[byte]string{0x00: "OFF", 0x01: "TONE", 0x02: "TSQL", 0x03: "DTCS"}
	dtcsPolarityNames = map[byte]string{0x00: "NN", 0x01: "NR", 0x10: "RN", 0x11: "RR"}
)

func enumSpan(id civ.FieldID, offset int, nibble civ.NibbleSel, values map[byte]string) civ.FieldSpan {
	return civ.FieldSpan{Field: id, Offset: offset, Length: 1, Nibble: nibble, Encoding: civ.EncodingEnum, Enum: values}
}

func bcdSpan(id civ.FieldID, offset, length int, order civ.ByteOrder, scale uint64) civ.FieldSpan {
	return civ.FieldSpan{Field: id, Offset: offset, Length: length, Encoding: civ.EncodingBCDNumber, Order: order, Scale: scale}
}

// fixedTemplateBytes is the state of every byte for which the landed neutral
// record has no FieldID. It is taken from the frozen G vector and is therefore
// conservative: later write preservation can refuse any record whose DSQL,
// CSQL or D-STAR bytes differ instead of inventing a writable interpretation.
// TestGoldenSetRecord pins these exact regions against the golden vector.
var fixedTemplateBytes = [RecordLength]byte{
	10: 0x00, 20: 0x00,
	24: 'C', 25: 'Q', 26: 'C', 27: 'Q', 28: 'C', 29: 'Q', 30: ' ', 31: ' ',
	32: ' ', 33: ' ', 34: ' ', 35: ' ', 36: ' ', 37: ' ', 38: ' ', 39: ' ',
	40: ' ', 41: ' ', 42: ' ', 43: ' ', 44: ' ', 45: ' ', 46: ' ', 47: ' ',
	57: 0x00, 67: 0x00,
	71: 'C', 72: 'Q', 73: 'C', 74: 'Q', 75: 'C', 76: 'Q', 77: ' ', 78: ' ',
	79: ' ', 80: ' ', 81: ' ', 82: ' ', 83: ' ', 84: ' ', 85: ' ', 86: ' ',
	87: ' ', 88: ' ', 89: ' ', 90: ' ', 91: ' ', 92: ' ', 93: ' ', 94: ' ',
}

func fixedTemplate() []byte {
	out := fixedTemplateBytes
	return out[:]
}

// recordFields maps only fields represented by civ.MemoryRecord. The DSQL,
// CSQL and three D-STAR call-sign regions remain explicit fixed bytes; mapping
// them onto unrelated neutral fields would manufacture write support.
func recordFields() []civ.FieldSpan {
	const d = duplicateBlockShift
	return []civ.FieldSpan{
		enumSpan(civ.FieldSelect, 0, civ.NibbleLow, selectNames),
		// ASSUMED: ic7100-wire-order; TestRecordRoundTrip pins little-endian
		// frequency bytes and the name's measured record offset.
		bcdSpan(civ.FieldRXFrequency, 1, 5, civ.OrderLittleEndian, 1),
		// ASSUMED: ic7100-dv-mode-code; TestRecordDVModeCode pins 0x17.
		enumSpan(civ.FieldMode, 6, civ.NibbleWhole, modeNames),
		enumSpan(civ.FieldFilter, 7, civ.NibbleWhole, filterNames),
		enumSpan(civ.FieldDataMode, 8, civ.NibbleWhole, dataModeNames),
		enumSpan(civ.FieldDuplex, 9, civ.NibbleHigh, duplexNames),
		enumSpan(civ.FieldToneMode, 9, civ.NibbleLow, toneModeNames),
		bcdSpan(civ.FieldToneTX, 11, 3, civ.OrderBigEndian, 1),
		bcdSpan(civ.FieldToneRX, 14, 3, civ.OrderBigEndian, 1),
		enumSpan(civ.FieldDTCSPolarity, 17, civ.NibbleWhole, dtcsPolarityNames),
		bcdSpan(civ.FieldDTCSCode, 18, 2, civ.OrderBigEndian, 1),
		bcdSpan(civ.FieldOffset, 21, 3, civ.OrderLittleEndian, 100),

		// The filled ❺–51 block has no internal witness cells, so every
		// mapped span is derived only by the documented +47 repetition.
		// ASSUMED: ic7100-tx-block-mandatory; TestGeometryTXDuplicate and
		// TestRecordDuplicateMismatch pin write equality and read refusal.
		bcdSpan(civ.FieldTXFrequency, 1+d, 5, civ.OrderLittleEndian, 1),
		enumSpan(civ.FieldMode, 6+d, civ.NibbleWhole, modeNames),
		enumSpan(civ.FieldFilter, 7+d, civ.NibbleWhole, filterNames),
		enumSpan(civ.FieldDataMode, 8+d, civ.NibbleWhole, dataModeNames),
		enumSpan(civ.FieldDuplex, 9+d, civ.NibbleHigh, duplexNames),
		enumSpan(civ.FieldToneMode, 9+d, civ.NibbleLow, toneModeNames),
		bcdSpan(civ.FieldToneTX, 11+d, 3, civ.OrderBigEndian, 1),
		bcdSpan(civ.FieldToneRX, 14+d, 3, civ.OrderBigEndian, 1),
		enumSpan(civ.FieldDTCSPolarity, 17+d, civ.NibbleWhole, dtcsPolarityNames),
		bcdSpan(civ.FieldDTCSCode, 18+d, 2, civ.OrderBigEndian, 1),
		bcdSpan(civ.FieldOffset, 21+d, 3, civ.OrderLittleEndian, 100),

		{Field: civ.FieldName, Offset: 95, Length: 16, Encoding: civ.EncodingName},
	}
}

var profile = civ.MustNewProfile(civ.ProfileConfig{
	Model:        "IC-7100",
	RadioAddress: 0x88,
	// ControllerAddress and MaxFrame are deliberately zero: the landed API
	// selects the shared E0 and 256-byte defaults, both pinned by TestProfilePolicy.
	AddressForm: civ.AddressFormBankChannel,
	Groups:      5,
	GroupBase:   1,
	ChannelLo:   1,
	ChannelHi:   99,
	ExtraRanges: nil,
	NameLength:  16,
	// ASSUMED: ic7100-name-pad-byte and ic7100-tag-charset-on-wire.
	NameCharset: nameCharset,
	NamePad:     0x20,
	Layouts: []civ.RecordLayout{{
		Length: RecordLength,
		Fields: recordFields(),
		Fixed:  fixedTemplate(),
	}},
	Discriminator: civ.DiscriminatorSingleLength,
	BuildLength:   RecordLength,
})

// Profile returns the validated IC-7100 profile by value.
func Profile() civ.Profile { return profile }
