// SPDX-License-Identifier: GPL-3.0-or-later

package fakeicr8600

import (
	"bytes"
	"sort"
	"testing"
)

// These tests pin the transcription this package carries — the record geometry,
// the mode table and the address encoding — against the arithmetic the two
// quarantined artefacts state for themselves. They are the check that a typo in
// a table becomes a red test rather than a fake that quietly serves the wrong
// shape.

// TestTheHeadTableIsGaplessAndSumsTo37 is the arithmetic
// IC-R8600-geometry-witness.md states for D1: rows of 10 + 9 + 22 = 41 bytes,
// indices (1)-(41), no gap, no overlap and no repeat — less the four address
// bytes, which the record excludes.
func TestTheHeadTableIsGaplessAndSumsTo37(t *testing.T) {
	next := 1
	total := 0
	for _, f := range headFields {
		if f.First != next {
			t.Errorf("%s starts at record byte %d, want %d — the head is drawn gapless", f.Index, f.First, next)
		}
		if f.Last-f.First+1 != f.Width {
			t.Errorf("%s spans %d..%d but declares width %d", f.Index, f.First, f.Last, f.Width)
		}
		next = f.Last + 1
		total += f.Width
	}
	if total != headRecordLen {
		t.Errorf("the head fields sum to %d bytes, want %d", total, headRecordLen)
	}
	if headRecordLen != 37 {
		t.Errorf("headRecordLen = %d; the witness measures a 41-byte data area less a 4-byte address", headRecordLen)
	}
	if last := headFields[len(headFields)-1].Last; last != 37 {
		t.Errorf("the head ends at record byte %d, want 37", last)
	}
}

// TestTheModeByteSitsAtPrintedIndex11 — the record's (11),(12) pair defers to
// the "Receiving mode" block on PDF p.10, which draws (1) Receiving mode then
// (2) Filter setting, so (11) is the mode and (12) the filter. Data-area
// position 11 is record-only position 7, so the zero-based offset is 6.
func TestTheModeByteSitsAtPrintedIndex11(t *testing.T) {
	if modeByteOffset != 6 {
		t.Errorf("modeByteOffset = %d, want 6 (data-area byte 11, less the 4-byte address, zero-based)", modeByteOffset)
	}
	var found bool
	for _, f := range headFields {
		if f.First == modeByteOffset+1 {
			found = true
			if f.Width != 2 {
				t.Errorf("the field at record byte %d is %d bytes wide; the witness draws (11),(12) as two cells", f.First, f.Width)
			}
		}
	}
	if !found {
		t.Errorf("no head field starts at record byte %d", modeByteOffset+1)
	}
}

// TestEveryTailTableIsGaplessAndSumsToItsMeasuredLength — the per-tail
// arithmetic IC-R8600-geometry-witness.md states: FM 1+3+3=7, P25 1+3=4,
// D-STAR 1+1=2, dPMR 1+2+1+1+3=8, NXDN 1+1+1+3=6, DCR 1+2+1+3=7.
func TestEveryTailTableIsGaplessAndSumsToItsMeasuredLength(t *testing.T) {
	want := map[string]int{"NONE": 0, "D-STAR": 2, "P25": 4, "NXDN": 6, "FM": 7, "DCR": 7, "dPMR": 8}
	if len(layouts) != len(want) {
		t.Fatalf("%d layouts declared, want %d — the document draws six tails and one no-tail class", len(layouts), len(want))
	}
	for _, l := range layouts {
		w, ok := want[l.Class]
		if !ok {
			t.Errorf("layout %q is not one of the seven the document draws", l.Class)
			continue
		}
		total := 0
		next := 42
		for _, f := range l.Tail {
			if f.First != next {
				t.Errorf("%s: %s starts at data-area byte %d, want %d — every tail is drawn gapless from (42)", l.Class, f.Index, f.First, next)
			}
			if f.Last-f.First+1 != f.Width {
				t.Errorf("%s: %s spans %d..%d but declares width %d", l.Class, f.Index, f.First, f.Last, f.Width)
			}
			next = f.Last + 1
			total += f.Width
		}
		if total != w {
			t.Errorf("%s tail sums to %d bytes, want %d", l.Class, total, w)
		}
		if l.RecordLen != headRecordLen+w {
			t.Errorf("%s record length = %d, want %d", l.Class, l.RecordLen, headRecordLen+w)
		}
	}
}

