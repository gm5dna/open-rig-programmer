// SPDX-License-Identifier: GPL-3.0-or-later

package ts590

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	kwts590 "github.com/gm5dna/open-rig-programmer/core/kw/ts590"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// allSpecFields is THIS package's own literal list of the twenty-seven
// spec.Field constants (plan P5). It is deliberately not spec.AllFields():
// a list that must be edited when a Field is added, and that fails loudly
// when one is, is what P5 asks every Kenwood package for.
//
// TestBankFields_NameEveryOneOfTheTwentySeven walks it, and
// TestFieldAudit_CoversEverySpecField faces it against the fleet helper —
// which calls spec.AllFields() internally precisely in order to compare it
// against a hand-written list, so the two are complementary rather than
// contradictory (P5's own paragraph).
var allSpecFields = []spec.Field{
	spec.FieldFrequency, spec.FieldMode, spec.FieldClarifier,
	spec.FieldCTCSSState, spec.FieldCTCSSTone, spec.FieldShift,
	spec.FieldTag, spec.FieldTagDisplay, spec.FieldScanSkip,
	spec.FieldErase, spec.FieldTxFrequency, spec.FieldDuplex,
	spec.FieldOffset, spec.FieldToneMode, spec.FieldToneTx,
	spec.FieldToneRx, spec.FieldDTCSCode, spec.FieldDTCSPolarity,
	spec.FieldFilter, spec.FieldDataMode, spec.FieldTuningStepEnabled,
	spec.FieldTuningStep, spec.FieldProgramTuningStep, spec.FieldAttenuator,
	spec.FieldPreamp, spec.FieldAntenna, spec.FieldIPPlus,
}

// bothRows is every registry row this package serves, so a per-row table
// cannot silently cover one of them twice.
var bothRows = []Row{RowS, RowSG}

func TestAllSpecFields_IsTwentySeven(t *testing.T) {
	if len(allSpecFields) != 27 {
		t.Fatalf("allSpecFields has %d entries, want 27 (core/spec/field.go)", len(allSpecFields))
	}
	seen := map[spec.Field]bool{}
	for _, f := range allSpecFields {
		if seen[f] {
			t.Errorf("allSpecFields lists %s twice", f)
		}
		seen[f] = true
	}
}

// TestAllSpecFields_MatchesTheAuditedAndUnexpressedFields is LOW-2's fix
// (Opus review, T12 fix round 1): the test above pins only a COUNT and no
// duplicates, both of which survive a fleet field addition landing in the
// wrong list — a new spec.Field added to unexpressedFields alone would leave
// requestedFieldRules (which is anchored to allSpecFields, not to
// spec.AllFields) silently ungated, the exact C-M1 class of loss the
// capability gate exists to close.
//
// ANCHORED WITHOUT NAMING spec.AllFields HERE (P5: no Kenwood file names it):
// TestFieldAudit_CoversEverySpecField already proves, via
// drivertest.AssertFieldAuditCoversEverySpecField, that auditedFields(row)
// and unexpressedFields(row) partition spec.AllFields() exactly, for both
// rows. Pinning allSpecFields against THAT union anchors it, transitively,
// to the real field list.
func TestAllSpecFields_MatchesTheAuditedAndUnexpressedFields(t *testing.T) {
	for _, row := range bothRows {
		want := map[spec.Field]bool{}
		for _, f := range auditedFields(row) {
			want[f] = true
		}
		for f := range unexpressedFields(row) {
			want[f] = true
		}
		for _, f := range allSpecFields {
			if !want[f] {
				t.Errorf("%s: allSpecFields names %s, which is neither audited nor unexpressed", modelNameFor(row), f)
			}
			delete(want, f)
		}
		for f := range want {
			t.Errorf("%s: audited/unexpressed names %s, which allSpecFields does not", modelNameFor(row), f)
		}
	}
}

// TestCapabilities_ValidateOnEveryRowAndProfile is the floor: a capability
// set that does not validate cannot be registered at all.
func TestCapabilities_ValidateOnEveryRowAndProfile(t *testing.T) {
	for _, row := range bothRows {
		for name, caps := range map[string]spec.Capabilities{
			"unverified": CapabilitiesUnverified(row),
			"simulated":  CapabilitiesSimulated(row),
		} {
			if err := caps.Validate(); err != nil {
				t.Errorf("%s %s: Validate: %v", modelNameFor(row), name, err)
			}
		}
	}
}

