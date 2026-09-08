// SPDX-License-Identifier: GPL-3.0-or-later

package ma

import (
	"errors"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/kw"
)

// fixtureItem is the one inventory row the EX tests work against: every
// positive EX leg here runs against a layout minted with this single row, so
// what these tests pin is the envelope and the membership machinery, not
// either radio's chart. Keeping the fixture out of the real inventories is
// what stops a chart edit from silently changing what an envelope test
// asserts — and it is why these frames stay legible as literals.
//
// Its address, 0/03/01, IS printed by both books. That is deliberate and it
// is not a claim: the fixture's Name and Digits are this file's, not the
// chart's, and no test here reads a real inventory THROUGH fixtureItem. The
// real inventories are asked their own questions, by address, in
// TestBuildEXRead_IsEightBytesAndAsksTheInventory.
var fixtureItem = kw.EXItem{Addr: kw.EXAddress{P1: 0, P2: 3, P3: 1}, Name: "fixture", Digits: 3}

// testLayout990WithEXItems is testLayoutWithEXItems for the other row: the
// 990S's EX Answer grid is printed FIXED and the 890S's floats, so both
// forms need a layout carrying the fixture row.
func testLayout990WithEXItems(t *testing.T, items []kw.EXItem) Layout {
	t.Helper()
	cfg := layout990Config()
	cfg.EXItems = items
	l, err := newLayout(cfg)
	if err != nil {
		t.Fatalf("newLayout: %v", err)
	}
	return l
}

// TestEnvelopeBuilders_BuildExactlyTheFramesBothBooksPrint pins all four
// fixed-frame builders on both rows. Each frame is wholly printed — no
// parameter field a caller can vary — so each builder has exactly one output
// on every row.
func TestEnvelopeBuilders_BuildExactlyTheFramesBothBooksPrint(t *testing.T) {
	builders := []struct {
		name  string
		build func(Layout) (Command, error)
		want  string
	}{
		{"ID read", Layout.BuildIDRead, "ID;"},
		{"AI read", Layout.BuildAIRead, "AI;"},
		{"AI set off", Layout.BuildAISetOff, "AI0;"},
		{"FV read", Layout.BuildFVRead, "FV;"},
	}
	for _, l := range []Layout{Layout890(), Layout990()} {
		for _, b := range builders {
			cmd, err := b.build(l)
			if err != nil {
				t.Errorf("%s: %s: %v", l.Model(), b.name, err)
				continue
			}
			if got := string(cmd.Bytes()); got != b.want {
				t.Errorf("%s: %s built %q, want %q", l.Model(), b.name, got, b.want)
			}
		}
	}
}

// TestEnvelopeBuilders_RefuseAnUnconfiguredLayout is the zero value's own
// pin: a package function would emit "ID;" on behalf of no radio at all,
// which is why every builder takes a Layout receiver even where the frame
// does not vary.
func TestEnvelopeBuilders_RefuseAnUnconfiguredLayout(t *testing.T) {
	var zero Layout
	builders := map[string]func(Layout) (Command, error){
		"ID read":    Layout.BuildIDRead,
		"AI read":    Layout.BuildAIRead,
		"AI set off": Layout.BuildAISetOff,
		"FV read":    Layout.BuildFVRead,
	}
	for name, build := range builders {
		cmd, err := build(zero)
		if err == nil {
			t.Errorf("%s: the zero Layout built %q", name, cmd.Bytes())
		}
		if !cmd.IsZero() {
			t.Errorf("%s: the zero Layout returned a non-zero Command alongside its error", name)
		}
	}
	if _, err := zero.BuildEXRead(fixtureItem.Addr); err == nil {
		t.Error("EX read: the zero Layout built a frame")
	}
	if _, err := zero.ParseIDAnswer([]byte("ID024;")); err == nil {
		t.Error("ID answer: the zero Layout parsed a frame")
	}
	if _, err := zero.ParseFVAnswer([]byte("FV1.00;")); err == nil {
		t.Error("FV answer: the zero Layout parsed a frame")
	}
	if _, err := zero.ParseEXAnswer([]byte("EX00301 005;"), fixtureItem); err == nil {
		t.Error("EX answer: the zero Layout parsed a frame")
	}
}

