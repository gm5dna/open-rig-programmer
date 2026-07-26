// SPDX-License-Identifier: GPL-3.0-or-later

package guards

import (
	"go/ast"
	"testing"
)

// TestNewEngineReachableOnlyFromDriver pins the other half of M9b's
// fail-closed story. NewEngine takes the outbound allowlist as a
// parameter, so WHOEVER CALLS IT CHOOSES THE GATE. That choice belongs to
// the driver layer: a call site in app/ or cmd/ could pass a permissive
// func and bypass every policy layer above it.
//
// Matches the name "NewEngine" WHEREVER it appears — as a
// *ast.SelectorExpr.Sel or a bare *ast.Ident — not only inside a
// *ast.CallExpr.Fun (task-58 fix wave, Codex Critical 1). A CallExpr-only
// matcher sees `transport.NewEngine(p, allow)` but misses every
// indirection that puts the same name somewhere else in the tree: a local
// alias (`mk := transport.NewEngine`), a package-level var, a struct
// field holding the constructor, a higher-order argument, generic
// instantiation, or an init()-time indirect call. All of those are a
// *ast.SelectorExpr or *ast.Ident naming NewEngine even though no
// CallExpr.Fun ever names it directly, so matching the name itself,
// wherever it sits, closes the whole family at once.
//
// core/transport is SCANNED, not skipped: only the single top-level,
// non-method NewEngine declaration itself is exempt, and only that
// declaration's own Name identifier — never its body, and never a
// same-named method (task-58 fix wave, Codex Critical 2). A method also
// named NewEngine, or a wrapper hiding an ungated call inside a pruned
// body, would otherwise borrow the real constructor's exemption.
//
// A composite literal — &Engine{...} — never mentions "NewEngine" at
// all, so no name-based matcher, however placed, can see it (task-58 fix
// wave, Codex Important 1). An EARLIER version of this comment argued
// that no OTHER zero-value construction needed catching either, because
// Engine's own allow field would simply stay nil and get refused by
// NewEngine/Do — that argument was wrong, and Codex proved it at runtime
// (task-58 fix round 2, Codex e13/e22): inside package transport, allow
// is an ordinary unexported field, freely assignable after new(Engine)
// (or after starting readLoop by hand), and a frame reached the wire
// from exactly that construction. The real fix is broader than a single
// composite-literal check: inside core/transport, every use of the
// IDENTIFIER Engine AS A TYPE — a composite-literal type (directly, or
// as an array/slice/map element type whose element literals elide their
// own type), a new() argument, a generic type argument, or a var
// declaration's type — is flagged outside NewEngine's own body, and
// `type X = Engine` aliases declared in the SAME FILE are resolved
// first, so renaming the type at the use site (Codex e12) does not
// evade it either. This closes every construction shape the fix-round-2
// review found (e12, e13, e14, e20, e22) — see APPROXIMATE below for
// what still isn't, and is not pretended to be, covered.
//
// APPROXIMATE, deliberately (see the package doc comment): a plain AST
// walk over parsed non-test source, not a type-checked or dataflow
// analysis. The package doc comment's own framing matters here: these
// guards pin THIS REPOSITORY'S OWN COMPOSITION, not a determined
// adversary. Concretely:
//   - Test files are excluded — parseRepo skips every *_test.go file
//     entirely (see its own doc comment). Deliberate, not an oversight:
//     test doubles, table-driven fixtures, and this task's own probe
//     files legitimately construct engines outside core/driver, and
//     policing them here would only get in the way of testing
//     core/transport itself.
//   - The FuncDecl exemption is name-AND-shape-narrow: only fd.Recv ==
//     nil, fd.Name.Name == "NewEngine", declared directly in
//     core/transport (relDir == "core/transport" exactly — Engine's
//     fields are unexported, so nothing outside that exact package could
//     compile a composite literal of it regardless; a hypothetical
//     core/transport subpackage is a DIFFERENT package under Go's rules
//     and gets no exemption). Only that FuncDecl's Name identifier is
//     skipped from the walk — its body is inspected like any other code.
//   - CONFIRMED FALSE POSITIVE, accepted: because matching is by name
//     alone (both "NewEngine" and, now, "Engine" as a type), an entirely
//     unrelated identifier that merely happens to share either name,
//     with no relationship at all to transport's own symbols, is also
//     flagged. Nothing in this repository does this today; if something
//     ever does, the failure prompts a human look at a genuinely unusual
//     name choice, the same trade this package's other guards
//     (WriteChannel, BuildMWSet/BuildMTSet) already accept for their own
//     name-only matches.
//   - GAP, not closed — a driver-tree re-export (Codex fix-round-2 e15,
//     conceptually the sharpest finding). A package under core/driver/**
//     that does nothing but forward NewEngine's own parameters straight
//     through — e.g. func Open(p transport.Port, allow transport.AllowFunc)
//     (*transport.Engine, error) { return transport.NewEngine(p, allow) }
//     — satisfies inTree(pf.relDir, "core/driver") and is treated as a
//     legitimate driver-tree construction, yet nothing stops app/
//     calling that re-export directly and choosing its OWN allow func:
//     exactly the bypass this guard exists to prevent. This guard pins
//     WHERE the identifier NewEngine textually appears, not WHO actually
//     chooses the gate at runtime through however many layers of
//     forwarding — the doc comment above's "whoever calls it chooses the
//     gate... that choice belongs to the driver layer" claim is only as
//     strong as "no driver-tree function forwards the choice back out
//     again", which an AST walk cannot verify. A thin public/internal
//     constructor split is a plausible ACCIDENTAL refactor, not deliberate
//     subversion, and sits squarely inside this package's own composition
//     threat model — recorded honestly as a live gap, not dismissed as
//     out of scope, because closing it would need a call-graph analysis
//     this guard does not attempt.
//   - GAP, not closed — field mutation after construction (Codex
//     fix-round-2 e19). Every guard in this package pins CONSTRUCTION
//     sites, not the field afterwards: nothing stops a future
//     func (e *Engine) SetAllow(AllowFunc) being added to core/transport
//     and called from app/ to widen an already-built engine's gate at
//     any time after NewEngine returns. Engine.allow's own doc comment
//     (core/transport/engine.go:249) already asserts it is
//     "immutable after construction… so Do reads it without
//     synchronisation" — a convention nothing in the type system or this
//     guard enforces, and whose breach would ALSO be a data race (Do
//     reads e.allow unlocked, on the stated assumption nothing writes it
//     after construction). A plausible accidental addition, not
//     deliberate subversion — inside this package's own threat model,
//     and recorded as a gap because pinning it needs a second guard
//     against any exported method assigning to Engine.allow, which this
//     file does not have.
//   - OUT OF THIS GUARD'S THREAT MODEL — //go:linkname (Codex
//     fix-round-2 e17). A //go:linkname directive can reach the real
//     constructor with the name "NewEngine" appearing only inside a
//     comment; parser.ParseFile runs with mode 0 here (see parseRepo),
//     so comments are never in the AST this guard walks at all. Reaching
//     for a linker directive most Go code never touches is deliberate
//     subversion of the toolchain, not an accidental composition
//     mistake — the same carve-out the package doc comment already
//     states for a third party hand-building a wire frame.
//   - OUT OF THIS GUARD'S THREAT MODEL — reflect + unsafe (Codex
//     fix-round-2 e18, confirmed at runtime: a frame reached the wire).
//     reflect.NewAt over an unsafe.Pointer can set Engine.allow directly
//     with no NewEngine mention, no composite literal, and no
//     type-position use of Engine anywhere in the source — it operates
//     entirely through the reflect/unsafe API on a value obtained some
//     other way. Deliberately defeating Go's own visibility rules is
//     outside "our own composition", the same threat-model boundary the
//     package doc comment already draws.
//   - scanned == 0 can no longer actually fire in practice: parseRepo
//     itself already t.Fatals on an empty walk. Kept as defence-in-depth
//     alongside the two sawX counters below, not because it is
//     load-bearing on its own.
func TestNewEngineReachableOnlyFromDriver(t *testing.T) {
	files := parseRepo(t)

	sawDriverConstruction := false
	sawExemptEngineLiteral := false
	scanned := 0

	for _, pf := range files {
		scanned++
		inDriver := inTree(pf.relDir, "core/driver")
		inTransportPkg := pf.relDir == "core/transport"

		// The exempt declaration: NewEngine's own top-level (non-method)
		// FuncDecl, declared directly in core/transport. Only
		// exemptName is skipped below; a same-named method's Name is
		// NOT exempt, and NewEngine's own body is walked exactly like
		// any other code — inExemptBody exists only to excuse the
		// legitimate Engine-typed construction NewEngine's own body
		// performs, not to prune anything from the walk.
		var exemptName *ast.Ident
		inExemptBody := func(ast.Node) bool { return false }
		if inTransportPkg {
			for _, decl := range pf.file.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok || fd.Recv != nil || fd.Name.Name != "NewEngine" {
					continue
				}
				exemptName = fd.Name
				if fd.Body != nil {
					start, end := fd.Body.Pos(), fd.Body.End()
					inExemptBody = func(n ast.Node) bool {
						return n.Pos() >= start && n.End() <= end
					}
				}
				break
			}
		}

		// engineTypeNames resolves every SAME-FILE name that denotes the
		// Engine type: "Engine" itself, plus any `type X = Engine` alias
		// declared in this file (Codex fix-round-2 e12) — chased to a
		// fixed point so a chain (type A = Engine; type B = A) resolves
		// too. Only TRUE aliases (the Assign token present, so X and
		// Engine are the identical type) qualify: a defined type
		// (`type X Engine`, no "=") is a DISTINCT type and would need its
		// own explicit conversion to become a real *Engine, which is a
		// different, unexamined evasion class — see the APPROXIMATE
		// section.
		engineTypeNames := map[string]bool{"Engine": true}
		if inTransportPkg {
			for changed := true; changed; {
				changed = false
				for _, decl := range pf.file.Decls {
					gd, ok := decl.(*ast.GenDecl)
					if !ok {
						continue
					}
					for _, spec := range gd.Specs {
						ts, ok := spec.(*ast.TypeSpec)
						if !ok || !ts.Assign.IsValid() {
							continue
						}
						name := typeNameOf(ts.Type)
						if name != "" && engineTypeNames[name] && !engineTypeNames[ts.Name.Name] {
							engineTypeNames[ts.Name.Name] = true
							changed = true
						}
					}
				}
			}
		}

		reportReference := func(shape string) {
			if inDriver {
				sawDriverConstruction = true
				return
			}
			t.Errorf("%s: references NewEngine as a %s — an Engine's allowlist is chosen at construction, so only core/driver/** may construct one", pf.relPath, shape)
		}

		reportTypeUse := func(node ast.Node, kind, name string) {
			if inExemptBody(node) {
				sawExemptEngineLiteral = true
				return
			}
			t.Errorf("%s: %s %s outside NewEngine's own body — an Engine's allowlist is chosen at construction, so only NewEngine (called exclusively from core/driver/**) may build one", pf.relPath, kind, name)
		}

		// visit is a hand-rolled walk, not a bare ast.Inspect callback,
		// because *ast.SelectorExpr needs non-default recursion: its Sel
		// field is itself an *ast.Ident, and ast.Inspect's automatic
		// recursion would otherwise visit it a SECOND time as a "bare"
		// identifier once the case below has already scored it as a
		// qualified reference. Recursing manually into X only (never
		// Sel) avoids the double count while still finding matches
		// nested inside the receiver expression (e.g. a call embedded in
		// a chained selector, or an argument buried inside one).
		var visit func(ast.Node) bool
		visit = func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.SelectorExpr:
				if x.Sel.Name == "NewEngine" {
					reportReference("qualified selector")
				}
				ast.Inspect(x.X, visit)
				return false
			case *ast.Ident:
				if x.Name == "NewEngine" && x != exemptName {
					reportReference("bare identifier")
				}
			case *ast.CompositeLit:
				if !inTransportPkg {
					return true
				}
				if name := typeNameOf(x.Type); engineTypeNames[name] {
					reportTypeUse(x, "constructs a composite literal naming", name)
					return true
				}
				// []Engine{{...}}, [N]Engine{{...}} and map[K]Engine{...}:
				// the OUTER literal's type names a collection whose
				// element/value type is an engine type, but the INNER
				// element literals elide their own type entirely
				// (Codex e14) — resolved from context here, since
				// nothing about an elided-type CompositeLit node itself
				// says what it builds.
				if elemName, isCollection := collectionElementTypeName(x.Type); isCollection && engineTypeNames[elemName] {
					for _, elt := range x.Elts {
						v := elt
						if kv, ok := elt.(*ast.KeyValueExpr); ok {
							v = kv.Value
						}
						if inner, ok := v.(*ast.CompositeLit); ok && inner.Type == nil {
							reportTypeUse(inner, "constructs a composite literal (elided element type) naming", elemName)
						}
					}
				}
			case *ast.CallExpr:
				// new(Engine), or new(<alias for Engine>) (Codex e13):
				// zero-value construction whose result is a *Engine with
				// every field, including allow, directly assignable
				// afterwards from the SAME package.
				if inTransportPkg {
					if id, ok := x.Fun.(*ast.Ident); ok && id.Name == "new" && len(x.Args) == 1 {
						if name := typeNameOf(x.Args[0]); engineTypeNames[name] {
							reportTypeUse(x, "calls new() naming", name)
						}
					}
				}
				return true
			case *ast.IndexExpr:
				// A single generic type argument, e.g. zero[Engine]()
				// (Codex e13/e22).
				if inTransportPkg {
					if name := typeNameOf(x.Index); engineTypeNames[name] {
						reportTypeUse(x, "instantiates a generic naming", name)
					}
				}
				return true
			case *ast.IndexListExpr:
				// Multiple generic type arguments, e.g. f[Engine, int]().
				if inTransportPkg {
					for _, idx := range x.Indices {
						if name := typeNameOf(idx); engineTypeNames[name] {
							reportTypeUse(x, "instantiates a generic naming", name)
						}
					}
				}
				return true
			case *ast.ValueSpec:
				// var e Engine (Codex e20): a zero-value var declaration
				// is exactly as exploitable as new(Engine) — allow is
				// assignable afterwards, same package, same risk. A
				// pointer-typed spec (var e *Engine) is NOT matched here:
				// it declares a nil pointer, not a struct value, and
				// constructs nothing on its own (typeNameOf returns ""
				// for the enclosing *ast.StarExpr).
				if inTransportPkg && x.Type != nil {
					if name := typeNameOf(x.Type); engineTypeNames[name] {
						reportTypeUse(x, "declares a var naming", name)
					} else if elemName, isCollection := collectionElementTypeName(x.Type); isCollection && engineTypeNames[elemName] {
						reportTypeUse(x, "declares a var naming a collection of", elemName)
					}
				}
				return true
			}
			return true
		}
		ast.Inspect(pf.file, visit)
	}

	if scanned == 0 {
		t.Fatal("scanned no files — the walker or its filters are broken, and this check passed vacuously")
	}
	if !sawDriverConstruction {
		t.Error("never saw core/driver/** reference NewEngine — the walker or its filters are broken, and this check passed vacuously")
	}
	if !sawExemptEngineLiteral {
		t.Error("never saw NewEngine's own body construct an Engine value — the type-use matcher or its exemption bounds are broken, and that half of this check passed vacuously")
	}
}

// typeNameOf returns the bare type name a type expression spells —
// "Engine" for both `Engine` and (were it ever written from outside the
// package) `transport.Engine` — or "" for any other expression shape
// (e.g. *Engine, []Engine, a call, an index expression). Used both for
// composite-literal types and for the other type-position uses
// (new()'s argument, a generic type argument, a var declaration's type)
// TestNewEngineReachableOnlyFromDriver also checks.
func typeNameOf(t ast.Expr) string {
	switch x := t.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return x.Sel.Name
	default:
		return ""
	}
}

// collectionElementTypeName reports the element type name of an array,
// slice, or map type expression ([]T, [N]T, map[K]T -> "T") — or ok=false
// for any other type expression. Used to resolve []Engine{{...}} and
// map[K]Engine{...}-shaped composite literals, whose inner element
// literals elide their own type (Codex e14).
func collectionElementTypeName(t ast.Expr) (elemName string, ok bool) {
	switch x := t.(type) {
	case *ast.ArrayType:
		return typeNameOf(x.Elt), true
	case *ast.MapType:
		return typeNameOf(x.Value), true
	default:
		return "", false
	}
}
