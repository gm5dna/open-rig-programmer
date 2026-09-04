// SPDX-License-Identifier: GPL-3.0-or-later

// Command gen projects internal/fakeft891's own copy of TRANSCRIPTION B into
// the fake's compact EX (MENU) inventory, emitting exinventory_gen.go. It is
// invoked by the //go:generate directive in internal/fakeft891/ex.go, whose
// working directory is internal/fakeft891 — hence the relative paths on the
// directive's flags.
//
// # STANDARD LIBRARY ONLY, and why that is the whole point
//
// This command imports nothing project-internal, and in particular NOT
// internal/extable — the machinery that generates the DIALECT's inventory from
// transcription A. fakeft891's recursive no-imports fence (imports_test.go,
// TestNoCoreImports) enforces that mechanically for this directory, and the
// reason is the design this file exists to serve:
//
//	the dialect's inventory is generated from transcription A by
//	internal/extable; this fake's is generated from transcription B by the
//	code below; and core/transport's cross-check proves the two agree.
//
// A defect in either transcription, or in either generator, therefore surfaces
// as a cross-check MISMATCH. Reaching for extable here — even for something as
// innocent as its CSV row parser — would put one parser on both sides of that
// comparison, and a shared parsing bug would reproduce itself identically into
// both inventories and be invisible. That is exactly the failure mode the
// FT-710's own Digits-column misreading (spec REVISION 3, D-baud) is a
// reminder of.
//
// So the CSV reading below is written afresh against B's OWN schema.
//
// # A NEW GENERATOR, not internal/fakedx10/gen's with the paths changed
//
// The FTdx10's B is six columns (P1,P2,P3,Function,P4,Digits) with the group
// labels wrapped inside the address cells. The FT-891's is three
// (menu_number,name,digits), and the three differences are structural
// properties of the printed chart, each of which removes machinery rather than
// renaming it:
//
//   - THE ADDRESS IS A PAIR. This chart prints a four-digit MENU Number whose
//     two halves ARE the address — 0803 is (P1,P2) = (08,03) — and every row's
//     P3 is zero (core/cat's EXAddressPair). So a GROUP here is one two-digit
//     P1 prefix and its items are indexed by P2; the FTdx10's (P1,P2) subgroup
//     key with a P3 item index has no counterpart.
//   - THERE ARE NO GROUP LABELS. The chart prints no label columns at all, so
//     there is no "NN (LABEL)" wrapper to split, nothing to check for
//     agreement across a group's rows, and nothing to emit as a comment.
//   - THERE IS NO TEXT ROW, AND NO COLUMN THAT COULD IDENTIFY ONE. B carries
//     no parameter-legend column, so the FTdx10 generator's text discriminator
//     — Digits == 12 AND a P4 cell beginning "Up to" — is not merely
//     unnecessary, it is INEXPRESSIBLE. Every row is therefore projected as
//     numeric. That is a statement about the delivered SCHEMA and not a claim
//     about the radio: see widthToken.
//
// # What is projected, and what is deliberately dropped
//
// The output models WIRE BEHAVIOUR ONLY — how many P2 items each P1 group has,
// and each item's raw P4 reply WIDTH. B's name column (the item's human name)
// is NOT emitted: this fake answers menu reads, it does not interpret menu
// meanings, and the dialect is the layer that carries names. The name is read
// only so that a malformed row can be NAMED in an error message, and so that a
// blank cell — the signature of a misparsed row — is refused rather than
// silently projected.
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
// that the FTdx10's six-column B could never be read by this generator as
// though it were the FT-891's three.
var bHeader = []string{"menu_number", "name", "digits"}

// Column indices into a B record.
const (
	colMenuNumber = iota
	colName
	colDigits
	numCols
)

// menuNumberDigits is the width of B's address cell: the chart's four-digit
// MENU Number, "P1 : 0101 - 1803 (MENU Number)" as the EX block prints it.
// The first two digits are P1, the last two P2, and there is no third
// component — which is what makes this radio's EX read frame seven bytes where
// every registered sibling's is nine.
const menuNumberDigits = 4

// maxWidth is the widest raw P4 field this chart declares: 5. It comes from
// exactly two rows (0803 OTHER DISP, 0804 OTHER SHIFT), whose signed
// "-3000 Hz - 0 - +3000 Hz" parameter counts its sign as a digit. It is spelt
// here as the number B's own digits column prints for those rows, and pinned
// independently from both sides — gen/main_test.go's
// TestParseB_TheOnlyFiveWideRowsAre0803And0804 from B, and
// core/cat/ft891/crosscheck_test.go's widestRowDigits from A.
const maxWidth = 5

// row is one parsed B data row, reduced to what the projection needs.
type row struct {
	// p1 is the group component, the menu number's first two digits, kept as
	// the two ASCII bytes the wire address is built from.
	p1 string
	// p2 is the item index within the group, 1-based: the menu number's last
	// two digits, as a number.
	p2 int
	// token is the width token: '1'..'5' for a numeric field of that many
	// bytes. There is no text token — see widthToken.
	token byte
	// line is the 1-based physical line in the CSV, header included, for
	// error messages and for the emitted provenance comment.
	line int
}

