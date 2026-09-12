// SPDX-License-Identifier: GPL-3.0-or-later

package ic7700

import "github.com/gm5dna/open-rig-programmer/core/civ"

// NameCharset is every byte a memory name may carry.
//
// Matrix §1 row 30 / §3.9: PDF p.211 (folio 14-11), the per-command
// character-availability table's `1A 00 Memory name` row reads "All
// characters are available" — the LEAST restrictive of that table's three
// rows (its siblings each enumerate a narrower set and explicitly add
// "and space"). This document supplies no digit/symbol code TABLE of its
// own the way the IC-7610's document does, so the charset below is the
// same printable-ASCII reading the tier's other 7610-family models use,
// carried here as a DERIVATION rather than a per-glyph transcription.
// ASSUMED, register entry ic7700-tagcharset-space (the space half
// specifically: PDF p.211 does not itself print "and space" the way its
// siblings on the same table do).
//
// 95 bytes: 26 + 26 + 10 + 32 symbols + 1 space.
const NameCharset = "" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
	"abcdefghijklmnopqrstuvwxyz" +
	"0123456789" +
	"!$&?'^-/,;<([{|~" + "#%\\\"`+*.:=>)]}_@" +
	" "

// RecordOnlyLength is the RECORD-ONLY length: the 1A 00 data block
// excluding the two channel-selector bytes (q,w) and all frame overhead —
// what civ.Profile carries and what BuildMemorySet's <record> denotes.
// Matrix §3.11's eight-term addition, resolved exact from the 400 dpi
// render of PDF p.213 (the matrix's own headline finding):
// 1+5+1+1+1+3+3+5+9+10 = 39.
//
// DataAreaLength is the 1A 00 data block INCLUDING the two selector bytes
// — the matrix's own printed-index accounting. AddressBytes is q,w. A
// 1A 00 SET frame is therefore 6 + AddressBytes + RecordOnlyLength + 1 =
// 48 bytes.
const (
	RecordOnlyLength = 39
	AddressBytes     = 2
	DataAreaLength   = RecordOnlyLength + AddressBytes
	// NameLength is idx29-38, ten bytes (matrix §1 row 7, PDF p.213
	// "!8-@7 Memory name setting", "Up to 10 characters.").
	NameLength = 10
)

// The record regions ruling E6 leaves UNMAPPED on this model, exported so
// the driver's refusal check names them rather than re-deriving them.
//
//	SplitSelectByteOffset  idx0, the WHOLE byte: split flag (hi nibble) /
//	                       select-scan group (lo nibble). Neither has a
//	                       faithful neutral home (matrix §3.15(1)).
//	DataModeNibbleOffset   idx8's HIGH nibble: the four-valued data mode.
//	                       Its LOW nibble (tone_mode) IS mapped.
//	TXDupUnmappedOffset/   idx20-28 (9 bytes): the TX-duplicate block's own
//	TXDupUnmappedLength    mode/filter/tone-type/tone_tx/tone_rx bytes —
//	                       "second TX-side copy, no neutral field"
//	                       (coordinator ruling 12/09/2026).
const (
	SplitSelectByteOffset = 0
	DataModeNibbleOffset  = 8
	TXDupUnmappedOffset   = 20
	TXDupUnmappedLength   = 9
)

// modeEnum is idx6 (RX) and idx15... no — idx6 only; the TX-duplicate
// block's own mode byte (idx20) is UNMAPPED, see doc.go's ruling on the
// TX-dup block's non-frequency bytes.
//
// Matrix §1 row 6: PDF p.210 (folio 14-10), "Operating mode", Command
// 01/04/06: 00 LSB, 01 USB, 02 AM, 03 CW, 04 RTTY, 05 FM, 07 CW-R,
// 08 RTTY-R, 12 PSK, 13 PSK-R. Codes 06 and 09-11 are printed nowhere and
// are deliberately absent (register entry ic7700-mode-code-completeness):
// a record carrying one fails to decode with a parse error naming the
// offset, rather than inventing an eleventh value.
//
// A var, not a func: FieldSpan.clone() is what hands a copy to any caller
// outside this package, so nothing here needs to be re-built per call.
var modeEnum = map[byte]string{
	0x00: "LSB", 0x01: "USB", 0x02: "AM", 0x03: "CW", 0x04: "RTTY",
	0x05: "FM", 0x07: "CW-R", 0x08: "RTTY-R",
	0x12: "PSK", 0x13: "PSK-R",
}

// filterEnum is idx7. Matrix §1 row 24: PDF p.210, filter column: 01 FIL1,
// 02 FIL2, 03 FIL3. 0x00 is not a member — a record whose filter byte is
// 0x00 fails to decode rather than reading as "no filter".
var filterEnum = map[byte]string{0x01: "FIL1", 0x02: "FIL2", 0x03: "FIL3"}

// toneModeEnum is idx8's LOW nibble. Matrix §1 row 21: PDF p.213 (folio
// 14-13), the `!1` sub-diagram's upper leader: "0: OFF, 1: TONE, 2: TSQL".
// idx8's HIGH nibble (the four-valued data mode) is deliberately UNMAPPED
// — see doc.go's ruling E6.
var toneModeEnum = map[byte]string{0x0: "OFF", 0x1: "TONE", 0x2: "TSQL"}

