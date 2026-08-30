// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/internal/fakedx10"
	"github.com/gm5dna/open-rig-programmer/internal/fakedx101"
	"github.com/gm5dna/open-rig-programmer/internal/fakeic705"
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

// ic705SlotViews builds the SlotViews a list of this radio's canonical
// group-addressed slots must produce. Display equals Slot for every one of
// them: codeplug.DisplaySlot's rewrite applies only to three-character
// slot strings, so a "Gnn-nnn" address passes through unchanged (the
// identity fallback spec.SparseSlot's doc comment relies on).
func ic705SlotViews(slots ...string) []SlotView {
	out := make([]SlotView, 0, len(slots))
	for _, s := range slots {
		out = append(out, SlotView{Slot: s, Display: s})
	}
	return out
}

// ic9700SlotViews builds the SlotViews a list of this radio's canonical
// <band>-<channel> slots must produce. Display equals Slot for every one
// of them, on the same footing as ic705SlotViews above: every IC-9700
// slot string (144-001, 144-P1A, 144-C1, and their longer siblings) is
// longer than three characters, so codeplug.DisplaySlot's rewrite — which
// applies only to three-character slot strings — never touches any of
// them.
func ic9700SlotViews(slots ...string) []SlotView {
	out := make([]SlotView, 0, len(slots))
	for _, s := range slots {
		out = append(out, SlotView{Slot: s, Display: s})
	}
	return out
}

// firstSlotViewDiff returns the index of the first position at which two
// SlotView lists differ, with each side's value there — the zero SlotView
// standing in where one list has run out — or -1 when the two are equal.
//
// A DENSE IC-9700 bank is 297 entries, and printing two of those whole
// tells a reader only that something moved. The index is what names the
// slot, and the pair at it is what says how it changed (R5 deferred
// minor).
func firstSlotViewDiff(got, want []SlotView) (int, SlotView, SlotView) {
	at := func(s []SlotView, i int) SlotView {
		if i < len(s) {
			return s[i]
		}
		return SlotView{}
	}
	n := len(got)
	if len(want) > n {
		n = len(want)
	}
	for i := 0; i < n; i++ {
		if at(got, i) != at(want, i) {
			return i, at(got, i), at(want, i)
		}
	}
	return -1, SlotView{}, SlotView{}
}

