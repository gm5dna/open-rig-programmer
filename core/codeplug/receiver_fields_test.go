// SPDX-License-Identifier: GPL-3.0-or-later

package codeplug

import (
	"reflect"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/spec"
)

func TestChannelData_ReceiverFieldTypesAndJSONTags(t *testing.T) {
	typeWant := map[string]struct {
		typeOf reflect.Type
		tag    string
	}{
		"TuningStepEnabled":   {reflect.TypeOf(BoolField{}), string(spec.FieldTuningStepEnabled)},
		"TuningStep":          {reflect.TypeOf(StringField{}), string(spec.FieldTuningStep)},
		"ProgramTuningStepHz": {reflect.TypeOf(FreqField{}), string(spec.FieldProgramTuningStep)},
		"AttenuatorDB":        {reflect.TypeOf(IntField{}), string(spec.FieldAttenuator)},
		"Preamp":              {reflect.TypeOf(StringField{}), string(spec.FieldPreamp)},
		"Antenna":             {reflect.TypeOf(StringField{}), string(spec.FieldAntenna)},
		"IPPlus":              {reflect.TypeOf(BoolField{}), string(spec.FieldIPPlus)},
	}
	typ := reflect.TypeOf(ChannelData{})
	for name, want := range typeWant {
		field, ok := typ.FieldByName(name)
		if !ok {
			t.Errorf("ChannelData.%s is missing", name)
			continue
		}
		if field.Type != want.typeOf {
			t.Errorf("ChannelData.%s type = %v, want %v", name, field.Type, want.typeOf)
		}
		if got := field.Tag.Get("json"); got != want.tag {
			t.Errorf("ChannelData.%s json tag = %q, want %q", name, got, want.tag)
		}
	}
}

func receiverCapabilities() spec.Capabilities {
	fields := map[spec.Field]spec.FieldSupport{}
	for _, field := range []spec.Field{
		spec.FieldTuningStepEnabled, spec.FieldTuningStep,
		spec.FieldProgramTuningStep, spec.FieldAttenuator,
		spec.FieldPreamp, spec.FieldAntenna, spec.FieldIPPlus,
	} {
		fields[field] = spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	}
	return spec.Capabilities{
		TuningSteps:            []string{"1 kHz", "5 kHz"},
		ProgramTuningStepRange: &spec.StepRange{MinHz: 100, MaxHz: 1000, ResolutionHz: 100},
		AttenuatorDB:           []int{0, 10, 20},
		PreampOptions:          []string{"OFF", "ON"},
		AntennaOptions:         []string{"ANT1", "ANT2"},
		Banks:                  []spec.Bank{{ID: spec.BankMemory, Slots: []string{"001"}, Fields: fields}},
	}
}

func validReceiverData() ChannelData {
	return ChannelData{
		TuningStepEnabled:   BoolField{State: Known, Value: true},
		TuningStep:          StringField{State: Known, Value: "5 kHz"},
		ProgramTuningStepHz: FreqField{State: Known, Value: 500},
		AttenuatorDB:        IntField{State: Known, Value: 10},
		Preamp:              StringField{State: Known, Value: "ON"},
		Antenna:             StringField{State: Known, Value: "ANT2"},
		IPPlus:              BoolField{State: Known, Value: true},
	}
}

func TestValidateTierFields_ReceiverTypedShapesAndDomains(t *testing.T) {
	cases := []struct {
		name   string
		field  spec.Field
		mutate func(*ChannelData)
		want   string
	}{
		{"tuning step enabled shape", spec.FieldTuningStepEnabled, func(d *ChannelData) { d.TuningStepEnabled = BoolField{State: Unknown, Value: true} }, "BoolField"},
		{"tuning step domain", spec.FieldTuningStep, func(d *ChannelData) { d.TuningStep = StringField{State: Known, Value: "2.5 kHz"} }, "not one of this radio's values"},
		{"program step shape", spec.FieldProgramTuningStep, func(d *ChannelData) { d.ProgramTuningStepHz = FreqField{State: Unknown, Value: 500} }, "FreqField"},
		{"program step range", spec.FieldProgramTuningStep, func(d *ChannelData) { d.ProgramTuningStepHz = FreqField{State: Known, Value: 550} }, "not admitted by this radio's range"},
		{"attenuator domain", spec.FieldAttenuator, func(d *ChannelData) { d.AttenuatorDB = IntField{State: Known, Value: 5} }, "not one of this radio's values"},
		{"preamp domain", spec.FieldPreamp, func(d *ChannelData) { d.Preamp = StringField{State: Known, Value: "PRE2"} }, "not one of this radio's values"},
		{"antenna domain", spec.FieldAntenna, func(d *ChannelData) { d.Antenna = StringField{State: Known, Value: "ANT3"} }, "not one of this radio's values"},
		{"ip plus shape", spec.FieldIPPlus, func(d *ChannelData) { d.IPPlus = BoolField{State: Unknown, Value: true} }, "BoolField"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := validReceiverData()
			tc.mutate(&data)
			issues := validateTierFields("001", spec.BankMemory, data, receiverCapabilities())
			var found bool
			for _, issue := range issues {
				if issue.Field == tc.field && strings.Contains(issue.Msg, tc.want) {
					found = true
				}
			}
			if !found {
				t.Fatalf("issues = %+v, want %s issue containing %q", issues, tc.field, tc.want)
			}
		})
	}

	if issues := validateTierFields("001", spec.BankMemory, validReceiverData(), receiverCapabilities()); len(issues) != 0 {
		t.Fatalf("valid receiver fields produced issues: %+v", issues)
	}
}

func TestReceiverFields_DiffTablesCoverAllSeven(t *testing.T) {
	tests := []struct {
		field spec.Field
		set   func(*ChannelData)
	}{
		{spec.FieldTuningStepEnabled, func(d *ChannelData) { d.TuningStepEnabled = BoolField{State: Known, Value: true} }},
		{spec.FieldTuningStep, func(d *ChannelData) { d.TuningStep = StringField{State: Known, Value: "5 kHz"} }},
		{spec.FieldProgramTuningStep, func(d *ChannelData) { d.ProgramTuningStepHz = FreqField{State: Known, Value: 500} }},
		{spec.FieldAttenuator, func(d *ChannelData) { d.AttenuatorDB = IntField{State: Known, Value: 10} }},
		{spec.FieldPreamp, func(d *ChannelData) { d.Preamp = StringField{State: Known, Value: "ON"} }},
		{spec.FieldAntenna, func(d *ChannelData) { d.Antenna = StringField{State: Known, Value: "ANT2"} }},
		{spec.FieldIPPlus, func(d *ChannelData) { d.IPPlus = BoolField{State: Known, Value: true} }},
	}
	for _, tc := range tests {
		t.Run(string(tc.field), func(t *testing.T) {
			var after ChannelData
			tc.set(&after)
			if got := changedFields(ChannelData{}, after); !reflect.DeepEqual(got, []spec.Field{tc.field}) {
				t.Errorf("changedFields() = %v, want [%s]", got, tc.field)
			}
			var found bool
			for _, entry := range tierAddedFieldFor {
				if entry.field == tc.field && entry.present(after) {
					found = true
				}
			}
			if !found {
				t.Errorf("tierAddedFieldFor does not report Known %s", tc.field)
			}
		})
	}
}
