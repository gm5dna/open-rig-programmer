// SPDX-License-Identifier: GPL-3.0-or-later

package icr8600

import (
	"reflect"
	"slices"
	"testing"

	civicr8600 "github.com/gm5dna/open-rig-programmer/core/civ/icr8600"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

func TestCapabilitiesUnverified_MatrixValues(t *testing.T) {
	caps := CapabilitiesUnverified()
	if err := caps.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if caps.Model != civicr8600.Model || caps.CATID != "96" {
		t.Errorf("identity = %q/%q, want %q/96 (matrix section 1 rows 1-2)", caps.Model, caps.CATID, civicr8600.Model)
	}
	if caps.Transmit != spec.ReceiveOnly {
		t.Errorf("Transmit = %v, want ReceiveOnly (matrix section 1 row 23)", caps.Transmit)
	}
	if !slices.Equal(caps.Bauds, []int{4800, 9600, 19200, 38400, 57600, 115200}) {
		t.Errorf("Bauds = %v (matrix section 1 row 10)", caps.Bauds)
	}
	if caps.DefaultBaud != 19200 {
		t.Errorf("DefaultBaud = %d, want assumed 19200 (register icr8600-default-baud)", caps.DefaultBaud)
	}
	if caps.TagLen != 16 || caps.TagCharset != civicr8600.NameCharset {
		t.Errorf("tag policy = len %d charset %q (matrix section 1 rows 5 and 22)", caps.TagLen, caps.TagCharset)
	}
	if !caps.TagByteOK(';') || !caps.TagByteOK('|') {
		t.Error("TagCharset omits ';' or '|' (matrix section 1 row 22)")
	}

	bank, ok := caps.Bank(spec.BankMemory)
	if !ok || len(caps.Banks) != 1 {
		t.Fatalf("Banks = %#v, want only MEM (matrix section 1b.3)", caps.Banks)
	}
	if !bank.Sparse || bank.Groups != 100 || bank.GroupBase != 0 || bank.PerGroup != 100 || bank.ChannelBase != 0 || bank.Budget != 0 || !bank.BudgetUnstated {
		t.Errorf("MEM geometry = %#v, want zero-based 100x100 with BudgetUnstated (matrix section 1b.3)", bank)
	}

	wantModes := []string{"LSB", "USB", "AM", "CW", "FSK", "FM", "WFM", "CW-R", "FSK-R", "S-AM (D)", "S-AM (L)", "S-AM (U)", "P25", "D-STAR", "dPMR", "NXDN-VN", "NXDN-N", "DCR"}
	if !slices.Equal(caps.Modes, wantModes) {
		t.Errorf("Modes = %v (matrix section 1 row 4)", caps.Modes)
	}
	if !slices.Equal(caps.TuningSteps, []string{"100 Hz", "1 kHz", "2.5 kHz", "3.125 kHz", "5 kHz", "6.25 kHz", "8.33 kHz", "9 kHz", "10 kHz", "12.5 kHz", "20 kHz", "25 kHz", "100 kHz", "programmable tuning step"}) {
		t.Errorf("TuningSteps = %v (matrix section 1b.2)", caps.TuningSteps)
	}
	if caps.ProgramTuningStepRange == nil || *caps.ProgramTuningStepRange != (spec.StepRange{MinHz: 100, MaxHz: 999900, ResolutionHz: 100}) {
		t.Errorf("ProgramTuningStepRange = %#v (matrix section 1b.2; floor register icr8600-progstep-floor)", caps.ProgramTuningStepRange)
	}
	if !slices.Equal(caps.AttenuatorDB, []int{0, 10, 20, 30}) || !slices.Equal(caps.PreampOptions, []string{"OFF", "ON"}) || !slices.Equal(caps.AntennaOptions, []string{"ANT1", "ANT2", "ANT3"}) {
		t.Errorf("D8 vocabularies = attenuator %v preamp %v antenna %v (matrix section 1b.2)", caps.AttenuatorDB, caps.PreampOptions, caps.AntennaOptions)
	}
}

func TestCapabilities_FieldGridAndReceiveOnlyToneSemantics(t *testing.T) {
	caps := CapabilitiesUnverified()
	wantRW := spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}
	reachable := map[spec.Field]bool{
		spec.FieldFrequency: true, spec.FieldMode: true, spec.FieldTag: true,
		spec.FieldDuplex: true, spec.FieldOffset: true,
		spec.FieldToneMode: true, spec.FieldToneRx: true, spec.FieldDTCSCode: true,
		spec.FieldDTCSPolarity: true, spec.FieldFilter: true,
		spec.FieldTuningStepEnabled: true, spec.FieldTuningStep: true,
		spec.FieldProgramTuningStep: true, spec.FieldAttenuator: true,
		spec.FieldPreamp: true, spec.FieldAntenna: true, spec.FieldIPPlus: true,
	}
	bank, _ := caps.Bank(spec.BankMemory)
	if len(bank.Fields) != 27 {
		t.Fatalf("MEM field cells = %d, want all 27 matrix section 2 rows", len(bank.Fields))
	}
	for _, field := range allFieldsForAudit() {
		want := spec.FieldSupport{}
		if reachable[field] {
			want = wantRW
		}
		if got := bank.Fields[field]; got != want {
			t.Errorf("field %s = %v, want %v (matrix section 2)", field, got, want)
		}
	}
	for _, field := range []spec.Field{spec.FieldTxFrequency, spec.FieldToneTx} {
		if got := bank.Fields[field]; got != (spec.FieldSupport{}) {
			t.Errorf("receive-only field %s = %v, want zero", field, got)
		}
	}
	wantToneModes := []spec.ToneMode{
		{Value: "OFF", Semantics: spec.ToneModeOff},
		{Value: "TSQL", Semantics: spec.ToneModeCTCSSRxSquelch},
		{Value: "DTCS", Semantics: spec.ToneModeDTCS},
	}
	if !reflect.DeepEqual(caps.ToneModes, wantToneModes) {
		t.Errorf("ToneModes = %#v, want receive-only FM vocabulary (matrix section 1 row 18; 4b Erratum 1)", caps.ToneModes)
	}
	for _, mode := range caps.ToneModes {
		if mode.NeedsTxTone() {
			t.Errorf("tone mode %q needs a TX tone on a ReceiveOnly model", mode.Value)
		}
	}
}

