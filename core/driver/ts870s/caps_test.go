// SPDX-License-Identifier: GPL-3.0-or-later

package ts870s

import (
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// allCapabilityFields is every spec.Field this driver's bank maps to a
// non-zero FieldSupport (matrix §2 bank table's "rw" rows).
var allCapabilityFields = []spec.Field{
	spec.FieldFrequency,
	spec.FieldMode,
	spec.FieldScanSkip,
	spec.FieldTxFrequency,
	spec.FieldToneMode,
	spec.FieldToneTx,
}

// deliberatelyUnexpressedFields is every other spec.Field, each with the
// bank-table reason bankFields' own comments cite (matrix §2).
var deliberatelyUnexpressedFields = map[spec.Field]string{
	spec.FieldSatBandSwap:  "ts2000-only: TS-2000/2000X/B2000 Satellite Memory bank flag (SA record); no home on this radio",
	spec.FieldSatTrace:     "ts2000-only: TS-2000/2000X/B2000 Satellite Memory bank flag (SA record); no home on this radio",
	spec.FieldSatTraceRev:  "ts2000-only: TS-2000/2000X/B2000 Satellite Memory bank flag (SA record); no home on this radio",
	spec.FieldClarifier:    "matrix §1.7/§1.8, §2: no clarifier field in the 22-byte record",
	spec.FieldCTCSSState:   "matrix §1.16/§1.19, §2: this row expresses ToneModes, the Icom-style pair, not CTCSSStates",
	spec.FieldCTCSSTone:    "matrix §1.9/§2: the single tone index is FieldToneTx, not FieldCTCSSTone",
	spec.FieldShift:        "matrix §1.16/§2: no shift selector; split is the two P1 frames",
	spec.FieldTag:          "matrix §1.6/§2: NoTag",
	spec.FieldTagDisplay:   "matrix §1.6/§2: NoTag",
	spec.FieldErase:        "matrix §2: CHOICE over undocumented short/zeroed-MW behaviour",
	spec.FieldDuplex:       "matrix §1.18/§2: no duplex selector or offset-magnitude field in the 22-byte record",
	spec.FieldOffset:       "matrix §1.18/§2: as FieldDuplex",
	spec.FieldToneRx:       "matrix §1.19/§2: no P9 byte exists at all on this row — a single tone index, not merely one this radio leaves fixed",
	spec.FieldDTCSCode:     "matrix §1.20/§1.21/§2: zero DCS/DTCS hits anywhere in the document",
	spec.FieldDTCSPolarity: "matrix §1.20/§1.21/§2: as FieldDTCSCode",
	spec.FieldFilter:       "matrix §1.22/§2: no FILTER A/B byte in the record",
	spec.FieldDataMode:     "matrix §2: no data-mode byte anywhere in the 22-byte frame",

	spec.FieldTuningStepEnabled: "additions design D8: no D8 receiver field applies to any of this wave's seven packages",
	spec.FieldTuningStep:        "additions design D8: as above",
	spec.FieldProgramTuningStep: "additions design D8: as above",
	spec.FieldAttenuator:        "additions design D8: as above",
	spec.FieldPreamp:            "additions design D8: as above",
	spec.FieldAntenna:           "additions design D8: as above",
	spec.FieldIPPlus:            "an Icom concept; no position here and no mention in this document",
}

func TestFieldAuditCoversEverySpecField(t *testing.T) {
	drivertest.AssertFieldAuditCoversEverySpecField(t, "allCapabilityFields", allCapabilityFields, deliberatelyUnexpressedFields)
}

// TestDeliberatelyZeroAudit is the brief's own proof obligation: every
// spec.Capabilities field this driver leaves at its zero value must be
// named in deliberatelyZero, and every field named there must actually be
// zero — the ic7200 shape.
func TestDeliberatelyZeroAudit(t *testing.T) {
	for _, caps := range []spec.Capabilities{CapabilitiesUnverified(), CapabilitiesSimulated()} {
		v := reflect.ValueOf(caps)
		for i := 0; i < v.NumField(); i++ {
			name := v.Type().Field(i).Name
			zero := v.Field(i).IsZero() || (v.Field(i).Kind() == reflect.Slice && v.Field(i).Len() == 0)
			_, listed := deliberatelyZero[name]
			if zero != listed {
				t.Errorf("%s zero=%v listed=%v", name, zero, listed)
			}
		}
	}
}

func TestCapabilitiesShape(t *testing.T) {
	c := CapabilitiesUnverified()
	if c.Model != "TS-870S" || c.CATID != "015" || c.Transmit != spec.HasTransmitter {
		t.Fatalf("capabilities = %+v", c)
	}
	if c.TagLen != 0 || !c.NoTag {
		t.Fatalf("NoTag capabilities = TagLen %d, NoTag %v, want 0, true", c.TagLen, c.NoTag)
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	mem, ok := c.Bank(spec.BankMemory)
	if !ok || len(mem.Slots) != 100 || mem.Slots[0] != "00" || mem.Slots[99] != "99" {
		t.Fatalf("MEM = %+v", mem)
	}
	if len(c.CTCSSTones) != 39 {
		t.Fatalf("CTCSSTones has %d entries, want 39 (matrix §1.9)", len(c.CTCSSTones))
	}
	if len(c.Modes) != 8 {
		t.Fatalf("Modes has %d entries, want 8 (matrix §1.5)", len(c.Modes))
	}
	if len(c.Bauds) != 7 {
		t.Fatalf("Bauds has %d entries, want 7 (matrix §1.11, all seven printed rates)", len(c.Bauds))
	}
}

func TestWriteTrialsComplete_PinnedFalse(t *testing.T) {
	if writeTrialsComplete {
		t.Fatal("write trial guard unlocked")
	}
	for _, b := range CapabilitiesUnverified().Banks {
		for f, sup := range b.Fields {
			if sup.CanWrite() {
				t.Errorf("%s is writable on bank %s with the write-trial guard false", f, b.ID)
			}
		}
	}
}

func TestBaseline_Validate(t *testing.T) {
	for name, caps := range map[string]spec.Capabilities{
		"unverified":         CapabilitiesUnverified(),
		"simulated":          CapabilitiesSimulated(),
		"unverified+consent": spec.ConsentUnverifiedWrites(CapabilitiesUnverified()),
		"simulated+consent":  spec.ConsentUnverifiedWrites(CapabilitiesSimulated()),
		"New constructor":    New(RealHardware).Capabilities(),
		"New simulated arm":  New(Simulated).Capabilities(),
	} {
		if err := caps.Validate(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// Open/ReadChannel/WriteChannel are exercised against a scripted radio in
// ts870s_test.go (respondingport_test.go), now that a live session is
// wired up — see the package doc comment (ts870s.go) for why that
// changed.
