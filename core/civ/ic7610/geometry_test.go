// SPDX-License-Identifier: GPL-3.0-or-later

package ic7610_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7610"
)

// goldenRecord is the neutral record the G leg's set vector encodes, stated
// as LITERALS read by hand off testdata/ic7610-golden-assumptions.csv, each
// carrying the frame byte position it came from. A test that parsed the frame
// and rebuilt it would prove the codec self-consistent and nothing else.
//
// Select and DataMode are deliberately ABSENT (the zero Optional, i.e.
// Unavailable): ruling E6 leaves both regions unmapped on this model, so the
// encoder writes the Fixed template's zeros there and the neutral record has
// nothing to say about them. The golden vector's own bytes agree - frame byte
// 9 is 0x00 and frame byte 17's high nibble is 0.
//
// IT LIVES IN THIS FILE because this is the first test that needs it and
// golden_test.go shares it: both files are package ic7610_test, so the plan's
// "the one civ.MemoryRecord literal Tasks 5 and 6 share" is one definition
// either way, and putting it where the earlier task can compile against it is
// the only ordering that lets each task land on its own.
func goldenRecord() civ.MemoryRecord {
	return civ.MemoryRecord{
		Address:      civ.ChannelAddress{Channel: 1},    // frame bytes 7-8: 00 01
		RXFreqHz:     civ.Available[uint64](14_250_000), // frame bytes 10-14: 00 00 25 14 00
		Mode:         civ.Available("USB"),              // frame byte 15: 01
		Filter:       civ.Available("FIL1"),             // frame byte 16: 01
		ToneMode:     civ.Available("TONE"),             // frame byte 17 low nibble: 1
		ToneTXDeciHz: civ.Available[uint64](885),        // frame bytes 18-20: 00 08 85
		ToneRXDeciHz: civ.Available[uint64](1000),       // frame bytes 21-23: 00 10 00
		Name:         civ.Available("HOME QTH01"),       // frame bytes 24-33
	}
}

// witnessRow is one measured row of the W leg.
type witnessRow struct {
	diagram, indexRaw, blockLabel, page, anchor, notes string
	key                                                indexKey
	firstByte, firstNibble, lastByte, lastNibble       int
	recordOffset, recordWidth                          int
	isAddress                                          bool
}

const witnessHeader = "diagram_id,field_index,block_label_verbatim,first_byte,first_nibble,last_byte,last_nibble,pdf_page,visual_anchor,notes"