// TestBuildAISetOff_IsTheFrameTheSessionACTUALLYWrites is the anti-drift pin,
// and it is the reason this builder exists at all.
//
// core/kw's framing writes its own init frame at open; this package spells
// "AI0;" a second time because kw's constant is unexported. A session that
// had disabled Auto Information with one spelling and re-disabled it with
// another would still work — until the day one of the two was edited, when
// the failure is a radio pushing unsolicited frames into a prefix-matched
// session. Comparing this builder's output against the framing's own
// InitSequence is what makes the two one datum in practice.
func TestBuildAISetOff_IsTheFrameTheSessionACTUALLYWrites(t *testing.T) {
	for _, l := range []Layout{Layout890(), Layout990()} {
		f, err := NewFramingFor(l)
		if err != nil {
			t.Fatalf("%s: NewFramingFor: %v", l.Model(), err)
		}
		init := f.InitSequence()
		if len(init) != 1 {
			t.Fatalf("%s: InitSequence has %d frames, want exactly 1", l.Model(), len(init))
		}
		cmd, err := l.BuildAISetOff()
		if err != nil {
			t.Fatalf("%s: BuildAISetOff: %v", l.Model(), err)
		}
		if got, want := string(cmd.Bytes()), string(init[0].Bytes()); got != want {
			t.Errorf("%s: BuildAISetOff built %q and the session opens with %q — the two must be the same frame", l.Model(), got, want)
		}
	}
}

// TestAI_NoOtherStateIsEverBuilt asserts the SURFACE rather than a refusal
// inside it: there is no parameter to pass a 2 or a 4 to. Both books print
// "1: Not used" and "3: Not used" and give 2 and 4 to AI ON (890:175-181,
// 990:173-178), and an AI-ON radio pushes a response per changed parameter
// into a session that correlates answers by prefix and length.
func TestAI_NoOtherStateIsEverBuilt(t *testing.T) {
	// The compiler is the assertion: BuildAISetOff takes no argument. If a
	// state parameter is ever added, this line stops compiling.
	var _ func(Layout) (Command, error) = Layout.BuildAISetOff
}

// TestParseIDAnswer_ReturnsTheTokenAndClassifiesNothing pins the division of
// labour: which model a token names is the DRIVER's question, asked once at
// probe against its own row's CAT ID.
func TestParseIDAnswer_ReturnsTheTokenAndClassifiesNothing(t *testing.T) {
	for _, l := range []Layout{Layout890(), Layout990()} {
		// A row parses the OTHER row's token perfectly well; refusing it is
		// the probe's job, not the codec's.
		for _, token := range []string{"024", "022", "020"} {
			got, err := l.ParseIDAnswer([]byte("ID" + token + ";"))
			if err != nil {
				t.Errorf("%s: ParseIDAnswer(%q): %v", l.Model(), token, err)
				continue
			}
			if got != token {
				t.Errorf("%s: ParseIDAnswer returned %q, want %q", l.Model(), got, token)
			}
		}
	}
}

// TestParseIDAnswer_RefusesWhatIsNotThePrintedShape covers the four ways a
// six-byte three-digit answer can be wrong, in checkAnswerShape's fixed
// order: length, prefix, terminator, then the field.
func TestParseIDAnswer_RefusesWhatIsNotThePrintedShape(t *testing.T) {
	tests := []struct {
		name  string
		frame string
	}{
		{"too short", "ID24;"},
		{"too long", "ID0244;"},
		{"wrong prefix", "IF024;"},
		{"no terminator", "ID0240"},
		{"a non-digit in P1", "IDO24;"},
		// A Kenwood answer is 6 bytes with 3 digits where a Yaesu one is 7
		// with 4. The negative is written against a REAL Yaesu frame,
		// because "a four-byte ID answer" describes no frame either family
		// sends.
		{"the Yaesu seven-byte four-digit answer", "ID0761;"},
	}
	l := Layout890()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := l.ParseIDAnswer([]byte(tt.frame))
			if err == nil {
				t.Fatalf("ParseIDAnswer(%q) = %q, want an error", tt.frame, got)
			}
			if !errors.Is(err, kw.ErrParse) {
				t.Errorf("ParseIDAnswer(%q) returned %v, want an error wrapping kw.ErrParse", tt.frame, err)
			}
		})
	}
}

