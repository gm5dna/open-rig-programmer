// SPDX-License-Identifier: GPL-3.0-or-later

package fakets480

import (
	"testing"
)

// recordFrame is this test file's OWN assembler for the 50-byte memory
// record, written from the position rulers the two charts print — MR's
// answer (480:923-943) and MW's Set (480:955-976), which number the same
// fifty positions the same way — and NOT by calling buildMRAnswer. A builder
// with a bug must still be catchable, which it cannot be if the expectation
// comes from the same builder.
//
// Positions, 1-indexed as the charts number them:
//
//	1-2    the command name
//	3      P1   0: RX frequency, 1: TX frequency  (480:908, 480:951)
//	4      P2   "Always 0 for the TS-480."        (480:910, 480:953)
//	5-6    P3   00 ~ 99, the channel number       (480:912, 480:955)
//	7-17   P4   frequency, 11 digits              (480:914, 480:957)
//	18     P5   mode, refer to MD                 (480:917, 480:959)
//	19     P6   lockout status                    (480:920, 480:962)
//	20     P7   0: OFF, 1: TONE, 2: CTCSS         (480:922, 480:964)
//	21-22  P8   tone number                       (480:924, 480:966)
//	23-24  P9   CTCSS tone number                 (480:927, 480:969)
//	25-27  P10  "Always 000 for the TS-480."      (480:929, 480:971)
//	28     P11  "Always 0 for the TS-480."        (480:931, 480:973)
//	29     P12  "Always 0 for the TS-480."        (480:933, 480:975)
//	30-38  P13  "Always 000000000 for the TS-480."(480:935, 480:977)
//	39-40  P14  step size, refer to ST            (480:937, 480:979)
//	41     P15  "Always 0 for the TS-480."        (480:939, 480:982)
//	42-49  P16  memory name, 8 characters         (480:941, 480:984)
//	50     ';'                                    (480:943, 480:976)
//
// THE GRID IS THE 590 PAIR'S GEOMETRY AND NOT ITS MEANINGS. Bytes 19, 28,
// 39-40 and 41 do different jobs on the two radios, and byte 4 is a printed
// constant here where the 590 pair carry the channel's hundreds digit — see
// doc.go's list.
type recordFrame struct {
	cmd      string // "MR" or "MW"
	p1       byte
	p2       byte
	p3       string
	freq     string
	mode     byte
	lockout  byte
	toneMode byte
	toneNo   string
	ctcssNo  string
	p10      string
	p11      byte
	p12      byte
	p13      string
	step     string
	p15      byte
	name     string
}

// newRecordFrame is one unremarkable populated record: the book's printed
// example frequency (480:549), FM (480:848), every other legend at its
// printed first value, every hard-wired run at its printed constant, and an
// eight-space name.
func newRecordFrame(cmd string, p1 byte, p3 string) recordFrame {
	return recordFrame{
		cmd:      cmd,
		p1:       p1,
		p2:       '0',
		p3:       p3,
		freq:     "00014195000",
		mode:     '4',
		lockout:  '0',
		toneMode: '0',
		toneNo:   "00",
		ctcssNo:  "00",
		p10:      "000",
		p11:      '0',
		p12:      '0',
		p13:      "000000000",
		step:     "00",
		p15:      '0',
		name:     "        ",
	}
}

func (f recordFrame) String() string {
	out := make([]byte, 0, recLen)
	out = append(out, f.cmd...)
	out = append(out, f.p1, f.p2)
	out = append(out, f.p3...)
	out = append(out, f.freq...)
	out = append(out, f.mode, f.lockout, f.toneMode)
	out = append(out, f.toneNo...)
	out = append(out, f.ctcssNo...)
	out = append(out, f.p10...)
	out = append(out, f.p11, f.p12)
	out = append(out, f.p13...)
	out = append(out, f.step...)
	out = append(out, f.p15)
	out = append(out, f.name...)
	out = append(out, ';')
	return string(out)
}

