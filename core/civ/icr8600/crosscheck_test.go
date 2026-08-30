// SPDX-License-Identifier: GPL-3.0-or-later

package icr8600_test

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/icr8600"
)

type evidenceRow map[string]string

type matrixJoin struct {
	class                     string
	bDiagram, field, lDiagram string
	width                     int
}

func TestCrosscheckMatrixBAndLedgerJoinByDiagramAndField(t *testing.T) {
	pinMatrixA(t)
	bRows := loadEvidenceCSV(t, "IC-R8600-transcription-b.csv")
	lRows := loadEvidenceCSV(t, "IC-R8600-field-ledger.csv")
	bIndex := indexEvidence(t, bRows, "diagram_id", "field_index")
	lIndex := indexEvidence(t, lRows, "diagram_id", "field_index")

	seenA := make(map[string]bool)
	seenB := make(map[string]bool)
	classes := map[string]int{"NONE": 0}
	for _, join := range matrixJoins() {
		// A's canonical key is B's mode-local diagram plus the exact printed
		// field index. L uses more diagram IDs because it records both byte
		// bands and detail boxes, so A names the one L detail diagram which
		// may join. No label-only or field-index-only fallback is permitted.
		aKey := evidenceKey(join.bDiagram, join.field)
		if seenA[aKey] {
			t.Fatalf("matrix A duplicates canonical key %s", aKey)
		}
		seenA[aKey] = true

		b, ok := bIndex[aKey]
		if !ok {
			t.Errorf("matrix A key %s has no exact B join", aKey)
			continue
		}
		seenB[aKey] = true
		lKey := evidenceKey(join.lDiagram, join.field)
		l, ok := lIndex[lKey]
		if !ok {
			t.Errorf("matrix A key %s names L source %s, which is absent", aKey, lKey)
			continue
		}
		if b["label_verbatim"] != l["label_verbatim"] {
			t.Errorf("%s label: B=%q L=%q", aKey, b["label_verbatim"], l["label_verbatim"])
		}
		width, err := strconv.Atoi(b["width_bytes"])
		if err != nil || width != join.width {
			t.Errorf("%s B width = %q, want matrix A width %d", aKey, b["width_bytes"], join.width)
		}
		classes[join.class] += width
	}

	for key, row := range bIndex {
		diagram := row["diagram_id"]
		if diagram >= "D1" && diagram <= "D7" && !seenB[key] {
			t.Errorf("B row %s is not joined by matrix A", key)
		}
	}
	wantClassWidths := map[string]int{
		"NONE": 0, "FM": 7, "P25": 4, "D-STAR": 2,
		"dPMR": 8, "NXDN": 6, "DCR": 7,
	}
	for class, want := range wantClassWidths {
		if got, ok := classes[class]; !ok || got != want {
			t.Errorf("joined tail width for %s = %d (present %v), want %d", class, got, ok, want)
		}
	}
	if got := classes["HEAD"]; got != 41 {
		t.Errorf("joined common data-area width = %d, want 41 (four address plus 37 record bytes)", got)
	}
	if got, want := len(icr8600.Profile().Layouts()), len(wantClassWidths); got != want {
		t.Errorf("profile has %d tail classes, matrix crosscheck has %d", got, want)
	}
}

// pinMatrixA makes matrixJoins an extraction of one exact authoritative A,
// rather than an untethered local restatement. The matrices are outside the
// quarantined testdata freeze, so their own digests are pinned here: a matrix
// correction must fail this crosscheck and be deliberately re-arbitrated.
//
// The matrices live under docs/superpowers/, which is gitignored: a fresh
// clone and CI do not have them (found the hard way — the v1.2.1 CI run,
// 30/08/2026). When the directory is absent the digest pin is reported and
// skipped, and every join below still runs against the frozen testdata,
// which IS in the repository and is the evidence the profile was derived
// from; the pin bites on the maintainer's checkout, where the matrices are.
func pinMatrixA(t *testing.T) {
	t.Helper()
	if !matrixAuthorityPresent() {
		t.Logf("matrix A authority absent (docs/superpowers is gitignored; not in this checkout) — digest pin skipped, frozen-testdata joins still enforced")
		return
	}
	for relative, want := range map[string]string{
		"icr8600-capability-matrix.md":        "3948edb2705caf393c6144130e6a24a546ea4a8acdb67cd11a7b605607a8db7e",
		"icr8600-capability-matrix-report.md": "8792f8f1bab14d0e66fd2a27c29c82fee1f606fbbd1aefc2de2429f9d53dd93d",
		"icr8600-capability-matrix-review.md": "8c9877ea1745d29badca780e4a15d8b739382fbe0fc8fefb8acf86137e0c4117",
	} {
		path := filepath.Join("..", "..", "..", "docs", "superpowers", "icom-matrices", relative)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read matrix A authority %s: %v", path, err)
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != want {
			t.Fatalf("matrix A authority %s changed: pinned %s, present %s; re-arbitrate matrixJoins against the revised matrix before updating this digest", relative, want, got)
		}
	}
}

