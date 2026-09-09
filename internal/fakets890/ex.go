// SPDX-License-Identifier: GPL-3.0-or-later

package fakets890

// This file is fakets890's own, independent model of the EX (MENU) command on
// the TS-890S — READ ONLY, exactly as internal/fakets590 models the 590 pair's:
// the book documents a Set form and this fake does not implement it (doc.go's
// "What this fake deliberately does NOT model").
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
// # EX GRAMMAR (890:1896-1922)
//
// Read frame (8 bytes): "E X P1 P2 P2 P3 P3 ;" — a ONE-digit menu type in P1
// ("0: Menu", "1: Advanced Menu", 890:1899-1900), a TWO-digit category in P2
// ("00 ~ 99", 890:1903) and a TWO-digit item in P3 ("00 ~ 99", 890:1908).
// THERE IS NO P4 ON THE READ (890:1907).
//
// Answer frame: the read's seven address positions, then P4 spliced in at
// position 8 and P5 from position 9 to the floating terminator
// (890:1909-1913). P4 on an answer is a SPACE and nothing else — "Response is
// always a space." (890:1915), which is printed and is not an assumption.
//
// Set frame: the same shape as the answer, with P4 either a space or '9'
// (890:1913-1914) — NOT modelled; see handleEX.
//
// THE FRAME SHAPE IS THIS ROW'S OWN AND NOT core/kw's. That package's EX read
// is TEN bytes with a three-digit menu number and a P2/P3/P4 of printed
// constants (590:552, 480:410); here it is EIGHT with a grouped triple and no
// P4 at all. Nothing about the two shapes is shared, which is why the lengths
// below are counted off this chart rather than inherited.

// exAddrLen is the width of the whole address field: P1 (1) + P2 (2) + P3 (2)
// (890:1900, 890:1907, 890:1910).
const exAddrLen = 5

// exReadBodyLen is the length of an EX READ body — the frame bytes after "EX"
// and before the ';'. FIVE, the address alone, making the whole read frame
// eight bytes (890:1907).
const exReadBodyLen = exAddrLen

// exAnswerP4 is the answer's P4 byte: "Response is always a space."
// (890:1915). It is a printed constant of the ANSWER direction, and the reason
// this fake never emits the '9' the SET direction admits.
const exAnswerP4 = ' '

// isEXReadBody reports whether body is a syntactically valid EX read body: a
// menu type of 0 or 1 followed by four digits. It says nothing about whether
// that address names a row this radio has.
//
// NOTE WHAT THE EXACT-LENGTH CHECK ALSO EXCLUDES: a TS-590's or a TS-480's
// ten-byte EX read frame, and an FT-891's seven-byte one. A check written as
// "at least five digits" would answer a sibling's frame with a TS-890S menu
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
// space, P5, and the floating terminator (890:1909-1913).
func buildEXAnswer(addr, p5 string) []byte {
	out := make([]byte, 0, 2+len(addr)+1+len(p5)+1)
	out = append(out, 'E', 'X')
	out = append(out, addr...)
	out = append(out, exAnswerP4)
	out = append(out, p5...)
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
// an error occurs, twice over: "Entering a non-existing number causes an error
// to occur" for both P2 and P3 (890:1904, 890:1909) and "Entering a number
// that cannot be set also causes an error to occur" (890:1910-1911). What it
// does not print is WHICH error message, and "?;" is the error table's first
// cause — a syntactically correct command the transceiver cannot execute
// (890:106-112) — applied to a menu the radio has none of.
//
// THE FOUR ADDRESSES THE PROJECTION EXCLUDES ANSWER "?;" THROUGH THIS SAME
// PATH, and that is the honest shape: the chart prints them with the body
// "Does not correspond to a command" (890:2273-2280), so they name no field an
// EX frame could read, and the sentence above — "Entering a number that cannot
// be set also causes an error to occur" — is the chart's own word for exactly
// that case.
//
// MEMBERSHIP COMES FROM THE CHART'S OWN ROWS, via the projected inventory, not
// from the printed range. P2 and P3 each print a domain of "00 ~ 99"
// (890:1903, 890:1908) which no menu type comes close to filling; enforcing it
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
