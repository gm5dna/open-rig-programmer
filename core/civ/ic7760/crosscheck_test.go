// SPDX-License-Identifier: GPL-3.0-or-later

package ic7760_test

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7760"
)

type printedRange struct{ lo, hi int }

func parsePrinted(s string) (printedRange, error) {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, r := range s {
		n := 0
		switch {
		case r >= '①' && r <= '⑳':
			n = int(r-'①') + 1
		case r >= '㉑' && r <= '㉟':
			n = int(r-'㉑') + 21
		}
		if n != 0 {
			b.WriteString(strconv.Itoa(n))
		} else {
			b.WriteRune(r)
		}
	}
	parts := strings.Split(b.String(), " ~ ")
	if len(parts) == 1 {
		parts = strings.Split(b.String(), ", ")
	}
	if len(parts) == 1 {
		n, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		return printedRange{n, n}, err
	}
	if len(parts) != 2 {
		return printedRange{}, fmt.Errorf("bad index %q", s)
	}
	lo, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return printedRange{}, err
	}
	hi, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return printedRange{}, err
	}
	return printedRange{lo, hi}, nil
}

func rangeKey(t *testing.T, s string) string {
	t.Helper()
	r, err := parsePrinted(s)
	if err != nil {
		t.Fatalf("parse printed range %q: %v", s, err)
	}
	return fmt.Sprintf("%d-%d", r.lo, r.hi)
}