// readMatrixA returns one matrix authority's text, having first re-pinned
// every authority's digest. A ruling that quotes the matrix must quote the
// arbitrated matrix, not whatever is on disk.
func readMatrixA(t *testing.T, relative string) string {
	t.Helper()
	if !matrixAuthorityPresent() {
		t.Skip("matrix A authority absent (docs/superpowers is gitignored; not in this checkout) — this quotation check runs only where the matrices are")
	}
	pinMatrixA(t)
	path := filepath.Join("..", "..", "..", "docs", "superpowers", "icom-matrices", relative)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read matrix A authority %s: %v", path, err)
	}
	return string(data)
}

// matrixAuthorityPresent reports whether the gitignored matrix directory
// exists in this checkout.
func matrixAuthorityPresent() bool {
	_, err := os.Stat(filepath.Join("..", "..", "..", "docs", "superpowers", "icom-matrices", "icr8600-capability-matrix.md"))
	return err == nil
}

func TestCrosscheckBReferencedFoliosRemainUnknownNotZero(t *testing.T) {
	b := indexEvidence(t, loadEvidenceCSV(t, "IC-R8600-transcription-b.csv"), "diagram_id", "field_index")
	for _, key := range []string{
		evidenceKey("D1", "⑥ ~ ⑩"),
		evidenceKey("D1", "⑪, ⑫"),
		evidenceKey("D1", "⑭ ~ ⑰"),
		evidenceKey("D2", "㊸ ~ ㊺"),
		evidenceKey("D2", "㊻ ~ ㊽"),
	} {
		row := b[key]
		if row == nil {
			t.Fatalf("missing B row %s", key)
		}
		if row["encoding"] != "unstated" || row["values_verbatim"] != "" {
			t.Errorf("%s = encoding %q values %q, want an unstated blank rather than fabricated zero", key, row["encoding"], row["values_verbatim"])
		}
		if !strings.Contains(row["notes"], "was not read") {
			t.Errorf("%s does not retain B's referenced-folio boundary: %q", key, row["notes"])
		}
	}
	// Folio 3's command-10 table was outside B's scope. B records only the
	// local invalid-00 sentence, which must not be expanded into a code table
	// or interpreted as a numeric zero.
	ts := b[evidenceKey("D1", "⑲")]
	if ts["encoding"] != "unstated" || ts["values_verbatim"] != "*00=10 Hz (TS OFF) is invalid." || strings.Contains(ts["values_verbatim"], "01=") {
		t.Errorf("B folio-3 boundary was not preserved: %+v", ts)
	}
}

