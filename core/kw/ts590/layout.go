// SPDX-License-Identifier: GPL-3.0-or-later

package ts590

import "github.com/gm5dna/open-rig-programmer/core/kw"

// THE TWO ROW VALUES, AND WHY THERE ARE TWO OF THEM.
//
// The TS-590S and the TS-590SG share one PC-command document, one 50-byte
// memory grid, one MD legend and one hard-wired byte set, so seven of
// kw.Layout's ten axes are the BOOK'S and are written once below
// (rowConfig). Three are the ROW'S, and each rests on a line the book
// scopes to one radio:
//
//   - BYTE 28 (P11), the FILTER A/B selection. One legend for both rows,
//     "0: FILTER A / 1: FILTER B" (590:1560-1563), and then "* In firmware
//     version 1.xx of TS-590S, always \"0\"." (590:1478 — MR's wording; MW
//     prints the same condition in different words at 590:1564, which is
//     erratum E7). So the SG's byte is live and the S's is a field whose
//     printed value must be ACCEPTED EITHER WAY on a read, because the
//     sentence is scoped to firmware 1.xx and a later firmware answers '1'.
//     That is A14, and what a WRITE may carry is the driver's question, not
//     this codec's.
//   - THE PRINTED EX MENU DOMAIN. The EX chart prints one line per row:
//     "000 ~ 087: Menu number (TS-590S)" (590:543) and "000 ~ 099: Menu
//     number (TS-590SG)" (590:544). Twelve addresses separate them, and
//     the two inventories they bound (EXItemsS, EXItemsSG) are one
//     identifier apart in this package.
//   - THE SLOT CEILING. "Channel numbers P00 ~ P09 are represented by 100 ~
//     109" is printed for both rows; "TS-590SG extension channel numbers E00
//     ~ P09 are represented by 110 ~ 119" (590:1346-1347, the printed typo
//     being erratum E2) is printed for one. The book never states the S's own
//     ceiling, which is A12, so the S stops at 109 and the SG runs to 119.
//
// EVERY OTHER AXIS AGREES, and core/kw/ts590/layout_test.go pins the
// agreement as well as the disagreement: a sibling pair sharing a grid is
// exactly where a value borrowed from the other row would never show, so the
// three-item disagreement list has to be exhaustive rather than approximate.
//
// THE VALUES ARE MINTED ONCE, AT INITIALISATION, AND HANDED OUT BY VALUE.
// kw.Layout's fields are unexported and its accessors copy, so a caller
// holding what these functions return cannot edit the radio; MustNewLayout
// is what turns a config kw.NewLayout would refuse into a panic at start-up
// rather than a zero Layout failing closed at every later site, which would
// read as "this radio refuses everything".

// layoutS is the TS-590S row, minted once. See LayoutS.
var layoutS = kw.MustNewLayout(rowConfig("TS-590S",
	// A14: the "always 0" sentence is scoped to THIS row's firmware 1.xx
	// (590:1478; E7 records MW's differing wording at 590:1564), and a
	// TS-590S at 2.00 or later answers '1'. The codec accepts either; the
	// write refusal is the driver's.
	kw.Byte28FilterEither,
	// "000 ~ 087: Menu number (TS-590S)" (590:543) — twelve addresses fewer
	// than the sibling row printed on the line below it.
	87,
	// A12: the book gives 110-119 to the SG and never states this row's
	// ceiling (590:1345-1347), so this row stops at the section-defined
	// channels.
	[]kw.SlotRange{
		{Class: kw.SlotMemory, Lo: 0, Hi: 99},
		{Class: kw.SlotScan, Lo: 100, Hi: 109},
	},
))

// layoutSG is the TS-590SG row, minted once. See LayoutSG.
var layoutSG = kw.MustNewLayout(rowConfig("TS-590SG",
	// P11 is live on this row: both printed values are meaningful
	// (590:1560-1563), and the firmware condition at 590:1478 names the
	// TS-590S alone.
	kw.Byte28FilterLive,
	// "000 ~ 099: Menu number (TS-590SG)" (590:544).
	99,
	// 000-099 ordinary memory (590:1341), 100-109 the section-defined
	// channels P00 ~ P09 (590:1345), and 110-119 the extension channels
	// E00 ~ E09 (590:1346-1347, printed "E00 ~ P09" — erratum E2).
	//
	// THE CODEC'S SLOT DOMAIN AND THE DRIVER'S PUBLISHED BANKS ARE TWO
	// DIFFERENT QUESTIONS, AND THIS IS THE DOMAIN. The book prints
	// 110-119 for this row — DOCUMENTED FACT (590:1346-1347), not A12,
	// which is the OTHER row's UN-stated ceiling, not this row's stated
	// one — so a front-panel recall of E00 produces "MC115;" and an
	// "MR0115;" answer that this codec must parse rather than refuse, and
	// the range is declared here. What an extension channel IS is never
	// explained anywhere in the book (A11), so Stuart ruled on 05/09/2026
	// (decision row 6) that the ten slots are OMITTED from the DRIVER'S
	// published Banks until A11 lifts. Both halves hold at once: the codec
	// admits what the book prints, and the driver publishes what is
	// confirmed.
	[]kw.SlotRange{
		{Class: kw.SlotMemory, Lo: 0, Hi: 99},
		{Class: kw.SlotScan, Lo: 100, Hi: 109},
		{Class: kw.SlotExtension, Lo: 110, Hi: 119},
	},
))

