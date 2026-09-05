// SPDX-License-Identifier: GPL-3.0-or-later

package ts480_test

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

	"github.com/gm5dna/open-rig-programmer/core/kw/ts480"
	"github.com/gm5dna/open-rig-programmer/internal/extable"
)

// This file is the TS-480 Stage 1 cross-check: it binds the three
// independently derived records of the PC control command reference's EX
// parameter chart to one another, and binds the generated inventory to the
// one of them that generates it.
//
// The three artefacts, and why there are three:
//
//   - TRANSCRIPTION A — menu480.csv, this package's ONLY generation source.
//     Layout-text-led, PDF-checked, in the ten-column extable shape, parsed
//     here through extable.ParseCSV under the REGISTERED "ts480" profile so
//     that the cross-check reads the same bytes through the same parser the
//     generator does.
//   - TRANSCRIPTION B — testdata/transcription-b-480.csv. Derived PDF-primary
//     at ≈600 dpi by a quarantined agent that never opened this repository,
//     never saw A or the ledger, and was told no row count.
//   - THE PAGE LEDGER — testdata/ledger-480.csv. Derived from the rendered
//     PDF's ruled cells by a second quarantined agent BEFORE either
//     transcription existed, and the source of the profile's ExpectedRows
//     (61).
//
// Agreement between three blind derivations is the evidence; this file is
// where that agreement is made mechanical rather than asserted in prose. ANY
// mismatch this file does not itself name and rule is a STOP for orchestrator
// arbitration AGAINST THE PDF, which may correct A, B or the ledger — never
// this test, and never an artefact edited merely to make the test pass.
//
// # What is compared, and what deliberately is not
//
// ADDRESS, NAME and DIGITS, and nothing else. The orchestrator's rulings at
// the legs' close, applied here:
//
//   - NAMES ARE COMPARED WHITESPACE-COLLAPSED, the same rule
//     core/kw/ts590/crosscheck_test.go applies and for the same reason: A
//     works from the layout text and B from a render, and the two instruments
//     disagree about a printed inter-word gap. On THIS chart the two legs
//     happen to agree byte for byte on all 61 names — the collapsing changes
//     nothing here today, which
//     TestCrossCheck_TheCollapsingIsLoadBearingOnNoRow pins rather than
//     merely says — and the rule is applied anyway so that the family's two
//     cross-checks state one comparison, not two.
//   - THE p4 COLUMN IS NOT COMPARED AT ALL: A's p4 is its own flattening
//     convention for the chart's parameter cells and B carries no such column.
//   - THE text FLAG IS NOT COMPARED AS A FLAG; the DIGITS are compared, and
//     the flag sets are asserted against the recorded conventions. See
//     wantATextRows/wantBTextRows.
//
// # Scope: what no leg here can catch
//
// All three legs read the same printed chart, so a defect PRINTED in the
// chart is transcribed faithfully by all three and is invisible to every
// comparison below. TestCrossCheck_TheMenu034Erratum pins the one such defect
// this chart is known to carry, so that the limit is recorded in a test
// rather than only in prose. No TS-480 has ever been asked anything by this
// project, so nothing here is hardware-verified and nothing here claims to
// be.

const (
	// The quarantined artefacts, relative to the package directory, which is
	// the working directory for `go test`.
	transcriptionBPath = "testdata/transcription-b-480.csv"
	pageLedgerPath     = "testdata/ledger-480.csv"

	// The two artefacts' exact header rows, pinned so that a schema change
	// fails LOUDLY here rather than being silently misparsed into false
	// agreement.
	transcriptionBHeader = "menu_number,name,digits,text"
	pageLedgerHeader     = "p1,first_menu_number,first_name,last_menu_number,last_name,row_count,pdf_page,visual_anchor"

	transcriptionBFields = 4
	pageLedgerFields     = 8
)

