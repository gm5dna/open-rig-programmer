// SPDX-License-Identifier: GPL-3.0-or-later

package driver

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/spec"
)

func TestProfileZeroValueIsRealHardware(t *testing.T) {
	var p Profile
	if p != RealHardware {
		t.Errorf("zero Profile = %v; want RealHardware — the fail-safe profile must be the zero value", p)
	}
	if RealHardware == Simulated {
		t.Fatal("RealHardware and Simulated must be distinct")
	}
}

func TestBaseRecognised(t *testing.T) {
	for _, tc := range []struct {
		name string
		base Base
		want bool
	}{
		{"the zero value is RealHardware", Base{}, true},
		{"RealHardware", Base{Profile: RealHardware}, true},
		{"Simulated", Base{Profile: Simulated}, true},
		{"consent does not change recognition", Base{Profile: Simulated, Consented: true}, true},
		{"a cast integer past the set", Base{Profile: Profile(2)}, false},
		{"a negative cast integer", Base{Profile: Profile(-1)}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.base.Recognised(); got != tc.want {
				t.Errorf("Recognised() = %v; want %v", got, tc.want)
			}
		})
	}
}

// unverifiedCaps is a minimal capability set with one Unverified write —
// the only thing SessionCaps' transform can be seen to act on.
func unverifiedCaps() spec.Capabilities {
	return spec.Capabilities{
		Model: "TEST",
		Banks: []spec.Bank{{
			ID:     spec.BankMemory,
			Slots:  []string{"001"},
			Fields: map[spec.Field]spec.FieldSupport{spec.FieldFrequency: {Read: spec.Unverified, Write: spec.Unverified}},
		}},
	}
}

func TestBaseSessionCaps(t *testing.T) {
	for _, tc := range []struct {
		name string
		base Base
		want spec.Support
	}{
		{"no consent leaves the label alone", Base{Profile: RealHardware}, spec.Unverified},
		{"consent opens a recognised RealHardware profile", Base{Profile: RealHardware, Consented: true}, spec.ConsentedUnverified},
		{"consent opens a recognised Simulated profile", Base{Profile: Simulated, Consented: true}, spec.ConsentedUnverified},
		{"consent does NOT open an unrecognised profile", Base{Profile: Profile(7), Consented: true}, spec.Unverified},
		{"an unrecognised profile without consent is untouched", Base{Profile: Profile(7)}, spec.Unverified},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := unverifiedCaps()
			got := tc.base.SessionCaps(in).FieldSupport(spec.BankMemory, spec.FieldFrequency).Write
			if got != tc.want {
				t.Errorf("SessionCaps write label = %v; want %v", got, tc.want)
			}
			if in.FieldSupport(spec.BankMemory, spec.FieldFrequency).Write != spec.Unverified {
				t.Error("SessionCaps mutated the capabilities it was given")
			}
		})
	}
}
