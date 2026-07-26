// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import "fmt"

// memoryFrameLen is the fixed length of an MR-answer/MW-set frame: 28
// bytes. Reference: "MR — MEMORY CHANNEL READ" position table, shared
// byte-for-byte by MW's Set frame ("MW — ... Set frame: identical 28-byte
// layout with MW").
const memoryFrameLen = 28

// Byte offsets (0-indexed) into a 28-byte MR-answer/MW-set frame, shared by
// mr.go's decoder and mw.go's encoder. Comments give the reference's
// 1-indexed position (Pn) for cross-checking against the manual's table.
const (
	memSlotOffset     = 2  // P1, positions 3-5, 3 bytes
	memFreqOffset     = 5  // P2, positions 6-14, 9 bytes
	memClarSignOffset = 14 // P3 sign, position 15, 1 byte
	memClarMagOffset  = 15 // P3 magnitude, positions 16-19, 4 bytes
	memRxClarOffset   = 19 // P4, position 20, 1 byte
	memTxClarOffset   = 20 // P5, position 21, 1 byte
	memModeOffset     = 21 // P6, position 22, 1 byte
	memKindOffset     = 22 // P7, position 23, 1 byte
	memCTCSSOffset    = 23 // P8, position 24, 1 byte
	memP9Offset       = 24 // P9, positions 25-26, 2 bytes, fixed "00"
	memShiftOffset    = 26 // P10, position 27, 1 byte
	memTermOffset     = 27 // position 28, 1 byte, ';'
)

// Field widths for the offsets above.
const (
	memFreqDigits    = 9
	memClarMagDigits = 4
)

// memFreqMax is the largest FreqHz value that fits the 9-digit P2 field.
const memFreqMax = 999_999_999

// MemoryData is the fully decoded content of an MR-answer or MW-set frame:
// one memory/PMS/5xx/EMG channel's frequency, clarifier, mode and related
// state. Reference: "MR — MEMORY CHANNEL READ" position table (P1-P10),
// shared byte-for-byte with MW's Set frame.
type MemoryData struct {
	Slot Slot

	// FreqHz is the channel frequency in Hz. Reference P2: "frequency in
	// Hz, 9 digits zero-padded" -> max representable value 999,999,999.
	FreqHz uint32

	// ClarHz is the clarifier offset in Hz: signed, a multiple of 10,
	// magnitude 0-9990. Reference P3: "clarifier: +/- then 4-digit offset
	// 0000-9990 Hz".
	ClarHz int16

	RxClar bool // Reference P4: "RX CLAR: 0 off, 1 on".
	TxClar bool // Reference P5: "TX CLAR: 0 off, 1 on".

	Mode Mode // Reference P6, "Mode nibble (P6)" table.

	// Kind is the raw P7 wire byte: KindVFO, KindMemory, KindMemTune,
	// KindQMB, KindUnset or KindPMS. Reference P7: "kind: 0 VFO, 1 Memory, 2
	// Memory Tune, 3 QMB, 4 \"-\" (documented placeholder — parsers must
	// ACCEPT it as unset, builders reject), 5 PMS". Deliberately a plain
	// byte, not a wrapped type like Mode/CTCSSState/Shift: unlike those,
	// P7's semantics are NOT fully settled even after M5b. The
	// write-direction PAIRING rule IS HW-CONFIRMED 2026-07-13 (M5b write
	// trials, docs/hardware-notes.md) — every MW write, memory or PMS
	// slot alike, must carry KindMemory ('1'); mw.go's validateMWFields
	// enforces exactly this. But P7's full semantics stay partially
	// murky: MEM channels have been observed reading BOTH '0' and '1'
	// (front-panel-created), only '1' is writable, the '0' state is not
	// recreatable via CAT, and '2'/'3'/'4' have never been sent or read
	// on a real radio at all. Given that, this field still gets no
	// Wire()/String() ceremony implying a settled semantics.
	Kind byte

	CTCSS CTCSSState // Reference P8: "CTCSS: 0 off, 1 ENC/DEC, 2 ENC".
	Shift Shift      // Reference P10: "shift: 0 simplex, 1 plus, 2 minus".
}

