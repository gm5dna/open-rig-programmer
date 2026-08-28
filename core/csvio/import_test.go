// SPDX-License-Identifier: GPL-3.0-or-later

package csvio

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// channelsEqual reports whether a and b hold the same slots, in the same
// order, with equal Data (nil-ness and, when populated, value).
func channelsEqual(t *testing.T, a, b []codeplug.Channel) bool {
	t.Helper()
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Slot != b[i].Slot {
			return false
		}
		if a[i].Empty() != b[i].Empty() {
			return false
		}
		if !a[i].Empty() && *a[i].Data != *b[i].Data {
			return false
		}
	}
	return true
}

// yaesuTier sets all seventeen fields the Icom model extensions added to
// Unavailable and returns d — what a read of any registered radio
// reports, what a load of a pre-tier codeplug migrates to, and what a
// VERSION-1 CSV import produces (markTierFieldsUnavailable). The
// round-trip fixtures below wrap their channel data in it so they state
// the same thing every real producer does; the zero value would make
// them the only ChannelData in the project claiming that nobody ever
// spoke about these fields for an FT-710.
func yaesuTier(d *codeplug.ChannelData) *codeplug.ChannelData {
	d.TxFreqHz = codeplug.FreqField{State: codeplug.Unavailable}
	d.Duplex = codeplug.StringField{State: codeplug.Unavailable}
	d.OffsetHz = codeplug.FreqField{State: codeplug.Unavailable}
	d.ToneMode = codeplug.StringField{State: codeplug.Unavailable}
	d.ToneTx = codeplug.ToneField{State: codeplug.Unavailable}
	d.ToneRx = codeplug.ToneField{State: codeplug.Unavailable}
	d.DTCSCode = codeplug.IntField{State: codeplug.Unavailable}
	d.DTCSPolarity = codeplug.StringField{State: codeplug.Unavailable}
	d.Filter = codeplug.StringField{State: codeplug.Unavailable}
	d.DataMode = codeplug.BoolField{State: codeplug.Unavailable}
	d.TuningStepEnabled = codeplug.BoolField{State: codeplug.Unavailable}
	d.TuningStep = codeplug.StringField{State: codeplug.Unavailable}
	d.ProgramTuningStepHz = codeplug.FreqField{State: codeplug.Unavailable}
	d.AttenuatorDB = codeplug.IntField{State: codeplug.Unavailable}
	d.Preamp = codeplug.StringField{State: codeplug.Unavailable}
	d.Antenna = codeplug.StringField{State: codeplug.Unavailable}
	d.IPPlus = codeplug.BoolField{State: codeplug.Unavailable}
	return d
}

// fullImage is a mixed radio image exercising every field-state
// combination this schema supports, plus an interleaved empty slot, used
// by the round-trip tests below. Since M9c-5 task 4 (E1d) that includes
// all FOUR tag_display states — Known-true (001), Known-false (003),
// Unknown (007) and Unavailable (501) — which is only round-trippable at
// all now the column speaks the BoolField spelling.
func fullImage() []codeplug.Channel {
	return []codeplug.Channel{
		{
			Slot: "001",
			Data: yaesuTier(&codeplug.ChannelData{
				FreqHz:     14250000,
				Mode:       "USB",
				ClarHz:     -120,
				RxClar:     true,
				TxClar:     true,
				CTCSS:      "ENC-DEC",
				CTCSSTone:  codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(885)},
				Shift:      "PLUS",
				Tag:        "MB9XYZ",
				TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: true},
				ScanSkip:   codeplug.BoolField{State: codeplug.Known, Value: true},
			}),
		},
		{Slot: "002"}, // empty slot, interleaved
		{
			Slot: "003",
			Data: yaesuTier(&codeplug.ChannelData{
				FreqHz:     14300000,
				Mode:       "LSB",
				CTCSS:      "OFF",
				CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
				Shift:      "SIMPLEX",
				Tag:        "NET",
				TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false},
				ScanSkip:   codeplug.BoolField{State: codeplug.Known, Value: false},
			}),
		},
		{
			Slot: "007",
			Data: yaesuTier(&codeplug.ChannelData{
				FreqHz:     10118000,
				Mode:       "CW-U",
				CTCSS:      "OFF",
				CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
				Shift:      "SIMPLEX",
				Tag:        "WSPR",
				TagDisplay: codeplug.BoolField{State: codeplug.Unknown},
				ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
			}),
		},
		{
			Slot: "501",
			Data: yaesuTier(&codeplug.ChannelData{
				FreqHz:     5330500,
				Mode:       "AM",
				ClarHz:     370,
				CTCSS:      "OFF",
				CTCSSTone:  codeplug.ToneField{State: codeplug.Unavailable},
				Shift:      "SIMPLEX",
				TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
				ScanSkip:   codeplug.BoolField{State: codeplug.Unavailable},
			}),
		},
		{Slot: "099"}, // empty slot, trailing
	}
}

