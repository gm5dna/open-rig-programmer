// SPDX-License-Identifier: GPL-3.0-or-later

package ft891

import (
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// allFields is EVERY spec.Field, in spec.AllFields' own declaration order,
// written out for exhaustive per-field iteration.
//
// All twenty-seven, unlike the FTdx10's ten: the capability matrix requires
// every spec.Field to appear EXPLICITLY in every bank's map (§2, §2.1),
// because a field left out of the map reads identically to a field
// deliberately zeroed and only a written-down zero is legible as a decision.
// core/driver/icr8600's allFieldsForAudit is the shape precedent.
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
// appear explicitly in every bank's map"). core/driver/icr8600's empty map
// is the precedent.
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
		t.Fatal("writeTrialsComplete = true: no FT-891 write trial has ever been run by this project — if one now has, land the hardware evidence, a CapabilitiesRealHardware profile built from it, the Capabilities arm that selects it, and this test's rewrite together")
	}

	caps := CapabilitiesUnverified()
	for _, b := range caps.Banks {
		for _, f := range allFields {
			if caps.FieldSupport(b.ID, f).CanWrite() {
				t.Errorf("bank %s field %s: CanWrite() = true on the all-Unverified profile — the FT-891 write guard is broken", b.ID, f)
			}
		}
	}
}

