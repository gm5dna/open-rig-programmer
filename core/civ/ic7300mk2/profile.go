// SPDX-License-Identifier: GPL-3.0-or-later

package ic7300mk2

import "github.com/gm5dna/open-rig-programmer/core/civ"

// The name charset, assembled from the groups PDF p.18 prints.
//
// Unlike its sibling's, THIS model's symbol codes are TRANSCRIBED rather
// than derived: the "- Character codes— Symbols" table prints an ASCII
// code against every glyph, and the B leg carried all thirty-two of them
// across (matrix Erratum 4 corrects §3.9's "34" to 32). The SPACE is the
// one ASSUMED member, exactly as on the IC-7300: p.18's usable-characters
// note names a space in terms while neither p.18 table prints a row for
// it.
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
// channel-address bytes and the terminating FD. The data area is 47 and
// the whole set frame is 54; see doc.go.
const recordLength = 45

// nameLength is ⑱ ~ ㉝, sixteen bytes — three printed statements
// (matrix §1 #5).
const nameLength = 16

// modeEnum is ⑨ / ❾, the whole byte. PDF p.16, "• Operating mode".
// `06` IS ABSENT FROM THE PRINTED TABLE and no value is invented for it —
// see doc.go, which also records where this package's treatment of that
// hole and the matrix's wording part company.
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

// filterEnum is ⑩ / ❿, the whole byte. PDF p.16, "②Filter".
func filterEnum() map[byte]string {
	return map[byte]string{
		0x01: "FIL1",
		0x02: "FIL2",
		0x03: "FIL3",
	}
}

// dataModeEnum is ⑪'s HIGH nibble. On this model the assignment is read
// off the inset's own arrow labels — DATA left, TONE right (PDF p.17) —
// rather than by following leaders, which is why the MK2's B leg records
// it directly.
func dataModeEnum() map[byte]string {
	return map[byte]string{
		0x0: "OFF",
		0x1: "ON",
	}
}

// toneModeEnum is ⑪'s LOW nibble: TONE 0=OFF, 1=TONE, 2=TSQL.
func toneModeEnum() map[byte]string {
	return map[byte]string{
		0x0: "OFF",
		0x1: "TONE",
		0x2: "TSQL",
	}
}

// selectEnum is ③'s LOW nibble, and its LOW nibble only (D14). PDF p.17's
// ③ table prints SELECT 0=OFF, 1=★1, 2=★2, 3=★3 beside SPLIT 0=OFF,
// 1=ON, and the inset's two up-arrows label the LEFT nibble SPLIT and the
// RIGHT nibble SELECT. The SPLIT half is deliberately UNMAPPED and lives
// under the Fixed template (E6).
func selectEnum() map[byte]string {
	return map[byte]string{
		0x0: "OFF",
		0x1: "SEL1",
		0x2: "SEL2",
		0x3: "SEL3",
	}
}

