// SPDX-License-Identifier: GPL-3.0-or-later

package ts480

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// allSpecFields is THIS package's own literal list of the twenty-seven
// spec.Field constants (plan P5). It is deliberately not spec.AllFields():
// a list that must be edited when a Field is added, and that fails loudly
// when one is, is what P5 asks every Kenwood package for.
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

// profiles is the two declared Profile values with their read/write pair, so
// every per-profile table below covers both.
var profiles = []struct {
	name string
	rw   spec.FieldSupport
	caps func() spec.Capabilities
}{
	{"unverified", spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}, CapabilitiesUnverified},
	{"simulated", spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}, CapabilitiesSimulated},
}

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

// TestAllSpecFields_MatchesTheAuditedAndUnexpressedFields anchors the literal
// list without naming spec.AllFields here (P5): the fleet helper already
// proves audited ∪ unexpressed partitions spec.AllFields() exactly, so
// pinning allSpecFields against THAT union anchors it transitively.
func TestAllSpecFields_MatchesTheAuditedAndUnexpressedFields(t *testing.T) {
	want := map[spec.Field]bool{}
	for _, f := range auditedFields() {
		want[f] = true
	}
	for f := range unexpressedFields() {
		want[f] = true
	}
	for _, f := range allSpecFields {
		if !want[f] {
			t.Errorf("allSpecFields names %s, which is neither audited nor unexpressed", f)
		}
		delete(want, f)
	}
	for f := range want {
		t.Errorf("audited/unexpressed names %s, which allSpecFields does not", f)
	}
}

// TestCapabilities_ValidateOnEveryProfile is the floor: a capability set that
// does not validate cannot be registered at all — and this row is built to
// the same bar as the two registered ones even though it is NOT registered
// (P3).
func TestCapabilities_ValidateOnEveryProfile(t *testing.T) {
	for _, p := range profiles {
		if err := p.caps().Validate(); err != nil {
			t.Errorf("%s profile: Validate: %v", p.name, err)
		}
	}
}

