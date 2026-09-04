// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"fmt"
	"strings"
)

// CombinedMTSetKind is the combined Set's P7 schema value, "0: (Fixed)".
// A FORM constant, deliberately NOT the dialect's mwWriteKind: MT-Set P7
// and MW-Set P7 are two command-specific facts that coincide on the
// evidenced radio, and deriving one from the other is the PadByte
// conflation (M9c-3 spec revision 2, adjudication). Set-direction '0'
// means "(Fixed)", not "VFO".
//
// EXPORTED at M9c-4 BY RENAME, not by an added alias: a combined-form
// dialect built outside this package (core/cat/ftdx10 is the first) must be
// able to NAME this value where it declares its own MWWriteKind, and a
// synonym pair — an exported alias beside a surviving unexported original —
// is exactly the shape that lets two spellings of one fact drift apart.
// There is one name.
//
// It is defined as the literal byte '0' and deliberately NOT as KindVFO,
// even though the two are equal. That equality is INCIDENTAL, across two
// different command meanings: KindVFO is the READ-side P7 vocabulary's
// "0: VFO" (memdata.go), whereas this is the combined Set's write-direction
// "0: (Fixed)". Defining one through the other would encode the coincidence
// as a rule, and a radio that ever separated them would take this constant
// with it. TestCombinedMTSetKindCoincidesWithKindVFO asserts the equality
// with that caveat, so the coincidence is on record without being load-
// bearing.
const CombinedMTSetKind byte = '0'

// combinedMTP11 is the byte P11Fixed puts at position 28 of the combined
// record, emitted by the builder and required by the parser.
//
// It was described here as "schema, not policy, and no dialect may vary it",
// with the note that "if hardware ever divorces the two, an MT-specific
// policy field is added then, additively". The FT-891's manual is that
// divorce — it prints `P11 0: TAG "OFF" 1: TAG "ON"` where every registered
// sibling prints "0: (Fixed)" — and MTPolicy.P11 is that field, added
// exactly as anticipated. This constant is now THE P11Fixed POLICY'S BYTE
// rather than the form's, which is why nothing about the FTdx10 family's
// frames moves: P11Fixed is what they declare.
const combinedMTP11 byte = '0'

// Positions AFTER the shared memory field block, fixed for every combined
// dialect. They are package constants, unlike the frame's LENGTH (see
// mtCombinedLen), because nothing about them varies by receiver: what
// varies is the width of the tag field that follows, and the length that
// width determines.
//
// They are expressed in terms of memdata.go's own offsets rather than as
// bare numbers, because that is the actual relationship: the combined
// record puts P11 exactly where the 28-byte MR/MW frame puts its ';'
// terminator — the byte immediately after the block — as memdata.go's
// offset commentary anticipates.
const (
	mtCombinedP11Offset = memTermOffset           // position 28, 1 byte, fixed '0'
	mtCombinedTagOffset = mtCombinedP11Offset + 1 // P12, the fixed-width tag field
)

// mtCombinedLen is the exact length of THIS dialect's combined MT
// Set/Answer record: the 28 fixed positions (the "MT" prefix, the shared
// memory field block at memdata.go's offsets 2-26, and P11) plus the
// full-width tag field plus the ';' terminator — 29 + TagMaxBytes, which is
// 41 for the evidenced 12-byte tag. There is no 41 constant anywhere, and
// there must not be.
//
// A receiver method rather than a package constant, exactly like
// mtShortAnswerMax and for the same reason: this length reaches the
// OUTBOUND WRITE GATE (allowlist.go's validMTCommand, M9c-3 task 5) as well
// as the builder and parser, and a gate consulting one family's tag width
// on every dialect is the seam this milestone exists to close.
//
// It is meaningful only under MTFormCombined; every caller refuses any other
// form before consulting it.
func (d Dialect) mtCombinedLen() int {
	return mtCombinedTagOffset + d.mt.TagMaxBytes + 1
}

