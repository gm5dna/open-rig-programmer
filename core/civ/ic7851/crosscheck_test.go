// SPDX-License-Identifier: GPL-3.0-or-later

package ic7851_test

import (
	"encoding/csv"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7851"
)

func readCSV(t *testing.T, name string) [][]string {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func fieldRange(s string) (int, int, error) {
	s = strings.TrimSpace(s)
	for i, r := range []rune("①②③④⑤⑥⑦⑧⑨⑩⑪⑫⑬⑭⑮⑯⑰⑱⑲⑳㉑㉒㉓㉔㉕㉖㉗㉘㉙㉚㉛㉜㉝㉞㉟") {
		s = strings.ReplaceAll(s, string(r), strconv.Itoa(i+1))
	}
	if strings.Contains(s, ",") {
		s = strings.Split(s, ",")[0]
	}
	s = strings.TrimSpace(s)
	if strings.ContainsAny(s, "~–—") {
		parts := strings.FieldsFunc(s, func(r rune) bool { return r == '~' || r == '–' || r == '—' })
		if len(parts) != 2 {
			return 0, 0, strconv.ErrSyntax
		}
		a, e := strconv.Atoi(parts[0])
		if e != nil {
			return 0, 0, e
		}
		b, e := strconv.Atoi(parts[1])
		return a, b, e
	}
	n, err := strconv.Atoi(s)
	return n, n, err
}

func TestCrosscheckLegsConsumeAllRows(t *testing.T) {
	ledger := readCSV(t, "IC-7851-field-ledger.csv")
	if len(ledger) != 10 {
		t.Fatalf("ledger rows = %d, want 9 data rows", len(ledger)-1)
	}
	for _, row := range ledger[1:] {
		if row[0] != "D1" && row[0] != "D2" {
			t.Fatalf("unexpected diagram %q", row[0])
		}
		if row[1] == "" {
			continue
		}
		if _, _, err := fieldRange(row[1]); err != nil {
			t.Fatalf("ledger index %q: %v", row[1], err)
		}
	}
	geometry := readCSV(t, "IC-7851-geometry-witness.csv")
	if len(geometry) != 12 {
		t.Fatalf("geometry rows = %d, want 11 data rows", len(geometry)-1)
	}
	for _, row := range geometry[1:] {
		if row[0] != "D1" && row[0] != "D2" {
			t.Fatalf("unexpected geometry diagram %q", row[0])
		}
		for _, col := range []int{3, 5} {
			if _, err := strconv.Atoi(row[col]); err != nil {
				t.Fatalf("geometry position %q: %v", row[col], err)
			}
		}
	}
	b := readCSV(t, "IC-7851-transcription-b.csv")
	if len(b) != 12 {
		t.Fatalf("transcription rows = %d, want 11 data rows", len(b)-1)
	}
	for _, row := range b[1:] {
		if _, _, err := fieldRange(row[1]); err != nil && row[1] != "" {
			t.Fatalf("transcription index %q: %v", row[1], err)
		}
	}
}

func TestCrosscheckProfileAgainstLegs(t *testing.T) {
	spans := ic7851.Profile().Layouts()[0].Fields
	want := []struct {
		field       civ.FieldID
		off, length int
	}{
		{civ.FieldRXFrequency, 1, 5}, {civ.FieldMode, 6, 1}, {civ.FieldFilter, 7, 1},
		{civ.FieldToneMode, 8, 1}, {civ.FieldToneTX, 9, 3}, {civ.FieldToneRX, 12, 3}, {civ.FieldName, 15, 10},
	}
	if len(spans) != len(want) {
		t.Fatalf("profile spans = %d, want %d; E6-unmapped select/data nibbles must have no span", len(spans), len(want))
	}
	for i, w := range want {
		if spans[i].Field != w.field || spans[i].Offset != w.off || spans[i].Length != w.length {
			t.Errorf("span %d = %#v, want %s at %d/%d", i, spans[i], w.field, w.off, w.length)
		}
	}
	if spans[3].Nibble != civ.NibbleLow {
		t.Fatalf("tone mode nibble = %v, want low", spans[3].Nibble)
	}
	if spans[4].Order != civ.OrderBigEndian || spans[5].Order != civ.OrderBigEndian {
		t.Fatal("tone fields must be big-endian packed BCD")
	}
	// Arbitration recorded by the matrix: the printed ellipses do not alter
	// widths; duplicate ⑪ is one byte with high=data mode and low=tone type.
	// E-1 (18-12/18-14), E-2 (⑪ label), and E-3 (digit/space codes) are
	// resolved by the plan and remain outside the shipped 1A 00 profile claim.
}
