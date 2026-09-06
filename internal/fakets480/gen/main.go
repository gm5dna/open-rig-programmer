// SPDX-License-Identifier: GPL-3.0-or-later

// Command gen projects internal/fakets480's own copy of TRANSCRIPTION B into
// this fake's compact EX (MENU) inventory, emitting exinventory_gen.go. It is
// invoked by the //go:generate directive in internal/fakets480/ex.go, whose
// working directory is internal/fakets480 — hence the relative paths on the
// directive's flags.
//
// # STANDARD LIBRARY ONLY, and why that is the whole point
//
// This command imports nothing project-internal, and in particular NOT
// internal/extable — the machinery that generates the CODEC's inventory from
// transcription A. fakets480's recursive no-imports fence (imports_test.go,
// TestNoCoreImports) enforces that mechanically for this directory, and the
// reason is the design this file exists to serve:
//
//	the codec's inventory (core/kw/ts480/exinventory_gen.go) is generated
//	from transcription A by internal/extable; this fake's is generated from
//	transcription B by the code below; and core/transport's cross-check
//	proves the two agree.
//
// A defect in either transcription, or in either generator, therefore
// surfaces as a cross-check MISMATCH. Reaching for extable here — even for
// something as innocent as its CSV row parser — would put one parser on both
// sides of that comparison, and a shared parsing bug would reproduce itself
// identically into both inventories and be invisible.
//
// So the CSV reading below is written afresh against B's OWN schema.
//
// # A SIBLING of internal/fakets590/gen, not an import of it
//
// The two Kenwood charts share a transcription schema, and this generator is
// deliberately a second copy of that one rather than a shared package. THE
// HARD RULE (doc.go) forbids this directory from importing anything
// project-internal, and a shared "kenwood B projector" would be exactly that
// — a package both fakes reach for, whose bug would land identically in both
// inventories and be invisible to a comparison of them. The two copies differ
// where the two books differ, and maxWidth is where that shows.
//
// # What is projected, and what is deliberately dropped
//
// The output models WIRE BEHAVIOUR ONLY — how many menu numbers the chart
// has, and each one's raw P5 reply WIDTH. B's name column is NOT emitted:
// this fake answers menu reads, it does not interpret menu meanings, and the
// codec's inventory is the layer that carries names. The name is read only so
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
var bHeader = []string{"menu_number", "name", "digits", "text"}

// Column indices into a B record.
const (
	colMenuNumber = iota
	colName
	colDigits
	colText
	numCols
)

// menuNumberDigits is the width of B's address cell: the chart's three-digit
// Menu number, "000 ~ 060: Menu No." as the EX block prints it (480:401). It
// is the WHOLE address on this family — P2, P3 and P4 are printed constants
// of the frame, each "Always 0 for the TS-480" (480:402-407).
const menuNumberDigits = 3

// maxWidth is the widest raw P5 field this chart declares: 2. The EX block
// prints the rule in prose — "A string of characters (Variable length)
// Normally 1-digit for the TS-480. Menu No. 32, 35 and 48 ~ 52 use 2-digit
// parameters." (480:409-411) — and the ceiling it names is two.
//
// IT IS A CEILING, NOT THE PROSE'S LIST. The prose omits menu 034, whose grid
// row nonetheless reaches the chart's second parameter column;
// core/kw/ts480's errata and its crosscheck_test.go pin that as a documented
// defect of the printed block, and both transcriptions read the GRID and
// record 034 as two digits. So this generator's bound is 2 and menu 034 sits
// inside it; nothing here re-adjudicates the erratum, and nothing here needs
// to, because the width it projects is the one both legs read.
//
// The independent check on the same fact is the codec's side: core/kw's
// MaxEXDigits bounds every profile, and core/kw/ts480/crosscheck_test.go
// compares A's digits against B's row for row.
const maxWidth = 2

