// SPDX-License-Identifier: GPL-3.0-or-later

package kwtest_test

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/kw/kwtest"
)

// This package's own tests drive the suite over layouts built HERE, so that
// the suite is exercised exactly as a per-model package will exercise it:
// from outside core/kw, through the exported API alone.
//
// THE FIXTURES DISAGREE ON EVERY AXIS THE TWO BOOKS DISAGREE ON, which is
// the point. A suite that quietly consulted one radio's reading — the
// hundreds digit most of all — would pass on one fixture and fail on the
// other, and a single fixture could never tell.
//
// They are transcribed here from the two charts rather than imported from
// core/kw's own test file, which is unreachable from outside that package,
// and rather than from core/kw/ts590 and core/kw/ts480, whose layout values
// task 8 mints: a conformance suite whose only witness were the value it is
// meant to check would be circular.

// fixture590SG is the TS-590SG row: three slot classes, a live filter byte,
// four tone modes, and a hundreds digit at byte 4.
func fixture590SG() kw.Layout {
	return kw.MustNewLayout(kw.LayoutConfig{
		Book:      kw.Book590,
		Model:     "KWTEST-590SG",
		P2:        kw.P2HundredsDigit,      // "refer to the MC command" (590:1539-1540)
		Byte19:    kw.Byte19DataMode,       // P6, the data mode (590:1546-1548)
		Byte28:    kw.Byte28FilterLive,     // P11, FILTER A/B (590:1560-1563)
		Byte3940:  kw.Byte3940FMNarrowFlag, // P14 (590:1569-1571)
		Byte41:    kw.Byte41Lockout,        // P15 (590:1572-1574)
		ToneModes: kw.ToneModesFour,        // P7's fourth value is Cross Tone (590:1553)
		ModeNames: modeNames590(),
		Slots: []kw.SlotRange{
			{Class: kw.SlotMemory, Lo: 0, Hi: 99},
			{Class: kw.SlotScan, Lo: 100, Hi: 109},
			{Class: kw.SlotExtension, Lo: 110, Hi: 119},
		},
		PrintedFixed: commonPrintedFixed(),
	})
}

// fixture590S is the TS-590S row: the SG's grid with byte 28 accepted
// either way (590:1478, E7) and the slot ceiling A12 leaves at 109. It is
// here so the suite is held to a per-ROW difference and not only to a
// per-BOOK one.
func fixture590S() kw.Layout {
	return kw.MustNewLayout(kw.LayoutConfig{
		Book:      kw.Book590,
		Model:     "KWTEST-590S",
		P2:        kw.P2HundredsDigit,
		Byte19:    kw.Byte19DataMode,
		Byte28:    kw.Byte28FilterEither,
		Byte3940:  kw.Byte3940FMNarrowFlag,
		Byte41:    kw.Byte41Lockout,
		ToneModes: kw.ToneModesFour,
		ModeNames: modeNames590(),
		Slots: []kw.SlotRange{
			{Class: kw.SlotMemory, Lo: 0, Hi: 99},
			{Class: kw.SlotScan, Lo: 100, Hi: 109},
		},
		PrintedFixed: commonPrintedFixed(),
	})
}

// fixture480 is the TS-480 row: one flat class, three hard-wired bytes the
// 590 pair spend on live fields, three tone modes, and no hundreds digit.
func fixture480() kw.Layout {
	return kw.MustNewLayout(kw.LayoutConfig{
		Book:      kw.Book480,
		Model:     "KWTEST-480",
		P2:        kw.P2FixedZero,       // "Always 0 for the TS-480." (480:953)
		Byte19:    kw.Byte19Lockout,     // P6 is the lockout here (480:962)
		Byte28:    kw.Byte28FixedZero,   // (480:973)
		Byte3940:  kw.Byte3940StepIndex, // P14 refers to ST (480:979)
		Byte41:    kw.Byte41FixedZero,   // (480:982)
		ToneModes: kw.ToneModesThree,    // no cross tone (480:964)
		ModeNames: modeNames480(),
		Slots:     []kw.SlotRange{{Class: kw.SlotMemory, Lo: 0, Hi: 99}},
		PrintedFixed: append(commonPrintedFixed(),
			kw.FixedField{Pos: 4, Printed: "0"},  // P2  (480:953)
			kw.FixedField{Pos: 28, Printed: "0"}, // P11 (480:973)
			kw.FixedField{Pos: 41, Printed: "0"}, // P15 (480:982)
		),
	})
}

// commonPrintedFixed is the hard-wiring both books print identically: P10
// (590:1558-1559, 480:971), P12 (590:1565-1566, 480:975) and P13
// (590:1567-1568, 480:977).
func commonPrintedFixed() []kw.FixedField {
	return []kw.FixedField{
		{Pos: 25, Printed: "000"},
		{Pos: 29, Printed: "0"},
		{Pos: 30, Printed: "000000000"},
	}
}

// modeNames590 is the 590 pair's MD legend (590:1353-1363); nibbles 0 and 8
// name no mode and are absent.
func modeNames590() map[kw.Mode]string {
	return map[kw.Mode]string{
		kw.ModeLSB: "LSB", kw.ModeUSB: "USB", kw.ModeCW: "CW", kw.ModeFM: "FM",
		kw.ModeAM: "AM", kw.ModeFSK: "FSK", kw.ModeCWR: "CW-R", kw.ModeFSKR: "FSK-R",
	}
}

