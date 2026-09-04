// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx101_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/cat/dialecttest"
	"github.com/gm5dna/open-rig-programmer/core/cat/ftdx10"
	"github.com/gm5dna/open-rig-programmer/core/cat/ftdx101"
)

// This file is M9d-1 task 6's evidence: the TWO FTdx101 dialects held to
// core/cat's conformance suite, then pinned three ways — against each other
// (the sibling pins, which are what makes "D and MP differ ONLY in the CAT
// ID" an assertion rather than a claim), against cat.FT710 (what the three
// radios SHARE, which this package retypes rather than references), and
// against the other two radios (what they must NOT share: the CAT ID, and,
// versus the FT-710, the MT frame form, the MW write kind and the answer
// bounds; versus the FTdx10, the EX inventory).
//
// TWO DIALECTS, ONE INVENTORY, ONE MANUAL. Yaesu prints one CAT manual for
// the FTDX101D and the FTDX101MP and distinguishes them in exactly THREE
// places: the ID answer's value, the P4 VALUE ranges of three MAX POWER rows
// in Table 2, and the PC (POWER CONTROL) command's P1 range — "005 - 100
// (FTDX101D)" against "005 - 200 (FTDX101MP)", layout 1496 and 1498. Only the
// FIRST is a dialect datum: P4 semantics are not stored, and PC is off this
// project's surface entirely (M9d-1 models no PC command). So the two
// instances differ in the CAT ID and in nothing else THIS PACKAGE MODELS.
// That is a claim about a constructor with one string parameter, and it is
// cheap to make and easy to erode; the sibling pins below enumerate the
// dialect's data accessors rather than trusting the constructor's shape. See
// doc.go's "One manual, two radios" for all three differences.
//
// The identity pins exist because this package's mode table and slot space
// are FRESH TRANSCRIPTIONS. core/cat's are unexported, so there is nothing to
// share even if sharing were right; retyping buys independence and costs a
// copy error, and these tests are where a copy error is caught. They are
// deliberately BEHAVIOURAL — everything is asked through the exported API,
// over the whole wire space, rather than by comparing two tables — because a
// table comparison would prove the tables match and say nothing about what
// either dialect DOES with them.
//
// TWO ASSUMED MEMBERS ARE EMBEDDED IN WHAT THE PINS COMPARE, and are noted
// at each site: cat.ModeUnset ('0', "-") is in no FTdx101 mode legend, and
// the none wire "000" is in no FTdx101 slot legend. So "identical to the
// FT-710" here means identical INCLUDING two members inherited from it.
// doc.go's ASSUMED register carries the full statement and the Stage R
// capture that lifts each — PER MODEL, since a capture taken from a D lifts
// nothing for an MP.

// bothModels is the pair every per-model assertion below runs over. Named
// rather than inlined so that a pin added later cannot quietly cover one
// model: a table of two is harder to half-fill than two copied blocks.
func bothModels() []struct {
	name string
	d    cat.Dialect
} {
	return []struct {
		name string
		d    cat.Dialect
	}{
		{"FTDX101D", ftdx101.DialectD()},
		{"FTDX101MP", ftdx101.DialectMP()},
	}
}

// TestConformance runs core/cat's whole exported-API conformance suite over
// BOTH real FTdx101 dialects: every builder's frames are well-formed and
// admitted by that dialect's own gate, both wrong-form APIs refuse and are
// seen to refuse, the clarifier's endpoints build and one step past is
// refused, and the walk fails if it built nothing.
//
// Both, not one. The two instances come from one constructor, so a suite run
// over the D "obviously" covers the MP — but that inference is exactly the
// sibling-pin claim, and a conformance suite that assumed the claim it is
// meant to backstop would be worth nothing the day someone splits the
// constructor in two.
//
// dialecttest.RunZeroValue IS called here, and that is a DELIBERATE
// divergence from core/cat/ftdx10/dialect_test.go, which records an omission
// on the ground that the zero-value suite is universal and core/cat already
// runs it. Both readings are defensible; the M9d spec mandates the call, the
// duplication costs microseconds, and a dialect package that runs the whole
// suite it is offered is the shape the next dialect should copy.
func TestConformance(t *testing.T) {
	for _, m := range bothModels() {
		t.Run(m.name, func(t *testing.T) {
			dialecttest.Run(t, m.d)
		})
	}
	dialecttest.RunZeroValue(t)
}

