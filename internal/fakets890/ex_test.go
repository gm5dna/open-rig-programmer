// SPDX-License-Identifier: GPL-3.0-or-later

package fakets890

import (
	"strings"
	"testing"
)

// exRead assembles an EX read frame at the positions the chart numbers
// (890:1907), independently of this package's own code: "EX" + P1 + P2P2 +
// P3P3 + ';' — EIGHT bytes, with NO P4.
func exRead(addr string) string { return "EX" + addr + ";" }

// exAnswer assembles the answer the chart draws (890:1909-1913): the read's
// address, P4 as the space the book says a response always carries
// (890:1915), then P5 and the floating terminator.
func exAnswer(addr, p5 string) string { return "EX" + addr + " " + p5 + ";" }

// TestEXRead_AnswersEveryProjectedAddress drives every address the fake's own
// inventory carries over the wire and checks the frame byte for byte against
// the assembler above.
func TestEXRead_AnswersEveryProjectedAddress(t *testing.T) {
	_, conn := newTestRadio(t)
	defaults := EXDefaults()
	if len(defaults) == 0 {
		t.Fatal("the projection is empty — this test would pass vacuously")
	}
	for addr, p5 := range defaults {
		got := exchange(t, conn, exRead(addr))
		if want := exAnswer(addr, p5); got != want {
			t.Fatalf("%s -> %q, want %q", exRead(addr), got, want)
		}
	}
}

// TestEXRead_TheReadFrameIsEightBytesAndCarriesNoP4. The Read row is drawn to
// position 8 with the terminator there and no P4 cell at all (890:1907), where
// the Set and Answer rows splice P4 in at position 8 (890:1900, 890:1910).
func TestEXRead_TheReadFrameIsEightBytesAndCarriesNoP4(t *testing.T) {
	if got := len(exRead("00000")); got != 8 {
		t.Fatalf("the test assembler builds a %d-byte read frame, want 8 (890:1907)", got)
	}
	_, conn := newTestRadio(t)
	// A read with a P4 spliced in is a nine-byte body this handler has no
	// ruler for.
	assertRejected(t, conn, "EX00000 ;")
}

// TestEXRead_RefusesAnAddressTheChartDoesNotPrint — doc.go's register entry AN
// OUT-OF-INVENTORY EX ADDRESS ANSWERS "?;". This is the round trip's negative
// control: without it, a fake that answered EVERY five-digit address with
// something plausible would pass the sweep above completely.
func TestEXRead_RefusesAnAddressTheChartDoesNotPrint(t *testing.T) {
	_, conn := newTestRadio(t)
	defaults := EXDefaults()
	for _, addr := range []string{"09999", "00099", "10099", "19999"} {
		if _, ok := defaults[addr]; ok {
			t.Fatalf("%s is in the projection — it cannot serve as a negative control", addr)
		}
		assertRejected(t, conn, exRead(addr))
	}
}

// TestEXRead_RefusesTheFourExcludedAddresses. The chart prints them with the
// body "Does not correspond to a command" (890:2273-2280), the production
// inventory excludes them by address, and this fake's own projection excludes
// the same four — so they answer through the ordinary out-of-inventory path,
// which is the chart's own "Entering a number that cannot be set also causes
// an error to occur" (890:1910-1911).
func TestEXRead_RefusesTheFourExcludedAddresses(t *testing.T) {
	_, conn := newTestRadio(t)
	for _, addr := range excludedHere {
		assertRejected(t, conn, exRead(addr))
	}
}

// TestEXRead_RefusesAMalformedBody, including a sibling family's own EX read
// frame: core/kw's is TEN bytes with a three-digit menu number and three
// printed constants, and a TS-890S must not answer one whatever its digits
// name.
func TestEXRead_RefusesAMalformedBody(t *testing.T) {
	_, conn := newTestRadio(t)
	for _, send := range []string{
		"EX;",
		"EX0000;",    // four digits: one short of the address
		"EX000000;",  // six
		"EX0000000;", // the 590/480 ten-byte read frame
		"EX2 0000;",  // a menu type outside the two printed values
		"EX20000;",   // likewise, well formed otherwise
		"EX0000x;",
	} {
		assertRejected(t, conn, send)
	}
}

