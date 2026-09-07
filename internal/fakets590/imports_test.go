// SPDX-License-Identifier: GPL-3.0-or-later

package fakets590

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

// This file is internal/fakeft891's imports_test.go, COPIED — copied because
// the rule it enforces forbids importing anything to share (see doc.go, THE
// HARD RULE), and because that package's version is already the RECURSIVE
// one: it walks the tree rather than calling parser.ParseDir("."), which
// reads one directory and stops.
//
// THE FENCE IS RECURSIVE FROM BIRTH HERE, ahead of the subdirectory it has to
// cover. internal/fakets590/gen/ — the stdlib-only generator for this
// family's two transcription-B copies — arrives at a later task of this
// milestone's plan, and it is the piece most likely to reach for
// internal/extable, the A-side machinery whose Digits parsing was a known
// defect locus. That is exactly the import this package must not have: one
// parser on both sides of the EX cross-check would reproduce a shared parsing
// bug into both inventories invisibly.
// TestScanForbiddenImports_CatchesAForbiddenImportInASubdirectory below is
// the fence's own red proof, run green, and it stands in that position until
// the directory arrives.

// modulePrefix is this project's module path (go.mod: "module
// github.com/gm5dna/open-rig-programmer") — NOT the repository directory name
// ("ft710-programmer"), which does not match it. Getting this wrong would make
// isForbiddenImport pass vacuously against a real core/ import;
// TestIsForbiddenImport below pins the correct behaviour against exactly that
// mistake.
const modulePrefix = "github.com/gm5dna/open-rig-programmer/"

// isForbiddenImport reports whether path is a project-internal import — which
// fakets590 must never have, in this directory or any beneath it (see doc.go,
// THE HARD RULE): it may depend only on the standard library.
// fakepipeImport is the ONE project-internal import this package may have.
//
// internal/fakepipe is PROTOCOL-FREE plumbing — the net.Pipe pair, the
// goroutine bookkeeping, the interruptible latency wait, the raw write — and
// nothing else: not a framing byte, not a field layout, not a reply. It is the
// single share Stuart permitted on 06/09/2026, and it is safe precisely
// because a bug in it cannot make a wrong codec look right; it can only stop
// bytes moving, which this package's own tests notice at once. Everything
// above the wire stays here, written independently (see doc.go).
const fakepipeImport = modulePrefix + "internal/fakepipe"

func isForbiddenImport(path string) bool {
	return path != fakepipeImport && strings.HasPrefix(path, modulePrefix)
}

// TestIsForbiddenImport pins isForbiddenImport's behaviour directly,
// independent of the filesystem — including the exact mistake that would make
// the scan pass vacuously (matching against the repo directory name,
// "ft710-programmer", instead of the real module path).
func TestIsForbiddenImport(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"core/kw — the codec this fake must be able to disagree with", "github.com/gm5dna/open-rig-programmer/core/kw", true},
		{"core/kw/ts590 — the two layout values", "github.com/gm5dna/open-rig-programmer/core/kw/ts590", true},
		{"core/codeplug", "github.com/gm5dna/open-rig-programmer/core/codeplug", true},
		{"core/spec", "github.com/gm5dna/open-rig-programmer/core/spec", true},
		{"core/driver/ts590 — the driver under test against this fake", "github.com/gm5dna/open-rig-programmer/core/driver/ts590", true},
		{"internal/fakeft891 — the sibling fake this package is shaped after, deliberately not shared", "github.com/gm5dna/open-rig-programmer/internal/fakeft891", true},
		{"internal/fakets480 — the other Kenwood fake, whose legends differ", "github.com/gm5dna/open-rig-programmer/internal/fakets480", true},
		{"internal/extable — gen/ must not reach for it", "github.com/gm5dna/open-rig-programmer/internal/extable", true},
		{"fakets590 itself", "github.com/gm5dna/open-rig-programmer/internal/fakets590", true},
		{"internal/fakepipe — protocol-free plumbing, the one permitted share", fakepipeImport, false},
		{"stdlib", "io", false},
		{"stdlib nested", "go/parser", false},
		{"third party", "github.com/wailsapp/wails/v2", false},
		{"repo dir name is not the module path", "ft710-programmer/core/kw", false},
		{"substring collision, different module", "github.com/someone/open-rig-programmer-fork/core/kw", false},
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
// exist for the vacuity checks: a scan that parsed no files, or files with no
// imports at all, would report "no violations" while proving nothing.
type scanResult struct {
	files      int
	imports    int
	violations []forbiddenImport
}

// scanForbiddenImports walks root and every directory beneath it, parsing each
// non-test .go file's import block, and reports every project-internal import
// found.
//
// Factored out of the test that runs it over "." so the WALK ITSELF is
// testable: the tests below run it over a temporary tree whose only violation
// is inside a subdirectory, which is the property a ParseDir version cannot
// have.
//
// Two kinds of file are skipped, and each skip is a decision:
//
//   - _test.go files. The rule is about what the PACKAGE depends on; its tests
//     may import whatever they need (this very file imports nothing project-
//     internal, but core/driver/ts590's tests will import both this package
//     and half of core/, and that is correct).
//   - anything under a directory named "testdata". The go tool ignores those
//     directories, so a .go file there is not compiled into anything and
//     cannot introduce a dependency; flagging one would be a false positive on
//     a fixture. Nothing in this package puts Go source there today.
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

