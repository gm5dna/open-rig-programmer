// SPDX-License-Identifier: GPL-3.0-or-later

package ft710

import (
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// TestWriteTrialsComplete_FlippedTrue_M5b is the M5b PR's rewrite of
// the former TestWriteTrialsComplete_PinnedFalse — exactly the visible,
// reviewable test change that test's own doc comment demanded of the
// flip. writeTrialsComplete flipped to true on 13/07/2026 with the
// hardware evidence linked in its doc comment (caps.go) and recorded in
// docs/hardware-notes.md's "M5b write trials" section: the committed
// write-trial protocol ran clean, controller-driven, against a physical
// UK FT-710 (sacrificial channel M-95; every field class exercised with
// immediate readback; no TX key-up; everything restored byte-identical
// except the non-writable P7 bit).
//
// This test pins BOTH halves of what the flip means: the constant is
// true, AND the flip actually engages the hardware-verified profile —
// a RealHardware driver's baseline equals CapabilitiesRealHardware()
// and is genuinely write-capable for the verified fields (the pre-flip
// placeholder returned the Unverified profile even with the constant
// flipped, precisely so a constant-only edit could never unlock a
// write).
func TestWriteTrialsComplete_FlippedTrue_M5b(t *testing.T) {
	if !writeTrialsComplete {
		t.Fatal("writeTrialsComplete = false: the M5b flip has been reverted — if that is deliberate (a safety rollback), revert this test alongside it and restate why")
	}
	caps := New(RealHardware).Capabilities()
	if !caps.FieldSupport(spec.BankMemory, spec.FieldFrequency).CanWrite() {
		t.Fatal("RealHardware baseline is not write-capable for MEM frequency — the flip is not engaging CapabilitiesRealHardware (constant-only edit?)")
	}
}

// TestProfiles_Validate: every capability profile must pass
// spec.Capabilities.Validate — Registry.Register enforces this for
// whichever one the composed driver exposes, but both must hold
// regardless of profile.
func TestProfiles_Validate(t *testing.T) {
	tests := []struct {
		name string
		caps spec.Capabilities
	}{
		{"Unverified", CapabilitiesUnverified()},
		{"Simulated", CapabilitiesSimulated()},
		{"RealHardware", CapabilitiesRealHardware()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.caps.Validate(); err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
		})
	}
}

// allFields is every spec.Field this project models, for exhaustive
// per-field iteration.
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

var receiverCapabilitiesDeliberatelyZero = map[string]string{
	"TuningSteps":            "additions design D8 — the FT-710 memory frame carries no receiver tuning-step field",
	"ProgramTuningStepRange": "additions design D8 — the FT-710 memory frame carries no programmable tuning-step field",
	"AttenuatorDB":           "additions design D8 — the FT-710 memory frame carries no attenuator field",
	"PreampOptions":          "additions design D8 — the FT-710 memory frame carries no preamp field",
	"AntennaOptions":         "additions design D8 — the FT-710 memory frame carries no antenna field",
}

func TestDeliberatelyZeroAudit_ReceiverCapabilities(t *testing.T) {
	if got := reflect.TypeOf(spec.Capabilities{}).NumField(); got != 27 {
		t.Fatalf("spec.Capabilities has %d fields, this audit knows 27", got)
	}
	for _, caps := range []spec.Capabilities{CapabilitiesUnverified(), CapabilitiesSimulated(), CapabilitiesRealHardware()} {
		value := reflect.ValueOf(caps)
		for name, reason := range receiverCapabilitiesDeliberatelyZero {
			if field := value.FieldByName(name); !field.IsValid() {
				t.Errorf("Capabilities.%s no longer exists (%s)", name, reason)
			} else if !field.IsZero() {
				t.Errorf("Capabilities.%s is non-zero but deliberatelyZero says %q", name, reason)
			}
		}
	}
}

// TestCapabilitiesUnverified_NothingWritable is THE write-guard test:
// while writeTrialsComplete is false, a real-hardware session's profile
// must make CanWrite() false for EVERY field of EVERY bank, so
// codeplug.Diff blocks every change and the clone service refuses to
// execute it.
func TestCapabilitiesUnverified_NothingWritable(t *testing.T) {
	caps := CapabilitiesUnverified()
	if len(caps.Banks) == 0 {
		t.Fatal("CapabilitiesUnverified() has no banks")
	}
	for _, b := range caps.Banks {
		for _, f := range allFields {
			if caps.FieldSupport(b.ID, f).CanWrite() {
				t.Errorf("bank %s field %s: CanWrite() = true in the Unverified profile — the hardware write guard is broken", b.ID, f)
			}
		}
	}
}

