// SPDX-License-Identifier: GPL-3.0-or-later

package fakepipe

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

// modulePrefix is this project's module path (go.mod: "module
// github.com/gm5dna/open-rig-programmer") — NOT the repository directory name
// ("ft710-programmer"), which does not match it.
const modulePrefix = "github.com/gm5dna/open-rig-programmer/"

// isForbiddenImport reports whether path is a project-internal import.
//
// fakepipe is the one package the fakes are allowed to share, and it earns
// that by depending on NOTHING in this module — not core/, not a dialect, not
// a sibling fake. There is no carve-out here: the share stops at this package.
func isForbiddenImport(path string) bool { return strings.HasPrefix(path, modulePrefix) }

func TestIsForbiddenImport(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"core/civ", "github.com/gm5dna/open-rig-programmer/core/civ", true},
		{"internal/fakeradio — a fake, and fakepipe must not reach back into one", "github.com/gm5dna/open-rig-programmer/internal/fakeradio", true},
		{"fakepipe itself", "github.com/gm5dna/open-rig-programmer/internal/fakepipe", true},
		{"stdlib", "net", false},
		{"stdlib nested", "go/parser", false},
		{"repo dir name is not the module path", "ft710-programmer/core/civ", false},
		{"substring collision, different module", "github.com/someone/open-rig-programmer-fork/core/civ", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isForbiddenImport(tt.path); got != tt.want {
				t.Errorf("isForbiddenImport(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

type forbiddenImport struct{ file, path string }

// scanForbiddenImports walks root and every directory beneath it, parsing each
// non-test .go file's import block. _test.go files and testdata/ are skipped:
// the rule is about what the PACKAGE depends on.
func scanForbiddenImports(root string) (violations []forbiddenImport, files, imports int, err error) {
	fset := token.NewFileSet()
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if perr != nil {
			return perr
		}
		files++
		for _, imp := range f.Imports {
			p, uerr := strconv.Unquote(imp.Path.Value)
			if uerr != nil {
				return uerr
			}
			imports++
			if isForbiddenImport(p) {
				violations = append(violations, forbiddenImport{path, p})
			}
		}
		return nil
	})
	return violations, files, imports, err
}

// TestNoCoreImports is the reason this package may be shared at all: it
// depends on the standard library and nothing else, so no protocol knowledge
// can leak into it from either side of a cross-check.
func TestNoCoreImports(t *testing.T) {
	violations, files, imports, err := scanForbiddenImports(".")
	if err != nil {
		t.Fatalf("scanForbiddenImports(\".\"): %v", err)
	}
	for _, v := range violations {
		t.Errorf("%s imports %q — fakepipe MUST NOT import any project-internal package", v.file, v.path)
	}
	if files == 0 {
		t.Fatal("scanned zero non-test .go files — the walk or the filter is broken, and this test would pass vacuously")
	}
	if imports == 0 {
		t.Fatal("scanned zero imports — this test would pass vacuously")
	}
}

// TestScanForbiddenImports_CatchesAForbiddenImportInASubdirectory proves the
// walk is recursive and bites where it should, and nowhere else.
func TestScanForbiddenImports_CatchesAForbiddenImportInASubdirectory(t *testing.T) {
	root := t.TempDir()
	for rel, content := range map[string]string{
		"clean.go":            "package p\n\nimport \"net\"\n\nvar _ net.Conn\n",
		"nested/bad.go":       "package nested\n\nimport _ \"github.com/gm5dna/open-rig-programmer/core/civ\"\n",
		"nested/bad_test.go":  "package nested\n\nimport _ \"github.com/gm5dna/open-rig-programmer/core/cat\"\n",
		"testdata/fixture.go": "package fixture\n\nimport _ \"github.com/gm5dna/open-rig-programmer/core/spec\"\n",
	} {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	violations, files, _, err := scanForbiddenImports(root)
	if err != nil {
		t.Fatalf("scanForbiddenImports: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("found %d violations (%+v), want exactly 1", len(violations), violations)
	}
	if want := filepath.Join(root, "nested", "bad.go"); violations[0].file != want {
		t.Errorf("violation file = %q, want %q", violations[0].file, want)
	}
	if files != 2 {
		t.Errorf("parsed %d files, want 2 (the _test.go and the testdata fixture are skipped)", files)
	}
}
