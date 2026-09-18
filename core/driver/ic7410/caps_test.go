// SPDX-License-Identifier: GPL-3.0-or-later

package ic7410

import (
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

func TestConstructor(t *testing.T) {
	d := New()
	if d.Model() != "IC-7410" {
		t.Fatalf("Model = %q", d.Model())
	}
	c := d.Capabilities()
	if c.CATID != "80" || c.Transmit != spec.HasTransmitter || c.TagLen != 9 {
		t.Fatalf("capabilities = %+v", c)
	}
	if c.SimplexTx != spec.SimplexTxEqualsRx {
		t.Fatalf("SimplexTx = %v, want SimplexTxEqualsRx", c.SimplexTx)
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestCapabilitiesShape(t *testing.T) {
	c := New().Capabilities()
	mem, ok := c.Bank(spec.BankMemory)
	if !ok || len(mem.Slots) != 99 || mem.Slots[0] != "0001" || mem.Slots[98] != "0099" {
		t.Fatalf("MEM = %+v", mem)
	}
	scan, ok := c.Bank(spec.BankScan)
	if !ok || len(scan.Slots) != 2 || scan.Slots[0] != "P1" || scan.Slots[1] != "P2" {
		t.Fatalf("SCAN = %+v", scan)
	}
	if _, ok := c.Bank(spec.BankCall); ok {
		t.Fatal("CALL bank unexpectedly present — matrix §1b: no call-channel form or command anywhere in the 124-page document")
	}

	// Write POSSIBILITY (as opposed to CanWrite, which additionally requires
	// consent on the real-hardware profile) is graded by Write != Unsupported,
	// checked against the SIMULATED profile: New().Capabilities() alone is the
	// real-hardware, pre-consent baseline, where CanWrite() is false for
	// every field until consent is recorded (spec.Capabilities.CanWrite's own
	// contract). tx_frequency: Sup/Sup on MEM, Sup/Uns on SCAN (matrix §2).
	sim := New(WithSimulatedProfile()).Capabilities()
	simMem, _ := sim.Bank(spec.BankMemory)
	simScan, _ := sim.Bank(spec.BankScan)
	if !simMem.Fields[spec.FieldTxFrequency].CanWrite() {
		t.Error("MEM tx_frequency is not writable under the simulated profile, want Sup/Sup")
	}
	if scan.Fields[spec.FieldTxFrequency].Read == spec.Unsupported {
		t.Error("SCAN tx_frequency is not readable, want Sup/Uns")
	}
	if simScan.Fields[spec.FieldTxFrequency].CanWrite() {
		t.Error("SCAN tx_frequency is writable, want Uns — byte ③'s Split nibble must read 0 on P1/P2 (matrix §2 SCAN row 11)")
	}

	// data_mode: genuinely writable on this model, unlike the IC-7610
	// family, because it occupies a whole byte (matrix §1b, offset 8 row).
	for _, b := range sim.Banks {
		if !b.Fields[spec.FieldDataMode].CanWrite() {
			t.Errorf("bank %s: data_mode is not writable under the simulated profile — this model's ⑪ is a genuine whole-byte boolean, not the IC-7610 family's nibble-shared four-valued version", b.ID)
		}
		if b.Fields[spec.FieldScanSkip].CanWrite() || b.Fields[spec.FieldDuplex].CanWrite() {
			t.Errorf("bank %s: an unmapped field is writable", b.ID)
		}
	}
}

// allFields is the twenty non-D8 spec.Fields, in core/spec/field.go's own
// declaration order — every one this model's capability grid explicitly
// grades, Sup or zero.
var allFields = []spec.Field{
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

	spec.FieldTxFrequency,
	spec.FieldDuplex,
	spec.FieldOffset,
	spec.FieldToneMode,
	spec.FieldToneTx,
	spec.FieldToneRx,
	spec.FieldDTCSCode,
	spec.FieldDTCSPolarity,
	spec.FieldFilter,
	spec.FieldDataMode,
}

// deliberatelyUnexpressedFields is the seven D8 receiver fields, absent
// from memFields/scanFields entirely (a map lookup for any of them returns
// the zero FieldSupport implicitly).
var deliberatelyUnexpressedFields = map[spec.Field]string{
	spec.FieldTuningStepEnabled: "additions design D8 — the IC-7410 40-byte record carries no tuning-step-enabled field",
	spec.FieldTuningStep:        "additions design D8 — no tuning-step field",
	spec.FieldProgramTuningStep: "additions design D8 — no programmable-tuning-step field",
	spec.FieldAttenuator:        "additions design D8 — no attenuator field",
	spec.FieldPreamp:            "additions design D8 — no preamp field",
	spec.FieldAntenna:           "additions design D8 — no antenna-selection field",
	spec.FieldIPPlus:            "additions design D8 — no IP+ field",
}

func TestFieldAuditCoversEverySpecField(t *testing.T) {
	drivertest.AssertFieldAuditCoversEverySpecField(t, "allFields", allFields, deliberatelyUnexpressedFields)
}

// TestCapabilities_EveryFieldExplicit is the spec.Capabilities struct-field
// audit: caps.go's deliberatelyZero map and every non-zero field in
// baseCapabilities must partition the struct's fields exactly.
func TestCapabilities_EveryFieldExplicit(t *testing.T) {
	caps := New().Capabilities()
	v := reflect.ValueOf(caps)
	ty := v.Type()
	if ty.NumField() != 29 {
		t.Errorf("spec.Capabilities has %d fields, want 29", ty.NumField())
	}
	for i := 0; i < ty.NumField(); i++ {
		name := ty.Field(i).Name
		zero := v.Field(i).IsZero()
		_, listed := deliberatelyZero[name]
		if zero != listed {
			t.Errorf("Capabilities.%s zero=%v listed-in-deliberatelyZero=%v", name, zero, listed)
		}
	}
}

func TestWriteTrialsComplete_PinnedFalse(t *testing.T) {
	if writeTrialsComplete {
		t.Fatal("writeTrialsComplete is true — no IC-7410 has ever been asked anything by this project; flipping this requires a real capabilitiesRealHardware profile built from an IC-7410's own trial evidence, per caps.go's doc comment")
	}
	caps := capabilitiesUnverified()
	for _, b := range caps.Banks {
		for f, sup := range b.Fields {
			if sup.Write == spec.Supported {
				t.Errorf("bank %s field %s is Write Supported while writeTrialsComplete is false — nothing may be writable without consent until a real write has been confirmed", b.ID, f)
			}
		}
	}
}

func TestSimulatedCapabilitiesAreFullyWritable(t *testing.T) {
	caps := capabilitiesSimulated()
	for _, b := range caps.Banks {
		for _, f := range []spec.Field{spec.FieldFrequency, spec.FieldMode, spec.FieldFilter, spec.FieldDataMode, spec.FieldToneMode, spec.FieldToneTx, spec.FieldToneRx, spec.FieldTag} {
			if !b.Fields[f].CanWrite() {
				t.Errorf("bank %s field %s is not writable under the simulated profile", b.ID, f)
			}
		}
	}
}
