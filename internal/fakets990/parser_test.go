// SPDX-License-Identifier: GPL-3.0-or-later

package fakets990

import (
	"strings"
	"testing"
)

// fields is a test-local assembler for the MA0 grid, INDEPENDENT of this
// package's own builder: it names each parameter as the chart names it and
// concatenates the cells in the chart's own order (990:2893-2915 Set,
// 990:2919-2938 Answer, which number the same positions the same way).
//
// A builder with a transposed field would still pass a test whose expectation
// came from that builder. It cannot pass one whose expectation is assembled
// here — which is the point on a grid carrying EIGHTEEN parameters and FOUR
// two-digit tone windows.
type fields struct {
	slot      string // P1, positions 4-6
	class     string // P2, position 7
	freq      string // P3, positions 8-18
	mode      string // P4, position 19
	fmNarrow  string // P5, position 20
	toneType  string // P6, position 21
	toneNo    string // P7, positions 22-23
	ctcssNo   string // P8, positions 24-25
	freq2     string // P9, positions 26-36
	mode2     string // P10, position 37
	fmNarrow2 string // P11, position 38
	toneType2 string // P12, position 39
	toneNo2   string // P13, positions 40-41
	ctcssNo2  string // P14, positions 42-43
	split     string // P15, position 44
	dualRX    string // P16, position 45
	lockout   string // P17, position 46
	name      string // P18, positions 47-56, a FIXED ten-byte window
}

// frame renders the fields as a complete MA0 frame — the same bytes on the Set
// and Answer directions, which is what the two identical grids print, and
// FIXED at 57 bytes, which is what this radio's own terminator position says.
func (f fields) frame() string {
	return "MA0" + f.slot + f.class + f.freq + f.mode + f.fmNarrow + f.toneType +
		f.toneNo + f.ctcssNo + f.freq2 + f.mode2 + f.fmNarrow2 + f.toneType2 +
		f.toneNo2 + f.ctcssNo2 + f.split + f.dualRX + f.lockout + f.name + ";"
}

// plainFields is one unremarkable populated record: the front matter's worked
// frequency (990:86), FM, and the printed FIRST value of every other legend in
// the grid. Frequency 2 is all zeroes, which the book itself prints for a
// single memory channel (990:2964-2965), and P17's OFF value is "1" and not
// "0" — this radio spends that datum on 1/2 (990:2952-2954, erratum E8).
func plainFields(slot string) fields {
	return fields{
		slot:      slot,
		class:     "0", // "0: Single Memory channel" (990:2898)
		freq:      "00007000000",
		mode:      "4", // "4: FM" (990:3711)
		fmNarrow:  "0", // "0: FM Wide for frequency 1" (990:2913)
		toneType:  "0", // "0: FM Tone function OFF for frequency 1" (990:2916)
		toneNo:    "00",
		ctcssNo:   "00",
		freq2:     "00000000000",
		mode2:     "0",
		fmNarrow2: "0",
		toneType2: "0",
		toneNo2:   "00",
		ctcssNo2:  "00",
		split:     "0",                     // "0: Simplex" (990:2947)
		dualRX:    "0",                     // "0: Dual reception OFF" (990:2950)
		lockout:   "1",                     // "1: Scan Lockout OFF" (990:2953)
		name:      strings.Repeat(" ", 10), // the fixed window, blank
	}
}

// blankFrame is the 57-byte answer a blank channel gives: "MA0", the slot, and
// P2 to P18 blank (990:2962-2963) with "blank" read as ASCII space (A6, on
// this book's own QR definition "this setting is blank <0x20>", 990:4081-4082).
// The 50 is counted here off the chart's own ruler: positions 7 to 56
// inclusive.
func blankFrame(slot string) string {
	return "MA0" + slot + strings.Repeat(" ", 50) + ";"
}

// --- The frame geometry itself ---

