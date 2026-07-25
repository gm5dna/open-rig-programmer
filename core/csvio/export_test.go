// SPDX-License-Identifier: GPL-3.0-or-later

package csvio

import (
	"bytes"
	"encoding/csv"
	"errors"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// failingWriter always returns an error, for exercising Export's write
// paths when the destination is unwritable (e.g. a full disk).
type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) {
	return 0, errors.New("simulated write failure")
}

// readCSV parses b as CSV and returns every record (including the
// header), for comparing against an expected [][]string without being
// brittle about quoting.
func readCSV(t *testing.T, b []byte) [][]string {
	t.Helper()
	rows, err := csv.NewReader(bytes.NewReader(b)).ReadAll()
	if err != nil {
		t.Fatalf("readCSV: %v", err)
	}
	return rows
}

// TestExport_Header covers the exact header row, byte for byte.
func TestExport_Header(t *testing.T) {
	var buf bytes.Buffer
	if err := Export(&buf, nil); err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	rows := readCSV(t, buf.Bytes())
	if len(rows) != 1 {
		t.Fatalf("Export(nil channels) = %d rows, want 1 (header only)", len(rows))
	}
	want := []string{"slot", "display", "freq_hz", "mode", "clar_hz", "rx_clar", "tx_clar", "ctcss", "ctcss_tone", "shift", "tag", "tag_display", "scan_skip"}
	if len(rows[0]) != len(want) {
		t.Fatalf("header = %v, want %v", rows[0], want)
	}
	for i, col := range want {
		if rows[0][i] != col {
			t.Errorf("header[%d] = %q, want %q", i, rows[0][i], col)
		}
	}
}

// TestExport_EmptySlot covers an empty channel: slot and display are
// filled in, every data column is empty.
func TestExport_EmptySlot(t *testing.T) {
	var buf bytes.Buffer
	channels := []codeplug.Channel{{Slot: "003"}}
	if err := Export(&buf, channels); err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	rows := readCSV(t, buf.Bytes())
	if len(rows) != 2 {
		t.Fatalf("Export() = %d rows, want 2 (header + 1 slot)", len(rows))
	}
	want := []string{"003", "M-03", "", "", "", "", "", "", "", "", "", "", ""}
	if len(rows[1]) != len(want) {
		t.Fatalf("row = %v, want %v", rows[1], want)
	}
	for i := range want {
		if rows[1][i] != want[i] {
			t.Errorf("row[%d] = %q, want %q", i, rows[1][i], want[i])
		}
	}
}

