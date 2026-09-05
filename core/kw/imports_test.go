// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// THE CROSS-FAMILY IMPORT FENCE. No file under core/kw — this directory or
// any beneath it, TESTS INCLUDED — may import core/cat or core/civ.
//
// It is the guard the plan names at T5, and it is the structural half of
// spec §"CANNOT reuse". core/kw copies core/cat's ';' splitter, its
// prefix matcher and its outbound-gate discipline; it does not import them.
// The reason is the one core/civ's own fence gives: a family that borrows
// one helper borrows the next, and the borrowed one carries a Yaesu fact
// (idAnswerLen = 7, memoryFrameLen = 28, a hex mode nibble, seven dialect
// axes named for Yaesu commands) into a package whose whole content is that
// those facts are different here. core/civ is fenced for the same reason in
// the other direction: three wire protocols, three codecs, no relatives.
//
// TESTS ARE IN SCOPE, and that is the departure from the fakes' version of
// this fence. There, _test.go files are skipped because the rule is about
// what the package DEPENDS on. Here the rule is about what may be BORROWED,
// and a test that asserts core/kw's answer equals core/cat's constant is
// exactly the borrowing this fence exists to forbid — it would make one
// family's number the authority for the other's without any production file
// ever importing anything.
//
// core/transport IS permitted and is imported: it is the neutral seam, it
// knows nothing of core/kw, and the direction is the cycle-free one
// core/civ's adapter already takes.

// modulePrefix is this project's module path (go.mod: "module
// github.com/gm5dna/open-rig-programmer") — NOT the repository directory
// name ("ft710-programmer"), which does not match it. Getting this wrong
// would make isForbiddenImport pass vacuously against a real core/cat
// import; TestIsForbiddenImport pins that exact mistake.
const modulePrefix = "github.com/gm5dna/open-rig-programmer/"

// forbiddenRoots are the two sibling codec trees, matched as whole import
// paths or as parents of one — so core/cat/ft891 and core/civ/ic905 are
// caught alongside their roots.
var forbiddenRoots = []string{"core/cat", "core/civ"}

// isForbiddenImport reports whether path is one of the two sibling codecs,
// or a package beneath either.
func isForbiddenImport(path string) bool {
	if !strings.HasPrefix(path, modulePrefix) {
		return false
	}
	rel := strings.TrimPrefix(path, modulePrefix)
	for _, root := range forbiddenRoots {
		if rel == root || strings.HasPrefix(rel, root+"/") {
			return true
		}
	}
	return false
}

