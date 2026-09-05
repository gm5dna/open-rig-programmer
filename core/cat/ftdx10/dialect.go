// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx10

import "github.com/gm5dna/open-rig-programmer/core/cat"

// modeNames is the FTdx10's P6 mode table, TRANSCRIBED FRESH from this
// radio's own manual rather than copied from the FT-710's.
//
// It has to be typed out again: core/cat's table is unexported, so there is
// nothing to reference even if referencing it were right — and it would not
// be. Two radios agreeing on a mode nibble is a fact about those two radios,
// not a shared definition, and the moment a third disagrees a shared table
// would have to be unpicked from every dialect that had quietly adopted it.
// The cost of retyping is a copy error, and dialect_test.go's mode identity
// pin is what catches one: it compares this table with cat.FT710's over all
// 256 wire bytes.
//
// SOURCE: the mode legend printed beside FOUR different commands in manual
// rev 2308-F, all four identical and all four running 1 to F with no other
// member — MD (layout 1146-1149), MR's P6 (1192-1194), MT's P6 (1227-1229)
// and MW's P6 (1267-1269). The keys below are the legend's own wire bytes,
// written as byte literals rather than through core/cat's Mode constants,
// so that what is transcribed here is the manual's "1: LSB" and not
// core/cat's spelling of it.
//
// The '0' entry is the exception, and it is deliberately spelt with the
// core/cat constant, because that is exactly what it is: ASSUMED, inherited,
// and named by the codec rather than by this manual. See doc.go's register.
var modeNames = map[cat.Mode]string{
	// ASSUMED — cat.ModeUnset ('0', "-") appears in NO FTdx10 mode legend;
	// all four run 1-F. It is here because parsers must accept the
	// placeholder: core/cat refuses to EMIT it in any Set frame, so its
	// presence widens what this dialect can read and nothing else.
	cat.ModeUnset: "-",

	cat.Mode('1'): "LSB",
	cat.Mode('2'): "USB",
	cat.Mode('3'): "CW-U",
	cat.Mode('4'): "FM",
	cat.Mode('5'): "AM",
	cat.Mode('6'): "RTTY-L",
	cat.Mode('7'): "CW-L",
	cat.Mode('8'): "DATA-L",
	cat.Mode('9'): "RTTY-U",
	cat.Mode('A'): "DATA-FM",
	cat.Mode('B'): "FM-N",
	cat.Mode('C'): "DATA-U",
	cat.Mode('D'): "AM-N",
	cat.Mode('E'): "PSK",
	cat.Mode('F'): "DATA-FM-N",
}

// dialect is the FTdx10, built once at init and validated by
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
var dialect = cat.MustNewDialect(cat.DialectConfig{
	CATID:     "0761",
	ModeNames: modeNames, // fresh transcription, 1..F per the manual;
	// '0' (ModeUnset, "-") included as an ASSUMED member — absent
	// from every FTdx10 legend; parsers must accept the placeholder.
	Slots: cat.SlotSpace{
		MemoryLo: 1, MemoryHi: 99,
		// ASSUMED bounds — the FTdx10 legends say only "5xx (5MHz
		// BAND)"; 501..599 (start, ceiling and count) is interpretation
		// inherited from the FT-710, whose own reference marks exactly
		// this numbering unverified. The SlotSpace.SixtyLo/SixtyHi register
		// entry.
		SixtyLo: 501, SixtyHi: 599,
		PMSPairs:      9,
		EmergencyWire: "EMG",
		NoneWire:      "000", // ASSUMED — in no FTdx10 slot legend
		// The FTdx10's MC block prints all four slot classes —
		// "001-099 (Memory Channel), P1L-P9U (PMS), 5xx (5MHz BAND),
		// EMG (EMERGENCY CH)" (layout 1131-1133) — so an MC Set may
		// select every one of them on this radio. Not an assumption:
		// this is the legend, transcribed.
		MCSelects: cat.MCSelectsAll,
	},
	EXItems: exItems,
	// The FTdx10's EX grammar block prints "E X P1 P1 P2 P2 P3 P3"
	// (ftdx10_layout.txt:636-645, Read frame at 642, Answer at 645): six
	// digits, three components.
	EXAddressForm: cat.EXAddressTriple,
	MT: cat.MTPolicy{
		Form: cat.MTFormCombined,
		// The FTdx10's MT block prints the same four slot classes its MR
		// block does — "001-099 (Memory Channel), P1L-P9U (PMS), 5xx (5MHz
		// BAND), EMG (EMERGENCY CH)" (layout 1218) — so an MT read may name
		// every slot this dialect can read. Not an assumption: this is the
		// legend, transcribed. The FT-891's MT legend prints memory and PMS
		// only, which is the disagreement this axis carries.
		ReadSlots:    cat.MTReadsReadable,
		TagMaxBytes:  12,
		ClearTagByte: 0,   // must be 0 under MTFormCombined (V9)
		PadByte:      0,   // must be 0 under MTFormCombined (V9)
		TagFill:      ' ', // ASSUMED — the FTdx10's padding byte has
		// never been observed
		// The FTdx10's MT block prints "P11 0: (Fixed)" (layout 1235), so
		// byte 28 of its combined record is schema and carries no state.
		// Not an assumption: this is the legend, transcribed. The FT-891
		// prints `P11 0: TAG "OFF" 1: TAG "ON"` there, which is the
		// disagreement this axis carries.
		P11: cat.P11Fixed,
	},
	Clarifier: cat.ClarifierPolicy{
		StepHz: 10, // ASSUMED — no step stated anywhere in the
		// manual; the 9990 range supports, not proves
		MaxAbsHz: 9990,
	},
	// The FTdx10's memory blocks print P5 "0: TX CLAR \"OFF\" 1: TX CLAR
	// \"ON\"" (layout 1226, the MT block; MR and MW print the same), so
	// byte 21 carries this radio's TX clarifier flag in both directions.
	// Not an assumption: this is the legend, transcribed. The FT-891 prints
	// "0: (Fixed)" on every one of those blocks, which is the disagreement
	// this axis carries.
	MemoryP5:    cat.P5TxClar,
	MWWriteKind: cat.CombinedMTSetKind, // the FTdx10's MW P7
	// "(Fixed)" — equal to the combined MT Set constant AS A FACT OF
	// THIS RADIO, not a rule; see the difference pins.
})

// Dialect returns the FTdx10's cat.Dialect.
//
// A function over an exported var so that the package-held value cannot be
// reassigned by a consumer: a Dialect is what the outbound write gate
// consults on every frame, and one a caller can swap after init is not a
// gate. cat.Dialect is a value type carrying only copied maps and slices,
// so the returned copy is inert in the other direction too.
func Dialect() cat.Dialect { return dialect }
