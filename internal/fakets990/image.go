// SPDX-License-Identifier: GPL-3.0-or-later

package fakets990

import "strings"

// Image is a factory image: a function returning a freshly populated set of
// memory records, keyed by channel number. EACH CALL MUST RETURN AN
// INDEPENDENT MAP, so that multiple *Radio instances — or repeated New() calls
// with one Image value — never share mutable state. Every Image in this
// package is a plain function building its map from scratch, which is what
// makes that hold; a caller supplying its own (WithFactoryImage) owes the same
// property. TestDefaultImage_EachCallIsIndependent pins it for DefaultImage,
// and TestTwoRadiosFromOneImageDoNotAlias pins what a shared map would
// actually break.
type Image func() map[int]MemState

// The compile-time proof that this package's own image satisfies the contract
// callers are typed against.
var _ Image = DefaultImage

// THE EVIDENCE POSTURE OF EVERY BYTE BELOW, and the claim it supports.
//
// Every byte value in a record this file builds is a constant, an example, a
// legend value or a character set PRINTED IN THIS RADIO'S OWN PC-command
// document. The RECORD is a synthetic COMPOSITION of those values: no MA0
// frame is printed as a literal anywhere in the book, so no image's cross-field
// combination has ever been printed or observed. These are not observed
// contents and not factory defaults. That composition is the design's A22 —
// PER ROW, so this package's images and internal/fakets890's are two
// compositions and two entries' worth of risk — and this package's register
// entry THE DEFAULT IMAGE'S RECORD COMPOSITION; PROVENANCE.md carries the
// whole statement with its citations and the union of family-level
// assumptions the images ride on.
//
// No byte here is invented, derived from another model, or padded to make a
// test pass. TestDefaultImage_EveryByteIsAPrintedValue holds the permitted
// sources mechanically.

// The THREE frequencies this document prints, and there are no others. The
// front matter's FA worked example gives 7 MHz (990:86-89, quoted again at
// 990:119 and 990:126), the AS2 block gives 14.175 MHz ("for example, 14.175
// MHz is displayed as 00014175000", 990:344-345) and FA's own parameter note
// gives 14.195 MHz ("For example, enter 00014195000 for 14.195 MHz",
// 990:2352, and FB's at 990:2374). THE 890S PRINTS ONLY TWO; any other
// frequency in an image would be an invented byte on either row.
const (
	printedFA   = "00007000000" // 990:86-89
	printedAS2  = "00014175000" // 990:344-345
	printedFA14 = "00014195000" // 990:2352
)

// zeroFreq is the eleven-digit all-zero spelling, and it is NOT a fourth
// printed frequency: it is the printed STATE of a single memory channel's
// frequency-2 parameters — "When reading a single memory channel, all
// parameters for frequency 2 become 0." (990:2964-2965), the design's A16.
//
// It is also what classFor reads to decide the P2 a record answers with.
const zeroFreq = "00000000000"

// The mode nibbles used below, from the OM P2 legend the MA0 chart refers to
// (990:3707-3730).
const (
	modeUnused = '0' // "0: Unused" (990:3707)
	modeUSB    = '2' // "2: USB" (990:3709)
	modeCW     = '3' // "3: CW" (990:3710)
	modeFM     = '4' // "4: FM" (990:3711)
)

// BlankRecord is what a blank channel answers: "When reading a blank channel,
// parameters P2 to P18 becomes blank." (990:2962-2963).
//
// ONE ASSUMPTION ONLY, AND IT IS NAMED. "Blank" means ASCII space 0x20 — the
// design's A6, on this book's own QR definition, "this setting is blank
// <0x20>" (990:4081-4082), which is more explicit than the 890S's "this
// setting is space" (890:4362-4363).
//
// NOTHING ELSE IS ASSUMED HERE, and that is the whole difference from
// internal/fakets890's blank record: this note covers P2 to P18, the name
// window included, so the frame it describes is completely printed — MA0, the
// slot, fifty spaces and the terminator. The 890S's note stops at P12
// (erratum E4), which leaves that radio's name window and even its frame
// LENGTH unprinted (its A4 and A17) and is why that fake ships a second blank
// fixture carrying a residual name under A21. There is no counterpart here and
// none may be invented.
//
// It is EXPORTED because a caller staging a blank channel must be able to
// build one without retyping eleven space runs and getting one width wrong.
func BlankRecord() MemState {
	blank := func(n int) string { return strings.Repeat(" ", n) }
	return MemState{
		Class:     ' ',
		Freq:      blank(recFreqLen),
		Mode:      ' ',
		FMNarrow:  ' ',
		ToneType:  ' ',
		ToneNo:    blank(recIndexLen),
		CTCSSNo:   blank(recIndexLen),
		Freq2:     blank(recFreqLen),
		Mode2:     ' ',
		FMNarrow2: ' ',
		ToneType2: ' ',
		ToneNo2:   blank(recIndexLen),
		CTCSSNo2:  blank(recIndexLen),
		Split:     ' ',
		DualRX:    ' ',
		Lockout:   ' ',
		Name:      blank(maxNameLen),
	}
}

