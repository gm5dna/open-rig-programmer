// SPDX-License-Identifier: GPL-3.0-or-later

package fakets590

import (
	"fmt"
	"strings"
	"testing"
)

// EVERY EXPECTED FRAME BELOW IS ASSEMBLED FROM THE BOOK'S OWN POSITION CHART
// rather than by calling buildEXAnswer: "E X P1 P1 P1 P2 P2 P3 P4 P5 … ;",
// with P2 "00" and P3 and P4 '0' (590:542-556). A test that built its
// expectation with the function under test could not catch that function
// putting a byte in the wrong position.

// exAnswerFrame assembles the answer the book prints for menu with raw P5 p5.
func exAnswerFrame(menu int, p5 string) string {
	return fmt.Sprintf("EX%03d00%s%s;", menu, "00", p5)
}

// exReadFrame assembles the ten-byte read the book prints for menu.
func exReadFrame(menu int) string {
	return fmt.Sprintf("EX%03d0000;", menu)
}

// The two rows' printed menu domains, "000 ~ 087: Menu number (TS-590S)" and
// "000 ~ 099: Menu number (TS-590SG)" (590:543-544), written as literals here
// rather than derived from the generated tables: a bound taken from the thing
// it bounds proves nothing.
const (
	lastMenuS  = 87
	lastMenuSG = 99
)

// TestEXDefaults_CoverExactlyThePrintedDomain pins each row's inventory to the
// domain its own book prints, from both ends: every menu from 000 to the
// printed last is present, and nothing beyond it is.
func TestEXDefaults_CoverExactlyThePrintedDomain(t *testing.T) {
	for _, tt := range []struct {
		row  Row
		last int
	}{
		{RowS, lastMenuS},
		{RowSG, lastMenuSG},
	} {
		t.Run(tt.row.String(), func(t *testing.T) {
			got := EXDefaults(tt.row)
			if len(got) != tt.last+1 {
				t.Errorf("EXDefaults(%v) has %d addresses, want %d (000 ~ %03d)", tt.row, len(got), tt.last+1, tt.last)
			}
			for menu := 0; menu <= tt.last; menu++ {
				if _, ok := got[fmt.Sprintf("%03d", menu)]; !ok {
					t.Errorf("menu %03d is absent from EXDefaults(%v)", menu, tt.row)
				}
			}
			if _, ok := got[fmt.Sprintf("%03d", tt.last+1)]; ok {
				t.Errorf("menu %03d is present in EXDefaults(%v), and this row's printed domain stops at %03d (590:543-544)", tt.last+1, tt.row, tt.last)
			}
		})
	}
}

// TestEXDefaults_TheTwoRowsAreTwoTABLES is the 590 pair's own non-borrowing
// pin, at the fake's layer. The book prints TWO parameter lists over COLLIDING
// addresses — the SG's is the S's shifted by two with a read-only version row
// at the top (590:564, 590:744) — so a fake that served one row's inventory to
// the other would answer plausible bytes for the wrong menu on every address.
//
// Menu 000 is where the collision is sharpest: "Display brightness" on the S
// (one digit) against "Version information (4 ASCII characters) read only" on
// the SG. The two widths differ, so the two tables cannot be the same table.
func TestEXDefaults_TheTwoRowsAreTwoTABLES(t *testing.T) {
	s, sg := EXDefaults(RowS), EXDefaults(RowSG)
	if len(s) == len(sg) {
		t.Errorf("both rows' inventories have %d addresses — the S prints 88 menus and the SG 100 (590:543-544)", len(s))
	}
	if got, want := len(s["000"]), 1; got != want {
		t.Errorf("the S's menu 000 default is %d bytes, want %d (Display brightness, 590:569)", got, want)
	}
	if got, want := len(sg["000"]), 4; got != want {
		t.Errorf("the SG's menu 000 default is %d bytes, want %d (Version information, 4 ASCII characters, 590:749)", got, want)
	}
}

