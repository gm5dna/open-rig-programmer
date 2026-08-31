// SPDX-License-Identifier: GPL-3.0-or-later

package wiring

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver/icr8600"
)

// TestICR8600Golden_ReceiverFieldsAreInCapabilityDomain machine-checks
// core/codeplug's schema-5 byte-identity golden against icr8600's own
// declared capability domains (codeplug-review.md Finding 4, carried
// forward as a closing-fix-wave minor).
//
// core/codeplug/schema_test.go's TestCanonicalV5Golden_RecordsEveryReceiverField
// asserts only that each of the golden's seven receiver fields is Known —
// nothing there ties "12.5 kHz", 20, "ON", "ANT2" or 500 Hz to
// core/driver/icr8600/caps.go. core/codeplug cannot import a driver
// package (that would be the import cycle), so that check has to live
// somewhere that imports both — this package already does, for wiring
// the driver into the registry, so it is where the check belongs.
//
// Without it, a later change to the R8600's TuningSteps or AttenuatorDB
// domain would leave the golden quietly claiming a value the model no
// longer admits, with nothing failing until a human noticed the
// mismatch. This test does not touch the golden itself (frozen, per the
// closing fix wave's constraints) — it only reads it.
func TestICR8600Golden_ReceiverFieldsAreInCapabilityDomain(t *testing.T) {
	path := filepath.Join("..", "..", "core", "codeplug", "testdata", "canonical-v5-basic.json")
	cp, err := codeplug.Load(path)
	if err != nil {
		t.Fatalf("Load(%s) error = %v", path, err)
	}
	if len(cp.Channels) == 0 || cp.Channels[0].Data == nil {
		t.Fatalf("%s: channel 0 has no Data — the fixture this test reads has changed shape", path)
	}
	d := cp.Channels[0].Data

	caps := icr8600.CapabilitiesUnverified()

	if d.TuningStep.State == codeplug.Known && !slices.Contains(caps.TuningSteps, d.TuningStep.Value) {
		t.Errorf("golden tuning_step = %q, not in icr8600's TuningSteps %v", d.TuningStep.Value, caps.TuningSteps)
	}
	if d.ProgramTuningStepHz.State == codeplug.Known && !codeplug.AdmitsProgramTuningStep(caps, d.ProgramTuningStepHz.Value) {
		t.Errorf("golden program_tuning_step = %d Hz, not admitted by icr8600's ProgramTuningStepRange %+v", d.ProgramTuningStepHz.Value, caps.ProgramTuningStepRange)
	}
	if d.AttenuatorDB.State == codeplug.Known && !slices.Contains(caps.AttenuatorDB, d.AttenuatorDB.Value) {
		t.Errorf("golden attenuator = %d, not in icr8600's AttenuatorDB %v", d.AttenuatorDB.Value, caps.AttenuatorDB)
	}
	if d.Preamp.State == codeplug.Known && !slices.Contains(caps.PreampOptions, d.Preamp.Value) {
		t.Errorf("golden preamp = %q, not in icr8600's PreampOptions %v", d.Preamp.Value, caps.PreampOptions)
	}
	if d.Antenna.State == codeplug.Known && !slices.Contains(caps.AntennaOptions, d.Antenna.Value) {
		t.Errorf("golden antenna = %q, not in icr8600's AntennaOptions %v", d.Antenna.Value, caps.AntennaOptions)
	}
}
