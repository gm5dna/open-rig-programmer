// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// THE REGISTER'S OWN GUARD.
//
// doc.go carries the AUTHORITATIVE ASSUMED register and the errata
// schedule. Both are prose, and prose decays: an entry loses its lift, a
// scope quietly widens from one registry row to two, a schedule row is
// "tidied away". These checks are structural — they cannot tell a correct
// scope from an incorrect one — but they catch the three decays that have
// no other reader: a missing row, a row with no lift, and the exact wording
// the scoping rule forbids.
//
// The semantic check is T20's and the HANDOFF's, and this test does not
// pretend to be it.

// docSource is doc.go, read from the package directory the test runs in.
func docSource(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("doc.go")
	if err != nil {
		t.Fatalf("reading doc.go: %v", err)
	}
	return string(b)
}

// registerEntry matches a register row's header line: a tab-indented
// comment beginning with the row's ID.
var registerEntry = regexp.MustCompile(`(?m)^//\t(A[0-9]+[ab]?) `)

// erratumEntry matches an errata-schedule row's header line.
var erratumEntry = regexp.MustCompile(`(?m)^//\t(E[0-9]+) `)

// liftID matches any of the three lift-ID forms the register uses.
var liftID = regexp.MustCompile(`L-(HW|DOC|DEC)-[0-9]+[a-z]?`)

// wantRegisterRows is the register's population, stated here so a row that
// disappears fails a test rather than a reader's attention.
//
// TWENTY-EIGHT A ROWS: A1-A17, A18a, A18b, A19-A27. A18 is SPLIT into
// A18a and A18b, not added alongside a surviving A18 — "A1-A27 plus
// A18a/A18b" over-counts by one. K-D1 and K-D2 are NOT among them: they are
// driver-register entries and land in the drivers' own doc.go files, which
// is what their prefix records. Twenty-eight here plus those two is the
// complete thirty-row register.
var wantRegisterRows = []string{
	"A1", "A2", "A3", "A4", "A5", "A6", "A7", "A8", "A9", "A10",
	"A11", "A12", "A13", "A14", "A15", "A16", "A17", "A18a", "A18b",
	"A19", "A20", "A21", "A22", "A23", "A24", "A25", "A26", "A27",
}

// TestAssumedRegister_HoldsEveryRowExactlyOnce pins the population.
func TestAssumedRegister_HoldsEveryRowExactlyOnce(t *testing.T) {
	src := docSource(t)
	seen := map[string]int{}
	for _, m := range registerEntry.FindAllStringSubmatch(src, -1) {
		seen[m[1]]++
	}
	for _, want := range wantRegisterRows {
		switch seen[want] {
		case 1:
		case 0:
			t.Errorf("register row %s is missing from doc.go", want)
		default:
			t.Errorf("register row %s appears %d times in doc.go, want exactly 1", want, seen[want])
		}
		delete(seen, want)
	}
	for extra, n := range seen {
		t.Errorf("doc.go carries an unaccounted register row %s (%d times) — add it to wantRegisterRows or remove it", extra, n)
	}
	if len(wantRegisterRows) != 28 {
		t.Errorf("wantRegisterRows has %d entries, want 28 A rows", len(wantRegisterRows))
	}
	// A18 must be SPLIT, not merely joined by its two halves.
	if strings.Contains(src, "//\tA18 ") {
		t.Error("doc.go carries a surviving A18 alongside A18a and A18b — the row is SPLIT, and keeping all three over-counts the register by one")
	}
}

