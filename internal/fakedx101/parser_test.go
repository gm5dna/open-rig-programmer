// SPDX-License-Identifier: GPL-3.0-or-later

package fakedx101

import (
	"strings"
	"testing"
)

// combinedFrame is one combined MT frame's content in WIRE bytes, field by
// field, so a test states what crosses the wire rather than what a builder
// would produce for it. It serves BOTH directions: a Set and an Answer are one
// 41-byte chart, differing only in the P7 byte each carries.
//
// The zero value is not a valid frame: every field is set explicitly at each
// call site, which is the point — see frame.
//
// This is core/driver/ftdx101's respondingport_test.go's mtAnswerFields, in the
// fake's own test package for the same reason it exists there: a fixture built
// by the code under test pins the parser against the builder, and the two would
// agree about a wrong offset exactly as happily as about a right one.
type combinedFrame struct {
	slot     string // P1, positions 3-5
	freq     string // P2, positions 6-14, 9 digits
	clarSign byte   // P3 sign, position 15
	clarMag  string // P3 magnitude, positions 16-19
	rxClar   byte   // P4, position 20
	txClar   byte   // P5, position 21
	mode     byte   // P6, position 22
	kind     byte   // P7, position 23
	ctcss    byte   // P8, position 24
	p9       string // P9, positions 25-26; "" means the documented fixed "00"
	shift    byte   // P10, position 27
	p11      byte   // P11, position 28; 0 means the documented fixed '0'
	tag      string // P12, positions 29-40, space-padded to 12
}

// frame assembles the 41-byte combined MT frame BY POSITION, from the CAT
// manual's own MT chart (rev 2308-L, layout 1311-1330): "MT" + the 25-position
// shared field block (chart positions 3-27) + P11 + a 12-byte tag field + ';'.
// The same 41 positions were counted independently off 300 dpi raster renders
// in core/cat/ftdx101/testdata/geometry-witness.csv.
//
// Unfilled positions are left as a visible '?' sentinel, so a position this
// assembler forgets shows up as a rejection or a mismatch rather than as an
// accidental zero that happens to be valid somewhere.
//
// The literal 41 belongs in this file for the same reason it must not appear in
// the package's own code: here it is the CHART being asserted.
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

// mrFrame assembles the 28-byte MR answer BY POSITION, from the MR chart
// (layout 1277-1294): "MR" + the same shared field block + ';'. Only the fields
// the block carries are read; tag and P11 are ignored.
func (f combinedFrame) mrFrame() string {
	c := f.frame()
	// The block occupies the same positions in both frames — that is the whole
	// point of one chart under two prefixes — so the MR answer is the combined
	// frame's first 27 positions with "MR" for "MT" and a ';' appended.
	return "MR" + c[2:27] + ";"
}

// with returns a copy of f with mutate applied — for stating one field's
// difference from a fixture at the call site.
func (f combinedFrame) with(mutate func(*combinedFrame)) combinedFrame {
	mutate(&f)
	return f
}

// ordinaryChannel is an unremarkable populated channel's field values: 14.250
// MHz USB, clarifier -150 Hz with the RX clarifier on, CTCSS ENC/DEC, PLUS
// shift, tag "CALLING". Every value is inside this radio's own printed
// vocabularies. The kind is the field a caller sets per direction.
func ordinaryChannel(slot string, kind byte) combinedFrame {
	return combinedFrame{
		slot: slot, freq: "014250000",
		clarSign: '-', clarMag: "0150", rxClar: '1', txClar: '0',
		mode: '2', kind: kind, ctcss: '1', shift: '1',
		tag: "CALLING",
	}
}

// ordinaryState is the MemState this fake must hold for ordinaryChannel — the
// same values, spelled independently of any parse.
func ordinaryState() MemState {
	return MemState{
		Freq: "014250000", ClarSign: '-', ClarMag: "0150",
		RXClar: true, TXClar: false,
		Mode: '2', Kind: kindMemory, CTCSS: '1', Shift: '1',
		Tag: "CALLING",
	}
}

