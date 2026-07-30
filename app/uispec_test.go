// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/internal/fakedx10"
	"github.com/gm5dna/open-rig-programmer/internal/fakeradio"
	"github.com/gm5dna/open-rig-programmer/internal/radiotext"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// findBank returns the BankView with the given ID, or fails the test.
func findBank(t *testing.T, banks []BankView, id string) BankView {
	t.Helper()
	for _, b := range banks {
		if b.ID == id {
			return b
		}
	}
	t.Fatalf("no bank with ID %q in %v", id, bankIDs(banks))
	return BankView{}
}

func bankIDs(banks []BankView) []string {
	out := make([]string, len(banks))
	for i, b := range banks {
		out[i] = b.ID
	}
	return out
}

func slotSet(slots []SlotView) map[string]string {
	out := make(map[string]string, len(slots))
	for _, s := range slots {
		out[s.Slot] = s.Display
	}
	return out
}

// TestBankReadOnly_Table is a pure unit test of bankReadOnly against
// hand-built spec.Capabilities, independent of App/session plumbing —
// table-driven per the task's TDD requirement.
//
// The fixtures are built over bankCoreCandidates (M9c-6 D5a: the CANDIDATE
// universe, from which each bank's core set is now DERIVED) rather than
// over a fixed field list. Every case's expectation is unchanged by that
// derivation, deliberately: a bank whose candidates are uniformly
// supported/unverified/unsupported derives the same verdict either way,
// and the cases that DO distinguish the two rules are
// TestBankCoreFields_* below.
func TestBankReadOnly_Table(t *testing.T) {
	rw := spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	unverified := spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}
	unsupported := spec.FieldSupport{}

	allWritable := map[spec.Field]spec.FieldSupport{}
	allUnverified := map[spec.Field]spec.FieldSupport{}
	allUnsupported := map[spec.Field]spec.FieldSupport{}
	mixedOneWritable := map[spec.Field]spec.FieldSupport{}
	inertClarifier := map[spec.Field]spec.FieldSupport{}
	for _, f := range bankCoreCandidates {
		allWritable[f] = rw
		allUnverified[f] = unverified
		allUnsupported[f] = unsupported
		mixedOneWritable[f] = unsupported
		inertClarifier[f] = rw
	}
	mixedOneWritable[spec.FieldFrequency] = rw
	// The M5b real shape: every core field writable except the clarifier,
	// whose Write is Inert (HW-CONFIRMED transmitted-but-ignored). Inert
	// is not Unsupported, so the bank — and with it the grid's clarifier
	// column — stays editable; a CHANGED clarifier is caught at send time
	// by codeplug.Diff's Inert gate, not by locking the cell.
	inertClarifier[spec.FieldClarifier] = spec.FieldSupport{Read: spec.Supported, Write: spec.Inert}

	tests := []struct {
		name   string
		fields map[spec.Field]spec.FieldSupport
		want   bool
	}{
		{"all Supported -> not read-only", allWritable, false},
		{"all Unverified -> not read-only (awaiting hardware trials, not locked)", allUnverified, false},
		{"all Unsupported -> read-only", allUnsupported, true},
		{"one field writable among Unsupported rest -> not read-only", mixedOneWritable, false},
		{"Inert clarifier among Supported rest -> not read-only (M5b real shape; clarifier column stays editable)", inertClarifier, false},
		{"absent bank entirely -> vacuously read-only (zero FieldSupport everywhere)", nil, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			caps := spec.Capabilities{}
			if tc.fields != nil {
				caps.Banks = []spec.Bank{{ID: "TEST", Fields: tc.fields}}
			}
			got := bankReadOnly(caps, "TEST")
			if got != tc.want {
				t.Errorf("bankReadOnly() = %v, want %v", got, tc.want)
			}
		})
	}
}

// fieldSet reduces a derived core-field list to a set, for membership
// assertions that must not depend on order.
func fieldSet(fields []spec.Field) map[spec.Field]bool {
	out := make(map[spec.Field]bool, len(fields))
	for _, f := range fields {
		out[f] = true
	}
	return out
}

// wantFields fails t unless got's membership is exactly want's, naming
// every missing and every unexpected field (membership is the acceptance
// criterion M9c-6 D5a settled on — a count would pass on a swap).
func wantFields(t *testing.T, context string, got, want []spec.Field) {
	t.Helper()
	gotSet, wantSet := fieldSet(got), fieldSet(want)
	for f := range wantSet {
		if !gotSet[f] {
			t.Errorf("%s: derived core set is MISSING %q; got %v, want exactly %v", context, f, got, want)
		}
	}
	for f := range gotSet {
		if !wantSet[f] {
			t.Errorf("%s: derived core set unexpectedly CONTAINS %q; got %v, want exactly %v", context, f, got, want)
		}
	}
}

// ft710CoreSeven is the core set every FT-710 bank derives, on every
// profile: the seven fields its memory frame carries. Tone and scan skip
// are absent because that radio's CAT protocol reaches neither (its own
// bankFields zeroes both); erase is absent structurally.
var ft710CoreSeven = []spec.Field{
	spec.FieldFrequency, spec.FieldMode, spec.FieldClarifier,
	spec.FieldShift, spec.FieldCTCSSState, spec.FieldTag, spec.FieldTagDisplay,
}

// ftdx10CoreSix is the core set every FTdx10 bank derives, on every
// profile: ft710CoreSeven minus tag_display, whose flag that radio's
// combined MT record has no room for at all.
var ftdx10CoreSix = []spec.Field{
	spec.FieldFrequency, spec.FieldMode, spec.FieldClarifier,
	spec.FieldShift, spec.FieldCTCSSState, spec.FieldTag,
}

// TestBankCoreFields_ExcludesEraseStructurally is M9c-6 D5a's structural
// exclusion, and the case that shows why the zero-value test alone could
// not carry it: spec.FieldErase is NON-zero on the FT-710's own fail-safe
// profile, where MEM erase is {Read: Unsupported, Write: Unverified}
// (core/driver/ft710/caps.go's CapabilitiesUnverified). A derivation that
// admitted every non-zero field would therefore re-admit erase on exactly
// that profile — and, worse, would then report the bank EDITABLE on the
// strength of an erase support, since Unverified is not Unsupported.
//
// The fixture is that profile's MEM shape, hand-built: this package must
// not import a concrete driver (the M9a-5 composition discipline
// internal/guards pins), and what the assertion needs is the SHAPE, not
// the driver's own value.
func TestBankCoreFields_ExcludesEraseStructurally(t *testing.T) {
	unverified := spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}
	fields := map[spec.Field]spec.FieldSupport{
		spec.FieldFrequency:  unverified,
		spec.FieldMode:       unverified,
		spec.FieldClarifier:  {Read: spec.Unverified, Write: spec.Inert},
		spec.FieldCTCSSState: unverified,
		spec.FieldShift:      unverified,
		spec.FieldTag:        unverified,
		spec.FieldTagDisplay: unverified,
		spec.FieldCTCSSTone:  {},
		spec.FieldScanSkip:   {},
		// The FT-710 fail-safe profile's MEM erase — non-zero, and the
		// whole point of this test.
		spec.FieldErase: {Read: spec.Unsupported, Write: spec.Unverified},
	}
	caps := spec.Capabilities{Banks: []spec.Bank{{ID: "TEST", Fields: fields}}}
	wantFields(t, "the FT-710 fail-safe profile's MEM shape", bankCoreFields(caps, "TEST"), ft710CoreSeven)

	// And the same fixture with every GRID field zeroed: erase alone is
	// non-zero and write-Unverified, and the bank must still be read-only.
	// Before D5a this fell out of a fixed list that never named erase;
	// now it falls out of the structural exclusion, and the assertion is
	// what stops the two ever being confused.
	eraseOnly := map[spec.Field]spec.FieldSupport{spec.FieldErase: {Read: spec.Unsupported, Write: spec.Unverified}}
	eraseCaps := spec.Capabilities{Banks: []spec.Bank{{ID: "TEST", Fields: eraseOnly}}}
	if got := bankCoreFields(eraseCaps, "TEST"); len(got) != 0 {
		t.Errorf("erase-only bank derived %v, want an empty core set", got)
	}
	if !bankReadOnly(eraseCaps, "TEST") {
		t.Error("bankReadOnly(erase-only bank) = false, want true — erase is not a grid column and can never make one editable")
	}
}

