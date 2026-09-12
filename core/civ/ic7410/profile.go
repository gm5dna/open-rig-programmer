// SPDX-License-Identifier: GPL-3.0-or-later

package ic7410

import "github.com/gm5dna/open-rig-programmer/core/civ"

// modeEnum is record byte 6 (printed ⑨). SOURCE: matrix §1 row 6 / §1b —
// eight printed pairs, MANUAL-EVIDENCED against PDF p.116's Specifications
// list and cross-referenced from the record diagram at PDF p.115. Unlike
// the IC-7610 family, this model carries NO PSK/PSK-R pair: codes 0x06 and
// 0x09-0x11 are printed nowhere and 0x12/0x13 do not appear either, so a
// record carrying one of them fails to decode with a parse error naming
// the offset rather than a guessed name.
var modeEnum = map[byte]string{
	0x00: "LSB", 0x01: "USB", 0x02: "AM", 0x03: "CW", 0x04: "RTTY",
	0x05: "FM", 0x07: "CW-R", 0x08: "RTTY-R",
}

// filterEnum is record byte 7 (printed ⑩). Matrix §1 row 24: 01/02/03,
// cross-referenced from the "Data mode with filter width setting" diagram
// (Command 1A 06, PDF p.115). 0x00 is not a member.
var filterEnum = map[byte]string{0x01: "FIL1", 0x02: "FIL2", 0x03: "FIL3"}

// dataModeEnum is record byte 8 (printed ⑪), a WHOLE byte on this model —
// unlike the IC-7610 family's nibble-shared four-valued version, this is a
// genuine two-valued boolean occupying its own byte, with no nibble-sharing
// and no third/fourth DATA-1/2/3 value left unmapped (matrix §1b, offset 8
// row).
var dataModeEnum = map[byte]string{0x00: "OFF", 0x01: "ON"}

// toneModeEnum is record byte 9 (printed ⑫), the HIGH nibble — INVERTED
// relative to the IC-7610 family, where the equivalent vocabulary sits on
// the LOW nibble. Matrix §1b, offset 9 row: "hi: 0 OFF/1 TONE/2 TSQL; lo:
// fixed 0", read off the 400 dpi render with "no crossed-leader hazard:
// this page draws the leaders straight, one nibble live, one printed
// '0 (fixed)'".
var toneModeEnum = map[byte]string{0x0: "OFF", 0x1: "TONE", 0x2: "TSQL"}

