// SPDX-License-Identifier: GPL-3.0-or-later

package guards

import (
	"go/ast"
	"go/token"
	"sort"
	"testing"
)

// This guard makes M9c-0's audit REPRODUCIBLE.
//
// The milestone's design document quotes a number — 44 package-level
// const/var reachable from a cat.Dialect method — with no committed way to
// re-derive it. That is precisely the failure the same document warns
// about: a claim written without a runnable mechanism behind it. Worse, the
// figure was WRONG twice before it was right, and both errors were in the
// measuring instrument rather than the code:
//
//  1. The first sweep reported modeNames as a global read by
//     Configured/ModeName/ValidMode. False positive: ast.Inspect visits
//     SelectorExpr.Sel as a bare Ident, so the FIELD d.modeNames read as the
//     package var of the same name. The walk below never descends into Sel,
//     for that reason.
//
//  2. The corrected sweep was then reported as exhaustive when it was
//     LEXICAL — it looked only at identifiers appearing directly inside
//     Dialect methods. The rule being tested is Dialect's own: "every method
//     here, and every helper those methods delegate to". A direct sweep
//     cannot test that rule at all. The walk below is transitive.
//
// WHAT IT ASSERTS, and what it deliberately does not.
//
// It does NOT assert that some identifier has vanished. KindMemory, for
// instance, remains legitimately reachable through validKindByte, which is
// an enum membership test and not a per-radio policy. Absence is the wrong
// question.
//
// WHAT IT ASSERTS, PRECISELY. For each gate-reaching validator: it is a
// Dialect method; its body contains a selector <recv>.<field>; it does not
// reference FT710 (or KindMemory, for validateMWFields); it does not assign
// to that field; it does not rebind the receiver's name; and it does not
// take the receiver's address.
//
// WHAT IT CANNOT ASSERT, MEASURED RATHER THAN GUESSED. This is an UNTYPED
// AST check: it matches a selector whose base identifier has the
// receiver's spelling. It cannot follow dataflow, so it cannot establish
// that the value the caller passed actually REACHES the decision. Probing
// it found two shapes that still pass, both confirmed to compile:
//
//	_ = d.mwWriteKind          // satisfies the selector requirement
//	if m.Kind != '1' { ... }   // decides on a literal anyway
//
//	k := d.mwWriteKind
//	_ = k
//	if m.Kind != '1' { ... }
//
// Closing those needs go/types plus dataflow analysis, which is a
// different instrument from this one. THE BEHAVIOURAL PEER-DIALECT TESTS
// CATCH BOTH — verified, not assumed — and they are the primary defence.
// This guard is a structural tripwire for the cheap regressions: a
// validator demoted to a package function, or one reaching for FT710.
//
// Stated this way deliberately. Two earlier versions of this comment
// claimed more than the code did, and each claim was falsified by the next
// review.

// promotedConstants are the package-level names M9c-0 moved onto the
// Dialect receiver. Each was read by a method through its receiver while
// the datum itself came from a package const — the milestone's recurring
// defect shape — and each reaches the OUTBOUND WRITE GATE.
var promotedConstants = []string{
	"mtTagMaxBytes", // bounded build, parse AND validMTCommand
	"mtClearTag",    // the empty-tag encoding, emitted into MT Set frames
	"clarMaxAbsHz",  // reached the gate through validateMWFields
	"clarStepHz",    // ditto, and a radio characteristic, not a field width
}