// TestBankCoreFields_WritableToneIsLoadBearing is M9c-6 D5a's proof that
// the derivation is not decorative. A radio whose memory frame DID carry a
// tone number would have tone in its core set — eight fields, not the
// FT-710's seven — and a bank of that radio on which ONLY the tone is
// writable must report EDITABLE, where the pre-D5a fixed list (which never
// consulted tone at all) called it read-only.
//
// This is the direction the old list could not express, and the one that
// matters most: it would have locked a whole bank of a future radio out of
// the grid on the strength of a comment about the FT-710's protocol.
func TestBankCoreFields_WritableToneIsLoadBearing(t *testing.T) {
	rw := spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	readOnly := spec.FieldSupport{Read: spec.Supported, Write: spec.Unsupported}
	fields := map[spec.Field]spec.FieldSupport{
		spec.FieldFrequency:  readOnly,
		spec.FieldMode:       readOnly,
		spec.FieldClarifier:  readOnly,
		spec.FieldCTCSSState: readOnly,
		spec.FieldShift:      readOnly,
		spec.FieldTag:        readOnly,
		spec.FieldTagDisplay: readOnly,
		// The one writable field, and one the FT-710 could never carry.
		spec.FieldCTCSSTone: rw,
		spec.FieldScanSkip:  {},
	}
	caps := spec.Capabilities{Banks: []spec.Bank{{ID: "TEST", Fields: fields}}}

	wantEight := append(append([]spec.Field(nil), ft710CoreSeven...), spec.FieldCTCSSTone)
	wantFields(t, "a radio whose frame carries a tone number", bankCoreFields(caps, "TEST"), wantEight)
	if bankReadOnly(caps, "TEST") {
		t.Error("bankReadOnly() = true, want false — the tone is writable on this bank, and the grid renders it as an editable column")
	}
}

// TestBankCoreFields_ZeroValueDecidesMembership is the inclusion rule
// itself, one support shape at a time: NON-ZERO means "this radio's frame
// carries the field here", to any degree of confidence and in either
// direction, and only the zero FieldSupport — declared zero, field absent
// from the bank, or bank absent from caps — excludes.
func TestBankCoreFields_ZeroValueDecidesMembership(t *testing.T) {
	tests := []struct {
		name    string
		support spec.FieldSupport
		want    bool
	}{
		{"Read+Write Supported", spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}, true},
		{"Read+Write Unverified (documented, unproven — still a field)", spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}, true},
		{"readable, write Unsupported (the discovered 60M/EMG shape)", spec.FieldSupport{Read: spec.Supported, Write: spec.Unsupported}, true},
		{"writable, read Unsupported", spec.FieldSupport{Read: spec.Unsupported, Write: spec.Supported}, true},
		{"Inert write (transmitted-but-ignored is still a frame field)", spec.FieldSupport{Read: spec.Unsupported, Write: spec.Inert}, true},
		{"the zero FieldSupport", spec.FieldSupport{}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			caps := spec.Capabilities{Banks: []spec.Bank{{ID: "TEST", Fields: map[spec.Field]spec.FieldSupport{
				spec.FieldTagDisplay: tc.support,
			}}}}
			got := fieldSet(bankCoreFields(caps, "TEST"))[spec.FieldTagDisplay]
			if got != tc.want {
				t.Errorf("tag_display in derived core set = %v, want %v", got, tc.want)
			}
		})
	}

	absent := spec.Capabilities{Banks: []spec.Bank{{ID: "TEST", Fields: map[spec.Field]spec.FieldSupport{
		spec.FieldFrequency: {Read: spec.Supported, Write: spec.Supported},
	}}}}
	wantFields(t, "a bank listing only frequency", bankCoreFields(absent, "TEST"), []spec.Field{spec.FieldFrequency})
	if got := bankCoreFields(spec.Capabilities{}, "NOSUCHBANK"); len(got) != 0 {
		t.Errorf("bankCoreFields(absent bank) = %v, want an empty core set", got)
	}
}

// registeredProfileCaps returns every capability profile of model that is
// REACHABLE THROUGH REAL REGISTRATION, keyed by a name for subtest output:
// the static baseline internal/wiring serves for the real-hardware driver,
// and the effective capabilities of a session opened against that model's
// own registered fake (the Simulated profile, plus whatever inventory
// discovery found).
//
// Registration, not construction, is the point: these are the capability
// values the GUI can actually be handed, obtained the way GetUISpec obtains
// them, so a model registered with the wrong profile — or a profile whose
// field map drifted — shows up here rather than in a hand-copied fixture
// that would drift with it. A model's remaining profiles (the FT-710's
// fail-safe CapabilitiesUnverified, reachable only by constructing the
// driver with an invalid Profile) cannot be reached from this package,
// which imports no concrete driver by design; its SHAPE is covered by
// TestBankCoreFields_ExcludesEraseStructurally.
func registeredProfileCaps(t *testing.T, model string) map[string]spec.Capabilities {
	t.Helper()
	static, err := wiring.StaticCapabilities(model)
	if err != nil {
		t.Fatalf("wiring.StaticCapabilities(%q): unexpected error: %v", model, err)
	}
	sess, closeAll, err := wiring.OpenFakeSessionFor(testAppCtx(t), model)
	if err != nil {
		t.Fatalf("wiring.OpenFakeSessionFor(%q): unexpected error: %v", model, err)
	}
	t.Cleanup(func() { _ = closeAll() })
	return map[string]spec.Capabilities{
		"RealHardware (wiring.StaticCapabilities)": static,
		"Simulated (registered fake session)":      sess.Capabilities(),
	}
}

