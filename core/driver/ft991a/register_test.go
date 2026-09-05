// SPDX-License-Identifier: GPL-3.0-or-later

package ft991a

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sharedRegisterEntryNames quotes the ELEVEN entry names of the DIALECT's
// ASSUMED register (core/cat/ft991a/doc.go) VERBATIM, in that register's own
// order — the same eleven strings core/cat/ft991a/dialect_test.go's
// registerEntryNames carries, and for the reason it gives: the register is
// ONE statement of record for the radio, carried in the dialect's doc.go and
// in this driver's (spec §The ASSUMED register), and nothing mechanical held
// the second copy to the first until this package existed.
//
// THAT IS WHAT THIS FILE IS FOR. The list below is checked against the
// dialect's own doc.go and against this package's, so an entry renamed in
// either file fails here rather than leaving two registers quietly disagreeing
// about what the radio's assumptions are called. It is deliberately not
// imported from the dialect package — registerEntryNames is a test-only
// identifier over there and unreachable from here — so the literal is the
// bridge and both ends are asserted against it.
//
// FOUR OF THE ELEVEN NAMES EMBED A VALUE — TagFill's ' ', the combined
// answer's 41, NoneWire's "000", the clarifier's 10 and 9990. A partial lift
// that changed one of those values would invalidate every by-name citation of
// it in this package, which is why the names live in one place and a test
// finds the citations rather than a reader.
var sharedRegisterEntryNames = []string{
	`MTPolicy.TagFill = ' '`,
	"THE COMBINED MT ANSWER'S EXACT LENGTH, 41",
	`SlotSpace.NoneWire = "000"`,
	"THE cat.ModeUnset MEMBER OF THE MODE TABLE",
	"ClarifierPolicy.StepHz = 10 AND ClarifierPolicy.MaxAbsHz = 9990",
	`THE CLARIFIER'S MINUS-DIRECTION BYTE, the ASCII HYPHEN-MINUS 0x2D ('-')`,
	"THE DCS STATES' SET ACCEPTANCE",
	"ROW 087 RADIO ID'S EXCLUSION",
	"FRAMING: 8 DATA BITS, NO PARITY, TWO STOP BITS",
	"DefaultBaud 38400",
	"THE ACKNOWLEDGEMENT CONVENTIONS",
}

// driverRegisterEntryNames quotes the TEN entries this driver adds beside
// the shared eleven, in doc.go's order — the matrix's §4b list, which the
// spec's own six-item reminder does not enumerate (matrix erratum M-E1: five
// of that list's six are already on the shared register, only the MT "?;"
// entry is new, and this matrix reaches ten).
//
// THREE OF THE TEN WERE ONE ENTRY UNTIL matrix erratum M-E10. A single
// "TONE-NUMBER, DCS-CODE AND SCAN-SKIP UNREACHABILITY" bundled three
// independent claims under one name against its own three separate lifting
// experiments, and the register's own rule — one entry, ONE capture — is what
// condemned it: a tone-number byte turning up says nothing about the DCS code
// or the skip flag, so an operator who takes one capture could retire no part
// of the bundle. §4b labels the three 4a, 4b and 4c; they are entries 4, 5
// and 6 of the numbered list in this package's doc.go, which is exactly why
// both registers say CITE BY NAME, NEVER BY POSITION.
//
// These are facts about the DRIVER's choreography and its capability values,
// where the eleven are facts about the dialect and the codec. Correcting one
// of these is a change in this package; correcting one of the eleven is a
// change in core/cat/ft991a. NEITHER REGISTER MAY ABSORB THE OTHER.
var driverRegisterEntryNames = []string{
	"CONTROL-LINE POLICY",
	"MinFreqHz 30 000 / MaxFreqHz 470 000 000 — THE FA/FB RANGE READ AS THE MEMORY-STORABLE RANGE",
	`RequiredSlots {"001"}`,
	"TONE-NUMBER UNREACHABILITY",
	"DCS-CODE UNREACHABILITY",
	"SCAN-SKIP UNREACHABILITY",
	`MT "?;" ON A MEMORY OR PMS SLOT MEANS THE SLOT IS EMPTY`,
	"THE MODE NIBBLE'S DOMAIN",
	"THE PRINTED-FIXED BYTES ARE ANSWERED AS PRINTED",
	"A DCS-STATE CHANNEL'S CODE SURVIVES A REWRITE",
}

