// SPDX-License-Identifier: GPL-3.0-or-later

package ft991a_test

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

	"github.com/gm5dna/open-rig-programmer/internal/extable"
)

// This file is the FT-991A Stage 1 cross-check: it binds the three
// independently derived records of the CAT manual's MENU chart to one
// another.
//
// The three artefacts, and why there are three:
//
//   - TRANSCRIPTION A — table2.csv, this package's ONLY generation source.
//     Layout-text-led, PDF-checked, in the ten-column extable shape, parsed
//     here through extable.ParseCSV under the REGISTERED "ft991a" profile so
//     that the cross-check reads the same bytes through the same parser the
//     generator does.
//   - TRANSCRIPTION B — testdata/transcription-b.csv. Derived PDF-primary by
//     a quarantined agent that never opened this repository, never saw A or
//     the ledger, and was told no row count and no address.
//   - THE PAGE LEDGER — testdata/ledger.csv. Derived from the rendered PDF's
//     ruled cells by a second quarantined agent, BEFORE either transcription
//     existed, and the source of the profile's ExpectedRows.
//
// Agreement between three blind derivations is the evidence; this test is
// where that agreement is made mechanical rather than asserted in prose. ANY
// mismatch is a STOP for orchestrator arbitration AGAINST THE PDF, which may
// correct A, B, or the ledger — never this test, and never an artefact
// edited merely to make the test pass. That is also why every failure below
// prints the offending MENU number and both sides' values: the failure
// output is the arbitration's input.
//
// # This chart's shape, and how it differs from the FT-891 cross-check
//
// core/cat/ft891/crosscheck_test.go is this file's shape precedent, and this
// chart differs from that one in three ways that change the comparison:
//
//   - THE ADDRESS IS ONE COMPONENT. The chart prints a three-digit MENU
//     Number, 001 to 153, which is the whole EX address — extable's
//     AddressSingle, core/cat's EXAddressSingle — so every row's p2 and p3
//     are 0 and the comparison key here is a single integer rather than the
//     FT-891's (P1,P2) pair.
//   - THE LEDGER IS PER PDF PAGE, NOT PER GROUP. This chart is FLAT: it
//     prints no group labels (LabelsAbsent) and its numbers carry no prefix,
//     so there is no prefix to group by. The ledger's own visual anchors say
//     so, and its p1 column is therefore the PDF page (8, 9, 10) that the
//     three ruled runs 001-042, 043-124 and 125-153 fall on. The ledger leg
//     below accordingly binds CONTIGUOUS RANGES rather than prefix sets.
//   - THE DIGITS CELL IS NOT ALWAYS AN INTEGER. One row, 087 RADIO ID,
//     prints a hyphen where every other row prints a width, so the tuple
//     compared below carries the Digits cell as a TOKEN and bounds it
//     against the profile only where it is numeric. See the normalisations.
//
// There is no text row here (TextRowsAbsent, TextWidth 0) and no label
// column, exactly as on the FT-891, so neither of those adjudications
// arises.
//
// # The TWO declared normalisations, and nothing else
//
// Names are otherwise compared VERBATIM, byte for byte. The plan
// (docs/superpowers/plans/2026-09-05-ft991a-registration.md, task 6)
// pre-declared exactly two normalisations, BEFORE this comparison was ever
// run, and both are applied HERE, at comparison time, and never by editing
// a file:
//
//	(i)  layout 655's "00 :" spaced colon on rows 119/125/128/134; and
//	(ii) row 087's Digits cell, where A writes "-" and B writes "?" (P18).
//
// (ii) fires, on exactly one address; (i) cannot fire at all, and the leg
// TestCrossCheck_A_B_Ledger/the_two_declared_normalisations proves both
// statements rather than leaving either as prose — see that leg's comment
// for why (i) is inert.
//
// # Scope: what no leg here can catch
//
// All three legs read the same printed chart, so a defect PRINTED in the
// chart is transcribed faithfully by all three and is invisible to every
// comparison below. TestCrossCheck_A_B_Ledger/the_printed_quirks_no_leg_can_catch
// pins the two this chart is known to carry in a COMPARED column, so that
// the limit is recorded in the test rather than only in prose.
//
// The generated inventory is not bound here, as it is on the FT-891, for one
// reason: this package has no dialect yet, exItems is unexported, and
// staleness_test.go already re-renders the generated file from table2.csv
// and byte-compares it. The semantic binding the FT-891 reaches through
// ft891.Dialect().EXItems() belongs with the dialect that provides it.

const (
	// transcriptionBPath and pageLedgerPath are relative to the package
	// directory, which is the working directory for `go test`.
	transcriptionBPath = "testdata/transcription-b.csv"
	pageLedgerPath     = "testdata/ledger.csv"

	// transcriptionBHeader and pageLedgerHeader are the two artefacts'
	// exact header rows, pinned so that a schema change fails LOUDLY here
	// rather than being silently misparsed into false agreement.
	transcriptionBHeader = "menu_number,name,digits"
	pageLedgerHeader     = "p1,first_menu_number,first_name,last_menu_number,last_name,row_count,pdf_page,visual_anchor"

	transcriptionBFields = 3
	pageLedgerFields     = 8
)

