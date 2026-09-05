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

// answer480 is the same channel as a TS-480 would answer it.
//
// IT MUST CARRY A BYTE THAT ACTUALLY DIVERGES, or it is answer590 under
// another name. Byte 19 is the lockout here and byte 41 a printed constant,
// but both hold '0' on both rows, so those two lines say nothing on their
// own. P14 is where the fixture earns its keep: it is the tuning step index
// on this radio (480:979), and "03" is a legal one in either of ST's two
// mode-conditional ranges (480:1494-1500) and a value the 590 book never
// prints for P14, whose only legend is "00: FM Normal / 01: FM Narrow"
// (590:1569-1571).
func answer480() recordFields {
	f := answer590()
	f.p6 = "0"   // lockout OFF (480:962), NOT the data mode
	f.p14 = "03" // ST step index 3, which no 590 row would admit
	f.p15 = "0"  // "Always 0 for the TS-480." (480:982)
	return f
}

// TestAnswer480_CarriesAByteNo590RowWouldAdmit keeps the fixture above
// honest. A TS-480 fixture that was byte-identical to the 590 one would let
// every table below claim to test two radios while testing one frame twice,
// and the collapse would be invisible: the two rows' happy paths assert the
// same decoded fields.
func TestAnswer480_CarriesAByteNo590RowWouldAdmit(t *testing.T) {
	if answer480() == answer590() {
		t.Fatal("answer480 is byte-identical to answer590, so no test using it states anything about the TS-480's own reading of the grid")
	}
	if _, err := layout480().ParseMRAnswer(answer480().frame(t)); err != nil {
		t.Fatalf("the TS-480 refused its own fixture: %v", err)
	}
	if _, err := layout590SG().ParseMRAnswer(answer480().frame(t)); err == nil {
		t.Error("the TS-590SG accepted the TS-480 fixture; its P14 prints only \"00\" FM Normal and \"01\" FM Narrow (590:1569-1571)")
	}
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
			f := emptyWindow()

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

// emptyWindow zeroes P4-P15 and blanks P16, which is the empty channel of
// 590:1492-1493 (A18a). The tests below start from it and put ONE byte of
// the window back, which is what separates "the whole window is zero" from
// "P5 is zero".
func emptyWindow() recordFields {
	f := answer590()
	f.p4, f.p5, f.p6, f.p7 = "00000000000", "0", "0", "0"
	f.p8, f.p9, f.p11, f.p14, f.p15 = "00", "00", "0", "00", "0"
	f.p16 = "        "
	return f
}

// TestParseMRAnswer_TheEmptyWindowIsTheWholeRangeAndNotP5Alone is the OTHER
// direction of A18a, and it is the dangerous one.
//
// 590:1492-1493 says "If the selected channel is empty, P4 ~ P15 will be 0
// and P16 will be blank", and plan P15 states the rule as the whole range,
// never "P5 is zero". A predicate that tested the mode nibble alone would
// satisfy TestParseMRAnswer_AnEmptyChannelIsNeverRefused identically — that
// frame zeroes the whole window — and would then report EVERY answer whose
// P5 is '0' as an empty channel, whatever P4-P15 held. A record with a live
// frequency, name, tone indices and lockout would be returned with
// Empty = true and its content silently discarded on the READ path, which a
// downstream cannot detect because Record.Empty is exactly the flag it is
// told to trust. That is the data-loss class decision 11 and M9 exist to
// prevent, arriving through the one predicate that decides whether any
// field is read at all.
//
// The two cases are the two ways the window can be got wrong: a live byte
// BELOW P5 (a P5-only predicate calls it empty) and a live byte at the
// window's far end (a predicate that stopped before P15 calls it empty).
func TestParseMRAnswer_TheEmptyWindowIsTheWholeRangeAndNotP5Alone(t *testing.T) {
	// A real 14.250 MHz channel whose mode nibble happens to be '0'. It is
	// NOT the empty channel, and it is not interpretable either: P5 names no
	// mode in any legend, so the frame is refused rather than read.
	t.Run("a live frequency with a zero mode nibble", func(t *testing.T) {
		for _, tt := range []struct {
			name   string
			layout Layout
		}{{"TS-590SG", layout590SG()}, {"TS-480", layout480()}} {
			t.Run(tt.name, func(t *testing.T) {
				f := emptyWindow()
				f.p4 = "00014250000"
				rec, err := tt.layout.ParseMRAnswer(f.frame(t))
				if err == nil {
					t.Fatalf("ParseMRAnswer accepted a frame with a live P4 and P5='0': Empty = %v, FreqHz = %d — the empty-channel test is P4-P15, not P5 alone", rec.Empty, rec.FreqHz)
				}
				if !strings.Contains(err.Error(), "not all zero") {
					t.Errorf("refusal = %q, want it to say a frame whose P4-P15 are not all zero is not the empty channel", err)
				}
			})
		}
	})

	// The window's far end. P15 is the channel lockout on the 590 pair
	// (590:1572-1574), so a lockout-ON byte is a live field inside the
	// window and the frame is not the empty channel. The 480 is not run
	// here: its P15 is a printed constant (480:982) and checkPrintedFixed
	// refuses '1' there for a different reason entirely.
	t.Run("the window zero except P15, the channel lockout", func(t *testing.T) {
		for _, tt := range []struct {
			name   string
			layout Layout
		}{{"TS-590SG", layout590SG()}, {"TS-590S", layout590S()}} {
			t.Run(tt.name, func(t *testing.T) {
				f := emptyWindow()
				f.p15 = "1"
				rec, err := tt.layout.ParseMRAnswer(f.frame(t))
				if err == nil {
					t.Fatalf("ParseMRAnswer accepted a frame whose P15 is '1': Empty = %v — position 41 is inside P4-P15 and a locked-out channel is not an empty one", rec.Empty)
				}
				if !strings.Contains(err.Error(), "not all zero") {
					t.Errorf("refusal = %q, want it to say a frame whose P4-P15 are not all zero is not the empty channel", err)
				}
			})
		}
	})
}

// TestParseMRAnswer_TheEmptyWindowRequiresABlankP16 is the OTHER half of
// 590:1492-1493's sentence, and until this pin only the first half was
// enforced: "If the selected channel is empty, P4 ~ P15 will be 0 AND P16
// WILL BE BLANK."
//
// A3 IS WHAT "BLANK" MEANS HERE — eight spaces — and it is ASSUMED: the book
// says "blank" and defines it nowhere, so the register carries the reading
// and L-HW-2b lifts it per row. This codec therefore states the assumption
// and refuses what contradicts it, rather than tolerating any P16 under a
// window it has already decided is empty. A frame with a zero window and a
// live name is not a shape either book describes; returning it as
// Empty = true WITH a name would hand a caller a channel that is empty and
// named at once, and the name is exactly the field a driver would then write
// back.
//
// BOTH ROWS ARE RUN. The window test is a property of the frame and is
// applied on both (A4 leaves open whether a TS-480 answers an empty channel
// at all, which is a question about the radio, not about this shape).
func TestParseMRAnswer_TheEmptyWindowRequiresABlankP16(t *testing.T) {
	for _, tt := range []struct {
		name   string
		layout Layout
	}{{"TS-590SG", layout590SG()}, {"TS-480", layout480()}} {
		t.Run(tt.name, func(t *testing.T) {
			// Codex's own frame: a valid-length answer whose P4-P15 are all
			// zero and whose P16 spells a name.
			for _, p16 := range []string{"ABCDEFGH", "       X", "X       ", "00000000"} {
				f := emptyWindow()
				f.p16 = p16
				rec, err := tt.layout.ParseMRAnswer(f.frame(t))
				if err == nil {
					t.Errorf("ParseMRAnswer accepted an empty window whose P16 is %q: Empty = %v, Name = %q — the book prints P16 blank for an empty channel (590:1492-1493) and A3 reads blank as eight spaces", p16, rec.Empty, rec.Name)
					continue
				}
				if !strings.Contains(err.Error(), "A3") {
					t.Errorf("refusal for P16 %q reads %q, and it should cite A3, whose reading of \"blank\" it applies", p16, err)
				}
			}
			// The positive control is the blank name itself, which
			// TestParseMRAnswer_AnEmptyChannelIsNeverRefused also asserts:
			// eight spaces are admitted and trim to the empty name.
			if rec, err := tt.layout.ParseMRAnswer(emptyWindow().frame(t)); err != nil {
				t.Errorf("ParseMRAnswer refused the empty channel of 590:1492-1493 with its P16 blank: %v", err)
			} else if !rec.Empty || rec.Name != "" {
				t.Errorf("the blank-named empty channel decoded as Empty = %v, Name = %q", rec.Empty, rec.Name)
			}
		})
	}
}

// TestParseMRAnswer_RefusesAModeNibbleThisRowsLegendDoesNotName is the READ
// direction of the per-layout MD legend, and it is why ParseMode is
// membership against the receiver rather than a byte range (mode.go: "a
// range check would pass every test in this tree while the per-layout seam
// was fiction").
//
// Nibble '8' is "None (setting failure)" on the 590 pair (590:1362) and
// "Tune (Not used for the TS-480)" on the 480 (480:853), so no legend names
// it and no answer carrying it can be read as a mode. The BUILD direction is
// pinned by TestBuildMWSet_RefusesTheModeNibblesThatNameNoMode; without this
// pin a parser that dropped ParseMode's second return would carry Mode(0)
// into a record and report success.
func TestParseMRAnswer_RefusesAModeNibbleThisRowsLegendDoesNotName(t *testing.T) {
	for _, tt := range []struct {
		name   string
		layout Layout
		fields recordFields
	}{{"TS-590SG", layout590SG(), answer590()}, {"TS-480", layout480(), answer480()}} {
		t.Run(tt.name, func(t *testing.T) {
			f := tt.fields
			f.p5 = "8"
			rec, err := tt.layout.ParseMRAnswer(f.frame(t))
			if err == nil {
				t.Fatalf("ParseMRAnswer accepted P5='8': Mode = %v — neither book names that nibble as a mode a channel can be in", rec.Mode)
			}
			if !strings.Contains(err.Error(), "MD legend") {
				t.Errorf("refusal = %q, want it to name the row's MD legend", err)
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

// TestParseMRAnswer_EveryPrintedFixedByteIsRequiredOnParse is DECISION 7 on
// both rows: a hard-wired byte is REQUIRED on parse, not merely emitted on
// build, for the 480's sixteen and the 590 pair's thirteen alike (spec
// :1531-1533).
//
// A24 IS THE 480'S HALF ALONE. That book carries a general permission the
// 590 book does not — digits for a parameter "not applicable to this
// transceiver" may be any character but a control code or ';' (480:108-110)
// — which governs the SET side, so strictness on the ANSWER side is this
// programme's choice there and A24 records it, with L-HW-18 to lift it. The
// 590 rows' strictness is decision 7 and no hardware item covers it.
//
// It runs per LAYOUT, because the two rows hard-wire different sets.
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
		{"the 42-byte erase form's width", full[:42:42]},
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

// TestParseMRAnswer_TheNameIsRightTrimmedAndOnlyRightTrimmed is A1's rule as
// A1 states it: P16 is padded with SPACES on write and RIGHT-trimmed on read
// (the 480's KY gives the same-document precedent, 480:785-787).
//
// TRIMMING BOTH ENDS WOULD EAT A LEGITIMATE BYTE. A leading space is a
// printable character inside A2's charset and a name a user may have given a
// channel; only the trailing run is padding this codec put there. A parser
// that trimmed both ends would round-trip " A" to "A" and report success,
// which is a silent edit of the user's own text.
func TestParseMRAnswer_TheNameIsRightTrimmedAndOnlyRightTrimmed(t *testing.T) {
	f := answer590()
	f.p16 = " A      "
	rec, err := layout590SG().ParseMRAnswer(f.frame(t))
	if err != nil {
		t.Fatalf("ParseMRAnswer = %v, want nil", err)
	}
	if rec.Name != " A" {
		t.Errorf("Name = %q, want %q — only the trailing padding is this codec's, and the leading space is the user's byte", rec.Name, " A")
	}

	// And the write side pads it back to the same eight bytes.
	slot, err := layout590SG().NewSlot(7, ScanHalfNone)
	if err != nil {
		t.Fatalf("NewSlot: %v", err)
	}
	back := populatedRecord(slot)
	back.Name = rec.Name
	cmd, err := layout590SG().BuildMWSet(back)
	if err != nil {
		t.Fatalf("BuildMWSet = %v, want nil", err)
	}
	if got := string(cmd.Bytes()[recNameOff : recNameOff+recNameLen]); got != f.p16 {
		t.Errorf("P16 round-tripped to %q, want the %q it was read from", got, f.p16)
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

// TestEmptyName_IsEightSpaces keeps emptyName honest against recNameLen: a
// literal of spaces cannot be read at a glance, and one space too few would
// make the empty-window P16 check refuse every genuinely blank name — the
// whole-radio-read failure A18a's own comment exists to prevent.
func TestEmptyName_IsEightSpaces(t *testing.T) {
	if len(emptyName) != recNameLen {
		t.Fatalf("emptyName is %d bytes, and P16 is %d (590:1576, 480:984)", len(emptyName), recNameLen)
	}
	for i := 0; i < len(emptyName); i++ {
		if emptyName[i] != ' ' {
			t.Errorf("emptyName byte %d is %q, want a space — A3 reads \"blank\" as spaces", i+1, emptyName[i])
		}
	}
}
