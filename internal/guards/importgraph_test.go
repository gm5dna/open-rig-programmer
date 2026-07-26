// SPDX-License-Identifier: GPL-3.0-or-later

// Package guards holds repository-wide architectural guard tests (M3
// Codex-review fix wave, Fix 9 — the adjudicated remedy for the review's
// HIGH findings #1/#3). It contains no production code: only tests that
// parse the repo's own source (go/parser, exactly the technique
// internal/fakeradio's imports_test already uses) and pin the
// composition-root discipline the write-safety story depends on.
//
// The threat model these guards pin is OUR OWN COMPOSITION, not external
// importers: nothing in Go stops a third-party module importing
// core/transport and calling Engine.Do with a hand-built MW frame — and
// nothing here tries to. What the hardware write guard's layered design
// (capability profiles, codeplug.Diff's gates, the clone service's
// choreography, WriteChannel's own re-check) actually promises is that
// THIS repository's binaries can only reach the wire's write path through
// every one of those layers. That promise holds only for as long as no
// code inside this repo quietly grows a new call site below the policy
// layers — which is precisely the regression these tests refuse.
//
// The FULL write-capability split (separating the write-capable surface
// into its own package/capability so the compiler, not a test, enforces
// this) was a ledgered M5b-flip precondition. At the flip (13/07/2026)
// it was adjudicated RATIFIED-AS-NOT-NEEDED: these guards pin the write
// path repo-wide; three consecutive Codex milestone reviews (M3, M4,
// M6) confirmed no writable RealHardware composition existed; and the
// M3 adjudication placed external importers outside the threat model
// (above). writeTrialsComplete has flipped with that adjudication
// restated in its doc comment (core/driver/ft710/caps.go) — and these
// guards REMAIN the enforcement, unchanged: post-flip they matter MORE,
// not less, since the capability veto no longer backstops them.
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

// modulePrefix is this project's module path (go.mod). Mirrors
// internal/fakeradio's imports_test, including its warning: this is NOT
// the repository directory name, and getting it wrong would make every
// import-based check below pass vacuously.
const modulePrefix = "github.com/gm5dna/open-rig-programmer/"

// repoRoot walks up from the test's working directory to the directory
// containing go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod found walking up from the test's working directory")
		}
		dir = parent
	}
}

// parsedFile is one non-test Go file's parse result plus where it lives.
type parsedFile struct {
	// relDir is the file's directory, repo-relative, slash-separated
	// (e.g. "core/driver/ft710").
	relDir string
	// relPath is the file itself, repo-relative, for failure messages.
	relPath string
	file    *ast.File
}

// parseRepo parses every non-test .go file in the repository (skipping
// dot-directories, node_modules, and app/frontend) and returns them with
// their repo-relative locations.
func parseRepo(t *testing.T) []parsedFile {
	t.Helper()
	root := repoRoot(t)
	fset := token.NewFileSet()

	var out []parsedFile
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && (strings.HasPrefix(name, ".") || name == "node_modules" || name == "frontend") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return perr
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		out = append(out, parsedFile{relDir: filepath.ToSlash(filepath.Dir(rel)), relPath: rel, file: f})
		return nil
	})
	if err != nil {
		t.Fatalf("walking the repository: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("parseRepo found zero non-test Go files — the walk's filters likely excluded everything; every guard below would pass vacuously")
	}
	return out
}

// inTree reports whether relDir is prefix itself or anywhere beneath it.
func inTree(relDir, prefix string) bool {
	return relDir == prefix || strings.HasPrefix(relDir, prefix+"/")
}

// importsPath reports whether f imports importPath, and under what local
// name (the explicit alias, or the path's base name by default). The ONLY
// remaining user is transport.Engine.Do's pre-filter below, which needs
// only the "does f import core/transport" existence check (ok); the
// returned localName is discarded there ("_ = transportName") because the
// .Do detection is receiver-typed, not package-selector-shaped.
// BuildMWSet/BuildMTSet no longer uses this helper at all: since M9b it
// matches by method name alone and needs no import-name lookup (see
// TestWritePathReachableOnlyThroughDriver's doc comment).
func importsPath(f *ast.File, importPath string) (localName string, ok bool) {
	for _, imp := range f.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil || p != importPath {
			continue
		}
		if imp.Name != nil {
			return imp.Name.Name, true
		}
		return path_Base(p), true
	}
	return "", false
}

