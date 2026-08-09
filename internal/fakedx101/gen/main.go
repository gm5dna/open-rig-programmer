// SPDX-License-Identifier: GPL-3.0-or-later

// Command gen projects internal/fakedx101's own copy of TRANSCRIPTION B into
// the fake's compact EX (MENU) inventory, emitting exinventory_gen.go. It is
// invoked by the //go:generate directive in internal/fakedx101/ex.go, whose
// working directory is internal/fakedx101 — hence the relative paths on the
// directive's flags.
//
// # STANDARD LIBRARY ONLY, and why that is the whole point
//
// This command imports nothing project-internal, and in particular NOT
// internal/extable — the machinery that generates the DIALECT's inventory from
// transcription A. fakedx101's recursive no-imports fence (imports_test.go,
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
// both inventories and be invisible. extable's Digits parsing is a KNOWN DEFECT
// LOCUS besides: the FT-710's own Digits-column misreading (spec REVISION 3,
// D-baud) came from exactly that column, which is the column this projection
// rests on entirely.
//
// So the CSV reading below is written afresh against B's OWN schema. It is not
// a copy of extable's parser: it reads a different, eight-column shape, it emits
// a widths table rather than a []cat.EXItem, and it carries none of extable's
// profile/registry/observation machinery.
//
// # B's SCHEMA HERE IS NOT B's SCHEMA FOR THE FTdx10
//
// internal/fakedx10/gen is this file's structural exemplar, and the ONE place
// this file departs from it is the parse, because the two artefacts are not the
// same shape. That package's B lost its briefed header to a mid-task
// stall/resume and was accepted verbatim as six columns —
// `P1,P2,P3,Function,P4,Digits` — with the group labels still WRAPPED
// ("01 (RADIO SETTING)") and no text flag, so its generator has to strip the
// wrapper and RECONSTRUCT the text flag from a "Up to 12 characters" prefix in
// the value-legend column.
//
// This B arrived on its briefed schema:
//
//	p1,p2,p3,p1_label,p2_label,name,digits,text
//
// Bare labels, and an explicit boolean `text` column that is the quarantined
// agent's OWN reading of the printed P4 cell (its pass 3 was devoted to exactly
// that question). So there is no wrapper to strip and no flag to reconstruct
// here, and this file does neither: adopting fakedx10's adaptations against a
// transcription that does not need them would be inventing a schema. The
// dialect's own cross-check records the same adjudication from the other side
// (core/cat/ftdx101/crosscheck_test.go, adjudication (a)).
//
// What survives from the exemplar unchanged is everything that is about the
// PROJECTION rather than about the CSV: the -csv/-out flags with no defaults,
// the pinned header, the 12-byte text width, the refuse-never-repair
// discipline, the contiguity and label-agreement checks, base-name-only
// rendering, and go/format on the way out.
//
// # What is projected, and what is deliberately dropped
//
// The output models WIRE BEHAVIOUR ONLY — how many P3 items each (P1,P2)
// subgroup has, and each item's raw P4 reply WIDTH. B's name column (the item's
// human name) is NOT emitted: this fake answers menu reads, it does not
// interpret menu meanings, and the dialect is the layer that carries names. The
// two label columns reach the output only as a comment on each row.
package main

