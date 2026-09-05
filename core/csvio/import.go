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
//
// Both appended column groups join the list. An absent column reads as
// "", so older versions and newer empty rows retain the same rule.
var dataColumns = func() []string {
	cols := []string{
		"freq_hz", "mode", "clar_hz", "rx_clar", "tx_clar",
		"ctcss", "ctcss_tone", "shift", "tag", "tag_display", "scan_skip",
	}
	cols = append(cols, tierColumns...)
	return append(cols, receiverColumns...)
}()

// requiredColumns lists every column Import requires to be present in
// the header. It is every VERSION-1 column except "display", which is
// optional and ignored (see Import).
//
// Added columns are deliberately NOT required: that is exactly what
// makes Import accept all three versions (design D4/D8).
var requiredColumns = func() []string {
	cols := make([]string, 0, len(header)-1)
	for _, c := range header {
		if c != "display" {
			cols = append(cols, c)
		}
	}
	return cols
}()

// knownColumnSet is every column Import recognises across all three
// header versions, "display" included.
var knownColumnSet = func() map[string]bool {
	set := make(map[string]bool, len(headerV3))
	for _, c := range headerV3 {
		set[c] = true
	}
	return set
}()

// parseTierState maps a tier column's reserved spellings onto their
// FieldStates, reporting whether it recognised one. A cell it does not
// recognise is a Known value, which only the caller can parse.
//
// See export.go's cellUnavailable/cellAbsent for the spellings and the
// vocabulary reservation they imply.
func parseTierState(s string) (codeplug.FieldState, bool) {
	switch s {
	case "":
		return codeplug.Unknown, true
	case cellUnavailable:
		return codeplug.Unavailable, true
	case cellAbsent:
		return codeplug.Absent, true
	default:
		return "", false
	}
}

// parseFreqFieldCell parses a tier frequency column (tx_frequency,
// offset): the reserved spellings, or a plain decimal hertz value.
func parseFreqFieldCell(s, column string) (codeplug.FreqField, error) {
	if state, ok := parseTierState(s); ok {
		return codeplug.FreqField{State: state}, nil
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return codeplug.FreqField{}, fmt.Errorf("%s must be \"\", %q, %q or a whole number of hertz, got %q", column, cellUnavailable, cellAbsent, s)
	}
	return codeplug.FreqField{State: codeplug.Known, Value: v}, nil
}

// parseStringFieldCell parses a tier vocabulary column (duplex,
// tone_mode, dtcs_polarity, filter). It is SYNTACTIC only, like every
// other cell parser here: whether the value is in this radio's
// vocabulary is codeplug.Validate's question.
func parseStringFieldCell(s string) codeplug.StringField {
	if state, ok := parseTierState(s); ok {
		return codeplug.StringField{State: state}
	}
	return codeplug.StringField{State: codeplug.Known, Value: s}
}

// parseIntFieldCell parses a tier integer column (dtcs_code).
func parseIntFieldCell(s, column string) (codeplug.IntField, error) {
	if state, ok := parseTierState(s); ok {
		return codeplug.IntField{State: state}, nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return codeplug.IntField{}, fmt.Errorf("%s must be \"\", %q, %q or a whole number, got %q", column, cellUnavailable, cellAbsent, s)
	}
	return codeplug.IntField{State: codeplug.Known, Value: v}, nil
}

// parseTierToneFieldCell parses a tier tone column (tone_tx, tone_rx).
// It differs from parseToneFieldCell (the ctcss_tone column) in one way
// only: it also recognises the Absent spelling, which the pre-tier
// column has no state for.
func parseTierToneFieldCell(s, column string) (codeplug.ToneField, error) {
	if state, ok := parseTierState(s); ok {
		return codeplug.ToneField{State: state}, nil
	}
	deciHz, err := parseExactToneDeciHz(s)
	if err != nil {
		return codeplug.ToneField{}, fmt.Errorf("%s must be \"\", %q, %q or a decimal Hz value with at most one decimal place, got %q", column, cellUnavailable, cellAbsent, s)
	}
	return codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(deciHz)}, nil
}