// validateCombinedMTFields applies the combined MT Set's write-direction
// policy to m, returning a *ParseError describing the first violation found,
// or nil if m is safe to encode UNDER THIS DIALECT.
//
// It is validateMWFields' complete checklist (mw.go) with exactly two
// substitutions and nothing dropped:
//
//  1. the slot predicate is MT's OWN write policy, d.mtSlotValid — memory
//     and PMS slots only, 5xx/EMG refused by project decision pending
//     hardware verification. Behaviourally identical to d.writableSlot
//     today, but it is the MT lineage, and the two rules are free to
//     diverge when 5xx/EMG are verified for one command and not the other;
//  2. the kind is the FORM's schema constant, CombinedMTSetKind, NOT this
//     dialect's mwWriteKind. See that constant's doc comment.
//
// Everything else is MW's rule, for MW's reason: Mode, CTCSSState and Shift
// are byte-alias types, so a caller-forged value must be re-validated
// through this dialect's own ParseMode and through ParseCTCSSState/
// ParseShift; ModeUnset is separately refused, because parsers must accept
// the '-' placeholder and builders must never emit it; the clarifier is
// bounded by THIS DIALECT'S policy; and the frequency must be nonzero as
// well as fitting the 9-digit field.
//
// One shared Dialect method rather than a body inside the builder, because
// the outbound write gate runs the SAME policy against a decoded wire frame
// (M9c-3 task 5): the rules governing what may reach a radio as a combined
// MT Set live in exactly one place, as MW's do.
func (d Dialect) validateCombinedMTFields(m MemoryData) error {
	if !d.mtSlotValid(m.Slot) {
		return newParseError([]byte(m.Slot.Wire()), "MT: slot must be memory (001-099) or PMS (P1L-P9U); 5xx/EMG rejected by project policy pending M5a, \"000\"/invalid rejected per reference")
	}

	// The combined Set's P7 is the FORM's constant, and this is the
	// validate-don't-rewrite posture MW takes with its own kind: a record
	// carrying anything else is refused rather than silently corrected, so a
	// caller that believed it was writing a VFO or Memory record finds out.
	if m.Kind != CombinedMTSetKind {
		return newParseError([]byte{m.Kind}, fmt.Sprintf("MT: Kind must be %q, the combined Set's fixed P7 — the reference documents it \"(Fixed)\" in the SET direction, not \"VFO\", and it is deliberately not this dialect's MW write kind", CombinedMTSetKind))
	}

	// Mode is a raw byte alias (mode.go): never trust a caller-forged value.
	// Re-validate via THIS DIALECT'S ParseMode and separately reject
	// ModeUnset, which parsers must accept and builders must never emit.
	validMode, err := d.ParseMode(m.Mode.Wire())
	if err != nil {
		return newParseError([]byte{m.Mode.Wire()}, "MT: mode field (P6) is not a valid Mode")
	}
	if validMode == ModeUnset {
		return newParseError([]byte{m.Mode.Wire()}, "MT: mode field (P6) must not be ModeUnset in a Set frame")
	}

	if !d.validClarHz(m.ClarHz) {
		return newParseError([]byte(fmt.Sprintf("%d", m.ClarHz)), fmt.Sprintf("MT: ClarHz must be a multiple of %d Hz, magnitude <= %d", d.clar.StepHz, d.clar.MaxAbsHz))
	}

	if m.FreqHz == 0 || m.FreqHz > memFreqMax {
		return newParseError([]byte(fmt.Sprintf("%d", m.FreqHz)), "MT: FreqHz must be nonzero and fit in 9 digits (<= 999999999)")
	}

	// P5, BY THIS DIALECT'S OWN READING. Under P5Fixed byte 21 is printed
	// "(Fixed)" on this radio's memory blocks, so a record asking for the TX
	// clarifier is REFUSED rather than quietly encoded as '0': a caller that
	// believed it was writing the clarifier finds out, which is the
	// validate-don't-rewrite posture this validator takes with every other
	// field. This is the WRITE-direction check on a caller-supplied
	// MemoryData; a FORGED wire frame is instead refused earlier, by
	// parseMemoryFields (memdata.go), which the gate's combined-MT grammar
	// check reaches before this validator ever runs.
	//
	// A SWITCH, not an if with an implicit "everything else passes" arm: see
	// parseMemoryFields' matching comment (memdata.go) — an omitted config
	// semantic refuses rather than defaults, and NewDialect's V14 already
	// keeps every registered dialect from reaching the default case.
	switch d.memoryP5 {
	case P5Fixed:
		if m.TxClar {
			return newParseError([]byte{boolDigit(m.TxClar)}, fmt.Sprintf("MT: TxClar must be false under %v — this dialect's manual prints P5 (position 21) \"(Fixed)\", so there is no TX clarifier flag to set", d.memoryP5))
		}
	case P5TxClar:
	default:
		return newParseError(nil, "MT: P5 (position 21) policy unset — refusing to guess whether the byte is fixed schema or the TX clarifier flag")
	}

	// CTCSSState/Shift are byte-alias types exactly like Mode: re-validate
	// via their own Parse functions for the same reason.
	if _, err := ParseCTCSSState(m.CTCSS.Wire()); err != nil {
		return newParseError([]byte{m.CTCSS.Wire()}, "MT: CTCSS field (P8) is not a valid CTCSSState")
	}
	if _, err := ParseShift(m.Shift.Wire()); err != nil {
		return newParseError([]byte{m.Shift.Wire()}, "MT: shift field (P10) is not a valid Shift")
	}

	return nil
}

