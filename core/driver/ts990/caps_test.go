// SPDX-License-Identifier: GPL-3.0-or-later

package ts990

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
	"github.com/gm5dna/open-rig-programmer/core/kw/ma"
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

// TestCapabilities_ValidateOnEveryProfile is the floor: a capability set that
// does not validate cannot be registered at all.
func TestCapabilities_ValidateOnEveryProfile(t *testing.T) {
	for name, caps := range map[string]spec.Capabilities{
		"unverified": CapabilitiesUnverified(),
		"simulated":  CapabilitiesSimulated(),
	} {
		if err := caps.Validate(); err != nil {
			t.Errorf("%s: Validate: %v", name, err)
		}
	}
}

// TestCapabilities_MatrixValues walks the A4 matrix's §1 for the TS-990S
// column. EVERY VALUE HERE IS THE MATRIX'S, cited by section, and a value
// that is not derivable from it is a STOP rather than a judgement call.
//
// NOT ONE VALUE IS COPIED FROM core/driver/ts890 (plan, Task 13's own
// preamble): each is transcribed from this book's own printed chart, and
// where the sibling's happens to agree the agreement is a coincidence of two
// readings rather than a shared source.
func TestCapabilities_MatrixValues(t *testing.T) {
	// §1.9 — the 51 TN entries in decihertz, index = the CAT tone number,
	// transcribed from 990:4960-4972.
	wantTones := []spec.Tone{
		670, 693, 719, 744, 770, 797, 825, 854, 885, 915, 948, 974, 1000,
		1035, 1072, 1109, 1148, 1188, 1230, 1273, 1318, 1365, 1413, 1462, 1514, 1567,
		1598, 1622, 1655, 1679, 1713, 1738, 1773, 1799, 1835, 1862, 1899, 1928, 1966,
		1995, 2035, 2065, 2107, 2181, 2257, 2291, 2336, 2418, 2503, 2541, 17500,
	}
	// §1.5 — twenty-six published names: the twenty-two live legend values
	// plus the four synthesised narrow twins, in wire-byte order.
	wantModes := []string{
		"LSB", "USB", "CW", "FM", "FM-N", "AM", "FSK", "CW-R", "FSK-R",
		"PSK", "PSK-R",
		"LSB-D1", "USB-D1", "FM-D1", "FM-D1-N", "AM-D1",
		"LSB-D2", "USB-D2", "FM-D2", "FM-D2-N", "AM-D2",
		"LSB-D3", "USB-D3", "FM-D3", "FM-D3-N", "AM-D3",
	}
	caps := CapabilitiesUnverified()

	if caps.Model != "TS-990S" {
		t.Errorf("Model = %q, want %q (§1.1)", caps.Model, "TS-990S")
	}
	if caps.CATID != "022" {
		t.Errorf("CATID = %q, want %q (§1.2, 990:2612)", caps.CATID, "022")
	}
	if caps.Transmit != spec.HasTransmitter {
		t.Errorf("Transmit = %v, want HasTransmitter (§1.3)", caps.Transmit)
	}
	if !reflect.DeepEqual(caps.Modes, wantModes) {
		t.Errorf("Modes =\n %v\nwant\n %v\n(§1.5)", caps.Modes, wantModes)
	}
	if caps.TagLen != 10 {
		t.Errorf("TagLen = %d, want 10 (§1.6, 990:2955-2956)", caps.TagLen)
	}
	if caps.ClarMaxHz != 0 || caps.ClarStepHz != 0 {
		t.Errorf("Clar bounds = %d/%d, want 0/0 (§1.7, §1.8)", caps.ClarMaxHz, caps.ClarStepHz)
	}
	if !reflect.DeepEqual(caps.CTCSSTones, wantTones) {
		t.Errorf("CTCSSTones =\n %v\nwant\n %v\n(§1.9)", caps.CTCSSTones, wantTones)
	}
	if caps.CTCSSToneRange != nil {
		t.Errorf("CTCSSToneRange = %v, want nil (§1.10)", caps.CTCSSToneRange)
	}
	if want := []int{9600, 19200, 38400, 57600, 115200}; !reflect.DeepEqual(caps.Bauds, want) {
		t.Errorf("Bauds = %v, want %v (§1.11)", caps.Bauds, want)
	}
	if caps.DefaultBaud != 9600 {
		t.Errorf("DefaultBaud = %d, want 9600 (§1.12, A11)", caps.DefaultBaud)
	}
	if caps.MinFreqHz != 0 || caps.MaxFreqHz != 0 {
		t.Errorf("frequency bounds = %d/%d, want 0/0 (§1.13, §1.14, A15)", caps.MinFreqHz, caps.MaxFreqHz)
	}
	if caps.RequiredSlots != nil {
		t.Errorf("RequiredSlots = %v, want nil (§1.15)", caps.RequiredSlots)
	}
	for name, got := range map[string]int{
		"ShiftOptions":   len(caps.ShiftOptions),
		"DuplexOptions":  len(caps.DuplexOptions),
		"DTCSPolarities": len(caps.DTCSPolarities),
		"DTCSCodes":      len(caps.DTCSCodes),
		"Filters":        len(caps.Filters),
		"TuningSteps":    len(caps.TuningSteps),
		"AttenuatorDB":   len(caps.AttenuatorDB),
		"PreampOptions":  len(caps.PreampOptions),
		"AntennaOptions": len(caps.AntennaOptions),
	} {
		if got != 0 {
			t.Errorf("%s has %d entries, want none (§1.16-§1.27)", name, got)
		}
	}
	if caps.ProgramTuningStepRange != nil {
		t.Errorf("ProgramTuningStepRange = %v, want nil (§1.24)", caps.ProgramTuningStepRange)
	}
	wantToneModes := []spec.ToneMode{
		{Value: "OFF", Semantics: spec.ToneModeOff},
		{Value: "TONE", Semantics: spec.ToneModeCTCSS},
		{Value: "CTCSS", Semantics: spec.ToneModeCTCSSRxSquelch},
		{Value: "CROSS", Semantics: spec.ToneModeCross},
	}
	if !reflect.DeepEqual(caps.ToneModes, wantToneModes) {
		t.Errorf("ToneModes = %v, want %v (§1.19, K-D1)", caps.ToneModes, wantToneModes)
	}
	if caps.TagCharset != "" {
		t.Errorf("TagCharset = %q, want the family default (§1.28, A2)", caps.TagCharset)
	}
}