import (
	"bytes"
	"encoding/csv"
	"flag"
	"fmt"
	"go/format"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// bHeader is transcription B's exact header row, as delivered and committed —
// the eight columns its brief asked for. It is pinned so that a schema change
// fails LOUDLY here rather than being silently misparsed into a plausible wrong
// table, and so that the FTdx10's six-column B (or any other file) cannot be
// fed to this generator by a mistyped -csv path.
var bHeader = []string{"p1", "p2", "p3", "p1_label", "p2_label", "name", "digits", "text"}

// Column indices into a B record.
const (
	colP1 = iota
	colP2
	colP3
	colP1Label
	colP2Label
	colName
	colDigits
	colText
	numCols
)

// csvComment is the byte that opens a comment line in B. The artefact begins
// with a long provenance block — source document, printed revision code, chart
// pages, the four-pass raster method, the verbatim policy — and every line of it
// is a '#' comment. Skipping it here is what core/cat/ftdx101/crosscheck_test.go
// and internal/extable both do with the same files.
const csvComment = '#'

// textWidth is the wire width of B's one text item's P4 field, and the width
// that selects the 'T' token. It is spelt here as the number B's digits column
// prints for that row (04,01,01 MY CALL.).
const textWidth = 12

// The two spellings B's boolean text column uses. Nothing else is accepted: a
// cell reading "TRUE", "1" or "yes" means the artefact's own convention changed,
// which is a finding rather than a value to normalise.
const (
	textTrue  = "true"
	textFalse = "false"
)

// row is one parsed B data row, reduced to what the projection needs.
type row struct {
	// p1, p2 are the wire components, two digits each, exactly as B prints
	// them.
	p1, p2 string
	// p1Label, p2Label are B's BARE group labels, verbatim. Emitted only as a
	// comment.
	p1Label, p2Label string
	// p3 is the item index, 1-based.
	p3 int
	// token is the width token: '1'..'4' for a numeric field of that many
	// bytes, or 'T' for the 12-byte text field.
	token byte
	// line is the 1-based physical line in the CSV, comment block and header
	// included, for error messages and for the emitted provenance comment.
	line int
}

// group is one (P1,P2) subgroup's projection: a widths string with one token
// per P3 item, in P3 order starting at 01.
type group struct {
	p1, p2           string
	p1Label, p2Label string
	widths           string
	firstLine        int
	lastLine         int
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("fakedx101/gen: ")

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

// parseB parses transcription B into rows, in file order. Every malformed input
// is an error rather than a skipped row: this CSV is a committed evidential
// artefact, so anything the projection cannot read is a finding to report, never
// a row to drop.
//
// Line numbers in errors are the CSV reader's own, so they count the '#'
// provenance block: they are the numbers a reader opening the file in an editor
// will see, which is the only thing they are for.
func parseB(data []byte) ([]row, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.Comment = csvComment
	r.FieldsPerRecord = numCols // the header sets it; this states it up front

	// Records are read ONE AT A TIME rather than with ReadAll, for one reason:
	// the reader collapses the '#' provenance block, so a record's index is no
	// longer its line, and only csv.Reader.FieldPos knows where each record
	// actually began. Re-deriving line numbers by arithmetic over a file with a
	// variable-length comment block is precisely the kind of plausible-but-wrong
	// number this generator refuses to invent elsewhere.
	header, err := r.Read()
	if err == io.EOF {
		return nil, fmt.Errorf("empty CSV: no header row (a file of nothing but '%c' comments reads the same way)", csvComment)
	}
	if err != nil {
		return nil, err
	}
	if !slices.Equal(header, bHeader) {
		return nil, fmt.Errorf("header row is %v, want %v — transcription B's schema changed, or the wrong file was read (the FTdx10's B is a DIFFERENT six-column shape and must not be projected by this generator)", header, bHeader)
	}

	var out []row
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		line, _ := r.FieldPos(0)

		p1, err := parseTwoDigit(rec[colP1])
		if err != nil {
			return nil, fmt.Errorf("line %d: p1 cell: %w", line, err)
		}
		p2, err := parseTwoDigit(rec[colP2])
		if err != nil {
			return nil, fmt.Errorf("line %d: p2 cell: %w", line, err)
		}
		p3, err := parseTwoDigit(rec[colP3])
		if err != nil {
			return nil, fmt.Errorf("line %d: p3 cell: %w", line, err)
		}
		p1Label, err := bareLabel(rec[colP1Label])
		if err != nil {
			return nil, fmt.Errorf("line %d: p1_label cell: %w", line, err)
		}
		p2Label, err := bareLabel(rec[colP2Label])
		if err != nil {
			return nil, fmt.Errorf("line %d: p2_label cell: %w", line, err)
		}
		token, err := widthToken(rec[colDigits], rec[colText])
		if err != nil {
			return nil, fmt.Errorf("line %d (%s %s / %s): %w", line, rec[colP1], rec[colP2], rec[colName], err)
		}
		out = append(out, row{
			p1: twoDigits(p1), p2: twoDigits(p2), p1Label: p1Label, p2Label: p2Label,
			p3: p3, token: token, line: line,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("CSV has a header and no data rows")
	}
	return out, nil
}

// widthToken derives one width token from B's digits cell, with B's own text
// column as the discriminator.
//
// Digits 1-4 is a numeric field of that many bytes and yields that digit itself,
// and its text flag must be false. Digits 12 with text true yields 'T' — the
// 12-byte text field. THE TWO CELLS MUST AGREE: a 12-digit NUMERIC item would
// answer twelve zeros rather than twelve spaces, and a text item of some other
// width would answer the wrong number of spaces, so a disagreement between B's
// own two cells is arbitrated against the PDF rather than resolved here by
// preferring one of them. Every other digits value is refused: the compact form
// has no token for it, and guessing one would be this generator inventing wire
// behaviour the transcription does not describe.
//
// This is where the FTdx10's generator and this one genuinely differ. Its B has
// no text column at all, so its widthToken has to READ A VALUE LEGEND ("Up to 12
// characters") to decide. Here the flag is transcribed, so the check is that the
// two transcribed cells are consistent — a stronger position, and B's own
// (core/cat/ftdx101/crosscheck_test.go's adjudication (a) reaches it too).
func widthToken(digits, text string) (byte, error) {
	isText, err := parseTextFlag(text)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(digits))
	if err != nil {
		return 0, fmt.Errorf("digits cell %q is not a number: %w", digits, err)
	}
	switch {
	case n >= 1 && n <= 4:
		if isText {
			return 0, fmt.Errorf("digits %d but text is %q — B's width and its own text flag disagree; arbitrate against the PDF, do not guess", n, textTrue)
		}
		return byte('0' + n), nil
	case n == textWidth:
		if !isText {
			return 0, fmt.Errorf("digits %d but text is %q — B's width and its own text flag disagree; arbitrate against the PDF, do not guess", n, textFalse)
		}
		return 'T', nil
	default:
		return 0, fmt.Errorf("digits %d is neither 1-4 (numeric) nor %d (text): the compact inventory has no token for it", n, textWidth)
	}
}

// parseTextFlag reads B's boolean text cell strictly. strconv.ParseBool is
// deliberately NOT used: it accepts "1", "T", "TRUE" and five more spellings,
// and this artefact prints exactly two. Widening the vocabulary here would let a
// re-transcribed file quietly change convention without anything noticing.
func parseTextFlag(cell string) (bool, error) {
	switch strings.TrimSpace(cell) {
	case textTrue:
		return true, nil
	case textFalse:
		return false, nil
	default:
		return false, fmt.Errorf("text cell %q is neither %q nor %q", cell, textTrue, textFalse)
	}
}

// bareLabel validates one of B's group-label cells and returns it verbatim.
//
// B records the BARE label ("RADIO SETTING"), not the chart's printed
// parenthesised form ("01 (RADIO SETTING)"): stripping the numbered wrapper is
// the convention A's header states, the ledger deliberately does NOT follow, and
// core/cat/ftdx101/crosscheck_test.go binds by composition. So a cell carrying a
// parenthesis is not a label with decoration — it is a cell on a DIFFERENT
// convention, most plausibly the FTdx10's wrapped shape, and reducing it to a
// plausible bare label here would silently project one chart's typography onto
// another's data.
func bareLabel(cell string) (string, error) {
	s := strings.TrimSpace(cell)
	if s == "" {
		return "", fmt.Errorf("%q is empty", cell)
	}
	if strings.ContainsAny(s, "()") {
		return "", fmt.Errorf("%q carries a parenthesis — B records BARE group labels, so a wrapped cell means the artefact's convention changed (or the FTdx10's differently-shaped B was read)", cell)
	}
	return s, nil
}

// parseTwoDigit accepts exactly two ASCII digits and returns their value. The
// EX wire address is a fixed six-digit field of three two-digit components, so a
// one- or three-digit cell is a schema error, not a value to normalise.
func parseTwoDigit(s string) (int, error) {
	if len(s) != 2 || !isDigit(s[0]) || !isDigit(s[1]) {
		return 0, fmt.Errorf("%q is not exactly two ASCII digits", s)
	}
	return int(s[0]-'0')*10 + int(s[1]-'0'), nil
}

// twoDigits renders a validated component back to its two-digit wire spelling.
// It exists so that the wire strings in the output come from the PARSED value
// rather than from the CSV cell: the two are equal by construction here, and
// routing them through the parse is what keeps them so if the cell's shape ever
// widens.
func twoDigits(n int) string { return string([]byte{byte('0' + n/10), byte('0' + n%10)}) }

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// groupRows folds rows into one group per (P1,P2), in file order, and enforces
// every structural property the compact widths-string form depends on:
//
//   - a group's rows form ONE CONTIGUOUS BLOCK. A group key reappearing after
//     another group intervened would mean the file interleaves subgroups, which
//     the chart does not, and which the widths string cannot express.
//   - P3 runs 01, 02, 03 … with no gaps. The string's index IS the item index,
//     so a gap would silently renumber every item after it.
//   - a group's label cells agree across its rows.
//
// Each is a refusal rather than a repair. A gap in a transcription of a ruled
// chart is either a transcription defect or a chart the compact form cannot
// model, and both are findings.
func groupRows(rows []row) ([]group, error) {
	var groups []group
	seen := map[[2]string]int{} // (p1,p2) -> index in groups

	for _, r := range rows {
		key := [2]string{r.p1, r.p2}
		idx, ok := seen[key]
		if !ok {
			if r.p3 != 1 {
				return nil, fmt.Errorf("line %d: group (%s,%s) opens at p3=%02d, want 01", r.line, r.p1, r.p2, r.p3)
			}
			groups = append(groups, group{
				p1: r.p1, p2: r.p2, p1Label: r.p1Label, p2Label: r.p2Label,
				widths: string(r.token), firstLine: r.line, lastLine: r.line,
			})
			seen[key] = len(groups) - 1
			continue
		}
		if idx != len(groups)-1 {
			return nil, fmt.Errorf("line %d: group (%s,%s) resumes after group (%s,%s) intervened — its rows are not one contiguous block", r.line, r.p1, r.p2, groups[len(groups)-1].p1, groups[len(groups)-1].p2)
		}
		g := &groups[idx]
		if want := len(g.widths) + 1; r.p3 != want {
			return nil, fmt.Errorf("line %d: group (%s,%s) item %d has p3=%02d, want %02d — p3 must run consecutively from 01", r.line, r.p1, r.p2, want, r.p3, want)
		}
		if r.p1Label != g.p1Label || r.p2Label != g.p2Label {
			return nil, fmt.Errorf("line %d: group (%s,%s) label cells disagree: %q/%q here, %q/%q at line %d", r.line, r.p1, r.p2, r.p1Label, r.p2Label, g.p1Label, g.p2Label, g.firstLine)
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
// validated (which the chart fixes, and which is (P1,P2)-ascending in the
// committed artefact), every value derives from the parsed rows, nothing is
// ranged over a map, and the whole buffer is run through go/format — so two runs
// over equal input produce byte-identical, gofmt-clean output. That is what makes
// the staleness test's byte comparison (gen/main_test.go) a meaningful check
// rather than a formatting lottery.
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
	fmt.Fprintf(&buf, "// Code generated by internal/fakedx101/gen from %s. DO NOT EDIT.\n\n", csvName)
	buf.WriteString("package fakedx101\n\n")
	buf.WriteString("// exGroups is this fake's EX (MENU) inventory in compact form: one entry per\n")
	buf.WriteString("// (P1,P2) subgroup, in the chart's own order, with one width token per P3 item\n")
	buf.WriteString("// in P3 order starting at 01. A token is '1'..'4' — a numeric field of that many\n")
	buf.WriteString("// raw ASCII bytes — or 'T', the 12-byte text field. ex.go expands it into the\n")
	buf.WriteString("// address -> default raw P4 map the fake answers from, and states what the\n")
	buf.WriteString("// table does and does not claim.\n")
	buf.WriteString("//\n")
	buf.WriteString("// ONE TABLE SERVES BOTH MODELS. The manual prints Table 2 once for the\n")
	buf.WriteString("// FTDX101D and the FTDX101MP, so a NewD radio and a NewMP one answer EX reads\n")
	buf.WriteString("// identically; the two differ on this wire in the ID answer and nowhere else.\n")
	buf.WriteString("//\n")
	fmt.Fprintf(&buf, "// It is a PROJECTION OF TRANSCRIPTION B (%s), derived from that\n", csvName)
	buf.WriteString("// artefact's digits column and its own text flag, and from nothing else. The\n")
	buf.WriteString("// dialect's inventory (core/cat/ftdx101/exinventory_gen.go) is generated from\n")
	buf.WriteString("// transcription A by different code, and core/transport's cross-check proves the\n")
	buf.WriteString("// two agree — so a defect in either transcription or either generator shows up\n")
	buf.WriteString("// there.\n")
	buf.WriteString("//\n")
	fmt.Fprintf(&buf, "// %s, %s. Regenerate with `go generate ./internal/fakedx101`;\n", plural(len(groups), "subgroup"), plural(items, "item"))
	buf.WriteString("// gen/main_test.go refuses a file that has drifted from the CSV.\n")
	buf.WriteString("var exGroups = []struct{ p1, p2, widths string }{\n")
	for _, g := range groups {
		fmt.Fprintf(&buf, "\t{%s, %s, %s}, // %s — %s / %s, %s %s\n",
			strconv.Quote(g.p1), strconv.Quote(g.p2), strconv.Quote(g.widths),
			plural(len(g.widths), "item"), g.p1Label, g.p2Label, csvName, lineRange(g.firstLine, g.lastLine))
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
// line number rather than printing "lines 130-130".
func lineRange(first, last int) string {
	if first == last {
		return fmt.Sprintf("line %d", first)
	}
	return fmt.Sprintf("lines %d-%d", first, last)
}
