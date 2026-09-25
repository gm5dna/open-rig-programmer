// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx5000

import (
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// allFields is the spec.Fields this driver EXPRESSES — maps to real
// FieldSupport in bankFields, whether or not that support is zero. Six are
// live (Frequency, Mode, Clarifier, CTCSSState, CTCSSTone, Shift); four are
// the zero FieldSupport by declaration (Tag, TagDisplay, ScanSkip, Erase).
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
}

// deliberatelyUnexpressedFields is the seventeen Icom-tier (D4/D8) fields
// this radio's 27-byte record has no room for at all. The "design D4"
// entries are the fields of yaesu.TierRequestedFields
// (core/driver/internal/yaesu/write.go) that this radio's memory frame
// cannot carry.
var deliberatelyUnexpressedFields = map[spec.Field]string{
	spec.FieldSatBandSwap:       "ts2000-only: TS-2000/2000X/B2000 Satellite Memory bank flag (SA record); no home on this radio",
	spec.FieldSatTrace:          "ts2000-only: TS-2000/2000X/B2000 Satellite Memory bank flag (SA record); no home on this radio",
	spec.FieldSatTraceRev:       "ts2000-only: TS-2000/2000X/B2000 Satellite Memory bank flag (SA record); no home on this radio",
	spec.FieldTxFrequency:       "design D4 — the FTdx5000 memory frame carries no independent transmit-frequency field (matrix §2)",
	spec.FieldDuplex:            "design D4 — the FTdx5000 memory frame carries no Icom duplex field (matrix §2)",
	spec.FieldOffset:            "design D4 — the FTdx5000 memory frame carries no per-channel repeater-offset field (matrix §2)",
	spec.FieldToneMode:          "design D4 — the FTdx5000 memory frame carries no Icom tone-mode field (matrix §2)",
	spec.FieldToneTx:            "design D4 — the FTdx5000 memory frame carries no separate transmit-tone field (matrix §2)",
	spec.FieldToneRx:            "design D4 — the FTdx5000 memory frame carries no separate receive-tone field (matrix §2)",
	spec.FieldDTCSCode:          "design D4 — the FTdx5000 memory frame carries no DTCS-code field (matrix §2)",
	spec.FieldDTCSPolarity:      "design D4 — the FTdx5000 memory frame carries no DTCS-polarity field (matrix §2)",
	spec.FieldFilter:            "design D4 — the FTdx5000 memory frame carries no per-channel IF-filter field (matrix §2)",
	spec.FieldDataMode:          "design D4 — the FTdx5000 memory frame carries no Icom data-mode flag (matrix §2)",
	spec.FieldTuningStepEnabled: "additions design D8 — the FTdx5000 memory frame carries no tuning-step-enabled field (matrix §2)",
	spec.FieldTuningStep:        "additions design D8 — the FTdx5000 memory frame carries no tuning-step field (matrix §2)",
	spec.FieldProgramTuningStep: "additions design D8 — the FTdx5000 memory frame carries no programmable-tuning-step field (matrix §2)",
	spec.FieldAttenuator:        "additions design D8 — the FTdx5000 memory frame carries no attenuator field (matrix §2)",
	spec.FieldPreamp:            "additions design D8 — the FTdx5000 memory frame carries no preamp field (matrix §2)",
	spec.FieldAntenna:           "additions design D8 — the FTdx5000 memory frame carries no antenna-selection field (matrix §2)",
	spec.FieldIPPlus:            "additions design D8 — the FTdx5000 memory frame carries no IP+ field (matrix §2)",
}

func TestFieldAuditCoversEverySpecField(t *testing.T) {
	drivertest.AssertFieldAuditCoversEverySpecField(t, "allFields", allFields, deliberatelyUnexpressedFields)
}

// TestWriteTrialsComplete_PinnedFalse: the constant is false, AND a
// RealHardware driver's baseline is genuinely nothing-writable.
func TestWriteTrialsComplete_PinnedFalse(t *testing.T) {
	if writeTrialsComplete {
		t.Fatal("writeTrialsComplete = true: no FTdx5000 write trial has ever been run by this project")
	}
	caps := New(RealHardware).Capabilities()
	for _, b := range caps.Banks {
		for _, f := range allFields {
			if caps.FieldSupport(b.ID, f).CanWrite() {
				t.Errorf("RealHardware baseline: bank %s field %s is writable, want nothing writable while writeTrialsComplete is false", b.ID, f)
			}
		}
	}
}

