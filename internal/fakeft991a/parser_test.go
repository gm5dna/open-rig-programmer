// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft991a

import (
	"testing"
)

// combinedFrame is one combined MT frame's content in WIRE bytes, field by
// field, so a test states what crosses the wire rather than what a builder
// would produce for it. It serves BOTH directions: a Set and an Answer are one
// 41-position chart (ft991a_layout.txt:998-1033), differing only in the P7 byte
// each carries — and it serves MR too, whose 28-position Answer chart (965-981)
// is this frame's first 27 positions under a different prefix.
//
// The zero value is not a valid frame: every field is set explicitly at each
// call site, which is the point — see frame.
//
// A fixture built by the code under test would pin the parser against the
// builder, and the two would agree about a wrong offset exactly as happily as
// about a right one. This assembler is written from the chart instead.
type combinedFrame struct {
	slot     string // P1, positions 3-5
	freq     string // P2, positions 6-14, 9 digits
	clarSign byte   // P3 sign, position 15
	clarMag  string // P3 magnitude, positions 16-19
	rxClar   byte   // P4, position 20
	txClar   byte   // P5, position 21 — this radio's LIVE TX clarifier flag
	mode     byte   // P6, position 22
	kind     byte   // P7, position 23
	ctcss    byte   // P8, position 24
	p9       string // P9, positions 25-26; "" means the documented fixed "00"
	shift    byte   // P10, position 27
	p11      byte   // P11, position 28; 0 means the documented fixed '0'
	tag      string // P12, positions 29-40, space-padded to 12
}

// frame assembles the 41-byte combined MT frame BY POSITION, from this
// manual's own MT chart (rev 1711-D, ft991a_layout.txt:998-1033) as counted
// twice by evidence leg G (core/cat/ft991a/testdata/mt-vectors.golden's
// position-by-position field map: "pos 1 M · 2 T · 3-5 P1 · 6-14 P2 · 15-19 P3
// · 20 P4 · 21 P5 · 22 P6 · 23 P7 · 24 P8 · 25-26 P9 · 27 P10 · 28 P11 ·
// 29-40 P12 · 41 ;").
//
// Unfilled positions are left as a visible '?' sentinel, so a position this
// assembler forgets shows up as a rejection or a mismatch rather than as an
// accidental zero that happens to be valid somewhere.
//
// The literal 41 belongs in this file for the same reason it must not appear
// in the package's own code: here it is the CHART being asserted.
func (f combinedFrame) frame() string {
	b := make([]byte, 41)
	for i := range b {
		b[i] = '?'
	}
	copy(b[0:2], "MT")
	copy(b[2:5], f.slot)
	copy(b[5:14], f.freq)
	b[14] = f.clarSign
	copy(b[15:19], f.clarMag)
	b[19] = f.rxClar
	b[20] = f.txClar
	b[21] = f.mode
	b[22] = f.kind
	b[23] = f.ctcss
	p9 := f.p9
	if p9 == "" {
		p9 = "00"
	}
	copy(b[24:26], p9)
	b[26] = f.shift
	p11 := f.p11
	if p11 == 0 {
		p11 = '0'
	}
	b[27] = p11
	tagField := b[28:40]
	n := copy(tagField, f.tag)
	for i := n; i < len(tagField); i++ {
		tagField[i] = ' '
	}
	b[40] = ';'
	return string(b)
}

// mrFrame assembles the 28-byte MR answer BY POSITION, from the MR Answer
// chart (ft991a_layout.txt:965-981, counted by evidence leg G at
// core/cat/ft991a/testdata/mr-vectors.golden): "MR" + the same shared field
// block + ';'. Only the fields the block carries are read; P11 and the tag are
// outside it.
func (f combinedFrame) mrFrame() string {
	c := f.frame()
	// The block occupies the same positions in both frames — that is the whole
	// point of one field grid under several prefixes, which is what
	// core/cat/ft991a/doc.go's reused-command verification established — so the
	// MR answer is the combined frame's first 27 positions with "MR" for "MT"
	// and a ';' appended.
	return "MR" + c[2:27] + ";"
}

