// SPDX-License-Identifier: GPL-3.0-or-later

// Package extable transcodes the FT-710 CAT manual's Table 2 (the "MENU
// Chart") into the generated Go inventory (core/cat/exinventory_gen.go),
// joining two committed sources of different provenance:
//
//   - core/cat/table2.csv — the manual TRANSCRIPTION: what Yaesu's chart
//     says, typos included, never edited from hardware.
//   - core/cat/table2-observed.csv — hardware OBSERVATIONS: the P4 wire
//     width and shape each address answered with during the M8c read
//     characterisation — two sweeps of one radio, one firmware, one
//     configuration, read direction only (see that file's own provenance
//     header).
//
// The two are deliberately kept apart and merely joined here, so the
// generated inventory can carry both what the manual claims and what one
// radio actually answered without either being quietly rewritten into the
// other.
//
// It is build-time tooling ONLY. Its importers are the generator
// (internal/extable/gen, invoked by `go generate ./core/cat`), the
// observation derivation tool (internal/extable/observe), and the core/cat
// staleness test that re-derives the generated file from both sources and
// byte-compares it. Nothing here talks to a session, a driver, the
// allowlist, or the wire.
package extable

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"go/format"
	"sort"
	"strconv"
	"strings"
)

// numColumns is the fixed Table 2 CSV column count:
// p1,p2,p3,p1_label,p2_label,name,p4,digits,text,manual_line.
const numColumns = 10

// Row is one transcribed Table 2 entry. P1/P2/P3 are the decimal (P1,P2,P3)
// triple; the *Label and Name fields are verbatim manual text; P4 is the
// manual's parameter-description column (retained for the audit trail, not
// emitted into the generated Go); Digits is the manual's Digits column:
// within the profile's MinDigits..MaxDigits for a numeric field, or exactly
// the profile's TextWidth for a text item (1..4 and 12 respectively for the
// FT-710); Text marks those text items; and ManualLine is the source line
// in the manual extract the row was transcribed from.
type Row struct {
	P1, P2, P3 int
	P1Label    string
	P2Label    string
	Name       string
	P4         string
	Digits     int
	Text       bool
	ManualLine int
}

// ParseCSV decodes the Table 2 CSV against the model profile p, which it
// validates first. Lines beginning with '#' are treated as provenance
// comments and skipped. Parsing is deliberately strict: a malformed row
// (wrong column count, unparseable integer/boolean fields), a blank (empty
// or whitespace-only) P1Label, P2Label, Name, or P4, a non-positive
// ManualLine, a duplicate (P1,P2,P3) triple, a non-text row whose Digits
// falls outside the profile's MinDigits..MaxDigits, a text row whose Digits
// is not the profile's TextWidth, an address component outside 0..99 each
// fail with a non-nil error rather than being guessed at. The returned rows
// preserve CSV order.
func ParseCSV(p Profile, data []byte) ([]Row, error) {
	// The registry validates registered profiles, but nothing forces a
	// caller through the registry — the test fixtures do not go through it.
	// An unvalidated profile here would let omitted digit bounds be READ as
	// bounds (Codex plan review, finding 4).
	if err := p.Validate(); err != nil {
		return nil, err
	}
	r := csv.NewReader(bytes.NewReader(data))
	r.Comment = '#'
	r.FieldsPerRecord = numColumns
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("extable: reading CSV: %w", err)
	}

	rows := make([]Row, 0, len(records))
	seen := make(map[[3]int]bool, len(records))
	for i, rec := range records {
		row, err := parseRecord(p, rec)
		if err != nil {
			return nil, fmt.Errorf("extable: CSV data row %d: %w", i+1, err)
		}
		key := [3]int{row.P1, row.P2, row.P3}
		if seen[key] {
			return nil, fmt.Errorf("extable: CSV data row %d: duplicate (P1,P2,P3) triple %02d/%02d/%02d", i+1, row.P1, row.P2, row.P3)
		}
		seen[key] = true
		rows = append(rows, row)
	}
	return rows, nil
}

