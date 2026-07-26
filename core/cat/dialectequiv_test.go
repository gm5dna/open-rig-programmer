// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"fmt"
	"testing"
)

// ft710ConfigFromIndependentLiterals describes the FT-710 for NewDialect
// WITHOUT reading anything back out of FT710 or its source tables.
//
// Independence is the whole point. An earlier draft of this test passed
// modeNames and the FT-710's own slot values straight through — the very
// objects the production literal is built from — so it compared FT710
// against itself and would have passed with the constructor storing its
// input under the wrong field names, or with FT710 itself wrong (Codex
// plan review, finding 4).
//
// Modes are written by WIRE BYTE rather than by the ModeLSB-style
// constants, so the fixture does not depend on those either. The names are
// transcribed from the reference's mode table, not copied from
// core/cat/mode.go's map.
//
// EXItems is the one input that is NOT independent, and saying so matters
// more than papering over it: the inventory is 296 generated rows and
// hand-transcribing it here would be a second transcription with its own
// error rate and no cross-check. What this test proves about EX is
// therefore narrower and stated exactly at each assertion — that the
// constructor DERIVES its three EX indices correctly from whatever
// inventory it is given, checked against recomputation rather than against
// FT710's stored fields.
func ft710ConfigFromIndependentLiterals() DialectConfig {
	return DialectConfig{
		CATID: "0800",
		ModeNames: map[Mode]string{
			'0': "-",
			'1': "LSB",
			'2': "USB",
			'3': "CW-U",
			'4': "FM",
			'5': "AM",
			'6': "RTTY-L",
			'7': "CW-L",
			'8': "DATA-L",
			'9': "RTTY-U",
			'A': "DATA-FM",
			'B': "FM-N",
			'C': "DATA-U",
			'D': "AM-N",
			'E': "PSK",
			'F': "DATA-FM-N",
		},
		Slots: SlotSpace{
			MemoryLo: 1, MemoryHi: 99,
			SixtyLo: 501, SixtyHi: 599,
			PMSPairs:      9,
			EmergencyWire: "EMG",
			NoneWire:      "000",
		},
		EXItems:     exItemsGen, // NOT independent — see the doc comment
		MT:          MTPolicy{TagMaxBytes: 12, ClearTagByte: ' '},
		Clarifier:   ClarifierPolicy{StepHz: 10, MaxAbsHz: 9990},
		MWWriteKind: KindMemory,
	}
}

// TestNewDialect_ReproducesFT710 is the sufficiency proof: the exported API
// is expressive enough to describe a real radio, and the FT-710's own data
// passes its own validation.
//
// Comparison is BEHAVIOURAL, not reflect.DeepEqual. A constructed dialect
// and a struct literal may legitimately differ in nil-versus-empty derived
// containers while behaving identically, and DeepEqual would report that as
// a failure — which invites someone to "fix" it by weakening the check.
func TestNewDialect_ReproducesFT710(t *testing.T) {
	got, err := NewDialect(ft710ConfigFromIndependentLiterals())
	if err != nil {
		t.Fatalf("NewDialect with the FT-710's data: %v — the shipped dialect fails its own validation", err)
	}
	assertDialectsBehaveIdentically(t, "FT710", FT710, got)
}

