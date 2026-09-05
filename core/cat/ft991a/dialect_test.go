// SPDX-License-Identifier: GPL-3.0-or-later

package ft991a_test

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/cat/dialecttest"
	"github.com/gm5dna/open-rig-programmer/core/cat/ft891"
	"github.com/gm5dna/open-rig-programmer/core/cat/ft991a"
	"github.com/gm5dna/open-rig-programmer/core/cat/ftdx10"
	"github.com/gm5dna/open-rig-programmer/internal/extable"
)

// This file is Stage 1's dialect evidence: the FT-991A dialect held to
// core/cat's conformance suite, then pinned against the two siblings whose
// readings it splits.
//
// THERE ARE TWO COUNTERPARTS HERE, NOT ONE, and that is the difference from
// core/cat/ft891/dialect_test.go, which uses the FTdx10 throughout. This
// radio disagrees with the FTdx10 about the slot space, the EX address and
// the mode table, and with the FT-891 about bytes 21 and 28 of the shared
// memory block — where the FT-891 prints "(Fixed)" this radio prints a live
// state, so EACH RADIO BUILDS WHAT THE OTHER REFUSES. A pin against one
// sibling alone would leave half of that unsaid.
//
// Every pin below is a fact about TWO manuals and cites both. Without the
// counter-example half, "this dialect refuses X" proves only that X is
// refused by everybody.
//
// The comparisons are BEHAVIOURAL — asked through the exported API, over
// the whole wire space where a space exists — rather than by comparing two
// tables. A table comparison would prove the tables match and say nothing
// about what either dialect DOES with them.
//
// ASSUMED MEMBERS ARE EMBEDDED IN WHAT THE PINS COMPARE and are noted at
// each site: cat.ModeUnset ('0', "-") is in no FT-991A mode legend, the none
// wire "000" is in no FT-991A slot legend, and the combined answer's exact
// 41 is core/cat's form assumption rather than this chart's. doc.go's
// ASSUMED register carries the full statement and the Stage R capture that
// lifts each; TestASSUMEDRegisterIsComplete holds this file's list of them
// against that register mechanically.

// TestConformance runs core/cat's whole exported-API conformance suite over
// the real FT-991A dialect.
//
// It is the first run of that suite over a dialect declaring the NUMERIC PMS
// form, the five-state P8 domain and the single-component EX address at once
// that is not a synthetic Stage 0 fixture.
func TestConformance(t *testing.T) {
	dialecttest.Run(t, ft991a.Dialect())
}

// TestZeroValue runs the universal zero-value suite from OUTSIDE core/cat,
// for the reason core/cat/ft891/dialect_test.go gives: this package is a
// consumer of the exported API, and a zero cat.Dialect is reachable from any
// consumer that declares one by mistake.
func TestZeroValue(t *testing.T) {
	dialecttest.RunZeroValue(t)
}

// modeNibbles is the FT-991A's mode legend beside the FTdx10's, nibble by
// nibble, as the two manuals print them.
//
// FT991A is transcribed from the FIVE identical FT-991A memory legends —
// MR's P6 at ft991a_layout.txt:973-975, MT's at 1006-1008, MW's at
// 1044-1046, IF's at 789-791 and OI's at 1124-1126 — and FTdx10 from that
// radio's five (ftdx10_layout.txt: IF 999-1001, MR 1192-1194, MT 1227-1229,
// MW 1267-1269, OI 1349-1352).
//
// BOTH SIDES DELIBERATELY LEAVE THE MD LEGEND OUT, because on this radio MD
// is the rival spelling: ft991a_layout.txt:927-929 prints "3: CW-U" and
// "7: CW-L" where the five memory legends print "3: CW" and "7: CW-R". The
// FTdx10's MD P2
// legend (ftdx10_layout.txt:1146-1149) agrees with that radio's memory five
// on every nibble, so including it would change nothing here; it is held out
// so that the comparison is memory-legend against memory-legend, which is
// the only comparison this package treats as like for like. (The FTdx10's OI
// printing carries a defect of its own — 1352 reads "E: PSK E: DATA-FM-N",
// a duplicated key where "F:" belongs — which does not touch the fourteen
// nibbles compared below.)
//
// SEVEN AGREE AND SEVEN DISAGREE over the fourteen nibbles both radios fill,
// which is the whole reason neither package references the other's table.
// 'E' is the interesting one and is marked: both radios name a REAL mode
// there and they are different modes, not two spellings of one.
var modeNibbles = []struct {
	Wire            byte
	FT991A, FTdx10  string
	SharedSpelling  bool
	BothRealDiffers bool
}{
	{Wire: '1', FT991A: "LSB", FTdx10: "LSB", SharedSpelling: true},
	{Wire: '2', FT991A: "USB", FTdx10: "USB", SharedSpelling: true},
	{Wire: '3', FT991A: "CW", FTdx10: "CW-U"},
	{Wire: '4', FT991A: "FM", FTdx10: "FM", SharedSpelling: true},
	{Wire: '5', FT991A: "AM", FTdx10: "AM", SharedSpelling: true},
	{Wire: '6', FT991A: "RTTY-LSB", FTdx10: "RTTY-L"},
	{Wire: '7', FT991A: "CW-R", FTdx10: "CW-L"},
	{Wire: '8', FT991A: "DATA-LSB", FTdx10: "DATA-L"},
	{Wire: '9', FT991A: "RTTY-USB", FTdx10: "RTTY-U"},
	{Wire: 'A', FT991A: "DATA-FM", FTdx10: "DATA-FM", SharedSpelling: true},
	{Wire: 'B', FT991A: "FM-N", FTdx10: "FM-N", SharedSpelling: true},
	{Wire: 'C', FT991A: "DATA-USB", FTdx10: "DATA-U"},
	{Wire: 'D', FT991A: "AM-N", FTdx10: "AM-N", SharedSpelling: true},
	// 'E' IS NOT A SPELLING DISAGREEMENT. The FT-991A prints "E: C4FM", a
	// digital voice mode the FTdx10 does not have; the FTdx10 prints
	// "E: PSK", which appears in no FT-991A legend. Two radios, one nibble,
	// two different modes.
	{Wire: 'E', FT991A: "C4FM", FTdx10: "PSK", BothRealDiffers: true},
}

// TestModeLegendTranscription pins the mode table's CONTENTS LITERALLY, over
// all 256 wire bytes, against the legend printed five times in manual
// revision 1711-D.
//
// The whole byte space, not the fifteen this package declares: a table with
// a SPURIOUS member — a lower-case 'a', a stray 'F' inherited from the
// FTdx10 — is invisible to a walk over its own keys, and that is precisely
// the copy error a fresh transcription risks.
//
// THE FIVE PRINTINGS ARE PINNED AS A LITERAL, NOT RE-READ. A test re-reading
// the extract is impossible here: ft991a_layout.txt is gitignored, so it is
// absent from a fresh clone and from CI. What is mechanical instead is that
// the transcription in dialect.go equals the transcription in this file,
// which were written at different times from the same five legends.
func TestModeLegendTranscription(t *testing.T) {
	d := ft991a.Dialect()

	want := make(map[byte]string, len(modeNibbles)+1)
	for _, m := range modeNibbles {
		want[m.Wire] = m.FT991A
	}
	// THE ASSUMED MEMBER, stated rather than smuggled into the table above.
	// cat.ModeUnset appears in no FT-991A mode legend; it is here because
	// parsers must accept the placeholder, and core/cat refuses to emit it
	// in any Set frame. doc.go's register entry "THE cat.ModeUnset MEMBER OF
	// THE MODE TABLE" carries the Stage R capture that lifts it.
	want[byte(cat.ModeUnset)] = "-"

	for c := 0; c < 256; c++ {
		m := cat.Mode(byte(c))
		wantName, wantValid := want[byte(c)]
		if got := d.ValidMode(m); got != wantValid {
			t.Errorf("ValidMode(%#02x): got %v, want %v — the legend printed at ft991a_layout.txt:973-975, 1006-1008, 1044-1046, 789-791 and 1124-1126 runs 1..9 then A..E, with no hole and no 'F'", c, got, wantValid)
			continue
		}
		if !wantValid {
			continue
		}
		if got := d.ModeName(m); got != wantName {
			t.Errorf("ModeName(%#02x) = %q, want %q", c, got, wantName)
		}
		// ModeByName is the WRITE direction: how a stored channel's mode
		// string becomes a wire byte again. A name resolving to a different
		// nibble would write the wrong mode into a memory, which no
		// read-side comparison above would notice.
		if got, ok := d.ModeByName(wantName); !ok || got != m {
			t.Errorf("ModeByName(%q) = (%#02x, %v), want (%#02x, true) — ModeName and ModeByName are inverses", wantName, byte(got), ok, c)
		}
	}

	if got, want := len(want), 15; got != want {
		t.Errorf("this file's transcription declares %d members, want %d — fourteen printed names plus the ASSUMED placeholder", got, want)
	}
}

// TestModeStringFallbackIsWrongHere is the first test in this repository to
// demonstrate cat.Mode.String()'s package-level fallback being ACTIVELY
// WRONG rather than merely unauthoritative.
//
// Mode.String() reads core/cat's own package-level table, which is the
// FT-710's (core/cat/mode.go:160-182 — the doc comment already says
// "ANYTHING USER-VISIBLE MUST GO THROUGH Dialect.ModeName INSTEAD"). On
// every dialect registered before this one, the fallback's answer for a
// nibble was either the same word or a word for a mode the radio does not
// have. Here it is a DIFFERENT REAL MODE: the FT-710's table says 'E' is
// "PSK"; this radio's five legends say 'E' is "C4FM".
//
// The pin is what stops the distinction being read as academic. doc.go
// records it, and a matching clause was added to Mode.String()'s own doc
// comment.
func TestModeStringFallbackIsWrongHere(t *testing.T) {
	const nibble = 'E'
	m := cat.Mode(nibble)

	if got, want := m.String(), "PSK"; got != want {
		t.Errorf("cat.Mode('E').String() = %q, want %q — the fallback reads core/cat's package-level table, which is the FT-710's; if that has changed, this radio's exposure has changed with it", got, want)
	}
	if got, want := ft991a.Dialect().ModeName(m), "C4FM"; got != want {
		t.Errorf("Dialect().ModeName('E') = %q, want %q — ft991a_layout.txt:1008 prints \"E: C4FM\"", got, want)
	}
	if m.String() == ft991a.Dialect().ModeName(m) {
		t.Error("the fallback and this dialect agree on 'E' — this test exists because they do NOT, and an agreement here means one of the two tables has taken the other's word")
	}
	// The counter-example half: on the FTdx10 the fallback is RIGHT, so the
	// wrongness above is a fact about this radio and not about the fallback
	// being broken for everybody.
	if got := ftdx10.Dialect().ModeName(m); got != m.String() {
		t.Errorf("the FTdx10's ModeName('E') = %q where the fallback says %q — the FTdx10's legend prints \"E: PSK\" (ftdx10_layout.txt:1146-1149) and the fallback matches it, which is what makes THIS radio the first for which it does not", got, m.String())
	}
}

