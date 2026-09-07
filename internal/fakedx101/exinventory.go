// SPDX-License-Identifier: GPL-3.0-or-later

package fakedx101

import (
	"bytes"
	_ "embed"
	"encoding/csv"
	"fmt"
	"io"
	"slices"
	"strings"
)

// # STANDARD LIBRARY ONLY, and why that is the whole point
//
// The parsing below imports nothing project-internal, and in particular NOT
// internal/extable — the machinery that derives the DIALECT's inventory from
// transcription A. imports_test.go's recursive fence enforces that
// mechanically, and the reason is the design this file exists to serve: the
// dialect's inventory comes from transcription A by one piece of code, this
// fake's from transcription B by the code below, and core/transport's
// cross-check proves the two agree. One parser on both sides of that
// comparison would reproduce a shared parsing bug into both inventories
// invisibly.
//
// It was a generator emitting a checked-in table until 06/09/2026. The
// generated file, and the second copy of the parser it needed, are gone; the
// projection now happens at init from the same committed bytes. The CSV itself
// has NOT moved — core/transport's cross-check reads it by path.

// transcriptionB is this package's OWN COPY of transcription B, embedded and
// projected here at init. PROVENANCE.md records where the copy came from and
// why it is a copy rather than a move.
//
//go:embed transcription-b.csv
var transcriptionB []byte

// exGroups is this fake's EX (MENU) inventory in compact form: one entry per
// (P1,P2) subgroup, in the chart's own order, with one width token per P3 item
// in P3 order starting at 01. A token is '1'..'4' — a numeric field of that
// many raw ASCII bytes — or 'T', the 12-byte text field. ex.go expands it into
// the address -> default raw P4 map the fake answers from, and states what the
// table does and does not claim.
//
// It is a PROJECTION OF TRANSCRIPTION B, derived from that artefact's Digits
// column alone, with its P4 column consulted only to tell a text item from a
// numeric one. ex_test.go pins the structural counts against the chart rather
// than against this table.
var exGroups = mustGroups(transcriptionB)

// mustGroups is the init-time projection. Every malformed input PANICS rather
// than yielding a shorter table: this CSV is a committed, hash-frozen
// evidential artefact, so anything the projection cannot read is a finding, and
// a fake answering from a truncated inventory would be worse than one that
// refuses to start.
func mustGroups(data []byte) []group {
	rows, err := parseB(data)
	if err != nil {
		panic("fakedx101: the embedded transcription B: " + err.Error())
	}
	groups, err := groupRows(rows)
	if err != nil {
		panic("fakedx101: the embedded transcription B: " + err.Error())
	}
	return groups
}

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
			p1: fmt.Sprintf("%02d", p1), p2: fmt.Sprintf("%02d", p2), p1Label: p1Label, p2Label: p2Label,
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
	n, err := parseDigitsCell(digits)
	if err != nil {
		return 0, err
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

// parseDigitsCell reads B's digits cell BYTE-EXACTLY: one or two ASCII digits,
// with no leading zero on the two-digit form, and nothing else — no sign, no
// padding, no surrounding whitespace. It is the same discipline parseTwoDigit
// applies to the address components and parseTextFlag to the boolean, and for
// the same reason, stated once here for all three:
//
// strconv.Atoi would accept "+4", "04" and (after a TrimSpace) " 4" as four.
// None of those appears in this artefact — its digits column prints exactly
// "1", "2", "3", "4" and "12" — so accepting them would not be tolerance of a
// real input, it would be a WIDER VOCABULARY THAN THE FILE HAS, and a
// re-transcription that quietly started zero-padding or aligning that column
// would be normalised into agreement instead of being reported. The projection
// refuses everywhere else rather than repairing; the width cell is the one
// column the entire inventory is derived from, and it should be the strictest
// read in the file, not the loosest.
//
// Note what it does NOT reject: "0". That is a well-formed cell carrying a
// value the compact form has no token for, so it is widthToken's refusal to
// make, with widthToken's message — a shape error and a value error are
// different findings and are reported as such.
func parseDigitsCell(cell string) (int, error) {
	if len(cell) < 1 || len(cell) > 2 {
		return 0, fmt.Errorf("digits cell %q is not one or two ASCII digits — B's digits column prints 1, 2, 3, 4 and 12, unpadded and unsigned", cell)
	}
	for i := 0; i < len(cell); i++ {
		if !isDigit(cell[i]) {
			return 0, fmt.Errorf("digits cell %q is not one or two ASCII digits — B's digits column prints 1, 2, 3, 4 and 12, unpadded and unsigned", cell)
		}
	}
	if len(cell) == 2 && cell[0] == '0' {
		return 0, fmt.Errorf("digits cell %q is zero-padded — B's digits column prints its widths bare, so a padded cell means the artefact's convention changed", cell)
	}
	if len(cell) == 2 {
		return int(cell[0]-'0')*10 + int(cell[1]-'0'), nil
	}
	return int(cell[0] - '0'), nil
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
				widths: string(r.token), firstLine: r.line,
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
	}
	return groups, nil
}
