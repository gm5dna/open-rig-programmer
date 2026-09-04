// SPDX-License-Identifier: GPL-3.0-or-later

package ft891

import "github.com/gm5dna/open-rig-programmer/core/cat"

// modeNames is the FT-891's P6 mode table, TRANSCRIBED FRESH from this
// radio's own manual rather than copied from the FT-710's or the FTdx10's.
//
// It has to be typed out again for the reason core/cat/ftdx10/dialect.go
// gives: core/cat's table is unexported, so there is nothing to reference
// even if referencing it were right — and it would not be. Two radios
// agreeing on a mode nibble is a fact about those two radios, not a shared
// definition. This radio is the proof of that: SIX of its twelve names
// disagree with the FTdx10's at the same nibble, and three nibbles the
// FTdx10 fills are empty here. dialect_test.go's mode pins compare the two
// dialects over all 256 wire bytes and hold both halves — the six shared
// names and the six differing ones — so a copy error in either direction
// fails there.
//
// SOURCE: the mode legend printed beside THREE commands in manual rev
// 1909-C — MR's P6 (ft891_layout.txt:972-974), MT's P6 (1007-1010) and
// MW's P6 (1043-1046). THE THREE ARE IDENTICAL: the same twelve names
// against the same twelve nibbles, in the same order, with the same hole.
// The only difference between the three printings is whitespace — MR's
// runs "1:LSB" with no space where MT's and MW's run "1: LSB" — which is
// typography, not vocabulary. The keys below are the legend's own wire
// bytes, written as byte literals rather than through core/cat's Mode
// constants, so that what is transcribed here is this manual's "6:
// RTTY-LSB" and not core/cat's spelling of it.
//
// THE HOLE AT 'A' IS TRANSCRIBED AS A HOLE. All three printings run
// "... 9: RTTY-USB A: - B: FM-N ...": the dash is the chart's way of
// printing "nothing here", so 'A' names no mode on this radio and is not a
// member. 'E' and 'F' are not printed at all. The FTdx10's legend fills all
// three (DATA-FM, PSK, DATA-FM-N), which is what
// TestDifferencePinModeMembership holds.
//
// The '0' entry is the exception, and it is deliberately spelt with the
// core/cat constant, because that is exactly what it is: ASSUMED,
// inherited, and named by the codec rather than by this manual. See doc.go's
// register, entry "the cat.ModeUnset member of the mode table".
var modeNames = map[cat.Mode]string{
	// ASSUMED — cat.ModeUnset ('0', "-") appears in NO FT-891 mode legend;
	// all three run 1..9 then B, C, D. It is here because parsers must
	// accept the placeholder: core/cat refuses to EMIT it in any Set frame,
	// so its presence widens what this dialect can read and nothing else.
	cat.ModeUnset: "-",

	cat.Mode('1'): "LSB",
	cat.Mode('2'): "USB",
	cat.Mode('3'): "CW",
	cat.Mode('4'): "FM",
	cat.Mode('5'): "AM",
	cat.Mode('6'): "RTTY-LSB",
	cat.Mode('7'): "CW-R",
	cat.Mode('8'): "DATA-LSB",
	cat.Mode('9'): "RTTY-USB",
	// 'A' is the legend's printed hole — "A: -" — and is deliberately absent.
	cat.Mode('B'): "FM-N",
	cat.Mode('C'): "DATA-USB",
	cat.Mode('D'): "AM-N",
	// 'E' and 'F' are not printed in any of the three legends.
}

