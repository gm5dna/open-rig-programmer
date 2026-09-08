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
// (internal/extable/gen, invoked by `go generate ./core/cat`) and the
// core/cat staleness test that re-derives the generated file from both
// sources and byte-compares it. The observation derivation tool
// (internal/extable/observe) was removed in v1.4.1; the committed
// core/cat/table2-observed.csv it produced is now the pinned record.
// Nothing here talks to a session, a driver, the allowlist, or the wire.
package extable

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"go/format"
	"slices"
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
// one of the profile's TextWidths for a text item (1..4 and 12 respectively
// for the FT-710); Text marks those text items; Parameterless marks the rows whose
// chart line names no field at all; and ManualLine is the source line in the
// manual extract the row was transcribed from.
type Row struct {
	P1, P2, P3 int
	P1Label    string
	P2Label    string
	Name       string
	P4         string
	Digits     int
	Text       bool
	// Parameterless is true for a row whose Digits cell is the single
	// hyphen parameterlessDigits, admitted only on an address the profile's
	// ParameterlessAddresses names. Digits is then LEFT AT ITS ZERO VALUE
	// and this flag is the only thing that says so: a Digits of 0 meaning
	// "no parameter" would be the omitted-semantic-defaulted hazard M9c-1
	// exists to refuse, and a -1 sentinel would be a width no type admits.
	// See ParameterlessRows; TestParseCSV_ParameterlessRow pins the shape.
	Parameterless bool
	ManualLine    int
}

// parameterlessDigits is the Digits cell a chart draws for a row with no
// parameter — one hyphen, which is what the FT-991A's chart prints at menu
// 087 (docs/fixtures-private/manuals/ft991a_layout.txt:623). It is tested
// for BEFORE strconv.Atoi below, so Atoi never sees a hyphen and its own
// refusal keeps meaning "this cell is not a number".
const parameterlessDigits = "-"