// TestDifferencePinCATID is the identity that makes this a different radio
// at all: the FT-991A answers "ID;" with 0670 (manual revision 1711-D,
// ft991a_layout.txt:772), the FTdx10 with 0761 and the FT-891 with 0650.
func TestDifferencePinCATID(t *testing.T) {
	if got := ft991a.Dialect().CATID(); got != "0670" {
		t.Errorf("CATID() = %q, want %q — the ID block prints \"P1 0670: FT-991A\"", got, "0670")
	}
	for _, other := range []struct {
		name string
		d    cat.Dialect
	}{{"FTdx10", ftdx10.Dialect()}, {"FT-891", ft891.Dialect()}} {
		if got, theirs := ft991a.Dialect().CATID(), other.d.CATID(); got == theirs {
			t.Errorf("CATID() = %q on the %s too — a shared identity would make radio detection pick whichever driver was registered first", got, other.name)
		}
	}
}

// TestDifferencePinModeMembership pins the nibble the FTdx10 fills and this
// radio leaves empty, the six names the two radios spell differently at a
// nibble they both fill, and 'E', where both name a real and DIFFERENT mode.
//
// 'F' is "DATA-FM-N" on the FTdx10 (ftdx10_layout.txt:1146-1149) and appears
// in no FT-991A legend at all. 'A' is the mirror of the FT-891's printed
// hole: that radio prints "A: -" and this one prints "A: DATA-FM", so the
// membership disagreement this axis carries runs in both directions across
// the family.
//
// THE PLAN AND SPEC SAY "FIVE DIFFERING NAMES"; THE MANUALS SAY SIX (beside
// 'E'). Re-derived nibble by nibble from the two extractions on 05/09/2026
// and reported as an erratum rather than transcribed: 3, 6, 7, 8, 9 and C
// differ, which is the same six the FT-891 differs at, because the FT-991A
// and the FT-891 spell those six alike.
func TestDifferencePinModeMembership(t *testing.T) {
	d := ft991a.Dialect()
	other := ftdx10.Dialect()

	const absent = 'F'
	if d.ValidMode(cat.Mode(absent)) {
		t.Errorf("ValidMode(%q) is TRUE on the FT-991A — none of its five legends prints an 'F', so this table has taken a sibling's member", absent)
	}
	if !other.ValidMode(cat.Mode(absent)) {
		t.Errorf("ValidMode(%q) is false on the FTdx10 — that is this pin's counter-example, and without it the assertion above proves only that the nibble is unknown to everybody", absent)
	}
	// 'A' is a member HERE and a printed hole on the FT-891 — the same axis
	// read the other way round.
	if !d.ValidMode(cat.Mode('A')) {
		t.Error(`ValidMode('A') is false on the FT-991A — its legends print "A: DATA-FM"`)
	}
	if ft891.Dialect().ValidMode(cat.Mode('A')) {
		t.Error(`ValidMode('A') is TRUE on the FT-891 — that radio's legends print "A: -", a printed hole, and without this half the membership above proves nothing about which radios fill the nibble`)
	}

	shared, differing, bothReal := 0, 0, 0
	for _, m := range modeNibbles {
		mode := cat.Mode(m.Wire)
		if !d.ValidMode(mode) {
			t.Errorf("ValidMode(%q) is false on the FT-991A — every nibble in this table is printed in all five of its legends", m.Wire)
			continue
		}
		if !other.ValidMode(mode) {
			t.Errorf("ValidMode(%q) is false on the FTdx10 — every nibble in this table is printed in all four of its legends too", m.Wire)
			continue
		}
		gotHere, gotThere := d.ModeName(mode), other.ModeName(mode)
		if gotHere != m.FT991A {
			t.Errorf("ModeName(%q) = %q on the FT-991A, want %q", m.Wire, gotHere, m.FT991A)
		}
		if gotThere != m.FTdx10 {
			t.Errorf("ModeName(%q) = %q on the FTdx10, want %q — this half of the pin is a fact about THAT manual", m.Wire, gotThere, m.FTdx10)
		}
		if m.SharedSpelling {
			if m.FT991A != m.FTdx10 {
				t.Errorf("nibble %q is tabled as shared but the two spellings differ (%q, %q)", m.Wire, m.FT991A, m.FTdx10)
			}
			shared++
			continue
		}
		differing++
		if m.BothRealDiffers {
			bothReal++
		}
		// THE DIFFERENCE, asserted rather than merely tabulated: if the two
		// spellings ever became equal, the loop above would still pass on a
		// table someone had "tidied".
		if gotHere == gotThere {
			t.Errorf("nibble %q spells %q on BOTH radios — the FT-991A prints %q and the FTdx10 %q, and a shared spelling here means one transcription has taken the other's", m.Wire, gotHere, m.FT991A, m.FTdx10)
		}
	}
	if shared != 7 || differing != 7 {
		t.Errorf("the fourteen nibbles both radios fill split %d shared / %d differing, want 7 / 7 — LSB, USB, FM, AM, DATA-FM, FM-N and AM-N are the seven the two manuals spell alike", shared, differing)
	}
	if bothReal != 1 {
		t.Errorf("%d nibbles are tabled as naming a real and DIFFERENT mode on each radio, want exactly 1 ('E': C4FM here, PSK there)", bothReal)
	}
}

// TestDifferencePinPMSFormAndSlotSpace pins the whole numeric slot space
// against the FTdx10's token one.
//
// The FT-991A's MC legend numbers its PMS pairs into the memory number line
// — "100: P-1L 101: P-1U ~ 116: P-9L 117: P-9U" (ft991a_layout.txt:916),
// under an outer span "001 - 117: Memory Channel Number" (913) — so pair 1's
// lower slot is the three decimal digits "100" and the token "P1L" is a wire
// form this radio has no legend for. Every registered sibling prints
// "P1L - P9U (PMS)" instead.
//
// THE REFUSAL SIDE IS THE POINT. cat.PMSSlotForm's own doc comment records
// what a default here would cost: Dialect.writableSlot returns true for
// every PMS slot, so a numeric-PMS radio silently given the token form would
// have "MW P1L…;" and "MT P1L…;" BUILT for it and admitted by its own
// outbound gate — frames its manual never prints.
func TestDifferencePinPMSFormAndSlotSpace(t *testing.T) {
	d := ft991a.Dialect()
	other := ftdx10.Dialect()

	if got := d.PMSForm(); got != cat.PMSFormNumeric {
		t.Fatalf("PMSForm() = %v, want cat.PMSFormNumeric — this manual's MC legend numbers the pairs 100..117", got)
	}
	if got := other.PMSForm(); got != cat.PMSFormToken {
		t.Fatalf("the FTdx10's PMSForm() = %v, want cat.PMSFormToken — this pin is a DIFFERENCE", got)
	}
	if got, want := d.PMSNumericLo(), 100; got != want {
		t.Errorf("PMSNumericLo() = %d, want %d — the legend's first PMS number", got, want)
	}
	if got, want := other.PMSNumericLo(), 0; got != want {
		t.Errorf("the FTdx10's PMSNumericLo() = %d, want %d under the token form", got, want)
	}

	// The nine pairs, rendered as the eighteen consecutive numbers the
	// legend prints, and classified as PMS by this dialect's own ParseSlot.
	for pair := 1; pair <= 9; pair++ {
		for i, upper := range []bool{false, true} {
			s, err := d.PMSSlot(pair, upper)
			if err != nil {
				t.Errorf("PMSSlot(%d, %v) = %v — the legend prints nine pairs", pair, upper, err)
				continue
			}
			want := fmt.Sprintf("%03d", 100+2*(pair-1)+i)
			if got := s.Wire(); got != want {
				t.Errorf("PMSSlot(%d, %v).Wire() = %q, want %q", pair, upper, got, want)
			}
			back, err := d.ParseSlot(want)
			if err != nil {
				t.Errorf("ParseSlot(%q) = %v — a wire form this dialect's own PMSSlot built", want, err)
				continue
			}
			if !back.IsPMS() {
				t.Errorf("ParseSlot(%q).IsPMS() is false — the MC legend gives 100..117 to the PMS pairs", want)
			}
		}
	}
	if _, err := d.PMSSlot(10, false); err == nil {
		t.Error("PMSSlot(10, false) was ACCEPTED — nine pairs is what the legend prints")
	}

	// THE TOKEN FORM IS REFUSED HERE AND BUILT THERE. Both halves, and both
	// directions of the refusal: the parser, and a frame assembled elsewhere
	// arriving at the gate.
	for _, wire := range []string{"P1L", "P9U"} {
		if got, err := d.ParseSlot(wire); err == nil {
			t.Errorf("ParseSlot(%q) was ACCEPTED, returning %q — no FT-991A legend prints a PMS token", wire, got.Wire())
		}
		for _, frame := range []string{"MC" + wire + ";", "MT" + wire + ";"} {
			if d.AllowedCommand([]byte(frame)) {
				t.Errorf("the gate ADMITTED %q — a token PMS form is a frame this manual never prints", frame)
			}
		}
	}
	if s, err := other.PMSSlot(1, false); err != nil {
		t.Errorf("the FTdx10's PMSSlot(1, false) = %v — this pin's counter-example", err)
	} else if got, want := s.Wire(), "P1L"; got != want {
		t.Errorf("the FTdx10's PMSSlot(1, false).Wire() = %q, want %q — without this half the refusals above prove only that nobody builds a token", got, want)
	}

	// …and the mirror: 100 and 117 are PMS here and NOT slots at all on the
	// FTdx10, whose memory range stops at 099 and whose 5xx bank starts at
	// 501.
	for _, wire := range []string{"100", "117"} {
		if got, err := other.ParseSlot(wire); err == nil {
			t.Errorf("the FTdx10's ParseSlot(%q) was ACCEPTED, returning kind for %q — that radio's number line has no 100..117", wire, got.Wire())
		}
	}
}

