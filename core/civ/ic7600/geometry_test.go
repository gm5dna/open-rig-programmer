// SPDX-License-Identifier: GPL-3.0-or-later

package ic7600_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7600"
)

// goldenRecord is the neutral record this package's golden vectors encode -
// see testdata/ic7600-golden-provenance.md for why these are synthetic test
// values rather than a manual quotation.
//
// Select and DataMode are deliberately ABSENT (the zero Optional, i.e.
// Unavailable): ruling E6 leaves both regions unmapped on this model, so
// the encoder writes the Fixed template's zeros there.
//
// Shared with golden_test.go: both files are package ic7600_test.
func goldenRecord() civ.MemoryRecord {
	return civ.MemoryRecord{
		Address:      civ.ChannelAddress{Channel: 1},    // frame bytes 7-8: 00 01
		RXFreqHz:     civ.Available[uint64](14_250_000), // frame bytes 10-14: 00 00 25 14 00
		Mode:         civ.Available("USB"),               // frame byte 15: 01
		Filter:       civ.Available("FIL1"),               // frame byte 16: 01
		ToneMode:     civ.Available("TONE"),               // frame byte 17 low nibble: 1
		ToneTXDeciHz: civ.Available[uint64](885),          // frame bytes 18-20: 00 08 85
		ToneRXDeciHz: civ.Available[uint64](1000),         // frame bytes 21-23: 00 10 00
		Name:         civ.Available("HOME QTH01"),         // frame bytes 24-33
	}
}

// witnessRow is one row of the geometry witness.
type witnessRow struct {
	indexRaw, blockLabel, page, anchor, notes string
	key                                       indexKey
	firstByte, firstNibble, lastByte, lastNibble int
	recordOffset, recordWidth                    int
	isAddress                                    bool
}

const witnessHeader = "diagram_id,field_index,block_label_verbatim,first_byte,first_nibble,last_byte,last_nibble,pdf_page,visual_anchor,notes"