// tierFieldsMustBeEmpty names the spec.Capabilities fields whose explicit
// decision for this radio is EMPTY/zero — the Icom-tier vocabularies this
// radio expresses none of, the Yaesu-family fields with no per-radio
// answer (SimplexTx, CTCSSToneRange, TagCharset), and — unlike every
// registered sibling — TagLen itself: this radio is NoTag, so TagLen must
// stay exactly 0 (core/spec/validate.go's NoTag pairing rule).
// RequiredSlots is also here: the matrix's own §4 leaves it deliberately
// unresolved (doc.go's ASSUMED register entry 4). ToneModes is NOT here:
// it carries this radio's own three-value CTCSS state
// (StandardToneModes()) now that the Yaesu and Icom/Kenwood tone
// vocabularies unified onto one enum.
var tierFieldsMustBeEmpty = map[string]bool{
	"DuplexOptions":          true,
	"DTCSPolarities":         true,
	"DTCSCodes":              true,
	"Filters":                true,
	"TuningSteps":            true,
	"ProgramTuningStepRange": true,
	"AttenuatorDB":           true,
	"PreampOptions":          true,
	"AntennaOptions":         true,
	"SimplexTx":              true,
	"TagCharset":             true,
	"CTCSSToneRange":         true,
	"TagLen":                 true,
	"RequiredSlots":          true,
}

// TestCapabilities_EveryFieldExplicit is the D-caps-explicit decision's
// enforcement: every field of spec.Capabilities is populated, in every
// profile, with nothing left at its zero value UNLESS this radio's own
// explicit decision is that it stays empty (tierFieldsMustBeEmpty).
func TestCapabilities_EveryFieldExplicit(t *testing.T) {
	const wantFieldCount = 29

	for _, tt := range []struct {
		name string
		caps spec.Capabilities
	}{
		{"Unverified", CapabilitiesUnverified()},
		{"Simulated", CapabilitiesSimulated()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			v := reflect.ValueOf(tt.caps)
			typ := v.Type()
			if typ.NumField() != wantFieldCount {
				t.Fatalf("spec.Capabilities has %d fields, this test knows %d — a field was added or removed and this driver must decide about it explicitly", typ.NumField(), wantFieldCount)
			}
			for i := 0; i < typ.NumField(); i++ {
				name := typ.Field(i).Name
				f := v.Field(i)
				if tierFieldsMustBeEmpty[name] {
					if !f.IsZero() || (f.Kind() == reflect.Slice && f.Len() != 0) {
						t.Errorf("field %s is populated — this radio's explicit decision is empty", name)
					}
					continue
				}
				if f.IsZero() {
					t.Errorf("field %s is the zero value — every spec.Capabilities field must be populated explicitly", name)
					continue
				}
				if f.Kind() == reflect.Slice && f.Len() == 0 {
					t.Errorf("field %s is an empty (but non-nil) slice — populated means populated", name)
				}
			}
		})
	}
}

// TestCapabilities_NoTag pins the NoTag pairing directly: TagLen 0, NoTag
// true, and no bank grades FieldTag/FieldTagDisplay above the zero
// FieldSupport.
func TestCapabilities_NoTag(t *testing.T) {
	for _, caps := range []spec.Capabilities{CapabilitiesUnverified(), CapabilitiesSimulated()} {
		if caps.TagLen != 0 || !caps.NoTag {
			t.Fatalf("TagLen %d, NoTag %v, want 0, true", caps.TagLen, caps.NoTag)
		}
		for _, b := range caps.Banks {
			for _, f := range []spec.Field{spec.FieldTag, spec.FieldTagDisplay} {
				if fs := b.Fields[f]; fs.Read != spec.Unsupported || fs.Write != spec.Unsupported {
					t.Errorf("bank %s field %s = %+v, want the zero FieldSupport under NoTag", b.ID, f, fs)
				}
			}
		}
	}
	if err := CapabilitiesUnverified().Validate(); err != nil {
		t.Errorf("CapabilitiesUnverified().Validate() = %v, want nil", err)
	}
	if err := CapabilitiesSimulated().Validate(); err != nil {
		t.Errorf("CapabilitiesSimulated().Validate() = %v, want nil", err)
	}
}

// TestCapabilities_Banks pins the two static banks' slot ranges (matrix
// §1.5/§4): MEM 001-099, PMS 100-117 (numeric form, not the token
// "P1L"-"P9U" every other registered sibling uses).
func TestCapabilities_Banks(t *testing.T) {
	caps := CapabilitiesUnverified()
	mem, ok := caps.Bank(spec.BankMemory)
	if !ok || len(mem.Slots) != 99 || mem.Slots[0] != "001" || mem.Slots[98] != "099" {
		t.Fatalf("MEM bank = %+v, want 99 slots 001..099", mem)
	}
	pms, ok := caps.Bank(spec.BankPMS)
	if !ok || len(pms.Slots) != 18 || pms.Slots[0] != "100" || pms.Slots[17] != "117" {
		t.Fatalf("PMS bank = %+v, want 18 slots 100..117", pms)
	}
	if len(caps.Banks) != 2 {
		t.Errorf("caps.Banks has %d entries, want 2 (no 5xx/EMG bank exists on this radio)", len(caps.Banks))
	}
}
