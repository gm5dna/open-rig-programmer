// SPDX-License-Identifier: GPL-3.0-or-later

package ma_test

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/kw/ma"
	"github.com/gm5dna/open-rig-programmer/internal/extable"
)

// This file is pair 2's Stage 1 cross-check. It binds the independently
// derived records of BOTH books' "EX Command Parameter Lists" to one another
// and to the inventories generated from them, and it replays evidence leg G's
// hand-derived wire frames through the two MA0 codecs.
//
// # The legs, and why there are four
//
//   - TRANSCRIPTION A — menu890s.csv and menu990s.csv, this package's ONLY
//     generation sources. Layout-text-led, PDF-checked, in the ten-column
//     extable shape, parsed here through extable.ParseCSV under the REGISTERED
//     "ts890s" and "ts990s" profiles, so the cross-check reads the same bytes
//     through the same parser the generator does.
//   - TRANSCRIPTION B — testdata/transcription-b-{890s,990s}.csv. Derived
//     PDF-primary by a quarantined agent that never opened this repository,
//     never saw A or the ledgers, and was told no row count.
//   - THE PAGE LEDGERS — testdata/ledger-{890s,990s}.csv. Derived from the
//     rendered PDFs' ruled cells by a second quarantined agent, blind to A and
//     to B, and the source of each profile's ExpectedRows (162 and 194).
//   - EVIDENCE LEG G — the eleven testdata/*-{890,990}.golden geometry vectors
//     and their provenance.md, derived by a third quarantined agent from the
//     printed position rulers alone: no code, no generator, no other document.
//
// All four were placed in this package by the ORCHESTRATOR, at commit c8f982d,
// whose message records one SHA-256 per file. This file NEVER edits one and no
// failure here is ever fixed by editing one: TestQuarantinedEvidenceFrozen
// re-enforces those hashes, so the freeze survives a rewritten history rather
// than depending on someone running a diff gate.
//
// # What is compared, and what deliberately is not
//
// SIX COLUMNS: p1, p2, p3, name (the chart's Function cell verbatim), digits
// and text — ruling R-A, on pair 1's precedent. THE p4 COLUMN IS NOT COMPARED
// AT ALL: A's p4 is its own flattening convention for the chart's parameter
// cells and B carries no such column.
//
// A DISAGREEMENT ON ANY OF THE SIX IS A COMPLAINT, and every complaint must be
// one the ruling table below NAMES, CITES and CHECKS THE SHAPE OF. An unruled
// complaint is a STOP for orchestrator arbitration AGAINST THE PDF, which may
// correct A, B or a ledger — never this test, and never an artefact edited
// merely to make the test pass. A ruling that stops firing is equally a
// failure: it means the leg it describes has moved.
//
// # Scope: what no leg here can catch
//
// All four legs read the same printed charts, so a defect PRINTED in a chart
// is transcribed faithfully by all of them and is invisible to every
// comparison below. Neither radio has ever been asked anything by this
// project (A19), so nothing here is hardware-verified and nothing here claims
// to be.

// The quarantined artefacts, relative to the package directory, which is the
// working directory for `go test`.
const (
	goldenDir = "testdata"

	transcriptionB890Path = "testdata/transcription-b-890s.csv"
	transcriptionB990Path = "testdata/transcription-b-990s.csv"
	pageLedger890Path     = "testdata/ledger-890s.csv"
	pageLedger990Path     = "testdata/ledger-990s.csv"

	// The two artefact schemas' exact header rows, pinned so that a schema
	// change fails LOUDLY here rather than being silently misparsed into
	// false agreement. Both radios' files share one header apiece.
	transcriptionBHeader = "p1,p2,p3,name,digits,text"
	pageLedgerHeader     = "pdf_page,first_address,first_name,last_address,last_name,row_count,visual_anchor"

	transcriptionBFields = 6
	pageLedgerFields     = 7
)

// frozenEvidenceSHA256 is the freeze. THE TWENTY QUARANTINED ARTEFACTS are
// transcribed from the commit message of c8f982d ("ma: commit the quarantined
// lane-E evidence for T10 (legs L, B, G)"), which records one hash per file.
// THE FIVE ma0-*.golden ROUND-TRIP VECTORS are task 7's, introduced at 8497698
// ("ma: the two hand-written MA0 record codecs") and hash-frozen HERE for the
// first time: shared_test.go compares against them but nothing pinned their
// bytes, so a regenerated vector would have moved the target and the
// comparison as one.
//
// THE .md COMPANIONS ARE IN HERE WITH THE CSVs, and provenance.md with the
// vectors, because they are not commentary about them: they are the derivation
// records and the assumption register this file cites for the conventions it
// declines to compare and for the rulings it applies, and an artefact whose
// stated method had been quietly rewritten would be as corrupt as one whose
// rows had.
//
// NOTHING IN THIS PACKAGE'S testdata IS EXCLUDED. Transcription A is not here
// because it is not in testdata at all: menu890s.csv and menu990s.csv are
// package-level generation sources, re-rendered and byte-compared by
// staleness890s_test.go and staleness990s_test.go.
var frozenEvidenceSHA256 = map[string]string{
	// Leg G — eleven geometry vectors and their provenance (c8f982d).
	"AI-890.golden":  "4af08c66e645b37eb32057f397492dd58ccc24fe110f93b286fffa6242aaeee1",
	"AI-990.golden":  "fce5ebd221b40dfc4f6034a5132b820ddcda42a0c6a4da6195df6037b5bc2536",
	"EX-890.golden":  "bc91c6eb4a1b03fe3ce3021015a08edfea1ce3106dd28b8a2c00215d5ec82936",
	"EX-990.golden":  "87ec054602aee2fb74f38d209a750975ee7743db04d9f6d365e67ace3204fcbe",
	"FV-890.golden":  "2c40e204c50d91450ee416f053d4dddf707a1f1eb053fe697d0c065591fa68cc",
	"FV-990.golden":  "6b960fb849a68453b25818e44df8febf859ef0a3eac09c86b10652cab4c64b25",
	"ID-890.golden":  "63bbb19fae1dc2757efdb0460d400c516589428c878d759a67d5e9e5045c42f0",
	"ID-990.golden":  "0b26fd2b7bfd5a85a99794cb47e448408001ec669bf7f267d795b0a8408f9985",
	"MA0-890.golden": "924000f16979cb4fc0a8292c64c8f25682b2361248c53c61405a20394f024bca",
	"MA0-990.golden": "cc90a5de3fcffded5c0cfa33d320d872d431bf1dad192ded17db7b816233662d",
	"MN-990.golden":  "6fc9c0516493f927253ab185ed8744f57d9061b364ef2c8363113b354416bdf5",
	"provenance.md":  "60b0d73a02822b217349dcdab98c14515b0250577df2514b157526b174af3daf",

	// Leg L — the two page ledgers and their derivation records (c8f982d).
	"ledger-890s.csv": "c2f4ff66de91605f14f58735607dfb6c02e59fc6c41fe113567d0f06cd0eae18",
	"ledger-890s.md":  "c6e97030a4dbda07e940fccff924746842d65a58f84858be94f03ce80b2f578b",
	"ledger-990s.csv": "ecf43decbd1d0ba77da8364dcbbaa54b991e92750a7b053ee6c48be41c698a07",
	"ledger-990s.md":  "690de3853f95ed196553716ca1834e2e6c6caa50b041503dee85f0a8cb2f98f5",

	// Leg B — the two blind transcriptions and their records (c8f982d).
	"transcription-b-890s.csv": "f45226d2bce269c76b966de596909b3783757477a5b09a66c3a955d9e6d12560",
	"transcription-b-890s.md":  "9cd6b2b9c7cc5e4e4fdc59166e6f36f8fbb093386a75dd2b2c1db4f173dcf528",
	"transcription-b-990s.csv": "b234f34ed45939bcde0ac7c90aad18c2ed857eb1b0b7f32d322aaff73ad3775a",
	"transcription-b-990s.md":  "56b4fd0e7314f11627aa652bb40f7ec253dbe32088f8cb08f64484180026e919",

	// Task 7's MA0 name round-trip vectors (8497698), frozen here.
	"ma0-890s-name0.golden":  "5c9427645c43b6a565f135ace2ef335d362ec8a6d0cf90596d369b93399d8dc4",
	"ma0-890s-name3.golden":  "8142da0e0d53c91f4fc16d1897f349ff060f26de19ed64f850e7b736799ef72e",
	"ma0-890s-name10.golden": "390dec54f2962c28ab77d89625aca0877f16c5a1bbd817154d51265d3ed49914",
	"ma0-990s-name3.golden":  "e71b5efa756a0cf8094ed8aa74de9c194fd7e911631a93d0297c078fb2933746",
	"ma0-990s-name10.golden": "68035537a66545a783ffa9b904c4a18ebbf6ed8c01b79360f1e25bfe32ad743e",
}

