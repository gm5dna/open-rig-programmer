// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft891

import (
	"testing"
)

// combinedFrame is one combined MT frame's content in WIRE bytes, field by
// field, so a test states what crosses the wire rather than what a builder
// would produce for it. It serves BOTH directions: a Set and an Answer are one
// 41-position chart (ft891_layout.txt:996-1027), differing only in the P7 byte
// each carries — and it serves MR too, whose 28-position Answer chart
// (968-975) is this frame's first 27 positions under a different prefix.
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
	p5       byte   // P5, position 21; 0 means the documented fixed '0'
	mode     byte   // P6, position 22
	kind     byte   // P7, position 23
	ctcss    byte   // P8, position 24
	p9       string // P9, positions 25-26; "" means the documented fixed "00"
	shift    byte   // P10, position 27
	p11      byte   // P11, position 28 — this radio's LIVE TAG display flag
	tag      string // P12, positions 29-40, space-padded to 12
}

// frame assembles the 41-byte combined MT frame BY POSITION, from this
// manual's own MT chart (rev 1909-C, ft891_layout.txt:996-1027) as counted
// twice by the geometry witness (core/cat/ft891/testdata/provenance.md §MT:
// "1 M · 2 T · 3-5 P1 · 6-14 P2 · 15 +/- · 16-19 P3 · 20 P4 · 21 P5 · 22 P6 ·
// 23 P7 · 24 P8 · 25-26 P9 · 27 P10 · 28 P11 · 29-40 P12 · 41 ;").
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
	p5 := f.p5
	if p5 == 0 {
		p5 = '0'
	}
	b[20] = p5
	b[21] = f.mode
	b[22] = f.kind
	b[23] = f.ctcss
	p9 := f.p9
	if p9 == "" {
		p9 = "00"
	}
	copy(b[24:26], p9)
	b[26] = f.shift
	b[27] = f.p11
	tagField := b[28:40]
	n := copy(tagField, f.tag)
	for i := n; i < len(tagField); i++ {
		tagField[i] = ' '
	}
	b[40] = ';'
	return string(b)
}

// mrFrame assembles the 28-byte MR answer BY POSITION, from the MR Answer
// chart (ft891_layout.txt:968-975): "MR" + the same shared field block + ';'.
// Only the fields the block carries are read; the TAG display flag and the tag
// are outside it, which is exactly why the discovered banks read them
// Unavailable.
func (f combinedFrame) mrFrame() string {
	c := f.frame()
	// The block occupies the same positions in both frames — that is the whole
	// point of one field grid under several prefixes, which is what
	// core/cat/ft891/doc.go's reused-command verification established — so the
	// MR answer is the combined frame's first 27 positions with "MR" for "MT"
	// and a ';' appended.
	return "MR" + c[2:27] + ";"
}

// ordinaryChannel is an unremarkable populated channel's field values: 14.250
// MHz USB, clarifier -150 Hz with the RX clarifier on, CTCSS ENC/DEC, PLUS
// shift, tag "CALLING" with the TAG display ON. Every value is inside this
// radio's own printed vocabularies. The kind is the field a caller sets per
// direction.
func ordinaryChannel(slot string, kind byte) combinedFrame {
	return combinedFrame{
		slot: slot, freq: "014250000",
		clarSign: '-', clarMag: "0150", rxClar: '1',
		mode: '2', kind: kind, ctcss: '1', shift: '1',
		p11: '1', tag: "CALLING",
	}
}

// ordinaryState is the MemState this fake must hold for ordinaryChannel — the
// same values, spelled independently of any parse.
func ordinaryState() MemState {
	return MemState{
		Freq: "014250000", ClarSign: '-', ClarMag: "0150",
		RXClar: true,
		Mode:   '2', Kind: kindMemory, CTCSS: '1', Shift: '1',
		TagDisplay: true, Tag: "CALLING",
	}
}