// assertDialectsBehaveIdentically compares two dialects across every
// observable this package exposes.
//
// The list is deliberately exhaustive rather than representative. An
// earlier draft checked CATID, modes and a "slot corpus" left undefined,
// and would have passed while a dialect carried zero policies, a corrupted
// EX inventory, or a wrong derived index (Codex plan review, finding 4).
func assertDialectsBehaveIdentically(t *testing.T, label string, want, got Dialect) {
	t.Helper()

	if want.CATID() != got.CATID() {
		t.Errorf("%s: CATID = %q, want %q", label, got.CATID(), want.CATID())
	}
	if want.Configured() != got.Configured() {
		t.Errorf("%s: Configured = %v, want %v", label, got.Configured(), want.Configured())
	}

	// Modes over ALL 256 byte values, not just the table's own keys: a
	// dialect that knows an EXTRA mode is just as wrong as one missing a
	// mode, and only exhausting the space catches the former.
	for i := 0; i < 256; i++ {
		m := Mode(i)
		if want.ValidMode(m) != got.ValidMode(m) {
			t.Errorf("%s: ValidMode(%#02x) = %v, want %v", label, i, got.ValidMode(m), want.ValidMode(m))
		}
		if want.ModeName(m) != got.ModeName(m) {
			t.Errorf("%s: ModeName(%#02x) = %q, want %q", label, i, got.ModeName(m), want.ModeName(m))
		}
		// The reverse index too — a dialect can agree on every forward
		// lookup and still have an inverted table built from the wrong map.
		wm, wok := want.ModeByName(want.ModeName(m))
		gm, gok := got.ModeByName(want.ModeName(m))
		if wok != gok || wm != gm {
			t.Errorf("%s: ModeByName(%q) = (%#02x,%v), want (%#02x,%v)", label, want.ModeName(m), byte(gm), gok, byte(wm), wok)
		}
	}

	// Slot classification over an EXHAUSTIVE corpus: every 3-digit numeric
	// form, every PMS form, and a set of malformed shapes. "A slot corpus"
	// left to the implementer's judgement is how a range boundary goes
	// unchecked.
	for n := 0; n <= 999; n++ {
		wire := fmt.Sprintf("%03d", n)
		if want.classifySlot(wire) != got.classifySlot(wire) {
			t.Errorf("%s: classifySlot(%q) = %v, want %v", label, wire, got.classifySlot(wire), want.classifySlot(wire))
		}
	}
	for _, pair := range []byte{'1', '2', '3', '4', '5', '6', '7', '8', '9'} {
		for _, end := range []byte{'L', 'U'} {
			wire := string([]byte{'P', pair, end})
			if want.classifySlot(wire) != got.classifySlot(wire) {
				t.Errorf("%s: classifySlot(%q) = %v, want %v", label, wire, got.classifySlot(wire), want.classifySlot(wire))
			}
		}
	}
	for _, wire := range []string{"", "0", "00", "0000", "EMG", "emg", "P0L", "PAL", "P1X", "***", "0 0", "00;"} {
		if want.classifySlot(wire) != got.classifySlot(wire) {
			t.Errorf("%s: classifySlot(%q) = %v, want %v", label, wire, got.classifySlot(wire), want.classifySlot(wire))
		}
	}

	// EX inventory: EXACT contents and order, not just length. Metadata
	// included — a constructor that dropped ObservedReadWidth would leave
	// every membership test passing.
	wi, gi := want.EXItems(), got.EXItems()
	if len(wi) != len(gi) {
		t.Fatalf("%s: EXItems length = %d, want %d", label, len(gi), len(wi))
	}
	for i := range wi {
		if wi[i] != gi[i] {
			t.Errorf("%s: EXItems[%d] = %+v, want %+v", label, i, gi[i], wi[i])
		}
	}
	wa, ga := want.EXAddresses(), got.EXAddresses()
	if len(wa) != len(ga) {
		t.Fatalf("%s: EXAddresses length = %d, want %d", label, len(ga), len(wa))
	}
	for i := range wa {
		if wa[i] != ga[i] {
			t.Errorf("%s: EXAddresses[%d] = %v, want %v (order is part of the contract)", label, i, ga[i], wa[i])
		}
	}

	// BOTH EX lookup paths. KnownEXAddress reads exMembers; NewEXAddress
	// reads exByTriple. Checking only the first leaves the second index
	// entirely unverified, and it is the one a caller-supplied triple goes
	// through.
	for _, a := range wa {
		if want.KnownEXAddress(a) != got.KnownEXAddress(a) {
			t.Errorf("%s: KnownEXAddress(%v) = %v, want %v", label, a, got.KnownEXAddress(a), want.KnownEXAddress(a))
		}
		_, werr := want.NewEXAddress(int(a.P1), int(a.P2), int(a.P3))
		_, gerr := got.NewEXAddress(int(a.P1), int(a.P2), int(a.P3))
		if (werr == nil) != (gerr == nil) {
			t.Errorf("%s: NewEXAddress(%v) error = %v, want error = %v", label, a, gerr, werr)
		}
	}
	// A triple that is NOT a member, so the negative direction is covered
	// too: an index that accepted everything would pass the loop above.
	for _, miss := range [][3]int{{5, 5, 5}, {99, 99, 99}, {0, 0, 0}} {
		_, werr := want.NewEXAddress(miss[0], miss[1], miss[2])
		_, gerr := got.NewEXAddress(miss[0], miss[1], miss[2])
		if (werr == nil) != (gerr == nil) {
			t.Errorf("%s: NewEXAddress%v error = %v, want error = %v", label, miss, gerr, werr)
		}
	}

	if want.exP4MaxBytes() != got.exP4MaxBytes() {
		t.Errorf("%s: exP4MaxBytes = %d, want %d", label, got.exP4MaxBytes(), want.exP4MaxBytes())
	}

	// The three promoted policies. Omitting these is how a dialect with
	// entirely zero policies passes an "equivalence" test.
	if want.mt != got.mt {
		t.Errorf("%s: MT policy = %+v, want %+v", label, got.mt, want.mt)
	}
	if want.clar != got.clar {
		t.Errorf("%s: clarifier policy = %+v, want %+v", label, got.clar, want.clar)
	}
	if want.mwWriteKind != got.mwWriteKind {
		t.Errorf("%s: MWWriteKind = %#02x, want %#02x", label, got.mwWriteKind, want.mwWriteKind)
	}
}