// TestMA0FrameIsFiftySevenBytes re-derives the width from the chart's ruler
// rather than taking it from this package: the terminator is nailed to
// position 57 (990:2915 Set, 990:2938 Answer), where the 890S's floats after a
// name of 0 to 10 characters.
//
// THE BLANK FRAME AND A POPULATED ONE ARE THE SAME LENGTH, and on this radio
// that is printed rather than assumed: the name window is a fixed ten bytes
// (990:2955-2956) whether or not it carries a name, so nothing here needs the
// 890S's A17.
func TestMA0FrameIsFiftySevenBytes(t *testing.T) {
	if got := len(blankFrame("000")); got != 57 {
		t.Fatalf("the test assembler builds a %d-byte blank frame, want 57 (990:2915)", got)
	}
	if got := len(plainFields("000").frame()); got != 57 {
		t.Errorf("a populated record is %d bytes, want 57", got)
	}
	if got := len(fieldsWithName("000", "0123456789").frame()); got != 57 {
		t.Errorf("a record with a full ten-character name is %d bytes, want 57 — the window is fixed and the terminator does not float", got)
	}
}

// TestGridOffsetsTileTheFrame re-states the Set and Answer rulers
// (990:2893-2915, 990:2919-2938) as the chart's own 1-INDEXED positions and
// checks that this package's offsets are those positions less one, in order,
// with no gap and no overlap up to the terminator.
//
// It is the pin a transposition would fail. Eighteen parameters, four of them
// two-digit windows and two of them eleven, is enough field map for a
// copy-and-adjust error to land inside a neighbouring parameter and still
// produce a well-formed 57-byte frame — which every round-trip test in this
// file would then agree with, because both directions would be wrong in the
// same place.
func TestGridOffsetsTileTheFrame(t *testing.T) {
	ruler := []struct {
		name     string
		position int // the chart's own 1-indexed first cell
		width    int
		off      int // this package's constant
	}{
		{"P1 channel number", 4, 3, recSlotOff},
		{"P2 memory channel type", 7, 1, recClassOff},
		{"P3 frequency 1", 8, 11, recFreqOff},
		{"P4 mode 1", 19, 1, recModeOff},
		{"P5 FM wide/narrow 1", 20, 1, recFMNarrowOff},
		{"P6 tone function 1", 21, 1, recToneTypeOff},
		{"P7 tone frequency 1", 22, 2, recToneNoOff},
		{"P8 CTCSS frequency 1", 24, 2, recCTCSSNoOff},
		{"P9 frequency 2", 26, 11, recFreq2Off},
		{"P10 mode 2", 37, 1, recMode2Off},
		{"P11 FM wide/narrow 2", 38, 1, recFMNarrow2Off},
		{"P12 tone function 2", 39, 1, recToneType2Off},
		{"P13 tone frequency 2", 40, 2, recToneNo2Off},
		{"P14 CTCSS frequency 2", 42, 2, recCTCSSNo2Off},
		{"P15 simplex/split", 44, 1, recSplitOff},
		{"P16 dual reception", 45, 1, recDualRXOff},
		{"P17 scan lockout", 46, 1, recLockoutOff},
		{"P18 channel name", 47, 10, recNameOff},
	}
	next := 4 // position 4: "MA0" occupies 1 to 3
	for _, f := range ruler {
		if f.position != next {
			t.Errorf("%s starts at position %d, and the previous field ends at %d — the ruler has a gap or an overlap", f.name, f.position, next-1)
		}
		if f.off != f.position-1 {
			t.Errorf("%s: this package's offset is %d, want %d (position %d, 0-indexed)", f.name, f.off, f.position-1, f.position)
		}
		next = f.position + f.width
	}
	if next != recLen {
		t.Errorf("the fields end at position %d and the terminator is at %d — the grid is drawn to 57 (990:2915)", next-1, recLen)
	}
}

// fieldsWithName sets the name into the FIXED ten-byte window, padding it with
// ASCII spaces to that width. THE PADDING IS THE TEST'S, not the fake's: this
// package neither pads nor trims (doc.go's register entry THE NAME WINDOW IS
// TEN BYTES, CARRIED VERBATIM), so a test that wants a short name in a fixed
// window has to say what the other bytes are.
func fieldsWithName(slot, name string) fields {
	f := plainFields(slot)
	f.name = name + strings.Repeat(" ", maxNameLen-len(name))
	return f
}