func TestCrosscheckArbitrations(t *testing.T) {
	b := indexEvidence(t, loadEvidenceCSV(t, "IC-R8600-transcription-b.csv"), "diagram_id", "field_index")
	l := indexEvidence(t, loadEvidenceCSV(t, "IC-R8600-field-ledger.csv"), "diagram_id", "field_index")
	w := indexEvidence(t, loadEvidenceCSV(t, "IC-R8600-geometry-witness.csv"), "diagram_id", "field_index")

	t.Run("W_STOP_1_PDF_mode_headings_win_over_global_indexing", func(t *testing.T) {
		// RULING — PDF pp.13–15 restart ㊷ beneath each distinct "For
		// receiving ..." heading. The heading-scoped B/L diagrams and the
		// matrix's mode discriminator win; W's possible global-index reading
		// loses. This is why ㊷ is tail-local, never a globally unique field.
		labels := map[string]string{}
		for _, diagram := range []string{"D2", "D3", "D4", "D5", "D6", "D7"} {
			row := w[evidenceKey(diagram, "㊷")]
			if row["first_byte"] != "42" || !strings.Contains(row["notes"], "STOP 1") {
				t.Errorf("%s ㊷ does not preserve W STOP 1: %+v", diagram, row)
			}
			labels[row["block_label_verbatim"]] = diagram
		}
		if len(labels) != 6 {
			t.Errorf("mode-local ㊷ headings = %v, want six distinct headings", labels)
		}
	})

	t.Run("W_STOP_2_PDF_ranges_win_over_drawn_ellipsis_cells", func(t *testing.T) {
		// RULING — PDF p.12's printed index ranges and the matching detail
		// descriptions win; W's count of visible solid cells loses because
		// each dotted cell is an ellipsis. The ranges therefore span 5, 4
		// and 16 bytes, not the two solid endpoint cells which are drawn.
		for _, tc := range []struct {
			field string
			width int
		}{{"⑥ ~ ⑩", 5}, {"⑭ ~ ⑰", 4}, {"㉖ ~ ㊶", 16}} {
			row := w[evidenceKey("D1", tc.field)]
			first := mustEvidenceInt(t, row, "first_byte")
			last := mustEvidenceInt(t, row, "last_byte")
			if last-first+1 != tc.width || !strings.Contains(row["notes"], "STOP 2") {
				t.Errorf("D1 %s geometry = %+v, want %d-byte range with STOP 2", tc.field, row, tc.width)
			}
		}
	})

	t.Run("B_programme_step_leaders_win_over_printed_list_order", func(t *testing.T) {
		// RULING — PDF p.12's leader endpoints, independently followed by B
		// and L, win over the reverse top-to-bottom label list. The wire
		// order is deliberately non-monotonic: 1 kHz, 100 Hz, 100 kHz,
		// 10 kHz. TestRecordCommonHeadEncodesAndDecodesEveryMappedField pins
		// the resulting 987.6 kHz encoding as 76 98.
		if got := b[evidenceKey("D1", "⑳")]["values_verbatim"]; got != "1 kHz digit: 0 ~ 9 | 100 Hz digit: 0 ~ 9" {
			t.Errorf("B ⑳ values = %q", got)
		}
		if got := b[evidenceKey("D1", "㉑")]["values_verbatim"]; got != "100 kHz digit: 0 ~ 9 | 10 kHz digit: 0 ~ 9" {
			t.Errorf("B ㉑ values = %q", got)
		}
	})

	t.Run("PDF_D_STAR_codes_win_over_sibling_analogy", func(t *testing.T) {
		// RULING — PDF p.13 explicitly prints D-STAR 0=OFF and 2=CSQL.
		// B's literal D-STAR row wins; the tempting analogy with sibling
		// tails' code 1 loses. TestTailTemplatesAndFMToneFieldsEncodeEveryDeclaredClass
		// pins the assumed icr8600-tail-templates writable template to code 2.
		values := b[evidenceKey("D4", "㊷")]["values_verbatim"]
		if values != "0 (Fixed) | 0=OFF | 2=CSQL" || strings.Contains(values, "1=") {
			t.Errorf("D-STAR D.SQL values = %q, want skipped code 1", values)
		}
	})

	t.Run("spec_full_layout_wins_over_documented_short_set", func(t *testing.T) {
		// RULING — PDF p.12 documents that entering ㊷ or later may be
		// omitted, but does not state the supplied defaults. The spec's
		// full-layout rule wins for emitted records; the ambiguous defaults
		// recorded by icr8600-short-set lose. TestModeLayoutsSelectSevenTailsAndSixRecordLengths
		// pins every selected build length until Stage W lifts the register.
		for _, mode := range []string{"FM", "P25", "D-STAR", "dPMR", "NXDN-N", "DCR"} {
			record := builtRecord(t, completeRecord(mode))
			if len(record) != icr8600.Profile().BuildRecordLengthFor(mode) || len(record) == 37 {
				t.Errorf("BuildMemorySet(%s) emitted short record length %d", mode, len(record))
			}
		}
	})

	t.Run("L_STOP_1_PDF_glyphs_win_over_ASCII_normalisation", func(t *testing.T) {
		// RULING — the 400/600-dpi PDF p.15 render shows U+301C wave
		// dashes and U+3001 ideographic commas in D35. L's literal glyphs
		// win; normalising them to ASCII "~" and "," loses.
		for _, field := range []string{"①〜⑤", "⑥〜⑩", "⑪、⑫", "⑭〜⑰", "⑱、⑲", "⑳、㉑"} {
			row := l[evidenceKey("D35", field)]
			if row == nil || !strings.Contains(row["notes"], "STOP 1") {
				t.Errorf("L D35 literal field %q missing STOP 1", field)
			}
		}
	})

	t.Run("L_STOP_2_PDF_mode_blocks_win_over_index_only_merge", func(t *testing.T) {
		// RULING — PDF pp.13–15 give each repeated ㊷ its own mode heading.
		// L's diagram plus field composite key wins; merging rows on ㊷
		// alone loses. The strict join above deliberately requires both.
		count := 0
		labels := map[string]bool{}
		for key, row := range l {
			if strings.HasSuffix(key, "\x00㊷") && strings.Contains(row["notes"], "STOP 2") {
				count++
				labels[row["label_verbatim"]] = true
			}
		}
		if count < 6 || len(labels) < 2 {
			t.Errorf("L tail-local ㊷ rows = %d across labels %v", count, labels)
		}
	})

	t.Run("L_STOP_3_record_D1_SELECT_wins_over_other_formats", func(t *testing.T) {
		// RULING — PDF p.12 D1, PDF p.15 D34 and PDF p.15 D35 are separate
		// formats which reuse indices. Matrix D1 plus ⑤ wins for the memory
		// record: its low nibble is SELECT membership, not D34's clear FF or
		// D35's lower-edge byte. TestRecordCommonHeadEncodesAndDecodesEveryMappedField
		// pins that mapping.
		if got := l[evidenceKey("D1", "⑤")]["label_verbatim"]; got != "Skip/Select Memory scan setting" {
			t.Errorf("D1 ⑤ label = %q", got)
		}
		if got := l[evidenceKey("D34", "⑤")]["label_verbatim"]; got != `"FF"` {
			t.Errorf("D34 ⑤ label = %q", got)
		}
		if got := l[evidenceKey("D35", "①〜⑤")]["label_verbatim"]; got != "Lower scan edge" {
			t.Errorf("D35 ①〜⑤ label = %q", got)
		}
		span := fieldSpan(t, icr8600.Profile().Layouts()[0], civ.FieldSelect)
		if span.Offset != 0 || span.Enum[1] != "SEL1" {
			t.Errorf("record SELECT span = %+v", span)
		}
	})

	t.Run("plan_global_SELECT_constraint_wins_over_matrix_scan_skip_grading", func(t *testing.T) {
		// RULING — this is a DEPARTURE FROM THE MATRIX and is recorded
		// as one. Matrix section 2 row 9 grades neutral scan_skip onto
		// ⑤'s HIGH nibble and rules the ★ LOW nibble "the select-memory
		// marker ... which the neutral model has no field for". The
		// implementation inverts that: the LOW nibble is mapped as
		// civ.FieldSelect, and spec.FieldScanSkip is a written-down zero
		// in the driver's capabilities.
		//
		// WINNER: the plan's global constraint — never map SELECT as
		// scan_skip, because collapsing a ten-valued group into a
		// BoolField destroys eight of its states and answers a question
		// about scanning with an answer about membership.
		// LOSER: matrix section 2 row 9's grading of which nibble
		// carries the neutral field.
		//
		// The departure is safe rather than silent. The high nibble is
		// mapped NOWHERE, so the driver's E6 gate refuses any record
		// whose SKIP/PSKIP state is non-zero rather than rebuilding it
		// as SKIP OFF, and a create into an empty slot is refused rather
		// than inventing a SELECT value.
		//
		// L_STOP_3 above settles only WHICH FORMAT owns ⑤. This subtest
		// settles which NIBBLE of it the neutral model may claim.
		if got := b[evidenceKey("D1", "⑤")]["values_verbatim"]; got != "0=SKIP OFF | 1=SKIP | 2=PSKIP | 0 =OFF | 1 ~ 9=★1 ~ ★9" {
			t.Errorf("B ⑤ values = %q, want both nibbles' printed enums in drawn order", got)
		}
		if got := b[evidenceKey("D1", "⑤")]["notes"]; !strings.Contains(got, "LEFT nibble carries 0=SKIP OFF") || !strings.Contains(got, "RIGHT nibble carries 0 =OFF") {
			t.Errorf("B ⑤ notes lost the drawn left-to-right nibble order: %q", got)
		}
		if got := l[evidenceKey("D2", "⑤")]["notes"]; !strings.Contains(got, "0=SKIP OFF / 1=SKIP / 2=PSKIP | 0 =OFF / 1 ~ 9=★1 ~ ★9") {
			t.Errorf("L D2 ⑤ detail box lost the same order: %q", got)
		}
		// The loser, quoted from the digest-pinned matrix A itself so a
		// re-arbitration cannot leave this ruling behind.
		matrix := readMatrixA(t, "icr8600-capability-matrix.md")
		if !strings.Contains(matrix, "| 9 | `scan_skip` | Unverified | Unverified | ⑤ **high** nibble") {
			t.Error("matrix section 2 row 9 no longer grades scan_skip onto the high nibble; re-arbitrate this departure")
		}
		// The winner, in the profile: SELECT on the LOW nibble of byte 0
		// with all ten states, and NOTHING on the high nibble, in every
		// layout.
		for _, layout := range icr8600.Profile().Layouts() {
			span := fieldSpan(t, layout, civ.FieldSelect)
			if span.Offset != 0 || span.Nibble != civ.NibbleLow || len(span.Enum) != 10 {
				t.Errorf("%s SELECT span = %+v, want byte 0's low nibble carrying all ten states", layout.ModeClass, span)
			}
			for _, other := range layout.Fields {
				if other.Offset == 0 && other.Nibble != civ.NibbleLow {
					t.Errorf("%s maps %v onto byte 0 as %v; the SKIP/PSKIP nibble must stay unmapped so E6 refuses it", layout.ModeClass, other.Field, other.Nibble)
				}
			}
		}
	})

	t.Run("L_STOP_4_semantic_NAC_position_wins_over_typo", func(t *testing.T) {
		// RULING — PDF p.13 visibly says "100th posiiton". L preserves the
		// typo as evidence, while B's leader tracing and the matrix's NAC
		// hundreds-position meaning win semantically; treating "posiiton"
		// as a distinct field loses.
		row := l[evidenceKey("D17", "㊸")]
		if !strings.Contains(row["notes"], "100th posiiton") || !strings.Contains(row["notes"], "STOP 4") {
			t.Errorf("L typo witness = %+v", row)
		}
		if got := b[evidenceKey("D3", "㊸")]["values_verbatim"]; !strings.Contains(got, "100th posiiton: 0 ~ F") {
			t.Errorf("B typo witness = %q", got)
		}
	})

	t.Run("L_STOP_5_complete_record_geometry_wins_over_incomplete_clear_range", func(t *testing.T) {
		// RULING — PDF p.15's clear list literally stops at "⑥ ~" and has
		// no terminal index. It proves only the clear grammar. The complete
		// PDF p.12 record band plus mode-tail diagrams win for record length;
		// extrapolating D34's incomplete range loses.
		row := l[evidenceKey("D34", "⑥ ~")]
		if row == nil || !strings.Contains(row["notes"], "STOP 5") {
			t.Errorf("L incomplete clear-list witness = %+v", row)
		}
		if got := icr8600.Profile().RecordLengths(); len(got) != 6 || got[0] != 37 || got[5] != 45 {
			t.Errorf("complete record geometry = %v", got)
		}
	})
}