// TestEXRead_ASetShapedBodyIsRefused. THE SET IS A MODELLING GAP, not a claim
// that a real TS-890S refuses EX Set: the book prints one (890:1900-1904), and
// the codec builds none either, because the Set and the Answer share an
// identical wire shape so admitting the Set would admit a captured answer
// being written back. A Set-shaped body therefore falls through handleEX's
// read check to "?;".
func TestEXRead_ASetShapedBodyIsRefused(t *testing.T) {
	_, conn := newTestRadio(t)
	addr := "00000"
	p5 := EXDefaults()[addr]
	for _, p4 := range []string{" ", "9"} {
		assertRejected(t, conn, "EX"+addr+p4+p5+";")
	}
	// And the state is unchanged.
	if got, want := exchange(t, conn, exRead(addr)), exAnswer(addr, p5); got != want {
		t.Errorf("after the refused Sets, %s -> %q, want %q", exRead(addr), got, want)
	}
}

// TestEXDefaults_AreTheInventedPlaceholder — doc.go's register entry THE EX
// MENU VALUES ARE INVENTED. The chart prints each menu's available settings
// and never a shipped default, so there is nothing to source a real one from;
// and `rigprog read --settings --fake` renders these bytes to a user, who must
// not read them as what a TS-890S ships with.
func TestEXDefaults_AreTheInventedPlaceholder(t *testing.T) {
	for addr, p5 := range EXDefaults() {
		if p5 == "" || strings.Trim(p5, "0") != "" {
			t.Errorf("menu %s answers %q, want its width in '0' bytes", addr, p5)
		}
	}
}

// TestWithEXSetting_OverlaysVerbatim, including the two widths A19's ceiling
// reading makes reachable: a SHORT P5, which the codec's parser must admit
// because A19 claims a maximum and nothing else, and an OVER-WIDE one, which
// the same parser must refuse and which can only be driven from a fake willing
// to send it.
func TestWithEXSetting_OverlaysVerbatim(t *testing.T) {
	const addr = "00000"
	for _, p5 := range []string{"0", "000000000000000000"} {
		t.Run(p5, func(t *testing.T) {
			_, conn := newTestRadio(t, WithEXSetting(addr, p5))
			if got, want := exchange(t, conn, exRead(addr)), exAnswer(addr, p5); got != want {
				t.Errorf("%s -> %q, want %q", exRead(addr), got, want)
			}
		})
	}
}

// TestWithEXSetting_IsLooseAboutMembership: an address the chart does not
// print becomes answerable, without editing the projection of transcription B
// that core/transport's cross-check depends on.
func TestWithEXSetting_IsLooseAboutMembership(t *testing.T) {
	const addr = "09999"
	_, conn := newTestRadio(t, WithEXSetting(addr, "123"))
	if got, want := exchange(t, conn, exRead(addr)), exAnswer(addr, "123"); got != want {
		t.Errorf("%s -> %q, want %q", exRead(addr), got, want)
	}
}

// TestWithEXUnavailable_MakesAKnownMenuAnswerLikeAnAbsentOne.
func TestWithEXUnavailable_MakesAKnownMenuAnswerLikeAnAbsentOne(t *testing.T) {
	const addr = "00000"
	if _, ok := EXDefaults()[addr]; !ok {
		t.Fatalf("%s is not in the projection — this test needs a real menu to remove", addr)
	}
	_, conn := newTestRadio(t, WithEXUnavailable(addr))
	assertRejected(t, conn, exRead(addr))
}

// TestEXOptions_RefuseAMalformedAddress. Every call site passes a literal, so
// a key that could never be reached by handleEX is a programming error in a
// fixture and must stop the programme rather than read as a working overlay
// that silently did nothing.
func TestEXOptions_RefuseAMalformedAddress(t *testing.T) {
	for _, addr := range []string{"", "087", "000000", "2000", "20000", "0000x"} {
		t.Run(addr, func(t *testing.T) {
			for _, build := range []struct {
				name string
				fn   func()
			}{
				{"WithEXSetting", func() { WithEXSetting(addr, "000") }},
				{"WithEXUnavailable", func() { WithEXUnavailable(addr) }},
			} {
				func() {
					defer func() {
						if recover() == nil {
							t.Errorf("%s(%q) returned instead of panicking", build.name, addr)
						}
					}()
					build.fn()
				}()
			}
		})
	}
}