// gateReachingValidators must be Dialect METHODS, and each must READ the
// receiver field naming its policy.
//
// Both halves are required, and the second is the one that matters. Until
// the M9c-0 milestone review (finding 3) this guard checked only that the
// functions were methods — so reverting validateMWFields to the hardcoded
// KindMemory, while leaving it a method, left the guard GREEN. Mutation-
// tested and confirmed: the cat package's own tests caught that
// regression; this guard did not, while its doc comment and the milestone
// summary both claimed it "asserts usage rather than absence".
//
// That is this project's recurring defect — a plausible mechanism written
// without running it — committed inside the instrument built to detect it.
// The root cause is worth recording: task 66's fix WAS mutation-tested and
// the mutation WAS caught, and the catch was attributed to this guard
// without checking which test had gone red.
//
// Each is reached by Dialect.AllowedCommand, so a version that consults a
// package global binds every dialect to the FT-710's rule at the one point
// that decides what is written to a radio.
// The `forbidden` list is the second lesson, learned while fixing the
// first. Requiring the body to MENTION its receiver field is still too
// weak, because an incidental mention satisfies it: hardwiring
// validateMWFields' decision to KindMemory left `d.mwWriteKind` in the
// diagnostic string, and hardwiring validClarHz' arithmetic to FT710 left
// a `d.clar.StepHz < 1` guard at the top. Both mutations passed the
// mention check. Naming the identifiers that must NOT appear closes the
// two shapes a mention cannot distinguish.
var gateReachingValidators = []struct {
	fn        string
	field     string   // the receiver field its body must read
	forbidden []string // identifiers its body must NOT reference
}{
	// FT710 is forbidden in all three: a Dialect method reaching for the
	// package's one configured dialect is the defect by definition,
	// whatever else it also reads.
	{"validMTTag", "mt", []string{"FT710"}},
	{"validClarHz", "clar", []string{"FT710"}},
	// KindMemory too: after M9c-0 this validator has no business naming
	// the FT-710's own P7 value. Its diagnostic mentions it as a STRING,
	// which is not an identifier, so this stays satisfiable.
	{"validateMWFields", "mwWriteKind", []string{"FT710", "KindMemory"}},
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

// funcDeclInfo is one function or method declaration, with enough of it
// retained to ask whether its body reads a particular receiver field.
type funcDeclInfo struct {
	isMethod bool
	recvType string
	recvName string // "" for a package function or an unnamed receiver
	where    string
	body     *ast.BlockStmt
}

// TestGateReachingValidatorsTakeADialectReceiver is the usage half.
//
// A promoted constant could be deleted and the value hardcoded inline at
// the same site, which the test above would not see. This one requires the
// validators themselves to be methods — the shape that makes the datum come
// from the receiver in the first place.
func TestGateReachingValidatorsTakeADialectReceiver(t *testing.T) {
	files := catFiles(t)

	found := map[string][]funcDeclInfo{}

	for _, pf := range files {
		for _, d := range pf.file.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok {
				continue
			}
			rec := funcDeclInfo{where: pf.relPath, body: fd.Body}
			if fd.Recv != nil && len(fd.Recv.List) > 0 {
				rt := fd.Recv.List[0].Type
				if se, ok := rt.(*ast.StarExpr); ok {
					rt = se.X
				}
				if id, ok := rt.(*ast.Ident); ok {
					rec.isMethod = true
					rec.recvType = id.Name
				}
				// The receiver's NAME, needed to recognise reads of its
				// own fields. An unnamed receiver (func (Dialect) f())
				// cannot read one at all, and leaving recvName empty makes
				// readsRecvField report false, which is the correct answer.
				if names := fd.Recv.List[0].Names; len(names) > 0 {
					rec.recvName = names[0].Name
				}
			}
			found[fd.Name.Name] = append(found[fd.Name.Name], rec)
		}
	}

	// Methods declared with a POINTER receiver on Dialect. Calling one on
	// the value receiver — d.forceMWWriteKind() — makes Go implicitly take
	// &d, so it can rewrite the policy with no `&d` and no assignment
	// appearing anywhere in this body (re-review 4, finding 3).
	ptrMethods := map[string]bool{}
	for _, pf := range files {
		for _, dcl := range pf.file.Decls {
			fd, ok := dcl.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || len(fd.Recv.List) == 0 {
				continue
			}
			if se, ok := fd.Recv.List[0].Type.(*ast.StarExpr); ok {
				if id, ok := se.X.(*ast.Ident); ok && id.Name == "Dialect" {
					ptrMethods[fd.Name.Name] = true
				}
			}
		}
	}

	for _, want := range gateReachingValidators {
		decls, ok := found[want.fn]
		if !ok {
			t.Errorf("%s is not declared anywhere in core/cat — if it was renamed, update this guard deliberately rather than letting it pass vacuously", want.fn)
			continue
		}
		for _, d := range decls {
			if !d.isMethod {
				t.Errorf("%s at %s is a package-level function, want a method on Dialect — it is reached by AllowedCommand, so a package-level version binds every dialect to the FT-710's rule at the gate", want.fn, d.where)
				continue
			}
			if d.recvType != "Dialect" {
				t.Errorf("%s at %s has receiver %s, want Dialect", want.fn, d.where, d.recvType)
			}
			// THE ASSERTION THAT MAKES THIS A USAGE GUARD. Being a method
			// is a shape; reading the receiver's own policy field is the
			// substance. Without this, hardwiring the datum back to a
			// package global or an inline literal leaves the guard green,
			// which is exactly what it did before finding 3.
			if !d.readsRecvField(want.field) {
				t.Errorf("%s at %s is a Dialect method but never reads %s.%s — it takes a receiver and gets its datum somewhere else, which is the shape of a seam with none of the substance",
					want.fn, d.where, d.recvName, want.field)
			}
			for _, bad := range want.forbidden {
				if d.referencesIdent(bad) {
					t.Errorf("%s at %s references %s — a gate-reaching validator must decide from its own receiver, not from the package's configured dialect or the FT-710's own constants",
						want.fn, d.where, bad)
				}
			}
			// A READ, not a write. Requiring the selector to appear is
			// satisfied by OVERWRITING it first: `d.mwWriteKind = '1'`
			// followed by a read of d.mwWriteKind passes every clause
			// above while substituting the FT-710's value for whatever the
			// caller configured. Confirmed by mutation — the guard stayed
			// green and only the behavioural peer tests caught it (fix-wave
			// re-review, finding 3 still open).
			if d.assignsRecvField(want.field) {
				t.Errorf("%s at %s ASSIGNS to %s.%s — the incoming policy is overwritten, so reading it back proves nothing about the dialect the caller built",
					want.fn, d.where, d.recvName, want.field)
			}
			// And the receiver's name must not be rebound: a local `d`
			// shadowing the receiver makes every `d.field` in scope a read
			// of something else entirely.
			if d.shadowsReceiver() {
				t.Errorf("%s at %s rebinds %q, shadowing its own receiver — selectors on it no longer refer to the dialect this method was called on",
					want.fn, d.where, d.recvName)
			}
			// Taking the address of the WHOLE receiver aliases it, and a
			// write through that pointer reaches the field without any
			// selector on the receiver appearing on an assignment's left
			// side. Found by probing this guard rather than by review.
			if d.takesAddressOfReceiver() {
				t.Errorf("%s at %s takes the address of its receiver %q — a write through that pointer changes the policy without any assignment to %s.%s appearing",
					want.fn, d.where, d.recvName, d.recvName, want.field)
			}
			if m := d.callsPointerMethodOnReceiver(ptrMethods); m != "" {
				t.Errorf("%s at %s calls pointer-receiver method %s on %q — Go takes &%s implicitly, so that call can rewrite the policy with no address-of and no assignment visible here",
					want.fn, d.where, m, d.recvName, d.recvName)
			}
		}
	}
}

