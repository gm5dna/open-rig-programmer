// SPDX-License-Identifier: GPL-3.0-or-later

package ic905_test

// HOW THIS TEST COMPARES, AND WHY IT IS NOT A RESTATEMENT.
//
// It does NOT copy the witness's numbers into constants and assert them
// equal to themselves. Each record is ASSEMBLED FROM THE WITNESS: a byte
// slice of the witnessed length is filled by placing each field's
// expected content at that field's OWN witnessed positions, converted
// with Task 7's recordOffsetFor, and the assembled record is then put to
// the profile's own parser and compared with the golden vector's record
// byte for byte. A witness position one byte out therefore fails as a
// record mismatch or a parse mismatch, not as an arithmetic
// disagreement between two hand-copied numbers. core/cat/ftdx101's
// geometry test states the same rule for its own witness.
//
// EVERY WITNESSED ROW IS CONSUMED. An unknown row, a missing row, a gap,
// an overlap or a row this test does not understand is a failure. There
// is no skip path: a silently ignored row is a piece of evidence that
// stopped being evidence.

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ/ic905"
)

// witnessRow is one measured row of ic905-geometry-witness.csv: the
// printed index it carries, and the byte positions W MEASURED for it on
// the render.
type witnessRow struct {
	key                   string
	printedFirst          int
	printedLast           int
	firstByte, lastByte   int
	rawIndex, visualLabel string
}

// loadGeometryWitness reads the witness's D1 rows in file order. It has
// its own loader rather than Task 7's because the witness names its
// label column block_label_verbatim and carries the two measured
// position columns no other artefact has.
func loadGeometryWitness(t *testing.T) []witnessRow {
	t.Helper()
	path := filepath.Join(evidenceDir, "ic905-geometry-witness.csv")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer f.Close()
	recs, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	col := func(header string) int {
		i := slices.Index(recs[0], header)
		if i < 0 {
			t.Fatalf("%s has no %q column; its header is %v", path, header, recs[0])
		}
		return i
	}
	diagram, index := col("diagram_id"), col("field_index")
	first, last, anchor := col("first_byte"), col("last_byte"), col("visual_anchor")

	var out []witnessRow
	for _, rec := range recs[1:] {
		if rec[diagram] != "D1" {
			continue
		}
		where := fmt.Sprintf("geometry witness row %q", rec[index])
		pf, pl := parseFieldIndex(t, where, rec[index])
		fb, err := strconv.Atoi(rec[first])
		if err != nil {
			t.Fatalf("%s: first_byte %q is not a number", where, rec[first])
		}
		lb, err := strconv.Atoi(rec[last])
		if err != nil {
			t.Fatalf("%s: last_byte %q is not a number", where, rec[last])
		}
		out = append(out, witnessRow{
			key: spanKey(pf, pl), printedFirst: pf, printedLast: pl,
			firstByte: fb, lastByte: lb, rawIndex: rec[index],
			visualLabel: strings.SplitN(rec[anchor], ";", 2)[0],
		})
	}
	return out
}

// witnessContentShort is the byte content each printed row takes for the
// value set the golden 68-byte vector carries, transcribed BY HAND from
// ic905-golden-assumptions.csv's field map — never read back out of the
// golden frame, because content derived from the thing it is compared
// with proves nothing.
//
// Each entry names the printed index, the value and where the value came
// from. The frequency row is absent: it is the one row whose width
// depends on the layout, and witnessFrequency supplies both forms.
var witnessContentShort = map[string][]byte{
	// (5) Split and Select memory setting. Left nibble printed 0 and
	// labelled "Fixed"; right nibble 0 = OFF, from the printed list
	// 0=OFF* / 1=star1 / 2=star2 / 3=star3 (PDF p.19, folio 18).
	"5-5": {0x00},
	// (11) Operating mode: 05 = FM, PDF p.17 (folio 16), column
	// "(1)Operating mode", row "05:FM".
	"11-11": {0x05},
	// (12) Filter setting: 01 = FIL1, same table, column "(2)Filter
	// setting", first row.
	"12-12": {0x01},
	// (13) Data mode: 00 = Data mode OFF, PDF p.19's own legend.
	"13-13": {0x00},
	// (14) ONE byte carrying TWO nibbles: left 0 = Duplex OFF, right
	// 0 = tone OFF, from the (14) breakout box's two leaders.
	"14-14": {0x00},
	// (15) Digital squelch: left nibble 0 = function OFF, right nibble
	// printed 0 and labelled "Fixed".
	"15-15": {0x00},
	// (16)~(18) and (19)~(21) both carry 88.5 Hz: byte (1) both halves
	// printed 0 and labelled "Fixed digit: 0*", then (100Hz 0)(10Hz 8),
	// then (1Hz 8)(0.1Hz 5) — PDF p.24 (folio 23).
	"16-18": {0x00, 0x08, 0x85},
	"19-21": {0x00, 0x08, 0x85},
	// (22)~(24) is THREE bytes carrying TWO fields: (22) polarity,
	// transmit 0=Normal and receive 0=Normal; (23),(24) DTCS code 023,
	// whose first byte is "0 (fixed)" then "First digit: 0 ~ 7".
	"22-24": {0x00, 0x00, 0x23},
	// (25) DV digital code squelch 00, both digits inside the printed
	// range 0 ~ 9 (PDF p.24, folio 23, command 1B 07).
	"25-25": {0x00},
	// (26)~(28) Duplex offset 000.000 MHz, all six printed digits 0.
	"26-28": {0x00, 0x00, 0x00},
	// (29)~(36), (37)~(44), (45)~(52): three call signs, eight spaces
	// apiece. "(Space) | 20" is printed in the call-sign character table
	// PDF p.19 points these fields at (PDF p.24, folio 23).
	"29-36": bytes.Repeat([]byte{0x20}, 8),
	"37-44": bytes.Repeat([]byte{0x20}, 8),
	"45-52": bytes.Repeat([]byte{0x20}, 8),
	// 53~68: the memory name, from PDF p.20 (folio 19)'s two character
	// tables. Its NINTH byte is the ASSUMED 0x20 (G's assumption A2; D5
	// entry 3; lift ic905-R-16) — the one byte of this record that no
	// printed table governing this field supplies.
	"53-68": []byte("HIGHLAND BASE905"),
	// (1),(2) and (3),(4) are the channel ADDRESS, which spec Erratum 1
	// excludes from the record: group 00, channel 01, chosen inside the
	// printed ranges. They are written into the frame's address field
	// and contribute NOTHING to the record.
	"1-2": {0x00, 0x00},
	"3-4": {0x00, 0x01},
}

