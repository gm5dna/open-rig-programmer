// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import (
	"strings"
	"testing"
)

// TestBuildMCRead_IsTheThreeByteFrameBothBooksPrint. "M C ;" is the whole
// Read chart on both radios (590:1337, 480:834).
func TestBuildMCRead_IsTheThreeByteFrameBothBooksPrint(t *testing.T) {
	for _, l := range []Layout{layout590SG(), layout590S(), layout480()} {
		cmd, err := l.BuildMCRead()
		if err != nil {
			t.Fatalf("%s: BuildMCRead: %v", l.Model(), err)
		}
		if got := string(cmd.Bytes()); got != "MC;" {
			t.Errorf("%s: BuildMCRead built %q, want %q (590:1337, 480:834)", l.Model(), got, "MC;")
		}
	}
}

// TestBuildMCSet_EmitsAZeroHundredsDigitOnEveryRow is A10's build half
// applied to MC itself, which is the chart the convention is PRINTED in
// (590:1332-1337): both '0' and a space are legal for a channel below 100,
// this codec always emits '0', and the 480's byte 3 is "Always 0" anyway
// (480:827).
func TestBuildMCSet_EmitsAZeroHundredsDigitOnEveryRow(t *testing.T) {
	for _, tc := range []struct {
		l     Layout
		slot  int
		frame string
	}{
		{layout590SG(), 0, "MC000;"},
		{layout590SG(), 3, "MC003;"},
		{layout590SG(), 99, "MC099;"},
		{layout590S(), 3, "MC003;"},
		{layout480(), 0, "MC000;"},
		{layout480(), 90, "MC090;"},
		{layout480(), 99, "MC099;"},
	} {
		cmd, err := tc.l.BuildMCSet(mustSlot(t, tc.l, tc.slot, ScanHalfNone))
		if err != nil {
			t.Errorf("%s: BuildMCSet(%d): %v", tc.l.Model(), tc.slot, err)
			continue
		}
		if got := string(cmd.Bytes()); got != tc.frame {
			t.Errorf("%s: BuildMCSet(%d) built %q, want %q", tc.l.Model(), tc.slot, got, tc.frame)
		}
		if len(cmd.Bytes()) != MCSetLen {
			t.Errorf("%s: BuildMCSet(%d) built %d bytes, want %d", tc.l.Model(), tc.slot, len(cmd.Bytes()), MCSetLen)
		}
	}
}

// TestBuildMCSet_IsNarrowedToOrdinaryMemory is A16 (L-DEC-2), and it is the
// one refusal in this file that is a DECISION rather than a document fact:
// 590:1345-1347 says the section and extension numbers are selectable, and
// this milestone declines to select them because recalling a channel changes
// the radio's operating state.
//
// The 480 has no second class at all, so the pin is a 590 one; the positive
// control is the ordinary channel beside it, which must still build.
func TestBuildMCSet_IsNarrowedToOrdinaryMemory(t *testing.T) {
	sg := layout590SG()

	if _, err := sg.BuildMCSet(mustSlot(t, sg, 100, ScanLower)); err == nil {
		t.Error("BuildMCSet recalled a section-defined channel; A16 narrows the Set domain to ordinary memory")
	} else if !strings.Contains(err.Error(), "A16") {
		t.Errorf("the section-channel refusal reads %v, and it should name A16 — the entry a later widening has to reopen", err)
	}
	if _, err := sg.BuildMCSet(mustSlot(t, sg, 110, ScanHalfNone)); err == nil {
		t.Error("BuildMCSet recalled an extension channel; A16 narrows the Set domain to ordinary memory")
	}
	// The positive control: the refusal is not vacuous.
	if _, err := sg.BuildMCSet(mustSlot(t, sg, 99, ScanHalfNone)); err != nil {
		t.Errorf("BuildMCSet refused ordinary channel 099: %v", err)
	}
}

// TestBuildMCSet_RefusesASlotFromAnotherLayoutAndAnUnresolvedOne. A Slot is
// a value and may have been minted under another radio's slot space; a Set
// is a side-effecting recall, so trusting the class the value carries would
// recall a channel on a radio that does not have one.
func TestBuildMCSet_RefusesASlotFromAnotherLayoutAndAnUnresolvedOne(t *testing.T) {
	sg, s := layout590SG(), layout590S()

	// 110 is the SG's extension space and is outside the S's (A12).
	if _, err := s.BuildMCSet(mustSlot(t, sg, 110, ScanHalfNone)); err == nil {
		t.Error("the TS-590S built an MC Set for slot 110, which A12 leaves outside its slot space")
	}
	if _, err := layout480().BuildMCSet(mustSlot(t, sg, 3, ScanHalfNone)); err != nil {
		// Slot 003 is ordinary memory on both rows, so this one must build:
		// the refusal above is about the SPACE, not about provenance alone.
		t.Errorf("the TS-480 refused a slot number both rows hold: %v", err)
	}
	if _, err := sg.BuildMCSet(Slot{}); err == nil {
		t.Error("BuildMCSet accepted a slot that was never resolved against a layout")
	}
}

