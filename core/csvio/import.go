// SPDX-License-Identifier: GPL-3.0-or-later

package csvio

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// ParseError reports a syntactic problem in a CSV import — own-schema
// (Import) or CHIRP (ImportCHIRP). Line is the 1-based PHYSICAL line
// number the problem was found on: the header is line 1 (so a
// header-level problem, e.g. an unknown or missing column, is always
// reported as line 1), and the first data row is line 2 — UNLESS an
// earlier row's quoted field contains an embedded newline (legal CSV),
// in which case it spans more than one physical line and every line
// number after it shifts accordingly. Both importers get this from
// csv.Reader.FieldPos (Go 1.17+) rather than a naive per-record counter,
// specifically so it stays correct in that case.
type ParseError struct {
	Line   int
	Reason string
}

// Error implements the error interface.
func (e *ParseError) Error() string {
	return fmt.Sprintf("csvio: line %d: %s", e.Line, e.Reason)
}

// dataColumns lists this schema's columns that carry ChannelData, i.e.
// every column except "slot" and "display". A row whose cells are ALL
// empty across exactly these columns decodes to an empty Channel (see
// Import).
var dataColumns = []string{
	"freq_hz", "mode", "clar_hz", "rx_clar", "tx_clar",
	"ctcss", "ctcss_tone", "shift", "tag", "tag_display", "scan_skip",
}

// requiredColumns lists every column Import requires to be present in
// the header. It is every column in header except "display", which is
// optional and ignored (see Import).
var requiredColumns = func() []string {
	cols := make([]string, 0, len(header)-1)
	for _, c := range header {
		if c != "display" {
			cols = append(cols, c)
		}
	}
	return cols
}()

// knownColumnSet is the set of every column name Import recognises
// (header, including "display").
var knownColumnSet = func() map[string]bool {
	set := make(map[string]bool, len(header))
	for _, c := range header {
		set[c] = true
	}
	return set
}()

// unescapeFormulaCell undoes EscapeCell: a single leading apostrophe, if
// present, is stripped.
func unescapeFormulaCell(s string) string {
	if strings.HasPrefix(s, "'") {
		return s[1:]
	}
	return s
}

// parseYesEmpty parses this schema's "yes"/"" boolean convention.
func parseYesEmpty(s string) (bool, error) {
	switch s {
	case "":
		return false, nil
	case "yes":
		return true, nil
	default:
		return false, fmt.Errorf("must be \"yes\" or empty, got %q", s)
	}
}

// parseToneFieldCell parses this schema's ctcss_tone column: "" ->
// Unknown, "n/a" -> Unavailable, otherwise a decimal Hz value (e.g.
// "88.5") -> Known, parsed EXACTLY (see parseExactToneDeciHz — no
// floating point, and no more than one decimal place of precision;
// "88.54" is a *ParseError, not silently rounded to "88.5"). It does not
// check the value against spec.StandardCTCSSTones — that is
// codeplug.ToneField.Valid's job (semantic), not this syntactic parse's.
func parseToneFieldCell(s string) (codeplug.ToneField, error) {
	switch s {
	case "":
		return codeplug.ToneField{State: codeplug.Unknown}, nil
	case "n/a":
		return codeplug.ToneField{State: codeplug.Unavailable}, nil
	default:
		deciHz, err := parseExactToneDeciHz(s)
		if err != nil {
			return codeplug.ToneField{}, fmt.Errorf("ctcss_tone must be \"\", \"n/a\" or a decimal Hz value with at most one decimal place, got %q", s)
		}
		return codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(deciHz)}, nil
	}
}

// parseBoolFieldCell parses this schema's scan_skip column: "" ->
// Unknown, "n/a" -> Unavailable, "yes"/"no" -> Known.
func parseBoolFieldCell(s string) (codeplug.BoolField, error) {
	switch s {
	case "":
		return codeplug.BoolField{State: codeplug.Unknown}, nil
	case "n/a":
		return codeplug.BoolField{State: codeplug.Unavailable}, nil
	case "yes":
		return codeplug.BoolField{State: codeplug.Known, Value: true}, nil
	case "no":
		return codeplug.BoolField{State: codeplug.Known, Value: false}, nil
	default:
		return codeplug.BoolField{}, fmt.Errorf("scan_skip must be \"\", \"n/a\", \"yes\" or \"no\", got %q", s)
	}
}