// TestExportImport_FullImageRoundTrip is the load-bearing round-trip
// test: every field state, an interleaved empty slot, and a trailing
// empty slot must all survive Export->Import unchanged.
func TestExportImport_FullImageRoundTrip(t *testing.T) {
	want := fullImage()
	var buf bytes.Buffer
	if err := Export(&buf, want); err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	got, err := Import(&buf)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if !channelsEqual(t, got, want) {
		t.Errorf("round trip mismatch:\n got  = %+v\n want = %+v", got, want)
	}
}

// TestExportImport_ApostropheTagRoundTrip covers a tag whose first byte
// is a legitimate apostrophe (legal per the FT-710 tag charset): Export
// must double-escape it (to two leading apostrophes) so that Import's
// unconditional single-apostrophe strip (unescapeFormulaCell) returns
// exactly the original tag rather than silently dropping the leading
// apostrophe.
func TestExportImport_ApostropheTagRoundTrip(t *testing.T) {
	cases := []struct {
		name     string
		tag      string
		wantCell string
	}{
		{"apostrophe-prefixed tag", "'TWAS", "''TWAS"},
		{"tag that is just an apostrophe", "'", "''"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := *yaesuTier(&codeplug.ChannelData{
				FreqHz:     14250000,
				Mode:       "USB",
				CTCSS:      "OFF",
				CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
				Shift:      "SIMPLEX",
				Tag:        tc.tag,
				TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false},
				ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
			})
			want := []codeplug.Channel{{Slot: "001", Data: &d}}

			var buf bytes.Buffer
			if err := Export(&buf, want); err != nil {
				t.Fatalf("Export() error = %v", err)
			}

			rows := readCSV(t, buf.Bytes())
			gotCell := rows[1][10] // tag column index
			if gotCell != tc.wantCell {
				t.Errorf("exported tag cell = %q, want %q", gotCell, tc.wantCell)
			}

			got, err := Import(bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Fatalf("Import() error = %v", err)
			}
			if !channelsEqual(t, got, want) {
				t.Errorf("round trip mismatch:\n got  = %+v\n want = %+v", got, want)
			}
			if got[0].Data.Tag != tc.tag {
				t.Errorf("Tag = %q, want %q (byte-identical round trip)", got[0].Data.Tag, tc.tag)
			}
		})
	}
}

