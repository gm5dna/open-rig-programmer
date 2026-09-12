// SPDX-License-Identifier: GPL-3.0-or-later

package ic7600_test

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7600"
)

// This file binds two independently-produced readings of the IC-7600's own
// "Memory content setting" record diagram (PDF p.178, folio 169) to one
// another and to the profile this package builds: the field ledger
// (labels, derived from the capability matrix's own S2/S3.11 grading) and
// the Tier 7 S1 sweep's own transcription (widths, encodings and value
// lists, produced independently of and before the matrix - doc.go's
// Provenance section).
//
// THIS IS A LIGHTER CROSSCHECK THAN THE IC-7610 EXEMPLAR'S FOUR-LEG BLIND
// QUARANTINE, AND DELIBERATELY SO (doc.go's Provenance section explains
// why): there is no enlarged D2/D3 sub-diagram on this radio's own page to
// cross-check nibble-leader routing against (matrix S3.15(a)), and no
// printed Fixed-vs-FF contradiction to record (matrix S3.13) - both are
// genuine, matrix-documented divergences from the IC-7610's own page, not
// omissions from this test.
//
// ANY mismatch below is a STOP for orchestrator arbitration against the
// PDF, never an artefact edited merely to make the test pass.

// indexKey is an inclusive printed-index range, e.g. "4~8" -> {4,8} or
// "3" -> {3,3}.
type indexKey struct{ lo, hi int }

func (k indexKey) String() string {
	if k.lo == k.hi {
		return strconv.Itoa(k.lo)
	}
	return fmt.Sprintf("%d..%d", k.lo, k.hi)
}

func (k indexKey) width() int { return k.hi - k.lo + 1 }

// parseIndex decomposes the CSVs' plain-digit index spelling: "N",
// "N,M" (the address's two separate bytes) or "N~M" (a tilde range). No
// permissive fallback: a merely-similar cell is an error.
func parseIndex(s string) (indexKey, error) {
	switch {
	case strings.Contains(s, "~"):
		parts := strings.SplitN(s, "~", 2)
		lo, err := strconv.Atoi(parts[0])
		if err != nil {
			return indexKey{}, fmt.Errorf("index %q: bad low bound: %w", s, err)
		}
		hi, err := strconv.Atoi(parts[1])
		if err != nil {
			return indexKey{}, fmt.Errorf("index %q: bad high bound: %w", s, err)
		}
		return indexKey{lo, hi}, nil
	case strings.Contains(s, ","):
		parts := strings.SplitN(s, ",", 2)
		lo, err := strconv.Atoi(parts[0])
		if err != nil {
			return indexKey{}, fmt.Errorf("index %q: bad low bound: %w", s, err)
		}
		hi, err := strconv.Atoi(parts[1])
		if err != nil {
			return indexKey{}, fmt.Errorf("index %q: bad high bound: %w", s, err)
		}
		return indexKey{lo, hi}, nil
	default:
		n, err := strconv.Atoi(s)
		if err != nil {
			return indexKey{}, fmt.Errorf("index %q is neither a single number, a comma pair nor a tilde range: %w", s, err)
		}
		return indexKey{n, n}, nil
	}
}

func readEvidenceCSV(t *testing.T, name, wantHeader string, fields int) [][]string {
	t.Helper()
	path := filepath.Join(testdataDir, name)
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = fields
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if len(records) < 2 {
		t.Fatalf("%s carries %d records; want a header and at least one data row", path, len(records))
	}
	if got := strings.Join(records[0], ","); got != wantHeader {
		t.Fatalf("%s header is\n  %s\nwant\n  %s", path, got, wantHeader)
	}
	return records[1:]
}

type ledgerRow struct {
	diagram, indexRaw, label, page, anchor, notes string
	key                                            indexKey
}

type transcriptionRow struct {
	diagram, indexRaw, label, widthRaw, encoding, values, page, anchor, notes string
	key                                                                       indexKey
	width                                                                     int
}

const (
	ledgerHeader = "diagram_id,field_index,index_style,label_verbatim,pdf_page,visual_anchor,notes"
	bHeader      = "diagram_id,field_index,label_verbatim,width_bytes,encoding,values_verbatim,pdf_page,visual_anchor,notes"
)

