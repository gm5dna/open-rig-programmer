// SPDX-License-Identifier: GPL-3.0-or-later

// Package dialecttest_test drives the conformance suite from a package that
// can see nothing but core/cat's EXPORTED API — exactly the position
// core/cat/ftdx10 occupies since M9c-4 (its dialect_test.go runs this
// suite over the real FTdx10 dialect), and the reason this suite exists at
// all: allTestDialects() is an in-package fixture, unreachable from any
// model package without an import cycle.
//
// An external test package (dialecttest_test, not dialecttest) deliberately:
// a suite whose own tests could reach its unexported helpers could grow a
// dependency on them and still look green, right up until the first real
// caller — which is the failure mode dialectexternal_test.go was written for
// one milestone earlier.
package dialecttest_test

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/cat/dialecttest"
)

// TestRun_FT710 runs the suite over the built-in reference dialect. (The
// other real dialect, core/cat/ftdx10's, runs this suite from its own
// tests — since M9c-4 "the only real dialect" is no longer true.) It is
// the short form's half of the conformance evidence, and it is what makes
// the combined half below meaningful: a suite that passed only on the form
// it was written against would prove nothing about the seam.
func TestRun_FT710(t *testing.T) {
	dialecttest.Run(t, cat.FT710)
}

// combinedRadioConfig is a COMBINED-form dialect described using only the
// exported API, as core/cat/ftdx10 will have to describe the real FTdx10.
//
// This doubles as the milestone's constructibility proof for the new form:
// if MTFormCombined, TagFill or anything else the combined form needs became
// unreachable from outside core/cat, this stops compiling — and it would stop
// compiling here, in this milestone, rather than at M9c-4's first task.
//
// It is a fiction with FT-710-like geometry, not the FT-710 and not the
// FTdx10: the CAT ID, the clarifier policy (5 Hz steps to 9995, where the
// FT-710 steps 10 to 9990) and the MW write kind are all deliberately not
// that radio's, so a suite that had quietly hardcoded any FT-710 value fails
// here. The MW write kind is KindMemTune specifically as the P7 DECOUPLING
// carrier: the combined MT Set's own P7 is a form schema constant, so this
// dialect's MT Sets must still carry '0' while its MW Sets carry '2'.
func combinedRadioConfig() cat.DialectConfig {
	return cat.DialectConfig{
		CATID: "0762",
		ModeNames: map[cat.Mode]string{
			cat.ModeUnset: "-",
			cat.ModeLSB:   "LSB",
			cat.ModeUSB:   "USB",
			cat.ModeCWU:   "CW-U",
			cat.ModeFM:    "FM",
			cat.ModeAM:    "AM",
		},
		Slots: cat.SlotSpace{
			MemoryLo: 1, MemoryHi: 99,
			SixtyLo: 501, SixtyHi: 599,
			PMSPairs:      9,
			EmergencyWire: "EMG",
			NoneWire:      "000",
			MCSelects:     cat.MCSelectsAll,
		},
		EXItems: []cat.EXItem{
			{Addr: cat.EXAddress{P1: 1, P2: 1, P3: 1}, P1Label: "RADIO", P2Label: "GROUP", Name: "ITEM ONE", Digits: 3},
			{Addr: cat.EXAddress{P1: 1, P2: 1, P3: 2}, P1Label: "RADIO", P2Label: "GROUP", Name: "TEXT ITEM", Digits: 16, Text: true},
		},
		EXAddressForm: cat.EXAddressTriple,
		MT:            cat.MTPolicy{Form: cat.MTFormCombined, P11: cat.P11Fixed, ReadSlots: cat.MTReadsReadable, TagMaxBytes: 6, TagFill: ' '},
		Clarifier:     cat.ClarifierPolicy{StepHz: 5, MaxAbsHz: 9995},
		MemoryP5:      cat.P5TxClar,
		MWWriteKind:   cat.KindMemTune,
	}
}

// TestRun_ExternallyBuiltCombinedDialect is the combined form's half.
//
// The three assertions BEFORE Run are not ceremony: a fixture that silently
// came back short-form would turn this test into a second, slower copy of
// TestRun_FT710 while reporting that the combined form was covered.
func TestRun_ExternallyBuiltCombinedDialect(t *testing.T) {
	d := cat.MustNewDialect(combinedRadioConfig())

	if got := d.MTForm(); got != cat.MTFormCombined {
		t.Fatalf("MTForm() = %v, want MTFormCombined — this fixture exists to cover the OTHER form, and a short-form one would make this test a duplicate", got)
	}
	min, max, err := d.MTAnswerBounds()
	if err != nil {
		t.Fatalf("MTAnswerBounds() on a configured combined dialect: %v", err)
	}
	// 29 + TagMaxBytes(6). The number is written out because it is this
	// config's own arithmetic, not a constant borrowed from core/cat.
	if min != 35 || max != 35 {
		t.Fatalf("MTAnswerBounds() = (%d, %d), want (35, 35) — the combined record's length is exact", min, max)
	}

	dialecttest.Run(t, d)
}