// TestParseFVAnswer_PinsTheWidthAndNotTheGrammar is A13. The answer chart
// gives four positions and the only format statement anywhere is one worked
// example, "FV1.00;" (890:2657, 990:2533), so a four-character answer is
// accepted whatever those characters are. A parser demanding
// digit-dot-digit-digit would assert a grammar the document does not print.
//
// BOTH BOOKS PRINT FV, WHICH IS THE DIFFERENCE FROM core/kw. There,
// ParseFVAnswer refuses Book480 because that document has no FV command at
// all; here neither row refuses the other's book, because both charts are
// printed identically (890:2650-2659, 990:2527-2536).
func TestParseFVAnswer_PinsTheWidthAndNotTheGrammar(t *testing.T) {
	for _, l := range []Layout{Layout890(), Layout990()} {
		for _, ver := range []string{"1.00", "2.13", "ABCD", "0000"} {
			got, err := l.ParseFVAnswer([]byte("FV" + ver + ";"))
			if err != nil {
				t.Errorf("%s: ParseFVAnswer(%q): %v", l.Model(), ver, err)
				continue
			}
			if got != ver {
				t.Errorf("%s: ParseFVAnswer returned %q, want %q verbatim", l.Model(), got, ver)
			}
		}
		for _, bad := range []string{"FV1.0;", "FV10.00;", "FV1.0\x01;", "FX1.00;", "FV1.00"} {
			if got, err := l.ParseFVAnswer([]byte(bad)); err == nil {
				t.Errorf("%s: ParseFVAnswer(%q) = %q, want an error", l.Model(), bad, got)
			}
		}
	}
}

// TestWireEXAddress_IsTheFiveCharacterGroupedForm is the whole of §"CANNOT
// reuse" item 6: kw.EXAddress.Wire renders THREE digits from P1 alone, which
// is right for the MR/MW rows and wrong here. This family's address is a
// (P1,P2,P3) triple of one, two and two digits.
func TestWireEXAddress_IsTheFiveCharacterGroupedForm(t *testing.T) {
	l := Layout890()
	tests := []struct {
		addr kw.EXAddress
		want string
	}{
		{kw.EXAddress{P1: 0, P2: 0, P3: 0}, "00000"},
		{kw.EXAddress{P1: 0, P2: 3, P3: 1}, "00301"},
		{kw.EXAddress{P1: 1, P2: 0, P3: 26}, "10026"},
		{kw.EXAddress{P1: 0, P2: 99, P3: 99}, "09999"},
	}
	for _, tt := range tests {
		if got := l.WireEXAddress(tt.addr); got != tt.want {
			t.Errorf("WireEXAddress(%v) = %q, want %q", tt.addr, got, tt.want)
		}
		if len(tt.want) != 5 {
			t.Fatalf("the fixture %q is not five characters — the test is wrong", tt.want)
		}
	}
	// It FAILS CLOSED on a value no chart prints, rather than rendering a
	// wider field or discarding a component: P1 is a menu-type flag with
	// two values and P2/P3 are two digits each.
	for _, addr := range []kw.EXAddress{
		{P1: 2},
		{P1: 9},
		{P1: 0, P2: 100},
		{P1: 0, P2: 0, P3: 255},
	} {
		if got := l.WireEXAddress(addr); got != "" {
			t.Errorf("WireEXAddress(%v) = %q, want \"\" — that is not an address either chart prints", addr, got)
		}
	}
	// kw.EXAddress.Wire is still the OTHER rows' rendering and is untouched.
	if got := (kw.EXAddress{P1: 0, P2: 3, P3: 1}).Wire(); got != "" {
		t.Errorf("kw.EXAddress.Wire on a grouped address = %q, want \"\" — it fails closed on a non-zero P2 or P3", got)
	}
}

