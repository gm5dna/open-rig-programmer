// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"fmt"
	"strings"
	"testing"
)

// namedDialect pairs a dialect with a label for table-driven tests.
type namedDialect struct {
	name string
	dia  Dialect
}

// allTestDialects is every CONFIGURED dialect this package can see: the
// real one plus the six fictions in this file — four SHORT-form and two
// COMBINED-form, one of the four (pairDialect) declaring the FOUR-DIGIT EX
// address form. Tests asserting a property that must hold of ANY dialect
// walk this, so adding a dialect later cannot quietly skip them.
//
// The zero Dialect is deliberately absent: it is unconfigured by design
// and its property is the opposite one (it must do nothing at all), tested
// separately by TestZeroDialectRejectsEveryCorpusFrame.
//
// KNOWN LIMIT — PEER-NESS IS NOT ENFORCED. Nothing here checks that at
// least one entry accepts a wire form the FT-710 rejects. A dialect added
// later that is a silent SUBSET of the FT-710, or that has an identical
// slot space, joins this table and every property below still passes —
// which is precisely the root cause that let 18 mutations escape the first
// round of this task (see task-57-report.md §9). peerDialect and
// combinedPeerDialect are the only entries that are peers rather than
// subsets, and they are held here by nothing stronger than this comment.
// A future assertion could compute it: for each dialect, require at least
// one wire form its classifySlot accepts and FT710's rejects. Not added
// now because it would need every dialect to be a peer, and
// testDialect/noneWireDialect are deliberately not.
//
// STRUCTURAL LIMIT — NO DIALECT CAN EVER BE A PMS PEER. pmsCap() clamps to
// 9 and the FT-710 already has 9, so every representable PMS pair set is a
// SUBSET of the FT-710's and peerDialect's P1L-P4U coincides with it.
// Benign in practice — every PMS hardwiring tried is observable from
// testDialect in the narrowing direction — but it is a permanent property
// of this fixture design, not an oversight to fix.
func allTestDialects() []namedDialect {
	return []namedDialect{
		{"FT710", FT710},
		{"testDialect", testDialect},
		{"noneWireDialect", noneWireDialect},
		{"peerDialect", peerDialect},
		{"combinedDialect", combinedDialect},
		{"combinedPeerDialect", combinedPeerDialect},
		{"pairDialect", pairDialect},
	}
}

// testDialect is a deliberately WRONG dialect: a fictional radio whose
// every varying attribute differs from the FT-710's. It exists only in
// this file and is never exported.
//
// Its whole purpose is to fail if any code path consults package-level
// FT-710 data through a Dialect receiver. If someone converts a helper's
// signature but leaves its body reading a global, these tests go red and
// nothing else in the suite does.
var testDialect = mustFixtureDialect(DialectConfig{
	CATID: "9999",
	ModeNames: map[Mode]string{
		ModeLSB: "LOWER", // the FT-710 calls this "LSB"
		ModeUSB: "UPPER", // the FT-710 calls this "USB"
		// Deliberately omits every other mode the FT-710 knows.
	},
	Slots: SlotSpace{
		MemoryLo: 1, MemoryHi: 5, // FT-710: 1-99
		SixtyLo: 0, SixtyHi: 0, // no 60m bank at all
		PMSPairs:      2,  // FT-710: 9
		EmergencyWire: "", // no emergency channel
		NoneWire:      "000",
	},
	EXItems:       nil,
	EXAddressForm: EXAddressTriple,
	MT:            MTPolicy{Form: MTFormShort, TagMaxBytes: 12, ClearTagByte: ' ', PadByte: ' '},
	Clarifier:     ClarifierPolicy{StepHz: 10, MaxAbsHz: 9990},
	MWWriteKind:   KindMemory,
})

// noneWireDialect exists for ONE attribute: slotSpace.noneWire, the only
// field of a slotSpace that testDialect above shares with the FT-710
// ("000"). No assertion testDialect can make distinguishes a helper that
// reads d.slots.noneWire from one that hardwires "000" or (as the
// package-level classifySlotWire did until M9d deleted it) classifies
// against the FT-710 — and THREE separate helpers decide "is this the none
// slot": Dialect.classifySlot, Dialect.readableSlot, and ParseMTAnswer's
// own explicit check (mt.go), each of which the milestone claims is
// seam-correct.
//
// So the none form here is "900", and the memory range starts at 0 — which
// makes "000" an ordinary MEMORY channel under this dialect and the
// unemittable none placeholder under the FT-710. Every assertion in
// TestNoneWireIsDialectData turns on that inversion, in both directions.
var noneWireDialect = mustFixtureDialect(DialectConfig{
	CATID:     "8888",
	ModeNames: map[Mode]string{ModeLSB: "LOWER", ModeUSB: "UPPER"},
	Slots: SlotSpace{
		MemoryLo: 0, MemoryHi: 5, // 000 is an ordinary channel here
		SixtyLo: 0, SixtyHi: 0,
		PMSPairs:      0,
		EmergencyWire: "",
		NoneWire:      "900", // FT-710: "000"
	},
	EXItems:       nil,
	EXAddressForm: EXAddressTriple,
	MT:            MTPolicy{Form: MTFormShort, TagMaxBytes: 12, ClearTagByte: ' ', PadByte: ' '},
	Clarifier:     ClarifierPolicy{StepHz: 10, MaxAbsHz: 9990},
	MWWriteKind:   KindMemory,
})

// peerDialect is the dialect that makes this file's proof complete, and it
// exists because of a MEASURED gap in the two above.
//
// testDialect and noneWireDialect are both SUBSETS of the FT-710: memory
// 1-5 ⊂ 1-99, 2 PMS pairs ⊂ 9, no 60m bank, no EMG, and noneWireDialect's
// 000-005 all parse under FT710 too. Every slot they accept, the FT-710
// accepts. That has a consequence neither of them can escape: WIDENING an
// inner check to the FT-710's rules can never admit anything the outer
// check then rejects, so a whole family of hardwirings — six of them,
// where a Dialect method delegates to an inner d.ParseSlot or
// d.parseMemoryFrame and then re-checks the result — is INVISIBLE to a
// subset dialect. Task 57's first round conceded one of those as
// "undetectable in principle". That was wrong: it is detectable, by a
// dialect that is a PEER of the FT-710 rather than a subset of it.
//
// Three attributes were also varied only by PRESENCE and never by VALUE:
// both dialects above set emgWire "" and sixtyLo/sixtyHi 0,0, which
// exercises each guard and never the datum. Four further hardwirings
// escaped the ENTIRE core/cat suite because of it — including SixtyMSlot
// formatting its wire as fmt.Sprintf("5%02d", n), which is byte-for-byte
// the bug an external review caught at Task 53 and which the constructor
// test below cites as its own reason for existing.
//
// So peerDialect ACCEPTS WHAT THE FT-710 REJECTS, at every attribute:
//
//	memory   100-200   (FT-710 1-99      — disjoint)
//	60m      600-620   (FT-710 501-599   — present, DIFFERENTLY NUMBERED)
//	emgWire  "XYZ"     (FT-710 "EMG"     — present, DIFFERENT VALUE)
//	noneWire "777"     (FT-710 "000"     — different again)
//	mode     'z'       (outside '0'-'9'/'A'-'F' entirely)
//	EX       09 01 01  (FT-710 has no P1=09 group at all)
//	EX P4    16 bytes  (FT-710's widest is 12 — WIDER, not narrower)
//
// Every one of those wire forms is REJECTED by FT710.classifySlot, and
// every peerDialect positive control below therefore fails the moment an
// inner check is widened to the FT-710's.
var peerDialect = mustFixtureDialect(DialectConfig{
	CATID: "7777",
	ModeNames: map[Mode]string{
		ModeUSB:   "USB-PEER", // shared with the FT-710, so frames build for both
		Mode('z'): "ZULU",     // OUTSIDE '0'-'9'/'A'-'F': a mode no range check admits
		// Deliberately omits ModeLSB, which the FT-710 has.
	},
	Slots: SlotSpace{
		MemoryLo: 100, MemoryHi: 200, // FT-710: 1-99, disjoint
		SixtyLo: 600, SixtyHi: 620, // FT-710: 501-599, present but renumbered
		PMSPairs:      4,     // FT-710: 9
		EmergencyWire: "XYZ", // FT-710: "EMG", present but different
		NoneWire:      "777", // FT-710: "000"
	},
	EXItems:       peerEXItems,
	EXAddressForm: EXAddressTriple,
	MT:            MTPolicy{Form: MTFormShort, TagMaxBytes: 12, ClearTagByte: ' ', PadByte: ' '},
	Clarifier:     ClarifierPolicy{StepHz: 10, MaxAbsHz: 9990},
	MWWriteKind:   KindMemory,
})

// ft710P4MaxBytes is the FT-710's own widest P4 answer field: 12, the
// width of its six Table 2 Text items, and until M9b's fix wave the value
// of a package const that EVERY dialect's ParseEXAnswer was bounded by.
//
// It is written here as a LITERAL rather than read back from FT710,
// because the assertions below are the FT-710's bit-identity guard: an
// expectation derived from the same data as the code under test would move
// with it and prove nothing.
const ft710P4MaxBytes = 12

// peerEXItems is a small inventory sharing NO address with the FT-710's:
// that inventory holds P1 groups {1,2,3,4,6} and nothing at P1=9, so no
// address here is a member of it and no address of its 296 is a member of
// this. An EMPTY inventory (testDialect's) only ever proves the negative
// direction; this proves the positive one too, which is what catches an EX
// check widened to the FT-710's index.
//
// THE THIRD ITEM IS 16 DIGITS WIDE, and that is the point of it. The first
// two use widths 3 and 1, both inside the FT-710's 12, and while those
// were the only two here the parser's answer-length bound could be — and
// was — taken from the FT-710's widest Text item and read through every
// dialect's receiver, with no test able to see it. A peer whose menu
// carries a WIDER field than the FT-710's is the only fixture that can:
// under the old bound it would have rejected its own valid answers. See
// TestPeerDialect_EXAnswerLengthBoundIsItsOwn (M9b fix wave, Codex
// finding 1).
var peerEXItems = []EXItem{
	{Addr: EXAddress{P1: 9, P2: 1, P3: 1}, P1Label: "PEER SETTING", P2Label: "PEER GROUP", Name: "PEER ITEM ONE", Digits: 3},
	{Addr: EXAddress{P1: 9, P2: 1, P3: 2}, P1Label: "PEER SETTING", P2Label: "PEER GROUP", Name: "PEER ITEM TWO", Digits: 1},
	{Addr: EXAddress{P1: 9, P2: 1, P3: 3}, P1Label: "PEER SETTING", P2Label: "PEER GROUP", Name: "PEER ITEM THREE (WIDE)", Digits: 16, Text: true},
}

