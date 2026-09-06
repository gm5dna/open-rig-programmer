// SPDX-License-Identifier: GPL-3.0-or-later

package fakets590

// Image is a factory image: a function returning a freshly populated set of
// memory records. EACH CALL MUST RETURN AN INDEPENDENT MAP, so that multiple
// *Radio instances — or repeated New() calls with one Image value — never
// share mutable state. Every Image in this package is a plain function
// building its map from scratch, which is what makes that hold; a caller
// supplying its own (WithFactoryImage) owes the same property.
// TestDefaultImage_EachCallIsIndependent pins it for DefaultImage, and
// TestTwoRadiosFromOneImageDoNotAlias pins what a shared map would actually
// break.
type Image func() map[recordKey]MemState

// The compile-time proof that this package's own image satisfies the contract
// callers are typed against.
var _ Image = DefaultImage

// THE EVIDENCE POSTURE OF EVERY BYTE BELOW, and the claim it supports.
//
// Every byte value in a record this file builds is a constant, an example or
// a legend value PRINTED IN THE RADIO'S OWN PC-command document. The 50-byte
// RECORD is a synthetic COMPOSITION of those values: no MR, MW or MC frame is
// printed as a literal anywhere in either Kenwood book, so no image's
// cross-field combination has ever been printed or observed. These are not
// observed contents and not factory defaults. That composition is the design's
// A27 and this package's register entry THE DEFAULT IMAGE'S RECORD
// COMPOSITION; the family entries an image rides on are A1, A3, A4, A10, A18a
// and A27, and PROVENANCE.md carries the whole statement with its citations.
//
// No byte here is invented, derived from another model, or padded to make a
// test pass. TestDefaultImage_EveryByteIsAPrintedValue holds the four
// permitted sources mechanically.

// printedExample is the ONE frequency this document prints as a worked value:
// "For example, enter 00014195000 for 14.195 MHz." (590:965). Every record
// this file builds carries it, because there is no second printed frequency
// to vary it with — a consequence accepted rather than engineered around.
const printedExample = "00014195000"

// blankName is P16's eight-byte field filled with the one name state this
// document describes: "If the selected channel is empty, P4 ~ P15 will be 0
// and P16 will be blank." (590:1492-1493). It is the one genuinely
// unevidenced byte-run in a populated record, which is why the shipped images
// use no other value.
const blankName = "        "

// The mode nibbles used below, from the MD legend both charts refer to
// (590:1355-1363).
const (
	modeUSB = '2'
	modeFM  = '4'
)

// defaultRecord is one unremarkable populated record: the printed example
// frequency, FM, and the printed FIRST value of every other legend in the
// grid.
//
// FM IS THE DEFAULT MODE FOR A REASON THAT IS THE DRIVER'S. Every non-FM
// channel write on either 590 row is refused (A23, decision 12), so an image
// whose channels were all SSB would leave the driver's write path with no
// positive control at all — every write refused, including the one that is
// meant to succeed, and the vacuity invisible.
func defaultRecord() MemState {
	return MemState{
		Freq:     printedExample, // 590:965
		Mode:     modeFM,         // "4: FM" (590:1358)
		DataMode: '0',            // "0: DATA mode OFF" (590:447)
		ToneMode: '0',            // "0: TONE/CTCSS OFF" (590:1464)
		ToneNo:   "00",           // TN index 00, 67.0 Hz (590:2296)
		CTCSSNo:  "00",           // CN index 00, 67.0 Hz (590:416)
		Filter:   '0',            // "0: FILTER A" (590:1476)
		FMNarrow: "00",           // "00: FM Normal" (590:1484)
		Lockout:  '0',            // "0: Channel Lockout OFF" (590:1487)
		Name:     blankName,      // 590:1492-1493
	}
}