// singleRecord is one populated Single Memory channel: a printed frequency, a
// printed mode nibble, and the printed all-zero frequency-2 side such a
// channel carries (990:2964-2965). Every other field is the caller's to set.
func singleRecord(freq string, mode byte) MemState {
	return MemState{
		Class:     classSingle, // "0: Single Memory channel" (990:2898)
		Freq:      freq,
		Mode:      mode,
		FMNarrow:  '0', // "0: FM Wide for frequency 1" (990:2913)
		ToneType:  '0', // "0: FM Tone function OFF for frequency 1" (990:2916)
		ToneNo:    "00",
		CTCSSNo:   "00",
		Freq2:     zeroFreq,
		Mode2:     modeUnused,
		FMNarrow2: '0',
		ToneType2: '0',
		ToneNo2:   "00",
		CTCSSNo2:  "00",
		Split:     '0',        // "0: Simplex" (990:2947)
		DualRX:    '0',        // "0: Dual reception OFF" (990:2950)
		Lockout:   lockoutOff, // "1: Scan Lockout OFF" (990:2953)
		Name:      strings.Repeat(" ", maxNameLen),
	}
}

// DefaultImage is the image New uses when no WithFactoryImage option is given:
// four populated channels and NOTHING ELSE.
//
// MINIMAL BY DESIGN, AND CONSTRAINED AT BOTH ENDS:
//
//   - AT LEAST ONE ORDINARY MEMORY CHANNEL IS POPULATED, so the fleet's
//     read-every-registered-model pins are non-vacuous against this row.
//   - EVERY LIVE TWO-VALUE BYTE APPEARS WITH BOTH ITS PRINTED VALUES — P5
//     (990:2913-2914), P11 (990:2933-2934), P15 (990:2947-2948), P16
//     (990:2950-2951) and P17's 1/2 (990:2953-2954) — and P6's FOUR tone
//     types all appear, so no driver behaviour on one of them is a fixture
//     accident. Nothing in the grid conditions the tone type on the mode, so
//     the pairing below is free.
//   - A PRINTED VALUE GOES ONLY WHERE ITS COMBINATION IS ONE THE FAMILY'S OWN
//     CODEC WILL BUILD, and P16 is the byte where those two rules meet.
//     "1: Dual reception ON" over a frequency-2 side that is entirely zero
//     is a second receiver with no frequency to receive on: every byte of it
//     is printed, the pairing is not, core/kw/ma refuses to build it and
//     core/driver/ts990 reports such an answer as one that cannot be written
//     back in any form. So P16's second value sits on the ONE channel with a
//     live frequency 2 (review s2-close-review-opus-1.md MED-2), which also
//     gives the image the P15 = 1 WITH P16 = 1 case — two flags the book
//     never defines against each other (990:2946-2951).
//     TestDefaultImage_NoRecordSetsP16OverAZeroedFrequency2 is the pin.
//   - EACH OF THE FOUR TWO-DIGIT INDEX WINDOWS CARRIES A DISTINCT NON-ZERO
//     PRINTED INDEX SOMEWHERE. P7, P8, P13 and P14 sit at positions 22-23,
//     24-25, 40-41 and 42-43, and the TN and CN charts print IDENTICAL
//     frequencies at 00-49 (990:4949-4974, 990:1240-1267) — so an image
//     leaving them all "00" reads the same whether a driver takes the right
//     window, a neighbouring one, or none at all.
//   - ONE CHANNEL HAS A LIVE FREQUENCY 2, because that side is half the grid
//     (P9 to P14) and an image without one would leave it answered by zeroes
//     everywhere — and because it is the only way P2's second printed value
//     appears at all (see classFor).
//   - THE THIRD CLASS IS DELIBERATELY ABSENT. "2: Section defined Memory
//     channel" is printed (990:2900), but what such a channel's P9 holds is
//     NOT: the section's end frequency is MA6's (990:3051-3059) and the
//     design's A8 records the assumption that P9 does not carry it. Composing
//     one would put an unevidenced combination in an image; WithChannel is the
//     seam for staging one when a driver's refusal needs it.
//   - NOTHING ELSE IS POPULATED. Nothing about an unwritten channel's contents
//     is printed beyond the blank-channel note itself, and an image of a
//     hundred channels would make every "this channel is blank" assertion a
//     fixture accident instead of a property.
//
// THERE IS NO SECOND BLANK FIXTURE, and internal/fakets890 has one: this
// book's blank-channel note covers P2 to P18 (990:2962-2963), so the blank
// frame is printed modulo A6 and every unpopulated channel reaches it through
// the one path handleMA0Read takes. The 890S's note stops at P12 (erratum E4),
// which is what leaves that radio's name window to A4 and its residue to A21.
//
// NO IMAGE IS SHIPPED FOR 100-119. Those numbers are printed (990:2894-2896)
// and the two classes they name are published in no bank of this registry row,
// so there is no slot for an image to represent — parser.go's slot-space
// comment and doc.go's register entry SLOTS 100-119 ARE NOT SERVED.
func DefaultImage() map[int]MemState {
	img := map[int]MemState{}

	// Channel 000 — the plain one: FM at the front matter's frequency, every
	// legend at its printed first value, and a blank name window.
	img[0] = singleRecord(printedFA, modeFM)

	// Channel 001 — the SECOND printed value of every live two-value byte
	// this side of the grid carries, so that no record in this image repeats
	// another on the axes that matter, plus a full ten-character name from
	// the printed character set (990:2770-2778). Still FM, so P5's "1: FM
	// Narrow" sits on the mode whose legend prints it. P16 is NOT among them:
	// this channel's frequency-2 side is the printed zeroed one, and dual
	// reception over it is the combination the constraint list rules out.
	second := singleRecord(printedAS2, modeFM)
	second.FMNarrow = '1'      // "1: FM Narrow for frequency 1" (990:2914)
	second.ToneType = '1'      // "1: Tone for frequency 1" (990:2917)
	second.ToneNo = "12"       // TN index 12 = 100.0 Hz (990:4972), a NON-ZERO
	second.Lockout = lockoutOn // "2: Scan Lockout ON" (990:2954)
	second.Name = "MEMORY 001"
	img[1] = second

	// Channel 002 — THE ONE WITH A LIVE FREQUENCY 2: split (990:2948), the
	// third printed frequency on the first side and the first on the second,
	// and its own mode, width, tone type and both index windows. Its P2
	// answers "1: Dual Memory channel" because the chart says the type is
	// decided by the P9/P10 values (990:2901-2903) — classFor, not a byte
	// composed here.
	//
	// IT ALSO CARRIES P16 = '1', and this is the only record in the image
	// that may: dual reception describes a SECOND RECEIVER, so the flag needs
	// a frequency underneath it (see the constraint list above). The pairing
	// with P15 = '1' is the case the book leaves undefined — it prints the
	// two flags side by side and never against each other (990:2946-2951) —
	// which is what core/driver/ts990's read path calls "P15 DECIDES ALONE".
	dual := singleRecord(printedFA14, modeUSB)
	dual.ToneType = '2' // "2: CTCSS for frequency 1" (990:2918)
	dual.CTCSSNo = "08" // CN index 08 = 88.5 Hz (990:1260), a NON-ZERO index
	dual.Freq2 = printedFA
	dual.Mode2 = modeFM
	dual.FMNarrow2 = '1' // "1: FM Narrow for frequency 2" (990:2934)
	dual.ToneType2 = '3' // "3: Cross Tone for frequency 2" (990:2939), which is
	dual.ToneNo2 = "21"  // why BOTH index windows on this side are live: TN 21
	dual.CTCSSNo2 = "05" // = 136.5 Hz (990:4968) and CN 05 = 79.7 Hz (990:1257)
	dual.Split = '1'     // "1: Split" (990:2948)
	dual.DualRX = '1'    // "1: Dual reception ON" (990:2951)
	dual.Class = classFor(dual.Freq2)
	dual.Name = "SPLIT     "
	img[2] = dual

	// Channel 003 — the fourth tone type, "3: Cross Tone for frequency 1"
	// (990:2919), on a third mode.
	cross := singleRecord(printedFA, modeCW)
	cross.ToneType = '3'
	cross.Name = "CW        "
	img[3] = cross

	return img
}
