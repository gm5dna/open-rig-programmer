// SPDX-License-Identifier: GPL-3.0-or-later

package ma

import (
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/kw"
)

// THE EIGHT ENVELOPE FRAMES, COPIED FROM core/kw RATHER THAN IMPORTED, AND
// THE COST IS STATED HERE.
//
// BuildIDRead, BuildAIRead, BuildAISetOff, BuildFVRead, BuildEXRead,
// ParseIDAnswer, ParseFVAnswer and ParseEXAnswer are all METHODS ON
// kw.Layout, as are the two generic helpers behind them (buildFixedFrame,
// checkAnswerShape). An MA0 radio cannot construct an honest kw.Layout
// (decision 1), so a method on that type is unreachable from this package
// however exported it is: the line that decides what is reusable in this
// family is not envelope-versus-record, it is PACKAGE FUNCTION versus
// LAYOUT METHOD. Everything package-level IS reused, and it is the larger
// half — kw.NewFramingWithGate and with it the whole framing adapter,
// NewFrameAccumulator, SplitFrames, DefaultMaxFrame, the typed error family,
// PrefixLenMatcher, EXAddress, EXItem, CopyEXItems, MaxEXDigits, and the
// frame-shape constants every builder below measures itself against.
//
// COST: about a hundred duplicated lines of five-to-fifteen-line functions —
// the same class, and the same order of magnitude, as pair 1's forty-line
// ';' splitter copied from core/civ.
//
// THE ALTERNATIVE, REJECTED: promote the eight to package functions taking a
// Book and re-plumb the existing methods onto them. That is a refactor of
// SHIPPING code for two radios' benefit, touching every call site in
// core/driver/ts590 and core/driver/ts480, to save a hundred lines — and it
// would move the eight frames' behaviour out from under pair 1's own
// conformance suite in the same commit that added two untested rows to it.
//
// EVERY FRAME THIS FILE CAN BUILD IS ONE ITS BOOK PRINTS AS A HOST-TO-RADIO
// FORM, WITH ALL ITS PARAMETERS, and each builder's doc comment cites the
// grid line. That is the standing rule of this repository and it is the
// whole admission test for a builder.

// The EX frame shape, which is THIS FAMILY'S OWN and not core/kw's.
//
// core/kw's EX read is TEN bytes with a three-digit P1 (590:552, 480:410);
// here it is EIGHT with a (P1,P2,P3) triple of one, two and two digits, and
// the answer splices P4 in before P5. Nothing about the two shapes is
// shared, which is why these constants are declared rather than imported.
const (
	// EXReadLen is "E X P1 P2 P2 P3 P3 ;" — eight bytes, carrying NO P4,
	// on both radios (890:1907, 990:1736).
	EXReadLen = 8

	// exAddrOff and exAddrLen locate the five-character address at
	// positions 3-7 (890:1900, 990:1723).
	exAddrOff = 2
	exAddrLen = 5

	// exP4Off is position 8, the configuration classification, on BOTH
	// books' own position rulers (890:1898-1900 Set, 890:1909-1910 Answer;
	// 990:1721-1723 Set, 990:1738-1740 Answer). The READ form carries no P4
	// at all.
	exP4Off = 7
	// exP4Answer is the literal ASCII space an ANSWER carries there:
	// "Response is always a space" (890:1912-1915, 990:1737-1741). It is
	// DOCUMENTED and not assumed — both books print it — and it is named
	// once because the parser requires it and because erratum E14 records
	// how easily a chart's "Unused (1 digit)" wording invites a builder to
	// emit '0' into such a field instead.
	exP4Answer = ' '

	// exP5Off is position 9, where P5 begins in an answer.
	exP5Off = 8
	// exAnswerMinLen is the shortest EX answer: the eight positions through
	// P4 plus the terminator, with a ZERO-character P5. Both books print
	// width classes that start at zero — "a power-on message can vary in
	// length from 0 to 15 characters" (890:1920, 990:1751) — so a
	// nine-byte answer is a printed form and not a truncation.
	exAnswerMinLen = exP5Off + 1

	// ex990AnswerLen is the TS-990S's PRINTED answer length, and the whole
	// of ERRATUM E19. That book's EX Set and Answer grids are both drawn to
	// position 24 — the ruler holds P5 to exactly fifteen bytes at
	// positions 9-23 and nails ';' to 24 (990:1719-1732 Set, 990:1738-1747
	// Answer) — while the SAME chart's P5 note is the variable-length one
	// both books print (990:1742-1756). The diagram and the note beside it
	// disagree, so this codec admits both readings; see ParseEXAnswer. The
	// TS-890S's diagram is honest about the variability instead: its ruler
	// reads "9~" and the terminator's header cell is the letter "x", never
	// a number (890:1898-1904 Set, 890:1909-1913 Answer).
	ex990AnswerLen = 24
	// ex990P5Window is that window's width, 24 - 8 - 1.
	ex990P5Window = ex990AnswerLen - exAnswerMinLen
)

