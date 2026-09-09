// SPDX-License-Identifier: GPL-3.0-or-later

package ma

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// THE REGISTERS' OWN GUARD.
//
// doc.go carries the AUTHORITATIVE ASSUMED register, the errata schedule and
// the A4 matrix's plan-level rulings. All three are prose, and prose decays:
// an entry loses its lift, a scope quietly widens from one registry row to
// two, a schedule row is "tidied away", a category is merged into its
// neighbour. These checks are STRUCTURAL — they cannot tell a correct scope
// from an incorrect one — but they catch the decays that have no other
// reader: a missing row, a row with no lift, a merged category, and the exact
// wording the scoping rule forbids.
//
// The semantic check is the milestone's closing review's, and this file does
// not pretend to be it.

// docSource is doc.go, read from the package directory the test runs in.
func docSource(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("doc.go")
	if err != nil {
		t.Fatalf("reading doc.go: %v", err)
	}
	return string(b)
}

// registerEntry matches a register row's header line: a tab-indented comment
// beginning with the row's ID.
var registerEntry = regexp.MustCompile(`(?m)^//\t(A[0-9]+) `)

// erratumEntry matches an errata-schedule row's header line.
var erratumEntry = regexp.MustCompile(`(?m)^//\t(E[0-9]+) `)

// matrixRuling matches one of the A4 matrix's plan-level rulings.
var matrixRuling = regexp.MustCompile(`(?m)^//\t(M-E[0-9]+) `)

// liftID matches any of the three lift-ID forms the register uses.
var liftID = regexp.MustCompile(`L-(HW|DOC|DEC)-[0-9]+[a-z]?`)

// entryBodies splits src into one body per header the regexp matched, so a
// per-row check reads that row and not its neighbour.
func entryBodies(src string, re *regexp.Regexp) map[string]string {
	idx := re.FindAllStringSubmatchIndex(src, -1)
	out := map[string]string{}
	for i, m := range idx {
		end := len(src)
		if i+1 < len(idx) {
			end = idx[i+1][0]
		}
		out[src[m[2]:m[3]]] = src[m[0]:end]
	}
	return out
}

// TestAssumedRegister_HoldsTwentyTwoRowsExactlyOnceEach pins the population.
//
// TWENTY-TWO A ROWS: twenty assumptions, one documented fact retained for
// safety (A16) and one recorded narrowing (A5). K-D1 and K-D2 are NOT among
// them — they are DRIVER-register entries and land in each driver package's
// own doc.go, which is what their prefix records.
func TestAssumedRegister_HoldsTwentyTwoRowsExactlyOnceEach(t *testing.T) {
	src := docSource(t)
	seen := map[string]int{}
	for _, m := range registerEntry.FindAllStringSubmatch(src, -1) {
		seen[m[1]]++
	}
	for i := 1; i <= 22; i++ {
		id := "A" + strconv.Itoa(i)
		if seen[id] != 1 {
			t.Errorf("register row %s appears %d times in doc.go, want exactly 1", id, seen[id])
		}
		delete(seen, id)
	}
	for extra, n := range seen {
		t.Errorf("doc.go carries an unaccounted register row %s (%d times) — this design's register is A1-A22", extra, n)
	}
}

// TestAssumedRegister_EveryRowNamesALift is the check that stops an entry
// decaying into an assumption nobody can retire: a lift reading "Hardware" or
// "an owner report" can be marked satisfied by a neighbouring command or an
// unspecified observation, which is precisely how an assumption gets retired
// without being tested.
//
// EXACTLY ONE ROW MAY HAVE NO LIFT, and it is named here rather than
// tolerated by a loose pattern: A16 is NOT ASSUMED — it is documentary fact
// (890:3217-3218, 990:2964-2965), kept in the table only because the whole of
// decision 9's narrower refusal rests on it. It says "NO LIFT NEEDED" in as
// many words, and that phrase is what this test accepts in place of a lift ID.
// A5 is a recorded NARROWING rather than an assumption and does carry one, so
// it is not an exception.
func TestAssumedRegister_EveryRowNamesALift(t *testing.T) {
	bodies := entryBodies(docSource(t), registerEntry)
	if len(bodies) == 0 {
		t.Fatal("found no register rows in doc.go — this test would pass vacuously")
	}
	for id, body := range bodies {
		saysNoneNeeded := strings.Contains(body, "NO LIFT NEEDED")
		if id == "A16" {
			if !saysNoneNeeded {
				t.Error("register row A16 no longer says NO LIFT NEEDED — it is documentary fact, and the sentence is what stops a reader treating it as an unlifted assumption")
			}
			continue
		}
		if saysNoneNeeded {
			t.Errorf("register row %s claims no lift is needed — A16 is the only documentary-fact row, and every assumption must name what would retire it", id)
		}
		if !liftID.MatchString(body) {
			t.Errorf("register row %s names no lift ID (L-HW-n, L-DOC-n or L-DEC-n) — an unnamed lift can be marked satisfied by any observation at all", id)
		}
	}
}

