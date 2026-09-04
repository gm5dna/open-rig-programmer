// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft891

import (
	"strings"
	"sync"
	"testing"
)

// EVERY EXPECTED ANSWER IN THIS FILE IS A LITERAL, spelt out as the manual's
// EX block prints the frame — "EX" + four address digits + the raw P4 + ";" —
// and never built by calling this package's own buildEXAnswer. That is
// fakeft891_test.go's standing rule for this package's goldens, and it applies
// with particular force here: the answer builder and the inventory expansion
// are the two things these tests exist to catch.
//
// THE ADDRESSES BELOW ARE THIS RADIO'S FOUR-DIGIT ONES. The FT-891's EX read
// frame is SEVEN bytes ("EX P1 P1 P1 P1 ;", ft891_layout.txt:513-522) where
// every registered sibling's is nine, and the four digits are P1 then P2 with
// no third component: 0803 is (P1,P2) = (08,03).

// --- EX read: the answer, and its width ---

// TestEXRead_KnownAddressDefault drives four addresses chosen to cover the
// width alphabet's extremes and the chart's own bounds, each with its answer
// spelt out in full.
//
// The chart's EX block prints "P1 : 0101 - 1803 (MENU Number)", which bounds it
// at exactly the first and last rows transcribed, so 0101 and 1803 are the
// endpoints. 0205 is the narrowest field this chart has (PEAK HOLD, one digit)
// and 0803 the widest (OTHER DISP, five) — the token this radio's alphabet
// extends to and no sibling's has.
//
// The values are INVENTED — n x '0' — doc.go's register entry THE EX MENU
// VALUES ARE INVENTED.
func TestEXRead_KnownAddressDefault(t *testing.T) {
	_, conn := newTestRadio(t)

	tests := []struct {
		name string
		send string
		want string
	}{
		{"0101 AGC FAST DELAY, four digits — the chart's first row", "EX0101;", "EX01010000;"},
		{"0205 PEAK HOLD, one digit — the narrowest field", "EX0205;", "EX02050;"},
		{"0803 OTHER DISP, five digits — the widest, and no sibling has it", "EX0803;", "EX080300000;"},
		{"1803 LCD VERSION, four digits — the chart's last row", "EX1803;", "EX18030000;"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := exchange(t, conn, tt.send); got != tt.want {
				t.Errorf("%s -> %q, want %q", tt.send, got, tt.want)
			}
		})
	}
}

// TestEXRead_UnknownAddress: a syntactically valid four-digit address that the
// chart never enumerated draws "?;" — doc.go's register entry AN
// OUT-OF-INVENTORY EX ADDRESS ANSWERS "?;". Membership comes from the chart's
// own rows via the generated inventory, not from any range rule.
//
// The three cases are the three ways an address can miss: past the end of a
// real group, a group prefix the chart has no rows for at all, and the zero
// address (which no chart row can carry, since P2 numbering starts at 01).
func TestEXRead_UnknownAddress(t *testing.T) {
	_, conn := newTestRadio(t)

	for _, tt := range []struct {
		name string
		send string
	}{
		{"0104 — group 01 has three items", "EX0104;"},
		{"1901 — the chart's last group is 18", "EX1901;"},
		{"0000 — P2 numbering starts at 01, so no row carries it", "EX0000;"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assertRejected(t, conn, tt.send)
		})
	}
}

// TestEXRead_MalformedBody: anything that is not exactly four ASCII digits is
// refused with the state unchanged, indistinguishably from an unknown address —
// "?;" being the protocol's single unattributed NAK.
//
// The six-digit case is the one this radio needs and its siblings do not: an
// FT-891 handed the FTdx10's or FT-710's nine-byte read frame must not answer
// it, and a length check written as "at least four digits" would.
func TestEXRead_MalformedBody(t *testing.T) {
	_, conn := newTestRadio(t)

	for _, tt := range []struct {
		name string
		send string
	}{
		{"no address at all", "EX;"},
		{"three digits", "EX010;"},
		{"the siblings' six-digit address", "EX010100;"},
		{"a non-digit byte", "EX01O1;"},
		{"a space", "EX 101;"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assertRejected(t, conn, tt.send)
		})
	}
}

