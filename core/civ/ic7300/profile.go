// SPDX-License-Identifier: GPL-3.0-or-later

package ic7300

import "github.com/gm5dna/open-rig-programmer/core/civ"

// The name charset, assembled from the groups the document prints rather
// than collapsed into one range, so each group carries its own provenance.
//
// The three RANGES are printed verbatim on PDF p.168 (folio 19-10),
// "• Codes for character entries": `A–Z 41–5A`, `a-z 61–7A`, `0–9 30–39`.
// The SPACE is ASSUMED (`ic7300-name-space`): neither printed table has a
// row for it. The 32 SYMBOL GLYPHS are enumerated on that page, but only
// three of their codes are printed (`! 21`, `~ 7E`, `@ 40`); taking the
// other twenty-nine at their ASCII code points is a DERIVATION, registered
// as `ic7300-name-charset-symbol-codes` (open question B in the plan).
//
// 95 bytes in all: 1 + 10 + 26 + 26 + 32.
const (
	nameSpace   = " "
	nameDigits  = "0123456789"
	nameUpper   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	nameLower   = "abcdefghijklmnopqrstuvwxyz"
	nameSymbols = "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"

	nameCharset = nameSpace + nameDigits + nameUpper + nameLower + nameSymbols
)

// recordLength is the RECORD-ONLY length: the bytes between the two
// channel-address bytes and the terminating FD. The data area is 41 and
// the whole set frame is 48; see doc.go, which says why a fingerprint
// built on 41 is a different test.
const recordLength = 39

// nameLength is ⑱–㉗, ten bytes (matrix §3.9, PDF p.169 "Up to 10
// characters." plus the strip's own ten index positions).
const nameLength = 10

// modeEnum is ⑨ / ❾, the whole byte. PDF p.167 (folio 19-9),
// "① Operating mode". `06` IS ABSENT FROM THE PRINTED COLUMN and no value
// is invented for it here — see doc.go's D12 paragraph.
func modeEnum() map[byte]string {
	return map[byte]string{
		0x00: "LSB",
		0x01: "USB",
		0x02: "AM",
		0x03: "CW",
		0x04: "RTTY",
		0x05: "FM",
		0x07: "CW-R",
		0x08: "RTTY-R",
	}
}

// filterEnum is ⑩ / ❿, the whole byte. PDF p.167, "② Filter".
func filterEnum() map[byte]string {
	return map[byte]string{
		0x01: "FIL1",
		0x02: "FIL2",
		0x03: "FIL3",
	}
}

// dataModeEnum is ⑪'s HIGH nibble. The nibble assignment is a leader-order
// reading (W hazard (c)): the LEFT (high) nibble reaches
// "0=Data mode OFF / 1=Data mode ON".
func dataModeEnum() map[byte]string {
	return map[byte]string{
		0x0: "OFF",
		0x1: "ON",
	}
}

// toneModeEnum is ⑪'s LOW nibble: "0: OFF, 1: TONE, 2: TSQL".
func toneModeEnum() map[byte]string {
	return map[byte]string{
		0x0: "OFF",
		0x1: "TONE",
		0x2: "TSQL",
	}
}

// selectEnum is ③'s LOW nibble, and its LOW nibble only (D14). The four
// printed values are `0=OFF`, `1= ★1`, `2= ★2`, `3= ★3` (PDF p.169's ③
// detail box). ③'s HIGH nibble is the SPLIT flag and is deliberately
// UNMAPPED — it lives under the Fixed template, which is what lets a
// driver see a Split-ON record and refuse to write it back (E6).
//
// The names are SEL1/SEL2/SEL3 rather than the printed stars because a
// neutral record's text values are compared, logged and round-tripped as
// Go strings; the printed glyphs are recorded in doc.go.
func selectEnum() map[byte]string {
	return map[byte]string{
		0x0: "OFF",
		0x1: "SEL1",
		0x2: "SEL2",
		0x3: "SEL3",
	}
}