// TestCombinedFrameAssembler_MatchesAHandWrittenLiteral pins the assembler
// above against a frame written out character by character, so every test using
// it rests on something checked rather than on a helper checked by nothing.
//
// The two literals also record the ONE byte that differs between a Set of this
// channel and the answer a read of it produces: position 23, P7. The Set's is
// the chart's fixed '0'; the answer's is '1', Memory — doc.go register entry 2.
//
// These literals are byte-identical to internal/fakedx10's, and that is a
// FINDING rather than a copy: two manufacturers' manuals print the same 41-
// position chart for two different radios, which is why core/cat can carry one
// combined-MT codec for both. If a future radio's chart differed by a position,
// this literal is where the difference would surface first.
func TestCombinedFrameAssembler_MatchesAHandWrittenLiteral(t *testing.T) {
	const wantSet = "MT010014250000-0150102010010CALLING     ;"
	const wantAnswer = "MT010014250000-0150102110010CALLING     ;"

	if len(wantSet) != 41 || len(wantAnswer) != 41 {
		t.Fatalf("the hand-written literals are %d and %d bytes, want 41 per the MT position chart", len(wantSet), len(wantAnswer))
	}
	if got := ordinaryChannel("010", mtSetKindFixed).frame(); got != wantSet {
		t.Errorf("assembled Set = %q, want %q", got, wantSet)
	}
	if got := ordinaryChannel("010", kindMemory).frame(); got != wantAnswer {
		t.Errorf("assembled answer = %q, want %q", got, wantAnswer)
	}
	// The two differ in exactly one position, and it is P7.
	diffs := 0
	for i := range wantSet {
		if wantSet[i] != wantAnswer[i] {
			diffs++
			if i != 22 {
				t.Errorf("Set and answer differ at 0-indexed byte %d, want only 22 (P7, position 23)", i)
			}
		}
	}
	if diffs != 1 {
		t.Errorf("Set and answer differ in %d positions, want exactly 1", diffs)
	}

	const wantMR = "MR010014250000-015010211001;"
	if len(wantMR) != 28 {
		t.Fatalf("the hand-written MR literal is %d bytes, want 28 per the MR position chart", len(wantMR))
	}
	if got := ordinaryChannel("010", kindMemory).mrFrame(); got != wantMR {
		t.Errorf("assembled MR answer = %q, want %q", got, wantMR)
	}
}

// --- MT read ---

// TestMTRead_PopulatedSlot is the flagship read assertion: the exact 41-byte
// answer, against a hand-built expectation from the position chart.
func TestMTRead_PopulatedSlot(t *testing.T) {
	_, conn := newTestRadio(t, WithSlot("010", ordinaryState()))

	want := ordinaryChannel("010", kindMemory).frame()
	if got := exchange(t, conn, "MT010;"); got != want {
		t.Errorf("MT010; -> %q, want %q", got, want)
	}
}

// TestMTRead_TagFieldIsSpacePaddedAndAllFillIsNoTag covers the two tag edges the
// combined form has (doc.go register entry 13): a short tag comes back padded to
// the full field, and a slot with no tag comes back all-fill.
func TestMTRead_TagFieldIsSpacePaddedAndAllFillIsNoTag(t *testing.T) {
	short := ordinaryState()
	short.Tag = "GB3TST" // six bytes, six pad bytes
	none := ordinaryState()
	none.Tag = ""

	_, conn := newTestRadio(t, WithSlot("011", short), WithSlot("012", none))

	if got, want := exchange(t, conn, "MT011;"), ordinaryChannel("011", kindMemory).with(func(f *combinedFrame) { f.tag = "GB3TST" }).frame(); got != want {
		t.Errorf("MT011; -> %q, want %q", got, want)
	}
	if got, want := exchange(t, conn, "MT012;"), ordinaryChannel("012", kindMemory).with(func(f *combinedFrame) { f.tag = "" }).frame(); got != want {
		t.Errorf("MT012; -> %q, want %q", got, want)
	}
	// And the padding is spaces, asserted on the raw bytes rather than through
	// the assembler that also writes them.
	got := exchange(t, conn, "MT012;")
	if field := got[28:40]; field != strings.Repeat(" ", 12) {
		t.Errorf("an untagged channel's P12 field = %q, want twelve spaces (the DIALECT's ASSUMED TagFill entry)", field)
	}
}

func TestMTRead_EmptyAndMalformedSlots(t *testing.T) {
	_, conn := newTestRadio(t)

	tests := []struct {
		name string
		send string
	}{
		// ASSUMED empty-slot rule — doc.go register entry 1. This is the
		// exchange core/driver/ftdx101 reads as an empty channel, and as
		// "absent from this radio" during 5xx/EMG discovery.
		{"never-written memory channel", "MT050;"},
		{"5xx channel, absent without With5xx", "MT501;"},
		{"EMG, absent without WithEMG", "MTEMG;"},
		{"out-of-inventory memory number", "MT100;"},
		{"the answer-only none form is never a request", "MT000;"},
		{"non-existent bank", "MT700;"},
		{"malformed PMS half", "MTP1X;"},
		{"two-byte slot", "MT01;"},
		{"four-byte slot, shorter than a Set", "MT0100;"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertRejected(t, conn, tt.send)
		})
	}
}

