// SPDX-License-Identifier: GPL-3.0-or-later

package drivertest

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// preTierFields is the ten Field constants core/spec declared before the
// Icom tier: everything AllFields() lists ahead of
// spec.FieldTxFrequency. Named here, rather than counted off AllFields()
// by position, so the list is legible beside the test that uses it.
var preTierFields = []spec.Field{
	spec.FieldFrequency,
	spec.FieldMode,
	spec.FieldClarifier,
	spec.FieldCTCSSState,
	spec.FieldCTCSSTone,
	spec.FieldShift,
	spec.FieldTag,
	spec.FieldTagDisplay,
	spec.FieldScanSkip,
	spec.FieldErase,
}

// TestTierFieldStates_MatchAllFields pins the mitigation
// tierFieldStates' own doc comment claims: the set of fields it names is
// exactly spec.AllFields() minus the ten pre-tier fields above. Without
// this, a newly minted Field could be added to core/spec and never wired
// into tierFieldStates, and nothing would say so — the seventeen-case
// table in TestAssertFreshReadSaveLoad_RefusesAnAbsentField only proves
// its OWN seventeen are covered, not that an eighteenth exists. This
// test fails the moment AllFields() grows past seventeen tier fields
// without a matching entry here, which is the whole point.
func TestTierFieldStates_MatchAllFields(t *testing.T) {
	preTier := make(map[spec.Field]bool, len(preTierFields))
	for _, f := range preTierFields {
		preTier[f] = true
	}
	var want []string
	for _, f := range spec.AllFields() {
		if !preTier[f] {
			want = append(want, string(f))
		}
	}

	d := yaesuShapeChannelData()
	var got []string
	for _, f := range tierFieldStates(&d) {
		got = append(got, f.name)
	}

	if !slices.Equal(got, want) {
		t.Errorf("tierFieldStates names = %v, want %v (spec.AllFields() minus the ten pre-tier fields)", got, want)
	}
}

// errorRecorder captures what a helper reports instead of failing the
// test that is checking the helper. Everything else — Helper, TempDir,
// Fatalf — forwards to the embedded real testing.TB, so the helper runs
// exactly as it does at its driver call sites.
//
// FOURTEEN of those, counted rather than guessed — one per driver package
// but the IC-R8600's, which reports the seven D8 fields Known and so
// calls AssertFreshReadSaveLoadNormalised instead, and but the FT-891's
// (Tier 1), which calls it TWICE — once per read.go path (the combined MT
// form and the MR-only form), both against its own single model. They
// cover fifteen of the sixteen registered models: the ftdx101 and ic7851
// packages each run their one call over a pair of constructors, so two
// models come out of each, while the FT-891's two calls both cover the
// same one. This comment said "fifteen" until the count was checked, and
// at that point the IC-7850 was in fact covered by nothing.
type errorRecorder struct {
	testing.TB
	errs []string
}

func (r *errorRecorder) Errorf(format string, args ...any) {
	r.errs = append(r.errs, fmt.Sprintf(format, args...))
}

// yaesuShapeChannelData is a fresh read that ANSWERS all seventeen
// fields, in the shape every Yaesu driver produces: "this radio has no
// such field". Each subtest below blanks exactly one of them back to the
// zero FieldState, Absent.
func yaesuShapeChannelData() codeplug.ChannelData {
	return codeplug.ChannelData{
		FreqHz: 145_500_000, Mode: "FM",
		CTCSSTone:           codeplug.ToneField{State: codeplug.Unavailable},
		TagDisplay:          codeplug.BoolField{State: codeplug.Unavailable},
		ScanSkip:            codeplug.BoolField{State: codeplug.Unavailable},
		TxFreqHz:            codeplug.FreqField{State: codeplug.Unavailable},
		Duplex:              codeplug.StringField{State: codeplug.Unavailable},
		OffsetHz:            codeplug.FreqField{State: codeplug.Unavailable},
		ToneMode:            codeplug.StringField{State: codeplug.Unavailable},
		ToneTx:              codeplug.ToneField{State: codeplug.Unavailable},
		ToneRx:              codeplug.ToneField{State: codeplug.Unavailable},
		DTCSCode:            codeplug.IntField{State: codeplug.Unavailable},
		DTCSPolarity:        codeplug.StringField{State: codeplug.Unavailable},
		Filter:              codeplug.StringField{State: codeplug.Unavailable},
		DataMode:            codeplug.BoolField{State: codeplug.Unavailable},
		TuningStepEnabled:   codeplug.BoolField{State: codeplug.Unavailable},
		TuningStep:          codeplug.StringField{State: codeplug.Unavailable},
		ProgramTuningStepHz: codeplug.FreqField{State: codeplug.Unavailable},
		AttenuatorDB:        codeplug.IntField{State: codeplug.Unavailable},
		Preamp:              codeplug.StringField{State: codeplug.Unavailable},
		Antenna:             codeplug.StringField{State: codeplug.Unavailable},
		IPPlus:              codeplug.BoolField{State: codeplug.Unavailable},
	}
}

