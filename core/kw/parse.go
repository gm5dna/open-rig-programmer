// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import (
	"fmt"
	"strings"
)

// RecordLengthError reports a memory frame that is not exactly RecordLen
// bytes, carrying both measured lengths so a caller can classify the
// failure and report the numbers without re-parsing the message.
//
// IT IS core/kw's OWN, AND NOT THE FLEET HELPER THE PLAN NAMED. Task 6 asks
// for the width to be asserted through drivertest.AssertRecordLengthMismatch
// (L5). That helper is in core/driver/internal/drivertest, which Go's
// internal-package rule makes importable only from within core/driver, so no
// test in core/kw can reach it; and its contract asserts driver.ErrWrongRadio
// and a *civ.RecordLengthError, a CI-V type and a probe-time radio
// classification that a Kenwood record codec neither has nor should acquire.
// The record_test.go helper reproduces the contract's SHAPE — classify,
// recover the measured lengths, exact text — in this family's own error
// vocabulary. T11 DOES NOT CONSUME THE FLEET HELPER EITHER, and the reason
// is the second one rather than the import rule: core/driver/ts590 can reach
// drivertest, but calling it would make that driver import core/civ and claim
// driver.ErrWrongRadio for a frame width, which on this family is false — a
// short MR answer is a malformed memory frame on the READ path, not a
// probe-time radio classification. It keeps a package-local helper of the
// same shape. The Kenwood-shaped sibling both packages can share
// (drivertest.AssertKenwoodRecordLengthMismatch, over *RecordLengthError and
// ErrParse) lands at T14 with the TS-480, which is the first point two
// drivers need it.
type RecordLengthError struct {
	// Command is the two-letter frame name, "MR" or "MW".
	Command string
	Got     int
	Want    int
	// Frame holds a defensive copy of the offending input, truncated to
	// maxParseErrorFrameLen, on the ParseError precedent (errors.go): the
	// error never aliases caller memory and never grows without bound, and
	// it is rendered %q-quoted because frame content is radio-supplied.
	Frame  []byte
	reason string
}

// Error implements the error interface.
func (e *RecordLengthError) Error() string {
	return fmt.Sprintf("kw: %s memory frame is %d bytes, want exactly %d bytes (%s) (input=%q)", e.Command, e.Got, e.Want, e.reason, e.Frame)
}

// Unwrap puts this in the package's parse-error family, so a caller that
// only wants "the frame was malformed" needs one comparison.
func (e *RecordLengthError) Unwrap() error { return ErrParse }

// checkRecordLen is the ONE width predicate this codec has, consulted by
// the answer parser and by the MW builder's own gate on its own output.
//
// A SINGLE PREDICATE RATHER THAN TWO LITERALS is the point. The reason
// string names what a wrong width would mean: 590:1579-1581 describes a
// short MW that ERASES the channel, its length is a reading rather than a
// printed number (A5, erratum E19), and this milestone never builds it
// (decision 8). Two `!= 50` comparisons one file apart would be one edit
// from disagreeing, and the disagreement would be silent in the direction
// that erases a user's channel.
func checkRecordLen(command string, got int, frame []byte) error {
	if got == RecordLen {
		return nil
	}
	n := len(frame)
	if n > maxParseErrorFrameLen {
		n = maxParseErrorFrameLen
	}
	return &RecordLengthError{
		Command: command,
		Got:     got,
		Want:    RecordLen,
		reason:  "both books print one 50-byte grid for MR's answer and MW's Set (590:1440-1461, 590:1518-1536; 480:923-943, 480:955-976), and the short MW form 590:1579-1581 describes ERASES the channel, so a frame of any other width is refused rather than interpreted",
		Frame:   copyBytes(frame[:n]),
	}
}

// ParseMRAnswer decodes a 50-byte MR ANSWER under THIS LAYOUT'S reading of
// the shared grid — the frame a radio sends.
//
// It refuses an "MW" prefix rather than treating the two frames as
// interchangeable, even though the fifty bytes after the prefix are the same
// fifty bytes: MR has no Set and MW has no Answer on either radio (erratum
// E17). parseRecordFrame is where that rule and every field check live, and
// it is the same decoder the outbound gate runs an MW Set through.
func (l Layout) ParseMRAnswer(frame []byte) (Record, error) {
	return l.parseRecordFrame("MR", "MR answer", frame)
}