// TestEXRead_LowerCaseCommandAccepted: the command NAME folds like every other
// on this radio ("You may use either lower or upper case charac-/ters",
// ft891_layout.txt:100-102), and the EX arm is reached through the same
// dispatch, so it inherits the fold rather than restating it.
func TestEXRead_LowerCaseCommandAccepted(t *testing.T) {
	_, conn := newTestRadio(t)
	for _, send := range []string{"ex0101;", "Ex0101;", "eX0101;"} {
		if got, want := exchange(t, conn, send), "EX01010000;"; got != want {
			t.Errorf("%s -> %q, want %q", send, got, want)
		}
	}
}

// TestEXSetShaped_NotModelled: an EX Set is a valid address followed by a P4
// payload, which is simply a too-long body to this READ-ONLY handler, so it
// draws "?;" and changes nothing.
//
// THAT IS A MODELLING GAP, NOT A CLAIM ABOUT THE RADIO — doc.go's "What this
// fake deliberately does NOT model" says so in terms. The test asserts the
// gap's shape (refused, and the address still answers its old value
// afterwards), which is what a later task adding EX Set would have to change
// deliberately rather than by accident.
func TestEXSetShaped_NotModelled(t *testing.T) {
	r, conn := newTestRadio(t)

	assertRejected(t, conn, "EX01010001;")
	if got, ok := r.EXState("0101"); !ok || got != "0000" {
		t.Errorf("EXState(\"0101\") after a Set-shaped frame = %q, %v, want the untouched default \"0000\", true", got, ok)
	}
	if got, want := exchange(t, conn, "EX0101;"), "EX01010000;"; got != want {
		t.Errorf("EX0101; after a Set-shaped frame -> %q, want %q", got, want)
	}
}

// --- The inventory the answers come from ---

// TestEXDefaults_CountsPerGroup recounts the expanded inventory against
// literals — 159 items across the 18 P1 groups the chart prints, sized as it
// prints them.
//
// The numbers are written out rather than derived from exGroups, so a
// projection that lost or duplicated a group fails here as well as at the
// generator's own structural count. The independent binding of these numbers to
// the DIALECT's inventory is core/transport's cross-check.
func TestEXDefaults_CountsPerGroup(t *testing.T) {
	defaults := EXDefaults()
	if len(defaults) != 159 {
		t.Errorf("EXDefaults() has %d addresses, want 159", len(defaults))
	}

	perP1 := map[string]int{}
	for addr := range defaults {
		if len(addr) != 4 {
			t.Errorf("address %q is %d bytes, want 4 — this radio's EX address is a pair", addr, len(addr))
			continue
		}
		perP1[addr[:2]]++
	}
	want := map[string]int{
		"01": 3, "02": 7, "03": 2, "04": 11, "05": 20, "06": 7,
		"07": 13, "08": 12, "09": 6, "10": 11, "11": 9, "12": 4,
		"13": 2, "14": 7, "15": 18, "16": 23, "17": 1, "18": 3,
	}
	for p1, n := range want {
		if perP1[p1] != n {
			t.Errorf("P1=%s has %d addresses, want %d", p1, perP1[p1], n)
		}
	}
	for p1 := range perP1 {
		if _, ok := want[p1]; !ok {
			t.Errorf("P1=%s is in the inventory and not in this test's table — a group has appeared", p1)
		}
	}
}

// TestEXDefaults_AddressesAreContiguousFromP2One pins the property the compact
// widths string encodes: within a group, P2 runs 01, 02, 03 … with no gaps, and
// the string's index IS the item index. A projection that renumbered a group
// would answer the right number of addresses at the wrong ones.
func TestEXDefaults_AddressesAreContiguousFromP2One(t *testing.T) {
	defaults := EXDefaults()
	if len(defaults) == 0 {
		t.Fatal("EXDefaults() is empty — this test would pass vacuously")
	}

	counts := map[string]int{}
	for addr := range defaults {
		counts[addr[:2]]++
	}
	for p1, n := range counts {
		for i := 1; i <= n; i++ {
			addr := p1 + twoDigits(i)
			if _, ok := defaults[addr]; !ok {
				t.Errorf("group %s has %d items but %s is absent — P2 is not contiguous from 01", p1, n, addr)
			}
		}
		if beyond := p1 + twoDigits(n+1); defaults[beyond] != "" {
			t.Errorf("group %s has %d items and %s also answers — the group is longer than its count", p1, n, beyond)
		}
	}
}

// twoDigits renders a 1-based item index as this chart's two-digit P2 field.
// Written out rather than reached for through fmt so that this test's
// expectations are built by test code alone.
func twoDigits(n int) string {
	return string([]byte{byte('0' + n/10), byte('0' + n%10)})
}

