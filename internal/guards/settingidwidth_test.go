// SPDX-License-Identifier: GPL-3.0-or-later

package guards

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// THE SETTING-ID WIDTH PROSE GUARD, on the fresh-clone docs-guard
// precedent (freshclone_test.go): no fleet-neutral source file may state a
// setting-ID width other than the three widths core/codeplug's
// isSettingIDWidth actually admits.
//
// WHY IT EXISTS. core/codeplug/menus.go's isSettingIDWidth has now been
// widened twice — six ASCII digits, then four, then three for the Kenwood
// MENU number — and each widening left a trail of prose behind it in
// packages that never call the function. Those statements are the only
// documentation a caller reads before minting an ID, so a stale one is a
// live instruction to build a refused snapshot. Nothing in the compiler
// notices; only a grep-shaped guard does.
//
// WHY THE PATTERN IS PART OF THE SPECIFICATION AND NOT AN IMPLEMENTATION
// DETAIL. The sweep that first enumerated these statements grepped
// "6-digit|six-digit" alone and missed four of them, every one of which
// spells the width out in words ("four or six ASCII digits", "exactly four
// or exactly six"). A guard whose pattern is narrower than the prose it
// guards is precisely the defect it was written to catch, so staleRe below
// matches the SPELLED forms as well as the hyphenated ones.
//
// HOW IT WORKS, in two passes rather than one. A file that correctly
// states the three admitted widths contains "3, 4 or 6 ASCII digits", and
// that string CONTAINS the stale "4 or 6 ASCII digits". A single negative
// pattern would therefore fire on the corrected prose. So admittedRe is
// matched FIRST and every occurrence blanked; staleRe then runs over what
// is left, and anything it finds is a statement of a width set that is not
// the admitted one. TestSettingIDWidthPatterns pins both passes over
// literal samples, so neither regexp can rot into vacuity.
//
// The scan is over a NORMALISED form of each file — every line's leading
// whitespace and comment marker stripped, then all lines joined by single
// spaces — because these statements wrap. app/types.go's second one is
// literally "... or four or\n// six ASCII digits ...", which no
// line-by-line grep of any of the spelled forms can see; that is why the
// hand sweep found one statement in that file and the guard finds two.

// settingIDWidthScopedDirs are the repository-relative directories this
// guard walks, immediate children only, non-test .go files only.
//
// SCOPE IS THE WHOLE DESIGN OF THIS GUARD, so it is justified directory by
// directory. The rule being guarded is about the FLEET-NEUTRAL setting-ID
// contract: the width codeplug.MenuSnapshot.Validate enforces on every
// radio's IDs, and the prose that tells a caller what that width is.
//
//   - "core/codeplug" — isSettingIDWidth itself, and the two doc comments
//     and one error string that state its rule.
//   - "core/clone" — the ReadSettings preflight, which restates the rule
//     to explain why it probes the snapshot before any wire traffic.
//   - "cmd/rigprog" — the settings CSV writer, which restates it to
//     explain why the id column is not escaped.
//   - "app" — the GUI's ProgressEvent contract (types.go) and the two
//     call sites that populate it (settings.go, send.go). Immediate
//     children only, so the Wails-generated app/frontend tree is out, as
//     it is for every other guard in this package.
//   - "core/driver" — the neutral driver contract (SettingsDescriptor,
//     SettingItem). Immediate children only: core/driver/<model> packages
//     are model-specific by construction.
//
// WHAT IS DELIBERATELY OUT OF SCOPE, and why it is not a hole. Every other
// package that says "six-digit" is saying it about ONE RADIO'S EX wire
// address — core/cat and its per-model subpackages, core/driver/<model>,
// internal/fake* — where the statement is a true fact about that radio and
// becomes FALSE if generalised. Those are the same statements the two
// allowlist entries below record, and there are roughly ninety of them; a
// guard that swept them would need an allowlist longer than the rule.
//
// _test.go files are out of scope for the same reason at one remove: their
// width statements belong to a fixture's radio, not to the contract.
// app/settings_test.go pins an FT-710 read and says so ("a 6-digit
// TargetID"); that test is deliberately NOT generalised to the Kenwood
// width, because a Kenwood-width test belongs with the Kenwood
// registration rather than retrofitted onto an FT-710 fixture.
var settingIDWidthScopedDirs = []string{
	"app",
	"cmd/rigprog",
	"core/clone",
	"core/codeplug",
	"core/driver",
}

