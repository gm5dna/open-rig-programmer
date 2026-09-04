// SPDX-License-Identifier: GPL-3.0-or-later

// Package drivertest contains assertions shared by driver package tests.
package drivertest

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// AssertFreshReadSaveLoad pins the D8 fresh-read rule for a driver that
// has no receiver fields: it reports all seven Unavailable, and that
// exact populated channel survives the normalised lowest-schema
// save/load migration.
//
// It is NOT every driver: the IC-R8600 reports the seven Known, which is
// why it calls AssertFreshReadSaveLoadNormalised directly
// (core/driver/icr8600/probe_read_test.go). The shared half of the two —
// the no-Absent check and the round trip — is in that function.
func AssertFreshReadSaveLoad(t testing.TB, ch codeplug.Channel, caps spec.Capabilities, load func(string) (*codeplug.Codeplug, error)) {
	t.Helper()
	if ch.Data == nil {
		t.Fatal("fresh-read channel is empty")
	}
	for name, state := range map[string]codeplug.FieldState{
		"tuning_step_enabled": ch.Data.TuningStepEnabled.State,
		"tuning_step":         ch.Data.TuningStep.State,
		"program_tuning_step": ch.Data.ProgramTuningStepHz.State,
		"attenuator":          ch.Data.AttenuatorDB.State,
		"preamp":              ch.Data.Preamp.State,
		"antenna":             ch.Data.Antenna.State,
		"ip_plus":             ch.Data.IPPlus.State,
	} {
		if state != codeplug.Unavailable {
			t.Errorf("fresh-read %s state = %q, want Unavailable", name, state)
		}
	}
	AssertFreshReadSaveLoadNormalised(t, ch, caps, load)
}

// AssertFreshReadSaveLoadNormalised checks the composition-root path for
// a fresh channel whose D8 states are pinned by its caller. Both sides
// are normalised before comparison because Load itself deliberately has
// no capabilities with which to resolve reachable and unreachable
// Absent fields.
//
// It also asserts, BEFORE Save, that the fresh read left none of the
// seventeen tier fields Absent: a read must ANSWER every field — Known,
// Unknown or Unavailable — because Absent is the one state that means
// "nobody has said anything", which a read has no business producing.
// codeplug.Validate refuses a reachable Absent field at the send gate,
// and codeplug.FieldState.RepresentableByOmission makes an Absent one
// promote the saved schema, so a driver that left one would break byte
// identity as well.
//
// The round trip below CANNOT stand in for that check, which is why it
// is written out: NormaliseTierFields resolves an unreachable Absent
// field to Unavailable on both sides of the comparison and leaves a
// reachable one Absent on both, so the DeepEqual agrees with itself
// whatever the driver answered. TestAssertFreshReadSaveLoad_
// RefusesAnAbsentField pins each of the seventeen separately.
//
// This is also where the fleet sweep's gap is closed. internal/wiring's
// TestOpenFakeSessionFor_EveryRegisteredModel_ReadsEveryDefaultSlot
// makes the same no-Absent assertion, but only over channels a default
// fake image actually populates — which ten of the Icom fakes do not.
// The check here runs at every per-driver call site, on that driver's
// own populated read.
func AssertFreshReadSaveLoadNormalised(t testing.TB, ch codeplug.Channel, caps spec.Capabilities, load func(string) (*codeplug.Codeplug, error)) {
	t.Helper()
	if ch.Data == nil {
		t.Fatal("fresh-read channel is empty")
	}
	for _, f := range tierFieldStates(ch.Data) {
		if f.state == codeplug.Absent {
			t.Errorf("fresh-read %s state is Absent; a read must answer every tier field — Known, Unknown or Unavailable", f.name)
		}
	}
	normalised := ch
	data := *ch.Data
	normalised.Data = &data

	cp := &codeplug.Codeplug{Generator: "driver fresh-read test", Channels: []codeplug.Channel{normalised}}
	codeplug.NormaliseTierFields(cp, caps)
	path := filepath.Join(t.TempDir(), "fresh-read.json")
	if err := codeplug.Save(path, cp); err != nil {
		t.Fatalf("Save(fresh read): %v", err)
	}
	got, err := load(path)
	if err != nil {
		t.Fatalf("Load(saved fresh read): %v", err)
	}
	codeplug.NormaliseTierFields(got, caps)
	if len(got.Channels) != 1 || !reflect.DeepEqual(got.Channels[0], normalised) {
		t.Errorf("fresh read differs after normalised save/load:\n got %+v\nwant %+v", got.Channels, normalised)
	}
}

// tierFieldState names one of the seventeen fields the two Icom model
// extensions added to codeplug.ChannelData, for the no-Absent sweep. A
// SLICE, not a map, so a failing driver reports its fields in the same
// order every run.
type tierFieldState struct {
	name  string
	state codeplug.FieldState
}

// tierFieldStates lists all seventeen — D4's ten and D8's seven — under
// the column names core/csvio and the file schemas use for them. Adding
// an eighteenth tier field without adding it here would silently narrow
// the assertion, which is why
// TestAssertFreshReadSaveLoad_RefusesAnAbsentField tables every entry.
func tierFieldStates(d *codeplug.ChannelData) []tierFieldState {
	return []tierFieldState{
		{"tx_frequency", d.TxFreqHz.State},
		{"duplex", d.Duplex.State},
		{"offset", d.OffsetHz.State},
		{"tone_mode", d.ToneMode.State},
		{"tone_tx", d.ToneTx.State},
		{"tone_rx", d.ToneRx.State},
		{"dtcs_code", d.DTCSCode.State},
		{"dtcs_polarity", d.DTCSPolarity.State},
		{"filter", d.Filter.State},
		{"data_mode", d.DataMode.State},
		{"tuning_step_enabled", d.TuningStepEnabled.State},
		{"tuning_step", d.TuningStep.State},
		{"program_tuning_step", d.ProgramTuningStepHz.State},
		{"attenuator", d.AttenuatorDB.State},
		{"preamp", d.Preamp.State},
		{"antenna", d.Antenna.State},
		{"ip_plus", d.IPPlus.State},
	}
}