// TestBuildEXRead_IsEightBytesAndAsksTheInventory pins the read frame and the
// membership check together. Both charts are SPARSE, so an address inside any
// bound at all may still be one the radio answers with an error.
func TestBuildEXRead_IsEightBytesAndAsksTheInventory(t *testing.T) {
	l := testLayoutWithEXItems(t, []kw.EXItem{fixtureItem})

	cmd, err := l.BuildEXRead(fixtureItem.Addr)
	if err != nil {
		t.Fatalf("BuildEXRead: %v", err)
	}
	if got, want := string(cmd.Bytes()), "EX00301;"; got != want {
		t.Errorf("BuildEXRead built %q, want %q", got, want)
	}
	if got := len(cmd.Bytes()); got != EXReadLen {
		t.Errorf("BuildEXRead built %d bytes, want %d", got, EXReadLen)
	}

	for _, miss := range []kw.EXAddress{
		{P1: 1, P2: 3, P3: 1},
		{P1: 0, P2: 4, P3: 1},
		{P1: 0, P2: 3, P3: 2},
		{P1: 2, P2: 3, P3: 1},
	} {
		if cmd, err := l.BuildEXRead(miss); err == nil {
			t.Errorf("BuildEXRead(%v) built %q — that address is not in this row's inventory", miss, cmd.Bytes())
		}
	}

	// And the same question asked of the REAL 890S layout, both ways round,
	// because a membership set that answered everything and one that
	// answered nothing would both satisfy a one-sided pin.
	//
	// This arm used to read "a real row, whose inventory is bootstrap-empty,
	// builds nothing at all", which is what it could say while T6's
	// placeholder inventory was still in the tree. It cannot say it now:
	// 0/03/01 is a row the TS-890S book actually prints (menu890s.csv), so
	// the transcription landing turned that assertion from a membership pin
	// into a falsehood about the chart.
	if _, err := Layout890().BuildEXRead(kw.EXAddress{P1: 0, P2: 3, P3: 1}); err != nil {
		t.Errorf("the TS-890S layout refused 0/03/01, a row its own chart prints: %v", err)
	}
	// 0/09/09 is in NEITHER book's chart — category 09 stops at item 03 on
	// both — so no address here is admitted by a bound alone. That is the
	// property the sparse charts need: an address inside every printed
	// domain the book states can still be one the radio answers with an
	// error.
	if cmd, err := Layout890().BuildEXRead(kw.EXAddress{P1: 0, P2: 9, P3: 9}); err == nil {
		t.Errorf("the TS-890S layout admitted %q — 0/09/09 is not a row of its chart", cmd.Bytes())
	}
}