// TestCombinedFrameAssembler_MatchesAHandWrittenLiteral pins the assembler
// above against frames written out character by character, so every test using
// it rests on something checked rather than on a helper checked by nothing.
//
// The three literals record the TWO bytes that differ between a Set of this
// channel and the answer a read of it produces:
//
//   - position 23, P7. The Set's is the chart's fixed '0' (legend 1011); the
//     answer's is '1', Memory, which this manual prints for MR (976) and not
//     for MT at all — doc.go's register entries P7 IN AN MT ANSWER and
//     PMS, 5 MHz AND EMG SLOTS ANSWER P7 '1'.
//   - nothing else. Position 21 is '0' in BOTH directions on this radio,
//     because P5 is "0: (Fixed)" here (971, 1006, 1042, 783, 1129) rather
//     than the TX clarifier flag its registered siblings print — which is why
//     the assembler has a p5 field at all, and why the ordinary channel never
//     sets it.
func TestCombinedFrameAssembler_MatchesAHandWrittenLiteral(t *testing.T) {
	const wantSet = "MT001014250000-0150102010011CALLING     ;"
	const wantAnswer = "MT001014250000-0150102110011CALLING     ;"
	const wantMR = "MR001014250000-015010211001;"

	if got := ordinaryChannel("001", '0').frame(); got != wantSet {
		t.Errorf("Set frame:\n got %q\nwant %q", got, wantSet)
	}
	if got := ordinaryChannel("001", kindMemory).frame(); got != wantAnswer {
		t.Errorf("Answer frame:\n got %q\nwant %q", got, wantAnswer)
	}
	if got := ordinaryChannel("001", kindMemory).mrFrame(); got != wantMR {
		t.Errorf("MR answer frame:\n got %q\nwant %q", got, wantMR)
	}

	// The counted widths, asserted rather than assumed — the geometry
	// witness's two independent counts (provenance.md §Method).
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
// THE 5 MHz BOUNDS ARE THIS RADIO'S OWN, TRANSCRIBED. MR's legend prints
// "501 - 510 (5 MHz, U.S. and U.K. version only)" (ft891_layout.txt:962),
// repeated by IF (776) and OI (1122), where the FTdx10's and the FT-710's
// print only "5xx (5MHz BAND)" and their 501..599 is an inherited
// interpretation carried on their own ASSUMED registers. So 511 is not a slot
// on this radio, and internal/fakedx10's grammar — which takes any 5xx —
// would be wrong here.
func TestParseSlotForm(t *testing.T) {
	tests := []struct {
		slot string
		want slotKind
	}{
		{"001", slotMemory},
		{"099", slotMemory},
		{"P1L", slotPMS},
		{"P9U", slotPMS},
		{"501", slotFiveMHz},
		{"510", slotFiveMHz},
		{"EMG", slotEMG},
		{"000", slotNone},

		{"100", slotInvalid},
		{"500", slotInvalid}, // below the printed floor
		{"511", slotInvalid}, // above the printed ceiling — 5xx is NOT a wildcard here
		{"599", slotInvalid},
		{"600", slotInvalid},
		{"P0L", slotInvalid},
		{"P1X", slotInvalid},
		{"p1l", slotInvalid}, // field values are case-sensitive
		{"emg", slotInvalid},
		{"01", slotInvalid},
		{"0011", slotInvalid},
		{"", slotInvalid},
	}
	for _, tt := range tests {
		if got := parseSlotForm(tt.slot); got != tt.want {
			t.Errorf("parseSlotForm(%q) = %v, want %v", tt.slot, got, tt.want)
		}
	}
}

// --- MR: MEMORY CHANNEL READ (availability 164; frames 959-979) ---

// TestMRRead_ServesEveryReadableSlotClass is the "MR for every readable slot"
// half of this fake's read surface, and the one core/driver/ft891's discovery
// walk depends on: 501..510 and EMG are probed BY MR on this radio, because
// MT's own legend does not name them (see TestMTRead_RefusesTheSlotsItsLegend
// DoesNotName).
func TestMRRead_ServesEveryReadableSlotClass(t *testing.T) {
	for _, slot := range []string{"001", "P1L", "501", "510", "EMG"} {
		ch := ordinaryChannel(slot, kindMemory)
		state := ordinaryState()
		_, conn := newTestRadio(t, WithSlot(slot, state))
		if got, want := exchange(t, conn, "MR"+slot+";"), ch.mrFrame(); got != want {
			t.Errorf("MR%s; ->\n got %q\nwant %q", slot, got, want)
		}
	}
}

func TestMRRead_EmptyAndMalformed(t *testing.T) {
	_, conn := newTestRadio(t)

	// An empty slot — ASSUMED, doc.go's register entry EMPTY-SLOT ANSWERS.
	assertRejected(t, conn, "MR001;")
	// Grammatical but not a slot of this radio.
	assertRejected(t, conn, "MR511;")
	assertRejected(t, conn, "MR100;")
	// The answer-only none form is never a request.
	assertRejected(t, conn, "MR000;")
	// Wrong slot width.
	assertRejected(t, conn, "MR01;")
	assertRejected(t, conn, "MR0011;")
	assertRejected(t, conn, "MR;")
}

// TestMR_HasNoSetDirection pins the availability row: MR is "X O O X"
// (ft891_layout.txt:164) — no Set — so a 28-byte MR frame in the Set shape is
// simply an unknown frame, not a write.
func TestMR_HasNoSetDirection(t *testing.T) {
	r, conn := newTestRadio(t)

	setShaped := ordinaryChannel("002", '0').mrFrame()
	assertRejected(t, conn, setShaped)
	if _, ok := r.SlotState("002"); ok {
		t.Error("a Set-shaped MR frame created a channel — MR has no Set direction on this radio")
	}
}

// mustBeAnMRAnswer fails the test unless got is a 28-byte MR answer, so a
// per-position assertion below reports a wrong reply rather than panicking on
// a short one.
func mustBeAnMRAnswer(t *testing.T, got string) string {
	t.Helper()
	if len(got) != 28 || got[:2] != "MR" || got[27] != ';' {
		t.Fatalf("reply %q is not a 28-byte MR answer", got)
	}
	return got
}

// TestMRAnswer_CarriesP5AsTheFixedZero pins the byte that separates this
// radio from every registered sibling: position 21 is schema here
// ("P5 0: (Fixed)", ft891_layout.txt:971 and four more blocks), not a TX
// clarifier flag, so an answer carries '0' whatever else the channel holds.
//
// The zero value of MemState.P5 means that fixed '0' — see its comment — and
// the field exists so a test can craft the answer this radio is ASSUMED never
// to give (doc.go's register entry, and the driver register's own "P5 IS
// ANSWERED '0'"): a strict parse refuses every read if that assumption is
// wrong, so the fake has to be able to play the radio that breaks it.
func TestMRAnswer_CarriesP5AsTheFixedZero(t *testing.T) {
	honest := ordinaryState()
	_, conn := newTestRadio(t, WithSlot("001", honest))
	got := mustBeAnMRAnswer(t, exchange(t, conn, "MR001;"))
	if got[20] != '0' {
		t.Errorf("MR001; answered P5 = %q, want '0' — the legend prints \"P5 0: (Fixed)\"", got[20])
	}

	crafted := ordinaryState()
	crafted.P5 = '1'
	_, conn2 := newTestRadio(t, WithSlot("001", crafted))
	got2 := mustBeAnMRAnswer(t, exchange(t, conn2, "MR001;"))
	if got2[20] != '1' {
		t.Errorf("a crafted P5 answered %q, want '1' — the field is there so the refutation is reachable", got2[20])
	}
}

// --- MT: MEMORY WRITE & TAG, THE COMBINED FORM (availability 166; frames
// 996-1027) ---
//
// The availability row gives MT "Set O, Read X, Ans. X" and its own detail
// block prints a filled Read chart and a filled 41-position Answer chart.
// BOTH CANNOT BE TRUE, and core/cat/ft891/doc.go records the contradiction
// without resolving it. This fake models the DETAIL BLOCK by default —
// doc.go's register entry MT READ IS ANSWERED — and models the COMMAND LIST
// under WithMTReadUnsupported(), so a driver's refusal path is reachable end
// to end without a scripted transcript.

// TestMTRead_ServesTheTagFlagInByte28 is this radio's one genuinely new axis:
// MT's P11 legend prints `0: TAG "OFF" 1: TAG "ON"` (ft891_layout.txt:1016)
// where every registered combined-form sibling prints "0: (Fixed)". The fake
// stores the flag per channel and answers it back, which is the half of
// core/driver/ft891's read path that reads TagDisplay Known.
func TestMTRead_ServesTheTagFlagInByte28(t *testing.T) {
	for _, tc := range []struct {
		name    string
		display bool
		want    byte
	}{
		{"TAG display ON", true, '1'},
		{"TAG display OFF", false, '0'},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := ordinaryState()
			state.TagDisplay = tc.display
			_, conn := newTestRadio(t, WithSlot("001", state))

			want := ordinaryChannel("001", kindMemory).with(func(f *combinedFrame) {
				f.p11 = tc.want
			}).frame()
			got := mustBeACombinedAnswer(t, exchange(t, conn, "MT001;"))
			if got != want {
				t.Fatalf("MT001; ->\n got %q\nwant %q", got, want)
			}
			if got[27] != tc.want {
				t.Errorf("byte 28 = %q, want %q", got[27], tc.want)
			}
		})
	}
}

