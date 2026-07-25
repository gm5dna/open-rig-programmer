// SPDX-License-Identifier: GPL-3.0-or-later

package guards

import (
	"strconv"
	"strings"
	"testing"
)

// TestCompositionRootImportDiscipline pins the M9a end-state composition
// discipline of the radio-neutral core (landed by tasks 39-41): the
// concrete driver packages and the CAT protocol layer must reach the
// application only through the sanctioned composition root, never
// directly.
//
// Two rules, both over NON-TEST files (the guards' standing convention;
// parseRepo already excludes _test.go, so the two cmd/rigprog test files
// and the core/clone tests that legitimately import core/driver/ft710 are
// out of scope exactly as the other guards intend):
//
//  1. CONCRETE-DRIVER CONFINEMENT. Outside core/ itself, only files in
//     internal/wiring (the composition root) may import a concrete driver
//     package — any package whose repo-relative path carries the prefix
//     "core/driver/" WITH THE TRAILING SLASH. That trailing slash is
//     load-bearing (Codex plan-review F10): the bare "core/driver" package
//     is the radio-NEUTRAL seam that app/ and cmd/rigprog import freely,
//     and matching "core/driver" without the slash would falsely reject
//     every one of those legitimate seam imports. app/ and cmd/rigprog
//     production code import NO concrete driver: they name radios by model
//     string and let internal/wiring pick the driver.
//
//  2. CAT ISOLATION. No file under app/ or cmd/rigprog may import
//     core/cat: the CAT frame layer is a driver-internal detail, and the
//     UI/CLI layers must speak only the neutral driver.Session contract.
//
// The failure each rule exists to catch is a quiet new call site that
// re-couples a UI/CLI (or a non-wiring internal package) directly to a
// specific radio or to its wire protocol — the regression that would undo
// the M9a neutral-core refactor one import at a time.
//
// EXACT, not approximate, for its purpose (contrast the package doc
// comment): this walks import PATHS only. An import either is or is not in
// a file's import list, and the rules turn on the imported path, not on
// any local name — so unlike the token guard there is no aliasing
// subtlety to resolve here.
func TestCompositionRootImportDiscipline(t *testing.T) {
	const (
		neutralSeam  = "core/driver"  // the bare, radio-neutral seam (allowed everywhere)
		driverPrefix = "core/driver/" // concrete drivers live below here (trailing slash: F10)
		catPath      = "core/cat"
	)

	files := parseRepo(t)

	// Non-vacuity counters: prove the trees this guard polices were
	// actually walked, and that both the allowed concrete import and the
	// allowed bare-seam import were really observed — otherwise a broken
	// walk or an over-eager filter would let every rule pass vacuously.
	var appFiles, cmdFiles, wiringFiles int
	sawWiringConcreteImport := false
	sawNeutralSeamFromAppOrCmd := false

	for _, pf := range files {
		inCore := inTree(pf.relDir, "core")
		inWiring := inTree(pf.relDir, "internal/wiring")
		inApp := inTree(pf.relDir, "app")
		inCmd := inTree(pf.relDir, "cmd/rigprog")

		if inApp {
			appFiles++
		}
		if inCmd {
			cmdFiles++
		}
		if inWiring {
			wiringFiles++
		}

		for _, imp := range pf.file.Imports {
			raw, err := strconv.Unquote(imp.Path.Value)
			if err != nil || !strings.HasPrefix(raw, modulePrefix) {
				continue // external or unparsable import: not our discipline
			}
			rel := strings.TrimPrefix(raw, modulePrefix)

			// Rule 1: concrete-driver confinement (outside core/).
			if strings.HasPrefix(rel, driverPrefix) && !inCore {
				if inWiring {
					sawWiringConcreteImport = true
				} else {
					t.Errorf("%s: imports concrete driver %q — only internal/wiring may import a core/driver/ package; app/, cmd/rigprog and other internal packages must select drivers by model through the composition root (M9a neutral-core discipline)", pf.relPath, raw)
				}
			}
			if rel == neutralSeam && (inApp || inCmd) {
				sawNeutralSeamFromAppOrCmd = true
			}

			// Rule 2: CAT isolation (app/ and cmd/rigprog).
			if rel == catPath && (inApp || inCmd) {
				t.Errorf("%s: imports %q — app/ and cmd/rigprog must never import core/cat; the CAT frame layer is a driver-internal detail behind the neutral driver.Session contract (M9a neutral-core discipline)", pf.relPath, raw)
			}
		}
	}

	if appFiles == 0 {
		t.Error("scanned zero non-test files under app/ — the walk or its filters are broken, and Rules 1 and 2 passed vacuously for app/")
	}
	if cmdFiles == 0 {
		t.Error("scanned zero non-test files under cmd/rigprog — the walk or its filters are broken, and Rules 1 and 2 passed vacuously for cmd/rigprog")
	}
	if wiringFiles == 0 {
		t.Error("scanned zero non-test files under internal/wiring — the composition root is missing from the walk, so Rule 1's allowed importer was never checked")
	}
	if !sawWiringConcreteImport {
		t.Error("never saw internal/wiring import a core/driver/ concrete package — the allowed concrete import was not observed, so Rule 1 may be passing vacuously")
	}
	if !sawNeutralSeamFromAppOrCmd {
		t.Error("never saw app/ or cmd/rigprog import the bare core/driver seam — the trailing-slash distinction (F10) is untested, so Rule 1 may be over- or under-matching")
	}
}
