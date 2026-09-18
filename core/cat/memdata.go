// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"fmt"
	"strconv"
)

// memoryFrameLen is the fixed length of the REGISTERED family's
// MR-answer/MW-set frame: 28 bytes, a 9-digit P2. Reference: "MR — MEMORY
// CHANNEL READ" position table, shared byte-for-byte by MW's Set frame
// ("MW — ... Set frame: identical 28-byte layout with MW").
//
// KEPT AS A PACKAGE CONSTANT for fixture code that addresses this one
// canonical (28-byte, 9-digit) shape directly. Since the S1 lift, PRODUCTION
// parsing/building/framing no longer consults it: mr.go's parseMemoryFrame
// and mw.go's BuildMWSet read Dialect.memoryFrameLen instead, because the
// ft2000 family's frame is 27 bytes (an 8-digit P2, DialectConfig.
// MemoryFrameLen/MemoryFreqDigits) and a hardwired constant would size
// every dialect's frame off the FT-710's own shape. Every registered
// dialect declares MemoryFrameLen 28, so this constant and
// Dialect.memoryFrameLen always agree for them, which is exactly why this
// value still equals 28: nothing about a registered dialect's bytes moved.
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
//
// KEPT AS PACKAGE CONSTANTS for the same reason as memoryFrameLen above:
// fixture code exercising only the registered (9-digit-frequency,
// 3-digit-slot) shape still addresses the field block at these fixed,
// familiar positions. Since the S1 lift, PRODUCTION parsing/building
// consult the mem*Off METHODS below instead, which derive every offset
// from THIS DIALECT'S OWN memoryFreqDigits AND slotDigits() —
// memSlotOffset never moves (P1 sits at the very start of the field
// block), but since the FTX-1 seam P2 SLIDES with slotDigits() too
// (memFreqOff, below), and everything from P3 onward slides by however
// many bytes narrower or wider the SLOT and the FREQUENCY fields are than
// 3 and 9 digits — a pure position slide and not a reordering (ft2000
// matrix §1.1, extended to the slot axis by the FTX-1 seam). Under
// slotDigits() 3 and memoryFreqDigits 9 — every registered dialect's own
// declared values — each method returns exactly the constant of the same
// name.
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
	memClarMagDigits = 4
)

// memFreqOff is THIS DIALECT'S OWN offset for P2, the field the package
// constant memFreqOffset names for the registered (3-digit-slot) shape.
// Since the FTX-1 seam it SLIDES with slotDigits() exactly as everything
// from P3 onward already slid with memoryFreqDigits (this file's own
// offset-constants doc comment): memSlotOffset never moves — P1 sits before
// the digit width that varies, at the start of the field block — but P2 now
// sits after memSlotOffset PLUS this dialect's own slot width, not the
// package constant's fixed 3. Under slotDigits() 3 (every dialect
// registered before this seam) this returns exactly memFreqOffset, so no
// registered dialect's bytes moved.
//
// Codex spec review BLOCKER 1: before this existed, parseMemoryFields and
// encodeMemoryFields both read/wrote the P2 field at the package constant
// memFreqOffset regardless of slot width — for a 5-digit dialect a READ
// would start two bytes into the slot field's own last two digits, and an
// ENCODE would clobber them, because encodeMemoryFields writes the slot via
// m.Slot.Wire() (5 bytes) then the frequency at the OLD fixed offset 5,
// three bytes short of where the 5-byte slot field actually ends.
func (d Dialect) memFreqOff() int { return memSlotOffset + d.slotDigits() }

// memClarSignOff, memClarMagOff, memRxClarOff, memTxClarOff, memModeOff,
// memKindOff, memCTCSSOff, memP9Off, memShiftOff and memTermOff are THIS
// DIALECT'S OWN offsets for the field block positions the same-named
// constants above name for the registered (9-digit) shape. See this file's
// offset-constants doc comment for why both exist.
func (d Dialect) memClarSignOff() int { return d.memFreqOff() + int(d.memoryFreqDigits) }
func (d Dialect) memClarMagOff() int  { return d.memClarSignOff() + 1 }
func (d Dialect) memRxClarOff() int   { return d.memClarMagOff() + memClarMagDigits }
func (d Dialect) memTxClarOff() int   { return d.memRxClarOff() + 1 }
func (d Dialect) memModeOff() int     { return d.memTxClarOff() + 1 }
func (d Dialect) memKindOff() int     { return d.memModeOff() + 1 }
func (d Dialect) memCTCSSOff() int    { return d.memKindOff() + 1 }
func (d Dialect) memP9Off() int       { return d.memCTCSSOff() + 1 }
func (d Dialect) memShiftOff() int    { return d.memP9Off() + 2 }
func (d Dialect) memTermOff() int     { return d.memShiftOff() + 1 }

