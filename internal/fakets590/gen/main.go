// SPDX-License-Identifier: GPL-3.0-or-later

// Command gen projects internal/fakets590's own copies of TRANSCRIPTION B
// into this fake's compact EX (MENU) inventories, emitting one generated Go
// file per registry row. It is invoked by the two //go:generate directives in
// internal/fakets590/ex.go, whose working directory is internal/fakets590 —
// hence the relative paths on the directives' flags.
//
// # STANDARD LIBRARY ONLY, and why that is the whole point
//
// This command imports nothing project-internal, and in particular NOT
// internal/extable — the machinery that generates the CODEC's inventories
// from transcription A. fakets590's recursive no-imports fence
// (imports_test.go, TestNoCoreImports) enforces that mechanically for this
// directory, and the reason is the design this file exists to serve:
//
//	the codec's inventories (core/kw/ts590/exinventory590s_gen.go and
//	exinventory590sg_gen.go) are generated from transcription A by
//	internal/extable; this fake's are generated from transcription B by the
//	code below; and core/transport's cross-check proves the two agree.
//
// A defect in either transcription, or in either generator, therefore
// surfaces as a cross-check MISMATCH. Reaching for extable here — even for
// something as innocent as its CSV row parser — would put one parser on both
// sides of that comparison, and a shared parsing bug would reproduce itself
// identically into both inventories and be invisible.
//
// So the CSV reading below is written afresh against B's OWN schema.
//
// # A NEW GENERATOR, not internal/fakeft891/gen's with the paths changed
//
// The FT-891's B is three columns (menu_number,name,digits) over a FOUR-digit
// MENU Number whose two halves are the wire address. This family's is four
// (menu_number,name,digits,text) over a THREE-digit menu number that is the
// whole address, and the three differences each remove machinery rather than
// renaming it:
//
//   - THE ADDRESS IS A SINGLE COMPONENT. All three Kenwood profiles register
//     internal/extable's AddressSingle form: the chart's three-digit Menu
//     number is P1, and P2 and P3 are printed constants of the frame rather
//     than parts of the address (590:546-553). So there is no group prefix to
//     split out, no per-group item index, and no group structure at all — the
//     inventory is one flat run of menu numbers.
//   - THE RUN IS CONTIGUOUS FROM 000, and each book prints its extent as a
//     RANGE: "000 ~ 087: Menu number (TS-590S)" and "000 ~ 099: Menu number
//     (TS-590SG)" (590:543-544). That is what lets the projection be a single
//     string whose INDEX IS THE MENU NUMBER. A gap is refused rather than
//     repaired — see projectWidths.
//   - THERE IS A text COLUMN, AND IT IS DELIBERATELY NOT PROJECTED. See
//     widthToken.
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
// silently misparsed into a plausible wrong table — and, in particular, so
// that the FT-891's three-column B could never be read by this generator as
// though it were this family's four.
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
// Menu number, "000 ~ 087: Menu number (TS-590S)" / "000 ~ 099: Menu number
// (TS-590SG)" as the EX block prints it (590:543-544). It is the WHOLE
// address on this family — P2 and P3 are printed constants of the frame, not
// address components (590:546-550).
const menuNumberDigits = 3

// maxWidth is the widest raw P5 field either 590 chart declares: 8. It comes
// from the one free-text row each list prints — "Power on message ... up to 8
// ASCII characters", menu 087 on the S and the same string renumbered to 001
// on the SG (590:741, 590:750) — and it is also the widest P5 the printed
// frame grid itself has room for, which reaches position 17 before the ';'
// (590:545-547).
//
// It is spelt here as the number B's own digits column prints for those rows.
// The independent check on the same fact is the codec's side: core/kw's
// MaxEXDigits bounds every profile, and core/kw/ts590/crosscheck_test.go
// compares A's digits against B's row for row.
const maxWidth = 8

