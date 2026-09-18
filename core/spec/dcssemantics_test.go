// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"strings"
	"testing"
)

// --- S0.4: ToneModeSemantics gains the two DCS members ---
//
// The FT-991A's P8 legend prints five CTCSS states, "3: DCS ENC/DEC" and
// "4: DCS ENC" among them. Validate forbids two ToneModes entries sharing
// Semantics with no canonical one marked, and this radio's five-member
// vocabulary has no duplicate — but before the DCS members existed at
// all, a five-member vocabulary was not expressible: the two DCS states
// would have had to borrow ToneModeCTCSSSquelch and ToneModeCTCSS.
//
// The change is ADDITIVE. The two members are APPENDED after every other
// declared ToneModeSemantics constant (including the Icom-only ones this
// radio never uses), so no existing constant's value moves and no
// serialised form changes; ToneModes is a capability VALUE rather than a
// schema field, and codeplug carries the state as an opaque string. No
// registered model's vocabulary moves.
//
// This file used to test the Yaesu-only ToneSemantics vocabulary
// (S0.4-era); ToneSemantics/ToneState were deleted once the Yaesu and
// Icom/Kenwood tone vocabularies unified onto ToneMode/ToneModeSemantics,
// so these tests now exercise the same properties on that shared type.

// TestToneModeSemantics_DCSMembersAreAppendedLast pins the constants'
// ORDER, which is the whole of the no-break claim: an insertion anywhere
// above would renumber an existing constant and silently change what an
// already-declared value means. The two DCS members sit AFTER the five
// Icom-only constants (ToneModeCTCSSRxSquelch/ToneModeDTCS/ToneModeCross)
// too, not merely after the family three, because they were appended to
// the ALREADY-SHARED enum, not inserted into a Yaesu-only one.
func TestToneModeSemantics_DCSMembersAreAppendedLast(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  ToneModeSemantics
		want int
	}{
		{"ToneModeUnspecified", ToneModeUnspecified, 0},
		{"ToneModeOff", ToneModeOff, 1},
		{"ToneModeCTCSS", ToneModeCTCSS, 2},
		{"ToneModeCTCSSSquelch", ToneModeCTCSSSquelch, 3},
		{"ToneModeCTCSSRxSquelch", ToneModeCTCSSRxSquelch, 4},
		{"ToneModeDTCS", ToneModeDTCS, 5},
		{"ToneModeCross", ToneModeCross, 6},
		{"ToneModeDCSEncodeDecode", ToneModeDCSEncodeDecode, 7},
		{"ToneModeDCSEncode", ToneModeDCSEncode, 8},
	} {
		if int(tc.got) != tc.want {
			t.Errorf("%s = %d, want %d — the DCS members are APPENDED LAST; moving an existing one changes what every declared value means", tc.name, int(tc.got), tc.want)
		}
	}
}

// TestValidate_AcceptsAFiveStateVocabulary is the axis's point: the
// FT-991A's five states, each with its own Semantics, must pass.
func TestValidate_AcceptsAFiveStateVocabulary(t *testing.T) {
	c := minimalCaps()
	c.ToneModes = fiveStateVocabulary()
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want a five-state vocabulary accepted", err)
	}
}

// TestValidate_StillRefusesADuplicatedSemanticsWithNoCanonical holds the
// rule the widening must not weaken: five members are legal, but two of
// them meaning the same thing needs exactly one marked Canonical (E5) —
// same rule every ToneModes duplication gets, not a special case for the
// DCS members.
func TestValidate_StillRefusesADuplicatedSemanticsWithNoCanonical(t *testing.T) {
	c := minimalCaps()
	states := fiveStateVocabulary()
	states[4].Semantics = ToneModeDCSEncodeDecode // now two states share it, neither Canonical
	c.ToneModes = states

	err := c.Validate()
	if err == nil {
		t.Fatal("Validate() accepted two ToneModes expressing the same semantics with no canonical one marked")
	}
	if !strings.Contains(err.Error(), "no canonical one is marked") {
		t.Errorf("Validate() = %v, want a complaint naming the missing canonical", err)
	}
}