// ic905SlotViews builds the SlotViews a list of this radio's canonical
// slots must produce — MEM's group-addressed "Gnn-nnn" form and CALL's
// own "Cnn" form alike. Display equals Slot for every one of them, on
// the same footing as ic705SlotViews above: codeplug.DisplaySlot's
// rewrite applies only to three-character slot strings whose first byte
// is '0' or '5' (the Yaesu 60m/EMG spellings), so neither this radio's
// seven-character MEM addresses nor its three-character "C01".."C12"
// CALL slots (which start with 'C', not '0' or '5') are ever touched by
// it.
func ic905SlotViews(slots ...string) []SlotView {
	out := make([]SlotView, 0, len(slots))
	for _, s := range slots {
		out = append(out, SlotView{Slot: s, Display: s})
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

	// The shape a CONSENTED session carries: spec.ConsentUnverifiedWrites
	// rewrites the WRITE label only, so Read stays Unverified.
	consented := spec.FieldSupport{Read: spec.Unverified, Write: spec.ConsentedUnverified}

	allWritable := map[spec.Field]spec.FieldSupport{}
	allUnverified := map[spec.Field]spec.FieldSupport{}
	allUnsupported := map[spec.Field]spec.FieldSupport{}
	mixedOneWritable := map[spec.Field]spec.FieldSupport{}
	inertClarifier := map[spec.Field]spec.FieldSupport{}
	allConsented := map[spec.Field]spec.FieldSupport{}
	mixedOneConsented := map[spec.Field]spec.FieldSupport{}
	for _, f := range bankCoreCandidates {
		allWritable[f] = rw
		allUnverified[f] = unverified
		allUnsupported[f] = unsupported
		mixedOneWritable[f] = unsupported
		inertClarifier[f] = rw
		allConsented[f] = consented
		mixedOneConsented[f] = unsupported
	}
	mixedOneWritable[spec.FieldFrequency] = rw
	mixedOneConsented[spec.FieldClarifier] = consented
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
		// The fifth state's two rows. A consented session is the shape a
		// RealHardware FTdx10/FTdx101 gets once the user has granted
		// unverified writes, and it must read exactly as the Unverified
		// row above does — not read-only. The MIXED row is the one that
		// bites: it fails the moment bankReadOnly is re-tested on
		// "Write == spec.Supported" (the tempting narrowing), because
		// consent is the only thing keeping that bank editable.
		{"all ConsentedUnverified -> not read-only (a consented session is editable, as its Unverified original was)", allConsented, false},
		{"one ConsentedUnverified field among Unsupported rest -> not read-only (a consented column stays editable)", mixedOneConsented, false},
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

// ic7610CoreThree is the core set every IC-7610 bank derives, on every
// profile — MEM and SCAN alike, since core/driver/ic7610/caps.go's
// bankFields applies the same support to both.
//
// It is a THIRD, DISTINCT shape from ft710CoreSeven and ftdx10CoreSix, not
// a subset of either: bankCoreCandidates (app/uispec.go) is the Yaesu
// grid's own nine-field candidate list, and the IC-7610's memory record
// maps only three of them — FieldFrequency, FieldMode and FieldTag.
// FieldClarifier, FieldShift, FieldCTCSSState, FieldCTCSSTone,
// FieldScanSkip and FieldTagDisplay are ALL the zero FieldSupport on this
// radio (core/driver/ic7610/caps.go's deliberatelyZero table and its E6
// ruling), so none of them survives bankCoreFields' zero-value test. This
// radio's OWN tier-added fields — tone_mode, tone_tx, tone_rx and filter,
// all of which ARE mapped — are outside bankCoreCandidates entirely and
// never contribute to this set; they surface instead through
// bankTierFields (BankView.Fields), which
// TestGetUISpec_RegisteredIC7610_EveryBankFieldsAndTagDisplay checks
// directly.
var ic7610CoreThree = []spec.Field{
	spec.FieldFrequency, spec.FieldMode, spec.FieldTag,
}

// ic7610TierFields is what bankTierFields (app/uispec.go) derives for
// every IC-7610 bank, on every profile: the four tier-added fields this
// radio's 1A 00 record maps (tone_mode, tone_tx, tone_rx, filter — in
// tierFields' own declaration order), and none of the other six
// (tx_frequency, duplex, offset, dtcs_code, dtcs_polarity, data_mode),
// which are all the zero FieldSupport on this radio too (the same
// deliberatelyZero table ic7610CoreThree's doc comment cites).
//
// THE IC-7610 WAS THE FIRST REGISTERED MODEL WHOSE BankView.Fields WAS EVER
// NON-EMPTY, at this model's own registration (Wave 4 task R1).
// bankTierFields' own doc comment (app/uispec.go) has since been rewritten
// as an open statement covering every registered Icom model by name,
// rather than the stale "every bank of every model registered today
// therefore returns nil" it originally carried — corrected as part of the
// IC-7300 pair's own registration (Wave 4 task R3 fix round 1), which is
// what first made that sentence false for a SECOND family.
var ic7610TierFields = []string{"tone_mode", "tone_tx", "tone_rx", "filter"}

// ic7300CoreThree is the core set every IC-7300 bank derives, on every
// profile — MEM and SCAN alike, since core/driver/ic7300/caps.go's
// bankFields applies the same support to both.
//
// SAME MEMBERS AS ic7610CoreThree, and that is a COINCIDENCE OF INDEPENDENT
// EVIDENCE, not a shared derivation: core/driver/ic7300 does not import
// core/driver/ic7610, and its own bankFields was written from the IC-7300
// FULL MANUAL alone (core/driver/ic7300/doc.go's Provenance section), which
// happens to grade bankCoreCandidates' nine fields the same way the
// IC-7610's CI-V Reference Guide does — FieldFrequency, FieldMode and
// FieldTag rw, and FieldClarifier, FieldShift, FieldCTCSSState,
// FieldCTCSSTone, FieldScanSkip and FieldTagDisplay all the zero
// FieldSupport (core/driver/ic7300/caps.go's bankFields). A SEPARATE
// variable, not a reuse of ic7610CoreThree, so this test's own map
// (TestBankCoreFields_EveryRegisteredModel_Membership) states each model's
// expectation independently rather than implying one radio's evidence
// proves another's.
var ic7300CoreThree = []spec.Field{
	spec.FieldFrequency, spec.FieldMode, spec.FieldTag,
}

// ic7300mk2CoreThree is the IC-7300MK2's own core set, on the same
// independent-evidence footing as ic7300CoreThree: core/driver/ic7300mk2's
// bankFields was written from the IC-7300MK2 CI-V Reference Guide alone
// (that document is mutually silent with the IC-7300's, per
// core/driver/ic7300mk2/doc.go's package comment) and happens to grade the
// same three candidates non-zero.
var ic7300mk2CoreThree = []spec.Field{
	spec.FieldFrequency, spec.FieldMode, spec.FieldTag,
}

// ic7300TierFields is what bankTierFields (app/uispec.go) derives for
// every IC-7300 bank, on every profile: SIX of the tier's ten fields —
// tx_frequency, tone_mode, tone_tx, tone_rx, filter and data_mode, in
// tierFields' own declaration order — because this radio's 1A 00 record
// maps its OWN transmit-frequency and filter/data-mode bytes, unlike the
// IC-7610's record, which does not (core/driver/ic7300/caps.go's
// bankFields: FieldTxFrequency, FieldFilter and FieldDataMode are all rw
// here, all the zero FieldSupport there). The other four
// (duplex, offset, dtcs_code, dtcs_polarity) are the zero FieldSupport on
// this radio too, on the same footing as the IC-7610's.
//
// A GENUINELY DIFFERENT SHAPE from ic7610TierFields, not a lengthening of
// it: this is the evidence this task's brief asks for — each model's own
// tier-field set, derived from its own record, stated here rather than
// assumed to match the tier's first Icom registration.
var ic7300TierFields = []string{"tx_frequency", "tone_mode", "tone_tx", "tone_rx", "filter", "data_mode"}

// ic7300mk2TierFields is the IC-7300MK2's own tier-field set. Its 1A 00
// record maps the SAME nine wire-carried fields as the IC-7300's, in the
// same shape (core/driver/ic7300mk2/caps.go's bankFields is structurally
// identical to the IC-7300's, independently written from a different
// document), so this list has the same MEMBERS as ic7300TierFields — a
// separate variable for the same independent-evidence reason
// ic7300mk2CoreThree is one rather than a reuse of ic7300CoreThree.
var ic7300mk2TierFields = []string{"tx_frequency", "tone_mode", "tone_tx", "tone_rx", "filter", "data_mode"}

// ic705CoreThree is the core set every IC-705 bank derives, on every
// profile — MEM and CALL alike, since core/driver/ic705/caps.go's
// bankFields applies the same support to both.
//
// SAME MEMBERS AS ic7610CoreThree AND ic7300CoreThree, and that is a
// COINCIDENCE OF INDEPENDENT EVIDENCE, not a shared derivation, on the
// same footing as ic7300CoreThree's own doc comment: core/driver/ic705
// does not import either sibling package, and its own bankFields was
// written from the IC-705 CI-V Reference Guide alone.
var ic705CoreThree = []spec.Field{
	spec.FieldFrequency, spec.FieldMode, spec.FieldTag,
}

// ic705TierFields is what bankTierFields (app/uispec.go) derives for
// every IC-705 bank, on every profile: ALL TEN of the tier's fields, in
// tierFields' own declaration order — because this radio's 111-byte
// record maps all thirteen of matrix §2's rw-graded rows
// (core/driver/ic705/caps.go's bankFields), including duplex, offset,
// dtcs_code and dtcs_polarity, none of which any other registered Icom
// model's record carries (core/driver/ic7610/caps.go's and
// core/driver/ic7300/caps.go's bankFields grade all four the zero
// FieldSupport).
//
// A GENUINELY DIFFERENT SHAPE from every other registered Icom model's,
// not a lengthening of the IC-7300 pair's six: this is the tier's own
// widest reach to date, derived from this radio's own record rather than
// assumed to match any prior registration.
var ic705TierFields = []string{"tx_frequency", "duplex", "offset", "tone_mode", "tone_tx", "tone_rx", "dtcs_code", "dtcs_polarity", "filter", "data_mode"}

// ic9700CoreThree is the core set every IC-9700 bank derives, on every
// profile — MEM, SCAN and CALL alike, since core/driver/ic9700/caps.go's
// bankFields (identical on all three banks) applies the same support to
// each.
//
// SAME MEMBERS AS ic7610CoreThree, ic7300CoreThree AND ic705CoreThree, and
// that is a COINCIDENCE OF INDEPENDENT EVIDENCE, not a shared derivation,
// on the same footing as ic705CoreThree's own doc comment:
// core/driver/ic9700 does not import any sibling package, and its own
// bankFields was written from the IC-9700 CI-V Reference Guide alone.
var ic9700CoreThree = []spec.Field{
	spec.FieldFrequency, spec.FieldMode, spec.FieldTag,
}

// ic9700TierFields is what bankTierFields (app/uispec.go) derives for
// every IC-9700 bank, on every profile: ALL TEN of the tier's fields, in
// tierFields' own declaration order — because this radio's 111-byte
// record maps all thirteen of matrix §2's rw-graded rows
// (core/driver/ic9700/caps.go's bankFields), including duplex, offset,
// dtcs_code and dtcs_polarity, none of which the IC-7610 or the IC-7300
// pair's record carries.
//
// SAME SHAPE AS ic705TierFields, and — exactly as ic9700CoreThree's own
// doc comment says of the core set — that is independent evidence, not a
// shared derivation: the IC-705 CI-V Reference Guide and the IC-9700 CI-V
// Reference Guide are two unrelated documents, and neither this driver
// nor core/driver/ic705 imports the other.
var ic9700TierFields = []string{"tx_frequency", "duplex", "offset", "tone_mode", "tone_tx", "tone_rx", "dtcs_code", "dtcs_polarity", "filter", "data_mode"}

// ic905CoreThree is the core set every IC-905 bank derives, on every
// profile — MEM and CALL alike, since core/driver/ic905/caps.go's
// bankFields applies the same support to both.
//
// SAME MEMBERS AS ic7610CoreThree, ic7300CoreThree, ic705CoreThree AND
// ic9700CoreThree, and that is a COINCIDENCE OF INDEPENDENT EVIDENCE, not
// a shared derivation, on the same footing as every one of those doc
// comments: core/driver/ic905 does not import any sibling package, and
// its own bankFields was written from the IC-905 CI-V Reference Guide
// alone.
var ic905CoreThree = []spec.Field{
	spec.FieldFrequency, spec.FieldMode, spec.FieldTag,
}

// ic905TierFields is what bankTierFields (app/uispec.go) derives for
// every IC-905 bank, on every profile: NINE of the tier's ten fields, in
// tierFields' own declaration order — every one the IC-705's and the
// IC-9700's own TEN-field lists carry EXCEPT tx_frequency.
//
// A GENUINELY DIFFERENT SHAPE from ic705TierFields and ic9700TierFields,
// not a repetition of either: core/driver/ic905/caps.go's bankFields
// zeroes FieldTxFrequency (MANUAL-EVIDENCED ABSENCE — this radio's
// 64/65-byte record carries exactly one frequency field, no duplicated
// TX block), so tx_frequency drops out of the derived set entirely and
// bankTierFields' loop over tierFields' declaration order starts at
// duplex.
var ic905TierFields = []string{"duplex", "offset", "tone_mode", "tone_tx", "tone_rx", "dtcs_code", "dtcs_polarity", "filter", "data_mode"}

// ic7851CoreThree is the core set every IC-7851 and IC-7850 bank derives,
// on every profile — MEM and SCAN alike, since core/driver/ic7851/caps.go's
// bankFields applies the same support to both.
//
// ONE VARIABLE FOR TWO REGISTERED ROWS, unlike every other pairing in
// this file, and the reason is that the two rows are served by ONE
// driver: core/driver/ic7851's New7851 and New7850 build the same
// implementation over the same civ.Profile and differ in Model() alone
// (additions spec D1.2), so a per-row difference in the derived core set
// would mean a model dimension had appeared in a capability table that
// has none. The membership map below still gives each ROW its own entry,
// and the loop there still walks each row's own registered capability
// data, exactly as the FTdx101D/FTdx101MP rows do over ftdx10CoreSix.
//
// SAME MEMBERS AS ic7610CoreThree and every other Icom entry's, and that
// is a COINCIDENCE OF INDEPENDENT EVIDENCE rather than a shared
// derivation: core/driver/ic7851 imports no sibling driver package, and
// its own bankFields was written from the IC-7850/IC-7851 Instruction
// Manual alone. FieldClarifier, FieldShift, FieldCTCSSState,
// FieldCTCSSTone, FieldScanSkip and FieldTagDisplay are all the zero
// FieldSupport on this radio (core/driver/ic7851/caps.go's bankFields and
// its deliberatelyZero table), so none of them survives bankCoreFields'
// zero-value test.
var ic7851CoreThree = []spec.Field{
	spec.FieldFrequency, spec.FieldMode, spec.FieldTag,
}

// ic7851TierFields is what bankTierFields (app/uispec.go) derives for
// every IC-7851 and IC-7850 bank, on every profile: the four tier-added
// fields this radio's 1A 00 record maps (tone_mode, tone_tx, tone_rx,
// filter — in tierFields' own declaration order), and none of the others.
//
// THE SAME SHAPE AS ic7610TierFields, and additions spec D1.1 is why it
// was expected: both documents draw the same 27-byte data area, so the
// same four tier fields are mapped. It is a SEPARATE variable and not a
// reuse of ic7610TierFields, for the reason every other model's is — the
// value is this radio's own derivation, and a future divergence between
// the two records must show up as a diff here rather than being absorbed
// by a shared name.
//
// Notably NOT present: data_mode. Byte ⑪'s HIGH nibble IS a data-mode
// field on this radio, and it is deliberately UNMAPPED under ruling E6
// (four printed values — OFF/DATA 1/DATA 2/DATA 3 — against a neutral
// BoolField), so it carries the zero FieldSupport and is unreachable.
// Absence here is "this build does not expose it", which is exactly what
// the grid must show.
var ic7851TierFields = []string{"tone_mode", "tone_tx", "tone_rx", "filter"}

// ic7760CoreThree is the core set every IC-7760 bank derives, on every
// profile — MEM and SCAN alike, since core/driver/ic7760/caps.go's
// bankFields applies the same support to both (its own comment says why:
// both banks read and write the SAME 1A 00 record at different values of
// the same selector).
//
// SAME MEMBERS AS ic7610CoreThree, ic7851CoreThree and every other Icom
// entry's, and that is a COINCIDENCE OF INDEPENDENT EVIDENCE rather than
// a shared derivation: core/driver/ic7760 imports no sibling driver
// package, and its bankFields was written from the IC-7760 CI-V Reference
// Guide revision 2 alone. FieldClarifier, FieldShift, FieldCTCSSState,
// FieldCTCSSTone, FieldScanSkip and FieldTagDisplay are all the zero
// FieldSupport on this radio (that file's bankFields and its
// deliberatelyZero table), so none of them survives bankCoreFields'
// zero-value test.
var ic7760CoreThree = []spec.Field{
	spec.FieldFrequency, spec.FieldMode, spec.FieldTag,
}

// ic7760TierFields is what bankTierFields (app/uispec.go) derives for
// every IC-7760 bank, on every profile: the four tier-added fields this
// radio's 1A 00 record maps (tone_mode, tone_tx, tone_rx, filter — in
// tierFields' own declaration order), and none of the others.
//
// A SEPARATE VARIABLE, not a reuse of ic7610TierFields or
// ic7851TierFields, for the reason every other model's is: the value is
// this radio's own derivation, and a future divergence between the three
// records must show up as a diff here rather than being absorbed by a
// shared name. That the three agree is what additions spec D1.1
// predicted; that they are measured separately is what would let a
// divergence be seen.
//
// Notably NOT present: data_mode. Byte ⑪'s HIGH nibble IS a data-mode
// field on this radio, and it is deliberately UNMAPPED under ruling E6
// (four printed values — OFF/DATA 1/DATA 2/DATA 3 — against a neutral
// BoolField), so it carries the zero FieldSupport and is unreachable.
// Absence here is "this build does not expose it", which is exactly what
// the grid must show.
var ic7760TierFields = []string{"tone_mode", "tone_tx", "tone_rx", "filter"}

// ic7100CoreThree is the core set the IC-7100's ONE bank derives, on every
// profile — there is only the dense 495-slot MEM space to derive it from,
// this build addressing no other bank on this radio (the scan-edge and
// call channels are refused until register entry
// ic7100-special-bank-byte is lifted).
//
// SAME MEMBERS AS ic7610CoreThree, ic705CoreThree and every other Icom
// entry's, and that is a COINCIDENCE OF INDEPENDENT EVIDENCE rather than
// a shared derivation: core/driver/ic7100 imports no sibling driver
// package, and its fieldGrid was written from section 20 of the IC-7100's
// own full manual alone. FieldClarifier, FieldShift, FieldCTCSSState,
// FieldCTCSSTone, FieldScanSkip and FieldTagDisplay are all the zero
// FieldSupport on this radio (that file's deliberately-zero audit), so
// none of them survives bankCoreFields' zero-value test.
var ic7100CoreThree = []spec.Field{
	spec.FieldFrequency, spec.FieldMode, spec.FieldTag,
}

// ic7100TierFields is what bankTierFields (app/uispec.go) derives for the
// IC-7100's one bank, on every profile: ALL TEN of the tier's fields, in
// tierFields' own declaration order — because this radio's 111-byte
// record maps all thirteen of matrix §2's rw-graded rows
// (core/driver/ic7100/caps.go's fieldGrid), including duplex, offset,
// dtcs_code, dtcs_polarity and data_mode.
//
// SAME SHAPE AS ic705TierFields AND ic9700TierFields, and — exactly as
// ic9700TierFields' own doc comment says of that pair — that is
// independent evidence, not a shared derivation: three unrelated
// documents, and none of the three drivers imports either of the others.
// A SEPARATE VARIABLE for the reason every other model's is: a future
// divergence between the three records must show up as a diff here rather
// than being absorbed by a shared name.
//
// Notably PRESENT, where the 25-byte Icom records leave it out: data_mode.
// Byte ⑫ is a two-valued OFF/ON data-mode field on this radio
// (core/civ/ic7100/profile.go's dataModeNames), so it is mapped and
// writable rather than refused under ruling E6 — which is why this row's
// list is ten long and the IC-7610's, IC-7851's and IC-7760's are four.
var ic7100TierFields = []string{"tx_frequency", "duplex", "offset", "tone_mode", "tone_tx", "tone_rx", "dtcs_code", "dtcs_polarity", "filter", "data_mode"}

// icr8600CoreThree is the core set the IC-R8600's ONE bank derives, on
// every profile: frequency, mode and tag, in bankCoreCandidates order.
//
// SAME MEMBERS AS EVERY OTHER ICOM ENTRY'S, and a coincidence of
// independent evidence rather than a shared derivation once more:
// core/driver/icr8600 imports no sibling driver package, and its
// bankFields was written from this receiver's own CI-V Reference Guide
// (revision 3a) alone. FieldClarifier, FieldShift, FieldCTCSSState,
// FieldCTCSSTone, FieldScanSkip and FieldTagDisplay are all the zero
// FieldSupport here (that file's deliberately-zero audit), so none of
// them survives bankCoreFields' zero-value test.
//
// AND BEING A RECEIVER COSTS NOTHING IN THIS SET, which is worth stating
// because it is the surprising half: spec.ReceiveOnly removes
// tx_frequency and tone_tx, and NEITHER is a bankCoreCandidates member.
// The core three a receiver's grid renders are the transceivers' three.
var icr8600CoreThree = []spec.Field{
	spec.FieldFrequency, spec.FieldMode, spec.FieldTag,
}

// icr8600TierFields is what bankTierFields (app/uispec.go) derives for
// the IC-R8600's one bank, on every profile: FOURTEEN of the seventeen
// tier-added fields, in tierFields' own declaration order.
//
// THE FIRST LIST HERE THAT REACHES THE SEVEN D8 RECEIVER FIELDS. No
// registered model before this one grades tuning_step_enabled,
// tuning_step, program_tuning_step, attenuator, preamp, antenna or ip_plus
// anything but the zero FieldSupport, so those seven columns have been
// unreachable on every bank of every row since the additions core minted
// them (additions spec D8.2). core/driver/icr8600/caps.go's bankFields
// maps all seven from the record's common head, so they appear here — and
// the ORDER is tierFields', which puts them after the tier's original ten
// because that is codeplug.ChannelData's own declaration order.
//
// AND THE FIRST THAT IS SHORT OF THE ORIGINAL TEN FOR TWO DIFFERENT
// REASONS. tx_frequency and tone_tx are missing by ANATOMY — this radio
// has no transmitter, and additions spec D4.2's invariant makes grading
// either above Unsupported a spec.Validate failure rather than a choice,
// so their absence here is structural and could not be edited away.
// data_mode is missing for the ordinary reason the 25-byte records' own
// is: this record carries no such byte.
//
// A SEPARATE VARIABLE for the reason every other model's is: a future
// divergence must show up as a diff here rather than being absorbed by a
// shared name.
var icr8600TierFields = []string{
	"duplex", "offset", "tone_mode", "tone_rx", "dtcs_code", "dtcs_polarity", "filter",
	"tuning_step_enabled", "tuning_step", "program_tuning_step", "attenuator", "preamp", "antenna", "ip_plus",
}

func TestBankTierFields_ReceiverFieldsFollowBankReachability(t *testing.T) {
	rw := spec.FieldSupport{Read: spec.Supported, Write: spec.Unverified}
	caps := spec.Capabilities{Banks: []spec.Bank{{
		ID: spec.BankMemory,
		Fields: map[spec.Field]spec.FieldSupport{
			spec.FieldTuningStepEnabled: rw,
			spec.FieldTuningStep:        rw,
			spec.FieldProgramTuningStep: rw,
			spec.FieldAttenuator:        rw,
			spec.FieldPreamp:            rw,
			spec.FieldAntenna:           rw,
			spec.FieldIPPlus:            rw,
		},
	}}}
	want := []string{
		"tuning_step_enabled", "tuning_step", "program_tuning_step",
		"attenuator", "preamp", "antenna", "ip_plus",
	}
	if got := bankTierFields(caps, spec.BankMemory); !reflect.DeepEqual(got, want) {
		t.Errorf("bankTierFields = %v, want %v", got, want)
	}
	if got := bankTierFields(caps, spec.BankScan); len(got) != 0 {
		t.Errorf("unregistered bank fields = %v, want none", got)
	}
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
		// The fifth state. A consented session's write label is non-zero,
		// so the field is in the derived core set exactly as its Unverified
		// original was: consent changes whose word the confidence rests on,
		// never whether the radio's frame carries the field.
		{"ConsentedUnverified write (the consented session's shape)", spec.FieldSupport{Read: spec.Unverified, Write: spec.ConsentedUnverified}, true},
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
		// The FTDX101D and FTDX101MP derive the SAME six fields as the
		// FTdx10, and ftdx10CoreSix is reused rather than copied because
		// the shape is the COMBINED MT RECORD's, which all three radios
		// share by construction — not a coincidence between two lists that
		// happen to match today.
		//
		// Capability matrix §2.1 is the statement of record: FieldFrequency,
		// FieldMode, FieldClarifier, FieldCTCSSState, FieldShift and
		// FieldTag carry support on every bank; FieldTagDisplay,
		// FieldCTCSSTone, FieldScanSkip and FieldErase are the zero
		// FieldSupport and drop out of the derived set. TagDisplay's absence
		// is the MANUAL-EVIDENCED one (matrix §3.7: P11 is "0: (Fixed)" at
		// layout 1329, and the 41-position geometry witness leaves nowhere
		// to put a display flag).
		//
		// The matrix's §2 values are identical for D and MP throughout
		// (§2.5), so one list serves both entries — but each model still
		// gets its own row here, and the loop below still walks each
		// model's own registered capability data.
		"FTdx101D":  ftdx10CoreSix,
		"FTdx101MP": ftdx10CoreSix,
		// The IC-7610 (Wave 4 task R1): a genuinely DIFFERENT three-field
		// set, not a coincidence of the derivation — see ic7610CoreThree's
		// own doc comment for which candidates this radio's memory record
		// maps and which it does not.
		"IC-7610": ic7610CoreThree,
		// The IC-7300 and IC-7300MK2 (Wave 4 task R3): the same three
		// candidate fields as the IC-7610's, independently derived — see
		// ic7300CoreThree's own doc comment for why each model gets its
		// own variable rather than a reuse of ic7610CoreThree's.
		"IC-7300":    ic7300CoreThree,
		"IC-7300MK2": ic7300mk2CoreThree,
		// The IC-705 (Wave 4 task R4): the same three candidate fields
		// again, independently derived — see ic705CoreThree's own doc
		// comment for why this model gets its own variable too.
		"IC-705": ic705CoreThree,
		// The IC-9700 (Wave 4 task R5): the same three candidate fields
		// again, independently derived across all three of this radio's
		// banks — see ic9700CoreThree's own doc comment for why this
		// model gets its own variable too.
		"IC-9700": ic9700CoreThree,
		// The IC-905 (Wave 4 task R6, the tier's LAST registration): the
		// same three candidate fields again, independently derived — see
		// ic905CoreThree's own doc comment for why this model gets its
		// own variable too.
		"IC-905": ic905CoreThree,
		// The IC-7851 and IC-7850 (Tier 4b): the same three candidate
		// fields again, and here ONE variable serves both rows because
		// ONE driver does — see ic7851CoreThree's own doc comment for why
		// that is structural rather than a shortcut, and why each row
		// still gets its own entry in this map.
		"IC-7851": ic7851CoreThree,
		"IC-7850": ic7851CoreThree,
		// The IC-7760 (Tier 4b's second registration): the same three
		// candidate fields once more, from its own driver's own table —
		// see ic7760CoreThree's doc comment for why it is a separate
		// variable rather than a reuse of either set above.
		"IC-7760": ic7760CoreThree,
		// The IC-7100 (Tier 4b's third registration): the same three
		// candidate fields once more, from its own driver's own table —
		// see ic7100CoreThree's doc comment for why it is a separate
		// variable rather than a reuse of any set above.
		"IC-7100": ic7100CoreThree,
		// The IC-R8600 (Tier 4b's fourth and last registration, and the
		// registry's only RECEIVER): the same three candidate fields once
		// more, from its own driver's own table — see icr8600CoreThree's
		// doc comment for why being receive-only costs nothing in THIS
		// set, none of the two fields spec.ReceiveOnly removes being a
		// bankCoreCandidates member.
		"IC-R8600": icr8600CoreThree,
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

// TestBankReadOnly_RegisteredFTdx101D_RealHardwareProfile and its MP sibling
// are the FTdx10 test above's counterparts for the two models M9d-2
// registered, and they pin the same rule against the same premise: both
// radios' RealHardware profile is the all-Unverified one
// (writeTrialsCompleteD and writeTrialsCompleteMP are both false — no
// FTDX101 of either model has ever been written to by this project), so
// their derived fields are Write spec.Unverified on MEM and PMS, which is
// NOT spec.Unsupported and therefore NOT read-only under bankReadOnly's
// standing rule. Those two banks stay EDITABLE and every write is refused
// later, at the capability gate.
//
// See TestBankReadOnly_RegisteredFTdx10_RealHardwareProfile's doc comment
// for why that is the right verdict and for the milestone-spec sentence it
// declines to implement; the reasoning is the rule's, not the FTdx10's, and
// applies here unchanged.
//
// EACH MODEL INDEPENDENTLY, static AND discovered, is what spec A6 asks
// for, and the independence is the point: these two radios share a dialect
// config, so a single test parameterised over "the FTdx101" would pass
// against a registration that had wired both keys to the same driver. Each
// test opens its OWN model's registered fake through its OWN option var.
// The D-vs-MP equality check lives separately, below, as a SUPPLEMENTAL
// assertion — never as the primary one, since two models sharing one wrong
// answer would satisfy an equality proof perfectly.
func TestBankReadOnly_RegisteredFTdx101D_RealHardwareProfile(t *testing.T) {
	assertFTdx101BankReadOnly(t, "FTdx101D")
}

// TestBankReadOnly_RegisteredFTdx101MP_RealHardwareProfile: see the D's doc
// comment.
func TestBankReadOnly_RegisteredFTdx101MP_RealHardwareProfile(t *testing.T) {
	assertFTdx101BankReadOnly(t, "FTdx101MP")
}

// assertFTdx101BankReadOnly runs both halves of the bankReadOnly class for
// one FTdx101 model and returns the verdicts it observed, keyed by bank, so
// the supplemental D-vs-MP comparison below can be built from the very
// values these tests asserted rather than from a second derivation.
//
// Half one, the STATIC baseline: every core field of every bank must be
// Write Unverified (the premise), and no bank may be read-only.
//
// Half two, a DISCOVERED 5 MHz bank, whose Writes ARE forced Unsupported
// (no profile may claim a 5xx slot writable): the same six fields derived,
// and read-only TRUE. One capability set, two different verdicts, from one
// rule — and the contrast is what keeps half one from passing because
// bankReadOnly always answers false.
func assertFTdx101BankReadOnly(t *testing.T, model string) map[spec.BankID]bool {
	t.Helper()
	verdicts := map[spec.BankID]bool{}

	caps, err := wiring.StaticCapabilities(model)
	if err != nil {
		t.Fatalf("wiring.StaticCapabilities(%q): unexpected error: %v", model, err)
	}
	if len(caps.Banks) == 0 {
		t.Fatalf("the registered %s's static baseline has no banks — nothing asserted", model)
	}
	for _, b := range caps.Banks {
		fields := bankCoreFields(caps, b.ID)
		if len(fields) == 0 {
			t.Fatalf("%s bank %s derives no core fields — the Write check below would be vacuous", model, b.ID)
		}
		for _, f := range fields {
			if got := caps.FieldSupport(b.ID, f).Write; got != spec.Unverified {
				t.Errorf("%s bank %s field %s Write = %v, want Unverified (the premise: nothing on a real %s is proven writable)", model, b.ID, f, got, model)
			}
		}
		verdicts[b.ID] = bankReadOnly(caps, b.ID)
		if verdicts[b.ID] {
			t.Errorf("%s bankReadOnly(%s) = true, want false — Unverified is not Unsupported, and locking it would break the offline clone workflow", model, b.ID)
		}
	}

	// A discovered 5 MHz bank, whose Writes ARE Unsupported: read-only.
	// Each model is steered through ITS OWN option variable — the pair that
	// internal/wiring keeps separate by which closure reads which, since
	// both are []fakedx101.Option and the compiler cannot tell them apart.
	restore := setFTdx101FakeOpts(t, model, []fakedx101.Option{fakedx101.With5xx()})
	defer restore()

	sess, closeAll, err := wiring.OpenFakeSessionFor(testAppCtx(t), model)
	if err != nil {
		t.Fatalf("wiring.OpenFakeSessionFor(%q): unexpected error: %v", model, err)
	}
	t.Cleanup(func() { _ = closeAll() })
	live := sess.Capabilities()
	if _, ok := live.Bank(spec.Bank60m); !ok {
		t.Fatalf("the 5xx-populated %s fake produced no 60M bank — the contrast half of this test would be vacuous (and its option var did not reach its rig)", model)
	}
	wantFields(t, model+"'s discovered 60M bank", bankCoreFields(live, spec.Bank60m), ftdx10CoreSix)
	verdicts[spec.Bank60m] = bankReadOnly(live, spec.Bank60m)
	if !verdicts[spec.Bank60m] {
		t.Errorf("%s bankReadOnly(60M) = false, want true — no profile may claim a discovered 5xx slot writable", model)
	}
	return verdicts
}

// setFTdx101FakeOpts points the given model's OWN option variable at opts
// and returns a function restoring the previous value. It exists so the
// tests in this file cannot accidentally set the sibling's variable — the
// one mistake the type system cannot catch here, since both variables are
// []fakedx101.Option.
func setFTdx101FakeOpts(t *testing.T, model string, opts []fakedx101.Option) func() {
	t.Helper()
	switch model {
	case "FTdx101D":
		prev := wiring.FTdx101DFakeSessionOpts
		wiring.FTdx101DFakeSessionOpts = opts
		return func() { wiring.FTdx101DFakeSessionOpts = prev }
	case "FTdx101MP":
		prev := wiring.FTdx101MPFakeSessionOpts
		wiring.FTdx101MPFakeSessionOpts = opts
		return func() { wiring.FTdx101MPFakeSessionOpts = prev }
	default:
		t.Fatalf("setFTdx101FakeOpts: %q is not an FTdx101 model", model)
		return func() {}
	}
}

// TestBankReadOnly_FTdx101DAndMPAgree is a SUPPLEMENTAL assertion and is
// explicitly not the proof of anything on its own (spec A6): the two models
// must each be checked against the RULE, which the two tests above do, and
// this only adds that they reached the same verdicts.
//
// It is worth having because a divergence would be genuinely surprising —
// the matrix says the two radios' §2 values are identical throughout (§2.5),
// and they share a dialect config — so a difference here means either a
// registration wired one model to something else's driver, or a capability
// set acquired a model dimension that nothing in the evidence supports.
//
// It is worth NOT trusting alone because a shared wrong answer satisfies it
// perfectly: two models both reporting every bank read-only, or neither,
// would compare equal and be equally wrong.
func TestBankReadOnly_FTdx101DAndMPAgree(t *testing.T) {
	d := assertFTdx101BankReadOnly(t, "FTdx101D")
	mp := assertFTdx101BankReadOnly(t, "FTdx101MP")
	if len(d) == 0 {
		t.Fatal("no bank verdicts collected — this comparison would hold vacuously")
	}
	if !reflect.DeepEqual(d, mp) {
		t.Errorf("bankReadOnly verdicts differ between the siblings:\n  FTdx101D  = %v\n  FTdx101MP = %v\nthe two radios share one dialect config and one capability shape (matrix §2.5), so a difference means a registration or a capability set has acquired a model dimension nothing supports", d, mp)
	}
}

// TestBankReadOnly_RegisteredIC7610_RealHardwareProfile pins what a REAL
// IC-7610's grid does today, bank by bank, through real registration —
// Wave 4 task R1's mirror of
// TestBankReadOnly_RegisteredFTdx10_RealHardwareProfile.
//
// The IC-7610's RealHardware profile is its all-Unverified one
// (writeTrialsComplete is false: no IC-7610 has ever been written to by
// this project), so its three derived core fields are Write
// spec.Unverified on both banks — which is NOT spec.Unsupported, and
// therefore NOT read-only under bankReadOnly's standing rule, on the same
// footing as every Yaesu row.
//
// NO DISCOVERED-BANK CONTRAST, unlike the FTdx10/FTdx101 exemplars: this
// radio has no 60M/EMG-style discovery mechanism at all. Its Banks are
// fixed at construction — spec.BankMemory and spec.BankScan,
// core/driver/ic7610/caps.go's baseCapabilities — so there is no
// discovered bank whose Writes are forced Unsupported to contrast
// against; the static baseline is the whole of what this radio's
// bankReadOnly verdict can be.
func TestBankReadOnly_RegisteredIC7610_RealHardwareProfile(t *testing.T) {
	caps, err := wiring.StaticCapabilities(wiring.IC7610Model)
	if err != nil {
		t.Fatalf("wiring.StaticCapabilities(%q): unexpected error: %v", wiring.IC7610Model, err)
	}
	if len(caps.Banks) == 0 {
		t.Fatal("the registered IC-7610's static baseline has no banks — nothing asserted")
	}
	for _, b := range caps.Banks {
		fields := bankCoreFields(caps, b.ID)
		wantFields(t, "IC-7610 bank "+string(b.ID), fields, ic7610CoreThree)
		for _, f := range fields {
			if got := caps.FieldSupport(b.ID, f).Write; got != spec.Unverified {
				t.Errorf("bank %s field %s Write = %v, want Unverified (the premise: nothing on a real IC-7610 is proven writable)", b.ID, f, got)
			}
		}
		if bankReadOnly(caps, b.ID) {
			t.Errorf("bankReadOnly(%s) = true, want false — Unverified is not Unsupported, and locking it would break the offline clone workflow", b.ID)
		}
	}
}

// TestGetUISpec_RegisteredIC7610_EveryBankFieldsAndTagDisplay is Wave 4
// task R1's mirror of TestGetUISpec_RegisteredFTdx10_EveryBankUnavailable
// (M9c-6 D5c): GetUISpec driven for the IC-7610 through real registration,
// connected and offline, and the first time this project's BankView.Fields
// is populated by a REAL registered model's real capability data rather
// than being nil, as bankTierFields' own doc comment says every model
// registered before this one leaves it.
//
//   - CONNECTED to the registered fake (Live true, the Simulated profile —
//     the `--fake --model IC-7610` path a user actually walks). Unlike the
//     FTdx10/FTdx101 exemplars this radio discovers no extra bank, so
//     "every bank" here is just MEM and SCAN.
//   - DISCONNECTED with an IC-7610 working copy loaded (Live false, the
//     static RealHardware baseline, resolved by currentModel from the
//     file's own Radio.Model) — the offline clone workflow's path.
//
// TagDisplayDefault must be {state: "unavailable"} on both banks of both
// paths: the IC-7610's 1A 00 record has no display flag at all
// (FieldTagDisplay carries the zero FieldSupport on both banks,
// core/driver/ic7610/caps.go's bankFields), so a blank row added anywhere
// in either bank must not carry a Known one. The FT-710 assertion at the
// end is the contrast that stops the whole thing passing because
// something returned a zero value.
//
// Fields must equal ic7610TierFields on both banks of both paths: this
// radio's memory record maps tone_mode, tone_tx, tone_rx and filter (and
// none of the other six tier-added fields), and bankTierFields must derive
// that same four-field list from whichever capability source GetUISpec
// used — the connected session's effective set, or the static baseline —
// exactly as it must for TagDisplayDefault.
func TestGetUISpec_RegisteredIC7610_EveryBankFieldsAndTagDisplay(t *testing.T) {
	unavailable := codeplug.BoolField{State: codeplug.Unavailable}

	sess, closeAll, err := wiring.OpenFakeSessionFor(testAppCtx(t), wiring.IC7610Model)
	if err != nil {
		t.Fatalf("wiring.OpenFakeSessionFor(%q): unexpected error: %v", wiring.IC7610Model, err)
	}
	t.Cleanup(func() { _ = closeAll() })

	a, _ := newTestApp(t)
	connectDirect(t, a, sess, nil)
	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec (connected to the IC-7610 fake): unexpected error: %v", err)
	}
	if !got.Live {
		t.Error("Live = false, want true (connected to the registered fake)")
	}
	if len(got.Banks) != 2 {
		t.Fatalf("banks = %v, want exactly MEM and SCAN — this radio discovers no extra bank", bankIDs(got.Banks))
	}
	for _, b := range got.Banks {
		if b.TagDisplayDefault != unavailable {
			t.Errorf("connected IC-7610 bank %s TagDisplayDefault = %+v, want %+v — this radio's memory frame has no display flag", b.ID, b.TagDisplayDefault, unavailable)
		}
		if !reflect.DeepEqual(b.Fields, ic7610TierFields) {
			t.Errorf("connected IC-7610 bank %s Fields = %v, want %v", b.ID, b.Fields, ic7610TierFields)
		}
	}

	// Offline, from an IC-7610 file: the same answers, from the static
	// RealHardware baseline this time.
	a.mu.Lock()
	a.conn = nil
	a.working = &codeplug.Codeplug{
		Schema:   codeplug.CurrentSchema,
		Radio:    codeplug.RadioInfo{Model: wiring.IC7610Model},
		Channels: []codeplug.Channel{{Slot: "001"}},
	}
	a.mu.Unlock()
	offline, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec (offline, IC-7610 working copy): unexpected error: %v", err)
	}
	if offline.Live {
		t.Error("Live = true, want false (disconnected)")
	}
	if len(offline.Banks) == 0 {
		t.Fatal("offline IC-7610 UISpec has no banks — nothing asserted")
	}
	for _, b := range offline.Banks {
		if b.TagDisplayDefault != unavailable {
			t.Errorf("offline IC-7610 bank %s TagDisplayDefault = %+v, want %+v", b.ID, b.TagDisplayDefault, unavailable)
		}
		if !reflect.DeepEqual(b.Fields, ic7610TierFields) {
			t.Errorf("offline IC-7610 bank %s Fields = %v, want %v", b.ID, b.Fields, ic7610TierFields)
		}
	}

	// The contrast: the FT-710, through the same offline path, still
	// answers Known-false and an empty Fields on every bank.
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
		if len(b.Fields) != 0 {
			t.Errorf("offline FT-710 bank %s Fields = %v, want empty — the FT-710 maps none of the Icom tier's fields", b.ID, b.Fields)
		}
	}
}