// LayoutS returns the TS-590S row's reading of the shared memory grid.
//
// THE NAME IS PART OF THIS PACKAGE'S CONTRACT, on EXItemsS's terms: a driver
// selects a row by calling the row's own function, never by passing a string
// to one function that branches. A branch would put the two rows one typo
// apart, which is the cross-model borrowing the Tier 4b sweep exists to
// forbid.
func LayoutS() kw.Layout { return layoutS }

// LayoutSG returns the TS-590SG row's reading of the shared memory grid.
//
// It differs from LayoutS on exactly three axes — byte 28's policy (A14),
// the slot ceiling (A12) and the printed EX menu domain (590:543 against
// 590:544) — and on nothing else; layout_test.go pins both lists.
func LayoutSG() kw.Layout { return layoutSG }

// rowConfig is the seven axes THE BOOK fixes for both rows, plus the three
// the caller supplies.
//
// IT IS A FUNCTION RATHER THAN A SHARED VARIABLE so that neither row can
// reach the other's config: a package-level kw.LayoutConfig copied and
// amended would share the ModeNames map and the PrintedFixed slice between
// the two values, and one mutation would then edit both radios. Every call
// builds fresh containers.
func rowConfig(model string, byte28 kw.Byte28Policy, maxEXAddress uint8, slots []kw.SlotRange) kw.LayoutConfig {
	return kw.LayoutConfig{
		// The document both rows speak, which is what an "O;" quotes its
		// cause sentence from (590:113; erratum E13 records that the TS-480
		// book gives the same token a different cause).
		Book:  kw.Book590,
		Model: model,

		// P2 is the channel number's hundreds digit: MR's and MW's charts
		// say only "Channel number (refer to the MC command)" (590:1453,
		// 590:1539-1540), and MC's own chart prints the space convention —
		// "enter 0 or a space for a channel number less than 100. For a
		// response command, a space is entered for a channel number less
		// than 100." (590:1332-1337). That MR and MW inherit MC's
		// convention at all is A10.
		P2: kw.P2HundredsDigit,
		// P6 is the data mode, "refer to the DA command" (590:1546-1548).
		// The TS-480 spends this byte on its channel lockout instead
		// (480:962), which is the whole of this axis.
		Byte19: kw.Byte19DataMode,
		Byte28: byte28,
		// P14 is the FM bandwidth flag: "00: FM Normal / 01: FM Narrow"
		// (590:1569-1571), and nothing else is printed. Whether the byte
		// means anything outside FM is unprinted, which is A23 and is the
		// driver's refusal rather than this axis's.
		Byte3940: kw.Byte3940FMNarrowFlag,
		// P15 is the channel lockout, "0: Channel Lockout OFF / 1: Channel
		// Lockout ON" (590:1572-1574) — the byte the TS-480 prints as a
		// constant (480:982).
		Byte41: kw.Byte41Lockout,
		// P7 prints FOUR values on this book, the fourth being "3: Cross
		// Tone ON" (590:1549-1553); the TS-480's legend stops at 2
		// (480:964).
		ToneModes: kw.ToneModesFour,

		// The EX chart prints a DIFFERENT menu domain for each of the two
		// rows — "000 ~ 087: Menu number (TS-590S)" (590:543) and "000 ~
		// 099: Menu number (TS-590SG)" (590:544) — on the one chart, two
		// lines apart. That is the third axis these rows differ on, and the
		// one an EX sweep that read the sibling's inventory would cross.
		MaxEXAddress: maxEXAddress,

		ModeNames: modeNames(),
		Slots:     slots,
		// P10 "000: Always 000" (590:1558-1559), P12 "0: Always 0"
		// (590:1565-1566) and P13 "000000000: Always 000000000"
		// (590:1567-1568). THIRTEEN of the 47 parameter bytes, and the whole
		// of this book's hard-wiring: bytes 4, 28 and 41 all carry meanings
		// here, and a layout claiming a constant at one of them would make
		// this codec refuse a legitimate answer.
		PrintedFixed: []kw.FixedField{
			{Pos: 25, Printed: "000"},
			{Pos: 29, Printed: "0"},
			{Pos: 30, Printed: "000000000"},
		},
	}
}

// modeNames is the MD legend both rows read MR/MW's P5 against
// (590:1353-1363), in this programme's own spellings.
//
// MR/MW P5 CARRIES NO LEGEND OF ITS OWN: both charts say "Mode (depending on
// the P1 setting, refer to the MD command)" (590:1544-1545), so the memory
// mode vocabulary IS MD's.
//
// NIBBLES 0 AND 8 ARE ABSENT, and their absence is the point rather than an
// omission. The book prints both as "None (setting failure)" (590:1353,
// 590:1362), so neither names a mode a channel can be in and kw.NewLayout
// refuses a legend that names either. Nibble 0 is still a documented ANSWER
// value — the empty channel's, "If the selected channel is empty, P4 ~ P15
// will be 0 and P16 will be blank" (590:1492-1493), which is A18a — and the
// record parser tests that window BEFORE it reaches this legend.
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