// TestParseEXAnswer_ReturnsP5Verbatim pins the 890S's floating-terminator
// form, the whole-address correlation and the P4 space.
func TestParseEXAnswer_ReturnsP5Verbatim(t *testing.T) {
	l := testLayoutWithEXItems(t, []kw.EXItem{fixtureItem})

	for _, tt := range []struct {
		frame string
		want  string
	}{
		{"EX00301 005;", "005"},
		{"EX00301 5;", "5"},
		// P5 may be zero characters: the chart's own width classes run
		// from 0 (890:1920-1921), and the frame is then nine bytes.
		{"EX00301 ;", ""},
	} {
		got, err := l.ParseEXAnswer([]byte(tt.frame), fixtureItem)
		if err != nil {
			t.Errorf("ParseEXAnswer(%q): %v", tt.frame, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseEXAnswer(%q) = %q, want %q", tt.frame, got, tt.want)
		}
	}

	// P5 IS RETURNED VERBATIM, TRAILING SPACES INCLUDED. Neither book states
	// a padding rule for P5 — A1's rule is the MA0 name field's and is
	// scoped to it — so trimming here would apply an assumption to a field
	// the register does not cover.
	got, err := l.ParseEXAnswer([]byte("EX00301 5  ;"), fixtureItem)
	if err != nil {
		t.Fatalf("ParseEXAnswer: %v", err)
	}
	if got != "5  " {
		t.Errorf("ParseEXAnswer returned %q, want %q — P5 is verbatim, and no book prints a pad rule for it", got, "5  ")
	}
}

// TestParseEXAnswer_RefusesWhatTheChartDoesNotPrint covers each structural
// rule with its own frame.
func TestParseEXAnswer_RefusesWhatTheChartDoesNotPrint(t *testing.T) {
	l := testLayoutWithEXItems(t, []kw.EXItem{fixtureItem})

	tests := []struct {
		name  string
		frame string
		item  kw.EXItem
	}{
		{"the READ frame is not an answer to itself", "EX00301;", fixtureItem},
		{"wrong prefix", "EY00301 005;", fixtureItem},
		{"no terminator", "EX00301 005", fixtureItem},
		{"a non-digit in the address", "EX0O301 005;", fixtureItem},
		{"another menu's answer", "EX00401 005;", fixtureItem},
		{"P4 is not the printed space", "EX003019005;", fixtureItem},
		{"P5 wider than the row's printed width", "EX00301 0005;", fixtureItem},
		{"an untranscribed inventory row", "EX00301 005;", kw.EXItem{Addr: fixtureItem.Addr}},
		{"an inventory row wider than the family's frame bound", "EX00301 005;", kw.EXItem{Addr: fixtureItem.Addr, Digits: kw.MaxEXDigits + 1}},
		{"an address this row's inventory does not carry", "EX00401 005;", kw.EXItem{Addr: kw.EXAddress{P1: 0, P2: 4, P3: 1}, Digits: 3}},
		{"a control byte in P5", "EX00301 0\x010;", fixtureItem},
		{"an embedded terminator in P5", "EX00301 0;0;", fixtureItem},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := l.ParseEXAnswer([]byte(tt.frame), tt.item)
			if err == nil {
				t.Fatalf("ParseEXAnswer(%q) = %q, want an error", tt.frame, got)
			}
			if !errors.Is(err, kw.ErrParse) {
				t.Errorf("ParseEXAnswer(%q) returned %v, want an error wrapping kw.ErrParse", tt.frame, err)
			}
		})
	}
}