// --- MA0 read (990:2916-2918) ---

// TestMA0Read_AnswersThePopulatedRecordVerbatim over the default image's own
// channels, with the expectation assembled here.
func TestMA0Read_AnswersThePopulatedRecordVerbatim(t *testing.T) {
	r, conn := newTestRadio(t)

	got := exchange(t, conn, "MA0000;")
	if want := plainFields("000").frame(); got != want {
		t.Errorf("MA0000; -> %q, want %q", got, want)
	}

	// The record the fake holds and the bytes it answered must agree.
	s, ok := r.ChannelState(0)
	if !ok {
		t.Fatal("ChannelState(0) reports no record, but the default image populates it")
	}
	if s.Freq != "00007000000" {
		t.Errorf("ChannelState(0).Freq = %q, want the front matter's worked frequency (990:86)", s.Freq)
	}
}

// TestMA0Read_AnAbsentChannelAnswersTheBlankFrame, never a rejection. The book
// prints the blank channel as a normal, answerable state — "When reading a
// blank channel, parameters P2 to P18 becomes blank." (990:2962-2963) — which
// is what core/driver/ts990 must read as "empty channel" and not as "parse
// error".
func TestMA0Read_AnAbsentChannelAnswersTheBlankFrame(t *testing.T) {
	_, conn := newTestRadio(t)
	for _, slot := range []string{"050", "099"} {
		if got, want := exchange(t, conn, "MA0"+slot+";"), blankFrame(slot); got != want {
			t.Errorf("MA0%s; -> %q, want %q", slot, got, want)
		}
	}
}

// TestMA0Read_TheBlankNoteCoversTheNameWindowToo, which is where this row and
// the 890S part company: that book's note stops at P12 and leaves the name
// window unspecified (erratum E4, the design's A4 and A21), while this one
// covers "P2 to P18" — the name window included. So this fake needs ONE blank
// fixture where internal/fakets890 needs two, and the residue case has no
// counterpart here to invent.
func TestMA0Read_TheBlankNoteCoversTheNameWindowToo(t *testing.T) {
	_, conn := newTestRadio(t)
	got := exchange(t, conn, "MA0050;")
	if want := strings.Repeat(" ", maxNameLen); got[recNameOff:recNameOff+maxNameLen] != want {
		t.Errorf("the blank answer's P18 window is %q, want %q (990:2962-2963)", got[recNameOff:recNameOff+maxNameLen], want)
	}
}

// TestMA0Read_TheAnswersSlotIsThreeZeroPaddedDigits — doc.go's register entry
// THE ANSWER'S CHANNEL NUMBER IS THREE ZERO-PADDED DIGITS, which is the
// design's A5. The failure direction is safe and is the reason the entry is
// cheap: a space-padded answer MISSES the codec's prefix matcher and times
// out; it cannot be mis-attributed to another channel.
func TestMA0Read_TheAnswersSlotIsThreeZeroPaddedDigits(t *testing.T) {
	_, conn := newTestRadio(t)
	for _, slot := range []string{"000", "007", "099"} {
		got := exchange(t, conn, "MA0"+slot+";")
		if !strings.HasPrefix(got, "MA0"+slot) {
			t.Errorf("MA0%s; -> %q, want a frame whose bytes 4-6 are %q", slot, got, slot)
		}
	}
}

// TestMA0Read_RefusesEveryMalformedRequest. The Read form is seven positions,
// "M A 0 P1 P1 P1 ;" (990:2916-2918), and P1 is three ASCII digits over
// "000 ~ 119" (990:2894). Nothing here normalises — doc.go's register entry
// SET-DIRECTION FIELD STRICTNESS.
func TestMA0Read_RefusesEveryMalformedRequest(t *testing.T) {
	_, conn := newTestRadio(t)
	for _, send := range []string{
		"MA00;",    // too short
		"MA00000;", // too long for a read, too short for a Set
		"MA0 00;",  // a space in the hundreds digit: this book prints no space form
		"MA0abc;",
		"MA0-01;",
	} {
		assertRejected(t, conn, send)
	}
}

