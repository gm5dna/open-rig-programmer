// SPDX-License-Identifier: GPL-3.0-or-later

package ts590_test

import (
	"reflect"
	"sort"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/kw/kwtest"
	"github.com/gm5dna/open-rig-programmer/core/kw/ts590"
)

// THE TWO 590 ROWS ARE TESTED AS TWO RADIOS, NEVER AS ONE.
//
// The package doc comment states the rule this file mechanises: Kenwood
// prints the S and the SG in one document, which is a property of the book
// and not of the radios. So every check below either holds ONE row to a
// property of its own, or holds the PAIR to a stated agreement or a stated
// disagreement — and the disagreements are the ones a reader will look for,
// because a sibling pair sharing a grid is where a value silently borrowed
// from the other row would never show.
//
// The conformance suite is kwtest's (core/kw/kwtest): the per-radio walk of
// the eight grammars through this layout's own builders, parsers and
// outbound gate. It cannot live in core/kw, which this package imports.

// TestConformance_TS590S runs core/kw's conformance suite over the S row.
func TestConformance_TS590S(t *testing.T) { kwtest.Run(t, ts590.LayoutS()) }

// TestConformance_TS590SG runs the same suite over the SG row.
func TestConformance_TS590SG(t *testing.T) { kwtest.Run(t, ts590.LayoutSG()) }

// TestLayouts_AreConfiguredAndNamedPerRow is the vacuity guard every other
// check in this file rests on: an accessor returning the zero Layout would
// make kwtest.Run fatal, but the pins below would quietly compare two
// unconfigured values and agree.
//
// NEITHER MODEL IS "TS-590". The ASSUMED register's standing rule is that no
// entry may name the pair with a bare singular, and a layout's Model() is
// what every refusal this package produces quotes.
func TestLayouts_AreConfiguredAndNamedPerRow(t *testing.T) {
	s, sg := ts590.LayoutS(), ts590.LayoutSG()
	if !s.Configured() {
		t.Error("LayoutS() is unconfigured — it describes no radio and refuses everything")
	}
	if !sg.Configured() {
		t.Error("LayoutSG() is unconfigured — it describes no radio and refuses everything")
	}
	if got := s.Model(); got != "TS-590S" {
		t.Errorf("LayoutS().Model() = %q, want %q", got, "TS-590S")
	}
	if got := sg.Model(); got != "TS-590SG" {
		t.Errorf("LayoutSG().Model() = %q, want %q", got, "TS-590SG")
	}
	if s.Model() == sg.Model() {
		t.Error("the two rows carry ONE model name — every refusal either produces would then name the wrong radio")
	}
	for _, l := range []kw.Layout{s, sg} {
		if l.Model() == "TS-590" {
			t.Errorf("a layout is named %q, which names TWO registry rows; no value in this package may say \"a TS-590\"", l.Model())
		}
	}
}

// TestLayoutConfig_HasExactlyTenComparedAxes makes "ten axes" a fact
// rather than a habit (T8 review LOW-4). kw.LayoutConfig's Book and Model
// fields are never compared as an axis — they are the row's own identity,
// not a fact about the memory grid — so the compared-axis count is
// NumField() minus those two. If a field is ever added to LayoutConfig,
// THIS test fails first, before the silently-short lists it names: the
// two lists below (share/differ), core/kw/ts480/layout_test.go's
// TestLayout_EveryAxisByValue, and core/kw/layout_test.go's
// TestNewLayout_RefusesAnUnsetAxis (which walks all twelve fields,
// Book and Model included).
func TestLayoutConfig_HasExactlyTenComparedAxes(t *testing.T) {
	const bookAndModel = 2 // identity fields, never compared as an axis
	if got := reflect.TypeOf(kw.LayoutConfig{}).NumField() - bookAndModel; got != 10 {
		t.Fatalf("kw.LayoutConfig has %d compared axes (NumField()-%d), want 10 — a field was added or removed; update TestLayouts_TheAxesTheTwoRowsShare and TestLayouts_TheThreeAxesTheRowsDifferOn in this file, core/kw/ts480/layout_test.go's TestLayout_EveryAxisByValue, and core/kw/layout_test.go's TestNewLayout_RefusesAnUnsetAxis for the new one", got, bookAndModel)
	}
}

