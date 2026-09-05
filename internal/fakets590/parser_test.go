// SPDX-License-Identifier: GPL-3.0-or-later

package fakets590

import (
	"strings"
	"testing"
)

// recordFrame is this test file's OWN assembler for the 50-byte memory
// record, written from the position rulers the two charts print — MR's
// answer (590:1440-1461) and MW's Set (590:1518-1536), which number the same
// fifty positions the same way — and NOT by calling buildMRAnswer. A builder
// with a bug must still be catchable, which it cannot be if the expectation
// comes from the same builder.
//
// Positions, 1-indexed as the charts number them:
//
//	1-2    the command name
//	3      P1   0: Simplex / 1: Split           (590:1441-1443)
//	4      P2   the channel's 100's digit       (590:1332-1337)
//	5-6    P3   the two-digit channel number    (590:1341-1343)
//	7-17   P4   frequency, 11 digits            (590:1455-1456)
//	18     P5   mode nibble                     (590:1458)
//	19     P6   data mode                       (590:1461-1462)
//	20     P7   tone mode                       (590:1464-1467)
//	21-22  P8   tone number                     (590:1469)
//	23-24  P9   CTCSS number                    (590:1471)
//	25-27  P10  "000: Always 000"               (590:1473)
//	28     P11  FILTER A/B                      (590:1476-1477)
//	29     P12  "0: Always 0"                   (590:1480)
//	30-38  P13  "000000000: Always 000000000"   (590:1482)
//	39-40  P14  00: FM Normal / 01: FM Narrow   (590:1484-1485)
//	41     P15  channel lockout                 (590:1487-1488)
//	42-49  P16  memory name, 8 bytes            (590:1490)
//	50     ';'                                  (590:1461)
type recordFrame struct {
	cmd      string // "MR" or "MW"
	p1       byte
	p2       byte
	p3       string
	freq     string
	mode     byte
	dataMode byte
	toneMode byte
	toneNo   string
	ctcssNo  string
	p10      string
	filter   byte
	p12      byte
	p13      string
	fmNarrow string
	lockout  byte
	name     string
}

// newRecordFrame is one unremarkable populated record: the book's printed
// example frequency (590:965), FM (590:1358), everything else the printed
// zero of its own legend, and the blank eight-space name (590:1492-1493).
func newRecordFrame(cmd string, p1, p2 byte, p3 string) recordFrame {
	return recordFrame{
		cmd:      cmd,
		p1:       p1,
		p2:       p2,
		p3:       p3,
		freq:     "00014195000",
		mode:     '4',
		dataMode: '0',
		toneMode: '0',
		toneNo:   "00",
		ctcssNo:  "00",
		p10:      "000",
		filter:   '0',
		p12:      '0',
		p13:      "000000000",
		fmNarrow: "00",
		lockout:  '0',
		name:     "        ",
	}
}

func (f recordFrame) String() string {
	var b strings.Builder
	b.WriteString(f.cmd)
	b.WriteByte(f.p1)
	b.WriteByte(f.p2)
	b.WriteString(f.p3)
	b.WriteString(f.freq)
	b.WriteByte(f.mode)
	b.WriteByte(f.dataMode)
	b.WriteByte(f.toneMode)
	b.WriteString(f.toneNo)
	b.WriteString(f.ctcssNo)
	b.WriteString(f.p10)
	b.WriteByte(f.filter)
	b.WriteByte(f.p12)
	b.WriteString(f.p13)
	b.WriteString(f.fmNarrow)
	b.WriteByte(f.lockout)
	b.WriteString(f.name)
	b.WriteByte(';')
	return b.String()
}

// TestRecordFrame_IsFiftyBytes guards the assembler above: a test fixture
// that had drifted from the chart would make every comparison below agree
// with the wrong thing.
func TestRecordFrame_IsFiftyBytes(t *testing.T) {
	got := newRecordFrame("MR", '0', ' ', "07").String()
	if len(got) != 50 {
		t.Fatalf("the test assembler produced %d bytes (%q), want the 50 the charts count (590:1459-1461)", len(got), got)
	}
}

// --- MR: the memory read (590:1438-1493) ---

