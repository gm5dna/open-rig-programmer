// SPDX-License-Identifier: GPL-3.0-or-later

package ic7800_test

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7800"
)

// This file cross-checks testdata/ic7800-transcription.csv — the S1 sweep's
// own field-by-field transcription of PDF pp.208-212, authored before this
// package existed and unchanged since — against the profile's layout().
//
// ONE SOURCE, NOT THREE. The IC-7610 lineage's crosscheck reconciles three
// independently-blind legs (a field ledger, a geometry witness and a
// transcription); this model's matrix was authored from a single reading
// of one document (matrix §0), so there is no second or third leg to
// reconcile it against. What this test CAN do honestly is confirm the one
// real external artefact this package has — a transcription written by an
// earlier evidence-gathering pass that never saw this profile — still
// agrees with the code.
func readTranscriptionCSV(t *testing.T) [][]string {
	t.Helper()
	f, err := os.Open(filepath.Join(testdataDir, "ic7800-transcription.csv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 2 {
		t.Fatalf("transcription CSV has %d rows, want a header plus at least one data row", len(rows))
	}
	return rows[1:]
}

// TestTranscriptionWidthsSumToTheDataArea is the same internal check the
// evidence file's own prose makes (evidence/ic7800.md: "1+5+2+1+3+3+10 =
// 25 bytes"), pinned mechanically against the CSV's own width_bytes column
// rather than trusted as prose.
func TestTranscriptionWidthsSumToTheDataArea(t *testing.T) {
	rows := readTranscriptionCSV(t)
	var sum int
	for _, row := range rows {
		w, err := strconv.Atoi(row[3])
		if err != nil {
			t.Fatalf("row %q: width_bytes %q is not an integer: %v", row[1], row[3], err)
		}
		sum += w
	}
	if sum != ic7800.DataAreaLength {
		t.Errorf("transcription widths sum to %d, want %d (DataAreaLength)", sum, ic7800.DataAreaLength)
	}
}

// fieldOffsetWidth maps a transcription row's label (by substring) to the
// record offset and width the profile's layout() carries for it, so a
// change to either side is caught without one being copied from the other.
var fieldOffsetWidth = map[string]struct {
	field       civ.FieldID
	offset, len int
}{
	"Operating frequency setting":     {civ.FieldRXFrequency, 1, 5},
	"Repeater tone frequency setting": {civ.FieldToneTX, 9, 3},
	"Tone squelch frequency setting":  {civ.FieldToneRX, 12, 3},
	"Memory name setting":             {civ.FieldName, 15, 10},
}

// TestTranscriptionOffsetsMatchTheProfile walks the transcription's own
// running byte count (the record starts after the 2-byte channel selector
// the transcription's first row describes) and checks it against every
// mapped field's span in the profile.
func TestTranscriptionOffsetsMatchTheProfile(t *testing.T) {
	rows := readTranscriptionCSV(t)
	spans := map[civ.FieldID]civ.FieldSpan{}
	for _, sp := range ic7800.Profile().Layouts()[0].Fields {
		spans[sp.Field] = sp
	}

	offset := -ic7800.AddressBytes // the first two rows (q,w) are the address, outside the record
	for _, row := range rows {
		label, widthStr := row[2], row[3]
		width, err := strconv.Atoi(widthStr)
		if err != nil {
			t.Fatalf("row %q: width_bytes %q is not an integer: %v", label, widthStr, err)
		}
		want, ok := fieldOffsetWidth[label]
		switch {
		case label == "Memory channel number":
			// the address, excluded from the record.
		case label == "Operating mode setting":
			// this transcription row covers BOTH mode and filter as one
			// printed group (indices o,!0); check each mapped sub-field
			// against its own half of the two-byte span.
			if sp, ok := spans[civ.FieldMode]; !ok || sp.Offset != offset {
				t.Errorf("FieldMode span = %+v, want offset %d", sp, offset)
			}
			if sp, ok := spans[civ.FieldFilter]; !ok || sp.Offset != offset+1 {
				t.Errorf("FieldFilter span = %+v, want offset %d", sp, offset+1)
			}
		case label == "Data mode and tone type setting":
			// one byte, two nibbles: tone_mode (mapped) and data mode
			// (E6-unmapped).
			if sp, ok := spans[civ.FieldToneMode]; !ok || sp.Offset != offset {
				t.Errorf("FieldToneMode span = %+v, want offset %d", sp, offset)
			}
			if ic7800.DataModeNibbleOffset != offset {
				t.Errorf("DataModeNibbleOffset = %d, want %d (transcription row %q)", ic7800.DataModeNibbleOffset, offset, label)
			}
		case ok:
			sp, present := spans[want.field]
			if !present {
				t.Errorf("transcription row %q names %s, which the profile does not map", label, want.field)
				break
			}
			if sp.Offset != offset || sp.Length != width {
				t.Errorf("%s span = offset %d length %d, want offset %d length %d (transcription row %q)",
					want.field, sp.Offset, sp.Length, offset, width, label)
			}
		case strings.Contains(label, "Select"):
			if ic7800.SelectNibbleOffset != offset {
				t.Errorf("SelectNibbleOffset = %d, want %d (transcription row %q)", ic7800.SelectNibbleOffset, offset, label)
			}
		default:
			t.Errorf("transcription row %q is not one this test knows how to check", label)
		}
		offset += width
	}
	if want := ic7800.RecordOnlyLength; offset != want {
		t.Errorf("walking the transcription's own widths ends at record offset %d, want %d", offset, want)
	}
}
