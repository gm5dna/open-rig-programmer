// SPDX-License-Identifier: GPL-3.0-or-later

package fakets990

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
// core/kw/ma/doc.go's register, and this test is what makes it hold rather
// than merely be asked for.
//
// A name is spelled here EXACTLY as doc.go heads it, truncated to the part that
// is unambiguous — enough to identify the entry, short enough to survive an
// editorial tidy of the sentence it heads.
var registerEntries = []string{
	"AN ACCEPTED SET PRODUCES NO REPLY",
	"SET-DIRECTION FIELD STRICTNESS",
	"THE ANSWER'S CHANNEL NUMBER IS THREE ZERO-PADDED DIGITS",
	"THE SET'S P2 IS IGNORED AND THE ANSWER'S CLASS FOLLOWS FREQUENCY 2",
	"THE NAME WINDOW IS TEN BYTES, CARRIED VERBATIM",
	`A SET CARRYING AN "UNUSED" MODE NIBBLE IS STORED`,
	"TONE INDICES ARE STORED, NOT RANGE-CHECKED",
	"A SET TO A BLANK CHANNEL IS STORED",
	"SLOTS 100-119 ARE NOT SERVED",
	"THE DEFAULT IMAGE'S RECORD COMPOSITION",
	"THE DEFAULT FIRMWARE STRING",
	"AUTOMATIC-INFORMATION SUPPRESSION",
	`THE AI VALUES PRINTED "NOT USED" ARE REFUSED`,
	"THE AI STATE AT CONSTRUCTION",
	"THE FRAME ACCUMULATOR'S CAP AND RESYNC",
	"THE EX MENU VALUES ARE INVENTED",
	`AN OUT-OF-INVENTORY EX ADDRESS ANSWERS "?;"`,
	"THE FIXED FORM'S PAD BYTE IS A SPACE",
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

// TestPROVENANCECarriesA22Verbatim. PROVENANCE.md is where this package's
// evidence posture is stated at the strength the design settled on, and the
// plan asks for A22 FOR THIS ROW plus the family-level entries an image rides
// on, listed by number. A provenance note that had drifted from the design's
// own wording would be a second, weaker claim standing where the first one is
// cited.
func TestPROVENANCECarriesA22Verbatim(t *testing.T) {
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

	const a22 = "The fake's complete MA0 record images are compositions of printed VALUES, not records any radio has produced."
	if !strings.Contains(prov, normalise(a22)) {
		t.Error("PROVENANCE.md does not carry A22's sentence verbatim")
	}
	const perRow = "Per row: the 890S's image and the 990S's are two compositions"
	if !strings.Contains(prov, normalise(perRow)) {
		t.Error("PROVENANCE.md does not carry A22's PER ROW scope — an entry scoped to the manufacturer would let one radio's session retire the other row's assumption")
	}

	// The family-level entries an image rides on, BY NUMBER (the plan's P19).
	// A1 IS ON THIS LIST AND IS NOT ON internal/fakets890's: its pad/trim rule
	// is TS-990S ONLY, because this radio's name field is a fixed ten-byte
	// window (990:2955-2956) where the 890S's terminator floats after the
	// name. A4, A17 and A21 are the mirror image — 890S only, all three
	// consequences of a blank-channel note that stops at P12 — and A9, A10 and
	// A14 are excluded for their own reasons, which the file states.
	//
	// These checks run against the RAW markdown and match the bullet form the
	// file uses — "- **A2** — …" — in full, leading hyphen included. A plain
	// substring search is vacuous here: Contains(prov, "A2") is also satisfied
	// by "A21" and "A22", so a missing A2 bullet would never be caught, and
	// "**A2**" alone is no better — this file says "the design's **A22**" in
	// running prose, so a search that dropped the leading "- " would find the
	// entry's NAME anywhere in the document and never notice its BULLET going
	// missing. "- **A2**" cannot collide with "- **A21**" because the two
	// asterisks immediately follow the digits only in A2's own bullet, and it
	// cannot collide with prose because prose never opens a line with "- ".
	raw := string(b)
	for _, entry := range []string{"A1", "A2", "A5", "A6", "A16", "A22"} {
		if !strings.Contains(raw, "- **"+entry+"**") {
			t.Errorf("PROVENANCE.md does not carry the bullet - **%s**, one of the family-level entries an image rides on", entry)
		}
	}
	for _, entry := range []string{"A4", "A9", "A10", "A14", "A17", "A21"} {
		if strings.Contains(raw, "- **"+entry+"**") {
			t.Errorf("PROVENANCE.md lists %s among the entries the images ride on; it does not apply to this row's images", entry)
		}
	}
}

// TestPROVENANCERecordsTheTranscriptionBCopy. The fake's inventory is only
// transcription B's for as long as this side's copy really is B, so the
// provenance note has to say where the copy came from, that it is a COPY and
// not a move, and what to do if the cross-check ever fires. It must also carry
// ruling R-B's own citation, because that ruling is the one place this
// projection departs from the leg it reads.
func TestPROVENANCERecordsTheTranscriptionBCopy(t *testing.T) {
	b, err := os.ReadFile("PROVENANCE.md")
	if err != nil {
		t.Fatalf("ReadFile PROVENANCE.md: %v", err)
	}
	raw := string(b)
	for _, want := range []string{
		"core/kw/ma/testdata/transcription-b-990s.csv",
		"b234f34ed45939bcde0ac7c90aad18c2ed857eb1b0b7f32d322aaff73ad3775a",
		"990:1747-1748",
		"UNVERIFIED",
	} {
		if !strings.Contains(raw, want) {
			t.Errorf("PROVENANCE.md does not name %q", want)
		}
	}
}