// TestAssumedRegister_NoEntryScopesItselfToTheMANUFACTURER is the scoping
// rule's own guard.
//
// This design registers TWO rows, in two different books, and every entry is
// per registry row unless it names one. An entry saying "a Kenwood" would
// scope a claim to a MANUFACTURER: a TS-890S owner's session would then
// retire an assumption on the TS-990S row nobody had touched. The two radios
// have different documents, different grids, different mode legends and a
// lockout encoding that disagrees.
func TestAssumedRegister_NoEntryScopesItselfToTheMANUFACTURER(t *testing.T) {
	src := docSource(t)
	for _, forbidden := range []string{"a Kenwood", "A Kenwood"} {
		if i := strings.Index(src, forbidden); i >= 0 {
			line := 1 + strings.Count(src[:i], "\n")
			t.Errorf("doc.go:%d scopes a claim to the manufacturer (%q) — this design has two rows in two books, and a lift stated over the pair would let one radio's observation retire the other's assumption", line, forbidden)
		}
	}
}

// TestAssumedRegister_DoesNotCarryTheDriverRegister pins the division:
// correcting an A-number is a DESIGN change; correcting a K-number is a
// DRIVER-PACKAGE change. K-D1 (the tone-mode semantics) and K-D2 (the
// control-line policy at open) belong in core/driver/ts890/doc.go and
// core/driver/ts990/doc.go, written TWICE with each row's own lift — a single
// shared entry would be the sibling inheritance the per-row rule exists to
// prevent, and a copy here would be a second place for an entry to be marked
// lifted.
func TestAssumedRegister_DoesNotCarryTheDriverRegister(t *testing.T) {
	src := docSource(t)
	driverEntry := regexp.MustCompile(`(?m)^//\t(K-D[0-9]+) `)
	for _, m := range driverEntry.FindAllStringSubmatch(src, -1) {
		t.Errorf("doc.go carries driver-register row %s — K rows land in core/driver/ts890/doc.go and core/driver/ts990/doc.go, not here", m[1])
	}
	// The prose that says where they DO live must survive, so that a reader
	// counting rows here does not conclude the register is short.
	if !strings.Contains(src, "K-D1") || !strings.Contains(src, "K-D2") {
		t.Error("doc.go no longer says where K-D1 and K-D2 live — a reader counting 22 rows against a register that also has two driver-side entries needs that sentence")
	}
}