// TestIsForbiddenImport pins the predicate directly, independent of the
// filesystem — including the two mistakes that would make the walk pass
// vacuously (matching the repo directory name instead of the module path,
// and a prefix match that catches a different module of similar name).
func TestIsForbiddenImport(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"core/cat", modulePrefix + "core/cat", true},
		{"core/cat/ft891", modulePrefix + "core/cat/ft891", true},
		{"core/cat/dialecttest", modulePrefix + "core/cat/dialecttest", true},
		{"core/civ", modulePrefix + "core/civ", true},
		{"core/civ/ic905", modulePrefix + "core/civ/ic905", true},
		{"core/transport — the neutral seam, permitted", modulePrefix + "core/transport", false},
		{"core/spec — the neutral field vocabulary, permitted", modulePrefix + "core/spec", false},
		{"internal/extable — the generator's registry, permitted", modulePrefix + "internal/extable", false},
		{"core/kw itself", modulePrefix + "core/kw", false},
		{"a package whose name merely starts with cat", modulePrefix + "core/catalogue", false},
		{"stdlib", "errors", false},
		{"repo dir name is not the module path", "ft710-programmer/core/cat", false},
		{"substring collision, different module", "github.com/someone/open-rig-programmer-fork/core/cat", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isForbiddenImport(tt.path); got != tt.want {
				t.Errorf("isForbiddenImport(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// forbiddenImport is one violation the scan found.
type forbiddenImport struct {
	file string
	path string
}

// scanResult is what one scan observed. The two counts exist for the
// vacuity checks: a scan that parsed no files, or files with no imports at
// all, would report "no violations" while proving nothing.
type scanResult struct {
	files      int
	imports    int
	violations []forbiddenImport
}

// scanCrossFamilyImports walks root and every directory beneath it, parsing
// each .go file's import block — TEST FILES INCLUDED, see this file's
// header — and reports every import of core/cat or core/civ.
//
// It is factored out of the test that runs it over "." so the WALK itself is
// testable: the self-test below runs it over a temporary tree whose only
// violation is inside a subdirectory, which is the property core/kw's own
// tree cannot demonstrate while it is clean.
//
// Directories named "testdata" are skipped: the go tool ignores them, so a
// .go file there is not compiled into anything and cannot introduce a
// dependency.
func scanCrossFamilyImports(root string) (scanResult, error) {
	var res scanResult
	fset := token.NewFileSet()

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		file, perr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if perr != nil {
			return perr
		}
		res.files++
		for _, imp := range file.Imports {
			p, uerr := strconv.Unquote(imp.Path.Value)
			if uerr != nil {
				return uerr
			}
			res.imports++
			if isForbiddenImport(p) {
				res.violations = append(res.violations, forbiddenImport{file: path, path: p})
			}
		}
		return nil
	})
	return res, err
}

// TestNoSiblingCodecImports is the fence itself, over core/kw and every
// directory beneath it — core/kw/ts590 and core/kw/ts480 included.
func TestNoSiblingCodecImports(t *testing.T) {
	res, err := scanCrossFamilyImports(".")
	if err != nil {
		t.Fatalf("scanCrossFamilyImports(\".\"): %v", err)
	}
	for _, v := range res.violations {
		t.Errorf("%s imports %q — no file under core/kw may import core/cat or core/civ, in this directory or any beneath it; this dialect COPIES the ';' splitter, the prefix matcher and the gate's discipline and imports none of them", v.file, v.path)
	}
	if res.files == 0 {
		t.Fatal("scanned zero .go files — the walk or the filter is broken, and this test would pass vacuously")
	}
	if res.imports == 0 {
		t.Fatal("scanned zero imports — every parsed file had an empty import block, which cannot be true of this package; this test would pass vacuously")
	}
}

// TestScanCrossFamilyImports_BitesInASubdirectoryAndInATestFile is the
// fence's own red proof, run green: it proves the scan WOULD bite a
// violation placed in a subdirectory (core/kw/ts590 is one) and in a
// _test.go file (which this fence, unlike the fakes', covers), and that it
// leaves a testdata fixture alone.
func TestScanCrossFamilyImports_BitesInASubdirectoryAndInATestFile(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"clean.go":             "package p\n\nimport _ \"" + modulePrefix + "core/transport\"\n",
		"ts590/layout.go":      "package ts590\n\nimport _ \"" + modulePrefix + "core/cat\"\n",
		"ts590/layout_test.go": "package ts590\n\nimport _ \"" + modulePrefix + "core/civ\"\n",
		"testdata/fixture.go":  "package fixture\n\nimport _ \"" + modulePrefix + "core/cat\"\n",
	}
	for rel, content := range files {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	res, err := scanCrossFamilyImports(root)
	if err != nil {
		t.Fatalf("scanCrossFamilyImports: %v", err)
	}
	got := map[string]string{}
	for _, v := range res.violations {
		rel, rerr := filepath.Rel(root, v.file)
		if rerr != nil {
			t.Fatalf("Rel: %v", rerr)
		}
		got[filepath.ToSlash(rel)] = v.path
	}
	want := map[string]string{
		"ts590/layout.go":      modulePrefix + "core/cat",
		"ts590/layout_test.go": modulePrefix + "core/civ",
	}
	if len(got) != len(want) {
		t.Fatalf("scan reported %v, want exactly %v", got, want)
	}
	for file, path := range want {
		if got[file] != path {
			t.Errorf("scan reported %q for %s, want %q", got[file], file, path)
		}
	}
}
