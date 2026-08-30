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

// TestAdmitsProgramTuningStep_MatchesValidationAtDomainEdges pins one shared
// predicate for codeplug validation and the IC-R8600 read mapper. The edges are
// where duplicated range logic is most likely to drift.
func TestAdmitsProgramTuningStep_MatchesValidationAtDomainEdges(t *testing.T) {
	cases := []struct {
		name       string
		rangeValue *spec.StepRange
		value      uint64
		want       bool
	}{
		{"nil range", nil, 100, false},
		{"zero resolution", &spec.StepRange{MinHz: 100, MaxHz: 1000}, 100, false},
		{"below minimum", &spec.StepRange{MinHz: 100, MaxHz: 1000, ResolutionHz: 100}, 99, false},
		{"minimum", &spec.StepRange{MinHz: 100, MaxHz: 1000, ResolutionHz: 100}, 100, true},
		{"misaligned interior", &spec.StepRange{MinHz: 100, MaxHz: 1000, ResolutionHz: 100}, 550, false},
		{"maximum", &spec.StepRange{MinHz: 100, MaxHz: 1000, ResolutionHz: 100}, 1000, true},
		{"above maximum", &spec.StepRange{MinHz: 100, MaxHz: 1000, ResolutionHz: 100}, 1001, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			caps := receiverCapabilities()
			caps.ProgramTuningStepRange = tc.rangeValue
			data := validReceiverData()
			data.ProgramTuningStepHz.Value = tc.value
			validatorAdmits := true
			for _, issue := range validateTierFields("001", spec.BankMemory, data, caps) {
				if issue.Field == spec.FieldProgramTuningStep {
					validatorAdmits = false
				}
			}
			if got := AdmitsProgramTuningStep(caps, tc.value); got != tc.want || got != validatorAdmits {
				t.Errorf("AdmitsProgramTuningStep = %v, validator admits = %v, want %v", got, validatorAdmits, tc.want)
			}
		})
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

// TestValidateTierFields_UnreachableFieldMayOnlySayUnavailable is the
// codeplug half of tier review M1. A bank that cannot REACH a field used
// to have its state ignored entirely, so a channel built anywhere but by
// a read — a native-CSV import parses tx_frequency capability-blind —
// could assert a field the radio does not have and pass the send gate.
// Known and Unknown are each a claim about anatomy the radio does not
// possess. Absent is a silence, not a claim, and stays silent — that is
// the state every pre-tier radio's hand-built channel is in before
// NormaliseTierFields resolves it, and refusing it would change the
// pre-tier Issue list this function is documented not to touch.
//
// The receive-only wording is the anatomical one, and is the same string
// core/csvio/chirp.go gives a receiver asked for split duplex.
func TestValidateTierFields_UnreachableFieldMayOnlySayUnavailable(t *testing.T) {
	receiver := func() spec.Capabilities {
		caps := receiverCapabilities()
		caps.Transmit = spec.ReceiveOnly
		return caps
	}
	transmitter := func() spec.Capabilities {
		caps := receiverCapabilities()
		caps.Transmit = spec.HasTransmitter
		return caps
	}

	for _, tc := range []struct {
		name   string
		caps   spec.Capabilities
		field  spec.Field
		mutate func(*ChannelData)
		want   string
	}{
		{"known tx frequency on a receiver", receiver(), spec.FieldTxFrequency,
			func(d *ChannelData) { d.TxFreqHz = FreqField{State: Known, Value: 433_500_000} },
			"this radio has no transmitter"},
		{"unknown tx frequency on a receiver", receiver(), spec.FieldTxFrequency,
			func(d *ChannelData) { d.TxFreqHz = FreqField{State: Unknown} },
			"this radio has no transmitter"},
		{"known tx tone on a receiver", receiver(), spec.FieldToneTx,
			func(d *ChannelData) { d.ToneTx = ToneField{State: Known, Value: 885} },
			"this radio has no transmitter"},
		{"known tx frequency on a transmitter", transmitter(), spec.FieldTxFrequency,
			func(d *ChannelData) { d.TxFreqHz = FreqField{State: Known, Value: 433_500_000} },
			"this radio has no tx_frequency field"},
		{"known filter on a transmitter", transmitter(), spec.FieldFilter,
			func(d *ChannelData) { d.Filter = StringField{State: Known, Value: "FIL1"} },
			"this radio has no filter field"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := validReceiverData()
			withUnavailableTierFields(&data)
			tc.mutate(&data)
			issues := validateTierFields("001", spec.BankMemory, data, tc.caps)
			var found bool
			for _, issue := range issues {
				if issue.Field == tc.field && issue.Severity == SeverityError && strings.Contains(issue.Msg, tc.want) {
					found = true
				}
			}
			if !found {
				t.Fatalf("issues = %+v, want a %s error containing %q", issues, tc.field, tc.want)
			}
		})
	}

	// Unavailable AND Absent are both silent everywhere unreachable, on
	// both anatomies: the first says "no such field", the second says
	// nothing at all, and neither claims the radio has a transmitter.
	for name, caps := range map[string]spec.Capabilities{"receiver": receiver(), "transmitter": transmitter()} {
		t.Run("silent when unavailable/"+name, func(t *testing.T) {
			data := validReceiverData()
			withUnavailableTierFields(&data)
			if issues := validateTierFields("001", spec.BankMemory, data, caps); len(issues) != 0 {
				t.Fatalf("issues = %+v, want none", issues)
			}
		})
		t.Run("silent when absent/"+name, func(t *testing.T) {
			data := validReceiverData()
			if issues := validateTierFields("001", spec.BankMemory, data, caps); len(issues) != 0 {
				t.Fatalf("issues = %+v, want none", issues)
			}
		})
	}
}
