// SPDX-License-Identifier: GPL-3.0-or-later

package transport_test

import (
	"fmt"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/kw/ts480"
	"github.com/gm5dna/open-rig-programmer/internal/fakets480"
)

// The TS-480's half of the two-independent-transcriptions cross-check. The
// file comment on ex_crosscheck_ts590_test.go states the design, why this is
// package transport_test, and what is deliberately not compared; the shared
// comparison (compareEXInventories), frame reader and copy-not-move assertion
// live there.
//
// THIS ROW IS BUILT AND NOT REGISTERED at this milestone's close (the plan's
// P3), and it is held to the same bar anyway: the cross-check runs, the whole
// domain goes over a wire, and the copy is pinned byte-identical. A row whose
// evidence chain were checked only once it was registered would be a row whose
// registration commit had to carry the checking.

// lastMenu480 is this chart's printed menu domain, "000 ~ 060: Menu No."
// (480:401), written as a literal: a bound taken from either table it bounds
// would prove nothing.
const lastMenu480 = 60

// TestEXInventoryCrossCheck_TS480AddressesAndWidthsAgree is the table-level
// leg.
func TestEXInventoryCrossCheck_TS480AddressesAndWidthsAgree(t *testing.T) {
	items, fake := ts480.EXItems(), fakets480.EXDefaults()

	// Vacuity guard: two empty sets are trivially equal, and an inventory
	// that failed to generate would be exactly that.
	if len(items) == 0 || len(fake) == 0 {
		t.Fatalf("empty inventory: the codec has %d addresses, the fake has %d — one side failed to generate, and the comparison below would pass vacuously", len(items), len(fake))
	}
	if want := lastMenu480 + 1; len(items) != want || len(fake) != want {
		t.Errorf("the codec has %d addresses and the fake %d; this chart's printed domain is 000 ~ %03d, which is %d menus (480:401)", len(items), len(fake), lastMenu480, want)
	}

	if ms := compareEXInventories(items, fake); len(ms) > 0 {
		t.Errorf("TS-480 EX inventories disagree between the two independent transcriptions of the parameter list — %d addresses:\n%s"+
			"This is a genuine cross-check finding, not a bug in this test: do not modify either table to make it pass — report the diff and arbitrate against the PDF.",
			len(ms), renderMismatches(ms))
	}
}

// TestEXInventoryCrossCheck_TS480RedProofs runs the same comparison over
// DELIBERATELY perturbed copies of the fake's side, so that the green result
// above is known to be a result rather than a function that cannot fail.
func TestEXInventoryCrossCheck_TS480RedProofs(t *testing.T) {
	items := ts480.EXItems()

	t.Run("a dropped address", func(t *testing.T) {
		fake := fakets480.EXDefaults()
		delete(fake, "034")
		ms := compareEXInventories(items, fake)
		if len(ms) != 1 || ms[0].addr != "034" {
			t.Errorf("dropping menu 034 from the fake's side reported %v, want exactly one mismatch at 034", ms)
		}
	})

	t.Run("a width-only perturbation", func(t *testing.T) {
		fake := fakets480.EXDefaults()
		if got := fake["034"]; got != "00" {
			t.Fatalf("menu 034's fake default is %q, and this proof assumes two bytes", got)
		}
		fake["034"] = "0"
		ms := compareEXInventories(items, fake)
		if len(ms) != 1 || ms[0].addr != "034" {
			t.Errorf("narrowing menu 034's fake default to one byte reported %v, want exactly one mismatch at 034", ms)
		}
	})

	// And the control: the comparison is not simply returning something for
	// everything.
	if ms := compareEXInventories(items, fakets480.EXDefaults()); len(ms) != 0 {
		t.Errorf("the unperturbed comparison reported %d mismatches — the proofs above prove nothing", len(ms))
	}
}