// path_Base is path.Base without the import (one dependency fewer to
// reason about in a guard test).
func path_Base(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}

// looksLikeOnce reports whether expr (a selector's receiver) is
// syntactically a sync.Once-ish value — an identifier or field selector
// whose name ends in "Once" or "once". Filters the one known systematic
// false positive of the approximate ".Do" check below (sync.Once.Do).
func looksLikeOnce(expr ast.Expr) bool {
	name := ""
	switch x := expr.(type) {
	case *ast.Ident:
		name = x.Name
	case *ast.SelectorExpr:
		name = x.Sel.Name
	}
	return strings.HasSuffix(name, "Once") || strings.HasSuffix(name, "once")
}

// TestWritePathReachableOnlyThroughDriver pins the composition-root
// discipline (see the package doc comment): the raw wire-write mechanisms
// — transport.Engine.Do (the only place bytes cross the wire) and
// cat.BuildMWSet/cat.BuildMTSet (the only builders of Set frames that
// mutate a radio's memory) — are referenced OUTSIDE their own packages
// only by core/driver/**; and driver.Session.WriteChannel (the policy-
// gated write seam those mechanisms compose into) is referenced only by
// core/driver/** and core/clone/** (the clone service being the single
// sanctioned policy layer above it).
//
// APPROXIMATE, deliberately (and per the adjudication): this is a plain
// AST walk over selector expressions, not a type-checked analysis.
// Concretely:
//   - "Engine.Do" is detected as any method CALL `x.Do(...)` in a file
//     that imports core/transport (excluding sync.Once-shaped receivers —
//     see looksLikeOnce); a file that imports transport AND calls some
//     unrelated type's .Do would false-positive, and a dot-import would
//     false-negative. Neither exists in this repo, and a false positive
//     merely prompts a human look at a genuinely unusual file.
//   - BuildMWSet/BuildMTSet are detected as ANY selector named
//     BuildMWSet or BuildMTSet OUTSIDE core/cat's own tree, matched by
//     method name alone, whatever the receiver — the SAME two-part shape
//     (name-only match + owning-tree carve-out) this test already applies
//     to WriteChannel below, not name-only alone. The core/cat carve-out
//     is not a migration-window nicety: this check's job has never been
//     to police core/cat's own internals, only what happens outside it
//     that isn't core/driver/**, and after the dialect seam lands,
//     core/cat's own dialect implementations will keep calling one
//     another via selectors (e.g. d.BuildMWSet(...)) forever, not just
//     during Task 54's transitional package-level delegates.
//     Amended at M9b: before the dialect seam these were package-level
//     functions and an exact package-qualified check (sel.X an
//     *ast.Ident naming the core/cat import) sufficed; the seam turns
//     every call into a nested selector — cat.FT710.BuildMWSet — that
//     check cannot see. Name-only is strictly MORE inclusive within the
//     tree this check still applies to, not weaker: it catches every
//     call the old form caught outside core/cat, plus the new shape.
//   - "Session.WriteChannel" is detected as ANY selector named
//     WriteChannel outside the allowed trees, whatever the receiver's
//     type. A future unrelated type with a WriteChannel method elsewhere
//     would false-positive — acceptable: such a name collision on this
//     project's most safety-loaded method name deserves the look.
//
// The compiler-enforced version of this boundary (a separate
// write-capability package) was adjudicated RATIFIED-AS-NOT-NEEDED at
// the M5b flip (see the package doc comment); THIS test is, and
// remains, the fence.
func TestWritePathReachableOnlyThroughDriver(t *testing.T) {
	const transportPath = modulePrefix + "core/transport"

	files := parseRepo(t)

	// Sanity counters: the allowed call sites must actually be SEEN, or
	// a walker/filter bug would let every check pass vacuously.
	var sawDriverEngineDo, sawDriverBuildMW, sawCloneWriteChannel bool

	for _, pf := range files {
		inDriver := inTree(pf.relDir, "core/driver")
		inClone := inTree(pf.relDir, "core/clone")

		// (a) transport.Engine.Do — approximate; see the test doc comment.
		if transportName, ok := importsPath(pf.file, transportPath); ok && !inTree(pf.relDir, "core/transport") {
			_ = transportName // the .Do check is receiver-typed, not package-selector-shaped
			ast.Inspect(pf.file, func(n ast.Node) bool {
				call, isCall := n.(*ast.CallExpr)
				if !isCall {
					return true
				}
				sel, isSel := call.Fun.(*ast.SelectorExpr)
				if !isSel || sel.Sel.Name != "Do" || looksLikeOnce(sel.X) {
					return true
				}
				if inDriver {
					sawDriverEngineDo = true
					return true
				}
				t.Errorf("%s: calls .Do on a value in a file importing core/transport — transport.Engine.Do may be reached outside core/transport only from core/driver/** (composition-root discipline; see this test's doc comment)", pf.relPath)
				return true
			})
		}

		// (a) BuildMWSet / BuildMTSet, matched by NAME alone, whatever
		// the receiver — OUTSIDE core/cat's own tree (the carve-out a
		// same-day Codex review, C1, found missing from the first cut of
		// this amendment: without it, the check fired inside core/cat
		// itself, which defeats the whole point — Task 54's package-level
		// delegates, and every dialect implementation's internal calls
		// after Task 55, are selectors named BuildMWSet/BuildMTSet living
		// INSIDE core/cat).
		//
		// Amended at M9b. Before the dialect seam these were
		// package-level functions and this check required sel.X to be an
		// *ast.Ident naming the core/cat import. The seam makes every
		// call a nested selector — cat.FT710.BuildMWSet, or
		// s.dialect.BuildMWSet — which that form silently stops
		// matching. Amended AHEAD of the migration precisely so no task
		// lands on a red tree; this form recognises both shapes.
		//
		// Name-only is LOOSER but strictly MORE INCLUSIVE within the tree
		// this check still applies to: every call the old form caught
		// outside core/cat, this one still catches. It is also the same
		// two-part shape (name-only match + owning-tree carve-out) this
		// guard already applies to WriteChannel in (b) below, so it is
		// house precedent rather than a new compromise.
		if !inTree(pf.relDir, "core/cat") {
			ast.Inspect(pf.file, func(n ast.Node) bool {
				sel, isSel := n.(*ast.SelectorExpr)
				if !isSel {
					return true
				}
				if sel.Sel.Name != "BuildMWSet" && sel.Sel.Name != "BuildMTSet" {
					return true
				}
				if inDriver {
					sawDriverBuildMW = true
					return true
				}
				t.Errorf("%s: references .%s — the Set-frame builders may be used outside core/cat only from core/driver/** (composition-root discipline; see this test's doc comment)", pf.relPath, sel.Sel.Name)
				return true
			})
		}

		// (b) Session.WriteChannel — approximate; see the test doc comment.
		if !inDriver && !inClone {
			ast.Inspect(pf.file, func(n ast.Node) bool {
				sel, isSel := n.(*ast.SelectorExpr)
				if !isSel || sel.Sel.Name != "WriteChannel" {
					return true
				}
				t.Errorf("%s: references .WriteChannel — driver.Session.WriteChannel may be referenced only from core/driver/** and core/clone/** (the clone service is the single sanctioned policy layer above the driver's write seam)", pf.relPath)
				return true
			})
		} else if inClone {
			ast.Inspect(pf.file, func(n ast.Node) bool {
				if sel, isSel := n.(*ast.SelectorExpr); isSel && sel.Sel.Name == "WriteChannel" {
					sawCloneWriteChannel = true
				}
				return true
			})
		}
	}

	if !sawDriverEngineDo {
		t.Error("never saw core/driver/** call Engine.Do — the walker or its filters are broken, and every check above passed vacuously")
	}
	if !sawDriverBuildMW {
		t.Error("never saw core/driver/** reference cat.BuildMWSet/BuildMTSet — the walker or its filters are broken, and every check above passed vacuously")
	}
	if !sawCloneWriteChannel {
		t.Error("never saw core/clone/** reference Session.WriteChannel — the walker or its filters are broken, and every check above passed vacuously")
	}
}
