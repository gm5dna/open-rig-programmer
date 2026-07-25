// SPDX-License-Identifier: GPL-3.0-or-later

package codeplug

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// TestToneFieldValid covers ToneField.Valid() across every FieldState,
// including the asymmetric zero-Value case: a Known ToneField whose Value
// is the zero Tone is INVALID (0 decihertz is not in
// spec.StandardCTCSSTones — there is no tone that low), unlike BoolField
// where a Known zero Value (false) is perfectly legitimate.
func TestToneFieldValid(t *testing.T) {
	cases := []struct {
		name    string
		field   ToneField
		wantErr bool
	}{
		{"known valid tone", ToneField{State: Known, Value: spec.Tone(670)}, false},
		{"known last table tone", ToneField{State: Known, Value: spec.Tone(2541)}, false},
		{"known zero value invalid", ToneField{State: Known, Value: 0}, true},
		{"known tone not in table (671)", ToneField{State: Known, Value: spec.Tone(671)}, true},
		{"unknown zero value valid", ToneField{State: Unknown, Value: 0}, false},
		{"unknown nonzero value invalid", ToneField{State: Unknown, Value: spec.Tone(670)}, true},
		{"unavailable zero value valid", ToneField{State: Unavailable, Value: 0}, false},
		{"unavailable nonzero value invalid", ToneField{State: Unavailable, Value: spec.Tone(670)}, true},
		{"invalid state", ToneField{State: FieldState("bogus"), Value: 0}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.field.Valid()
			if (err != nil) != tc.wantErr {
				t.Errorf("Valid() = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

// TestBoolFieldValid covers BoolField.Valid() across every FieldState,
// including the Known-with-zero-Value case: a Known BoolField with Value
// == false is VALID, since false is a legitimate known value (contrast
// with ToneField, above).
func TestBoolFieldValid(t *testing.T) {
	cases := []struct {
		name    string
		field   BoolField
		wantErr bool
	}{
		{"known true valid", BoolField{State: Known, Value: true}, false},
		{"known false valid (zero value)", BoolField{State: Known, Value: false}, false},
		{"unknown false valid", BoolField{State: Unknown, Value: false}, false},
		{"unknown true invalid", BoolField{State: Unknown, Value: true}, true},
		{"unavailable false valid", BoolField{State: Unavailable, Value: false}, false},
		{"unavailable true invalid", BoolField{State: Unavailable, Value: true}, true},
		{"invalid state", BoolField{State: FieldState("bogus"), Value: false}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.field.Valid()
			if (err != nil) != tc.wantErr {
				t.Errorf("Valid() = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
