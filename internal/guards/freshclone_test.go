// SPDX-License-Identifier: GPL-3.0-or-later

package guards

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Follow-up 10 (Tier 4b tier review), landed as the ci-clean-clone fix
// (v1.2.1, 30/08/2026): docs/superpowers/ is gitignored, so a fresh
// clone and CI never have it. Two tests read it UNCONDITIONALLY that
// day — core/civ/ic7100/crosscheck_test.go and
// core/civ/icr8600/crosscheck_test.go — and broke CI before each was
// given the same absent-guard: read (or stat) the file, and if it is
// simply not there, t.Logf (or t.Skip) and carry on rather than failing.
// See crosscheck_test.go:148-160 and :98-133 for the exact idiom.
//
// THIS FILE IS THE MECHANICAL VERSION of "review every future
// docs/superpowers read for the same guard": it scans every *_test.go in
// the module, finds every one that actually READS under the gitignored
// tree, and requires the same absent-guard idiom in each.
//
// THE SIGNAL IS A FUNCTION THAT BOTH NAMES "superpowers" AND CALLS A
// FILE-READING os FUNCTION, not the bare text "docs/superpowers"
// anywhere in a file. That text also appears in plain CITATIONS —
// core/civ/tier_test.go and three files in this very package cite
// docs/superpowers/*.md as evidence in prose — which never touch the
// filesystem there at all; flagging those would be a false positive
// against files with nothing to guard. fileReadsUnderSuperpowers'
// function-body scope is what tells the two apart: in both known
// readers, the same function that builds the path
// (filepath.Join(..., "docs", "superpowers", ...)) is the one that
// calls os.ReadFile or os.Stat on it.

// fileReadsUnderSuperpowers reports whether f contains a function whose
// body BOTH names the literal path segment "superpowers" (as it appears
// in filepath.Join("...", "docs", "superpowers", ...) — the two known
// readers never write "docs/superpowers" as one joined string) AND calls
// one of os.ReadFile, os.Open, os.Stat or os.Lstat — the shape of a real
// filesystem read under the gitignored tree, as distinct from a citation
// of it in a comment or an unrelated string literal.
func fileReadsUnderSuperpowers(f *ast.File) bool {
	touches := false
	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			return true
		}
		names, reads := false, false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.BasicLit:
				if v.Kind == token.STRING {
					if s, err := strconv.Unquote(v.Value); err == nil && s == "superpowers" {
						names = true
					}
				}
			case *ast.CallExpr:
				if sel, ok := v.Fun.(*ast.SelectorExpr); ok {
					if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "os" {
						switch sel.Sel.Name {
						case "ReadFile", "Open", "Stat", "Lstat":
							reads = true
						}
					}
				}
			}
			return true
		})
		if names && reads {
			touches = true
		}
		return true
	})
	return touches
}

// TestFreshCloneGuardCoversEveryDocsSuperpowersRead is this file's one
// test: every *_test.go that reads under docs/superpowers must also
// carry the absent-guard idiom.
func TestFreshCloneGuardCoversEveryDocsSuperpowersRead(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()

	var touched []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && (strings.HasPrefix(name, ".") || name == "node_modules" || name == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return perr
		}
		if fileReadsUnderSuperpowers(f) {
			rel, rerr := filepath.Rel(root, path)
			if rerr != nil {
				return rerr
			}
			touched = append(touched, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	// Non-vacuity, and a regression pin on the scan itself: today's
	// population is exactly these two files (grep '"superpowers"'
	// --include=*.go . finds nowhere else in the module). A scan that
	// found neither, or that silently stopped matching one, would leave
	// every check below passing on nothing.
	want := map[string]bool{
		"core/civ/ic7100/crosscheck_test.go":  true,
		"core/civ/icr8600/crosscheck_test.go": true,
	}
	got := make(map[string]bool, len(touched))
	for _, f := range touched {
		got[f] = true
	}
	for f := range want {
		if !got[f] {
			t.Errorf("scan did not find the known docs/superpowers reader %s — fileReadsUnderSuperpowers is broken", f)
		}
	}
	if len(touched) == 0 {
		t.Fatal("scanned zero files reading under docs/superpowers — the walk or the filter is broken, and this test would pass vacuously")
	}

	// A THIRD reader is not a failure here — it is new evidence this
	// list has not caught up with yet, and t.Logf says so rather than
	// silently ignoring it: the guard loop below still runs against it.
	for f := range got {
		if !want[f] {
			t.Logf("a new docs/superpowers reader was found: %s — extend `want` above once its own absent-guard is confirmed", f)
		}
	}

	for _, rel := range touched {
		body, rerr := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if rerr != nil {
			t.Fatalf("read %s: %v", rel, rerr)
		}
		text := string(body)
		hasSkipOrLog := strings.Contains(text, "t.Logf(") || strings.Contains(text, "t.Skip(")
		hasExistenceCheck := strings.Contains(text, "os.IsNotExist(") || strings.Contains(text, "err == nil")
		if !hasSkipOrLog || !hasExistenceCheck {
			t.Errorf("%s reads under docs/superpowers (gitignored; absent on a fresh clone or in CI) but carries no absent-guard idiom — an os.Stat/existence check (os.IsNotExist or err == nil) paired with a t.Logf or t.Skip, the pattern core/civ/ic7100/crosscheck_test.go:148-160 and core/civ/icr8600/crosscheck_test.go:98-133 use — so a fresh clone fails this file the way the v1.2.1 CI run did on 30/08/2026", rel)
		}
	}
}
