// SPDX-License-Identifier: GPL-3.0-or-later

package fakedx10

import (
	"bytes"
	_ "embed"
	"encoding/csv"
	"fmt"
	"slices"
	"strconv"
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
		panic("fakedx10: the embedded transcription B: " + err.Error())
	}
	groups, err := groupRows(rows)
	if err != nil {
		panic("fakedx10: the embedded transcription B: " + err.Error())
	}
	return groups
}

// bHeader is transcription B's exact header row, as delivered and committed.
// It is pinned so that a schema change fails LOUDLY here rather than being
// silently misparsed into a plausible wrong table.
//
// This is the DELIVERED schema, not the one B's brief asked for: the briefed
// `p1,p2,p3,p1_label,p2_label,name,digits,text` was lost to a mid-task
// stall/resume and the quarantined agent shipped these six columns instead,
// accepted verbatim (evidence integrity over format compliance). Consequences
// for this generator: the group labels arrive WRAPPED ("01 (RADIO SETTING)"),
// and there is no text flag to read — it has to be reconstructed from Digits
// and P4 (see widthToken).
var bHeader = []string{"P1", "P2", "P3", "Function", "P4", "Digits"}

// Column indices into a B record.
const (
	colP1 = iota
	colP2
	colP3
	colFunction
	colP4
	colDigits
	numCols
)

// textWidth is the wire width of B's one text item's P4 field, and the width
// that selects the 'T' token. It is spelt here as the number B's Digits column
// prints for that row.
const textWidth = 12

// textP4Prefix is how B's P4 column writes a character-count parameter ("Up to
// 12 characters"), as against the "0: X 1: Y" value legends every numeric row
// carries. Since the delivered schema has no text flag, this prefix IS the
// discriminator — and it is B's own cell, not a fact borrowed from the other
// transcription or from the dialect.
const textP4Prefix = "Up to"

// row is one parsed B data row, reduced to what the projection needs.
type row struct {
	// p1, p2 are the wire components, two digits each, exactly as they are
	// printed inside B's label cells.
	p1, p2 string
	// p1Label, p2Label are the label text inside those cells' parentheses,
	// verbatim (wrapper stripped). Read only so that groupRows can refuse a
	// group whose rows disagree about them.
	p1Label, p2Label string
	// p3 is the item index, 1-based.
	p3 int
	// token is the width token: '1'..'4' for a numeric field of that many
	// bytes, or 'T' for the 12-byte text field.
	token byte
	// line is the 1-based physical line in the CSV, header included, for
	// error messages.
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

// parseB parses transcription B into rows, in file order. Every malformed
// input is an error rather than a skipped row: this CSV is a committed
// evidential artefact, so anything the projection cannot read is a finding to
// report, never a row to drop.
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
		p1, p1Label, err := splitWrapped(rec[colP1])
		if err != nil {
			return nil, fmt.Errorf("line %d: P1 cell: %w", line, err)
		}
		p2, p2Label, err := splitWrapped(rec[colP2])
		if err != nil {
			return nil, fmt.Errorf("line %d: P2 cell: %w", line, err)
		}
		p3, err := parseTwoDigit(rec[colP3])
		if err != nil {
			return nil, fmt.Errorf("line %d: P3 cell: %w", line, err)
		}
		token, err := widthToken(rec[colDigits], rec[colP4])
		if err != nil {
			return nil, fmt.Errorf("line %d (%s %s / %s): %w", line, rec[colP1], rec[colP2], rec[colFunction], err)
		}
		out = append(out, row{
			p1: p1, p2: p2, p1Label: p1Label, p2Label: p2Label,
			p3: p3, token: token, line: line,
		})
	}
	return out, nil
}

// widthToken derives one width token from B's Digits cell, with B's P4 cell as
// the text discriminator the delivered schema left out.
//
// Digits 1-4 is a numeric field of that many bytes and yields that digit
// itself. Digits 12 yields 'T' — the 12-byte text field — but ONLY if P4 also
// describes a character count rather than a value legend: a hypothetical
// 12-digit NUMERIC item would answer twelve zeros, not twelve spaces, so
// deciding on the width alone would invent the wrong answer for it. Every
// other Digits value is refused: the compact form has no token for it, and
// guessing one would be this generator inventing wire behaviour the
// transcription does not describe.
func widthToken(digits, p4 string) (byte, error) {
	n, err := strconv.Atoi(strings.TrimSpace(digits))
	if err != nil {
		return 0, fmt.Errorf("Digits cell %q is not a number: %w", digits, err)
	}
	switch {
	case n >= 1 && n <= 4:
		return byte('0' + n), nil
	case n == textWidth:
		if !strings.HasPrefix(strings.TrimSpace(p4), textP4Prefix) {
			return 0, fmt.Errorf("Digits %d but P4 %q does not begin %q — B's width and its text discriminator disagree; arbitrate against the PDF, do not guess", n, p4, textP4Prefix)
		}
		return 'T', nil
	default:
		return 0, fmt.Errorf("Digits %d is neither 1-4 (numeric) nor %d (text): the compact inventory has no token for it", n, textWidth)
	}
}

// splitWrapped splits one of B's label cells — "01 (RADIO SETTING)" — into its
// two-digit wire component and the label text inside the parentheses.
//
// It refuses anything that is not exactly that shape, so a genuinely different
// cell cannot be silently reduced to a plausible (P1,P2) key. B prints the
// wrapper verbatim because that is how the chart prints it; the dialect's own
// transcription strips it. Neither reading is wrong (it is a question about
// typography), and this generator needs both halves: the digits for the wire
// address, the label for the emitted comment.
func splitWrapped(cell string) (digits, label string, err error) {
	s := strings.TrimSpace(cell)
	open := strings.IndexByte(s, '(')
	if open < 0 || !strings.HasSuffix(s, ")") {
		return "", "", fmt.Errorf("%q is not of the form \"NN (LABEL)\"", cell)
	}
	digits = strings.TrimSpace(s[:open])
	if _, err := parseTwoDigit(digits); err != nil {
		return "", "", fmt.Errorf("%q: %w", cell, err)
	}
	label = s[open+1 : len(s)-1]
	if label == "" {
		return "", "", fmt.Errorf("%q has an empty label", cell)
	}
	return digits, label, nil
}

// parseTwoDigit accepts exactly two ASCII digits and returns their value. The
// EX wire address is a fixed six-digit field of three two-digit components, so
// a one- or three-digit cell is a schema error, not a value to normalise.
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
//     another group intervened would mean the file interleaves subgroups,
//     which the chart does not, and which the widths string cannot express.
//   - P3 runs 01, 02, 03 … with no gaps. The string's index IS the item
//     index, so a gap would silently renumber every item after it.
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
				return nil, fmt.Errorf("line %d: group (%s,%s) opens at P3=%02d, want 01", r.line, r.p1, r.p2, r.p3)
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
			return nil, fmt.Errorf("line %d: group (%s,%s) item %d has P3=%02d, want %02d — P3 must run consecutively from 01", r.line, r.p1, r.p2, want, r.p3, want)
		}
		if r.p1Label != g.p1Label || r.p2Label != g.p2Label {
			return nil, fmt.Errorf("line %d: group (%s,%s) label cells disagree: %q/%q here, %q/%q at line %d", r.line, r.p1, r.p2, r.p1Label, r.p2Label, g.p1Label, g.p2Label, g.firstLine)
		}
		g.widths += string(r.token)
	}
	return groups, nil
}
