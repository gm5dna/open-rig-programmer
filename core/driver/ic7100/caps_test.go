// SPDX-License-Identifier: GPL-3.0-or-later

package ic7100

import (
	"reflect"
	"testing"

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

var mappedFields = map[spec.Field]bool{
	spec.FieldFrequency: true, spec.FieldMode: true, spec.FieldTag: true,
	spec.FieldTxFrequency: true, spec.FieldDuplex: true, spec.FieldOffset: true,
	spec.FieldToneMode: true, spec.FieldToneTx: true, spec.FieldToneRx: true,
	spec.FieldDTCSCode: true, spec.FieldDTCSPolarity: true,
	spec.FieldFilter: true, spec.FieldDataMode: true,
}

var deliberatelyZeroCapabilityFields = map[string]string{
	"ClarMaxHz":              "matrix §1 row 6",
	"ClarStepHz":             "matrix §1 row 7",
	"CTCSSTones":             "matrix §1 row 8",
	"RequiredSlots":          "matrix §1 row 14",
	"ShiftOptions":           "matrix §1 row 15",
	"CTCSSStates":            "matrix §1 row 16",
	"TuningSteps":            "matrix §1b D8",
	"ProgramTuningStepRange": "matrix §1b D8",
	"AttenuatorDB":           "matrix §1b D8",
	"PreampOptions":          "matrix §1b D8",
	"AntennaOptions":         "matrix §1b D8",
}

func TestCapabilitiesEveryStructFieldIsExplicitlyNonZeroOrAudited(t *testing.T) {
	value := reflect.ValueOf(CapabilitiesUnverified())
	typeOf := value.Type()
	if typeOf.NumField() != 28 {
		t.Fatalf("spec.Capabilities has %d fields; this deliberately-zero audit knows 28", typeOf.NumField())
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
		t.Fatal("writeTrialsComplete is true; no IC-7100 write trial has been completed")
	}
}

func TestCapabilitiesProfilesAndFieldGrid(t *testing.T) {
	real := CapabilitiesUnverified()
	sim := CapabilitiesSimulated()
	for name, caps := range map[string]spec.Capabilities{"real": real, "simulated": sim} {
		if err := caps.Validate(); err != nil {
			t.Fatalf("%s Validate: %v", name, err)
		}
		if caps.Model != "IC-7100" || caps.CATID != "88" {
			t.Errorf("%s identity = %q/%q, want IC-7100/88", name, caps.Model, caps.CATID)
		}
		if caps.Transmit != spec.HasTransmitter {
			t.Errorf("%s Transmit = %v, want HasTransmitter (matrix §1 row 23)", name, caps.Transmit)
		}
		if len(caps.Banks) != 1 {
			t.Fatalf("%s has %d banks, want one dense MEM bank (matrix §1b)", name, len(caps.Banks))
		}
		bank := caps.Banks[0]
		if bank.ID != spec.BankMemory || bank.Sparse || bank.Groups != 0 || bank.PerGroup != 0 || bank.Budget != 0 || bank.BudgetUnstated {
			t.Errorf("%s MEM bank metadata = %+v, want dense and deliberately zero (matrix §1b)", name, bank)
		}
		if len(bank.Slots) != 495 || bank.Slots[0] != "A-001" || bank.Slots[len(bank.Slots)-1] != "E-099" {
			t.Errorf("%s MEM slots = %d, first/last %q/%q, want 495 A-001..E-099", name, len(bank.Slots), bank.Slots[0], bank.Slots[len(bank.Slots)-1])
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
	if !reflect.DeepEqual(caps.Modes, []string{"LSB", "USB", "AM", "CW", "RTTY", "FM", "WFM", "CW-R", "RTTY-R", "DV"}) {
		t.Errorf("Modes = %v (matrix §1 row 4)", caps.Modes)
	}
	if caps.TagLen != 16 || len(caps.TagCharset) != 95 || !caps.TagByteOK(';') || !caps.TagByteOK(' ') {
		t.Errorf("tag policy = len %d charset %d semicolon=%v space=%v (matrix §1 rows 5/22)", caps.TagLen, len(caps.TagCharset), caps.TagByteOK(';'), caps.TagByteOK(' '))
	}
	if caps.ClarMaxHz != 0 || caps.ClarStepHz != 0 || len(caps.CTCSSTones) != 0 || len(caps.ShiftOptions) != 0 || len(caps.CTCSSStates) != 0 {
		t.Error("deliberately-zero legacy capability fields drifted from matrix §1 rows 6–8/15–16")
	}
	if caps.CTCSSToneRange == nil || *caps.CTCSSToneRange != (spec.ToneRange{MinDeciHz: 670, MaxDeciHz: 2541, StepDeciHz: 1871}) {
		t.Errorf("CTCSSToneRange = %+v, want endpoint-only fail-closed policy pinned by the profile (matrix §1 row 9; ic7100-tone-range-step)", caps.CTCSSToneRange)
	}
	if !reflect.DeepEqual(caps.Bauds, []int{300, 1200, 4800, 9600, 19200}) || caps.DefaultBaud != 19200 {
		t.Errorf("baud policy = %v/default %d (matrix §1 rows 10–11; ic7100-default-baud-auto)", caps.Bauds, caps.DefaultBaud)
	}
	if caps.MinFreqHz != 30_000 || caps.MaxFreqHz != 470_000_000 {
		t.Errorf("frequency bounds = %d..%d (matrix §1 rows 12–13)", caps.MinFreqHz, caps.MaxFreqHz)
	}
	if len(caps.RequiredSlots) != 0 || len(caps.TuningSteps) != 0 || caps.ProgramTuningStepRange != nil || len(caps.AttenuatorDB) != 0 || len(caps.PreampOptions) != 0 || len(caps.AntennaOptions) != 0 {
		t.Error("deliberately-zero inventory/receiver fields drifted from matrix §1 row 14 and §1b")
	}
	if len(caps.DuplexOptions) != 3 || len(caps.ToneModes) != 4 || !reflect.DeepEqual(caps.DTCSPolarities, []string{"NN", "NR", "RN", "RR"}) || len(caps.DTCSCodes) != 104 || !reflect.DeepEqual(caps.Filters, []string{"FIL1", "FIL2", "FIL3"}) {
		t.Errorf("Icom vocabularies drifted: duplex=%d tone=%d polarity=%v DTCS=%d filters=%v (matrix §1 rows 17–21)", len(caps.DuplexOptions), len(caps.ToneModes), caps.DTCSPolarities, len(caps.DTCSCodes), caps.Filters)
	}
}