// The two spellings of the parameterless Digits cell, one per leg (P18).
//
// A carries the chart's own glyph, the single hyphen, which is what
// extable's ParameterlessExcluded policy keys on; B carries "?", which is
// what B's brief told it to write for a cell that is not an integer, and its
// own record (testdata/transcription-b.md §5) says in as many words that the
// cell "is legibly a hyphen". So the two legs AGREE on the glyph and differ
// only in transcription convention. Both spellings are kept: editing either
// quarantined artefact is a STOP.
const (
	parameterlessDigitsA = "-"
	parameterlessDigitsB = "?"
)

// The spaced-colon pin. Four rows open their option list "00 : OFF" where
// their two siblings of the same shape open "00: OFF" — a printed
// inconsistency BOTH quarantined derivations noticed independently
// (testdata/ledger.md defect 11 and testdata/transcription-b.md §4.9 name
// the same six rows) and which table2.csv's header records as a
// NORMALISATION RULE: the spaced colon is transcribed verbatim and never
// respaced.
//
// The addresses are written out here from the chart rather than derived from
// an artefact, because a pin computed from the thing it pins proves nothing;
// the leg below then derives the same two sets out of transcription A and
// requires the derivation to agree with these literals, so neither a silent
// transcription change nor a mistaken literal can pass alone.
var (
	spacedColonRows = []menuAddr{119, 125, 128, 134}
	tightColonRows  = []menuAddr{122, 131}
)

const (
	spacedColonOpening = "00 : OFF"
	tightColonOpening  = "00: OFF"
)

// The widest-row pin, fully hardcoded. This chart's Digits column runs 1..8,
// and the 8 comes from exactly one row: 151 PRESET FREQUENCY, whose
// parameter is the eight-digit frequency range "00030000 ~ 47000000". The
// four rows below it at 5, and the fact that these five are the ONLY rows
// wider than 4, are the L ledger's own table ("Every row whose Digits value
// is greater than 4", testdata/ledger.md) restated as literals so that
// leg L's prose is bound mechanically here rather than merely filed.
var wideRows = []struct {
	addr   menuAddr
	name   string
	digits int
}{
	{27, "TIME ZONE", 5},
	{64, "OTHER DISP (SSB)", 5},
	{65, "OTHER SHIFT (SSB)", 5},
	{83, "RPT SHIFT 430MHz", 5},
	{151, "PRESET FREQUENCY", 8},
}

// widestRowDigits is the profile's MaxDigits, spelt as a literal here so
// that the "exactly one row is this wide" leg does not derive its threshold
// from the bound it is checking. The two are bound to each other by the
// test.
const widestRowDigits = 8

// The printed-quirk pins. Two of this chart's printed defects fall in a
// column BOTH transcriptions carry, so both derivations read the same wrong
// cell and agree perfectly — which is exactly the class no leg of this
// cross-check can catch:
//
//   - 068 DATA HCUT FREQ and 069 DATA HCUT SLOPE PRINT EACH OTHER'S DIGITS.
//     068's parameter needs two digits and prints 1; 069's needs one and
//     prints 2. Every other HCUT FREQ/SLOPE pair in the chart prints 2 then
//     1, which is why the four of them are pinned alongside: if a sibling
//     ever went the same way, the "one pair is transposed" claim would be
//     wrong and this test would say so.
//   - 088 GM DISPLY is printed without the A of DISPLAY. B's record
//     (testdata/transcription-b.md, Reconciliation) is a three-look
//     settlement of exactly this cell, and A transcribed the same spelling.
//
// Both are defects of the MANUAL, and this repository has no FT-991A to ask
// which reading the radio answers. Pinning them means they are a recorded,
// deliberate state rather than an unnoticed one, and that silently
// "correcting" either in a transcription fails a test instead of passing
// unremarked.
var transposedDigitsPair = []struct {
	addr   menuAddr
	name   string
	digits int
}{
	{68, "DATA HCUT FREQ", 1},
	{69, "DATA HCUT SLOPE", 2},
}

var untransposedHCUTPairs = []struct {
	freqAddr, slopeAddr     menuAddr
	freqName, slopeName     string
	freqDigits, slopeDigits int
}{
	{43, 44, "AM HCUT FREQ", "AM HCUT SLOPE", 2, 1},
	{52, 53, "CW HCUT FREQ", "CW HCUT SLOPE", 2, 1},
	{94, 95, "RTTY HCUT FREQ", "RTTY HCUT SLOPE", 2, 1},
	{104, 105, "SSB HCUT FREQ", "SSB HCUT SLOPE", 2, 1},
}

const (
	misspeltRowAddr   menuAddr = 88
	misspeltRowName            = "GM DISPLY"
	misspeltRowDigits          = 1
)

// frozenEvidenceSHA256 is the freeze, transcribed from the commit message of
// 8f2bad4 ("core/cat/ft991a: import the quarantined evidence legs — L
// ledger, B transcription, G geometry"), whose "SHA-256 of every imported
// file:" block records one hash per artefact.
//
// The two .md companions are in here with the two CSVs because they are not
// commentary about them: they are the derivation records this file cites for
// the quirks it pins, for the wide rows it binds and for the spaced colon it
// declines to normalise, and an artefact whose stated method had been
// quietly rewritten would be as corrupt as one whose rows had. The remaining
// artefacts 8f2bad4 froze — the five *.golden frame vectors and their
// provenance.md, evidence leg G — belong to the frame-geometry task and are
// pinned by its own test, as core/cat/ft891/golden_test.go pins that model's;
// this map covers the legs THIS file reads.
var frozenEvidenceSHA256 = map[string]string{
	"transcription-b.csv": "676002b6a751bbe3c4ea93d97d7ee6f2dd1fd0fc439d912e3195d02f8499313f",
	"transcription-b.md":  "c1acb704da7430e734e086019916709e048b71310c9d7459b8aa86dede82eae1",
	"ledger.csv":          "58eecbe327df50b2fcf61b9daa723d182e4f624b99b058f2775a237057b8af41",
	"ledger.md":           "f034a5b13d3aaaa7747061e25223b261cc7892df84922f3d621f17509c51712f",
}

