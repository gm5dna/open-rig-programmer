// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import "fmt"

// THE THREE IDENTITY GRAMMARS, AND WHY THEY ARE THREE RATHER THAN ONE.
//
// ID is common to both books and answers a THREE-DIGIT model number in a
// SIX-byte frame. FV is the TS-590S/SG document's alone and answers four
// characters in seven bytes; TY is the 2003 TS-480 document's alone and
// answers two opaque reserved bytes plus a variant digit in six. Neither
// command appears in the other book at all — a grep of each layout text for
// the other's name returns nothing — so a layout that built the wrong one
// would emit a frame its own radio's document does not describe. Every
// builder and parser below is therefore a Layout method that consults
// Layout.Book, and the two book-specific ones REFUSE the other book rather
// than emitting on a venture.
//
// NONE OF THIS IS core/cat's IDENTITY. That package's answer is seven bytes
// with four digits (idAnswerLen = 7) and its CATID contract says "exactly
// four bytes"; those are Yaesu facts. The negative pin
// TestParseIDAnswer_RefusesTheYaesuSevenByteFourDigitAnswer is what stops
// the two being conflated, and it is written against a REAL Yaesu frame
// because a Kenwood answer is six bytes and a Yaesu one is seven — "a
// four-byte ID answer" describes no frame either family sends.

// The identity frame lengths, each counted off its own book's printed
// position ruler.
//
// THEY ARE EXPORTED BECAUSE A DRIVER NEEDS THEM. Every read this milestone
// issues is correlated by PrefixLenMatcher(prefix, exactLen), and exactLen
// is one of these numbers; a driver that transcribed the literal instead
// would hold a second copy of a datum this package owns.
const (
	// IDReadLen is "I D ;" (590:1115, 480:683).
	IDReadLen = 3
	// IDAnswerLen is "I D P1 P1 P1 ;" (590:1119, 480:687) — SIX bytes,
	// THREE digits.
	IDAnswerLen = 6
	// IDDigits is P1's width in that answer: three, not four
	// (590:1114-1116, 480:678).
	IDDigits = 3

	// FVReadLen is "F V ;" (590:1034), the TS-590S/SG document's alone.
	FVReadLen = 3
	// FVAnswerLen is "F V P1 P1 P1 P1 ;" (590:1037).
	FVAnswerLen = 7
	// FVChars is P1's width in that answer: four characters. THE WIDTH IS
	// PINNED AND THE GRAMMAR IS NOT — see ParseFVAnswer and A13.
	FVChars = 4

	// TYReadLen is "T Y ;" (480:1630), the 2003 TS-480 document's alone.
	TYReadLen = 3
	// TYAnswerLen is "T Y P1 P1 P2 ;" (480:1634).
	TYAnswerLen = 6
	// TYReservedLen is P1's width in that answer: two bytes printed
	// "Reserved" (480:1623), accepted OPAQUELY.
	TYReservedLen = 2
)

// The three read frames, written once each because each is the whole of its
// builder's output and its gate's admission rule. Two literals a file apart
// would be one edit from disagreeing, and the gate is the last defence
// before a physical radio.
const (
	idReadFrame = "ID;"
	fvReadFrame = "FV;"
	tyReadFrame = "TY;"
)

// BuildIDRead builds the transceiver-identity read, "ID;" (590:1115,
// 480:683). Both books print the same three bytes, so this builder has
// exactly one output on every row.
//
// IT TAKES A LAYOUT RECEIVER EVEN THOUGH NOTHING ABOUT THE FRAME VARIES,
// for core/cat's reason at validIDCommand and one of this package's own: a
// mixed API, half Layout methods and half package functions, is the shape in
// which a site that consults no radio hides. The receiver is also what makes
// the zero-layout refusal reachable — a package function would emit "ID;" on
// behalf of no radio at all.
func (l Layout) BuildIDRead() (Command, error) {
	return l.buildFixedRead("ID read", idReadFrame, IDReadLen)
}

// BuildFVRead builds the firmware-version read, "FV;" (590:1034).
//
// THE TS-480 HAS NO FIRMWARE VERSION TO READ AT ALL — no FV command
// anywhere in its book, and no firmware statement anywhere in it either —
// so a Book480 layout refuses rather than emitting a frame that document
// does not describe. TY is that radio's nearest equivalent and it reports a
// hardware variant, not a version.
func (l Layout) BuildFVRead() (Command, error) {
	if err := l.requireBook("FV read", Book590); err != nil {
		return Command{}, err
	}
	return l.buildFixedRead("FV read", fvReadFrame, FVReadLen)
}

