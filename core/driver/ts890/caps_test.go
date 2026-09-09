// SPDX-License-Identifier: GPL-3.0-or-later

package ts890

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

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

// TestCapabilities_MatrixValues walks the A4 matrix's §1 field by field.
// EVERY VALUE HERE IS THE MATRIX'S, cited by section, and a value that is not
// derivable from it is a STOP rather than a judgement call.
func TestCapabilities_MatrixValues(t *testing.T) {
	caps := CapabilitiesUnverified()

	// §1.1, §1.2: ID's own legend, "024: TS-890S" (890:2733).
	if caps.Model != "TS-890S" {
		t.Errorf("Model = %q, want %q (§1.1)", caps.Model, "TS-890S")
	}
	if caps.CATID != "024" {
		t.Errorf("CATID = %q, want %q (§1.2)", caps.CATID, "024")
	}
	// The registry contract: Capabilities().Model must equal the driver
	// registry key, which is this package's own modelName const.
	if caps.Model != modelName {
		t.Errorf("Model = %q but modelName is %q", caps.Model, modelName)
	}
	// §1.3.
	if caps.Transmit != spec.HasTransmitter {
		t.Errorf("Transmit = %v, want HasTransmitter (§1.3)", caps.Transmit)
	}
	// §1.6: P13 "Up to 10 characters" (890:3208-3209) — TEN, not pair 1's
	// eight, and per row rather than per manufacturer.
	if caps.TagLen != 10 {
		t.Errorf("TagLen = %d, want 10 (§1.6)", caps.TagLen)
	}
	// §1.11: five rates, 4800 OMITTED although the book prints six.
	wantBauds := []int{9600, 19200, 38400, 57600, 115200}
	if !slices.Equal(caps.Bauds, wantBauds) {
		t.Errorf("Bauds = %v, want %v (§1.11)", caps.Bauds, wantBauds)
	}
	if slices.Contains(caps.Bauds, 4800) {
		t.Error("Bauds publishes 4800, which the book conditions on the connector (890:20) and spec.Capabilities cannot express (§1.11)")
	}
	// §1.12, A11: an OPERATIONAL assumption, and spec.Validate requires it
	// to appear in Bauds.
	if caps.DefaultBaud != 9600 {
		t.Errorf("DefaultBaud = %d, want 9600 (§1.12, A11)", caps.DefaultBaud)
	}
	// §1.19, M-E2, K-D1: four values, semantics ASSUMED.
	wantToneModes := []spec.ToneMode{
		{Value: "OFF", Semantics: spec.ToneModeOff},
		{Value: "TONE", Semantics: spec.ToneModeCTCSS},
		{Value: "CTCSS", Semantics: spec.ToneModeCTCSSRxSquelch},
		{Value: "CROSS", Semantics: spec.ToneModeCross},
	}
	if !slices.Equal(caps.ToneModes, wantToneModes) {
		t.Errorf("ToneModes = %v, want %v (§1.19)", caps.ToneModes, wantToneModes)
	}
	// §1.9: the 51-entry chart, index = the CAT tone number.
	wantTones := []spec.Tone{
		670, 693, 719, 744, 770, 797, 825, 854, 885, 915, 948, 974,
		1000, 1035, 1072, 1109, 1148, 1188, 1230, 1273, 1318, 1365,
		1413, 1462, 1514, 1567, 1598, 1622, 1655, 1679, 1713, 1738,
		1773, 1799, 1835, 1862, 1899, 1928, 1966, 1995, 2035, 2065,
		2107, 2181, 2257, 2291, 2336, 2418, 2503, 2541,
		17500,
	}
	if !slices.Equal(caps.CTCSSTones, wantTones) {
		t.Errorf("CTCSSTones = %v, want the 51 entries of 890:5149-5163 (§1.9)", caps.CTCSSTones)
	}
}

