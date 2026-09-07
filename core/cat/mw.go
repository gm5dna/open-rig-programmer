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
	if err := d.encodeMemoryFields(frame, m); err != nil {
		return Command{}, err
	}
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
// (slot.go) and the mode by d.ParseMode — both on the RECEIVER. The
// MemoryData reaching here is caller-supplied and may be forged whole, so
// its Slot may have been built under ANOTHER dialect; consulting anything
// carried by the value rather than asking the receiver would make the
// receiver here decorative — and, because this same validator is what
// AllowedCommand's MW check runs, would let a frame legal only under
// another radio's slot space through the outbound write gate.
//
// There is no longer a value-form Slot.Writable to reach for by mistake:
// M9d removed it precisely because it sat one import from this gate
// answering for a different dialect. The note is kept because the HAZARD
// is not gone, only that one instance of it — the mode leg still has a
// value-shaped temptation, and the next datum promoted into Dialect will
// too.
func (d Dialect) validateMWFields(m MemoryData) error {
	// THE SLOT REFUSAL WORDING IS FROZEN, and still says "Writable()"
	// although M9d removed that method. The FT-710's render of it is
	// baked into six lines of core/cat/testdata/frame-corpus.golden, one
	// of the twenty paths the milestone golden gate forbids moving
	// (twenty-five once the FT-991A's own artefacts join them); rewording
	// it here would move that golden. Harmless in practice — the
	// parenthetical spells the rule out in full, so the message stands
	// alone without the symbol — but it is frozen deliberately, not by
	// oversight. Reword it in a change that is ALLOWED to regenerate the
	// frame corpus, never on its own.
	//
	// WHAT S0.2 CHANGED IS WHERE THE BYTES COME FROM, not what they are.
	// The sentence is composed by Dialect.mwSlotDomainRefusal (slot.go)
	// from this dialect's own memory range, PMS domain, declared special
	// banks and none form, because all four were literals true only of
	// the token-PMS radios: the FT-991A's pairs are 100-117 and it has
	// neither a 5 MHz bank nor an emergency channel. All four token
	// dialects render byte-for-byte what stood here, so the corpus does
	// not move — proved directly by
	// TestSlotDomainText_FT710SentencesAreByteIdentical, on the OTHER
	// FOUR registered dialects by
	// TestSlotDomainRefusals_EveryTokenPMSDialectIsByteIdentical
	// (core/transport, which can import them where this package cannot),
	// and, on the whole corpus, by the golden itself.
	//
	// KIND-ON-WRITE PAIRING is THIS DIALECT'S policy.
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
	// Because writableSlot above already guarantees memory XOR PMS, this
	// single check also structurally rejects every OTHER Kind value for
	// either slot kind — no separate validKindByte call is needed here.
	// NewDialect has already checked that the policy byte is itself a
	// documented P7 value.
	return d.validateSetFields(m, "MW", d.writableSlot, d.mwSlotDomainRefusal(),
		d.mwWriteKind, fmt.Sprintf("MW: Kind must be %q for both memory-channel and PMS slots (the FT-710's own value is KindMemory ('1'), HW-CONFIRMED 2026-07-13: PMS writes with KindPMS ('5') are REJECTED by the radio — docs/hardware-notes.md)", d.mwWriteKind))
}