// dialect is the FT-891, built once at init and validated by
// cat.MustNewDialect's fourteen rules. EVERY FIELD IS SET EXPLICITLY,
// including the two the combined MT form requires to be zero: a field left
// out of this literal would be indistinguishable from a field deliberately
// zeroed, and V9's "an inapplicable field must be explicitly zero" rule is
// only readable as a decision if the zero is written down.
//
// MustNewDialect rather than NewDialect because this is a compile-time
// constant table: a mistake in it is a build-time defect that must stop the
// programme loudly on first use, not an error threaded through model
// registration.
//
// FIVE OF ITS FIELDS TAKE THE MINORITY READING of an axis added for this
// radio — EXAddressForm, Slots.MCSelects, MT.ReadSlots, MT.P11 and
// MemoryP5 — and each is transcribed from the block that prints it, with
// the sibling that disagrees named. None of the five is an assumption; the
// assumptions this dialect does carry are marked ASSUMED below and
// registered by name in doc.go.
var dialect = cat.MustNewDialect(cat.DialectConfig{
	// The ID block prints "P1 0650: FT-891" (ft891_layout.txt:763).
	CATID:     "0650",
	ModeNames: modeNames, // fresh transcription, 1..9 and B..D per the
	// three legends; '0' (ModeUnset, "-") included as an ASSUMED member —
	// absent from every FT-891 legend; parsers must accept the placeholder.
	Slots: cat.SlotSpace{
		// "001 - 099 (Regular Memory Channel)", MR's P0/1 legend
		// (ft891_layout.txt:960), MT's P1 (998), MW's P1 (1035).
		MemoryLo: 1, MemoryHi: 99,
		// MANUAL, NOT ASSUMED, and this radio is the first Yaesu dialect
		// of which that can be said: MR's legend prints the 5 MHz bank's
		// ACTUAL NUMBERS — "501 - 510 (5 MHz, U.S. and U.K. version only)"
		// (ft891_layout.txt:962) — where the FT-710's and the FTdx10's say
		// only "5xx (5MHz BAND)" and their 501..599 is an inherited
		// interpretation on their own ASSUMED registers. So the bounds
		// here are transcribed, and doc.go's register deliberately does
		// NOT carry an entry for them. TestDifferencePinSixtyRange holds
		// the difference against the FTdx10.
		//
		// The region condition — "U.S. and U.K. version only" — is a fact
		// about which unit is in front of you, not about the dialect: a
		// dialect describes what the wire vocabulary IS, and the presence
		// of the bank on a given radio is a matrix and discovery question
		// (Stage 2 reads 501..510 by MR and treats "?;" as absent).
		SixtyLo: 501, SixtyHi: 510,
		// "P1L - P9U (PMS)" on all three memory blocks (961, 999, 1036).
		PMSPairs: 9,
		// "EMG (Emergency)", MR's legend (ft891_layout.txt:964). MT's and
		// MW's do not print it; that is what MT.ReadSlots carries.
		EmergencyWire: "EMG",
		// ASSUMED — "000" appears in NO FT-891 slot legend. It is the
		// FT-710's MR-answer fact, and cat.SlotSpace structurally requires
		// a none form, so one is supplied. See doc.go's register, entry
		// "SlotSpace.NoneWire".
		NoneWire: "000",
		// The FT-891's MC block prints TWO slot classes only — "001 - 099:
		// Regular Memory Channel" and "P1L - P9U (PMS)"
		// (ft891_layout.txt:907-909) — where every registered sibling's MC
		// legend prints 5xx and EMG as well. So an MC Set on this radio may
		// name memory and PMS and nothing else. Not an assumption: this is
		// the legend, transcribed. TestDifferencePinMCSelects holds it
		// against the FTdx10, which builds what this refuses.
		MCSelects: cat.MCSelectsMemoryPMS,
	},
	EXItems: exItems,
	// The FT-891's EX grammar block prints "E X P1 P1 P1 P1 ;" for the Read
	// and "P1 : 0101 - 1803 (MENU Number)" for the address
	// (ft891_layout.txt:513-522): FOUR digits, where every registered
	// sibling prints six. Note the naming trap cat.EXAddressPair's own doc
	// comment records — this manual spells the whole four-digit field a
	// single P1 and calls the parameter body P2 — the BYTES agree exactly,
	// only the component names differ.
	EXAddressForm: cat.EXAddressPair,
	MT: cat.MTPolicy{
		// The MT Set/Answer charts run to 41 positions — the 28 shared
		// memory positions, P11 at 28, a 12-byte P12 tag at 29-40 and ';'
		// at 41 (ft891_layout.txt:996-1027).
		Form: cat.MTFormCombined,
		// The FT-891's MT block prints its slot legend as "001 - 099
		// (Regular Memory Channel)" and "P1L - P9U (PMS)" ONLY
		// (ft891_layout.txt:998-999), where its own MR block prints the 5xx
		// and EMG banks too (960-964) and every registered sibling's MT
		// legend prints all four classes. Its Read chart is "M T P0 P0 P0 ;"
		// (1016) and P0 is defined NOWHERE in the block, so there is no
		// second legend to read the read's domain from: the block's one
		// slot legend is it. Not an assumption: this is the legend,
		// transcribed. TestDifferencePinMTReadSlots holds it against the
		// FTdx10, which builds MT reads this dialect refuses.
		ReadSlots: cat.MTReadsMemoryPMS,
		// "P12 TAG Characters (up to 12 characters) (ASCII)"
		// (ft891_layout.txt:1017), and the Set chart draws that field over
		// positions 29-40 — twelve.
		TagMaxBytes:  12,
		ClearTagByte: 0, // must be 0 under MTFormCombined (V9)
		PadByte:      0, // must be 0 under MTFormCombined (V9)
		// ASSUMED — the byte this radio pads a short tag with, in both
		// directions. The P12 legend names a width and an alphabet and no
		// fill; no FT-891 has ever been asked. See doc.go's register,
		// entry "MTPolicy.TagFill".
		TagFill: ' ',
		// The FT-891's MT block prints `P11 0: TAG "OFF" 1: TAG "ON"`
		// (ft891_layout.txt:1016), so byte 28 of its combined record is a
		// LIVE FLAG the caller supplies and the radio reports — where
		// every registered combined-form sibling prints it "0: (Fixed)".
		// Not an assumption: this is the legend, transcribed. Under this
		// policy the display-LESS pair (BuildMTSetCombined,
		// ParseMTAnswerCombined) refuses, because a live flag is never
		// defaulted; TestDifferencePinMTP11 holds both halves against the
		// FTdx10, where the refusals run the other way.
		P11: cat.P11TagDisplay,
	},
	Clarifier: cat.ClarifierPolicy{
		// BOTH FIELDS ARE ASSUMED, and they are ONE register entry with one
		// lifting capture (doc.go, entry "ClarifierPolicy.StepHz and
		// MaxAbsHz"). The manual prints "Clarifier Offset: 0000 - 9999
		// (Hz)" on every block that carries the field — MR 967, MT 1003,
		// MW 1040, IF 781, OI 1126 — and states NO step. 9999 is not a
		// multiple of the inherited 10, so this pair cannot be read off the
		// printed range: the ceiling here is 9990, the largest multiple of
		// the assumed step inside the printed range, which is a DEDUCTION
		// FROM AN ASSUMPTION and not a transcription.
		StepHz:   10,
		MaxAbsHz: 9990,
	},
	// The FT-891's memory blocks print "P5 0: (Fixed)" on every one of them
	// — MR 971, MT 1006, MW 1042, IF 783 and OI 1129 — where the three registered
	// dialects print `P5 0: TX CLAR "OFF" 1: TX CLAR "ON"`. So byte 21 is
	// schema on this radio and carries no TX clarifier state in either
	// direction. Not an assumption: this is the legend, transcribed.
	// TestDifferencePinMemoryP5 holds it against the FTdx10, which builds a
	// TxClar-true record this dialect refuses.
	MemoryP5: cat.P5Fixed,
	// The FT-891's MW legend prints "P7 0: (Fixed)" (ft891_layout.txt:1047),
	// and cat.CombinedMTSetKind is the byte '0' — so the constant on the
	// right is the correct SPELLING of what this radio documents. That the
	// two coincide is A FACT OF THIS RADIO, not a rule: MW's P7 and the
	// combined MT Set's P7 (also "0: (Fixed)", 1011) are different fields of
	// different commands that this manual happens to fix at the same byte.
	// See TestIdentityPinMWWriteKind, which says exactly this much and no
	// more.
	MWWriteKind: cat.CombinedMTSetKind,
})

// Dialect returns the FT-891's cat.Dialect.
//
// A function over an exported var so that the package-held value cannot be
// reassigned by a consumer: a Dialect is what the outbound write gate
// consults on every frame, and one a caller can swap after init is not a
// gate. cat.Dialect is a value type carrying only copied maps and slices,
// so the returned copy is inert in the other direction too — which is also
// what makes Dialect().EXItems() the package's single route to the
// generated inventory, and why exinventory.go declares nothing beside the
// generated variable.
func Dialect() cat.Dialect { return dialect }