func readCSV(t *testing.T, name string) [][]string {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

type joinedField struct {
	label, encoding string
	valueFragments  []string
}

var joinedFields = map[string]joinedField{
	"1-2":   {"Memory group number", "enum_byte", []string{"00 01 ~ 00 99", "01 00: Programmed scan edge P1", "01 01: Programmed scan edge P2"}},
	"3-3":   {"Select memory setting", "enum_nibble", []string{"0=OFF", "1= ★1", "2= ★2", "3= ★3"}},
	"4-8":   {"Operating frequency setting", "bcd_packed", []string{"10 MHz digit: 0 ~ 6", "1 GHz digit: 0 (Fixed)", "100 MHz digit: 0 (Fixed)"}},
	"9-10":  {"Operating mode setting", "enum_byte", []string{"00:LSB", "01:USB", "12:PSK", "13:PSK-R", "01:FIL1", "03:FIL3"}},
	"11-11": {"Data mode and tone type settings", "enum_nibble", []string{"0: OFF, 1: DATA 1, 2: DATA 2, 3: DATA 3", "0: OFF, 1: TONE, 2: TSQL"}},
	"12-14": {"Repeater tone frequency setting", "bcd_packed", []string{"Fixed digit: 0*", "100Hz digit: 0 ~ 2", "0.1 Hz digit: 0 ~ 9"}},
	"15-17": {"Tone squelch frequency setting", "bcd_packed", []string{"Fixed digit: 0*", "100Hz digit: 0 ~ 2", "0.1 Hz digit: 0 ~ 9"}},
	"18-27": {"Memory name settings", "ascii", []string{"A ~ Z 41 ~ 5A", "a ~ z 61 ~ 7A", "0 ~ 9 30 ~ 39", "@ 40"}},
}

// TestCrosscheckKeyedLBWJoin turns the L/W/B evidence comparison into a
// keyed assertion. A missing label, width, encoding, enum or measured span
// is therefore a test failure, not an empty STOP-labelled subtest.
func TestCrosscheckKeyedLBWJoin(t *testing.T) {
	ledger := readCSV(t, "IC-7760-field-ledger.csv")
	transcription := readCSV(t, "IC-7760-transcription-b.csv")
	witness := readCSV(t, "IC-7760-geometry-witness.csv")

	lRows, bRows, wRows := map[string][]string{}, map[string][]string{}, map[string][]string{}
	for _, row := range ledger[1:] {
		if row[0] == "D1" {
			lRows[rangeKey(t, row[1])] = row
		}
	}
	for _, row := range transcription[1:] {
		if row[0] == "D1" && row[3] != "0" { // the width-zero row is the separate clear form
			bRows[rangeKey(t, row[1])] = row
		}
	}
	for _, row := range witness[1:] {
		if row[0] == "D1" {
			wRows[rangeKey(t, row[1])] = row
		}
	}
	if len(lRows) != 8 || len(bRows) != 8 || len(wRows) != 8 {
		t.Fatalf("keyed D1 cardinality L/B/W = %d/%d/%d, want 8/8/8", len(lRows), len(bRows), len(wRows))
	}

	for key, want := range joinedFields {
		t.Run(key, func(t *testing.T) {
			l, lok := lRows[key]
			b, bok := bRows[key]
			w, wok := wRows[key]
			if !lok || !bok || !wok {
				t.Fatalf("STOP: keyed join %s has L/B/W=%t/%t/%t", key, lok, bok, wok)
			}
			if l[3] != want.label || b[2] != want.label {
				t.Errorf("labels L/B = %q/%q, want %q", l[3], b[2], want.label)
			}
			r, _ := parsePrinted(l[1])
			width, err := strconv.Atoi(b[3])
			if err != nil || width != r.hi-r.lo+1 {
				t.Errorf("B width = %q, want %d from printed range", b[3], r.hi-r.lo+1)
			}
			first, firstErr := strconv.Atoi(w[3])
			last, lastErr := strconv.Atoi(w[5])
			if firstErr != nil || lastErr != nil || first != r.lo || last != r.hi || w[4] != "1" || w[6] != "2" {
				t.Errorf("W span = byte %s nibble %s .. byte %s nibble %s, want %d/1..%d/2", w[3], w[4], w[5], w[6], r.lo, r.hi)
			}
			if b[4] != want.encoding {
				t.Errorf("B encoding = %q, want %q", b[4], want.encoding)
			}
			for _, fragment := range want.valueFragments {
				if !strings.Contains(b[5], fragment) {
					t.Errorf("B values for %s do not contain %q", key, fragment)
				}
			}
		})
	}
}

func TestCrosscheckSubdiagramsAndSTOPs(t *testing.T) {
	ledger := readCSV(t, "IC-7760-field-ledger.csv")
	transcription := readCSV(t, "IC-7760-transcription-b.csv")
	witness := readCSV(t, "IC-7760-geometry-witness.csv")

	for _, stop := range []string{"STOP 1", "STOP 2", "STOP 3", "STOP 4", "STOP 5", "STOP 6", "STOP 7"} {
		found := false
		for _, row := range witness[1:] {
			found = found || row[0] == "D1" && strings.Contains(row[9], stop)
		}
		if !found {
			t.Errorf("W arbitration %s has no pinned evidence row", stop)
		}
	}

	var lD2, lD3, wD2, wD3 []string
	var bD2, bD3 [][]string
	for _, row := range ledger[1:] {
		switch row[0] {
		case "D2":
			lD2 = row
		case "D3":
			lD3 = row
		}
	}
	for _, row := range witness[1:] {
		switch row[0] {
		case "D2":
			wD2 = row
		case "D3":
			wD3 = row
		}
	}
	for _, row := range transcription[1:] {
		switch row[0] {
		case "D2":
			bD2 = append(bD2, row)
		case "D3":
			bD3 = append(bD3, row)
		}
	}
	if lD2[1] != "3" || wD2[3] != "3" || wD2[4] != "1" || wD2[6] != "2" || len(bD2) != 2 {
		t.Errorf("L/W/B D2 join does not pin byte 3 and its two half-byte rows")
	}
	if !strings.Contains(lD2[6], "STOP 1") || !strings.Contains(wD2[9], "STOP 8") {
		t.Errorf("duplicate index 3 arbitration missing: L=%q W=%q", lD2[6], wD2[9])
	}
	if bD2[0][3] != "0.5" || bD2[0][4] != "fixed" || bD2[0][5] != "0" || bD2[1][3] != "0.5" || bD2[1][4] != "enum_nibble" || bD2[1][5] != "0=OFF | 1= ★1 | 2= ★2 | 3= ★3" {
		t.Errorf("D2 fixed/enum halves = %#v", bD2)
	}
	if lD3[1] != "11" || wD3[3] != "11" || wD3[4] != "1" || wD3[6] != "2" || len(bD3) != 2 {
		t.Errorf("L/W/B D3 join does not pin byte 11 and its two half-byte rows")
	}
	if !strings.Contains(lD3[6], "STOP 2") || !strings.Contains(wD3[9], "STOP 9") {
		t.Errorf("duplicate index 11 arbitration missing: L=%q W=%q", lD3[6], wD3[9])
	}
	if bD3[0][3] != "0.5" || bD3[0][5] != "0: OFF, 1: DATA 1, 2: DATA 2, 3: DATA 3" || bD3[1][3] != "0.5" || bD3[1][5] != "0: OFF, 1: TONE, 2: TSQL" {
		t.Errorf("D3 data/tone halves = %#v", bD3)
	}

	clearRows := 0
	toneHeadingStops := 0
	for _, row := range transcription[1:] {
		if row[0] == "D1" && (rangeKey(t, row[1]) == "12-14" || rangeKey(t, row[1]) == "15-17") {
			if row[6] == "24" && strings.Contains(row[7], "PDF page 20") && strings.Contains(row[8], "STOP 1") {
				toneHeadingStops++
			}
		}
		if row[0] == "D1" && row[3] == "0" {
			clearRows++
			if rangeKey(t, row[1]) != "4-4" || row[4] != "conditional" || row[5] != "None" || !strings.Contains(row[8], "To clear") {
				t.Errorf("clear row = %#v", row)
			}
		}
	}
	if clearRows != 1 {
		t.Errorf("width-zero clear rows = %d, want 1", clearRows)
	}
	if toneHeadingStops != 2 {
		t.Errorf("B tone-heading STOP rows citing PDF p.20 and p.24 = %d, want 2", toneHeadingStops)
	}
	for _, row := range ledger[1:] {
		if row[0] == "D1" && rangeKey(t, row[1]) == "4-8" && !strings.Contains(row[6], "STOP 3") {
			t.Errorf("L clear-list duplicate index arbitration missing from %q", row[6])
		}
	}
	clear := []byte{0xFE, 0xFE, 0xB2, 0xE0, 0x1A, 0x00, 0x00, 0x01, 0xFF, 0xFD}
	if ic7760.Profile().AllowedCommand(clear) {
		t.Error("the conditional clear evidence was admitted as executable grammar")
	}

	// E1 is executed as channel semantics, not a group field: the same two
	// bytes carry MEM 99, P1 and P2 consecutively.
	for channel, wantAddress := range map[int][]byte{99: {0x00, 0x99}, 100: {0x01, 0x00}, 101: {0x01, 0x01}} {
		cmd, err := ic7760.Profile().BuildMemoryRead(civ.ChannelAddress{Channel: channel})
		if err != nil {
			t.Errorf("E1 channel %d: %v", channel, err)
			continue
		}
		got := cmd.Bytes()
		if got[6] != wantAddress[0] || got[7] != wantAddress[1] {
			t.Errorf("E1 channel %d address = %02X %02X, want % X", channel, got[6], got[7], wantAddress)
		}
	}
}

func TestCrosscheckGoldenCoversAllTwentySevenFields(t *testing.T) {
	rows := readCSV(t, "IC-7760-golden-assumptions.csv")
	covered := make(map[int]bool)
	for _, row := range rows[1:] {
		if row[0] != "set-record-name-with-space" || row[6] == "-" {
			continue
		}
		var fields []int
		for _, part := range strings.Split(row[6], " | ") {
			n, err := strconv.Atoi(part)
			if err != nil {
				t.Fatalf("G field index %q: %v", row[6], err)
			}
			fields = append(fields, n)
			covered[n] = true
		}
		first, _ := strconv.Atoi(row[1])
		last, _ := strconv.Atoi(row[3])
		if first-6 != fields[0] || last-6 != fields[len(fields)-1] {
			t.Errorf("G frame bytes %d..%d do not key to record fields %v", first, last, fields)
		}
	}
	for field := 1; field <= 27; field++ {
		if !covered[field] {
			t.Errorf("G has no set-vector evidence for record field %d", field)
		}
	}
}

// PDF p.20 (folio 19) says DATA 1/2/3; PDF p.24 (folio 23), under
// “② Data mode setting”, says Data mode 1 (D1)/(D2)/(D3). The frozen G
// arbitration chooses p.20 because it is the record field's own diagram.
// This test executes that choice and also pins the undisputed OFF byte used
// by the golden vector, while the codec deliberately leaves the lossy
// four-valued field unmapped.
func TestSTOPG1ArbitrationUsesThePage20DataModeNames(t *testing.T) {
	provenance, err := os.ReadFile(filepath.Join("testdata", "IC-7760-golden-provenance.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(provenance)
	for _, exact := range []string{
		"PDF page 20 (folio 19) contradicts PDF page 24 (folio 23)",
		"0: OFF, 1: DATA 1, 2: DATA 2, 3: DATA 3",
		"02: Data mode 1 (D2)",
		"03: Data mode 1 (D3)",
		"What I built from: the PDF page 20 statement",
	} {
		if !strings.Contains(text, exact) {
			t.Errorf("G1 provenance does not contain %q", exact)
		}
	}

	rows := readCSV(t, "IC-7760-transcription-b.csv")
	var chosen string
	for _, row := range rows[1:] {
		if row[0] == "D3" && strings.Contains(row[5], "DATA 1") {
			chosen = row[5]
		}
	}
	if chosen != "0: OFF, 1: DATA 1, 2: DATA 2, 3: DATA 3" {
		t.Fatalf("chosen D3 domain = %q", chosen)
	}

	golden := goldenVectors(t)["set-record-name-with-space"]
	if golden[16] != 0x00 { // CSV byte 17; slice index 16.
		t.Errorf("G1 golden data/tone byte = %02X, want undisputed OFF/OFF 00", golden[16])
	}
	assumptions := readCSV(t, "IC-7760-golden-assumptions.csv")
	g1Rows := 0
	for _, row := range assumptions[1:] {
		if row[0] == "set-record-name-with-space" && row[1] == "17" && row[6] == "11" && row[7] == "manual_derived" && strings.Contains(row[10], "STOP 1") {
			g1Rows++
		}
	}
	if g1Rows != 1 {
		t.Errorf("G1 keyed arbitration rows = %d, want 1", g1Rows)
	}
	answer := append([]byte{0xFE, 0xFE, 0xE0, 0xB2}, golden[4:len(golden)-1]...)
	answer = append(answer, civ.EndByte)
	if _, _, err := ic7760.Profile().MemoryAnswerRecord(answer); err != nil {
		t.Fatalf("chosen OFF/OFF record is not executable: %v", err)
	}
}
