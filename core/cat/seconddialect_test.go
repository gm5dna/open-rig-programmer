// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import "testing"

// testDialect is a deliberately WRONG dialect: a fictional radio whose
// every varying attribute differs from the FT-710's. It exists only in
// this file and is never exported.
//
// Its whole purpose is to fail if any code path consults package-level
// FT-710 data through a Dialect receiver. If someone converts a helper's
// signature but leaves its body reading a global, these tests go red and
// nothing else in the suite does.
var testDialect = Dialect{
	catID: "9999",
	modeNames: map[Mode]string{
		ModeLSB: "LOWER", // the FT-710 calls this "LSB"
		ModeUSB: "UPPER", // the FT-710 calls this "USB"
		// Deliberately omits every other mode the FT-710 knows.
	},
	slots: slotSpace{
		memoryLo: 1, memoryHi: 5, // FT-710: 1-99
		sixtyLo: 0, sixtyHi: 0, // no 60m bank at all
		pmsPairs: 2,  // FT-710: 9
		emgWire:  "", // no emergency channel
		noneWire: "000",
	},
	exItems:   nil,
	exMembers: map[EXAddress]bool{},
}

// noneWireDialect exists for ONE attribute: slotSpace.noneWire, the only
// field of a slotSpace that testDialect above shares with the FT-710
// ("000"). No assertion testDialect can make distinguishes a helper that
// reads d.slots.noneWire from one that hardwires "000" or classifies
// through classifySlotWire — and THREE separate helpers decide "is this
// the none slot": Dialect.classifySlot, Dialect.readableSlot, and
// ParseMTAnswer's own explicit check (mt.go), each of which the milestone
// claims is seam-correct.
//
// So the none form here is "900", and the memory range starts at 0 — which
// makes "000" an ordinary MEMORY channel under this dialect and the
// unemittable none placeholder under the FT-710. Every assertion in
// TestNoneWireIsDialectData turns on that inversion, in both directions.
var noneWireDialect = Dialect{
	catID:     "8888",
	modeNames: map[Mode]string{ModeLSB: "LOWER", ModeUSB: "UPPER"},
	slots: slotSpace{
		memoryLo: 0, memoryHi: 5, // 000 is an ordinary channel here
		sixtyLo: 0, sixtyHi: 0,
		pmsPairs: 0,
		emgWire:  "",
		noneWire: "900", // FT-710: "000"
	},
	exItems:   nil,
	exMembers: map[EXAddress]bool{},
}

// EVERY ASSERTION IN THIS FILE MUST FAIL FOR ITS OWN REASON. A test that
// asserts "testDialect rejects X" passes just as happily when the helper
// is correct, when it is broken in some unrelated way, and when the test's
// own premise is wrong. Two habits keep that honest and both are load
// bearing here:
//
//   - Every rejection is paired with the SAME call on FT710 succeeding, so
//     a helper that started rejecting everything cannot masquerade as a
//     working seam.
//   - Every rejection is paired with a POSITIVE CONTROL on testDialect —
//     the same entry point, an input inside testDialect's own slot space or
//     mode set, which must be ACCEPTED. Without those, replacing any
//     converted helper's body with `return false` would turn this whole
//     file green.
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
		t.Errorf("testDialect claims to know EX address %s — its inventory is empty, so this is reading a global", ft710Addrs[0].Wire())
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
	if _, err := FT710.ParseEXAddress(a.Wire()); err != nil {
		t.Errorf("FT710.ParseEXAddress(%q) failed for its own member: %v", a.Wire(), err)
	}
	if _, err := testDialect.ParseEXAddress(a.Wire()); err == nil {
		t.Errorf("testDialect.ParseEXAddress(%q) succeeded for an address it does not have", a.Wire())
	}

	// ParseEXAnswer: a frame-level entry point into the same membership
	// rule. This is the read direction's equivalent of the allowlist case
	// below — a hardwired membership check inside a PARSER is invisible to
	// every builder-side assertion.
	answer := []byte("EX" + a.Wire() + "0;")
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
func TestEveryConfiguredDialect_HasASaneSlotSpace(t *testing.T) {
	for _, d := range []struct {
		name string
		dia  Dialect
	}{
		{"FT710", FT710},
		{"testDialect", testDialect},
	} {
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
// IT MUST NOT BE ALLOWED TO PASS VACUOUSLY. splitCorpusLine deliberately
// distinguishes a genuine builder REJECTION from a MALFORMED line
// (framecorpus_test.go, fix-round finding M5); folding the second into the
// first would let a truncated or mis-read corpus present as an
// all-rejections clean sweep. So malformed is FATAL here, an empty frame
// with no rejection is FATAL, and the count of frames actually put to the
// zero dialect is asserted non-zero and logged.
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
	if checked == 0 {
		t.Fatal("checked no frames — the corpus or its parser is broken, and this passed vacuously")
	}
	t.Logf("zero Dialect refused all %d frames FT710's builders produced (%d further corpus lines were builder rejections, which carry no frame)", checked, rejected)
}
