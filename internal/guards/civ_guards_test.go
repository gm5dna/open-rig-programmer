// SPDX-License-Identifier: GPL-3.0-or-later

package guards

import (
	"go/ast"
	"strconv"
	"strings"
	"testing"
)

// Guards over core/civ, the CI-V codec (spec D1, D2's guard list).
//
// A NEW FILE, not an amendment to the existing guards. The Icom tier is
// built in three parallel worktrees and the existing guard files are
// assigned to others; every rule this worktree owns lives here, and the
// carve-outs and non-vacuity conventions follow the house style set by
// importgraph_test.go, composition_imports_test.go and
// dialectglobals_test.go.
//
// FIVE RULES:
//
//  1. CI-V ISOLATION FROM THE UI. No file under app/ or cmd/ may import
//     core/civ or any package beneath it — the CAT rule
//     (composition_imports_test.go Rule 2) restated for the second wire
//     protocol, for the same reason: the frame layer is a driver-internal
//     detail behind the neutral driver.Session contract.
//
//  2. THE TWO CODECS ARE SIBLINGS, NOT RELATIVES. core/cat and core/civ
//     must never import each other, in either direction. Spec D1 opens
//     with it: CI-V is not a dialect of CAT, nothing in core/cat is
//     reused, and an import either way would start the erosion by
//     borrowing one shared helper.
//
//  3. THE NEUTRAL SEAM STAYS NEUTRAL. The bare core/driver package must
//     never import core/civ, mirroring TestDriverSeamPackageDoesNotImportCAT
//     exactly — including its EXACT-directory scope, since
//     core/driver/<model> packages are radio-specific by design and will
//     import core/civ.
//
//  4. THE WRITE BUILDER IS REACHABLE ONLY FROM THE DRIVER. civ.BuildMemorySet
//     is the one builder in the Icom tier that mutates a radio, and it may
//     be referenced only from core/driver/**, with a test-only carve-out
//     for core/civ itself and core/civ/civtest — the same two-package
//     exact set core/cat and core/cat/dialecttest have in
//     TestWritePathReachableOnlyThroughDriver, and for the same reasons.
//
//  5. GATE-REACHING VALIDATORS ARE PROFILE METHODS. The dialectglobals
//     mirror spec D2 asks for (round 2, F10). A package-level validator
//     cannot consult a profile at all, and would bind every Icom model to
//     one model's rule at the point that decides what reaches a radio.
//
// EXACT, not approximate, for rules 1-3: they walk import PATHS, and an
// import either is or is not in a file's list. Rule 4 is the house's
// approximate name-match, with the same acknowledged trade-off its CAT
// twin documents. Rule 5 is a structural tripwire, deliberately small —
// see dialectglobals_test.go's own note on why the semantic version was
// abandoned.

const (
	civPath     = "core/civ"
	civtestPath = "core/civ/civtest"
	catPathCIV  = "core/cat"
)

// TestCIVUnreachableFromAppAndCmd is Rule 1.
//
// A forbidden-import rule has no legitimate positive site, so it gets no
// "we saw the allowed case" counter — there is nothing true to count. Its
// non-vacuity is the walked-file count: if the walk sees no files under
// app/ or cmd/, the rule passed over nothing.
func TestCIVUnreachableFromAppAndCmd(t *testing.T) {
	files := parseRepo(t)

	var appFiles, cmdFiles int
	for _, pf := range files {
		inApp := inTree(pf.relDir, "app")
		inCmd := inTree(pf.relDir, "cmd")
		if inApp {
			appFiles++
		}
		if inCmd {
			cmdFiles++
		}
		if !inApp && !inCmd {
			continue
		}
		for _, raw := range moduleImports(pf.file) {
			if raw == civPath || strings.HasPrefix(raw, civPath+"/") {
				t.Errorf("%s: imports %q — app/ and cmd/ must never import core/civ or any package beneath it; the CI-V frame layer is a driver-internal detail behind the neutral driver.Session contract, and a core/civ/** model package drags it in transitively", pf.relPath, modulePrefix+raw)
			}
		}
	}

	if appFiles == 0 {
		t.Error("scanned zero non-test files under app/ — the walk or its filters are broken, and this rule passed vacuously for app/")
	}
	if cmdFiles == 0 {
		t.Error("scanned zero non-test files under cmd/ — the walk or its filters are broken, and this rule passed vacuously for cmd/")
	}
}