// TestLayouts_TheAxesTheTwoRowsShare pins the agreement side.
//
// SEVEN OF THE TEN AXES ARE THE BOOK'S, not the row's: one document
// (590:*) prints one 50-byte grid, one MD legend and one hard-wired byte
// set for both radios, so a difference appearing on any of these would be a
// transcription error rather than a discovery. Stating the agreement is what
// makes the three-item disagreement list below exhaustive rather than
// approximate. TestLayoutConfig_HasExactlyTenComparedAxes above is what
// makes "ten" itself a fact.
func TestLayouts_TheAxesTheTwoRowsShare(t *testing.T) {
	s, sg := ts590.LayoutS(), ts590.LayoutSG()

	if s.Book() != kw.Book590 || sg.Book() != kw.Book590 {
		t.Errorf("the rows name books %v and %v; both speak the TS-590S/TS-590SG reference guide", s.Book(), sg.Book())
	}
	for _, tc := range []struct {
		axis     string
		sv, sgv  any
		expected any
	}{
		{"byte 4's policy", s.P2Policy(), sg.P2Policy(), kw.P2HundredsDigit},
		{"byte 19's meaning", s.Byte19(), sg.Byte19(), kw.Byte19DataMode},
		{"bytes 39-40's meaning", s.Byte3940(), sg.Byte3940(), kw.Byte3940FMNarrowFlag},
		{"byte 41's meaning", s.Byte41(), sg.Byte41(), kw.Byte41Lockout},
		{"the tone-mode value set", s.ToneModes(), sg.ToneModes(), kw.ToneModesFour},
	} {
		if tc.sv != tc.sgv {
			t.Errorf("%s differs between the rows (S %v, SG %v), and this document prints one legend for both", tc.axis, tc.sv, tc.sgv)
		}
		if tc.sv != tc.expected {
			t.Errorf("%s is %v on the S, want %v", tc.axis, tc.sv, tc.expected)
		}
	}

	if !reflect.DeepEqual(s.ModeNames(), sg.ModeNames()) {
		t.Errorf("the MD legends differ:\n  S  %v\n  SG %v\nMD is printed once, under \"[TS-590S / TS-590SG common]\" (590:1350-1363)", s.ModeNames(), sg.ModeNames())
	}
	if !reflect.DeepEqual(s.PrintedFixed(), sg.PrintedFixed()) {
		t.Errorf("the hard-wired byte sets differ:\n  S  %v\n  SG %v\nP10, P12 and P13 are the whole of this book's hard-wiring (590:1558-1568)", s.PrintedFixed(), sg.PrintedFixed())
	}

	// The legend, by value: eight nibbles, and NOT the two the book prints
	// as "None (setting failure)" (590:1353, 590:1362).
	want := map[kw.Mode]string{
		kw.ModeLSB: "LSB", kw.ModeUSB: "USB", kw.ModeCW: "CW", kw.ModeFM: "FM",
		kw.ModeAM: "AM", kw.ModeFSK: "FSK", kw.ModeCWR: "CW-R", kw.ModeFSKR: "FSK-R",
	}
	if got := sg.ModeNames(); !reflect.DeepEqual(got, want) {
		t.Errorf("the MD legend is %v, want %v (590:1353-1363)", got, want)
	}
	for _, nibble := range []kw.Mode{kw.ModeNone, kw.ModeTune} {
		if _, ok := sg.ParseMode(byte(nibble)); ok {
			t.Errorf("the legend names nibble %q, which the book prints as \"None (setting failure)\" (590:1353, 590:1362) and which names no mode a channel can be in", byte(nibble))
		}
	}

	// The hard-wired set, by value: THIRTEEN bytes in three runs, and no
	// fourth run — byte 4, byte 28 and byte 41 all carry meanings on this
	// book and a layout claiming a constant at one of them would make this
	// codec refuse a legitimate answer.
	wantFixed := []kw.FixedField{
		{Pos: 25, Printed: "000"},
		{Pos: 29, Printed: "0"},
		{Pos: 30, Printed: "000000000"},
	}
	if got := sg.PrintedFixed(); !reflect.DeepEqual(got, wantFixed) {
		t.Errorf("the hard-wired set is %v, want %v (590:1558-1559, 590:1565-1566, 590:1567-1568)", got, wantFixed)
	}
}