// ordinaryChannel is an unremarkable populated channel's field values: 14.250
// MHz USB, clarifier -150 Hz with BOTH clarifier flags on, CTCSS ENC/DEC, PLUS
// shift, tag "CALLING". Every value is inside this radio's own printed
// vocabularies. The kind is the field a caller sets per direction.
//
// BOTH CLARIFIER FLAGS ARE ON, deliberately: position 21 is a LIVE TX
// clarifier flag on this radio (ft991a_layout.txt:1004) where the FT-891
// prints "P5 0: (Fixed)", so an ordinary channel that left it '0' would leave
// this radio's divergence from that sibling unexercised by every test using
// this fixture.
func ordinaryChannel(slot string, kind byte) combinedFrame {
	return combinedFrame{
		slot: slot, freq: "014250000",
		clarSign: '-', clarMag: "0150", rxClar: '1', txClar: '1',
		mode: '2', kind: kind, ctcss: '1', shift: '1',
		tag: "CALLING",
	}
}

// ordinaryState is the MemState this fake must hold for ordinaryChannel — the
// same values, spelled independently of any parse.
func ordinaryState() MemState {
	return MemState{
		Freq: "014250000", ClarSign: '-', ClarMag: "0150",
		RXClar: true, TXClar: true,
		Mode: '2', Kind: kindMemory, CTCSS: '1', Shift: '1',
		Tag: "CALLING",
	}
}

// TestCombinedFrameAssembler_MatchesAHandWrittenLiteral pins the assembler
// above against frames written out character by character, so every test using
// it rests on something checked rather than on a helper checked by nothing.
//
// The literals record the ONE byte that differs between a Set of this channel
// and the answer a read of it produces:
//
//   - position 23, P7. The Set's is the chart's fixed '0' and the answer's is
//     '1', Memory — and on this radio BOTH are printed, in one legend:
//     "P7 Set: 0: (Fixed) / Read: 0: VFO 1: Memory" (ft991a_layout.txt:1009).
//     The FT-891 prints no read vocabulary beside MT at all and has to read
//     the answer's domain across from MR; here there is nothing to infer.
//   - nothing else. Position 21 is '1' in BOTH directions, because P5 is the
//     LIVE TX clarifier flag on this radio (971, 1004, 1042, 787, 1122) and
//     the fake stores and answers what a Set carried.
func TestCombinedFrameAssembler_MatchesAHandWrittenLiteral(t *testing.T) {
	const wantSet = "MT001014250000-0150112010010CALLING     ;"
	const wantAnswer = "MT001014250000-0150112110010CALLING     ;"
	const wantMR = "MR001014250000-015011211001;"

	if got := ordinaryChannel("001", mtSetKindFixed).frame(); got != wantSet {
		t.Errorf("Set frame:\n got %q\nwant %q", got, wantSet)
	}
	if got := ordinaryChannel("001", kindMemory).frame(); got != wantAnswer {
		t.Errorf("Answer frame:\n got %q\nwant %q", got, wantAnswer)
	}
	if got := ordinaryChannel("001", kindMemory).mrFrame(); got != wantMR {
		t.Errorf("MR answer frame:\n got %q\nwant %q", got, wantMR)
	}

	// The counted widths, asserted rather than assumed — evidence leg G's two
	// independent counts (mt-vectors.golden and mr-vectors.golden headers).
	if got := len(wantSet); got != 41 {
		t.Errorf("the combined MT frame is %d bytes, want the counted 41", got)
	}
	if got := len(wantMR); got != 28 {
		t.Errorf("the MR answer frame is %d bytes, want the counted 28", got)
	}
}

// with returns a copy of f with mutate applied — for stating "this frame, but
// one field wrong".
func (f combinedFrame) with(mutate func(*combinedFrame)) combinedFrame {
	mutate(&f)
	return f
}

// --- The slot grammar ---

