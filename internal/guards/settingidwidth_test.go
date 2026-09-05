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
// isSettingIDWidth actually admits — as far as a regexp over prose can
// see, which is exactly the reach TestSettingIDWidthPatterns enumerates.
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
// or exactly six"). Replacing that grep with a list of the five phrasings
// then found repeated the same mistake one level up: a review demonstrated
// that "id is always 4 or 6 digits" — the guarded wording minus the word
// "ASCII" — passed the list green. A guard whose pattern is narrower than
// the prose it guards is precisely the defect it was written to catch, so
// staleRe below matches on the SHAPE of a width statement rather than on
// any list of wordings; see its own comment for the four shapes and the
// samples that pin each.
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
// WHAT IS DELIBERATELY OUT OF SCOPE, and why it is not a hole. MOST other
// packages that say "six-digit" are saying it about one radio's — or one
// dialect FORM's — own EX wire address: core/cat and its per-model
// subpackages (dialectconfig.go calls EXAddressTriple "the six-digit …
// field, the form"), core/driver/<model>, internal/fake*. Each such
// statement is true as written and becomes FALSE if generalised, which is
// the opposite of the drift this guard catches. Counted with the two
// passes below at the time of writing: 66 of them in non-test files
// outside the scoped directories, 144 across every .go file including
// tests. A guard that swept them would need an allowlist longer than the
// rule.
//
// TWO out-of-scope statements are NOT of that kind, and neither is this
// guard's to fix. internal/extable states a six-digit key width three
// times, about the Triple address FORM rather than any radio; that
// package is being widened with a three-digit AddressSingle in this same
// milestone, so keeping those three consistent belongs to that work.
// core/csvio's chirp reader says "a six-digit value could genuinely
// exceed nothing" about a CSV numeric bound — nothing to do with a
// setting ID at all.
//
// _test.go files are out of scope because their width statements belong to
// a fixture's radio — with exactly ONE exception, found by sweeping and
// classifying every test-file hit. core/driver/ft891's settings test
// stated the CONTRACT, not its fixture (its
// TestCloneReadSettings_WalksTheWholeDescriptor doc comment names both
// codeplug.MenuSnapshot.Validate and isSettingIDWidth), so it was widened
// by hand in the same change that widened the rule rather than left for a
// guard that will never look at it. app/settings_test.go pins an FT-710
// read and says so ("a 6-digit TargetID"); that test is deliberately NOT
// generalised to the Kenwood width, because a Kenwood-width test belongs
// with the Kenwood registration rather than retrofitted onto an FT-710
// fixture.
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

// staleRe matches a statement of any OTHER width set. It runs over text
// admittedRe has already blanked, so what it finds is prose that still
// names four-and-six, or six alone, as the rule — or that still counts
// the widths as two.
//
// IT IS MATCHED ON SHAPE, NOT ON WORDING, and that is the point. The
// first version of this guard listed five exact phrasings, and a
// statement as stale as "id is always 4 or 6 digits" — the guarded
// wording minus the word "ASCII" — passed it green. A list of spellings
// guards the spellings; the property this file's doc comment claims to
// guard is a WIDTH SET, so each half generalises over the numeral or the
// count word:
//
//   - six alone, hyphenated, with the numeral or the word, and tolerating
//     a wrap between the hyphen and its noun ("6-\n// digit" normalises
//     to "6- digit");
//   - four-and-six joined by "or" or "to", either spelling, either or
//     both emphasised with "exactly", and named as ASCII digits or as
//     characters;
//   - four-and-six with no noun at all, after "either" or "exactly";
//   - the COUNT word, which a find-and-replace over the numerals leaves
//     behind: two (or "both") widths where the rule now has three.
//
// It is still a regexp over prose and not a parser, so what it pins is
// exactly the twenty-one samples of TestSettingIDWidthPatterns, eight of
// them escapes that the phrasing-list version let through. Read that
// table as the enumeration of the guard's reach.
var staleRe = regexp.MustCompile(`(?i)` +
	`(?:six|6)-[\s]*digit` +
	`|(?:four|4)[\s-]*(?:or|to)[\s-]*(?:exactly[\s-]*)?(?:six|6)[\s-]*(?:ascii[\s-]*)?(?:digit|char)` +
	`|(?:either|exactly)[\s-]+(?:four|4)[\s-]+or[\s-]+(?:exactly[\s-]+)?(?:six|6)` +
	`|(?:two|2|both)[\s-]+(?:exact[\s-]+)?(?:ex[\s-]+address[\s-]+)?widths`)

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
// would miss it. This table IS the guard's reach — staleRe matches on
// shape, but only the shapes enumerated here are pinned, so a claim about
// what the guard catches should be read off these rows and no further.
//
// Every "want a hit" sample is either a real statement this repository
// carried at some point or an escape demonstrated against the
// phrasing-list version of staleRe. The "want no hit" samples are
// statements the repository carries now, or close paraphrases of them,
// with one exception: "TargetID is three, four or six ASCII digits" is a
// spelling the tree does not use, kept because the spelled three-width
// form must not fire if someone writes it.
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

		// The five samples below are the ESCAPES a phrasing list lets
		// through: each states the old two-width rule, or the old
		// six-only one, in a spelling no earlier sweep had used. The
		// first is the one demonstrated live against the phrasing-list
		// pattern (it passed green), and it differs from the guarded
		// wording only by dropping the word "ASCII".
		{"the two-width numeric form with ASCII dropped", "id is always 4 or 6 digits", true},
		{"the two widths named as characters rather than digits", "TargetID is four or six ASCII characters", true},
		{"the two widths stated with 'to' rather than 'or'", "an EX address is 4 to 6 digits", true},
		{"the two widths after 'either', with no noun at all", "every ID is either four or six", true},
		{"the hyphenated form wrapped between hyphen and noun", "a four- or 6- digit ID", true},

		// The three samples below are the COUNT-WORD half of a
		// statement. A find-and-replace over the numerals produces
		// exactly this shape: correct widths, stale arithmetic. The
		// first is the half-updated form demonstrated live against the
		// phrasing-list pattern (it too passed green).
		{"the count word left stale beside corrected numerals", "id is always 3, 4 or 6 ASCII digits — the two EX address widths", true},
		{"the count word as a bare count of widths", "the rule names two exact widths", true},
		{"the count word implied rather than spelled", "an ID may take both widths", true},

		{"the three-width numeric form", "id must be exactly 3, 4 or 6 ASCII digits", false},
		{"the three-width spelled form", "TargetID is three, four or six ASCII digits", false},
		{"the three-width emphatic form", "exactly THREE, exactly FOUR or exactly SIX ASCII digits", false},
		{"the corrected count word, which must not fire", "id must be exactly 3, 4 or 6 ASCII digits — the three EX address widths", false},
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