// TestProfiles_Validate: every capability profile must pass
// spec.Capabilities.Validate. internal/wiring's registry enforces this for
// whichever profile a composed driver exposes; both must hold regardless.
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
	// §1.19: the record's tone vocabulary is P8's three-value CTCSS state.
	"ToneModes": true,
	// §1.20/§1.21: this radio HAS DCS (Table 2, CN's P2=1, CT's "3: DCS")
	// but the 41-position MT record has no per-channel DCS field at all.
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
	// 0x20-0x7E less ';'), which is the STRICT subset of the alphabet this
	// manual's own sentence would admit — narrowing, not widening, is the
	// direction that cannot put an unexpected byte on the wire.
	"TagCharset": true,
	// §1.10: this radio names a tone by INDEX into CTCSSTones (CN's P3,
	// "000 - 049: Tone Frequency Number"), so a range would describe a
	// domain it does not have — and spec.Validate refuses a list and a
	// range together, so declaring one would make these capabilities
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
	const wantFieldCount = 28

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
			// over the manual's fact that the radio is an FT-891.
			if caps.Model != "FT-891" {
				t.Errorf("Model = %q, want \"FT-891\" — the exact registry key internal/wiring and internal/radiotext will expect (matrix §1.1)", caps.Model)
			}
			// §1.2: MANUAL-EVIDENCED, ID's P1 legend at layout 763.
			if caps.CATID != "0650" {
				t.Errorf("CATID = %q, want \"0650\" (matrix §1.2: ID's P1 legend, \"0650: FT-891\", layout 763)", caps.CATID)
			}
			// §1.3: MANUAL-EVIDENCED — this is a transceiver, and the zero
			// value (TransmitUnspecified) is refused by spec.Validate.
			if caps.Transmit != spec.HasTransmitter {
				t.Errorf("Transmit = %v, want spec.HasTransmitter (matrix §1.3)", caps.Transmit)
			}

			// §1.4.1: "001 - 099 (Regular Memory Channel)", printed on
			// MR's P0/1 (960), MT's P1 (998) and MW's P1 (1035).
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
				t.Error("MEM bank NoBlank = true, want false stated explicitly (matrix §2.4): an empty memory channel is an ordinary state, and only \"001\" is individually claimed, via RequiredSlots")
			}

			// §1.4.2: "P1L - P9U (PMS)", nine pairs, eighteen slots.
			pms, ok := caps.Bank(spec.BankPMS)
			if !ok {
				t.Fatal("missing PMS bank")
			}
			if pms.Label != "Scan limits (PMS)" {
				t.Errorf("PMS label = %q, want \"Scan limits (PMS)\" (a display CHOICE — matrix §1.4.2)", pms.Label)
			}
			if len(pms.Slots) != 18 || pms.Slots[0] != "P1L" || pms.Slots[17] != "P9U" {
				t.Errorf("PMS slots = %d entries [%q..%q], want 18 [\"P1L\"..\"P9U\"]", len(pms.Slots), pms.Slots[0], pms.Slots[len(pms.Slots)-1])
			}
			if pms.NoBlank {
				t.Error("PMS bank NoBlank = true, want false stated explicitly (matrix §2.4) — nothing establishes that an FT-891 ships with populated PMS pairs, and the FT-710's own NoBlank PMS bank was REMOVED at M5b because real radios ship all-PMS-empty")
			}

			// §1.4.3: the 60M and EMG banks are DISCOVERED per session by
			// Open, never asserted statically.
			if _, ok := caps.Bank(spec.Bank60m); ok {
				t.Error("static baseline contains a 60M bank — the 5xx inventory is DISCOVERED per session (matrix §1.4.3, §3.4), never baseline")
			}
			if _, ok := caps.Bank(spec.BankEMG); ok {
				t.Error("static baseline contains an EMG bank — EMG is DISCOVERED per session, never baseline")
			}

			// §1.15: ASSUMED — this manual states no such rule anywhere.
			if len(caps.RequiredSlots) != 1 || caps.RequiredSlots[0] != "001" {
				t.Errorf("RequiredSlots = %v, want [\"001\"] (ASSUMED — the RequiredSlots {\"001\"} register entry; matrix §1.15)", caps.RequiredSlots)
			}

			// §1.6: MANUAL-EVIDENCED — P12's legend and the Set chart's
			// positions 29-40, counted twice off 300 dpi renders.
			if caps.TagLen != 12 {
				t.Errorf("TagLen = %d, want 12 (matrix §1.6: \"TAG Characters (up to 12 characters) (ASCII)\", layout 1017)", caps.TagLen)
			}
			// §1.7/§1.8: ASSUMED, ONE dialect register entry
			// (ClarifierPolicy.StepHz = 10 AND MaxAbsHz = 9990). The manual
			// prints 9999 and states no step; 9990 is the largest multiple
			// of the assumed step inside the printed range.
			if caps.ClarMaxHz != 9990 {
				t.Errorf("ClarMaxHz = %d, want 9990 (matrix §1.7: the manual PRINTS 9999 and no step — this is a deduction from the DIALECT register's ClarifierPolicy.StepHz/MaxAbsHz entry, not a transcription)", caps.ClarMaxHz)
			}
			if caps.ClarStepHz != 10 {
				t.Errorf("ClarStepHz = %d, want 10 (matrix §1.8: ASSUMED in the DIALECT's register, ClarifierPolicy.StepHz — cited by this driver, not re-registered)", caps.ClarStepHz)
			}

			// §1.11: MANUAL-EVIDENCED — menu 0506 CAT RATE, layout 553.
			// FOUR rates and no 115200.
			wantBauds := []int{4800, 9600, 19200, 38400}
			if !reflect.DeepEqual(caps.Bauds, wantBauds) {
				t.Errorf("Bauds = %v, want %v (matrix §1.11: menu 0506 CAT RATE's legend, layout 553 — four rates, NO 115200)", caps.Bauds, wantBauds)
			}
			// §1.12: ASSUMED, and the matrix's own "entry most likely to be
			// wrong". internal/wiring opens a real radio at exactly this.
			if caps.DefaultBaud != 38400 {
				t.Errorf("DefaultBaud = %d, want 38400 (ASSUMED — the DefaultBaud 38400 register entry; this manual has NO factory-default column at all, and the trailing 1 on layout 553 is the Digits field)", caps.DefaultBaud)
			}

			// §1.13/§1.14: the NUMBERS are manual-evidenced (FA's and FB's
			// identical "000030000 - 056000000 (Hz)" legends, layout 702
			// and 718); reading the VFO range as the memory-storable range
			// is the ASSUMED step.
			if caps.MinFreqHz != 30_000 || caps.MaxFreqHz != 56_000_000 {
				t.Errorf("freq range = %d..%d Hz, want 30000..56000000 (matrix §1.13/§1.14: FA/FB's printed range, read as the memory-storable range — the MinFreqHz/MaxFreqHz register entry)", caps.MinFreqHz, caps.MaxFreqHz)
			}

			// §1.16/§1.17: MANUAL-EVIDENCED three-value vocabularies,
			// printed identically on all five blocks that carry them.
			if !reflect.DeepEqual(caps.ShiftOptions, spec.StandardShiftOptions()) {
				t.Errorf("ShiftOptions = %+v, want the standard three (matrix §1.16: P10's \"0: Simplex 1: Plus Shift 2: Minus Shift\")", caps.ShiftOptions)
			}
			if !reflect.DeepEqual(caps.CTCSSStates, spec.StandardCTCSSStates()) {
				t.Errorf("CTCSSStates = %+v, want the standard three (matrix §1.17: P8's three values, and CT's fourth is LIVE STATE, not a memory field)", caps.CTCSSStates)
			}
		})
	}
}

