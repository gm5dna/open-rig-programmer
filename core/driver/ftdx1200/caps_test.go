// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx1200

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

// deliberatelyZero is the audit table TestDeliberatelyZeroAudit reflects
// over.
var deliberatelyZero = map[string]string{
	"TagLen":                 "NoTag (matrix §0/§2.6): this radio has no channel-name route over CAT at all, so TagLen is 0 by declaration, not omission",
	"RequiredSlots":          "matrix §2.12: no manual statement that any channel must stay populated",
	"DuplexOptions":          "matrix §2.14: Icom-tier vocabulary, MANUAL-EVIDENCED ABSENCE",
	"ToneModes":              "matrix §2.14: Icom-tier vocabulary, MANUAL-EVIDENCED ABSENCE",
	"DTCSPolarities":         "matrix §2.14: Icom-tier vocabulary, MANUAL-EVIDENCED ABSENCE",
	"DTCSCodes":              "matrix §2.14: Icom-tier vocabulary, MANUAL-EVIDENCED ABSENCE",
	"Filters":                "matrix §2.14: Icom-tier vocabulary, MANUAL-EVIDENCED ABSENCE",
	"TuningSteps":            "additions design D8: no D8 receiver field applies to this transceiver",
	"ProgramTuningStepRange": "additions design D8: as above",
	"AttenuatorDB":           "additions design D8: as above",
	"PreampOptions":          "additions design D8: as above",
	"AntennaOptions":         "additions design D8: as above",
	"TagCharset":             "NoTag (matrix §0/§2.6): no charset to declare",
	"CTCSSToneRange":         "matrix §2.8: this radio names a tone by INDEX, never a frequency number",
	"SimplexTx":              "no TxFrequency field for either FieldTxFrequency or FieldDuplex to answer the question about (matrix §2.14)",
}

func TestDeliberatelyZeroAudit(t *testing.T) {
	for _, caps := range []spec.Capabilities{capabilitiesUnverified(), capabilitiesSimulated()} {
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

func TestWriteTrialsComplete_PinnedFalse(t *testing.T) {
	if writeTrialsComplete {
		t.Fatal("writeTrialsComplete is true — no FTDX1200 has ever been asked anything by this project")
	}
}

func TestCapabilities_Baseline(t *testing.T) {
	for _, c := range []spec.Capabilities{capabilitiesUnverified(), capabilitiesSimulated()} {
		if c.Model != modelName {
			t.Errorf("Model = %q, want %q", c.Model, modelName)
		}
		if c.CATID != dialect.CATID() {
			t.Errorf("CATID = %q, want %q", c.CATID, dialect.CATID())
		}
		if !c.NoTag || c.TagLen != 0 {
			t.Errorf("NoTag/TagLen = %v/%d, want true/0", c.NoTag, c.TagLen)
		}
		if err := c.Validate(); err != nil {
			t.Errorf("Validate: %v", err)
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

// TestCapabilities_CTCSSToneAlwaysZero pins the sibling contrast:
// FieldCTCSSTone is the zero FieldSupport on BOTH directions, on BOTH
// profiles — no read-side liveness the way ftdx3000 has.
func TestCapabilities_CTCSSToneAlwaysZero(t *testing.T) {
	for _, c := range []spec.Capabilities{capabilitiesUnverified(), capabilitiesSimulated()} {
		fs := c.FieldSupport(spec.BankMemory, spec.FieldCTCSSTone)
		if fs.Read != spec.Unsupported || fs.Write != spec.Unsupported {
			t.Errorf("FieldCTCSSTone = %+v, want the zero FieldSupport on both directions", fs)
		}
	}
}

func TestCapabilities_FrequencyRange(t *testing.T) {
	c := capabilitiesUnverified()
	if c.MinFreqHz != 30_000 || c.MaxFreqHz != 60_000_000 {
		t.Errorf("Min/MaxFreqHz = %d/%d, want 30000/60000000", c.MinFreqHz, c.MaxFreqHz)
	}
}