// TestModeIdentityWithFT710 sweeps ALL 256 wire bytes through both FTdx101
// dialects and cat.FT710 and requires the same verdict and the same display
// name from each.
//
// The whole byte space, not the sixteen this package declares: a table with a
// SPURIOUS member — a lower-case 'a', a stray 'G' — is invisible to a walk
// over its own keys, and that is precisely the copy error a fresh
// transcription risks. The sweep also fixes the count at 16, so a table that
// silently lost a mode fails here rather than passing a comparison of two
// short tables.
//
// The transcription's own provenance is in dialect.go: FIVE identical
// legends in manual rev 2308-L (MD's P2 at layout 1240-1243, IF's P6 at
// 1089-1091, MR's P6 at 1286-1288, MT's P6 at 1321-1323, MW's P6 at
// 1361-1363), all running 1-F, plus a SIXTH — OI's P6 at 1443-1446 — which
// misprints "F: DATA-FM-N" as "E: DATA-FM-N" and is excluded from sourcing.
func TestModeIdentityWithFT710(t *testing.T) {
	for _, m := range bothModels() {
		t.Run(m.name, func(t *testing.T) {
			d := m.d

			valid := 0
			for c := 0; c < 256; c++ {
				mode := cat.Mode(byte(c))
				gotValid, wantValid := d.ValidMode(mode), cat.FT710.ValidMode(mode)
				if gotValid != wantValid {
					t.Errorf("ValidMode(%#02x): %s %v, FT-710 %v — the two mode tables are transcribed from the same 1-F legend and must accept exactly the same bytes", c, m.name, gotValid, wantValid)
					continue
				}
				if !gotValid {
					continue
				}
				valid++
				if got, want := d.ModeName(mode), cat.FT710.ModeName(mode); got != want {
					t.Errorf("ModeName(%#02x): %s %q, FT-710 %q", c, m.name, got, want)
				}
			}
			if valid != 16 {
				t.Errorf("the %s dialect accepts %d mode bytes, want 16 — this manual's legends run 1-F (fifteen) plus the ASSUMED '0' placeholder", m.name, valid)
			}

			// ModeByName is the WRITE direction: it is how a stored channel's
			// mode string becomes a wire byte again. A name that resolves to a
			// different nibble than the one it was rendered from would write
			// the wrong mode into a memory, which no read-side comparison
			// above would notice.
			for c := 0; c < 256; c++ {
				mode := cat.Mode(byte(c))
				if !d.ValidMode(mode) {
					continue
				}
				name := d.ModeName(mode)
				got, ok := d.ModeByName(name)
				if !ok {
					t.Errorf("ModeByName(%q) did not resolve, but ModeName(%#02x) produced it — the two are inverses", name, c)
					continue
				}
				if got != mode {
					t.Errorf("ModeByName(%q) = %#02x, want %#02x — a name resolving to another dialect member would write the wrong mode nibble", name, byte(got), c)
				}
			}

			// THE ASSUMED MEMBER, stated rather than left inside the sweep
			// above. cat.ModeUnset appears in NO FTdx101 mode legend: all
			// five clean ones run 1-F, and so does the defective OI one. It
			// is here because parsers must accept the placeholder; core/cat
			// refuses to emit it in any Set frame.
			if !d.ValidMode(cat.ModeUnset) {
				t.Errorf("ValidMode(cat.ModeUnset) is false on the %s — the ASSUMED placeholder member is missing, and a radio answering '0' would be unreadable", m.name)
			}
			if got := d.ModeName(cat.ModeUnset); got != "-" {
				t.Errorf("ModeName(cat.ModeUnset) = %q, want %q", got, "-")
			}
		})
	}
}

// threeDigits renders n as the three-digit wire form every slot-bearing
// command carries. Local to this file rather than shared: the conformance
// suite has its own, and a helper reaching across package boundaries to be
// reused is how two walks end up sweeping the same space by accident.
func threeDigits(n int) string {
	return fmt.Sprintf("%03d", n)
}

// slotWires is every wire form either slot space can produce: all thousand
// three-digit codes, all eighteen PMS forms, and "EMG".
func slotWires() []string {
	wires := make([]string, 0, 1000+18+1)
	for n := 0; n <= 999; n++ {
		wires = append(wires, threeDigits(n))
	}
	for pair := 1; pair <= 9; pair++ {
		wires = append(wires, fmt.Sprintf("P%dL", pair), fmt.Sprintf("P%dU", pair))
	}
	return append(wires, "EMG")
}

