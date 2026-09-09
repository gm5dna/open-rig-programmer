// SPDX-License-Identifier: GPL-3.0-or-later

package ft991a

import (
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// allFields is EVERY spec.Field, in spec.AllFields' own declaration order,
// written out for exhaustive per-field iteration.
//
// All twenty-seven, because the capability matrix requires every spec.Field
// to appear EXPLICITLY in every bank's map (§2, §2.1): a field left out of
// the map reads identically to a field deliberately zeroed, and only a
// written-down zero is legible as a decision. core/driver/ft891's namesake
// is the shape precedent.
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
	spec.FieldTxFrequency,
	spec.FieldDuplex,
	spec.FieldOffset,
	spec.FieldToneMode,
	spec.FieldToneTx,
	spec.FieldToneRx,
	spec.FieldDTCSCode,
	spec.FieldDTCSPolarity,
	spec.FieldFilter,
	spec.FieldDataMode,
	spec.FieldTuningStepEnabled,
	spec.FieldTuningStep,
	spec.FieldProgramTuningStep,
	spec.FieldAttenuator,
	spec.FieldPreamp,
	spec.FieldAntenna,
	spec.FieldIPPlus,
}

// deliberatelyUnexpressedFields is EMPTY, and that is the decision rather
// than an omission: this driver's bank maps name every spec.Field, so there
// is no field whose absence needs a reason (matrix §2 — "All twenty-seven
// appear explicitly in every bank's map").
var deliberatelyUnexpressedFields = map[spec.Field]string{}

func TestFieldAuditCoversEverySpecField(t *testing.T) {
	drivertest.AssertFieldAuditCoversEverySpecField(t, "allFields", allFields, deliberatelyUnexpressedFields)
}

// TestWriteTrialsComplete_PinnedFalse is this driver's write guard, pinned
// in BOTH halves (matrix §3.11): the constant is false, AND a RealHardware
// profile is genuinely nothing-writable.
//
// The second half is what makes the pin worth having. A constant-only edit
// must not unlock a write — no production code consults the constant at all
// — so the property that actually protects a radio is the consequence, not
// the constant. Both are asserted here so that a flip has to arrive as a
// visible, reviewable change to this test alongside a real
// CapabilitiesRealHardware profile and the hardware evidence for it.
func TestWriteTrialsComplete_PinnedFalse(t *testing.T) {
	if writeTrialsComplete {
		t.Fatal("writeTrialsComplete = true: no FT-991A write trial has ever been run by this project — if one now has, land the hardware evidence, a CapabilitiesRealHardware profile built from it, the Capabilities arm that selects it, and this test's rewrite together")
	}

	caps := CapabilitiesUnverified()
	for _, b := range caps.Banks {
		for _, f := range allFields {
			if caps.FieldSupport(b.ID, f).CanWrite() {
				t.Errorf("bank %s field %s: CanWrite() = true on the all-Unverified profile — the FT-991A write guard is broken", b.ID, f)
			}
		}
	}
}

// TestProfiles_Validate: every capability profile must pass
// spec.Capabilities.Validate. internal/wiring's registry enforces this for
// whichever profile a composed driver exposes; both must hold regardless.
//
// On THIS radio the check has a second edge no sibling's has: spec.Validate
// admits a five-member CTCSSStates list only while every member's Semantics
// is a declared value and no two members share one (core/spec/validate.go's
// tone-state rules). A five-state vocabulary with a repeated semantic — the
// mistake a copy-paste of the standard three plus two would make — fails
// here.
func TestProfiles_Validate(t *testing.T) {
	for _, tt := range []struct {
		name string
		caps spec.Capabilities
	}{
		{"Unverified", CapabilitiesUnverified()},
		{"Simulated", CapabilitiesSimulated()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.caps.Validate(); err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
		})
	}
}