// TestMA0_SlotsAbove099AreNotServed — doc.go's register entry SLOTS 100-119
// ARE NOT SERVED. The chart prints the domain "000 ~ 119" (990:2894) and maps
// 100-109 to P0-P9 and 110-119 to E0-E9 (990:2895-2896); nothing anywhere in
// the book says what either class holds or what a read of one answers (the
// design's A9 and A10, and E18 records that E0-E9 are never explained at all).
// The slots are published in no bank of this row, so there is nothing for this
// fake to represent, and the narrowing is deliberate rather than a claim that
// a TS-990S refuses those numbers.
func TestMA0_SlotsAbove099AreNotServed(t *testing.T) {
	_, conn := newTestRadio(t)
	for _, slot := range []string{"100", "109", "110", "119", "120", "999"} {
		assertRejected(t, conn, "MA0"+slot+";")
	}
}

// --- MA0 Set (990:2893-2915) ---

// TestMA0Set_IsFireAndForgetAndStores — doc.go's register entry AN ACCEPTED
// SET PRODUCES NO REPLY (the design's A20: MA0's own block is silent about an
// acknowledgement where MA1, MA2, MA3 and MA6 each print one).
func TestMA0Set_IsFireAndForgetAndStores(t *testing.T) {
	_, conn := newTestRadio(t)
	f := fieldsWithName("007", "TEST")
	f.freq = "00014175000" // the AS2 block's worked frequency (990:344-345)
	writeFrame(t, conn, f.frame())
	assertNoReply(t, conn)

	if got, want := exchange(t, conn, "MA0007;"), f.frame(); got != want {
		t.Errorf("after the Set, MA0007; -> %q, want %q", got, want)
	}
}

// TestMA0Set_P2IsIgnoredAndTheClassComesFromFrequency2 — doc.go's register
// entry THE SET'S P2 IS IGNORED AND THE ANSWER'S CLASS FOLLOWS FREQUENCY 2.
// The chart prints the byte as a dummy the radio discards — "The memory channel
// type is decided while setting the P9 and P10 values, so this parameter is
// ignored. Enter a dummy value." (990:2901-2903), the design's A14 — so a fake
// that stored what it was sent would answer a class the radio would not.
func TestMA0Set_P2IsIgnoredAndTheClassComesFromFrequency2(t *testing.T) {
	for _, tt := range []struct {
		name  string
		sent  string
		freq2 string
		want  string
	}{
		{"a dummy 9 on a channel with no frequency 2", "9", "00000000000", "0"},
		{"a dummy 0 on a channel with a live frequency 2", "0", "00014195000", "1"},
		{"a dummy 2 on a channel with no frequency 2", "2", "00000000000", "0"},
		{"a dummy 2 on a channel with a live frequency 2", "2", "00014195000", "1"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r, conn := newTestRadio(t)
			f := fieldsWithName("017", "CLASS")
			f.class = tt.sent
			f.freq2 = tt.freq2
			writeFrame(t, conn, f.frame())
			assertNoReply(t, conn)

			s, ok := r.ChannelState(17)
			if !ok {
				t.Fatal("the Set stored nothing")
			}
			if string(s.Class) != tt.want {
				t.Errorf("stored Class = %q, want %q — P2 is a dummy the radio ignores (990:2901-2903)", string(s.Class), tt.want)
			}
			want := f
			want.class = tt.want
			if got := exchange(t, conn, "MA0017;"); got != want.frame() {
				t.Errorf("read back %q, want %q", got, want.frame())
			}
		})
	}
}

