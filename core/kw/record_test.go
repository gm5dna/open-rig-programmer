// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import (
	"errors"
	"strings"
	"testing"
)

// recordFields is one 50-byte memory frame written out parameter by
// parameter, so a test can vary a single field without re-counting fifty
// bytes — and so that a frame's SHAPE is visible beside the chart it came
// from. frame() asserts the total, which is what stops a typo in one part
// from silently borrowing a byte from its neighbour.
type recordFields struct {
	prefix string // positions 1-2
	p1     string // position 3
	p2     string // position 4
	p3     string // positions 5-6
	p4     string // positions 7-17
	p5     string // position 18
	p6     string // position 19
	p7     string // position 20
	p8     string // positions 21-22
	p9     string // positions 23-24
	p10    string // positions 25-27
	p11    string // position 28
	p12    string // position 29
	p13    string // positions 30-38
	p14    string // positions 39-40
	p15    string // position 41
	p16    string // positions 42-49
	term   string // position 50
}

// answer590 is a populated TS-590SG channel: memory 007, 14.250 MHz, USB,
// no tone, FILTER A, no lockout, named "TEST".
func answer590() recordFields {
	return recordFields{
		prefix: "MR", p1: "0", p2: "0", p3: "07",
		p4: "00014250000", p5: "2", p6: "0", p7: "0",
		p8: "00", p9: "00", p10: "000", p11: "0", p12: "0",
		p13: "000000000", p14: "00", p15: "0", p16: "TEST    ", term: ";",
	}
}

// answer480 is the same channel as a TS-480 would answer it: byte 19 is the
// lockout there and byte 41 a printed constant, so the same bytes mean
// different things and the fixture must say so.
func answer480() recordFields {
	f := answer590()
	f.p6 = "0"  // lockout OFF (480:962), NOT the data mode
	f.p15 = "0" // "Always 0 for the TS-480." (480:982)
	return f
}

// frame renders f, asserting the total width so a mis-sized part cannot
// pass as a shorter neighbour.
func (f recordFields) frame(t *testing.T) []byte {
	t.Helper()
	out := f.prefix + f.p1 + f.p2 + f.p3 + f.p4 + f.p5 + f.p6 + f.p7 +
		f.p8 + f.p9 + f.p10 + f.p11 + f.p12 + f.p13 + f.p14 + f.p15 + f.p16 + f.term
	if len(out) != RecordLen && f.term == ";" {
		t.Fatalf("test fixture is %d bytes, want %d: %q", len(out), RecordLen, out)
	}
	return []byte(out)
}

// TestParseMRAnswer_APopulatedChannel is the happy path on both radios, and
// it decodes every field of the grid.
func TestParseMRAnswer_APopulatedChannel(t *testing.T) {
	tests := []struct {
		name   string
		layout Layout
		fields recordFields
	}{
		{"TS-590SG", layout590SG(), answer590()},
		{"TS-480", layout480(), answer480()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, err := tt.layout.ParseMRAnswer(tt.fields.frame(t))
			if err != nil {
				t.Fatalf("ParseMRAnswer = %v, want nil", err)
			}
			if rec.Empty {
				t.Error("a populated channel parsed as Empty")
			}
			if rec.Slot.Number() != 7 || rec.Slot.Class() != SlotMemory {
				t.Errorf("Slot = %v (%v), want 007 SlotMemory", rec.Slot, rec.Slot.Class())
			}
			if rec.Slot.Half() != ScanHalfNone {
				t.Errorf("Slot.Half() = %v, want ScanHalfNone on an ordinary memory slot", rec.Slot.Half())
			}
			if rec.FreqHz != 14_250_000 {
				t.Errorf("FreqHz = %d, want 14250000", rec.FreqHz)
			}
			if rec.Mode != ModeUSB {
				t.Errorf("Mode = %v, want ModeUSB", rec.Mode)
			}
			if rec.ToneMode != ToneModeOff {
				t.Errorf("ToneMode = %v, want off", rec.ToneMode)
			}
			if rec.Name != "TEST" {
				t.Errorf("Name = %q, want %q — A1's right-trim", rec.Name, "TEST")
			}
			if rec.AnswerP1 != '0' {
				t.Errorf("AnswerP1 = %q, want '0'", rec.AnswerP1)
			}
		})
	}
}