// TestCTCSSTones_MatchTheStandardChart is the tone-table pin: this driver's
// CTCSSTones equals spec.StandardCTCSSTones() element for element.
//
// FIXTURE PROVENANCE (matrix §1.9): the FT-891's own CTCSS chart is Table 1
// of CAT Operation Reference Book rev 1909-C, layout 420-429, printed folio
// 6 — fifty entries numbered 000-049, 67.0 Hz to 254.1 Hz — and it was
// spot-checked element for element against spec.standardCTCSSTones while
// the matrix was written, including the dense 026/027 and 044/045 run where
// two charts of the same family are most likely to diverge. This test does
// not re-read the manual (it cannot); it pins the CONSEQUENCE, so that a
// future edit to the shared chart is caught here rather than quietly
// changing which tone a stored CAT tone number means.
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
			got := tt.caps.CTCSSTones
			if len(got) != len(want) {
				t.Fatalf("CTCSSTones has %d entries, want %d (the manual's Table 1 numbers them 000-049)", len(got), len(want))
			}
			for i := range want {
				if got[i] != want[i] {
					t.Errorf("CTCSSTones[%d] = %v, want %v (CAT tone number %03d)", i, got[i], want[i], i)
				}
			}
		})
	}
}

// TestCTCSSTones_NonEmptyForTheDenseBankRule states, as its own pin, the
// dependence app/uispec_test.go's dense-bank class has on this list being
// non-empty (matrix §1.9/§1.17, and the plan's task 1 note about
// app/uispec_test.go:2938). The registration task adds "FT-891" to that
// list; if this driver ever advertised an empty tone list or an empty CTCSS
// state list, that membership would become false in another package, far
// from the change that caused it.
func TestCTCSSTones_NonEmptyForTheDenseBankRule(t *testing.T) {
	caps := CapabilitiesSimulated()
	if len(caps.CTCSSTones) == 0 {
		t.Error("CTCSSTones is empty — the dense-bank class in app/uispec_test.go relies on this radio offering a tone list")
	}
	if len(caps.CTCSSStates) == 0 {
		t.Error("CTCSSStates is empty — spec.Validate refuses that outright, and the dense-bank class relies on it too")
	}
}