// TestParseMCAnswer_TheAnswerDomainIsTheWholePrintedSpace. The Set is
// narrowed and the Answer is not, for the reason core/cat gives: a radio
// sitting on a section channel reached from the front panel will answer with
// it. So the two directions have DIFFERENT domains and this is the pin that
// says so.
func TestParseMCAnswer_TheAnswerDomainIsTheWholePrintedSpace(t *testing.T) {
	sg := layout590SG()
	for _, tc := range []struct {
		frame  string
		number int
		class  SlotClass
	}{
		{"MC 03;", 3, SlotMemory},  // the printed answer form below 100
		{"MC003;", 3, SlotMemory},  // and the digit the Set chart also admits
		{"MC099;", 99, SlotMemory}, //
		{"MC100;", 100, SlotScan},  // P00, a section-defined channel
		{"MC109;", 109, SlotScan},  // P09
		{"MC110;", 110, SlotExtension},
		{"MC119;", 119, SlotExtension},
	} {
		got, err := sg.ParseMCAnswer([]byte(tc.frame))
		if err != nil {
			t.Errorf("ParseMCAnswer(%q): %v", tc.frame, err)
			continue
		}
		if got.Number != tc.number || got.Class != tc.class {
			t.Errorf("ParseMCAnswer(%q) = %+v, want number %d class %v", tc.frame, got, tc.number, tc.class)
		}
	}

	// And every one of the three that the Set direction refuses is one the
	// Answer direction accepts — the property, not the examples.
	for _, n := range []int{100, 110} {
		frame := []byte("MC" + string(rune('0'+n/100)) + string(rune('0'+n/10%10)) + string(rune('0'+n%10)) + ";")
		if _, err := sg.ParseMCAnswer(frame); err != nil {
			t.Errorf("ParseMCAnswer refused %q, which the Set direction refuses for a different reason entirely", frame)
		}
	}
}

// TestParseMCAnswer_IsBoundedByTHISRowsSlotSpace, both ways: 110 is the SG's
// and the S's book never gives the S one (A12), and the 480's space stops at
// 99 (480:830).
func TestParseMCAnswer_IsBoundedByTHISRowsSlotSpace(t *testing.T) {
	if _, err := layout590S().ParseMCAnswer([]byte("MC110;")); err == nil {
		t.Error("the TS-590S parsed an MC answer naming 110, which the book gives the SG (590:1346-1347, A12)")
	}
	if _, err := layout480().ParseMCAnswer([]byte("MC100;")); err == nil {
		t.Error("the TS-480 parsed an MC answer naming 100, and its channel number is P2's two digits alone (480:830)")
	}
	if _, err := layout480().ParseMCAnswer([]byte("MC 03;")); err == nil {
		t.Error("the TS-480 parsed a space in byte 3, which its book prints \"Always 0\" (480:827)")
	}
	if _, err := layout480().ParseMCAnswer([]byte("MC003;")); err != nil {
		t.Errorf("the TS-480 refused its own printed answer form: %v", err)
	}
}

// TestParseMCAnswer_RefusesAMalformedFrame. Six bytes, "MC" prefix,
// terminator last, two digits in the low field — MC prints "the first digit
// is 0" below 10 (590:1342-1343), so those two bytes are always digits and
// the space convention is byte 3's alone.
func TestParseMCAnswer_RefusesAMalformedFrame(t *testing.T) {
	for _, frame := range []string{
		"MC03;",   // five bytes
		"MC0033;", // seven bytes
		"MC;",     // the READ frame, which is not an answer
		"MX003;",  // the wrong command name
		"MC003!",  // no terminator
		"MC0 3;",  // a space in the two-digit field
		"MC0A3;",  // a non-digit in the two-digit field
		"MC\x0003;",
	} {
		if _, err := layout590SG().ParseMCAnswer([]byte(frame)); err == nil {
			t.Errorf("ParseMCAnswer accepted %q", frame)
		}
	}
}

// TestMC_ZeroLayoutBuildsAndParsesNothing. "MC;" consults no radio datum at
// all, so without the Configured guard a zero Layout would emit a
// side-effecting command's read frame on behalf of nobody.
func TestMC_ZeroLayoutBuildsAndParsesNothing(t *testing.T) {
	var l Layout
	if _, err := l.BuildMCRead(); err == nil {
		t.Error("a zero Layout built an MC read")
	}
	if _, err := l.BuildMCSet(mustSlot(t, layout590SG(), 3, ScanHalfNone)); err == nil {
		t.Error("a zero Layout built an MC Set")
	}
	if _, err := l.ParseMCAnswer([]byte("MC003;")); err == nil {
		t.Error("a zero Layout parsed an MC answer")
	}
}

// TestMCChannel_StringIsThreeDigitsAndCarriesNoHalf. An MC answer names a
// channel NUMBER and carries no byte that could say which of a section
// channel's two frequencies is meant, which is exactly why the parser
// returns an MCChannel rather than a Slot.
func TestMCChannel_StringIsThreeDigitsAndCarriesNoHalf(t *testing.T) {
	got, err := layout590SG().ParseMCAnswer([]byte("MC100;"))
	if err != nil {
		t.Fatalf("ParseMCAnswer: %v", err)
	}
	if got.String() != "100" {
		t.Errorf("MCChannel.String() = %q, want %q", got.String(), "100")
	}
	if strings.ContainsAny(got.String(), "LU") {
		t.Errorf("MCChannel.String() = %q and names a section channel's half, which no MC frame carries", got.String())
	}
}
