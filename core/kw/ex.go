// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import "fmt"

// EX IS THE ONE VARIABLE-LENGTH FRAME IN THIS FAMILY, AND THIS CODEC BUILDS
// ONLY ITS READ.
//
// NO EX SET AND NO EX ANSWER IS EVER BUILT. The Set and the Answer share an
// identical wire shape — the same ten fixed bytes with P5 inserted before
// the terminator (590:543-560, 480:403-416) — so admitting the Set outbound
// would admit a captured answer being written back, and the outbound gate
// refuses the whole shape. That is core/cat's shipped EX policy applied
// here, and here it is not even a policy question yet: this milestone reads
// the menu surface and writes none of it, so there is nothing for a Set
// builder to serve.
//
// THE READ FRAME IS ALWAYS AT THE FULL THREE-DIGIT ADDRESS. Both books print
// ten positions with P1 occupying three of them (590:552, 480:410), and
// internal/extable registers all three Kenwood profiles as AddressSingle, so
// there is one width on every row and no second form for a builder to get
// wrong.

// The EX frame lengths.
const (
	// EXReadLen is "E X P1 P1 P1 P2 P2 P3 P4 ;" — ten bytes, fixed on both
	// radios (590:552, 480:410). It is exAnswerFixedBytes by another name
	// and by the same arithmetic; see MaxEXDigits (exdigits.go), which is
	// derived from it.
	EXReadLen = exAnswerFixedBytes
	// EXMinAnswerLen is the shortest EX ANSWER: the ten fixed bytes plus at
	// least one character of P5. A frame of exactly EXReadLen bytes is the
	// READ, which is not an answer to itself.
	EXMinAnswerLen = EXReadLen + 1
)

// Byte offsets (0-indexed) into an EX read or answer frame, with the books'
// 1-indexed positions beside them.
const (
	exPrefixOff = 0 // positions 1-2,  "EX"
	exAddrOff   = 2 // positions 3-5,  P1, the three-digit menu number
	exAddrLen   = 3
	exP2Off     = 5 // positions 6-7,  P2, "00: Always 00"
	exP2Len     = 2
	exP3Off     = 7 // position 8,     P3, "0: Always 0"
	exP4Off     = 8 // position 9,     P4, "0: Always 0"
	exP5Off     = 9 // position 10 onwards, P5 in an answer; ';' in a read
)

// The three printed constants of the EX read frame, named once because the
// builder emits them and the gate and the parser both require them
// (590:546-553, 480:402-407).
const (
	exP2Printed = "00"
	exP3Printed = '0'
	exP4Printed = '0'
)

// BuildEXRead builds the menu read for addr: "E X P1 P1 P1 P2 P2 P3 P4 ;",
// ten bytes with P2 "00" and P3 and P4 '0' (590:546-553, 480:402-407).
//
// IT KNOWS NOTHING OF MEMBERSHIP, AND THAT IS A DELIBERATE DIVISION rather
// than a gap. Which addresses a radio HAS is the per-row inventory's fact —
// 88 rows on the TS-590S, 100 on the TS-590SG, 61 on the TS-480 (A26) — and
// those three generated inventories live in core/kw/ts590 and core/kw/ts480,
// which import this package. A membership check here would be an import
// cycle; a Layout carrying its own copy of an inventory would be a second
// copy of a generated artefact, which is the drift this repository's
// staleness tests exist to catch. The caller reads its own inventory and
// asks for what is in it. EXAddress's own doc comment states the same
// division for the address type.
//
// WHAT IT DOES ENFORCE is the address FORM: EXAddress.Wire fails closed on a
// non-zero P2 or P3 — values that are not Kenwood addresses at all — and
// this builder turns that "" into a refusal rather than emitting a short
// frame.
func (l Layout) BuildEXRead(addr EXAddress) (Command, error) {
	if !l.Configured() {
		return Command{}, newParseError(nil, "EX read: this layout is unconfigured and describes no radio")
	}
	wire := addr.Wire()
	if wire == "" {
		return Command{}, newParseError(nil, "EX read: %v is not a Kenwood menu address — every Kenwood profile registers the AddressSingle form, whose P2 and P3 are zero, and rendering P1 alone from this value would discard what the caller supplied", addr)
	}
	if len(wire) != exAddrLen {
		return Command{}, newParseError([]byte(wire), "EX read: the address rendered %d bytes, and both books print three (590:552, 480:410)", len(wire))
	}
	frame := make([]byte, 0, EXReadLen)
	frame = append(frame, 'E', 'X')
	frame = append(frame, wire...)
	frame = append(frame, exP2Printed...)
	frame = append(frame, exP3Printed, exP4Printed, ';')
	if len(frame) != EXReadLen {
		return Command{}, newParseError(frame, "EX read: built %d bytes, want exactly %d (590:552, 480:410)", len(frame), EXReadLen)
	}
	return newCommand(frame), nil
}