// TestBankCoreFields_EveryRegisteredModel_Membership is M9c-6 D5a's
// acceptance, stated as MEMBERSHIP per model, per reachable profile, per
// bank — never a count, which a swapped pair would satisfy.
//
// It walks wiring.SupportedModels() and fails on a model it has no
// expectation for, so registering a third radio cannot leave this test
// silently vacuous: whoever adds it must state that radio's core set here,
// which is the moment to notice if it is a surprising one.
func TestBankCoreFields_EveryRegisteredModel_Membership(t *testing.T) {
	want := map[string][]spec.Field{
		"FT-710": ft710CoreSeven,
		"FTdx10": ftdx10CoreSix,
	}
	models := wiring.SupportedModels()
	if len(models) == 0 {
		t.Fatal("wiring.SupportedModels() is empty — this test would assert nothing")
	}
	for _, model := range models {
		wantSet, ok := want[model]
		if !ok {
			t.Errorf("model %q is registered but has no expected core set here — state it (and check it is the honest one) rather than deleting this failure", model)
			continue
		}
		t.Run(model, func(t *testing.T) {
			for profile, caps := range registeredProfileCaps(t, model) {
				if len(caps.Banks) == 0 {
					t.Fatalf("%s: no banks — nothing asserted", profile)
				}
				for _, b := range caps.Banks {
					wantFields(t, model+" "+profile+" bank "+string(b.ID), bankCoreFields(caps, b.ID), wantSet)
				}
			}
		})
	}
}

