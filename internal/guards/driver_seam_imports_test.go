// SPDX-License-Identifier: GPL-3.0-or-later

package guards

import "testing"

// TestDriverSeamPackageDoesNotImportCAT pins the seam-package independence
// task-32's brief requires: core/driver — the radio-NEUTRAL Driver/Session
// contract and, as of this task, the neutral settings surface
// (SettingsDescriptor/SettingItem/SettingsReader, settings.go) — must
// never import core/cat: generic layers walking a SettingsDescriptor must
// never see a cat.EXAddress or any other core/cat identifier, only opaque
// IDs and labels a concrete driver package (core/driver/ft710) minted.
//
// Scope: ONLY core/driver's own package files — core/driver/ft710 and any
// future driver subpackage are radio-specific by design and DO import
// core/cat; this guard deliberately excludes anything below core/driver
// itself (parseRepo's relDir is compared for EXACT equality with
// "core/driver", not inTree, which would also match "core/driver/ft710").
//
// Uses parseRepo (importgraph_test.go) for the identical AST-walking idiom
// every other guard in this package already relies on; this is a NEW file,
// not a modification of importgraph_test.go itself (task-32 brief,
// "Working environment").
func TestDriverSeamPackageDoesNotImportCAT(t *testing.T) {
	const (
		wantDir = "core/driver"
		catPath = modulePrefix + "core/cat"
	)

	files := parseRepo(t)

	var seamFiles []string
	for _, pf := range files {
		if pf.relDir != wantDir {
			continue
		}
		seamFiles = append(seamFiles, pf.relPath)

		if _, ok := importsPath(pf.file, catPath); ok {
			t.Errorf("%s: imports %q — core/driver (the radio-neutral seam package) must never import core/cat; a driver's protocol identifiers may only appear in a concrete driver subpackage (e.g. core/driver/ft710)", pf.relPath, catPath)
		}
	}

	if len(seamFiles) == 0 {
		t.Fatal("parseRepo found zero files in core/driver — the walk or its filters are broken, and the check above passed vacuously (task-32 brief: assert the walked file set is non-empty)")
	}
}