// emptyRecord is what an unwritten channel answers: "If the selected channel
// is empty, P4 ~ P15 will be 0 and P16 will be blank." (590:1492-1493).
//
// P4-P15 is bytes 7 to 41 of the frame, so EVERY field of the record is its
// zero spelling — the frequency's eleven digits included, and the mode nibble
// with them, which is why nibble '0' is the empty-channel marker on this pair
// and not a parse error. It is DOCUMENTARY FACT on the 590 rows (the design's
// A18a) and carries no assumption of its own.
func emptyRecord() MemState {
	return MemState{
		Freq:     "00000000000",
		Mode:     '0',
		DataMode: '0',
		ToneMode: '0',
		ToneNo:   "00",
		CTCSSNo:  "00",
		Filter:   '0',
		FMNarrow: "00",
		Lockout:  '0',
		Name:     blankName,
	}
}

// DefaultImage is the image New uses when no WithFactoryImage option is
// given: three ordinary memory channels and one section-defined channel's two
// halves, and NOTHING ELSE.
//
// MINIMAL BY DESIGN, AND CONSTRAINED AT BOTH ENDS:
//
//   - AT LEAST ONE ORDINARY MEMORY CHANNEL IS POPULATED, so the fleet's
//     read-every-registered-model pins are non-vacuous against these rows.
//   - BOTH HALVES OF ONE SECTION-DEFINED CHANNEL ARE POPULATED, because the
//     driver reads 100-109 as a PAIR of frames (590:1449-1451) and a
//     one-sided fixture would leave half that path answered by a silence.
//   - EVERY LIVE TWO-VALUE BYTE APPEARS WITH BOTH ITS PRINTED VALUES — byte
//     28 (590:1476-1477), P14 (590:1484-1485), P15 (590:1487-1488) and P6
//     (590:447-449) — so no driver behaviour on one of them is a fixture
//     accident.
//   - NOTHING ELSE IS POPULATED. Nothing about an unwritten channel's
//     contents is printed beyond the empty-channel marker itself
//     (590:1492-1493), and an image of a hundred channels would make every
//     "this channel is empty" assertion a fixture accident instead of a
//     property.
//
// NO IMAGE IS SHIPPED FOR 110-119. The book prints those numbers for the
// TS-590SG (590:1346-1347), but what an extension channel IS is never
// explained, and Stuart ruled on 05/09/2026 that the ten slots are omitted
// from the driver's published banks until that is lifted. There is therefore
// no slot for an image to represent, and the fake serves the same 000-109 on
// both rows — doc.go's register entry THE SG'S EXTENSION CHANNELS ARE NOT
// SERVED.
func DefaultImage() map[recordKey]MemState {
	img := map[recordKey]MemState{}

	// Channel 000 — the plain one: FM, every legend at its printed first
	// value.
	img[recordKey{channel: 0, half: HalfRXOrStart}] = defaultRecord()

	// Channel 001 — the SECOND printed value of every live two-value byte,
	// so that no record in this image is a repeat of another on the axes
	// that matter. Still FM, so P14's "01: FM Narrow" sits on the mode
	// whose legend prints it.
	second := defaultRecord()
	second.ToneMode = '1' // "1: TONE ON" (590:1465)
	second.Filter = '1'   // "1: FILTER B" (590:1477)
	second.FMNarrow = "01"
	second.Lockout = '1' // "1: Channel Lockout ON" (590:1488)
	img[recordKey{channel: 1, half: HalfRXOrStart}] = second

	// Channel 002 — a non-FM channel with DATA mode ON, which is the only
	// way byte 19's second printed value ("1: DATA mode ON", 590:449) gets
	// into the image at all: the DA block says the command may be used "in
	// LSB, USB, FM, and AM mode" (590:454).
	data := defaultRecord()
	data.Mode = modeUSB
	data.DataMode = '1'
	img[recordKey{channel: 2, half: HalfRXOrStart}] = data

	// Channel 100 — the first section-defined channel, "P00 ~ P09 are
	// represented by 100 ~ 109" (590:1345), with BOTH halves populated. The
	// two carry the same frequency because the book prints only one, and
	// that is a state the book itself describes: "When registering a section
	// defined channel and parameter P1 is set to 1, the Start and End
	// frequencies are the same." (590:1537-1538).
	img[recordKey{channel: 100, half: HalfRXOrStart}] = defaultRecord()
	img[recordKey{channel: 100, half: HalfTXOrEnd}] = defaultRecord()

	return img
}