// EVERY ASSERTION IN THIS FILE MUST FAIL FOR ITS OWN REASON. A test that
// asserts "testDialect rejects X" passes just as happily when the helper
// is correct, when it is broken in some unrelated way, and when the test's
// own premise is wrong. Three habits keep that honest and all three are
// load bearing here:
//
//   - Every rejection is paired with a POSITIVE CONTROL on the same
//     dialect — the same entry point, an input inside ITS OWN slot space
//     or mode set, which must be ACCEPTED. Without those, replacing any
//     converted helper's body with `return false` would turn this whole
//     file green. (Measured: a reviewer ran `return false` AND `return
//     true` over eleven helpers; all 20 blanket mutations were caught.)
//   - Wherever a wire form is legal under BOTH dialects, the FT710 side is
//     asserted too, so a helper that started rejecting everything cannot
//     masquerade as a working seam. This does NOT hold call-for-call:
//     ParseSlot("501"), ("EMG") and ("P3L") have no FT710.ParseSlot
//     counterpart here, because the FT710 side of those three is asserted
//     through the CONSTRUCTORS instead (SixtyMSlot, EMGSlot, PMSSlot in
//     TestSecondDialect_SlotConstructorsHonourTheirReceiver).
//   - At least one dialect must be a PEER of the FT-710 rather than a
//     subset, or a widened inner check is unobservable. See peerDialect.
//
// Task 57's report records, assertion by assertion, which of these were
// observed to fail against a deliberately re-hardwired tree.

// TestSecondDialect_SlotSpaceIsHonoured fails if slot classification
// reads a global instead of the receiver.
func TestSecondDialect_SlotSpaceIsHonoured(t *testing.T) {
	if _, err := testDialect.ParseSlot("050"); err == nil {
		t.Error("testDialect parsed slot 050 — its memory range is 1-5, so this is reading the FT-710's slot space")
	}
	if _, err := testDialect.ParseSlot("003"); err != nil {
		t.Errorf("testDialect rejected slot 003, inside its own memory range: %v", err)
	}
	if _, err := testDialect.ParseSlot("501"); err == nil {
		t.Error("testDialect parsed slot 501 — it has no 60m bank")
	}
	if _, err := testDialect.ParseSlot("EMG"); err == nil {
		t.Error("testDialect parsed EMG — it has no emergency channel")
	}
	if _, err := testDialect.ParseSlot("P3L"); err == nil {
		t.Error("testDialect parsed P3L — it has only 2 PMS pairs")
	}
}

// TestSecondDialect_SlotConstructorsHonourTheirReceiver is the
// constructor-side counterpart of the test above. ParseSlot proves the
// DECODE path reads the receiver; MemorySlot/PMSSlot/SixtyMSlot/EMGSlot are
// four separate bodies with their own bounds checks, and each could be
// hardwired independently of classifySlot (SixtyMSlot in particular was
// caught doing exactly that at Task 53 — it read the receiver for the
// bounds and hardcoded a '5' wire prefix).
//
// Each rejection is paired with the equivalent call succeeding on FT710
// and with a positive control on testDialect, so "the constructor always
// errors" cannot pass for "the constructor honours its receiver".
func TestSecondDialect_SlotConstructorsHonourTheirReceiver(t *testing.T) {
	// MemorySlot: 50 is FT-710 range, outside testDialect's 1-5; 3 is
	// inside both.
	if _, err := FT710.MemorySlot(50); err != nil {
		t.Errorf("FT710.MemorySlot(50) failed: %v", err)
	}
	if _, err := testDialect.MemorySlot(50); err == nil {
		t.Error("testDialect.MemorySlot(50) succeeded — its memory range is 1-5, so this is reading the FT-710's bounds")
	}
	if s, err := testDialect.MemorySlot(3); err != nil {
		t.Errorf("testDialect.MemorySlot(3) failed, inside its own range: %v", err)
	} else if s.Wire() != "003" {
		t.Errorf("testDialect.MemorySlot(3).Wire() = %q, want %q", s.Wire(), "003")
	}

	// PMSSlot: pair 3 exists on the FT-710 (9 pairs), not on testDialect
	// (2 pairs). Pair 2 exists on both.
	if _, err := FT710.PMSSlot(3, false); err != nil {
		t.Errorf("FT710.PMSSlot(3, false) failed: %v", err)
	}
	if _, err := testDialect.PMSSlot(3, false); err == nil {
		t.Error("testDialect.PMSSlot(3, false) succeeded — it has only 2 PMS pairs, so this is reading the FT-710's pair count")
	}
	if s, err := testDialect.PMSSlot(2, true); err != nil {
		t.Errorf("testDialect.PMSSlot(2, true) failed, inside its own pair count: %v", err)
	} else if s.Wire() != "P2U" {
		t.Errorf("testDialect.PMSSlot(2, true).Wire() = %q, want %q", s.Wire(), "P2U")
	}

	// SixtyMSlot: testDialect has no 60m bank at all, so EVERY ordinal
	// must fail. There is no positive control to pair with this one — the
	// dialect's own answer is "never" — so FT710's success is what stops
	// it passing for the wrong reason.
	if _, err := FT710.SixtyMSlot(1); err != nil {
		t.Errorf("FT710.SixtyMSlot(1) failed: %v", err)
	}
	if _, err := testDialect.SixtyMSlot(1); err == nil {
		t.Error("testDialect.SixtyMSlot(1) succeeded — it has no 60m bank, so this is reading the FT-710's range")
	}

	// EMGSlot: FT710 has one, testDialect does not, and the absent case
	// must be the zero Slot rather than a borrowed "EMG".
	if got := FT710.EMGSlot().Wire(); got != "EMG" {
		t.Errorf("FT710.EMGSlot().Wire() = %q, want %q", got, "EMG")
	}
	if got := testDialect.EMGSlot().Wire(); got != "" {
		t.Errorf("testDialect.EMGSlot().Wire() = %q, want %q — it has no emergency channel, so this is reading the FT-710's wire form", got, "")
	}
}

// TestSecondDialect_ModeSetIsHonoured fails if mode validation or
// rendering reads the package-level modeNames map.
func TestSecondDialect_ModeSetIsHonoured(t *testing.T) {
	if got := testDialect.ModeName(ModeLSB); got != "LOWER" {
		t.Errorf("testDialect.ModeName(ModeLSB) = %q, want %q — this is reading the FT-710's table", got, "LOWER")
	}
	if testDialect.ValidMode(ModeDATAFMN) {
		t.Error("testDialect claims to know ModeDATAFMN, which is not in its mode set")
	}
	if !FT710.ValidMode(ModeDATAFMN) {
		t.Error("FT710 lost a mode it should have — the two dialects have been conflated")
	}
}

// TestSecondDialect_ParseModeHonoursItsReceiver covers ParseMode, the byte
// decoder reached from parseMemoryFrame and validateMWFields. ValidMode
// and ModeName could both read the receiver correctly whilst ParseMode
// kept a hardcoded '0'-'9'/'A'-'F' range check — which is precisely what
// the package-level form used to be.
func TestSecondDialect_ParseModeHonoursItsReceiver(t *testing.T) {
	// '3' is CW-U: a mode the FT-710 knows and testDialect does not.
	if _, err := FT710.ParseMode('3'); err != nil {
		t.Errorf("FT710.ParseMode('3') failed: %v", err)
	}
	if _, err := testDialect.ParseMode('3'); err == nil {
		t.Error("testDialect.ParseMode('3') succeeded — CW-U is not in its mode set, so this is range-checking rather than consulting the receiver")
	}
	// Positive control: '1' is LSB, which testDialect DOES know.
	if m, err := testDialect.ParseMode('1'); err != nil {
		t.Errorf("testDialect.ParseMode('1') failed, though LSB is in its mode set: %v", err)
	} else if m != ModeLSB {
		t.Errorf("testDialect.ParseMode('1') = %v, want ModeLSB", m)
	}
	// ModeName's fallback must be the placeholder, never the FT-710's name.
	if got := testDialect.ModeName(ModeCWU); got == "CW-U" {
		t.Errorf("testDialect.ModeName(ModeCWU) = %q — it does not know that mode, so this is reading the FT-710's table", got)
	}
}

// TestSecondDialect_EXMembershipIsHonoured fails if EX membership reads
// the package-level index.
func TestSecondDialect_EXMembershipIsHonoured(t *testing.T) {
	ft710Addrs := FT710.EXAddresses()
	if len(ft710Addrs) == 0 {
		t.Fatal("FT710 has no EX addresses")
	}
	if testDialect.KnownEXAddress(ft710Addrs[0]) {
		t.Errorf("testDialect claims to know EX address %s — its inventory is empty, so this is reading a global", FT710.EXWire(ft710Addrs[0]))
	}
	if _, err := testDialect.BuildEXRead(ft710Addrs[0]); err == nil {
		t.Error("testDialect built an EX read for an address it does not have")
	}
}

// TestSecondDialect_EXLookupsHonourTheirReceiver covers the rest of the EX
// surface. KnownEXAddress reads exMembers; NewEXAddress reads exByTriple, a
// SEPARATE index, so one can be converted whilst the other still reads the
// package-level inventory. EXItems/EXAddresses read exItems, a third field.
func TestSecondDialect_EXLookupsHonourTheirReceiver(t *testing.T) {
	ft710Addrs := FT710.EXAddresses()
	if len(ft710Addrs) == 0 {
		t.Fatal("FT710 has no EX addresses")
	}
	a := ft710Addrs[0]

	// NewEXAddress: the decimal-triple index.
	if _, err := FT710.NewEXAddress(int(a.P1), int(a.P2), int(a.P3)); err != nil {
		t.Errorf("FT710.NewEXAddress(%d,%d,%d) failed for its own member: %v", a.P1, a.P2, a.P3, err)
	}
	if _, err := testDialect.NewEXAddress(int(a.P1), int(a.P2), int(a.P3)); err == nil {
		t.Errorf("testDialect.NewEXAddress(%d,%d,%d) succeeded — its inventory is empty, so this is reading a global index", a.P1, a.P2, a.P3)
	}

	// ParseEXAddress: the wire form, which routes through NewEXAddress.
	if _, err := FT710.ParseEXAddress(FT710.EXWire(a)); err != nil {
		t.Errorf("FT710.ParseEXAddress(%q) failed for its own member: %v", FT710.EXWire(a), err)
	}
	if _, err := testDialect.ParseEXAddress(FT710.EXWire(a)); err == nil {
		t.Errorf("testDialect.ParseEXAddress(%q) succeeded for an address it does not have", FT710.EXWire(a))
	}

	// ParseEXAnswer: a frame-level entry point into the same membership
	// rule. This is the read direction's equivalent of the allowlist case
	// below — a hardwired membership check inside a PARSER is invisible to
	// every builder-side assertion.
	answer := []byte("EX" + FT710.EXWire(a) + "0;")
	if _, _, err := FT710.ParseEXAnswer(answer); err != nil {
		t.Errorf("FT710.ParseEXAnswer(%q) failed for its own member: %v", answer, err)
	}
	if _, _, err := testDialect.ParseEXAnswer(answer); err == nil {
		t.Errorf("testDialect.ParseEXAnswer(%q) succeeded for an address it does not have", answer)
	}

	// The inventory accessors themselves.
	if n := len(testDialect.EXItems()); n != 0 {
		t.Errorf("testDialect.EXItems() returned %d items — its inventory is nil, so this is reading a global", n)
	}
	if n := len(testDialect.EXAddresses()); n != 0 {
		t.Errorf("testDialect.EXAddresses() returned %d addresses — its inventory is nil, so this is reading a global", n)
	}
}

