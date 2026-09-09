// SPDX-License-Identifier: GPL-3.0-or-later

package ts590_test

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/kw/ts590"
	"github.com/gm5dna/open-rig-programmer/internal/extable"
)

// This file is the TS-590S/TS-590SG Stage 1 cross-check. It binds the three
// independently derived records of EACH of the book's TWO EX charts to one
// another, and binds each generated inventory to the transcription that
// generates it.
//
// THERE ARE TWO CHARTS AND THEY ARE NEVER ONE. The TS-590S/TS-590SG PC
// Control Command Reference Guide prints "EX Command Parameter List (for
// TS-590S)" and "EX Command Parameter List (for TS-590SG)" over COLLIDING
// addresses with different meanings — 000 is Display brightness on the S and
// Firmware Version, read only, on the SG. Every leg below runs twice, once
// per chart, over its own three artefacts;
// TestCrossCheck_TheTwoTablesArePinnedAsTwo is the leg that makes a
// copy-paste between them fail.
//
// The three artefacts per chart, and why there are three:
//
//   - TRANSCRIPTION A — menu590s.csv / menu590sg.csv, each its profile's ONLY
//     generation source. Layout-text-led, PDF-checked, in the ten-column
//     extable shape, parsed here through extable.ParseCSV under the
//     REGISTERED "ts590s"/"ts590sg" profile so that the cross-check reads the
//     same bytes through the same parser the generator does.
//   - TRANSCRIPTION B — testdata/transcription-b-590s.csv and
//     testdata/transcription-b-590sg.csv. Derived PDF-primary at 600 dpi by
//     quarantined agents that never opened this repository, never saw A or a
//     ledger, and were told no row count.
//   - THE PAGE LEDGER — testdata/ledger-590s.csv and ledger-590sg.csv.
//     Derived from the rendered PDF's ruled cells by a further quarantined
//     agent before transcription A existed, and blind to transcription B,
//     which ran alongside it — and the source of each profile's
//     ExpectedRows (88 and 100).
//
// Agreement between three blind derivations is the evidence; this file is
// where that agreement is made mechanical rather than asserted in prose. ANY
// mismatch this file does not itself name and rule is a STOP for orchestrator
// arbitration AGAINST THE PDF, which may correct A, B or a ledger — never
// this test, and never an artefact edited merely to make the test pass. That
// is why every complaint below prints the offending menu number and both
// sides' values: the failure output is the arbitration's input.
//
// Transcription A was given each radio's row COUNT by the orchestrator as a
// single integer (plan §T9a) and nothing else of the ledger — no page, no
// boundary, no name — so the COUNT term of the four-way equality below is
// not blind for A, while the boundary tiling (checkAgainstLedger) and every
// per-row name/digits comparison are.
//
// # What is compared, and what deliberately is not
//
// ADDRESS, NAME and DIGITS, and nothing else. The orchestrator's rulings at
// the legs' close, applied here:
//
//   - NAMES ARE COMPARED WHITESPACE-COLLAPSED, not verbatim, which is the
//     departure from core/cat/ft891/crosscheck_test.go's byte-for-byte rule.
//     The two derivations read the same glyphs through different instruments:
//     A works from the layout text, which collapses the printed inter-word
//     gap, and B works from a 600 dpi render, which preserves it. Eleven rows
//     across the two charts differ by exactly that — "Mic  PF 2 function"
//     against "Mic PF 2 function" and, on the SG, 028's "Shift  change" — and
//     none differs in any other byte. Collapsing runs of whitespace to one
//     space is therefore normalising an INSTRUMENT difference, not papering
//     over a reading difference: a genuinely different word still fails.
//     TestCrossCheck_TheCollapsingIsLoadBearingOnExactlyTheseRows pins WHICH
//     eleven, so the claim in this paragraph is a test rather than prose and
//     a twelfth row quietly acquiring the exemption is a visible event.
//   - THE p4 COLUMN IS NOT COMPARED AT ALL. A's p4 is its own flattening
//     convention for the chart's parameter cells and B carries no such
//     column, so p4 is single-sourced audit here exactly as it is for the
//     FT-891.
//   - THE text FLAG IS NOT COMPARED AS A FLAG; the DIGITS are compared, and
//     the flag sets are asserted against the recorded conventions. See
//     wantATextRows/wantBTextRows below.
//
// # Scope: what no leg here can catch
//
// All three legs read the same printed chart, so a defect PRINTED in the
// chart is transcribed faithfully by all three and is invisible to every
// comparison below. No TS-590S or TS-590SG has ever been asked anything by
// this project, so nothing here is hardware-verified and nothing here claims
// to be.

const (
	// The quarantined artefacts, relative to the package directory, which is
	// the working directory for `go test`.
	transcriptionBSPath  = "testdata/transcription-b-590s.csv"
	transcriptionBSGPath = "testdata/transcription-b-590sg.csv"
	ledgerSPath          = "testdata/ledger-590s.csv"
	ledgerSGPath         = "testdata/ledger-590sg.csv"

	// The two artefacts' exact header rows, pinned so that a schema change
	// fails LOUDLY here rather than being silently misparsed into false
	// agreement.
	transcriptionBHeader = "menu_number,name,digits,text"
	pageLedgerHeader     = "p1,first_menu_number,first_name,last_menu_number,last_name,row_count,pdf_page,visual_anchor"

	transcriptionBFields = 4
	pageLedgerFields     = 8
)

// The two-tables pin, fully hardcoded. These literals came from the CHART and
// are written out here rather than derived from an artefact, because a pin
// computed from the thing it pins proves nothing. The legs below then derive
// the same facts from the transcriptions and the generated inventories and
// require the derivations to agree, so a silent change is caught from both
// directions at once.
const (
	// Address 000 is the collision that names the whole hazard: the same
	// three digits, two different functions, one book (590:569 against
	// 590:749).
	name000OnS  = "Display brightness"
	name000OnSG = "Firmware Version"
	// The S chart stops at 087 and the SG chart runs on to 099. An S
	// inventory carrying an address above 087 is the SG's list wearing the
	// S's name.
	lastAddrOnS  = 87
	lastAddrOnSG = 99
	// sharedAddrCount is 088: every address the S chart prints also exists
	// on the SG chart — arithmetically forced, since the S runs 000..087
	// contiguously and the SG 000..099, so this is little more than a
	// restatement of the S's row count. sharedAddrsWithEqualNames is the
	// leg that earns its place and is what makes the pair dangerous: it is
	// asserted to be ZERO — not one of those 88 addresses means the same
	// thing on both radios, so an address-keyed copy-paste between the two
	// CSVs changes every row it touches.
	sharedAddrCount           = 88
	sharedAddrsWithEqualNames = 0
	// namesCommonToBothCharts is the other half of the same fact: 86 of the
	// two charts' function names appear on BOTH lists — at DIFFERENT
	// addresses. The lists are one catalogue renumbered, which is exactly
	// why borrowing by address is a defect rather than an approximation.
	// (The renumbering is a plain shift of two for 000..013 only; from S 014
	// onwards the SG inserts and splits rows, so no single offset maps the
	// two.)
	namesCommonToBothCharts = 86
)