// registerSection returns the text of the "# The ASSUMED register" section of
// the doc.go at path, bounded by its own heading and the next one.
//
// It reads a COMMITTED SOURCE FILE, so unlike the manual extraction it is
// present in a fresh clone and in CI — the same property core/cat/ft991a's
// own TestASSUMEDRegisterIsComplete relies on.
func registerSection(t *testing.T, path string) string {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	const startHeading = "// # The ASSUMED register"
	start := strings.Index(string(src), startHeading)
	if start < 0 {
		t.Fatalf("%s has no %q heading — the register is this radio's statement of record", path, startHeading)
	}
	rest := string(src)[start+len(startHeading):]
	end := strings.Index(rest, "\n// # ")
	if end < 0 {
		// The last section of the file: everything to the package clause.
		end = strings.Index(rest, "\npackage ")
	}
	if end < 0 {
		t.Fatalf("%s's ASSUMED register is not followed by another heading or the package clause — this test cannot tell where the section ends", path)
	}
	return rest[:end]
}

// bulletsAndNumbered splits a register section into its "  - " bullets and
// its " 1. " numbered entries, each UNWRAPPED into one line so that a name
// the comment happened to wrap across two lines is still found by a plain
// prefix test.
//
// Go doc comments indent a bullet's marker three spaces and a numbered
// entry's two, and both continue at five — which is what makes the two lists
// separable without the section having to label them.
func bulletsAndNumbered(section string) (bullets, numbered []string) {
	last := func(s []string) *string {
		if len(s) == 0 {
			return nil
		}
		return &s[len(s)-1]
	}
	var inNumbered bool
	for _, line := range strings.Split(section, "\n") {
		rest, ok := strings.CutPrefix(line, "//")
		if !ok {
			continue
		}
		switch {
		case strings.HasPrefix(rest, "   - "):
			bullets = append(bullets, strings.TrimSpace(rest[len("   - "):]))
			inNumbered = false
		case isNumberedEntry(rest):
			numbered = append(numbered, strings.TrimSpace(rest[strings.Index(rest, ".")+1:]))
			inNumbered = true
		case strings.HasPrefix(rest, "     "):
			target := last(bullets)
			if inNumbered {
				target = last(numbered)
			}
			if target != nil {
				*target += " " + strings.TrimSpace(rest)
			}
		}
	}
	return bullets, numbered
}

// registerEntryOpensWith reports whether entry opens with name AND ENDS THE
// NAME THERE.
//
// A plain strings.HasPrefix is the obvious test and it is not enough: an
// entry that GROWS a qualifier — "THE ACKNOWLEDGEMENT CONVENTIONS" becoming
// "… CONVENTIONS ZZPROBE" — still has the old name as a prefix, so it drifts
// silently and every by-name citation of the old spelling goes stale
// unnoticed. Growing a qualifier is the commonest way an entry in this
// repository is renamed, which makes it exactly the case worth catching
// (task 10 review, LOW-1).
//
// HasPrefix cannot simply become equality, because a register entry
// continues from its name straight into prose. What ends a name instead is
// one of the four terminators BOTH registers use — a colon, a full stop, a
// bracketed file citation, or a spaced em-dash — and a name followed by a
// space and another word is therefore a rename, not a match.
func registerEntryOpensWith(entry, name string) bool {
	rest, ok := strings.CutPrefix(entry, name)
	if !ok {
		return false
	}
	if rest == "" {
		return true
	}
	for _, terminator := range []string{":", ".", " — ", " ("} {
		if strings.HasPrefix(rest, terminator) {
			return true
		}
	}
	return false
}

// isNumberedEntry reports whether a comment line opens a Go doc-comment
// numbered list item, i.e. "  1. " — two spaces, digits, a full stop, a
// space. Written out rather than regexp'd so the shape it accepts is the
// shape gofmt produces and nothing wider.
//
// THE TWO SPACES ARE LOAD-BEARING AND A TEN-ENTRY LIST IS WHERE THAT BITES:
// gofmt writes "//  10." with the same two-space marker indent as "//  1.",
// and a hand-written "// 10." parses as neither an entry nor a continuation,
// so the entry vanishes and its body is glued onto its predecessor. The
// count assertions below catch that — they did, once — but the cost is a
// confusing failure, so the shape is stated here rather than left implicit.
func isNumberedEntry(rest string) bool {
	body, ok := strings.CutPrefix(rest, "  ")
	if !ok || body == "" || body[0] < '0' || body[0] > '9' {
		return false
	}
	dot := strings.Index(body, ". ")
	if dot < 0 {
		return false
	}
	for i := 0; i < dot; i++ {
		if body[i] < '0' || body[i] > '9' {
			return false
		}
	}
	return true
}