// TestCTCSSTones_AreCopiedPerCall pins that no two capability values share the
// backing array — the mistake a caller mutating what it was handed would turn
// into a silently altered tone domain everywhere.
func TestCTCSSTones_AreCopiedPerCall(t *testing.T) {
	first := CapabilitiesUnverified().CTCSSTones
	first[0] = 1
	if again := CapabilitiesUnverified().CTCSSTones; again[0] != 670 {
		t.Errorf("CTCSSTones[0] = %v after a caller mutated an earlier copy, want 67.0 Hz", again[0])
	}
	if ctcssTones890[0] != 670 {
		t.Errorf("the package chart itself was mutated: ctcssTones890[0] = %v", ctcssTones890[0])
	}
}

// TestCTCSSTones_AreNotTheProjectsSharedChart is plan P6's negative pin, and
// on this row it needs a DIFFERENT argument from pair 1's: this chart IS the
// shared 50-tone Yaesu table plus 1750 Hz, in the same order (§1.9), so an
// index-by-index inequality would be false. What is asserted instead is the
// pair of facts that make it a per-row reading and not an alias — the LENGTH
// against the shared chart, and the eight interstitial tones that make it
// eight entries longer than pair 1's 43-entry Kenwood chart.
//
// It is the ONE place in this package spec.StandardCTCSSTones() may be named,
// and TestNoProductionFileNamesTheSharedToneChart holds the production half
// down.
func TestCTCSSTones_AreNotTheProjectsSharedChart(t *testing.T) {
	shared := spec.StandardCTCSSTones()
	own := CapabilitiesUnverified().CTCSSTones
	if len(own) != 51 {
		t.Fatalf("this row's chart has %d entries, want 51 — TN runs 00-50 (890:5149-5163)", len(own))
	}
	if len(own) == len(shared) {
		t.Errorf("this row's chart and the project's shared one both have %d entries; the 51st is 1750 Hz, which no Yaesu chart carries (§1.9)", len(own))
	}
	if own[len(own)-1] != 17500 {
		t.Errorf("the last tone is %v, want 1750.0 Hz — TN index 50 (890:5162), which the CN chart has no equivalent of", own[len(own)-1])
	}
	// The eight this book prints and pair 1's 590 chart does not, which is
	// why the domain is PER ROW and not per manufacturer (§1.9). Their
	// presence is the not-equal-to-pair-1 pin, stated without importing that
	// package.
	for index, want := range map[int]spec.Tone{
		26: 1598, 28: 1655, 30: 1713, 32: 1773,
		34: 1835, 36: 1899, 38: 1966, 39: 1995,
	} {
		if own[index] != want {
			t.Errorf("index %d is %v, want %v — one of the eight interstitial tones pair 1's 43-entry chart lacks (§1.9)", index, own[index], want)
		}
	}
}

// TestNoProductionFileNamesTheSharedToneChart is P6's rule as a guard. The
// coincidence of contents makes it MORE necessary here than on pair 1, not
// less: an alias to the shared accessor would produce a chart that is right
// for fifty indices and missing 1750 Hz, which no other test in this package
// would notice.
func TestNoProductionFileNamesTheSharedToneChart(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	seen := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			continue
		}
		seen++
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", name, err)
		}
		if strings.Contains(string(b), "StandardCTCSSTones") {
			t.Errorf("%s names spec.StandardCTCSSTones; this row's chart is a reading of TN's own printed table and carries a tone no Yaesu chart has (P6, §1.9)", name)
		}
	}
	if seen == 0 {
		t.Fatal("no non-test .go file was read; the walk is broken and this check passed vacuously")
	}
}

// TestModes_SixteenNamesWithTheNarrowTwins pins §1.5 exactly: fourteen live
// legend values (890:3976-3992, '0' and '8' printed "Unused") plus the two
// SYNTHESISED narrow twins, in wire-byte order with each twin beside its base.
func TestModes_SixteenNamesWithTheNarrowTwins(t *testing.T) {
	want := []string{
		"LSB", "USB", "CW", "FM", "FM-N", "AM", "FSK", "CW-R", "FSK-R",
		"PSK", "PSK-R", "LSB-D", "USB-D", "FM-D", "FM-D-N", "AM-D",
	}
	if got := CapabilitiesUnverified().Modes; !slices.Equal(got, want) {
		t.Errorf("Modes = %v, want %v (§1.5)", got, want)
	}
}

