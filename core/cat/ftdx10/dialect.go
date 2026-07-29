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
// cat.MustNewDialect's eleven rules. EVERY FIELD IS SET EXPLICITLY,
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
		SixtyLo: 501, SixtyHi: 599,
		PMSPairs:      9,
		EmergencyWire: "EMG",
		NoneWire:      "000", // ASSUMED — in no FTdx10 slot legend
	},
	EXItems: exItems,
	MT: cat.MTPolicy{
		Form:         cat.MTFormCombined,
		TagMaxBytes:  12,
		ClearTagByte: 0,   // must be 0 under MTFormCombined (V9)
		PadByte:      0,   // must be 0 under MTFormCombined (V9)
		TagFill:      ' ', // ASSUMED — the FTdx10's padding byte has
		// never been observed
	},
	Clarifier: cat.ClarifierPolicy{
		StepHz: 10, // ASSUMED — no step stated anywhere in the
		// manual; the 9990 range supports, not proves
		MaxAbsHz: 9990,
	},
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