// TestParseSlotForm covers every class this radio's slot legends print, and
// the ones they do not.
//
// THE PMS SLOTS ARE DECIMAL CHANNEL NUMBERS ON THIS RADIO. The MC legend runs
// "100: P-1L 101: P-1U ~ 116: P-9L 117: P-9U" (ft991a_layout.txt:916), so
// "P1L" — every registered sibling's spelling, and internal/fakeft891's — is
// NOT a slot form here and must be refused. There is no 5 MHz bank and no EMG
// channel either: "5xx", "5 MHz" and "EMG" appear in no slot legend of this
// manual, which is why "501" and "EMG" are invalid below.
func TestParseSlotForm(t *testing.T) {
	tests := []struct {
		slot string
		want slotKind
	}{
		{"001", slotMemory},
		{"099", slotMemory},
		{"100", slotPMS}, // P-1L, the first PMS slot
		{"117", slotPMS}, // P-9U, the last
		{"000", slotNone},

		{"118", slotInvalid}, // one past the printed ceiling
		{"999", slotInvalid},
		{"P1L", slotInvalid}, // the SIBLINGS' PMS spelling, not this radio's
		{"P9U", slotInvalid},
		{"501", slotInvalid}, // no 5 MHz bank in any legend of this manual
		{"EMG", slotInvalid}, // no emergency channel either
		{"", slotInvalid},
		{"01", slotInvalid},
		{"0011", slotInvalid},
		{"00A", slotInvalid},
	}
	for _, tt := range tests {
		if got := parseSlotForm(tt.slot); got != tt.want {
			t.Errorf("parseSlotForm(%q) = %v, want %v", tt.slot, got, tt.want)
		}
	}
}

// --- MR: MEMORY CHANNEL READ ---

// TestMRRead_ServesEveryReadableSlotClass holds MR against BOTH classes its
// own legend names — "P0/1 001-117 (Memory Channel)"
// (ft991a_layout.txt:966) — which on this radio is memory and PMS and nothing
// else.
func TestMRRead_ServesEveryReadableSlotClass(t *testing.T) {
	_, conn := newTestRadio(t)
	for _, slot := range []string{"001", "002", "100", "117"} {
		got := exchange(t, conn, "MR"+slot+";")
		if len(got) != 28 || got[:2] != "MR" || got[2:5] != slot {
			t.Errorf("MR%s; -> %q, want a 28-byte MR answer for that slot", slot, got)
		}
	}
}

func TestMRRead_EmptyAndMalformed(t *testing.T) {
	_, conn := newTestRadio(t)
	// Grammatically valid, no state — the EMPTY-SLOT ANSWERS entry. Both
	// classes: a memory channel and one of the seven PMS pairs the default
	// image deliberately leaves empty.
	assertRejected(t, conn, "MR050;")
	assertRejected(t, conn, "MR110;")
	// Grammatically invalid, one past the printed ceiling.
	assertRejected(t, conn, "MR118;")
	// The siblings' PMS spelling, which this radio's legend does not print.
	assertRejected(t, conn, "MRP1L;")
	// The answer-only none form is never a valid request.
	assertRejected(t, conn, "MR000;")
	assertRejected(t, conn, "MR;")
	assertRejected(t, conn, "MR01;")
}

// TestMR_HasNoSetDirection holds the availability row: MR is "X O O X"
// (ft991a_layout.txt:178), so a 28-byte MR frame in the shape of its own
// Answer is an unknown frame rather than a write.
func TestMR_HasNoSetDirection(t *testing.T) {
	r, conn := newTestRadio(t)
	setShaped := ordinaryChannel("003", mtSetKindFixed).mrFrame()
	assertRejected(t, conn, setShaped)
	if _, ok := r.SlotState("003"); ok {
		t.Error("an MR frame in the Set shape created a channel — MR has no Set direction on this radio")
	}
}

func mustBeAnMRAnswer(t *testing.T, got string) string {
	t.Helper()
	if len(got) != 28 || got[:2] != "MR" || got[27] != ';' {
		t.Fatalf("not a 28-byte MR answer: %q", got)
	}
	return got
}

// TestMRAnswer_CarriesTheLiveTXClarifierFlag is this radio's divergence from
// the FT-891 at position 21, from the answer side: P5 is
// `0: TX CLAR "OFF" 1: TX CLAR "ON"` on every block that carries the field
// (ft991a_layout.txt:971, 1004, 1042, 787, 1122), so what a Set stored is what
// a read answers, and a crafted slot can answer either value.
func TestMRAnswer_CarriesTheLiveTXClarifierFlag(t *testing.T) {
	on := ordinaryState()
	off := ordinaryState()
	off.TXClar = false

	_, conn := newTestRadio(t,
		WithSlot("011", on),
		WithSlot("012", off),
	)
	if got := mustBeAnMRAnswer(t, exchange(t, conn, "MR011;")); got[20] != '1' {
		t.Errorf("MR011; position 21 = %q, want '1' (TX CLAR ON)", got[20])
	}
	if got := mustBeAnMRAnswer(t, exchange(t, conn, "MR012;")); got[20] != '0' {
		t.Errorf("MR012; position 21 = %q, want '0' (TX CLAR OFF)", got[20])
	}
}

