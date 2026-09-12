// SPDX-License-Identifier: GPL-3.0-or-later

package ic7800

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
//	SelectNibbleOffset   record byte 0 = printed 'e'. Its HIGH nibble is
//	                     implicitly fixed 0; its LOW nibble is the
//	                     four-valued SELECT-group marker.
//	DataModeNibbleOffset record byte 8 = printed '!1'. Its LOW nibble is
//	                     the four-valued data mode; its HIGH nibble is
//	                     tone_mode, which IS mapped. THIS IS THE OPPOSITE
//	                     nibble assignment to the IC-7610 family's
//	                     equivalent byte (matrix §1b) — not a copy-paste
//	                     artefact.
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

// NameCharset is every byte a memory name may carry, transcribed from PDF
// p.209 (folio 14-11), "Codes for memory name, opening message and Clock 2
// name contents" (matrix §1 row 25 — the matrix's own self-review moved
// this table from an earlier draft's p.211/212 to its correct page, p.209).
//
// Alphabetic and symbol ranges are MANUAL-EVIDENCED off that page's two
// subtables. Digits and space are ASSUMED, carried from two OTHER pages'
// tables rather than this one, which prints neither: 0-9 from the
// "Network Radio name contents" table on PDF p.211 (folio 14-13, command
// 1A 05 0213), and space from the CW-message/keyer charset table on PDF
// p.208 (folio 14-10), "space | 20". profile_test.go observes that the
// four together are exactly printable ASCII 0x20-0x7E.
const NameCharset = "" +
	// PDF p.209, "- Character's code- Alphabetical characters": A-Z = 41-5A.
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
	// the same table's second pair: a-z = 61-7A.
	"abcdefghijklmnopqrstuvwxyz" +
	// ASSUMED - carried from PDF p.211's Network Radio name table, 0-9 = 30-39.
	"0123456789" +
	// PDF p.209, "- Character's code- Symbols", in the printed order the
	// transcription (testdata/ic7800-transcription.csv, row !8-@7) records.
	"!#$%&\\?\"'`^+-*/.,:;=<>()[]{}|_~@" +
	// ASSUMED - carried from PDF p.208's CW-message/keyer charset table,
	// "space | 20".
	" "

// modeEnum is record byte 6 (printed 'o'), the "Operating mode setting"
// block's first byte.
//
// SOURCE: PDF p.208 (folio 14-10), "Operating mode" / "Command: 01, 04, 06",
// the two-column table's "Receiving mode" column (matrix §1 row 6).
// Corroborated at PDF p.212 (folio 14-14), "Command: 26", column
// "Operating mode".
//
// UNLIKE THE IC-7610 FAMILY, no leader-crossing hazard and no radix ruling
// is needed here: this document prints the mode table as plain two-digit
// codes with no rotated leader diagram (matrix §1b note, "No
// leader-crossing hazard on this document"). Code 06 (WFM) and codes
// 09-11 are printed nowhere and are deliberately absent — this is an
// HF/50 MHz-only transceiver with no broadcast-FM reception — so a record
// carrying one fails to decode with a parse error naming the offset
// (matrix §1 row 6, completeness ASSUMED, register home
// ic7800-mode-code-completeness).
var modeEnum = map[byte]string{
	0x00: "LSB", 0x01: "USB", 0x02: "AM", 0x03: "CW", 0x04: "RTTY",
	0x05: "FM", 0x07: "CW-R", 0x08: "RTTY-R",
	0x12: "PSK", 0x13: "PSK-R",
}

// filterEnum is record byte 7 (printed '!0'). SOURCE: the same PDF p.208
// table, column "Filter setting" (matrix §1 row 24). Corroborated at
// PDF p.212, "Command: 26".
//
// 0x00 IS NOT A MEMBER, and that is a decision with a consequence: a record
// whose byte 7 is 0x00 fails to decode rather than being read as "no
// filter". The page prints three values and no default; inventing a
// fourth would be a radio claim. Register entry ic7800-filter-value-set.
var filterEnum = map[byte]string{0x01: "FIL1", 0x02: "FIL2", 0x03: "FIL3"}