// TestImport_TrimsLegacyPaddedTagCell: a CSV cell carrying a
// space-padded tag — as a hand-edited file, or a file exported before
// the Fix (tag normalisation), might still contain (this package's own
// Export never pads a tag; see TestExportImport_FullImageRoundTrip) — is
// trimmed on Import, so re-importing such a legacy cell never
// resurrects the padding-induced verify-mismatch this fix exists to
// prevent (core/clone's write-verify compares codeplug.ChannelData.Tag
// byte-for-byte).
func TestImport_TrimsLegacyPaddedTagCell(t *testing.T) {
	want := codeplug.ChannelData{
		FreqHz:     14250000,
		Mode:       "USB",
		CTCSS:      "OFF",
		CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
		Shift:      "SIMPLEX",
		Tag:        "BBC ANT 3",
		TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false},
		ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
	}
	var buf bytes.Buffer
	if err := Export(&buf, []codeplug.Channel{{Slot: "001", Data: &want}}); err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	// Simulate a legacy/hand-padded cell: splice trailing spaces onto
	// the tag Export itself wrote unpadded.
	padded := strings.Replace(buf.String(), "BBC ANT 3", "BBC ANT 3   ", 1)

	got, err := Import(strings.NewReader(padded))
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if len(got) != 1 || got[0].Data == nil || got[0].Data.Tag != "BBC ANT 3" {
		t.Fatalf("Import(legacy padded tag cell) = %+v, want Tag %q (trimmed)", got, "BBC ANT 3")
	}
}

// TestImport_HeaderValidation covers unknown-column and
// missing-required-column errors, and confirms "display" is optional.
func TestImport_HeaderValidation(t *testing.T) {
	cases := []struct {
		name      string
		header    string
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "exact header",
			header:  "slot,display,freq_hz,mode,clar_hz,rx_clar,tx_clar,ctcss,ctcss_tone,shift,tag,tag_display,scan_skip",
			wantErr: false,
		},
		{
			name:    "display column omitted is fine",
			header:  "slot,freq_hz,mode,clar_hz,rx_clar,tx_clar,ctcss,ctcss_tone,shift,tag,tag_display,scan_skip",
			wantErr: false,
		},
		{
			name:      "unknown column",
			header:    "slot,display,freq_hz,mode,clar_hz,rx_clar,tx_clar,ctcss,ctcss_tone,shift,tag,tag_display,scan_skip,bogus",
			wantErr:   true,
			errSubstr: "bogus",
		},
		{
			name:      "missing required column",
			header:    "slot,display,freq_hz,mode,clar_hz,rx_clar,tx_clar,ctcss,ctcss_tone,shift,tag_display,scan_skip", // tag missing
			wantErr:   true,
			errSubstr: "tag",
		},
		{
			name:      "duplicate column",
			header:    "slot,slot,freq_hz,mode,clar_hz,rx_clar,tx_clar,ctcss,ctcss_tone,shift,tag,tag_display,scan_skip",
			wantErr:   true,
			errSubstr: "duplicate",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Import(strings.NewReader(tc.header + "\n"))
			if tc.wantErr && err == nil {
				t.Fatalf("Import() error = nil, want error containing %q", tc.errSubstr)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Import() error = %v, want nil", err)
			}
			if tc.wantErr && !strings.Contains(err.Error(), tc.errSubstr) {
				t.Errorf("Import() error = %q, want substring %q", err.Error(), tc.errSubstr)
			}
		})
	}
}

// header used by the row-level test cases below (matches Export's
// header exactly).
const testHeader = "slot,display,freq_hz,mode,clar_hz,rx_clar,tx_clar,ctcss,ctcss_tone,shift,tag,tag_display,scan_skip\n"