// Kind byte values for MemoryData.Kind (reference P7).
const (
	KindVFO     byte = '0'
	KindMemory  byte = '1'
	KindMemTune byte = '2'
	KindQMB     byte = '3'

	// KindUnset is the P7 "-" placeholder value. Reference P7 row: "4 \"-\"
	// (documented placeholder — parsers must ACCEPT it as unset, builders
	// reject)". This mirrors ModeUnset's '0' = "-" convention (mode.go):
	// ParseMRAnswer accepts it as a structurally valid kind byte, but
	// BuildMWSet must never emit it — and never can, because its Kind
	// check (mw.go, validateMWFields) only ever accepts KindMemory, for
	// ANY writable slot, so KindUnset is rejected there by construction,
	// with no separate check required.
	KindUnset byte = '4'

	// KindPMS is the P7 value the manual's own worked example implies for
	// a PMS slot's MR answer. HW-CONFIRMED 2026-07-13 (M5b write trials,
	// docs/hardware-notes.md): this project's former ASSUMED write-side
	// pairing (KindPMS on a PMS MW) is REFUTED — the radio REJECTS a PMS
	// write carrying KindPMS, requiring KindMemory instead (see mw.go).
	// KindPMS remains a legal READ-side value: a populated PMS slot may
	// answer MR with EITHER KindMemory (CAT-written, HW-CONFIRMED) or
	// KindPMS (front-panel-created origin — UNKNOWN, never observed at
	// M5b); the driver's read-side leniency accepts both (see
	// core/driver/ft710/read.go's wantKind).
	KindPMS byte = '5'
)

// validKindByte reports whether b is one of the reference's documented P7
// values, INCLUDING KindUnset ('4'): see KindUnset's doc comment for why a
// parser must accept it. Anything else is rejected.
func validKindByte(b byte) bool {
	switch b {
	case KindVFO, KindMemory, KindMemTune, KindQMB, KindUnset, KindPMS:
		return true
	default:
		return false
	}
}

// CTCSSState is the CAT P8 CTCSS field: a single ASCII digit byte, styled
// like Mode's P6 nibble (mode.go) — the underlying byte value IS the wire
// byte. Reference: "CTCSS: 0 off, 1 ENC/DEC, 2 ENC".
type CTCSSState byte

// CTCSSState constants for the 3 states in the reference table.
const (
	CTCSSOff    CTCSSState = '0'
	CTCSSEncDec CTCSSState = '1'
	CTCSSEnc    CTCSSState = '2'
)

// ctcssNames maps every valid CTCSSState to its reference display name.
var ctcssNames = map[CTCSSState]string{
	CTCSSOff:    "off",
	CTCSSEncDec: "ENC/DEC",
	CTCSSEnc:    "ENC",
}

// ParseCTCSSState parses a single P8 wire byte into a CTCSSState. Anything
// other than '0', '1' or '2' is rejected with a *ParseError.
func ParseCTCSSState(c byte) (CTCSSState, error) {
	switch c {
	case '0', '1', '2':
		return CTCSSState(c), nil
	default:
		return 0, newParseError([]byte{c}, "invalid CTCSS code: want '0'-'2'")
	}
}

// Wire returns the single wire byte for c.
func (c CTCSSState) Wire() byte { return byte(c) }

// String returns the reference table's display name for c, or a
// diagnostic placeholder for a value constructed by an invalid cast.
func (c CTCSSState) String() string {
	if name, ok := ctcssNames[c]; ok {
		return name
	}
	return fmt.Sprintf("CTCSSState(%#02x)", byte(c))
}

// Shift is the CAT P10 repeater shift field: a single ASCII digit byte,
// styled like Mode's P6 nibble. Reference: "shift: 0 simplex, 1 plus, 2
// minus".
type Shift byte

// Shift constants for the 3 states in the reference table.
const (
	ShiftSimplex Shift = '0'
	ShiftPlus    Shift = '1'
	ShiftMinus   Shift = '2'
)

