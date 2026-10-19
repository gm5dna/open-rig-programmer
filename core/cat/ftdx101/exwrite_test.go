// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx101

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

// deniedGolden is the exact denied set, spec v2 §2.1 (28 rows), against
// table2.csv; a renamed row in a chart edit fails the build.
var deniedGolden = []a3{
	{1, 1, 11}, {1, 1, 12}, {1, 1, 14}, {1, 2, 11}, {1, 2, 13}, {1, 2, 15},
	{1, 3, 10}, {1, 3, 12}, {1, 3, 14}, {1, 4, 13}, {1, 4, 14}, {1, 4, 16},
	{1, 5, 12}, {2, 1, 15}, {3, 1, 3}, {3, 1, 8}, {3, 1, 9}, {3, 1, 10},
	{3, 1, 11}, {3, 1, 12}, {3, 1, 13}, {3, 1, 18}, {3, 4, 1}, {3, 4, 2},
	{3, 4, 3}, {3, 4, 4}, {3, 4, 7}, {4, 1, 1},
}

// heldGolden is the exact held set, spec v2 §2.2 (8 rows).
var heldGolden = []a3{
	{1, 5, 13}, {1, 5, 14}, {2, 1, 12}, {3, 1, 14}, {3, 1, 15}, {3, 1, 23},
	{3, 4, 5}, {3, 4, 6},
}

// ledger is spec v2 §2.3's FTdx101 column: total/denied/held/admitted per
// (P1,P2) group.
var ledger = map[[2]int][4]int{
	{1, 1}: {14, 3, 0, 11}, {1, 2}: {15, 3, 0, 12}, {1, 3}: {16, 3, 0, 13},
	{1, 4}: {16, 3, 0, 13}, {1, 5}: {14, 1, 2, 11}, {1, 6}: {5, 0, 0, 5},
	{1, 7}: {6, 0, 0, 6}, {2, 1}: {17, 1, 1, 15}, {2, 2}: {13, 0, 0, 13},
	{2, 3}: {1, 0, 0, 1}, {3, 1}: {23, 8, 3, 12}, {3, 2}: {5, 0, 0, 5},
	{3, 3}: {20, 0, 0, 20}, {3, 4}: {7, 5, 2, 0}, {3, 5}: {7, 0, 0, 7},
	{4, 1}: {8, 1, 0, 7}, {4, 2}: {4, 0, 0, 4}, {4, 3}: {2, 0, 0, 2},
}

