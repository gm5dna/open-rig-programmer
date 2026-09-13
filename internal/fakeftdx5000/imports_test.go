// SPDX-License-Identifier: GPL-3.0-or-later

package fakeftdx5000

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

// This file is internal/fakeft991a's imports_test.go, copied — standing
// infrastructure carried sibling to sibling, mechanism-for-mechanism, with
// only the package clause and the test-table prose updated for this
// package's own siblings and dialect.

// modulePrefix is this project's module path (go.mod: "module
// github.com/gm5dna/open-rig-programmer") — NOT the repository directory
// name ("ft710-programmer"), which does not match it. Getting this wrong
// would make isForbiddenImport pass vacuously against a real core/ import;
// TestIsForbiddenImport below pins the correct behaviour against exactly
// that mistake.
const modulePrefix = "github.com/gm5dna/open-rig-programmer/"

// isForbiddenImport reports whether path is a project-internal import —
// which fakeftdx5000 must never have, in this directory or any beneath it
// (see doc.go, THE HARD RULE): it may depend only on the standard library.
// fakepipeImport is the ONE project-internal import this package may have.
const fakepipeImport = modulePrefix + "internal/fakepipe"

func isForbiddenImport(path string) bool {
	return path != fakepipeImport && strings.HasPrefix(path, modulePrefix)
}

// TestIsForbiddenImport pins isForbiddenImport's behaviour directly,
// independent of the filesystem — including the exact mistake that would
// make the scan pass vacuously (matching against the repo directory name,
// "ft710-programmer", instead of the real module path).
func TestIsForbiddenImport(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"core/cat — the shared Yaesu dialect config this radio's driver configures", "github.com/gm5dna/open-rig-programmer/core/cat", true},
		{"core/codeplug", "github.com/gm5dna/open-rig-programmer/core/codeplug", true},
		{"core/spec", "github.com/gm5dna/open-rig-programmer/core/spec", true},
		{"core/driver/ftdx5000 — the driver under test against this fake", "github.com/gm5dna/open-rig-programmer/core/driver/ftdx5000", true},
		{"core/driver/internal/yaesu — the shared write ladder this radio's driver bypasses (no MT)", "github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu", true},
		{"internal/fakeft991a — the shape template this package's doc.go and imports_test.go come from", "github.com/gm5dna/open-rig-programmer/internal/fakeft991a", true},
		{"internal/fakedx101 — another Yaesu sibling fake", "github.com/gm5dna/open-rig-programmer/internal/fakedx101", true},
		{"internal/fakeradio — the FT-710's", "github.com/gm5dna/open-rig-programmer/internal/fakeradio", true},
		{"fakeftdx5000 itself", "github.com/gm5dna/open-rig-programmer/internal/fakeftdx5000", true},
		{"internal/fakepipe — protocol-free plumbing, the one permitted share", fakepipeImport, false},
		{"stdlib", "io", false},
		{"stdlib nested", "go/parser", false},
		{"third party", "github.com/wailsapp/wails/v2", false},
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

// forbiddenImport is one violation the scan found: the file that has it and
// the path it imports.
type forbiddenImport struct {
	file string
	path string
}

// scanResult is what one scan of a directory tree observed. The two counts
// exist for the vacuity checks: a scan that parsed no files, or files with
// no imports at all, would report "no violations" while proving nothing.
type scanResult struct {
	files      int
	imports    int
	violations []forbiddenImport
}

// scanForbiddenImports walks root and every directory beneath it, parsing
// each non-test .go file's import block, and reports every project-internal
// import found.
//
// Two kinds of file are skipped, and each skip is a decision:
//
//   - _test.go files. The rule is about what the PACKAGE depends on; its
//     tests may import whatever they need.
//   - anything under a directory named "testdata". The go tool ignores
//     those directories, so a .go file there is not compiled into anything
//     and cannot introduce a dependency.
func scanForbiddenImports(root string) (scanResult, error) {
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
		name := d.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
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

// TestNoCoreImports enforces THE HARD RULE (see doc.go): fakeftdx5000 must
// not import anything project-internal — not core/cat, not
// core/driver/ftdx5000, not core/codeplug, not core/spec, and not any
// sibling fake — so that a systematic bug in the production codec cannot
// pass end-to-end tests invisibly. It covers this directory AND EVERY
// DIRECTORY BENEATH IT.
func TestNoCoreImports(t *testing.T) {
	res, err := scanForbiddenImports(".")
	if err != nil {
		t.Fatalf("scanForbiddenImports(\".\"): %v", err)
	}
	for _, v := range res.violations {
		t.Errorf("%s imports %q — fakeftdx5000 MUST NOT import any project-internal package, in this directory or any beneath it (see doc.go, THE HARD RULE)", v.file, v.path)
	}

	// Vacuity guards. Without them a filter bug, a rename, or a walk that
	// silently visited nothing would leave this test passing with nothing
	// examined.
	if res.files == 0 {
		t.Fatal("scanned zero non-test .go files — the walk or the filter is broken, and this test would pass vacuously")
	}
	if res.imports == 0 {
		t.Fatal("scanned zero imports — every parsed file had an empty import block, which cannot be true of this package; this test would pass vacuously")
	}
}

// writeTree writes a set of relative path -> content files under a fresh
// temporary directory and returns its root.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", full, err)
		}
	}
	return root
}

