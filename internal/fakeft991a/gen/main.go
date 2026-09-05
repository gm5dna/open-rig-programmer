// SPDX-License-Identifier: GPL-3.0-or-later

// Command gen projects internal/fakeft991a's own copy of TRANSCRIPTION B into
// the fake's EX (MENU) inventory, emitting exinventory_gen.go. It is invoked by
// the //go:generate directive in internal/fakeft991a/ex.go, whose working
// directory is internal/fakeft991a — hence the relative paths on the
// directive's flags.
//
// # STANDARD LIBRARY ONLY, and why that is the whole point
//
// This command imports nothing project-internal, and in particular NOT
// internal/extable — the machinery that generates the DIALECT's inventory from
// transcription A. fakeft991a's recursive no-imports fence (imports_test.go,
// TestNoCoreImports and TestNoCoreImports_ReachesTheGenerator) enforces that
// mechanically for this directory, and the reason is the design this file
// exists to serve:
//
//	the dialect's inventory is generated from transcription A by
//	internal/extable; this fake's is generated from transcription B by the
//	code below; and core/transport's cross-check proves the two agree.
//
// A defect in either transcription, or in either generator, therefore surfaces
// as a cross-check MISMATCH. Reaching for extable here — even for something as
// innocent as its CSV row parser — would put one parser on both sides of that
// comparison, and a shared parsing bug would reproduce itself identically into
// both inventories and be invisible.
//
// So the CSV reading below is written afresh against B's OWN schema.
//
// # A NEW GENERATOR, not internal/fakeft891/gen's with the paths changed
//
// The two charts deliver the SAME three columns — menu_number,name,digits —
// and that resemblance is exactly why this file is written rather than copied:
// three of its four structural facts differ, and each difference is a property
// of the printed chart.
//
//   - THE ADDRESS IS A SINGLE COMPONENT. This chart prints a THREE-digit MENU
//     Number that is the whole address — 087 is P1=87, with P2 and P3 zero
//     (core/cat's EXAddressSingle) — where the FT-891's four digits are a
//     (P1,P2) pair. So there are no GROUPS: the FT-891 generator's group key,
//     its per-group widths string, its "one contiguous block" rule and its
//     "P2 runs from 01" rule all have no counterpart, and the projection is a
//     flat list of one entry per address. What replaces them is the property
//     this chart does have, which theirs does not: ONE run of addresses,
//     001 upwards, consecutive to the last row (checkRun).
//   - THERE IS A ROW WITH NO PARAMETER AT ALL. 087 RADIO ID prints a single
//     hyphen for its Digits and ten spaced hyphens for its parameter legend,
//     so it names no field an EX frame could read or write. It is transcribed
//     and counted, and EXCLUDED from the inventory — see parameterlessAddrs.
//     The FT-891's chart has no such row and its generator has no such rule.
//   - THE WIDTH ALPHABET RUNS TO EIGHT, from one row: 151 PRESET FREQUENCY,
//     whose "00030000 ~ 47000000" parameter is eight digits wide. The FT-891's
//     stops at 5 and the FTdx10's at 4. See widthToken and maxWidth.
//
// The fourth fact is shared and is stated because it is a SILENCE rather than
// a difference: there are no group labels and no parameter-legend column, so
// there is no cell from which a TEXT item could be identified, and every
// projected row is numeric. That is a statement about the delivered SCHEMA and
// not a claim about the radio — see widthToken.
//
// # What is projected, and what is deliberately dropped
//
// The output models WIRE BEHAVIOUR ONLY — which addresses answer, and each
// one's raw P4 reply WIDTH. B's name column (the item's human name) is NOT
// emitted: this fake answers menu reads, it does not interpret menu meanings,
// and the dialect is the layer that carries names. The name is read only so
// that a malformed row can be NAMED in an error message, and so that a blank
// cell — the signature of a misparsed row — is refused rather than silently
// projected.
package main