// TestGeometry_WitnessBindsTheRecord binds the W leg's measured byte and
// nibble positions to the geometry this package's profile encodes and
// decodes.
//
// THE WITNESS IS INDEPENDENT EVIDENCE, which is the whole point.
// testdata/ic7610-geometry-witness.csv was produced by a quarantined agent
// from 400-500 dpi raster renders alone - no text layer, no repository
// access, no sight of any code - so the positions on the left of every
// comparison below were counted off the printed page by someone who did not
// know what this profile expects.
//
// HOW THE COMPARISON IS MADE. A 25-byte record is ASSEMBLED by placing each
// witnessed field's chosen content at that field's own witnessed positions,
// and the result must equal, byte for byte, what BuildMemorySet produces
// from the same neutral values. A witness position one byte out therefore
// fails as a frame mismatch, not as an arithmetic disagreement between two
// hand-copied numbers.
//
// EVERY WITNESSED ROW IS CONSUMED. There is no skip path.
func TestGeometry_WitnessBindsTheRecord(t *testing.T) {
	p := ic7610.Profile()
	layout := p.Layouts()[0]
	spans := map[civ.FieldID]civ.FieldSpan{}
	for _, sp := range layout.Fields {
		spans[sp.Field] = sp
	}

	// --- 1. Load all ten W rows -------------------------------------------
	var d1 []witnessRow
	sub := map[string]witnessRow{}
	seen := map[string]bool{}
	for i, rec := range readEvidenceCSV(t, "ic7610-geometry-witness.csv", witnessHeader, 10) {
		row := witnessRow{
			diagram: rec[0], indexRaw: rec[1], blockLabel: rec[2],
			page: rec[7], anchor: rec[8], notes: rec[9],
		}
		key, err := parseCircledIndex(row.indexRaw)
		if err != nil {
			t.Fatalf("witness row %d: %v", i+2, err) // W spells its indices the way B does
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
		id := row.diagram + "," + row.indexRaw
		if seen[id] {
			t.Fatalf("witness carries two rows for %s; every measured block appears once", id)
		}
		seen[id] = true

		switch row.diagram {
		case "D1":
			d1 = append(d1, row)
		case "D2", "D3":
			sub[row.diagram] = row
		default:
			t.Fatalf("witness row %d carries diagram_id %q; W measured D1, D2 and D3 and nothing else", i+2, row.diagram)
		}
	}
	if len(d1) != 8 {
		t.Fatalf("witness carries %d D1 rows, want 8", len(d1))
	}
	if len(sub) != 2 {
		t.Fatalf("witness carries %d sub-diagram rows, want 2 (D2,(3) and D3,(11))", len(sub))
	}

	// --- 2. Data-area coordinates to record coordinates -------------------
	//
	// W's first_byte/last_byte are 1-based within the 27-byte DATA AREA.
	// THESE ARE THE ONE TABLE's OFFSETS; if they disagree with the profile,
	// that disagreement is the arbitration.
	var addressRows int
	for i := range d1 {
		row := &d1[i]
		if row.lastByte < 3 {
			addressRows++
			row.isAddress = true
			if w := row.lastByte - row.firstByte + 1; w != ic7610.AddressBytes {
				t.Errorf("the witness's address row %s measures %d bytes, want %d (ic7610.AddressBytes)", row.key, w, ic7610.AddressBytes)
			}
			continue
		}
		row.recordOffset = row.firstByte - 3
		row.recordWidth = row.lastByte - row.firstByte + 1
	}
	if addressRows != 1 {
		t.Fatalf("%d D1 rows lie wholly before data-area byte 3; exactly one does - the channel address (1),(2)", addressRows)
	}

	// --- 3. The tiling ----------------------------------------------------
	dataArea := make([]int, ic7610.DataAreaLength+1) // 1-based
	for _, row := range d1 {
		for b := row.firstByte; b <= row.lastByte; b++ {
			if b < 1 || b > ic7610.DataAreaLength {
				t.Fatalf("witness row %s reaches data-area byte %d, outside 1..%d", row.key, b, ic7610.DataAreaLength)
			}
			dataArea[b]++
		}
	}
	for b := 1; b <= ic7610.DataAreaLength; b++ {
		if dataArea[b] != 1 {
			t.Fatalf("the witness covers data-area byte %d %d times, want exactly once - the eight D1 rows must tile 1..%d",
				b, dataArea[b], ic7610.DataAreaLength)
		}
	}
	record := make([]int, ic7610.RecordOnlyLength)
	for _, row := range d1 {
		if row.isAddress {
			continue
		}
		for o := row.recordOffset; o < row.recordOffset+row.recordWidth; o++ {
			if o < 0 || o >= ic7610.RecordOnlyLength {
				t.Fatalf("witness row %s reaches record offset %d, outside 0..%d", row.key, o, ic7610.RecordOnlyLength-1)
			}
			record[o]++
		}
	}
	for o, n := range record {
		if n != 1 {
			t.Fatalf("the witness covers record offset %d %d times, want exactly once - the seven record rows must tile 0..%d",
				o, n, ic7610.RecordOnlyLength-1)
		}
	}

	// --- 4. The nibble columns --------------------------------------------
	for _, row := range d1 {
		if row.firstNibble != 1 || row.lastNibble != 2 {
			t.Errorf("witness D1 row %s spans nibbles %d..%d, want 1..2 - every D1 block is measured as whole bytes",
				row.key, row.firstNibble, row.lastNibble)
		}
	}
	// The two sub-diagram rows are single-cell boxes whose two nibbles carry
	// SEPARATE meanings, which is exactly why ruling E6 leaves one nibble of
	// each unmapped.
	for name, row := range sub {
		if row.firstByte != 1 || row.lastByte != 1 {
			t.Errorf("witness %s row %s measures bytes %d..%d; a sub-diagram is one enlarged cell of its own block",
				name, row.key, row.firstByte, row.lastByte)
		}
		if row.firstNibble != 1 || row.lastNibble != 2 {
			t.Errorf("witness %s row %s spans nibbles %d..%d, want 1..2", name, row.key, row.firstNibble, row.lastNibble)
		}
	}
	var atEight []civ.FieldSpan
	for _, sp := range layout.Fields {
		if sp.Offset == 8 {
			atEight = append(atEight, sp)
		}
		if sp.Offset == 0 {
			t.Errorf("%s claims record offset 0. Ruling E6 leaves printed (3) UNMAPPED: its high nibble is the page's printed Fixed 0 and its low nibble is a four-valued SELECT-group marker with no faithful neutral home.", sp.Field)
		}
	}
	if len(atEight) != 1 || atEight[0].Field != civ.FieldToneMode || atEight[0].Nibble != civ.NibbleLow {
		t.Fatalf("record offset 8 carries %v; want exactly one span, tone_mode on the LOW nibble, its HIGH nibble claimed by none. Ruling E6 leaves the four-valued data mode there unmapped.", atEight)
	}
	fixed := ic7610.FixedTemplate()
	if fixed[0] != 0x00 || fixed[8] != 0x00 {
		t.Errorf("FixedTemplate()[0] = %#02x and [8] = %#02x, want 0x00 and 0x00 - ruling E6's template is what an unmapped region is judged against", fixed[0], fixed[8])
	}

	// --- 5. Assemble from the witness, and compare ------------------------
	//
	// The content is placed at the WITNESS's own positions, not at the
	// profile's, so a witness position one byte out fails as a frame
	// mismatch rather than as a disagreement between two copied numbers.
	content := map[indexKey][]byte{
		{3, 3}:   {0x00},                         // both nibbles unmapped
		{4, 8}:   {0x00, 0x00, 0x25, 0x14, 0x00}, // 14.250000 MHz, little-endian packed BCD
		{9, 10}:  {wireByte(t, spans[civ.FieldMode], "USB"), wireByte(t, spans[civ.FieldFilter], "FIL1")},
		{11, 11}: {wireByte(t, spans[civ.FieldToneMode], "TONE")}, // high nibble 0, low nibble the tone-mode value
		{12, 14}: {0x00, 0x08, 0x85},                              // 88.5 Hz -> 885 deci-Hz, big-endian
		{15, 17}: {0x00, 0x10, 0x00},                              // 100.0 Hz -> 1000 deci-Hz, big-endian
		{18, 27}: []byte("HOME QTH01"),
	}
	assembled := make([]byte, ic7610.RecordOnlyLength)
	placed := map[indexKey]bool{}
	for _, row := range d1 {
		if row.isAddress {
			continue
		}
		bytes, ok := content[row.key]
		if !ok {
			t.Fatalf("witness row %s has no assembly content declared; EVERY witnessed row is consumed and there is no skip path", row.key)
		}
		if len(bytes) != row.recordWidth {
			t.Fatalf("witness row %s measures %d bytes but its assembly content is %d - the content is stated per field and must fit that field's OWN measured width",
				row.key, row.recordWidth, len(bytes))
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
	const prefix = 6 + ic7610.AddressBytes // FE FE 98 E0 1A 00 <ch-hi> <ch-lo>
	if len(frame) != prefix+ic7610.RecordOnlyLength+1 {
		t.Fatalf("BuildMemorySet produced %d frame bytes, want %d (6 overhead + %d address + %d record + FD)",
			len(frame), prefix+ic7610.RecordOnlyLength+1, ic7610.AddressBytes, ic7610.RecordOnlyLength)
	}
	built := frame[prefix : len(frame)-1]
	if string(built) != string(assembled) {
		t.Errorf("THE WITNESS AND THE BUILDER DISAGREE - this is a STOP for arbitration against PDF p.12.\n"+
			"  assembled from the witness (%d bytes): % X\n"+
			"  built by BuildMemorySet     (%d bytes): % X\n"+
			"  first differing record offset: %s",
			len(assembled), assembled, len(built), built, firstDiff(assembled, built))
	}

	// --- 6. Parse back ----------------------------------------------------
	answer := append([]byte{0xFE, 0xFE, 0xE0, 0x98, 0x1A, 0x00, 0x00, 0x01}, assembled...)
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
	// E6's READ-SIDE consequence: an unmapped region is not decoded.
	if !rec.Select.Unavailable() {
		t.Errorf("parsed Select = %v, want Unavailable - printed (3)'s low nibble is UNMAPPED under ruling E6 and is never decoded", rec.Select)
	}
	if !rec.DataMode.Unavailable() {
		t.Errorf("parsed DataMode = %v, want Unavailable - printed (11)'s high nibble is UNMAPPED under ruling E6 and is never decoded", rec.DataMode)
	}

	// --- 7. The unmapped regions, both directions -------------------------
	t.Run("the_encoder_always_writes_the_template", func(t *testing.T) {
		cmd, err := p.BuildMemorySet(goldenRecord())
		if err != nil {
			t.Fatalf("BuildMemorySet(goldenRecord()): %v", err)
		}
		built := cmd.Bytes()[prefix : len(cmd.Bytes())-1]
		if built[ic7610.SelectNibbleOffset] != 0x00 {
			t.Errorf("BuildMemorySet emitted %#02x at record offset %d, want 0x00 - the encoder writes the Fixed template at every unmapped region",
				built[ic7610.SelectNibbleOffset], ic7610.SelectNibbleOffset)
		}
		if built[ic7610.DataModeNibbleOffset]&0xF0 != 0x00 {
			t.Errorf("BuildMemorySet emitted %#02x at record offset %d, want a ZERO high nibble",
				built[ic7610.DataModeNibbleOffset], ic7610.DataModeNibbleOffset)
		}
	})

	// A neutral record that CLAIMS one of the two unmapped fields is not
	// encoded with the template silently: it is REFUSED, and the refusal is
	// asserted here rather than swallowed. E6 forbids rewriting an unmapped
	// region, and dropping a value the caller supplied would be exactly the
	// silent rewrite the ruling exists to prevent - so core/civ names the
	// field and stops. Two arms, because the two regions are independent
	// evidence and neither capture speaks for the other.
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
				t.Errorf("%s: BuildMemorySet SUCCEEDED, producing % X.\nRuling E6 says an unmapped region is never rewritten, so a record carrying a value the 25-byte layout has nowhere to put must be refused with the field named - dropping it would write a record the caller did not ask for.",
					tc.name, cmd.Bytes())
				continue
			}
			if !strings.Contains(err.Error(), tc.field) {
				t.Errorf("%s: BuildMemorySet refused, but the error does not name %q, so a caller cannot tell WHICH field it may not send:\n  %v",
					tc.name, tc.field, err)
			}
		}
	})

	t.Run("the_gate_refuses_a_non_template_unmapped_region", func(t *testing.T) {
		// The gate's re-encode rule (spec Erratum 2) enforces E6
		// INDEPENDENTLY of the driver: it decodes, re-validates and
		// re-encodes byte-identically, so a set carrying a non-zero unmapped
		// nibble cannot survive it.
		for _, tc := range []struct {
			name   string
			offset int
			value  byte
		}{
			{"an E6-unmapped SELECT marker of 2", ic7610.SelectNibbleOffset, 0x02},
			{"an E6-unmapped data mode of 2", ic7610.DataModeNibbleOffset, 0x21},
		} {
			mutated := make([]byte, len(frame))
			copy(mutated, frame)
			mutated[prefix+tc.offset] = tc.value
			if p.AllowedCommand(mutated) {
				t.Errorf("%s: AllowedCommand admitted a set whose record byte %d is %#02x. The gate is the last defence before a radio sees bytes, and ruling E6 says a slot whose unmapped regions differ from the template is REFUSED, never rewritten.\n  frame: % X",
					tc.name, tc.offset, tc.value, mutated)
			}
		}
		// The unmutated frame must still be admitted, or the test above
		// proves nothing.
		if !p.AllowedCommand(frame) {
			t.Fatalf("AllowedCommand refused the golden set frame itself: % X", frame)
		}
	})
}

// wireByte returns the wire value an enum span gives a neutral name, read out
// of THE PROFILE'S OWN layout rather than restated as a literal: the point of
// the assembly is that the witness supplies the POSITION and the profile
// supplies the value, so that neither can quietly agree with itself.
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

// firstDiff names the first offset at which two records differ, for the
// arbitration's input.
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