// TestBankReadOnly_RegisteredFTdx10_RealHardwareProfile pins what a REAL
// FTdx10's grid does today, bank by bank, through real registration.
//
// The FTdx10's RealHardware profile is its all-Unverified one
// (writeTrialsComplete is false: no FTdx10 has ever been written to by
// this project), so its six derived fields are Write spec.Unverified on
// MEM and PMS — which is NOT spec.Unsupported, and therefore NOT read-only
// under bankReadOnly's standing rule. Those two banks stay EDITABLE, and
// every write is refused later, at the capability gate, exactly as the
// FT-710's were between M5a and the M5b trials that unlocked them: the
// offline clone workflow (read, edit, save a file) is the reason that rule
// exists, and it is as valuable for an unproven radio as it was for a
// proven one.
//
// The milestone spec's D5a asserts the opposite consequence ("bankReadOnly
// is TRUE for a real FTdx10 — a read-only grid pre-trials is CORRECT").
// That sentence does not follow from D5a's own rule, which changes the
// candidate SET and not the Unsupported test; making it true would mean
// re-testing on FieldSupport.CanWrite() and reversing a documented
// adjudication for every radio. See bankReadOnly's doc comment. This test
// therefore pins the OBSERVED verdicts, so that whichever way the project
// adjudicates it, the change is a visible edit here and not a drift.
//
// The discovered 5 MHz bank is the contrast that keeps the test honest:
// its Writes ARE forced Unsupported (no profile may claim a 5xx slot
// writable), so it derives the same six fields and reports read-only true
// — one capability set, two different verdicts, from one rule.
func TestBankReadOnly_RegisteredFTdx10_RealHardwareProfile(t *testing.T) {
	caps, err := wiring.StaticCapabilities("FTdx10")
	if err != nil {
		t.Fatalf("wiring.StaticCapabilities(\"FTdx10\"): unexpected error: %v", err)
	}
	if len(caps.Banks) == 0 {
		t.Fatal("the registered FTdx10's static baseline has no banks — nothing asserted")
	}
	for _, b := range caps.Banks {
		for _, f := range bankCoreFields(caps, b.ID) {
			if got := caps.FieldSupport(b.ID, f).Write; got != spec.Unverified {
				t.Errorf("bank %s field %s Write = %v, want Unverified (the premise: nothing on a real FTdx10 is proven writable)", b.ID, f, got)
			}
		}
		if bankReadOnly(caps, b.ID) {
			t.Errorf("bankReadOnly(%s) = true, want false — Unverified is not Unsupported, and locking it would break the offline clone workflow", b.ID)
		}
	}

	// A discovered 5 MHz bank, whose Writes ARE Unsupported: read-only.
	prev := wiring.FTdx10FakeSessionOpts
	wiring.FTdx10FakeSessionOpts = []fakedx10.Option{fakedx10.With5xx()}
	t.Cleanup(func() { wiring.FTdx10FakeSessionOpts = prev })
	sess, closeAll, err := wiring.OpenFakeSessionFor(testAppCtx(t), "FTdx10")
	if err != nil {
		t.Fatalf("wiring.OpenFakeSessionFor(\"FTdx10\"): unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = closeAll() })
	live := sess.Capabilities()
	if _, ok := live.Bank(spec.Bank60m); !ok {
		t.Fatal("the 5xx-populated fake produced no 60M bank — the contrast half of this test would be vacuous")
	}
	wantFields(t, "the discovered 60M bank", bankCoreFields(live, spec.Bank60m), ftdx10CoreSix)
	if !bankReadOnly(live, spec.Bank60m) {
		t.Error("bankReadOnly(60M) = false, want true — no profile may claim a discovered 5xx slot writable")
	}
}

// TestBankTagDisplayDefault_Table is a pure unit test of
// bankTagDisplayDefault against hand-built spec.Capabilities, independent
// of App/session plumbing: the ONE trigger for Unavailable is BOTH
// directions Unsupported, and every other combination — including the
// write-Unsupported-but-readable shape the discovered 60M/EMG banks
// actually carry — is a Known-false blank-row default.
func TestBankTagDisplayDefault_Table(t *testing.T) {
	known := codeplug.BoolField{State: codeplug.Known, Value: false}
	unavailable := codeplug.BoolField{State: codeplug.Unavailable}

	tests := []struct {
		name   string
		fields map[spec.Field]spec.FieldSupport
		want   codeplug.BoolField
	}{
		{"Read and Write Supported -> Known-false", map[spec.Field]spec.FieldSupport{
			spec.FieldTagDisplay: {Read: spec.Supported, Write: spec.Supported},
		}, known},
		{"readable, write Unsupported (the discovered 60M/EMG shape) -> Known-false", map[spec.Field]spec.FieldSupport{
			spec.FieldTagDisplay: {Read: spec.Supported, Write: spec.Unsupported},
		}, known},
		{"writable, read Unsupported -> Known-false", map[spec.Field]spec.FieldSupport{
			spec.FieldTagDisplay: {Read: spec.Unsupported, Write: spec.Supported},
		}, known},
		{"Unverified both ways -> Known-false (not yet proven is not absent)", map[spec.Field]spec.FieldSupport{
			spec.FieldTagDisplay: {Read: spec.Unverified, Write: spec.Unverified},
		}, known},
		{"Inert write -> Known-false (transmitted-but-ignored is still a frame field)", map[spec.Field]spec.FieldSupport{
			spec.FieldTagDisplay: {Read: spec.Supported, Write: spec.Inert},
		}, known},
		{"both Unsupported -> Unavailable", map[spec.Field]spec.FieldSupport{
			spec.FieldTagDisplay: {},
		}, unavailable},
		{"field absent from a present bank -> Unavailable", map[spec.Field]spec.FieldSupport{
			spec.FieldFrequency: {Read: spec.Supported, Write: spec.Supported},
		}, unavailable},
		{"absent bank entirely -> Unavailable", nil, unavailable},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			caps := spec.Capabilities{}
			if tc.fields != nil {
				caps.Banks = []spec.Bank{{ID: "TEST", Fields: tc.fields}}
			}
			got := bankTagDisplayDefault(caps, "TEST")
			if got != tc.want {
				t.Errorf("bankTagDisplayDefault() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestGetUISpec_TagDisplayDefaultFollowsCapabilities is W1's Unavailable
// shape, served through the whole GetUISpec path rather than the helper
// alone: a model whose one bank's FieldTagDisplay is Unsupported in BOTH
// directions must hand the grid {state:"unavailable"}, so a blank row
// created there never manufactures a Known value for a flag that radio's
// frame does not carry. The second bank — identical but for a
// Read/Write-Supported FieldTagDisplay — is the contrast that stops this
// passing vacuously: one UISpecView, two different per-bank defaults.
//
// The FT-710's own (Known-false) side of the pair is asserted against the
// real static baseline in TestGetUISpec_Disconnected_StaticBaseline and
// against live discovered banks in TestGetUISpec_ConnectedSimulated. What
// the capsForModel seam buys here is the CONTRAST — two banks of ONE radio
// disagreeing — which no registered model provides: the FTdx10 (registered
// since M9c-6, and the first real Unavailable producer) answers Unavailable
// on every bank it has, and is asserted doing so, end to end and without
// any seam, by TestGetUISpec_RegisteredFTdx10_EveryBankUnavailable.
func TestGetUISpec_TagDisplayDefaultFollowsCapabilities(t *testing.T) {
	rw := spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	fieldsWith := func(tagDisplay spec.FieldSupport) map[spec.Field]spec.FieldSupport {
		m := make(map[spec.Field]spec.FieldSupport, len(bankCoreCandidates))
		for _, f := range bankCoreCandidates {
			m[f] = rw
		}
		m[spec.FieldTagDisplay] = tagDisplay
		return m
	}
	recogniseModelCaps(t, spec.Capabilities{
		Model: testModel, CATID: "9999", TagLen: 42,
		Banks: []spec.Bank{
			{ID: "FLAG", Label: "Flag readable and writable", Slots: []string{"001"}, Fields: fieldsWith(rw)},
			{ID: "NOFLAG", Label: "No display flag in the frame", Slots: []string{"002"}, Fields: fieldsWith(spec.FieldSupport{})},
		},
	})

	a, _ := newTestApp(t)
	a.mu.Lock()
	a.working = &codeplug.Codeplug{
		Schema:   codeplug.CurrentSchema,
		Radio:    codeplug.RadioInfo{Model: testModel},
		Channels: []codeplug.Channel{{Slot: "001"}, {Slot: "002"}},
	}
	a.mu.Unlock()

	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec: unexpected error: %v", err)
	}
	if flag := findBank(t, got.Banks, "FLAG"); flag.TagDisplayDefault != (codeplug.BoolField{State: codeplug.Known, Value: false}) {
		t.Errorf("FLAG.TagDisplayDefault = %+v, want {known false}", flag.TagDisplayDefault)
	}
	noflag := findBank(t, got.Banks, "NOFLAG")
	if noflag.TagDisplayDefault != (codeplug.BoolField{State: codeplug.Unavailable}) {
		t.Errorf("NOFLAG.TagDisplayDefault = %+v, want {unavailable false} — the grid must never invent a Known value for a flag this radio's frame has no room for", noflag.TagDisplayDefault)
	}
}

// TestGetUISpec_RegisteredFTdx10_EveryBankUnavailable is M9c-6 D5c — the
// end-to-end acceptance test for the whole E1 chain, and the first time
// this project's Unavailable state is produced by a REAL radio's real
// capability data rather than by a test fixture.
//
// GetUISpec is driven for model FTdx10 through real registration, twice,
// because the two paths reach bankTagDisplayDefault with different
// capability values and the grid must get the same answer from both:
//
//   - CONNECTED to the registered fake (Live true, the Simulated profile
//     plus discovery's own inventory — a populated 5 MHz bank here, so
//     "every bank" spans a discovered one too). This is the `--fake
//     --model FTdx10` path a user actually walks.
//   - DISCONNECTED with an FTdx10 working copy loaded (Live false, the
//     static RealHardware baseline, resolved by currentModel from the
//     file's own Radio.Model). This is the offline clone workflow's path.
//
// Every bank of both must serve {state: "unavailable"}: the FTdx10's
// combined MT record has no display flag at all, so a blank row added
// anywhere in that grid must not carry a Known one. The FT-710 assertion
// at the end is the contrast that stops the whole thing passing because
// something returned a zero value — one call each, two registered radios,
// two different answers, no seam and no fixture in either.
func TestGetUISpec_RegisteredFTdx10_EveryBankUnavailable(t *testing.T) {
	unavailable := codeplug.BoolField{State: codeplug.Unavailable}

	prev := wiring.FTdx10FakeSessionOpts
	wiring.FTdx10FakeSessionOpts = []fakedx10.Option{fakedx10.With5xx()}
	t.Cleanup(func() { wiring.FTdx10FakeSessionOpts = prev })
	sess, closeAll, err := wiring.OpenFakeSessionFor(testAppCtx(t), "FTdx10")
	if err != nil {
		t.Fatalf("wiring.OpenFakeSessionFor(\"FTdx10\"): unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = closeAll() })

	a, _ := newTestApp(t)
	connectDirect(t, a, sess, nil)
	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec (connected to the FTdx10 fake): unexpected error: %v", err)
	}
	if !got.Live {
		t.Error("Live = false, want true (connected to the registered fake)")
	}
	if len(got.Banks) < 3 {
		t.Fatalf("banks = %v, want MEM, PMS and the discovered 60M — 'every bank' must span a discovered one", bankIDs(got.Banks))
	}
	for _, b := range got.Banks {
		if b.TagDisplayDefault != unavailable {
			t.Errorf("connected FTdx10 bank %s TagDisplayDefault = %+v, want %+v — this radio's memory frame has no display flag", b.ID, b.TagDisplayDefault, unavailable)
		}
	}

	// Offline, from an FTdx10 file: the same answer, from the static
	// RealHardware baseline this time.
	a.mu.Lock()
	a.conn = nil
	a.working = &codeplug.Codeplug{
		Schema:   codeplug.CurrentSchema,
		Radio:    codeplug.RadioInfo{Model: "FTdx10"},
		Channels: []codeplug.Channel{{Slot: "001"}},
	}
	a.mu.Unlock()
	offline, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec (offline, FTdx10 working copy): unexpected error: %v", err)
	}
	if offline.Live {
		t.Error("Live = true, want false (disconnected)")
	}
	if len(offline.Banks) == 0 {
		t.Fatal("offline FTdx10 UISpec has no banks — nothing asserted")
	}
	for _, b := range offline.Banks {
		if b.TagDisplayDefault != unavailable {
			t.Errorf("offline FTdx10 bank %s TagDisplayDefault = %+v, want %+v", b.ID, b.TagDisplayDefault, unavailable)
		}
	}

	// The contrast: the FT-710, through the same offline path, still
	// answers Known-false on every bank.
	a.mu.Lock()
	a.working = &codeplug.Codeplug{
		Schema:   codeplug.CurrentSchema,
		Radio:    codeplug.RadioInfo{Model: wiring.DefaultModel},
		Channels: []codeplug.Channel{{Slot: "001"}},
	}
	a.mu.Unlock()
	ft710, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec (offline, FT-710 working copy): unexpected error: %v", err)
	}
	if len(ft710.Banks) == 0 {
		t.Fatal("offline FT-710 UISpec has no banks — the contrast would be vacuous")
	}
	knownOff := codeplug.BoolField{State: codeplug.Known, Value: false}
	for _, b := range ft710.Banks {
		if b.TagDisplayDefault != knownOff {
			t.Errorf("offline FT-710 bank %s TagDisplayDefault = %+v, want %+v", b.ID, b.TagDisplayDefault, knownOff)
		}
	}
}

// TestGetUISpec_Disconnected_StaticBaseline pins the offline/no-session
// shape: Live false, only MEM/PMS banks (the static baseline never
// carries 60M/EMG), both editable, slot lists exactly the static bank
// definitions, vocab/tone/limit fields straight from the static caps.
func TestGetUISpec_Disconnected_StaticBaseline(t *testing.T) {
	a, _ := newTestApp(t)

	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec: unexpected error: %v", err)
	}
	if got.Live {
		t.Error("Live = true while disconnected, want false")
	}

	staticCaps, err := wiring.StaticCapabilities(wiring.DefaultModel)
	if err != nil {
		t.Fatalf("wiring.StaticCapabilities(%q): unexpected error: %v", wiring.DefaultModel, err)
	}

	if len(got.Banks) != 2 {
		t.Fatalf("len(Banks) = %d, want 2 (MEM, PMS only — static baseline has no 60M/EMG); got IDs %v", len(got.Banks), bankIDs(got.Banks))
	}

	mem := findBank(t, got.Banks, "MEM")
	if mem.ReadOnly {
		t.Error("MEM.ReadOnly = true while disconnected (static caps), want false — Unverified must not lock the grid")
	}
	pms := findBank(t, got.Banks, "PMS")
	if pms.ReadOnly {
		t.Error("PMS.ReadOnly = true while disconnected (static caps), want false — Unverified must not lock the grid")
	}

	// The FT-710's own blank-row default (W1's first shape, from the REAL
	// static literals, not a fixture): the CAT protocol reads and writes
	// this radio's display flag on both banks, so a row added there is
	// Known-off — the value the grid used to hardcode in JS.
	wantKnownOff := codeplug.BoolField{State: codeplug.Known, Value: false}
	if mem.TagDisplayDefault != wantKnownOff {
		t.Errorf("MEM.TagDisplayDefault = %+v, want %+v", mem.TagDisplayDefault, wantKnownOff)
	}
	if pms.TagDisplayDefault != wantKnownOff {
		t.Errorf("PMS.TagDisplayDefault = %+v, want %+v", pms.TagDisplayDefault, wantKnownOff)
	}

	memBank, _ := staticCaps.Bank(spec.BankMemory)
	if len(mem.Slots) != len(memBank.Slots) {
		t.Fatalf("len(MEM.Slots) = %d, want %d", len(mem.Slots), len(memBank.Slots))
	}
	memSlots := slotSet(mem.Slots)
	if disp, ok := memSlots["001"]; !ok || disp != "M-01" {
		t.Errorf("MEM.Slots[\"001\"].Display = %q, ok=%v, want \"M-01\"", disp, ok)
	}
	if disp, ok := memSlots["099"]; !ok || disp != "M-99" {
		t.Errorf("MEM.Slots[\"099\"].Display = %q, ok=%v, want \"M-99\"", disp, ok)
	}

	pmsBank, _ := staticCaps.Bank(spec.BankPMS)
	if len(pms.Slots) != len(pmsBank.Slots) {
		t.Fatalf("len(PMS.Slots) = %d, want %d", len(pms.Slots), len(pmsBank.Slots))
	}
	pmsSlots := slotSet(pms.Slots)
	if disp, ok := pmsSlots["P1L"]; !ok || disp != "P1L" {
		t.Errorf("PMS.Slots[\"P1L\"].Display = %q, ok=%v, want \"P1L\" (unchanged — not the M-/5- pattern)", disp, ok)
	}

	if len(got.ShiftOptions) != 3 || got.ShiftOptions[0] != "SIMPLEX" || got.ShiftOptions[1] != "PLUS" || got.ShiftOptions[2] != "MINUS" {
		t.Errorf("ShiftOptions = %v, want [SIMPLEX PLUS MINUS]", got.ShiftOptions)
	}
	if len(got.CTCSSStateOptions) != 3 || got.CTCSSStateOptions[0] != "OFF" || got.CTCSSStateOptions[1] != "ENC-DEC" || got.CTCSSStateOptions[2] != "ENC" {
		t.Errorf("CTCSSStateOptions = %v, want [OFF ENC-DEC ENC]", got.CTCSSStateOptions)
	}
	if len(got.Modes) != len(staticCaps.Modes) {
		t.Errorf("len(Modes) = %d, want %d", len(got.Modes), len(staticCaps.Modes))
	}
	if got.TagMaxBytes != staticCaps.TagLen {
		t.Errorf("TagMaxBytes = %d, want %d", got.TagMaxBytes, staticCaps.TagLen)
	}
	if got.ClarMaxHz != staticCaps.ClarMaxHz {
		t.Errorf("ClarMaxHz = %d, want %d", got.ClarMaxHz, staticCaps.ClarMaxHz)
	}
	if got.ClarStepHz != staticCaps.ClarStepHz {
		t.Errorf("ClarStepHz = %d, want %d", got.ClarStepHz, staticCaps.ClarStepHz)
	}
	if len(got.Tones) != len(staticCaps.CTCSSTones) {
		t.Fatalf("len(Tones) = %d, want %d", len(got.Tones), len(staticCaps.CTCSSTones))
	}
	if got.Tones[0].Decihertz != 670 || got.Tones[0].Display != "67.0 Hz" {
		t.Errorf("Tones[0] = %+v, want {670 \"67.0 Hz\"}", got.Tones[0])
	}
}

// TestGetUISpec_ConnectedSimulated pins the connected/live shape against
// a real simulated session (openTestSimSession) over fakeradio with
// ImageUS (so EMG is present alongside the full 15-channel 60m set):
// Live true, four banks, MEM/PMS editable, 60M/EMG read-only, and their
// slot counts matching the discovered inventory.
func TestGetUISpec_ConnectedSimulated(t *testing.T) {
	a, _ := newTestApp(t)
	sess := openTestSimSession(t, fakeradio.WithFactoryImage(fakeradio.ImageUS))
	connectDirect(t, a, sess, nil)

	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec: unexpected error: %v", err)
	}
	if !got.Live {
		t.Error("Live = false while connected, want true")
	}
	if len(got.Banks) != 4 {
		t.Fatalf("len(Banks) = %d, want 4 (MEM, PMS, 60M, EMG); got IDs %v", len(got.Banks), bankIDs(got.Banks))
	}

	mem := findBank(t, got.Banks, "MEM")
	if mem.ReadOnly {
		t.Error("MEM.ReadOnly = true while connected (Simulated), want false")
	}
	pms := findBank(t, got.Banks, "PMS")
	if pms.ReadOnly {
		t.Error("PMS.ReadOnly = true while connected (Simulated), want false")
	}
	sixty := findBank(t, got.Banks, "60M")
	if !sixty.ReadOnly {
		t.Error("60M.ReadOnly = false while connected, want true (discovered banks are always write-Unsupported)")
	}
	if len(sixty.Slots) != 15 {
		t.Errorf("len(60M.Slots) = %d, want 15 (ImageUS)", len(sixty.Slots))
	}
	// Label pins shared with the offline-synthesis test
	// (TestGetUISpec_OfflineWorkingCopy_Synthesises60mEMGBanks): both
	// sides assert the same literal, so if either the driver's discovered
	// -bank labels or app/'s synthesised ones drift, a test fails.
	if sixty.Label != "60 m channels" {
		t.Errorf("60M.Label = %q, want %q", sixty.Label, "60 m channels")
	}
	emg := findBank(t, got.Banks, "EMG")
	if !emg.ReadOnly {
		t.Error("EMG.ReadOnly = false while connected, want true")
	}
	if emg.Label != "Emergency (EMG)" {
		t.Errorf("EMG.Label = %q, want %q", emg.Label, "Emergency (EMG)")
	}
	if len(emg.Slots) != 1 {
		t.Errorf("len(EMG.Slots) = %d, want 1", len(emg.Slots))
	}
	emgSlots := slotSet(emg.Slots)
	if disp, ok := emgSlots["EMG"]; !ok || disp != "EMG" {
		t.Errorf("EMG.Slots[\"EMG\"].Display = %q, ok=%v, want \"EMG\"", disp, ok)
	}

	// Every bank, discovered ones included, carries the Known-false
	// blank-row default on this radio: the 60M/EMG field maps mirror MEM's
	// READ supports with Write forced Unsupported
	// (core/driver/ft710.effectiveCapabilities), and read-Supported alone
	// is enough — the flag exists in the frame, it is only unwritable
	// there. Pinned here as well as offline because a live session's caps
	// and the offline synthesis are different code paths that must agree.
	wantKnownOff := codeplug.BoolField{State: codeplug.Known, Value: false}
	for _, b := range got.Banks {
		if b.TagDisplayDefault != wantKnownOff {
			t.Errorf("%s.TagDisplayDefault = %+v, want %+v", b.ID, b.TagDisplayDefault, wantKnownOff)
		}
	}
}

// TestGetUISpec_SlotClassification_OfflineWorkingCopy pins the "offline
// with a working copy" branch: slots come from the WORKING COPY —
// MEM/PMS filtered to membership in the static baseline's bank
// definitions (not the raw static list), and 60m/EMG slots grouped under
// SYNTHESISED read-only banks (controller adjudication: loaded data must
// never be invisible in the UI, and the static baseline carries no
// 60M/EMG bank to group them under).
func TestGetUISpec_SlotClassification_OfflineWorkingCopy(t *testing.T) {
	a, _ := newTestApp(t)
	a.mu.Lock()
	a.working = &codeplug.Codeplug{
		Schema: codeplug.CurrentSchema,
		Radio:  codeplug.RadioInfo{Model: "FT-710", CATID: "0800"},
		Channels: []codeplug.Channel{
			{Slot: "001"},
			{Slot: "050"},
			{Slot: "P1L"},
			{Slot: "501"}, // no static bank -> synthesised 60M bank
		},
	}
	a.mu.Unlock()

	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec: unexpected error: %v", err)
	}
	if got.Live {
		t.Error("Live = true while disconnected, want false")
	}
	if len(got.Banks) != 3 {
		t.Fatalf("len(Banks) = %d, want 3 (MEM, PMS + synthesised 60M); got IDs %v", len(got.Banks), bankIDs(got.Banks))
	}

	mem := findBank(t, got.Banks, "MEM")
	memSlots := slotSet(mem.Slots)
	if len(memSlots) != 2 {
		t.Fatalf("MEM.Slots = %v, want exactly {001, 050}", memSlots)
	}
	if memSlots["001"] != "M-01" || memSlots["050"] != "M-50" {
		t.Errorf("MEM.Slots = %v, want {001:M-01 050:M-50}", memSlots)
	}

	pms := findBank(t, got.Banks, "PMS")
	pmsSlots := slotSet(pms.Slots)
	if len(pmsSlots) != 1 || pmsSlots["P1L"] != "P1L" {
		t.Errorf("PMS.Slots = %v, want exactly {P1L:P1L}", pmsSlots)
	}

	sixty := findBank(t, got.Banks, "60M")
	if !sixty.ReadOnly {
		t.Error("synthesised 60M.ReadOnly = false, want true")
	}
	sixtySlots := slotSet(sixty.Slots)
	if len(sixtySlots) != 1 || sixtySlots["501"] != "5-01" {
		t.Errorf("60M.Slots = %v, want exactly {501:5-01}", sixtySlots)
	}
}

// TestGetUISpec_OfflineWorkingCopy_Synthesises60mEMGBanks pins the
// controller-adjudicated offline synthesis in full: a working copy
// holding 60m AND EMG channels (e.g. loaded from an earlier read of a
// US-region radio) gets synthesised read-only 60M/EMG BankViews with the
// SAME labels a live session's discovered banks carry, correct Display
// forms, and — the invariant — every working-copy slot appearing in
// exactly one BankView (no orphans, no duplicates, nothing invented).
func TestGetUISpec_OfflineWorkingCopy_Synthesises60mEMGBanks(t *testing.T) {
	a, _ := newTestApp(t)
	workingSlots := []string{"001", "050", "P1L", "P9U", "501", "502", "515", "EMG"}
	channels := make([]codeplug.Channel, 0, len(workingSlots))
	for _, s := range workingSlots {
		channels = append(channels, codeplug.Channel{Slot: s})
	}
	a.mu.Lock()
	a.working = &codeplug.Codeplug{
		Schema:   codeplug.CurrentSchema,
		Radio:    codeplug.RadioInfo{Model: "FT-710", CATID: "0800"},
		Channels: channels,
	}
	a.mu.Unlock()

	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec: unexpected error: %v", err)
	}
	if got.Live {
		t.Error("Live = true while disconnected, want false")
	}
	if len(got.Banks) != 4 {
		t.Fatalf("len(Banks) = %d, want 4 (MEM, PMS + synthesised 60M, EMG); got IDs %v", len(got.Banks), bankIDs(got.Banks))
	}

	sixty := findBank(t, got.Banks, "60M")
	if !sixty.ReadOnly {
		t.Error("synthesised 60M.ReadOnly = false, want true")
	}
	if sixty.Label != "60 m channels" {
		t.Errorf("synthesised 60M.Label = %q, want %q (must match the live session's discovered-bank label)", sixty.Label, "60 m channels")
	}
	sixtySlots := slotSet(sixty.Slots)
	if len(sixtySlots) != 3 || sixtySlots["501"] != "5-01" || sixtySlots["502"] != "5-02" || sixtySlots["515"] != "5-15" {
		t.Errorf("60M.Slots = %v, want exactly {501:5-01 502:5-02 515:5-15}", sixtySlots)
	}

	emg := findBank(t, got.Banks, "EMG")
	if !emg.ReadOnly {
		t.Error("synthesised EMG.ReadOnly = false, want true")
	}
	if emg.Label != "Emergency (EMG)" {
		t.Errorf("synthesised EMG.Label = %q, want %q (must match the live session's discovered-bank label)", emg.Label, "Emergency (EMG)")
	}
	emgSlots := slotSet(emg.Slots)
	if len(emgSlots) != 1 || emgSlots["EMG"] != "EMG" {
		t.Errorf("EMG.Slots = %v, want exactly {EMG:EMG}", emgSlots)
	}

	// The SYNTHESISED banks' blank-row default must match what a live
	// session's discovered banks report (TestGetUISpec_ConnectedSimulated
	// asserts the same literal): the synthesis derives it from the
	// synthesised banks' OWN field maps, not from the static baseline —
	// which defines no 60M/EMG bank at all and would therefore have
	// answered Unavailable for both, contradicting the same radio's live
	// answer.
	wantKnownOff := codeplug.BoolField{State: codeplug.Known, Value: false}
	for _, b := range got.Banks {
		if b.TagDisplayDefault != wantKnownOff {
			t.Errorf("%s.TagDisplayDefault = %+v, want %+v", b.ID, b.TagDisplayDefault, wantKnownOff)
		}
	}

	// The invariant: the union of every BankView's slots is EXACTLY the
	// working copy's slot set — each working-copy slot in exactly one
	// bank, no orphans, no bank slot the working copy does not hold.
	seen := make(map[string]int)
	total := 0
	for _, b := range got.Banks {
		for _, s := range b.Slots {
			seen[s.Slot]++
			total++
		}
	}
	if total != len(workingSlots) {
		t.Errorf("total slots across all BankViews = %d, want %d (the working copy's slot count)", total, len(workingSlots))
	}
	for _, s := range workingSlots {
		if seen[s] != 1 {
			t.Errorf("working-copy slot %q appears in %d BankViews, want exactly 1", s, seen[s])
		}
	}
}

