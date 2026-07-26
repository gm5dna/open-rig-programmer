// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"testing"
)

// findItem returns the inventory item at (p1,p2,p3), or ok=false. It scans
// EXItems() rather than the generator's own maps, so the spot-check tests
// exercise the public surface end to end.
func findItem(t *testing.T, p1, p2, p3 uint8) (EXItem, bool) {
	t.Helper()
	for _, it := range FT710.EXItems() {
		if it.Addr == (EXAddress{p1, p2, p3}) {
			return it, true
		}
	}
	return EXItem{}, false
}

// TestEXInventory_CountsPerGroup pins the per-P1 item counts and the grand
// total. These numbers are the milestone's load-bearing cross-check.
func TestEXInventory_CountsPerGroup(t *testing.T) {
	perP1 := map[uint8]int{}
	for _, it := range FT710.EXItems() {
		perP1[it.Addr.P1]++
	}
	want := map[uint8]int{1: 94, 2: 31, 3: 65, 4: 16, 6: 90}
	for p1, n := range want {
		if perP1[p1] != n {
			t.Errorf("P1=%d: got %d items, want %d", p1, perP1[p1], n)
		}
	}
	if len(perP1) != len(want) {
		t.Errorf("got %d distinct P1 menus, want %d (%v)", len(perP1), len(want), perP1)
	}
	if total := len(FT710.EXItems()); total != 296 {
		t.Errorf("total items = %d, want 296", total)
	}
}

// TestEXInventory_MenuAndGroupStructure pins exactly 5 P1 menus and 21
// (P1,P2) subgroups, non-empty labels, label constancy within a menu/
// subgroup, and (P1,P2,P3) sort order.
func TestEXInventory_MenuAndGroupStructure(t *testing.T) {
	items := FT710.EXItems()

	menus := map[uint8]bool{}
	subgroups := map[[2]uint8]bool{}
	p1Labels := map[uint8]string{}
	p2Labels := map[[2]uint8]string{}

	var prev EXAddress
	for i, it := range items {
		a := it.Addr
		menus[a.P1] = true
		subgroups[[2]uint8{a.P1, a.P2}] = true

		if it.P1Label == "" {
			t.Errorf("item %v has empty P1Label", a)
		}
		if it.P2Label == "" {
			t.Errorf("item %v has empty P2Label", a)
		}

		if got, ok := p1Labels[a.P1]; ok {
			if got != it.P1Label {
				t.Errorf("P1=%d label not constant: %q vs %q", a.P1, got, it.P1Label)
			}
		} else {
			p1Labels[a.P1] = it.P1Label
		}
		key := [2]uint8{a.P1, a.P2}
		if got, ok := p2Labels[key]; ok {
			if got != it.P2Label {
				t.Errorf("(P1=%d,P2=%d) label not constant: %q vs %q", a.P1, a.P2, got, it.P2Label)
			}
		} else {
			p2Labels[key] = it.P2Label
		}

		if i > 0 && !addrLess(prev, a) {
			t.Errorf("items not strictly sorted at index %d: %v then %v", i, prev, a)
		}
		prev = a
	}

	if len(menus) != 5 {
		t.Errorf("got %d distinct P1 menus, want 5", len(menus))
	}
	if len(subgroups) != 21 {
		t.Errorf("got %d distinct (P1,P2) subgroups, want 21", len(subgroups))
	}
}

// addrLess is the strict (P1,P2,P3) ordering the inventory must be sorted by.
func addrLess(a, b EXAddress) bool {
	if a.P1 != b.P1 {
		return a.P1 < b.P1
	}
	if a.P2 != b.P2 {
		return a.P2 < b.P2
	}
	return a.P3 < b.P3
}

// TestEXInventory_P1AnomalyRecorded pins the transcribed P1
// anomaly at the data level: no item sits at P1==5, and P1==6 is present.
// (M8c put two P1==05 addresses to a real radio; both were rejected —
// see KnownEXAddress's doc comment.)
func TestEXInventory_P1AnomalyRecorded(t *testing.T) {
	var sawP5, sawP6 bool
	for _, it := range FT710.EXItems() {
		switch it.Addr.P1 {
		case 5:
			sawP5 = true
		case 6:
			sawP6 = true
		}
	}
	if sawP5 {
		t.Error("found an item at P1==5; the grammar's phantom 05 group must not appear in Table 2")
	}
	if !sawP6 {
		t.Error("no item at P1==6; EXTENSION SETTING must be present (Table 2 line ~904)")
	}
}

