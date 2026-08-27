// SPDX-License-Identifier: GPL-3.0-or-later

package ic9700

import "github.com/gm5dna/open-rig-programmer/core/civ"

// THE THREE LENGTH NUMBERS THIS MODEL IS EASIEST TO CONFUSE (spec
// Erratum 1). The wire presents 114 bytes where the profile declares 111,
// and every later file in this tier cites these by NAME rather than
// repeating a digit.
const (
	// RecordLength is the RECORD-ONLY length — the payload after the
	// three address bytes, which is what civ.Profile's layout, the gate
	// and BuildMemorySet's <record> argument all denote (spec Erratum 1).
	RecordLength = 111
	// AddressBytes is the (band, channel) address ahead of the record:
	// ① one packed-BCD band code, ②,③ two packed-BCD channel bytes.
	AddressBytes = 3
	// DataAreaLength is what the WIRE shows between `1A 00` and `FD` on
	// an occupied-channel answer, and is spec D6's figure for this model.
	DataAreaLength = RecordLength + AddressBytes // 114
)

// nameCharset is every printable ASCII byte, 0x20..0x7E.
//
// The page prints two "Codes for character entries" tables — letters and
// numbers, then symbols — for the memory name, and NEITHER table prints a
// space row. A third table maps commands to set items and prints,
// verbatim, for `1A 00`: "Memory name / All characters are usable." (leg
// B's `(52) ~ (67)` row). The document DOES print "(Space) / 20" twice,
// but both times for a DIFFERENT field — PDF p.21's call-sign (`1F 01`)
// and DV TX message (`1F 02`) tables, and PDF p.16's memory-KEYER table
// (`1A 02`) — never the memory name. Spec D5 entry 3 records that most
// Icom charset tables omit the space while the radios plainly accept one;
// this model's memory-name tables fit that family pattern rather than
// being an exception, so ACCEPTING 0x20 in a written name is ASSUMED, not
// printed (matrix §3.9, this model's own row, space half).
const nameCharset = " !\"#$%&'()*+,-./0123456789:;<=>?@" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`" +
	"abcdefghijklmnopqrstuvwxyz{|}~"

// The record's enums, one map per printed value list. Each is transcribed
// from a named page and nothing is interpolated: a wire value the page
// does not print is a value this profile refuses, in both directions.

// selectNames is ④'s LOW nibble — the SELECT-memory scan-group tag, NOT a
// skip flag. The page prints `0=OFF*`, `1=★1`, `2=★2`, `3=★3` (leg B's ④
// row, PDF p.15). The star glyphs are spelt SEL1/SEL2/SEL3 here because a
// neutral name is what civ.FieldSelect carries and a black star is not
// one; the wire values are unchanged.
//
// See doc.go and the driver's OQ-4 record: the driver reports
// spec.FieldScanSkip Unsupported on every bank rather than mapping a
// four-valued group tag onto a boolean.
var selectNames = map[byte]string{
	0x00: "OFF",
	0x01: "SEL1",
	0x02: "SEL2",
	0x03: "SEL3",
}

// modeNames is ⑩, the operating mode. matrix §1 #4, PDF p.14's operating
// mode table.
//
// THE BASE IS ASSUMED HEXADECIMAL — register entry
// `ic9700-mode-codes-are-hexadecimal`, doc.go. For eight of the ten it
// does not matter; DV and DD are the two where reading the printed glyphs
// as decimal would give bytes 0x11 and 0x16 instead.
var modeNames = map[byte]string{
	0x00: "LSB", 0x01: "USB", 0x02: "AM", 0x03: "CW", 0x04: "RTTY",
	0x05: "FM", 0x07: "CW-R", 0x08: "RTTY-R", 0x17: "DV", 0x22: "DD",
}

// filterNames is ⑪. PDF p.14, the filter table rows `01:FIL1`,
// `02:FIL2`, `03:FIL3` — leg G's byte walk cites the first of them for
// frame byte 17. THERE IS NO `00`: the page prints none, so a record
// carrying one fails to parse rather than decoding to a filter no
// document names.
var filterNames = map[byte]string{
	0x01: "FIL1",
	0x02: "FIL2",
	0x03: "FIL3",
}

// dataModeNames is ⑫. Leg B's ⑫ row: "1 byte data (XX) | 00: Data mode
// OFF | 01: Data mode ON".
var dataModeNames = map[byte]string{
	0x00: "OFF",
	0x01: "ON",
}

// duplexNames is ⑬'s HIGH nibble: `0=Duplex OFF`, `1=Duplex−`,
// `2=Duplex+`, `3=RPS` (leg B's ⑬ row, leaders followed by eye).
//
// ALL FOUR WIRE VALUES ARE CARRIED HERE so the record round-trips
// exactly, and that is deliberate even though core/spec's
// DuplexDirection has only three (OQ-6): a channel set to RPS is a
// channel this dialect can READ and the driver then REFUSES to write,
// which is honest. Flattening RPS onto OFF at this layer would make the
// refusal impossible, because nothing downstream would know.
var duplexNames = map[byte]string{
	0x00: "OFF",
	0x01: "DUP-",
	0x02: "DUP+",
	0x03: "RPS",
}

// toneModeNames is ⑬'s LOW nibble: `0=OFF`, `1=TONE`, `2=TSQL`,
// `3=DTCS` (leg B's ⑬ row).
var toneModeNames = map[byte]string{
	0x00: "OFF",
	0x01: "TONE",
	0x02: "TSQL",
	0x03: "DTCS",
}

// dtcsPolarityNames is ㉑, a WHOLE byte whose two nibbles are two
// independent polarities: PDF p.21 "DTCS code and polarity setting"
// prints `0=Normal 1=Reverse` for each, the HIGH nibble carrying the
// TRANSMIT polarity and the LOW the RECEIVE one.
//
// THE PRINTED LABEL ORDER IS REVERSED AND THE LEADERS SAY SO — leg G's
// recorded hazard (c): "Receive polarity" is printed ABOVE "Transmit
// polarity", but the upper label's leader lands on the RIGHT half of the
// cell and the lower label's on the LEFT. Followed by eye, left =
// transmit, right = receive, which is what this map encodes. Read in
// printed vertical order instead, every DTCS channel would have its two
// polarities swapped.
//
// The names are TX-then-RX: "NR" is transmit Normal, receive Reverse.
var dtcsPolarityNames = map[byte]string{
	0x00: "NN",
	0x01: "NR",
	0x10: "RN",
	0x11: "RR",
}

// duplicateBlockShift is the record-offset distance from a field in the
// primary block ⑤~51 to its copy in the filled block ❺~❺❶: the primary
// block is 47 bytes wide and the copy begins immediately after it.
//
// The manual's grey NOTE asserts the identity — "The same data as ⑤ ~ 51
// are stored in ❺ ~ 51" — and this layout maps the copies as REPEATS of
// their primary field ids, so civ.encodeRecord writes both and
// civ.decodeRecord REQUIRES them to agree. A radio whose two blocks
// disagree fails to parse rather than letting one copy win silently.
// Register entry `ic9700-duplicate-block-agrees-on-read` (lift W2).
const duplicateBlockShift = 47

// fixedTemplateBytes is the RecordLayout.Fixed template: the value of
// every record byte NO field span maps.
//
// IT IS DERIVED FROM THE FROZEN GOLDEN, NOT ASSUMED BLANK. The
// set-record-name-with-space vector carries `43 51 43 51 43 51 20 20` —
// `CQCQCQ` plus two pad spaces — at record offsets 24..31 and again at
// 71..78, and blanks at the other two call signs. civ.encodeRecord fills
// unmapped bytes from this template, so a blank one would make the
// builder disagree with the frozen evidence at twelve offsets.
// profile_test.go's TestFixedTemplateIsTheGoldensUnmappedState proves the
// equality against the vector itself.
//
// WHAT THE CHOICE COSTS, and why it is safe to ship on an inference.
// Under the tier's E6 rule a slot is writable only when its unmapped
// regions EQUAL this template, so the one channel state this tier can
// write is the one leg G transcribed. `CQCQCQ` is the D-STAR broadcast
// destination and is the plausible factory state of an untouched memory,
// but no capture confirms it. The guard compares and REFUSES on mismatch
// in every case, so a wrong template costs MORE REFUSALS and never
// corruption — and the refusal happens before any frame is built, so no
// wrong-template outcome reaches a radio. Register entry
// `ic9700-unmapped-template-is-the-golden-state` (lift R23).
//
// Every byte not named below is 0x00, and V8 refuses a non-zero template
// nibble under a mapped span, so no mapped offset appears here.
var fixedTemplateBytes = [RecordLength]byte{
	// ⑭ digital squelch and ㉔ DV code squelch: one byte each, `00` in
	// the golden. Stated rather than left to the array's zero so the two
	// unmapped one-byte fields are visible as regions.
	10: 0x00,
	20: 0x00,

	// ㉘~㉟ UR (Destination) call sign — `CQCQCQ` + two pad spaces.
	24: 'C', 25: 'Q', 26: 'C', 27: 'Q', 28: 'C', 29: 'Q', 30: ' ', 31: ' ',
	// ㊱~㊸ R1 (Access repeater) call sign — blank.
	32: ' ', 33: ' ', 34: ' ', 35: ' ', 36: ' ', 37: ' ', 38: ' ', 39: ' ',
	// ㊹~51 R2 (Gateway/Link repeater) call sign — blank.
	40: ' ', 41: ' ', 42: ' ', 43: ' ', 44: ' ', 45: ' ', 46: ' ', 47: ' ',

	// The filled block's copies of the same five regions, at +47.
	57: 0x00,
	67: 0x00,
	71: 'C', 72: 'Q', 73: 'C', 74: 'Q', 75: 'C', 76: 'Q', 77: ' ', 78: ' ',
	79: ' ', 80: ' ', 81: ' ', 82: ' ', 83: ' ', 84: ' ', 85: ' ', 86: ' ',
	87: ' ', 88: ' ', 89: ' ', 90: ' ', 91: ' ', 92: ' ', 93: ' ', 94: ' ',
}

// fixedTemplate returns a fresh copy of the template, so no caller can
// reach into the package's own array.
func fixedTemplate() []byte {
	out := fixedTemplateBytes
	return out[:]
}

// enumSpan is one EncodingEnum field. Nibble selects the half of the byte
// it occupies; civ.NibbleWhole is the whole byte.
func enumSpan(id civ.FieldID, offset int, nibble civ.NibbleSel, enum map[byte]string) civ.FieldSpan {
	return civ.FieldSpan{
		Field:    id,
		Offset:   offset,
		Length:   1,
		Nibble:   nibble,
		Encoding: civ.EncodingEnum,
		Enum:     enum,
	}
}

// bcdSpan is one EncodingBCDNumber field. Scale multiplies the wire value
// to reach the neutral unit: 1 where the wire already carries Hz or
// tenths of a Hz, 100 where the field's lowest printed digit place is the
// 100 Hz one.
func bcdSpan(id civ.FieldID, offset, length int, order civ.ByteOrder, scale uint64) civ.FieldSpan {
	return civ.FieldSpan{
		Field:    id,
		Offset:   offset,
		Length:   length,
		Encoding: civ.EncodingBCDNumber,
		Order:    order,
		Scale:    scale,
	}
}

// recordFields is the 111-byte record, term by term, in record order.
//
// Record offset 0 is printed index ④ — leg B's measured data-area
// position 4, less the three address bytes. The primary block ⑤~51
// occupies offsets 1..47; the filled duplicate ❺~❺❶ repeats it at
// +duplicateBlockShift, offsets 48..94; the name occupies 95..110.
//
// Fifty-two of the 111 bytes have no civ.FieldID home — ⑭, ㉔, the three
// eight-byte call signs, and each of their copies — and are written from
// the Fixed template above. Register entry
// `ic9700-unmapped-regions-refused` (lift W8).
func recordFields() []civ.FieldSpan {
	const d = duplicateBlockShift
	return []civ.FieldSpan{
		// ④ low nibble. The HIGH nibble is printed as a literal `0` with
		// the leader label "Fixed" and stays with the template.
		enumSpan(civ.FieldSelect, 0, civ.NibbleLow, selectNames),

		// ⑤~⑨ operating frequency, five packed-BCD bytes, least
		// significant digit pair first, in Hz. PDF p.14 labels cell n's
		// halves 10^(2n−1) and 10^(2n−2).
		bcdSpan(civ.FieldRXFrequency, 1, 5, civ.OrderLittleEndian, 1),

		enumSpan(civ.FieldMode, 6, civ.NibbleWhole, modeNames),     // ⑩
		enumSpan(civ.FieldFilter, 7, civ.NibbleWhole, filterNames), // ⑪
		enumSpan(civ.FieldDataMode, 8, civ.NibbleWhole, dataModeNames),

		// ⑬, one byte and two independent enums.
		enumSpan(civ.FieldDuplex, 9, civ.NibbleHigh, duplexNames),
		enumSpan(civ.FieldToneMode, 9, civ.NibbleLow, toneModeNames),

		// ⑭ (offset 10) is unmapped.

		// ⑮~⑰ and ⑱~⑳: three packed-BCD bytes each, MOST significant
		// pair first, in TENTHS of a Hz. PDF p.21 prints the digit
		// places 100 Hz / 10 Hz / 1 Hz / 0.1 Hz, and ⑮'s two halves are
		// both a literal "Fixed digit: 0".
		bcdSpan(civ.FieldToneTX, 11, 3, civ.OrderBigEndian, 1),
		bcdSpan(civ.FieldToneRX, 14, 3, civ.OrderBigEndian, 1),

		enumSpan(civ.FieldDTCSPolarity, 17, civ.NibbleWhole, dtcsPolarityNames), // ㉑
		// ㉒㉓, the printed code as a decimal integer. ㉒'s high nibble is
		// a printed literal "0 (fixed)", which a big-endian BCD field
		// covers by carrying a leading zero.
		bcdSpan(civ.FieldDTCSCode, 18, 2, civ.OrderBigEndian, 1),

		// ㉔ (offset 20) is unmapped.

		// ㉕~㉗ duplex offset. THIS FIELD IMPLEMENTS LEG G'S READING, NOT
		// THE PRINTED §1b ROW, and the two disagree about the page. Leg
		// G's provenance gives the printed half-labels as "1 kHz /
		// 100 Hz, 100 kHz / 10 kHz, 10 MHz / 1 MHz" and derives 600 kHz
		// from `00 60 00`; the wire value is a count of 100 Hz units and
		// the scale is 100. §1b's own `offset` row instead prints "digit
		// places 1 kHz … 10 MHz" — five places for a six-nibble field —
		// under which the same bytes would read 6 MHz. ONE OF THE TWO IS
		// WRONG ABOUT THE PAGE, and the disagreement is UNRESOLVED (matrix
		// Erratum 14). STATUS: ASSUMED — register entry
		// `ic9700-offset-scale-100hz`, doc.go. A wrong choice is a factor
		// of ten on every offset. LIFTED BY: a hardware capture of one
		// known offset, read back and compared.
		bcdSpan(civ.FieldOffset, 21, 3, civ.OrderLittleEndian, 100),

		// ㉘~㉟, ㊱~㊸, ㊹~51 (offsets 24..47) are unmapped.

		// ❺~❾ — the ONE field the duplicate renames. Everything else in
		// the filled block repeats its primary's id at +47, which is how
		// this layout states the printed NOTE's identity claim in a form
		// the codec enforces.
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

		// 52~67 memory name, sixteen bytes of the profile's own charset.
		{
			Field:    civ.FieldName,
			Offset:   95,
			Length:   16,
			Encoding: civ.EncodingName,
		},
	}
}

// profile is this dialect's civ.Profile. MustNewProfile, not NewProfile:
// the config is package data with no runtime input, so a validation
// failure is a transcription error that must not be deferred to a caller.
var profile = civ.MustNewProfile(civ.ProfileConfig{
	Model: "IC-9700",

	// PDF p.4 "Preparing": the transceiver's default address is A2, and
	// E0 is the CI-V convention for a PC controller. Leg G recorded both
	// from the frame picture, whose leaders cross.
	RadioAddress:      0xA2,
	ControllerAddress: 0xE0,

	// MaxFrame 0 selects civ.DefaultMaxFrame (256). This profile's own
	// longest frame is the 121-byte memory set, so the default clears it
	// with room; stated explicitly because ProfileConfig's standing rule
	// is that an omitted field is a decision, not an oversight.
	MaxFrame: 0,

	// ① is a BAND code, not a group index: 01 = 144 MHz, 02 = 430 MHz,
	// 03 = 1.2 GHz (leg B's D2 ① row, PDF p.16). Under E4 Group carries
	// the WIRE index, so the base is 1 and the wire bytes are 01/02/03
	// directly.
	AddressForm: civ.AddressFormBandChannel,
	Groups:      3,
	GroupBase:   1,

	// ②,③, the four printed decimal digits: 0001~0099 memory channels,
	// 0100~0105 the six program scan edge channels (1A/1B, 2A/2B,
	// 3A/3B), 0106~0107 the two call channels (C1, C2). Leg B's ②,③ row.
	ChannelLo: 1,
	ChannelHi: 107,

	NameLength:  16,
	NameCharset: nameCharset,
	NamePad:     0x20,

	Layouts: []civ.RecordLayout{{
		Length: RecordLength,
		Fields: recordFields(),
		Fixed:  fixedTemplate(),
	}},

	// No field on this model has a conditional printed width, so the
	// accepted set is the singleton {111} and the length discriminates
	// nothing. Declared rather than inferred: a two-layout profile that
	// meant to have one is a transcription error nothing else would
	// report.
	Discriminator: civ.DiscriminatorSingleLength,
	BuildLength:   RecordLength,
})

// Profile returns the IC-9700's CI-V profile: the record geometry, the
// address form, the name policy and the enums, as data.
//
// It is DATA AND NOTHING ELSE. No frame is sent, no port is opened and no
// radio is addressed by anything in this package.
func Profile() civ.Profile { return profile }
