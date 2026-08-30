// SPDX-License-Identifier: GPL-3.0-or-later

package ic7760_test

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7760"
)

func TestGeometryWitnessBindsEveryByte(t *testing.T) {
	rows := readCSV(t, "IC-7760-geometry-witness.csv")
	if len(rows) != 11 {
		t.Fatalf("geometry witness has %d data rows, want 10", len(rows)-1)
	}
	covered := make([]bool, ic7760.DataAreaLength)
	for _, row := range rows[1:] {
		if row[0] != "D1" {
			continue
		}
		first, firstErr := strconv.Atoi(row[3])
		last, lastErr := strconv.Atoi(row[5])
		if firstErr != nil || lastErr != nil || row[4] != "1" || row[6] != "2" {
			t.Fatalf("witness row %q has invalid whole-byte span %s/%s..%s/%s", row[1], row[3], row[4], row[5], row[6])
		}
		for dataByte := first; dataByte <= last; dataByte++ {
			idx := dataByte - 1
			if covered[idx] {
				t.Fatalf("witness overlaps data-area byte %d", dataByte)
			}
			covered[idx] = true
		}
	}
	for i, ok := range covered {
		if !ok {
			t.Errorf("witness leaves data-area byte %d uncovered", i+1)
		}
	}
	if ic7760.AddressBytes+ic7760.RecordOnlyLength != ic7760.DataAreaLength {
		t.Fatalf("address %d + record-only %d != data-area %d", ic7760.AddressBytes, ic7760.RecordOnlyLength, ic7760.DataAreaLength)
	}
}

func TestGeometryPinsSpansFixedValuesAndEnums(t *testing.T) {
	layouts := ic7760.Profile().Layouts()
	if len(layouts) != 1 {
		t.Fatalf("Layouts() has %d entries, want 1", len(layouts))
	}
	layout := layouts[0]
	if layout.Length != 25 || len(layout.Fixed) != 25 {
		t.Fatalf("layout length/fixed = %d/%d, want 25/25", layout.Length, len(layout.Fixed))
	}
	for off, b := range layout.Fixed {
		if b != 0 {
			t.Errorf("Fixed[%d] = %02X, want 00", off, b)
		}
	}

	want := []civ.FieldSpan{
		{Field: civ.FieldRXFrequency, Offset: 1, Length: 5, Encoding: civ.EncodingBCDNumber, Order: civ.OrderLittleEndian, Scale: 1},
		{Field: civ.FieldMode, Offset: 6, Length: 1, Encoding: civ.EncodingEnum, Enum: map[byte]string{0x00: "LSB", 0x01: "USB", 0x02: "AM", 0x03: "CW", 0x04: "RTTY", 0x05: "FM", 0x07: "CW-R", 0x08: "RTTY-R", 0x12: "PSK", 0x13: "PSK-R"}},
		{Field: civ.FieldFilter, Offset: 7, Length: 1, Encoding: civ.EncodingEnum, Enum: map[byte]string{0x01: "FIL1", 0x02: "FIL2", 0x03: "FIL3"}},
		{Field: civ.FieldToneMode, Offset: 8, Length: 1, Nibble: civ.NibbleLow, Encoding: civ.EncodingEnum, Enum: map[byte]string{0: "OFF", 1: "TONE", 2: "TSQL"}},
		{Field: civ.FieldToneTX, Offset: 9, Length: 3, Encoding: civ.EncodingBCDNumber, Order: civ.OrderBigEndian, Scale: 1},
		{Field: civ.FieldToneRX, Offset: 12, Length: 3, Encoding: civ.EncodingBCDNumber, Order: civ.OrderBigEndian, Scale: 1},
		{Field: civ.FieldName, Offset: 15, Length: 10, Encoding: civ.EncodingName},
	}
	if !reflect.DeepEqual(layout.Fields, want) {
		t.Errorf("layout fields do not match the complete keyed witness:\n got %#v\nwant %#v", layout.Fields, want)
	}

	// Record offset 0 is select byte ③ and offset 8's high nibble is data
	// mode in byte ⑪. Both are intentionally unmapped because their four
	// values cannot fit the neutral bools, so the zero template is what
	// makes the builder preserve the only lossless writable form.
	if layout.Fixed[0] != 0 || layout.Fixed[8] != 0 {
		t.Errorf("unmapped select/data template = %02X/%02X, want 00/00", layout.Fixed[0], layout.Fixed[8])
	}
}