// TestEXInventory_NoDuplicatesSortedAndWireStable pins uniqueness, six-digit
// Wire() output, and a ParseEXAddress(Wire()) round-trip for every item.
func TestEXInventory_NoDuplicatesSortedAndWireStable(t *testing.T) {
	seen := map[EXAddress]bool{}
	for _, a := range FT710.EXAddresses() {
		if seen[a] {
			t.Errorf("duplicate address %v", a)
		}
		seen[a] = true

		wire := a.Wire()
		if len(wire) != 6 {
			t.Errorf("Wire()=%q for %v is not 6 digits", wire, a)
		}
		for i := 0; i < len(wire); i++ {
			if wire[i] < '0' || wire[i] > '9' {
				t.Errorf("Wire()=%q for %v contains a non-digit", wire, a)
				break
			}
		}
		if a.String() != wire {
			t.Errorf("String()=%q != Wire()=%q for %v", a.String(), wire, a)
		}

		back, err := FT710.ParseEXAddress(wire)
		if err != nil {
			t.Errorf("ParseEXAddress(%q) round-trip failed: %v", wire, err)
			continue
		}
		if back != a {
			t.Errorf("ParseEXAddress(%q) = %v, want %v", wire, back, a)
		}
	}
	if len(seen) != 296 {
		t.Errorf("got %d unique addresses, want 296", len(seen))
	}
}

// TestEXInventory_SpotChecksAgainstManual hand-recomputes selected rows from
// the manual (NOT via the generator) and asserts the inventory matches. Each
// expectation cites the manual extract line it was read from.
func TestEXInventory_SpotChecksAgainstManual(t *testing.T) {
	spots := []struct {
		p1, p2, p3 uint8
		name       string
		digits     int
		text       bool
		manualLine int // for the failure message only
	}{
		{1, 1, 1, "AF TREBLE GAIN", 3, false, 646},
		{1, 3, 21, "TONE FREQ", 2, false, 711},
		{3, 1, 5, "CAT-1 RATE", 1, false, 801},
		{3, 1, 26, "SCU-LAN10", 1, false, 827},
		{4, 1, 1, "MY CALL", 12, true, 879},
		{6, 5, 18, "RPTT SELECT", 1, false, 915},
		{6, 1, 1, "PRESET NAME", 12, true, 895},
	}
	for _, s := range spots {
		it, ok := findItem(t, s.p1, s.p2, s.p3)
		if !ok {
			t.Errorf("(%02d,%02d,%02d) %s (manual line %d): not in inventory", s.p1, s.p2, s.p3, s.name, s.manualLine)
			continue
		}
		if it.Name != s.name {
			t.Errorf("(%02d,%02d,%02d): Name=%q, want %q (manual line %d)", s.p1, s.p2, s.p3, it.Name, s.name, s.manualLine)
		}
		if it.Digits != s.digits {
			t.Errorf("(%02d,%02d,%02d) %s: Digits=%d, want %d (manual line %d)", s.p1, s.p2, s.p3, s.name, it.Digits, s.digits, s.manualLine)
		}
		if it.Text != s.text {
			t.Errorf("(%02d,%02d,%02d) %s: Text=%v, want %v (manual line %d)", s.p1, s.p2, s.p3, s.name, it.Text, s.text, s.manualLine)
		}
	}
}

