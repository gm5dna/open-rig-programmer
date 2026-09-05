// SPDX-License-Identifier: GPL-3.0-or-later

package ts480_test

import (
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/kw/kwtest"
	"github.com/gm5dna/open-rig-programmer/core/kw/ts480"
	"github.com/gm5dna/open-rig-programmer/core/kw/ts590"
)

// THIS FILE IMPORTS THE SIBLING PACKAGE, AND THAT IS THE OPPOSITE OF
// BORROWING.
//
// The Tier 4b non-borrowing rule forbids one model's DATA standing in for
// another's. Every use of core/kw/ts590 below asserts a DIFFERENCE: a frame
// one row's book prints and the other's does not, refused by the layout that
// did not print it. No value is read from the 590 rows and used as the
// 480's, and nothing in this package's non-test files imports ts590 at all.
//
// It is also the only place the difference CAN be stated. core/kw's own
// fixtures (testlayouts_test.go) are in-package and unreachable from here,
// and a per-radio difference is by construction a fact about two radios: a
// test holding one layout could not express it.

// TestConformance_TS480 runs core/kw's conformance suite over this row.
func TestConformance_TS480(t *testing.T) { kwtest.Run(t, ts480.Layout()) }

// TestLayout_IsConfiguredAndNamed is the vacuity guard the pins below rest
// on.
func TestLayout_IsConfiguredAndNamed(t *testing.T) {
	l := ts480.Layout()
	if !l.Configured() {
		t.Fatal("Layout() is unconfigured — it describes no radio and refuses everything")
	}
	if got := l.Model(); got != "TS-480" {
		t.Errorf("Layout().Model() = %q, want %q", got, "TS-480")
	}
	if l.Book() != kw.Book480 {
		t.Errorf("Layout().Book() = %v, want %v — an \"O;\" quotes this book's own cause sentence (480:143-144, erratum E13)", l.Book(), kw.Book480)
	}
}

// TestLayout_EveryAxisByValue pins all ten axes outright, each against the
// line its own book prints it on. "Ten" is made a fact, not a habit, by
// core/kw/ts590/layout_test.go's TestLayoutConfig_HasExactlyTenComparedAxes.
func TestLayout_EveryAxisByValue(t *testing.T) {
	l := ts480.Layout()

	if got := l.P2Policy(); got != kw.P2FixedZero {
		t.Errorf("byte 4's policy is %v, want %v — \"Always 0 for the TS-480.\" (480:953)", got, kw.P2FixedZero)
	}
	if got := l.Byte19(); got != kw.Byte19Lockout {
		t.Errorf("byte 19's meaning is %v, want %v — \"Lockout status. 0: Lockout OFF, 1: Lockout ON\" (480:962)", got, kw.Byte19Lockout)
	}
	if got := l.Byte28(); got != kw.Byte28FixedZero {
		t.Errorf("byte 28's policy is %v, want %v — \"Always 0 for the TS-480.\" (480:973)", got, kw.Byte28FixedZero)
	}
	if got := l.Byte3940(); got != kw.Byte3940StepIndex {
		t.Errorf("bytes 39-40's meaning is %v, want %v — \"Step size. Refer to the ST command.\" (480:979)", got, kw.Byte3940StepIndex)
	}
	if got := l.Byte41(); got != kw.Byte41FixedZero {
		t.Errorf("byte 41's meaning is %v, want %v — \"Always 0 for the TS-480.\" (480:982)", got, kw.Byte41FixedZero)
	}
	if got := l.ToneModes(); got != kw.ToneModesThree {
		t.Errorf("the tone-mode value set is %v, want %v — \"0: OFF, 1: TONE, 2: CTCSS\" and no cross tone (480:964)", got, kw.ToneModesThree)
	}

	if got := l.MaxEXAddress(); got != 60 {
		t.Errorf("the printed EX menu domain stops at %d, want 60 — \"000 ~ 060: Menu No.\" (480:401), the narrowest of the three registry rows", got)
	}

	wantModes := map[kw.Mode]string{
		kw.ModeLSB: "LSB", kw.ModeUSB: "USB", kw.ModeCW: "CW", kw.ModeFM: "FM",
		kw.ModeAM: "AM", kw.ModeFSK: "FSK", kw.ModeCWR: "CW-R", kw.ModeFSKR: "FSK-R",
	}
	if got := l.ModeNames(); !reflect.DeepEqual(got, wantModes) {
		t.Errorf("the MD legend is %v, want %v (480:843-854)", got, wantModes)
	}

	wantSlots := []kw.SlotRange{{Class: kw.SlotMemory, Lo: 0, Hi: 99}}
	if got := l.Slots(); !reflect.DeepEqual(got, wantSlots) {
		t.Errorf("the slot space is %v, want %v — one flat bank, \"00 ~ 99: Memory channel number\" (480:955), with no bank field in the record (480:827)", got, wantSlots)
	}

	// SIXTEEN of the 47 parameter bytes, in six runs: the three both books
	// print plus this radio's own P2, P11 and P15.
	wantFixed := []kw.FixedField{
		{Pos: 4, Printed: "0"},          // P2  (480:953)
		{Pos: 25, Printed: "000"},       // P10 (480:971)
		{Pos: 28, Printed: "0"},         // P11 (480:973)
		{Pos: 29, Printed: "0"},         // P12 (480:975)
		{Pos: 30, Printed: "000000000"}, // P13 (480:977)
		{Pos: 41, Printed: "0"},         // P15 (480:982)
	}
	if got := l.PrintedFixed(); !reflect.DeepEqual(got, wantFixed) {
		t.Errorf("the hard-wired set is %v, want %v", got, wantFixed)
	}
	n := 0
	for _, ff := range wantFixed {
		n += len(ff.Printed)
	}
	if n != 16 {
		t.Errorf("the hard-wired set covers %d bytes, want 16 of the 47 parameter bytes", n)
	}
}

