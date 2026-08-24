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

// header is VERSION 1 of this package's own CSV schema: the exact column
// order Export wrote, and Import required, before the Icom tier — and
// still writes and still requires. "display" is the sole optional,
// import-ignored column.
//
// The schema is VERSIONED BY ITS COLUMN SET (design D4), not by a
// version marker cell, and the reason is worth stating because the
// alternative looks tidier: any marker column or header comment would
// change the bytes of every export this program has ever produced, and
// the tier's standing requirement is that the existing FT-710/FTdx
// output stays byte-identical. A column set IS a version — it says
// exactly which fields the file can carry — and it costs nothing on a
// file that carries none of the new ones.
var header = []string{
	"slot", "display", "freq_hz", "mode", "clar_hz", "rx_clar", "tx_clar",
	"ctcss", "ctcss_tone", "shift", "tag", "tag_display", "scan_skip",
}

// tierColumns are the columns VERSION 2 adds, in
// codeplug.ChannelData's own declaration order — one per field the Icom
// tier added to the neutral memory model.
var tierColumns = []string{
	"tx_frequency", "duplex", "offset", "tone_mode", "tone_tx", "tone_rx",
	"dtcs_code", "dtcs_polarity", "filter", "data_mode",
}

// headerV2 is version 2: every version-1 column, unchanged and in
// unchanged order, then the tier's. A version-1 file is therefore a
// prefix of a version-2 one, which is what lets Import accept both by
// looking columns up by name.
var headerV2 = append(append([]string(nil), header...), tierColumns...)

// The reserved cell spellings for a tier column. A tier field is
// TRI-STATE PLUS ABSENT (see codeplug.Absent), so three of the four
// states need a spelling that no value can take:
//
//	""       -> Unknown      (the same spelling ctcss_tone and scan_skip
//	                          have always used)
//	"n/a"    -> Unavailable  (likewise)
//	"absent" -> Absent       (new: the file says nothing about this field)
//
// anything else is a Known value. The reservation is on the RADIO's
// vocabularies: a duplex, tone-mode, DTCS-polarity or filter value
// spelled "n/a" or "absent" would be read back as a state rather than a
// value. No wire vocabulary this project models comes close, and saying
// so here is cheaper than a check nothing would ever fire.
const (
	cellUnavailable = "n/a"
	cellAbsent      = "absent"
)

// exportFieldState renders the three non-Known states, and reports
// whether it handled f. A Known state is the caller's business, since
// only the caller knows how to render its own value.
func exportFieldState(state codeplug.FieldState) (string, bool) {
	switch state {
	case codeplug.Known:
		return "", false
	case codeplug.Unavailable:
		return cellUnavailable, true
	case codeplug.Absent:
		return cellAbsent, true
	default: // Unknown, or any unrecognised state: treat as not-yet-known.
		return "", true
	}
}

// exportFreqField renders a FreqField as a tier column: a plain decimal
// hertz value when Known, and the reserved spellings otherwise.
func exportFreqField(f codeplug.FreqField) string {
	if s, done := exportFieldState(f.State); done {
		return s
	}
	return strconv.FormatUint(f.Value, 10)
}

// exportStringField renders a StringField as a tier column: the
// wire-form vocabulary value when Known, and the reserved spellings
// otherwise.
func exportStringField(f codeplug.StringField) string {
	if s, done := exportFieldState(f.State); done {
		return s
	}
	return f.Value
}

// exportIntField renders an IntField as a tier column.
func exportIntField(f codeplug.IntField) string {
	if s, done := exportFieldState(f.State); done {
		return s
	}
	return strconv.Itoa(f.Value)
}

// exportTierToneField renders a ToneField as a TIER column. It differs
// from exportToneField (the ctcss_tone column) in one way only: it can
// also spell Absent, which the pre-tier column has no state for.
func exportTierToneField(f codeplug.ToneField) string {
	if s, done := exportFieldState(f.State); done {
		return s
	}
	return fmt.Sprintf("%.1f", f.Value.Hz())
}

// exportTierBoolField renders a BoolField as a TIER column, with the
// same one difference from exportBoolField.
func exportTierBoolField(f codeplug.BoolField) string {
	if s, done := exportFieldState(f.State); done {
		return s
	}
	if f.Value {
		return "yes"
	}
	return "no"
}

