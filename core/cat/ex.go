// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import "fmt"

// exReadLen is the length of an EX read request for THIS DIALECT:
// "EX"(2) + address(d.EXAddressWidth()) + ";"(1). Reference: the EX
// grammar block's Read frame — "E X P1 P1 P2 P2 P3 P3 ;" (FT-710 manual
// extract line ~629) is 9 bytes; a four-digit family's is 7.
//
// It was a package const of 9 until the FT-891 Stage 0 seam, consulted
// THROUGH a Dialect receiver by validEXRead — the exact shape this package
// keeps eliminating: a bound taken from one radio and applied to every
// other. TestEveryDialect_FrameLengthsFollowTheAddressWidth holds all three
// lengths here to the width they derive from.
func (d Dialect) exReadLen() int { return 2 + d.EXAddressWidth() + 1 }

// exAnswerMinLen is the smallest EX Answer frame THIS DIALECT can send:
// "EX"(2) + address(d.EXAddressWidth()) + P4(1) + ";"(1). Reference: the EX
// grammar block's Answer frame ("E X P1 P1 P2 P2 P3 P3 P4 ~ P4 ;", manual
// extract line ~629). One byte is the narrowest P4 a menu item can have —
// the FT-710's narrowest Table 2 Digits value, e.g. CAT-1 RATE — and a
// dialect whose own narrowest item is wider is not made unsafe by the
// slack: the P4 body is returned verbatim and no width policy is enforced
// here (see Dialect.ParseEXAnswer).
//
// The UPPER bound is also per-dialect: Dialect.exAnswerMaxLen, over
// Dialect.exP4MaxBytes (dialect.go).
func (d Dialect) exAnswerMinLen() int { return 2 + d.EXAddressWidth() + 1 + 1 }

// exAnswerMaxLen is the largest EX Answer frame THIS DIALECT can send:
// "EX"(2) + address(d.EXAddressWidth()) + P4(d.exP4MaxBytes()) + ";"(1).
// Both variable terms are derived from this dialect's own data, so a radio
// whose address field is narrower or whose menu carries a field wider than
// the FT-710's is bounded by itself.
func (d Dialect) exAnswerMaxLen() int {
	return 2 + d.EXAddressWidth() + d.exP4MaxBytes() + 1
}

// BuildEXRead builds this dialect's EX read frame for addr — 9 bytes under
// EXAddressTriple, 7 under EXAddressPair. Reference: the EX grammar
// block's Read frame (manual extract line ~629). The only
// validation is membership of THIS DIALECT'S inventory
// (d.KnownEXAddress) — never a numeric range check on P1/P2/P3, mirroring
// Dialect.NewEXAddress/Dialect.ParseEXAddress in exinventory.go. This rejects both the zero value and the P1==05
// grammar/Table-2 anomaly address (see Dialect.KnownEXAddress's doc comment
// in dialect.go): the grammar block names P1 "01 - 04, 05" but Table 2
// has no P1==05 group, so no (05,*,*) triple is ever a member. M8c put two
// such addresses to a real radio and both were rejected with "?;", which
// supports that reading without surveying the whole P1=05 space.
//
// THE NON-MEMBER REFUSAL REPORTS THE WIRE RENDER under EXAddressTriple, so
// that core/cat/testdata/frame-corpus.golden line 357 — which pins this
// refusal's input bytes verbatim ("000000") — stays byte-identical, per
// the milestone's standing claim that no existing golden moves through
// Stage 0. Under EXAddressPair the wire render drops P3 (EXWire renders
// only P1 and P2 for that form), so a Pair non-member refusal reports the
// debug String() form instead — the only rendering that names all three
// components. TestBuildEXRead_UsesThisDialectsWidth pins both sides: the
// Triple refusal's reported input unchanged, the Pair refusal's naming all
// three components.
func (d Dialect) BuildEXRead(addr EXAddress) (Command, error) {
	wire := d.EXWire(addr)
	if !d.KnownEXAddress(addr) {
		reported := wire
		if d.exAddrForm == EXAddressPair {
			reported = addr.String()
		}
		return Command{}, newParseError([]byte(reported), "EX: address is not a known Table 2 member")
	}
	frame := make([]byte, 0, d.exReadLen())
	frame = append(frame, 'E', 'X')
	frame = append(frame, wire...)
	frame = append(frame, ';')
	return newCommand(frame), nil
}

// ParseEXAnswer parses an EX Answer frame ("EX" + this dialect's address
// field, six digits or four + a raw P4 body of 1 to d.exP4MaxBytes() bytes
// + ";", reference: the EX grammar block's Answer frame, manual extract
// line ~629) and returns the address and the raw P4 body.
//
// THE LENGTH BOUND IS THIS DIALECT'S OWN, derived from its inventory's
// widest Digits (dialect.go's maxEXP4Bytes). It was a package const until
// M9b's fix wave, when Codex found it: a bound taken from the FT-710's
// twelve-byte Text items, read through every dialect's receiver, would
// have made a radio with a wider menu field reject its own valid answers.
// For the FT-710 the derived bound is the same 12, so nothing about this
// parser's FT-710 behaviour changed.
//
// SHAPE (total length bounds, "EX" prefix, ';' terminator, exactly
// d.EXAddressWidth() ASCII digits in the address field) and MEMBERSHIP of
// the address in THIS
// DIALECT'S inventory are both strictly enforced, via d.ParseEXAddress
// applied to the address
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
func (d Dialect) ParseEXAnswer(frame []byte) (EXAddress, string, error) {
	// The form check comes FIRST, before the length window, so that a
	// dialect with no declared form is refused for the reason that is true
	// of it — it has no address field — rather than by a length window that
	// a zero width has collapsed. Only the zero Dialect reaches it: V12
	// refuses a formless config.
	width := d.EXAddressWidth()
	if width == 0 {
		return EXAddress{}, "", newParseError(frame, "EX answer: this dialect declares no EXAddressForm, so it has no address field")
	}
	minLen, maxLen := d.exAnswerMinLen(), d.exAnswerMaxLen()
	if len(frame) < minLen || len(frame) > maxLen {
		return EXAddress{}, "", newParseError(frame, fmt.Sprintf("EX answer must be %d-%d bytes", minLen, maxLen))
	}
	if frame[0] != 'E' || frame[1] != 'X' {
		return EXAddress{}, "", newParseError(frame, "EX answer missing \"EX\" prefix")
	}
	if frame[len(frame)-1] != ';' {
		return EXAddress{}, "", newParseError(frame, "EX answer missing ';' terminator")
	}
	addr, err := d.ParseEXAddress(string(frame[2 : 2+width]))
	if err != nil {
		return EXAddress{}, "", newParseError(frame, "EX answer: invalid or unknown address field")
	}
	raw := string(frame[2+width : len(frame)-1])
	return addr, raw, nil
}