// profile is the IC-7300's civ.Profile, built once at package
// initialisation. MustNewProfile is right here because every value below
// is a compile-time literal: a malformed one is a programming mistake that
// should stop the programme loudly on first use, not an error threaded
// through model registration.
var profile = civ.MustNewProfile(civ.ProfileConfig{
	Model: "IC-7300",
	// Matrix §3.4, PDF p.126 (folio 12-10): CI-V Address (Default: 94h).
	RadioAddress: 0x94,
	// The CI-V convention, written down rather than defaulted.
	ControllerAddress: 0xE0,
	// A CHOICE, argued in doc.go: the smallest round bound that admits
	// BOTH siblings' longest frames (48 here, 54 on the MK2), so a foreign
	// 54-byte answer fails as a *civ.RecordLengthError — the length
	// fingerprint spec D3.2 asks for — rather than being pre-empted by
	// ErrFrameTooLong before a record ever exists.
	MaxFrame: 64,
	// Flat: the channel is two packed-BCD bytes and nothing precedes them.
	AddressForm: civ.AddressFormFlat,
	// Written-down zeros: this model has no group or band index, so the
	// count and the base are both explicitly nothing rather than omitted.
	Groups:    0,
	GroupBase: 0,
	// 1..99 are M-CH01..99; 100 is P1 (wire `01 00`) and 101 is P2 (wire
	// `01 01`), which the two-byte packed-BCD channel field encodes
	// directly.
	ChannelLo:   1,
	ChannelHi:   101,
	NameLength:  nameLength,
	NameCharset: nameCharset,
	// ASSUMED (D5 entry 3, pad half — lift `ic7300-name-pad`): the field
	// is always ten bytes and the document names no padding character.
	NamePad: 0x20,
	Layouts: []civ.RecordLayout{{
		Length: recordLength,
		// D14/E6: a FULL-LENGTH, ALL-ZERO template. V8 permits an empty
		// template or one of exactly Length bytes and refuses a template
		// nibble lying under a mapped span, so all-zero-at-39 is the only
		// shape that declares this record's one unmapped nibble — ③'s
		// HIGH half, the split flag — while leaving every mapped nibble to
		// its own span. Fixed[0]&0xF0 == 0x00 (Split OFF) IS this model's
		// unmapped-region contract.
		Fixed: make([]byte, recordLength),
		// §E's order exactly. OFFSETS ARE ACCUMULATED FIELD WIDTHS, NEVER
		// THE PRINTED INDEX: indices 4–17 are printed twice on this strip
		// (once outlined, once reversed out of filled discs), so a table
		// keyed on the printed index would be ambiguous. That derivation
		// carries D5 entry 5's assumption, lift `ic7300-wire-order`.
		Fields: []civ.FieldSpan{
			// ③ low nibble — SELECT. The high nibble is the split flag
			// and is UNMAPPED.
			{Field: civ.FieldSelect, Offset: 0, Length: 1, Nibble: civ.NibbleLow, Encoding: civ.EncodingEnum, Enum: selectEnum()},
			// ④–⑧ — RX frequency, five packed-BCD bytes, least
			// significant pair first (PDF p.167's five-cell diagram).
			{Field: civ.FieldRXFrequency, Offset: 1, Length: 5, Encoding: civ.EncodingBCDNumber, Order: civ.OrderLittleEndian, Scale: 1},
			// ⑨ — operating mode.
			{Field: civ.FieldMode, Offset: 6, Length: 1, Encoding: civ.EncodingEnum, Enum: modeEnum()},
			// ⑩ — filter.
			{Field: civ.FieldFilter, Offset: 7, Length: 1, Encoding: civ.EncodingEnum, Enum: filterEnum()},
			// ⑪ high / low — data mode and tone type.
			{Field: civ.FieldDataMode, Offset: 8, Length: 1, Nibble: civ.NibbleHigh, Encoding: civ.EncodingEnum, Enum: dataModeEnum()},
			{Field: civ.FieldToneMode, Offset: 8, Length: 1, Nibble: civ.NibbleLow, Encoding: civ.EncodingEnum, Enum: toneModeEnum()},
			// ⑫–⑭ — repeater tone, three packed-BCD bytes, most
			// significant pair first, in TENTHS of a hertz.
			{Field: civ.FieldToneTX, Offset: 9, Length: 3, Encoding: civ.EncodingBCDNumber, Order: civ.OrderBigEndian, Scale: 1},
			// ⑮–⑰ — tone squelch, same encoding.
			{Field: civ.FieldToneRX, Offset: 12, Length: 3, Encoding: civ.EncodingBCDNumber, Order: civ.OrderBigEndian, Scale: 1},
			// ❹–⓱ — the duplicated TX block. The transmit FREQUENCY is a
			// distinct field, so a split channel round-trips; the other
			// nine bytes reuse their RX copies' field ids, which makes the
			// encoder mirror them and the decoder require the two copies
			// to AGREE.
			{Field: civ.FieldTXFrequency, Offset: 15, Length: 5, Encoding: civ.EncodingBCDNumber, Order: civ.OrderLittleEndian, Scale: 1},
			{Field: civ.FieldMode, Offset: 20, Length: 1, Encoding: civ.EncodingEnum, Enum: modeEnum()},
			{Field: civ.FieldFilter, Offset: 21, Length: 1, Encoding: civ.EncodingEnum, Enum: filterEnum()},
			{Field: civ.FieldDataMode, Offset: 22, Length: 1, Nibble: civ.NibbleHigh, Encoding: civ.EncodingEnum, Enum: dataModeEnum()},
			{Field: civ.FieldToneMode, Offset: 22, Length: 1, Nibble: civ.NibbleLow, Encoding: civ.EncodingEnum, Enum: toneModeEnum()},
			{Field: civ.FieldToneTX, Offset: 23, Length: 3, Encoding: civ.EncodingBCDNumber, Order: civ.OrderBigEndian, Scale: 1},
			{Field: civ.FieldToneRX, Offset: 26, Length: 3, Encoding: civ.EncodingBCDNumber, Order: civ.OrderBigEndian, Scale: 1},
			// ⑱–㉗ — the memory name.
			{Field: civ.FieldName, Offset: 29, Length: nameLength, Encoding: civ.EncodingName},
		},
	}},
	// One accepted length, and the profile says so out loud: this model
	// documents no conditional field width.
	Discriminator: civ.DiscriminatorSingleLength,
	BuildLength:   recordLength,
})

// Profile returns the IC-7300's civ.Profile.
//
// A function over an exported var, on core/cat/ftdx10's pattern: the
// Profile is what the outbound gate consults on every frame, and one a
// caller could reassign after init is not a gate.
func Profile() civ.Profile { return profile }