// parseTierBoolFieldCell parses a tier boolean column (data_mode), with
// the same one difference from parseBoolFieldCell.
func parseTierBoolFieldCell(s, column string) (codeplug.BoolField, error) {
	if state, ok := parseTierState(s); ok {
		return codeplug.BoolField{State: state}, nil
	}
	switch s {
	case "yes":
		return codeplug.BoolField{State: codeplug.Known, Value: true}, nil
	case "no":
		return codeplug.BoolField{State: codeplug.Known, Value: false}, nil
	default:
		return codeplug.BoolField{}, fmt.Errorf("%s must be \"\", %q, %q, \"yes\" or \"no\", got %q", column, cellUnavailable, cellAbsent, s)
	}
}

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

// parseBoolFieldCell parses one of this schema's BoolField columns: "" ->
// Unknown, "n/a" -> Unavailable, "yes"/"no" -> Known. column is the
// column's name, used ONLY for the diagnostic — it was hardcoded
// "scan_skip" while scan_skip was the sole BoolField column, and became a
// parameter at M9c-5 (E1d) when tag_display joined it, so that a bad
// tag_display cell is never reported as a scan_skip problem.
func parseBoolFieldCell(s, column string) (codeplug.BoolField, error) {
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
		return codeplug.BoolField{}, fmt.Errorf("%s must be \"\", \"n/a\", \"yes\" or \"no\", got %q", column, s)
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
// Encoding: a single leading UTF-8 byte-order mark is dropped before
// anything else (skipUTF8BOM), so a file saved by Excel as "CSV UTF-8"
// imports like any other; CRLF line endings need no handling of their own,
// encoding/csv drops the CR itself. Nothing else about the encoding is
// interpreted. Export writes neither a BOM nor a CR.
//
// Header: validated against Export's header — an unknown column is an
// error naming it, a missing required column is an error naming it, and
// "display" is optional and, when present, ignored (Import never reads
// it; it is a convenience column for spreadsheet viewing only). Column
// order in the file does not matter: columns are looked up by name.
//
// ALL THREE HEADER VERSIONS are accepted (design D4/D8). The schema is versioned
// by its column set: version 1 is the thirteen columns this package has
// always written (header), version 2 appends D4's ten fields, and version
// 3 appends D8's seven. Only version 1's columns are
// REQUIRED, so a version-1 file — every file this program wrote before
// the Icom tier — imports unchanged; the tier columns are optional and
// recognised, so later files have them read. A version-1 file's seventeen
// added fields come back Unavailable, not at the zero value: see
// markTierFieldsUnavailable for why that distinction is load-bearing
// rather than cosmetic.
//
// Rows: every per-row problem is returned as a *ParseError carrying the
// row's 1-based line number (the header is line 1). A leading apostrophe
// on any cell is stripped before parsing (undoing Export's
// formula-injection escaping — see EscapeCell). A row whose slot is
// duplicated (case-sensitive, exact match) is an error naming the slot
// and the line of the first occurrence. A row whose data columns (see
// dataColumns) are ALL empty decodes to an empty Channel (Data == nil);
// otherwise every data column is parsed into ChannelData, with
// ctcss_tone/scan_skip/tag_display's "" -> Unknown, "n/a" -> Unavailable,
// value -> Known mapping applied exactly as Export produced it. A tier
// column adds one spelling to that mapping, "absent" -> Absent, for the
// state those fields have that the pre-tier ones do not.
//
// tag_display joined that mapping at M9c-5 (E1d), and the change is not
// backward-compatible in ONE direction, recorded here because a user can
// meet it: a CSV exported BEFORE that milestone wrote Known-false as an
// EMPTY cell (the old spelling had only "yes" and ""), so re-importing
// such a file yields Unknown rather than Known-false, and
// codeplug.Diff then blocks those channels until the display is set. The
// mitigation is to put an explicit "no" (or "yes") in the column before
// importing the old file, or to set the value in the UI afterwards; every
// export from this version onwards writes an explicit spelling, so the
// reinterpretation can only bite pre-E1 files. See
// TestImport_PreE1EmptyTagDisplayCell_ReinterpretedAsUnknown.
func Import(r io.Reader) ([]codeplug.Channel, error) {
	// A leading UTF-8 BOM is dropped before the CSV reader sees it —
	// otherwise it sticks to the first header cell and this file is
	// rejected for a column it plainly has. See skipUTF8BOM.
	cr := csv.NewReader(skipUTF8BOM(r))
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
	// Which header version this file is: version 2 iff it carries any of
	// the tier columns. Partial adoption (some tier columns, not all) is
	// accepted for the same reason column ORDER is: columns are looked up
	// by name, an absent one reads as "", and a hand-edited file that
	// carries only the column its author cared about is more useful
	// accepted than refused.
	hasTier := false
	for _, c := range tierColumns {
		if _, ok := colIndex[c]; ok {
			hasTier = true
			break
		}
	}
	hasReceiver := false
	for _, c := range receiverColumns {
		if _, ok := colIndex[c]; ok {
			hasReceiver = true
			break
		}
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

		// A column absent from this file's header reads as "" — which is
		// what makes ONE row parser serve both header versions (design
		// D4). The ok test is load-bearing rather than defensive: a bare
		// map lookup yields index 0 for a missing name, so an absent
		// column would silently read back the SLOT cell of every row.
		// For a name in requiredColumns the lookup always succeeds
		// (validateImportHeader guarantees it), and the length check
		// above guarantees every index colIndex does hold is in range.
		cell := func(name string) string {
			i, ok := colIndex[name]
			if !ok {
				return ""
			}
			return unescapeFormulaCell(record[i])
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

		// 64, not 32 (design D4, round 2 C8/F11): this was a hard
		// 32-bit parse, which would have refused every frequency above
		// 4.29 GHz — a range the neutral model now reaches. The bound
		// here is the REPRESENTABLE one; whether a frequency is one the
		// target radio can store is codeplug.Validate's question, asked
		// against that radio's own MinFreqHz/MaxFreqHz.
		freqHz, err := strconv.ParseUint(cell("freq_hz"), 10, 64)
		if err != nil {
			return nil, &ParseError{Line: line, Reason: fmt.Sprintf("freq_hz: %v", err)}
		}
		data.FreqHz = freqHz

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

		tagDisplay, err := parseBoolFieldCell(cell("tag_display"), "tag_display")
		if err != nil {
			return nil, &ParseError{Line: line, Reason: err.Error()}
		}
		data.TagDisplay = tagDisplay

		scanSkip, err := parseBoolFieldCell(cell("scan_skip"), "scan_skip")
		if err != nil {
			return nil, &ParseError{Line: line, Reason: err.Error()}
		}
		data.ScanSkip = scanSkip

		// The tier columns. In a VERSION-1 file none of them is in the
		// header, so every cell() below reads "" — and "" would parse as
		// Unknown, "this radio has the field and we have not read it",
		// which is the wrong answer for a file that has no such column at
		// all. hasTier is what keeps the two apart: without the columns
		// the ten are set to UNAVAILABLE (markTierFieldsUnavailable —
		// design D4, decision 1, documented at that function), the state
		// every producer in this project gives a field the radio does not
		// have, and the one that leaves an imported channel comparing
		// equal to a read of the same radio instead of modified in ten
		// fields the file never mentioned.
		if hasReceiver {
			if err := parseTierCells(&data, cell); err != nil {
				return nil, &ParseError{Line: line, Reason: err.Error()}
			}
			if err := parseReceiverCells(&data, cell); err != nil {
				return nil, &ParseError{Line: line, Reason: err.Error()}
			}
		} else if hasTier {
			if err := parseTierCells(&data, cell); err != nil {
				return nil, &ParseError{Line: line, Reason: err.Error()}
			}
			markReceiverFieldsUnavailable(&data)
		} else {
			markTierFieldsUnavailable(&data)
		}

		channels = append(channels, codeplug.Channel{Slot: slot, Data: &data})
	}

	return channels, nil
}

// parseTierCells fills the ten tier-added fields of data from a
// version-2 row, using cell to look each column up by name. A tier
// column absent from a partially-adopted version-2 header reads as "",
// i.e. Unknown — which is the right answer there, since the file DOES
// declare itself version 2 and simply leaves that field unstated.
//
// The first error stops the row: as everywhere in this importer, a cell
// that cannot be understood is refused, never guessed at.
func parseTierCells(data *codeplug.ChannelData, cell func(string) string) error {
	txFreq, err := parseFreqFieldCell(cell("tx_frequency"), "tx_frequency")
	if err != nil {
		return err
	}
	data.TxFreqHz = txFreq

	data.Duplex = parseStringFieldCell(cell("duplex"))

	offset, err := parseFreqFieldCell(cell("offset"), "offset")
	if err != nil {
		return err
	}
	data.OffsetHz = offset

	data.ToneMode = parseStringFieldCell(cell("tone_mode"))

	toneTx, err := parseTierToneFieldCell(cell("tone_tx"), "tone_tx")
	if err != nil {
		return err
	}
	data.ToneTx = toneTx

	toneRx, err := parseTierToneFieldCell(cell("tone_rx"), "tone_rx")
	if err != nil {
		return err
	}
	data.ToneRx = toneRx

	dtcsCode, err := parseIntFieldCell(cell("dtcs_code"), "dtcs_code")
	if err != nil {
		return err
	}
	data.DTCSCode = dtcsCode

	data.DTCSPolarity = parseStringFieldCell(cell("dtcs_polarity"))
	data.Filter = parseStringFieldCell(cell("filter"))

	dataMode, err := parseTierBoolFieldCell(cell("data_mode"), "data_mode")
	if err != nil {
		return err
	}
	data.DataMode = dataMode

	return nil
}

func parseReceiverCells(data *codeplug.ChannelData, cell func(string) string) error {
	var err error
	data.TuningStepEnabled, err = parseTierBoolFieldCell(cell("tuning_step_enabled"), "tuning_step_enabled")
	if err != nil {
		return err
	}
	data.TuningStep = parseStringFieldCell(cell("tuning_step"))
	data.ProgramTuningStepHz, err = parseFreqFieldCell(cell("program_tuning_step"), "program_tuning_step")
	if err != nil {
		return err
	}
	data.AttenuatorDB, err = parseIntFieldCell(cell("attenuator"), "attenuator")
	if err != nil {
		return err
	}
	data.Preamp = parseStringFieldCell(cell("preamp"))
	data.Antenna = parseStringFieldCell(cell("antenna"))
	data.IPPlus, err = parseTierBoolFieldCell(cell("ip_plus"), "ip_plus")
	return err
}

// markTierFieldsUnavailable sets all seventeen added fields Unavailable
// for a VERSION-1 file, which has neither appended column group.
//
// It is the CSV importer's exact counterpart to core/codeplug's
// migrateV3ChannelData, and it exists for the same reason, which is
// worth stating because the zero value looks like the safer answer. A
// version-1 CSV was written by a build that modelled none of these
// fields, for a radio that has none of them, so "this radio has no such
// field" is what the file says by having no column for it — and it is
// what a read of such a radio reports. Leaving them at the zero value
// (Absent) would make a CSV-imported channel differ, field for field,
// from the very baseline it is about to be diffed against, and
// codeplug.Diff compares ChannelData with ==: every channel of every
// import would come back "modified".
//
// A version-2 file takes the other branch and gets what its cells
// spell, including the explicit "absent" spelling — a file that DOES
// have the column and says nothing in it is a different statement from
// a file with no column at all.
func markTierFieldsUnavailable(data *codeplug.ChannelData) {
	data.TxFreqHz = codeplug.FreqField{State: codeplug.Unavailable}
	data.Duplex = codeplug.StringField{State: codeplug.Unavailable}
	data.OffsetHz = codeplug.FreqField{State: codeplug.Unavailable}
	data.ToneMode = codeplug.StringField{State: codeplug.Unavailable}
	data.ToneTx = codeplug.ToneField{State: codeplug.Unavailable}
	data.ToneRx = codeplug.ToneField{State: codeplug.Unavailable}
	data.DTCSCode = codeplug.IntField{State: codeplug.Unavailable}
	data.DTCSPolarity = codeplug.StringField{State: codeplug.Unavailable}
	data.Filter = codeplug.StringField{State: codeplug.Unavailable}
	data.DataMode = codeplug.BoolField{State: codeplug.Unavailable}
	markReceiverFieldsUnavailable(data)
}

func markReceiverFieldsUnavailable(data *codeplug.ChannelData) {
	data.TuningStepEnabled = codeplug.BoolField{State: codeplug.Unavailable}
	data.TuningStep = codeplug.StringField{State: codeplug.Unavailable}
	data.ProgramTuningStepHz = codeplug.FreqField{State: codeplug.Unavailable}
	data.AttenuatorDB = codeplug.IntField{State: codeplug.Unavailable}
	data.Preamp = codeplug.StringField{State: codeplug.Unavailable}
	data.Antenna = codeplug.StringField{State: codeplug.Unavailable}
	data.IPPlus = codeplug.BoolField{State: codeplug.Unavailable}
}