// TestCapabilities_EveryFieldExplicit reflects over the struct so that a
// zero left in a POPULATED field cannot pass as an omission: a zero
// MaxFreqHz reads as "no ceiling" to core/codeplug's validator, a zero
// TagLen makes core/csvio's CHIRP import truncate every imported name to "",
// and an empty vocabulary a bank reaches fails spec.Validate outright.
func TestCapabilities_EveryFieldExplicit(t *testing.T) {
	// The eighteen the matrix records as deliberately EMPTY on this row,
	// each with the section that says so. Filters is here and is NOT
	// per-row as it is on the 590 pair: neither MA0 grid has a filter
	// position (§1.22, M-E5).
	deliberatelyEmpty := map[string]string{
		"ClarMaxHz":              "§1.7, M-E5 — no clarifier position in the eighteen-parameter grid",
		"ClarStepHz":             "§1.8, M-E5 — as ClarMaxHz",
		"CTCSSToneRange":         "§1.10 — the tone field is an INDEX into a printed chart",
		"MinFreqHz":              "§1.13, A15 — no frequency range is printed in this book",
		"MaxFreqHz":              "§1.14, A15 — as MinFreqHz; a zero DISABLES the ceiling check",
		"RequiredSlots":          "§1.15 — this book marks no channel mandatory",
		"ShiftOptions":           "§1.16 — the Yaesu half of the vocabulary pair (decision 6)",
		"DuplexOptions":          "§1.18 — zero occurrences of \"duplex\" in this book",
		"DTCSPolarities":         "§1.20 — DCS appears nowhere in this book",
		"DTCSCodes":              "§1.21 — as DTCSPolarities",
		"Filters":                "§1.22, M-E5 — no filter byte in this grid; FL0-FL3 are radio-level",
		"TuningSteps":            "§1.23 — no step field, and this radio has no ST command at all",
		"ProgramTuningStepRange": "§1.24 — no step magnitude in hertz in the record",
		"AttenuatorDB":           "§1.25, M-E5 — RA is radio-level, with no record position",
		"PreampOptions":          "§1.26, M-E5 — as AttenuatorDB, for PA",
		"AntennaOptions":         "§1.27, M-E5 — as AttenuatorDB, for AN0/AN1",
		"TagCharset":             "§1.28, A2 — the empty string selects the family default",
		"NoTag":                  "the TS-990S supports channel names via the MT tag field; NoTag is false",
	}
	caps := CapabilitiesUnverified()
	v := reflect.ValueOf(caps)
	typ := v.Type()
	if typ.NumField() != 29 {
		t.Fatalf("spec.Capabilities has %d fields, want 29 — this test's list is stale", typ.NumField())
	}
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		zero := v.Field(i).IsZero()
		reason, expectedEmpty := deliberatelyEmpty[name]
		switch {
		case zero && !expectedEmpty:
			t.Errorf("%s is the zero value and is not on the deliberately-empty list", name)
		case !zero && expectedEmpty:
			t.Errorf("%s is populated but the deliberately-empty list says %q", name, reason)
		}
	}
}