// THE MENU 034 ERRATUM, pinned as literals read off the chart.
//
// The EX command block's own prose says the widths out loud: "(Variable
// length) Normally 1-digit for the TS-480. Menu No. 32, 35 and 48 ~ 52 use
// 2-digit parameters" (480:409-411, layout line numbers re-derived from
// menu480.csv's provenance header). The CHART disagrees with the BLOCK: menu
// 034 "CW RX pitch/ TX sidetone frequency" runs 400/450/…/850 under codes 0-9
// and continues into the "Over" column, so it is a two-digit row that the
// block's list omits.
//
// ALL THREE LEGS READ 034 AS TWO DIGITS — transcription A (menu480.csv's
// quirk 1), transcription B (testdata/transcription-b-480.md §2) and the
// chart itself — so no comparison in this file can catch it: three faithful
// readings of one incomplete printed sentence agree perfectly. Pinning both
// sets here makes the discrepancy a recorded, deliberate state rather than an
// unnoticed one, and means that silently "correcting" 034 to one digit, or
// quietly adding 034 to the block's list, fails a test instead of passing
// unremarked.
//
// IT IS AN ERRATUM OF THE MANUAL, and this repository has no TS-480 to ask
// which of the two the radio answers. The errata schedule lives in
// core/kw/ts480/doc.go, which is task 8's file, not this one's; the
// orchestrator routes the entry there.
var (
	// twoDigitRowsThePrintedBlockLists is the block's own sentence, verbatim
	// as a set.
	twoDigitRowsThePrintedBlockLists = []int{32, 35, 48, 49, 50, 51, 52}
	// twoDigitRowsTheChartPrints is what the chart's grid actually carries,
	// and it is the block's list plus 034.
	twoDigitRowsTheChartPrints = []int{32, 34, 35, 48, 49, 50, 51, 52}
	// menu034 and its name, so a failure names the row rather than a number.
	menu034     = 34
	menu034Name = "CW RX pitch/ TX sidetone frequency"
)