// TestModeName_IsTheOneRuleBothPathsUse is the anti-drift pin: every name the
// read path can produce must be one Capabilities.Modes advertises, because
// codeplug.Validate checks a channel's mode against that list. A suffix
// concatenated in read.go instead of asked of this function is exactly the
// second rule this test forbids.
func TestModeName_IsTheOneRuleBothPathsUse(t *testing.T) {
	l := layout()
	published := CapabilitiesUnverified().Modes
	for b, base := range l.ModeNames() {
		for _, narrow := range []bool{false, true} {
			name, ok := modeName(l, b, narrow)
			if !ok {
				t.Errorf("modeName(%q, narrow=%v) refused a legend value the layout publishes as %q", b, narrow, base)
				continue
			}
			if !slices.Contains(published, name) {
				t.Errorf("modeName(%q, narrow=%v) = %q, which Capabilities.Modes does not carry", b, narrow, name)
			}
		}
	}
	// The width byte reaches the NAME only on an FM-bearing value; on
	// anything else the base name stands, which is kw.RecordModeName's own
	// rule one book over. '1' is LSB (890:3977-3992).
	if name, _ := modeName(l, '1', true); name != "LSB" {
		t.Errorf("modeName('1', narrow) = %q, want %q — P4 does not reach a non-FM name (§1.5)", name, "LSB")
	}
}

// TestBanks_OneMemoryBank pins plan P11 and matrix §1.4: ONE BankMemory,
// "000".."099", the label, NoBlank false and Sparse false with every sparse
// field zero.
func TestBanks_OneMemoryBank(t *testing.T) {
	caps := CapabilitiesUnverified()
	if len(caps.Banks) != 1 {
		t.Fatalf("Banks has %d entries, want 1 — there is NO scan bank on this row (§1.4.5, P11)", len(caps.Banks))
	}
	bank := caps.Banks[0]
	if bank.ID != spec.BankMemory {
		t.Errorf("bank ID = %v, want BankMemory (§1.4)", bank.ID)
	}
	if bank.Label != "Memories" {
		t.Errorf("bank Label = %q, want %q (§1.4.1)", bank.Label, "Memories")
	}
	var want []string
	for n := 0; n <= 99; n++ {
		want = append(want, fmt.Sprintf("%03d", n))
	}
	if !slices.Equal(bank.Slots, want) {
		t.Errorf("bank Slots = %v, want 000..099 (§1.4.1)", bank.Slots)
	}
	if bank.NoBlank {
		t.Error("NoBlank is true; the book documents the empty channel as a normal state (890:3215-3216) and a NoBlank MEM bank would refuse every real codeplug (§2.5)")
	}
	if bank.Sparse || bank.Groups != 0 || bank.GroupBase != 0 || bank.PerGroup != 0 ||
		bank.ChannelBase != 0 || bank.Budget != 0 || bank.BudgetUnstated {
		t.Errorf("the sparse space is populated on a dense bank: %+v (§1.4)", bank)
	}
	if caps.RequiredSlots != nil {
		t.Errorf("RequiredSlots = %v, want nil (§1.15)", caps.RequiredSlots)
	}
}

