// SPDX-License-Identifier: GPL-3.0-or-later

package ft991a

import "github.com/gm5dna/open-rig-programmer/core/cat"

// modeNames is the FT-991A's P6 mode table, TRANSCRIBED FRESH from this
// radio's own manual rather than copied from the FTdx10's or the FT-891's.
//
// It has to be typed out again for the reason core/cat/ftdx10/dialect.go
// gives: core/cat's table is unexported, so there is nothing to reference
// even if referencing it were right — and it would not be. Two radios
// agreeing on a mode nibble is a fact about those two radios, not a shared
// definition. This radio is a sharper proof of that than either sibling,
// because it is the first whose table makes core/cat's own package-level
// FALLBACK actively wrong: cat.Mode('E').String() renders "PSK", the
// FT-710's word, where this manual prints "C4FM". See doc.go's section
// "The mode fallback is WRONG for this radio", and
// TestModeStringFallbackIsWrongHere, which pins both halves.
//
// SOURCE: the mode legend printed beside FIVE commands in manual revision
// 1711-D — MR's P6 (ft991a_layout.txt:973-975), MT's (1006-1008), MW's
// (1044-1046), IF's (789-791) and OI's (1124-1126). THE FIVE ARE
// IDENTICAL: the same fourteen names against the same fourteen nibbles, in
// the same order, with no hole and no 'F'. The keys below are the legend's
// own wire bytes, written as byte literals rather than through core/cat's
// Mode constants, so that what is transcribed here is this manual's
// "6: RTTY-LSB" and not core/cat's spelling of it.
//
// MD'S OWN LEGEND IS NOT THIS TABLE and is deliberately not merged into it.
// The MD (OPERATING MODE) command prints the same fourteen nibbles with
// "3: CW-U" and "7: CW-L" (ft991a_layout.txt:927-929) where all five memory
// legends print "3: CW" and "7: CW-R". This dialect encodes the MEMORY
// commands' P6, so the memory legend is the one transcribed; the rival
// spelling is recorded in doc.go rather than resolved, because this
// repository has no FT-991A to ask which word the radio would display.
//
// THERE IS NO HOLE AND NO 'F'. The FTdx10's legend fills 'F' with
// "DATA-FM-N" and the FT-891's prints "A: -", a hole; this one runs 1..9
// then A..E with every nibble named. TestDifferencePinModeMembership holds
// all three facts against their counter-examples.
//
// The '0' entry is the exception, and it is deliberately spelt with the
// core/cat constant, because that is exactly what it is: ASSUMED,
// inherited, and named by the codec rather than by this manual. See doc.go's
// register, entry "THE cat.ModeUnset MEMBER OF THE MODE TABLE".
var modeNames = map[cat.Mode]string{
	// ASSUMED — cat.ModeUnset ('0', "-") appears in NO FT-991A mode legend;
	// all five run 1..9 then A..E. It is here because parsers must accept
	// the placeholder: core/cat refuses to EMIT it in any Set frame, so its
	// presence widens what this dialect can read and nothing else. See
	// doc.go's register, entry "THE cat.ModeUnset MEMBER OF THE MODE TABLE".
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
	cat.Mode('A'): "DATA-FM",
	cat.Mode('B'): "FM-N",
	cat.Mode('C'): "DATA-USB",
	cat.Mode('D'): "AM-N",
	// 'E' is C4FM here and PSK on the FTdx10: one nibble, two REAL and
	// different modes. It is the reason core/cat's package-level fallback is
	// wrong for this radio and merely unauthoritative for the others.
	cat.Mode('E'): "C4FM",
	// 'F' is not printed in any of the five legends.
}

