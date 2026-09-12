// SPDX-License-Identifier: GPL-3.0-or-later

package ic7200

import "github.com/gm5dna/open-rig-programmer/core/civ"

// RecordOnlyLength is 17 bytes, NOT the 9 spec.md §1's IC-7200 clause and
// the S1 evidence file both state. Matrix §3.11 (the matrix's central
// finding): a 300 dpi render of PDF p.120 (folio 11-6) resolves a second,
// distinct 8-byte TX-duplicate block (printed in filled/reversed circled
// numerals, a glyph class pdftotext -layout cannot tell apart from the
// primary outline indices) that the manual's own NOTE mirrors the primary
// block into. AddressBytes is the flat two-byte channel selector, excluded
// from RecordOnlyLength by the same convention every sibling Icom package
// in this tier uses.
const (
	RecordOnlyLength = 17
	AddressBytes     = 2
)

// The four record bytes (record-only, 0-based) that carry no FieldSpan.
//
//   - SplitOffset is printed ③, a TX-block-enable flag with no "+/-"
//     sense — matrix §3.15(a): it maps to no spec.Field (duplex describes
//     shift direction, not "is the TX-duplicate block active", and the
//     block is unconditionally present regardless of Split's value).
//   - TXModeOffset, TXFilterOffset and TXDataModeOffset are printed
//     ❾❿⓫, the TX-duplicate block's mode/filter/data-mode mirror. The
//     tier vocabulary gives a transmit-only counterpart to frequency
//     (tx_frequency) but none to mode, filter or data_mode — matrix §1b,
//     "genuinely unmapped", the identical shape spec.md §1 records for the
//     IC-9100's D-STAR bytes and the IC-7100 precedent it cites.
//
// All four are held at FixedTemplate's zero; the driver refuses a write
// whose actual bytes differ from it (matrix §3.16 ADDED-1).
const (
	SplitOffset      = 0
	TXModeOffset     = 14
	TXFilterOffset   = 15
	TXDataModeOffset = 16
)

// modeEnum is byte ⑨. SOURCE: matrix §1 row 5, PDF p.120 (folio 11-6),
// "⑨ Operating mode": 00 LSB, 01 USB, 02 AM, 03 CW, 04 RTTY, 07 CW-R,
// 08 RTTY-R. No FM, WFM, DV or PSK code is printed anywhere for this
// radio — matrix §1 row 5: "HF+6m only", corroborated by the mode-select
// command rows (PDF p.117-118, folio 11-3/11-4).
var modeEnum = map[byte]string{
	0x00: "LSB", 0x01: "USB", 0x02: "AM", 0x03: "CW", 0x04: "RTTY",
	0x07: "CW-R", 0x08: "RTTY-R",
}

// filterEnum is byte ⑩. SOURCE: matrix §1b, PDF p.120, "⑩ Filter
// setting": 01 Wide, 02 Mid, 03 Narrow — THIS RADIO'S OWN NAMES, not the
// FIL1/FIL2/FIL3 label the IC-7610/IC-7100 family prints for the
// identical concept.
var filterEnum = map[byte]string{0x01: "Wide", 0x02: "Mid", 0x03: "Narrow"}

// dataModeEnum is byte ⑪. SOURCE: matrix §1b, PDF p.120, "⑪ Data mode
// setting": "1 byte data (XX)", 00 Data mode OFF, 10 Data mode ON — a
// FULL BYTE carrying 00/10, not the 00/01 nibble encoding every sibling
// Icom model in this tier uses for the same concept. Matrix §3.15(a) /
// §3.16 ADDED-3 flags this radio's OWN live 0F split command using 00/01
// for the unrelated Split concept; recorded, not resolved.
var dataModeEnum = map[byte]string{0x00: "OFF", 0x10: "ON"}

// FixedTemplate returns a fresh all-zero RecordOnlyLength template. This
// model's four unmapped bytes (SplitOffset, TXModeOffset, TXFilterOffset,
// TXDataModeOffset) are all written zero and read back compared against
// it; every other byte lies under a mapped span, where civ's own V8 rule
// requires the template to be zero anyway.
func FixedTemplate() []byte { return make([]byte, RecordOnlyLength) }