// TestParseMRAnswer_AnEmptyChannelIsNeverRefused is A18a, which is
// DOCUMENTARY FACT rather than an assumption: "If the selected channel is
// empty, P4 ~ P15 will be 0 and P16 will be blank." (590:1492-1493). P5 is
// byte 18 and lies inside P4-P15, so every empty 590 channel answers with
// mode nibble '0' — on a lightly-used radio the commonest record there is.
//
// THE TEST IS THE WHOLE WINDOW, NOT P5 ALONE. A parser that refused on the
// zero mode nibble would fail a whole-radio read of a fresh radio at its
// first empty channel.
func TestParseMRAnswer_AnEmptyChannelIsNeverRefused(t *testing.T) {
	for _, tt := range []struct {
		name   string
		layout Layout
	}{{"TS-590SG", layout590SG()}, {"TS-480", layout480()}} {
		t.Run(tt.name, func(t *testing.T) {
			f := answer590()
			f.p4, f.p5, f.p6, f.p7 = "00000000000", "0", "0", "0"
			f.p8, f.p9, f.p11, f.p14, f.p15 = "00", "00", "0", "00", "0"
			f.p16 = "        "

			rec, err := tt.layout.ParseMRAnswer(f.frame(t))
			if err != nil {
				t.Fatalf("ParseMRAnswer of an empty channel = %v, want nil", err)
			}
			if !rec.Empty {
				t.Error("Empty = false for a frame whose P4-P15 are all zero")
			}
			if rec.Name != "" {
				t.Errorf("Name = %q, want %q", rec.Name, "")
			}
			if rec.Slot.Number() != 7 {
				t.Errorf("Slot = %v, want 007 — the slot is outside the empty window and is still decoded", rec.Slot)
			}
		})
	}
}

// TestParseMRAnswer_ASpaceIsALegalNumericByte pins the convention MC prints
// and MR/MW inherit by reference: "When entering a setting command, enter 0
// or a space for a channel number less than 100. For a response command, a
// space is entered for a channel number less than 100." (590:1334-1337).
//
// EVERY DIGIT PREDICATE IN THIS CODEC IS WRITTEN KNOWING THIS. A parser that
// required '0'..'9' at byte 4 would refuse the answer form the 590 pair
// actually send for channels 000-099 — which is most of them.
func TestParseMRAnswer_ASpaceIsALegalNumericByte(t *testing.T) {
	f := answer590()
	f.p2 = " "
	rec, err := layout590SG().ParseMRAnswer(f.frame(t))
	if err != nil {
		t.Fatalf("ParseMRAnswer with a space at byte 4 = %v, want nil", err)
	}
	if rec.Slot.Number() != 7 {
		t.Errorf("Slot = %v, want 007", rec.Slot)
	}

	// A10's other half: this codec ACCEPTS EITHER on parse and always EMITS
	// '0' (slotWire, builders.go). The two forms must decode to the same
	// slot, or a channel read with one convention would be stored under a
	// different identity from the same channel read with the other.
	g := answer590()
	g.p2 = "0"
	zero, err := layout590SG().ParseMRAnswer(g.frame(t))
	if err != nil {
		t.Fatalf("ParseMRAnswer with '0' at byte 4 = %v, want nil", err)
	}
	if zero.Slot != rec.Slot {
		t.Errorf("byte 4 '0' decoded to %v and ' ' to %v; A10 says both name the same channel", zero.Slot, rec.Slot)
	}
	cmd, err := layout590SG().BuildMRRead(rec.Slot)
	if err != nil {
		t.Fatalf("BuildMRRead: %v", err)
	}
	if got := cmd.Bytes()[recP2Off]; got != '0' {
		t.Errorf("BuildMRRead emitted %q at byte 4, want '0' — A10 emits the digit", got)
	}
}

