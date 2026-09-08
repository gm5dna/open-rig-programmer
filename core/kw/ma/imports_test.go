// SPDX-License-Identifier: GPL-3.0-or-later

package ma

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// THIS PACKAGE'S THREE IMPORT FENCES, IN ITS OWN SUITE.
//
// core/kw/imports_test.go's TestNoSiblingCodecImports already walks this
// directory — it walks core/kw and everything beneath it, test files included
// — and the first fence below therefore holds twice. That is deliberate and
// not a copy of a fact: a lane task runs the packages it touched, so a
// violation introduced here must fail in THIS package's suite rather than
// only in a sibling's, and the other two fences have no other home at all.
//
// The scan is one DIRECTORY rather than a tree, because this package has no
// subdirectories and is not going to grow one: both layouts, both codecs and
// both generated inventories live here, which is the whole of decision 10's
// single-package shape.

// maModulePrefix is this project's module path (go.mod: "module
// github.com/gm5dna/open-rig-programmer") — NOT the repository directory
// name, which does not match it. Getting this wrong would make every check
// below pass vacuously, which is what TestImportScan_SeesWhatIsReallyThere
// exists to catch.
const maModulePrefix = "github.com/gm5dna/open-rig-programmer/"

// The three import paths no file in this package may name.
//
// core/cat and core/civ are the cross-FAMILY fence core/kw states in terms: a
// family that borrows one helper borrows the next, and the borrowed one
// carries a Yaesu or an Icom fact into a package whose whole content is that
// those facts are different here.
//
// core/kw/kwtest is the third, and it is the structural half of spec
// §"CANNOT reuse" 5b. kwtest.Run takes a kw.LAYOUT, its identity check
// switches on Book into an FV leg or a TY leg, and its non-vacuity check
// names the eight MR/MW-shaped builders. No ma.Layout is a kw.Layout, so none
// can be passed to it by a compiling programme — but a file that imported it
// would be a file trying, and its default: arm for an unrecognised Book must
// stay unreached.
var maForbiddenImports = []string{
	"core/cat",
	"core/civ",
	"core/kw/kwtest",
}

// dirImports parses every .go file directly in dir and returns each file's
// import paths, keyed by base name.
func dirImports(t *testing.T, dir string) map[string][]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	fset := token.NewFileSet()
	out := map[string][]string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parsing %s: %v", path, err)
		}
		paths := []string{}
		for _, imp := range f.Imports {
			p, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatalf("%s: unquoting %s: %v", path, imp.Path.Value, err)
			}
			paths = append(paths, p)
		}
		out[e.Name()] = paths
	}
	return out
}

// namesPackage reports whether importPath is rel, or a package beneath it —
// so core/cat/ft891 is caught alongside core/cat.
func namesPackage(importPath, rel string) bool {
	if !strings.HasPrefix(importPath, maModulePrefix) {
		return false
	}
	got := strings.TrimPrefix(importPath, maModulePrefix)
	return got == rel || strings.HasPrefix(got, rel+"/")
}

// TestNamesPackage pins the predicate directly, independent of the
// filesystem — including the two mistakes that would make every walk below
// pass vacuously.
func TestNamesPackage(t *testing.T) {
	tests := []struct {
		path, rel string
		want      bool
	}{
		{maModulePrefix + "core/cat", "core/cat", true},
		{maModulePrefix + "core/cat/ft891", "core/cat", true},
		{maModulePrefix + "core/civ/ic905", "core/civ", true},
		{maModulePrefix + "core/kw/kwtest", "core/kw/kwtest", true},
		{maModulePrefix + "core/kw", "core/kw/kwtest", false},
		{maModulePrefix + "core/kw", "core/kw", true},
		{maModulePrefix + "core/catalogue", "core/cat", false},
		{"errors", "core/cat", false},
		{"ft710-programmer/core/cat", "core/cat", false},
		{"github.com/someone/open-rig-programmer-fork/core/cat", "core/cat", false},
	}
	for _, tt := range tests {
		if got := namesPackage(tt.path, tt.rel); got != tt.want {
			t.Errorf("namesPackage(%q, %q) = %v, want %v", tt.path, tt.rel, got, tt.want)
		}
	}
}

