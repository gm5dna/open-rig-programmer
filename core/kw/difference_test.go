// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import (
	"strings"
	"testing"
)

// THE PER-RADIO DIFFERENCE PINS, AND WHY EVERY ONE OF THEM NAMES TWO RADIOS.
//
// The six axes below are the counts on which the two books read the same
// fifty bytes differently, and each is a byte whose wrong reading is silent:
// a TS-480's channel lockout sits where the 590 pair keep the data mode, and
// the 480's printed constants sit where they keep the filter and the lockout.
//
// A pin that asserted one layout's axis alone would still pass if a later
// edit copied that layout's value into the other — which is the ONE failure
// this whole arrangement exists to prevent, and the one a per-model test
// suite cannot see. So every test in this file states a fact about BOTH
// radios in a single assertion pass: the axis value on each row, and, where
// the wire admits it, a byte that one radio accepts and the other refuses.
//
// The 590S is here as well wherever the two 590 rows differ (byte 28), so a
// per-BOOK axis cannot be mistaken for a per-ROW one.

// TestDifference_Byte19MeansDataModeOnThe590PairAndLockoutOnThe480 is the
// axis the whole Layout arrangement was built for. Both books print '0' and
// '1' there, so NO frame distinguishes them: "Data mode ... refer to the DA
// command" (590:1546-1548) against "Lockout status. 0: Lockout OFF, 1:
// Lockout ON." (480:962). A parser that hard-coded either reading would put
// one radio's lockout under the other's field name, and every consumer would
// then be wrong in the same direction — silently, on a wire byte that parses.
func TestDifference_Byte19MeansDataModeOnThe590PairAndLockoutOnThe480(t *testing.T) {
	if got := layout590SG().Byte19(); got != Byte19DataMode {
		t.Errorf("TS-590SG byte 19 = %v, want Byte19DataMode (590:1546-1548)", got)
	}
	if got := layout590S().Byte19(); got != Byte19DataMode {
		t.Errorf("TS-590S byte 19 = %v, want Byte19DataMode (590:1546-1548)", got)
	}
	if got := layout480().Byte19(); got != Byte19Lockout {
		t.Errorf("TS-480 byte 19 = %v, want Byte19Lockout (480:962)", got)
	}

	// Both values parse on both rows — which is exactly why the axis, and
	// not a frame, is the only thing that can carry the difference.
	for _, b := range []string{"0", "1"} {
		f := answer590()
		f.p6 = b
		for _, tt := range []struct {
			name   string
			layout Layout
		}{{"TS-590SG", layout590SG()}, {"TS-480", layout480()}} {
			rec, err := tt.layout.ParseMRAnswer(f.frame(t))
			if err != nil {
				t.Fatalf("%s refused byte 19 = %q: %v", tt.name, b, err)
			}
			if rec.Byte19 != b[0] {
				t.Errorf("%s Byte19 = %q, want the raw wire byte %q", tt.name, rec.Byte19, b)
			}
		}
	}

	// And the refusal each row produces names its OWN meaning, so a layout
	// that borrowed the other's axis would report the wrong field.
	f := answer590()
	f.p6 = "2"
	for _, tt := range []struct {
		name     string
		layout   Layout
		wantText string
	}{
		{"TS-590SG", layout590SG(), "Byte19DataMode"},
		{"TS-480", layout480(), "Byte19Lockout"},
	} {
		_, err := tt.layout.ParseMRAnswer(f.frame(t))
		if err == nil {
			t.Fatalf("%s accepted byte 19 = '2'", tt.name)
		}
		if !strings.Contains(err.Error(), tt.wantText) {
			t.Errorf("%s refusal = %q, want it to name %s", tt.name, err, tt.wantText)
		}
	}
}

