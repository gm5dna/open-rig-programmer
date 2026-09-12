// SPDX-License-Identifier: GPL-3.0-or-later

package ts480

import "github.com/gm5dna/open-rig-programmer/core/kw"

// THE ROW VALUE, AND THE FOUR BYTES THIS BOOK SPENDS DIFFERENTLY.
//
// The 2003 TS-480 document prints the SAME 50-byte memory grid the 590 book
// prints — same order, same widths (480:923-943 for MR, 480:955-976 for MW)
// — and then gives several of those bytes entirely different jobs. Each is
// an axis of kw.Layout rather than a branch in a parser, and each is cited
// below to the line of THIS book that prints it:
//
//   - BYTE 4 (P2) is a printed constant here, "Always 0 for the TS-480."
//     (480:953), where the 590 pair carry the channel's hundreds digit. That
//     is why this row's slot space stops at 99: the channel number is P3's
//     two digits alone, "00 ~ 99" (480:955), and there is no bank field in
//     the record at all — MC's own P1 reads "0: Always 0 for the TS-480
//     (Memory bank number)." (480:827).
//   - BYTE 19 (P6) is the CHANNEL LOCKOUT, "Lockout status. 0: Lockout OFF,
//     1: Lockout ON." (480:962), where the 590 pair carry the data mode
//     (590:1546-1548). The two radios then swap: the 590 pair carry their
//     lockout at byte 41 and this radio prints a constant there.
//   - BYTES 39-40 (P14) are the tuning step, "Step size. Refer to the ST
//     command." (480:979), where the 590 pair carry an FM bandwidth flag.
//     ST's own legend is mode-conditional over two different ranges — 00 ~
//     04 for SSB/CW/FSK and 00 ~ 09 for AM/FM, with index 00 meaning 0.5 kHz
//     in the first and 5 kHz in the second (480:1494-1500) — which is why
//     A22 refuses every TS-480 channel write. That refusal is the DRIVER'S;
//     this axis is only what tells it which byte it is looking at.
//   - BYTE 28 (P11) and BYTE 41 (P15) are printed constants (480:973,
//     480:982) where the 590 pair carry the FILTER A/B selection and the
//     lockout, so this row's hard-wired set has three runs the 590 pair's
//     does not.
//
// SIXTEEN OF THE 47 PARAMETER BYTES ARE HARD-WIRED HERE, against the 590
// pair's thirteen, and every one of them is REQUIRED ON PARSE and not merely
// emitted on build (decision 7). On this row that strictness is a CHOICE
// rather than a deduction, and the entry that says so is A24: the book's own
// general note permits a Set to fill an inapplicable parameter with "any
// character except the ASCII control codes (00 to 1Fh) and the terminator
// (;)" (480:108-111), so a radio answering a hard-wired byte with something
// else would not necessarily be faulty. A24's lift is a dozen real reads of
// a real TS-480, and a single counter-example turns the rule from "required"
// into "accepted and normalised".
//
// THE VALUE IS MINTED ONCE, AT INITIALISATION, AND HANDED OUT BY VALUE — the
// shape core/kw/ts590/layout.go states in full and for the same reasons.

