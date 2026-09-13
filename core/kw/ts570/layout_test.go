// SPDX-License-Identifier: GPL-3.0-or-later

package ts570_test

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/kw/kwtest"
	"github.com/gm5dna/open-rig-programmer/core/kw/ts570"
)

// THREE ROWS, TESTED AS THREE RADIOS, NEVER AS ONE — ts590/layout_test.go's
// own rule, applied here. TS-570DG additionally carries no independent
// evidence at all (doc.go): the pins below hold it to being D's config with
// only Model changed, rather than to any document reading of its own.

// TestConformance_TS570D and TestConformance_TS570S run core/kw's shared
// conformance suite (core/kw/kwtest) over the two evidenced rows.
//
// TS-570DG IS DELIBERATELY ABSENT FROM THIS LIST. kwtest.Run's own suite
// already runs over D and S, which share every axis with DG; running it a
// third time over a config that is byte-identical to D's proves nothing new
// and would only double the two known kwtest gaps below.
func TestConformance_TS570D(t *testing.T) { runConformance(t, ts570.LayoutD()) }
func TestConformance_TS570S(t *testing.T) { runConformance(t, ts570.LayoutS()) }

// runConformance is kwtest.Run, with the ONE REMAINING RecordLen=28 gap
// documented rather than silenced.
//
// core/kw's Lift K follow-up (commit e7515d0) and its own follow-up
// (commit 51b61dc, "checkLayoutSelfConsistency/checkIdentity/
// checkEmptyChannel are Book570- and width-aware") together closed every
// gap this comment used to cite: the width assertion and PrintedFixed
// requirement, BuildMWSet's P9-span and isEmptyWindow's width-awareness,
// the Book590/Book480-only switches in checkLayoutSelfConsistency and
// checkIdentity, and checkEmptyChannel's out-of-range panic.
//
// ONE BUG REMAINS, NAMED IN 51b61dc's OWN COMMIT MESSAGE AS "ALREADY-
// DOCUMENTED, NOT-YET-FIXED": checkMemorySets asserts the built MW frame
// is exactly kw.RecordLen (the package CONSTANT, 50) bytes wide rather
// than l.RecordLen(), and its round-trip equality check compares
// Byte28/Byte3940/Byte41/DCSCode/Shift/OffsetHz unconditionally — none of
// which a no-tail record's wire form carries at all, so BuildMWSet
// neither reads nor writes them and a fixture record that sets them
// nonzero can never survive the comparison. Re-running
// kwtest.Run(t, ts570.LayoutD()) directly (a throwaway probe, not
// committed) confirms these are the ONLY failures left — no panic, no
// Book570-specific fault — and every downstream symptom (e.g. "no refusal
// of kind 'another channel's MR answer' was ever SEEN") traces to the same
// root: the round-trip failure `continue`s past the refusal legs below it
// in the sample loop.
//
// Not fixable from core/kw/ts570: checkMemorySets is internal to
// kwtest.go, which this milestone's brief reserves editing core/kw to one
// cited exception (errors.go's Book570/Book870S stream-error entries)
// that does not cover this file. So this test still SKIPS, with the
// current, narrowed citation — see reviews/driver-ts570.md's "## Follow-up"
// for the full account.
func runConformance(t *testing.T, l kw.Layout) {
	t.Helper()
	t.Skip("kwtest.go's checkMemorySets still asserts len(frame) != kw.RecordLen (the package constant, 50) rather than l.RecordLen(), and its round-trip comparison checks Byte28/Byte3940/Byte41/DCSCode/Shift/OffsetHz unconditionally though a no-tail record carries none of them — the one gap 51b61dc's own commit message names as already-documented and not yet fixed; every other kwtest gap for this row (Book570 awareness, the out-of-range panic) is now fixed — see reviews/driver-ts570.md's Follow-up section")
	kwtest.Run(t, l)
}

// TestLayouts_AreConfiguredAndNamedPerRow is the vacuity guard every other
// check in this file rests on.
func TestLayouts_AreConfiguredAndNamedPerRow(t *testing.T) {
	d, s, dg := ts570.LayoutD(), ts570.LayoutS(), ts570.LayoutDG()
	for _, tc := range []struct {
		l    kw.Layout
		want string
	}{
		{d, "TS-570D"}, {s, "TS-570S"}, {dg, "TS-570DG"},
	} {
		if !tc.l.Configured() {
			t.Errorf("%s: unconfigured — it describes no radio and refuses everything", tc.want)
		}
		if got := tc.l.Model(); got != tc.want {
			t.Errorf("Model() = %q, want %q", got, tc.want)
		}
		if tc.l.Model() == "TS-570" {
			t.Errorf("a layout is named %q, which names no one row; no value in this package may say \"a TS-570\"", tc.l.Model())
		}
	}
}