// mustBeACombinedAnswer fails the test unless got is a 41-byte MT answer, so
// a per-position assertion below reports a wrong reply rather than panicking
// on a short one — which a "?;" would otherwise do, taking the whole test
// binary with it.
func mustBeACombinedAnswer(t *testing.T, got string) string {
	t.Helper()
	if len(got) != 41 || got[:2] != "MT" || got[40] != ';' {
		t.Fatalf("reply %q is not a 41-byte combined MT answer", got)
	}
	return got
}

// TestMTRead_TagFieldIsSpacePaddedAndAllFillIsNoTag pins the storage rule:
// stored TRIMMED, answered PADDED to the full 12-byte field, so an all-fill
// field means "no tag" with no branch of its own. The fill byte is a space
// because the DIALECT says so — core/cat/ft891/doc.go's ASSUMED register entry
// "MTPolicy.TagFill = ' '", cited here and never re-derived.
func TestMTRead_TagFieldIsSpacePaddedAndAllFillIsNoTag(t *testing.T) {
	short := ordinaryState()
	short.Tag = "TWENTY"
	_, conn := newTestRadio(t, WithSlot("001", short))
	got := mustBeACombinedAnswer(t, exchange(t, conn, "MT001;"))
	if want := "TWENTY      "; got[28:40] != want {
		t.Errorf("tag field = %q, want %q (six characters then six spaces)", got[28:40], want)
	}

	none := ordinaryState()
	none.Tag = ""
	_, conn2 := newTestRadio(t, WithSlot("001", none))
	got2 := mustBeACombinedAnswer(t, exchange(t, conn2, "MT001;"))
	if want := "            "; got2[28:40] != want {
		t.Errorf("empty tag field = %q, want twelve spaces", got2[28:40])
	}

	// Only reachable through WithSlot — the wire cannot deliver a 13th byte.
	long := ordinaryState()
	long.Tag = "THIRTEENCHARS"
	_, conn3 := newTestRadio(t, WithSlot("001", long))
	got3 := mustBeACombinedAnswer(t, exchange(t, conn3, "MT001;"))
	if want := "THIRTEENCHAR"; got3[28:40] != want {
		t.Errorf("truncated tag field = %q, want %q", got3[28:40], want)
	}
}