// modeNames480 is the TS-480's MD legend (480:843-854) in this project's own
// spellings; the book prints "CWR" and "FSR" (erratum E12).
func modeNames480() map[kw.Mode]string {
	return map[kw.Mode]string{
		kw.ModeLSB: "LSB", kw.ModeUSB: "USB", kw.ModeCW: "CW", kw.ModeFM: "FM",
		kw.ModeAM: "AM", kw.ModeFSK: "FSK", kw.ModeCWR: "CW-R", kw.ModeFSKR: "FSK-R",
	}
}

func TestRun_OverDisagreeingLayouts(t *testing.T) {
	for _, tc := range []struct {
		name string
		l    kw.Layout
	}{
		{"590SG (three slot classes, live filter byte)", fixture590SG()},
		{"590S (the sibling: byte 28 either way, no extension slots)", fixture590S()},
		{"480 (flat space, three hard-wired bytes, three tone modes)", fixture480()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kwtest.Run(t, tc.l)
		})
	}
}

func TestRunZeroValue(t *testing.T) {
	kwtest.RunZeroValue(t)
}

// TestRunRefusesAnUnconfiguredLayout is the suite's own vacuity-trap proof,
// and the reason kwtest.Run takes an interface rather than a *testing.T.
//
// A model package whose exported layout was never initialised — a failed
// init, a typo selecting the wrong var — reaches Run looking exactly like a
// radio. If Run silently switched to the refusal suite, that package would
// get a GREEN conformance report for a radio it cannot describe. This test
// is what says it does not.
func TestRunRefusesAnUnconfiguredLayout(t *testing.T) {
	rec := &recorder{}
	rec.run(func() { kwtest.Run(rec, kw.Layout{}) })

	if !rec.fatal {
		t.Fatal("kwtest.Run accepted an UNCONFIGURED layout — a model package whose exported var was never initialised would get a green conformance report for a radio it cannot describe")
	}
	if rec.errors != 0 {
		t.Errorf("Run reported %d ordinary failures before refusing — the refusal must be the FIRST thing it does, or a misuse looks like a conformance failure", rec.errors)
	}
}

// TestRun_ATinySlotSpaceStillSatisfiesEveryLeg exercises the branch the
// three fixtures above cannot reach: a slot space SMALLER than the suite's
// own sample size, where sampleNumbers must enumerate the range whole rather
// than spread five picks across it.
//
// A ten-channel layout is well formed, so the suite must report nothing —
// and it must still report the walk it did, which is what its own
// non-vacuity counters check. A sampler that returned an empty slice on a
// short range would fail the class-coverage leg rather than passing here in
// silence.
func TestRun_ATinySlotSpaceStillSatisfiesEveryLeg(t *testing.T) {
	tiny := kw.MustNewLayout(kw.LayoutConfig{
		Book:         kw.Book480,
		Model:        "KWTEST-TINY",
		P2:           kw.P2FixedZero,
		Byte19:       kw.Byte19Lockout,
		Byte28:       kw.Byte28FixedZero,
		Byte3940:     kw.Byte3940StepIndex,
		Byte41:       kw.Byte41FixedZero,
		ToneModes:    kw.ToneModesThree,
		ModeNames:    modeNames480(),
		Slots:        []kw.SlotRange{{Class: kw.SlotMemory, Lo: 0, Hi: 9}},
		PrintedFixed: fixture480().PrintedFixed(),
	})
	rec := &recorder{}
	rec.run(func() { kwtest.Run(rec, tiny) })
	if rec.fatal || rec.errors != 0 {
		t.Errorf("the suite reported %d failures (fatal=%v) on a well-formed ten-channel layout", rec.errors, rec.fatal)
	}
}

// TestRecorderSeesOrdinaryFailures keeps the two tests above honest: the
// recorder must be capable of observing a plain Errorf, or "rec.errors == 0"
// would prove nothing.
func TestRecorderSeesOrdinaryFailures(t *testing.T) {
	rec := &recorder{}
	rec.run(func() { rec.Errorf("boom") })
	if rec.errors != 1 {
		t.Fatalf("the recorder saw %d errors, want 1 — it cannot observe failures, so the proofs above are vacuous themselves", rec.errors)
	}
	if rec.fatal {
		t.Error("the recorder reported a fatal for an ordinary Errorf")
	}
}

// recorder is the smallest thing satisfying kwtest.T. Fatal and Fatalf
// unwind via panic, as *testing.T's do via runtime.Goexit.
type recorder struct {
	fatal  bool
	errors int
}

func (r *recorder) Helper()                           {}
func (r *recorder) Logf(string, ...any)               {}
func (r *recorder) Errorf(format string, args ...any) { r.errors++ }
func (r *recorder) Fatal(args ...any)                 { r.fatal = true; panic(sentinel) }
func (r *recorder) Fatalf(format string, args ...any) { r.fatal = true; panic(sentinel) }

// run calls f, absorbing the recorder's own Fatal panic and re-panicking on
// anything else — so a genuine bug in the suite is not swallowed.
func (r *recorder) run(f func()) {
	defer func() {
		if p := recover(); p != nil && p != sentinel {
			panic(p)
		}
	}()
	f()
}

const sentinel = "kwtest recorder: Fatal"