// row is one parsed B data row, reduced to what the projection needs.
type row struct {
	// menu is the three-digit Menu number, as a number: the whole wire
	// address.
	menu int
	// token is the width token: '1' or '2' for a numeric field of that many
	// raw ASCII bytes. There is no text token — see widthToken.
	token byte
	// line is the 1-based physical line in the CSV, header included, for
	// error messages and for the emitted provenance comment. It is the
	// RECORD's starting line, which is not the record's index plus two on a
	// chart whose names carry embedded commas or newlines.
	line int
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("fakets480/gen: ")

	csvPath := flag.String("csv", "", "path to this package's copy of transcription B (required)")
	outPath := flag.String("out", "", "path of the generated Go file to write (required)")
	flag.Parse()

	// No positional operands, and no defaulted paths. Silently ignoring an
	// operand — or defaulting a path — would let a mistyped invocation read
	// as a successful run that generated something other than what was asked
	// for.
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
	widths, err := projectWidths(rows)
	if err != nil {
		log.Fatalf("projecting %s: %v", *csvPath, err)
	}
	out, err := render(widths, rows, *csvPath)
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

	// The RECORD's starting line, taken from the file itself rather than
	// counted: this chart's menu 035 is "CW keying dot, dash weight ratio", a
	// QUOTED cell carrying a comma, and a quoted cell may equally carry a
	// newline — so record index + 2 is not the physical line number.
	lines := recordLines(data)
	if len(lines) != len(records) {
		return nil, fmt.Errorf("counted %d record start lines for %d records — the line ledger and the CSV reader disagree about this file's shape", len(lines), len(records))
	}

	out := make([]row, 0, len(records)-1)
	for i, rec := range records[1:] {
		line := lines[i+1]
		menu, err := parseMenuNumber(rec[colMenuNumber])
		if err != nil {
			return nil, fmt.Errorf("line %d: menu_number cell: %w", line, err)
		}
		if strings.TrimSpace(rec[colName]) == "" {
			return nil, fmt.Errorf("line %d (%s): empty name cell — a blank name is the signature of a misparsed row, not an item without a name", line, rec[colMenuNumber])
		}
		token, err := widthToken(rec[colDigits], rec[colText])
		if err != nil {
			return nil, fmt.Errorf("line %d (%s %s): %w", line, rec[colMenuNumber], rec[colName], err)
		}
		out = append(out, row{menu: menu, token: token, line: line})
	}
	return out, nil
}

// recordLines returns the 1-based physical line on which each CSV record
// starts, header included.
//
// It exists because encoding/csv does not report it and because this chart
// makes the obvious arithmetic wrong: menu 035 is a quoted cell carrying a
// comma, and a quoted cell may equally carry a newline. Counting quote parity
// is the whole of the job — a doubled quote inside a quoted field flips the
// state twice and so leaves it unchanged, which is the correct reading.
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

// widthToken derives one width token from B's digits cell.
//
// A width of 1..maxWidth is a field of that many raw ASCII bytes and yields
// that digit itself. Every other value is REFUSED: the compact inventory has
// no token for it, and a chart row this generator cannot classify is a finding
// to report rather than a width to invent.
//
// # THE text COLUMN IS VALIDATED AND DELIBERATELY NOT PROJECTED
//
// B flags five rows of this chart as text — menus 048-052, the PF-key rows
// whose legend is printed once in a cell merged across the whole grid. This
// projection reads the column only to REFUSE a cell that is neither "0" nor
// "1" — a value outside the schema is a misparsed row — and emits the same
// numeric token either way, so every address this fake answers replies with
// its width in '0' bytes.
//
// That is the orchestrator's RULING, applied, not this generator declining to
// model a distinction. core/kw/ts480/crosscheck_test.go records it: the merged
// cell prints "00 ~ 99 (2-digit)", a NUMERIC legend, so text=0 is the
// repository's reading of all five rows; transcription A marks no text row at
// all on this chart (the ts480 profile registers TextRowsAbsent, and
// internal/extable's ParseCSV would refuse a flagged row outright); and B's
// own record flags the divergence rather than resolving it. The flag is a
// CONVENTION and the digits are the datum.
//
// A fake that projected B's flag would therefore answer SPACES at five
// addresses the repository has ruled numeric, on the authority of a column its
// own cross-check declines to compare. Projecting the width alone is what
// keeps this side's claim exactly as strong as the evidence the two legs
// share.
func widthToken(digits, text string) (byte, error) {
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
		return 0, fmt.Errorf("digits %d is outside 1-%d: the compact inventory has no token for it, and this book's EX block prints %d as the widest parameter (480:409-411)", n, maxWidth, maxWidth)
	}
	return byte('0' + n), nil
}

// parseMenuNumber parses one of B's address cells — "052" — into the menu
// number it names.
//
// It refuses anything that is not exactly three ASCII digits, so a cell of a
// different shape cannot be silently reduced to a plausible address. Two
// mistakes it is aimed at in particular: a two-digit cell (a lost leading
// zero, which on a chart numbered from 000 would still parse as a number and
// would shift the whole run), and a four-digit one (the FT-891's paired MENU
// Number, which would parse as a plausible menu with one digit quietly
// discarded).
func parseMenuNumber(cell string) (int, error) {
	s := strings.TrimSpace(cell)
	if len(s) != menuNumberDigits {
		return 0, fmt.Errorf("%q is not exactly three ASCII digits", cell)
	}
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, fmt.Errorf("%q is not exactly three ASCII digits", cell)
		}
		n = n*10 + int(s[i]-'0')
	}
	return n, nil
}