// layout is the 40-byte record. EVERY OFFSET COMES FROM THE MATRIX'S §1b
// TABLE, corrected per spec.md's ruling 7 (the tier spec's own "25 B, offsets +1
// from byte 11" reading is superseded — see consts.go's RecordOnlyLength
// comment). Offsets are 0-based from the start of the RECORD: printed ③
// sits at record offset 0, so a printed index N (for N in ④..㉗) sits at
// offset N-3. The unindexed 15-byte TX-duplicate block has no printed
// index at all and is addressed by its own absolute offsets, 16-30.
//
//	printed        offset  width  field
//	③              0       1      UNMAPPED (SelectSplitOffset, whole byte)
//	④~⑧            1       5      rx_frequency
//	⑨              6       1      mode
//	⑩              7       1      filter
//	⑪              8       1      data_mode (WHOLE byte, unlike IC-7610)
//	⑫              9       1      tone_mode (HIGH nibble, unlike IC-7610)
//	⑬~⑮            10      3      tone_tx
//	⑯~⑱            13      3      tone_rx
//	❹~❽ (dup)      16      5      tx_frequency (spec.md's ruling 7)
//	❾~⑱ (dup)      21      10     UNMAPPED (TXDupUnmappedOffset)
//	⑲~㉗            31      9      name
func layout() civ.RecordLayout {
	return civ.RecordLayout{
		Length: RecordOnlyLength,
		Fields: []civ.FieldSpan{
			// Matrix §1b, offset 1: all five bytes are LIVE BCD digits — the
			// 10 MHz digit is printed 0-9, not the IC-7610 family's 0-6, and
			// there is no fixed-pad fifth cell on this model (matrix §1 row
			// 16). Least-significant pair first, matching every sibling's
			// frequency convention.
			{Field: civ.FieldRXFrequency, Offset: 1, Length: 5, Encoding: civ.EncodingBCDNumber, Order: civ.OrderLittleEndian, Scale: 1},
			{Field: civ.FieldMode, Offset: 6, Length: 1, Encoding: civ.EncodingEnum, Enum: modeEnum},
			{Field: civ.FieldFilter, Offset: 7, Length: 1, Encoding: civ.EncodingEnum, Enum: filterEnum},
			{Field: civ.FieldDataMode, Offset: 8, Length: 1, Encoding: civ.EncodingEnum, Enum: dataModeEnum},
			{Field: civ.FieldToneMode, Offset: 9, Length: 1, Nibble: civ.NibbleHigh, Encoding: civ.EncodingEnum, Enum: toneModeEnum},
			// Matrix §1b, offsets 10-15: three live bytes each, 0.1 Hz
			// resolution, big-endian — the same byte order as every sibling's
			// tone spans, and no fixed lead byte is printed for this model's
			// 40-byte record (unlike the IC-7851/IC-7610-family's tone
			// triples, whose FIRST cell is a printed fixed zero).
			{Field: civ.FieldToneTX, Offset: 10, Length: 3, Encoding: civ.EncodingBCDNumber, Order: civ.OrderBigEndian, Scale: 1},
			{Field: civ.FieldToneRX, Offset: 13, Length: 3, Encoding: civ.EncodingBCDNumber, Order: civ.OrderBigEndian, Scale: 1},
			// THE HEADLINE FINDING. Matrix §1b/§3.11: the fifteen-byte
			// TX-duplicate block's first five bytes mirror the RX frequency
			// span "in the same manner as ④~⑧" and are MANUAL-EVIDENCED as
			// the wire's only expression of spec.FieldTxFrequency on this
			// model (spec.md's ruling 7). Same shape, same byte order, same
			// scale as the RX span above — the manual's own cross-reference
			// sentence, not a re-derivation.
			{Field: civ.FieldTXFrequency, Offset: 16, Length: 5, Encoding: civ.EncodingBCDNumber, Order: civ.OrderLittleEndian, Scale: 1},
			// Offsets 21-30 (the duplicate block's remaining ten bytes: TX
			// mode, TX filter, TX data mode, TX tone-mode nibble, TX
			// repeater tone frequency and TX tone-squelch frequency) carry
			// NO neutral field — spec.md's ruling 7: "no neutral field, deliberately
			// zero" — and are left OUTSIDE every span, so the Fixed template
			// speaks for them (consts.go's TXDupUnmappedOffset/Length).
			{Field: civ.FieldName, Offset: 31, Length: 9, Encoding: civ.EncodingName},
		},
		Fixed: make([]byte, RecordOnlyLength),
	}
}

// profile is built once at package init. MustNewProfile rather than
// NewProfile: a mistake here is a build-time defect that must stop the
// programme loudly on first use.
var profile = civ.MustNewProfile(civ.ProfileConfig{
	Model: "IC-7410", RadioAddress: 0x80,
	// ControllerAddress left zero, which selects civ.ControllerAddressDefault
	// (0xE0) — matrix §1 row 2, the "Controller to IC-7410" frame skeleton.
	AddressForm: civ.AddressFormFlat,
	// 1..99 are the memories; 100 and 101 are the two scan edges P1/P2 —
	// one contiguous space, three printed forms (matrix §1b "The banks").
	ChannelLo:     1,
	ChannelHi:     101,
	NameLength:    9, // matrix §1 row 7 / §3.9(i): "9 characters (fixed)"
	NameCharset:   NameCharset,
	NamePad:       0x20, // ASSUMED — matrix §3.9(iii), undocumented
	Layouts:       []civ.RecordLayout{layout()},
	Discriminator: civ.DiscriminatorSingleLength,
	BuildLength:   RecordOnlyLength,
})

// Profile returns the IC-7410's CI-V profile.
func Profile() civ.Profile { return profile }
