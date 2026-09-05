// SPDX-License-Identifier: GPL-3.0-or-later

package ts480_test

import (
	"os"
	"strings"
	"testing"
)

// THE MENU 034 ERRATUM IS PROSE, AND PROSE IS THE ONLY RECORD IT HAS.
//
// crosscheck_test.go pins the two SETS — the two-digit rows the chart prints
// and the ones the EX block's own sentence lists — and it says in as many
// words that the schedule entry belongs in doc.go rather than in itself. But
// a set comparison cannot say WHY the sets differ, and a reader who met
// TestCrossCheck_TheMenu034Erratum failing with no entry to read would have
// to re-derive the whole finding from the PDF.
//
// So this file holds the one link that would otherwise decay silently: the
// erratum row is in this package's doc comment and it names the test. The
// authoritative schedule is core/kw/doc.go's, where E22's population and
// category are pinned by core/kw/register_test.go; this pin is the local
// half, and neither is a substitute for the other.
func TestDoc_CarriesTheMenu034Erratum(t *testing.T) {
	b, err := os.ReadFile("doc.go")
	if err != nil {
		t.Fatalf("reading doc.go: %v", err)
	}
	src := string(b)

	for _, want := range []string{
		// The row itself, in the errata share.
		"E22",
		// The two readings it records, so a reader can see the
		// disagreement without opening the manual.
		"Menu No.\n//	     32, 35 and 48 ~ 52 use 2-digit parameters",
		"menu 034",
		// The citations: the block's sentence and the chart's own row.
		"480:411",
		"480:483-486",
		// And the cross-reference, which is what this test exists for.
		"TestCrossCheck_TheMenu034Erratum",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("doc.go no longer carries %q.\nThe menu-034 erratum is a defect PRINTED in the manual: all three transcription legs read it faithfully and agree, so no comparison in crosscheck_test.go can find it and this entry is the only place it is recorded.", want)
		}
	}
}