// TestCapabilities_MatrixValuesPerRow walks the A4 matrix's §1 row by row.
// EVERY VALUE HERE IS THE MATRIX'S, cited by section, and a value that is not
// derivable from it is a STOP rather than a judgement call.
func TestCapabilities_MatrixValuesPerRow(t *testing.T) {
	kenwoodTonesWant := []spec.Tone{
		670, 693, 719, 744, 770, 797, 825, 854, 885, 915, 948, 974,
		1000, 1035, 1072, 1109, 1148, 1188, 1230, 1273, 1318, 1365,
		1413, 1462, 1514, 1567, 1622, 1679, 1738, 1799, 1862, 1928,
		2035, 2065, 2107, 2181, 2257, 2291, 2336, 2418, 2503, 2541,
		17500,
	}
	for _, tc := range []struct {
		row     Row
		model   string
		catID   string
		filters []string
	}{
		// §1.1, §1.2: ID's own legend, "021: TS-590S" (590:1114) and
		// "023: TS-590SG" (590:1116). §1.22: the SG publishes the two
		// printed labels; the S publishes none, which is Q12's refusal.
		{RowS, "TS-590S", "021", nil},
		{RowSG, "TS-590SG", "023", []string{"FILTER A", "FILTER B"}},
	} {
		caps := CapabilitiesUnverified(tc.row)
		if caps.Model != tc.model {
			t.Errorf("%v: Model = %q, want %q (§1.1)", tc.row, caps.Model, tc.model)
		}
		if caps.CATID != tc.catID {
			t.Errorf("%s: CATID = %q, want %q (§1.2)", tc.model, caps.CATID, tc.catID)
		}
		if caps.Transmit != spec.HasTransmitter { // §1.3
			t.Errorf("%s: Transmit = %v, want HasTransmitter (§1.3)", tc.model, caps.Transmit)
		}
		// §1.5: nine names over eight nibbles, FM-N synthesised.
		wantModes := []string{"LSB", "USB", "CW", "FM", "FM-N", "AM", "FSK", "CW-R", "FSK-R"}
		if !reflect.DeepEqual(caps.Modes, wantModes) {
			t.Errorf("%s: Modes = %v, want %v (§1.5)", tc.model, caps.Modes, wantModes)
		}
		if caps.TagLen != 8 { // §1.6, P16 "up to 8 digits" (590:1576)
			t.Errorf("%s: TagLen = %d, want 8 (§1.6)", tc.model, caps.TagLen)
		}
		// §1.7, §1.8, M-E5: the record has NO clarifier position.
		if caps.ClarMaxHz != 0 || caps.ClarStepHz != 0 {
			t.Errorf("%s: Clar{Max,Step}Hz = %d/%d, want 0/0 (§1.7, §1.8, M-E5)", tc.model, caps.ClarMaxHz, caps.ClarStepHz)
		}
		if !reflect.DeepEqual(caps.CTCSSTones, kenwoodTonesWant) { // §1.9
			t.Errorf("%s: CTCSSTones = %v, want the 43-entry TN chart (§1.9)", tc.model, caps.CTCSSTones)
		}
		if caps.CTCSSToneRange != nil { // §1.10
			t.Errorf("%s: CTCSSToneRange = %v, want nil (§1.10)", tc.model, caps.CTCSSToneRange)
		}
		wantBauds := []int{9600, 19200, 38400, 57600, 115200} // §1.11, M-E4
		if !reflect.DeepEqual(caps.Bauds, wantBauds) {
			t.Errorf("%s: Bauds = %v, want %v with 4800 omitted (§1.11, M-E4)", tc.model, caps.Bauds, wantBauds)
		}
		if caps.DefaultBaud != 9600 { // §1.12, A15
			t.Errorf("%s: DefaultBaud = %d, want 9600 (§1.12, A15)", tc.model, caps.DefaultBaud)
		}
		if caps.MinFreqHz != 0 || caps.MaxFreqHz != 0 { // §1.13, §1.14, M-E6
			t.Errorf("%s: Min/MaxFreqHz = %d/%d, want 0/0 (§1.13, §1.14, M-E6)", tc.model, caps.MinFreqHz, caps.MaxFreqHz)
		}
		if caps.RequiredSlots != nil { // §1.15
			t.Errorf("%s: RequiredSlots = %v, want nil (§1.15)", tc.model, caps.RequiredSlots)
		}
		if len(caps.ShiftOptions) != 0 || len(caps.CTCSSStates) != 0 { // §1.16, §1.17
			t.Errorf("%s: Shift/CTCSSStates = %v/%v, want both empty (§1.16, §1.17)", tc.model, caps.ShiftOptions, caps.CTCSSStates)
		}
		if len(caps.DuplexOptions) != 0 { // §1.18
			t.Errorf("%s: DuplexOptions = %v, want empty (§1.18)", tc.model, caps.DuplexOptions)
		}
		wantToneModes := []spec.ToneMode{ // §1.19: four values, "3: Cross Tone ON"
			{Value: "OFF", Semantics: spec.ToneModeOff},
			{Value: "TONE", Semantics: spec.ToneModeCTCSS},
			{Value: "CTCSS", Semantics: spec.ToneModeCTCSSRxSquelch},
			{Value: "CROSS", Semantics: spec.ToneModeCross},
		}
		if !reflect.DeepEqual(caps.ToneModes, wantToneModes) {
			t.Errorf("%s: ToneModes = %v, want %v (§1.19)", tc.model, caps.ToneModes, wantToneModes)
		}
		if len(caps.DTCSPolarities) != 0 || len(caps.DTCSCodes) != 0 { // §1.20, §1.21
			t.Errorf("%s: DTCS vocabularies = %v/%v, want both empty (§1.20, §1.21)", tc.model, caps.DTCSPolarities, caps.DTCSCodes)
		}
		if !reflect.DeepEqual(caps.Filters, tc.filters) { // §1.22, M-E7
			t.Errorf("%s: Filters = %v, want %v (§1.22, M-E7)", tc.model, caps.Filters, tc.filters)
		}
		if len(caps.TuningSteps) != 0 || caps.ProgramTuningStepRange != nil { // §1.23, §1.24
			t.Errorf("%s: tuning-step vocabulary = %v/%v, want empty/nil (§1.23, §1.24)", tc.model, caps.TuningSteps, caps.ProgramTuningStepRange)
		}
		// §1.25, §1.26, §1.27: radio-level functions with no record position.
		if len(caps.AttenuatorDB) != 0 || len(caps.PreampOptions) != 0 || len(caps.AntennaOptions) != 0 {
			t.Errorf("%s: attenuator/preamp/antenna = %v/%v/%v, want all empty (§1.25-§1.27)", tc.model, caps.AttenuatorDB, caps.PreampOptions, caps.AntennaOptions)
		}
		if caps.TagCharset != "" { // §1.28, A2
			t.Errorf("%s: TagCharset = %q, want the family default (§1.28, A2)", tc.model, caps.TagCharset)
		}
	}
}

