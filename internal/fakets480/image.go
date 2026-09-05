// SPDX-License-Identifier: GPL-3.0-or-later

package fakets480

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
// a legend value PRINTED IN THIS RADIO'S OWN PC-command document. The 50-byte
// RECORD is a synthetic COMPOSITION of those values: no MR, MW or MC frame is
// printed as a literal anywhere in either Kenwood book, so no image's
// cross-field combination has ever been printed or observed. These are not
// observed contents and not factory defaults. That composition is the design's
// A27 and this package's register entry THE DEFAULT IMAGE'S RECORD
// COMPOSITION; the family entries an image rides on are A1, A3, A4, A24 and
// A27, and PROVENANCE.md carries the whole statement with its citations.
//
// No byte here is invented, derived from another model, or padded to make a
// test pass. TestDefaultImage_EveryByteIsAPrintedValue holds the four
// permitted sources mechanically.

// printedExample is the frequency this document prints as a worked value:
// "For example, 00014195000 for 14.195 MHz." (480:549, and again at 480:572
// and 480:1746). Every record this file builds carries it, because there is
// no second printed frequency to vary it with — a consequence accepted rather
// than engineered around.
const printedExample = "00014195000"

// blankName is P16's eight-byte field filled with spaces.
//
// IT IS THE ONE GENUINELY UNEVIDENCED BYTE-RUN IN A POPULATED RECORD ON THIS
// ROW, and more so than on the 590 pair. That book at least prints one name
// state — "If the selected channel is empty ... P16 will be blank."
// (590:1492-1493) — and THIS one prints nothing about an empty channel
// anywhere; all it says of P16 is "A maximum of 8 characters." (480:941).
// Eight spaces is the design's A3, whose TS-480 half is unlifted, and the
// shipped images use no other value so that the assumption has exactly one
// place to be corrected.
const blankName = "        "

// The mode nibbles used below, from the MD legend both memory charts refer to
// (480:843-854).
const (
	modeUSB = '2'
	modeFM  = '4'
)

// The step indices used below. ST prints TWO legends — 00 ~ 04 for
// SSB/CW/FSK and 00 ~ 09 for AM/FM, where 00 means 0.5 kHz in the first and
// 5 kHz in the second (480:1494-1500) — and these two are printed in BOTH, so
// no record's step depends on which class its own mode nibble falls in. That
// ambiguity is the design's A22 and this image deliberately does not exercise
// it.
const (
	stepFirstPrinted = "00"
	stepAlsoInBoth   = "03"
)

// defaultRecord is one unremarkable populated record: the printed example
// frequency, FM, and the printed FIRST value of every other legend in the
// grid.
//
// THE MODE CHOICE CARRIES NO DRIVER LEVER HERE, unlike the 590 pair's. There
// every non-FM channel write is refused (A23), so an all-SSB image would
// leave that write path with no positive control; on this row EVERY channel
// write is refused (A22), so the mode nibble decides nothing about a write
// and is varied across the image only so that no read behaviour on one
// nibble is a fixture accident.
func defaultRecord() MemState {
	return MemState{
		Freq:     printedExample,   // 480:549
		Mode:     modeFM,           // "4: FM" (480:848)
		Lockout:  '0',              // "0: Lockout OFF" (480:920)
		ToneMode: '0',              // "0: OFF" (480:922)
		ToneNo:   "00",             // the first TN index (480:1557)
		CTCSSNo:  "00",             // the first CN index (480:337)
		Step:     stepFirstPrinted, // 480:1495, 480:1498
		Name:     blankName,        // A3, unlifted on this row
	}
}

// emptyRecord is what an unwritten channel answers, AND IT IS THIS FAKE
// ASSERTING A4.
//
// THIS BOOK SAYS NOTHING ABOUT AN EMPTY CHANNEL ANYWHERE. The 590 pair's
// document prints "If the selected channel is empty, P4 ~ P15 will be 0 and
// P16 will be blank." (590:1492-1493); this one prints no counterpart, so
// BOTH halves of what happens here are assumed — that an MR of an unwritten
// channel answers at all rather than refusing, and that the answer's shape is
// the sibling document's. That is the design's A4, whose lift (L-HW-3) is the
// TS-480 row's RELEASE GATE, and NO RADIO HAS CONFIRMED EITHER HALF.
//
// The zeros are internally consistent with every hard-wired byte this row
// prints — P2, P10, P11, P12, P13 and P15 are all '0' runs already
// (480:910-939) — so the zero record is the one shape that requires no
// printed constant to be violated. The mode nibble is '0', which this book
// calls "No mode (Not used for the TS-480)" (480:843) rather than an
// empty-channel marker: the 590 pair's A18a reading is NOT available here.
func emptyRecord() MemState {
	return MemState{
		Freq:     "00000000000",
		Mode:     '0',
		Lockout:  '0',
		ToneMode: '0',
		ToneNo:   "00",
		CTCSSNo:  "00",
		Step:     "00",
		Name:     blankName,
	}
}

// DefaultImage is the image New uses when no WithFactoryImage option is
// given: three ordinary memory channels, and NOTHING ELSE.
//
// MINIMAL BY DESIGN, AND CONSTRAINED AT BOTH ENDS:
//
//   - AT LEAST ONE MEMORY CHANNEL IS POPULATED, so the fleet's
//     read-every-registered-model pins are non-vacuous against this row.
//   - EVERY PRINTED VALUE OF EVERY LIVE LEGEND APPEARS — the lockout's two
//     (480:920) and the tone mode's three (480:922) — so no driver behaviour
//     on one of them is a fixture accident.
//   - NOTHING ELSE IS POPULATED. Nothing about an unwritten channel's
//     contents is printed in this document at all, and an image of a hundred
//     channels would make every "this channel is empty" assertion a fixture
//     accident — on the row where that assertion IS the release gate (A4).
//
// ONLY THE P1=0 HALF IS SHIPPED. Decision 15 gives this row one flat MEM
// bank, 00-99, and no scan bank, so the P1=1 half of a channel is not a
// published slot and P19 ships no image for a slot the row does not publish.
// The book's own start/end overload on 90-99 (480:943-944) is still SERVED —
// the frame is a documented read — and answers the zero record.
func DefaultImage() map[recordKey]MemState {
	img := map[recordKey]MemState{}

	// Channel 00 — the plain one: FM, every legend at its printed first
	// value.
	img[recordKey{channel: 0, half: HalfRXOrStart}] = defaultRecord()

	// Channel 01 — the SECOND printed value of the lockout and of the tone
	// mode, and the other step index, so that no record in this image is a
	// repeat of another on the axes that matter.
	second := defaultRecord()
	second.Lockout = '1'  // "1: Lockout ON" (480:920)
	second.ToneMode = '1' // "1: TONE" (480:922)
	second.Step = stepAlsoInBoth
	img[recordKey{channel: 1, half: HalfRXOrStart}] = second

	// Channel 02 — the THIRD tone mode, on a second mode nibble. P7's legend
	// stops at 2 on this radio (480:922), so these three records carry the
	// whole of it.
	third := defaultRecord()
	third.Mode = modeUSB
	third.ToneMode = '2' // "2: CTCSS" (480:922)
	img[recordKey{channel: 2, half: HalfRXOrStart}] = third

	return img
}