// TestCapabilities_MatrixValues walks the matrix's TS-480 column, field by
// field. Every value below is derivable from a named matrix section; a value
// that is not is a STOP under the ground rules.
func TestCapabilities_MatrixValues(t *testing.T) {
	caps := CapabilitiesSimulated()
	if caps.Model != "TS-480" {
		t.Errorf("Model = %q, want %q (§1.1, 480:678)", caps.Model, "TS-480")
	}
	if caps.CATID != "020" {
		t.Errorf("CATID = %q, want %q (§1.2, 480:678)", caps.CATID, "020")
	}
	if caps.Transmit != spec.HasTransmitter {
		t.Errorf("Transmit = %v, want HasTransmitter (§1.3, 480:51-52)", caps.Transmit)
	}
	// §1.5: EIGHT names, in wire-code order, and NO FM-N — P14 is the
	// tuning step on this row, not an FM width flag (480:979).
	wantModes := []string{"LSB", "USB", "CW", "FM", "AM", "FSK", "CW-R", "FSK-R"}
	if !reflect.DeepEqual(caps.Modes, wantModes) {
		t.Errorf("Modes = %v, want %v (§1.5)", caps.Modes, wantModes)
	}
	for _, name := range caps.Modes {
		if name == "FM-N" {
			t.Error("Modes carries FM-N; bytes 39-40 are the STEP INDEX on this row (480:979), not the 590 pair's FM Normal/Narrow flag")
		}
	}
	if caps.TagLen != 8 {
		t.Errorf("TagLen = %d, want 8 (§1.6, 480:984)", caps.TagLen)
	}
	if caps.ClarMaxHz != 0 || caps.ClarStepHz != 0 {
		t.Errorf("Clar bounds = %d/%d, want 0/0 (§1.7, §1.8, M-E5: no clarifier POSITION over the complete account at 480:951-984)", caps.ClarMaxHz, caps.ClarStepHz)
	}
	// §1.9: CANNOT ESTABLISH. Neither printed chart is in this book, so no
	// tone is sendable to a TS-480 by this programme.
	if caps.CTCSSTones != nil {
		t.Errorf("CTCSSTones = %v, want nil (§1.9: TN's chart is \"page 32 of the TS-480 instruction manual\", 480:1559-1560, and CN's page 33, 480:339-340)", caps.CTCSSTones)
	}
	if caps.CTCSSToneRange != nil {
		t.Errorf("CTCSSToneRange = %v, want nil (§1.10)", caps.CTCSSToneRange)
	}
	// The fail-closed consequence, asserted rather than assumed: with
	// neither a list nor a range declared, AdmitsTone refuses everything.
	for _, tone := range []spec.Tone{670, 885, 2541, 17500} {
		if caps.AdmitsTone(tone) {
			t.Errorf("AdmitsTone(%v) = true on a row with no tone domain; §1.9 requires the fail-closed branch", tone)
		}
	}
	wantBauds := []int{9600, 19200, 38400, 57600, 115200}
	if !reflect.DeepEqual(caps.Bauds, wantBauds) {
		t.Errorf("Bauds = %v, want %v (§1.11, M-E4: 4800 omitted because it requires TWO stop bits on this radio, 480:22-23)", caps.Bauds, wantBauds)
	}
	if caps.DefaultBaud != 9600 {
		t.Errorf("DefaultBaud = %d, want 9600 (§1.12, A15)", caps.DefaultBaud)
	}
	if caps.MinFreqHz != 0 || caps.MaxFreqHz != 0 {
		t.Errorf("frequency bounds = %d/%d, want 0/0 (§1.13, §1.14, M-E6: a field width is not a tuning range)", caps.MinFreqHz, caps.MaxFreqHz)
	}
	if caps.RequiredSlots != nil {
		t.Errorf("RequiredSlots = %v, want nil (§1.15)", caps.RequiredSlots)
	}
	if len(caps.ShiftOptions) != 0 || len(caps.CTCSSStates) != 0 {
		t.Error("the Yaesu vocabulary half is populated; §1.16/§1.17 require both empty under decision 6")
	}
	if len(caps.DuplexOptions) != 0 {
		t.Errorf("DuplexOptions = %v, want empty (§1.18: no duplex selector in the record)", caps.DuplexOptions)
	}
	// §1.19: THREE values, no Cross Tone — "0: OFF, 1: TONE, 2: CTCSS"
	// (480:964), against the 590 pair's four (590:1549-1553).
	wantToneModes := []spec.ToneMode{
		{Value: "OFF", Semantics: spec.ToneModeOff},
		{Value: "TONE", Semantics: spec.ToneModeCTCSS},
		{Value: "CTCSS", Semantics: spec.ToneModeCTCSSRxSquelch},
	}
	if !reflect.DeepEqual(caps.ToneModes, wantToneModes) {
		t.Errorf("ToneModes = %+v, want %+v (§1.19, 480:964; the SEMANTICS are K-D1 and unlifted)", caps.ToneModes, wantToneModes)
	}
	if len(caps.DTCSPolarities) != 0 || len(caps.DTCSCodes) != 0 {
		t.Error("DTCS vocabulary is populated; §1.20/§1.21: DCS appears nowhere in either book")
	}
	if len(caps.Filters) != 0 {
		t.Errorf("Filters = %v, want empty (§1.22: byte 28 is \"Always 0 for the TS-480.\", 480:973)", caps.Filters)
	}
	if len(caps.TuningSteps) != 0 {
		t.Errorf("TuningSteps = %v, want empty (§1.23, A22: ST's legend is mode-conditional over two ranges, 480:1494-1500, and a flat label list cannot carry a mode axis)", caps.TuningSteps)
	}
	if caps.ProgramTuningStepRange != nil {
		t.Errorf("ProgramTuningStepRange = %v, want nil (§1.24)", caps.ProgramTuningStepRange)
	}
	if len(caps.AttenuatorDB) != 0 || len(caps.PreampOptions) != 0 || len(caps.AntennaOptions) != 0 {
		t.Error("a radio-level vocabulary is published as a per-channel one; §1.25-§1.27 require all three empty")
	}
	if caps.TagCharset != "" {
		t.Errorf("TagCharset = %q, want \"\" — the family default, which on this row is STRICTER than the book (§1.28, A2)", caps.TagCharset)
	}
}