// TestValidate_StillRefusesUnspecifiedSemantics holds the other rule: the
// zero value is still not a semantic, and widening the set of MEANINGFUL
// members must not have widened the set of admissible ones.
func TestValidate_StillRefusesUnspecifiedSemantics(t *testing.T) {
	c := minimalCaps()
	states := fiveStateVocabulary()
	states[3].Semantics = ToneModeUnspecified
	c.ToneModes = states

	if err := c.Validate(); err == nil {
		t.Fatal("Validate() accepted a ToneMode whose Semantics was never set")
	}
	// And a value past the end of the vocabulary.
	states = fiveStateVocabulary()
	states[3].Semantics = ToneModeSemantics(99)
	c.ToneModes = states
	if err := c.Validate(); err == nil {
		t.Fatal("Validate() accepted a ToneMode with an undeclared Semantics value")
	}
}

// TestNeedsTone_IsNotWidenedByTheDCSMembers pins the deliberate
// non-change vocab.go's own doc comment promises.
//
// A DCS state needs a CODE, not a CTCSS tone, and the code is not in the
// FT-991A's memory record. Widening NeedsTxTone/NeedsRxTone would make
// core/codeplug's validator demand a FieldCTCSSTone that radio never
// carries — its only consumer raises a warning from exactly this
// predicate. NeedsDTCS must stay false too: that is the reason the two
// members are their OWN constants rather than a reuse of ToneModeDTCS,
// whose NeedsDTCS() is true and would make the validator demand a
// FieldDTCSCode the FT-991A's P8 byte does not carry either.
func TestNeedsTone_IsNotWidenedByTheDCSMembers(t *testing.T) {
	for _, tc := range []struct {
		semantics ToneModeSemantics
		wantTx    bool
		wantRx    bool
		wantDTCS  bool
	}{
		{ToneModeOff, false, false, false},
		{ToneModeCTCSS, true, false, false},
		{ToneModeCTCSSSquelch, true, true, false},
		{ToneModeDCSEncodeDecode, false, false, false},
		{ToneModeDCSEncode, false, false, false},
	} {
		m := ToneMode{Value: "X", Semantics: tc.semantics}
		if got := m.NeedsTxTone(); got != tc.wantTx {
			t.Errorf("ToneMode{Semantics: %d}.NeedsTxTone() = %t, want %t", int(tc.semantics), got, tc.wantTx)
		}
		if got := m.NeedsRxTone(); got != tc.wantRx {
			t.Errorf("ToneMode{Semantics: %d}.NeedsRxTone() = %t, want %t", int(tc.semantics), got, tc.wantRx)
		}
		if got := m.NeedsDTCS(); got != tc.wantDTCS {
			t.Errorf("ToneMode{Semantics: %d}.NeedsDTCS() = %t, want %t — a DCS state needs a CODE, which is not a field of this record, and reusing ToneModeDTCS would wrongly report true here", int(tc.semantics), got, tc.wantDTCS)
		}
	}
}

// TestStandardToneModes_FamilyThreeUnchanged holds the family vocabulary.
// The FT-991A supplies its own five-member list as a capability VALUE; no
// registered model's ToneModes moves, and the three wire spellings this
// helper returns are byte-identical to the pre-unification
// StandardCTCSSStates().
func TestStandardToneModes_FamilyThreeUnchanged(t *testing.T) {
	got := StandardToneModes()
	want := []ToneMode{
		{Value: "OFF", Semantics: ToneModeOff},
		{Value: "ENC-DEC", Semantics: ToneModeCTCSSSquelch},
		{Value: "ENC", Semantics: ToneModeCTCSS},
	}
	if len(got) != len(want) {
		t.Fatalf("StandardToneModes() has %d entries, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("StandardToneModes()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// fiveStateVocabulary is the FT-991A's P8 legend as a spec vocabulary: the
// three CTCSS states plus the two DCS ones, each with its own Semantics.
// A TEST FIXTURE ONLY — no registered model declares it until Stage 2.
func fiveStateVocabulary() []ToneMode {
	return []ToneMode{
		{Value: "OFF", Semantics: ToneModeOff},
		{Value: "ENC-DEC", Semantics: ToneModeCTCSSSquelch},
		{Value: "ENC", Semantics: ToneModeCTCSS},
		{Value: "DCS-ENC-DEC", Semantics: ToneModeDCSEncodeDecode},
		{Value: "DCS-ENC", Semantics: ToneModeDCSEncode},
	}
}