// TestMRAnswer_MatchesTheHandWrittenFrame is the whole answer, byte for byte,
// against a frame this test assembled from the chart.
func TestMRAnswer_MatchesTheHandWrittenFrame(t *testing.T) {
	_, conn := newTestRadio(t, WithSlot("011", ordinaryState()))
	want := ordinaryChannel("011", kindMemory).mrFrame()
	if got := exchange(t, conn, "MR011;"); got != want {
		t.Errorf("MR011; ->\n got %q\nwant %q", got, want)
	}
}

// --- MT: MEMORY CHANNEL WRITE/TAG, the combined form ---

func mustBeACombinedAnswer(t *testing.T, got string) string {
	t.Helper()
	if len(got) != 41 || got[:2] != "MT" || got[40] != ';' {
		t.Fatalf("not a 41-byte combined MT answer: %q", got)
	}
	return got
}

// TestMTRead_IsAnsweredForMemoryAndPMS holds the read direction against MT's
// own legend, the whole "P0/1 001-117 (Memory Channel)" span
// (ft991a_layout.txt:999) — memory AND PMS, which is what the driver's read
// path walks.
func TestMTRead_IsAnsweredForMemoryAndPMS(t *testing.T) {
	_, conn := newTestRadio(t)
	for _, slot := range []string{"001", "002", "100", "117"} {
		got := mustBeACombinedAnswer(t, exchange(t, conn, "MT"+slot+";"))
		if got[2:5] != slot {
			t.Errorf("MT%s; answered for slot %q", slot, got[2:5])
		}
	}
}

// TestMTAnswer_CarriesP11AsTheFixedZero is the inversion of the FT-891's most
// distinctive cell. MT's P11 legend prints "0: (Fixed)"
// (ft991a_layout.txt:1015) where that radio prints `0: TAG "OFF" 1: TAG "ON"`,
// so byte 28 is SCHEMA here and there is no per-channel display flag to store.
//
// The second half is why MemState carries a P11 field at all: a test may craft
// the answer this radio is assumed never to give, so a driver's parse-error
// path is reachable through a real fake.
func TestMTAnswer_CarriesP11AsTheFixedZero(t *testing.T) {
	crafted := ordinaryState()
	crafted.P11 = '1'

	_, conn := newTestRadio(t,
		WithSlot("011", ordinaryState()),
		WithSlot("012", crafted),
	)
	if got := mustBeACombinedAnswer(t, exchange(t, conn, "MT011;")); got[27] != '0' {
		t.Errorf("MT011; position 28 = %q, want the schema's fixed '0'", got[27])
	}
	if got := mustBeACombinedAnswer(t, exchange(t, conn, "MT012;")); got[27] != '1' {
		t.Errorf("MT012; position 28 = %q, want the crafted '1'", got[27])
	}
}

// TestMTRead_TagFieldIsSpacePaddedAndAllFillIsNoTag holds both halves of the
// register entry THE TAG IS STORED TRIMMED AND ANSWERED PADDED, with the fill
// byte the DIALECT's own register supplies.
func TestMTRead_TagFieldIsSpacePaddedAndAllFillIsNoTag(t *testing.T) {
	short := ordinaryState()
	short.Tag = "HI"
	none := ordinaryState()
	none.Tag = ""

	_, conn := newTestRadio(t,
		WithSlot("011", short),
		WithSlot("012", none),
	)
	if got := mustBeACombinedAnswer(t, exchange(t, conn, "MT011;")); got[28:40] != "HI          " {
		t.Errorf("short tag field = %q, want %q", got[28:40], "HI          ")
	}
	if got := mustBeACombinedAnswer(t, exchange(t, conn, "MT012;")); got[28:40] != "            " {
		t.Errorf("empty tag field = %q, want twelve spaces", got[28:40])
	}
}