// TestRecordFrame_IsFiftyBytes guards the assembler above: a test fixture
// that had drifted from the chart would make every comparison below agree
// with the wrong thing.
func TestRecordFrame_IsFiftyBytes(t *testing.T) {
	got := newRecordFrame("MR", '0', "07").String()
	if len(got) != 50 {
		t.Fatalf("the test assembler produced %d bytes (%q), want the 50 the charts count (480:941-943)", len(got), got)
	}
}

// --- MR: the memory read (480:906-944) ---

// TestMR_ReadFrameIsSevenBytes pins the request grammar: "M R P1 P2 P3 P3 ;"
// — seven positions (480:918).
func TestMR_ReadFrameIsSevenBytes(t *testing.T) {
	_, conn := newTestRadio(t)

	got := exchange(t, conn, "MR0000;")
	if len(got) != 50 {
		t.Fatalf("MR0000; -> %d bytes, want the 50-byte answer", len(got))
	}

	for _, bad := range []string{"MR;", "MR0;", "MR000;", "MR00000;"} {
		assertRejected(t, conn, bad)
	}
}

// TestMR_AnswersThePopulatedRecordByteForByte compares the fake's answer with
// a frame this file assembles from the chart's own ruler.
func TestMR_AnswersThePopulatedRecordByteForByte(t *testing.T) {
	_, conn := newTestRadio(t)

	want := newRecordFrame("MR", '0', "00").String()
	if got := exchange(t, conn, "MR0000;"); got != want {
		t.Errorf("MR0000; ->\n %q\nwant\n %q", got, want)
	}
}

// TestMR_RefusesABankByteOtherThanZero is the sharpest place this radio is
// NOT the 590 pair. Byte 4 is a printed constant here — "Always 0 for the
// TS-480." (480:910) — where the 590 pair carry the channel's hundreds digit
// and print a space/zero spelling rule for it (590:1334-1337). So a frame
// this fake's sibling would accept is refused here, in both spellings, and
// the channel number is P3's two digits alone (480:912).
func TestMR_RefusesABankByteOtherThanZero(t *testing.T) {
	_, conn := newTestRadio(t)
	for _, send := range []string{
		"MR0 00;", // the 590 pair's space spelling
		"MR0100;", // a hundreds digit this radio's byte 4 is not
		"MR0900;",
		"MR0X00;",
	} {
		assertRejected(t, conn, send)
	}
}

// TestMR_TheWholeSlotSpaceIsExpressibleAndNothingElseIs. P3 is two digits
// (480:912), so 00-99 is not merely the served space but the whole of what a
// well-formed frame can name: there is no out-of-domain channel NUMBER on
// this radio to refuse, and a frame reaching for one is refused as a width
// violation instead. The 590 pair's 100-119 question does not arise here.
func TestMR_TheWholeSlotSpaceIsExpressibleAndNothingElseIs(t *testing.T) {
	_, conn := newTestRadio(t)

	for _, send := range []string{"MR0000;", "MR0050;", "MR0099;"} {
		if got := exchange(t, conn, send); len(got) != 50 {
			t.Errorf("%s -> %q, want a 50-byte answer", send, got)
		}
	}
	// Three digits is an eight-byte frame, not a channel above 99.
	assertRejected(t, conn, "MR00100;")
}

// TestMR_AnUnwrittenChannelAnswersTheZeroRecord IS THIS FAKE ASSERTING A4,
// and A4 is unlifted. This book says NOTHING about an empty channel
// anywhere: the sentence the 590 pair's book prints — "If the selected
// channel is empty, P4 ~ P15 will be 0 and P16 will be blank."
// (590:1492-1493) — has no counterpart here, and reading it across to this
// radio is the design's A4, whose lift (L-HW-3) is the TS-480 row's RELEASE
// GATE. No radio has confirmed either that an empty channel answers at all
// or what it answers with.
func TestMR_AnUnwrittenChannelAnswersTheZeroRecord(t *testing.T) {
	_, conn := newTestRadio(t)

	got := exchange(t, conn, "MR0055;")
	if len(got) != 50 {
		t.Fatalf("MR0055; -> %q, want a 50-byte answer", got)
	}
	// Bytes 7-41 inclusive, 1-indexed: the whole P4-P15 run.
	for i := 7; i <= 41; i++ {
		if got[i-1] != '0' {
			t.Errorf("byte %d of the zero answer is %q, want '0'", i, got[i-1])
		}
	}
	if name := got[41:49]; name != "        " {
		t.Errorf("the zero answer's name field is %q, want eight spaces (the design's A3, unlifted on this row)", name)
	}
	if got[2] != '0' || got[3] != '0' || got[4:6] != "55" {
		t.Errorf("the zero answer names channel %q, want P1 '0', P2 '0' and channel 55", got[2:6])
	}
}