// TestModes_MatchTheDialect pins the capability Modes list: exactly the
// FT-891 dialect's own TWELVE selectable modes, in WIRE-CODE order, each
// round-tripping through the dialect's two directions, and the unset
// placeholder absent (matrix §1.5).
//
// The list is DERIVED from the dialect (caps.go's modeNames), so this test
// is not guarding a transcription — there is none. What it guards is the
// derivation's three real risks: that the COUNT is the manual's twelve (the
// legends run 1-9 then B-D, with a printed HOLE at 'A' and no 'E'/'F' at
// all, where the FTdx10 fills all three), that the ORDER is the wire's
// rather than a map's (Go randomises map iteration), and that every
// advertised name resolves BACK to the mode it came from — the property the
// write path will depend on when it resolves a stored mode string through
// dialect.ModeByName.
func TestModes_MatchTheDialect(t *testing.T) {
	caps := CapabilitiesUnverified()
	if len(caps.Modes) != 12 {
		t.Fatalf("Modes = %d entries, want 12 (matrix §1.5: MR's P6 at layout 972-974, MT's at 1007-1010 and MW's at 1043-1046, identical name-for-name and nibble-for-nibble)", len(caps.Modes))
	}

	// Wire-code order, with 'A' ABSENT: the legends' own order, spelled
	// out here so the ORDER is asserted against the manual rather than
	// against whatever the derivation produced.
	wantWire := "123456789BCD"
	wantNames := []string{"LSB", "USB", "CW", "FM", "AM", "RTTY-LSB", "CW-R", "DATA-LSB", "RTTY-USB", "FM-N", "DATA-USB", "AM-N"}
	if !reflect.DeepEqual(caps.Modes, wantNames) {
		t.Errorf("Modes = %v,\nwant %v (matrix §1.5's twelve names, in wire-code order)", caps.Modes, wantNames)
	}
	for i, name := range caps.Modes {
		if name == "-" {
			t.Error("Modes contains \"-\" (cat.ModeUnset) — the placeholder appears in no FT-891 mode legend (the DIALECT register's \"THE cat.ModeUnset MEMBER OF THE MODE TABLE\" entry) and is not a selectable mode")
			continue
		}
		mode, ok := catDialect.ModeByName(name)
		if !ok {
			t.Errorf("Modes[%d] = %q, which the dialect's own ModeByName does not resolve — an advertised mode the write path could not encode", i, name)
			continue
		}
		if got := byte(mode); got != wantWire[i] {
			t.Errorf("Modes[%d] = %q is wire byte %q, want %q — the list must be in wire-code order, not a map's iteration order", i, name, rune(got), rune(wantWire[i]))
		}
		if got := catDialect.ModeName(mode); got != name {
			t.Errorf("ModeName(ModeByName(%q)) = %q, want the same name back", name, got)
		}
	}
}

