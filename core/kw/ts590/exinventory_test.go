// SPDX-License-Identifier: GPL-3.0-or-later

package ts590

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/kw"
)

// THIS FILE PINS SIGNATURES, NOT VALUES, AND THE THINNESS IS THE CONTRACT.
//
// exinventory590s_gen.go and exinventory590sg_gen.go are BOOTSTRAP
// placeholders at this task: they hold empty declarations so that this
// package compiles between task 5 and lane P's transcription, and they are
// lane P's files from the commit that created them. Nothing on lane K may
// read their contents, and a test here that asserted a row count, a name or
// a width would be asserting something about a file this lane does not own
// and cannot regenerate — and would then have to be edited by lane P, which
// the file split forbids.
//
// So what is pinned here is the DECLARATION SURFACE: the three accessor
// names, their exact signature, and WHICH VARIABLE EACH ONE READS — that
// last by a difference the test makes and undoes for itself, never by an
// absolute row count. Values are pinned by lane P's own
// staleness tests (each asserting its inventory's length equals that
// profile's ExpectedRows, so an ungenerated bootstrap file fails loudly
// rather than presenting an empty menu table as a valid one) and by T10's
// cross-check, which re-asserts all three.
//
// The copy behaviour these accessors rely on is pinned where it can be
// pinned with real data: kw.CopyEXItems, in core/kw's own tests. A copy
// test written here would run over an empty slice and prove nothing while
// looking as though it did.

// The signatures, asserted by the COMPILER. Lane P must not touch these
// names and T8's layout values consume them as written.
var (
	_ func() []kw.EXItem = EXItemsS
	_ func() []kw.EXItem = EXItemsSG
)

// TestAccessors_AreCallableAndDistinct is the runtime half: both accessors
// exist, neither panics, and they read DIFFERENT inventories — the S and
// the SG do not share a menu table (590:564 and 590:744 print two lists,
// over colliding addresses with different meanings), so two accessors over
// one variable would be the exact cross-model borrowing this package is
// split to prevent.
//
// "DIFFERENT INVENTORIES" IS PROVED BY A DIFFERENCE THIS TEST ITSELF MAKES,
// which is what lets it be proved without reading either inventory. The test
// appends one zero item to a COPY of exItems590S, rebinds the variable to
// it, and requires exactly two things: EXItemsS' length moved by one, so it
// reads exItems590S; and EXItemsSG' length did not move, so it does not.
// Either half of the borrowing is caught — if both accessors read
// exItems590S the second assertion fails, and if both read exItems590SG the
// first does. The original is restored before the test returns.
//
// It asserts no absolute row count, no name and no width: the only numbers
// it uses are the lengths it measured a line earlier and the delta it
// created. So it holds identically over the empty bootstrap declarations and
// over lane P's generated tables, and it needs no edit when generation runs.
//
// The two accessors are NOT compared as function values. Go func values are
// comparable only to nil, and the reflect.Pointer route proves nothing about
// the subject anyway: two distinct functions reading ONE variable — the bug
// — have two distinct code pointers.
func TestAccessors_AreCallableAndDistinct(t *testing.T) {
	gotS := EXItemsS()
	gotSG := EXItemsSG()
	if gotS == nil {
		t.Error("EXItemsS() returned nil — an accessor over a copy never returns nil, even for an empty inventory")
	}
	if gotSG == nil {
		t.Error("EXItemsSG() returned nil")
	}
	if len(gotS) != len(EXItemsS()) {
		t.Error("EXItemsS() is not stable across calls")
	}

	origS := exItems590S
	t.Cleanup(func() { exItems590S = origS })
	wasS, wasSG := len(EXItemsS()), len(EXItemsSG())

	exItems590S = append(append([]kw.EXItem(nil), origS...), kw.EXItem{})
	if got := len(EXItemsS()); got != wasS+1 {
		t.Errorf("one item was added to exItems590S and EXItemsS() went from %d to %d, want %d — EXItemsS does not read exItems590S", wasS, got, wasS+1)
	}
	if got := len(EXItemsSG()); got != wasSG {
		t.Errorf("only exItems590S was changed and EXItemsSG() went from %d to %d — the two accessors read ONE inventory, which is the S/SG table sharing this package is split to prevent", wasSG, got)
	}
}