func TestFTdx101WritePolicy_MatchesTable2ByAddress(t *testing.T) {
	var denied, heldA, admitted []a3
	groups := map[[2]int][4]int{}
	byAddr := map[cat.EXAddress]cat.EXWriteItem{}
	for _, w := range exWrite {
		byAddr[w.Addr] = w
	}
	if len(exWrite) != 193 || len(exItems) != 193 {
		t.Fatalf("rows = %d write, %d items, want 193", len(exWrite), len(exItems))
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
	if len(admitted) != 157 {
		t.Errorf("admitted = %d, want 157", len(admitted))
	}
	if !reflect.DeepEqual(groups, ledger) {
		t.Errorf("per-group ledger = %v, want %v", groups, ledger)
	}
	// Every held address exists (FilterEXWrite also errors on one that does not).
	for a := range held {
		if !DialectD().KnownEXAddress(a) {
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
	if err != nil || len(rows) != 193 {
		t.Fatalf("ParseCSV: %d rows, %v", len(rows), err)
	}
}

func TestFTdx101WriteDomains_Sane(t *testing.T) {
	// The built (filtered) table: held rows such as SHIFT FREQUENCY carry the
	// chart's own duplicate code and never reach it.
	for _, w := range mustFilter() {
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
	for _, a := range []cat.EXAddress{{P1: 1, P2: 2, P3: 12}, {P1: 1, P2: 3, P3: 11}} {
		for _, w := range mustFilter() {
			if w.Addr == a && !reflect.DeepEqual(w.Domain, cat.Domain{Codes: []int{1000}, Lo: 0, Hi: 100, Step: 1}) {
				t.Errorf("MIC GAIN %v domain = %+v", a, w.Domain)
			}
		}
	}
}

func eq3FreqCoversItsLegend(t *testing.T, static, paper cat.Dialect) {
	// Legend "00: OFF 01..05 ... 06: 2000 ~ 18: 3200 Hz": codes 0..18 (layout 909-911, 922-924).
	for _, a := range []cat.EXAddress{{P1: 3, P2: 3, P3: 9}, {P1: 3, P2: 3, P3: 18}} {
		d, _, ok := paper.EXWriteDescriptor(a)
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

func unconsentedRefusesAllPaperAcceptsExactly157(t *testing.T, static, paper cat.Dialect) {
	items := static.EXItems()
	if len(items) != 193 {
		t.Fatalf("items = %d", len(items))
	}
	want := map[cat.EXAddress]bool{}
	for _, w := range mustFilter() {
		want[w.Addr] = true
	}
	if len(want) != 157 {
		t.Fatalf("filtered write table = %d rows, want 157", len(want))
	}
	if static.EXWritePaper() || !paper.EXWritePaper() {
		t.Fatal("EXWritePaper flags wrong")
	}
	// Paper dialect first, then prove the unconsented one is unmoved (aliasing).
	accepted := 0
	for _, it := range items {
		d, w, _ := paper.EXWriteDescriptor(it.Addr)
		if !want[it.Addr] {
			if paper.CanSetEX(it.Addr) {
				t.Errorf("paper dialect admits denied/held %v %s", it.Addr, it.Name)
			}
			continue
		}
		if w != it.Digits {
			t.Errorf("%v width %d, want Digits %d", it.Addr, w, it.Digits)
		}
		if !paper.CanSetEX(it.Addr) {
			t.Errorf("paper dialect refuses admitted %v %s", it.Addr, it.Name)
			continue
		}
		cmd, err := paper.BuildEXSet(it.Addr, setValue(d, w))
		if err != nil {
			t.Errorf("BuildEXSet(%v): %v", it.Addr, err)
			continue
		}
		accepted++
		// Outbound allowlist: the Set frame passes the paper dialect, never the unconsented one.
		frame := cmd.Bytes()
		if !paper.AllowedCommand(frame) {
			t.Errorf("paper AllowedCommand(%q) = false", frame)
		}
		if static.AllowedCommand(frame) {
			t.Errorf("unconsented AllowedCommand(%q) = true", frame)
		}
		// A value outside the domain or a wrong width is refused.
		if _, err := paper.BuildEXSet(it.Addr, setValue(d, w)+"0"); err == nil {
			t.Errorf("paper BuildEXSet(%v) accepted a wrong width", it.Addr)
		}
	}
	if accepted != 157 {
		t.Errorf("paper dialect accepted %d, want 157", accepted)
	}
	for _, it := range items {
		if static.CanSetEX(it.Addr) {
			t.Errorf("unconsented CanSetEX(%v) = true", it.Addr)
		}
		p4 := strings.Repeat("0", it.Digits)
		if _, err := static.BuildEXSet(it.Addr, p4); err == nil {
			t.Errorf("unconsented BuildEXSet(%v) built a frame", it.Addr)
		}
		// An EX Answer is byte-identical to a Set: refuse every one outbound (spec §3.5).
		if static.AllowedCommand([]byte("EX" + static.EXWire(it.Addr) + p4 + ";")) {
			t.Errorf("unconsented AllowedCommand accepts the Answer for %v", it.Addr)
		}
		if _, _, adm := static.EXWriteDescriptor(it.Addr); adm && static.CanSetEX(it.Addr) {
			t.Errorf("unconsented descriptor writable at %v", it.Addr)
		}
	}
}

func deniedAndHeldAnswersRefused(t *testing.T, static, paper cat.Dialect) {
	for _, it := range paper.EXItems() {
		p4 := strings.Repeat("0", it.Digits)
		frame := []byte("EX" + paper.EXWire(it.Addr) + p4 + ";")
		_, isHeld := held[it.Addr]
		if (cat.EXWriteDeniedFTdx(it) || isHeld) && paper.AllowedCommand(frame) {
			t.Errorf("paper AllowedCommand(%q) = true for denied/held %v %s", frame, it.Addr, it.Name)
		}
	}
}

func noAliasing(t *testing.T, static, paper cat.Dialect) {
	if reflect.ValueOf(static).FieldByName("exWrite").Pointer() == reflect.ValueOf(paper).FieldByName("exWrite").Pointer() {
		t.Error("dialect pair shares one exWrite map")
	}
}

// models lists the two FTdx101 constants: one table, two dialect pairs.
var models = []struct {
	name          string
	static, paper func() cat.Dialect
}{
	{"D", DialectD, PaperDialectD},
	{"MP", DialectMP, PaperDialectMP},
}

func TestFTdx101Dialects_PerModel(t *testing.T) {
	for _, m := range models {
		t.Run(m.name, func(t *testing.T) {
			s, p := m.static(), m.paper()
			eq3FreqCoversItsLegend(t, s, p)
			unconsentedRefusesAllPaperAcceptsExactly157(t, s, p)
			deniedAndHeldAnswersRefused(t, s, p)
			noAliasing(t, s, p)
			// After the paper dialect was exercised, the static one still refuses all.
			for _, it := range s.EXItems() {
				if s.CanSetEX(it.Addr) {
					t.Errorf("static CanSetEX(%v) = true after paper use", it.Addr)
				}
			}
		})
	}
}

// One table serves D and MP: same inventory, same write table contents, but
// four distinct dialects with no shared storage.
func TestFTdx101Dialects_DAndMPShareTableNotStorage(t *testing.T) {
	d, mp := PaperDialectD(), PaperDialectMP()
	if !reflect.DeepEqual(d.EXItems(), mp.EXItems()) {
		t.Error("D and MP inventories differ")
	}
	for _, it := range d.EXItems() {
		dd, dw, dok := d.EXWriteDescriptor(it.Addr)
		md, mw, mok := mp.EXWriteDescriptor(it.Addr)
		if dok != mok || dw != mw || !reflect.DeepEqual(dd, md) {
			t.Errorf("%v: D and MP write rows differ", it.Addr)
		}
		if len(dd.Codes) > 0 && &dd.Codes[0] == &md.Codes[0] {
			t.Errorf("%v: D and MP share a Codes backing array", it.Addr)
		}
	}
	ptr := func(x cat.Dialect) uintptr { return reflect.ValueOf(x).FieldByName("exWrite").Pointer() }
	seen := map[uintptr]bool{}
	for _, x := range []cat.Dialect{DialectD(), DialectMP(), d, mp} {
		if seen[ptr(x)] {
			t.Error("two FTdx101 dialects share one exWrite map")
		}
		seen[ptr(x)] = true
	}
}

// The MAX POWER legends are model-conditional and parse first-match, so they
// are denied (layout 927-931): the paper dialect refuses all four.
func TestFTdx101WritePolicy_MaxPowerDenied(t *testing.T) {
	for p3 := uint16(1); p3 <= 4; p3++ {
		a := cat.EXAddress{P1: 3, P2: 4, P3: p3}
		for _, m := range models {
			if m.paper().CanSetEX(a) {
				t.Errorf("%s paper dialect admits MAX POWER %v", m.name, a)
			}
		}
	}
}