func TestMTRead_EmptyAndMalformedSlots(t *testing.T) {
	_, conn := newTestRadio(t)

	assertRejected(t, conn, "MT001;") // empty — the register's EMPTY-SLOT ANSWERS
	assertRejected(t, conn, "MT100;") // grammatical, not a slot of this radio
	assertRejected(t, conn, "MT000;") // the answer-only none form
	assertRejected(t, conn, "MT01;")
	assertRejected(t, conn, "MT;")
}

// TestMTRead_RefusesTheSlotsItsLegendDoesNotName is the divergence from
// internal/fakedx10 that matters most, and it is a MANUAL FACT rather than a
// policy: this radio's MT block prints its slot legend as memory and PMS ONLY
// (ft891_layout.txt:998-999) where its own MR block prints the 5 MHz bank and
// EMG as well (960-964). So a populated 501 answers MR and refuses MT.
//
// It is the fake's half of core/driver/ft891's negative discovery pin — that
// no MT read of a 5xx or EMG slot is ever built — and of the dialect's
// MTPolicy.ReadSlots = cat.MTReadsMemoryPMS.
func TestMTRead_RefusesTheSlotsItsLegendDoesNotName(t *testing.T) {
	for _, slot := range []string{"501", "510", "EMG"} {
		_, conn := newTestRadio(t, WithSlot(slot, ordinaryState()))

		// Populated, and MR proves it in the same session, so the MT refusal
		// below cannot be an empty slot in disguise.
		if got := exchange(t, conn, "MR"+slot+";"); got == "?;" {
			t.Fatalf("MR%s; -> %q, so the MT refusal proves nothing", slot, got)
		}
		assertRejected(t, conn, "MT"+slot+";")
	}
}

