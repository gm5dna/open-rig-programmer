// SPDX-License-Identifier: GPL-3.0-or-later

package cat

//go:generate go run github.com/gm5dna/open-rig-programmer/internal/extable/gen -csv table2.csv -out exinventory_gen.go

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

// exP4MaxBytes is the maximum P4 field width over the whole inventory: 12,
// the width of the six Text items. A test pins it equal to the largest
// Digits value returned by EXItems, so it can never silently drift from the
// data.
const exP4MaxBytes = 12

// exMembers is the membership set keyed by the concrete EXAddress; exByTriple
// maps a decimal (P1,P2,P3) triple to its member address. Both are built once
// at package init from the generated inventory. exByTriple lets NewEXAddress
// and ParseEXAddress test membership purely by lookup — no independent
// numeric range logic on P1/P2/P3 — so out-of-range or negative inputs simply
// miss the map rather than being range-checked.
//
// exMembers ITSELF IS NO LONGER READ as of Task 53: KnownEXAddress now
// delegates to FT710.KnownEXAddress, which consults the Dialect's own
// exMembers FIELD of the same name, not this package var. This one is kept
// only because exByTriple's init loop still builds both together, and is
// removed alongside the delegates in Task 55. Do not reach for it from
// inside a Dialect method — that would be exactly the global-not-receiver
// bug this milestone exists to prevent (codex review Minor-5).
var (
	exMembers  map[EXAddress]bool
	exByTriple map[[3]int]EXAddress
)

func init() {
	exMembers = make(map[EXAddress]bool, len(exItemsGen))
	exByTriple = make(map[[3]int]EXAddress, len(exItemsGen))
	for _, it := range exItemsGen {
		exMembers[it.Addr] = true
		exByTriple[[3]int{int(it.Addr.P1), int(it.Addr.P2), int(it.Addr.P3)}] = it.Addr
	}
}

// KnownEXAddress reports whether a is a member of the transcribed Table 2
// inventory. Membership is descriptor-based: an address is valid iff it
// appears in table2.csv, never because its components fall in some numeric
// range.
//
// P1 ANOMALY — evidence at M8c (24/07/2026): the EX grammar block (manual
// extract line ~629) says "P1: 01 - 04, 05", yet Table 2 names four groups
// at P1 01-04 plus EXTENSION SETTING at P1=06 (manual extract line ~904)
// and none at P1=05. This inventory follows Table 2: it holds members at P1
// in {1,2,3,4,6} and none at 5. A real radio then rejected both probed
// P1=05 addresses (EX050101, EX050505) with "?;" — consistent with Table 2
// being right and the grammar note's "05" being a typo, on two samples
// rather than a survey of the P1=05 space (docs/hardware-notes.md). It is
// deliberately NOT in table2-corrections.csv: that artefact records
// corrections the manual needs, and two samples do not establish one. The
// transcription in table2.csv still records the manual as found, as its own
// provenance requires, and membership behaviour is unchanged by the
// finding — the evidence is consistent with the reading this inventory
// already had rather than prompting a change to it.
//
// Migration scaffold: delegates to FT710; removed in Task 55.
func KnownEXAddress(a EXAddress) bool {
	return FT710.KnownEXAddress(a)
}

// NewEXAddress returns the member EXAddress for the decimal triple
// (p1,p2,p3), or a *ParseError if that triple is not in the inventory.
// Validation is membership-only: there is no numeric range check on the
// components, so any non-member triple — including negative or oversized
// values — is rejected by the same lookup miss.
func NewEXAddress(p1, p2, p3 int) (EXAddress, error) {
	if a, ok := exByTriple[[3]int{p1, p2, p3}]; ok {
		return a, nil
	}
	return EXAddress{}, newParseError([]byte(fmt.Sprintf("%d,%d,%d", p1, p2, p3)), "EX address is not a known Table 2 member")
}

// ParseEXAddress parses a six-ASCII-digit wire field ("010203") into a member
// EXAddress. It performs only a shape check (exactly six digits) followed by
// a membership lookup; it applies NO numeric range logic to the parsed
// components. A malformed shape or a non-member address yields a *ParseError.
func ParseEXAddress(wire string) (EXAddress, error) {
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
	return NewEXAddress(p1, p2, p3)
}

// EXItems returns a fresh copy of the full inventory, sorted by (P1,P2,P3),
// with exactly 296 items. Callers may freely mutate the returned slice; it
// never aliases the package's own data.
//
// Migration scaffold: delegates to FT710; removed in Task 55.
func EXItems() []EXItem {
	return FT710.EXItems()
}

// EXAddresses returns a fresh copy of every inventory address, sorted by
// (P1,P2,P3). Callers may freely mutate the returned slice.
//
// Migration scaffold: delegates to FT710; removed in Task 55.
func EXAddresses() []EXAddress {
	return FT710.EXAddresses()
}
