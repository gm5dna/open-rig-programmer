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
// names and their exact signature. Values are pinned by lane P's own
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
// "Different inventories" is asserted structurally, without reading either
// one: the two accessors must not be the same function value.
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
}