// TestQuarantinedEvidenceFrozen recomputes each quarantined artefact's
// SHA-256 and compares it with the value commit 8f2bad4 recorded, so that
// the freeze is self-enforcing in CI rather than a fact recoverable only by
// git archaeology. Every leg of the cross-check below reads bytes this test
// has vouched for.
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
			t.Errorf("FREEZE BROKEN — %s has changed since commit 8f2bad4.\n"+
				"  recorded SHA-256 %s\n"+
				"  present  SHA-256 %s\n"+
				"This artefact is quarantined evidence: it is never regenerated and\n"+
				"never edited to satisfy a test. Restore it from the repository root\n"+
				"with `git checkout 8f2bad4 -- core/cat/ft991a/testdata/%s` and report\n"+
				"the change.",
				path, want, got, name)
		}
	}
	for _, path := range []string{transcriptionBPath, pageLedgerPath} {
		if _, ok := frozenEvidenceSHA256[filepath.Base(path)]; !ok {
			t.Errorf("%s is read by this cross-check but has no recorded SHA-256: every artefact a leg parses must be frozen by a commit that records its hash", path)
		}
	}
}

// menuAddr is one FT-991A EX address: the chart's printed MENU Number, which
// is the WHOLE address. There is no P2 and no P3 because this radio's
// address has no second or third component — see the file comment.
type menuAddr int

// String renders the address the way the CHART prints it, three digits, so
// that a failure message can be grepped for straight in the manual.
func (m menuAddr) String() string { return fmt.Sprintf("%03d", int(m)) }

// chartRow is the tuple both transcriptions carry for an address.
//
// Digits is a TOKEN and not an int: one row of this chart prints a hyphen
// there rather than a width, the two legs spell that cell differently by
// their own conventions (P18), and an int field would force this file to
// decide what the hyphen means before the comparison had run. The token is
// what each leg wrote; digitsAgree decides what agreement means.
type chartRow struct {
	Name   string
	Digits string
}

func (r chartRow) String() string { return fmt.Sprintf("name=%q digits=%q", r.Name, r.Digits) }

// ledgerRow is one ledger row's bindable content: the run's first and last
// MENU numbers with their names, and its row count. visual_anchor is the
// ledger's own provenance and describes the printed page rather than the
// inventory, so it is read (the column count is pinned) but not bound.
//
// P1 is the PDF PAGE, not an address prefix. This chart is flat; see the
// file comment.
type ledgerRow struct {
	PDFPage   int
	First     menuAddr
	FirstName string
	Last      menuAddr
	LastName  string
	RowCount  int
}