// TestMR_TheTransmitHalfOfAChannelWithNoStoredRecordIsEmpty. What an MR with
// P1=1 answers on a channel holding only a receive record is unprinted here
// — the design's A9, which gates every Kenwood channel write on all three
// rows — so this fake gives it the same zero record A4 already carries,
// rather than inventing a rejection or echoing the receive frequency back as
// a transmit one. doc.go's register entry THE TRANSMIT HALF OF A CHANNEL
// WITH NO STORED RECORD.
func TestMR_TheTransmitHalfOfAChannelWithNoStoredRecordIsEmpty(t *testing.T) {
	_, conn := newTestRadio(t)

	got := exchange(t, conn, "MR1000;")
	if len(got) != 50 {
		t.Fatalf("MR1000; -> %q, want a 50-byte answer", got)
	}
	if freq := got[6:17]; freq != "00000000000" {
		t.Errorf("MR1000; answered frequency %q, want the zero record's zeros", freq)
	}
}

// TestMR_ServesTheStartEndOverloadOn90To99. The book prints the overload —
// "Memory channel 90 ~ 99: P1=0 (start frequency), P1=1 (end frequency)"
// (480:943-944) — so both frames are documented reads and this fake serves
// both. THE DEFAULT IMAGE POPULATES NEITHER of the upper halves: decision 15
// gives this row one flat MEM bank of 00-99 and no scan bank, so the P1=1
// half of 90-99 is not a published slot, and P19 ships no image for a slot
// the row does not publish. Both halves answer; the upper one answers empty.
func TestMR_ServesTheStartEndOverloadOn90To99(t *testing.T) {
	_, conn := newTestRadio(t)

	for _, p1 := range []string{"0", "1"} {
		got := exchange(t, conn, "MR"+p1+"095;")
		if len(got) != 50 {
			t.Fatalf("MR%s095; -> %q, want a 50-byte answer", p1, got)
		}
		if string(got[2]) != p1 {
			t.Errorf("MR%s095; answered with P1 %q, want %q", p1, got[2], p1)
		}
	}
}

// TestMR_RefusesAMalformedRequest covers each field of the seven-byte read.
func TestMR_RefusesAMalformedRequest(t *testing.T) {
	_, conn := newTestRadio(t)
	for _, send := range []string{
		"MR2000;", // P1 outside "0: RX frequency, 1: TX frequency" (480:908)
		"MRX000;",
		"MR000X;", // P3 not two digits (480:912)
		"MR00X0;",
	} {
		assertRejected(t, conn, send)
	}
}

// TestWithMemoryReadUnsupported_RefusesEveryMR makes decision 5's rule
// reachable: a "?;" is a definitive rejection, never retried and never read
// as absence. The option refuses every MR while MW and MC stay untouched, so
// the driver's typed whole-read failure can be driven through a real fake.
//
// It is NOT a claim that any TS-480 refuses MR. It plays the second cause the
// error table itself prints — "Command was not executed due to the current
// status of the transceiver (even though the command syntax was correct)"
// (480:130-135).
func TestWithMemoryReadUnsupported_RefusesEveryMR(t *testing.T) {
	_, conn := newTestRadio(t, WithMemoryReadUnsupported())

	assertRejected(t, conn, "MR0000;")
	assertRejected(t, conn, "MR0055;")

	// MC still answers, and MW is still accepted: only the read is refused.
	if got, want := exchange(t, conn, "MC;"), "MC000;"; got != want {
		t.Errorf("MC; -> %q, want %q", got, want)
	}
	writeFrame(t, conn, newRecordFrame("MW", '0', "07").String())
	assertNoReply(t, conn)
}