// TestCATandCIVDoNotImportEachOther is Rule 2.
//
// Spec D1: "Sibling of core/cat; no import in either direction
// (guarded)." CI-V is binary framing with packed BCD and bus echo; CAT is
// printable ASCII terminated by ';'. They share a Command SHAPE and
// nothing else, and core/civ restates that shape rather than importing it
// — deliberately, because the first shared helper is how a sibling
// becomes a dependency.
//
// Non-vacuity here is real rather than structural: both trees must be
// SEEN, since a rule about two directories that walked neither would pass
// in silence.
func TestCATandCIVDoNotImportEachOther(t *testing.T) {
	files := parseRepo(t)

	var catFiles, civFiles int
	for _, pf := range files {
		inCAT := inTree(pf.relDir, catPathCIV)
		inCIV := inTree(pf.relDir, civPath)
		if inCAT {
			catFiles++
		}
		if inCIV {
			civFiles++
		}
		if !inCAT && !inCIV {
			continue
		}
		for _, raw := range moduleImports(pf.file) {
			if inCAT && (raw == civPath || strings.HasPrefix(raw, civPath+"/")) {
				t.Errorf("%s: core/cat imports %q — the two codecs are siblings, not relatives, and an import either way starts the erosion by borrowing one shared helper (spec D1)", pf.relPath, modulePrefix+raw)
			}
			if inCIV && (raw == catPathCIV || strings.HasPrefix(raw, catPathCIV+"/")) {
				t.Errorf("%s: core/civ imports %q — CI-V is not a dialect of CAT: nothing in core/cat is reused, and core/civ restates the Command shape rather than importing it (spec D1)", pf.relPath, modulePrefix+raw)
			}
		}
	}

	if catFiles == 0 {
		t.Error("scanned zero non-test files under core/cat — the walk is broken and this rule passed vacuously in that direction")
	}
	if civFiles == 0 {
		t.Error("scanned zero non-test files under core/civ — the walk is broken and this rule passed vacuously in that direction")
	}
}

// TestDriverSeamPackageDoesNotImportCIV is Rule 3, mirroring
// TestDriverSeamPackageDoesNotImportCAT.
//
// SCOPE IS EXACT-DIRECTORY, and the exactness is load-bearing: core/driver
// is the radio-NEUTRAL Driver/Session contract, while core/driver/ic7300
// and its siblings are radio-specific by design and WILL import core/civ.
// Comparing relDir for equality rather than using inTree is what keeps the
// rule about the seam alone.
func TestDriverSeamPackageDoesNotImportCIV(t *testing.T) {
	const wantDir = "core/driver"

	files := parseRepo(t)

	var seamFiles int
	for _, pf := range files {
		if pf.relDir != wantDir {
			continue
		}
		seamFiles++
		for _, raw := range moduleImports(pf.file) {
			if raw == civPath || strings.HasPrefix(raw, civPath+"/") {
				t.Errorf("%s: imports %q — core/driver (the radio-neutral seam package) must never import core/civ; a protocol identifier may appear only in a concrete driver subpackage (e.g. core/driver/ic7300)", pf.relPath, modulePrefix+raw)
			}
		}
	}

	if seamFiles == 0 {
		t.Fatal("parseRepo found zero files in core/driver — the walk or its filters are broken, and the check above passed vacuously")
	}
}

// civWriteBuilders are the core/civ builders that MUTATE a radio.
//
// One name, and that is the tier's whole write surface: spec D1
// (adjudications 3 and 19) ships NO clear/erase builder and NO
// transceive-set builder, so there is nothing else to fence. A new
// mutating builder must be ADDED HERE BY NAME — nothing about the shape of
// this check makes that automatic, exactly as its CAT twin says of
// BuildMWSet/BuildMTSet/BuildMTSetCombined.
var civWriteBuilders = []string{"BuildMemorySet"}

