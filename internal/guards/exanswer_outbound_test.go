// SPDX-License-Identifier: GPL-3.0-or-later

package guards

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestParseEXAnswerNeverBuildsAnOutboundSet is the milestone spec's §3
// "Set/Answer collision" claim, pinned: EX Set and Answer share an
// identical wire shape (manual lines ~630-637), so a value this codec
// parsed FROM a received Answer must never be re-offered OUTBOUND as a
// Set — that would let a radio's own read reply get replayed as a write.
// The spec's own review confirmed this by grepping every
// cat.Dialect.ParseEXAnswer call site and every outbound gate call site
// (core/transport/catframing.go:49, core/cat/dialecttest) by hand; this
// guard is that grep, automated, on the retirednames_test.go/importgraph_
// test.go precedent (M3 Codex fix 9's "no code inside this repo quietly
// grows a new call site below the policy layers").
//
// TWO CHECKS, because either alone can be worked around by a future edit
// that reshuffles which file does what:
//
//  1. No source file outside core/cat itself calls BOTH ParseEXAnswer AND
//     BuildEXSet — the ONE EX Set builder (ex.go). A file that never holds
//     both a parsed Answer and the Set builder in scope together cannot
//     wire the first into the second by accident. core/driver/ft710's
//     own files hold neither today: ReadSetting reaches ParseEXAnswer only
//     through yaesu.ParseEXResponse, a wrapper this grep cannot see
//     through — which is closer to the spec's actual claim ("nothing in
//     core/driver/ft710 feeds a parsed ParseEXAnswer back to Engine.Do"),
//     not a weaker one.
//  2. Every ParseEXAnswer call site outside core/cat's own package (which
//     needs it for validEXRead's Set arm, allowlist.go) is in the
//     reviewed allowlist below — every one a read-response parser
//     (yaesu/ts590/ts890/ts990/ts480 settings.go's ParseEXResponse), none
//     a write path. A new call site anywhere else fails this and demands
//     the same review before joining the list.
func TestParseEXAnswerNeverBuildsAnOutboundSet(t *testing.T) {
	root := repoRoot(t)
	skipDirs := map[string]bool{
		".git": true, ".superpowers": true, "docs": true,
		"node_modules": true, "build": true, "dist": true,
	}
	// Reviewed 19/09/2026 (task c): every one of these is a read-response
	// parser — ParseEXResponse or its callers — that turns a wire answer
	// into a driver.SettingValue for ReadSetting, never into an outbound
	// frame. core/cat itself is out of scope for this list (see the
	// core/cat/ skip below): it is where ParseEXAnswer and BuildEXSet are
	// BOTH defined, and where validEXRead's Set arm legitimately parses an
	// inbound-shaped Set frame to judge whether it may go out — the gate
	// this guard exists to keep honest, not a violation of it.
	parseExAnswerAllowlist := map[string]bool{
		"core/driver/internal/yaesu/settings.go": true,
		"core/driver/ts590/settings.go":          true,
		"core/driver/ts890/settings.go":          true,
		"core/driver/ts990/settings.go":          true,
		"core/driver/ts480/settings.go":          true,
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
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel != "." && skipDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		// Scope is Yaesu CAT only — core/cat, core/driver, core/transport —
		// the family this milestone's cat.Dialect.ParseEXAnswer/BuildEXSet
		// belong to. core/kw declares an UNRELATED ParseEXAnswer method on
		// its own Layout type, for Kenwood's menu protocol; grepping the
		// text "ParseEXAnswer(" there would flag a different function by
		// name collision alone, not this milestone's write gate.
		if !strings.HasPrefix(rel, "core/cat/") && !strings.HasPrefix(rel, "core/driver/") && !strings.HasPrefix(rel, "core/transport/") {
			return nil
		}
		// core/cat is where ParseEXAnswer and BuildEXSet are both DEFINED,
		// and the one place (validEXRead's Set arm) that legitimately holds
		// both — see the allowlist comment above. Test-support packages
		// (their own name says so, e.g. dialecttest, kwtest) exercise the
		// gate directly and are not outbound call sites either — the spec's
		// own grep names dialecttest as one of the two GATE call sites, not
		// a ParseEXAnswer one.
		if strings.HasPrefix(rel, "core/cat/") || strings.HasSuffix(filepath.Dir(rel), "test") {
			return nil
		}

		body, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		scanned++
		text := string(body)
		hasParse := strings.Contains(text, "ParseEXAnswer(")
		hasBuild := strings.Contains(text, "BuildEXSet(")

		if hasParse && hasBuild {
			t.Errorf("%s calls both ParseEXAnswer and BuildEXSet — a parsed EX Answer must never feed an outbound Set; route the write through BuildEXSet from caller-supplied data only", rel)
		}
		if hasParse && !parseExAnswerAllowlist[rel] {
			t.Errorf("%s calls ParseEXAnswer but is not in parseExAnswerAllowlist — confirm it never feeds an outbound BuildEXSet/AllowedCommand call before adding it", rel)
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