// --- MT Set ---

func TestMTSet_CreatesAnAbsentChannelWithEitherTagFlag(t *testing.T) {
	for _, tc := range []struct {
		name    string
		p11     byte
		display bool
	}{
		{"TAG ON", '1', true},
		{"TAG OFF", '0', false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, conn := newTestRadio(t)

			set := ordinaryChannel("002", '0').with(func(f *combinedFrame) {
				f.p11 = tc.p11
			}).frame()
			writeFrame(t, conn, set)
			assertNoReply(t, conn)

			got, ok := r.SlotState("002")
			if !ok {
				t.Fatal("the Set did not create the channel")
			}
			want := ordinaryState()
			want.TagDisplay = tc.display
			if got != want {
				t.Errorf("stored state:\n got %+v\nwant %+v", got, want)
			}
		})
	}
}

func TestMTSet_OverwritesAPopulatedChannel(t *testing.T) {
	before := ordinaryState()
	before.Freq = "007100000"
	before.Tag = "OLD"
	r, conn := newTestRadio(t, WithSlot("001", before))

	set := ordinaryChannel("001", '0').with(func(f *combinedFrame) {
		f.p11 = '0'
		f.tag = "NEW"
	}).frame()
	writeFrame(t, conn, set)
	assertNoReply(t, conn)

	got, ok := r.SlotState("001")
	if !ok {
		t.Fatal("the channel vanished")
	}
	want := ordinaryState()
	want.TagDisplay = false
	want.Tag = "NEW"
	if got != want {
		t.Errorf("stored state:\n got %+v\nwant %+v", got, want)
	}
}

// TestMTSet_RoundTripsByteFaithfully is the property core/clone's
// write-then-verify rests on: everything the Set carried comes back, and the
// ONLY byte that differs is P7 — the Set's fixed '0' against the answer's
// '1'.
func TestMTSet_RoundTripsByteFaithfully(t *testing.T) {
	_, conn := newTestRadio(t)

	ch := ordinaryChannel("003", '0').with(func(f *combinedFrame) {
		f.clarSign = '+'
		f.clarMag = "9999" // this manual's printed ceiling, off the dialect's assumed 10 Hz step
		f.mode = 'D'       // AM-N, the top of this radio's printed nibble range
		f.ctcss = '2'
		f.shift = '2'
		f.p11 = '1'
		f.tag = "PMSLOWEREDGE"
	})
	writeFrame(t, conn, ch.frame())
	assertNoReply(t, conn)

	want := ch.with(func(f *combinedFrame) { f.kind = kindMemory }).frame()
	if got := exchange(t, conn, "MT003;"); got != want {
		t.Errorf("read back:\n got %q\nwant %q", got, want)
	}
}

// TestMTSet_RefusedOnTheSlotsItsLegendDoesNotName mirrors the read refusal,
// and is again this manual's legend rather than a project policy — the
// direction internal/fakedx10 registers a LENIENCY for, because its MT legend
// names all four classes.
func TestMTSet_RefusedOnTheSlotsItsLegendDoesNotName(t *testing.T) {
	for _, slot := range []string{"501", "510", "EMG"} {
		r, conn := newTestRadio(t)
		assertRejected(t, conn, ordinaryChannel(slot, '0').frame())
		if _, ok := r.SlotState(slot); ok {
			t.Errorf("a refused MT Set created %s anyway", slot)
		}
	}
}

