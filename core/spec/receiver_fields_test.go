// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"strings"
	"testing"
)

func TestReceiverFieldConstants(t *testing.T) {
	want := map[Field]string{
		FieldTuningStepEnabled: "tuning_step_enabled",
		FieldTuningStep:        "tuning_step",
		FieldProgramTuningStep: "program_tuning_step",
		FieldAttenuator:        "attenuator",
		FieldPreamp:            "preamp",
		FieldAntenna:           "antenna",
		FieldIPPlus:            "ip_plus",
	}
	if len(want) != 7 {
		t.Fatalf("receiver field count = %d, want 7", len(want))
	}
	for field, spelling := range want {
		if string(field) != spelling {
			t.Errorf("field %q spelling = %q, want %q", field, field, spelling)
		}
	}
}

func TestCapabilitiesValidate_ReceiverVocabularies(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Capabilities)
		want   string
	}{
		{"blank tuning step", func(c *Capabilities) { c.TuningSteps = []string{"1 kHz", ""} }, "TuningSteps must not contain a blank value"},
		{"duplicate tuning step", func(c *Capabilities) { c.TuningSteps = []string{"1 kHz", "1 kHz"} }, `TuningSteps contains duplicate value "1 kHz"`},
		{"blank preamp", func(c *Capabilities) { c.PreampOptions = []string{"OFF", ""} }, "PreampOptions must not contain a blank value"},
		{"duplicate preamp", func(c *Capabilities) { c.PreampOptions = []string{"OFF", "OFF"} }, `PreampOptions contains duplicate value "OFF"`},
		{"blank antenna", func(c *Capabilities) { c.AntennaOptions = []string{"ANT1", ""} }, "AntennaOptions must not contain a blank value"},
		{"duplicate antenna", func(c *Capabilities) { c.AntennaOptions = []string{"ANT1", "ANT1"} }, `AntennaOptions contains duplicate value "ANT1"`},
		{"attenuator descending", func(c *Capabilities) { c.AttenuatorDB = []int{0, 20, 10} }, "AttenuatorDB is not strictly ascending"},
		{"attenuator duplicate", func(c *Capabilities) { c.AttenuatorDB = []int{0, 10, 10} }, "AttenuatorDB is not strictly ascending"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			caps := validTestCapabilities()
			tc.mutate(&caps)
			err := caps.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestCapabilitiesValidate_ProgramTuningStepRange(t *testing.T) {
	cases := []struct {
		name       string
		rangeValue StepRange
		want       string
	}{
		{"zero resolution", StepRange{MinHz: 100, MaxHz: 100000}, "ResolutionHz 0 must be greater than zero"},
		{"inverted", StepRange{MinHz: 1000, MaxHz: 100, ResolutionHz: 100}, "MinHz 1000 is greater than MaxHz 100"},
		{"minimum unaligned", StepRange{MinHz: 150, MaxHz: 1000, ResolutionHz: 100}, "MinHz 150 is not aligned"},
		{"maximum unaligned", StepRange{MinHz: 100, MaxHz: 1050, ResolutionHz: 100}, "MaxHz 1050 is not aligned"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			caps := validTestCapabilities()
			caps.ProgramTuningStepRange = &tc.rangeValue
			err := caps.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() error = %v, want substring %q", err, tc.want)
			}
		})
	}

	caps := validTestCapabilities()
	caps.ProgramTuningStepRange = &StepRange{MinHz: 100, MaxHz: 100000, ResolutionHz: 100}
	if err := caps.Validate(); err != nil {
		t.Fatalf("Validate() rejected aligned receiver range: %v", err)
	}
}