// violations returns "file imports path" for every file importing one of the
// forbidden packages, sorted, so both fences below share one detector and the
// detector itself can be tested against a synthetic tree.
func violations(files map[string][]string, forbidden []string) []string {
	var out []string
	for name, paths := range files {
		for _, p := range paths {
			for _, f := range forbidden {
				if namesPackage(p, f) {
					out = append(out, name+" imports "+p)
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

// TestViolations_BitesOnEveryFencedPackage is the detector's own red proof,
// run green. Both fences below report nothing on the real tree, which is
// indistinguishable from a predicate that never says yes; this is what tells
// the two apart.
func TestViolations_BitesOnEveryFencedPackage(t *testing.T) {
	files := map[string][]string{
		"clean.go":     {maModulePrefix + "core/kw", maModulePrefix + "core/transport", "fmt"},
		"cat.go":       {maModulePrefix + "core/cat/ft891"},
		"civ.go":       {maModulePrefix + "core/civ"},
		"kwtest.go":    {maModulePrefix + "core/kw/kwtest"},
		"reverse.go":   {maModulePrefix + "core/kw/ma"},
		"lookalike.go": {maModulePrefix + "core/catalogue", "ft710-programmer/core/cat"},
	}
	got := violations(files, maForbiddenImports)
	want := []string{
		"cat.go imports " + maModulePrefix + "core/cat/ft891",
		"civ.go imports " + maModulePrefix + "core/civ",
		"kwtest.go imports " + maModulePrefix + "core/kw/kwtest",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("violations(forbidden) = %v, want %v", got, want)
	}
	got = violations(files, []string{"core/kw/ma"})
	want = []string{"reverse.go imports " + maModulePrefix + "core/kw/ma"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("violations(core/kw/ma) = %v, want %v", got, want)
	}
}

// TestNoForbiddenImports is the fence over this package's own files, TEST
// FILES INCLUDED. The rule is about what may be BORROWED, and a test that
// asserted this codec's answer equalled core/cat's constant would be exactly
// the borrowing it exists to forbid — it would make one family's number the
// authority for another's without any production file importing anything.
func TestNoForbiddenImports(t *testing.T) {
	files := dirImports(t, ".")
	if len(files) == 0 {
		t.Fatal("parsed zero .go files in this directory — the scan is broken and every check here would pass vacuously")
	}
	for _, v := range violations(files, maForbiddenImports) {
		t.Errorf("%s — no file in core/kw/ma may import core/cat, core/civ or core/kw/kwtest; this dialect COPIES the envelope frames and imports neither sibling family, and no ma.Layout is a kw.Layout for kwtest to take", v)
	}
}

// TestCoreKWDoesNotImportThisPackage pins the sibling relation's DIRECTION.
//
// THE COMPILER IS THE PRIMARY ENFORCEMENT AND THIS IS THE DIAGNOSTIC. A
// core/kw file importing this package is an import cycle, so the build fails
// before any test runs — but it fails with "import cycle not allowed", which
// tells a reader nothing about WHY the relation is one-way. It is one-way
// because core/kw must stay radio-agnostic: the whole reason this package
// exists is that the MA grid is not the 50-byte record core/kw describes, and
// a reader who reaches for the reverse import has already lost that. The
// detector's own red proof is above, on a synthetic tree, because a real one
// would not compile.
func TestCoreKWDoesNotImportThisPackage(t *testing.T) {
	files := dirImports(t, "..")
	if len(files) == 0 {
		t.Fatal("parsed zero .go files in core/kw — the scan is broken and this check would pass vacuously")
	}
	for _, v := range violations(files, []string{"core/kw/ma"}) {
		t.Errorf("core/kw/%s — the sibling relation is one-way: core/kw/ma imports core/kw and never the reverse", v)
	}
}

// TestImportScan_SeesWhatIsReallyThere is the non-vacuity control every fence
// above depends on: it asserts the scan finds an import this package really
// has, so that "no violations" means the walk looked rather than that it
// found nothing to look at.
func TestImportScan_SeesWhatIsReallyThere(t *testing.T) {
	seen := false
	for _, paths := range dirImports(t, ".") {
		for _, p := range paths {
			if namesPackage(p, "core/kw") {
				seen = true
			}
		}
	}
	if !seen {
		t.Error("the scan found no import of core/kw anywhere in this package — this package is core/kw's sibling codec and imports it in several files, so the scan or its predicate is broken")
	}
}
