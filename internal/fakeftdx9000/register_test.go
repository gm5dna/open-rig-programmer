// SPDX-License-Identifier: GPL-3.0-or-later

package fakeftdx9000

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// registerEntries is the ASSUMED register's roll, by NAME — the names doc.go
// heads its numbered entries with and the names this package's code cites at
// every dependence site. Same CITE-BY-NAME discipline as every sibling fake
// (internal/fakeft991a's register_test.go is this file's template); this
// package carries no separate dialect register to check against, because its
// dialect facts live in core/driver/ftdx9000, which this package is forbidden
// to read.
var registerEntries = []string{
	`THE "?;" REJECTION CONVENTION`,
	`EMPTY-SLOT ANSWERS "?;"`,
	"PMS SLOTS ANSWER P7 '1'",
	"THE CLARIFIER IS STORED",
	"SET-DIRECTION FIELD STRICTNESS",
	"A SET DOES NOT MOVE THE SELECTED CHANNEL",
	"THE DEFAULT IMAGE'S CONTENT IS INVENTED",
	"AUTOMATIC-INFORMATION SUPPRESSION",
	"THE FRAME ACCUMULATOR'S CAP AND RESYNC",
	"THE DEFAULT CAT-ID ANSWER",
}

// normalise strips Go comment markers and collapses every run of whitespace to
// one space, so a register name wrapped across two comment lines is found by
// the same search as one that fits on a line.
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

// TestASSUMEDRegisterIsComplete holds doc.go's completeness claim mechanically:
// the roll above and doc.go's numbered entries are the same size, every name
// on the roll appears in doc.go, and every name on the roll appears in at
// least one non-doc.go source file (so a register entry with no point of use
// fails, and a citation deleted during a refactor fails too).
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