// readsRecvField reports whether the declaration's body contains a
// selector on its own receiver naming field.
//
// It matches `<recv>.<field>` at any depth, including through further
// selectors such as d.clar.StepHz, by looking for a SelectorExpr whose X
// is the receiver identifier and whose Sel is the field. A method that
// reads the field only via a helper it delegates to will NOT satisfy this
// — deliberately: the three validators named here are the ones the gate
// reaches directly, and the point is that each consults its own receiver.
// callsPointerMethodOnReceiver reports the name of any pointer-receiver
// Dialect method the body invokes on its own receiver, or "".
//
// Go inserts the address-of automatically for such a call, so it is a
// write path that leaves no syntactic trace of taking an address.
func (d funcDeclInfo) callsPointerMethodOnReceiver(ptrMethods map[string]bool) string {
	if d.recvName == "" || d.body == nil || len(ptrMethods) == 0 {
		return ""
	}
	name := ""
	ast.Inspect(d.body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		se, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		id, ok := se.X.(*ast.Ident)
		if ok && id.Name == d.recvName && ptrMethods[se.Sel.Name] {
			name = se.Sel.Name
		}
		return true
	})
	return name
}

// takesAddressOfReceiver reports whether the body evaluates &<recv>.
func (d funcDeclInfo) takesAddressOfReceiver() bool {
	if d.recvName == "" || d.body == nil {
		return false
	}
	found := false
	ast.Inspect(d.body, func(n ast.Node) bool {
		ue, ok := n.(*ast.UnaryExpr)
		if !ok || ue.Op != token.AND {
			return true
		}
		if id, ok := ue.X.(*ast.Ident); ok && id.Name == d.recvName {
			found = true
		}
		return true
	})
	return found
}

