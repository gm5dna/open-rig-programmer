// SPDX-License-Identifier: GPL-3.0-or-later

package guards

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// retiredWriteResultNames are the eight names M9c-5's E6 removed when
// driver.WriteResult stopped describing the FT-710's own write
// choreography: four Go fields on the neutral driver seam, and the four
// journal keys core/clone projected them into.
//
// Why they get a guard rather than a one-off grep in a review. These were
// not an internal spelling: they were the FT-710's two-frame MW-then-MT
// sequence written into the type EVERY radio's driver must return, and
// into a durable, user-visible audit file. A radio that writes a channel
// and its label in ONE combined frame has no honest answer to them —
// reported as "MT sent, MW not", its perfectly ordinary write reads, in
// FT-710 terms, as a tag-only write that never touched the channel data.
// Any of these names reappearing means that assumption has come back, in
// the one place a second registered model is guaranteed to meet it.
//
// The convention this follows is the repo's own, recorded in
// docs/superpowers/m9c3-baseline-manifest.md for `mtAnswerMaxLen`: the
// milestone grep for a retired name passes with exactly ONE hit — the
// entry here that forbids it returning.
var retiredWriteResultNames = []string{
	// core/driver's fields (M9c-5 E6 -> driver.WriteStep{Command, Sent,
	// Confirmed}).
	"MWSent",
	"MWConfirmed",
	"MTSent",
	"MTConfirmed",
	// core/clone's journal keys (M9c-5 E6 -> the projected "steps" list,
	// stamped "write_result_format": 2).
	"mw_sent",
	"mw_confirmed",
	"mt_sent",
	"mt_confirmed",
}

// guardedSourceExts are the extensions this guard reads: the Go tree plus
// the frontend, since the wailsjs bindings are GENERATED from the Go
// types and would carry a resurrected field name straight into the GUI.
var guardedSourceExts = map[string]bool{
	".go": true, ".js": true, ".ts": true, ".svelte": true, ".json": true,
}

// TestRetiredWriteResultNamesAreGone walks the repository's source and
// fails if any retired name reappears.
//
// EXCLUSIONS, and why each is not a hole:
//
//   - This file. It must name them to forbid them.
//   - docs/ and .superpowers/. These are the milestone's own written
//     history — specs, plans, manifests and handoffs that RECORD what was
//     removed and why. Scrubbing the names from the record would destroy
//     the only durable explanation of a change to a durable file format.
//   - .git, node_modules, build/ and dist/. Not source.
//
// Everything a compiler or a bundler reads is in scope.
func TestRetiredWriteResultNamesAreGone(t *testing.T) {
	root := repoRoot(t)
	skipDirs := map[string]bool{
		".git": true, ".superpowers": true, "docs": true,
		"node_modules": true, "build": true, "dist": true,
	}

	scanned := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		if d.IsDir() {
			if rel != "." && skipDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if rel == filepath.Join("internal", "guards", "retirednames_test.go") {
			return nil
		}
		if !guardedSourceExts[filepath.Ext(path)] {
			return nil
		}
		body, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		scanned++
		text := string(body)
		for _, name := range retiredWriteResultNames {
			if strings.Contains(text, name) {
				t.Errorf("%s contains the retired name %q — driver.WriteResult reports neutral steps since M9c-5's E6; see retiredWriteResultNames", filepath.ToSlash(rel), name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	if scanned == 0 {
		t.Fatal("scanned no source files — this guard would pass vacuously")
	}
}
