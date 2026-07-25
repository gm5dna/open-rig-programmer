// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import "fmt"

// exReadLen is the fixed length of an EX read request: "EX" + 6-digit
// address + ";". Reference: the EX grammar block's Read frame ("E X P1 P1
// P2 P2 P3 P3 ;", manual extract line ~629), 9 bytes.
const exReadLen = 9

// exAnswerMinLen/exAnswerMaxLen bound an EX Answer frame's total length:
// "EX"(2) + address(6) + P4(1-exP4MaxBytes) + ";"(1). Reference: the EX
// grammar block's Answer frame ("E X P1 P1 P2 P2 P3 P3 P4 ~ P4 ;", manual
// extract line ~629), with the P4 width bounded below by 1 (the narrowest
// Table 2 Digits value, e.g. CAT-1 RATE) and above by exP4MaxBytes (the
// widest, the six Text items).
const (
	exAnswerMinLen = 2 + 6 + 1 + 1
	exAnswerMaxLen = 2 + 6 + exP4MaxBytes + 1
)

// BuildEXRead builds the 9-byte EX read frame for addr. Reference: the EX
// grammar block's Read frame (manual extract line ~629). The only
// validation is Table 2 membership (KnownEXAddress) — never a numeric
// range check on P1/P2/P3, mirroring NewEXAddress/ParseEXAddress in
// exinventory.go. This rejects both the zero value and the P1==05
// grammar/Table-2 anomaly address (see KnownEXAddress's doc comment in
// exinventory.go): the grammar block names P1 "01 - 04, 05" but Table 2
// has no P1==05 group, so no (05,*,*) triple is ever a member. M8c put two
// such addresses to a real radio and both were rejected with "?;", which
// supports that reading without surveying the whole P1=05 space.
func BuildEXRead(addr EXAddress) (Command, error) {
	if !KnownEXAddress(addr) {
		return Command{}, newParseError([]byte(addr.Wire()), "EX: address is not a known Table 2 member")
	}
	frame := make([]byte, 0, exReadLen)
	frame = append(frame, 'E', 'X')
	frame = append(frame, addr.Wire()...)
	frame = append(frame, ';')
	return newCommand(frame), nil
}

// ParseEXAnswer parses an EX Answer frame ("EX" + 6-digit address + a
// 1-to-exP4MaxBytes-byte raw P4 body + ";", reference: the EX grammar
// block's Answer frame, manual extract line ~629) and returns the address
// and the raw P4 body.
//
// SHAPE (total length bounds, "EX" prefix, ';' terminator, six ASCII
// digits in the address field) and Table 2 MEMBERSHIP of the address are
// both strictly enforced, via ParseEXAddress applied to the address
// field — mirroring ParseMCAnswer's precedent of applying mcValid to the
// answer, not just the set/read direction (mc.go). One consequence: a
// frame answering at the phantom P1==05 address the grammar block names
// but Table 2 does not enumerate would be rejected here as an unknown
// address. M8c probed two such addresses below this parser, at the
// raw-frame level, and the radio rejected both ("?;") — so the restriction
// has supporting evidence, not merely the manual's ambiguity.
//
// The P4 body itself is returned VERBATIM: no trimming, no typed value
// model, no width/charset re-validation against the item's Table 2 Digits
// column. M8c gives that policy read-direction support rather than leaving
// it assumed — in the scope that session had, two successive sweeps of one
// UK radio on one firmware in one configuration (docs/hardware-notes.md):
// every one of the 296 addresses answered a P4 whose width
// matched the transcribed Digits column, save one where the MANUAL is
// wrong (01 03 21 TONE FREQ answers 3 bytes, not 2 — see
// table2-corrections.csv), and the six text items answered a fixed 12
// bytes, right-space-padded, exactly as MT's hardware-confirmed tag field
// does (mt.go's ParseMTAnswer; docs/hardware-notes.md §MT short-form
// answer). Verbatim return is therefore the right parse: re-validating
// width here would have rejected the radio's own honest TONE FREQ answer.
// Trimming, width checks and a typed value model remain M8e's business,
// and any Set-direction width policy needs M8f evidence — none of the
// above is evidence about what the radio ACCEPTS.
func ParseEXAnswer(frame []byte) (EXAddress, string, error) {
	if len(frame) < exAnswerMinLen || len(frame) > exAnswerMaxLen {
		return EXAddress{}, "", newParseError(frame, fmt.Sprintf("EX answer must be %d-%d bytes", exAnswerMinLen, exAnswerMaxLen))
	}
	if frame[0] != 'E' || frame[1] != 'X' {
		return EXAddress{}, "", newParseError(frame, "EX answer missing \"EX\" prefix")
	}
	if frame[len(frame)-1] != ';' {
		return EXAddress{}, "", newParseError(frame, "EX answer missing ';' terminator")
	}
	addr, err := ParseEXAddress(string(frame[2:8]))
	if err != nil {
		return EXAddress{}, "", newParseError(frame, "EX answer: invalid or unknown address field")
	}
	raw := string(frame[8 : len(frame)-1])
	return addr, raw, nil
}