// TestExport_PopulatedSlot covers a fully populated channel with every
// field at a non-default, non-zero value, including the three FieldState
// values for ctcss_tone and scan_skip.
func TestExport_PopulatedSlot(t *testing.T) {
	cases := []struct {
		name string
		data codeplug.ChannelData
		want []string // freq_hz..scan_skip (10 cols, cols 2..12)
	}{
		{
			name: "known tone, known scan_skip true, all bools set",
			data: codeplug.ChannelData{
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
			want: []string{"14250000", "USB", "-120", "yes", "yes", "ENC-DEC", "88.5", "PLUS", "MB9XYZ", "yes", "yes"},
		},
		{
			name: "unknown tone, known scan_skip false, no bools",
			data: codeplug.ChannelData{
				FreqHz:    14300000,
				Mode:      "LSB",
				CTCSS:     "OFF",
				CTCSSTone: codeplug.ToneField{State: codeplug.Unknown},
				Shift:     "SIMPLEX",
				Tag:       "NET",
				ScanSkip:  codeplug.BoolField{State: codeplug.Known, Value: false},
			},
			want: []string{"14300000", "LSB", "", "", "", "OFF", "", "SIMPLEX", "NET", "", "no"},
		},
		{
			name: "unavailable tone, unavailable scan_skip",
			data: codeplug.ChannelData{
				FreqHz:    5330500,
				Mode:      "AM",
				CTCSS:     "OFF",
				CTCSSTone: codeplug.ToneField{State: codeplug.Unavailable},
				Shift:     "SIMPLEX",
				ScanSkip:  codeplug.BoolField{State: codeplug.Unavailable},
			},
			want: []string{"5330500", "AM", "", "", "", "OFF", "n/a", "SIMPLEX", "", "", "n/a"},
		},
		{
			name: "clar_hz zero omitted",
			data: codeplug.ChannelData{
				FreqHz:    7100000,
				Mode:      "LSB",
				ClarHz:    0,
				CTCSS:     "OFF",
				CTCSSTone: codeplug.ToneField{State: codeplug.Unknown},
				Shift:     "SIMPLEX",
				ScanSkip:  codeplug.BoolField{State: codeplug.Unknown},
			},
			want: []string{"7100000", "LSB", "", "", "", "OFF", "", "SIMPLEX", "", "", ""},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			d := tc.data
			channels := []codeplug.Channel{{Slot: "001", Data: &d}}
			if err := Export(&buf, channels); err != nil {
				t.Fatalf("Export() error = %v", err)
			}
			rows := readCSV(t, buf.Bytes())
			if len(rows) != 2 {
				t.Fatalf("Export() = %d rows, want 2", len(rows))
			}
			row := rows[1]
			if row[0] != "001" || row[1] != "M-01" {
				t.Errorf("row[0:2] = %v, want [001 M-01]", row[0:2])
			}
			got := row[2:]
			if len(got) != len(tc.want) {
				t.Fatalf("data cols = %v, want %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("col[%d] = %q, want %q", i+2, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestExport_FormulaInjectionEscaping covers the OWASP CSV-injection
// guard: a tag beginning with '=', '+', '-' or '@' that is NOT a plain
// signed decimal number gets a leading apostrophe; clar_hz's own signed
// decimal integers (e.g. "-120") must NOT be escaped.
func TestExport_FormulaInjectionEscaping(t *testing.T) {
	cases := []struct {
		name    string
		tag     string
		clarHz  int
		wantTag string
	}{
		{"formula tag =SUM", "=SUM(A1)", 0, "'=SUM(A1)"},
		{"at-command tag", "@cmd", 0, "'@cmd"},
		{"plus non-numeric tag", "+notanumber", 0, "'+notanumber"},
		{"minus non-numeric tag", "-notanumber", 0, "'-notanumber"},
		{"plain signed integer tag NOT escaped", "+123", 0, "+123"},
		{"ordinary tag untouched", "MB9XYZ", 0, "MB9XYZ"},
		{"negative clar_hz NOT escaped (checked separately)", "CALLING", -120, "CALLING"},
		{"tag with legitimate leading apostrophe is double-escaped", "'TWAS", 0, "''TWAS"},
		{"tag that is just an apostrophe is double-escaped", "'", 0, "''"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			d := codeplug.ChannelData{
				FreqHz:    14250000,
				Mode:      "USB",
				ClarHz:    tc.clarHz,
				CTCSS:     "OFF",
				CTCSSTone: codeplug.ToneField{State: codeplug.Unknown},
				Shift:     "SIMPLEX",
				Tag:       tc.tag,
				ScanSkip:  codeplug.BoolField{State: codeplug.Unknown},
			}
			channels := []codeplug.Channel{{Slot: "001", Data: &d}}
			if err := Export(&buf, channels); err != nil {
				t.Fatalf("Export() error = %v", err)
			}
			rows := readCSV(t, buf.Bytes())
			gotTag := rows[1][10] // tag column index
			if gotTag != tc.wantTag {
				t.Errorf("tag column = %q, want %q", gotTag, tc.wantTag)
			}
			if tc.clarHz != 0 {
				gotClar := rows[1][4]
				wantClar := "-120"
				if gotClar != wantClar {
					t.Errorf("clar_hz column = %q, want %q (must not be escaped)", gotClar, wantClar)
				}
			}
		})
	}
}

// TestEscapeCell directly unit-tests the exported EscapeCell (task-34
// brief, Codex plan-review F10): the same case set
// TestExport_FormulaInjectionEscaping already proves end-to-end through
// the memory-row export path (that test is left unchanged, still green,
// as the behaviour-identity check for that path — see EscapeCell's doc
// comment). This test exercises the function directly, schema-neutral,
// standing in for any future caller's free-text column (e.g. cmd/rigprog's
// "settings" CSV export).
func TestEscapeCell(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"empty string untouched", "", ""},
		{"formula prefix =", "=SUM(A1)", "'=SUM(A1)"},
		{"at-command prefix @", "@cmd", "'@cmd"},
		{"plus non-numeric", "+notanumber", "'+notanumber"},
		{"minus non-numeric", "-notanumber", "'-notanumber"},
		{"plain signed integer NOT escaped", "+123", "+123"},
		{"plain negative integer NOT escaped", "-120", "-120"},
		{"ordinary text untouched", "AF TREBLE GAIN", "AF TREBLE GAIN"},
		{"leading apostrophe double-escaped", "'TWAS", "''TWAS"},
		{"lone apostrophe double-escaped", "'", "''"},
		{"ordinary text with internal punctuation untouched", "MODE SSB (01-01)", "MODE SSB (01-01)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := EscapeCell(tc.in); got != tc.want {
				t.Errorf("EscapeCell(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestExport_WriteFailure covers Export's error path when the
// destination io.Writer fails on the buffered Flush at the end (the
// common case: csv.Writer buffers small writes, so a failing writer's
// error normally only surfaces there).
func TestExport_WriteFailure(t *testing.T) {
	err := Export(failingWriter{}, []codeplug.Channel{{Slot: "001"}})
	if err == nil {
		t.Fatal("Export() error = nil, want error for a failing writer")
	}
}

// TestExport_WriteFailure_MidRow covers Export's error path when a
// single row is large enough that csv.Writer's internal buffer flushes
// (and so fails against a failing writer) DURING that row's Write call,
// rather than only being caught by the final Flush/Error check.
func TestExport_WriteFailure_MidRow(t *testing.T) {
	d := codeplug.ChannelData{
		FreqHz:    14250000,
		Mode:      "USB",
		CTCSS:     "OFF",
		CTCSSTone: codeplug.ToneField{State: codeplug.Unknown},
		Shift:     "SIMPLEX",
		Tag:       strings.Repeat("A", 8192), // forces csv.Writer's bufio flush mid-row
		ScanSkip:  codeplug.BoolField{State: codeplug.Unknown},
	}
	err := Export(failingWriter{}, []codeplug.Channel{{Slot: "001", Data: &d}})
	if err == nil {
		t.Fatal("Export() error = nil, want error for a failing writer")
	}
}

// TestExport_FullImageRoundTripSlotOrder covers a mixed image (empty and
// populated slots interleaved) to make sure row order and slot-only rows
// both come out correctly — the fuller round-trip assertion (byte
// identical Channels after Export->Import) lives in import_test.go once
// Import exists.
func TestExport_FullImageRoundTripSlotOrder(t *testing.T) {
	var buf bytes.Buffer
	channels := []codeplug.Channel{
		{Slot: "001", Data: &codeplug.ChannelData{FreqHz: 14250000, Mode: "USB", CTCSS: "OFF", CTCSSTone: codeplug.ToneField{State: codeplug.Unknown}, Shift: "SIMPLEX", ScanSkip: codeplug.BoolField{State: codeplug.Unknown}}},
		{Slot: "002"},
		{Slot: "003", Data: &codeplug.ChannelData{FreqHz: 7100000, Mode: "LSB", CTCSS: "OFF", CTCSSTone: codeplug.ToneField{State: codeplug.Unknown}, Shift: "SIMPLEX", ScanSkip: codeplug.BoolField{State: codeplug.Unknown}}},
	}
	if err := Export(&buf, channels); err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	rows := readCSV(t, buf.Bytes())
	if len(rows) != 4 {
		t.Fatalf("Export() = %d rows, want 4 (header + 3 slots)", len(rows))
	}
	wantSlots := []string{"001", "002", "003"}
	for i, want := range wantSlots {
		if rows[i+1][0] != want {
			t.Errorf("row %d slot = %q, want %q", i+1, rows[i+1][0], want)
		}
	}
}
