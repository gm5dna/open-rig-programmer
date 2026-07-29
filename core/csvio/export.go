// SPDX-License-Identifier: GPL-3.0-or-later

package csvio

import (
	"encoding/csv"
	"fmt"
	"io"
	"regexp"
	"strconv"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
)

// header is this package's own CSV schema, in the exact column order
// Export writes and Import requires (see Import's header-validation
// doc). "display" is the sole optional, import-ignored column.
var header = []string{
	"slot", "display", "freq_hz", "mode", "clar_hz", "rx_clar", "tx_clar",
	"ctcss", "ctcss_tone", "shift", "tag", "tag_display", "scan_skip",
}

// plainSignedInt matches a cell that is nothing but an optionally-signed
// decimal integer, e.g. "-120" or "42" — the one shape EscapeCell must
// NOT escape even though it begins with '+' or '-'.
var plainSignedInt = regexp.MustCompile(`^[+-]?[0-9]+$`)

// EscapeCell guards against CSV/formula injection (OWASP guidance): when
// a CSV file this package writes is opened in a spreadsheet application,
// a cell beginning with '=', '+', '-' or '@' can be interpreted as a
// formula. Any such cell gets a leading apostrophe UNLESS it is a plain
// signed decimal integer (plainSignedInt) — so a tag like "=SUM(A1)" or
// "@cmd" is escaped to "'=SUM(A1)" / "'@cmd", but a clar_hz value like
// "-120" is real data and is left untouched.
//
// A cell that already begins with a literal apostrophe (a legitimate
// value: the apostrophe is in the radio's tag charset) is ALSO escaped, by
// prefixing a second apostrophe — otherwise Import's unescape
// (unescapeFormulaCell, which unconditionally strips exactly one leading
// apostrophe) would strip the data's own apostrophe rather than an
// escape, silently corrupting it on round trip. Prefixing a second
// apostrophe means unescape strips exactly the one escaping added, and
// the data's own leading apostrophe survives.
//
// Schema-neutral (task-34 brief, Codex plan-review F10): exported so
// every CSV export this package (or a caller such as cmd/rigprog's
// "settings" subcommand) writes can apply the identical escaping rule to
// its own free-text columns, rather than each growing its own copy.
// exportRow (this file's memory-channel row path) was the sole caller
// before this export and is refactored onto this exact function,
// behaviour-identically — see TestExport_FormulaInjectionEscaping (still
// green, unchanged fixtures) and TestEscapeCell (export_test.go) for the
// direct unit coverage.
func EscapeCell(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '\'':
		return "'" + s
	case '=', '+', '-', '@':
		if plainSignedInt.MatchString(s) {
			return s
		}
		return "'" + s
	default:
		return s
	}
}

// yesEmpty renders a plain bool as this schema's "yes"/"" convention
// (used for rx_clar, tx_clar, tag_display): empty means no, so sheets
// stay visually clean.
func yesEmpty(b bool) string {
	if b {
		return "yes"
	}
	return ""
}

// exportToneField renders a ToneField as this schema's ctcss_tone
// column: a decimal Hz value with one decimal place when Known (e.g.
// "88.5"), "" when Unknown, "n/a" when Unavailable.
func exportToneField(f codeplug.ToneField) string {
	switch f.State {
	case codeplug.Known:
		return fmt.Sprintf("%.1f", f.Value.Hz())
	case codeplug.Unavailable:
		return "n/a"
	default: // Unknown, or any other value: treat as not-yet-known.
		return ""
	}
}

// exportBoolField renders a BoolField as this schema's scan_skip
// column: "yes"/"no" when Known, "" when Unknown, "n/a" when
// Unavailable.
func exportBoolField(f codeplug.BoolField) string {
	switch f.State {
	case codeplug.Known:
		if f.Value {
			return "yes"
		}
		return "no"
	case codeplug.Unavailable:
		return "n/a"
	default:
		return ""
	}
}

// exportRow builds one CSV row for ch, in header order. An empty channel
// (ch.Empty()) gets a slot-only row: slot and display are filled in,
// every data column is "" — this is what lets a full radio image
// (including its empty slots) round-trip through Export/Import.
func exportRow(ch codeplug.Channel) []string {
	row := make([]string, len(header))
	row[0] = ch.Slot
	row[1] = codeplug.DisplaySlot(ch.Slot)

	if !ch.Empty() {
		d := ch.Data
		row[2] = strconv.FormatUint(uint64(d.FreqHz), 10)
		row[3] = d.Mode
		if d.ClarHz != 0 {
			row[4] = strconv.Itoa(d.ClarHz)
		}
		row[5] = yesEmpty(d.RxClar)
		row[6] = yesEmpty(d.TxClar)
		row[7] = d.CTCSS
		row[8] = exportToneField(d.CTCSSTone)
		row[9] = d.Shift
		row[10] = d.Tag
		// Interim (M9c-5 task 1): still the PRE-E1 yes/empty spelling, over
		// the BoolField's Value alone. Behaviour-preserving by construction —
		// Known-true renders "yes" and Known-false "" exactly as the old bool
		// did. Task 4 replaces this with exportBoolField's four-state
		// spelling, which is the change that alters the column's output.
		row[11] = yesEmpty(d.TagDisplay.Value)
		row[12] = exportBoolField(d.ScanSkip)
	}

	for i, cell := range row {
		row[i] = EscapeCell(cell)
	}
	return row
}

// Export writes channels to w as this package's own, lossless CSV
// schema: one row per slot INCLUDING empty slots (see exportRow), so
// that a full radio image round-trips through Export followed by
// Import. The header is written first, exact and in order (see
// header).
func Export(w io.Writer, channels []codeplug.Channel) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(header); err != nil {
		return fmt.Errorf("csvio: export: writing header: %w", err)
	}
	for _, ch := range channels {
		if err := cw.Write(exportRow(ch)); err != nil {
			return fmt.Errorf("csvio: export: writing slot %q: %w", ch.Slot, err)
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("csvio: export: %w", err)
	}
	return nil
}
