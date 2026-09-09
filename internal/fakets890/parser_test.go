// SPDX-License-Identifier: GPL-3.0-or-later

package fakets890

import (
	"strings"
	"testing"
)

// fields is a test-local assembler for the MA0 grid, INDEPENDENT of this
// package's own builder: it names each parameter as the chart names it and
// concatenates the cells in the chart's own order (890:3166-3182 Set,
// 890:3189-3204 Answer, which number the same positions the same way).
//
// A builder with a transposed field would still pass a test whose expectation
// came from that builder. It cannot pass one whose expectation is assembled
// here.
type fields struct {
	slot          string // P1, positions 4-6
	freq          string // P2, positions 7-17
	mode          string // P3, position 18
	fmNarrow      string // P4, position 19
	toneType      string // P5, position 20
	toneNo        string // P6, positions 21-22
	ctcssNo       string // P7, positions 23-24
	splitFreq     string // P8, positions 25-35
	splitMode     string // P9, position 36
	splitFMNarrow string // P10, position 37
	split         string // P11, position 38
	lockout       string // P12, position 39
	name          string // P13, position 40 onwards, 0-10 characters
}

// frame renders the fields as a complete MA0 frame — the same bytes on the Set
// and Answer directions, which is what the two identical grids print.
func (f fields) frame() string {
	return "MA0" + f.slot + f.freq + f.mode + f.fmNarrow + f.toneType +
		f.toneNo + f.ctcssNo + f.splitFreq + f.splitMode + f.splitFMNarrow +
		f.split + f.lockout + f.name + ";"
}

// plainFields is one unremarkable populated record: the front matter's worked
// frequency (890:86), FM, and the printed FIRST value of every other legend in
// the grid. Split transmission is all zeroes, which the book itself prints for
// a single memory channel (890:3217-3218).
func plainFields(slot string) fields {
	return fields{
		slot:          slot,
		freq:          "00007000000",
		mode:          "4", // "4: FM" (890:3981)
		fmNarrow:      "0", // "0: Normal" (890:3177)
		toneType:      "0", // "0: OFF" (890:3181)
		toneNo:        "00",
		ctcssNo:       "00",
		splitFreq:     "00000000000",
		splitMode:     "0",
		splitFMNarrow: "0",
		split:         "0", // "0: Simplex" (890:3202)
		lockout:       "0", // "0: Lockout OFF" (890:3206)
		name:          "",
	}
}

// blankFrame is the 40-byte answer a blank channel gives: "MA0", the slot, and
// P2 to P12 blank (890:3215-3216) with "blank" read as ASCII space (A6), the
// name window absent (A4) — doc.go's register entry A BLANK CHANNEL ANSWERS
// THE 40-BYTE FRAME. The 33 is counted here off the chart's own ruler:
// positions 7 to 39 inclusive.
func blankFrame(slot string) string {
	return "MA0" + slot + strings.Repeat(" ", 33) + ";"
}

// --- The frame geometry itself ---

// TestBlankFrameIsFortyBytes re-derives A17's figure from the chart's ruler
// rather than taking it from this package: 3 name bytes + 3 slot digits + P2
// to P12 + the terminator.
func TestBlankFrameIsFortyBytes(t *testing.T) {
	if got := len(blankFrame("000")); got != 40 {
		t.Fatalf("the test assembler builds a %d-byte blank frame, want 40 (890:3177-3182, A17)", got)
	}
	if got := len(plainFields("000").frame()); got != 40 {
		t.Errorf("a populated record with a zero-character name is %d bytes, want 40 — P13 is \"Up to 10 characters\" (890:3208-3209) and the terminator floats after it", got)
	}
	if got := len(fieldsWithName("000", "0123456789").frame()); got != 50 {
		t.Errorf("a populated record with a ten-character name is %d bytes, want 50", got)
	}
}

func fieldsWithName(slot, name string) fields {
	f := plainFields(slot)
	f.name = name
	return f
}

// --- MA0 read (890:3184-3186) ---

// TestMA0Read_AnswersThePopulatedRecordVerBatim over the default image's own
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
		t.Errorf("ChannelState(0).Freq = %q, want the front matter's worked frequency (890:86)", s.Freq)
	}
}