// TestGetUISpec_OfflineWorkingCopy_PreservesDuplicateEMG pins the offline
// synthesis's duplicate-input behaviour (M9a Codex-review finding 1): a
// working copy holding the EMG slot more than once — reachable via
// LoadFile, which validates only AFTER loading a semantically invalid file
// — must have every occurrence rendered, never collapsed to one, so loaded
// rows are never silently dropped from the grid. This matches the pre-M9a
// synthesis, and the single physical EMG slot a live session probes can
// never produce such a duplicate.
func TestGetUISpec_OfflineWorkingCopy_PreservesDuplicateEMG(t *testing.T) {
	a, _ := newTestApp(t)
	a.mu.Lock()
	a.working = &codeplug.Codeplug{
		Schema:   codeplug.CurrentSchema,
		Radio:    codeplug.RadioInfo{Model: "FT-710", CATID: "0800"},
		Channels: []codeplug.Channel{{Slot: "EMG"}, {Slot: "EMG"}},
	}
	a.mu.Unlock()

	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec: unexpected error: %v", err)
	}
	if got.Live {
		t.Error("Live = true while disconnected, want false")
	}
	emg := findBank(t, got.Banks, "EMG")
	if len(emg.Slots) != 2 {
		t.Fatalf("EMG.Slots = %v, want 2 entries (both occurrences preserved, not collapsed)", emg.Slots)
	}
	for i, sv := range emg.Slots {
		if sv.Slot != "EMG" || sv.Display != "EMG" {
			t.Errorf("EMG.Slots[%d] = %+v, want {Slot:EMG Display:EMG}", i, sv)
		}
	}
}