// civWriteBuilderCarveOut is the EXACT set of packages, other than
// core/driver/**, that may reference a civ write builder.
//
// TWO MEMBERS, each for its own reason, and NOT a prefix. core/civ is the
// builder's own package. core/civ/civtest is a NON-test file — the
// exported conformance suite — that must call BuildMemorySet to check the
// property that matters, exactly as core/cat/dialecttest calls
// BuildMWSet. Every FUTURE core/civ subpackage is a data-only model
// package, and a write-builder call site appearing in one is precisely the
// regression this fence exists to refuse: a prefix would exempt them all
// in advance.
var civWriteBuilderCarveOut = map[string]bool{
	civPath:     true,
	civtestPath: true,
}

// TestCIVWritePathReachableOnlyThroughDriver is Rule 4.
//
// APPROXIMATE, deliberately, and on the house pattern: any SELECTOR named
// BuildMemorySet outside the carve-out, whatever the receiver. A future
// unrelated type with a method of that name elsewhere would false-positive
// — acceptable, since a name collision on this tier's one mutating builder
// deserves the look.
//
// NON-VACUITY IS THE CARVE-OUT SITE, not a driver call site. There is no
// core/driver/ic* package yet — those land in Wave 3 — so demanding that
// the walk SEE a driver call would fail this guard for a schedule reason
// rather than a correctness one. What the counter proves is what it can:
// that the walk reaches a file which really does reference the builder, so
// a broken filter cannot let the rule pass over an empty set.
func TestCIVWritePathReachableOnlyThroughDriver(t *testing.T) {
	files := parseRepo(t)

	sawCarveOutSite := false
	sawDriverSite := false

	for _, pf := range files {
		inDriver := inTree(pf.relDir, "core/driver")
		inCarveOut := civWriteBuilderCarveOut[pf.relDir]

		ast.Inspect(pf.file, func(n ast.Node) bool {
			sel, isSel := n.(*ast.SelectorExpr)
			if !isSel {
				return true
			}
			named := false
			for _, name := range civWriteBuilders {
				if sel.Sel.Name == name {
					named = true
					break
				}
			}
			if !named {
				return true
			}
			switch {
			case inCarveOut:
				sawCarveOutSite = true
			case inDriver:
				sawDriverSite = true
			default:
				t.Errorf("%s: references .%s — core/civ's memory-set builder may be used only from core/civ, core/civ/civtest (the conformance suite) and core/driver/**; other core/civ subpackages are NOT exempt, the carve-out being an exact two-package set rather than a prefix (composition-root discipline)", pf.relPath, sel.Sel.Name)
			}
			return true
		})
	}

	if !sawCarveOutSite {
		t.Errorf("never saw %v referenced from %v — the walker or its filters are broken, and this rule passed over an empty set", civWriteBuilders, sortedKeys(civWriteBuilderCarveOut))
	}
	if sawDriverSite {
		t.Logf("core/driver/** references a civ write builder — expected from Wave 3 onwards")
	}
}

// civGateReachingValidators must be Profile METHODS. Each is reached by
// Profile.AllowedCommand, so a package-level version would bind every Icom
// model to one model's rule at the one point deciding what is written to a
// radio.
//
// WHAT BELONGS HERE: a method applying WRITE-DIRECTION POLICY on the path
// from AllowedCommand to a verdict — the rules saying which addresses,
// values and names may reach a radio. NOT parsers (decodeRecord and
// decodeAddress are Profile methods the gate reaches too, and neither is
// listed: they decode bytes, they do not decide policy), and NOT the
// package-level frame grammar (WellFormed, the interior-byte injection
// defence, which is form-invariant and no profile may relax).
//
// SHAPE ONLY — that the seam exists. Whether a body then honours its
// receiver is the behavioural tests' job: core/civ's own table-driven
// tests run over allTestProfiles(), whose fixtures disagree at every
// attribute including the controller address. This file is a tripwire, on
// dialectglobals_test.go's explicit terms; do not grow it into the
// semantic check that file records abandoning after five review rounds.
var civGateReachingValidators = []string{
	"validAddress",
	"validateRecordFields",
	"validName",
}

