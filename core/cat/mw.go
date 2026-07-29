// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import "fmt"

// BuildMWSet builds a strict 28-byte MW (memory channel write) Set frame
// from m. Reference: "MW — MEMORY CHANNEL WRITE (Set only; no Read/Answer)
// ... identical 28-byte layout with MW; P1 restricted to 001-099, P1L-P9U
// (no 5xx, no EMG; 000 listed but semantics unknown — reject in
// builder)". Golden vectors G5, G7.
//
// BuildMWSet rejects, with a *ParseError, any MemoryData that cannot be
// safely represented on the wire: see validateMWFields, which performs
// every check and is shared with AllowedCommand's MW grammar check
// (allowlist.go) so the write-direction policy is expressed in exactly one
// place.
func (d Dialect) BuildMWSet(m MemoryData) (Command, error) {
	if err := d.validateMWFields(m); err != nil {
		return Command{}, err
	}

	// Framing here, field block in encodeMemoryFields (memdata.go): the
	// same offsets 2-26 the combined MT record writes, extracted from this
	// body in M9c-3 task 3 with the golden vectors G5/G7 as the proof that
	// not a byte moved.
	frame := make([]byte, memoryFrameLen)
	frame[0], frame[1] = 'M', 'W'
	encodeMemoryFields(frame, m)
	frame[memTermOffset] = ';'

	return newCommand(frame), nil
}

// validateMWFields applies MW's write-direction policy to m, returning a
// *ParseError describing the first violation found, or nil if m is safe to
// encode as an MW Set frame UNDER THIS DIALECT. It rejects:
//   - a slot that is not writable under this dialect's slot space (5xx,
//     EMG, "000"/none, or an invalid Slot);
//   - a Kind other than THIS DIALECT'S declared write kind
//     (Dialect.mwWriteKind). The FT-710's is KindMemory ('1'),
//     HW-CONFIRMED 2026-07-13: that radio requires it on EVERY MW write
//     regardless of slot bank. Since M9c-0 the value comes from the
//     receiver rather than a constant, because that finding is about one
//     radio and the outbound gate reaches this validator (see the
//     Kind-pairing note below);
//   - m.Mode == ModeUnset, or any Mode value that does not round-trip
//     through ParseMode (Mode is a raw byte alias, mode.go — never trust a
//     caller-forged Mode value, per Task 2 review);
//   - a forged CTCSSState/Shift byte that does not round-trip through
//     their own Parse functions, for the same reason;
//   - a ClarHz that violates THIS DIALECT'S clarifier policy
//     (Dialect.clar): not a multiple of its step, or beyond its range.
//     The FT-710's own policy is 10 Hz steps to +-9990 Hz;
//   - a FreqHz that needs more than 9 digits, or is zero.
//
// This is shared, unchanged, between BuildMWSet (validating a
// caller-constructed MemoryData, which may be entirely forged) and
// AllowedCommand's MW grammar check (allowlist.go), which first decodes a
// raw wire frame via parseMemoryFrame and then runs the SAME policy check
// against the result — so the write-direction rules governing what may
// reach the radio as an MW command live in exactly one place, not two.
//
// SEAM NOTE (Task 54): writability is decided by Dialect.writableSlot
// (slot.go), not by Slot.Writable, and the mode by d.ParseMode, not the
// package-level ParseMode. Both of those package-level forms answer for
// the FT-710 whatever dialect this method was called on, which would make
// the receiver here decorative — and, because this same validator is what
// AllowedCommand's MW check runs, would let a frame legal only under
// another radio's slot space through the outbound write gate.
func (d Dialect) validateMWFields(m MemoryData) error {
	if !d.writableSlot(m.Slot) {
		return newParseError([]byte(m.Slot.Wire()), "MW: slot must be Writable() (memory 001-099 or PMS P1L-P9U; 5xx/EMG/\"000\" rejected)")
	}

	// Kind-on-write pairing, from THIS DIALECT'S policy.
	//
	// THE FT-710's VALUE IS HW-CONFIRMED 2026-07-13 (M5b write trials
	// against Stuart's real UK FT-710 — see docs/hardware-notes.md's M5b
	// findings section). The manual does not document P7's meaning in a
	// Set at all; this project's former ASSUMED pairing (KindMemory '1'
	// for memory slots, KindPMS '5' for PMS slots) is HARDWARE-REFUTED:
	// the radio requires P7 = KindMemory ('1') on EVERY MW write,
	// regardless of slot bank — a PMS write carrying KindPMS ('5') is
	// REJECTED with an immediate "?;" (~10ms), while the identical PMS
	// write carrying KindMemory ('1') is accepted.
	//
	// That evidence is about ONE RADIO, and it is why the value is dialect
	// data rather than a constant. Until M9c-0 this read `m.Kind !=
	// KindMemory`, so every dialect inherited the FT-710's hardware
	// finding — and because the outbound gate reaches this validator
	// through validMWCommand, a second radio with a different P7 rule
	// would have had its legitimate writes refused by this program's own
	// gate. No claim is made that any other radio DOES differ; only that
	// the FT-710's value is the FT-710's.
	//
	// Because d.writableSlot(m.Slot) above already guarantees memory XOR
	// PMS, this single check also structurally rejects every OTHER Kind
	// value for either slot kind — no separate validKindByte call is
	// needed here. NewDialect has already checked that the policy byte is
	// itself a documented P7 value.
	if m.Kind != d.mwWriteKind {
		return newParseError([]byte{m.Kind}, fmt.Sprintf("MW: Kind must be %q for both memory-channel and PMS slots (the FT-710's own value is KindMemory ('1'), HW-CONFIRMED 2026-07-13: PMS writes with KindPMS ('5') are REJECTED by the radio — docs/hardware-notes.md)", d.mwWriteKind))
	}

	// Mode is a raw byte alias (mode.go): never trust a caller-forged
	// value. Re-validate via THIS DIALECT'S ParseMode and separately
	// reject ModeUnset, which parsers must accept but builders must never
	// emit (mode.go doc comment; Task 2 review note).
	validMode, err := d.ParseMode(m.Mode.Wire())
	if err != nil {
		return newParseError([]byte{m.Mode.Wire()}, "MW: mode field (P6) is not a valid Mode")
	}
	if validMode == ModeUnset {
		return newParseError([]byte{m.Mode.Wire()}, "MW: mode field (P6) must not be ModeUnset in a Set frame")
	}

	if !d.validClarHz(m.ClarHz) {
		return newParseError([]byte(fmt.Sprintf("%d", m.ClarHz)), fmt.Sprintf("MW: ClarHz must be a multiple of %d Hz, magnitude <= %d", d.clar.StepHz, d.clar.MaxAbsHz))
	}

	if m.FreqHz == 0 || m.FreqHz > memFreqMax {
		return newParseError([]byte(fmt.Sprintf("%d", m.FreqHz)), "MW: FreqHz must be nonzero and fit in 9 digits (<= 999999999)")
	}

	// CTCSSState/Shift are byte-alias types exactly like Mode: never trust
	// a caller-forged value (e.g. CTCSSState('9')). Re-validate via their
	// own Parse functions for the same reason as the Mode check above.
	if _, err := ParseCTCSSState(m.CTCSS.Wire()); err != nil {
		return newParseError([]byte{m.CTCSS.Wire()}, "MW: CTCSS field (P8) is not a valid CTCSSState")
	}
	if _, err := ParseShift(m.Shift.Wire()); err != nil {
		return newParseError([]byte{m.Shift.Wire()}, "MW: shift field (P10) is not a valid Shift")
	}

	return nil
}
