// SPDX-License-Identifier: GPL-3.0-or-later

package guards

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
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
// tree, and requires an absent-guard idiom in each.
//
// FIX ROUND 1 (F7) WIDENED AND RE-SCOPED BOTH HALVES:
//
//   - Detection (fileReadsUnderSuperpowers) used to require the path
//     segment "superpowers" as its OWN string literal — the two-argument
//     filepath.Join("...", "docs", "superpowers", ...) shape both known
//     readers happen to use — which missed a reader spelling the same
//     path as one joined literal, "docs/superpowers/x.md", or built via
//     string concatenation. It now matches the substring "superpowers"
//     inside ANY string literal. That widening costs nothing against the
//     four files that merely CITE docs/superpowers/*.md as evidence
//     (core/civ/tier_test.go and three files in this package): those
//     citations live in `//` comments, which are never ast.BasicLits, or
//     — tier_test.go's case — inside a package-level var with no os call
//     anywhere in scope, so the function-body co-occurrence still
//     excludes them.
//   - The idiom check used to be a FILE-WIDE substring match on
//     "os.IsNotExist("/"err == nil" and "t.Logf("/"t.Skip(", which
//     rejected a correctly-guarded reader spelled any other way
//     (errors.Is(err, fs.ErrNotExist), t.Skipf, t.Log) and accepted an
//     unrelated "err == nil" anywhere else in a large file as if it were
//     the guard. hasAbsentGuardIdiom below is AST-based instead: the
//     existence check must live in the SAME FUNCTION as an actual
//     os-package read call (any of ReadFile, Open, Stat, Lstat, ReadDir,
//     OpenFile), and accepts os.IsNotExist(err), errors.Is(err,
//     ...ErrNotExist) (fs.ErrNotExist and the older os.ErrNotExist
//     alike), or a plain "err == nil"/"err != nil" comparison — err ==
//     nil stays accepted (icr8600's own matrixAuthorityPresent helper
//     uses exactly that spelling, with no os.IsNotExist call anywhere in
//     its file) but can no longer pass vacuously on an occurrence
//     unrelated to the read, because it must share the read's own
//     function body. The log/skip half (t.Skip/t.Skipf/t.Log/t.Logf)
//     stays file-scoped rather than function-scoped: icr8600's read
//     functions (pinMatrixA, readMatrixA) call the existence check
//     helper and only THEN t.Logf/t.Skip in their OWN body, so the two
//     halves of that file's guard are never in one function.

// namesSuperpowersPath reports whether s is (or contains) the path this
// guard cares about: either the bare path segment "superpowers", exactly
// as filepath.Join("...", "docs", "superpowers", ...) — the two known
// readers' shape — spells it as its OWN argument, or the joined form
// "docs/superpowers" appearing anywhere inside a longer literal (F7: a
// reader is at least as likely to write
// filepath.Join(root, "docs/superpowers", "x.md") or
// ".../docs/superpowers/x.md" as one string).
//
// NOT A BARE "superpowers" SUBSTRING ANYWHERE, deliberately: fix round 1
// tried that first and it false-positived on
// internal/guards/retirednames_test.go's skipDirs[".superpowers"] — a
// WalkDir EXCLUSION entry that keeps that test from ever descending into
// the directory, the opposite of a read. Requiring the "docs/" prefix
// alongside the bare-segment form keeps the F7(a) widening (a joined
// literal is now caught) without that false positive.
func namesSuperpowersPath(s string) bool {
	return s == "superpowers" || strings.Contains(s, "docs/superpowers")
}

// superpowersReadCalls is every os-package call this guard treats as a
// real filesystem read/stat — the shape a docs/superpowers reader takes,
// as distinct from a citation of the path in prose.
var superpowersReadCalls = map[string]bool{
	"ReadFile": true, "Open": true, "Stat": true, "Lstat": true,
	"ReadDir": true, "OpenFile": true,
}

// isOSCall reports whether call is os.<name> for name in
// superpowersReadCalls.
func isOSCall(call *ast.CallExpr) (name string, ok bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "os" {
		return "", false
	}
	return sel.Sel.Name, superpowersReadCalls[sel.Sel.Name]
}

