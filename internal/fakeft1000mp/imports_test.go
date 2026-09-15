// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft1000mp

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Shape copied from internal/fakeft950's own imports_test.go (doc.go, THE
// HARD RULE) — the fleet-wide fence every fake plays, checking every
// directory beneath this one too, though fakeft1000mp has none today.
// The self-tests proving the walk is genuinely recursive live on that
// sibling; not duplicated here since nothing here has a subdirectory to
// miss (ponytail — a test for a defect this package cannot yet have).

const modulePrefix = "github.com/gm5dna/open-rig-programmer/"
const fakepipeImport = modulePrefix + "internal/fakepipe"

// isForbiddenImport reports whether path is a project-internal import
// other than fakepipe — the one share this package is permitted (doc.go).
func isForbiddenImport(path string) bool {
	return path != fakepipeImport && strings.HasPrefix(path, modulePrefix)
}

func TestIsForbiddenImport(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"core/bincat — this milestone's own codec", modulePrefix + "core/bincat", true},
		{"core/driver/ft1000mp — the driver under test against this fake", modulePrefix + "core/driver/ft1000mp", true},
		{"core/civ", modulePrefix + "core/civ", true},
		{"core/spec", modulePrefix + "core/spec", true},
		{"internal/fakeft950 — a sibling fake, deliberately not shared", modulePrefix + "internal/fakeft950", true},
		{"fakeft1000mp itself", modulePrefix + "internal/fakeft1000mp", true},
		{"internal/fakepipe — the one permitted share", fakepipeImport, false},
		{"stdlib", "io", false},
		{"stdlib nested", "go/parser", false},
		{"third party", "github.com/wailsapp/wails/v2", false},
		{"repo dir name is not the module path", "ft710-programmer/core/bincat", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isForbiddenImport(tt.path); got != tt.want {
				t.Errorf("isForbiddenImport(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

type forbiddenImport struct {
	file string
	path string
}

type scanResult struct {
	files      int
	imports    int
	violations []forbiddenImport
}

// scanForbiddenImports walks root and every directory beneath it, parsing
// each non-test .go file's import block.
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

// TestNoCoreImports enforces THE HARD RULE (doc.go): fakeft1000mp must
// not import anything project-internal but internal/fakepipe, in this
// directory or any beneath it — so that a systematic bug in core/bincat
// or a future core/driver/ft1000mp cannot pass an end-to-end cross-check
// invisibly.
func TestNoCoreImports(t *testing.T) {
	res, err := scanForbiddenImports(".")
	if err != nil {
		t.Fatalf("scanForbiddenImports(\".\"): %v", err)
	}
	for _, v := range res.violations {
		t.Errorf("%s imports %q — fakeft1000mp MUST NOT import any project-internal package but internal/fakepipe (doc.go, THE HARD RULE)", v.file, v.path)
	}
	// Vacuity guards: without them a filter bug or a walk that silently
	// visited nothing would pass with nothing examined.
	if res.files == 0 {
		t.Fatal("scanned zero non-test .go files — the walk or the filter is broken, and this test would pass vacuously")
	}
	if res.imports == 0 {
		t.Fatal("scanned zero imports — every parsed file had an empty import block, which cannot be true of this package; this test would pass vacuously")
	}
}
