// SPDX-License-Identifier: GPL-3.0-or-later

package fakets890

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
// PER ROW, so this package's images and internal/fakets990's are two
// compositions and two entries' worth of risk — and this package's register
// entry THE DEFAULT IMAGE'S RECORD COMPOSITION; PROVENANCE.md carries the
// whole statement with its citations and the union of family-level
// assumptions the images ride on.
//
// No byte here is invented, derived from another model, or padded to make a
// test pass. TestDefaultImage_EveryByteIsAPrintedValue holds the permitted
// sources mechanically.

// The TWO frequencies this document prints, and there are no others. THE 890S
// IS THE POOREST ROW IN THE FLEET FOR THIS: the front matter's FA worked
// example gives 7 MHz (890:86, quoted again at 890:118 and 890:125) and the
// AS0 block gives 14.175 MHz (890:342). Any other frequency in an image would
// be an invented byte.
const (
	printedFA  = "00007000000" // 890:86
	printedAS0 = "00014175000" // 890:342
)

// zeroFreq is the eleven-digit all-zero spelling, and it is NOT a third
// printed frequency: it is the printed STATE of a single memory channel's
// split-transmission parameters — "When reading a single memory channel, all
// parameters for Split Transmission become 0." (890:3217-3218).
const zeroFreq = "00000000000"

// The mode nibbles used below, from the OM P2 legend the MA0 chart refers to
// (890:3976-3992).
const (
	modeUSB = '2' // "2: USB" (890:3979)
	modeCW  = '3' // "3: CW" (890:3980)
	modeFM  = '4' // "4: FM" (890:3981)
)

// residualNameSlot is the channel the A21 fixture occupies: a BLANK record
// carrying a residual name in P13. See DefaultImage.
const residualNameSlot = 4

// BlankRecord is what a blank channel answers: "When reading a blank channel,
// parameters P2 to P12 becomes blank." (890:3215-3216).
//
// TWO ASSUMPTIONS MEET HERE AND THEY ARE SEPARATE.
//
//   - "Blank" means ASCII space 0x20. Neither MA0 block defines it; both books
//     define it for QR ("this setting is space", 890:4362-4363). That is the
//     design's A6.
//   - The note stops at P12 and says nothing about P13 (erratum E4, where the
//     990S's equivalent covers P2 to P18), so the NAME WINDOW'S state is
//     unprinted. This fake takes A4's reading — the window is blank too, so
//     the frame is 40 bytes and carries no name at all (the design's A17
//     counts that length).
//
// doc.go's register entry A BLANK CHANNEL ANSWERS THE 40-BYTE FRAME carries
// both, and PROVENANCE.md names A4 and A21 by number.
//
// It is EXPORTED because a caller staging a blank channel — with or without a
// residual name — must be able to build one without retyping eleven space runs
// and getting one width wrong.
func BlankRecord() MemState {
	blank := func(n int) string { return strings.Repeat(" ", n) }
	return MemState{
		Freq:          blank(recFreqLen),
		Mode:          ' ',
		FMNarrow:      ' ',
		ToneType:      ' ',
		ToneNo:        blank(recIndexLen),
		CTCSSNo:       blank(recIndexLen),
		SplitFreq:     blank(recFreqLen),
		SplitMode:     ' ',
		SplitFMNarrow: ' ',
		Split:         ' ',
		Lockout:       ' ',
		Name:          "",
	}
}

// simplexRecord is one populated simplex channel: a printed frequency, a
// printed mode nibble, and the printed all-zero split side a single memory
// channel carries (890:3217-3218). Every other field is the caller's to set.
func simplexRecord(freq string, mode byte) MemState {
	return MemState{
		Freq:          freq,
		Mode:          mode,
		FMNarrow:      '0', // "0: Normal" (890:3177)
		ToneType:      '0', // "0: OFF" (890:3181)
		ToneNo:        "00",
		CTCSSNo:       "00",
		SplitFreq:     zeroFreq,
		SplitMode:     '0',
		SplitFMNarrow: '0',
		Split:         '0', // "0: Simplex" (890:3202)
		Lockout:       '0', // "0: Lockout OFF" (890:3206)
	}
}

