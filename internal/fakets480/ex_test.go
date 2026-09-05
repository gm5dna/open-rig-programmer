// SPDX-License-Identifier: GPL-3.0-or-later

package fakets480

import (
	"fmt"
	"strings"
	"testing"
)

// EVERY EXPECTED FRAME BELOW IS ASSEMBLED FROM THE BOOK'S OWN POSITION CHART
// rather than by calling buildEXAnswer: "E X P1 P1 P1 P2 P2 P3 P4 P5 … ;",
// with P2 "00" and P3 and P4 '0' (480:399-416). A test that built its
// expectation with the function under test could not catch that function
// putting a byte in the wrong position.

// exAnswerFrame assembles the answer the book prints for menu with raw P5 p5.
func exAnswerFrame(menu int, p5 string) string {
	// EX  P1P1P1  P2P2  P3  P4  P5...  ;
	return fmt.Sprintf("EX%03d%s%c%c%s;", menu, "00", '0', '0', p5)
}

// exReadFrame assembles the ten-byte read the book prints for menu.
func exReadFrame(menu int) string {
	return fmt.Sprintf("EX%03d0000;", menu)
}

// lastMenu is this chart's printed domain, "000 ~ 060: Menu No." (480:401),
// written as a literal rather than derived from the generated table: a bound
// taken from the thing it bounds proves nothing.
const lastMenu = 60

// TestEXDefaults_CoverExactlyThePrintedDomain pins the inventory to the domain
// the book prints, from both ends: every menu from 000 to 060 is present, and
// nothing beyond it is.
func TestEXDefaults_CoverExactlyThePrintedDomain(t *testing.T) {
	got := EXDefaults()
	if len(got) != lastMenu+1 {
		t.Errorf("EXDefaults() has %d addresses, want %d (000 ~ %03d, 480:401)", len(got), lastMenu+1, lastMenu)
	}
	for menu := 0; menu <= lastMenu; menu++ {
		if _, ok := got[fmt.Sprintf("%03d", menu)]; !ok {
			t.Errorf("menu %03d is absent from EXDefaults()", menu)
		}
	}
	if _, ok := got[fmt.Sprintf("%03d", lastMenu+1)]; ok {
		t.Errorf("menu %03d is present, and this chart's printed domain stops at %03d (480:401)", lastMenu+1, lastMenu)
	}
}

// TestEXDefaults_TheTwoDigitMenusAreTheOnesTheGridPrints pins the width
// distribution against the book, from literals — including the printed defect
// this fake carries unchanged.
//
// The EX block's prose lists "Menu No. 32, 35 and 48 ~ 52" as the two-digit
// ones (480:411) and OMITS 034, whose grid row reaches the second parameter
// column all the same. Both quarantined transcriptions read the GRID;
// core/kw/ts480 pins the omission as an erratum of the printed block; and this
// fake answers two bytes at 034 because that is what the transcription says.
// No TS-480 has been asked which the radio answers.
func TestEXDefaults_TheTwoDigitMenusAreTheOnesTheGridPrints(t *testing.T) {
	want := map[int]bool{32: true, 34: true, 35: true, 48: true, 49: true, 50: true, 51: true, 52: true}
	for menu, p5 := range EXDefaults() {
		var n int
		if _, err := fmt.Sscanf(menu, "%d", &n); err != nil {
			t.Fatalf("unparseable address %q in EXDefaults()", menu)
		}
		wantLen := 1
		if want[n] {
			wantLen = 2
		}
		if len(p5) != wantLen {
			t.Errorf("menu %s default is %d bytes (%q), want %d", menu, len(p5), p5, wantLen)
		}
	}
}

// TestEXDefaults_ReturnsAnIndependentCopy: the map is the fake's half of
// core/transport's cross-check and a test-inspection API, so a caller
// mutating what it is handed must not affect the next call's result nor any
// *Radio's stored settings.
func TestEXDefaults_ReturnsAnIndependentCopy(t *testing.T) {
	first := EXDefaults()
	first["000"] = "XX"
	delete(first, "001")
	second := EXDefaults()
	if second["000"] == "XX" {
		t.Error("mutating one EXDefaults result changed the next one")
	}
	if _, ok := second["001"]; !ok {
		t.Error("deleting from one EXDefaults result removed the entry from the next one")
	}
}

// TestEX_ReadIsAnsweredAtThePrintedWidth drives every address over the wire
// and requires each answer to be exactly the frame the book prints, carrying
// that menu's own default.
//
// It is the fake's own end-to-end leg. core/transport's cross-check does the
// same sweep with the CODEC's inventory on the other side, which is what makes
// it a cross-check; this one proves the fake self-consistent, so a failure
// there is unambiguously about the two transcriptions rather than about this
// package's plumbing.
func TestEX_ReadIsAnsweredAtThePrintedWidth(t *testing.T) {
	_, conn := newTestRadio(t)
	defaults := EXDefaults()
	for menu := 0; menu <= lastMenu; menu++ {
		read := exReadFrame(menu)
		want := exAnswerFrame(menu, defaults[fmt.Sprintf("%03d", menu)])
		if got := exchange(t, conn, read); got != want {
			t.Fatalf("%s -> %q, want %q", read, got, want)
		}
	}
}