// TestFMAndDCRAreTwoLayoutsAtOneLength — the fact that mints the mode
// discriminator. Both measure 7 tail bytes; FM divides them 1+3+3 and DCR
// 1+2+1+3, and the two must never be one layout.
func TestFMAndDCRAreTwoLayoutsAtOneLength(t *testing.T) {
	fm, ok := layoutForClass("FM")
	if !ok {
		t.Fatal("no FM layout")
	}
	dcr, ok := layoutForClass("DCR")
	if !ok {
		t.Fatal("no DCR layout")
	}
	if fm.RecordLen != dcr.RecordLen {
		t.Fatalf("FM is %d bytes and DCR %d; both diagrams measure 7 tail cells", fm.RecordLen, dcr.RecordLen)
	}
	if len(fm.Tail) == len(dcr.Tail) {
		t.Errorf("FM has %d tail fields and DCR %d; the witness divides the same 7 cells 1+3+3 and 1+2+1+3", len(fm.Tail), len(dcr.Tail))
	}
}

// layoutForClass is a test-local lookup, so the assertions above do not depend
// on whatever index the package keeps for its own use.
func layoutForClass(class string) (layout, bool) {
	for _, l := range layouts {
		if l.Class == class {
			return l, true
		}
	}
	return layout{}, false
}

// TestTheAcceptedRecordLengthSet is the deduplicated ascending set of the seven
// layouts' lengths: {37, 39, 41, 43, 44, 45}, six values from seven layouts
// because FM and DCR collide.
func TestTheAcceptedRecordLengthSet(t *testing.T) {
	seen := map[int]bool{}
	for _, l := range layouts {
		seen[l.RecordLen] = true
	}
	got := make([]int, 0, len(seen))
	for n := range seen {
		got = append(got, n)
	}
	sort.Ints(got)

	want := []int{37, 39, 41, 43, 44, 45}
	if len(got) != len(want) {
		t.Fatalf("accepted record-only lengths %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("accepted record-only lengths %v, want %v", got, want)
		}
	}
}

// TestTheModeTableCarriesTheEighteenPrintedCodesAndNoOthers. PDF p.10's
// "(1) Receiving mode" table prints eighteen codes — 00-08, 11, 14-21 — and
// prints 09, 10, 12 and 13 nowhere.
func TestTheModeTableCarriesTheEighteenPrintedCodesAndNoOthers(t *testing.T) {
	want := map[byte]string{
		0x00: "NONE", 0x01: "NONE", 0x02: "NONE", 0x03: "NONE", 0x04: "NONE",
		0x05: "FM", 0x06: "NONE", 0x07: "NONE", 0x08: "NONE",
		0x11: "NONE", 0x14: "NONE", 0x15: "NONE",
		0x16: "P25", 0x17: "D-STAR", 0x18: "dPMR",
		0x19: "NXDN", 0x20: "NXDN", 0x21: "DCR",
	}
	if len(modeClasses) != len(want) {
		t.Errorf("the mode table has %d codes, want %d", len(modeClasses), len(want))
	}
	for code, class := range want {
		got, ok := modeClasses[code]
		if !ok {
			t.Errorf("mode code %#02X is not in the table; the printed table lists it", code)
			continue
		}
		if got != class {
			t.Errorf("mode code %#02X selects %q, want %q", code, got, class)
		}
	}
	for _, absent := range []byte{0x09, 0x10, 0x12, 0x13, 0x22, 0x0A, 0xFF} {
		if class, ok := modeClasses[absent]; ok {
			t.Errorf("mode code %#02X selects %q; it is printed nowhere and must select nothing", absent, class)
		}
	}
	// Every class the layouts declare must be reachable from at least one wire
	// code, or a declared tail could never be served.
	reachable := map[string]bool{}
	for _, class := range modeClasses {
		reachable[class] = true
	}
	for _, l := range layouts {
		if !reachable[l.Class] {
			t.Errorf("no wire code selects the %s layout", l.Class)
		}
	}
}

