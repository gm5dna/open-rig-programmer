// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// --- FT-991A Stage 0 (S0.4): a five-state CTCSS vocabulary ---
//
// The grid's CTCSS-state picker is built from spec.Capabilities.CTCSSStates
// directly, so a radio whose P8 legend prints five states must offer five
// options in its own order. No registered model declares such a vocabulary
// until Stage 2, which is why this is stated over the extractor rather than
// over GetUISpec: the property has to be provable before the radio exists.
//
// TestGetUISpec_Disconnected_StaticBaseline holds the other half — that the
// FT-710's own three-member list still comes out as [OFF ENC-DEC ENC].
func TestCTCSSStateValues_PassesEveryDeclaredStateThrough(t *testing.T) {
	states := []spec.ToneState{
		{Value: "OFF", Semantics: spec.ToneOff},
		{Value: "ENC-DEC", Semantics: spec.ToneEncodeDecode},
		{Value: "ENC", Semantics: spec.ToneEncode},
		{Value: "DCS-ENC-DEC", Semantics: spec.ToneDCSEncodeDecode},
		{Value: "DCS-ENC", Semantics: spec.ToneDCSEncode},
	}
	got := ctcssStateValues(states)
	want := []string{"OFF", "ENC-DEC", "ENC", "DCS-ENC-DEC", "DCS-ENC"}
	if len(got) != len(want) {
		t.Fatalf("ctcssStateValues() = %v, want %v — the picker must offer every state its radio declares, not the family three", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ctcssStateValues()[%d] = %q, want %q — caps' own order is preserved", i, got[i], want[i])
		}
	}
}

// TestCTCSSStateValues_MatchesTheStandardVocabulary is the unchanged half,
// stated here so the extraction cannot silently reorder or filter the list
// every registered model uses.
func TestCTCSSStateValues_MatchesTheStandardVocabulary(t *testing.T) {
	got := ctcssStateValues(spec.StandardCTCSSStates())
	want := []string{"OFF", "ENC-DEC", "ENC"}
	if len(got) != len(want) {
		t.Fatalf("ctcssStateValues(StandardCTCSSStates()) = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ctcssStateValues(StandardCTCSSStates())[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestGetUISpec_CarriesAFiveStateVocabularyThroughToTheGrid is the ROUND
// TRIP the two tests above only approach: GetUISpec itself, on a radio whose
// capability set declares the FT-991A's five P8 states, must put all five in
// UISpecView.CTCSSStateOptions in that radio's own order.
//
// The adversarial review recorded (finding L6) that what landed for the
// "UISpec round-trip pin" was a unit test of the extracted helper, thinner
// than the codeplug and csvio pins beside it, which drive Save/Load and
// Export/Import end to end. This is the missing half. It uses capsForModel's
// own test seam (app.go) and the testModel name, exactly as every other
// GetUISpec shape test in this package does, because no REGISTERED model
// declares five states until Stage 2 — and needing the radio to exist first
// is precisely what would have made the property unprovable until then.
func TestGetUISpec_CarriesAFiveStateVocabularyThroughToTheGrid(t *testing.T) {
	recogniseModelCaps(t, spec.Capabilities{
		Model: testModel, CATID: "9999", Transmit: spec.ReceiveOnly,
		CTCSSStates: []spec.ToneState{
			{Value: "OFF", Semantics: spec.ToneOff},
			{Value: "ENC-DEC", Semantics: spec.ToneEncodeDecode},
			{Value: "ENC", Semantics: spec.ToneEncode},
			{Value: "DCS-ENC-DEC", Semantics: spec.ToneDCSEncodeDecode},
			{Value: "DCS-ENC", Semantics: spec.ToneDCSEncode},
		},
	})

	a, _ := newTestApp(t)
	a.mu.Lock()
	a.working = &codeplug.Codeplug{Schema: codeplug.CurrentSchema, Radio: codeplug.RadioInfo{Model: testModel}}
	a.mu.Unlock()

	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec: %v", err)
	}
	want := []string{"OFF", "ENC-DEC", "ENC", "DCS-ENC-DEC", "DCS-ENC"}
	if len(got.CTCSSStateOptions) != len(want) {
		t.Fatalf("CTCSSStateOptions = %v, want %v — the grid's picker must offer every state this radio declares", got.CTCSSStateOptions, want)
	}
	for i := range want {
		if got.CTCSSStateOptions[i] != want[i] {
			t.Errorf("CTCSSStateOptions[%d] = %q, want %q — the radio's own order reaches the grid unreordered", i, got.CTCSSStateOptions[i], want[i])
		}
	}
}
