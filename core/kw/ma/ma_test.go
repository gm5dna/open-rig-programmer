// SPDX-License-Identifier: GPL-3.0-or-later

package ma

import (
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/kw"
)

// THE OUTBOUND GATE'S OWN SUITE: the positive roster, the negative roster,
// the two-tier conformance walk and the zero-value walk, over both layouts.
//
// IT IS AN IN-PACKAGE WALK RATHER THAN A kwtest-SHAPED SUITE, which is spec
// decision 10's dividend. core/kw needed core/kw/kwtest because its two model
// packages cannot reach its in-package fixtures; both MA layouts live here, so
// the walk is an ordinary range over the two values.
//
// THIS FILE PROVES WHAT AllowedCommand REFUSES. internal/guards'
// TestMAFramingGatesTheRoster proves that the predicate the ENGINE really
// holds is that same method, and NEITHER SUBSTITUTES FOR THE OTHER: a gate
// whose method is perfect and whose constructor wired kw.NewFraming instead
// would pass every test in this file.

// bothLayouts is the walk's subject: the two registry rows, in book order.
func bothLayouts() []Layout { return []Layout{Layout890(), Layout990()} }

// otherLayout returns the row l is not.
func otherLayout(l Layout) Layout {
	if l.Book() == kw.Book890 {
		return Layout990()
	}
	return Layout890()
}

// exMember returns an EX address in l's inventory that is NOT in the other
// row's — the row-specific EX frame tier 2 needs.
//
// IT IS COMPUTED FROM THE TWO INVENTORIES rather than transcribed, because a
// literal address written here would be a third copy of a generated artefact
// and would rot the first time either chart was re-transcribed. The two
// charts are different books' charts, so such an address exists; if one ever
// did not, this helper fails loudly rather than leaving tier 2 vacuous.
func exMember(t *testing.T, l Layout) kw.EXAddress {
	t.Helper()
	other := otherLayout(l)
	for _, it := range l.EXItems() {
		if _, ok := other.EXItem(it.Addr); !ok {
			return it.Addr
		}
	}
	t.Fatalf("%s: every address in its inventory is also in the %s's — tier 2 has no row-specific EX frame to drive", l.Model(), other.Model())
	return kw.EXAddress{}
}

// positiveCorpus is every frame THIS ROW's builders produce: the seven
// grammars of spec decision 5, one row's spelling each.
//
// IT IS BUILT THROUGH THE BUILDERS, never written out as literals, which is
// what makes "the gate admits exactly what this package builds" a property
// rather than two tables that happen to agree today.
func positiveCorpus(t *testing.T, l Layout) map[string][]byte {
	t.Helper()
	must := func(cmd Command, err error) []byte {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", l.Model(), err)
		}
		return cmd.Bytes()
	}
	return map[string][]byte{
		"ID read":    must(l.BuildIDRead()),
		"AI read":    must(l.BuildAIRead()),
		"AI set off": must(l.BuildAISetOff()),
		"FV read":    must(l.BuildFVRead()),
		"EX read":    must(l.BuildEXRead(exMember(t, l))),
		"MA0 read":   must(l.BuildMA0Read(slotOf(t, l, 7))),
		"MA0 set":    must(l.BuildMA0Set(rowRecord(t, l))),
	}
}

// rowRecord is a Record this row's own grid can carry: the codec suites'
// fixture frame, parsed back into a record.
func rowRecord(t *testing.T, l Layout) Record {
	t.Helper()
	rec, err := l.ParseMA0Answer([]byte(rowSetFrame(l)))
	if err != nil {
		t.Fatalf("%s: ParseMA0Answer of its own fixture: %v", l.Model(), err)
	}
	return rec
}

// rowSetFrame is this row's own MA0 Set frame — 40 + len(name) bytes on the
// TS-890S (890:3166-3182), a fixed 57 on the TS-990S (990:2893-2915). The two
// are tier 2's pair: neither row's grid can carry the other's.
func rowSetFrame(l Layout) string {
	if l.Book() == kw.Book890 {
		return frame890("GB3IV")
	}
	return frame990("GB3       ")
}

