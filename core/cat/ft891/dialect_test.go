// SPDX-License-Identifier: GPL-3.0-or-later

package ft891_test

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/cat/dialecttest"
	"github.com/gm5dna/open-rig-programmer/core/cat/ft891"
	"github.com/gm5dna/open-rig-programmer/core/cat/ftdx10"
	"github.com/gm5dna/open-rig-programmer/internal/extable"
)

// This file is Stage 1 item 4's evidence: the FT-891 dialect held to
// core/cat's conformance suite, then pinned against the FTdx10 in both
// directions — what the two radios SHARE and what they must NOT.
//
// THE FTdx10 IS THE COUNTERPART THROUGHOUT, not the FT-710, because the
// FTdx10 is the sibling whose combined MT form, 28-position memory block and
// six-digit EX address the FT-891 was said to resemble. Every difference
// pin below is a fact about TWO radios' manuals, and each half is cited to
// its own: without the counter-example half, a pin that "this dialect
// refuses X" proves only that X is refused by everybody.
//
// The comparisons are deliberately BEHAVIOURAL — asked through the exported
// API, over the whole wire space where a space exists — rather than by
// comparing two tables. A table comparison would prove the tables match and
// say nothing about what either dialect DOES with them.
//
// ASSUMED MEMBERS ARE EMBEDDED IN WHAT THE PINS COMPARE and are noted at
// each site: cat.ModeUnset ('0', "-") is in no FT-891 mode legend, the none
// wire "000" is in no FT-891 slot legend, and the combined answer's exact
// 41 is core/cat's form assumption rather than this chart's. doc.go's
// ASSUMED register carries the full statement and the Stage R capture that
// lifts each; TestASSUMEDRegisterIsComplete holds this file's list of them
// against that register mechanically.

// TestConformance runs core/cat's whole exported-API conformance suite over
// the real FT-891 dialect: every builder's frames are well-formed and
// admitted by this dialect's own gate, both wrong-form APIs refuse and are
// seen to refuse, the clarifier's endpoints build and one step past is
// refused, and the walk fails if it built nothing.
//
// It is the first run of that suite over a dialect declaring the FT-891
// reading of all five Stage 0 axes at once that is not a synthetic fixture.
func TestConformance(t *testing.T) {
	dialecttest.Run(t, ft891.Dialect())
}

// TestZeroValue runs the universal zero-value suite from OUTSIDE core/cat.
//
// core/cat runs it too, on the same function; the brief asks for it here
// because this package is a consumer of the exported API and the zero
// cat.Dialect is reachable from any consumer that declares one by mistake.
// A zero dialect that refused inside core/cat and answered plausibly outside
// it would be the failure this second call catches.
func TestZeroValue(t *testing.T) {
	dialecttest.RunZeroValue(t)
}

// modeNibbles is the FT-891's mode legend beside the FTdx10's, nibble by
// nibble, as the two manuals print them.
//
// FT891 is transcribed from the three identical FT-891 legends — MR's P6 at
// ft891_layout.txt:972-974, MT's at 1007-1010, MW's at 1043-1046 — and
// FTdx10 from that radio's four (ftdx10_layout.txt:1146-1149, 1192-1194,
// 1227-1229, 1267-1269). SIX AGREE AND SIX DISAGREE, which is the whole
// reason neither package references the other's table.
var modeNibbles = []struct {
	Wire          byte
	FT891, FTdx10 string
}{
	{'1', "LSB", "LSB"},         // shared
	{'2', "USB", "USB"},         // shared
	{'3', "CW", "CW-U"},         // differs
	{'4', "FM", "FM"},           // shared
	{'5', "AM", "AM"},           // shared
	{'6', "RTTY-LSB", "RTTY-L"}, // differs
	{'7', "CW-R", "CW-L"},       // differs
	{'8', "DATA-LSB", "DATA-L"}, // differs
	{'9', "RTTY-USB", "RTTY-U"}, // differs
	{'B', "FM-N", "FM-N"},       // shared
	{'C', "DATA-USB", "DATA-U"}, // differs
	{'D', "AM-N", "AM-N"},       // shared
}