// TestEXInventory_ExactlySixTextItems pins that the Text flag marks exactly
// MY CALL and the five PRESET NAME items, each 12 digits wide.
func TestEXInventory_ExactlySixTextItems(t *testing.T) {
	var texts []EXItem
	for _, it := range FT710.EXItems() {
		if it.Text {
			texts = append(texts, it)
			if it.Digits != 12 {
				t.Errorf("text item %v (%s) has Digits=%d, want 12", it.Addr, it.Name, it.Digits)
			}
		}
	}
	if len(texts) != 6 {
		t.Fatalf("got %d text items, want 6: %v", len(texts), texts)
	}
	// One MY CALL (04,01,01) plus PRESET NAME at (06,01..05,01).
	wantAddrs := map[EXAddress]string{
		{4, 1, 1}: "MY CALL",
		{6, 1, 1}: "PRESET NAME",
		{6, 2, 1}: "PRESET NAME",
		{6, 3, 1}: "PRESET NAME",
		{6, 4, 1}: "PRESET NAME",
		{6, 5, 1}: "PRESET NAME",
	}
	for _, it := range texts {
		want, ok := wantAddrs[it.Addr]
		if !ok {
			t.Errorf("unexpected text item at %v (%s)", it.Addr, it.Name)
			continue
		}
		if it.Name != want {
			t.Errorf("text item %v: Name=%q, want %q", it.Addr, it.Name, want)
		}
		delete(wantAddrs, it.Addr)
	}
	for a, name := range wantAddrs {
		t.Errorf("expected text item %v (%s) missing", a, name)
	}
}

// TestEXP4MaxBytesMatchesMaxDigits pins FT710.exP4MaxBytes() == the
// largest Digits over ITS OWN inventory, recomputed here from the public
// EXItems() rather than read from the stored field, so the bound the
// parser enforces can never drift from the data it is supposed to describe
// — including by a dialect literal that simply forgets to derive it, which
// the 1-byte floor would otherwise mask.
//
// It was a package const until M9b's fix wave (Codex finding 1); the
// per-dialect direction is proved in seconddialect_test.go, which is where
// a peer whose widest field is NOT the FT-710's lives.
func TestEXP4MaxBytesMatchesMaxDigits(t *testing.T) {
	max := 0
	for _, it := range FT710.EXItems() {
		if it.Digits > max {
			max = it.Digits
		}
	}
	if max != FT710.exP4MaxBytes() {
		t.Errorf("max Digits over inventory = %d, but exP4MaxBytes = %d", max, FT710.exP4MaxBytes())
	}
}

func TestKnownEXAddress(t *testing.T) {
	// Every inventory address is known.
	for _, a := range FT710.EXAddresses() {
		if !FT710.KnownEXAddress(a) {
			t.Errorf("KnownEXAddress(%v) = false, want true", a)
		}
	}
	// Non-members: the zero value, the phantom P1==5 group, an out-of-range
	// P3 in a real subgroup, and a P3 one past the PRESET block's 18.
	nonMembers := []EXAddress{
		{0, 0, 0},
		{5, 1, 1},
		{1, 1, 99},
		{6, 6, 19},
	}
	for _, a := range nonMembers {
		if FT710.KnownEXAddress(a) {
			t.Errorf("KnownEXAddress(%v) = true, want false", a)
		}
	}
}

func TestParseEXAddress_RejectTable(t *testing.T) {
	// Well-formed six-digit shapes that are not members.
	nonMemberWires := []string{"000000", "050101", "010199", "060619"}
	for _, w := range nonMemberWires {
		if _, err := FT710.ParseEXAddress(w); err == nil {
			t.Errorf("ParseEXAddress(%q): expected error (non-member), got nil", w)
		} else if _, ok := err.(*ParseError); !ok {
			t.Errorf("ParseEXAddress(%q): error is %T, want *ParseError", w, err)
		}
	}
	// Malformed shapes: non-digits and wrong lengths.
	malformed := []string{"", "01020", "0102030", "01020a", "abcdef", "01 203", "-10203"}
	for _, w := range malformed {
		if _, err := FT710.ParseEXAddress(w); err == nil {
			t.Errorf("ParseEXAddress(%q): expected error (bad shape), got nil", w)
		} else if _, ok := err.(*ParseError); !ok {
			t.Errorf("ParseEXAddress(%q): error is %T, want *ParseError", w, err)
		}
	}

	// NewEXAddress rejects a non-member triple (incl. the zero triple) as a
	// *ParseError, with no numeric-range shortcut.
	for _, tr := range [][3]int{{0, 0, 0}, {5, 1, 1}, {-1, 2, 3}, {256, 1, 1}} {
		if _, err := FT710.NewEXAddress(tr[0], tr[1], tr[2]); err == nil {
			t.Errorf("NewEXAddress%v: expected error, got nil", tr)
		} else if _, ok := err.(*ParseError); !ok {
			t.Errorf("NewEXAddress%v: error is %T, want *ParseError", tr, err)
		}
	}
}