// TestCapabilities_EveryFieldExplicit reflects over spec.Capabilities and
// requires each of its twenty-eight fields to be either populated or
// DELIBERATELY empty by name, the core/driver/ftdx10 shape. A zero left in one
// of the populated ones is not a neutral omission: a zero MaxFreqHz reads as
// "no ceiling" to every validator, and a zero TagLen makes CHIRP import
// truncate every name to "".
func TestCapabilities_EveryFieldExplicit(t *testing.T) {
	// The seventeen the matrix records as deliberately EMPTY on the 590 rows,
	// each with the section that says so. Filters is NOT here: it is empty
	// on one row and populated on the other, so it is checked per row in
	// TestCapabilities_MatrixValuesPerRow instead.
	deliberatelyEmpty := map[string]string{
		"ClarMaxHz":              "§1.7, M-E5 — the 50-byte record has no clarifier position",
		"ClarStepHz":             "§1.8, M-E5 — as ClarMaxHz",
		"CTCSSToneRange":         "§1.10 — the tone field is an INDEX into a printed chart",
		"MinFreqHz":              "§1.13, M-E6 — no frequency range is printed anywhere",
		"MaxFreqHz":              "§1.14, M-E6 — as MinFreqHz; a zero DISABLES the ceiling check",
		"RequiredSlots":          "§1.15 — neither book marks any channel mandatory",
		"ShiftOptions":           "§1.16 — the Yaesu half of the vocabulary pair (decision 6)",
		"CTCSSStates":            "§1.17 — as ShiftOptions",
		"DuplexOptions":          "§1.18 — no duplex selector in the record",
		"DTCSPolarities":         "§1.20 — DCS appears nowhere in either book",
		"DTCSCodes":              "§1.21 — as DTCSPolarities",
		"TuningSteps":            "§1.23 — bytes 39-40 are the FM Normal/Narrow flag here",
		"ProgramTuningStepRange": "§1.24 — no step magnitude in hertz in the record",
		"AttenuatorDB":           "§1.25 — a radio-level function with no record position",
		"PreampOptions":          "§1.26 — as AttenuatorDB",
		"AntennaOptions":         "§1.27 — as AttenuatorDB",
		"TagCharset":             "§1.28, A2 — the empty string selects the family default",
	}
	for _, row := range bothRows {
		caps := CapabilitiesUnverified(row)
		v := reflect.ValueOf(caps)
		typ := v.Type()
		if typ.NumField() != 28 {
			t.Fatalf("spec.Capabilities has %d fields, want 28 — this test's list is stale", typ.NumField())
		}
		for i := 0; i < typ.NumField(); i++ {
			name := typ.Field(i).Name
			zero := v.Field(i).IsZero()
			reason, expectedEmpty := deliberatelyEmpty[name]
			switch {
			case name == "Filters":
				// Per row; see this test's doc comment.
			case zero && !expectedEmpty:
				t.Errorf("%s: %s is the zero value and is not on the deliberately-empty list", modelNameFor(row), name)
			case !zero && expectedEmpty:
				t.Errorf("%s: %s is populated but the deliberately-empty list says %q", modelNameFor(row), name, reason)
			}
		}
	}
}

