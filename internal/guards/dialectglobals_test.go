// SPDX-License-Identifier: GPL-3.0-or-later

package guards

import (
	"go/ast"
	"sort"
	"testing"
)

// Guards over core/cat's dialect data: that the policies M9c-0 promoted
// onto cat.Dialect have not drifted back to package level, that the three
// gate-reaching validators still take a receiver, and that the milestone's
// transitive audit can be RE-DERIVED rather than merely quoted.
//
// SCOPE, deliberately small. These are cheap structural checks. They do NOT
// attempt to prove that a validator's DECISION uses its receiver's value.
// THE BEHAVIOURAL PEER-DIALECT TESTS DO THAT — core/cat/clarifier_test.go,
// mtpolicy_test.go, mwkind_test.go and milestonefixes_test.go build
// dialects that DISAGREE with the FT-710 and check each behaves as its own
// data says. That is the defence; this file is a tripwire.
//
// WHY IT IS THIS SMALL. An earlier version did attempt the semantic proof
// with an untyped AST approximation — whether the body read the receiver
// field, assigned to it, rebound the receiver, took its address, called a
// pointer-receiver method, and so on. It reached 654 lines, and FIVE
// successive review rounds each found another shape it missed: a nested
// write, a method value, an aliased receiver type. Each round closed the
// named shape and asserted a new boundary; each next round found something
// inside that boundary.
//
// A process review judged the loop disproportionate — the guard was
// "a syntactic tripwire trying to approximate a semantic dataflow property
// already tested behaviourally", and treating each new bypass as a release
// blocker confused an approximate instrument with the product. Its verdict:
// keep the cheap checks, keep the audit reporter, let the behavioural tests
// carry the semantics.
//
// So do not reintroduce the semantic checks here. If a stronger guarantee
// is ever wanted, the instrument is go/types plus dataflow analysis — not
// more AST shapes.

// promotedConstants are the package-level names M9c-0 moved onto the
// Dialect receiver. Each was previously read by a method THROUGH its
// receiver while the datum came from a package const — the defect shape the
// milestone existed to remove — and each reaches the OUTBOUND WRITE GATE,
// where a wrong value can authorise bytes that reach a radio.
var promotedConstants = []string{
	"mtTagMaxBytes", // bounded build, parse AND validMTCommand
	"mtClearTag",    // the empty-tag encoding, emitted into MT Set frames
	"clarMaxAbsHz",  // reached the gate through validateMWFields
	"clarStepHz",    // ditto, and a radio characteristic, not a field width
}

// gateReachingValidators must be Dialect METHODS. Each is reached by
// Dialect.AllowedCommand, so a package-level version would bind every
// dialect to the FT-710's rule at the one point deciding what is written to
// a radio.
//
// SHAPE ONLY — that the seam exists. Whether a body then honours its
// receiver is the behavioural tests' job, not this one's.
var gateReachingValidators = []string{
	"validMTTag",
	"validClarHz",
	"validateMWFields",
}

// TestDialectPromotedDataIsNotAPackageGlobal pins that none of the promoted
// names has come back as a package-level declaration in core/cat.
func TestDialectPromotedDataIsNotAPackageGlobal(t *testing.T) {
	files := catFiles(t)
	if len(files) == 0 {
		t.Fatal("parsed no core/cat files — this guard would pass vacuously")
	}

	declared := map[string]string{} // name -> file it is declared in
	for _, pf := range files {
		for _, d := range pf.file.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok {
				continue
			}
			if gd.Tok.String() != "const" && gd.Tok.String() != "var" {
				continue
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, n := range vs.Names {
					declared[n.Name] = pf.relPath
				}
			}
		}
	}
	if len(declared) == 0 {
		t.Fatal("found no package-level declarations in core/cat — the walk is broken")
	}

	for _, name := range promotedConstants {
		if where, found := declared[name]; found {
			t.Errorf("%s is a package-level declaration again, at %s — it is dialect data and belongs on the receiver; a method reading it binds every dialect to the FT-710's value at the point that decides what reaches a radio", name, where)
		}
	}
	t.Logf("scanned %d core/cat files, %d package-level declarations", len(files), len(declared))
}

// TestGateReachingValidatorsAreDialectMethods pins that the three
// gate-reaching validators still take a Dialect receiver.
//
// A package-level version cannot consult a dialect at all, so this catches
// the cheap regression — a validator demoted back to a function taking a
// fixed constant — without pretending to verify what the body does with the
// receiver it has.
func TestGateReachingValidatorsAreDialectMethods(t *testing.T) {
	files := catFiles(t)

	type decl struct {
		isMethod bool
		recvType string
		where    string
	}
	found := map[string][]decl{}

	for _, pf := range files {
		for _, d := range pf.file.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok {
				continue
			}
			rec := decl{where: pf.relPath}
			if fd.Recv != nil && len(fd.Recv.List) > 0 {
				rt := fd.Recv.List[0].Type
				if se, ok := rt.(*ast.StarExpr); ok {
					rt = se.X
				}
				if id, ok := rt.(*ast.Ident); ok {
					rec.isMethod = true
					rec.recvType = id.Name
				}
			}
			found[fd.Name.Name] = append(found[fd.Name.Name], rec)
		}
	}

	for _, name := range gateReachingValidators {
		decls, ok := found[name]
		if !ok {
			t.Errorf("%s is not declared anywhere in core/cat — if it was renamed, update this guard deliberately rather than letting it pass vacuously", name)
			continue
		}
		for _, d := range decls {
			if !d.isMethod {
				t.Errorf("%s at %s is a package-level function, want a method on Dialect — it is reached by AllowedCommand, so a package-level version binds every dialect to the FT-710's rule at the gate", name, d.where)
				continue
			}
			if d.recvType != "Dialect" {
				t.Errorf("%s at %s has receiver %s, want Dialect", name, d.where, d.recvType)
			}
		}
	}
}