// TestCapabilitiesSimulated_ExactWritableSet pins the Simulated profile's
// writable set to exactly the fields the codec can express AND real
// hardware honours on MEM and PMS: freq/mode/ctcss-state/shift/tag/
// tag-display. The clarifier is deliberately NOT in the writable set —
// HW-CONFIRMED 2026-07-13 (M5b write trials), the radio silently ignores
// it on write, so its Write is spec.Inert (CanWrite false; see
// TestFieldClarifier_WriteInert_HWConfirmed) even in the simulator, so
// --fake/demo behaviour matches real behaviour. Erase and tone/scan-skip
// stay unwritable — the CODEC cannot express them regardless of what
// hardware might do.
func TestCapabilitiesSimulated_ExactWritableSet(t *testing.T) {
	writable := map[spec.Field]bool{
		spec.FieldFrequency:  true,
		spec.FieldMode:       true,
		spec.FieldCTCSSState: true,
		spec.FieldShift:      true,
		spec.FieldTag:        true,
		spec.FieldTagDisplay: true,
	}

	caps := CapabilitiesSimulated()
	for _, bankID := range []spec.BankID{spec.BankMemory, spec.BankPMS} {
		if _, ok := caps.Bank(bankID); !ok {
			t.Fatalf("Simulated profile missing bank %s", bankID)
		}
		for _, f := range allFields {
			got := caps.FieldSupport(bankID, f).CanWrite()
			if got != writable[f] {
				t.Errorf("bank %s field %s: CanWrite() = %v, want %v", bankID, f, got, writable[f])
			}
		}
	}
}

// TestFieldClarifier_WriteInert_HWConfirmed pins the M5b clarifier
// finding across EVERY profile: FieldClarifier's Write support is
// spec.Inert — HW-CONFIRMED 2026-07-13 (docs/hardware-notes.md), live MW
// frames carrying non-zero clarifier values and Rx/Tx flags were
// accepted without rejection and read back zeros every time; the radio
// transmits-and-ignores. Inert (not Unsupported: the fixed MW frame
// layout always transmits the field, and marking an always-transmitted
// field Unsupported would block EVERY write via codeplug.Diff's
// all-or-nothing gate; not Supported: a changed value provably never
// takes effect). See spec.Inert for the Diff/driver enforcement split.
func TestFieldClarifier_WriteInert_HWConfirmed(t *testing.T) {
	for _, tt := range []struct {
		name string
		caps spec.Capabilities
	}{
		{"Unverified", CapabilitiesUnverified()},
		{"Simulated", CapabilitiesSimulated()},
		{"RealHardware", CapabilitiesRealHardware()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, bankID := range []spec.BankID{spec.BankMemory, spec.BankPMS} {
				fs := tt.caps.FieldSupport(bankID, spec.FieldClarifier)
				if fs.Write != spec.Inert {
					t.Errorf("bank %s FieldClarifier.Write = %s, want Inert (HW-CONFIRMED: transmitted but ignored by the radio)", bankID, fs.Write)
				}
			}
		})
	}
}

// TestCapabilitiesRealHardware_ExactWritableSet pins the flipped
// real-hardware profile's writable set to EXACTLY the six fields the
// M5b trials verified (docs/hardware-notes.md, "M5b write trials") — no
// more, no fewer. This is the flip's safety assertion: an over-broad
// profile (clarifier, tone, skip, or erase creeping into the writable
// set) would claim hardware verification the trials never produced.
func TestCapabilitiesRealHardware_ExactWritableSet(t *testing.T) {
	writable := map[spec.Field]bool{
		spec.FieldFrequency:  true,
		spec.FieldMode:       true,
		spec.FieldCTCSSState: true,
		spec.FieldShift:      true,
		spec.FieldTag:        true,
		spec.FieldTagDisplay: true,
	}

	caps := CapabilitiesRealHardware()
	for _, bankID := range []spec.BankID{spec.BankMemory, spec.BankPMS} {
		if _, ok := caps.Bank(bankID); !ok {
			t.Fatalf("RealHardware profile missing bank %s", bankID)
		}
		for _, f := range allFields {
			got := caps.FieldSupport(bankID, f).CanWrite()
			if got != writable[f] {
				t.Errorf("bank %s field %s: CanWrite() = %v, want %v", bankID, f, got, writable[f])
			}
		}
	}
	// Erase is Unsupported OUTRIGHT on MEM now (NO CAT erase,
	// HW-CONFIRMED) — no longer merely Unverified.
	if fs := caps.FieldSupport(spec.BankMemory, spec.FieldErase); fs.Write != spec.Unsupported {
		t.Errorf("MEM FieldErase.Write = %s, want Unsupported (HW-CONFIRMED: no CAT erase exists)", fs.Write)
	}
}

