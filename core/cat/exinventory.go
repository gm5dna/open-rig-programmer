// SPDX-License-Identifier: GPL-3.0-or-later

package cat

//go:generate go run github.com/gm5dna/open-rig-programmer/internal/extable/gen -profile ft710

import "fmt"

// EXAddress is one FT-710 menu/EX address: the (P1,P2,P3) triple carried by
// an EX command. Reference: the manual's EX grammar block (manual extract
// line ~629) and Table 2 "MENU Chart" (lines ~644-915), transcribed into
// table2.csv. The zero value is NOT a member of the inventory; construct
// addresses only via NewEXAddress or ParseEXAddress, both of which validate
// membership.
//
// It has NO wire rendering of its own. How many digits the field carries is
// a fact of the RADIO, not of the address — see EXAddressForm and
// wireEXAddress — so a caller renders through Dialect.EXWire.
type EXAddress struct {
	P1, P2, P3 uint8
}

// wireEXAddress renders a as the CAT address field of a dialect declaring
// form: six zero-padded digits under EXAddressTriple ("E X P1 P1 P2 P2 P3
// P3", the FT-710/FTdx10/FTdx101 grammar blocks), four under EXAddressPair
// — a's own (P1,P2) components with P3 dropped; that naming is this
// package's, not necessarily the radio's own — see EXAddressPair's doc
// comment (dialectconfig.go) for the FT-891 naming caveat.
//
// IT IS THE ONLY PLACE AN ADDRESS BECOMES WIRE DIGITS. Until this seam that
// place was EXAddress.Wire(), a method on the ADDRESS — which carries no
// family — so every caller rendered six digits whatever dialect it was
// working for, and a four-digit radio was inexpressible. Routing every
// render through the form serves both callers that have one: Dialect.EXWire
// for anything holding a Dialect, and validateEXItems, which has
// cfg.EXAddressForm in scope before any Dialect exists.
//
// Dropping P3 under Pair is safe ONLY because V12 requires every Pair
// member's P3 to be zero (dialectvalidate.go); the rule and this render are
// the two halves of one fact. TestWireEXAddress_RendersPerForm pins both
// branches.
//
// An unspecified form renders "": a dialect that never declared one has no
// wire address at all, NewDialect refuses to build such a dialect, and the
// only receiver that reaches this branch is the hand-built zero Dialect,
// which is inert by design. Returning "" rather than a plausible six digits
// is what lets Dialect.EXAddressWidth measure this render instead of
// keeping a second table of widths.
func wireEXAddress(form EXAddressForm, a EXAddress) string {
	switch form {
	case EXAddressTriple:
		return fmt.Sprintf("%02d%02d%02d", a.P1, a.P2, a.P3)
	case EXAddressPair:
		return fmt.Sprintf("%02d%02d", a.P1, a.P2)
	default:
		return ""
	}
}

// String returns a NON-WIRE debug rendering, "P1=08 P2=03 P3=00".
//
// It used to return the six-digit wire form, and stopped when Wire() was
// deleted: an address alone cannot know how many digits its family's field
// has, so any wire-shaped String() would be the FT-710's answer given to
// every radio. The debug form is deliberately unmistakable — it carries
// bytes no address field may hold, so a debug print can never be read back
// as one, and it names all THREE components, which the four-digit wire form
// cannot. TestEXAddressString_IsNotAWireField pins both properties, the
// second against every configured dialect's own ParseEXAddress.
func (a EXAddress) String() string {
	return fmt.Sprintf("P1=%02d P2=%02d P3=%02d", a.P1, a.P2, a.P3)
}

// EXItem is one transcribed Table 2 row: an address plus the manual's
// labels, function name, and field width. The manual's P4
// parameter-description text is deliberately NOT carried here — it lives only
// in table2.csv as an audit trail — nor is the manual line reference (kept as
// a comment in the generated file).
type EXItem struct {
	// Addr is the (P1,P2,P3) EX address.
	Addr EXAddress
	// P1Label is the menu label (manual P1 column), e.g. "RADIO SETTING".
	P1Label string
	// P2Label is the subgroup label (manual P2 column), e.g. "MODE SSB".
	P2Label string
	// Name is the manual's Function column, verbatim.
	Name string
	// Digits is the manual's Digits column: for a numeric field, whatever
	// width that radio's own chart prints — 1..4 across the FT-710, FTdx10
	// and FTdx101 inventories, and a registered inventory is free to be
	// wider; or the model's text width (12 on all three) for a Text item. A
	// signed field counts its sign in the width (e.g. the manual's
	// "-20".."+10" is 3). The per-model bounds are the profile's, enforced
	// by internal/extable's CSV validator, not a fact of this type.
	//
	// It is also the source of a dialect's P4 answer-length bound: the
	// largest Digits over an inventory sets that dialect's exAnswerMaxLen
	// (dialect.go's maxEXP4Bytes). Those ranges describe the FT-710's Table
	// 2 and are enforced for it by internal/extable's CSV validator; another
	// radio's inventory is free to be wider, and its parser widens with it.
	Digits int
	// Text marks the six free-text items (MY CALL + 5x PRESET NAME), whose
	// P4 is "Up to 12 characters" rather than an enumerated/numeric range.
	Text bool
	// ObservedReadWidth is the P4 wire width this address ANSWERED with
	// during the M8c read-characterisation: two successive sweeps of one UK
	// FT-710, CAT ID 0800, firmware V01-12, in one configuration, on
	// 24/07/2026 (docs/hardware-notes.md, core/cat/table2-observed.csv).
	//
	// It is hardware evidence, READ DIRECTION ONLY. No EX Set frame was
	// probed at M8c, so this width must NOT be used to size one: read and
	// Set widths are not known to agree, and no Set width policy exists —
	// the menu surface is read-only for v1.x by the M8d decision of
	// 25/07/2026 (docs/menu-write-decision.md), which cites precisely this
	// gap as one of its reasons. It differs from the manual's Digits column
	// for exactly one address — see table2-corrections.csv. Zero means no
	// observation.
	ObservedReadWidth int
	// ObservedReadShape classifies that answer: "numeric", "signed" (an
	// explicit leading '+'/'-' counted inside the width) or "text" (the six
	// 12-byte free-text items). Same evidence and same read-only scope as
	// ObservedReadWidth. Empty means no observation.
	ObservedReadShape string
}