// BuildMTSetCombined builds a COMBINED-form MT (memory channel tag) Set
// frame: the whole memory record and its tag in one command, which is the
// FTdx10 family's shape. Reference: the CAT manual's "MT" position table
// (rev 2308-F, page 16), positions 1-41 for a 12-byte tag — "MT"(2) + the
// shared 28-position memory field block's fields (P1-P10) + P11 "0" fixed +
// the tag field + ";".
//
// It refuses any dialect that is not MTFormCombined, symmetric with
// BuildMTSet's own form refusal: the two forms share a prefix and a
// terminator, so a frame built in the wrong shape would look plausible all
// the way to the radio.
//
// THE TAG FIELD IS ALWAYS EMITTED AT FULL WIDTH, padded with this dialect's
// TagFill, which is correct under both readings of the manual's "up to 12
// characters" legend (the FT-710 accepts its own full-width padded form on
// hardware). An EMPTY tag is therefore the all-fill field: the combined form
// documents no distinct clear encoding, unlike the short form's clear byte.
//
// The tag rules are INPUT rules on the LOGICAL tag: charset (validMTTagByte
// — the same command-injection defence as the short form), width, and a
// refusal of any non-empty tag ENDING in TagFill. That last one is what
// makes build -> parse exact for every accepted tag, since the parse trims
// trailing fill; the caller is told to trim rather than having it silently
// trimmed. A tag of only fill bytes is refused by the same rule, "" being
// its canonical spelling. None of this transfers to the WIRE field, where
// trailing fill is inevitable — see the gate's own comment.
//
// Every refusal returns the zero Command alongside its error, the contract
// command.go documents for all fallible builders.
func (d Dialect) BuildMTSetCombined(m MemoryData, tag string) (Command, error) {
	// FORM FIRST, as BuildMTSet does. The error carries no input frame
	// because the offending value is the RECEIVER, not any argument.
	if d.mt.Form != MTFormCombined {
		return Command{}, newParseError(nil, fmt.Sprintf("MT: combined-form Set called on a %v dialect — use the short-form API", d.mt.Form))
	}
	// POLICY SECOND. A dialect whose P11 is a live TAG flag gets no default
	// from this builder: it has no display argument to offer, and writing
	// '0' for a flag the caller never expressed an intention about is exactly
	// the silent defaulting the M9c-1 ruling forbids.
	if d.mt.P11 == P11TagDisplay {
		return Command{}, newParseError(nil, "MT: this dialect's P11 is a live TAG flag; use the display-bearing builder/parser (BuildMTSetCombinedDisplay/ParseMTAnswerCombinedDisplay)")
	}
	return d.buildMTSetCombined(m, tag, false)
}