// TestLayout_TheSiblingsFramesAreRefused is the cross-row difference pin,
// stated in both directions and on the four axes where a frame one book
// prints is a frame the other does not.
//
// EVERY CASE IS A FRAME A REAL RADIO COULD SEND. That is what makes the pin
// worth having: it is not a malformed frame either layout would refuse, it
// is a well-formed one under the OTHER row's reading of the same fifty
// bytes, and a layout that had quietly acquired the sibling's axis would
// admit it.
func TestLayout_TheSiblingsFramesAreRefused(t *testing.T) {
	the480 := ts480.Layout()
	rows590 := []struct {
		name string
		l    kw.Layout
	}{{"TS-590S", ts590.LayoutS()}, {"TS-590SG", ts590.LayoutSG()}}

	for _, tc := range []struct {
		what  string
		frame []byte
		on590 bool // admitted by both 590 rows?
		on480 bool // admitted by the TS-480?
		why   string
	}{
		{
			what:  "byte 28 = '1' (FILTER B)",
			frame: memoryFrame(map[int]string{28: "1"}),
			on590: true, on480: false,
			why: "P11 selects FILTER A or B on the 590 pair (590:1560-1563) and prints \"Always 0 for the TS-480.\" (480:973)",
		},
		{
			what:  "byte 41 = '1' (Channel Lockout ON)",
			frame: memoryFrame(map[int]string{41: "1"}),
			on590: true, on480: false,
			why: "P15 is the lockout on the 590 pair (590:1572-1574) and prints \"Always 0 for the TS-480.\" (480:982); the 480 carries its lockout at byte 19 instead (480:962)",
		},
		{
			what:  "bytes 39-40 = \"05\" (ST step index 5)",
			frame: memoryFrame(map[int]string{39: "05"}),
			on590: false, on480: true,
			why: "P14 is an ST step index on the 480, whose AM/FM legend runs 00 ~ 09 (480:979, 480:1497-1500), and the 590 pair print only \"00\" FM Normal and \"01\" FM Narrow (590:1569-1571)",
		},
		{
			what:  "P7 = '3' (Cross Tone ON)",
			frame: memoryFrame(map[int]string{20: "3"}),
			on590: true, on480: false,
			why: "the 590 pair's P7 legend has a fourth value, \"3: Cross Tone ON\" (590:1553), and the 480's stops at 2 (480:964)",
		},
	} {
		t.Run(tc.what, func(t *testing.T) {
			// The MW SET, through each layout's own outbound gate.
			set := append([]byte{}, tc.frame...)
			set[0], set[1] = 'M', 'W'
			for _, row := range rows590 {
				if got := row.l.AllowedCommand(set); got != tc.on590 {
					t.Errorf("the %s's gate returned %v for an MW with %s, want %v — %s", row.name, got, tc.what, tc.on590, tc.why)
				}
			}
			if got := the480.AllowedCommand(set); got != tc.on480 {
				t.Errorf("the TS-480's gate returned %v for an MW with %s, want %v — %s", got, tc.what, tc.on480, tc.why)
			}

			// And the MR ANSWER, which is the direction a radio drives.
			answer := append([]byte{}, tc.frame...)
			answer[0], answer[1] = 'M', 'R'
			for _, row := range rows590 {
				_, err := row.l.ParseMRAnswer(answer)
				if (err == nil) != tc.on590 {
					t.Errorf("the %s parsed=%v an MR answer with %s, want parsed=%v (%v) — %s", row.name, err == nil, tc.what, tc.on590, err, tc.why)
				}
			}
			_, err := the480.ParseMRAnswer(answer)
			if (err == nil) != tc.on480 {
				t.Errorf("the TS-480 parsed=%v an MR answer with %s, want parsed=%v (%v) — %s", err == nil, tc.what, tc.on480, err, tc.why)
			}
		})
	}
}