// The three read frames, written once each because each is the whole of its
// builder's output and, once task 8 lands, its gate's admission rule. Two
// literals a file apart would be one edit from disagreeing, and the gate is
// the last defence before a physical radio.
const (
	idReadFrame = "ID;"
	aiReadFrame = "AI;"
	aiSetFrame  = "AI0;"
	fvReadFrame = "FV;"
)

// maxParseErrorFrameLen bounds how much offending input a ParseError
// retains, so malformed or hostile input cannot make a log line unbounded.
// It is core/kw's own bound, restated because kw's constant is unexported.
const maxParseErrorFrameLen = 64

// newParseError builds a *kw.ParseError from the offending input, copying and
// truncating it.
//
// THE TYPE IS core/kw'S AND THE MINTER IS THIS PACKAGE'S. kw.ParseError's
// fields are exported, so this package's refusals are the SAME type a driver
// already tests for and wrap the SAME kw.ErrParse sentinel — which is what
// lets one driver-side errors.As arm cover both codecs of one family.
func newParseError(input []byte, format string, args ...any) *kw.ParseError {
	n := min(len(input), maxParseErrorFrameLen)
	return &kw.ParseError{Frame: copyBytes(input[:n]), Reason: fmt.Sprintf(format, args...)}
}

// BuildIDRead builds the transceiver-identity read, "ID;" (890:2735,
// 990:2614). Both books print the same three bytes, so this builder has
// exactly one output on every row.
//
// IT TAKES A LAYOUT RECEIVER EVEN THOUGH NOTHING ABOUT THE FRAME VARIES.
// A mixed API, half Layout methods and half package functions, is the shape
// in which a site that consults no radio hides; and the receiver is what
// makes the zero-layout refusal reachable at all — a package function would
// emit "ID;" on behalf of no radio.
func (l Layout) BuildIDRead() (Command, error) {
	return l.buildFixedFrame("ID read", idReadFrame, kw.IDReadLen)
}

// BuildAIRead builds the Auto Information read, "AI;" (890:181, 990:179).
//
// NOTHING IN THIS MILESTONE SENDS IT: the session writes AI0; at open and
// never asks what the state was. It exists because the frame is printed on
// both radios and the gate will admit it, and a gate admitting a frame no
// builder can produce is a gate nothing pins.
func (l Layout) BuildAIRead() (Command, error) {
	return l.buildFixedFrame("AI read", aiReadFrame, kw.AIReadLen)
}

// BuildAISetOff builds the ONE Auto Information Set this codec has: "AI0;",
// "0: AI OFF" (890:175-177, 990:173-175).
//
// THERE IS NO PARAMETER TO PASS ANOTHER STATE TO, and the API is what
// enforces that rather than a refusal inside it. Both books print "1: Not
// used" and "3: Not used" and give 2 and 4 to AI ON (890:175-181,
// 990:173-178), and an AI-ON radio pushes a response per changed parameter
// (890:183-185, 990:180-182) into a session that correlates answers by prefix
// and length.
//
// IT MUST BE THE FRAME THE SESSION ACTUALLY OPENS WITH, and this package
// spells it a second time only because core/kw's initFrame is unexported.
// TestBuildAISetOff_IsTheFrameTheSessionACTUALLYWrites compares this output
// against the framing's own InitSequence, which is what makes the two one
// datum in practice: a session that disabled Auto Information with one
// spelling and re-disabled it with another would still work, until the day
// one of the two was edited.
func (l Layout) BuildAISetOff() (Command, error) {
	return l.buildFixedFrame("AI set", aiSetFrame, kw.AISetLen)
}