// TestSecondDialect_CATIDIsHonoured is the cheapest possible check that
// the identity field is read from the receiver. It is here because CATID
// is the one attribute a caller uses to decide WHICH radio answered, so a
// hardwired one would misidentify every radio as an FT-710.
func TestSecondDialect_CATIDIsHonoured(t *testing.T) {
	if got := FT710.CATID(); got != "0800" {
		t.Errorf("FT710.CATID() = %q, want %q", got, "0800")
	}
	if got := testDialect.CATID(); got != "9999" {
		t.Errorf("testDialect.CATID() = %q, want %q — this is reading the FT-710's identity", got, "9999")
	}
}

// TestSecondDialect_BuildersHonourTheirReceiver walks the slot-taking
// builders, where a hardwired validator hides most easily.
func TestSecondDialect_BuildersHonourTheirReceiver(t *testing.T) {
	ft710Slot, err := FT710.MemorySlot(50) // valid for FT710, not testDialect
	if err != nil {
		t.Fatal(err)
	}

	if _, err := testDialect.BuildMRRead(ft710Slot); err == nil {
		t.Error("testDialect.BuildMRRead accepted slot 050, outside its slot space")
	}
	if _, err := testDialect.BuildMTRead(ft710Slot); err == nil {
		t.Error("testDialect.BuildMTRead accepted slot 050, outside its slot space")
	}
	if _, err := testDialect.BuildMCSet(ft710Slot); err == nil {
		t.Error("testDialect.BuildMCSet accepted slot 050, outside its slot space")
	}
	if _, err := testDialect.BuildMTSet(ft710Slot, true, "TAG"); err == nil {
		t.Error("testDialect.BuildMTSet accepted slot 050, outside its slot space")
	}
	if _, err := testDialect.BuildMWSet(corpusMemoryData(ft710Slot)); err == nil {
		t.Error("testDialect.BuildMWSet accepted slot 050, outside its slot space")
	}
}

// TestSecondDialect_BuildersAcceptTheirOwnSlots is the positive control for
// the test above, and it is not optional. Every assertion up there is a
// rejection, so replacing readableSlot, mcValid, mtSlotValid or
// writableSlot with `return false` would leave all five green whilst
// breaking the codec outright. This pins the other side: the same five
// builders must ACCEPT a slot inside testDialect's OWN space, and each must
// put testDialect's own wire form on the frame.
func TestSecondDialect_BuildersAcceptTheirOwnSlots(t *testing.T) {
	own, err := testDialect.MemorySlot(3) // inside testDialect's 1-5
	if err != nil {
		t.Fatalf("testDialect.MemorySlot(3) failed: %v", err)
	}

	if c, err := testDialect.BuildMRRead(own); err != nil {
		t.Errorf("testDialect.BuildMRRead rejected its own slot 003: %v", err)
	} else if string(c.Bytes()) != "MR003;" {
		t.Errorf("testDialect.BuildMRRead(003) = %q, want %q", c.Bytes(), "MR003;")
	}
	if _, err := testDialect.BuildMTRead(own); err != nil {
		t.Errorf("testDialect.BuildMTRead rejected its own slot 003: %v", err)
	}
	if _, err := testDialect.BuildMCSet(own); err != nil {
		t.Errorf("testDialect.BuildMCSet rejected its own slot 003: %v", err)
	}
	if _, err := testDialect.BuildMTSet(own, true, "TAG"); err != nil {
		t.Errorf("testDialect.BuildMTSet rejected its own slot 003: %v", err)
	}
	if _, err := testDialect.BuildMWSet(corpusMemoryData(own)); err != nil {
		t.Errorf("testDialect.BuildMWSet rejected its own slot 003: %v", err)
	}

	// A PMS slot inside testDialect's 2-pair space must also build, so
	// writableSlot's memory-XOR-PMS rule is exercised on both branches.
	pms, err := testDialect.PMSSlot(2, false)
	if err != nil {
		t.Fatalf("testDialect.PMSSlot(2, false) failed: %v", err)
	}
	if _, err := testDialect.BuildMWSet(corpusMemoryData(pms)); err != nil {
		t.Errorf("testDialect.BuildMWSet rejected its own PMS slot P2L: %v", err)
	}
}

// TestSecondDialect_MWValidationHonoursItsModeSet covers validateMWFields'
// SECOND membership decision. Every other MW assertion in this file varies
// the SLOT, so all of them stay green if only the slot leg reads the
// receiver — and validateMWFields re-validates the caller-forged Mode
// separately, through d.ParseMode (mw.go's SEAM NOTE).
//
// MEASURED: Task 57's mutation sweep reverted that one call to
// FT710.ParseMode and NO assertion in the file caught it. This is the
// assertion that does. It is the only MW path that reaches the mode check
// without first passing through parseMemoryFrame, so nothing else can
// substitute for it: BuildMWSet takes a MemoryData directly.
func TestSecondDialect_MWValidationHonoursItsModeSet(t *testing.T) {
	own, err := testDialect.MemorySlot(3) // inside BOTH dialects' memory range
	if err != nil {
		t.Fatalf("testDialect.MemorySlot(3) failed: %v", err)
	}

	// CW-U is in the FT-710's mode set and not in testDialect's, so the
	// mode field is the only thing that can decide this frame.
	m := corpusMemoryData(own)
	m.Mode = ModeCWU
	if _, err := FT710.BuildMWSet(m); err != nil {
		t.Fatalf("FT710.BuildMWSet rejected mode CW-U, which is in its own mode set: %v", err)
	}
	if _, err := testDialect.BuildMWSet(m); err == nil {
		t.Error("testDialect.BuildMWSet accepted mode CW-U, which is not in its mode set — validateMWFields' mode re-validation is reading a global")
	}

	// Positive control: the same MemoryData with a mode testDialect knows.
	m.Mode = ModeUSB
	if _, err := testDialect.BuildMWSet(m); err != nil {
		t.Errorf("testDialect.BuildMWSet rejected mode USB, which IS in its mode set: %v", err)
	}

	// And the gate: a frame carrying a mode only the other radio knows
	// must not reach the wire. (This one is decided jointly by
	// parseMemoryFrame's mode decode and validateMWFields' re-check —
	// stated here because it is the safety claim, not because it isolates
	// either helper.)
	m.Mode = ModeCWU
	cmd, err := FT710.BuildMWSet(m)
	if err != nil {
		t.Fatalf("FT710.BuildMWSet failed: %v", err)
	}
	if !FT710.AllowedCommand(cmd.Bytes()) {
		t.Fatalf("FT710 refuses its own builder's output %q", cmd.Bytes())
	}
	if testDialect.AllowedCommand(cmd.Bytes()) {
		t.Errorf("testDialect ACCEPTED %q, whose mode CW-U is not in its mode set — the gate is reading a global mode table", cmd.Bytes())
	}
}

// TestSecondDialect_ParsersHonourTheirReceiver covers the path Codex
// finding F3 identified: parseMemoryFrame is shared between ParseMRAnswer
// and AllowedCommand's MW check, and a hardwired helper there is
// invisible to a builder-only corpus.
func TestSecondDialect_ParsersHonourTheirReceiver(t *testing.T) {
	if _, err := testDialect.ParseMCAnswer([]byte("MC050;")); err == nil {
		t.Error("testDialect.ParseMCAnswer accepted slot 050 — the parser is reading the FT-710's slot space")
	}
	if _, err := FT710.ParseMCAnswer([]byte("MC050;")); err != nil {
		t.Errorf("FT710.ParseMCAnswer rejected its own slot 050: %v", err)
	}
	if _, _, _, err := testDialect.ParseMTAnswer([]byte("MT0501TAG;")); err == nil {
		t.Error("testDialect.ParseMTAnswer accepted slot 050")
	}

	// parseMemoryFrame is the helper Codex F3 named, and ParseMRAnswer is
	// its ONLY direct caller. Without this case a hardwired slot decode
	// inside it is invisible to every other test in this file: BuildMWSet
	// reaches validateMWFields rather than parseMemoryFrame, the allowlist
	// case builds an MR *read*, and the zero-dialect test short-circuits on
	// the Configured() guard before any per-command check.
	//
	// ADDED IN REVISION 3 (Task 54's implementer measured the gap and wrote
	// this case, verifying it FAILS on a deliberately reverted tree and
	// PASSES on the committed one — see task-54-report.md).
	//
	// Byte 21 is the P6 mode field (memdata.go). It is set to '1' — LSB,
	// which testDialect DOES know — so the ONLY field left that can reject
	// this frame is the slot.
	mr := []byte("MR099052354000-012010411002;") // golden G7
	mr[21] = '1'
	if _, err := FT710.ParseMRAnswer(mr); err != nil {
		t.Fatalf("FT710 rejected its own golden frame: %v", err)
	}
	if _, err := testDialect.ParseMRAnswer(mr); err == nil {
		t.Error("testDialect.ParseMRAnswer accepted slot 099, outside its 1-5 memory range — parseMemoryFrame is reading a global")
	}
}

// TestSecondDialect_MemoryFrameFieldsHonourTheirReceiver is the positive
// control and the second leg for the parseMemoryFrame case above.
//
// The case above asserts only a REJECTION, which a parseMemoryFrame that
// rejected every frame would satisfy. This pins the other side — the same
// decoder must ACCEPT a frame inside testDialect's own slot space and mode
// set — and separately exercises the MODE leg (d.ParseMode at byte 21),
// which is the other membership decision inside that helper and could be
// hardwired independently of the slot decode.
func TestSecondDialect_MemoryFrameFieldsHonourTheirReceiver(t *testing.T) {
	// Slot 003 (inside testDialect's 1-5), mode '1' = LSB (in its mode
	// set). Both dialects must accept.
	ownSlot := []byte("MR003052354000-012010411002;")
	ownSlot[21] = '1'
	if _, err := FT710.ParseMRAnswer(ownSlot); err != nil {
		t.Fatalf("FT710.ParseMRAnswer rejected %q: %v", ownSlot, err)
	}
	m, err := testDialect.ParseMRAnswer(ownSlot)
	if err != nil {
		t.Fatalf("testDialect.ParseMRAnswer rejected %q, which is inside its own slot space and mode set: %v", ownSlot, err)
	}
	if m.Slot.Wire() != "003" {
		t.Errorf("testDialect.ParseMRAnswer decoded slot %q, want %q", m.Slot.Wire(), "003")
	}
	if m.Mode != ModeLSB {
		t.Errorf("testDialect.ParseMRAnswer decoded mode %v, want ModeLSB", m.Mode)
	}

	// Same slot, mode '4' = FM: the FT-710 knows it, testDialect does not.
	// Only the mode field can distinguish the two here.
	otherMode := []byte("MR003052354000-012010411002;")
	otherMode[21] = '4'
	if _, err := FT710.ParseMRAnswer(otherMode); err != nil {
		t.Fatalf("FT710.ParseMRAnswer rejected %q, whose mode FM it knows: %v", otherMode, err)
	}
	if _, err := testDialect.ParseMRAnswer(otherMode); err == nil {
		t.Error("testDialect.ParseMRAnswer accepted mode FM, which is not in its mode set — parseMemoryFrame's mode decode is reading a global")
	}
}

