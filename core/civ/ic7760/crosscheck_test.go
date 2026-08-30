// SPDX-License-Identifier: GPL-3.0-or-later

package ic7760_test

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
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

// ---------------------------------------------------------------------------
// Complete printed domains.
//
// The keyed join above compares each field's values_verbatim against a
// SAMPLE of its printed entries, which is enough to catch a transposed or
// mislabelled row but not a missing one. Everything below asserts the
// COMPLETE printed domain instead: the enumerable fields — memory name,
// operating mode, filter setting and tone type — are reconstructed entry by
// entry from the frozen B artefact and compared byte for byte against the
// shipped constant and the layout's enums, so an omitted code is a failure
// rather than an unsampled gap.

// splitVerbatim splits a frozen values_verbatim cell on the printed
// separator " | ". The symbols table itself prints a vertical bar as a
// character — the entry reading "| 7C" — and the frozen row's own notes
// column records that the padded separator is what keeps the two apart, so
// splitting on " | " leaves that entry intact.
func splitVerbatim(s string) []string {
	parts := strings.Split(s, " | ")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func printedCode(t *testing.T, s string) byte {
	t.Helper()
	n, err := strconv.ParseUint(s, 16, 8)
	if err != nil {
		t.Fatalf("printed code %q: %v", s, err)
	}
	return byte(n)
}

// d1TranscriptionRow returns the single frozen D1 row keyed by the given
// printed index range, skipping the width-zero conditional clear row.
func d1TranscriptionRow(t *testing.T, key string) []string {
	t.Helper()
	for _, row := range readCSV(t, "IC-7760-transcription-b.csv")[1:] {
		if row[0] == "D1" && row[3] != "0" && rangeKey(t, row[1]) == key {
			return row
		}
	}
	t.Fatalf("frozen transcription has no D1 row keyed %s", key)
	return nil
}

// printedNameCodes derives every character code the two "Codes for
// character entries" tables print, from the frozen name row's
// values_verbatim. The glyph column is not consulted: three of the printed
// glyphs are drawn as typographic look-alikes (” for 22, ’ for 27 and an
// en-dash-length stroke for 2D, which the -b.md transcription annotates
// "transcribed as drawn"), so the ASCII code column is the authority.
func printedNameCodes(t *testing.T, verbatim string) []byte {
	t.Helper()
	var codes []byte
	for _, entry := range splitVerbatim(verbatim) {
		fields := strings.Fields(entry)
		switch len(fields) {
		case 2: // A single printed pair, e.g. "! 21" or "| 7C".
			codes = append(codes, printedCode(t, fields[1]))
		case 6: // A printed run, e.g. "A ~ Z 41 ~ 5A".
			if fields[1] != "~" || fields[4] != "~" {
				t.Fatalf("run entry %q is not '<c> ~ <c> <hex> ~ <hex>'", entry)
			}
			lo, hi := printedCode(t, fields[3]), printedCode(t, fields[5])
			if hi < lo {
				t.Fatalf("run entry %q ends below where it starts", entry)
			}
			for b := lo; b <= hi; b++ {
				codes = append(codes, b)
			}
		default:
			t.Fatalf("name entry %q has %d printed fields, want 2 or 6", entry, len(fields))
		}
	}
	return codes
}

// TestNameCharsetIsTheCompletePrintedDomain rebuilds NameCharset from the
// frozen evidence and compares it byte for byte. The letter, lower-case and
// digit runs come out in the order the "Letters and Numbers" table prints
// them; the symbols follow in ascending code order, which is the order of
// the p.20 footnote ("! \" # $ % & ' ( ) * +, - . / : ; < = > ? @ …") and is
// what fixes 2D between 2C (,) and 2E (.). The assumed space closes the set.
func TestNameCharsetIsTheCompletePrintedDomain(t *testing.T) {
	codes := printedNameCodes(t, d1TranscriptionRow(t, "18-27")[5])
	if len(codes) != 94 {
		t.Errorf("frozen evidence prints %d character codes, want the 94 the two tables carry", len(codes))
	}

	seen := map[byte]bool{}
	var upper, lower, digits, symbols []byte
	for _, b := range codes {
		if seen[b] {
			t.Errorf("frozen evidence prints code %02X twice", b)
		}
		seen[b] = true
		switch {
		case b >= 'A' && b <= 'Z':
			upper = append(upper, b)
		case b >= 'a' && b <= 'z':
			lower = append(lower, b)
		case b >= '0' && b <= '9':
			digits = append(digits, b)
		default:
			symbols = append(symbols, b)
		}
	}
	sort.Slice(symbols, func(i, j int) bool { return symbols[i] < symbols[j] })
	want := string(upper) + string(lower) + string(digits) + string(symbols) + " "

	if ic7760.NameCharset != want {
		t.Errorf("NameCharset does not reproduce the complete printed domain:\n got %q\nwant %q", ic7760.NameCharset, want)
	}
	if len(ic7760.NameCharset) != 95 {
		t.Errorf("NameCharset is %d bytes, want 95 — the 94 printed codes plus the assumed space", len(ic7760.NameCharset))
	}
	if got := string(ic7760.Profile().NameCharset()); got != ic7760.NameCharset {
		t.Errorf("the built profile's charset = %q, want the constant %q", got, ic7760.NameCharset)
	}
	// The printed hyphen-minus, named explicitly so a regression says so.
	if !strings.Contains(ic7760.NameCharset, ",-.") {
		t.Errorf("NameCharset does not carry 2D in printed order between 2C and 2E: %q", ic7760.NameCharset)
	}
	if bytes.IndexByte(ic7760.Profile().NameCharset(), 0x2D) < 0 {
		t.Error("the built profile refuses the printed hyphen-minus 2D")
	}
}

func layoutEnum(t *testing.T, field civ.FieldID) map[byte]string {
	t.Helper()
	for _, sp := range ic7760.Profile().Layouts()[0].Fields {
		if sp.Field == field {
			return sp.Enum
		}
	}
	t.Fatalf("layout has no span for field %v", field)
	return nil
}

// TestPrintedEnumDomainsAreCompleteNotSampled asserts the whole printed
// operating-mode, filter-setting and tone-type domains against the layout's
// enums, so a code the document prints and the profile omits — or the
// reverse — fails here rather than slipping through an unsampled entry.
func TestPrintedEnumDomainsAreCompleteNotSampled(t *testing.T) {
	wantMode, wantFilter := map[byte]string{}, map[byte]string{}
	target := wantMode
	for _, entry := range splitVerbatim(d1TranscriptionRow(t, "9-10")[5]) {
		entry = strings.TrimPrefix(entry, "①Operating mode ")
		if rest := strings.TrimPrefix(entry, "②Filter setting "); rest != entry {
			target, entry = wantFilter, rest
		}
		if entry == "—" { // The two printed em dashes carry no code.
			continue
		}
		code, name, ok := strings.Cut(entry, ":")
		if !ok {
			t.Fatalf("mode/filter entry %q is not '<hex>:<name>'", entry)
		}
		target[printedCode(t, code)] = name
	}
	if len(wantMode) != 10 || len(wantFilter) != 3 {
		t.Fatalf("printed mode/filter domains = %d/%d codes, want 10/3", len(wantMode), len(wantFilter))
	}
	if got := layoutEnum(t, civ.FieldMode); !reflect.DeepEqual(got, wantMode) {
		t.Errorf("mode enum is not the complete printed domain:\n got %v\nwant %v", got, wantMode)
	}
	if got := layoutEnum(t, civ.FieldFilter); !reflect.DeepEqual(got, wantFilter) {
		t.Errorf("filter enum is not the complete printed domain:\n got %v\nwant %v", got, wantFilter)
	}

	// The tone-type half of D3: the UPPER printed line, reached by the
	// arrow under the RIGHT digit, which is the low nibble the layout maps.
	var toneRow []string
	for _, row := range readCSV(t, "IC-7760-transcription-b.csv")[1:] {
		if row[0] == "D3" && strings.Contains(row[5], "TSQL") {
			toneRow = row
		}
	}
	if toneRow == nil {
		t.Fatal("frozen transcription has no D3 tone-type row")
	}
	wantTone := map[byte]string{}
	for _, entry := range strings.Split(toneRow[5], ", ") {
		code, name, ok := strings.Cut(strings.TrimSpace(entry), ": ")
		if !ok {
			t.Fatalf("tone entry %q is not '<code>: <name>'", entry)
		}
		wantTone[printedCode(t, code)] = name
	}
	if len(wantTone) != 3 {
		t.Fatalf("printed tone-type domain = %d codes, want 3", len(wantTone))
	}
	if got := layoutEnum(t, civ.FieldToneMode); !reflect.DeepEqual(got, wantTone) {
		t.Errorf("tone-mode enum is not the complete printed domain:\n got %v\nwant %v", got, wantTone)
	}
}

// TestRoundTripCarriesAHyphenatedName runs a record whose name uses the
// printed hyphen-minus through build → parse → re-encode, in the shape of
// the golden test. A repeater tag such as GB3-IV is the ordinary case that
// a charset hole makes unreadable: the read-path refusal is not a
// *civ.RecordLengthError, so probeSlot returns it unwrapped and Open aborts.
func TestRoundTripCarriesAHyphenatedName(t *testing.T) {
	p := ic7760.Profile()
	rec := goldenRecord()
	rec.Name = civ.Available("GB3-IV")

	set, err := p.BuildMemorySet(rec)
	if err != nil {
		t.Fatalf("build a record named GB3-IV: %v", err)
	}
	frame := set.Bytes()
	if len(frame) != 34 {
		t.Fatalf("set frame is %d bytes, want 34", len(frame))
	}
	// Frame index 8 is record offset 0, so the ten name bytes start at 23.
	if got, want := frame[23:33], []byte("GB3-IV    "); !bytes.Equal(got, want) {
		t.Errorf("encoded name = % X, want % X", got, want)
	}
	if frame[26] != 0x2D {
		t.Errorf("the hyphen was encoded as %02X, want the printed 2D", frame[26])
	}

	answer := append([]byte(nil), frame...)
	answer[2], answer[3] = answer[3], answer[2]
	got, err := p.ParseMemoryAnswer(answer)
	if err != nil {
		t.Fatalf("parse a record named GB3-IV: %v", err)
	}
	if got != rec {
		t.Errorf("hyphenated round trip did not compare field-for-field:\n got %+v\nwant %+v", got, rec)
	}
	reencoded, err := p.BuildMemorySet(got)
	if err != nil {
		t.Fatalf("re-encode the parsed record: %v", err)
	}
	if !bytes.Equal(reencoded.Bytes(), frame) {
		t.Errorf("re-encoded = % X, want byte-identical % X", reencoded.Bytes(), frame)
	}
}