// TestMA0Set_ToABlankChannelIsStored — doc.go's register entry A SET TO A
// BLANK CHANNEL IS STORED. Whether MA0 alone can create a channel is the
// design's A3, unlifted, and core/driver/ts990 REFUSES such a write on that
// ground. A fake that refused it here would assert A3 as a fact about the
// radio AND would put the driver's own refusal out of reach of a real fake.
func TestMA0Set_ToABlankChannelIsStored(t *testing.T) {
	r, conn := newTestRadio(t)
	if _, ok := r.ChannelState(60); ok {
		t.Fatal("channel 060 is populated by the default image — this test needs a blank one")
	}
	f := fieldsWithName("060", "NEW")
	writeFrame(t, conn, f.frame())
	assertNoReply(t, conn)
	if got, want := exchange(t, conn, "MA0060;"), f.frame(); got != want {
		t.Errorf("after a Set to a blank channel, MA0060; -> %q, want %q", got, want)
	}
}

// TestMA0Set_TheNameWindowIsCarriedVerbatim, its trailing spaces included.
//
// THE FAKE NEITHER PADS NOR TRIMS, and that is the point rather than an
// omission: P18 is a FIXED ten-byte window (990:2955-2956) whose Set direction
// therefore always carries ten bytes, and the design's A1 — space padding on
// write, trailing spaces not part of the name on read — is the CODEC's and the
// DRIVER's rule. A fake that applied A1 too would apply it on both sides of
// every round trip and could never contradict it.
func TestMA0Set_TheNameWindowIsCarriedVerbatim(t *testing.T) {
	for _, name := range []string{"          ", "A         ", "AB        ", " AB       ", "0123456789", "  SPACED  "} {
		t.Run(strings.ReplaceAll(name, " ", "_"), func(t *testing.T) {
			r, conn := newTestRadio(t)
			f := plainFields("008")
			f.name = name
			writeFrame(t, conn, f.frame())
			assertNoReply(t, conn)
			s, ok := r.ChannelState(8)
			if !ok {
				t.Fatal("the Set stored nothing")
			}
			if s.Name != name {
				t.Errorf("stored Name = %q, want %q verbatim", s.Name, name)
			}
			if got, want := exchange(t, conn, "MA0008;"), f.frame(); got != want {
				t.Errorf("read back %q, want %q", got, want)
			}
		})
	}
}

// TestMA0Set_RefusesANameOutsideTheCharset. A2(i)'s bound: the printable-ASCII
// characters this design writes, inside 0x20-0x7E. ';' cannot arrive here at
// all — the reassembler ends a frame at the first one — so the reachable half
// is the control bytes and 0x7F upwards.
func TestMA0Set_RefusesANameOutsideTheCharset(t *testing.T) {
	_, conn := newTestRadio(t)
	for _, name := range []string{"A\x01B       ", "A\x7fB       ", "A\xffB       "} {
		f := plainFields("010")
		f.name = name
		assertRejected(t, conn, f.frame())
	}
}