// ParseEXAnswer decodes one EX answer against the inventory row that read it
// and returns P5 VERBATIM.
//
// THE WIDTH BOUND IS A19, AND IT IS A CEILING. Both books call P5 "variable
// length" and neither prints a ceiling anywhere (590:555-556, 480:409-411);
// A19 is the assumption that an answer never EXCEEDS the width the parameter
// list prints for that menu number, and its lift is an exhaustive sweep of
// the row's whole printed domain, because a ceiling is not lifted by a
// sample. So the bound is item.Digits, item.Digits comes from the caller's
// own transcribed chart, and a SHORTER answer is admitted: A19 claims a
// maximum and nothing else, and refusing a short answer would fail a session
// on a claim the register does not make.
//
// THE WHOLE ADDRESS IS THE CORRELATION KEY. Every one of a radio's menu
// addresses answers with a frame starting "EX", so an answer whose address
// field is not the one item names is a DIFFERENT menu's reply — still in
// flight, or arriving out of order — and returning its P5 would hand the
// caller one setting under another's name. PrefixLenMatcher's doc comment
// states the same obligation for the transport-level matcher; this is the
// codec-level half of it, and the two are independent defences rather than
// one repeated.
//
// P5 IS RETURNED VERBATIM, INCLUDING ANY TRAILING SPACES. Neither book
// states a padding rule for P5 — A1's rule is MR/MW's P16 and is scoped to
// that field — so trimming here would apply an assumption to a field the
// register does not cover. The 590SG's one free-text row is the case that
// matters: "up to 8 ASCII characters" (590:750), and what a radio puts in
// the unused ones is unprinted.
//
// THE INVENTORY ROW IS VALIDATED TOO. A zero Digits is a row that was never
// transcribed, and parsing against it would apply a ceiling of nothing; a
// Digits above MaxEXDigits describes an answer longer than this family's own
// maximum frame, which this family's own accumulator would discard as
// contamination.
func (l Layout) ParseEXAnswer(frame []byte, item EXItem) (string, error) {
	if !l.Configured() {
		return "", newParseError(frame, "EX answer: this layout is unconfigured and describes no radio, so no byte of this frame has a meaning to read")
	}
	if item.Digits < 1 || item.Digits > MaxEXDigits {
		return "", newParseError(frame, "EX answer: the inventory row for %v declares a printed width of %d, and this codec admits 1 to %d — a zero width is a row that was never transcribed, and a wider one describes an answer longer than this family's own %d-byte frame bound (A19)", item.Addr, item.Digits, MaxEXDigits, DefaultMaxFrame)
	}
	if len(frame) < EXMinAnswerLen {
		return "", newParseError(frame, "EX answer: the frame is %d bytes; the ten fixed bytes are followed by at least one character of P5, and a frame of exactly %d bytes is the READ (590:552, 480:410)", len(frame), EXReadLen)
	}
	if len(frame) > DefaultMaxFrame {
		return "", newParseError(frame, "EX answer: the frame is %d bytes, past this family's own %d-byte bound, which its accumulator would have discarded as contamination", len(frame), DefaultMaxFrame)
	}
	if string(frame[exPrefixOff:exPrefixOff+2]) != "EX" {
		return "", newParseError(frame, "EX answer: missing %q prefix, got %q", "EX", frame[exPrefixOff:exPrefixOff+2])
	}
	if frame[len(frame)-1] != ';' {
		return "", newParseError(frame, "EX answer: missing ';' terminator at position %d", len(frame))
	}

	got := string(frame[exAddrOff : exAddrOff+exAddrLen])
	for i, b := range []byte(got) {
		if b < '0' || b > '9' {
			return "", newParseError(frame, "EX answer: address byte %d is %q; P1 is three decimal digits on both radios (590:552, 480:410)", i+1, b)
		}
	}
	if want := item.Addr.Wire(); got != want {
		return "", newParseError(frame, "EX answer: this frame answers menu %s and the read asked for menu %s — every menu address answers with a frame starting \"EX\", so the whole address is what correlates an answer to its read (590:543-544, 480:401)", got, want)
	}

	if p2 := string(frame[exP2Off : exP2Off+exP2Len]); p2 != exP2Printed {
		return "", newParseError(frame, "EX answer: P2 is %q, and both books print %q there (590:546-547, 480:402-403)", p2, exP2Printed)
	}
	if frame[exP3Off] != exP3Printed {
		return "", newParseError(frame, "EX answer: P3 is %q, and both books print %q there (590:548-550, 480:404-405)", frame[exP3Off], exP3Printed)
	}
	if frame[exP4Off] != exP4Printed {
		return "", newParseError(frame, "EX answer: P4 is %q, and both books print %q there (590:551-553, 480:406-407)", frame[exP4Off], exP4Printed)
	}

	p5 := frame[exP5Off : len(frame)-1]
	if len(p5) > item.Digits {
		return "", newParseError(frame, "EX answer: P5 is %d characters and the parameter list prints %d for menu %s (%s) — A19 is that an answer never exceeds its printed width, and a wider one is refused rather than truncated", len(p5), item.Digits, item.Addr.Wire(), item.Name)
	}
	for i, b := range p5 {
		if b < 0x20 || b > 0x7e || b == ';' {
			return "", newParseError(frame, "EX answer: P5 byte %d is %#02x; the 480 forbids the control codes generally (480:127-129), an embedded ';' is a second frame to the radio's own parser, and A2's charset claim is bounded at 0x7E", i+1, b)
		}
	}
	return string(p5), nil
}