// assignsRecvField reports whether the body assigns to <recv>.<field>,
// including compound assignment and taking its address.
func (d funcDeclInfo) assignsRecvField(field string) bool {
	if d.recvName == "" || d.body == nil {
		return false
	}
	// Matches <recv>.<field> AND anything rooted at it, so a nested write
	// like `d.clar.StepHz = 10` counts. Requiring the LHS to be exactly
	// `d.clar` missed that entirely, while the inner `d.clar` still
	// satisfied the read check — the policy overwritten and the guard
	// green (re-review 4, finding 3). This needs no type information; it
	// was simply the wrong shape to match on.
	var isTarget func(ast.Expr) bool
	isTarget = func(e ast.Expr) bool {
		se, ok := e.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		if id, ok := se.X.(*ast.Ident); ok {
			return id.Name == d.recvName && se.Sel.Name == field
		}
		return isTarget(se.X) // d.field.inner = ...
	}
	found := false
	ast.Inspect(d.body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range x.Lhs {
				if isTarget(lhs) {
					found = true
				}
			}
		case *ast.IncDecStmt:
			if isTarget(x.X) {
				found = true
			}
		case *ast.UnaryExpr:
			// &d.field hands a mutable pointer to something else.
			if x.Op == token.AND && isTarget(x.X) {
				found = true
			}
		}
		return true
	})
	return found
}

// shadowsReceiver reports whether the body rebinds the receiver's name,
// by declaration OR by plain assignment.
//
// Covers `d := ...`, `d = ...`, `var d ...`, `for d := range`, `for d =
// range`, and a function-literal parameter or named result called d. It
// does not attempt full scope analysis — see the guard's stated limits.
func (d funcDeclInfo) shadowsReceiver() bool {
	if d.recvName == "" || d.body == nil {
		return false
	}
	found := false
	names := func(idents []*ast.Ident) {
		for _, id := range idents {
			if id != nil && id.Name == d.recvName {
				found = true
			}
		}
	}
	ast.Inspect(d.body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			// Both DEFINE (`d := ...`) and plain assignment (`d = ...`).
			// Restricting this to declaring forms let a whole-receiver
			// overwrite through: `r := d; r.field = '1'; d = r` rebinds the
			// policy without any assignment to d.field appearing
			// (fix-wave re-review 3, finding 3).
			for _, lhs := range x.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && id.Name == d.recvName {
					found = true
				}
			}
		case *ast.ValueSpec:
			names(x.Names)
		case *ast.RangeStmt:
			for _, ex := range []ast.Expr{x.Key, x.Value} {
				if id, ok := ex.(*ast.Ident); ok && id.Name == d.recvName {
					found = true
				}
			}
		case *ast.FuncLit:
			if x.Type.Params != nil {
				for _, f := range x.Type.Params.List {
					names(f.Names)
				}
			}
			if x.Type.Results != nil {
				for _, f := range x.Type.Results.List {
					names(f.Names)
				}
			}
		}
		return true
	})
	return found
}

// referencesIdent reports whether the body names ident anywhere, INCLUDING
// as the Sel of a selector, so both `FT710` and `FT710.clar.StepHz` match.
//
// String literals do not match: a diagnostic that mentions "KindMemory" in
// prose is documentation, not a decision, and forbidding that would make
// the honest error messages this milestone added unwritable.
func (d funcDeclInfo) referencesIdent(ident string) bool {
	if d.body == nil {
		return false
	}
	found := false
	ast.Inspect(d.body, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == ident {
			found = true
		}
		return true
	})
	return found
}

func (d funcDeclInfo) readsRecvField(field string) bool {
	if d.recvName == "" || d.body == nil {
		return false
	}
	found := false
	ast.Inspect(d.body, func(n ast.Node) bool {
		se, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if id, ok := se.X.(*ast.Ident); ok && id.Name == d.recvName && se.Sel.Name == field {
			found = true
		}
		return true
	})
	return found
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