// TestModeLegendTranscription pins the mode table's CONTENTS LITERALLY,
// over all 256 wire bytes, against the legend printed three times in manual
// rev 1909-C.
//
// The whole byte space, not the thirteen this package declares: a table with
// a SPURIOUS member — a lower-case 'a', a stray 'E' — is invisible to a walk
// over its own keys, and that is precisely the copy error a fresh
// transcription risks.
//
// THE THREE PRINTINGS ARE PINNED AS A LITERAL, NOT RE-READ. MR's P6
// (ft891_layout.txt:972-974), MT's (1007-1010) and MW's (1043-1046) carry
// the same twelve names against the same twelve nibbles, in the same order,
// with the same hole at 'A' and no 'E' or 'F'; the only difference between
// the three is whitespace (MR prints "1:LSB", MT and MW "1: LSB"). A test
// re-reading those extracts is impossible here: ft891_layout.txt is
// gitignored, so it is absent from a fresh clone and from CI. What is
// mechanical instead is that the transcription in dialect.go equals the
// transcription in this file, which two people wrote at different times from
// the same three legends.
func TestModeLegendTranscription(t *testing.T) {
	d := ft891.Dialect()

	want := make(map[byte]string, len(modeNibbles)+1)
	for _, m := range modeNibbles {
		want[m.Wire] = m.FT891
	}
	// THE ASSUMED MEMBER, stated rather than smuggled into the table above.
	// cat.ModeUnset appears in no FT-891 mode legend; it is here because
	// parsers must accept the placeholder, and core/cat refuses to emit it
	// in any Set frame. doc.go's register entry "the cat.ModeUnset member of
	// the mode table" carries the Stage R capture that lifts it.
	want[byte(cat.ModeUnset)] = "-"

	for c := 0; c < 256; c++ {
		m := cat.Mode(byte(c))
		wantName, wantValid := want[byte(c)]
		if got := d.ValidMode(m); got != wantValid {
			t.Errorf("ValidMode(%#02x): got %v, want %v — the legend printed at ft891_layout.txt:972-974, 1007-1010 and 1043-1046 runs 1..9 then B, C, D, with a printed hole at 'A' and no 'E' or 'F'", c, got, wantValid)
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

	if got, want := len(want), 13; got != want {
		t.Errorf("this file's transcription declares %d members, want %d — twelve printed names plus the ASSUMED placeholder", got, want)
	}
}

// TestDifferencePinModeMembership pins the three nibbles the FTdx10 fills
// and the FT-891 leaves empty, and the six names the two radios spell
// differently at a nibble they both fill.
//
// Each half is a fact about TWO manuals. 'A' is "DATA-FM" on the FTdx10
// (ftdx10_layout.txt:1146-1149) and a printed hole — "A: -" — on the FT-891
// (ft891_layout.txt:972-974): the dash is the chart's way of printing
// "nothing here", not a mode called "-". 'E' (PSK) and 'F' (DATA-FM-N) are
// printed on the FTdx10 and appear nowhere in any FT-891 legend.
func TestDifferencePinModeMembership(t *testing.T) {
	d := ft891.Dialect()
	other := ftdx10.Dialect()

	for _, c := range []byte{'A', 'E', 'F'} {
		m := cat.Mode(c)
		if d.ValidMode(m) {
			t.Errorf("ValidMode(%q) is TRUE on the FT-891 — its legends print \"A: -\" and no 'E' or 'F' at all, so this table has taken on a sibling's members", c)
		}
		if !other.ValidMode(m) {
			t.Errorf("ValidMode(%q) is false on the FTdx10 — that is this pin's counter-example, and without it the assertion above proves only that the nibble is unknown to everybody", c)
		}
	}

	shared, differing := 0, 0
	for _, m := range modeNibbles {
		mode := cat.Mode(m.Wire)
		if !d.ValidMode(mode) {
			t.Errorf("ValidMode(%q) is false on the FT-891 — every nibble in this table is printed in all three of its legends", m.Wire)
			continue
		}
		if !other.ValidMode(mode) {
			t.Errorf("ValidMode(%q) is false on the FTdx10 — every nibble in this table is printed in all four of its legends too", m.Wire)
			continue
		}
		gotHere, gotThere := d.ModeName(mode), other.ModeName(mode)
		if gotHere != m.FT891 {
			t.Errorf("ModeName(%q) = %q on the FT-891, want %q", m.Wire, gotHere, m.FT891)
		}
		if gotThere != m.FTdx10 {
			t.Errorf("ModeName(%q) = %q on the FTdx10, want %q — this half of the pin is a fact about THAT manual", m.Wire, gotThere, m.FTdx10)
		}
		if m.FT891 == m.FTdx10 {
			shared++
			continue
		}
		differing++
		// THE DIFFERENCE, asserted rather than merely tabulated: if the two
		// spellings ever became equal, the loop above would still pass on a
		// table someone had "tidied".
		if gotHere == gotThere {
			t.Errorf("nibble %q spells %q on BOTH radios — the FT-891 prints %q and the FTdx10 %q, and a shared spelling here means one transcription has taken the other's", m.Wire, gotHere, m.FT891, m.FTdx10)
		}
	}
	if shared != 6 || differing != 6 {
		t.Errorf("the twelve printed nibbles split %d shared / %d differing, want 6 / 6 — LSB, USB, FM, AM, FM-N and AM-N are the six the two manuals spell alike", shared, differing)
	}
}

// TestDifferencePinCATID is the identity that makes this a different radio
// at all: the FT-891 answers "ID;" with 0650 (manual rev 1909-C,
// ft891_layout.txt:763), the FTdx10 with 0761.
func TestDifferencePinCATID(t *testing.T) {
	if got := ft891.Dialect().CATID(); got != "0650" {
		t.Errorf("CATID() = %q, want %q — the ID block prints \"P1 0650: FT-891\"", got, "0650")
	}
	if got, other := ft891.Dialect().CATID(), ftdx10.Dialect().CATID(); got == other {
		t.Errorf("CATID() = %q on BOTH dialects — a shared identity would make radio detection pick whichever driver was registered first", got)
	}
}

// TestDifferencePinEXAddressForm pins the four-digit EX address against the
// FTdx10's six.
//
// The FT-891's EX grammar block prints "P1 : 0101 - 1803 (MENU Number)" and
// the Read chart "E X P1 P1 P1 P1 ;" (ft891_layout.txt:513-522); the
// FTdx10's prints "E X P1 P1 P2 P2 P3 P3" (ftdx10_layout.txt:636-645). The
// WIDTH is asserted alongside the FORM because the width is what sizes every
// EX frame this codec builds and every one its gate measures, and a form
// declared without its width reaching the frame would be a comment.
func TestDifferencePinEXAddressForm(t *testing.T) {
	d := ft891.Dialect()
	other := ftdx10.Dialect()

	if got, want := d.EXAddressWidth(), 4; got != want {
		t.Errorf("EXAddressWidth() = %d, want %d", got, want)
	}
	if got, want := other.EXAddressWidth(), 6; got != want {
		t.Errorf("the FTdx10's EXAddressWidth() = %d, want %d — this pin is a DIFFERENCE and proves nothing if both radios declare the same width", got, want)
	}

	// 0803 OTHER DISP: a member of THIS chart, and the four digits its MENU
	// Number is printed as.
	addr := cat.EXAddress{P1: 8, P2: 3, P3: 0}
	if !d.KnownEXAddress(addr) {
		t.Fatalf("KnownEXAddress(%v) is false — 0803 OTHER DISP is a row of this chart (ft891_layout.txt:595)", addr)
	}
	if got, want := d.EXWire(addr), "0803"; got != want {
		t.Errorf("EXWire(%v) = %q, want %q — the wire form IS the chart's printed MENU Number", addr, got, want)
	}

	cmd, err := d.BuildEXRead(addr)
	if err != nil {
		t.Fatalf("BuildEXRead(%v) = %v", addr, err)
	}
	if got, want := string(cmd.Bytes()), "EX0803;"; got != want {
		t.Errorf("BuildEXRead(%v) built %q, want %q — seven bytes, not nine", addr, got, want)
	}
	if !d.AllowedCommand(cmd.Bytes()) {
		t.Errorf("its own gate refused BuildEXRead(%v)'s frame %q", addr, cmd.Bytes())
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
	if got, want := len(cmd.Bytes()), 7; got != want {
		t.Errorf("the FT-891's BuildEXRead built %d bytes, want %d", got, want)
	}
}

// TestDifferencePinSixtyRange pins the 5 MHz bank's numbering.
//
// THE BOUNDS ARE TRANSCRIBED HERE AND ASSUMED THERE, which is the point.
// The FT-891's MR legend prints "501 - 510 (5 MHz, U.S. and U.K. version
// only)" (ft891_layout.txt:962, repeated by IF at 776 and OI at 1122) — the
// first Yaesu manual in this repository to print the bank's actual numbers.
// The FTdx10's says only "5xx (5MHz BAND)", and its 501..599 sits on its own
// ASSUMED register (core/cat/ftdx10/doc.go, entry "SlotSpace.SixtyLo/
// SixtyHi"). So "511" is a wire form that radio's dialect accepts and this
// one refuses, and the refusal is a reading of a printed range rather than a
// tightening of a guess.
func TestDifferencePinSixtyRange(t *testing.T) {
	d := ft891.Dialect()
	other := ftdx10.Dialect()

	for _, w := range []string{"501", "510"} {
		if _, err := d.ParseSlot(w); err != nil {
			t.Errorf("ParseSlot(%q) = %v — the MR legend prints 501 - 510", w, err)
		}
	}
	for _, w := range []string{"500", "511", "599"} {
		if _, err := d.ParseSlot(w); err == nil {
			t.Errorf("ParseSlot(%q) was ACCEPTED — this manual prints the bank as 501 - 510 and nothing wider", w)
		}
	}
	if _, err := other.ParseSlot("511"); err != nil {
		t.Errorf("the FTdx10's ParseSlot(\"511\") = %v — that is this pin's counter-example, and without it the refusal above proves only that 511 is unknown to everybody", err)
	}

	// The CONSTRUCTOR side, which is a different bound: SixtyMSlot takes an
	// ordinal and derives the wire form from sixtyLo, so a dialect with the
	// same range and a different base would agree above and disagree here.
	if s, err := d.SixtyMSlot(10); err != nil {
		t.Errorf("SixtyMSlot(10) = %v — the tenth channel of a 501..510 bank exists", err)
	} else if got, want := s.Wire(), "510"; got != want {
		t.Errorf("SixtyMSlot(10).Wire() = %q, want %q", got, want)
	}
	if _, err := d.SixtyMSlot(11); err == nil {
		t.Error("SixtyMSlot(11) was ACCEPTED — the bank this manual prints holds ten channels")
	}
	if _, err := other.SixtyMSlot(11); err != nil {
		t.Errorf("the FTdx10's SixtyMSlot(11) = %v — the counter-example half of the constructor bound", err)
	}
}

// TestDifferencePinMCSelects pins the MC command's SEND domain.
//
// The FT-891's MC block prints two slot classes — "001 - 099: Regular Memory
// Channel" and "P1L - P9U (PMS)" (ft891_layout.txt:907-909) — where the
// FTdx10's prints all four (ftdx10_layout.txt:1131-1133). An MC Set RECALLS
// a channel and changes the radio's operating state, so a frame built for a
// bank the manual never lists against MC is a side-effecting frame no
// document describes.
//
// The GATE half is asserted alongside the builder because they are two
// separate consultations of the same policy: a dialect whose gate admitted
// what its builder refuses would pass a frame assembled anywhere else.
func TestDifferencePinMCSelects(t *testing.T) {
	d := ft891.Dialect()
	other := ftdx10.Dialect()

	if got := d.MemoryP5(); got != cat.P5Fixed { // guard: wrong dialect under test
		t.Fatalf("MemoryP5() = %v — this file's subject is the FT-891", got)
	}

	mem, err := d.MemorySlot(1)
	if err != nil {
		t.Fatalf("MemorySlot(1): %v", err)
	}
	pms, err := d.PMSSlot(1, false)
	if err != nil {
		t.Fatalf("PMSSlot(1, false): %v", err)
	}
	for _, s := range []cat.Slot{mem, pms} {
		cmd, err := d.BuildMCSet(s)
		if err != nil {
			t.Errorf("BuildMCSet(%q) = %v — both classes this MC legend prints must build", s.Wire(), err)
			continue
		}
		if !d.AllowedCommand(cmd.Bytes()) {
			t.Errorf("its own gate refused BuildMCSet(%q)'s frame %q", s.Wire(), cmd.Bytes())
		}
	}

	sixty, err := d.SixtyMSlot(1)
	if err != nil {
		t.Fatalf("SixtyMSlot(1): %v", err)
	}
	for _, s := range []cat.Slot{sixty, d.EMGSlot()} {
		if cmd, err := d.BuildMCSet(s); err == nil {
			t.Errorf("BuildMCSet(%q) built %q — this radio's MC legend prints memory and PMS only", s.Wire(), cmd.Bytes())
		}
		// The gate, independently: "MC" + the wire + ";" is the frame
		// anything else would assemble.
		frame := []byte("MC" + s.Wire() + ";")
		if d.AllowedCommand(frame) {
			t.Errorf("the gate ADMITTED %q — the send-side domain is consulted in two places and both must narrow", frame)
		}
	}

	// The counter-example: the same two frames on the radio whose MC legend
	// prints all four classes.
	otherSixty, err := other.SixtyMSlot(1)
	if err != nil {
		t.Fatalf("the FTdx10's SixtyMSlot(1): %v", err)
	}
	for _, s := range []cat.Slot{otherSixty, other.EMGSlot()} {
		if _, err := other.BuildMCSet(s); err != nil {
			t.Errorf("the FTdx10's BuildMCSet(%q) = %v — its MC legend prints 5xx and EMG, and without this half the refusals above prove only that nobody builds them", s.Wire(), err)
		}
	}
}

// TestDifferencePinMTReadSlots pins the MT READ's slot domain.
//
// The FT-891's MT block prints one slot legend — "001 - 099 (Regular Memory
// Channel)" and "P1L - P9U (PMS)" (ft891_layout.txt:998-999) — and its Read
// chart is "M T P0 P0 P0 ;" (1016) with P0 DEFINED NOWHERE IN THE BLOCK, so
// there is no second legend to widen the read from. Its own MR block prints
// the 5 MHz and EMG banks (960-964), and the FTdx10's MT block prints all
// four classes (ftdx10_layout.txt:1218). The discovered banks are read by MR
// alone on this radio.
func TestDifferencePinMTReadSlots(t *testing.T) {
	d := ft891.Dialect()
	other := ftdx10.Dialect()

	mem, err := d.MemorySlot(1)
	if err != nil {
		t.Fatalf("MemorySlot(1): %v", err)
	}
	pms, err := d.PMSSlot(9, true)
	if err != nil {
		t.Fatalf("PMSSlot(9, true): %v", err)
	}
	for _, s := range []cat.Slot{mem, pms} {
		cmd, err := d.BuildMTRead(s)
		if err != nil {
			t.Errorf("BuildMTRead(%q) = %v — both classes this MT legend prints must build", s.Wire(), err)
			continue
		}
		if !d.AllowedCommand(cmd.Bytes()) {
			t.Errorf("its own gate refused BuildMTRead(%q)'s frame %q", s.Wire(), cmd.Bytes())
		}
	}

	sixty, err := d.SixtyMSlot(1)
	if err != nil {
		t.Fatalf("SixtyMSlot(1): %v", err)
	}
	for _, s := range []cat.Slot{sixty, d.EMGSlot()} {
		if cmd, err := d.BuildMTRead(s); err == nil {
			t.Errorf("BuildMTRead(%q) built %q — this radio's MT legend prints memory and PMS only", s.Wire(), cmd.Bytes())
		}
		frame := []byte("MT" + s.Wire() + ";")
		if d.AllowedCommand(frame) {
			t.Errorf("the gate ADMITTED %q — BuildMTRead and the gate's MT read branch consult one policy and both must narrow", frame)
		}
		// MR is untouched by this axis: the same slot must still be
		// readable there, which is how Stage 2 reaches the discovered banks.
		if _, err := d.BuildMRRead(s); err != nil {
			t.Errorf("BuildMRRead(%q) = %v — MTReadSlots narrows MT alone, and MR's own legend prints this slot", s.Wire(), err)
		}
	}

	otherSixty, err := other.SixtyMSlot(1)
	if err != nil {
		t.Fatalf("the FTdx10's SixtyMSlot(1): %v", err)
	}
	for _, s := range []cat.Slot{otherSixty, other.EMGSlot()} {
		if _, err := other.BuildMTRead(s); err != nil {
			t.Errorf("the FTdx10's BuildMTRead(%q) = %v — its MT legend prints all four classes, and without this half the refusals above prove only that nobody builds them", s.Wire(), err)
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

// TestDifferencePinMemoryP5 pins byte 21 of the shared memory block.
//
// The FT-891 prints "P5 0: (Fixed)" on every block that carries the
// 28-position grid — MR 971, MT 1006, MW 1042, IF 783, OI 1129 — where the
// FTdx10 prints `P5 0: TX CLAR "OFF" 1: TX CLAR "ON"`
// (ftdx10_layout.txt:1226). So on this radio the byte is schema, and a
// record carrying TxClar true describes something the manual does not: the
// builder refuses it rather than silently correcting it, so a caller who
// believed it was writing the TX clarifier finds out.
func TestDifferencePinMemoryP5(t *testing.T) {
	d := ft891.Dialect()
	other := ftdx10.Dialect()

	if got := d.MemoryP5(); got != cat.P5Fixed {
		t.Fatalf("MemoryP5() = %v, want P5Fixed", got)
	}
	if got := other.MemoryP5(); got != cat.P5TxClar {
		t.Fatalf("the FTdx10's MemoryP5() = %v, want P5TxClar — this pin is a DIFFERENCE", got)
	}

	slot, err := d.MemorySlot(7)
	if err != nil {
		t.Fatalf("MemorySlot(7): %v", err)
	}
	cmd, err := d.BuildMWSet(recordFor(d, slot, false))
	if err != nil {
		t.Fatalf("BuildMWSet with TxClar false = %v — P5Fixed refuses the FLAG, not the record", err)
	}
	// Position 21, 1-indexed as the manual's table numbers it.
	if got := cmd.Bytes()[20]; got != '0' {
		t.Errorf("BuildMWSet emitted %q, whose position 21 is %q, want '0' — the legend prints it \"(Fixed)\"", cmd.Bytes(), got)
	}
	if _, err := d.BuildMWSet(recordFor(d, slot, true)); err == nil {
		t.Error("BuildMWSet with TxClar true SUCCEEDED — this manual prints byte 21 \"(Fixed)\" on every memory block")
	}

	otherSlot, err := other.MemorySlot(7)
	if err != nil {
		t.Fatalf("the FTdx10's MemorySlot(7): %v", err)
	}
	otherCmd, err := other.BuildMWSet(recordFor(other, otherSlot, true))
	if err != nil {
		t.Fatalf("the FTdx10's BuildMWSet with TxClar true = %v — its P5 legend prints both values, and without this half the refusal above proves only that nobody writes a TX clarifier", err)
	}
	if got := otherCmd.Bytes()[20]; got != '1' {
		t.Errorf("the FTdx10's TxClar-true frame %q has position 21 %q, want '1'", otherCmd.Bytes(), got)
	}
}

// TestDifferencePinMTP11 pins byte 28 of the combined MT record.
//
// The FT-891's MT block prints `P11 0: TAG "OFF" 1: TAG "ON"`
// (ft891_layout.txt:1016): byte 28 is a LIVE FLAG the caller supplies and
// the radio reports. The FTdx10's prints "P11 0: (Fixed)"
// (ftdx10_layout.txt:1235). A live flag is never defaulted, so the
// display-LESS pair refuses here and the display-BEARING pair refuses there
// — the refusals run in opposite directions, which is what makes this a
// difference rather than a restriction.
func TestDifferencePinMTP11(t *testing.T) {
	d := ft891.Dialect()
	other := ftdx10.Dialect()

	if got := d.MTP11(); got != cat.P11TagDisplay {
		t.Fatalf("MTP11() = %v, want P11TagDisplay", got)
	}
	if got := other.MTP11(); got != cat.P11Fixed {
		t.Fatalf("the FTdx10's MTP11() = %v, want P11Fixed — this pin is a DIFFERENCE", got)
	}

	slot, err := d.MemorySlot(7)
	if err != nil {
		t.Fatalf("MemorySlot(7): %v", err)
	}
	m := recordFor(d, slot, false)
	m.Kind = cat.CombinedMTSetKind // the FORM's constant, not this dialect's MW kind

	for _, display := range []bool{false, true} {
		cmd, err := d.BuildMTSetCombinedDisplay(m, "CQ", display)
		if err != nil {
			t.Errorf("BuildMTSetCombinedDisplay(display=%v) = %v — under P11TagDisplay this is the radio's own builder", display, err)
			continue
		}
		want := byte('0')
		if display {
			want = '1'
		}
		// Position 28, 1-indexed as the manual's table numbers it.
		if got := cmd.Bytes()[27]; got != want {
			t.Errorf("BuildMTSetCombinedDisplay(display=%v) emitted %q, whose position 28 is %q, want %q", display, cmd.Bytes(), got, want)
		}
		if !d.AllowedCommand(cmd.Bytes()) {
			t.Errorf("its own gate refused %q", cmd.Bytes())
		}
		_, gotTag, gotDisplay, err := d.ParseMTAnswerCombinedDisplay(cmd.Bytes())
		if err != nil {
			t.Errorf("ParseMTAnswerCombinedDisplay(%q) = %v", cmd.Bytes(), err)
			continue
		}
		if gotTag != "CQ" || gotDisplay != display {
			t.Errorf("ParseMTAnswerCombinedDisplay(%q) = (%q, %v), want (%q, %v)", cmd.Bytes(), gotTag, gotDisplay, "CQ", display)
		}
	}

	// The display-LESS pair must refuse here: writing '0' for a flag the
	// caller never expressed an intention about is what P11TagDisplay
	// forbids.
	if got, err := d.BuildMTSetCombined(m, "CQ"); err == nil {
		t.Errorf("BuildMTSetCombined succeeded, emitting %q — this manual prints byte 28 as a live TAG flag, which may not be defaulted", got.Bytes())
	}
	built, err := d.BuildMTSetCombinedDisplay(m, "CQ", true)
	if err != nil {
		t.Fatalf("BuildMTSetCombinedDisplay = %v", err)
	}
	if _, _, err := d.ParseMTAnswerCombined(built.Bytes()); err == nil {
		t.Error("ParseMTAnswerCombined accepted a frame whose byte 28 is a live flag — the flag would be dropped on the floor")
	}

	// The counter-example, refusing the other way round.
	otherSlot, err := other.MemorySlot(7)
	if err != nil {
		t.Fatalf("the FTdx10's MemorySlot(7): %v", err)
	}
	om := recordFor(other, otherSlot, false)
	om.Kind = cat.CombinedMTSetKind
	if _, err := other.BuildMTSetCombined(om, "CQ"); err != nil {
		t.Errorf("the FTdx10's BuildMTSetCombined = %v — under P11Fixed that is ITS pair, and without this half the refusal above proves only that nobody builds a combined MT Set", err)
	}
	if got, err := other.BuildMTSetCombinedDisplay(om, "CQ", true); err == nil {
		t.Errorf("the FTdx10's BuildMTSetCombinedDisplay succeeded, emitting %q — that radio has no TAG flag for a caller to set", got.Bytes())
	}
}

// TestIdentityPinFrameGeometry pins the three memory frame lengths the two
// radios SHARE: the combined MT record at 41, the MW Set at 28 and the MR
// Answer at 28.
//
// These are facts about two radios, not a shared definition. The FT-891's
// MT chart runs to 41 (ft891_layout.txt:996-1027), its MW Set to 28
// (1034-1042) and its MR Answer to 28 (968-975); the FTdx10's charts run to
// the same three numbers. 41 is written nowhere in core/cat and must not be
// — the geometry is derived, 29 + TagMaxBytes — so what this pin states is
// the arithmetic's ANSWER for these two radios.
//
// THE COMBINED ANSWER'S EXACTNESS IS ASSUMED, NOT CHART-PROVEN (doc.go's
// register, entry "the combined MT answer's EXACT length"): the grid draws
// the MAXIMAL frame, and the FT-710 precedent — hardware accepting short MT
// Sets against a maximal grid — makes a variable-width answer live. What
// this pin proves is that the DIALECT declares the exact form core/cat's
// combined seam implements.
func TestIdentityPinFrameGeometry(t *testing.T) {
	d := ft891.Dialect()
	other := ftdx10.Dialect()

	for _, r := range []struct {
		name string
		d    cat.Dialect
	}{{"FT-891", d}, {"FTdx10", other}} {
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
		// and ParseMRAnswer is this package's only route to the decoder
		// since MW has no Answer form.
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

// TestIdentityPinSlotSpaceShared pins the parts of the slot space the two
// radios really do share: nine PMS pairs, and "EMG" as the emergency wire.
//
// The 5 MHz bank is deliberately NOT here — it is the difference pin above —
// and neither is the none wire, which is ASSUMED on both dialects (doc.go's
// register, entry "SlotSpace.NoneWire") and so would be an identity between
// two inheritances rather than between two manuals.
func TestIdentityPinSlotSpaceShared(t *testing.T) {
	d := ft891.Dialect()
	other := ftdx10.Dialect()

	for _, r := range []struct {
		name string
		d    cat.Dialect
	}{{"FT-891", d}, {"FTdx10", other}} {
		for pair := 1; pair <= 9; pair++ {
			for _, upper := range []bool{false, true} {
				s, err := r.d.PMSSlot(pair, upper)
				if err != nil {
					t.Errorf("%s: PMSSlot(%d, %v) = %v — both manuals print P1L - P9U", r.name, pair, upper, err)
					continue
				}
				suffix := "L"
				if upper {
					suffix = "U"
				}
				if got, want := s.Wire(), fmt.Sprintf("P%d%s", pair, suffix); got != want {
					t.Errorf("%s: PMSSlot(%d, %v).Wire() = %q, want %q", r.name, pair, upper, got, want)
				}
			}
		}
		if _, err := r.d.PMSSlot(10, false); err == nil {
			t.Errorf("%s: PMSSlot(10, false) was ACCEPTED — nine pairs is what both legends print", r.name)
		}
		if got, want := r.d.EMGSlot().Wire(), "EMG"; got != want {
			t.Errorf("%s: EMGSlot().Wire() = %q, want %q", r.name, got, want)
		}
	}
}

// TestIdentityPinTagWidth pins the twelve-character tag field both manuals
// print, through the behaviour it governs rather than through the field.
//
// The FT-891's P12 legend reads "TAG Characters (up to 12 characters)
// (ASCII)" (ft891_layout.txt:1017) and its Set chart draws the field over
// positions 29-40; the FTdx10's says the same (ftdx10_layout.txt:1236). The
// FILL BYTE a shorter tag is padded with is ASSUMED on both (doc.go's
// register, entry "MTPolicy.TagFill"), so this pin asserts the WIDTH and the
// boundary, never what the padding is.
func TestIdentityPinTagWidth(t *testing.T) {
	d := ft891.Dialect()
	other := ftdx10.Dialect()

	twelve := "GM5DNA......" // 12 bytes
	if len(twelve) != 12 {
		t.Fatalf("the test's tag is %d bytes, want 12", len(twelve))
	}

	slot, err := d.MemorySlot(7)
	if err != nil {
		t.Fatalf("MemorySlot(7): %v", err)
	}
	m := recordFor(d, slot, false)
	m.Kind = cat.CombinedMTSetKind
	if _, err := d.BuildMTSetCombinedDisplay(m, twelve, true); err != nil {
		t.Errorf("BuildMTSetCombinedDisplay with a 12-byte tag = %v — the legend says \"up to 12 characters\"", err)
	}
	if got, err := d.BuildMTSetCombinedDisplay(m, twelve+"X", true); err == nil {
		t.Errorf("BuildMTSetCombinedDisplay accepted a 13-byte tag, emitting %q", got.Bytes())
	}

	otherSlot, err := other.MemorySlot(7)
	if err != nil {
		t.Fatalf("the FTdx10's MemorySlot(7): %v", err)
	}
	om := recordFor(other, otherSlot, false)
	om.Kind = cat.CombinedMTSetKind
	if _, err := other.BuildMTSetCombined(om, twelve); err != nil {
		t.Errorf("the FTdx10's BuildMTSetCombined with a 12-byte tag = %v — the shared width is a fact about two manuals and needs both halves", err)
	}
	if got, err := other.BuildMTSetCombined(om, twelve+"X"); err == nil {
		t.Errorf("the FTdx10's BuildMTSetCombined accepted a 13-byte tag, emitting %q", got.Bytes())
	}
}

// TestIdentityPinMWWriteKind pins the MW P7 byte.
//
// THE CAVEAT IS THE POINT, and it is the FTdx10 test's caveat repeated
// because it applies to a second radio now. The FT-891's MW legend reads
// "P7 0: (Fixed)" (ft891_layout.txt:1047) and cat.CombinedMTSetKind is the
// byte '0', so the constant on the right is the correct SPELLING of what
// this radio documents. That the two coincide is A FACT OF THIS RADIO, not a
// rule: MW's P7 and the combined MT Set's P7 (also "0: (Fixed)", 1011) are
// different fields of different commands that this manual happens to fix at
// the same byte. core/cat keeps them apart on purpose —
// validateCombinedMTFields uses the FORM's constant and never this dialect's
// mwWriteKind — and nothing here may be read as permission to derive one
// from the other.
//
// The FT-710 is the counter-example, and it is asserted rather than
// described: its MW kind is cat.KindMemory ('1'), hardware-confirmed.
func TestIdentityPinMWWriteKind(t *testing.T) {
	if got := ft891.Dialect().MWWriteKind(); got != cat.CombinedMTSetKind {
		t.Errorf("MWWriteKind() = %q, want %q (cat.CombinedMTSetKind) — this manual's MW legend reads \"P7 0: (Fixed)\"", got, cat.CombinedMTSetKind)
	}
	if got := ftdx10.Dialect().MWWriteKind(); got != cat.CombinedMTSetKind {
		t.Errorf("the FTdx10's MWWriteKind() = %q, want %q — the two manuals print the same MW P7, which is what makes this an IDENTITY pin", got, cat.CombinedMTSetKind)
	}
	if got := cat.FT710.MWWriteKind(); got == cat.CombinedMTSetKind {
		t.Errorf("the FT-710's MWWriteKind() is %q too — that radio documents '1' (Memory), hardware-confirmed, and without the counter-example this pin would read as a rule of the codec", got)
	}
}

// TestEXItemsCountMatchesProfile holds the dialect's inventory against the
// REGISTERED profile's ExpectedRows.
//
// ExpectedRows comes from the group-boundary ledger, derived from the
// rendered PDF by a quarantined agent before either transcription existed
// (core/cat/ft891/testdata/); the inventory comes from table2.csv through
// the generator. So this is a THIRD consultation of the row count, taken
// from the registry rather than from a literal here — the "bound consulted
// from one place with its datum taken from another" rule, which a hardcoded
// 159 in this file would break.
//
// The profile is selected by Package, not by the lookup name, for the reason
// staleness_test.go gives.
func TestEXItemsCountMatchesProfile(t *testing.T) {
	var matches []extable.NamedProfile
	for _, np := range extable.RegisteredProfiles() {
		if np.Profile.Package == "ft891" {
			matches = append(matches, np)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("registry holds %d profiles emitting into package ft891, want exactly 1: %v", len(matches), names(matches))
	}
	p := matches[0].Profile

	items := ft891.Dialect().EXItems()
	if len(items) != p.ExpectedRows {
		t.Errorf("Dialect().EXItems() holds %d items, the registered ft891 profile's ExpectedRows is %d — the ledger, the transcription and the dialect must agree, and the arbitration is against the PDF", len(items), p.ExpectedRows)
	}
	if len(items) == 0 {
		t.Fatal("Dialect().EXItems() is empty — every EX assertion in this file would be vacuous")
	}
	// Every member's P3 is 0, which V12 requires of an EXAddressPair
	// inventory: the four-digit render drops P3, and a component silently
	// dropped from every frame is what that rule exists to make impossible.
	for _, it := range items {
		if it.Addr.P3 != 0 {
			t.Errorf("item %v has P3 %d, want 0 — this chart's MENU Number is two components", it.Addr, it.Addr.P3)
		}
	}
}

// TestEXAnswerBound proves the EX answer's upper length bound is THIS
// DIALECT'S OWN, derived from its own inventory.
//
// The maximum is recomputed here from Dialect().EXItems() rather than taken
// from a constant, from the profile or from core/cat: the bound and the
// datum it is derived from must not come from the same place twice. It
// reaches 5 through exactly two items — 0803 OTHER DISP and 0804 OTHER SHIFT
// (ft891_layout.txt:595-596), whose signed "-3000 Hz - 0 - +3000 Hz"
// parameter counts its sign as a digit, and which are why the ft891 profile
// declares MaxDigits 5 where the other three declare 4.
//
// The behavioural half is what matters: the derived width must be what the
// parser actually enforces, one byte either side.
func TestEXAnswerBound(t *testing.T) {
	d := ft891.Dialect()

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
	for _, it := range items {
		if it.Digits == maxDigits {
			wide++
		}
	}
	if maxDigits != 5 {
		t.Fatalf("max(Digits) over the FT-891's %d inventory items is %d, want 5", len(items), maxDigits)
	}
	if wide != 2 {
		t.Errorf("%d items carry Digits 5, want 2 (0803 OTHER DISP and 0804 OTHER SHIFT)", wide)
	}

	addr := cat.EXAddress{P1: 8, P2: 3, P3: 0}
	if !d.KnownEXAddress(addr) {
		t.Fatalf("KnownEXAddress(%v) is false — this test needs a member address to answer at", addr)
	}

	// The body is returned VERBATIM — no per-item width policy is applied at
	// parse, by core/cat's documented decision — so a five-byte body is
	// admissible at any member address, not only at the two widest rows'.
	body := "-3000" // 5 bytes, and 0803's own printed form
	if len(body) != maxDigits {
		t.Fatalf("the test's parameter body is %d bytes, want %d", len(body), maxDigits)
	}
	frame := []byte("EX" + d.EXWire(addr) + body + ";")
	if got, want := string(frame), "EX0803-3000;"; got != want {
		t.Fatalf("the test built %q, want %q — a twelve-byte answer: \"EX\", four address digits, five body bytes and the terminator", got, want)
	}
	gotAddr, gotBody, err := d.ParseEXAnswer(frame)
	if err != nil {
		t.Errorf("ParseEXAnswer(%q) = %v — a %d-byte parameter is this dialect's own widest item, so its parser must read one", frame, err, maxDigits)
	} else {
		if gotAddr != addr {
			t.Errorf("ParseEXAnswer(%q) returned address %v, want %v", frame, gotAddr, addr)
		}
		if gotBody != body {
			t.Errorf("ParseEXAnswer(%q) returned the parameter %q, want %q verbatim", frame, gotBody, body)
		}
	}

	over := []byte("EX" + d.EXWire(addr) + body + "0" + ";")
	if _, _, err := d.ParseEXAnswer(over); err == nil {
		t.Errorf("ParseEXAnswer(%q) ACCEPTED a %d-byte parameter, one past this dialect's widest inventory item — the bound is not deriving from this inventory", over, maxDigits+1)
	}
}

// assumedRegister is this file's copy of doc.go's ASSUMED register: one row
// per entry, naming the entry EXACTLY as doc.go opens its bullet, and saying
// where the assumption is USED.
//
// It is not a second statement of the assumptions — the register is the
// statement — it is the machinery that keeps the register and dialect.go
// from drifting apart. Four of the eight are fields of this package's own
// DialectConfig literal and must carry an ASSUMED marker at that field;
// the other four are assumptions this dialect INHERITS from core/cat's codec
// and so have no field here to mark, which the Where column records rather
// than leaves to inference.
var assumedRegister = []struct {
	// Entry is the opening text of the register bullet in doc.go.
	Entry string
	// Anchor is the line in dialect.go whose declaration carries the
	// assumption; "" means the point of use is outside this package.
	Anchor string
	// Elsewhere says where an entry with no Anchor is actually used.
	Elsewhere string
}{
	{Entry: "MTPolicy.TagFill", Anchor: "TagFill:"},
	{Entry: "THE COMBINED MT ANSWER'S EXACT LENGTH", Elsewhere: "core/cat/mtcombined.go's fixed 29 + TagMaxBytes geometry"},
	{Entry: "SlotSpace.NoneWire", Anchor: "NoneWire:"},
	{Entry: "THE cat.ModeUnset MEMBER OF THE MODE TABLE", Anchor: "cat.ModeUnset:"},
	{Entry: "ClarifierPolicy.StepHz = 10 AND ClarifierPolicy.MaxAbsHz", Anchor: "StepHz:"},
	{Entry: "THE CLARIFIER'S MINUS-DIRECTION BYTE", Elsewhere: "core/cat/memdata.go's sign encoding and parsing"},
	{Entry: "THE COMBINED ANSWER'S P7 READ DOMAIN", Elsewhere: "core/cat/mtcombined.go's parse-side P7 tolerance"},
	{Entry: "THE MC ANSWER DOMAIN BEYOND MEMORY AND PMS", Elsewhere: "core/cat/mc.go's mcParseValid, the parse-side predicate"},
}

// TestASSUMEDRegisterIsComplete holds doc.go's ASSUMED register and
// dialect.go's ASSUMED markers to each other, and both to the table above.
//
// THE FAILURE IT EXISTS TO CATCH is an assumption that travels
// unregistered: a field marked ASSUMED in dialect.go with no register entry
// is invisible to Stage R, and a register entry whose field lost its marker
// is a value a later reader will take for a transcription. The M9d-1
// adjudication found exactly that shape — the clarifier's minus byte
// assumed in two dialects and registered in neither — which is why this
// package gets the check mechanically rather than by review.
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
		t.Fatalf("doc.go has no %q heading — the register is this package's statement of record", startHeading)
	}
	rest := docSrc[start+len(startHeading):]
	end := bytes.Index(rest, []byte("\n// # "))
	if end < 0 {
		t.Fatal("doc.go's ASSUMED register is not followed by another heading — this test cannot tell where the section ends")
	}
	section := string(rest[:end])

	// Every bullet in the section, in order.
	var bullets []string
	for _, line := range strings.Split(section, "\n") {
		if s, ok := strings.CutPrefix(line, "//   - "); ok {
			bullets = append(bullets, s)
		}
	}
	if len(bullets) != len(assumedRegister) {
		t.Fatalf("doc.go's ASSUMED register holds %d entries, this file's table holds %d — an entry added to one and not the other is exactly the drift this test exists to stop; the bullets are %q", len(bullets), len(assumedRegister), bullets)
	}
	if !strings.Contains(section, "EIGHT members") || len(assumedRegister) != 8 {
		t.Errorf("the register's prose says a count that no longer matches its %d entries — the sentence opening the section names the number out loud", len(assumedRegister))
	}
	for i, row := range assumedRegister {
		if !strings.HasPrefix(bullets[i], row.Entry) {
			t.Errorf("register entry %d opens %q, this file's table names it %q — entries are cited BY NAME, so the two spellings must match", i+1, bullets[i], row.Entry)
		}
	}

	// The 5 MHz numbering is a NON-entry, and the register says so out loud
	// because every sibling carries it as an assumption.
	for _, want := range []string{"DELIBERATELY NOT AN ENTRY", "SixtyLo", "501 - 510"} {
		if !strings.Contains(section, want) {
			t.Errorf("the register no longer records %q — the FT-891's 5 MHz bounds are TRANSCRIBED where its siblings' are assumed, and dropping that statement would leave a reader to assume the sibling reading", want)
		}
	}

	// The markers in dialect.go, each within reach of its own field.
	lines := strings.Split(string(dialectSrc), "\n")
	for _, row := range assumedRegister {
		if row.Anchor == "" {
			if row.Elsewhere == "" {
				t.Errorf("register entry %q has neither a dialect.go anchor nor a statement of where it IS used", row.Entry)
			}
			continue
		}
		at := -1
		for i, line := range lines {
			if strings.Contains(line, row.Anchor) && !strings.HasPrefix(strings.TrimSpace(line), "//") {
				if at >= 0 {
					t.Errorf("dialect.go declares %q more than once — this test cannot say which declaration the register entry %q means", row.Anchor, row.Entry)
				}
				at = i
			}
		}
		if at < 0 {
			t.Errorf("dialect.go has no declaration matching %q, which register entry %q names as its point of use", row.Anchor, row.Entry)
			continue
		}
		// The marker sits in the field's own comment, immediately above it
		// or on the line itself. Twelve lines is the longest such comment in
		// this file; a wider window would start reaching the field above.
		const window = 12
		lo := at - window
		if lo < 0 {
			lo = 0
		}
		if !strings.Contains(strings.Join(lines[lo:at+1], "\n"), "ASSUMED") {
			t.Errorf("dialect.go's %q carries no ASSUMED marker within %d lines above it, but doc.go registers %q — a registered assumption with no marker at its point of use is one a later reader takes for a transcription", row.Anchor, window, row.Entry)
		}
	}
}
