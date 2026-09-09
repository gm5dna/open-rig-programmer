// SPDX-License-Identifier: GPL-3.0-or-later

package fakets990

import (
	"bytes"
	_ "embed"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
)

// # STANDARD LIBRARY ONLY, and why that is the whole point
//
// The parsing below imports nothing project-internal, and in particular NOT
// internal/extable — the machinery that derives the DIALECT's inventory from
// transcription A. imports_test.go's recursive fence enforces that
// mechanically, and the reason is the design this file exists to serve: the
// dialect's inventory comes from transcription A (core/kw/ma/menu990s.csv) by
// one piece of code, this fake's from transcription B by the code below, and
// core/transport/ex_crosscheck_ts990_test.go proves the two agree. One parser
// on both sides of that comparison would reproduce a shared parsing bug into
// both inventories invisibly.
//
// There is NO GENERATOR and no generated file here (the plan's P19): the
// projection happens at init from the same committed bytes.

// transcriptionB990S is this package's OWN COPY of transcription B, embedded
// and projected at init. PROVENANCE.md records where the copy came from and
// why it is a copy rather than a move; core/transport's cross-check asserts it
// is still byte-identical to the codec-side artefact, because "the codec from
// A versus the fake from B" holds only for as long as this side's copy really
// is B.
//
//go:embed transcription-b-990s.csv
var transcriptionB990S []byte

// exWidths990S is the TS-990S's EX (MENU) inventory as this fake sees it:
// five-character wire address -> raw P5 width in bytes.
//
// IT IS A MAP AND NOT A COMPACT INDEXED STRING, which is where this family
// parts company with internal/fakets590's projection. That chart's address is
// a single three-digit menu number running from 000 with no gaps, so the index
// of a string could BE the address. This one is a GROUPED TRIPLE — "P1", "P2
// (Category Number)", "P3 (Entry Number)" (990:1720-1735) — over a domain the
// chart prints sparsely: menu type 0 carries categories 00 to 09 with
// different item counts each, and menu type 1 carries category 00 alone. No
// index arithmetic can express that, so the address is carried whole.
var exWidths990S = mustWidths("transcription-b-990s.csv", transcriptionB990S)

// exDefaultDigit is the digit byte a menu's raw P5 defaults to.
//
// INVENTED — doc.go's register entry THE EX MENU VALUES ARE INVENTED. The two
// parameter lists print each menu's available SETTINGS and never a shipped
// default, so there is nothing to source a real one from; and `rigprog read
// --settings --fake` renders these bytes to a user, who must not read them as
// what a TS-990S ships with. A placeholder that is obviously uniform is harder
// to mistake for evidence than a plausible-looking spread of values.
const exDefaultDigit = "0"

// maxWidth is the widest raw P5 this chart declares: 15, from "A power-on
// message can vary in length from 0 to 15 characters." (990:1751), the widest
// of the FIVE classes the EX block prints beside it — 3 normally (990:1746), 4
// for PF keys (990:1747-1748), 8 for a frequency setting (990:1749-1750) and 0
// to 10 for screen-saver text (990:1752).
//
// THE EIGHT-DIGIT CLASS IS THIS BOOK'S OWN and the 890S prints no such class,
// which is one reason no Kenwood fake may borrow a sibling's widths.
//
// It is spelt here as the number transcription B's own digits column prints
// for that row. The independent check on the same fact is the codec's side:
// the design's A19 bounds every EX answer at fifteen characters and
// core/kw/ma's ParseEXAnswer enforces the per-row width the codec's own
// inventory carries.
const maxWidth = 15

// THIS CHART HAS NO PARAMETERLESS ROW, and the absence is a fact of this book
// rather than an omission here.
//
// internal/fakets890's projection excludes four Advanced Menu rows the 890S
// prints with real addresses and the body "Does not correspond to a command"
// (890:2273-2280, erratum E16). This book prints no such row: every one of
// transcription B's addressed rows carries a width, the production inventory
// excludes none, and the two sides are therefore the same length as the leg.
// There is nothing here for a sibling's four-address list to be copied into,
// and TestEXDefaults_CarriesEveryAddressTranscriptionBPrints holds it in both
// directions.