// TestImport_RowErrors_LineNumbers covers per-row parse errors, each
// checked for the correct 1-based line number.
func TestImport_RowErrors_LineNumbers(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantLine int
	}{
		{
			name:     "bad freq_hz",
			body:     "001,,notanumber,USB,,,,OFF,,SIMPLEX,TAG,,\n",
			wantLine: 2,
		},
		{
			name:     "bad clar_hz",
			body:     "001,,14250000,USB,notanumber,,,OFF,,SIMPLEX,TAG,,\n",
			wantLine: 2,
		},
		{
			name:     "bad rx_clar",
			body:     "001,,14250000,USB,,maybe,,OFF,,SIMPLEX,TAG,,\n",
			wantLine: 2,
		},
		{
			name:     "bad tx_clar",
			body:     "001,,14250000,USB,,,maybe,OFF,,SIMPLEX,TAG,,\n",
			wantLine: 2,
		},
		{
			name:     "bad tag_display",
			body:     "001,,14250000,USB,,,,OFF,,SIMPLEX,TAG,maybe,\n",
			wantLine: 2,
		},
		{
			name:     "row has too few fields",
			body:     "001,,14250000,USB,,,,OFF,,SIMPLEX,TAG\n",
			wantLine: 2,
		},
		{
			name:     "bad ctcss_tone",
			body:     "001,,14250000,USB,,,,ENC,notanumber,SIMPLEX,TAG,,\n",
			wantLine: 2,
		},
		{
			name:     "bad scan_skip",
			body:     "001,,14250000,USB,,,,OFF,,SIMPLEX,TAG,,maybe\n",
			wantLine: 2,
		},
		{
			name:     "empty slot",
			body:     ",,14250000,USB,,,,OFF,,SIMPLEX,TAG,,\n",
			wantLine: 2,
		},
		{
			name:     "error on second data row",
			body:     "001,,14250000,USB,,,,OFF,,SIMPLEX,TAG,,\n002,,notanumber,USB,,,,OFF,,SIMPLEX,TAG,,\n",
			wantLine: 3,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Import(strings.NewReader(testHeader + tc.body))
			if err == nil {
				t.Fatalf("Import() error = nil, want error")
			}
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("Import() error = %T, want *ParseError", err)
			}
			if pe.Line != tc.wantLine {
				t.Errorf("ParseError.Line = %d, want %d (err=%v)", pe.Line, tc.wantLine, err)
			}
		})
	}
}

// TestImport_DuplicateHeaderColumn covers the duplicate-header-column
// error directly: a typed *ParseError at line 1, naming the duplicate.
func TestImport_DuplicateHeaderColumn(t *testing.T) {
	header := "slot,display,freq_hz,mode,clar_hz,rx_clar,tx_clar,ctcss,ctcss_tone,ctcss_tone,shift,tag,tag_display,scan_skip"
	_, err := Import(strings.NewReader(header + "\n"))
	if err == nil {
		t.Fatal("Import() error = nil, want error for duplicate header column")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("Import() error = %T, want *ParseError", err)
	}
	if pe.Line != 1 {
		t.Errorf("ParseError.Line = %d, want 1", pe.Line)
	}
	if !strings.Contains(err.Error(), "ctcss_tone") {
		t.Errorf("Import() error = %q, want it to name the duplicate column ctcss_tone", err.Error())
	}
}

// TestImport_DuplicateSlot covers the duplicate-slot error.
func TestImport_DuplicateSlot(t *testing.T) {
	body := "001,,14250000,USB,,,,OFF,,SIMPLEX,TAG,,\n001,,14300000,LSB,,,,OFF,,SIMPLEX,TAG2,,\n"
	_, err := Import(strings.NewReader(testHeader + body))
	if err == nil {
		t.Fatal("Import() error = nil, want duplicate-slot error")
	}
	if !strings.Contains(err.Error(), "001") {
		t.Errorf("Import() error = %q, want it to name the duplicate slot", err.Error())
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("Import() error = %T, want *ParseError", err)
	}
	if pe.Line != 3 {
		t.Errorf("ParseError.Line = %d, want 3 (the second occurrence)", pe.Line)
	}
}