// TestCapabilities_EveryFieldExplicit is the FTdx10 discipline: every one of
// spec.Capabilities' twenty-nine fields is populated deliberately, the
// non-zero ones and the deliberately empty ones alike, so an omission and a
// decision cannot read the same.
func TestCapabilities_EveryFieldExplicit(t *testing.T) {
	// The seventeen §1 records as deliberately EMPTY on this row, plus
	// CTCSSTones, which is empty HERE and populated on the 590 pair — the
	// one entry of this list that is not a family fact.
	deliberatelyEmpty := map[string]string{
		"ClarMaxHz":              "§1.7, M-E5",
		"ClarStepHz":             "§1.8, M-E5",
		"CTCSSTones":             "§1.9: CANNOT ESTABLISH — neither chart is in this book",
		"CTCSSToneRange":         "§1.10",
		"MinFreqHz":              "§1.13, M-E6",
		"MaxFreqHz":              "§1.14, M-E6",
		"RequiredSlots":          "§1.15",
		"ShiftOptions":           "§1.16",
		"CTCSSStates":            "§1.17",
		"DuplexOptions":          "§1.18",
		"DTCSPolarities":         "§1.20",
		"DTCSCodes":              "§1.21",
		"Filters":                "§1.22, 480:973",
		"TuningSteps":            "§1.23, A22",
		"ProgramTuningStepRange": "§1.24",
		"AttenuatorDB":           "§1.25",
		"PreampOptions":          "§1.26",
		"AntennaOptions":         "§1.27",
		"TagCharset":             "§1.28",
		"SimplexTx":              "this row does not grade FieldTxFrequency, so the blank arm leaves nothing to state. THE TS-480 IS UNREGISTERED, and the 09/09/2026 design declared only the six registered rows",
	}
	caps := CapabilitiesSimulated()
	v := reflect.ValueOf(caps)
	typ := v.Type()
	if typ.NumField() != 29 {
		t.Fatalf("spec.Capabilities has %d fields, this test knows 29", typ.NumField())
	}
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		zero := v.Field(i).IsZero()
		if _, empty := deliberatelyEmpty[name]; empty {
			if !zero {
				t.Errorf("%s is populated, and this row records it as deliberately empty (%s)", name, deliberatelyEmpty[name])
			}
			continue
		}
		if zero {
			t.Errorf("%s is the zero value and is not on the deliberately-empty list — a zero capability field is not a neutral omission", name)
		}
	}
}

