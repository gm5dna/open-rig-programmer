// SPDX-License-Identifier: GPL-3.0-or-later

package ft710

import (
	"fmt"
	"testing"
)

// writeBoundaryDeniedGolden mirrors
// core/cat/exdenylist_test.go's own golden lists (58 denied) by
// (P1,P2,P3) triple — a second, independent pin against the exact same
// milestone spec table, from this driver's own write-boundary dump, so a
// future change that broke EITHER side's filter fails at least one of the
// two tests. Comments name the item for a human diffing a failure only;
// the test does not read them.
//
// Moved here from cmd/rigprog/writeboundary_test.go (task h2, guard fix):
// the row derivation it pins moved from cmd/rigprog to this package —
// cmd/rigprog may not import core/cat, and building a row needs
// catDialect.EXItems()/EXWriteDescriptor, both core/cat-typed
// (internal/guards' composition-root discipline).
var writeBoundaryDeniedGolden = [][3]int{
	{1, 1, 14}, {1, 1, 17}, {1, 2, 14}, {1, 2, 17}, {1, 3, 13}, {1, 3, 16},
	{1, 4, 14}, {1, 4, 17}, {1, 5, 13}, {2, 1, 13}, {2, 1, 15},
	{3, 1, 3}, {3, 1, 4}, {3, 1, 5}, {3, 1, 6}, {3, 1, 7}, {3, 1, 8},
	{3, 1, 9}, {3, 1, 10}, {3, 1, 11}, {3, 1, 15},
	{3, 4, 1}, {3, 4, 2}, {3, 4, 3}, {3, 4, 4}, {3, 4, 6}, {3, 4, 7},
	{4, 1, 1},
	{6, 1, 1}, {6, 1, 2}, {6, 1, 3}, {6, 1, 4}, {6, 1, 15}, {6, 1, 18},
	{6, 2, 1}, {6, 2, 2}, {6, 2, 3}, {6, 2, 4}, {6, 2, 15}, {6, 2, 18},
	{6, 3, 1}, {6, 3, 2}, {6, 3, 3}, {6, 3, 4}, {6, 3, 15}, {6, 3, 18},
	{6, 4, 1}, {6, 4, 2}, {6, 4, 3}, {6, 4, 4}, {6, 4, 15}, {6, 4, 18},
	{6, 5, 1}, {6, 5, 2}, {6, 5, 3}, {6, 5, 4}, {6, 5, 15}, {6, 5, 18},
}

func tripleID(t [3]int) string {
	return fmt.Sprintf("%02d%02d%02d", t[0], t[1], t[2])
}

// TestWriteBoundaryRows_238AdmittedAndDeniedAbsent pins task-h2's own
// safety-critical counts against the shipped FT-710 dialect: exactly 238
// rows, none of the 58 denied addresses present among them, and every
// denied address individually confirmed absent by id (not just by count).
func TestWriteBoundaryRows_238AdmittedAndDeniedAbsent(t *testing.T) {
	rows := WriteBoundaryRows()
	if len(rows) != 238 {
		t.Fatalf("WriteBoundaryRows() returned %d rows, want 238", len(rows))
	}

	present := make(map[string]bool, len(rows))
	for _, r := range rows {
		present[r.ID] = true
	}

	for _, triple := range writeBoundaryDeniedGolden {
		if present[tripleID(triple)] {
			t.Errorf("denied address %s present in write-boundary dump", tripleID(triple))
		}
	}
}

// TestWriteBoundaryRows_WidthIsObserved pins that Session W (18/10/2026)
// filled table2-write-observed.csv: every row carries a real, nonzero Set
// width and a real, nonzero ReadWidth.
func TestWriteBoundaryRows_WidthIsObserved(t *testing.T) {
	rows := WriteBoundaryRows()
	for _, r := range rows {
		if r.Width == 0 {
			t.Errorf("row %s width = 0, want the Session W observed Set width", r.ID)
		}
		if r.ReadWidth == 0 {
			t.Errorf("row %s readWidth = 0, want the manual's real Digits value", r.ID)
		}
	}
}

// TestWriteBoundaryRows_ToneFreqSizedFromObservedRead pins Session W
// (18/10/2026): 010321 answers 3 bytes (manual Digits 2) and a 2-digit Set
// was refused, so its row sizes Sets at 3; its domain stays 0-49.
func TestWriteBoundaryRows_ToneFreqSizedFromObservedRead(t *testing.T) {
	for _, r := range WriteBoundaryRows() {
		if r.ID != "010321" {
			continue
		}
		if r.ReadWidth != 3 || r.Width != 3 || r.Lo != 0 || r.Hi != 49 {
			t.Errorf("010321 = readWidth %d, %d-%d; want 3, 0-49", r.ReadWidth, r.Lo, r.Hi)
		}
		return
	}
	t.Fatal("010321 missing from write-boundary rows")
}