// TestQuarantinedEvidenceFrozen recomputes each artefact's SHA-256 and
// compares it with the value its introducing commit recorded, so that the
// freeze is self-enforcing in CI rather than a fact recoverable only by git
// archaeology. Every leg below reads bytes this test has vouched for.
//
// The second half is the one that catches the interesting case: a walk of
// testdata requires EVERY file present to be covered by the map above.
// Without it a new unfrozen vector could be added beside the twenty-five and
// pass a test that only ever looked up names it already knew — which is
// exactly the gap task 7's five vectors sat in until now.
func TestQuarantinedEvidenceFrozen(t *testing.T) {
	for name, want := range frozenEvidenceSHA256 {
		path := filepath.Join(goldenDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("reading frozen artefact %s: %v", path, err)
			continue
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != want {
			t.Errorf("FREEZE BROKEN — %s has changed since the commit that recorded its hash.\n"+
				"  recorded SHA-256 %s\n"+
				"  present  SHA-256 %s\n"+
				"This artefact is quarantined evidence: it is never regenerated and\n"+
				"never edited to satisfy a test. Restore it from the repository root\n"+
				"with `git checkout c8f982d -- core/kw/ma/testdata/%s` (or 8497698 for\n"+
				"an ma0-*.golden) and report the change.",
				path, want, got, name)
		}
	}

	present, err := filepath.Glob(filepath.Join(goldenDir, "*"))
	if err != nil {
		t.Fatalf("globbing %s: %v", goldenDir, err)
	}
	if len(present) == 0 {
		t.Fatalf("no artefacts found in %s — the evidence legs are missing", goldenDir)
	}
	for _, path := range present {
		if _, ok := frozenEvidenceSHA256[filepath.Base(path)]; !ok {
			t.Errorf("%s has no recorded SHA-256: every artefact in %s must be frozen by a commit that records its hash", path, goldenDir)
		}
	}
}

// --- the menu-chart cross-check -------------------------------------------

// chartAddr is the GROUPED menu address both books print: a one-digit menu
// type, a two-digit category and a two-digit item. All three are on the wire,
// which is why all three are the key here and why neither chart can be keyed
// on a single number the way pair 1's are.
type chartAddr [3]int

func (a chartAddr) String() string { return fmt.Sprintf("%d/%02d/%02d", a[0], a[1], a[2]) }

// chartRow is the tuple both transcriptions carry for one address.
//
// Parameterless is A's only: transcription B was told nothing about the
// hyphen convention and reads the four "Does not correspond to a command"
// rows as ordinary 3-digit rows, which is ruling R-PARAMETERLESS below.
type chartRow struct {
	Name          string
	Digits        int
	Text          bool
	Parameterless bool
}

// digitsCell renders the row's width the way its chart prints it, so a
// complaint about a parameterless row shows the hyphen rather than a zero
// that could be mistaken for a width.
func (r chartRow) digitsCell() string {
	if r.Parameterless {
		return "-"
	}
	return strconv.Itoa(r.Digits)
}

func (r chartRow) String() string {
	return fmt.Sprintf("name=%q digits=%s text=%v", r.Name, r.digitsCell(), r.Text)
}

// ledgerRow is one ledger row's bindable content. THE LEDGER IS PER PDF PAGE:
// neither chart prints a group label or an address hierarchy a reader of the
// rendered page could see, so the only boundary that leg could derive is where
// the page ends. visual_anchor is the deriver's prose description of the
// page's first and last ruled bands — read (the column count is pinned) but
// not otherwise bound.
type ledgerRow struct {
	PDFPage   int
	First     chartAddr
	FirstName string
	Last      chartAddr
	LastName  string
	RowCount  int
}

// chart is one radio's four legs plus the labels a complaint needs.
type chart struct {
	label        string
	aPath        string
	bPath        string
	ledgerPath   string
	expectedRows int
	a            map[chartAddr]chartRow
	b            map[chartAddr]chartRow
	ledger       []ledgerRow
	rulings      []ruling
}

// complaint is one disagreement, addressable so that the ruling table can be
// matched against it mechanically rather than by substring.
//
// It is a VALUE RATHER THAN A t.Errorf because a falsification has to observe
// that the comparison bites, and a comparison that reported straight into t
// could only be falsified by making the test fail.
type complaint struct {
	Addr   chartAddr
	Field  string // "membership", "name", "digits", "text", or "" for a whole-chart complaint
	Detail string
}

// The six compared columns, named once. p1/p2/p3 are the key, so a
// disagreement about them surfaces as a membership complaint.
const (
	fieldMembership = "membership"
	fieldName       = "name"
	fieldDigits     = "digits"
	fieldText       = "text"
)

// whitespaceRun collapses a run of whitespace to one space. It is the ONE
// normalisation applied to a name, and only at COMPARISON time, so the
// evidence is never rewritten on the way in.
var whitespaceRun = regexp.MustCompile(`\s+`)

func collapseSpace(s string) string {
	return strings.TrimSpace(whitespaceRun.ReplaceAllString(s, " "))
}

func stripSpaces(s string) string { return strings.ReplaceAll(collapseSpace(s), " ", "") }

// crossCheck compares one radio's four legs and RETURNS one complaint per
// disagreement. Everything here is deterministic and ordered, so a complaint
// list is stable between runs.
func crossCheck(c chart) []complaint {
	var out []complaint
	add := func(addr chartAddr, field, format string, args ...any) {
		out = append(out, complaint{Addr: addr, Field: field, Detail: fmt.Sprintf(format, args...)})
	}

	// Both directions, separately, so a failure says WHICH leg is missing the
	// address rather than merely that the sets differ.
	for _, m := range sortedAddrs(c.a) {
		if _, in := c.b[m]; !in {
			add(m, fieldMembership, "%s: menu %s is in transcription A (%s) but NOT in transcription B (%s): A has %s", c.label, m, c.aPath, c.bPath, c.a[m])
		}
	}
	for _, m := range sortedAddrs(c.b) {
		if _, in := c.a[m]; !in {
			add(m, fieldMembership, "%s: menu %s is in transcription B (%s) but NOT in transcription A (%s): B has %s", c.label, m, c.bPath, c.aPath, c.b[m])
		}
	}

	for _, m := range sortedAddrs(c.a) {
		bv, in := c.b[m]
		if !in {
			continue // already reported above
		}
		av := c.a[m]
		if an, bn := collapseSpace(av.Name), collapseSpace(bv.Name); an != bn {
			add(m, fieldName, "%s: menu %s: the NAME differs (compared whitespace-collapsed):\n  A (%s): %q\n  B (%s): %q",
				c.label, m, c.aPath, av.Name, c.bPath, bv.Name)
		}
		if av.digitsCell() != bv.digitsCell() {
			add(m, fieldDigits, "%s: menu %s (%q): the DIGITS differ:\n  A (%s): %s\n  B (%s): %s",
				c.label, m, av.Name, c.aPath, av.digitsCell(), c.bPath, bv.digitsCell())
		}
		if av.Text != bv.Text {
			add(m, fieldText, "%s: menu %s (%q): the TEXT flag differs:\n  A (%s): %v\n  B (%s): %v",
				c.label, m, av.Name, c.aPath, av.Text, c.bPath, bv.Text)
		}
	}

	out = append(out, checkAgainstLedger(c, "transcription A", c.aPath, c.a)...)
	out = append(out, checkAgainstLedger(c, "transcription B", c.bPath, c.b)...)

	// One four-way equality, reported whole: which of the four moved is the
	// first question arbitration asks.
	sum := 0
	for _, l := range c.ledger {
		sum += l.RowCount
	}
	if len(c.a) != len(c.b) || len(c.a) != sum || len(c.a) != c.expectedRows {
		add(chartAddr{}, "", "%s: row totals disagree: transcription A = %d, transcription B = %d, ledger row_count sum = %d, profile ExpectedRows = %d",
			c.label, len(c.a), len(c.b), sum, c.expectedRows)
	}
	return out
}

// checkAgainstLedger binds one transcription to its page ledger.
//
// THE LEDGER TILES THE ADDRESS SEQUENCE. The boundary is the printed page and
// neither transcription carries a page column, so the binding is not a join on
// a key: the ledger's rows, in ascending PDF-page order, must consume the
// transcription's ascending addresses in contiguous runs of row_count, each
// run opening at first_address and closing at last_address with the recorded
// names, and the last run must exhaust the transcription. That pins the ORDER
// and the BOUNDARIES as well as the sizes, which is what the ledger's own
// visual_anchor prose describes.
func checkAgainstLedger(c chart, srcLabel, srcPath string, rows map[chartAddr]chartRow) []complaint {
	var out []complaint
	add := func(format string, args ...any) {
		out = append(out, complaint{Detail: fmt.Sprintf(format, args...)})
	}

	addrs := sortedAddrs(rows)
	i := 0
	for _, l := range c.ledger {
		if i+l.RowCount > len(addrs) {
			add("%s: PDF page %d: the ledger (%s) records row_count %d starting at %s's address index %d, but only %d addresses remain",
				c.label, l.PDFPage, c.ledgerPath, l.RowCount, srcLabel, i, len(addrs)-i)
			return out
		}
		seg := addrs[i : i+l.RowCount]
		i += l.RowCount

		first, last := seg[0], seg[len(seg)-1]
		if first != l.First {
			add("%s: PDF page %d: %s (%s) opens the page's run at menu %s, the ledger (%s) records first_address %s",
				c.label, l.PDFPage, srcLabel, srcPath, first, c.ledgerPath, l.First)
		} else if got, want := collapseSpace(rows[first].Name), collapseSpace(l.FirstName); got != want {
			add("%s: PDF page %d: %s (%s) names menu %s %q, the ledger (%s) records first_name %q",
				c.label, l.PDFPage, srcLabel, srcPath, first, rows[first].Name, c.ledgerPath, l.FirstName)
		}
		if last != l.Last {
			add("%s: PDF page %d: %s (%s) closes the page's run at menu %s, the ledger (%s) records last_address %s",
				c.label, l.PDFPage, srcLabel, srcPath, last, c.ledgerPath, l.Last)
		} else if got, want := collapseSpace(rows[last].Name), collapseSpace(l.LastName); got != want {
			add("%s: PDF page %d: %s (%s) names menu %s %q, the ledger (%s) records last_name %q",
				c.label, l.PDFPage, srcLabel, srcPath, last, rows[last].Name, c.ledgerPath, l.LastName)
		}
	}
	if i != len(addrs) {
		add("%s: the ledger's (%s) row_counts consume %d of %s's (%s) %d addresses; the pages must tile the chart exactly",
			c.label, c.ledgerPath, i, srcLabel, srcPath, len(addrs))
	}
	return out
}

// --- the rulings ----------------------------------------------------------

// ruling is one recorded, cited divergence between A and B: the addresses it
// covers, the column it is on, the SHAPE the divergence must have, and the
// reason. Together they are the complete list of ways the two legs are allowed
// to disagree; anything else goes to arbitration.
//
// EVERY RULING IS CHECKED IN BOTH DIRECTIONS. Its addresses must all complain
// (a ruling that stops firing describes a leg that has moved), and each one's
// shape predicate must hold (so "PF rows differ" cannot quietly become "PF
// rows differ by something else").
type ruling struct {
	name  string
	field string
	addrs []chartAddr
	// holds is the shape the divergence must have, checked per address.
	holds func(a, b chartRow) bool
	why   string
}

// The four shapes. Each is small and named, so the ruling table reads as a
// list of decisions rather than a list of exceptions.
var (
	// shapePFKeyDigits: A reads four digits where B reads three.
	shapePFKeyDigits = func(a, b chartRow) bool { return a.Digits == 4 && b.Digits == 3 }

	// shapeParameterless: A carries the chart's hyphen cell, B an ordinary
	// three-digit width.
	shapeParameterless = func(a, b chartRow) bool { return a.Parameterless && b.Digits == 3 && !b.Parameterless }

	// shapeNumericLegend: B flags the row as text, A does not, and the two
	// legs agree about the WIDTH — which is the datum. A flag disagreement
	// that carried a width disagreement with it would be a different animal.
	shapeNumericLegend = func(a, b chartRow) bool { return !a.Text && b.Text && a.Digits == b.Digits }

	// shapeWrapSpace: the two names differ ONLY in spaces, and A is the one
	// with fewer of them.
	shapeWrapSpace = func(a, b chartRow) bool {
		return a.Name != b.Name && stripSpaces(a.Name) == stripSpaces(b.Name) &&
			strings.Count(collapseSpace(a.Name), " ") < strings.Count(collapseSpace(b.Name), " ")
	}

	// shapeSecondFragment: B's name is a proper leading fragment of A's.
	shapeSecondFragment = func(a, b chartRow) bool {
		an, bn := collapseSpace(a.Name), collapseSpace(b.Name)
		return an != bn && strings.HasPrefix(an, bn+" ")
	}
)

// addrRange builds the ascending run p1/p2/lo … p1/p2/hi, so a seventeen-row
// PF block is one line rather than seventeen.
func addrRange(p1, p2, lo, hi int) []chartAddr {
	out := make([]chartAddr, 0, hi-lo+1)
	for i := lo; i <= hi; i++ {
		out = append(out, chartAddr{p1, p2, i})
	}
	return out
}

// rulings890S is the complete list of ways the TS-890S's legs may disagree.
//
// The parameterless block takes its addresses FROM THE PROFILE rather than
// from a literal here: those four are the profile's own
// ParameterlessAddresses, and a bound consulted from one place with its datum
// taken from another is the defect shape internal/extable.Profile exists to
// prevent.
func rulings890S(p extable.Profile) []ruling {
	return []ruling{{
		name:  "R-B, the PF key width",
		field: fieldDigits,
		addrs: addrRange(0, 0, 15, 31),
		holds: shapePFKeyDigits,
		why: "Both books print \"PF key settings use 4 digits (refer to the PF Key assignment ID lists)\" " +
			"in the EX command's own P5 note (890:1918-1919, 990:1747-1748), and the PF ID list runs to 9999. " +
			"A carries 4 on all seventeen of this radio's PF rows; B read the chart's ordinary \"3-digit\" " +
			"legend instead. THE LEG IS WRONG, NOT A — and the leg is frozen evidence, so the error is " +
			"recorded here rather than corrected there.",
	}, {
		name:  "R-PARAMETERLESS, the four \"Does not correspond to a command\" rows",
		field: fieldDigits,
		addrs: profileParameterlessAddrs(p),
		holds: shapeParameterless,
		why: "The chart's P5 cell on these four rows reads \"Does not correspond to a command\" " +
			"(890:2270-2279), so no EX frame can read or write them. A spells that as the hyphen cell " +
			"extable.ParseCSV admits under ParameterlessExcluded; B was told nothing of the convention and " +
			"recorded the ordinary 3. The rows ARE counted in the 162 and are omitted from the inventory BY " +
			"ADDRESS, which TestCrossCheck_The890SExclusionsArePinnedByAddress pins.",
	}, {
		name:  "the numeric-legend text flag",
		field: fieldText,
		addrs: []chartAddr{{0, 5, 12}, {1, 0, 5}},
		holds: shapeNumericLegend,
		why: "Contest Number (0/05/12) prints \"(4-digit)\" and Reference Oscillator Calibration (1/00/05) " +
			"prints a numeric parameter range; a prose cell reading \"(n-digit)\" is a NUMERIC legend and so " +
			"text=0, which is pair 1's own ruling (core/kw/ts480/crosscheck_test.go's merged-cell ruling) " +
			"applied to the same shape of cell. ONLY a row reading \"Up to N alphanumeric characters\" is " +
			"text (890:1946-1947), which is the pair TestCrossCheck_TheTextRowsAreTheTwoPrintedOnes pins. " +
			"B's own record flags both as judgement calls rather than resolving them.",
	}, {
		name:  "the render's line-wrap space",
		field: fieldName,
		addrs: []chartAddr{{0, 3, 0}, {0, 3, 3}, {0, 3, 4}, {0, 6, 8}, {0, 6, 9}},
		holds: shapeWrapSpace,
		why: "Five Function cells wrap mid-token in the printed chart — \"Multi/ Channel Control\", " +
			"\"AM- DATA\" — and B, derived from a render, carries the wrap as a space where A, derived from " +
			"the layout text and closed up by lane P's fix 1, does not. The difference is spaces alone, " +
			"which shapeWrapSpace requires; A's reading is the inventory's.",
	}}
}

// rulings990S is the complete list for the TS-990S. It has no parameterless
// entry: this book DASH-ADDRESSES its four \"Does not correspond to a command\"
// rows (990:2272-2279), so they carry no address at all, are not counted in
// the 194 and appear in neither transcription.
func rulings990S() []ruling {
	return []ruling{{
		name:  "R-B, the PF key width",
		field: fieldDigits,
		addrs: addrRange(0, 0, 15, 32),
		holds: shapePFKeyDigits,
		why: "990:1747-1748, the same printed sentence as the TS-890S's, over this radio's eighteen PF " +
			"rows. See rulings890S for the full reasoning.",
	}, {
		name:  "the numeric-legend text flag",
		field: fieldText,
		addrs: append([]chartAddr{{0, 5, 11}}, addrRange(0, 8, 5, 32)...),
		holds: shapeNumericLegend,
		why: "Contest Number (0/05/11) prints \"(4-digit)\", and the twenty-eight Fixed-Mode band limit rows " +
			"(0/08/05 … 0/08/32) print \"8-digit frequency (in Hz) with unused digits entered as 0\" " +
			"(990:2070 onwards) — the MaxDigits 8 class. Both are NUMERIC legends and so text=0. Only " +
			"990:1778-1779's two \"Up to N alphanumeric characters\" rows are text.",
	}, {
		name:  "the doubled Function cell at 0/03/01",
		field: fieldName,
		addrs: []chartAddr{{0, 3, 1}},
		holds: shapeSecondFragment,
		why: "This chart prints ONE ruled row whose Function cell carries TWO firmware-scoped sentences — " +
			"\"… <Firmware version 1.20 or later>\" followed by \"… <Firmware version 1.13 or lower>\". A " +
			"transcribes the cell verbatim and entire; B recorded only the leading sentence. It is one row " +
			"on both legs, which the address membership and the 194 total already say; only the Function " +
			"cell's extent differs, and A's is the chart's.",
	}}
}

// profileParameterlessAddrs projects the profile's own ParameterlessAddresses
// onto this file's key type.
func profileParameterlessAddrs(p extable.Profile) []chartAddr {
	out := make([]chartAddr, 0, len(p.ParameterlessAddresses))
	for _, a := range p.ParameterlessAddresses {
		out = append(out, chartAddr{a[0], a[1], a[2]})
	}
	return out
}

// TestCrossCheck_A_B_Ledger is the milestone's A = B = ledger = ExpectedRows
// leg, for both radios. The pass condition is that every complaint is a ruled
// one and every ruling fires.
func TestCrossCheck_A_B_Ledger(t *testing.T) {
	for _, c := range loadCharts(t) {
		t.Run(c.label, func(t *testing.T) {
			ruled := map[complaintKey]*ruling{}
			for i := range c.rulings {
				r := &c.rulings[i]
				for _, a := range r.addrs {
					k := complaintKey{a, r.field}
					if prev, dup := ruled[k]; dup {
						t.Fatalf("two rulings cover %s's %s column: %q and %q — a row must have ONE recorded reading", a, r.field, prev.name, r.name)
					}
					ruled[k] = r
				}
			}

			fired := map[complaintKey]bool{}
			for _, cp := range crossCheck(c) {
				k := complaintKey{cp.Addr, cp.Field}
				r, ok := ruled[k]
				if !ok {
					t.Errorf("CROSS-CHECK DISAGREEMENT WITH NO RECORDED RULING — THIS IS A STOP FOR ARBITRATION AGAINST THE PDF, not a test to adjust:\n%s", cp.Detail)
					continue
				}
				fired[k] = true
				if !r.holds(c.a[cp.Addr], c.b[cp.Addr]) {
					t.Errorf("menu %s: the %s column disagrees, and ruling %q covers it, but the disagreement is NOT THE SHAPE THAT RULING DESCRIBES — a STOP, not a pin to widen:\n%s\n\nThe ruling says: %s",
						cp.Addr, cp.Field, r.name, cp.Detail, r.why)
				}
			}

			for k, r := range ruled {
				if !fired[k] {
					t.Errorf("ruling %q covers menu %s's %s column, but the two legs AGREE there — a ruling that has stopped firing describes a leg that has moved, and is removed deliberately rather than left standing.\n\nThe ruling says: %s",
						r.name, k.addr, k.field, r.why)
				}
			}
		})
	}
}

type complaintKey struct {
	addr  chartAddr
	field string
}

// TestCrossCheck_TheChartShapePolicies asserts of transcription A the facts
// each registered profile declares about its chart's shape, so that the
// inventory-vs-A leg's claims rest on a source checked for the same things.
//
// ParseCSV already refuses a non-blank label and an out-of-domain address
// under LabelsAbsent and AddressGrouped, so this leg is not the first line of
// defence; it is here because a claim about a generated file is only worth
// making if the source it was generated from carries it too.
func TestCrossCheck_TheChartShapePolicies(t *testing.T) {
	for _, tc := range []struct{ name string }{{"ts890s"}, {"ts990s"}} {
		t.Run(tc.name, func(t *testing.T) {
			p := lookupProfile(t, tc.name)
			for _, r := range parseTranscriptionA(t, p) {
				addr := chartAddr{r.P1, r.P2, r.P3}
				if r.P1Label != "" || r.P2Label != "" {
					t.Errorf("menu %s (%q): transcription A (%s) carries labels p1_label=%q p2_label=%q, but neither chart prints a group-label column (LabelsAbsent)", addr, r.Name, p.ManualCSV, r.P1Label, r.P2Label)
				}
				if r.P1 != 0 && r.P1 != 1 {
					t.Errorf("menu %s (%q): transcription A (%s) carries p1=%d, and P1 is the menu-type enumeration with exactly two values, 0 Menu and 1 Advanced Menu (890:1897-1900, 990:1720-1723)", addr, r.Name, p.ManualCSV, r.P1)
				}
			}
		})
	}
}

// TestCrossCheck_TheTextRowsAreTheTwoPrintedOnes pins each chart's text rows
// as a set with their widths, which is the positive half of the numeric-legend
// ruling: only a row reading "Up to N alphanumeric characters" is a text row
// (890:1946-1947, 990:1778-1779), and each book prints exactly two.
func TestCrossCheck_TheTextRowsAreTheTwoPrintedOnes(t *testing.T) {
	want := map[string]map[chartAddr]int{
		"TS-890S": {{0, 0, 5}: 10, {0, 0, 6}: 15},
		"TS-990S": {{0, 0, 6}: 10, {0, 0, 7}: 15},
	}
	for _, c := range loadCharts(t) {
		t.Run(c.label, func(t *testing.T) {
			got := map[chartAddr]int{}
			for a, r := range c.a {
				if r.Text {
					got[a] = r.Digits
				}
			}
			if !reflect.DeepEqual(got, want[c.label]) {
				t.Errorf("transcription A (%s) marks text rows %v; the chart prints exactly two, %v, and their widths are the profile's own TextWidths", c.aPath, got, want[c.label])
			}
		})
	}
}

// TestCrossCheck_TheTwoChartsArePinnedAsTwo is the non-borrowing rule applied
// to a menu chart, and it is the leg no single-radio test can write.
//
// The two books print charts of DIFFERENT LENGTHS whose address spaces overlap
// heavily and whose meanings do not: 153 addresses appear in both, and only 42
// of those name the same setting. An address resolved against the wrong row
// would therefore publish a real, plausible, WRONG setting name a hundred and
// eleven times over — which is precisely why each layout consults its own
// inventory and never a shared table.
func TestCrossCheck_TheTwoChartsArePinnedAsTwo(t *testing.T) {
	charts := loadCharts(t)
	c890, c990 := charts[0], charts[1]
	l890, l990 := ma.Layout890(), ma.Layout990()

	// 1. The lengths differ by what the ledgers say, and by nothing else.
	if got, want := c990.expectedRows-c890.expectedRows, ledgerSum(c990.ledger)-ledgerSum(c890.ledger); got != want {
		t.Errorf("the two profiles' ExpectedRows differ by %d and the two ledgers' row_count sums differ by %d", got, want)
	}
	if len(c890.a) == len(c990.a) {
		t.Errorf("both transcriptions hold %d rows; these are two different books' charts and the milestone's whole premise is that they are not the same size", len(c890.a))
	}

	// 2. An address in one inventory and not the other resolves in exactly
	//    one layout. The four addresses the TS-890S profile excludes are not
	//    in ITS inventory by construction, so the statement is made about the
	//    published tables, which is what a driver reaches.
	in890, in990 := inventoryAddrs(l890.EXItems()), inventoryAddrs(l990.EXItems())
	only890, only990 := 0, 0
	for a := range in890 {
		if _, both := in990[a]; !both {
			only890++
			if _, ok := l990.EXItem(exAddr(a)); ok {
				t.Errorf("menu %s is in the TS-890S inventory and not the TS-990S's, yet Layout990 resolves it", a)
			}
		}
	}
	for a := range in990 {
		if _, both := in890[a]; !both {
			only990++
			if _, ok := l890.EXItem(exAddr(a)); ok {
				t.Errorf("menu %s is in the TS-990S inventory and not the TS-890S's, yet Layout890 resolves it", a)
			}
		}
	}
	if only890 == 0 || only990 == 0 {
		t.Errorf("%d addresses are the TS-890S's alone and %d the TS-990S's alone; if either were zero one chart would be a subset of the other and borrowing would be invisible", only890, only990)
	}

	// 3. A shared address resolves to ITS OWN BOOK'S setting on each row. This
	//    is the copy-paste the file split exists to prevent, stated over the
	//    live inventories.
	differ := 0
	for a, it890 := range in890 {
		it990, both := in990[a]
		if !both {
			continue
		}
		if collapseSpace(it890.Name) == collapseSpace(it990.Name) {
			continue
		}
		differ++
		if got := c890.a[a].Name; it890.Name != got {
			t.Errorf("menu %s: the TS-890S inventory names it %q and its own transcription %q", a, it890.Name, got)
		}
		if got := c990.a[a].Name; it990.Name != got {
			t.Errorf("menu %s: the TS-990S inventory names it %q and its own transcription %q", a, it990.Name, got)
		}
	}
	if differ == 0 {
		t.Error("no shared address names a different setting on the two radios; that would make cross-model borrowing undetectable, and it is not what the two charts print")
	}
}

// TestRedProof_CopyPastingOneChartUnderTheOtherProfileIsRefused fires the
// falsification the bullet above cannot: whole-file substitution.
//
// Each direction is refused by a DIFFERENT declared policy, which is the point
// — neither refusal is a lucky coincidence of row counts.
func TestRedProof_CopyPastingOneChartUnderTheOtherProfileIsRefused(t *testing.T) {
	p890, p990 := lookupProfile(t, "ts890s"), lookupProfile(t, "ts990s")

	data990, err := os.ReadFile(p990.ManualCSV)
	if err != nil {
		t.Fatalf("reading %s: %v", p990.ManualCSV, err)
	}
	if _, err := extable.ParseCSV(p890, data990); err == nil {
		t.Errorf("ParseCSV(ts890s, %s) succeeded: the TS-990S chart carries twenty-eight 8-digit frequency rows and the TS-890S profile declares MaxDigits %d, so this must be refused", p990.ManualCSV, p890.MaxDigits)
	} else {
		t.Logf("refused as required: %v", err)
	}

	data890, err := os.ReadFile(p890.ManualCSV)
	if err != nil {
		t.Fatalf("reading %s: %v", p890.ManualCSV, err)
	}
	if _, err := extable.ParseCSV(p990, data890); err == nil {
		t.Errorf("ParseCSV(ts990s, %s) succeeded: the TS-890S chart carries four hyphen-width rows and the TS-990S profile declares ParameterlessRefused, so this must be refused", p890.ManualCSV)
	} else {
		t.Logf("refused as required: %v", err)
	}
}

// TestRedProof_The890SProfileRefusesAWidth8Row is the sharper half of the same
// statement, and the one the sterile A legs could not write: the MaxDigits 8
// class is the TS-990S's and DOES NOT TRANSFER.
//
// It takes a real 8-digit row out of the TS-990S transcription and offers it,
// alone, under the TS-890S profile. A row that is well-formed in every other
// respect must still be refused on its width.
func TestRedProof_The890SProfileRefusesAWidth8Row(t *testing.T) {
	p890, p990 := lookupProfile(t, "ts890s"), lookupProfile(t, "ts990s")

	var record string
	for _, line := range csvBody(t, p990.ManualCSV) {
		// 0/08/05, the first Fixed Mode band limit row (990:2070).
		if strings.HasPrefix(line, "0,08,05,") {
			record = line
			break
		}
	}
	if record == "" {
		t.Fatalf("no 0/08/05 row in %s; this proof needs a real width-8 row from that chart", p990.ManualCSV)
	}
	if !strings.Contains(record, ",8,") {
		t.Fatalf("the 0/08/05 row of %s is %q, which does not carry the width 8 this proof is about", p990.ManualCSV, record)
	}

	if _, err := extable.ParseCSV(p890, []byte(record+"\n")); err == nil {
		t.Errorf("ParseCSV(ts890s) accepted %q: the TS-890S's widest printed parameter is %d digits (its book prints no frequency-setting class at all), and a width-8 row is one that chart does not have", record, p890.MaxDigits)
	} else {
		t.Logf("refused as required: %v", err)
	}
}

// TestCrossCheck_The890SExclusionsArePinnedByAddress pins the four omissions
// BY ADDRESS rather than by count — a count-only gate is satisfied by any four
// omissions — and pins the addressless firmware row's absence with its reason.
func TestCrossCheck_The890SExclusionsArePinnedByAddress(t *testing.T) {
	p := lookupProfile(t, "ts890s")
	a := transcriptionA(t, p)
	inv := inventoryAddrs(ma.EXItems890S())

	want := map[chartAddr]bool{}
	for _, addr := range profileParameterlessAddrs(p) {
		want[addr] = true
	}
	if len(want) != 4 {
		t.Fatalf("the profile names %d parameterless addresses, want the four \"Does not correspond to a command\" rows", len(want))
	}

	var missing []chartAddr
	for _, addr := range sortedAddrs(a) {
		if _, in := inv[addr]; !in {
			missing = append(missing, addr)
		}
	}
	for _, addr := range missing {
		if !want[addr] {
			t.Errorf("menu %s is in transcription A (%s) and absent from the inventory, and the profile does NOT name it parameterless — the inventory omits exactly 1/00/23 … 1/00/26 and nothing else", addr, p.ManualCSV)
		}
	}
	if len(missing) != len(want) {
		t.Errorf("the inventory omits %v; the profile names %d parameterless addresses, and a row omitted for any other reason is a STOP", missing, len(want))
	}
	for addr := range want {
		if _, in := inv[addr]; in {
			t.Errorf("menu %s is in the inventory, but the profile names it parameterless — that row's chart cell reads \"Does not correspond to a command\" and names no field an EX frame could read or write", addr)
		}
		if _, in := a[addr]; !in {
			t.Errorf("menu %s is not in transcription A (%s), but it IS counted in the 162: the four rows are transcribed and counted, and excluded from the inventory by address", addr, p.ManualCSV)
		}
	}

	// THE ADDRESSLESS FIRMWARE ROW. The chart's last printed line reads
	// "1 — 27 Firmware Version … Reading command only" (890:2281): its P2 cell
	// is an em dash, not an address. It is in NEITHER the CSV nor the
	// inventory, and it cannot be: no AddressGrouped domain can express a
	// literal em dash, and FV; already reads the version — which the FV
	// builder and FV-890.golden's replay below cover.
	for _, addr := range sortedAddrs(a) {
		if addr[0] == 1 && addr[2] == 27 {
			t.Errorf("transcription A (%s) carries menu %s; the chart's 1 — 27 Firmware Version row prints an em dash where its P2 address should be (890:2281) and is not an addressed row", p.ManualCSV, addr)
		}
	}
	for addr := range inv {
		if addr[0] == 1 && addr[2] == 27 {
			t.Errorf("the inventory carries menu %s; the 1 — 27 Firmware Version row has no address to carry (890:2281)", addr)
		}
	}
}

// TestCrossCheck_InventoryAgainstTranscriptionA binds each GENERATED inventory
// to the transcription it is generated from.
//
// The staleness tests already re-render the generated file from its CSV and
// byte-compare it, which catches drift between the two. This test binds
// something else: what the inventory MEANS once loaded — that the rows are the
// chart's rows, in address order, carrying this family's shape and this
// chart's widths — reached through the package accessors every consumer uses.
func TestCrossCheck_InventoryAgainstTranscriptionA(t *testing.T) {
	for _, tc := range []struct {
		profile string
		items   []kw.EXItem
	}{
		{"ts890s", ma.EXItems890S()},
		{"ts990s", ma.EXItems990S()},
	} {
		t.Run(tc.profile, func(t *testing.T) {
			p := lookupProfile(t, tc.profile)
			a := transcriptionA(t, p)
			excluded := map[chartAddr]bool{}
			for _, addr := range profileParameterlessAddrs(p) {
				excluded[addr] = true
			}
			var order []chartAddr
			for _, addr := range sortedAddrs(a) {
				if !excluded[addr] {
					order = append(order, addr)
				}
			}
			if len(tc.items) != len(order) {
				t.Fatalf("the generated inventory holds %d items, transcription A (%s) holds %d rows less %d excluded addresses", len(tc.items), p.ManualCSV, len(a), len(excluded))
			}
			// Ascending address order in its own right, not merely a
			// permutation that matches once sorted: the generated file's doc
			// comment claims the sort, so the claim is pinned.
			for i := 1; i < len(tc.items); i++ {
				prev, cur := itemAddr(tc.items[i-1]), itemAddr(tc.items[i])
				if !addrLess(prev, cur) {
					t.Errorf("the generated inventory is not in ascending address order at index %d: %s follows %s", i, cur, prev)
				}
			}
			for i, want := range order {
				got := tc.items[i]
				if itemAddr(got) != want {
					t.Errorf("inventory item %d is menu %s, transcription A (%s) has %s there", i, itemAddr(got), p.ManualCSV, want)
					continue
				}
				row := a[want]
				if got.Name != row.Name {
					t.Errorf("menu %s: the inventory's Name is %q, transcription A (%s) has %q", want, got.Name, p.ManualCSV, row.Name)
				}
				if got.Digits != row.Digits {
					t.Errorf("menu %s (%q): the inventory's Digits is %d, transcription A (%s) has %s", want, row.Name, got.Digits, p.ManualCSV, row.digitsCell())
				}
				if got.Text != row.Text {
					t.Errorf("menu %s (%q): the inventory's Text is %v, transcription A (%s) has %v", want, row.Name, got.Text, p.ManualCSV, row.Text)
				}
				if got.P1Label != "" || got.P2Label != "" {
					t.Errorf("menu %s (%q): the inventory carries labels P1Label=%q P2Label=%q, but neither chart prints a label column (LabelsAbsent)", want, row.Name, got.P1Label, got.P2Label)
				}
				// ObservationsAbsent: neither radio has ever been asked
				// anything by this project, so both observation fields must
				// carry their absence sentinels. A non-zero one here would be
				// a hardware claim nothing supports.
				if got.ObservedReadWidth != 0 || got.ObservedReadShape != "" {
					t.Errorf("menu %s (%q): the inventory carries ObservedReadWidth=%d ObservedReadShape=%q, but this profile registers ObservationsAbsent and no %s has ever been asked anything",
						want, row.Name, got.ObservedReadWidth, got.ObservedReadShape, p.Model)
				}
			}
		})
	}
}

// TestCrossCheck_BothProfilesResolveByName is lane P's integration check (P4),
// asserted here because it is the one nothing else fails on: a profile var
// merged without its registry key compiles silently and every other test in
// this package stays green.
func TestCrossCheck_BothProfilesResolveByName(t *testing.T) {
	for _, name := range []string{"ts890s", "ts990s"} {
		if _, ok := extable.Lookup(name); !ok {
			var registered []string
			for _, np := range extable.RegisteredProfiles() {
				registered = append(registered, np.Name)
			}
			t.Errorf("extable.Lookup(%q) found no profile; registered: %v", name, registered)
		}
	}
}

// --- the falsifications ---------------------------------------------------
//
// One per leg, fired against an IN-MEMORY mutation of the loaded evidence. No
// artefact is ever written. Without these the cross-check's green is
// unfalsifiable: a comparison that silently compared nothing would pass every
// run above just as happily.

// TestRedProof_DroppingARowFromTranscriptionA is caught, on both radios.
func TestRedProof_DroppingARowFromTranscriptionA(t *testing.T) {
	for _, c := range loadCharts(t) {
		t.Run(c.label, func(t *testing.T) {
			// The LAST row of the chart, so the falsification moves the final
			// page's boundary and the four-way total at once.
			victim := sortedAddrs(c.a)[len(c.a)-1]
			c.a = withoutAddr(c.a, victim)
			requireComplaint(t, crossCheck(c), "menu "+victim.String())
		})
	}
}

// TestRedProof_DroppingARowFromTranscriptionB is caught, on both radios.
func TestRedProof_DroppingARowFromTranscriptionB(t *testing.T) {
	for _, c := range loadCharts(t) {
		t.Run(c.label, func(t *testing.T) {
			// The FIRST row this time, so the two proofs between them exercise
			// both ends of the tiling.
			victim := sortedAddrs(c.b)[0]
			c.b = withoutAddr(c.b, victim)
			requireComplaint(t, crossCheck(c), "menu "+victim.String())
		})
	}
}

// TestRedProof_PerturbingAWidth is caught — the leg no row-count check could
// ever see, because the counts still agree.
func TestRedProof_PerturbingAWidth(t *testing.T) {
	for _, c := range loadCharts(t) {
		t.Run(c.label, func(t *testing.T) {
			// A row NO ruling covers, so the complaint cannot be absorbed by
			// one: the first address whose digits column is unruled.
			victim := firstUnruledAddr(c, fieldDigits)
			row := c.a[victim]
			row.Digits++
			c.a = withAddr(c.a, victim, row)
			requireComplaint(t, crossCheck(c), "the DIGITS differ")
		})
	}
}

// TestRedProof_PerturbingTheLedgerSum is caught. row_count is what
// ExpectedRows was derived from, so a ledger that quietly disagreed with itself
// would take the profile's bound with it.
//
// BOTH consequences are required, not either: the four-way total moves, and the
// page the count belongs to stops tiling. Requiring only the total would leave
// checkAgainstLedger unfalsified.
func TestRedProof_PerturbingTheLedgerSum(t *testing.T) {
	for _, c := range loadCharts(t) {
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

// TestRedProof_PerturbingALedgerBoundaryName is caught. It is the leg no count
// can see: the page boundaries and their addresses still tile, and only the
// NAME the ledger recorded for a boundary row has moved.
func TestRedProof_PerturbingALedgerBoundaryName(t *testing.T) {
	for _, c := range loadCharts(t) {
		t.Run(c.label, func(t *testing.T) {
			ledger := append([]ledgerRow(nil), c.ledger...)
			ledger[0].FirstName += " (perturbed)"
			c.ledger = ledger
			requireComplaint(t, crossCheck(c), "records first_name")
		})
	}
}

// requireComplaint asserts the cross-check bit, and that at least one
// complaint names the mutation — a leg that failed for an unrelated reason
// would prove nothing about the leg under falsification.
func requireComplaint(t *testing.T, complaints []complaint, want string) {
	t.Helper()
	if len(complaints) == 0 {
		t.Fatalf("the cross-check reported NO complaint against the falsified evidence: the leg that should have caught %q is not binding anything", want)
	}
	var all []string
	for _, c := range complaints {
		if strings.Contains(c.Detail, want) {
			return
		}
		all = append(all, c.Detail)
	}
	t.Errorf("the cross-check complained, but no complaint names %q; it caught something else:\n  %s", want, strings.Join(all, "\n  "))
}

// firstUnruledAddr returns the lowest address of c whose named column no
// ruling covers, so a perturbation there cannot be mistaken for a recorded
// divergence.
func firstUnruledAddr(c chart, field string) chartAddr {
	ruled := map[chartAddr]bool{}
	for _, r := range c.rulings {
		if r.field != field {
			continue
		}
		for _, a := range r.addrs {
			ruled[a] = true
		}
	}
	for _, a := range sortedAddrs(c.a) {
		if !ruled[a] {
			return a
		}
	}
	panic("every address is ruled on " + field)
}

// --- loaders --------------------------------------------------------------

// loadCharts returns the TS-890S chart then the TS-990S chart, in that order.
func loadCharts(t *testing.T) []chart {
	t.Helper()
	p890 := lookupProfile(t, "ts890s")
	p990 := lookupProfile(t, "ts990s")
	return []chart{{
		label:        "TS-890S",
		aPath:        p890.ManualCSV,
		bPath:        transcriptionB890Path,
		ledgerPath:   pageLedger890Path,
		expectedRows: p890.ExpectedRows,
		a:            transcriptionA(t, p890),
		b:            loadTranscriptionB(t, transcriptionB890Path),
		ledger:       loadPageLedger(t, pageLedger890Path),
		rulings:      rulings890S(p890),
	}, {
		label:        "TS-990S",
		aPath:        p990.ManualCSV,
		bPath:        transcriptionB990Path,
		ledgerPath:   pageLedger990Path,
		expectedRows: p990.ExpectedRows,
		a:            transcriptionA(t, p990),
		b:            loadTranscriptionB(t, transcriptionB990Path),
		ledger:       loadPageLedger(t, pageLedger990Path),
		rulings:      rulings990S(),
	}}
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

// parseTranscriptionA reads a chart's CSV through extable.ParseCSV — the same
// parser, the same bounds, the same duplicate-address refusal and the same
// AddressGrouped rule the generator runs.
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

// transcriptionA projects A onto the comparison tuple, keyed by the grouped
// address.
func transcriptionA(t *testing.T, p extable.Profile) map[chartAddr]chartRow {
	t.Helper()
	rows := parseTranscriptionA(t, p)
	out := make(map[chartAddr]chartRow, len(rows))
	for _, r := range rows {
		out[chartAddr{r.P1, r.P2, r.P3}] = chartRow{Name: r.Name, Digits: r.Digits, Text: r.Text, Parameterless: r.Parameterless}
	}
	// ParseCSV refuses a duplicate (P1,P2,P3), so this map cannot silently
	// absorb one; the check is here because "cannot" is an argument and this
	// is a fact.
	if len(out) != len(rows) {
		t.Fatalf("transcription A (%s) holds %d rows but only %d distinct addresses", p.ManualCSV, len(rows), len(out))
	}
	return out
}

// loadTranscriptionB reads B with encoding/csv. Its name is taken with NO
// normalisation at all; the collapsing this file does happens at COMPARISON
// time, so the evidence is never rewritten on the way in.
func loadTranscriptionB(t *testing.T, path string) map[chartAddr]chartRow {
	t.Helper()
	records := readEvidenceCSV(t, path, transcriptionBHeader, transcriptionBFields)

	out := make(map[chartAddr]chartRow, len(records))
	for i, rec := range records {
		where := fmt.Sprintf("%s data row %d", path, i+1)
		addr := chartAddr{
			atoiOrFatal(t, where, "p1", rec[0]),
			atoiOrFatal(t, where, "p2", rec[1]),
			atoiOrFatal(t, where, "p3", rec[2]),
		}
		if prev, dup := out[addr]; dup {
			t.Fatalf("%s: duplicate address %s (already held %s)", where, addr, prev)
		}
		out[addr] = chartRow{
			Name:   rec[3],
			Digits: atoiOrFatal(t, where, "digits", rec[4]),
			Text:   parseBoolDigit(t, where, "text", rec[5]),
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
		page := atoiOrFatal(t, where, "pdf_page", rec[0])
		if seen[page] {
			t.Fatalf("%s: duplicate PDF page %d", where, page)
		}
		seen[page] = true
		out = append(out, ledgerRow{
			PDFPage:   page,
			First:     parseLedgerAddr(t, where+" first_address", rec[1]),
			FirstName: rec[2],
			Last:      parseLedgerAddr(t, where+" last_address", rec[3]),
			LastName:  rec[4],
			RowCount:  atoiOrFatal(t, where, "row_count", rec[5]),
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
// companion), but the two menu CSVs are '#'-commented and a provenance block
// added to one of these later must not break this parser. FieldsPerRecord is
// set explicitly rather than inferred from the first record, so a file that is
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

// parseLedgerAddr decodes a ledger's address cell, "P1 P2 P3" with each
// component printed as the chart prints it.
//
// It is deliberately exact. A cell that is merely similar is a Fatal, not a
// silent partial parse, because a permissive reading is precisely how two
// genuinely different addresses would be folded into false agreement.
func parseLedgerAddr(t *testing.T, where, raw string) chartAddr {
	t.Helper()
	parts := strings.Fields(raw)
	if len(parts) != 3 {
		t.Fatalf("%s: %q is not a three-component menu address", where, raw)
	}
	if len(parts[0]) != 1 || len(parts[1]) != 2 || len(parts[2]) != 2 {
		t.Fatalf("%s: %q does not print as one, two and two digits, which is what both charts print (890:1897-1911, 990:1720-1735)", where, raw)
	}
	var addr chartAddr
	for i, p := range parts {
		for j := 0; j < len(p); j++ {
			if p[j] < '0' || p[j] > '9' {
				t.Fatalf("%s: %q carries a non-digit in component %d", where, raw, i+1)
			}
		}
		addr[i] = atoiOrFatal(t, where, "address component", p)
	}
	return addr
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

func atoiOrFatal(t *testing.T, where, field, raw string) int {
	t.Helper()
	n, err := strconv.Atoi(raw)
	if err != nil {
		t.Fatalf("%s: bad %s %q: %v", where, field, raw, err)
	}
	return n
}

// csvBody returns a menu CSV's data rows with its provenance comments and
// blank lines removed, so a test can offer ONE row to another profile without
// reproducing the transcription's facts here.
func csvBody(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var out []string
	for _, l := range strings.Split(string(data), "\n") {
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		out = append(out, l)
	}
	return out
}

// --- small helpers --------------------------------------------------------

// sortedAddrs returns m's addresses in ascending order, so that every failure
// list is stable and the ledger tiling walks each chart in the order its pages
// print it.
func sortedAddrs(m map[chartAddr]chartRow) []chartAddr {
	out := make([]chartAddr, 0, len(m))
	for a := range m {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return addrLess(out[i], out[j]) })
	return out
}

func addrLess(x, y chartAddr) bool {
	for i := range x {
		if x[i] != y[i] {
			return x[i] < y[i]
		}
	}
	return false
}

func itemAddr(it kw.EXItem) chartAddr {
	return chartAddr{int(it.Addr.P1), int(it.Addr.P2), int(it.Addr.P3)}
}

func exAddr(a chartAddr) kw.EXAddress {
	return kw.EXAddress{P1: uint8(a[0]), P2: uint8(a[1]), P3: uint8(a[2])}
}

func inventoryAddrs(items []kw.EXItem) map[chartAddr]kw.EXItem {
	out := make(map[chartAddr]kw.EXItem, len(items))
	for _, it := range items {
		out[itemAddr(it)] = it
	}
	return out
}

func ledgerSum(rows []ledgerRow) int {
	sum := 0
	for _, r := range rows {
		sum += r.RowCount
	}
	return sum
}

// withoutAddr and withAddr return an independent copy of m with one address
// removed or replaced, so a red proof never mutates the map another test is
// reading.
func withoutAddr(m map[chartAddr]chartRow, addr chartAddr) map[chartAddr]chartRow {
	out := make(map[chartAddr]chartRow, len(m))
	for k, v := range m {
		if k != addr {
			out[k] = v
		}
	}
	return out
}

func withAddr(m map[chartAddr]chartRow, addr chartAddr, row chartRow) map[chartAddr]chartRow {
	out := make(map[chartAddr]chartRow, len(m))
	for k, v := range m {
		out[k] = v
	}
	out[addr] = row
	return out
}

// --- evidence leg G: the geometry vectors, replayed ------------------------
//
// The eleven *-890/-990.golden files are forty-nine hand-derived wire frames,
// counted off the two books' printed position rulers by a quarantined agent
// that never opened this repository: no code, no generator, no fixture and no
// other document, only renders of the charts and their parameter legends.
// Every field width, every position boundary and every assumption that had to
// be inherited rather than read is itemised in testdata/provenance.md and
// repeated in each file's own header block, which the roster below cites for
// the lengths it hardcodes.
//
// A GOLDEN-VS-CODEC MISMATCH IS A STOP for orchestrator arbitration AGAINST
// THE PDF — either the hand derivation or the codec misreads the book — which
// is why every failure below prints both sides and both lengths: the failure
// output is the arbitration's input. No vector is ever edited to make a test
// pass.
//
// # What "replayed" means here, and why some frames are not built
//
// Each vector carries its disposition. replayBuild means this package produces
// those exact bytes; replayParse means it decodes them into the fields pinned
// below; replayNone means it has NO entry point for that frame BY DESIGN, and
// the roster records which standing rule says so. The three sets are pinned
// together with the roster, so a frame sliding from "built" to "no builder" —
// or the reverse, which would be a new frame reaching a radio — moves a
// literal in this file.
//
// # Hardware status
//
// UNVERIFIED, for all forty-nine. Neither radio has ever been asked anything
// by this project (A19). Green here means the frames satisfy the grammars both
// books print, as one agent read those books, and NOT that any radio accepts
// these bytes.

type replay int

const (
	replayBuild replay = iota
	replayParse
	replayNone
)

func (r replay) String() string {
	switch r {
	case replayBuild:
		return "built"
	case replayParse:
		return "parsed"
	}
	return "no entry point"
}

// goldenVector is one roster entry: the file, the vector's name, the frame
// length the DERIVER COUNTED off the printed ruler, and how this package
// replays it.
//
// THE LENGTHS ARE LITERALS, transcribed from each file's own "COUNTED FRAME
// LENGTHS" block or from the per-vector comment beside the frame, and the
// vectors are the independent side: a length computed from the bytes it checks
// would prove nothing.
type goldenVector struct {
	File       string
	Name       string
	CountedLen int
	How        replay
	Why        string // replayNone only: the rule that says this package builds no such frame
}

// The three reasons a frame in this evidence is not replayed through an entry
// point, each a standing rule of this repository rather than an omission.
const (
	whyNoTransceiveSet = "the standing no-transceive-set rule: this programme turns auto-information OFF at init " +
		"and never on, so AI2; and AI4; have no builder. The answer forms are the same four bytes as the sets " +
		"and are not correlated either — nothing here ever reads an AI answer."
	whyNoEXWrite = "the EX Set direction is not built at all in this milestone: settings write is a later " +
		"milestone's scope, and an omitted config semantic is REFUSED rather than defaulted. P4 = '9' " +
		"(initial value setting) is doubly out of scope — it is a factory-reset write."
	whyMNOffTheRoster = "decision 5 removed MN from the outbound roster in both directions: the MA0 Read carries " +
		"its own channel number (A18), so no channel-select frame is needed, and one that was built would be a " +
		"frame this programme has no use for reaching a radio."
)

// goldenRoster is every vector in every file, in the order its file prints
// them. Order is part of the pin: the walk below requires the roster and the
// files to be the same list, so a vector added, removed or reordered fails
// here rather than being quietly skipped.
var goldenRoster = []goldenVector{
	// AI — Set 4, Read 3, Answer 4 on both books; AI-990's header records the
	// geometry as identical to the 890S's.
	{"AI-890.golden", "set_off", 4, replayBuild, ""},
	{"AI-890.golden", "set_on_not_backed_up", 4, replayNone, whyNoTransceiveSet},
	{"AI-890.golden", "set_on_backed_up", 4, replayNone, whyNoTransceiveSet},
	{"AI-890.golden", "read", 3, replayBuild, ""},
	{"AI-890.golden", "answer_off", 4, replayBuild, ""},
	{"AI-890.golden", "answer_on_not_backed_up", 4, replayNone, whyNoTransceiveSet},
	{"AI-890.golden", "answer_on_backed_up", 4, replayNone, whyNoTransceiveSet},
	{"AI-990.golden", "set_off", 4, replayBuild, ""},
	{"AI-990.golden", "set_on_not_backed_up", 4, replayNone, whyNoTransceiveSet},
	{"AI-990.golden", "set_on_backed_up", 4, replayNone, whyNoTransceiveSet},
	{"AI-990.golden", "read", 3, replayBuild, ""},
	{"AI-990.golden", "answer_off", 4, replayBuild, ""},
	{"AI-990.golden", "answer_on_not_backed_up", 4, replayNone, whyNoTransceiveSet},
	{"AI-990.golden", "answer_on_backed_up", 4, replayNone, whyNoTransceiveSet},

	// EX — Read 8 on both. The 890S Set/Answer is 8 + len(P5) + 1 under a
	// ruler whose terminator cell is headed "x", so a 3-digit P5 gives 12; the
	// 990S's is a FIXED 24 with P5 filling positions 9-23. That is erratum
	// E19, and G's 990S vectors are in the fixed form, which ParseEXAnswer
	// admits alongside the shorter one.
	{"EX-890.golden", "read", 8, replayBuild, ""},
	{"EX-890.golden", "set_normal", 12, replayNone, whyNoEXWrite},
	{"EX-890.golden", "answer_normal", 12, replayParse, ""},
	{"EX-890.golden", "set_initialize", 12, replayNone, whyNoEXWrite},
	{"EX-990.golden", "read", 8, replayBuild, ""},
	{"EX-990.golden", "set_normal", 24, replayNone, whyNoEXWrite},
	{"EX-990.golden", "answer_normal", 24, replayParse, ""},
	{"EX-990.golden", "set_initialize", 24, replayNone, whyNoEXWrite},

	// FV — Read 3, Answer 7 on both.
	{"FV-890.golden", "read", 3, replayBuild, ""},
	{"FV-890.golden", "answer_printed_example", 7, replayParse, ""},
	{"FV-990.golden", "read", 3, replayBuild, ""},
	{"FV-990.golden", "answer_printed_example", 7, replayParse, ""},

	// ID — Read 3, Answer 6 on both.
	{"ID-890.golden", "read", 3, replayBuild, ""},
	{"ID-890.golden", "answer", 6, replayParse, ""},
	{"ID-990.golden", "read", 3, replayBuild, ""},
	{"ID-990.golden", "answer", 6, replayParse, ""},

	// MA0 — Read 7 on both. The 890S Set/Answer is 40 + len(name) under the
	// floating "x" terminator, so the four answer lengths its own comments
	// record are 50, 40, 43 and 46; the 990S's is a FIXED 57 whatever the name
	// length. Those two rules are the whole of spec decision 10.
	{"MA0-890.golden", "read_ch000", 7, replayBuild, ""},
	{"MA0-890.golden", "read_ch119", 7, replayBuild, ""},
	{"MA0-890.golden", "answer_name_full10", 50, replayParse, ""},
	{"MA0-890.golden", "answer_name_empty", 40, replayParse, ""},
	{"MA0-890.golden", "answer_name_short3", 43, replayParse, ""},
	{"MA0-890.golden", "answer_tone_split", 46, replayParse, ""},
	{"MA0-890.golden", "set_name_full10", 50, replayBuild, ""},
	{"MA0-890.golden", "set_tone_split", 46, replayBuild, ""},
	{"MA0-990.golden", "read_ch000", 7, replayBuild, ""},
	{"MA0-990.golden", "read_ch119", 7, replayBuild, ""},
	{"MA0-990.golden", "answer_name_full10", 57, replayParse, ""},
	{"MA0-990.golden", "answer_name_empty", 57, replayParse, ""},
	{"MA0-990.golden", "answer_name_short3", 57, replayParse, ""},
	{"MA0-990.golden", "answer_tone_dual_split", 57, replayParse, ""},
	{"MA0-990.golden", "set_name_full10", 57, replayBuild, ""},
	{"MA0-990.golden", "set_tone_dual_split", 57, replayBuild, ""},

	// MN — Set 7, Read 4, Answer 7. The TS-890S book prints MN too; leg G
	// derived it for the 990S only, which is the row this book gives a
	// Main/Sub pointer.
	{"MN-990.golden", "set", 7, replayNone, whyMNOffTheRoster},
	{"MN-990.golden", "read", 4, replayNone, whyMNOffTheRoster},
	{"MN-990.golden", "answer", 7, replayNone, whyMNOffTheRoster},
}

// TestGoldenVectors_RosterAndCountedLengths pins the roster against the files
// and each frame against the length its own derivation counted.
func TestGoldenVectors_RosterAndCountedLengths(t *testing.T) {
	byFile := map[string][]goldenVector{}
	var order []string
	for _, v := range goldenRoster {
		if _, seen := byFile[v.File]; !seen {
			order = append(order, v.File)
		}
		byFile[v.File] = append(byFile[v.File], v)
	}

	present, err := filepath.Glob(filepath.Join(goldenDir, "*-89?.golden"))
	if err != nil {
		t.Fatalf("globbing %s: %v", goldenDir, err)
	}
	more, err := filepath.Glob(filepath.Join(goldenDir, "*-99?.golden"))
	if err != nil {
		t.Fatalf("globbing %s: %v", goldenDir, err)
	}
	present = append(present, more...)
	if len(present) != len(order) {
		t.Errorf("leg G holds %d vector files and the roster names %d — every file must be rostered", len(present), len(order))
	}

	for _, file := range order {
		got := loadVectors(t, filepath.Join(goldenDir, file))
		want := byFile[file]
		if len(got) != len(want) {
			t.Errorf("%s carries %d vectors and the roster names %d", file, len(got), len(want))
			continue
		}
		for i, v := range want {
			if got[i].Name != v.Name {
				t.Errorf("%s vector %d is %q, the roster names %q — the roster is the file's order, not a set", file, i, got[i].Name, v.Name)
				continue
			}
			if len(got[i].Frame) != v.CountedLen {
				t.Errorf("%s/%s is %d bytes; the deriver COUNTED %d off the printed ruler. This is a STOP for arbitration against the PDF, not a literal to adjust:\n  %q",
					file, v.Name, len(got[i].Frame), v.CountedLen, got[i].Frame)
			}
		}
	}
}

// TestGoldenVectors_EveryVectorIsOneFrameToTheSplitter replays all forty-nine
// through the envelope both books print, independently of whether any grammar
// here recognises them.
//
// It is the leg that still holds when a grammar is widened, which is when it
// matters most, and it is the only mechanical statement the replayNone
// vectors get: those frames are well-formed to this family's own framing even
// though nothing here builds or reads them.
func TestGoldenVectors_EveryVectorIsOneFrameToTheSplitter(t *testing.T) {
	for _, v := range goldenRoster {
		frame := goldenFrame(t, v)
		frames, rest := kw.SplitFrames([]byte(frame))
		if len(frames) != 1 || string(frames[0]) != frame || len(rest) != 0 {
			t.Errorf("%s/%s is not exactly one frame to the ';' splitter: %d frame(s), %d trailing bytes — an embedded terminator is two frames to the radio's own parser:\n  %q",
				v.File, v.Name, len(frames), len(rest), frame)
		}
	}
}

// TestGoldenVectors_TheUnbuiltFramesRecordWhyNot requires every replayNone
// entry to name the rule that keeps it unbuilt, and every replayed one to name
// none — so "we did not get to it" cannot pass as "the rules forbid it".
func TestGoldenVectors_TheUnbuiltFramesRecordWhyNot(t *testing.T) {
	none := 0
	for _, v := range goldenRoster {
		if v.How == replayNone {
			none++
			if v.Why == "" {
				t.Errorf("%s/%s has no entry point in this package and records no reason", v.File, v.Name)
			}
			continue
		}
		if v.Why != "" {
			t.Errorf("%s/%s is %s and still carries a no-builder reason", v.File, v.Name, v.How)
		}
	}
	// Stated out loud so that the set emptying — every frame suddenly
	// buildable — is a visible change rather than a quiet one.
	if want := 15; none != want {
		t.Errorf("%d vectors have no entry point here, want %d (four AI ON sets and their answers less answer_off's Set twin, four EX Set forms, three MN forms)", none, want)
	}
}

// TestGoldenVectors_EveryBuiltFrameIsBuiltByteForByte is the outbound half of
// the replay: for each replayBuild vector, this package's own builder produces
// exactly the bytes leg G counted.
//
// THE AI ANSWER FORM IS BUILT ON PURPOSE. "AI0;" is what both books print for
// the Set AND for the Answer (890:3-4 charts, 990's the same), so answer_off
// is byte-identical to the frame this package's init sequence sends; the pin
// says so rather than leaving the coincidence unremarked.
func TestGoldenVectors_EveryBuiltFrameIsBuiltByteForByte(t *testing.T) {
	for _, v := range goldenRoster {
		if v.How != replayBuild {
			continue
		}
		t.Run(v.File+"/"+v.Name, func(t *testing.T) {
			frame := goldenFrame(t, v)
			got, err := buildGoldenFrame(t, v)
			if err != nil {
				t.Fatalf("building %s/%s: %v", v.File, v.Name, err)
			}
			if v.File+"/"+v.Name == a14DummyByteVector {
				// Reproduced everywhere except the one byte this book calls a
				// dummy; the exception is pinned whole by
				// TestGoldenVectors_The990SSetVectorDiffersOnlyInA14sDummyByte
				// rather than waved through here.
				if string(got) == frame {
					t.Errorf("the A14 exception for %s is STALE: the built frame now matches the vector byte for byte, so the exception is removed rather than left standing", a14DummyByteVector)
				}
				return
			}
			if string(got) != frame {
				t.Errorf("BUILT BYTES DIFFER FROM THE COUNTED VECTOR — a STOP for arbitration against the PDF:\n  built    (%d bytes) %q\n  vector   (%d bytes) %q",
					len(got), got, len(frame), frame)
			}
		})
	}
}

// buildGoldenFrame produces one replayBuild vector through this package's own
// builder. The MA0 Set vectors are built from the record their byte-identical
// ANSWER parses to, which is the round trip the two grids' geometry rules
// actually have to satisfy.
func buildGoldenFrame(t *testing.T, v goldenVector) ([]byte, error) {
	t.Helper()
	l890, l990 := ma.Layout890(), ma.Layout990()
	menu000 := kw.EXAddress{P1: 0, P2: 0, P3: 0}

	var (
		cmd ma.Command
		err error
	)
	switch v.File + "/" + v.Name {
	case "AI-890.golden/read":
		cmd, err = l890.BuildAIRead()
	case "AI-990.golden/read":
		cmd, err = l990.BuildAIRead()
	case "AI-890.golden/set_off", "AI-890.golden/answer_off":
		cmd, err = l890.BuildAISetOff()
	case "AI-990.golden/set_off", "AI-990.golden/answer_off":
		cmd, err = l990.BuildAISetOff()
	case "ID-890.golden/read":
		cmd, err = l890.BuildIDRead()
	case "ID-990.golden/read":
		cmd, err = l990.BuildIDRead()
	case "FV-890.golden/read":
		cmd, err = l890.BuildFVRead()
	case "FV-990.golden/read":
		cmd, err = l990.BuildFVRead()
	case "EX-890.golden/read":
		cmd, err = l890.BuildEXRead(menu000)
	case "EX-990.golden/read":
		cmd, err = l990.BuildEXRead(menu000)
	case "MA0-890.golden/read_ch000":
		cmd, err = l890.BuildMA0Read(mustSlot(t, l890, 0))
	case "MA0-890.golden/read_ch119":
		cmd, err = l890.BuildMA0Read(mustSlot(t, l890, 119))
	case "MA0-990.golden/read_ch000":
		cmd, err = l990.BuildMA0Read(mustSlot(t, l990, 0))
	case "MA0-990.golden/read_ch119":
		cmd, err = l990.BuildMA0Read(mustSlot(t, l990, 119))
	case "MA0-890.golden/set_name_full10":
		cmd, err = rebuildMA0(t, l890, "MA0-890.golden", "answer_name_full10")
	case "MA0-890.golden/set_tone_split":
		cmd, err = rebuildMA0(t, l890, "MA0-890.golden", "answer_tone_split")
	case "MA0-990.golden/set_name_full10":
		cmd, err = rebuildMA0(t, l990, "MA0-990.golden", "answer_name_full10")
	case "MA0-990.golden/set_tone_dual_split":
		cmd, err = rebuildMA0(t, l990, "MA0-990.golden", "answer_tone_dual_split")
	default:
		t.Fatalf("%s/%s is rostered as built and this switch has no arm for it", v.File, v.Name)
	}
	if err != nil {
		return nil, err
	}
	return cmd.Bytes(), nil
}

// rebuildMA0 parses one MA0 answer vector and re-emits it as a Set.
func rebuildMA0(t *testing.T, l ma.Layout, file, answer string) (ma.Command, error) {
	t.Helper()
	rec, err := l.ParseMA0Answer([]byte(goldenFrameNamed(t, file, answer)))
	if err != nil {
		t.Fatalf("parsing %s/%s to rebuild its Set: %v", file, answer, err)
	}
	return l.BuildMA0Set(rec)
}

// a14DummyByteVector is the ONE replayBuild vector whose bytes this package
// does not reproduce exactly. See the test below for why, and for the pin.
const a14DummyByteVector = "MA0-990.golden/set_tone_dual_split"

// TestGoldenVectors_The990SSetVectorDiffersOnlyInA14sDummyByte is the one
// replayBuild vector whose bytes this package does NOT reproduce, isolated so
// that the exception is a pinned fact rather than a widened comparison.
//
// MA0-990's set_tone_dual_split carries P2 = '1' at position 7. That byte is
// the channel type, and this book says of it: "The memory channel type is
// decided while setting the P9 and P10 values, so this parameter is ignored.
// Enter a dummy value" (990:2901-2903). The deriver's own header records its
// choice as ASSUMED and says it "matches what a corresponding Answer would
// plausibly report"; this codec's choice is '0', which is A14 and the
// milestone's only defaulted byte. TWO DIFFERENT ASSUMED DUMMIES, in a
// position the book says is ignored — not a disagreement about the chart.
//
// The pin is the whole frame: EXACTLY ONE byte differs, and it is that one.
func TestGoldenVectors_The990SSetVectorDiffersOnlyInA14sDummyByte(t *testing.T) {
	const classPos = 6 // P2, position 7 (990:2897-2903)
	l990 := ma.Layout990()
	want := goldenFrameNamed(t, "MA0-990.golden", "set_tone_dual_split")

	cmd, err := rebuildMA0(t, l990, "MA0-990.golden", "answer_tone_dual_split")
	if err != nil {
		t.Fatalf("rebuilding the 990S split Set: %v", err)
	}
	got := string(cmd.Bytes())
	if len(got) != len(want) {
		t.Fatalf("the rebuilt frame is %d bytes and the vector is %d", len(got), len(want))
	}
	var differ []int
	for i := range want {
		if got[i] != want[i] {
			differ = append(differ, i)
		}
	}
	if len(differ) != 1 || differ[0] != classPos {
		t.Errorf("the rebuilt Set differs from the vector at byte offsets %v, want exactly [%d] — every other position must be reproduced byte for byte:\n  built  %q\n  vector %q",
			differ, classPos, got, want)
		return
	}
	if got[classPos] != '0' || want[classPos] != '1' {
		t.Errorf("P2 is %q in the built frame and %q in the vector; A14 records this codec's dummy as '0' and the deriver's as the answer's own value", got[classPos], want[classPos])
	}
}

// TestGoldenVectors_EveryAnswerParsesToItsPinnedFields is the inbound half:
// each replayParse vector decoded field by field, against values read off the
// vector file's own annotations.
func TestGoldenVectors_EveryAnswerParsesToItsPinnedFields(t *testing.T) {
	l890, l990 := ma.Layout890(), ma.Layout990()
	menu000 := kw.EXAddress{P1: 0, P2: 0, P3: 0}

	// The two envelope answers whose value is a string.
	for _, tc := range []struct {
		file, name string
		parse      func(frame []byte) (string, error)
		want       string
		why        string
	}{
		{"ID-890.golden", "answer", l890.ParseIDAnswer, l890.CATID(),
			"the ID legend prints exactly one model/number pair, \"024: TS-890S\" (890:2733), and that is this row's own CAT ID"},
		{"ID-990.golden", "answer", l990.ParseIDAnswer, l990.CATID(),
			"the 990S book prints its ID bare, \"022\" (990:2612); the binding of that token to a model is this project's, and it is this row's own CAT ID"},
		{"FV-890.golden", "answer_printed_example", l890.ParseFVAnswer, "1.00",
			"the books' only format statement is one worked example, \"FV1.00;\" (890:2657, 990:2533) — A13: the WIDTH is pinned and the grammar is not"},
		{"FV-990.golden", "answer_printed_example", l990.ParseFVAnswer, "1.00", "as the 890S; both books print the same example"},
	} {
		t.Run(tc.file+"/"+tc.name, func(t *testing.T) {
			got, err := tc.parse([]byte(goldenFrameNamed(t, tc.file, tc.name)))
			if err != nil {
				t.Fatalf("parsing: %v — %s", err, tc.why)
			}
			if got != tc.want {
				t.Errorf("parsed %q, want %q — %s", got, tc.want, tc.why)
			}
		})
	}

	// The two EX answers. BOTH ARE MENU 0/00/00, which each book's own chart
	// prints, so each is parsed against the inventory row that would have read
	// it. The 990S's is the FIXED 24-byte form of erratum E19: P5 fills its
	// fifteen-wide window and the pad is returned VERBATIM, because neither
	// book says which byte the pad is and trimming would be this codec making
	// the caller's assumption for it.
	for _, tc := range []struct {
		file, name string
		layout     ma.Layout
		want       string
	}{
		{"EX-890.golden", "answer_normal", l890, "000"},
		{"EX-990.golden", "answer_normal", l990, "000            "},
	} {
		t.Run(tc.file+"/"+tc.name, func(t *testing.T) {
			item, ok := tc.layout.EXItem(menu000)
			if !ok {
				t.Fatalf("menu %v is not in the %s's inventory", menu000, tc.layout.Model())
			}
			got, err := tc.layout.ParseEXAnswer([]byte(goldenFrameNamed(t, tc.file, tc.name)), item)
			if err != nil {
				t.Fatalf("parsing: %v", err)
			}
			if got != tc.want {
				t.Errorf("P5 parsed as %q (%d bytes), want %q (%d bytes)", got, len(got), tc.want, len(tc.want))
			}
			if item.Digits != 3 {
				t.Errorf("the inventory row for %v declares %d digits; both charts print menu 0/00/00 as an ordinary 3-digit row, which is what makes the 990S's 15-byte P5 the FIXED form rather than a wide value", menu000, item.Digits)
			}
		})
	}

	// The eight MA0 answers, decoded field by field. Every value below is read
	// off the vector file's own per-vector comment and position map.
	for _, tc := range []struct {
		file, name string
		layout     ma.Layout
		want       ma.Record
	}{{
		"MA0-890.golden", "answer_name_full10", l890,
		ma.Record{Slot: mustSlot(t, l890, 0), FreqHz: 14_250_000, Mode: '2', ToneType: '0', Name: "HOMEBASE10"},
	}, {
		// The name window is ZERO bytes wide: the terminator floats to x = 40.
		"MA0-890.golden", "answer_name_empty", l890,
		ma.Record{Slot: mustSlot(t, l890, 1), FreqHz: 7_074_000, Mode: '6', ToneType: '0', Name: ""},
	}, {
		"MA0-890.golden", "answer_name_short3", l890,
		ma.Record{Slot: mustSlot(t, l890, 119), FreqHz: 50_313_000, Mode: '4', FMNarrow: true, ToneType: '0', Lockout: true, Name: "SIX"},
	}, {
		// P5 = 1 Tone, P6 = 08 (88.5 Hz), P7 = 12 (100.0 Hz), P11 = 1 Split.
		"MA0-890.golden", "answer_tone_split", l890,
		ma.Record{Slot: mustSlot(t, l890, 100), FreqHz: 29_600_000, Mode: '4', ToneType: '1', ToneIndex: 8, CTCSSIndex: 12,
			TXFreqHz: 29_500_000, TXMode: '4', Split: true, Name: "REPEAT"},
	}, {
		// Class '0' is P2, Single Memory channel, and it is a PARSER OUTPUT.
		// P17 = '1' is Scan Lockout OFF on this radio — erratum E8.
		"MA0-990.golden", "answer_name_full10", l990,
		ma.Record{Slot: mustSlot(t, l990, 0), Class: '0', FreqHz: 14_250_000, Mode: '2', ToneType: '0', Name: "HOMEBASE10"},
	}, {
		// The name window is a FIXED ten bytes of ASCII space here, which is
		// A1's pad rule; the parser right-trims it to "".
		"MA0-990.golden", "answer_name_empty", l990,
		ma.Record{Slot: mustSlot(t, l990, 1), Class: '0', FreqHz: 7_074_000, Mode: '6', ToneType: '0', Name: ""},
	}, {
		"MA0-990.golden", "answer_name_short3", l990,
		ma.Record{Slot: mustSlot(t, l990, 119), Class: '0', FreqHz: 50_313_000, Mode: '4', FMNarrow: true, ToneType: '0', Lockout: true, Name: "ABC"},
	}, {
		// The whole second tuple this radio has and the 890S does not: P12 = 2
		// CTCSS for frequency 2, P14 = 21 (136.5 Hz), P16 = 1 dual reception.
		"MA0-990.golden", "answer_tone_dual_split", l990,
		ma.Record{Slot: mustSlot(t, l990, 100), Class: '1', FreqHz: 29_600_000, Mode: '4', ToneType: '1', ToneIndex: 8, CTCSSIndex: 12,
			TXFreqHz: 29_500_000, TXMode: '4', TXToneType: '2', TXCTCSSIndex: 21, Split: true, DualRecv: true, Lockout: true, Name: "REPEAT"},
	}} {
		t.Run(tc.file+"/"+tc.name, func(t *testing.T) {
			got, err := tc.layout.ParseMA0Answer([]byte(goldenFrameNamed(t, tc.file, tc.name)))
			if err != nil {
				t.Fatalf("parsing: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("PARSED RECORD DIFFERS FROM THE VECTOR'S OWN ANNOTATION — a STOP for arbitration against the PDF:\n  parsed %+v\n  want   %+v", got, tc.want)
			}
			if got.Empty {
				t.Error("the record is marked EMPTY; none of these vectors is a blank channel — leg G's headers record that the blank case is not represented at all")
			}
		})
	}
}

func mustSlot(t *testing.T, l ma.Layout, n int) ma.Slot {
	t.Helper()
	s, err := l.NewSlot(n)
	if err != nil {
		t.Fatalf("NewSlot(%d): %v", n, err)
	}
	return s
}

// vector is one parsed line of a leg G file.
type vector struct{ Name, Frame string }

// loadVectors reads a golden file's vectors: "name<TAB>frame", with '#' lines
// and blank lines skipped. A frame's bytes are taken VERBATIM, spaces and all.
func loadVectors(t *testing.T, path string) []vector {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var out []vector
	for i, line := range strings.Split(string(data), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, frame, ok := strings.Cut(line, "\t")
		if !ok {
			t.Fatalf("%s line %d is neither a comment nor a name<TAB>frame vector: %q", path, i+1, line)
		}
		out = append(out, vector{Name: name, Frame: frame})
	}
	if len(out) == 0 {
		t.Fatalf("%s carries no vectors", path)
	}
	return out
}

func goldenFrame(t *testing.T, v goldenVector) string {
	t.Helper()
	return goldenFrameNamed(t, v.File, v.Name)
}

func goldenFrameNamed(t *testing.T, file, name string) string {
	t.Helper()
	for _, v := range loadVectors(t, filepath.Join(goldenDir, file)) {
		if v.Name == name {
			return v.Frame
		}
	}
	t.Fatalf("%s carries no vector named %q", file, name)
	return ""
}