// witnessFrequency is the operating-frequency field in each of its two
// widths, transcribed from PDF p.17 (folio 16)'s rotated nibble labels
// — "10 Hz digit", "1 Hz", "1 kHz", "100 Hz", "100 kHz", "10 kHz",
// "10 MHz", "1 MHz", "1 GHz", "100 MHz", and in the wide form a sixth
// byte whose halves are "100 GHz digit: 0 (Fixed)" and "10 GHz digit:
// 0, 1".
//
// 144.500000 MHz at five bytes; 10250.000000 MHz at six. The wide form
// is where the printed indices stop: the memory-content diagram prints
// NO index for that sixth byte (G's hazard (d), STOP 1).
func witnessFrequency(freqBytes int) []byte {
	if freqBytes == 6 {
		return []byte{0x00, 0x00, 0x00, 0x50, 0x02, 0x01}
	}
	return []byte{0x00, 0x00, 0x50, 0x44, 0x01}
}

// loadGoldenVectors parses testdata/ic905-vectors.golden: one vector
// per line, "name<TAB>space-separated hex bytes", LF only, no trimming
// beyond the line ending, and no blank lines tolerated.
//
// It lives in this file because Task 8 is the first task that needs a
// golden record; Task 9's golden_test.go consumes the same loader and
// adds requireVectorNames, which pins the count AND the order.
func loadGoldenVectors(t *testing.T) map[string][]byte {
	t.Helper()
	path := filepath.Join(evidenceDir, "ic905-vectors.golden")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Fatalf("%s does not end in a newline", path)
	}
	out := make(map[string][]byte)
	for i, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		name, hexBytes, ok := strings.Cut(line, "\t")
		if !ok {
			t.Fatalf("%s line %d: no tab separator in %q", path, i+1, line)
		}
		if name == "" || hexBytes == "" {
			t.Fatalf("%s line %d: empty name or body", path, i+1)
		}
		var frame []byte
		for _, tok := range strings.Split(hexBytes, " ") {
			b, err := strconv.ParseUint(tok, 16, 8)
			if err != nil || len(tok) != 2 {
				t.Fatalf("%s line %d: %q is not a two-digit hex byte", path, i+1, tok)
			}
			frame = append(frame, byte(b))
		}
		if _, dup := out[name]; dup {
			t.Fatalf("%s line %d: vector %q appears twice", path, i+1, name)
		}
		out[name] = frame
	}
	return out
}

// goldenRecord returns the RECORD bytes of a 1A 00 set vector: the frame
// less its six-byte header, its four address bytes and its terminator.
func goldenRecord(t *testing.T, frame []byte) []byte {
	t.Helper()
	if len(frame) < 12 || frame[len(frame)-1] != 0xFD {
		t.Fatalf("golden set vector is %d bytes and does not end FD", len(frame))
	}
	return frame[10 : len(frame)-1]
}