// frozenEvidenceSHA256 is the freeze, transcribed from the commit message of
// a1e3779 ("core/kw: import the quarantined evidence legs — L×3 ledgers, B×3
// transcriptions, G geometry"), whose "SHA-256 of every imported file:" block
// records one hash per artefact.
//
// The .md companions are in here with the CSVs because they are not
// commentary about them: they are the derivation records this file cites for
// the text-flag conventions it declines to compare and for the glyph
// conventions it collapses, and an artefact whose stated method had been
// quietly rewritten would be as corrupt as one whose rows had. The TS-480's
// three artefacts are frozen by core/kw/ts480/crosscheck_test.go and leg G's
// fifteen by core/kw/golden_test.go; between the three files every artefact
// a1e3779 froze is pinned by whichever file reads it.
var frozenEvidenceSHA256 = map[string]string{
	"transcription-b-590s.csv":  "d341142188d3a40f2bdcc24eb9c9069ae3be0c0f4a9904d4653bf548f47c73ee",
	"transcription-b-590s.md":   "9c635deefbf38431006e1a94ad09d0f2537ad50cfde2adc86fcf612432a4ffe0",
	"transcription-b-590sg.csv": "37ab7cd7bf78f3e6644a915e76448913cbd9931e963b719d83355d2bdaf33c9e",
	"transcription-b-590sg.md":  "b7c49240a8fdda1e86e6bb39629747b718a5513fb55643764ce5420c9be3a94c",
	"ledger-590s.csv":           "0c947121d2d9279169b0d8f33807dbf82d62f62371159ab754b9f3ac3127b4cb",
	"ledger-590s.md":            "19e58ea810a159f9a1c96aab1c6f7f47cd7b90b506eb461aad9e0c94f7cbd0f5",
	"ledger-590sg.csv":          "8e16d07ad12eb3ffa511643d73facab975f5a9e67d8cf30d2809cdc116ddfb6f",
	"ledger-590sg.md":           "ca9e8162c0f0858a444144d20b043833d7a2bcec0dd04e946fb41476f2f4f1e0",
}