// memoryFrameLenFor returns the MR-answer/MW-set frame length a P2 field
// freqDigits digits wide and a slot field slotDigits bytes wide IMPLY:
// memTermOff()+1, computed on a throwaway Dialect carrying only those two
// axes, so this is the SAME offset chain above rather than a second,
// hand-derived formula that could drift from it.
//
// Codex close-review finding P2: before this existed, V17
// (dialectvalidate.go) checked MemoryFrameLen and MemoryFreqDigits each
// against zero but never against EACH OTHER, so a config could declare an
// unmatched pair (e.g. MemoryFrameLen 27 with MemoryFreqDigits 9) — every
// mem*Off method above is anchored to memoryFreqDigits alone, so BuildMWSet
// would then index a 27-byte frame at offsets computed for 9 digits, one
// byte past the end, and panic rather than build or refuse.
//
// THE slotDigits PARAMETER IS THE FTX-1 SEAM'S OWN ADDITION to this same
// finding: memFreqOff() now slides with slot width too (its own doc
// comment), so a config's declared MemoryFrameLen must agree with BOTH axes
// together, not freqDigits alone — a 5-digit-slot dialect declaring the
// registered 28-byte frame would size BuildMWSet's allocation two bytes
// short of what its own slot width needs.
func memoryFrameLenFor(freqDigits uint8, slotDigits int) int {
	d := Dialect{memoryFreqDigits: freqDigits, slots: slotSpace{slotDigits: slotDigits}}
	return d.memTermOff() + 1
}

// memFreqMax is the largest FreqHz value that fits the REGISTERED family's
// 9-digit P2 field. Kept as the package-level ceiling MemoryFreqHz enforces
// (a generic, pre-dialect conversion — see its own doc comment) and as the
// widest bound any dialect's own field could ever need; the OUTBOUND WRITE
// GATE (validateSetFields, mw.go) enforces the narrower, per-dialect bound
// instead — see memFreqDigitsMax.
const memFreqMax = 999_999_999

// memFreqDigitsMax returns the largest FreqHz value THIS DIALECT'S OWN P2
// field can hold: 10^memoryFreqDigits - 1. The registered family's 9-digit
// field allows memFreqMax (999,999,999); the ft2000/ftdx9000 family's
// 8-digit field allows one digit fewer, 99,999,999 (Lift Y's
// MemoryFreqDigits axis).
//
// Codex close-review finding P1: encodeMemoryFields' "%0*d" verb pads to
// AT LEAST memoryFreqDigits digits, never truncates — a FreqHz needing
// more digits than this dialect's field would overflow into the clarifier
// sign byte that follows, corrupting the frame rather than refusing it.
// This bound is what validateSetFields (mw.go) checks BEFORE that encode
// ever runs, so no caller can reach it with a value its own dialect's
// field cannot hold.
//
// A memoryFreqDigits of 0 (only reachable via a hand-built Dialect that
// bypasses NewDialect's V17, e.g. the zero Dialect) reports 0 rather than
// underflowing the uint32 subtraction below — the same fail-closed shape
// validClarHz's StepHz guard already has for its own zero axis.
func (d Dialect) memFreqDigitsMax() uint32 {
	if d.memoryFreqDigits == 0 {
		return 0
	}
	max := uint32(1)
	for i := uint8(0); i < d.memoryFreqDigits; i++ {
		max *= 10
	}
	return max - 1
}