// TestEXDefaults_ReturnsAnIndependentCopy: the map is the fake's half of
// core/transport's cross-check and a test-inspection API, so a caller
// mutating what it is handed must not affect the next call's result nor any
// *Radio's stored settings.
func TestEXDefaults_ReturnsAnIndependentCopy(t *testing.T) {
	first := EXDefaults(RowSG)
	first["000"] = "XXXX"
	delete(first, "001")
	second := EXDefaults(RowSG)
	if second["000"] == "XXXX" {
		t.Error("mutating one EXDefaults result changed the next one")
	}
	if _, ok := second["001"]; !ok {
		t.Error("deleting from one EXDefaults result removed the entry from the next one")
	}
}

// TestEXDefaults_RefusesAnUnsetRow. The row is REQUIRED everywhere in this
// package (New's own panic), and a defaulted inventory would be the sharpest
// form of the borrowing this pair must not do.
func TestEXDefaults_RefusesAnUnsetRow(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("EXDefaults(RowUnset) returned instead of panicking")
		}
	}()
	_ = EXDefaults(RowUnset)
}

// TestEX_ReadIsAnsweredAtThePrintedWidth drives every address of both rows
// over the wire and requires each answer to be exactly the frame the book
// prints, carrying that menu's own default.
//
// It is the fake's own end-to-end leg. core/transport's cross-check does the
// same sweep with the CODEC's inventory on the other side, which is what makes
// it a cross-check; this one proves the fake self-consistent, so a failure
// there is unambiguously about the two transcriptions rather than about this
// package's plumbing.
func TestEX_ReadIsAnsweredAtThePrintedWidth(t *testing.T) {
	for _, tt := range []struct {
		row  Row
		last int
	}{
		{RowS, lastMenuS},
		{RowSG, lastMenuSG},
	} {
		t.Run(tt.row.String(), func(t *testing.T) {
			_, conn := newTestRadio(t, tt.row)
			defaults := EXDefaults(tt.row)
			for menu := 0; menu <= tt.last; menu++ {
				p5 := defaults[fmt.Sprintf("%03d", menu)]
				read := exReadFrame(menu)
				if got, want := exchange(t, conn, read), exAnswerFrame(menu, p5); got != want {
					t.Fatalf("%s -> %q, want %q", read, got, want)
				}
			}
		})
	}
}

// TestEX_DefaultsAreUniformZeros pins the INVENTED placeholder convention:
// every menu's default is its printed width in '0' bytes, on every row.
//
// doc.go's register entry THE EX MENU VALUES ARE INVENTED says why a uniform
// placeholder rather than a plausible spread, and this test is what keeps a
// later reader from quietly seeding one.
func TestEX_DefaultsAreUniformZeros(t *testing.T) {
	for _, row := range []Row{RowS, RowSG} {
		for addr, p5 := range EXDefaults(row) {
			if p5 == "" || strings.Trim(p5, "0") != "" {
				t.Errorf("%v menu %s default is %q, want its width in '0' bytes", row, addr, p5)
			}
		}
	}
}

// TestEX_OutOfInventoryAddressIsRefused. Without it, a fake that answered
// EVERY three-digit address with something plausible would pass the sweep
// above completely.
//
// The S's 088 is the sharp case: it is a REAL menu on the SG and past the end
// of the S's own chart, so a fake that served one inventory to both rows would
// answer it. doc.go's register entry AN OUT-OF-INVENTORY EX ADDRESS ANSWERS
// "?;" is the assumption this pins.
func TestEX_OutOfInventoryAddressIsRefused(t *testing.T) {
	for _, tt := range []struct {
		row  Row
		menu int
		why  string
	}{
		{RowS, 88, "a real SG menu, one past the end of the S's own domain (590:543)"},
		{RowS, 100, "past both rows' domains"},
		{RowSG, 100, "one past the end of the SG's domain (590:544)"},
		{RowSG, 999, "the widest three-digit address there is"},
	} {
		t.Run(fmt.Sprintf("%v/%03d", tt.row, tt.menu), func(t *testing.T) {
			_, conn := newTestRadio(t, tt.row)
			assertRejected(t, conn, exReadFrame(tt.menu))
		})
	}
}

