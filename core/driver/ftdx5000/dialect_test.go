// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx5000

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/cat/dialecttest"
)

// TestConformance runs core/cat's whole exported-API conformance suite
// over this package's inline dialect (doc.go explains why it is inline
// rather than a core/cat/ftdx5000 subpackage) — every builder's frames
// are well-formed and admitted by this dialect's own gate, both wrong-form
// APIs refuse and are seen to refuse, the clarifier's endpoints build and
// one step past is refused, and the EX-empty/MT-dead configuration (V9/
// V12's structural requirements with no real protocol behind them) is
// handled gracefully rather than exercised for real.
func TestConformance(t *testing.T) {
	dialecttest.Run(t, dialect)
}

// TestCATID_ComesFromTheDialect pins the linkage: the driver's identity
// value is derived from the dialect rather than restated alongside it.
func TestCATID_ComesFromTheDialect(t *testing.T) {
	if catID != dialect.CATID() {
		t.Errorf("catID = %q, want the dialect's %q", catID, dialect.CATID())
	}
	if catID != "0362" {
		t.Errorf("catID = %q, want the documented \"0362\" (matrix §4: ID's own P1 legend)", catID)
	}
	if got := CapabilitiesUnverified().CATID; got != catID {
		t.Errorf("Capabilities().CATID = %q, want the same %q the probe compares against", got, catID)
	}
}

// TestModeNames_TwelveOfFifteen pins the mode table's own count and the
// three absent values (matrix §1.3): D (AM-N), E (PSK), F (DATA-FM-N).
func TestModeNames_TwelveOfFifteen(t *testing.T) {
	got := modeDisplayNames()
	if len(got) != 12 {
		t.Errorf("modeDisplayNames() has %d entries, want 12 (matrix §1.3: twelve of core/cat's fifteen named modes)", len(got))
	}
	for _, absent := range []byte{'D', 'E', 'F'} {
		if dialect.ValidMode(cat.Mode(absent)) {
			t.Errorf("mode %q is valid on this dialect, want absent (matrix §1.3)", absent)
		}
	}
}
