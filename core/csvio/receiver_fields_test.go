// SPDX-License-Identifier: GPL-3.0-or-later

package csvio

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
)

func TestHeaderVersions_V2IsAPrefixOfV3(t *testing.T) {
	if len(headerV3) != len(headerV2)+len(receiverColumns) {
		t.Fatalf("headerV3 has %d columns, want %d + %d", len(headerV3), len(headerV2), len(receiverColumns))
	}
	if !reflect.DeepEqual(headerV3[:len(headerV2)], headerV2) {
		t.Errorf("headerV3 does not start with version 2:\n got %v\nwant %v", headerV3[:len(headerV2)], headerV2)
	}
	if !reflect.DeepEqual(headerV3[len(headerV2):], receiverColumns) {
		t.Errorf("headerV3 tail = %v, want %v", headerV3[len(headerV2):], receiverColumns)
	}
}

func TestExport_Version3ChosenOnlyByRecordedReceiverField(t *testing.T) {
	headerOf := func(channels []codeplug.Channel) string {
		t.Helper()
		var buf bytes.Buffer
		if err := Export(&buf, channels); err != nil {
			t.Fatal(err)
		}
		return strings.SplitN(buf.String(), "\n", 2)[0]
	}

	for name, mutate := range map[string]func(*codeplug.ChannelData){
		"known":       func(d *codeplug.ChannelData) { d.Antenna = codeplug.StringField{State: codeplug.Known, Value: "ANT2"} },
		"unknown":     func(d *codeplug.ChannelData) { d.IPPlus = codeplug.BoolField{State: codeplug.Unknown} },
		"unavailable": func(d *codeplug.ChannelData) { d.Preamp = codeplug.StringField{State: codeplug.Unavailable} },
		"absent":      func(d *codeplug.ChannelData) { d.AttenuatorDB = codeplug.IntField{} },
	} {
		t.Run(name, func(t *testing.T) {
			channels := yaesuLikeChannels()
			channels[0].Data.Duplex = codeplug.StringField{State: codeplug.Known, Value: "OFF"}
			mutate(channels[0].Data)
			want := strings.Join(headerV2, ",")
			if name == "known" || name == "unknown" {
				want = strings.Join(headerV3, ",")
			}
			if got := headerOf(channels); got != want {
				t.Errorf("header = %q, want %q", got, want)
			}
		})
	}
}

func TestExportImport_ReceiverFieldsRoundTripThroughV3(t *testing.T) {
	d := &codeplug.ChannelData{
		FreqHz: 145_500_000, Mode: "FM",
		CTCSSTone:           codeplug.ToneField{State: codeplug.Unavailable},
		TagDisplay:          codeplug.BoolField{State: codeplug.Unavailable},
		ScanSkip:            codeplug.BoolField{State: codeplug.Unavailable},
		TuningStepEnabled:   codeplug.BoolField{State: codeplug.Known, Value: true},
		TuningStep:          codeplug.StringField{State: codeplug.Known, Value: "5 kHz"},
		ProgramTuningStepHz: codeplug.FreqField{State: codeplug.Known, Value: 500},
		AttenuatorDB:        codeplug.IntField{State: codeplug.Known, Value: 10},
		Preamp:              codeplug.StringField{State: codeplug.Unknown},
		Antenna:             codeplug.StringField{State: codeplug.Unavailable},
		IPPlus:              codeplug.BoolField{State: codeplug.Absent},
	}
	markTierFieldsUnavailable(d)
	d.TuningStepEnabled = codeplug.BoolField{State: codeplug.Known, Value: true}
	d.TuningStep = codeplug.StringField{State: codeplug.Known, Value: "5 kHz"}
	d.ProgramTuningStepHz = codeplug.FreqField{State: codeplug.Known, Value: 500}
	d.AttenuatorDB = codeplug.IntField{State: codeplug.Known, Value: 10}
	d.Preamp = codeplug.StringField{State: codeplug.Unknown}
	d.Antenna = codeplug.StringField{State: codeplug.Unavailable}
	d.IPPlus = codeplug.BoolField{State: codeplug.Absent}
	want := []codeplug.Channel{{Slot: "001", Data: d}}

	var buf bytes.Buffer
	if err := Export(&buf, want); err != nil {
		t.Fatal(err)
	}
	got, err := Import(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Import: %v\n%s", err, buf.String())
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip differs:\n got %+v\nwant %+v", got, want)
	}
}

func TestImport_ThreeHeaderBranches(t *testing.T) {
	baseRow := "001,M-01,145500000,FM,,,,OFF,,SIMPLEX,CALLING,yes,no"
	for _, tt := range []struct {
		name        string
		header      []string
		row         string
		wantD4      codeplug.FieldState
		wantD8      codeplug.FieldState
		wantAntenna codeplug.StringField
	}{
		{"v1", header, baseRow, codeplug.Unavailable, codeplug.Unavailable, codeplug.StringField{State: codeplug.Unavailable}},
		{"partial v2", append(append([]string(nil), header...), "duplex"), baseRow + ",OFF", codeplug.Unknown, codeplug.Unavailable, codeplug.StringField{State: codeplug.Unavailable}},
		{"partial v3", append(append([]string(nil), header...), "antenna"), baseRow + ",ANT2", codeplug.Unknown, codeplug.Unknown, codeplug.StringField{State: codeplug.Known, Value: "ANT2"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Import(strings.NewReader(strings.Join(tt.header, ",") + "\n" + tt.row + "\n"))
			if err != nil {
				t.Fatal(err)
			}
			d := got[0].Data
			if d.Filter.State != tt.wantD4 || d.Preamp.State != tt.wantD8 || d.Antenna != tt.wantAntenna {
				t.Errorf("states = filter %q preamp %q antenna %+v, want %q/%q/%+v", d.Filter.State, d.Preamp.State, d.Antenna, tt.wantD4, tt.wantD8, tt.wantAntenna)
			}
		})
	}
}