// frozenEvidenceSHA256 is the freeze, transcribed from the commit message of
// a1e3779 ("core/kw: import the quarantined evidence legs — L×3 ledgers, B×3
// transcriptions, G geometry"), whose "SHA-256 of every imported file:" block
// records one hash per artefact.
//
// The .md companions are in here with the CSVs because they are not
// commentary about them: they are the derivation records this file cites for
// the text-flag convention it declines to compare and for the menu 034
// erratum it pins, and an artefact whose stated method had been quietly
// rewritten would be as corrupt as one whose rows had. The two TS-590 charts'
// artefacts are frozen by core/kw/ts590/crosscheck_test.go and leg G's
// fifteen by core/kw/golden_test.go; between the three files every artefact
// a1e3779 froze is pinned by whichever file reads it.
var frozenEvidenceSHA256 = map[string]string{
	"transcription-b-480.csv": "61eebbcf873eb89a25bba700d536289ecee863e213b126c4f2f9e93e81e8264e",
	"transcription-b-480.md":  "cf3cbb786cb4b031fa80dc641cdd0aa78d9a48b43d85363dd6778f8af45e9161",
	"ledger-480.csv":          "ab756b478c27f4a6590c5b94a17d212fdeb75f474303181cf2b03c45ac8ba893",
	"ledger-480.md":           "7c5fe9948def0f6f6e7a2827bf3788deb46b6ee1632211df66814d58a0d252bd",
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
				"with `git checkout a1e3779 -- core/kw/ts480/testdata/%s` and report\n"+
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
// address is the map key: the ts480 profile registers AddressSingle, so the
// chart's three-digit Menu No. IS the whole address and P2/P3 are zero on
// every row (checked separately, against the profile's own policy).
type chartRow struct {
	Name   string
	Digits int
	Text   bool
}

func (r chartRow) String() string {
	return fmt.Sprintf("name=%q digits=%d text=%v", r.Name, r.Digits, r.Text)
}

// ledgerRow is one ledger row's bindable content. THE LEDGER IS PER PDF PAGE,
// not per address group: this chart prints no group labels and no address
// hierarchy at all, so the only boundary a reader of the rendered page can
// see is where the page ends. The p1 column is the PDF page number, pdf_page
// repeats it (annotated with the printed folio, "8 (folio 7)"), and
// visual_anchor is the deriver's prose description of the page's first and
// last ruled bands — read (the column count is pinned, and pdf_page is bound
// to p1 below) but not otherwise bound.
type ledgerRow struct {
	PDFPage     int
	PDFPageCell string
	First       int
	FirstName   string
	Last        int
	LastName    string
	RowCount    int
}

// chart is the chart's three legs plus the labels a complaint needs.
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
// and only to a name; see the file comment.
var whitespaceRun = regexp.MustCompile(`\s+`)

func collapseSpace(s string) string {
	return strings.TrimSpace(whitespaceRun.ReplaceAllString(s, " "))
}

// crossCheck compares the chart's three legs and RETURNS one complaint per
// disagreement rather than reporting through *testing.T.
//
// The return value is what makes the red proofs below possible: a
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
	// first question arbitration asks. The ExpectedRows value itself is
	// pinned on the profile by internal/extable's registration test, so this
	// leg binds the artefacts to that pinned constant rather than keeping a
	// second copy of it.
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
// leg. Zero complaints is the pass condition.
func TestCrossCheck_A_B_Ledger(t *testing.T) {
	c := loadChart(t)
	for _, complaint := range crossCheck(c) {
		t.Errorf("CROSS-CHECK DISAGREEMENT — THIS IS A STOP FOR ARBITRATION AGAINST THE PDF, not a test to adjust:\n%s", complaint)
	}
}

// TestCrossCheck_TheChartShapePolicies asserts of transcription A the facts
// the registered profile declares about this chart's shape, so that the
// inventory-vs-A leg's claims about the GENERATED items rest on a source that
// was checked for the same things.
//
// ParseCSV already refuses a non-blank label, a non-zero P2/P3 and a text row
// under LabelsAbsent, AddressSingle and TextRowsAbsent, so this leg is not
// the first line of defence; it is here because a claim about a generated
// file is only worth making if the source it was generated from carries it
// too.
func TestCrossCheck_TheChartShapePolicies(t *testing.T) {
	p := lookupProfile(t)
	for _, r := range parseTranscriptionA(t, p) {
		if r.P1Label != "" || r.P2Label != "" {
			t.Errorf("menu %03d (%q): transcription A (%s) carries labels p1_label=%q p2_label=%q, but this chart prints no label columns (LabelsAbsent)", r.P1, r.Name, p.ManualCSV, r.P1Label, r.P2Label)
		}
		if r.P2 != 0 || r.P3 != 0 {
			t.Errorf("menu %03d (%q): transcription A (%s) carries p2=%d p3=%d, but this radio's EX address is a SINGLE component and both must be 0 (AddressSingle)", r.P1, r.Name, p.ManualCSV, r.P2, r.P3)
		}
		if r.Text {
			t.Errorf("menu %03d (%q): transcription A (%s) marks a text row, but this chart prints no free-text item (TextRowsAbsent)", r.P1, r.Name, p.ManualCSV)
		}
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
//   - A marks NO row. The ts480 profile registers TextRowsAbsent — this book
//     prints no "ASCII characters" row, no message and no version string —
//     and ParseCSV would refuse a flagged row outright.
//   - B applied a STRUCTURAL test, "prose printed across the grid instead of
//     cells", and so flagged menus 048-052: the five PF-key rows share ONE
//     cell merged across all eleven grid columns and down all five rows,
//     printing "00 ~ 99 (2-digit)" and a cross-reference to page 64 of the
//     instruction manual. B's own record
//     (testdata/transcription-b-480.md §1) flags the divergence rather than
//     resolving it and says in as many words that a consumer reading text as
//     "the parameter is a character string" would want text=0 here.
//
// The merged cell prints a NUMERIC legend, and the orchestrator ruled on
// exactly that: a merged cell printing "(n-digit)" is a numeric legend, so
// text=0 is the repository's reading. All five rows carry DIGITS 2 on both
// legs — which is the datum, and which crossCheck already binds.
var (
	wantATextRows     = []int(nil)
	wantBTextRows     = []int{48, 49, 50, 51, 52}
	textFlagDiffersAt = []int{48, 49, 50, 51, 52}
)

// TestCrossCheck_TextFlagsAreTheRecordedConventions pins both legs' text-row
// sets and requires every disagreement between them to be one the rulings
// name AND to carry equal DIGITS on both sides.
func TestCrossCheck_TextFlagsAreTheRecordedConventions(t *testing.T) {
	c := loadChart(t)
	if got := textRows(c.a); !equalInts(got, wantATextRows) {
		t.Errorf("transcription A (%s) marks text rows %v, the recorded convention is %v", c.aPath, got, wantATextRows)
	}
	if got := textRows(c.b); !equalInts(got, wantBTextRows) {
		t.Errorf("transcription B (%s) marks text rows %v, the recorded convention is %v", c.bPath, got, wantBTextRows)
	}

	var differ []int
	for _, m := range sortedAddrs(c.a) {
		bv, in := c.b[m]
		if !in {
			continue
		}
		if c.a[m].Text != bv.Text {
			differ = append(differ, m)
			if c.a[m].Digits != bv.Digits {
				t.Errorf("menu %03d (%q): the two legs disagree about the text FLAG *and* about the DIGITS (A %d, B %d) — only the flag is a recorded convention; a digits difference is a STOP",
					m, c.a[m].Name, c.a[m].Digits, bv.Digits)
			}
		}
	}
	if !equalInts(differ, textFlagDiffersAt) {
		t.Errorf("the text flag differs between the legs at %v, the recorded rulings name %v — a new divergence is a STOP for arbitration, not a pin to widen", differ, textFlagDiffersAt)
	}
}

// TestCrossCheck_TheCollapsingIsLoadBearingOnNoRow measures the
// normalisation's extent on THIS chart, which crossCheck by construction
// cannot: it compares collapsed names, so it can never report how much the
// collapsing hid.
//
// On the TS-480 the answer is nothing at all — the two legs agree byte for
// byte on all 61 names — and pinning the empty set is what makes that a fact
// rather than a remark. The TS-590 pair's counterpart names five rows on the
// S and six on the SG (core/kw/ts590/crosscheck_test.go), so an empty set
// here is a property of this chart, not of the rule.
func TestCrossCheck_TheCollapsingIsLoadBearingOnNoRow(t *testing.T) {
	c := loadChart(t)
	var differ []int
	for _, m := range sortedAddrs(c.a) {
		bv, in := c.b[m]
		if !in {
			continue
		}
		av := c.a[m]
		if av.Name == bv.Name {
			continue
		}
		differ = append(differ, m)
		if collapseSpace(av.Name) != collapseSpace(bv.Name) {
			t.Errorf("menu %03d: the two legs differ in a name byte the collapsing does NOT normalise — this is a reading difference and a STOP:\n  A (%s): %q\n  B (%s): %q",
				m, c.aPath, av.Name, c.bPath, bv.Name)
		} else {
			t.Errorf("menu %03d: the two legs differ only in whitespace:\n  A (%s): %q\n  B (%s): %q\nThe recorded state of this chart is that the collapsing is load-bearing on NO row; a first one is a deliberate change to record, not a pin to widen.",
				m, c.aPath, av.Name, c.bPath, bv.Name)
		}
	}
	if len(differ) != 0 {
		t.Errorf("the legs differ verbatim at %v; the recorded state is that they agree byte for byte on all %d names", differ, len(c.a))
	}
}

// TestCrossCheck_TheMenu034Erratum pins the printed defect no leg above can
// catch: see the erratum's own comment for why three faithful readings of one
// incomplete sentence agree.
func TestCrossCheck_TheMenu034Erratum(t *testing.T) {
	c := loadChart(t)

	// Both legs read 034 as a two-digit row, and both name it the same
	// thing. If either ever reads one digit, the erratum has become an
	// ordinary disagreement and goes to arbitration.
	for _, src := range []struct {
		label string
		path  string
		rows  map[int]chartRow
	}{
		{"transcription A", c.aPath, c.a},
		{"transcription B", c.bPath, c.b},
	} {
		row, in := src.rows[menu034]
		if !in {
			t.Errorf("%s (%s) has no menu %03d, which the erratum pin requires", src.label, src.path, menu034)
			continue
		}
		if collapseSpace(row.Name) != menu034Name {
			t.Errorf("%s (%s): menu %03d is named %q, the erratum pin says %q", src.label, src.path, menu034, row.Name, menu034Name)
		}
		if row.Digits != 2 {
			t.Errorf("%s (%s): menu %03d (%q) carries %d digits, the erratum pin says 2 — the chart's grid reaches the Over column here even though the EX block's prose omits the row from its 2-digit list",
				src.label, src.path, menu034, row.Name, row.Digits)
		}
	}

	// The set the chart prints, derived, against the set the block's
	// sentence lists, hardcoded. The difference is exactly {034}, and the
	// test states it that way rather than merely counting, so a SECOND
	// omission appearing later reads as itself.
	if got := digitRows(c.a, 2); !equalInts(got, twoDigitRowsTheChartPrints) {
		t.Errorf("transcription A (%s) carries two-digit rows %v, the chart pin says %v", c.aPath, got, twoDigitRowsTheChartPrints)
	}
	if got := digitRows(c.b, 2); !equalInts(got, twoDigitRowsTheChartPrints) {
		t.Errorf("transcription B (%s) carries two-digit rows %v, the chart pin says %v", c.bPath, got, twoDigitRowsTheChartPrints)
	}
	if got := difference(twoDigitRowsTheChartPrints, twoDigitRowsThePrintedBlockLists); !equalInts(got, []int{menu034}) {
		t.Errorf("the chart's two-digit rows exceed the EX block's own list by %v; the recorded erratum is exactly menu %03d, and a second omission is a STOP for arbitration rather than a pin to widen", got, menu034)
	}
}

// TestCrossCheck_InventoryAgainstTranscriptionA binds the GENERATED inventory
// to transcription A, the file it is generated from.
//
// staleness480_test.go already re-renders the generated file from the CSV and
// byte-compares it, which catches drift between the two. This test binds
// something else: what the inventory MEANS once loaded — that the rows are
// the chart's rows, in address order, carrying this family's shape (P2 and P3
// zero, no labels, no text, the observation sentinels) and this chart's
// widths. It reaches the inventory through ts480.EXItems, the route every
// other consumer uses.
func TestCrossCheck_InventoryAgainstTranscriptionA(t *testing.T) {
	p := lookupProfile(t)
	a := transcriptionA(t, p)
	order := sortedAddrs(a)
	items := ts480.EXItems()

	if len(items) != len(a) {
		t.Fatalf("the generated inventory holds %d items, transcription A (%s) holds %d rows", len(items), p.ManualCSV, len(a))
	}
	// The items must be in ascending P1 order in their own right, not merely
	// be a permutation that happens to match A once sorted: the generated
	// file's doc comment claims the sort, so the claim is pinned here rather
	// than assumed.
	for i := 1; i < len(items); i++ {
		if prev, cur := items[i-1].Addr.P1, items[i].Addr.P1; prev >= cur {
			t.Errorf("the generated inventory is not in ascending P1 order at index %d: %03d follows %03d", i, cur, prev)
		}
	}
	for i, want := range order {
		got := items[i]
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
		if got.Text {
			t.Errorf("menu %03d (%q): the inventory marks a text item, but this chart prints no free-text item (TextRowsAbsent)", want, row.Name)
		}
		if got.Addr.P2 != 0 || got.Addr.P3 != 0 {
			t.Errorf("menu %03d (%q): the inventory carries P2=%d P3=%d, but this radio's EX address is a SINGLE component and both must be 0 (AddressSingle)", want, row.Name, got.Addr.P2, got.Addr.P3)
		}
		if got.P1Label != "" || got.P2Label != "" {
			t.Errorf("menu %03d (%q): the inventory carries labels P1Label=%q P2Label=%q, but this chart prints no label columns (LabelsAbsent)", want, row.Name, got.P1Label, got.P2Label)
		}
		// ObservationsAbsent: no TS-480 has ever been asked anything by this
		// project, so both observation fields must carry their absence
		// sentinels. A non-zero one here would be a hardware claim nothing
		// supports.
		if got.ObservedReadWidth != 0 || got.ObservedReadShape != "" {
			t.Errorf("menu %03d (%q): the inventory carries ObservedReadWidth=%d ObservedReadShape=%q, but this profile registers ObservationsAbsent and no TS-480 has ever been asked anything",
				want, row.Name, got.ObservedReadWidth, got.ObservedReadShape)
		}
	}
}

// The falsifications the milestone requires, one per leg, fired against an
// IN-MEMORY mutation of the loaded evidence. No artefact is ever written.
//
// Without these the cross-check's green is unfalsifiable: a comparison that
// silently compared nothing would pass every run above just as happily.

// TestRedProof_DroppingARowFromTranscriptionA is caught.
func TestRedProof_DroppingARowFromTranscriptionA(t *testing.T) {
	c := loadChart(t)
	// The LAST row of the chart, so the falsification also moves the final
	// page's boundary and the four-way total at once.
	victim := sortedAddrs(c.a)[len(c.a)-1]
	c.a = withoutAddr(c.a, victim)
	requireComplaint(t, crossCheck(c), fmt.Sprintf("menu %03d", victim))
}

// TestRedProof_DroppingARowFromTranscriptionB is caught.
func TestRedProof_DroppingARowFromTranscriptionB(t *testing.T) {
	c := loadChart(t)
	// The FIRST row this time, so the two proofs between them exercise both
	// ends of the tiling.
	victim := sortedAddrs(c.b)[0]
	c.b = withoutAddr(c.b, victim)
	requireComplaint(t, crossCheck(c), fmt.Sprintf("menu %03d", victim))
}

// TestRedProof_PerturbingAWidth is caught — the leg that no row-count check
// could ever see, because the counts still agree.
func TestRedProof_PerturbingAWidth(t *testing.T) {
	c := loadChart(t)
	// Menu 034 itself, so the proof also demonstrates that the erratum pin
	// above is guarding a value the cross-check would otherwise let drift.
	row := c.a[menu034]
	row.Digits++
	c.a = withAddr(c.a, menu034, row)
	requireComplaint(t, crossCheck(c), "the DIGITS differ")
}

// TestRedProof_PerturbingTheLedgerSum is caught. row_count is what
// ExpectedRows was derived from, so a ledger that quietly disagreed with
// itself would take the profile's bound with it.
//
// BOTH consequences are required, not either: the four-way total moves, and
// the page the count belongs to stops tiling. Requiring only the total would
// leave checkAgainstLedger unfalsified.
func TestRedProof_PerturbingTheLedgerSum(t *testing.T) {
	c := loadChart(t)
	ledger := append([]ledgerRow(nil), c.ledger...)
	ledger[0].RowCount++
	c.ledger = ledger
	complaints := crossCheck(c)
	requireComplaint(t, complaints, "row totals disagree")
	requireComplaint(t, complaints, "closes the page's run at menu")
}

// TestRedProof_PerturbingALedgerBoundaryName is caught. It is the leg no
// count can see: the page boundaries and their names still tile, and only the
// NAME the ledger recorded for a boundary row has moved.
func TestRedProof_PerturbingALedgerBoundaryName(t *testing.T) {
	c := loadChart(t)
	ledger := append([]ledgerRow(nil), c.ledger...)
	ledger[0].FirstName += " (perturbed)"
	c.ledger = ledger
	requireComplaint(t, crossCheck(c), "the ledger ("+c.ledgerPath+") records first_name")
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

func loadChart(t *testing.T) chart {
	t.Helper()
	p := lookupProfile(t)
	return chart{
		label:        "TS-480",
		aPath:        p.ManualCSV,
		bPath:        transcriptionBPath,
		ledgerPath:   pageLedgerPath,
		expectedRows: p.ExpectedRows,
		a:            transcriptionA(t, p),
		b:            loadTranscriptionB(t, transcriptionBPath),
		ledger:       loadPageLedger(t, pageLedgerPath),
	}
}

// lookupProfile takes the REGISTERED profile, not a literal of this file's
// own: the whole point is to read A exactly as the generator reads it, under
// the same digit bounds and the same address policy.
func lookupProfile(t *testing.T) extable.Profile {
	t.Helper()
	p, ok := extable.Lookup("ts480")
	if !ok {
		t.Fatal(`extable.Lookup("ts480"): the profile is not registered`)
	}
	return p
}

// parseTranscriptionA reads the chart's CSV through extable.ParseCSV — the
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

// loadPageLedger reads the ledger with encoding/csv, one row per PDF page, in
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
		// matched on its LEADING DIGITS because this ledger annotates it
		// with the printed folio ("8 (folio 7)"), and the folio is the
		// deriver's provenance rather than a datum this file binds.
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
// companion), but menu480.csv is '#'-commented and a provenance block added
// to one of these later must not break this parser. FieldsPerRecord is set
// explicitly rather than inferred from the first record, so a file that is
// internally consistent but the wrong shape still fails.
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

// parseMenuNumber decodes the chart's printed three-digit Menu No., which is
// this family's WHOLE EX address.
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

// digitRows returns the ascending menu numbers whose Digits is n.
func digitRows(m map[int]chartRow, n int) []int {
	var out []int
	for _, a := range sortedAddrs(m) {
		if m[a].Digits == n {
			out = append(out, a)
		}
	}
	return out
}

// difference returns the ascending members of got that are not in want.
func difference(got, want []int) []int {
	in := make(map[int]bool, len(want))
	for _, w := range want {
		in[w] = true
	}
	var out []int
	for _, g := range got {
		if !in[g] {
			out = append(out, g)
		}
	}
	sort.Ints(out)
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
// removed or replaced, so a red proof never mutates the map another test is
// reading.
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
