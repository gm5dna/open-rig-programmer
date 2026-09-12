// SPDX-License-Identifier: GPL-3.0-or-later

package ic7600

import "github.com/gm5dna/open-rig-programmer/core/civ"

// The three figures spec Erratum 1 requires a per-radio package to state
// TOGETHER, with the address width named. RecordOnlyLength is what
// civ.Profile carries and what BuildMemorySet's <record> argument denotes;
// DataAreaLength is the 1A 00 data block the matrix's arithmetic summed
// (matrix S3.11: 1+5+2+1+3+3+10 = 25, plus the 2-byte address = 27, the sum
// equalling the last printed index). Matches the IC-7610's own record
// exactly - matrix S3.11/S5: "no offset differs from spec.md S1."
const (
	RecordOnlyLength = 25
	DataAreaLength   = 27
	AddressBytes     = 2
)

// SelectByteOffset is record byte 0 = printed (3), left UNMAPPED under
// ruling E6 (core/civ/ic7610/doc.go), applied to this radio directly per
// the matrix's S0 "prior art" note and S3.15(a).
//
// A GENUINE DIVERGENCE FROM THE IC-7610's OWN PAGE, RECORDED HERE SO A
// READER DOES NOT ASSUME THE MECHANISM CARRIED ACROSS UNCHANGED (matrix S2
// row 9, S3.15(a)): the IC-7610's page draws (3) as a two-nibble split,
// high nibble a printed "Fixed" 0, low nibble the four-valued SELECT-group
// marker. THIS RADIO'S OWN PAGE (PDF p.178, folio 169, confirmed on a
// direct 300 dpi render) draws (3) as ONE UNDIVIDED enum cell,
// 00 OFF / 01 star1 / 02 star2 / 03 star3, with NO STATED HIGH-NIBBLE
// CONSTRAINT of any kind - no "Fixed" leader, and (matrix S3.13, S3.15(c))
// no printed contradiction against the clear list either, unlike the
// IC-7610's own page.
//
// So this offset is named as a WHOLE BYTE, not a nibble, and the E6
// refusal check (core/driver/ic7600/write.go) compares it whole rather
// than splitting it into two nibble checks: there is no textual hook on
// this page for a "Fixed 0" high-nibble claim to refuse against. The
// vocabulary itself (the four-valued SELECT-group marker) is
// MANUAL-EVIDENCED (PDF p.178, corroborated by command 0E's B0-B2
// sub-commands, PDF p.169 folio 160); WHETHER/HOW to map it is this
// driver's decision, following E6's reasoning rather than re-deriving it:
// a 4-valued marker routed through the neutral BoolField ScanSkip would
// silently collapse a user's SELECT group on write-back while readback
// verification compared equal. Register entry
// ic7600-select-marker-semantics mirrors ic7610-select-marker-semantics.
const SelectByteOffset = 0

// DataModeNibbleOffset is record byte 8 = printed (11)'s HIGH nibble, the
// four-valued data mode, UNMAPPED under E6 for the identical reason as the
// IC-7610's own byte (11): matrix S2 row 20 finds NO divergence here -
// both radios print (11) the same way, a single-byte two-nibble
// sub-diagram with two independent, non-crossing leader labels. The LOW
// nibble is tone_mode, which IS mapped.
const DataModeNibbleOffset = 8