// TestBanks_NoSlotInTheUpperClassesIsPublished is P11's negative, and it is
// the pin that carries the design: 100-119 are printed (890:3168-3169) and
// published NOWHERE — not appended to MEM, not a bank of their own, not slot
// IDs anywhere in this row's capabilities. It searches the WHOLE capability
// value rather than the one bank, so a later task that added a second bank
// would trip it.
func TestBanks_NoSlotInTheUpperClassesIsPublished(t *testing.T) {
	for _, caps := range []spec.Capabilities{CapabilitiesUnverified(), CapabilitiesSimulated()} {
		published := map[string]spec.BankID{}
		for _, bank := range caps.Banks {
			for _, slot := range bank.Slots {
				if other, dup := published[slot]; dup {
					t.Errorf("slot %q appears in both %v and %v", slot, other, bank.ID)
				}
				published[slot] = bank.ID
			}
		}
		for n := 100; n <= 119; n++ {
			if id, ok := published[fmt.Sprintf("%03d", n)]; ok {
				t.Errorf("slot %03d is published in bank %v; the Programmable VFO channels are A9/A7 and the E channels A10, and neither class is published (§1.4.2, §1.4.3, P11)", n, id)
			}
		}
	}
}

// TestBankFields_NameEveryOneOfTheTwentySeven is P5's coverage rule: all
// twenty-seven spec.Field constants appear EXPLICITLY in the bank map,
// including the nineteen that are the zero FieldSupport, because a field left
// out reads identically to one deliberately zeroed (§2, §2.1).
//
// P5 REVERSES PAIR 1'S P5 and this package uses spec.AllFields() directly: a
// hand-written list beside a helper that already exists is a second copy of
// the same fact.
func TestBankFields_NameEveryOneOfTheTwentySeven(t *testing.T) {
	all := spec.AllFields()
	if len(all) != 27 {
		t.Fatalf("spec.AllFields() has %d entries, want 27", len(all))
	}
	fields := CapabilitiesUnverified().Banks[0].Fields
	for _, f := range all {
		if _, ok := fields[f]; !ok {
			t.Errorf("bankFields omits %s; an absent key and a written-down zero read identically (§2)", f)
		}
	}
	for f := range fields {
		if !slices.Contains(all, f) {
			t.Errorf("bankFields names %s, which spec.AllFields() does not", f)
		}
	}
}

// TestBankFields_EightGradedNineteenZero pins the matrix §2.1 table cell by
// cell, on both profiles, and it is the test that would catch a field graded
// from another row's book.
func TestBankFields_EightGradedNineteenZero(t *testing.T) {
	graded := []spec.Field{
		spec.FieldFrequency, spec.FieldMode, spec.FieldTag,
		spec.FieldScanSkip, spec.FieldToneMode, spec.FieldToneTx,
		spec.FieldToneRx, spec.FieldTxFrequency,
	}
	for name, tc := range map[string]struct {
		caps spec.Capabilities
		rw   spec.FieldSupport
	}{
		"unverified": {CapabilitiesUnverified(), spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}},
		"simulated":  {CapabilitiesSimulated(), spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}},
	} {
		fields := tc.caps.Banks[0].Fields
		gradedCount := 0
		for _, f := range spec.AllFields() {
			want := spec.FieldSupport{}
			if slices.Contains(graded, f) {
				want, gradedCount = tc.rw, gradedCount+1
			}
			if got := fields[f]; got != want {
				t.Errorf("%s: %s = %+v, want %+v (§2.1)", name, f, got, want)
			}
		}
		if gradedCount != 8 {
			t.Errorf("%s: %d fields graded, want 8 (§2.1, §2.2)", name, gradedCount)
		}
	}
}

// TestBankFields_TheTwoErrataCellsAreZeroForTheirOwnReasons calls out the two
// zeroes a reader arriving from pair 1 would expect to be graded, so that a
// later edit "restoring" either has to argue with a named erratum.
func TestBankFields_TheTwoErrataCellsAreZeroForTheirOwnReasons(t *testing.T) {
	fields := CapabilitiesSimulated().Banks[0].Fields
	// M-E3: no DA command, no data byte — the data-ness is in the mode NAMES
	// (890:3989-3992). Pair 1's 590 rows grade this field from byte 19.
	if got := fields[spec.FieldDataMode]; got != (spec.FieldSupport{}) {
		t.Errorf("FieldDataMode = %+v, want the zero FieldSupport — M-E3: the data modes are legend values C-F, not a byte", got)
	}
	// M-E4: a CHOICE over a PRINTED MA5 deletion command (890:3305,
	// 890:3311), not a consequence of missing evidence.
	if got := fields[spec.FieldErase]; got != (spec.FieldSupport{}) {
		t.Errorf("FieldErase = %+v, want the zero FieldSupport — the standing no-erase rule over a printed MA5", got)
	}
}

