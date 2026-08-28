// SPDX-License-Identifier: GPL-3.0-or-later

package clone

import (
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

func TestWritableFieldsMismatch_ReceiverFields(t *testing.T) {
	for _, tt := range []struct {
		field spec.Field
		set   func(*codeplug.ChannelData, bool)
	}{
		{spec.FieldTuningStepEnabled, func(d *codeplug.ChannelData, other bool) {
			d.TuningStepEnabled = codeplug.BoolField{State: codeplug.Known, Value: other}
		}},
		{spec.FieldTuningStep, func(d *codeplug.ChannelData, other bool) {
			d.TuningStep = codeplug.StringField{State: codeplug.Known, Value: map[bool]string{false: "5 kHz", true: "10 kHz"}[other]}
		}},
		{spec.FieldProgramTuningStep, func(d *codeplug.ChannelData, other bool) {
			d.ProgramTuningStepHz = codeplug.FreqField{State: codeplug.Known, Value: map[bool]uint64{false: 500, true: 1000}[other]}
		}},
		{spec.FieldAttenuator, func(d *codeplug.ChannelData, other bool) {
			d.AttenuatorDB = codeplug.IntField{State: codeplug.Known, Value: map[bool]int{false: 10, true: 20}[other]}
		}},
		{spec.FieldPreamp, func(d *codeplug.ChannelData, other bool) {
			d.Preamp = codeplug.StringField{State: codeplug.Known, Value: map[bool]string{false: "OFF", true: "ON"}[other]}
		}},
		{spec.FieldAntenna, func(d *codeplug.ChannelData, other bool) {
			d.Antenna = codeplug.StringField{State: codeplug.Known, Value: map[bool]string{false: "ANT1", true: "ANT2"}[other]}
		}},
		{spec.FieldIPPlus, func(d *codeplug.ChannelData, other bool) {
			d.IPPlus = codeplug.BoolField{State: codeplug.Known, Value: other}
		}},
	} {
		t.Run(string(tt.field), func(t *testing.T) {
			want, got := codeplug.ChannelData{}, codeplug.ChannelData{}
			tt.set(&want, false)
			tt.set(&got, true)
			if bad := writableFieldsMismatch(want, got); !reflect.DeepEqual(bad, []spec.Field{tt.field}) {
				t.Errorf("mismatch = %v, want %v", bad, []spec.Field{tt.field})
			}

			got = codeplug.ChannelData{}
			if bad := writableFieldsMismatch(want, got); len(bad) != 0 {
				t.Errorf("non-Known readback mismatch = %v, want none", bad)
			}
		})
	}
}