// TestBankReadOnly_RegisteredIC7300_RealHardwareProfile is Wave 4 task R3's
// mirror of TestBankReadOnly_RegisteredIC7610_RealHardwareProfile, for the
// FIRST of this task's registered pair.
//
// The IC-7300's RealHardware profile is its all-Unverified one
// (writeTrialsComplete is false: no IC-7300 has ever been written to by
// this project), so its three derived core fields are Write
// spec.Unverified on both banks — NOT spec.Unsupported, and therefore NOT
// read-only under bankReadOnly's standing rule, on the same footing as
// every other registered row.
//
// NO DISCOVERED-BANK CONTRAST, on the same footing as the IC-7610: this
// radio has no 60M/EMG-style discovery mechanism at all. Its Banks are
// fixed at construction — spec.BankMemory and spec.BankScan,
// core/driver/ic7300/caps.go's baseCapabilities.
func TestBankReadOnly_RegisteredIC7300_RealHardwareProfile(t *testing.T) {
	caps, err := wiring.StaticCapabilities(wiring.IC7300Model)
	if err != nil {
		t.Fatalf("wiring.StaticCapabilities(%q): unexpected error: %v", wiring.IC7300Model, err)
	}
	if len(caps.Banks) == 0 {
		t.Fatal("the registered IC-7300's static baseline has no banks — nothing asserted")
	}
	for _, b := range caps.Banks {
		fields := bankCoreFields(caps, b.ID)
		wantFields(t, "IC-7300 bank "+string(b.ID), fields, ic7300CoreThree)
		for _, f := range fields {
			if got := caps.FieldSupport(b.ID, f).Write; got != spec.Unverified {
				t.Errorf("bank %s field %s Write = %v, want Unverified (the premise: nothing on a real IC-7300 is proven writable)", b.ID, f, got)
			}
		}
		if bankReadOnly(caps, b.ID) {
			t.Errorf("bankReadOnly(%s) = true, want false — Unverified is not Unsupported, and locking it would break the offline clone workflow", b.ID)
		}
	}
}

// TestBankReadOnly_RegisteredIC7300MK2_RealHardwareProfile is the
// IC-7300MK2's own mirror, registered in the same commit — a SEPARATE test
// against a SEPARATE driver package's own StaticCapabilities lookup, on
// the same footing as every other model-specific test in this file: no
// loop over the pair, since the pair's evidence is independent.
func TestBankReadOnly_RegisteredIC7300MK2_RealHardwareProfile(t *testing.T) {
	caps, err := wiring.StaticCapabilities(wiring.IC7300MK2Model)
	if err != nil {
		t.Fatalf("wiring.StaticCapabilities(%q): unexpected error: %v", wiring.IC7300MK2Model, err)
	}
	if len(caps.Banks) == 0 {
		t.Fatal("the registered IC-7300MK2's static baseline has no banks — nothing asserted")
	}
	for _, b := range caps.Banks {
		fields := bankCoreFields(caps, b.ID)
		wantFields(t, "IC-7300MK2 bank "+string(b.ID), fields, ic7300mk2CoreThree)
		for _, f := range fields {
			if got := caps.FieldSupport(b.ID, f).Write; got != spec.Unverified {
				t.Errorf("bank %s field %s Write = %v, want Unverified (the premise: nothing on a real IC-7300MK2 is proven writable)", b.ID, f, got)
			}
		}
		if bankReadOnly(caps, b.ID) {
			t.Errorf("bankReadOnly(%s) = true, want false — Unverified is not Unsupported, and locking it would break the offline clone workflow", b.ID)
		}
	}
}

// TestBankReadOnly_RegisteredIC705_RealHardwareProfile is Wave 4 task R4's
// mirror of TestBankReadOnly_RegisteredIC7610_RealHardwareProfile, for
// this task's registered lone model.
//
// The IC-705's RealHardware profile is its all-Unverified one
// (writeTrialsComplete is false: no IC-705 has ever been written to by
// this project), so its three derived core fields are Write
// spec.Unverified on both banks — NOT spec.Unsupported, and therefore NOT
// read-only under bankReadOnly's standing rule, on the same footing as
// every other registered row.
//
// NO DISCOVERED-BANK CONTRAST, on the same footing as every other
// registered Icom model: this radio has no 60M/EMG-style discovery
// mechanism at all. Its Banks are fixed at construction —
// spec.BankMemory and spec.BankCall, core/driver/ic705/caps.go's
// baseCapabilities (the MEM bank is SPARSE — Groups/PerGroup/Budget, no
// static Slots — and the CALL bank is a fixed four-slot dense one; that
// shape difference does not change what StaticCapabilities' write-side
// grading is, which is the one thing this test asserts).
func TestBankReadOnly_RegisteredIC705_RealHardwareProfile(t *testing.T) {
	caps, err := wiring.StaticCapabilities(wiring.IC705Model)
	if err != nil {
		t.Fatalf("wiring.StaticCapabilities(%q): unexpected error: %v", wiring.IC705Model, err)
	}
	if len(caps.Banks) == 0 {
		t.Fatal("the registered IC-705's static baseline has no banks — nothing asserted")
	}
	for _, b := range caps.Banks {
		fields := bankCoreFields(caps, b.ID)
		wantFields(t, "IC-705 bank "+string(b.ID), fields, ic705CoreThree)
		for _, f := range fields {
			if got := caps.FieldSupport(b.ID, f).Write; got != spec.Unverified {
				t.Errorf("bank %s field %s Write = %v, want Unverified (the premise: nothing on a real IC-705 is proven writable)", b.ID, f, got)
			}
		}
		if bankReadOnly(caps, b.ID) {
			t.Errorf("bankReadOnly(%s) = true, want false — Unverified is not Unsupported, and locking it would break the offline clone workflow", b.ID)
		}
	}
}

// TestBankReadOnly_RegisteredIC9700_RealHardwareProfile is Wave 4 task
// R5's mirror of TestBankReadOnly_RegisteredIC705_RealHardwareProfile,
// for this task's registered lone model.
//
// The IC-9700's RealHardware profile is its all-Unverified one
// (writeTrialsComplete is false: no IC-9700 has ever been written to by
// this project), so its three derived core fields are Write
// spec.Unverified on EVERY bank — NOT spec.Unsupported, and therefore NOT
// read-only under bankReadOnly's standing rule, on the same footing as
// every other registered row.
//
// THREE BANKS, not two: MEM, SCAN and CALL (core/driver/ic9700/caps.go's
// banks), all DENSE and all graded identically by bankFields — the loop
// below walks all three and the len(caps.Banks) assertion pins the count
// so a future change that silently dropped one of them would fail here
// rather than in a test that only ever iterates what caps.Banks happens
// to return.
//
// NO DISCOVERED-BANK CONTRAST, on the same footing as every other
// registered Icom model: this radio has no 60M/EMG-style discovery
// mechanism at all. Its three Banks are fixed at construction, and their
// DENSE shape (321 addressable slots total, completely enumerable) does
// not change what StaticCapabilities' write-side grading is, which is
// the one thing this test asserts.
func TestBankReadOnly_RegisteredIC9700_RealHardwareProfile(t *testing.T) {
	caps, err := wiring.StaticCapabilities(wiring.IC9700Model)
	if err != nil {
		t.Fatalf("wiring.StaticCapabilities(%q): unexpected error: %v", wiring.IC9700Model, err)
	}
	if len(caps.Banks) != 3 {
		t.Fatalf("the registered IC-9700's static baseline has %d banks, want exactly 3 (MEM, SCAN, CALL)", len(caps.Banks))
	}
	for _, b := range caps.Banks {
		fields := bankCoreFields(caps, b.ID)
		wantFields(t, "IC-9700 bank "+string(b.ID), fields, ic9700CoreThree)
		for _, f := range fields {
			if got := caps.FieldSupport(b.ID, f).Write; got != spec.Unverified {
				t.Errorf("bank %s field %s Write = %v, want Unverified (the premise: nothing on a real IC-9700 is proven writable)", b.ID, f, got)
			}
		}
		if bankReadOnly(caps, b.ID) {
			t.Errorf("bankReadOnly(%s) = true, want false — Unverified is not Unsupported, and locking it would break the offline clone workflow", b.ID)
		}
	}
}

