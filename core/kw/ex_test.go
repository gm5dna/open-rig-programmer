// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import (
	"strings"
	"testing"
)

// TestBuildEXRead_IsTenBytesAtTheFullThreeDigitAddress. The read chart is
// "E X P1 P1 P1 P2 P2 P3 P4 ;", ten positions on both radios (590:552,
// 480:410), with P2 "00", P3 '0' and P4 '0' (590:546-553, 480:402-407).
func TestBuildEXRead_IsTenBytesAtTheFullThreeDigitAddress(t *testing.T) {
	for _, tc := range []struct {
		l     Layout
		p1    uint8
		frame string
	}{
		{layout590SG(), 0, "EX0000000;"},
		{layout590SG(), 1, "EX0010000;"},
		{layout590S(), 87, "EX0870000;"},
		{layout590SG(), 99, "EX0990000;"},
		{layout480(), 0, "EX0000000;"},
		{layout480(), 32, "EX0320000;"},
		{layout480(), 60, "EX0600000;"},
	} {
		cmd, err := tc.l.BuildEXRead(EXAddress{P1: tc.p1})
		if err != nil {
			t.Errorf("%s: BuildEXRead(%d): %v", tc.l.Model(), tc.p1, err)
			continue
		}
		if got := string(cmd.Bytes()); got != tc.frame {
			t.Errorf("%s: BuildEXRead(%d) built %q, want %q", tc.l.Model(), tc.p1, got, tc.frame)
		}
		if len(cmd.Bytes()) != EXReadLen {
			t.Errorf("%s: BuildEXRead(%d) built %d bytes, want %d", tc.l.Model(), tc.p1, len(cmd.Bytes()), EXReadLen)
		}
	}
}

// TestBuildEXRead_FailsClosedOnAnAddressThatIsNotAKenwoodOne. EXAddress
// carries P2 and P3 because internal/extable's renderer hard-codes all three
// field names; both are zero on every Kenwood row, and Wire() returns "" for
// anything else. A builder that rendered P1 alone from such a value would
// silently discard information the caller believed it had supplied.
func TestBuildEXRead_FailsClosedOnAnAddressThatIsNotAKenwoodOne(t *testing.T) {
	for _, addr := range []EXAddress{{P1: 3, P2: 1}, {P1: 3, P3: 1}, {P1: 3, P2: 9, P3: 9}} {
		if _, err := layout590SG().BuildEXRead(addr); err == nil {
			t.Errorf("BuildEXRead accepted %v, whose P2/P3 are not the AddressSingle form every Kenwood profile registers", addr)
		}
	}
	// The positive control: the same P1 with zero P2 and P3 builds.
	if _, err := layout590SG().BuildEXRead(EXAddress{P1: 3}); err != nil {
		t.Errorf("BuildEXRead refused a well-formed Kenwood address: %v", err)
	}
}

// TestBuildEXRead_BoundsTheDomainAndKnowsNothingOfMEMBERSHIP separates the
// two questions a reader will otherwise run together.
//
// THE PRINTED DOMAIN IS THIS BUILDER'S, because it is one number per row on
// the row's own chart — "000 ~ 087: Menu number (TS-590S)" (590:543), "000 ~
// 099: Menu number (TS-590SG)" (590:544), "000 ~ 060: Menu No." (480:401) —
// carried as the MaxEXAddress axis and read from the same place the outbound
// gate reads it.
//
// MEMBERSHIP IS NOT. Which addresses inside that domain a radio has, what
// each is called and how wide its answer is are the per-radio INVENTORY's
// business — 88 rows on the TS-590S, 100 on the TS-590SG, 61 on the TS-480
// (A26) — and those inventories live in core/kw/ts590 and core/kw/ts480,
// which import this package. A membership test here would be an import
// cycle, and a Layout that carried a copy of an inventory would be a second
// copy of a generated artefact.
func TestBuildEXRead_BoundsTheDomainAndKnowsNothingOfMEMBERSHIP(t *testing.T) {
	// 200 is outside every printed Kenwood menu domain, and every row
	// refuses it — the domain is the row's, so the refusal is too.
	for _, l := range []Layout{layout590S(), layout590SG(), layout480()} {
		if _, err := l.BuildEXRead(EXAddress{P1: 200}); err == nil {
			t.Errorf("%s: BuildEXRead built a read of menu 200, past every domain any of the three books prints", l.Model())
		}
	}
	// 088 is the SG's and not the S's, which is the whole reason the bound
	// is per row rather than per family: the two inventories are one
	// identifier apart in core/kw/ts590.
	if _, err := layout590SG().BuildEXRead(EXAddress{P1: 88}); err != nil {
		t.Errorf("the TS-590SG refused menu 088, inside its printed domain 000 ~ 099 (590:544): %v", err)
	}
	if _, err := layout590S().BuildEXRead(EXAddress{P1: 88}); err == nil {
		t.Error("the TS-590S built a read of menu 088, one past the domain its own book prints, 000 ~ 087 (590:543)")
	}
	if _, err := layout480().BuildEXRead(EXAddress{P1: 61}); err == nil {
		t.Error("the TS-480 built a read of menu 061, one past its printed domain 000 ~ 060 (480:401)")
	}
	// Nothing here claims the radio HAS the addresses it does build: the
	// inventory is never consulted, so an address inside the domain builds
	// whether or not a row for it was ever transcribed.
	if _, err := layout480().BuildEXRead(EXAddress{P1: 60}); err != nil {
		t.Errorf("the TS-480 refused menu 060, the last address its own book prints (480:401): %v", err)
	}
}