// TestModes_TheModeByteIsNotAHexNibbleOnThisRow is §1.5's divergence 3 as a
// pin rather than as prose: 'I' is FM-D2 here (990:3725) and names no mode
// the TS-890S has, whose legend stops at 'F' (890:3976-3992). A codec or a
// capability list built from a hex nibble would lose two thirds of this
// radio's data modes.
func TestModes_TheModeByteIsNotAHexNibbleOnThisRow(t *testing.T) {
	name, ok := layout().ModeName('I')
	if !ok || name != "FM-D2" {
		t.Errorf("this row's ModeName('I') = %q/%v, want \"FM-D2\"/true (990:3725)", name, ok)
	}
	if _, ok := ma.Layout890().ModeName('I'); ok {
		t.Error("the TS-890S names a mode for 'I'; this pin exists because its legend is sixteen values and stops at 'F' (890:3976-3992)")
	}
	if !slices.Contains(CapabilitiesUnverified().Modes, "FM-D2") {
		t.Error("Modes omits FM-D2, which this row's legend prints (§1.5)")
	}
}

// TestCTCSSTones_AreNeitherSharedChart is plan P6's inequality pin. This
// row's chart is 51 entries; the project's shared Yaesu chart is 50 and pair
// 1's Kenwood chart is 43, so neither can stand in for it — and the fact
// that this chart's first fifty entries COINCIDE with the shared one (§1.9)
// is exactly why the inequality has to be asserted rather than assumed.
func TestCTCSSTones_AreNeitherSharedChart(t *testing.T) {
	tones := CapabilitiesUnverified().CTCSSTones
	if len(tones) != 51 {
		t.Fatalf("this row's chart has %d entries, want 51 — TN 00-50 (990:4960-4972)", len(tones))
	}
	if shared := spec.StandardCTCSSTones(); len(tones) == len(shared) {
		t.Errorf("this row's chart is %d entries and the project's shared chart is %d; they must differ (P6, §1.9)", len(tones), len(shared))
	}
	if len(tones) == 43 {
		t.Error("this row's chart is 43 entries, which is pair 1's TS-590 chart; this pair's charts are eight entries longer (P6, §1.9)")
	}
	// 159.8 Hz at index 26 is one of the eight interstitial tones pair 1's
	// 43-entry chart does not carry, so its presence is what makes the
	// domain per row rather than per manufacturer (§1.9).
	if tones[26] != 1598 {
		t.Errorf("index 26 is %v, want 159.8 Hz (990:4960) — one of the eight tones pair 1's chart drops", tones[26])
	}
	// Index 50 is 1750 Hz, which TN prints and CN does not (990:4971).
	if tones[50] != 17500 {
		t.Errorf("index 50 is %v, want 1750 Hz (990:4971)", tones[50])
	}
}

// TestNoProductionFileNamesTheSharedToneChart holds P6's production half
// down: this row's chart is a reading of this radio's own printed table, and
// a coincidence of contents is not a licence to alias.
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
			t.Errorf("%s names spec.StandardCTCSSTones (P6, §1.9)", name)
		}
	}
}

// TestBanks_OneMemoryBankAndNothingAbove099 is plan P11 and M-E1. The
// NEGATIVE is the half that carries the design: no slot string in
// "100".."119" appears anywhere in Capabilities().
func TestBanks_OneMemoryBankAndNothingAbove099(t *testing.T) {
	caps := CapabilitiesUnverified()
	if len(caps.Banks) != 1 {
		t.Fatalf("this row publishes %d banks, want exactly one (§1.4, §1.4.5)", len(caps.Banks))
	}
	b := caps.Banks[0]
	if b.ID != spec.BankMemory {
		t.Errorf("bank ID = %q, want %q (§1.4)", b.ID, spec.BankMemory)
	}
	if b.Label != "Memories" {
		t.Errorf("bank label = %q, want %q (§1.4.1, a CHOICE)", b.Label, "Memories")
	}
	if b.NoBlank {
		t.Error("NoBlank is true; this book documents the empty channel as a normal state (990:2962-2963) and prints MA5 (990:3042-3047) — §2.5")
	}
	if b.Sparse {
		t.Error("Sparse is true; this is a small dense space fully printed in the book (§1.4)")
	}
	if len(b.Slots) != 100 {
		t.Fatalf("MEM has %d slots, want 100 (§1.4.1, 990:2894)", len(b.Slots))
	}
	if b.Slots[0] != "000" || b.Slots[99] != "099" {
		t.Errorf("MEM runs %q..%q, want \"000\"..\"099\" (§1.4.1)", b.Slots[0], b.Slots[99])
	}
	// The negative: the Programmable-VFO/section-defined class (100-109)
	// and the E channels (110-119) are published NOWHERE (§1.4.2, §1.4.3).
	for n := 100; n <= 119; n++ {
		id := string(rune('0'+n/100)) + string(rune('0'+n/10%10)) + string(rune('0'+n%10))
		if slices.Contains(b.Slots, id) {
			t.Errorf("slot %q is published; 100-119 appear in no bank of this row (§1.4.2, §1.4.3, P11)", id)
		}
	}
}