// TestLayoutDG_MirrorsLayoutD holds TS-570DG to doc.go's own claim: it
// carries no independent reading, so every axis, the slot space and the EX
// domain must be byte-identical to LayoutD's, differing only in Model.
func TestLayoutDG_MirrorsLayoutD(t *testing.T) {
	d, dg := ts570.LayoutD(), ts570.LayoutDG()
	if dg.Model() == d.Model() {
		t.Fatalf("LayoutDG().Model() = %q, same as LayoutD()'s — the two must differ in name", dg.Model())
	}
	if dg.RecordLen() != d.RecordLen() ||
		dg.P2Policy() != d.P2Policy() ||
		dg.Byte19() != d.Byte19() ||
		dg.ToneModes() != d.ToneModes() ||
		dg.MaxEXAddress() != d.MaxEXAddress() {
		t.Errorf("LayoutDG()'s axes diverge from LayoutD()'s: RecordLen %d/%d, P2 %v/%v, Byte19 %v/%v, ToneModes %v/%v, MaxEXAddress %d/%d",
			dg.RecordLen(), d.RecordLen(), dg.P2Policy(), d.P2Policy(), dg.Byte19(), d.Byte19(), dg.ToneModes(), d.ToneModes(), dg.MaxEXAddress(), d.MaxEXAddress())
	}
	if len(dg.Slots()) != len(d.Slots()) || dg.Slots()[0] != d.Slots()[0] {
		t.Errorf("LayoutDG()'s slot space diverges from LayoutD()'s: %+v vs %+v", dg.Slots(), d.Slots())
	}
}

// TestLayout_TheTwoAxesThisRowExistsFor pins the two P2Unused/ToneModesTwo
// values matrix §1.2 names, and the reused ones beside them.
func TestLayout_TheTwoAxesThisRowExistsFor(t *testing.T) {
	l := ts570.LayoutD()
	if l.RecordLen() != 28 {
		t.Errorf("RecordLen() = %d, want 28 (matrix §1.2, the 28-byte prefix)", l.RecordLen())
	}
	if l.P2Policy() != kw.P2Unused {
		t.Errorf("P2Policy() = %v, want P2Unused (matrix §1.2: byte 4 carries no meaning at all)", l.P2Policy())
	}
	if l.ToneModes() != kw.ToneModesTwo {
		t.Errorf("ToneModes() = %v, want ToneModesTwo (matrix §1.2: byte 20 is OFF/ON only)", l.ToneModes())
	}
	if l.Byte19() != kw.Byte19Lockout {
		t.Errorf("Byte19() = %v, want Byte19Lockout (matrix §1.2: byte 19 is the channel lockout, the TS-480's reading)", l.Byte19())
	}
	// No tail past P8 at all: NewLayout would have refused a config that
	// set any of these on a 28-byte row, so their zero values here are the
	// axis, not an oversight.
	if l.Byte28() != kw.Byte28Unset || l.Byte3940() != kw.Byte3940Unset || l.Byte41() != kw.Byte41Unset ||
		l.P10Policy() != kw.P10Unset || l.P12Policy() != kw.P12Unset || l.P13Policy() != kw.P13Unset {
		t.Errorf("a tail axis is set on a 28-byte row with no byte past P8: Byte28 %v, Byte3940 %v, Byte41 %v, P10 %v, P12 %v, P13 %v",
			l.Byte28(), l.Byte3940(), l.Byte41(), l.P10Policy(), l.P12Policy(), l.P13Policy())
	}
	if got := len(l.PrintedFixed()); got != 0 {
		t.Errorf("PrintedFixed() has %d entries, want 0 — this row's record has no byte past position 27 to hard-wire", got)
	}
}

// TestLayout_SlotSpaceIsOneFlatMEMBankOfTwoDigitSlots pins the 000-099
// ceiling P2Unused's absent hundreds digit imposes (matrix §2).
func TestLayout_SlotSpaceIsOneFlatMEMBankOfTwoDigitSlots(t *testing.T) {
	l := ts570.LayoutD()
	if _, err := l.NewSlot(0, kw.ScanHalfNone); err != nil {
		t.Errorf("NewSlot(0): %v", err)
	}
	if _, err := l.NewSlot(99, kw.ScanHalfNone); err != nil {
		t.Errorf("NewSlot(99): %v", err)
	}
	if _, err := l.NewSlot(100, kw.ScanHalfNone); err == nil {
		t.Error("NewSlot(100) succeeded; P2Unused carries no hundreds digit, so this row's channel number is P3's two digits alone")
	}
	slots := l.Slots()
	if len(slots) != 1 || slots[0].Class != kw.SlotMemory || slots[0].Lo != 0 || slots[0].Hi != 99 {
		t.Errorf("Slots() = %+v, want one SlotMemory range 0-99", slots)
	}
}

