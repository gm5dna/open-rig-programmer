// SPDX-License-Identifier: GPL-3.0-or-later

package driver_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// walkCaps is a capability set with a NON-EMPTY vocabulary for every
// vocab-keyed field the walk consults, so that a Known value can be shown
// both passing and failing the domain half of the stance. A driver whose
// own vocabularies are empty (every Yaesu driver's are) fails closed on
// every Known value instead — that is StringField.Valid's and
// IntField.Valid's own documented rule, not something this walk decides.
//
// Every field's vocabulary is DISJOINT from every other field's (Opus
// review LOW-2, 05/09/2026): the pre-fix fixture gave DuplexOptions,
// ToneModes and PreampOptions a shared "OFF" member, which is exactly the
// condition under which a field judged against the WRONG caps vocabulary
// (Preamp.Valid(caps.AntennaOptions) instead of caps.PreampOptions, say)
// can still admit a value and go undetected. With disjoint vocabularies a
// value that belongs to one field's list is never accidentally valid
// against another's, so TestCheckFieldStates_VocabularyIsFieldSpecific's
// per-field rows below actually exercise the BINDING, not just the type.
func walkCaps() spec.Capabilities {
	return spec.Capabilities{
		CTCSSTones:     []spec.Tone{670, 1000},
		DuplexOptions:  []spec.DuplexOption{{Value: "DUP+"}, {Value: "DUP-"}},
		ToneModes:      []spec.ToneMode{{Value: "TSQL"}, {Value: "DCS"}},
		DTCSCodes:      []int{23, 25},
		DTCSPolarities: []string{"NN", "NR"},
		Filters:        []string{"FIL1", "FIL2"},
		TuningSteps:    []string{"5", "10"},
		AttenuatorDB:   []int{0, 12},
		PreampOptions:  []string{"AMP1", "AMP2"},
		AntennaOptions: []string{"ANT1", "ANT2"},
	}
}

// TestFieldStateWalk_CoversEveryFieldStateField is the fleet's ONE coverage
// pin, shared by every driver that calls the walk (FT-710, FTdx10, FTdx101,
// FT-891) rather than restated once per package: the walk's field list does
// not depend on which radio's capabilities it is handed, so a per-driver
// copy of this assertion would be the same assertion four times.
//
// codeplug exports no enumeration of "every ChannelData field that carries
// a FieldState", so the wanted set is derived INDEPENDENTLY from
// spec.AllFields() minus the seven fields ChannelData carries with no
// FieldState at all — FreqHz, Mode, ClarHz, CTCSS and Shift are plain
// uint64/string/int, Tag is a plain string, and spec.FieldErase has no
// ChannelData field whatsoever (an empty channel IS the erase request).
// A spec.Field the walk forgets to mirror, or that AllFields gains and the
// walk does not, fails HERE rather than silently letting a future field's
// malformed value reach a wire unchecked.
//
// THIS PIN BINDS NAMES AND ORDER, NOT NAME↔MEMBER↔VOCABULARY (Opus review
// LOW-3, 05/09/2026): an all-Absent codeplug.ChannelData{} makes every
// check in the walk return nil, so a copy/paste swap of two fields' judge
// calls would pass this test unchanged. Individual bindings are pinned
// elsewhere, in three layers:
//
//   - TestCheckFieldStates_FleetStance's and
//     TestCheckFieldStates_ReportsTheFirstIncoherentField's value-carrying
//     rows name a refusal's field directly, covering seven of the twenty
//     (CTCSSTone, ScanSkip, TxFrequency, Duplex, ToneTx, Attenuator,
//     IPPlus);
//   - TestCheckFieldStates_VocabularyIsFieldSpecific (below) pins the
//     VOCABULARY ARGUMENT for all nine vocab-keyed fields directly, in
//     this package (Opus review LOW-2, 05/09/2026): a field judged
//     against the wrong caps member — the review's own reproduction,
//     d.Preamp.Valid(caps.PreampOptions) swapped for caps.AntennaOptions
//     — turns that field's row red, because walkCaps gives every
//     vocab-keyed field a vocabulary disjoint from every other's;
//   - each driver's own TestWriteChannel_KnownD8TierFieldsRefusedBeforeWire
//     (or equivalent) additionally pins the MEMBER binding — not the
//     vocabulary argument, since every Yaesu vocabulary is empty — for
//     TuningStep, Attenuator, Preamp and Antenna, at all four drivers.
//
// That leaves the eleven non-vocab-keyed fields' member bindings pinned
// only where the two rows above happen to name them (five of the eleven);
// no hand check stands in for the rest — a wrong binding for one of those
// six would be caught, if at all, by whichever caller-level test happens
// to exercise that exact field, not by anything in this package.
func TestFieldStateWalk_CoversEveryFieldStateField(t *testing.T) {
	plain := map[spec.Field]bool{
		spec.FieldFrequency:  true,
		spec.FieldMode:       true,
		spec.FieldClarifier:  true,
		spec.FieldCTCSSState: true,
		spec.FieldShift:      true,
		spec.FieldTag:        true,
		spec.FieldErase:      true,
	}
	var want []spec.Field
	for _, f := range spec.AllFields() {
		if !plain[f] {
			want = append(want, f)
		}
	}
	if len(want) != 20 {
		t.Fatalf("spec.AllFields() minus the seven plain fields has %d entries, want 20 — this test's own derivation is wrong, not the walk", len(want))
	}

	checks := driver.FieldStateChecks(walkCaps(), codeplug.ChannelData{})
	got := make([]spec.Field, len(checks))
	for i, c := range checks {
		got[i] = c.Field
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FieldStateChecks names\n %v\nbut spec.AllFields() minus the seven plain fields is\n %v\n(the two must be exactly the same twenty fields, in the same order)", got, want)
	}
}