// TestParseMRAnswer_TheHundredsDigitReachesTheScanAndExtensionClasses pins
// that byte 4 is a real digit above 99 on the 590 pair, and that P1 resolves
// a section channel's half.
func TestParseMRAnswer_TheHundredsDigitReachesTheScanAndExtensionClasses(t *testing.T) {
	tests := []struct {
		name      string
		p1, p2    string
		p3        string
		wantSlot  string
		wantClass SlotClass
		wantHalf  ScanHalf
	}{
		{"section channel, start frequency", "0", "1", "03", "103L", SlotScan, ScanLower},
		{"section channel, end frequency", "1", "1", "03", "103U", SlotScan, ScanUpper},
		{"extension channel", "0", "1", "15", "115", SlotExtension, ScanHalfNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := answer590()
			f.p1, f.p2, f.p3 = tt.p1, tt.p2, tt.p3
			rec, err := layout590SG().ParseMRAnswer(f.frame(t))
			if err != nil {
				t.Fatalf("ParseMRAnswer = %v, want nil", err)
			}
			if got := rec.Slot.String(); got != tt.wantSlot {
				t.Errorf("Slot = %q, want %q", got, tt.wantSlot)
			}
			if rec.Slot.Class() != tt.wantClass {
				t.Errorf("Slot.Class() = %v, want %v", rec.Slot.Class(), tt.wantClass)
			}
			if rec.Slot.Half() != tt.wantHalf {
				t.Errorf("Slot.Half() = %v, want %v", rec.Slot.Half(), tt.wantHalf)
			}
		})
	}
}

// TestParseMRAnswer_RefusesASlotOutsideThisLayoutsSpace pins that the slot
// domain is the RECEIVER'S. 110-119 are the TS-590SG's extension channels
// and the book never states the S's ceiling (A12), so the S layout stops at
// 109 and the same frame is admitted by one row and refused by the other.
func TestParseMRAnswer_RefusesASlotOutsideThisLayoutsSpace(t *testing.T) {
	f := answer590()
	f.p2, f.p3 = "1", "15"
	frame := f.frame(t)

	if _, err := layout590SG().ParseMRAnswer(frame); err != nil {
		t.Errorf("the TS-590SG refused slot 115: %v", err)
	}
	if _, err := layout590S().ParseMRAnswer(frame); err == nil {
		t.Error("the TS-590S accepted slot 115, which is above the ceiling A12 leaves it")
	}
	if _, err := layout480().ParseMRAnswer(frame); err == nil {
		t.Error("the TS-480 accepted slot 115, whose slot space is the flat 00-99 of 480:955")
	}
}

// TestParseMRAnswer_EveryPrintedFixedByteIsRequiredOnParse is A24: on the
// TS-480 a hard-wired byte is REQUIRED on parse, not merely emitted on
// build. The 480's general permission at 480:108-110 — digits for a
// parameter "not applicable to this transceiver" may be any character but a
// control code or ';' — governs the SET side, so strictness on the ANSWER
// side is this programme's choice and is recorded as such.
//
// It runs per LAYOUT, because the two rows hard-wire different sets: the
// 590SG's thirteen bytes against the 480's sixteen.
func TestParseMRAnswer_EveryPrintedFixedByteIsRequiredOnParse(t *testing.T) {
	for _, tt := range []struct {
		name   string
		layout Layout
		fields recordFields
	}{{"TS-590SG", layout590SG(), answer590()}, {"TS-480", layout480(), answer480()}} {
		t.Run(tt.name, func(t *testing.T) {
			fixed := tt.layout.PrintedFixed()
			if len(fixed) == 0 {
				t.Fatal("the layout declares no printed-fixed bytes, so this test would pass vacuously")
			}
			for _, ff := range fixed {
				for off := ff.Pos - 1; off < ff.Pos-1+len(ff.Printed); off++ {
					frame := tt.fields.frame(t)
					frame[off] = '9'
					if _, err := tt.layout.ParseMRAnswer(frame); err == nil {
						t.Errorf("ParseMRAnswer accepted a frame whose printed-fixed byte at position %d was '9'", off+1)
					}
				}
			}
		})
	}
}