// validateSetFields is validateMWFields' and validateCombinedMTFields'
// shared checklist — slot writability, the fixed P7 kind byte, Mode,
// ClarHz, FreqHz, P5 and CTCSSState/Shift — with only the SLOT PREDICATE
// (slotOK), its refusal wording (slotRefusal), the WANTED KIND BYTE
// (wantKind) and its refusal wording (kindRefusal) varying between MW and
// combined-form MT. prefix ("MW" or "MT") names the command in every
// other message.
//
// Everything below is MW's rule, for MW's reason, restated for whichever
// command called it: Mode, CTCSSState and Shift are byte-alias types, so
// a caller-forged value must be re-validated through this dialect's own
// ParseMode and through ParseCTCSSState/ParseShift; ModeUnset is
// separately refused, because parsers must accept the '-' placeholder and
// builders must never emit it; the clarifier is bounded by THIS DIALECT'S
// policy; and the frequency must be nonzero as well as fitting the
// 9-digit field.
//
// slotRefusal and kindRefusal are passed in fully composed, never built
// here: mw.go's slot refusal is FROZEN (golden-pinned, see
// validateMWFields), and a shared body that assembled the wording itself
// would put every caller's frozen message one edit away from moving it.
func (d Dialect) validateSetFields(m MemoryData, prefix string, slotOK func(Slot) bool, slotRefusal string, wantKind byte, kindRefusal string) error {
	if !slotOK(m.Slot) {
		return newParseError([]byte(m.Slot.Wire()), slotRefusal)
	}

	if m.Kind != wantKind {
		return newParseError([]byte{m.Kind}, kindRefusal)
	}

	// Mode is a raw byte alias (mode.go): never trust a caller-forged
	// value. Re-validate via THIS DIALECT'S ParseMode and separately
	// reject ModeUnset, which parsers must accept but builders must never
	// emit (mode.go doc comment; Task 2 review note).
	validMode, err := d.ParseMode(m.Mode.Wire())
	if err != nil {
		return newParseError([]byte{m.Mode.Wire()}, prefix+": mode field (P6) is not a valid Mode")
	}
	if validMode == ModeUnset {
		return newParseError([]byte{m.Mode.Wire()}, prefix+": mode field (P6) must not be ModeUnset in a Set frame")
	}

	if !d.validClarHz(m.ClarHz) {
		return newParseError([]byte(fmt.Sprintf("%d", m.ClarHz)), fmt.Sprintf("%s: ClarHz must be a multiple of %d Hz, magnitude <= %d", prefix, d.clar.StepHz, d.clar.MaxAbsHz))
	}

	if m.FreqHz == 0 || m.FreqHz > memFreqMax {
		return newParseError([]byte(fmt.Sprintf("%d", m.FreqHz)), prefix+": FreqHz must be nonzero and fit in 9 digits (<= 999999999)")
	}

	// P5, BY THIS DIALECT'S OWN READING. Under P5Fixed byte 21 is printed
	// "(Fixed)" on this radio's memory blocks, so a record asking for the TX
	// clarifier is REFUSED rather than quietly encoded as '0': a caller that
	// believed it was writing the clarifier finds out, which is the
	// validate-don't-rewrite posture this validator takes with every other
	// field. This is the WRITE-direction check on a caller-supplied
	// MemoryData; a FORGED wire frame is instead refused earlier, by
	// parseMemoryFields (memdata.go), which the gate's grammar check
	// reaches before this validator ever runs.
	//
	// A SWITCH, not an if with an implicit "everything else passes" arm: see
	// parseMemoryFields' matching comment (memdata.go) — an omitted config
	// semantic refuses rather than defaults, and NewDialect's V14 already
	// keeps every registered dialect from reaching the default case.
	switch d.memoryP5 {
	case P5Fixed:
		if m.TxClar {
			return newParseError([]byte{boolDigit(m.TxClar)}, fmt.Sprintf("%s: TxClar must be false under %v — this dialect's manual prints P5 (position 21) \"(Fixed)\", so there is no TX clarifier flag to set", prefix, d.memoryP5))
		}
	case P5TxClar:
	default:
		return newParseError(nil, prefix+": P5 (position 21) policy unset — refusing to guess whether the byte is fixed schema or the TX clarifier flag")
	}

	// CTCSSState/Shift are byte-alias types exactly like Mode: never trust
	// a caller-forged value (e.g. CTCSSState('9')). Re-validate via their
	// own Parse functions for the same reason as the Mode check above.
	if _, err := d.ParseCTCSSState(m.CTCSS.Wire()); err != nil {
		return newParseError([]byte{m.CTCSS.Wire()}, prefix+": CTCSS field (P8) is not a valid CTCSSState")
	}
	if _, err := ParseShift(m.Shift.Wire()); err != nil {
		return newParseError([]byte{m.Shift.Wire()}, prefix+": shift field (P10) is not a valid Shift")
	}

	return nil
}
