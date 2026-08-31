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
		Model:     "ic7760",
		Dirs:      []string{dir},
		ListPath:  list,
		Authority: []string{filepath.Join(dir, "no-such-matrix.md")},
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