// BuildTYRead builds the microprocessor firmware-type read, "TY;"
// (480:1630).
//
// THE 590 PAIR HAVE NO TY — the name appears nowhere in their document — so
// a Book590 layout refuses. What TY reports is a TS-480 HARDWARE VARIANT
// (480:1626-1629); decision 4 reads it at probe and reports it, and never
// uses it to select a registry row.
func (l Layout) BuildTYRead() (Command, error) {
	if err := l.requireBook("TY read", Book480); err != nil {
		return Command{}, err
	}
	return l.buildFixedRead("TY read", tyReadFrame, TYReadLen)
}

// buildFixedRead is the one constructor for a read whose bytes are wholly
// printed — a frame with no parameter field at all.
//
// The width assertion is not defensive clutter: it is the same discipline
// BuildMRRead and BuildMWSet apply to their own output, so that a literal
// edited without its length constant, or the reverse, fails here rather than
// on a radio.
func (l Layout) buildFixedRead(what, frame string, wantLen int) (Command, error) {
	if !l.Configured() {
		return Command{}, newParseError(nil, "%s: this layout is unconfigured and describes no radio", what)
	}
	if len(frame) != wantLen {
		return Command{}, newParseError([]byte(frame), "%s: built %d bytes, want exactly %d", what, len(frame), wantLen)
	}
	return newCommand([]byte(frame)), nil
}

// requireBook refuses a command whose chart is printed in only one of the
// two documents.
//
// IT IS A REFUSAL AND NOT A NO-OP, because the alternative is worse than
// untidy: an FV built for a TS-480 is three bytes the 2003 document never
// prints, and both books answer an unrecognised command with "?;", whose two
// causes are indistinguishable (590:100-105, 480:130-135). The session would
// report a refusal it could not explain.
func (l Layout) requireBook(what string, want Book) error {
	if !l.Configured() {
		return newParseError(nil, "%s: this layout is unconfigured and describes no radio", what)
	}
	if l.book != want {
		return newParseError(nil, "%s: this command is printed only in the %v, and the %s speaks the %v", what, want, l.model, l.book)
	}
	return nil
}

// ParseIDAnswer decodes "I D P1 P1 P1 ;" and returns P1 as its three
// printed digits (590:1119, 480:687).
//
// IT RETURNS THE TOKEN AND CLASSIFIES NOTHING. Which model a token names —
// 021 TS-590S, 023 TS-590SG (590:1114-1116), 020 TS-480 (480:678), and the
// siblings 022 and 024 that this milestone does not register — is the
// DRIVER's question, asked once at probe against its own table. A codec that
// answered it would hold a registry, and every row of that registry would be
// a second copy of a fact the driver already states.
//
// THE THREE DIGITS ARE REQUIRED TO BE DIGITS. Both charts print P1 as
// numeric values and nothing in either document admits another byte there,
// and this package refuses what is not explicitly valid rather than guessing
// at it.
func (l Layout) ParseIDAnswer(frame []byte) (string, error) {
	if err := l.checkAnswerShape("ID answer", frame, "ID", IDAnswerLen); err != nil {
		return "", err
	}
	field := frame[2 : 2+IDDigits]
	for i, b := range field {
		if b < '0' || b > '9' {
			return "", newParseError(frame, "ID answer: P1 byte %d is %q; both books print P1 as three digits (590:1114-1116, 480:678), and a Kenwood answer is %d bytes with %d digits where a Yaesu one is 7 bytes with 4", i+1, b, IDAnswerLen, IDDigits)
		}
	}
	return string(field), nil
}

// ParseFVAnswer decodes "F V P1 P1 P1 P1 ;" and returns P1 VERBATIM
// (590:1037).
//
// THE WIDTH IS PINNED AND THE GRAMMAR IS NOT, WHICH IS A13. The answer chart
// gives four positions; the only format statement anywhere is one worked
// example, "for firmware version 1.00, it reads FV1.00;" (590:1035). So a
// four-character answer is accepted whatever those characters are, and the
// caller — the TS-590S write gate, which must decide whether byte 28 is live
// (A14) — is what interprets them and what must survive an answer it cannot
// interpret. A parser that demanded digit-dot-digit-digit would assert a
// grammar the document does not print, and would fail a session on a radio
// the document permits.
//
// THE CHARSET IS THE ENVELOPE'S, NOT A GUESS. Each of the four bytes must be
// printable ASCII other than ';' — the 480 states the control-code rule
// generally (480:127-129) and A2's claim is bounded at 0x7E — because a
// frame carrying an embedded terminator is two frames to the radio's own
// parser and one to this one.
func (l Layout) ParseFVAnswer(frame []byte) (string, error) {
	if err := l.requireBook("FV answer", Book590); err != nil {
		return "", err
	}
	if err := l.checkAnswerShape("FV answer", frame, "FV", FVAnswerLen); err != nil {
		return "", err
	}
	field := frame[2 : 2+FVChars]
	for i, b := range field {
		if b < 0x20 || b > 0x7e || b == ';' {
			return "", newParseError(frame, "FV answer: P1 byte %d is %#02x, which is not a printable ASCII character this codec will read as part of a version string (480:127-129 states the control-code rule; A2's claim is bounded at 0x7E)", i+1, b)
		}
	}
	return string(field), nil
}