// TestGetUISpec_SlotClassification_LivePrefersCapsOverWorkingCopy pins
// that, when connected, the bank's caps slot list is authoritative even
// if the working copy (e.g. mid-edit, or stale) does not exactly match
// it — unlike the offline case, live does not filter by/restrict to the
// working copy's own channels.
func TestGetUISpec_SlotClassification_LivePrefersCapsOverWorkingCopy(t *testing.T) {
	a, _ := newTestApp(t)
	sess := openTestSimSession(t, fakeradio.WithFactoryImage(fakeradio.ImageUS))
	connectDirect(t, a, sess, nil)

	// A working copy missing almost everything MEM/PMS caps would list.
	a.mu.Lock()
	a.working = &codeplug.Codeplug{
		Schema:   codeplug.CurrentSchema,
		Radio:    codeplug.RadioInfo{Model: "FT-710", CATID: "0800"},
		Channels: []codeplug.Channel{{Slot: "001"}},
	}
	a.mu.Unlock()

	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec: unexpected error: %v", err)
	}
	mem := findBank(t, got.Banks, "MEM")
	if len(mem.Slots) != 99 {
		t.Errorf("len(MEM.Slots) = %d, want 99 (caps authoritative while live, ignoring the sparse working copy)", len(mem.Slots))
	}
}