// NameCharset is every byte a memory name may carry.
//
// CHOICE, not a fresh transcription: the matrix's own S1 TagCharset entry
// records a printed contradiction on THIS radio's own page (PDF p.176,
// folio 167) - two character tables captioned for the memory name print
// only lowercase letters and twenty symbols (no digits, no space, no
// uppercase), while the applicability table's own "1A00 / Memory name" row
// says "All characters are available," a broader claim neither adjacent
// table supports. The matrix leaves the question open for the
// driver-authoring stage rather than guess which table the applicability
// line means (matrix S1 TagCharset, S3.9(ii)).
//
// THIS PACKAGE RESOLVES IT AS A CHOICE: adopt the IC-7610's own
// NameCharset verbatim (core/civ/ic7610/profile.go's NameCharset - upper +
// lower + digits + symbols + space), because (a) the "All characters are
// available" applicability line is at least as broad as the IC-7610's set
// and nothing on this page's own contradiction narrows the charset below
// it - the two printed tables are themselves narrower than the
// applicability line they sit under, not a competing wider claim - and
// (b) matrix S1 row 6/S2 confirm every OTHER field on this record is a
// literal, unmodified copy of the IC-7610's, so a driver that also copies
// its name charset keeps one fewer un-evidenced choice in the wave than
// inventing a third vocabulary (narrower-printed-tables-only) would. The
// contradiction itself is NOT resolved by this choice - it is recorded
// here, and the narrower reading (only lowercase + twenty symbols)
// remains available to a future capture in the style of the matrix's own
// R3/R4 lifts: a front-panel entry of a digit or uppercase letter into a
// memory name, photographed.
const NameCharset = "" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
	"abcdefghijklmnopqrstuvwxyz" +
	"0123456789" +
	"!$&?'^-/,;<([{|~" + "#%\\\"`+*.:=>)]}_@" +
	" "

// modeEnum is byte (9), the "Operating mode setting" byte.
//
// SOURCE: PDF p.175 (folio 166), "Operating mode" / "Command: 01, 04, 06",
// corroborated PDF p.178 (folio 169), record diagram brace (9),(10)
// (matrix S1 row 6). IDENTICAL TEN CODES to the IC-7610's own modeEnum -
// no narrowing was actually needed: the IC-7610 package already excludes
// 06 (WFM) and any D-STAR code, so this radio's "-WFM,-DV" delta from
// spec.md S1 lands on a set that was already narrow. Radix: RULING OQ1
// (24/08/2026, core/civ/ic7610/doc.go) applies directly - the printed 12
// and 13 are the wire bytes 0x12 and 0x13, the same family convention
// this document offers no contrary reading of.
var modeEnum = map[byte]string{
	0x00: "LSB", 0x01: "USB", 0x02: "AM", 0x03: "CW", 0x04: "RTTY",
	0x05: "FM", 0x07: "CW-R", 0x08: "RTTY-R",
	0x12: "PSK", 0x13: "PSK-R", // <- RULING OQ1 (inherited), same two keys
}

// filterEnum is byte (10). SOURCE: PDF p.175 (folio 166), "Operating mode"
// table, column "Filter setting"; corroborated PDF p.178 (folio 169).
// Matrix S1 row 24: FIL1, FIL2, FIL3, identical set and codes to the
// IC-7610. 0x00 IS NOT A MEMBER: a record whose (10) is 0x00 fails to
// decode rather than being read as "no filter."
var filterEnum = map[byte]string{0x01: "FIL1", 0x02: "FIL2", 0x03: "FIL3"}

// toneModeEnum is byte (11)'s RIGHT (low) nibble.
//
// SOURCE: PDF p.178 (folio 169), (11) "Data mode setting" sub-diagram,
// lower leader "0: OFF, 1: TONE, 2: TSQL"; corroborated PDF p.175
// (folio 166), band-stacking-register description (matrix S1 row 21). NO
// DTCS value: the document carries no digital-code-squelch vocabulary
// anywhere (matrix S1 row 21's full-document sweep) - this radio predates
// Icom's digital code squelch feature, consistent with spec.md's
// "-DV (no D-STAR)" delta. Identical three-value set to the IC-7610's own
// toneModeEnum; no narrowing was actually needed there either.
//
// The LEFT nibble's four-valued data mode is UNMAPPED under E6 and so has
// no enum here; DataModeNibbleOffset names its byte.
var toneModeEnum = map[byte]string{0x0: "OFF", 0x1: "TONE", 0x2: "TSQL"}

// fixedTemplate is the 25-byte template E6 compares a slot's unmapped
// regions against. Every byte is zero: the only unmapped regions on this
// model are byte 0 (printed (3), the whole-byte SELECT marker, whose OFF
// value is 0 - see SelectByteOffset's comment for why this is a whole-byte
// comparison, not a nibble one) and byte 8's high nibble (data mode, whose
// OFF value is 0).
//
// FixedTemplate returns a fresh copy for the driver's E6 comparison and
// for tests.
func FixedTemplate() []byte { return make([]byte, RecordOnlyLength) }

