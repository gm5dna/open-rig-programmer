// SPDX-License-Identifier: GPL-3.0-or-later

package ic9100

import "github.com/gm5dna/open-rig-programmer/core/civ"

var (
	selectNames = map[byte]string{0x00: "OFF", 0x01: "ON"}
	// modeNames: matrix §1 row 6, PDF p.199/p.204. No WFM(06), unlike
	// IC-7610/IC-7100; DV(0x17) added, matching the D-STAR family.
	modeNames = map[byte]string{
		0x00: "LSB", 0x01: "USB", 0x02: "AM", 0x03: "CW", 0x04: "RTTY",
		0x05: "FM", 0x07: "CW-R", 0x08: "RTTY-R", 0x17: "DV",
	}
	filterNames       = map[byte]string{0x01: "FIL1", 0x02: "FIL2", 0x03: "FIL3"}
	dataModeNames     = map[byte]string{0x00: "OFF", 0x01: "ON"}
	duplexNames       = map[byte]string{0x00: "OFF", 0x01: "DUP-", 0x02: "DUP+"}
	toneModeNames     = map[byte]string{0x00: "OFF", 0x01: "TONE", 0x02: "TSQL", 0x03: "DTCS"}
	dtcsPolarityNames = map[byte]string{0x00: "NN", 0x01: "NR", 0x10: "RN", 0x11: "RR"}
)

// fixedTemplateBytes is the state of every byte for which the landed
// neutral record has no FieldID: the two D-STAR squelch bytes and the
// three 8-byte call signs (doc.go). Taken from the IC-7100 package's own
// convention (its identical fixedTemplateBytes region), which is a CHOICE
// and not a manual-stated default — the manual gives no factory value for
// any of the four. TestRecordFixedTemplate pins these exact regions.
var fixedTemplateBytes = [RecordLength]byte{
	DigitalSquelchOffset:     0x00,
	DigitalCodeSquelchOffset: 0x00,
	DestCallOffset:           'C', DestCallOffset + 1: 'Q', DestCallOffset + 2: 'C', DestCallOffset + 3: 'Q',
	DestCallOffset + 4: 'C', DestCallOffset + 5: 'Q', DestCallOffset + 6: ' ', DestCallOffset + 7: ' ',
	R1CallOffset: 'C', R1CallOffset + 1: 'Q', R1CallOffset + 2: 'C', R1CallOffset + 3: 'Q',
	R1CallOffset + 4: 'C', R1CallOffset + 5: 'Q', R1CallOffset + 6: ' ', R1CallOffset + 7: ' ',
	R2CallOffset: 'C', R2CallOffset + 1: 'Q', R2CallOffset + 2: 'C', R2CallOffset + 3: 'Q',
	R2CallOffset + 4: 'C', R2CallOffset + 5: 'Q', R2CallOffset + 6: ' ', R2CallOffset + 7: ' ',
}

// recordFields maps only fields spec D1's civ.MemoryRecord carries. The two
// D-STAR squelch bytes and the three call-sign regions remain explicit
// fixed bytes; mapping them onto unrelated neutral fields would manufacture
// write support the matrix's own §1b rules out.
//
// Offsets are record-only (0-based from the start of the 57-byte record,
// after the 3-byte band+channel address): matrix §3.11's term-by-term
// derivation, term 3 ("r") onward.
func recordFields() []civ.FieldSpan {
	return []civ.FieldSpan{
		civ.EnumSpan(civ.FieldSelect, 0, civ.NibbleLow, selectNames),
		// ASSUMED: ic9100-read-request-form / ic9100-wire-order share the
		// family convention (little-endian packed BCD), matching
		// IC-7100/IC-7610's own frequency spans.
		civ.BCDSpan(civ.FieldRXFrequency, 1, 5, civ.OrderLittleEndian, 1),
		civ.EnumSpan(civ.FieldMode, 6, civ.NibbleWhole, modeNames),
		civ.EnumSpan(civ.FieldFilter, 7, civ.NibbleWhole, filterNames),
		civ.EnumSpan(civ.FieldDataMode, 8, civ.NibbleWhole, dataModeNames),
		civ.EnumSpan(civ.FieldDuplex, 9, civ.NibbleHigh, duplexNames),
		civ.EnumSpan(civ.FieldToneMode, 9, civ.NibbleLow, toneModeNames),
		// offset 10: DigitalSquelchOffset — unmapped, doc.go.
		civ.BCDSpan(civ.FieldToneTX, 11, 3, civ.OrderBigEndian, 1),
		civ.BCDSpan(civ.FieldToneRX, 14, 3, civ.OrderBigEndian, 1),
		civ.EnumSpan(civ.FieldDTCSPolarity, 17, civ.NibbleWhole, dtcsPolarityNames),
		civ.BCDSpan(civ.FieldDTCSCode, 18, 2, civ.OrderBigEndian, 1),
		// offset 20: DigitalCodeSquelchOffset — unmapped, doc.go.
		// matrix §1b "offset": three bytes, byte1 = 1kHz|100Hz digit,
		// byte2 = 100kHz|10kHz digit, byte3 = 10MHz(fixed 0)|1MHz digit —
		// the same little-endian, scale-100 shape as IC-7100's own
		// duplex-offset span.
		civ.BCDSpan(civ.FieldOffset, 21, 3, civ.OrderLittleEndian, 100),
		// offsets 24-47: the three D-STAR call signs — unmapped, doc.go.
		{Field: civ.FieldName, Offset: 48, Length: 9, Encoding: civ.EncodingName},
	}
}

var profile = civ.MustNewProfile(civ.ProfileConfig{
	Model: "IC-9100",
	// HEADLINE FINDING (matrix §3.4, doc.go): 7Ch, not the 88h/E0h pair
	// spec.md §1 and the dispatch assumed.
	RadioAddress: 0x7C,
	// ControllerAddress and MaxFrame are deliberately zero: the landed API
	// selects the shared E0 and default frame bound.
	AddressForm: civ.AddressFormBankChannel,
	// matrix §1 row 5 / §1b: band values 00-02 (HF/50, 144, 430 MHz) on the
	// base 3-band radio, 297 slots. The optional UX-9100's 4th band (03,
	// 1200 MHz) is deferred — doc.go explains why.
	Groups:      3,
	GroupBase:   0,
	ChannelLo:   1,
	ChannelHi:   99,
	ExtraRanges: nil,
	NameLength:  9, // matrix §1 row 7 / §3.9: "9 characters (Fixed)".
	NameCharset: nameCharset,
	NamePad:     0x20, // ASSUMED: ic9100-name-pad-byte (matrix §3.9).
	Layouts: []civ.RecordLayout{{
		Length: RecordLength,
		Fields: recordFields(),
		Fixed:  fixedTemplateBytes[:],
	}},
	Discriminator: civ.DiscriminatorSingleLength,
	BuildLength:   RecordLength,
})

// Profile returns the validated IC-9100 profile by value.
func Profile() civ.Profile { return profile }