// NewEXAddress returns the member EXAddress for the decimal triple
// (p1,p2,p3) in THIS DIALECT'S inventory, or a *ParseError if that triple
// is not a member of it. Validation is membership-only: there is no numeric
// range check on the components, so any non-member triple — including
// negative or oversized values — is rejected by the same lookup miss.
//
// It consults d.exByTriple, the dialect's own index (dialect.go), never the
// package-level one: membership is dialect data, and a second radio's menu
// inventory is a different table, not a different function. A zero Dialect
// has an empty index and is therefore a member of nothing.
func (d Dialect) NewEXAddress(p1, p2, p3 int) (EXAddress, error) {
	if a, ok := d.exByTriple[[3]int{p1, p2, p3}]; ok {
		return a, nil
	}
	return EXAddress{}, newParseError([]byte(fmt.Sprintf("%d,%d,%d", p1, p2, p3)), "EX address is not a known Table 2 member")
}

// ParseEXAddress parses THIS DIALECT'S wire address field — six ASCII
// digits ("010203") under EXAddressTriple, four ("0803") under
// EXAddressPair — into an address that is a member of its inventory. It
// performs only a shape check (exactly the declared width, all ASCII
// digits) followed by a membership lookup; it applies NO numeric range
// logic to the parsed components. A malformed shape or a non-member address
// yields a *ParseError.
//
// The two widths carry SEPARATE refusal sentences rather than one composed
// from a number. The six-digit spellings are shipped text, pinned verbatim
// in core/cat/testdata/parser-corpus.golden, and had to survive this change
// byte for byte; the four-digit ones are their counterparts.
// TestParseEXAddress_RefusalTextNamesTheFormsWidthInWords pins all four.
//
// A dialect with no declared form refuses every field, and says so: it has
// no address width, so there is no shape to check. That branch is
// reachable only from the zero Dialect (V12 refuses a formless config), and
// the refusal still names the address, which is what
// TestEveryDialect_EXAnswerBoundIsWellOrdered requires of every
// inventory-less receiver.
func (d Dialect) ParseEXAddress(wire string) (EXAddress, error) {
	switch d.exAddrForm {
	case EXAddressTriple:
		if len(wire) != 6 {
			return EXAddress{}, newParseError([]byte(wire), "EX address must be exactly six digits")
		}
		if !allASCIIDigits(wire) {
			return EXAddress{}, newParseError([]byte(wire), "EX address must be six ASCII digits")
		}
		return d.NewEXAddress(twoDigitsAt(wire, 0), twoDigitsAt(wire, 2), twoDigitsAt(wire, 4))
	case EXAddressPair:
		if len(wire) != 4 {
			return EXAddress{}, newParseError([]byte(wire), "EX address must be exactly four digits")
		}
		if !allASCIIDigits(wire) {
			return EXAddress{}, newParseError([]byte(wire), "EX address must be four ASCII digits")
		}
		// P3 is not on the wire, and V12 has already required every member
		// of a Pair inventory to have P3 == 0, so 0 is the only value the
		// lookup below can match — not a default standing in for an absent
		// datum.
		return d.NewEXAddress(twoDigitsAt(wire, 0), twoDigitsAt(wire, 2), 0)
	default:
		return EXAddress{}, newParseError([]byte(wire), "EX address: this dialect declares no EXAddressForm, so it has no address field")
	}
}

// allASCIIDigits reports whether every byte of s is '0'..'9'.
func allASCIIDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// twoDigitsAt reads the two-digit decimal component at offset i. Callers
// have already established that s is all digits and long enough.
func twoDigitsAt(s string, i int) int {
	return int(s[i]-'0')*10 + int(s[i+1]-'0')
}