// TestMR_ReadFrameIsSevenBytes pins the request grammar: "M R P1 P2 P3 P3 ;"
// — seven positions (590:1440-1442). The chart's own terminator cell prints
// ':' rather than ';' at that position, a print defect core/kw/doc.go's
// errata schedule records as E1; the terminator is the ';' the front matter
// requires of every command (590:87-91).
func TestMR_ReadFrameIsSevenBytes(t *testing.T) {
	_, conn := newTestRadio(t, RowSG)

	got := exchange(t, conn, "MR0 00;")
	if len(got) != 50 {
		t.Fatalf("MR0 00; -> %d bytes, want the 50-byte answer", len(got))
	}

	for _, bad := range []string{"MR;", "MR0;", "MR0 0;", "MR0 000;"} {
		assertRejected(t, conn, bad)
	}
}

// TestMR_AnswersThePopulatedRecordByteForByte compares the fake's answer with
// a frame this file assembles from the chart's own ruler.
func TestMR_AnswersThePopulatedRecordByteForByte(t *testing.T) {
	_, conn := newTestRadio(t, RowSG)

	want := newRecordFrame("MR", '0', ' ', "00").String()
	if got := exchange(t, conn, "MR0 00;"); got != want {
		t.Errorf("MR0 00; ->\n %q\nwant\n %q", got, want)
	}
}

// TestMR_AnswersASpaceForTheHundredsDigitBelow100 pins the one place this
// fake's ANSWER differs from what core/kw builds. MR's P2 cell says
// "Channel number (refer to the MC command)" (590:1452-1453), and MC's own
// chart says "For a response command, a space is entered for a channel
// number less than 100" (590:1336-1337). So this fake answers a space and
// the codec — which emits '0' and accepts either (A10) — must accept it. A
// fake that echoed the request's byte would never exercise that tolerance.
func TestMR_AnswersASpaceForTheHundredsDigitBelow100(t *testing.T) {
	_, conn := newTestRadio(t, RowSG)

	for _, send := range []string{"MR0000;", "MR0 00;"} {
		got := exchange(t, conn, send)
		if len(got) != 50 {
			t.Fatalf("%s -> %q, want a 50-byte answer", send, got)
		}
		if got[3] != ' ' {
			t.Errorf("%s -> byte 4 is %q, want a space (590:1336-1337)", send, got[3])
		}
	}

	// Above 99 the digit is present and is the hundreds digit itself.
	got := exchange(t, conn, "MR0100;")
	if got[3] != '1' {
		t.Errorf("MR0100; -> byte 4 is %q, want '1'", got[3])
	}
}

// TestMR_AcceptsEitherSpellingOfTheHundredsDigitOnARequest is A10's read
// half, played: "When entering a setting command, enter 0 or a space for a
// channel number less than 100" (590:1334-1335), which MR's P2 refers to.
// Both spellings must answer the same record.
func TestMR_AcceptsEitherSpellingOfTheHundredsDigitOnARequest(t *testing.T) {
	_, conn := newTestRadio(t, RowSG)

	zero := exchange(t, conn, "MR0000;")
	space := exchange(t, conn, "MR0 00;")
	if zero != space {
		t.Errorf("MR0000; -> %q but MR0 00; -> %q — the two spellings name one channel (590:1334-1335)", zero, space)
	}
}

// TestMR_AnUnwrittenChannelAnswersTheEmptyRecord is A18a, which is
// DOCUMENTARY FACT on this pair and not an assumption: "If the selected
// channel is empty, P4 ~ P15 will be 0 and P16 will be blank."
// (590:1492-1493). The whole of P4-P15 is zero — bytes 7 to 41 — and the
// eight name bytes are spaces.
//
// It is the exchange core/driver/ts590 must read as "empty channel" and not
// as "parse error", which is the pin the spec asks the driver's own test to
// carry.
func TestMR_AnUnwrittenChannelAnswersTheEmptyRecord(t *testing.T) {
	_, conn := newTestRadio(t, RowSG)

	got := exchange(t, conn, "MR0 55;")
	if len(got) != 50 {
		t.Fatalf("MR0 55; -> %q, want a 50-byte answer", got)
	}
	// Bytes 7-41 inclusive, 1-indexed: the whole P4-P15 run.
	for i := 7; i <= 41; i++ {
		if got[i-1] != '0' {
			t.Errorf("byte %d of the empty answer is %q, want '0' (590:1492-1493)", i, got[i-1])
		}
	}
	if name := got[41:49]; name != "        " {
		t.Errorf("the empty answer's name field is %q, want eight spaces (590:1492-1493)", name)
	}
	if got[2] != '0' || got[3] != ' ' || got[4:6] != "55" {
		t.Errorf("the empty answer names channel %q, want P1 '0' and channel 55", got[2:6])
	}
}

