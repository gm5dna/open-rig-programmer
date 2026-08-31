// SPDX-License-Identifier: GPL-3.0-or-later

package drivertest

import (
	"fmt"
	"os"
	"path/filepath"
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
		Matrix:   []string{filepath.Join(dir, "no-such-matrix.md")},
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
		Model:    "ic7760",
		Dirs:     []string{dir},
		ListPath: list,
		Matrix:   []string{filepath.Join(dir, "matrix.md")},
	}.Assert(spy)

	if len(spy.errs) != 1 {
		t.Fatalf("got %d failures, want 1 (the listed page the matrix never prints): %v", len(spy.errs), spy.errs)
	}
	if !strings.Contains(spy.errs[0], `allows "PDF p.7"`) || !strings.Contains(spy.errs[0], "does not supply it") {
		t.Errorf("failure does not name the unsupplied page: %q", spy.errs[0])
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
		Model:    "ic7760",
		Dirs:     []string{dir},
		ListPath: list,
		Matrix:   []string{filepath.Join(dir, "matrix.md")},
		Plans:    []string{filepath.Join(dir, "plan.md")},
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