// TestRealHardwareAndSimulated_WriteSupportAligned pins the M5b brief's
// alignment requirement: the Simulated profile's per-field WRITE
// support equals the flipped real profile's, field for field, on MEM
// and PMS — so --fake/demo behaviour matches real-radio behaviour
// (same writable set, same Inert clarifier, same Unsupported
// tone/skip/erase). Read supports legitimately differ (the simulator
// claims Supported reads; the real profile keeps its labels).
func TestRealHardwareAndSimulated_WriteSupportAligned(t *testing.T) {
	real := CapabilitiesRealHardware()
	sim := CapabilitiesSimulated()
	for _, bankID := range []spec.BankID{spec.BankMemory, spec.BankPMS} {
		for _, f := range allFields {
			r := real.FieldSupport(bankID, f).Write
			sm := sim.FieldSupport(bankID, f).Write
			if r != sm {
				t.Errorf("bank %s field %s: RealHardware Write=%s, Simulated Write=%s — the two profiles' write sets must stay aligned", bankID, f, r, sm)
			}
		}
	}
}

// TestBaseline_Shape pins the static baseline all profiles share: bank
// inventory (MEM 001-099, PMS P1L-P9U, and NOTHING discovered — no
// 60M/EMG), required slots, and the radio parameters.
func TestBaseline_Shape(t *testing.T) {
	for _, profile := range []struct {
		name string
		caps spec.Capabilities
	}{
		{"Unverified", CapabilitiesUnverified()},
		{"Simulated", CapabilitiesSimulated()},
		{"RealHardware", CapabilitiesRealHardware()},
	} {
		t.Run(profile.name, func(t *testing.T) {
			caps := profile.caps

			if caps.Model != "FT-710" {
				t.Errorf("Model = %q, want \"FT-710\"", caps.Model)
			}
			if caps.CATID != "0800" {
				t.Errorf("CATID = %q, want \"0800\"", caps.CATID)
			}

			mem, ok := caps.Bank(spec.BankMemory)
			if !ok {
				t.Fatal("missing MEM bank")
			}
			if len(mem.Slots) != 99 || mem.Slots[0] != "001" || mem.Slots[98] != "099" {
				t.Errorf("MEM slots = %d entries [%q..%q], want 99 [\"001\"..\"099\"]",
					len(mem.Slots), mem.Slots[0], mem.Slots[len(mem.Slots)-1])
			}
			if mem.NoBlank {
				t.Error("MEM bank NoBlank = true, want false (only M-01 is individually required)")
			}

			pms, ok := caps.Bank(spec.BankPMS)
			if !ok {
				t.Fatal("missing PMS bank")
			}
			if len(pms.Slots) != 18 || pms.Slots[0] != "P1L" || pms.Slots[17] != "P9U" {
				t.Errorf("PMS slots = %d entries [%q..%q], want 18 [\"P1L\"..\"P9U\"]",
					len(pms.Slots), pms.Slots[0], pms.Slots[len(pms.Slots)-1])
			}
			// Codex M5b fix wave, Fix 3 (adjudicated HIGH): PMS is
			// deliberately NOT NoBlank. Real radios ship all-PMS-empty
			// (M5a's characterised radio began with all 18 PMS slots
			// empty; M5b's write trials created only P1L). A NoBlank PMS
			// bank made codeplug.Validate reject every real-derived
			// candidate containing an empty PMS slot before Diff ever
			// ran — including a plain MEM-only edit. A populated PMS
			// slot going back to empty stays blocked regardless, by
			// FieldErase never being write-Supported (see
			// CapabilitiesRealHardware's doc comment).
			if pms.NoBlank {
				t.Error("PMS bank NoBlank = true, want false (real radios ship all-PMS-empty; erase stays blocked via FieldErase, not NoBlank)")
			}

			if _, ok := caps.Bank(spec.Bank60m); ok {
				t.Error("static baseline contains a 60M bank — 60 m inventory is DISCOVERED per session, never baseline")
			}
			if _, ok := caps.Bank(spec.BankEMG); ok {
				t.Error("static baseline contains an EMG bank — EMG is DISCOVERED per session, never baseline")
			}

			if len(caps.RequiredSlots) != 1 || caps.RequiredSlots[0] != "001" {
				t.Errorf("RequiredSlots = %v, want [\"001\"]", caps.RequiredSlots)
			}

			if caps.TagLen != 12 {
				t.Errorf("TagLen = %d, want 12", caps.TagLen)
			}
			if caps.ClarMaxHz != 9990 || caps.ClarStepHz != 10 {
				t.Errorf("Clar = %d/%d, want 9990/10", caps.ClarMaxHz, caps.ClarStepHz)
			}
			if len(caps.CTCSSTones) != 50 {
				t.Errorf("CTCSSTones = %d entries, want 50", len(caps.CTCSSTones))
			}
			wantBauds := []int{4800, 9600, 19200, 38400, 115200}
			if len(caps.Bauds) != len(wantBauds) {
				t.Fatalf("Bauds = %v, want %v", caps.Bauds, wantBauds)
			}
			for i, b := range wantBauds {
				if caps.Bauds[i] != b {
					t.Fatalf("Bauds = %v, want %v", caps.Bauds, wantBauds)
				}
			}
			if caps.DefaultBaud != 38400 {
				t.Errorf("DefaultBaud = %d, want 38400", caps.DefaultBaud)
			}
			if caps.MinFreqHz != 30_000 || caps.MaxFreqHz != 75_000_000 {
				t.Errorf("freq range = %d..%d Hz, want 30000..75000000", caps.MinFreqHz, caps.MaxFreqHz)
			}
		})
	}
}