// TestBankFields_NameEveryOneOfTheTwentySeven is the whole of matrix §2.1's
// TS-990S column, including the nineteen written-down zeroes: a field left
// OUT of the map reads identically to a field deliberately zeroed, and only
// a written-down zero is legible as a decision.
func TestBankFields_NameEveryOneOfTheTwentySeven(t *testing.T) {
	graded := map[spec.Field]bool{
		spec.FieldFrequency:   true,
		spec.FieldMode:        true,
		spec.FieldTag:         true,
		spec.FieldScanSkip:    true,
		spec.FieldToneMode:    true,
		spec.FieldToneTx:      true,
		spec.FieldToneRx:      true,
		spec.FieldTxFrequency: true,
	}
	if len(graded) != 8 {
		t.Fatalf("the graded list has %d entries, want 8 (§2.1)", len(graded))
	}
	for _, profile := range []struct {
		name string
		caps spec.Capabilities
		rw   spec.FieldSupport
	}{
		{"unverified", CapabilitiesUnverified(), spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}},
		{"simulated", CapabilitiesSimulated(), spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}},
	} {
		fields := profile.caps.Banks[0].Fields
		if len(fields) != 27 {
			t.Errorf("%s: the bank map has %d entries, want all 27 written down (§2.1)", profile.name, len(fields))
		}
		for _, f := range allSpecFields {
			want := spec.FieldSupport{}
			if graded[f] {
				want = profile.rw
			}
			got, ok := fields[f]
			if !ok {
				t.Errorf("%s: %s is absent from the bank map; an absent field and a zeroed one read the same (§2.1)", profile.name, f)
				continue
			}
			if got != want {
				t.Errorf("%s: %s = %+v, want %+v (§2.1)", profile.name, f, got, want)
			}
		}
	}
}

// TestBankFields_DataModeAndFilterAndEraseAreZero pins the three zeroes a
// reader coming from pair 1 will expect to be graded, because each is a
// ruling rather than an absence a glance would confirm.
func TestBankFields_DataModeAndFilterAndEraseAreZero(t *testing.T) {
	fields := CapabilitiesSimulated().Banks[0].Fields
	for f, why := range map[spec.Field]string{
		spec.FieldDataMode: "M-E3: there is no DA command and no data byte; the data-ness is in the mode NAMES (990:3719-3730)",
		spec.FieldFilter:   "§1.22: no filter byte in this grid, unlike the 590SG's byte 28",
		spec.FieldErase:    "M-E4: the standing no-erase rule declines even the printed MA5 (990:3042-3047)",
	} {
		if got := fields[f]; got != (spec.FieldSupport{}) {
			t.Errorf("%s = %+v, want the zero FieldSupport — %s", f, got, why)
		}
	}
}

// TestFieldAudit_CoversEverySpecField consumes the fleet helper as a BLACK
// BOX (plan P5): the audited list is this driver's own hand-written one and
// every other field carries a written reason.
func TestFieldAudit_CoversEverySpecField(t *testing.T) {
	drivertest.AssertFieldAuditCoversEverySpecField(t, "auditedFields", auditedFields(), unexpressedFields())
}

// TestAllSpecFields_MatchesTheAuditedAndUnexpressedFields anchors
// allSpecFields to the real field list transitively, without naming
// spec.AllFields in this package (P5).
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

// auditedFields is the hand-written list of the fields THIS ROW's record
// expresses — the list a Field addition must be added to.
func auditedFields() []spec.Field {
	return []spec.Field{
		spec.FieldFrequency, spec.FieldMode, spec.FieldTag,
		spec.FieldScanSkip, spec.FieldTxFrequency, spec.FieldToneMode,
		spec.FieldToneTx, spec.FieldToneRx,
	}
}