// BuildFVRead builds the firmware-version read, "FV;" (890:2655, 990:2532).
//
// BOTH BOOKS PRINT FV, WHICH IS THE DIFFERENCE FROM core/kw's BuildFVRead.
// There the builder refuses Book480, because that document has no FV command
// anywhere; here the charts are printed identically on the two radios
// (890:2650-2659, 990:2527-2536), so neither row refuses the other's book and
// there is no requireBook helper in this file at all.
func (l Layout) BuildFVRead() (Command, error) {
	return l.buildFixedFrame("FV read", fvReadFrame, kw.FVReadLen)
}

// buildFixedFrame is the one constructor for a frame whose bytes are wholly
// printed — a frame with no parameter field a caller can vary. Every read in
// this file is one, and so is the single AI Set it builds.
//
// The width assertion is not defensive clutter: it is the same discipline
// BuildEXRead applies to its own output, so that a literal edited without its
// length constant, or the reverse, fails here rather than on a radio.
func (l Layout) buildFixedFrame(what, frame string, wantLen int) (Command, error) {
	if !l.Configured() {
		return Command{}, newParseError(nil, "%s: this layout is unconfigured and describes no radio", what)
	}
	if len(frame) != wantLen {
		return Command{}, newParseError([]byte(frame), "%s: built %d bytes, want exactly %d", what, len(frame), wantLen)
	}
	return newCommand([]byte(frame)), nil
}

// ParseIDAnswer decodes "I D P1 P1 P1 ;" and returns P1 as its three printed
// digits (890:2738, 990:2617).
//
// IT RETURNS THE TOKEN AND CLASSIFIES NOTHING. Which model a token names —
// 024 TS-890S, printed with the model name (890:2733); 022, printed bare
// (990:2612); and 020, 021 and 023 in the other book — is the DRIVER's
// question, asked once at probe against its own row's CAT ID. A codec that
// answered it would hold a registry, and every row of that registry would be
// a second copy of a fact the driver already states.
//
// THE THREE DIGITS ARE REQUIRED TO BE DIGITS. Both charts print P1 as numeric
// values and nothing in either document admits another byte there, and this
// package refuses what is not explicitly valid rather than guessing at it.
func (l Layout) ParseIDAnswer(frame []byte) (string, error) {
	if err := l.checkAnswerShape("ID answer", frame, "ID", kw.IDAnswerLen); err != nil {
		return "", err
	}
	field := frame[2 : 2+kw.IDDigits]
	for i, b := range field {
		if b < '0' || b > '9' {
			return "", newParseError(frame, "ID answer: P1 byte %d is %q; both books print P1 as three digits (890:2738, 990:2617), and this family's answer is %d bytes with %d digits where a Yaesu one is 7 bytes with 4", i+1, b, kw.IDAnswerLen, kw.IDDigits)
		}
	}
	return string(field), nil
}

// ParseFVAnswer decodes "F V P1 P1 P1 P1 ;" and returns P1 VERBATIM
// (890:2659, 990:2536).
//
// THE WIDTH IS PINNED AND THE GRAMMAR IS NOT, WHICH IS A13. Both answer
// charts give four positions and the only format statement in either book is
// one worked example, "FV1.00;" (890:2657, 990:2533). So a four-character
// answer is accepted whatever those characters are: a parser demanding
// digit-dot-digit-digit would assert a grammar the documents do not print and
// would fail a session on a radio they permit. A version needing five
// characters does not fit the printed grid, and nothing says what the radio
// would then send — which is the open half of A13.
//
// THE CHARSET IS THE ENVELOPE'S, NOT A GUESS. Each of the four bytes must be
// printable ASCII other than ';' — a frame carrying an embedded terminator is
// two frames to the radio's own parser and one to this one, and A2's charset
// claim is bounded at 0x7E.
func (l Layout) ParseFVAnswer(frame []byte) (string, error) {
	if err := l.checkAnswerShape("FV answer", frame, "FV", kw.FVAnswerLen); err != nil {
		return "", err
	}
	field := frame[2 : 2+kw.FVChars]
	for i, b := range field {
		if b < 0x20 || b > 0x7e || b == ';' {
			return "", newParseError(frame, "FV answer: P1 byte %d is %#02x, which is not a printable ASCII character this codec will read as part of a version string (A2's charset claim is bounded at 0x7E, and an embedded ';' is a second frame to the radio's own parser)", i+1, b)
		}
	}
	return string(field), nil
}