// TestEXItems_ReturnsFreshCopies proves EXItems and EXAddresses never leak
// the package's backing data: mutating a returned slice cannot affect a later
// call.
func TestEXItems_ReturnsFreshCopies(t *testing.T) {
	items := FT710.EXItems()
	if len(items) == 0 {
		t.Fatal("EXItems returned nothing")
	}
	firstName := items[0].Name
	items[0].Name = "CLOBBERED"
	items[0].Addr = EXAddress{9, 9, 9}
	if again := FT710.EXItems(); again[0].Name != firstName || again[0].Addr == (EXAddress{9, 9, 9}) {
		t.Errorf("EXItems leaked backing data: second call sees %q / %v", again[0].Name, again[0].Addr)
	}

	addrs := FT710.EXAddresses()
	firstAddr := addrs[0]
	addrs[0] = EXAddress{9, 9, 9}
	if again := FT710.EXAddresses(); again[0] != firstAddr {
		t.Errorf("EXAddresses leaked backing data: second call sees %v", again[0])
	}
}

// --- M8c hardware READ observations (task 46) ---

// TestEXItems_ObservedReadWidthDeviatesOnlyForToneFreq pins the M8c
// finding that the manual's Digits column matched the observed READ width
// for 295 of the 296 addresses in those two sweeps, and did not for one:
// 01 03 21 TONE FREQ answered a three-byte P4 ("EX010321012;", captured
// outside this package's own codec) against the manual's printed 2.
//
// This is a read-direction observation about one radio, one firmware and
// one configuration. It says nothing about EX Set frame widths, which M8c
// did not probe.
func TestEXItems_ObservedReadWidthDeviatesOnlyForToneFreq(t *testing.T) {
	var deviations []EXItem
	for _, it := range FT710.EXItems() {
		if it.ObservedReadWidth == 0 {
			t.Errorf("%s has no M8c observation — every inventory address was read", it.Addr.Wire())
			continue
		}
		want := it.Digits
		if it.Text {
			want = 12
		}
		if it.ObservedReadWidth != want {
			deviations = append(deviations, it)
		}
	}
	if len(deviations) != 1 {
		t.Fatalf("manual/observed read-width deviations = %d, want exactly 1", len(deviations))
	}
	got := deviations[0]
	if got.Addr != (EXAddress{P1: 1, P2: 3, P3: 21}) || got.Name != "TONE FREQ" || got.Digits != 2 || got.ObservedReadWidth != 3 {
		t.Errorf("deviation = %s %s (manual %d / observed %d), want 01 03 21 TONE FREQ manual 2 / observed 3",
			got.Addr.Wire(), got.Name, got.Digits, got.ObservedReadWidth)
	}
}

// TestEXItems_ObservedReadShapes pins the shape classification the two M8c
// sweeps produced: 26 addresses answered with an explicit leading sign, the
// six Text items answered 12 bytes, and everything else answered plain
// digits.
func TestEXItems_ObservedReadShapes(t *testing.T) {
	counts := map[string]int{}
	for _, it := range FT710.EXItems() {
		counts[it.ObservedReadShape]++
		if it.Text && it.ObservedReadShape != "text" {
			t.Errorf("%s is a Text item but its observed read shape is %q", it.Addr.Wire(), it.ObservedReadShape)
		}
	}
	for shape, want := range map[string]int{"numeric": 264, "signed": 26, "text": 6} {
		if counts[shape] != want {
			t.Errorf("observed read shape %q = %d items, want %d (M8c session record)", shape, counts[shape], want)
		}
	}
}

// TestEXItems_ObservedReadWidthWithinP4Bounds proves the observations stay
// inside the wire bound the parser enforces for THIS dialect
// (FT710.exP4MaxBytes()), so no observation can describe a frame
// FT710.ParseEXAnswer would reject.
func TestEXItems_ObservedReadWidthWithinP4Bounds(t *testing.T) {
	for _, it := range FT710.EXItems() {
		if it.ObservedReadWidth < 1 || it.ObservedReadWidth > FT710.exP4MaxBytes() {
			t.Errorf("%s: observed read width %d is outside 1..%d", it.Addr.Wire(), it.ObservedReadWidth, FT710.exP4MaxBytes())
		}
	}
}