// TestImport_ApostropheUnescape covers undoing Export's formula-injection
// escaping: a leading apostrophe is stripped from tag; clar_hz's own
// signed decimal ("-120", never escaped by Export) parses normally.
func TestImport_ApostropheUnescape(t *testing.T) {
	cases := []struct {
		name    string
		tagCell string
		wantTag string
	}{
		{"escaped formula", "'=SUM(A1)", "=SUM(A1)"},
		{"escaped at-command", "'@cmd", "@cmd"},
		{"escaped plus", "'+notanumber", "+notanumber"},
		{"unescaped ordinary tag", "MB9XYZ", "MB9XYZ"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := "001,,14250000,USB,-120,,,OFF,,SIMPLEX," + tc.tagCell + ",,\n"
			got, err := Import(strings.NewReader(testHeader + body))
			if err != nil {
				t.Fatalf("Import() error = %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("Import() = %d channels, want 1", len(got))
			}
			if got[0].Data.Tag != tc.wantTag {
				t.Errorf("Tag = %q, want %q", got[0].Data.Tag, tc.wantTag)
			}
			if got[0].Data.ClarHz != -120 {
				t.Errorf("ClarHz = %d, want -120 (plain signed decimal, never escaped)", got[0].Data.ClarHz)
			}
		})
	}
}

// TestImport_FieldStateMapping covers the "" -> Unknown, "n/a" ->
// Unavailable, value -> Known mapping for ctcss_tone and scan_skip, both
// directions (via the value produced by Import).
func TestImport_FieldStateMapping(t *testing.T) {
	cases := []struct {
		name          string
		toneCell      string
		skipCell      string
		wantToneState codeplug.FieldState
		wantSkipState codeplug.FieldState
	}{
		{"empty -> Unknown", "", "", codeplug.Unknown, codeplug.Unknown},
		{"n/a -> Unavailable", "n/a", "n/a", codeplug.Unavailable, codeplug.Unavailable},
		{"value -> Known", "88.5", "yes", codeplug.Known, codeplug.Known},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctcss := "OFF"
			if tc.toneCell != "" {
				ctcss = "ENC"
			}
			body := "001,,14250000,USB,,,," + ctcss + "," + tc.toneCell + ",SIMPLEX,TAG,," + tc.skipCell + "\n"
			got, err := Import(strings.NewReader(testHeader + body))
			if err != nil {
				t.Fatalf("Import() error = %v", err)
			}
			if got[0].Data.CTCSSTone.State != tc.wantToneState {
				t.Errorf("CTCSSTone.State = %v, want %v", got[0].Data.CTCSSTone.State, tc.wantToneState)
			}
			if got[0].Data.ScanSkip.State != tc.wantSkipState {
				t.Errorf("ScanSkip.State = %v, want %v", got[0].Data.ScanSkip.State, tc.wantSkipState)
			}
		})
	}
}

// TestImport_TagDisplayFourStates is E1d's import side: the tag_display
// column now speaks the same four spellings scan_skip does, and each maps
// to exactly one BoolField. "no" is the spelling that did not exist
// before (Known-false used to be ""), and "" now means Unknown rather
// than Known-false — see
// TestImport_PreE1EmptyTagDisplayCell_ReinterpretedAsUnknown.
func TestImport_TagDisplayFourStates(t *testing.T) {
	cases := []struct {
		name      string
		cell      string
		wantState codeplug.FieldState
		wantValue bool
	}{
		{"yes -> Known true", "yes", codeplug.Known, true},
		{"no -> Known false", "no", codeplug.Known, false},
		{"empty -> Unknown", "", codeplug.Unknown, false},
		{"n/a -> Unavailable", "n/a", codeplug.Unavailable, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := "001,,14250000,USB,,,,OFF,,SIMPLEX,TAG," + tc.cell + ",\n"
			got, err := Import(strings.NewReader(testHeader + body))
			if err != nil {
				t.Fatalf("Import() error = %v", err)
			}
			if len(got) != 1 || got[0].Data == nil {
				t.Fatalf("Import() = %+v, want exactly one populated channel", got)
			}
			gotField := got[0].Data.TagDisplay
			if gotField.State != tc.wantState || gotField.Value != tc.wantValue {
				t.Errorf("tag_display cell %q -> %+v, want {State:%v Value:%v}", tc.cell, gotField, tc.wantState, tc.wantValue)
			}
		})
	}
}