// TYAnswer is one decoded TY answer: P1's two reserved bytes, kept opaque,
// and P2's variant digit.
//
// P1 IS CARRIED AND NOT INTERPRETED. The chart prints it "Reserved"
// (480:1623) and says nothing else about it anywhere, so this type reports
// the bytes and makes no claim; decision 4 is explicit that a guessed digit
// predicate over P1 would refuse a legitimate radio.
type TYAnswer struct {
	// Reserved is P1's two bytes verbatim. It is a string rather than a
	// name because nothing in the document gives it one.
	Reserved string
	// Variant is P2, one of '0'..'3' (480:1626-1629). The parser refuses
	// any other byte, so a TYAnswer this package produced always carries
	// one of the four.
	Variant byte
}

// VariantName is the 480's own wording for this variant (480:1626-1629).
//
// IT NAMES THE UNPRINTED BYTE RATHER THAN GUESSING AT IT. A TYAnswer built
// by ParseTYAnswer can only carry '0'..'3', but the type is constructible by
// any caller, and a default arm returning one of the four names — or an
// empty string a caller would print as a blank probe note — would put a
// variant in a manufacturer's mouth. Decision 4 refuses an unprinted P2 at
// the parser for the same reason.
func (a TYAnswer) VariantName() string {
	switch a.Variant {
	case '0':
		return "TS-480HX (200 W)"
	case '1':
		return "TS-480SAT (100 W + AT)"
	case '2':
		return "Japanese 50 W type"
	case '3':
		return "Japanese 20 W type"
	default:
		return fmt.Sprintf("unprinted TY variant %q — the 2003 document prints '0'..'3' and no other value (480:1626-1629)", a.Variant)
	}
}

// ParseTYAnswer decodes "T Y P1 P1 P2 ;" (480:1634), on DECISION 4's
// grammar exactly.
//
// Six bytes; prefix "TY"; terminator at byte 6; P1's TWO bytes accepted
// OPAQUELY — any byte that is neither a control code 00-1Fh nor ';', which
// is the document's own general interior-byte rule (480:127-129) and the
// only rule it gives for a field printed "Reserved" (480:1623); P2 accepted
// ONLY in '0'..'3'.
//
// AN UNEXPECTED P2 IS REFUSED, AND THE REFUSAL IS THE CONSERVATIVE SIDE OF A
// REAL TRADE. A future TS-480 variant answering P2='4' is refused by this
// programme until the row is widened. Reporting it opaquely instead would
// mean shipping a capability table whose provenance is four printed variants
// to a fifth nobody has read about; defaulting it to one of the four would
// be worse again.
//
// P1 ADMITS 0x7F AND ABOVE, WHICH NO OTHER FIELD IN THIS PACKAGE DOES, and
// the difference is deliberate rather than an oversight. Elsewhere the bound
// is A2's, whose claim stops at 0x7E; here the rule is the 480's own
// interior-byte sentence, which excludes the control codes and ';' and
// nothing else — and decision 4 quotes it as the only rule the document
// gives for this field.
func (l Layout) ParseTYAnswer(frame []byte) (TYAnswer, error) {
	if err := l.requireBook("TY answer", Book480); err != nil {
		return TYAnswer{}, err
	}
	if err := l.checkAnswerShape("TY answer", frame, "TY", TYAnswerLen); err != nil {
		return TYAnswer{}, err
	}
	reserved := frame[2 : 2+TYReservedLen]
	for i, b := range reserved {
		if b < 0x20 || b == ';' {
			return TYAnswer{}, newParseError(frame, "TY answer: P1 byte %d is %#02x; P1 is printed \"Reserved\" (480:1623) and is accepted opaquely, but the document forbids the control codes 00-1Fh and ';' in any parameter (480:127-129)", i+1, b)
		}
	}
	variant := frame[2+TYReservedLen]
	if variant < '0' || variant > '3' {
		return TYAnswer{}, newParseError(frame, "TY answer: P2 is %q, and the document prints exactly four variants, '0'..'3' (480:1626-1629); decision 4 refuses a fifth rather than reporting it as an unknown variant or defaulting it to one of the four", variant)
	}
	return TYAnswer{Reserved: string(reserved), Variant: variant}, nil
}

// checkAnswerShape applies the three structural checks every fixed-width
// answer in this file shares, IN THE ORDER ParseMRAnswer FIXED: length,
// then prefix, then terminator.
//
// THE ORDER IS PART OF THE CONTRACT, not an implementation detail. A frame
// that is wrong in several ways must report the same failure whichever
// answer it was offered to, so that a session log reads consistently and a
// test asserting "the length first" is asserting one rule rather than four
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