// TestParseEXAnswer_IsBoundedByThePerMenuPrintedWidth is A19: "variable
// length" with no printed ceiling (590:555-556, 480:409-411), so the bound
// this parser applies is the width the parameter list prints for THAT menu
// number, supplied by the caller's own inventory row.
func TestParseEXAnswer_IsBoundedByThePerMenuPrintedWidth(t *testing.T) {
	sg := layout590SG()
	item := EXItem{Addr: EXAddress{P1: 2}, Name: "Display brightness", Digits: 1}

	got, err := sg.ParseEXAnswer([]byte("EX00200003;"), item)
	if err != nil {
		t.Fatalf("ParseEXAnswer: %v", err)
	}
	if got != "3" {
		t.Errorf("ParseEXAnswer returned %q, want %q", got, "3")
	}

	// Two digits where the chart prints one: A19's claim is that the answer
	// never EXCEEDS the printed width, so this is the frame the ceiling
	// exists to refuse.
	if _, err := sg.ParseEXAnswer([]byte("EX002000034;"), item); err == nil {
		t.Error("ParseEXAnswer accepted a two-character P5 for a menu whose chart prints one (A19)")
	}

	// The TS-480's own two-digit menus, which is the other half of the same
	// rule read off the other book: "Normally 1-digit for the TS-480. Menu
	// No. 32, 35 and 48 ~ 52 use 2-digit parameters" (480:410-411).
	wide := EXItem{Addr: EXAddress{P1: 32}, Digits: 2}
	if got, err := layout480().ParseEXAnswer([]byte("EX032000005;"), wide); err != nil {
		t.Errorf("ParseEXAnswer refused menu 032's two-character answer: %v", err)
	} else if got != "05" {
		t.Errorf("ParseEXAnswer returned %q, want %q", got, "05")
	}
	// A19 is a CEILING and not an exact width: it claims that an answer
	// never EXCEEDS the printed width and says nothing about a shorter one,
	// so a one-character P5 on that same two-character menu is admitted
	// rather than failing the session on a claim the register does not make.
	if got, err := layout480().ParseEXAnswer([]byte("EX03200005;"), wide); err != nil {
		t.Errorf("ParseEXAnswer refused a one-character P5 on a two-character menu: %v", err)
	} else if got != "5" {
		t.Errorf("ParseEXAnswer returned %q, want %q", got, "5")
	}

	// The 590SG's one free-text row, "up to 8 ASCII characters" (590:750),
	// padded to its full width in the evidence.
	text := EXItem{Addr: EXAddress{P1: 1}, Digits: 8, Text: true}
	if got, err := sg.ParseEXAnswer([]byte("EX0010000MYCALL  ;"), text); err != nil {
		t.Errorf("ParseEXAnswer refused the power-on message row: %v", err)
	} else if got != "MYCALL  " {
		t.Errorf("ParseEXAnswer returned %q, want the field VERBATIM — nothing in either book states a padding rule for P5", got)
	}
}

// TestParseEXAnswer_RequiresTHISReadsOwnAddress is the full-address
// obligation applied to the parser, and it is the same safety property
// PrefixLenMatcher's doc comment states: every one of a radio's menu
// addresses answers with a frame starting "EX", so a parser that did not
// compare the address field would hand a caller a DIFFERENT address's
// setting under the address it asked for.
func TestParseEXAnswer_RequiresTHISReadsOwnAddress(t *testing.T) {
	sg := layout590SG()
	item := EXItem{Addr: EXAddress{P1: 2}, Digits: 1}
	if _, err := sg.ParseEXAnswer([]byte("EX00300003;"), item); err == nil {
		t.Error("ParseEXAnswer accepted menu 003's answer as menu 002's; the whole address is the correlation key")
	} else if !strings.Contains(err.Error(), "003") {
		t.Errorf("the wrong-address refusal reads %v, and it should name the address that answered", err)
	}
	// The positive control.
	if _, err := sg.ParseEXAnswer([]byte("EX00200003;"), item); err != nil {
		t.Errorf("ParseEXAnswer refused its own address's answer: %v", err)
	}
}

