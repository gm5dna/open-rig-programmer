// SPDX-License-Identifier: GPL-3.0-or-later

package icr8600

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
)

// TestProvenanceCitesOnlyTheICR8600Authority is the POSITIVE-LIST provenance
// pin the tier review asked for. core/driver/ic7760/provenance_test.go holds
// the other half for its model: a blacklist of literals a rejected branch
// actually copied from the IC-7610, which is cheap and documents a real
// contamination, but which a paraphrase walks straight past.
//
// A PARAPHRASE DOES NOT GET PAST THIS TEST BY RE-SPELLING A CITATION, because
// it does not look for anything in particular. It extracts EVERY document
// citation core/driver/icr8600 and core/civ/icr8600 make — PDF pages, printed
// folios, matrix sections and errata,
// and assumption-register ids — and requires each to be one testdata/citations.txt
// allows. That list was populated from what the packages cite today, one token
// at a time, against the IC-R8600 authority; where that authority is present the
// test re-checks the list against it, and where it is gitignored away (a fresh
// clone, and CI) the checked-in list is still enforced.
//
// The grammar reads a citation in whatever spelling and casing the prose
// chose: "PDF p.375", "page 375" and "PDF P.375" are one token, and "§3.16.4",
// "matrix section 3.16.4" and a bare "section 3.16.4" are another. TWO SHAPES
// ARE DELIBERATELY OUTSIDE IT, each argued where the grammar is written: a bare
// "Erratum N", which in these packages belongs to the shared tier additions
// spec rather than to any one radio's matrix, and an undotted spelled-out
// "Section 18", which is the manual's own part number and not a matrix section.
// A claim made ONLY in one of those two forms is not covered here.
func TestProvenanceCitesOnlyTheICR8600Authority(t *testing.T) {
	drivertest.IcomCitationPin("icr8600").Assert(t)
}
