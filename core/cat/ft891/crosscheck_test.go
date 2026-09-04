// SPDX-License-Identifier: GPL-3.0-or-later

package ft891_test

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat/ft891"
	"github.com/gm5dna/open-rig-programmer/internal/extable"
)

// This file is the FT-891 Stage 1 cross-check: it binds the three
// independently derived records of the CAT manual's MENU chart to one
// another, and binds the generated inventory to the one of them that
// generates it.
//
// The three artefacts, and why there are three:
//
//   - TRANSCRIPTION A — table2.csv, this package's ONLY generation source.
//     Layout-text-led, PDF-checked, in the ten-column extable shape, parsed
//     here through extable.ParseCSV under the REGISTERED "ft891" profile so
//     that the cross-check reads the same bytes through the same parser the
//     generator does.
//   - TRANSCRIPTION B — testdata/transcription-b.csv. Derived PDF-primary by
//     a quarantined agent that never opened this repository, never saw A or
//     the ledger, and was told no row count and no address.
//   - THE GROUP-BOUNDARY LEDGER — testdata/group-ledger.csv. Derived from
//     the rendered PDF's ruled cells by a second quarantined agent, BEFORE
//     either transcription existed, and the source of the profile's
//     ExpectedRows.
//
// Agreement between three blind derivations is the evidence; this test is
// where that agreement is made mechanical rather than asserted in prose. ANY
// mismatch is a STOP for orchestrator arbitration AGAINST THE PDF, which may
// correct A, B, or the ledger — never this test, and never an artefact
// edited merely to make the test pass. That is also why every failure below
// prints the offending MENU number or prefix and both sides' values: the
// failure output is the arbitration's input.
//
// # This chart's shape, and how it differs from the FTdx10 cross-check
//
// core/cat/ftdx10/crosscheck_test.go is this file's shape precedent, and
// three of its four adjudications simply do not arise here:
//
//   - THE ADDRESS IS A PAIR. The FT-891 chart prints a four-digit MENU
//     Number whose two halves are the whole address — 0803 is (P1,P2) =
//     (08,03) — and every row's P3 is 0 (extable's AddressPair, core/cat's
//     EXAddressPair). So the comparison key here is the pair, and B carries
//     the four digits verbatim rather than three columns.
//   - THERE ARE NO GROUP LABELS. The chart prints no label columns at all
//     (LabelsAbsent), so A's p1_label/p2_label are empty on every row and
//     there is nothing to normalise: the FTdx10's "NN (LABEL)" wrapper
//     adjudication has no counterpart. The ledger accordingly groups by the
//     bare two-digit PREFIX, not by a labelled subgroup.
//   - THERE IS NO TEXT ROW. TextRowsAbsent, TextWidth 0: every A row's text
//     flag is false and B carries no such column to reconstruct.
//
// What DOES carry over is that names are compared VERBATIM, byte for byte,
// with no normalisation whatever. Both extractions of this chart are pure
// ASCII (A's header records that its extraction transliterates nothing; B's
// companion transcription-b.md records the same for its 600 dpi reading), so
// a glyph-convention difference between the two extractors would be a real
// difference to arbitrate against the PDF and not a typographic accident to
// paper over.
//
// B's parameter-legend column does not exist — B was briefed to carry the
// MENU number, the name and the Digits only — so A's p4 column is
// single-sourced audit here as it is for the FTdx10, and is bound by
// nothing below.
//
// # Scope: what no leg here can catch
//
// All three legs read the same printed chart, so a defect PRINTED in the
// chart is transcribed faithfully by all three and is invisible to every
// comparison below. TestCrossCheck_A_B_Ledger/the_0905_printed_quirk pins
// the one such defect this chart is known to carry, so that the limit is
// recorded in the test rather than only in prose.

const (
	// transcriptionBPath and groupLedgerPath are relative to the package
	// directory, which is the working directory for `go test`.
	transcriptionBPath = "testdata/transcription-b.csv"
	groupLedgerPath    = "testdata/group-ledger.csv"

	// transcriptionBHeader and groupLedgerHeader are the two artefacts'
	// exact header rows, pinned so that a schema change fails LOUDLY here
	// rather than being silently misparsed into false agreement.
	transcriptionBHeader = "menu_number,name,digits"
	groupLedgerHeader    = "p1,first_menu_number,first_name,last_menu_number,last_name,row_count,pdf_page,visual_anchor"

	transcriptionBFields = 3
	groupLedgerFields    = 8
)

