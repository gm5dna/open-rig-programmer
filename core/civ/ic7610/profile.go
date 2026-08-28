// SPDX-License-Identifier: GPL-3.0-or-later

package ic7610

import "github.com/gm5dna/open-rig-programmer/core/civ"

// The three figures spec Erratum 1 requires a per-radio package to state
// TOGETHER, with the address width named. RecordOnlyLength is what
// civ.Profile carries and what BuildMemorySet's <record> argument denotes;
// DataAreaLength is the 1A 00 data block the matrix's arithmetic summed
// (matrix S3.11: 2+1+5+2+1+3+3+10 = 27, the sum equalling the last printed
// index). Neither accounting is wrong; the tier needs one convention, and
// this package uses the record-only one everywhere below.
const (
	RecordOnlyLength = 25
	DataAreaLength   = 27
	AddressBytes     = 2
)

// The two record regions ruling E6 leaves UNMAPPED on this model, exported
// so the driver's refusal check names them rather than re-deriving them.
//
//	SelectNibbleOffset   record byte 0 = printed (3). Its HIGH nibble is
//	                     the page's printed "Fixed" 0; its LOW nibble is
//	                     the four-valued SELECT-group marker.
//	DataModeNibbleOffset record byte 8 = printed (11). Its HIGH nibble is
//	                     the four-valued data mode; its LOW nibble is
//	                     tone_mode, which IS mapped.
//
// Neither four-valued field has a faithful neutral home: codeplug's
// ScanSkip and DataMode are both BoolField, and a 4->2 collapse would
// rewrite a user's SELECT group or data mode on every write-back while
// readback verification compared equal. E6 rules them unmapped; the driver
// refuses to write a slot whose actual bytes differ from FixedTemplate().
const (
	SelectNibbleOffset   = 0
	DataModeNibbleOffset = 8
)

// NameCharset is every byte a memory name may carry, built table by table
// from PDF p.12 (folio 11), "Codes for character entries" / "Command:
// 1A 00, 1A 05 ...", as the B leg transcribed it (testdata/
// ic7610-transcription-b.csv, row D1,(18)~(27)).
//
// FOUR SOURCES, kept apart so each is checkable on its own, and the fourth
// is ASSUMED. profile_test.go then observes that the four together are
// exactly printable ASCII 0x20-0x7E - an observation, not the definition,
// because a definition by range would stop being a transcription.
const NameCharset = "" +
	// "- Character codes- Letters and Numbers", row 1: A-Z = 41-5A.
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
	// the same table, row 1 right pair: a-z = 61-7A.
	"abcdefghijklmnopqrstuvwxyz" +
	// the same table, row 2: 0-9 = 30-39.
	"0123456789" +
	// "- Character codes- Symbols", both column pairs, in printed order.
	"!$&?'^-/,;<([{|~" + "#%\\\"`+*.:=>)]}_@" +
	// ASSUMED - D5 entry 3, lift R3. Neither 1A 00 character table prints
	// a space row; the same block's footnote lists "(space)" among the
	// usable memory-name characters, and the document prints a space's
	// ASCII code twice, both times under OTHER commands (PDF p.11
	// "Codes for CW message contents", row "Space | 20"; PDF p.14
	// "Memory keyer character entries", row "space | 20 | Word space").
	// The G leg derived its set-record vector's byte 28 from those rows.
	" "

// modeEnum is byte (9), the "Operating mode setting" block's first byte.
//
// SOURCE: PDF p.11 (folio 10), "Operating mode" / "Command: 01, 04, 06",
// the two-column table's "Receiving mode" column, left sub-column then
// right (B leg row D1,(9),(10)). Corroborated at PDF p.14 (folio 13),
// "Command: 26", column "Operating mode".
//
// THE RADIX OF THE LAST TWO KEYS IS A RULING, NOT A READING - see doc.go's
// register entry ic7610-mode-code-radix and the plan's OQ1. The ruling of
// 24/08/2026 is HEXADECIMAL: the printed 12 and 13 are the wire bytes 0x12
// and 0x13. Codes 06 and 09-11 are printed nowhere and are deliberately
// absent: a record carrying one fails to decode with a parse error naming
// the offset, which is the honest outcome (entry
// ic7610-mode-code-completeness, lift R19).
var modeEnum = map[byte]string{
	0x00: "LSB", 0x01: "USB", 0x02: "AM", 0x03: "CW", 0x04: "RTTY",
	0x05: "FM", 0x07: "CW-R", 0x08: "RTTY-R",
	0x12: "PSK", 0x13: "PSK-R", // <- RULING OQ1 sets these two keys
}