// parseRecord decodes one already-length-checked CSV record.
func parseRecord(p Profile, rec []string) (Row, error) {
	var row Row
	var err error
	if row.P1, err = strconv.Atoi(rec[0]); err != nil {
		return Row{}, fmt.Errorf("bad P1 %q: %w", rec[0], err)
	}
	if row.P2, err = strconv.Atoi(rec[1]); err != nil {
		return Row{}, fmt.Errorf("bad P2 %q: %w", rec[1], err)
	}
	if row.P3, err = strconv.Atoi(rec[2]); err != nil {
		return Row{}, fmt.Errorf("bad P3 %q: %w", rec[2], err)
	}
	for i, v := range []int{row.P1, row.P2, row.P3} {
		if v < 0 || v > 99 {
			return Row{}, fmt.Errorf("address component P%d must be 0..99, got %d", i+1, v)
		}
	}
	row.P1Label = rec[3]
	row.P2Label = rec[4]
	row.Name = rec[5]
	row.P4 = rec[6]
	if strings.TrimSpace(row.P1Label) == "" {
		return Row{}, fmt.Errorf("blank p1_label")
	}
	if strings.TrimSpace(row.P2Label) == "" {
		return Row{}, fmt.Errorf("blank p2_label")
	}
	if strings.TrimSpace(row.Name) == "" {
		return Row{}, fmt.Errorf("blank name")
	}
	if strings.TrimSpace(row.P4) == "" {
		return Row{}, fmt.Errorf("blank p4")
	}
	if row.Digits, err = strconv.Atoi(rec[7]); err != nil {
		return Row{}, fmt.Errorf("bad digits %q: %w", rec[7], err)
	}
	if row.Text, err = strconv.ParseBool(rec[8]); err != nil {
		return Row{}, fmt.Errorf("bad text flag %q: %w", rec[8], err)
	}
	if row.ManualLine, err = strconv.Atoi(rec[9]); err != nil {
		return Row{}, fmt.Errorf("bad manual_line %q: %w", rec[9], err)
	}
	if row.ManualLine <= 0 {
		return Row{}, fmt.Errorf("manual_line must be > 0, got %d", row.ManualLine)
	}

	// Digits/Text consistency: a text item carries exactly this radio's text
	// width; every other item is a numeric field within its digit bounds.
	if row.Text {
		if row.Digits != p.TextWidth {
			return Row{}, fmt.Errorf("text row (%s) must have digits %d, got %d", row.Name, p.TextWidth, row.Digits)
		}
	} else if row.Digits < p.MinDigits || row.Digits > p.MaxDigits {
		return Row{}, fmt.Errorf("non-text row (%s) digits must be %d..%d, got %d", row.Name, p.MinDigits, p.MaxDigits, row.Digits)
	}
	return row, nil
}

// observedColumns is the fixed observation CSV column count:
// p1,p2,p3,observed_read_width,observed_read_shape.
const observedColumns = 5

// maxObservedWidth is the largest P4 width any EX item can have (the
// 12-byte Text items). A wider observation means a malformed artefact.
const maxObservedWidth = 12

// Observed is one address's M8c hardware READ observation: the P4 wire
// width the radio answered with, and that answer's shape class
// ("numeric", "signed" or "text").
//
// READ DIRECTION ONLY. The M8c session probed no EX Set frame, so nothing
// here may be used to size or shape one — Set width policy is M8e's to
// define and M8f's to verify against hardware. The type is also
// deliberately value-free: it carries what a value LOOKED like, never
// what it was.
type Observed struct {
	ReadWidth int
	ReadShape string
}

// ParseObservedCSV decodes the hardware observation CSV
// (core/cat/table2-observed.csv) into observations keyed by six-digit wire
// address, e.g. "010321". Lines beginning with '#' are provenance comments
// and are skipped, as in ParseCSV.
//
// Parsing is strict for privacy as much as correctness: each address
// component must be exactly two digits, each width an integer in
// 1..maxObservedWidth, and each shape one of the three known classes, so a
// row cannot carry free text. Duplicates are rejected. Error text names the
// row and address only — never another field — so a malformed artefact
// cannot leak captured content through a build log.
func ParseObservedCSV(data []byte) (map[string]Observed, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.Comment = '#'
	r.FieldsPerRecord = observedColumns
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("extable: reading observation CSV: %w", err)
	}

	out := make(map[string]Observed, len(records))
	for i, rec := range records {
		for c := 0; c < 3; c++ {
			if !isTwoDigits(rec[c]) {
				return nil, fmt.Errorf("extable: observation row %d: address component %d must be exactly two digits", i+1, c)
			}
		}
		addr := rec[0] + rec[1] + rec[2]
		width, err := strconv.Atoi(rec[3])
		if err != nil || width < 1 || width > maxObservedWidth {
			return nil, fmt.Errorf("extable: observation row %d (%s): observed_read_width must be an integer in 1..%d", i+1, addr, maxObservedWidth)
		}
		switch rec[4] {
		case "numeric", "signed", "text":
		default:
			return nil, fmt.Errorf("extable: observation row %d (%s): unknown observed_read_shape", i+1, addr)
		}
		if _, dup := out[addr]; dup {
			return nil, fmt.Errorf("extable: observation row %d: duplicate address %s", i+1, addr)
		}
		out[addr] = Observed{ReadWidth: width, ReadShape: rec[4]}
	}
	return out, nil
}