// TestDifferencePinAbsentBanks pins the two banks this radio does not have
// against the FTdx10, which has both.
//
// NEITHER ABSENCE IS AN ASSUMPTION. "5xx", "5 MHz" and "EMG" appear in NO
// slot legend of manual revision 1711-D — checked mechanically over the
// whole extraction — where the FTdx10's MR legend prints both. The absence
// is also what makes the degeneracy pin below true, so it is asserted here
// rather than left implicit in a policy comparison.
func TestDifferencePinAbsentBanks(t *testing.T) {
	d := ft991a.Dialect()
	other := ftdx10.Dialect()

	if _, err := d.SixtyMSlot(1); err == nil {
		t.Error("SixtyMSlot(1) was ACCEPTED — this manual prints no 5 MHz bank in any slot legend")
	}
	if got := d.EMGSlot(); got.Wire() != "" {
		t.Errorf("EMGSlot() = %q, want the zero Slot — this manual prints no emergency channel", got.Wire())
	}
	// The wire forms themselves, through the parser: 501 is an ordinary
	// out-of-range number here, and "EMG" is not a slot at all.
	for _, wire := range []string{"501", "599", "EMG"} {
		if got, err := d.ParseSlot(wire); err == nil {
			t.Errorf("ParseSlot(%q) was ACCEPTED, returning %q — neither bank is printed anywhere in this manual", wire, got.Wire())
		}
	}

	if _, err := other.SixtyMSlot(1); err != nil {
		t.Errorf("the FTdx10's SixtyMSlot(1) = %v — that is this pin's counter-example, and without it the refusals above prove only that nobody has these banks", err)
	}
	if got := other.EMGSlot().Wire(); got != "EMG" {
		t.Errorf("the FTdx10's EMGSlot().Wire() = %q, want %q", got, "EMG")
	}
}

// TestDifferencePinEXAddressForm pins the three-digit, single-component EX
// address against the FTdx10's six-digit triple.
//
// The FT-991A's EX grammar block prints "P1 : 001 - 153 (MENU Number)" and
// the Read chart "E X P1 P1 P1 ;" — SIX bytes (ft991a_layout.txt:519-528).
// The FTdx10's prints "E X P1 P1 P2 P2 P3 P3 ;" (ftdx10_layout.txt:636-645).
// The WIDTH is asserted alongside the FORM because the width is what sizes
// every EX frame this codec builds and every one its gate measures, and a
// form declared without its width reaching the frame would be a comment.
func TestDifferencePinEXAddressForm(t *testing.T) {
	d := ft991a.Dialect()
	other := ftdx10.Dialect()

	if got, want := d.EXAddressWidth(), 3; got != want {
		t.Errorf("EXAddressWidth() = %d, want %d", got, want)
	}
	if got, want := other.EXAddressWidth(), 6; got != want {
		t.Errorf("the FTdx10's EXAddressWidth() = %d, want %d — this pin is a DIFFERENCE and proves nothing if both radios declare the same width", got, want)
	}

	// 031 CAT RATE: a member of THIS chart (ft991a_layout.txt:561), and a row
	// the driver's own baud ASSUMED-register entry names.
	addr := cat.EXAddress{P1: 31, P2: 0, P3: 0}
	if !d.KnownEXAddress(addr) {
		t.Fatalf("KnownEXAddress(%v) is false — 031 CAT RATE is a row of this chart", addr)
	}
	if got, want := d.EXWire(addr), "031"; got != want {
		t.Errorf("EXWire(%v) = %q, want %q — the wire form IS the chart's printed MENU Number", addr, got, want)
	}

	cmd, err := d.BuildEXRead(addr)
	if err != nil {
		t.Fatalf("BuildEXRead(%v) = %v", addr, err)
	}
	if got, want := string(cmd.Bytes()), "EX031;"; got != want {
		t.Errorf("BuildEXRead(%v) built %q, want %q — six bytes, not nine", addr, got, want)
	}
	if !d.AllowedCommand(cmd.Bytes()) {
		t.Errorf("its own gate refused BuildEXRead(%v)'s frame %q", addr, cmd.Bytes())
	}

	// EVERY ITEM'S P2 AND P3 ARE ZERO, which V12 requires of an
	// EXAddressSingle inventory: the three-digit render drops both, and a
	// component silently dropped from every frame is what that rule exists to
	// make impossible.
	items := d.EXItems()
	if len(items) == 0 {
		t.Fatal("EXItems() is empty — every EX assertion in this file would be vacuous")
	}
	for _, it := range items {
		if it.Addr.P2 != 0 || it.Addr.P3 != 0 {
			t.Errorf("item %v has P2 %d P3 %d, want 0 and 0 — this chart's MENU Number is ONE component", it.Addr, it.Addr.P2, it.Addr.P3)
		}
	}

	// The counter-example: the FTdx10's own read of one of ITS members is
	// nine bytes, so the length above is this radio's and not the codec's.
	otherAddr := cat.EXAddress{P1: 1, P2: 6, P3: 1}
	if !other.KnownEXAddress(otherAddr) {
		t.Fatalf("KnownEXAddress(%v) is false on the FTdx10 — this pin needs a member address there", otherAddr)
	}
	otherCmd, err := other.BuildEXRead(otherAddr)
	if err != nil {
		t.Fatalf("the FTdx10's BuildEXRead(%v) = %v", otherAddr, err)
	}
	if got, want := len(otherCmd.Bytes()), 9; got != want {
		t.Errorf("the FTdx10's BuildEXRead built %d bytes, want %d", got, want)
	}
	if got, want := len(cmd.Bytes()), 6; got != want {
		t.Errorf("the FT-991A's BuildEXRead built %d bytes, want %d", got, want)
	}
}

// TestDifferencePinToneStates pins the P8 state domain.
//
// The FT-991A prints FIVE states — "0: CTCSS \"OFF\" 1: CTCSS ENC/DEC
// 2: CTCSS ENC 3: DCS ENC/DEC 4: DCS ENC" — on all five blocks that carry
// the field (ft991a_layout.txt:795-796, 977-978, 1010-1011, 1048-1049,
// 1128-1129). Every registered sibling prints "0/1/2" and nothing beyond '2'
// (ftdx10_layout.txt:1197, ft891_layout.txt:977).
//
// The GATE half is asserted alongside the builder because they are two
// separate consultations of one policy: a dialect whose gate admitted what
// its builder refuses would pass a frame assembled anywhere else.
func TestDifferencePinToneStates(t *testing.T) {
	d := ft991a.Dialect()

	if got := d.ToneStates(); got != cat.ToneStatesCTCSSAndDCS {
		t.Fatalf("ToneStates() = %v, want cat.ToneStatesCTCSSAndDCS — this manual's P8 legend prints five states", got)
	}
	for _, c := range []byte{'0', '1', '2', '3', '4'} {
		if _, err := d.ParseCTCSSState(c); err != nil {
			t.Errorf("ParseCTCSSState(%q) = %v — the P8 legend prints '0'..'4'", c, err)
		}
	}
	if _, err := d.ParseCTCSSState('5'); err == nil {
		t.Error("ParseCTCSSState('5') was ACCEPTED — the legend stops at '4'")
	}

	slot, err := d.MemorySlot(7)
	if err != nil {
		t.Fatalf("MemorySlot(7): %v", err)
	}
	for _, state := range []cat.CTCSSState{cat.CTCSSDCSEncDec, cat.CTCSSDCSEnc} {
		m := recordFor(d, slot, false)
		m.CTCSS = state
		cmd, err := d.BuildMWSet(m)
		if err != nil {
			t.Errorf("BuildMWSet with P8 %q = %v — this manual prints the state", state.Wire(), err)
			continue
		}
		if got := cmd.Bytes()[23]; got != state.Wire() {
			t.Errorf("BuildMWSet emitted %q, whose position 24 is %q, want %q", cmd.Bytes(), got, state.Wire())
		}
		if !d.AllowedCommand(cmd.Bytes()) {
			t.Errorf("its own gate refused BuildMWSet's frame %q", cmd.Bytes())
		}
	}

	// The counter-example, on the two siblings whose P8 legend stops at '2':
	// the SAME record is refused at their builders and at their gates.
	for _, other := range []struct {
		name string
		d    cat.Dialect
	}{{"FTdx10", ftdx10.Dialect()}, {"FT-891", ft891.Dialect()}} {
		if got := other.d.ToneStates(); got != cat.ToneStatesCTCSS {
			t.Errorf("the %s's ToneStates() = %v, want cat.ToneStatesCTCSS — this pin is a DIFFERENCE", other.name, got)
			continue
		}
		otherSlot, err := other.d.MemorySlot(7)
		if err != nil {
			t.Fatalf("the %s's MemorySlot(7): %v", other.name, err)
		}
		m := recordFor(other.d, otherSlot, false)
		m.CTCSS = cat.CTCSSDCSEncDec
		if got, err := other.d.BuildMWSet(m); err == nil {
			t.Errorf("the %s's BuildMWSet built %q with a DCS state — its P8 legend prints 0/1/2 only, and without this half the acceptances above prove only that everybody accepts DCS", other.name, got.Bytes())
		}
		if _, err := other.d.ParseCTCSSState('3'); err == nil {
			t.Errorf("the %s's ParseCTCSSState('3') was ACCEPTED", other.name)
		}
	}
}

// recordFor builds a memory record for one dialect's own write kind. Local
// to this file: the conformance suite has its own, and a helper reaching
// across package boundaries to be reused is how two walks end up sweeping
// the same space by accident.
func recordFor(d cat.Dialect, s cat.Slot, txClar bool) cat.MemoryData {
	return cat.MemoryData{
		Slot: s, FreqHz: 14_250_000, TxClar: txClar,
		Mode: cat.Mode('2'), Kind: d.MWWriteKind(),
		CTCSS: cat.CTCSSOff, Shift: cat.ShiftSimplex,
	}
}