// settingIDWidthAllowlist is every in-scope site that states a width other
// than the admitted three and is CORRECT as written, keyed by
// repository-relative file:line and justified entry by entry.
//
// Both entries are MODEL-SPECIFIC statements about the FT-710 in a
// fleet-neutral file. Neither claims a rule; each names one radio, and
// each would become false if it were "finished" into the general form.
//
// A THIRD SITE IS NOT A NEW ENTRY HERE. Two sites were enumerated when the
// rule was widened to three, and a third appearing means either a real
// drift back to the old prose (fix the prose) or a genuinely new
// model-specific statement in a neutral file (which needs the milestone
// owner's ruling, not a silent exemption).
//
// Every entry MUST be hit — see the unused-entry check in the test. An
// allowlist that stops matching is an allowlist that has stopped
// protecting anything, and the usual cause is that the file moved under
// it, which is exactly when the two statements need re-reading.
var settingIDWidthAllowlist = map[string]string{
	// "core/driver/ft710/settings.go mints item IDs as the 6-digit wire
	// address, but nothing in this package or its callers may assume that
	// shape belongs to every future driver". The sentence's whole point is
	// that the width is the FT-710's and not the contract's; it is the
	// opposite of the drift this guard catches.
	"core/driver/settings.go:20": "SettingsDescriptor's doc comment, naming the FT-710's own width to deny that it is the contract's",

	// "A driver mints this however suits its own protocol (the FT-710 uses
	// its 6-digit EX wire address); callers must treat it as an opaque
	// token, never parse it." Same shape: one radio named as an example
	// inside a sentence that forbids parsing the ID at all.
	"core/driver/settings.go:61": "SettingItem.ID's doc comment, naming the FT-710's own width as an example of an opaque token",
}

// admittedRe matches a statement of the THREE admitted widths, in any of
// the spellings this repository uses for them. Occurrences are blanked
// before staleRe runs — see this file's doc comment for why the two-pass
// shape is necessary rather than merely tidy.
var admittedRe = regexp.MustCompile(`(?i)(?:exactly )?(?:three|3), (?:exactly )?(?:four|4) or (?:exactly )?(?:six|6)`)

// staleRe matches a statement of any OTHER width set: the two hyphenated
// forms the first sweep used, and the three spelled forms that sweep
// missed. It runs over text admittedRe has already blanked, so what it
// finds is prose that names four-and-six, or six alone, as the rule.
var staleRe = regexp.MustCompile(`(?i)6-digit|six-digit|(?:four|4) or (?:six|6) ASCII digits|exactly (?:four|4) or exactly (?:six|6)`)

// TestNoStaleSettingIDWidthProse is the guard proper.
func TestNoStaleSettingIDWidthProse(t *testing.T) {
	root := repoRoot(t)

	used := make(map[string]bool, len(settingIDWidthAllowlist))
	scanned := 0

	for _, dir := range settingIDWidthScopedDirs {
		entries, err := os.ReadDir(filepath.Join(root, dir))
		if err != nil {
			t.Fatalf("reading scoped directory %s: %v — this guard's scope names a directory that is not there", dir, err)
		}
		filesHere := 0
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			rel := dir + "/" + e.Name()
			body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
			if err != nil {
				t.Fatalf("reading %s: %v", rel, err)
			}
			filesHere++
			scanned++

			norm, lineOf := normaliseForWidthProse(string(body))
			blanked := blankAll(norm, admittedRe)
			for _, loc := range staleRe.FindAllStringIndex(blanked, -1) {
				line := lineOf(loc[0])
				key := fmt.Sprintf("%s:%d", rel, line)
				if _, ok := settingIDWidthAllowlist[key]; ok {
					used[key] = true
					continue
				}
				t.Errorf("%s states a setting-ID width that is not the admitted one: %q\n"+
					"\tcore/codeplug/menus.go's isSettingIDWidth admits exactly 3, 4 or 6 ASCII digits.\n"+
					"\tIf this is prose about the rule, widen it. If it is a model-specific statement in a\n"+
					"\tfleet-neutral file, it needs the milestone owner's ruling, not a new allowlist entry.",
					key, strings.TrimSpace(blanked[loc[0]:loc[1]]))
			}
		}
		if filesHere == 0 {
			t.Errorf("scoped directory %s contributed no non-test .go file — this guard would be vacuous over it", dir)
		}
	}

	if scanned == 0 {
		t.Fatal("scanned no source files — this guard would pass vacuously")
	}

	var unused []string
	for key := range settingIDWidthAllowlist {
		if !used[key] {
			unused = append(unused, key)
		}
	}
	sort.Strings(unused)
	for _, key := range unused {
		t.Errorf("allowlist entry %s matched nothing — the statement it exempts has moved or gone.\n"+
			"\tRe-read the file, re-derive the line, and update the entry (its reason: %s).",
			key, settingIDWidthAllowlist[key])
	}
}

