// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx10

import (
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// allFields is the TRANSMIT-AND-MEMORY spec.Fields — those core/spec/field.go
// declared before the additions design's D8 minted seven receiver fields on
// 28/08/2026 — written out for exhaustive per-field iteration.
//
// IT IS NOT EVERY spec.Field, and the comment here said it was until this
// sweep. Its LENGTH is asserted against the bank field maps below, so a Field
// added to core/spec, listed in this radio's banks and forgotten here still
// fails a test; a Field this radio's banks do not carry at all — which is
// every one of the seven D8 receiver fields — escapes both sides equally.
// TestFieldAuditCoversEverySpecField now catches that case without widening
// this slice or making a capability-table decision.
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
}

var deliberatelyUnexpressedFields = map[spec.Field]string{
	spec.FieldTxFrequency:       "design D4 — the FTdx10 memory frame carries no independent transmit-frequency field",
	spec.FieldDuplex:            "design D4 — the FTdx10 memory frame carries no Icom duplex field",
	spec.FieldOffset:            "design D4 — the FTdx10 memory frame carries no per-channel repeater-offset field",
	spec.FieldToneMode:          "design D4 — the FTdx10 memory frame carries no Icom tone-mode field",
	spec.FieldToneTx:            "design D4 — the FTdx10 memory frame carries no separate transmit-tone field",
	spec.FieldToneRx:            "design D4 — the FTdx10 memory frame carries no separate receive-tone field",
	spec.FieldDTCSCode:          "design D4 — the FTdx10 memory frame carries no DTCS-code field",
	spec.FieldDTCSPolarity:      "design D4 — the FTdx10 memory frame carries no DTCS-polarity field",
	spec.FieldFilter:            "design D4 — the FTdx10 memory frame carries no per-channel IF-filter field",
	spec.FieldDataMode:          "design D4 — the FTdx10 memory frame carries no Icom data-mode flag",
	spec.FieldTuningStepEnabled: "additions design D8 — the FTdx10 memory frame carries no tuning-step-enabled field",
	spec.FieldTuningStep:        "additions design D8 — the FTdx10 memory frame carries no tuning-step field",
	spec.FieldProgramTuningStep: "additions design D8 — the FTdx10 memory frame carries no programmable-tuning-step field",
	spec.FieldAttenuator:        "additions design D8 — the FTdx10 memory frame carries no attenuator field",
	spec.FieldPreamp:            "additions design D8 — the FTdx10 memory frame carries no preamp field",
	spec.FieldAntenna:           "additions design D8 — the FTdx10 memory frame carries no antenna-selection field",
	spec.FieldIPPlus:            "additions design D8 — the FTdx10 memory frame carries no IP+ field",
}

func TestFieldAuditCoversEverySpecField(t *testing.T) {
	drivertest.AssertFieldAuditCoversEverySpecField(t, "allFields", allFields, deliberatelyUnexpressedFields)
}