// TestBanks_PerRow pins P11 and matrix §1.4 exactly, including the ruling
// that the TS-590SG's extension channels 110-119 are OMITTED until A11 lifts.
func TestBanks_PerRow(t *testing.T) {
	var wantMem []string
	for n := 0; n <= 99; n++ {
		wantMem = append(wantMem, fmt.Sprintf("%03d", n))
	}
	var wantScan []string
	for n := 100; n <= 109; n++ {
		wantScan = append(wantScan, fmt.Sprintf("%03dL", n), fmt.Sprintf("%03dU", n))
	}

	for _, row := range bothRows {
		caps := CapabilitiesUnverified(row)
		if len(caps.Banks) != 2 {
			t.Fatalf("%s: %d banks, want MEM and SCAN (§1.4)", modelNameFor(row), len(caps.Banks))
		}
		mem, ok := caps.Bank(spec.BankMemory)
		if !ok {
			t.Fatalf("%s: no MEM bank", modelNameFor(row))
		}
		if !reflect.DeepEqual(mem.Slots, wantMem) {
			t.Errorf("%s: MEM slots = %v, want 000-099 (§1.4.1)", modelNameFor(row), mem.Slots)
		}
		if mem.Label != "Memories" {
			t.Errorf("%s: MEM label = %q, want %q (§1.4.1)", modelNameFor(row), mem.Label, "Memories")
		}
		scan, ok := caps.Bank(spec.BankScan)
		if !ok {
			t.Fatalf("%s: no SCAN bank", modelNameFor(row))
		}
		if !reflect.DeepEqual(scan.Slots, wantScan) {
			t.Errorf("%s: SCAN slots = %v, want 100L-109U (§1.4.3)", modelNameFor(row), scan.Slots)
		}
		if scan.Label != "Scan edges (P00–P09)" {
			t.Errorf("%s: SCAN label = %q (§1.4.3)", modelNameFor(row), scan.Label)
		}
		for _, b := range caps.Banks {
			if b.NoBlank {
				t.Errorf("%s: bank %s has NoBlank true, want false stated explicitly (§2.5)", modelNameFor(row), b.ID)
			}
			if b.Sparse || b.Groups != 0 || b.GroupBase != 0 || b.PerGroup != 0 || b.ChannelBase != 0 || b.Budget != 0 || b.BudgetUnstated {
				t.Errorf("%s: bank %s declares sparse-space fields; these are small dense printed spaces (§1.4)", modelNameFor(row), b.ID)
			}
		}
		// The negative that carries the design: no slot string appears in
		// two banks of one row.
		seen := map[string]spec.BankID{}
		for _, b := range caps.Banks {
			for _, s := range b.Slots {
				if other, dup := seen[s]; dup {
					t.Errorf("%s: slot %q appears in both %s and %s", modelNameFor(row), s, other, b.ID)
				}
				seen[s] = b.ID
			}
		}
	}
}