// TestMA0Set_FieldValidators walks every wire-level rule on the Set direction.
// Each is ASSUMED to be what the radio itself enforces — doc.go's register
// entry SET-DIRECTION FIELD STRICTNESS — and nothing NORMALISES: a byte
// outside its printed legend is refused, never quietly corrected, because a
// driver that sent one would otherwise pass its own tests and fail on
// hardware.
func TestMA0Set_FieldValidators(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*fields)
	}{
		{"a non-digit frequency 1 (990:2906)", func(f *fields) { f.freq = "0000700000x" }},
		{"a blank-spelt frequency on the SET direction", func(f *fields) { f.freq = "           " }},
		{"a mode nibble outside OM's twenty-four (990:3707-3730)", func(f *fields) { f.mode = "O" }},
		{"an FM wide/narrow flag outside 0/1 (990:2913-2914)", func(f *fields) { f.fmNarrow = "2" }},
		{"a tone function outside 0-3 (990:2916-2919)", func(f *fields) { f.toneType = "4" }},
		{"a non-numeric tone index (990:2921-2922)", func(f *fields) { f.toneNo = "0x" }},
		{"a non-numeric CTCSS index (990:2925-2926)", func(f *fields) { f.ctcssNo = "x0" }},
		{"a non-digit frequency 2 (990:2928)", func(f *fields) { f.freq2 = "0000700000x" }},
		{"a mode 2 nibble outside OM's twenty-four (990:2930-2931)", func(f *fields) { f.mode2 = "O" }},
		{"an FM wide/narrow flag for frequency 2 outside 0/1 (990:2933-2934)", func(f *fields) { f.fmNarrow2 = "2" }},
		{"a tone function for frequency 2 outside 0-3 (990:2936-2939)", func(f *fields) { f.toneType2 = "4" }},
		{"a non-numeric tone index for frequency 2 (990:2941-2942)", func(f *fields) { f.toneNo2 = "0x" }},
		{"a non-numeric CTCSS index for frequency 2 (990:2944-2945)", func(f *fields) { f.ctcssNo2 = "x0" }},
		{"a split flag outside 0/1 (990:2947-2948)", func(f *fields) { f.split = "2" }},
		{"a dual-reception flag outside 0/1 (990:2950-2951)", func(f *fields) { f.dualRX = "2" }},
		{"a scan lockout outside 1/2 (990:2953-2954, E8)", func(f *fields) { f.lockout = "3" }},
		{"a scan lockout spelt with the 890S's 0 (990:2953-2954, E8)", func(f *fields) { f.lockout = "0" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, conn := newTestRadio(t)
			f := plainFields("011")
			tt.mutate(&f)
			assertRejected(t, conn, f.frame())
		})
	}
}

// TestMA0Set_AdmitsEveryPrintedModeNibble, the two the OM legend calls
// "Unused" included (990:3707, 990:3715) — doc.go's register entry A SET
// CARRYING AN "UNUSED" MODE NIBBLE IS STORED. Nothing says what a radio does
// with a Set carrying one, so this fake stores the nibble rather than
// inventing a refusal, which is what lets a test drive the codec's own build
// refusal against a real fake.
//
// THERE ARE TWENTY-FOUR, 0-9 and A-N (990:3707-3730), where the 890S's legend
// prints sixteen. A fake that borrowed the sibling's validator would refuse
// eight of this radio's own printed modes.
func TestMA0Set_AdmitsEveryPrintedModeNibble(t *testing.T) {
	nibbles := strings.Split("0123456789ABCDEFGHIJKLMN", "")
	if len(nibbles) != 24 {
		t.Fatalf("the test names %d nibbles, want the 24 the OM P2 legend prints (990:3707-3730)", len(nibbles))
	}
	for _, nibble := range nibbles {
		t.Run(nibble, func(t *testing.T) {
			r, conn := newTestRadio(t)
			f := plainFields("012")
			f.mode = nibble
			writeFrame(t, conn, f.frame())
			assertNoReply(t, conn)
			s, ok := r.ChannelState(12)
			if !ok || string(s.Mode) != nibble {
				t.Errorf("mode nibble %q was not stored (ok=%v, got %q)", nibble, ok, string(s.Mode))
			}
		})
	}
}

// TestMA0Set_ToneIndicesAreStoredNotRangeChecked — doc.go's register entry
// TONE INDICES ARE STORED, NOT RANGE-CHECKED. Both charts print "Entering a
// value that does not exist is invalid" for the TN and CN COMMANDS (990:4973,
// 990:1266-1267); whether the same rule holds inside an MA0 frame is
// unprinted, so only the field's SHAPE is enforced here. Refusing would assert
// a fact about the radio and put the codec's own refusal out of reach of a
// real fake.
func TestMA0Set_ToneIndicesAreStoredNotRangeChecked(t *testing.T) {
	r, conn := newTestRadio(t)
	f := plainFields("013")
	f.toneNo = "98" // above TN's printed 50 and below its set-only 99
	f.ctcssNo = "97"
	f.toneNo2 = "96"
	f.ctcssNo2 = "95"
	writeFrame(t, conn, f.frame())
	assertNoReply(t, conn)
	s, ok := r.ChannelState(13)
	if !ok || s.ToneNo != "98" || s.CTCSSNo != "97" || s.ToneNo2 != "96" || s.CTCSSNo2 != "95" {
		t.Errorf("stored (%v) P7=%q P8=%q P13=%q P14=%q, want them carried verbatim", ok, s.ToneNo, s.CTCSSNo, s.ToneNo2, s.CTCSSNo2)
	}
}