import (
	"bytes"
	"encoding/csv"
	"flag"
	"fmt"
	"go/format"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// bHeader is transcription B's exact header row, as delivered and committed.
// It is pinned so that a schema change fails LOUDLY here rather than being
// silently misparsed into a plausible wrong table.
//
// IT IS THE FT-891'S HEADER TOO, BYTE FOR BYTE, and that is the one place this
// generator's refusals cannot help: the two radios' transcriptions were
// delivered in the same three-column schema by the same brief. What separates
// them is the ADDRESS WIDTH — parseMenuNumber refuses anything that is not
// exactly three digits, so an FT-891 row ("0101,AGC FAST DELAY,4") is rejected
// at its first data line rather than read as a plausible FT-991A address.
var bHeader = []string{"menu_number", "name", "digits"}

// Column indices into a B record.
const (
	colMenuNumber = iota
	colName
	colDigits
	numCols
)

// menuNumberDigits is the width of B's address cell: the chart's three-digit
// MENU Number, "P1 : 001 - 153 (MENU Number)" as the EX block prints it. There
// is no second or third component, which is what makes this radio's EX read
// frame six bytes — the narrowest in the family.
const menuNumberDigits = 3

// maxWidth is the widest raw P4 field this chart declares: 8. It comes from
// exactly ONE row, 151 PRESET FREQUENCY, whose "00030000 ~ 47000000" parameter
// is eight digits wide, and it is pinned independently from both sides —
// gen/main_test.go's TestParseB_TheOnlyEightWideRowIs151 from B, and
// core/cat/ft991a/crosscheck_test.go's widestRowDigits/widestRowAddr from A.
//
// EIGHT, where the FT-891's alphabet stops at five and the FTdx10's at four.
// It is a reading of THIS chart and not a widening of theirs.
const maxWidth = 8

// parameterlessToken is the Digits cell this transcription writes for a row
// that prints NO parameter: a question mark.
//
// # IT IS '?' HERE AND '-' IN internal/extable, AND BOTH ARE RIGHT (plan P18)
//
// One glyph is printed on the page. Row 087 RADIO ID's Digits cell prints a
// SINGLE HYPHEN, and its parameter legend prints ten spaced hyphens; both were
// confirmed at 900 dpi by the quarantined agent that produced this artefact
// (core/cat/ft991a/testdata/transcription-b.md §2(b)).
//
// The two derivations of that page then SPELL it differently, because they
// were written under different transcription conventions:
//
//   - TRANSCRIPTION A keeps the raw '-', and internal/extable's
//     ParameterlessExcluded policy keys on that byte
//     (internal/extable/profile.go's ft991aProfile);
//   - TRANSCRIPTION B — this file's source — was told to write '?' for any
//     cell that is not an integer, so it reads "087,RADIO ID,?" and its own
//     report says in terms that the cell prints a hyphen.
//
// So this generator's token is '?' BECAUSE THAT IS WHAT ITS OWN SOURCE SPELLS.
// It is not extable's bound consulted from here, nor extable's bound restated
// here: each generator reads the spelling of the artefact it is generating
// from, which is the project's standing rule that a bound is consulted from
// the same place as its datum. Teaching this generator extable's '-' would
// break that rule in the one direction that matters — it would make this side
// of the cross-check depend on the other side's reading of the page.
//
// The milestone's plan states this ruling (P18) rather than leaving the two
// generators, written weeks apart, to meet it separately. Task 6's cross-check
// of the two transcriptions normalises '?' to '-' on this ONE address at
// comparison time, and never by editing either artefact.
const parameterlessToken = "?"

// parameterlessAddrs are the addresses this chart prints with no parameter at
// all, and which are therefore EXCLUDED from the inventory: a menu number
// naming no field is not an address an EX frame could read or write.
//
// It is spelt here as this generator's OWN declaration, read from this
// package's own source artefact — not imported from, nor derived from,
// internal/extable's ft991aProfile.ParameterlessAddresses, which says the same
// thing about the same row from transcription A. Two independent readings of
// one page is the whole mechanism of the cross-check; one of them consulting
// the other would dissolve it.
//
// The rule is enforced in BOTH directions (parseB): a '?' cell on an address
// in this set is excluded, and a '?' cell on any other address is REFUSED —
// as is an address in this set whose cell is NOT '?', which would mean the
// chart, or its transcription, had changed under a declaration that no longer
// describes it.
var parameterlessAddrs = map[string]bool{"087": true}

// row is one parsed B data row, reduced to what the projection needs.
type row struct {
	// addr is the whole EX address: the menu number's three ASCII digits, as
	// the wire carries them.
	addr string
	// num is that address as a number, for the consecutive-run check.
	num int
	// token is the width token: '1'..'8' for a numeric field of that many
	// bytes. There is no text token — see widthToken.
	token byte
	// excluded marks a row that is transcribed and counted but names no
	// field: parameterlessAddrs. Its token is meaningless and it is not
	// rendered.
	excluded bool
	// line is the 1-based physical line in the CSV, header included, for
	// error messages and for the emitted provenance comment.
	line int
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("fakeft991a/gen: ")

	csvPath := flag.String("csv", "", "path to this package's copy of transcription B (required)")
	outPath := flag.String("out", "", "path of the generated Go file to write (required)")
	flag.Parse()

	// No positional operands, and no defaulted paths. Silently ignoring an
	// operand — or defaulting a path — would let a mistyped invocation read as
	// a successful run that generated something other than what was asked for
	// (internal/extable/gen's own reasoning, which is about invocation
	// discipline rather than about EX tables, and is worth sharing).
	if flag.NArg() > 0 {
		log.Fatalf("unexpected positional arguments %v — gen takes only -csv and -out", flag.Args())
	}
	if *csvPath == "" || *outPath == "" {
		log.Fatal("-csv and -out are both required; see the //go:generate directive in ex.go")
	}

	data, err := os.ReadFile(*csvPath)
	if err != nil {
		log.Fatalf("reading %s: %v", *csvPath, err)
	}
	rows, err := parseB(data)
	if err != nil {
		log.Fatalf("parsing %s: %v", *csvPath, err)
	}
	if err := checkRun(rows); err != nil {
		log.Fatalf("checking %s: %v", *csvPath, err)
	}
	out, err := render(rows, *csvPath)
	if err != nil {
		log.Fatalf("rendering %s: %v", *outPath, err)
	}
	if err := os.WriteFile(*outPath, out, 0o644); err != nil {
		log.Fatalf("writing %s: %v", *outPath, err)
	}
}

// parseB parses transcription B into rows, in file order. Every malformed
// input is an error rather than a skipped row: this CSV is a committed,
// hash-frozen evidential artefact, so anything the projection cannot read is a
// finding to report, never a row to drop.
//
// The ONE row it does not project is the parameterless one, and that is not a
// skip: it is parsed, counted and marked, so that checkRun still sees the
// chart's whole run of addresses and the rendered file can name what it left
// out.
func parseB(data []byte) ([]row, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = numCols // the header sets it; this states it up front
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("empty CSV: no header row")
	}
	if got := records[0]; !slices.Equal(got, bHeader) {
		return nil, fmt.Errorf("header row is %v, want %v — transcription B's schema changed, or the wrong file was read", got, bHeader)
	}
	if len(records) == 1 {
		return nil, fmt.Errorf("CSV has a header and no data rows")
	}

	out := make([]row, 0, len(records)-1)
	for i, rec := range records[1:] {
		line := i + 2 // 1-based, header included
		addr, num, err := parseMenuNumber(rec[colMenuNumber])
		if err != nil {
			return nil, fmt.Errorf("line %d: menu_number cell: %w", line, err)
		}
		if strings.TrimSpace(rec[colName]) == "" {
			return nil, fmt.Errorf("line %d (%s): empty name cell — a blank name is the signature of a misparsed row, not an item without a name", line, rec[colMenuNumber])
		}

		digits := strings.TrimSpace(rec[colDigits])
		parameterless := parameterlessAddrs[addr]
		switch {
		case digits == parameterlessToken && parameterless:
			// The declared parameterless row, spelt as its own artefact spells
			// it. Counted, not projected — see parameterlessToken.
			out = append(out, row{addr: addr, num: num, excluded: true, line: line})
			continue
		case digits == parameterlessToken:
			return nil, fmt.Errorf("line %d (%s %s): digits cell %q means NO PARAMETER, and %s is not one of this generator's declared parameterless addresses (%v) — either the chart prints a row this generator does not know is parameterless, or the transcription has drifted; arbitrate against the PDF rather than widening the declaration to fit",
				line, rec[colMenuNumber], rec[colName], parameterlessToken, addr, sortedAddrs())
		case parameterless:
			return nil, fmt.Errorf("line %d (%s %s): %s is declared parameterless but its digits cell is %q, not %q — the chart, or its transcription, no longer matches the declaration",
				line, rec[colMenuNumber], rec[colName], addr, digits, parameterlessToken)
		}

		token, err := widthToken(digits)
		if err != nil {
			return nil, fmt.Errorf("line %d (%s %s): %w", line, rec[colMenuNumber], rec[colName], err)
		}
		out = append(out, row{addr: addr, num: num, token: token, line: line})
	}
	return out, nil
}