// TestBankReadOnly_RegisteredIC905_RealHardwareProfile is Wave 4 task
// R6's mirror of TestBankReadOnly_RegisteredIC705_RealHardwareProfile,
// for this task's registered lone model — the tier's LAST registration.
//
// The IC-905's RealHardware profile is its all-Unverified one
// (writeTrialsComplete is false: no IC-905 has ever been written to by
// this project), so its three derived core fields are Write
// spec.Unverified on both banks — NOT spec.Unsupported, and therefore NOT
// read-only under bankReadOnly's standing rule, on the same footing as
// every other registered row.
//
// NO DISCOVERED-BANK CONTRAST for THIS derivation, on the same footing as
// every other registered Icom model: bankCoreFields' candidate universe
// (frequency, mode, tag and six Yaesu-only fields) does not include any
// of the tier-added fields a sparse MEM bank's discovery walk touches, so
// what discovery finds has no bearing on what this test asserts. Its two
// Banks are fixed at construction — spec.BankMemory and spec.BankCall,
// core/driver/ic905/caps.go's baseCapabilities (MEM is SPARSE —
// Groups/PerGroup/Budget, no static Slots; CALL is a fixed twelve-slot
// dense one, a distinct namespace from MEM's per ruling R4) — and that
// shape difference does not change what StaticCapabilities' write-side
// grading is, which is the one thing this test asserts.
func TestBankReadOnly_RegisteredIC905_RealHardwareProfile(t *testing.T) {
	caps, err := wiring.StaticCapabilities(wiring.IC905Model)
	if err != nil {
		t.Fatalf("wiring.StaticCapabilities(%q): unexpected error: %v", wiring.IC905Model, err)
	}
	if len(caps.Banks) == 0 {
		t.Fatal("the registered IC-905's static baseline has no banks — nothing asserted")
	}
	for _, b := range caps.Banks {
		fields := bankCoreFields(caps, b.ID)
		wantFields(t, "IC-905 bank "+string(b.ID), fields, ic905CoreThree)
		for _, f := range fields {
			if got := caps.FieldSupport(b.ID, f).Write; got != spec.Unverified {
				t.Errorf("bank %s field %s Write = %v, want Unverified (the premise: nothing on a real IC-905 is proven writable)", b.ID, f, got)
			}
		}
		if bankReadOnly(caps, b.ID) {
			t.Errorf("bankReadOnly(%s) = true, want false — Unverified is not Unsupported, and locking it would break the offline clone workflow", b.ID)
		}
	}
}

// TestGetUISpec_RegisteredIC7300_EveryBankFieldsAndTagDisplay is Wave 4
// task R3's mirror of
// TestGetUISpec_RegisteredIC7610_EveryBankFieldsAndTagDisplay: GetUISpec
// driven for the IC-7300 through real registration, connected and offline.
//
//   - CONNECTED to the registered fake (Live true, the Simulated profile —
//     the `--fake --model IC-7300` path a user actually walks). This radio
//     discovers no extra bank, so "every bank" here is just MEM and SCAN.
//   - DISCONNECTED with an IC-7300 working copy loaded (Live false, the
//     static RealHardware baseline, resolved by currentModel from the
//     file's own Radio.Model) — the offline clone workflow's path.
//
// TagDisplayDefault must be {state: "unavailable"} on both banks of both
// paths: the IC-7300's 1A 00 record has no display flag at all
// (FieldTagDisplay carries the zero FieldSupport on both banks,
// core/driver/ic7300/caps.go's bankFields).
//
// Fields must equal ic7300TierFields on both banks of both paths: SIX
// tier-added fields, not the IC-7610's four — see ic7300TierFields' own
// doc comment for which of this radio's record maps and which it does
// not.
func TestGetUISpec_RegisteredIC7300_EveryBankFieldsAndTagDisplay(t *testing.T) {
	unavailable := codeplug.BoolField{State: codeplug.Unavailable}

	sess, closeAll, err := wiring.OpenFakeSessionFor(testAppCtx(t), wiring.IC7300Model)
	if err != nil {
		t.Fatalf("wiring.OpenFakeSessionFor(%q): unexpected error: %v", wiring.IC7300Model, err)
	}
	t.Cleanup(func() { _ = closeAll() })

	a, _ := newTestApp(t)
	connectDirect(t, a, sess, nil)
	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec (connected to the IC-7300 fake): unexpected error: %v", err)
	}
	if !got.Live {
		t.Error("Live = false, want true (connected to the registered fake)")
	}
	if len(got.Banks) != 2 {
		t.Fatalf("banks = %v, want exactly MEM and SCAN — this radio discovers no extra bank", bankIDs(got.Banks))
	}
	for _, b := range got.Banks {
		if b.TagDisplayDefault != unavailable {
			t.Errorf("connected IC-7300 bank %s TagDisplayDefault = %+v, want %+v — this radio's memory frame has no display flag", b.ID, b.TagDisplayDefault, unavailable)
		}
		if !reflect.DeepEqual(b.Fields, ic7300TierFields) {
			t.Errorf("connected IC-7300 bank %s Fields = %v, want %v", b.ID, b.Fields, ic7300TierFields)
		}
	}

	// Offline, from an IC-7300 file: the same answers, from the static
	// RealHardware baseline this time.
	a.mu.Lock()
	a.conn = nil
	a.working = &codeplug.Codeplug{
		Schema:   codeplug.CurrentSchema,
		Radio:    codeplug.RadioInfo{Model: wiring.IC7300Model},
		Channels: []codeplug.Channel{{Slot: "001"}},
	}
	a.mu.Unlock()
	offline, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec (offline, IC-7300 working copy): unexpected error: %v", err)
	}
	if offline.Live {
		t.Error("Live = true, want false (disconnected)")
	}
	if len(offline.Banks) == 0 {
		t.Fatal("offline IC-7300 UISpec has no banks — nothing asserted")
	}
	for _, b := range offline.Banks {
		if b.TagDisplayDefault != unavailable {
			t.Errorf("offline IC-7300 bank %s TagDisplayDefault = %+v, want %+v", b.ID, b.TagDisplayDefault, unavailable)
		}
		if !reflect.DeepEqual(b.Fields, ic7300TierFields) {
			t.Errorf("offline IC-7300 bank %s Fields = %v, want %v", b.ID, b.Fields, ic7300TierFields)
		}
	}

	// The contrast: the FT-710, through the same offline path, still
	// answers Known-false and an empty Fields on every bank.
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
		if len(b.Fields) != 0 {
			t.Errorf("offline FT-710 bank %s Fields = %v, want empty — the FT-710 maps none of the Icom tier's fields", b.ID, b.Fields)
		}
	}
}

// TestGetUISpec_RegisteredIC7300MK2_EveryBankFieldsAndTagDisplay is the
// IC-7300MK2's own mirror, registered in the same commit — a SEPARATE test
// against a SEPARATE driver package and a SEPARATE fake
// (internal/fakeic7300mk2), on the same footing as every other
// model-specific acceptance test in this file.
func TestGetUISpec_RegisteredIC7300MK2_EveryBankFieldsAndTagDisplay(t *testing.T) {
	unavailable := codeplug.BoolField{State: codeplug.Unavailable}

	sess, closeAll, err := wiring.OpenFakeSessionFor(testAppCtx(t), wiring.IC7300MK2Model)
	if err != nil {
		t.Fatalf("wiring.OpenFakeSessionFor(%q): unexpected error: %v", wiring.IC7300MK2Model, err)
	}
	t.Cleanup(func() { _ = closeAll() })

	a, _ := newTestApp(t)
	connectDirect(t, a, sess, nil)
	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec (connected to the IC-7300MK2 fake): unexpected error: %v", err)
	}
	if !got.Live {
		t.Error("Live = false, want true (connected to the registered fake)")
	}
	if len(got.Banks) != 2 {
		t.Fatalf("banks = %v, want exactly MEM and SCAN — this radio discovers no extra bank", bankIDs(got.Banks))
	}
	for _, b := range got.Banks {
		if b.TagDisplayDefault != unavailable {
			t.Errorf("connected IC-7300MK2 bank %s TagDisplayDefault = %+v, want %+v — this radio's memory frame has no display flag", b.ID, b.TagDisplayDefault, unavailable)
		}
		if !reflect.DeepEqual(b.Fields, ic7300mk2TierFields) {
			t.Errorf("connected IC-7300MK2 bank %s Fields = %v, want %v", b.ID, b.Fields, ic7300mk2TierFields)
		}
	}

	// Offline, from an IC-7300MK2 file: the same answers, from the static
	// RealHardware baseline this time.
	a.mu.Lock()
	a.conn = nil
	a.working = &codeplug.Codeplug{
		Schema:   codeplug.CurrentSchema,
		Radio:    codeplug.RadioInfo{Model: wiring.IC7300MK2Model},
		Channels: []codeplug.Channel{{Slot: "001"}},
	}
	a.mu.Unlock()
	offline, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec (offline, IC-7300MK2 working copy): unexpected error: %v", err)
	}
	if offline.Live {
		t.Error("Live = true, want false (disconnected)")
	}
	if len(offline.Banks) == 0 {
		t.Fatal("offline IC-7300MK2 UISpec has no banks — nothing asserted")
	}
	for _, b := range offline.Banks {
		if b.TagDisplayDefault != unavailable {
			t.Errorf("offline IC-7300MK2 bank %s TagDisplayDefault = %+v, want %+v", b.ID, b.TagDisplayDefault, unavailable)
		}
		if !reflect.DeepEqual(b.Fields, ic7300mk2TierFields) {
			t.Errorf("offline IC-7300MK2 bank %s Fields = %v, want %v", b.ID, b.Fields, ic7300mk2TierFields)
		}
	}

	// The contrast: the FT-710, through the same offline path, still
	// answers Known-false and an empty Fields on every bank.
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
		if len(b.Fields) != 0 {
			t.Errorf("offline FT-710 bank %s Fields = %v, want empty — the FT-710 maps none of the Icom tier's fields", b.ID, b.Fields)
		}
	}
}

// TestGetUISpec_RegisteredIC705_EveryBankFieldsAndTagDisplay is Wave 4
// task R4's mirror of
// TestGetUISpec_RegisteredIC7610_EveryBankFieldsAndTagDisplay: GetUISpec
// driven for the IC-705 through real registration, connected and offline.
//
//   - CONNECTED to the registered fake (Live true, the Simulated profile —
//     the `--fake --model IC-705` path a user actually walks), seeded with
//     fakeic705.DefaultImage through the registered IC705FakeSessionOpts
//     seam so that the open-time inventory walk has something to
//     materialise: an unseeded fake answers an EMPTY MEM bank, and the
//     connected half of this test would then assert nothing about the one
//     field fix round 1 is about. This radio discovers no extra bank, so
//     "every bank" here is exactly MEM and CALL — pinned BY ID, not by
//     count (fix round 1, F3).
//   - DISCONNECTED with an IC-705 working copy loaded (Live false, the
//     static RealHardware baseline, resolved by currentModel from the
//     file's own Radio.Model) — the offline clone workflow's path.
//
// BankView.Slots is asserted on BOTH banks of BOTH paths, and that is what
// fix round 1 (F1/F2) turns on. The working copy carries THIS radio's own
// slot strings ("G01-001" in MEM, "G101-001" in CALL), not the FT-710's
// "001" this test was first written with, and the MEM bank's static Slots
// is nil BY CONTRACT (core/driver/ic705/caps.go: a sparse bank's Slots
// lists what a READ MATERIALISED, and the static baseline has read
// nothing). Under the old literal-membership rule every MEM channel of an
// offline IC-705 working copy therefore classified into NO bank at all and
// vanished from the grid; under spec.Bank.WithinSpace they land in MEM —
// "G02-050" included, an address no read has ever materialised and that
// the user is perfectly entitled to add.
//
// "G101-005" is the control. It is outside CALL's four fixed slots and
// outside MEM's hundred-group space alike, so it must stay an ORPHAN: no
// bank may claim it, and no synthesised bank rescues it either (that
// rescue is driver.DiscoveredBankSynthesizer's, which this driver does not
// implement — see synthesiseDiscoveredBanks). The new rule must widen the
// sparse bank's answer to its declared space and not one address further.
//
// TagDisplayDefault must be {state: "unavailable"} on both banks of both
// paths: the IC-705's 1A 00 record has no display flag at all
// (FieldTagDisplay carries the zero FieldSupport on both banks,
// core/driver/ic705/caps.go's bankFields).
//
// Fields must equal ic705TierFields on both banks of both paths: ALL TEN
// tier-added fields, not the IC-7610's four or the IC-7300 pair's six —
// see ic705TierFields' own doc comment for why this radio's record maps
// more of the tier than any other registered Icom model.
func TestGetUISpec_RegisteredIC705_EveryBankFieldsAndTagDisplay(t *testing.T) {
	unavailable := codeplug.BoolField{State: codeplug.Unavailable}
	wantBanks := []string{"MEM", "CALL"}

	prevOpts := wiring.IC705FakeSessionOpts
	wiring.IC705FakeSessionOpts = []fakeic705.Option{fakeic705.WithFactoryImage(fakeic705.DefaultImage())}
	t.Cleanup(func() { wiring.IC705FakeSessionOpts = prevOpts })

	sess, closeAll, err := wiring.OpenFakeSessionFor(testAppCtx(t), wiring.IC705Model)
	if err != nil {
		t.Fatalf("wiring.OpenFakeSessionFor(%q): unexpected error: %v", wiring.IC705Model, err)
	}
	t.Cleanup(func() { _ = closeAll() })

	a, _ := newTestApp(t)
	connectDirect(t, a, sess, nil)
	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec (connected to the IC-705 fake): unexpected error: %v", err)
	}
	if !got.Live {
		t.Error("Live = false, want true (connected to the registered fake)")
	}
	if !reflect.DeepEqual(bankIDs(got.Banks), wantBanks) {
		t.Fatalf("connected banks = %v, want exactly %v — this radio discovers no extra bank", bankIDs(got.Banks), wantBanks)
	}
	// Connected, caps' own list is authoritative: the four slots
	// fakeic705.DefaultImage seeds within the bounded walk's reach, in the
	// walk's own ascending order, under slots.go's one wire = display − 1
	// rule (wire 0/0, 0/1, 0/7 and 1/0).
	wantLiveMem := ic705SlotViews("G01-001", "G01-002", "G01-008", "G02-001")
	if liveMem := findBank(t, got.Banks, "MEM").Slots; !reflect.DeepEqual(liveMem, wantLiveMem) {
		t.Errorf("connected IC-705 MEM.Slots = %v, want %v (the seeded inventory the open-time walk materialised)", liveMem, wantLiveMem)
	}
	// CALL is DENSE and fixed: its four slots come from the static bank
	// itself, seeded or not.
	wantCall := ic705SlotViews("G101-001", "G101-002", "G101-003", "G101-004")
	if liveCall := findBank(t, got.Banks, "CALL").Slots; !reflect.DeepEqual(liveCall, wantCall) {
		t.Errorf("connected IC-705 CALL.Slots = %v, want %v (four fixed call channels)", liveCall, wantCall)
	}
	for _, b := range got.Banks {
		if b.TagDisplayDefault != unavailable {
			t.Errorf("connected IC-705 bank %s TagDisplayDefault = %+v, want %+v — this radio's memory frame has no display flag", b.ID, b.TagDisplayDefault, unavailable)
		}
		if !reflect.DeepEqual(b.Fields, ic705TierFields) {
			t.Errorf("connected IC-705 bank %s Fields = %v, want %v", b.ID, b.Fields, ic705TierFields)
		}
	}

	// Offline, from an IC-705 file: the same answers, from the static
	// RealHardware baseline this time — and the working copy's own slots,
	// classified by each bank's SPACE.
	a.mu.Lock()
	a.conn = nil
	a.working = &codeplug.Codeplug{
		Schema: codeplug.CurrentSchema,
		Radio:  codeplug.RadioInfo{Model: wiring.IC705Model},
		Channels: []codeplug.Channel{
			{Slot: "G01-001"},  // MEM, and one the fake's walk did materialise
			{Slot: "G101-001"}, // CALL, a listed slot of a dense bank
			{Slot: "G02-050"},  // MEM by SPACE alone: in no Slots list, anywhere
			{Slot: "G101-005"}, // in neither bank's space: an orphan, by design
		},
	}
	a.mu.Unlock()
	offline, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec (offline, IC-705 working copy): unexpected error: %v", err)
	}
	if offline.Live {
		t.Error("Live = true, want false (disconnected)")
	}
	if !reflect.DeepEqual(bankIDs(offline.Banks), wantBanks) {
		t.Fatalf("offline banks = %v, want exactly %v — this driver synthesises no discovered bank", bankIDs(offline.Banks), wantBanks)
	}
	wantOfflineMem := ic705SlotViews("G01-001", "G02-050")
	if offMem := findBank(t, offline.Banks, "MEM").Slots; !reflect.DeepEqual(offMem, wantOfflineMem) {
		t.Errorf("offline IC-705 MEM.Slots = %v, want %v — a SPARSE bank's working-copy slots are classified by its declared space (spec.Bank.WithinSpace), not by the Slots list it deliberately does not carry", offMem, wantOfflineMem)
	}
	wantOfflineCall := ic705SlotViews("G101-001")
	if offCall := findBank(t, offline.Banks, "CALL").Slots; !reflect.DeepEqual(offCall, wantOfflineCall) {
		t.Errorf("offline IC-705 CALL.Slots = %v, want %v — CALL is dense, so only its four listed slots may appear", offCall, wantOfflineCall)
	}
	for _, b := range offline.Banks {
		for _, s := range b.Slots {
			if s.Slot == "G101-005" {
				t.Errorf("offline IC-705 bank %s claims slot G101-005, which lies outside CALL's four fixed channels and outside MEM's hundred-group space alike — it must stay an orphan", b.ID)
			}
		}
		if b.TagDisplayDefault != unavailable {
			t.Errorf("offline IC-705 bank %s TagDisplayDefault = %+v, want %+v", b.ID, b.TagDisplayDefault, unavailable)
		}
		if !reflect.DeepEqual(b.Fields, ic705TierFields) {
			t.Errorf("offline IC-705 bank %s Fields = %v, want %v", b.ID, b.Fields, ic705TierFields)
		}
	}

	// The contrast: the FT-710, through the same offline path, still
	// answers Known-false and an empty Fields on every bank.
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
		if len(b.Fields) != 0 {
			t.Errorf("offline FT-710 bank %s Fields = %v, want empty — the FT-710 maps none of the Icom tier's fields", b.ID, b.Fields)
		}
	}
}