// BuildMTSetCombinedDisplay is BuildMTSetCombined for a dialect whose P11 is
// a live TAG ON/OFF flag: the caller supplies the flag, and it is written
// into byte 28 of the record.
//
// It REFUSES a P11Fixed dialect, symmetric with BuildMTSetCombined's refusal
// of a P11TagDisplay one. The two are not interchangeable in either
// direction: on a radio whose manual prints "0: (Fixed)" there is no flag to
// express, and a caller asking to set one has misunderstood the radio rather
// than made a harmless request.
//
// Everything else — the form check, the field validation, the tag rules, the
// fixed-width fill — is BuildMTSetCombined's, unchanged and shared.
func (d Dialect) BuildMTSetCombinedDisplay(m MemoryData, tag string, display bool) (Command, error) {
	if d.mt.Form != MTFormCombined {
		return Command{}, newParseError(nil, fmt.Sprintf("MT: combined-form Set called on a %v dialect — use the short-form API", d.mt.Form))
	}
	if d.mt.P11 != P11TagDisplay {
		return Command{}, newParseError(nil, fmt.Sprintf("MT: this dialect's P11 is %v, the record's printed-fixed byte 28; use the display-less builder/parser (BuildMTSetCombined/ParseMTAnswerCombined)", d.mt.P11))
	}
	return d.buildMTSetCombined(m, tag, display)
}

// buildMTSetCombined is the one body both combined builders share. display
// is meaningful only under P11TagDisplay; under P11Fixed the caller passes
// false and this writes combinedMTP11, which is the same byte the builder
// wrote before this policy existed.
func (d Dialect) buildMTSetCombined(m MemoryData, tag string, display bool) (Command, error) {
	if err := d.validateCombinedMTFields(m); err != nil {
		return Command{}, err
	}
	if !d.validMTTag(tag) {
		return Command{}, newParseError([]byte(tag), fmt.Sprintf("MT: tag must be 0-%d bytes of printable ASCII 0x20-0x7E, excluding ';', with no control bytes", d.mt.TagMaxBytes))
	}
	if tag != "" && tag[len(tag)-1] == d.mt.TagFill {
		return Command{}, newParseError([]byte(tag), fmt.Sprintf("MT: tag must not end in the fill byte %q — the field is padded to %d bytes with it and trimmed of it on the way back, so a trailing fill byte would not round-trip; trim it (an all-fill tag is refused by this rule too, \"\" being its canonical spelling)", d.mt.TagFill, d.mt.TagMaxBytes))
	}

	// Framing here, field block in encodeMemoryFields (memdata.go): the same
	// offsets 2-26 the MR answer and the MW Set use, which is the whole
	// point of task 3's extraction. m.Kind already equals the form constant
	// — validated above, not overwritten here.
	frame := make([]byte, d.mtCombinedLen())
	frame[0], frame[1] = 'M', 'T'
	if err := d.encodeMemoryFields(frame, m); err != nil {
		return Command{}, err
	}
	// P11, BY THIS DIALECT'S OWN READING. A SWITCH, not an if with an
	// implicit "everything else is P11Fixed" arm: NewDialect's V9
	// (dialectvalidate.go) already refuses a zero MTP11Policy on a
	// combined-form dialect at construction, but the default branch below
	// is what keeps this builder from silently taking the P11Fixed (wide)
	// reading in the one place V9 cannot reach.
	switch d.mt.P11 {
	case P11Fixed:
		frame[mtCombinedP11Offset] = combinedMTP11
	case P11TagDisplay:
		frame[mtCombinedP11Offset] = boolDigit(display)
	default:
		return Command{}, newParseError(nil, "MT: P11 (position 28) policy unset — refusing to guess whether the byte is fixed schema or a live TAG flag")
	}

	field := frame[mtCombinedTagOffset : mtCombinedTagOffset+d.mt.TagMaxBytes]
	n := copy(field, tag)
	for i := n; i < len(field); i++ {
		field[i] = d.mt.TagFill
	}
	frame[len(frame)-1] = ';'

	return newCommand(frame), nil
}