// TestBanks_TheSGExtensionChannelsAreOmitted is Stuart's decisions row 6,
// RULED 05/09/2026, at the capability layer: 110-119 are not slot IDs
// anywhere on the SG row.
//
// THE CODEC STILL DECLARES THEM (A12's counterpart in core/kw/ts590: the SG
// layout's SlotExtension range 110-119), so an MC answer of 115 parses. What
// this pins is the DRIVER's published banks, which is the other half of that
// pair and the half a user sees.
func TestBanks_TheSGExtensionChannelsAreOmitted(t *testing.T) {
	for _, row := range bothRows {
		caps := CapabilitiesUnverified(row)
		for _, b := range caps.Banks {
			for _, s := range b.Slots {
				for _, ext := range []string{"110", "115", "119"} {
					if strings.HasPrefix(s, ext) {
						t.Errorf("%s: bank %s publishes slot %q; 110-119 are OMITTED until A11 lifts (§1.4.2, Stuart decisions row 6)", modelNameFor(row), b.ID, s)
					}
				}
			}
		}
	}
}

// TestBankFields_NameEveryOneOfTheTwentySeven is P5's own list applied to
// every bank of every row: a field left OUT of a map reads identically to a
// field deliberately zeroed, and only a written-down zero is legible as a
// decision.
func TestBankFields_NameEveryOneOfTheTwentySeven(t *testing.T) {
	for _, row := range bothRows {
		for _, profile := range []struct {
			name string
			caps spec.Capabilities
		}{
			{"unverified", CapabilitiesUnverified(row)},
			{"simulated", CapabilitiesSimulated(row)},
		} {
			for _, b := range profile.caps.Banks {
				if len(b.Fields) != len(allSpecFields) {
					t.Errorf("%s %s bank %s: %d field entries, want all %d written explicitly", modelNameFor(row), profile.name, b.ID, len(b.Fields), len(allSpecFields))
				}
				for _, f := range allSpecFields {
					if _, ok := b.Fields[f]; !ok {
						t.Errorf("%s %s bank %s: %s is absent from the map, so its zero is not legible as a decision", modelNameFor(row), profile.name, b.ID, f)
					}
				}
			}
		}
	}
}

// TestBankFields_MatrixGrades walks matrix §2.1 cell by cell, on both banks
// of both rows and in both profiles.
func TestBankFields_MatrixGrades(t *testing.T) {
	for _, row := range bothRows {
		for _, profile := range []struct {
			name string
			caps spec.Capabilities
			rw   spec.FieldSupport
		}{
			{"unverified", CapabilitiesUnverified(row), spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}},
			{"simulated", CapabilitiesSimulated(row), spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}},
		} {
			for _, bankID := range []spec.BankID{spec.BankMemory, spec.BankScan} {
				b, ok := profile.caps.Bank(bankID)
				if !ok {
					t.Fatalf("%s: no %s bank", modelNameFor(row), bankID)
				}
				for _, f := range allSpecFields {
					want := spec.FieldSupport{}
					switch f {
					case spec.FieldFrequency, spec.FieldMode, spec.FieldTag,
						spec.FieldScanSkip, spec.FieldToneMode,
						spec.FieldToneTx, spec.FieldToneRx, spec.FieldDataMode:
						want = profile.rw
					case spec.FieldTxFrequency:
						// §2.4, M-E2: graded in MEM, zero in SCAN, where
						// P1=1 is the section channel's END frequency.
						if bankID == spec.BankMemory {
							want = profile.rw
						}
					case spec.FieldFilter:
						// §1.22, §2.7, Q12: the SG's byte 28 is live; the
						// S's is firmware-conditional and a table is not.
						if row == RowSG {
							want = profile.rw
						}
					}
					if got := b.Fields[f]; got != want {
						t.Errorf("%s %s bank %s: %s = %+v, want %+v (§2.1)", modelNameFor(row), profile.name, bankID, f, got, want)
					}
				}
			}
		}
	}
}