// TestParseEXAnswer_The990SAnswerIsPrintedFIXEDAndTheS890SFloats is ERRATUM
// E19, pinned both ways: the one place the two books' EX charts genuinely
// differ, and a finding read off the printed rulers rather than off the prose.
//
// The 890S draws its EX Set and Answer with a floating terminator — the ruler
// reads "9~" and the terminator's own header cell is the letter "x", never a
// number (890:1898-1904 Set, 890:1909-1913 Answer) — so its answer is 9 bytes
// plus P5. The 990S draws BOTH to position 24: the ruler holds P5 to exactly
// fifteen bytes at positions 9-23 and nails ';' to 24 (990:1719-1732 Set,
// 990:1738-1747 Answer). Yet the SAME chart's own P5 note is the
// variable-length one both books print (990:1742-1756), so the diagram and the
// note beside it disagree.
//
// SO BOTH FORMS ARE ADMITTED ON THE 990S — the printed fixed 24 with P5 padded
// to its window, and a shorter unpadded frame carrying the row's own printed
// width — and a length between the two, which neither reading produces, is
// refused. A parser built to the diagram alone would refuse or mis-scan every
// item class narrower than the widest.
func TestParseEXAnswer_The990SAnswerIsPrintedFIXEDAndTheS890SFloats(t *testing.T) {
	l := testLayout990WithEXItems(t, []kw.EXItem{fixtureItem})

	padded := "EX00301 " + "005" + strings.Repeat(" ", 12) + ";"
	if len(padded) != 24 {
		t.Fatalf("the fixture is %d bytes, want the printed 24 — the test is wrong", len(padded))
	}
	got, err := l.ParseEXAnswer([]byte(padded), fixtureItem)
	if err != nil {
		t.Fatalf("ParseEXAnswer on the printed fixed form: %v", err)
	}
	if want := "005" + strings.Repeat(" ", 12); got != want {
		t.Errorf("ParseEXAnswer on the fixed form = %q, want %q — P5 is verbatim, pad included, because no book prints a pad rule for it", got, want)
	}

	if got, err := l.ParseEXAnswer([]byte("EX00301 005;"), fixtureItem); err != nil || got != "005" {
		t.Errorf("ParseEXAnswer on the shorter unpadded form = %q, %v; want %q and no error", got, err, "005")
	}

	// Between the two readings is a length neither produces.
	if got, err := l.ParseEXAnswer([]byte("EX00301 005      ;"), fixtureItem); err == nil {
		t.Errorf("ParseEXAnswer accepted an 18-byte answer = %q — that is neither the row's printed width nor the printed 24-byte frame", got)
	}
	// And nothing past the printed frame is admitted at all.
	if got, err := l.ParseEXAnswer([]byte("EX00301 "+strings.Repeat("0", 16)+";"), fixtureItem); err == nil {
		t.Errorf("ParseEXAnswer accepted a 25-byte answer = %q — the 990S's grid is drawn to 24", got)
	}

	// The 890S has no such fixed form: a 24-byte frame there is simply a P5
	// wider than the row's printed width.
	if got, err := testLayoutWithEXItems(t, []kw.EXItem{fixtureItem}).ParseEXAnswer([]byte(padded), fixtureItem); err == nil {
		t.Errorf("the TS-890S accepted the 990S's padded 24-byte form = %q — its terminator floats and it prints no window", got)
	}
}

// TestParseEXAnswer_990SFixedFormPadIsForTheCallerToStrip pins the hand-over
// ParseEXAnswer's doc comment names for the 990S's fixed-24 form: the
// returned P5 is fifteen bytes with the value and its pad undifferentiated,
// and neither book says what byte the pad is. This codec does not trim —
// trimming would apply a pad rule no book prints — so a settings reader must
// take the row's own item.Digits leading characters as the value and treat
// the rest as pad; any narrower trim it performs is that reader's own
// ASSUMED entry, not this package's.
//
// Probed with a NON-SPACE pad, so a reader cannot mistake "returned
// verbatim" for "returned trimmed of whitespace": the fixture's item.Digits
// is 3, and all fifteen bytes come back exactly as sent.
func TestParseEXAnswer_990SFixedFormPadIsForTheCallerToStrip(t *testing.T) {
	l := testLayout990WithEXItems(t, []kw.EXItem{fixtureItem})

	if fixtureItem.Digits != 3 {
		t.Fatalf("fixtureItem.Digits = %d, want 3 — this test's obligation-shape claim is pinned to that value", fixtureItem.Digits)
	}

	frame := "EX00301 " + "005XXXXXXXXXXXX" + ";"
	if len(frame) != 24 {
		t.Fatalf("the fixture is %d bytes, want the printed 24 — the test is wrong", len(frame))
	}
	got, err := l.ParseEXAnswer([]byte(frame), fixtureItem)
	if err != nil {
		t.Fatalf("ParseEXAnswer: %v", err)
	}
	if want := "005XXXXXXXXXXXX"; got != want {
		t.Errorf("ParseEXAnswer(%q) = %q, want %q — P5 is verbatim, including a non-space pad", frame, got, want)
	}
	// The obligation itself: the value is the leading item.Digits bytes,
	// the rest is pad, and this codec never draws that line for the caller.
	if value := got[:fixtureItem.Digits]; value != "005" {
		t.Errorf("the row's own leading %d characters = %q, want %q — that slice, not the whole string, is the value a settings reader must take", fixtureItem.Digits, value, "005")
	}
}