// sortedAddrs renders parameterlessAddrs deterministically, for an error
// message that must not vary between runs over one map.
func sortedAddrs() []string {
	out := make([]string, 0, len(parameterlessAddrs))
	for a := range parameterlessAddrs {
		out = append(out, a)
	}
	slices.Sort(out)
	return out
}

// widthToken derives one width token from B's digits cell.
//
// A width of 1..maxWidth is a numeric field of that many raw ASCII bytes and
// yields that digit itself. EVERY OTHER VALUE IS REFUSED, including 12 — and
// the refusal of 12 is the one worth stating, because it is where this
// generator, like the FT-891's, deliberately declines to do what the FTdx10's
// does.
//
// That generator reads a 12 as the text item (MY CALL.) when B's P4 cell also
// describes a character count, and emits a 'T' token which the fake expands to
// twelve SPACES rather than twelve zeros. THIS CHART'S B HAS NO SUCH CELL:
// three columns, no parameter legend, no text flag. So there is nothing here
// from which textness could be decided, and deciding it on the width alone
// would invent wire behaviour. The quarantined agent looked for one and
// recorded finding none — "Free-text parameter legends: none"
// (core/cat/ft991a/testdata/transcription-b.md §2(a)) — but that is a report
// about the chart, not a column in the delivered file, and this generator can
// only read the file.
//
// Refusing is therefore the honest answer AND the one that fails loudly: if
// this chart ever turns out to carry a text row, the generator stops rather
// than guessing, and the arbitration is against the PDF. The independent check
// on the whole question is the cross-check, whose dialect side comes from
// transcription A — which does carry a text column.
func widthToken(digits string) (byte, error) {
	s := strings.TrimSpace(digits)
	if len(s) != 1 || !isDigit(s[0]) {
		return 0, fmt.Errorf("digits cell %q is not exactly one ASCII digit", digits)
	}
	n := int(s[0] - '0')
	if n < 1 || n > maxWidth {
		return 0, fmt.Errorf("digits %d is outside 1-%d: the inventory has no token for it, and this three-column schema carries nothing from which a wider or a text field could be described", n, maxWidth)
	}
	return byte('0' + n), nil
}