// TestProfileMatrix_StaticPerField is the STATIC profile matrix: for every
// reachable profile, the EXACT FieldSupport of every one of the
// twenty-seven fields on both static banks (matrix §2.1).
//
// Membership, not counting: each profile's expectation is written out field
// by field, so a support level that moves (Unverified drifting to
// Supported, a zero field gaining a level) fails with the field named.
//
// SEVEN reachable fields, not the FTdx10's six: spec.FieldTagDisplay is
// REACHABLE on this radio (matrix §3.7 — MT's P11 is a LIVE flag, "0: TAG
// \"OFF\" 1: TAG \"ON\"" at layout 1016, where every registered
// combined-form sibling prints "0: (Fixed)"). That inversion is the single
// biggest capability difference between this driver and its siblings.
func TestProfileMatrix_StaticPerField(t *testing.T) {
	var (
		unverifiedRW = spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}
		supportedRW  = spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
		zero         = spec.FieldSupport{}
	)

	profileFor := func(rw spec.FieldSupport) map[spec.Field]spec.FieldSupport {
		want := map[spec.Field]spec.FieldSupport{}
		for _, f := range allFields {
			want[f] = zero
		}
		// The seven the combined MT record expresses and this driver maps
		// in both directions (matrix §2.1): frequency (P2), mode (P6), the
		// clarifier (P3/P4, with the TX half refused pre-wire rather than
		// graded — §2.2), CTCSS state (P8), shift (P10), the tag (P12) and
		// the LIVE display flag (P11).
		want[spec.FieldFrequency] = rw
		want[spec.FieldMode] = rw
		want[spec.FieldClarifier] = rw
		want[spec.FieldCTCSSState] = rw
		want[spec.FieldShift] = rw
		want[spec.FieldTag] = rw
		want[spec.FieldTagDisplay] = rw
		return want
	}

	for _, tt := range []struct {
		name string
		caps spec.Capabilities
		want map[spec.Field]spec.FieldSupport
	}{
		{"CapabilitiesUnverified", CapabilitiesUnverified(), profileFor(unverifiedRW)},
		{"CapabilitiesSimulated", CapabilitiesSimulated(), profileFor(supportedRW)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, bankID := range []spec.BankID{spec.BankMemory, spec.BankPMS} {
				bank, ok := tt.caps.Bank(bankID)
				if !ok {
					t.Fatalf("profile is missing bank %s", bankID)
				}
				if len(bank.Fields) != len(allFields) {
					t.Errorf("bank %s lists %d fields, want all %d spelled out — an ABSENT field reads exactly like a deliberately zeroed one (matrix §2)", bankID, len(bank.Fields), len(allFields))
				}
				for _, f := range allFields {
					got := tt.caps.FieldSupport(bankID, f)
					if got != tt.want[f] {
						t.Errorf("bank %s field %s: FieldSupport = {Read:%s Write:%s}, want {Read:%s Write:%s} (matrix §2.1)", bankID, f, got.Read, got.Write, tt.want[f].Read, tt.want[f].Write)
					}
				}
			}
		})
	}
}

// TestCapabilitiesSimulated_ExactlySevenWritable states the Simulated
// profile's writable set as a SET, alongside the per-field matrix above:
// exactly frequency, mode, clarifier, ctcss_state, shift, tag AND
// tag_display can be written, on MEM and PMS alike.
//
// SEVEN, where the FTdx10's is six: the count is not incidental, because
// app/uispec.go's capability-derived core-field rule (a field is core iff
// its FieldSupport on that bank is non-zero) selects the same set, so this
// is the number of columns an FT-891 grid offers — the plan's ft891CoreSeven
// (task 7).
func TestCapabilitiesSimulated_ExactlySevenWritable(t *testing.T) {
	writable := map[spec.Field]bool{
		spec.FieldFrequency:  true,
		spec.FieldMode:       true,
		spec.FieldClarifier:  true,
		spec.FieldCTCSSState: true,
		spec.FieldShift:      true,
		spec.FieldTag:        true,
		spec.FieldTagDisplay: true,
	}

	caps := CapabilitiesSimulated()
	count := 0
	for _, bankID := range []spec.BankID{spec.BankMemory, spec.BankPMS} {
		for _, f := range allFields {
			got := caps.FieldSupport(bankID, f).CanWrite()
			if got != writable[f] {
				t.Errorf("bank %s field %s: CanWrite() = %v, want %v", bankID, f, got, writable[f])
			}
			if got {
				count++
			}
		}
	}
	if want := 2 * len(writable); count != want {
		t.Errorf("Simulated profile has %d writable (bank, field) pairs, want %d — exactly seven per bank", count, want)
	}
}

