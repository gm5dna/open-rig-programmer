// SPDX-License-Identifier: GPL-3.0-or-later

// Package extable transcodes a radio model's menu chart (for the FT-710, the
// CAT manual's Table 2) into that model's generated Go inventory, under a
// per-model Profile, joining two committed sources of different provenance:
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
// or whitespace-only) P1Label or P2Label under LabelsRequired — or a
// NON-blank one under LabelsAbsent — a blank Name or P4, a non-positive
// ManualLine, a duplicate (P1,P2,P3) triple, a non-zero P3 under
// AddressPair, a text row under TextRowsAbsent, a non-text row whose Digits
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
	// A SWITCH, not an if/else with an implicit AddressTriple arm — the
	// shape ParseObservedCSV below already takes, and for the same reason:
	// Profile.Validate (profile.go) has already required p.Addresses to be
	// one of the two known forms, but THIS is the site that reads it, and an
	// omitted config semantic is refused here too rather than defaulted to
	// the permissive arm.
	//
	// Under AddressPair the radio's field carries P1 and P2 only, so a
	// non-zero p3 names a component no frame can express. Refused rather
	// than dropped — a value silently discarded here would reach the
	// generated inventory as a 0 that nothing recorded having changed.
	switch p.Addresses {
	case AddressTriple:
		// All three components are on the wire; nothing further to check.
	case AddressPair:
		if row.P3 != 0 {
			return Row{}, fmt.Errorf("p3 must be 0 under %v, got %d", p.Addresses, row.P3)
		}
	default:
		return Row{}, fmt.Errorf("extable: profile %s: AddressForm %v must be set explicitly", p.Model, p.Addresses)
	}
	row.P1Label = rec[3]
	row.P2Label = rec[4]
	row.Name = rec[5]
	row.P4 = rec[6]
	// The label columns are the LabelPolicy's to rule on, in both
	// directions: a labelled chart's blank column is a transcription error,
	// and an unlabelled chart's non-blank one is an invented label.
	switch p.LabelPolicy {
	case LabelsAbsent:
		if strings.TrimSpace(row.P1Label) != "" {
			return Row{}, fmt.Errorf("p1_label is %q under %v, want blank — this model's chart prints no group labels", row.P1Label, p.LabelPolicy)
		}
		if strings.TrimSpace(row.P2Label) != "" {
			return Row{}, fmt.Errorf("p2_label is %q under %v, want blank — this model's chart prints no group labels", row.P2Label, p.LabelPolicy)
		}
	default:
		if strings.TrimSpace(row.P1Label) == "" {
			return Row{}, fmt.Errorf("blank p1_label")
		}
		if strings.TrimSpace(row.P2Label) == "" {
			return Row{}, fmt.Errorf("blank p2_label")
		}
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
	//
	// Under TextRowsAbsent there is no such width — the model's chart prints
	// no text row — so the flag itself is refused. That makes the
	// transcriber's "a text row is a STOP" convention mechanical instead of
	// a note in a brief.
	if row.Text {
		if p.TextRowPolicy == TextRowsAbsent {
			return Row{}, fmt.Errorf("row (%s) is flagged text under %v — this model's chart prints no free-text row, so a text row is a transcription error", row.Name, p.TextRowPolicy)
		}
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

// ParseObservedCSV decodes a model's hardware observation CSV — for the
// FT-710, core/cat/table2-observed.csv, but the path is the profile's
// ObservedCSV, not this one — into observations keyed by THIS PROFILE'S
// OWN address form (S0-close review's MEDIUM-2 finding): six digits under
// AddressTriple, e.g. "010321", or four under AddressPair, e.g. "0801". The
// key follows p.Addresses for the same reason RenderGo's lookup does (see
// that function's matching comment) — it is a CSV join token, not a wire
// render, but the two sides of the join must agree on its shape or a
// complete Pair-form observation CSV can never be found by RenderGo's own
// lookup, however exhaustively it was captured. Lines beginning with '#'
// are provenance comments and are skipped, as in ParseCSV.
//
// Parsing is strict for privacy as much as correctness: each address
// component must be exactly two digits, each width an integer in 1..the
// profile's MaxObservedWidth, and each shape one of the three known
// classes, so a row cannot carry free text. Under AddressPair the p3
// column must additionally be "0" — mirroring parseRecord's own P3 rule
// for the inventory CSV — and is not part of the key: a Pair-form radio's
// wire field carries P1 and P2 only, so a component the wire can never
// express must be refused, not silently folded into a six-digit key
// nothing else can produce. Duplicates are rejected. Error text names the
// row and address only — never another field — so a malformed artefact
// cannot leak captured content through a build log.
//
// That bound is hardware-evidence policy and is deliberately independent of
// the manual-schema widths in MinDigits/MaxDigits/TextWidth — the two
// categories can disagree, as table2-corrections.csv records.
func ParseObservedCSV(p Profile, data []byte) (map[string]Observed, error) {
	// Same self-validation as ParseCSV: nothing forces a caller through the
	// registry, and an unvalidated zero MaxObservedWidth would refuse every
	// width rather than the right ones.
	if err := p.Validate(); err != nil {
		return nil, err
	}
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
		// The key follows p.Addresses — RenderGo's lookup key's own form,
		// not always six digits (S0-close review, MEDIUM-2). A switch, not
		// an AddressTriple-shaped default: p.Validate above has already
		// required p.Addresses to be one of the two known forms, but this
		// switch is the site that actually reads it, and an omitted config
		// semantic is refused here too, not defaulted to the wider key.
		var addr string
		switch p.Addresses {
		case AddressTriple:
			addr = rec[0] + rec[1] + rec[2]
		case AddressPair:
			// p3 is not on the wire under this form (parseRecord enforces
			// the same rule for the inventory CSV's own P3), so it is
			// checked here and dropped from the key rather than folded
			// into a six-digit form RenderGo's Pair-form lookup can never
			// produce.
			if p3, err := strconv.Atoi(rec[2]); err != nil || p3 != 0 {
				return nil, fmt.Errorf("extable: observation row %d: p3 must be 0 under %v, got %q", i+1, p.Addresses, rec[2])
			}
			addr = rec[0] + rec[1]
		default:
			return nil, fmt.Errorf("extable: profile %s: AddressForm %v must be set explicitly", p.Model, p.Addresses)
		}
		width, err := strconv.Atoi(rec[3])
		if err != nil || width < 1 || width > p.MaxObservedWidth {
			return nil, fmt.Errorf("extable: observation row %d (%s): observed_read_width must be an integer in 1..%d", i+1, addr, p.MaxObservedWidth)
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

// RenderGo renders rows as the profile's generated inventory file, joined
// with the hardware READ observations keyed by wire address. The output is
// deterministic — rows are sorted by (P1,P2,P3) before emission and the
// result is run through go/format, so two calls on equal input produce
// byte-identical, gofmt-clean output. The audit-only P4 column is
// intentionally NOT emitted; each item's manual line is preserved as a
// trailing comment.
//
// The profile declares which of two observation regimes applies. Under
// ObservationsRequired the join is set-equal in BOTH directions: an
// inventory row with no observation, or an observation for an address the
// inventory does not have, is an error. Neither is a case to paper over
// with a zero value — the artefact is meant to be a complete sweep of
// exactly this inventory. Under ObservationsAbsent no hardware exists for
// the model, so the observation map must be EMPTY rather than partial, and
// every row renders the absence sentinels ObservedReadWidth 0 and
// ObservedReadShape "" that core/cat's EXItem already documents.
//
// Both regimes compare the two SUPPLIED sets against each other only, so
// neither can see a jointly truncated pair of sources. The profile's
// ExpectedRows is therefore checked first: the inventory must carry exactly
// that many rows, which is what makes deleting the same address from both
// CSVs — or emptying both — a refusal rather than a smaller happy render.
func RenderGo(p Profile, rows []Row, observed map[string]Observed) ([]byte, error) {
	// Self-validation, as in both parsers: a caller with an unvalidated
	// profile must get a refusal, not a plausible wrong file.
	if err := p.Validate(); err != nil {
		return nil, err
	}
	// Completeness first. Neither regime below can detect a JOINTLY
	// truncated pair of sources: RenderGo compares the two supplied sets
	// against each other, so deleting the same address from both — or
	// emptying both — would otherwise render happily.
	if len(rows) != p.ExpectedRows {
		return nil, fmt.Errorf("extable: profile %s: parsed %d inventory rows, want exactly %d — a source is incomplete", p.Model, len(rows), p.ExpectedRows)
	}
	switch p.Observations {
	case ObservationsRequired:
		if len(observed) != len(rows) {
			return nil, fmt.Errorf("extable: profile %s: %d observations for %d inventory rows — the observation CSV must cover the inventory exactly", p.Model, len(observed), len(rows))
		}
	case ObservationsAbsent:
		if len(observed) != 0 {
			return nil, fmt.Errorf("extable: profile %s declares no hardware observations, but %d were supplied", p.Model, len(observed))
		}
	default:
		return nil, fmt.Errorf("extable: profile %s: ObservationPolicy %v must be set explicitly", p.Model, p.Observations)
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
	// The generated-by marker names the profile's own sources. It is split
	// across two physical lines for a two-source profile because that is how
	// the FT-710's committed file has always been written; no physical line
	// matches Go's ^// Code generated .* DO NOT EDIT\.$ convention, and it
	// never has. Byte identity of the committed artefact is the acceptance
	// bar for M9c-2, so the non-conformance is preserved deliberately rather
	// than fixed here.
	//
	// The regime is branched on ObservedCSV here while it is ENFORCED on
	// p.Observations above — two proxies for one fact, safe only because
	// Validate runs first and forces the biconditional (ObservedCSV is
	// non-empty iff Observations is ObservationsRequired).
	if p.ObservedCSV != "" {
		fmt.Fprintf(&buf, "// Code generated by internal/extable/gen from %s and\n// %s. DO NOT EDIT.\n\n", p.ManualCSV, p.ObservedCSV)
	} else {
		fmt.Fprintf(&buf, "// Code generated by internal/extable/gen from %s. DO NOT EDIT.\n\n", p.ManualCSV)
	}
	fmt.Fprintf(&buf, "package %s\n\n", p.Package)
	// Under TypesImported the type qualifier IS the import alias, emitted
	// explicitly on the import — one string, so qualifier and import cannot
	// drift apart (Codex plan review, finding 3).
	qual := ""
	if p.Types == TypesImported {
		qual = p.ImportAlias + "."
		fmt.Fprintf(&buf, "import %s %s\n\n", p.ImportAlias, strconv.Quote(p.ImportPath))
	}
	for _, l := range p.DocLines {
		fmt.Fprintf(&buf, "// %s\n", l)
	}
	fmt.Fprintf(&buf, "var %s = []%sEXItem{\n", p.VarName, qual)
	// Under LabelsAbsent the generated item carries "" for both labels.
	// ParseCSV has already required the columns to be BLANK, which admits a
	// whitespace-only cell; emitting that verbatim would give a consumer a
	// space where it must see an absence. TestRenderGo_LabelsAbsentEmitsEmptyLabels
	// pins it; the labelled regime is untouched.
	if p.LabelPolicy == LabelsAbsent {
		for i := range sorted {
			sorted[i].P1Label, sorted[i].P2Label = "", ""
		}
	}
	for _, r := range sorted {
		// The observation lookup key follows THIS PROFILE'S OWN address
		// form (S0-close review, LOW-3) rather than always being rendered
		// six digits wide: under AddressPair the wire field carries P1 and
		// P2 only (parseRecord above refuses a non-zero P3), so a Pair
		// radio's own observation CSV can never carry a six-digit address —
		// keying the lookup that way would refuse every row's observation,
		// however complete the CSV was. It is a CSV join token, not a wire
		// render — core/cat's wireEXAddress is the wire-side counterpart —
		// so it is derived here rather than through that renderer.
		// A SWITCH, not an if/else with an implicit AddressPair arm, for the
		// reason parseRecord's own switch above gives: Profile.Validate has
		// already required one of the two known forms, and this site refuses
		// an unset one rather than quietly rendering the narrower key.
		var addr string
		switch p.Addresses {
		case AddressTriple:
			addr = fmt.Sprintf("%02d%02d%02d", r.P1, r.P2, r.P3)
		case AddressPair:
			addr = fmt.Sprintf("%02d%02d", r.P1, r.P2)
		default:
			return nil, fmt.Errorf("extable: profile %s: AddressForm %v must be set explicitly", p.Model, p.Addresses)
		}
		var obs Observed
		if p.Observations == ObservationsRequired {
			var ok bool
			if obs, ok = observed[addr]; !ok {
				return nil, fmt.Errorf("extable: no hardware observation for address %s", addr)
			}
		}
		fmt.Fprintf(&buf,
			"\t{Addr: %sEXAddress{P1: %d, P2: %d, P3: %d}, P1Label: %s, P2Label: %s, Name: %s, Digits: %d, Text: %t, ObservedReadWidth: %d, ObservedReadShape: %s}, // manual line %d\n",
			qual, r.P1, r.P2, r.P3,
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
