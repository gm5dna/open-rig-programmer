// SPDX-License-Identifier: GPL-3.0-or-later

package guards

import (
	"fmt"
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
// docs/superpowers read for the same guard": it scans every .go file in
// the module, finds every one that actually READS under the gitignored
// tree, and requires an absent-guard idiom in each.
//
// THE CLOSING FIX WAVE MOVED THREE THINGS, all of them cases where this
// guard was green over something it claims to catch:
//
//   - Scope. The follow-up was written as "a *_test.go that reads
//     docs/superpowers", and this file implemented it literally — then the
//     same sweep produced a reader that is not a _test.go
//     (core/driver/internal/drivertest/citations.go, which four packages'
//     provenance pins depend on). The requirement was under-specified, not
//     the implementation. Every .go file is walked now; the Skip/Log half
//     is still asked only of _test.go, which is the only kind holding a
//     *testing.T.
//   - Detection. Requiring the path literal and the os call in ONE function
//     body missed that same reader, which builds its paths in
//     IcomCitationPin and reads them in readAuthority. Detection is
//     file-scoped now; measured over this module it finds exactly the three
//     real readers.
//   - Binding. The existence check merely had to be an err-shaped "x == nil"
//     somewhere in a function that read something. It must now name the
//     error THAT READ bound, so a probe of an unrelated path in the same
//     function no longer stands in for it, and the Skip/Log call must be on
//     a testing handle the function was handed rather than on any receiver
//     with a method of that name.
//
// FIX ROUND 1 (F7) WIDENED AND RE-SCOPED BOTH HALVES:
//
//   - Detection (fileReadsUnderSuperpowers) used to require the path
//     segment "superpowers" as its OWN string literal — the two-argument
//     filepath.Join("...", "docs", "superpowers", ...) shape both known
//     readers happen to use — which missed a reader spelling the same
//     path as one joined literal, "docs/superpowers/x.md", or built via
//     string concatenation. It now also matches that joined form, and
//     the bare "superpowers/..." segment without the "docs/" prefix, and
//     both forms' Windows-separator spelling — see
//     namesSuperpowersPath's own doc comment for the exact shapes and
//     why a bare "superpowers" substring anywhere is still refused. That
//     widening costs nothing against the four files that merely CITE
//     docs/superpowers/*.md as evidence
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
// guard cares about: the bare path segment "superpowers", exactly as
// filepath.Join("...", "docs", "superpowers", ...) — the two known
// readers' shape — spells it as its OWN argument; the joined form
// "docs/superpowers" appearing anywhere inside a longer literal (F7: a
// reader is at least as likely to write
// filepath.Join(root, "docs/superpowers", "x.md") or
// ".../docs/superpowers/x.md" as one string); the bare joined segment
// "superpowers/..." with no "docs/" prefix at all (closing fix wave,
// carried-forward minor: a reader that already holds the docs root and
// writes filepath.Join(docsDir, "superpowers/matrix.md") names the same
// path without ever spelling "docs"); and every one of those forms
// re-spelled with a Windows path separator ("docs\superpowers",
// "superpowers\...").
//
// NOT A BARE "superpowers" SUBSTRING ANYWHERE, deliberately: fix round 1
// tried that first and it false-positived on
// internal/guards/retirednames_test.go's skipDirs[".superpowers"] — a
// WalkDir EXCLUSION entry that keeps that test from ever descending into
// the directory, the opposite of a read. Every shape accepted above
// requires a following path separator (or the exact bare segment), which
// ".superpowers" — with no separator anywhere in the literal — cannot
// satisfy, so the false positive stays excluded even with the "docs/"
// prefix no longer required.
func namesSuperpowersPath(s string) bool {
	for _, sep := range [...]string{"/", `\`} {
		if s == "superpowers" ||
			strings.Contains(s, "docs"+sep+"superpowers") ||
			strings.HasPrefix(s, "superpowers"+sep) ||
			strings.Contains(s, sep+"superpowers"+sep) {
			return true
		}
	}
	return false
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

// fileReadsUnderSuperpowers reports whether f BOTH names a path under the
// gitignored tree in a string literal AND calls one of superpowersReadCalls.
// Both are necessary for a real read; a citation of the path in a comment is
// never an ast.BasicLit, and a file that names no such path reads no such
// path.
//
// FILE-SCOPED, NOT FUNCTION-SCOPED (closing review). The earlier rule wanted
// the literal and the os call in ONE function body, which is the shape both
// crosscheck readers happen to have — and the shape this very sweep's new
// reader does not. core/driver/internal/drivertest/citations.go builds its
// paths in IcomCitationPin and reads them in readAuthority, two functions
// apart, so the co-occurrence rule saw nothing at all: the guard added to
// stop an unguarded docs/superpowers read did not cover the sweep's own.
// Measured over this module, the file-scoped rule finds exactly the three
// real readers and nothing else — core/civ/tier_test.go names the tree but
// calls no os read, and this file names it only in namesSuperpowersPath's
// own table.
func fileReadsUnderSuperpowers(f *ast.File) bool {
	names, reads := false, false
	ast.Inspect(f, func(n ast.Node) bool {
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
	return names && reads
}

// isEligibleRead reports whether call is an os read that could be reading
// under the gitignored tree: one whose arguments NAME such a path, or whose
// arguments carry no concrete path literal at all and so may have been handed
// one (citations.go's `os.ReadFile(path)`, ic7100's `os.ReadFile(matrixPath)`,
// icr8600's `os.ReadFile(path)`).
//
// A READ THAT NAMES SOME OTHER PATH IS NOT ELIGIBLE, and that exclusion is
// the whole point: os.Stat("go.mod") is demonstrably not the docs/superpowers
// read, so an existence check on ITS error cannot stand in for one on the
// read that matters. TestFreshCloneGuard_DoesNotAcceptAnUnrelatedErrEqualsNil
// and TestFreshCloneGuard_DoesNotAcceptACheckOnAnotherReadsError are the two
// fixtures for it.
func isEligibleRead(call *ast.CallExpr) bool {
	if _, ok := isOSCall(call); !ok {
		return false
	}
	namesPath, otherLiteral := false, false
	for _, arg := range call.Args {
		ast.Inspect(arg, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			s, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			if namesSuperpowersPath(s) {
				namesPath = true
			} else {
				otherLiteral = true
			}
			return true
		})
	}
	return namesPath || !otherLiteral
}

// isErrShapedIdent reports whether e is an identifier whose name looks like an
// error variable: exactly "err", or any name ending in "err"
// case-insensitively ("readErr", "statErr"). A read binds a payload as well as
// an error — `data, err := os.ReadFile(path)` — and only the error half is a
// question about whether the file is there.
func isErrShapedIdent(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	if !ok {
		return false
	}
	name := strings.ToLower(id.Name)
	return name == "err" || strings.HasSuffix(name, "err")
}

// readErrorsIn returns the names of the error variables that eligible reads in
// fn BIND. `data, err := os.ReadFile(path)` contributes "err"; so does
// `if _, statErr := os.Stat(p); ...`, whose assignment is the if statement's
// own Init.
func readErrorsIn(fn *ast.FuncDecl) map[string]bool {
	out := map[string]bool{}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || len(as.Rhs) != 1 {
			return true
		}
		call, ok := as.Rhs[0].(*ast.CallExpr)
		if !ok || !isEligibleRead(call) {
			return true
		}
		for _, lhs := range as.Lhs {
			if isErrShapedIdent(lhs) {
				out[lhs.(*ast.Ident).Name] = true
			}
		}
		return true
	})
	return out
}

// notExistCheckSubject returns the name of the error variable an accepted
// existence check is asking about: os.IsNotExist(err), errors.Is(err,
// fs.ErrNotExist) or errors.Is(err, os.ErrNotExist) (either spelling of the
// sentinel), or a plain "err == nil" comparison — icr8600's own
// matrixAuthorityPresent spelling ("return err == nil"), treating absence as
// fine rather than as a failure.
//
// EQL ONLY, NOT NEQ: "err != nil { t.Fatalf(...) }" is ORDINARY error
// handling present in nearly every test that does any I/O at all, guard or
// not, and accepting it here would let any such check satisfy this idiom
// vacuously — exactly what F7(c) flagged. The fixtures below all carry one
// after the read they leave unguarded.
func notExistCheckSubject(n ast.Node) (string, bool) {
	if call, ok := n.(*ast.CallExpr); ok {
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return "", false
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok {
			return "", false
		}
		accepted := (pkg.Name == "os" && sel.Sel.Name == "IsNotExist" && len(call.Args) == 1) ||
			(pkg.Name == "errors" && sel.Sel.Name == "Is" && len(call.Args) == 2 && isErrNotExistSentinel(call.Args[1]))
		if !accepted {
			return "", false
		}
		id, ok := call.Args[0].(*ast.Ident)
		if !ok {
			return "", false
		}
		return id.Name, true
	}
	bin, ok := n.(*ast.BinaryExpr)
	if !ok || bin.Op != token.EQL {
		return "", false
	}
	isNil := func(e ast.Expr) bool {
		id, ok := e.(*ast.Ident)
		return ok && id.Name == "nil"
	}
	var other ast.Expr
	switch {
	case isNil(bin.X):
		other = bin.Y
	case isNil(bin.Y):
		other = bin.X
	default:
		return "", false
	}
	id, ok := other.(*ast.Ident)
	if !ok {
		return "", false
	}
	return id.Name, true
}

// isErrNotExistSentinel reports whether e is fs.ErrNotExist or os.ErrNotExist.
func isErrNotExistSentinel(e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "ErrNotExist"
}

// testingHandlesIn returns the names of fn's parameters (and receiver) whose
// type is a testing handle, so a Skip/Log call can be checked against the
// RECEIVER it is called on rather than merely against its method name.
func testingHandlesIn(fn *ast.FuncDecl) map[string]bool {
	out := map[string]bool{}
	isHandle := func(e ast.Expr) bool {
		if star, ok := e.(*ast.StarExpr); ok {
			e = star.X
		}
		sel, ok := e.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "testing" {
			return false
		}
		switch sel.Sel.Name {
		case "T", "B", "F", "TB":
			return true
		}
		return false
	}
	var lists []*ast.FieldList
	if fn.Recv != nil {
		lists = append(lists, fn.Recv)
	}
	if fn.Type != nil {
		lists = append(lists, fn.Type.Params)
	}
	for _, list := range lists {
		if list == nil {
			continue
		}
		for _, field := range list.List {
			if !isHandle(field.Type) {
				continue
			}
			for _, name := range field.Names {
				out[name.Name] = true
			}
		}
	}
	return out
}

// isSkipOrLogCall reports whether call is <handle>.Skip/Skipf/Log/Logf on one
// of the given testing handles.
//
// RECEIVER-CHECKED (closing review): the earlier version matched the method
// NAME on any receiver at all, so a logger's own .Log, or any type with a
// Skip method, satisfied the reporting half of the idiom. Only a *testing.T,
// *testing.B, *testing.F or testing.TB the enclosing function was actually
// handed can report a skipped check to the test runner.
func isSkipOrLogCall(call *ast.CallExpr, handles map[string]bool) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	switch sel.Sel.Name {
	case "Skip", "Skipf", "Log", "Logf":
	default:
		return false
	}
	recv, ok := sel.X.(*ast.Ident)
	return ok && handles[recv.Name]
}

// hasAbsentGuardIdiom reports whether f carries a fresh-clone absent guard.
// reportHalf asks for the Skip/Log leg as well, which only a _test.go file
// can have: a non-test reader holds no *testing.T and reports absence to its
// CALLER instead (core/driver/internal/drivertest/citations.go's readAuthority
// returns present=false, and CitationPin.Assert does the logging).
//
// THE EXISTENCE CHECK IS BOUND TO THE READ'S OWN ERROR (closing review). It
// used to be enough that some accepted check sat in the same function as some
// os call, which two shapes walked straight past: a check on an UNRELATED
// error in that function (an os.Stat("go.mod") probe), and a check on a
// non-error variable that merely compared to nil. The check must now name a
// variable an ELIGIBLE read in that same function actually bound, so the
// question it asks is the question the read raises.
//
// THE REPORT HALF IS FUNCTION-SCOPED TOO, to a function containing an
// eligible read — icr8600's pinMatrixA and readMatrixA each read under
// docs/superpowers and t.Logf/t.Skip in their own body, having asked
// matrixAuthorityPresent for the answer, so the two halves live in different
// functions and only the CHECK half can be pinned to the checking one.
func hasAbsentGuardIdiom(f *ast.File, reportHalf bool) bool {
	existenceOK, reportOK := false, false
	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			return true
		}
		bound := readErrorsIn(fn)
		if len(bound) == 0 {
			return true
		}
		handles := testingHandlesIn(fn)
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if name, ok := notExistCheckSubject(n); ok && bound[name] {
				existenceOK = true
			}
			if call, ok := n.(*ast.CallExpr); ok && isSkipOrLogCall(call, handles) {
				reportOK = true
			}
			return true
		})
		return true
	})
	if !reportHalf {
		return existenceOK
	}
	return existenceOK && reportOK
}

// TestFreshCloneGuardCoversEveryDocsSuperpowersRead is this file's main test:
// every Go file that reads under docs/superpowers must also carry the
// absent-guard idiom.
//
// EVERY GO FILE, NOT EVERY _test.go (closing review). The follow-up was
// written as "a *_test.go that reads docs/superpowers must carry the
// absent-guard idiom", and this guard implemented that literally — then the
// same sweep produced a reader that is not a _test.go, in
// core/driver/internal/drivertest/citations.go, which four packages'
// provenance pins now depend on. The requirement was under-specified, not the
// implementation; the walk now covers the whole module. The Skip/Log half is
// still asked only of _test.go files, because it is the only kind that holds
// a *testing.T.
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
		if !strings.HasSuffix(name, ".go") {
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
		if !hasAbsentGuardIdiom(f, strings.HasSuffix(name, "_test.go")) {
			t.Errorf("%s reads under docs/superpowers (gitignored; absent on a fresh clone or in CI) but carries no absent-guard idiom — an existence check (os.IsNotExist, errors.Is(err, fs.ErrNotExist)/os.ErrNotExist, or err == nil) naming THE ERROR THAT READ ITSELF BOUND, in the same function, and in a _test.go also a Skip/Skipf/Log/Logf on the testing handle: the pattern core/civ/ic7100/crosscheck_test.go:148-160 and core/civ/icr8600/crosscheck_test.go:98-133 use, and core/driver/internal/drivertest/citations.go's readAuthority uses without the reporting half. Otherwise a fresh clone fails this file the way the v1.2.1 CI run did on 30/08/2026", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	// Non-vacuity, and a regression pin on the scan itself: today's
	// population is exactly these three files. A scan that found none of
	// them, or that silently stopped matching one, would leave every check
	// above passing on nothing.
	//
	// THE OLD COMMENT HERE CLAIMED A GREP RESULT THAT WAS FALSE (closing
	// review). It said grep '"superpowers"' --include=*.go finds these two
	// files and nowhere else in the module; by the time it was written the
	// provenance lane had already added a third,
	// core/driver/internal/drivertest/citations.go, and the guard did not
	// notice because it only walked _test.go. Two other files DO name the
	// tree in a string literal and are correctly not readers:
	// core/civ/tier_test.go, which calls no os read at all, and this file,
	// whose only such literals are namesSuperpowersPath's own test table.
	want := map[string]bool{
		"core/civ/ic7100/crosscheck_test.go":           true,
		"core/civ/icr8600/crosscheck_test.go":          true,
		"core/driver/internal/drivertest/citations.go": true,
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
	if hasAbsentGuardIdiom(f, true) {
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
	if !hasAbsentGuardIdiom(f, true) {
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
	if hasAbsentGuardIdiom(f, true) {
		t.Fatal("hasAbsentGuardIdiom = true, want false — the file's only \"err == nil\" and Skip call guard an UNRELATED os.Stat(\"go.mod\"), not the docs/superpowers read, and must not satisfy the idiom vacuously")
	}
}

// TestFreshCloneGuard_DoesNotAcceptANonErrShapedNilCheck pins the
// residual F7(c) vacuity minors-fix1-rereview.md carried forward: the
// old isNotExistCheck accepted "X == nil" for ANY identifier X, so a
// reader function that reads under docs/superpowers unguarded but also
// happens to contain an unrelated "caps == nil" check in its OWN body —
// satisfying the old function-scoped co-occurrence rule vacuously — must
// still fail the idiom. caps is not err-shaped, so it must not be
// mistaken for the existence check.
func TestFreshCloneGuard_DoesNotAcceptANonErrShapedNilCheck(t *testing.T) {
	f := parseSnippet(t, `package p

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUnguardedReadWithAnUnrelatedNilCheck(t *testing.T, caps *int) {
	if caps == nil {
		t.Skip("no caps")
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
	if hasAbsentGuardIdiom(f, true) {
		t.Fatal("hasAbsentGuardIdiom = true, want false — \"caps == nil\" is not err-shaped and must not be accepted as the read's existence check, even though it shares the read's function body and is followed by t.Skip")
	}
}

// TestNamesSuperpowersPath_WidenedForms pins the detection widening
// minors-fix1-rereview.md carried forward: the bare "superpowers/..."
// segment with no "docs/" prefix, and the Windows-separator spelling of
// both forms, must now be detected; the ".superpowers" WalkDir exclusion
// entry (internal/guards/retirednames_test.go's skipDirs) must still not
// be, since it names a directory to SKIP, not a path to read.
func TestNamesSuperpowersPath_WidenedForms(t *testing.T) {
	cases := []struct {
		s    string
		want bool
	}{
		{"superpowers", true},
		{"docs/superpowers", true},
		{"docs/superpowers/matrix.md", true},
		{"superpowers/matrix.md", true},
		{`docs\superpowers\matrix.md`, true},
		{`superpowers\matrix.md`, true},
		{"root/superpowers/matrix.md", true},
		{`root\superpowers\matrix.md`, true},
		{".superpowers", false},
		{"go.mod", false},
	}
	for _, c := range cases {
		if got := namesSuperpowersPath(c.s); got != c.want {
			t.Errorf("namesSuperpowersPath(%q) = %v, want %v", c.s, got, c.want)
		}
	}
}

// TestFreshCloneGuard_DoesNotAcceptACheckOnAnotherReadsError is the closing
// review's own mutation case, and the one shape every earlier round let
// through. The unrelated check now lives in the SAME FUNCTION as the
// docs/superpowers read, and its variable is err-shaped, so neither the
// function-scoping of fix round 1 nor the err-shape requirement of fix round 2
// refuses it: only binding the check to the error THAT READ bound does.
//
// os.Stat("go.mod") is demonstrably not the docs/superpowers read, so a
// successful stat of it says nothing about whether the matrices are there.
func TestFreshCloneGuard_DoesNotAcceptACheckOnAnotherReadsError(t *testing.T) {
	f := parseSnippet(t, `package p

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUnguardedReadBesideAnotherReadsCheck(t *testing.T) {
	if _, statErr := os.Stat("go.mod"); statErr == nil {
		t.Skip("go.mod is present")
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
	if hasAbsentGuardIdiom(f, true) {
		t.Fatal("hasAbsentGuardIdiom = true, want false — \"statErr == nil\" asks about os.Stat(\"go.mod\"), not about the matrix the next line reads unguarded")
	}
}

// TestFreshCloneGuard_CoversANonTestReaderHandedItsPath is MINOR 2's fixture:
// the shape core/driver/internal/drivertest/citations.go actually has, and the
// shape the guard used to miss entirely. The path is built in one function and
// read in another, so no single function body both names the tree and calls
// os.ReadFile — the co-occurrence rule saw nothing at all. Detection is now
// file-scoped, and the guard is asked of the READING function's own error.
//
// The reporting half is not asked of a non-test file: it holds no *testing.T,
// and reports absence to its caller (citations.go returns present=false, and
// CitationPin.Assert does the t.Logf).
func TestFreshCloneGuard_CoversANonTestReaderHandedItsPath(t *testing.T) {
	const src = `package p

import "os"

func authorityPaths() []string {
	return []string{"../../../docs/superpowers/icom-matrices/ic7100-capability-matrix.md"}
}

func readAuthority(paths []string) (string, bool) {
	present := true
	for _, path := range paths {
		body, err := os.ReadFile(path)
		%s
		if err != nil {
			return "", false
		}
		_ = body
	}
	return "", present
}
`
	guarded := parseSnippet(t, fmt.Sprintf(src, `if os.IsNotExist(err) {
			present = false
			continue
		}`))
	if !fileReadsUnderSuperpowers(guarded) {
		t.Fatal("fileReadsUnderSuperpowers = false, want true — the path is named in one function and read in another, which is exactly the reader this guard missed")
	}
	if !hasAbsentGuardIdiom(guarded, false) {
		t.Fatal("hasAbsentGuardIdiom = false, want true — os.IsNotExist names the error os.ReadFile bound, in the reading function")
	}

	unguarded := parseSnippet(t, fmt.Sprintf(src, ""))
	if !fileReadsUnderSuperpowers(unguarded) {
		t.Fatal("fileReadsUnderSuperpowers = false, want true for the unguarded variant too")
	}
	if hasAbsentGuardIdiom(unguarded, false) {
		t.Fatal("hasAbsentGuardIdiom = true, want false — dropping the os.IsNotExist branch is the regression this covers, and it must go red")
	}
}

// TestFreshCloneGuard_DoesNotAcceptASkipOnANonTestingReceiver is MINOR 8's
// fixture. The reporting half used to match the METHOD NAME on any receiver at
// all, so a logger's own .Logf satisfied it. Only a testing handle the
// function was handed can report a skipped check to the test runner.
func TestFreshCloneGuard_DoesNotAcceptASkipOnANonTestingReceiver(t *testing.T) {
	f := parseSnippet(t, `package p

import (
	"os"
	"path/filepath"
	"testing"
)

type logger struct{}

func (logger) Logf(string, ...any) {}

func TestReadWithALoggerNotATestingT(t *testing.T, l logger) {
	body, err := os.ReadFile(filepath.Join("..", "docs", "superpowers", "matrix.md"))
	if os.IsNotExist(err) {
		l.Logf("matrix absent")
		return
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
	if hasAbsentGuardIdiom(f, true) {
		t.Fatal("hasAbsentGuardIdiom = true, want false — l.Logf logs to a logger; nothing tells the test runner this check was skipped")
	}
}