// TestBanks_IsOneFlatMEMBankOfTwoDigitSlots is §1.4's TS-480 row, and the two
// differences from the 590 pair are BOTH in the assertion:
//
// ONE BANK, NO SCAN. Channels 90-99 really do answer a second frame through
// the P1 overload (480:943-944, 480:986-987), but on this radio they are
// ordinary memories rather than a separate class: there is no bank field in
// the record at all (480:827), a slot string is unique across a codeplug, and
// a Bank.Fields map is per bank rather than per slot. A scan bank here would
// either duplicate ten slot identities or take ten ordinary memories away
// from the owner (§1.4.3, decision 15). The P1=1 half of 90-99 is therefore
// unreachable through this programme — a real loss, published.
//
// TWO-DIGIT SLOT STRINGS, and they are NOT kw.Slot.String()'s. The codec
// renders every slot "%03d" because that is the 590 pair's printed width,
// where MC prints a hundreds digit and a two-digit remainder; this radio's MC
// prints neither — "0: Always 0 for the TS-480 (Memory bank number)."
// (480:827) and "00 ~ 99: Channel number" (480:830) — so its canonical slot
// string is the book's own two digits (§1.4.1, "the wire form is the
// CHOICE"). A row that published "042" would misdescribe its own wire.
func TestBanks_IsOneFlatMEMBankOfTwoDigitSlots(t *testing.T) {
	caps := CapabilitiesSimulated()
	if len(caps.Banks) != 1 {
		t.Fatalf("Banks has %d entries, want exactly one — this row has NO scan bank (§1.4.3)", len(caps.Banks))
	}
	b := caps.Banks[0]
	if b.ID != spec.BankMemory {
		t.Errorf("bank ID = %v, want %v", b.ID, spec.BankMemory)
	}
	if b.Label != memBankLabel {
		t.Errorf("bank Label = %q, want %q", b.Label, memBankLabel)
	}
	want := make([]string, 0, 100)
	for n := 0; n <= 99; n++ {
		want = append(want, string([]byte{byte('0' + n/10), byte('0' + n%10)}))
	}
	if !reflect.DeepEqual(b.Slots, want) {
		t.Errorf("bank Slots = %v…%v (%d), want \"00\"…\"99\" (100), the book's own two-digit width (480:830, 480:955)", b.Slots[:2], b.Slots[len(b.Slots)-1:], len(b.Slots))
	}
	if b.NoBlank {
		t.Error("NoBlank is true; §2.5 states it false on every bank of every row, and nothing in this book imposes the PMS invariant")
	}
	if b.Sparse || b.Groups != 0 || b.GroupBase != 0 || b.PerGroup != 0 || b.ChannelBase != 0 || b.Budget != 0 || b.BudgetUnstated {
		t.Errorf("bank %v declares sparse-space fields; §1.4 requires Sparse false with the sparse fields zero — this is a small dense space fully printed in the book (480:955)", b.ID)
	}
	for _, id := range b.Slots {
		if len(id) != 2 {
			t.Errorf("slot %q is not two digits — kw.Slot.String()'s %%03d rendering is the 590 pair's printed width, not this row's", id)
		}
	}
}

// TestBankFields_NameEveryOneOfTheTwentySeven: a field left OUT of the map
// reads identically to a field deliberately zeroed, and only a written-down
// zero is legible as a decision (§2).
func TestBankFields_NameEveryOneOfTheTwentySeven(t *testing.T) {
	for _, p := range profiles {
		for _, b := range p.caps().Banks {
			if len(b.Fields) != len(allSpecFields) {
				t.Errorf("%s bank %s: %d field entries, want all %d written explicitly", p.name, b.ID, len(b.Fields), len(allSpecFields))
			}
			for _, f := range allSpecFields {
				if _, ok := b.Fields[f]; !ok {
					t.Errorf("%s bank %s: %s is absent from the map", p.name, b.ID, f)
				}
			}
		}
	}
}

// TestBankFields_MatrixGrades is §2.1's TS-480 column: FIVE graded fields and
// twenty-two zeroed, with the six zeroes that are NOT family facts each
// carrying its own distinct reason.
func TestBankFields_MatrixGrades(t *testing.T) {
	graded := map[spec.Field]bool{
		spec.FieldFrequency: true, // P4 at 7-17 (480:957)
		spec.FieldMode:      true, // P5 at 18 via MD (480:959)
		spec.FieldTag:       true, // P16 at 42-49 (480:984)
		// BYTE 19 HERE, NOT BYTE 41: "Lockout status. 0: Lockout OFF,
		// 1: Lockout ON" (480:962), where the 590 pair carry their
		// lockout at byte 41 and this radio prints a constant there
		// (480:982). The two radios swap the bytes (§5).
		spec.FieldScanSkip: true,
		spec.FieldToneMode: true, // P7 at 20, three values (480:964)
	}
	for _, p := range profiles {
		caps := p.caps()
		for _, b := range caps.Banks {
			for _, f := range allSpecFields {
				want := spec.FieldSupport{}
				if graded[f] {
					want = p.rw
				}
				if got := b.Fields[f]; got != want {
					t.Errorf("%s bank %s: %s = %+v, want %+v (§2.1)", p.name, b.ID, f, got, want)
				}
			}
		}
	}
}