// TestNewDialect_DerivedEXIndicesMatchRecomputation covers the half of the
// FT-710 comparison that cannot be independent.
//
// Because ft710ConfigFromIndependentLiterals passes exItemsGen — the same
// slice the production literal uses — the EX assertions above compare like
// with like. This test closes that gap from the other side: it recomputes
// each derived index here, from the inventory, by a method written
// independently of dialect.go's builders, and checks the constructor agrees.
func TestNewDialect_DerivedEXIndicesMatchRecomputation(t *testing.T) {
	d, err := NewDialect(ft710ConfigFromIndependentLiterals())
	if err != nil {
		t.Fatalf("NewDialect: %v", err)
	}
	items := d.EXItems()
	if len(items) == 0 {
		t.Fatal("no EX items — this test would run vacuously")
	}

	// Membership, recomputed by linear scan rather than by map lookup.
	for _, it := range items {
		found := false
		for _, other := range items {
			if other.Addr == it.Addr {
				found = true
				break
			}
		}
		if found != d.KnownEXAddress(it.Addr) {
			t.Errorf("KnownEXAddress(%v) = %v, linear scan says %v", it.Addr, d.KnownEXAddress(it.Addr), found)
		}
	}

	// Widest P4, recomputed by scan.
	widest := 0
	for _, it := range items {
		if it.Digits > widest {
			widest = it.Digits
		}
	}
	if got := d.exP4MaxBytes(); got != widest {
		t.Errorf("exP4MaxBytes = %d, scan of the inventory says %d", got, widest)
	}

	// The triple index, checked against a scan for a sample of members and
	// for a triple no item holds.
	for _, it := range items[:10] {
		if _, err := d.NewEXAddress(int(it.Addr.P1), int(it.Addr.P2), int(it.Addr.P3)); err != nil {
			t.Errorf("NewEXAddress(%v) = %v, but that triple is in the inventory", it.Addr, err)
		}
	}
}