// TestParseMRAnswer_RefusesAFrameOfTheWrongWidth pins the 50-byte rule on
// the read side, through the same predicate the builder's gate uses.
func TestParseMRAnswer_RefusesAFrameOfTheWrongWidth(t *testing.T) {
	full := answer590().frame(t)
	for _, tt := range []struct {
		name  string
		frame []byte
	}{
		{"the 42-byte erase form's width", full[:41:41]},
		{"one byte short", full[:49:49]},
		{"one byte long", append(append([]byte{}, full...), '0')},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := layout590SG().ParseMRAnswer(tt.frame)
			assertRecordLengthMismatch(t, err, len(tt.frame), RecordLen, "MR")
		})
	}
}

// TestParseMRAnswer_ChecksLengthThenPrefixThenTerminator pins the ORDER, so
// a frame that is wrong in several ways reports the most structural fault
// first and a later refactor cannot quietly reorder the diagnosis.
func TestParseMRAnswer_ChecksLengthThenPrefixThenTerminator(t *testing.T) {
	f := answer590()
	f.prefix, f.term = "XX", "!"
	short := f.frame(t)[:10:10]
	if _, err := layout590SG().ParseMRAnswer(short); !strings.Contains(err.Error(), "50 bytes") {
		t.Errorf("a short, wrongly prefixed, unterminated frame reported %v, want the length first", err)
	}

	f2 := answer590()
	f2.prefix, f2.term = "XX", "!"
	if _, err := layout590SG().ParseMRAnswer(f2.frame(t)); !strings.Contains(err.Error(), "prefix") {
		t.Errorf("a wrongly prefixed, unterminated frame reported %v, want the prefix before the terminator", err)
	}

	f3 := answer590()
	f3.term = "!"
	if _, err := layout590SG().ParseMRAnswer(f3.frame(t)); !strings.Contains(err.Error(), "terminator") {
		t.Errorf("an unterminated frame reported %v, want the terminator", err)
	}
}

// TestParseMRAnswer_RefusesAnMWFrame: MW has no Answer on either radio. The
// 2003 document prints the label over an empty chart (480:980, 480:985),
// which is erratum E17 and a transcription trap; a parser that accepted the
// prefix would be inventing a frame.
func TestParseMRAnswer_RefusesAnMWFrame(t *testing.T) {
	f := answer590()
	f.prefix = "MW"
	if _, err := layout590SG().ParseMRAnswer(f.frame(t)); err == nil {
		t.Error("ParseMRAnswer accepted an \"MW\" frame, which no radio sends")
	}
}

// TestParseMRAnswer_ZeroLayoutParsesNothing: a parser that decoded a frame
// on behalf of no radio would attribute a byte to a meaning nobody declared.
func TestParseMRAnswer_ZeroLayoutParsesNothing(t *testing.T) {
	var l Layout
	if _, err := l.ParseMRAnswer(answer590().frame(t)); err == nil {
		t.Error("a zero Layout parsed an MR answer")
	}
}

// TestParseMRAnswer_RefusesANameByteOutsideA2sCharset. A2's claim is
// printable ASCII 0x20-0x7E excluding ';', and it is BOUNDED AT 0x7F: the
// 590SG prints only "';' cannot be used" (590:1577) and the 480 forbids the
// control codes generally (480:108-110, 480:127-129), so nothing in either
// book says what a radio does with 0x7F or above and this codec refuses it
// rather than claiming it.
func TestParseMRAnswer_RefusesANameByteOutsideA2sCharset(t *testing.T) {
	for _, b := range []byte{0x00, 0x1f, 0x7f, 0x80, 0xff} {
		f := answer590()
		f.p16 = "A" + string([]byte{b}) + "      "
		if _, err := layout590SG().ParseMRAnswer(f.frame(t)); err == nil {
			t.Errorf("ParseMRAnswer accepted a name byte %#02x", b)
		}
	}
}