// ParseMTAnswerCombined strictly parses a COMBINED-form MT Set/Answer frame
// into the memory record it carries and its tag. It refuses any dialect that
// is not MTFormCombined, symmetric with ParseMTAnswer.
//
// Check order is parseMemoryFrame's, deliberately: length, prefix,
// terminator, then the field block, and only then this form's own narrowing.
// A frame wrong in several ways reports the same violation first whichever
// memory-shaped frame it is.
//
// THE LENGTH IS EXACT, and that is ASSUMED UNTIL STAGE R. The manual's tag
// legend says "up to 12 characters" — the same variable-length wording as
// the FT-710's, whose hardware does accept short Sets — so whether the
// radio ANSWERS at full width is unverified. If Stage R shows it does not,
// the recorded contingency is a window of 30..29+TagMaxBytes here and in the
// gate: contained, and not a schema change.
//
// THE KIND VOCABULARY NARROWS to the documented read pair {'0' VFO, '1'
// Memory}. parseMemoryFields accepts MR's wider '0'-'5', which is right for
// MR and wrong here: this record's own table documents two values, so
// accepting the others would turn an undocumented frame into data.
//
// The tag comes back through decodeCombinedTag: trailing fill is a
// wire-encoding concern and never reaches a caller, and an all-fill field
// is no tag.
func (d Dialect) ParseMTAnswerCombined(frame []byte) (MemoryData, string, error) {
	if d.mt.Form != MTFormCombined {
		return MemoryData{}, "", newParseError(frame, fmt.Sprintf("MT: combined-form answer parsed on a %v dialect — use the short-form API", d.mt.Form))
	}
	if d.mt.P11 == P11TagDisplay {
		return MemoryData{}, "", newParseError(frame, "MT: this dialect's P11 is a live TAG flag; use the display-bearing builder/parser (BuildMTSetCombinedDisplay/ParseMTAnswerCombinedDisplay)")
	}
	m, tag, _, err := d.parseMTAnswerCombined(frame)
	return m, tag, err
}

// ParseMTAnswerCombinedDisplay is ParseMTAnswerCombined for a dialect whose
// P11 is a live TAG ON/OFF flag: the flag comes back beside the tag, which is
// the shape the FT-710's SHORT form already uses. MemoryData is unchanged —
// the flag travels beside the record, not inside it.
//
// It REFUSES a P11Fixed dialect, symmetric with ParseMTAnswerCombined's
// refusal of a P11TagDisplay one: a caller reading a flag off a radio whose
// manual prints "(Fixed)" would be reading schema as state.
func (d Dialect) ParseMTAnswerCombinedDisplay(frame []byte) (MemoryData, string, bool, error) {
	if d.mt.Form != MTFormCombined {
		return MemoryData{}, "", false, newParseError(frame, fmt.Sprintf("MT: combined-form answer parsed on a %v dialect — use the short-form API", d.mt.Form))
	}
	if d.mt.P11 != P11TagDisplay {
		return MemoryData{}, "", false, newParseError(frame, fmt.Sprintf("MT: this dialect's P11 is %v, the record's printed-fixed byte 28; use the display-less builder/parser (BuildMTSetCombined/ParseMTAnswerCombined)", d.mt.P11))
	}
	return d.parseMTAnswerCombined(frame)
}

// p11Valid reports whether b is an admissible byte 28 of a combined MT
// record for THIS dialect's P11 reading: under P11TagDisplay, one of the two
// documented TAG ON/OFF values; under P11Fixed (and any other/zero value,
// which validateDialectConfig's V9 already refuses at construction), the
// record's printed-fixed byte and nothing else.
//
// ONE FUNCTION DECIDES. Both the parser (parseMTAnswerCombined, below) and
// the gate (AllowedCommand's MTFormCombined branch, allowlist.go) call this
// rather than each re-testing d.mt.P11 against the byte — so byte 28's
// admission rule cannot drift into two answers from one dialect.
func (d Dialect) p11Valid(b byte) bool {
	// A SWITCH, not an if/else with an implicit "everything else is
	// P11Fixed" arm: that shape was the S0-close review's HIGH-1 finding —
	// a zero MTP11Policy fell through to the P11Fixed reading and let a
	// combined answer's undocumented byte 28 (including a live '0' that
	// happened to coincide with combinedMTP11) pass both this parser and
	// the gate. NewDialect's V9 (validateMTPolicy, dialectvalidate.go)
	// already refuses a zero MTP11Policy on a combined-form dialect at
	// construction, but the default branch below is what keeps this
	// shared predicate from taking EITHER reading in the one place V9
	// cannot reach — see unsetpolicy_test.go.
	switch d.mt.P11 {
	case P11TagDisplay:
		_, err := parseBoolDigit(b)
		return err == nil
	case P11Fixed:
		return b == combinedMTP11
	default:
		return false
	}
}