// THE ONE CORRECTION THIS PROJECTION APPLIES, and it is one the evidence leg
// got wrong rather than one the codec's side invented.
//
// The EX block's own P5 note prints five width classes, and one of them is
// scoped to the PF keys: "PF key settings use 4 digits (refer to the PF Key
// assignment ID lists)." (990:1747-1748). Transcription B recorded the
// ordinary three-digit legend on all EIGHTEEN of this chart's PF rows, and
// transcription A recorded four. THE LEG IS WRONG, NOT A: the sentence is
// printed, it names the class by name, and the list it sends the reader to —
// the PF Key Assignment Lists (990:2287 onwards) — prints every allotment ID
// as four digits. So the book settles the width twice, in the note and in the
// list the note refers to, and the leg is FROZEN EVIDENCE, so the error is
// corrected HERE, in the projection, and never in the CSV.
//
// APPLYING IT IS NOT "EDITING A TABLE TO MAKE THE CROSS-CHECK PASS", and the
// distinction matters. The authority is this book's own sentence, read at this
// side independently; nothing here consults transcription A, the generated
// inventory or internal/extable, and nothing here reads the 890S's sentence
// either — that radio's run is SEVENTEEN rows and ends at 0/00/31, where this
// one is eighteen and ends at 0/00/32. It is the SAME CLASS AS
// internal/fakets890's FOUR-ADDRESS EXCLUSION: a fact this book prints that
// the leg does not carry, applied at this side from an independently written
// list — a fake that answered a width the book contradicts would be modelling
// the radio wrongly on a point the book settles, which is not what an
// independent evidence leg is for.
//
// The correction is applied BY ADDRESS, and it is refused unless every one of
// the eighteen is present AND carries the width the ruling's shape predicate
// requires (projectWidths). "The PF rows differ" must not quietly become "the
// PF rows differ by something else".
//
// THE EIGHTEEN ARE THIS CHART'S OWN RUN, 0/00/15 … 0/00/32 (990:1793-1810).
// The 990S inserts Voice (Main Band) and Voice (Sub Band) at items 17 and 18,
// which the 890S has not, so its run is one longer and shifted at the tail;
// copying the sibling's seventeen addresses would leave two of this chart's PF
// rows answering the three bytes the leg records.
var pfKeyAddresses = []string{
	"00015", "00016", // PF A, PF B (990:1793-1794)
	"00017", "00018", // Voice (Main Band), Voice (Sub Band) — this radio's own two (990:1795-1796)
	"00019", "00020", "00021", "00022", "00023", "00024", "00025", "00026", // External PF 1-8 (990:1797-1804)
	"00027", "00028", "00029", "00030", // Microphone PF 1-4 (990:1805-1808)
	"00031", "00032", // Microphone Down, Microphone Up (990:1809-1810)
}

// The two widths ruling R-B is about: what the book prints for a PF key row,
// and what transcription B recorded there.
const (
	pfKeyWidth    = 4
	pfKeyLegWidth = 3
)

// EXDefaults returns a fresh copy of this radio's default menu state:
// five-character wire address -> default raw P5 (width n -> n x '0').
//
// THE VALUE IS THE MENU'S OWN WIDTH AND NOT THE FIFTEEN-WIDE WINDOW the
// answer's position ruler draws (erratum E19). The padding into that window is
// the WIRE's business and happens in ex.go, so that this table stays
// comparable, address for address and width for width, with the codec's
// inventory in core/transport's cross-check.
//
// THERE IS NO RUNTIME-OVERRIDE TABLE HERE, and the absence is a decision.
// internal/fakeradio has two tables — its manual transcription and a runtime
// view with hardware observations overlaid — because it HAS observations. No
// TS-990S has ever been asked anything by this project, so there is nothing to
// overlay: what this function returns IS what a *Radio answers. When 990S
// evidence does arrive, fakeradio's split is the pattern to copy — a separate
// overrides table with its own citation, never an edit to this projection or
// to the CSV it reads.
//
// Test-inspection API, and the fake's half of the cross-check: every call
// returns an independent map, so mutating one call's result can never affect
// another call's, nor any *Radio's own stored exSettings.
func EXDefaults() map[string]string {
	out := make(map[string]string, len(exWidths990S))
	for addr, w := range exWidths990S {
		out[addr] = strings.Repeat(exDefaultDigit, w)
	}
	return out
}

// mustWidths is the init-time projection. Every malformed input PANICS rather
// than yielding a shorter table: this CSV is a committed, hash-frozen
// evidential artefact, so anything the projection cannot read is a finding,
// and a fake answering from a truncated inventory would be worse than one that
// refuses to start.
func mustWidths(name string, data []byte) map[string]int {
	rows, err := parseB(data)
	if err != nil {
		panic("fakets990: " + name + ": " + err.Error())
	}
	widths, err := projectWidths(rows)
	if err != nil {
		panic("fakets990: " + name + ": " + err.Error())
	}
	return widths
}

// bHeader is transcription B's exact header row, as delivered and committed.
// It is pinned so that a schema change fails LOUDLY here rather than being
// silently misparsed into a plausible wrong table — and, in particular, so
// that internal/fakets590's four-column B could never be read by this
// projection as though it were this chart's six.
var bHeader = []string{"p1", "p2", "p3", "name", "digits", "text"}