// TestEX_MenuZeroAnswersTheBooksOwnWorkedFRAME. Menu 000 is the one address
// where this fake's uniform placeholder coincides with a frame the book prints
// in full: "EX00000000; (Display illumination OFF)" (480:415). Nothing else in
// either Kenwood book prints a complete EX answer, so this is the only byte
// sequence on this surface that can be checked against a literal rather than
// against a composition.
func TestEX_MenuZeroAnswersTheBooksOwnWorkedFRAME(t *testing.T) {
	_, conn := newTestRadio(t)
	if got, want := exchange(t, conn, exReadFrame(0)), "EX00000000;"; got != want {
		t.Errorf("EX0000000; -> %q, want %q — the book's own worked answer (480:415)", got, want)
	}
	// And the book's SECOND worked answer, reached the only honest way: by
	// scripting the value, since '3' is a setting rather than a default
	// (480:416).
	_, conn2 := newTestRadio(t, WithEXSetting("000", "3"))
	if got, want := exchange(t, conn2, exReadFrame(0)), "EX00000003;"; got != want {
		t.Errorf("EX0000000; with P5 scripted to '3' -> %q, want %q (480:416)", got, want)
	}
}

// TestEX_DefaultsAreUniformZeros pins the INVENTED placeholder convention:
// every menu's default is its printed width in '0' bytes.
//
// doc.go's register entry THE EX MENU VALUES ARE INVENTED says why a uniform
// placeholder rather than a plausible spread, and this test is what keeps a
// later reader from quietly seeding one.
func TestEX_DefaultsAreUniformZeros(t *testing.T) {
	for addr, p5 := range EXDefaults() {
		if p5 == "" || strings.Trim(p5, "0") != "" {
			t.Errorf("menu %s default is %q, want its width in '0' bytes", addr, p5)
		}
	}
}

// TestEX_OutOfInventoryAddressIsRefused. Without it, a fake that answered
// EVERY three-digit address with something plausible would pass the sweep
// above completely.
//
// 061 is the sharp case: one past this chart's printed domain, and a real menu
// on both 590 rows — so a fake that had borrowed a sibling's inventory would
// answer it. doc.go's register entry AN OUT-OF-INVENTORY EX ADDRESS ANSWERS
// "?;" is the assumption this pins.
func TestEX_OutOfInventoryAddressIsRefused(t *testing.T) {
	_, conn := newTestRadio(t)
	for _, tt := range []struct {
		menu int
		why  string
	}{
		{61, "one past this chart's domain, and a real menu on both 590 rows (480:401)"},
		{87, "the TS-590S's last menu"},
		{99, "the TS-590SG's last menu"},
		{999, "the widest three-digit address there is"},
	} {
		t.Run(fmt.Sprintf("%03d", tt.menu), func(t *testing.T) {
			assertRejected(t, conn, exReadFrame(tt.menu))
		})
	}
}

// TestEX_MalformedAndSetShapedBodiesAreRefused.
//
// The SET is the deliberate modelling gap: this book prints one (480:399-406)
// and this fake does not implement it, exactly as core/kw builds no EX Set.
// doc.go's "What this fake deliberately does NOT model" says so, and the
// refusal here is this package's unknown-body path rather than a claim that a
// real TS-480 refuses EX Set.
//
// "EX00000003;" IS THIS BOOK'S OWN LITERAL, and it is printed as an ANSWER
// (480:416) — the Set and the Answer share a wire shape, which is exactly why
// admitting the Set would admit a captured answer being written back. So the
// frame the book prints is refused HERE while the same bytes are ANSWERED by
// the test above, and the difference is the direction.
//
// THE SIBLING FORMS ARE THIS FAMILY'S OWN HAZARD: an FT-891's seven-byte read
// ("EX0101;") and an FTdx10's nine-byte one must not be answered with a
// Kenwood menu value, whatever their digits name.
func TestEX_MalformedAndSetShapedBodiesAreRefused(t *testing.T) {
	_, conn := newTestRadio(t)
	for _, tt := range []struct {
		send string
		why  string
	}{
		{"EX;", "no body at all"},
		{"EX000;", "the address alone, without P2, P3 and P4"},
		{"EX000000;", "one byte short of the printed ten"},
		{"EX00000003;", "a SET: the book's own worked frame, sent the other way (480:416)"},
		{"EX0000000  ;", "a SET carrying spaces"},
		{"EXABC0000;", "a non-digit address"},
		{"EX000 000;", "a space inside the address field"},
		{"EX0000100;", "P2 is not the printed \"00\" (480:402-403)"},
		{"EX0000010;", "P3 is not the printed '0' (480:404-405)"},
		{"EX0000001;", "P4 is not the printed '0' (480:406-407)"},
		{"EX0101;", "an FT-891's seven-byte read frame"},
		{"EX010100;", "an FTdx10's nine-byte read frame"},
	} {
		t.Run(tt.why, func(t *testing.T) {
			assertRejected(t, conn, tt.send)
		})
	}
}