// TestAllowedCommand_AdmitsExactlyTheSevenGrammars is the POSITIVE half, and
// without it every refusal in this file is satisfied by a gate that admits
// nothing.
//
// The always-false placeholder this task replaced is exactly that gate, and
// this test is its red proof: every frame here is one a builder in this
// package produced, so a gate refusing any of them would leave a driver
// unable to send a frame its own codec built.
func TestAllowedCommand_AdmitsExactlyTheSevenGrammars(t *testing.T) {
	for _, l := range bothLayouts() {
		corpus := positiveCorpus(t, l)
		if len(corpus) != 7 {
			t.Fatalf("%s: the positive corpus holds %d frames, want the seven grammars of decision 5", l.Model(), len(corpus))
		}
		for what, frame := range corpus {
			if !l.AllowedCommand(frame) {
				t.Errorf("%s: the gate refused %q, which its own %s builder produced", l.Model(), frame, what)
			}
		}
	}
}

// TestAllowedCommand_AdmitsOnlyFramesTheEnvelopeAlsoAdmits pins the INCLUSION
// the conjunction rests on: the seven grammars are strictly narrower than the
// envelope both books print.
//
// It is what makes NewFramingFor's conjunction free, and it is why
// TestNewFramingFor_KeepsTheEnvelopeInFrontOfTheRoster has to substitute a
// predicate to find its witness: with the real roster there is no frame the
// gate admits and the envelope refuses.
func TestAllowedCommand_AdmitsOnlyFramesTheEnvelopeAlsoAdmits(t *testing.T) {
	for _, l := range bothLayouts() {
		envelope, err := kw.NewFraming(l.Book())
		if err != nil {
			t.Fatalf("%s: kw.NewFraming: %v", l.Model(), err)
		}
		for what, frame := range positiveCorpus(t, l) {
			if !envelope.Allow(frame) {
				t.Errorf("%s: the ENVELOPE refuses %q (%s), which the roster admits — the grammars must be narrower than the envelope, or NewFramingFor's conjunction would refuse this row's own traffic", l.Model(), frame, what)
			}
		}
	}
}