// MemoryData is the fully decoded content of an MR-answer or MW-set frame:
// one memory/PMS/5xx/EMG channel's frequency, clarifier, mode and related
// state. Reference: "MR — MEMORY CHANNEL READ" position table (P1-P10),
// shared byte-for-byte with MW's Set frame.
//
// SINCE M9c-3 IT ALSO FEEDS THE COMBINED MT RECORD (mtcombined.go), which
// carries this same P1-P10 block at these same offsets and then adds P11
// and a fixed-width tag field. So a MemoryData is not "an MR/MW frame's
// contents" so much as one channel's state in whichever frame a dialect's
// form declares — and the write-direction rules that judge it before it is
// encoded are per-command, not per-type: validateMWFields (mw.go) and
// validateCombinedMTFields (mtcombined.go) differ on exactly two of them,
// the slot predicate and the P7 kind.
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

	// ToneIndex is the raw P9 wire value UNDER THIS DIALECT'S OWN READING
	// (MemoryP9Policy, dialectconfig.go). It carries no meaning under
	// P9Fixed00 — every registered dialect's reading, where the two-byte
	// field is printed-fixed "00" — the same "encodes, does not judge,
	// except this one default" shape memoryP5 already has. Under
	// P9ToneIndex (the ft2000 family) it is a two-digit index, 0-49, into
	// the standard 50-entry CTCSS tone chart the manual reprints on MR/MT/
	// MW/OI alike ("CTCSS Tone Chart", ft2000 matrix §1.3). This package
	// only makes the value round-trip: mapping it to a spec.Field grading
	// is a Phase-3 driver decision, not this lift's.
	ToneIndex uint8

	Shift Shift // Reference P10: "shift: 0 simplex, 1 plus, 2 minus".
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

// CTCSSState constants for the 3 states in the FT-710's reference table,
// and the TWO MORE the FT-991A's P8 legend prints.
//
// WHICH OF THEM A GIVEN RADIO HAS IS DIALECT DATA (DialectConfig.
// ToneStates): the FT-991A prints five on all five blocks that carry P8,
// where every registered sibling prints "0/1/2" only. The constants are
// declared here for both, because a byte alias's members are the wire
// vocabulary this codec can express; whether a particular dialect may
// EMIT one is Dialect.ParseCTCSSState's question, not this list's.
const (
	CTCSSOff    CTCSSState = '0'
	CTCSSEncDec CTCSSState = '1'
	CTCSSEnc    CTCSSState = '2'
	// CTCSSDCSEncDec and CTCSSDCSEnc are the FT-991A's "3: DCS ENC/DEC"
	// and "4: DCS ENC". They carry the STATE only: the DCS code itself is
	// not in this record — that radio's CN command carries it — so nothing
	// here implies a code can be read or written.
	CTCSSDCSEncDec CTCSSState = '3'
	CTCSSDCSEnc    CTCSSState = '4'
)

// ctcssNames maps every valid CTCSSState to its reference display name.
var ctcssNames = map[CTCSSState]string{
	CTCSSOff:       "off",
	CTCSSEncDec:    "ENC/DEC",
	CTCSSEnc:       "ENC",
	CTCSSDCSEncDec: "DCS ENC/DEC",
	CTCSSDCSEnc:    "DCS ENC",
}

// ParseCTCSSState parses a single P8 wire byte into a CTCSSState. Anything
// other than '0', '1' or '2' is rejected with a *ParseError.
//
// IT IS THE LEGACY THREE-STATE DOMAIN, DELIBERATELY UNCHANGED, and it is
// retained rather than widened or deleted. Its refusal text is pinned four
// times in core/cat/testdata/parser-corpus.golden, and it has a consumer
// outside this package (core/cat/dialecttest) as well as core/transport's
// tests, so removing it would be a public API break inconsistent with a
// MINOR release. The PER-RADIO domain travels on Dialect.ParseCTCSSState
// instead, which is what every codec and gate site in this package
// consults. TestParseCTCSSState_PackageFunctionIsUNCHANGED holds this one.
func ParseCTCSSState(c byte) (CTCSSState, error) {
	switch c {
	case '0', '1', '2':
		return CTCSSState(c), nil
	default:
		return 0, newParseError([]byte{c}, "invalid CTCSS code: want '0'-'2'")
	}
}