// --- MW: the memory write (480:949-987) ---

// TestMW_AcceptsExactlyFiftyBytesAndAnswersNothing. The Set chart counts
// fifty positions (480:955-976) and prints Read and Answer LABELS over EMPTY
// charts (480:980, 480:985) — erratum E17 — so an accepted write is silent.
func TestMW_AcceptsExactlyFiftyBytesAndAnswersNothing(t *testing.T) {
	r, conn := newTestRadio(t)

	f := newRecordFrame("MW", '0', "07")
	f.freq = "00007100000"
	writeFrame(t, conn, f.String())
	assertNoReply(t, conn)

	got, ok := r.ChannelState(7, HalfRXOrStart)
	if !ok {
		t.Fatal("channel 7 holds no record after the MW")
	}
	if got.Freq != "00007100000" {
		t.Errorf("stored frequency %q, want %q", got.Freq, "00007100000")
	}

	// And it reads back, byte for byte. Byte 4 is the same printed constant
	// in both directions here, so unlike the 590 pair's answer there is
	// nothing to re-spell.
	want := f
	want.cmd = "MR"
	if back := exchange(t, conn, "MR0007;"); back != want.String() {
		t.Errorf("MR0007; ->\n %q\nwant\n %q", back, want.String())
	}
}

// TestMW_HasNoReadOrAnswerDirection. The MW block prints Read and Answer
// labels over empty charts (480:980, 480:985) — erratum E17's shape — so
// "MW;" is not a read, it is an unknown frame.
func TestMW_HasNoReadOrAnswerDirection(t *testing.T) {
	_, conn := newTestRadio(t)
	assertRejected(t, conn, "MW;")
}

// TestMW_RefusesEveryOtherWidth, the erase-shaped frame included. THIS BOOK
// DESCRIBES NO ERASE FORM AT ALL — the sentence the 590 pair's book prints
// (590:1579-1581) has no counterpart here — so a 42-byte MW is not even a
// documented shape on this radio, and the width rule that refuses it is the
// same one that refuses any other malformed width. This programme builds no
// erase frame on any radio in any case.
func TestMW_RefusesEveryOtherWidth(t *testing.T) {
	r, conn := newTestRadio(t)
	full := newRecordFrame("MW", '0', "07").String()

	erase := full[:41] + ";" // the record without its eight name bytes
	if len(erase) != 42 {
		t.Fatalf("the erase-shaped fixture is %d bytes, want 42", len(erase))
	}
	assertRejected(t, conn, erase)
	assertRejected(t, conn, full[:len(full)-2]+";")
	assertRejected(t, conn, full[:len(full)-1]+" ;")

	if _, ok := r.ChannelState(7, HalfRXOrStart); ok {
		t.Error("a refused frame reached the record map")
	}
}