// TestNXDNHasTwoWireCodesAndOneLayout — PDF p.14 heads a single diagram "For
// receiving an NXDN signal" while the mode table prints 19 NXDN-VN and
// 20 NXDN-N separately.
func TestNXDNHasTwoWireCodesAndOneLayout(t *testing.T) {
	if modeClasses[0x19] != "NXDN" || modeClasses[0x20] != "NXDN" {
		t.Fatalf("19 selects %q and 20 selects %q; both are NXDN", modeClasses[0x19], modeClasses[0x20])
	}
	n := 0
	for _, l := range layouts {
		if l.Class == "NXDN" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("%d NXDN layouts; the document draws one diagram for both wire codes", n)
	}
}

// TestWFMTakesTheEmptyTail — the note names "FM and Digital", and the tail page
// is headed "For receiving an FM signal". WFM (06) is neither.
func TestWFMTakesTheEmptyTail(t *testing.T) {
	if got := modeClasses[0x06]; got != "NONE" {
		t.Errorf("WFM (06) selects %q, want NONE — the note groups it with the modes that use no tail", got)
	}
}

// ---------------------------------------------------------------------------
// Addressing
// ---------------------------------------------------------------------------

// TestBCD2MatchesThePrintedValueLists. Both address fields print four-decimal-
// digit strings — "0000 ~ 0099" for the group and "0000 ~ 0099", "0000 ~ 0199"
// for the channel — carried in two bytes, and both are 0-based.
func TestBCD2MatchesThePrintedValueLists(t *testing.T) {
	tests := []struct {
		n    int
		want [2]byte
	}{
		{0, [2]byte{0x00, 0x00}},   // group 0000, the first Normal memory group
		{1, [2]byte{0x00, 0x01}},   // channel 0001
		{99, [2]byte{0x00, 0x99}},  // the last Normal memory group and channel
		{100, [2]byte{0x01, 0x00}}, // group 0100, Auto Write Memory
		{101, [2]byte{0x01, 0x01}}, // group 0101, Scan Skip
		{102, [2]byte{0x01, 0x02}}, // group 0102, Programmable Scan Edge
		{199, [2]byte{0x01, 0x99}}, // channel 0199, the last Auto Write channel
	}
	for _, tt := range tests {
		if got := bcd2(tt.n); got != tt.want {
			t.Errorf("bcd2(%d) = % X, want % X", tt.n, got[:], tt.want[:])
		}
	}
}

// TestBCD2PanicsOutsideTheFieldItCanSpell — every caller is a test or a
// consumer's fixture, so a bad address must stop loudly rather than silently
// alias some other channel.
func TestBCD2PanicsOutsideTheFieldItCanSpell(t *testing.T) {
	for _, n := range []int{-1, 10000} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("bcd2(%d) did not panic", n)
				}
			}()
			_ = bcd2(n)
		}()
	}
}

// TestTheAddressIsFourBytesGroupThenChannel — the witness measures (1),(2) as
// the group and (3),(4) as the channel, in that order.
func TestTheAddressIsFourBytesGroupThenChannel(t *testing.T) {
	if addressBytes != 4 {
		t.Fatalf("addressBytes = %d, want 4", addressBytes)
	}
	a := addrOf(1, 23)
	if a.group != [2]byte{0x00, 0x01} || a.channel != [2]byte{0x00, 0x23} {
		t.Errorf("addrOf(1, 23) = %v, want group 00 01 / channel 00 23", a)
	}
}

// ---------------------------------------------------------------------------
// The reassembler
// ---------------------------------------------------------------------------