// TestGetUISpec_RegisteredIC9700_EveryBankFieldsAndTagDisplay is Wave 4
// task R5's mirror of
// TestGetUISpec_RegisteredIC705_EveryBankFieldsAndTagDisplay: GetUISpec
// driven for the IC-9700 through real registration, connected and
// offline — for a THREE-BANK, ALL-DENSE model, the first this file's
// exemplar tests have covered.
//
//   - CONNECTED to the registered fake (Live true, the Simulated profile
//     — the `--fake --model IC-9700` path a user actually walks). "Every
//     bank" here is MEM, SCAN and CALL: this radio discovers no extra
//     bank, and its Banks are fixed at construction
//     (core/driver/ic9700/caps.go's banks).
//   - DISCONNECTED with an IC-9700 working copy loaded (Live false, the
//     static RealHardware baseline, resolved by currentModel from the
//     file's own Radio.Model) — the offline clone workflow's path.
//
// BankView.Slots is asserted on ALL THREE banks of BOTH paths, with this
// radio's OWN slot spelling (core/driver/ic9700/slots.go's
// <band>-<channel> forms — "144-001", "144-P1A", "144-C1" and their
// siblings), never a borrowed placeholder like "001".
//
// CONNECTED: this radio is DENSE, not sparse (unlike the IC-705's MEM
// bank) — every bank's Slots is the SAME completely-enumerable canonical
// list whether or not anything is connected (core/driver/ic9700/caps.go's
// banks calls bankSlots(id) unconditionally, with no runtime discovery
// step), so the fake needs no seeding at all for this assertion to mean
// something: the expected lists here are RECOMPUTED from
// wiring.StaticCapabilities' own bank.Slots (the ground truth this
// driver's own slots.go builds), not hand-typed — the same
// recompute-rather-than-hardcode discipline
// TestGetUISpec_SlotClassification_DenseBanksUnchangedByWithinSpace uses
// for the four Yaesu models — because hand-typing 321 slot strings is 321
// chances to mistype one, exactly the risk core/driver/ic9700/slots.go's
// own bankSlots doc comment names. The sanity-check loop below confirms
// three REAL addresses are actually present in that recomputed list, so
// the comparison is not merely the derivation checked against itself.
//
// OFFLINE: the working copy carries a small, deliberately chosen set of
// this radio's OWN slot strings, one per bank plus an orphan
// ("999-001" — band 999 does not exist on this radio; matrix §1 #11's
// printed band codes are 01, 02 and 03 only), and the assertion is exact
// membership: on a DENSE bank spec.Bank.WithinSpace IS literal Slots
// membership (app/uispec.go's bankSlotViews doc comment), so only the
// three seeded real slots may appear, each in its own bank, and the
// orphan must appear in none.
//
// TagDisplayDefault must be {state: "unavailable"} on all three banks of
// both paths: the IC-9700's 1A 00 record has no display flag at all
// (FieldTagDisplay carries the zero FieldSupport on every bank,
// core/driver/ic9700/caps.go's bankFields).
//
// Fields must equal ic9700TierFields on all three banks of both paths:
// ALL TEN tier-added fields, on the same footing as the IC-705 — see
// ic9700TierFields' own doc comment.
func TestGetUISpec_RegisteredIC9700_EveryBankFieldsAndTagDisplay(t *testing.T) {
	unavailable := codeplug.BoolField{State: codeplug.Unavailable}
	wantBanks := []string{"MEM", "SCAN", "CALL"}

	staticCaps, err := wiring.StaticCapabilities(wiring.IC9700Model)
	if err != nil {
		t.Fatalf("wiring.StaticCapabilities(%q): unexpected error: %v", wiring.IC9700Model, err)
	}
	staticBankIDs := make([]string, len(staticCaps.Banks))
	for i, b := range staticCaps.Banks {
		staticBankIDs[i] = string(b.ID)
	}
	if !reflect.DeepEqual(staticBankIDs, wantBanks) {
		t.Fatalf("static banks = %v, want exactly %v", staticBankIDs, wantBanks)
	}
	// The want lists are built with THIS FILE'S OWN ic9700SlotViews, not
	// with app's slotViewsFor (R5 deferred minor). slotViewsFor is the
	// production helper GetUISpec itself calls, so building the
	// expectation with it made the comparison below a derivation checked
	// against itself: a bug in slotViewsFor would appear identically on
	// both sides and the test would stay green. ic9700SlotViews states
	// the mapping this radio's slots must satisfy — Display equals Slot,
	// because every IC-9700 slot string is longer than the three
	// characters codeplug.DisplaySlot's rewrite applies to — as an
	// independent claim.
	wantSlotsFor := make(map[string][]SlotView, 3)
	for _, b := range staticCaps.Banks {
		if b.Sparse {
			t.Fatalf("static bank %s is Sparse — the IC-9700 is DENSE by design (core/driver/ic9700/caps.go); this test's whole premise no longer holds", b.ID)
		}
		if len(b.Slots) == 0 {
			t.Fatalf("static bank %s lists no slots — nothing to compare against", b.ID)
		}
		wantSlotsFor[string(b.ID)] = ic9700SlotViews(b.Slots...)
	}
	// The per-bank COUNTS, pinned rather than taken on trust: three bands
	// times 99 memory channels, 6 program-scan edges and 2 call channels
	// (core/driver/ic9700/slots.go), 321 slots in all. Without these the
	// loop above would happily build its expectation from a truncated
	// static list and then confirm the same truncation on both paths.
	for _, want := range []struct {
		bank    string
		n       int
		perBand int
	}{{"MEM", 297, 99}, {"SCAN", 18, 6}, {"CALL", 6, 2}} {
		if got := len(wantSlotsFor[want.bank]); got != want.n {
			t.Fatalf("static bank %s lists %d slots, want %d (3 bands x %d) — this radio's canonical slot list has changed shape, and every comparison below would be against the new one rather than the one this test was written for", want.bank, got, want.n, want.perBand)
		}
	}
	// Spot-check: these are this radio's own canonical slot strings
	// (core/driver/ic9700/slots.go), not fabricated placeholders, and
	// their presence here is what makes the comparison below meaningful
	// rather than the derivation checked against itself.
	for bankID, want := range map[string]string{"MEM": "144-001", "SCAN": "144-P1A", "CALL": "144-C1"} {
		found := false
		for _, sv := range wantSlotsFor[bankID] {
			if sv.Slot == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("wantSlotsFor[%q] does not contain %q — this test's own fixture is wrong", bankID, want)
		}
	}

	sess, closeAll, err := wiring.OpenFakeSessionFor(testAppCtx(t), wiring.IC9700Model)
	if err != nil {
		t.Fatalf("wiring.OpenFakeSessionFor(%q): unexpected error: %v", wiring.IC9700Model, err)
	}
	t.Cleanup(func() { _ = closeAll() })

	a, _ := newTestApp(t)
	connectDirect(t, a, sess, nil)
	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec (connected to the IC-9700 fake): unexpected error: %v", err)
	}
	if !got.Live {
		t.Error("Live = false, want true (connected to the registered fake)")
	}
	if !reflect.DeepEqual(bankIDs(got.Banks), wantBanks) {
		t.Fatalf("connected banks = %v, want exactly %v — this radio discovers no extra bank", bankIDs(got.Banks), wantBanks)
	}
	for _, b := range got.Banks {
		if want := wantSlotsFor[b.ID]; !reflect.DeepEqual(b.Slots, want) {
			// The INDEX of the first difference, and the pair at it: a
			// 297-entry list printed whole says only that something
			// moved, and the position is what identifies which slot
			// (R5 deferred minor).
			at, gotAt, wantAt := firstSlotViewDiff(b.Slots, want)
			t.Errorf("connected IC-9700 bank %s Slots has %d entries, want %d matching the static baseline's own canonical list — a DENSE bank discovers nothing at open time; first difference at index %d: got %+v, want %+v", b.ID, len(b.Slots), len(want), at, gotAt, wantAt)
		}
		if b.TagDisplayDefault != unavailable {
			t.Errorf("connected IC-9700 bank %s TagDisplayDefault = %+v, want %+v — this radio's memory frame has no display flag", b.ID, b.TagDisplayDefault, unavailable)
		}
		if !reflect.DeepEqual(b.Fields, ic9700TierFields) {
			t.Errorf("connected IC-9700 bank %s Fields = %v, want %v", b.ID, b.Fields, ic9700TierFields)
		}
	}

	// Offline, from an IC-9700 file: the same answers, from the static
	// RealHardware baseline this time — and the working copy's own
	// slots, classified by each bank's SPACE (which on a DENSE bank is
	// literal Slots membership).
	a.mu.Lock()
	a.conn = nil
	a.working = &codeplug.Codeplug{
		Schema: codeplug.CurrentSchema,
		Radio:  codeplug.RadioInfo{Model: wiring.IC9700Model},
		Channels: []codeplug.Channel{
			{Slot: "144-001"}, // MEM, band 144 channel 1
			{Slot: "430-P2B"}, // SCAN, band 430, pair 2, half B
			{Slot: "1200-C2"}, // CALL, band 1200, call channel 2
			{Slot: "999-001"}, // no such band on this radio: an orphan
		},
	}
	a.mu.Unlock()
	offline, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec (offline, IC-9700 working copy): unexpected error: %v", err)
	}
	if offline.Live {
		t.Error("Live = true, want false (disconnected)")
	}
	if !reflect.DeepEqual(bankIDs(offline.Banks), wantBanks) {
		t.Fatalf("offline banks = %v, want exactly %v — this driver synthesises no discovered bank", bankIDs(offline.Banks), wantBanks)
	}
	wantOfflineMem := ic9700SlotViews("144-001")
	if offMem := findBank(t, offline.Banks, "MEM").Slots; !reflect.DeepEqual(offMem, wantOfflineMem) {
		t.Errorf("offline IC-9700 MEM.Slots = %v, want %v", offMem, wantOfflineMem)
	}
	wantOfflineScan := ic9700SlotViews("430-P2B")
	if offScan := findBank(t, offline.Banks, "SCAN").Slots; !reflect.DeepEqual(offScan, wantOfflineScan) {
		t.Errorf("offline IC-9700 SCAN.Slots = %v, want %v", offScan, wantOfflineScan)
	}
	wantOfflineCall := ic9700SlotViews("1200-C2")
	if offCall := findBank(t, offline.Banks, "CALL").Slots; !reflect.DeepEqual(offCall, wantOfflineCall) {
		t.Errorf("offline IC-9700 CALL.Slots = %v, want %v", offCall, wantOfflineCall)
	}
	for _, b := range offline.Banks {
		for _, s := range b.Slots {
			if s.Slot == "999-001" {
				t.Errorf("offline IC-9700 bank %s claims slot 999-001, which names a band this radio does not have — it must stay an orphan", b.ID)
			}
		}
		if b.TagDisplayDefault != unavailable {
			t.Errorf("offline IC-9700 bank %s TagDisplayDefault = %+v, want %+v", b.ID, b.TagDisplayDefault, unavailable)
		}
		if !reflect.DeepEqual(b.Fields, ic9700TierFields) {
			t.Errorf("offline IC-9700 bank %s Fields = %v, want %v", b.ID, b.Fields, ic9700TierFields)
		}
	}

	// The contrast: the FT-710, through the same offline path, still
	// answers Known-false and an empty Fields on every bank.
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
		if len(b.Fields) != 0 {
			t.Errorf("offline FT-710 bank %s Fields = %v, want empty — the FT-710 maps none of the Icom tier's fields", b.ID, b.Fields)
		}
	}
}