// TestMW_RefusesAFieldOutsideItsPrintedLegend is the SET-DIRECTION FIELD
// STRICTNESS register entry, exercised field by field. Every refusal here is
// ASSUMED to be what the radio itself enforces; what the fake must not do is
// silently normalise a byte, because a driver that sent one would then pass
// its own tests and fail on hardware.
//
// THE SIX HARD-WIRED RUNS ARE REQUIRED ON THIS SIDE TOO, which is the design's
// A24 and is a CHOICE rather than a deduction: this book's own general note
// permits a Set to fill an inapplicable parameter with "any character except
// the ASCII control codes (00 to 1Fh) and the terminator (;)" (480:108-111).
// Strictness is this programme's, and A24's lift would turn it into "accepted
// and normalised".
func TestMW_RefusesAFieldOutsideItsPrintedLegend(t *testing.T) {
	base := func() recordFrame { return newRecordFrame("MW", '0', "07") }
	tests := []struct {
		name  string
		mutis func(f *recordFrame)
	}{
		{"P1 outside 0/1 (480:951)", func(f *recordFrame) { f.p1 = '2' }},
		{"P2 not the printed constant (480:953)", func(f *recordFrame) { f.p2 = '1' }},
		{"P2 the 590 pair's space spelling (480:953)", func(f *recordFrame) { f.p2 = ' ' }},
		{"P3 not two digits (480:955)", func(f *recordFrame) { f.p3 = "X7" }},
		{"P4 not eleven digits (480:957)", func(f *recordFrame) { f.freq = "0001419500X" }},
		{"P5 outside the MD legend (480:843-854)", func(f *recordFrame) { f.mode = 'A' }},
		{"P6 outside 0/1 (480:962)", func(f *recordFrame) { f.lockout = '2' }},
		{"P7 outside 0..2 (480:964)", func(f *recordFrame) { f.toneMode = '3' }},
		{"P8 not two digits (480:966)", func(f *recordFrame) { f.toneNo = "0X" }},
		{"P9 not two digits (480:969)", func(f *recordFrame) { f.ctcssNo = "X0" }},
		{"P10 not the printed constant (480:971)", func(f *recordFrame) { f.p10 = "001" }},
		{"P11 not the printed constant (480:973)", func(f *recordFrame) { f.p11 = '1' }},
		{"P12 not the printed constant (480:975)", func(f *recordFrame) { f.p12 = '1' }},
		{"P13 not the printed constant (480:977)", func(f *recordFrame) { f.p13 = "000000001" }},
		{"P14 not two digits (480:979)", func(f *recordFrame) { f.step = "0X" }},
		{"P15 not the printed constant (480:982)", func(f *recordFrame) { f.p15 = '1' }},
		{"P16 outside printable ASCII (480:127-129, A2)", func(f *recordFrame) { f.name = "AB\x01     " }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, conn := newTestRadio(t)
			f := base()
			tt.mutis(&f)
			if len(f.String()) != 50 {
				t.Fatalf("the mutated fixture is %d bytes, want 50 — this case would be testing the width rule instead", len(f.String()))
			}
			assertRejected(t, conn, f.String())
			if _, ok := r.ChannelState(7, HalfRXOrStart); ok {
				t.Error("the refused frame reached the record map")
			}
		})
	}
}

// TestMW_RefusesTheToneModeTheSiblingPrints is the negative half of the tone
// legend's divergence, stated as its own pin because it is exactly the value
// a transcriber working from the wrong book would admit: this radio's P7
// legend stops at "2: CTCSS" (480:964), where the 590 pair print a fourth
// value, "3: Cross Tone ON" (590:1467).
func TestMW_RefusesTheToneModeTheSiblingPrints(t *testing.T) {
	_, conn := newTestRadio(t)
	f := newRecordFrame("MW", '0', "07")
	f.toneMode = '3'
	assertRejected(t, conn, f.String())
}

// TestMW_StoresAModeNibbleThisBookCallsNotUsed. Nibbles 0 and 8 are printed
// "0: No mode (Not used for the TS-480)" and "8: Tune (Not used for the
// TS-480)" (480:843, 480:853) and NOTHING says what a Set carrying one does
// — that is the design's A18b, whose lift is a hardware trial. So this fake
// stores the nibble rather than inventing a refusal, which is what lets a
// test drive core/kw's own build refusal against a real fake. doc.go's
// register entry A SET CARRYING AN UNUSED MODE NIBBLE IS STORED.
func TestMW_StoresAModeNibbleThisBookCallsNotUsed(t *testing.T) {
	for _, nibble := range []byte{'0', '8'} {
		r, conn := newTestRadio(t)
		f := newRecordFrame("MW", '0', "07")
		f.mode = nibble
		writeFrame(t, conn, f.String())
		assertNoReply(t, conn)

		got, ok := r.ChannelState(7, HalfRXOrStart)
		if !ok {
			t.Fatalf("nibble %q: channel 7 holds no record", nibble)
		}
		if got.Mode != nibble {
			t.Errorf("stored mode %q, want %q", got.Mode, nibble)
		}
	}
}