// tierFieldsMustBeEmpty names the spec.Capabilities fields for which this
// radio's explicit decision is EMPTY — matrix §1.10 and §1.18-1.28, twelve
// of them, each with its own reason recorded there. See
// TestCapabilities_EveryFieldExplicit.
var tierFieldsMustBeEmpty = map[string]bool{
	// §1.18: the record expresses repeater shift as P10's three-value
	// Simplex/Plus/Minus, which is spec.FieldShift, not FieldDuplex.
	"DuplexOptions": true,
	// §1.19: the record's tone vocabulary is P8's FIVE-value CTCSS state.
	// Two of those five are DCS states and that still does not make this an
	// Icom ToneMode radio — ToneModes is a separate spec.Field with its own
	// per-channel byte, and this record has one tone-ish position, not two.
	"ToneModes": true,
	// §1.20/§1.21: this radio has Table 2 (DCS Code Chart, layout 431-446),
	// a DCS POLARITY menu (086, layout 622) and a memory record that can
	// NAME a DCS state — and no per-channel code or polarity field
	// anywhere in the 41 positions. Empty means "no such per-channel
	// field", never "this radio has no DCS".
	"DTCSPolarities": true,
	"DTCSCodes":      true,
	// §1.22: SH WIDTH and NA NARROW are radio-level commands; the record
	// carries no per-channel filter position.
	"Filters": true,
	// §1.23/§1.24: no tuning-step position exists in the record.
	"TuningSteps":            true,
	"ProgramTuningStepRange": true,
	// §1.25/§1.26/§1.27: RA, PA and AC are radio-level commands; the record
	// has no attenuator, preamp or antenna byte. (IPO is not Icom's IP+.)
	"AttenuatorDB":   true,
	"PreampOptions":  true,
	"AntennaOptions": true,
	// §1.28: empty selects the pre-Icom family default (printable ASCII
	// 0x20-0x7E less ';'). MT's P12 legend says only "TAG Characters (up to
	// 12 characters) (ASCII)" (layout 1017) and names no set, and THIS
	// MANUAL HAS NO STATEMENT ABOUT THE TAG'S OWN BYTE ALPHABET AT ALL.
	// Layout 106-109 is NOT that statement and must not be read as one
	// (matrix erratum M-E8): under its governing conditional, "If a
	// particular parameter is not applicable to the FT-991A", it is a
	// filler-byte rule for a parameter this radio does not implement. So
	// nothing points either way, and "" is the conservative direction —
	// narrowing, not widening, is what cannot put an unexpected byte on the
	// wire.
	"SimplexTx":  true,
	"TagCharset": true,
	// §1.10: this radio names a tone by INDEX into CTCSSTones (CN's P3,
	// "000 - 049: Tone Frequency Number", layout 369), so a range would
	// describe a domain it does not have — and spec.Validate refuses a list
	// and a range together, so declaring one would make these capabilities
	// invalid outright.
	"CTCSSToneRange": true,
}

// TestCapabilities_EveryFieldExplicit is the D-caps-explicit decision's
// enforcement (matrix §1): EVERY field of spec.Capabilities is populated,
// in every profile, with nothing left at its zero value — except the twelve
// for which EMPTY is the positive statement "this radio expresses no such
// vocabulary" (§1.10, §1.18-1.28), where populating one would be the
// mistake and the rule is inverted rather than waived.
//
// It reflects over the struct rather than listing the fields, so a field
// ADDED to spec.Capabilities later is caught here as an unpopulated zero
// instead of silently defaulting for this radio. The field COUNT is
// asserted too: reflection over a struct that gained a field would
// otherwise report the new zero as just another failure, whereas the count
// says plainly that the shape moved and this driver has not decided about
// the addition yet.
//
// Why zero is never acceptable in the other sixteen (§1's own preamble): a
// zero MaxFreqHz reads as "no ceiling" to every validator, a zero TagLen
// makes core/csvio's CHIRP import truncate every imported name to "", a
// non-positive entry in Bauds reaches SerialConfig.Baud, and an empty
// ShiftOptions or CTCSSStates fails spec.Validate outright. Four of the
// values populated here are ASSUMED rather than manual-evidenced
// (DefaultBaud, the two frequency bounds, RequiredSlots) and doc.go's
// register carries each one's provenance — the honest response to an
// unverified value is to populate it and record why, never to leave a zero
// that reads as a decision nobody took.
func TestCapabilities_EveryFieldExplicit(t *testing.T) {
	// 28 since additions design D4.2 added the transmit declaration
	// (matrix §1, §5).
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
				t.Fatalf("spec.Capabilities has %d fields, this test knows %d — a field was added or removed and this driver must decide about it explicitly, not inherit a zero", typ.NumField(), wantFieldCount)
			}
			for i := 0; i < typ.NumField(); i++ {
				name := typ.Field(i).Name
				f := v.Field(i)
				if tierFieldsMustBeEmpty[name] {
					if !f.IsZero() || (f.Kind() == reflect.Slice && f.Len() != 0) {
						t.Errorf("field %s is populated — this radio expresses no such vocabulary, and an empty value is the decision, not an omission (matrix §1.10, §1.18-1.28)", name)
					}
					continue
				}
				if f.IsZero() {
					t.Errorf("field %s is the zero value — every spec.Capabilities field must be populated explicitly (see this test's doc comment for why zero is never neutral)", name)
					continue
				}
				// A non-nil but EMPTY slice is not the zero value and
				// would slip past IsZero: check length too.
				if f.Kind() == reflect.Slice && f.Len() == 0 {
					t.Errorf("field %s is an empty (but non-nil) slice — populated means populated", name)
				}
			}
		})
	}
}