// TestDifference_Byte28IsALiveFilterOnThe590PairAndAConstantOnThe480, and it
// is the ONE axis on which the two 590 ROWS differ as well.
//
// The 590 book prints one P11 legend for both rows — "0: FILTER A / 1:
// FILTER B" (590:1560-1563) — then scopes a sentence to one of them: "In
// firmware version 1.xx of TS-590S, always \"0\"." (590:1564). So BOTH 590
// rows must accept '1' on a read, because an S at firmware 2.00 or later
// answers with it, and the difference between the rows is what a WRITE may
// carry — the driver's question, not this codec's. The 480 prints "Always 0
// for the TS-480." (480:973) instead, which puts that byte in its
// printed-fixed set and makes '1' a refusal there.
func TestDifference_Byte28IsALiveFilterOnThe590PairAndAConstantOnThe480(t *testing.T) {
	if got := layout590SG().Byte28(); got != Byte28FilterLive {
		t.Errorf("TS-590SG byte 28 = %v, want Byte28FilterLive (590:1560-1563)", got)
	}
	if got := layout590S().Byte28(); got != Byte28FilterEither {
		t.Errorf("TS-590S byte 28 = %v, want Byte28FilterEither (590:1564)", got)
	}
	if got := layout480().Byte28(); got != Byte28FixedZero {
		t.Errorf("TS-480 byte 28 = %v, want Byte28FixedZero (480:973)", got)
	}

	f := answer590()
	f.p11 = "1" // FILTER B
	if _, err := layout590SG().ParseMRAnswer(f.frame(t)); err != nil {
		t.Errorf("the TS-590SG refused FILTER B at byte 28: %v", err)
	}
	if _, err := layout590S().ParseMRAnswer(f.frame(t)); err != nil {
		t.Errorf("the TS-590S refused FILTER B at byte 28, and 590:1564's \"always 0\" is scoped to firmware 1.xx: %v", err)
	}
	if _, err := layout480().ParseMRAnswer(f.frame(t)); err == nil {
		t.Error("the TS-480 accepted '1' at byte 28, where its book prints \"Always 0 for the TS-480.\" (480:973)")
	}

	// The same difference stated the other way: byte 28 is a member of the
	// 480's printed-fixed set and of neither 590 row's.
	if positionIsPrintedFixed(layout590SG(), 28) || positionIsPrintedFixed(layout590S(), 28) {
		t.Error("a 590 row declares byte 28 printed-fixed, where its book prints a live FILTER A/B selection")
	}
	if !positionIsPrintedFixed(layout480(), 28) {
		t.Error("the TS-480 does not declare byte 28 printed-fixed, where its book prints \"Always 0\"")
	}
}

// TestDifference_Bytes3940AreAnFMFlagOnThe590PairAndAStepIndexOnThe480. The
// 590 pair print exactly two values, "00: FM Normal" and "01: FM Narrow"
// (590:1569-1571); the 480 prints "Step size. Refer to the ST command."
// (480:979), whose own legend runs to 09 in AM and FM (480:1494-1500). So
// "05" is a legal answer on one radio and a byte no sentence in the other's
// book reaches — and "01" parses on both while meaning different things,
// which is what the FM-N synthesis turns on (RecordModeName, mode.go).
func TestDifference_Bytes3940AreAnFMFlagOnThe590PairAndAStepIndexOnThe480(t *testing.T) {
	if got := layout590SG().Byte3940(); got != Byte3940FMNarrowFlag {
		t.Errorf("TS-590SG bytes 39-40 = %v, want Byte3940FMNarrowFlag (590:1569-1571)", got)
	}
	if got := layout480().Byte3940(); got != Byte3940StepIndex {
		t.Errorf("TS-480 bytes 39-40 = %v, want Byte3940StepIndex (480:979)", got)
	}

	f := answer590()
	f.p14 = "05"
	if _, err := layout480().ParseMRAnswer(f.frame(t)); err != nil {
		t.Errorf("the TS-480 refused step index 05, which ST prints in its AM/FM legend (480:1494-1500): %v", err)
	}
	if _, err := layout590SG().ParseMRAnswer(f.frame(t)); err == nil {
		t.Error("the TS-590SG accepted \"05\" at bytes 39-40, where its book prints only \"00\" and \"01\"")
	}

	// And the same two bytes name different things: "01" is FM Narrow on the
	// 590 pair and step 1 on the 480, so only one of the two publishes a
	// narrow name for it.
	narrow := Record{Mode: ModeFM, Byte3940: "01"}
	if name, _ := layout590SG().RecordModeName(narrow); name != "FM-N" {
		t.Errorf("TS-590SG named an FM record with P14 \"01\" %q, want %q", name, "FM-N")
	}
	if name, _ := layout480().RecordModeName(narrow); name != "FM" {
		t.Errorf("TS-480 named an FM record with step index 1 %q, want %q — its P14 says nothing about bandwidth", name, "FM")
	}
}

