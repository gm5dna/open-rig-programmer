// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"os"
	"sort"
	"testing"

	"github.com/gm5dna/open-rig-programmer/internal/extable"
)

// exDenylistGolden is the exact 58 denied (P1,P2,P3) triples, address by
// address, from the milestone spec's §1 scope table
// (.superpowers/sdd/2026-09-19-v1110-settings-write/reviews/spec.md) — a
// golden list, not derived from table2.csv, so the test below fails loudly
// if a future table2.csv edit moves an address in or out of a denied class.
// Comment on each is the address's Name for a human diffing a failure; the
// test does not read the comments.
var exDenylistGolden = [][3]int{
	{1, 1, 14}, // MOD SOURCE
	{1, 1, 17}, // RPTT SELECT
	{1, 2, 14}, // MOD SOURCE
	{1, 2, 17}, // RPTT SELECT
	{1, 3, 13}, // MOD SOURCE
	{1, 3, 16}, // RPTT SELECT
	{1, 4, 14}, // MOD SOURCE
	{1, 4, 17}, // RPTT SELECT
	{1, 5, 13}, // RPTT SELECT
	{2, 1, 13}, // RPTT SELECT
	{2, 1, 15}, // PC KEYING
	{3, 1, 3},  // TUN/LIN PORT SELECT
	{3, 1, 4},  // TUNER TYPE SELECT
	{3, 1, 5},  // CAT-1 RATE
	{3, 1, 6},  // CAT-1 TIME OUT TIMER
	{3, 1, 7},  // CAT-1 CAT-3 STOP BIT
	{3, 1, 8},  // CAT-2 RATE
	{3, 1, 9},  // CAT-2 TIME OUT TIMER
	{3, 1, 10}, // CAT-3 RATE
	{3, 1, 11}, // CAT-3 TIME OUT TIMER
	{3, 1, 15}, // TX TIME OUT TIMER
	{3, 4, 1},  // HF MAX POWER
	{3, 4, 2},  // 50M MAX POWER
	{3, 4, 3},  // 70M MAX POWER
	{3, 4, 4},  // AM MAX POWER
	{3, 4, 6},  // EMERGENCY FREQ TX
	{3, 4, 7},  // TX INHIBIT
	{4, 1, 1},  // MY CALL
	{6, 1, 1},  // PRESET NAME
	{6, 1, 2},  // CAT-1 RATE
	{6, 1, 3},  // CAT-1 TIME OUT TIMER
	{6, 1, 4},  // CAT-1 CAT-3 STOP BIT
	{6, 1, 15}, // MOD SOURCE
	{6, 1, 18}, // RPTT SELECT
	{6, 2, 1},  // PRESET NAME
	{6, 2, 2},  // CAT-1 RATE
	{6, 2, 3},  // CAT-1 TIME OUT TIMER
	{6, 2, 4},  // CAT-1 CAT-3 STOP BIT
	{6, 2, 15}, // MOD SOURCE
	{6, 2, 18}, // RPTT SELECT
	{6, 3, 1},  // PRESET NAME
	{6, 3, 2},  // CAT-1 RATE
	{6, 3, 3},  // CAT-1 TIME OUT TIMER
	{6, 3, 4},  // CAT-1 CAT-3 STOP BIT
	{6, 3, 15}, // MOD SOURCE
	{6, 3, 18}, // RPTT SELECT
	{6, 4, 1},  // PRESET NAME
	{6, 4, 2},  // CAT-1 RATE
	{6, 4, 3},  // CAT-1 TIME OUT TIMER
	{6, 4, 4},  // CAT-1 CAT-3 STOP BIT
	{6, 4, 15}, // MOD SOURCE
	{6, 4, 18}, // RPTT SELECT
	{6, 5, 1},  // PRESET NAME
	{6, 5, 2},  // CAT-1 RATE
	{6, 5, 3},  // CAT-1 TIME OUT TIMER
	{6, 5, 4},  // CAT-1 CAT-3 STOP BIT
	{6, 5, 15}, // MOD SOURCE
	{6, 5, 18}, // RPTT SELECT
}

