// SPDX-License-Identifier: GPL-3.0-or-later

package extable

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"go/format"
	"os"
	"sort"
	"strconv"
)

// writeObservedColumns is table2-write-observed.csv's fixed column count,
// the same shape as table2-observed.csv's observedColumns:
// p1,p2,p3,observed_set_width,observed_set_shape.
const writeObservedColumns = 5

// WriteObserved is one address's Session W hardware SET observation: the P4
// wire width the radio accepted a Set at, and that Set's shape class. It is
// ParseObservedCSV's Observed, Set direction rather than Read.
type WriteObserved struct {
	SetWidth int
	SetShape string
}

// ParseWriteObservedCSV decodes core/cat/table2-write-observed.csv — the
// FT-710's Session W Set-width observations — into observations keyed by
// six-digit (P1,P2,P3) triple, the FT-710's own AddressTriple form. It
// takes no Profile, for the reason RenderWriteGo does not: this file has
// exactly one caller, ever, so its shape is this package's own constant
// rather than a field read off a caller-supplied profile. Parsing is
// otherwise ParseObservedCSV's own rules: '#' comment lines skipped, each
// address component exactly two digits, width 1..12 (the FT-710's own
// MaxObservedWidth), shape one of "numeric"/"signed"/"text", no duplicate
// address.
func ParseWriteObservedCSV(data []byte) (map[string]WriteObserved, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.Comment = '#'
	r.FieldsPerRecord = writeObservedColumns
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("extable: reading write-observation CSV: %w", err)
	}

	out := make(map[string]WriteObserved, len(records))
	for i, rec := range records {
		for c := 0; c < 3; c++ {
			if !isTwoDigits(rec[c]) {
				return nil, fmt.Errorf("extable: write-observation row %d: address component %d must be exactly two digits", i+1, c)
			}
		}
		addr := rec[0] + rec[1] + rec[2]
		width, err := strconv.Atoi(rec[3])
		if err != nil || width < 1 || width > 12 {
			return nil, fmt.Errorf("extable: write-observation row %d (%s): observed_set_width must be an integer in 1..12", i+1, addr)
		}
		switch rec[4] {
		case "numeric", "signed", "text":
		default:
			return nil, fmt.Errorf("extable: write-observation row %d (%s): unknown observed_set_shape", i+1, addr)
		}
		if _, dup := out[addr]; dup {
			return nil, fmt.Errorf("extable: write-observation row %d: duplicate address %s", i+1, addr)
		}
		out[addr] = WriteObserved{SetWidth: width, SetShape: rec[4]}
	}
	return out, nil
}