// parseRecordFrame is the ONE decoder for the shared 50-byte grid, in
// whichever direction the frame is travelling: command is the two-letter
// name required at positions 1-2 and what names the frame in every refusal.
//
// IT IS PARAMETERISED BECAUSE THE OUTBOUND GATE MUST RE-VALIDATE AN MW
// FIELD BY FIELD (allowlist.go), and the only honest way to do that is
// through the same decoder ParseMRAnswer uses. A second copy of these
// checks, written for the write direction, would be one edit from
// disagreeing with this one — and the disagreement would be a frame the
// parser refuses and the gate admits, which is the wrong way round for the
// last defence before a physical radio.
//
// THE ORDER OF THE STRUCTURAL CHECKS IS DELIBERATE AND PINNED: a frame that
// is wrong in several ways reports its length first, then its prefix, then
// its terminator, then a hard-wired byte, and only then a field. See
// TestParseMRAnswer_ChecksLengthThenPrefixThenTerminator.
//
// THE PREFIX IS STILL REQUIRED EXACTLY, AND NEITHER NAME IS AN ALTERNATIVE
// SPELLING OF THE OTHER: MR has no Set and MW has no Answer on either radio.
// On the 2003 document that is visible only as an empty chart under a
// printed label (480:911 for MR Set, 480:980 and 480:985 for MW Read and
// Answer), which is erratum E17.
func (l Layout) parseRecordFrame(command, what string, frame []byte) (Record, error) {
	if !l.Configured() {
		return Record{}, newParseError(frame, "%s: this layout is unconfigured and describes no radio, so no byte of this frame has a meaning to read", what)
	}
	if err := checkRecordLen(command, len(frame), frame); err != nil {
		return Record{}, err
	}
	if frame[recPrefixOff] != command[0] || frame[recPrefixOff+1] != command[1] {
		return Record{}, newParseError(frame, "%s: missing %q prefix — MR has no Set and MW has no Answer on either radio (480:911, 480:985, erratum E17), so an %q frame is not an alternative spelling of this one", what, command, frame[:recPrefixLen])
	}
	if frame[recTermOff] != ';' {
		return Record{}, newParseError(frame, "%s: missing ';' terminator at position %d", what, recTermOff+1)
	}
	if err := l.checkPrintedFixed(what, frame); err != nil {
		return Record{}, err
	}

	p1 := frame[recP1Off]
	if p1 != '0' && p1 != '1' {
		return Record{}, newParseError(frame, "%s: P1 is %q, and both books print only '0' and '1' (590:1519-1520, 480:951)", what, p1)
	}

	slot, err := l.parseSlot(what, frame, p1)
	if err != nil {
		return Record{}, err
	}

	rec := Record{Slot: slot, AnswerP1: p1}

	// A18a, AND IT IS TESTED BEFORE ANY FIELD IS INTERPRETED. "If the
	// selected channel is empty, P4 ~ P15 will be 0 and P16 will be blank."
	// (590:1492-1493). P5 is byte 18, inside that window, so an empty 590
	// channel answers with mode nibble '0' — a nibble no legend names. A
	// parser that reached ParseMode first would refuse at the first empty
	// channel and fail a whole-radio read of a fresh radio.
	//
	// THE TS-480 HALF IS A4 AND IS UNLIFTED, but what A4 leaves open is
	// whether that radio ANSWERS an empty channel at all rather than
	// rejecting — a question about the radio, settled at hardware item 3,
	// the release gate for that row. The structural test below is a property
	// of the FRAME and is applied on both rows: it makes no claim that a
	// TS-480 ever sends one.
	// AND P16 MUST BE BLANK, which is the second half of that same sentence
	// and A3's reading of it: "P4 ~ P15 will be 0 AND P16 WILL BE BLANK."
	// The book never defines blank, so A3 does — eight spaces — and this
	// codec states its assumption rather than tolerating whatever P16
	// carries under a window it has already decided is empty. A frame that
	// is empty and named at once is not a shape either book describes, and
	// the name is the field a driver would write back.
	// TestParseMRAnswer_TheEmptyWindowRequiresABlankP16 pins both rows.
	if isEmptyWindow(frame) {
		if got := string(frame[recNameOff : recNameOff+recNameLen]); got != emptyName {
			return Record{}, newParseError(frame, "%s: P4-P15 are all zero, which is the empty channel of 590:1492-1493, but P16 is %q — the same sentence says P16 \"will be blank\", and A3 reads blank as %d spaces", what, got, recNameLen)
		}
		rec.Empty = true
		return rec, nil
	}

	mode, ok := l.ParseMode(frame[recModeOff])
	if !ok {
		return Record{}, newParseError(frame, "%s: P5 is %q, which the %s's MD legend does not name as a mode; a frame whose P4-P15 are not all zero is not the empty channel of 590:1492-1493 (A18a)", what, frame[recModeOff], l.model)
	}
	rec.Mode = mode

	freq, err := parseDigits(frame[recFreqOff:recFreqOff+recFreqDigits], "P4, the frequency")
	if err != nil {
		return Record{}, newParseError(frame, "%s: %v", what, err)
	}
	rec.FreqHz = freq

	if b := frame[recByte19Off]; b == '0' || b == '1' {
		rec.Byte19 = b
	} else {
		return Record{}, newParseError(frame, "%s: byte 19 is %q, and it is %v on the %s, whose book prints only '0' and '1' there", what, b, l.byte19, l.model)
	}

	tone := ToneMode(frame[recToneModeOff])
	if !l.ValidToneMode(tone) {
		return Record{}, newParseError(frame, "%s: P7 is %q, and the %s prints %s", what, byte(tone), l.model, l.toneModeText())
	}
	rec.ToneMode = tone

	toneIdx, err := parseDigits(frame[recToneOff:recToneOff+recToneDigits], "P8, the tone number")
	if err != nil {
		return Record{}, newParseError(frame, "%s: %v", what, err)
	}
	if toneIdx > MaxToneIndex {
		return Record{}, newParseError(frame, "%s: P8 is %d, and TN prints 00 ~ %d (590:2291, 480:1557); an index outside its own chart is refused rather than clamped (A21)", what, toneIdx, MaxToneIndex)
	}
	rec.ToneIndex = int(toneIdx)

	ctcssIdx, err := parseDigits(frame[recCTCSSOff:recCTCSSOff+recCTCSSDigits], "P9, the CTCSS number")
	if err != nil {
		return Record{}, newParseError(frame, "%s: %v", what, err)
	}
	if ctcssIdx > MaxCTCSSIndex {
		return Record{}, newParseError(frame, "%s: P9 is %d, and CN prints 00 ~ %d (590:411, 480:337); an index outside its own chart is refused rather than clamped (A21)", what, ctcssIdx, MaxCTCSSIndex)
	}
	rec.CTCSSIndex = int(ctcssIdx)

	if err := l.checkByte28(frame[recByte28Off]); err != nil {
		return Record{}, newParseError(frame, "%s: %v", what, err)
	}
	rec.Byte28 = frame[recByte28Off]

	b3940 := string(frame[recByte3940Off : recByte3940Off+recByte3940Len])
	if err := l.checkByte3940(b3940); err != nil {
		return Record{}, newParseError(frame, "%s: %v", what, err)
	}
	rec.Byte3940 = b3940

	if err := l.checkByte41(frame[recByte41Off]); err != nil {
		return Record{}, newParseError(frame, "%s: %v", what, err)
	}
	rec.Byte41 = frame[recByte41Off]

	name, err := parseName(what, frame)
	if err != nil {
		return Record{}, err
	}
	rec.Name = name

	return rec, nil
}