// TestImport_PreE1EmptyTagDisplayCell_ReinterpretedAsUnknown is THE
// RECORDED REINTERPRETATION of M9c-5 E1d, kept as an executable statement
// of it rather than prose alone.
//
// A CSV exported BEFORE this milestone wrote Known-false as an EMPTY
// tag_display cell (the old yes/empty spelling had no third symbol). The
// same file re-imported today yields Unknown, not Known-false: the empty
// cell is now the schema's "not yet known" spelling, shared with
// ctcss_tone and scan_skip. This is deliberate — the old spelling could
// not distinguish "off" from "nobody has said" — but it does mean a
// pre-E1 file's channels arrive unresolved, and codeplug.Diff then BLOCKS
// each of them ("tag display unknown — set On or Off before sending")
// until the user decides.
//
// The mitigation, for a user who hits this: write an explicit "no" (or
// "yes") into the tag_display column of the old file before importing it,
// or set the value in the UI after import. Either resolves the state
// permanently; a fresh export from this version onwards always writes an
// explicit spelling, so the reinterpretation can only ever bite once, on
// files written before E1.
func TestImport_PreE1EmptyTagDisplayCell_ReinterpretedAsUnknown(t *testing.T) {
	// Byte-for-byte what a pre-E1 Export wrote for a Known-FALSE
	// tag_display: the cell is empty.
	const preE1Row = "001,M-01,14250000,USB,,,,OFF,,SIMPLEX,MB9XYZ,,\n"

	got, err := Import(strings.NewReader(testHeader + preE1Row))
	if err != nil {
		t.Fatalf("Import(pre-E1 CSV) error = %v", err)
	}
	if len(got) != 1 || got[0].Data == nil {
		t.Fatalf("Import(pre-E1 CSV) = %+v, want exactly one populated channel", got)
	}
	if want := (codeplug.BoolField{State: codeplug.Unknown}); got[0].Data.TagDisplay != want {
		t.Errorf("pre-E1 empty tag_display cell imported as %+v, want %+v (the reinterpretation E1 records: previously Known-false)", got[0].Data.TagDisplay, want)
	}

	// The mitigation, proven: the same row with an explicit "no" imports
	// as the Known-false the pre-E1 file meant.
	const mitigatedRow = "001,M-01,14250000,USB,,,,OFF,,SIMPLEX,MB9XYZ,no,\n"
	fixed, err := Import(strings.NewReader(testHeader + mitigatedRow))
	if err != nil {
		t.Fatalf("Import(mitigated CSV) error = %v", err)
	}
	if want := (codeplug.BoolField{State: codeplug.Known, Value: false}); fixed[0].Data.TagDisplay != want {
		t.Errorf("explicit \"no\" imported as %+v, want %+v", fixed[0].Data.TagDisplay, want)
	}
}

// TestImport_AllDataColumnsEmpty_ProducesEmptyChannel covers a row whose
// slot is set but every data column is blank: it must decode to an empty
// Channel (Data == nil), not a populated one with zero values.
func TestImport_AllDataColumnsEmpty_ProducesEmptyChannel(t *testing.T) {
	body := "042,M-42,,,,,,,,,,,\n"
	got, err := Import(strings.NewReader(testHeader + body))
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("Import() = %d channels, want 1", len(got))
	}
	if got[0].Slot != "042" {
		t.Errorf("Slot = %q, want \"042\"", got[0].Slot)
	}
	if !got[0].Empty() {
		t.Errorf("Empty() = false, want true (all data columns blank)")
	}
}

// TestImport_DisplayColumnIgnored covers that the display column's
// content is never consulted, even when it disagrees with the slot.
func TestImport_DisplayColumnIgnored(t *testing.T) {
	body := "001,THIS IS WRONG,14250000,USB,,,,OFF,,SIMPLEX,TAG,,\n"
	got, err := Import(strings.NewReader(testHeader + body))
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if got[0].Slot != "001" {
		t.Errorf("Slot = %q, want \"001\"", got[0].Slot)
	}
}