// RenderWriteGo renders rows as p's write-descriptor table (the FT-710's is
// core/cat/exwrite_gen.go): one core/cat.EXWriteItem per row, ALL
// p.ExpectedRows, admitted or denied or held — the denylist (core/cat/exdenylist.go) is
// never consulted here, on the same "no denylist awareness" ruling
// RenderGo's own doc comment would give if it had one (B2 fix 2 of the
// milestone plan: the generator must not import core/cat/exdenylist.go, or
// internal/extable would depend on core/cat the other way).
//
// It is a SIBLING of RenderGo, not an extension of it: core/cat.Domain and
// core/cat.EXWriteItem are declared in core/cat and emitted here as literal
// Go source by field name — the same textual-emission trick RenderGo
// already uses to write EXItem/cat.EXItem/kw.EXItem literals for types it
// never imports.
//
// observed may be nil, partial, or cover every row — RenderWriteGo never
// requires set equality between rows and observed, unlike RenderGo's
// ObservationsRequired regime: Session W characterises the 238 admitted
// addresses incrementally, over hours at the radio, and a row with no entry
// yet renders ObservedSetWidth's zero sentinel rather than failing the
// whole generation. A legend that fails to parse (ParseP4Domain's ok ==
// false) is listed to stderr as generation proceeds — spec §10.3 — and
// renders the zero Domain, which Contains refuses for every value.
// p.DomainOverrides replaces the parsed Domain of the addresses it names
// (hand-corrected legends); an override naming an address with no row is an
// error. A nil observed (a model with no hardware observations) renders
// ObservedSetWidth 0 for every row.
func RenderWriteGo(p Profile, rows []Row, observed map[string]WriteObserved) ([]byte, error) {
	if len(rows) != p.ExpectedRows {
		return nil, fmt.Errorf("extable: RenderWriteGo: parsed %d rows, want exactly %d — a source is incomplete", len(rows), p.ExpectedRows)
	}
	have := make(map[[3]int]bool, len(rows))
	for _, r := range rows {
		have[[3]int{r.P1, r.P2, r.P3}] = true
	}
	for a := range p.DomainOverrides {
		if !have[a] {
			return nil, fmt.Errorf("extable: RenderWriteGo: DomainOverrides names %02d/%02d/%02d, which has no row", a[0], a[1], a[2])
		}
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
	if p.WriteObservedCSV != "" {
		fmt.Fprintf(&buf, "// Code generated by internal/extable/gen from %s and\n// %s. DO NOT EDIT.\n\n", p.ManualCSV, p.WriteObservedCSV)
	} else {
		fmt.Fprintf(&buf, "// Code generated by internal/extable/gen from %s. DO NOT EDIT.\n\n", p.ManualCSV)
	}
	fmt.Fprintf(&buf, "package %s\n\n", p.Package)
	qual := ""
	if p.Types == TypesImported {
		qual = p.ImportAlias + "."
		fmt.Fprintf(&buf, "import %s %s\n\n", p.ImportAlias, strconv.Quote(p.ImportPath))
	}
	gen := "./core/cat"
	if p.Package != "cat" {
		gen += "/" + p.Package
	}
	fmt.Fprintf(&buf, "// %s is the %s's write-descriptor table: one EXWriteItem per\n", p.VarName, p.Model)
	buf.WriteString("// table2.csv row, admitted or not — task (c) filters this against the\n")
	buf.WriteString("// denylist and held set to build the dialect's actual write gate. Domain is\n")
	buf.WriteString("// parsed from the manual's P4 legend (internal/extable.ParseP4Domain); a\n")
	buf.WriteString("// legend that fails to parse renders the zero Domain, which Contains\n")
	buf.WriteString("// refuses every value. ObservedSetWidth is the rendered sentinel 0 until\n")
	buf.WriteString("// Session W's hardware Set-width characterisation\n")
	buf.WriteString("// (table2-write-observed.csv) fills it in. Regenerate with\n")
	fmt.Fprintf(&buf, "// `go generate %s`; do not edit by hand.\n", gen)
	fmt.Fprintf(&buf, "var %s = []%sEXWriteItem{\n", p.VarName, qual)
	for _, r := range sorted {
		domain, ok := ParseP4Domain(r.P4)
		if o, found := p.DomainOverrides[[3]int{r.P1, r.P2, r.P3}]; found {
			domain, ok = o, true
		}
		if !ok {
			fmt.Fprintf(os.Stderr, "extable: RenderWriteGo: legend does not parse, held read-only: %02d/%02d/%02d %s (%q)\n", r.P1, r.P2, r.P3, r.Name, r.P4)
		}
		addr := fmt.Sprintf("%02d%02d%02d", r.P1, r.P2, r.P3)
		obs := observed[addr]
		fmt.Fprintf(&buf,
			"\t{Addr: %[1]sEXAddress{P1: %[2]d, P2: %[3]d, P3: %[4]d}, Domain: %[1]sDomain{Codes: %#[5]v, Lo: %[6]d, Hi: %[7]d, Step: %[8]d, Signed: %[9]t}, ObservedSetWidth: %[10]d, ObservedSetShape: %[11]s}, // manual line %[12]d\n",
			qual, r.P1, r.P2, r.P3,
			domain.Codes, domain.Lo, domain.Hi, domain.Step, domain.Signed,
			obs.SetWidth, strconv.Quote(obs.SetShape), r.ManualLine)
	}
	buf.WriteString("}\n")

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("extable: formatting generated Go: %w", err)
	}
	return formatted, nil
}
