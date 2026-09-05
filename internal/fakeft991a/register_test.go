// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft991a

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// registerEntries is the ASSUMED register's roll, by NAME — the names doc.go
// heads its numbered entries with and the names this package's code cites at
// every dependence site. The order matches doc.go's numbering, which exists for
// readability only: the CITE-BY-NAME rule is doc.go's own, inherited from
// core/cat/ft991a/doc.go's register, and this test is what makes it hold rather
// than merely be asked for.
//
// A name is spelled here EXACTLY as doc.go heads it, truncated to the part that
// is unambiguous — enough to identify the entry, short enough to survive an
// editorial tidy of the sentence it heads.
//
// TWELVE, WHERE internal/fakeft891's ROLL HAS SIXTEEN, and the difference is
// not a thinner register. Two of that package's entries are this radio's
// DIALECT register's rather than its fake's ("THE ACKNOWLEDGEMENT CONVENTIONS"
// covers both the "?;" convention and silence on an accepted Set); one — "MT
// READ IS ANSWERED, BY DEFAULT" — exists only because the FT-891's manual
// contradicts itself and this one does not; and one — "P7 IN AN MT ANSWER IS
// '1' (Memory)" — is a MANUAL FACT here, printed in MT's own legend, leaving
// only the PMS half assumed.
//
// THE LAST TWO ARRIVED WITH THE MENU INVENTORY, which is why the roll was TEN
// when this file was written: EX was deliberately unmodelled in the task that
// built this package's core, so its two assumptions had nothing to sit beside.
var registerEntries = []string{
	"EMPTY-SLOT ANSWERS",
	"PMS SLOTS ANSWER P7 '1'",
	"THE CLARIFIER IS STORED",
	"AN MT SET CREATES AN ABSENT CHANNEL",
	"SET-DIRECTION FIELD STRICTNESS",
	"THE TAG IS STORED TRIMMED AND ANSWERED PADDED",
	"A SET DOES NOT MOVE THE SELECTED CHANNEL",
	"THE DEFAULT IMAGE'S CONTENT IS INVENTED",
	"THE FRAME ACCUMULATOR'S CAP AND RESYNC",
	"AUTOMATIC-INFORMATION SUPPRESSION",
	"THE EX MENU VALUES ARE INVENTED",
	`AN OUT-OF-INVENTORY EX ADDRESS ANSWERS "?;"`,
}

// dialectEntries are the entries of core/cat/ft991a/doc.go's ELEVEN-member
// register that this package DEPENDS ON, spelled as that register heads them.
// They are cited here and never re-registered — doc.go's own rule, and the
// reason this package's roll is shorter than internal/fakeft891's.
//
// The test below holds only that each is cited SOMEWHERE in this package. It
// deliberately does not assert the reverse direction, or that the dialect's
// register still contains them: this package must not import that one, and a
// test that read core/cat/ft991a/doc.go from disk would be asserting a file
// outside the package it tests. What it does catch is the failure that
// actually happens — a citation deleted or reworded during a refactor, leaving
// an assumption in force with nothing naming it.
var dialectEntries = []string{
	`MTPolicy.TagFill = ' '`,
	"THE COMBINED MT ANSWER'S EXACT LENGTH, 41",
	`SlotSpace.NoneWire = "000"`,
	"THE cat.ModeUnset MEMBER OF THE MODE TABLE",
	"ClarifierPolicy.StepHz = 10 AND ClarifierPolicy.MaxAbsHz = 9990",
	"THE CLARIFIER'S MINUS-DIRECTION BYTE",
	"THE DCS STATES' SET ACCEPTANCE",
	"THE ACKNOWLEDGEMENT CONVENTIONS",
}

// normalise strips Go comment markers and collapses every run of whitespace to
// one space, so that a register name wrapped across two comment lines is found
// by the same search as one that fits on a line. Without it this test would
// enforce a line-wrapping accident rather than a citation.
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
// claim together mechanically, which is the claim's whole value: the preamble
// promises that every place this fake had to guess is listed in one place AND
// that each entry appears as an inline comment beside the code that implements
// it. A register nobody checks drifts from the code it describes within a
// milestone.
//
// It asserts three things:
//
//   - the roll above and doc.go's numbered entries are the same size, so an
//     entry added to doc.go without being tabled here fails;
//   - every name on the roll appears in doc.go, so a renamed entry fails;
//   - every name on the roll appears in at least one non-doc.go source file of
//     this package, so an entry with no point of use — a register that has
//     outlived its code — fails too.
//
// What it deliberately does NOT assert is the reverse direction, that every
// "register entry" phrase in the code names a tabled entry: those citations are
// prose, phrased several ways, and a regexp over them would pin the phrasing
// rather than the fact.
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

// TestDialectRegisterEntriesAreCitedByName is the other half of the same
// discipline, and it is the half internal/fakeft891 does not have: this radio's
// DIALECT carries an eleven-entry register, this fake depends on eight of them,
// and doc.go's rule is that they are cited by name here and never re-registered.
// A citation that is silently reworded — or deleted with the code it explained —
// leaves an assumption in force with nothing naming it, which is exactly the
// drift the by-name rule exists to prevent.
//
// It requires each name in doc.go (where the dependence is declared) AND in at
// least one other file (where it is depended upon), so a citation cannot be
// satisfied by the doc comment alone.
func TestDialectRegisterEntriesAreCitedByName(t *testing.T) {
	sources := readPackageSources(t)
	doc := sources["doc.go"]

	for _, name := range dialectEntries {
		want := normalise(name)
		if !strings.Contains(doc, want) {
			t.Errorf("doc.go does not name the DIALECT register entry %q — its list of the entries this package depends on has drifted", name)
		}
		cited := false
		for file, text := range sources {
			if file == "doc.go" {
				continue
			}
			if strings.Contains(text, want) {
				cited = true
				break
			}
		}
		if !cited {
			t.Errorf("no non-doc.go file of this package cites the DIALECT register entry %q — an assumption in force with nothing naming it at the code that rests on it", name)
		}
	}
}