// TestSlotIdentityWithFT710 compares each FTdx101 dialect's slot space with
// the FT-710's BEHAVIOURALLY, over every wire form either can produce.
//
// Behaviour, not tables, and specifically NOT Slot.IsMemory/IsPMS/Is60m/
// IsEMG/IsNone. Those predicates were FT-710-scoped when this test was
// written — an M9b deferral, ledgered and documented on the package-level
// classifySlotWire in core/cat/slot.go — so on an FTdx101 slot they would
// have answered for the wrong radio and this test would have pinned that
// mistake rather than the slot space. M9d discharged the deferral (a Slot
// now carries its constructing dialect's classification, cat.Slot's doc
// comment), so they would answer correctly today; this test still goes
// through a DIALECT receiver throughout, because what it is comparing is
// two dialects' slot SPACES, and a receiver is what names the space under
// test unambiguously.
//
// THE NONE WIRE "000" IS ONE OF THE FORMS COMPARED, and it is ASSUMED: no
// FTdx101 slot legend names it. MC's gives "001-099 (Memory Channel), P1L
// -P9U (PMS), 5xx (5MHz BAND), EMG (EMERGENCY CH)" (layout 1225-1227), MR's
// the same (1278-1279), MT's the same (1312-1313), and MW's is restricted to
// "001-099 (Memory Channel), P1L -P9U (PMS)" (1353). It is the FT-710's
// MR-answer fact, and cat.SlotSpace structurally requires a none form, so one
// is supplied.
//
// THE 5xx BOUNDS ARE ASSUMED TOO, and they are what the 501..599 half of the
// count below is made of: every legend says only "5xx (5MHz BAND)". See
// doc.go's register.
func TestSlotIdentityWithFT710(t *testing.T) {
	wires := slotWires()

	for _, m := range bothModels() {
		t.Run(m.name, func(t *testing.T) {
			d := m.d

			accepted := 0
			for _, w := range wires {
				got, gotErr := d.ParseSlot(w)
				want, wantErr := cat.FT710.ParseSlot(w)
				if (gotErr == nil) != (wantErr == nil) {
					t.Errorf("ParseSlot(%q): %s err=%v, FT-710 err=%v — the two slot spaces are declared identically and must accept the same wire forms", w, m.name, gotErr, wantErr)
					continue
				}
				if gotErr != nil {
					continue
				}
				accepted++
				if got.Wire() != want.Wire() {
					t.Errorf("ParseSlot(%q): %s canonicalised to %q, FT-710 to %q", w, m.name, got.Wire(), want.Wire())
				}
			}
			// 99 memory + 99 sixty-metre + "000" + 18 PMS + "EMG".
			if want := 99 + 99 + 1 + 18 + 1; accepted != want {
				t.Errorf("the %s dialect classifies %d of the %d wire forms swept, want %d — a sweep this test agrees on but that accepts nothing would pass every comparison above in silence", m.name, accepted, len(wires), want)
			}

			// THE CONSTRUCTORS, over their boundaries in both directions.
			// ParseSlot above proves the two dialects READ the same wire
			// space; these prove they WRITE it the same way, which is a
			// different function on a different bound — SixtyMSlot in
			// particular takes an ordinal and derives the wire form from its
			// own sixtyLo, so a dialect with the same range and a different
			// base would agree above and disagree here.
			for n := -1; n <= 101; n++ {
				gotSlot, gotErr := d.MemorySlot(n)
				wantSlot, wantErr := cat.FT710.MemorySlot(n)
				compareConstructor(t, m.name, fmt.Sprintf("MemorySlot(%d)", n), gotSlot, gotErr, wantSlot, wantErr)

				gotSlot, gotErr = d.SixtyMSlot(n)
				wantSlot, wantErr = cat.FT710.SixtyMSlot(n)
				compareConstructor(t, m.name, fmt.Sprintf("SixtyMSlot(%d)", n), gotSlot, gotErr, wantSlot, wantErr)
			}
			for pair := -1; pair <= 10; pair++ {
				for _, upper := range []bool{false, true} {
					gotSlot, gotErr := d.PMSSlot(pair, upper)
					wantSlot, wantErr := cat.FT710.PMSSlot(pair, upper)
					compareConstructor(t, m.name, fmt.Sprintf("PMSSlot(%d, %v)", pair, upper), gotSlot, gotErr, wantSlot, wantErr)
				}
			}
			if got, want := d.EMGSlot().Wire(), cat.FT710.EMGSlot().Wire(); got != want {
				t.Errorf("EMGSlot(): %s %q, FT-710 %q", m.name, got, want)
			}
			if got := d.EMGSlot().Wire(); got != "EMG" {
				t.Errorf("EMGSlot() = %q, want %q — this manual's MC, MR and MT legends all name EMG (EMERGENCY CH)", got, "EMG")
			}
		})
	}
}

// compareConstructor holds one constructor call's outcome on two dialects to
// each other: the same acceptance verdict, and on acceptance the same wire
// form.
func compareConstructor(t *testing.T, who, what string, got cat.Slot, gotErr error, want cat.Slot, wantErr error) {
	t.Helper()

	if (gotErr == nil) != (wantErr == nil) {
		t.Errorf("%s: %s err=%v, other err=%v", what, who, gotErr, wantErr)
		return
	}
	if gotErr != nil {
		return
	}
	if got.Wire() != want.Wire() {
		t.Errorf("%s: %s built %q, other built %q", what, who, got.Wire(), want.Wire())
	}
}