// filterEnum is byte (10). SOURCE: the same PDF p.11 table, column
// "Filter setting", whose fourth and fifth rows are printed "-" (matrix
// Errata (rev 1) erratum 6). Corroborated at PDF p.14, "Command: 26".
//
// 0x00 IS NOT A MEMBER, and that is a decision with a consequence: a record
// whose (10) is 0x00 fails to decode rather than being read as "no filter".
// The page prints three values and no default; inventing a fourth would be
// a radio claim. Register entry ic7610-filter-value-set.
var filterEnum = map[byte]string{0x01: "FIL1", 0x02: "FIL2", 0x03: "FIL3"}

// toneModeEnum is byte (11)'s RIGHT (low) nibble.
//
// THE ASSIGNMENT IS INVERTED RELATIVE TO READING ORDER, AND MUST NOT BE
// "CORRECTED". PDF p.12's (11) sub-diagram stacks two labels to the right
// of the box; the FIRST-PRINTED (upper) label "0: OFF, 1: TONE, 2: TSQL"
// belongs to the RIGHT nibble and the SECOND-PRINTED (lower) label
// "0: OFF, 1: DATA 1, 2: DATA 2, 3: DATA 3" to the LEFT. The two leaders
// NEST rather than cross (matrix Errata (rev 1) erratum 5, which exists
// precisely so a later reader who re-renders the page, finds no crossing
// and concludes the matrix misread the artwork does not flip the two -
// which would be a data-corrupting error). The W leg (STOP 5), the B leg
// (rows D1,(11) and D3,(11)) and the G leg (hazard (c)) each traced the
// leaders independently and agree.
//
// The LEFT nibble's four-valued data mode is UNMAPPED under E6 and so has
// no enum here; DataModeNibbleOffset names its byte.
var toneModeEnum = map[byte]string{0x0: "OFF", 0x1: "TONE", 0x2: "TSQL"}

// fixedTemplate is the 25-byte template E6 compares a slot's unmapped
// regions against. Every byte is zero: the only unmapped regions on this
// model are byte 0 (printed (3): a "Fixed" 0 high nibble and a SELECT
// marker whose OFF value is 0) and byte 8's high nibble (data mode, whose
// OFF value is 0). Every other byte lies under a mapped span, where V8
// requires the template to be zero anyway.
//
// Written out explicitly rather than left nil - which would mean the same
// bytes - because E6's ruling is stated in terms of "the profile's Fixed
// template", and an explicit template is what a reader checks against.
func fixedTemplate() []byte { return make([]byte, RecordOnlyLength) }

// FixedTemplate returns a fresh copy of the template, for the driver's E6
// comparison and for tests. A copy, not the slice: a caller must not be
// able to move the thing every write is judged against.
func FixedTemplate() []byte { return fixedTemplate() }