// TestMW_StoresAToneIndexOutsideThePrintedRange. TN prints "00 ~ 42"
// (480:1557) and CN "00 ~ 41" (480:337), and this book says nothing at all
// about what either rule does inside a memory frame — the design's A21,
// unlifted. Refusing here would assert A21 as a fact about the radio, so the
// fake stores the index and can therefore serve one the codec must refuse to
// parse. doc.go's register entry TONE INDICES ARE STORED, NOT RANGE-CHECKED.
func TestMW_StoresAToneIndexOutsideThePrintedRange(t *testing.T) {
	r, conn := newTestRadio(t)
	f := newRecordFrame("MW", '0', "07")
	f.toneNo = "43"
	f.ctcssNo = "99"
	writeFrame(t, conn, f.String())
	assertNoReply(t, conn)

	got, ok := r.ChannelState(7, HalfRXOrStart)
	if !ok {
		t.Fatal("channel 7 holds no record")
	}
	if got.ToneNo != "43" || got.CTCSSNo != "99" {
		t.Errorf("stored tone/CTCSS indices %q/%q, want %q/%q", got.ToneNo, got.CTCSSNo, "43", "99")
	}
}

// TestMW_StoresAStepIndexOutsideEitherPrintedRange. P14 says only "Step
// size. Refer to the ST command." (480:979), and ST's legend is
// MODE-CONDITIONAL over two different ranges — 00 ~ 04 for SSB/CW/FSK and
// 00 ~ 09 for AM/FM, where index 00 means 0.5 kHz in the first and 5 kHz in
// the second (480:1494-1500). A record carries no way to know which range
// applies without reading its own mode nibble, and no printed value is a "no
// change" value — that is the design's A22, and it is why every TS-480
// channel write in this programme is refused. This fake enforces the FIELD'S
// SHAPE alone. doc.go's register entry THE STEP INDEX IS STORED, NOT
// RANGE-CHECKED.
func TestMW_StoresAStepIndexOutsideEitherPrintedRange(t *testing.T) {
	r, conn := newTestRadio(t)
	f := newRecordFrame("MW", '0', "07")
	f.step = "99"
	writeFrame(t, conn, f.String())
	assertNoReply(t, conn)

	got, ok := r.ChannelState(7, HalfRXOrStart)
	if !ok {
		t.Fatal("channel 7 holds no record")
	}
	if got.Step != "99" {
		t.Errorf("stored step index %q, want %q", got.Step, "99")
	}
}

// TestMW_TheTwoHalvesAreSeparateRecords. P1 selects which frequency a write
// carries — "0: RX frequency, 1: TX frequency" (480:951) — so a P1=1 write
// must not disturb the P1=0 record.
func TestMW_TheTwoHalvesAreSeparateRecords(t *testing.T) {
	r, conn := newTestRadio(t)

	rx := newRecordFrame("MW", '0', "07")
	rx.freq = "00007100000"
	writeFrame(t, conn, rx.String())
	assertNoReply(t, conn)

	tx := newRecordFrame("MW", '1', "07")
	tx.freq = "00007200000"
	writeFrame(t, conn, tx.String())
	assertNoReply(t, conn)

	first, ok := r.ChannelState(7, HalfRXOrStart)
	if !ok || first.Freq != "00007100000" {
		t.Errorf("the P1=0 record is %+v, want the untouched 00007100000", first)
	}
	second, ok := r.ChannelState(7, HalfTXOrEnd)
	if !ok || second.Freq != "00007200000" {
		t.Errorf("the P1=1 record is %+v, want 00007200000", second)
	}
}

// TestMW_DoesNotMoveTheSelectedChannel — doc.go's register entry A SET DOES
// NOT MOVE THE SELECTED CHANNEL. Nothing in the MW block mentions the
// selection, and a fake that moved it would let a driver depend on a
// side-effect the book does not describe.
func TestMW_DoesNotMoveTheSelectedChannel(t *testing.T) {
	r, conn := newTestRadio(t)
	writeFrame(t, conn, newRecordFrame("MW", '0', "07").String())
	assertNoReply(t, conn)
	if got := r.CurrentChannel(); got != 0 {
		t.Errorf("CurrentChannel = %d after an MW, want the unchanged 0", got)
	}
}