// TestCheckFieldStates_FleetStance walks the four rules the fleet stance is
// made of, one sub-test per rule, over a representative of each of the five
// FieldState-carrying types (ToneField, BoolField, FreqField, StringField,
// IntField) so no type's own Valid is trusted on another's evidence.
//
// THE Absent ROW IS THE ONE THAT DISTINGUISHES THIS STANCE from a naive
// "call Valid on everything": codeplug's typed validators reject Absent
// outright (fieldstate.go's Absent doc comment says so), so a walk that
// called Valid unconditionally would refuse every ordinary MODIFY — every
// hand-built ChannelData that predates the tier fields leaves them Absent.
// A caller who set nothing has requested nothing.
func TestCheckFieldStates_FleetStance(t *testing.T) {
	for _, tt := range []struct {
		name      string
		mutate    func(*codeplug.ChannelData)
		wantField spec.Field
		reasonHas string
	}{
		{
			name:   "an all-Absent channel is admitted: a caller who set nothing requested nothing",
			mutate: func(*codeplug.ChannelData) {},
		},
		{
			name: "a Known value inside this radio's domain is admitted",
			mutate: func(d *codeplug.ChannelData) {
				d.Duplex = codeplug.StringField{State: codeplug.Known, Value: "DUP+"}
				d.CTCSSTone = codeplug.ToneField{State: codeplug.Known, Value: 670}
				d.DTCSCode = codeplug.IntField{State: codeplug.Known, Value: 23}
				d.ScanSkip = codeplug.BoolField{State: codeplug.Known, Value: true}
				d.TxFreqHz = codeplug.FreqField{State: codeplug.Known, Value: 14_250_000}
			},
		},
		{
			name: "a Known value outside this radio's vocabulary is refused",
			mutate: func(d *codeplug.ChannelData) {
				d.Duplex = codeplug.StringField{State: codeplug.Known, Value: "RPS"}
			},
			wantField: spec.FieldDuplex,
			reasonHas: "not one of this radio's values",
		},
		{
			name: "a Known tone outside this radio's chart is refused",
			mutate: func(d *codeplug.ChannelData) {
				d.ToneTx = codeplug.ToneField{State: codeplug.Known, Value: 2000}
			},
			wantField: spec.FieldToneTx,
			reasonHas: "not a tone this radio can express",
		},
		{
			name: "an Unavailable field carrying a value is refused as incoherent",
			mutate: func(d *codeplug.ChannelData) {
				d.TxFreqHz = codeplug.FreqField{State: codeplug.Unavailable, Value: 1}
			},
			wantField: spec.FieldTxFrequency,
			reasonHas: "must have zero Value",
		},
		{
			name: "an Unknown field carrying a value is refused as incoherent",
			mutate: func(d *codeplug.ChannelData) {
				d.AttenuatorDB = codeplug.IntField{State: codeplug.Unknown, Value: 12}
			},
			wantField: spec.FieldAttenuator,
			reasonHas: "must have zero Value",
		},
		// The five rows below are MEDIUM-1 (Opus review, 05/09/2026) and the
		// DECISION erratum it forced: codeplug.Absent is the ZERO
		// FieldState, so a caller who sets a Value and forgets to set
		// State — a copy/paste slip, not a hand-built ChannelData that
		// left a field untouched — produces exactly the struct the
		// all-Absent row above admits. Before the fix judge(state, ...)
		// looked only at state, so every one of these five was admitted
		// and its value silently dropped; one row per FieldState-carrying
		// type (ToneField, BoolField, FreqField, StringField, IntField)
		// so no type's zero check is trusted on another's evidence, and
		// each reuses the SAME field/value pair the review's reproduction
		// used.
		{
			name: "an Absent ToneField carrying a non-zero value is refused as incoherent",
			mutate: func(d *codeplug.ChannelData) {
				d.CTCSSTone = codeplug.ToneField{Value: 1000}
			},
			wantField: spec.FieldCTCSSTone,
			reasonHas: "must have zero Value",
		},
		{
			name: "an Absent BoolField carrying a non-zero value is refused as incoherent",
			mutate: func(d *codeplug.ChannelData) {
				d.ScanSkip = codeplug.BoolField{Value: true}
			},
			wantField: spec.FieldScanSkip,
			reasonHas: "must have zero Value",
		},
		{
			name: "an Absent FreqField carrying a non-zero value is refused as incoherent",
			mutate: func(d *codeplug.ChannelData) {
				d.TxFreqHz = codeplug.FreqField{Value: 1}
			},
			wantField: spec.FieldTxFrequency,
			reasonHas: "must have zero Value",
		},
		{
			name: "an Absent StringField carrying a non-zero value is refused as incoherent",
			mutate: func(d *codeplug.ChannelData) {
				d.Duplex = codeplug.StringField{Value: "RPS"}
			},
			wantField: spec.FieldDuplex,
			reasonHas: "must have zero Value",
		},
		{
			name: "an Absent IntField carrying a non-zero value is refused as incoherent",
			mutate: func(d *codeplug.ChannelData) {
				d.AttenuatorDB = codeplug.IntField{Value: 99}
			},
			wantField: spec.FieldAttenuator,
			reasonHas: "must have zero Value",
		},
		{
			name: "an Unknown field with a zero value is admitted: preserve what the radio has",
			mutate: func(d *codeplug.ChannelData) {
				d.CTCSSTone = codeplug.ToneField{State: codeplug.Unknown}
				d.ScanSkip = codeplug.BoolField{State: codeplug.Unknown}
				d.TagDisplay = codeplug.BoolField{State: codeplug.Unavailable}
				d.Preamp = codeplug.StringField{State: codeplug.Unavailable}
			},
		},
		{
			name: "an unrecognised State string is refused",
			mutate: func(d *codeplug.ChannelData) {
				d.IPPlus = codeplug.BoolField{State: codeplug.FieldState("bogus")}
			},
			wantField: spec.FieldIPPlus,
			reasonHas: `invalid State "bogus"`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var d codeplug.ChannelData
			tt.mutate(&d)
			field, err := driver.CheckFieldStates(walkCaps(), d)
			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("CheckFieldStates = (%s, %v), want no refusal", field, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("CheckFieldStates = (%s, nil), want a refusal naming %s", field, tt.wantField)
			}
			if field != tt.wantField {
				t.Errorf("CheckFieldStates named %s, want %s", field, tt.wantField)
			}
			if !strings.Contains(err.Error(), tt.reasonHas) {
				t.Errorf("CheckFieldStates reason = %q, want it to contain %q", err, tt.reasonHas)
			}
		})
	}
}