// ParseCSV decodes the Table 2 CSV against the model profile p, which it
// validates first. Lines beginning with '#' are treated as provenance
// comments and skipped. Parsing is deliberately strict: a malformed row
// (wrong column count, unparseable integer/boolean fields), a blank (empty
// or whitespace-only) P1Label or P2Label under LabelsRequired — or a
// NON-blank one under LabelsAbsent — a blank Name or P4, a non-positive
// ManualLine, a duplicate (P1,P2,P3) triple, a non-zero P3 under
// AddressPair, a non-zero P2 or P3 under AddressSingle, a text row under
// TextRowsAbsent, a non-text row whose Digits falls outside the profile's
// MinDigits..MaxDigits, a text row whose Digits is named by none of the
// profile's TextWidths, an address component outside the DOMAIN THIS
// PROFILE'S OWN FORM gives it (0..99 per component under AddressTriple and
// AddressPair; 0..999 for P1 under AddressSingle, whose field is three digits
// wide; 0..1 for P1 under AddressGrouped, which is the menu-type enumeration
// and not its one-digit field's capacity), a hyphen Digits cell on an
// address the profile's ParameterlessAddresses does not name (or under
// ParameterlessRefused at all), and a numeric Digits cell on an address it
// does name, each fail with a non-nil
// error rather than being guessed at. The returned rows preserve CSV order.
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
		// %02d here is a MINIMUM width, and it is LEFT ALONE deliberately.
		// Under AddressSingle a P1 of 153 renders "153" and one of 8 renders
		// "08", which is fine because this is a DIAGNOSTIC and not a join
		// token: nothing reads it back. The two %02d sites that ARE join
		// tokens — ParseObservedCSV's key and RenderGo's lookup — became
		// %03d under Single at the FT-991A seam, and this one is named here
		// so a later sweep can see it was considered rather than missed, and
		// so nobody "fixes" it and moves a shipped refusal string. A FOURTH
		// %02d address-key site once existed outside this package, in
		// internal/extable/observe/main.go's isText map — that tool was
		// hard-wired to FT710Profile() and could only ever see a Triple
		// chart. It was removed in v1.4.1; the committed
		// core/cat/table2-observed.csv it produced is now the pinned
		// record, so the site is gone rather than named here.
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
	// A SWITCH, not an if/else with an implicit AddressTriple arm — the
	// shape ParseObservedCSV below already takes, and for the same reason:
	// Profile.Validate (profile.go) has already required p.Addresses to be
	// one of the four known forms, but THIS is the site that reads it, and an
	// omitted config semantic is refused here too rather than defaulted to
	// the permissive arm.
	//
	// Under AddressPair the radio's field carries P1 and P2 only, and under
	// AddressSingle P1 alone, so a non-zero component beyond the field names
	// something no frame can express. Refused rather than dropped — a value
	// silently discarded here would reach the generated inventory as a 0 that
	// nothing recorded having changed.
	//
	// THE COMPONENT DOMAIN IS CHECKED INSIDE THIS SWITCH, not before it,
	// because it is the FORM'S fact: three of the four forms bound a
	// component by the capacity of the field it renders into, and the four
	// forms have four fields. AddressGrouped's P1 is the exception and its
	// arm says why.
	// It sat above the switch while every form's components were two digits
	// wide, which made 0..99 look like a property of an address rather than
	// of a wire field, and made rows 100-153 of a single-number chart
	// untranscribable.
	switch p.Addresses {
	case AddressTriple:
		// All three components are on the wire, each two digits of a
		// six-digit field; nothing further to check.
		if err := checkTwoDigitComponents(row); err != nil {
			return Row{}, err
		}
	case AddressPair:
		if err := checkTwoDigitComponents(row); err != nil {
			return Row{}, err
		}
		if row.P3 != 0 {
			return Row{}, fmt.Errorf("p3 must be 0 under %v, got %d", p.Addresses, row.P3)
		}
	case AddressSingle:
		// P1 IS the whole address here and its field is three digits, so
		// its domain is 0..999 — the field's capacity, not the chart's row
		// count, which membership is what refuses. The refusal names the
		// FORM as well as the number: a sentence quoting a bound without
		// saying which form is in force leaves a reader to guess which of
		// two rules they broke.
		// TestParseCSV_AddressSingleP1DomainIs0To999 pins the whole domain,
		// 001 to 153 included, and
		// TestParseCSV_TheOtherFormsKeepTheTwoDigitDomain the disagreement.
		if row.P1 < 0 || row.P1 > singleP1Ceiling {
			return Row{}, fmt.Errorf("address component P1 must be 0..%d under %v, got %d", singleP1Ceiling, p.Addresses, row.P1)
		}
		// The same rule one component further down: the chart prints ONE
		// menu number and it is the whole address, so p2 joins p3 in having
		// to be 0. TestParseCSV_AddressSingleRefusesNonZeroP2AndP3 pins both,
		// and pins that AddressPair still accepts the p2 its own field
		// carries. Neither needs a domain check of its own: 0 is the only
		// value either may hold.
		if row.P2 != 0 {
			return Row{}, fmt.Errorf("p2 must be 0 under %v, got %d", p.Addresses, row.P2)
		}
		if row.P3 != 0 {
			return Row{}, fmt.Errorf("p3 must be 0 under %v, got %d", p.Addresses, row.P3)
		}
	case AddressGrouped:
		// ALL THREE components are on the wire — one digit, then two, then
		// two — so this arm refuses none of them. The zero rules the two
		// arms above carry would refuse every row of a grouped chart;
		// TestParseCSV_AddressGroupedCarriesAllThreeComponents is the pin.
		//
		// P1's bound is checked FIRST and it is the only one in this switch
		// that is not a field's capacity: the manuals enumerate the two
		// menu-type values, so 0..1 is narrower than the one-digit field's
		// 0..9 (see AddressGrouped's own doc comment). The two-digit sweep
		// then runs over all three, which is a second no-op pass on a P1
		// this arm has already bounded more tightly, and is what gives P2
		// and P3 the SHIPPED two-digit refusal sentence rather than a
		// grouped copy of it.
		if row.P1 < 0 || row.P1 > groupedP1Ceiling {
			return Row{}, fmt.Errorf("address component P1 must be 0..%d under %v, got %d", groupedP1Ceiling, p.Addresses, row.P1)
		}
		if err := checkTwoDigitComponents(row); err != nil {
			return Row{}, err
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
	// The parameterless hyphen is ruled on BEFORE the Atoi, and it is ruled
	// on PER ADDRESS: the profile's ParameterlessExcluded policy licenses
	// the hyphen on the addresses it names and nowhere else, so a stray
	// hyphen anywhere in a 153-row chart is still a transcription error.
	// The converse is checked too — a declared address carrying a NUMBER is
	// a width smuggled onto a row the profile says has none — because a
	// policy enforced in one direction only would let either source drift.
	parameterless := isParameterlessAddress(p, row.P1, row.P2, row.P3)
	if rec[7] == parameterlessDigits {
		if p.ParameterlessPolicy != ParameterlessExcluded {
			return Row{}, fmt.Errorf("row (%s) has a %q digits cell under %v — this model's chart prints no parameterless row, so a hyphen there is a transcription error", row.Name, parameterlessDigits, p.ParameterlessPolicy)
		}
		if !parameterless {
			return Row{}, fmt.Errorf("row (%s) has a %q digits cell, but address %d/%d/%d is not one this profile's ParameterlessAddresses names", row.Name, parameterlessDigits, row.P1, row.P2, row.P3)
		}
		row.Parameterless = true
	} else {
		if parameterless {
			return Row{}, fmt.Errorf("row (%s) at address %d/%d/%d is declared parameterless, but its digits cell is %q — a declared exclusion may not carry a width", row.Name, row.P1, row.P2, row.P3, rec[7])
		}
		if row.Digits, err = strconv.Atoi(rec[7]); err != nil {
			return Row{}, fmt.Errorf("bad digits %q: %w", rec[7], err)
		}
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
		// MEMBERSHIP, not equality: a chart may print text rows at more
		// than one width, and the refusal names the whole declared set
		// because a sentence quoting one number would be false of such a
		// chart. TestParseCSV_TextRowMatchesAnyDeclaredWidth pins both
		// halves.
		if !slices.Contains(p.TextWidths, row.Digits) {
			return Row{}, fmt.Errorf("text row (%s) must have digits %v, got %d", row.Name, p.TextWidths, row.Digits)
		}
	} else if !row.Parameterless && (row.Digits < p.MinDigits || row.Digits > p.MaxDigits) {
		// The MinDigits..MaxDigits check is skipped for a parameterless row
		// ALONE — it has no width to bound, and 0 is not one. Everything
		// else about the row, its printed cells included, is transcribed and
		// checked exactly as any other row's.
		return Row{}, fmt.Errorf("non-text row (%s) digits must be %d..%d, got %d", row.Name, p.MinDigits, p.MaxDigits, row.Digits)
	}
	return row, nil
}

// singleP1Ceiling is the largest P1 an AddressSingle chart may print: the
// capacity of the three-digit menu-number field the form renders into. It is
// core/cat's maxEXComponentSingleP1 stated on this side of the seam, as the
// two-digit forms' 99 is core/cat's maxEXComponent — the two packages have
// no import relationship (this one RENDERS core/cat source text) so each
// states the bound its own parser enforces, and core/cat's V8 is what refuses
// an inventory that disagrees.
//
// The FT-991A's own chart stops at 153. That is not this bound: membership
// refuses 154, and this refuses the address the FIELD could never carry.
const singleP1Ceiling = 999

// groupedP1Ceiling is the largest P1 an AddressGrouped chart may print. It is
// the ENUMERATION the manuals give the menu-type digit — 0: Menu, 1: Advanced
// Menu — and NOT the capacity of the one-digit field it renders into, which
// is 9. It is the only component bound in this file that is not a field's
// capacity, and AddressGrouped's doc comment carries the citations.
const groupedP1Ceiling = 1

// checkTwoDigitComponents applies the two-digit component domain — the one
// AddressTriple and AddressPair render every component into — to all three of
// row's components.
//
// Its sentence is SHIPPED TEXT, unchanged since before the address form
// existed, and it is a separate literal from AddressSingle's rather than one
// composed from whichever bound is in force, because a composed sentence
// would have moved this one for every model in the repository the day a
// third form arrived.
func checkTwoDigitComponents(row Row) error {
	for i, v := range []int{row.P1, row.P2, row.P3} {
		if v < 0 || v > 99 {
			return fmt.Errorf("address component P%d must be 0..99, got %d", i+1, v)
		}
	}
	return nil
}

// isParameterlessAddress reports whether the profile names (p1,p2,p3) as a
// row its chart prints with no parameter. It reads the ADDRESS SET, which is
// the datum — never a count of it — so parseRecord and RenderGo below rule on
// the same fact rather than on two proxies for it.
func isParameterlessAddress(p Profile, p1, p2, p3 int) bool {
	for _, a := range p.ParameterlessAddresses {
		if a == [3]int{p1, p2, p3} {
			return true
		}
	}
	return false
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
// AddressTriple, e.g. "010321", four under AddressPair, e.g. "0801", three
// under AddressSingle, e.g. "008", or five under AddressGrouped, e.g.
// "10203". The key follows p.Addresses for the same
// reason RenderGo's lookup does (see that function's matching comment) — it
// is a CSV join token, not a wire render, but the two sides of the join must
// agree on its shape or a complete narrow-form observation CSV can never be
// found by RenderGo's own lookup, however exhaustively it was captured.
// Lines beginning with '#' are provenance comments and are skipped, as in
// ParseCSV.
//
// Parsing is strict for privacy as much as correctness: each address
// component must be exactly two digits — EXCEPT p1 under AddressSingle,
// whose field is three digits wide and whose column must therefore be
// exactly three, and p1 under AddressGrouped, whose menu-type field is one
// digit wide and whose column must therefore be exactly one, each with its
// own refusal sentence, because a sentence saying "two" would be simply
// false of them — each width an integer in 1..the
// profile's MaxObservedWidth, and each shape one of the three known
// classes, so a row cannot carry free text. Under AddressPair the p3
// column must additionally be "0" — and under AddressSingle the p2 column
// as well — mirroring parseRecord's own rules for the inventory CSV, and
// neither is part of the key: those forms' wire fields carry P1 and P2, or
// P1 alone, so a component the wire can never express must be refused, not
// silently folded into a wider key nothing else can produce. AddressGrouped
// puts all three on the wire and so requires no zero of any of them.
// Duplicates are rejected. Error text names the
// row and address only — never another field — so a malformed artefact
// cannot leak captured content through a build log.
//
// That bound is hardware-evidence policy and is deliberately independent of
// the manual-schema widths in MinDigits/MaxDigits/TextWidths — the two
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
		// The SHAPE check is form-dependent for the same reason the domain
		// is: a column's width is the width of the field it stands for. Only
		// p1 under AddressSingle differs, and its sentence branches with it
		// — the two-digit one below is SHIPPED TEXT and is what the other two
		// forms (and this form's own p2/p3 columns) still refuse with.
		for c := 0; c < 3; c++ {
			if c == 0 && p.Addresses == AddressGrouped {
				// One digit, because that is the width of the field this
				// form's P1 renders into — the capacity, as the two- and
				// three-digit sentences are, even though the VALUE domain
				// parseRecord enforces is narrower. Its sentence names P1
				// and the form, on the AddressSingle arm's precedent.
				if !isOneDigit(rec[c]) {
					return nil, fmt.Errorf("extable: observation row %d: address component P1 must be exactly one digit under %v", i+1, p.Addresses)
				}
				continue
			}
			if c == 0 && p.Addresses == AddressSingle {
				if !isThreeDigits(rec[c]) {
					// "P1", not the loop's 0-based c: this arm runs for c ==
					// 0 alone, and parseRecord's own domain refusal for the
					// same column says "address component P1". The two-digit
					// sentence below keeps its index because it is SHIPPED
					// TEXT; this one was new (seat 1 LOW-2).
					return nil, fmt.Errorf("extable: observation row %d: address component P1 must be exactly three digits under %v", i+1, p.Addresses)
				}
				continue
			}
			if !isTwoDigits(rec[c]) {
				return nil, fmt.Errorf("extable: observation row %d: address component %d must be exactly two digits", i+1, c)
			}
		}
		// The key follows p.Addresses — RenderGo's lookup key's own form,
		// not always six digits (S0-close review, MEDIUM-2). A switch, not
		// an AddressTriple-shaped default: p.Validate above has already
		// required p.Addresses to be one of the four known forms, but this
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
		case AddressSingle:
			// Neither p2 nor p3 is on the wire under this form, so both are
			// checked and dropped and the key is P1 alone — the same THREE
			// digits RenderGo's own "%03d" lookup renders below, which is
			// what makes a captured observation findable at all. The column
			// has already been required to be exactly three digits above, so
			// taking it verbatim IS the fixed-width token; a %02d key would
			// render 153 as "153" and 8 as "08", and the two sides would
			// agree only by accident.
			// TestParseObservedCSV_AddressSingleKeysOnP1Alone pins the key
			// and TestParseObservedCSV_AddressSingleRefusesNonZeroP2AndP3
			// the two refusals.
			if p2, err := strconv.Atoi(rec[1]); err != nil || p2 != 0 {
				return nil, fmt.Errorf("extable: observation row %d: p2 must be 0 under %v, got %q", i+1, p.Addresses, rec[1])
			}
			if p3, err := strconv.Atoi(rec[2]); err != nil || p3 != 0 {
				return nil, fmt.Errorf("extable: observation row %d: p3 must be 0 under %v, got %q", i+1, p.Addresses, rec[2])
			}
			addr = rec[0]
		case AddressGrouped:
			// All three columns, verbatim: the shape checks above have
			// required exactly one, two and two digits, so concatenating
			// them IS the five-character fixed-width token RenderGo's own
			// "%d%02d%02d" renders. No component is dropped, because none is
			// off the wire.
			addr = rec[0] + rec[1] + rec[2]
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

// isOneDigit reports whether s is exactly one ASCII digit — the p1 column's
// shape under AddressGrouped, whose menu-type field is one digit wide. A
// sibling of isTwoDigits and isThreeDigits, for the reason they are siblings
// of each other: each form's own arm names its own width.
func isOneDigit(s string) bool {
	return len(s) == 1 && s[0] >= '0' && s[0] <= '9'
}

// isThreeDigits reports whether s is exactly three ASCII digits — the p1
// column's shape under AddressSingle, whose wire field is three digits wide.
// A sibling of isTwoDigits rather than a width-parameterised version of it,
// for the reason core/cat's threeDigitsAt is a sibling of twoDigitsAt: each
// form's own arm names its own width, and a shared helper taking a width
// would put that width somewhere other than the arm that knows it.
func isThreeDigits(s string) bool {
	if len(s) != 3 {
		return false
	}
	return s[0] >= '0' && s[0] <= '9' && s[1] >= '0' && s[1] <= '9' && s[2] >= '0' && s[2] <= '9'
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
	// The exclusion is recorded in the generated header BY ADDRESS, so a
	// reader of the artefact alone can see which of the chart's rows is
	// missing and why, without holding the profile beside it. Under
	// ParameterlessRefused the set is empty and nothing is emitted, which is
	// what keeps the five registered inventories byte-identical.
	// TestRenderGo_ParameterlessAddressIsAbsentByAddress pins both halves.
	if len(p.ParameterlessAddresses) > 0 {
		buf.WriteString("//\n")
		fmt.Fprintf(&buf, "// EXCLUDED, by address, under %v: ", p.ParameterlessPolicy)
		excl := append([][3]int(nil), p.ParameterlessAddresses...)
		sort.Slice(excl, func(i, j int) bool {
			a, b := excl[i], excl[j]
			if a[0] != b[0] {
				return a[0] < b[0]
			}
			if a[1] != b[1] {
				return a[1] < b[1]
			}
			return a[2] < b[2]
		})
		for i, a := range excl {
			if i > 0 {
				buf.WriteString(", ")
			}
			fmt.Fprintf(&buf, "%d/%d/%d", a[0], a[1], a[2])
		}
		buf.WriteString(".\n")
		buf.WriteString("// The chart prints that row with no parameter at all, so it is transcribed\n")
		buf.WriteString("// and counted but names no field an EX frame could read or write.\n")
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
	emitted := 0
	for _, r := range sorted {
		// A row the profile names as parameterless is omitted BY ADDRESS —
		// the profile's own datum — rather than by the Row flag alone. The
		// two agree by construction after parseRecord, but RenderGo is a
		// separate entry point and a caller may hand it rows it did not
		// parse; keying on the set means the omitted row is the DECLARED
		// one, never merely a row that happened to arrive flagged. THE
		// GROUPED ARM BELOW IS THE ONE WHOSE KEY WIDTH ALSO DEPENDS ON A
		// PARSE-TIME BOUND, not only on the row's own address: its "%d" P1
		// digit stays one character only because parseRecord's
		// groupedP1Ceiling stays at or below 9, the one-digit field's
		// capacity, which TestParseCSV_AddressGroupedP1DomainIs0To1 pins —
		// a caller handing RenderGo an unparsed row is not re-checked here.
		if isParameterlessAddress(p, r.P1, r.P2, r.P3) {
			continue
		}
		emitted++
		// The observation lookup key follows THIS PROFILE'S OWN address
		// form (S0-close review, LOW-3) rather than always being rendered
		// six digits wide: under AddressPair the wire field carries P1 and
		// P2 only (parseRecord above refuses a non-zero P3), and under
		// AddressSingle P1 alone, so those radios' own observation CSVs can
		// never carry a six-digit address — keying the lookup that way would
		// refuse every row's observation, however complete the CSV was. It
		// is a CSV join token, not a wire render — core/cat's wireEXAddress
		// is the wire-side counterpart — so it is derived here rather than
		// through that renderer.
		// A SWITCH, not an if/else with an implicit AddressPair arm, for the
		// reason parseRecord's own switch above gives: Profile.Validate has
		// already required one of the four known forms, and this site
		// refuses an unset one rather than quietly rendering a key of the
		// wrong width.
		var addr string
		switch p.Addresses {
		case AddressTriple:
			addr = fmt.Sprintf("%02d%02d%02d", r.P1, r.P2, r.P3)
		case AddressPair:
			addr = fmt.Sprintf("%02d%02d", r.P1, r.P2)
		case AddressSingle:
			// P1 alone, THREE digits: the chart's menu number IS the address,
			// its field is three digits wide, and ParseObservedCSV builds the
			// same key from the same column (which it has required to be
			// exactly three digits). %02d would be a MINIMUM width here — it
			// renders 153 as "153" but 8 as "08" — so a complete capture of a
			// 153-row chart would miss on every row below 100 while looking
			// like it agreed on the rest.
			// TestRenderGo_SingleProfileKeysObservationsByThreeDigitForm goes
			// through both sides of the join, so a disagreement fails there.
			addr = fmt.Sprintf("%03d", r.P1)
		case AddressGrouped:
			// One digit, two, two — five characters, all three components,
			// matching the token ParseObservedCSV builds from the three
			// columns it has required to be exactly those widths, so the
			// zero group keys "00005" and not "0005".
			//
			// "%d" is a MINIMUM width, and it is one character here only
			// because parseRecord has already bounded P1 by groupedP1Ceiling
			// — the two are COUPLED, and widening that constant past 9 would
			// silently widen this key while the observation CSV's one-digit
			// column kept producing the old one. That is the AddressSingle
			// "%02d" failure in a new place;
			// TestParseCSV_AddressGroupedP1DomainIs0To1 is what holds the
			// bound that makes it unreachable.
			// TestRenderGo_GroupedProfileKeysObservationsByFiveDigitForm goes
			// through both sides of the join, so a disagreement fails there.
			addr = fmt.Sprintf("%d%02d%02d", r.P1, r.P2, r.P3)
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
	// The accounting the exclusion owes: ExpectedRows counts the chart's
	// printed rows, the declared set says how many of them name no field, and
	// what is emitted is the difference. The check catches the case the
	// address-keyed skip above cannot — a profile declaring an address its
	// CSV never carries, which would silently emit one item too many while
	// every count in the profile still looked consistent.
	// TestRenderGo_ParameterlessArithmetic pins it.
	if want := p.ExpectedRows - len(p.ParameterlessAddresses); emitted != want {
		return nil, fmt.Errorf("extable: profile %s: emitted %d items, want %d — ExpectedRows %d less the %d address(es) ParameterlessAddresses names, so a declared exclusion is missing from the inventory", p.Model, emitted, want, p.ExpectedRows, len(p.ParameterlessAddresses))
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("extable: formatting generated Go: %w", err)
	}
	return formatted, nil
}