// isTwoDigits reports whether s is exactly two ASCII digits.
func isTwoDigits(s string) bool {
	if len(s) != 2 {
		return false
	}
	return s[0] >= '0' && s[0] <= '9' && s[1] >= '0' && s[1] <= '9'
}

// RenderGo renders rows as the core/cat generated inventory file
// (exinventory_gen.go), joined with the hardware READ observations keyed by
// wire address. The output is deterministic — rows are sorted by (P1,P2,P3)
// before emission and the result is run through go/format, so two calls on
// equal input produce byte-identical, gofmt-clean output. The audit-only P4
// column is intentionally NOT emitted; each item's manual line is preserved
// as a trailing comment.
//
// The join is set-equal in BOTH directions: an inventory row with no
// observation, or an observation for an address the inventory does not
// have, is an error. Neither is a case to paper over with a zero value —
// the artefact is meant to be a complete sweep of exactly this inventory.
func RenderGo(rows []Row, observed map[string]Observed) ([]byte, error) {
	if len(observed) != len(rows) {
		return nil, fmt.Errorf("extable: %d observations for %d inventory rows — the observation CSV must cover the inventory exactly", len(observed), len(rows))
	}
	sorted := make([]Row, len(rows))
	copy(sorted, rows)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if a.P1 != b.P1 {
			return a.P1 < b.P1
		}
		if a.P2 != b.P2 {
			return a.P2 < b.P2
		}
		return a.P3 < b.P3
	})

	var buf bytes.Buffer
	buf.WriteString("// SPDX-License-Identifier: GPL-3.0-or-later\n\n")
	buf.WriteString("// Code generated by internal/extable/gen from table2.csv and\n")
	buf.WriteString("// table2-observed.csv. DO NOT EDIT.\n\n")
	buf.WriteString("package cat\n\n")
	buf.WriteString("// exItemsGen is the EX address inventory, sorted by (P1,P2,P3), built from\n")
	buf.WriteString("// TWO sources of different provenance: the manual transcription in\n")
	buf.WriteString("// table2.csv (the FT-710 CAT manual's Table 2 \"MENU Chart\"), and the M8c\n")
	buf.WriteString("// hardware READ observations in table2-observed.csv (what one radio\n")
	buf.WriteString("// answered — see that file's header for the scope). Regenerate with\n")
	buf.WriteString("// `go generate ./core/cat`; do not edit by hand.\n")
	buf.WriteString("var exItemsGen = []EXItem{\n")
	for _, r := range sorted {
		addr := fmt.Sprintf("%02d%02d%02d", r.P1, r.P2, r.P3)
		obs, ok := observed[addr]
		if !ok {
			return nil, fmt.Errorf("extable: no hardware observation for address %s", addr)
		}
		fmt.Fprintf(&buf,
			"\t{Addr: EXAddress{P1: %d, P2: %d, P3: %d}, P1Label: %s, P2Label: %s, Name: %s, Digits: %d, Text: %t, ObservedReadWidth: %d, ObservedReadShape: %s}, // manual line %d\n",
			r.P1, r.P2, r.P3,
			strconv.Quote(r.P1Label), strconv.Quote(r.P2Label), strconv.Quote(r.Name),
			r.Digits, r.Text, obs.ReadWidth, strconv.Quote(obs.ReadShape), r.ManualLine)
	}
	buf.WriteString("}\n")

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("extable: formatting generated Go: %w", err)
	}
	return formatted, nil
}