// toneModeEnum is record byte 8 (printed '!1')'s HIGH nibble.
//
// SOURCE: PDF p.211 (folio 14-13), "!1 Data mode and tone type setting" —
// the upper printed value list, "0: OFF, 1: TONE, 2: TSQL" (matrix §1b).
// UNLIKE the IC-7610 family's equivalent byte, this document's !1 cell
// carries no rotated leader-arrow diagram at all: the two value lists sit
// directly beneath the byte's inline glyph, upper list first for the high
// (tone-type) nibble and lower list second for the low (data-mode) nibble,
// with no leader-crossing to resolve (matrix §1b, "No leader-crossing
// hazard on this document").
//
// ONLY THREE VALUES — no DTCS — which is this model's one genuine
// narrowing relative to the IC-7610 lineage's tone_mode nibble (matrix §1
// row 21). The LOW nibble's four-valued data mode is UNMAPPED under E6 and
// so has no enum here; DataModeNibbleOffset names its byte.
var toneModeEnum = map[byte]string{0x0: "OFF", 0x1: "TONE", 0x2: "TSQL"}

// fixedTemplate is the 25-byte template E6 compares a slot's unmapped
// regions against. Every byte is zero: the only unmapped regions on this
// model are byte 0 (printed 'e': a SELECT marker whose OFF value is 0,
// documented as one full byte carrying only 00-03 — matrix §1 row 5's
// notes on the printed-width-vs-wire-width distinction) and byte 8's high
// nibble (data mode, whose OFF value is 0). Every other byte lies under a
// mapped span, where V8 requires the template to be zero anyway.
//
// Written out explicitly rather than left nil - which would mean the same
// bytes - because E6's ruling is stated in terms of "the profile's Fixed
// template", and an explicit template is what a reader checks against.
//
// FixedTemplate returns a fresh copy for the driver's E6 comparison and
// for tests. A fresh make(), not a shared slice: a caller must not be
// able to move the thing every write is judged against.
func FixedTemplate() []byte { return make([]byte, RecordOnlyLength) }