// --- MT Set ---

func TestMTSet_CreatesAnAbsentChannel(t *testing.T) {
	r, conn := newTestRadio(t)

	if _, ok := r.SlotState("050"); ok {
		t.Fatal("M-50 is populated in the default image — this test needs an absent slot")
	}
	// Absent slots answer "?;" before the write, which is what makes the
	// after-state meaningful.
	assertRejected(t, conn, "MT050;")

	// ASSUMED — doc.go register entry 6, and the fake's half of
	// core/driver/ftdx101's entry 9: one combined Set creates the channel, with
	// no MW first. An MT-only driver against a fake that demanded one could not
	// write at all.
	writeFrame(t, conn, ordinaryChannel("050", mtSetKindFixed).frame())
	assertNoReply(t, conn) // fire-and-forget success — register entry 11

	if got, want := exchange(t, conn, "MT050;"), ordinaryChannel("050", kindMemory).frame(); got != want {
		t.Errorf("MT050; after the creating Set -> %q, want %q", got, want)
	}
	got, ok := r.SlotState("050")
	if !ok {
		t.Fatal("SlotState(\"050\") reports no state after a successful Set")
	}
	if want := ordinaryState(); got != want {
		t.Errorf("stored state = %+v, want %+v", got, want)
	}
}

func TestMTSet_OverwritesAPopulatedChannel(t *testing.T) {
	r, conn := newTestRadio(t)

	// M-01 is the default image's populated channel: 7.000000 MHz LSB, no tag.
	before, ok := r.SlotState("001")
	if !ok {
		t.Fatal("M-01 is absent from the default image")
	}
	if before.Freq != "007000000" {
		t.Fatalf("M-01 = %+v, want the default image's 7.000000 MHz channel", before)
	}

	writeFrame(t, conn, ordinaryChannel("001", mtSetKindFixed).frame())
	assertNoReply(t, conn)

	if got, want := exchange(t, conn, "MT001;"), ordinaryChannel("001", kindMemory).frame(); got != want {
		t.Errorf("MT001; after the overwriting Set -> %q, want %q", got, want)
	}
}

// TestMTSet_RoundTripsByteFaithfully is the round-trip the plan names: a 41-byte
// Set, then a read of the same slot, and the answer's field bytes — INCLUDING
// THE CLARIFIER BYTES — equal the Set's.
//
// The clarifier is the point. fakeradio stores zeros and ignores what was sent,
// on an FT-710 HARDWARE finding about ITS MW command (its register item 20, and
// the spec.Inert write policy resting on it); that finding is not either of
// these radios', this fake stores what arrives, and BOTH FTdx101 Simulated
// profiles write the clarifier as Supported because of it (matrix §2.1's
// profile table, plan D3, doc.go register entry 5). A non-zero, negative,
// on-step offset with the RX flag set and the TX flag clear is the value that
// would be destroyed by any of the plausible wrong behaviours: zeroing,
// sign-dropping, flag-dropping or rounding.
//
// The negative sign additionally exercises the byte the DIALECT records as
// UNREAD (its "CLARIFIER'S MINUS-DIRECTION BYTE" entry — this manual prints
// that direction as a two-hyphen glyph): if a capture ever moves it, this test
// moves with core/cat.
//
// P7 IS THE ONE EXCEPTION, and it is not a slip: the chart's legend reads "Set:
// 0: (Fixed) / Read: 0: VFO 1: Memory". The Set direction's P7 is a fixed
// placeholder carrying no channel information, so there is nothing in it to
// round-trip — echoing it would answer "VFO" for a memory channel. The
// assertion below therefore requires equality at all 41 positions EXCEPT P7,
// and requires P7 itself to be '1' (register entry 2). A fake that echoed the
// placeholder fails the second half.
func TestMTSet_RoundTripsByteFaithfully(t *testing.T) {
	_, conn := newTestRadio(t)

	set := combinedFrame{
		slot: "042", freq: "007123450",
		clarSign: '-', clarMag: "9990", // the range's ceiling, on the 10 Hz step
		rxClar: '1', txClar: '0',
		mode: 'C', kind: mtSetKindFixed, ctcss: '2', shift: '2',
		tag: "ROUND TRIP", // ten bytes, padded to twelve
	}.frame()

	writeFrame(t, conn, set)
	assertNoReply(t, conn)

	answer := exchange(t, conn, "MT042;")
	if len(answer) != len(set) {
		t.Fatalf("answer is %d bytes, Set was %d — the two directions are one 41-byte chart", len(answer), len(set))
	}

	const p7 = 22 // 0-indexed; position 23
	for i := range set {
		if i == p7 {
			continue
		}
		if set[i] != answer[i] {
			t.Errorf("0-indexed byte %d: Set %q, answer %q — the combined form round-trips every field but P7\n Set    %s\n answer %s", i, set[i], answer[i], set, answer)
		}
	}
	if answer[p7] != kindMemory {
		t.Errorf("answer P7 = %q, want %q (Memory) — the Set's fixed %q must not be echoed", answer[p7], byte(kindMemory), byte(mtSetKindFixed))
	}
}