// TestMTSet_RejectionsLeaveTheChannelUntouched walks the SET-DIRECTION FIELD
// STRICTNESS register entry field by field. Every case is a frame this
// radio's own charts do not describe, and every one must draw "?;" with no
// state change.
func TestMTSet_RejectionsLeaveTheChannelUntouched(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*combinedFrame)
	}{
		{"frequency not nine digits", func(f *combinedFrame) { f.freq = "01425000X" }},
		{"clarifier sign outside +/-", func(f *combinedFrame) { f.clarSign = '*' }},
		{"clarifier magnitude not four digits", func(f *combinedFrame) { f.clarMag = "01X0" }},
		{"RX clarifier flag outside 0/1", func(f *combinedFrame) { f.rxClar = '2' }},
		{"P5 not the fixed '0'", func(f *combinedFrame) { f.p5 = '1' }},
		{"mode at the legend's printed hole 'A'", func(f *combinedFrame) { f.mode = 'A' }},
		{"mode 'E', not printed in any legend", func(f *combinedFrame) { f.mode = 'E' }},
		{"mode 'F', not printed in any legend", func(f *combinedFrame) { f.mode = 'F' }},
		{"P7 not the Set chart's fixed '0'", func(f *combinedFrame) { f.kind = '1' }},
		{"CTCSS state outside 0-2", func(f *combinedFrame) { f.ctcss = '3' }},
		{"P9 not the fixed \"00\"", func(f *combinedFrame) { f.p9 = "01" }},
		{"shift outside 0-2", func(f *combinedFrame) { f.shift = '3' }},
		{"P11 outside the TAG flag's 0/1", func(f *combinedFrame) { f.p11 = '2' }},
		{"tag carrying a control byte", func(f *combinedFrame) { f.tag = "A\x01B" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, conn := newTestRadio(t)
			assertRejected(t, conn, ordinaryChannel("004", '0').with(tt.mutate).frame())
			if _, ok := r.SlotState("004"); ok {
				t.Error("a refused Set stored a channel")
			}
		})
	}
}

// TestMTSet_AcceptsTheClarifierRangeThisManualPrints is the other side of
// TestMTSet_RejectionsLeaveTheChannelUntouched: 9999 is inside the printed
// "Clarifier Offset: 0000 - 9999 (Hz)" and OUTSIDE the dialect's assumed
// 10 Hz step and 9990 ceiling, and this fake models the radio rather than the
// project's own build policy.
func TestMTSet_AcceptsTheClarifierRangeThisManualPrints(t *testing.T) {
	for _, mag := range []string{"0000", "0005", "9990", "9999"} {
		r, conn := newTestRadio(t)
		writeFrame(t, conn, ordinaryChannel("005", '0').with(func(f *combinedFrame) {
			f.clarMag = mag
		}).frame())
		assertNoReply(t, conn)
		got, ok := r.SlotState("005")
		if !ok {
			t.Fatalf("clarifier magnitude %q was refused", mag)
		}
		if got.ClarMag != mag {
			t.Errorf("stored magnitude = %q, want %q", got.ClarMag, mag)
		}
	}
}

// TestWithMTReadUnsupported_HonoursTheCommandList makes the fake play the
// OTHER radio the manual describes — the one whose availability row says MT is
// Set-only (ft891_layout.txt:166) — so core/driver/ft891's
// ErrMTReadRejectedForOccupiedSlot is reachable end to end against a real
// fake rather than a scripted transcript. Set and MR are untouched, which is
// exactly the pair that produces the driver's typed refusal.
func TestWithMTReadUnsupported_HonoursTheCommandList(t *testing.T) {
	r, conn := newTestRadio(t, WithMTReadUnsupported(), WithSlot("001", ordinaryState()))

	assertRejected(t, conn, "MT001;")

	// The same slot answers MR in the same session — the contradiction the
	// driver reports rather than diagnoses.
	if got, want := exchange(t, conn, "MR001;"), ordinaryChannel("001", kindMemory).mrFrame(); got != want {
		t.Errorf("MR001; ->\n got %q\nwant %q", got, want)
	}

	// The Set direction is what the command list DOES give MT, so it still
	// works.
	writeFrame(t, conn, ordinaryChannel("002", '0').frame())
	assertNoReply(t, conn)
	if _, ok := r.SlotState("002"); !ok {
		t.Error("WithMTReadUnsupported() disabled the Set direction too — the command list gives MT \"Set O\"")
	}
}