// TestGeometry_WitnessBindsTheRecord binds the geometry witness's byte
// positions (testdata/ic7600-geometry-witness.csv - matrix S3.11's own
// arithmetic, restated as a cumulative table; see doc.go's Provenance
// section for why this is not an independent raster re-measurement) to
// the geometry this package's profile encodes and decodes.
//
// HOW THE COMPARISON IS MADE. A 25-byte record is ASSEMBLED by placing
// each witnessed field's chosen content at that field's own witnessed
// positions, and the result must equal, byte for byte, what
// BuildMemorySet produces from the same neutral values. A witness
// position one byte out therefore fails as a frame mismatch, not as an
// arithmetic disagreement between two hand-copied numbers.
func TestGeometry_WitnessBindsTheRecord(t *testing.T) {
	p := ic7600.Profile()
	layout := p.Layouts()[0]
	spans := map[civ.FieldID]civ.FieldSpan{}
	for _, sp := range layout.Fields {
		spans[sp.Field] = sp
	}

	var d1 []witnessRow
	seen := map[string]bool{}
	for i, rec := range readEvidenceCSV(t, "ic7600-geometry-witness.csv", witnessHeader, 10) {
		row := witnessRow{
			indexRaw: rec[1], blockLabel: rec[2],
			page: rec[7], anchor: rec[8], notes: rec[9],
		}
		key, err := parseIndex(row.indexRaw)
		if err != nil {
			t.Fatalf("witness row %d: %v", i+2, err)
		}
		row.key = key
		nums := [...]*int{&row.firstByte, &row.firstNibble, &row.lastByte, &row.lastNibble}
		for j, col := range []int{3, 4, 5, 6} {
			n, err := strconv.Atoi(rec[col])
			if err != nil {
				t.Fatalf("witness row %d, column %d: %q is not a number: %v", i+2, col+1, rec[col], err)
			}
			*nums[j] = n
		}
		if seen[row.indexRaw] {
			t.Fatalf("witness carries two rows for %s; every measured block appears once", row.indexRaw)
		}
		seen[row.indexRaw] = true
		d1 = append(d1, row)
	}
	if len(d1) != 9 {
		t.Fatalf("witness carries %d rows, want 9", len(d1))
	}

	// --- Data-area coordinates to record coordinates -----------------------
	var addressRows int
	for i := range d1 {
		row := &d1[i]
		if row.lastByte < 3 {
			addressRows++
			row.isAddress = true
			if w := row.lastByte - row.firstByte + 1; w != ic7600.AddressBytes {
				t.Errorf("the witness's address row %s measures %d bytes, want %d (ic7600.AddressBytes)", row.key, w, ic7600.AddressBytes)
			}
			continue
		}
		row.recordOffset = row.firstByte - 3
		row.recordWidth = row.lastByte - row.firstByte + 1
	}
	if addressRows != 1 {
		t.Fatalf("%d rows lie wholly before data-area byte 3; exactly one does - the channel address (1),(2)", addressRows)
	}

	// --- The tiling ----------------------------------------------------------
	dataArea := make([]int, ic7600.DataAreaLength+1) // 1-based
	for _, row := range d1 {
		for b := row.firstByte; b <= row.lastByte; b++ {
			if b < 1 || b > ic7600.DataAreaLength {
				t.Fatalf("witness row %s reaches data-area byte %d, outside 1..%d", row.key, b, ic7600.DataAreaLength)
			}
			dataArea[b]++
		}
	}
	for b := 1; b <= ic7600.DataAreaLength; b++ {
		if dataArea[b] != 1 {
			t.Fatalf("the witness covers data-area byte %d %d times, want exactly once", b, dataArea[b])
		}
	}
	record := make([]int, ic7600.RecordOnlyLength)
	for _, row := range d1 {
		if row.isAddress {
			continue
		}
		for o := row.recordOffset; o < row.recordOffset+row.recordWidth; o++ {
			if o < 0 || o >= ic7600.RecordOnlyLength {
				t.Fatalf("witness row %s reaches record offset %d, outside 0..%d", row.key, o, ic7600.RecordOnlyLength-1)
			}
			record[o]++
		}
	}
	for o, n := range record {
		if n != 1 {
			t.Fatalf("the witness covers record offset %d %d times, want exactly once", o, n)
		}
	}

	// --- The unmapped regions, both directions --------------------------------
	var atEight []civ.FieldSpan
	for _, sp := range layout.Fields {
		if sp.Offset == ic7600.DataModeNibbleOffset {
			atEight = append(atEight, sp)
		}
		if sp.Offset == ic7600.SelectByteOffset {
			t.Errorf("%s claims record offset %d. Ruling E6 leaves printed (3) UNMAPPED (whole byte, no nibble split printed - matrix S3.15(a)).", sp.Field, sp.Offset)
		}
	}
	if len(atEight) != 1 || atEight[0].Field != civ.FieldToneMode || atEight[0].Nibble != civ.NibbleLow {
		t.Fatalf("record offset %d carries %v; want exactly one span, tone_mode on the LOW nibble", ic7600.DataModeNibbleOffset, atEight)
	}
	fixed := ic7600.FixedTemplate()
	if fixed[0] != 0x00 || fixed[8] != 0x00 {
		t.Errorf("FixedTemplate()[0] = %#02x and [8] = %#02x, want 0x00 and 0x00", fixed[0], fixed[8])
	}

	// --- Assemble from the witness, and compare -------------------------------
	content := map[indexKey][]byte{
		{3, 3}:   {0x00},
		{4, 8}:   {0x00, 0x00, 0x25, 0x14, 0x00}, // 14.250000 MHz, little-endian packed BCD
		{9, 9}:   {wireByte(t, spans[civ.FieldMode], "USB")},
		{10, 10}: {wireByte(t, spans[civ.FieldFilter], "FIL1")},
		{11, 11}: {wireByte(t, spans[civ.FieldToneMode], "TONE")}, // high nibble 0, low nibble the tone-mode value
		{12, 14}: {0x00, 0x08, 0x85},                              // 88.5 Hz -> 885 deciHz, big-endian
		{15, 17}: {0x00, 0x10, 0x00},                              // 100.0 Hz -> 1000 deciHz, big-endian
		{18, 27}: []byte("HOME QTH01"),
	}
	assembled := make([]byte, ic7600.RecordOnlyLength)
	placed := map[indexKey]bool{}
	for _, row := range d1 {
		if row.isAddress {
			continue
		}
		bytes, ok := content[row.key]
		if !ok {
			t.Fatalf("witness row %s has no assembly content declared", row.key)
		}
		if len(bytes) != row.recordWidth {
			t.Fatalf("witness row %s measures %d bytes but its assembly content is %d", row.key, row.recordWidth, len(bytes))
		}
		copy(assembled[row.recordOffset:], bytes)
		placed[row.key] = true
	}
	for key := range content {
		if !placed[key] {
			t.Errorf("assembly content was declared for printed index %s, which the witness does not measure", key)
		}
	}

	cmd, err := p.BuildMemorySet(goldenRecord())
	if err != nil {
		t.Fatalf("BuildMemorySet: %v", err)
	}
	frame := cmd.Bytes()
	const prefix = 6 + ic7600.AddressBytes // FE FE 7A E0 1A 00 <ch-hi> <ch-lo>
	if len(frame) != prefix+ic7600.RecordOnlyLength+1 {
		t.Fatalf("BuildMemorySet produced %d frame bytes, want %d", len(frame), prefix+ic7600.RecordOnlyLength+1)
	}
	built := frame[prefix : len(frame)-1]
	if string(built) != string(assembled) {
		t.Errorf("THE WITNESS AND THE BUILDER DISAGREE - this is a STOP for arbitration against PDF p.178.\n"+
			"  assembled from the witness (%d bytes): % X\n"+
			"  built by BuildMemorySet     (%d bytes): % X\n"+
			"  first differing record offset: %s",
			len(assembled), assembled, len(built), built, firstDiff(assembled, built))
	}

	// --- Parse back --------------------------------------------------------
	answer := append([]byte{0xFE, 0xFE, 0xE0, 0x7A, 0x1A, 0x00, 0x00, 0x01}, assembled...)
	answer = append(answer, 0xFD)
	rec, err := p.ParseMemoryAnswer(answer)
	if err != nil {
		t.Fatalf("ParseMemoryAnswer of the assembled record: %v", err)
	}
	want := goldenRecord()
	if rec.Address != want.Address {
		t.Errorf("parsed Address = %v, want %v", rec.Address, want.Address)
	}
	for _, c := range []struct {
		name      string
		got, want civ.Optional[uint64]
	}{
		{"RXFreqHz", rec.RXFreqHz, want.RXFreqHz},
		{"ToneTXDeciHz", rec.ToneTXDeciHz, want.ToneTXDeciHz},
		{"ToneRXDeciHz", rec.ToneRXDeciHz, want.ToneRXDeciHz},
	} {
		if c.got != c.want {
			t.Errorf("parsed %s = %v, want %v", c.name, c.got, c.want)
		}
	}
	for _, c := range []struct {
		name      string
		got, want civ.Optional[string]
	}{
		{"Mode", rec.Mode, want.Mode},
		{"Filter", rec.Filter, want.Filter},
		{"ToneMode", rec.ToneMode, want.ToneMode},
		{"Name", rec.Name, want.Name},
	} {
		if c.got != c.want {
			t.Errorf("parsed %s = %v, want %v", c.name, c.got, c.want)
		}
	}
	if !rec.Select.Unavailable() {
		t.Errorf("parsed Select = %v, want Unavailable - printed (3) is UNMAPPED under ruling E6", rec.Select)
	}
	if !rec.DataMode.Unavailable() {
		t.Errorf("parsed DataMode = %v, want Unavailable - printed (11)'s high nibble is UNMAPPED under ruling E6", rec.DataMode)
	}

	t.Run("the_encoder_always_writes_the_template", func(t *testing.T) {
		cmd, err := p.BuildMemorySet(goldenRecord())
		if err != nil {
			t.Fatalf("BuildMemorySet(goldenRecord()): %v", err)
		}
		built := cmd.Bytes()[prefix : len(cmd.Bytes())-1]
		if built[ic7600.SelectByteOffset] != 0x00 {
			t.Errorf("BuildMemorySet emitted %#02x at record offset %d, want 0x00", built[ic7600.SelectByteOffset], ic7600.SelectByteOffset)
		}
		if built[ic7600.DataModeNibbleOffset]&0xF0 != 0x00 {
			t.Errorf("BuildMemorySet emitted %#02x at record offset %d, want a ZERO high nibble", built[ic7600.DataModeNibbleOffset], ic7600.DataModeNibbleOffset)
		}
	})

	t.Run("the_encoder_refuses_a_record_claiming_an_unmapped_field", func(t *testing.T) {
		for _, tc := range []struct {
			name  string
			rec   civ.MemoryRecord
			field string
		}{
			{"a record claiming a SELECT group", withSelect(goldenRecord(), "★2"), "select"},
			{"a record claiming a data mode", withDataMode(goldenRecord(), "DATA 2"), "data_mode"},
		} {
			cmd, err := p.BuildMemorySet(tc.rec)
			if err == nil {
				t.Errorf("%s: BuildMemorySet SUCCEEDED, producing % X", tc.name, cmd.Bytes())
				continue
			}
			if !strings.Contains(err.Error(), tc.field) {
				t.Errorf("%s: BuildMemorySet refused, but the error does not name %q:\n  %v", tc.name, tc.field, err)
			}
		}
	})

	t.Run("the_gate_refuses_a_non_template_unmapped_region", func(t *testing.T) {
		for _, tc := range []struct {
			name   string
			offset int
			value  byte
		}{
			{"an E6-unmapped SELECT marker of 2", ic7600.SelectByteOffset, 0x02},
			{"an E6-unmapped data mode of 2", ic7600.DataModeNibbleOffset, 0x21},
		} {
			mutated := make([]byte, len(frame))
			copy(mutated, frame)
			mutated[prefix+tc.offset] = tc.value
			if p.AllowedCommand(mutated) {
				t.Errorf("%s: AllowedCommand admitted a set whose record byte %d is %#02x", tc.name, tc.offset, tc.value)
			}
		}
		if !p.AllowedCommand(frame) {
			t.Fatalf("AllowedCommand refused the golden set frame itself: % X", frame)
		}
	})
}

func wireByte(t *testing.T, sp civ.FieldSpan, name string) byte {
	t.Helper()
	for b, n := range sp.Enum {
		if n == name {
			return b
		}
	}
	t.Fatalf("%s's enum has no member named %q", sp.Field, name)
	return 0
}

func firstDiff(a, b []byte) string {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return strconv.Itoa(i) + " (" + strconv.Itoa(int(a[i])) + " vs " + strconv.Itoa(int(b[i])) + ")"
		}
	}
	if len(a) != len(b) {
		return "no differing byte; the lengths differ"
	}
	return "none"
}

func withSelect(r civ.MemoryRecord, v string) civ.MemoryRecord {
	r.Select = civ.Available(v)
	return r
}

func withDataMode(r civ.MemoryRecord, v string) civ.MemoryRecord {
	r.DataMode = civ.Available(v)
	return r
}
