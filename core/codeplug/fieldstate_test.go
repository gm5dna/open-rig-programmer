// SPDX-License-Identifier: GPL-3.0-or-later

package codeplug

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// standardChartCaps returns a spec.Capabilities whose CTCSSTones is the
// full standard 50-tone chart — the FT-710's own chart (see
// core/driver/ft710/caps.go's baseCapabilities), used for cases that
// exercise ToneField.Valid's ordinary behaviour rather than its
// caps-driven-ness specifically.
func standardChartCaps() spec.Capabilities {
	tones := spec.StandardCTCSSTones()
	return spec.Capabilities{CTCSSTones: tones[:]}
}

// TestToneFieldValid covers ToneField.Valid(caps) across every FieldState,
// including the asymmetric zero-Value case: a Known ToneField whose Value
// is the zero Tone is INVALID (0 decihertz is not in any real radio's
// chart — there is no tone that low), unlike BoolField where a Known zero
// Value (false) is perfectly legitimate. It also proves the check is
// genuinely caps-driven (a narrower radio chart rejects a tone the
// standard 50-tone chart would accept, and accepts a tone only that
// narrower chart has) and that an empty caps.CTCSSTones fails closed
// rather than accepting every tone.
func TestToneFieldValid(t *testing.T) {
	narrowChart := spec.Capabilities{CTCSSTones: []spec.Tone{670}}

	cases := []struct {
		name    string
		field   ToneField
		caps    spec.Capabilities
		wantErr bool
	}{
		{"known valid tone", ToneField{State: Known, Value: spec.Tone(670)}, standardChartCaps(), false},
		{"known last table tone", ToneField{State: Known, Value: spec.Tone(2541)}, standardChartCaps(), false},
		{"known zero value invalid", ToneField{State: Known, Value: 0}, standardChartCaps(), true},
		{"known tone not in table (671)", ToneField{State: Known, Value: spec.Tone(671)}, standardChartCaps(), true},
		{"unknown zero value valid", ToneField{State: Unknown, Value: 0}, standardChartCaps(), false},
		{"unknown nonzero value invalid", ToneField{State: Unknown, Value: spec.Tone(670)}, standardChartCaps(), true},
		{"unavailable zero value valid", ToneField{State: Unavailable, Value: 0}, standardChartCaps(), false},
		{"unavailable nonzero value invalid", ToneField{State: Unavailable, Value: spec.Tone(670)}, standardChartCaps(), true},
		{"invalid state", ToneField{State: FieldState("bogus"), Value: 0}, standardChartCaps(), true},
		{"known tone in standard chart but absent from this radio's narrower chart", ToneField{State: Known, Value: spec.Tone(2541)}, narrowChart, true},
		{"known tone present only in this radio's narrower chart", ToneField{State: Known, Value: spec.Tone(670)}, narrowChart, false},
		{"known tone fails closed against an empty chart", ToneField{State: Known, Value: spec.Tone(670)}, spec.Capabilities{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.field.Valid(tc.caps)
			if (err != nil) != tc.wantErr {
				t.Errorf("Valid(%+v) = %v, wantErr %v", tc.caps, err, tc.wantErr)
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
