// SPDX-License-Identifier: GPL-3.0-or-later

package guards

import (
	"go/ast"
	"strconv"
	"strings"
	"testing"
)

// The codeplug-load guard (Wave 4 task R2, deviation (c)).
//
// codeplug.Load is no longer the whole of loading a file. A schema-4
// file's ten tier-added fields can be ABSENT from the JSON for either of
// two opposite reasons — "this radio has no such field" or "nobody has
// said anything about this field yet" — and telling them apart needs the
// loaded model's own capabilities, which core/codeplug cannot obtain:
// resolving a model name means internal/wiring, which imports
// core/driver, which imports core/codeplug. So the rule lives in
// core/codeplug (NormaliseTierFields) while the model-to-capabilities
// lookup lives at the composition roots, and a load that skips the roots
// gets a codeplug whose Absent fields were never resolved — one that
// compares unequal, field for field, to a fresh read of the very same
// radio, which is exactly what codeplug.Diff would then report as every
// channel modified.
//
// That is a discipline no compiler enforces, so this guard does: exactly
// two non-test call sites, both named below.
//
// EXACT, not approximate, unlike civ_guards_test.go's Rule 4: the match
// is a selector on the identifier that THIS FILE binds to
// core/codeplug (its import name, alias included), so an unrelated
// type's Load method — and there are several in this repo — cannot
// false-positive, and an aliased import cannot slip past. What it cannot
// see is an indirect call (a func value taken from codeplug.Load and
// called elsewhere); that is the acknowledged limit of every guard in
// this package, and the same one Rule 4 records.
const codeplugPath = "core/codeplug"

// codeplugLoadRoots is the exact, file-level carve-out: the two
// composition roots sanctioned to call codeplug.Load, each of which runs
// codeplug.NormaliseTierFields on the result against the capabilities of
// the model the FILE names.
//
// FILE-level rather than directory-level, deliberately. A package-level
// carve-out would let a second, unnormalised load appear in a sibling
// file of the same package — app/ and cmd/rigprog are large packages —
// which is the very thing being prevented, not a variant of it.
var codeplugLoadRoots = map[string]string{
	"app/fileio.go":         "loadFilePath (the GUI's file-open path; normalises via normaliseTierFieldsForOwnModel)",
	"cmd/rigprog/fileio.go": "loadCodeplugStrict (the CLI's ONE strict-load helper, shared by diff/export/import/write/settings)",
}

// TestCodeplugLoadReachableOnlyThroughACompositionRoot pins the two
// sanctioned call sites.
//
// NON-VACUITY IS BOTH CARVE-OUT SITES, not a file count: each named root
// must really contain a codeplug.Load call. A rule whose allowed sites
// had moved or been renamed would otherwise pass in silence while the
// discipline it describes no longer existed anywhere.
func TestCodeplugLoadReachableOnlyThroughACompositionRoot(t *testing.T) {
	files := parseRepo(t)

	seen := make(map[string]bool, len(codeplugLoadRoots))
	for _, pf := range files {
		local, imports := codeplugImportName(pf.file)
		if !imports {
			continue
		}
		ast.Inspect(pf.file, func(n ast.Node) bool {
			sel, isSel := n.(*ast.SelectorExpr)
			if !isSel || sel.Sel.Name != "Load" {
				return true
			}
			pkg, isIdent := sel.X.(*ast.Ident)
			if !isIdent || pkg.Name != local {
				return true
			}
			if _, ok := codeplugLoadRoots[pf.relPath]; ok {
				seen[pf.relPath] = true
				return true
			}
			t.Errorf("%s: calls %s.Load — codeplug.Load may be called from exactly two non-test composition roots (%s), because Load alone does NOT finish loading a schema-4 file: its ten tier-added fields are left Absent until codeplug.NormaliseTierFields resolves them against the loaded model's own capabilities. Route this load through one of those roots, or call codeplug.NormaliseTierFields(cp, caps) here with the capabilities of the model the FILE names — never leave a loaded codeplug unnormalised", pf.relPath, local, strings.Join(sortedRootNames(), " and "))
			return true
		})
	}

	for path, what := range codeplugLoadRoots {
		if !seen[path] {
			t.Errorf("%s no longer calls codeplug.Load — this guard's carve-out names it as %s, so either the call moved (update codeplugLoadRoots, and check the new site normalises) or this rule is now guarding nothing", path, what)
		}
	}
}

// sortedRootNames returns the carve-out's paths in a stable order, for a
// deterministic failure message.
func sortedRootNames() []string {
	out := make([]string, 0, len(codeplugLoadRoots))
	for path := range codeplugLoadRoots {
		out = append(out, path)
	}
	// Two entries; insertion sort keeps this dependency-free.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// codeplugImportName returns the identifier f binds to core/codeplug —
// the alias when one is given, otherwise the package name — and whether
// f imports it at all. A blank or dot import answers false: neither can
// produce a qualified codeplug.Load call for this rule to judge, and a
// dot import of this package exists nowhere in the repo.
func codeplugImportName(f *ast.File) (string, bool) {
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path != modulePrefix+codeplugPath {
			continue
		}
		if imp.Name == nil {
			return "codeplug", true
		}
		if imp.Name.Name == "_" || imp.Name.Name == "." {
			return "", false
		}
		return imp.Name.Name, true
	}
	return "", false
}