// TestSecondDialect_MTAndMCParsersAcceptTheirOwnSlots is the positive
// control for the MC/MT rejections in
// TestSecondDialect_ParsersHonourTheirReceiver, for the same reason as
// TestSecondDialect_BuildersAcceptTheirOwnSlots: a parser that rejected
// everything would satisfy those rejections.
func TestSecondDialect_MTAndMCParsersAcceptTheirOwnSlots(t *testing.T) {
	if s, err := testDialect.ParseMCAnswer([]byte("MC003;")); err != nil {
		t.Errorf("testDialect.ParseMCAnswer rejected its own slot 003: %v", err)
	} else if s.Wire() != "003" {
		t.Errorf("testDialect.ParseMCAnswer decoded slot %q, want %q", s.Wire(), "003")
	}
	if s, _, tag, err := testDialect.ParseMTAnswer([]byte("MT0031TAG;")); err != nil {
		t.Errorf("testDialect.ParseMTAnswer rejected its own slot 003: %v", err)
	} else if s.Wire() != "003" || tag != "TAG" {
		t.Errorf("testDialect.ParseMTAnswer decoded slot %q tag %q, want %q / %q", s.Wire(), tag, "003", "TAG")
	}
}

// TestPeerDialect_AcceptsWhatTheFT710Rejects is the positive direction no
// subset dialect can express: four wire forms that are legal under
// peerDialect and INVALID under the FT-710, one per varying slot
// attribute. Any hardwiring of classifySlot's legs, or of any caller that
// widens to the FT-710's rules, turns these from accept to reject.
func TestPeerDialect_AcceptsWhatTheFT710Rejects(t *testing.T) {
	for _, tc := range []struct {
		wire string
		kind slotKind
		what string
	}{
		{"150", slotKindMemory, "memory channel (peer 100-200)"},
		{"605", slotKind60m, "60m channel (peer 600-620)"},
		{"XYZ", slotKindEMG, "emergency channel (peer \"XYZ\")"},
		{"777", slotKindNone, "none placeholder (peer \"777\")"},
	} {
		// The FT-710 must NOT know this form. If it did, the case would
		// prove nothing.
		if _, err := FT710.ParseSlot(tc.wire); err == nil {
			t.Fatalf("premise broken: FT710.ParseSlot(%q) succeeded, so this case cannot distinguish the two dialects", tc.wire)
		}
		if _, err := peerDialect.ParseSlot(tc.wire); err != nil {
			t.Errorf("peerDialect.ParseSlot(%q) failed, though it is this dialect's own %s — a slot check is reading the FT-710's slot space: %v", tc.wire, tc.what, err)
		}
		if got := peerDialect.classifySlot(tc.wire); got != tc.kind {
			t.Errorf("peerDialect.classifySlot(%q) = %v, want %v (%s)", tc.wire, got, tc.kind, tc.what)
		}
	}

	// And the reverse: forms the FT-710 knows and peerDialect must not.
	for _, wire := range []string{"050", "501", "EMG", "000"} {
		if _, err := FT710.ParseSlot(wire); err != nil {
			t.Fatalf("premise broken: FT710.ParseSlot(%q) failed: %v", wire, err)
		}
		if _, err := peerDialect.ParseSlot(wire); err == nil {
			t.Errorf("peerDialect.ParseSlot(%q) succeeded, though that form belongs to the FT-710's slot space and not to its own", wire)
		}
	}
}

// TestEveryDialect_ConstructorsRoundTripThroughTheirOwnParser is the
// property that pins the Task 53 bug for the first time in this
// repository.
//
// A constructor may read its receiver for the BOUNDS and still hardwire
// the WIRE FORM — SixtyMSlot did exactly that, formatting a '5' prefix
// while range-checking against the dialect. An external review caught it
// then; nothing in the tree pinned it afterwards, because both earlier
// test dialects have no 60m bank at all and so exercise the guard rather
// than the datum. Reverting the wire form to fmt.Sprintf("5%02d", n)
// escaped the entire core/cat suite.
//
// The invariant is simple and dialect-independent: WHAT A DIALECT BUILDS,
// THAT SAME DIALECT MUST PARSE, as the kind it meant. peerDialect's 60m
// bank at 600-620 is what gives it teeth.
//
// KNOWN LIMIT — THIS SAMPLES RANGE ENDPOINTS ONLY (memoryLo/memoryHi, the
// first and last 60m channel). A structurally inconsistent slot space
// whose collision falls in the INTERIOR of a range passes green: a
// noneWire or emgWire equal to, say, the middle of its own memory bank is
// caught only if it happens to land on an endpoint. An independent sweep
// measured 8 of 9 structural inconsistencies bound by this property
// (overlapping ranges, inverted ranges, emgWire colliding with a PMS form,
// a 2-byte emgWire, pmsPairs 90, memoryHi 1500); the interior-collision
// case is the ninth. Exhaustive sampling is affordable for the ranges
// these dialects declare, but not for a 5-digit slot space, so this stays
// endpoint-sampled deliberately rather than by omission.
func TestEveryDialect_ConstructorsRoundTripThroughTheirOwnParser(t *testing.T) {
	for _, d := range allTestDialects() {
		check := func(what string, s Slot, err error, want slotKind) {
			t.Helper()
			if err != nil {
				t.Errorf("%s: %s failed: %v", d.name, what, err)
				return
			}
			got, perr := d.dia.ParseSlot(s.Wire())
			if perr != nil {
				t.Errorf("%s: %s built %q, which this SAME dialect's own ParseSlot then rejected — the constructor is reading its receiver for the bounds and hardwiring the wire form: %v", d.name, what, s.Wire(), perr)
				return
			}
			if kind := d.dia.classifySlot(got.Wire()); kind != want {
				t.Errorf("%s: %s built %q, which this dialect classifies as %v, not %v", d.name, what, s.Wire(), kind, want)
			}
		}

		sl := d.dia.slots
		if sl.memoryHi > 0 {
			for _, n := range []int{sl.memoryLo, sl.memoryHi} {
				s, err := d.dia.MemorySlot(n)
				check(fmt.Sprintf("MemorySlot(%d)", n), s, err, slotKindMemory)
			}
		}
		for pair := 1; pair <= d.dia.pmsCap(); pair++ {
			for _, upper := range []bool{false, true} {
				s, err := d.dia.PMSSlot(pair, upper)
				check(fmt.Sprintf("PMSSlot(%d, %v)", pair, upper), s, err, slotKindPMS)
			}
		}
		if sl.sixtyHi > 0 {
			for _, n := range []int{1, sl.sixtyHi - sl.sixtyLo + 1} {
				s, err := d.dia.SixtyMSlot(n)
				check(fmt.Sprintf("SixtyMSlot(%d)", n), s, err, slotKind60m)
			}
		}
		if sl.emgWire != "" {
			check("EMGSlot()", d.dia.EMGSlot(), nil, slotKindEMG)
		}
	}
}

// TestPeerDialect_SixtyMSlotWireFormIsItsOwn states the Task 53 case
// directly as well as by property, so a failure names the bug rather than
// leaving a reader to infer it from a round-trip message.
func TestPeerDialect_SixtyMSlotWireFormIsItsOwn(t *testing.T) {
	for _, tc := range []struct {
		n    int
		want string
	}{
		{1, "600"}, {2, "601"}, {21, "620"},
	} {
		s, err := peerDialect.SixtyMSlot(tc.n)
		if err != nil {
			t.Errorf("peerDialect.SixtyMSlot(%d) failed: %v", tc.n, err)
			continue
		}
		if s.Wire() != tc.want {
			t.Errorf("peerDialect.SixtyMSlot(%d).Wire() = %q, want %q — the wire form is hardwired to the FT-710's 5xx numbering while only the BOUNDS read the receiver (the Task 53 bug)", tc.n, s.Wire(), tc.want)
		}
	}
	// 22 is out of peerDialect's 21-channel bank.
	if _, err := peerDialect.SixtyMSlot(22); err == nil {
		t.Error("peerDialect.SixtyMSlot(22) succeeded, though its bank is 600-620 (21 channels)")
	}
	// EMGSlot's wire form, same shape of bug.
	if got := peerDialect.EMGSlot().Wire(); got != "XYZ" {
		t.Errorf("peerDialect.EMGSlot().Wire() = %q, want %q — the wire form is hardwired to the FT-710's", got, "XYZ")
	}
}

// TestPeerDialect_ModeSetIsItsOwn uses a mode byte OUTSIDE '0'-'9'/'A'-'F'
// so that a ParseMode reverted to a hardcoded range check — which is
// exactly what the package-level form used to be — cannot accept it, and a
// mode the FT-710 has but peerDialect does not, so the converse holds.
func TestPeerDialect_ModeSetIsItsOwn(t *testing.T) {
	if _, err := FT710.ParseMode('z'); err == nil {
		t.Fatal("premise broken: FT710.ParseMode('z') succeeded")
	}
	if m, err := peerDialect.ParseMode('z'); err != nil {
		t.Errorf("peerDialect.ParseMode('z') failed, though 'z' is in its own mode set — this is range-checking rather than consulting the receiver: %v", err)
	} else if got := peerDialect.ModeName(m); got != "ZULU" {
		t.Errorf("peerDialect.ModeName('z') = %q, want %q", got, "ZULU")
	}
	// LSB is the FT-710's and not peerDialect's.
	if _, err := peerDialect.ParseMode('1'); err == nil {
		t.Error("peerDialect.ParseMode('1') succeeded, though LSB is not in its mode set")
	}
}

