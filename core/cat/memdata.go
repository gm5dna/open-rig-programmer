// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"fmt"
	"strconv"
)

// memoryFrameLen is the fixed length of an MR-answer/MW-set frame: 28
// bytes. Reference: "MR — MEMORY CHANNEL READ" position table, shared
// byte-for-byte by MW's Set frame ("MW — ... Set frame: identical 28-byte
// layout with MW").
const memoryFrameLen = 28

// Byte offsets (0-indexed) into a 28-byte MR-answer/MW-set frame, shared by
// parseMemoryFields and encodeMemoryFields below (and so by mr.go's parser,
// mw.go's builder and the outbound gate). Comments give the reference's
// 1-indexed position (Pn) for cross-checking against the manual's table.
//
// Offsets 2-26 are the FIELD BLOCK: everything between the two-byte command
// prefix and the frame's last byte. In the 28-byte MR/MW frame the byte
// after the block is the ';' terminator (memTermOffset); a frame form that
// carries more after the block — the combined MT record, M9c-3 task 4 —
// reuses these same offsets and puts its own bytes there instead.
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

	// ClarHz is the clarifier offset in Hz: signed, and constrained by the
	// DIALECT'S OWN clarifier policy (Dialect.clar) — a multiple of its
	// step, within its range. The FT-710's policy is 10 Hz steps to
	// +-9990 Hz, which is what Reference P3 documents ("clarifier: +/- then
	// 4-digit offset 0000-9990 Hz"); a constructed dialect may declare
	// another, and this package's own tests use a 1 Hz/9999 Hz peer. The
	// 4-digit wire field bounds every dialect at 9999.
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
	// trials, docs/hardware-notes.md) FOR THE FT-710 — every MW write to
	// that radio, memory or PMS slot alike, must carry KindMemory ('1').
	// Since M9c-0 that is DIALECT DATA (Dialect.mwWriteKind), not a
	// universal rule: mw.go's validateMWFields enforces whichever value
	// the receiver declares, and the FT-710's value is KindMemory because
	// its hardware says so, not because the grammar requires it. But P7's
	// full semantics stay partially
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
	// BuildMWSet must never emit it — and never can, because NewDialect
	// refuses it as a dialect's MWWriteKind (validMWWriteKindByte, which
	// is deliberately narrower than this read-side predicate), so no
	// constructed dialect can declare it as the value its builder writes.
	// Before M9c-0's milestone review the two domains were the same and a
	// dialect COULD declare it, emitting P7 '4' past its own gate.
	//
	// validateMWFields (mw.go) accepts THIS DIALECT'S OWN configured
	// mwWriteKind for any writable slot — not KindMemory specifically.
	// KindMemory is merely the FT-710's value. KindUnset is rejected there
	// by construction whatever a dialect declares, because NewDialect will
	// not let any dialect declare it in the first place.
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
	// Arithmetic in int, NEVER narrowed to int16.
	//
	// This function took int16(d.clar.StepHz) until the M9c-0 milestone
	// review (finding 1). The StepHz < 1 guard above looks like the
	// defence and is not: the narrowing happened AFTER it, so a policy of
	// StepHz 65536 — which passes that guard, and passed every clause of
	// V10 — became int16(65536) == 0 and executed v % 0. Reproduced as a
	// genuine "integer divide by zero" from a CONSTRUCTOR-APPROVED config,
	// reachable through both BuildMWSet and AllowedCommand, so a caller
	// could panic the outbound write gate.
	//
	// V10 now also bounds StepHz, which closes the same hole at
	// construction. Both fixes are kept: this one makes the arithmetic
	// correct for any value the type permits, rather than correct only for
	// values some other function happened to filter.
	iv, max, step := int(v), d.clar.MaxAbsHz, d.clar.StepHz
	if iv < -max || iv > max {
		return false
	}
	// Go's % preserves the sign of the dividend for a positive step, so
	// this is correct for negative v without a separate abs().
	return iv%step == 0
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