// TestBaseline_Shape pins the static baseline both profiles share, VALUE BY
// VALUE against the capability matrix: the identity, the bank inventory,
// the absence of any discovered bank, and every radio parameter — including
// the four ASSUMED ones, whose VALUES are pinned here and whose PROVENANCE
// is doc.go's register.
func TestBaseline_Shape(t *testing.T) {
	for _, profile := range []struct {
		name string
		caps spec.Capabilities
	}{
		{"Unverified", CapabilitiesUnverified()},
		{"Simulated", CapabilitiesSimulated()},
	} {
		t.Run(profile.name, func(t *testing.T) {
			caps := profile.caps

			// §1.1: the registry key's spelling is the project's CHOICE
			// over the manual's fact that the radio is an FT-991A. It is
			// NOT "FT-991", which is a DIFFERENT REAL RADIO.
			if caps.Model != "FT-991A" {
				t.Errorf("Model = %q, want \"FT-991A\" — the exact registry key internal/wiring and internal/radiotext will expect (matrix §1.1)", caps.Model)
			}
			// §1.2: MANUAL-EVIDENCED, ID's P1 legend at layout 772.
			if caps.CATID != "0670" {
				t.Errorf("CATID = %q, want \"0670\" (matrix §1.2: ID's P1 legend, \"0670: FT-991A\", layout 772)", caps.CATID)
			}
			// §1.3: MANUAL-EVIDENCED — this is a transceiver, and the zero
			// value (TransmitUnspecified) is refused by spec.Validate.
			if caps.Transmit != spec.HasTransmitter {
				t.Errorf("Transmit = %v, want spec.HasTransmitter (matrix §1.3)", caps.Transmit)
			}

			// §1.4.1: "001 - 099: Regular Memory Channel", printed on the
			// MC block's legend (layout 915) — the only legend in this
			// manual that decomposes the span.
			mem, ok := caps.Bank(spec.BankMemory)
			if !ok {
				t.Fatal("missing MEM bank")
			}
			if mem.Label != "Memories" {
				t.Errorf("MEM label = %q, want \"Memories\" (a display CHOICE, minted as this package's own const — matrix §1.4.1)", mem.Label)
			}
			if len(mem.Slots) != 99 || mem.Slots[0] != "001" || mem.Slots[98] != "099" {
				t.Errorf("MEM slots = %d entries [%q..%q], want 99 [\"001\"..\"099\"]", len(mem.Slots), mem.Slots[0], mem.Slots[len(mem.Slots)-1])
			}
			if mem.NoBlank {
				t.Error("MEM bank NoBlank = true, want false stated explicitly (matrix §2.5): an empty memory channel is an ordinary state, and only \"001\" is individually claimed, via RequiredSlots")
			}

			// §1.4.2: "100: P-1L 101: P-1U ~ 116: P-9L 117: P-9U", the MC
			// legend's own line (layout 916) — nine pairs over EIGHTEEN
			// CONSECUTIVE CHANNEL NUMBERS, not the siblings' P1L..P9U
			// tokens.
			pms, ok := caps.Bank(spec.BankPMS)
			if !ok {
				t.Fatal("missing PMS bank")
			}
			if pms.Label != "Scan limits (PMS)" {
				t.Errorf("PMS label = %q, want \"Scan limits (PMS)\" (a display CHOICE — matrix §1.4.2)", pms.Label)
			}
			if len(pms.Slots) != 18 || pms.Slots[0] != "100" || pms.Slots[17] != "117" {
				t.Errorf("PMS slots = %d entries [%q..%q], want 18 [\"100\"..\"117\"] — this radio's PMS pairs are DECIMAL CHANNEL NUMBERS (matrix §1.4.2, plan P20)", len(pms.Slots), pms.Slots[0], pms.Slots[len(pms.Slots)-1])
			}
			if pms.NoBlank {
				t.Error("PMS bank NoBlank = true, want false stated explicitly (matrix §2.5) — nothing establishes that an FT-991A ships with populated PMS pairs, and the FT-710's own NoBlank PMS bank was REMOVED at M5b because real radios ship all-PMS-empty")
			}

			// §1.4.3, §3.4: there is NOTHING to discover on this radio —
			// "5xx", "5 MHz", "5MHz" and "EMG" appear in no slot legend of
			// this manual, established mechanically over the whole
			// extraction. Neither bank is declared statically either, so
			// its absence here is the whole statement.
			if _, ok := caps.Bank(spec.Bank60m); ok {
				t.Error("baseline contains a 60M bank — this manual prints no 5 MHz memory bank at all (matrix §1.4.3), and Open discovers nothing (§3.4)")
			}
			if _, ok := caps.Bank(spec.BankEMG); ok {
				t.Error("baseline contains an EMG bank — this manual prints no emergency channel at all (matrix §1.4.3)")
			}
			if len(caps.Banks) != 2 {
				t.Errorf("Banks = %d, want exactly 2 (MEM and PMS — matrix §1.4: \"TWO static banks, and NOTHING DISCOVERED\")", len(caps.Banks))
			}

			// §1.15: ASSUMED — this manual states no such rule anywhere.
			if len(caps.RequiredSlots) != 1 || caps.RequiredSlots[0] != "001" {
				t.Errorf("RequiredSlots = %v, want [\"001\"] (ASSUMED — the RequiredSlots {\"001\"} register entry; matrix §1.15)", caps.RequiredSlots)
			}

			// §1.6: MANUAL-EVIDENCED — P12's legend at layout 1017 and the
			// Set chart's positions 29-40, counted twice by evidence leg G.
			if caps.TagLen != 12 {
				t.Errorf("TagLen = %d, want 12 (matrix §1.6: \"TAG Characters (up to 12 characters) (ASCII)\", layout 1017)", caps.TagLen)
			}
			// §1.7/§1.8: ASSUMED, ONE dialect register entry
			// (ClarifierPolicy.StepHz = 10 AND ClarifierPolicy.MaxAbsHz =
			// 9990). The manual prints 9999 and states no step; 9990 is the
			// largest multiple of the assumed step inside the printed range.
			if caps.ClarMaxHz != 9990 {
				t.Errorf("ClarMaxHz = %d, want 9990 (matrix §1.7: the manual PRINTS 9999 and no step — this is a deduction from the DIALECT register's ClarifierPolicy.StepHz = 10 AND ClarifierPolicy.MaxAbsHz = 9990 entry, not a transcription)", caps.ClarMaxHz)
			}
			if caps.ClarStepHz != 10 {
				t.Errorf("ClarStepHz = %d, want 10 (matrix §1.8: ASSUMED in the DIALECT's register — cited by this driver, not re-registered)", caps.ClarStepHz)
			}

			// §1.11: MANUAL-EVIDENCED — menu 031 CAT RATE, layout 561, and
			// menu 029 (232C RATE, layout 559) is a DIFFERENT PORT and not
			// a source for this field (plan P11).
			wantBauds := []int{4800, 9600, 19200, 38400}
			if !reflect.DeepEqual(caps.Bauds, wantBauds) {
				t.Errorf("Bauds = %v, want %v (matrix §1.11: menu 031 CAT RATE's legend, layout 561 — four rates, NO 115200)", caps.Bauds, wantBauds)
			}
			// §1.12: ASSUMED, and the matrix's own "entry most likely to be
			// wrong". internal/wiring opens a real radio at exactly this,
			// and no baud override exists in the CLI or the GUI.
			if caps.DefaultBaud != 38400 {
				t.Errorf("DefaultBaud = %d, want 38400 (ASSUMED — the DefaultBaud 38400 register entry; this manual has NO factory-default column at all, and the trailing 1 on layout 561 is the Digits field)", caps.DefaultBaud)
			}

			// §1.13/§1.14: the NUMBERS are manual-evidenced (FA's and FB's
			// identical "000030000 - 470000000 (Hz)" legends, layout 699
			// and 715); reading the VFO range as the memory-storable range
			// is the ASSUMED step. THIS IS THE FIRST REGISTERED YAESU WITH
			// VHF/UHF.
			if caps.MinFreqHz != 30_000 || caps.MaxFreqHz != 470_000_000 {
				t.Errorf("freq range = %d..%d Hz, want 30000..470000000 (matrix §1.13/§1.14: FA/FB's printed range, read as the memory-storable range — the MinFreqHz/MaxFreqHz register entry)", caps.MinFreqHz, caps.MaxFreqHz)
			}

			// §1.16: MANUAL-EVIDENCED three-value vocabulary, printed
			// identically on all five blocks that carry it.
			if !reflect.DeepEqual(caps.ShiftOptions, spec.StandardShiftOptions()) {
				t.Errorf("ShiftOptions = %+v, want the standard three (matrix §1.16: P10's \"0: Simplex 1: Plus Shift 2: Minus Shift\")", caps.ShiftOptions)
			}
		})
	}
}

