// SPDX-License-Identifier: GPL-3.0-or-later

package ftx1

import (
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// allCapabilityFields is the SIX fields the shared MR/MW/MT record always
// carries (caps.go's bankFields).
var allCapabilityFields = []spec.Field{
	spec.FieldFrequency, spec.FieldMode, spec.FieldClarifier,
	spec.FieldCTCSSState, spec.FieldShift, spec.FieldTag,
}

var deliberatelyUnexpressedFields = map[spec.Field]string{
	spec.FieldTagDisplay: "MTFormShortNoDisplay carries no display byte at all (spec.md §3.3/§8) — there is no such flag anywhere on this radio's CAT surface",
	spec.FieldCTCSSTone:  "no CAT command reads or writes a memory channel's live tone-table index on this radio",
	spec.FieldScanSkip:   "no scan-skip byte exists in the 27-byte field block (spec.md §3.1's own offset table)",
	spec.FieldErase:      "no CAT erase/clear command is documented anywhere in this manual",

	spec.FieldTxFrequency:       "Icom-tier vocabulary, MANUAL-EVIDENCED ABSENCE (matrix §6)",
	spec.FieldDuplex:            "Icom-tier vocabulary, MANUAL-EVIDENCED ABSENCE",
	spec.FieldOffset:            "Icom-tier vocabulary, MANUAL-EVIDENCED ABSENCE",
	spec.FieldToneMode:          "Icom-tier vocabulary, MANUAL-EVIDENCED ABSENCE",
	spec.FieldToneTx:            "Icom-tier vocabulary, MANUAL-EVIDENCED ABSENCE",
	spec.FieldToneRx:            "Icom-tier vocabulary, MANUAL-EVIDENCED ABSENCE",
	spec.FieldDTCSCode:          "Icom-tier vocabulary, MANUAL-EVIDENCED ABSENCE",
	spec.FieldDTCSPolarity:      "Icom-tier vocabulary, MANUAL-EVIDENCED ABSENCE",
	spec.FieldFilter:            "Icom-tier vocabulary, MANUAL-EVIDENCED ABSENCE",
	spec.FieldDataMode:          "Icom-tier vocabulary, MANUAL-EVIDENCED ABSENCE",
	spec.FieldTuningStepEnabled: "additions design D8: no D8 receiver field applies to this transceiver",
	spec.FieldTuningStep:        "additions design D8: as above",
	spec.FieldProgramTuningStep: "additions design D8: as above",
	spec.FieldAttenuator:        "additions design D8: as above",
	spec.FieldPreamp:            "additions design D8: as above",
	spec.FieldAntenna:           "additions design D8: as above",
	spec.FieldIPPlus:            "additions design D8: as above",
}

func TestFieldAuditCoversEverySpecField(t *testing.T) {
	drivertest.AssertFieldAuditCoversEverySpecField(t, "allCapabilityFields", allCapabilityFields, deliberatelyUnexpressedFields)
}

func TestWriteTrialsComplete_PinnedFalse(t *testing.T) {
	if writeTrialsComplete {
		t.Fatal("writeTrialsComplete is true — no FTX-1 has ever been asked anything by this project")
	}
}

func TestCapabilities_Baseline(t *testing.T) {
	for _, c := range []spec.Capabilities{CapabilitiesUnverified(), CapabilitiesSimulated()} {
		if c.Model != modelName {
			t.Errorf("Model = %q, want %q", c.Model, modelName)
		}
		if c.CATID != dialect.CATID() {
			t.Errorf("CATID = %q, want %q", c.CATID, dialect.CATID())
		}
		if c.NoTag || c.TagLen != 12 {
			t.Errorf("NoTag/TagLen = %v/%d, want false/12", c.NoTag, c.TagLen)
		}
		if err := c.Validate(); err != nil {
			t.Errorf("Validate: %v", err)
		}
		if len(c.Banks) != 4 {
			t.Fatalf("Banks = %d, want 4 (MEM, PMS, 60M, EMG)", len(c.Banks))
		}
		wantIDs := []spec.BankID{spec.BankMemory, spec.BankPMS, spec.Bank60m, spec.BankEMG}
		for i, want := range wantIDs {
			if c.Banks[i].ID != want {
				t.Errorf("Banks[%d].ID = %s, want %s", i, c.Banks[i].ID, want)
			}
		}
		if got := len(c.Banks[0].Slots); got != 999 {
			t.Errorf("MEM slots = %d, want 999", got)
		}
		if got := len(c.Banks[1].Slots); got != 100 {
			t.Errorf("PMS slots = %d, want 100 (fifty pairs)", got)
		}
		if got := len(c.Banks[2].Slots); got != 20 {
			t.Errorf("5 MHz slots = %d, want 20", got)
		}
		if got := len(c.Banks[3].Slots); got != 1 {
			t.Errorf("EMGCH slots = %d, want 1", got)
		}
	}
}

// TestCapabilities_60mAndEMGAlwaysReadOnly pins caps.go's bankFields
// writable=false arm: the 5 MHz and EMGCH banks never grade a write
// above Unsupported, on EITHER profile — MW cannot target either bank
// (cat.Dialect.writableSlot).
func TestCapabilities_60mAndEMGAlwaysReadOnly(t *testing.T) {
	for _, c := range []spec.Capabilities{CapabilitiesUnverified(), CapabilitiesSimulated()} {
		for _, id := range []spec.BankID{spec.Bank60m, spec.BankEMG} {
			for _, f := range allCapabilityFields {
				if fs := c.FieldSupport(id, f); fs.Write != spec.Unsupported {
					t.Errorf("bank %s field %s: Write = %v, want Unsupported", id, f, fs.Write)
				}
			}
		}
	}
}

func TestCapabilities_ClarifierDerivesFromDialect(t *testing.T) {
	c := CapabilitiesUnverified()
	if c.ClarMaxHz != dialect.Clarifier().MaxAbsHz || c.ClarStepHz != dialect.Clarifier().StepHz {
		t.Errorf("Clar Max/Step = %d/%d, want %d/%d (from the dialect)", c.ClarMaxHz, c.ClarStepHz, dialect.Clarifier().MaxAbsHz, dialect.Clarifier().StepHz)
	}
}

// TestCapabilities_ToneModesSixValues pins the FTX-1's own wider tone
// vocabulary (spec.md §6): six values, none of them
// spec.StandardToneModes()' three.
func TestCapabilities_ToneModesSixValues(t *testing.T) {
	c := CapabilitiesUnverified()
	if len(c.ToneModes) != 6 {
		t.Fatalf("ToneModes = %d entries, want 6", len(c.ToneModes))
	}
	want := map[string]spec.ToneModeSemantics{
		"OFF": spec.ToneModeOff, "ENC-DEC": spec.ToneModeCTCSSSquelch, "ENC": spec.ToneModeCTCSS,
		"DCS": spec.ToneModeDCSEncodeDecode, "PR-FREQ": spec.ToneModePRFreq, "REV-TONE": spec.ToneModeRevTone,
	}
	for _, tm := range c.ToneModes {
		sem, ok := want[tm.Value]
		if !ok {
			t.Errorf("unexpected ToneModes value %q", tm.Value)
			continue
		}
		if tm.Semantics != sem {
			t.Errorf("ToneModes[%q].Semantics = %v, want %v", tm.Value, tm.Semantics, sem)
		}
	}
}

func TestCapabilities_ModelIsNotBodySuffixed(t *testing.T) {
	if modelName != "FTX-1" {
		t.Errorf("modelName = %q, want \"FTX-1\" — no body suffix (doc.go register item 1)", modelName)
	}
}

// deliberatelyZero is the audit table TestDeliberatelyZeroAudit reflects
// over — every spec.Capabilities field this driver leaves at its zero
// value, and why.
var deliberatelyZero = map[string]string{
	"NoTag":                  "this radio HAS a tag route (MT, TagLen 12) — NoTag's zero value (false) is simply correct here, not an omission",
	"SimplexTx":              "no TxFrequency field for either FieldTxFrequency or FieldDuplex to answer the question about",
	"CTCSSTones":             "matrix §6: no CTCSS tone-frequency chart located in the manual excerpts this spec pass read — OPEN, doc.go register item 9",
	"CTCSSToneRange":         "this radio names a tone by STATE (P8), never a frequency number or index",
	"MinFreqHz":              "matrix §6: not derived this pass — OPEN, doc.go register item 9",
	"MaxFreqHz":              "as above",
	"RequiredSlots":          "matrix §6: ASSUMED absent, no manual statement of a mandatory slot",
	"DuplexOptions":          "Icom-tier vocabulary, MANUAL-EVIDENCED ABSENCE",
	"DTCSPolarities":         "Icom-tier vocabulary, MANUAL-EVIDENCED ABSENCE",
	"DTCSCodes":              "Icom-tier vocabulary, MANUAL-EVIDENCED ABSENCE",
	"Filters":                "Icom-tier vocabulary, MANUAL-EVIDENCED ABSENCE",
	"TuningSteps":            "additions design D8: no D8 receiver field applies to this transceiver",
	"ProgramTuningStepRange": "additions design D8: as above",
	"AttenuatorDB":           "additions design D8: as above",
	"PreampOptions":          "additions design D8: as above",
	"AntennaOptions":         "additions design D8: as above",
	"TagCharset":             "not declared this pass — ASCII per spec.md §8, no charset enum exists on Capabilities beyond TagLen",
}

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