// TestSiblingPins is the enumerated form of this package's central claim:
// the FTDX101D and the FTDX101MP differ in the CAT ID AND IN NOTHING ELSE
// that this dialect models.
//
// It is asserted ACCESSOR BY ACCESSOR rather than argued from newDialect's
// single parameter, because the constructor's shape is not the claim — the
// claim is about what consumers can observe, and a future field that
// legitimately differs per model (a per-model EX inventory, say, if a firmware
// revision ever splits the chart) must break this test rather than slip
// through a structural argument.
//
// WHAT THESE PINS COVER, EXACTLY. They directly compare every dialect-DATA
// accessor D-against-MP: CATID, Configured, the mode table over all 256 wire
// bytes, ParseSlot and the four slot constructors over their full sweeps, the
// EX inventory and its address projection, KnownEXAddress over the union AND
// over the grammar block's whole declared range, MTForm, MWWriteKind,
// Clarifier and MTAnswerBounds. They do NOT re-run behaviour — the builders,
// the parsers, AllowedCommand, the error paths — through a second D-vs-MP
// comparison here, and the earlier wording of this comment, which claimed the
// "whole public Dialect surface", overstated that. BEHAVIOURAL equality is
// carried by three other things, and this pin is narrow because they exist:
//
//   - dialecttest.Run passes identically on BOTH instances (TestConformance
//     above runs the whole conformance suite over each), so every builder,
//     parser and refusal path the suite exercises is exercised on both.
//
//   - golden_test.go's frame-level D-vs-MP legs build and parse every frozen
//     vector on both instances and require byte equality, and they compare
//     AllowedCommand admissibility on both for every built frame.
//
//   - structurally, catID is read by exactly TWO methods in the whole of
//     core/cat — Configured() and CATID(), core/cat/dialect.go:266-269 — and
//     it is the ONLY field newDialect varies between the two instances, so no
//     other method CAN diverge. That is a fact about the field's readers, and
//     it is what makes the data-accessor sweep above sufficient rather than
//     merely reassuring.
//
// The evidence for the claim itself is task 1's applicability attestation
// (testdata/group-ledger.md): every property an EXItem models — address,
// labels, name, digits, text flag — is printed identically for both models on
// every row of Table 2, and the only model-conditional printing in the whole
// chart is the P4 VALUE range of three MAX POWER rows, which no EXItem
// stores. doc.go records both, and records the third per-model difference —
// the PC command's P1 range — as off this project's surface.
func TestSiblingPins(t *testing.T) {
	dd, mp := ftdx101.DialectD(), ftdx101.DialectMP()

	// The ONE difference, stated first so the rest reads as "and nothing
	// else". Layout 1070-1072: "0681: FTDX101D", "0682: FTDX101MP".
	if got := dd.CATID(); got != "0681" {
		t.Errorf("DialectD().CATID() = %q, want %q", got, "0681")
	}
	if got := mp.CATID(); got != "0682" {
		t.Errorf("DialectMP().CATID() = %q, want %q", got, "0682")
	}
	if dd.CATID() == mp.CATID() {
		t.Fatalf("both FTdx101 dialects answer CATID() = %q — the two instances exist ONLY so that identity can be honest per model, and a shared ID makes them one dialect wearing two names", dd.CATID())
	}

	if !dd.Configured() || !mp.Configured() {
		t.Fatalf("Configured(): D %v, MP %v — both must be true, and every assertion below is vacuous on an unconfigured dialect", dd.Configured(), mp.Configured())
	}

	// Modes, over all 256 wire bytes rather than the declared sixteen: the
	// same spurious-member argument as the FT-710 identity pin.
	for c := 0; c < 256; c++ {
		mode := cat.Mode(byte(c))
		if got, want := dd.ValidMode(mode), mp.ValidMode(mode); got != want {
			t.Errorf("ValidMode(%#02x): D %v, MP %v — one manual, one legend, one table", c, got, want)
			continue
		}
		if !dd.ValidMode(mode) {
			continue
		}
		if got, want := dd.ModeName(mode), mp.ModeName(mode); got != want {
			t.Errorf("ModeName(%#02x): D %q, MP %q", c, got, want)
		}
		gotByName, gotOK := dd.ModeByName(dd.ModeName(mode))
		wantByName, wantOK := mp.ModeByName(mp.ModeName(mode))
		if gotOK != wantOK || gotByName != wantByName {
			t.Errorf("ModeByName(%q): D (%#02x, %v), MP (%#02x, %v)", dd.ModeName(mode), byte(gotByName), gotOK, byte(wantByName), wantOK)
		}
	}

	// Slots, read side and write side, over the same full sweep the FT-710
	// pin uses.
	for _, w := range slotWires() {
		got, gotErr := dd.ParseSlot(w)
		want, wantErr := mp.ParseSlot(w)
		if (gotErr == nil) != (wantErr == nil) {
			t.Errorf("ParseSlot(%q): D err=%v, MP err=%v", w, gotErr, wantErr)
			continue
		}
		if gotErr != nil {
			continue
		}
		if got.Wire() != want.Wire() {
			t.Errorf("ParseSlot(%q): D %q, MP %q", w, got.Wire(), want.Wire())
		}
	}
	for n := -1; n <= 101; n++ {
		gotSlot, gotErr := dd.MemorySlot(n)
		wantSlot, wantErr := mp.MemorySlot(n)
		compareConstructor(t, "FTDX101D", fmt.Sprintf("MemorySlot(%d) vs MP", n), gotSlot, gotErr, wantSlot, wantErr)

		gotSlot, gotErr = dd.SixtyMSlot(n)
		wantSlot, wantErr = mp.SixtyMSlot(n)
		compareConstructor(t, "FTDX101D", fmt.Sprintf("SixtyMSlot(%d) vs MP", n), gotSlot, gotErr, wantSlot, wantErr)
	}
	for pair := -1; pair <= 10; pair++ {
		for _, upper := range []bool{false, true} {
			gotSlot, gotErr := dd.PMSSlot(pair, upper)
			wantSlot, wantErr := mp.PMSSlot(pair, upper)
			compareConstructor(t, "FTDX101D", fmt.Sprintf("PMSSlot(%d, %v) vs MP", pair, upper), gotSlot, gotErr, wantSlot, wantErr)
		}
	}
	if got, want := dd.EMGSlot().Wire(), mp.EMGSlot().Wire(); got != want {
		t.Errorf("EMGSlot(): D %q, MP %q", got, want)
	}

	// The EX inventory, COMPLETE — labels, names, digits and text flags, not
	// merely the addresses. reflect.DeepEqual over the whole slice is the
	// only comparison that catches a per-model divergence in a field nobody
	// thought to enumerate, and the two slices are independent copies
	// (Dialect.EXItems returns a copy), so equality here is real.
	itemsD, itemsMP := dd.EXItems(), mp.EXItems()
	if len(itemsD) == 0 {
		t.Fatal("EXItems() is empty on the D — every inventory assertion below would be vacuous")
	}
	if !reflect.DeepEqual(itemsD, itemsMP) {
		t.Errorf("EXItems() differ between the D and the MP (%d vs %d items) — one printed MENU Chart serves both models, and every field an EXItem stores is printed identically for both (task 1's applicability attestation)", len(itemsD), len(itemsMP))
	}

	// EXAddresses element-wise as well as DeepEqual above: the two are
	// different projections (the accessor sorts and de-duplicates), and a
	// dialect whose address list disagreed with its own item list would pass
	// one and fail the other.
	addrD, addrMP := dd.EXAddresses(), mp.EXAddresses()
	if len(addrD) != len(addrMP) {
		t.Errorf("EXAddresses(): D has %d, MP has %d", len(addrD), len(addrMP))
	} else {
		for i := range addrD {
			if addrD[i] != addrMP[i] {
				t.Errorf("EXAddresses()[%d]: D %v, MP %v", i, addrD[i], addrMP[i])
			}
		}
	}

	// KnownEXAddress over the UNION of both inventories, both directions —
	// a one-way walk over the D's own addresses can only ever show the MP
	// knows at least as much.
	union := make(map[cat.EXAddress]bool, len(addrD)+len(addrMP))
	for _, a := range addrD {
		union[a] = true
	}
	for _, a := range addrMP {
		union[a] = true
	}
	if len(union) != len(addrD) {
		t.Errorf("the union of the two inventories has %d addresses against the D's %d — the two models' membership sets are not the same set", len(union), len(addrD))
	}
	for a := range union {
		if got, want := dd.KnownEXAddress(a), mp.KnownEXAddress(a); got != want {
			t.Errorf("KnownEXAddress(%v): D %v, MP %v", a, got, want)
		}
	}
	// And over the grammar block's whole declared range, so that NON-members
	// are compared too: membership agreeing on every address either radio
	// carries says nothing about the addresses neither does.
	for p1 := uint8(1); p1 <= 6; p1++ {
		for p2 := uint8(1); p2 <= 8; p2++ {
			for p3 := uint8(1); p3 <= 25; p3++ {
				a := cat.EXAddress{P1: p1, P2: p2, P3: p3}
				if got, want := dd.KnownEXAddress(a), mp.KnownEXAddress(a); got != want {
					t.Errorf("KnownEXAddress(%v): D %v, MP %v", a, got, want)
				}
			}
		}
	}

	// The frame-shape accessors, one by one.
	if got, want := dd.MTForm(), mp.MTForm(); got != want {
		t.Errorf("MTForm(): D %v, MP %v", got, want)
	}
	if got, want := dd.MWWriteKind(), mp.MWWriteKind(); got != want {
		t.Errorf("MWWriteKind(): D %q, MP %q", got, want)
	}
	if got, want := dd.Clarifier(), mp.Clarifier(); got != want {
		t.Errorf("Clarifier(): D %+v, MP %+v", got, want)
	}
	dmin, dmax, derr := dd.MTAnswerBounds()
	mmin, mmax, merr := mp.MTAnswerBounds()
	if (derr == nil) != (merr == nil) || dmin != mmin || dmax != mmax {
		t.Errorf("MTAnswerBounds(): D (%d, %d, %v), MP (%d, %d, %v)", dmin, dmax, derr, mmin, mmax, merr)
	}
}