// TestNoCoreImports enforces THE HARD RULE (see doc.go): fakets590 must not
// import anything project-internal — not core/kw, not core/kw/ts590, not
// core/codeplug, not core/spec, and not any sibling fake — so that a
// systematic bug in the production codec cannot pass end-to-end tests
// invisibly. It covers this directory AND EVERY DIRECTORY BENEATH IT.
func TestNoCoreImports(t *testing.T) {
	res, err := scanForbiddenImports(".")
	if err != nil {
		t.Fatalf("scanForbiddenImports(\".\"): %v", err)
	}
	for _, v := range res.violations {
		t.Errorf("%s imports %q — fakets590 MUST NOT import any project-internal package, in this directory or any beneath it (see doc.go, THE HARD RULE)", v.file, v.path)
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

// fenceTestTree is the tree both self-tests below run against: one clean file
// in the root, one VIOLATING file in a subdirectory (gen/, by name — the shape
// this package's generator will have), one violating _test.go beside it, and
// one violating file under testdata. Only the subdirectory's non-test file may
// be reported.
func fenceTestTree(t *testing.T) string {
	t.Helper()
	return writeTree(t, map[string]string{
		"clean.go":           "package p\n\nimport \"io\"\n\nvar _ io.Reader\n",
		"gen/main.go":        "package main\n\nimport _ \"github.com/gm5dna/open-rig-programmer/internal/extable\"\n",
		"gen/main_test.go":   "package main\n\nimport _ \"github.com/gm5dna/open-rig-programmer/core/kw\"\n",
		"testdata/vendor.go": "package fixture\n\nimport _ \"github.com/gm5dna/open-rig-programmer/core/spec\"\n",
	})
}

// TestScanForbiddenImports_CatchesAForbiddenImportInASubdirectory is the
// fence's own red proof, run green: it proves the scan WOULD bite a violation
// placed where this package's EX generator is going to live, and that it bites
// there and nowhere else — the _test.go file beside it and the testdata
// fixture are both skipped by design.
//
// It is the test that makes the fence meaningful BEFORE gen/ exists, which is
// the whole reason the recursive form lands with the package core rather than
// with the generator.
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
	if want := "github.com/gm5dna/open-rig-programmer/internal/extable"; got.path != want {
		t.Errorf("violation import = %q, want %q", got.path, want)
	}
	// The clean root file and the subdirectory file are both parsed: two
	// files, two imports. The skipped pair contributes nothing.
	if res.files != 2 {
		t.Errorf("parsed %d files, want 2 (clean.go and gen/main.go; the _test.go and the testdata fixture are skipped)", res.files)
	}
}

// TestScanForbiddenImports_IsRecursiveWhereParseDirIsNot pins the REASON this
// file uses a walk rather than internal/fakeradio's parser.ParseDir, by
// demonstrating the miss rather than asserting it in a comment: ParseDir over
// the same tree's root reports no violation at all, because it never descends
// into gen/.
//
// If this test ever fails because ParseDir started recursing, the divergence is
// no longer needed — which is worth knowing, and is why the assertion is
// stated in that direction.
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
		t.Fatalf("parser.ParseDir found %d forbidden imports in the tree's root — it appears to recurse now, so this package's walking scan is no longer the reason fakeft891's copy had to be extended", missed)
	}

	res, err := scanForbiddenImports(root)
	if err != nil {
		t.Fatalf("scanForbiddenImports: %v", err)
	}
	if len(res.violations) != 1 {
		t.Fatalf("the walking scan found %d violations, want 1 — the subdirectory violation ParseDir cannot see", len(res.violations))
	}
}

// TestNoCoreImports_ReachesTheGenerator makes TestNoCoreImports's coverage of
// gen/ VISIBLE rather than merely intended. That test reports violations, so
// it passes whether the walk reached the generator or never descended into it
// at all — a silent regression in the walk would look exactly like a clean
// tree. This one asserts the scan actually parsed the file, by name.
//
// It is the standing half of the fence's red proof: the tree fixture above
// proves the scan WOULD bite a violation in gen/, and this proves it is
// looking at the real gen/ while doing so.
func TestNoCoreImports_ReachesTheGenerator(t *testing.T) {
	fset := token.NewFileSet()
	const genMain = "gen/main.go"
	if _, err := parser.ParseFile(fset, genMain, nil, parser.ImportsOnly); err != nil {
		t.Fatalf("parsing %s: %v — the generator this package's EX inventory comes from is not where the fence expects it", genMain, err)
	}

	var seen int
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if !d.IsDir() && filepath.ToSlash(path) == genMain {
			seen++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir: %v", err)
	}
	if seen != 1 {
		t.Errorf("the walk visited %s %d times, want 1 — TestNoCoreImports scans the same tree, so a zero here means its coverage of gen/ is vacuous", genMain, seen)
	}
}