// TestLayouts_TheThreeAxesTheRowsDifferOn is the disagreement side, and the
// list is exhaustive: byte 28's policy, the slot ceiling and the printed EX
// menu domain, and nothing else.
func TestLayouts_TheThreeAxesTheRowsDifferOn(t *testing.T) {
	s, sg := ts590.LayoutS(), ts590.LayoutSG()

	// A14 / E7. The book prints one P11 legend for both rows
	// (590:1560-1563) and then a sentence scoped to one of them — "* In
	// firmware version 1.xx of TS-590S, always \"0\"." (590:1478).
	if got := sg.Byte28(); got != kw.Byte28FilterLive {
		t.Errorf("the SG's byte-28 policy is %v, want %v", got, kw.Byte28FilterLive)
	}
	if got := s.Byte28(); got != kw.Byte28FilterEither {
		t.Errorf("the S's byte-28 policy is %v, want %v", got, kw.Byte28FilterEither)
	}
	if s.Byte28() == sg.Byte28() {
		t.Error("the two rows carry ONE byte-28 policy — A14 is the entry that says they do not, and the axis is what a driver reads to decide whether a write may carry FILTER B")
	}

	// A12. The book gives 110-119 to the SG (590:1346-1347) and never
	// states the S's own ceiling.
	if got := ceiling(s); got != 109 {
		t.Errorf("the S's slot space reaches %d, want 109 — the book never states this row's ceiling and A12 is that entry (590:1345-1347)", got)
	}
	if got := ceiling(sg); got != 119 {
		t.Errorf("the SG's slot space reaches %d, want 119 (590:1346-1347)", got)
	}
	if !hasClass(sg, kw.SlotExtension) {
		t.Error("the SG declares no SlotExtension range, and 590:1346-1347 prints 110-119 as this row's extension channels")
	}
	if hasClass(s, kw.SlotExtension) {
		t.Error("the S declares a SlotExtension range; 590:1346-1347 gives 110-119 to the SG and says nothing about the S (A12)")
	}

	// The printed EX menu domain: two lines of one chart, one per row —
	// "000 ~ 087: Menu number (TS-590S)" (590:543) and "000 ~ 099: Menu
	// number (TS-590SG)" (590:544). This is DOCUMENTED FACT on both rows,
	// unlike the slot ceiling above, which rests on the S's silence (A12).
	if got := s.MaxEXAddress(); got != 87 {
		t.Errorf("the S's printed EX menu domain stops at %d, want 87 (590:543)", got)
	}
	if got := sg.MaxEXAddress(); got != 99 {
		t.Errorf("the SG's printed EX menu domain stops at %d, want 99 (590:544)", got)
	}
	if s.MaxEXAddress() == sg.MaxEXAddress() {
		t.Error("the two rows carry ONE EX menu domain — the chart prints the two on consecutive lines (590:543-544), and their two inventories are one identifier apart in this package")
	}
}

// TestRedProof_TheEXDomainIsThePrintedOnePerRow is the behavioural half of
// 590:543-544, in the direction that matters: an EX sweep that took the SG's
// hundred rows as the S's bound would put twelve frames on a TS-590S whose
// own book stops at 087, and the radio's whole answer would be "?;".
func TestRedProof_TheEXDomainIsThePrintedOnePerRow(t *testing.T) {
	s, sg := ts590.LayoutS(), ts590.LayoutSG()

	cmd, err := sg.BuildEXRead(kw.EXAddress{P1: 88})
	if err != nil {
		t.Fatalf("the SG refused menu 088, inside the domain its own book prints (590:544): %v", err)
	}
	frame := cmd.Bytes()
	if got := string(frame); got != "EX0880000;" {
		t.Errorf("the SG's EX read of menu 088 is %q, want %q", got, "EX0880000;")
	}
	if !sg.AllowedCommand(frame) {
		t.Errorf("the SG's own gate refused %q, which its own builder produced", frame)
	}
	if s.AllowedCommand(frame) {
		t.Errorf("the S's gate ADMITTED %q — 088 is outside the domain its own book prints, 000 ~ 087 (590:543)", frame)
	}
	if _, err := s.BuildEXRead(kw.EXAddress{P1: 88}); err == nil {
		t.Error("the S built a read of menu 088 (590:543)")
	}
	// And the last address each row DOES print, so the bound is not simply
	// refusing everything.
	if _, err := s.BuildEXRead(kw.EXAddress{P1: 87}); err != nil {
		t.Errorf("the S refused menu 087, the last address its own book prints (590:543): %v", err)
	}
}

