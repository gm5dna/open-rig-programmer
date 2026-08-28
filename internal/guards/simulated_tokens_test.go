// SPDX-License-Identifier: GPL-3.0-or-later

package guards

import (
	"go/ast"
	"strings"
	"testing"
)

// TestSimulatedProfileTokensConfinement is the DATA-DRIVEN, N-driver
// generalisation of the single-driver guard
// TestSimulatedTokenSingleNonTestFileRepoWide that used to live in
// importgraph_test.go — retired at Task 58 once its pin lifted (see the
// PIN-LIFT LEDGER NOTE below). For every concrete driver listed in
// simulatedProfiles it pins the same structural-exclusivity invariant
// (task-11 brief §3) that the retired single-driver guard pinned for
// ft710 alone: the driver's simulated-profile
// selector — e.g. ft710.Simulated — may appear in exactly ONE non-test
// .go file across the whole repository; that one file must live in
// internal/wiring (the shared fake-wiring home since task-15's
// extraction); and it must ALSO call the driver's fake constructor
// (fakeradio.New). Together these keep the RealHardware/fakeradio and
// Simulated/real-port pairings structurally unrepresentable rather than
// merely conventionally so — the failure this guard exists to catch is a
// second, stray reference to a simulated-profile constant leaking the
// fake-only path into production wiring.
//
// PIN-LIFT LEDGER NOTE. This guard ran in parallel with the pinned
// single-driver TestSimulatedTokenSingleNonTestFileRepoWide, which the
// M8 roadmap kept byte-identical in importgraph_test.go. The pin lifted
// at M9b, as planned — M8e was the other candidate, and the M8d
// menu-write no-go (25/07/2026) removed it. It folded into this table
// (its ft710 row was already the sole entry here) and was deleted from
// importgraph_test.go at Task 58 (26/07/2026); this data-driven guard is
// now the sole check of the ft710 fact.
//
// ALIAS-PROOF, deliberately (Codex plan-review F10). Unlike the retired
// single-driver guard's bare `x.Name == "ft710"` identifier match, this guard resolves
// the driver package through each file's AST import map: it looks for the
// token selector on whatever LOCAL name core/driver/<pkg> is imported as.
// A file that smuggled in a second reference via an aliased import —
//
//	import drv "…/core/driver/ft710"
//	… drv.Simulated …
//
// would evade the bare-identifier guard but is caught here. The fake
// constructor is resolved the same way, through its FULL import path
// (internal/fakeradio, from the profile row's fakeCtorPath), so an aliased
// fakeradio import is honoured too — and, unlike a path-BASE match, an
// unrelated package that merely ends in "/fakeradio" cannot satisfy the
// pairing check in place of the real internal/fakeradio.New (M9a
// Codex-review finding 2, LOW).
//
// APPROXIMATE (see the package doc comment): a plain AST walk over
// selector and call expressions, not a type-checked analysis. The token
// is matched by (resolved-package-local-name, selector-name); a
// dot-import of the driver package would evade it, and nothing in this
// repo dot-imports anything.
func TestSimulatedProfileTokensConfinement(t *testing.T) {
	// simulatedProfiles is the driver table this guard walks. Each row
	// names a concrete driver package (core/driver/<pkg>), its simulated-
	// profile constant (<pkg>.<token>), and the fake constructor the sole
	// file referencing that token must also call.
	//
	// TWO rows since M9c-6 task 6, and that is the design working: the
	// FTdx10's registration added a ROW here — not a second test, not a
	// second walk — so the FTdx10's simulated profile is confined by
	// exactly the check that already confined the FT-710's, including the
	// pairing clause (the one file naming ftdx10.Simulated must also call
	// fakedx10.New, so a Simulated driver can never be wired to another
	// model's rig or to a real port). Each further driver is one more row.
	//
	// FOUR ROWS SINCE M9d-2 TASK 7, and the last two are NOT one per driver
	// package: core/driver/ftdx101 drives BOTH FTDX101 siblings from one
	// type, so there is ONE package and ONE Simulated token, but TWO fake
	// constructors — fakedx101.NewD and fakedx101.NewMP — and internal/wiring
	// must call both in its one fake-wiring file. A row is therefore
	// (package, token, fake CONSTRUCTOR), and the FTdx101 contributes two
	// rows differing only in fakeCtor. The confinement clauses (exactly one
	// non-test file, and it lives in internal/wiring) are checked twice over
	// the same token and agree trivially; the PAIRING clause is what earns
	// the second row, because a registration that wired both models to
	// fakedx101.NewD would satisfy the D row and fail the MP one.
	simulatedProfiles := []struct {
		pkg          string // base name of core/driver/<pkg>
		token        string // the simulated-profile constant, e.g. "Simulated"
		fakeCtor     string // "<pkgbase>.<Func>" the sole file must also call — for messages and the func name
		fakeCtorPath string // the constructor package's FULL import path, below modulePrefix (e.g. "internal/fakeradio")
	}{
		{"ft710", "Simulated", "fakeradio.New", "internal/fakeradio"},
		{"ftdx10", "Simulated", "fakedx10.New", "internal/fakedx10"},
		{"ftdx101", "Simulated", "fakedx101.NewD", "internal/fakedx101"},
		{"ftdx101", "Simulated", "fakedx101.NewMP", "internal/fakedx101"},
		// The IC-7610 (Wave 4 task R1), this project's first non-Yaesu
		// row: one package, one Simulated token and one fake constructor —
		// the ftdx10 shape, not the ftdx101 shared-driver/two-siblings
		// one, since core/driver/ic7610 has no registered sibling (matrix
		// §4). The AST walk this guard runs is name-based, not
		// import-path-based, so it does not care that internal/fakeic7610's
		// New is wrapped in an ic7610FakeAdapter{...} composite literal at
		// its one call site in internal/wiring/fake.go — the call
		// expression fakeic7610.New(...) is still there, nested inside it,
		// and fileHasCall's ast.Inspect walk finds it regardless of what
		// encloses it.
		{"ic7610", "Simulated", "fakeic7610.New", "internal/fakeic7610"},
		// The IC-7300 and IC-7300MK2 (Wave 4 task R3), this project's
		// second Icom family and first Icom PAIR: two rows, not one,
		// because — unlike the IC-7610 — this pair has SEPARATE driver
		// packages and SEPARATE fakes (core/driver/ic7300 /
		// core/driver/ic7300mk2, internal/fakeic7300 /
		// internal/fakeic7300mk2), so each contributes its own pkg, its
		// own Simulated token and its own fake constructor. Both fakes'
		// New calls appear directly in internal/wiring/fake.go's
		// fakeDrivers table (no adapter wraps either — both Port()
		// methods already return io.ReadWriteCloser), so the AST walk
		// finds each fakeic7300.New(...) / fakeic7300mk2.New(...) call
		// expression exactly where the ic7610 row's comment says it
		// would even if one had been wrapped.
		{"ic7300", "Simulated", "fakeic7300.New", "internal/fakeic7300"},
		{"ic7300mk2", "Simulated", "fakeic7300mk2.New", "internal/fakeic7300mk2"},
		// The IC-705 (Wave 4 task R4), this project's third Icom
		// registration and second lone-model one: one package, one
		// Simulated token and one fake constructor, on the same ic7610
		// shape as above (no adapter wraps fakeic705.New — its Port()
		// already returns io.ReadWriteCloser — so the row's shape is the
		// simpler of the two this table already carries).
		{"ic705", "Simulated", "fakeic705.New", "internal/fakeic705"},
		// The IC-9700 (Wave 4 task R5), this project's fourth Icom
		// registration and second lone-model one since the IC-705: one
		// package, one Simulated token and one fake constructor, on the
		// same ic705 shape as above (no adapter wraps fakeic9700.New —
		// its Port() already returns io.ReadWriteCloser — so the row's
		// shape is the simpler of the two this table carries, three
		// static banks notwithstanding: this table asks nothing about
		// bank shape).
		{"ic9700", "Simulated", "fakeic9700.New", "internal/fakeic9700"},
	}

	// Non-vacuity: an empty table would make the loop below a no-op and
	// pass silently.
	if len(simulatedProfiles) == 0 {
		t.Fatal("simulatedProfiles is empty — this guard would pass vacuously; it must list every concrete driver")
	}

	files := parseRepo(t)

	for _, prof := range simulatedProfiles {
		driverPath := modulePrefix + "core/driver/" + prof.pkg
		_, ctorFunc, ok := strings.Cut(prof.fakeCtor, ".")
		if !ok {
			t.Fatalf("profile %q: fakeCtor %q is not in <pkg>.<Func> form", prof.pkg, prof.fakeCtor)
		}
		ctorPath := modulePrefix + prof.fakeCtorPath

		var filesWithToken []string
		sawCtorInWiring := false

		for _, pf := range files {
			// Alias-proof: only files that actually import the driver can
			// name <pkg>.token, and we match the token on the LOCAL name
			// that import was bound to (alias or path base).
			driverLocal, importsDriver := importsPath(pf.file, driverPath)
			if !importsDriver {
				continue
			}
			if !fileHasSelector(pf.file, driverLocal, prof.token) {
				continue
			}
			filesWithToken = append(filesWithToken, pf.relPath)

			if pf.relDir != "internal/wiring" {
				t.Errorf("profile %q: %s references %s.%s but lives outside internal/wiring — the simulated-profile token must be confined to the fake-wiring file (task-11 brief §3)", prof.pkg, pf.relPath, prof.pkg, prof.token)
				continue
			}
			// The sole in-wiring file must also construct the fake rig,
			// resolving the constructor's package alias-proof by its FULL
			// import path (not merely its path base — see this test's doc
			// comment, finding 2): only the real internal/fakeradio import,
			// under whatever local name it is bound to, can satisfy this.
			if ctorLocal, importsCtor := importsPath(pf.file, ctorPath); importsCtor && fileHasCall(pf.file, ctorLocal, ctorFunc) {
				sawCtorInWiring = true
			}
		}

		// Non-vacuity per row: the token must have been observed at all. A
		// typo'd pkg (wrong driverPath) or a broken walk would match zero
		// files; without this the "exactly one" check could not tell
		// "moved to zero" apart from "walk found nothing".
		if len(filesWithToken) == 0 {
			t.Errorf("profile %q: never saw %s.%s in any non-test file — the driver import path (%s) or the walk is wrong, and this row was checked vacuously", prof.pkg, prof.pkg, prof.token, driverPath)
			continue
		}
		if len(filesWithToken) != 1 {
			t.Errorf("profile %q: %s.%s appears in %d non-test files repo-wide (%v), want exactly 1 — the Simulated/fake pairing must stay confined to the one fake-wiring file (task-11 brief §3)", prof.pkg, prof.pkg, prof.token, len(filesWithToken), filesWithToken)
			continue
		}
		if !sawCtorInWiring {
			t.Errorf("profile %q: the sole file referencing %s.%s (%s) does not also call %s — the fake-wiring constructor must build the simulated driver and its fake rig together (task-11 brief §3)", prof.pkg, prof.pkg, prof.token, filesWithToken[0], prof.fakeCtor)
		}
	}
}

// fileHasSelector reports whether f contains a selector expression
// <recv>.<sel> whose receiver is the bare identifier recv (the local name
// an import was bound to). Comments are AST-invisible, so a token named
// only in prose does not count.
func fileHasSelector(f *ast.File, recv, sel string) bool {
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		s, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if id, ok := s.X.(*ast.Ident); ok && id.Name == recv && s.Sel.Name == sel {
			found = true
			return false
		}
		return true
	})
	return found
}

// fileHasCall reports whether f contains a call expression <recv>.<fn>(...)
// whose receiver is the bare identifier recv.
func fileHasCall(f *ast.File, recv, fn string) bool {
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if s, ok := call.Fun.(*ast.SelectorExpr); ok {
			if id, ok := s.X.(*ast.Ident); ok && id.Name == recv && s.Sel.Name == fn {
				found = true
				return false
			}
		}
		return true
	})
	return found
}