// TestAssertFreshReadSaveLoad_RefusesAnAbsentField pins the check the
// helper was missing. Its Save/Load DeepEqual CANNOT catch an Absent
// field: NormaliseTierFields deliberately leaves an unreachable Absent
// field resolved to Unavailable on BOTH sides of the comparison, and
// leaves a reachable one Absent on both — so the round trip agrees with
// itself either way and says nothing about whether the driver answered.
//
// That mattered enough to pin because the fleet sweep in
// internal/wiring's TestOpenFakeSessionFor_EveryRegisteredModel_
// ReadsEveryDefaultSlot is vacuous for the ten Icom fakes whose default
// images have no populated channel: this helper, called on a real
// populated read by each of those drivers' own tests, is the only place
// their answer is checked.
//
// Every one of the seventeen is a separate case, so no field can be
// dropped from the assertion loop without a failure here.
func TestAssertFreshReadSaveLoad_RefusesAnAbsentField(t *testing.T) {
	for _, tt := range []struct {
		field string
		blank func(*codeplug.ChannelData)
	}{
		{"tx_frequency", func(d *codeplug.ChannelData) { d.TxFreqHz = codeplug.FreqField{} }},
		{"duplex", func(d *codeplug.ChannelData) { d.Duplex = codeplug.StringField{} }},
		{"offset", func(d *codeplug.ChannelData) { d.OffsetHz = codeplug.FreqField{} }},
		{"tone_mode", func(d *codeplug.ChannelData) { d.ToneMode = codeplug.StringField{} }},
		{"tone_tx", func(d *codeplug.ChannelData) { d.ToneTx = codeplug.ToneField{} }},
		{"tone_rx", func(d *codeplug.ChannelData) { d.ToneRx = codeplug.ToneField{} }},
		{"dtcs_code", func(d *codeplug.ChannelData) { d.DTCSCode = codeplug.IntField{} }},
		{"dtcs_polarity", func(d *codeplug.ChannelData) { d.DTCSPolarity = codeplug.StringField{} }},
		{"filter", func(d *codeplug.ChannelData) { d.Filter = codeplug.StringField{} }},
		{"data_mode", func(d *codeplug.ChannelData) { d.DataMode = codeplug.BoolField{} }},
		{"tuning_step_enabled", func(d *codeplug.ChannelData) { d.TuningStepEnabled = codeplug.BoolField{} }},
		{"tuning_step", func(d *codeplug.ChannelData) { d.TuningStep = codeplug.StringField{} }},
		{"program_tuning_step", func(d *codeplug.ChannelData) { d.ProgramTuningStepHz = codeplug.FreqField{} }},
		{"attenuator", func(d *codeplug.ChannelData) { d.AttenuatorDB = codeplug.IntField{} }},
		{"preamp", func(d *codeplug.ChannelData) { d.Preamp = codeplug.StringField{} }},
		{"antenna", func(d *codeplug.ChannelData) { d.Antenna = codeplug.StringField{} }},
		{"ip_plus", func(d *codeplug.ChannelData) { d.IPPlus = codeplug.BoolField{} }},
	} {
		t.Run(tt.field, func(t *testing.T) {
			d := yaesuShapeChannelData()
			tt.blank(&d)
			rec := &errorRecorder{TB: t}
			// The Normalised variant, because it is the shared loop:
			// AssertFreshReadSaveLoad runs it too, after its own
			// seven-field Unavailable check.
			AssertFreshReadSaveLoadNormalised(rec, codeplug.Channel{Slot: "001", Data: &d}, spec.Capabilities{}, codeplug.Load)
			if len(rec.errs) != 1 {
				t.Fatalf("helper reported %d errors, want exactly one naming %s: %v", len(rec.errs), tt.field, rec.errs)
			}
			// Match the field name EXACTLY, delimited by the fixed
			// wording either side ("fresh-read X state is Absent"):
			// a plain strings.Contains(rec.errs[0], tt.field) would
			// let "tuning_step" pass on a report that actually named
			// "tuning_step_enabled", since the former is a prefix of
			// the latter.
			want := fmt.Sprintf("fresh-read %s state is Absent", tt.field)
			if !strings.Contains(rec.errs[0], want) {
				t.Errorf("helper reported %q, want it to contain %q", rec.errs[0], want)
			}
		})
	}

	// The control: with every field answered the helper must stay
	// silent, or the table above would pass for the wrong reason.
	t.Run("all seventeen answered", func(t *testing.T) {
		d := yaesuShapeChannelData()
		rec := &errorRecorder{TB: t}
		AssertFreshReadSaveLoadNormalised(rec, codeplug.Channel{Slot: "001", Data: &d}, spec.Capabilities{}, codeplug.Load)
		if len(rec.errs) != 0 {
			t.Errorf("helper reported %v on a fully answered fresh read, want nothing", rec.errs)
		}
	})
}
