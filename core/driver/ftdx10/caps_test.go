// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx10

import (
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// allFields is every spec.Field this project models, for exhaustive
// per-field iteration. Its LENGTH is asserted against the bank field maps
// below, so a Field added to core/spec and forgotten here fails a test
// rather than escaping the matrix.
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
func TestCapabilities_EveryFieldExplicit(t *testing.T) {
	const wantFieldCount = 15

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