// parseMTAnswerCombined is the one body both combined parsers share. The
// display flag it returns is false under P11Fixed, where byte 28 is required
// to be combinedMTP11 and carries no state.
func (d Dialect) parseMTAnswerCombined(frame []byte) (MemoryData, string, bool, error) {
	if want := d.mtCombinedLen(); len(frame) != want {
		return MemoryData{}, "", false, newParseError(frame, fmt.Sprintf("MT combined answer must be %d bytes", want))
	}
	if frame[0] != 'M' || frame[1] != 'T' {
		return MemoryData{}, "", false, newParseError(frame, "MT combined answer missing \"MT\" prefix")
	}
	if frame[len(frame)-1] != ';' {
		return MemoryData{}, "", false, newParseError(frame, "MT combined answer missing ';' terminator")
	}

	m, err := d.parseMemoryFields(frame, "MT")
	if err != nil {
		return MemoryData{}, "", false, err
	}
	switch m.Kind {
	case KindVFO, KindMemory:
	default:
		return MemoryData{}, "", false, newParseError(frame, fmt.Sprintf("MT frame: kind field (P7) must be %q (VFO) or %q (Memory)", KindVFO, KindMemory))
	}
	// P11, BY THIS DIALECT'S OWN READING, through p11Valid — the same
	// predicate the gate consults. Under P11Fixed this is the pre-existing
	// refusal, wording untouched — the badP11 test in mtcombined_test.go
	// stands on it. Under P11TagDisplay the byte is a flag, and only its two
	// documented values are accepted: a third value is an undocumented
	// frame, and this package does not turn one into data.
	display := false
	if !d.p11Valid(frame[mtCombinedP11Offset]) {
		if d.mt.P11 == P11TagDisplay {
			return MemoryData{}, "", false, newParseError(frame, fmt.Sprintf("MT frame: P11 (position 28) must be '0' or '1' under %v — it is this dialect's TAG ON/OFF flag", d.mt.P11))
		}
		return MemoryData{}, "", false, newParseError(frame, fmt.Sprintf("MT frame: P11 must be fixed %q", combinedMTP11))
	}
	if d.mt.P11 == P11TagDisplay {
		// p11Valid has already confirmed this succeeds.
		display, _ = parseBoolDigit(frame[mtCombinedP11Offset])
	}

	return m, d.decodeCombinedTag(string(frame[mtCombinedTagOffset : mtCombinedTagOffset+d.mt.TagMaxBytes])), display, nil
}

// decodeCombinedTag turns a combined answer's raw tag field into a tag
// value: trailing TagFill trimmed, and an all-fill field decoded as "".
//
// ONE RULE, not the short form's two. There, ClearTagByte and PadByte
// describe genuinely different events — how an empty tag is WRITTEN, and how
// a short tag COMES BACK — and conflating them destroyed data on a dialect
// where they differ. Here the form itself makes them one event: the field is
// fixed-width in both directions and filled with one declared byte, which is
// why MTPolicy carries TagFill alone for this form.
//
// The all-fill case therefore needs no branch of its own — trimming every
// trailing fill byte from a field of nothing else yields "" by the same
// rule, which is exactly the "an empty tag is the all-fill field" semantics
// stated in BuildMTSetCombined. A separate branch would be dead code
// asserting the same thing twice.
//
// Leading and interior bytes are always preserved, and the field is passed
// through with no charset re-validation, matching ParseMTAnswer's precedent:
// charset enforcement is a build-time (write-to-radio) defence, and
// rejecting a genuine radio reply over an unanticipated byte would be worse
// than accepting it.
func (d Dialect) decodeCombinedTag(raw string) string {
	return strings.TrimRight(raw, string(d.mt.TagFill))
}