func TestMTRead_EmptyAndMalformedSlots(t *testing.T) {
	_, conn := newTestRadio(t)
	assertRejected(t, conn, "MT050;") // grammatical, no state
	assertRejected(t, conn, "MT110;") // likewise, an empty PMS slot
	assertRejected(t, conn, "MT118;") // past the printed ceiling
	assertRejected(t, conn, "MTP1L;") // the siblings' PMS spelling
	assertRejected(t, conn, "MT000;") // the answer-only none form
	assertRejected(t, conn, "MT;")
	assertRejected(t, conn, "MT0011;")
}

// TestMTSet_CreatesAnAbsentChannel is the register entry AN MT SET CREATES AN
// ABSENT CHANNEL, and it is the fake's half of the driver's write path: this
// driver has no MW, so a fake that demanded one could not be written to at
// all.
func TestMTSet_CreatesAnAbsentChannel(t *testing.T) {
	for _, slot := range []string{"050", "110"} {
		t.Run(slot, func(t *testing.T) {
			r, conn := newTestRadio(t)
			if _, ok := r.SlotState(slot); ok {
				t.Fatalf("slot %s is populated in the default image — this test needs an absent one", slot)
			}
			writeFrame(t, conn, ordinaryChannel(slot, mtSetKindFixed).frame())
			assertNoReply(t, conn)

			got, ok := r.SlotState(slot)
			if !ok {
				t.Fatalf("slot %s still absent after a combined MT Set", slot)
			}
			if want := ordinaryState(); got != want {
				t.Errorf("stored state:\n got %+v\nwant %+v", got, want)
			}
		})
	}
}

// TestMTSet_RoundTripsByteFaithfully is the register entry THE CLARIFIER IS
// STORED seen end to end: what a Set carried is what the next read answers,
// with only P7 differing between the two directions.
func TestMTSet_RoundTripsByteFaithfully(t *testing.T) {
	_, conn := newTestRadio(t)
	set := ordinaryChannel("050", mtSetKindFixed)
	writeFrame(t, conn, set.frame())
	assertNoReply(t, conn)

	want := ordinaryChannel("050", kindMemory).frame()
	if got := exchange(t, conn, "MT050;"); got != want {
		t.Errorf("round trip:\n got %q\nwant %q", got, want)
	}
}

// TestMTSet_OverwritesAPopulatedChannel — the Set carries the whole record, so
// the overwrite leaves nothing of the old one behind.
func TestMTSet_OverwritesAPopulatedChannel(t *testing.T) {
	r, conn := newTestRadio(t)
	replacement := ordinaryChannel("001", mtSetKindFixed)
	writeFrame(t, conn, replacement.frame())
	assertNoReply(t, conn)

	got, ok := r.SlotState("001")
	if !ok {
		t.Fatal("slot 001 vanished")
	}
	if want := ordinaryState(); got != want {
		t.Errorf("stored state:\n got %+v\nwant %+v", got, want)
	}
}

// TestMTSet_RefusedOnTheSlotsItsLegendDoesNotName pairs with the read arm:
// MT's legend is the 001-117 span and nothing outside it is a slot.
func TestMTSet_RefusedOnTheSlotsItsLegendDoesNotName(t *testing.T) {
	r, conn := newTestRadio(t)
	for _, slot := range []string{"118", "000", "501"} {
		assertRejected(t, conn, ordinaryChannel(slot, mtSetKindFixed).frame())
		if _, ok := r.SlotState(slot); ok {
			t.Errorf("a refused MT Set created slot %q", slot)
		}
	}
}