// Column indices into a B record.
const (
	colP1 = iota
	colP2
	colP3
	colName
	colDigits
	colText
	numCols
)

// bRow is one parsed B data row, reduced to what the projection needs.
type bRow struct {
	// p1, p2, p3 are the address triple: menu type, category, entry.
	p1, p2, p3 int
	// width is the raw P5 field's width in bytes, from the digits cell.
	width int
	// line is the 1-based physical line in the CSV, header included, for
	// error messages. It is the RECORD's starting line, which is not the
	// record's index plus two on a chart whose names may carry embedded
	// commas or newlines.
	line int
}

// wire renders the row's address as the five characters the frame carries:
// P1 + P2P2 + P3P3 (990:1723, 990:1726, 990:1732).
func (r bRow) wire() string { return fmt.Sprintf("%d%02d%02d", r.p1, r.p2, r.p3) }

// parseB reads transcription B and validates every cell of every row.
func parseB(data []byte) ([]bRow, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	// FieldsPerRecord is -1 so that the HEADER comparison below is what
	// reports a schema change. Left at numCols, encoding/csv refuses a
	// four-column file — internal/fakets590's own transcription B — with
	// "wrong number of fields" and never reaches the header check, which is
	// the loud failure this projection wants but pointed at the wrong fact.
	// The per-record width is checked below instead.
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading the CSV: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("the file is empty")
	}
	if strings.Join(records[0], ",") != strings.Join(bHeader, ",") {
		return nil, fmt.Errorf("header row is %q, want %q — a schema change must fail here rather than be misparsed into a plausible wrong table", strings.Join(records[0], ","), strings.Join(bHeader, ","))
	}
	if len(records) == 1 {
		return nil, fmt.Errorf("no data rows")
	}

	// The physical line of each record, because a quoted cell may carry a
	// newline and record index + 2 is then not the line number.
	lines := recordLines(data)
	if len(lines) != len(records) {
		return nil, fmt.Errorf("counted %d record start lines for %d records — the line ledger and the CSV reader disagree about this file's shape", len(lines), len(records))
	}

	out := make([]bRow, 0, len(records)-1)
	for i, rec := range records[1:] {
		line := lines[i+1]
		if len(rec) != numCols {
			return nil, fmt.Errorf("line %d: %d cells, want %d — every data row carries this chart's six columns", line, len(rec), numCols)
		}
		row := bRow{line: line}

		var err error
		// P1 is a MENU TYPE FLAG with exactly two printed values,
		// "0: Menu" and "1: Advanced Menu" (990:1722-1723), so it is one
		// digit and it is bounded. A two-digit cell here would be a
		// category cell shifted left by a lost column.
		if row.p1, err = parseComponent(rec[colP1], 1); err != nil || row.p1 > 1 {
			return nil, fmt.Errorf("line %d: p1 cell %q is not one of the two printed menu types, 0 (Menu) or 1 (Advanced Menu) (990:1722-1723)", line, rec[colP1])
		}
		// P2 and P3 are "00 ~ 99" (990:1726, 990:1732): exactly two digits
		// each. A one-digit cell is a lost leading zero, which would parse
		// as a plausible number and address the wrong menu.
		if row.p2, err = parseComponent(rec[colP2], 2); err != nil {
			return nil, fmt.Errorf("line %d: p2 cell %q is not exactly two ASCII digits (990:1726)", line, rec[colP2])
		}
		if row.p3, err = parseComponent(rec[colP3], 2); err != nil {
			return nil, fmt.Errorf("line %d: p3 cell %q is not exactly two ASCII digits (990:1732)", line, rec[colP3])
		}
		if strings.TrimSpace(rec[colName]) == "" {
			return nil, fmt.Errorf("line %d (%s): empty name cell — a blank name is the signature of a misparsed row, not an item without a name", line, row.wire())
		}
		if row.width, err = widthFor(rec[colDigits], rec[colText]); err != nil {
			return nil, fmt.Errorf("line %d (%s %s): %w", line, row.wire(), rec[colName], err)
		}
		out = append(out, row)
	}
	return out, nil
}

// parseComponent parses one address cell of exactly n ASCII digits.
func parseComponent(cell string, n int) (int, error) {
	s := strings.TrimSpace(cell)
	if len(s) != n {
		return 0, fmt.Errorf("%q is not exactly %d ASCII digits", cell, n)
	}
	v := 0
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, fmt.Errorf("%q is not exactly %d ASCII digits", cell, n)
		}
		v = v*10 + int(s[i]-'0')
	}
	return v, nil
}

