// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import "strings"

// This file is HAND-MAINTAINED, not generated: unlike exinventory_gen.go
// (internal/extable/gen, driven by table2.csv), nothing here is derived
// mechanically from the CSV. Each predicate below encodes one of the six
// permanently-denied classes from docs/menu-write-decision.md ("What this
// decision is not") and the milestone spec's §1 scope table
// (.superpowers/sdd/2026-09-19-v1110-settings-write/reviews/spec.md), plus
// the four addresses HELD read-only pending further evidence (same §1).
// TestEXDenylist_MatchesTable2ByAddress (exdenylist_test.go) is the guard: it
// walks table2.csv through internal/extable.ParseCSV and pins the exact
// denied/held/admitted counts and the denied set address by address, so a
// future edit to table2.csv that silently moves an address in or out of a
// class fails the build rather than shipping quietly.
//
// NOT WIRED INTO THE GATE YET. Dialect.AllowedCommand's validEXRead still
// refuses every EX Set/Answer-shaped frame outright (allowlist.go) — these
// predicates classify addresses for the golden test only. Consulting them
// from the gate, so an admitted-but-uncharacterised address is refused and a
// denied one can never reach a Set builder regardless of what the write
// descriptor table says, is a later task on this milestone.
//
// A predicate here matches on BEHAVIOUR — the legend text (what the address
// actually does) or, where the legend alone does not say enough, the (P1,P2)
// group and P3 item number — never on the item's Name alone where behaviour
// and label can diverge. exKeyingDenied is the case that forced this rule:
// PC KEYING (02-01-15) carries the same RTS/DTR/DAKY legend as every RPTT
// SELECT item, but a rule matching the name "RPTT SELECT" alone missed it
// (Stuart's ruling, spec-adjudication.md; both reviewers flagged the gap).

// exCATLinkDenied reports whether name is one of the CAT-1/CAT-2/CAT-3 link
// settings (baud rate, timeout, stop bit): changing the CAT link's own
// parameters over that same CAT link can sever the link being used to change
// them. 22 addresses: the three OPERATION/GENERAL rows plus one triple per
// EXTENSION SETTING PRESET (5 presets x 3 rows) (table2.csv:172-178,
// :250-252,268-270,286-288,304-306,322-324).
func exCATLinkDenied(name string) bool {
	switch name {
	case "CAT-1 RATE", "CAT-1 TIME OUT TIMER", "CAT-1 CAT-3 STOP BIT",
		"CAT-2 RATE", "CAT-2 TIME OUT TIMER",
		"CAT-3 RATE", "CAT-3 TIME OUT TIMER":
		return true
	default:
		return false
	}
}

// exKeyingDenied reports whether p4 (the manual's parameter-description
// legend for the address, table2.csv's p4 column) is the RTS/DTR/DAKY
// keying-port legend: PTT and CW keying routed over the CAT-1 serial lines
// rather than a dedicated keying line. Matched by LEGEND, not name, because
// the legend is what every RPTT SELECT item shares with PC KEYING (its name
// alone does not say "keying port"). 12 addresses: eleven RPTT SELECT items
// plus PC KEYING (table2.csv:59,78,94,116,132,149,151,266,284,302,320,338).
func exKeyingDenied(p4 string) bool {
	return containsAll(p4, "RTS", "DTR", "DAKY")
}

// exTunerRoutingDenied reports whether name is one of the two tuner/antenna
// routing items: which physical port (internal tuner, external tuner, linear
// amp, CAT-3-controlled accessory) TX RF is routed to. 2 addresses
// (table2.csv:170-171).
func exTunerRoutingDenied(name string) bool {
	switch name {
	case "TUN/LIN PORT SELECT", "TUNER TYPE SELECT":
		return true
	default:
		return false
	}
}

// exTXSafetyDenied reports whether (p1,p2,p3,name) is one of the TX
// power/TX safety items: the 03-04 GENERAL TX items 01-04 and 06-07 (max
// power per band, EMERGENCY FREQ TX, TX INHIBIT — item 05 VOX SELECT and
// item 08 METER DETECTOR are admitted, not denied, so they are deliberately
// excluded here) plus TX TIME OUT TIMER, which lives under 03-01 GENERAL.
// 7 addresses (table2.csv:219-222,224-225,182).
func exTXSafetyDenied(p1, p2, p3 int, name string) bool {
	if p1 == 3 && p2 == 4 {
		switch p3 {
		case 1, 2, 3, 4, 6, 7:
			return true
		}
	}
	return name == "TX TIME OUT TIMER"
}

// exUnintendedTXSourceDenied reports whether name is MOD SOURCE: it selects
// the audio source (MIC/USB/REAR/AUTO) a mode's TX audio is drawn from, and
// paired with an admitted VOX SELECT it can route an unexpected source into
// a VOX-armed transmit path (Stuart, 19/09/2026). 9 addresses
// (table2.csv:56,75,91,113,263,281,299,317,335).
func exUnintendedTXSourceDenied(name string) bool {
	return name == "MOD SOURCE"
}

// exTextDenied reports whether the address is one of the six free-text
// items (MY CALL + 5x PRESET NAME): text has no bounded value domain to
// characterise against, so it is out of scope regardless of what it
// controls. 6 addresses (table2.csv:233,249,267,285,303,321).
func exTextDenied(text bool) bool {
	return text
}

// containsAll reports whether s contains every one of subs, in any order.
func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

// exHeldTriples lists the four addresses held read-only for v1.11.0 —
// excluded from the admitted set but NOT permanently denied, revisited
// without silence once the evidence below exists (spec §1):
//
//   - (01,05,16) SHIFT FREQUENCY: the manual prints the code "1:" twice and
//     offers no "0"; M8c's hardware answer of 0 proves a 0-code exists but
//     not which label it means. Held until Session R's front-panel trial
//     settles the mapping.
//   - (03,01,12) QMB CH, (03,01,13) BAND STACK, (03,01,14) MEM GROUP: MEM
//     GROUP may change how the radio organises memory channels, which may
//     change the slot addressing core/clone's channel write path (MR/MW/MT)
//     assumes. Held pending Session R's investigative step.
var exHeldTriples = [4][3]int{
	{1, 5, 16},
	{3, 1, 12},
	{3, 1, 13},
	{3, 1, 14},
}

// exHeld reports whether (p1,p2,p3) is one of exHeldTriples.
func exHeld(p1, p2, p3 int) bool {
	for _, t := range exHeldTriples {
		if t[0] == p1 && t[1] == p2 && t[2] == p3 {
			return true
		}
	}
	return false
}