// shiftNames maps every valid Shift to its reference display name.
var shiftNames = map[Shift]string{
	ShiftSimplex: "simplex",
	ShiftPlus:    "plus",
	ShiftMinus:   "minus",
}

// ParseShift parses a single P10 wire byte into a Shift. Anything other
// than '0', '1' or '2' is rejected with a *ParseError.
func ParseShift(c byte) (Shift, error) {
	switch c {
	case '0', '1', '2':
		return Shift(c), nil
	default:
		return 0, newParseError([]byte{c}, "invalid shift code: want '0'-'2'")
	}
}

// Wire returns the single wire byte for s.
func (s Shift) Wire() byte { return byte(s) }

// String returns the reference table's display name for s, or a
// diagnostic placeholder for a value constructed by an invalid cast.
func (s Shift) String() string {
	if name, ok := shiftNames[s]; ok {
		return name
	}
	return fmt.Sprintf("Shift(%#02x)", byte(s))
}

// validClarHz reports whether v is a legal clarifier value UNDER THIS
// DIALECT'S ClarifierPolicy (dialectconfig.go): a multiple of d.clar.StepHz
// Hz, with magnitude at most d.clar.MaxAbsHz. Reference P3's "4-digit
// offset 0000-9990 Hz" (magnitude) is the FT-710's OWN figure — see
// FT710's clar literal (dialect.go) — not a package constant any more:
// clarifier_test.go's peer dialect legally builds and admits clarifier
// values (e.g. 9999 Hz, 1 Hz steps) the FT-710 rejects outright.
//
// Promoted from a package-level function reading two former package
// constants (clarMaxAbsHz, clarStepHz) to a Dialect method: THE SEAM M9c-0
// task 65 exists to close. mr.go's parseMemoryFrame and mw.go's
// validateMWFields both reach the OUTBOUND WRITE GATE, so a hardwired
// bound here would authorise, or refuse, bytes on the strength of another
// radio's clarifier policy rather than this dialect's own.
//
// A StepHz below 1 (only reachable via a hand-built Dialect literal that
// bypasses NewDialect's validation, e.g. the zero Dialect) reports false
// rather than dividing by zero — consistent with the zero value's
// documented fail-closed behaviour (dialect.go), not merely an accident of
// avoiding a panic. Go's % operator preserves the sign of the dividend for
// a positive step, so this works correctly for negative v without a
// separate abs() step.
func (d Dialect) validClarHz(v int16) bool {
	if d.clar.StepHz < 1 {
		return false
	}
	max, step := int16(d.clar.MaxAbsHz), int16(d.clar.StepHz)
	if v < -max || v > max {
		return false
	}
	return v%step == 0
}

// validClarHz (the package-level function below) is a compatibility shim
// kept ONLY because memdata_test.go's TestValidClarHz — outside this
// task's file ownership (M9c-0 task 65: memdata.go, mr.go, mw.go,
// clarifier_test.go) — still calls it directly by this name and int16
// signature. No production code reaches this any longer: mr.go and mw.go
// both call the Dialect method above on their own receiver, which is the
// seam this task closes. It delegates to FT710's OWN policy rather than
// restating 9990/10 as a second, independently-drifting literal.
func validClarHz(v int16) bool {
	return FT710.validClarHz(v)
}

// allDigits reports whether every byte in b is an ASCII digit '0'-'9'
// (vacuously true for an empty slice).
func allDigits(b []byte) bool {
	for _, c := range b {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// parseBoolDigit parses a single '0'/'1' wire byte into a bool. Anything
// else is rejected with a *ParseError.
func parseBoolDigit(b byte) (bool, error) {
	switch b {
	case '0':
		return false, nil
	case '1':
		return true, nil
	default:
		return false, newParseError([]byte{b}, "expected '0' or '1'")
	}
}

// boolDigit returns the wire byte for a bool flag: '1' for true, '0' for
// false.
func boolDigit(b bool) byte {
	if b {
		return '1'
	}
	return '0'
}