// TestDifferencePinCATID is the identity that makes these different radios at
// all: the FTDX101D answers "ID;" with 0681 and the FTDX101MP with 0682
// (manual rev 2308-L, layout 1070-1072), against the FTdx10's 0761 and the
// FT-710's 0800.
//
// PAIRWISE over all four, because a probe that cannot tell two radios apart
// picks whichever driver was registered first, and the failure is silent: the
// wrong dialect's gate then approves frames for a radio it does not describe.
func TestDifferencePinCATID(t *testing.T) {
	ids := []struct {
		name string
		id   string
	}{
		{"FTDX101D", ftdx101.DialectD().CATID()},
		{"FTDX101MP", ftdx101.DialectMP().CATID()},
		{"FTdx10", ftdx10.Dialect().CATID()},
		{"FT-710", cat.FT710.CATID()},
	}
	for i := range ids {
		for j := i + 1; j < len(ids); j++ {
			if ids[i].id == ids[j].id {
				t.Errorf("%s and %s both answer CATID() = %q — a shared identity would make radio detection pick whichever driver was registered first", ids[i].name, ids[j].name, ids[i].id)
			}
		}
	}
}

// TestDifferencePinMTForm pins the frame shape against the FT-710's. The
// FTdx101's MT carries the whole 28-position memory field block plus P11 and
// a 12-character tag (layout 1311-1345); the FT-710's carries a slot, a
// display flag and a tag. They are different commands wearing the same two
// letters, which is why MTForm is data and each form's API refuses a dialect
// declaring the other.
func TestDifferencePinMTForm(t *testing.T) {
	for _, m := range bothModels() {
		if got := m.d.MTForm(); got != cat.MTFormCombined {
			t.Errorf("%s MTForm() = %v, want %v", m.name, got, cat.MTFormCombined)
		}
	}
	if got := cat.FT710.MTForm(); got != cat.MTFormShort {
		t.Errorf("the FT-710's MTForm() = %v, want %v — this pin is a DIFFERENCE, and it proves nothing if both dialects declare the same form", got, cat.MTFormShort)
	}
}

