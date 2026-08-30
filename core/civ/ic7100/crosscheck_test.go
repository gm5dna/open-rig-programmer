// SPDX-License-Identifier: GPL-3.0-or-later

package ic7100

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
)

type evidenceTable struct {
	header []string
	rows   []map[string]string
}

func readEvidenceCSV(t *testing.T, name string) evidenceTable {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	if len(records) < 2 {
		t.Fatalf("%s has %d CSV records, want header and evidence", name, len(records))
	}
	table := evidenceTable{header: records[0]}
	for line, record := range records[1:] {
		if len(record) != len(table.header) {
			t.Fatalf("%s line %d has %d columns, want %d", name, line+2, len(record), len(table.header))
		}
		row := make(map[string]string, len(record))
		for i, value := range record {
			row[table.header[i]] = value
		}
		table.rows = append(table.rows, row)
	}
	return table
}

func normaliseIndex(row map[string]string) string {
	s := row["field_index"]
	replacements := []struct{ old, new string }{
		{"①", "1"}, {"②", "2"}, {"③", "3"}, {"④", "4"}, {"⑤", "5"},
		{"⑥", "6"}, {"⑦", "7"}, {"⑧", "8"}, {"⑨", "9"}, {"⑩", "10"},
		{"⑪", "11"}, {"⑫", "12"}, {"⑬", "13"}, {"⑭", "14"}, {"⑮", "15"},
		{"⑯", "16"}, {"⑰", "17"}, {"⑱", "18"}, {"⑲", "19"}, {"⑳", "20"},
		{"㉑", "21"}, {"㉒", "22"}, {"㉓", "23"}, {"㉔", "24"}, {"㉕", "25"},
		{"㉖", "26"}, {"㉗", "27"}, {"㉘", "28"}, {"㉙", "29"}, {"㉚", "30"},
		{"㉛", "31"}, {"㉜", "32"}, {"㉝", "33"}, {"㉞", "34"}, {"㉟", "35"},
		{"㊱", "36"}, {"㊲", "37"}, {"㊳", "38"}, {"㊴", "39"}, {"㊵", "40"},
		{"㊶", "41"}, {"㊷", "42"}, {"㊸", "43"}, {"㊹", "44"}, {"㊺", "45"},
		{"㊻", "46"}, {"㊼", "47"}, {"㊽", "48"}, {"㊾", "49"}, {"㊿", "50"},
		{"❺", "5"}, {"[51]", "51"}, {"(51)", "51"}, {"(52)", "52"},
		{"(60)", "60"}, {"(67)", "67"}, {"〜", "-"}, {"~", "-"},
		{"–", "-"}, {"、", ","},
	}
	for _, replacement := range replacements {
		s = strings.ReplaceAll(s, replacement.old, replacement.new)
	}
	filled := row["index_style"] == "filled" || strings.Contains(row["field_index"], "❺") ||
		strings.Contains(row["field_index"], "[51]") || strings.Contains(row["notes"], "FILLED / REVERSED")
	if filled {
		s = "filled:" + s
	}
	return s
}

func evidenceKey(row map[string]string) string {
	return row["diagram_id"] + "/" + normaliseIndex(row)
}

func indexEvidence(t *testing.T, name string, table evidenceTable) map[string]map[string]string {
	t.Helper()
	indexed := make(map[string]map[string]string, len(table.rows))
	for _, row := range table.rows {
		key := evidenceKey(row)
		if _, duplicate := indexed[key]; duplicate {
			t.Fatalf("%s has duplicate join key %q (original token %q)", name, key, row["field_index"])
		}
		indexed[key] = row
	}
	return indexed
}

