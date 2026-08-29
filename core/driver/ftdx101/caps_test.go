// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx101

import (
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver"
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

// TestWriteTrialsComplete_PinnedFalse is this package's write guard, pinned
// PER MODEL and in BOTH halves: each model's own constant is false, AND
// that model's RealHardware baseline is genuinely nothing-writable.
//
// TABLE-DRIVEN OVER THE TWO CONSTANTS, and the table row is where the
// per-model discipline becomes mechanical rather than aspirational. The
// matrix's rule (§3.10, restated at §3.11) is that a capture from one radio
// is never evidence about the other: a D write trial must be able to flip
// the D's guard and nothing else. One shared constant could not express
// that, and a test asserting "the constant" once would pass with the wrong
// model's evidence behind it.
//
// The second half of each row is what makes the pin worth having. A
// constant-only edit must not unlock a write — this driver's Capabilities
// switch does not consult either constant at all (see their doc comments) —
// so the property that actually protects a radio is the consequence, not
// the constant. Both are asserted so that a flip has to arrive as a
// visible, reviewable change to this test alongside a real
// CapabilitiesRealHardware profile and that MODEL'S hardware evidence.
func TestWriteTrialsComplete_PinnedFalse(t *testing.T) {
	for _, tt := range []struct {
		model    testModel
		complete bool
	}{
		{testModels[0], writeTrialsCompleteD},
		{testModels[1], writeTrialsCompleteMP},
	} {
		t.Run(tt.model.name, func(t *testing.T) {
			if tt.complete {
				t.Fatalf("the %s write-trial guard = true: no %s has ever been written to by this project — if one now has, land THAT MODEL's hardware evidence, a CapabilitiesRealHardware profile built from it, the Capabilities arm that selects it, and this row's rewrite together. A trial on the other model lifts nothing here.", tt.model.name, tt.model.name)
			}

			caps := tt.model.newDrv(RealHardware).Capabilities()
			for _, b := range caps.Banks {
				for _, f := range allFields {
					if caps.FieldSupport(b.ID, f).CanWrite() {
						t.Errorf("bank %s field %s: CanWrite() = true on the %s RealHardware profile — that model's write guard is broken", b.ID, f, tt.model.name)
					}
				}
			}
		})
	}
}

// TestProfiles_Validate: every capability profile of every model must pass
// spec.Capabilities.Validate. internal/wiring's registry enforces this for
// whichever profile a composed driver exposes; all four must hold
// regardless.
func TestProfiles_Validate(t *testing.T) {
	for _, m := range testModels {
		for _, tt := range []struct {
			name string
			caps spec.Capabilities
		}{
			{"Unverified", capabilitiesUnverified(m.params)},
			{"Simulated", capabilitiesSimulated(m.params)},
		} {
			t.Run(m.name+"/"+tt.name, func(t *testing.T) {
				if err := tt.caps.Validate(); err != nil {
					t.Errorf("Validate() = %v, want nil", err)
				}
			})
		}
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
// enforcement, for BOTH models: EVERY field of spec.Capabilities is
// populated, in every profile, with nothing left at its zero value — and
// every BANK this driver can produce carries its Label and its stated
// NoBlank.
//
// It reflects over the struct rather than listing the fields, so a field
// ADDED to spec.Capabilities later is caught here as an unpopulated zero
// instead of silently defaulting for these radios. The field COUNT is
// asserted too: reflection over a struct that gained a field would
// otherwise report the new zero as just another failure, whereas the count
// says plainly that the shape moved and this driver has not decided about
// the addition yet.
//
// Why zero is never acceptable in this data: a zero MaxFreqHz reads as "no
// ceiling" to every validator, a zero TagLen makes core/csvio's CHIRP
// import truncate every channel name to nothing, an empty Bauds makes
// core/transport substitute a guessed baud, and an empty ShiftOptions or
// CTCSSStates fails Validate outright. Four of the values populated here
// are ASSUMED rather than manual-evidenced (ClarStepHz — the DIALECT's,
// cited — plus DefaultBaud, the frequency bounds and RequiredSlots, this
// driver's own register entries 3, 4 and 5) and each one's provenance is
// written down PER MODEL: the honest response to an unverified value is to
// populate it and record why, never to leave a zero that reads as a
// decision nobody took.
//
// THE BANK LABELS AND NoBlank VALUES ARE ASSERTED HERE, ON ALL FOUR BANKS,
// because they are transcribed capability data like any other (matrix
// §1.3.1, §1.3.2, §1.3.4, §2.4) and because NoBlank's two values are
// opposite for a reason that a silent flip would destroy: false on the
// static banks (an empty channel is ordinary, and a NoBlank MEM bank would
// make codeplug.Validate refuse every candidate with one blank channel —
// the M5b failure), true on the discovered ones (they exist because they
// answered, and this project's write surface offers no way to blank them).
// The discovered banks are taken from the OFFLINE synthesiser, which is
// the same effectiveCapabilities call a live session's Open makes.
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
	// 27 since additions design D8 added the five receiver vocabularies.
	const wantFieldCount = 27

	for _, m := range testModels {
		for _, tt := range []struct {
			name string
			caps spec.Capabilities
		}{
			{"Unverified", capabilitiesUnverified(m.params)},
			{"Simulated", capabilitiesSimulated(m.params)},
		} {
			t.Run(m.name+"/"+tt.name, func(t *testing.T) {
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

		// The bank labels and NoBlank values, all four banks of this model.
		t.Run(m.name+"/bank labels and NoBlank", func(t *testing.T) {
			drv := m.newDrv(Simulated)
			caps := drv.Capabilities()
			discovered := drv.(driver.DiscoveredBankSynthesizer).SynthesiseDiscoveredBanks([]string{"501", "EMG"})
			if len(discovered) != 2 {
				t.Fatalf("SynthesiseDiscoveredBanks([501 EMG]) produced %d banks, want the 60M and EMG pair this leg asserts the labels of", len(discovered))
			}

			for _, tt := range []struct {
				bankID      spec.BankID
				bank        spec.Bank
				wantLabel   string
				wantNoBlank bool
				why         string
			}{
				{
					bankID: spec.BankMemory, bank: mustBank(t, caps, spec.BankMemory),
					wantLabel: "Memories", wantNoBlank: false,
					why: "matrix §1.3.1 — the label is a CHOICE; NoBlank false is a CHOICE, conservative: nothing in this manual says a memory channel must stay populated, and the one channel this driver claims is RequiredSlots' \"001\", the per-SLOT mechanism",
				},
				{
					bankID: spec.BankPMS, bank: mustBank(t, caps, spec.BankPMS),
					wantLabel: "Scan limits (PMS)", wantNoBlank: false,
					why: "matrix §1.3.2 — NoBlank false, deliberately NOT the FT-710's original guess: its own NoBlank PMS bank was REMOVED at M5b because real radios ship all-PMS-empty and codeplug.Validate then rejected every real-derived candidate, MEM-only edits included",
				},
				{
					bankID: spec.Bank60m, bank: discovered[0],
					wantLabel: "60 m channels", wantNoBlank: true,
					why: "matrix §1.3.4 — the label is a CHOICE (the manual's own words are \"5xx (5MHz BAND)\"); NoBlank true is a statement about the write surface this project offers, not about factory contents",
				},
				{
					bankID: spec.BankEMG, bank: discovered[1],
					wantLabel: "Emergency (EMG)", wantNoBlank: true,
					why: "matrix §1.3.4 — as above; the manual's own words are \"EMG (EMERGENCY CH)\"",
				},
			} {
				if tt.bank.ID != tt.bankID {
					t.Errorf("expected bank %s, got %s", tt.bankID, tt.bank.ID)
					continue
				}
				if tt.bank.Label != tt.wantLabel {
					t.Errorf("bank %s Label = %q, want %q (%s)", tt.bankID, tt.bank.Label, tt.wantLabel, tt.why)
				}
				if tt.bank.NoBlank != tt.wantNoBlank {
					t.Errorf("bank %s NoBlank = %v, want %v (%s)", tt.bankID, tt.bank.NoBlank, tt.wantNoBlank, tt.why)
				}
			}
		})
	}
}

// mustBank fetches one bank or fails the test.
func mustBank(t *testing.T, caps spec.Capabilities, id spec.BankID) spec.Bank {
	t.Helper()
	b, ok := caps.Bank(id)
	if !ok {
		t.Fatalf("capabilities have no bank %s", id)
	}
	return b
}

// TestProfileMatrix_StaticPerField is the STATIC profile matrix: for every
// reachable profile of every model, the EXACT FieldSupport of every field
// on every static bank. No fake, no session, no wire — this is what the
// driver claims before anything is connected, which is what every write
// gate above it consults.
//
// Membership, not counting: each profile's expectation is written out field
// by field, so a support level that moves (Unverified drifting to
// Supported, a zero field gaining a level) fails with the field named.
//
// The rows deliberately include the ZERO-VALUE Profile and an UNRECOGNISED
// one, for each model. The zero value must be RealHardware — a forgotten
// Profile must not select the simulator's writable set — and an
// unrecognised value must fail safe to nothing-writable, which is the
// property that holds whatever a forged or corrupted Profile carries.
func TestProfileMatrix_StaticPerField(t *testing.T) {
	var (
		unverifiedRW = spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}
		supportedRW  = spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
		zero         = spec.FieldSupport{}
	)

	// The all-Unverified profile (matrix §2.1's profile table): the five
	// fields the combined MT record expresses plus the clarifier are
	// Unverified in both directions; tag_display (no such flag in the
	// record — a manual-evidenced absence), tone, skip (register entry 6)
	// and erase (no erase command exists at all) are the zero FieldSupport.
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
	// FTdx101 of either model has ever been asked, and internal/fakedx101 —
	// the only thing this profile is ever legal against — will store the
	// clarifier and round-trip it byte-faithfully. See doc.go's
	// non-borrowing note.
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

	for _, m := range testModels {
		for _, tt := range []struct {
			name string
			caps spec.Capabilities
			want map[spec.Field]spec.FieldSupport
		}{
			{"RealHardware (both write guards false)", m.newDrv(RealHardware).Capabilities(), unverifiedProfile},
			{"the zero-value Profile is RealHardware", m.newDrv(Profile(0)).Capabilities(), unverifiedProfile},
			{"an unrecognised Profile fails safe", m.newDrv(Profile(99)).Capabilities(), unverifiedProfile},
			{"Simulated", m.newDrv(Simulated).Capabilities(), simulatedProfile},
			{"capabilitiesUnverified", capabilitiesUnverified(m.params), unverifiedProfile},
			{"capabilitiesSimulated", capabilitiesSimulated(m.params), simulatedProfile},
		} {
			t.Run(m.name+"/"+tt.name, func(t *testing.T) {
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
}

// TestCapabilitiesSimulated_ExactlySixWritable states the Simulated
// profile's writable set as a SET, alongside the per-field matrix above:
// exactly frequency, mode, clarifier, ctcss_state, shift and tag can be
// written, on MEM and PMS alike, for each model.
//
// These are the same six fields app/uispec.go's capability-derived
// core-field rule selects for this radio (D5a: a field is core iff its
// FieldSupport on that bank is non-zero), so the count is not incidental —
// it is the number of columns an FTdx101 grid offers.
func TestCapabilitiesSimulated_ExactlySixWritable(t *testing.T) {
	writable := map[spec.Field]bool{
		spec.FieldFrequency:  true,
		spec.FieldMode:       true,
		spec.FieldClarifier:  true,
		spec.FieldCTCSSState: true,
		spec.FieldShift:      true,
		spec.FieldTag:        true,
	}

	for _, m := range testModels {
		t.Run(m.name, func(t *testing.T) {
			caps := capabilitiesSimulated(m.params)
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
		})
	}
}

// TestCTCSSTones_MatchTheStandardChart is the tone-table pin: this driver's
// CTCSSTones equals spec.StandardCTCSSTones() element for element, for both
// models.
//
// FIXTURE PROVENANCE: the FTdx101's own CTCSS chart is "Table 1 (CTCSS Tone
// Chart)" of the FTDX101MP/FTDX101D CAT Operation Reference Manual rev
// 2308-L — heading at layout 566, body 567-575, printed page 8, reached
// from CN's P3 legend "000 - 049: Tone Frequency Number (See Table 1)" at
// layout 559 — and it was compared against spec.standardCTCSSTones ENTRY BY
// ENTRY, all fifty, while the M9d-2 capability matrix was written (§1.8):
// every index 000-049 agrees, 67.0 Hz at 000 through 254.1 Hz at 049, with
// no insertion, omission or reordering. This test does not re-read the
// manual (it cannot); it pins the CONSEQUENCE of that comparison, so that a
// future edit to either table has to face the check rather than quietly
// changing which tone a stored CAT tone number means.
func TestCTCSSTones_MatchTheStandardChart(t *testing.T) {
	want := spec.StandardCTCSSTones()
	for _, m := range testModels {
		for _, tt := range []struct {
			name string
			caps spec.Capabilities
		}{
			{"Unverified", capabilitiesUnverified(m.params)},
			{"Simulated", capabilitiesSimulated(m.params)},
		} {
			t.Run(m.name+"/"+tt.name, func(t *testing.T) {
				got := tt.caps.CTCSSTones
				if len(got) != len(want) {
					t.Fatalf("CTCSSTones has %d entries, want %d (this manual's Table 1 numbers them 000-049)", len(got), len(want))
				}
				for i := range want {
					if got[i] != want[i] {
						t.Errorf("CTCSSTones[%d] = %v, want %v (CAT tone number %03d)", i, got[i], want[i], i)
					}
				}
			})
		}
	}
}

// TestBaseline_Shape pins the static baseline both profiles share, for each
// model: the identity, the bank inventory, the absence of any discovered
// bank, and every radio parameter — including the ASSUMED ones, whose
// VALUES are pinned here and whose PROVENANCE is doc.go's per-model
// register.
func TestBaseline_Shape(t *testing.T) {
	for _, m := range testModels {
		for _, profile := range []struct {
			name string
			caps spec.Capabilities
		}{
			{"Unverified", capabilitiesUnverified(m.params)},
			{"Simulated", capabilitiesSimulated(m.params)},
		} {
			t.Run(m.name+"/"+profile.name, func(t *testing.T) {
				caps := profile.caps

				if caps.Model != m.name {
					t.Errorf("Model = %q, want %q — the exact registry key the M9d spec fixes (the manual sets both model names in full capitals; the spelling here is the project's)", caps.Model, m.name)
				}
				if caps.CATID != m.catID {
					t.Errorf("CATID = %q, want %q (ID's P1 legend, layout 1070 and 1072 — the one genuine D-vs-MP divergence)", caps.CATID, m.catID)
				}

				mem, ok := caps.Bank(spec.BankMemory)
				if !ok {
					t.Fatal("missing MEM bank")
				}
				if len(mem.Slots) != 99 || mem.Slots[0] != "001" || mem.Slots[98] != "099" {
					t.Errorf("MEM slots = %d entries [%q..%q], want 99 [\"001\"..\"099\"]", len(mem.Slots), mem.Slots[0], mem.Slots[len(mem.Slots)-1])
				}

				pms, ok := caps.Bank(spec.BankPMS)
				if !ok {
					t.Fatal("missing PMS bank")
				}
				if len(pms.Slots) != 18 || pms.Slots[0] != "P1L" || pms.Slots[17] != "P9U" {
					t.Errorf("PMS slots = %d entries [%q..%q], want 18 [\"P1L\"..\"P9U\"]", len(pms.Slots), pms.Slots[0], pms.Slots[len(pms.Slots)-1])
				}

				if _, ok := caps.Bank(spec.Bank60m); ok {
					t.Error("static baseline contains a 60M bank — the 5xx inventory is DISCOVERED per session, never baseline")
				}
				if _, ok := caps.Bank(spec.BankEMG); ok {
					t.Error("static baseline contains an EMG bank — EMG is DISCOVERED per session, never baseline")
				}

				if len(caps.RequiredSlots) != 1 || caps.RequiredSlots[0] != "001" {
					t.Errorf("RequiredSlots = %v, want [\"001\"] (ASSUMED — register entry 5, per model)", caps.RequiredSlots)
				}

				if caps.TagLen != 12 {
					t.Errorf("TagLen = %d, want 12 (MT's P12: \"TAG Characters (up to 12 characters) (ASCII)\", layout 1330, and positions 29-40 in the geometry witness)", caps.TagLen)
				}
				if caps.ClarMaxHz != 9990 {
					t.Errorf("ClarMaxHz = %d, want 9990 (manual-evidenced: five frame blocks and the RD/RU pages all print 0000-9990 Hz)", caps.ClarMaxHz)
				}
				if caps.ClarStepHz != 10 {
					t.Errorf("ClarStepHz = %d, want 10 (ASSUMED in the DIALECT's register, entry \"ClarifierPolicy.StepHz = 10\" — cited, not re-registered)", caps.ClarStepHz)
				}

				// FOUR rates and no 115200: (03,01,11) CAT RATE's P4 legend
				// at layout 863 is manual-evidenced. DefaultBaud is the
				// ASSUMED half (register entry 3): this manual has no
				// factory-default column at all, and the trailing 1 on that
				// line is the DIGITS field.
				wantBauds := []int{4800, 9600, 19200, 38400}
				if !reflect.DeepEqual(caps.Bauds, wantBauds) {
					t.Errorf("Bauds = %v, want %v (four rates, NO 115200)", caps.Bauds, wantBauds)
				}
				if caps.DefaultBaud != 38400 {
					t.Errorf("DefaultBaud = %d, want 38400 (ASSUMED — register entry 3; internal/wiring opens a real radio at exactly this rate)", caps.DefaultBaud)
				}

				if caps.MinFreqHz != 30_000 || caps.MaxFreqHz != 75_000_000 {
					t.Errorf("freq range = %d..%d Hz, want 30000..75000000 (ASSUMED — register entry 4; BS's band-button legend is NOT evidence for it)", caps.MinFreqHz, caps.MaxFreqHz)
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
}

// TestModes_MatchTheDialect pins the capability Modes list: exactly this
// model's dialect's own 15 selectable modes, in WIRE-CODE order, each one
// round-tripping through the dialect's two directions, and the unset
// placeholder absent.
//
// The list is derived from the dialect (caps.go's modeNames), so this test
// is not guarding a transcription — there is none. What it guards is the
// derivation's three real risks: that the COUNT is the manual's 15 (this
// radio's five clean legends run 1-F, so a 16th entry means the placeholder
// leaked in or the dialect's table grew), that the ORDER is the wire's
// rather than a map's (Go randomises map iteration, and a naive derivation
// over the dialect's mode map would produce a differently-ordered mode
// picker on every run), and that every advertised name resolves BACK to the
// mode it came from — the property the write path will depend on when it
// resolves a stored mode string through dialect.ModeByName.
//
// It renders through the MODEL'S OWN dialect and deliberately not
// cat.Mode.String, whose table is the FT-710's: a test round-tripping
// through the package fallback would pass while the UI offered this radio
// another radio's mode list.
func TestModes_MatchTheDialect(t *testing.T) {
	for _, m := range testModels {
		t.Run(m.name, func(t *testing.T) {
			caps := capabilitiesUnverified(m.params)
			if len(caps.Modes) != 15 {
				t.Fatalf("Modes = %d entries, want 15 (this manual's five identical mode legends each run 1-F)", len(caps.Modes))
			}

			// Wire-code order: '1'..'9' then 'A'..'F', which is the legends'
			// own order. Spelled out here so the ORDER is asserted against
			// the manual rather than against whatever the derivation
			// produced.
			wantWire := "123456789ABCDEF"
			for i, name := range caps.Modes {
				if name == "-" {
					t.Error("Modes contains \"-\" (cat.ModeUnset) — the unset placeholder appears in no FTdx101 mode legend (the DIALECT register's \"cat.ModeUnset member of the mode table\" entry) and is not a selectable mode")
					continue
				}
				mode, ok := m.params.dialect.ModeByName(name)
				if !ok {
					t.Errorf("Modes[%d] = %q, which the dialect's own ModeByName does not resolve — an advertised mode the write path could not encode", i, name)
					continue
				}
				if got := byte(mode); got != wantWire[i] {
					t.Errorf("Modes[%d] = %q is wire byte %q, want %q — the list must be in wire-code order, not a map's iteration order", i, name, rune(got), rune(wantWire[i]))
				}
				if got := m.params.dialect.ModeName(mode); got != name {
					t.Errorf("ModeName(ModeByName(%q)) = %q, want the same name back", name, got)
				}
			}
		})
	}
}

// TestCapabilities_DVsMPIdentity is the matrix §4 claim made mechanical:
// the two models' capability sets are IDENTICAL except for the two
// model-conditional values, Model and CATID.
//
// Yaesu prints ONE CAT manual for the FTDX101MP and the FTDX101D and
// distinguishes them in exactly three places — the ID answer's P1 legend
// (layout 1070-1072), the P4 VALUE ranges of three MAX POWER rows in Table
// 2 (927, 928, 931), and the PC command's P1 range (1496, 1498). The second
// is menu-value semantics, which nothing here stores; the third is off this
// project's surface entirely (core/cat has no PC builder or parser). So the
// ID, and the Model string that names it, are the whole
// model-distinguishing surface of this driver's capability data.
//
// The comparison NORMALISES those two fields and then demands byte
// equality of everything else — banks, labels, NoBlank, every field-support
// map, modes, tag length, clarifier policy, tones, bauds, bounds, required
// slots and both vocabularies. A per-model divergence introduced anywhere
// else (a "corrected" MP-only frequency ceiling, a second mode table, a
// different PMS label) fails here with the field named, which is exactly
// the review this project would otherwise have to do by eye across two
// hundred lines of capability data.
//
// It is a pin on a CLAIM, not on evidence: if a Stage R capture ever shows
// the two radios genuinely differing in some capability, this test is
// rewritten with that finding, per model, in the same commit.
func TestCapabilities_DVsMPIdentity(t *testing.T) {
	normalise := func(caps spec.Capabilities) spec.Capabilities {
		caps.Model = "<model>"
		caps.CATID = "<catid>"
		return caps
	}

	for _, tt := range []struct {
		name string
		d    spec.Capabilities
		mp   spec.Capabilities
	}{
		{"Unverified", capabilitiesUnverified(modelD), capabilitiesUnverified(modelMP)},
		{"Simulated", capabilitiesSimulated(modelD), capabilitiesSimulated(modelMP)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// The two model-conditional values must actually DIFFER before
			// the rest is compared: a normalisation that hid an accidental
			// equality would turn this test into one that passed when both
			// constructors built the same radio.
			if tt.d.Model == tt.mp.Model || tt.d.CATID == tt.mp.CATID {
				t.Fatalf("D and MP report the same Model (%q/%q) or CATID (%q/%q) — the two model-conditional values must differ", tt.d.Model, tt.mp.Model, tt.d.CATID, tt.mp.CATID)
			}
			if !reflect.DeepEqual(normalise(tt.d), normalise(tt.mp)) {
				t.Errorf("the D's and the MP's capabilities differ in something OTHER than Model and CATID:\n D  = %#v\n MP = %#v\nmatrix §4 says the manual distinguishes the models in three places and only the ID answer touches a capability value — a genuine per-model difference needs that model's own evidence and a rewrite of this test", normalise(tt.d), normalise(tt.mp))
			}
		})
	}
}

// TestProfilesNeverEmitConsented: no capability PROFILE of either model
// mints spec.ConsentedUnverified. The state is a session-time statement
// about a user's recorded decision, so the only thing that may ever produce
// it is the consent transform at the assembly point — never a label written
// down in caps.go, where it would apply to every user of the model whether
// they consented or not.
func TestProfilesNeverEmitConsented(t *testing.T) {
	for _, m := range testModels {
		for _, tt := range []struct {
			name string
			caps spec.Capabilities
		}{
			{"Simulated", m.newDrv(Simulated).Capabilities()},
			{"RealHardware", m.newDrv(RealHardware).Capabilities()},
			{"unrecognised", m.newDrv(Profile(99)).Capabilities()},
		} {
			t.Run(m.name+"/"+tt.name, func(t *testing.T) {
				if capsContains(tt.caps, spec.ConsentedUnverified) {
					t.Error("a profile baseline carries ConsentedUnverified — consent belongs to a session, not to the radio's capability data")
				}
			})
		}
	}
}

// TestConsentOption_StaticCapabilitiesNeverConsented: the option changes
// what a SESSION carries and nothing else. A driver built with it still
// describes the radio exactly as one built without it does — for both
// models, since each constructor threads its own options.
//
// That boundary is load-bearing above this package: internal/wiring's
// registry publishes driver.Capabilities() and refuses a registered set
// carrying ConsentedUnverified on either side (core/driver's registry
// baseline guard), and the app's static surfaces — capability tables,
// settings descriptors, offline bank synthesis — describe the model rather
// than one user's decision.
func TestConsentOption_StaticCapabilitiesNeverConsented(t *testing.T) {
	for _, m := range testModels {
		for _, tt := range []struct {
			name string
			p    Profile
		}{
			{"Simulated", Simulated},
			{"RealHardware", RealHardware},
			{"unrecognised", Profile(99)},
		} {
			t.Run(m.name+"/"+tt.name, func(t *testing.T) {
				got := m.newDrv(tt.p, WithConsentedUnverifiedWrites()).Capabilities()
				if capsContains(got, spec.ConsentedUnverified) {
					t.Error("the consent option reached the STATIC capability set — it must apply only at session-capability assembly")
				}
				if !reflect.DeepEqual(got, m.newDrv(tt.p).Capabilities()) {
					t.Error("a consented driver's static Capabilities() differ from an unconsented one's")
				}
			})
		}
	}
}

// TestProfileRecognised_MatchesTheDeclaredConstants is the consent gate's
// DRIFT GUARD: profileRecognised must be true for exactly the Profile
// constants this package declares, and false for everything else.
//
// The dangerous direction is the one this test exists for. A profile the
// GATE recognised but Capabilities' switch did not would take the default
// arm's fail-safe all-Unverified set and then have the consent transform
// applied to it — fail-safe labels turned writable, which is the precise
// opposite of what the fail-safe is for. (The other direction merely
// withholds consent from a declared profile: unhelpful, not unsafe.)
//
// The two sides are restated in two places on purpose — profileRecognised's
// switch and Capabilities' switch — because there is no way in Go to derive
// one from the other for an open integer type, so a test is what holds them
// together. The sweep below deliberately includes the values NEXT to the
// declared ones (a constant added without a gate arm lands there), a
// negative, and the extremes.
func TestProfileRecognised_MatchesTheDeclaredConstants(t *testing.T) {
	declared := []struct {
		name string
		p    Profile
	}{
		{"RealHardware", RealHardware},
		{"Simulated", Simulated},
	}
	others := []Profile{
		-1, -2, 2, 3, 4, 7, 42, 99, 1000,
		Profile(math.MinInt), Profile(math.MaxInt),
	}

	for _, m := range testModels {
		for _, tt := range declared {
			t.Run(m.name+"/declared/"+tt.name, func(t *testing.T) {
				d, ok := m.newDrv(tt.p).(*ftdx101Driver)
				if !ok {
					t.Fatal("the constructor did not return a *ftdx101Driver")
				}
				if !d.profileRecognised() {
					t.Errorf("profileRecognised() = false for the declared constant %s — a declared profile must be able to receive consent", tt.name)
				}
			})
		}
		for _, p := range others {
			t.Run(fmt.Sprintf("%s/other/%d", m.name, int(p)), func(t *testing.T) {
				d, ok := m.newDrv(p).(*ftdx101Driver)
				if !ok {
					t.Fatal("the constructor did not return a *ftdx101Driver")
				}
				if d.profileRecognised() {
					t.Errorf("profileRecognised() = true for Profile(%d), which this package does not declare — Capabilities' switch would hand that profile the fail-safe set and the gate would then let consent make it writable", int(p))
				}
			})
		}
	}
}

// TestConsentTransform_SimulatedIsANoOp records — and pins — WHY this
// package's Simulated profile needs no consented-shape expectation of its
// own: it carries NO write-side spec.Unverified label at all. Every field
// the combined MT form can express is Write Supported against
// internal/fakedx101, and the four it cannot are the zero FieldSupport, so
// spec.ConsentUnverifiedWrites has nothing to convert and returns a
// deep-equal set.
//
// The FTdx10's Simulated profile has exactly this shape, which is why its
// task pinned only the RealHardware transformed shape too. Should a future
// Stage W finding ever label some Simulated write Unverified, this test
// fails — and the consented Simulated shape then needs pinning alongside
// the RealHardware one in TestConsentOption_SessionCapsTransformed.
//
// It runs on the static baseline, so it costs no Open.
func TestConsentTransform_SimulatedIsANoOp(t *testing.T) {
	for _, m := range testModels {
		t.Run(m.name, func(t *testing.T) {
			caps := capabilitiesSimulated(m.params)
			if capsContains(caps, spec.Unverified) {
				t.Fatal("the Simulated profile now carries an Unverified label — check whether it is WRITE-side, and if so pin the consented Simulated shape as well as the RealHardware one")
			}
			if got := spec.ConsentUnverifiedWrites(caps); !reflect.DeepEqual(got, caps) {
				t.Error("the consent transform CHANGED the Simulated baseline, which has no write-side Unverified to convert")
			}
		})
	}
}

// TestEffectiveCapabilities_Validate: every capability set a session can
// ever carry passes spec.Capabilities.Validate — both models × profiles ×
// discovered inventories × consent, assembled through the one seam that
// builds them (sessionCapabilities).
//
// TestProfiles_Validate covers the static baselines only, and the sets this
// driver actually hands out are strictly larger: they carry the discovered
// read-only banks, and now the consent transform's relabelling too. Validate
// is meaningful for a consented set in particular because its read-side rule
// refuses ConsentedUnverified outright, so a transform that leaked onto the
// read side fails HERE rather than at whatever layer first tried to enforce
// it.
//
// It calls sessionCapabilities DIRECTLY rather than opening sessions, and
// that is what makes a sweep this wide affordable at all: an Open in this
// package costs ~2-3 s and this table has 48 cells.
func TestEffectiveCapabilities_Validate(t *testing.T) {
	for _, m := range testModels {
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
					name := m.name + "/" + prof.name + "/" + disc.name
					if consent {
						name += "/consented"
					}
					t.Run(name, func(t *testing.T) {
						var opts []Option
						if consent {
							opts = append(opts, WithConsentedUnverifiedWrites())
						}
						d, ok := m.newDrv(prof.p, opts...).(*ftdx101Driver)
						if !ok {
							t.Fatal("the constructor did not return a *ftdx101Driver")
						}
						if err := d.sessionCapabilities(disc.slots60m, disc.emg).Validate(); err != nil {
							t.Errorf("Validate() = %v, want nil", err)
						}
					})
				}
			}
		}
	}
}
