// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx10

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/internal/extable"
)

type a3 = [3]int

func addr(a cat.EXAddress) a3 { return a3{int(a.P1), int(a.P2), int(a.P3)} }

func sortA3(s []a3) {
	sort.Slice(s, func(i, j int) bool {
		for k := 0; k < 3; k++ {
			if s[i][k] != s[j][k] {
				return s[i][k] < s[j][k]
			}
		}
		return false
	})
}

// deniedGolden is the exact denied set, spec v2 §2.1 (27 rows), against
// table2.csv; a renamed row in a chart edit fails the build.
var deniedGolden = []a3{
	{1, 1, 13}, {1, 1, 14}, {1, 1, 16}, {1, 2, 13}, {1, 2, 15}, {1, 2, 17},
	{1, 3, 12}, {1, 3, 14}, {1, 3, 16}, {1, 4, 15}, {1, 4, 16}, {1, 4, 18},
	{1, 5, 14}, {2, 1, 16}, {3, 1, 5}, {3, 1, 6}, {3, 1, 7}, {3, 1, 8},
	{3, 1, 9}, {3, 1, 10}, {3, 1, 16}, {3, 4, 1}, {3, 4, 2}, {3, 4, 3},
	{3, 4, 4}, {3, 4, 7}, {4, 1, 1},
}

// heldGolden is the exact held set, spec v2 §2.2 (8 rows).
var heldGolden = []a3{
	{1, 3, 20}, {1, 5, 15}, {1, 5, 16}, {3, 1, 11}, {3, 1, 12}, {3, 1, 20},
	{3, 4, 5}, {3, 4, 6},
}

// ledger is spec v2 §2.3's FTdx10 column: total/denied/held/admitted per
// (P1,P2) group.
var ledger = map[[2]int][4]int{
	{1, 1}: {16, 3, 0, 13}, {1, 2}: {17, 3, 0, 14}, {1, 3}: {21, 3, 1, 17},
	{1, 4}: {18, 3, 0, 15}, {1, 5}: {16, 1, 2, 13}, {1, 6}: {5, 0, 0, 5},
	{1, 7}: {6, 0, 0, 6}, {2, 1}: {18, 1, 0, 17}, {2, 2}: {11, 0, 0, 11},
	{2, 3}: {1, 0, 0, 1}, {3, 1}: {20, 7, 3, 10}, {3, 2}: {4, 0, 0, 4},
	{3, 3}: {19, 0, 0, 19}, {3, 4}: {7, 5, 2, 0}, {3, 5}: {7, 0, 0, 7},
	{4, 1}: {5, 1, 0, 4}, {4, 2}: {4, 0, 0, 4}, {4, 3}: {2, 0, 0, 2},
}

func TestFTdx10WritePolicy_MatchesTable2ByAddress(t *testing.T) {
	var denied, heldA, admitted []a3
	groups := map[[2]int][4]int{}
	byAddr := map[cat.EXAddress]cat.EXWriteItem{}
	for _, w := range exWrite {
		byAddr[w.Addr] = w
	}
	if len(exWrite) != 197 || len(exItems) != 197 {
		t.Fatalf("rows = %d write, %d items, want 197", len(exWrite), len(exItems))
	}
	for _, it := range exItems {
		g := [2]int{int(it.Addr.P1), int(it.Addr.P2)}
		c := groups[g]
		c[0]++
		_, isHeld := held[it.Addr]
		switch {
		case cat.EXWriteDeniedFTdx(it):
			denied = append(denied, addr(it.Addr))
			c[1]++
		case isHeld:
			heldA = append(heldA, addr(it.Addr))
			c[2]++
		default:
			admitted = append(admitted, addr(it.Addr))
			c[3]++
			if d := byAddr[it.Addr].Domain; len(d.Codes) == 0 && d.Hi == 0 && d.Lo == 0 {
				t.Errorf("admitted row %v %s has a zero Domain (legend did not parse)", it.Addr, it.Name)
			}
		}
		groups[g] = c
	}
	sortA3(denied)
	sortA3(heldA)
	if !reflect.DeepEqual(denied, deniedGolden) {
		t.Errorf("denied = %v, want %v", denied, deniedGolden)
	}
	if !reflect.DeepEqual(heldA, heldGolden) {
		t.Errorf("held = %v, want %v", heldA, heldGolden)
	}
	if len(admitted) != 162 {
		t.Errorf("admitted = %d, want 162", len(admitted))
	}
	if !reflect.DeepEqual(groups, ledger) {
		t.Errorf("per-group ledger = %v, want %v", groups, ledger)
	}
	// Every held address exists (FilterEXWrite also errors on one that does not).
	for a := range held {
		if !dialect.KnownEXAddress(a) {
			t.Errorf("held address %v is not in the inventory", a)
		}
	}
	// The same denied set falls out of the CSV, not just the generated inventory.
	data, err := os.ReadFile("table2.csv")
	if err != nil {
		t.Fatal(err)
	}
	p, _ := extable.Lookup("ftdx10")
	rows, err := extable.ParseCSV(p, data)
	if err != nil || len(rows) != 197 {
		t.Fatalf("ParseCSV: %d rows, %v", len(rows), err)
	}
}