func TestReassembler(t *testing.T) {
	tests := []struct {
		name   string
		in     []byte
		frames [][]byte
	}{
		{
			"one whole frame",
			[]byte{0xFE, 0xFE, 0x96, 0xE0, 0x19, 0x00, 0xFD},
			[][]byte{{0xFE, 0xFE, 0x96, 0xE0, 0x19, 0x00, 0xFD}},
		},
		{
			"leading noise is discarded",
			[]byte{0x11, 0x22, 0xFE, 0xFE, 0x96, 0xE0, 0x19, 0x00, 0xFD},
			[][]byte{{0xFE, 0xFE, 0x96, 0xE0, 0x19, 0x00, 0xFD}},
		},
		{
			"padding collapses to the canonical two",
			[]byte{0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0x96, 0xE0, 0x19, 0x00, 0xFD},
			[][]byte{{0xFE, 0xFE, 0x96, 0xE0, 0x19, 0x00, 0xFD}},
		},
		{
			"two frames back to back",
			[]byte{0xFE, 0xFE, 0x96, 0xE0, 0x19, 0x00, 0xFD, 0xFE, 0xFE, 0x96, 0xE0, 0x19, 0x00, 0xFD},
			[][]byte{
				{0xFE, 0xFE, 0x96, 0xE0, 0x19, 0x00, 0xFD},
				{0xFE, 0xFE, 0x96, 0xE0, 0x19, 0x00, 0xFD},
			},
		},
		{
			"a preamble mid-body abandons the truncated frame",
			[]byte{0xFE, 0xFE, 0x96, 0xE0, 0xFE, 0xFE, 0x96, 0xE0, 0x19, 0x00, 0xFD},
			[][]byte{{0xFE, 0xFE, 0x96, 0xE0, 0x19, 0x00, 0xFD}},
		},
		{
			"a lone FE starts nothing",
			[]byte{0xFE, 0x11, 0xFE, 0xFE, 0x96, 0xE0, 0x19, 0x00, 0xFD},
			[][]byte{{0xFE, 0xFE, 0x96, 0xE0, 0x19, 0x00, 0xFD}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newReassembler(maxBodyBytes)
			var got [][]byte
			for _, b := range tt.in {
				for _, ev := range a.push([]byte{b}) {
					if ev.overflow {
						t.Fatalf("unexpected overflow")
					}
					got = append(got, ev.frame)
				}
			}
			if len(got) != len(tt.frames) {
				t.Fatalf("got %d frames, want %d (%X)", len(got), len(tt.frames), got)
			}
			for i := range got {
				if !bytes.Equal(got[i], tt.frames[i]) {
					t.Errorf("frame %d = % X, want % X", i, got[i], tt.frames[i])
				}
			}
		})
	}
}

// TestTheOverflowCapIsNotHitByTheLongestFrameThisFakeCanReceive. The longest
// legitimate frame is a dPMR set: to, from, cn, sc, four address bytes and a
// 45-byte record — 53 body bytes.
func TestTheOverflowCapIsNotHitByTheLongestFrameThisFakeCanReceive(t *testing.T) {
	longest := 2 + 2 + addressBytes + 45
	if maxBodyBytes <= longest {
		t.Fatalf("maxBodyBytes = %d, which is not clear of the longest legitimate body, %d", maxBodyBytes, longest)
	}
}

// TestAnOverLengthRunIsOneOverflowEventAndThenResynchronises — it must not
// wedge, and it must not emit an event per byte thereafter.
func TestAnOverLengthRunIsOneOverflowEventAndThenResynchronises(t *testing.T) {
	a := newReassembler(8)
	overflows := 0
	for _, b := range append([]byte{0xFE, 0xFE}, bytes.Repeat([]byte{0x11}, 40)...) {
		for _, ev := range a.push([]byte{b}) {
			if ev.overflow {
				overflows++
			}
		}
	}
	if overflows != 1 {
		t.Fatalf("%d overflow events, want exactly 1", overflows)
	}

	var got [][]byte
	for _, ev := range a.push([]byte{0xFE, 0xFE, 0x96, 0xE0, 0x19, 0x00, 0xFD}) {
		if ev.overflow {
			t.Fatal("a second overflow after resynchronising")
		}
		got = append(got, ev.frame)
	}
	if len(got) != 1 {
		t.Fatalf("after an overflow the reader recovered %d frames, want 1", len(got))
	}
}