// checkAnswerShape applies the three structural checks every fixed-width
// answer in this file shares, IN THE ORDER core/kw FIXED: length, then
// prefix, then terminator.
//
// THE ORDER IS PART OF THE CONTRACT, not an implementation detail. A frame
// that is wrong in several ways must report the same failure whichever answer
// it was offered to, so that a session log reads consistently and a test
// asserting "the length first" is asserting one rule rather than several
// copies of one.
func (l Layout) checkAnswerShape(what string, frame []byte, prefix string, wantLen int) error {
	if !l.Configured() {
		return newParseError(frame, "%s: this layout is unconfigured and describes no radio, so no byte of this frame has a meaning to read", what)
	}
	if len(frame) != wantLen {
		return newParseError(frame, "%s: the frame is %d bytes, want exactly %d", what, len(frame), wantLen)
	}
	if string(frame[:len(prefix)]) != prefix {
		return newParseError(frame, "%s: missing %q prefix, got %q", what, prefix, frame[:len(prefix)])
	}
	if frame[wantLen-1] != ';' {
		return newParseError(frame, "%s: missing ';' terminator at position %d", what, wantLen)
	}
	return nil
}

// WireEXAddress renders addr as this family's EX address field: FIVE
// characters, P1 + P2P2 + P3P3 (890:1900, 990:1723).
//
// IT IS A LAYOUT METHOD AND kw.EXAddress.Wire IS UNCHANGED, which is
// core/cat's lesson applied rather than repeated. kw.EXAddress.Wire renders
// "%03d" from P1 alone and its doc comment says in terms that the MR/MW rows
// have ONE width and "no second form for a method to get wrong". There now is
// a second form, and the repair is NOT to parameterise that method — it is to
// leave it exactly as it is for the rows that can reach it and put this
// family's rendering here. Cost: one four-line method. No behaviour change to
// any registered radio.
//
// IT FAILS CLOSED on any component the charts do not print: P1 is a menu-type
// flag with exactly two values, "0: Menu" and "1: Advanced Menu"
// (890:1897-1900, 990:1720-1723), and P2 and P3 are two digits each, "00 ~
// 99" (890:1901-1911, 990:1724-1735). Rendering a wider component would emit
// a field no chart prints; discarding one would silently lose what the caller
// believed it had supplied. "" then reaches the outbound gate as a frame that
// cannot pass it.
func (l Layout) WireEXAddress(addr kw.EXAddress) string {
	if addr.P1 > 1 || addr.P2 > 99 || addr.P3 > 99 {
		return ""
	}
	return fmt.Sprintf("%d%02d%02d", addr.P1, addr.P2, addr.P3)
}

// BuildEXRead builds the menu read for addr: "E X P1 P2 P2 P3 P3 ;", eight
// bytes carrying NO P4 (890:1905-1907, 990:1734-1736).
//
// IT IS BOUNDED BY MEMBERSHIP AND NOT BY A SCALAR, which is the difference
// from core/kw's builder and the reason ma.Layout carries an inventory at all.
// core/kw bounds an EX read with Layout.maxEXAddress because pair 1's books
// print a CONTIGUOUS menu domain ("000 ~ 087", "000 ~ 099", "000 ~ 060");
// NEITHER of these books prints a contiguous domain — both charts are sparse,
// with gaps inside every category — so a ceiling would admit an address no
// chart prints, and the book's own EX block says what the radio does with
// one: "Entering a non-existing number causes an error to occur", and
// "Entering a number that cannot be set also causes an error to occur"
// (890:1904, 890:1909-1911; 990:1727, 990:1733-1735).
//
// A BOUND IS CONSULTED FROM THE SAME PLACE AS ITS DATUM. The builder, the
// parser and the outbound gate all ask Layout.EXItem, so no two of them can
// disagree about what this radio has.
func (l Layout) BuildEXRead(addr kw.EXAddress) (Command, error) {
	if !l.Configured() {
		return Command{}, newParseError(nil, "EX read: this layout is unconfigured and describes no radio")
	}
	if _, ok := l.EXItem(addr); !ok {
		return Command{}, newParseError(nil, "EX read: menu %v is not in %s's transcribed menu inventory — both books' charts are sparse, and the book says an address the chart does not print \"causes an error to occur\" (890:1904, 990:1727)", addr, l.model)
	}
	wire := l.WireEXAddress(addr)
	if len(wire) != exAddrLen {
		return Command{}, newParseError(nil, "EX read: %v does not render as a %d-character menu address; P1 is 0 or 1 and P2 and P3 are two digits each (890:1897-1911, 990:1720-1735)", addr, exAddrLen)
	}
	frame := make([]byte, 0, EXReadLen)
	frame = append(frame, 'E', 'X')
	frame = append(frame, wire...)
	frame = append(frame, ';')
	if len(frame) != EXReadLen {
		return Command{}, newParseError(frame, "EX read: built %d bytes, want exactly %d (890:1907, 990:1736)", len(frame), EXReadLen)
	}
	return newCommand(frame), nil
}