// validateImportHeader checks got (the CSV file's actual header row)
// against this package's schema: any column name appearing more than
// once in got is a duplicate; any column not in knownColumnSet is
// unknown; any column in requiredColumns absent from got is missing. All
// three problems are reported together in one error, naming every
// offending column, when any occur. A duplicate column's SECOND (and
// later) occurrence is not also independently evaluated for
// unknown/missing purposes — only its first occurrence is, exactly as if
// it appeared once — so a duplicated but otherwise-known, otherwise-
// required column is reported solely as a duplicate, not also as
// spuriously "missing".
func validateImportHeader(got []string) error {
	seen := make(map[string]bool, len(got))
	var unknown []string
	var duplicates []string
	for _, c := range got {
		if seen[c] {
			duplicates = append(duplicates, c)
			continue
		}
		seen[c] = true
		if !knownColumnSet[c] {
			unknown = append(unknown, c)
		}
	}
	var missing []string
	for _, c := range requiredColumns {
		if !seen[c] {
			missing = append(missing, c)
		}
	}
	if len(duplicates) == 0 && len(unknown) == 0 && len(missing) == 0 {
		return nil
	}
	var parts []string
	if len(duplicates) > 0 {
		parts = append(parts, fmt.Sprintf("duplicate column(s): %s", strings.Join(duplicates, ", ")))
	}
	if len(unknown) > 0 {
		parts = append(parts, fmt.Sprintf("unknown column(s): %s", strings.Join(unknown, ", ")))
	}
	if len(missing) > 0 {
		parts = append(parts, fmt.Sprintf("missing required column(s): %s", strings.Join(missing, ", ")))
	}
	return &ParseError{Line: 1, Reason: strings.Join(parts, "; ")}
}