// TestDifferencePinMWWriteKind pins the FTdx101's MW P7.
//
// THE CAVEAT IS THE POINT. This manual's MW legend reads "P7 0: (Fixed)"
// (layout 1364), and cat.CombinedMTSetKind is the byte '0' — so the constant
// on the right is the correct SPELLING of what this radio documents. That the
// two coincide is A FACT OF THIS RADIO, not a rule: MW's P7 and the combined
// MT Set's P7 are different fields of different commands that this manual
// happens to fix at the same byte. A dialect whose MW kind differed from the
// combined form's Set constant is entirely expressible, and core/cat keeps
// them apart on purpose — validateCombinedMTFields uses the FORM's constant
// and never a dialect's mwWriteKind. Nothing here may be read as permission
// to derive one from the other.
//
// The FT-710 is the counter-example, and it is asserted rather than
// described: its MW kind is cat.KindMemory ('1'), HW-confirmed.
func TestDifferencePinMWWriteKind(t *testing.T) {
	for _, m := range bothModels() {
		if got := m.d.MWWriteKind(); got != cat.CombinedMTSetKind {
			t.Errorf("%s MWWriteKind() = %q, want %q (cat.CombinedMTSetKind) — this manual's MW legend reads \"P7 0: (Fixed)\"", m.name, got, cat.CombinedMTSetKind)
		}
		if cat.FT710.MWWriteKind() == m.d.MWWriteKind() {
			t.Errorf("the FT-710's MWWriteKind() is %q too — the two radios document different MW P7 bytes ('1' Memory against '0' Fixed), and a shared value here would mean one of the two transcriptions took the other's", cat.FT710.MWWriteKind())
		}
	}
}

// TestDifferencePinMTAnswerBounds pins the combined answer's EXACT length.
//
// 41 is not written anywhere in core/cat and must not be: the geometry is
// derived, 29 + TagMaxBytes, so a family with a different tag field gets a
// different window from the same code. This test states the arithmetic's
// ANSWER for this radio, which the manual's own Set/Answer chart runs to
// (layout 1311-1345: the 28 shared positions, P11 at 28, a 12-byte P12 tag at
// 29-40, ';' at 41) and which geometry_test.go re-derives from the committed
// raster-counted witness rather than from this manual's text layer.
//
// EXACTNESS IS ASSUMED, NOT CHART-PROVEN (register entry 6 in doc.go): the
// grid draws the MAXIMAL frame, and the FT-710 precedent makes a
// variable-width answer live. What this pin proves is that the DIALECT
// declares the exact form core/cat's combined seam currently implements;
// Stage R's short-tag answer capture decides whether the radio agrees, and
// the recorded contingency is a 30..41 window.
func TestDifferencePinMTAnswerBounds(t *testing.T) {
	for _, m := range bothModels() {
		min, max, err := m.d.MTAnswerBounds()
		if err != nil {
			t.Fatalf("%s MTAnswerBounds() = %v, want no error from a configured dialect", m.name, err)
		}
		if min != 41 || max != 41 {
			t.Errorf("%s MTAnswerBounds() = (%d, %d), want (41, 41) — the combined record's length is exact, so its bounds are equal", m.name, min, max)
		}
	}

	// The FT-710's window is a range, not a point. Asserting only its
	// inequality keeps this pin about the DIFFERENCE without restating that
	// radio's own numbers, which its own tests own.
	fmin, fmax, ferr := cat.FT710.MTAnswerBounds()
	if ferr != nil {
		t.Fatalf("the FT-710's MTAnswerBounds() = %v", ferr)
	}
	if fmin == fmax {
		t.Errorf("the FT-710's MTAnswerBounds() = (%d, %d) — equal bounds are the COMBINED form's signature, so this difference pin has lost its counter-example", fmin, fmax)
	}
}