// TestClarifierIsNeverInert pins a NON-borrowing (matrix §2.1's third
// bullet): spec.Inert is the FT-710's HARDWARE finding — on 13/07/2026 a
// real FT-710 accepted MW frames carrying non-zero clarifier values without
// rejection and read back zeros every time. No FT-891 has ever been asked,
// so there is no finding to record here, and borrowing one would answer a
// question about one radio with another radio's evidence.
func TestClarifierIsNeverInert(t *testing.T) {
	for _, tt := range []struct {
		name string
		caps spec.Capabilities
	}{
		{"Unverified", CapabilitiesUnverified()},
		{"Simulated", CapabilitiesSimulated()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, bankID := range []spec.BankID{spec.BankMemory, spec.BankPMS} {
				fs := tt.caps.FieldSupport(bankID, spec.FieldClarifier)
				if fs.Read == spec.Inert || fs.Write == spec.Inert {
					t.Errorf("bank %s clarifier = {Read:%s Write:%s} — Inert is the FT-710's hardware finding about the FT-710, and no FT-891 has been asked", bankID, fs.Read, fs.Write)
				}
			}
		})
	}
}

// TestDiscoveredBankFields_TagAndTagDisplayAreZero is matrix §2.5, and it
// is the plan's P4: the FT-891's discovered banks are read by MR ALONE, and
// the MR Answer is 28 positions carrying neither a tag nor a display flag
// (layout 968-975). So this driver's readOnlyFields forces FieldTag AND
// FieldTagDisplay to the ZERO FieldSupport as well as forcing every other
// Write to Unsupported.
//
// THIS IS NOT THE FTdx10'S OR FTdx101'S DERIVATION, and the plan says so in
// terms: those drivers carry every Read support through unchanged, because
// they read every bank with the same combined MT frame that carries the
// tag. Reusing theirs here would advertise a readable tag on a bank whose
// only frame cannot carry one.
//
// The honest reading of the zero is "this driver's read of this bank cannot
// reach the field", not "this radio has no such field" — nothing here
// claims an FT-891's 5 MHz channels have no tags.
func TestDiscoveredBankFields_TagAndTagDisplayAreZero(t *testing.T) {
	for _, tt := range []struct {
		name string
		caps spec.Capabilities
	}{
		{"Unverified", CapabilitiesUnverified()},
		{"Simulated", CapabilitiesSimulated()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fields := readOnlyFields(tt.caps)
			if len(fields) != len(allFields) {
				t.Fatalf("a discovered bank lists %d fields, want all %d", len(fields), len(allFields))
			}
			for _, f := range allFields {
				fs := fields[f]
				if fs.Write != spec.Unsupported {
					t.Errorf("discovered bank field %s: Write = %s, want Unsupported (matrix §1.4.3: no profile may claim a 5xx/EMG slot writable)", f, fs.Write)
				}
				if f == spec.FieldTag || f == spec.FieldTagDisplay {
					if fs != (spec.FieldSupport{}) {
						t.Errorf("discovered bank field %s = {Read:%s Write:%s}, want the ZERO FieldSupport — MR's 28-position answer carries neither (matrix §2.5)", f, fs.Read, fs.Write)
					}
					continue
				}
				// Every OTHER field keeps MEM's read support: the MR
				// answer carries the whole 28-position field block.
				if want := tt.caps.FieldSupport(spec.BankMemory, f).Read; fs.Read != want {
					t.Errorf("discovered bank field %s: Read = %s, want MEM's %s carried through", f, fs.Read, want)
				}
			}
		})
	}
}

// TestBankFieldMapsAreNotShared: each bank gets a FRESH field map, so a
// caller (or a future derivation) mutating one bank's map cannot reach
// another's. cloneCapabilities makes the copies a Session hands out
// defensive; this is the same property one layer lower, at construction.
func TestBankFieldMapsAreNotShared(t *testing.T) {
	caps := CapabilitiesSimulated()
	if len(caps.Banks) < 2 {
		t.Fatalf("baseline has %d banks, want MEM and PMS", len(caps.Banks))
	}
	caps.Banks[0].Fields[spec.FieldErase] = spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	if got := caps.Banks[1].Fields[spec.FieldErase]; got != (spec.FieldSupport{}) {
		t.Errorf("PMS erase = {Read:%s Write:%s} after MEM's map was mutated — the two banks share one map", got.Read, got.Write)
	}
}
