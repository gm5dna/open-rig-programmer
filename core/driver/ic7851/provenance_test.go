// SPDX-License-Identifier: GPL-3.0-or-later

package ic7851

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
)

// TestProvenanceCitesOnlyTheIC7851Authority is the POSITIVE-LIST provenance
// pin the tier review asked for. core/driver/ic7760/provenance_test.go holds
// the other half for its model: a blacklist of literals a rejected branch
// actually copied from the IC-7610, which is cheap and documents a real
// contamination, but which a paraphrase walks straight past.
//
// This test cannot be paraphrased past, because it does not look for anything
// in particular. It extracts EVERY document citation core/driver/ic7851 and
// core/civ/ic7851 make — PDF pages, printed folios, matrix sections and errata,
// and assumption-register ids — and requires each to be one testdata/citations.txt
// allows. That list was populated from what the packages cite today, one token
// at a time, against the IC-7851 authority; where that authority is present the
// test re-checks the list against it, and where it is gitignored away (a fresh
// clone, and CI) the checked-in list is still enforced.
func TestProvenanceCitesOnlyTheIC7851Authority(t *testing.T) {
	drivertest.IcomCitationPin("ic7851").Assert(t)
}
