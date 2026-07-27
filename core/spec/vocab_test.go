// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import "testing"

// TestStandardVocab_MatchesLegacyLiterals pins StandardShiftOptions and
// StandardCTCSSStates against the exact literal switches they replace
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

	wantCTCSS := []ToneState{
		{Value: "OFF", RequiresTone: false, Encodes: false, Decodes: false},
		{Value: "ENC-DEC", RequiresTone: true, Encodes: true, Decodes: true},
		{Value: "ENC", RequiresTone: true, Encodes: true, Decodes: false},
	}
	gotCTCSS := StandardCTCSSStates()
	if len(gotCTCSS) != len(wantCTCSS) {
		t.Fatalf("StandardCTCSSStates() = %+v, want %+v", gotCTCSS, wantCTCSS)
	}
	for i := range wantCTCSS {
		if gotCTCSS[i] != wantCTCSS[i] {
			t.Errorf("StandardCTCSSStates()[%d] = %+v, want %+v", i, gotCTCSS[i], wantCTCSS[i])
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

// TestStandardCTCSSStatesReturnsCopy checks that StandardCTCSSStates
// hands back an independent slice each call: mutating one call's result
// must never be observable through a second, separate call.
func TestStandardCTCSSStatesReturnsCopy(t *testing.T) {
	a := StandardCTCSSStates()
	a[0] = ToneState{Value: "TAMPERED", RequiresTone: true, Encodes: true}
	b := StandardCTCSSStates()
	if b[0].Value == "TAMPERED" {
		t.Fatal("mutating one call's result changed a later call's result: StandardCTCSSStates() is not returning an independent copy")
	}
	if b[0] != (ToneState{Value: "OFF", RequiresTone: false, Encodes: false, Decodes: false}) {
		t.Errorf("StandardCTCSSStates()[0] = %+v after a prior call was mutated, want {OFF false false false} (unaffected)", b[0])
	}
}