// TestEXInventoryCrossCheck_TS480MenuThirtyFourIsTwoWideOnBothSides pins the
// printed defect this pair carries unchanged, from BOTH sides at once.
//
// The EX block's prose lists the two-digit menus as "Menu No. 32, 35 and 48 ~
// 52" (480:411) and omits 034, whose grid row reaches the second parameter
// column all the same. Both derivations read the GRID. No comparison in this
// repository can CATCH a defect that is printed — three faithful readings of
// one wrong cell agree perfectly — so the pin is what makes it a deliberate
// state rather than a coincidence, and it is deliberately written at the one
// address the prose and the grid disagree about.
func TestEXInventoryCrossCheck_TS480MenuThirtyFourIsTwoWideOnBothSides(t *testing.T) {
	const menu, wantDigits = 34, 2
	fake := fakets480.EXDefaults()
	if got := len(fake[fmt.Sprintf("%03d", menu)]); got != wantDigits {
		t.Errorf("the fake answers %d bytes at menu %03d, want %d", got, menu, wantDigits)
	}
	found := false
	for _, item := range ts480.EXItems() {
		if int(item.Addr.P1) != menu {
			continue
		}
		found = true
		if item.Digits != wantDigits {
			t.Errorf("the codec's inventory gives menu %03d Digits=%d (%q), want %d", menu, item.Digits, item.Name, wantDigits)
		}
	}
	if !found {
		t.Errorf("menu %03d is absent from the codec's inventory", menu)
	}
}

// TestEXInventoryCrossCheck_TS480HasNoTextRow. The ts480 profile registers
// TextRowsAbsent — this book prints no "ASCII characters" row, no message and
// no version string — so the codec's side must mark none, and the fake's
// uniform placeholder is the whole of the shape question on this radio.
//
// It is the counterpart of the 590 pair's text-row pin, in the direction this
// chart demands: transcription B DOES flag five rows here (menus 048-052,
// whose merged cell prints the numeric legend "00 ~ 99 (2-digit)"), and
// core/kw/ts480/crosscheck_test.go records the ruling that text=0 is the
// repository's reading of all five. The fake's generator therefore does not
// project the flag, and this test is what would notice if A ever acquired one.
func TestEXInventoryCrossCheck_TS480HasNoTextRow(t *testing.T) {
	for _, item := range ts480.EXItems() {
		if item.Text {
			t.Errorf("the codec's inventory marks menu %s (%q) as text, and this chart prints no free-text item (TextRowsAbsent) — a new text row is a finding to arbitrate, not a pin to widen", item.Addr.Wire(), item.Name)
		}
	}
}

// TestEXTS480RoundTrip_AllAddressesRawPort is the wire-level leg: every
// address the CODEC's inventory declares is asked of a real fakets480.Radio
// through its Port(), and each answer is parsed back with the codec's own
// ParseEXAnswer against that inventory row.
//
// A projection that produced the right table but answered the wrong bytes — a
// handler bug, an off-by-one in the answer builder, a P2/P3/P4 in the wrong
// position — fails only here.
func TestEXTS480RoundTrip_AllAddressesRawPort(t *testing.T) {
	layout, items := ts480.Layout(), ts480.EXItems()
	if len(items) == 0 {
		t.Fatal("the codec's inventory is empty — this test would pass vacuously")
	}

	r := fakets480.New()
	t.Cleanup(func() { _ = r.Close() })
	fake := fakets480.EXDefaults()
	reader := newKWFrameReader(t, r.Port())

	answered := 0
	for _, item := range items {
		cmd, err := layout.BuildEXRead(item.Addr)
		if err != nil {
			t.Fatalf("BuildEXRead(%v): unexpected error: %v", item.Addr, err)
		}
		if got := len(cmd.Bytes()); got != kw.EXReadLen {
			t.Fatalf("BuildEXRead(%v) built a %d-byte frame (%q), want %d (480:410)", item.Addr, got, cmd.Bytes(), kw.EXReadLen)
		}
		if _, err := r.Port().Write(cmd.Bytes()); err != nil {
			t.Fatalf("Write(%v %q): unexpected error: %v", item.Addr, cmd.Bytes(), err)
		}

		frame := reader.readOneFrame()
		gotP5, err := layout.ParseEXAnswer(frame, item)
		if err != nil {
			t.Fatalf("ParseEXAnswer(%q) for %v: unexpected error: %v — the fake refused or malformed an address the codec's inventory declares", frame, item.Addr, err)
		}
		if len(gotP5) != item.Digits {
			t.Errorf("%v (%s): answered %d raw P5 bytes (%q), want %d per the codec's inventory", item.Addr, item.Name, len(gotP5), gotP5, item.Digits)
		}
		if want := fake[item.Addr.Wire()]; gotP5 != want {
			t.Errorf("%v: raw P5 = %q, want %q (the fake's own default)", item.Addr, gotP5, want)
		}
		answered++
	}
	if answered != len(items) {
		t.Errorf("%d of %d inventory addresses answered as expected", answered, len(items))
	}
}

