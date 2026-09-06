// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import (
	"errors"
	"strings"
	"testing"
)

// TestBuildIDRead_IsTheThreeByteFrameBothBooksPrint. "I D ;" is the whole
// Read chart on both radios (590:1115, 480:683), so this builder has one
// output and the test is a literal comparison rather than a shape check.
func TestBuildIDRead_IsTheThreeByteFrameBothBooksPrint(t *testing.T) {
	for _, l := range []Layout{layout590SG(), layout590S(), layout480()} {
		cmd, err := l.BuildIDRead()
		if err != nil {
			t.Fatalf("%s: BuildIDRead: %v", l.Model(), err)
		}
		if got := string(cmd.Bytes()); got != "ID;" {
			t.Errorf("%s: BuildIDRead built %q, want %q (590:1115, 480:683)", l.Model(), got, "ID;")
		}
		if len(cmd.Bytes()) != IDReadLen {
			t.Errorf("%s: BuildIDRead built %d bytes, want %d", l.Model(), len(cmd.Bytes()), IDReadLen)
		}
	}
}

// TestParseIDAnswer_IsSixBytesAndThreeDigits is the positive half of the
// negative pin below: a Kenwood ID answer is "I D P1 P1 P1 ;" (590:1119,
// 480:687), and the three printed values are 021, 023 and 020 (590:1114,
// 590:1116, 480:678).
func TestParseIDAnswer_IsSixBytesAndThreeDigits(t *testing.T) {
	cases := []struct {
		frame string
		want  string
	}{
		{"ID021;", "021"}, // TS-590S  (590:1114)
		{"ID023;", "023"}, // TS-590SG (590:1116)
		{"ID020;", "020"}, // TS-480   (480:678)
		// The sibling table's two values parse as FRAMES; which radio a
		// token names is the driver's question, not this parser's.
		{"ID022;", "022"},
		{"ID024;", "024"},
	}
	for _, tc := range cases {
		got, err := layout590SG().ParseIDAnswer([]byte(tc.frame))
		if err != nil {
			t.Errorf("ParseIDAnswer(%q): %v", tc.frame, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseIDAnswer(%q) = %q, want %q", tc.frame, got, tc.want)
		}
	}
}

// TestParseIDAnswer_RefusesTheYaesuSevenByteFourDigitAnswer is the plan's
// negative pin, written in the shape the spec insists on: a Kenwood answer
// is SIX bytes with THREE digits and a Yaesu one is SEVEN with FOUR
// (core/cat's idAnswerLen = 7), so "ID0800;" is a real frame of the other
// family and this parser must refuse it. "A four-byte ID answer", which an
// earlier draft asked for, describes no frame either family sends.
func TestParseIDAnswer_RefusesTheYaesuSevenByteFourDigitAnswer(t *testing.T) {
	for _, frame := range []string{
		"ID0800;", // the FT-710's own answer: seven bytes, four digits
		"ID0570;", // the FT-891's
		"ID02;",   // five bytes
		"ID21;",   // two digits, five bytes
		"IDABC;",  // six bytes, not digits
		"ID 21;",  // a space is MC's convention, not this field's
		"XX021;",  // the right shape, the wrong command
		"ID021",   // no terminator
	} {
		if _, err := layout590SG().ParseIDAnswer([]byte(frame)); err == nil {
			t.Errorf("ParseIDAnswer accepted %q, which is not the 6-byte, three-digit answer both books print (590:1119, 480:687)", frame)
		} else if !errors.Is(err, ErrParse) {
			t.Errorf("ParseIDAnswer(%q) returned %v, want a member of the ErrParse family", frame, err)
		}
	}
}

// TestBuildFVRead_IsThe590sAloneAndTY_IsThe480s pins the two identity
// frames that are NOT common, as a fact about two books at once: FV appears
// nowhere in the 2003 TS-480 document and TY appears nowhere in the
// TS-590S/SG one, so a layout that built the wrong one would emit a frame
// its radio's book does not describe.
func TestBuildFVRead_IsThe590sAloneAndTY_IsThe480s(t *testing.T) {
	sg, s, t480 := layout590SG(), layout590S(), layout480()

	for _, l := range []Layout{sg, s} {
		cmd, err := l.BuildFVRead()
		if err != nil {
			t.Fatalf("%s: BuildFVRead: %v", l.Model(), err)
		}
		if got := string(cmd.Bytes()); got != "FV;" {
			t.Errorf("%s: BuildFVRead built %q, want %q (590:1034)", l.Model(), got, "FV;")
		}
		if _, err := l.BuildTYRead(); err == nil {
			t.Errorf("%s: BuildTYRead built a frame, and TY appears nowhere in the TS-590S/SG document", l.Model())
		}
	}

	cmd, err := t480.BuildTYRead()
	if err != nil {
		t.Fatalf("%s: BuildTYRead: %v", t480.Model(), err)
	}
	if got := string(cmd.Bytes()); got != "TY;" {
		t.Errorf("%s: BuildTYRead built %q, want %q (480:1630)", t480.Model(), got, "TY;")
	}
	if _, err := t480.BuildFVRead(); err == nil {
		t.Errorf("%s: BuildFVRead built a frame, and FV appears nowhere in the 2003 TS-480 document", t480.Model())
	}
}

// TestParseFVAnswer_PinsTheWidthAndNotTheGrammar is A13 written as a test:
// the answer chart gives four characters (590:1037) and the only format
// statement is one worked example, "for firmware version 1.00, it reads
// FV1.00;" (590:1035). So the width is refused when wrong and the STRING is
// returned verbatim — a parser that demanded digit-dot-digit-digit would be
// asserting a grammar the book does not print.
func TestParseFVAnswer_PinsTheWidthAndNotTheGrammar(t *testing.T) {
	for _, tc := range []struct{ frame, want string }{
		{"FV1.00;", "1.00"},
		{"FV2.00;", "2.00"},
		{"FV1.08;", "1.08"},
		// Four characters that are not the worked example's shape. A13
		// claims the width and explicitly not the grammar, so this parses.
		{"FVABCD;", "ABCD"},
		{"FV    ;", "    "},
	} {
		got, err := layout590S().ParseFVAnswer([]byte(tc.frame))
		if err != nil {
			t.Errorf("ParseFVAnswer(%q): %v", tc.frame, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseFVAnswer(%q) = %q, want %q", tc.frame, got, tc.want)
		}
	}

	for _, frame := range []string{
		"FV1.0;",     // six bytes
		"FV1.000;",   // eight bytes
		"FV;",        // the READ frame, which is not an answer
		"FV1.0\x00;", // a control byte inside the field
		"FV1;0.;",    // an embedded terminator
	} {
		if _, err := layout590S().ParseFVAnswer([]byte(frame)); err == nil {
			t.Errorf("ParseFVAnswer accepted %q; the chart prints seven bytes, four characters (590:1037)", frame)
		}
	}

	// The 480 has no FV at all, so its layout parses none.
	if _, err := layout480().ParseFVAnswer([]byte("FV1.00;")); err == nil {
		t.Error("the TS-480 layout parsed an FV answer, and FV appears nowhere in its book")
	}
}

// TestParseTYAnswer_IsDecision4sGrammarExactly. Six bytes; prefix "TY";
// terminator at byte 6; P1's TWO bytes accepted OPAQUELY — anything that is
// neither a control code 00-1Fh nor ';', which is the 480's own general
// interior-byte rule (480:127-129) and the only rule it gives for a field
// printed "Reserved" (480:1623); P2 accepted only in '0'..'3'
// (480:1626-1629).
func TestParseTYAnswer_IsDecision4sGrammarExactly(t *testing.T) {
	l := layout480()

	for _, tc := range []struct {
		frame    string
		reserved string
		variant  byte
		name     string
	}{
		{"TY000;", "00", '0', "TS-480HX (200 W)"},
		{"TY001;", "00", '1', "TS-480SAT (100 W + AT)"},
		{"TY002;", "00", '2', "Japanese 50 W type"},
		{"TY003;", "00", '3', "Japanese 20 W type"},
		// P1 IS OPAQUE. These two bytes are printed "Reserved" and this
		// parser makes no claim about them; refusing a legitimate radio
		// over a guessed digit predicate is what decision 4 forbids.
		{"TYZ~1;", "Z~", '1', "TS-480SAT (100 W + AT)"},
		{"TY  2;", "  ", '2', "Japanese 50 W type"},
		{"TY\x7f\xff3;", "\x7f\xff", '3', "Japanese 20 W type"},
	} {
		got, err := l.ParseTYAnswer([]byte(tc.frame))
		if err != nil {
			t.Errorf("ParseTYAnswer(%q): %v", tc.frame, err)
			continue
		}
		if got.Reserved != tc.reserved || got.Variant != tc.variant {
			t.Errorf("ParseTYAnswer(%q) = %+v, want Reserved %q Variant %q", tc.frame, got, tc.reserved, tc.variant)
		}
		if got.VariantName() != tc.name {
			t.Errorf("ParseTYAnswer(%q).VariantName() = %q, want %q (480:1626-1629)", tc.frame, got.VariantName(), tc.name)
		}
	}

	for _, tc := range []struct{ frame, why string }{
		{"TY004;", "P2 '4' names a fifth variant nobody has read about — decision 4 refuses rather than reporting it opaquely or defaulting to one of the four"},
		{"TY00;", "five bytes"},
		{"TY0004;", "seven bytes"},
		{"TY\x0012;", "a control code in P1, which 480:127-129 forbids"},
		{"TY;03;", "an embedded terminator in P1"},
		{"XY003;", "the wrong command name"},
		{"TY003!", "no terminator"},
	} {
		if _, err := l.ParseTYAnswer([]byte(tc.frame)); err == nil {
			t.Errorf("ParseTYAnswer accepted %q: %s", tc.frame, tc.why)
		}
	}

	// The 590 pair have no TY at all.
	if _, err := layout590SG().ParseTYAnswer([]byte("TY001;")); err == nil {
		t.Error("the TS-590SG layout parsed a TY answer, and TY appears nowhere in its book")
	}
}

// TestIdentity_ZeroLayoutBuildsAndParsesNothing. A zero Layout speaks for no
// radio, so it may not emit "ID;" — the one identity frame whose bytes vary
// with nothing — and may not attribute a byte of any answer to a meaning
// nobody declared.
func TestIdentity_ZeroLayoutBuildsAndParsesNothing(t *testing.T) {
	var l Layout
	if _, err := l.BuildIDRead(); err == nil {
		t.Error("a zero Layout built an ID read")
	}
	if _, err := l.BuildFVRead(); err == nil {
		t.Error("a zero Layout built an FV read")
	}
	if _, err := l.BuildTYRead(); err == nil {
		t.Error("a zero Layout built a TY read")
	}
	if _, err := l.ParseIDAnswer([]byte("ID021;")); err == nil {
		t.Error("a zero Layout parsed an ID answer")
	}
	if _, err := l.ParseFVAnswer([]byte("FV1.00;")); err == nil {
		t.Error("a zero Layout parsed an FV answer")
	}
	if _, err := l.ParseTYAnswer([]byte("TY001;")); err == nil {
		t.Error("a zero Layout parsed a TY answer")
	}
}

// TestTYAnswer_VariantNameCoversTheFourPrintedVariantsAndNothingElse. The
// four names are the 480's own words (480:1626-1629); a value the parser
// refuses has no name to report, and VariantName says so rather than
// inventing one.
func TestTYAnswer_VariantNameCoversTheFourPrintedVariantsAndNothingElse(t *testing.T) {
	if got := (TYAnswer{Variant: '4'}).VariantName(); got == "" || !strings.Contains(got, "'4'") {
		t.Errorf("TYAnswer{Variant:'4'}.VariantName() = %q, want a string naming the unprinted byte rather than one of the four variants", got)
	}
	seen := map[string]bool{}
	for _, v := range []byte{'0', '1', '2', '3'} {
		name := (TYAnswer{Variant: v}).VariantName()
		if seen[name] {
			t.Errorf("variant %q repeats the name %q", v, name)
		}
		seen[name] = true
	}
}