// ParseEXAnswer decodes one EX answer against the inventory row that read it
// and returns P5 VERBATIM.
//
// THE WIDTH BOUND IS A19, AND IT IS A CEILING. Both books print P5's widths
// per item class — normally 3 digits, 4 for PF keys, 8 for the 990S's
// frequency settings, 0-15 for a power-on message, 0-10 for screen-saver text
// (890:1916-1921, 990:1742-1752) — and neither prints a general ceiling. A19
// is the assumption that an answer never EXCEEDS the width the parameter list
// prints for that menu number, and its lift is an exhaustive sweep of the
// row's whole printed domain, because a ceiling is not lifted by a sample. So
// the bound is item.Digits, item.Digits comes from that row's own transcribed
// chart, and a SHORTER answer is admitted: A19 claims a maximum and nothing
// else.
//
// AND THE 990S HAS A SECOND ADMITTED FORM, WHICH IS ERRATUM E19. Its Set and
// Answer grids are both drawn to position 24, with P5 at positions 9-23 and
// the ';' nailed to 24 (990:1719-1732 Set, 990:1738-1747 Answer), while the
// same chart's own P5 note prints the variable-length classes both books give
// (990:1742-1756). The two readings cannot both be literal, so BOTH are
// admitted and both are pinned: the printed twenty-four-byte frame with P5
// filling its fifteen-wide window, and a shorter frame carrying the row's own
// printed width. A length between them is refused, because neither reading
// produces one. A parser built to the diagram alone would refuse or mis-scan
// every item class narrower than the widest, which is what E19 exists to stop
// a later reader doing. The TS-890S has no fixed form at all — its terminator
// floats under a ruler head printed "x" (890:1898-1904, 890:1909-1913) — so
// this arm is inert on that row.
//
// P5 IS RETURNED VERBATIM, INCLUDING ANY TRAILING SPACES. Neither book states
// a padding rule for P5 — A1's rule is the MA0 name field's and is scoped to
// it — so trimming here would apply an assumption to a field the register
// does not cover, and on the 990S's padded form the pad IS the evidence that
// the fixed reading was the right one.
//
// THE WHOLE ADDRESS IS THE CORRELATION KEY. Every one of a radio's menu
// addresses answers with a frame starting "EX", so an answer whose address
// field is not the one item names is a DIFFERENT menu's reply — still in
// flight, or arriving out of order — and returning its P5 would hand the
// caller one setting under another's name.
//
// THE INVENTORY ROW IS VALIDATED TOO, and against this row's own inventory. A
// zero Digits is a row that was never transcribed, and parsing against it
// would apply a ceiling of nothing; a Digits above kw.MaxEXDigits describes an
// answer longer than this family's own maximum frame, which its accumulator
// would discard as contamination. And without the membership check this
// parser would be the one UNBOUNDED path through the domain: the builder
// refuses to SEND an address this chart does not print, and an answer
// carrying one is not a setting this radio has.
func (l Layout) ParseEXAnswer(frame []byte, item kw.EXItem) (string, error) {
	if !l.Configured() {
		return "", newParseError(frame, "EX answer: this layout is unconfigured and describes no radio, so no byte of this frame has a meaning to read")
	}
	if item.Digits < 1 || item.Digits > kw.MaxEXDigits {
		return "", newParseError(frame, "EX answer: the inventory row for %v declares a printed width of %d, and this codec admits 1 to %d — a zero width is a row that was never transcribed, and a wider one describes an answer longer than this family's own %d-byte frame bound (A19)", item.Addr, item.Digits, kw.MaxEXDigits, kw.DefaultMaxFrame)
	}
	if _, ok := l.EXItem(item.Addr); !ok {
		return "", newParseError(frame, "EX answer: menu %v is not in %s's transcribed menu inventory — the builder and the gate refuse to send that address, and an answer carrying it is not a setting this row has", item.Addr, l.model)
	}
	if len(frame) < exAnswerMinLen {
		return "", newParseError(frame, "EX answer: the frame is %d bytes; the eight positions through P4 are followed by the terminator, and a frame of exactly %d bytes is the READ, which carries no P4 at all (890:1907, 990:1736)", len(frame), EXReadLen)
	}
	if len(frame) > kw.DefaultMaxFrame {
		return "", newParseError(frame, "EX answer: the frame is %d bytes, past this family's own %d-byte bound, which its accumulator would have discarded as contamination", len(frame), kw.DefaultMaxFrame)
	}
	if string(frame[:2]) != "EX" {
		return "", newParseError(frame, "EX answer: missing %q prefix, got %q", "EX", frame[:2])
	}
	if frame[len(frame)-1] != ';' {
		return "", newParseError(frame, "EX answer: missing ';' terminator at position %d", len(frame))
	}

	got := string(frame[exAddrOff : exAddrOff+exAddrLen])
	for i, b := range []byte(got) {
		if b < '0' || b > '9' {
			return "", newParseError(frame, "EX answer: address byte %d is %q; the address is five decimal digits on both radios, P1 + P2P2 + P3P3 (890:1900, 990:1723)", i+1, b)
		}
	}
	if want := l.WireEXAddress(item.Addr); got != want {
		return "", newParseError(frame, "EX answer: this frame answers menu %s and the read asked for menu %s — every menu address answers with a frame starting \"EX\", so the whole address is what correlates an answer to its read", got, want)
	}
	if frame[exP4Off] != exP4Answer {
		return "", newParseError(frame, "EX answer: P4 is %q, and both books print \"Response is always a space\" (890:1912-1915, 990:1737-1741)", frame[exP4Off])
	}

	p5 := frame[exP5Off : len(frame)-1]
	if err := l.checkEXP5Width(frame, p5, item); err != nil {
		return "", err
	}
	for i, b := range p5 {
		if b < 0x20 || b > 0x7e || b == ';' {
			return "", newParseError(frame, "EX answer: P5 byte %d is %#02x; both books forbid the control codes generally, an embedded ';' is a second frame to the radio's own parser, and A2's charset claim is bounded at 0x7E", i+1, b)
		}
	}
	return string(p5), nil
}