// TestAssumedRegister_EveryRowNamesALift is the check that stops an entry
// decaying into an assumption nobody can retire: a lift reading "Hardware"
// or "an owner report" can be marked satisfied by a neighbouring command or
// an unspecified observation, which is precisely how an assumption gets
// retired without being tested.
//
// EXACTLY ONE ROW MAY HAVE NO LIFT, and it is named here rather than
// tolerated by a loose pattern: A18a is NOT ASSUMED — it is documentary
// fact (590:1492-1493), kept in the table only so that no later reader
// restores draft 1's refusal of an empty channel. It says "NO LIFT NEEDED"
// in as many words, and that phrase is what this test accepts in place of a
// lift ID. A16 is a recorded NARROWING rather than an assumption and does
// carry one (L-DEC-2), so it is not an exception.
func TestAssumedRegister_EveryRowNamesALift(t *testing.T) {
	src := docSource(t)
	idx := registerEntry.FindAllStringSubmatchIndex(src, -1)
	if len(idx) == 0 {
		t.Fatal("found no register rows in doc.go — this test would pass vacuously")
	}
	for i, m := range idx {
		id := src[m[2]:m[3]]
		end := len(src)
		if i+1 < len(idx) {
			end = idx[i+1][0]
		}
		body := src[m[0]:end]
		hasLift := liftID.MatchString(body)
		saysNoneNeeded := strings.Contains(body, "NO LIFT NEEDED")
		if id == "A18a" {
			if !saysNoneNeeded {
				t.Errorf("register row A18a no longer says NO LIFT NEEDED — it is documentary fact, and the sentence is what stops a reader treating it as an unlifted assumption")
			}
			continue
		}
		if saysNoneNeeded {
			t.Errorf("register row %s claims no lift is needed — A18a is the only documentary-fact row, and every assumption must name what would retire it", id)
		}
		if !hasLift {
			t.Errorf("register row %s names no lift ID (L-HW-n, L-DOC-n or L-DEC-n) — an unnamed lift can be marked satisfied by any observation at all", id)
		}
	}
}

// TestAssumedRegister_NoEntrySaysAKenwoodPairIsOneRadio is the scoping
// rule's own guard, and it is the one the plan makes a T20 checkbox.
//
// This design registers THREE rows. The bare singular names TWO of them,
// and every time it appeared in an earlier draft it hid a real defect: an
// owner's TS-590S session would have lifted A9 and unblocked channel writes
// on the TS-590SG row nobody had touched, A10 would have certified MW
// addressing on the unobserved sibling, and A23 would have enabled non-FM
// writes on it. The two radios have different firmware, different menu
// domains and a byte whose liveness differs between them.
//
// The check is exact: every occurrence of the string this test looks for
// must be continued by an S or a G, which is what makes it name one row.
func TestAssumedRegister_NoEntrySaysAKenwoodPairIsOneRadio(t *testing.T) {
	src := docSource(t)
	const forbidden = "a TS-590"
	for i := 0; ; {
		j := strings.Index(src[i:], forbidden)
		if j < 0 {
			break
		}
		at := i + j
		next := at + len(forbidden)
		if next >= len(src) || (src[next] != 'S' && src[next] != 'G') {
			line := 1 + strings.Count(src[:at], "\n")
			t.Errorf("doc.go:%d names the 590 pair with a bare singular: %q — that is TWO registry rows, and a lift stated over it would let one sibling's observation retire the other's assumption", line, strings.TrimSpace(lineAt(src, at)))
		}
		i = next
	}
}

// lineAt returns the whole line containing byte offset at.
func lineAt(src string, at int) string {
	start := strings.LastIndexByte(src[:at], '\n') + 1
	end := strings.IndexByte(src[at:], '\n')
	if end < 0 {
		return src[start:]
	}
	return src[start : at+end]
}

// TestAssumedRegister_DoesNotCarryTheDriverRegister pins P21's division:
// correcting an A-number is a design change; correcting a K-number is a
// driver-package change. K-D1 (the TS-480's P7 direction semantics) and
// K-D2 (the control-line policy at open) belong in the drivers' own doc.go
// files, and a copy here would be a second place for an entry to be marked
// lifted — the exact thing one register exists to prevent.
func TestAssumedRegister_DoesNotCarryTheDriverRegister(t *testing.T) {
	src := docSource(t)
	driverEntry := regexp.MustCompile(`(?m)^//\t(K-D[0-9]+) `)
	for _, m := range driverEntry.FindAllStringSubmatch(src, -1) {
		t.Errorf("doc.go carries driver-register row %s — K rows land in core/driver/ts590/doc.go and core/driver/ts480/doc.go, not here", m[1])
	}
	// The prose that says where they DO live must survive, so that a
	// reader counting rows here does not conclude the register is short.
	if !strings.Contains(src, "K-D1") || !strings.Contains(src, "K-D2") {
		t.Error("doc.go no longer says where K-D1 and K-D2 live — a reader counting 28 rows against a thirty-row register needs that sentence")
	}
}