// TestEXDenylist_MatchesTable2ByAddress walks table2.csv via
// internal/extable.ParseCSV — the same parser exinventory_gen.go is
// generated from — and classifies every row through this file's six
// denylist predicates plus exHeld. It pins spec §1's exact counts (58
// denied, 4 held, 234 admitted, across 21 (P1,P2) groups) and the denied set
// address by address against exDenylistGolden, so a future table2.csv edit
// that silently moves an address in or out of a class fails the build
// rather than shipping quietly.
//
// package cat, not cat_test: this is a build-time-only dependency on
// internal/extable, the same shape exdigits_ceiling_test.go already has.
// core/cat's own production code never imports internal/extable (that
// package's own doc comment: "build-time tooling ONLY"), and
// `go list -test -deps ./core/cat` shows no import cycle either direction —
// checked before writing this test rather than assumed.
func TestEXDenylist_MatchesTable2ByAddress(t *testing.T) {
	data, err := os.ReadFile("table2.csv")
	if err != nil {
		t.Fatalf("reading table2.csv: %v", err)
	}
	rows, err := extable.ParseCSV(extable.FT710Profile(), data)
	if err != nil {
		t.Fatalf("ParseCSV(table2.csv): %v", err)
	}

	var denied, held, admitted [][3]int
	groups := map[[2]int]bool{}
	for _, r := range rows {
		groups[[2]int{r.P1, r.P2}] = true
		addr := [3]int{r.P1, r.P2, r.P3}

		isDenied := exCATLinkDenied(r.Name) ||
			exKeyingDenied(r.P4) ||
			exTunerRoutingDenied(r.Name) ||
			exTXSafetyDenied(r.P1, r.P2, r.P3, r.Name) ||
			exUnintendedTXSourceDenied(r.Name) ||
			exTextDenied(r.Text)
		isHeld := exHeld(r.P1, r.P2, r.P3)

		switch {
		case isDenied && isHeld:
			t.Fatalf("address %v (%s): denied AND held — the two sets must be disjoint", addr, r.Name)
		case isDenied:
			denied = append(denied, addr)
		case isHeld:
			held = append(held, addr)
		default:
			admitted = append(admitted, addr)
		}
	}

	if len(rows) != 296 {
		t.Fatalf("table2.csv row count = %d, want 296", len(rows))
	}
	if got, want := len(denied), 58; got != want {
		t.Errorf("denied count = %d, want %d", got, want)
	}
	if got, want := len(held), 4; got != want {
		t.Errorf("held count = %d, want %d", got, want)
	}
	if got, want := len(admitted), 234; got != want {
		t.Errorf("admitted count = %d, want %d", got, want)
	}
	if got, want := len(groups), 21; got != want {
		t.Errorf("(P1,P2) group count = %d, want %d", got, want)
	}

	sort.Slice(denied, func(i, j int) bool {
		if denied[i][0] != denied[j][0] {
			return denied[i][0] < denied[j][0]
		}
		if denied[i][1] != denied[j][1] {
			return denied[i][1] < denied[j][1]
		}
		return denied[i][2] < denied[j][2]
	})
	if len(denied) == len(exDenylistGolden) {
		for i, addr := range denied {
			if addr != exDenylistGolden[i] {
				t.Errorf("denied[%d] = %v, want %v (exDenylistGolden)", i, addr, exDenylistGolden[i])
			}
		}
	} else {
		t.Errorf("denied set has %d addresses, exDenylistGolden has %d — set:\n%v", len(denied), len(exDenylistGolden), denied)
	}

	wantHeld := map[[3]int]bool{
		{1, 5, 16}: true,
		{3, 1, 12}: true,
		{3, 1, 13}: true,
		{3, 1, 14}: true,
	}
	for _, addr := range held {
		if !wantHeld[addr] {
			t.Errorf("held set contains unexpected address %v", addr)
		}
		delete(wantHeld, addr)
	}
	for addr := range wantHeld {
		t.Errorf("held set is missing expected address %v", addr)
	}
}