// TestCTCSSStates_AreThisRadiosOwnFive is the P8 vocabulary pin, and it is
// this radio's sharpest divergence from every registered sibling (matrix
// §1.17, §3.7).
//
// FIVE members, spelt exactly as the Stage 0 seams' own tests already fix
// them across four packages — core/spec/dcssemantics_test.go,
// core/codeplug/dcsstate_roundtrip_test.go,
// core/csvio/dcsstate_roundtrip_test.go and app/dcsstate_uispec_test.go —
// so the driver must use these strings or those tests are false.
//
// IT IS NOT spec.StandardCTCSSStates(), and the negative half is asserted
// as well as the positive one: every registered sibling prints 0/1/2 only
// (ftdx10_layout.txt:1197, ftdx101_layout.txt:1291, ft891_layout.txt:977),
// and the standard three are a PREFIX of this radio's five, so a driver
// that reached for the shared helper would pass every length-agnostic check
// and silently publish a three-state vocabulary for a five-state radio.
func TestCTCSSStates_AreThisRadiosOwnFive(t *testing.T) {
	want := []spec.ToneState{
		{Value: "OFF", Semantics: spec.ToneOff},
		{Value: "ENC-DEC", Semantics: spec.ToneEncodeDecode},
		{Value: "ENC", Semantics: spec.ToneEncode},
		{Value: "DCS-ENC-DEC", Semantics: spec.ToneDCSEncodeDecode},
		{Value: "DCS-ENC", Semantics: spec.ToneDCSEncode},
	}
	for _, tt := range []struct {
		name string
		caps spec.Capabilities
	}{
		{"Unverified", CapabilitiesUnverified()},
		{"Simulated", CapabilitiesSimulated()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if !reflect.DeepEqual(tt.caps.CTCSSStates, want) {
				t.Errorf("CTCSSStates = %+v, want %+v (matrix §1.17: P8's five values, printed identically on IF 795-796, MR 977-978, MT 1010-1011, MW 1048-1049 and OI 1128-1129)", tt.caps.CTCSSStates, want)
			}
			if reflect.DeepEqual(tt.caps.CTCSSStates, spec.StandardCTCSSStates()) {
				t.Error("CTCSSStates equals spec.StandardCTCSSStates() — this radio's P8 legend prints FIVE values and the shared helper carries three (matrix §1.17)")
			}
		})
	}
}