// parseMenuNumber reads one of B's address cells — "087" — as the whole EX
// address it is, returning both the three ASCII digits the wire carries and
// their numeric value.
//
// It refuses anything that is not exactly three ASCII digits, so a cell of a
// different shape cannot be silently reduced to a plausible address. Two
// mistakes it is aimed at in particular: a FOUR-digit cell (the FT-891's pair
// address, whose transcription B has this file's exact header, and which would
// otherwise parse as a plausible three-digit address with one digit quietly
// discarded), and a two-digit one (a lost leading zero).
func parseMenuNumber(cell string) (addr string, num int, err error) {
	s := strings.TrimSpace(cell)
	if len(s) != menuNumberDigits {
		return "", 0, fmt.Errorf("%q is not exactly three ASCII digits", cell)
	}
	for i := 0; i < len(s); i++ {
		if !isDigit(s[i]) {
			return "", 0, fmt.Errorf("%q is not exactly three ASCII digits", cell)
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil { // unreachable: every byte is a digit
		return "", 0, fmt.Errorf("%q is not exactly three ASCII digits", cell)
	}
	return s, n, nil
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// checkRun enforces the one structural property this flat chart has, in place
// of the FT-891 generator's group rules: the menu numbers are ONE CONSECUTIVE
// RUN, opening at 001 and increasing by exactly one to the last row, with no
// gap and no repeat.
//
// That is what B's own reconciliation records — "both passes read 001 … 153,
// strictly increasing by one, no gaps and no repeats"
// (core/cat/ft991a/testdata/transcription-b.md §3) — and it is worth enforcing
// rather than trusting, because it is what makes the rendered file's
// address-to-CSV-line mapping true: a row's line is its address plus one,
// EXCLUDED ROWS INCLUDED, so the generated file can state the mapping once
// instead of repeating a line number on each of 152 entries.
//
// It is a refusal rather than a repair. A gap in a transcription of a ruled
// chart is either a transcription defect or a chart this projection cannot
// model, and both are findings.
func checkRun(rows []row) error {
	if len(rows) == 0 {
		return fmt.Errorf("no rows")
	}
	if rows[0].num != 1 {
		return fmt.Errorf("line %d: the chart opens at %s, want 001", rows[0].line, rows[0].addr)
	}
	for i := 1; i < len(rows); i++ {
		if want := rows[i-1].num + 1; rows[i].num != want {
			return fmt.Errorf("line %d: menu number %s follows %s — the chart's numbers must run consecutively, and %03d is missing or repeated", rows[i].line, rows[i].addr, rows[i-1].addr, want)
		}
	}
	return nil
}

// render emits the generated Go file. csvPath names the source; only its BASE
// NAME is written into the output, so where the generator was invoked from
// cannot leak into the committed bytes — which is what lets gen's own staleness
// test read the CSV as "../transcription-b.csv" and still render the file the
// //go:generate directive produces from "transcription-b.csv".
//
// The output is DETERMINISTIC: entries are emitted in the file order checkRun
// validated, every value derives from the parsed rows, nothing is ranged over
// a map, and the whole buffer is run through go/format — so two runs over equal
// input produce byte-identical, gofmt-clean output. That is what makes the
// staleness test's byte comparison (gen/main_test.go) a meaningful check rather
// than a formatting lottery.
func render(rows []row, csvPath string) ([]byte, error) {
	csvName := filepath.Base(csvPath)
	var kept, excluded []row
	for _, r := range rows {
		if r.excluded {
			excluded = append(excluded, r)
			continue
		}
		kept = append(kept, r)
	}
	if len(kept) == 0 {
		return nil, fmt.Errorf("no rows to render")
	}

	var buf bytes.Buffer
	buf.WriteString("// SPDX-License-Identifier: GPL-3.0-or-later\n\n")
	fmt.Fprintf(&buf, "// Code generated by internal/fakeft991a/gen from %s. DO NOT EDIT.\n\n", csvName)
	buf.WriteString("package fakeft991a\n\n")
	buf.WriteString("// exItems is this fake's EX (MENU) inventory: one entry per menu address, in\n")
	buf.WriteString("// the chart's own order, with the raw P4 reply WIDTH that address answers as a\n")
	buf.WriteString("// token '1'..'8' — a numeric field of that many raw ASCII bytes. There is NO\n")
	buf.WriteString("// text token: this chart's transcription carries no column from which a text\n")
	buf.WriteString("// item could be identified, so every row is projected as numeric\n")
	buf.WriteString("// (gen/main.go's widthToken says what that does and does not claim).\n")
	buf.WriteString("// ex.go expands this into the address -> default raw P4 map the fake answers\n")
	buf.WriteString("// from, and states what the table does and does not claim.\n")
	buf.WriteString("//\n")
	buf.WriteString("// THE ADDRESS IS A SINGLE COMPONENT: the chart's three-digit MENU Number is the\n")
	buf.WriteString("// whole address, with P2 and P3 zero (core/cat's EXAddressSingle), which is why\n")
	buf.WriteString("// this radio's EX read frame is six bytes — the narrowest in the family. There\n")
	buf.WriteString("// are no groups, so this is a flat list rather than the FT-891's widths\n")
	buf.WriteString("// strings.\n")
	buf.WriteString("//\n")
	fmt.Fprintf(&buf, "// It is a PROJECTION OF TRANSCRIPTION B (%s), derived from that\n", csvName)
	buf.WriteString("// artefact's digits column alone. The dialect's inventory\n")
	buf.WriteString("// (core/cat/ft991a/exinventory_gen.go) is generated from transcription A by\n")
	buf.WriteString("// different code, and core/transport's cross-check proves the two agree — so a\n")
	buf.WriteString("// defect in either transcription or either generator shows up there.\n")
	buf.WriteString("//\n")
	buf.WriteString("// EVERY ENTRY'S SOURCE LINE IS ITS ADDRESS PLUS ONE, and no line number is\n")
	buf.WriteString("// repeated below because of it: the chart's menu numbers are one consecutive\n")
	buf.WriteString("// run from 001, which gen/main.go's checkRun enforces rather than assumes.\n")
	buf.WriteString("//\n")
	buf.WriteString("// The exItem type is declared in ex.go, not here: this file is DATA, and a\n")
	buf.WriteString("// type declared in it would be a second thing a hand-edit could reach.\n")
	for _, r := range excluded {
		fmt.Fprintf(&buf, "//\n// EXCLUDED, and counted in that run: %s (%s line %d),\n", r.addr, csvName, r.line)
		buf.WriteString("// whose digits cell is this transcription's no-parameter token. The chart\n")
		buf.WriteString("// prints that row with no parameter at all, so it names no field an EX frame\n")
		buf.WriteString("// could read or write (gen/main.go's parameterlessToken, and plan decision\n")
		buf.WriteString("// P18 on why the two transcriptions spell one printed hyphen differently).\n")
	}
	buf.WriteString("//\n")
	fmt.Fprintf(&buf, "// %s of a %d-row chart. Regenerate with `go generate ./internal/fakeft991a`;\n",
		plural(len(kept), "item"), len(rows))
	buf.WriteString("// gen/main_test.go refuses a file that has drifted from the CSV.\n")
	buf.WriteString("var exItems = []exItem{\n")
	for _, r := range kept {
		fmt.Fprintf(&buf, "\t{%s, %s},\n", strconv.Quote(r.addr), strconv.QuoteRune(rune(r.token)))
	}
	buf.WriteString("}\n")

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("formatting generated Go: %w", err)
	}
	return formatted, nil
}

// plural renders a count with its noun, pluralised. A generated comment reading
// "1 items" is the kind of small wrongness that makes a reader distrust the
// numbers beside it.
func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