func TestFTdx10WriteDomains_Sane(t *testing.T) {
	// The built (filtered) table: held rows such as SHIFT FREQUENCY carry the
	// chart's own duplicate code and never reach it.
	for _, w := range dialectConfig.EXWrite {
		d := w.Domain
		if d.Hi != 0 && d.Lo > d.Hi {
			t.Errorf("%v: Lo %d > Hi %d", w.Addr, d.Lo, d.Hi)
		}
		seen := map[int]bool{}
		for _, c := range d.Codes {
			if seen[c] {
				t.Errorf("%v: duplicate code %d", w.Addr, c)
			}
			seen[c] = true
			if d.Hi != 0 && c >= d.Lo && c <= d.Hi {
				t.Errorf("%v: code %d inside range %d..%d", w.Addr, c, d.Lo, d.Hi)
			}
		}
	}
	for _, a := range []cat.EXAddress{{P1: 1, P2: 2, P3: 14}, {P1: 1, P2: 3, P3: 13}} {
		for _, w := range dialectConfig.EXWrite {
			if w.Addr == a && !reflect.DeepEqual(w.Domain, cat.Domain{Codes: []int{1000}, Lo: 0, Hi: 100, Step: 1}) {
				t.Errorf("MIC GAIN %v domain = %+v", a, w.Domain)
			}
		}
	}
}

func TestFTdx10WriteDomains_EQ3FreqCoversItsLegend(t *testing.T) {
	// Legend "00: OFF 01..05 ... 06: 2000 ~ 18: 3200 Hz": codes 0..18 (layout 848-850, 864-866).
	for _, a := range []cat.EXAddress{{P1: 3, P2: 3, P3: 8}, {P1: 3, P2: 3, P3: 17}} {
		d, _, ok := PaperDialect().EXWriteDescriptor(a)
		if !ok {
			t.Fatalf("%v missing", a)
		}
		for v := 0; v <= 18; v++ {
			if !d.Contains(v) {
				t.Errorf("%v refuses code %d", a, v)
			}
		}
		if d.Contains(19) {
			t.Errorf("%v admits code 19", a)
		}
	}
}

// setValue renders a legal P4 for d at width w (sign plus w-1 digits when signed).
func setValue(d cat.Domain, w int) string {
	v := d.Lo
	if len(d.Codes) > 0 {
		v = d.Codes[0]
	}
	if d.Signed {
		sign := "+"
		if v < 0 {
			sign, v = "-", -v
		}
		return fmt.Sprintf("%s%0*d", sign, w-1, v)
	}
	return fmt.Sprintf("%0*d", w, v)
}

