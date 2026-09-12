// SPDX-License-Identifier: GPL-3.0-or-later

package ic7200

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
)

// TestProvenanceCitesOnlyTheIC7200Authority is the positive-list
// provenance pin the tier review asks every Icom package for (see
// core/driver/ic7851/provenance_test.go for the full rationale). It
// extracts every document citation this package and core/civ/ic7200
// make and requires each to be one testdata/citations.txt allows; where
// the matrix authority is present (a working checkout) it further
// requires the authority to actually supply each listed token, and
// where it is absent (docs/superpowers is gitignored — a fresh clone,
// CI, or this worktree) the checked-in list is still enforced.
func TestProvenanceCitesOnlyTheIC7200Authority(t *testing.T) {
	drivertest.IcomCitationPin("ic7200").Assert(t)
}