// TestEXDefaults_WidthShape is the '5' token's RED PROOF at the fake's own
// level, and the shape check that goes with it.
//
// Every default is all-'0' and between one and five bytes wide — five being
// this radio's widest field, where the FTdx10's alphabet stops at four (plus
// its one 12-byte text item, which this chart has no counterpart for and whose
// transcription could not describe one: gen/main.go's widthToken).
//
// The five-wide addresses are pinned BY NAME, as literals: 0803 OTHER DISP and
// 0804 OTHER SHIFT, whose signed "-3000 Hz - 0 - +3000 Hz" parameter counts its
// sign as a digit. core/cat/ft891/crosscheck_test.go pins the same two from the
// A side. A generator that refused '5' would have failed before this test ran;
// what this test catches is an EXPANSION that mishandled the token — five
// spaces, or a truncated field — which no count would notice.
func TestEXDefaults_WidthShape(t *testing.T) {
	defaults := EXDefaults()
	if len(defaults) == 0 {
		t.Fatal("EXDefaults() is empty — this test would pass vacuously")
	}

	widest := map[string]string{}
	for addr, p4 := range defaults {
		if len(p4) < 1 || len(p4) > 5 {
			t.Errorf("%s: default P4 %q is %d bytes, want 1-5", addr, p4, len(p4))
			continue
		}
		if strings.Trim(p4, "0") != "" {
			t.Errorf("%s: default P4 = %q, want %d x '0' — every item on this chart is numeric", addr, p4, len(p4))
		}
		if len(p4) == 5 {
			widest[addr] = p4
		}
	}
	if want := map[string]string{"0803": "00000", "0804": "00000"}; len(widest) != len(want) {
		t.Fatalf("five-byte defaults = %v, want exactly %v (0803 OTHER DISP and 0804 OTHER SHIFT)", widest, want)
	}
	for addr, want := range map[string]string{"0803": "00000", "0804": "00000"} {
		if got := widest[addr]; got != want {
			t.Errorf("%s: five-byte default = %q, want %q", addr, got, want)
		}
	}
}

// TestEXDefaults_Independent: every call must return a FRESH map, so mutating
// one result can never reach another call's, nor a constructed Radio's own
// state. New seeds each Radio from EXDefaults(), so a shared map would make
// every fake in a test binary share one menu.
func TestEXDefaults_Independent(t *testing.T) {
	first := EXDefaults()
	first["0101"] = "9999"
	delete(first, "0102")

	second := EXDefaults()
	if got := second["0101"]; got != "0000" {
		t.Errorf("EXDefaults()[\"0101\"] on a fresh call = %q, want \"0000\" (unaffected by mutating an earlier copy)", got)
	}
	if _, ok := second["0102"]; !ok {
		t.Error("EXDefaults() lost address 0102 after it was deleted from an earlier copy")
	}

	_, conn := newTestRadio(t)
	if got, want := exchange(t, conn, "EX0101;"), "EX01010000;"; got != want {
		t.Errorf("EX0101; after mutating an EXDefaults() copy -> %q, want %q", got, want)
	}
}

// --- WithEXSetting / WithEXUnavailable / EXState ---

func TestWithEXSetting_OverlayAndEXState(t *testing.T) {
	r, conn := newTestRadio(t, WithEXSetting("0506", "3"))

	got, ok := r.EXState("0506")
	if !ok || got != "3" {
		t.Fatalf("EXState(\"0506\") = %q, %v, want \"3\", true", got, ok)
	}

	if got, want := exchange(t, conn, "EX0506;"), "EX05063;"; got != want {
		t.Errorf("EX0506; after WithEXSetting -> %q, want %q", got, want)
	}

	// The overlay is verbatim and unvalidated, and it does not disturb its
	// neighbours.
	if got, ok := r.EXState("0507"); !ok || got != "0" {
		t.Errorf("EXState(\"0507\") = %q, %v, want the untouched default \"0\", true", got, ok)
	}
}

// TestWithEXSetting_AcceptsAnAddressTheInventoryDoesNotHave pins the option's
// deliberate looseness: it does not consult exGroups, so a test can make an
// out-of-inventory address answerable without editing the projection of
// transcription B that the cross-check depends on.
func TestWithEXSetting_AcceptsAnAddressTheInventoryDoesNotHave(t *testing.T) {
	if _, ok := EXDefaults()["1901"]; ok {
		t.Fatal("test fixture error: 1901 is in the inventory, so this test proves nothing")
	}
	r, conn := newTestRadio(t, WithEXSetting("1901", "7"))
	if got, ok := r.EXState("1901"); !ok || got != "7" {
		t.Fatalf("EXState(\"1901\") = %q, %v, want \"7\", true", got, ok)
	}
	if got, want := exchange(t, conn, "EX1901;"), "EX19017;"; got != want {
		t.Errorf("EX1901; after WithEXSetting on an out-of-inventory address -> %q, want %q", got, want)
	}
}