// TestFieldAudit_CoversEverySpecField consumes the fleet helper as a BLACK
// BOX (plan P5): the audited list is this driver's own hand-written one, per
// row, and every other field carries a written reason.
func TestFieldAudit_CoversEverySpecField(t *testing.T) {
	for _, row := range bothRows {
		drivertest.AssertFieldAuditCoversEverySpecField(t, "auditedFields("+modelNameFor(row)+")", auditedFields(row), unexpressedFields(row))
	}
}

// auditedFields is the hand-written list of the fields THIS ROW's record
// expresses — the list a Field addition must be added to.
func auditedFields(row Row) []spec.Field {
	fields := []spec.Field{
		spec.FieldFrequency, spec.FieldMode, spec.FieldTag,
		spec.FieldScanSkip, spec.FieldTxFrequency, spec.FieldToneMode,
		spec.FieldToneTx, spec.FieldToneRx, spec.FieldDataMode,
	}
	if row == RowSG {
		fields = append(fields, spec.FieldFilter)
	}
	return fields
}

// unexpressedFields is the other half of the audit: every field this row
// grades the zero FieldSupport, with the matrix's own reason.
func unexpressedFields(row Row) map[spec.Field]string {
	out := map[spec.Field]string{
		spec.FieldClarifier:         "§1.7, M-E5: the 50-byte record has NO clarifier position over a complete 47-byte account (590:1539-1577); the radios DO have RIT and XIT",
		spec.FieldCTCSSState:        "§2.1: this record expresses tone as P7's mode selector, which is FieldToneMode",
		spec.FieldCTCSSTone:         "§2.1: the record carries TWO independent tone indices (P8, P9) and FieldCTCSSTone is ONE field",
		spec.FieldShift:             "§1.18: no shift selector; split is two frames",
		spec.FieldTagDisplay:        "§2.1: no tag-display flag anywhere in the record",
		spec.FieldErase:             "§2.8, decision 8: the short MW erase form is never built and the gate refuses any MW that is not 50 bytes",
		spec.FieldDuplex:            "§1.18: no duplex selector in the record",
		spec.FieldOffset:            "§1.18: no per-channel offset magnitude anywhere in the record",
		spec.FieldDTCSCode:          "§1.21: DCS appears nowhere in either book",
		spec.FieldDTCSPolarity:      "§1.20: as FieldDTCSCode",
		spec.FieldTuningStepEnabled: "§2.1: no on/off flag for a step exists in the record",
		spec.FieldTuningStep:        "§1.23: bytes 39-40 are the FM Normal/Narrow flag on these rows, not a step",
		spec.FieldProgramTuningStep: "§1.24: no step magnitude in hertz in the record",
		spec.FieldAttenuator:        "§1.25: a radio-level function (RA) with no record position",
		spec.FieldPreamp:            "§1.26: a radio-level function (PA) with no record position",
		spec.FieldAntenna:           "§1.27: no antenna-select position in the record",
		spec.FieldIPPlus:            "§2.1: an Icom concept with no position in this frame and no mention in either book",
	}
	if row == RowS {
		out[spec.FieldFilter] = "§1.22, §2.7, Q12: byte 28 is live on a TS-590S at firmware >= 2.00 and \"always 0\" only at 1.xx (590:1478), and a static per-model table cannot publish a firmware condition"
	}
	return out
}