// fileReadsUnderSuperpowers reports whether f contains a function whose
// body BOTH mentions the path segment "superpowers" — as its own string
// literal (filepath.Join("...", "docs", "superpowers", ...), the two
// known readers' shape) OR as a substring of a longer one
// (".../docs/superpowers/x.md", a joined-literal shape neither uses
// today but the next reader plausibly would) — AND calls one of
// superpowersReadCalls: the shape of a real filesystem read under the
// gitignored tree, as distinct from a citation of it in a comment or an
// unrelated string literal.
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
					if s, err := strconv.Unquote(v.Value); err == nil && namesSuperpowersPath(s) {
						names = true
					}
				}
			case *ast.CallExpr:
				if _, ok := isOSCall(v); ok {
					reads = true
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

// isNotExistCheck reports whether n is one of the accepted existence
// checks: os.IsNotExist(x), errors.Is(x, fs.ErrNotExist) or
// errors.Is(x, os.ErrNotExist) (either spelling of the sentinel), or a
// plain "err == nil" comparison — icr8600's own matrixAuthorityPresent
// spelling ("return err == nil"), treating absence as fine rather than
// as a failure.
//
// EQL ONLY, NOT NEQ: "err != nil { t.Fatalf(...) }" is ORDINARY error
// handling present in nearly every test that does any I/O at all, guard
// or not, and accepting it here would let any such check satisfy this
// idiom vacuously — exactly what F7(c) flagged.
// TestFreshCloneGuard_DoesNotAcceptAnUnrelatedErrEqualsNil's own
// "if err != nil { t.Fatalf(...) }" after the unguarded read is what
// this exclusion is for.
func isNotExistCheck(n ast.Node) bool {
	call, ok := n.(*ast.CallExpr)
	if ok {
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if pkg, ok := sel.X.(*ast.Ident); ok {
				if pkg.Name == "os" && sel.Sel.Name == "IsNotExist" {
					return true
				}
				if pkg.Name == "errors" && sel.Sel.Name == "Is" && len(call.Args) == 2 {
					if argSel, ok := call.Args[1].(*ast.SelectorExpr); ok && argSel.Sel.Name == "ErrNotExist" {
						return true
					}
				}
			}
		}
		return false
	}
	bin, ok := n.(*ast.BinaryExpr)
	if !ok || bin.Op != token.EQL {
		return false
	}
	isNilIdent := func(e ast.Expr) bool {
		id, ok := e.(*ast.Ident)
		return ok && id.Name == "nil"
	}
	return isNilIdent(bin.X) || isNilIdent(bin.Y)
}

// isSkipOrLogCall reports whether call is <recv>.Skip/Skipf/Log/Logf —
// the reporting half of the idiom, whatever the *testing.T variable is
// named.
func isSkipOrLogCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	switch sel.Sel.Name {
	case "Skip", "Skipf", "Log", "Logf":
		return true
	}
	return false
}

// hasAbsentGuardIdiom reports whether f carries a fresh-clone absent
// guard.
//
// THE EXISTENCE CHECK MUST SHARE ITS FUNCTION WITH A SUPERPOWERS READ,
// not merely with ANY os read: the same names-and-reads test
// fileReadsUnderSuperpowers runs per function is run again here, with an
// additional requirement that an accepted existence check
// (isNotExistCheck) sits in that SAME function body. Scoping to "any
// function that happens to read a file and compare err == nil" would
// accept a check that guards something else entirely — a helper that
// stats go.mod, say — while an unrelated function reads under
// docs/superpowers with no guard at all;
// TestFreshCloneGuard_DoesNotAcceptAnUnrelatedErrEqualsNil pins exactly
// that shape failing. icr8600's own guard still passes this scoping: its
// matrixAuthorityPresent helper independently names "superpowers", calls
// os.Stat, AND compares err == nil, all three in its own one-line body.
//
// THE REPORT HALF STAYS FILE-SCOPED, deliberately not tied to the same
// function: icr8600's pinMatrixA and readMatrixA each read under
// docs/superpowers and call matrixAuthorityPresent for the existence
// answer, then t.Logf or t.Skip in THEIR OWN body based on it — the
// check and the report are two functions apart in those two, so pinning
// the report to the checking function would fail a file the tier review
// confirmed correct.
func hasAbsentGuardIdiom(f *ast.File) bool {
	existenceOK := false
	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			return true
		}
		names, reads, checks := false, false, false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.BasicLit:
				if v.Kind == token.STRING {
					if s, err := strconv.Unquote(v.Value); err == nil && namesSuperpowersPath(s) {
						names = true
					}
				}
			case *ast.CallExpr:
				if _, ok := isOSCall(v); ok {
					reads = true
				}
			}
			if isNotExistCheck(n) {
				checks = true
			}
			return true
		})
		if names && reads && checks {
			existenceOK = true
		}
		return true
	})
	if !existenceOK {
		return false
	}
	skipOrLog := false
	ast.Inspect(f, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && isSkipOrLogCall(call) {
			skipOrLog = true
		}
		return true
	})
	return skipOrLog
}