// TestEX_MalformedAndSetShapedBodiesAreRefused.
//
// The SET is the deliberate modelling gap: this book prints one (590:542-547)
// and this fake does not implement it, exactly as core/kw builds no EX Set.
// doc.go's "What this fake deliberately does NOT model" says so, and the
// refusal here is this package's unknown-body path rather than a claim that a
// real TS-590 refuses EX Set.
//
// THE SIBLING FORMS ARE THIS FAMILY'S OWN HAZARD: an FT-891's seven-byte read
// ("EX0101;") and an FTdx10's nine-byte one must not be answered with a
// Kenwood menu value, whatever their digits name.
func TestEX_MalformedAndSetShapedBodiesAreRefused(t *testing.T) {
	_, conn := newTestRadio(t, RowSG)
	for _, tt := range []struct {
		send string
		why  string
	}{
		{"EX;", "no body at all"},
		{"EX000;", "the address alone, without P2, P3 and P4"},
		{"EX000000;", "one byte short of the printed ten"},
		{"EX00000001;", "a SET: the read's ten bytes with a P5 payload before the ';'"},
		{"EX0000000    ;", "a SET carrying spaces"},
		{"EXABC0000;", "a non-digit address"},
		{"EX000 000;", "a space inside the address field"},
		{"EX0000100;", "P2 is not the printed \"00\" (590:546-547)"},
		{"EX0000010;", "P3 is not the printed '0' (590:548-550)"},
		{"EX0000001;", "P4 is not the printed '0' (590:551-553)"},
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
	_, conn := newTestRadio(t, RowSG, WithEXSetting("005", "12"))
	if got, want := exchange(t, conn, exReadFrame(5)), exAnswerFrame(5, "12"); got != want {
		t.Errorf("the overlaid menu 005 -> %q, want %q", got, want)
	}
	if got, want := exchange(t, conn, exReadFrame(6)), exAnswerFrame(6, "00"); got != want {
		t.Errorf("the neighbouring menu 006 -> %q, want %q — the overlay must not have moved it", got, want)
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
	if _, ok := EXDefaults(RowS)["088"]; ok {
		t.Fatal("test fixture error: 088 is in the S's inventory, so this test proves nothing")
	}
	_, conn := newTestRadio(t, RowS,
		WithEXSetting("088", "9"),  // an address the S's chart does not print
		WithEXSetting("001", "AB"), // shorter than nothing, wider than one: menu 001 prints one digit
		WithEXSetting("002", "1234"))
	for _, tt := range []struct {
		menu int
		p5   string
	}{
		{88, "9"},
		{1, "AB"},
		{2, "1234"},
	} {
		if got, want := exchange(t, conn, exReadFrame(tt.menu)), exAnswerFrame(tt.menu, tt.p5); got != want {
			t.Errorf("menu %03d -> %q, want %q", tt.menu, got, want)
		}
	}
}

// TestWithEXSetting_RefusesAMalformedAddress. The looseness above is about
// MEMBERSHIP, not about the address's SHAPE: this family's wire address is
// three ASCII digits (590:543-544), and an entry keyed "87" or "0870" could
// never be reached by handleEX — it would read as a working overlay that
// silently did nothing. Every call site passes a literal, so it panics.
func TestWithEXSetting_RefusesAMalformedAddress(t *testing.T) {
	for _, addr := range []string{"87", "0087", "", "8 7", "abc"} {
		t.Run(addr, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("New returned instead of panicking")
				}
			}()
			_ = New(RowSG, WithEXSetting(addr, "0"))
		})
	}
}

// TestWithEXUnavailable_MakesOneMenuAnswerTheRejection is the other half of
// "a partial snapshot is testable": a driver sweeping the whole domain has to
// meet an address that answers "?;" among addresses that answer, and this is
// the option that puts one there without touching any other menu.
func TestWithEXUnavailable_MakesOneMenuAnswerTheRejection(t *testing.T) {
	_, conn := newTestRadio(t, RowS, WithEXUnavailable("042"))
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
	_ = New(RowS, WithEXUnavailable("42"))
}