// TestParseMRAnswer_ToneIndicesAreBoundedByTheirPrintedCharts is A21 on the
// read side. TN prints "00 ~ 42" (590:2291, 480:1557) and CN "00 ~ 41"
// (590:411, 480:337); an index outside its own chart is refused rather than
// clamped, which is the choice that never silently stores a tone the user
// did not ask for.
func TestParseMRAnswer_ToneIndicesAreBoundedByTheirPrintedCharts(t *testing.T) {
	f := answer590()
	f.p7, f.p8 = "1", "42"
	if _, err := layout590SG().ParseMRAnswer(f.frame(t)); err != nil {
		t.Errorf("tone index 42 (TN's last, 1750 Hz) was refused: %v", err)
	}
	f.p8 = "43"
	if _, err := layout590SG().ParseMRAnswer(f.frame(t)); err == nil {
		t.Error("tone index 43 was accepted, and TN prints 00 ~ 42")
	}

	g := answer590()
	g.p7, g.p9 = "2", "41"
	if _, err := layout590SG().ParseMRAnswer(g.frame(t)); err != nil {
		t.Errorf("CTCSS index 41 (CN's last) was refused: %v", err)
	}
	g.p9 = "42"
	if _, err := layout590SG().ParseMRAnswer(g.frame(t)); err == nil {
		t.Error("CTCSS index 42 was accepted, and CN prints 00 ~ 41")
	}
}

// TestOutOfDomainError_NamesTheFieldWidth pins that the frequency refusal
// says what it knows. The only bound this codec has is the printed digit
// count; neither book prints a tuning range for MR/MW P4 (A17), so a message
// about "the radio's range" would be an invention.
func TestOutOfDomainError_NamesTheFieldWidth(t *testing.T) {
	l := layout590SG()
	slot, err := l.NewSlot(7, ScanHalfNone)
	if err != nil {
		t.Fatalf("NewSlot: %v", err)
	}
	rec := populatedRecord(slot)
	rec.FreqHz = MaxRecordFreqHz + 1

	_, err = l.BuildMWSet(rec)
	var domain *OutOfDomainError
	if !errors.As(err, &domain) {
		t.Fatalf("BuildMWSet = %v, want an *OutOfDomainError", err)
	}
	if !errors.Is(err, ErrOutOfDomain) {
		t.Errorf("errors.Is(err, ErrOutOfDomain) = false for %v", err)
	}
	if domain.Digits != 11 || domain.Max != MaxRecordFreqHz {
		t.Errorf("OutOfDomainError = %+v, want Digits 11 and Max %d", domain, uint64(MaxRecordFreqHz))
	}
	if !strings.Contains(domain.Error(), "FIELD WIDTH") {
		t.Errorf("Error() = %q, want it to say the bound is the field width", domain.Error())
	}
	if strings.Contains(domain.Error(), "tuning range, which") == false {
		t.Errorf("Error() = %q, want it to disclaim any tuning-range claim", domain.Error())
	}
}