// TestCrossCheck_A_B_Ledger binds transcription A, transcription B and the
// page ledger to one another on every leg the milestone specifies.
func TestCrossCheck_A_B_Ledger(t *testing.T) {
	// The REGISTERED profile, not a literal of this file's own: the whole
	// point is to read A exactly as the generator reads it, under the same
	// digit bounds, the same address policy and the same declared
	// parameterless address. Lookup by name is sufficient here because this
	// test binds the ARTEFACTS rather than the ownership of the generated
	// file (which is what makes staleness_test.go select by Lookup name for
	// a different reason, stated there).
	p, ok := extable.Lookup("ft991a")
	if !ok {
		t.Fatal(`extable.Lookup("ft991a"): the profile is not registered`)
	}

	aRows := loadTranscriptionA(t, p)
	a := namesAndDigits(aRows)
	b := loadTranscriptionB(t)
	ledger := loadPageLedger(t)
	parameterless := declaredParameterless(t, p)

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
		// The tuple, field by field. Names are compared as BYTES: see the
		// file comment on why a glyph difference here is arbitration
		// material, not something to fold away.
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
			if ok, _ := digitsAgree(parameterless, m, av.Digits, bv.Digits); !ok {
				t.Errorf("MENU number %s (%q): the DIGITS differ between the transcriptions:\n  A (%s): %q\n  B (%s): %q",
					m, av.Name, p.ManualCSV, av.Digits, transcriptionBPath, bv.Digits)
			}
		}
	})

	t.Run("the_two_declared_normalisations", func(t *testing.T) {
		checkDeclaredNormalisations(t, p, aRows, a, b, parameterless)
	})

	t.Run("A_against_the_ledger", func(t *testing.T) {
		checkAgainstLedger(t, "transcription A", p.ManualCSV, a, ledger)
	})

	t.Run("B_against_the_ledger", func(t *testing.T) {
		checkAgainstLedger(t, "transcription B", transcriptionBPath, b, ledger)
	})

	t.Run("totals", func(t *testing.T) {
		sum := 0
		for _, page := range sortedPages(ledger) {
			sum += ledger[page].RowCount
		}
		// One four-way equality, reported whole: which of the four moved is
		// the first question arbitration asks. The value itself (153) is
		// pinned on the profile by internal/extable/profile_test.go's
		// registration test, so it is deliberately not re-spelt here — this
		// leg binds the artefacts to that pinned constant rather than
		// keeping a second copy of it.
		if len(a) != len(b) || len(a) != sum || len(a) != p.ExpectedRows {
			t.Errorf("row totals disagree: transcription A = %d, transcription B = %d, ledger row_count sum = %d, profile ExpectedRows = %d",
				len(a), len(b), sum, p.ExpectedRows)
		}
	})

	t.Run("the_chart_shape_both_legs_must_carry", func(t *testing.T) {
		// The chart prints no label columns, no free-text item and a
		// one-component address, which is what LabelsAbsent, TextRowsAbsent
		// and AddressSingle say. ParseCSV already refuses a non-blank label
		// and a text row under those policies, so this leg is not the first
		// line of defence; it is here because the whole milestone rests on
		// the address being P1 alone, and a claim that load-bearing is worth
		// stating of the source as well as of the parser.
		for _, m := range sortedAddrs(a) {
			r := aRows[m]
			if r.P1Label != "" || r.P2Label != "" {
				t.Errorf("MENU number %s (%q): transcription A (%s) carries labels p1_label=%q p2_label=%q, but this chart prints no label columns (LabelsAbsent)", m, r.Name, p.ManualCSV, r.P1Label, r.P2Label)
			}
			if r.Text {
				t.Errorf("MENU number %s (%q): transcription A (%s) marks a text row, but this chart prints no free-text item (TextRowsAbsent)", m, r.Name, p.ManualCSV)
			}
			if r.P2 != 0 || r.P3 != 0 {
				t.Errorf("MENU number %s (%q): transcription A (%s) carries p2 %d p3 %d, but this radio's EX address is a SINGLE component and both must be 0 (AddressSingle)", m, r.Name, p.ManualCSV, r.P2, r.P3)
			}
		}
		// Both legs' numeric Digits cells are bounded from the same place —
		// the registered profile — rather than from a second copy of 1..8
		// written here. The parameterless cell is not a width and is
		// excluded from the bound by address, which is the same licence
		// ParseCSV grants A.
		for _, src := range []struct {
			label string
			path  string
			rows  map[menuAddr]chartRow
		}{
			{"transcription A", p.ManualCSV, a},
			{"transcription B", transcriptionBPath, b},
		} {
			for _, m := range sortedAddrs(src.rows) {
				tok := src.rows[m].Digits
				if parameterless[m] {
					continue
				}
				d, err := strconv.Atoi(tok)
				if err != nil {
					t.Errorf("MENU number %s (%q): %s (%s) carries a Digits cell %q that is not a width, and %s is not the declared parameterless address", m, src.rows[m].Name, src.label, src.path, tok, m)
					continue
				}
				if d < p.MinDigits || d > p.MaxDigits {
					t.Errorf("MENU number %s (%q): %s (%s) carries digits %d, outside the profile's %d..%d", m, src.rows[m].Name, src.label, src.path, d, p.MinDigits, p.MaxDigits)
				}
			}
		}
	})

	t.Run("the_widest_rows", func(t *testing.T) {
		// Two independent statements of the same fact, deliberately: the
		// literals in wideRows came from the chart and from the L ledger's
		// own table, and the derivation walks transcription A for whatever
		// is wider than 4. A silent change to the transcription moves the
		// derivation away from the literals; a mistaken literal disagrees
		// with the derivation. Neither can be satisfied by editing the
		// other.
		var over4 []menuAddr
		widest, widestAt := 0, []menuAddr(nil)
		for _, m := range sortedAddrs(a) {
			if parameterless[m] {
				continue
			}
			d, err := strconv.Atoi(a[m].Digits)
			if err != nil {
				continue // already reported by the shape leg
			}
			if d > 4 {
				over4 = append(over4, m)
			}
			switch {
			case d > widest:
				widest, widestAt = d, []menuAddr{m}
			case d == widest:
				widestAt = append(widestAt, m)
			}
		}
		if widest != widestRowDigits {
			t.Errorf("the widest Digits in transcription A (%s) is %d, the chart pin says %d", p.ManualCSV, widest, widestRowDigits)
		}
		// The profile's MaxDigits is the BOUND the parser enforces; the
		// widest row OBSERVED is what the chart actually prints. They are
		// the same number for this chart, and binding them here is what
		// stops MaxDigits drifting into a bound nothing reaches.
		if widest != p.MaxDigits {
			t.Errorf("the widest Digits observed is %d, the registered profile's MaxDigits is %d", widest, p.MaxDigits)
		}
		if len(widestAt) != 1 || widestAt[0] != wideRows[len(wideRows)-1].addr {
			t.Errorf("the rows carrying the widest Digits %d are %v, the chart pin says exactly one, %s", widest, widestAt, wideRows[len(wideRows)-1].addr)
		}
		var wantOver4 []menuAddr
		for _, w := range wideRows {
			wantOver4 = append(wantOver4, w.addr)
		}
		if !sameAddrs(over4, wantOver4) {
			t.Errorf("the rows wider than 4 Digits in transcription A (%s) are %v, the pin (testdata/ledger.md's own table) says %v", p.ManualCSV, over4, wantOver4)
		}
		// …and each of the five read out of BOTH transcriptions by name and
		// width, so the literals are checked against the two legs and not
		// only against A's derivation.
		for _, w := range wideRows {
			requireRow(t, p, a, b, w.addr, w.name, strconv.Itoa(w.digits), "the wide-row pin")
		}
	})

	t.Run("the_printed_quirks_no_leg_can_catch", func(t *testing.T) {
		// See the quirk pins' comment: these are the defects no leg above
		// can catch, because both derivations read the same wrong printed
		// cell and agree.
		for _, q := range transposedDigitsPair {
			requireRow(t, p, a, b, q.addr, q.name, strconv.Itoa(q.digits), "the transposed-Digits pin")
		}
		for _, s := range untransposedHCUTPairs {
			requireRow(t, p, a, b, s.freqAddr, s.freqName, strconv.Itoa(s.freqDigits), "the untransposed-sibling pin")
			requireRow(t, p, a, b, s.slopeAddr, s.slopeName, strconv.Itoa(s.slopeDigits), "the untransposed-sibling pin")
		}
		requireRow(t, p, a, b, misspeltRowAddr, misspeltRowName, strconv.Itoa(misspeltRowDigits), "the printed-misspelling pin")
	})
}

