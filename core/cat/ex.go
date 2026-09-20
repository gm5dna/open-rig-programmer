// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"fmt"
	"strconv"
	"strings"
)

// exReadLen is the length of an EX read request for THIS DIALECT:
// "EX"(2) + address(d.EXAddressWidth()) + ";"(1). Reference: the EX
// grammar block's Read frame — "E X P1 P1 P2 P2 P3 P3 ;" (FT-710 manual
// extract line ~629) is 9 bytes under EXAddressTriple; a four-digit
// family's is 7 under EXAddressPair; a three-digit family's is 6 under
// EXAddressSingle.
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
// EXAddressTriple, 7 under EXAddressPair, 6 under EXAddressSingle.
// Reference: the EX grammar
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
// THE NON-MEMBER REFUSAL REPORTS THE WIRE RENDER ONLY UNDER EXAddressTriple,
// where the six-digit field carries all three components. Reporting it there
// keeps core/cat/testdata/frame-corpus.golden line 357 — which pins this
// refusal's input bytes verbatim ("000000") — byte-identical, per the
// milestone's standing claim that no existing golden moves through Stage 0.
//
// EVERY LOSSY FORM REPORTS THE DEBUG String() FORM INSTEAD, which is the
// only rendering that names all three components: EXWire drops P3 under
// EXAddressPair and drops both P2 and P3 under EXAddressSingle. The Single
// form's loss is the sharper one, because a three-digit render of a
// non-member can BE a member's wire — under an inventory holding 001, the
// address (01,03,07) reported `input="001"` and named the very item it had
// just refused (Codex third seat, LOW C-L1). The test that meant to cover
// this asserted only that an error existed. The condition is now the
// form's lossiness rather than a named form, so a fourth form added later
// is safe by default. TestBuildEXRead_UsesThisDialectsWidth pins the
// Triple and Pair frames; TestBuildEXRead_SingleIsSixBytesAndGateAdmissible
// pins the Single one.
func (d Dialect) BuildEXRead(addr EXAddress) (Command, error) {
	wire := d.EXWire(addr)
	if !d.KnownEXAddress(addr) {
		reported := wire
		if d.exAddrForm != EXAddressTriple {
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

// exSetP4OK is the ONE predicate the write gate's every consumer calls —
// CanSetEX, BuildEXSet and validEXRead's Set arm (allowlist.go) — so "may I
// Set this address at all" and "is this particular value legal" can never
// drift apart (Opus-3 minor 8: the Width != 0 guard drifted once already
// when more than one place restated it).
//
// A missing d.exWrite entry, or a present one with Width == 0 (Session W
// has not characterised this address, or never will because it was denied
// or held before construction — dialect.go's buildFT710ExWrite), answers
// false before anything else runs: no zero sentinel renders as writable
// anywhere (spec A1). p4 == nil asks only "may I Set here at all" —
// CanSetEX's own probe. A non-nil p4 additionally requires it to be
// exactly Width bytes and, parsed as a decimal (sign included), inside the
// descriptor's Domain.
func (d Dialect) exSetP4OK(a EXAddress, p4 []byte) bool {
	desc, ok := d.exWrite[a]
	if !ok || desc.Width == 0 {
		return false
	}
	if p4 == nil {
		return true
	}
	if len(p4) != desc.Width {
		return false
	}
	v, err := strconv.Atoi(strings.TrimSpace(string(p4)))
	if err != nil {
		return false
	}
	return desc.Domain.Contains(v)
}

// CanSetEX reports whether addr is writable on this dialect right now:
// admitted (not denied, not held) AND characterised (a non-zero
// ObservedSetWidth — Session W has produced a row for it). It is
// exSetP4OK's own "may I Set at all" question, asked with no P4 to judge —
// CanSetEX does not re-implement the guard, it calls the one that owns it.
func (d Dialect) CanSetEX(addr EXAddress) bool {
	return d.exSetP4OK(addr, nil)
}

// BuildEXSet builds this dialect's EX Set frame for addr carrying value —
// value already rendered to the write descriptor's own width (Session W's
// ObservedSetWidth), sign included where the domain is Signed. It is the
// ONE EX Set builder (allowlist.go:25-30,431-433's "cannot drift apart"
// rule): it calls exSetP4OK exactly as validEXRead's Set arm does, rather
// than re-checking width or domain membership itself.
//
// One call covers every refusal reason pre-wire: unknown address, denied,
// held, uncharacterised (Width == 0), wrong width, and out-of-domain value
// all fail exSetP4OK and are reported alike — task (d)'s WriteSetting
// relies on that to refuse before any bytes reach the transport.
func (d Dialect) BuildEXSet(addr EXAddress, value string) (Command, error) {
	p4 := []byte(value)
	if !d.exSetP4OK(addr, p4) {
		return Command{}, newParseError([]byte(value), "EX: address is not writable, or value is outside its domain")
	}
	frame := make([]byte, 0, 2+d.EXAddressWidth()+len(p4)+1)
	frame = append(frame, 'E', 'X')
	frame = append(frame, d.EXWire(addr)...)
	frame = append(frame, p4...)
	frame = append(frame, ';')
	return newCommand(frame), nil
}

// DialectWithEXWriteForTest returns a copy of d whose entire write table
// is replaced by one entry, addr -> {Domain: domain, Width: width} — a
// cross-package test seam for driving CanSetEX/BuildEXSet's accepting
// path from OUTSIDE this package, mirroring this package's own local-copy
// technique (d := FT710; d.exWrite = map[...]{...}, e.g.
// TestBuildEXSet_AcceptsCharacterisedAddress, ex_test.go) for a caller
// that cannot spell an exWriteDescriptor literal itself: exWrite is
// unexported, so a package such as core/driver/ft710, testing
// WriteSetting before Session W exists (spec §3,
// table2-write-observed.csv is empty), has no other way to construct a
// Dialect whose write table admits anything.
//
// An export_test.go alias cannot do this job: a _test.go file's exported
// names are linked only into ITS OWN package's test binary, never into an
// importing package's (core/driver/ft710 imports core/cat as an ordinary
// dependency, built without core/cat's test files — verified against go's
// own behaviour before adding this, rather than assumed).
//
// d itself is never mutated — Dialect is a value type, so d is already
// this function's own copy — and no registered dialect literal (FT710 or
// any MustNewDialect model) is ever passed through here in production:
// every real Dialect value in this codebase is built once, at init, from
// a fixed literal or config, never reassigned at runtime. Test-only in
// effect, not in enforcement: it is exported like any other API, but the
// one thing it is FOR — constructing a Dialect the outbound gate would
// then trust — has no production call site to reach it through.
func DialectWithEXWriteForTest(d Dialect, addr EXAddress, domain Domain, width int) Dialect {
	d.exWrite = map[EXAddress]exWriteDescriptor{addr: {Domain: domain, Width: width}}
	return d
}

// ParseEXAnswer parses an EX Answer frame ("EX" + this dialect's address
// field, six digits, four or three + a raw P4 body of 1 to d.exP4MaxBytes() bytes
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
