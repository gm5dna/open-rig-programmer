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
type EXAddress struct {
	P1, P2, P3 uint8
}

// Wire renders the address as its six-digit CAT field: two zero-padded
// decimal digits per component, e.g. EXAddress{1,2,3} -> "010203". Reference:
// the EX grammar block's "E X P1 P1 P2 P2 P3 P3" layout.
func (a EXAddress) Wire() string {
	return fmt.Sprintf("%02d%02d%02d", a.P1, a.P2, a.P3)
}

// String returns the same six-digit wire form as Wire.
func (a EXAddress) String() string {
	return a.Wire()
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
	// Digits is the manual's Digits column: 1..4 for a numeric field, or 12
	// for a Text item. A signed field counts its sign in the width (e.g. the
	// manual's "-20".."+10" is 3).
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

// ParseEXAddress parses a six-ASCII-digit wire field ("010203") into an
// address that is a member of THIS DIALECT'S inventory. It performs only a
// shape check (exactly six digits) followed by a membership lookup; it
// applies NO numeric range logic to the parsed components. A malformed
// shape or a non-member address yields a *ParseError.
func (d Dialect) ParseEXAddress(wire string) (EXAddress, error) {
	if len(wire) != 6 {
		return EXAddress{}, newParseError([]byte(wire), "EX address must be exactly six digits")
	}
	for i := 0; i < 6; i++ {
		if wire[i] < '0' || wire[i] > '9' {
			return EXAddress{}, newParseError([]byte(wire), "EX address must be six ASCII digits")
		}
	}
	p1 := int(wire[0]-'0')*10 + int(wire[1]-'0')
	p2 := int(wire[2]-'0')*10 + int(wire[3]-'0')
	p3 := int(wire[4]-'0')*10 + int(wire[5]-'0')
	return d.NewEXAddress(p1, p2, p3)
}
