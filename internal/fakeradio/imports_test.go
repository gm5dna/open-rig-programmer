// SPDX-License-Identifier: GPL-3.0-or-later

package fakeradio

import (
	"go/parser"
	"go/token"
	"io/fs"
	"strconv"
	"strings"
	"testing"
)

// modulePrefix is this project's module path (go.mod: "module
// github.com/gm5dna/open-rig-programmer") — NOT the repository directory
// name ("ft710-programmer"), which does not match it. Getting this wrong
// would make isForbiddenImport pass vacuously against a real core/
// import; TestIsForbiddenImport below pins the correct behaviour against
// exactly that mistake.
const modulePrefix = "github.com/gm5dna/open-rig-programmer/"

// isForbiddenImport reports whether path is a project-internal import —
// which fakeradio must never have (see doc.go, THE HARD RULE): it may
// depend only on the standard library.
func isForbiddenImport(path string) bool {
	return strings.HasPrefix(path, modulePrefix)
}

// TestIsForbiddenImport pins isForbiddenImport's behaviour directly,
// independent of the filesystem — including the exact mistake that would
// make TestNoCoreImports pass vacuously (matching against the repo
// directory name, "ft710-programmer", instead of the real module path).
func TestIsForbiddenImport(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"core/cat", "github.com/gm5dna/open-rig-programmer/core/cat", true},
		{"core/codeplug", "github.com/gm5dna/open-rig-programmer/core/codeplug", true},
		{"core/spec", "github.com/gm5dna/open-rig-programmer/core/spec", true},
		{"core/csvio", "github.com/gm5dna/open-rig-programmer/core/csvio", true},
		{"fakeradio itself", "github.com/gm5dna/open-rig-programmer/internal/fakeradio", true},
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

// TestNoCoreImports enforces THE HARD RULE (see doc.go): fakeradio must
// not import core/cat, core/codeplug, or core/spec (or anything else
// project-internal) — it is a pure, independent byte-level simulator, so
// that a systematic bug in the production codec cannot pass end-to-end
// tests invisibly. It parses every non-test .go file in this package
// with go/parser and asserts none of their import paths is
// project-internal (isForbiddenImport).
func TestNoCoreImports(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parser.ParseDir: %v", err)
	}
	if len(pkgs) == 0 {
		t.Fatal("parser.ParseDir found no packages in .")
	}

	checked := 0
	for _, pkg := range pkgs {
		for filename, file := range pkg.Files {
			for _, imp := range file.Imports {
				path, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					t.Fatalf("%s: unquoting import path %s: %v", filename, imp.Path.Value, err)
				}
				checked++
				if isForbiddenImport(path) {
					t.Errorf("%s imports %q — fakeradio MUST NOT import any project-internal package (see doc.go, THE HARD RULE)", filename, path)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("TestNoCoreImports found zero imports to check — the file-filter predicate likely excluded everything; this test would pass vacuously")
	}
}