// tierCells renders one channel's ten tier columns, in tierColumns
// order. An EMPTY slot gets ten empty cells, exactly as it gets an empty
// cell for every other data column — so the "all data cells empty means
// an empty channel" rule Import applies is unchanged.
func tierCells(ch codeplug.Channel) []string {
	if ch.Empty() {
		return make([]string, len(tierColumns))
	}
	d := ch.Data
	return []string{
		exportFreqField(d.TxFreqHz),
		exportStringField(d.Duplex),
		exportFreqField(d.OffsetHz),
		exportStringField(d.ToneMode),
		exportTierToneField(d.ToneTx),
		exportTierToneField(d.ToneRx),
		exportIntField(d.DTCSCode),
		exportStringField(d.DTCSPolarity),
		exportStringField(d.Filter),
		exportTierBoolField(d.DataMode),
	}
}

// needsTierColumns reports whether any channel carries a tier-added
// field whose state RECORDS something (Known or Unknown — see
// codeplug.FieldState.Recorded). It is the CSV exporter's exact analogue
// of core/codeplug's schemaFor, down to treating Unavailable as nothing
// to record, and it is the reason an FT-710 export — every one of whose
// tier fields comes back Unavailable — is byte-identical to what this
// program wrote before the tier.
func needsTierColumns(channels []codeplug.Channel) bool {
	for _, ch := range channels {
		if ch.Empty() {
			continue
		}
		d := ch.Data
		if d.TxFreqHz.State.Recorded() || d.Duplex.State.Recorded() ||
			d.OffsetHz.State.Recorded() || d.ToneMode.State.Recorded() ||
			d.ToneTx.State.Recorded() || d.ToneRx.State.Recorded() ||
			d.DTCSCode.State.Recorded() || d.DTCSPolarity.State.Recorded() ||
			d.Filter.State.Recorded() || d.DataMode.State.Recorded() {
			return true
		}
	}
	return false
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
// (used for rx_clar and tx_clar): empty means no, so sheets stay
// visually clean. It suits a plain bool and nothing else — a field that
// can also be Unknown or Unavailable needs exportBoolField's four
// spellings, which is why tag_display left this convention at M9c-5
// (E1d).
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

// exportBoolField renders a BoolField as this schema's BoolField columns
// (scan_skip, and tag_display since M9c-5's E1d) are spelled: "yes"/"no"
// when Known, "" when Unknown, "n/a" when Unavailable.
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
//
// tier says whether this export is version 2 (see needsTierColumns); a
// version-1 row is built and escaped exactly as it was before the tier
// existed.
func exportRow(ch codeplug.Channel, tier bool) []string {
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
		// M9c-5 (E1d): the four-state BoolField spelling, the same one
		// scan_skip has always used. This CHANGES the column's output for a
		// Known-FALSE display — "" before, "no" now — which is the recorded
		// consequence of the field gaining a state: "" is needed as the
		// spelling for Unknown, and can no longer double as "off". See
		// Import's own doc comment for what that means for a pre-E1 file.
		row[11] = exportBoolField(d.TagDisplay)
		row[12] = exportBoolField(d.ScanSkip)
	}

	if tier {
		row = append(row, tierCells(ch)...)
	}

	for i, cell := range row {
		row[i] = EscapeCell(cell)
	}
	return row
}

// Export writes channels to w as this package's own, lossless CSV
// schema: one row per slot INCLUDING empty slots (see exportRow), so
// that a full radio image round-trips through Export followed by
// Import. The header is written first, exact and in order.
//
// WHICH header depends on the content, by the same lowest-that-can-
// represent-it rule core/codeplug's file writer uses (design D4): the
// version-1 header (see header) while no channel RECORDS a tier-added
// field — i.e. every one of them is Absent or Unavailable
// (FieldState.Recorded, via needsTierColumns) — and the version-2 header
// (headerV2) as soon as one is Known or Unknown. UNAVAILABLE is the case
// that actually decides this for the radios registered today: every read
// of a Yaesu leaves all ten Unavailable, and it is precisely because
// Unavailable is not Recorded that such an export stays version 1. An
// export of any radio registered before the
// Icom tier is therefore byte-identical to what this program produced
// before it, and losslessness holds in both versions — a version-1 file
// is only ever written for content version 1 can hold.
func Export(w io.Writer, channels []codeplug.Channel) error {
	cw := csv.NewWriter(w)
	tier := needsTierColumns(channels)
	head := header
	if tier {
		head = headerV2
	}
	if err := cw.Write(head); err != nil {
		return fmt.Errorf("csvio: export: writing header: %w", err)
	}
	for _, ch := range channels {
		if err := cw.Write(exportRow(ch, tier)); err != nil {
			return fmt.Errorf("csvio: export: writing slot %q: %w", ch.Slot, err)
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("csvio: export: %w", err)
	}
	return nil
}