// TestMR_RefusesASlotOutsideTheServedSpace. The fake serves 000-109 on BOTH
// rows: 000-099 ordinary memory and 100-109 the section-defined channels
// P00 ~ P09 (590:1341, 590:1345). The SG's printed 110-119 (590:1346-1347)
// are DELIBERATELY ABSENT under Stuart's ruling of 05/09/2026 — doc.go's
// register entry THE SG'S EXTENSION CHANNELS ARE NOT SERVED — so both rows
// refuse them identically.
func TestMR_RefusesASlotOutsideTheServedSpace(t *testing.T) {
	for _, row := range []Row{RowS, RowSG} {
		t.Run(row.String(), func(t *testing.T) {
			_, conn := newTestRadio(t, row)
			for _, send := range []string{"MR0110;", "MR0115;", "MR0119;", "MR0200;", "MR0999;"} {
				assertRejected(t, conn, send)
			}
			// The boundary below it answers, on both rows.
			if got := exchange(t, conn, "MR0109;"); len(got) != 50 {
				t.Errorf("MR0109; -> %q, want a 50-byte answer", got)
			}
		})
	}
}

// TestMR_RefusesAMalformedRequest covers each field of the seven-byte read.
func TestMR_RefusesAMalformedRequest(t *testing.T) {
	_, conn := newTestRadio(t, RowSG)
	for _, send := range []string{
		"MR2 00;", // P1 outside "0: Simplex / 1: Split" (590:1441-1443)
		"MRX 00;",
		"MR0X00;", // P2 neither a digit nor a space (590:1334-1337)
		"MR0 X0;", // P3 not two digits (590:1341-1343)
		"MR0 0X;",
	} {
		assertRejected(t, conn, send)
	}
}

// TestMR_ServesBothHalvesOfASectionDefinedChannel: on 100-109 the P1 byte
// selects the START or the END frequency — "When reading the start frequency
// of a section defined channel, enter 0 for parameter P1. When reading the
// end frequency, enter 1." (590:1449-1451) — and the default image populates
// both halves of channel 100 so a driver reading the pair reads two stored
// records rather than one and one silence.
func TestMR_ServesBothHalvesOfASectionDefinedChannel(t *testing.T) {
	_, conn := newTestRadio(t, RowSG)

	for _, p1 := range []string{"0", "1"} {
		got := exchange(t, conn, "MR"+p1+"100;")
		if len(got) != 50 {
			t.Fatalf("MR%s100; -> %q, want a 50-byte answer", p1, got)
		}
		if string(got[2]) != p1 {
			t.Errorf("MR%s100; answered with P1 %q, want %q", p1, got[2], p1)
		}
		if freq := got[6:17]; freq == "00000000000" {
			t.Errorf("MR%s100; answered an EMPTY record — the default image populates both halves of the section channel", p1)
		}
	}
}

// TestMR_TheSecondHalfOfASimplexChannelIsEmpty. What an MR with P1=1 answers
// on a channel with no transmit half is unprinted on either radio (A9), so
// this fake gives it the one shape the book DOES describe for a channel
// holding nothing — the empty record (590:1492-1493) — rather than inventing
// a rejection or echoing the receive frequency back as a transmit one.
// doc.go's register entry THE SECOND HALF OF A CHANNEL WITH NO STORED
// TRANSMIT RECORD.
func TestMR_TheSecondHalfOfASimplexChannelIsEmpty(t *testing.T) {
	_, conn := newTestRadio(t, RowSG)

	got := exchange(t, conn, "MR1 00;")
	if len(got) != 50 {
		t.Fatalf("MR1 00; -> %q, want a 50-byte answer", got)
	}
	if freq := got[6:17]; freq != "00000000000" {
		t.Errorf("MR1 00; answered frequency %q, want the empty record's zeros", freq)
	}
}