// TestMTSet_RejectionsLeaveTheChannelUntouched is the register entry
// SET-DIRECTION FIELD STRICTNESS, one deliberately off-vocabulary field per
// case, each drawing "?;" with no state change.
//
// THE FIVE-STATE P8 IS NOT AMONG THE REFUSALS: '3' and '4' are printed values
// on this radio (ft991a_layout.txt:1010-1011) and are accepted — see
// TestMTSet_AcceptsAllFiveP8States. '5' is the first byte past the legend.
func TestMTSet_RejectionsLeaveTheChannelUntouched(t *testing.T) {
	cases := []struct {
		name  string
		frame string
	}{
		{"frequency not nine digits", ordinaryChannel("001", mtSetKindFixed).with(func(f *combinedFrame) { f.freq = "01425000X" }).frame()},
		{"clarifier sign not +/-", ordinaryChannel("001", mtSetKindFixed).with(func(f *combinedFrame) { f.clarSign = '*' }).frame()},
		{"clarifier magnitude not digits", ordinaryChannel("001", mtSetKindFixed).with(func(f *combinedFrame) { f.clarMag = "01X0" }).frame()},
		{"RX clarifier flag outside 0/1", ordinaryChannel("001", mtSetKindFixed).with(func(f *combinedFrame) { f.rxClar = '2' }).frame()},
		{"TX clarifier flag outside 0/1", ordinaryChannel("001", mtSetKindFixed).with(func(f *combinedFrame) { f.txClar = '2' }).frame()},
		{"mode nibble the legend does not print", ordinaryChannel("001", mtSetKindFixed).with(func(f *combinedFrame) { f.mode = 'F' }).frame()},
		// The closing review's C-M2. P6's SET vocabulary is the printed
		// legend and nothing else: `1`-`9`, `A`-`E`
		// (ft991a_layout.txt:1006-1008). '0' is the "-" placeholder that
		// appears in NO legend of this radio — a PARSE-direction courtesy
		// the dialect registers ("THE cat.ModeUnset MEMBER OF THE MODE
		// TABLE") so an answer carrying it can be read, not a value a Set
		// may carry. core/cat refuses to BUILD it (mtcombined.go's
		// Set-frame check), so a fake that accepted it modelled a
		// permissiveness no evidence supports AND stored a byte it would
		// then answer with.
		{"the ModeUnset placeholder, which no Set legend prints", ordinaryChannel("001", mtSetKindFixed).with(func(f *combinedFrame) { f.mode = '0' }).frame()},
		{"P7 not the Set chart's fixed 0", ordinaryChannel("001", kindMemory).frame()},
		{"P8 past the five printed states", ordinaryChannel("001", mtSetKindFixed).with(func(f *combinedFrame) { f.ctcss = '5' }).frame()},
		{"P9 not the fixed 00", ordinaryChannel("001", mtSetKindFixed).with(func(f *combinedFrame) { f.p9 = "01" }).frame()},
		{"shift outside 0-2", ordinaryChannel("001", mtSetKindFixed).with(func(f *combinedFrame) { f.shift = '3' }).frame()},
		{"P11 not the fixed 0", ordinaryChannel("001", mtSetKindFixed).with(func(f *combinedFrame) { f.p11 = '1' }).frame()},
		{"tag carrying an ASCII control code", ordinaryChannel("001", mtSetKindFixed).with(func(f *combinedFrame) { f.tag = "A\x01B" }).frame()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, conn := newTestRadio(t)
			before, _ := r.SlotState("001")
			assertRejected(t, conn, tc.frame)
			after, ok := r.SlotState("001")
			if !ok {
				t.Fatal("slot 001 vanished")
			}
			if after != before {
				t.Errorf("a refused Set changed the channel:\n got %+v\nwant %+v", after, before)
			}
		})
	}
}

// TestMTSet_AcceptsAllFiveP8States is the seam this radio exists to exercise.
// The P8 legend prints FIVE states — `0: CTCSS "OFF" 1: CTCSS ENC/DEC 2: CTCSS
// ENC 3: DCS ENC/DEC 4: DCS ENC` (ft991a_layout.txt:1010-1011) — where every
// registered sibling prints three, and this fake stores and answers all five
// in both directions.
//
// THAT THE RADIO ACCEPTS THE TWO DCS STATES IS THE DIALECT'S ASSUMPTION, cited
// by name: "THE DCS STATES' SET ACCEPTANCE" (core/cat/ft991a/doc.go's
// register), which records that this manual never says whether a DCS state may
// be written into a memory without a DCS code having been set first.
func TestMTSet_AcceptsAllFiveP8States(t *testing.T) {
	for _, state := range []byte{'0', '1', '2', '3', '4'} {
		t.Run(string(state), func(t *testing.T) {
			r, conn := newTestRadio(t)
			set := ordinaryChannel("050", mtSetKindFixed).with(func(f *combinedFrame) { f.ctcss = state })
			writeFrame(t, conn, set.frame())
			assertNoReply(t, conn)

			stored, ok := r.SlotState("050")
			if !ok {
				t.Fatal("slot 050 absent after an accepted Set")
			}
			if stored.CTCSS != state {
				t.Errorf("stored P8 = %q, want %q", stored.CTCSS, state)
			}
			want := ordinaryChannel("050", kindMemory).with(func(f *combinedFrame) { f.ctcss = state }).frame()
			if got := exchange(t, conn, "MT050;"); got != want {
				t.Errorf("MT answer:\n got %q\nwant %q", got, want)
			}
			if got := mustBeAnMRAnswer(t, exchange(t, conn, "MR050;")); got[23] != state {
				t.Errorf("MR answer position 24 = %q, want %q", got[23], state)
			}
		})
	}
}

