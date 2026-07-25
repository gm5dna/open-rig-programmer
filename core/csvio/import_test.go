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

// fullImage is a mixed radio image exercising every field-state
// combination this schema supports, plus an interleaved empty slot, used
// by the round-trip tests below.
func fullImage() []codeplug.Channel {
	return []codeplug.Channel{
		{
			Slot: "001",
			Data: &codeplug.ChannelData{
				FreqHz:     14250000,
				Mode:       "USB",
				ClarHz:     -120,
				RxClar:     true,
				TxClar:     true,
				CTCSS:      "ENC-DEC",
				CTCSSTone:  codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(885)},
				Shift:      "PLUS",
				Tag:        "MB9XYZ",
				TagDisplay: true,
				ScanSkip:   codeplug.BoolField{State: codeplug.Known, Value: true},
			},
		},
		{Slot: "002"}, // empty slot, interleaved
		{
			Slot: "003",
			Data: &codeplug.ChannelData{
				FreqHz:    14300000,
				Mode:      "LSB",
				CTCSS:     "OFF",
				CTCSSTone: codeplug.ToneField{State: codeplug.Unknown},
				Shift:     "SIMPLEX",
				Tag:       "NET",
				ScanSkip:  codeplug.BoolField{State: codeplug.Known, Value: false},
			},
		},
		{
			Slot: "501",
			Data: &codeplug.ChannelData{
				FreqHz:    5330500,
				Mode:      "AM",
				ClarHz:    370,
				CTCSS:     "OFF",
				CTCSSTone: codeplug.ToneField{State: codeplug.Unavailable},
				Shift:     "SIMPLEX",
				ScanSkip:  codeplug.BoolField{State: codeplug.Unavailable},
			},
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
			d := codeplug.ChannelData{
				FreqHz:    14250000,
				Mode:      "USB",
				CTCSS:     "OFF",
				CTCSSTone: codeplug.ToneField{State: codeplug.Unknown},
				Shift:     "SIMPLEX",
				Tag:       tc.tag,
				ScanSkip:  codeplug.BoolField{State: codeplug.Unknown},
			}
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
		FreqHz:    14250000,
		Mode:      "USB",
		CTCSS:     "OFF",
		CTCSSTone: codeplug.ToneField{State: codeplug.Unknown},
		Shift:     "SIMPLEX",
		Tag:       "BBC ANT 3",
		ScanSkip:  codeplug.BoolField{State: codeplug.Unknown},
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