// TestPeerDialect_EXInventoryIsItsOwn is the positive EX direction an
// EMPTY inventory cannot express. peerDialect's two members sit at P1=09,
// a group the FT-710's Table 2 does not have at all, so each assertion
// below fails the moment an EX check is widened to the FT-710's index.
func TestPeerDialect_EXInventoryIsItsOwn(t *testing.T) {
	own := peerEXItems[0].Addr

	if FT710.KnownEXAddress(own) {
		t.Fatalf("premise broken: FT710 claims to know %s", peerDialect.EXWire(own))
	}
	if !peerDialect.KnownEXAddress(own) {
		t.Errorf("peerDialect does not know %s, its OWN inventory member — KnownEXAddress is reading a global index", peerDialect.EXWire(own))
	}
	if _, err := peerDialect.NewEXAddress(9, 1, 1); err != nil {
		t.Errorf("peerDialect.NewEXAddress(9,1,1) failed for its own member: %v", err)
	}
	if _, err := peerDialect.ParseEXAddress(peerDialect.EXWire(own)); err != nil {
		t.Errorf("peerDialect.ParseEXAddress(%q) failed for its own member: %v", peerDialect.EXWire(own), err)
	}
	if _, err := peerDialect.BuildEXRead(own); err != nil {
		t.Errorf("peerDialect.BuildEXRead failed for its own member %s: %v", peerDialect.EXWire(own), err)
	}
	if _, _, err := peerDialect.ParseEXAnswer([]byte("EX" + peerDialect.EXWire(own) + "123;")); err != nil {
		t.Errorf("peerDialect.ParseEXAnswer failed for its own member %s: %v", peerDialect.EXWire(own), err)
	}
	// The P4 body above is 3 bytes, INSIDE the FT-710's 12: this assertion
	// proves membership, and is blind to the answer-length bound. That is
	// TestPeerDialect_EXAnswerLengthBoundIsItsOwn's job.
	// THE INVENTORY ACCESSORS, ELEMENT BY ELEMENT — NOT BY LENGTH.
	//
	// FIX ROUND 2. This asserted only the CARDINALITY, which pins almost
	// nothing: three mutations of the form "take the length from the
	// receiver, take the payload from the package global" —
	//
	//	out := make([]EXItem, len(d.exItems)); copy(out, exItemsGen)
	//
	// — passed gofmt, go vet and the ENTIRE repository test suite. Under
	// the first, peerDialect.EXItems() returned the FT-710's AF TREBLE
	// GAIN and AF MIDDLE TONE GAIN at 010101/010102 in place of this
	// dialect's own PEER ITEM ONE/TWO at 090101/090102, and every test
	// stayed green. A third variant returned the right ADDRESSES while
	// copying Name/Digits/P1Label from exItemsGen.
	//
	// That is byte-for-byte the shape this whole fix round exists to close
	// — receiver consulted for the bound, datum taken from a global — the
	// Task 53 SixtyMSlot bug wearing a third hat. Cardinality is exactly
	// the property such a mutation preserves, so comparing lengths can
	// never see it. Compare the CONTENT.
	//
	// Digits is not incidental: it is the field ParseEXAnswer's doc
	// comment discusses at length (the M8c read-characterisation, and the
	// one address where the manual is wrong), so an inventory that
	// silently carried the FT-710's widths would mislead exactly the
	// reader that comment is written for.
	items := peerDialect.EXItems()
	if len(items) != len(peerEXItems) {
		t.Fatalf("peerDialect.EXItems() returned %d items, want %d — this is reading a global inventory", len(items), len(peerEXItems))
	}
	for i, want := range peerEXItems {
		if items[i] != want {
			t.Errorf("peerDialect.EXItems()[%d] = %+v,\n want %+v\n — the LENGTH comes from this dialect and the CONTENT from a package global", i, items[i], want)
		}
	}

	// EXAddresses() is a SEPARATE method over the same field and was not
	// called anywhere in this file before fix round 2, so it was pinned by
	// nothing at all for any dialect but the FT-710.
	addrs := peerDialect.EXAddresses()
	if len(addrs) != len(peerEXItems) {
		t.Fatalf("peerDialect.EXAddresses() returned %d addresses, want %d — this is reading a global inventory", len(addrs), len(peerEXItems))
	}
	for i, it := range peerEXItems {
		if addrs[i] != it.Addr {
			t.Errorf("peerDialect.EXAddresses()[%d] = %s, want %s — the length comes from this dialect and the content from a package global", i, peerDialect.EXWire(addrs[i]), peerDialect.EXWire(it.Addr))
		}
	}

	// Belt and braces: not one address this dialect reports may be a
	// member of the FT-710's inventory. This is the assertion that stays
	// meaningful if peerEXItems is ever extended.
	for _, a := range addrs {
		if FT710.KnownEXAddress(a) {
			t.Errorf("peerDialect.EXAddresses() reported %s, which is a member of the FT-710's inventory — this is a global leaking through", peerDialect.EXWire(a))
		}
	}

	// And the FT-710 must refuse peerDialect's address.
	if _, err := FT710.ParseEXAddress(peerDialect.EXWire(own)); err == nil {
		t.Errorf("FT710.ParseEXAddress(%q) succeeded for an address it does not have", peerDialect.EXWire(own))
	}
}

// TestPeerDialect_EXAnswerLengthBoundIsItsOwn is the M9b fix wave's answer
// to Codex finding 1, and the one assertion in this file that fails
// against the tree as it stood at f8fbbda.
//
// ParseEXAnswer bounded EVERY dialect's answer length by a package const,
// exP4MaxBytes = 12, derived from the width of the FT-710's six Table 2
// Text items. A Dialect method reading a package global for a value that
// is plainly per-radio data is the exact shape this milestone exists to
// eliminate, and here it had a consequence rather than merely a smell: a
// dialect whose menu carries a P4 field wider than 12 REJECTED ITS OWN
// VALID ANSWERS. No test in the tree could see it, because every fixture
// inventory was empty or narrower than the FT-710's — peerEXItems used
// only widths 3 and 1, and the positive parser assertion above supplies
// three bytes.
//
// The bound is now derived per dialect from its own inventory
// (dialect.go's maxEXP4Bytes) and this test pins BOTH directions:
//
//   - the peer accepts its own widest answer at exactly its maximum, and
//     rejects one byte past it, so the bound tracks the peer's data and is
//     still a bound rather than "no bound";
//   - the FT-710's stays 12 to the byte — asserted against a literal, not
//     against the code under test — so widening the seam widened nothing
//     for the radio this program actually talks to.
//
// The FT-710 rejections use ITS OWN member address, so the refusal is by
// WIDTH and not by membership; a bound hardwired to the widest inventory
// in the process (16) would pass every other assertion here and fail
// those.
func TestPeerDialect_EXAnswerLengthBoundIsItsOwn(t *testing.T) {
	wide := peerEXItems[2]
	if wide.Digits <= ft710P4MaxBytes {
		t.Fatalf("fixture broken: peerEXItems[2].Digits = %d, which does not exceed the FT-710's %d — the premise of this whole test is a peer whose menu is WIDER", wide.Digits, ft710P4MaxBytes)
	}

	// The derived bounds themselves.
	if got := FT710.exP4MaxBytes(); got != ft710P4MaxBytes {
		t.Errorf("FT710.exP4MaxBytes() = %d, want %d — the FT-710's own answer-length bound has MOVED, which is a behaviour change for every existing FT-710 user of this program", got, ft710P4MaxBytes)
	}
	if got := peerDialect.exP4MaxBytes(); got != wide.Digits {
		t.Errorf("peerDialect.exP4MaxBytes() = %d, want %d (its own widest item) — the bound is not derived from this dialect's inventory", got, wide.Digits)
	}

	// THE PEER, at its maximum and one byte past it.
	atMax := "EX" + peerDialect.EXWire(wide.Addr) + strings.Repeat("W", wide.Digits) + ";"
	if _, raw, err := peerDialect.ParseEXAnswer([]byte(atMax)); err != nil {
		t.Errorf("peerDialect.ParseEXAnswer(%q) REJECTED its own valid answer at its own maximum width of %d — the length bound is the FT-710's, not this dialect's: %v", atMax, wide.Digits, err)
	} else if len(raw) != wide.Digits {
		t.Errorf("peerDialect.ParseEXAnswer(%q) returned a %d-byte P4, want %d", atMax, len(raw), wide.Digits)
	}
	overMax := "EX" + peerDialect.EXWire(wide.Addr) + strings.Repeat("W", wide.Digits+1) + ";"
	if _, raw, err := peerDialect.ParseEXAnswer([]byte(overMax)); err == nil {
		t.Errorf("peerDialect.ParseEXAnswer(%q) ACCEPTED a %d-byte P4, one past its own maximum, returning %q — the bound is not being enforced at all", overMax, wide.Digits+1, raw)
	}

	// THE FT-710, at its maximum and one byte past it, both at one of ITS
	// OWN member addresses so that membership cannot be what decides.
	var ownTwelve EXAddress
	var found bool
	for _, it := range FT710.EXItems() {
		if it.Digits == ft710P4MaxBytes {
			ownTwelve, found = it.Addr, true
			break
		}
	}
	if !found {
		t.Fatalf("fixture broken: no FT-710 inventory item has Digits == %d, so there is no address at which to test its own maximum", ft710P4MaxBytes)
	}
	ft710AtMax := "EX" + FT710.EXWire(ownTwelve) + strings.Repeat("0", ft710P4MaxBytes) + ";"
	if _, raw, err := FT710.ParseEXAnswer([]byte(ft710AtMax)); err != nil {
		t.Errorf("FT710.ParseEXAnswer(%q) rejected a %d-byte P4 at its own %d-digit item — the FT-710's bound has NARROWED: %v", ft710AtMax, ft710P4MaxBytes, ft710P4MaxBytes, err)
	} else if len(raw) != ft710P4MaxBytes {
		t.Errorf("FT710.ParseEXAnswer(%q) returned a %d-byte P4, want %d", ft710AtMax, len(raw), ft710P4MaxBytes)
	}
	ft710OverMax := "EX" + FT710.EXWire(ownTwelve) + strings.Repeat("0", ft710P4MaxBytes+1) + ";"
	if _, raw, err := FT710.ParseEXAnswer([]byte(ft710OverMax)); err == nil {
		t.Errorf("FT710.ParseEXAnswer(%q) ACCEPTED a %d-byte P4 at its own address, returning %q — the FT-710's bound has WIDENED, most likely to the widest inventory in the process rather than to its own", ft710OverMax, ft710P4MaxBytes+1, raw)
	}
	// And at the peer's width specifically, which is the value a
	// process-wide maximum would have taken.
	ft710AtPeerWidth := "EX" + FT710.EXWire(ownTwelve) + strings.Repeat("0", wide.Digits) + ";"
	if _, raw, err := FT710.ParseEXAnswer([]byte(ft710AtPeerWidth)); err == nil {
		t.Errorf("FT710.ParseEXAnswer(%q) ACCEPTED a %d-byte P4 — the peer dialect's width, at an FT-710 address — returning %q: the bound is shared across dialects", ft710AtPeerWidth, wide.Digits, raw)
	}
}

// TestEveryDialect_EXAnswerBoundIsWellOrdered holds of ANY dialect,
// including the two with no EX inventory at all and the zero value: the
// answer-length range must never be inverted.
//
// It exists because the per-dialect bound has a floor. A dialect with no
// items has a true maximum of 0, which would give the range 10..9 — every
// EX answer rejected on LENGTH, before the membership check that ought to
// be doing that work and that the empty fixtures above assert on. The
// floor of 1 keeps the range well-ordered; this pins that it does, and the
// paired membership assertion pins that the rejection still comes from the
// right place.
func TestEveryDialect_EXAnswerBoundIsWellOrdered(t *testing.T) {
	dialects := append(allTestDialects(), namedDialect{"zero", Dialect{}})
	for _, d := range dialects {
		if got := d.dia.exAnswerMaxLen(); got < d.dia.exAnswerMinLen() {
			t.Errorf("%s: exAnswerMaxLen() = %d, below exAnswerMinLen (%d) — the answer-length range is inverted, so this dialect rejects every EX answer on length before membership is ever consulted", d.name, got, d.dia.exAnswerMinLen())
		}
	}

	// An inventory-less dialect must still reject an FT-710 answer for the
	// right reason: it is a member of nothing. The frame is exactly
	// exAnswerMinLen bytes, so the length check cannot be what refuses it.
	ft710Addr := FT710.EXAddresses()[0]
	minimal := []byte("EX" + FT710.EXWire(ft710Addr) + "0;")
	if len(minimal) != FT710.exAnswerMinLen() {
		t.Fatalf("fixture broken: %q is %d bytes, want exAnswerMinLen (%d)", minimal, len(minimal), FT710.exAnswerMinLen())
	}
	for _, d := range []namedDialect{{"testDialect", testDialect}, {"noneWireDialect", noneWireDialect}, {"zero", Dialect{}}} {
		if _, _, err := d.dia.ParseEXAnswer(minimal); err == nil {
			t.Errorf("%s: ParseEXAnswer(%q) succeeded for an address it does not have", d.name, minimal)
		} else if !strings.Contains(err.Error(), "address") {
			t.Errorf("%s: ParseEXAnswer(%q) was refused by %q, not by the membership check — an empty inventory's answer bound has stopped being well-ordered", d.name, minimal, err)
		}
	}
}