// fenceTestTree is the tree both self-tests below run against: one clean
// file in the root, one VIOLATING file in a subdirectory (gen/, by name),
// one violating _test.go beside it, and one violating file under testdata.
// Only the subdirectory's non-test file may be reported.
func fenceTestTree(t *testing.T) string {
	t.Helper()
	return writeTree(t, map[string]string{
		"clean.go":           "package p\n\nimport \"io\"\n\nvar _ io.Reader\n",
		"gen/main.go":        "package main\n\nimport _ \"github.com/gm5dna/open-rig-programmer/core/driver/ftdx5000\"\n",
		"gen/main_test.go":   "package main\n\nimport _ \"github.com/gm5dna/open-rig-programmer/core/cat\"\n",
		"testdata/vendor.go": "package fixture\n\nimport _ \"github.com/gm5dna/open-rig-programmer/core/spec\"\n",
	})
}

// TestScanForbiddenImports_CatchesAForbiddenImportInASubdirectory is the
// fence's own red proof, run green: it proves the scan WOULD bite a
// violation placed in a subdirectory, and that it bites there and nowhere
// else — the _test.go file beside it and the testdata fixture are both
// skipped by design.
func TestScanForbiddenImports_CatchesAForbiddenImportInASubdirectory(t *testing.T) {
	root := fenceTestTree(t)

	res, err := scanForbiddenImports(root)
	if err != nil {
		t.Fatalf("scanForbiddenImports: %v", err)
	}
	if len(res.violations) != 1 {
		t.Fatalf("found %d violations (%+v), want exactly 1 — gen/main.go's forbidden import, and nothing else", len(res.violations), res.violations)
	}
	got := res.violations[0]
	if want := filepath.Join(root, "gen", "main.go"); got.file != want {
		t.Errorf("violation file = %q, want %q", got.file, want)
	}
	if want := "github.com/gm5dna/open-rig-programmer/core/driver/ftdx5000"; got.path != want {
		t.Errorf("violation import = %q, want %q", got.path, want)
	}
	if res.files != 2 {
		t.Errorf("parsed %d files, want 2 (clean.go and gen/main.go; the _test.go and the testdata fixture are skipped)", res.files)
	}
}

// TestScanForbiddenImports_IsRecursiveWhereParseDirIsNot pins the reason
// this file uses a walk rather than parser.ParseDir, by demonstrating the
// miss rather than asserting it in a comment: ParseDir over the same
// tree's root reports no violation at all, because it never descends into
// gen/.
func TestScanForbiddenImports_IsRecursiveWhereParseDirIsNot(t *testing.T) {
	root := fenceTestTree(t)

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, root, func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parser.ParseDir: %v", err)
	}

	missed := 0
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, imp := range file.Imports {
				p, uerr := strconv.Unquote(imp.Path.Value)
				if uerr != nil {
					t.Fatalf("unquoting %s: %v", imp.Path.Value, uerr)
				}
				if isForbiddenImport(p) {
					missed++
				}
			}
		}
	}
	if missed != 0 {
		t.Fatalf("parser.ParseDir found %d forbidden imports in the tree's root — it appears to recurse now, so this package's walking scan is no longer needed for the reason stated here", missed)
	}

	res, err := scanForbiddenImports(root)
	if err != nil {
		t.Fatalf("scanForbiddenImports: %v", err)
	}
	if len(res.violations) != 1 {
		t.Fatalf("the walking scan found %d violations, want 1 — the subdirectory violation ParseDir cannot see", len(res.violations))
	}
}