// TestWriteTrialsComplete_PinnedFalse is P22's TWO-PART pin: the constant is
// false, AND the RealHardware baseline it travels with is genuinely
// nothing-writable on BOTH rows — so a constant-only edit cannot pass while
// leaving the consequence untested.
func TestWriteTrialsComplete_PinnedFalse(t *testing.T) {
	if writeTrialsComplete {
		t.Error("writeTrialsComplete is true; no Kenwood radio has ever been written to by this project (matrix §3.12)")
	}
	for _, row := range bothRows {
		caps := CapabilitiesUnverified(row)
		for _, b := range caps.Banks {
			for _, f := range allSpecFields {
				if b.Fields[f].CanWrite() {
					t.Errorf("%s: RealHardware bank %s grades %s writable while writeTrialsComplete is false", modelNameFor(row), b.ID, f)
				}
			}
		}
	}
}

// TestCTCSSTones_AreNotTheProjectsSharedChart is P6's negative pin. It is the
// ONE place in this package spec.StandardCTCSSTones() may be named, and
// TestNoProductionFileNamesTheSharedToneChart holds the production half down.
func TestCTCSSTones_AreNotTheProjectsSharedChart(t *testing.T) {
	shared := spec.StandardCTCSSTones()
	kenwood := CapabilitiesUnverified(RowS).CTCSSTones
	if len(kenwood) == len(shared) {
		t.Fatalf("the Kenwood chart has %d entries and the shared one %d; they must differ (§1.9)", len(kenwood), len(shared))
	}
	// Every index above 25 differs, because the Kenwood chart is the shared
	// one minus eight interstitial tones (§1.9).
	for i := 26; i < len(kenwood) && i < len(shared); i++ {
		if kenwood[i] == shared[i] {
			t.Errorf("index %d is %v in both charts; the Kenwood chart drops eight interstitial tones from index 26 on (§1.9)", i, kenwood[i])
		}
	}
	if kenwood[len(kenwood)-1] != 17500 {
		t.Errorf("the last Kenwood tone is %v, want 1750.0 Hz — TN index 42, which CN has no equivalent of (§1.9, M-E1)", kenwood[len(kenwood)-1])
	}
}

// TestNoProductionFileNamesTheSharedToneChart is P6's rule as a guard: the
// Kenwood chart is the shared chart minus eight tones, so every index above
// 25 differs, and a package that reached for the shared one would misaddress
// every tone on both rows.
func TestNoProductionFileNamesTheSharedToneChart(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			continue
		}
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", name, err)
		}
		if strings.Contains(string(b), "StandardCTCSSTones") {
			t.Errorf("%s names spec.StandardCTCSSTones; the Kenwood chart is the shared chart minus eight interstitial tones and every index above 25 differs (P6, §1.9)", name)
		}
	}
}

// TestCapabilities_AreDefensiveCopies pins that a caller mutating what it was
// handed can never alter what the next caller sees — load-bearing for the
// write gate, exactly as in the sibling drivers.
func TestCapabilities_AreDefensiveCopies(t *testing.T) {
	for _, row := range bothRows {
		caps := CapabilitiesUnverified(row)
		caps.Modes[0] = "MUTATED"
		caps.CTCSSTones[0] = 1
		caps.Bauds[0] = 1
		caps.Banks[0].Slots[0] = "MUTATED"
		caps.Banks[0].Fields[spec.FieldFrequency] = spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}

		fresh := CapabilitiesUnverified(row)
		if fresh.Modes[0] == "MUTATED" || fresh.Banks[0].Slots[0] == "MUTATED" {
			t.Errorf("%s: a mutated capability copy reached the next caller", modelNameFor(row))
		}
		if fresh.CTCSSTones[0] != 670 || fresh.Bauds[0] != 9600 {
			t.Errorf("%s: a mutated slice reached the next caller", modelNameFor(row))
		}
		if fresh.Banks[0].Fields[spec.FieldFrequency].CanWrite() {
			t.Errorf("%s: a mutated field map reached the next caller", modelNameFor(row))
		}
	}
}