// assertRecordLengthMismatch pins the width-refusal contract for a Kenwood
// memory frame: the caller can classify the failure, recover the codec's own
// measured lengths, and read a message that names both.
//
// IT IS NOT drivertest.AssertRecordLengthMismatch, AND IT CANNOT BE. The
// plan asks T6 to assert the width through that helper (L5). It lives in
// core/driver/internal/drivertest, and Go's internal-package rule allows an
// import only from within core/driver, so a core/kw test importing it does
// not compile: "use of internal package ... not allowed". Nor would the
// contract fit if it did: that helper asserts driver.ErrWrongRadio and a
// *civ.RecordLengthError — a CI-V type and a probe-time radio
// classification, neither of which a Kenwood record codec has or should
// acquire. So the SHAPE of the contract is reproduced here, in the family's
// own error vocabulary, and the sub-item is reported.
func assertRecordLengthMismatch(t testing.TB, err error, wantGot, wantWant int, wantCommand string) {
	t.Helper()
	var lengthErr *RecordLengthError
	if !errors.As(err, &lengthErr) {
		t.Fatalf("errors.As(err, *RecordLengthError) = false for %v", err)
	}
	if !errors.Is(err, ErrParse) {
		t.Errorf("errors.Is(err, ErrParse) = false for %v", err)
	}
	if lengthErr.Got != wantGot || lengthErr.Want != wantWant {
		t.Errorf("RecordLengthError = Got %d/Want %d, want %d/%d", lengthErr.Got, lengthErr.Want, wantGot, wantWant)
	}
	if lengthErr.Command != wantCommand {
		t.Errorf("RecordLengthError.Command = %q, want %q", lengthErr.Command, wantCommand)
	}
	if msg := err.Error(); !strings.Contains(msg, "50 bytes") {
		t.Errorf("Error() = %q, want it to name the 50-byte width", msg)
	}
}

// TestCheckRecordLen_IsTheOneWidthPredicateBothDirectionsConsult. The MW
// builder's own gate and the MR answer parser call the SAME predicate, which
// is why the width can be pinned on the build side at all: the builder fills
// a fifty-byte array, so no input can make it emit another length, and what
// there is to pin is that its gate would refuse one.
//
// TWO `!= 50` COMPARISONS ONE FILE APART WOULD BE ONE EDIT FROM DISAGREEING,
// and the disagreement would be silent in the direction that erases a user's
// channel: 590:1579-1581 describes a short MW that ERASES the channel
// specified by P2 and P3, its length is a reading rather than a printed
// number (A5, erratum E19), and this milestone never builds it (decision 8).
func TestCheckRecordLen_IsTheOneWidthPredicateBothDirectionsConsult(t *testing.T) {
	if err := checkRecordLen("MW", RecordLen, make([]byte, RecordLen)); err != nil {
		t.Errorf("checkRecordLen refused the printed width: %v", err)
	}
	for _, got := range []int{0, 7, 41, 42, 49, 51} {
		err := checkRecordLen("MW", got, make([]byte, got))
		assertRecordLengthMismatch(t, err, got, RecordLen, "MW")
		if !strings.Contains(err.Error(), "ERASES") {
			t.Errorf("Error() = %q, want it to say what a short MW does (590:1579-1581)", err)
		}
	}
}

// TestRecordLengthError_CarriesABoundedCopyOfTheOffendingFrame, on the
// ParseError precedent: the error never aliases caller memory and never
// grows without bound, and the content is radio-supplied so it is rendered
// %q-quoted rather than raw.
func TestRecordLengthError_CarriesABoundedCopyOfTheOffendingFrame(t *testing.T) {
	long := make([]byte, maxParseErrorFrameLen+20)
	for i := range long {
		long[i] = 'A'
	}
	var lengthErr *RecordLengthError
	if !errors.As(checkRecordLen("MR", len(long), long), &lengthErr) {
		t.Fatal("checkRecordLen did not return a *RecordLengthError")
	}
	if len(lengthErr.Frame) != maxParseErrorFrameLen {
		t.Errorf("Frame is %d bytes, want it truncated to %d", len(lengthErr.Frame), maxParseErrorFrameLen)
	}
	long[0] = 'Z'
	if lengthErr.Frame[0] == 'Z' {
		t.Error("Frame aliases the caller's slice")
	}
	if !strings.Contains(lengthErr.Error(), `input="AAA`) {
		t.Errorf("Error() = %q, want the offending input rendered %%q-quoted", lengthErr)
	}
}