// TestSharedRegisterNamesMatchTheDialects is the CROSS-PACKAGE half, and the
// reason this file exists: the eleven names quoted above are asserted against
// core/cat/ft991a/doc.go's own register, entry for entry and in order.
//
// The spec requires ONE register carried in two files. Until this package
// existed, nothing could hold the driver's copy to the dialect's, and the
// dialect's own test says so in terms ("this slice is the form the driver
// package mirrors"). A rename in either file now fails here — a rename at
// the head of the name, and, since registerEntryOpensWith replaced a plain
// HasPrefix, a name that merely GROWS a qualifier as well.
func TestSharedRegisterNamesMatchTheDialects(t *testing.T) {
	section := registerSection(t, filepath.Join("..", "..", "cat", "ft991a", "doc.go"))
	bullets, numbered := bulletsAndNumbered(section)
	if len(numbered) != 0 {
		t.Errorf("core/cat/ft991a's register has %d numbered entries — its eleven are a bullet list, and this test reads them as one", len(numbered))
	}
	if len(bullets) != len(sharedRegisterEntryNames) {
		t.Fatalf("core/cat/ft991a's register holds %d entries, this package quotes %d — an entry added to one register and not the other is exactly the drift this test exists to stop; the bullets are %q", len(bullets), len(sharedRegisterEntryNames), bullets)
	}
	for i, name := range sharedRegisterEntryNames {
		if !registerEntryOpensWith(bullets[i], name) {
			t.Errorf("dialect register entry %d opens %q, this package quotes it %q — the eleven names are mirrored VERBATIM, and every citation of an entry anywhere is by name", i+1, bullets[i], name)
		}
	}
}

// TestDriverRegisterCarriesBothHalves holds THIS package's doc.go to both
// lists: the shared eleven as a bullet list, in the dialect's order and by
// its names, and this driver's own eight as a numbered list, each naming the
// ONE capture that lifts it.
//
// The "STAGE R LIFTS IT WITH:" count is asserted rather than merely spot
// checked because a register whose entries do not say what would settle them
// is a list of caveats rather than a plan — the same rule the dialect's own
// register test applies to its eleven.
func TestDriverRegisterCarriesBothHalves(t *testing.T) {
	section := registerSection(t, "doc.go")
	bullets, numbered := bulletsAndNumbered(section)

	if len(bullets) != len(sharedRegisterEntryNames) {
		t.Fatalf("this driver's register cites %d dialect entries, want the %d the dialect declares — the driver carries the same eleven, cited at their dependence sites and never restated; the bullets are %q", len(bullets), len(sharedRegisterEntryNames), bullets)
	}
	for i, name := range sharedRegisterEntryNames {
		if !registerEntryOpensWith(bullets[i], name) {
			t.Errorf("driver register citation %d opens %q, the dialect names that entry %q", i+1, bullets[i], name)
		}
	}

	if len(numbered) != len(driverRegisterEntryNames) {
		t.Fatalf("this driver's register holds %d own entries, want %d (matrix §4b); the entries are %q", len(numbered), len(driverRegisterEntryNames), numbered)
	}
	for i, name := range driverRegisterEntryNames {
		if !registerEntryOpensWith(numbered[i], name) {
			t.Errorf("driver register entry %d opens %q, this file names it %q — CITE BY NAME, NEVER BY POSITION, so the name is what must not drift", i+1, numbered[i], name)
		}
	}
	for i, entry := range numbered {
		if !strings.Contains(entry, "STAGE R LIFTS IT WITH:") {
			t.Errorf("driver register entry %d (%q) names no lifting capture — every entry states the ONE capture that settles it", i+1, driverRegisterEntryNames[i])
		}
	}

	// The two rules the register's own prose must keep saying, because both
	// are the kind a later editor removes as boilerplate: entries are cited
	// by NAME (a positional citation silently points at the wrong assumption
	// once one is inserted), and neither register may absorb the other's
	// purpose even though this milestone shares their CONTENT.
	for _, want := range []string{
		"CITE THESE ENTRIES BY NAME, NEVER BY POSITION",
		"NEITHER REGISTER MAY ABSORB THE OTHER",
		"TEN",
	} {
		if !strings.Contains(section, want) {
			t.Errorf("this driver's register no longer says %q", want)
		}
	}
}