// TestBankFields_TheSixZeroesThatAreNotFamilyFacts is the plan's own list,
// and each of the six has a DISTINCT reason that a single "not in the
// record" would flatten. It is a documentation pin as much as a value one:
// the reasons live in unexpressedFields, which the fleet audit walks, and
// this test is what stops one of the six being folded into another's wording.
func TestBankFields_TheSixZeroesThatAreNotFamilyFacts(t *testing.T) {
	reasons := unexpressedFields()
	for field, wantSubstring := range map[spec.Field]string{
		// M-E2: 90-99 are ordinary memories that also answer a second
		// frame, and no bank split can separate them.
		spec.FieldTxFrequency: "M-E2",
		// Q2: the TN chart is not in this book.
		spec.FieldToneTx: "Q2",
		// Q2 again: the CN chart is not in this book either.
		spec.FieldToneRx: "Q2",
		// The absent byte-19 data flag: byte 19 is the LOCKOUT here.
		spec.FieldDataMode: "480:962",
		// A22's refusal: present at 39-40 and refused.
		spec.FieldTuningStep: "A22",
		// The hard-wired byte 28.
		spec.FieldFilter: "480:973",
	} {
		got, ok := reasons[field]
		if !ok {
			t.Errorf("%s carries no recorded reason for its zero grade", field)
			continue
		}
		if !strings.Contains(got, wantSubstring) {
			t.Errorf("%s's reason %q does not name %q", field, got, wantSubstring)
		}
	}
	// The six reasons must be six DIFFERENT sentences: a reader who found
	// the same words under tx_frequency and tuning_step would learn nothing
	// about why either is zero.
	seen := map[string]spec.Field{}
	for _, f := range []spec.Field{spec.FieldTxFrequency, spec.FieldToneTx, spec.FieldToneRx, spec.FieldDataMode, spec.FieldTuningStep, spec.FieldFilter} {
		r := reasons[f]
		if prev, dup := seen[r]; dup {
			t.Errorf("%s and %s carry the SAME reason %q; the plan requires each its own", prev, f, r)
		}
		seen[r] = f
	}
}

// TestFieldAudit_CoversEverySpecField consumes the fleet helper as a BLACK
// BOX (plan P5): the audited list is this driver's own hand-written one, and
// every other field carries a written reason.
func TestFieldAudit_CoversEverySpecField(t *testing.T) {
	drivertest.AssertFieldAuditCoversEverySpecField(t, "auditedFields()", auditedFields(), unexpressedFields())
}

// auditedFields is the hand-written list of the fields THIS ROW's record
// expresses — the list a Field addition must be added to. FIVE, against the
// 590 pair's nine or ten.
func auditedFields() []spec.Field {
	return []spec.Field{
		spec.FieldFrequency, spec.FieldMode, spec.FieldTag,
		spec.FieldScanSkip, spec.FieldToneMode,
	}
}

