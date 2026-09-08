// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/internal/extable"
)

// kwImportPath is the ImportPath a profile carries when it renders into
// this package. It is how the ceiling test below selects the Kenwood
// profiles without naming them: five have landed and the test must bite on
// all five, including a sixth nobody has planned.
const kwImportPath = "github.com/gm5dna/open-rig-programmer/core/kw"

// TestMaxEXDigits_IsDerivedFromThisPackagesOwnFrameBound pins the
// arithmetic rather than the number, so a change to DefaultMaxFrame moves
// the ceiling with it instead of leaving a stale literal behind.
func TestMaxEXDigits_IsDerivedFromThisPackagesOwnFrameBound(t *testing.T) {
	// "EX"(2) + P1(3) + P2(2) + P3(1) + P4(1) + ";"(1) = 10 bytes of an EX
	// ANSWER that are not P5 (590:552-556, 480:409-411).
	const fixed = 10
	if want := DefaultMaxFrame - fixed; MaxEXDigits != want {
		t.Errorf("MaxEXDigits = %d, want DefaultMaxFrame - %d = %d", MaxEXDigits, fixed, want)
	}
	// A P5 exactly MaxEXDigits wide must fit inside a frame this package's
	// own accumulator will reassemble; one byte wider must not.
	if MaxEXDigits+fixed != DefaultMaxFrame {
		t.Errorf("an EX answer carrying MaxEXDigits (%d) is %d bytes, want exactly DefaultMaxFrame (%d)", MaxEXDigits, MaxEXDigits+fixed, DefaultMaxFrame)
	}
}

// TestMaxEXDigits_IsNotCoreCatsCeiling is the copy-paste trap named, and it
// is the whole reason Stage 0 gave Profile its own DigitsCeiling field.
//
// internal/extable's MaxDigitsCeiling is CORE/CAT's number: 247, mirroring
// that package's maxEXDigits, derived from ITS nine-byte EX answer overhead
// and pinned by core/cat/exdigits_ceiling_test.go. A Kenwood profile that
// carried it would be a bound consulted from one place with its datum taken
// from another — the defect shape Profile's own doc comment says the type
// exists to prevent. The two numbers differ by exactly one byte because a
// Kenwood EX answer has one more fixed byte than a Yaesu one, and the
// closeness is precisely what would make the mistake invisible.
func TestMaxEXDigits_IsNotCoreCatsCeiling(t *testing.T) {
	if MaxEXDigits == extable.MaxDigitsCeiling {
		t.Errorf("MaxEXDigits (%d) equals extable.MaxDigitsCeiling (%d) — core/cat's ceiling is not this family's, and a stanza that copied it would pass every test in internal/extable", MaxEXDigits, extable.MaxDigitsCeiling)
	}
}

// TestExtableCeilingMatchesKenwoodBound is the core/kw twin of
// core/cat/exdigits_ceiling_test.go: every registered profile that renders
// into THIS package must carry MaxEXDigits as its DigitsCeiling.
//
// The two are declared separately — a build-time tool must not import the
// runtime package it generates into, so internal/extable cannot read
// MaxEXDigits and each Kenwood stanza transcribes the value — and without
// this pin they could drift silently.
//
// FIVE STANZAS HAVE LANDED — ts590s, ts590sg and ts480 from pair 1, and
// ts890s and ts990s from pair 2 — so this loop runs over five profiles and
// the count check below requires exactly that. ZERO IS NOT A LEGITIMATE
// OUTCOME: it would mean every Kenwood registration had been dropped, and a
// pin that reported that as success would be the decay it exists to catch.
//
// THE SELECTOR DID NOT MOVE, AND THAT IS THE POINT OF THE ONE-LINE EDIT. It
// is the exact ImportPath at :15, not a name list, so the two new profiles
// were inside this population from the moment they registered: pair 2's
// arrival changed the COUNT and nothing else, and a sixth Kenwood profile
// nobody planned would be caught the same way.
func TestExtableCeilingMatchesKenwoodBound(t *testing.T) {
	seen := 0
	for _, np := range extable.RegisteredProfiles() {
		if np.Profile.ImportPath != kwImportPath {
			continue
		}
		seen++
		if np.Profile.DigitsCeiling != MaxEXDigits {
			t.Errorf("profile %q renders into core/kw but declares DigitsCeiling = %d, want MaxEXDigits (%d)", np.Name, np.Profile.DigitsCeiling, MaxEXDigits)
		}
	}
	// The expected population, stated so this test says out loud what it
	// covers rather than passing quietly over a set that has emptied.
	if seen != 5 {
		t.Errorf("%d profiles render into core/kw, want 5 (ts590s, ts590sg, ts480, ts890s, ts990s) — a stanza was dropped, or one landed that nobody accounted for", seen)
	}
}