// TestLayouts_TheSlotSpacesByValue pins both spaces outright, so a range
// that shifted by one fails here rather than in a driver's bank map.
func TestLayouts_TheSlotSpacesByValue(t *testing.T) {
	wantS := []kw.SlotRange{
		{Class: kw.SlotMemory, Lo: 0, Hi: 99},
		{Class: kw.SlotScan, Lo: 100, Hi: 109},
	}
	wantSG := append(append([]kw.SlotRange{}, wantS...),
		kw.SlotRange{Class: kw.SlotExtension, Lo: 110, Hi: 119})

	if got := sorted(ts590.LayoutS().Slots()); !reflect.DeepEqual(got, wantS) {
		t.Errorf("the S's slot space is %v, want %v (590:1341, 590:1345, A12)", got, wantS)
	}
	if got := sorted(ts590.LayoutSG().Slots()); !reflect.DeepEqual(got, wantSG) {
		t.Errorf("the SG's slot space is %v, want %v (590:1341, 590:1345, 590:1346-1347)", got, wantSG)
	}
}

// TestRedProof_TheExtensionSlotsAreTheSGsAlone is the behavioural half of
// A12, and it is red-proved four ways: an S that declared 110-119 would pass
// every other test in this file and fail all four of these.
func TestRedProof_TheExtensionSlotsAreTheSGsAlone(t *testing.T) {
	s, sg := ts590.LayoutS(), ts590.LayoutSG()

	slot, err := sg.NewSlot(110, kw.ScanHalfNone)
	if err != nil {
		t.Fatalf("the SG refused slot 110, which its own book prints as E00 (590:1346-1347): %v", err)
	}
	if got := slot.Class(); got != kw.SlotExtension {
		t.Errorf("the SG resolved slot 110 to %v, want %v", got, kw.SlotExtension)
	}
	if _, err := s.NewSlot(110, kw.ScanHalfNone); err == nil {
		t.Error("the S resolved slot 110; the book gives that range to the SG and never states the S's own ceiling (A12)")
	}

	// The read frame, built and gated.
	cmd, err := sg.BuildMRRead(slot)
	if err != nil {
		t.Fatalf("the SG could not build an MR read of slot 110: %v", err)
	}
	frame := cmd.Bytes()
	if got := string(frame); got != "MR0110;" {
		t.Errorf("the SG's MR read of slot 110 is %q, want %q", got, "MR0110;")
	}
	if !sg.AllowedCommand(frame) {
		t.Errorf("the SG's own gate refused %q, which its own builder produced", frame)
	}
	if s.AllowedCommand(frame) {
		t.Errorf("the S's gate ADMITTED %q — slot 110 is outside its space, and a frame the sibling may send is not one this row may (A12)", frame)
	}

	// And the ANSWER: a 50-byte record naming 110 decodes on the SG and is
	// refused on the S, which is the direction that matters — a front-panel
	// recall of E00 is a frame an SG really sends.
	answer := recordFrame("MR", '0', "110", "00014250000", '2', "00")
	if _, err := sg.ParseMRAnswer(answer); err != nil {
		t.Errorf("the SG refused an MR answer naming slot 110: %v", err)
	}
	if _, err := s.ParseMRAnswer(answer); err == nil {
		t.Error("the S parsed an MR answer naming slot 110, a channel its slot space does not contain (A12)")
	}
}