// TestDifferencePinMemoryP5 pins byte 21 of the shared memory block AGAINST
// THE FT-891 — the sibling that refuses what this radio builds.
//
// The FT-991A prints `P5 0: TX CLAR "OFF" 1: TX CLAR "ON"` on every block
// that carries the 28-position grid (ft991a_layout.txt:1004 for MT, 971 for
// MR, 1042 for MW), where the FT-891 prints "P5 0: (Fixed)"
// (ft891_layout.txt:1006 and its four siblings). So on this radio byte 21
// carries a live TX-clarifier state a caller supplies, and on that one it is
// schema.
func TestDifferencePinMemoryP5(t *testing.T) {
	d := ft991a.Dialect()
	other := ft891.Dialect()

	if got := d.MemoryP5(); got != cat.P5TxClar {
		t.Fatalf("MemoryP5() = %v, want P5TxClar", got)
	}
	if got := other.MemoryP5(); got != cat.P5Fixed {
		t.Fatalf("the FT-891's MemoryP5() = %v, want P5Fixed — this pin is a DIFFERENCE", got)
	}

	slot, err := d.MemorySlot(7)
	if err != nil {
		t.Fatalf("MemorySlot(7): %v", err)
	}
	for _, txClar := range []bool{false, true} {
		cmd, err := d.BuildMWSet(recordFor(d, slot, txClar))
		if err != nil {
			t.Errorf("BuildMWSet with TxClar %v = %v — under P5TxClar both values are the legend's", txClar, err)
			continue
		}
		want := byte('0')
		if txClar {
			want = '1'
		}
		// Position 21, 1-indexed as the manual's table numbers it.
		if got := cmd.Bytes()[20]; got != want {
			t.Errorf("BuildMWSet(TxClar=%v) emitted %q, whose position 21 is %q, want %q", txClar, cmd.Bytes(), got, want)
		}
		if !d.AllowedCommand(cmd.Bytes()) {
			t.Errorf("its own gate refused %q", cmd.Bytes())
		}
	}

	otherSlot, err := other.MemorySlot(7)
	if err != nil {
		t.Fatalf("the FT-891's MemorySlot(7): %v", err)
	}
	if got, err := other.BuildMWSet(recordFor(other, otherSlot, true)); err == nil {
		t.Errorf("the FT-891's BuildMWSet with TxClar true built %q — that manual prints byte 21 \"(Fixed)\" on every memory block, and without this half the acceptance above proves only that everybody writes a TX clarifier", got.Bytes())
	}
}

// TestDifferencePinMTP11 pins byte 28 of the combined MT record AGAINST THE
// FT-891 — the other half of "each radio building what the other refuses".
//
// The FT-991A's MT block prints "P11 0: (Fixed)" (ft991a_layout.txt:1015):
// byte 28 is schema, so the display-BEARING pair has no flag to carry and
// refuses. The FT-891's prints `P11 0: TAG "OFF" 1: TAG "ON"`
// (ft891_layout.txt:1016): a live flag, which is never defaulted, so the
// display-LESS pair refuses there. The refusals run in opposite directions,
// which is what makes this a difference rather than a restriction.
func TestDifferencePinMTP11(t *testing.T) {
	d := ft991a.Dialect()
	other := ft891.Dialect()

	if got := d.MTP11(); got != cat.P11Fixed {
		t.Fatalf("MTP11() = %v, want P11Fixed", got)
	}
	if got := other.MTP11(); got != cat.P11TagDisplay {
		t.Fatalf("the FT-891's MTP11() = %v, want P11TagDisplay — this pin is a DIFFERENCE", got)
	}

	slot, err := d.MemorySlot(7)
	if err != nil {
		t.Fatalf("MemorySlot(7): %v", err)
	}
	m := recordFor(d, slot, false)
	m.Kind = cat.CombinedMTSetKind // the FORM's constant, not this dialect's MW kind

	cmd, err := d.BuildMTSetCombined(m, "CQ")
	if err != nil {
		t.Fatalf("BuildMTSetCombined = %v — under P11Fixed this is the radio's own builder", err)
	}
	// Position 28, 1-indexed as the manual's table numbers it.
	if got := cmd.Bytes()[27]; got != '0' {
		t.Errorf("BuildMTSetCombined emitted %q, whose position 28 is %q, want '0' — the legend prints it \"(Fixed)\"", cmd.Bytes(), got)
	}
	if !d.AllowedCommand(cmd.Bytes()) {
		t.Errorf("its own gate refused %q", cmd.Bytes())
	}
	if _, gotTag, err := d.ParseMTAnswerCombined(cmd.Bytes()); err != nil {
		t.Errorf("ParseMTAnswerCombined(%q) = %v", cmd.Bytes(), err)
	} else if gotTag != "CQ" {
		t.Errorf("ParseMTAnswerCombined(%q) returned the tag %q, want %q", cmd.Bytes(), gotTag, "CQ")
	}

	// The display-BEARING pair must refuse here: this radio has no TAG flag
	// for a caller to set.
	if got, err := d.BuildMTSetCombinedDisplay(m, "CQ", true); err == nil {
		t.Errorf("BuildMTSetCombinedDisplay succeeded, emitting %q — this manual prints byte 28 \"(Fixed)\"", got.Bytes())
	}
	if _, _, _, err := d.ParseMTAnswerCombinedDisplay(cmd.Bytes()); err == nil {
		t.Error("ParseMTAnswerCombinedDisplay accepted a frame whose byte 28 is schema — it would report a flag the radio never sent")
	}

	// The counter-example, refusing the other way round.
	otherSlot, err := other.MemorySlot(7)
	if err != nil {
		t.Fatalf("the FT-891's MemorySlot(7): %v", err)
	}
	om := recordFor(other, otherSlot, false)
	om.Kind = cat.CombinedMTSetKind
	if _, err := other.BuildMTSetCombinedDisplay(om, "CQ", true); err != nil {
		t.Errorf("the FT-891's BuildMTSetCombinedDisplay = %v — under P11TagDisplay that is ITS pair, and without this half the refusal above proves only that nobody sets a TAG flag", err)
	}
	if got, err := other.BuildMTSetCombined(om, "CQ"); err == nil {
		t.Errorf("the FT-891's BuildMTSetCombined succeeded, emitting %q — a live flag is never defaulted", got.Bytes())
	}
}

// TestIdentityPinFrameGeometry pins the three memory frame lengths this
// radio SHARES with the FTdx10: the combined MT record at 41, the MW Set at
// 28 and the MR Answer at 28.
//
// These are facts about two radios, not a shared definition. The FT-991A's
// MT chart runs to 41 (ft991a_layout.txt:998-1033, counted position by
// position by evidence leg G — testdata/mt-vectors.golden's own header
// records both counts), its MW Set to 28 (1036-1046) and its MR Answer to 28
// (965-981); the FTdx10's charts run to the same three numbers. 41 is
// written nowhere in core/cat and must not be — the geometry is derived,
// 29 + TagMaxBytes — so what this pin states is the arithmetic's ANSWER for
// these two radios.
//
// THE COMBINED ANSWER'S EXACTNESS IS ASSUMED, NOT CHART-PROVEN (doc.go's
// register, entry "THE COMBINED MT ANSWER'S EXACT LENGTH"): the grid draws
// the MAXIMAL frame, and the FT-710 precedent — hardware accepting short MT
// Sets against a maximal grid — makes a variable-width answer live.
func TestIdentityPinFrameGeometry(t *testing.T) {
	for _, r := range []struct {
		name string
		d    cat.Dialect
	}{{"FT-991A", ft991a.Dialect()}, {"FTdx10", ftdx10.Dialect()}} {
		min, max, err := r.d.MTAnswerBounds()
		if err != nil {
			t.Fatalf("%s: MTAnswerBounds() = %v", r.name, err)
		}
		if min != 41 || max != 41 {
			t.Errorf("%s: MTAnswerBounds() = (%d, %d), want (41, 41) — 29 shared positions plus a 12-byte tag field, and equal bounds are the combined form's signature", r.name, min, max)
		}

		slot, err := r.d.MemorySlot(7)
		if err != nil {
			t.Fatalf("%s: MemorySlot(7): %v", r.name, err)
		}
		cmd, err := r.d.BuildMWSet(recordFor(r.d, slot, false))
		if err != nil {
			t.Fatalf("%s: BuildMWSet = %v", r.name, err)
		}
		if got := len(cmd.Bytes()); got != 28 {
			t.Errorf("%s: BuildMWSet built %d bytes, want 28 — the MW Set chart runs to 28 positions on both radios", r.name, got)
		}
		// The MR Answer is the same 28-position chart under an "MR" prefix,
		// and ParseMRAnswer is this package's only route to the decoder since
		// MW has no Answer form on either radio.
		mr := append([]byte(nil), cmd.Bytes()...)
		mr[0], mr[1] = 'M', 'R'
		if _, err := r.d.ParseMRAnswer(mr); err != nil {
			t.Errorf("%s: ParseMRAnswer(%q) = %v — the MR Answer chart is the MW Set chart under another prefix", r.name, mr, err)
		}
		if _, err := r.d.ParseMRAnswer(append(mr[:27:27], '0', ';')); err == nil {
			t.Errorf("%s: ParseMRAnswer accepted a 29-byte frame — the chart runs to 28", r.name)
		}
	}
}

// TestIdentityPinPMSPairCount pins the NINE PMS pairs both radios have —
// deliberately apart from the form pin above, because nine pairs is what the
// two manuals agree about and the wire form is what they do not.
//
// The FT-991A's MC legend runs "100: P-1L ... 117: P-9U"
// (ft991a_layout.txt:916) and the FTdx10's slot legends "P1L - P9U (PMS)".
// Nine pairs, eighteen slots, on both.
func TestIdentityPinPMSPairCount(t *testing.T) {
	for _, r := range []struct {
		name string
		d    cat.Dialect
	}{{"FT-991A", ft991a.Dialect()}, {"FTdx10", ftdx10.Dialect()}} {
		slots := map[string]bool{}
		for pair := 1; pair <= 9; pair++ {
			for _, upper := range []bool{false, true} {
				s, err := r.d.PMSSlot(pair, upper)
				if err != nil {
					t.Errorf("%s: PMSSlot(%d, %v) = %v — both manuals print nine pairs", r.name, pair, upper, err)
					continue
				}
				if slots[s.Wire()] {
					t.Errorf("%s: PMSSlot(%d, %v) repeats the wire form %q", r.name, pair, upper, s.Wire())
				}
				slots[s.Wire()] = true
			}
		}
		if got, want := len(slots), 18; got != want {
			t.Errorf("%s: nine pairs produced %d distinct slots, want %d", r.name, got, want)
		}
		if _, err := r.d.PMSSlot(10, false); err == nil {
			t.Errorf("%s: PMSSlot(10, false) was ACCEPTED — nine pairs is what both legends print", r.name)
		}
	}
}