// TestParseToneFieldCell covers parseToneFieldCell directly, including
// the exact-decimal-precision rule: "88.5" parses; "88.54" (more than
// one decimal place) is rejected outright rather than rounded; "88.50"
// (a trailing zero beyond one place, exactly representable) is accepted.
func TestParseToneFieldCell(t *testing.T) {
	cases := []struct {
		name      string
		in        string
		wantState codeplug.FieldState
		wantValue spec.Tone
		wantErr   bool
	}{
		{"empty is Unknown", "", codeplug.Unknown, 0, false},
		{"n/a is Unavailable", "n/a", codeplug.Unavailable, 0, false},
		{"one decimal place is Known", "88.5", codeplug.Known, spec.Tone(885), false},
		{"trailing zero beyond one place accepted", "88.50", codeplug.Known, spec.Tone(885), false},
		{"more than one decimal place rejected", "88.54", "", 0, true},
		{"unparseable text rejected", "notanumber", "", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseToneFieldCell(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseToneFieldCell(%q) error = nil, want non-nil", tc.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseToneFieldCell(%q) unexpected error: %v", tc.in, err)
			}
			if got.State != tc.wantState || got.Value != tc.wantValue {
				t.Errorf("parseToneFieldCell(%q) = %+v, want {State:%v Value:%v}", tc.in, got, tc.wantState, tc.wantValue)
			}
		})
	}
}

// TestParseBoolFieldCell covers parseBoolFieldCell directly, including
// M9c-5 E1d's parameterisation: the function serves TWO columns now
// (scan_skip and tag_display), so the column name it puts in its
// diagnostic must come from its caller rather than being hardcoded. Both
// callers' names are asserted — a hardcoded "scan_skip" would pass the
// first sub-test and fail the second, which is exactly the regression
// this test exists to catch.
func TestParseBoolFieldCell(t *testing.T) {
	states := []struct {
		name      string
		in        string
		wantState codeplug.FieldState
		wantValue bool
	}{
		{"empty is Unknown", "", codeplug.Unknown, false},
		{"n/a is Unavailable", "n/a", codeplug.Unavailable, false},
		{"yes is Known true", "yes", codeplug.Known, true},
		{"no is Known false", "no", codeplug.Known, false},
	}
	for _, column := range []string{"scan_skip", "tag_display"} {
		t.Run(column, func(t *testing.T) {
			for _, tc := range states {
				t.Run(tc.name, func(t *testing.T) {
					got, err := parseBoolFieldCell(tc.in, column)
					if err != nil {
						t.Fatalf("parseBoolFieldCell(%q, %q) unexpected error: %v", tc.in, column, err)
					}
					if got.State != tc.wantState || got.Value != tc.wantValue {
						t.Errorf("parseBoolFieldCell(%q, %q) = %+v, want {State:%v Value:%v}", tc.in, column, got, tc.wantState, tc.wantValue)
					}
				})
			}
			t.Run("diagnostic names this column", func(t *testing.T) {
				_, err := parseBoolFieldCell("maybe", column)
				if err == nil {
					t.Fatalf("parseBoolFieldCell(%q, %q) error = nil, want non-nil", "maybe", column)
				}
				if !strings.HasPrefix(err.Error(), column+" must be ") {
					t.Errorf("parseBoolFieldCell(%q, %q) error = %q, want it to open by naming %q", "maybe", column, err.Error(), column)
				}
				if !strings.Contains(err.Error(), `got "maybe"`) {
					t.Errorf("parseBoolFieldCell(%q, %q) error = %q, want it to quote the offending value", "maybe", column, err.Error())
				}
			})
		})
	}
}