// TestLayouts_BothRowsAcceptEitherFilterByteOnARead is the honest statement
// of what the byte-28 axis does and does not do TODAY.
//
// IT IS A DRIVER-VISIBLE DIFFERENCE, NOT A PARSER-VISIBLE ONE. The "always
// 0" sentence is scoped to firmware 1.xx of the TS-590S (590:1478), so a
// TS-590S at 2.00 or later answers with '1' and a parser refusing it would
// fail a session on a radio the document permits. Both rows therefore accept
// both printed values on a READ, and what a WRITE may carry is the driver's
// question. This pin exists so that narrowing the S's parser later is a
// deliberate change rather than an unnoticed one.
func TestLayouts_BothRowsAcceptEitherFilterByteOnARead(t *testing.T) {
	for _, tc := range []struct {
		row string
		l   kw.Layout
	}{
		{"TS-590S", ts590.LayoutS()},
		{"TS-590SG", ts590.LayoutSG()},
	} {
		for _, filter := range []byte{'0', '1'} {
			frame := recordFrame("MR", filter, "003", "00014250000", '2', "00")
			rec, err := tc.l.ParseMRAnswer(frame)
			if err != nil {
				t.Errorf("%s refused an MR answer with byte 28 = %q: %v\nP11 prints '0' FILTER A and '1' FILTER B (590:1560-1563) and the \"always 0\" sentence is scoped to firmware 1.xx of the TS-590S (590:1478)", tc.row, filter, err)
				continue
			}
			if rec.Byte28 != filter {
				t.Errorf("%s decoded byte 28 as %q, want %q", tc.row, rec.Byte28, filter)
			}
		}
	}
}

// TestLayouts_AreIndependentValues is the borrowing guard at the level of
// memory rather than of prose: the two rows must not share the slice or the
// map either accessor hands out, or one caller's mutation would edit the
// other radio.
func TestLayouts_AreIndependentValues(t *testing.T) {
	slots := ts590.LayoutS().Slots()
	if len(slots) == 0 {
		t.Fatal("the S declares no slot ranges")
	}
	slots[0].Hi = -1
	if ts590.LayoutS().Slots()[0].Hi == -1 {
		t.Error("LayoutS() hands out the package's own slot slice — a caller could empty the radio's slot space")
	}
	if ts590.LayoutSG().Slots()[0].Hi == -1 {
		t.Error("the S and the SG share one slot slice; the two rows' spaces differ and a shared backing array is one mutation from making them agree")
	}

	names := ts590.LayoutSG().ModeNames()
	delete(names, kw.ModeLSB)
	if _, ok := ts590.LayoutSG().ModeNames()[kw.ModeLSB]; !ok {
		t.Error("LayoutSG() hands out the package's own legend map — a caller could delete a mode from the radio")
	}
	if _, ok := ts590.LayoutS().ModeNames()[kw.ModeLSB]; !ok {
		t.Error("the S and the SG share one legend map")
	}
}

// ceiling is the top of a layout's declared slot space.
func ceiling(l kw.Layout) int {
	hi := -1
	for _, r := range l.Slots() {
		if r.Hi > hi {
			hi = r.Hi
		}
	}
	return hi
}

// hasClass reports whether l declares a range of class c.
func hasClass(l kw.Layout, c kw.SlotClass) bool {
	for _, r := range l.Slots() {
		if r.Class == c {
			return true
		}
	}
	return false
}

// sorted orders slot ranges by their low end, which Layout.Slots does not
// promise.
func sorted(rs []kw.SlotRange) []kw.SlotRange {
	sort.Slice(rs, func(i, j int) bool { return rs[i].Lo < rs[j].Lo })
	return rs
}

// recordFrame renders a 50-byte memory frame from the few bytes these tests
// vary, with every other position at the quiet printed value both books
// admit.
//
// IT IS A TEST-SIDE RENDERER ON PURPOSE. The frames it produces stand in for
// what a RADIO sends, and a helper that called this package's own builder
// could only ever produce frames this package already agrees with.
func recordFrame(command string, byte28 byte, channel, freq string, mode byte, byte3940 string) []byte {
	f := make([]byte, 50)
	for i := range f {
		f[i] = '0'
	}
	f[0], f[1] = command[0], command[1]
	copy(f[3:6], channel)    // P2 and P3
	copy(f[6:17], freq)      // P4
	f[17] = mode             // P5
	f[27] = byte28           // P11
	copy(f[38:40], byte3940) // P14
	copy(f[41:49], "        ")
	f[49] = ';'
	return f
}