// profile is the IC-7300MK2's civ.Profile. Every value is a compile-time
// literal, which is what makes MustNewProfile the right constructor here.
var profile = civ.MustNewProfile(civ.ProfileConfig{
	Model: "IC-7300MK2",
	// Matrix §3.4, PDF p.3: cell 3 of "Controller (PC) to IC-7300MK2"
	// prints B6 under index ②, "Transceiver's default address".
	RadioAddress: 0xB6,
	// PDF p.3, index ③, "Controller's (PC's) default address".
	ControllerAddress: 0xE0,
	// A CHOICE, argued in doc.go: the smallest round bound admitting BOTH
	// siblings' longest frames (54 here, 48 on the IC-7300), so a foreign
	// 48-byte answer fails as a *civ.RecordLengthError rather than being
	// pre-empted by ErrFrameTooLong.
	MaxFrame: 64,
	// Flat: two packed-BCD channel bytes and nothing before them.
	AddressForm: civ.AddressFormFlat,
	// Written-down zeros: no group or band index on this model.
	Groups:    0,
	GroupBase: 0,
	// PDF p.17's ①, ② legend: 00 01 ~ 00 99 are M-CH01 ~ M-CH99, 01 00 is
	// P1 and 01 01 is P2 — three address forms and no fourth.
	ChannelLo:   1,
	ChannelHi:   101,
	NameLength:  nameLength,
	NameCharset: nameCharset,
	// ASSUMED (D5 entry 3, pad half — lift MK2-W3): the field is a fixed
	// sixteen bytes and this document prints no padding rule, no
	// termination rule and no "trailing spaces are unnecessary" note for
	// 1A 00.
	NamePad: 0x20,
	Layouts: []civ.RecordLayout{{
		Length: recordLength,
		// D14/E6: FULL-LENGTH and ALL ZERO. The template declares this
		// record's one unmapped nibble — ③'s HIGH half, the split flag —
		// and Fixed[0]&0xF0 == 0x00 (Split OFF) is what the driver's E6
		// check compares a just-read record against.
		Fixed: make([]byte, recordLength),
		// §E's order exactly, and the offsets are ACCUMULATED FIELD
		// WIDTHS, never the printed index: PDF p.17's band prints 4–17
		// twice, outlined over drawn cells and filled over one undivided
		// region. That derivation carries D5 entry 5's assumption, lift
		// MK2-R1.
		//
		// IDENTICAL TO THE IC-7300's THROUGH OFFSET 28. That is not a
		// borrowing: each model's layout is read from its own document,
		// and the agreement is a finding rather than an input. Only the
		// name field's width differs — 16 here against 10 — and nothing
		// else moves.
		Fields: []civ.FieldSpan{
			// ③ low nibble — SELECT. The high nibble is SPLIT and is
			// UNMAPPED.
			{Field: civ.FieldSelect, Offset: 0, Length: 1, Nibble: civ.NibbleLow, Encoding: civ.EncodingEnum, Enum: selectEnum()},
			// ④ ~ ⑧ — RX frequency, five packed-BCD bytes, least
			// significant pair first (PDF p.16's per-nibble weights).
			{Field: civ.FieldRXFrequency, Offset: 1, Length: 5, Encoding: civ.EncodingBCDNumber, Order: civ.OrderLittleEndian, Scale: 1},
			// ⑨ — operating mode.
			{Field: civ.FieldMode, Offset: 6, Length: 1, Encoding: civ.EncodingEnum, Enum: modeEnum()},
			// ⑩ — filter.
			{Field: civ.FieldFilter, Offset: 7, Length: 1, Encoding: civ.EncodingEnum, Enum: filterEnum()},
			// ⑪ high / low — data mode and tone type.
			{Field: civ.FieldDataMode, Offset: 8, Length: 1, Nibble: civ.NibbleHigh, Encoding: civ.EncodingEnum, Enum: dataModeEnum()},
			{Field: civ.FieldToneMode, Offset: 8, Length: 1, Nibble: civ.NibbleLow, Encoding: civ.EncodingEnum, Enum: toneModeEnum()},
			// ⑫ ~ ⑭ — repeater tone. THE HEADING PRINTS NOTHING BENEATH
			// IT (§3.16 A6); this encoding is p.23's, ASSUMED, registered
			// as ic7300mk2-tone-tx-encoding with lift MK2-R17.
			{Field: civ.FieldToneTX, Offset: 9, Length: 3, Encoding: civ.EncodingBCDNumber, Order: civ.OrderBigEndian, Scale: 1},
			// ⑮ ~ ⑰ — tone squelch, p.23's form, printed.
			{Field: civ.FieldToneRX, Offset: 12, Length: 3, Encoding: civ.EncodingBCDNumber, Order: civ.OrderBigEndian, Scale: 1},
			// ❹ ~ ⓱ — the duplicated TX block: a distinct transmit
			// FREQUENCY so a split channel round-trips, and six mirrored
			// field ids the decoder requires to AGREE with their RX
			// copies.
			{Field: civ.FieldTXFrequency, Offset: 15, Length: 5, Encoding: civ.EncodingBCDNumber, Order: civ.OrderLittleEndian, Scale: 1},
			{Field: civ.FieldMode, Offset: 20, Length: 1, Encoding: civ.EncodingEnum, Enum: modeEnum()},
			{Field: civ.FieldFilter, Offset: 21, Length: 1, Encoding: civ.EncodingEnum, Enum: filterEnum()},
			{Field: civ.FieldDataMode, Offset: 22, Length: 1, Nibble: civ.NibbleHigh, Encoding: civ.EncodingEnum, Enum: dataModeEnum()},
			{Field: civ.FieldToneMode, Offset: 22, Length: 1, Nibble: civ.NibbleLow, Encoding: civ.EncodingEnum, Enum: toneModeEnum()},
			{Field: civ.FieldToneTX, Offset: 23, Length: 3, Encoding: civ.EncodingBCDNumber, Order: civ.OrderBigEndian, Scale: 1},
			{Field: civ.FieldToneRX, Offset: 26, Length: 3, Encoding: civ.EncodingBCDNumber, Order: civ.OrderBigEndian, Scale: 1},
			// ⑱ ~ ㉝ — the memory name, sixteen bytes.
			{Field: civ.FieldName, Offset: 29, Length: nameLength, Encoding: civ.EncodingName},
		},
	}},
	// One accepted length: this document describes no conditional field
	// width.
	Discriminator: civ.DiscriminatorSingleLength,
	BuildLength:   recordLength,
})

// Profile returns the IC-7300MK2's civ.Profile.
//
// A function over an exported var, on core/cat/ftdx10's pattern: the
// Profile is what the outbound gate consults on every frame, and one a
// caller could reassign after init is not a gate.
func Profile() civ.Profile { return profile }
