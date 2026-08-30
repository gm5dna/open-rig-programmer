// SPDX-License-Identifier: GPL-3.0-or-later

package ic7100

import (
	"bytes"
	"testing"
)

type geometryClaim struct {
	kind       string
	recordLo   int
	recordSize int
}

func TestGeometryWitnessRowsHaveOneClaim(t *testing.T) {
	// One entry per D1 witness row. A claim is deliberately region-level:
	// the drawn elisions establish extents, not fabricated internal cells.
	claims := map[string]geometryClaim{
		"D1/1":           {"address", -3, 1},
		"D1/2,3":         {"address", -2, 2},
		"D1/4":           {"field", 0, 1},
		"D1/5-9":         {"field", 1, 5},
		"D1/10,11":       {"field", 6, 2},
		"D1/12":          {"field", 8, 1},
		"D1/13":          {"field", 9, 1},
		"D1/14":          {"fixed", 10, 1},
		"D1/15-17":       {"field", 11, 3},
		"D1/18-20":       {"field", 14, 3},
		"D1/21-23":       {"field", 17, 3},
		"D1/24":          {"fixed", 20, 1},
		"D1/25-27":       {"field", 21, 3},
		"D1/28-35":       {"fixed", 24, 8},
		"D1/36-43":       {"fixed", 32, 8},
		"D1/44-51":       {"fixed", 40, 8},
		"D1/filled:5-51": {"field", 48, 47},
		// The bar says 52–60, but the arbitrated body row establishes 16.
		"D1/52-60": {"field", 95, 16},
	}

	witness := readEvidenceCSV(t, "IC-7100-geometry-witness.csv")
	seen := make(map[string]bool, len(claims))
	for _, row := range witness.rows {
		key := evidenceKey(row)
		if len(key) < 3 || key[:3] != "D1/" {
			continue
		}
		claim, ok := claims[key]
		if !ok {
			t.Errorf("witness row %q (original %q) has no profile claim", key, row["field_index"])
			continue
		}
		if seen[key] {
			t.Errorf("witness row %q binds more than once", key)
		}
		seen[key] = true
		if claim.kind == "" || claim.recordSize <= 0 {
			t.Errorf("witness row %q has an empty claim: %+v", key, claim)
		}
		switch claim.kind {
		case "address", "field", "fixed", "unmapped":
		default:
			t.Errorf("witness row %q has more than one claim kind: %q", key, claim.kind)
		}
	}
	if len(seen) != len(claims) {
		t.Errorf("bound %d D1 witness rows, want %d", len(seen), len(claims))
	}
}

func TestAddressWidthAndGeometry(t *testing.T) {
	if AddressBytes != 3 || RecordLength != 111 || DataAreaLength != 114 {
		t.Fatalf("address/record/data lengths = %d/%d/%d, want 3/111/114", AddressBytes, RecordLength, DataAreaLength)
	}
	if got, want := 51-5+1, 47; got != want {
		t.Fatalf("duplicate arithmetic = %d, want 47 == (51 - 5 + 1)", got)
	}
	if duplicateBlockShift != 47 {
		t.Fatalf("duplicate shift = %d, want 47", duplicateBlockShift)
	}
	if got, want := 48+duplicateBlockShift-1, 94; got != want {
		t.Errorf("duplicate ends at record offset %d, want %d", got, want)
	}
	layout, ok := Profile().LayoutFor(RecordLength)
	if !ok {
		t.Fatal("111-byte layout missing")
	}
	for _, span := range layout.Fields {
		if span.Field == "name" && (span.Offset != 95 || span.Length != 16) {
			t.Errorf("name span = offset %d length %d, want 95/16", span.Offset, span.Length)
		}
	}
}

func TestGeometryTXDuplicate(t *testing.T) {
	cmd, err := Profile().BuildMemorySet(knownRecord())
	if err != nil {
		t.Fatalf("BuildMemorySet: %v", err)
	}
	frame := cmd.Bytes()
	dataArea := frame[6 : len(frame)-1]
	if len(dataArea) != DataAreaLength {
		t.Fatalf("data area length = %d, want %d", len(dataArea), DataAreaLength)
	}
	// ASSUMED: ic7100-tx-block-mandatory. In one-based data-area terms,
	// RX bytes 5–51 and TX bytes 52–98 must match for this known channel.
	if rx, tx := dataArea[4:51], dataArea[51:98]; !bytes.Equal(rx, tx) {
		t.Errorf("TX duplicate differs:\n RX % X\n TX % X", rx, tx)
	}
}