// parseSlot decodes P2 and P3 into a slot under this layout's slot space,
// with p1 resolving a section channel's half.
//
// A SPACE IS A LEGAL "NUMERIC" BYTE HERE. MR/MW's P2 and P3 are "Channel
// number (refer to the MC command)" — 590:1452-1453 in the MR ANSWER's own
// chart and 590:1539-1540 in MW's, and this method decodes both, since
// parseRecordFrame serves the answer parser and the outbound gate alike —
// and MC's own chart prints the
// convention: "When entering a setting command, enter 0 or a
// space for a channel number less than 100. For a response command, a space
// is entered for a channel number less than 100." (590:1334-1337). So the
// ANSWER form for channels 000-099 — most of a radio's channels — carries a
// space at byte 4, and a predicate demanding '0'..'9' there would refuse
// nearly every record the 590 pair send. That MR and MW inherit MC's
// convention at all is A10 — those two frames only say "refer to the MC
// command" — and A10's other half is the build side, where this codec always
// emits '0' (slotWire, builders.go). THE OUTBOUND GATE DOES NOT WIDEN ITSELF
// ON THAT ASSUMPTION: an MW offered to it is decoded here and then
// re-encoded through slotWire, so a space-spelled one differs from the frame
// the builder would have emitted and is refused there rather than here. (MC
// is the opposite case and the gate says so: its space IS printed.)
// P3 is not affected: MC prints "When the
// channel number is less than 10 ... the first digit is \"0\""
// (590:1342-1343), so those two bytes are always digits.
func (l Layout) parseSlot(what string, frame []byte, p1 byte) (Slot, error) {
	hundreds := 0
	p2 := frame[recP2Off]
	switch l.p2 {
	case P2HundredsDigit:
		switch {
		case p2 == ' ':
			hundreds = 0
		case p2 >= '0' && p2 <= '9':
			hundreds = int(p2 - '0')
		default:
			return Slot{}, newParseError(frame, "%s: byte 4 is %q; on the %s it is the channel's hundreds digit, which MC prints as a digit or a space below 100 (590:1332-1337)", what, p2, l.model)
		}
	case P2FixedZero:
		// checkPrintedFixed has already required '0' here, which is what the
		// 480 prints (480:953); the arm exists so the two policies are
		// exhaustive rather than one being an implicit default.
		hundreds = 0
	default:
		return Slot{}, newParseError(frame, "%s: byte 4's policy is unset on this layout", what)
	}

	digits := frame[recP3Off : recP3Off+recP3Digits]
	for i, b := range digits {
		if b < '0' || b > '9' {
			return Slot{}, newParseError(frame, "%s: P3 byte %d is %q; MC prints both digits of the channel number, zero-padded below 10 (590:1342-1343, 480:955)", what, i+1, b)
		}
	}
	number := hundreds*100 + int(digits[0]-'0')*10 + int(digits[1]-'0')

	class := l.classOf(number)
	if class == SlotClassInvalid {
		return Slot{}, newParseError(frame, "%s: slot %d is outside the %s's slot space %s", what, number, l.model, l.slotSpaceText())
	}
	half := ScanHalfNone
	if class == SlotScan {
		half = ScanLower
		if p1 == '1' {
			half = ScanUpper
		}
	}
	return Slot{number: number, class: class, half: half}, nil
}