// DefaultImage is the image New uses when no WithFactoryImage option is given:
// four populated channels and one blank one carrying a residual name, and
// NOTHING ELSE.
//
// MINIMAL BY DESIGN, AND CONSTRAINED AT BOTH ENDS:
//
//   - AT LEAST ONE ORDINARY MEMORY CHANNEL IS POPULATED, so the fleet's
//     read-every-registered-model pins are non-vacuous against this row.
//   - EVERY LIVE TWO-VALUE BYTE APPEARS WITH BOTH ITS PRINTED VALUES — P4
//     (890:3177-3178), P11 (890:3202-3203) and P12 (890:3206-3207) — and P5's
//     FOUR tone types all appear, so no driver behaviour on one of them is a
//     fixture accident. Nothing in the grid conditions the tone type on the
//     mode, so the pairing below is free.
//   - ONE CHANNEL IS SPLIT, because the split side is half the grid (P8 to
//     P10) and a wholly simplex image would leave it answered by zeroes
//     everywhere.
//   - BOTH BLANK SHAPES ARE REACHABLE. The 40-byte no-name-window frame is
//     what EVERY unpopulated channel answers, through the one path
//     handleMA0Read takes; the second, A21's, is the record at
//     residualNameSlot below.
//   - NOTHING ELSE IS POPULATED. Nothing about an unwritten channel's contents
//     is printed beyond the blank-channel note itself, and an image of a
//     hundred channels would make every "this channel is blank" assertion a
//     fixture accident instead of a property.
//
// NO IMAGE IS SHIPPED FOR 100-119. Those numbers are printed (890:3167-3169)
// and the two classes they name are published in no bank of this registry row,
// so there is no slot for an image to represent — parser.go's slot-space
// comment and doc.go's register entry SLOTS 100-119 ARE NOT SERVED.
func DefaultImage() map[int]MemState {
	img := map[int]MemState{}

	// Channel 000 — the plain one: FM at the front matter's frequency, every
	// legend at its printed first value, and NO NAME, which makes its answer
	// the shortest a populated record can give. It is deliberately the same
	// LENGTH as a blank channel's and nothing like it in content, which is
	// what stops "40 bytes" from being read as "blank".
	img[0] = simplexRecord(printedFA, modeFM)

	// Channel 001 — the SECOND printed value of every live two-value byte, so
	// that no record in this image repeats another on the axes that matter,
	// plus a full ten-character name from the printed character set
	// (890:2891-2900). Still FM, so P4's "1: Narrow" sits on the mode whose
	// legend prints it.
	second := simplexRecord(printedAS0, modeFM)
	second.FMNarrow = '1' // "1: Narrow" (890:3178)
	second.ToneType = '1' // "1: Tone" (890:3182)
	second.ToneNo = "12"  // TN chart index 12 = 100.0 Hz (890:5149-5163), a
	// NON-ZERO index: index 00 is also printed and would leave P6 answering
	// the same bytes as a dropped index or a P6/P7 offset error would give.
	second.Lockout = '1' // "1: Lockout ON" (890:3207)
	second.Name = "MEMORY 001"
	img[1] = second

	// Channel 002 — the SPLIT one, "1: Split" (890:3203), with the split side
	// carrying the other printed frequency and its own mode. P4 and P10 AGREE,
	// which is what the book instructs a host to do on a split channel
	// (890:3219-3221); the fake does not enforce it, but its own image has no
	// business modelling the state the driver refuses.
	split := simplexRecord(printedAS0, modeUSB)
	split.ToneType = '2' // "2: CTCSS" (890:3183)
	split.CTCSSNo = "08" // CN chart index 08 = 88.5 Hz (890:1354-1369), a
	// NON-ZERO index for the same reason second.ToneNo above is: TN and CN
	// print identical frequencies at 00-49, so a P6/P7 offset error or a
	// dropped index would still pass a "00" fixture.
	split.SplitFreq = printedFA
	split.SplitMode = modeUSB
	split.Split = '1'
	split.Name = "SPLIT"
	img[2] = split

	// Channel 003 — the fourth tone type, "3: Cross Tone" (890:3185), on a
	// third mode.
	cross := simplexRecord(printedFA, modeCW)
	cross.ToneType = '3'
	cross.Name = "CW"
	img[3] = cross

	// Channel 004 — THE A21 FIXTURE: a blank record carrying a residual name
	// in P13. A4 assumes the name window of a blank channel is blank; A21 is
	// the SEPARATE assumption that a residue, if one exists, is not channel
	// content, and it is the one the design acts on. The empty predicate must
	// be shown IGNORING this record rather than erroring on it, because
	// core/clone's ReadAll abandons the whole read on the first channel error
	// — so one residual byte in one slot would otherwise make this radio
	// unreadable end to end.
	residue := BlankRecord()
	residue.Name = "OLD"
	img[residualNameSlot] = residue

	return img
}