// TestWriteTrialsComplete_PinnedFalse is this driver's write guard, pinned
// in BOTH halves: the constant is false, AND a RealHardware driver's
// baseline is genuinely nothing-writable.
//
// The second half is what makes the pin worth having. A constant-only edit
// must not unlock a write — the FT-710's pre-flip design refused exactly
// that, and this driver's Capabilities switch does not consult the constant
// at all (see its doc comment) — so the property that actually protects a
// radio is the consequence, not the constant. Both are asserted here so
// that a flip has to arrive as a visible, reviewable change to this test
// alongside a real CapabilitiesRealHardware profile and the hardware
// evidence for it.
func TestWriteTrialsComplete_PinnedFalse(t *testing.T) {
	if writeTrialsComplete {
		t.Fatal("writeTrialsComplete = true: no FTdx10 write trial has ever been run by this project — if one now has, land the hardware evidence, a CapabilitiesRealHardware profile built from it, the Capabilities arm that selects it, and this test's rewrite together")
	}

	caps := New(RealHardware).Capabilities()
	for _, b := range caps.Banks {
		for _, f := range allFields {
			if caps.FieldSupport(b.ID, f).CanWrite() {
				t.Errorf("bank %s field %s: CanWrite() = true on the RealHardware profile — the FTdx10 write guard is broken", b.ID, f)
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

// tierFieldsMustBeEmpty names the spec.Capabilities fields the Icom tier
// added, for which this radio's explicit decision is EMPTY — see
// TestCapabilities_EveryFieldExplicit's doc comment.
var tierFieldsMustBeEmpty = map[string]bool{
	"DuplexOptions":          true,
	"ToneModes":              true,
	"DTCSPolarities":         true,
	"DTCSCodes":              true,
	"Filters":                true,
	"TuningSteps":            true,
	"ProgramTuningStepRange": true,
	"AttenuatorDB":           true,
	"PreampOptions":          true,
	"AntennaOptions":         true,
	"TagCharset":             true,
	// CTCSSToneRange (Wave 2.5, E3) is the OPTIONAL numeric tone domain a
	// radio whose tone field is a number declares INSTEAD of a chart.
	// This radio's explicit decision is NIL, and it is a decision rather
	// than an omission twice over: this radio names a tone by its INDEX
	// into CTCSSTones, so a range would describe a domain it does not
	// have — and spec.Validate refuses a list and a range together, so
	// declaring one here would make these capabilities invalid outright.
	"CTCSSToneRange": true,
}

// TestCapabilities_EveryFieldExplicit is the D-caps-explicit decision's
// enforcement: EVERY field of spec.Capabilities is populated, in every
// profile, with nothing left at its zero value.
//
// It reflects over the struct rather than listing the fields, so a field
// ADDED to spec.Capabilities later is caught here as an unpopulated zero
// instead of silently defaulting for this radio. The field COUNT is
// asserted too: reflection over a struct that gained a field would
// otherwise report the new zero as just another failure, whereas the count
// says plainly that the shape moved and this driver has not decided about
// the addition yet.
//
// Why zero is never acceptable in this data: a zero MaxFreqHz reads as "no
// ceiling" to every validator, a zero TagLen makes core/csvio's CHIRP
// import truncate every channel name to "", an empty Bauds makes
// core/transport substitute a guessed baud, and an empty ShiftOptions or
// CTCSSStates fails Validate outright. Three of the values populated here
// are ASSUMED rather than manual-evidenced (DefaultBaud, the frequency
// bounds, RequiredSlots) and doc.go's register carries each one's
// provenance — the honest response to an unverified value is to populate it
// and record why, never to leave a zero that reads as a decision nobody
// took.
//
// The Icom tier added SEVEN fields to spec.Capabilities — design D4's
// DuplexOptions, ToneModes, DTCSPolarities, DTCSCodes, Filters and
// TagCharset, and Wave 2.5's CTCSSToneRange — and the additions design's
// D8 added FIVE more (TuningSteps, ProgramTuningStepRange, AttenuatorDB,
// PreampOptions, AntennaOptions); for THIS radio the
// explicit decision about every one of them is that it must be EMPTY (nil,
// for the pointer-declared range). That is not a zero slipping through:
// empty is the positive statement "this radio expresses no such
// vocabulary", which is what makes every capability-keyed check in
// core/codeplug and core/csvio skip the Icom branch and leave this
// radio's behaviour exactly as it was. Populating any of them would be
// the mistake, so the rule for those six is inverted here rather than
// waived, and the test still fails if one is ever filled in.
func TestCapabilities_EveryFieldExplicit(t *testing.T) {
	// 28 since additions design D4.2 added the transmit declaration.
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
						t.Errorf("field %s is populated — this radio expresses no Icom-family vocabulary, and an empty value is the decision, not an omission", name)
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

// TestProfileMatrix_StaticPerField is the STATIC profile matrix: for every
// reachable profile, the EXACT FieldSupport of every field on every static
// bank. No fake, no session, no wire — this is what the driver claims
// before anything is connected, which is what every write gate above it
// consults.
//
// Membership, not counting: each profile's expectation is written out field
// by field, so a support level that moves (Unverified drifting to
// Supported, a zero field gaining a level) fails with the field named.
//
// The rows deliberately include the ZERO-VALUE Profile and an
// UNRECOGNISED one. The zero value must be RealHardware — a forgotten
// Profile must not select the simulator's writable set — and an
// unrecognised value must fail safe to nothing-writable, which is the
// property that holds whatever a forged or corrupted Profile carries.
func TestProfileMatrix_StaticPerField(t *testing.T) {
	var (
		unverifiedRW = spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}
		supportedRW  = spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
		zero         = spec.FieldSupport{}
	)

	// The all-Unverified profile: the five fields the combined MT record
	// expresses plus the clarifier are Unverified in both directions;
	// tag_display (no such flag in the record), tone, skip and erase are
	// the zero FieldSupport.
	unverifiedProfile := map[spec.Field]spec.FieldSupport{
		spec.FieldFrequency:  unverifiedRW,
		spec.FieldMode:       unverifiedRW,
		spec.FieldClarifier:  unverifiedRW,
		spec.FieldCTCSSState: unverifiedRW,
		spec.FieldShift:      unverifiedRW,
		spec.FieldTag:        unverifiedRW,
		spec.FieldTagDisplay: zero,
		spec.FieldCTCSSTone:  zero,
		spec.FieldScanSkip:   zero,
		spec.FieldErase:      zero,
	}

	// The Simulated profile: the SAME six fields Supported in both
	// directions — INCLUDING the clarifier, which is Supported and NOT
	// spec.Inert. Inert is the FT-710's hardware finding about the FT-710
	// (its radio accepts a clarifier write and reads back zeros); no
	// FTdx10 has ever been asked, and internal/fakedx10 — the only thing
	// this profile is ever legal against — stores the clarifier and
	// round-trips it byte-faithfully. See doc.go's non-borrowing note.
	simulatedProfile := map[spec.Field]spec.FieldSupport{
		spec.FieldFrequency:  supportedRW,
		spec.FieldMode:       supportedRW,
		spec.FieldClarifier:  supportedRW,
		spec.FieldCTCSSState: supportedRW,
		spec.FieldShift:      supportedRW,
		spec.FieldTag:        supportedRW,
		spec.FieldTagDisplay: zero,
		spec.FieldCTCSSTone:  zero,
		spec.FieldScanSkip:   zero,
		spec.FieldErase:      zero,
	}

	for _, tt := range []struct {
		name string
		caps spec.Capabilities
		want map[spec.Field]spec.FieldSupport
	}{
		{"RealHardware (writeTrialsComplete false)", New(RealHardware).Capabilities(), unverifiedProfile},
		{"the zero-value Profile is RealHardware", New(Profile(0)).Capabilities(), unverifiedProfile},
		{"an unrecognised Profile fails safe", New(Profile(99)).Capabilities(), unverifiedProfile},
		{"Simulated", New(Simulated).Capabilities(), simulatedProfile},
		{"CapabilitiesUnverified", CapabilitiesUnverified(), unverifiedProfile},
		{"CapabilitiesSimulated", CapabilitiesSimulated(), simulatedProfile},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, bankID := range []spec.BankID{spec.BankMemory, spec.BankPMS} {
				bank, ok := tt.caps.Bank(bankID)
				if !ok {
					t.Fatalf("profile is missing bank %s", bankID)
				}
				if len(bank.Fields) != len(allFields) {
					t.Errorf("bank %s lists %d fields, want all %d spelled out — an ABSENT field reads exactly like a deliberately zeroed one", bankID, len(bank.Fields), len(allFields))
				}
				for _, f := range allFields {
					got := tt.caps.FieldSupport(bankID, f)
					if got != tt.want[f] {
						t.Errorf("bank %s field %s: FieldSupport = {Read:%s Write:%s}, want {Read:%s Write:%s}", bankID, f, got.Read, got.Write, tt.want[f].Read, tt.want[f].Write)
					}
				}
			}
		})
	}
}

// TestCapabilitiesSimulated_ExactlySixWritable states the Simulated
// profile's writable set as a SET, alongside the per-field matrix above:
// exactly frequency, mode, clarifier, ctcss_state, shift and tag can be
// written, on MEM and PMS alike.
//
// These are the same six fields app/uispec.go's capability-derived core-field
// rule will select for this radio (D5a: a field is core iff its
// FieldSupport on that bank is non-zero), so the count is not incidental —
// it is the number of columns an FTdx10 grid offers.
func TestCapabilitiesSimulated_ExactlySixWritable(t *testing.T) {
	writable := map[spec.Field]bool{
		spec.FieldFrequency:  true,
		spec.FieldMode:       true,
		spec.FieldClarifier:  true,
		spec.FieldCTCSSState: true,
		spec.FieldShift:      true,
		spec.FieldTag:        true,
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
		t.Errorf("Simulated profile has %d writable (bank, field) pairs, want %d — exactly six per bank", count, want)
	}
}

// TestCTCSSTones_MatchTheStandardChart is the tone-table pin: this
// driver's CTCSSTones equals spec.StandardCTCSSTones() element for
// element.
//
// FIXTURE PROVENANCE: the FTdx10's own CTCSS chart is Table 1 of the CAT
// Operation Reference Manual rev 2308-F (layout lines 501-514) — 50
// entries numbered 000-049, 67.0 Hz to 254.1 Hz — and it was compared
// against spec.standardCTCSSTones ENTRY BY ENTRY during the M9c-6 spec
// review's manual-access leg, including the dense 026-035 run where two
// charts of the same family are most likely to diverge. All 50 matched.
// This test does not re-read the manual (it cannot); it pins the
// CONSEQUENCE of that comparison, so that a future edit to either table
// has to face the check rather than quietly changing which tone a stored
// CAT tone number means.
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

// TestBaseline_Shape pins the static baseline both profiles share: the
// identity, the bank inventory, the absence of any discovered bank, and
// every radio parameter — including the three ASSUMED ones, whose VALUES
// are pinned here and whose PROVENANCE is doc.go's register.
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

			if caps.Model != "FTdx10" {
				t.Errorf("Model = %q, want \"FTdx10\" — the exact registry key internal/wiring's TestModelSlug and internal/radiotext both expect (NOT \"FT-DX10\")", caps.Model)
			}
			if caps.CATID != "0761" {
				t.Errorf("CATID = %q, want \"0761\"", caps.CATID)
			}

			mem, ok := caps.Bank(spec.BankMemory)
			if !ok {
				t.Fatal("missing MEM bank")
			}
			if len(mem.Slots) != 99 || mem.Slots[0] != "001" || mem.Slots[98] != "099" {
				t.Errorf("MEM slots = %d entries [%q..%q], want 99 [\"001\"..\"099\"]", len(mem.Slots), mem.Slots[0], mem.Slots[len(mem.Slots)-1])
			}
			if mem.NoBlank {
				t.Error("MEM bank NoBlank = true, want false (an empty memory channel is ordinary; only \"001\" is individually claimed, via RequiredSlots)")
			}

			pms, ok := caps.Bank(spec.BankPMS)
			if !ok {
				t.Fatal("missing PMS bank")
			}
			if len(pms.Slots) != 18 || pms.Slots[0] != "P1L" || pms.Slots[17] != "P9U" {
				t.Errorf("PMS slots = %d entries [%q..%q], want 18 [\"P1L\"..\"P9U\"]", len(pms.Slots), pms.Slots[0], pms.Slots[len(pms.Slots)-1])
			}
			if pms.NoBlank {
				t.Error("PMS bank NoBlank = true, want false — nothing establishes that an FTdx10 ships with populated PMS pairs, and the FT-710's own NoBlank PMS bank was REMOVED at M5b because real radios ship all-PMS-empty and codeplug.Validate then rejected every real-derived candidate")
			}

			if _, ok := caps.Bank(spec.Bank60m); ok {
				t.Error("static baseline contains a 60M bank — the 5xx inventory is DISCOVERED per session, never baseline")
			}
			if _, ok := caps.Bank(spec.BankEMG); ok {
				t.Error("static baseline contains an EMG bank — EMG is DISCOVERED per session, never baseline")
			}

			if len(caps.RequiredSlots) != 1 || caps.RequiredSlots[0] != "001" {
				t.Errorf("RequiredSlots = %v, want [\"001\"] (ASSUMED — the RequiredSlots register entry)", caps.RequiredSlots)
			}

			if caps.TagLen != 12 {
				t.Errorf("TagLen = %d, want 12 (P12: \"TAG Characters (up to 12 characters) (ASCII)\")", caps.TagLen)
			}
			if caps.ClarMaxHz != 9990 {
				t.Errorf("ClarMaxHz = %d, want 9990 (manual-evidenced: the MR/MT/MW legends and the RD/RU pages agree)", caps.ClarMaxHz)
			}
			if caps.ClarStepHz != 10 {
				t.Errorf("ClarStepHz = %d, want 10 (ASSUMED in the DIALECT's register, ClarifierPolicy.StepHz — cited here, not re-registered)", caps.ClarStepHz)
			}

			// FOUR rates and no 115200: the FTdx10's CAT RATE menu
			// (3-01-08, manual line 811) is the first real Bauds
			// divergence from the FT-710's five, and it is
			// manual-evidenced. DefaultBaud is the ASSUMED half (register
			// DefaultBaud 38400 entry): this manual has no factory-default
			// column at all.
			wantBauds := []int{4800, 9600, 19200, 38400}
			if !reflect.DeepEqual(caps.Bauds, wantBauds) {
				t.Errorf("Bauds = %v, want %v (four rates, NO 115200)", caps.Bauds, wantBauds)
			}
			if caps.DefaultBaud != 38400 {
				t.Errorf("DefaultBaud = %d, want 38400 (ASSUMED — the DefaultBaud 38400 register entry; internal/wiring opens a real radio at exactly this rate)", caps.DefaultBaud)
			}

			if caps.MinFreqHz != 30_000 || caps.MaxFreqHz != 75_000_000 {
				t.Errorf("freq range = %d..%d Hz, want 30000..75000000 (ASSUMED — the MinFreqHz/MaxFreqHz register entry)", caps.MinFreqHz, caps.MaxFreqHz)
			}

			if !reflect.DeepEqual(caps.ShiftOptions, spec.StandardShiftOptions()) {
				t.Errorf("ShiftOptions = %+v, want the standard three", caps.ShiftOptions)
			}
			if !reflect.DeepEqual(caps.CTCSSStates, spec.StandardCTCSSStates()) {
				t.Errorf("CTCSSStates = %+v, want the standard three", caps.CTCSSStates)
			}
		})
	}
}

