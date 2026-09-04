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

// AssertFreshReadSaveLoad pins the D8 fresh-read rule: every existing
// driver reports the seven receiver fields Unavailable, and that exact
// populated channel survives the normalised lowest-schema save/load
// migration.
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
func AssertFreshReadSaveLoadNormalised(t testing.TB, ch codeplug.Channel, caps spec.Capabilities, load func(string) (*codeplug.Codeplug, error)) {
	t.Helper()
	if ch.Data == nil {
		t.Fatal("fresh-read channel is empty")
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