// --- MC: the selected channel (480:825-838) ---

// TestMC_ReadAnswersTheSelectedChannel. The answer frame is six bytes
// (480:838), its P1 is the printed constant "0: Always 0 for the TS-480
// (Memory bank number)." (480:827) and its P2 the two-digit channel number
// (480:830). THERE IS NO SPACE CONVENTION ON THIS RADIO: byte 3 is the same
// '0' at every channel, where the 590 pair's MC answer prints a space below
// channel 100 (590:1336-1337).
func TestMC_ReadAnswersTheSelectedChannel(t *testing.T) {
	_, conn := newTestRadio(t)

	if got, want := exchange(t, conn, "MC;"), "MC000;"; got != want {
		t.Errorf("MC; -> %q, want %q at construction", got, want)
	}

	writeFrame(t, conn, "MC007;")
	assertNoReply(t, conn)
	if got, want := exchange(t, conn, "MC;"), "MC007;"; got != want {
		t.Errorf("MC; -> %q after MC007;, want %q", got, want)
	}

	writeFrame(t, conn, "MC099;")
	assertNoReply(t, conn)
	if got, want := exchange(t, conn, "MC;"), "MC099;"; got != want {
		t.Errorf("MC; -> %q after MC099;, want %q", got, want)
	}
}

// TestMC_RefusesABankByteOtherThanZeroOrAMalformedSet. "0: Always 0 for the
// TS-480 (Memory bank number)." (480:827) is the whole of P1's legend, so
// the 590 pair's space spelling and any other digit are both refused.
func TestMC_RefusesABankByteOtherThanZeroOrAMalformedSet(t *testing.T) {
	r, conn := newTestRadio(t)
	for _, send := range []string{"MC 07;", "MC107;", "MCX07;", "MC0X7;", "MC07;", "MC0007;"} {
		assertRejected(t, conn, send)
	}
	if got := r.CurrentChannel(); got != 0 {
		t.Errorf("CurrentChannel = %d after the refused sets, want the unchanged 0", got)
	}
}

// TestMC_SelectsAnUnwrittenChannel: nothing in the MC block conditions the
// selection on the channel holding anything. What an unwritten channel HOLDS
// is A4 and unlifted on this row, but that is a question about MR's answer,
// not about whether MC accepts the number.
func TestMC_SelectsAnUnwrittenChannel(t *testing.T) {
	r, conn := newTestRadio(t)
	writeFrame(t, conn, "MC055;")
	assertNoReply(t, conn)
	if got := r.CurrentChannel(); got != 55 {
		t.Errorf("CurrentChannel = %d, want 55", got)
	}
}

// TestUpperASCII_LeavesNonASCIIAlone pins the fold against the swap that broke
// it: bytes.ToUpper is Unicode-aware and changes the LENGTH of what it is
// given — "\u0131" is the two bytes C4 B1 and uppercases to the one byte "I" —
// so a two-byte command name built from its result can panic on line noise,
// and a non-ASCII byte comes back rewritten. This fold touches ASCII and
// nothing else.
func TestUpperASCII_LeavesNonASCIIAlone(t *testing.T) {
	for _, tt := range []struct{ in, want [2]byte }{
		{[2]byte{'m', 'r'}, [2]byte{'M', 'R'}},
		{[2]byte{'M', 'r'}, [2]byte{'M', 'R'}},
		{[2]byte{0xC4, 0xB1}, [2]byte{0xC4, 0xB1}},
		{[2]byte{0x80, 'a'}, [2]byte{0x80, 'A'}},
		{[2]byte{'{', '`'}, [2]byte{'{', '`'}},
	} {
		if got := upperASCII(tt.in); got != tt.want {
			t.Errorf("upperASCII(% 02X) = % 02X, want % 02X", tt.in, got, tt.want)
		}
	}
}