// TestGetUISpec_NoWorkingCopy_NoConnection pins the third branch
// explicitly (offline, nothing loaded at all): banks come back with
// their static slot lists as-is (already covered in the disconnected
// baseline test above, restated narrowly here as its own case per the
// task's three-branch requirement).
func TestGetUISpec_NoWorkingCopy_NoConnection(t *testing.T) {
	a, _ := newTestApp(t)
	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec: unexpected error: %v", err)
	}
	mem := findBank(t, got.Banks, "MEM")
	if len(mem.Slots) != 99 {
		t.Errorf("len(MEM.Slots) = %d, want 99", len(mem.Slots))
	}
}

// TestGetUISpec_ToneFormatting is a table-driven check of the
// Decihertz->Display mapping against known CTCSS chart values.
func TestGetUISpec_ToneFormatting(t *testing.T) {
	a, _ := newTestApp(t)
	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec: unexpected error: %v", err)
	}
	byDeci := make(map[int]string, len(got.Tones))
	for _, tv := range got.Tones {
		byDeci[tv.Decihertz] = tv.Display
	}

	tests := []struct {
		deci int
		want string
	}{
		{670, "67.0 Hz"},
		{885, "88.5 Hz"},
		{1000, "100.0 Hz"},
		{2541, "254.1 Hz"},
	}
	for _, tc := range tests {
		got, ok := byDeci[tc.deci]
		if !ok {
			t.Errorf("no tone with Decihertz=%d in Tones", tc.deci)
			continue
		}
		if got != tc.want {
			t.Errorf("tone %d Display = %q, want %q", tc.deci, got, tc.want)
		}
	}
}

// TestGetUISpec_VocabMatchesValidate cross-checks that every literal
// GetUISpec exposes for Mode/Shift/CTCSS-state is one codeplug.Validate
// itself accepts for that field — i.e. the grid's option lists can never
// offer a value Validate would then reject.
func TestGetUISpec_VocabMatchesValidate(t *testing.T) {
	a, _ := newTestApp(t)
	uiSpec, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec: unexpected error: %v", err)
	}
	caps, err := wiring.StaticCapabilities(wiring.DefaultModel)
	if err != nil {
		t.Fatalf("wiring.StaticCapabilities(%q): unexpected error: %v", wiring.DefaultModel, err)
	}

	baseData := func() codeplug.ChannelData {
		return codeplug.ChannelData{
			FreqHz:     7_000_000,
			Mode:       "USB",
			CTCSS:      "OFF",
			CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
			Shift:      "SIMPLEX",
			TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false},
			ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
		}
	}

	assertNoFieldIssue := func(t *testing.T, cp *codeplug.Codeplug, field spec.Field, value string) {
		t.Helper()
		issues := codeplug.Validate(cp, caps)
		for _, is := range issues {
			if is.Slot == "001" && is.Field == field {
				t.Errorf("Validate flagged %s=%q on slot 001: %s", field, value, is.Msg)
			}
		}
	}

	for _, mode := range uiSpec.Modes {
		d := baseData()
		d.Mode = mode
		cp := &codeplug.Codeplug{Schema: codeplug.CurrentSchema, Channels: []codeplug.Channel{{Slot: "001", Data: &d}}}
		assertNoFieldIssue(t, cp, "mode", mode)
	}
	for _, shift := range uiSpec.ShiftOptions {
		d := baseData()
		d.Shift = shift
		cp := &codeplug.Codeplug{Schema: codeplug.CurrentSchema, Channels: []codeplug.Channel{{Slot: "001", Data: &d}}}
		assertNoFieldIssue(t, cp, "shift", shift)
	}
	for _, ctcss := range uiSpec.CTCSSStateOptions {
		d := baseData()
		d.CTCSS = ctcss
		cp := &codeplug.Codeplug{Schema: codeplug.CurrentSchema, Channels: []codeplug.Channel{{Slot: "001", Data: &d}}}
		assertNoFieldIssue(t, cp, "ctcss_state", ctcss)
	}
}