// parseName decodes P16, right-trimming the spaces a write pads it with.
//
// A1 IS THE RULE AND IT IS ASSUMED, NOT PRINTED. Neither book states the
// padding rule for P16; the 480's KY gives the same-document precedent for a
// different command — "The fixed 24-byte length is used for the P2
// parameter. \" \" (space) character must be used for the unused characters."
// (480:785-787) — and the lift is a write-then-read on each registry row.
//
// The charset check is A2's, whose claim is BOUNDED AT 0x7F: 0x80-0xFF is
// unevidenced and unclaimed, so this codec refuses those bytes by its own
// rule and says nothing about what a radio would do with one.
func parseName(what string, frame []byte) (string, error) {
	raw := frame[recNameOff : recNameOff+recNameLen]
	for i, b := range raw {
		if err := checkNameByte(b); err != nil {
			return "", newParseError(frame, "%s: P16 byte %d: %v", what, i+1, err)
		}
	}
	return strings.TrimRight(string(raw), " "), nil
}

// checkNameByte applies A2's charset to one P16 byte.
func checkNameByte(b byte) error {
	switch {
	case b == ';':
		return fmt.Errorf("%q cannot be used in a memory name (590:1577)", b)
	case b < 0x20 || b > 0x7e:
		return fmt.Errorf("byte %#02x is outside printable ASCII 0x20-0x7E; the 480 forbids the control codes generally (480:127-129) and A2's claim is bounded at 0x7F, so nothing in either book says what a radio would do with this byte", b)
	default:
		return nil
	}
}