// TestFieldCTCSSTone_ReadUnsupported_HWConfirmed pins the HW-CONFIRMED
// fact (2026-07-13, docs/hardware-notes.md §MR bytes-25/26 refutation):
// per-channel CTCSS tone is NOT readable over CAT, on MEM or PMS, on
// EITHER capability profile — bytes 25-26 (P9) of a live MR answer for
// M-06 read fixed "00" even with a tone demonstrably SET and ACTIVE on
// the radio, refuting the Hamlib live-tone-index theory. This does not
// change CanWrite() (already false via the zero FieldSupport, pinned
// elsewhere) — it pins the READ side specifically, since that is what
// M5a actually settled.
func TestFieldCTCSSTone_ReadUnsupported_HWConfirmed(t *testing.T) {
	for _, tt := range []struct {
		name string
		caps spec.Capabilities
	}{
		{"Unverified", CapabilitiesUnverified()},
		{"Simulated", CapabilitiesSimulated()},
		{"RealHardware", CapabilitiesRealHardware()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, bankID := range []spec.BankID{spec.BankMemory, spec.BankPMS} {
				fs := tt.caps.FieldSupport(bankID, spec.FieldCTCSSTone)
				if fs.Read != spec.Unsupported {
					t.Errorf("bank %s FieldCTCSSTone.Read = %s, want Unsupported (HW-CONFIRMED: no CAT read path exists)", bankID, fs.Read)
				}
			}
		})
	}
}

// TestModes_MatchCatModeNames pins that the capability Modes list is
// exactly the 15 named cat modes, in wire-code order, and that each name
// round-trips through the driver's own name->Mode table back to the same
// display name — the read path and the write path (modeByName) must
// agree on every spelling.
//
// The round-trip renders through catDialect.ModeName, which is what
// read.go's ReadChannel actually calls since M9b task 56 rerouted it.
// Deliberately NOT cat.Mode.String: that survives only as a
// dialect-free diagnostic fallback, so a test round-tripping through it
// would guard the fallback rather than the real rendering path —
// exactly what Mode.String's own doc comment warns against. For
// cat.FT710 the two are byte-identical by construction (cat/dialect.go
// wires the dialect's mode table to the package map Mode.String reads),
// so aiming at the dialect costs nothing and pins the right thing.
func TestModes_MatchCatModeNames(t *testing.T) {
	caps := CapabilitiesUnverified()
	if len(caps.Modes) != 15 {
		t.Fatalf("Modes = %d entries, want 15", len(caps.Modes))
	}
	for _, name := range caps.Modes {
		if name == "-" {
			t.Error("Modes contains \"-\" (ModeUnset) — the unset placeholder is not a selectable mode")
			continue
		}
		mode, ok := modeByName[name]
		if !ok {
			t.Errorf("mode %q has no modeByName entry", name)
			continue
		}
		if got := catDialect.ModeName(mode); got != name {
			t.Errorf("catDialect.ModeName(modeByName[%q]) = %q, want the same name back", name, got)
		}
	}
	if len(modeByName) != len(caps.Modes) {
		t.Errorf("modeByName has %d entries, caps.Modes has %d — the two must cover the same set", len(modeByName), len(caps.Modes))
	}
}