// group is one P1 group's projection: a widths string with one token per P2
// item, in P2 order starting at 01.
type group struct {
	p1        string
	widths    string
	firstLine int
	lastLine  int
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("fakeft891/gen: ")

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
	groups, err := groupRows(rows)
	if err != nil {
		log.Fatalf("grouping %s: %v", *csvPath, err)
	}
	out, err := render(groups, *csvPath)
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

	out := make([]row, 0, len(records)-1)
	for i, rec := range records[1:] {
		line := i + 2 // 1-based, header included
		p1, p2, err := splitMenuNumber(rec[colMenuNumber])
		if err != nil {
			return nil, fmt.Errorf("line %d: menu_number cell: %w", line, err)
		}
		if strings.TrimSpace(rec[colName]) == "" {
			return nil, fmt.Errorf("line %d (%s): empty name cell — a blank name is the signature of a misparsed row, not an item without a name", line, rec[colMenuNumber])
		}
		token, err := widthToken(rec[colDigits])
		if err != nil {
			return nil, fmt.Errorf("line %d (%s %s): %w", line, rec[colMenuNumber], rec[colName], err)
		}
		out = append(out, row{p1: p1, p2: p2, token: token, line: line})
	}
	return out, nil
}

// widthToken derives one width token from B's digits cell.
//
// A width of 1..maxWidth is a numeric field of that many raw ASCII bytes and
// yields that digit itself. EVERY OTHER VALUE IS REFUSED, including 12 — and
// the refusal of 12 is the one worth stating, because it is where this
// generator deliberately declines to do what its FTdx10 sibling does.
//
// That generator reads a 12 as the text item (MY CALL.) when B's P4 cell also
// describes a character count, and emits a 'T' token which the fake expands to
// twelve SPACES rather than twelve zeros. THE FT-891'S B HAS NO SUCH CELL:
// three columns, no parameter legend, no text flag. So there is nothing here
// from which textness could be decided, and deciding it on the width alone
// would invent wire behaviour — a hypothetical twelve-digit NUMERIC item would
// answer twelve zeros, and answering spaces for it would be a fabrication that
// no test in this package could see, because both sides of every such test
// would come from this one projection.
//
// Refusing is therefore the honest answer AND the one that fails loudly: if
// this chart ever turns out to carry a text row, the generator stops rather
// than guessing, and the arbitration is against the PDF. The independent check
// on the whole question is the cross-check, whose dialect side comes from
// transcription A — which does carry a text column.
func widthToken(digits string) (byte, error) {
	n, err := strconv.Atoi(strings.TrimSpace(digits))
	if err != nil {
		return 0, fmt.Errorf("digits cell %q is not a number: %w", digits, err)
	}
	if n < 1 || n > maxWidth {
		return 0, fmt.Errorf("digits %d is outside 1-%d: the compact inventory has no token for it, and this three-column schema carries nothing from which a wider or a text field could be described", n, maxWidth)
	}
	return byte('0' + n), nil
}