// requireRow asserts that BOTH transcriptions carry addr with exactly the
// given name and Digits token. Every pin in this file goes through it, so a
// pin can never be satisfied by one leg alone.
func requireRow(t *testing.T, p extable.Profile, a, b map[menuAddr]chartRow, addr menuAddr, name, digits, pin string) {
	t.Helper()
	want := chartRow{Name: name, Digits: digits}
	for _, src := range []struct {
		label string
		path  string
		rows  map[menuAddr]chartRow
	}{
		{"transcription A", p.ManualCSV, a},
		{"transcription B", transcriptionBPath, b},
	} {
		got, in := src.rows[addr]
		if !in {
			t.Errorf("%s (%s) has no MENU number %s, which %s requires", src.label, src.path, addr, pin)
			continue
		}
		if got != want {
			t.Errorf("%s (%s): MENU number %s is %s, %s says %s (both quarantined records transcribe this cell as printed; a change here is arbitration against the PDF, not an edit)",
				src.label, src.path, addr, got, pin, want)
		}
	}
}

// The two pre-declared normalisations, named so that a result can say WHICH
// one fired on WHICH row rather than reporting a bare zero.
const (
	normParameterlessDigits = `the 087 Digits normalisation: A "-" is B "?" on the declared parameterless address (P18)`
	normSpacedColon         = `the spaced-colon normalisation: "00 : OFF" is never respaced to "00: OFF"`
)

// digitsAgree reports whether A's and B's Digits tokens for addr are the
// same reading of the same printed cell, and names the pre-declared
// normalisation that had to fire for them to be. fired is empty when the two
// tokens are already byte-equal.
//
// The ONE normalisation this comparison applies is (ii): on an address the
// profile's ParameterlessAddresses names, and in the Digits column alone,
// A's "-" and B's "?" are one cell. Both legs read a hyphen off the page —
// testdata/transcription-b.md §5 says so of B in as many words — and differ
// only in what their own briefs told them to write for a cell that is not an
// integer. Anywhere else, and in any other column, either spelling is a
// mismatch: the licence is per address, exactly as extable's
// ParameterlessExcluded grants it to ParseCSV.
//
// It is applied HERE and never by editing a file. Both artefacts are
// quarantined evidence, and TestQuarantinedEvidenceFrozen holds them to the
// bytes commit 8f2bad4 recorded.
func digitsAgree(parameterless map[menuAddr]bool, addr menuAddr, aTok, bTok string) (ok bool, fired string) {
	if aTok == bTok {
		return true, ""
	}
	if parameterless[addr] && aTok == parameterlessDigitsA && bTok == parameterlessDigitsB {
		return true, normParameterlessDigits
	}
	return false, ""
}

