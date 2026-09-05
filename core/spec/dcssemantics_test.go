// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"strings"
	"testing"
)

// --- S0.4: ToneSemantics gains the two DCS members ---
//
// The FT-991A's P8 legend prints five CTCSS states, "3: DCS ENC/DEC" and
// "4: DCS ENC" among them. Validate forbids two CTCSSStates entries sharing
// Semantics, and ToneSemantics had three meaningful members, so a
// five-member vocabulary was not expressible at all: the two DCS states
// would have had to borrow ToneEncodeDecode and ToneEncode, and Validate
// would then have refused the profile for a duplication that is really a
// gap in this vocabulary.
//
// The change is ADDITIVE. The two members are APPENDED, so no existing
// constant's value moves and no serialised form changes; CTCSSStates is a
// capability VALUE rather than a schema field, and codeplug carries the
// state as an opaque string. No registered model's vocabulary moves.

// TestToneSemantics_DCSMembersAreAppended pins the constants' ORDER, which
// is the whole of the no-break claim: an insertion anywhere above would
// renumber ToneEncodeDecode and silently change what an existing declared
// value means.
func TestToneSemantics_DCSMembersAreAppended(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  ToneSemantics
		want int
	}{
		{"ToneSemanticsUnspecified", ToneSemanticsUnspecified, 0},
		{"ToneOff", ToneOff, 1},
		{"ToneEncode", ToneEncode, 2},
		{"ToneEncodeDecode", ToneEncodeDecode, 3},
		{"ToneDCSEncodeDecode", ToneDCSEncodeDecode, 4},
		{"ToneDCSEncode", ToneDCSEncode, 5},
	} {
		if int(tc.got) != tc.want {
			t.Errorf("%s = %d, want %d — the DCS members are APPENDED; moving an existing one changes what every declared value means", tc.name, int(tc.got), tc.want)
		}
	}
}

// TestValidate_AcceptsAFiveStateVocabulary is the axis's point: the
// FT-991A's five states, each with its own Semantics, must pass.
func TestValidate_AcceptsAFiveStateVocabulary(t *testing.T) {
	c := minimalCaps()
	c.CTCSSStates = fiveStateVocabulary()
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want a five-state vocabulary accepted", err)
	}
}

// TestValidate_StillRefusesADuplicatedSemantics holds the rule the widening
// must not weaken: five members are legal, five members two of which mean
// the same thing are not.
func TestValidate_StillRefusesADuplicatedSemantics(t *testing.T) {
	c := minimalCaps()
	states := fiveStateVocabulary()
	states[4].Semantics = ToneDCSEncodeDecode // now two states share it
	c.CTCSSStates = states

	err := c.Validate()
	if err == nil {
		t.Fatal("Validate() accepted two CTCSS states expressing the same semantics")
	}
	if !strings.Contains(err.Error(), "same semantics") {
		t.Errorf("Validate() = %v, want a complaint naming the duplicated semantics", err)
	}
}

// TestValidate_StillRefusesUnspecifiedSemantics holds the other rule: the
// zero value is still not a semantic, and widening the set of MEANINGFUL
// members must not have widened the set of admissible ones.
func TestValidate_StillRefusesUnspecifiedSemantics(t *testing.T) {
	c := minimalCaps()
	states := fiveStateVocabulary()
	states[3].Semantics = ToneSemanticsUnspecified
	c.CTCSSStates = states

	if err := c.Validate(); err == nil {
		t.Fatal("Validate() accepted a CTCSS state whose Semantics was never set")
	}
	// And a value past the end of the vocabulary.
	states = fiveStateVocabulary()
	states[3].Semantics = ToneSemantics(99)
	c.CTCSSStates = states
	if err := c.Validate(); err == nil {
		t.Fatal("Validate() accepted a CTCSS state with an undeclared Semantics value")
	}
}

// TestRequiresTone_IsNotWidenedByTheDCSMembers pins the deliberate
// non-change.
//
// A DCS state needs a CODE, not a CTCSS tone, and the code is not in this
// record. Widening RequiresTone would make codeplug.Validate demand a
// FieldCTCSSTone the FT-991A never carries — its only consumer,
// core/codeplug/validate.go, raises a warning from exactly this predicate.
func TestRequiresTone_IsNotWidenedByTheDCSMembers(t *testing.T) {
	for _, tc := range []struct {
		semantics ToneSemantics
		want      bool
	}{
		{ToneOff, false},
		{ToneEncode, true},
		{ToneEncodeDecode, true},
		{ToneDCSEncodeDecode, false},
		{ToneDCSEncode, false},
	} {
		if got := (ToneState{Value: "X", Semantics: tc.semantics}.RequiresTone()); got != tc.want {
			t.Errorf("ToneState{Semantics: %d}.RequiresTone() = %t, want %t — a DCS state needs a CODE, which is not a field of this record", int(tc.semantics), got, tc.want)
		}
	}
}

// TestStandardCTCSSStates_IsUNCHANGED holds the family vocabulary. The
// FT-991A supplies its own five-member list as a capability VALUE; no
// registered model's CTCSSStates moves.
func TestStandardCTCSSStates_IsUNCHANGED(t *testing.T) {
	got := StandardCTCSSStates()
	want := []ToneState{
		{Value: "OFF", Semantics: ToneOff},
		{Value: "ENC-DEC", Semantics: ToneEncodeDecode},
		{Value: "ENC", Semantics: ToneEncode},
	}
	if len(got) != len(want) {
		t.Fatalf("StandardCTCSSStates() has %d entries, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("StandardCTCSSStates()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// fiveStateVocabulary is the FT-991A's P8 legend as a spec vocabulary: the
// three CTCSS states plus the two DCS ones, each with its own Semantics.
// A TEST FIXTURE ONLY — no registered model declares it until Stage 2.
func fiveStateVocabulary() []ToneState {
	return []ToneState{
		{Value: "OFF", Semantics: ToneOff},
		{Value: "ENC-DEC", Semantics: ToneEncodeDecode},
		{Value: "ENC", Semantics: ToneEncode},
		{Value: "DCS-ENC-DEC", Semantics: ToneDCSEncodeDecode},
		{Value: "DCS-ENC", Semantics: ToneDCSEncode},
	}
}