// TestIdentityPinTagWidth pins the twelve-character tag field both manuals
// print, through the behaviour it governs rather than through the field.
//
// The FT-991A's P12 legend reads "TAG Characters (up to 12 characters)
// (ASCII)" (ft991a_layout.txt:1017) and its Set chart draws the field over
// positions 29-40; the FTdx10's says the same (ftdx10_layout.txt:1236). The
// FILL BYTE a shorter tag is padded with is ASSUMED on both (doc.go's
// register, entry "MTPolicy.TagFill"), so this pin asserts the WIDTH and the
// boundary, never what the padding is.
func TestIdentityPinTagWidth(t *testing.T) {
	twelve := "GM5DNA......" // 12 bytes
	if len(twelve) != 12 {
		t.Fatalf("the test's tag is %d bytes, want 12", len(twelve))
	}
	for _, r := range []struct {
		name string
		d    cat.Dialect
	}{{"FT-991A", ft991a.Dialect()}, {"FTdx10", ftdx10.Dialect()}} {
		slot, err := r.d.MemorySlot(7)
		if err != nil {
			t.Fatalf("%s: MemorySlot(7): %v", r.name, err)
		}
		m := recordFor(r.d, slot, false)
		m.Kind = cat.CombinedMTSetKind
		if _, err := r.d.BuildMTSetCombined(m, twelve); err != nil {
			t.Errorf("%s: BuildMTSetCombined with a 12-byte tag = %v — both legends say \"up to 12 characters\"", r.name, err)
		}
		if got, err := r.d.BuildMTSetCombined(m, twelve+"X"); err == nil {
			t.Errorf("%s: BuildMTSetCombined accepted a 13-byte tag, emitting %q", r.name, got.Bytes())
		}
	}
}

// TestIdentityPinMWWriteKind pins the MW P7 byte.
//
// THE CAVEAT IS THE POINT, and it is the FT-891 test's caveat repeated
// because it applies to a third radio now. The FT-991A's MW legend reads
// "P7 00: (Fixed)" (ft991a_layout.txt:1047) and cat.CombinedMTSetKind is the
// byte '0', so the constant on the right is the correct SPELLING of what
// this radio documents. That the two coincide is A FACT OF THIS RADIO, not a
// rule: MW's P7 and the combined MT Set's P7 (also "Set: 0: (Fixed)", 1009)
// are different fields of different commands that this manual happens to fix
// at the same byte. core/cat keeps them apart on purpose —
// validateCombinedMTFields uses the FORM's constant and never this dialect's
// mwWriteKind — and nothing here may be read as permission to derive one
// from the other.
//
// THE PRINTED LEGEND IS TWO CHARACTERS WIDE AND THE FIELD IS ONE. That
// defect is doc.go's to record; what this pin states is the value, which the
// grid resolves to a single byte.
//
// The FT-710 is the counter-example, and it is asserted rather than
// described: its MW kind is cat.KindMemory ('1'), hardware-confirmed.
func TestIdentityPinMWWriteKind(t *testing.T) {
	if got := ft991a.Dialect().MWWriteKind(); got != cat.CombinedMTSetKind {
		t.Errorf("MWWriteKind() = %q, want %q (cat.CombinedMTSetKind) — this manual's MW legend reads \"P7 00: (Fixed)\"", got, cat.CombinedMTSetKind)
	}
	if got := ftdx10.Dialect().MWWriteKind(); got != cat.CombinedMTSetKind {
		t.Errorf("the FTdx10's MWWriteKind() = %q, want %q — the two manuals print the same MW P7 value, which is what makes this an IDENTITY pin", got, cat.CombinedMTSetKind)
	}
	if got := cat.FT710.MWWriteKind(); got == cat.CombinedMTSetKind {
		t.Errorf("the FT-710's MWWriteKind() is %q too — that radio documents '1' (Memory), hardware-confirmed, and without the counter-example this pin would read as a rule of the codec", got)
	}
}

// TestIdentityPinMCSelectsAndMTReadSlots pins the two WIDE policy values
// this dialect shares with the FTdx10.
//
// Both are TRANSCRIPTIONS, not configurations: MC's legend gives the whole
// "001 - 117" space (ft991a_layout.txt:913-916) and MT's read legend the
// same span (999), so the wide value is what those legends say. The FT-891
// is the counter-example on both axes — its MC and MT blocks print memory
// and PMS only — and it is asserted here so that "wide" is a fact about
// which radios print which legend rather than a default nobody chose.
//
// WHAT THE WIDE VALUE DOES NOT MEAN HERE is the degeneracy pin's subject:
// this radio has no 60m and no EMG bank, so the wide and narrow values
// behave identically. See TestDegeneracyPinWideAndNarrowAgree.
func TestIdentityPinMCSelectsAndMTReadSlots(t *testing.T) {
	d := ft991a.Dialect()

	if got := d.MCSelects(); got != cat.MCSelectsAll {
		t.Errorf("MCSelects() = %v, want cat.MCSelectsAll — MC's legend prints the whole 001 - 117 span", got)
	}
	if got := d.MTReadSlots(); got != cat.MTReadsReadable {
		t.Errorf("MTReadSlots() = %v, want cat.MTReadsReadable — MT's read legend prints the same span", got)
	}
	if got := ftdx10.Dialect().MCSelects(); got != cat.MCSelectsAll {
		t.Errorf("the FTdx10's MCSelects() = %v, want cat.MCSelectsAll — this is the IDENTITY half", got)
	}
	if got := ftdx10.Dialect().MTReadSlots(); got != cat.MTReadsReadable {
		t.Errorf("the FTdx10's MTReadSlots() = %v, want cat.MTReadsReadable — this is the IDENTITY half", got)
	}
	// The counter-example, so that "wide" names a legend and not a default.
	if got := ft891.Dialect().MCSelects(); got != cat.MCSelectsMemoryPMS {
		t.Errorf("the FT-891's MCSelects() = %v, want cat.MCSelectsMemoryPMS — its MC block prints memory and PMS only, and without this half the two values above would read as the only value there is", got)
	}
	if got := ft891.Dialect().MTReadSlots(); got != cat.MTReadsMemoryPMS {
		t.Errorf("the FT-891's MTReadSlots() = %v, want cat.MTReadsMemoryPMS — the counter-example half", got)
	}
}

// --- The degeneracy pin (spec D10) ---

// ft991aConfig returns the FT-991A's DialectConfig with the two policies the
// degeneracy pin varies supplied by the caller.
//
// IT IS A SECOND COPY OF THE DIALECT AND IS TREATED AS ONE. The package's
// literal is unexported, so a wide/narrow comparison built from the exported
// API alone is impossible; what makes this copy honest is
// assertTwinMatchesTheDialect below, which holds the WIDE twin to the real
// ft991a.Dialect() over the whole three-digit wire space and over every
// policy the comparison touches. A drift between this literal and dialect.go
// therefore fails LOUDLY here rather than quietly weakening the pin.
//
// EXItems comes from the real dialect rather than being re-declared, so the
// one part of the config that is 152 rows long is shared rather than copied.
func ft991aConfig(mc cat.MCSlotPolicy, mtRead cat.MTReadSlotPolicy) cat.DialectConfig {
	names := map[cat.Mode]string{cat.ModeUnset: "-"}
	for _, m := range modeNibbles {
		names[cat.Mode(m.Wire)] = m.FT991A
	}
	return cat.DialectConfig{
		CATID:     "0670",
		ModeNames: names,
		Slots: cat.SlotSpace{
			MemoryLo: 1, MemoryHi: 99,
			SixtyLo: 0, SixtyHi: 0,
			PMSPairs:      9,
			PMSForm:       cat.PMSFormNumeric,
			PMSNumericLo:  100,
			EmergencyWire: "",
			NoneWire:      "000",
			MCSelects:     mc,
		},
		EXItems:       ft991a.Dialect().EXItems(),
		EXAddressForm: cat.EXAddressSingle,
		MT: cat.MTPolicy{
			Form:         cat.MTFormCombined,
			ReadSlots:    mtRead,
			TagMaxBytes:  12,
			ClearTagByte: 0,
			PadByte:      0,
			TagFill:      ' ',
			P11:          cat.P11Fixed,
		},
		Clarifier:   cat.ClarifierPolicy{StepHz: 10, MaxAbsHz: 9990},
		MemoryP5:    cat.P5TxClar,
		ToneStates:  cat.ToneStatesCTCSSAndDCS,
		MWWriteKind: cat.CombinedMTSetKind,
	}
}

// threeDigitForms is every three-digit decimal wire form, "000" to "999" —
// all 1,000 of them, which is the whole space a three-byte numeric slot
// field can carry.
func threeDigitForms() []string {
	out := make([]string, 0, 1000)
	for n := 0; n < 1000; n++ {
		out = append(out, fmt.Sprintf("%03d", n))
	}
	return out
}

// tokenPMSForms is every "P<n><L|U>" form, the shape every registered
// sibling's PMS slots take and this radio's take none of.
func tokenPMSForms() []string {
	var out []string
	for n := '0'; n <= '9'; n++ {
		for _, suffix := range []byte{'L', 'U'} {
			out = append(out, fmt.Sprintf("P%c%c", n, suffix))
		}
	}
	return out
}

