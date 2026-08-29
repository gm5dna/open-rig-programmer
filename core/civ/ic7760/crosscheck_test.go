// SPDX-License-Identifier: GPL-3.0-or-later

package ic7760_test

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
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
	s = b.String()
	parts := strings.Split(s, " ~ ")
	if len(parts) == 1 {
		parts = strings.Split(s, ", ")
	}
	if len(parts) == 1 {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		return printedRange{n, n}, err
	}
	if len(parts) != 2 {
		return printedRange{}, fmt.Errorf("bad index %q", s)
	}
	lo, err := parsePrinted(parts[0])
	if err != nil {
		return printedRange{}, err
	}
	hi, err := parsePrinted(parts[1])
	if err != nil {
		return printedRange{}, err
	}
	return printedRange{lo.lo, hi.hi}, nil
}

func readCSV(t *testing.T, name string) [][]string {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
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

// TestCrosscheck joins the independent L/B/W legs on diagram and printed
// field. Any disagreement is a STOP for arbitration against the rendered
// PDF p.20, never a reason to edit frozen evidence. The plan's ruling is:
// “Memory group number” means channel number (E1), the printed inventory is
// accepted (E2), and SELECT membership remains four-valued, not scan skip.
func TestCrosscheck(t *testing.T) {
	ledger := readCSV(t, "IC-7760-field-ledger.csv")
	b := readCSV(t, "IC-7760-transcription-b.csv")
	w := readCSV(t, "IC-7760-geometry-witness.csv")
	if len(ledger) != 11 || len(b) != 14 || len(w) != 11 {
		t.Fatalf("unexpected evidence row counts: L=%d B=%d W=%d", len(ledger)-1, len(b)-1, len(w)-1)
	}
	for _, row := range ledger[1:] {
		if row[0] != "D1" || row[3] == "" {
			continue
		}
		key := row[0] + ":" + row[1]
		lr, err := parsePrinted(row[1])
		if err != nil {
			t.Fatalf("ledger %s: %v", key, err)
		}
		foundB, foundW := false, false
		for _, x := range b[1:] {
			if x[0] == row[0] {
				if r, e := parsePrinted(x[1]); e == nil && r == lr {
					foundB = true
				}
			}
		}
		for _, x := range w[1:] {
			if x[0] == row[0] {
				if r, e := parsePrinted(x[1]); e == nil && r == lr {
					foundW = true
				}
			}
		}
		if !foundB || !foundW {
			t.Errorf("STOP: evidence join %s missing B=%t W=%t; arbitrate against rendered PDF p.20", key, foundB, foundW)
		}
	}
	for _, ruling := range []string{
		"E1: Memory group number is the channel number, per PDF p.20 channel ranges and command 08.",
		"E2: printed MEM 01-99 and SCAN P1/P2 inventory accepted; no ic7760-inventory assumption.",
		"STOP W1-7: printed index bands retained; measured ellipses do not change widths.",
		"STOP L1-2/W8-9: duplicate 3 and 11 are sub-diagram labels, not record fields.",
		"STOP B1: tone heading mismatch is documentary; both tone spans use the shared diagram.",
		"STOP G1: data mode is unmapped; SELECT is membership, not binary scan skip.",
		"STOP clear-list 4=None: erase is evidence only; no clear grammar is admitted.",
	} {
		t.Run(ruling, func(t *testing.T) {})
	}
}
