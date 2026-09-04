// SPDX-License-Identifier: GPL-3.0-or-later

package ft891

import (
	"testing"

	catft891 "github.com/gm5dna/open-rig-programmer/core/cat/ft891"
)

// TestCATID_ComesFromTheDialect pins the linkage (matrix §1.2): the
// driver's identity value is DERIVED from the dialect rather than restated
// alongside it, so the value the ID probe compares against and the value
// the capability data advertises cannot drift. The second assertion keeps
// the documented literal in the test as well as in the code — a dialect
// edit that silently changed the FT-891's CAT ID would fail here rather
// than quietly redefining what "an FT-891" is.
func TestCATID_ComesFromTheDialect(t *testing.T) {
	if catID != catft891.Dialect().CATID() {
		t.Errorf("catID = %q, want the dialect's %q", catID, catft891.Dialect().CATID())
	}
	if catID != "0650" {
		t.Errorf("catID = %q, want the documented %q (matrix §1.2: ID's P1 legend, \"0650: FT-891\", layout 763)", catID, "0650")
	}
	if got := CapabilitiesUnverified().CATID; got != catID {
		t.Errorf("Capabilities().CATID = %q, want the same %q the probe compares against", got, catID)
	}
}
