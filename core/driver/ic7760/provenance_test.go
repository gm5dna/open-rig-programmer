// SPDX-License-Identifier: GPL-3.0-or-later

package ic7760

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
)

// TestProvenanceCitesOnlyTheIC7760Authority is the POSITIVE-LIST half, added
// by the Tier 4b review. TestProvenanceUsesOnlyTheIC7760Authority below is a
// BLACKLIST: it names literals the rejected branch actually copied from the
// IC-7610, which is cheap and documents a real contamination, but which a
// paraphrase walks straight past.
//
// A PARAPHRASE DOES NOT GET PAST THIS TEST BY RE-SPELLING A CITATION, because
// it does not look for anything in particular. It extracts EVERY document
// citation core/driver/ic7760 and core/civ/ic7760 make — PDF pages, printed
// folios, matrix sections and errata, and assumption-register ids — and requires each to be one
// testdata/citations.txt allows. That list was populated from what the
// packages cite today, one token at a time, against the IC-7760 authority;
// where that authority is present the test re-checks the list against it, and
// where it is gitignored away (a fresh clone, and CI) the checked-in list is
// still enforced.
//
// The two tests are kept side by side deliberately. The blacklist remembers
// WHICH lifts leaked and from where; the positive list is the one that would
// have stopped them.
//
// The grammar reads a citation in whatever spelling and casing the prose
// chose: "PDF p.375", "page 375" and "PDF P.375" are one token, and "§3.16.4",
// "matrix section 3.16.4" and a bare "section 3.16.4" are another. TWO SHAPES
// ARE DELIBERATELY OUTSIDE IT, each argued where the grammar is written: a bare
// "Erratum N", which in these packages belongs to the shared tier additions
// spec rather than to any one radio's matrix, and an undotted spelled-out
// "Section 18", which is the manual's own part number and not a matrix section.
// A claim made ONLY in one of those two forms is not covered here.
func TestProvenanceCitesOnlyTheIC7760Authority(t *testing.T) {
	drivertest.IcomCitationPin("ic7760").Assert(t)
}

// sourceText concatenates paths with comment markers stripped and every run
// of whitespace collapsed to one space.
//
// THE NORMALISATION IS LOAD-BEARING, not tidiness: a phrase that wraps
// across two comment lines is "matrix lift\n// R7" on disk and would slip
// past a naive Contains for "matrix lift R7". One IC-7610 lift identifier
// survived this sweep's first pass on exactly that trick.
func sourceText(t *testing.T, paths ...string) string {
	t.Helper()
	var out strings.Builder
	for _, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		out.WriteString(strings.Join(strings.Fields(strings.ReplaceAll(string(b), "//", " ")), " "))
		out.WriteByte('\n')
	}
	return out.String()
}

// packageSources returns every .go file in a package directory except the
// named exclusions, so a provenance sweep covers COMMENTS IN TESTS too.
// Finding F2's wrong-address lifts survived in test-file comments after
// the production sites were corrected; a sweep restricted to doc.go,
// caps.go and framing.go would have missed them again.
func packageSources(t *testing.T, dir string, exclude ...string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	skip := map[string]bool{}
	for _, name := range exclude {
		skip[name] = true
	}
	var paths []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || skip[e.Name()] {
			continue
		}
		paths = append(paths, filepath.Join(dir, e.Name()))
	}
	if len(paths) == 0 {
		t.Fatalf("no .go sources found under %s", dir)
	}
	return sourceText(t, paths...)
}

func TestProvenanceUsesOnlyTheIC7760Authority(t *testing.T) {
	// This file is excluded because it is the list itself: the forbidden
	// literals below necessarily appear in it.
	driverDocs := packageSources(t, ".", "provenance_test.go")
	driverDocs += packageSources(t, filepath.Join("..", "..", "civ", "ic7760"))
	for _, forbidden := range []string{
		"Reference Guide rev 4",
		"17 pages",
		"USB1",
		"1A 05 00 95",
		"1A 05 00 96",
		"1A 05 00 97",
		"matrix erratum 11",
		"matrix erratum 12",
		"FE FE 98 E0",
		// The IC-7760 matrix numbers its 21 register entries; it has no
		// R-numbered lift scheme and no OQ rulings. Every "matrix lift Rn",
		// "adjudication Rn", "RULING OQn" and "R9-SPLIT" token in the
		// rejected branch came from core/driver/ic7610, which is a SHAPE
		// precedent only.
		"matrix lift R",
		"lift R1",
		"lift R2",
		"lift R7",
		"adjudication R",
		"RULING OQ",
		"R9-SPLIT",
		// Register names invented for this package instead of taken from
		// the matrix's register column (finding F3).
		"ic7760-1a00-set-ack",
		"ic7760-full-record-mandatory",
		"ic7760-default-tone-undocumented",
		"ic7760-mode-code-completeness",
		"ic7760-filter-value-set",
		"ic7760-storable-frequency-ceiling",
		// IC-7610 page numbers for the tables this radio prints elsewhere:
		// modes/filters/frequency are PDF p.18 (folio 17), the record bar,
		// the character tables and the ⑪ sub-diagram are PDF p.20 (folio
		// 19), and the tone digit strip is PDF p.24 (folio 23).
		"PDF p.11",
		"PDF p.12",
		"PDF p.14",
		"①Receiving mode",
	} {
		if strings.Contains(driverDocs, forbidden) {
			t.Errorf("IC-7760 provenance contains foreign claim %q", forbidden)
		}
	}
	for _, required := range []string{
		"Revision 2",
		"28 PDF pages",
		"A7788-8EX-2",
		"May 2025",
		"1A 05 01 33",
		"1A 05 01 34",
		"1A 05 01 35",
		"USB (B)",
		"FE FE B2 E0 19 00 FD",
		"transport safety obligation 4",
	} {
		if !strings.Contains(driverDocs, required) {
			t.Errorf("IC-7760 provenance is missing %q", required)
		}
	}
}

func TestAssumptionRegisterUsesEveryExactMatrixEntryOnce(t *testing.T) {
	profileDoc := sourceText(t, filepath.Join("..", "..", "civ", "ic7760", "doc.go"))
	driverDoc := sourceText(t, "doc.go")
	all := profileDoc + driverDoc

	entries := []string{
		"ic7760-serial-framing",
		"ic7760-control-lines",
		"ic7760-default-baud",
		"ic7760-baud-list",
		"ic7760-address-menu",
		"ic7760-transceive-default",
		"ic7760-broadcast-form",
		"ic7760-echo-default",
		"ic7760-read-request-form",
		"ic7760-empty-reply-fa",
		"ic7760-empty-reply-ff",
		"ic7760-name-space-code",
		"ic7760-name-pad-byte",
		"ic7760-write-full-record",
		"ic7760-record-length",
		"ic7760-id-reply",
		"ic7760-clear-scope",
		"ic7760-tone-domain",
		"ic7760-freq-range",
		"ic7760-scan-edge-record-shape",
		"ic7760-usb-b-function",
	}
	for _, entry := range entries {
		marker := "Register entry `" + entry + "` (ASSUMED)."
		if got := strings.Count(all, marker); got != 1 {
			t.Errorf("%s occurs %d times, want exactly one named register entry with one lift", entry, got)
		}
	}
	if strings.Contains(all, "ic7760-inventory") {
		t.Error("inventory was recreated as an assumption register entry; Erratum 5 grades it MANUAL-EVIDENCED")
	}
	if !strings.Contains(all, "inventory is MANUAL-EVIDENCED") {
		t.Error("provenance does not record the Erratum 5 inventory grading")
	}
}