// shortPeerRadioConfig is a SHORT-form dialect with a SIX-byte tag field,
// where the FT-710's is twelve.
//
// It is here for one reason, and it is the milestone's own headline defect
// class. Until M9c-3 the short form's answer window was a package constant
// sized to the FT-710's tag, so every check that window could make was
// satisfied by 7-19 whatever dialect it ran on. TestRun_FT710 cannot notice
// that: 7 + 12 IS 19, so a suite driven only by that radio passes just as
// happily against a hardcoded 19 as against a derived one. This fixture's
// window is 7-13, and it is what makes Run's frame-length and over-long
// checks evidence of a DERIVED window rather than of a coincidence.
//
// Verified by mutation while this task was written: replacing
// mtShortAnswerMax's body with a literal 19 leaves TestRun_FT710 green and
// fails this test.
func shortPeerRadioConfig() cat.DialectConfig {
	return cat.DialectConfig{
		CATID: "0763",
		ModeNames: map[cat.Mode]string{
			cat.ModeUnset: "-",
			cat.ModeLSB:   "LSB",
			cat.ModeUSB:   "USB",
			cat.ModeFM:    "FM",
		},
		Slots: cat.SlotSpace{
			MemoryLo: 1, MemoryHi: 20,
			SixtyLo: 0, SixtyHi: 0,
			PMSPairs:      2,
			EmergencyWire: "",
			NoneWire:      "000",
			MCSelects:     cat.MCSelectsAll,
		},
		EXItems: []cat.EXItem{
			{Addr: cat.EXAddress{P1: 2, P2: 3, P3: 4}, P1Label: "RADIO", P2Label: "GROUP", Name: "ITEM", Digits: 2},
		},
		EXAddressForm: cat.EXAddressTriple,
		MT:            cat.MTPolicy{Form: cat.MTFormShort, ReadSlots: cat.MTReadsReadable, TagMaxBytes: 6, ClearTagByte: ' ', PadByte: ' '},
		Clarifier:     cat.ClarifierPolicy{StepHz: 1, MaxAbsHz: 9999},
		MemoryP5:      cat.P5TxClar,
		MWWriteKind:   cat.KindMemory,
	}
}

// TestRun_ExternallyBuiltShortPeerDialect runs the suite over that narrower
// window. See shortPeerRadioConfig for why a second SHORT-form dialect earns
// its place.
func TestRun_ExternallyBuiltShortPeerDialect(t *testing.T) {
	d := cat.MustNewDialect(shortPeerRadioConfig())

	min, max, err := d.MTAnswerBounds()
	if err != nil {
		t.Fatalf("MTAnswerBounds() on a configured short-form dialect: %v", err)
	}
	// 7 + TagMaxBytes(6). Written out for the same reason as the combined
	// fixture's 35: it is this config's own arithmetic, and if it ever read
	// 19 the fixture has stopped being the FT-710's opposite number.
	if min != 7 || max != 13 {
		t.Fatalf("MTAnswerBounds() = (%d, %d), want (7, 13) — this fixture exists to have a window the FT-710's constant would not fit", min, max)
	}

	dialecttest.Run(t, d)
}

// TestRunZeroValue_RefusesEverything is the suite's other half: the zero
// Dialect, which no config can produce and every uninitialised package-level
// var is.
//
// It is a SEPARATE exported entry point rather than something Run detects,
// and dialecttest.go's own doc comment records why at length: Run(zero)
// quietly becoming a refusal suite is precisely how a model package with an
// uninitialised dialect would get a green conformance report.
func TestRunZeroValue_RefusesEverything(t *testing.T) {
	dialecttest.RunZeroValue(t)
}