// TestCTCSSStates_NeitherDCSMemberRequiresATone pins the consequence
// core/codeplug's validator actually consumes (matrix §1.17, spec open
// question 3): a DCS state needs a CODE, not a tone, and on this radio the
// code is not a field of the memory record at all (§2.4). Reporting true
// would make the validator demand a Known CTCSSTone for a channel whose
// tone this programme can never read.
func TestCTCSSStates_NeitherDCSMemberRequiresATone(t *testing.T) {
	for _, st := range CapabilitiesUnverified().CTCSSStates {
		wantTone := st.Value == "ENC" || st.Value == "ENC-DEC"
		if got := st.RequiresTone(); got != wantTone {
			t.Errorf("state %q RequiresTone() = %v, want %v — the two DCS members need a CODE this record cannot carry, never a tone (matrix §1.17, §2.4)", st.Value, got, wantTone)
		}
	}
}

// TestCTCSSTones_MatchTheStandardChart is the tone-table pin: this driver's
// CTCSSTones equals spec.StandardCTCSSTones() element for element.
//
// FIXTURE PROVENANCE (matrix §1.9): the FT-991A's own CTCSS chart is Table 1
// of the CAT Operation Reference Book rev 1711-D, header at layout 419, rows
// 420-428 — fifty entries numbered 000-049, 67.0 Hz to 254.1 Hz — and it was
// spot-checked element for element against spec.standardCTCSSTones while
// the matrix was written (026 = 159.8, 027 = 162.2, 044 = 225.7,
// 045 = 229.1, 049 = 254.1, and both endpoints). This test does not re-read
// the manual (it cannot); it pins the CONSEQUENCE, so that a future edit to
// the shared chart is caught here rather than quietly changing which tone a
// stored CAT tone number means.
//
// THE CHART IS A PUBLISHED DOMAIN WITH NO WRITABLE FIELD BEHIND IT on this
// radio (§2.4): CN names a tone by index and the memory record carries no
// tone number at all.
func TestCTCSSTones_MatchTheStandardChart(t *testing.T) {
	want := spec.StandardCTCSSTones()
	for _, tt := range []struct {
		name string
		caps spec.Capabilities
	}{
		{"Unverified", CapabilitiesUnverified()},
		{"Simulated", CapabilitiesSimulated()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.caps.CTCSSTones) != len(want) {
				t.Fatalf("CTCSSTones has %d entries, want %d (matrix §1.9: Table 1, layout 419-428)", len(tt.caps.CTCSSTones), len(want))
			}
			for i := range want {
				if tt.caps.CTCSSTones[i] != want[i] {
					t.Errorf("CTCSSTones[%d] = %+v, want %+v", i, tt.caps.CTCSSTones[i], want[i])
				}
			}
		})
	}
}