// splitMenuNumber splits one of B's address cells — "0803" — into its
// two-digit P1 group prefix and its P2 item index.
//
// It refuses anything that is not exactly four ASCII digits, so a cell of a
// different shape cannot be silently reduced to a plausible address. Two
// mistakes it is aimed at in particular: a three-digit cell (a lost leading
// zero, which would shift every component), and a six-digit one (a sibling
// radio's triple address, which would parse as a plausible pair with two
// digits quietly discarded).
func splitMenuNumber(cell string) (p1 string, p2 int, err error) {
	s := strings.TrimSpace(cell)
	if len(s) != menuNumberDigits {
		return "", 0, fmt.Errorf("%q is not exactly four ASCII digits", cell)
	}
	for i := 0; i < len(s); i++ {
		if !isDigit(s[i]) {
			return "", 0, fmt.Errorf("%q is not exactly four ASCII digits", cell)
		}
	}
	return s[:2], int(s[2]-'0')*10 + int(s[3]-'0'), nil
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// groupRows folds rows into one group per P1, in file order, and enforces
// every structural property the compact widths-string form depends on:
//
//   - a group's rows form ONE CONTIGUOUS BLOCK. A P1 reappearing after another
//     group intervened would mean the file interleaves groups, which the chart
//     does not, and which the widths string cannot express.
//   - P2 runs 01, 02, 03 … with no gaps. The string's index IS the item index,
//     so a gap would silently renumber every item after it.
//
// Each is a refusal rather than a repair. A gap in a transcription of a ruled
// chart is either a transcription defect or a chart the compact form cannot
// model, and both are findings.
func groupRows(rows []row) ([]group, error) {
	var groups []group
	seen := map[string]int{} // p1 -> index in groups

	for _, r := range rows {
		idx, ok := seen[r.p1]
		if !ok {
			if r.p2 != 1 {
				return nil, fmt.Errorf("line %d: group %s opens at P2=%02d, want 01", r.line, r.p1, r.p2)
			}
			groups = append(groups, group{
				p1: r.p1, widths: string(r.token), firstLine: r.line, lastLine: r.line,
			})
			seen[r.p1] = len(groups) - 1
			continue
		}
		if idx != len(groups)-1 {
			return nil, fmt.Errorf("line %d: group %s resumes after group %s intervened — its rows are not one contiguous block", r.line, r.p1, groups[len(groups)-1].p1)
		}
		g := &groups[idx]
		if want := len(g.widths) + 1; r.p2 != want {
			return nil, fmt.Errorf("line %d: group %s item %d has P2=%02d, want %02d — P2 must run consecutively from 01", r.line, r.p1, want, r.p2, want)
		}
		g.widths += string(r.token)
		g.lastLine = r.line
	}
	return groups, nil
}

// render emits the generated Go file. csvPath names the source; only its BASE
// NAME is written into the output, so where the generator was invoked from
// cannot leak into the committed bytes — which is what lets gen's own staleness
// test read the CSV as "../transcription-b.csv" and still render the file the
// //go:generate directive produces from "transcription-b.csv".
//
// The output is DETERMINISTIC: groups are emitted in the file order groupRows
// validated (which the chart fixes, and which is P1-ascending in the committed
// artefact), every value derives from the parsed rows, nothing is ranged over a
// map, and the whole buffer is run through go/format — so two runs over equal
// input produce byte-identical, gofmt-clean output. That is what makes the
// staleness test's byte comparison (gen/main_test.go) a meaningful check rather
// than a formatting lottery.
func render(groups []group, csvPath string) ([]byte, error) {
	if len(groups) == 0 {
		return nil, fmt.Errorf("no groups to render")
	}
	csvName := filepath.Base(csvPath)
	items := 0
	for _, g := range groups {
		items += len(g.widths)
	}

	var buf bytes.Buffer
	buf.WriteString("// SPDX-License-Identifier: GPL-3.0-or-later\n\n")
	fmt.Fprintf(&buf, "// Code generated by internal/fakeft891/gen from %s. DO NOT EDIT.\n\n", csvName)
	buf.WriteString("package fakeft891\n\n")
	buf.WriteString("// exGroups is this fake's EX (MENU) inventory in compact form: one entry per\n")
	buf.WriteString("// P1 group, in the chart's own order, with one width token per P2 item in P2\n")
	buf.WriteString("// order starting at 01. A token is '1'..'5' — a numeric field of that many raw\n")
	buf.WriteString("// ASCII bytes. There is NO text token: this chart's transcription carries no\n")
	buf.WriteString("// column from which a text item could be identified, so every row is projected\n")
	buf.WriteString("// as numeric (gen/main.go's widthToken says what that does and does not claim).\n")
	buf.WriteString("// ex.go expands this into the address -> default raw P4 map the fake answers\n")
	buf.WriteString("// from, and states what the table does and does not claim.\n")
	buf.WriteString("//\n")
	buf.WriteString("// THE ADDRESS IS A PAIR: the chart's four-digit MENU Number is P1 then P2, and\n")
	buf.WriteString("// every item's P3 is zero (core/cat's EXAddressPair), which is why this radio's\n")
	buf.WriteString("// EX read frame is seven bytes where every registered sibling's is nine.\n")
	buf.WriteString("//\n")
	fmt.Fprintf(&buf, "// It is a PROJECTION OF TRANSCRIPTION B (%s), derived from that\n", csvName)
	buf.WriteString("// artefact's digits column alone. The dialect's inventory\n")
	buf.WriteString("// (core/cat/ft891/exinventory_gen.go) is generated from transcription A by\n")
	buf.WriteString("// different code, and core/transport's cross-check proves the two agree — so a\n")
	buf.WriteString("// defect in either transcription or either generator shows up there.\n")
	buf.WriteString("//\n")
	fmt.Fprintf(&buf, "// %s, %s. Regenerate with `go generate ./internal/fakeft891`;\n", plural(len(groups), "group"), plural(items, "item"))
	buf.WriteString("// gen/main_test.go refuses a file that has drifted from the CSV.\n")
	buf.WriteString("var exGroups = []struct{ p1, widths string }{\n")
	for _, g := range groups {
		fmt.Fprintf(&buf, "\t{%s, %s}, // %s — %s %s\n",
			strconv.Quote(g.p1), strconv.Quote(g.widths),
			plural(len(g.widths), "item"), csvName, lineRange(g.firstLine, g.lastLine))
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

// lineRange renders a group's CSV extent, collapsing a single-row group to one
// line number rather than printing "lines 130-130". The FT-891's chart has such
// a group — 17 RESET, one item — so this branch is exercised by the committed
// artefact rather than only by a hypothetical.
func lineRange(first, last int) string {
	if first == last {
		return fmt.Sprintf("line %d", first)
	}
	return fmt.Sprintf("lines %d-%d", first, last)
}
