// SPDX-License-Identifier: GPL-3.0-or-later

package ft2000

import (
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// allCapabilityFields lists all twenty-seven spec.Fields, matching
// core/driver/ic7200/caps.go's own convention: bankFields lists every one
// of them explicitly, including the zero FieldSupport, so every field is
// "audited" and none needs a separate deliberately-unexpressed reason.
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

var deliberatelyUnexpressedFields = map[spec.Field]string{
	spec.FieldSatBandSwap: "ts2000-only: TS-2000/2000X/B2000 Satellite Memory bank flag (SA record); no home on this radio",
	spec.FieldSatTrace:    "ts2000-only: TS-2000/2000X/B2000 Satellite Memory bank flag (SA record); no home on this radio",
	spec.FieldSatTraceRev: "ts2000-only: TS-2000/2000X/B2000 Satellite Memory bank flag (SA record); no home on this radio",
}

func TestFieldAuditCoversEverySpecField(t *testing.T) {
	drivertest.AssertFieldAuditCoversEverySpecField(t, "allCapabilityFields", allCapabilityFields, deliberatelyUnexpressedFields)
}

// deliberatelyZero is the audit table TestDeliberatelyZeroAudit reflects
// over: every top-level spec.Capabilities field this driver leaves at its
// zero value, with the reason — mirrors core/driver/ic7200/caps.go's own
// shape.
var deliberatelyZero = map[string]string{
	"TagLen":                 "NoTag (matrix §0/§2.6): this radio has no channel-name route over CAT at all, so TagLen is 0 by declaration, not omission",
	"RequiredSlots":          "matrix §2.12: no manual statement that any channel must stay populated",
	"DuplexOptions":          "matrix §2.14: Icom-tier vocabulary, MANUAL-EVIDENCED ABSENCE",
	"DTCSPolarities":         "matrix §2.14: Icom-tier vocabulary, MANUAL-EVIDENCED ABSENCE",
	"DTCSCodes":              "matrix §2.14: Icom-tier vocabulary, MANUAL-EVIDENCED ABSENCE",
	"Filters":                "matrix §2.14: Icom-tier vocabulary, MANUAL-EVIDENCED ABSENCE",
	"TuningSteps":            "additions design D8: no D8 receiver field applies to this transceiver",
	"ProgramTuningStepRange": "additions design D8: as above",
	"AttenuatorDB":           "additions design D8: as above",
	"PreampOptions":          "additions design D8: as above",
	"AntennaOptions":         "additions design D8: as above",
	"TagCharset":             "NoTag (matrix §0/§2.6): no charset to declare",
	"CTCSSToneRange":         "matrix §2.8: this radio names a tone by INDEX (CTCSSTones), never a frequency number",
	"SimplexTx":              "no TxFrequency field for either FieldTxFrequency or FieldDuplex to answer the question about (matrix §2.14)",
}

func TestDeliberatelyZeroAudit(t *testing.T) {
	for _, m := range []modelParams{modelFT2000, modelFT2000D} {
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

func TestWriteTrialsComplete_PinnedFalse(t *testing.T) {
	if writeTrialsComplete {
		t.Fatal("writeTrialsComplete is true — no FT-2000 or FT-2000D has ever been asked anything by this project; flipping this constant needs real hardware evidence first")
	}
}

func TestCapabilities_Baseline(t *testing.T) {
	for _, m := range []modelParams{modelFT2000, modelFT2000D} {
		for _, c := range []spec.Capabilities{capabilitiesUnverified(m), capabilitiesSimulated(m)} {
			if c.Model != m.name {
				t.Errorf("Model = %q, want %q", c.Model, m.name)
			}
			if c.CATID != m.dialect.CATID() {
				t.Errorf("CATID = %q, want %q", c.CATID, m.dialect.CATID())
			}
			if !c.NoTag || c.TagLen != 0 {
				t.Errorf("NoTag/TagLen = %v/%d, want true/0", c.NoTag, c.TagLen)
			}
			if err := c.Validate(); err != nil {
				t.Errorf("%s: Validate: %v", m.name, err)
			}
			if len(c.Banks) != 2 {
				t.Fatalf("Banks = %d, want 2 (MEM, PMS)", len(c.Banks))
			}
			if got := len(c.Banks[0].Slots); got != 99 {
				t.Errorf("MEM slots = %d, want 99", got)
			}
			if got := len(c.Banks[1].Slots); got != 18 {
				t.Errorf("PMS slots = %d, want 18 (nine pairs)", got)
			}
		}
	}
}

func TestCapabilities_FrequencyRange(t *testing.T) {
	c := capabilitiesUnverified(modelFT2000)
	if c.MinFreqHz != 30_000 || c.MaxFreqHz != 60_000_000 {
		t.Errorf("Min/MaxFreqHz = %d/%d, want 30000/60000000 (FA/FB Set legend, ft2000_layout.txt:654/:666)", c.MinFreqHz, c.MaxFreqHz)
	}
}