// TestModes_MatchTheDialect: the advertised mode list is DERIVED from the
// dialect, in wire-code order, with cat.ModeUnset excluded (matrix §1.5).
//
// FOURTEEN names, and the count itself is this radio's own: all five mode
// legends run '1'…'9' then 'A'…'E' with every nibble named — NO HOLE and NO
// 'F', where the FTdx10 fills 'F' with DATA-FM-N and the FT-891 prints
// "A: -", a hole.
//
// There is deliberately no local mode table in this driver to compare
// against; the list below is the MATRIX's transcription, so the assertion
// is driver-vs-matrix rather than driver-vs-itself.
func TestModes_MatchTheDialect(t *testing.T) {
	want := []string{
		"LSB", "USB", "CW", "FM", "AM", "RTTY-LSB", "CW-R",
		"DATA-LSB", "RTTY-USB", "DATA-FM", "FM-N", "DATA-USB", "AM-N", "C4FM",
	}
	for _, tt := range []struct {
		name string
		caps spec.Capabilities
	}{
		{"Unverified", CapabilitiesUnverified()},
		{"Simulated", CapabilitiesSimulated()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if !reflect.DeepEqual(tt.caps.Modes, want) {
				t.Errorf("Modes = %v, want %v (matrix §1.5: the legend printed beside MR 973-975, MT 1006-1008, MW 1044-1046, IF 789-791 and OI 1124-1126)", tt.caps.Modes, want)
			}
			for _, m := range tt.caps.Modes {
				if m == "-" {
					t.Error("Modes offers \"-\" — cat.ModeUnset is a parse-accept-only placeholder that appears in NO FT-991A legend, and offering it would invite a user to write a value core/cat refuses to emit (the DIALECT register's THE cat.ModeUnset MEMBER OF THE MODE TABLE entry)")
				}
			}
		})
	}
}

// TestModes_AreNotTransciribedHere is the negative half of the entry above,
// and it exists because 'E' is the nibble that makes this radio different:
// it is C4FM here and PSK on the FTdx10 — one nibble, two REAL and
// different modes — so core/cat's package-level Mode.String() fallback is
// ACTIVELY WRONG for this radio (matrix §1.5). Every mode string this
// driver publishes must come from the dialect's own table.
func TestModes_AreNotTranscribedHere(t *testing.T) {
	got := modeNames()
	for i, m := range got {
		// The wire byte for index i: '1'..'9' then 'A'..'E' on this radio.
		b := byte('1' + i)
		if i >= 9 {
			b = byte('A' + i - 9)
		}
		if want := catDialect.ModeName(cat.Mode(b)); m != want {
			t.Errorf("modeNames()[%d] = %q, but the dialect names wire byte %q %q — the names must BE the dialect's, never a second transcription", i, m, b, want)
		}
	}
	if len(got) != 14 {
		t.Errorf("modeNames() has %d entries, want 14 (matrix §1.5: 1-9 then A-E, no hole and no 'F')", len(got))
	}
}

// TestPMSSlotsAreGeneratedThroughTheDialect is the pin spec Stage 2 item 7
// and plan task 10 both ask for by name, and it is this radio's own hazard
// rather than a generic tidiness rule.
//
// Every registered sibling's PMS slot strings are "P1L"…"P9U", so the first
// copy-paste from core/driver/ftdx10/caps.go that keeps the literals
// produces eighteen slot strings THIS dialect's ParseSlot refuses — a
// silent non-slot rather than a compile error (matrix §1.4.2). The
// assertion is therefore in both directions: every advertised slot parses,
// and "P1L" does not.
func TestPMSSlotsAreGeneratedThroughTheDialect(t *testing.T) {
	caps := CapabilitiesUnverified()
	for _, b := range caps.Banks {
		for _, s := range b.Slots {
			if _, err := catDialect.ParseSlot(s); err != nil {
				t.Errorf("bank %s advertises slot %q, which this dialect's ParseSlot refuses: %v — bank inventories are GENERATED through the dialect's own MemorySlot/PMSSlot", b.ID, s, err)
			}
		}
	}
	if _, err := catDialect.ParseSlot("P1L"); err == nil {
		t.Error("ParseSlot(\"P1L\") succeeded — this radio's PMS form is NUMERIC (matrix §1.4.2), so the sibling token form must be a non-slot here; a caps.go that kept the literals would advertise eighteen of them")
	}
}

// TestBanksPartition pins the property spec.Validate already enforces and
// this radio makes CHECKABLE rather than assumed (matrix §1.4.5): MEM is
// "001"-"099" and PMS is "100"-"117", one contiguous number line split in
// two, so a slot string appearing in both banks is a real possibility a
// literal list could produce and the dialect's own slot space cannot.
func TestBanksPartition(t *testing.T) {
	seen := map[string]spec.BankID{}
	for _, b := range CapabilitiesUnverified().Banks {
		for _, s := range b.Slots {
			if other, dup := seen[s]; dup {
				t.Errorf("slot %q appears in both bank %s and bank %s — spec.Validate refuses a slot in two banks outright", s, other, b.ID)
			}
			seen[s] = b.ID
		}
	}
	if len(seen) != 117 {
		t.Errorf("the two banks name %d distinct slots, want 117 (001-099 plus 100-117 — the whole span this manual prints)", len(seen))
	}
}