// negativeRoster is every frame the books print that this programme never
// sends, and driving it is the single most important test in the milestone: a
// physical radio would act on each of them.
//
// SPEC DECISION 15 IS WHAT PUTS THEM HERE, with decision 5's roster beside it.
// Decision 5 admits seven grammars across five opcodes; decision 15 gives each
// refusal its reason — MA1 and MI are the create route decision 6 declines;
// MA2 duplicates the name MA0 P13 already carries; MA3 duplicates the lockout
// with a DIFFERENT encoding on the 990S (E8); MA4 is a radio-side copy this
// programme has no verb for; MA5 erases and this programme has a standing
// no-erase rule; MA6 writes a Programmable VFO end frequency; MA7 temporarily
// changes the displayed frequency and is not a codeplug operation at all; MV
// switches the radio between VFO and memory mode; and MN is refused on exactly
// the same ground as MV — an MN Set changes the radio's selected memory
// channel. MA0's parameter list names no band on either radio (890:3164-3221,
// 990:2891-2965), so nothing this milestone sends needs an MN in front of it,
// and there is no BuildMNSet or BuildMNRead anywhere in this package. QA, QD
// and QI are the Quick Memory commands, refused on decision 5's default arm.
//
// MR, MW, MC AND TY ARE ANOTHER PAIR'S BOOK ENTIRELY. Neither of these two
// documents prints one, so a gate admitting them would be admitting a TS-590
// or TS-480 frame to a radio that has no such command — the cross-model
// borrowing the whole per-row gate exists to refuse.
//
// EVERY FRAME IS SPELLED FROM ITS OWN GRID'S POSITION RULER, and BOTH books'
// spellings are driven against BOTH layouts: the 990S's MN, MI and MV carry
// that book's Main/Sub P1 and the 890S's do not, and a gate must refuse both
// shapes on both rows.
func negativeRoster() []struct{ what, frame string } {
	return []struct{ what, frame string }{
		// Pair 1's book, printed nowhere in these two.
		{"MR read, which no book here prints", "MR0007;"},
		{"MW set, which no book here prints", "MW0007" + strings.Repeat("0", 43) + ";"},
		{"MC read, which no book here prints", "MC;"},
		{"MC set, which no book here prints", "MC007;"},
		{"TY read, which no book here prints", "TY;"},

		// TS-890S: MA1-MA7, MI, MN, MV, QA, QD, QI.
		{"890S MA1 Set (890:3227-3233)", "MA1" + "00014250000" + "2" + "0" + ";"},
		{"890S MA2 Set (890:3255-3258)", "MA2" + "007" + " " + "GB3IV" + ";"},
		{"890S MA3 Set (890:3273-3276)", "MA3" + "007" + "1" + ";"},
		{"890S MA4 Set (890:3290-3293)", "MA4" + "007" + "008" + ";"},
		{"890S MA5 Set, the printed erase (890:3305-3311)", "MA5" + "007" + ";"},
		{"890S MA6 Set (890:3317-3323)", "MA6" + "100" + "00014250000" + ";"},
		{"890S MA7 Set (890:3338-3344, E2 — this row carries no P1)", "MA7" + "00014250000" + ";"},
		{"890S MA7 Read (890:3345-3347)", "MA7" + "0" + ";"},
		{"890S MI Set (890:3594-3597)", "MI" + "007" + ";"},
		{"890S MN Set (890:3647-3650)", "MN" + "007" + ";"},
		{"890S MN Read (890:3652-3654)", "MN;"},
		{"890S MV Set (890:3783-3786)", "MV" + "1" + ";"},
		{"890S MV Read (890:3787-3789)", "MV;"},
		{"890S QA Read (890:4293-4296)", "QA" + "0" + ";"},
		{"890S QD Set (890:4328-4330)", "QD;"},
		{"890S QI Set (890:4341-4343)", "QI;"},

		// TS-990S: MA1-MA6 — this book prints no MA7 at all, which is E7 —
		// then MI, MN, MV, QA, QD, QI.
		{"990S MA1 Set (990:2975-2981)", "MA1" + "00014250000" + "2" + "0" + ";"},
		{"990S MA2 Set (990:2999-3005)", "MA2" + "007" + " " + "GB3       " + ";"},
		{"990S MA3 Set (990:3016-3019)", "MA3" + "007" + "1" + ";"},
		{"990S MA4 Set (990:3031-3034)", "MA4" + "007" + "008" + ";"},
		{"990S MA5 Set, the printed erase (990:3042-3047)", "MA5" + "007" + ";"},
		{"990S MA6 Set (990:3053-3059)", "MA6" + "100" + "00014250000" + ";"},
		{"990S MI Set (990:3260-3263)", "MI" + "0" + "007" + ";"},
		{"990S MN Set (990:3326-3329)", "MN" + "0" + "007" + ";"},
		{"990S MN Read (990:3331-3333)", "MN" + "0" + ";"},
		{"990S MV Set (990:3463-3466)", "MV" + "0" + "1" + ";"},
		{"990S MV Read (990:3468-3470)", "MV" + "0" + ";"},
		{"990S QA Read (990:4010-4013)", "QA" + "0" + ";"},
		{"990S QD Set (990:4049-4051)", "QD;"},
		{"990S QI Set (990:4061-4063)", "QI;"},
	}
}

// TestAllowedCommand_RefusesTheWholeNegativeRoster is the decision-5 gate.
// See negativeRoster for what is on it and why each entry is there.
func TestAllowedCommand_RefusesTheWholeNegativeRoster(t *testing.T) {
	roster := negativeRoster()
	if len(roster) == 0 {
		t.Fatal("the negative roster is empty, so every refusal below passed vacuously")
	}
	for _, l := range bothLayouts() {
		for _, tc := range roster {
			if l.AllowedCommand([]byte(tc.frame)) {
				t.Errorf("%s: the gate ADMITTED %s — %q is a frame a physical radio would act on and nothing in this package builds it (decision 15)", l.Model(), tc.what, tc.frame)
			}
		}
	}
}