// assertTwinMatchesTheDialect holds a twin built by ft991aConfig to the real
// ft991a.Dialect(), so that the degeneracy comparison is a statement about
// THIS radio and not about a literal that has drifted.
func assertTwinMatchesTheDialect(t *testing.T, name string, twin cat.Dialect) {
	t.Helper()
	d := ft991a.Dialect()

	if got, want := twin.CATID(), d.CATID(); got != want {
		t.Fatalf("%s twin's CATID() = %q, the real dialect's is %q", name, got, want)
	}
	if got, want := twin.PMSForm(), d.PMSForm(); got != want {
		t.Fatalf("%s twin's PMSForm() = %v, the real dialect's is %v", name, got, want)
	}
	if got, want := twin.PMSNumericLo(), d.PMSNumericLo(); got != want {
		t.Fatalf("%s twin's PMSNumericLo() = %d, the real dialect's is %d", name, got, want)
	}
	if got, want := twin.EXAddressWidth(), d.EXAddressWidth(); got != want {
		t.Fatalf("%s twin's EXAddressWidth() = %d, the real dialect's is %d", name, got, want)
	}
	if got, want := twin.MTP11(), d.MTP11(); got != want {
		t.Fatalf("%s twin's MTP11() = %v, the real dialect's is %v", name, got, want)
	}
	if got, want := twin.MemoryP5(), d.MemoryP5(); got != want {
		t.Fatalf("%s twin's MemoryP5() = %v, the real dialect's is %v", name, got, want)
	}
	if got, want := twin.ToneStates(), d.ToneStates(); got != want {
		t.Fatalf("%s twin's ToneStates() = %v, the real dialect's is %v", name, got, want)
	}
	if got, want := twin.EMGSlot().Wire(), d.EMGSlot().Wire(); got != want {
		t.Fatalf("%s twin's EMGSlot() = %q, the real dialect's is %q", name, got, want)
	}
	// THE SLOT SPACE ITSELF, over the whole three-digit field and every
	// token form: same wire form, same classification, on every one of the
	// 1,020 inputs. This is what makes the twin the same radio.
	for _, wire := range append(threeDigitForms(), tokenPMSForms()...) {
		gotSlot, gotErr := twin.ParseSlot(wire)
		wantSlot, wantErr := d.ParseSlot(wire)
		if (gotErr == nil) != (wantErr == nil) {
			t.Fatalf("%s twin's ParseSlot(%q) = (%q, %v), the real dialect's = (%q, %v) — the twin is not this radio", name, wire, gotSlot.Wire(), gotErr, wantSlot.Wire(), wantErr)
		}
		if gotErr != nil {
			continue
		}
		if gotSlot.IsMemory() != wantSlot.IsMemory() || gotSlot.IsPMS() != wantSlot.IsPMS() || gotSlot.IsNone() != wantSlot.IsNone() {
			t.Fatalf("%s twin classifies %q differently from the real dialect", name, wire)
		}
	}
	// The mode table, over the whole byte space.
	for c := 0; c < 256; c++ {
		m := cat.Mode(byte(c))
		if twin.ValidMode(m) != d.ValidMode(m) {
			t.Fatalf("%s twin's ValidMode(%#02x) disagrees with the real dialect's", name, c)
		}
		if twin.ValidMode(m) && twin.ModeName(m) != d.ModeName(m) {
			t.Fatalf("%s twin's ModeName(%#02x) = %q, the real dialect's is %q", name, c, twin.ModeName(m), d.ModeName(m))
		}
	}
}

// TestDegeneracyPinWideAndNarrowAgree is spec D10, made mechanical.
//
// cat.MCSelectsAll and cat.MCSelectsMemoryPMS differ ONLY over the 60m and
// EMG banks, and cat.MTReadsReadable and cat.MTReadsMemoryPMS likewise. This
// radio declares SixtyLo/SixtyHi = (0, 0) and EmergencyWire = "" — neither
// bank is printed in any legend of manual revision 1711-D — so
// Dialect.classifySlot can never return either kind, and the two readings
// give IDENTICAL VERDICTS on every wire form there is.
//
// THE WIDE VALUE IS DECLARED ANYWAY, AND THE REASON MATTERS. MC's legend
// gives the whole "001 - 117: Memory Channel Number" space
// (ft991a_layout.txt:913-916), which the hand-derived evidence leg G
// independently recorded and reduced to four frames — testdata/
// mc-vectors.golden's MC012;, MC099;, MC100; and MC117;, spanning the
// regular range's interior and top and the PMS range's bottom and top. The
// field is a CITATION OF THAT LEGEND, not a statement about this dialect's
// configuration, and the citation has to be made from the legend even where
// the choice is currently inert.
//
// WHAT THIS TEST BUYS: the moment a bank IS added — a region variant, a
// firmware revision, a corrected reading — the coincidence stops holding and
// this test fails LOUDLY, at which point the declared value stops being
// inert and starts being a decision somebody has to make on evidence.
func TestDegeneracyPinWideAndNarrowAgree(t *testing.T) {
	d := ft991a.Dialect()

	// The precondition, asserted rather than assumed: the degeneracy exists
	// BECAUSE the two banks are absent, and if either ever appears the
	// reasoning below is void even before the verdicts diverge.
	if _, err := d.SixtyMSlot(1); err == nil {
		t.Fatal("SixtyMSlot(1) succeeded — this radio has acquired a 60m bank, and the wide/narrow degeneracy this test pins no longer holds by construction")
	}
	if got := d.EMGSlot().Wire(); got != "" {
		t.Fatalf("EMGSlot() = %q — this radio has acquired an emergency channel, and the wide/narrow degeneracy this test pins no longer holds by construction", got)
	}

	wide := cat.MustNewDialect(ft991aConfig(cat.MCSelectsAll, cat.MTReadsReadable))
	narrow := cat.MustNewDialect(ft991aConfig(cat.MCSelectsMemoryPMS, cat.MTReadsMemoryPMS))
	assertTwinMatchesTheDialect(t, "wide", wide)

	// The wide twin must also be the real dialect on the two policies, so
	// that "wide" below is the value this package actually declares.
	if got, want := wide.MCSelects(), d.MCSelects(); got != want {
		t.Fatalf("the wide twin declares MCSelects %v, the real dialect declares %v", got, want)
	}
	if got, want := wide.MTReadSlots(), d.MTReadSlots(); got != want {
		t.Fatalf("the wide twin declares MTReadSlots %v, the real dialect declares %v", got, want)
	}
	if narrow.MCSelects() == wide.MCSelects() || narrow.MTReadSlots() == wide.MTReadSlots() {
		t.Fatal("the two twins declare the same policies — this test would compare a dialect with itself")
	}

	forms := append(threeDigitForms(), tokenPMSForms()...)
	if got, want := len(forms), 1020; got != want {
		t.Fatalf("the sweep covers %d wire forms, want %d — 1,000 three-digit forms and 20 P<n><L|U> forms", got, want)
	}

	built, refused := 0, 0
	for _, wire := range forms {
		// The PARSER first: both must agree on whether the form is a slot at
		// all, since every builder below consults classifySlot.
		wideSlot, wideParse := wide.ParseSlot(wire)
		narrowSlot, narrowParse := narrow.ParseSlot(wire)
		if (wideParse == nil) != (narrowParse == nil) {
			t.Errorf("ParseSlot(%q): wide = %v, narrow = %v — the two policies govern the SEND and READ domains, never what a wire form IS", wire, wideParse, narrowParse)
			continue
		}

		// The GATE, on the frames anything else would assemble. This half
		// runs on EVERY form, parseable or not, because the gate never sees
		// a cat.Slot.
		for _, frame := range []string{"MC" + wire + ";", "MT" + wire + ";"} {
			w, n := wide.AllowedCommand([]byte(frame)), narrow.AllowedCommand([]byte(frame))
			if w != n {
				t.Errorf("the gate disagrees on %q: wide = %v, narrow = %v — with no 60m and no EMG bank the two policies have nothing to differ about, so a disagreement means a bank has appeared and the declared value is no longer inert", frame, w, n)
			}
			if w {
				built++
			} else {
				refused++
			}
		}
		if wideParse != nil {
			continue
		}

		// The BUILDERS, which are the other consultation of the same two
		// policies.
		wideMC, wideMCErr := wide.BuildMCSet(wideSlot)
		narrowMC, narrowMCErr := narrow.BuildMCSet(narrowSlot)
		if (wideMCErr == nil) != (narrowMCErr == nil) {
			t.Errorf("BuildMCSet(%q): wide = %v, narrow = %v", wire, wideMCErr, narrowMCErr)
		} else if wideMCErr == nil && !bytes.Equal(wideMC.Bytes(), narrowMC.Bytes()) {
			t.Errorf("BuildMCSet(%q) built %q wide and %q narrow", wire, wideMC.Bytes(), narrowMC.Bytes())
		}

		wideMT, wideMTErr := wide.BuildMTRead(wideSlot)
		narrowMT, narrowMTErr := narrow.BuildMTRead(narrowSlot)
		if (wideMTErr == nil) != (narrowMTErr == nil) {
			t.Errorf("BuildMTRead(%q): wide = %v, narrow = %v", wire, wideMTErr, narrowMTErr)
		} else if wideMTErr == nil && !bytes.Equal(wideMT.Bytes(), narrowMT.Bytes()) {
			t.Errorf("BuildMTRead(%q) built %q wide and %q narrow", wire, wideMT.Bytes(), narrowMT.Bytes())
		}
	}

	// NON-VACUITY. A sweep in which everything was refused, or everything
	// admitted, would agree perfectly and prove nothing.
	if built == 0 || refused == 0 {
		t.Errorf("the gate sweep admitted %d frames and refused %d — a sweep with nothing on one side of the line is a comparison of two constants", built, refused)
	}

	// The four frames evidence leg G hand-derived from the MC legend, which
	// are the reason the WIDE value is the one declared. They are read from
	// the golden rather than restated, so that the citation and the artefact
	// cannot drift apart.
	for _, frame := range mcGoldenFrames(t) {
		if !d.AllowedCommand([]byte(frame)) {
			t.Errorf("the real dialect's gate refused %q, a frame evidence leg G derived from the MC legend (testdata/mc-vectors.golden)", frame)
		}
	}
}

// mcGoldenFrames reads the four Set frames out of evidence leg G's MC
// golden. It parses the artefact rather than restating its content: the
// degeneracy pin's justification IS that file, and a literal copy here would
// be a bound consulted from somewhere other than its datum.
//
// The golden's own hash freeze belongs to the frame-geometry task's
// golden_test.go, as core/cat/ft891/golden_test.go carries that model's;
// this reader only needs the frames.
func mcGoldenFrames(t *testing.T) []string {
	t.Helper()
	const path = "testdata/mc-vectors.golden"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		_, frame, ok := strings.Cut(line, "\t")
		if !ok {
			t.Fatalf("%s: line %q is not <name>\\t<frame>", path, line)
		}
		out = append(out, frame)
	}
	if got, want := len(out), 4; got != want {
		t.Fatalf("%s holds %d vectors, want %d — the MC legend's four hand-derived frames", path, got, want)
	}
	return out
}