// TestDifference_Byte41IsTheLockoutOnThe590PairAndAConstantOnThe480, which is
// the other half of the byte-19 swap: each book spends ONE byte on the
// channel lockout and hard-wires the other. "0: Channel Lockout OFF / 1:
// Channel Lockout ON" (590:1572-1574) against "Always 0 for the TS-480."
// (480:982).
func TestDifference_Byte41IsTheLockoutOnThe590PairAndAConstantOnThe480(t *testing.T) {
	if got := layout590SG().Byte41(); got != Byte41Lockout {
		t.Errorf("TS-590SG byte 41 = %v, want Byte41Lockout (590:1572-1574)", got)
	}
	if got := layout480().Byte41(); got != Byte41FixedZero {
		t.Errorf("TS-480 byte 41 = %v, want Byte41FixedZero (480:982)", got)
	}

	f := answer590()
	f.p15 = "1" // Channel Lockout ON
	if _, err := layout590SG().ParseMRAnswer(f.frame(t)); err != nil {
		t.Errorf("the TS-590SG refused Channel Lockout ON at byte 41: %v", err)
	}
	if _, err := layout480().ParseMRAnswer(f.frame(t)); err == nil {
		t.Error("the TS-480 accepted '1' at byte 41, where its book prints \"Always 0 for the TS-480.\" (480:982) and carries the lockout at byte 19")
	}

	if positionIsPrintedFixed(layout590SG(), 41) {
		t.Error("the TS-590SG declares byte 41 printed-fixed, where its book prints the channel lockout")
	}
	if !positionIsPrintedFixed(layout480(), 41) {
		t.Error("the TS-480 does not declare byte 41 printed-fixed, where its book prints \"Always 0\"")
	}
}

// TestDifference_Byte4IsTheHundredsDigitOnThe590PairAndAConstantOnThe480.
// P2 is "Channel number (refer to the MC command)" on the 590 pair
// (590:1539-1540), and MC prints the space convention at 590:1332-1337; the
// 480 prints "Always 0 for the TS-480." (480:953) and has no bank digit in
// this record at all. So a digit above '0' there reaches the 590 pair's
// section and extension classes and is a refusal on the 480, and a SPACE —
// the answer form for most of a 590's channels — is legal on one radio and
// not on the other.
func TestDifference_Byte4IsTheHundredsDigitOnThe590PairAndAConstantOnThe480(t *testing.T) {
	if got := layout590SG().P2Policy(); got != P2HundredsDigit {
		t.Errorf("TS-590SG byte 4 = %v, want P2HundredsDigit (590:1539-1540)", got)
	}
	if got := layout480().P2Policy(); got != P2FixedZero {
		t.Errorf("TS-480 byte 4 = %v, want P2FixedZero (480:953)", got)
	}

	hundreds := answer590()
	hundreds.p2, hundreds.p3 = "1", "03"
	rec, err := layout590SG().ParseMRAnswer(hundreds.frame(t))
	if err != nil {
		t.Fatalf("the TS-590SG refused byte 4 = '1': %v", err)
	}
	if rec.Slot.Number() != 103 {
		t.Errorf("TS-590SG slot = %v, want 103 — byte 4 is the hundreds digit there", rec.Slot)
	}
	if _, err := layout480().ParseMRAnswer(hundreds.frame(t)); err == nil {
		t.Error("the TS-480 accepted byte 4 = '1', where its book prints \"Always 0 for the TS-480.\" (480:953)")
	}

	space := answer590()
	space.p2 = " "
	if _, err := layout590SG().ParseMRAnswer(space.frame(t)); err != nil {
		t.Errorf("the TS-590SG refused a space at byte 4, which is the answer form MC prints for every channel below 100 (590:1334-1337): %v", err)
	}
	if _, err := layout480().ParseMRAnswer(space.frame(t)); err == nil {
		t.Error("the TS-480 accepted a space at byte 4, where its book hard-wires '0'")
	}

	// The builders state the same difference in the other direction: the
	// 480's P2 is always '0', and the 590 pair's carries the hundreds.
	sg, err := layout590SG().BuildMRRead(mustSlot(t, layout590SG(), 103, ScanLower))
	if err != nil {
		t.Fatalf("BuildMRRead: %v", err)
	}
	if got := sg.Bytes()[recP2Off]; got != '1' {
		t.Errorf("TS-590SG MR read byte 4 = %q, want '1' for slot 103", got)
	}
	ts480 := layout480()
	r480, err := ts480.BuildMRRead(mustSlot(t, ts480, 99, ScanHalfNone))
	if err != nil {
		t.Fatalf("BuildMRRead: %v", err)
	}
	if got := r480.Bytes()[recP2Off]; got != '0' {
		t.Errorf("TS-480 MR read byte 4 = %q, want '0' for every slot (480:953)", got)
	}
}