// TestFieldAudit_CoversEverySpecField faces this package's own audit against
// the fleet helper, consumed as a BLACK BOX.
func TestFieldAudit_CoversEverySpecField(t *testing.T) {
	drivertest.AssertFieldAuditCoversEverySpecField(t, "auditedFields", auditedFields(), unexpressedFields())
}

// auditedFields is the eight fields the MA0 record expresses (§2.1).
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
		spec.FieldClarifier:         "§1.7, M-E4/M-E5: no clarifier position over the complete 13-parameter account (890:3164-3209); the radio HAS RIT and XIT (890:4558, 890:5411)",
		spec.FieldCTCSSState:        "§2.1: tone is a four-value mode selector, which is FieldToneMode",
		spec.FieldCTCSSTone:         "§2.1: the record carries TWO independent indices (P6, P7) and FieldCTCSSTone is ONE field",
		spec.FieldShift:             "§1.18: no shift selector; split is an absolute second frequency plus a flag",
		spec.FieldTagDisplay:        "§2.1: no tag-display flag in the grid",
		spec.FieldErase:             "§2.8, M-E4: a CHOICE over a PRINTED MA5 deletion command (890:3305, 890:3311)",
		spec.FieldDuplex:            "§1.18: TOTAL absence — zero occurrences of \"duplex\" in this book",
		spec.FieldOffset:            "§1.18: no per-channel offset magnitude; XO is radio-level (890:5389-5395)",
		spec.FieldDTCSCode:          "§1.21: TOTAL absence — zero hits for DCS/DTCS in this book",
		spec.FieldDTCSPolarity:      "§1.20: as FieldDTCSCode",
		spec.FieldFilter:            "§1.22, M-E5: no filter byte in the grid; FL0-FL3 are radio-level (890:2410, 890:2431, 890:2458, 890:2480)",
		spec.FieldDataMode:          "§2.1, M-E3: no DA command and no data byte — the data modes are legend values C-F (890:3989-3992)",
		spec.FieldTuningStepEnabled: "§1.23: no step flag in the grid, and no ST command in this book at all",
		spec.FieldTuningStep:        "§1.23: as FieldTuningStepEnabled — and unlike the TS-480 there is no step to lose, so no write is refused for it",
		spec.FieldProgramTuningStep: "§1.24: no step magnitude in hertz in the record",
		spec.FieldAttenuator:        "§1.25, M-E5: a radio-level function, RA (890:4391), with no record position",
		spec.FieldPreamp:            "§1.26, M-E5: a radio-level function, PA (890:4000), with no record position",
		spec.FieldAntenna:           "§1.27, M-E5: a radio-level command, AN (890:214), with no record position",
		spec.FieldIPPlus:            "§2.1: an Icom concept with no position in this frame and no mention in this book",
	}
}

