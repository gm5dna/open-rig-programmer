// SPDX-License-Identifier: GPL-3.0-or-later

package ic7410

import (
	"path/filepath"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
)

// TestProvenanceCitesOnlyTheIC7410Authority is the positive-list provenance
// pin: every document citation core/driver/ic7410 and core/civ/ic7410 make
// (PDF pages, matrix sections, register ids) must be one testdata/citations.txt
// allows, and — where docs/superpowers is present in this checkout — one the
// IC-7410 capability matrix itself can supply.
//
// freeze_test.go and golden_test.go are excluded in ADDITION to
// drivertest.IcomCitationPin's standard provenance_test.go exclusion: their
// only "citation"-shaped token is the self-referential filename of THIS
// package's own golden-vectors testdata
// (core/civ/ic7410/testdata/ic7410-golden-vectors.md), a file this package
// authored itself rather than a document citation the matrix or an
// implementation plan could ever be expected to supply.
func TestProvenanceCitesOnlyTheIC7410Authority(t *testing.T) {
	p := drivertest.IcomCitationPin("ic7410")
	p.Exclude = append(p.Exclude, "freeze_test.go", "golden_test.go")
	p.ListPath = filepath.Join("testdata", "citations.txt")
	p.Assert(t)
}
