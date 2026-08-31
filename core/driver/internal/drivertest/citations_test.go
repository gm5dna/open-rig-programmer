// SPDX-License-Identifier: GPL-3.0-or-later

package drivertest

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestNormaliseRejoinsCitationsBrokenByCommentWrapping pins the three jobs
// normalise does, because each one is a way a real citation hid from a scan.
func TestNormaliseRejoinsCitationsBrokenByCommentWrapping(t *testing.T) {
	for _, tc := range []struct {
		name, src, want string
	}{
		{
			name: "a citation wrapped across two comment lines",
			src:  "// The record is\n// PDF p.263 (folio 18-14).\n",
			want: "The record is PDF p.263 (folio 18-14).",
		},
		{
			// core/driver/ic7851/doc.go:494 wraps a register id at its own
			// hyphen. Folding the break to a space invents "ic7851-serial",
			// which no authority can supply.
			name: "a register id wrapped at its own hyphen",
			src:  "// entry ic7851-serial-\n// framing lives here\n",
			want: "entry ic7851-serial-framing lives here",
		},
		{
			// A hyphen that merely ends a word before a line break must NOT
			// swallow the space: only the wrap case joins.
			name: "an en dash folded to a hyphen",
			src:  "// printed folios 1–26 plus cover\n",
			want: "printed folios 1-26 plus cover",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalise(tc.src).text; got != tc.want {
				t.Errorf("normalise = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestNormaliseKeepsTheSourceLineOfEveryToken pins that a failure can be
// walked straight to the line that wrote the citation, including when the
// citation began on the earlier of two wrapped lines.
func TestNormaliseKeepsTheSourceLineOfEveryToken(t *testing.T) {
	n := normalise("package x\n\n// nothing here\n// then PDF\n// p.263 arrives\n")
	i := strings.Index(n.text, "PDF")
	if i < 0 {
		t.Fatalf("citation lost from %q", n.text)
	}
	if got := n.lineAt(i); got != 4 {
		t.Errorf("citation reported at line %d, want 4 (where it starts)", got)
	}
}

// TestCitationPinEnforcesTheListWhenTheAuthorityIsAbsent pins the CI case.
// docs/superpowers/ is gitignored, so a fresh clone and CI have no matrix (the
// v1.2.1 CI run, 30/08/2026). The checked-in list must still be enforced there
// — an absent authority may not turn the pin into a no-op.
func TestCitationPinEnforcesTheListWhenTheAuthorityIsAbsent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte("package x\n\n// see PDF p.7\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	list := filepath.Join(dir, "citations.txt")
	if err := os.WriteFile(list, []byte("# a list\nPDF p.9\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	pin := CitationPin{
		Model:    "ic7760",
		Dirs:     []string{dir},
		ListPath: list,
		Matrix:   filepath.Join(dir, "no-such-matrix.md"),
	}
	spy := &recordingTB{TB: t}
	pin.Assert(spy)
	if len(spy.errs) != 2 {
		t.Fatalf("got %d failures, want 2 (the unlisted citation and the uncited entry): %v", len(spy.errs), spy.errs)
	}
	if !strings.Contains(spy.errs[0], `cites "PDF p.7"`) {
		t.Errorf("first failure does not name the unlisted citation: %q", spy.errs[0])
	}
	if !strings.Contains(spy.errs[1], `allows "PDF p.9"`) {
		t.Errorf("second failure does not name the uncited list entry: %q", spy.errs[1])
	}
}

// TestCitationPinRefusesAPageTheMatrixDoesNotSupply is the permanent red proof
// for the THIRD arm — "allows X, but the authority does not supply it". The
// first round proved this arm with a one-off injection into a real list, which
// is evidence that does not survive the commit.
func TestCitationPinRefusesAPageTheMatrixDoesNotSupply(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "x.go", "package x\n\n// see PDF p.7\n")
	write(t, dir, "matrix.md", "# a matrix\n\nThe record is at PDF p.9.\n")
	list := filepath.Join(dir, "citations.txt")
	write(t, dir, "citations.txt", "# a list\nPDF p.7\n")

	spy := &recordingTB{TB: t}
	CitationPin{
		Model:         "ic7760",
		Dirs:          []string{dir},
		ListPath:      list,
		Matrix:        filepath.Join(dir, "matrix.md"),
		MatrixSupport: supportFor(t, dir),
		Plans:         planFor(t, dir),
	}.Assert(spy)

	if len(spy.errs) != 1 {
		t.Fatalf("got %d failures, want 1 (the listed page the matrix never prints): %v", len(spy.errs), spy.errs)
	}
	if !strings.Contains(spy.errs[0], `allows "PDF p.7"`) || !strings.Contains(spy.errs[0], "does not supply it") {
		t.Errorf("failure does not name the unsupplied page: %q", spy.errs[0])
	}
}

// TestCitationPinRefusesASpelledOutPageParaphrase is the red proof for the
// recall the closing review asked for, and it is the attack that review named:
// core/civ/ic7100/crosscheck_test.go cites "page 375" three times in words, and
// before this round re-typing one of them as "page 999" changed no token, so
// the positive list stayed green over a page the IC-7100 manual does not have.
//
// The fixture is that attack in miniature. The package writes the citation in
// words and in mixed case; the matrix prints a different page; the pin must
// name the token it could not source.
func TestCitationPinRefusesASpelledOutPageParaphrase(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "x.go", "package x\n\n// PDF arbitration ruling, Page 999 (folio 19).\n")
	write(t, dir, "matrix.md", "# a matrix\n\nThe ruling is at PDF p.9, printed folio 19.\n")
	list := filepath.Join(dir, "citations.txt")
	write(t, dir, "citations.txt", "# a list\nPDF p.999\nfolio 19\n")

	spy := &recordingTB{TB: t}
	CitationPin{
		Model:         "ic7760",
		Dirs:          []string{dir},
		ListPath:      list,
		Matrix:        filepath.Join(dir, "matrix.md"),
		MatrixSupport: supportFor(t, dir),
		Plans:         planFor(t, dir),
	}.Assert(spy)

	if len(spy.errs) != 1 {
		t.Fatalf("got %d failures, want 1 (the spelled-out page the matrix never prints): %v", len(spy.errs), spy.errs)
	}
	if !strings.Contains(spy.errs[0], `allows "PDF p.999"`) {
		t.Errorf("the spelled-out page was not read as a citation at all: %q", spy.errs[0])
	}
}

// TestASelfAttributedCitationStillFacesTheAuthority pins the one shape a
// paraphrase could use to walk past the positive list entirely. isForeign
// exempts a citation the prose attributes to ANOTHER radio's document, because
// this model's authority is the wrong place to look for it; it used to exempt
// every "IC-nnnn"-qualified token, including one naming the package's OWN
// model. "IC-7100 matrix §5.1" inside core/driver/ic7100 says no more than
// "matrix §5.1" does, and no document was ever asked for it.
//
// The fixture puts both qualifiers side by side over one matrix that heads
// neither section: the foreign one is exempt and passes, the self-attributed
// one is refused exactly as the bare form would be.
func TestASelfAttributedCitationStillFacesTheAuthority(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "x.go", "package x\n\n// See IC-7100 matrix §5.1, and IC-7300 matrix §5.1 for the tone doctrine.\n")
	write(t, dir, "matrix.md", "# a matrix\n\n## §1 Scope\n\nThere is no §5.1 here.\n")
	list := filepath.Join(dir, "citations.txt")
	write(t, dir, "citations.txt", "# a list\nIC-7100 matrix §5.1\nIC-7300 matrix §5.1\n")

	spy := &recordingTB{TB: t}
	CitationPin{
		Model:         "ic7100",
		Dirs:          []string{dir},
		ListPath:      list,
		Matrix:        filepath.Join(dir, "matrix.md"),
		MatrixSupport: supportFor(t, dir),
		Plans:         planFor(t, dir),
	}.Assert(spy)

	if len(spy.errs) != 1 {
		t.Fatalf("got %d failures, want exactly 1 — the self-attributed section refused, the foreign one exempt: %v", len(spy.errs), spy.errs)
	}
	if !strings.Contains(spy.errs[0], `allows "IC-7100 matrix §5.1"`) {
		t.Errorf("the model's own name still buys a citation past the authority: %q", spy.errs[0])
	}
}

// TestASelfAttributedCitationIsAnsweredLikeTheBareForm is the other half of the
// pin above: dropping the own-name exemption must not turn an HONEST
// self-attributed citation into a failure. The qualifier is stripped and the
// ordinary section reading answers it, so a headed §3.2 is supplied whether the
// prose wrote "matrix §3.2" or "IC-7100 matrix §3.2".
func TestASelfAttributedCitationIsAnsweredLikeTheBareForm(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "x.go", "package x\n\n// See IC-7100 matrix §3.2 for the record layout.\n")
	write(t, dir, "matrix.md", "# a matrix\n\n### 3.2 The record layout\n\nBytes 0-11.\n")
	list := filepath.Join(dir, "citations.txt")
	write(t, dir, "citations.txt", "# a list\nIC-7100 matrix §3.2\n")

	spy := &recordingTB{TB: t}
	CitationPin{
		Model:         "ic7100",
		Dirs:          []string{dir},
		ListPath:      list,
		Matrix:        filepath.Join(dir, "matrix.md"),
		MatrixSupport: supportFor(t, dir),
		Plans:         planFor(t, dir),
	}.Assert(spy)

	if len(spy.errs) != 0 {
		t.Errorf("a self-attributed citation the matrix heads was refused: %v", spy.errs)
	}
}

// TestReadAuthorityTreatsAnEmptyPathListAsAbsent pins the difference between
// "no documents were asked for" and "the documents are all here and say
// nothing". CitationPin.Plans comes from a filepath.Glob that returns no paths
// and no error when the plan is renamed, and an empty-but-present reading would
// let the register-id supply check run against the matrix trio alone — failing
// an honest id with a message blaming the citation rather than the missing
// document. core/civ/icr8600/doc.go:77 attributes an id to the plan, so the
// misdiagnosis would be a real one.
func TestReadAuthorityTreatsAnEmptyPathListAsAbsent(t *testing.T) {
	for _, paths := range [][]string{nil, {}} {
		got, err := readAuthority(paths)
		if err != nil {
			t.Fatalf("readAuthority(%v): %v", paths, err)
		}
		if got.present {
			t.Errorf("readAuthority(%v).present = true; a missed glob must look exactly like a missing file", paths)
		}
	}
}

// TestAMissedPlanGlobSkipsRatherThanBlamesTheCitation is the same rule seen from
// the pin: with the matrices present and the plan glob empty, the supply half
// must report itself skipped instead of refusing the register id.
func TestAMissedPlanGlobSkipsRatherThanBlamesTheCitation(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "x.go", "package x\n\n// Register entry ic7760-plan-only-entry.\n")
	write(t, dir, "matrix.md", "# a matrix that does not name the entry\n")
	list := filepath.Join(dir, "citations.txt")
	write(t, dir, "citations.txt", "# a list\nic7760-plan-only-entry\n")

	spy := &recordingTB{TB: t}
	CitationPin{
		Model:         "ic7760",
		Dirs:          []string{dir},
		ListPath:      list,
		Matrix:        filepath.Join(dir, "matrix.md"),
		MatrixSupport: supportFor(t, dir),
		Plans:         nil, // what filepath.Glob returns for a renamed plan
	}.Assert(spy)

	if len(spy.errs) != 0 {
		t.Errorf("a missed plan glob blamed the citation instead of skipping: %v", spy.errs)
	}
}

// TestThePlanMaySupplyARegisterIDButNeverAPage pins the orchestrator's ruling
// STRUCTURALLY. The implementation plans carry page and folio citations of
// their own, so an authority that concatenated plan and matrix would let a
// plan answer a page claim about the radio; the first round did exactly that
// and was right only by luck, because no package happened to cite such a page.
//
// The fixture is that luck removed: the plan alone prints "PDF p.7" and the
// register id, and the matrix prints neither. The page must be REFUSED and the
// id ADMITTED.
func TestThePlanMaySupplyARegisterIDButNeverAPage(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "x.go", "package x\n\n// PDF p.7, register entry ic7760-plan-only-entry.\n")
	write(t, dir, "matrix.md", "# a matrix that prints neither\n\nNothing to see.\n")
	write(t, dir, "plan.md", "# a plan\n\nRead PDF p.7. Register entry `ic7760-plan-only-entry`.\n")
	list := filepath.Join(dir, "citations.txt")
	write(t, dir, "citations.txt", "# a list\nPDF p.7\nic7760-plan-only-entry\n")

	spy := &recordingTB{TB: t}
	CitationPin{
		Model:         "ic7760",
		Dirs:          []string{dir},
		ListPath:      list,
		Matrix:        filepath.Join(dir, "matrix.md"),
		MatrixSupport: supportFor(t, dir),
		Plans:         []string{filepath.Join(dir, "plan.md")},
	}.Assert(spy)

	if len(spy.errs) != 1 {
		t.Fatalf("got %d failures, want exactly 1 — the page refused, the register id admitted: %v", len(spy.errs), spy.errs)
	}
	if !strings.Contains(spy.errs[0], `allows "PDF p.7"`) {
		t.Errorf("the plan answered a PAGE citation, which is the ruling this test exists to hold: %q", spy.errs[0])
	}
	if strings.Contains(spy.errs[0], "ic7760-plan-only-entry") {
		t.Errorf("the plan was refused a REGISTER ID, which it is allowed to supply: %q", spy.errs[0])
	}
}