// checkDeclaredNormalisations is the leg that stops a bare zero standing in
// for an unexamined one: it names which of the two pre-declared
// normalisations fired, and on which rows.
//
// (ii), the 087 Digits normalisation, must fire on EXACTLY the addresses the
// profile declares parameterless and on no others — asserted in both
// directions, so neither a normalisation that quietly stopped being needed
// nor one that started firing somewhere new can pass unremarked.
//
// (i), the spaced colon, fires on NOTHING, and that is a finding rather than
// an omission. The spaced colon is printed in the chart's PARAMETER column,
// which transcription B does not carry — B was briefed to record the MENU
// number, the name and the Digits only — so the cell it concerns is
// single-sourced audit here, as A's p4 is on the FT-891. What this leg can
// still do, and does, is pin the printed inconsistency in A against the
// literals both quarantined records independently describe
// (testdata/ledger.md defect 11, testdata/transcription-b.md §4.9), and
// prove that no COMPARED cell in either leg carries a spaced colon, so the
// normalisation cannot have been silently needed and skipped.
func checkDeclaredNormalisations(t *testing.T, p extable.Profile, aRows map[menuAddr]aRow, a, b map[menuAddr]chartRow, parameterless map[menuAddr]bool) {
	t.Helper()

	t.Run("087_digits_A_hyphen_is_B_question_mark", func(t *testing.T) {
		var firedOn []menuAddr
		for _, m := range sortedAddrs(a) {
			bv, in := b[m]
			if !in {
				continue // reported by the row-for-row leg
			}
			if _, fired := digitsAgree(parameterless, m, a[m].Digits, bv.Digits); fired != "" {
				firedOn = append(firedOn, m)
				t.Logf("NORMALISATION FIRED on MENU number %s (%q): %s — A (%s) %q, B (%s) %q",
					m, a[m].Name, fired, p.ManualCSV, a[m].Digits, transcriptionBPath, bv.Digits)
			}
		}
		var want []menuAddr
		for m := range parameterless {
			want = append(want, m)
		}
		sort.Slice(want, func(i, j int) bool { return want[i] < want[j] })
		if !sameAddrs(firedOn, want) {
			t.Errorf("the 087 Digits normalisation fired on %v; the profile declares it over exactly %v (Profile.ParameterlessAddresses). It is declared per ADDRESS: firing anywhere else, or not firing where it was declared, is a STOP for arbitration and never an edit to either artefact", firedOn, want)
		}
		// Each leg's own spelling, stated separately, so that a leg quietly
		// adopting the other's convention is caught even though the
		// normalisation would still make the two agree.
		for _, m := range want {
			if got := a[m].Digits; got != parameterlessDigitsA {
				t.Errorf("MENU number %s: transcription A (%s) spells the parameterless Digits cell %q, P18 says A writes %q (the chart's own glyph, which ParameterlessExcluded keys on)", m, p.ManualCSV, got, parameterlessDigitsA)
			}
			if got := b[m].Digits; got != parameterlessDigitsB {
				t.Errorf("MENU number %s: transcription B (%s) spells the parameterless Digits cell %q, P18 says B writes %q (its brief's rule for a cell that is not an integer)", m, transcriptionBPath, got, parameterlessDigitsB)
			}
			// A's parse and the profile's declaration are two statements of
			// one fact and must agree: ParseCSV sets Parameterless only on a
			// declared address, so a row flagged here that the profile does
			// not name would mean the parser and this leg disagree about
			// which row the licence covers.
			if !aRows[m].Parameterless {
				t.Errorf("MENU number %s: the profile declares it parameterless but ParseCSV did not flag transcription A's row (%s)", m, p.ManualCSV)
			}
		}
		for m, r := range aRows {
			if r.Parameterless && !parameterless[m] {
				t.Errorf("MENU number %s: ParseCSV flagged transcription A's row (%s) parameterless, but the profile does not name that address", m, p.ManualCSV)
			}
		}
	})

	t.Run("the_spaced_colon_fires_on_nothing_compared", func(t *testing.T) {
		// The two derived sets, out of A's parameter column, against the
		// literals: the four rows that open "00 : OFF" and the two of the
		// same option-list shape that open "00: OFF".
		var spaced []menuAddr
		for _, m := range sortedAddrs(a) {
			if strings.HasPrefix(aRows[m].P4, spacedColonOpening) {
				spaced = append(spaced, m)
			}
		}
		if !sameAddrs(spaced, spacedColonRows) {
			t.Errorf("transcription A (%s) opens %v with %q; the pin (and testdata/ledger.md defect 11) says exactly %v", p.ManualCSV, spaced, spacedColonOpening, spacedColonRows)
		}
		for _, m := range tightColonRows {
			r, in := aRows[m]
			if !in {
				t.Errorf("transcription A (%s) has no MENU number %s, which the spaced-colon pin requires as a tight-colon sibling", p.ManualCSV, m)
				continue
			}
			if !strings.HasPrefix(r.P4, tightColonOpening) || strings.HasPrefix(r.P4, spacedColonOpening) {
				t.Errorf("MENU number %s (%q): transcription A (%s) opens its parameter %q, the pin says %q — the four/two split is what makes the spaced colon a printed inconsistency rather than this chart's house style", m, r.Name, p.ManualCSV, r.P4, tightColonOpening)
			}
		}
		// …and the finding: the normalisation fires on nothing, because
		// neither compared column can carry a spaced colon. Proved rather
		// than asserted, over both legs and both compared fields.
		for _, src := range []struct {
			label string
			path  string
			rows  map[menuAddr]chartRow
		}{
			{"transcription A", p.ManualCSV, a},
			{"transcription B", transcriptionBPath, b},
		} {
			for _, m := range sortedAddrs(src.rows) {
				r := src.rows[m]
				for _, f := range []struct{ field, val string }{{"name", r.Name}, {"digits", r.Digits}} {
					if strings.Contains(f.val, " :") {
						t.Errorf("MENU number %s: %s (%s) carries a spaced colon in its %s (%q). %s is declared over the chart's PARAMETER column, which only transcription A carries; a spaced colon in a COMPARED cell is outside the declaration and is arbitration material, not something this leg may fold away",
							m, src.label, src.path, f.field, f.val, normSpacedColon)
					}
				}
			}
		}
		t.Logf("%s fired on 0 compared rows: it is declared over the chart's parameter column, which transcription B does not carry, so %v are pinned in transcription A alone", normSpacedColon, spacedColonRows)
	})
}