// TestRedProof_NoSlotAbove99IsOnThisRow is A-shaped guard on the P2 axis:
// this record has no bank field at all — "Always 0 for the TS-480 (Memory
// bank number)." (480:827) — so the channel number is P3's two digits alone
// and there is no hundreds digit to carry a section or extension channel.
//
// The 590 pair's own 100-109 and 110-119 are the red proof: both are
// resolvable there and neither is here.
func TestRedProof_NoSlotAbove99IsOnThisRow(t *testing.T) {
	l := ts480.Layout()

	if _, err := l.NewSlot(99, kw.ScanHalfNone); err != nil {
		t.Errorf("the TS-480 refused slot 99, the top of its own printed range \"00 ~ 99\" (480:955): %v", err)
	}
	for _, n := range []int{100, 109, 110, 119} {
		if _, err := l.NewSlot(n, kw.ScanHalfNone); err == nil {
			t.Errorf("the TS-480 resolved slot %d; P2 prints \"Always 0\" (480:953) and P3 holds \"00 ~ 99\" (480:955), so there is no digit to carry the hundreds", n)
		}
	}
	// The sibling resolves both, which is what makes the refusals above a
	// property of THIS row rather than of the slot constructor.
	for _, n := range []int{100, 110} {
		if _, err := ts590.LayoutSG().NewSlot(n, half(n)); err != nil {
			t.Errorf("the TS-590SG refused slot %d, which its own book prints (590:1345-1347): %v", n, err)
		}
	}

	// And the gate: the 590 pair's section-channel read is a frame this row
	// must never emit.
	if l.AllowedCommand([]byte("MR0100;")) {
		t.Error("the TS-480's gate ADMITTED \"MR0100;\" — channel 100 is not in its slot space")
	}
	if !ts590.LayoutSG().AllowedCommand([]byte("MR0100;")) {
		t.Error("the TS-590SG's gate refused \"MR0100;\", the printed read of P00's start frequency (590:1449-1451) — the refusal above would then prove nothing")
	}
}

// TestRedProof_EveryHardWiredByteIsRequiredOnParse is decision 7 and A24 at
// the level of bytes: this row hard-wires sixteen positions and a frame
// carrying anything else at one of them is refused, in BOTH directions.
//
// A24 IS WHY THE PARSE SIDE IS A CHOICE RATHER THAN A DEDUCTION. The 480's
// own general note permits a Set to fill an inapplicable parameter with
// "any character except the ASCII control codes (00 to 1Fh) and the
// terminator (;)" (480:108-111), so a radio answering a hard-wired byte with
// something else would not necessarily be faulty. Strictness here is this
// programme's decision and its lift is a dozen real reads of that radio.
func TestRedProof_EveryHardWiredByteIsRequiredOnParse(t *testing.T) {
	l := ts480.Layout()

	good := memoryFrame(nil)
	good[0], good[1] = 'M', 'R'
	if _, err := l.ParseMRAnswer(good); err != nil {
		t.Fatalf("the unmutated record was already refused, so every mutation below would prove nothing: %v", err)
	}

	seen := 0
	for _, ff := range l.PrintedFixed() {
		for off := ff.Pos - 1; off < ff.Pos-1+len(ff.Printed); off++ {
			mutated := append([]byte{}, good...)
			mutated[off] = '1'
			if _, err := l.ParseMRAnswer(mutated); err == nil {
				t.Errorf("position %d is hard-wired in this radio's own chart and an MR answer carrying '1' there was accepted (decision 7, A24)", off+1)
			}
			seen++
			set := append([]byte{}, mutated...)
			set[0], set[1] = 'M', 'W'
			if l.AllowedCommand(set) {
				t.Errorf("the gate ADMITTED an MW carrying '1' at hard-wired position %d", off+1)
			}
		}
	}
	if seen != 16 {
		t.Errorf("%d hard-wired positions were mutated, want 16", seen)
	}
}

// half is ScanLower for a section-defined channel and ScanHalfNone
// otherwise, so the sibling check above can name both kinds in one loop.
func half(n int) kw.ScanHalf {
	if n >= 100 && n <= 109 {
		return kw.ScanLower
	}
	return kw.ScanHalfNone
}

// memoryFrame renders a 50-byte memory frame for channel 003 at 14.250 MHz
// in USB, with every raw byte at the quiet printed value BOTH books admit,
// and then applies override: a map from the book's own 1-indexed position to
// the bytes to write there.
//
// IT IS A TEST-SIDE RENDERER ON PURPOSE. These frames stand in for what a
// RADIO sends, and a helper that called either package's builder could only
// ever produce frames that package already agrees with.
func memoryFrame(override map[int]string) []byte {
	f := make([]byte, 50)
	for i := range f {
		f[i] = '0'
	}
	f[0], f[1] = 'M', 'R'
	copy(f[3:6], "003")          // P2 and P3
	copy(f[6:17], "00014250000") // P4
	f[17] = '2'                  // P5, USB
	copy(f[41:49], "        ")   // P16
	f[49] = ';'
	for pos, s := range override {
		copy(f[pos-1:], s)
	}
	return f
}