// checkPrintedFixed requires every byte THIS LAYOUT declares hard-wired to
// carry its printed constant.
//
// IT RUNS ON EVERY ROW, UNDER DECISION 7: the 480's sixteen printed-fixed
// bytes and the 590 pair's thirteen are all REQUIRED ON PARSE and not merely
// emitted on build (spec :1531-1533).
//
// A24 IS THE 480'S ESCAPE HATCH AND ONLY THE 480'S. That book carries a
// general permission the 590 book does not: digits for a parameter "not
// applicable to this transceiver" may be filled with any character except
// the control codes and ';' (480:108-111). It governs the SET side, so a
// TS-480 answering a hard-wired byte with something else would not
// necessarily be faulty. Strictness on the answer side is this programme's
// choice, and A24's lift L-HW-18 is a dozen real reads OF THAT RADIO: a
// single counter-example converts the rule from "required" to "accepted and
// normalised" there. The 590 pair's thirteen bytes rest on decision 7 and on
// nothing L-HW-18 will ever observe, so no reading of this method should
// take them as covered by that lift.
func (l Layout) checkPrintedFixed(what string, frame []byte) error {
	// The refusal names decision 7, which covers both rows, and adds A24 on
	// the 480 alone — that entry's claim and its lift are about that radio,
	// so quoting it on a 590 refusal would offer a reader a hardware
	// observation that will never be made.
	why := "decision 7"
	if l.book == Book480 {
		why = "decision 7, and this parse-side strictness on the 480 is A24"
	}
	for _, ff := range l.printedFixed {
		got := string(frame[ff.Pos-1 : ff.Pos-1+len(ff.Printed)])
		if got != ff.Printed {
			return newParseError(frame, "%s: positions %d-%d are %q, and the %s's book prints %q there (%s)", what, ff.Pos, ff.end(), got, l.model, ff.Printed, why)
		}
	}
	return nil
}

// checkByte28 applies this row's byte-28 policy to one wire byte.
//
// THE THREE POLICIES ARE THE THREE READINGS THE BOOK SUPPORTS. The 590 pair
// share one printed legend, "0: FILTER A / 1: FILTER B" (590:1560-1563), and
// then a sentence scoped to one of them: "* In firmware version 1.xx of
// TS-590S, always \"0\"." (590:1478, MR's wording; MW says the same thing in
// different words at 590:1564, which is erratum E7). So both 590 rows must
// ACCEPT '1' on a read — an S at firmware 2.00 or later answers with it —
// and the difference between the rows is what a WRITE may carry, which is
// the driver's question. The 480 prints a constant instead (480:973), and
// that arm is already covered by checkPrintedFixed; it is restated here so
// the switch is exhaustive rather than leaving a policy to an implicit
// default.
func (l Layout) checkByte28(b byte) error {
	switch l.byte28 {
	case Byte28FilterLive, Byte28FilterEither:
		if b != '0' && b != '1' {
			return fmt.Errorf("byte 28 is %q, and the %s's P11 prints '0' FILTER A and '1' FILTER B (590:1560-1563)", b, l.model)
		}
		return nil
	case Byte28FixedZero:
		if b != '0' {
			return fmt.Errorf("byte 28 is %q, and the %s prints \"Always 0\" there (480:973)", b, l.model)
		}
		return nil
	default:
		return fmt.Errorf("byte 28's policy is unset on this layout — refusing to guess whether it is a live filter selection or a printed constant")
	}
}