// TestWithEXUnavailable_ForcesRejectionForAKnownAddress: removing a map entry
// makes a KNOWN, otherwise-valid address answer exactly as an out-of-inventory
// one does. This is the seam a driver-level "this setting is unavailable" test
// needs, and it introduces no new assumed behaviour — it triggers the fake's
// existing documented "?;".
func TestWithEXUnavailable_ForcesRejectionForAKnownAddress(t *testing.T) {
	if _, ok := EXDefaults()["0101"]; !ok {
		t.Fatal("test fixture error: 0101 is not in the inventory, so this test proves nothing")
	}
	r, conn := newTestRadio(t, WithEXUnavailable("0101"))

	if p4, ok := r.EXState("0101"); ok {
		t.Errorf("EXState(\"0101\") = %q, true after WithEXUnavailable; want absent", p4)
	}
	assertRejected(t, conn, "EX0101;")

	// Its neighbour still answers, so the option removed one entry and not the
	// table.
	if got, want := exchange(t, conn, "EX0102;"), "EX01020000;"; got != want {
		t.Errorf("EX0102; (neighbour of the removed address) -> %q, want %q", got, want)
	}
}

// TestWithEXOptions_ComposeInOrder: the options are overlays applied in the
// order given, so the LAST one wins — the semantics WithSlot already has, and
// the reason both option doc comments say "applied to whatever exSettings
// already holds".
func TestWithEXOptions_ComposeInOrder(t *testing.T) {
	r, _ := newTestRadio(t, WithEXSetting("0205", "1"), WithEXUnavailable("0205"))
	if p4, ok := r.EXState("0205"); ok {
		t.Errorf("EXState(\"0205\") = %q, true; want absent (WithEXUnavailable came last)", p4)
	}

	r2, conn2 := newTestRadio(t, WithEXUnavailable("0205"), WithEXSetting("0205", "1"))
	if got, ok := r2.EXState("0205"); !ok || got != "1" {
		t.Errorf("EXState(\"0205\") = %q, %v, want \"1\", true (WithEXSetting came last)", got, ok)
	}
	if got, want := exchange(t, conn2, "EX0205;"), "EX02051;"; got != want {
		t.Errorf("EX0205; -> %q, want %q", got, want)
	}
}

// TestWithFactoryImage_LeavesTheMenuAlone pins the boundary between the two
// kinds of state this fake holds: an Image is the SLOT map and nothing else, so
// replacing it wholesale leaves the menu at its defaults.
//
// It matters because both are seeded in New and a future image-shaped option
// that reached the menu too would silently change what every --fake settings
// read renders.
func TestWithFactoryImage_LeavesTheMenuAlone(t *testing.T) {
	r, conn := newTestRadio(t, WithFactoryImage(func() map[string]MemState { return map[string]MemState{} }))
	if _, ok := r.SlotState("001"); ok {
		t.Fatal("test fixture error: the empty image left slot 001 populated")
	}
	if got, want := exchange(t, conn, "EX0101;"), "EX01010000;"; got != want {
		t.Errorf("EX0101; against an empty factory image -> %q, want %q", got, want)
	}
}

// TestEXState_ConcurrentWithReads polls EXState from one goroutine while the
// main test goroutine drives EX read exchanges through Port() — the EX
// equivalent of exercising SlotState concurrently with command processing (see
// the Radio doc comment: safe for concurrent use, run tests with -race). No
// *testing.T call is made from the background goroutine; its only job is to
// give -race something to catch if exSettings access were ever unguarded.
func TestEXState_ConcurrentWithReads(t *testing.T) {
	r, conn := newTestRadio(t)

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_, _ = r.EXState("0101")
			}
		}
	}()

	for i := 0; i < 20; i++ {
		if got, want := exchange(t, conn, "EX0101;"), "EX01010000;"; got != want {
			close(stop)
			wg.Wait()
			t.Fatalf("exchange %d: EX0101; -> %q, want %q", i, got, want)
		}
	}
	close(stop)
	wg.Wait()
}