// layout480 is the TS-480 row, minted once. See Layout.
var layout480 = kw.MustNewLayout(kw.LayoutConfig{
	// The document this row speaks, which is what an "O;" quotes its cause
	// sentence from: "Receive data was sent but processing was not
	// completed." (480:143-144). The 590 book gives the same token a
	// different cause, which is erratum E13 and the reason a Book is a
	// semantic rather than a nicety.
	Book:  kw.Book480,
	Model: "TS-480",

	// RecordLen was a package constant before the Kenwood/Yaesu wave's
	// RecordLen lift; it is pinned here explicitly so this row's frames
	// stay byte-identical by construction rather than by an unstated
	// default surviving the lift.
	RecordLen: kw.RecordLen,

	P2:        kw.P2FixedZero,       // "Always 0 for the TS-480." (480:953)
	Byte19:    kw.Byte19Lockout,     // (480:962)
	Byte28:    kw.Byte28FixedZero,   // "Always 0 for the TS-480." (480:973)
	Byte3940:  kw.Byte3940StepIndex, // "Step size. Refer to the ST command." (480:979)
	Byte41:    kw.Byte41FixedZero,   // "Always 0 for the TS-480." (480:982)
	ToneModes: kw.ToneModesThree,    // "0: OFF, 1: TONE, 2: CTCSS" (480:964)
	// P10, P12 and P13 are the TS-2000 lift's three axes, pinned to their
	// constant readings here: "Always 000 for the TS-480." (480:971),
	// "Always 0 for the TS-480." (480:975), "Always 000000000 for the
	// TS-480." (480:977).
	P10: kw.P10FixedZero,
	P12: kw.P12FixedZero,
	P13: kw.P13FixedZero,

	// The EX chart's own printed domain, "000 ~ 060: Menu No." (480:401) —
	// the narrowest of the three registry rows, and 27 addresses below the
	// TS-590S's.
	MaxEXAddress: 60,

	ModeNames: modeNames(),

	// ONE FLAT BANK, "00 ~ 99: Memory channel number" (480:955).
	//
	// CHANNELS 90-99 ARE NOT A SCAN CLASS HERE, AND THAT IS DECISION 15
	// RATHER THAN AN OVERSIGHT. The book really does print the P1 overload —
	// "Memory channel 90 ~ 99: P1=0 (start frequency), P1=1 (end frequency)"
	// (480:943-944, 480:986-987) — but on this radio those ten are ORDINARY
	// memories that also answer a second frame, not a separate bank: there
	// is no bank field in the record (480:827), a slot string is unique
	// across a codeplug, and a Bank.Fields map is per bank rather than per
	// slot. So this row gets no scan bank, FieldTxFrequency is Unsupported
	// on the whole of it, and the P1=1 half of 90-99 is unreachable through
	// this programme. Declaring a SlotScan range here instead would make
	// kw.Slot.P1 emit '1' for those channels' upper half and put a frame on
	// the wire this design has ruled out.
	Slots: []kw.SlotRange{
		{Class: kw.SlotMemory, Lo: 0, Hi: 99},
	},

	// SIXTEEN bytes in six runs. The three both books print — P10 "Always
	// 000 for the TS-480." (480:971), P12 "Always 0 for the TS-480."
	// (480:975) and P13 "Always 000000000 for the TS-480." (480:977) — plus
	// this radio's own P2, P11 and P15.
	//
	// THE SET AND THE THREE AXES MUST AGREE, and kw.NewLayout cross-checks
	// them: byte 4, byte 28 and byte 41 each appear BOTH as an axis above
	// and as a possible member of this set, and the two would otherwise be
	// one edit from disagreeing with nothing failing.
	PrintedFixed: []kw.FixedField{
		{Pos: 4, Printed: "0"},          // P2  (480:953)
		{Pos: 25, Printed: "000"},       // P10 (480:971)
		{Pos: 28, Printed: "0"},         // P11 (480:973)
		{Pos: 29, Printed: "0"},         // P12 (480:975)
		{Pos: 30, Printed: "000000000"}, // P13 (480:977)
		{Pos: 41, Printed: "0"},         // P15 (480:982)
	},
})

// Layout returns the TS-480's reading of the shared 50-byte memory grid.
//
// IT IS Layout AND NOT Layout480, on EXItems' terms exactly: the package
// clause already says which radio this is, and this row has no sibling
// inside this package to distinguish it from.
//
// THE ROW IS BUILT AND NOT REGISTERED at this milestone's close — see the
// package doc comment and A4 — so nothing in internal/wiring reaches this
// value yet. It is complete, tested and absent from SupportedModels().
func Layout() kw.Layout { return layout480 }

// modeNames is the MD legend this row reads MR/MW's P5 against
// (480:843-854), in this programme's own spellings.
//
// MR/MW P5 CARRIES NO LEGEND OF ITS OWN: both charts say "Mode. Refer to the
// MD command." (480:917, 480:959), so the memory mode vocabulary IS MD's.
//
// THE PRINTED SPELLINGS ARE NOT THE PUBLISHED ONES, AND THAT IS ERRATUM E12.
// This book prints "CWR (CW Reverse)" for nibble 7 and "FSR (FSK Reverse)"
// for nibble 9 (480:852, 480:854) where the 590 book names the same two
// nibbles CW-R and FSK-R (590:1361, 590:1363). The names published here are
// the programme's own consistent ones, on the FT-891 precedent; the printed
// forms are recorded in doc.go so that nobody re-derives them as a
// correction.
//
// NIBBLES 0 AND 8 ARE ABSENT. This book prints them "0: No mode (Not used
// for the TS-480)" and "8: Tune (Not used for the TS-480)" (480:843,
// 480:853), so neither names a mode a channel can be in and kw.NewLayout
// refuses a legend naming either. The 590 pair's empty-channel rule for
// nibble 0 (A18a) is NOT available here: this book prints no empty-channel
// note anywhere, which is A4 and is the release gate for this row.
func modeNames() map[kw.Mode]string {
	return map[kw.Mode]string{
		kw.ModeLSB:  "LSB",
		kw.ModeUSB:  "USB",
		kw.ModeCW:   "CW",
		kw.ModeFM:   "FM",
		kw.ModeAM:   "AM",
		kw.ModeFSK:  "FSK",
		kw.ModeCWR:  "CW-R",
		kw.ModeFSKR: "FSK-R",
	}
}