// TestModes_MatchTheDialect pins the capability Modes list: exactly the
// FTdx10 dialect's own 15 selectable modes, in WIRE-CODE order, each one
// round-tripping through the dialect's two directions, and the unset
// placeholder absent.
//
// The list is derived from the dialect (caps.go's modeNames), so this test
// is not guarding a transcription — there is none. What it guards is the
// derivation's three real risks: that the COUNT is the manual's 15 (the
// legends run 1-F, so a 16th entry means the placeholder leaked in or the
// dialect's table grew), that the ORDER is the wire's rather than a map's
// (Go randomises map iteration, and a naive derivation over the dialect's
// mode map would produce a differently-ordered mode picker on every run),
// and that every advertised name resolves BACK to the mode it came from —
// the property the write path will depend on when it resolves a stored
// mode string through dialect.ModeByName.
//
// It renders through catDialect and deliberately not cat.Mode.String,
// whose table is the FT-710's: a test round-tripping through the package
// fallback would pass while the UI offered this radio another radio's mode
// list.
func TestModes_MatchTheDialect(t *testing.T) {
	caps := CapabilitiesUnverified()
	if len(caps.Modes) != 15 {
		t.Fatalf("Modes = %d entries, want 15 (the manual's four identical mode legends each run 1-F)", len(caps.Modes))
	}

	// Wire-code order: '1'..'9' then 'A'..'F', which is the legends' own
	// order. Spelled out here so the ORDER is asserted against the manual
	// rather than against whatever the derivation produced.
	wantWire := "123456789ABCDEF"
	for i, name := range caps.Modes {
		if name == "-" {
			t.Error("Modes contains \"-\" (cat.ModeUnset) — the unset placeholder appears in no FTdx10 mode legend (the dialect's ASSUMED register entry for the cat.ModeUnset table member) and is not a selectable mode")
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

// TestProfilesNeverEmitConsented: no capability PROFILE mints
// spec.ConsentedUnverified. The state is a session-time statement about a
// user's recorded decision, so the only thing that may ever produce it is
// the consent transform at the assembly point — never a label written down
// in caps.go, where it would apply to every user of the model whether they
// consented or not.
func TestProfilesNeverEmitConsented(t *testing.T) {
	for _, tt := range []struct {
		name string
		caps spec.Capabilities
	}{
		{"Simulated", New(Simulated).Capabilities()},
		{"RealHardware", New(RealHardware).Capabilities()},
		{"unrecognised", New(Profile(99)).Capabilities()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if capsContains(tt.caps, spec.ConsentedUnverified) {
				t.Error("a profile baseline carries ConsentedUnverified — consent belongs to a session, not to the radio's capability data")
			}
		})
	}
}

// TestConsentOption_StaticCapabilitiesNeverConsented: the option changes
// what a SESSION carries and nothing else. A driver built with it still
// describes the radio exactly as one built without it does.
//
// That boundary is load-bearing above this package: internal/wiring's
// registry publishes driver.Capabilities() and refuses a registered set
// carrying ConsentedUnverified on either side (core/driver's registry
// baseline guard), and the app's static surfaces — capability tables,
// settings descriptors, offline bank synthesis — describe the model rather
// than one user's decision.
func TestConsentOption_StaticCapabilitiesNeverConsented(t *testing.T) {
	for _, tt := range []struct {
		name string
		p    Profile
	}{
		{"Simulated", Simulated},
		{"RealHardware", RealHardware},
		{"unrecognised", Profile(99)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := New(tt.p, WithConsentedUnverifiedWrites()).Capabilities()
			if capsContains(got, spec.ConsentedUnverified) {
				t.Error("the consent option reached the STATIC capability set — it must apply only at session-capability assembly")
			}
			if !reflect.DeepEqual(got, New(tt.p).Capabilities()) {
				t.Error("a consented driver's static Capabilities() differ from an unconsented one's")
			}
		})
	}
}

// TestEffectiveCapabilities_Validate: every capability set a session can
// ever carry passes spec.Capabilities.Validate — profiles × discovered
// inventories × consent, assembled through the one seam that builds them
// (sessionCapabilities).
//
// TestProfiles_Validate covers the static baselines only, and the sets
// this driver actually hands out are strictly larger: they carry the
// discovered read-only banks, and now the consent transform's relabelling
// too. Validate is meaningful for a consented set in particular because
// its read-side rule refuses ConsentedUnverified outright, so a transform
// that leaked onto the read side fails HERE rather than at whatever layer
// first tried to enforce it.
func TestEffectiveCapabilities_Validate(t *testing.T) {
	for _, prof := range []struct {
		name string
		p    Profile
	}{
		{"RealHardware", RealHardware},
		{"Simulated", Simulated},
		{"unrecognised", Profile(99)},
	} {
		for _, disc := range []struct {
			name     string
			slots60m []string
			emg      bool
		}{
			{"no discovery", nil, false},
			{"60m only", []string{"503", "599"}, false},
			{"EMG only", nil, true},
			{"60m and EMG", []string{"501"}, true},
		} {
			for _, consent := range []bool{false, true} {
				name := prof.name + "/" + disc.name
				if consent {
					name += "/consented"
				}
				t.Run(name, func(t *testing.T) {
					var opts []Option
					if consent {
						opts = append(opts, WithConsentedUnverifiedWrites())
					}
					d, ok := New(prof.p, opts...).(*ftdx10Driver)
					if !ok {
						t.Fatal("New did not return a *ftdx10Driver")
					}
					if err := d.sessionCapabilities(disc.slots60m, disc.emg).Validate(); err != nil {
						t.Errorf("Validate() = %v, want nil", err)
					}
				})
			}
		}
	}
}

// TestCapabilities_ClarifierDerivesFromDialect is the write-gate sweep's
// item (h), the FT-891's C-H2 applied to this driver: a bound must be
// CONSULTED from the same place as its datum. Before this fix caps.go
// carried its own literal 9990/10, a second transcription of the values
// write.go's builder already reads through the dialect's Clarifier() — two
// places able to drift silently if either were ever edited alone.
//
// WHAT THIS PIN CAN AND CANNOT SEE: it asserts that caps carry the dialect's
// CURRENT ClarifierPolicy values. A literal in caps.go that happened to equal
// 9990/10 would pass it too — no value comparison can tell a literal from a
// consultation. What it adds over the bare "== 9990" the shape tests make is
// DIRECTION: if the dialect's entry is ever lifted or corrected, this test
// fails until caps follow it, where a bare check would keep asserting the
// stale number. The red-proof (caps.go hard-coded to 9999) fails both, which
// is fine — they guard different things.
//
// THE VALUES DO NOT CHANGE, only their source, so no capability VALUE moves
// and no golden or byte-identity artefact does either.
func TestCapabilities_ClarifierDerivesFromDialect(t *testing.T) {
	want := catDialect.Clarifier()
	for _, tt := range []struct {
		name string
		caps spec.Capabilities
	}{
		{"Unverified", CapabilitiesUnverified()},
		{"Simulated", CapabilitiesSimulated()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.caps.ClarMaxHz != want.MaxAbsHz {
				t.Errorf("ClarMaxHz = %d, want catDialect.Clarifier().MaxAbsHz = %d — the bound must be CONSULTED from the dialect, not a second literal that can drift from it", tt.caps.ClarMaxHz, want.MaxAbsHz)
			}
			if tt.caps.ClarStepHz != want.StepHz {
				t.Errorf("ClarStepHz = %d, want catDialect.Clarifier().StepHz = %d — the bound must be CONSULTED from the dialect, not a second literal that can drift from it", tt.caps.ClarStepHz, want.StepHz)
			}
		})
	}
}