// unexpressedFields is the other half of the audit: every field this row
// grades the zero FieldSupport, with the matrix's own reason.
func unexpressedFields() map[spec.Field]string {
	return map[spec.Field]string{
		spec.FieldClarifier:         "§1.7, M-E5: the eighteen-parameter grid has no clarifier position; the radio DOES have RIT and XIT (990:4252, 990:5210)",
		spec.FieldCTCSSState:        "§2.1: this record expresses tone as P6's mode selector, which is FieldToneMode",
		spec.FieldCTCSSTone:         "§2.1: the record carries TWO independent tone indices (P7, P8) and FieldCTCSSTone is ONE field",
		spec.FieldShift:             "§1.18: no shift selector; split is an absolute second frequency plus P15",
		spec.FieldTagDisplay:        "§2.1: no tag-display flag anywhere in the grid",
		spec.FieldErase:             "§2.8, M-E4: MA5 is printed (990:3042-3047) and the standing no-erase rule declines to build it",
		spec.FieldDuplex:            "§1.18: zero occurrences of \"duplex\" in this book",
		spec.FieldOffset:            "§1.18: no per-channel offset magnitude in the grid",
		spec.FieldDTCSCode:          "§1.21: DCS appears nowhere in this book",
		spec.FieldDTCSPolarity:      "§1.20: as FieldDTCSCode",
		spec.FieldFilter:            "§1.22, M-E5: no filter byte in the grid; FL0-FL3 are radio-level (990:2411, 990:2432, 990:2467, 990:2488)",
		spec.FieldDataMode:          "M-E3: no DA command and no data byte — the data modes are inside the mode legend (990:3719-3730)",
		spec.FieldTuningStepEnabled: "§1.23: no on/off flag for a step, and this radio has no ST command at all",
		spec.FieldTuningStep:        "§1.23: as FieldTuningStepEnabled",
		spec.FieldProgramTuningStep: "§1.24: no step magnitude in hertz in the record",
		spec.FieldAttenuator:        "§1.25, M-E5: RA is radio-level (990:4094) with no record position",
		spec.FieldPreamp:            "§1.26, M-E5: PA is radio-level (990:3734) with no record position",
		spec.FieldAntenna:           "§1.27, M-E5: AN0/AN1 are radio-level (990:211, 990:240) and this radio NAMES its antennas",
		spec.FieldIPPlus:            "§2.1: an Icom concept with no position in this frame and no mention in this book",
	}
}

// TestWriteTrialsComplete_PinnedFalse is P18's TWO-PART pin: the constant is
// false, AND the RealHardware baseline it travels with is genuinely
// nothing-writable — so a constant-only edit cannot pass while leaving the
// consequence untested.
func TestWriteTrialsComplete_PinnedFalse(t *testing.T) {
	if writeTrialsComplete {
		t.Error("writeTrialsComplete is true; no TS-990S has been written to by this project (§3.12)")
	}
	for _, f := range allSpecFields {
		if CapabilitiesUnverified().FieldSupport(spec.BankMemory, f).CanWrite() {
			t.Errorf("the RealHardware baseline makes %s writable while writeTrialsComplete is false (P18)", f)
		}
	}
}

// TestBauds_4800IsAbsentForBothPrintedReasons is §1.11's omission as a pin.
func TestBauds_4800IsAbsentForBothPrintedReasons(t *testing.T) {
	// 4800 is printed (990:13-14) and CONDITIONAL: "4800 bps cannot be used
	// with the USB-B connector" (990:23), which is the path this book's own
	// "Using a USB Cable" section steers a PC user to. A Bauds list is one
	// flat list per row and cannot carry a per-connector condition, so
	// publishing 4800 would promise a rate this programme cannot deliver on
	// the ordinary connection.
	for _, b := range CapabilitiesUnverified().Bauds {
		if b == 4800 {
			t.Error("Bauds publishes 4800, which this book makes conditional on the connector (990:23, §1.11)")
		}
	}
}

// TestCapabilities_SimplexTx pins the transmit disposition of a channel
// this record calls simplex, transcribed from THIS book: MA0's frequency-2
// side and P15 "0: Simplex / 1: Split" (990:2946-2948), with the printed
// answer "…all parameters for frequency 2 become 0" (990:2964-2965).
func TestCapabilities_SimplexTx(t *testing.T) {
	if got := CapabilitiesUnverified().SimplexTx; got != spec.SimplexTxZero {
		t.Errorf("SimplexTx = %v, want SimplexTxZero (990:2964-2965)", got)
	}
}