// row is one parsed B data row, reduced to what the projection needs.
type row struct {
	// menu is the three-digit Menu number, as a number: the whole wire
	// address.
	menu int
	// token is the width token: '1'..'8' for a numeric field of that many
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
	log.SetPrefix("fakets590/gen: ")

	csvPath := flag.String("csv", "", "path to this package's copy of transcription B (required)")
	outPath := flag.String("out", "", "path of the generated Go file to write (required)")
	varName := flag.String("var", "", "name of the generated package-level widths variable (required)")
	flag.Parse()

	// No positional operands, and no defaulted paths. Silently ignoring an
	// operand — or defaulting a path — would let a mistyped invocation read
	// as a successful run that generated something other than what was asked
	// for. It matters more here than in the FT-891's single-row generator:
	// this one is invoked TWICE over two charts of one family, and a
	// defaulted -csv or -out would quietly generate one row's table into the
	// other row's file.
	if flag.NArg() > 0 {
		log.Fatalf("unexpected positional arguments %v — gen takes only -csv, -out and -var", flag.Args())
	}
	if *csvPath == "" || *outPath == "" || *varName == "" {
		log.Fatal("-csv, -out and -var are all required; see the //go:generate directives in ex.go")
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
	out, err := render(widths, rows, *csvPath, *varName)
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

	// The RECORD's starting line, taken from the reader itself rather than
	// counted: this family's charts carry names with embedded commas
	// ("CW keying dot, dash weight ratio" on the 480), and a quoted field may
	// span lines, so record index + 2 is not the physical line number.
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
// It exists because encoding/csv does not report it and because this family's
// charts make the obvious arithmetic wrong: the TS-480's menu 035 is "CW
// keying dot, dash weight ratio", a QUOTED cell carrying a comma, and a quoted
// cell may equally carry a newline. Counting quote parity is the whole of the
// job — a doubled quote inside a quoted field flips the state twice and so
// leaves it unchanged, which is the correct reading.
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
// B carries a fourth column the FT-891's does not, flagging the rows its
// transcriber read as text. This projection reads it only to REFUSE a cell
// that is neither "0" nor "1" — a value outside the schema is a misparsed row
// — and emits the same numeric token either way, so every address this fake
// answers replies with its width in '0' bytes.
//
// That is not the generator declining to model a distinction. It is the
// orchestrator's RULING, applied: core/kw/ts590/crosscheck_test.go and
// core/kw/ts480/crosscheck_test.go both record that the text flag is a
// CONVENTION and the digits are the datum, because A and B were briefed with
// different definitions of "text row" and each applied its own consistently.
// The two legs disagree at six addresses across the three charts — the
// TS-590SG's menu 000 (Version information, which A transcribes as a
// fixed-width numeric row) and the TS-480's menus 048-052 (whose merged cell
// prints the numeric legend "00 ~ 99 (2-digit)") — and in every one of the
// six the repository's ruling is that the row is NOT text.
//
// A fake that projected B's flag would therefore answer SPACES at six
// addresses the repository has ruled numeric, on the authority of a column its
// own cross-check declines to compare. Projecting the width alone is what
// keeps this side's claim exactly as strong as the evidence the two legs
// share. The two genuine free-text rows both legs agree on — menu 087 on the S
// and 001 on the SG — are answered as their width in '0' bytes like every
// other row, and core/transport's cross-check names them as the deliberate
// state rather than leaving it to a reader to notice.
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
		return 0, fmt.Errorf("digits %d is outside 1-%d: the compact inventory has no token for it, and %d is the widest field either 590 chart prints (590:741, 590:750)", n, maxWidth, maxWidth)
	}
	return byte('0' + n), nil
}

// parseMenuNumber parses one of B's address cells — "087" — into the menu
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
// It is a refusal rather than a repair. Each book prints its menu domain as a
// RANGE — "000 ~ 087" and "000 ~ 099" (590:543-544) — so a gap in a
// transcription of it is either a transcription defect or a chart this compact
// form cannot model, and both are findings. A projection that silently indexed
// past a gap would renumber every menu after it, which is the one error this
// table could make that no width comparison would catch.
func projectWidths(rows []row) (string, error) {
	var b strings.Builder
	for i, r := range rows {
		if r.menu != i {
			return "", fmt.Errorf("line %d: menu %03d appears at position %d — this chart's menu numbers run from 000 with no gaps (590:543-544), and the compact inventory's index IS the menu number", r.line, r.menu, i)
		}
		b.WriteByte(r.token)
	}
	return b.String(), nil
}