// TestGetUISpec_RegisteredIC905_EveryBankFieldsAndTagDisplay is Wave 4
// task R6's mirror of
// TestGetUISpec_RegisteredIC705_EveryBankFieldsAndTagDisplay: GetUISpec
// driven for the IC-905 through real registration, connected and offline —
// this project's FIFTH Icom registration, and the tier's LAST.
//
//   - CONNECTED to the registered fake (Live true, the Simulated profile —
//     the `--fake --model IC-905` path a user actually walks): LIKE
//     internal/fakeic705, the demo IC-905 opens EMPTY, not at
//     internal/fakeic905's own frozen default image (ten occupied
//     channels in group 0, image.go's defaultImage, all-zero content).
//     internal/wiring/fake.go's IC905Model row explains why: that default
//     image's records are UNDECODABLE by core/civ/ic905/profile.go's own
//     filter (it refuses byte 0x00), so the row issues ten
//     fakeic905.WithEmpty(0, ch) calls ahead of any IC905FakeSessionOpts
//     override, deleting every one of them before the open-time bounded
//     discovery walk runs — set to nil explicitly here anyway, so the
//     seam this test relies on is stated rather than merely inherited
//     from whatever ran before it. The walk therefore has nothing to
//     materialise in MEM; this radio discovers no extra bank either, so
//     "every bank" here is exactly MEM and CALL — pinned BY ID, not by
//     count.
//   - DISCONNECTED with an IC-905 working copy loaded (Live false, the
//     static RealHardware baseline, resolved by currentModel from the
//     file's own Radio.Model) — the offline clone workflow's path.
//
// BankView.Slots is asserted on both banks of both paths. The working
// copy carries THIS radio's own slot strings — "G01-001" in MEM
// (spec.SparseSlot's shared "G%02d-%03d" form) and "C01" in CALL
// (core/civ/ic905's own CallSlot, "C01".."C12", a namespace
// spec.ParseSparseSlot structurally refuses to parse: ruling R4) — not a
// borrowed placeholder. The MEM bank's static Slots is nil BY CONTRACT
// (core/driver/ic905/caps.go: a sparse bank's Slots lists what a READ
// MATERIALISED, and the static baseline has read nothing). Under
// spec.Bank.WithinSpace (app/uispec.go's bankSlotViews, since bf458e1) a
// discovered-style address no read has ever materialised — "G02-050" —
// still lands in MEM, because "is this slot within the space" is
// decidable from the Bank's Sparse/Groups/PerGroup descriptor alone,
// independent of what Slots happens to list — the SAME finding this
// task's SPECIAL CHECK makes about core/codeplug.NormaliseTierFields'
// own bankForSlot, which asks the identical question
// (TestLoadFilePath_TierFieldsNormalisedAgainstTheFileSOwnModel's IC-905
// subtest in app/fileio_test.go).
//
// "C13" is the control. It lies outside CALL's twelve fixed slots (a
// DENSE bank, so WithinSpace there is exactly Slots membership) and
// outside MEM's space alike (spec.ParseSparseSlot refuses any string
// without a leading "G"), so it must stay an ORPHAN: no bank may claim
// it, and no synthesised bank rescues it either (this driver does not
// implement driver.DiscoveredBankSynthesizer).
//
// TagDisplayDefault must be {state: "unavailable"} on both banks of both
// paths: the IC-905's 1A 00 record has no display flag at all
// (FieldTagDisplay carries the zero FieldSupport on both banks,
// core/driver/ic905/caps.go's bankFields).
//
// Fields must equal ic905TierFields on both banks of both paths: NINE of
// the tier's ten fields, not the IC-705's or the IC-9700's own ten — see
// ic905TierFields' own doc comment for why tx_frequency is the one field
// this radio's record does not map.
func TestGetUISpec_RegisteredIC905_EveryBankFieldsAndTagDisplay(t *testing.T) {
	unavailable := codeplug.BoolField{State: codeplug.Unavailable}
	wantBanks := []string{"MEM", "CALL"}

	// THE PREMISE, GUARDED — the IC-9700 exemplar's opening move
	// (TestGetUISpec_RegisteredIC9700_EveryBankFieldsAndTagDisplay), which
	// fails fast rather than comparing against a shape that has silently
	// changed under it. This radio's is the MIRROR IMAGE of the 9700's:
	// MEM is SPARSE and carries NO static Slots by contract
	// (core/driver/ic905/caps.go — a sparse bank's Slots lists what a READ
	// found, not what the radio has), so every MEM expectation below is a
	// statement about the DISCOVERY WALK. A MEM bank that stopped being
	// sparse, or that grew a static Slots list, would make those
	// expectations mean something else entirely while still comparing
	// equal for the wrong reason.
	staticCaps, err := wiring.StaticCapabilities(wiring.IC905Model)
	if err != nil {
		t.Fatalf("wiring.StaticCapabilities(%q): unexpected error: %v", wiring.IC905Model, err)
	}
	staticBankIDs := make([]string, len(staticCaps.Banks))
	for i, b := range staticCaps.Banks {
		staticBankIDs[i] = string(b.ID)
	}
	if !reflect.DeepEqual(staticBankIDs, wantBanks) {
		t.Fatalf("static banks = %v, want exactly %v", staticBankIDs, wantBanks)
	}
	for _, b := range staticCaps.Banks {
		switch string(b.ID) {
		case "MEM":
			if !b.Sparse {
				t.Fatalf("static bank MEM is not Sparse — the IC-905's MEM is a group-addressed SPARSE space by design (core/driver/ic905/caps.go); this test's whole premise no longer holds")
			}
			if len(b.Slots) != 0 {
				t.Fatalf("static bank MEM lists %d slots — a sparse bank's Slots is nil BY CONTRACT, and a MEM expectation below would then be comparing against a static list rather than the discovery walk it is written about", len(b.Slots))
			}
		case "CALL":
			if b.Sparse {
				t.Fatalf("static bank CALL is Sparse — this radio's CALL is a fixed DENSE bank, seeded or not")
			}
			if len(b.Slots) == 0 {
				t.Fatalf("static bank CALL lists no slots — a dense bank's twelve call channels come from the static bank itself, and there is nothing to compare against")
			}
		}
	}

	prevOpts := wiring.IC905FakeSessionOpts
	wiring.IC905FakeSessionOpts = nil // this radio's fake seeds its own default image with no options at all
	t.Cleanup(func() { wiring.IC905FakeSessionOpts = prevOpts })

	sess, closeAll, err := wiring.OpenFakeSessionFor(testAppCtx(t), wiring.IC905Model)
	if err != nil {
		t.Fatalf("wiring.OpenFakeSessionFor(%q): unexpected error: %v", wiring.IC905Model, err)
	}
	t.Cleanup(func() { _ = closeAll() })

	a, _ := newTestApp(t)
	connectDirect(t, a, sess, nil)
	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec (connected to the IC-905 fake): unexpected error: %v", err)
	}
	if !got.Live {
		t.Error("Live = false, want true (connected to the registered fake)")
	}
	if !reflect.DeepEqual(bankIDs(got.Banks), wantBanks) {
		t.Fatalf("connected banks = %v, want exactly %v — this radio discovers no extra bank", bankIDs(got.Banks), wantBanks)
	}
	// Connected, MEM discovers NOTHING: internal/wiring/fake.go's
	// IC905Model row deletes all ten of internal/fakeic905's default
	// image's channels (fakeic905.WithEmpty(0, ch) for ch in 0-9) before
	// the bounded discovery walk runs, precisely because those channels'
	// all-zero invented content is UNDECODABLE by
	// core/civ/ic905/profile.go's own filter — this is the ruled EMPTY
	// demo IC-905, not a stale expectation of the frozen fake's raw
	// default. That MEM's connected classification is otherwise
	// EMPTY-and-decodable (rather than merely absent) is covered at
	// wiring level, not here: internal/wiring's
	// TestOpenFakeSessionFor_EveryRegisteredModel_ReadsEveryDefaultSlot
	// walks every registered model's session Capabilities(), and its
	// IC-905 case is the one that proves this bank's discovered Slots is
	// empty by design rather than by an undecodable read failing
	// silently. Seeding one MEM channel here to assert the opposite case
	// would need a record core/civ/ic905/profile.go's filter accepts, and
	// fakeic905's only seeding option (WithRecord) takes raw bytes with
	// no such helper — inventing one is out of scope for this test.
	wantLiveMem := ic905SlotViews()
	if liveMem := findBank(t, got.Banks, "MEM").Slots; !reflect.DeepEqual(liveMem, wantLiveMem) {
		t.Errorf("connected IC-905 MEM.Slots = %v, want %v (the demo IC-905 opens empty — see this block's comment)", liveMem, wantLiveMem)
	}
	// CALL is DENSE and fixed: its twelve slots come from the static bank
	// itself, seeded or not.
	wantCall := ic905SlotViews(
		"C01", "C02", "C03", "C04", "C05", "C06",
		"C07", "C08", "C09", "C10", "C11", "C12",
	)
	if liveCall := findBank(t, got.Banks, "CALL").Slots; !reflect.DeepEqual(liveCall, wantCall) {
		t.Errorf("connected IC-905 CALL.Slots = %v, want %v (twelve fixed call channels)", liveCall, wantCall)
	}
	for _, b := range got.Banks {
		if b.TagDisplayDefault != unavailable {
			t.Errorf("connected IC-905 bank %s TagDisplayDefault = %+v, want %+v — this radio's memory frame has no display flag", b.ID, b.TagDisplayDefault, unavailable)
		}
		if !reflect.DeepEqual(b.Fields, ic905TierFields) {
			t.Errorf("connected IC-905 bank %s Fields = %v, want %v", b.ID, b.Fields, ic905TierFields)
		}
	}

	// Offline, from an IC-905 file: the same answers, from the static
	// RealHardware baseline this time — and the working copy's own slots,
	// classified by each bank's SPACE.
	a.mu.Lock()
	a.conn = nil
	a.working = &codeplug.Codeplug{
		Schema: codeplug.CurrentSchema,
		Radio:  codeplug.RadioInfo{Model: wiring.IC905Model},
		Channels: []codeplug.Channel{
			{Slot: "G01-001"}, // MEM, one the fake's walk did materialise
			{Slot: "C01"},     // CALL, a listed slot of a dense bank
			{Slot: "G02-050"}, // MEM by SPACE alone: in no Slots list, anywhere
			{Slot: "C13"},     // in neither bank's space: an orphan, by design
		},
	}
	a.mu.Unlock()
	offline, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec (offline, IC-905 working copy): unexpected error: %v", err)
	}
	if offline.Live {
		t.Error("Live = true, want false (disconnected)")
	}
	if !reflect.DeepEqual(bankIDs(offline.Banks), wantBanks) {
		t.Fatalf("offline banks = %v, want exactly %v — this driver synthesises no discovered bank", bankIDs(offline.Banks), wantBanks)
	}
	wantOfflineMem := ic905SlotViews("G01-001", "G02-050")
	if offMem := findBank(t, offline.Banks, "MEM").Slots; !reflect.DeepEqual(offMem, wantOfflineMem) {
		t.Errorf("offline IC-905 MEM.Slots = %v, want %v — a SPARSE bank's working-copy slots are classified by its declared space (spec.Bank.WithinSpace), not by the Slots list it deliberately does not carry", offMem, wantOfflineMem)
	}
	wantOfflineCall := ic905SlotViews("C01")
	if offCall := findBank(t, offline.Banks, "CALL").Slots; !reflect.DeepEqual(offCall, wantOfflineCall) {
		t.Errorf("offline IC-905 CALL.Slots = %v, want %v — CALL is dense, so only its twelve listed slots may appear", offCall, wantOfflineCall)
	}
	for _, b := range offline.Banks {
		for _, s := range b.Slots {
			if s.Slot == "C13" {
				t.Errorf("offline IC-905 bank %s claims slot C13, which lies outside CALL's twelve fixed channels and outside MEM's group-addressed space alike (it does not begin with \"G\") — it must stay an orphan", b.ID)
			}
		}
		if b.TagDisplayDefault != unavailable {
			t.Errorf("offline IC-905 bank %s TagDisplayDefault = %+v, want %+v", b.ID, b.TagDisplayDefault, unavailable)
		}
		if !reflect.DeepEqual(b.Fields, ic905TierFields) {
			t.Errorf("offline IC-905 bank %s Fields = %v, want %v", b.ID, b.Fields, ic905TierFields)
		}
	}

	// The contrast: the FT-710, through the same offline path, still
	// answers Known-false and an empty Fields on every bank.
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
		if len(b.Fields) != 0 {
			t.Errorf("offline FT-710 bank %s Fields = %v, want empty — the FT-710 maps none of the Icom tier's fields", b.ID, b.Fields)
		}
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
		// The fifth state, and the answer must be the plain-Unverified row's:
		// Unavailable is triggered by "absent from the frame in BOTH
		// directions", and a consented write label is not Unsupported, so a
		// blank row on a consented session still states the flag.
		{"ConsentedUnverified write -> Known-false (consent does not make a flag appear or vanish)", map[spec.Field]spec.FieldSupport{
			spec.FieldTagDisplay: {Read: spec.Unverified, Write: spec.ConsentedUnverified},
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

func TestGetUISpec_TransmitFollowsRadioCapabilities(t *testing.T) {
	for _, tc := range []struct {
		name     string
		transmit spec.Transmit
		want     string
	}{
		{"transceiver", spec.HasTransmitter, "has_transmitter"},
		{"receiver", spec.ReceiveOnly, "receive_only"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recogniseModelCaps(t, spec.Capabilities{
				Model: testModel, CATID: "9999", Transmit: tc.transmit,
			})
			a, _ := newTestApp(t)
			a.mu.Lock()
			a.working = &codeplug.Codeplug{Schema: codeplug.CurrentSchema, Radio: codeplug.RadioInfo{Model: testModel}}
			a.mu.Unlock()

			got, err := a.GetUISpec()
			if err != nil {
				t.Fatalf("GetUISpec: %v", err)
			}
			if got.Transmit != tc.want {
				t.Errorf("Transmit = %q, want %q", got.Transmit, tc.want)
			}
		})
	}
}

func TestGetUISpec_BudgetUnstatedFollowsBankCapabilities(t *testing.T) {
	recogniseModelCaps(t, spec.Capabilities{
		Model: testModel, CATID: "9999", Transmit: spec.ReceiveOnly,
		Banks: []spec.Bank{{
			ID: spec.BankMemory, Sparse: true, Groups: 1, PerGroup: 1,
			BudgetUnstated: true,
		}},
	})
	a, _ := newTestApp(t)
	a.mu.Lock()
	a.working = &codeplug.Codeplug{Schema: codeplug.CurrentSchema, Radio: codeplug.RadioInfo{Model: testModel}}
	a.mu.Unlock()

	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec: %v", err)
	}
	if len(got.Banks) != 1 || !got.Banks[0].BudgetUnstated {
		t.Errorf("Banks = %+v, want one bank with BudgetUnstated", got.Banks)
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

// TestGetUISpec_RegisteredFTdx101D_EveryBankUnavailable and its MP sibling
// are the FTdx10 D5c test's counterparts for the two models M9d-2
// registered: GetUISpec driven for each model through REAL registration,
// twice, because the two paths reach bankTagDisplayDefault with different
// capability values and the grid must get the same answer from both.
//
//   - CONNECTED to that model's registered fake (Live true, the Simulated
//     profile plus discovery's own inventory — a populated 5 MHz bank here,
//     so "every bank" spans a discovered one too). This is the
//     `--fake --model FTdx101D` path a user actually walks.
//   - DISCONNECTED with that model's working copy loaded (Live false, the
//     static RealHardware baseline, resolved by currentModel from the file's
//     own Radio.Model). This is the offline clone workflow's path.
//
// Every bank of both must serve {state: "unavailable"} (matrix §3.7): the
// FTDX101's combined MT record has NO display flag — its P11 is "0: (Fixed)"
// at layout 1329, and the independent 41-position geometry witness leaves
// nowhere to put one — so a blank row added anywhere in that grid must not
// carry a Known one. That is a MANUAL-EVIDENCED absence, not an assumption,
// which is why it is safe to assert as a hard verdict rather than a hedge.
//
// The FT-710 contrast at the end of each is what stops the whole thing
// passing because something returned a zero value: two registered radios,
// two different answers, no seam and no fixture in either.
func TestGetUISpec_RegisteredFTdx101D_EveryBankUnavailable(t *testing.T) {
	assertFTdx101EveryBankUnavailable(t, "FTdx101D")
}

// TestGetUISpec_RegisteredFTdx101MP_EveryBankUnavailable: see the D's doc
// comment. A separate test over a separate session, because the MP's
// registration is a separate fact and a shared session would let a
// half-crossed fakeDrivers pair pass.
func TestGetUISpec_RegisteredFTdx101MP_EveryBankUnavailable(t *testing.T) {
	assertFTdx101EveryBankUnavailable(t, "FTdx101MP")
}

// assertFTdx101EveryBankUnavailable is the shared body of the two tests
// above, run wholly within one model's own registration.
func assertFTdx101EveryBankUnavailable(t *testing.T, model string) {
	t.Helper()
	unavailable := codeplug.BoolField{State: codeplug.Unavailable}

	restore := setFTdx101FakeOpts(t, model, []fakedx101.Option{fakedx101.With5xx()})
	defer restore()
	sess, closeAll, err := wiring.OpenFakeSessionFor(testAppCtx(t), model)
	if err != nil {
		t.Fatalf("wiring.OpenFakeSessionFor(%q): unexpected error: %v", model, err)
	}
	t.Cleanup(func() { _ = closeAll() })

	a, _ := newTestApp(t)
	connectDirect(t, a, sess, nil)
	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec (connected to the %s fake): unexpected error: %v", model, err)
	}
	if !got.Live {
		t.Error("Live = false, want true (connected to the registered fake)")
	}
	if len(got.Banks) < 3 {
		t.Fatalf("banks = %v, want MEM, PMS and the discovered 60M — 'every bank' must span a discovered one", bankIDs(got.Banks))
	}
	for _, b := range got.Banks {
		if b.TagDisplayDefault != unavailable {
			t.Errorf("connected %s bank %s TagDisplayDefault = %+v, want %+v — this radio's memory frame has no display flag (matrix §3.7)", model, b.ID, b.TagDisplayDefault, unavailable)
		}
	}

	// Offline, from a working copy naming this model: the same answer, from
	// the static RealHardware baseline this time.
	a.mu.Lock()
	a.conn = nil
	a.working = &codeplug.Codeplug{
		Schema:   codeplug.CurrentSchema,
		Radio:    codeplug.RadioInfo{Model: model},
		Channels: []codeplug.Channel{{Slot: "001"}},
	}
	a.mu.Unlock()
	offline, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec (offline, %s working copy): unexpected error: %v", model, err)
	}
	if offline.Live {
		t.Error("Live = true, want false (disconnected)")
	}
	if len(offline.Banks) == 0 {
		t.Fatalf("offline %s UISpec has no banks — nothing asserted", model)
	}
	for _, b := range offline.Banks {
		if b.TagDisplayDefault != unavailable {
			t.Errorf("offline %s bank %s TagDisplayDefault = %+v, want %+v", model, b.ID, b.TagDisplayDefault, unavailable)
		}
	}

	// The contrast: the FT-710, through the same offline path, still answers
	// Known-false on every bank.
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

// TestGetUISpec_SlotClassification_DenseBanksUnchangedByWithinSpace is fix
// round 1's regression pin for F1. bankSlotViews now classifies an offline
// working copy's slots with spec.Bank.WithinSpace instead of literal
// membership of the static bank's Slots, and on a DENSE bank those are the
// same question by construction: WithinSpace scans Slots and then answers
// false unless the bank is Sparse (core/spec/bank.go). Every registered
// Yaesu model is dense on every static bank, so nothing about their grids
// may move by so much as a slot.
//
// The test does not restate the expected lists — it RECOMPUTES them with
// the OLD rule (literal membership of each static bank's own Slots, in
// working-copy order) and demands byte-identical output. Any future change
// to the classification rule that moves a dense model's answer fails here,
// whatever the new rule happens to be.
//
// The working copy holds every slot each static bank lists, plus two the
// static banks do not: "501", a 60m slot these drivers' own
// DiscoveredBankSynthesizer rescues into a read-only synthesised bank, and
// "ZZZ", which nothing claims — GetUISpec's orphan case, and the pin that
// the widened rule admits nothing it should not.
func TestGetUISpec_SlotClassification_DenseBanksUnchangedByWithinSpace(t *testing.T) {
	for _, model := range []string{wiring.DefaultModel, "FTdx10", "FTdx101D", "FTdx101MP"} {
		t.Run(model, func(t *testing.T) {
			caps, err := wiring.StaticCapabilities(model)
			if err != nil {
				t.Fatalf("wiring.StaticCapabilities(%q): unexpected error: %v", model, err)
			}
			if len(caps.Banks) == 0 {
				t.Fatalf("%s: static baseline has no banks — nothing asserted", model)
			}
			var channels []codeplug.Channel
			for _, b := range caps.Banks {
				if b.Sparse {
					t.Fatalf("%s bank %s is Sparse — this test is the DENSE pin and its premise no longer holds; give the model its own assertion rather than deleting this failure", model, b.ID)
				}
				if len(b.Slots) == 0 {
					t.Fatalf("%s bank %s lists no slots — the comparison would be vacuous", model, b.ID)
				}
				for _, s := range b.Slots {
					channels = append(channels, codeplug.Channel{Slot: s})
				}
			}
			channels = append(channels,
				codeplug.Channel{Slot: "501"},
				codeplug.Channel{Slot: "ZZZ"},
			)

			a, _ := newTestApp(t)
			a.mu.Lock()
			a.working = &codeplug.Codeplug{
				Schema:   codeplug.CurrentSchema,
				Radio:    codeplug.RadioInfo{Model: model},
				Channels: channels,
			}
			a.mu.Unlock()
			got, err := a.GetUISpec()
			if err != nil {
				t.Fatalf("GetUISpec (offline, %s working copy): unexpected error: %v", model, err)
			}
			if got.Live {
				t.Fatal("Live = true while disconnected, want false")
			}

			for _, b := range caps.Banks {
				member := make(map[string]bool, len(b.Slots))
				for _, s := range b.Slots {
					member[s] = true
				}
				var want []SlotView
				for _, ch := range channels {
					if member[ch.Slot] {
						want = append(want, SlotView{Slot: ch.Slot, Display: codeplug.DisplaySlot(ch.Slot)})
					}
				}
				view := findBank(t, got.Banks, string(b.ID))
				if !reflect.DeepEqual(view.Slots, want) {
					t.Errorf("%s bank %s Slots = %v, want %v — the WithinSpace rule must be byte-identical to literal Slots membership on a dense bank", model, b.ID, view.Slots, want)
				}
			}
			for _, b := range got.Banks {
				for _, s := range b.Slots {
					if s.Slot == "ZZZ" {
						t.Errorf("%s bank %s claims the orphan slot ZZZ, which no bank's space contains", model, b.ID)
					}
				}
			}
		})
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
		if got.GridLegendNote != want.GridLegendNote {
			t.Errorf("GridLegendNote = %q, want %q", got.GridLegendNote, want.GridLegendNote)
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
	if !ok || ft710Text.GridLegendNote == "" || ft710Text.EraseDialogNote == "" {
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
		{"GridLegendNote", got.GridLegendNote},
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

// TestGetUISpec_UnverifiedWritesConsentedFollowsTheSession pins the
// amber-state flag's derivation: it is read from the CONNECTED session's
// own capability labels — a write-side spec.ConsentedUnverified anywhere —
// and from nothing else. Not from the settings file, which a concurrent
// CLI can change under a running session (userconfig's documented
// last-writer-wins), and not from the model, which says only that consent
// is possible. The interface therefore cannot show an armed state that the
// session in front of the user does not actually have.
//
// The consented capability set is built by core/spec's OWN transform, the
// same one a driver applies at session assembly (app/ may not import a
// concrete driver package — the M9a neutral-core discipline).
func TestGetUISpec_UnverifiedWritesConsentedFollowsTheSession(t *testing.T) {
	caps, err := capsForModel(wiring.FTdx10Model)
	if err != nil {
		t.Fatalf("capsForModel(%q): unexpected error: %v", wiring.FTdx10Model, err)
	}
	consented := spec.ConsentUnverifiedWrites(caps)

	t.Run("consented session", func(t *testing.T) {
		a, _ := newTestApp(t)
		connectDirect(t, a, fixedCapsSession{caps: consented}, nil)
		got, err := a.GetUISpec()
		if err != nil {
			t.Fatalf("GetUISpec: unexpected error: %v", err)
		}
		if !got.UnverifiedWritesConsented {
			t.Error("UnverifiedWritesConsented = false, want true — the session's own caps carry a consented write")
		}
	})

	t.Run("unconsented session", func(t *testing.T) {
		a, _ := newTestApp(t)
		connectDirect(t, a, fixedCapsSession{caps: caps}, nil)
		got, err := a.GetUISpec()
		if err != nil {
			t.Fatalf("GetUISpec: unexpected error: %v", err)
		}
		if got.UnverifiedWritesConsented {
			t.Error("UnverifiedWritesConsented = true, want false — the same radio, with no consent spent on the session")
		}
	})

	t.Run("offline", func(t *testing.T) {
		a, _ := newTestApp(t)
		a.mu.Lock()
		a.conn = nil
		a.working = &codeplug.Codeplug{
			Schema:   codeplug.CurrentSchema,
			Radio:    codeplug.RadioInfo{Model: wiring.FTdx10Model},
			Channels: []codeplug.Channel{{Slot: "001"}},
		}
		a.mu.Unlock()
		got, err := a.GetUISpec()
		if err != nil {
			t.Fatalf("GetUISpec: unexpected error: %v", err)
		}
		if got.UnverifiedWritesConsented {
			t.Error("UnverifiedWritesConsented = true offline, want false — a static baseline describes the radio, never a session's consent")
		}
	})

	t.Run("demo", func(t *testing.T) {
		a, _ := newTestApp(t)
		if _, err := a.ConnectDemo(wiring.FTdx10Model); err != nil {
			t.Fatalf("ConnectDemo(%q): unexpected error: %v", wiring.FTdx10Model, err)
		}
		t.Cleanup(func() { _ = a.Disconnect() })
		got, err := a.GetUISpec()
		if err != nil {
			t.Fatalf("GetUISpec: unexpected error: %v", err)
		}
		if got.UnverifiedWritesConsented {
			t.Error("UnverifiedWritesConsented = true in demo, want false — a simulator session spends no consent")
		}
	})
}

// ic7610MEMBankJSON is the IC-7610's MEM BankView, EXACTLY as GetUISpec
// serves it to the frontend, for an offline working copy holding the
// single slot "001".
//
// It is the source of the `ic7610MemBank` fixture in
// app/frontend/src/lib/grid/__tests__/tierColumns.test.js, which is a
// verbatim copy of this literal (JSON being a subset of the JS object
// syntax it is written in). That test used to build a HYPOTHETICAL Icom
// bank instead — "no such driver exists", as its own comment said — and
// the IC-7610's registration (Wave 4 task R1) ended the need for one: the
// grid's conditional-column rules are now pinned against a bank a user
// can really be served.
//
// Slots is the only field trimmed by that copy, and it is trimmed by the
// working copy this test builds rather than by hand: a real IC-7610 MEM
// bank lists "001".."099" (core/driver/ic7610/caps.go's memSlots), and
// what the grid fixture needs is one row, not an inventory. Every other
// value here is the radio's own answer.
const ic7610MEMBankJSON = `{"ID":"MEM","Label":"Memories","ReadOnly":false,` +
	`"Slots":[{"Slot":"001","Display":"M-01"}],` +
	`"TagDisplayDefault":{"state":"unavailable"},` +
	`"Fields":["tone_mode","tone_tx","tone_rx","filter"]}`

// TestGetUISpec_IC7610MEMBank_IsTheJSGridFixture pins the JSON the
// frontend actually receives for the IC-7610's MEM bank, so the JS
// fixture copied from it cannot drift away from the Go side unnoticed:
// change this radio's capabilities, or bankTierFields, or BankView's
// shape, and this test fails naming the difference — at which point the
// JS fixture named in ic7610MEMBankJSON's comment must be updated with
// the new literal.
//
// It asserts the SERIALISED form deliberately, rather than the Go struct:
// what the frontend consumes is the JSON, key names and all, and a
// renamed field would leave a struct-level assertion passing while the
// grid silently stopped finding the value.
func TestGetUISpec_IC7610MEMBank_IsTheJSGridFixture(t *testing.T) {
	a, _ := newTestApp(t)
	a.mu.Lock()
	a.working = &codeplug.Codeplug{
		Schema:   codeplug.CurrentSchema,
		Radio:    codeplug.RadioInfo{Model: wiring.IC7610Model},
		Channels: []codeplug.Channel{{Slot: "001"}},
	}
	a.mu.Unlock()

	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec (offline, IC-7610 working copy): unexpected error: %v", err)
	}
	mem := findBank(t, got.Banks, "MEM")
	raw, err := json.Marshal(mem)
	if err != nil {
		t.Fatalf("marshalling the MEM BankView: %v", err)
	}
	if string(raw) != ic7610MEMBankJSON {
		t.Errorf("the IC-7610's MEM BankView is now\n  %s\nbut the JS grid fixture was copied from\n  %s\n(update app/frontend/src/lib/grid/__tests__/tierColumns.test.js's ic7610MemBank with the new value)", raw, ic7610MEMBankJSON)
	}
	t.Logf("IC-7610 MEM BankView as the frontend receives it: %s", raw)
}

// TestBankReadOnly_RegisteredIC7851Pair_RealHardwareProfile pins what a
// REAL IC-7851's and a REAL IC-7850's grids do today, bank by bank,
// through real registration — the additions tier's mirror of
// TestBankReadOnly_RegisteredIC7610_RealHardwareProfile.
//
// Each row's RealHardware profile is its all-Unverified one
// (writeTrialsComplete7851 and writeTrialsComplete7850 are BOTH false: no
// radio of either model has ever been written to by this project), so the
// three derived core fields are Write spec.Unverified on both banks —
// which is NOT spec.Unsupported, and therefore NOT read-only under
// bankReadOnly's standing rule, on the same footing as every other row.
//
// BOTH ROWS ARE DRIVEN, and their verdicts are then compared with each
// other: the two share one implementation and one capability shape
// (additions spec D1.2), so a DIFFERENCE between them would mean a model
// dimension had appeared in a capability table that supports none — the
// same assertion TestBankReadOnly_FTdx101SiblingsAgree makes for the
// Yaesu pair.
//
// NO DISCOVERED-BANK CONTRAST, as for the IC-7610 and for the same
// reason: this radio has no discovery mechanism at all. Its Banks are
// fixed at construction — spec.BankMemory and spec.BankScan,
// core/driver/ic7851/caps.go's baseCapabilities — so the static baseline
// is the whole of what its bankReadOnly verdict can be.
func TestBankReadOnly_RegisteredIC7851Pair_RealHardwareProfile(t *testing.T) {
	verdicts := map[string]map[spec.BankID]bool{}
	for _, model := range []string{wiring.IC7851Model, wiring.IC7850Model} {
		t.Run(model, func(t *testing.T) {
			caps, err := wiring.StaticCapabilities(model)
			if err != nil {
				t.Fatalf("wiring.StaticCapabilities(%q): unexpected error: %v", model, err)
			}
			if len(caps.Banks) == 0 {
				t.Fatalf("the registered %s's static baseline has no banks — nothing asserted", model)
			}
			got := map[spec.BankID]bool{}
			for _, b := range caps.Banks {
				fields := bankCoreFields(caps, b.ID)
				wantFields(t, model+" bank "+string(b.ID), fields, ic7851CoreThree)
				for _, f := range fields {
					if w := caps.FieldSupport(b.ID, f).Write; w != spec.Unverified {
						t.Errorf("bank %s field %s Write = %v, want Unverified (the premise: nothing on a real %s is proven writable)", b.ID, f, w, model)
					}
				}
				if bankReadOnly(caps, b.ID) {
					t.Errorf("bankReadOnly(%s) = true, want false — Unverified is not Unsupported, and locking it would break the offline clone workflow", b.ID)
				}
				got[b.ID] = bankReadOnly(caps, b.ID)
			}
			verdicts[model] = got
		})
	}
	if len(verdicts) == 2 && !reflect.DeepEqual(verdicts[wiring.IC7851Model], verdicts[wiring.IC7850Model]) {
		t.Errorf("bankReadOnly verdicts differ between the two rows:\n  IC-7851 = %v\n  IC-7850 = %v\nthe two share one driver, one profile and one capability shape (additions spec D1.2), so a difference means a registration or a capability set has acquired a model dimension nothing supports", verdicts[wiring.IC7851Model], verdicts[wiring.IC7850Model])
	}
}

// TestGetUISpec_RegisteredIC7851Pair_EveryBankFieldsAndTagDisplay is the
// additions tier's mirror of
// TestGetUISpec_RegisteredIC7610_EveryBankFieldsAndTagDisplay: GetUISpec
// driven for BOTH new rows through real registration, connected and
// offline.
//
//   - CONNECTED to the registered fake (Live true, the Simulated profile
//     — the `--fake --model IC-7851` path a user actually walks). This
//     radio discovers no extra bank, so "every bank" here is just MEM and
//     SCAN.
//   - DISCONNECTED with that row's own working copy loaded (Live false,
//     the static RealHardware baseline, resolved by currentModel from the
//     file's own Radio.Model) — the offline clone workflow's path.
//
// TagDisplayDefault must be {state: "unavailable"} on both banks of both
// paths: this radio's 1A 00 record has no display flag at all
// (FieldTagDisplay carries the zero FieldSupport on both banks,
// core/driver/ic7851/caps.go's bankFields), so a blank row added anywhere
// must not carry a Known one.
//
// Fields must equal ic7851TierFields on both banks of both paths, for
// both rows: the record maps tone_mode, tone_tx, tone_rx and filter and
// none of the other tier fields, and bankTierFields must derive that same
// four-field list from whichever capability source GetUISpec used.
//
// THE CONNECTED LEG IS WHERE THE TWO ROWS COULD DIVERGE and where the
// registration is actually exercised: each row opens its OWN fakeDrivers
// entry, so a row wired to the sibling's driver or the sibling's option
// source shows up here as a wrong Capabilities().Model.
func TestGetUISpec_RegisteredIC7851Pair_EveryBankFieldsAndTagDisplay(t *testing.T) {
	unavailable := codeplug.BoolField{State: codeplug.Unavailable}

	for _, model := range []string{wiring.IC7851Model, wiring.IC7850Model} {
		t.Run(model, func(t *testing.T) {
			sess, closeAll, err := wiring.OpenFakeSessionFor(testAppCtx(t), model)
			if err != nil {
				t.Fatalf("wiring.OpenFakeSessionFor(%q): unexpected error: %v", model, err)
			}
			t.Cleanup(func() { _ = closeAll() })
			if got := sess.Capabilities().Model; got != model {
				t.Fatalf("the fake session's Capabilities().Model = %q, want %q — this row is wired to the wrong driver", got, model)
			}

			a, _ := newTestApp(t)
			connectDirect(t, a, sess, nil)
			got, err := a.GetUISpec()
			if err != nil {
				t.Fatalf("GetUISpec (connected to the %s fake): unexpected error: %v", model, err)
			}
			if !got.Live {
				t.Error("Live = false, want true (connected to the registered fake)")
			}
			if len(got.Banks) != 2 {
				t.Fatalf("banks = %v, want exactly MEM and SCAN — this radio discovers no extra bank", bankIDs(got.Banks))
			}
			for _, b := range got.Banks {
				if b.TagDisplayDefault != unavailable {
					t.Errorf("connected %s bank %s TagDisplayDefault = %+v, want %+v — this radio's memory frame has no display flag", model, b.ID, b.TagDisplayDefault, unavailable)
				}
				if !reflect.DeepEqual(b.Fields, ic7851TierFields) {
					t.Errorf("connected %s bank %s Fields = %v, want %v", model, b.ID, b.Fields, ic7851TierFields)
				}
			}

			// Offline, from this row's own file: the same answers, from
			// the static RealHardware baseline this time.
			a.mu.Lock()
			a.conn = nil
			a.working = &codeplug.Codeplug{
				Schema:   codeplug.CurrentSchema,
				Radio:    codeplug.RadioInfo{Model: model},
				Channels: []codeplug.Channel{{Slot: "001"}},
			}
			a.mu.Unlock()
			offline, err := a.GetUISpec()
			if err != nil {
				t.Fatalf("GetUISpec (offline, %s working copy): unexpected error: %v", model, err)
			}
			if offline.Live {
				t.Error("Live = true, want false (disconnected)")
			}
			if len(offline.Banks) == 0 {
				t.Fatalf("offline %s UISpec has no banks — nothing asserted", model)
			}
			for _, b := range offline.Banks {
				if b.TagDisplayDefault != unavailable {
					t.Errorf("offline %s bank %s TagDisplayDefault = %+v, want %+v", model, b.ID, b.TagDisplayDefault, unavailable)
				}
				if !reflect.DeepEqual(b.Fields, ic7851TierFields) {
					t.Errorf("offline %s bank %s Fields = %v, want %v", model, b.ID, b.Fields, ic7851TierFields)
				}
			}
		})
	}
}

// TestBankReadOnly_RegisteredIC7760_RealHardwareProfile pins what a REAL
// IC-7760's grid does today, bank by bank, through real registration —
// the additions tier's second mirror of
// TestBankReadOnly_RegisteredIC7610_RealHardwareProfile.
//
// Its RealHardware profile is its all-Unverified one (writeTrialsComplete
// is false: no IC-7760 has ever been written to by this project), so the
// three derived core fields are Write spec.Unverified on both banks —
// which is NOT spec.Unsupported, and therefore NOT read-only under
// bankReadOnly's standing rule, on the same footing as every other row.
//
// NO DISCOVERED-BANK CONTRAST, as for the IC-7610 and the IC-7851 pair
// and for the same reason: this radio has no discovery mechanism at all.
// Its Banks are fixed at construction — spec.BankMemory and spec.BankScan,
// core/driver/ic7760/caps.go's baseCapabilities — so the static baseline
// is the whole of what its bankReadOnly verdict can be.
func TestBankReadOnly_RegisteredIC7760_RealHardwareProfile(t *testing.T) {
	caps, err := wiring.StaticCapabilities(wiring.IC7760Model)
	if err != nil {
		t.Fatalf("wiring.StaticCapabilities(%q): unexpected error: %v", wiring.IC7760Model, err)
	}
	if len(caps.Banks) == 0 {
		t.Fatalf("the registered %s's static baseline has no banks — nothing asserted", wiring.IC7760Model)
	}
	for _, b := range caps.Banks {
		fields := bankCoreFields(caps, b.ID)
		wantFields(t, "IC-7760 bank "+string(b.ID), fields, ic7760CoreThree)
		for _, f := range fields {
			if w := caps.FieldSupport(b.ID, f).Write; w != spec.Unverified {
				t.Errorf("bank %s field %s Write = %v, want Unverified (the premise: nothing on a real IC-7760 is proven writable)", b.ID, f, w)
			}
		}
		if bankReadOnly(caps, b.ID) {
			t.Errorf("bankReadOnly(%s) = true, want false — Unverified is not Unsupported, and locking it would break the offline clone workflow", b.ID)
		}
	}
}

// TestGetUISpec_RegisteredIC7760_EveryBankFieldsAndTagDisplay is the
// additions tier's second mirror of
// TestGetUISpec_RegisteredIC7610_EveryBankFieldsAndTagDisplay: GetUISpec
// driven for the new row through real registration, connected and
// offline.
//
//   - CONNECTED to the registered fake (Live true, the Simulated profile
//     — the `--fake --model IC-7760` path a user actually walks). This
//     radio discovers no extra bank, so "every bank" here is just MEM and
//     SCAN.
//   - DISCONNECTED with its own working copy loaded (Live false, the
//     static RealHardware baseline, resolved by currentModel from the
//     file's own Radio.Model) — the offline clone workflow's path.
//
// TagDisplayDefault must be {state: "unavailable"} on both banks of both
// paths: this radio's 1A 00 record has no display flag at all
// (FieldTagDisplay carries the zero FieldSupport on both banks,
// core/driver/ic7760/caps.go's bankFields), so a blank row added anywhere
// must not carry a Known one.
//
// Fields must equal ic7760TierFields on both banks of both paths: the
// record maps tone_mode, tone_tx, tone_rx and filter and none of the
// other tier fields, and bankTierFields must derive that same four-field
// list from whichever capability source GetUISpec used.
//
// THE CONNECTED LEG IS WHERE THE REGISTRATION IS ACTUALLY EXERCISED: a
// row wired to another model's driver or another model's option source
// shows up here as a wrong Capabilities().Model, which is asserted before
// anything else.
func TestGetUISpec_RegisteredIC7760_EveryBankFieldsAndTagDisplay(t *testing.T) {
	unavailable := codeplug.BoolField{State: codeplug.Unavailable}

	sess, closeAll, err := wiring.OpenFakeSessionFor(testAppCtx(t), wiring.IC7760Model)
	if err != nil {
		t.Fatalf("wiring.OpenFakeSessionFor(%q): unexpected error: %v", wiring.IC7760Model, err)
	}
	t.Cleanup(func() { _ = closeAll() })
	if got := sess.Capabilities().Model; got != wiring.IC7760Model {
		t.Fatalf("the fake session's Capabilities().Model = %q, want %q — this row is wired to the wrong driver", got, wiring.IC7760Model)
	}

	a, _ := newTestApp(t)
	connectDirect(t, a, sess, nil)
	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec (connected to the IC-7760 fake): unexpected error: %v", err)
	}
	if !got.Live {
		t.Error("Live = false, want true (connected to the registered fake)")
	}
	if len(got.Banks) != 2 {
		t.Fatalf("banks = %v, want exactly MEM and SCAN — this radio discovers no extra bank", bankIDs(got.Banks))
	}
	for _, b := range got.Banks {
		if b.TagDisplayDefault != unavailable {
			t.Errorf("connected bank %s TagDisplayDefault = %+v, want %+v — this radio's memory frame has no display flag", b.ID, b.TagDisplayDefault, unavailable)
		}
		if !reflect.DeepEqual(b.Fields, ic7760TierFields) {
			t.Errorf("connected bank %s Fields = %v, want %v", b.ID, b.Fields, ic7760TierFields)
		}
	}

	// Offline, from this row's own file: the same answers, from the
	// static RealHardware baseline this time.
	a.mu.Lock()
	a.conn = nil
	a.working = &codeplug.Codeplug{
		Schema:   codeplug.CurrentSchema,
		Radio:    codeplug.RadioInfo{Model: wiring.IC7760Model},
		Channels: []codeplug.Channel{{Slot: "001"}},
	}
	a.mu.Unlock()
	offline, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec (offline, IC-7760 working copy): unexpected error: %v", err)
	}
	if offline.Live {
		t.Error("Live = true, want false (disconnected)")
	}
	if len(offline.Banks) == 0 {
		t.Fatal("offline IC-7760 UISpec has no banks — nothing asserted")
	}
	for _, b := range offline.Banks {
		if b.TagDisplayDefault != unavailable {
			t.Errorf("offline bank %s TagDisplayDefault = %+v, want %+v", b.ID, b.TagDisplayDefault, unavailable)
		}
		if !reflect.DeepEqual(b.Fields, ic7760TierFields) {
			t.Errorf("offline bank %s Fields = %v, want %v", b.ID, b.Fields, ic7760TierFields)
		}
	}
}

// TestBankReadOnly_RegisteredIC7100_RealHardwareProfile pins what a REAL
// IC-7100's grid does through real registration — the additions tier's
// third mirror of TestBankReadOnly_RegisteredIC7610_RealHardwareProfile.
//
// Its RealHardware profile is its all-Unverified one (writeTrialsComplete
// is false: no IC-7100 has ever been written to by this project), so the
// three derived core fields are Write spec.Unverified — which is NOT
// spec.Unsupported, and therefore NOT read-only under bankReadOnly's
// standing rule, on the same footing as every other row.
//
// ONE BANK, and no discovered-bank contrast: this radio has no discovery
// mechanism, and the single spec.BankMemory it declares is fixed at
// construction (core/driver/ic7100/caps.go's capabilities), so the static
// baseline is the whole of what its bankReadOnly verdict can be. The
// scan-edge and call channels are not a second bank here: they are
// refused entirely until register entry ic7100-special-bank-byte is
// lifted, and a bank this build cannot address must not appear in a grid.
func TestBankReadOnly_RegisteredIC7100_RealHardwareProfile(t *testing.T) {
	caps, err := wiring.StaticCapabilities(wiring.IC7100Model)
	if err != nil {
		t.Fatalf("wiring.StaticCapabilities(%q): unexpected error: %v", wiring.IC7100Model, err)
	}
	if len(caps.Banks) != 1 {
		t.Fatalf("the registered %s's static baseline has %d banks, want exactly 1 (dense MEM) — the ten special channels are refused, not a second bank", wiring.IC7100Model, len(caps.Banks))
	}
	for _, b := range caps.Banks {
		fields := bankCoreFields(caps, b.ID)
		wantFields(t, "IC-7100 bank "+string(b.ID), fields, ic7100CoreThree)
		for _, f := range fields {
			if w := caps.FieldSupport(b.ID, f).Write; w != spec.Unverified {
				t.Errorf("bank %s field %s Write = %v, want Unverified (the premise: nothing on a real IC-7100 is proven writable)", b.ID, f, w)
			}
		}
		if bankReadOnly(caps, b.ID) {
			t.Errorf("bankReadOnly(%s) = true, want false — Unverified is not Unsupported, and locking it would break the offline clone workflow", b.ID)
		}
	}
}

// TestGetUISpec_RegisteredIC7100_EveryBankFieldsAndTagDisplay is the
// additions tier's third mirror of
// TestGetUISpec_RegisteredIC7610_EveryBankFieldsAndTagDisplay: GetUISpec
// driven for the new row through real registration, connected and
// offline.
//
//   - CONNECTED to the registered fake (Live true, the Simulated profile
//     — the `--fake --model IC-7100` path a user actually walks). This
//     radio discovers no extra bank, so "every bank" here is the one dense
//     MEM space.
//   - DISCONNECTED with its own working copy loaded (Live false, the
//     static RealHardware baseline, resolved by currentModel from the
//     file's own Radio.Model) — the offline clone workflow's path.
//
// TagDisplayDefault must be {state: "unavailable"} on both paths: this
// radio's 1A 00 record has no display flag at all (FieldTagDisplay carries
// the zero FieldSupport, core/driver/ic7100/caps.go's fieldGrid), so a
// blank row added anywhere must not carry a Known one.
//
// Fields must equal ic7100TierFields on both paths: the record maps all
// ten tier fields, data_mode included, and bankTierFields must derive that
// same ten-field list from whichever capability source GetUISpec used.
//
// THE OFFLINE LEG USES A BANK-FORM SLOT, "A-001", and not the bare "001"
// every other Icom row's mirror uses: this is the first registered model
// whose slot strings carry a bank letter (civ.AddressFormBankChannel,
// core/civ/ic7100/profile.go), and a working copy holding a slot its own
// driver would refuse would be testing the wrong thing.
//
// THE CONNECTED LEG IS WHERE THE REGISTRATION IS ACTUALLY EXERCISED: a row
// wired to another model's driver or another model's option source shows
// up here as a wrong Capabilities().Model, which is asserted before
// anything else.
func TestGetUISpec_RegisteredIC7100_EveryBankFieldsAndTagDisplay(t *testing.T) {
	unavailable := codeplug.BoolField{State: codeplug.Unavailable}

	sess, closeAll, err := wiring.OpenFakeSessionFor(testAppCtx(t), wiring.IC7100Model)
	if err != nil {
		t.Fatalf("wiring.OpenFakeSessionFor(%q): unexpected error: %v", wiring.IC7100Model, err)
	}
	t.Cleanup(func() { _ = closeAll() })
	if got := sess.Capabilities().Model; got != wiring.IC7100Model {
		t.Fatalf("the fake session's Capabilities().Model = %q, want %q — this row is wired to the wrong driver", got, wiring.IC7100Model)
	}

	a, _ := newTestApp(t)
	connectDirect(t, a, sess, nil)
	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec (connected to the IC-7100 fake): unexpected error: %v", err)
	}
	if !got.Live {
		t.Error("Live = false, want true (connected to the registered fake)")
	}
	if len(got.Banks) != 1 {
		t.Fatalf("banks = %v, want exactly the one dense MEM bank — this radio discovers nothing and its special channels are refused", bankIDs(got.Banks))
	}
	for _, b := range got.Banks {
		if b.TagDisplayDefault != unavailable {
			t.Errorf("connected bank %s TagDisplayDefault = %+v, want %+v — this radio's memory frame has no display flag", b.ID, b.TagDisplayDefault, unavailable)
		}
		if !reflect.DeepEqual(b.Fields, ic7100TierFields) {
			t.Errorf("connected bank %s Fields = %v, want %v", b.ID, b.Fields, ic7100TierFields)
		}
	}

	// Offline, from this row's own file: the same answers, from the
	// static RealHardware baseline this time.
	a.mu.Lock()
	a.conn = nil
	a.working = &codeplug.Codeplug{
		Schema:   codeplug.CurrentSchema,
		Radio:    codeplug.RadioInfo{Model: wiring.IC7100Model},
		Channels: []codeplug.Channel{{Slot: "A-001"}},
	}
	a.mu.Unlock()
	offline, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec (offline, IC-7100 working copy): unexpected error: %v", err)
	}
	if offline.Live {
		t.Error("Live = true, want false (disconnected)")
	}
	if len(offline.Banks) == 0 {
		t.Fatal("offline IC-7100 UISpec has no banks — nothing asserted")
	}
	for _, b := range offline.Banks {
		if b.TagDisplayDefault != unavailable {
			t.Errorf("offline bank %s TagDisplayDefault = %+v, want %+v", b.ID, b.TagDisplayDefault, unavailable)
		}
		if !reflect.DeepEqual(b.Fields, ic7100TierFields) {
			t.Errorf("offline bank %s Fields = %v, want %v", b.ID, b.Fields, ic7100TierFields)
		}
	}
}