// profile is the IC-7700's civ.Profile, built once at package
// initialisation. MustNewProfile is right here because every value below
// is a compile-time literal: a malformed one is a programming mistake that
// should stop the programme loudly on first use, not an error threaded
// through model registration.
var profile = civ.MustNewProfile(civ.ProfileConfig{
	Model: "IC-7700",
	// Matrix §1 row 2 / §3.4: PDF p.180 (folio 12-18), "CI-V Address":
	// "The IC-7700's address is 74h."
	RadioAddress: 0x74,
	// ControllerAddress left zero, which selects civ.ControllerAddressDefault
	// (0xE0) — the CI-V convention every model in this tier shares.
	AddressForm: civ.AddressFormFlat,
	Groups:      0,
	// 1..99 are the memories (BCD "00 01".."00 99"); 100 and 101 ARE the
	// scan edges P1/P2 (BCD "01 00"/"01 01") — one contiguous space, three
	// printed forms. Matrix §1 row 5.
	ChannelLo:     1,
	ChannelHi:     101,
	NameLength:    NameLength,
	NameCharset:   NameCharset,
	NamePad:       0x20, // ASSUMED, register entry ic7700-name-pad-byte (matrix §3.9)
	Layouts:       []civ.RecordLayout{layout()},
	Discriminator: civ.DiscriminatorSingleLength,
	BuildLength:   RecordOnlyLength,
})

// layout is the 39-byte record. EVERY OFFSET COMES FROM THE MATRIX'S §3.15
// TABLE. Offsets are 0-based from the start of the RECORD; the two
// channel-selector bytes (q,w) are the ADDRESS and lie outside it.
//
//	idx    width  field                          mapped to
//	0      1      split flag (hi) / select-group  UNMAPPED (E6) — whole byte
//	       (lo)
//	1-5    5      RX frequency                    FieldRXFrequency
//	6      1      RX mode                          FieldMode
//	7      1      RX filter                        FieldFilter
//	8 hi   -      RX data mode                     UNMAPPED (E6)
//	8 lo   1      RX tone mode                     FieldToneMode
//	9-11   3      repeater tone freq               FieldToneTX
//	12-14  3      tone squelch freq                FieldToneRX
//	15-19  5      TX frequency (TX-dup block)       FieldTXFrequency
//	20-28  9      TX mode/filter/data/tone bytes    UNMAPPED — "second
//	                                                 TX-side copy, no
//	                                                 neutral field"
//	                                                 (coordinator ruling
//	                                                 12/09/2026)
//	29-38  10     name                             FieldName
//
// The widths sum to 1+5+1+1+1+3+3+5+9+10 = 39.
func layout() civ.RecordLayout {
	return civ.RecordLayout{
		Length: RecordOnlyLength,
		// D14/E6: a FULL-LENGTH, ALL-ZERO template. Every byte this
		// layout does not map (idx0 whole, idx8 high nibble, idx20-28)
		// must equal Fixed for a slot to be written at all (ruling E6);
		// every byte a FieldSpan DOES cover must also be zero in the
		// template, which an all-zero 39-byte array trivially satisfies.
		Fixed: make([]byte, RecordOnlyLength),
		Fields: []civ.FieldSpan{
			// idx1-5 — RX frequency, five packed-BCD bytes, least
			// significant pair first (matrix §3.15(1), same convention
			// as every 7610-family sibling).
			{Field: civ.FieldRXFrequency, Offset: 1, Length: 5, Encoding: civ.EncodingBCDNumber, Order: civ.OrderLittleEndian, Scale: 1},
			// idx6 — operating mode.
			{Field: civ.FieldMode, Offset: 6, Length: 1, Encoding: civ.EncodingEnum, Enum: modeEnum},
			// idx7 — filter.
			{Field: civ.FieldFilter, Offset: 7, Length: 1, Encoding: civ.EncodingEnum, Enum: filterEnum},
			// idx8 low nibble — tone mode. The high nibble (data mode) is
			// deliberately absent from this list: E6 leaves it UNMAPPED.
			{Field: civ.FieldToneMode, Offset: 8, Length: 1, Nibble: civ.NibbleLow, Encoding: civ.EncodingEnum, Enum: toneModeEnum},
			// idx9-11 — repeater tone, three packed-BCD bytes, most
			// significant pair first, in tenths of a hertz.
			{Field: civ.FieldToneTX, Offset: 9, Length: 3, Encoding: civ.EncodingBCDNumber, Order: civ.OrderBigEndian, Scale: 1},
			// idx12-14 — tone squelch, same encoding.
			{Field: civ.FieldToneRX, Offset: 12, Length: 3, Encoding: civ.EncodingBCDNumber, Order: civ.OrderBigEndian, Scale: 1},
			// idx15-19 — the TX-duplicate block's OWN frequency span, a
			// DISTINCT field so a split channel round-trips (matrix §1b,
			// coordinator ruling 12/09/2026). idx20-28 (the block's
			// mode/filter/data/tone bytes) carry NO FieldSpan at all —
			// deliberately zero, "second TX-side copy, no neutral field"
			// — and are therefore governed by the same all-zero Fixed
			// template as idx0 and idx8's high nibble.
			{Field: civ.FieldTXFrequency, Offset: 15, Length: 5, Encoding: civ.EncodingBCDNumber, Order: civ.OrderLittleEndian, Scale: 1},
			// idx29-38 — the memory name.
			{Field: civ.FieldName, Offset: 29, Length: NameLength, Encoding: civ.EncodingName},
		},
	}
}

// Profile returns the IC-7700's civ.Profile.
//
// A function over an exported var, so the package-held value cannot be
// reassigned by a consumer: a Profile is what the outbound gate consults
// on every frame, and one a caller could reassign after init is not a
// gate.
func Profile() civ.Profile { return profile }