// The widest-field pin, fully hardcoded. The FT-891 chart's Digits column
// runs 1..5, and the 5 comes from exactly two rows: 0803 OTHER DISP and 0804
// OTHER SHIFT, whose signed "-3000 Hz - 0 - +3000 Hz" parameter counts its
// sign as a digit. These literals came from the chart and are written out
// here rather than derived from an artefact, because a pin computed from the
// thing it pins proves nothing. The test below then ALSO derives the same
// two addresses from transcription A and requires the derivation to agree
// with the literals, so a silent transcription change is caught from both
// directions at once.
const (
	widestRowAP1   = 8
	widestRowAP2   = 3
	widestRowAName = "OTHER DISP"
	widestRowBP1   = 8
	widestRowBP2   = 4
	widestRowBName = "OTHER SHIFT"
	// widestRowDigits is the profile's MaxDigits, spelt as a literal here so
	// that the "exactly two rows are this wide" leg does not derive its
	// threshold from the bound it is checking. The two are bound to each
	// other by the test.
	widestRowDigits = 5
)

// The printed-quirk pin. 0905 RPT SHIFT 50MHz prints Digits 1 against a
// legend ("0 - 4000 kHz (P2= 0000 - 4000) (10 kHz/step)") that needs four,
// where its twin 0904 RPT SHIFT 28MHz prints 4 for the same shape of legend.
// BOTH quarantined derivations recorded the printed 1 — testdata/
// transcription-b.md §"0905 RPT SHIFT 50MHz — Digits value contradicts its
// own legend" records three separate re-reads of that cell at 300 %
// enlargement of a 600 dpi render, and testdata/group-ledger.md records the
// same cell independently — and table2.csv's own quirks block records it as
// transcribed-as-printed. It is a defect of the MANUAL, and this repository
// has no FT-891 to ask which of 1 and 4 the radio answers.
//
// The pin exists precisely because the cross-check CANNOT catch this class:
// three faithful readings of one wrong printed cell agree perfectly. Pinning
// the value here means the quirk is a recorded, deliberate state rather than
// an unnoticed one, and that silently "correcting" 1 to 4 in either
// transcription fails a test instead of passing unremarked.
const (
	quirkRowP1     = 9
	quirkRowP2     = 5
	quirkRowName   = "RPT SHIFT 50MHz"
	quirkRowDigits = 1
	// quirkTwinP2 is 0904, the same-shaped row that prints 4. It is pinned
	// alongside so the quirk reads as the anomaly it is: if 0904 ever went
	// to 1 as well, the "one row contradicts its legend" claim would be
	// wrong and this test would say so.
	quirkTwinP2     = 4
	quirkTwinName   = "RPT SHIFT 28MHz"
	quirkTwinDigits = 4
)

// frozenEvidenceSHA256 is the freeze, transcribed from the commit message of
// adf3d21 ("ft891: the three quarantined evidence legs, committed verbatim"),
// whose "SHA-256 at commit:" block records one hash per artefact.
//
// The two .md companions are in here with the two CSVs because they are not
// commentary about them: they are the derivation records this file cites for
// the quirk it pins and for the glyph conventions it declines to normalise,
// and an artefact whose stated method had been quietly rewritten would be as
// corrupt as one whose rows had. The remaining artefacts frozen by adf3d21 —
// the four *.golden frame vectors and their provenance.md, evidence leg G —
// belong to the frame-geometry task and are pinned by its own test; this map
// covers the legs THIS file reads.
var frozenEvidenceSHA256 = map[string]string{
	"transcription-b.csv": "5bb41932712c8c11237ce9e1a44b782d3c133f54170cb2147a9bc6b23cf54bb5",
	"transcription-b.md":  "39c06f9208476d59cf48a96bf085503eb4f59a4e3a6ab7cd08ab2c4d03245758",
	"group-ledger.csv":    "75525b01ba65464c5770b11f7a55ae3b5350cb83a861a9ade6733fbb9a5c2716",
	"group-ledger.md":     "7ad2902175a5ffabf25ada0571252555da9c66bc317883251f52f88eae0296db",
}