// unexpressedFields is the other half of the audit: every field this row
// grades the zero FieldSupport, with the matrix's own reason.
func unexpressedFields() map[spec.Field]string {
	return map[spec.Field]string{
		spec.FieldClarifier:         "§1.7, M-E5: the 50-byte record has NO clarifier position over this book's own complete 47-byte account (480:951-984); the TS-480 DOES have RIT and XIT, as radio-level settings no memory channel stores — RC \"Clears the RIT offset frequency\" (480:1205). NOT decision 6, which constrains exactly one vocabulary pair and does not name this field",
		spec.FieldCTCSSState:        "§2.1: this record expresses tone as P7's mode selector, which is FieldToneMode",
		spec.FieldCTCSSTone:         "§2.1: the record carries TWO independent tone indices (P8, P9) and FieldCTCSSTone is ONE field",
		spec.FieldShift:             "§1.18: no shift selector; split is two frames",
		spec.FieldTagDisplay:        "§2.1: no tag-display flag anywhere in the record",
		spec.FieldErase:             "§2.8: this radio has NO erase route at all — the only \"clear\" in the whole book is RC, \"Clears the RIT offset frequency\" (480:1205). The 590 pair's short-MW ambiguity (A5) does not arise here",
		spec.FieldTxFrequency:       "§2.4, M-E2: a split channel is two frames over one number (480:951), but on THIS row channels 90-99 are ordinary memories that ALSO answer a second frame (480:943-944, 480:986-987) and a Bank.Fields map is per bank rather than per slot, so no bank split can grade the field for 00-89 and not for 90-99. Unsupported on the WHOLE row, and the P1=1 half of 90-99 is unreachable through this programme",
		spec.FieldDuplex:            "§1.18: no duplex selector in the record",
		spec.FieldOffset:            "§1.18: no per-channel offset magnitude anywhere in the record",
		spec.FieldToneTx:            "§1.9, Q2: P8 is a printed index (480:966) whose CHART is not in this book — \"Refer to page 32 of the TS-480 instruction manual\" (480:1559-1560) — so this programme cannot say what any index means in hertz, and borrowing the TS-590's table across a model boundary is the cross-model inference this milestone refuses",
		spec.FieldToneRx:            "§1.9, Q2: P9's chart is likewise absent — \"Refer to page 33 of the TS-480 instruction manual\" (480:339-340). The RANGES are printed (00 ~ 42 and 00 ~ 41, 480:1557, 480:337) and the MAPPING is not",
		spec.FieldDTCSCode:          "§1.21: DCS appears nowhere in either book",
		spec.FieldDTCSPolarity:      "§1.20: as FieldDTCSCode",
		spec.FieldFilter:            "§1.22: byte 28 is \"Always 0 for the TS-480.\" (480:973) — one of this row's sixteen printed-fixed parameter bytes. There is no per-channel filter selection to publish, where the TS-590SG's byte 28 is a live FILTER A/B selector",
		spec.FieldDataMode:          "§2.1: byte 19 is the channel LOCKOUT on this radio (480:962), not the 590 pair's data-mode flag (590:1546-1548). There is no data-mode position anywhere in this record — the divergence with no roadmap line (§5)",
		spec.FieldTuningStepEnabled: "§2.1: no on/off flag for a step exists in the record",
		spec.FieldTuningStep:        "§1.23, A22: bytes 39-40 ARE a step here — \"Step size. Refer to the ST command.\" (480:979) — so this zero is a REFUSAL rather than an absence. ST's legend is mode-conditional over two different ranges, 00 ~ 04 for SSB/CW/FSK and 00 ~ 09 for AM/FM with index 00 meaning 0.5 kHz in the first and 5 kHz in the second (480:1494-1500), and Capabilities.TuningSteps is a flat label list with no mode axis, so no honest vocabulary exists",
		spec.FieldProgramTuningStep: "§1.24: the 480's step is an INDEX into ST, not a magnitude in hertz",
		spec.FieldAttenuator:        "§1.25: a radio-level function (480:1185) with no record position",
		spec.FieldPreamp:            "§1.26: a radio-level function (480:1055-1058) with no record position",
		spec.FieldAntenna:           "§1.27: no antenna-select position in the record; TY P2's AT-equipped variant (480:1627) is a radio-level fact for the probe note",
		spec.FieldIPPlus:            "§2.1: an Icom concept with no position in this frame and no mention in either book",
	}
}

// TestWriteTrialsComplete_PinnedFalse is P22's TWO-PART pin: the constant is
// false, AND the RealHardware baseline it travels with is genuinely
// nothing-writable — so a constant-only edit cannot pass while leaving the
// consequence untested.
func TestWriteTrialsComplete_PinnedFalse(t *testing.T) {
	if writeTrialsComplete {
		t.Error("writeTrialsComplete is true; no TS-480 has ever been written to by this project (matrix §3.12)")
	}
	for _, b := range CapabilitiesUnverified().Banks {
		for _, f := range allSpecFields {
			if b.Fields[f].CanWrite() {
				t.Errorf("RealHardware bank %s grades %s writable while writeTrialsComplete is false", b.ID, f)
			}
		}
	}
}

