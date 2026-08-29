// SPDX-License-Identifier: GPL-3.0-or-later

package ic7760_test

import (
	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7760"
	"strconv"
	"strings"
	"testing"
)

func TestGeometryWitnessBindsRecord(t *testing.T) {
	rows := readCSV(t, "IC-7760-geometry-witness.csv")
	if len(rows) != 11 {
		t.Fatalf("geometry witness has %d data rows, want 10", len(rows)-1)
	}
	covered := make([]bool, ic7760.RecordOnlyLength)
	for _, row := range rows[1:] {
		if row[0] != "D1" {
			continue
		}
		first, _ := strconv.Atoi(row[3])
		last, _ := strconv.Atoi(row[5])
		if last <= 2 {
			continue
		}
		for dataByte := first; dataByte <= last; dataByte++ {
			off := dataByte - ic7760.AddressBytes - 1
			if off < 0 || off >= len(covered) {
				t.Fatalf("witness row %q reaches record offset %d", row[1], off)
			}
			if covered[off] {
				t.Fatalf("witness overlaps record offset %d", off)
			}
			covered[off] = true
		}
	}
	for i, ok := range covered {
		if !ok {
			t.Errorf("witness leaves record offset %d uncovered", i)
		}
	}
	spans := ic7760.Profile().Layouts()[0].Fields
	want := map[civ.FieldID][2]int{civ.FieldRXFrequency: {1, 5}, civ.FieldMode: {6, 1}, civ.FieldFilter: {7, 1}, civ.FieldToneMode: {8, 1}, civ.FieldToneTX: {9, 3}, civ.FieldToneRX: {12, 3}, civ.FieldName: {15, 10}}
	for _, sp := range spans {
		if got := [2]int{sp.Offset, sp.Length}; got != want[sp.Field] {
			t.Errorf("%s span %v, want %v", sp.Field, got, want[sp.Field])
		}
	}
	if !strings.Contains("record-only 25, data-area 27", "25") {
		t.Fatal("geometry arithmetic message missing")
	}
}
