// SPDX-License-Identifier: GPL-3.0-or-later

package kw

// THE THREE TEST LAYOUTS, AND WHY THEY LIVE HERE RATHER THAN IN ts590 AND
// ts480.
//
// Task 8 mints the shipping layout values in the model packages, where each
// axis is cited to its own layout line and each package's doc.go carries its
// share of the errata. This file is the codec's OWN pair of witnesses: the
// per-radio difference pins in this package assert facts about TWO radios at
// once ("byte 19 means the data mode HERE and the lockout THERE"), and a
// test that could only see one layout could not state such a fact at all.
//
// They are transcribed from the same two charts Task 8 will read — the
// citations below are this file's own, re-derived, not copied from a plan —
// so a divergence between these and the shipping values is a real
// disagreement between two readings of one book rather than a copy drifting
// from its source. Task 8 pins its own values against its own citations; it
// does not import these.
//
// THE THIRD FIXTURE IS THE TS-590S, not a second copy of the SG. The two 590
// rows share the grid and differ on byte 28 alone (A14/A12), so a fixture
// pair that held only the SG and the 480 could not tell a per-BOOK axis from
// a per-ROW one.

// layout590SG is the TS-590SG row.
func layout590SG() Layout {
	return MustNewLayout(LayoutConfig{
		Book:  Book590,
		Model: "TS-590SG",
		// P2 is the channel's hundreds digit, "refer to the MC command"
		// (590:1539-1540), and MC prints the space convention at
		// 590:1332-1337.
		P2: P2HundredsDigit,
		// Byte 19 is P6, "Data mode ... refer to the DA command"
		// (590:1546-1548).
		Byte19: Byte19DataMode,
		// Byte 28 is P11, FILTER A/B, live on this row (590:1560-1563).
		Byte28: Byte28FilterLive,
		// Bytes 39-40 are P14, "00: FM Normal / 01: FM Narrow"
		// (590:1569-1571).
		Byte3940: Byte3940FMNarrowFlag,
		// Byte 41 is P15, "0: Channel Lockout OFF / 1: Channel Lockout ON"
		// (590:1572-1574).
		Byte41: Byte41Lockout,
		// P7 prints four values, 3 being "Cross Tone ON" (590:1549-1553).
		ToneModes: ToneModesFour,
		ModeNames: modeNames590(),
		// 000-099 ordinary memory and 100-109 the section-defined pairs on
		// both 590 rows; 110-119 the SG's extension channels
		// (590:1345-1347). What the DRIVER publishes as a bank is a
		// different question — Stuart ruled 110-119 OMITTED from the
		// published bank on 05/09/2026 — and the codec's slot domain is
		// what the book prints (A12).
		Slots: []SlotRange{
			{Class: SlotMemory, Lo: 0, Hi: 99},
			{Class: SlotScan, Lo: 100, Hi: 109},
			{Class: SlotExtension, Lo: 110, Hi: 119},
		},
		PrintedFixed: commonPrintedFixed(),
	})
}

// layout590S is the TS-590S row: the SG's grid with byte 28's policy
// changed, and the slot ceiling A12 leaves at 109.
func layout590S() Layout {
	return MustNewLayout(LayoutConfig{
		Book:     Book590,
		Model:    "TS-590S",
		P2:       P2HundredsDigit,
		Byte19:   Byte19DataMode,
		Byte28:   Byte28FilterEither, // "always \"0\"" in firmware 1.xx (590:1478; E7)
		Byte3940: Byte3940FMNarrowFlag,
		Byte41:   Byte41Lockout,

		ToneModes: ToneModesFour,
		ModeNames: modeNames590(),
		Slots: []SlotRange{
			{Class: SlotMemory, Lo: 0, Hi: 99},
			{Class: SlotScan, Lo: 100, Hi: 109},
		},
		PrintedFixed: commonPrintedFixed(),
	})
}

// layout480 is the TS-480 row.
func layout480() Layout {
	return MustNewLayout(LayoutConfig{
		Book:  Book480,
		Model: "TS-480",
		// P2 is "Always 0 for the TS-480." (480:953) — there is no bank
		// field in this record.
		P2: P2FixedZero,
		// Byte 19 is P6, "Lockout status. 0: Lockout OFF, 1: Lockout ON."
		// (480:962) — the byte the 590 pair spends on the data mode.
		Byte19: Byte19Lockout,
		// Byte 28 is P11, "Always 0 for the TS-480." (480:973).
		Byte28: Byte28FixedZero,
		// Bytes 39-40 are P14, "Step size. Refer to the ST command."
		// (480:979).
		Byte3940: Byte3940StepIndex,
		// Byte 41 is P15, "Always 0 for the TS-480." (480:982).
		Byte41: Byte41FixedZero,
		// P7 prints three values and no cross tone (480:964).
		ToneModes: ToneModesThree,
		ModeNames: modeNames480(),
		// "00 ~ 99: Memory channel number" (480:955), one flat bank
		// (480:830-838). Channels 90-99 also answer a second frame
		// (480:943-944) but they are ordinary memories in this record, so
		// they are not a scan CLASS here.
		Slots: []SlotRange{
			{Class: SlotMemory, Lo: 0, Hi: 99},
		},
		PrintedFixed: append(commonPrintedFixed(),
			// The 480's three extra hard-wirings, each printed "Always 0
			// for the TS-480".
			FixedField{Pos: 4, Printed: "0"},  // P2  (480:953)
			FixedField{Pos: 28, Printed: "0"}, // P11 (480:973)
			FixedField{Pos: 41, Printed: "0"}, // P15 (480:982)
		),
	})
}

// commonPrintedFixed is the hard-wiring both books print identically: P10
// "000: Always 000" (590:1558-1559, 480:971), P12 "0: Always 0"
// (590:1565-1566, 480:975) and P13 "000000000: Always 000000000"
// (590:1567-1568, 480:977). Thirteen bytes, and they are the whole of the
// TS-590SG's hard-wired set.
func commonPrintedFixed() []FixedField {
	return []FixedField{
		{Pos: 25, Printed: "000"},
		{Pos: 29, Printed: "0"},
		{Pos: 30, Printed: "000000000"},
	}
}

// modeNames590 is the 590 pair's MD legend (590:1353-1363), in this
// project's own spellings. Nibbles 0 and 8 are "None (setting failure)" on
// this book and name no mode, so neither is here.
func modeNames590() map[Mode]string {
	return map[Mode]string{
		ModeLSB: "LSB", ModeUSB: "USB", ModeCW: "CW", ModeFM: "FM",
		ModeAM: "AM", ModeFSK: "FSK", ModeCWR: "CW-R", ModeFSKR: "FSK-R",
	}
}

// modeNames480 is the TS-480's MD legend (480:843-854) in the same
// spellings. The book prints "CWR (CW Reverse)" and "FSR (FSK Reverse)";
// erratum E12 records that, and the names published here are the
// programme's own consistent ones.
func modeNames480() map[Mode]string {
	return map[Mode]string{
		ModeLSB: "LSB", ModeUSB: "USB", ModeCW: "CW", ModeFM: "FM",
		ModeAM: "AM", ModeFSK: "FSK", ModeCWR: "CW-R", ModeFSKR: "FSK-R",
	}
}