// layout is the 25-byte record. EVERY OFFSET COMES FROM THE PLAN'S ONE
// TABLE. Offsets are 0-based from the start of the RECORD, so a printed
// index N sits at Offset N-3: the two channel-selector bytes (1),(2) are
// the ADDRESS field and lie outside the record (spec Erratum 1).
//
//	printed   record bytes  offset  width  field
//	(3)       1             0       1      UNMAPPED (E6)
//	(4)~(8)   2-6           1       5      rx_frequency
//	(9)       7             6       1      mode
//	(10)      8             7       1      filter
//	(11) hi   9             8       -      UNMAPPED (E6)
//	(11) lo   9             8       1      tone_mode (low nibble)
//	(12)~(14) 10-12         9       3      tone_tx
//	(15)~(17) 13-15         12      3      tone_rx
//	(18)~(27) 16-25         15      10     name
func layout() civ.RecordLayout {
	return civ.RecordLayout{
		Length: RecordOnlyLength,
		Fields: []civ.FieldSpan{
			// PDF p.11's five-cell strip, ten rotated digit labels running
			// 10 Hz, 1 Hz, 1 kHz, 100 Hz, 100 kHz, 10 kHz, 10 MHz, 1 MHz,
			// then a fifth cell printed "0 : 0" labelled 1 GHz and 100 MHz
			// "(Fixed)": least significant pair first, so little-endian.
			{Field: civ.FieldRXFrequency, Offset: 1, Length: 5, Encoding: civ.EncodingBCDNumber, Order: civ.OrderLittleEndian, Scale: 1},
			{Field: civ.FieldMode, Offset: 6, Length: 1, Encoding: civ.EncodingEnum, Enum: modeEnum},
			{Field: civ.FieldFilter, Offset: 7, Length: 1, Encoding: civ.EncodingEnum, Enum: filterEnum},
			{Field: civ.FieldToneMode, Offset: 8, Length: 1, Nibble: civ.NibbleLow, Encoding: civ.EncodingEnum, Enum: toneModeEnum},
			// PDF p.14's three-cell strip: cell 1 printed "0 | 0", then
			// 100 Hz, 10 Hz, 1 Hz, 0.1 Hz - most significant pair first, so
			// big-endian, and the OPPOSITE of the frequency field's
			// convention on the same radio. Scale 1: the wire value is
			// already tenths of a Hz, which is MemoryRecord's own unit.
			{Field: civ.FieldToneTX, Offset: 9, Length: 3, Encoding: civ.EncodingBCDNumber, Order: civ.OrderBigEndian, Scale: 1},
			{Field: civ.FieldToneRX, Offset: 12, Length: 3, Encoding: civ.EncodingBCDNumber, Order: civ.OrderBigEndian, Scale: 1},
			{Field: civ.FieldName, Offset: 15, Length: 10, Encoding: civ.EncodingName},
		},
		Fixed: fixedTemplate(),
	}
}

// profile is built once at package init. MustNewProfile rather than
// NewProfile because this is a compile-time constant table: a mistake in it
// is a build-time defect that must stop the programme loudly on first use,
// not an error threaded through model registration
// (core/cat/ftdx101/dialect.go's reasoning, applied to a Profile).
var profile = civ.MustNewProfile(civ.ProfileConfig{
	Model:        "IC-7610",
	RadioAddress: 0x98,
	// ControllerAddress left zero, which selects civ.ControllerAddressDefault
	// (0xE0) - the address PDF p.3's "Controller to IC-7610" strip prints.
	AddressForm: civ.AddressFormFlat,
	Groups:      0,
	// 1..99 are the memories; 100 and 101 ARE the scan edges, because
	// BCD(100) is the wire form "01 00" the page prints for P1 and BCD(101)
	// is "01 01" for P2. One contiguous space, three printed forms.
	ChannelLo:     1,
	ChannelHi:     101,
	NameLength:    10,
	NameCharset:   NameCharset,
	NamePad:       0x20, // ASSUMED - D5 entry 3, lift R4
	Layouts:       []civ.RecordLayout{layout()},
	Discriminator: civ.DiscriminatorSingleLength,
	BuildLength:   RecordOnlyLength,
})

// Profile returns the IC-7610's CI-V profile.
//
// A function over an exported var so the package-held value cannot be
// reassigned by a consumer: a Profile is what the outbound gate consults on
// every frame, and one a caller can swap after init is not a gate.
// civ.Profile is a value type carrying only copied maps and slices, so the
// returned copy is inert in the other direction too.
func Profile() civ.Profile { return profile }