// TestSuppliedBySection pins BOTH arms and BOTH guards. Round 1 shipped this
// function with no test at all, and the asymmetry in case "a lettered heading
// does not answer its bare parent" went unnoticed because of it: the two arms
// carried different trailing guards, so one half of a guard the comment
// claimed was not in fact enforced.
//
// The cases below are the re-review's, on synthetic authority texts so each
// reads as one claim about one rule.
func TestSuppliedBySection(t *testing.T) {
	const realShape = "### 3.12 Probe identity\n\nThe token is undocumented.\n§3.12a states the same entry from the identity side.\n"
	for _, tc := range []struct {
		name, matrix, token string
		want                bool
	}{
		{
			name:   "a heading supplies its own section",
			matrix: "### 3.15 Model-specific\n",
			token:  "matrix §3.15",
			want:   true,
		},
		{
			// The IC-7851 §3.12a case, and the only reason the second arm
			// exists: the parent is headed and the matrix labels the
			// sub-part in its own body.
			name:   "a lettered sub-part is supplied by a headed parent plus a cross-reference",
			matrix: realShape,
			token:  "matrix §3.12a",
			want:   true,
		},
		{
			name:   "a headed parent alone does not supply an unlabelled sub-part",
			matrix: "### 3.12 Probe identity\n",
			token:  "matrix §3.12a",
			want:   false,
		},
		{
			name:   "a cross-reference alone does not supply an unheaded sub-part",
			matrix: "§3.12a states the same entry from the identity side.\n",
			token:  "matrix §3.12a",
			want:   false,
		},
		{
			// A matrix that mentions a section only to DENY it must not
			// thereby supply it. The first attempt returned true here.
			name:   "a denial does not supply a bare numbered section",
			matrix: "there is NO §3.20 in this matrix\n",
			token:  "matrix §3.20",
			want:   false,
		},
		{
			// Nor may a matrix supply a section by citing ANOTHER
			// document's numbering.
			name:   "a reference to another document's section does not supply it",
			matrix: "the tier spec's own §4.2 says otherwise\n",
			token:  "matrix §4.2",
			want:   false,
		},
		{
			// The report's and review's numbering reached this function in
			// round 1 because it ran over the concatenated trio. It now
			// receives the matrix file alone, so their text is simply not
			// here to answer with — this case stands for that removal.
			name:   "the report's own numbering is not in the matrix to answer with",
			matrix: "## §5 Completeness claim\n\n## §6 Errata discipline\n",
			token:  "matrix §5.1",
			want:   false,
		},
		{
			name:   "a deeper heading does not answer its parent",
			matrix: "#### 3.15.1 The name field\n",
			token:  "matrix §3.15",
			want:   false,
		},
		{
			name:   "a deeper cross-reference does not answer its parent",
			matrix: "see §3.15.1 for detail\n",
			token:  "matrix §3.15",
			want:   false,
		},
		{
			name:   "a lettered cross-reference does not answer its bare parent",
			matrix: "§3.12a states the same entry from the identity side.\n",
			token:  "matrix §3.12",
			want:   false,
		},
		{
			// THE ASYMMETRY ROUND 1 LEFT: the heading arm's guard permitted a
			// letter, so a "### 3.12a" heading answered §3.12. Both arms now
			// share sectionGuard.
			name:   "a lettered heading does not answer its bare parent",
			matrix: "### 3.12a Probe identity sub-part\n",
			token:  "matrix §3.12",
			want:   false,
		},
		{
			name:   "a section-marked heading is read the same way",
			matrix: "## §1b The fields and banks the tier adds\n",
			token:  "matrix §1b",
			want:   true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := suppliedBySection(tc.matrix, tc.token); got != tc.want {
				t.Errorf("suppliedBySection(%q, %q) = %v, want %v", tc.matrix, tc.token, got, tc.want)
			}
		})
	}
}