// TestAllowedCommand_RefusesTheShapesTheDisciplineForbids is core/cat's gate
// discipline restated as a table: an answer frame outbound, an embedded
// terminator, a non-printed AI state, and an EX read of a NON-MEMBER address.
//
// THE EX ARM IS THE ONE WORTH NAMING. It asks the same INVENTORY MEMBERSHIP
// question BuildEXRead asks — never a scalar bound — because both charts are
// sparse and a ceiling would admit an address no chart prints. core/kw could
// not ask its own inventories that question (core/kw/ex.go: they live in the
// packages that IMPORT it, so the check would be an import cycle); here both
// layouts and both generated inventories are in one package, so the obstacle
// does not exist.
func TestAllowedCommand_RefusesTheShapesTheDisciplineForbids(t *testing.T) {
	for _, l := range bothLayouts() {
		// An address inside the printed WIDTH and outside both charts.
		nonMember := kw.EXAddress{P1: 1, P2: 99, P3: 99}
		if _, ok := l.EXItem(nonMember); ok {
			t.Fatalf("%s: %v is in its inventory, so this probe tests nothing", l.Model(), nonMember)
		}
		for _, tc := range []struct{ what, frame string }{
			{"an EX read of an address this chart does not print", "EX19999;"},
			{"an EX read whose address field is not digits", "EX1 999;"},
			{"the ID ANSWER, which is a frame the radio sends", "ID" + l.CATID() + ";"},
			{"an AI state other than 0", "AI2;"},
			{"two commands in one write", "ID;ID;"},
			{"a leading terminator", ";ID;"},
			{"no terminator at all", "ID"},
			{"a lower-case opcode", "id;"},
			{"the empty frame", ""},
		} {
			if l.AllowedCommand([]byte(tc.frame)) {
				t.Errorf("%s: the gate admitted %s: %q", l.Model(), tc.what, tc.frame)
			}
		}
	}
}

// TestZeroLayout_BuildsNothingParsesNothingAdmitsNothing is RunZeroValue's
// shape for this package. A zero ma.Layout is constructible by any caller and
// its methods are all perfectly non-nil, so every entry point must fail closed
// rather than speak for a radio it does not describe.
func TestZeroLayout_BuildsNothingParsesNothingAdmitsNothing(t *testing.T) {
	var l Layout

	if l.Configured() {
		t.Fatal("the zero Layout reports itself configured")
	}

	// Builds nothing.
	for what, err := range map[string]error{
		"BuildIDRead":   errOf(l.BuildIDRead()),
		"BuildAIRead":   errOf(l.BuildAIRead()),
		"BuildAISetOff": errOf(l.BuildAISetOff()),
		"BuildFVRead":   errOf(l.BuildFVRead()),
		"BuildEXRead":   errOf(l.BuildEXRead(kw.EXAddress{})),
		"BuildMA0Read":  errOf(l.BuildMA0Read(Slot{})),
		"BuildMA0Set":   errOf(l.BuildMA0Set(Record{})),
	} {
		if err == nil {
			t.Errorf("the zero Layout's %s built a frame", what)
		}
	}

	// Parses nothing.
	if _, err := l.ParseIDAnswer([]byte("ID024;")); err == nil {
		t.Error("the zero Layout parsed an ID answer")
	}
	if _, err := l.ParseFVAnswer([]byte("FV1.00;")); err == nil {
		t.Error("the zero Layout parsed an FV answer")
	}
	if _, err := l.ParseMA0Answer([]byte(frame890("GB3IV"))); err == nil {
		t.Error("the zero Layout parsed an MA0 answer")
	}
	if _, err := l.ParseEXAnswer([]byte("EX00000 123;"), kw.EXItem{Digits: 3}); err == nil {
		t.Error("the zero Layout parsed an EX answer")
	}

	// Resolves no slot, correlates no answer, gates for no radio.
	if _, err := l.NewSlot(7); err == nil {
		t.Error("the zero Layout resolved a slot")
	}
	if l.MA0AnswerMatcher(Slot{})([]byte(frame890("GB3IV"))) {
		t.Error("the zero Layout's matcher correlated a frame")
	}
	if _, err := NewFramingFor(l); err == nil {
		t.Error("NewFramingFor built a framing for the zero Layout")
	}

	// AND ADMITS NOTHING — asserted against the frames the two configured
	// rows really build, so this half cannot pass on a corpus of rubbish.
	seen := 0
	for _, real := range bothLayouts() {
		for what, frame := range positiveCorpus(t, real) {
			seen++
			if l.AllowedCommand(frame) {
				t.Errorf("the zero Layout admitted %s's %s (%q)", real.Model(), what, frame)
			}
		}
	}
	if seen == 0 {
		t.Fatal("no frame was offered to the zero layout, so its gate passed vacuously")
	}
}