// TestPeerDialect_ParsersAcceptItsOwnFrames closes the read-direction half
// of the subsumption family: each parser delegates to an inner d.ParseSlot
// or d.parseMemoryFrame, and widening that inner call to the FT-710's
// rules is invisible to a SUBSET dialect but rejects a PEER's own frames
// outright.
func TestPeerDialect_ParsersAcceptItsOwnFrames(t *testing.T) {
	if s, err := peerDialect.ParseMCAnswer([]byte("MC150;")); err != nil {
		t.Errorf("peerDialect.ParseMCAnswer rejected MC150;, its own memory channel — the inner slot parse is reading the FT-710's slot space: %v", err)
	} else if s.Wire() != "150" {
		t.Errorf("peerDialect.ParseMCAnswer decoded %q, want %q", s.Wire(), "150")
	}
	if _, _, tag, err := peerDialect.ParseMTAnswer([]byte("MT1501TAG;")); err != nil {
		t.Errorf("peerDialect.ParseMTAnswer rejected MT1501TAG;, its own memory channel: %v", err)
	} else if tag != "TAG" {
		t.Errorf("peerDialect.ParseMTAnswer decoded tag %q, want %q", tag, "TAG")
	}

	// parseMemoryFrame, read direction: peerDialect's own slot 150 and its
	// own mode 'z', neither of which the FT-710 can decode.
	mr := []byte("MR150052354000-012010411002;")
	mr[21] = 'z'
	if _, err := FT710.ParseMRAnswer(mr); err == nil {
		t.Fatalf("premise broken: FT710.ParseMRAnswer accepted %q", mr)
	}
	m, err := peerDialect.ParseMRAnswer(mr)
	if err != nil {
		t.Fatalf("peerDialect.ParseMRAnswer rejected %q, whose slot and mode are both its own — parseMemoryFrame is reading a global: %v", mr, err)
	}
	if m.Slot.Wire() != "150" || m.Mode != Mode('z') {
		t.Errorf("peerDialect.ParseMRAnswer decoded slot %q mode %q, want %q / %q", m.Slot.Wire(), byte(m.Mode), "150", "z")
	}
}

// TestPeerDialect_GateAcceptsItsOwnFrames is the security half of the same
// argument, and it is what closes five of the six subsumption instances.
//
// AllowedCommand's per-command checks each parse a slot and then re-check
// it. Under a SUBSET dialect, widening the inner parse to FT710.ParseSlot
// changes nothing observable, because the outer check still rejects
// everything the inner one newly admits — which is why Task 57's first
// round recorded these as escapes and argued one of them was undetectable.
// Under a PEER, the inner FT-710 parse REJECTS the dialect's own legal
// frame and the gate wrongly refuses it. Each case below therefore fails
// on the ACCEPT, not on the refuse.
func TestPeerDialect_GateAcceptsItsOwnFrames(t *testing.T) {
	own, err := peerDialect.MemorySlot(150)
	if err != nil {
		t.Fatalf("peerDialect.MemorySlot(150) failed: %v", err)
	}

	cases := []struct {
		name  string
		build func() (Command, error)
	}{
		{"MR read", func() (Command, error) { return peerDialect.BuildMRRead(own) }},
		{"MT read", func() (Command, error) { return peerDialect.BuildMTRead(own) }},
		{"MC set", func() (Command, error) { return peerDialect.BuildMCSet(own) }},
		{"MT set", func() (Command, error) { return peerDialect.BuildMTSet(own, true, "TAG") }},
		{"MW set", func() (Command, error) { return peerDialect.BuildMWSet(corpusMemoryData(own)) }},
		{"EX read", func() (Command, error) { return peerDialect.BuildEXRead(peerEXItems[0].Addr) }},
	}

	for _, tc := range cases {
		cmd, err := tc.build()
		if err != nil {
			t.Fatalf("%s: peerDialect failed to build for its own slot/address: %v", tc.name, err)
		}
		if !peerDialect.AllowedCommand(cmd.Bytes()) {
			t.Errorf("%s: peerDialect REFUSED %q, its OWN builder's output — this per-command check parses through the FT-710's rules, which reject a peer dialect's legal frame", tc.name, cmd.Bytes())
		}
		if FT710.AllowedCommand(cmd.Bytes()) {
			t.Errorf("%s: FT710 ACCEPTED %q, a frame legal only under another radio's slot space or EX inventory — the gate is not dialect-aware", tc.name, cmd.Bytes())
		}
	}
}

// TestPeerDialect_GateMWDecodeIsIsolated pins the ONE membership decision
// the rest of this file reaches only in company: parseMemoryFrame as
// called by AllowedCommand's MW check (validMWCommand), i.e. the WRITE
// direction of the helper an external review named as the milestone's
// central risk.
//
// The read direction is genuinely isolated — ParseMRAnswer's slot decode
// and its mode decode each fail on their own assertion. The write
// direction was not: the earlier allowlist case mutated
// FT710.parseMemoryFrame AND FT710.validateMWFields together, so mutating
// only the DECODE escaped. Not a live safety bug, since validateMWFields
// re-decides writability and mode on the receiver — but it is one layer
// inside the exact "we only tested it via the accepting path" shape that
// three separate agents already fell into on this specific helper.
//
// Both frames below are rejected by FT710.parseMemoryFrame and accepted by
// peerDialect's, one on the slot field and one on the mode field, so
// either leg of the decode being widened is caught here alone.
func TestPeerDialect_GateMWDecodeIsIsolated(t *testing.T) {
	own, err := peerDialect.MemorySlot(150)
	if err != nil {
		t.Fatalf("peerDialect.MemorySlot(150) failed: %v", err)
	}

	// Leg 1 — the SLOT field. Mode USB is shared, so only the slot can
	// distinguish the two decoders.
	bySlot, err := peerDialect.BuildMWSet(corpusMemoryData(own))
	if err != nil {
		t.Fatalf("peerDialect.BuildMWSet failed: %v", err)
	}
	if _, err := FT710.parseMemoryFrame(bySlot.Bytes(), "MW"); err == nil {
		t.Fatalf("premise broken: FT710.parseMemoryFrame decoded %q", bySlot.Bytes())
	}
	if !peerDialect.AllowedCommand(bySlot.Bytes()) {
		t.Errorf("peerDialect REFUSED its own MW frame %q — validMWCommand's parseMemoryFrame call is decoding against the FT-710's slot space", bySlot.Bytes())
	}

	// Leg 2 — the MODE field. Slot 150 is peer-only either way, so to make
	// the mode the deciding field this frame uses peerDialect's own 'z'
	// mode and asserts the decode leg through the SAME entry point.
	md := corpusMemoryData(own)
	md.Mode = Mode('z')
	byMode, err := peerDialect.BuildMWSet(md)
	if err != nil {
		t.Fatalf("peerDialect.BuildMWSet with mode 'z' failed: %v", err)
	}
	if _, err := FT710.parseMemoryFrame(byMode.Bytes(), "MW"); err == nil {
		t.Fatalf("premise broken: FT710.parseMemoryFrame decoded %q", byMode.Bytes())
	}
	if !peerDialect.AllowedCommand(byMode.Bytes()) {
		t.Errorf("peerDialect REFUSED its own MW frame %q, whose mode 'z' is in its own mode set — validMWCommand's decode is reading a global mode table", byMode.Bytes())
	}

	// The write-direction POLICY still bites on the receiver: a 60m slot is
	// readable but never writable, under any dialect.
	sixty, err := peerDialect.SixtyMSlot(6) // "605"
	if err != nil {
		t.Fatalf("peerDialect.SixtyMSlot(6) failed: %v", err)
	}
	if _, err := peerDialect.BuildMWSet(corpusMemoryData(sixty)); err == nil {
		t.Error("peerDialect.BuildMWSet accepted its own 60m slot 605, which is not writable under any dialect")
	}
	if _, err := peerDialect.BuildMRRead(sixty); err != nil {
		t.Errorf("peerDialect.BuildMRRead rejected its own 60m slot 605, which IS readable: %v", err)
	}
}

// TestEveryConfiguredDialect_HasASaneSlotSpace positively asserts the
// bounds that Dialect.pmsCap() otherwise only clamps.
//
// ADDED IN REVISION 3, carried from Task 53's re-review: clamping an
// out-of-range pmsPairs to 9 closes the correctness cliff but MASKS
// misconfiguration — a data-entry slip of `pmsPairs: 90` would silently
// present as 9, indistinguishable from a deliberate 9, with no error, no
// log and no test failure. That is quieter than this package's fail-closed
// philosophy elsewhere (the zero Dialect is deliberately INERT, not
// plausible-looking). Until M9c's dialect constructor can reject at
// construction time, this table walk is the backstop.
//
// FIX ROUND: it walked only FT710 and testDialect, silently omitting
// noneWireDialect — the dialect whose whole stated role is to be a
// backstop — and would have omitted peerDialect too. It now walks
// allTestDialects(), so a dialect added later is covered by construction
// rather than by whoever remembers to extend a literal.
func TestEveryConfiguredDialect_HasASaneSlotSpace(t *testing.T) {
	for _, d := range allTestDialects() {
		if d.dia.slots.pmsPairs < 0 || d.dia.slots.pmsPairs > 9 {
			t.Errorf("%s: pmsPairs = %d, outside the representable 0-9 (the wire form is P<digit><L|U>) — pmsCap would silently clamp this rather than surface it", d.name, d.dia.slots.pmsPairs)
		}
		if d.dia.slots.memoryHi > 999 || d.dia.slots.sixtyHi > 999 {
			t.Errorf("%s: a slot bound exceeds 999, which the 3-digit wire form cannot represent", d.name)
		}
	}
}

// TestSecondDialect_AllowlistHonoursItsReceiver is the security half: the
// gate must refuse frames that are legal only under another dialect.
func TestSecondDialect_AllowlistHonoursItsReceiver(t *testing.T) {
	ft710Slot, err := FT710.MemorySlot(50)
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := FT710.BuildMRRead(ft710Slot)
	if err != nil {
		t.Fatal(err)
	}

	if !FT710.AllowedCommand(cmd.Bytes()) {
		t.Fatal("FT710 refuses its own builder's output — the property test should already have caught this")
	}
	if testDialect.AllowedCommand(cmd.Bytes()) {
		t.Errorf("testDialect ACCEPTED %q, a frame for a slot outside its space — the gate is reading a global", cmd.Bytes())
	}
}