// TestErrataSchedule_NineteenRowsInThreeCategories pins the schedule's
// population AND its partition, because the partition is the part a later
// reader gets wrong: two of the nineteen rows are not defects at all, and
// "correcting" either into evidence is exactly what recording them prevents.
//
// NINETEEN, NOT EIGHTEEN. Spec draft 3 minted an E19 that duplicated E5;
// draft 3a struck it and the count returned to eighteen; draft 3b added a
// DIFFERENT E19 — the 990S's EX Set/Answer diagram drawn fixed to 24 where
// the field beside it is variable — and the count is nineteen again. A doc.go
// carrying eighteen rows, or an E19 about the CN chart, was written from a
// stale draft, which is why the row's subject is pinned below and not only
// its number.
func TestErrataSchedule_NineteenRowsInThreeCategories(t *testing.T) {
	src := docSource(t)
	seen := map[string]int{}
	for _, m := range erratumEntry.FindAllStringSubmatch(src, -1) {
		seen[m[1]]++
	}
	for i := 1; i <= 19; i++ {
		id := "E" + strconv.Itoa(i)
		if seen[id] != 1 {
			t.Errorf("erratum %s appears %d times in doc.go, want exactly 1", id, seen[id])
		}
		delete(seen, id)
	}
	for extra, n := range seen {
		t.Errorf("doc.go carries an unaccounted erratum %s (%d times) — this design's schedule is E1-E19", extra, n)
	}

	categories := []struct {
		heading string
		want    int
	}{
		{"SEVENTEEN DOCUMENT DEFECTS:", 17},
		{"ONE CROSS-BOOK NAMING DIVERGENCE THAT IS A DEFECT OF NEITHER BOOK:", 1},
		{"ONE TRANSCRIPTION TRAP THAT IS NOT A DEFECT:", 1},
	}
	for i, c := range categories {
		start := strings.Index(src, c.heading)
		if start < 0 {
			t.Errorf("doc.go has no %q category heading", c.heading)
			continue
		}
		end := len(src)
		if i+1 < len(categories) {
			if next := strings.Index(src, categories[i+1].heading); next > start {
				end = next
			}
		}
		if got := len(erratumEntry.FindAllString(src[start:end], -1)); got != c.want {
			t.Errorf("category %q holds %d errata rows, want %d", c.heading, got, c.want)
		}
	}
}

// TestErrataSchedule_E19IsTheEXDiagramAndNotTheStruckDuplicate is the one
// content check in this file, and it earns its place: a stale doc.go would
// carry an E19 about the 890S's CN chart — draft 3's duplicate of E5, struck
// at draft 3a — and a purely numeric pin would pass on it.
func TestErrataSchedule_E19IsTheEXDiagramAndNotTheStruckDuplicate(t *testing.T) {
	body, ok := entryBodies(docSource(t), erratumEntry)["E19"]
	if !ok {
		t.Fatal("doc.go carries no E19 — draft 3b's schedule has nineteen rows")
	}
	if !strings.Contains(body, "EX") || !strings.Contains(body, "24") {
		t.Errorf("E19 does not record the 990S's EX Set/Answer diagram drawn fixed to 24 bytes:\n%s", body)
	}
	if strings.Contains(body, "CN") {
		t.Errorf("E19 names the CN chart — that was draft 3's duplicate of E5 and was STRUCK at draft 3a:\n%s", body)
	}
}

// TestMatrixRulings_AllNineAreRecorded pins M-E1 to M-E9, the A4 capability
// matrix's own findings against the design. Each is a plan-level ruling that
// names where it lands, and one of them (M-E9) lands INSIDE E5 rather than as
// a row of its own — which is exactly the kind of fold a later tidy-up drops.
func TestMatrixRulings_AllNineAreRecorded(t *testing.T) {
	src := docSource(t)
	seen := map[string]int{}
	for _, m := range matrixRuling.FindAllStringSubmatch(src, -1) {
		seen[m[1]]++
	}
	for i := 1; i <= 9; i++ {
		id := "M-E" + strconv.Itoa(i)
		if seen[id] != 1 {
			t.Errorf("matrix ruling %s appears %d times in doc.go, want exactly 1", id, seen[id])
		}
		delete(seen, id)
	}
	for extra, n := range seen {
		t.Errorf("doc.go carries an unaccounted matrix ruling %s (%d times) — the matrix raised nine", extra, n)
	}
}

// TestDesignCommitments_SurviveATidyUp pins the three NEGATIVE facts doc.go
// carries. Each is a statement about something this package does NOT have,
// which is the class of sentence a later reader deletes as redundant — and
// each is the answer to a question a reader coming from pair 1 will ask.
func TestDesignCommitments_SurviveATidyUp(t *testing.T) {
	src := docSource(t)
	for _, want := range []string{
		// The E;/O; route is core/kw's, unchanged.
		"inherited from core/kw unchanged",
		// MA0's length is a RANGE, not unbounded.
		"a RANGE rather than unbounded",
		// Decision 7's negative, and the whole defaulted-byte list.
		"no hard-wired byte",
		"one item on one row",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("doc.go no longer states the design commitment containing %q", want)
		}
	}
}
