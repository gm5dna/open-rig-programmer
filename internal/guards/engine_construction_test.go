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
// all, so no name-based matcher, however placed, can see it. Only
// package transport itself can write one (Engine's fields are
// unexported), so a SEPARATE check below flags any *ast.CompositeLit of
// type Engine found in core/transport outside NewEngine's own body
// (task-58 fix wave, Codex Important 1).
//
// APPROXIMATE, deliberately (see the package doc comment): a plain AST
// walk over parsed non-test source, not a type-checked or dataflow
// analysis. Concretely:
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
//   - new(Engine) or any other zero-value construction is not targeted:
//     Engine's own allow field would be nil, and both NewEngine
//     (ErrNoAllowlist) and Do itself refuse a nil AllowFunc, so a
//     zero-value Engine cannot be used to smuggle a PERMISSIVE gate past
//     this guard's threat model — it can only be permanently broken,
//     which is a correctness bug this guard does not claim to police.
//   - CONFIRMED FALSE POSITIVE, accepted: because the match is by name
//     alone, an entirely unrelated identifier that merely happens to be
//     named "NewEngine" elsewhere in the repository — with no
//     relationship at all to transport.NewEngine — would also be
//     flagged. Nothing in this repository does this today; if something
//     ever does, the failure prompts a human look at a genuinely unusual
//     name choice, the same trade this package's other guards
//     (WriteChannel, BuildMWSet/BuildMTSet) already accept for their own
//     name-only matches.
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
		// any other code — inExemptBody exists only to excuse the ONE
		// composite literal NewEngine's own body legitimately builds,
		// not to prune anything from the walk.
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

		reportReference := func(shape string) {
			if inDriver {
				sawDriverConstruction = true
				return
			}
			t.Errorf("%s: references NewEngine as a %s — an Engine's allowlist is chosen at construction, so only core/driver/** may construct one", pf.relPath, shape)
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
				if !inTransportPkg || compositeLitTypeName(x.Type) != "Engine" {
					return true
				}
				if inExemptBody(x) {
					sawExemptEngineLiteral = true
					return true
				}
				t.Errorf("%s: constructs an Engine{} composite literal outside NewEngine's own body — an Engine's allowlist is chosen at construction, so only NewEngine (called exclusively from core/driver/**) may build one", pf.relPath)
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
		t.Error("never saw NewEngine's own body construct an Engine{} literal — the composite-literal matcher or its exemption bounds are broken, and that half of this check passed vacuously")
	}
}

// compositeLitTypeName returns the bare type name a composite literal's
// Type expression spells — "Engine" for both `Engine{...}` and (were it
// ever written from outside the package) `transport.Engine{...}` — or ""
// if Type is nil (an elided type inside a nested literal, e.g. the inner
// literals of []Engine{{...}}, which this does not attempt to resolve;
// nothing in this repository writes Engine that way).
func compositeLitTypeName(t ast.Expr) string {
	switch x := t.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return x.Sel.Name
	default:
		return ""
	}
}