// TestSecondDialect_AllowlistHonoursItsReceiverPerCommand extends the case
// above across the gate's whole switch, because AllowedCommand dispatches
// to six separate per-command validators and the MR case exercises exactly
// one of them.
//
// The MW case matters most: validMWCommand is the ONLY frame-level entry
// point that reaches parseMemoryFrame with prefix "MW", and it is the
// SAFETY half of the F3 finding — a hardwired decode there would let a
// frame legal only under another radio's slot space through the outbound
// write gate. TestSecondDialect_ParsersHonourTheirReceiver reaches
// parseMemoryFrame only via ParseMRAnswer, i.e. the read direction.
//
// Every rejection here is paired with the same frame ACCEPTED by
// testDialect once its slot is inside testDialect's own space, so a gate
// that refused everything cannot pass this.
func TestSecondDialect_AllowlistHonoursItsReceiverPerCommand(t *testing.T) {
	ft710Slot, err := FT710.MemorySlot(50) // FT-710 only
	if err != nil {
		t.Fatal(err)
	}
	ownSlot, err := testDialect.MemorySlot(3) // both dialects
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		// build produces the frame from the FT-710 dialect, which is the
		// only one guaranteed to be able to build for either slot.
		build func(Slot) (Command, error)
	}{
		{"MR read", FT710.BuildMRRead},
		{"MT read", FT710.BuildMTRead},
		{"MC set", FT710.BuildMCSet},
		{"MT set", func(s Slot) (Command, error) { return FT710.BuildMTSet(s, true, "TAG") }},
		{"MW set", func(s Slot) (Command, error) { return FT710.BuildMWSet(corpusMemoryData(s)) }},
	}

	for _, tc := range cases {
		// Foreign slot: FT710 accepts, testDialect must refuse.
		foreign, err := tc.build(ft710Slot)
		if err != nil {
			t.Fatalf("%s: FT710 failed to build for its own slot 050: %v", tc.name, err)
		}
		if !FT710.AllowedCommand(foreign.Bytes()) {
			t.Fatalf("%s: FT710 refuses its own builder's output %q", tc.name, foreign.Bytes())
		}
		if testDialect.AllowedCommand(foreign.Bytes()) {
			t.Errorf("%s: testDialect ACCEPTED %q, whose slot 050 is outside its 1-5 memory range — this per-command check is reading a global", tc.name, foreign.Bytes())
		}

		// Shared slot: BOTH must accept. Without this the assertion above
		// would pass against a gate that refused every frame.
		shared, err := tc.build(ownSlot)
		if err != nil {
			t.Fatalf("%s: FT710 failed to build for slot 003: %v", tc.name, err)
		}
		if !testDialect.AllowedCommand(shared.Bytes()) {
			t.Errorf("%s: testDialect REFUSED %q, whose slot 003 is inside its own memory range and whose mode is in its own mode set — the gate is not merely dialect-aware, it is broken", tc.name, shared.Bytes())
		}
	}

	// EX read: the gate's seventh case, which consults the EX inventory
	// rather than the slot space. testDialect's inventory is empty, so it
	// must refuse every EX frame — FT710's acceptance is the control.
	addrs := FT710.EXAddresses()
	if len(addrs) == 0 {
		t.Fatal("FT710 has no EX addresses")
	}
	ex, err := FT710.BuildEXRead(addrs[0])
	if err != nil {
		t.Fatalf("FT710.BuildEXRead failed for its own member: %v", err)
	}
	if !FT710.AllowedCommand(ex.Bytes()) {
		t.Fatalf("FT710 refuses its own EX read %q", ex.Bytes())
	}
	if testDialect.AllowedCommand(ex.Bytes()) {
		t.Errorf("testDialect ACCEPTED %q, an EX address it does not have — validEXRead is reading a global inventory", ex.Bytes())
	}
}

// TestNoneWireIsDialectData covers the one slotSpace attribute testDialect
// cannot reach: which 3-byte form means "none" (the VFO/MT/QMB placeholder
// the reference marks UNKNOWN and forbids emitting). See
// noneWireDialect's doc comment.
//
// Under noneWireDialect "000" is an ordinary memory channel and "900" is
// the placeholder; under FT710 it is the other way round. So each pair
// below asserts BOTH dialects, in opposite directions, and no assertion
// here can be satisfied by a helper that simply rejects everything.
func TestNoneWireIsDialectData(t *testing.T) {
	ft710None, err := FT710.ParseSlot("000")
	if err != nil {
		t.Fatalf("FT710.ParseSlot(\"000\") failed: %v", err)
	}
	ownMemory, err := noneWireDialect.ParseSlot("000")
	if err != nil {
		t.Fatalf("noneWireDialect.ParseSlot(\"000\") failed, though 000 is inside its 0-5 memory range: %v", err)
	}
	ownNone, err := noneWireDialect.ParseSlot("900")
	if err != nil {
		t.Fatalf("noneWireDialect.ParseSlot(\"900\") failed, though 900 is its own none form: %v", err)
	}

	// readableSlot: every slot is a legal read target EXCEPT this
	// dialect's own none form.
	if _, err := FT710.BuildMRRead(ft710None); err == nil {
		t.Error("FT710.BuildMRRead accepted 000, its own none placeholder")
	}
	if _, err := noneWireDialect.BuildMRRead(ownMemory); err != nil {
		t.Errorf("noneWireDialect.BuildMRRead rejected 000, an ordinary memory channel under its slot space — readableSlot is reading the FT-710's none form: %v", err)
	}
	if _, err := noneWireDialect.BuildMRRead(ownNone); err == nil {
		t.Error("noneWireDialect.BuildMRRead accepted 900, its OWN none placeholder — readableSlot is reading the FT-710's none form")
	}

	// writableSlot, via BuildMWSet.
	if _, err := FT710.BuildMWSet(corpusMemoryData(ft710None)); err == nil {
		t.Error("FT710.BuildMWSet accepted 000, its own none placeholder")
	}
	if _, err := noneWireDialect.BuildMWSet(corpusMemoryData(ownMemory)); err != nil {
		t.Errorf("noneWireDialect.BuildMWSet rejected 000, an ordinary memory channel under its slot space: %v", err)
	}
	if _, err := noneWireDialect.BuildMWSet(corpusMemoryData(ownNone)); err == nil {
		t.Error("noneWireDialect.BuildMWSet accepted 900, its OWN none placeholder")
	}

	// ParseMTAnswer carries its OWN explicit none check, separate from
	// classifySlot's callers above (mt.go).
	if _, _, _, err := FT710.ParseMTAnswer([]byte("MT0001TAG;")); err == nil {
		t.Error("FT710.ParseMTAnswer accepted 000, its own none placeholder")
	}
	if _, _, _, err := noneWireDialect.ParseMTAnswer([]byte("MT0001TAG;")); err != nil {
		t.Errorf("noneWireDialect.ParseMTAnswer rejected slot 000, an ordinary memory channel under its slot space — the none check is reading the FT-710's wire form: %v", err)
	}
	if _, _, _, err := noneWireDialect.ParseMTAnswer([]byte("MT9001TAG;")); err == nil {
		t.Error("noneWireDialect.ParseMTAnswer accepted 900, its OWN none placeholder")
	}

	// mcValid, via ParseMCAnswer.
	if _, err := FT710.ParseMCAnswer([]byte("MC000;")); err == nil {
		t.Error("FT710.ParseMCAnswer accepted 000, its own none placeholder")
	}
	if _, err := noneWireDialect.ParseMCAnswer([]byte("MC000;")); err != nil {
		t.Errorf("noneWireDialect.ParseMCAnswer rejected 000, an ordinary memory channel under its slot space: %v", err)
	}
	if _, err := noneWireDialect.ParseMCAnswer([]byte("MC900;")); err == nil {
		t.Error("noneWireDialect.ParseMCAnswer accepted 900, its OWN none placeholder")
	}

	// And the gate, which is where getting this wrong stops being a
	// correctness bug and becomes a safety one.
	if FT710.AllowedCommand([]byte("MR000;")) {
		t.Error("FT710.AllowedCommand accepted MR000;, a read of its own none placeholder")
	}
	if !noneWireDialect.AllowedCommand([]byte("MR000;")) {
		t.Error("noneWireDialect.AllowedCommand refused MR000;, a read of an ordinary memory channel under its slot space")
	}
	if noneWireDialect.AllowedCommand([]byte("MR900;")) {
		t.Error("noneWireDialect.AllowedCommand accepted MR900;, a read of its OWN none placeholder — the gate is reading the FT-710's none form")
	}
}

// TestZeroDialectRejectsEveryCorpusFrame is the fail-closed property the
// gate design rests on (Codex F6). A zero Dialect is constructible by any
// caller and its AllowedCommand is a non-nil method value satisfying
// transport.NewEngine's nil check — so it must accept NOTHING.
//
// IT MUST NOT BE ALLOWED TO PASS VACUOUSLY, and the guards below are
// ordered by how likely each is to fire.
//
// The REAL exposure is the count. buildFrameCorpus's structural floor is
// only 4 lines — the four fixed-literal frames emitted before any loop —
// so a slot table trimmed to nothing, an EX inventory that failed to
// generate, or a builder that started rejecting everything would leave
// this test walking a handful of frames and still reporting PASS. The
// floor of 300 below is what makes that loud. It is set well under the
// current 344 so that adding or removing a corpus case is not a failure,
// and well over 4 so that a collapse is.
//
// The malformed guard is a STRUCTURAL INVARIANT, not a live risk: nothing
// is read from disk here, the corpus is built in memory by
// buildFrameCorpus, and every line it emits is "label\tvalue" by
// construction — so splitCorpusLine cannot presently return malformed at
// all. It is kept, and kept FATAL, because the distinction exists in
// splitCorpusLine for a reason (framecorpus_test.go, fix-round finding
// M5): if buildFrameCorpus's line format ever changes, folding malformed
// into rejected is how this test would degrade to a silent all-skip
// instead of failing. Fatal-on-malformed costs nothing and removes that
// future mode.
func TestZeroDialectRejectsEveryCorpusFrame(t *testing.T) {
	var zero Dialect

	corpus := buildFrameCorpus(t)
	if len(corpus) == 0 {
		t.Fatal("buildFrameCorpus returned no lines at all")
	}

	checked, rejected := 0, 0
	for i, line := range corpus {
		cl := splitCorpusLine(line)
		if cl.malformed {
			t.Fatalf("corpus line %d is malformed (%q) — the corpus or its parser is broken. Skipping it would let this test pass vacuously, so it is fatal instead", i+1, line)
		}
		if cl.rejected {
			rejected++
			continue
		}
		if cl.frame == "" {
			t.Fatalf("corpus line %d (%s) yielded an empty frame with no rejection — treat as broken, never as a frame to skip", i+1, cl.label)
		}
		checked++
		if zero.AllowedCommand([]byte(cl.frame)) {
			t.Errorf("zero Dialect ACCEPTED %q (%s) — an unconfigured dialect must accept nothing", cl.frame, cl.label)
		}
	}
	// ASSERTED, not merely logged. See the doc comment: the structural
	// floor is 4, so "checked > 0" is nearly no assurance at all.
	const corpusFloor = 300
	if checked < corpusFloor {
		t.Fatalf("checked only %d frames, below the floor of %d — buildFrameCorpus has collapsed (a trimmed slot table, an EX inventory that failed to generate, or a builder rejecting everything), and this test was about to pass on a fraction of its intended corpus", checked, corpusFloor)
	}
	t.Logf("zero Dialect refused all %d frames FT710's builders produced (%d further corpus lines were builder rejections, which carry no frame)", checked, rejected)
}