// exReadAddress returns the address field of a well-formed EX READ frame, or
// an error. It is the gate's own decoder (allowlist.go) and it is here
// rather than there so that the ten fixed bytes are read from one place in
// both directions.
func exReadAddress(frame []byte) (EXAddress, error) {
	if len(frame) != EXReadLen {
		return EXAddress{}, fmt.Errorf("an EX read is %d bytes and this frame is %d (590:552, 480:410)", EXReadLen, len(frame))
	}
	if string(frame[exPrefixOff:exPrefixOff+2]) != "EX" {
		return EXAddress{}, fmt.Errorf("missing %q prefix", "EX")
	}
	if frame[EXReadLen-1] != ';' {
		return EXAddress{}, fmt.Errorf("missing ';' terminator at position %d", EXReadLen)
	}
	field := string(frame[exAddrOff : exAddrOff+exAddrLen])
	n := 0
	for _, b := range []byte(field) {
		if b < '0' || b > '9' {
			return EXAddress{}, fmt.Errorf("the address field is %q, and P1 is three decimal digits", field)
		}
		n = n*10 + int(b-'0')
	}
	if string(frame[exP2Off:exP2Off+exP2Len]) != exP2Printed || frame[exP3Off] != exP3Printed || frame[exP4Off] != exP4Printed {
		return EXAddress{}, fmt.Errorf("P2/P3/P4 are %q, and both books print %q, %q and %q", frame[exP2Off:exP4Off+1], exP2Printed, string(exP3Printed), string(exP4Printed))
	}
	// THE ROUND TRIP IS THE CHECK, not the digit test above. EXAddress.P1 is
	// a uint8, so 256 and above is not an address this package can hold and
	// certainly not one BuildEXRead could have produced — a plain conversion
	// would fold "EX3000000;" onto address 044 and let the gate admit a
	// frame the builder cannot emit. Re-rendering and comparing is what
	// keeps the admitted set equal to the buildable one.
	addr := EXAddress{P1: uint8(n % 256)}
	if addr.Wire() != field {
		return EXAddress{}, fmt.Errorf("the address field is %q, and no EXAddress renders it: P1 is a uint8, so this package holds 000 to 255 and no Kenwood menu domain reaches even that far (590:543-544, 480:401)", field)
	}
	return addr, nil
}