// TestMTSet_AcceptsTheClarifierRangeThisManualPrints holds the DELIBERATE
// divergence from the dialect: the manual prints "Clarifier Offset: 0000 -
// 9999 (Hz)" (ft991a_layout.txt:1001 and four more blocks) and states no step,
// so this fake takes the printed range — including a magnitude core/cat's
// ClarifierPolicy would refuse to BUILD, which is a different question from
// what the radio accepts. See the dialect's register entry
// "ClarifierPolicy.StepHz = 10 AND ClarifierPolicy.MaxAbsHz = 9990".
func TestMTSet_AcceptsTheClarifierRangeThisManualPrints(t *testing.T) {
	for _, mag := range []string{"0000", "9990", "9999", "0005"} {
		t.Run(mag, func(t *testing.T) {
			r, conn := newTestRadio(t)
			set := ordinaryChannel("050", mtSetKindFixed).with(func(f *combinedFrame) { f.clarMag = mag })
			writeFrame(t, conn, set.frame())
			assertNoReply(t, conn)
			stored, ok := r.SlotState("050")
			if !ok {
				t.Fatalf("magnitude %q was refused; this manual prints 0000 - 9999", mag)
			}
			if stored.ClarMag != mag {
				t.Errorf("stored magnitude = %q, want %q", stored.ClarMag, mag)
			}
		})
	}
}

// TestMTSet_AcceptsEveryModeNibbleTheLegendPrints walks all fourteen. The
// legend runs 1..9 then A..E with NO hole and no 'F'
// (ft991a_layout.txt:1006-1008), where the FT-891 prints "A: -" and the FTdx10
// fills 'F' — the reason this table is transcribed afresh rather than
// borrowed.
//
// FOURTEEN, NOT FIFTEEN: the '0' placeholder is NOT a fifteenth Set value.
// It is the DIALECT's parse-direction assumption ("THE cat.ModeUnset MEMBER
// OF THE MODE TABLE"), which is why validModeWireByte admits it and the
// build-direction validModeBuildByte — the predicate incoming Sets are
// judged by — does not. Its refusal has its own row in
// TestMTSet_RejectionsLeaveTheChannelUntouched (the closing review's C-M2).
func TestMTSet_AcceptsEveryModeNibbleTheLegendPrints(t *testing.T) {
	printed := []byte("123456789ABCDE")
	for _, m := range printed {
		r, conn := newTestRadio(t)
		writeFrame(t, conn, ordinaryChannel("050", mtSetKindFixed).with(func(f *combinedFrame) { f.mode = m }).frame())
		assertNoReply(t, conn)
		stored, ok := r.SlotState("050")
		if !ok {
			t.Fatalf("mode nibble %q was refused, and this radio's legend prints it", m)
		}
		if stored.Mode != m {
			t.Errorf("stored mode = %q, want %q", stored.Mode, m)
		}
	}
	if !validModeWireByte('0') {
		t.Error("the '0' placeholder must be accepted on the parse side — the dialect's ModeUnset member")
	}
	if validModeBuildByte('0') {
		t.Error("the '0' placeholder must NOT be accepted on the build side, which is what an incoming Set is judged by (C-M2)")
	}
	if validModeWireByte('F') {
		t.Error("'F' is printed in none of this radio's five mode legends and must be refused")
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