// TestQuarantinedEvidenceFrozen recomputes each quarantined artefact's
// SHA-256 and compares it with the value commit adf3d21 recorded, so that the
// freeze is self-enforcing in CI rather than a fact recoverable only by git
// archaeology. Every leg of the cross-check below reads bytes this test has
// vouched for.
//
// The second half is the one that catches the interesting case: both files
// this test PARSES must be covered by the map above, so a leg could not be
// re-pointed at an unfrozen artefact and still look bound.
func TestQuarantinedEvidenceFrozen(t *testing.T) {
	for name, want := range frozenEvidenceSHA256 {
		path := filepath.Join("testdata", name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading quarantined artefact %s: %v", path, err)
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != want {
			t.Errorf("FREEZE BROKEN — %s has changed since commit adf3d21.\n"+
				"  recorded SHA-256 %s\n"+
				"  present  SHA-256 %s\n"+
				"This artefact is quarantined evidence: it is never regenerated and\n"+
				"never edited to satisfy a test. Restore it from the repository root\n"+
				"with `git checkout adf3d21 -- core/cat/ft891/testdata/%s` and report\n"+
				"the change.",
				path, want, got, name)
		}
	}
	for _, path := range []string{transcriptionBPath, groupLedgerPath} {
		if _, ok := frozenEvidenceSHA256[filepath.Base(path)]; !ok {
			t.Errorf("%s is read by this cross-check but has no recorded SHA-256: every artefact a leg parses must be frozen by a commit that records its hash", path)
		}
	}
}

// menuAddr is one FT-891 EX address: the two halves of the chart's printed
// four-digit MENU Number. P3 does not appear because this radio's address
// has no third component — see the file comment.
type menuAddr struct{ P1, P2 int }

// String renders the address the way the CHART prints it, four digits, so
// that a failure message can be grepped for straight in the manual.
func (m menuAddr) String() string { return fmt.Sprintf("%02d%02d", m.P1, m.P2) }

// chartRow is the tuple both transcriptions carry for an address. A's labels
// and text flag are absent by this chart's shape and are checked separately,
// against the profile's policies, rather than compared between the legs.
type chartRow struct {
	Name   string
	Digits int
}

func (r chartRow) String() string { return fmt.Sprintf("name=%q digits=%d", r.Name, r.Digits) }

// ledgerRow is one ledger row's bindable content: the group's first and last
// MENU numbers with their names, and its row count. pdf_page and
// visual_anchor are the ledger's own provenance and describe the printed
// page rather than the inventory, so they are read (the column count is
// pinned) but not bound.
type ledgerRow struct {
	First     menuAddr
	FirstName string
	Last      menuAddr
	LastName  string
	RowCount  int
}

// TestCrossCheck_A_B_Ledger binds transcription A, transcription B and the
// group-boundary ledger to one another on every leg the milestone specifies.
func TestCrossCheck_A_B_Ledger(t *testing.T) {
	// The REGISTERED profile, not a literal of this file's own: the whole
	// point is to read A exactly as the generator reads it, under the same
	// digit bounds and the same address policy. Lookup by name is sufficient
	// here because this test binds the ARTEFACTS rather than the ownership of
	// the generated file (which is what makes staleness_test.go select by
	// Package instead).
	p, ok := extable.Lookup("ft891")
	if !ok {
		t.Fatal(`extable.Lookup("ft891"): the profile is not registered`)
	}

	aRows := loadTranscriptionA(t, p)
	a := namesAndDigits(aRows)
	b := loadTranscriptionB(t)
	ledger := loadGroupLedger(t)

	t.Run("A_and_B_agree_row_for_row", func(t *testing.T) {
		// Both directions, separately, so a failure says WHICH artefact is
		// missing the MENU number rather than merely that the sets differ.
		for _, m := range sortedAddrs(a) {
			if _, in := b[m]; !in {
				t.Errorf("MENU number %s is in transcription A (%s) but NOT in transcription B (%s): A has %s", m, p.ManualCSV, transcriptionBPath, a[m])
			}
		}
		for _, m := range sortedAddrs(b) {
			if _, in := a[m]; !in {
				t.Errorf("MENU number %s is in transcription B (%s) but NOT in transcription A (%s): B has %s", m, transcriptionBPath, p.ManualCSV, b[m])
			}
		}
		// The tuple, field by field. Names are compared as BYTES with no
		// normalisation of any kind: see the file comment on why a glyph
		// difference here is arbitration material, not something to fold
		// away.
		for _, m := range sortedAddrs(a) {
			bv, in := b[m]
			if !in {
				continue // already reported above
			}
			av := a[m]
			if av.Name != bv.Name {
				t.Errorf("MENU number %s: the NAME differs between the transcriptions:\n  A (%s): %q\n  B (%s): %q",
					m, p.ManualCSV, av.Name, transcriptionBPath, bv.Name)
			}
			if av.Digits != bv.Digits {
				t.Errorf("MENU number %s (%q): the DIGITS differ between the transcriptions:\n  A (%s): %d\n  B (%s): %d",
					m, av.Name, p.ManualCSV, av.Digits, transcriptionBPath, bv.Digits)
			}
		}
	})

	t.Run("A_against_the_ledger", func(t *testing.T) {
		checkAgainstLedger(t, "transcription A", p.ManualCSV, a, ledger)
	})

	t.Run("B_against_the_ledger", func(t *testing.T) {
		checkAgainstLedger(t, "transcription B", transcriptionBPath, b, ledger)
	})

	t.Run("totals", func(t *testing.T) {
		sum := 0
		for _, p1 := range sortedPrefixes(ledger) {
			sum += ledger[p1].RowCount
		}
		// One four-way equality, reported whole: which of the four moved is
		// the first question arbitration asks. The value itself (159) is
		// pinned on the profile by internal/extable/profile_test.go's
		// registration test, so it is deliberately not re-spelt here — this
		// leg binds the artefacts to that pinned constant rather than keeping
		// a second copy of it.
		if len(a) != len(b) || len(a) != sum || len(a) != p.ExpectedRows {
			t.Errorf("row totals disagree: transcription A = %d, transcription B = %d, ledger row_count sum = %d, profile ExpectedRows = %d",
				len(a), len(b), sum, p.ExpectedRows)
		}
	})

	t.Run("no_labels_and_no_text_rows", func(t *testing.T) {
		// The chart prints no label columns and no free-text item, which is
		// what LabelsAbsent and TextRowsAbsent say. ParseCSV already refuses
		// a non-blank label and a text row under those policies, so this leg
		// is not the first line of defence; it is here because the
		// inventory-vs-A leg below asserts the SAME three facts of the
		// generated items, and a claim about the generated file is only worth
		// making if the source it was generated from is checked for it too.
		for _, m := range sortedAddrs(a) {
			r := aRows[m]
			if r.P1Label != "" || r.P2Label != "" {
				t.Errorf("MENU number %s (%q): transcription A (%s) carries labels p1_label=%q p2_label=%q, but this chart prints no label columns (LabelsAbsent)", m, r.Name, p.ManualCSV, r.P1Label, r.P2Label)
			}
			if r.Text {
				t.Errorf("MENU number %s (%q): transcription A (%s) marks a text row, but this chart prints no free-text item (TextRowsAbsent)", m, r.Name, p.ManualCSV)
			}
			if r.P3 != 0 {
				t.Errorf("MENU number %s (%q): transcription A (%s) carries p3 %d, but this radio's EX address is a PAIR and every p3 must be 0 (AddressPair)", m, r.Name, p.ManualCSV, r.P3)
			}
		}
		// B's digits are bounded from the same place A's are — the registered
		// profile — rather than from a second copy of 1..5 written here.
		for _, m := range sortedAddrs(b) {
			if d := b[m].Digits; d < p.MinDigits || d > p.MaxDigits {
				t.Errorf("MENU number %s (%q): transcription B (%s) carries digits %d, outside the profile's %d..%d", m, b[m].Name, transcriptionBPath, d, p.MinDigits, p.MaxDigits)
			}
		}
	})

	t.Run("the_0905_printed_quirk", func(t *testing.T) {
		// See the quirk pin's comment: this is the one defect no leg above
		// can catch, because all three derivations read the same wrong
		// printed cell and agree.
		quirk := menuAddr{quirkRowP1, quirkRowP2}
		twin := menuAddr{quirkRowP1, quirkTwinP2}
		for _, src := range []struct {
			label string
			path  string
			rows  map[menuAddr]chartRow
		}{
			{"transcription A", p.ManualCSV, a},
			{"transcription B", transcriptionBPath, b},
		} {
			for _, want := range []struct {
				addr menuAddr
				row  chartRow
			}{
				{quirk, chartRow{quirkRowName, quirkRowDigits}},
				{twin, chartRow{quirkTwinName, quirkTwinDigits}},
			} {
				got, in := src.rows[want.addr]
				if !in {
					t.Errorf("%s (%s) has no MENU number %s, which the printed-quirk pin requires", src.label, src.path, want.addr)
					continue
				}
				if got != want.row {
					t.Errorf("%s (%s): MENU number %s is %s, the recorded quirk pins %s (testdata/transcription-b.md and testdata/group-ledger.md both record this cell as printed; a change here is arbitration against the PDF, not an edit)",
						src.label, src.path, want.addr, got, want.row)
				}
			}
		}
	})
}

// TestCrossCheck_InventoryAgainstTranscriptionA binds the GENERATED
// inventory to transcription A, the file it is generated from.
//
// staleness_test.go already re-renders the generated file from the CSV and
// byte-compares it, which catches any drift between the two. This test binds
// something else: what the inventory MEANS once loaded — that the rows are
// the chart's rows, in address order, carrying this chart's shape (P3 zero,
// no labels, no text) and this chart's widths. It reaches the inventory
// through ft891.Dialect().EXItems() — the package's only access to it now
// that the dialect exists, and the route every other consumer uses.
func TestCrossCheck_InventoryAgainstTranscriptionA(t *testing.T) {
	p, ok := extable.Lookup("ft891")
	if !ok {
		t.Fatal(`extable.Lookup("ft891"): the profile is not registered`)
	}
	aRows := loadTranscriptionA(t, p)
	a := namesAndDigits(aRows)
	order := sortedAddrs(a)

	items := ft891.Dialect().EXItems()
	if len(items) != len(a) {
		t.Fatalf("the generated inventory holds %d items, transcription A (%s) holds %d rows", len(items), p.ManualCSV, len(a))
	}

	// The items must be in ascending (P1,P2) order in their own right, not
	// merely be a permutation that happens to match A once sorted: the
	// generated file's doc comment claims the sort, so the claim is pinned
	// here rather than assumed.
	for i := 1; i < len(items); i++ {
		prev, cur := items[i-1].Addr, items[i].Addr
		if prev.P1 > cur.P1 || (prev.P1 == cur.P1 && prev.P2 >= cur.P2) {
			t.Errorf("the generated inventory is not in ascending (P1,P2) order at index %d: %s follows %s", i, cur, prev)
		}
	}

	for i, want := range order {
		got := items[i]
		gotAddr := menuAddr{int(got.Addr.P1), int(got.Addr.P2)}
		if gotAddr != want {
			t.Errorf("inventory item %d is MENU number %s, transcription A (%s) has %s there", i, gotAddr, p.ManualCSV, want)
			continue
		}
		row := a[want]
		if got.Name != row.Name {
			t.Errorf("MENU number %s: the inventory's Name is %q, transcription A (%s) has %q", want, got.Name, p.ManualCSV, row.Name)
		}
		if got.Digits != row.Digits {
			t.Errorf("MENU number %s (%q): the inventory's Digits is %d, transcription A (%s) has %d", want, row.Name, got.Digits, p.ManualCSV, row.Digits)
		}
		if got.Addr.P3 != 0 {
			t.Errorf("MENU number %s (%q): the inventory's P3 is %d, but this radio's EX address is a PAIR and every P3 must be 0 (AddressPair)", want, row.Name, got.Addr.P3)
		}
		if got.P1Label != "" || got.P2Label != "" {
			t.Errorf("MENU number %s (%q): the inventory carries labels P1Label=%q P2Label=%q, but this chart prints no label columns (LabelsAbsent)", want, row.Name, got.P1Label, got.P2Label)
		}
		if got.Text {
			t.Errorf("MENU number %s (%q): the inventory marks a text item, but this chart prints no free-text item (TextRowsAbsent)", want, row.Name)
		}
	}

	t.Run("the_two_widest_rows", func(t *testing.T) {
		// Two independent statements of the same fact, deliberately: the
		// literal addresses below came from the chart, and the derivation
		// walks the inventory for whatever is widest. A silent change to the
		// transcription moves the derivation away from the literals; a
		// mistaken literal disagrees with the derivation. Neither can be
		// satisfied by editing the other.
		widest := 0
		var at []menuAddr
		for _, it := range items {
			m := menuAddr{int(it.Addr.P1), int(it.Addr.P2)}
			switch {
			case it.Digits > widest:
				widest, at = it.Digits, []menuAddr{m}
			case it.Digits == widest:
				at = append(at, m)
			}
		}
		if widest != widestRowDigits {
			t.Errorf("the widest Digits in the generated inventory is %d, the chart pin says %d", widest, widestRowDigits)
		}
		// The profile's MaxDigits is the BOUND the parser enforces; the
		// widest row OBSERVED is what the chart actually prints. They are
		// the same number for this chart, and binding them here is what
		// stops MaxDigits drifting into a bound nothing reaches.
		if widest != p.MaxDigits {
			t.Errorf("the widest Digits observed is %d, the registered profile's MaxDigits is %d", widest, p.MaxDigits)
		}
		wantAt := []menuAddr{{widestRowAP1, widestRowAP2}, {widestRowBP1, widestRowBP2}}
		if len(at) != len(wantAt) {
			t.Fatalf("%d inventory items carry the widest Digits %d (%v), the chart pin says exactly 2 (%v)", len(at), widest, at, wantAt)
		}
		for i, want := range wantAt {
			if at[i] != want {
				t.Errorf("the widest inventory items are %v, the chart pin says %v", at, wantAt)
				break
			}
		}
		// …and the same two rows, read out of transcription A by name, so
		// that the literal pin is checked against the CSV as well as against
		// the generated file.
		for _, want := range []struct {
			addr menuAddr
			name string
		}{
			{menuAddr{widestRowAP1, widestRowAP2}, widestRowAName},
			{menuAddr{widestRowBP1, widestRowBP2}, widestRowBName},
		} {
			row, in := a[want.addr]
			if !in {
				t.Errorf("transcription A (%s) has no MENU number %s, which the widest-row pin requires", p.ManualCSV, want.addr)
				continue
			}
			if row.Name != want.name || row.Digits != widestRowDigits {
				t.Errorf("MENU number %s: transcription A (%s) has %s, the widest-row pin says name=%q digits=%d", want.addr, p.ManualCSV, row, want.name, widestRowDigits)
			}
		}
	})
}

// checkAgainstLedger binds one transcription's two-digit-prefix grouping to
// the ledger: prefix sets both directions, per-prefix row_count, and
// per-prefix first/last (MENU number, name).
func checkAgainstLedger(t *testing.T, srcLabel, srcPath string, rows map[menuAddr]chartRow, ledger map[int]ledgerRow) {
	t.Helper()

	byPrefix := map[int][]menuAddr{}
	for _, m := range sortedAddrs(rows) {
		byPrefix[m.P1] = append(byPrefix[m.P1], m)
	}

	prefixes := sortedGroupKeys(byPrefix)
	for _, p1 := range prefixes {
		if _, in := ledger[p1]; !in {
			t.Errorf("prefix %02d is in %s (%s) but NOT in the ledger (%s): %d rows, first %s", p1, srcLabel, srcPath, groupLedgerPath, len(byPrefix[p1]), byPrefix[p1][0])
		}
	}
	for _, p1 := range sortedPrefixes(ledger) {
		if _, in := byPrefix[p1]; !in {
			l := ledger[p1]
			t.Errorf("prefix %02d is in the ledger (%s) but NOT in %s (%s): the ledger says %d rows, %q..%q", p1, groupLedgerPath, srcLabel, srcPath, l.RowCount, l.FirstName, l.LastName)
		}
	}

	for _, p1 := range prefixes {
		l, in := ledger[p1]
		if !in {
			continue // already reported above
		}
		// sortedAddrs already ordered these by (P1,P2), so the prefix's slice
		// is ascending in P2. First and last are taken from that order rather
		// than from file order, so neither artefact's row ordering can change
		// what "first" and "last" mean.
		addrs := byPrefix[p1]
		first, last := addrs[0], addrs[len(addrs)-1]

		if len(addrs) != l.RowCount {
			t.Errorf("prefix %02d: %s (%s) holds %d rows, the ledger (%s) records row_count %d", p1, srcLabel, srcPath, len(addrs), groupLedgerPath, l.RowCount)
		}
		if first != l.First || rows[first].Name != l.FirstName {
			t.Errorf("prefix %02d: %s (%s) opens at %s %q, the ledger (%s) records %s %q", p1, srcLabel, srcPath, first, rows[first].Name, groupLedgerPath, l.First, l.FirstName)
		}
		if last != l.Last || rows[last].Name != l.LastName {
			t.Errorf("prefix %02d: %s (%s) closes at %s %q, the ledger (%s) records %s %q", p1, srcLabel, srcPath, last, rows[last].Name, groupLedgerPath, l.Last, l.LastName)
		}
	}
}

// aRow is transcription A's row as this file needs it: the comparison tuple
// plus the three fields this chart's shape policies make claims about.
type aRow struct {
	Name    string
	Digits  int
	P1Label string
	P2Label string
	Text    bool
	P3      int
}

// loadTranscriptionA reads table2.csv through extable.ParseCSV under the
// registered profile — the same parser, the same bounds, the same
// duplicate-address refusal and the same AddressPair rule the generator runs.
func loadTranscriptionA(t *testing.T, p extable.Profile) map[menuAddr]aRow {
	t.Helper()
	data, err := os.ReadFile(p.ManualCSV)
	if err != nil {
		t.Fatalf("reading transcription A (%s): %v", p.ManualCSV, err)
	}
	rows, err := extable.ParseCSV(p, data)
	if err != nil {
		t.Fatalf("ParseCSV(%s): %v", p.ManualCSV, err)
	}
	// ParseCSV itself refuses a duplicate (P1,P2,P3), and every P3 here is 0
	// under AddressPair, so this pair-keyed map cannot silently absorb one.
	out := make(map[menuAddr]aRow, len(rows))
	for _, r := range rows {
		out[menuAddr{r.P1, r.P2}] = aRow{
			Name:    r.Name,
			Digits:  r.Digits,
			P1Label: r.P1Label,
			P2Label: r.P2Label,
			Text:    r.Text,
			P3:      r.P3,
		}
	}
	if len(out) != len(rows) {
		t.Fatalf("transcription A (%s) holds %d rows but only %d distinct (p1,p2) pairs", p.ManualCSV, len(rows), len(out))
	}
	return out
}

// namesAndDigits projects A onto the tuple the comparisons use.
func namesAndDigits(rows map[menuAddr]aRow) map[menuAddr]chartRow {
	out := make(map[menuAddr]chartRow, len(rows))
	for m, r := range rows {
		out[m] = chartRow{Name: r.Name, Digits: r.Digits}
	}
	return out
}

// loadTranscriptionB reads B with encoding/csv. Its menu_number is the
// chart's four printed digits verbatim, split here into the two halves that
// are this radio's address; its name is taken with no normalisation of any
// kind.
func loadTranscriptionB(t *testing.T) map[menuAddr]chartRow {
	t.Helper()
	records := readEvidenceCSV(t, transcriptionBPath, transcriptionBHeader, transcriptionBFields)

	out := make(map[menuAddr]chartRow, len(records))
	for i, rec := range records {
		where := fmt.Sprintf("%s data row %d", transcriptionBPath, i+1)
		m := splitMenuNumber(t, where+" menu_number", rec[0])
		if prev, dup := out[m]; dup {
			t.Fatalf("%s: duplicate MENU number %s (already held %s)", where, m, prev)
		}
		out[m] = chartRow{
			Name:   rec[1],
			Digits: atoiOrFatal(t, where, "digits", rec[2]),
		}
	}
	return out
}

// loadGroupLedger reads the ledger with encoding/csv, one row per two-digit
// prefix.
func loadGroupLedger(t *testing.T) map[int]ledgerRow {
	t.Helper()
	records := readEvidenceCSV(t, groupLedgerPath, groupLedgerHeader, groupLedgerFields)

	out := make(map[int]ledgerRow, len(records))
	for i, rec := range records {
		where := fmt.Sprintf("%s data row %d", groupLedgerPath, i+1)
		p1 := atoiOrFatal(t, where, "p1", rec[0])
		first := splitMenuNumber(t, where+" first_menu_number", rec[1])
		last := splitMenuNumber(t, where+" last_menu_number", rec[3])
		// The ledger records each group's prefix three times — once as a
		// column and once inside each of its two MENU numbers — so the three
		// are bound to each other here. Nothing else in the cross-check would
		// notice them disagreeing.
		if first.P1 != p1 {
			t.Fatalf("%s: the p1 column is %02d but first_menu_number %q carries prefix %02d", where, p1, rec[1], first.P1)
		}
		if last.P1 != p1 {
			t.Fatalf("%s: the p1 column is %02d but last_menu_number %q carries prefix %02d", where, p1, rec[3], last.P1)
		}
		if _, dup := out[p1]; dup {
			t.Fatalf("%s: duplicate prefix %02d", where, p1)
		}
		out[p1] = ledgerRow{
			First:     first,
			FirstName: rec[2],
			Last:      last,
			LastName:  rec[4],
			RowCount:  atoiOrFatal(t, where, "row_count", rec[5]),
		}
	}
	return out
}

// readEvidenceCSV reads path as CSV, requires its first record to be exactly
// wantHeader, and returns the data records.
//
// Comment = '#' covers the repository's CSV convention: neither quarantined
// artefact carries comment lines today (both put their prose in a .md
// companion), but table2.csv is '#'-commented and a provenance block added
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

// splitMenuNumber decodes the chart's printed four-digit MENU Number into
// the (P1,P2) pair that is this radio's whole EX address — 0803 into (08,03).
//
// It is deliberately exact: four bytes, every one a decimal digit. A cell
// that is merely similar is a Fatal, not a silent partial parse, because a
// permissive reading is precisely how two genuinely different addresses
// would be folded into false agreement. strconv.Atoi is not given the whole
// cell for the same reason — it would accept a sign or spaces.
func splitMenuNumber(t *testing.T, where, raw string) menuAddr {
	t.Helper()
	const want = 4
	if len(raw) != want {
		t.Fatalf("%s: %q is not a four-digit MENU number (%d bytes)", where, raw, len(raw))
	}
	for i := 0; i < want; i++ {
		if raw[i] < '0' || raw[i] > '9' {
			t.Fatalf("%s: %q is not a four-digit MENU number (byte %d is %q)", where, raw, i, raw[i:i+1])
		}
	}
	return menuAddr{
		P1: atoiOrFatal(t, where, "prefix", raw[:2]),
		P2: atoiOrFatal(t, where, "suffix", raw[2:]),
	}
}

func atoiOrFatal(t *testing.T, where, field, raw string) int {
	t.Helper()
	n, err := strconv.Atoi(raw)
	if err != nil {
		t.Fatalf("%s: bad %s %q: %v", where, field, raw, err)
	}
	return n
}

// sortedAddrs returns m's MENU numbers in (P1,P2) order, so that every
// failure list is stable and every prefix's slice is ascending in P2.
func sortedAddrs(m map[menuAddr]chartRow) []menuAddr {
	out := make([]menuAddr, 0, len(m))
	for a := range m {
		out = append(out, a)
	}
	sortAddrs(out)
	return out
}

func sortedGroupKeys(m map[int][]menuAddr) []int {
	out := make([]int, 0, len(m))
	for p1 := range m {
		out = append(out, p1)
	}
	sort.Ints(out)
	return out
}

func sortedPrefixes(m map[int]ledgerRow) []int {
	out := make([]int, 0, len(m))
	for p1 := range m {
		out = append(out, p1)
	}
	sort.Ints(out)
	return out
}

func sortAddrs(as []menuAddr) {
	sort.Slice(as, func(i, j int) bool {
		if as[i].P1 != as[j].P1 {
			return as[i].P1 < as[j].P1
		}
		return as[i].P2 < as[j].P2
	})
}