// TestBankReadOnly_RegisteredICR8600_RealHardwareProfile pins what a REAL
// IC-R8600's grid does through real registration — the additions tier's
// fourth mirror of TestBankReadOnly_RegisteredIC7610_RealHardwareProfile.
//
// Its RealHardware profile is its all-Unverified one (writeTrialsComplete
// is false: no IC-R8600 has ever been written to by this project), so the
// three derived core fields are Write spec.Unverified — which is NOT
// spec.Unsupported, and therefore NOT read-only under bankReadOnly's
// standing rule, on the same footing as every other row.
//
// A RECEIVER IS NOT A READ-ONLY RADIO, and this is the test that says so.
// spec.ReceiveOnly is anatomy — it removes tx_frequency and tone_tx from
// the graded set (additions spec D4.2's invariant) — and it says nothing
// whatever about whether the memories may be written. Collapsing the two
// would lock a receiver's grid and break the offline clone workflow for
// the one model in this registry that is most obviously a memory-list
// radio, so the assertion below is deliberately identical to every
// transceiver's.
//
// ONE BANK, and no discovered-bank contrast at the STATIC baseline: the
// single spec.BankMemory this driver declares is fixed at construction
// (core/driver/icr8600/caps.go), and it is SPARSE with no materialised
// Slots until a session walks the space, which is a property of the slot
// list rather than of the bank's field grading.
func TestBankReadOnly_RegisteredICR8600_RealHardwareProfile(t *testing.T) {
	caps, err := wiring.StaticCapabilities(wiring.ICR8600Model)
	if err != nil {
		t.Fatalf("wiring.StaticCapabilities(%q): unexpected error: %v", wiring.ICR8600Model, err)
	}
	if len(caps.Banks) != 1 {
		t.Fatalf("the registered %s's static baseline has %d banks, want exactly 1 (the sparse MEM space)", wiring.ICR8600Model, len(caps.Banks))
	}
	if caps.Transmit != spec.ReceiveOnly {
		t.Fatalf("the registered %s declares Transmit = %v, want spec.ReceiveOnly — every assertion below is about a receiver", wiring.ICR8600Model, caps.Transmit)
	}
	for _, b := range caps.Banks {
		if !b.Sparse || !b.BudgetUnstated {
			t.Errorf("bank %s Sparse = %v, BudgetUnstated = %v, want both true — the guide states no capacity anywhere (additions spec D3.4, register entry icr8600-budget)", b.ID, b.Sparse, b.BudgetUnstated)
		}
		fields := bankCoreFields(caps, b.ID)
		wantFields(t, "IC-R8600 bank "+string(b.ID), fields, icr8600CoreThree)
		for _, f := range fields {
			if w := caps.FieldSupport(b.ID, f).Write; w != spec.Unverified {
				t.Errorf("bank %s field %s Write = %v, want Unverified (the premise: nothing on a real IC-R8600 is proven writable)", b.ID, f, w)
			}
		}
		if bankReadOnly(caps, b.ID) {
			t.Errorf("bankReadOnly(%s) = true, want false — Unverified is not Unsupported, a receiver is not a read-only radio, and locking it would break the offline clone workflow", b.ID)
		}
	}
}

