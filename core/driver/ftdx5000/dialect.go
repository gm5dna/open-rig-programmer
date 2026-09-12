// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx5000

import (
	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"
)

// modeNames is the FTdx5000's P6 mode table, transcribed from the memory
// record's own legend (matrix §1.3, layout:945-947/977-979): twelve of
// core/cat's fifteen named modes, D/E/F absent. The wire-nibble spellings
// follow core/cat/mode.go's own family naming (matrix §4's own framing:
// "PKT-L"/"PKT-FM"/"PKT-U" render as "DATA-L"/"DATA-FM"/"DATA-U",
// "FSK(RTTY-LSB)"/"FSK-R(RTTY-USB)" render as "RTTY-LSB"/"RTTY-USB") EXCEPT
// where this radio's own legend genuinely diverges from core/cat's
// package-level table: nibble '3' is "CW" here (core/cat's fallback:
// "CW-U") and nibble '7' is "CW-R" (core/cat's fallback: "CW-L") — both
// transcribed verbatim from this radio's own legend, not translated.
//
// cat.ModeUnset ('0', "-") is an ASSUMED accept-only member: it appears in
// no FTdx5000 mode legend (MR/MW/MD all print 1-9,A-C only), but parsers
// must accept it and core/cat's own NewDialect never lets a builder emit
// it (memdata.go's validateSetFields refuses ModeUnset in a Set frame
// unconditionally).
var modeNames = map[cat.Mode]string{
	cat.ModeUnset: "-",

	cat.Mode('1'): "LSB",
	cat.Mode('2'): "USB",
	cat.Mode('3'): "CW",
	cat.Mode('4'): "FM",
	cat.Mode('5'): "AM",
	cat.Mode('6'): "RTTY-LSB",
	cat.Mode('7'): "CW-R",
	cat.Mode('8'): "DATA-L",
	cat.Mode('9'): "RTTY-USB",
	cat.Mode('A'): "DATA-FM",
	cat.Mode('B'): "FM-N",
	cat.Mode('C'): "DATA-U",
}

