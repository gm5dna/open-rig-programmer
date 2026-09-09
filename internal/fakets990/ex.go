// SPDX-License-Identifier: GPL-3.0-or-later

package fakets990

import "strings"

// This file is fakets990's own, independent model of the EX (MENU) command on
// the TS-990S — READ ONLY, exactly as internal/fakets590 and
// internal/fakets890 model their rows': the book documents a Set form and this
// fake does not implement it (doc.go's "What this fake deliberately does NOT
// model").
//
// The widths table it answers from is projected at init by exinventory.go from
// this package's own copy of transcription B; doc.go's "WHERE THE EX INVENTORY
// COMES FROM" states the two-source mechanism in full.
//
// # WHAT THE TABLE MODELS
//
// Wire behaviour only: which addresses this chart has, and each one's raw P5
// reply WIDTH. It records nothing about what a menu MEANS — no name, no
// legend, no valid range. The names live in the codec's inventory, which is
// the layer with a reason to know them; this fake answers reads.
//
// # EX GRAMMAR (990:1719-1756)
//
// Read frame (8 bytes): "E X P1 P2 P2 P3 P3 ;" — a ONE-digit menu type in P1
// ("0: Menu", "1: Advanced Menu", 990:1722-1723), a TWO-digit category in P2
// ("00 ~ 99", 990:1726) and a TWO-digit entry in P3 ("00 ~ 99", 990:1732).
// THERE IS NO P4 ON THE READ (990:1734-1736).
//
// Answer frame: the read's seven address positions, then P4 spliced in at
// position 8 and P5 from position 9 (990:1738-1747). P4 on an answer is a
// SPACE and nothing else — "Response is always a space." (990:1741), which is
// printed and is not an assumption.
//
// Set frame: the same shape as the answer, with P4 either a space or '9'
// (990:1738-1739) — NOT modelled; see handleEX.
//
// # THE ANSWER IS TWENTY-FOUR BYTES, AND THAT IS ERRATUM E19
//
// This chart's Set and Answer diagrams are drawn FIXED: the position ruler
// holds P5 to exactly fifteen cells, positions 9 to 23, and nails ';' to
// position 24 (990:1721-1732 Set, 990:1738-1747 Answer). The SAME chart's own
// P5 note is the variable-length one both books print — 3 digits normally, 4
// for PF keys, 8 for a frequency setting, 0 to 15 for a power-on message, 0 to
// 10 for screen-saver text (990:1746-1752) — so the diagram and the note
// beside it disagree, and the two readings cannot both be literal. The 890S's
// diagram for the identical field is honest about the variability: its ruler
// reads "9~" and the terminator's own header cell is the letter "x", never a
// number.
//
// THIS FAKE PRINTS THE FORM THE DIAGRAM DRAWS: a P5 narrower than the window
// is padded into it and the frame is 24 bytes. That is a modelling choice
// between two printed readings, not a third invention, and it is the one this
// radio's own picture of its own answer shows. What the pad BYTE is, the
// diagram does not say — doc.go's register entry THE FIXED FORM'S PAD BYTE IS
// A SPACE.
//
// THE FRAME SHAPE IS THIS ROW'S OWN AND NOT core/kw's. That package's EX read
// is TEN bytes with a three-digit menu number and a P2/P3/P4 of printed
// constants (590:552, 480:410); here it is EIGHT with a grouped triple and no
// P4 at all. Nothing about the two shapes is shared, which is why the lengths
// below are counted off this chart rather than inherited.

// exAddrLen is the width of the whole address field: P1 (1) + P2 (2) + P3 (2)
// (990:1723, 990:1736, 990:1740).
const exAddrLen = 5

// exReadBodyLen is the length of an EX READ body — the frame bytes after "EX"
// and before the ';'. FIVE, the address alone, making the whole read frame
// eight bytes (990:1734-1736).
const exReadBodyLen = exAddrLen

// exAnswerP4 is the answer's P4 byte: "Response is always a space."
// (990:1741). It is a printed constant of the ANSWER direction, and the reason
// this fake never emits the '9' the SET direction admits.
const exAnswerP4 = ' '

// exP5Window is the width of P5 on this chart's Answer diagram: fifteen cells,
// positions 9 to 23, with the terminator at 24 (990:1738-1747). Erratum E19.
const exP5Window = 15