// TestEXItemsCountMatchesProfile holds the dialect's inventory against the
// REGISTERED profile's ExpectedRows, MINUS the declared parameterless
// address.
//
// ExpectedRows comes from the page ledger, derived from the rendered PDF by
// a quarantined agent before either transcription existed
// (testdata/ledger.csv); the inventory comes from table2.csv through the
// generator; the exclusion comes from the profile's own
// ParameterlessAddresses. So this is a THIRD consultation of the row count,
// taken from the registry rather than from a literal here — the "bound
// consulted from one place with its datum taken from another" rule, which a
// hardcoded 152 in this file would break.
//
// THE ARITHMETIC IS THE FT-891'S PLUS ONE TERM. That model's inventory is
// its whole chart; this one's is the chart less the ONE row that names no
// field, and the difference is exactly that address.
func TestEXItemsCountMatchesProfile(t *testing.T) {
	var matches []extable.NamedProfile
	for _, np := range extable.RegisteredProfiles() {
		if np.Profile.Package == "ft991a" {
			matches = append(matches, np)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("registry holds %d profiles emitting into package ft991a, want exactly 1", len(matches))
	}
	p := matches[0].Profile

	items := ft991a.Dialect().EXItems()
	want := p.ExpectedRows - len(p.ParameterlessAddresses)
	if len(items) != want {
		t.Errorf("Dialect().EXItems() holds %d items; the registered ft991a profile counts %d chart rows less %d declared parameterless address(es) = %d — the ledger, the transcription and the dialect must agree, and the arbitration is against the PDF", len(items), p.ExpectedRows, len(p.ParameterlessAddresses), want)
	}
	if len(items) == 0 {
		t.Fatal("Dialect().EXItems() is empty — every EX assertion in this file would be vacuous")
	}
	// The excluded address is excluded, and every other chart row is not.
	for _, addr := range p.ParameterlessAddresses {
		a := cat.EXAddress{P1: uint16(addr[0]), P2: uint16(addr[1]), P3: uint16(addr[2])}
		if ft991a.Dialect().KnownEXAddress(a) {
			t.Errorf("KnownEXAddress(%v) is TRUE — the profile declares that address parameterless, so the chart prints it and the inventory must not carry it", a)
		}
	}
}

// TestEXAnswerBound proves the EX answer's upper length bound is THIS
// DIALECT'S OWN, derived from its own inventory.
//
// The maximum is recomputed here from Dialect().EXItems() rather than taken
// from a constant, from the profile or from core/cat: the bound and the
// datum it is derived from must not come from the same place twice. It
// reaches 8 through exactly ONE item — 151 PRESET FREQUENCY
// (ft991a_layout.txt:692), whose parameter is the eight-digit range
// "00030000 ~ 47000000" — which is why the ft991a profile declares
// MaxDigits 8 where the other three declare 4 or 5.
//
// The behavioural half is what matters: the derived width must be what the
// parser actually enforces, one byte either side.
func TestEXAnswerBound(t *testing.T) {
	d := ft991a.Dialect()

	items := d.EXItems()
	if len(items) == 0 {
		t.Fatal("EXItems() is empty — every assertion below would be vacuous")
	}
	maxDigits, wide := 0, 0
	for _, it := range items {
		if it.Digits > maxDigits {
			maxDigits = it.Digits
		}
	}
	var widest cat.EXAddress
	for _, it := range items {
		if it.Digits == maxDigits {
			wide++
			widest = it.Addr
		}
	}
	if maxDigits != 8 {
		t.Fatalf("max(Digits) over the FT-991A's %d inventory items is %d, want 8", len(items), maxDigits)
	}
	if wide != 1 {
		t.Errorf("%d items carry Digits 8, want 1 (151 PRESET FREQUENCY)", wide)
	}
	if got, want := d.EXWire(widest), "151"; got != want {
		t.Errorf("the widest item is %s, want %s", got, want)
	}

	// The body is returned VERBATIM — no per-item width policy is applied at
	// parse, by core/cat's documented decision — so an eight-byte body is
	// admissible at any member address, not only at the widest row's.
	body := "00030000" // 8 bytes, and 151's own printed lower bound
	if len(body) != maxDigits {
		t.Fatalf("the test's parameter body is %d bytes, want %d", len(body), maxDigits)
	}
	frame := []byte("EX" + d.EXWire(widest) + body + ";")
	if got, want := string(frame), "EX15100030000;"; got != want {
		t.Fatalf("the test built %q, want %q — a fourteen-byte answer: \"EX\", three address digits, eight body bytes and the terminator", got, want)
	}
	gotAddr, gotBody, err := d.ParseEXAnswer(frame)
	if err != nil {
		t.Errorf("ParseEXAnswer(%q) = %v — a %d-byte parameter is this dialect's own widest item, so its parser must read one", frame, err, maxDigits)
	} else {
		if gotAddr != widest {
			t.Errorf("ParseEXAnswer(%q) returned address %v, want %v", frame, gotAddr, widest)
		}
		if gotBody != body {
			t.Errorf("ParseEXAnswer(%q) returned the parameter %q, want %q verbatim", frame, gotBody, body)
		}
	}

	over := []byte("EX" + d.EXWire(widest) + body + "0" + ";")
	if _, _, err := d.ParseEXAnswer(over); err == nil {
		t.Errorf("ParseEXAnswer(%q) ACCEPTED a %d-byte parameter, one past this dialect's widest inventory item — the bound is not deriving from this inventory", over, maxDigits+1)
	}
}

// registerEntryNames quotes doc.go's ASSUMED register VERBATIM: the eleven
// entry NAMES, in doc.go's order, each being the opening of its bullet up to
// the parenthetical citation or the full stop where the explanation starts.
// TestASSUMEDRegisterIsComplete asserts every bullet opens with its name, so
// a name that drifts in either file fails here rather than silently.
//
// WHY THE ELEVEN STRINGS ARE WRITTEN OUT RATHER THAN DERIVED. doc.go's
// register insists entries be cited BY NAME, and Stage 2's core/driver/ft991a
// carries the SAME eleven in its own doc comment (spec §The ASSUMED
// register). Nothing mechanical can hold that copy to this one until it
// exists, so this slice is the form the driver package mirrors: one literal
// list a test over there can copy verbatim and assert against its own doc
// comment, in this order.
//
// FOUR OF THE ELEVEN NAMES EMBED A VALUE — TagFill's ' ', the combined
// answer's 41, NoneWire's "000", the clarifier's 10 and 9990. A partial lift
// that changes one of those values therefore invalidates every by-name
// citation of it, which is a reason to keep the names in ONE place and let a
// test find the citations rather than a reader.
var registerEntryNames = []string{
	`MTPolicy.TagFill = ' '`,
	"THE COMBINED MT ANSWER'S EXACT LENGTH, 41",
	`SlotSpace.NoneWire = "000"`,
	"THE cat.ModeUnset MEMBER OF THE MODE TABLE",
	"ClarifierPolicy.StepHz = 10 AND ClarifierPolicy.MaxAbsHz = 9990",
	`THE CLARIFIER'S MINUS-DIRECTION BYTE, the ASCII HYPHEN-MINUS 0x2D ('-')`,
	"THE DCS STATES' SET ACCEPTANCE",
	"ROW 087 RADIO ID'S EXCLUSION",
	"FRAMING: 8 DATA BITS, NO PARITY, TWO STOP BITS",
	"DefaultBaud 38400",
	"THE ACKNOWLEDGEMENT CONVENTIONS",
}

// assumedRegister says, for each entry of registerEntryNames IN THE SAME
// ORDER, where the assumption is USED and how dialect.go cites it.
//
// It is not a second statement of the assumptions — the register is the
// statement — it is the machinery that keeps the register and dialect.go
// from drifting apart. FOUR of the eleven are fields of this package's own
// DialectConfig literal and must carry an ASSUMED marker at that field; the
// other seven have no field here to mark, which the Elsewhere column records
// rather than leaves to inference. Three of those seven belong to Stage 2's
// driver and are carried here because the register is ONE statement of
// record for the radio, not one per package (spec §The ASSUMED register).
var assumedRegister = []struct {
	// Cite is the text dialect.go's comment quotes when it names this
	// entry — the entry's name, shortened where a full name would put a
	// nested quotation inside a Go comment. It must be a PREFIX of the
	// entry's name in registerEntryNames, which the test asserts, so a
	// shortened citation can never come to mean a different entry.
	Cite string
	// Anchor is the line in dialect.go whose declaration carries the
	// assumption; "" means the point of use is outside this package.
	Anchor string
	// Elsewhere says where an entry with no Anchor is actually used.
	Elsewhere string
}{
	{Cite: "MTPolicy.TagFill", Anchor: "TagFill:"},
	{Cite: "THE COMBINED MT ANSWER'S EXACT LENGTH", Elsewhere: "core/cat/mtcombined.go's fixed 29 + TagMaxBytes geometry"},
	{Cite: "SlotSpace.NoneWire", Anchor: "NoneWire:"},
	{Cite: "THE cat.ModeUnset MEMBER OF THE MODE TABLE", Anchor: "cat.ModeUnset:"},
	{Cite: "ClarifierPolicy.StepHz = 10 AND ClarifierPolicy.MaxAbsHz", Anchor: "StepHz:"},
	{Cite: "THE CLARIFIER'S MINUS-DIRECTION BYTE", Elsewhere: "core/cat/memdata.go's sign encoding and parsing"},
	{Cite: "THE DCS STATES' SET ACCEPTANCE", Elsewhere: "the radio's behaviour, not a field: ToneStates is TRANSCRIBED from the P8 legend"},
	{Cite: "ROW 087 RADIO ID'S EXCLUSION", Elsewhere: "internal/extable's ft991a profile stanza (ParameterlessAddresses) and table2.csv"},
	{Cite: "FRAMING: 8 DATA BITS, NO PARITY, TWO STOP BITS", Elsewhere: "core/transport's DefaultStopBits, reached by ABSENCE — Stage 2's core/driver/ft991a declares no SerialFramingReporter"},
	{Cite: "DefaultBaud 38400", Elsewhere: "Stage 2's core/driver/ft991a capability table"},
	{Cite: "THE ACKNOWLEDGEMENT CONVENTIONS", Elsewhere: "Stage 2's core/driver/ft991a write path"},
}

// isDialectComment says whether a line of dialect.go is a whole-line comment.
// A trailing comment on a declaration is not one: the declaration is what
// ends a field's comment block.
func isDialectComment(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "//")
}

// isASSUMEDMarker says whether a comment line FLAGS ITS OWN FIELD as assumed.
//
// It is deliberately not "any line that uses the word". dialect.go also says
// "NOT ASSUMED" (ruling an assumption OUT) and carries narrative prose about
// assumptions in general; neither names a field of this dialect as assumed,
// so neither should have to anchor to one. Every field genuinely flagged in
// that file does it one of two ways: the register's own opening word
// ("// ASSUMED — ...") or a declarative "...ARE ASSUMED" / "...IS ASSUMED"
// sentence. The comparisons are case-sensitive on purpose — lower-case
// "assumed" in running prose is discussion, not a flag.
func isASSUMEDMarker(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "//") || strings.Contains(line, "NOT ASSUMED") {
		return false
	}
	return strings.HasPrefix(trimmed, "// ASSUMED") ||
		strings.Contains(line, "ARE ASSUMED") ||
		strings.Contains(line, "IS ASSUMED")
}