// pairAddressRadioConfig is a dialect whose EX address field is FOUR digits,
// where every other fixture here and every registered radio's is six.
//
// It is here for the same reason shortPeerRadioConfig is, one seam along.
// Until the FT-891 Stage 0 change the EX read's length was the package
// constant exReadLen = 9 and the answer's address slice was frame[2:8], both
// consulted through a Dialect receiver — so every check Run could make about
// an EX frame was satisfied by a six-digit answer whatever dialect it ran
// on. TestRun_FT710 cannot notice that: the FT-710's width IS six.
//
// Every member's P3 is 0, which cat's rule V12 requires of an
// EXAddressPair inventory: the four-digit render carries P1 and P2 only.
func pairAddressRadioConfig() cat.DialectConfig {
	return cat.DialectConfig{
		CATID: "0764",
		ModeNames: map[cat.Mode]string{
			cat.ModeUnset: "-",
			cat.ModeLSB:   "LSB",
			cat.ModeUSB:   "USB",
		},
		Slots: cat.SlotSpace{
			MemoryLo: 1, MemoryHi: 20,
			SixtyLo: 0, SixtyHi: 0,
			PMSPairs:      2,
			EmergencyWire: "",
			NoneWire:      "000",
			MCSelects:     cat.MCSelectsAll,
		},
		EXItems: []cat.EXItem{
			{Addr: cat.EXAddress{P1: 8, P2: 1, P3: 0}, P1Label: "RADIO", P2Label: "GROUP", Name: "ITEM ONE", Digits: 2},
			{Addr: cat.EXAddress{P1: 8, P2: 3, P3: 0}, P1Label: "RADIO", P2Label: "GROUP", Name: "ITEM TWO", Digits: 5},
		},
		EXAddressForm: cat.EXAddressPair,
		MT:            cat.MTPolicy{Form: cat.MTFormShort, ReadSlots: cat.MTReadsReadable, TagMaxBytes: 8, ClearTagByte: ' ', PadByte: ' '},
		Clarifier:     cat.ClarifierPolicy{StepHz: 10, MaxAbsHz: 9990},
		MemoryP5:      cat.P5TxClar,
		MWWriteKind:   cat.KindMemory,
	}
}

// TestRun_ExternallyBuiltPairAddressDialect runs the whole suite over the
// four-digit form.
//
// The two assertions before Run are the fixture's own guard, in the shape
// the two fixtures above use: a fixture that silently came back six-digit
// would make this a slower copy of TestRun_ExternallyBuiltShortPeerDialect
// while reporting that the narrow address form was covered.
func TestRun_ExternallyBuiltPairAddressDialect(t *testing.T) {
	d := cat.MustNewDialect(pairAddressRadioConfig())

	if got := d.EXAddressWidth(); got != 4 {
		t.Fatalf("EXAddressWidth() = %d, want 4 — this fixture exists to cover the OTHER address form", got)
	}
	// This config's own arithmetic, written out rather than derived: "EX" +
	// a 4-byte address + ";".
	if got, want := len(d.EXWire(d.EXAddresses()[0])), 4; got != want {
		t.Fatalf("EXWire is %d bytes, want %d", got, want)
	}

	dialecttest.Run(t, d)
}

// p5FixedRadioConfig is a SHORT-form dialect declaring cat.P5Fixed: byte 21
// of the shared memory block is printed "(Fixed)" on its manual, not the TX
// clarifier flag.
//
// It is here for the reason shortPeerRadioConfig is: an axis exercised only
// by dialects that all take the same value of it is not exercised at all.
// Every other fixture in this file and every registered radio declares
// P5TxClar, so without this one the P5Fixed arm of dialecttest.checkMemoryP5
// — the refusals, which are the half that keeps a printed-fixed byte off the
// wire — would never run from outside core/cat.
func p5FixedRadioConfig() cat.DialectConfig {
	return cat.DialectConfig{
		CATID: "0764",
		ModeNames: map[cat.Mode]string{
			cat.ModeUnset: "-",
			cat.ModeLSB:   "LSB",
			cat.ModeUSB:   "USB",
		},
		Slots: cat.SlotSpace{
			MemoryLo: 1, MemoryHi: 30,
			SixtyLo: 0, SixtyHi: 0,
			PMSPairs:      3,
			EmergencyWire: "",
			NoneWire:      "000",
			MCSelects:     cat.MCSelectsAll,
		},
		EXItems: []cat.EXItem{
			{Addr: cat.EXAddress{P1: 5, P2: 2, P3: 1}, P1Label: "RADIO", P2Label: "GROUP", Name: "ITEM", Digits: 2},
		},
		EXAddressForm: cat.EXAddressTriple,
		MT: cat.MTPolicy{
			Form: cat.MTFormShort, ReadSlots: cat.MTReadsReadable,
			TagMaxBytes: 8, ClearTagByte: ' ', PadByte: ' ',
		},
		Clarifier:   cat.ClarifierPolicy{StepHz: 20, MaxAbsHz: 9980},
		MemoryP5:    cat.P5Fixed,
		MWWriteKind: cat.KindMemory,
	}
}

