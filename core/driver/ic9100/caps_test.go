// SPDX-License-Identifier: GPL-3.0-or-later

package ic9100

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

// deliberatelyUnexpressedFields carries only the three TS-2000-only
// Satellite Memory bank flags (v1.10.0): every OTHER spec.Field is graded
// in fieldGrid (RW for the twelve the record maps, the zero FieldSupport
// for the rest), so every one of those is AUDITED rather than a separate
// "unexpressed" entry — matching core/driver/ic7100's identical
// convention.
var deliberatelyUnexpressedFields = map[spec.Field]string{
	spec.FieldSatBandSwap: "ts2000-only: TS-2000/2000X/B2000 Satellite Memory bank flag (SA record); no home on this radio",
	spec.FieldSatTrace:    "ts2000-only: TS-2000/2000X/B2000 Satellite Memory bank flag (SA record); no home on this radio",
	spec.FieldSatTraceRev: "ts2000-only: TS-2000/2000X/B2000 Satellite Memory bank flag (SA record); no home on this radio",
}

func TestFieldAuditCoversEverySpecField(t *testing.T) {
	drivertest.AssertFieldAuditCoversEverySpecField(t, "allCapabilityFields", allCapabilityFields, deliberatelyUnexpressedFields)
}

// mappedFields: the twelve fields the 57-byte record maps (matrix §1b/§2).
// No FieldTxFrequency: this record has no TX-side frequency block.
var mappedFields = map[spec.Field]bool{
	spec.FieldFrequency: true, spec.FieldMode: true, spec.FieldTag: true,
	spec.FieldDuplex: true, spec.FieldOffset: true,
	spec.FieldToneMode: true, spec.FieldToneTx: true, spec.FieldToneRx: true,
	spec.FieldDTCSCode: true, spec.FieldDTCSPolarity: true,
	spec.FieldFilter: true, spec.FieldDataMode: true,
}

var deliberatelyZeroCapabilityFields = map[string]string{
	"ClarMaxHz":              "matrix §1 row 9",
	"ClarStepHz":             "matrix §1 row 10",
	"CTCSSTones":             "matrix §1 row 11",
	"RequiredSlots":          "matrix §1 row 17",
	"ShiftOptions":           "matrix §1 row 18",
	"TuningSteps":            "matrix §1 row 25 / §1b D8",
	"ProgramTuningStepRange": "matrix §1 row 26 / §1b D8",
	"AttenuatorDB":           "matrix §1 row 27 / §1b D8",
	"PreampOptions":          "matrix §1 row 28 / §1b D8",
	"AntennaOptions":         "matrix §1 row 29 / §1b D8",
	"SimplexTx":              "matrix §1 row 4: this model grades FieldDuplex but not FieldTxFrequency, so the question SimplexTx answers does not arise",
	"NoTag":                  "the IC-9100 supports channel names via the memory name field (matrix §1 row 8); NoTag is false",
}

func TestCapabilitiesEveryStructFieldIsExplicitlyNonZeroOrAudited(t *testing.T) {
	value := reflect.ValueOf(CapabilitiesUnverified())
	typeOf := value.Type()
	if typeOf.NumField() != 29 {
		t.Fatalf("spec.Capabilities has %d fields; this deliberately-zero audit knows 29", typeOf.NumField())
	}
	for i := 0; i < typeOf.NumField(); i++ {
		name := typeOf.Field(i).Name
		_, audited := deliberatelyZeroCapabilityFields[name]
		if value.Field(i).IsZero() != audited {
			t.Errorf("Capabilities.%s zero=%v, deliberately-zero audit=%v", name, value.Field(i).IsZero(), audited)
		}
	}
}

func TestWriteTrialsCompletePinnedFalse(t *testing.T) {
	if writeTrialsComplete {
		t.Fatal("writeTrialsComplete is true; no IC-9100 write trial has been completed")
	}
}

func TestCapabilitiesProfilesAndFieldGrid(t *testing.T) {
	real := CapabilitiesUnverified()
	sim := CapabilitiesSimulated()
	for name, caps := range map[string]spec.Capabilities{"real": real, "simulated": sim} {
		if err := caps.Validate(); err != nil {
			t.Fatalf("%s Validate: %v", name, err)
		}
		if caps.Model != "IC-9100" || caps.CATID != "7C" {
			t.Errorf("%s identity = %q/%q, want IC-9100/7C (matrix §3.4)", name, caps.Model, caps.CATID)
		}
		if caps.Transmit != spec.HasTransmitter {
			t.Errorf("%s Transmit = %v, want HasTransmitter (matrix §1 row 3)", name, caps.Transmit)
		}
		if len(caps.Banks) != 1 {
			t.Fatalf("%s has %d banks, want one dense MEM bank (matrix §1b)", name, len(caps.Banks))
		}
		bank := caps.Banks[0]
		if bank.ID != spec.BankMemory || bank.Sparse || bank.Groups != 0 || bank.PerGroup != 0 || bank.Budget != 0 || bank.BudgetUnstated {
			t.Errorf("%s MEM bank metadata = %+v, want dense and deliberately zero (matrix §1b)", name, bank)
		}
		if len(bank.Slots) != 297 || bank.Slots[0] != "HF-001" || bank.Slots[len(bank.Slots)-1] != "430-099" {
			t.Errorf("%s MEM slots = %d, first/last %q/%q, want 297 HF-001..430-099", name, len(bank.Slots), bank.Slots[0], bank.Slots[len(bank.Slots)-1])
		}
		if len(bank.Fields) != len(allCapabilityFields) {
			t.Errorf("%s field audit has %d entries, want every %d field written down", name, len(bank.Fields), len(allCapabilityFields))
		}
		for _, field := range allCapabilityFields {
			fs, present := bank.Fields[field]
			if !present {
				t.Errorf("%s/%s is omitted from the deliberately-zero audit", name, field)
				continue
			}
			if mappedFields[field] {
				wantWrite := spec.Unverified
				if name == "simulated" {
					wantWrite = spec.Supported
				}
				if fs.Read != spec.Unverified || fs.Write != wantWrite {
					t.Errorf("%s/%s = %+v, want Read Unverified/Write %v (matrix §2)", name, field, fs, wantWrite)
				}
			} else if !fs.Unreachable() {
				t.Errorf("%s/%s = %+v, want deliberate zero (matrix §2)", name, field, fs)
			}
		}
		if fs := caps.FieldSupport(spec.BankMemory, spec.FieldErase); !fs.Unreachable() {
			t.Errorf("%s erase = %+v, consent and simulation must never enable erase", name, fs)
		}
	}
}

