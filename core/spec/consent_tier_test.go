// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"reflect"
	"testing"
	"unsafe"
)

// tierAddedFields is every Field the Icom tier added to the neutral
// memory model (design D4). Kept here rather than derived, so that
// adding a Field to field.go without deciding what consent means for it
// is a deliberate act: the test below walks this list.
var tierAddedFields = []Field{
	FieldTxFrequency, FieldDuplex, FieldOffset, FieldToneMode,
	FieldToneTx, FieldToneRx, FieldDTCSCode, FieldDTCSPolarity,
	FieldFilter, FieldDataMode,
	FieldTuningStepEnabled, FieldTuningStep, FieldProgramTuningStep,
	FieldAttenuator, FieldPreamp, FieldAntenna, FieldIPPlus,
}

// TestConsentUnverifiedWrites_CoversTierAddedFields is the deliverable
// design D4 (round 2, F9) asked for by name: the consent transform walks
// every field map, so the tier's new Fields are covered BY CONSTRUCTION
// — and that is verified rather than assumed.
//
// Both halves are asserted, because "covered" means two things at once:
// a write-side Unverified label on a new Field becomes
// ConsentedUnverified exactly as it does for the ten Fields that
// predate the tier, and nothing else about the new Field moves (its
// read label in particular).
func TestConsentUnverifiedWrites_CoversTierAddedFields(t *testing.T) {
	fields := make(map[Field]FieldSupport, len(tierAddedFields))
	for _, f := range tierAddedFields {
		fields[f] = FieldSupport{Read: Unverified, Write: Unverified}
	}
	in := Capabilities{
		Model: "TEST", CATID: "0000", TagLen: 8,
		Banks: []Bank{{ID: BankMemory, Slots: []string{"001"}, Fields: fields}},
	}

	out := ConsentUnverifiedWrites(in)

	bank, ok := out.Bank(BankMemory)
	if !ok {
		t.Fatal("Bank(BankMemory) missing from the transformed capabilities")
	}
	for _, f := range tierAddedFields {
		fs := bank.Fields[f]
		if fs.Write != ConsentedUnverified {
			t.Errorf("field %s: Write = %v, want ConsentedUnverified", f, fs.Write)
		}
		if fs.Read != Unverified {
			t.Errorf("field %s: Read = %v, want Unverified (consent is write-side only)", f, fs.Read)
		}
		if !fs.CanWrite() {
			t.Errorf("field %s: CanWrite() = false after consent, want true", f)
		}
	}

	// The input must be untouched: caps is typically a driver's
	// long-lived baseline.
	for _, f := range tierAddedFields {
		if got := in.Banks[0].Fields[f].Write; got != Unverified {
			t.Errorf("input field %s: Write = %v, want the input left at Unverified", f, got)
		}
	}
}

// TestConsentUnverifiedWrites_CopiesTierVocabularies pins the other half
// of the transform's promise for the tier's additions: the returned
// value shares NO storage with its input, the new top-level vocabulary
// slices included.
func TestConsentUnverifiedWrites_CopiesTierVocabularies(t *testing.T) {
	in := Capabilities{
		Model: "TEST", CATID: "0000", TagLen: 8,
		DuplexOptions:  []DuplexOption{{Value: "OFF", Direction: DuplexOff}},
		ToneModes:      []ToneMode{{Value: "OFF", Semantics: ToneModeOff}},
		DTCSPolarities: []string{"NN"},
		DTCSCodes:      []int{23},
		Filters:        []string{"FIL1"},
	}
	out := ConsentUnverifiedWrites(in)

	if !reflect.DeepEqual(in.DuplexOptions, out.DuplexOptions) ||
		!reflect.DeepEqual(in.ToneModes, out.ToneModes) ||
		!reflect.DeepEqual(in.DTCSPolarities, out.DTCSPolarities) ||
		!reflect.DeepEqual(in.DTCSCodes, out.DTCSCodes) ||
		!reflect.DeepEqual(in.Filters, out.Filters) {
		t.Fatal("the tier vocabularies came back with different CONTENT; consent must not alter them")
	}

	if sameBacking(unsafe.SliceData(in.DuplexOptions), unsafe.SliceData(out.DuplexOptions)) {
		t.Error("DuplexOptions shares backing storage with the input")
	}
	if sameBacking(unsafe.SliceData(in.ToneModes), unsafe.SliceData(out.ToneModes)) {
		t.Error("ToneModes shares backing storage with the input")
	}
	if sameBacking(unsafe.SliceData(in.DTCSPolarities), unsafe.SliceData(out.DTCSPolarities)) {
		t.Error("DTCSPolarities shares backing storage with the input")
	}
	if sameBacking(unsafe.SliceData(in.DTCSCodes), unsafe.SliceData(out.DTCSCodes)) {
		t.Error("DTCSCodes shares backing storage with the input")
	}
	if sameBacking(unsafe.SliceData(in.Filters), unsafe.SliceData(out.Filters)) {
		t.Error("Filters shares backing storage with the input")
	}
}

func TestConsentUnverifiedWrites_CopiesReceiverVocabularies(t *testing.T) {
	in := Capabilities{
		TuningSteps:            []string{"1 kHz"},
		ProgramTuningStepRange: &StepRange{MinHz: 100, MaxHz: 100000, ResolutionHz: 100},
		AttenuatorDB:           []int{0, 10},
		PreampOptions:          []string{"OFF", "ON"},
		AntennaOptions:         []string{"ANT1", "ANT2"},
	}
	out := ConsentUnverifiedWrites(in)

	if !reflect.DeepEqual(in.TuningSteps, out.TuningSteps) ||
		!reflect.DeepEqual(in.ProgramTuningStepRange, out.ProgramTuningStepRange) ||
		!reflect.DeepEqual(in.AttenuatorDB, out.AttenuatorDB) ||
		!reflect.DeepEqual(in.PreampOptions, out.PreampOptions) ||
		!reflect.DeepEqual(in.AntennaOptions, out.AntennaOptions) {
		t.Fatal("receiver vocabularies changed content during consent copy")
	}
	if sameBacking(unsafe.SliceData(in.TuningSteps), unsafe.SliceData(out.TuningSteps)) ||
		sameBacking(unsafe.SliceData(in.AttenuatorDB), unsafe.SliceData(out.AttenuatorDB)) ||
		sameBacking(unsafe.SliceData(in.PreampOptions), unsafe.SliceData(out.PreampOptions)) ||
		sameBacking(unsafe.SliceData(in.AntennaOptions), unsafe.SliceData(out.AntennaOptions)) {
		t.Error("a receiver vocabulary slice shares backing storage with the input")
	}
	if in.ProgramTuningStepRange == out.ProgramTuningStepRange {
		t.Error("ProgramTuningStepRange pointer shares storage with the input")
	}
}

// sameBacking reports whether two slice data pointers are the same
// allocation. Generic so each call site reads as the type it is about.
func sameBacking[T any](a, b *T) bool { return a == b }