// TestEveryConfiguredDialect_ModeNameRoundTripsThroughModeByName is the
// general property ModeByName exists for: for every dialect, every mode's
// display name must resolve back to that same mode.
//
// It binds dialects not yet written, which is the durable half of this
// milestone's evidence — a specific fixture proves one case, a property
// over allDialects() constrains the next radio someone adds.
func TestEveryConfiguredDialect_ModeNameRoundTripsThroughModeByName(t *testing.T) {
	for _, nd := range allTestDialects() {
		checked := 0
		for _, m := range allModeValues() {
			if !nd.dia.ValidMode(m) {
				continue
			}
			name := nd.dia.ModeName(m)
			got, ok := nd.dia.ModeByName(name)
			if !ok {
				t.Errorf("%s: ModeByName(%q) not found, but ModeName(%#02x) returned it", nd.name, name, byte(m))
				continue
			}
			if got != m {
				t.Errorf("%s: ModeByName(%q) = %#02x, want %#02x", nd.name, name, byte(got), byte(m))
			}
			checked++
		}
		if checked == 0 {
			t.Errorf("%s: no modes checked — the property ran vacuously", nd.name)
		}
	}
}

// allModeValues enumerates every possible Mode byte, so the property above
// cannot miss a mode by only walking a table it also trusts.
func allModeValues() []Mode {
	out := make([]Mode, 0, 256)
	for i := 0; i < 256; i++ {
		out = append(out, Mode(i))
	}
	return out
}

// mustFixtureDialect builds a test fixture through the PUBLIC constructor.
//
// Until M9c-0 these three were raw struct literals reaching straight into
// unexported fields. Routing them through NewDialect is this milestone's
// sufficiency proof, and it is a real test rather than tidying: if the
// exported API cannot express a fixture, that is the API being WRONG, and
// M9c would have discovered it at its first task instead of here.
//
// It also removes two hazards the literals carried. They bypassed
// validation entirely, so a fixture could describe a dialect NewDialect
// would refuse; and they left every derived index unset unless someone
// remembered to fill it, which is how a nil mode reverse index made
// ModeByName silently return false for all three.
func mustFixtureDialect(cfg DialectConfig) Dialect {
	d, err := NewDialect(cfg)
	if err != nil {
		panic("seconddialect_test: fixture rejected by NewDialect: " + err.Error())
	}
	return d
}

// combinedDialect is the first of this file's two COMBINED-form fictions,
// and it is here so that every property this package states over "any
// dialect" is stated over a dialect whose MT frames are the FTdx10 family's
// 29+tag combined record rather than the FT-710's short one. Until M9c-3
// every entry above spoke the short form, so "any dialect" meant "any
// short-form dialect" and no walk could tell the difference.
//
// It is the NARROW geometry — a 6-byte tag field padded with ' ' — chosen to
// disagree with combinedPeerDialect on both dimensions the combined form
// derives from its receiver: the exact frame length (35 here, 41 there) and
// the byte that both pads an outbound tag and trims an inbound one.
//
// Its MW write kind is KindMemory ('1'), which is NOT the combined Set's own
// P7 schema constant ('0'), so this fixture carries the DECOUPLING proof into
// every walk that builds its frames: MT-Set P7 and MW-Set P7 are two
// command-specific facts that merely coincide on the evidenced radio, and a
// builder deriving one from the other would put '1' on the wire here.
//
// Its slot space and mode table are the FT-710's, because this fixture's job
// is to vary the MT FORM and nothing else — combinedPeerDialect is the one
// that varies the slot space along with it.
//
// THE FORM-AWARE COUNTERS LIVE IN mtcombined_test.go, in
// TestEveryDialect_MTFormCoverage. The walk in dialectgate_test.go skips any
// builder that returns an error, which is right for it and is exactly why it
// cannot see either half of this seam: the combined frames this dialect DOES
// build reach that walk through no builder it knows, and its refusal of
// BuildMTSet is indistinguishable there from a slot it merely does not have.
var combinedDialect = mustFixtureDialect(DialectConfig{
	CATID:     "6666",
	ModeNames: modeNames, // the FT-710's own table: only the FORM varies here
	Slots: SlotSpace{
		MemoryLo: 1, MemoryHi: 99,
		SixtyLo: 501, SixtyHi: 599,
		PMSPairs:      9,
		EmergencyWire: "EMG",
		NoneWire:      "000",
	},
	EXItems:       combinedEXItems,
	EXAddressForm: EXAddressTriple,
	MT:            MTPolicy{Form: MTFormCombined, TagMaxBytes: 6, TagFill: ' '},
	Clarifier:     ClarifierPolicy{StepHz: 10, MaxAbsHz: 9990},
	MWWriteKind:   KindMemory,
})

// combinedEXItems is combinedDialect's own small inventory. Its P1 group is
// 07, which the FT-710's Table 2 does not have at all, so no address here is
// a member of the FT-710's inventory and the EX properties that walk every
// dialect keep their meaning for this entry.
var combinedEXItems = []EXItem{
	{Addr: EXAddress{P1: 7, P2: 1, P3: 1}, P1Label: "COMBINED SETTING", P2Label: "COMBINED GROUP", Name: "COMBINED ITEM ONE", Digits: 3},
}

// combinedPeerDialect is the second COMBINED-form fiction, and it is a PEER
// in both of this file's senses at once.
//
// In the SLOT sense it carries peerDialect's whole disagreeing posture —
// memory 100-200, a 60m bank at 600-620, "XYZ" for emergency, "777" for
// none, a mode byte outside '0'-'9'/'A'-'F', and the P1=09 inventory whose
// widest item is wider than the FT-710's — so an inner check widened to the
// FT-710's rules is as visible through the combined form as peerDialect
// makes it through the short one. Without this entry the combined form would
// be exercised only by a fixture that is a SUBSET of the FT-710, which is
// the measured gap peerDialect itself exists to close.
//
// In the FORM sense it disagrees with combinedDialect at every dimension the
// combined record derives from its receiver: a 12-byte tag field (the
// geometry the FTdx10 evidence records) rather than 6, filled with '_'
// rather than ' ', and an MW write kind of KindMemTune ('2') rather than
// KindMemory ('1'). A frame length, fill byte or Set kind hardwired to
// either fixture's value is refused by the other.
//
// THE FORM-AWARE COUNTERS LIVE IN mtcombined_test.go, in
// TestEveryDialect_MTFormCoverage — see combinedDialect's comment for why
// the walk in dialectgate_test.go cannot state them.
var combinedPeerDialect = mustFixtureDialect(DialectConfig{
	CATID: "5555",
	ModeNames: map[Mode]string{
		ModeUSB:   "USB-PEER", // shared with the FT-710, so frames build for both
		Mode('z'): "ZULU",     // OUTSIDE '0'-'9'/'A'-'F', as peerDialect's is
	},
	Slots: SlotSpace{
		MemoryLo: 100, MemoryHi: 200, // FT-710: 1-99, disjoint
		SixtyLo: 600, SixtyHi: 620, // FT-710: 501-599, present but renumbered
		PMSPairs:      4,     // FT-710: 9
		EmergencyWire: "XYZ", // FT-710: "EMG"
		NoneWire:      "777", // FT-710: "000"
	},
	EXItems:       peerEXItems,
	EXAddressForm: EXAddressTriple,
	MT:            MTPolicy{Form: MTFormCombined, TagMaxBytes: 12, TagFill: '_'},
	Clarifier:     ClarifierPolicy{StepHz: 10, MaxAbsHz: 9990},
	MWWriteKind:   KindMemTune,
})

// pairEXItems is the four-digit form's inventory: two items in a P1=08
// group the FT-710 does not have, BOTH with P3 == 0.
//
// P3 == 0 is not decoration — it is what V12 requires of every member of a
// EXAddressPair dialect, because a four-digit field renders P1 and P2 only
// and a non-zero P3 would be dropped from every frame silently. The
// disagreeing fixture in exaddressform_test.go asserts the refusal.
//
// The second item's Digits is 5, which is WIDER than the FT-710's numeric
// maximum of 4: the FT-891's own chart carries two five-digit rows, and a
// fixture that stayed inside 1..4 would let a P4 bound taken from the
// FT-710 pass unnoticed here exactly as peerEXItems' 16-byte item exists to
// prevent for the answer bound.
var pairEXItems = []EXItem{
	{Addr: EXAddress{P1: 8, P2: 1, P3: 0}, P1Label: "PAIR SETTING", P2Label: "PAIR GROUP", Name: "PAIR ITEM ONE", Digits: 2},
	{Addr: EXAddress{P1: 8, P2: 3, P3: 0}, P1Label: "PAIR SETTING", P2Label: "PAIR GROUP", Name: "PAIR ITEM TWO", Digits: 5},
}

// pairDialect is the fixture that makes the EX address WIDTH a variable
// rather than a constant, and it is in allTestDialects() so that every
// gate, conformance and round-trip property in this package runs over a
// four-digit dialect as well as a six-digit one.
//
// Its EX read frame is SEVEN bytes ("EX" + 4 + ";"), against the FT-710's
// nine. Before EXAddressForm existed every one of those lengths was the
// package constant exReadLen = 9, consulted through a Dialect receiver —
// the seam shape this file exists to catch — so a fixture that merely had a
// different INVENTORY could never see it. This one can: a length hardwired
// to 9 refuses this dialect's own builder's output at its own gate, which
// TestEveryDialect_BuiltFramesAreCleanAndGateAdmissible reports.
//
// Everything else is deliberately unremarkable (short MT form, a small
// memory range, no 60m bank, no emergency channel) so that a failure here
// points at the address width and not at some second difference.
var pairDialect = mustFixtureDialect(DialectConfig{
	CATID: "6666",
	ModeNames: map[Mode]string{
		ModeLSB: "LSB-PAIR",
		ModeUSB: "USB-PAIR",
	},
	Slots: SlotSpace{
		MemoryLo: 1, MemoryHi: 20,
		SixtyLo: 0, SixtyHi: 0,
		PMSPairs:      2,
		EmergencyWire: "",
		NoneWire:      "000",
	},
	EXItems:       pairEXItems,
	EXAddressForm: EXAddressPair, // the FT-891's four-digit field (EXAddressPair's doc comment, dialectconfig.go, has the naming caveat)
	MT:            MTPolicy{Form: MTFormShort, TagMaxBytes: 12, ClearTagByte: ' ', PadByte: ' '},
	Clarifier:     ClarifierPolicy{StepHz: 10, MaxAbsHz: 9990},
	MWWriteKind:   KindMemory,
})