// TestGetUISpec_RegisteredICR8600_EveryBankFieldsAndTagDisplay is the
// additions tier's fourth mirror of
// TestGetUISpec_RegisteredIC7610_EveryBankFieldsAndTagDisplay: GetUISpec
// driven for the new row through real registration, connected and
// offline.
//
//   - CONNECTED to the registered fake (Live true, the Simulated profile
//     — the `--fake --model IC-R8600` path a user actually walks). This
//     receiver discovers no extra bank; the sparse walk fills the one MEM
//     space's slot list instead.
//   - DISCONNECTED with its own working copy loaded (Live false, the
//     static RealHardware baseline, resolved by currentModel from the
//     file's own Radio.Model) — the offline clone workflow's path.
//
// TagDisplayDefault must be {state: "unavailable"} on both paths: this
// receiver's 1A 00 record has no display flag at all (FieldTagDisplay
// carries the zero FieldSupport, core/driver/icr8600/caps.go's
// bankFields), so a blank row added anywhere must not carry a Known one.
//
// FIELDS MUST EQUAL icr8600TierFields ON BOTH PATHS, and this is the
// assertion the whole D8 design comes to rest on: the seven receiver
// columns must appear here, derived from THIS MODEL'S CAPABILITIES ALONE
// through the bankTierFields body that no registration edited, and the
// two TX-adjacent columns must not.
//
// THE OFFLINE LEG USES A ZERO-BASED WIDE-GROUP SLOT, "G00-000": this
// receiver's groups AND channels both count from zero (additions spec
// Erratum 2, core/civ/icr8600/profile.go), which no earlier registered
// model does, and a working copy holding a slot its own driver would
// refuse would be testing the wrong thing.
//
// THE CONNECTED LEG IS WHERE THE REGISTRATION IS ACTUALLY EXERCISED: a row
// wired to another model's driver or another model's option source shows
// up here as a wrong Capabilities().Model, which is asserted before
// anything else.
func TestGetUISpec_RegisteredICR8600_EveryBankFieldsAndTagDisplay(t *testing.T) {
	unavailable := codeplug.BoolField{State: codeplug.Unavailable}

	sess, closeAll, err := wiring.OpenFakeSessionFor(testAppCtx(t), wiring.ICR8600Model)
	if err != nil {
		t.Fatalf("wiring.OpenFakeSessionFor(%q): unexpected error: %v", wiring.ICR8600Model, err)
	}
	t.Cleanup(func() { _ = closeAll() })
	if got := sess.Capabilities().Model; got != wiring.ICR8600Model {
		t.Fatalf("the fake session's Capabilities().Model = %q, want %q — this row is wired to the wrong driver", got, wiring.ICR8600Model)
	}

	a, _ := newTestApp(t)
	connectDirect(t, a, sess, nil)
	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec (connected to the IC-R8600 fake): unexpected error: %v", err)
	}
	if !got.Live {
		t.Error("Live = false, want true (connected to the registered fake)")
	}
	if len(got.Banks) != 1 {
		t.Fatalf("banks = %v, want exactly the one sparse MEM bank — this receiver discovers no second namespace", bankIDs(got.Banks))
	}
	for _, b := range got.Banks {
		if b.TagDisplayDefault != unavailable {
			t.Errorf("connected bank %s TagDisplayDefault = %+v, want %+v — this receiver's memory frame has no display flag", b.ID, b.TagDisplayDefault, unavailable)
		}
		if !reflect.DeepEqual(b.Fields, icr8600TierFields) {
			t.Errorf("connected bank %s Fields = %v, want %v", b.ID, b.Fields, icr8600TierFields)
		}
	}

	// Offline, from this row's own file: the same answers, from the
	// static RealHardware baseline this time.
	a.mu.Lock()
	a.conn = nil
	a.working = &codeplug.Codeplug{
		Schema:   codeplug.CurrentSchema,
		Radio:    codeplug.RadioInfo{Model: wiring.ICR8600Model},
		Channels: []codeplug.Channel{{Slot: "G00-000"}},
	}
	a.mu.Unlock()
	offline, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec (offline, IC-R8600 working copy): unexpected error: %v", err)
	}
	if offline.Live {
		t.Error("Live = true, want false (disconnected)")
	}
	if len(offline.Banks) == 0 {
		t.Fatal("offline IC-R8600 UISpec has no banks — nothing asserted")
	}
	for _, b := range offline.Banks {
		if b.TagDisplayDefault != unavailable {
			t.Errorf("offline bank %s TagDisplayDefault = %+v, want %+v", b.ID, b.TagDisplayDefault, unavailable)
		}
		if !reflect.DeepEqual(b.Fields, icr8600TierFields) {
			t.Errorf("offline bank %s Fields = %v, want %v", b.ID, b.Fields, icr8600TierFields)
		}
	}
}

// TestGetUISpec_RegisteredICR8600_IsAReceiver is the receiver half of the
// registration, asserted through the real registry rather than the
// hand-built fixture TestGetUISpec_TransmitFollowsRadioCapabilities uses.
//
// THE TWO TESTS ARE NOT DUPLICATES, and the difference is the point. That
// one proves the MECHANISM — a fixture declaring spec.ReceiveOnly renders
// "receive_only" — and it has passed since additions spec D4.2 landed,
// against a capability value no registered radio held. This one proves the
// FACT: the registry's IC-R8600 row really is a receiver, on both the live
// simulated path and the static offline one, so the mechanism is now
// carrying a real model rather than a fixture.
//
// AND IT PINS THE GRID LEGEND'S OWN WORDING. UISpecView.GridLegendNote is
// served from internal/radiotext (app/uispec.go), and D4.2 asks for the
// receiver's legend to say "receiver — no transmit fields" IN THOSE WORDS
// rather than labelling an absent column unwritable. That substring is
// asserted here because this is the seam where the sentence reaches the
// UI; internal/radiotext's own TestRadiotext_ICR8600Verbatim pins the
// whole string at its source.
//
// BOTH PATHS, deliberately: Transmit is radio ANATOMY, so a connected
// session and a file loaded offline must agree about it. A build that
// derived it from the live session alone would tell an offline user
// nothing about the radio their file came from.
func TestGetUISpec_RegisteredICR8600_IsAReceiver(t *testing.T) {
	const receiverLegend = "receiver — no transmit fields"

	sess, closeAll, err := wiring.OpenFakeSessionFor(testAppCtx(t), wiring.ICR8600Model)
	if err != nil {
		t.Fatalf("wiring.OpenFakeSessionFor(%q): unexpected error: %v", wiring.ICR8600Model, err)
	}
	t.Cleanup(func() { _ = closeAll() })

	a, _ := newTestApp(t)
	connectDirect(t, a, sess, nil)
	got, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec (connected to the IC-R8600 fake): unexpected error: %v", err)
	}
	if got.Transmit != "receive_only" {
		t.Errorf("connected Transmit = %q, want \"receive_only\" — the IC-R8600 is the one registered row whose radio has no transmitter", got.Transmit)
	}
	if !strings.Contains(got.GridLegendNote, receiverLegend) {
		t.Errorf("connected GridLegendNote = %q, want it to contain %q — additions spec D4.2 asks for those words, and this is the seam that serves them", got.GridLegendNote, receiverLegend)
	}

	a.mu.Lock()
	a.conn = nil
	a.working = &codeplug.Codeplug{
		Schema:   codeplug.CurrentSchema,
		Radio:    codeplug.RadioInfo{Model: wiring.ICR8600Model},
		Channels: []codeplug.Channel{{Slot: "G00-000"}},
	}
	a.mu.Unlock()
	offline, err := a.GetUISpec()
	if err != nil {
		t.Fatalf("GetUISpec (offline, IC-R8600 working copy): unexpected error: %v", err)
	}
	if offline.Transmit != "receive_only" {
		t.Errorf("offline Transmit = %q, want \"receive_only\" — anatomy does not depend on being connected", offline.Transmit)
	}
	if !strings.Contains(offline.GridLegendNote, receiverLegend) {
		t.Errorf("offline GridLegendNote = %q, want it to contain %q", offline.GridLegendNote, receiverLegend)
	}
	// And the one bank must carry the undocumented-capacity flag on the
	// offline path too: additions spec D3.4 makes BudgetUnstated the
	// positive declaration of a silence, and a UI that lost it offline
	// would be back to implying a capacity nobody has printed.
	if len(offline.Banks) != 1 || !offline.Banks[0].BudgetUnstated {
		t.Errorf("offline Banks = %+v, want one bank carrying BudgetUnstated", offline.Banks)
	}
}