// TestImport_BadBoolCellDiagnostics pins the parameterised diagnostic
// through the full Import path, for BOTH columns parseBoolFieldCell now
// serves: a bad tag_display cell must not be reported as a scan_skip
// problem (and vice versa), which is precisely what the pre-E1d hardcoded
// column name would have produced.
func TestImport_BadBoolCellDiagnostics(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantNamed string
		notNamed  string
	}{
		{
			name:      "bad tag_display",
			body:      "001,,14250000,USB,,,,OFF,,SIMPLEX,TAG,maybe,\n",
			wantNamed: "tag_display",
			notNamed:  "scan_skip",
		},
		{
			name:      "bad scan_skip",
			body:      "001,,14250000,USB,,,,OFF,,SIMPLEX,TAG,,maybe\n",
			wantNamed: "scan_skip",
			notNamed:  "tag_display",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Import(strings.NewReader(testHeader + tc.body))
			if err == nil {
				t.Fatal("Import() error = nil, want a *ParseError")
			}
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("Import() error = %T, want *ParseError", err)
			}
			if !strings.Contains(pe.Reason, tc.wantNamed) {
				t.Errorf("ParseError.Reason = %q, want it to name %q", pe.Reason, tc.wantNamed)
			}
			if strings.Contains(pe.Reason, tc.notNamed) {
				t.Errorf("ParseError.Reason = %q, want it NOT to name the other column %q", pe.Reason, tc.notNamed)
			}
			if !strings.Contains(pe.Reason, `"yes"`) || !strings.Contains(pe.Reason, `"no"`) || !strings.Contains(pe.Reason, `"n/a"`) {
				t.Errorf("ParseError.Reason = %q, want it to list the accepted spellings", pe.Reason)
			}
		})
	}
}

// TestImport_ToneExactPrecision covers the exact-decimal-precision rule
// through the full Import() own-schema path: a ctcss_tone cell with more
// than one decimal place is a *ParseError, not a silently-rounded value.
func TestImport_ToneExactPrecision(t *testing.T) {
	body := "001,,14250000,USB,,,,ENC,88.54,SIMPLEX,TAG,,\n"
	_, err := Import(strings.NewReader(testHeader + body))
	if err == nil {
		t.Fatal("Import() error = nil, want *ParseError for a ctcss_tone with more than one decimal place")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("Import() error = %T, want *ParseError", err)
	}
}

// TestImport_PhysicalLineNumbers_QuotedMultilineField covers the
// physical-line-number requirement: row 1's tag cell is a quoted field
// containing an embedded newline (legal CSV), spanning TWO physical
// lines by itself. A naive per-RECORD line counter would report row 2's
// error at line 3 (header=1, row1=2, row2=3); the correct PHYSICAL line
// is 4 (header=1, row1 spans 2-3, row2=4).
func TestImport_PhysicalLineNumbers_QuotedMultilineField(t *testing.T) {
	body := testHeader +
		"001,,14250000,USB,,,,OFF,,SIMPLEX,\"A\nB\",,\n" +
		"002,,notanumber,USB,,,,OFF,,SIMPLEX,TAG,,\n"
	_, err := Import(strings.NewReader(body))
	if err == nil {
		t.Fatal("Import() error = nil, want error for bad freq_hz on the second data row")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("Import() error = %T, want *ParseError", err)
	}
	if pe.Line != 4 {
		t.Errorf("ParseError.Line = %d, want 4 (physical line; row 1's quoted multiline tag field spans 2 physical lines)", pe.Line)
	}
}

// TestImport_UnparseableCSV covers a structurally malformed CSV stream.
func TestImport_UnparseableCSV(t *testing.T) {
	body := testHeader + "001,,14250000,USB,,,,OFF,,SIMPLEX,\"unterminated,,,\n"
	_, err := Import(strings.NewReader(body))
	if err == nil {
		t.Fatal("Import() error = nil, want error for malformed CSV")
	}
}