// widthFor derives one raw P5 width from B's digits cell.
//
// # THE text COLUMN IS VALIDATED AND DELIBERATELY NOT PROJECTED
//
// B carries a text column flagging the rows its transcriber read as text. This
// projection reads it only to REFUSE a cell that is neither "0" nor "1" — a
// value outside the schema is a misparsed row — and derives the width either
// way, so every address this fake answers replies with its width in '0' bytes.
//
// That is not the projection declining to model a distinction. It is the
// repository's RULING, applied: the text flag is a CONVENTION and the digits
// are the datum, because A and B were briefed with different definitions of
// "text row" and each applied its own consistently. On this chart the two legs
// disagree at TWENTY-NINE addresses — the 28 Fixed Mode band-limit rows and
// Contest Number, all flagged text by B — and the ruling is that only a row
// reading "Up to N alphanumeric characters" is a text row (990:1778-1779),
// which leaves this chart's Screen Saver Message and Power-on Message as the
// only two. A fake that projected B's flag would answer SPACES at
// twenty-nine addresses the repository has ruled numeric, on the authority of
// a column its own cross-check declines to compare. NOTE WHAT IT WOULD NOT
// TOUCH EITHER WAY: the flag is not a WIDTH, so projecting it or not changes
// no address's byte count and cannot disturb the cross-check's width leg.
func widthFor(digits, text string) (int, error) {
	switch strings.TrimSpace(text) {
	case "0", "1":
	default:
		return 0, fmt.Errorf("text cell %q is neither %q nor %q — transcription B's text column is a flag, and a third value is a misparsed row", text, "0", "1")
	}
	n, err := strconv.Atoi(strings.TrimSpace(digits))
	if err != nil {
		return 0, fmt.Errorf("digits cell %q is not a number: %w", digits, err)
	}
	if n < 1 || n > maxWidth {
		return 0, fmt.Errorf("digits %d is outside 1-%d: %d is the widest field this chart prints (990:1751), and a zero-width answer carries no P5 at all", n, maxWidth, maxWidth)
	}
	return n, nil
}

// recordLines returns the 1-based physical line on which each CSV record
// starts, header included.
//
// It exists because encoding/csv does not report it and because a quoted cell
// may carry a comma or a newline — this chart's names are long and
// parenthesised, and one acquiring a comma is a matter of an editorial pass,
// not of luck. Counting quote parity is the whole of the job: a doubled quote
// inside a quoted field flips the state twice and so leaves it unchanged,
// which is the correct reading.
//
// It is internal/fakets890's function, COPIED rather than re-derived: two
// fakes that may not share code should not carry two DIFFERENT readings of one
// CSV convention, and a third variant is what a re-derivation would produce.
func recordLines(data []byte) []int {
	var out []int
	line, inQuotes, started := 1, false, false
	for i := 0; i < len(data); i++ {
		switch data[i] {
		case '"':
			inQuotes = !inQuotes
			if !started {
				out, started = append(out, line), true
			}
		case '\n':
			if !inQuotes {
				line, started = line+1, false
				continue
			}
			line++
		case '\r':
			// Part of a CRLF pair; the '\n' does the work.
		default:
			if !started {
				out, started = append(out, line), true
			}
		}
	}
	return out
}

// projectWidths folds rows into the address -> width table, applying ruling
// R-B and enforcing the structural property the table depends on.
//
// A REPEATED ADDRESS IS REFUSED rather than overwritten. A silent overwrite
// would leave the table one row short, and the cross-check would then report
// the address as ABSENT — pointing at the wrong defect, in the wrong document.
//
// NOTHING IS EXCLUDED. This chart prints no parameterless row (see the comment
// above pfKeyAddresses' neighbour), so unlike internal/fakets890's projection
// this one removes no address and the table is the leg's own set.
func projectWidths(rows []bRow) (map[string]int, error) {
	out := make(map[string]int, len(rows))
	for _, r := range rows {
		addr := r.wire()
		if _, seen := out[addr]; seen {
			return nil, fmt.Errorf("line %d: address %s appears twice — a repeat would overwrite one row's width with another's and be reported as a MISSING address by the cross-check", r.line, addr)
		}
		out[addr] = r.width
	}
	for _, addr := range pfKeyAddresses {
		w, ok := out[addr]
		if !ok {
			return nil, fmt.Errorf("PF key address %s is not in this transcription: ruling R-B is about a row that exists, so its absence means this is not transcription B", addr)
		}
		if w != pfKeyLegWidth {
			return nil, fmt.Errorf("PF key address %s carries width %d, and ruling R-B's shape says this leg reads %d there (990:1747-1748 prints %d): the divergence has changed rather than gone, which is an arbitration and not a correction to apply here", addr, w, pfKeyLegWidth, pfKeyWidth)
		}
		out[addr] = pfKeyWidth
	}
	return out, nil
}