// checkAgainstLedger binds one transcription to the ledger's three ruled
// runs: that the runs partition the chart contiguously and exhaustively,
// that each run's row_count is the number of transcribed rows inside it, and
// that each run's first and last (MENU number, name) are that transcription's.
//
// This is where this file departs furthest from the FT-891 precedent, which
// groups by the two-digit address prefix. There is no prefix here: the chart
// is FLAT, and the ledger's rows are the three PDF PAGES the chart occupies.
func checkAgainstLedger(t *testing.T, srcLabel, srcPath string, rows map[menuAddr]chartRow, ledger map[int]ledgerRow) {
	t.Helper()

	addrs := sortedAddrs(rows)
	pages := sortedPages(ledger)

	// The runs, in page order, must tile 001..153 with no gap, no overlap
	// and nothing outside them. Walking the transcription's own sorted
	// addresses against the runs does all three at once, and says which
	// address broke it.
	next := 0
	for _, page := range pages {
		l := ledger[page]
		if l.First > l.Last {
			t.Errorf("ledger (%s) PDF page %d runs %s..%s, which is backwards", pageLedgerPath, page, l.First, l.Last)
			continue
		}
		var inRun []menuAddr
		for next < len(addrs) && addrs[next] <= l.Last {
			m := addrs[next]
			if m < l.First {
				t.Errorf("MENU number %s is in %s (%s) but falls in no ledger run (%s): the run for PDF page %d begins at %s", m, srcLabel, srcPath, pageLedgerPath, page, l.First)
			} else {
				inRun = append(inRun, m)
			}
			next++
		}
		if len(inRun) != l.RowCount {
			t.Errorf("PDF page %d: %s (%s) holds %d rows in %s..%s, the ledger (%s) records row_count %d", page, srcLabel, srcPath, len(inRun), l.First, l.Last, pageLedgerPath, l.RowCount)
		}
		// The ledger's row_count and its own printed endpoints are two
		// separate readings of one ruled run — pass 4's rule count and pass
		// 1's numbers, in testdata/ledger.md's terms — so they are bound to
		// each other here. Nothing else in this file would notice them
		// disagreeing.
		if span := int(l.Last-l.First) + 1; span != l.RowCount {
			t.Errorf("ledger (%s) PDF page %d records row_count %d but its endpoints %s..%s span %d rows", pageLedgerPath, page, l.RowCount, l.First, l.Last, span)
		}
		if len(inRun) == 0 {
			continue
		}
		first, last := inRun[0], inRun[len(inRun)-1]
		if first != l.First || rows[first].Name != l.FirstName {
			t.Errorf("PDF page %d: %s (%s) opens at %s %q, the ledger (%s) records %s %q", page, srcLabel, srcPath, first, rows[first].Name, pageLedgerPath, l.First, l.FirstName)
		}
		if last != l.Last || rows[last].Name != l.LastName {
			t.Errorf("PDF page %d: %s (%s) closes at %s %q, the ledger (%s) records %s %q", page, srcLabel, srcPath, last, rows[last].Name, pageLedgerPath, l.Last, l.LastName)
		}
	}
	for ; next < len(addrs); next++ {
		t.Errorf("MENU number %s is in %s (%s) but beyond every ledger run (%s), the last of which ends at %s", addrs[next], srcLabel, srcPath, pageLedgerPath, ledger[pages[len(pages)-1]].Last)
	}
}

// aRow is transcription A's row as this file needs it: the comparison tuple
// plus the fields this chart's shape policies make claims about, plus the
// parameter column, which B does not carry and which the spaced-colon leg
// reads.
type aRow struct {
	Name          string
	Digits        string
	P4            string
	P1Label       string
	P2Label       string
	Text          bool
	P2            int
	P3            int
	Parameterless bool
}