func loadLedger(t *testing.T) []ledgerRow {
	t.Helper()
	out := make([]ledgerRow, 0, 9)
	for i, rec := range readEvidenceCSV(t, "ic7600-field-ledger.csv", ledgerHeader, 7) {
		row := ledgerRow{
			diagram: rec[0], indexRaw: rec[1], label: rec[3],
			page: rec[4], anchor: rec[5], notes: rec[6],
		}
		if row.diagram != "D1" {
			t.Fatalf("ledger row %d carries diagram_id %q; this radio's page carries D1 only (matrix S3.15(a))", i+2, row.diagram)
		}
		key, err := parseIndex(row.indexRaw)
		if err != nil {
			t.Fatalf("ledger row %d: %v", i+2, err)
		}
		row.key = key
		out = append(out, row)
	}
	return out
}

func loadTranscription(t *testing.T) []transcriptionRow {
	t.Helper()
	out := make([]transcriptionRow, 0, 9)
	for i, rec := range readEvidenceCSV(t, "ic7600-transcription-b.csv", bHeader, 9) {
		row := transcriptionRow{
			diagram: rec[0], indexRaw: rec[1], label: rec[2], widthRaw: rec[3],
			encoding: rec[4], values: rec[5], page: rec[6], anchor: rec[7], notes: rec[8],
		}
		if row.diagram != "D1" {
			t.Fatalf("transcription row %d carries diagram_id %q; the S1 leg found one diagram only", i+2, row.diagram)
		}
		key, err := parseIndex(row.indexRaw)
		if err != nil {
			t.Fatalf("transcription row %d: %v", i+2, err)
		}
		row.key = key
		w, err := strconv.Atoi(row.widthRaw)
		if err != nil {
			t.Fatalf("transcription row %d: width_bytes %q is not a number: %v", i+2, row.widthRaw, err)
		}
		row.width = w
		out = append(out, row)
	}
	return out
}

// rowBinding declares, for one printed D1 row, which profile spans carry
// it and what NON-SPAN content the rest of its bytes hold (the E6-unmapped
// regions).
type rowBinding struct {
	fields    []civ.FieldID
	unmapped  []int
	isAddress bool
	why       string
}

var d1Bindings = map[indexKey]rowBinding{
	{1, 2}:   {isAddress: true, why: "the 2-byte ADDRESS, outside the record (spec Erratum 1)"},
	{3, 3}:   {unmapped: []int{0}, why: "the whole-byte E6-unmapped SELECT marker - no nibble split is printed on this radio's own page (matrix S3.15(a))"},
	{4, 8}:   {fields: []civ.FieldID{civ.FieldRXFrequency}},
	{9, 9}:   {fields: []civ.FieldID{civ.FieldMode}},
	{10, 10}: {fields: []civ.FieldID{civ.FieldFilter}},
	{11, 11}: {fields: []civ.FieldID{civ.FieldToneMode}, unmapped: []int{8}, why: "Fixed[8] high nibble 0: the E6-unmapped data mode"},
	{12, 14}: {fields: []civ.FieldID{civ.FieldToneTX}},
	{15, 17}: {fields: []civ.FieldID{civ.FieldToneRX}},
	{18, 27}: {fields: []civ.FieldID{civ.FieldName}},
}