// TestDifference_TheToneModeLegendHasFourValuesOnThe590PairAndThreeOnThe480.
// "3: Cross Tone ON" is the 590 pair's alone (590:1549-1553); the 480's P7
// legend stops at 2 (480:964) and its book has no cross tone anywhere. A
// package-level range check would admit a cross-tone record on a radio whose
// book has no such setting, and the record would then be written back to it.
func TestDifference_TheToneModeLegendHasFourValuesOnThe590PairAndThreeOnThe480(t *testing.T) {
	if got := layout590SG().ToneModes(); got != ToneModesFour {
		t.Errorf("TS-590SG tone modes = %v, want ToneModesFour (590:1549-1553)", got)
	}
	if got := layout480().ToneModes(); got != ToneModesThree {
		t.Errorf("TS-480 tone modes = %v, want ToneModesThree (480:964)", got)
	}

	// The three both books print are legal on both rows; the fourth is legal
	// on one and refused on the other, on the wire and at the builder.
	for _, tm := range []ToneMode{ToneModeOff, ToneModeTone, ToneModeCTCSS} {
		if !layout590SG().ValidToneMode(tm) || !layout480().ValidToneMode(tm) {
			t.Errorf("tone mode %q is not valid on both rows, and both books print it", byte(tm))
		}
	}
	if !layout590SG().ValidToneMode(ToneModeCross) {
		t.Error("the TS-590SG refused Cross Tone ON, which 590:1553 prints")
	}
	if layout480().ValidToneMode(ToneModeCross) {
		t.Error("the TS-480 admitted Cross Tone ON, and its P7 legend stops at 2 (480:964)")
	}

	f := answer590()
	f.p7 = "3"
	if _, err := layout590SG().ParseMRAnswer(f.frame(t)); err != nil {
		t.Errorf("the TS-590SG refused a cross-tone record: %v", err)
	}
	if _, err := layout480().ParseMRAnswer(f.frame(t)); err == nil {
		t.Error("the TS-480 parsed a cross-tone record, which its book does not describe")
	}

	sgRec := populatedRecord(mustSlot(t, layout590SG(), 7, ScanHalfNone))
	sgRec.ToneMode = ToneModeCross
	if _, err := layout590SG().BuildMWSet(sgRec); err != nil {
		t.Errorf("the TS-590SG refused to build a cross-tone write: %v", err)
	}
	rec480 := populatedRecord(mustSlot(t, layout480(), 7, ScanHalfNone))
	rec480.ToneMode = ToneModeCross
	if _, err := layout480().BuildMWSet(rec480); err == nil {
		t.Error("the TS-480 built a cross-tone write, and its book prints no such value")
	}
}

// positionIsPrintedFixed reports whether l declares the 1-indexed position
// pos hard-wired, which is the other half of three of the six axes: byte 4,
// byte 28 and byte 41 each carry a meaning on one radio and a printed
// constant on the other.
func positionIsPrintedFixed(l Layout, pos int) bool {
	for _, ff := range l.PrintedFixed() {
		if pos >= ff.Pos && pos <= ff.end() {
			return true
		}
	}
	return false
}
