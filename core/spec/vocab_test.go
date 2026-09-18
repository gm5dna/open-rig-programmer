// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import "testing"

// TestStandardVocab_MatchesLegacyLiterals pins StandardShiftOptions and
// StandardToneModes against the exact literal switches they replace
// (core/codeplug/validate.go's old "SIMPLEX"/"PLUS"/"MINUS" and
// "OFF"/"ENC-DEC"/"ENC" switches, and app/uispec.go's restated
// ShiftOptions/CTCSSStateOptions lists) — same values, same order. A
// future edit to either function that silently drifts from today's
// FT-710 vocabulary fails here first.
func TestStandardVocab_MatchesLegacyLiterals(t *testing.T) {
	wantShift := []ShiftOption{
		{Value: "SIMPLEX", Direction: ShiftNone},
		{Value: "PLUS", Direction: ShiftUp},
		{Value: "MINUS", Direction: ShiftDown},
	}
	gotShift := StandardShiftOptions()
	if len(gotShift) != len(wantShift) {
		t.Fatalf("StandardShiftOptions() = %+v, want %+v", gotShift, wantShift)
	}
	for i := range wantShift {
		if gotShift[i] != wantShift[i] {
			t.Errorf("StandardShiftOptions()[%d] = %+v, want %+v", i, gotShift[i], wantShift[i])
		}
	}

	wantCTCSS := []ToneMode{
		{Value: "OFF", Semantics: ToneModeOff},
		{Value: "ENC-DEC", Semantics: ToneModeCTCSSSquelch},
		{Value: "ENC", Semantics: ToneModeCTCSS},
	}
	gotCTCSS := StandardToneModes()
	if len(gotCTCSS) != len(wantCTCSS) {
		t.Fatalf("StandardToneModes() = %+v, want %+v", gotCTCSS, wantCTCSS)
	}
	for i := range wantCTCSS {
		if gotCTCSS[i] != wantCTCSS[i] {
			t.Errorf("StandardToneModes()[%d] = %+v, want %+v", i, gotCTCSS[i], wantCTCSS[i])
		}
	}
}

// TestStandardShiftOptionsReturnsCopy checks that StandardShiftOptions
// hands back an independent slice each call: mutating one call's result
// must never be observable through a second, separate call.
func TestStandardShiftOptionsReturnsCopy(t *testing.T) {
	a := StandardShiftOptions()
	a[0] = ShiftOption{Value: "TAMPERED"}
	b := StandardShiftOptions()
	if b[0].Value == "TAMPERED" {
		t.Fatal("mutating one call's result changed a later call's result: StandardShiftOptions() is not returning an independent copy")
	}
	if b[0] != (ShiftOption{Value: "SIMPLEX", Direction: ShiftNone}) {
		t.Errorf("StandardShiftOptions()[0] = %+v after a prior call was mutated, want {SIMPLEX ShiftNone} (unaffected)", b[0])
	}
}

// TestStandardToneModesReturnsCopy checks that StandardToneModes hands
// back an independent slice each call: mutating one call's result must
// never be observable through a second, separate call.
func TestStandardToneModesReturnsCopy(t *testing.T) {
	a := StandardToneModes()
	a[0] = ToneMode{Value: "TAMPERED", Semantics: ToneModeCTCSS}
	b := StandardToneModes()
	if b[0].Value == "TAMPERED" {
		t.Fatal("mutating one call's result changed a later call's result: StandardToneModes() is not returning an independent copy")
	}
	if b[0] != (ToneMode{Value: "OFF", Semantics: ToneModeOff}) {
		t.Errorf("StandardToneModes()[0] = %+v after a prior call was mutated, want {OFF ToneModeOff} (unaffected)", b[0])
	}
}

// TestToneMode_NeedsTxTone covers the NeedsTxTone method directly: true
// for ToneModeCTCSS/ToneModeCTCSSSquelch, false for ToneModeOff — the
// RequiresTone-style predicate core/codeplug's validator consults, now
// that ToneState/RequiresTone were deleted and ToneMode/NeedsTxTone
// serves both the Yaesu (FieldCTCSSState) and Icom/Kenwood
// (FieldToneMode) identities.
func TestToneMode_NeedsTxTone(t *testing.T) {
	cases := []struct {
		semantics ToneModeSemantics
		want      bool
	}{
		{ToneModeOff, false},
		{ToneModeCTCSS, true},
		{ToneModeCTCSSSquelch, true},
	}
	for _, tc := range cases {
		tm := ToneMode{Value: "X", Semantics: tc.semantics}
		if got := tm.NeedsTxTone(); got != tc.want {
			t.Errorf("ToneMode{Semantics: %d}.NeedsTxTone() = %v, want %v", tc.semantics, got, tc.want)
		}
	}
}