// checkEXP5Width is the two-form width rule ParseEXAnswer's doc comment
// states, kept apart so the two readings are one visible either/or rather
// than a condition a later edit could widen by accident.
//
// It switches ONCE on the receiver's book, which is this package's sanctioned
// idiom for a difference between the two charts: a package function handed a
// frame could not know which grid it was looking at and would have to infer
// the book from the byte count.
func (l Layout) checkEXP5Width(frame, p5 []byte, item kw.EXItem) error {
	if l.book == kw.Book990 {
		if len(frame) == ex990AnswerLen && len(p5) == ex990P5Window {
			// The printed fixed form: P5 fills its window (E19,
			// 990:1719-1732, 990:1738-1747). The pad is returned verbatim.
			return nil
		}
		if len(frame) > ex990AnswerLen {
			return newParseError(frame, "EX answer: the frame is %d bytes and this book draws its EX Answer to %d, with P5 in a %d-wide window at positions 9-%d (E19; 990:1738-1747)", len(frame), ex990AnswerLen, ex990P5Window, exP5Off+ex990P5Window)
		}
	}
	if len(p5) > item.Digits {
		return newParseError(frame, "EX answer: P5 is %d characters and the parameter list prints %d for menu %s (%s) — A19 is that an answer never exceeds its printed width, and a wider one is refused rather than truncated", len(p5), item.Digits, l.WireEXAddress(item.Addr), item.Name)
	}
	return nil
}