func TestGeometry_TheWitnessAssemblesTheGoldenRecords(t *testing.T) {
	rows := loadGeometryWitness(t)
	// 1. Exactly eighteen measured rows on the memory-content diagram.
	if len(rows) != 18 {
		t.Fatalf("the geometry witness has %d D1 rows, want 18", len(rows))
	}

	// 2. The MEASURED positions tile 1...68 contiguously — W's own
	//    claim, "no gap, no overlap", with its 22 drawn units standing
	//    for 24 printed bytes in band 1 and 16 for 44 in band 2, the
	//    ellipses reconciled. The measured columns are checked against
	//    the printed index they carry, which is the join key.
	next := 1
	for _, r := range rows {
		if r.firstByte != next {
			t.Fatalf("witness row %s measures bytes %d..%d, but the previous row ended at %d — the witness must tile 1..68 with no gap and no overlap", r.rawIndex, r.firstByte, r.lastByte, next-1)
		}
		if r.firstByte != r.printedFirst || r.lastByte != r.printedLast {
			t.Errorf("witness row %s measures bytes %d..%d but its printed index says %d..%d", r.rawIndex, r.firstByte, r.lastByte, r.printedFirst, r.printedLast)
		}
		next = r.lastByte + 1
	}
	if next != 69 {
		t.Fatalf("the witness's measured positions end at byte %d, want 68", next-1)
	}

	for _, tc := range []struct {
		freqBytes int
		vector    string
		wantFreq  uint64
	}{
		{5, "set-record-name-with-space-68", 144_500_000},
		{6, "set-record-name-with-space-69", 10_250_000_000},
	} {
		t.Run(tc.vector, func(t *testing.T) {
			// 3. Assemble the record by writing each witnessed row's
			//    content at that row's OWN measured positions.
			length := ic905.RecordLengthShort + tc.freqBytes - 5
			record := make([]byte, length)
			written := make([]bool, length)
			addr := make([]byte, 0, 4)
			consumed := 0

			for _, r := range rows {
				start := recordOffsetFor(r.firstByte, tc.freqBytes)
				// The row ends where the NEXT printed byte starts, which
				// is what gives the frequency row its sixth byte in the
				// wide layout and changes nothing anywhere else.
				end := recordOffsetFor(r.lastByte+1, tc.freqBytes)

				var content []byte
				switch {
				case r.key == "6-10":
					content = witnessFrequency(tc.freqBytes)
				default:
					var ok bool
					content, ok = witnessContentShort[r.key]
					if !ok {
						t.Fatalf("witness row %s is a row this test does not understand — there is no skip path, because a silently ignored row is a piece of evidence that stopped being evidence", r.rawIndex)
					}
				}
				consumed++

				// The two ADDRESS rows contribute nothing to the record.
				if addressRows[r.key] {
					addr = append(addr, content...)
					continue
				}
				if len(content) != end-start {
					t.Fatalf("witness row %s measures %d record bytes in the %d-byte layout, but its content is %d bytes", r.rawIndex, end-start, length, len(content))
				}
				for i, b := range content {
					if written[start+i] {
						t.Fatalf("witness row %s writes record offset %d twice", r.rawIndex, start+i)
					}
					record[start+i] = b
					written[start+i] = true
				}
			}

			// 6. Every witnessed row consumed, and every record byte
			//    written by one of them.
			if consumed != len(rows) {
				t.Fatalf("consumed %d witness rows of %d", consumed, len(rows))
			}
			for i, w := range written {
				if !w {
					t.Fatalf("record offset %d was written by no witnessed row", i)
				}
			}
			if !bytes.Equal(addr, []byte{0x00, 0x00, 0x00, 0x01}) {
				t.Fatalf("the two address rows assembled % X, want 00 00 00 01", addr)
			}

			// 3, concluded — the strongest assertion in this file: the
			// record the WITNESS assembles must be the record the GOLDEN
			// VECTOR carries, byte for byte.
			want := goldenRecord(t, loadGoldenVectors(t)[tc.vector])
			if !bytes.Equal(record, want) {
				t.Fatalf("the witness-assembled %d-byte record is not the golden vector's:\n  assembled % X\n  golden    % X", length, record, want)
			}

			// 4 and 5. The assembled record through the profile's own
			//    parser, against the same neutral values Task 4 asserts.
			rec, err := ic905.Profile().ParseMemoryAnswer(answerFrame(addr, record))
			if err != nil {
				t.Fatalf("ParseMemoryAnswer: %v", err)
			}
			assertNumeric(t, "rx_frequency", rec.RXFreqHz, tc.wantFreq)
			assertNumeric(t, "tone_tx", rec.ToneTXDeciHz, 885)
			assertNumeric(t, "tone_rx", rec.ToneRXDeciHz, 885)
			assertNumeric(t, "dtcs_code", rec.DTCSCode, 23)
			assertNumeric(t, "offset", rec.OffsetHz, 0)
			assertText(t, "mode", rec.Mode, "FM")
			assertText(t, "filter", rec.Filter, "FIL1")
			assertText(t, "data_mode", rec.DataMode, "OFF")
			assertText(t, "duplex", rec.Duplex, "OFF")
			assertText(t, "tone_mode", rec.ToneMode, "OFF")
			assertText(t, "dtcs_polarity", rec.DTCSPolarity, "NN")
			assertText(t, "name", rec.Name, "HIGHLAND BASE905")
		})
	}
}