// civProfileOwnedNames must never appear as package-level declarations in
// core/civ. Each names a datum a Profile carries, and a method reading a
// package-level version of it would gate for one radio while claiming to
// gate for the receiver it was called on.
//
// A TRIPWIRE FOR NAMES THAT DO NOT EXIST, which is the point: core/civ has
// no history of promoting these off the package, so unlike core/cat's
// promotedConstants this list is prospective. It says which shapes are
// forbidden BEFORE somebody writes one — the cheapest moment to say so.
//
// ControllerAddressDefault is deliberately ABSENT. It is a package
// constant on purpose: 0xE0 is a property of this program's role on the
// CI-V bus, not of any radio, and Profile.ControllerAddress is what every
// gate-reaching path actually reads (a fixture answering to 0xE1 is what
// proves that). Forbidding the constant would forbid the default itself.
var civProfileOwnedNames = []string{
	"radioAddress",
	"controllerAddress",
	"channelLo",
	"channelHi",
	"groupCount",
	"nameLength",
	"namePadByte",
	"nameCharset",
	"recordLength",
	"acceptedRecordLengths",
	"buildRecordLength",
	"maxFrameLen",
}

// TestCIVGateReachingValidatorsAreProfileMethods is Rule 5's first half.
func TestCIVGateReachingValidatorsAreProfileMethods(t *testing.T) {
	files := civFiles(t)
	if len(files) == 0 {
		t.Fatal("parsed no core/civ files — this guard would pass vacuously")
	}

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

	for _, name := range civGateReachingValidators {
		decls, ok := found[name]
		if !ok {
			t.Errorf("%s is not declared anywhere in core/civ — if it was renamed, update this guard deliberately rather than letting it pass vacuously", name)
			continue
		}
		for _, d := range decls {
			if !d.isMethod {
				t.Errorf("%s at %s is a package-level function, want a method on Profile — it is reached by AllowedCommand, so a package-level version binds every Icom model to one model's rule at the gate", name, d.where)
				continue
			}
			if d.recvType != "Profile" {
				t.Errorf("%s at %s has receiver %s, want Profile", name, d.where, d.recvType)
			}
		}
	}
	t.Logf("scanned %d core/civ files, %d top-level funcs", len(files), len(found))
}

// TestCIVProfileDataIsNotAPackageGlobal is Rule 5's second half: the
// dialectglobals mirror.
func TestCIVProfileDataIsNotAPackageGlobal(t *testing.T) {
	files := civFiles(t)
	if len(files) == 0 {
		t.Fatal("parsed no core/civ files — this guard would pass vacuously")
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
					if n.Name != "_" {
						declared[n.Name] = pf.relPath
					}
				}
			}
		}
	}
	if len(declared) == 0 {
		t.Fatal("found no package-level declarations in core/civ — the walk is broken")
	}

	for _, name := range civProfileOwnedNames {
		if where, found := declared[name]; found {
			t.Errorf("%s is a package-level declaration, at %s — it is profile data and belongs on the receiver; a method reading it gates for one radio while claiming to gate for the receiver it was called on", name, where)
		}
	}
	t.Logf("scanned %d core/civ files, %d package-level declarations", len(files), len(declared))
}

// civFiles parses core/civ's own non-test files — the package itself, NOT
// its subpackages, since civtest is a test-support package and future
// subpackages are per-model data.
func civFiles(t *testing.T) []parsedFile {
	t.Helper()
	var out []parsedFile
	for _, pf := range parseRepo(t) {
		if pf.relDir == civPath {
			out = append(out, pf)
		}
	}
	return out
}

// moduleImports returns f's imports that belong to this module, with the
// module prefix stripped. External imports are not this file's discipline.
func moduleImports(f *ast.File) []string {
	var out []string
	for _, imp := range f.Imports {
		raw, err := strconv.Unquote(imp.Path.Value)
		if err != nil || !strings.HasPrefix(raw, modulePrefix) {
			continue
		}
		out = append(out, strings.TrimPrefix(raw, modulePrefix))
	}
	return out
}

// sortedKeys renders a set for a failure message, deterministically.
func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