// TestLayout_ModeNamesAreTheFamilysEightUnchanged pins matrix §1.3: zero new
// vocabulary, the family's own eight named modes and nothing else.
func TestLayout_ModeNamesAreTheFamilysEightUnchanged(t *testing.T) {
	want := map[kw.Mode]string{
		kw.ModeLSB: "LSB", kw.ModeUSB: "USB", kw.ModeCW: "CW", kw.ModeFM: "FM",
		kw.ModeAM: "AM", kw.ModeFSK: "FSK", kw.ModeCWR: "CW-R", kw.ModeFSKR: "FSK-R",
	}
	got := ts570.LayoutD().ModeNames()
	if len(got) != len(want) {
		t.Fatalf("ModeNames() has %d entries, want %d", len(got), len(want))
	}
	for m, name := range want {
		if got[m] != name {
			t.Errorf("ModeNames()[%v] = %q, want %q", m, got[m], name)
		}
	}
}

// TestLayout_ManualConformance is the self-check ponytail leaves in place
// of kwtest.Run: build -> parse round trips and gate behaviour for a
// RecordLen=28, no-tail layout, exercised directly rather than through the
// shared harness runConformance documents as unusable here.
func TestLayout_ManualConformance(t *testing.T) {
	l := ts570.LayoutD()

	s, err := l.NewSlot(7, kw.ScanHalfNone)
	if err != nil {
		t.Fatalf("NewSlot(7): %v", err)
	}
	rec := kw.Record{
		Slot: s, FreqHz: 14_250_000, Mode: kw.ModeUSB, Byte19: '0',
		ToneMode: kw.ToneModeTone, ToneIndex: 12, Name: "IGNORED",
	}
	cmd, err := l.BuildMWSet(rec)
	if err != nil {
		t.Fatalf("BuildMWSet: %v", err)
	}
	frame := cmd.Bytes()
	if len(frame) != 28 {
		t.Fatalf("BuildMWSet produced %d bytes, want exactly 28 (matrix §1.2)", len(frame))
	}
	if !l.AllowedCommand(frame) {
		t.Fatalf("the layout's own gate refused its own MW frame %q", frame)
	}

	answer := append([]byte{}, frame...)
	answer[0], answer[1] = 'M', 'R'
	got, err := l.ParseMRAnswer(answer)
	if err != nil {
		t.Fatalf("ParseMRAnswer refused its own MW frame's answer form: %v", err)
	}
	if got.FreqHz != rec.FreqHz || got.Mode != rec.Mode || got.ToneMode != rec.ToneMode || got.ToneIndex != rec.ToneIndex {
		t.Errorf("record did not survive build -> parse: sent %+v, back %+v", rec, got)
	}
	if got.Name != "" {
		t.Errorf("Name = %q, want empty — this row's 28-byte record has no P16 to carry one", got.Name)
	}

	if _, err := l.BuildMWSet(kw.Record{Slot: s, FreqHz: 14_250_000, Mode: kw.ModeUSB, ToneMode: kw.ToneModeCross}); err == nil {
		t.Error("BuildMWSet accepted ToneModeCross, which ToneModesTwo does not admit")
	}

	if _, err := l.BuildIDRead(); err != nil {
		t.Errorf("BuildIDRead: %v", err)
	}
	if _, err := l.BuildAIRead(); err != nil {
		t.Errorf("BuildAIRead: %v", err)
	}
	if _, err := l.BuildMCRead(); err != nil {
		t.Errorf("BuildMCRead: %v", err)
	}
}

// TestLayout_EXDomainIsThe000To051Ceiling pins Format 35's printed domain
// (matrix §2) in both directions.
func TestLayout_EXDomainIsThe000To051Ceiling(t *testing.T) {
	l := ts570.LayoutD()
	if l.MaxEXAddress() != 51 {
		t.Fatalf("MaxEXAddress() = %d, want 51 (Parameter Table Format 35, \"Represented using 000~051\")", l.MaxEXAddress())
	}
	if _, err := l.BuildEXRead(kw.EXAddress{P1: 51}); err != nil {
		t.Errorf("BuildEXRead(51): %v", err)
	}
	if _, err := l.BuildEXRead(kw.EXAddress{P1: 52}); err == nil {
		t.Error("BuildEXRead(52) succeeded; the printed domain stops at 051")
	}
}