// errOf is a builder's error alone, so the table above reads as one line per
// entry point.
func errOf(_ Command, err error) error { return err }

// TestAllowedCommand_TheTwoTierConformanceWalk is the conformance suite, and
// it is TWO TIERS because a blanket cross-refusal is IMPOSSIBLE here.
//
// "ID;", "AI;", "AI0;", "FV;" and every MA0 Read are byte-identical and valid
// on BOTH radios, so "every builder output is refused by the other layout"
// could only pass by wrongly refusing documented common frames. That is the
// per-family / per-radio distinction core/kw's own gate already draws
// (core/kw/allowlist.go's three tiers). So:
//
//   - TIER 1, the COMMON frames, must be ADMITTED BY BOTH, asserted rather
//     than merely allowed. A gate refusing them would leave one row unable to
//     send a frame both books print identically.
//   - TIER 2, the ROW-SPECIFIC frames and VALUES, must be REFUSED BY THE
//     OTHER: the 57-byte 990S MA0 Set against the 890S layout and the 890S's
//     variable-length Set against the 990S's; an EX read of an address in one
//     row's inventory and not the other's; and a VALUE case that is not a
//     shape case at all — mode byte 'I', printed on the 990S's legend
//     (990:3706-3730) and absent from the 890S's, which stops at 'F'
//     (890:3976-3992), inside an otherwise 890S-shaped frame.
//
// NON-VACUITY IS COUNTED IN BOTH TIERS: a layout contributing no frame to
// either fails loudly, and so does a tier that ends up empty.
func TestAllowedCommand_TheTwoTierConformanceWalk(t *testing.T) {
	// TIER 1 — the same bytes on both radios.
	common := []string{
		"ID;",     // 890:2735, 990:2614
		"AI;",     // 890:181, 990:179
		"AI0;",    // 890:175-177, 990:173-175
		"FV;",     // 890:2655, 990:2532
		"MA0000;", // 890:3184-3186, 990:2916-2918
		"MA0007;",
		"MA0119;",
	}
	tier1 := 0
	for _, l := range bothLayouts() {
		for _, frame := range common {
			tier1++
			if !l.AllowedCommand([]byte(frame)) {
				t.Errorf("%s: tier 1 frame %q was refused — both books print it, byte for byte", l.Model(), frame)
			}
		}
	}
	if tier1 == 0 {
		t.Fatal("tier 1 is empty, so the common frames passed vacuously")
	}

	// TIER 2 — this row's own frames and values, refused by the other.
	tier2 := 0
	for _, l := range bothLayouts() {
		other := otherLayout(l)
		mine := map[string][]byte{
			"its own MA0 Set grid":                    []byte(rowSetFrame(l)),
			"an EX read of an address only it prints": positiveCorpus(t, l)["EX read"],
		}
		for what, frame := range mine {
			tier2++
			if !l.AllowedCommand(frame) {
				t.Errorf("%s: its own gate refused %s (%q)", l.Model(), what, frame)
			}
			if other.AllowedCommand(frame) {
				t.Errorf("%s ADMITTED %s's %s (%q) — a frame legal on one row must be refused by a layout describing the other", other.Model(), l.Model(), what, frame)
			}
		}
	}

	// The VALUE half: an 890S-SHAPED frame carrying the 990S's mode byte.
	// Its refusal is the legend's, not the length's, which is the whole
	// reason this case is listed separately from the two Set grids.
	tier2++
	if modeI := []byte(with890("GB3IV", 18, "I")); Layout890().AllowedCommand(modeI) {
		t.Errorf("the 890S admitted %q — mode 'I' is FM-D2 on the 990S's legend (990:3706-3730) and the 890S's chart stops at 'F' (890:3976-3992)", modeI)
	}
	if tier2 == 0 {
		t.Fatal("tier 2 is empty, so the cross-refusals passed vacuously")
	}
}