// TestWithMemoryReadUnsupported_RefusesEveryMR makes decision 5's rule
// reachable: a "?;" is a definitive rejection, never retried and never read
// as absence. The option refuses every MR while MW and MC stay untouched, so
// the driver's typed whole-read failure can be driven through a real fake.
//
// It is NOT a claim that any TS-590 refuses MR. It plays the second cause the
// error table itself prints — "Command was not executed due to the current
// status of the transceiver (even though the command syntax was correct)"
// (590:101-103).
func TestWithMemoryReadUnsupported_RefusesEveryMR(t *testing.T) {
	_, conn := newTestRadio(t, RowSG, WithMemoryReadUnsupported())

	assertRejected(t, conn, "MR0 00;")
	assertRejected(t, conn, "MR0 55;")

	// MC still answers, and MW is still accepted: only the read is refused.
	if got, want := exchange(t, conn, "MC;"), "MC 00;"; got != want {
		t.Errorf("MC; -> %q, want %q", got, want)
	}
	writeFrame(t, conn, newRecordFrame("MW", '0', '0', "07").String())
	assertNoReply(t, conn)
}

// --- MW: the memory write (590:1516-1581) ---

// TestMW_AcceptsExactlyFiftyBytesAndAnswersNothing. The Set chart counts
// fifty positions (590:1518-1536) and prints no Read and no Answer row at
// all, so an accepted write is silent — doc.go's register entry AN ACCEPTED
// SET PRODUCES NO REPLY.
func TestMW_AcceptsExactlyFiftyBytesAndAnswersNothing(t *testing.T) {
	r, conn := newTestRadio(t, RowSG)

	f := newRecordFrame("MW", '0', '0', "07")
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

	// And it reads back, byte for byte, with the ANSWER's space convention
	// in P2.
	want := f
	want.cmd = "MR"
	want.p2 = ' '
	if back := exchange(t, conn, "MR0 07;"); back != want.String() {
		t.Errorf("MR0 07; ->\n %q\nwant\n %q", back, want.String())
	}
}

// TestMW_RefusesTheEraseForm is a standing rule of this repository made
// mechanical. The book describes an erase: "If you do not specify one digit
// in P16 and execute all the parameters from P4 to P15 set to 0, the
// channels specified by P2 and P3 will be erased." (590:1579-1581) — a
// SHORTER frame, 42 bytes. This programme builds no erase on any radio, and
// this fake accepts MW at exactly the 50-byte width its chart counts, so the
// erase form dies as a width violation and never reaches any state.
func TestMW_RefusesTheEraseForm(t *testing.T) {
	r, conn := newTestRadio(t, RowSG)

	full := newRecordFrame("MW", '0', '0', "07").String()
	erase := full[:41] + ";" // the record without its eight name bytes
	if len(erase) != 42 {
		t.Fatalf("the erase-shaped fixture is %d bytes, want 42", len(erase))
	}
	assertRejected(t, conn, erase)

	if _, ok := r.ChannelState(7, HalfRXOrStart); ok {
		t.Error("the refused erase-shaped frame reached the record map")
	}
}

// TestMW_RefusesEveryOtherWidth.
func TestMW_RefusesEveryOtherWidth(t *testing.T) {
	_, conn := newTestRadio(t, RowSG)
	full := newRecordFrame("MW", '0', '0', "07").String()
	assertRejected(t, conn, "MW;")
	assertRejected(t, conn, full[:len(full)-2]+";")
	assertRejected(t, conn, full[:len(full)-1]+" ;")
}