// TestCitationGrammarReadsTheSpelledOutAndCasedForms is the recall half of the
// closing review's finding, at the level of the grammar itself. Each POSITIVE
// case is a spelling the packages actually use that the first two rounds did
// not read; each NEGATIVE case is a shape that must stay out, because a
// grammar that invents citations is as useless as one that misses them.
func TestCitationGrammarReadsTheSpelledOutAndCasedForms(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		want      []string
	}{
		{
			// core/civ/ic7100/crosscheck_test.go:126,165,174.
			name: "a spelled-out page reaches the same token as the abbreviation",
			src:  "// PDF arbitration ruling, page 375 (folio 20-16)\n",
			want: []string{"PDF p.375", "folio 20-16"},
		},
		{
			name: "a spelled-out page range reaches the same token as the abbreviation",
			src:  "// its CI-V material is pages 250-265\n",
			want: []string{"PDF pp.250-265"},
		},
		{
			// core/driver/ic7851/e2e_test.go:23 shouts its citation.
			name: "an upper-case abbreviation is the same citation",
			src:  "// THIS FILE IS WHERE TWO READINGS OF PDF P.263 MEET.\n",
			want: []string{"PDF p.263"},
		},
		{
			name: "a lower-case PDF is the same citation",
			src:  "// see pdf p.263 and PDF PAGE 264\n",
			want: []string{"PDF p.263", "PDF p.264"},
		},
		{
			// core/driver/icr8600/write_test.go:259 leaves "matrix" behind on
			// the first clause: "(matrix section 2 row 9, section 3.16.4)".
			name: "a dotted spelled-out section collapses onto the bare section token",
			src:  "// (matrix section 2 row 9, section 3.16.4)\n",
			want: []string{"matrix §2", "matrix §3.16.4"},
		},
		{
			name: "a plural matrix-qualified section is read",
			src:  "// Matrix sections 1b.3 and 3.15.4: one sparse bank\n",
			want: []string{"matrix §1b.3"},
		},
		{
			// core/driver/ic7851/doc.go:29. Unqualified and undotted, this is
			// the MANUAL's part number; no IC-7851 matrix heads a §18, so
			// reading it as one would manufacture a citation.
			name: "an undotted spelled-out section is the manual's own part number",
			src:  "// Its CI-V material is Section 18, \"CONTROL COMMAND\", which is PDF pages 250-265\n",
			want: []string{"PDF pp.250-265"},
		},
		{
			name: "a file-qualified spelled-out section is an internal cross-reference",
			src:  "// see doc.go section 6.1 for the rest\n",
			want: nil,
		},
		{
			// The count is not a page and the word is not a lead-in.
			name: "a page count is not a page citation",
			src:  "// 28 PDF pages (cover, folios 1-26 and back cover)\n",
			want: []string{"folio 1-26"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := extractTokens(normalise(tc.src).text)
			for _, w := range tc.want {
				if !got[w] {
					t.Errorf("token %q not extracted from %q; got %v", w, tc.src, sortedKeys(got))
				}
				delete(got, w)
			}
			if len(got) != 0 {
				t.Errorf("extra tokens %v extracted from %q", sortedKeys(got), tc.src)
			}
		})
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// supportFor writes the matrix's report and review for a synthetic pin. They
// carry no citations of their own: a fixture that wanted the trio to answer
// something would say so in matrix.md. They exist because readAuthority reports
// an EMPTY path list as absent, so a pin that named no support would skip the
// supply half rather than run it.
func supportFor(t *testing.T, dir string) []string {
	t.Helper()
	write(t, dir, "report.md", "# the matrix report\n")
	write(t, dir, "review.md", "# the matrix review\n")
	return []string{filepath.Join(dir, "report.md"), filepath.Join(dir, "review.md")}
}

// planFor writes an implementation plan that supplies nothing, for the same
// reason supportFor does.
func planFor(t *testing.T, dir string) []string {
	t.Helper()
	write(t, dir, "empty-plan.md", "# the implementation plan\n")
	return []string{filepath.Join(dir, "empty-plan.md")}
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// recordingTB captures Errorf instead of failing, so a test can assert on what
// the pin reports. Only the methods Assert uses are overridden.
type recordingTB struct {
	testing.TB
	errs []string
}

func (r *recordingTB) Errorf(format string, args ...any) {
	r.errs = append(r.errs, strings.TrimSpace(fmt.Sprintf(format, args...)))
}

func (r *recordingTB) Logf(string, ...any) {}

func (r *recordingTB) Helper() {}