// dialect is the FTdx5000, built once at init and validated by
// cat.MustNewDialect's rules. Built INLINE (doc.go explains why no
// core/cat/ftdx5000 subpackage exists): every field is set explicitly,
// including the ones this configuration requires to be a structural
// placeholder (MT, EXAddressForm — see their own comments below) rather
// than a real protocol fact for this radio.
var dialect = cat.MustNewDialect(cat.DialectConfig{
	// ID's own P1 legend, "P1 0362: FTDX5000" (matrix §4, layout:770).
	CATID:     "0362",
	ModeNames: modeNames,
	Slots: cat.SlotSpace{
		// "001 - 099: Regular Memory Channel" (matrix §1.5, MC's own
		// legend, layout:891).
		MemoryLo: 1, MemoryHi: 99,
		// NO 5 MHz BANK, NO EMERGENCY CHANNEL: neither "5xx"/"5MHz" nor
		// "EMG" appears anywhere in this manual's slot legends (matrix,
		// checked mechanically over the whole extraction) — a
		// TRANSCRIBED absence, not an assumed one.
		SixtyLo: 0, SixtyHi: 0,
		// Nine pairs, NUMERIC form: MC's own legend runs "100: P1L 101:
		// P1U ~ 116: P9L 117: P9U" (matrix §1.5, layout:892-897) —
		// decimal channel numbers continuing the memory range, exactly
		// FT-991A's PMSFormNumeric shape, not the token "P1L"-"P9U" every
		// other registered sibling prints.
		PMSPairs: 9,
		PMSForm:  cat.PMSFormNumeric,
		// Pair 1's lower slot, "100" (matrix §1.5).
		PMSNumericLo: 100,
		// No "000"/VFO placeholder is documented anywhere in this
		// manual's MR, MW or MC legends (unlike the FT-710's, which
		// prints one) — left absent rather than assumed; see doc.go.
		EmergencyWire: "",
		NoneWire:      "",
		// MC's own legend enumerates 001-117 only — memory and PMS, no
		// 60m/EMG class printed (there being none to print) — layout:
		// 890-897. Direct transcription; see doc.go's ASSUMED register
		// entry 5.
		MCSelects: cat.MCSelectsMemoryPMS,
	},
	// No EX (menu) inventory is modelled this wave (brief: "EX/menu
	// inventory is OUT for all seven packages... EXItems empty, MaxEXAddress
	// unset-refused"). EXAddressForm is nonetheless a REQUIRED field (V12
	// refuses the zero value even for an empty EXItems) — cat.
	// EXAddressSingle IS manual-evidenced, though: the EX grammar block's
	// own address field is a bare three-digit "E X P1 P1 P1", "P1 :
	// 001-176 (MENU Number)" (layout:463-472), the same one-component
	// shape as the FT-991A's, not the six-digit triple every other
	// registered sibling's EX grammar prints. No EXItems are built from
	// it this wave.
	EXItems:       nil,
	EXAddressForm: cat.EXAddressSingle,
	// MT is a STRUCTURALLY REQUIRED field (V9 refuses the zero Form) for
	// a command this radio's Control Command List does not carry at all
	// (doc.go). The smallest legal, never-exercised placeholder:
	// MTFormShort with a 1-byte tag ceiling, which core/cat/dialecttest's
	// own suite explicitly anticipates ("a dialect declaring TagMaxBytes
	// 1, which V9 permits"). No method on this dialect's driver ever
	// calls BuildMTRead, BuildMTSet or ParseMTAnswer*.
	MT: cat.MTPolicy{
		Form:         cat.MTFormShort,
		ReadSlots:    cat.MTReadsMemoryPMS,
		TagMaxBytes:  1,
		ClearTagByte: ' ',
	},
	// 0000-9999 printed (MW's own P3 legend, matrix §2); the 10 Hz step
	// and 9990 ceiling are ASSUMED — doc.go's register entry 2, the
	// family's single shared entry, cited not re-derived.
	Clarifier: cat.ClarifierPolicy{StepHz: 10, MaxAbsHz: 9990},
	// P5 "0: TX CLAR OFF 1: TX CLAR ON" on both MR and MW (matrix §2,
	// layout:944/976) — a live flag, not the FT-891's printed-fixed byte.
	MemoryP5: cat.P5TxClar,
	// P8 "0: CTCSS OFF 1: CTCSS ENC/DEC 2: CTCSS ENC" (matrix §4,
	// layout:949/982) — three states, no DCS.
	ToneStates: cat.ToneStatesCTCSS,
	// 27-byte frame, 8-digit P2 (matrix §1.1/§1.2), the ft2000 family's
	// shape Lift Y built the axis for.
	MemoryFrameLen:   27,
	MemoryFreqDigits: 8,
	// P9 is a LIVE two-digit CTCSS tone-table index, 00-49 (matrix §1.4)
	// — see doc.go's own section on why this is mapped rather than left
	// zero.
	MemoryP9: cat.P9ToneIndex,
	// P7 "0: (Fixed)" on MW's Set (matrix §2, layout:981) — this
	// dialect's write-fixed byte reuses the EXISTING cat.KindVFO ('0'),
	// no new Kind value needed.
	MWWriteKind: cat.KindVFO,
})

// catID is this dialect's CAT ID, sourced from the dialect rather than
// restated: one place this string exists, and the value the ID probe
// compares against is the same value the capability data advertises.
var catID = dialect.CATID()

// ctcssVocab is this radio's CTCSS state vocabulary, in P8's own legend
// order (matrix §4, layout:949/982) — the order write.go's refusal text
// names the states in.
var ctcssVocab = []yaesu.CTCSSName{
	{Name: "OFF", State: cat.CTCSSOff},
	{Name: "ENC-DEC", State: cat.CTCSSEncDec},
	{Name: "ENC", State: cat.CTCSSEnc},
}

// ctcssNames is ctcssVocab's read-direction lookup (wire state -> display
// name), the mirror of yaesu.CTCSSMap's write-direction one.
var ctcssNames = map[cat.CTCSSState]string{
	cat.CTCSSOff:    "OFF",
	cat.CTCSSEncDec: "ENC-DEC",
	cat.CTCSSEnc:    "ENC",
}

// shiftNames is the read-direction repeater-shift lookup (wire shift ->
// display name), the mirror of yaesu.ShiftByName.
var shiftNames = map[cat.Shift]string{
	cat.ShiftSimplex: "SIMPLEX",
	cat.ShiftPlus:    "PLUS",
	cat.ShiftMinus:   "MINUS",
}

// eraseReason is the refusal text for a write of an empty channel: this
// radio's CAT command set has no erase command (doc.go's audit reason,
// caps.go), so this codec cannot express one.
const eraseReason = "the FTdx5000's CAT command set has no erase command, so this codec cannot express an erase, and FieldErase is not write-Supported"
