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
// *ast.CallExpr.Fun (task-58 fix wave, Codex Critical 1): a local alias,
// a package-level var, a struct field holding the constructor, a
// higher-order argument, generic instantiation, or an init()-time
// indirect call are all a *ast.SelectorExpr or *ast.Ident naming
// NewEngine even though no CallExpr.Fun ever names it directly.
//
// core/transport is SCANNED, not skipped: only NewEngine's own top-level,
// non-method declaration is exempt — its Name identifier specifically
// (task-58 fix wave, Codex Critical 2), and (from fix round 3) its whole
// signature and body by POSITION, so a legitimate Engine-typed parameter,
// result, or local construction inside NewEngine itself is never
// mistaken for a violation. A same-named method borrows none of this: its
// own Name and body are walked exactly like any other code.
//
// A composite literal never mentions "NewEngine" at all, so no
// name-based matcher can see it (task-58 fix wave, Codex Important 1).
// Fix round 2 (Codex e13/e22, runtime-proven: a frame reached the wire)
// found that name-matching alone was not enough EVEN for the
// construction site: inside package transport, Engine's allow field is
// an ordinary unexported field, freely assignable after new(Engine),
// after a zero-value var declaration, or after starting readLoop by
// hand — none of which mention "NewEngine" or, in some shapes, even
// "Engine" as anything other than a TYPE.
//
// Fix round 3 (Codex re-review) found that round 2's response —
// composite-literal/new()/generic/var-decl checks written as separate,
// ad hoc cases — was still an enumeration of INSTANCES, not the CLASS:
// each new probe shape (a pointer or collection wrapping Engine, a
// cross-file alias, make(), a struct field, a named function result)
// sat one small step outside what had been checked. The fix is a single
// general type resolver, applied at every AST position Go's grammar
// allows a type expression to appear in a construction-adjacent
// position:
//   - typeBaseName recurses through *T, []T, [N]T, map[K]T, chan T and
//     nested combinations of these down to the innermost named type —
//     so []Engine, []*Engine, [N]Engine, map[K]*Engine and chan Engine
//     all resolve to "Engine" — and is used wherever a type expression
//     ALWAYS represents genuine, immediate allocation of the value(s)
//     it names: a composite literal's type (whatever pointer/collection
//     wrapping it carries — Codex e02, e07, e08), a new()/make()
//     argument (Codex e03), and a generic type argument.
//   - declaredTypeBaseName is its DELIBERATELY NARROWER sibling for
//     positions that merely DECLARE a type with no accompanying
//     literal — a var/const spec, a struct field (named OR embedded),
//     a function parameter or named result (*ast.ValueSpec.Type and
//     *ast.Field.Type are the SAME shape of problem: Codex e04, e05).
//     A bare pointer, slice, map or channel of Engine allocates NOTHING
//     at the point of declaration (a nil pointer, nil slice, nil map,
//     nil channel — no Engine struct exists yet); only a direct name,
//     or a FIXED-SIZE array of a direct name, allocates real storage
//     inline. Applying typeBaseName's full unwrap here would
//     false-positive on the overwhelmingly common, harmless
//     `*Engine` parameter or field every driver legitimately holds
//     (including, notably, every one of Engine's own pointer-receiver
//     methods and every Option func in this very file).
//   - engineTypeNames resolves `type X = Engine` aliases PACKAGE-WIDE,
//     not per file (Codex e01: an earlier version of this comment said
//     "declared in the SAME FILE", which was itself the gap — an alias
//     declared in one core/transport file and used in another evaded
//     it), chased to a fixed point so a chain (type A = Engine; type
//     B = A) resolves too. Only TRUE aliases (Go's "=" form,
//     ts.Assign.IsValid()) qualify: a DEFINED type (`type X Engine`, no
//     "="), which needs its own explicit conversion and genuine type
//     information to resolve safely, is a different, unexamined evasion
//     class — see the APPROXIMATE section's entry for it (Codex e06),
//     not merely a dangling reference to one.
//
// APPROXIMATE, deliberately (see the package doc comment): a plain AST
// walk over parsed non-test source, not a type-checked or dataflow
// analysis. The package doc comment's own framing matters here: these
// guards pin THIS REPOSITORY'S OWN COMPOSITION, not a determined
// adversary. The list below is the KNOWN set of gaps, found across four
// Codex re-review rounds, each of which found at least one shape the
// previous round's list did not name (round 4 found the value-copy
// class below, which rounds 1-3 missed entirely) — it is offered as the
// known set, not claimed as the complete one, and a future round would
// not be surprising. Concretely:
//
//   - SEVERITY CONTEXT, which belongs here because it reframes the
//     whole type-use family above: every runtime proof in fix rounds 2
//     and 3 (Codex e13/e22, e18) EXPLICITLY set a permissive allow func.
//     An Engine reached through any of the CONSTRUCTION shapes this
//     guard's type-use rule targets, with allow left at its zero value,
//     is FAIL-SAFE, not fail-open — Do returns ErrNoAllowlist and
//     writes nothing (see NewEngine's own doc comment and Do's
//     nil-check). So the risk a genuinely accidental refactor — a
//     struct gaining an Engine field, a helper minting one via make()
//     for a slice, a named result — actually carries on its own is a
//     NIL gate, which refuses. Reaching the wire through any of THOSE
//     shapes requires DELIBERATELY assigning a permissive AllowFunc
//     afterwards, a different, more deliberate act than the
//     accidental-refactor framing below.
//
//     ONE CLASS IS THE EXCEPTION, and it matters (Codex fix-round-4):
//     value-copy construction (see the GAP entry below) does NOT start
//     from a zero value at all — c := *e copies whatever gate the
//     ORIGINAL Engine already had, non-nil included. An accidental
//     "widen a copy, forget to re-gate it" refactor of that shape is
//     NOT fail-safe: it silently inherits a working, already-permissive
//     gate. This is the one place in this family where "accidental"
//     does not imply "refused".
//
//     None of this excuses leaving any class open — a nil-gated Engine
//     escaping into production is still a live bug this guard should
//     catch (and does, for every class it closes), and a
//     deliberately-gated one is strictly worse. It only corrects the
//     record on what an accidental hit, by itself, would actually do —
//     which, for every class except value-copy, is nothing.
//
//   - Test files are excluded — parseRepo skips every *_test.go file
//     entirely (see its own doc comment). Deliberate, not an oversight:
//     test doubles, table-driven fixtures, and this task's own probe
//     files legitimately construct engines outside core/driver, and
//     policing them here would only get in the way of testing
//     core/transport itself.
//
//   - The FuncDecl exemption is name-AND-shape-narrow: only fd.Recv ==
//     nil, fd.Name.Name == "NewEngine", declared directly in
//     core/transport (relDir == "core/transport" exactly — Engine's
//     fields are unexported, so nothing outside that exact package could
//     compile a composite literal of it regardless; a hypothetical
//     core/transport subpackage is a DIFFERENT package under Go's rules
//     and gets no exemption). The exempt POSITION RANGE is the whole
//     declaration (fd.Pos() to fd.End() — signature and body), not only
//     the body: NewEngine's own return type names *Engine, and a future
//     signature change naming Engine directly (e.g. a named result)
//     must not be mistaken for a violation of its own declaration.
//
//   - CONFIRMED FALSE POSITIVE, accepted, precisely stated (Codex
//     fix-round-4 corrected an earlier over-broad version of this
//     entry, which claimed an unrelated "Engine" type could collide —
//     it cannot: the type-use rule is core/transport-scoped, and Go
//     forbids two same-package types sharing a name, so no unrelated
//     "Engine" type can exist there to be confused with the real one).
//     The genuine false positive is the "NewEngine" bare-identifier/
//     selector match, which IS repo-wide by design: an entirely
//     unrelated function or var named "NewEngine" anywhere in the
//     repository, with no relationship to transport's own symbol, is
//     flagged (demonstrated directly by this task's own probes). A
//     SEPARATE, smaller false positive comes from engineTypeNames
//     itself: because the package-wide alias fixed point resolves
//     through typeBaseName's FULL unwrap, a pointer or collection alias
//     — type P = *Engine, type S = []Engine — is added to
//     engineTypeNames exactly as a direct alias would be, and
//     declaredTypeBaseName then matches "P" or "S" directly at a var or
//     field declaration (var e P) even though P and S themselves
//     allocate nothing on their own (P is a nil pointer, S a nil slice)
//     — confirmed by construction. Both are accepted, not fixed: each
//     failure prompts a human look at a genuinely unusual construct,
//     the same trade this package's other guards (WriteChannel,
//     BuildMWSet/BuildMTSet) already accept for their own name-only
//     matches.
//
//   - GAP, not closed — a driver-tree re-export (Codex e15). A package
//     under core/driver/** that does nothing but forward NewEngine's
//     own parameters straight through — e.g.
//     func Open(p transport.Port, allow transport.AllowFunc)
//     (*transport.Engine, error) { return transport.NewEngine(p, allow) }
//     — satisfies inTree(pf.relDir, "core/driver") and is treated as a
//     legitimate driver-tree construction, yet the re-export's own
//     caller chooses the gate. This guard pins WHERE the identifier
//     NewEngine textually appears, not WHO actually chooses the gate
//     through however many layers of forwarding. THE ACTUAL PERIMETER,
//     found by construction, not assumed: from app/ or cmd/rigprog, a
//     call through such a re-export IS caught, but by a DIFFERENT
//     guard, not this one — TestCompositionRootImportDiscipline (Rule
//     1) refuses ANY import of a concrete core/driver/** package from
//     outside core/** unless the importer is internal/wiring, so
//     `app/probeXXX.go: imports concrete driver ".../core/driver/relay"`
//     fires regardless of what this guard sees. The shape escapes ALL
//     FIVE guards only when the caller of the re-export sits under
//     core/** itself (Rule 1 does not apply there at all) or under
//     internal/wiring (Rule 1's own sanctioned exception) — narrower
//     than fix round 2's documentation claimed, verified by building
//     both variants and running the whole guard suite against each. A
//     thin public/internal constructor split reachable ONLY from inside
//     core/** or internal/wiring is a plausible ACCIDENTAL refactor, not
//     deliberate subversion, and sits inside this package's own
//     composition threat model — recorded as a live gap, not dismissed
//     as out of scope, because closing it would need a call-graph
//     analysis this guard does not attempt.
//
//   - GAP, not closed — field mutation after construction (Codex e19,
//     widened at fix round 4). Every guard in this package pins
//     CONSTRUCTION sites, not the field afterwards: nothing stops a
//     future EXPORTED SYMBOL of any kind — not only a method — assigning
//     to Engine.allow from being added to core/transport.
//     func (e *Engine) SetAllow(AllowFunc) is one shape; the SHARPER one
//     is an Option func, since NewEngine applies every opt AFTER setting
//     e.allow from its own allow parameter (see NewEngine's body): a
//     hypothetical func WithAllow(a AllowFunc) Option { return func(e
//     *Engine) { e.allow = a } } would let WithAllow(permissive)
//     override the gate from INSIDE an otherwise fully compliant
//     NewEngine(p, restrictive, WithAllow(permissive)) call — one this
//     guard's own non-vacuity counter would count as the sanctioned
//     core/driver/** construction, since it genuinely is one. Engine.
//     allow's own doc comment (core/transport/engine.go:249) already
//     asserts it is "immutable after construction… so Do reads it
//     without synchronisation" — a convention nothing in the type
//     system or this guard enforces, and whose breach would ALSO be a
//     data race. A plausible accidental addition, inside this package's
//     own threat model, recorded as a gap because pinning it needs a
//     second guard against any exported symbol assigning to
//     Engine.allow, which this file does not have.
//
//   - GAP, not closed — a defined (non-alias) type sharing Engine's
//     layout (Codex e06): `type shadowEngine Engine` (no "=") is a
//     DISTINCT type under Go's rules, not caught by engineTypeNames'
//     alias resolution (deliberately: resolving it would need to know
//     shadowEngine and Engine share an underlying type, which is a
//     type-identity question, not a syntactic one). Constructing
//     `shadowEngine{allow: permissive}` is NOT itself a *Engine — it
//     only becomes one via an explicit conversion, `(*Engine)(&s)`,
//     which DOES textually use "Engine" as a type (inside a ParenExpr
//     wrapping a StarExpr) and could in principle be added to the
//     type-use rule's positions — but doing so soundly, without also
//     flagging every UNRELATED pointer conversion in the file (Go
//     allows converting between any two types with identical underlying
//     types, so a bare AST walk cannot tell "(*Engine)(&s)" apart from
//     any other same-shaped conversion without also confirming s's
//     static type), needs actual type information. This guard is a
//     go/ast walk, not a go/types-checked one (see the package doc
//     comment); closing this properly would need the latter. Recorded
//     as a gap, not chased with another syntactic special case.
//
//   - GAP, not closed — a Go package beneath app/frontend (Codex e09).
//     parseRepo's app/frontend skip (fixed to an exact PATH match at
//     task-58 fix round 2, closing a different, broader blind spot —
//     see parseRepo's own doc comment) is a `filepath.SkipDir` on the
//     WHOLE app/frontend subtree, which is correct for its purpose (a
//     large generated Svelte/JS/TS tree with its own node_modules) but
//     has a residual cost: a real Go package placed beneath
//     app/frontend (e.g. app/frontend/gobridge) — visible to `go list`
//     — is skipped along with everything else there, invisible to all
//     five guards in this package, not only this one. Recorded as a
//     gap rather than special-cased, because distinguishing "this
//     subdirectory of app/frontend holds Go source" from "this one
//     holds only the generated frontend" by path alone is exactly the
//     kind of narrow, brittle carve-out this package's guards otherwise
//     avoid.
//
//   - OUT OF THIS GUARD'S THREAT MODEL — //go:linkname (Codex e17). A
//     //go:linkname directive can reach the real constructor with the
//     name "NewEngine" appearing only inside a comment pragma;
//     parser.ParseFile runs with mode 0 here (see parseRepo), so
//     comment text is never in the AST this guard walks at all,
//     regardless of what the comment says. Deliberate subversion of the
//     toolchain, not an accidental composition mistake — the same
//     carve-out the package doc comment already states for a third
//     party hand-building a wire frame.
//
//   - OUT OF THIS GUARD'S THREAT MODEL — reflect + unsafe (Codex e18,
//     confirmed at runtime: a frame reached the wire).
//     reflect.New(reflect.TypeOf((*transport.Engine)(nil)).Elem()) MINTS
//     a fresh Engine from OUTSIDE the package. (*transport.Engine)(nil)
//     genuinely IS a type-position use of Engine, textually — but the
//     TRUE reason this guard cannot see it is NOT scope (an earlier
//     version of this entry wrongly blamed "scoped to core/transport
//     ONLY"; fix round 4 disproved that by construction: the IDENTICAL
//     expression, moved INSIDE core/transport —
//     func zzP27() reflect.Type { return reflect.TypeOf((*Engine)(nil)).Elem() }
//     — is STILL not flagged). The real reason is that a type
//     CONVERSION — (*Engine)(x), parsed as CallExpr{Fun:
//     ParenExpr{StarExpr{...}}} — is not a checked position anywhere in
//     this guard, at any scope: the *ast.CallExpr case above inspects
//     x.Fun.(*ast.Ident) only for "new"/"make", and a ParenExpr is never
//     an *ast.Ident. This is the SAME unchecked position the e06 entry
//     above already names, for the same underlying reason (soundly
//     recognising a conversion as "this names Engine" needs to rule out
//     every OTHER, harmless same-shaped conversion, which needs real
//     type information this go/ast walk does not have). Even were
//     conversions added as a checked position, it would not fully close
//     THIS shape regardless: reflect.New and reflect.NewAt take a
//     runtime reflect.Type/reflect.Value, not source-visible "Engine"
//     syntax at their OWN call site, so the actual allocation and field
//     write stay fundamentally opaque to any AST-level check. reflect.
//     NewAt over an unsafe.Pointer then sets allow directly, bypassing
//     Go's own visibility rules entirely — deliberate subversion,
//     outside "our own composition" regardless of any of the above, the
//     same threat-model boundary the package doc comment already draws.
//
//   - GAP, not closed — value-copy construction (Codex fix-round-4
//     re-review, a class rounds 1-3 missed entirely).
//     func widen(e *Engine) *Engine { c := *e; c.allow = permissive;
//     return &c } — and the same class via append(s, *e), a v.(Engine)
//     type assertion, or a case Engine: type-switch arm — contains NO
//     Engine type-position use and NO "NewEngine" mention at all: only
//     *e, a plain dereference of an already-legitimately-obtained
//     *Engine. Confirmed by construction: not flagged by this guard.
//     No matcher this guard could add at the AST-shape level closes
//     this without effectively re-deriving a borrow-checker, since the
//     shape is indistinguishable, syntactically, from copying any other
//     ordinary struct. MEDIUM, not HIGH, for a reason worth stating
//     precisely: go vet's copylocks check ALREADY flags every one of
//     these ("assignment copies lock value... contains sync.Mutex"),
//     because Engine embeds sync.Mutex — confirmed by running go vet
//     against this exact shape. go vet ./... is both a CI step and this
//     repository's documented local gate, so the class is not silent in
//     the TOOLCHAIN, only in this ONE guard. It is, however, the ONE
//     class in this file whose ACCIDENTAL form is not fail-safe (see
//     the SEVERITY CONTEXT entry above): a copy inherits the original's
//     already-non-nil allow, so an accidental "c := *e" yields a
//     working, permissively-gated Engine on the spot — not a nil-gated
//     one Do would refuse.
//
//   - scanned == 0 can no longer actually fire in practice: parseRepo
//     itself already t.Fatals on an empty walk. Kept as defence-in-depth
//     alongside the two sawX counters below, not because it is
//     load-bearing on its own.
func TestNewEngineReachableOnlyFromDriver(t *testing.T) {
	files := parseRepo(t)

	// engineTypeNames resolves every PACKAGE-WIDE name that denotes the
	// Engine type: "Engine" itself, plus any `type X = Engine` alias
	// declared ANYWHERE in core/transport (Codex fix-round-3 e01 — an
	// earlier version resolved aliases per file, which is exactly the
	// gap a cross-file alias walks through), chased to a fixed point so
	// a chain (type A = Engine; type B = A) resolves too, however many
	// files or declarations apart. Only true aliases (ts.Assign.IsValid())
	// qualify — see the doc comment above for why a defined type is a
	// deliberately different, undocumented-as-closed case.
	engineTypeNames := map[string]bool{"Engine": true}
	for changed := true; changed; {
		changed = false
		for _, pf := range files {
			if pf.relDir != "core/transport" {
				continue
			}
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
					name := typeBaseName(ts.Type)
					if name != "" && engineTypeNames[name] && !engineTypeNames[ts.Name.Name] {
						engineTypeNames[ts.Name.Name] = true
						changed = true
					}
				}
			}
		}
	}

	sawDriverConstruction := false
	sawExemptEngineLiteral := false
	scanned := 0

	for _, pf := range files {
		scanned++
		inDriver := inTree(pf.relDir, "core/driver")
		inTransportPkg := pf.relDir == "core/transport"

		// The exempt declaration: NewEngine's own top-level (non-method)
		// FuncDecl, declared directly in core/transport. exemptName
		// excuses only that one Name identifier from the bare-identifier
		// check below; inExemptDecl excuses the declaration's WHOLE
		// span — signature (so NewEngine's own *Engine result and any
		// Engine-typed parameter are never mistaken for a violation of
		// themselves) and body (so the one legitimate Engine construction
		// inside it is excused) — by position, not by pruning: nothing
		// about this stops the walk from continuing, it only excuses
		// nodes that fall within the range.
		var exemptName *ast.Ident
		inExemptDecl := func(ast.Node) bool { return false }
		if inTransportPkg {
			for _, decl := range pf.file.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok || fd.Recv != nil || fd.Name.Name != "NewEngine" {
					continue
				}
				exemptName = fd.Name
				start, end := fd.Pos(), fd.End()
				inExemptDecl = func(n ast.Node) bool {
					return n.Pos() >= start && n.End() <= end
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

		reportTypeUse := func(node ast.Node, kind, name string) {
			if inExemptDecl(node) {
				sawExemptEngineLiteral = true
				return
			}
			t.Errorf("%s: %s %s outside NewEngine's own declaration — an Engine's allowlist is chosen at construction, so only NewEngine (called exclusively from core/driver/**) may build one", pf.relPath, kind, name)
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
				// Any composite literal whose type resolves — through
				// any pointer/array/slice/map/chan wrapping — to an
				// engine type name is a real allocation, whether or not
				// it has any elements: [1]Engine{} allocates one
				// zero-value Engine with an EMPTY Elts list (Codex e07),
				// exactly as much as []Engine{{allow: x}} or
				// []*Engine{{allow: x}} (Codex e02) or
				// map[string]*Engine{...} (Codex e08) do with theirs —
				// so the check is on x.Type alone, not on iterating
				// Elts for an elided-type inner literal.
				if inTransportPkg && x.Type != nil {
					if name := typeBaseName(x.Type); engineTypeNames[name] {
						reportTypeUse(x, "constructs a composite literal naming", name)
					}
				}
			case *ast.CallExpr:
				// new(Engine) (Codex e13) and make([]Engine, n) /
				// make(map[K]Engine) / make(chan Engine) (Codex e03):
				// both take a TYPE as their first argument, and both
				// allocate real storage for it immediately.
				if inTransportPkg {
					if id, ok := x.Fun.(*ast.Ident); ok && (id.Name == "new" || id.Name == "make") && len(x.Args) >= 1 {
						if name := typeBaseName(x.Args[0]); engineTypeNames[name] {
							reportTypeUse(x, "calls "+id.Name+"() naming", name)
						}
					}
				}
				return true
			case *ast.IndexExpr:
				// A single generic type argument, e.g. zero[Engine]()
				// or zero[*Engine]().
				if inTransportPkg {
					if name := typeBaseName(x.Index); engineTypeNames[name] {
						reportTypeUse(x, "instantiates a generic naming", name)
					}
				}
				return true
			case *ast.IndexListExpr:
				// Multiple generic type arguments, e.g. f[Engine, int]().
				if inTransportPkg {
					for _, idx := range x.Indices {
						if name := typeBaseName(idx); engineTypeNames[name] {
							reportTypeUse(x, "instantiates a generic naming", name)
						}
					}
				}
				return true
			case *ast.ValueSpec:
				// var e Engine, var arr [N]Engine: a zero-value
				// declaration allocates real storage inline, exactly as
				// new(Engine) does. Deliberately the NARROW resolver —
				// var e *Engine (a nil pointer, no struct exists) is
				// NOT matched; see declaredTypeBaseName's doc comment.
				if inTransportPkg && x.Type != nil {
					if name := declaredTypeBaseName(x.Type); engineTypeNames[name] {
						reportTypeUse(x, "declares a var naming", name)
					}
				}
				return true
			case *ast.Field:
				// A struct field (named or embedded), a function
				// parameter, or a named function result — Codex e04
				// (type engineHolder struct{ inner Engine }, and an
				// embedded struct{ Engine }) and e05
				// (func buildNamed(p Port) (e Engine)) escaped the
				// round-2 guard entirely because struct fields and
				// function results are *ast.Field, never *ast.ValueSpec.
				// Same narrow resolver as ValueSpec, for the same
				// reason: a *Engine field or parameter — the shape
				// every Option func and every one of Engine's own
				// pointer-receiver methods in this very package uses —
				// allocates nothing and must not be flagged.
				if inTransportPkg && x.Type != nil {
					if name := declaredTypeBaseName(x.Type); engineTypeNames[name] {
						reportTypeUse(x, "declares a field naming", name)
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
		t.Error("never saw NewEngine's own declaration construct an Engine value — the type-use matcher or its exemption bounds are broken, and that half of this check passed vacuously")
	}
}

// typeBaseName recurses through a type expression's pointer, array,
// slice, map and channel wrapping down to the innermost named type —
// "Engine" for Engine, *Engine, []Engine, []*Engine, [N]Engine,
// map[K]Engine, map[K]*Engine, chan Engine, and any nesting of these —
// or "" for anything else (a call, an index expression, a struct type
// literal, etc.). Used wherever a type expression ALWAYS represents
// genuine, immediate allocation of the value(s) it names: a composite
// literal's type, a new()/make() argument, and a generic type argument.
// See declaredTypeBaseName for the narrower resolver used where a type
// merely DECLARES without allocating.
func typeBaseName(t ast.Expr) string {
	switch x := t.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return x.Sel.Name
	case *ast.StarExpr:
		return typeBaseName(x.X)
	case *ast.ArrayType:
		return typeBaseName(x.Elt)
	case *ast.MapType:
		return typeBaseName(x.Value)
	case *ast.ChanType:
		return typeBaseName(x.Value)
	case *ast.ParenExpr:
		return typeBaseName(x.X)
	default:
		return ""
	}
}

// declaredTypeBaseName is typeBaseName's deliberately NARROWER sibling
// for positions that merely declare a type with no accompanying literal
// — a var/const spec, a struct field, a function parameter or result
// (*ast.ValueSpec.Type and *ast.Field.Type). A bare pointer, slice, map
// or channel of Engine allocates NOTHING at the declaration itself (a
// nil pointer, nil slice, nil map, nil channel — no Engine struct exists
// yet); only a direct name, or a FIXED-SIZE array (Len != nil — a slice
// has Len == nil and is nil, empty, until made or filled) of a direct
// (non-pointer) name, allocates real storage inline. Applying
// typeBaseName's full unwrap here would false-positive on the
// overwhelmingly common, harmless `*Engine` parameter or field every
// driver legitimately holds.
func declaredTypeBaseName(t ast.Expr) string {
	switch x := t.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return x.Sel.Name
	case *ast.ParenExpr:
		return declaredTypeBaseName(x.X)
	case *ast.ArrayType:
		if x.Len == nil {
			return "" // a slice: nil, no backing array, allocates nothing
		}
		switch x.Elt.(type) {
		case *ast.Ident, *ast.SelectorExpr:
			return typeBaseName(x.Elt) // a fixed array of a DIRECT name allocates inline
		default:
			return "" // e.g. [N]*Engine: N nil pointers, not N structs
		}
	default:
		return "" // *T, map[K]T, chan T: declares, does not allocate
	}
}