// exPadByte fills the cells of that window a narrower value leaves.
//
// ASSUMED — doc.go's register entry THE FIXED FORM'S PAD BYTE IS A SPACE. The
// diagram gives the window its width and never says what fills it; the byte
// chosen is the one this book defines "blank" as, "this setting is blank
// <0x20>" (990:4081-4082, the design's A6 for the MA0 grid), and it is the
// only candidate that cannot be read as a digit of the value itself — a '0'
// pad would make a three-digit menu's answer indistinguishable from a
// fifteen-digit one.
const exPadByte = " "

// isEXReadBody reports whether body is a syntactically valid EX read body: a
// menu type of 0 or 1 followed by four digits. It says nothing about whether
// that address names a row this radio has.
//
// NOTE WHAT THE EXACT-LENGTH CHECK ALSO EXCLUDES: a TS-590's or a TS-480's
// ten-byte EX read frame, and an FT-891's seven-byte one. A check written as
// "at least five digits" would answer a sibling's frame with a TS-990S menu
// value, which is the wrong answer given confidently — the class of mistake a
// shared "fake core" would have made structural (doc.go, THE HARD RULE).
func isEXReadBody(body []byte) bool {
	if len(body) != exReadBodyLen {
		return false
	}
	if body[0] != '0' && body[0] != '1' {
		return false
	}
	return allDigits(string(body))
}

// buildEXAnswer assembles an answer frame: the read's address, the printed P4
// space, P5 in its fifteen-wide window, and the terminator (990:1738-1747).
//
// A P5 AT OR ABOVE THE WINDOW'S WIDTH IS EMITTED AS IT STANDS, unpadded, so
// that WithEXSetting can script the over-wide answer the codec's parser must
// refuse. Nothing this fake answers of its own accord is ever that long: every
// projected width is at most fifteen (exinventory.go's maxWidth).
func buildEXAnswer(addr, p5 string) []byte {
	out := make([]byte, 0, 2+len(addr)+1+exP5Window+1)
	out = append(out, 'E', 'X')
	out = append(out, addr...)
	out = append(out, exAnswerP4)
	out = append(out, p5...)
	if n := exP5Window - len(p5); n > 0 {
		out = append(out, strings.Repeat(exPadByte, n)...)
	}
	out = append(out, ';')
	return out
}

// handleEX validates and answers an EX body (the frame bytes after "EX",
// before the trailing ';').
//
// READ ONLY. Any body that is not the printed eight-position read naming an
// address THIS CHART has draws "?;" with the state unchanged — which covers
// malformed bodies, a menu type outside the two printed values, addresses
// outside this row's own domain, and SET-SHAPED bodies alike (the read's five
// bytes followed by a P4 and a P5 payload is simply a too-long body to this
// handler).
//
// THE OUT-OF-INVENTORY REFUSAL IS ASSUMED — doc.go's register entry AN
// OUT-OF-INVENTORY EX ADDRESS ANSWERS "?;". What the chart DOES print is that
// an error occurs, three times over: "Entering a non-existing number causes an
// error to occur" for both P2 and P3 (990:1727, 990:1733) and "Entering a
// number that cannot be set also causes an error to occur" (990:1734-1735).
// What it does not print is WHICH error message, and "?;" is the error table's
// first cause — a syntactically correct command the transceiver cannot execute
// (990:108-113) — applied to a menu the radio has none of.
//
// MEMBERSHIP COMES FROM THE CHART'S OWN ROWS, via the projected inventory, not
// from the printed range. P2 and P3 each print a domain of "00 ~ 99"
// (990:1726, 990:1732) which no menu type comes close to filling; enforcing it
// separately would add a second, redundant authority over the same fact and
// would disagree with the inventory the moment the two were ever edited apart.
func (r *Radio) handleEX(body []byte) []byte {
	if !isEXReadBody(body) {
		return rejection
	}
	addr := string(body)
	r.mu.Lock()
	p5, ok := r.exSettings[addr]
	r.mu.Unlock()
	if !ok {
		return rejection // register entry AN OUT-OF-INVENTORY EX ADDRESS ANSWERS "?;"
	}
	return buildEXAnswer(addr, p5)
}