func TestFTdx10Dialects_UnconsentedRefusesAllPaperAcceptsExactly162(t *testing.T) {
	items := Dialect().EXItems()
	if len(items) != 197 {
		t.Fatalf("items = %d", len(items))
	}
	want := map[cat.EXAddress]bool{}
	for _, w := range dialectConfig.EXWrite {
		want[w.Addr] = true
	}
	if len(want) != 162 {
		t.Fatalf("filtered write table = %d rows, want 162", len(want))
	}
	if Dialect().EXWritePaper() || !PaperDialect().EXWritePaper() {
		t.Fatal("EXWritePaper flags wrong")
	}
	// Paper dialect first, then prove the unconsented one is unmoved (aliasing).
	accepted := 0
	for _, it := range items {
		d, w, _ := PaperDialect().EXWriteDescriptor(it.Addr)
		if !want[it.Addr] {
			if PaperDialect().CanSetEX(it.Addr) {
				t.Errorf("paper dialect admits denied/held %v %s", it.Addr, it.Name)
			}
			continue
		}
		if w != it.Digits {
			t.Errorf("%v width %d, want Digits %d", it.Addr, w, it.Digits)
		}
		if !PaperDialect().CanSetEX(it.Addr) {
			t.Errorf("paper dialect refuses admitted %v %s", it.Addr, it.Name)
			continue
		}
		cmd, err := PaperDialect().BuildEXSet(it.Addr, setValue(d, w))
		if err != nil {
			t.Errorf("BuildEXSet(%v): %v", it.Addr, err)
			continue
		}
		accepted++
		// Outbound allowlist: the Set frame passes the paper dialect, never the unconsented one.
		frame := cmd.Bytes()
		if !PaperDialect().AllowedCommand(frame) {
			t.Errorf("paper AllowedCommand(%q) = false", frame)
		}
		if Dialect().AllowedCommand(frame) {
			t.Errorf("unconsented AllowedCommand(%q) = true", frame)
		}
		// A value outside the domain or a wrong width is refused.
		if _, err := PaperDialect().BuildEXSet(it.Addr, setValue(d, w)+"0"); err == nil {
			t.Errorf("paper BuildEXSet(%v) accepted a wrong width", it.Addr)
		}
	}
	if accepted != 162 {
		t.Errorf("paper dialect accepted %d, want 162", accepted)
	}
	for _, it := range items {
		if Dialect().CanSetEX(it.Addr) {
			t.Errorf("unconsented CanSetEX(%v) = true", it.Addr)
		}
		p4 := strings.Repeat("0", it.Digits)
		if _, err := Dialect().BuildEXSet(it.Addr, p4); err == nil {
			t.Errorf("unconsented BuildEXSet(%v) built a frame", it.Addr)
		}
		// An EX Answer is byte-identical to a Set: refuse every one outbound (spec §3.5).
		if Dialect().AllowedCommand([]byte("EX" + Dialect().EXWire(it.Addr) + p4 + ";")) {
			t.Errorf("unconsented AllowedCommand accepts the Answer for %v", it.Addr)
		}
		if _, _, adm := Dialect().EXWriteDescriptor(it.Addr); adm && Dialect().CanSetEX(it.Addr) {
			t.Errorf("unconsented descriptor writable at %v", it.Addr)
		}
	}
}

func TestFTdx10PaperDialect_DeniedAndHeldAnswersRefused(t *testing.T) {
	for _, it := range PaperDialect().EXItems() {
		p4 := strings.Repeat("0", it.Digits)
		frame := []byte("EX" + PaperDialect().EXWire(it.Addr) + p4 + ";")
		_, isHeld := held[it.Addr]
		if (cat.EXWriteDeniedFTdx(it) || isHeld) && PaperDialect().AllowedCommand(frame) {
			t.Errorf("paper AllowedCommand(%q) = true for denied/held %v %s", frame, it.Addr, it.Name)
		}
	}
}

func TestFTdx10Dialects_NoAliasing(t *testing.T) {
	if reflect.ValueOf(Dialect()).FieldByName("exWrite").Pointer() == reflect.ValueOf(PaperDialect()).FieldByName("exWrite").Pointer() {
		t.Error("dialect pair shares one exWrite map")
	}
}