// TestQuarantinedEvidenceFrozen recomputes each quarantined artefact's
// SHA-256 and compares it with the value commit a1e3779 recorded, so that the
// freeze is self-enforcing in CI rather than a fact recoverable only by git
// archaeology. Every leg of the cross-check below reads bytes this test has
// vouched for.
//
// The second half is the one that catches the interesting case: every file
// this package's testdata directory holds must be covered by the map above,
// so a leg could not be re-pointed at an unfrozen artefact and still look
// bound.
func TestQuarantinedEvidenceFrozen(t *testing.T) {
	for name, want := range frozenEvidenceSHA256 {
		path := filepath.Join("testdata", name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading quarantined artefact %s: %v", path, err)
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != want {
			t.Errorf("FREEZE BROKEN — %s has changed since commit a1e3779.\n"+
				"  recorded SHA-256 %s\n"+
				"  present  SHA-256 %s\n"+
				"This artefact is quarantined evidence: it is never regenerated and\n"+
				"never edited to satisfy a test. Restore it from the repository root\n"+
				"with `git checkout a1e3779 -- core/kw/ts590/testdata/%s` and report\n"+
				"the change.",
				path, want, got, name)
		}
	}

	present, err := filepath.Glob(filepath.Join("testdata", "*"))
	if err != nil {
		t.Fatalf("globbing testdata: %v", err)
	}
	if len(present) == 0 {
		t.Fatal("no artefacts found in testdata — the evidence legs are missing")
	}
	for _, path := range present {
		if _, ok := frozenEvidenceSHA256[filepath.Base(path)]; !ok {
			t.Errorf("%s has no recorded SHA-256: every quarantined artefact in this package's testdata must be frozen by a commit that records its hash", path)
		}
	}
}

// chartRow is the tuple both transcriptions carry for a menu number. The
// address is the map key: pair 1's three Kenwood profiles — ts590sProfile,
// ts590sgProfile and ts480Profile — register AddressSingle, so the chart's
// three-digit Menu number IS the whole address and P2/P3 are zero on every
// row (checked separately, against the profile's own policy). Pair 2's two,
// ts890sProfile and ts990sProfile, register AddressGrouped with a
// five-character address instead; this package never looks up either.
type chartRow struct {
	Name   string
	Digits int
	Text   bool
}

func (r chartRow) String() string {
	return fmt.Sprintf("name=%q digits=%d text=%v", r.Name, r.Digits, r.Text)
}

// ledgerRow is one ledger row's bindable content. THE LEDGERS ARE PER PDF
// PAGE, not per address group: this family's chart prints no group labels and
// no address hierarchy at all, so the only boundary a reader of the rendered
// page can see is where the page ends. The p1 column is therefore the PDF
// page number, pdf_page repeats it, and visual_anchor is the deriver's prose
// description of the page's first and last ruled bands — read (the column
// count is pinned, and pdf_page is bound to p1 below) but not otherwise
// bound.
type ledgerRow struct {
	PDFPage     int
	PDFPageCell string
	First       int
	FirstName   string
	Last        int
	LastName    string
	RowCount    int
}

// chart is one radio row's three legs plus the labels a complaint needs.
type chart struct {
	label        string
	aPath        string
	bPath        string
	ledgerPath   string
	expectedRows int
	a            map[int]chartRow
	b            map[int]chartRow
	ledger       []ledgerRow
}

// collapseSpace replaces every run of whitespace with a single space and
// trims the ends. It is the ONE normalisation this file applies to a name,
// and only to a name; see the file comment for why an instrument difference
// is folded here and a reading difference is not.
var whitespaceRun = regexp.MustCompile(`\s+`)

func collapseSpace(s string) string {
	return strings.TrimSpace(whitespaceRun.ReplaceAllString(s, " "))
}

// crossCheck compares one chart's three legs and RETURNS one complaint per
// disagreement rather than reporting through *testing.T.
//
// The return value is what makes the four red proofs below possible: a
// falsification has to observe that the comparison bites, and a comparison
// that reported straight into t could only be falsified by making the test
// fail. Everything here is deterministic and ordered, so a complaint list is
// stable between runs.
func crossCheck(c chart) []string {
	var out []string
	add := func(format string, args ...any) { out = append(out, fmt.Sprintf(format, args...)) }

	// Both directions, separately, so a failure says WHICH artefact is
	// missing the menu number rather than merely that the sets differ.
	for _, m := range sortedAddrs(c.a) {
		if _, in := c.b[m]; !in {
			add("%s: menu %03d is in transcription A (%s) but NOT in transcription B (%s): A has %s", c.label, m, c.aPath, c.bPath, c.a[m])
		}
	}
	for _, m := range sortedAddrs(c.b) {
		if _, in := c.a[m]; !in {
			add("%s: menu %03d is in transcription B (%s) but NOT in transcription A (%s): B has %s", c.label, m, c.bPath, c.aPath, c.b[m])
		}
	}

	// The tuple, field by field: NAME whitespace-collapsed and DIGITS. The
	// text flag is deliberately absent — see the file comment and
	// TestCrossCheck_TextFlagsAreTheRecordedConventions.
	for _, m := range sortedAddrs(c.a) {
		bv, in := c.b[m]
		if !in {
			continue // already reported above
		}
		av := c.a[m]
		if an, bn := collapseSpace(av.Name), collapseSpace(bv.Name); an != bn {
			add("%s: menu %03d: the NAME differs between the transcriptions (compared whitespace-collapsed):\n  A (%s): %q\n  B (%s): %q",
				c.label, m, c.aPath, av.Name, c.bPath, bv.Name)
		}
		if av.Digits != bv.Digits {
			add("%s: menu %03d (%q): the DIGITS differ between the transcriptions:\n  A (%s): %d\n  B (%s): %d",
				c.label, m, av.Name, c.aPath, av.Digits, c.bPath, bv.Digits)
		}
	}

	out = append(out, checkAgainstLedger(c, "transcription A", c.aPath, c.a)...)
	out = append(out, checkAgainstLedger(c, "transcription B", c.bPath, c.b)...)

	// One four-way equality, reported whole: which of the four moved is the
	// first question arbitration asks. Nothing in internal/extable pins
	// ExpectedRows' VALUE for this family — that registration test covers
	// the four Yaesu profiles only. For the TS-590S and TS-590SG, ExpectedRows
	// is read from the registered profile rather than re-typed here, and what
	// actually holds 88 and 100 to the evidence is this leg together with the
	// staleness tests' length assertions: TestEXItemsS_HasExpectedRows
	// (ts590/staleness590s_test.go) and
	// TestEXItemsSG_LengthIsTheProfilesExpectedRows
	// (ts590/staleness590sg_test.go).
	sum := 0
	for _, l := range c.ledger {
		sum += l.RowCount
	}
	if len(c.a) != len(c.b) || len(c.a) != sum || len(c.a) != c.expectedRows {
		add("%s: row totals disagree: transcription A = %d, transcription B = %d, ledger row_count sum = %d, profile ExpectedRows = %d",
			c.label, len(c.a), len(c.b), sum, c.expectedRows)
	}
	return out
}

// checkAgainstLedger binds one transcription to the page ledger.
//
// THE LEDGER TILES THE ADDRESS SEQUENCE. Because the boundary is the printed
// page and neither transcription carries a page column, the binding is not a
// join on a key: the ledger's rows, taken in ascending PDF-page order, must
// consume the transcription's ascending addresses in contiguous runs of
// row_count, each run opening at first_menu_number and closing at
// last_menu_number with the recorded names, and the last run must exhaust the
// transcription. That is a stronger statement than a per-page count would be
// — it pins the ORDER and the boundaries as well as the sizes — and it is
// what the ledger's own visual_anchor prose describes.
func checkAgainstLedger(c chart, srcLabel, srcPath string, rows map[int]chartRow) []string {
	var out []string
	add := func(format string, args ...any) { out = append(out, fmt.Sprintf(format, args...)) }

	addrs := sortedAddrs(rows)
	i := 0
	for _, l := range c.ledger {
		if i+l.RowCount > len(addrs) {
			add("%s: PDF page %d: the ledger (%s) records row_count %d starting at %s's address %d, but only %d addresses remain",
				c.label, l.PDFPage, c.ledgerPath, l.RowCount, srcLabel, i, len(addrs)-i)
			return out
		}
		seg := addrs[i : i+l.RowCount]
		i += l.RowCount

		first, last := seg[0], seg[len(seg)-1]
		if first != l.First {
			add("%s: PDF page %d: %s (%s) opens the page's run at menu %03d, the ledger (%s) records first_menu_number %03d",
				c.label, l.PDFPage, srcLabel, srcPath, first, c.ledgerPath, l.First)
		} else if got, want := collapseSpace(rows[first].Name), collapseSpace(l.FirstName); got != want {
			add("%s: PDF page %d: %s (%s) names menu %03d %q, the ledger (%s) records first_name %q",
				c.label, l.PDFPage, srcLabel, srcPath, first, rows[first].Name, c.ledgerPath, l.FirstName)
		}
		if last != l.Last {
			add("%s: PDF page %d: %s (%s) closes the page's run at menu %03d, the ledger (%s) records last_menu_number %03d",
				c.label, l.PDFPage, srcLabel, srcPath, last, c.ledgerPath, l.Last)
		} else if got, want := collapseSpace(rows[last].Name), collapseSpace(l.LastName); got != want {
			add("%s: PDF page %d: %s (%s) names menu %03d %q, the ledger (%s) records last_name %q",
				c.label, l.PDFPage, srcLabel, srcPath, last, rows[last].Name, c.ledgerPath, l.LastName)
		}
	}
	if i != len(addrs) {
		add("%s: the ledger's (%s) row_counts consume %d of %s's (%s) %d addresses; the pages must tile the chart exactly",
			c.label, c.ledgerPath, i, srcLabel, srcPath, len(addrs))
	}
	return out
}

// TestCrossCheck_A_B_Ledger is the milestone's A = B = ledger = ExpectedRows
// leg, run once per chart. Zero complaints is the pass condition.
func TestCrossCheck_A_B_Ledger(t *testing.T) {
	for _, c := range []chart{loadChartS(t), loadChartSG(t)} {
		t.Run(c.label, func(t *testing.T) {
			for _, complaint := range crossCheck(c) {
				t.Errorf("CROSS-CHECK DISAGREEMENT — THIS IS A STOP FOR ARBITRATION AGAINST THE PDF, not a test to adjust:\n%s", complaint)
			}
		})
	}
}

// TestCrossCheck_TheChartShapePolicies asserts of both transcriptions the
// three facts the registered profiles declare about this family's chart
// shape, so that the inventory-vs-A leg's claims about the GENERATED items
// rest on a source that was checked for the same things.
//
// ParseCSV already refuses a non-blank label and a non-zero P2/P3 under
// LabelsAbsent and AddressSingle, so this leg is not the first line of
// defence for transcription A; it is here because a claim about a generated
// file is only worth making if the source it was generated from carries it
// too.
func TestCrossCheck_TheChartShapePolicies(t *testing.T) {
	for _, tc := range []struct {
		label   string
		profile string
	}{
		{"TS-590S", "ts590s"},
		{"TS-590SG", "ts590sg"},
	} {
		t.Run(tc.label, func(t *testing.T) {
			p := lookupProfile(t, tc.profile)
			for _, r := range parseTranscriptionA(t, p) {
				if r.P1Label != "" || r.P2Label != "" {
					t.Errorf("menu %03d (%q): transcription A (%s) carries labels p1_label=%q p2_label=%q, but this chart prints no label columns (LabelsAbsent)", r.P1, r.Name, p.ManualCSV, r.P1Label, r.P2Label)
				}
				if r.P2 != 0 || r.P3 != 0 {
					t.Errorf("menu %03d (%q): transcription A (%s) carries p2=%d p3=%d, but this radio's EX address is a SINGLE component and both must be 0 (AddressSingle)", r.P1, r.Name, p.ManualCSV, r.P2, r.P3)
				}
			}
		})
	}
}

// The text-flag conventions, recorded as literals.
//
// THE FLAG IS NOT A DATUM THE TWO LEGS AGREE ON, AND THE ORCHESTRATOR RULED
// THAT IT NEED NOT BE. A and B were briefed with different definitions of
// "text row" and each applied its own consistently, so the flag records a
// CONVENTION and the DIGITS record the chart. The digits are compared row for
// row in crossCheck; the flag sets are pinned here so that a change to either
// convention is a visible, deliberate act rather than a silent drift.
//
//   - A marks the chart's FREE-TEXT row — the one a transcriber must stop at
//     — and nothing else: menu 087 on the S, the same "up to 8 ASCII
//     characters" string renumbered to 001 on the SG (590:741, 590:750). The
//     SG's 000 "Version information (4 ASCII characters) read only" is
//     transcribed digits=4 text=false BY DESIGN DECISION, recorded in
//     menu590sg.csv's own "THE VERSION ROW IS NOT A TEXT ROW" block and
//     following the FT-891's treatment of its MAIN VERSION row; the profile's
//     TextWidths names ONE width and this chart prints strings of two.
//   - B applied a STRUCTURAL test — "prose printed across the grid instead of
//     cells" — and so also flagged the SG's 000
//     (testdata/transcription-b-590sg.md, "TEXT rows (prose across the grid)
//     — 2"). Its own record flags the divergence rather than resolving it.
//
// The one address where the flags differ is therefore 000 on the SG, and both
// legs record DIGITS 4 there — which is the datum, and which crossCheck
// already binds.
var (
	wantATextRowsS   = []int{87}
	wantBTextRowsS   = []int{87}
	wantATextRowsSG  = []int{1}
	wantBTextRowsSG  = []int{0, 1}
	textFlagDiffersS = []int{}
	// The SG's 000 and nothing else.
	textFlagDiffersSG = []int{0}
)

// TestCrossCheck_TextFlagsAreTheRecordedConventions pins both legs' text-row
// sets and requires every disagreement between them to be one the rulings
// name AND to carry equal DIGITS on both sides.
func TestCrossCheck_TextFlagsAreTheRecordedConventions(t *testing.T) {
	for _, tc := range []struct {
		c          chart
		wantA      []int
		wantB      []int
		wantDiffer []int
	}{
		{loadChartS(t), wantATextRowsS, wantBTextRowsS, textFlagDiffersS},
		{loadChartSG(t), wantATextRowsSG, wantBTextRowsSG, textFlagDiffersSG},
	} {
		t.Run(tc.c.label, func(t *testing.T) {
			if got := textRows(tc.c.a); !equalInts(got, tc.wantA) {
				t.Errorf("transcription A (%s) marks text rows %v, the recorded convention is %v", tc.c.aPath, got, tc.wantA)
			}
			if got := textRows(tc.c.b); !equalInts(got, tc.wantB) {
				t.Errorf("transcription B (%s) marks text rows %v, the recorded convention is %v", tc.c.bPath, got, tc.wantB)
			}

			var differ []int
			for _, m := range sortedAddrs(tc.c.a) {
				bv, in := tc.c.b[m]
				if !in {
					continue
				}
				if tc.c.a[m].Text != bv.Text {
					differ = append(differ, m)
					if tc.c.a[m].Digits != bv.Digits {
						t.Errorf("menu %03d (%q): the two legs disagree about the text FLAG *and* about the DIGITS (A %d, B %d) — only the flag is a recorded convention; a digits difference is a STOP",
							m, tc.c.a[m].Name, tc.c.a[m].Digits, bv.Digits)
					}
				}
			}
			if !equalInts(differ, tc.wantDiffer) {
				t.Errorf("the text flag differs between the legs at %v, the recorded rulings name %v — a new divergence is a STOP for arbitration, not a pin to widen", differ, tc.wantDiffer)
			}
		})
	}
}

// The rows where the whitespace collapsing is LOAD-BEARING — the ones on
// which A and B differ verbatim and agree only once collapsed.
//
// Five on the S and six on the SG: the chart's five "Mic  PF …" rows, plus
// the SG's 028 "… Low Cut and Width/ Shift  change (SSB)". Every
// one is a doubled space inside a printed name, which is the instrument
// difference the file comment describes — A reads the layout text, which
// collapses the printed inter-word gap, and B reads a 600 dpi render, which
// keeps it.
var (
	whitespaceOnlyDiffsS  = []int{82, 83, 84, 85, 86}
	whitespaceOnlyDiffsSG = []int{28, 95, 96, 97, 98, 99}
)

// TestCrossCheck_TheCollapsingIsLoadBearingOnExactlyTheseRows measures the
// normalisation's whole extent, which crossCheck by construction cannot: it
// compares collapsed names, so it can never report how much the collapsing
// hid.
//
// Two assertions, and the second is the one that matters. The set of rows
// where the two legs differ VERBATIM is pinned, so the exemption cannot
// silently spread; and every such row must agree once collapsed, so a row
// that differed verbatim AND after collapsing would be reported here as a
// reading difference rather than merely counted — the same STOP crossCheck
// raises, stated from the other side.
func TestCrossCheck_TheCollapsingIsLoadBearingOnExactlyTheseRows(t *testing.T) {
	for _, tc := range []struct {
		c    chart
		want []int
	}{
		{loadChartS(t), whitespaceOnlyDiffsS},
		{loadChartSG(t), whitespaceOnlyDiffsSG},
	} {
		t.Run(tc.c.label, func(t *testing.T) {
			var differ []int
			for _, m := range sortedAddrs(tc.c.a) {
				bv, in := tc.c.b[m]
				if !in {
					continue
				}
				av := tc.c.a[m]
				if av.Name == bv.Name {
					continue
				}
				differ = append(differ, m)
				if collapseSpace(av.Name) != collapseSpace(bv.Name) {
					t.Errorf("menu %03d: the two legs differ in a name byte the collapsing does NOT normalise — this is a reading difference and a STOP:\n  A (%s): %q\n  B (%s): %q",
						m, tc.c.aPath, av.Name, tc.c.bPath, bv.Name)
				}
			}
			if !equalInts(differ, tc.want) {
				t.Errorf("the legs differ verbatim at %v, the recorded instrument difference is %v — the collapsing must not quietly acquire a new row", differ, tc.want)
			}
		})
	}
}

// TestCrossCheck_TheTwoTablesArePinnedAsTwo is the Tier 4b non-borrowing rule
// applied to a menu chart: the S's inventory and the SG's are two tables, and
// a copy-paste between them fails.
//
// It runs against BOTH the transcriptions and the two GENERATED inventories,
// because the defect it guards against — one chart's rows reaching the other
// radio — can be introduced at either place, and a pin that only read the
// CSVs would miss a generated file built from the wrong source.
func TestCrossCheck_TheTwoTablesArePinnedAsTwo(t *testing.T) {
	s, sg := loadChartS(t), loadChartSG(t)
	sItems, sgItems := ts590.EXItemsS(), ts590.EXItemsSG()

	t.Run("address_000_resolves_differently_on_the_two_rows", func(t *testing.T) {
		for _, tc := range []struct {
			label string
			rows  map[int]chartRow
			items []kw.EXItem
			path  string
			want  string
		}{
			{"TS-590S", s.a, sItems, s.aPath, name000OnS},
			{"TS-590SG", sg.a, sgItems, sg.aPath, name000OnSG},
		} {
			row, in := tc.rows[0]
			if !in {
				t.Errorf("%s: transcription A (%s) has no menu 000", tc.label, tc.path)
			} else if got := collapseSpace(row.Name); got != tc.want {
				t.Errorf("%s: transcription A (%s) names menu 000 %q, the chart pin says %q", tc.label, tc.path, row.Name, tc.want)
			}
			if len(tc.items) == 0 {
				t.Fatalf("%s: the generated inventory is empty", tc.label)
			}
			if tc.items[0].Addr.P1 != 0 {
				t.Errorf("%s: the generated inventory's first item is menu %03d, want 000", tc.label, tc.items[0].Addr.P1)
			} else if got := collapseSpace(tc.items[0].Name); got != tc.want {
				t.Errorf("%s: the generated inventory names menu 000 %q, the chart pin says %q — this is the copy-paste the two-table rule exists to catch", tc.label, tc.items[0].Name, tc.want)
			}
		}
	})

	t.Run("the_S_chart_stops_at_087_and_the_SG_runs_to_099", func(t *testing.T) {
		for _, tc := range []struct {
			label string
			rows  map[int]chartRow
			items []kw.EXItem
			path  string
			want  int
		}{
			{"TS-590S", s.a, sItems, s.aPath, lastAddrOnS},
			{"TS-590SG", sg.a, sgItems, sg.aPath, lastAddrOnSG},
		} {
			addrs := sortedAddrs(tc.rows)
			if got := addrs[len(addrs)-1]; got != tc.want {
				t.Errorf("%s: transcription A (%s) runs to menu %03d, the chart pin says %03d",
					tc.label, tc.path, got, tc.want)
			}
			if len(tc.items) == 0 {
				t.Fatalf("%s: the generated inventory is empty", tc.label)
			}
			if got := int(tc.items[len(tc.items)-1].Addr.P1); got != tc.want {
				t.Errorf("%s: the generated inventory runs to menu %03d, the chart pin says %03d", tc.label, got, tc.want)
			}
		}
	})

	t.Run("no_shared_address_means_the_same_thing_on_both_radios", func(t *testing.T) {
		shared, equal := 0, 0
		for _, m := range sortedAddrs(s.a) {
			sgRow, in := sg.a[m]
			if !in {
				continue
			}
			shared++
			if collapseSpace(s.a[m].Name) == collapseSpace(sgRow.Name) {
				equal++
				t.Errorf("menu %03d names %q on BOTH charts; the two lists collide on every address they share and agree on none of them, so an equal name here is either a transcription defect or a copy-paste between %s and %s",
					m, s.a[m].Name, s.aPath, sg.aPath)
			}
		}
		if shared != sharedAddrCount {
			t.Errorf("the two charts share %d addresses, the chart pin says %d", shared, sharedAddrCount)
		}
		if equal != sharedAddrsWithEqualNames {
			t.Errorf("%d shared addresses carry the same name on both charts, the chart pin says %d", equal, sharedAddrsWithEqualNames)
		}
	})

	t.Run("the_two_lists_are_one_catalogue_renumbered", func(t *testing.T) {
		// The other half of the same fact, and the reason the collision is
		// dangerous rather than obvious: most of the S's function names DO
		// appear on the SG's list — at different addresses. A borrower
		// working by name would find its row; a borrower working by address
		// silently gets a different setting.
		sNames := map[string]bool{}
		for _, r := range s.a {
			sNames[collapseSpace(r.Name)] = true
		}
		common := 0
		for _, r := range sg.a {
			if sNames[collapseSpace(r.Name)] {
				common++
			}
		}
		if common != namesCommonToBothCharts {
			t.Errorf("%d of the SG chart's names also appear on the S chart, the chart pin says %d", common, namesCommonToBothCharts)
		}
	})
}

// TestCrossCheck_InventoryAgainstTranscriptionA binds each GENERATED
// inventory to the transcription it is generated from.
//
// The staleness tests already re-render each generated file from its CSV and
// byte-compare it, which catches drift between the two. This test binds
// something else: what the inventory MEANS once loaded — that the rows are
// the chart's rows, in address order, carrying this family's shape (P2 and P3
// zero, no labels, the observation sentinels) and this chart's widths. It
// reaches each inventory through the package accessor, the route every other
// consumer uses.
func TestCrossCheck_InventoryAgainstTranscriptionA(t *testing.T) {
	for _, tc := range []struct {
		label   string
		profile string
		items   []kw.EXItem
	}{
		{"TS-590S", "ts590s", ts590.EXItemsS()},
		{"TS-590SG", "ts590sg", ts590.EXItemsSG()},
	} {
		t.Run(tc.label, func(t *testing.T) {
			p := lookupProfile(t, tc.profile)
			a := transcriptionA(t, p)
			order := sortedAddrs(a)

			if len(tc.items) != len(a) {
				t.Fatalf("the generated inventory holds %d items, transcription A (%s) holds %d rows", len(tc.items), p.ManualCSV, len(a))
			}
			// The items must be in ascending P1 order in their own right,
			// not merely be a permutation that happens to match A once
			// sorted: the generated file's doc comment claims the sort, so
			// the claim is pinned here rather than assumed.
			for i := 1; i < len(tc.items); i++ {
				if prev, cur := tc.items[i-1].Addr.P1, tc.items[i].Addr.P1; prev >= cur {
					t.Errorf("the generated inventory is not in ascending P1 order at index %d: %03d follows %03d", i, cur, prev)
				}
			}
			for i, want := range order {
				got := tc.items[i]
				if int(got.Addr.P1) != want {
					t.Errorf("inventory item %d is menu %03d, transcription A (%s) has %03d there", i, got.Addr.P1, p.ManualCSV, want)
					continue
				}
				row := a[want]
				if got.Name != row.Name {
					t.Errorf("menu %03d: the inventory's Name is %q, transcription A (%s) has %q", want, got.Name, p.ManualCSV, row.Name)
				}
				if got.Digits != row.Digits {
					t.Errorf("menu %03d (%q): the inventory's Digits is %d, transcription A (%s) has %d", want, row.Name, got.Digits, p.ManualCSV, row.Digits)
				}
				if got.Text != row.Text {
					t.Errorf("menu %03d (%q): the inventory's Text is %v, transcription A (%s) has %v", want, row.Name, got.Text, p.ManualCSV, row.Text)
				}
				if got.Addr.P2 != 0 || got.Addr.P3 != 0 {
					t.Errorf("menu %03d (%q): the inventory carries P2=%d P3=%d, but this radio's EX address is a SINGLE component and both must be 0 (AddressSingle)", want, row.Name, got.Addr.P2, got.Addr.P3)
				}
				if got.P1Label != "" || got.P2Label != "" {
					t.Errorf("menu %03d (%q): the inventory carries labels P1Label=%q P2Label=%q, but this chart prints no label columns (LabelsAbsent)", want, row.Name, got.P1Label, got.P2Label)
				}
				// ObservationsAbsent: no TS-590 has ever been asked
				// anything by this project, so both observation fields must
				// carry their absence sentinels. A non-zero one here would
				// be a hardware claim nothing supports.
				if got.ObservedReadWidth != 0 || got.ObservedReadShape != "" {
					t.Errorf("menu %03d (%q): the inventory carries ObservedReadWidth=%d ObservedReadShape=%q, but this profile registers ObservationsAbsent and no TS-590 has ever been asked anything",
						want, row.Name, got.ObservedReadWidth, got.ObservedReadShape)
				}
			}
		})
	}
}

// The four falsifications the milestone requires, one per leg, fired against
// an IN-MEMORY mutation of the loaded evidence. No artefact is ever written.
//
// Without these the cross-check's green is unfalsifiable: a comparison that
// silently compared nothing would pass every run above just as happily.

// TestRedProof_DroppingARowFromA is caught.
func TestRedProof_DroppingARowFromA(t *testing.T) {
	for _, c := range []chart{loadChartS(t), loadChartSG(t)} {
		t.Run(c.label, func(t *testing.T) {
			// The LAST row of the chart, so the falsification also moves the
			// final page's boundary and the four-way total at once.
			victim := sortedAddrs(c.a)[len(c.a)-1]
			c.a = withoutAddr(c.a, victim)
			requireComplaint(t, crossCheck(c), fmt.Sprintf("menu %03d", victim))
		})
	}
}

// TestRedProof_DroppingARowFromB is caught.
func TestRedProof_DroppingARowFromB(t *testing.T) {
	for _, c := range []chart{loadChartS(t), loadChartSG(t)} {
		t.Run(c.label, func(t *testing.T) {
			// The FIRST row this time, so the two proofs between them
			// exercise both ends of the tiling.
			victim := sortedAddrs(c.b)[0]
			c.b = withoutAddr(c.b, victim)
			requireComplaint(t, crossCheck(c), fmt.Sprintf("menu %03d", victim))
		})
	}
}

// TestRedProof_PerturbingAWidth is caught — the leg that no row-count check
// could ever see, because the counts still agree.
func TestRedProof_PerturbingAWidth(t *testing.T) {
	for _, c := range []chart{loadChartS(t), loadChartSG(t)} {
		t.Run(c.label, func(t *testing.T) {
			victim := sortedAddrs(c.a)[0]
			row := c.a[victim]
			row.Digits++
			c.a = withAddr(c.a, victim, row)
			requireComplaint(t, crossCheck(c), "the DIGITS differ")
		})
	}
}

// TestRedProof_PerturbingTheLedgerSum is caught. row_count is what
// ExpectedRows was derived from, so a ledger that quietly disagreed with
// itself would take the profile's bound with it.
//
// BOTH consequences are required, not either: the four-way total moves, and
// the page the count belongs to stops tiling. Requiring only the total would
// leave checkAgainstLedger unfalsified — asserted here because the first
// draft of these proofs did exactly that, and unplugging the two
// checkAgainstLedger calls from crossCheck left every red proof green.
func TestRedProof_PerturbingTheLedgerSum(t *testing.T) {
	for _, c := range []chart{loadChartS(t), loadChartSG(t)} {
		t.Run(c.label, func(t *testing.T) {
			ledger := append([]ledgerRow(nil), c.ledger...)
			ledger[0].RowCount++
			c.ledger = ledger
			complaints := crossCheck(c)
			requireComplaint(t, complaints, "row totals disagree")
			requireComplaint(t, complaints, "closes the page's run at menu")
		})
	}
}

// TestRedProof_PerturbingALedgerBoundaryName is caught. It is the leg no
// count can see: the page boundaries and their names still tile, and only the
// NAME the ledger recorded for a boundary row has moved — which is the ledger
// leg's own content rather than a restatement of the totals.
func TestRedProof_PerturbingALedgerBoundaryName(t *testing.T) {
	for _, c := range []chart{loadChartS(t), loadChartSG(t)} {
		t.Run(c.label, func(t *testing.T) {
			ledger := append([]ledgerRow(nil), c.ledger...)
			ledger[0].FirstName += " (perturbed)"
			c.ledger = ledger
			requireComplaint(t, crossCheck(c), "the ledger ("+c.ledgerPath+") records first_name")
		})
	}
}

// requireComplaint asserts the cross-check bit, and that at least one of its
// complaints names the mutation — a leg that failed for an unrelated reason
// would prove nothing about the leg under falsification.
func requireComplaint(t *testing.T, complaints []string, want string) {
	t.Helper()
	if len(complaints) == 0 {
		t.Fatalf("the cross-check reported NO complaint against the falsified evidence: the leg that should have caught %q is not binding anything", want)
	}
	for _, c := range complaints {
		if strings.Contains(c, want) {
			return
		}
	}
	t.Errorf("the cross-check complained, but no complaint names %q; it caught something else:\n  %s", want, strings.Join(complaints, "\n  "))
}

// --- loaders -------------------------------------------------------------

func loadChartS(t *testing.T) chart {
	t.Helper()
	p := lookupProfile(t, "ts590s")
	return chart{
		label:        "TS-590S",
		aPath:        p.ManualCSV,
		bPath:        transcriptionBSPath,
		ledgerPath:   ledgerSPath,
		expectedRows: p.ExpectedRows,
		a:            transcriptionA(t, p),
		b:            loadTranscriptionB(t, transcriptionBSPath),
		ledger:       loadPageLedger(t, ledgerSPath),
	}
}

func loadChartSG(t *testing.T) chart {
	t.Helper()
	p := lookupProfile(t, "ts590sg")
	return chart{
		label:        "TS-590SG",
		aPath:        p.ManualCSV,
		bPath:        transcriptionBSGPath,
		ledgerPath:   ledgerSGPath,
		expectedRows: p.ExpectedRows,
		a:            transcriptionA(t, p),
		b:            loadTranscriptionB(t, transcriptionBSGPath),
		ledger:       loadPageLedger(t, ledgerSGPath),
	}
}

// lookupProfile takes the REGISTERED profile, not a literal of this file's
// own: the whole point is to read A exactly as the generator reads it, under
// the same digit bounds and the same address policy.
func lookupProfile(t *testing.T, name string) extable.Profile {
	t.Helper()
	p, ok := extable.Lookup(name)
	if !ok {
		t.Fatalf("extable.Lookup(%q): the profile is not registered", name)
	}
	return p
}

// parseTranscriptionA reads one chart's CSV through extable.ParseCSV — the
// same parser, the same bounds, the same duplicate-address refusal and the
// same AddressSingle rule the generator runs.
func parseTranscriptionA(t *testing.T, p extable.Profile) []extable.Row {
	t.Helper()
	data, err := os.ReadFile(p.ManualCSV)
	if err != nil {
		t.Fatalf("reading transcription A (%s): %v", p.ManualCSV, err)
	}
	rows, err := extable.ParseCSV(p, data)
	if err != nil {
		t.Fatalf("ParseCSV(%s): %v", p.ManualCSV, err)
	}
	return rows
}

// transcriptionA projects A onto the comparison tuple, keyed by the single
// menu number that is this family's whole address.
func transcriptionA(t *testing.T, p extable.Profile) map[int]chartRow {
	t.Helper()
	rows := parseTranscriptionA(t, p)
	out := make(map[int]chartRow, len(rows))
	for _, r := range rows {
		out[r.P1] = chartRow{Name: r.Name, Digits: r.Digits, Text: r.Text}
	}
	// ParseCSV refuses a duplicate (P1,P2,P3) and AddressSingle forces P2
	// and P3 to 0, so this P1-keyed map cannot silently absorb one; the
	// check is here because "cannot" is an argument and this is a fact.
	if len(out) != len(rows) {
		t.Fatalf("transcription A (%s) holds %d rows but only %d distinct menu numbers", p.ManualCSV, len(rows), len(out))
	}
	return out
}

// loadTranscriptionB reads B with encoding/csv. Its menu_number is the
// chart's three printed digits; its name is taken with no normalisation at
// all, and the collapsing this file does happens at COMPARISON time so that
// the evidence is never rewritten on the way in.
func loadTranscriptionB(t *testing.T, path string) map[int]chartRow {
	t.Helper()
	records := readEvidenceCSV(t, path, transcriptionBHeader, transcriptionBFields)

	out := make(map[int]chartRow, len(records))
	for i, rec := range records {
		where := fmt.Sprintf("%s data row %d", path, i+1)
		m := parseMenuNumber(t, where+" menu_number", rec[0])
		if prev, dup := out[m]; dup {
			t.Fatalf("%s: duplicate menu number %03d (already held %s)", where, m, prev)
		}
		out[m] = chartRow{
			Name:   rec[1],
			Digits: atoiOrFatal(t, where, "digits", rec[2]),
			Text:   parseBoolDigit(t, where, "text", rec[3]),
		}
	}
	return out
}

// loadPageLedger reads a ledger with encoding/csv, one row per PDF page, in
// ascending page order — which is also the order checkAgainstLedger walks the
// chart in, so a ledger whose rows were shuffled is re-ordered here rather
// than mis-tiled.
func loadPageLedger(t *testing.T, path string) []ledgerRow {
	t.Helper()
	records := readEvidenceCSV(t, path, pageLedgerHeader, pageLedgerFields)

	out := make([]ledgerRow, 0, len(records))
	seen := map[int]bool{}
	for i, rec := range records {
		where := fmt.Sprintf("%s data row %d", path, i+1)
		page := atoiOrFatal(t, where, "p1", rec[0])
		if seen[page] {
			t.Fatalf("%s: duplicate PDF page %d", where, page)
		}
		seen[page] = true
		// The ledger records the page twice — once as p1 and once in
		// pdf_page — so the two are bound to each other here. Nothing else
		// in the cross-check would notice them disagreeing. The cell is
		// matched on its LEADING DIGITS because the TS-590 ledgers write it
		// bare ("10") while the TS-480's annotates it with the printed folio
		// ("8 (folio 7)"), and the folio is the deriver's provenance rather
		// than a datum this file binds.
		if got := leadingDigits(t, where, "pdf_page", rec[6]); got != page {
			t.Fatalf("%s: the p1 column is %d but pdf_page %q names page %d", where, page, rec[6], got)
		}
		out = append(out, ledgerRow{
			PDFPage:     page,
			PDFPageCell: rec[6],
			First:       parseMenuNumber(t, where+" first_menu_number", rec[1]),
			FirstName:   rec[2],
			Last:        parseMenuNumber(t, where+" last_menu_number", rec[3]),
			LastName:    rec[4],
			RowCount:    atoiOrFatal(t, where, "row_count", rec[5]),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PDFPage < out[j].PDFPage })
	return out
}

// readEvidenceCSV reads path as CSV, requires its first record to be exactly
// wantHeader, and returns the data records.
//
// Comment = '#' covers the repository's CSV convention: neither quarantined
// artefact carries comment lines today (both put their prose in a .md
// companion), but the transcription-A CSVs are '#'-commented and a provenance
// block added to one of these later must not break this parser.
// FieldsPerRecord is set explicitly rather than inferred from the first
// record, so a file that is internally consistent but the wrong shape still
// fails.
func readEvidenceCSV(t *testing.T, path, wantHeader string, fields int) [][]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comment = '#'
	r.FieldsPerRecord = fields
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if len(records) == 0 {
		t.Fatalf("%s is empty", path)
	}
	if got := strings.Join(records[0], ","); got != wantHeader {
		t.Fatalf("%s header is\n  %s\nwant\n  %s", path, got, wantHeader)
	}
	if len(records) == 1 {
		t.Fatalf("%s carries a header and no data rows", path)
	}
	return records[1:]
}

// parseMenuNumber decodes the chart's printed three-digit Menu number, which
// is this family's WHOLE EX address.
//
// It is deliberately exact: three bytes, every one a decimal digit. A cell
// that is merely similar is a Fatal, not a silent partial parse, because a
// permissive reading is precisely how two genuinely different addresses would
// be folded into false agreement. strconv.Atoi is not given the whole cell
// for the same reason — it would accept a sign or spaces.
func parseMenuNumber(t *testing.T, where, raw string) int {
	t.Helper()
	const want = 3
	if len(raw) != want {
		t.Fatalf("%s: %q is not a three-digit menu number (%d bytes)", where, raw, len(raw))
	}
	for i := 0; i < want; i++ {
		if raw[i] < '0' || raw[i] > '9' {
			t.Fatalf("%s: %q is not a three-digit menu number (byte %d is %q)", where, raw, i, raw[i:i+1])
		}
	}
	return atoiOrFatal(t, where, "menu number", raw)
}

// parseBoolDigit decodes transcription B's text column, which is "0" or "1"
// and nothing else. B's own header names it that way, and anything else is a
// schema change this file must not read past.
func parseBoolDigit(t *testing.T, where, field, raw string) bool {
	t.Helper()
	switch raw {
	case "0":
		return false
	case "1":
		return true
	}
	t.Fatalf("%s: bad %s %q, want \"0\" or \"1\"", where, field, raw)
	return false
}

// leadingDigits reads the run of decimal digits at the start of raw. See
// loadPageLedger for why the pdf_page cell is read this way and not with
// strconv.Atoi.
func leadingDigits(t *testing.T, where, field, raw string) int {
	t.Helper()
	n := 0
	for n < len(raw) && raw[n] >= '0' && raw[n] <= '9' {
		n++
	}
	if n == 0 {
		t.Fatalf("%s: %s %q does not begin with a decimal number", where, field, raw)
	}
	return atoiOrFatal(t, where, field, raw[:n])
}

func atoiOrFatal(t *testing.T, where, field, raw string) int {
	t.Helper()
	n, err := strconv.Atoi(raw)
	if err != nil {
		t.Fatalf("%s: bad %s %q: %v", where, field, raw, err)
	}
	return n
}

// --- small helpers -------------------------------------------------------

// sortedAddrs returns m's menu numbers in ascending order, so that every
// failure list is stable and the ledger tiling walks the chart in the order
// the pages print it.
func sortedAddrs(m map[int]chartRow) []int {
	out := make([]int, 0, len(m))
	for a := range m {
		out = append(out, a)
	}
	sort.Ints(out)
	return out
}

// textRows returns the ascending menu numbers m marks as text rows.
func textRows(m map[int]chartRow) []int {
	var out []int
	for _, a := range sortedAddrs(m) {
		if m[a].Text {
			out = append(out, a)
		}
	}
	return out
}

func equalInts(got, want []int) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// withoutAddr and withAddr return an independent copy of m with one address
// removed or replaced, so a red proof never mutates the map another subtest
// is reading.
func withoutAddr(m map[int]chartRow, addr int) map[int]chartRow {
	out := make(map[int]chartRow, len(m))
	for k, v := range m {
		if k != addr {
			out[k] = v
		}
	}
	return out
}

func withAddr(m map[int]chartRow, addr int, row chartRow) map[int]chartRow {
	out := make(map[int]chartRow, len(m))
	for k, v := range m {
		out[k] = v
	}
	out[addr] = row
	return out
}
