// SPDX-License-Identifier: GPL-3.0-or-later

package icr8600_test

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/icr8600"
)

func TestGeometryWitnessPinsFixedHeadSpans(t *testing.T) {
	w := indexEvidence(t, loadEvidenceCSV(t, "IC-R8600-geometry-witness.csv"), "diagram_id", "field_index")
	for _, tc := range []struct {
		field       string
		first, last int
	}{
		{"①, ②", 1, 2}, {"③, ④", 3, 4}, {"⑤", 5, 5},
		{"⑥ ~ ⑩", 6, 10}, {"⑪, ⑫", 11, 12}, {"⑬", 13, 13},
		{"⑭ ~ ⑰", 14, 17}, {"⑱, ⑲", 18, 19}, {"⑳, ㉑", 20, 21},
		{"㉒", 22, 22}, {"㉓", 23, 23}, {"㉔", 24, 24},
		{"㉕", 25, 25}, {"㉖ ~ ㊶", 26, 41},
	} {
		row := w[evidenceKey("D1", tc.field)]
		if got := mustEvidenceInt(t, row, "first_byte"); got != tc.first {
			t.Errorf("D1 %s first byte = %d, want %d", tc.field, got, tc.first)
		}
		if got := mustEvidenceInt(t, row, "last_byte"); got != tc.last {
			t.Errorf("D1 %s last byte = %d, want %d", tc.field, got, tc.last)
		}
	}

	// W numbers the four-byte address as ①–④. Profile offsets begin after
	// that address, so each mapped record field starts at W byte offset+5.
	layout := icr8600.Profile().Layouts()[0]
	wByField := map[civ.FieldID][2]int{
		civ.FieldSelect: {5, 5}, civ.FieldRXFrequency: {6, 10},
		civ.FieldMode: {11, 11}, civ.FieldFilter: {12, 12},
		civ.FieldDuplex: {13, 13}, civ.FieldOffset: {14, 17},
		civ.FieldTuningStepEnabled: {18, 18}, civ.FieldTuningStep: {19, 19},
		civ.FieldProgramTuningStep: {20, 21}, civ.FieldAttenuator: {22, 22},
		civ.FieldPreamp: {23, 23}, civ.FieldAntenna: {24, 24},
		civ.FieldIPPlus: {25, 25}, civ.FieldName: {26, 41},
	}
	for field, want := range wByField {
		span := fieldSpan(t, layout, field)
		if gotFirst, gotLast := span.Offset+5, span.Offset+span.Length+4; gotFirst != want[0] || gotLast != want[1] {
			t.Errorf("%s occupies W bytes %d-%d, want %d-%d", field, gotFirst, gotLast, want[0], want[1])
		}
	}
}

func TestGeometryWitnessPinsEveryTailOffsetAndLength(t *testing.T) {
	wRows := loadEvidenceCSV(t, "IC-R8600-geometry-witness.csv")
	byDiagram := make(map[string][]evidenceRow)
	for _, row := range wRows {
		byDiagram[row["diagram_id"]] = append(byDiagram[row["diagram_id"]], row)
	}
	want := []struct {
		diagram, class         string
		lastByte, recordLength int
	}{
		{"D2", "FM", 48, 44}, {"D3", "P25", 45, 41},
		{"D4", "D-STAR", 43, 39}, {"D5", "dPMR", 49, 45},
		{"D6", "NXDN", 47, 43}, {"D7", "DCR", 48, 44},
	}
	layouts := map[string]civ.RecordLayout{}
	for _, layout := range icr8600.Profile().Layouts() {
		layouts[layout.ModeClass] = layout
	}
	for _, tc := range want {
		rows := byDiagram[tc.diagram]
		if len(rows) == 0 {
			t.Fatalf("W has no rows for %s", tc.diagram)
		}
		first, last := 1<<30, 0
		for _, row := range rows {
			rowFirst := mustEvidenceInt(t, row, "first_byte")
			rowLast := mustEvidenceInt(t, row, "last_byte")
			if rowFirst < first {
				first = rowFirst
			}
			if rowLast > last {
				last = rowLast
			}
		}
		if first != 42 || first-5 != 37 {
			t.Errorf("%s tail starts at W byte %d / record offset %d, want 42 / 37", tc.diagram, first, first-5)
		}
		if last != tc.lastByte || last-4 != tc.recordLength {
			t.Errorf("%s tail ends at W byte %d, giving record length %d; want %d/%d", tc.diagram, last, last-4, tc.lastByte, tc.recordLength)
		}
		if layout := layouts[tc.class]; layout.Length != tc.recordLength {
			t.Errorf("profile %s length = %d, W gives %d", tc.class, layout.Length, tc.recordLength)
		}
	}
}

func TestGeometryWitnessFMAndDCRShareLengthButNotContent(t *testing.T) {
	wRows := loadEvidenceCSV(t, "IC-R8600-geometry-witness.csv")
	byDiagram := make(map[string][]evidenceRow)
	for _, row := range wRows {
		if row["diagram_id"] == "D2" || row["diagram_id"] == "D7" {
			byDiagram[row["diagram_id"]] = append(byDiagram[row["diagram_id"]], row)
		}
	}
	for _, rows := range byDiagram {
		sort.Slice(rows, func(i, j int) bool {
			return mustEvidenceInt(t, rows[i], "first_byte") < mustEvidenceInt(t, rows[j], "first_byte")
		})
	}
	if got := evidenceSignature(byDiagram["D2"]); got == evidenceSignature(byDiagram["D7"]) {
		t.Errorf("FM and DCR W field geometry unexpectedly identical: %s", got)
	}

	b := indexEvidence(t, loadEvidenceCSV(t, "IC-R8600-transcription-b.csv"), "diagram_id", "field_index")
	if b[evidenceKey("D2", "㊷")]["label_verbatim"] == b[evidenceKey("D7", "㊷")]["label_verbatim"] {
		t.Error("FM and DCR ㊷ content unexpectedly agrees")
	}
	fmRecord := builtRecord(t, completeRecord("FM"))
	dcrRecord := builtRecord(t, commonRecord("DCR"))
	if len(fmRecord) != len(dcrRecord) || len(fmRecord) != 44 {
		t.Fatalf("FM/DCR lengths = %d/%d, want 44/44", len(fmRecord), len(dcrRecord))
	}
	fmLayout, fmErr := icr8600.Profile().LayoutForRecord(fmRecord)
	dcrLayout, dcrErr := icr8600.Profile().LayoutForRecord(dcrRecord)
	if fmErr != nil || dcrErr != nil || reflect.DeepEqual(fmLayout, dcrLayout) {
		t.Errorf("mode-selected FM/DCR layouts = (%+v, %v) / (%+v, %v), want distinct", fmLayout, fmErr, dcrLayout, dcrErr)
	}
}

func evidenceSignature(rows []evidenceRow) string {
	parts := make([]string, len(rows))
	for i, row := range rows {
		parts[i] = row["field_index"] + ":" + row["first_byte"] + "-" + row["last_byte"]
	}
	return strings.Join(parts, "|")
}