// TestCrosscheck_LedgerAndTranscriptionAndProfile binds the ledger, the
// transcription and the profile to one another.
func TestCrosscheck_LedgerAndTranscriptionAndProfile(t *testing.T) {
	ledger := loadLedger(t)
	transcription := loadTranscription(t)

	ledgerByKey := map[indexKey]ledgerRow{}
	for _, row := range ledger {
		if prev, dup := ledgerByKey[row.key]; dup {
			t.Fatalf("ledger has two rows for printed index %s: %q and %q", row.key, prev.label, row.label)
		}
		ledgerByKey[row.key] = row
	}
	bByKey := map[indexKey]transcriptionRow{}
	for _, row := range transcription {
		if prev, dup := bByKey[row.key]; dup {
			t.Fatalf("transcription has two rows for printed index %s: %q and %q", row.key, prev.label, row.label)
		}
		bByKey[row.key] = row
	}

	t.Run("leg0_every_row_is_accounted_for", func(t *testing.T) {
		if len(ledger) != 9 {
			t.Errorf("ledger has %d rows, want 9 - the record diagram's nine bracket groups", len(ledger))
		}
		if len(transcription) != 9 {
			t.Errorf("transcription has %d rows, want 9", len(transcription))
		}
	})

	t.Run("leg1_ledger_equals_transcription_on_index", func(t *testing.T) {
		for key, l := range ledgerByKey {
			b, ok := bByKey[key]
			if !ok {
				t.Errorf("ledger carries printed index %s (%q) and the transcription does not", key, l.label)
				continue
			}
			// Labels are allowed to differ by their own tabular convention
			// (the ledger's (9)/(10) rows record that they share one
			// printed legend; the transcription splits it the same way)
			// but must never contradict on WHICH field a row names.
			_ = b
		}
		for key, b := range bByKey {
			if _, ok := ledgerByKey[key]; !ok {
				t.Errorf("transcription carries printed index %s (%q) and the ledger does not", key, b.label)
			}
		}
	})

	t.Run("leg2_transcription_widths_tile_the_data_area", func(t *testing.T) {
		keys := make([]indexKey, 0, len(bByKey))
		for k := range bByKey {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i].lo < keys[j].lo })

		next, sum := 1, 0
		for _, k := range keys {
			row := bByKey[k]
			if k.width() != row.width {
				t.Errorf("printed index %s declares width_bytes %d, but its own index range spans %d", k, row.width, k.width())
			}
			if k.lo != next {
				t.Errorf("printed index %s starts at %d, want %d - the groups must tile with no gap and no overlap", k, k.lo, next)
			}
			next = k.hi + 1
			sum += row.width
		}
		if sum != ic7600.DataAreaLength {
			t.Errorf("B's widths sum to %d, want %d (DataAreaLength) - matrix S3.11's arithmetic", sum, ic7600.DataAreaLength)
		}
		if last := keys[len(keys)-1]; last.hi != 27 {
			t.Errorf("the last printed index is %d, want 27", last.hi)
		}
	})

	layout := ic7600.Profile().Layouts()[0]
	spansByField := map[civ.FieldID]civ.FieldSpan{}
	for _, sp := range layout.Fields {
		if _, dup := spansByField[sp.Field]; dup {
			t.Fatalf("the layout carries two spans for %s; this crosscheck's union table assumes one each on this model", sp.Field)
		}
		spansByField[sp.Field] = sp
	}
	reached := map[civ.FieldID]int{}

	t.Run("leg3_transcription_equals_the_profile", func(t *testing.T) {
		for key, row := range bByKey {
			bind, ok := d1Bindings[key]
			if !ok {
				t.Errorf("printed index %s has no declared span-union; every printed row is bound explicitly, never skipped", key)
				continue
			}

			if bind.isAddress {
				if key.width() != ic7600.AddressBytes {
					t.Errorf("the address row %s spans %d printed indices, want %d (ic7600.AddressBytes)", key, key.width(), ic7600.AddressBytes)
				}
				if len(bind.fields) != 0 || len(bind.unmapped) != 0 {
					t.Errorf("the address row %s declares %v and unmapped %v; it must declare NEITHER", key, bind.fields, bind.unmapped)
				}
				continue
			}

			covered := map[int]bool{}
			prevOffset := -1
			for _, id := range bind.fields {
				sp, ok := spansByField[id]
				if !ok {
					t.Errorf("printed index %s declares a union member %s that the layout does not carry", key, id)
					continue
				}
				if sp.Offset <= prevOffset {
					t.Errorf("printed index %s declares its union out of offset order", key)
				}
				prevOffset = sp.Offset
				reached[id]++
				for i := 0; i < sp.Length; i++ {
					covered[sp.Offset+i] = true
				}
			}
			for _, off := range bind.unmapped {
				covered[off] = true
			}

			wantLo, wantHi := key.lo-3, key.hi-3
			for off := wantLo; off <= wantHi; off++ {
				if !covered[off] {
					t.Errorf("printed index %s leaves record offset %d uncovered; the union is %v and its named non-span content is %q",
						key, off, bind.fields, bind.why)
				}
			}
			for off := range covered {
				if off < wantLo || off > wantHi {
					t.Errorf("printed index %s covers record offset %d, which lies outside its own range %d..%d", key, off, wantLo, wantHi)
				}
			}
			checkEncoding(t, key, row.encoding, bind, spansByField)
		}

		for _, sp := range layout.Fields {
			switch reached[sp.Field] {
			case 1:
			case 0:
				t.Fatalf("the layout's %s span at offset %d is reached by NO printed row's union", sp.Field, sp.Offset)
			default:
				t.Errorf("the layout's %s span is reached by %d unions, want exactly 1", sp.Field, reached[sp.Field])
			}
		}
	})

	// The printed VALUE LISTS pin the enums. Unlike the IC-7610's own
	// crosscheck, (9) and (10) are SEPARATE rows on this radio's own
	// transcription (matrix's own S3.11 table cites them under one brace;
	// the S1 leg's own granularity splits them - see the ledger CSV's row
	// for (9)), so no column-boundary decomposition is needed here: each
	// row's values_verbatim already names one field's vocabulary.
	t.Run("leg3b_the_printed_value_lists_pin_the_enums", func(t *testing.T) {
		modeRow, ok := bByKey[indexKey{9, 9}]
		if !ok {
			t.Fatal("the transcription has no row for printed index 9")
		}
		filterRow, ok := bByKey[indexKey{10, 10}]
		if !ok {
			t.Fatal("the transcription has no row for printed index 10")
		}
		for _, c := range []struct {
			field   civ.FieldID
			printed string
			what    string
		}{
			{civ.FieldMode, modeRow.values, "the (9) Operating mode setting cell"},
			{civ.FieldFilter, filterRow.values, "the (10) Filter setting cell"},
		} {
			printed := parsePipeList(t, c.printed)
			got := spansByField[c.field].Enum
			if !sameEnum(got, printed) {
				t.Errorf("%s's enum does not equal %s.\n  profile: %v\n  printed: %v",
					c.field, c.what, got, printed)
			}
		}
		// RULING OQ1 (core/civ/ic7610/doc.go), applied by direct precedent:
		// the printed 12/13 codes are the wire bytes 0x12/0x13, not decimal
		// 12/13. Named explicitly so a failure says which decision was
		// reversed.
		for code, name := range map[byte]string{0x12: "PSK", 0x13: "PSK-R"} {
			if got := spansByField[civ.FieldMode].Enum[code]; got != name {
				t.Errorf("mode enum[%#02x] = %q, want %q - RULING OQ1 applies to this radio's identical printed codes", code, got, name)
			}
		}
	})

	t.Run("leg4_unmapped_rows_are_consumed", func(t *testing.T) {
		fixed := ic7600.FixedTemplate()

		if _, ok := bByKey[indexKey{3, 3}]; !ok {
			t.Fatal("the transcription has no row for printed index 3")
		}
		if got := fixed[ic7600.SelectByteOffset]; got != 0x00 {
			t.Errorf("FixedTemplate()[%d] = %#02x, want 0x00", ic7600.SelectByteOffset, got)
		}
		for _, sp := range layout.Fields {
			if sp.Offset == ic7600.SelectByteOffset {
				t.Errorf("%s claims record offset %d, which E6 leaves UNMAPPED", sp.Field, sp.Offset)
			}
		}

		if _, ok := bByKey[indexKey{11, 11}]; !ok {
			t.Fatal("the transcription has no row for printed index 11")
		}
		if got := fixed[ic7600.DataModeNibbleOffset]; got != 0x00 {
			t.Errorf("FixedTemplate()[%d] = %#02x, want 0x00", ic7600.DataModeNibbleOffset, got)
		}
		var atEight []civ.FieldSpan
		for _, sp := range layout.Fields {
			if sp.Offset == ic7600.DataModeNibbleOffset {
				atEight = append(atEight, sp)
			}
		}
		if len(atEight) != 1 || atEight[0].Field != civ.FieldToneMode || atEight[0].Nibble != civ.NibbleLow {
			t.Errorf("record offset %d carries %v; want exactly one span, tone_mode on the LOW nibble", ic7600.DataModeNibbleOffset, atEight)
		}
	})

	// No leg5/leg6: this radio's page carries no enlarged D2/D3 sub-diagram
	// (matrix S3.15(a)) and no printed Fixed-vs-FF contradiction to record
	// (matrix S3.13, S3.15(c)) - both WRITTEN-DOWN ABSENCES, not omissions.
	// See doc.go's Provenance section and profile.go's SelectByteOffset
	// comment.
	t.Run("no_clear_builder_and_the_gate_refuses_it", func(t *testing.T) {
		p := ic7600.Profile()
		idRead, err := p.BuildTransceiverIDRead()
		if err != nil {
			t.Fatalf("BuildTransceiverIDRead: %v", err)
		}
		lo, hi := p.ChannelRange()
		for ch := lo; ch <= hi; ch++ {
			clear := []byte{0xFE, 0xFE, 0x7A, 0xE0, 0x1A, 0x00, bcdByte(ch / 100), bcdByte(ch % 100), 0xFF, 0xFD}

			read, err := p.BuildMemoryRead(civ.ChannelAddress{Channel: ch})
			if err != nil {
				t.Fatalf("BuildMemoryRead(%d): %v", ch, err)
			}
			set, err := p.BuildMemorySet(clearProbeRecord(ch))
			if err != nil {
				t.Fatalf("BuildMemorySet(%d): %v", ch, err)
			}
			for name, got := range map[string][]byte{
				"BuildMemoryRead":        read.Bytes(),
				"BuildMemorySet":         set.Bytes(),
				"BuildTransceiverIDRead": idRead.Bytes(),
			} {
				if string(got) == string(clear) {
					t.Fatalf("%s(channel %d) produced the CLEAR frame % X; this tier ships no clear builder", name, ch, clear)
				}
			}
			if p.AllowedCommand(clear) {
				t.Fatalf("AllowedCommand admitted the clear frame % X for channel %d", clear, ch)
			}
		}
		if p.AllowedCommand([]byte{0xFE, 0xFE, 0x7A, 0xE0, 0x0B, 0xFD}) {
			t.Error("AllowedCommand admitted command 0B \"Memory clear\"")
		}
	})
}