// chunk is how many width tokens one emitted string line carries. TEN, so that
// a line's first menu number is a round hundred-plus-ten and a reader can find
// a given menu by counting lines rather than characters.
const chunk = 10

// render emits the generated Go file. csvPath names the source; only its BASE
// NAME is written into the output, so where the generator was invoked from
// cannot leak into the committed bytes — which is what lets gen's own
// staleness test read the CSV as "../transcription-b-590s.csv" and still
// render the file the //go:generate directive produces from
// "transcription-b-590s.csv".
//
// The output is DETERMINISTIC: the widths string comes from the parsed rows in
// the file order projectWidths validated, nothing is ranged over a map, and
// the whole buffer is run through go/format — so two runs over equal input
// produce byte-identical, gofmt-clean output. That is what makes the staleness
// test's byte comparison (gen/main_test.go) a meaningful check rather than a
// formatting lottery.
func render(widths string, rows []row, csvPath, varName string) ([]byte, error) {
	if widths == "" {
		return nil, fmt.Errorf("no rows to render")
	}
	csvName := filepath.Base(csvPath)

	var buf bytes.Buffer
	buf.WriteString("// SPDX-License-Identifier: GPL-3.0-or-later\n\n")
	fmt.Fprintf(&buf, "// Code generated by internal/fakets590/gen from %s. DO NOT EDIT.\n\n", csvName)
	buf.WriteString("package fakets590\n\n")
	fmt.Fprintf(&buf, "// %s is this row's EX (MENU) inventory in compact form: ONE WIDTH\n", varName)
	buf.WriteString("// TOKEN PER MENU NUMBER, and THE STRING'S INDEX IS THE MENU NUMBER — this\n")
	buf.WriteString("// chart runs from 000 with no gaps (590:543-544). A token is '1'..'8', a\n")
	buf.WriteString("// field of that many raw ASCII bytes of P5. There is no text token:\n")
	buf.WriteString("// transcription B's text column is validated and deliberately not projected\n")
	buf.WriteString("// (gen/main.go's widthToken says why, and what that does and does not\n")
	buf.WriteString("// claim). ex.go expands this into the address -> default raw P5 map the\n")
	buf.WriteString("// fake answers from.\n")
	buf.WriteString("//\n")
	buf.WriteString("// THE ADDRESS IS A SINGLE COMPONENT: the chart's three-digit Menu number is\n")
	buf.WriteString("// the whole of it, and the frame's P2, P3 and P4 are printed constants\n")
	buf.WriteString("// (590:546-553) rather than address parts.\n")
	buf.WriteString("//\n")
	buf.WriteString("// It is a PROJECTION OF TRANSCRIPTION B — this package's own copy,\n")
	fmt.Fprintf(&buf, "// %s — derived from that artefact's digits\n", csvName)
	buf.WriteString("// column alone. The codec's inventory for this row\n")
	buf.WriteString("// (core/kw/ts590) is generated from transcription A by different code, and\n")
	buf.WriteString("// core/transport's cross-check proves the two agree — so a defect in either\n")
	buf.WriteString("// transcription or either generator shows up there.\n")
	buf.WriteString("//\n")
	fmt.Fprintf(&buf, "// %d menus, 000 to %03d. Regenerate with `go generate ./internal/fakets590`;\n", len(widths), len(widths)-1)
	buf.WriteString("// gen/main_test.go refuses a file that has drifted from the CSV.\n")
	fmt.Fprintf(&buf, "var %s = \"\" +\n", varName)
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
// line number rather than printing "lines 90-90".
func lineRange(first, last int) string {
	if first == last {
		return fmt.Sprintf("line %d", first)
	}
	return fmt.Sprintf("lines %d-%d", first, last)
}
