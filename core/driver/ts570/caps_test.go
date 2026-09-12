// SPDX-License-Identifier: GPL-3.0-or-later

package ts570

import (
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

var allCapabilityFields = []spec.Field{
	spec.FieldFrequency, spec.FieldMode, spec.FieldClarifier,
	spec.FieldCTCSSState, spec.FieldCTCSSTone, spec.FieldShift,
	spec.FieldTag, spec.FieldTagDisplay, spec.FieldScanSkip, spec.FieldErase,
	spec.FieldTxFrequency, spec.FieldDuplex, spec.FieldOffset,
	spec.FieldToneMode, spec.FieldToneTx, spec.FieldToneRx,
	spec.FieldDTCSCode, spec.FieldDTCSPolarity, spec.FieldFilter,
	spec.FieldDataMode, spec.FieldTuningStepEnabled, spec.FieldTuningStep,
	spec.FieldProgramTuningStep, spec.FieldAttenuator, spec.FieldPreamp,
	spec.FieldAntenna, spec.FieldIPPlus,
}

var deliberatelyUnexpressedFields = map[spec.Field]string{}

func TestFieldAuditCoversEverySpecField(t *testing.T) {
	drivertest.AssertFieldAuditCoversEverySpecField(t, "allCapabilityFields", allCapabilityFields, deliberatelyUnexpressedFields)
}

// TestDeliberatelyZeroAudit reflects over every row's Capabilities and
// requires deliberatelyZero to name exactly the zero-valued fields — the
// ic7200 shape this brief names as the NoTag citation precedent.
func TestDeliberatelyZeroAudit(t *testing.T) {
	for _, m := range []modelParams{modelD, modelS, modelDG} {
		for _, caps := range []spec.Capabilities{capabilitiesUnverified(m), capabilitiesSimulated(m)} {
			v := reflect.ValueOf(caps)
			for i := 0; i < v.NumField(); i++ {
				name := v.Type().Field(i).Name
				zero := v.Field(i).IsZero() || (v.Field(i).Kind() == reflect.Slice && v.Field(i).Len() == 0)
				_, listed := deliberatelyZero[name]
				if zero != listed {
					t.Errorf("%s: %s zero=%v listed=%v", m.name, name, zero, listed)
				}
			}
		}
	}
}

// TestWriteTrialsComplete_PinnedFalse is this row's own hardware-write
// guard, referenced so it is not an orphaned constant.
func TestWriteTrialsComplete_PinnedFalse(t *testing.T) {
	if writeTrialsComplete {
		t.Fatal("writeTrialsComplete flipped true with no accompanying hardware-verified profile and evidence link — see core/driver/ts480/caps.go's own pin")
	}
}

// TestCapabilitiesShape pins the per-row facts a caller relies on: three
// distinct models, two distinct CAT IDs (DG assumes the D's), NoTag, and a
// valid Capabilities value for every row and profile.
func TestCapabilitiesShape(t *testing.T) {
	cases := []struct {
		m         modelParams
		wantModel string
		wantCATID string
	}{
		{modelD, "TS-570D", "017"},
		{modelS, "TS-570S", "018"},
		{modelDG, "TS-570DG", "017"},
	}
	for _, tc := range cases {
		for _, caps := range []spec.Capabilities{capabilitiesUnverified(tc.m), capabilitiesSimulated(tc.m)} {
			if caps.Model != tc.wantModel || caps.CATID != tc.wantCATID {
				t.Errorf("%s: Model/CATID = %q/%q, want %q/%q", tc.wantModel, caps.Model, caps.CATID, tc.wantModel, tc.wantCATID)
			}
			if caps.TagLen != 0 || !caps.NoTag {
				t.Errorf("%s: NoTag capabilities = TagLen %d, NoTag %v, want 0, true", tc.wantModel, caps.TagLen, caps.NoTag)
			}
			if err := caps.Validate(); err != nil {
				t.Fatalf("%s: Validate: %v", tc.wantModel, err)
			}
			mem, ok := caps.Bank(spec.BankMemory)
			if !ok || len(mem.Slots) != 100 || mem.Slots[0] != "00" || mem.Slots[99] != "99" {
				t.Fatalf("%s: MEM = %+v", tc.wantModel, mem)
			}
			if len(caps.Modes) != 8 {
				t.Errorf("%s: Modes = %v, want 8 entries (matrix §1.3)", tc.wantModel, caps.Modes)
			}
			if len(caps.CTCSSTones) != 39 {
				t.Errorf("%s: CTCSSTones has %d entries, want 39 (matrix §2)", tc.wantModel, len(caps.CTCSSTones))
			}
		}
	}
}

// TestToneChart_IsThisRowsOwnNotTS590s pins the divergence doc.go names:
// three conventional tones ts590's 43-entry chart carries are absent here.
func TestToneChart_IsThisRowsOwnNotTS590s(t *testing.T) {
	for _, absent := range []spec.Tone{693, 2065, 2291} { // 69.3, 206.5, 229.1 Hz
		for _, got := range ts570CTCSSTones {
			if got == absent {
				t.Errorf("ts570CTCSSTones contains %v, which this row's own manual table (layout lines 2179-2190) does not print", absent)
			}
		}
	}
	if got := ts570CTCSSTones[38]; got != 17500 {
		t.Errorf("ts570CTCSSTones[38] (wire index 39) = %v, want 1750.0 Hz", got)
	}
}