// loadTranscriptionA reads table2.csv through extable.ParseCSV under the
// registered profile — the same parser, the same bounds, the same
// duplicate-address refusal, the same AddressSingle rule and the same
// per-address parameterless licence the generator runs.
//
// The Digits TOKEN is reconstructed from the parse rather than re-read out
// of the file: ParseCSV leaves Digits at its zero value and sets
// Parameterless for the hyphen cell (internal/extable/extable.go's Row doc
// comment), so "-" here is A's cell as the GENERATOR understood it, which is
// the reading this cross-check is about.
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
	// ParseCSV itself refuses a duplicate (P1,P2,P3), and every P2 and P3
	// here is 0 under AddressSingle, so this P1-keyed map cannot silently
	// absorb one.
	out := make(map[menuAddr]aRow, len(rows))
	for _, r := range rows {
		digits := strconv.Itoa(r.Digits)
		if r.Parameterless {
			digits = parameterlessDigitsA
		}
		out[menuAddr(r.P1)] = aRow{
			Name:          r.Name,
			Digits:        digits,
			P4:            r.P4,
			P1Label:       r.P1Label,
			P2Label:       r.P2Label,
			Text:          r.Text,
			P2:            r.P2,
			P3:            r.P3,
			Parameterless: r.Parameterless,
		}
	}
	if len(out) != len(rows) {
		t.Fatalf("transcription A (%s) holds %d rows but only %d distinct MENU numbers", p.ManualCSV, len(rows), len(out))
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
// chart's three printed digits verbatim; its name and its Digits cell are
// taken with no normalisation of any kind, the Digits cell as a token
// because one of them is not a number.
func loadTranscriptionB(t *testing.T) map[menuAddr]chartRow {
	t.Helper()
	records := readEvidenceCSV(t, transcriptionBPath, transcriptionBHeader, transcriptionBFields)

	out := make(map[menuAddr]chartRow, len(records))
	for i, rec := range records {
		where := fmt.Sprintf("%s data row %d", transcriptionBPath, i+1)
		m := parseMenuNumber(t, where+" menu_number", rec[0])
		if prev, dup := out[m]; dup {
			t.Fatalf("%s: duplicate MENU number %s (already held %s)", where, m, prev)
		}
		out[m] = chartRow{Name: rec[1], Digits: rec[2]}
	}
	return out
}

// loadPageLedger reads the ledger with encoding/csv, one row per PDF page
// the chart occupies.
func loadPageLedger(t *testing.T) map[int]ledgerRow {
	t.Helper()
	records := readEvidenceCSV(t, pageLedgerPath, pageLedgerHeader, pageLedgerFields)

	out := make(map[int]ledgerRow, len(records))
	for i, rec := range records {
		where := fmt.Sprintf("%s data row %d", pageLedgerPath, i+1)
		page := atoiOrFatal(t, where, "p1", rec[0])
		// The ledger records its page number twice — once as the p1 column
		// and once at the head of the pdf_page cell, which spells it
		// "8 (folio 7)" — so the two are bound to each other here. Nothing
		// else in the cross-check would notice them disagreeing.
		anchor, _, _ := strings.Cut(rec[6], " ")
		if got := atoiOrFatal(t, where, "pdf_page", anchor); got != page {
			t.Fatalf("%s: the p1 column is %d but pdf_page %q names page %d", where, page, rec[6], got)
		}
		if _, dup := out[page]; dup {
			t.Fatalf("%s: duplicate PDF page %d", where, page)
		}
		out[page] = ledgerRow{
			PDFPage:   page,
			First:     parseMenuNumber(t, where+" first_menu_number", rec[1]),
			FirstName: rec[2],
			Last:      parseMenuNumber(t, where+" last_menu_number", rec[3]),
			LastName:  rec[4],
			RowCount:  atoiOrFatal(t, where, "row_count", rec[5]),
		}
	}
	return out
}

// declaredParameterless projects the profile's ParameterlessAddresses onto
// this file's single-component address, refusing any member that is not
// shaped like one. The set is read from the PROFILE and never spelt as a
// literal here: the plan's normalisation is declared over "the address in
// Profile.ParameterlessAddresses", so a second copy of 087 in this file
// would be a bound consulted from somewhere other than its datum.
func declaredParameterless(t *testing.T, p extable.Profile) map[menuAddr]bool {
	t.Helper()
	if p.ParameterlessPolicy != extable.ParameterlessExcluded {
		t.Fatalf("the ft991a profile declares %v; this cross-check's 087 normalisation is declared over ParameterlessExcluded's address set", p.ParameterlessPolicy)
	}
	if len(p.ParameterlessAddresses) == 0 {
		t.Fatal("the ft991a profile declares ParameterlessExcluded with no addresses")
	}
	out := make(map[menuAddr]bool, len(p.ParameterlessAddresses))
	for _, addr := range p.ParameterlessAddresses {
		if addr[1] != 0 || addr[2] != 0 {
			t.Fatalf("the ft991a profile's ParameterlessAddresses holds %v, but this radio's EX address is a SINGLE component and both later members must be 0 (AddressSingle)", addr)
		}
		out[menuAddr(addr[0])] = true
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

// parseMenuNumber decodes the chart's printed three-digit MENU Number, which
// is this radio's whole EX address — "087" into 87.
//
// It is deliberately exact: three bytes, every one a decimal digit. A cell
// that is merely similar is a Fatal, not a silent partial parse, because a
// permissive reading is precisely how two genuinely different addresses
// would be folded into false agreement. strconv.Atoi is not given the cell
// unchecked for the same reason — it would accept a sign or spaces.
func parseMenuNumber(t *testing.T, where, raw string) menuAddr {
	t.Helper()
	const want = 3
	if len(raw) != want {
		t.Fatalf("%s: %q is not a three-digit MENU number (%d bytes)", where, raw, len(raw))
	}
	for i := 0; i < want; i++ {
		if raw[i] < '0' || raw[i] > '9' {
			t.Fatalf("%s: %q is not a three-digit MENU number (byte %d is %q)", where, raw, i, raw[i:i+1])
		}
	}
	return menuAddr(atoiOrFatal(t, where, "menu number", raw))
}

func atoiOrFatal(t *testing.T, where, field, raw string) int {
	t.Helper()
	n, err := strconv.Atoi(raw)
	if err != nil {
		t.Fatalf("%s: bad %s %q: %v", where, field, raw, err)
	}
	return n
}

// sortedAddrs returns m's MENU numbers in ascending order, so that every
// failure list is stable and the ledger walk sees the chart in chart order.
func sortedAddrs(m map[menuAddr]chartRow) []menuAddr {
	out := make([]menuAddr, 0, len(m))
	for a := range m {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func sortedPages(m map[int]ledgerRow) []int {
	out := make([]int, 0, len(m))
	for page := range m {
		out = append(out, page)
	}
	sort.Ints(out)
	return out
}

func sameAddrs(got, want []menuAddr) bool {
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