// Import is the strict inverse of Export: it reads this package's own
// CSV schema and returns the Channels it describes.
//
// Import is SYNTACTIC ONLY: it turns well-formed cells into
// codeplug.Channel/ChannelData values without judging whether those
// values make sense for any particular radio (e.g. it does not check
// mode against a Capabilities' supported modes, or a tone against
// spec.StandardCTCSSTones). That semantic gate is codeplug.Validate;
// callers must run it before treating an imported codeplug as
// send-ready.
//
// Header: validated against Export's header (see header) — an unknown
// column is an error naming it, a missing required column is an error
// naming it, and "display" is optional and, when present, ignored
// (Import never reads it; it is a convenience column for spreadsheet
// viewing only). Column order in the file does not matter: columns are
// looked up by name.
//
// Rows: every per-row problem is returned as a *ParseError carrying the
// row's 1-based line number (the header is line 1). A leading apostrophe
// on any cell is stripped before parsing (undoing Export's
// formula-injection escaping — see EscapeCell). A row whose slot is
// duplicated (case-sensitive, exact match) is an error naming the slot
// and the line of the first occurrence. A row whose data columns (see
// dataColumns) are ALL empty decodes to an empty Channel (Data == nil);
// otherwise every data column is parsed into ChannelData, with
// ctcss_tone/scan_skip's "" -> Unknown, "n/a" -> Unavailable, value ->
// Known mapping applied exactly as Export produced it.
func Import(r io.Reader) ([]codeplug.Channel, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1

	gotHeader, err := cr.Read()
	if err != nil {
		return nil, &ParseError{Line: 1, Reason: fmt.Sprintf("reading header: %v", err)}
	}
	if err := validateImportHeader(gotHeader); err != nil {
		return nil, err
	}
	colIndex := make(map[string]int, len(gotHeader))
	for i, c := range gotHeader {
		colIndex[c] = i
	}

	var channels []codeplug.Channel
	seenSlots := make(map[string]int) // slot -> first line seen

	line := 1
	for {
		record, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			// No successful record to get a physical line from here —
			// best-effort fallback, one past whatever the last known-good
			// line was.
			line++
			return nil, &ParseError{Line: line, Reason: fmt.Sprintf("reading row: %v", err)}
		}
		// This row's PHYSICAL starting line, from the CSV reader itself
		// (Go 1.17+) — NOT a naive per-record counter: a quoted field
		// containing an embedded newline earlier in the file spans
		// multiple physical lines within a single record, and a counter
		// that only advanced once per record would silently under-report
		// every line number after it.
		line, _ = cr.FieldPos(0)
		if len(record) != len(gotHeader) {
			return nil, &ParseError{Line: line, Reason: fmt.Sprintf("row has %d fields, header has %d", len(record), len(gotHeader))}
		}

		// cell must only be called with a name in requiredColumns:
		// validateImportHeader already guarantees every such name is a
		// key in colIndex, and the length check above guarantees every
		// value in colIndex is a valid index into record.
		cell := func(name string) string {
			return unescapeFormulaCell(record[colIndex[name]])
		}

		slot := cell("slot")
		if slot == "" {
			return nil, &ParseError{Line: line, Reason: "slot must not be empty"}
		}
		if firstLine, dup := seenSlots[slot]; dup {
			return nil, &ParseError{Line: line, Reason: fmt.Sprintf("duplicate slot %q (first seen at line %d)", slot, firstLine)}
		}
		seenSlots[slot] = line

		allEmpty := true
		for _, c := range dataColumns {
			if cell(c) != "" {
				allEmpty = false
				break
			}
		}
		if allEmpty {
			channels = append(channels, codeplug.Channel{Slot: slot})
			continue
		}

		data := codeplug.ChannelData{}

		freqHz, err := strconv.ParseUint(cell("freq_hz"), 10, 32)
		if err != nil {
			return nil, &ParseError{Line: line, Reason: fmt.Sprintf("freq_hz: %v", err)}
		}
		data.FreqHz = uint32(freqHz)

		data.Mode = cell("mode")

		if c := cell("clar_hz"); c != "" {
			clarHz, err := strconv.Atoi(c)
			if err != nil {
				return nil, &ParseError{Line: line, Reason: fmt.Sprintf("clar_hz: %v", err)}
			}
			data.ClarHz = clarHz
		}

		rxClar, err := parseYesEmpty(cell("rx_clar"))
		if err != nil {
			return nil, &ParseError{Line: line, Reason: fmt.Sprintf("rx_clar: %v", err)}
		}
		data.RxClar = rxClar

		txClar, err := parseYesEmpty(cell("tx_clar"))
		if err != nil {
			return nil, &ParseError{Line: line, Reason: fmt.Sprintf("tx_clar: %v", err)}
		}
		data.TxClar = txClar

		data.CTCSS = cell("ctcss")

		toneField, err := parseToneFieldCell(cell("ctcss_tone"))
		if err != nil {
			return nil, &ParseError{Line: line, Reason: err.Error()}
		}
		data.CTCSSTone = toneField

		data.Shift = cell("shift")
		// TrimRight, not the raw cell: a legacy CSV (exported before the
		// tag-normalisation fix, or hand-edited from one) may still carry
		// a space-padded tag exactly as a pre-fix Export/radio read left
		// it. codeplug's canonical tag form is always trimmed (see
		// cat.Dialect.ParseMTAnswer/codeplug.Load's doc comments) —
		// trimming here too keeps a legacy cell importing correctly instead of
		// resurrecting the false verify-mismatch this fix exists to
		// prevent. A fresh export never has trailing spaces to trim, so
		// this is a no-op for any file this package itself wrote.
		data.Tag = strings.TrimRight(cell("tag"), " ")

		tagDisplay, err := parseYesEmpty(cell("tag_display"))
		if err != nil {
			return nil, &ParseError{Line: line, Reason: fmt.Sprintf("tag_display: %v", err)}
		}
		// Interim (M9c-5 task 1): the native CSV still speaks the pre-E1
		// yes/empty spelling, which can only express a value, never a state —
		// so a parsed cell is exactly as known as it was before the flip.
		// Task 4 replaces this with the four-state BoolField spelling.
		data.TagDisplay = codeplug.BoolField{State: codeplug.Known, Value: tagDisplay}

		scanSkip, err := parseBoolFieldCell(cell("scan_skip"))
		if err != nil {
			return nil, &ParseError{Line: line, Reason: err.Error()}
		}
		data.ScanSkip = scanSkip

		channels = append(channels, codeplug.Channel{Slot: slot, Data: &data})
	}

	return channels, nil
}