// TestWithEXSetting_OverlaysOneMenu, and leaves every other menu at its
// default: a partial snapshot is what a driver's settings read meets, and an
// option that replaced the whole table would make every such test a fixture
// accident.
func TestWithEXSetting_OverlaysOneMenu(t *testing.T) {
	_, conn := newTestRadio(t, WithEXSetting("032", "12"))
	if got, want := exchange(t, conn, exReadFrame(32)), exAnswerFrame(32, "12"); got != want {
		t.Errorf("the overlaid menu 032 -> %q, want %q", got, want)
	}
	if got, want := exchange(t, conn, exReadFrame(33)), exAnswerFrame(33, "0"); got != want {
		t.Errorf("the neighbouring menu 033 -> %q, want %q — the overlay must not have moved it", got, want)
	}
}

// TestWithEXSetting_IsDeliberatelyLooseAboutTheInventory pins the option's
// looseness, which is internal/fakedx10's and is a decision rather than an
// omission: it does not consult the widths table, so a test can make an
// out-of-inventory address answerable, or script a P5 wider or shorter than
// the chart's printed width, WITHOUT editing the projection of transcription B
// that the cross-check depends on.
//
// Both halves matter to a Kenwood driver test. A19 claims a MAXIMUM and
// nothing else, so core/kw's parser admits a SHORT answer and must be
// exercised with one; and the same parser REFUSES an over-wide answer, which
// can only be driven from a fake willing to send one. Neither is a claim about
// any radio: doc.go's register entry THE EX MENU VALUES ARE INVENTED covers
// what these bytes are.
func TestWithEXSetting_IsDeliberatelyLooseAboutTheInventory(t *testing.T) {
	if _, ok := EXDefaults()["061"]; ok {
		t.Fatal("test fixture error: 061 is in the inventory, so this test proves nothing")
	}
	_, conn := newTestRadio(t,
		WithEXSetting("061", "9"),   // an address this chart does not print
		WithEXSetting("032", "1"),   // shorter than menu 032's printed two
		WithEXSetting("033", "123")) // wider than menu 033's printed one
	for _, tt := range []struct {
		menu int
		p5   string
	}{
		{61, "9"},
		{32, "1"},
		{33, "123"},
	} {
		if got, want := exchange(t, conn, exReadFrame(tt.menu)), exAnswerFrame(tt.menu, tt.p5); got != want {
			t.Errorf("menu %03d -> %q, want %q", tt.menu, got, want)
		}
	}
}

// TestWithEXSetting_RefusesAMalformedAddress. The looseness above is about
// MEMBERSHIP, not about the address's SHAPE: this radio's wire address is
// three ASCII digits (480:401), and an entry keyed "42" or "0042" could never
// be reached by handleEX — it would read as a working overlay that silently
// did nothing. Every call site passes a literal, so it panics.
func TestWithEXSetting_RefusesAMalformedAddress(t *testing.T) {
	for _, addr := range []string{"42", "0042", "", "4 2", "abc"} {
		t.Run(addr, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("New returned instead of panicking")
				}
			}()
			_ = New(WithEXSetting(addr, "0"))
		})
	}
}

// TestWithEXUnavailable_MakesOneMenuAnswerTheRejection is the other half of
// "a partial snapshot is testable": a driver sweeping the whole domain has to
// meet an address that answers "?;" among addresses that answer, and this is
// the option that puts one there without touching any other menu.
func TestWithEXUnavailable_MakesOneMenuAnswerTheRejection(t *testing.T) {
	_, conn := newTestRadio(t, WithEXUnavailable("042"))
	assertRejected(t, conn, exReadFrame(42))
	if got, want := exchange(t, conn, exReadFrame(43)), exAnswerFrame(43, "0"); got != want {
		t.Errorf("the neighbouring menu 043 -> %q, want %q — removing 042 must not have moved it", got, want)
	}
}

// TestWithEXUnavailable_RefusesAMalformedAddress, for handleEX's reason again:
// a delete keyed "42" removes nothing and reads as a working option.
func TestWithEXUnavailable_RefusesAMalformedAddress(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("New returned instead of panicking")
		}
	}()
	_ = New(WithEXUnavailable("42"))
}