// TestDifferencePinEXDisjointnessWithFTdx10 proves the FTdx101's and the
// FTdx10's inventories are DIFFERENT TABLES, in both directions, against
// addresses each chart really carries.
//
// Both directions, because a one-way check passes on a subset: if this
// package had somehow been generated from the FTdx10's CSV, "the FTdx101
// knows something the FTdx10 does not" would fail, but "the FTdx10 knows
// something the FTdx101 does not" is what catches the reverse mistake.
//
// The two radios' charts are close enough that this pin has to be specific.
// Both have P1 01-04 and no EXTENSION group, both name the same four menus,
// and most rows coincide — 193 addresses here against 197 there, agreeing on
// 183. The four addresses below are drawn from the two COMMITTED table2.csv
// files and each is cited to its own chart line:
//
//   - (03,01,23) KEYBOARD LANGUAGE — core/cat/ftdx101/table2.csv, layout 881.
//     It is THIS chart's highest P3, and the reason its EX grammar block's
//     "P3 : 01 - 23" is REACHED where the FTdx10's is not: that chart tops out
//     at P3=21 (core/cat/ftdx10/doc.go's own anomaly note), so no (03,01,23)
//     can exist there.
//
//   - (04,01,08) FREQ STYLE — core/cat/ftdx101/table2.csv, layout 956. The
//     FTdx101's DISPLAY subgroup runs P3 01-08 because it carries TFT
//     CONTRAST and DIMMER TFT, which the FTdx10 (a radio with no TFT) does
//     not; that chart's DISPLAY subgroup ends at (04,01,05).
//
//   - (01,03,21) ENC/DEC — core/cat/ftdx10/table2.csv, layout 711. The
//     FTdx10's MODE FM subgroup runs to P3=21; the FTdx101's ends at
//     (01,03,16) RPT SHIFT(50MHz).
//
//   - (02,01,18) CW INDICATOR — core/cat/ftdx10/table2.csv, layout 787. Both
//     charts carry a CW INDICATOR row, and it is the LAST row of the MODE CW
//     subgroup in each; the two subgroups simply have different memberships
//     ahead of it, so the same function lands on a different address. THE
//     ARITHMETIC, against the two committed CSVs:
//
//     The FTdx10's MODE CW runs to EIGHTEEN rows and OPENS with three AF-tone
//     rows the FTdx101's does not carry at all — (02,01,01) AF TREBLE GAIN,
//     (02,01,02) AF MIDDLE TONE GAIN, (02,01,03) AF BASS GAIN
//     (core/cat/ftdx10/table2.csv:199-201, layout 769-771). Every later member
//     of that subgroup is therefore displaced by three.
//
//     The FTdx101's MODE CW runs to SEVENTEEN and carries two members the
//     FTdx10's does not — (02,01,08) CW OUT SELECT
//     (core/cat/ftdx101/table2.csv:219, layout 821) and (02,01,12) CW BK-IN
//     DELAY (core/cat/ftdx101/table2.csv:223, layout 826). 18 - 3 + 2 = 17.
//
//     CW FREQ DISPLAY IS NOT THE MECHANISM, and this comment said it was
//     until the M9d-1 milestone review. BOTH charts carry it: the FTdx101's at
//     (02,01,14) (core/cat/ftdx101/table2.csv:225, layout 829) and the
//     FTdx10's at (02,01,15) (core/cat/ftdx10/table2.csv:213, layout 783) —
//     itself an instance of the same displacement: three positions earlier
//     here for the AF rows this chart lacks, two later for the two extra rows
//     that precede it, 15 - 3 + 2 = 14.
//
//     The ADDRESS is what differs, which is exactly what an inventory models.
func TestDifferencePinEXDisjointnessWithFTdx10(t *testing.T) {
	onlyHere := []struct {
		addr cat.EXAddress
		what string
	}{
		{cat.EXAddress{P1: 3, P2: 1, P3: 23}, "KEYBOARD LANGUAGE (ftdx101/table2.csv, layout 881)"},
		{cat.EXAddress{P1: 4, P2: 1, P3: 8}, "FREQ STYLE (ftdx101/table2.csv, layout 956)"},
	}
	onlyThere := []struct {
		addr cat.EXAddress
		what string
	}{
		{cat.EXAddress{P1: 1, P2: 3, P3: 21}, "ENC/DEC (ftdx10/table2.csv, layout 711)"},
		{cat.EXAddress{P1: 2, P2: 1, P3: 18}, "CW INDICATOR (ftdx10/table2.csv, layout 787)"},
	}

	for _, m := range bothModels() {
		for _, c := range onlyHere {
			if !m.d.KnownEXAddress(c.addr) {
				t.Errorf("KnownEXAddress(%v) is false on the %s — this chart carries it: %s", c.addr, m.name, c.what)
			}
		}
		for _, c := range onlyThere {
			if m.d.KnownEXAddress(c.addr) {
				t.Errorf("KnownEXAddress(%v) is TRUE on the %s — that address belongs to the FTdx10's chart (%s) and this inventory has taken it on", c.addr, m.name, c.what)
			}
		}
	}

	// The counter-examples, without which every assertion above would be
	// satisfied by an address simply unknown to everybody.
	for _, c := range onlyHere {
		if ftdx10.Dialect().KnownEXAddress(c.addr) {
			t.Errorf("KnownEXAddress(%v) is TRUE on the FTdx10 — this pin's disjointness has been lost in that direction (%s)", c.addr, c.what)
		}
	}
	for _, c := range onlyThere {
		if !ftdx10.Dialect().KnownEXAddress(c.addr) {
			t.Errorf("KnownEXAddress(%v) is false on the FTdx10 — that is this pin's counter-example (%s)", c.addr, c.what)
		}
	}
}