func matrixJoins() []matrixJoin {
	return []matrixJoin{
		{"HEAD", "D1", "①, ②", "D1", 2}, {"HEAD", "D1", "③, ④", "D1", 2},
		{"HEAD", "D1", "⑤", "D1", 1}, {"HEAD", "D1", "⑥ ~ ⑩", "D1", 5},
		{"HEAD", "D1", "⑪, ⑫", "D1", 2}, {"HEAD", "D1", "⑬", "D1", 1},
		{"HEAD", "D1", "⑭ ~ ⑰", "D1", 4}, {"HEAD", "D1", "⑱", "D4", 1},
		{"HEAD", "D1", "⑲", "D4", 1}, {"HEAD", "D1", "⑳", "D5", 1},
		{"HEAD", "D1", "㉑", "D5", 1}, {"HEAD", "D1", "㉒", "D6", 1},
		{"HEAD", "D1", "㉓", "D7", 1}, {"HEAD", "D1", "㉔", "D8", 1},
		{"HEAD", "D1", "㉕", "D9", 1}, {"HEAD", "D1", "㉖ ~ ㊶", "D1", 16},
		{"FM", "D2", "㊷", "D10", 1}, {"FM", "D2", "㊸ ~ ㊺", "D10", 3},
		{"FM", "D2", "㊻ ~ ㊽", "D10", 3},
		{"P25", "D3", "㊷", "D16", 1}, {"P25", "D3", "㊸", "D17", 1},
		{"P25", "D3", "㊹", "D17", 1}, {"P25", "D3", "㊺", "D17", 1},
		{"D-STAR", "D4", "㊷", "D13", 1}, {"D-STAR", "D4", "㊸", "D14", 1},
		{"dPMR", "D5", "㊷", "D19", 1}, {"dPMR", "D5", "㊸", "D20", 1},
		{"dPMR", "D5", "㊹", "D20", 1}, {"dPMR", "D5", "㊺", "D21", 1},
		{"dPMR", "D5", "㊻", "D22", 1}, {"dPMR", "D5", "㊼", "D23", 1},
		{"dPMR", "D5", "㊽", "D23", 1}, {"dPMR", "D5", "㊾", "D23", 1},
		{"NXDN", "D6", "㊷", "D25", 1}, {"NXDN", "D6", "㊸", "D26", 1},
		{"NXDN", "D6", "㊹", "D27", 1}, {"NXDN", "D6", "㊺", "D28", 1},
		{"NXDN", "D6", "㊻", "D28", 1}, {"NXDN", "D6", "㊼", "D28", 1},
		{"DCR", "D7", "㊷", "D30", 1}, {"DCR", "D7", "㊸", "D31", 1},
		{"DCR", "D7", "㊹", "D31", 1}, {"DCR", "D7", "㊺", "D32", 1},
		{"DCR", "D7", "㊻", "D33", 1}, {"DCR", "D7", "㊼", "D33", 1},
		{"DCR", "D7", "㊽", "D33", 1},
	}
}