// TestTransitiveGlobalReachSetIsReported re-derives the audit and reports
// it, so the number in the design document has a mechanism behind it.
//
// It asserts SHAPE rather than an exact count: pinning "44" would make an
// unrelated new constant fail this guard for no reason, and a guard that
// cries wolf gets its number bumped rather than read. What it does pin is
// that the walk is genuinely transitive — it must reach at least one
// identifier that is NOT directly referenced in any Dialect method body,
// because that is the property the first two attempts at this audit lacked.
func TestTransitiveGlobalReachSetIsReported(t *testing.T) {
	files := catFiles(t)

	pkgLevel := map[string]bool{}
	funcs := map[string]*ast.FuncDecl{}
	var methods []*ast.FuncDecl

	for _, pf := range files {
		for _, d := range pf.file.Decls {
			switch x := d.(type) {
			case *ast.GenDecl:
				if x.Tok.String() != "const" && x.Tok.String() != "var" {
					continue
				}
				for _, spec := range x.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok {
						for _, n := range vs.Names {
							if n.Name != "_" {
								pkgLevel[n.Name] = true
							}
						}
					}
				}
			case *ast.FuncDecl:
				if x.Recv == nil {
					funcs[x.Name.Name] = x
					continue
				}
				rt := x.Recv.List[0].Type
				if se, ok := rt.(*ast.StarExpr); ok {
					rt = se.X
				}
				if id, ok := rt.(*ast.Ident); ok && id.Name == "Dialect" {
					methods = append(methods, x)
				}
			}
		}
	}
	if len(methods) == 0 {
		t.Fatal("found no Dialect methods — the walk is broken and every assertion here is vacuous")
	}

	// scan collects package-level identifier references and calls to
	// package-level functions, NEVER descending into SelectorExpr.Sel — a
	// field or method name, not a package identifier.
	scan := func(fd *ast.FuncDecl) (refs, calls []string) {
		var visit func(ast.Node) bool
		visit = func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.SelectorExpr:
				ast.Inspect(x.X, visit)
				return false
			case *ast.CallExpr:
				if id, ok := x.Fun.(*ast.Ident); ok {
					if _, is := funcs[id.Name]; is {
						calls = append(calls, id.Name)
					}
				}
			case *ast.Ident:
				if pkgLevel[x.Name] {
					refs = append(refs, x.Name)
				}
			}
			return true
		}
		ast.Inspect(fd, visit)
		return
	}

	direct := map[string]bool{}
	transitive := map[string]bool{}
	for _, m := range methods {
		r, _ := scan(m)
		for _, name := range r {
			direct[name] = true
		}

		seen := map[string]bool{}
		var walk func(*ast.FuncDecl)
		walk = func(fn *ast.FuncDecl) {
			refs, calls := scan(fn)
			for _, name := range refs {
				transitive[name] = true
			}
			for _, c := range calls {
				if seen[c] {
					continue
				}
				seen[c] = true
				if next, ok := funcs[c]; ok {
					walk(next)
				}
			}
		}
		walk(m)
	}

	var onlyViaHelper []string
	for name := range transitive {
		if !direct[name] {
			onlyViaHelper = append(onlyViaHelper, name)
		}
	}
	sort.Strings(onlyViaHelper)

	// THE LOAD-BEARING ASSERTION. If this set is empty, the walk collapsed
	// to the lexical sweep that missed mtTagMaxBytes inside validMTTag and
	// clarMaxAbsHz inside validClarHz — the misses that made the original
	// audit's headline number wrong.
	if len(onlyViaHelper) == 0 {
		t.Error("the transitive walk found nothing a direct sweep would miss — it has degenerated into a lexical sweep, which cannot test Dialect's 'every helper those methods delegate to' rule")
	}

	t.Logf("Dialect methods: %d; package-level const/var reachable: %d (direct %d, +%d only via a delegated helper: %v)",
		len(methods), len(transitive), len(direct), len(onlyViaHelper), onlyViaHelper)
}

// catFiles parses core/cat's non-test files.
func catFiles(t *testing.T) []parsedFile {
	t.Helper()
	var out []parsedFile
	for _, pf := range parseRepo(t) {
		if pf.relDir == "core/cat" {
			out = append(out, pf)
		}
	}
	return out
}