// projectWidths folds rows into the compact widths string, and enforces the
// one structural property that form depends on:
//
//	the menu numbers run 000, 001, 002 … with no gaps and no repeats,
//	because THE STRING'S INDEX IS THE MENU NUMBER.
//
// It is a refusal rather than a repair. The book prints this chart's menu
// domain as a RANGE — "000 ~ 060: Menu No." (480:401) — so a gap in a
// transcription of it is either a transcription defect or a chart this compact
// form cannot model, and both are findings. A projection that silently indexed
// past a gap would renumber every menu after it, which is the one error this
// table could make that no width comparison would catch.
func projectWidths(rows []row) (string, error) {
	var b strings.Builder
	for i, r := range rows {
		if r.menu != i {
			return "", fmt.Errorf("line %d: menu %03d appears at position %d — this chart's menu numbers run from 000 with no gaps (480:401), and the compact inventory's index IS the menu number", r.line, r.menu, i)
		}
		b.WriteByte(r.token)
	}
	return b.String(), nil
}

// chunk is how many width tokens one emitted string line carries. TEN, so that
// a line's first menu number is a round ten and a reader can find a given menu
// by counting lines rather than characters.
const chunk = 10

// render emits the generated Go file. csvPath names the source; only its BASE
// NAME is written into the output, so where the generator was invoked from
// cannot leak into the committed bytes — which is what lets gen's own
// staleness test read the CSV as "../transcription-b-480.csv" and still render
// the file the //go:generate directive produces from "transcription-b-480.csv".
//
// The output is DETERMINISTIC: the widths string comes from the parsed rows in
// the file order projectWidths validated, nothing is ranged over a map, and
// the whole buffer is run through go/format — so two runs over equal input
// produce byte-identical, gofmt-clean output. That is what makes the staleness
// test's byte comparison (gen/main_test.go) a meaningful check rather than a
// formatting lottery.
func render(widths string, rows []row, csvPath string) ([]byte, error) {
	if widths == "" {
		return nil, fmt.Errorf("no rows to render")
	}
	csvName := filepath.Base(csvPath)

	var buf bytes.Buffer
	buf.WriteString("// SPDX-License-Identifier: GPL-3.0-or-later\n\n")
	fmt.Fprintf(&buf, "// Code generated by internal/fakets480/gen from %s. DO NOT EDIT.\n\n", csvName)
	buf.WriteString("package fakets480\n\n")
	buf.WriteString("// exWidths480 is this radio's EX (MENU) inventory in compact form: ONE\n")
	buf.WriteString("// WIDTH TOKEN PER MENU NUMBER, and THE STRING'S INDEX IS THE MENU NUMBER —\n")
	buf.WriteString("// this chart runs from 000 with no gaps (480:401). A token is '1' or '2', a\n")
	buf.WriteString("// field of that many raw ASCII bytes of P5 (480:409-411). There is no text\n")
	buf.WriteString("// token: transcription B's text column is validated and deliberately not\n")
	buf.WriteString("// projected (gen/main.go's widthToken says why, and what that does and does\n")
	buf.WriteString("// not claim). ex.go expands this into the address -> default raw P5 map the\n")
	buf.WriteString("// fake answers from.\n")
	buf.WriteString("//\n")
	buf.WriteString("// THE ADDRESS IS A SINGLE COMPONENT: the chart's three-digit Menu number is\n")
	buf.WriteString("// the whole of it, and the frame's P2, P3 and P4 are printed constants, each\n")
	buf.WriteString("// \"Always 0 for the TS-480\" (480:402-407), rather than address parts.\n")
	buf.WriteString("//\n")
	buf.WriteString("// It is a PROJECTION OF TRANSCRIPTION B — this package's own copy,\n")
	fmt.Fprintf(&buf, "// %s — derived from that artefact's digits\n", csvName)
	buf.WriteString("// column alone. The codec's inventory (core/kw/ts480) is generated from\n")
	buf.WriteString("// transcription A by different code, and core/transport's cross-check proves\n")
	buf.WriteString("// the two agree — so a defect in either transcription or either generator\n")
	buf.WriteString("// shows up there.\n")
	buf.WriteString("//\n")
	fmt.Fprintf(&buf, "// %d menus, 000 to %03d. Regenerate with `go generate ./internal/fakets480`;\n", len(widths), len(widths)-1)
	buf.WriteString("// gen/main_test.go refuses a file that has drifted from the CSV.\n")
	buf.WriteString("var exWidths480 = \"\" +\n")
	for start := 0; start < len(widths); start += chunk {
		end := min(start+chunk, len(widths))
		plus := " +"
		if end == len(widths) {
			plus = ""
		}
		fmt.Fprintf(&buf, "\t%s%s // menus %03d-%03d — %s %s\n",
			strconv.Quote(widths[start:end]), plus,
			start, end-1, csvName, lineRange(rows[start].line, rows[end-1].line))
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("formatting generated Go: %w", err)
	}
	return formatted, nil
}

// lineRange renders a chunk's CSV extent, collapsing a single-row chunk to one
// line number rather than printing "lines 62-62".
func lineRange(first, last int) string {
	if first == last {
		return fmt.Sprintf("line %d", first)
	}
	return fmt.Sprintf("lines %d-%d", first, last)
}