func TestMTSet_AcceptedOnFiveMHzAndEMG(t *testing.T) {
	// doc.go register entry 8: WIDER than core/cat's own MT write policy, which
	// refuses both by PROJECT DECISION pending hardware verification. This fake
	// models what the radio accepts, not what this project permits itself to
	// send — which is also why this leniency is unreachable through the driver
	// and reachable only by writing bytes to Port() as this test does. (Matrix
	// §1.3.5 records the one degree of hedging this radio's own "P0/1" legend
	// heading requires.)
	for _, slot := range []string{"507", "EMG"} {
		t.Run(slot, func(t *testing.T) {
			_, conn := newTestRadio(t)
			assertRejected(t, conn, "MT"+slot+";") // absent beforehand

			writeFrame(t, conn, ordinaryChannel(slot, mtSetKindFixed).frame())
			assertNoReply(t, conn)

			if got, want := exchange(t, conn, "MT"+slot+";"), ordinaryChannel(slot, kindMemory).frame(); got != want {
				t.Errorf("MT%s; after a Set -> %q, want %q", slot, got, want)
			}
		})
	}
}

// TestMTSet_RejectionsLeaveTheChannelUntouched walks every field vocabulary the
// charts print (doc.go register entry 7) plus the frame's own shape. Each case
// is the ordinary channel with exactly one thing wrong, so a rejection is
// attributable to that one thing.
//
// Every case additionally asserts that the target channel is unchanged
// afterwards — a fake that rejected loudly while storing anyway would pass a
// reply-only assertion.
func TestMTSet_RejectionsLeaveTheChannelUntouched(t *testing.T) {
	tests := []struct {
		name string
		send string
	}{
		{"frequency with a non-digit", ordinaryChannel("001", mtSetKindFixed).with(func(f *combinedFrame) { f.freq = "01425000X" }).frame()},
		{"clarifier sign not +/-", ordinaryChannel("001", mtSetKindFixed).with(func(f *combinedFrame) { f.clarSign = ' ' }).frame()},
		{"clarifier magnitude off the 10 Hz step", ordinaryChannel("001", mtSetKindFixed).with(func(f *combinedFrame) { f.clarMag = "0155" }).frame()},
		{"clarifier magnitude above 9990", ordinaryChannel("001", mtSetKindFixed).with(func(f *combinedFrame) { f.clarMag = "9999" }).frame()},
		{"RX clarifier flag outside 0/1", ordinaryChannel("001", mtSetKindFixed).with(func(f *combinedFrame) { f.rxClar = '2' }).frame()},
		{"TX clarifier flag outside 0/1", ordinaryChannel("001", mtSetKindFixed).with(func(f *combinedFrame) { f.txClar = 'X' }).frame()},
		{"mode nibble above F", ordinaryChannel("001", mtSetKindFixed).with(func(f *combinedFrame) { f.mode = 'G' }).frame()},
		{"mode nibble in lower case", ordinaryChannel("001", mtSetKindFixed).with(func(f *combinedFrame) { f.mode = 'c' }).frame()},
		{"P7 not the chart's fixed 0", ordinaryChannel("001", kindMemory).frame()},
		{"CTCSS state above 2", ordinaryChannel("001", mtSetKindFixed).with(func(f *combinedFrame) { f.ctcss = '3' }).frame()},
		{"P9 not the fixed 00", ordinaryChannel("001", mtSetKindFixed).with(func(f *combinedFrame) { f.p9 = "01" }).frame()},
		{"shift above 2", ordinaryChannel("001", mtSetKindFixed).with(func(f *combinedFrame) { f.shift = '3' }).frame()},
		{"P11 not the fixed 0", ordinaryChannel("001", mtSetKindFixed).with(func(f *combinedFrame) { f.p11 = '1' }).frame()},
		{"tag with a control byte", ordinaryChannel("001", mtSetKindFixed).with(func(f *combinedFrame) { f.tag = "CALL\x01NG" }).frame()},
		{"tag with a high byte", ordinaryChannel("001", mtSetKindFixed).with(func(f *combinedFrame) { f.tag = "CALL\x80NG" }).frame()},
		{"the none form as a Set target", ordinaryChannel("000", mtSetKindFixed).frame()},
		{"an out-of-inventory slot", ordinaryChannel("100", mtSetKindFixed).frame()},
		{"a lower-case PMS slot — the case leniency is the command name's only", ordinaryChannel("p1l", mtSetKindFixed).frame()},
		// Shape: one byte short and one byte long are neither the 3-byte read
		// body nor the 38-byte Set body.
		{"one byte short", strings.Replace(ordinaryChannel("001", mtSetKindFixed).frame(), "CALLING     ;", "CALLING    ;", 1)},
		{"one byte long", strings.Replace(ordinaryChannel("001", mtSetKindFixed).frame(), "CALLING     ;", "CALLING      ;", 1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, conn := newTestRadio(t)
			before, _ := r.SlotState("001")

			assertRejected(t, conn, tt.send)

			after, _ := r.SlotState("001")
			if after != before {
				t.Errorf("a rejected Set changed M-01: %+v -> %+v", before, after)
			}
		})
	}
}