// TestMA0Read_AnAbsentChannelAnswersTheBlankFrame, never a rejection. The book
// prints the blank channel as a normal, answerable state — "When reading a
// blank channel, parameters P2 to P12 becomes blank." (890:3215-3216) — which
// is what core/driver/ts890 must read as "empty channel" and not as "parse
// error".
func TestMA0Read_AnAbsentChannelAnswersTheBlankFrame(t *testing.T) {
	_, conn := newTestRadio(t)
	for _, slot := range []string{"050", "099"} {
		if got, want := exchange(t, conn, "MA0"+slot+";"), blankFrame(slot); got != want {
			t.Errorf("MA0%s; -> %q, want %q", slot, got, want)
		}
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
// "M A 0 P1 P1 P1 ;" (890:3186), and P1 is three ASCII digits over "000 ~ 119"
// (890:3167). Nothing here normalises — doc.go's register entry SET-DIRECTION
// FIELD STRICTNESS.
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
// ARE NOT SERVED. The chart prints the domain "000 ~ 119" (890:3167) and maps
// 100-109 to P0-P9 and 110-119 to E0-E9 (890:3168-3169); nothing anywhere in
// the book says what either class holds or what a read of one answers (the
// design's A9 and A10, and E18 records that E0-E9 are never explained at all).
// The slots are published in no bank of this row, so there is nothing for this
// fake to represent, and the narrowing is deliberate rather than a claim that
// a TS-890S refuses those numbers.
func TestMA0_SlotsAbove099AreNotServed(t *testing.T) {
	_, conn := newTestRadio(t)
	for _, slot := range []string{"100", "109", "110", "119", "120", "999"} {
		assertRejected(t, conn, "MA0"+slot+";")
	}
}

// --- MA0 Set (890:3166-3182) ---

// TestMA0Set_IsFireAndForgetAndStores — doc.go's register entry AN ACCEPTED
// SET PRODUCES NO REPLY (the design's A20: MA0's own block is silent about an
// acknowledgement where MA2, MA3 and MA6 each print one).
func TestMA0Set_IsFireAndForgetAndStores(t *testing.T) {
	_, conn := newTestRadio(t)
	f := fieldsWithName("007", "TEST")
	f.freq = "00014175000" // the AS0 block's worked frequency (890:342)
	writeFrame(t, conn, f.frame())
	assertNoReply(t, conn)

	if got, want := exchange(t, conn, "MA0007;"), f.frame(); got != want {
		t.Errorf("after the Set, MA0007; -> %q, want %q", got, want)
	}
}

// TestMA0Set_ToABlankChannelIsStored — doc.go's register entry A SET TO A
// BLANK CHANNEL IS STORED. Whether MA0 alone can create a channel is the
// design's A3, unlifted, and core/driver/ts890 REFUSES such a write on that
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

// TestMA0Set_TheNameIsCarriedVerbatim, trailing space included. On this row
// the terminator FLOATS after the name (890:3181-3182), so "AB ;" and "AB;"
// are DISTINCT, unambiguous frames and a trailing space is content rather than
// padding — the C-MED-1 reversal the codec carries. A fake that trimmed on
// read or padded on write would apply an assumption on both sides of every
// round trip and could never contradict it.
func TestMA0Set_TheNameIsCarriedVerbatim(t *testing.T) {
	for _, name := range []string{"", "A", "AB ", " AB", "0123456789"} {
		t.Run(strings.ReplaceAll(name, " ", "_"), func(t *testing.T) {
			r, conn := newTestRadio(t)
			f := fieldsWithName("008", name)
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

// TestMA0Set_RefusesAnOverLongName. "Up to 10 characters" (890:3208-3209) is a
// counted bound, and an eleven-character name makes a 51-byte frame the chart
// has no ruler for.
func TestMA0Set_RefusesAnOverLongName(t *testing.T) {
	_, conn := newTestRadio(t)
	assertRejected(t, conn, fieldsWithName("009", "01234567890").frame())
}

// TestMA0Set_RefusesANameOutsideTheCharset. A2(i)'s bound: the printable-ASCII
// characters this design writes, inside 0x20-0x7E. ';' cannot arrive here at
// all — the reassembler ends a frame at the first one — so the reachable half
// is the control bytes and 0x7F upwards.
func TestMA0Set_RefusesANameOutsideTheCharset(t *testing.T) {
	_, conn := newTestRadio(t)
	for _, name := range []string{"A\x01B", "A\x7fB", "A\xffB"} {
		assertRejected(t, conn, fieldsWithName("010", name).frame())
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
		{"a non-digit frequency (890:3171-3172, \"Blank digits must be entered as 0\")", func(f *fields) { f.freq = "0000700000x" }},
		{"a blank-spelt frequency on the SET direction", func(f *fields) { f.freq = "           " }},
		{"a mode nibble outside OM's sixteen (890:3976-3992)", func(f *fields) { f.mode = "G" }},
		{"an FM narrow flag outside 0/1 (890:3177-3178)", func(f *fields) { f.fmNarrow = "2" }},
		{"a tone type outside 0-3 (890:3181-3185)", func(f *fields) { f.toneType = "4" }},
		{"a non-numeric tone index (890:3186-3187)", func(f *fields) { f.toneNo = "0x" }},
		{"a non-numeric CTCSS index (890:3188-3190)", func(f *fields) { f.ctcssNo = "x0" }},
		{"a non-digit split frequency (890:3191-3192)", func(f *fields) { f.splitFreq = "0000700000x" }},
		{"a split mode nibble outside OM's sixteen (890:3193-3195)", func(f *fields) { f.splitMode = "G" }},
		{"a split FM narrow flag outside 0/1 (890:3197-3200)", func(f *fields) { f.splitFMNarrow = "2" }},
		{"a split flag outside 0/1 (890:3201-3203)", func(f *fields) { f.split = "2" }},
		{"a lockout flag outside 0/1 (890:3205-3207)", func(f *fields) { f.lockout = "2" }},
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

// TestMA0Set_AdmitsEverySixteenthModeNibble, the two the OM legend calls
// "Unused" included (890:3977, 890:3985) — doc.go's register entry A SET
// CARRYING AN "UNUSED" MODE NIBBLE IS STORED. Nothing says what a radio does
// with a Set carrying one, so this fake stores the nibble rather than
// inventing a refusal, which is what lets a test drive the codec's own build
// refusal against a real fake.
func TestMA0Set_AdmitsEverySixteenthModeNibble(t *testing.T) {
	for _, nibble := range strings.Split("0123456789ABCDEF", "") {
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
// value that does not exist is invalid" for the TN and CN COMMANDS
// (890:5163-5164, 890:1370-1371); whether the same rule holds inside an MA0
// frame is unprinted, so only the field's SHAPE is enforced here. Refusing
// would assert a fact about the radio and put the codec's own refusal out of
// reach of a real fake.
func TestMA0Set_ToneIndicesAreStoredNotRangeChecked(t *testing.T) {
	r, conn := newTestRadio(t)
	f := plainFields("013")
	f.toneNo = "98" // above TN's printed 50 and below its set-only 99
	f.ctcssNo = "97"
	writeFrame(t, conn, f.frame())
	assertNoReply(t, conn)
	s, ok := r.ChannelState(13)
	if !ok || s.ToneNo != "98" || s.CTCSSNo != "97" {
		t.Errorf("stored (%v) ToneNo=%q CTCSSNo=%q, want them carried verbatim", ok, s.ToneNo, s.CTCSSNo)
	}
}

// TestMA0Set_TheP4P10AgreementIsNotEnforced — doc.go's register entry THE
// P4/P10 AGREEMENT IS NOT ENFORCED. The book instructs the HOST: "When setting
// the split memory channel, set the same setting on the transmission side and
// the reception side for FM normal / narrow information (P4, P10)."
// (890:3219-3221). What the radio does with a disagreeing Set is unprinted,
// and core/driver/ts890 carries a refusal rung for exactly this — a rung this
// fake must not pre-empt, or the driver's own refusal could never be shown
// against a real fake.
func TestMA0Set_TheP4P10AgreementIsNotEnforced(t *testing.T) {
	r, conn := newTestRadio(t)
	f := plainFields("014")
	f.split = "1"
	f.fmNarrow = "1"
	f.splitFMNarrow = "0"
	writeFrame(t, conn, f.frame())
	assertNoReply(t, conn)
	s, ok := r.ChannelState(14)
	if !ok || s.FMNarrow != '1' || s.SplitFMNarrow != '0' {
		t.Errorf("stored (%v) P4=%q P10=%q, want the disagreeing pair carried as sent", ok, string(s.FMNarrow), string(s.SplitFMNarrow))
	}
}

// TestMA0Set_RefusesEveryWidthTheChartHasNoRulerFor. The grid is 39 fixed
// positions, a name of 0 to 10 characters and the terminator (890:3166-3182),
// so a frame is 40 to 50 bytes and nothing else.
func TestMA0Set_RefusesEveryWidthTheChartHasNoRulerFor(t *testing.T) {
	_, conn := newTestRadio(t)
	base := plainFields("015").frame()
	body := strings.TrimSuffix(base, ";")
	for _, send := range []string{
		body[:len(body)-1] + ";",             // 39 bytes: one short of the fixed grid
		body + strings.Repeat("A", 11) + ";", // 51 bytes: one past the name window
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