// parseMemoryFields validates and decodes the FIELD BLOCK at offsets 2-26
// of frame — P1-P10, the shared memory record — and nothing else. It makes
// no length check, no prefix check and no terminator check: those are its
// CALLER'S framing, and keeping them out here is the whole point of the
// split (M9c-3 task 3). frame must therefore be long enough to hold the
// block; every caller has already established that with its own length
// check.
//
// wantPrefix is threaded solely for the error text — "MR frame: ...",
// "MW frame: ...", "MT frame: ..." — so that a caller's messages keep
// naming the command the caller was parsing. It is NOT re-checked against
// frame's first two bytes here.
//
// Extracted verbatim from parseMemoryFrame (mr.go), whose body this was
// until M9c-3 task 3, so that the combined MT record (mtcombined.go, task
// 4) can decode the same block from a longer frame without a second copy of
// these ten field rules. parseMemoryFrame's check order — length, prefix,
// terminator, then this — is unchanged and pinned by
// memfields_test.go's TestParseMemoryFrame_DoublyInvalidFrameErrorOrder.
//
// THE RECEIVER MATTERS HERE (Codex plan-review F3, carried over from
// parseMemoryFrame): this decoder is reached from a parser AND from the
// outbound write gate, so every membership decision must be taken against
// THIS DIALECT — d.ParseSlot for P1, d.ParseMode for P6, d.validClarHz for
// P3 — never the package-level delegates, which would answer for the FT-710
// whatever dialect was asked.
//
// The kind byte (P7) is checked against validKindByte only: the documented
// read vocabulary '0'-'5', exactly as before. A caller whose form documents
// a narrower vocabulary narrows it AFTER this returns.
func (d Dialect) parseMemoryFields(frame []byte, wantPrefix string) (MemoryData, error) {
	slot, err := d.ParseSlot(string(frame[memSlotOffset : memSlotOffset+3]))
	if err != nil {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: invalid slot field (P1)", wantPrefix))
	}

	freqField := frame[memFreqOffset : memFreqOffset+memFreqDigits]
	if !allDigits(freqField) {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: frequency field (P2) must be 9 digits", wantPrefix))
	}
	freq, err := strconv.ParseUint(string(freqField), 10, 32)
	if err != nil {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: frequency field (P2) out of range", wantPrefix))
	}

	sign := frame[memClarSignOffset]
	if sign != '+' && sign != '-' {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: clarifier sign (P3) must be '+' or '-'", wantPrefix))
	}
	clarField := frame[memClarMagOffset : memClarMagOffset+memClarMagDigits]
	if !allDigits(clarField) {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: clarifier field (P3) must be 4 digits", wantPrefix))
	}
	clarMag, err := strconv.ParseUint(string(clarField), 10, 16)
	if err != nil {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: clarifier field (P3) out of range", wantPrefix))
	}
	clar := int16(clarMag)
	if sign == '-' {
		clar = -clar
	}
	if !d.validClarHz(clar) {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: clarifier (P3) must be a multiple of %d Hz, magnitude <= %d", wantPrefix, d.clar.StepHz, d.clar.MaxAbsHz))
	}

	rxClar, err := parseBoolDigit(frame[memRxClarOffset])
	if err != nil {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: RX CLAR field (P4) must be '0' or '1'", wantPrefix))
	}
	txClar, err := parseBoolDigit(frame[memTxClarOffset])
	if err != nil {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: TX CLAR field (P5) must be '0' or '1'", wantPrefix))
	}

	mode, err := d.ParseMode(frame[memModeOffset])
	if err != nil {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: mode field (P6) invalid", wantPrefix))
	}

	kind := frame[memKindOffset]
	if !validKindByte(kind) {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: kind field (P7) must be one of '0','1','2','3','4','5'", wantPrefix))
	}

	ctcss, err := ParseCTCSSState(frame[memCTCSSOffset])
	if err != nil {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: CTCSS field (P8) invalid", wantPrefix))
	}

	if string(frame[memP9Offset:memP9Offset+2]) != "00" {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: P9 field must be fixed \"00\"", wantPrefix))
	}

	shift, err := ParseShift(frame[memShiftOffset])
	if err != nil {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: shift field (P10) invalid", wantPrefix))
	}

	return MemoryData{
		Slot:   slot,
		FreqHz: uint32(freq),
		ClarHz: clar,
		RxClar: rxClar,
		TxClar: txClar,
		Mode:   mode,
		Kind:   kind,
		CTCSS:  ctcss,
		Shift:  shift,
	}, nil
}

// encodeMemoryFields writes m into the FIELD BLOCK at offsets 2-26 of
// frame, the mirror of parseMemoryFields. It writes nothing outside that
// block: the two-byte command prefix, and whatever the form puts after the
// block (the ';' terminator of an MR/MW frame; P11, the tag field and the
// terminator of a combined MT record), belong to the caller.
//
// frame is CALLER-SIZED and must already be at least memShiftOffset+1 bytes
// long; every caller allocates its own form's exact length. m must already
// have passed that caller's write-direction validation — this function
// encodes, it does not judge. (Both callers validate first: BuildMWSet via
// validateMWFields, and the combined MT builder via
// validateCombinedMTFields.)
//
// Extracted verbatim from BuildMWSet's body (mw.go) in M9c-3 task 3, which
// is why the byte-identity of the MW golden vectors G5/G7 is this
// extraction's proof.
//
// No dialect receiver: unlike the decoder, encoding a validated MemoryData
// involves no membership decision — every byte written here comes from m,
// via the same Wire() accessors the caller's validator already round-tripped
// them through.
func encodeMemoryFields(frame []byte, m MemoryData) {
	copy(frame[memSlotOffset:], m.Slot.Wire())
	copy(frame[memFreqOffset:], fmt.Sprintf("%0*d", memFreqDigits, m.FreqHz))

	clarMag := m.ClarHz
	sign := byte('+')
	if clarMag < 0 {
		sign = '-'
		clarMag = -clarMag
	}
	frame[memClarSignOffset] = sign
	copy(frame[memClarMagOffset:], fmt.Sprintf("%0*d", memClarMagDigits, clarMag))

	frame[memRxClarOffset] = boolDigit(m.RxClar)
	frame[memTxClarOffset] = boolDigit(m.TxClar)
	frame[memModeOffset] = m.Mode.Wire()
	frame[memKindOffset] = m.Kind
	frame[memCTCSSOffset] = m.CTCSS.Wire()
	copy(frame[memP9Offset:], "00")
	frame[memShiftOffset] = m.Shift.Wire()
}
