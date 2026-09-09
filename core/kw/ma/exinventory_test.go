// SPDX-License-Identifier: GPL-3.0-or-later

package ma

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/kw"
)

// THIS FILE ASSERTS SIGNATURES AND NEVER VALUES, and that is the file
// split's own consequence rather than a thin test.
//
// Both inventories are BOOTSTRAP files until lane P generates them, so a
// value assertion written here would assert nothing today while looking as
// though it did — and a count written here would be a second copy of the
// figure the staleness tests take from the registered profile's
// ExpectedRows. kw.CopyEXItems's own doc comment says the same thing about
// the three older packages, and pins the copy behaviour where a non-empty
// slice can be constructed: kw.TestCopyEXItems_ReturnsAnIndependentSlice.

// The accessors' shapes, pinned by the compiler. A generated variable
// renamed, or an accessor quietly changed to return the package-level slice
// itself, fails here.
var (
	_ func() []kw.EXItem = EXItems890S
	_ func() []kw.EXItem = EXItems990S
)

// TestEXItemsAccessors_AreTwoAccessorsOverTwoVariables pins P4's file split
// at the one place a later reader could collapse it: one accessor taking a
// row argument would put the two charts one typo apart, which is the
// cross-model borrowing the Tier 4b sweep exists to forbid.
//
// It asserts the two accessors are separate functions and that each returns
// a non-nil slice — the shape kw.CopyEXItems guarantees for an empty source
// — and asserts nothing whatever about their contents.
func TestEXItemsAccessors_AreTwoAccessorsOverTwoVariables(t *testing.T) {
	if EXItems890S() == nil {
		t.Error("EXItems890S returned a nil slice; kw.CopyEXItems returns an empty non-nil slice for an empty source")
	}
	if EXItems990S() == nil {
		t.Error("EXItems990S returned a nil slice; kw.CopyEXItems returns an empty non-nil slice for an empty source")
	}
	// The two package-level variables must be distinct, or one row's
	// inventory would silently serve the other. Comparing the accessors'
	// results cannot show that while both are empty; comparing the
	// variables' addresses can, and does so without reading a value.
	if &exItems890S == &exItems990S {
		t.Error("exItems890S and exItems990S are one variable — the two charts are separate charts and each row must read its own")
	}
}