func TestSetsDoNotMoveTheSelection(t *testing.T) {
	// doc.go register entry 10: fakeradio's MW moves the FT-710's selection, on
	// that radio's own M5b hardware finding. Not borrowed — inventing a side
	// effect for two radios nobody has written to is the class of borrowed fact
	// this milestone refuses.
	_, conn := newTestRadio(t)

	writeFrame(t, conn, ordinaryChannel("001", mtSetKindFixed).frame())
	assertNoReply(t, conn)
	if got, want := selectionOverTheWire(t, conn), "000"; got != want {
		t.Errorf("after an MT Set the selection is %q, want it left at %q", got, want)
	}

	writeFrame(t, conn, mwSetFrame("002", "014250000"))
	assertNoReply(t, conn)
	if got, want := selectionOverTheWire(t, conn), "000"; got != want {
		t.Errorf("after an MW Set the selection is %q, want it left at %q", got, want)
	}
}

// --- MR ---

func TestMRRead_PopulatedSlot(t *testing.T) {
	_, conn := newTestRadio(t, WithSlot("010", ordinaryState()))

	// MR is answered even though core/driver/ftdx101 NEVER SENDS IT: the
	// command is on the radio, so it is on the fake (parser.go's MR section).
	// The tag is absent from this frame by the chart, so a tagged channel's MR
	// answer carries the field block alone.
	want := ordinaryChannel("010", kindMemory).mrFrame()
	if got := exchange(t, conn, "MR010;"); got != want {
		t.Errorf("MR010; -> %q, want %q", got, want)
	}
}

func TestMRRead_EmptyAndMalformed(t *testing.T) {
	_, conn := newTestRadio(t)

	assertRejected(t, conn, "MR050;") // empty — register entry 1
	assertRejected(t, conn, "MR100;") // out of inventory
	assertRejected(t, conn, "MR000;") // the answer-only none form
	assertRejected(t, conn, "MREMG;") // absent without WithEMG
	assertRejected(t, conn, "MR;")    // MR has no Read-without-slot form
}

func TestMR_HasNoSetDirection(t *testing.T) {
	r, conn := newTestRadio(t)
	before, _ := r.SlotState("002")

	// This radio's availability table gives MR no Set (X O O X, layout 331) — a
	// MANUAL FACT, not an assumption. An MR frame in the 28-byte Set shape is
	// simply not a command.
	assertRejected(t, conn, "MR"+mwSetFrame("002", "014250000")[2:])

	if after, _ := r.SlotState("002"); after != before {
		t.Errorf("an MR frame in the Set shape changed M-02: %+v -> %+v", before, after)
	}
}

// --- MW ---

