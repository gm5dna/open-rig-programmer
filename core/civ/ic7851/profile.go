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
			// ④~⑦ ONLY, AND ⑧ IS DELIBERATELY OUTSIDE IT. W/B pin
			// little-endian packed BCD over the printed five-cell strip;
			// it is not borrowed from a sibling. The FIFTH cell, printed
			// ⑧, is drawn with a literal "0" in both halves and the
			// rotated leaders "1000 MHz digit: 0 (Fixed)" and "100 MHz
			// digit: 0 (Fixed)" (matrix §3.16.3, register entry
			// ic7851-fixed-nibble-reencode), so it is not a frequency
			// digit at all: it is a two-nibble pad.
			//
			// IT IS EXCLUDED FROM THE SPAN RATHER THAN CONSTRAINED
			// INSIDE IT because civ.FieldSpan carries no numeric domain.
			// A five-byte span would encode a digit there for any value
			// at or above 100 MHz, and AllowedCommand's re-encode leg
			// would reproduce that byte and admit the frame; the layout's
			// Fixed template only supplies bytes NO span maps, so
			// excluding ⑧ is what gives the template the byte back.
			// TestFixedBytesLieUnderNoMappedSpan, TestGeometryAndFixedRegions,
			// TestBuilderRefusesValuesNeedingAFixedByte and
			// TestGateRefusesNonZeroFixedBytes pin all four consequences.
			//
			// THE COST, STATED: the span now encodes eight digits, so the
			// builder refuses any frequency at or above 100 MHz outright.
			// That is inside this radio's own printed receiver coverage
			// (30 kHz–60 MHz, matrix §1 row 12/13), so no reachable
			// value is lost.
			{Field: civ.FieldRXFrequency, Offset: 1, Length: 4, Encoding: civ.EncodingBCDNumber, Order: civ.OrderLittleEndian, Scale: 1},
			{Field: civ.FieldMode, Offset: 6, Length: 1, Encoding: civ.EncodingEnum, Enum: map[byte]string{0: "LSB", 1: "USB", 2: "AM", 3: "CW", 4: "RTTY", 5: "FM", 7: "CW-R", 8: "RTTY-R", 0x12: "PSK", 0x13: "PSK-R"}},
			{Field: civ.FieldFilter, Offset: 7, Length: 1, Encoding: civ.EncodingEnum, Enum: map[byte]string{1: "FIL1", 2: "FIL2", 3: "FIL3"}},
			{Field: civ.FieldToneMode, Offset: 8, Length: 1, Nibble: civ.NibbleLow, Encoding: civ.EncodingEnum, Enum: map[byte]string{0: "OFF", 1: "TONE", 2: "TSQL"}},
			// ⑬⑭ and ⑯⑰ ONLY. The repeater-tone diagram both triples
			// point at draws its FIRST cell with two "Fixed digit: 0*"
			// leaders (matrix §3.16.4, register entry
			// ic7851-tone-fixed-byte), so printed ⑫ and ⑮ are pads and
			// are excluded from their spans for the frequency field's
			// reason. What remains is big-endian packed BCD in tenths of
			// a hertz — the OPPOSITE byte order to the frequency field on
			// the same radio, which is why each span states its own.
			//
			// The two remaining bytes carry 000.0–999.9 Hz, which covers
			// the whole printed 100 Hz: 0–2 / 10 Hz / 1 Hz / 0.1 Hz digit
			// domain and the whole 67.0–254.1 Hz selectable chart.
			{Field: civ.FieldToneTX, Offset: 10, Length: 2, Encoding: civ.EncodingBCDNumber, Order: civ.OrderBigEndian, Scale: 1},
			{Field: civ.FieldToneRX, Offset: 13, Length: 2, Encoding: civ.EncodingBCDNumber, Order: civ.OrderBigEndian, Scale: 1},
			{Field: civ.FieldName, Offset: 15, Length: 10, Encoding: civ.EncodingName},
		},
		Fixed: make([]byte, RecordOnlyLength),
	}},
	Discriminator: civ.DiscriminatorSingleLength, BuildLength: RecordOnlyLength,
})

func Profile() civ.Profile { return profile }