// TestNoProductionFileNamesTheSharedToneChart is P6's production half, and on
// this row it is a stronger statement than on the 590 pair: they publish a
// 43-entry chart transcribed from their own book, while this row publishes
// NONE, so any appearance of the project's shared Yaesu chart here would be a
// tone domain invented from nothing.
func TestNoProductionFileNamesTheSharedToneChart(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading this package's directory: %v", err)
	}
	seen := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		seen++
		b, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		if strings.Contains(string(b), "StandardCTCSSTones") {
			t.Errorf("%s names spec.StandardCTCSSTones; the Kenwood chart is the project's shared chart minus eight interstitial tones (§1.9, P6), and this row publishes no tone domain at all", name)
		}
	}
	if seen == 0 {
		t.Fatal("no non-test .go file was read; the walk is broken and this check passed vacuously")
	}
}

// TestCapabilities_ASessionHandsOutDefensiveCopies is the T11 review's M2 one
// row over: spec.Capabilities.Clone's claim is load-bearing for the write gate, and
// it is asserted THROUGH AN OPENED SESSION rather than against a freshly
// constructed set — which would be trivially true whatever the function did,
// since baseCapabilities allocates on every call.
func TestCapabilities_ASessionHandsOutDefensiveCopies(t *testing.T) {
	sess, _ := openSession(t, Simulated, radioImage{})
	handed := sess.Capabilities()
	if len(handed.Banks) == 0 || len(handed.Banks[0].Slots) == 0 {
		t.Fatal("the session published no banks to mutate")
	}
	handed.Banks[0].Slots[0] = "MUTATED"
	handed.Banks[0].Fields[spec.FieldFrequency] = spec.FieldSupport{}
	handed.Modes[0] = "MUTATED"

	again := sess.Capabilities()
	if again.Banks[0].Slots[0] == "MUTATED" {
		t.Error("a caller's mutation of a handed-out bank slot reached the session's own capability set")
	}
	if again.Banks[0].Fields[spec.FieldFrequency] == (spec.FieldSupport{}) {
		t.Error("a caller's mutation of a handed-out field map reached the session's own capability set")
	}
	if again.Modes[0] == "MUTATED" {
		t.Error("a caller's mutation of a handed-out mode list reached the session's own capability set")
	}
}