func TestConsentAndWriteTrialsRemainFailSafe(t *testing.T) {
	if writeTrialsComplete {
		t.Fatal("writeTrialsComplete = true; matrix section 3.14 pins false until an IC-R8600 write trial exists")
	}
	base := New(RealHardware).Capabilities()
	if base.FieldSupport(spec.BankMemory, spec.FieldFrequency).CanWrite() {
		t.Fatal("unconsented real-hardware capabilities are writable")
	}
	consented := New(RealHardware, WithConsentedUnverifiedWrites()).(*icr8600Driver).sessionCapabilities(nil, "96:01")
	if !consented.FieldSupport(spec.BankMemory, spec.FieldFrequency).CanWrite() {
		t.Fatal("consent did not transform an Unverified write")
	}
	if consented.FieldSupport(spec.BankMemory, spec.FieldErase).CanWrite() {
		t.Fatal("consent transformed erase")
	}
	consented.Modes[0] = "MUTATED"
	consented.TuningSteps[0] = "MUTATED"
	consented.AttenuatorDB[0] = 99
	consented.PreampOptions[0] = "MUTATED"
	consented.AntennaOptions[0] = "MUTATED"
	consented.ProgramTuningStepRange.MinHz = 999
	fresh := CapabilitiesUnverified()
	if fresh.Modes[0] == "MUTATED" || fresh.TuningSteps[0] == "MUTATED" || fresh.AttenuatorDB[0] == 99 || fresh.PreampOptions[0] == "MUTATED" || fresh.AntennaOptions[0] == "MUTATED" || fresh.ProgramTuningStepRange.MinHz == 999 {
		t.Fatal("consented capabilities alias the static capability slices or range")
	}
}

func TestDeliberatelyZeroAudit(t *testing.T) {
	typ := reflect.TypeOf(spec.Capabilities{})
	if typ.NumField() != 28 {
		t.Fatalf("spec.Capabilities field count = %d, want Wave 1b count 28; audit every new field before updating this pin", typ.NumField())
	}
	zero := map[string]bool{
		"ClarMaxHz": true, "ClarStepHz": true, "CTCSSTones": true,
		"MinFreqHz": true, "MaxFreqHz": true, "RequiredSlots": true,
		"ShiftOptions": true, "CTCSSStates": true,
	}
	caps := CapabilitiesUnverified()
	v := reflect.ValueOf(caps)
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		isZero := v.Field(i).IsZero()
		if isZero != zero[name] {
			t.Errorf("Capabilities.%s zero = %v, want %v; cite the matrix row in caps.go", name, isZero, zero[name])
		}
	}
}

func allFieldsForAudit() []spec.Field {
	return []spec.Field{
		spec.FieldFrequency, spec.FieldMode, spec.FieldClarifier, spec.FieldCTCSSState,
		spec.FieldCTCSSTone, spec.FieldShift, spec.FieldTag, spec.FieldTagDisplay,
		spec.FieldScanSkip, spec.FieldErase, spec.FieldTxFrequency, spec.FieldDuplex,
		spec.FieldOffset, spec.FieldToneMode, spec.FieldToneTx, spec.FieldToneRx,
		spec.FieldDTCSCode, spec.FieldDTCSPolarity, spec.FieldFilter, spec.FieldDataMode,
		spec.FieldTuningStepEnabled, spec.FieldTuningStep, spec.FieldProgramTuningStep,
		spec.FieldAttenuator, spec.FieldPreamp, spec.FieldAntenna, spec.FieldIPPlus,
	}
}