// TestEXTS480RoundTrip_TheBooksOwnWorkedAnswerIsTheOneAtMenuZero. Both Kenwood
// books print exactly one complete EX ANSWER between them, and it is this
// radio's: "EX00000000; (Display illumination OFF)" (480:415). So this is the
// only place in the milestone where a frame on a wire can be compared with a
// literal the document itself prints, rather than with a composition of
// printed fields — and it is checked from both ends, the fake's bytes and the
// codec's parse of them.
func TestEXTS480RoundTrip_TheBooksOwnWorkedAnswerIsTheOneAtMenuZero(t *testing.T) {
	layout := ts480.Layout()
	var zero kw.EXItem
	for _, item := range ts480.EXItems() {
		if item.Addr.P1 == 0 {
			zero = item
		}
	}
	if zero.Digits == 0 {
		t.Fatal("menu 000 is absent from the codec's inventory")
	}

	r := fakets480.New()
	t.Cleanup(func() { _ = r.Close() })
	reader := newKWFrameReader(t, r.Port())

	cmd, err := layout.BuildEXRead(zero.Addr)
	if err != nil {
		t.Fatalf("BuildEXRead(%v): %v", zero.Addr, err)
	}
	if _, err := r.Port().Write(cmd.Bytes()); err != nil {
		t.Fatalf("Write: %v", err)
	}
	frame := reader.readOneFrame()
	if got, want := string(frame), "EX00000000;"; got != want {
		t.Errorf("the answer at menu 000 is %q, want %q — the book's own worked frame (480:415)", got, want)
	}
	if got, err := layout.ParseEXAnswer(frame, zero); err != nil || got != "0" {
		t.Errorf("ParseEXAnswer(%q) = %q, %v, want \"0\", nil", frame, got, err)
	}
}

// TestEXTS480RoundTrip_OutOfDomainAddressIsRefused is the round trip's
// negative control. Without it, a fake that answered EVERY three-digit address
// with something plausible would pass the sweep above completely.
//
// 061 IS THE FAMILY'S OWN CASE: one past this chart's printed domain
// (480:401), and a real menu on BOTH 590 rows — so a fake or a codec that had
// borrowed a sibling's inventory answers it. The codec REFUSES TO BUILD a read
// for it, which is itself worth pinning, so the frame is written by hand.
func TestEXTS480RoundTrip_OutOfDomainAddressIsRefused(t *testing.T) {
	layout := ts480.Layout()
	r := fakets480.New()
	t.Cleanup(func() { _ = r.Close() })
	reader := newKWFrameReader(t, r.Port())

	beyond := kw.EXAddress{P1: lastMenu480 + 1}
	if _, err := layout.BuildEXRead(beyond); err == nil {
		t.Fatalf("the codec built a read for menu %v, which is outside this chart's printed domain — this negative control is testing the wrong thing", beyond)
	}
	for _, wire := range []string{
		fmt.Sprintf("EX%03d0000;", lastMenu480+1), // a real menu on both 590 rows
		"EX0870000;", // the TS-590S's last menu
		"EX0990000;", // the TS-590SG's last menu
		"EX0101;",    // an FT-891's seven-byte read frame
		"EX010100;",  // an FTdx10's nine-byte read frame
	} {
		if _, err := r.Port().Write([]byte(wire)); err != nil {
			t.Fatalf("Write: unexpected error: %v", err)
		}
		if got, want := string(reader.readOneFrame()), "?;"; got != want {
			t.Errorf("%s -> %q, want %q", wire, got, want)
		}
	}
}

// TestTS480TranscriptionBCopy_ByteIdenticalToTheCodecs pins the COPY-NOT-MOVE
// rule mechanically (internal/fakets480/PROVENANCE.md). See its 590 sibling
// for the reasoning.
func TestTS480TranscriptionBCopy_ByteIdenticalToTheCodecs(t *testing.T) {
	assertByteIdentical(t,
		"../kw/ts480/testdata/transcription-b-480.csv",
		"../../internal/fakets480/transcription-b-480.csv",
		"./internal/fakets480")
}