// TestCapabilities_ZeroFrequencyBoundsDisableTheCheck is the POSITIVE PROOF
// that 0/0 is a disabled check rather than a forgotten field (matrix M-E6,
// §1.13, §1.14). A channel at 1 Hz and one at 99 GHz both pass
// codeplug.Validate's frequency rules, because both guards test != 0 first —
// no false refusal and no false promise — while a frequency needing more than
// the printed ELEVEN DIGITS is refused by the CODEC, whose error says in as
// many words that it names the field width and not any radio's tuning range
// (A17).
func TestCapabilities_ZeroFrequencyBoundsDisableTheCheck(t *testing.T) {
	for _, row := range bothRows {
		caps := CapabilitiesUnverified(row)
		for _, hz := range []uint64{1, 99_000_000_000} {
			cp := &codeplug.Codeplug{
				Radio:    codeplug.RadioInfo{Model: caps.Model, CATID: caps.CATID},
				Channels: []codeplug.Channel{{Slot: "000", Data: &codeplug.ChannelData{FreqHz: hz, Mode: "FM"}}},
			}
			for _, issue := range codeplug.Validate(cp, caps) {
				if issue.Field == spec.FieldFrequency {
					t.Errorf("%s at %d Hz: %s — a zero Min/MaxFreqHz must DISABLE the bound, not enforce one", modelNameFor(row), hz, issue.Msg)
				}
			}
		}
	}

	// The one frequency guard this milestone ships is the codec's, and it is
	// about the WIRE: MR/MW P4 is eleven digits (590:1541-1543).
	l := kwts590.LayoutS()
	slot, err := l.NewSlot(0, kw.ScanHalfNone)
	if err != nil {
		t.Fatalf("NewSlot: %v", err)
	}
	_, err = l.BuildMWSet(kw.Record{
		Slot: slot, FreqHz: kw.MaxRecordFreqHz + 1, Mode: kw.ModeFM,
		Byte19: '0', ToneMode: kw.ToneModeOff, Byte28: '0',
		Byte3940: "00", Byte41: '0', Name: "X",
	})
	if !errors.Is(err, kw.ErrOutOfDomain) {
		t.Errorf("BuildMWSet at 12 digits: err = %v, want kw.ErrOutOfDomain", err)
	}
}

// TestCapabilities_ASessionHandsOutDefensiveCopies is the SESSION half of the
// defence, and it is the half spec.Capabilities.Clone is actually for. Its
// sibling above mutates the result of CapabilitiesUnverified and compares
// against a freshly BUILT set, which baseCapabilities makes true whatever
// spec.Capabilities.Clone does; only a second call on the SAME session can
// witness that Session.Capabilities copied anything.
//
// It is load-bearing from Stage 2 task 12, when WriteChannel begins enforcing
// against s.caps: a caller that mutated what it was handed must not be able
// to widen the gate it is about to be measured by.
//
// RED PROOF, observed: with spec.Capabilities.Clone's "return out" replaced
// by "return caps" this test fails at the mutated bank slot and the mutated
// field grade, while the rest of the package stays green.
func TestCapabilities_ASessionHandsOutDefensiveCopies(t *testing.T) {
	for _, row := range bothRows {
		sess, _ := openTestSession(t, row, radioImage{})
		handed := sess.Capabilities()
		handed.Modes[0] = "MUTATED"
		handed.Bauds[0] = 1
		handed.Banks[0].Slots[0] = "MUTATED"
		handed.Banks[0].Fields[spec.FieldErase] = spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}

		next := sess.Capabilities()
		if next.Modes[0] == "MUTATED" || next.Bauds[0] == 1 {
			t.Errorf("%s: a mutated slice reached the next Session.Capabilities caller", modelNameFor(row))
		}
		if next.Banks[0].Slots[0] == "MUTATED" {
			t.Errorf("%s: a mutated bank slot reached the next Session.Capabilities caller", modelNameFor(row))
		}
		if next.Banks[0].Fields[spec.FieldErase].CanWrite() {
			t.Errorf("%s: a mutated field grade reached the next Session.Capabilities caller — this is the write gate T12 enforces against, and no Kenwood row grades an erase at all", modelNameFor(row))
		}
	}
}