func TestCapabilityValuesFromMatrix(t *testing.T) {
	caps := CapabilitiesUnverified()
	if !reflect.DeepEqual(caps.Modes, []string{"LSB", "USB", "AM", "CW", "RTTY", "FM", "CW-R", "RTTY-R", "DV"}) {
		t.Errorf("Modes = %v (matrix §1 row 6)", caps.Modes)
	}
	if caps.TagLen != 9 || len(caps.TagCharset) != 95 || !caps.TagByteOK(';') || !caps.TagByteOK(' ') {
		t.Errorf("tag policy = len %d charset %d semicolon=%v space=%v (matrix §1 rows 7/30)", caps.TagLen, len(caps.TagCharset), caps.TagByteOK(';'), caps.TagByteOK(' '))
	}
	if caps.ClarMaxHz != 0 || caps.ClarStepHz != 0 || len(caps.CTCSSTones) != 0 || len(caps.ShiftOptions) != 0 {
		t.Error("deliberately-zero legacy capability fields drifted from matrix §1 rows 9-11/18")
	}
	if caps.CTCSSToneRange == nil || *caps.CTCSSToneRange != (spec.ToneRange{MinDeciHz: 670, MaxDeciHz: 2541, StepDeciHz: 1}) {
		t.Errorf("CTCSSToneRange = %+v, want the printed 50-tone chart's own bounds at 0.1 Hz resolution (matrix §1 row 12)", caps.CTCSSToneRange)
	}
	if !reflect.DeepEqual(caps.Bauds, []int{300, 1200, 4800, 9600, 19200}) || caps.DefaultBaud != 19200 {
		t.Errorf("baud policy = %v/default %d (matrix §1 rows 13-14; ic9100-default-baud-auto)", caps.Bauds, caps.DefaultBaud)
	}
	if caps.MinFreqHz != 30_000 || caps.MaxFreqHz != 480_000_000 {
		t.Errorf("frequency bounds = %d..%d (matrix §1 rows 15-16)", caps.MinFreqHz, caps.MaxFreqHz)
	}
	if len(caps.RequiredSlots) != 0 || len(caps.TuningSteps) != 0 || caps.ProgramTuningStepRange != nil || len(caps.AttenuatorDB) != 0 || len(caps.PreampOptions) != 0 || len(caps.AntennaOptions) != 0 {
		t.Error("deliberately-zero inventory/receiver fields drifted from matrix §1 row 17 and §1b")
	}
	if len(caps.DuplexOptions) != 3 || len(caps.ToneModes) != 4 || !reflect.DeepEqual(caps.DTCSPolarities, []string{"NN", "NR", "RN", "RR"}) || len(caps.DTCSCodes) != 104 || !reflect.DeepEqual(caps.Filters, []string{"FIL1", "FIL2", "FIL3"}) {
		t.Errorf("Icom vocabularies drifted: duplex=%d tone=%d polarity=%v DTCS=%d filters=%v (matrix §1 rows 20-24)", len(caps.DuplexOptions), len(caps.ToneModes), caps.DTCSPolarities, len(caps.DTCSCodes), caps.Filters)
	}
}

// TestCTCSSToneDomainAdmitsEveryChartTone pins the declared tone domain
// against the printed 50-tone chart (matrix §1 row 12): every chart tone
// must be admitted, and the boundary is a strict comparison — the chart's
// own bounds, 67.0 and 254.1 Hz, admitted; one step either side refused.
func TestCTCSSToneDomainAdmitsEveryChartTone(t *testing.T) {
	for _, caps := range []spec.Capabilities{CapabilitiesUnverified(), CapabilitiesSimulated()} {
		for i, tone := range spec.StandardCTCSSTones() {
			if !caps.AdmitsTone(tone) {
				t.Errorf("AdmitsTone(%v) = false, want true — chart tone %d of the 50 printed on PDF p.74", tone, i)
			}
		}
		for _, tone := range []spec.Tone{0, 669, 2542, 3000} {
			if caps.AdmitsTone(tone) {
				t.Errorf("AdmitsTone(%v) = true, want false — outside the declared 670..2541 domain (matrix §1 row 12)", tone)
			}
		}
	}
}