// layout is the 25-byte record. EVERY OFFSET IS THE IC-7610's OWN,
// UNCHANGED - matrix S3.11's independent derivation from a direct render
// of PDF p.178 agrees exactly, byte for byte, with core/civ/ic7610/doc.go's
// table; "no offset differs from spec.md S1" (matrix S5). Offsets are
// 0-based from the start of the RECORD; the two channel-selector bytes
// (1),(2) are the ADDRESS and lie outside it (spec Erratum 1).
//
//	printed   record bytes  offset  width  field
//	(3)       1             0       1      UNMAPPED (E6, whole byte)
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
			// PDF p.175 (folio 166)'s five-cell strip, corroborated PDF
			// p.178 (folio 169): little-endian packed BCD, identical shape
			// to the IC-7610's own (4)-(8) group.
			{Field: civ.FieldRXFrequency, Offset: 1, Length: 5, Encoding: civ.EncodingBCDNumber, Order: civ.OrderLittleEndian, Scale: 1},
			{Field: civ.FieldMode, Offset: 6, Length: 1, Encoding: civ.EncodingEnum, Enum: modeEnum},
			{Field: civ.FieldFilter, Offset: 7, Length: 1, Encoding: civ.EncodingEnum, Enum: filterEnum},
			{Field: civ.FieldToneMode, Offset: 8, Length: 1, Nibble: civ.NibbleLow, Encoding: civ.EncodingEnum, Enum: toneModeEnum},
			// PDF p.177 (folio 168)'s three-cell strip: big-endian, tenths
			// of a Hz - the opposite byte order to the frequency field, and
			// identical to the IC-7610's own convention.
			{Field: civ.FieldToneTX, Offset: 9, Length: 3, Encoding: civ.EncodingBCDNumber, Order: civ.OrderBigEndian, Scale: 1},
			{Field: civ.FieldToneRX, Offset: 12, Length: 3, Encoding: civ.EncodingBCDNumber, Order: civ.OrderBigEndian, Scale: 1},
			{Field: civ.FieldName, Offset: 15, Length: 10, Encoding: civ.EncodingName},
		},
		Fixed: make([]byte, RecordOnlyLength),
	}
}

// profile is built once at package init. MustNewProfile rather than
// NewProfile because this is a compile-time constant table: a mistake in
// it is a build-time defect that must stop the programme loudly on first
// use, not an error threaded through model registration.
var profile = civ.MustNewProfile(civ.ProfileConfig{
	Model:        "IC-7600",
	RadioAddress: 0x7A,
	// ControllerAddress left zero, which selects civ.ControllerAddressDefault
	// (0xE0) - PDF p.168 (folio 159)'s "Controller to IC-7600" strip, and
	// PDF p.151 (folio 142)'s CI-V Address item, which states the range
	// 01h-DFh and the front-panel main-dial mechanism directly (a genuine
	// improvement in evidence quality over the IC-7610's own document -
	// matrix S3.4).
	AddressForm: civ.AddressFormFlat,
	Groups:      0,
	// 1..99 are the memories; 100 and 101 ARE the scan edges P1/P2 - matrix
	// S1 row 5, identical shape to the IC-7610.
	ChannelLo:     1,
	ChannelHi:     101,
	NameLength:    10,
	NameCharset:   NameCharset,
	NamePad:       0x20, // ASSUMED - D5 entry 3/4, lift R3/R4
	Layouts:       []civ.RecordLayout{layout()},
	Discriminator: civ.DiscriminatorSingleLength,
	BuildLength:   RecordOnlyLength,
})

// Profile returns the IC-7600's CI-V profile.
//
// A function over an exported var so the package-held value cannot be
// reassigned by a consumer: a Profile is what the outbound gate consults
// on every frame, and one a caller can swap after init is not a gate.
func Profile() civ.Profile { return profile }