// TestFreshCloneGuardCoversEveryDocsSuperpowersRead is this file's main
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
		if !fileReadsUnderSuperpowers(f) {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		touched = append(touched, rel)
		if !hasAbsentGuardIdiom(f) {
			t.Errorf("%s reads under docs/superpowers (gitignored; absent on a fresh clone or in CI) but carries no absent-guard idiom — an existence check (os.IsNotExist, errors.Is(err, fs.ErrNotExist)/os.ErrNotExist, or err == nil) in the SAME function as the read, paired with a Skip/Skipf/Log/Logf call, the pattern core/civ/ic7100/crosscheck_test.go:148-160 and core/civ/icr8600/crosscheck_test.go:98-133 use — so a fresh clone fails this file the way the v1.2.1 CI run did on 30/08/2026", rel)
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
	// every check above passing on nothing.
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
	// silently ignoring it: the guard loop above still ran against it.
	for f := range got {
		if !want[f] {
			t.Logf("a new docs/superpowers reader was found: %s — extend `want` above once its own absent-guard is confirmed", f)
		}
	}
}

// parseSnippet parses a synthetic Go source string for the three
// self-tests below, which exercise fileReadsUnderSuperpowers and
// hasAbsentGuardIdiom directly against fixtures no real file needs to
// hold.
func parseSnippet(t *testing.T, src string) *ast.File {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "snippet_test.go", src, 0)
	if err != nil {
		t.Fatalf("parse snippet: %v\n%s", err, src)
	}
	return f
}

// TestFreshCloneGuard_FlagsAnUnguardedJoinBasedReader is F7's first red
// proof: a reader that joins "docs/superpowers" as a filepath.Join
// argument list (rather than as one string literal, which the old
// exact-match detection also caught) and has NO existence check or
// Skip/Log call at all must be flagged as a reader AND fail the idiom
// check.
func TestFreshCloneGuard_FlagsAnUnguardedJoinBasedReader(t *testing.T) {
	f := parseSnippet(t, `package p

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUnguardedRead(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "docs", "superpowers", "matrix.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	_ = body
}
`)
	if !fileReadsUnderSuperpowers(f) {
		t.Fatal("fileReadsUnderSuperpowers = false, want true — this snippet reads under docs/superpowers")
	}
	if hasAbsentGuardIdiom(f) {
		t.Fatal("hasAbsentGuardIdiom = true, want false — this snippet has no existence check and no Skip/Log call at all")
	}
}

// TestFreshCloneGuard_AcceptsErrorsIsFsErrNotExistWithSkipf is F7's
// second red proof, run green: a reader guarded with the MODERN spelling
// — errors.Is(err, fs.ErrNotExist) paired with t.Skipf, neither of which
// the old file-wide "os.IsNotExist("/"err == nil" and "t.Logf("/
// "t.Skip(" substring match would have accepted — must pass.
func TestFreshCloneGuard_AcceptsErrorsIsFsErrNotExistWithSkipf(t *testing.T) {
	f := parseSnippet(t, `package p

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestGuardedRead(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "docs", "superpowers", "matrix.md"))
	if errors.Is(err, fs.ErrNotExist) {
		t.Skipf("matrix authority absent: %v", err)
	}
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	_ = body
}
`)
	if !fileReadsUnderSuperpowers(f) {
		t.Fatal("fileReadsUnderSuperpowers = false, want true — this snippet reads under docs/superpowers")
	}
	if !hasAbsentGuardIdiom(f) {
		t.Fatal("hasAbsentGuardIdiom = false, want true — errors.Is(err, fs.ErrNotExist) + t.Skipf is a legitimate absent-guard idiom")
	}
}

// TestFreshCloneGuard_DoesNotAcceptAnUnrelatedErrEqualsNil is F7(c)'s own
// pin: an "err == nil" that has NOTHING to do with the superpowers read
// — it lives in a different function, checking a different err — must
// not satisfy the idiom vacuously. The read's own function has no
// existence check of its own here.
func TestFreshCloneGuard_DoesNotAcceptAnUnrelatedErrEqualsNil(t *testing.T) {
	f := parseSnippet(t, `package p

import (
	"os"
	"path/filepath"
	"testing"
)

func unrelatedHelper() bool {
	_, err := os.Stat("go.mod")
	return err == nil
}

func TestUnguardedReadNearAnUnrelatedCheck(t *testing.T) {
	if !unrelatedHelper() {
		t.Skip("no go.mod")
	}
	body, err := os.ReadFile(filepath.Join("..", "docs", "superpowers", "matrix.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	_ = body
}
`)
	if !fileReadsUnderSuperpowers(f) {
		t.Fatal("fileReadsUnderSuperpowers = false, want true — this snippet reads under docs/superpowers")
	}
	if hasAbsentGuardIdiom(f) {
		t.Fatal("hasAbsentGuardIdiom = true, want false — the file's only \"err == nil\" and Skip call guard an UNRELATED os.Stat(\"go.mod\"), not the docs/superpowers read, and must not satisfy the idiom vacuously")
	}
}