// The two values the 590 book prints for P14: "00: FM Normal" and "01: FM
// Narrow" (590:1569-1571). They are named ONCE because the set this parser
// admits and the set the FM-N synthesis reads (Layout.RecordModeName,
// mode.go) are ONE DATUM: two copies of it a file apart would be one edit
// from disagreeing, and the disagreement would show as a channel named
// "FM" whose byte says narrow.
const (
	byte3940FMNormal = "00"
	byte3940FMNarrow = "01"
)

// checkByte3940 applies this row's bytes-39-40 meaning to the two wire
// bytes.
//
// The 590 pair print exactly two values, "00: FM Normal" and "01: FM Narrow"
// (590:1569-1571), and nothing else; the 480 prints a step index referred to
// ST (480:979), whose own legend is mode-conditional over two ranges,
// 00-04 for SSB/CW/FSK and 00-09 for AM/FM (480:1494-1500). The bound
// enforced here is the UNION, 00-09, AND THAT IS A CHOICE RATHER THAN A
// LIMITATION: the parser holds frame[recModeOff] and a Record holds
// rec.Mode, either of which maps onto ST's SSB/CW/FSK against AM/FM split,
// so a narrower bound is available. It is not taken, because MR and MW only
// refer P14 to ST and neither chart says the channel's own P5 selects which
// of ST's two legends applies; enforcing that pairing would make this codec
// the author of a rule no page prints. The wider printed bound is admitted
// and nothing narrower is invented. The cost is nil while A22 refuses every
// TS-480 channel write, and a milestone that lifts A22 should revisit this
// line rather than inherit it. That the 480 cannot be WRITTEN at all is A22,
// and it is the driver's refusal, not this one.
func (l Layout) checkByte3940(s string) error {
	switch l.byte3940 {
	case Byte3940FMNarrowFlag:
		if s != byte3940FMNormal && s != byte3940FMNarrow {
			return fmt.Errorf("bytes 39-40 are %q, and the %s's P14 prints only \"00\" FM Normal and \"01\" FM Narrow (590:1569-1571)", s, l.model)
		}
		return nil
	case Byte3940StepIndex:
		if s[0] != '0' || s[1] < '0' || s[1] > '9' {
			return fmt.Errorf("bytes 39-40 are %q, and the %s's P14 is an ST step index, whose widest printed legend is 00-09 (480:979, 480:1494-1500)", s, l.model)
		}
		return nil
	default:
		return fmt.Errorf("bytes 39-40's meaning is unset on this layout — refusing to guess whether they are an FM bandwidth flag or a tuning step index")
	}
}

// checkByte41 applies this row's byte-41 meaning to one wire byte: the
// channel lockout on the 590 pair (590:1572-1574), a printed constant on the
// 480 (480:982).
func (l Layout) checkByte41(b byte) error {
	switch l.byte41 {
	case Byte41Lockout:
		if b != '0' && b != '1' {
			return fmt.Errorf("byte 41 is %q, and the %s's P15 prints '0' Channel Lockout OFF and '1' Channel Lockout ON (590:1572-1574)", b, l.model)
		}
		return nil
	case Byte41FixedZero:
		if b != '0' {
			return fmt.Errorf("byte 41 is %q, and the %s prints \"Always 0\" there (480:982)", b, l.model)
		}
		return nil
	default:
		return fmt.Errorf("byte 41's meaning is unset on this layout — refusing to guess whether it is the channel lockout or a printed constant")
	}
}

// parseDigits reads a run of ASCII digits, naming the field in its refusal.
// A SPACE IS NOT ADMITTED HERE: the space convention is MC's P1 alone
// (590:1334-1337), and widening it to every numeric run in the grid would
// admit a frequency with a hole in it.
func parseDigits(b []byte, field string) (uint64, error) {
	var n uint64
	for i, c := range b {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("%s byte %d is %q, which is not a digit", field, i+1, c)
		}
		n = n*10 + uint64(c-'0')
	}
	return n, nil
}