// dialect is the FT-991A, built once at init and validated by
// cat.MustNewDialect's sixteen rules. EVERY FIELD IS SET EXPLICITLY,
// including the four this configuration requires to be zero or empty —
// SixtyLo, SixtyHi, EmergencyWire and the combined form's ClearTagByte and
// PadByte: a field left out of this literal would be indistinguishable from
// a field deliberately zeroed, and "an inapplicable field must be explicitly
// zero" is only readable as a decision if the zero is written down.
//
// MustNewDialect rather than NewDialect because this is a compile-time
// constant table: a mistake in it is a build-time defect that must stop the
// programme loudly on first use, not an error threaded through model
// registration.
//
// THIS DIALECT SPLITS THE TWO SIBLINGS IT MOST RESEMBLES. Against the FTdx10
// it declares the numeric PMS form and the whole slot number line, no 60m
// bank, no emergency channel, a three-digit EX address, a fourteen-name mode
// table and a five-state P8 domain. Against the FT-891 it declares P11Fixed
// and P5TxClar, where that radio declares P11TagDisplay and P5Fixed — so
// each radio's builders make what the other's refuse. None of that is an
// assumption; the assumptions this dialect does carry are marked ASSUMED
// below and registered by name in doc.go.
var dialect = cat.MustNewDialect(cat.DialectConfig{
	// The ID block prints "P1 0670: FT-991A" (ft991a_layout.txt:772), and
	// evidence leg G counted the seven-byte Answer chart that carries it
	// (testdata/mc-vectors.golden's ID section).
	CATID:     "0670",
	ModeNames: modeNames, // fresh transcription, 1..9 and A..E per the five
	// memory legends; '0' (ModeUnset, "-") included as an ASSUMED member —
	// absent from every FT-991A legend; parsers must accept the placeholder.
	Slots: cat.SlotSpace{
		// "001 - 099: Regular Memory Channel", the MC block's legend
		// (ft991a_layout.txt:915). THE MC BLOCK IS THE ONLY LEGEND IN THIS
		// MANUAL THAT DECOMPOSES THE SPAN: MR (966), MT (999), MW (1037),
		// IF (783) and OI (1117) all print the outer "001-117 (Memory
		// Channel)" and nothing else. Evidence leg G recorded that
		// difference verbatim and declined to resolve it
		// (testdata/mc-vectors.golden).
		MemoryLo: 1, MemoryHi: 99,
		// NO 5 MHz BANK. "5xx", "5 MHz" and "5MHz" appear in no slot legend
		// of this manual — checked mechanically over the whole extraction;
		// the only "5 MHz" is the BS band-select command's band list
		// (ft991a_layout.txt:324), which is not a memory bank. (0, 0) is
		// how cat.SlotSpace spells absent, and it is a TRANSCRIBED absence,
		// not an assumed one: the FTdx10's 501..599 sits on that other
		// dialect's own register of assumptions, because its manual prints
		// "5xx (5MHz BAND)" and this one prints nothing at all.
		// TestDifferencePinAbsentBanks holds it against the FTdx10, which
		// has the bank.
		SixtyLo: 0, SixtyHi: 0,
		// Nine pairs: the MC legend runs "100: P-1L 101: P-1U ~ 116: P-9L
		// 117: P-9U" (ft991a_layout.txt:916), which is nine lower/upper
		// pairs over eighteen consecutive channel numbers.
		PMSPairs: 9,
		// THE PAIRS ARE DECIMAL CHANNEL NUMBERS ON THIS RADIO, not tokens.
		// The same legend gives them numbers continuing the memory range,
		// where every registered sibling's legend spells them "P1L - P9U
		// (PMS)". Not an assumption: this is the legend, transcribed. Under
		// this form the pair number never reaches the wire at all, so this
		// dialect builds and accepts no token form — which is what
		// TestDifferencePinPMSFormAndSlotSpace holds in both directions.
		PMSForm: cat.PMSFormNumeric,
		// Pair 1's LOWER slot, "100" (ft991a_layout.txt:916). The range's
		// top is DERIVED from this and PMSPairs rather than declared beside
		// them, so there is no second field for it to disagree with.
		PMSNumericLo: 100,
		// NO EMERGENCY CHANNEL. "EMG" appears in NO slot legend of this
		// manual — checked mechanically over the whole extraction — where
		// the FTdx10's and the FT-891's MR legends both print "EMG". The
		// word EMERGENCY does appear once, and it is not a bank: the chart's
		// row 149 EMERGENCY FREQ TX (ft991a_layout.txt:690, and this
		// package's testdata/transcription-b.csv:150) is a menu that enables
		// a transmission. "" is how cat.SlotSpace spells absent, and this
		// absence is transcribed too. doc.go and TestDifferencePinAbsentBanks
		// scope the same statement the same way.
		EmergencyWire: "",
		// ASSUMED — "000" appears in NO FT-991A slot legend. MC's gives
		// 001-117 with its PMS decomposition (913-916), and MR's, MT's,
		// MW's, IF's and OI's give the bare span (966, 999, 1037, 783,
		// 1117). It is the FT-710's MR-answer fact, and cat.SlotSpace
		// structurally requires a none form, so one is supplied. See doc.go's
		// register, entry "SlotSpace.NoneWire".
		NoneWire: "000",
		// The FT-991A's MC block prints the WHOLE span — "P1 001 - 117:
		// Memory Channel Number" (ft991a_layout.txt:913) — so an MC Set may
		// name every slot this radio has. Not an assumption: this is the
		// legend, transcribed, and evidence leg G reduced it to four
		// hand-derived frames (MC012;, MC099;, MC100;, MC117;).
		//
		// IT IS A CITATION, AND ON THIS RADIO IT IS ALSO INERT. MCSelectsAll
		// and MCSelectsMemoryPMS differ only over the 60m and EMG banks, and
		// this radio has neither, so the two give identical verdicts on
		// every wire form there is. The wide value is declared because it is
		// what the legend says, not because anything currently depends on
		// it; TestDegeneracyPinWideAndNarrowAgree converts that coincidence
		// into a checked fact that fails loudly if a bank is ever added.
		MCSelects: cat.MCSelectsAll,
	},
	EXItems: exItems,
	// The FT-991A's EX grammar block prints "P1 : 001 - 153 (MENU Number)"
	// for the address and "E X P1 P1 P1 ;" for the Read
	// (ft991a_layout.txt:519-528): THREE digits in ONE component, where the
	// FTdx10 prints six in three and the FT-891 four in two. Its Read frame
	// is therefore six bytes, the narrowest in the family.
	EXAddressForm: cat.EXAddressSingle,
	MT: cat.MTPolicy{
		// The MT Set/Answer charts run to 41 positions — the 28 shared
		// memory positions, P11 at 28, a 12-byte P12 tag at 29-40 and ';'
		// at 41 (ft991a_layout.txt:998-1033). Evidence leg G counted the
		// grid twice, left-to-right and by row totals, and recorded both
		// counts (testdata/mt-vectors.golden).
		Form: cat.MTFormCombined,
		// The FT-991A's MT block prints its slot legend as the whole span,
		// "P0/1 001-117 (Memory Channel)" (ft991a_layout.txt:999), and its
		// Read chart is "M T P0 P0 P0 ;" (1018) — the same P0 the legend
		// names, so the read's domain is the block's own span. The FT-891's
		// MT legend prints memory and PMS only, which is the disagreement
		// this axis carries. Not an assumption: this is the legend,
		// transcribed — and, like MCSelects above, inert on a radio with no
		// 60m and no EMG bank, which the degeneracy pin states.
		ReadSlots: cat.MTReadsReadable,
		// "P12 TAG Characters (up to 12 characters) (ASCII)"
		// (ft991a_layout.txt:1017), and the Set chart draws that field over
		// positions 29-40 — twelve.
		TagMaxBytes:  12,
		ClearTagByte: 0, // must be 0 under MTFormCombined (V9)
		PadByte:      0, // must be 0 under MTFormCombined (V9)
		// ASSUMED — the byte this radio pads a short tag with, in both
		// directions. The P12 legend names a width and an alphabet and no
		// fill; no FT-991A has ever been asked. See doc.go's register,
		// entry "MTPolicy.TagFill".
		TagFill: ' ',
		// The FT-991A's MT block prints "P11 0: (Fixed)"
		// (ft991a_layout.txt:1015), so byte 28 of its combined record is
		// SCHEMA — where the FT-891 prints `P11 0: TAG "OFF" 1: TAG "ON"`
		// and the byte is a live flag. Not an assumption: this is the
		// legend, transcribed. Under this policy the display-BEARING pair
		// (BuildMTSetCombinedDisplay, ParseMTAnswerCombinedDisplay) refuses,
		// because there is no flag for a caller to set or a radio to report;
		// TestDifferencePinMTP11 holds both halves against the FT-891, where
		// the refusals run the other way.
		P11: cat.P11Fixed,
	},
	Clarifier: cat.ClarifierPolicy{
		// BOTH FIELDS ARE ASSUMED, and they are ONE register entry with one
		// lifting capture (doc.go, entry "ClarifierPolicy.StepHz = 10 AND
		// ClarifierPolicy.MaxAbsHz"). The manual prints "Clarifier Offset:
		// 0000 - 9999 (Hz)" on every block that carries the field — IF 785,
		// MR 968, MT 1001, MW 1039, OI 1119 — and states NO step. 9999 is
		// not a multiple of the inherited 10, so this pair cannot be read
		// off the printed range: the ceiling here is 9990, the largest
		// multiple of the assumed step inside the printed range, which is a
		// DEDUCTION FROM AN ASSUMPTION and not a transcription.
		StepHz:   10,
		MaxAbsHz: 9990,
	},
	// The FT-991A's memory blocks print `P5 0: TX CLAR "OFF" 1: TX CLAR
	// "ON"` on every one of them — MR 971, MT 1004, MW 1042, IF 787 and
	// OI 1122 — where the FT-891 prints "P5 0: (Fixed)". So byte 21 carries
	// a live TX-clarifier state on this radio, in both directions. Not an
	// assumption: this is the legend, transcribed. TestDifferencePinMemoryP5
	// holds it against the FT-891, which refuses the TxClar-true record this
	// dialect builds.
	MemoryP5: cat.P5TxClar,
	// The FT-991A's P8 legend prints FIVE states — "0: CTCSS \"OFF\"
	// 1: CTCSS ENC/DEC 2: CTCSS ENC 3: DCS ENC/DEC 4: DCS ENC" — on all five
	// blocks that carry the field (ft991a_layout.txt:795-796, 977-978,
	// 1010-1011, 1048-1049, 1128-1129), where every registered sibling
	// prints 0/1/2 only. Not an assumption: this is the legend, transcribed.
	// Whether the radio ACCEPTS a DCS state written without a CN code first
	// is a different question, and it is on doc.go's register as "THE DCS
	// STATES' SET ACCEPTANCE".
	ToneStates: cat.ToneStatesCTCSSAndDCS,
	// The FT-991A's MW legend prints "P7 00: (Fixed)"
	// (ft991a_layout.txt:1047) against a ONE-position field — a printed
	// width defect doc.go records — and cat.CombinedMTSetKind is the byte
	// '0', so the constant on the right is the correct SPELLING of the one
	// character the grid draws. That the two coincide is A FACT OF THIS
	// RADIO, not a rule: MW's P7 and the combined MT Set's P7 (also
	// "Set: 0: (Fixed)", 1009) are different fields of different commands
	// that this manual happens to fix at the same byte. See
	// TestIdentityPinMWWriteKind, which says exactly this much and no more.
	MWWriteKind: cat.CombinedMTSetKind,
})

// Dialect returns the FT-991A's cat.Dialect.
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