// TestErrataSchedule_TwentyTwoRowsInThreeCategories pins the schedule's
// population AND its partition, because the partition is the part a later
// reader gets wrong: two of the twenty-two are not defects at all, and
// "correcting" either into evidence is exactly what recording them
// prevents.
//
// E22 IS THE ONE THE CROSS-CHECKS COULD NOT FIND. It was routed here by the
// orchestrator out of T10's cross-check, which discovered that all three
// TS-480 menu-chart transcription legs read menu 034 as a two-digit row and
// the EX block's own prose omits it from the two-digit list. Three faithful
// readings of one incomplete printed sentence agree perfectly, so no
// comparison between the legs can catch it and only a recorded erratum can.
func TestErrataSchedule_TwentyTwoRowsInThreeCategories(t *testing.T) {
	src := docSource(t)
	seen := map[string]int{}
	for _, m := range erratumEntry.FindAllStringSubmatch(src, -1) {
		seen[m[1]]++
	}
	for i := 1; i <= 22; i++ {
		id := "E" + strconv.Itoa(i)
		if seen[id] != 1 {
			t.Errorf("erratum %s appears %d times in doc.go, want exactly 1", id, seen[id])
		}
		delete(seen, id)
	}
	for extra := range seen {
		t.Errorf("doc.go carries an unaccounted erratum %s", extra)
	}

	categories := []struct {
		heading string
		want    int
	}{
		{"TWENTY DOCUMENT DEFECTS:", 20},
		{"ONE ANTI-DEFECT:", 1},
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
		} else if next := strings.Index(src[start:], "# The ASSUMED register"); next > 0 {
			end = start + next
		}
		if got := len(erratumEntry.FindAllString(src[start:end], -1)); got != c.want {
			t.Errorf("category %q holds %d errata rows, want %d", c.heading, got, c.want)
		}
	}
}

// TestDoc_NamesTheCeilingConstant is a small obligation with a named
// consumer: the three profile stanzas transcribe MaxEXDigits' VALUE as a
// literal, because internal/extable may not import the package it generates
// into, so the stanzas' authors need the constant's name recorded where
// they will look for it.
func TestDoc_NamesTheCeilingConstant(t *testing.T) {
	src := docSource(t)
	if !strings.Contains(src, "MaxEXDigits") {
		t.Error("doc.go does not name MaxEXDigits — the profile stanzas transcribe its value and need the name of the authority")
	}
	if !strings.Contains(src, "DigitsCeiling") {
		t.Error("doc.go does not name the extable field the stanzas carry it in")
	}
	// And the VALUE, because a literal is what the stanzas carry. Pinned
	// against the constant so the sentence cannot outlive it: a change to
	// DefaultMaxFrame moves MaxEXDigits, and this fails until doc.go says
	// the new number.
	want := "ITS VALUE IS " + strconv.Itoa(MaxEXDigits) + ","
	if !strings.Contains(src, want) {
		t.Errorf("doc.go does not state MaxEXDigits' current value (%d) in the form %q — the three profile stanzas transcribe it as a literal and have nowhere else to read it", MaxEXDigits, want)
	}
}

// TestDoc_StatesTheOneVariableLengthFrame pins the negative fact the plan
// asks doc.go to carry: PrefixLenMatcher's variable-length branch has
// exactly one user, and a reader meeting that branch would otherwise take
// it for spare generality.
func TestDoc_StatesTheOneVariableLengthFrame(t *testing.T) {
	src := docSource(t)
	if !strings.Contains(src, "THE EX ANSWER IS THE ONLY VARIABLE-LENGTH FRAME THIS MILESTONE PARSES") {
		t.Error("doc.go no longer states that the EX answer is the only variable-length frame this milestone parses")
	}
}