// TestCapabilities_EveryFieldExplicit reflects over spec.Capabilities and
// enforces both halves of the rule: every field is populated unless it is on
// the deliberately-empty list, and every field on that list is empty. A zero
// left in a populated field is not a neutral omission — a zero MaxFreqHz
// reads as "no ceiling", a zero TagLen truncates every CHIRP import to "".
func TestCapabilities_EveryFieldExplicit(t *testing.T) {
	deliberatelyEmpty := map[string]string{
		"ClarMaxHz":              "§1.7, M-E4/M-E5 — no clarifier position in the grid",
		"ClarStepHz":             "§1.8 — as ClarMaxHz",
		"CTCSSToneRange":         "§1.10 — the tone field is an INDEX into a printed chart",
		"MinFreqHz":              "§1.13, A15 — this book prints no frequency range",
		"MaxFreqHz":              "§1.14, A15 — as MinFreqHz; a zero DISABLES the ceiling check",
		"RequiredSlots":          "§1.15 — the book marks no channel mandatory",
		"ShiftOptions":           "§1.16 — the Yaesu half of the vocabulary pair",
		"CTCSSStates":            "§1.17 — as ShiftOptions",
		"DuplexOptions":          "§1.18 — TOTAL absence; this row publishes only one half of the Icom half",
		"DTCSPolarities":         "§1.20 — DCS appears nowhere in this book",
		"DTCSCodes":              "§1.21 — as DTCSPolarities",
		"Filters":                "§1.22, M-E5 — no filter byte in the grid; FL0-FL3 are radio-level",
		"TuningSteps":            "§1.23 — no step field, and no ST command at all",
		"ProgramTuningStepRange": "§1.24 — no step magnitude in hertz in the record",
		"AttenuatorDB":           "§1.25, M-E5 — RA is radio-level",
		"PreampOptions":          "§1.26, M-E5 — PA is radio-level",
		"AntennaOptions":         "§1.27, M-E5 — AN is radio-level",
		"TagCharset":             "§1.28, A2 — the empty string selects the family default",
	}
	caps := CapabilitiesUnverified()
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
		case zero && !expectedEmpty:
			t.Errorf("%s is the zero value and is not on the deliberately-empty list", name)
		case !zero && expectedEmpty:
			t.Errorf("%s is populated but the deliberately-empty list says %q", name, reason)
		}
	}
	if len(deliberatelyEmpty) != 18 {
		t.Errorf("the deliberately-empty list has %d entries; §1 records eighteen empty cells on this row", len(deliberatelyEmpty))
	}
}

// TestCapabilities_ZeroFrequencyBoundsDisableTheCheck is A15's positive
// proof: the zeroes are a DISABLED bound and not a refusal, so a channel at
// 1 Hz and one at 99 GHz both pass codeplug validation on this row. No false
// refusal and no false promise — and the 11-digit encodability guard still
// lives in the codec, where it is a statement about the wire.
func TestCapabilities_ZeroFrequencyBoundsDisableTheCheck(t *testing.T) {
	caps := CapabilitiesUnverified()
	if caps.MinFreqHz != 0 || caps.MaxFreqHz != 0 {
		t.Fatalf("MinFreqHz/MaxFreqHz = %d/%d, want 0/0 (§1.13, §1.14, A15)", caps.MinFreqHz, caps.MaxFreqHz)
	}
	for _, hz := range []uint64{1, 99_000_000_000} {
		cp := &codeplug.Codeplug{
			Radio:    codeplug.RadioInfo{Model: caps.Model, CATID: caps.CATID},
			Channels: []codeplug.Channel{{Slot: "000", Data: &codeplug.ChannelData{FreqHz: hz, Mode: "FM"}}},
		}
		for _, issue := range codeplug.Validate(cp, caps) {
			if issue.Field == spec.FieldFrequency {
				t.Errorf("at %d Hz: %s — a zero Min/MaxFreqHz must DISABLE the bound, not enforce one", hz, issue.Msg)
			}
		}
	}
}

// TestWriteTrialsComplete_PinnedFalse is plan P18's TWO-PART pin: the
// constant AND the consequence it travels with. A constant-only edit cannot
// pass while leaving the second half untested, which is what stops a
// one-character change unlocking a write.
func TestWriteTrialsComplete_PinnedFalse(t *testing.T) {
	if writeTrialsComplete {
		t.Fatal("writeTrialsComplete is true; no TS-890S has ever been written to by this project (§3.12). Flipping it needs a CapabilitiesRealHardware profile built field class by field class from this row's own trial evidence, and this pin rewritten with the evidence linked")
	}
	caps := CapabilitiesUnverified()
	for _, bank := range caps.Banks {
		for _, f := range spec.AllFields() {
			if bank.Fields[f].CanWrite() {
				t.Errorf("%v %s is writable under the RealHardware baseline while writeTrialsComplete is false", bank.ID, f)
			}
		}
	}
}