// TestParseEXAnswer_RefusesAMalformedFrame. The ten fixed bytes are fixed on
// both radios (590:546-553, 480:402-407), and P5 must carry at least one
// character — an "answer" with an empty P5 is the READ frame, which is not
// an answer to itself.
func TestParseEXAnswer_RefusesAMalformedFrame(t *testing.T) {
	sg := layout590SG()
	item := EXItem{Addr: EXAddress{P1: 2}, Digits: 1}
	for _, tc := range []struct{ frame, why string }{
		{"EX0020000;", "the READ frame: ten bytes, no P5"},
		{"EX002000;", "short of the fixed ten"},
		{"EX00200103;", "P3 is not the printed '0'"},
		{"EX00201003;", "P2 is not the printed \"00\""},
		{"EX00200013;", "P4 is not the printed '0'"},
		{"EX00200003", "no terminator"},
		{"EY00200003;", "the wrong command name"},
		{"EX0A200003;", "a non-digit in the address"},
		{"EX0020000\x003;", "a control byte in P5"},
		{"EX0020000;3;", "an embedded terminator"},
	} {
		if _, err := sg.ParseEXAnswer([]byte(tc.frame), item); err == nil {
			t.Errorf("ParseEXAnswer accepted %q: %s", tc.frame, tc.why)
		}
	}
}

// TestParseEXAnswer_RefusesAnInventoryRowWithNoWidth. A zero Digits is an
// EXItem that was never transcribed; parsing against it would apply a
// ceiling of nothing and admit any answer at all, which is exactly the
// unbounded read A19 exists to prevent.
func TestParseEXAnswer_RefusesAnInventoryRowWithNoWidth(t *testing.T) {
	sg := layout590SG()
	if _, err := sg.ParseEXAnswer([]byte("EX00200003;"), EXItem{Addr: EXAddress{P1: 2}}); err == nil {
		t.Error("ParseEXAnswer accepted an inventory row whose printed width is zero")
	}
	if _, err := sg.ParseEXAnswer([]byte("EX00200003;"), EXItem{Addr: EXAddress{P1: 2}, Digits: MaxEXDigits + 1}); err == nil {
		t.Errorf("ParseEXAnswer accepted an inventory row wider than MaxEXDigits (%d), which describes an answer this family's own accumulator would discard", MaxEXDigits)
	}
}

// TestExReadAddress_AdmitsExactlyWhatTheBuilderCanProduce is the gate's own
// decoder held to the property the gate depends on: an admitted read frame
// is one BuildEXRead could have emitted, and nothing else.
//
// The interesting case is 256 and above. EXAddress.P1 is a uint8, so a plain
// three-digit conversion would fold "EX3000000;" onto address 044 — a frame
// the builder can never produce, admitted by its own gate. The round-trip
// comparison is what refuses it.
//
// THIS DECODER IS DELIBERATELY LAYOUT-FREE and so its domain is the whole of
// what an EXAddress can hold, 000 to 255. The per-row printed domain is the
// GATE's bound, applied on top of this by validEXRead and pinned by
// TestAllowedCommand_BoundsTheEXAddressToTheRowsPrintedMenuDomain, which is
// why the builder loop below stops at the TS-590SG's own ceiling of 099.
func TestExReadAddress_AdmitsExactlyWhatTheBuilderCanProduce(t *testing.T) {
	for _, p1 := range []uint8{0, 1, 87, 99} {
		cmd, err := layout590SG().BuildEXRead(EXAddress{P1: p1})
		if err != nil {
			t.Fatalf("BuildEXRead(%d): %v", p1, err)
		}
		got, err := exReadAddress(cmd.Bytes())
		if err != nil {
			t.Errorf("exReadAddress refused BuildEXRead(%d)'s own frame %q: %v", p1, cmd.Bytes(), err)
			continue
		}
		if got.P1 != p1 || got.P2 != 0 || got.P3 != 0 {
			t.Errorf("exReadAddress(%q) = %v, want P1 %d", cmd.Bytes(), got, p1)
		}
	}

	for _, tc := range []struct{ frame, why string }{
		{"EX3000000;", "address 300 is not representable in a uint8 P1, so no builder could have emitted this"},
		{"EX2560000;", "one past the widest address this package holds"},
		{"EX9990000;", "the widest three-digit field, and not an address"},
		{"EX0020000", "no terminator"},
		{"EX00200000;", "eleven bytes: the answer shape, not the read"},
		{"EX002000;", "nine bytes"},
		{"EX0020100;", "P3 is not the printed '0'"},
		{"EX0021000;", "P2 is not the printed \"00\""},
		{"EX0020001;", "P4 is not the printed '0'"},
		{"EY0020000;", "the wrong command name"},
		{"EX00A0000;", "a non-digit in the address"},
	} {
		if _, err := exReadAddress([]byte(tc.frame)); err == nil {
			t.Errorf("exReadAddress accepted %q: %s", tc.frame, tc.why)
		}
	}
}

// TestEX_ZeroLayoutBuildsAndParsesNothing.
func TestEX_ZeroLayoutBuildsAndParsesNothing(t *testing.T) {
	var l Layout
	if _, err := l.BuildEXRead(EXAddress{P1: 2}); err == nil {
		t.Error("a zero Layout built an EX read")
	}
	if _, err := l.ParseEXAnswer([]byte("EX00200003;"), EXItem{Addr: EXAddress{P1: 2}, Digits: 1}); err == nil {
		t.Error("a zero Layout parsed an EX answer")
	}
}