// TestNewDialect_InputIndependenceAcrossEveryDerivedStructure checks the
// copy promise through ALL FOUR derived structures, not just the two that
// are easy to reach.
//
// Task 62's constructor test covers membership and the P4 width. This adds
// exByTriple — observable only through NewEXAddress — and modeByName, which
// were both unverified (Codex plan review, finding 9). A constructor that
// copied the slice but built one index from the caller's original would
// pass the narrower test and fail here.
func TestNewDialect_InputIndependenceAcrossEveryDerivedStructure(t *testing.T) {
	cfg := DialectConfig{
		CATID:     "5555",
		ModeNames: map[Mode]string{Mode('1'): "ONE", Mode('2'): "TWO"},
		Slots: SlotSpace{
			MemoryLo: 1, MemoryHi: 50,
			PMSPairs: 2, NoneWire: "000",
		},
		EXItems: []EXItem{
			{Addr: EXAddress{P1: 3, P2: 1, P3: 1}, Name: "A", Digits: 2},
			{Addr: EXAddress{P1: 3, P2: 1, P3: 2}, Name: "B", Digits: 4},
		},
		MT:          MTPolicy{TagMaxBytes: 10, ClearTagByte: ' '},
		Clarifier:   ClarifierPolicy{StepHz: 10, MaxAbsHz: 100},
		MWWriteKind: KindMemory,
	}

	d, err := NewDialect(cfg)
	if err != nil {
		t.Fatalf("NewDialect: %v", err)
	}

	// Everything the dialect should keep saying afterwards.
	const (
		wantWidth = 4
		wantName  = "ONE"
	)
	memberBefore := EXAddress{P1: 3, P2: 1, P3: 2}

	// Now scribble on every container the caller still holds.
	cfg.ModeNames[Mode('1')] = "MUTATED"
	cfg.ModeNames[Mode('9')] = "ADDED"
	cfg.EXItems[0].Addr = EXAddress{P1: 9, P2: 9, P3: 9}
	cfg.EXItems[1].Addr = EXAddress{P1: 8, P2: 8, P3: 8}
	cfg.EXItems[1].Digits = 99

	// 1. modeNames
	if got := d.ModeName(Mode('1')); got != wantName {
		t.Errorf("ModeName('1') = %q after caller mutation, want %q", got, wantName)
	}
	if d.ValidMode(Mode('9')) {
		t.Error("ValidMode('9') = true — a mode ADDED to the caller's map after construction reached the dialect")
	}

	// 2. modeByName
	if m, ok := d.ModeByName("ONE"); !ok || m != Mode('1') {
		t.Errorf("ModeByName(%q) = (%#02x,%v) after caller mutation, want ('1',true)", wantName, byte(m), ok)
	}
	if _, ok := d.ModeByName("MUTATED"); ok {
		t.Error("ModeByName found a name the caller wrote into its map after construction")
	}

	// 3. exMembers
	if !d.KnownEXAddress(memberBefore) {
		t.Errorf("KnownEXAddress(%v) = false after caller mutation — membership followed the caller's slice", memberBefore)
	}
	if d.KnownEXAddress(EXAddress{P1: 9, P2: 9, P3: 9}) {
		t.Error("KnownEXAddress reports an address the caller wrote in after construction")
	}

	// 4. exByTriple — reachable ONLY through NewEXAddress, which is why it
	// went unchecked before.
	if _, err := d.NewEXAddress(3, 1, 2); err != nil {
		t.Errorf("NewEXAddress(3,1,2) = %v after caller mutation, want the original member", err)
	}
	if _, err := d.NewEXAddress(8, 8, 8); err == nil {
		t.Error("NewEXAddress accepted a triple the caller wrote in after construction")
	}

	// 5. exP4Max, derived from Digits.
	if got := d.exP4MaxBytes(); got != wantWidth {
		t.Errorf("exP4MaxBytes = %d after caller mutation, want %d", got, wantWidth)
	}
}