// TestProfileMatrix_StaticPerField pins matrix §2.1's table cell by cell,
// for both profiles and both banks: SIX graded fields and TWENTY-ONE zero
// ones, which is twelve graded cells and forty-two zero cells over the
// fifty-four the table draws.
//
// spec.FieldTagDisplay is among the ZEROS here, and that is this radio's
// inversion of the FT-891's most distinctive cell (matrix §2.3, erratum
// M-E3): MT's P11 legend reads "P11 0: (Fixed)" (layout 1015) where the
// FT-891 prints `0: TAG "OFF" 1: TAG "ON"`. It is a MANUAL-EVIDENCED
// ABSENCE, not a policy.
func TestProfileMatrix_StaticPerField(t *testing.T) {
	graded := map[spec.Field]bool{
		spec.FieldFrequency:  true,
		spec.FieldMode:       true,
		spec.FieldClarifier:  true,
		spec.FieldCTCSSState: true,
		spec.FieldShift:      true,
		spec.FieldTag:        true,
	}
	for _, tt := range []struct {
		name string
		caps spec.Capabilities
		want spec.FieldSupport
	}{
		{"Unverified", CapabilitiesUnverified(), spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}},
		{"Simulated", CapabilitiesSimulated(), spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			gradedCells, zeroCells := 0, 0
			for _, b := range tt.caps.Banks {
				for _, f := range allFields {
					got := tt.caps.FieldSupport(b.ID, f)
					if graded[f] {
						gradedCells++
						if got != tt.want {
							t.Errorf("bank %s field %s = %+v, want %+v (matrix §2.1)", b.ID, f, got, tt.want)
						}
						continue
					}
					zeroCells++
					if got != (spec.FieldSupport{}) {
						t.Errorf("bank %s field %s = %+v, want the ZERO FieldSupport (matrix §2.1)", b.ID, f, got)
					}
				}
			}
			if gradedCells != 12 || zeroCells != 42 {
				t.Errorf("counted %d graded and %d zero cells, want 12 and 42 over the 54 the matrix draws (§2.1)", gradedCells, zeroCells)
			}
		})
	}
}

// TestTagDisplayIsTheZeroFieldSupport states matrix §2.3 on its own,
// because the ZERO is the whole finding and a per-cell loop states it only
// incidentally.
//
// The honest reading is "this radio's memory frame has no display flag",
// which is STRONGER than the FT-891 matrix's §2.5 zero ("this driver's read
// of this bank cannot reach the field"). Two existing project behaviours
// follow and neither is new code: a CHIRP-imported channel arrives
// Unavailable and never blocks (core/csvio/chirp.go's chirpTagDisplay), and
// a GUI-added blank row arrives Unavailable too (app/uispec.go's
// bankTagDisplayDefault).
func TestTagDisplayIsTheZeroFieldSupport(t *testing.T) {
	for _, tt := range []struct {
		name string
		caps spec.Capabilities
	}{
		{"Unverified", CapabilitiesUnverified()},
		{"Simulated", CapabilitiesSimulated()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, b := range tt.caps.Banks {
				fs := tt.caps.FieldSupport(b.ID, spec.FieldTagDisplay)
				if fs != (spec.FieldSupport{}) {
					t.Errorf("bank %s: FieldTagDisplay = %+v, want the zero FieldSupport — MT's P11 is SCHEMA on this radio, \"P11 0: (Fixed)\" at layout 1015 (matrix §2.3, erratum M-E3)", b.ID, fs)
				}
				if !fs.Unreachable() {
					t.Errorf("bank %s: FieldTagDisplay is reachable — chirpTagDisplay and bankTagDisplayDefault both key on Unreachable() to answer Unavailable", b.ID)
				}
			}
		})
	}
}

// TestCapabilitiesSimulated_ExactlySixWritable: the simulator profile
// claims Write Supported for SIX fields and no more — frequency, mode,
// clarifier, CTCSS state, shift and the tag, which is exactly what the
// combined MT form expresses on this radio.
//
// SIX, where the FT-891's is seven: that radio's P11 is a live TAG flag and
// this radio's is fixed (matrix §2.3, §3.7). The count is asserted as well
// as the membership so that a field silently gaining a Supported write
// fails here rather than in a downstream gate.
func TestCapabilitiesSimulated_ExactlySixWritable(t *testing.T) {
	want := map[spec.Field]bool{
		spec.FieldFrequency:  true,
		spec.FieldMode:       true,
		spec.FieldClarifier:  true,
		spec.FieldCTCSSState: true,
		spec.FieldShift:      true,
		spec.FieldTag:        true,
	}
	caps := CapabilitiesSimulated()
	for _, b := range caps.Banks {
		n := 0
		for _, f := range allFields {
			if !caps.FieldSupport(b.ID, f).CanWrite() {
				continue
			}
			n++
			if !want[f] {
				t.Errorf("bank %s: field %s is writable on the Simulated profile — only the six the combined MT form expresses may be", b.ID, f)
			}
		}
		if n != len(want) {
			t.Errorf("bank %s: %d writable fields, want %d (matrix §2.1's rw column)", b.ID, n, len(want))
		}
	}
}