// ParseCTCSSState parses a single P8 wire byte UNDER THIS DIALECT'S
// declared state domain: '0'-'2' under ToneStatesCTCSS, '0'-'4' under
// ToneStatesCTCSSAndDCS.
//
// EVERY CODEC AND GATE SITE IN THIS PACKAGE GOES THROUGH IT, and there are
// three: parseMemoryFields (the parse), validateMWFields and
// validateCombinedMTFields (the two halves of the OUTBOUND WRITE GATE,
// which the builders and AllowedCommand share). Widening only the parse
// site would have let a MemoryData{CTCSS: CTCSSState('3')} be built,
// admitted and SENT to an FTdx10, FTdx101D/MP, FT-891 or FT-710, whose
// manuals print P8 0/1/2 only.
//
// Under ToneStatesCTCSS the behaviour AND THE TEXT are the package
// function's, byte for byte, so nothing a three-state dialect refuses
// reads any differently than it did. The zero Dialect declares no domain
// and accepts nothing, consistent with the rest of this type.
func (d Dialect) ParseCTCSSState(c byte) (CTCSSState, error) {
	switch d.toneStates {
	case ToneStatesCTCSSAndDCS:
		switch c {
		case '0', '1', '2', '3', '4':
			return CTCSSState(c), nil
		default:
			return 0, newParseError([]byte{c}, "invalid CTCSS code: want '0'-'4'")
		}
	case ToneStatesSix:
		// The FTX-1's own six-value P8 domain (ToneStatesSix's own doc
		// comment, dialectconfig.go): '0'-'5'. Bytes '4' and '5' carry no
		// named CTCSSState constant — CTCSSState(c) round-trips them
		// without one.
		switch c {
		case '0', '1', '2', '3', '4', '5':
			return CTCSSState(c), nil
		default:
			return 0, newParseError([]byte{c}, "invalid CTCSS code: want '0'-'5'")
		}
	case ToneStatesCTCSS:
		return ParseCTCSSState(c)
	default:
		// An omitted config semantic refuses rather than defaults. V16
		// keeps every constructed dialect out of this branch; the zero
		// Dialect reaches it, and answering "the FT-710's domain" for a
		// receiver describing no radio is exactly the seam defect this
		// package exists to prevent.
		return 0, newParseError([]byte{c}, "invalid CTCSS code: this dialect declares no P8 state domain")
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

// CTCSSStateString names c under d's declared tone-state domain, for
// diagnostics that need a per-dialect label rather than c.String()'s
// shared one. Every domain but ToneStatesSix defers to c.String()
// unchanged. Under ToneStatesSix, bytes '4'/'5' carry no named
// CTCSSState constant — dialectconfig.go's ToneStatesSix doc comment
// explains why: '4' already names the FT-991A's CTCSSDCSEnc in the
// shared ctcssNames table, and a second name for the same byte would be
// a duplicate map key — so they are named here instead, straight from
// the FTX-1's own P8 legend ("4: PR FREQ", "5: REV TONE", spec.md §6),
// rather than falling back to that FT-991A label.
func (d Dialect) CTCSSStateString(c CTCSSState) string {
	if d.toneStates == ToneStatesSix {
		switch c {
		case CTCSSState('4'):
			return "PR FREQ"
		case CTCSSState('5'):
			return "REV TONE"
		}
	}
	return c.String()
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
	slot, err := d.ParseSlot(string(frame[memSlotOffset : memSlotOffset+d.slotDigits()]))
	if err != nil {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: invalid slot field (P1)", wantPrefix))
	}

	freqField := frame[d.memFreqOff() : d.memFreqOff()+int(d.memoryFreqDigits)]
	if !allDigits(freqField) {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: frequency field (P2) must be %d digits", wantPrefix, d.memoryFreqDigits))
	}
	freq, err := strconv.ParseUint(string(freqField), 10, 32)
	if err != nil {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: frequency field (P2) out of range", wantPrefix))
	}

	sign := frame[d.memClarSignOff()]
	if sign != '+' && sign != '-' {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: clarifier sign (P3) must be '+' or '-'", wantPrefix))
	}
	clarField := frame[d.memClarMagOff() : d.memClarMagOff()+memClarMagDigits]
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

	rxClar, err := parseBoolDigit(frame[d.memRxClarOff()])
	if err != nil {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: RX CLAR field (P4) must be '0' or '1'", wantPrefix))
	}
	// P5, BY THIS DIALECT'S OWN READING (MemoryP5Policy, dialectconfig.go).
	// Under P5TxClar the byte is the TX clarifier flag and this is the
	// pre-existing '0'/'1' rule, unchanged. Under P5Fixed the radio's own
	// manual prints the byte "(Fixed)" on every memory-bearing block, so a
	// '1' is an undocumented frame and is refused rather than decoded into a
	// flag — the same treatment P9 gets, and the combined form's P11 under
	// P11Fixed. Turning an undocumented byte into data is what this package
	// refuses to do.
	//
	// A SWITCH, not an if/else with a wide else arm: NewDialect's V14
	// (dialectvalidate.go) already refuses a zero MemoryP5Policy at
	// construction, but this package's stated posture is that an omitted
	// config semantic refuses rather than defaults, and the default branch
	// below is what makes that hold even in the one place V14 cannot reach.
	var txClar bool
	switch d.memoryP5 {
	case P5Fixed:
		if frame[d.memTxClarOff()] != '0' {
			return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: P5 (position 21) must be fixed '0' under %v — this dialect's manual prints the byte \"(Fixed)\", so it carries no TX clarifier state to decode", wantPrefix, d.memoryP5))
		}
	case P5TxClar:
		var err error
		txClar, err = parseBoolDigit(frame[d.memTxClarOff()])
		if err != nil {
			return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: TX CLAR field (P5) must be '0' or '1'", wantPrefix))
		}
	default:
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: P5 (position 21) policy unset — refusing to guess whether the byte is fixed schema or the TX clarifier flag", wantPrefix))
	}

	mode, err := d.ParseMode(frame[d.memModeOff()])
	if err != nil {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: mode field (P6) invalid", wantPrefix))
	}

	kind := frame[d.memKindOff()]
	if !validKindByte(kind) {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: kind field (P7) must be one of '0','1','2','3','4','5'", wantPrefix))
	}

	// P8, BY THIS DIALECT'S OWN DOMAIN (S0.3): three CTCSS states, or
	// those plus the two DCS ones the FT-991A's legend prints. Through the
	// RECEIVER, never the package function, which is the legacy three-state
	// domain and is retained for its external callers only.
	ctcss, err := d.ParseCTCSSState(frame[d.memCTCSSOff()])
	if err != nil {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: CTCSS field (P8) invalid", wantPrefix))
	}

	// P9, BY THIS DIALECT'S OWN READING (MemoryP9Policy, dialectconfig.go),
	// copied verbatim from P5's shape above. Under P9Fixed00 the field is
	// printed-fixed and a value other than "00" is an undocumented frame,
	// refused rather than decoded. Under P9ToneIndex or P9ToneIndexReadOnly
	// it is a two-digit index into the standard 50-entry CTCSS tone chart —
	// the two share a read side, and only the write side (encode, plus the
	// builder refusal in validateSetFields) tells them apart.
	//
	// A SWITCH, not an if/else with a wide else arm, for the same reason as
	// P5's: NewDialect's V18 already refuses a zero MemoryP9Policy at
	// construction, but the default branch below is what makes that hold
	// even in the one place V18 cannot reach.
	var toneIndex uint8
	p9Field := frame[d.memP9Off() : d.memP9Off()+2]
	switch d.memoryP9 {
	case P9Fixed00:
		if string(p9Field) != "00" {
			return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: P9 field must be fixed \"00\" under %v", wantPrefix, d.memoryP9))
		}
	case P9ToneIndex, P9ToneIndexReadOnly:
		if !allDigits(p9Field) {
			return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: P9 field (tone index) must be 2 digits", wantPrefix))
		}
		idx, err := strconv.ParseUint(string(p9Field), 10, 8)
		if err != nil || idx > 49 {
			return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: P9 tone index must be 00-49", wantPrefix))
		}
		toneIndex = uint8(idx)
	default:
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: P9 (positions 25-26) policy unset — refusing to guess whether the field is fixed schema or a tone-table index", wantPrefix))
	}

	shift, err := ParseShift(frame[d.memShiftOff()])
	if err != nil {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame: shift field (P10) invalid", wantPrefix))
	}

	return MemoryData{
		Slot:      slot,
		FreqHz:    uint32(freq),
		ClarHz:    clar,
		RxClar:    rxClar,
		TxClar:    txClar,
		Mode:      mode,
		Kind:      kind,
		CTCSS:     ctcss,
		ToneIndex: toneIndex,
		Shift:     shift,
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
// encodes, it does not judge, EXCEPT for the P5 default below, which is
// defense-in-depth rather than a rule this function enforces on m's other
// fields. (Both callers validate first: BuildMWSet via validateMWFields, and
// the combined MT builder via validateCombinedMTFields.)
//
// Extracted verbatim from BuildMWSet's body (mw.go) in M9c-3 task 3, which
// is why the byte-identity of the MW golden vectors G5/G7 is this
// extraction's proof.
//
// IT TOOK NO DIALECT RECEIVER UNTIL STAGE 0, on the reasoning that encoding
// a validated MemoryData involves no membership decision — every byte
// written here comes from m, via the same Wire() accessors the caller's
// validator already round-tripped them through. That held while every field
// meant the same thing on every radio. P5 does not: MemoryP5Policy decides
// whether byte 21 is the TX clarifier flag or a printed-fixed '0', and the
// M9b lesson is that a bound must be consulted from the same place as its
// datum. So it is a method, and the policy comes off the receiver rather
// than a package global.
//
// Under P5TxClar this writes exactly what it always wrote. The returned
// error is always nil there and under P5Fixed — a zero MemoryP5Policy is the
// only case that produces one, and NewDialect's V14 already keeps every
// registered dialect from reaching this function with one; see
// TestMemoryP5_ZeroPolicyRefusesRatherThanDefaultingWide.
func (d Dialect) encodeMemoryFields(frame []byte, m MemoryData) error {
	copy(frame[memSlotOffset:], m.Slot.Wire())
	copy(frame[d.memFreqOff():], fmt.Sprintf("%0*d", int(d.memoryFreqDigits), m.FreqHz))

	clarMag := m.ClarHz
	sign := byte('+')
	if clarMag < 0 {
		sign = '-'
		clarMag = -clarMag
	}
	frame[d.memClarSignOff()] = sign
	copy(frame[d.memClarMagOff():], fmt.Sprintf("%0*d", memClarMagDigits, clarMag))

	frame[d.memRxClarOff()] = boolDigit(m.RxClar)
	// P5. Under P5Fixed the byte is schema, not state, and both callers have
	// already REFUSED a record carrying TxClar true (validateMWFields,
	// validateCombinedMTFields) — so this is not a silent correction of a
	// flag the caller asked for, it is the encoding of a byte that has only
	// one legal value on such a radio.
	//
	// A SWITCH, not an if/else with a wide else arm: see parseMemoryFields'
	// matching comment above. The default here can only fire if a caller
	// reaches this function with an unvalidated Dialect, which is why it
	// exists alongside, not instead of, validateMWFields'/
	// validateCombinedMTFields' own refusal.
	switch d.memoryP5 {
	case P5Fixed:
		frame[d.memTxClarOff()] = '0'
	case P5TxClar:
		frame[d.memTxClarOff()] = boolDigit(m.TxClar)
	default:
		return newParseError(frame, "P5 (position 21) policy unset — refusing to guess whether the byte is fixed schema or the TX clarifier flag")
	}
	frame[d.memModeOff()] = m.Mode.Wire()
	frame[d.memKindOff()] = m.Kind
	frame[d.memCTCSSOff()] = m.CTCSS.Wire()
	// P9, BY THIS DIALECT'S OWN READING, copied verbatim from P5's shape
	// above: under P9Fixed00 OR P9ToneIndexReadOnly the field is schema on
	// this (write) side, and both callers have already REFUSED a record
	// carrying a nonzero ToneIndex (validateSetFields, mw.go); under
	// P9ToneIndex it is the two-digit tone-table index.
	switch d.memoryP9 {
	case P9Fixed00, P9ToneIndexReadOnly:
		copy(frame[d.memP9Off():], "00")
	case P9ToneIndex:
		copy(frame[d.memP9Off():], fmt.Sprintf("%02d", m.ToneIndex))
	default:
		return newParseError(frame, "P9 (positions 25-26) policy unset — refusing to guess whether the field is fixed schema or a tone-table index")
	}
	frame[d.memShiftOff()] = m.Shift.Wire()
	return nil
}

// MemoryFreqHz converts a neutral-model frequency into the uint32
// MemoryData.FreqHz carries, refusing anything this protocol's 9-digit
// frequency field could not hold.
//
// It is the ONE conversion point between the widened neutral model and
// this package, and it is a function rather than a cast on purpose: a
// bare uint32(v) would TRUNCATE a 10 GHz frequency into a plausible-
// looking small one and send it to a radio, which is precisely the class
// of silent corruption this project refuses. Every driver that builds a
// MemoryData from a codeplug.ChannelData calls this and propagates the
// error.
//
// For the six Yaesu NEWCAT models registered today the error arm is
// UNREACHABLE in practice — codeplug.Validate has already rejected any
// frequency above those radios' own declared ceilings, the highest of them
// the FT-991A's 470 MHz, and the write path refuses a channel Validate
// rejected — so this is defence in depth at a type boundary, tested
// directly rather than left to be discovered. THE CEILING IS PER MODEL AND
// NOT THE FAMILY'S: it read "75 MHz" while every registered model was HF,
// and the sixth is the first with VHF/UHF.
func MemoryFreqHz(v uint64) (uint32, error) {
	if v > memFreqMax {
		return 0, fmt.Errorf("cat: frequency %d Hz is too large for this protocol's memory frame (maximum %d Hz)", v, memFreqMax)
	}
	return uint32(v), nil
}
