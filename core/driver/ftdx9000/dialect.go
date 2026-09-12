// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx9000

import "github.com/gm5dna/open-rig-programmer/core/cat"

// modeNames is the FTdx9000's P6 mode table, TRANSCRIBED from this radio's
// own manual (matrix §1.5) rather than copied from any sibling's. The five
// blocks that carry it (IF, MD, MR, MW, OI) agree word for word — unlike
// the FT-991A, whose MD legend disagrees with its memory legends — so
// there is only one table to transcribe.
//
// Twelve real nibbles, '1'-'C' with none skipped; 'D'/'E'/'F' are printed
// on NO block (matrix §1.5) and are simply absent from this map — ValidMode
// says so without special-casing a hole.
//
// The names keep the manual's own spelling, parenthetical and all ("FSK
// (RTTY-LSB)", not "RTTY-LSB"): '6', '7', '8', '9', 'A' and 'C' are spelt
// differently here than on cat.Mode's canonical fallback (CW/CW-R vs
// CW-U/CW-L, PKT vs DATA) on the SAME underlying nibbles — a spelling
// difference the matrix flags as a CHOICE for the driver (§1.5), not a
// second real mode. No per-model ModeName override is built for it: unlike
// the FT-991A's 'E' (two DIFFERENT real modes on one nibble), no nibble
// here disagrees about WHICH mode it is, only about the word for it, and
// core/cat's package-level fallback is merely a different — not a wrong —
// display word. See doc.go.
var modeNames = map[cat.Mode]string{
	// ASSUMED — cat.ModeUnset ('0', "-") appears in no FTdx9000 mode
	// legend; parsers must accept the placeholder regardless.
	cat.ModeUnset: "-",

	cat.Mode('1'): "LSB",
	cat.Mode('2'): "USB",
	cat.Mode('3'): "CW",
	cat.Mode('4'): "FM",
	cat.Mode('5'): "AM",
	cat.Mode('6'): "FSK (RTTY-LSB)",
	cat.Mode('7'): "CW-R",
	cat.Mode('8'): "PKT-L",
	cat.Mode('9'): "FSK-R (RTTY-USB)",
	cat.Mode('A'): "PKT-FM",
	cat.Mode('B'): "FM-N",
	cat.Mode('C'): "PKT-U",
	// 'D', 'E', 'F' printed on no block (matrix §1.5) — deliberately
	// absent.
}

// dialect is the FTdx9000, built once at init and validated by
// cat.MustNewDialect. EVERY FIELD IS SET EXPLICITLY, including the ones
// this configuration requires to be zero (SixtyLo/Hi, EmergencyWire, the
// short-form MT fields this radio never actually sends — see below).
//
// MustNewDialect rather than NewDialect: a mistake in this literal is a
// build-time defect that must stop the programme loudly on first use.
var dialect = cat.MustNewDialect(cat.DialectConfig{
	// matrix §1.2: the ID legend prints THREE four-digit answers for the
	// one row this project registers — 0101 (FTDX9000D), 0102 (Contest),
	// 0103 (MP). DialectConfig.CATID is a single string, so 0101 (the
	// base/D variant) is this dialect's own canonical identity; ftdx9000.go
	// accepts all three at the ID probe (see acceptedCATIDs), which is this
	// package's own decision, not a matrix pin (matrix §1.2 leaves it
	// explicitly open).
	CATID:     "0101",
	ModeNames: modeNames,
	Slots: cat.SlotSpace{
		// matrix §1.4: "001"-"099", MANUAL-EVIDENCED by the MC legend's own
		// decomposition.
		MemoryLo: 1, MemoryHi: 99,
		// NO 60 m BANK (matrix §1.4): no "5xx"/"5 MHz"/"EMG" slot legend
		// anywhere in the document, checked mechanically over the whole
		// extraction.
		SixtyLo: 0, SixtyHi: 0,
		// Nine numeric pairs, "100: P1L" ... "117: P9U" (matrix §1.4),
		// the SAME numeric-PMS form as FT-991A's dialect.
		PMSPairs:     9,
		PMSForm:      cat.PMSFormNumeric,
		PMSNumericLo: 100,
		// NO EMERGENCY CHANNEL (matrix §1.4): transcribed absence, not
		// assumed.
		EmergencyWire: "",
		NoneWire:      "000",
		// Inert on this radio either way: MCSelectsAll and
		// MCSelectsMemoryPMS differ only over the 60m/EMG banks, and this
		// radio has neither. MCSelectsMemoryPMS is chosen as the more
		// conservative reading — the matrix cites no MC legend claiming a
		// wider domain.
		MCSelects: cat.MCSelectsMemoryPMS,
	},
	// EX/menu inventory is OUT of scope for this package (brief, spec.md
	// §3): no EXItems, and EXAddressForm is DialectConfig's own structural
	// requirement (no zero value) rather than a claim about this radio's
	// menu grammar. EXAddressTriple is the majority form and is never
	// exercised, since EXItems is empty.
	EXItems:       nil,
	EXAddressForm: cat.EXAddressTriple,
	// THIS RADIO'S MANUAL DOCUMENTS NO MT (COMBINED MEMORY+TAG) COMMAND AT
	// ALL — matrix §2's whole record is MR/MW only, and MTPolicy is
	// DialectConfig's own structural requirement, not a claim this radio
	// has an MT command. MTFormShort with the smallest legal TagMaxBytes
	// (1) is configured and NEVER EXERCISED: this package's read/write
	// paths (read.go, write.go) call BuildMRRead/ParseMRAnswer/BuildMWSet
	// directly and never a BuildMTSet*/ParseMTAnswer* function. See doc.go.
	MT: cat.MTPolicy{
		Form:         cat.MTFormShort,
		ReadSlots:    cat.MTReadsReadable,
		TagMaxBytes:  1,
		ClearTagByte: ' ',
	},
	// matrix §1.7: printed "0000-9999 (Hz)" with no step stated — a pure
	// position shift of the family's own ASSUMED 9990/10 register entry,
	// not re-derived.
	Clarifier: cat.ClarifierPolicy{StepHz: 10, MaxAbsHz: 9990},
	// matrix §2, P5 (byte 19/position 20): a LIVE "TX CLAR on/off" digit,
	// not fixed — this radio joins the FT-991A/FTdx101 side of that split.
	MemoryP5: cat.P5TxClar,
	// matrix §1.16: P8 prints THREE states only (OFF/ENC-DEC/ENC) — no
	// DCS member.
	ToneStates: cat.ToneStatesCTCSS,
	// matrix §2, P7 (byte 21/position 22): write side hard-fixed '0',
	// which mode.go names KindVFO — NOT KindMemory, the naming trap the
	// brief flags for this family.
	MWWriteKind: cat.KindVFO,
	// Lift Y (spec.md §2, commit 99cdaf9): this radio's MR-answer/MW-set
	// frame is 27 bytes with an 8-digit P2, one byte narrower than the
	// registered 28/9 shape (matrix §2).
	MemoryFrameLen:   27,
	MemoryFreqDigits: 8,
	// matrix §1.10: P9 (positions 24-25) is a LIVE two-digit index into the
	// SAME standard 50-tone chart every registered dialect shares — the
	// Lift-Y P9ToneIndex axis, not the fixed "00" every sibling declares.
	MemoryP9: cat.P9ToneIndex,
})

// Dialect returns the FTdx9000's cat.Dialect.
//
// A function over an exported var so the package-held value cannot be
// reassigned by a consumer — see the sibling dialects' identical reasoning.
func Dialect() cat.Dialect { return dialect }