func loadEvidenceCSV(t *testing.T, name string) []evidenceRow {
	t.Helper()
	path := filepath.Join("testdata", name)
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if len(records) < 2 {
		t.Fatalf("%s has no evidence rows", path)
	}
	head := records[0]
	rows := make([]evidenceRow, 0, len(records)-1)
	for line, record := range records[1:] {
		if len(record) != len(head) {
			t.Fatalf("%s line %d has %d columns, want %d", path, line+2, len(record), len(head))
		}
		row := make(evidenceRow, len(head))
		for i, column := range head {
			row[column] = record[i]
		}
		rows = append(rows, row)
	}
	return rows
}

func indexEvidence(t *testing.T, rows []evidenceRow, columns ...string) map[string]evidenceRow {
	t.Helper()
	index := make(map[string]evidenceRow, len(rows))
	for _, row := range rows {
		parts := make([]string, len(columns))
		for i, column := range columns {
			parts[i] = row[column]
		}
		key := strings.Join(parts, "\x00")
		if _, exists := index[key]; exists {
			t.Fatalf("duplicate evidence key %q", key)
		}
		index[key] = row
	}
	return index
}

func evidenceKey(diagram, field string) string { return diagram + "\x00" + field }

func mustEvidenceInt(t *testing.T, row evidenceRow, column string) int {
	t.Helper()
	if row == nil {
		t.Fatalf("missing evidence row while reading %s", column)
	}
	value, err := strconv.Atoi(row[column])
	if err != nil {
		t.Fatalf("evidence %s=%q: %v", column, row[column], err)
	}
	return value
}