// layout is the 17-byte record. EVERY OFFSET COMES FROM MATRIX §3.11's
// ONE ARITHMETIC TABLE. Offsets are 0-based from the start of the RECORD:
// the two channel-selector bytes ①② are the ADDRESS field and lie outside
// it (the same convention civic7610.RecordOnlyLength documents).
//
//	printed   width  offset  field
//	③         1      0       UNMAPPED (SplitOffset)
//	④~⑧       5      1       rx_frequency (little-endian BCD)
//	⑨         1      6       mode
//	⑩         1      7       filter
//	⑪         1      8       data_mode
//	❹~❽       5      9       tx_frequency (little-endian BCD, same convention)
//	❾         1      14      UNMAPPED (TXModeOffset)
//	❿         1      15      UNMAPPED (TXFilterOffset)
//	⓫         1      16      UNMAPPED (TXDataModeOffset)
func layout() civ.RecordLayout {
	return civ.RecordLayout{
		Length: RecordOnlyLength,
		Fields: []civ.FieldSpan{
			{Field: civ.FieldRXFrequency, Offset: 1, Length: 5, Encoding: civ.EncodingBCDNumber, Order: civ.OrderLittleEndian, Scale: 1},
			{Field: civ.FieldMode, Offset: 6, Length: 1, Encoding: civ.EncodingEnum, Enum: modeEnum},
			{Field: civ.FieldFilter, Offset: 7, Length: 1, Encoding: civ.EncodingEnum, Enum: filterEnum},
			{Field: civ.FieldDataMode, Offset: 8, Length: 1, Encoding: civ.EncodingEnum, Enum: dataModeEnum},
			// ❹~❽, the TX-duplicate block's frequency span (matrix §3.11,
			// the central finding of the matrix): mapped to FieldTXFrequency
			// with the same little-endian BCD convention as the RX span,
			// per the manual's NOTE ("the same data as ④-⑪ are stored in
			// ❹-⓫"). SplitOffset and the three TX-mirror bytes at
			// TXModeOffset/TXFilterOffset/TXDataModeOffset carry no span —
			// matrix §3.16 ADDED-1.
			{Field: civ.FieldTXFrequency, Offset: 9, Length: 5, Encoding: civ.EncodingBCDNumber, Order: civ.OrderLittleEndian, Scale: 1},
		},
		Fixed: FixedTemplate(),
	}
}

// profile is built once at package init: a mistake in it is a build-time
// defect that must stop the programme loudly on first use, the same
// MustNewProfile convention every sibling Icom package in this tier uses.
var profile = civ.MustNewProfile(civ.ProfileConfig{
	Model:        "IC-7200",
	RadioAddress: 0x76, // matrix §1 row 2 / §3.4: PDF p.113 (folio 10-16).
	// ControllerAddress left zero, selecting civ.ControllerAddressDefault
	// (0xE0) — matrix §3.4: PDF p.116 (folio 11-2) prints "FE FE 76 E0".
	AddressForm: civ.AddressFormFlat,
	Groups:      0,
	// 1..199 are the memories; 200 and 201 ARE the scan edges P1/P2 — one
	// contiguous two-byte selector with three printed forms (matrix §1
	// row 4 / §2 SCAN bank / §3.15(d)).
	ChannelLo: 1,
	ChannelHi: 201,
	// NoTag (matrix §1 row 6, §3.9): the record has no name field at all,
	// so NameLength/NameCharset/NamePad all stay at civ's own "this model
	// has no name field" zero values (validateNamePolicy V4).
	Layouts:       []civ.RecordLayout{layout()},
	Discriminator: civ.DiscriminatorSingleLength,
	BuildLength:   RecordOnlyLength,
})

// Profile returns the IC-7200's CI-V profile.
//
// A function over an exported var so the package-held value cannot be
// reassigned by a consumer, mirroring every sibling Icom profile package's
// own Profile() accessor.
func Profile() civ.Profile { return profile }
