// SPDX-License-Identifier: GPL-3.0-or-later

package ic7760

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sourceText(t *testing.T, paths ...string) string {
	t.Helper()
	var out strings.Builder
	for _, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		out.Write(b)
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