// TestCapabilities_ZeroFrequencyBoundsDisableTheCheck is M-E6's positive
// proof: 0/0 is a DISABLED check rather than a fabricated bound, so no
// frequency is falsely refused — while a frequency the eleven-digit field
// cannot hold is still refused by the CODEC, whose message names the field
// width and not any radio's tuning range (A17).
func TestCapabilities_ZeroFrequencyBoundsDisableTheCheck(t *testing.T) {
	caps := CapabilitiesUnverified()
	for _, hz := range []uint64{1, 99_000_000_000} {
		cp := &codeplug.Codeplug{
			Radio:    codeplug.RadioInfo{Model: caps.Model, CATID: caps.CATID},
			Channels: []codeplug.Channel{{Slot: "00", Data: &codeplug.ChannelData{FreqHz: hz, Mode: "FM"}}},
		}
		for _, issue := range codeplug.Validate(cp, caps) {
			if issue.Field == spec.FieldFrequency {
				t.Errorf("at %d Hz: %s — a zero Min/MaxFreqHz must DISABLE the bound, not enforce one", hz, issue.Msg)
			}
		}
	}

	// The one frequency guard this milestone ships is the codec's, and it is
	// about the WIRE: MR/MW P4 is eleven digits (480:957).
	l := layout()
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

// TestNew_ModelAndCATID pins the registry key and the three-digit identity
// (§1.1, §1.2), and that Capabilities().Model equals the key, which
// core/driver.Driver's contract requires.
func TestNew_ModelAndCATID(t *testing.T) {
	d := New(RealHardware)
	if got := d.Model(); got != modelName {
		t.Errorf("Model() = %q, want %q", got, modelName)
	}
	if got := d.Capabilities().Model; got != modelName {
		t.Errorf("Capabilities().Model = %q, want %q (core/driver.Driver's contract)", got, modelName)
	}
	if got := d.Capabilities().CATID; got != catID {
		t.Errorf("Capabilities().CATID = %q, want %q", got, catID)
	}
}

// TestDriver_ProfileSelection pins the fail-safe direction: RealHardware —
// the ZERO Profile — and ANY unrecognised value select the all-Unverified
// set, never the simulator's (matrix §2.1).
func TestDriver_ProfileSelection(t *testing.T) {
	var zero Profile
	if zero != RealHardware {
		t.Fatalf("the zero Profile is %v, want RealHardware (matrix §2.1)", zero)
	}
	for _, profile := range []Profile{RealHardware, Profile(7), Profile(-1)} {
		if got := New(profile).Capabilities(); !reflect.DeepEqual(got, CapabilitiesUnverified()) {
			t.Errorf("profile %v does not select CapabilitiesUnverified", profile)
		}
	}
	if got := New(Simulated).Capabilities(); !reflect.DeepEqual(got, CapabilitiesSimulated()) {
		t.Error("Simulated does not select CapabilitiesSimulated")
	}
}

// TestConsent_TransformsTheSessionSetOnly: consent is a statement about a
// SESSION, never about the radio, so the static profile is untouched and an
// unrecognised profile stays on the untransformed fail-safe even WITH the
// option.
func TestConsent_TransformsTheSessionSetOnly(t *testing.T) {
	d := New(RealHardware, WithConsentedUnverifiedWrites())
	if !reflect.DeepEqual(d.Capabilities(), CapabilitiesUnverified()) {
		t.Error("consent changed the driver's STATIC capability set; it must change only the session's")
	}
	sess, _ := openSession(t, RealHardware, radioImage{}, WithConsentedUnverifiedWrites())
	if !sess.Capabilities().FieldSupport(spec.BankMemory, spec.FieldFrequency).CanWrite() {
		t.Error("a consented session's frequency field is still unwritable; the consent transform did not run")
	}

	unrecognised := &ts480Driver{Base: driver.Base{Profile: Profile(9), Consented: true}}
	if unrecognised.SessionCaps(unrecognised.Capabilities()).FieldSupport(spec.BankMemory, spec.FieldFrequency).CanWrite() {
		t.Error("consent widened an UNRECOGNISED profile; the fail-safe direction must survive consent")
	}
}

// TestStopBits_IsOneOnTheConcreteDriverValue is P10 leg 1: a POSITIVE
// assertion on the concrete driver value, never a type switch that would pass
// when the interface is absent.
//
// It is a SESSION PRECONDITION, not a nicety: transport.DefaultStopBits is 2
// (the Yaesu family's framing) and this book specifies one — "Each data is
// constructed with 1 start bit, 8 data bits, and 1 stop bit (4800 bps must be
// configured as 2 stop bits). No parity is used." (480:22-23). A driver that
// omitted the interface would open every session at the wrong framing and
// fail like a dead port.
//
// P10 leg 2 — that internal/wiring's stopBitsFor answers 1 for this row too —
// is T18's and cannot be written here: stopBitsFor is unexported and lives in
// another package.
func TestStopBits_IsOneOnTheConcreteDriverValue(t *testing.T) {
	d := New(RealHardware)
	r, ok := d.(driver.SerialFramingReporter)
	if !ok {
		t.Fatal("the driver does not implement driver.SerialFramingReporter")
	}
	if got := r.StopBits(); got != 1 {
		t.Errorf("StopBits() = %d, want 1 (§3.1, 480:22-23)", got)
	}
}

// TestSession_ImplementsTheOptionalCapabilities keeps the compile-time
// assertions honest at run time too: a driver that lost an optional interface
// would still build, and internal/wiring type-asserts for each. The settings
// pair is asserted in settings_test.go, beside the methods that satisfy it.
func TestSession_ImplementsTheOptionalCapabilities(t *testing.T) {
	sess, _ := openSession(t, Simulated, radioImage{})
	if _, ok := any(sess).(driver.DiagnosticsReporter); !ok {
		t.Error("*Session does not implement driver.DiagnosticsReporter")
	}
}