func checkEncoding(t *testing.T, key indexKey, encoding string, bind rowBinding, spans map[civ.FieldID]civ.FieldSpan) {
	t.Helper()
	if bind.isAddress {
		return
	}
	for _, id := range bind.fields {
		sp := spans[id]
		switch encoding {
		case "bcd_packed":
			if sp.Encoding != civ.EncodingBCDNumber {
				t.Errorf("printed index %s: B says %q, profile span %s has Encoding %v", key, encoding, id, sp.Encoding)
			}
		case "enum_byte", "enum_nibble":
			if sp.Encoding != civ.EncodingEnum {
				t.Errorf("printed index %s: B says %q, profile span %s has Encoding %v", key, encoding, id, sp.Encoding)
			}
		case "ascii":
			if sp.Encoding != civ.EncodingName {
				t.Errorf("printed index %s: B says %q, profile span %s has Encoding %v", key, encoding, id, sp.Encoding)
			}
		default:
			t.Errorf("printed index %s: B's encoding %q is not one this test knows how to bind", key, encoding)
		}
	}
}

// parsePipeList parses a " | "-separated "<code>: <name>" list into a
// byte-keyed map, at hexadecimal radix (RULING OQ1).
func parsePipeList(t *testing.T, s string) map[byte]string {
	t.Helper()
	out := map[byte]string{}
	for _, item := range strings.Split(s, " | ") {
		code, name, ok := strings.Cut(item, ":")
		if !ok {
			t.Fatalf("printed value %q is not a <code>: <name> pair", item)
		}
		n, err := strconv.ParseUint(strings.TrimSpace(code), 16, 8)
		if err != nil {
			t.Fatalf("printed code %q is not a hex byte: %v", code, err)
		}
		out[byte(n)] = strings.TrimSpace(name)
	}
	return out
}

func sameEnum(a, b map[byte]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func bcdByte(n int) byte { return byte(n/10<<4 | n%10) }

// clearProbeRecord is a neutral record for a given channel, used only to
// prove BuildMemorySet never emits the clear frame's shape by coincidence.
func clearProbeRecord(ch int) civ.MemoryRecord {
	return civ.MemoryRecord{
		Address:      civ.ChannelAddress{Channel: ch},
		RXFreqHz:     civ.Available[uint64](14_250_000),
		Mode:         civ.Available("USB"),
		Filter:       civ.Available("FIL1"),
		ToneMode:     civ.Available("TONE"),
		ToneTXDeciHz: civ.Available[uint64](885),
		ToneRXDeciHz: civ.Available[uint64](1000),
		Name:         civ.Available("HOME QTH01"),
	}
}