// TestEXAnswerBound proves the EX answer's upper length bound is THIS
// DIALECT'S OWN, derived from its own inventory.
//
// The maximum is recomputed here from each model's own EXItems() — this
// package exports DialectD() and DialectMP(), not a bare Dialect() — rather
// than taken
// from a constant, from the profile, or from core/cat: the bound and the
// datum it is derived from must not come from the same place twice, which is
// the "bound consulted from one place with its datum taken from another"
// failure M9b hit repeatedly. It reaches 12 through exactly one item, MY CALL.
// at (04,01,01) — the chart's only text row, pinned as the only one by
// crosscheck_test.go.
//
// The behavioural half is what matters: the derived width must be what the
// parser actually enforces, one byte either side.
func TestEXAnswerBound(t *testing.T) {
	for _, m := range bothModels() {
		t.Run(m.name, func(t *testing.T) {
			d := m.d

			items := d.EXItems()
			if len(items) == 0 {
				t.Fatal("EXItems() is empty — every assertion below would be vacuous")
			}
			maxDigits := 0
			for _, it := range items {
				if it.Digits > maxDigits {
					maxDigits = it.Digits
				}
			}
			if maxDigits != 12 {
				t.Fatalf("max(Digits) over the %s's %d inventory items is %d, want 12 (MY CALL. at (04,01,01), the chart's one text row)", m.name, len(items), maxDigits)
			}

			// A known address, and the widest legal P4 body for this dialect.
			// The body is returned VERBATIM — no per-item width policy is
			// applied at parse, by core/cat's documented decision — so a
			// 12-byte answer is admissible at any member address, not only at
			// the text row's.
			addr := cat.EXAddress{P1: 4, P2: 1, P3: 1}
			if !d.KnownEXAddress(addr) {
				t.Fatalf("KnownEXAddress(%v) is false — this test needs a member address to answer at", addr)
			}

			body := "GM5DNA......" // 12 bytes
			if len(body) != maxDigits {
				t.Fatalf("the test's P4 body is %d bytes, want %d", len(body), maxDigits)
			}
			frame := []byte("EX" + addr.Wire() + body + ";")
			gotAddr, gotBody, err := d.ParseEXAnswer(frame)
			if err != nil {
				t.Errorf("ParseEXAnswer(%q) = %v — a %d-byte P4 is this dialect's own widest item, so its parser must read one", frame, err, maxDigits)
			} else {
				if gotAddr != addr {
					t.Errorf("ParseEXAnswer(%q) returned address %v, want %v", frame, gotAddr, addr)
				}
				if gotBody != body {
					t.Errorf("ParseEXAnswer(%q) returned P4 %q, want %q verbatim", frame, gotBody, body)
				}
			}

			over := []byte("EX" + addr.Wire() + body + "X" + ";")
			if _, _, err := d.ParseEXAnswer(over); err == nil {
				t.Errorf("ParseEXAnswer(%q) ACCEPTED a %d-byte P4, one past this dialect's widest inventory item — the bound is not deriving from this inventory", over, maxDigits+1)
			}
		})
	}
}

// --- FT-891 Stage 0 (S0.2): the MC send domain ---

// TestMCSelects_MatchesTheMCLegend pins the cat.MCSelectsAll both models
// declare against the legend it is transcribed from.
//
// This manual's MC block prints all four slot classes — "001-099 (Memory
// Channel), P1L -P9U (PMS), 5xx (5MHz BAND), EMG (EMERGENCY CH)" (rev
// 2308-L, layout 1225-1227) — so an MC Set may select every one of them on
// this radio. The FT-891's MC block prints memory and PMS only, which is the
// disagreement the cat.MCSlotPolicy axis exists to carry; without this pin
// the declaration in dialect.go would be a comment claiming more than any
// test holds.
func TestMCSelects_MatchesTheMCLegend(t *testing.T) {
	for _, m := range bothModels() {
		t.Run(m.name, func(t *testing.T) {
			d := m.d

			sixty, err := d.SixtyMSlot(1)
			if err != nil {
				t.Fatalf("SixtyMSlot(1): %v", err)
			}
			emg := d.EMGSlot()
			if emg.Wire() == "" {
				t.Fatal("EMGSlot() is empty — this manual's MC legend names EMG (EMERGENCY CH)")
			}
			mem, err := d.MemorySlot(1)
			if err != nil {
				t.Fatalf("MemorySlot(1): %v", err)
			}
			pms, err := d.PMSSlot(1, false)
			if err != nil {
				t.Fatalf("PMSSlot(1, false): %v", err)
			}

			for _, s := range []cat.Slot{mem, pms, sixty, emg} {
				cmd, err := d.BuildMCSet(s)
				if err != nil {
					t.Errorf("BuildMCSet(%q) = %v — every one of the four classes this manual's MC legend prints must build", s.Wire(), err)
					continue
				}
				if !d.AllowedCommand(cmd.Bytes()) {
					t.Errorf("its own gate refused BuildMCSet(%q)'s frame %q", s.Wire(), cmd.Bytes())
				}
			}
		})
	}
}
