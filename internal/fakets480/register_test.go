// SPDX-License-Identifier: GPL-3.0-or-later

package fakets480

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
// core/kw/doc.go's register, and this test is what makes it hold rather than
// merely be asked for.
//
// A name is spelled here EXACTLY as doc.go heads it, truncated to the part
// that is unambiguous — enough to identify the entry, short enough to survive
// an editorial tidy of the sentence it heads.
var registerEntries = []string{
	"AN ACCEPTED SET PRODUCES NO REPLY",
	"SET-DIRECTION FIELD STRICTNESS",
	"AN UNWRITTEN CHANNEL ANSWERS THE ZERO RECORD",
	"THE TRANSMIT HALF OF A CHANNEL WITH NO STORED RECORD",
	"A SET CARRYING AN UNUSED MODE NIBBLE IS STORED",
	"TONE INDICES ARE STORED, NOT RANGE-CHECKED",
	"THE STEP INDEX IS STORED, NOT RANGE-CHECKED",
	"A SET DOES NOT MOVE THE SELECTED CHANNEL",
	"THE SELECTED CHANNEL AT CONSTRUCTION",
	"THE DEFAULT IMAGE'S RECORD COMPOSITION",
	"THE DEFAULT TY ANSWER",
	"THE INITIAL AI VALUE IS THE POWER-OFF ONE",
	"AUTOMATIC-INFORMATION SUPPRESSION",
	"THE FRAME ACCUMULATOR'S CAP AND RESYNC",
	"A REFUSAL TO A CONTROL BYTE IS ALWAYS ANSWERED",
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
// "register entry" phrase in the code names a tabled entry: those citations
// are prose, phrased several ways, and a regexp over them would pin the
// phrasing rather than the fact.
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

// TestPROVENANCECarriesA27Verbatim. PROVENANCE.md is where this package's
// evidence posture is stated at the strength the design settled on, and the
// plan asks for A27's sentence VERBATIM plus the family-level entries an
// image rides on, listed by number. A provenance note that had drifted from
// the design's own wording would be a second, weaker claim standing where the
// first one is cited.
func TestPROVENANCECarriesA27Verbatim(t *testing.T) {
	b, err := os.ReadFile("PROVENANCE.md")
	if err != nil {
		t.Fatalf("ReadFile PROVENANCE.md: %v", err)
	}
	// The quotation lives in a Markdown blockquote, so the block markers and
	// the emphasis/code markup have to come off before the WORDS can be
	// compared. Stripping them here rather than forbidding them in the file
	// keeps the test about the sentence and not about its typography.
	markup := strings.NewReplacer(">", " ", "*", " ", "`", " ")
	prov := normalise(markup.Replace(string(b)))

	const a27 = "Every byte value in a fake image is a constant, example or legend value printed in the radio's own PC-command document. The 50-byte RECORD is a synthetic composition of those values: no MR, MW or MC frame is printed anywhere in either book, so no image's cross-field combination has ever been printed or observed. These are not observed contents and not factory defaults."
	if !strings.Contains(prov, normalise(a27)) {
		t.Error("PROVENANCE.md does not carry A27's sentence verbatim")
	}

	// The family-level entries an image rides on, BY NUMBER (the plan's P19).
	// TWO of the 590 pair's five are deliberately absent here, and each
	// absence is a fact about this radio rather than an omission: A10 says in
	// its own text that the TS-480 is unclaimed by it (byte 4 is a printed
	// constant here, not a hundreds digit), and A18a is documentary on the 590
	// pair alone — the TS-480 half of that question is A4, which IS listed and
	// is this row's release gate.
	//
	// These checks run against the RAW markdown (before the "*" markup
	// stripper above runs), and match the bullet form the file actually uses
	// — "- **A1** — …" — in full, leading hyphen included, on BOTH loops
	// below. A plain substring search on the stripped text is vacuous here:
	// Contains(prov, "A1") is also satisfied by "A10", "A18a" and "A24", so a
	// missing A1 bullet would never be caught, and "**A1**" alone is no
	// better — this file also says "an image rides on **A1**" and similar in
	// running prose for A3, A4 and A27, so a search that drops the leading
	// "- " would find the entry's NAME anywhere in the document and never
	// notice its BULLET going missing. "- **A1**" cannot collide with
	// "- **A10**" or "- **A18a**" because the two asterisks immediately
	// follow the digits only in A1's own bullet, and it cannot collide with
	// prose because prose never opens a line with "- ".
	raw := string(b)
	for _, entry := range []string{"A1", "A3", "A4", "A24", "A27"} {
		if !strings.Contains(raw, "- **"+entry+"**") {
			t.Errorf("PROVENANCE.md does not carry the bullet - **%s**, one of the family-level entries an image rides on", entry)
		}
	}
	for _, entry := range []string{"A10", "A18a"} {
		if strings.Contains(raw, "- **"+entry+"**") {
			t.Errorf("PROVENANCE.md lists %s among the entries the images ride on — it does not claim this row", entry)
		}
	}
}
