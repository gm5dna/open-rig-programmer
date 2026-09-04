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