func TestEvidenceCrosscheck(t *testing.T) {
	ledger := indexEvidence(t, "L", readEvidenceCSV(t, "IC-7100-field-ledger.csv"))
	witness := indexEvidence(t, "W", readEvidenceCSV(t, "IC-7100-geometry-witness.csv"))
	semantic := indexEvidence(t, "B", readEvidenceCSV(t, "IC-7100-transcription-b.csv"))

	// Join only diagram_id + field_index. The one B-only row is the body
	// text's 52–67 reading, retained below as an arbitration anchor rather
	// than being merged with the diagram bar's distinct 52–60 token.
	joined := 0
	for key, lrow := range ledger {
		if !strings.HasPrefix(key, "D1/") {
			continue
		}
		wrow, wok := witness[key]
		brow, bok := semantic[key]
		if !wok || !bok {
			t.Errorf("evidence join %q (L original %q): W present=%t B present=%t", key, lrow["field_index"], wok, bok)
			continue
		}
		if wrow["field_index"] == "" || brow["field_index"] == "" {
			t.Errorf("evidence join %q lost its original field-index token", key)
		}
		joined++
	}
	if joined != 18 {
		t.Errorf("joined %d D1 diagram rows, want 18", joined)
	}
	if _, ok := semantic["D1/52-67"]; !ok {
		t.Error("B evidence lost the independent body-text 52–67 name row")
	}

	// PDF arbitration ruling, page 375 (folio 20-16), recorded here so a
	// future reader cannot turn a STOP into an unexplained preference. The
	// local private PDF is not committed; the governing matrix §3.15.1 and
	// its independent review both record their 600-dpi renders. The bar says
	// (52)–(60), while the body says (52)–(67), "16 characters (Fixed)",
	// and the operating chapter says "up to 16 characters". The latter two
	// independent anchors agree, so 16 wins; the conflicting 9-byte bar
	// reading remains asserted above and is not erased.
	for _, fileAndNeedle := range []struct{ file, needle string }{
		{"IC-7100-field-ledger.md", "STOP findings"},
		{"IC-7100-field-ledger.md", "The bar and the body text disagree about the last group's index range"},
		{"IC-7100-geometry-witness.md", "STOP 16"},
		{"IC-7100-transcription-b.md", "16 characters (Fixed)"},
	} {
		contents, err := os.ReadFile(filepath.Join("testdata", fileAndNeedle.file))
		if err != nil {
			t.Fatalf("read %s: %v", fileAndNeedle.file, err)
		}
		if !strings.Contains(string(contents), fileAndNeedle.needle) {
			t.Errorf("%s lost arbitration anchor %q", fileAndNeedle.file, fileAndNeedle.needle)
		}
	}
	matrix, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "superpowers", "icom-matrices", "ic7100-capability-matrix.md"))
	if err != nil {
		t.Fatalf("read matrix arbitration: %v", err)
	}
	if !strings.Contains(string(matrix), "#### 3.15.1 The name field") || !strings.Contains(string(matrix), "wire bytes 99–114") {
		t.Error("matrix §3.15.1 no longer carries the 16-byte name arbitration")
	}

	// PDF arbitration ruling, page 375: the clearing block's "channel 0
	// to 99" is scoped only to an unshipped clear form and omits bank byte
	// 1; the field legend defines ordinary memories as 0001–0099. With no
	// erase builder and ic7100-special-bank-byte still CANNOT ESTABLISH,
	// the declared readable rectangle remains 01–05 × 0001–0099.
	if !strings.Contains(string(matrix), "#### 3.15.3 The clearing block's \"channel 0 to 99\"") {
		t.Error("matrix lost the channel-0 contradiction ruling")
	}

	// PDF arbitration ruling, page 375: the filled ❺–(51) box has no
	// internal cell rules. Its 47-byte width comes only from inclusive
	// index arithmetic and the NOTE saying it repeats ⑤–(51); no test may
	// fabricate per-cell witness claims for that box.
	filled := witness["D1/filled:5-51"]
	if got := filled["first_nibble"] + "/" + filled["last_nibble"]; got != "UNREADABLE/UNREADABLE" {
		t.Errorf("filled duplicate claims fabricated nibble geometry %q", got)
	}
	if !strings.Contains(filled["visual_anchor"]+" "+filled["notes"], "no internal cell divider") {
		t.Error("filled duplicate lost the witness that no internal cell rules exist")
	}
	if got := semantic["D1/filled:5-51"]["width_bytes"]; got != "47" {
		t.Errorf("B duplicate width = %q, want 47 from 51-5+1", got)
	}

	layout, ok := Profile().LayoutFor(RecordLength)
	if !ok {
		t.Fatalf("profile has no %d-byte layout", RecordLength)
	}
	type spanClaim struct {
		id       civ.FieldID
		offset   int
		length   int
		nibble   civ.NibbleSel
		encoding civ.EncodingKind
		order    civ.ByteOrder
		scale    uint64
	}
	claim := func(id civ.FieldID, offset, length int, nibble civ.NibbleSel, encoding civ.EncodingKind, order civ.ByteOrder, scale uint64) spanClaim {
		return spanClaim{id, offset, length, nibble, encoding, order, scale}
	}
	expectedSpans := map[spanClaim]bool{
		claim(civ.FieldSelect, 0, 1, civ.NibbleLow, civ.EncodingEnum, 0, 0):                    true,
		claim(civ.FieldRXFrequency, 1, 5, 0, civ.EncodingBCDNumber, civ.OrderLittleEndian, 1):  true,
		claim(civ.FieldMode, 6, 1, 0, civ.EncodingEnum, 0, 0):                                  true,
		claim(civ.FieldFilter, 7, 1, 0, civ.EncodingEnum, 0, 0):                                true,
		claim(civ.FieldDataMode, 8, 1, 0, civ.EncodingEnum, 0, 0):                              true,
		claim(civ.FieldDuplex, 9, 1, civ.NibbleHigh, civ.EncodingEnum, 0, 0):                   true,
		claim(civ.FieldToneMode, 9, 1, civ.NibbleLow, civ.EncodingEnum, 0, 0):                  true,
		claim(civ.FieldToneTX, 11, 3, 0, civ.EncodingBCDNumber, civ.OrderBigEndian, 1):         true,
		claim(civ.FieldToneRX, 14, 3, 0, civ.EncodingBCDNumber, civ.OrderBigEndian, 1):         true,
		claim(civ.FieldDTCSPolarity, 17, 1, 0, civ.EncodingEnum, 0, 0):                         true,
		claim(civ.FieldDTCSCode, 18, 2, 0, civ.EncodingBCDNumber, civ.OrderBigEndian, 1):       true,
		claim(civ.FieldOffset, 21, 3, 0, civ.EncodingBCDNumber, civ.OrderLittleEndian, 100):    true,
		claim(civ.FieldTXFrequency, 48, 5, 0, civ.EncodingBCDNumber, civ.OrderLittleEndian, 1): true,
		claim(civ.FieldMode, 53, 1, 0, civ.EncodingEnum, 0, 0):                                 true,
		claim(civ.FieldFilter, 54, 1, 0, civ.EncodingEnum, 0, 0):                               true,
		claim(civ.FieldDataMode, 55, 1, 0, civ.EncodingEnum, 0, 0):                             true,
		claim(civ.FieldDuplex, 56, 1, civ.NibbleHigh, civ.EncodingEnum, 0, 0):                  true,
		claim(civ.FieldToneMode, 56, 1, civ.NibbleLow, civ.EncodingEnum, 0, 0):                 true,
		claim(civ.FieldToneTX, 58, 3, 0, civ.EncodingBCDNumber, civ.OrderBigEndian, 1):         true,
		claim(civ.FieldToneRX, 61, 3, 0, civ.EncodingBCDNumber, civ.OrderBigEndian, 1):         true,
		claim(civ.FieldDTCSPolarity, 64, 1, 0, civ.EncodingEnum, 0, 0):                         true,
		claim(civ.FieldDTCSCode, 65, 2, 0, civ.EncodingBCDNumber, civ.OrderBigEndian, 1):       true,
		claim(civ.FieldOffset, 68, 3, 0, civ.EncodingBCDNumber, civ.OrderLittleEndian, 100):    true,
		claim(civ.FieldName, 95, 16, 0, civ.EncodingName, 0, 0):                                true,
	}
	for _, span := range layout.Fields {
		got := claim(span.Field, span.Offset, span.Length, span.Nibble, span.Encoding, span.Order, span.Scale)
		if !expectedSpans[got] {
			t.Errorf("profile span is not accounted for by L/W/B: %+v", got)
			continue
		}
		delete(expectedSpans, got)
	}
	for missing := range expectedSpans {
		t.Errorf("L/W/B span is absent from profile: %+v", missing)
	}

	for _, want := range []struct {
		id      civ.FieldID
		primary int
		copy    int
	}{
		{civ.FieldMode, 6, 53},
		{civ.FieldFilter, 7, 54},
		{civ.FieldDataMode, 8, 55},
		{civ.FieldDuplex, 9, 56},
		{civ.FieldToneMode, 9, 56},
		{civ.FieldToneTX, 11, 58},
		{civ.FieldToneRX, 14, 61},
		{civ.FieldDTCSPolarity, 17, 64},
		{civ.FieldDTCSCode, 18, 65},
		{civ.FieldOffset, 21, 68},
	} {
		for _, offset := range []int{want.primary, want.copy} {
			found := false
			for _, span := range layout.Fields {
				if span.Field == want.id && span.Offset == offset {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("profile missing %s span at record offset %d (ic7100-tx-block-mandatory)", want.id, offset)
			}
		}
	}

	for _, width := range []struct {
		key  string
		want int
	}{
		{"D1/1", 1}, {"D1/2,3", 2}, {"D1/4", 1}, {"D1/5-9", 5},
		{"D1/10,11", 2}, {"D1/12", 1}, {"D1/13", 1}, {"D1/14", 1},
		{"D1/15-17", 3}, {"D1/18-20", 3}, {"D1/21-23", 3}, {"D1/24", 1},
		{"D1/25-27", 3}, {"D1/28-35", 8}, {"D1/36-43", 8}, {"D1/44-51", 8},
		{"D1/filled:5-51", 47}, {"D1/52-67", 16},
	} {
		row, ok := semantic[width.key]
		if !ok {
			t.Errorf("B evidence missing settled span %s", width.key)
			continue
		}
		got, err := strconv.Atoi(row["width_bytes"])
		if err != nil || got != width.want {
			t.Errorf("B %s width = %q, want %d", width.key, row["width_bytes"], width.want)
		}
	}

}