// TestRun_ExternallyBuiltP5FixedDialect runs the suite over that dialect.
//
// The assertion before Run is not ceremony, for shortPeerRadioConfig's
// reason: a fixture that silently came back P5TxClar would make this a
// slower copy of TestRun_ExternallyBuiltShortPeerDialect while reporting
// that the fixed reading was covered.
func TestRun_ExternallyBuiltP5FixedDialect(t *testing.T) {
	d := cat.MustNewDialect(p5FixedRadioConfig())

	if got := d.MemoryP5(); got != cat.P5Fixed {
		t.Fatalf("MemoryP5() = %v, want P5Fixed — this fixture exists to cover the OTHER reading of byte 21", got)
	}

	dialecttest.Run(t, d)
}

// tagDisplayRadioConfig is a COMBINED-form dialect whose P11 is a live TAG
// ON/OFF flag rather than the printed "0: (Fixed)" byte 28 carries on every
// registered combined-form radio.
//
// It is here for the reason p5FixedRadioConfig is: without it the
// P11TagDisplay arm of dialecttest — the display-bearing builder and parser,
// and the two refusals that stop a live flag being defaulted — would never
// run from outside core/cat, and a model package declaring that policy would
// arrive with no conformance evidence for the half of the API it actually
// uses.
//
// Its geometry disagrees with combinedRadioConfig's on both dimensions the
// combined record derives from its receiver — a 9-byte tag field filled with
// '.', against 6 and ' ' — so a length or fill byte hardwired to the other
// fixture's is refused here too. Its EX address form is the narrow one as
// well (see the test below): this is the one fixture that carries every
// Stage 0 axis at its FT-891 reading.
func tagDisplayRadioConfig() cat.DialectConfig {
	return cat.DialectConfig{
		CATID: "0765",
		ModeNames: map[cat.Mode]string{
			cat.ModeUnset: "-",
			cat.ModeLSB:   "LSB",
			cat.ModeUSB:   "USB",
			cat.ModeCWU:   "CW-U",
		},
		Slots: cat.SlotSpace{
			MemoryLo: 1, MemoryHi: 40,
			SixtyLo: 0, SixtyHi: 0,
			PMSPairs:      5,
			EmergencyWire: "",
			NoneWire:      "000",
			MCSelects:     cat.MCSelectsMemoryPMS,
		},
		EXItems: []cat.EXItem{
			// P3 is 0 because cat's V12 requires it of every member of an
			// EXAddressPair inventory: the four-digit render carries P1 and
			// P2 only, so a non-zero P3 would be dropped from every frame.
			{Addr: cat.EXAddress{P1: 8, P2: 1, P3: 0}, P1Label: "RADIO", P2Label: "GROUP", Name: "ITEM", Digits: 4},
		},
		EXAddressForm: cat.EXAddressPair,
		MT: cat.MTPolicy{
			Form: cat.MTFormCombined, ReadSlots: cat.MTReadsMemoryPMS,
			P11:         cat.P11TagDisplay,
			TagMaxBytes: 9,
			TagFill:     '.',
		},
		Clarifier:   cat.ClarifierPolicy{StepHz: 10, MaxAbsHz: 9990},
		MemoryP5:    cat.P5Fixed,
		MWWriteKind: cat.KindMemory,
	}
}

// TestRun_ExternallyBuiltTagDisplayDialect runs the suite over that dialect.
//
// It is also this milestone's constructibility proof for the narrow reading
// of all five Stage 0 axes at once: EXAddressPair, MCSelectsMemoryPMS,
// MTReadsMemoryPMS, P5Fixed and P11TagDisplay together are the FT-891's
// declared shape, and if any of them became unreachable from outside
// core/cat this stops compiling here rather than at Stage 1's first task.
func TestRun_ExternallyBuiltTagDisplayDialect(t *testing.T) {
	d := cat.MustNewDialect(tagDisplayRadioConfig())

	if got := d.MTP11(); got != cat.P11TagDisplay {
		t.Fatalf("MTP11() = %v, want P11TagDisplay — this fixture exists to cover the OTHER reading of byte 28", got)
	}
	if got := d.MTForm(); got != cat.MTFormCombined {
		t.Fatalf("MTForm() = %v, want MTFormCombined", got)
	}

	dialecttest.Run(t, d)
}