// unwrapComment joins a run of comment lines into one line of prose, so that
// a quoted register name the comment happened to wrap across two lines is
// still found by a plain substring search. The clarifier's marker wraps
// exactly there.
func unwrapComment(block []string) string {
	parts := make([]string, 0, len(block))
	for _, line := range block {
		parts = append(parts, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "//")))
	}
	return strings.Join(parts, " ")
}

// TestASSUMEDRegisterIsComplete holds doc.go's ASSUMED register and
// dialect.go's ASSUMED markers to each other, and both to the two tables
// above — registerEntryNames, the eleven names verbatim, and assumedRegister,
// which says where each is used.
//
// THE FAILURE IT EXISTS TO CATCH is an assumption that travels
// unregistered: a field marked ASSUMED in dialect.go with no register entry
// is invisible to Stage R, and a register entry whose field lost its marker
// is a value a later reader will take for a transcription. The M9d-1
// adjudication found exactly that shape — the clarifier's minus byte assumed
// in two dialects and registered in neither — which is why this package gets
// the check mechanically rather than by review.
//
// It reads the two SOURCE FILES. They are committed Go in this package's own
// directory, so unlike the manual extraction they are present in a fresh
// clone and in CI.
func TestASSUMEDRegisterIsComplete(t *testing.T) {
	docSrc, err := os.ReadFile("doc.go")
	if err != nil {
		t.Fatalf("reading doc.go: %v", err)
	}
	dialectSrc, err := os.ReadFile("dialect.go")
	if err != nil {
		t.Fatalf("reading dialect.go: %v", err)
	}

	// The register section, bounded by its own heading and the next one.
	const startHeading = "// # The ASSUMED register"
	start := bytes.Index(docSrc, []byte(startHeading))
	if start < 0 {
		t.Fatalf("doc.go has no %q heading — the register is this radio's statement of record", startHeading)
	}
	rest := docSrc[start+len(startHeading):]
	end := bytes.Index(rest, []byte("\n// # "))
	if end < 0 {
		t.Fatal("doc.go's ASSUMED register is not followed by another heading — this test cannot tell where the section ends")
	}
	section := string(rest[:end])

	// Every bullet in the section, in order, UNWRAPPED. A bullet opens
	// "//   - " and its continuation lines are indented five spaces, so a
	// name quoted in registerEntryNames need not stop at whatever column the
	// comment happened to wrap on — the clarifier's minus-byte name spans a
	// wrap.
	var bullets []string
	for _, line := range strings.Split(section, "\n") {
		rest, ok := strings.CutPrefix(line, "//")
		if !ok {
			continue
		}
		switch {
		case strings.HasPrefix(rest, "   - "):
			bullets = append(bullets, strings.TrimSpace(rest[len("   - "):]))
		case strings.HasPrefix(rest, "     ") && len(bullets) > 0:
			bullets[len(bullets)-1] += " " + strings.TrimSpace(rest)
		}
	}
	if len(registerEntryNames) != len(assumedRegister) {
		t.Fatalf("registerEntryNames holds %d names and assumedRegister %d rows — the two are index-aligned, one row per name", len(registerEntryNames), len(assumedRegister))
	}
	if len(bullets) != len(assumedRegister) {
		t.Fatalf("doc.go's ASSUMED register holds %d entries, this file's table holds %d — an entry added to one and not the other is exactly the drift this test exists to stop; the bullets are %q", len(bullets), len(assumedRegister), bullets)
	}
	if !strings.Contains(section, "ELEVEN members") || len(assumedRegister) != 11 {
		t.Errorf("the register's prose says a count that no longer matches its %d entries — the sentence opening the section names the number out loud", len(assumedRegister))
	}
	for i, name := range registerEntryNames {
		if !strings.HasPrefix(bullets[i], name) {
			t.Errorf("register entry %d opens %q, registerEntryNames spells it %q — the eleven names are quoted VERBATIM so that Stage 2's core/driver/ft991a can mirror them, and every citation of an entry is by name", i+1, bullets[i], name)
		}
		if cite := assumedRegister[i].Cite; cite == "" || !strings.HasPrefix(name, cite) {
			t.Errorf("register entry %d is named %q but this file cites it as %q — a citation must be a prefix of the name, or it names something else", i+1, name, cite)
		}
	}

	// EVERY ENTRY NAMES ITS ONE LIFTING CAPTURE. A register whose entries do
	// not say what would settle them is a list of caveats rather than a plan,
	// and the spec's own table is built the other way round.
	if got, want := strings.Count(section, "STAGE R LIFTS IT WITH:"), len(assumedRegister); got != want {
		t.Errorf("the register carries %d \"STAGE R LIFTS IT WITH:\" clauses for %d entries — each entry states the ONE capture that lifts it", got, want)
	}

	// The NON-entries, which this register says out loud because a reader
	// coming from a sibling package would expect at least the first of them.
	for _, want := range []string{"DELIBERATELY NOT AN ENTRY", "MCSelects", "100", "117"} {
		if !strings.Contains(section, want) {
			t.Errorf("the register no longer records %q — the wide MC and MT read domains are a CITATION of this radio's own legend, and the PMS numbering is printed where its siblings' 5 MHz numbering is assumed; dropping either statement would leave a reader to assume the sibling reading", want)
		}
	}

	// THE MARKERS IN dialect.go, MATCHED TO THE REGISTER BOTH WAYS AND BY
	// NAME.
	//
	// The first form of this walk searched for the substring "ASSUMED"
	// anywhere in a fixed fourteen-line window above each anchored field and
	// marked the whole window covered. The Stage 1 task 7 review defeated it
	// in both directions with reverted probes. Deleting the cat.ModeUnset
	// marker outright still PASSED, because the window then reached narrative
	// prose in modeNames' own doc comment that uses the word. And a bogus
	// "// ASSUMED — the twelve-byte tag width is a guess nobody registered."
	// planted above TagMaxBytes — a different, unregistered field — was
	// SWALLOWED, because it fell inside the neighbouring anchor's window. A
	// fixed window over a substring cannot do this job.
	//
	// This walk uses the FIELD'S OWN CONTIGUOUS COMMENT BLOCK — the run of
	// whole-line comments immediately above the declaration, ending at the
	// first line that is not one, so it can never reach the field above — and
	// matches marker to entry by name:
	//
	//   - the block must hold EXACTLY ONE marker line;
	//   - the block must quote the citation of EXACTLY ONE register entry,
	//     and it must be this entry's;
	//   - every marker line in the whole file must be one an entry claimed;
	//   - and the file's marker count must equal the number of anchored
	//     entries, which is what catches a second marker planted INSIDE a
	//     registered field's own block, where the per-block checks alone
	//     would take it for that field's.
	//
	// Both of the review's probes fail this walk.
	lines := strings.Split(string(dialectSrc), "\n")

	var markers []int
	for i, line := range lines {
		if isASSUMEDMarker(line) {
			markers = append(markers, i)
		}
	}
	anchored := 0
	for _, row := range assumedRegister {
		if row.Anchor != "" {
			anchored++
		}
	}
	if len(markers) != anchored {
		found := make([]string, 0, len(markers))
		for _, i := range markers {
			found = append(found, fmt.Sprintf("dialect.go:%d %s", i+1, strings.TrimSpace(lines[i])))
		}
		t.Errorf("dialect.go carries %d ASSUMED marker lines for %d anchored register entries — one marker per anchored entry and no others; the markers are:\n\t%s", len(markers), anchored, strings.Join(found, "\n\t"))
	}

	claimed := make(map[int]string, anchored)
	for i, row := range assumedRegister {
		name := registerEntryNames[i]
		if row.Anchor == "" {
			if row.Elsewhere == "" {
				t.Errorf("register entry %q has neither a dialect.go anchor nor a statement of where it IS used", name)
			}
			continue
		}
		at := -1
		for j, line := range lines {
			if strings.Contains(line, row.Anchor) && !isDialectComment(line) {
				if at >= 0 {
					t.Errorf("dialect.go declares %q more than once — this test cannot say which declaration the register entry %q means", row.Anchor, name)
				}
				at = j
			}
		}
		if at < 0 {
			t.Errorf("dialect.go has no declaration matching %q, which register entry %q names as its point of use", row.Anchor, name)
			continue
		}

		lo := at
		for lo > 0 && isDialectComment(lines[lo-1]) {
			lo--
		}
		block := lines[lo:at]
		var found []int
		for j := lo; j < at; j++ {
			if isASSUMEDMarker(lines[j]) {
				found = append(found, j)
			}
		}
		for _, j := range found {
			claimed[j] = name
		}
		switch {
		case len(found) == 0:
			t.Errorf("dialect.go:%d %q carries NO ASSUMED marker in its own comment block (lines %d-%d), but doc.go registers %q — a registered assumption with no marker at its point of use is one a later reader takes for a transcription", at+1, row.Anchor, lo+1, at, name)
		case len(found) > 1:
			t.Errorf("dialect.go:%d %q has %d ASSUMED markers in its own comment block (lines %d-%d) — one field, one marker, one register entry", at+1, row.Anchor, len(found), lo+1, at)
		}

		// The block must NAME its entry, and only its entry. This is what
		// makes the match a match rather than a coincidence of position: a
		// marker that has drifted onto the wrong field still says which
		// entry it belongs to, and says it in one place a reader can grep.
		text := unwrapComment(block)
		var cited []string
		for k, other := range assumedRegister {
			if strings.Contains(text, `"`+other.Cite+`"`) {
				cited = append(cited, registerEntryNames[k])
			}
		}
		if len(cited) != 1 || cited[0] != name {
			t.Errorf("dialect.go:%d %q's own comment block (lines %d-%d) quotes %d register entry names %q — a marker is matched to its entry BY NAME, so the block must quote exactly one, %q", at+1, row.Anchor, lo+1, at, len(cited), cited, name)
		}
	}

	// REVERSE WALK: every marker line must be one an anchored entry claimed.
	// A marker in no registered field's block is a field the register does
	// not know about — the M9d-1 shape (an assumption travelling
	// unregistered) run the other way.
	for _, i := range markers {
		if _, ok := claimed[i]; !ok {
			t.Errorf("dialect.go:%d carries an ASSUMED marker no register entry claims: %q — it is in no anchored field's own comment block, so it is an assumption travelling unregistered", i+1, strings.TrimSpace(lines[i]))
		}
	}
}