// TestGetUISpec_ServesProse pins the served sentences server-side (task
// 41, M9a-5): every radiotext-sourced UISpecView field is byte-equal to
// internal/radiotext.For(wiring.DefaultModel)'s own value — not merely
// non-empty — so a future edit to either side (this call site or
// radiotext's own FT-710 entry) that let them drift would fail here
// first. Checked both offline and connected: since M9c-5 (E4) the prose
// is keyed off currentModel's resolved model rather than
// wiring.DefaultModel, and both branches resolve to the FT-710 here — the
// simulated session's own model offline-or-not — so both must still serve
// exactly these strings.
func TestGetUISpec_ServesProse(t *testing.T) {
	want, ok := radiotext.For(wiring.DefaultModel)
	if !ok {
		t.Fatalf("radiotext.For(%q): ok = false, want true", wiring.DefaultModel)
	}

	assertProse := func(t *testing.T, got UISpecView) {
		t.Helper()
		if got.ToneScanSkipNote != want.ToneScanSkipNote {
			t.Errorf("ToneScanSkipNote = %q, want %q", got.ToneScanSkipNote, want.ToneScanSkipNote)
		}
		if got.ToneScanSkipVerification != want.ToneScanSkipVerification {
			t.Errorf("ToneScanSkipVerification = %q, want %q", got.ToneScanSkipVerification, want.ToneScanSkipVerification)
		}
		if got.EraseDialogNote != want.EraseDialogNote {
			t.Errorf("EraseDialogNote = %q, want %q", got.EraseDialogNote, want.EraseDialogNote)
		}
		if got.PreservationTooltips.Tone != want.PreservationTooltips.Tone {
			t.Errorf("PreservationTooltips.Tone = %q, want %q", got.PreservationTooltips.Tone, want.PreservationTooltips.Tone)
		}
		if got.PreservationTooltips.ScanSkip != want.PreservationTooltips.ScanSkip {
			t.Errorf("PreservationTooltips.ScanSkip = %q, want %q", got.PreservationTooltips.ScanSkip, want.PreservationTooltips.ScanSkip)
		}
		if got.FirmwarePlaceholder != want.FirmwarePlaceholder {
			t.Errorf("FirmwarePlaceholder = %q, want %q", got.FirmwarePlaceholder, want.FirmwarePlaceholder)
		}
	}

	t.Run("offline", func(t *testing.T) {
		a, _ := newTestApp(t)
		got, err := a.GetUISpec()
		if err != nil {
			t.Fatalf("GetUISpec: unexpected error: %v", err)
		}
		assertProse(t, got)
	})

	t.Run("connected", func(t *testing.T) {
		a, _ := newTestApp(t)
		sess := openTestSimSession(t)
		connectDirect(t, a, sess, nil)
		got, err := a.GetUISpec()
		if err != nil {
			t.Fatalf("GetUISpec: unexpected error: %v", err)
		}
		assertProse(t, got)
	})
}

// TestGetUISpec_ProseFollowsResolvedModel is the prose cluster's
// threading pin (M9c-5 E4): the served sentences follow the model
// currentModel resolves, and radiotext.For's ok is honoured — a model
// with no radiotext entry gets SILENCE (every prose field empty), never
// the FT-710's own wording attributed to a different radio.
//
// testModel is admitted through the capsForModel seam only, so
// internal/radiotext genuinely has no entry for it; the empty fields
// below are therefore the honest served value for a model whose prose has
// not been written yet, and they are observably different from the
// FT-710's (pinned non-empty first, so this cannot pass vacuously).
func TestGetUISpec_ProseFollowsResolvedModel(t *testing.T) {
	ft710Text, ok := radiotext.For(wiring.DefaultModel)
	if !ok || ft710Text.ToneScanSkipNote == "" || ft710Text.EraseDialogNote == "" {
		t.Fatalf("test setup: radiotext.For(%q) ok=%v with empty prose — the contrast below would be vacuous", wiring.DefaultModel, ok)
	}
	recogniseTestModel(t)

	a, _ := newTestApp(t)
	a.mu.Lock()
	a.working = &codeplug.Codeplug{
		Schema:   codeplug.CurrentSchema,
		Radio:    codeplug.RadioInfo{Model: testModel},
		Channels: []codeplug.Channel{{Slot: "001"}},
	}
	a.mu.Unlock()

	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec: unexpected error: %v", err)
	}
	for _, f := range []struct {
		name string
		got  string
	}{
		{"ToneScanSkipNote", got.ToneScanSkipNote},
		{"ToneScanSkipVerification", got.ToneScanSkipVerification},
		{"EraseDialogNote", got.EraseDialogNote},
		{"PreservationTooltips.Tone", got.PreservationTooltips.Tone},
		{"PreservationTooltips.ScanSkip", got.PreservationTooltips.ScanSkip},
		{"FirmwarePlaceholder", got.FirmwarePlaceholder},
	} {
		if f.got != "" {
			t.Errorf("%s = %q for a model radiotext has no entry for, want \"\" (silence, never another radio's wording)", f.name, f.got)
		}
	}
}

// TestGetUISpec_UnrecognisedWorkingModelStillSynthesisesBanks pins the
// reason the bank-synthesis site consumes the RESOLVER rather than the
// working copy's raw Radio.Model (M9c-5 E4's recorded design note): a
// legacy or hand-edited file naming a model no driver is registered for
// must still show its 60m/EMG channels. Handed the raw name,
// wiring.SynthesiseDiscoveredBanks would report ok == false and those
// channels would vanish from the grid — loaded but invisible. The
// resolver falls back to wiring.DefaultModel, so they stay.
func TestGetUISpec_UnrecognisedWorkingModelStillSynthesisesBanks(t *testing.T) {
	a, _ := newTestApp(t)
	a.mu.Lock()
	a.working = &codeplug.Codeplug{
		Schema: codeplug.CurrentSchema,
		Radio:  codeplug.RadioInfo{Model: "NoSuchRadioModel"},
		Channels: []codeplug.Channel{
			{Slot: "001"}, {Slot: "P1L"}, {Slot: "501"}, {Slot: "EMG"},
		},
	}
	a.mu.Unlock()

	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec: unexpected error: %v", err)
	}
	sixty := findBank(t, got.Banks, "60M")
	if s := slotSet(sixty.Slots); len(s) != 1 || s["501"] != "5-01" {
		t.Errorf("synthesised 60M.Slots = %v, want exactly {501:5-01}", s)
	}
	emg := findBank(t, got.Banks, "EMG")
	if s := slotSet(emg.Slots); len(s) != 1 || s["EMG"] != "EMG" {
		t.Errorf("synthesised EMG.Slots = %v, want exactly {EMG:EMG}", s)
	}
}