// TestMW_RefusesAFieldOutsideItsPrintedLegend is the SET-DIRECTION FIELD
// STRICTNESS register entry, exercised field by field. Every refusal here is
// ASSUMED to be what the radio itself enforces; what the fake must not do is
// silently normalise a byte, because a driver that sent one would then pass
// its own tests and fail on hardware.
func TestMW_RefusesAFieldOutsideItsPrintedLegend(t *testing.T) {
	base := func() recordFrame { return newRecordFrame("MW", '0', '0', "07") }
	tests := []struct {
		name  string
		mutis func(f *recordFrame)
	}{
		{"P1 outside 0/1 (590:1519-1520)", func(f *recordFrame) { f.p1 = '2' }},
		{"P2 neither digit nor space (590:1334-1337)", func(f *recordFrame) { f.p2 = 'X' }},
		{"P3 not two digits (590:1341-1343)", func(f *recordFrame) { f.p3 = "X7" }},
		{"P4 not eleven digits (590:1455-1456)", func(f *recordFrame) { f.freq = "0001419500X" }},
		{"P5 outside the MD legend (590:1353-1363)", func(f *recordFrame) { f.mode = 'A' }},
		{"P6 outside the DA legend (590:447-449)", func(f *recordFrame) { f.dataMode = '2' }},
		{"P7 outside 0..3 (590:1464-1467)", func(f *recordFrame) { f.toneMode = '4' }},
		{"P8 not two digits (590:1469)", func(f *recordFrame) { f.toneNo = "0X" }},
		{"P9 not two digits (590:1471)", func(f *recordFrame) { f.ctcssNo = "X0" }},
		{"P10 not the printed constant (590:1473)", func(f *recordFrame) { f.p10 = "001" }},
		{"P11 outside 0/1 (590:1476-1477)", func(f *recordFrame) { f.filter = '2' }},
		{"P12 not the printed constant (590:1480)", func(f *recordFrame) { f.p12 = '1' }},
		{"P13 not the printed constant (590:1482)", func(f *recordFrame) { f.p13 = "000000001" }},
		{"P14 outside 00/01 (590:1484-1485)", func(f *recordFrame) { f.fmNarrow = "02" }},
		{"P15 outside 0/1 (590:1487-1488)", func(f *recordFrame) { f.lockout = '2' }},
		{"P16 outside printable ASCII (590:1577, A2)", func(f *recordFrame) { f.name = "AB\x01     " }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, conn := newTestRadio(t, RowSG)
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

// TestMW_StoresAModeNibbleTheLegendCallsASettingFailure. Nibbles 0 and 8 are
// printed as "None (setting failure)" (590:1353, 590:1362) and NOTHING says
// what a radio does with a Set carrying one — that is A18b, and its lift is a
// hardware trial. So this fake stores the nibble rather than inventing a
// refusal, which is what lets a test drive core/kw's own build refusal and
// its parse behaviour against a real fake. doc.go's register entry A SET
// CARRYING A "NONE" MODE NIBBLE IS STORED.
func TestMW_StoresAModeNibbleTheLegendCallsASettingFailure(t *testing.T) {
	for _, nibble := range []byte{'0', '8'} {
		r, conn := newTestRadio(t, RowSG)
		f := newRecordFrame("MW", '0', '0', "07")
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

// TestMW_StoresAToneIndexOutsideThePrintedRange. TN's own chart prints "An
// entered value of 43 or higher results in an error" (590:2309) for the TN
// command; whether the same rule holds inside a memory frame is A21, and
// unlifted. Refusing here would assert A21 as a fact about the radio, so the
// fake stores the index — and can therefore serve one the codec must refuse
// to parse. doc.go's register entry TONE INDICES ARE STORED, NOT
// RANGE-CHECKED.
func TestMW_StoresAToneIndexOutsideThePrintedRange(t *testing.T) {
	r, conn := newTestRadio(t, RowSG)
	f := newRecordFrame("MW", '0', '0', "07")
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

// TestMW_AcceptsByte28EitherWayOnBothRows. The book prints one P11 legend for
// both siblings — "0: FILTER A / 1: FILTER B" (590:1476-1477) — and scopes
// the "always 0" sentence to the TS-590S's firmware 1.xx (590:1478). Tying
// this fake's acceptance to its firmware string would be the fake asserting
// A14; that decision belongs to the driver's write path, which is where the
// spec puts it. doc.go's register entry BYTE 28 IS ACCEPTED EITHER WAY ON
// BOTH ROWS.
func TestMW_AcceptsByte28EitherWayOnBothRows(t *testing.T) {
	for _, row := range []Row{RowS, RowSG} {
		for _, filter := range []byte{'0', '1'} {
			r, conn := newTestRadio(t, row, WithFirmwareVersion("1.00"))
			f := newRecordFrame("MW", '0', '0', "07")
			f.filter = filter
			writeFrame(t, conn, f.String())
			assertNoReply(t, conn)

			got, ok := r.ChannelState(7, HalfRXOrStart)
			if !ok {
				t.Fatalf("%v, filter %q: channel 7 holds no record", row, filter)
			}
			if got.Filter != filter {
				t.Errorf("%v: stored byte 28 %q, want %q", row, got.Filter, filter)
			}
		}
	}
}

// TestMW_TheTwoHalvesAreSeparateRecords. P1 selects which frequency a write
// carries — "When registering a split channel, set parameter P1 to 1 (set the
// transmission frequency and mode). The reception frequency and mode are not
// updated at this time." (590:1525-1527) — so a P1=1 write must not disturb
// the P1=0 record.
func TestMW_TheTwoHalvesAreSeparateRecords(t *testing.T) {
	r, conn := newTestRadio(t, RowSG)

	rx := newRecordFrame("MW", '0', '0', "07")
	rx.freq = "00007100000"
	writeFrame(t, conn, rx.String())
	assertNoReply(t, conn)

	tx := newRecordFrame("MW", '1', '0', "07")
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
	r, conn := newTestRadio(t, RowSG)
	writeFrame(t, conn, newRecordFrame("MW", '0', '0', "07").String())
	assertNoReply(t, conn)
	if got := r.CurrentChannel(); got != 0 {
		t.Errorf("CurrentChannel = %d after an MW, want the unchanged 0", got)
	}
}

// --- MC: the selected channel (590:1329-1347) ---

// TestMC_ReadAnswersTheSelectedChannelWithMCsSpaceConvention. The answer
// frame is six bytes (590:1341) and its P1 is a SPACE below 100 —
// "For a response command, a space is entered for a channel number less than
// 100" (590:1336-1337) — which is printed rather than assumed.
func TestMC_ReadAnswersTheSelectedChannelWithMCsSpaceConvention(t *testing.T) {
	_, conn := newTestRadio(t, RowSG)

	if got, want := exchange(t, conn, "MC;"), "MC 00;"; got != want {
		t.Errorf("MC; -> %q, want %q at construction", got, want)
	}

	writeFrame(t, conn, "MC007;")
	assertNoReply(t, conn)
	if got, want := exchange(t, conn, "MC;"), "MC 07;"; got != want {
		t.Errorf("MC; -> %q after MC007;, want %q", got, want)
	}

	writeFrame(t, conn, "MC105;")
	assertNoReply(t, conn)
	if got, want := exchange(t, conn, "MC;"), "MC105;"; got != want {
		t.Errorf("MC; -> %q after MC105;, want %q", got, want)
	}
}

// TestMC_SetAcceptsEitherSpellingOfTheHundredsDigit (590:1334-1335).
func TestMC_SetAcceptsEitherSpellingOfTheHundredsDigit(t *testing.T) {
	for _, send := range []string{"MC007;", "MC 07;"} {
		r, conn := newTestRadio(t, RowSG)
		writeFrame(t, conn, send)
		assertNoReply(t, conn)
		if got := r.CurrentChannel(); got != 7 {
			t.Errorf("%s -> CurrentChannel %d, want 7", send, got)
		}
	}
}

// TestMC_RefusesAnOutOfDomainOrMalformedSet.
func TestMC_RefusesAnOutOfDomainOrMalformedSet(t *testing.T) {
	r, conn := newTestRadio(t, RowSG)
	for _, send := range []string{"MC110;", "MC119;", "MC200;", "MC0X7;", "MCX07;", "MC07;", "MC0007;"} {
		assertRejected(t, conn, send)
	}
	if got := r.CurrentChannel(); got != 0 {
		t.Errorf("CurrentChannel = %d after the refused sets, want the unchanged 0", got)
	}
}

// TestMC_SelectsAnEmptyChannel: nothing in the MC block conditions the
// selection on the channel holding anything, and an empty channel is a
// documented, answerable state on this pair (590:1492-1493), so the fake
// selects it.
func TestMC_SelectsAnEmptyChannel(t *testing.T) {
	r, conn := newTestRadio(t, RowSG)
	writeFrame(t, conn, "MC055;")
	assertNoReply(t, conn)
	if got := r.CurrentChannel(); got != 55 {
		t.Errorf("CurrentChannel = %d, want 55", got)
	}
}