// mwSetFrame assembles the 28-byte MW Set BY POSITION, from the MW chart
// (layout 1352-1367): "MW" + the same shared field block + ';', with P7 the
// chart's fixed '0'. Everything but the slot and the frequency is the
// unremarkable simplex/off value, since MW's tests are about the block and the
// tag, not about field variety.
func mwSetFrame(slot, freq string) string {
	return "MW" + slot + freq + "+" + "0000" + "0" + "0" + "1" + string(byte(mwSetKindFixed)) + "0" + "00" + "0" + ";"
}

func TestMWSetFrameAssembler_MatchesAHandWrittenLiteral(t *testing.T) {
	// "MW" + "002" + "014250000" + "+" + "0000" + "0"(rx) + "0"(tx) + "1"(mode
	// LSB) + "0"(P7 fixed) + "0"(CTCSS off) + "00"(P9) + "0"(simplex) + ";"
	const want = "MW002014250000+000000100000;"
	if len(want) != 28 {
		t.Fatalf("the hand-written literal is %d bytes, want 28 per the MW position chart", len(want))
	}
	if got := mwSetFrame("002", "014250000"); got != want {
		t.Errorf("mwSetFrame = %q, want %q", got, want)
	}
}

// TestMWSet_StoresTheFieldBlockAndLeavesTheTag is MW's flagship: the command
// exists on this radio (availability O X X X, layout 336) even though this
// project's driver never sends one, and a fake that rejected it would be faking
// a different radio — doc.go register entry 9.
func TestMWSet_StoresTheFieldBlockAndLeavesTheTag(t *testing.T) {
	tagged := ordinaryState()
	tagged.Tag = "KEEP ME"
	r, conn := newTestRadio(t, WithSlot("020", tagged))

	writeFrame(t, conn, mwSetFrame("020", "007050000"))
	assertNoReply(t, conn) // Set-only, fire-and-forget — register entry 11

	got, ok := r.SlotState("020")
	if !ok {
		t.Fatal("SlotState(\"020\") reports no state after an MW Set")
	}
	want := MemState{
		Freq: "007050000", ClarSign: '+', ClarMag: "0000",
		RXClar: false, TXClar: false,
		Mode: '1', Kind: kindMemory, CTCSS: '0', Shift: '0',
		Tag: "KEEP ME", // ASSUMED untouched — MW has no tag field
	}
	if got != want {
		t.Errorf("stored state = %+v, want %+v", got, want)
	}

	// And the tag survives on the wire too, in the frame that actually carries
	// one.
	if got, want := exchange(t, conn, "MT020;")[28:40], "KEEP ME     "; got != want {
		t.Errorf("MT020; P12 field after an MW = %q, want %q", got, want)
	}
}

func TestMWSet_CreatesAnAbsentChannelWithNoTag(t *testing.T) {
	r, conn := newTestRadio(t)

	writeFrame(t, conn, mwSetFrame("050", "007050000"))
	assertNoReply(t, conn)

	got, ok := r.SlotState("050")
	if !ok {
		t.Fatal("an MW Set to an absent slot stored nothing — ASSUMED creation, doc.go register entry 9")
	}
	if got.Tag != "" {
		t.Errorf("the created channel's tag = %q, want empty", got.Tag)
	}
}

func TestMWSet_Rejections(t *testing.T) {
	tests := []struct {
		name string
		send string
	}{
		// MW's own restricted P1 legend (layout 1353) — a MANUAL FACT of this
		// radio, unlike MT's leniency above.
		{"5xx is outside MW's slot legend", mwSetFrame("501", "005300000")},
		{"EMG is outside MW's slot legend", mwSetFrame("EMG", "005167500")},
		{"the none form", mwSetFrame("000", "007050000")},
		{"out of inventory", mwSetFrame("100", "007050000")},
		{"P7 not the chart's fixed 0", strings.Replace(mwSetFrame("002", "014250000"), "+000000100000;", "+000000110000;", 1)},
		{"a Read-shaped MW: this radio's MW has no Read direction", "MW002;"},
		{"MW with no body at all", "MW;"},
		{"the combined 41-byte shape under an MW prefix", "MW" + ordinaryChannel("002", mtSetKindFixed).frame()[2:]},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, conn := newTestRadio(t)
			before, _ := r.SlotState("002")

			assertRejected(t, conn, tt.send)

			if after, _ := r.SlotState("002"); after != before {
				t.Errorf("a rejected MW changed M-02: %+v -> %+v", before, after)
			}
			for _, slot := range []string{"501", "EMG", "000", "100"} {
				if _, ok := r.SlotState(slot); ok {
					t.Errorf("a rejected MW created %q", slot)
				}
			}
		})
	}
}