// layout is the 25-byte record. EVERY OFFSET COMES FROM THE MATRIX'S §1b/§2
// TABLES, cross-checked against testdata/ic7800-transcription.csv. This
// document prints its byte-strip indices as inline glyphs (q, w, e, r...i,
// o, !0, !1, !2...) rather than circled numerals, and — unlike the IC-7610
// family — that glyph run survives text extraction losslessly, so no
// raster re-measurement was needed (matrix §0). Offset is 0-based from the
// start of the RECORD; the two channel-selector bytes q,w are the ADDRESS
// field and lie outside it (spec Erratum 1, applied identically here: a
// literal copy of the IC-7610 record shape — spec.md §1's own finding).
//
//	printed  record byte  offset  width  field
//	e        1            0       1      UNMAPPED (E6): select-group
//	r ~ i    2-6          1       5      rx_frequency
//	o        7            6       1      mode
//	!0       8            7       1      filter
//	!1 hi    9            8       1      tone_mode (high nibble)
//	!1 lo    9            8       -      UNMAPPED (E6): data mode
//	!2 ~ !4  10-12        9       3      tone_tx
//	!5 ~ !7  13-15        12      3      tone_rx
//	!8 ~ @7  16-25        15      10     name
func layout() civ.RecordLayout {
	return civ.RecordLayout{
		Length: RecordOnlyLength,
		Fields: []civ.FieldSpan{
			// PDF p.208's five-cell strip (folio 14-10, cross-referenced
			// from record bytes r~i on PDF p.211): 10 Hz, 1 Hz, 1 kHz,
			// 100 Hz, 10 kHz, 100 kHz, 10 MHz, 1 MHz digits, then a fixed
			// "0:0" pair for 1 GHz/100 MHz — least significant pair first,
			// so little-endian.
			{Field: civ.FieldRXFrequency, Offset: 1, Length: 5, Encoding: civ.EncodingBCDNumber, Order: civ.OrderLittleEndian, Scale: 1},
			{Field: civ.FieldMode, Offset: 6, Length: 1, Encoding: civ.EncodingEnum, Enum: modeEnum},
			{Field: civ.FieldFilter, Offset: 7, Length: 1, Encoding: civ.EncodingEnum, Enum: filterEnum},
			// UNLIKE THE IC-7610 FAMILY (whose equivalent byte carries
			// tone_mode in the LOW nibble), this document's !1 byte prints
			// tone_mode as the HIGH nibble and data mode as the LOW
			// (matrix §1b: "byte !1, high nibble" for tone_mode, "byte
			// !1, low nibble" for data_mode) — a genuine nibble-order
			// swap between the two models, not a copy-paste artefact.
			{Field: civ.FieldToneMode, Offset: 8, Length: 1, Nibble: civ.NibbleHigh, Encoding: civ.EncodingEnum, Enum: toneModeEnum},
			// PDF p.209's three-cell strip (folio 14-11, commands 1B 00 /
			// 1B 01): a fixed "0|0" pair, then 100 Hz, 10 Hz, 1 Hz, 0.1 Hz
			// digits — most significant pair first, so big-endian, the
			// OPPOSITE of the frequency field's convention on this same
			// radio. Scale 1: the wire value is already tenths of a Hz.
			{Field: civ.FieldToneTX, Offset: 9, Length: 3, Encoding: civ.EncodingBCDNumber, Order: civ.OrderBigEndian, Scale: 1},
			{Field: civ.FieldToneRX, Offset: 12, Length: 3, Encoding: civ.EncodingBCDNumber, Order: civ.OrderBigEndian, Scale: 1},
			{Field: civ.FieldName, Offset: 15, Length: 10, Encoding: civ.EncodingName},
		},
		Fixed: make([]byte, RecordOnlyLength),
	}
}

// profile is built once at package init. MustNewProfile rather than
// NewProfile because this is a compile-time constant table: a mistake in it
// is a build-time defect that must stop the programme loudly on first use,
// not an error threaded through model registration
// (core/cat/ftdx101/dialect.go's reasoning, applied to a Profile).
var profile = civ.MustNewProfile(civ.ProfileConfig{
	Model:        "IC-7800",
	RadioAddress: 0x6A,
	// ControllerAddress left zero, which selects civ.ControllerAddressDefault
	// (0xE0) - PDF p.200 (folio 14-2)'s "Controller (PC) to IC-7800" strip.
	AddressForm: civ.AddressFormFlat,
	Groups:      0,
	// 1..99 are the memories; 100 and 101 ARE the scan edges, because
	// BCD(100) is the wire form "01 00" the page prints for P1 and BCD(101)
	// is "01 01" for P2 (PDF p.211, matrix §1 row 5). One contiguous space,
	// three printed forms.
	ChannelLo:     1,
	ChannelHi:     101,
	NameLength:    10,
	NameCharset:   NameCharset,
	NamePad:       0x20, // ASSUMED - register ic7800-name-pad-byte
	Layouts:       []civ.RecordLayout{layout()},
	Discriminator: civ.DiscriminatorSingleLength,
	BuildLength:   RecordOnlyLength,
})

// Profile returns the IC-7800's CI-V profile.
//
// A function over an exported var so the package-held value cannot be
// reassigned by a consumer: a Profile is what the outbound gate consults on
// every frame, and one a caller can swap after init is not a gate.
// civ.Profile is a value type carrying only copied maps and slices, so the
// returned copy is inert in the other direction too.
func Profile() civ.Profile { return profile }