// TestSettingIDWidthPatterns pins both regexps over literal samples, so
// the guard above cannot rot into vacuity in either direction: admittedRe
// blanking too much would hide real drift, and staleRe matching too little
// would miss it. Every "want a hit" sample is a real statement this
// repository carried at some point; every "want no hit" sample is a real
// statement it carries now.
func TestSettingIDWidthPatterns(t *testing.T) {
	for _, tc := range []struct {
		name    string
		text    string
		wantHit bool
	}{
		{"the hyphenated form the first sweep used", "the FT-710 uses its 6-digit EX wire address", true},
		{"its spelled-out twin", "a syntactically valid six-digit EX wire address", true},
		{"the two-width numeric form", "id is always 4 or 6 ASCII digits", true},
		{"the two-width spelled form", "TargetID is four or six ASCII digits", true},
		{"the emphatic two-width form", "every ID is exactly four or exactly six ASCII digits", true},
		{"the emphatic two-width form, shouted", "reports whether id is exactly FOUR or exactly SIX ASCII digits", true},
		{"the three-width numeric form", "id must be exactly 3, 4 or 6 ASCII digits", false},
		{"the three-width spelled form", "TargetID is three, four or six ASCII digits", false},
		{"the three-width emphatic form", "exactly THREE, exactly FOUR or exactly SIX ASCII digits", false},
		{"a four-digit statement, which was never the stale one", "a Pair-form dialect renders four digits, not six", false},
		{"a range this rule refuses to be, stated as a range", "three exact widths rather than a 3..6 range", false},
		{"prose that names the truncation without naming a width", "the truncated (P1,P2,P3) address it was written to catch", false},
	} {
		got := staleRe.MatchString(blankAll(tc.text, admittedRe))
		if got != tc.wantHit {
			t.Errorf("%s: guard fired = %v, want %v, over %q", tc.name, got, tc.wantHit, tc.text)
		}
	}
}

// TestSettingIDWidthProseSurvivesWrapping pins the normalisation, because
// the statement it exists for is real: app/types.go's TargetID field
// comment wraps mid-phrase, and a line-by-line matcher sees neither half.
func TestSettingIDWidthProseSurvivesWrapping(t *testing.T) {
	src := "package p\n\n// TargetID is the slot's WIRE form (e.g. \"001\") for a channel event, or four or\n" +
		"// six ASCII digits — the EX address in its dialect's wire form — for a\n// settings event.\n"
	norm, lineOf := normaliseForWidthProse(src)
	loc := staleRe.FindStringIndex(blankAll(norm, admittedRe))
	if loc == nil {
		t.Fatalf("the wrapped two-width statement was not found in %q — normalisation has stopped joining wrapped comment lines", norm)
	}
	if got, want := lineOf(loc[0]), 3; got != want {
		t.Errorf("the wrapped statement reported line %d, want %d — the offset-to-line map must name the line the statement STARTS on, which is what the allowlist keys on", got, want)
	}
}

// normaliseForWidthProse returns src with every line's leading whitespace
// and comment marker stripped and all lines joined by single spaces, plus
// a function mapping an offset in that normalised text back to the 1-based
// source line it came from.
//
// Joining is what lets a wrapped statement be matched at all
// (TestSettingIDWidthProseSurvivesWrapping). Mapping back to the line the
// MATCH STARTS on is what lets the allowlist key on a file:line a reader
// can open.
func normaliseForWidthProse(src string) (string, func(int) int) {
	lines := strings.Split(src, "\n")
	var b strings.Builder
	starts := make([]int, 0, len(lines))
	nums := make([]int, 0, len(lines))
	for i, ln := range lines {
		t := strings.TrimLeft(ln, " \t")
		switch {
		case strings.HasPrefix(t, "//"):
			t = strings.TrimSpace(strings.TrimPrefix(t, "//"))
		case strings.HasPrefix(t, "/*"):
			t = strings.TrimSpace(strings.TrimPrefix(t, "/*"))
		case strings.HasPrefix(t, "*"):
			t = strings.TrimSpace(strings.TrimPrefix(t, "*"))
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		starts = append(starts, b.Len())
		nums = append(nums, i+1)
		b.WriteString(t)
	}
	out := b.String()
	return out, func(off int) int {
		idx := sort.Search(len(starts), func(i int) bool { return starts[i] > off })
		if idx == 0 {
			return 1
		}
		return nums[idx-1]
	}
}

// blankAll replaces every match of re in s with spaces of the same length,
// so offsets — and therefore the line map — are preserved exactly.
func blankAll(s string, re *regexp.Regexp) string {
	out := []byte(s)
	for _, loc := range re.FindAllStringIndex(s, -1) {
		for i := loc[0]; i < loc[1]; i++ {
			out[i] = ' '
		}
	}
	return string(out)
}