// TestMA0Set_TheTwoSidesAreNeverCompared. The 890S's book instructs a HOST to
// keep its P4 and P10 equal on a split channel (890:3219-3221); THIS book
// prints no such instruction at all, so there is even less to enforce here —
// each byte is checked against its own legend and the two are never compared.
func TestMA0Set_TheTwoSidesAreNeverCompared(t *testing.T) {
	r, conn := newTestRadio(t)
	f := plainFields("014")
	f.split = "1"
	f.freq2 = "00014195000"
	f.fmNarrow = "1"
	f.fmNarrow2 = "0"
	writeFrame(t, conn, f.frame())
	assertNoReply(t, conn)
	s, ok := r.ChannelState(14)
	if !ok || s.FMNarrow != '1' || s.FMNarrow2 != '0' {
		t.Errorf("stored (%v) P5=%q P11=%q, want the disagreeing pair carried as sent", ok, string(s.FMNarrow), string(s.FMNarrow2))
	}
}

// TestMA0Set_RefusesEveryWidthTheChartHasNoRulerFor. The grid is 57 positions
// and nothing else (990:2893-2915): the name window is fixed, so unlike the
// 890S there is no admitted range of lengths at all.
func TestMA0Set_RefusesEveryWidthTheChartHasNoRulerFor(t *testing.T) {
	_, conn := newTestRadio(t)
	base := plainFields("015").frame()
	body := strings.TrimSuffix(base, ";")
	for _, send := range []string{
		body[:len(body)-1] + ";", // 56 bytes: one short of the grid
		body + "A;",              // 58 bytes: one past it
	} {
		assertRejected(t, conn, send)
	}
}

// TestMA0Set_DoesNotDisturbAnotherChannel.
func TestMA0Set_DoesNotDisturbAnotherChannel(t *testing.T) {
	r, conn := newTestRadio(t)
	before, _ := r.ChannelState(0)
	writeFrame(t, conn, fieldsWithName("016", "X").frame())
	assertNoReply(t, conn)
	after, ok := r.ChannelState(0)
	if !ok || after != before {
		t.Errorf("channel 000 changed from %+v to %+v (ok=%v) after a Set to 016", before, after, ok)
	}
}

// TestMA0Set_ARefusedSetDoesNotDisturbTheTargetChannel. The validation switch
// runs before the store, so a rejected frame never reaches it — but nothing
// else pins that ORDERING. Moving the store above the switch leaves the whole
// package suite green apart from this test: the reply is "?;" either way, so
// the corruption is silent, and T17/T18 would read it as channel data. This is
// the "erase-by-side-effect" class the repository's standing no-erase rule
// exists to prevent (the T15 review's M1, ruled ACCEPT for both fakes).
func TestMA0Set_ARefusedSetDoesNotDisturbTheTargetChannel(t *testing.T) {
	_, conn := newTestRadio(t)
	f := fieldsWithName("018", "OK")
	writeFrame(t, conn, f.frame())
	assertNoReply(t, conn)

	before := exchange(t, conn, "MA0018;")
	if before != f.frame() {
		t.Fatalf("the populating Set was not stored: MA0018; -> %q, want %q", before, f.frame())
	}

	bad := f
	bad.mode = "O" // outside the OM legend's twenty-four nibbles (990:3707-3730) — refused
	assertRejected(t, conn, bad.frame())

	if got := exchange(t, conn, "MA0018;"); got != before {
		t.Errorf("after a REFUSED Set, MA0018; -> %q, want the pre-Set answer %q, byte-identical", got, before)
	}
}
