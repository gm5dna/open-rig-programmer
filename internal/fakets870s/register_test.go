// SPDX-License-Identifier: GPL-3.0-or-later

package fakets870s

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// registerEntries is the ASSUMED register's roll, by NAME — the names doc.go
// heads its numbered entries with and the names this package's code cites at
// every dependence site. The order matches doc.go's numbering, which exists
// for readability only: the CITE-BY-NAME rule is doc.go's own, inherited from
// internal/fakets480's register (itself inherited from core/kw/doc.go), and
// this test is what makes it hold rather than merely be asked for.
var registerEntries = []string{
	"AN ACCEPTED MW PRODUCES NO REPLY",
	"A WRITE WITH ALL FREQUENCY DIGITS ZERO MARKS THE CHANNEL VACANT",
	"SET-DIRECTION FIELD STRICTNESS ON EVERY OTHER WRITE",
	"THE TONE NUMBER IS STORED, NOT RANGE-CHECKED",
	"AN UNWRITTEN OR VACANT CHANNEL'S EITHER HALF ANSWERS THE ZERO RECORD",
	"STREAM ERRORS ARE SCRIPTABLE",
	"THE FRAME ACCUMULATOR'S CAP AND RESYNC",
	"AI NEVER PUSHES ANYTHING UNSOLICITED",
	"THE DEFAULT IMAGE'S RECORD COMPOSITION",
}

// normalise strips Go comment markers and collapses every run of whitespace to
// one space, so that a register name wrapped across two comment lines is
// found by the same search as one that fits on a line.
func normalise(src string) string {
	replacer := strings.NewReplacer("//", " ", "\t", " ", "\n", " ", "\r", " ")
	return strings.Join(strings.Fields(replacer.Replace(src)), " ")
}

// readPackageSources returns the normalised text of every non-test .go file in
// this directory, keyed by file name.
func readPackageSources(t *testing.T) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	out := map[string]string{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		b, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatalf("ReadFile %s: %v", name, err)
		}
		out[name] = normalise(string(b))
	}
	if len(out) == 0 {
		t.Fatal("read zero non-test .go files — this test would pass vacuously")
	}
	return out
}

// entryHeading matches one numbered register entry's opening line in doc.go.
var entryHeading = regexp.MustCompile(`(?m)^//\s+\d+\.\s+[A-Z"]`)

// TestASSUMEDRegisterIsComplete holds the two halves of doc.go's completeness
// claim together mechanically: the preamble promises that every place this
// fake had to guess is listed in one place AND that each entry appears as an
// inline comment beside the code that implements it.
//
// It asserts three things:
//
//   - the roll above and doc.go's numbered entries are the same size, so an
//     entry added to doc.go without being tabled here fails;
//   - every name on the roll appears in doc.go, so a renamed entry fails;
//   - every name on the roll appears in at least one non-doc.go source file of
//     this package, so an entry with no point of use fails too.
func TestASSUMEDRegisterIsComplete(t *testing.T) {
	sources := readPackageSources(t)
	doc, ok := sources["doc.go"]
	if !ok {
		t.Fatal("doc.go is not in this package — the register has no home")
	}

	raw, err := os.ReadFile("doc.go")
	if err != nil {
		t.Fatalf("ReadFile doc.go: %v", err)
	}
	if got, want := len(entryHeading.FindAllString(string(raw), -1)), len(registerEntries); got != want {
		t.Errorf("doc.go carries %d numbered register entries, and the roll above names %d — one of the two has moved", got, want)
	}

	for _, name := range registerEntries {
		if !strings.Contains(doc, normalise(name)) {
			t.Errorf("doc.go does not carry the register entry %q under that name", name)
			continue
		}
		cited := false
		for file, text := range sources {
			if file == "doc.go" {
				continue
			}
			if strings.Contains(text, normalise(name)) {
				cited = true
				break
			}
		}
		if !cited {
			t.Errorf("no non-doc.go file of this package cites the register entry %q — the register's promise is that each entry sits beside the code that implements it", name)
		}
	}
}