// TestClarifierIsNeverInert: spec.Inert is the FT-710's HARDWARE finding
// (13/07/2026 — a real FT-710 accepted MW frames carrying non-zero
// clarifier values and read back zeros every time). No FT-991A has ever
// been asked, so there is no finding to borrow, and borrowing one would
// answer a question about one radio with another radio's evidence (matrix
// §2.1).
func TestClarifierIsNeverInert(t *testing.T) {
	for _, tt := range []struct {
		name string
		caps spec.Capabilities
	}{
		{"Unverified", CapabilitiesUnverified()},
		{"Simulated", CapabilitiesSimulated()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, b := range tt.caps.Banks {
				fs := tt.caps.FieldSupport(b.ID, spec.FieldClarifier)
				if fs.Read == spec.Inert || fs.Write == spec.Inert {
					t.Errorf("bank %s: FieldClarifier = %+v — Inert is the FT-710's hardware finding about the FT-710", b.ID, fs)
				}
			}
		})
	}
}

// TestCapabilities_ClarifierDerivesFromDialect pins the IDENTITY, not
// merely the values: the bound and its datum must live in one place.
//
// The dialect carries the single register entry "ClarifierPolicy.StepHz =
// 10 AND ClarifierPolicy.MaxAbsHz = 9990"; this package carries NO literal
// for the pair, so a partial lift in core/cat/ft991a moves the capability
// table with it rather than leaving two transcriptions to drift.
func TestCapabilities_ClarifierDerivesFromDialect(t *testing.T) {
	pol := catDialect.Clarifier()
	for _, tt := range []struct {
		name string
		caps spec.Capabilities
	}{
		{"Unverified", CapabilitiesUnverified()},
		{"Simulated", CapabilitiesSimulated()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.caps.ClarMaxHz != pol.MaxAbsHz || tt.caps.ClarStepHz != pol.StepHz {
				t.Errorf("caps clarifier = %d/%d, dialect = %d/%d — the capability table must CONSULT the dialect, never re-transcribe it", tt.caps.ClarMaxHz, tt.caps.ClarStepHz, pol.MaxAbsHz, pol.StepHz)
			}
		})
	}
}

// TestCATID_ComesFromTheDialect: one place this string exists, so the value
// the ID probe compares against is the value the capability data
// advertises (matrix §1.2).
func TestCATID_ComesFromTheDialect(t *testing.T) {
	if catID != catDialect.CATID() {
		t.Errorf("catID = %q, dialect CATID = %q — the driver must source it from the dialect", catID, catDialect.CATID())
	}
	if catID != "0670" {
		t.Errorf("catID = %q, want \"0670\" (matrix §1.2: ID's P1 legend, layout 772)", catID)
	}
}

// TestBankFieldMapsAreNotShared: each bank gets its own map, so a caller
// mutating one bank's Fields can never reach another's.
func TestBankFieldMapsAreNotShared(t *testing.T) {
	caps := CapabilitiesSimulated()
	if len(caps.Banks) < 2 {
		t.Fatalf("expected two banks, got %d", len(caps.Banks))
	}
	caps.Banks[0].Fields[spec.FieldFrequency] = spec.FieldSupport{}
	if got := caps.Banks[1].Fields[spec.FieldFrequency]; got == (spec.FieldSupport{}) {
		t.Error("mutating the MEM bank's field map changed the PMS bank's — each bank must get a fresh map")
	}
}

// TestCloneCapabilities_IsADeepCopy: Session.Capabilities hands copies out
// and a caller mutating one must never alter what the write gate enforces.
func TestCloneCapabilities_IsADeepCopy(t *testing.T) {
	base := CapabilitiesSimulated()
	cp := base.Clone()

	cp.Banks[0].Slots[0] = "CLOBBERED"
	cp.Banks[0].Fields[spec.FieldFrequency] = spec.FieldSupport{}
	cp.Modes[0] = "CLOBBERED"
	cp.Bauds[0] = -1
	cp.RequiredSlots[0] = "CLOBBERED"
	cp.CTCSSStates[0] = spec.ToneState{}
	cp.ShiftOptions[0] = spec.ShiftOption{}
	cp.CTCSSTones[0] = spec.Tone(0)

	again := CapabilitiesSimulated()
	if !reflect.DeepEqual(base, again) {
		t.Error("mutating Clone's product reached the original — the copy is not deep, and Session.Capabilities' defensive-copy guarantee rests on it")
	}
}