// TestCheckFieldStates_ReportsTheFirstIncoherentField pins that the walk
// stops at the FIRST failure in ChannelData's own declaration order, so a
// channel broken in several ways at once is reported for one field
// deterministically rather than for whichever the map iteration reached.
func TestCheckFieldStates_ReportsTheFirstIncoherentField(t *testing.T) {
	d := codeplug.ChannelData{
		// ScanSkip is third in the walk; IPPlus is twentieth.
		ScanSkip: codeplug.BoolField{State: codeplug.Unknown, Value: true},
		IPPlus:   codeplug.BoolField{State: codeplug.Unknown, Value: true},
	}
	field, err := driver.CheckFieldStates(walkCaps(), d)
	if err == nil {
		t.Fatal("CheckFieldStates = nil, want a refusal")
	}
	if field != spec.FieldScanSkip {
		t.Errorf("CheckFieldStates named %s, want %s — the walk must report the first failure in ChannelData's declaration order", field, spec.FieldScanSkip)
	}
}

// TestCheckFieldStates_VocabularyIsFieldSpecific pins the vocabulary
// ARGUMENT the walk consults for each of the nine vocab-keyed fields
// (Opus review LOW-2, 05/09/2026: "the vocabulary ARGUMENT is unpinned
// for six of the nine vocab-keyed fields"). One row per field, each
// setting ONLY that field to Known with a value drawn from its OWN
// walkCaps list; every other field stays Absent.
//
// walkCaps gives every vocab-keyed field a vocabulary DISJOINT from
// every other field's, so a value that is valid here is valid against
// NO other field's list — that is what makes this a binding pin rather
// than a domain pin. A field judged against the WRONG caps member turns
// its own row red: the review's own reproduction (mutation M9) swapped
// d.Preamp.Valid(caps.PreampOptions) for caps.AntennaOptions and
// d.Filter.Valid(caps.Filters) for caps.DTCSPolarities, and re-running
// that swap against this table sends the "Preamp" and "Filter" subtests
// red, because "AMP1" is not in AntennaOptions and "FIL1" is not in
// DTCSPolarities.
func TestCheckFieldStates_VocabularyIsFieldSpecific(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func(*codeplug.ChannelData)
	}{
		{"CTCSSTone", func(d *codeplug.ChannelData) {
			d.CTCSSTone = codeplug.ToneField{State: codeplug.Known, Value: 670}
		}},
		{"Duplex", func(d *codeplug.ChannelData) {
			d.Duplex = codeplug.StringField{State: codeplug.Known, Value: "DUP+"}
		}},
		{"ToneMode", func(d *codeplug.ChannelData) {
			d.ToneMode = codeplug.StringField{State: codeplug.Known, Value: "TSQL"}
		}},
		{"DTCSCode", func(d *codeplug.ChannelData) {
			d.DTCSCode = codeplug.IntField{State: codeplug.Known, Value: 23}
		}},
		{"DTCSPolarity", func(d *codeplug.ChannelData) {
			d.DTCSPolarity = codeplug.StringField{State: codeplug.Known, Value: "NN"}
		}},
		{"Filter", func(d *codeplug.ChannelData) {
			d.Filter = codeplug.StringField{State: codeplug.Known, Value: "FIL1"}
		}},
		{"TuningStep", func(d *codeplug.ChannelData) {
			d.TuningStep = codeplug.StringField{State: codeplug.Known, Value: "5"}
		}},
		{"Preamp", func(d *codeplug.ChannelData) {
			d.Preamp = codeplug.StringField{State: codeplug.Known, Value: "AMP1"}
		}},
		{"Antenna", func(d *codeplug.ChannelData) {
			d.Antenna = codeplug.StringField{State: codeplug.Known, Value: "ANT1"}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var d codeplug.ChannelData
			tt.mutate(&d)
			field, err := driver.CheckFieldStates(walkCaps(), d)
			if err != nil {
				t.Fatalf("CheckFieldStates = (%s, %v), want no refusal — the value is in THIS field's own walkCaps list and no other field's", field, err)
			}
		})
	}
}
