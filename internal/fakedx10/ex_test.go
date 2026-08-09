// SPDX-License-Identifier: GPL-3.0-or-later

package fakedx10

import (
	"strings"
	"sync"
	"testing"
)

// This file's exchange tests recompute every expected reply byte string as a
// LITERAL, from the EX grammar and from transcription B's own Digits column —
// never by calling this package's own builders, and never from the generated
// table. The addresses and widths below were read off the committed CSV row by
// row; that is the same independence discipline the rest of this package's tests
// keep (see fakedx10_test.go's header), and it is what makes a mistake in the
// projection visible here rather than agreed with.
//
// The B rows these literals come from:
//
//	010101  AF TREBLE GAIN   Digits 3   (line 2)
//	010104  AGC FAST DELAY   Digits 4   (line 5)
//	010107  LCUT FREQ        Digits 2   (line 8)
//	010108  LCUT SLOP        Digits 1   (line 9)
//	030108  CAT RATE         Digits 1   (line 138)
//	040101  MY CALL.         Digits 12  (line 188 — the chart's ONE text item)

// --- Reads: known address, default state ---

func TestEXRead_KnownAddressDefault(t *testing.T) {
	// The text item's answer is 21 bytes: "EX"(2) + addr(6) + P4(12) + ";"(1).
	textWant := "EX040101" + strings.Repeat(" ", 12) + ";"
	if len(textWant) != 21 {
		t.Fatalf("test fixture error: textWant %q has length %d, want 21", textWant, len(textWant))
	}

	tests := []struct {
		name string
		req  string
		want string
	}{
		{"1-digit width default (LCUT SLOP)", "EX010108;", "EX0101080;"},
		{"1-digit width default (CAT RATE)", "EX030108;", "EX0301080;"},
		{"2-digit width default (LCUT FREQ)", "EX010107;", "EX01010700;"},
		{"3-digit width default (AF TREBLE GAIN)", "EX010101;", "EX010101000;"},
		{"4-digit width default (AGC FAST DELAY)", "EX010104;", "EX0101040000;"},
		{"text default, 12 spaces (MY CALL.)", "EX040101;", textWant},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, conn := newTestRadio(t)
			writeFrame(t, conn, tt.req)
			if got := mustReadFrame(t, conn); got != tt.want {
				t.Errorf("%s -> %q, want %q", tt.req, got, tt.want)
			}
		})
	}
}

// --- Reads: valid-shape but unknown address ---

// TestEXRead_UnknownAddress covers doc.go register entry 17's first half: a
// syntactically valid six-digit address this fake holds no entry for answers
// "?;", the protocol's single unattributed NAK. ASSUMED for this radio — the
// FT-710's equivalent was OBSERVED at M8c for six addresses; no FTdx10 has been
// asked anything.
//
// The P1=05 case is the FTdx10's own chart anomaly: the EX grammar block says
// "P1 : 01 - 05" while the chart populates 01-04 with no P1=05 group at all
// (core/cat/ftdx10/doc.go records it UNRESOLVED). Membership here comes from the
// chart's rows, so 05xxxx answers "?;" because nothing was transcribed for it —
// not because a range was enforced.
func TestEXRead_UnknownAddress(t *testing.T) {
	tests := []struct{ name, req string }{
		{"no P1=05 group in the chart", "EX050101;"},
		{"no P1=06 group either (unlike the FT-710)", "EX060101;"},
		{"P3 beyond its group's item count (01/01 has 16)", "EX010117;"},
		{"P3 beyond its group's item count (04/03 has 2)", "EX040303;"},
		{"P2 beyond its P1's subgroup count (01 has 07)", "EX010801;"},
		{"all zeros", "EX000000;"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, conn := newTestRadio(t)
			writeFrame(t, conn, tt.req)
			if got, want := mustReadFrame(t, conn), "?;"; got != want {
				t.Errorf("%s -> %q, want %q", tt.req, got, want)
			}
		})
	}
}

// --- Malformed bodies ---

func TestEXRead_MalformedBody(t *testing.T) {
	tests := []struct{ name, req string }{
		{"empty body", "EX;"},
		{"5-digit body", "EX01010;"},
		{"7-digit body", "EX0101011;"},
		{"non-digit body", "EX01A101;"},
		{"space in body", "EX0101 1;"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, conn := newTestRadio(t)
			writeFrame(t, conn, tt.req)
			if got, want := mustReadFrame(t, conn), "?;"; got != want {
				t.Errorf("%s -> %q, want %q", tt.req, got, want)
			}
		})
	}
}

// TestEXRead_LowerCaseCommandAccepted checks that the case leniency this
// radio's manual states — "You may use either lower or upper case characters."
// (manual lines 160-161) — reaches the EX arm too, and not merely the arms
// fakedx10_test.go's TestCommandNamesAreAcceptedInEitherCase exercises.
// EX is checked here because it is the newest command arm, and the one a later
// re-tightening would most plausibly miss.
//
// It REPLACES TestEXRead_LowerCaseCommandRejected, which asserted the opposite
// on the strength of the withdrawn register entry 12 — see doc.go's "What is
// NOT in this register, and why".
func TestEXRead_LowerCaseCommandAccepted(t *testing.T) {
	_, conn := newTestRadio(t)
	writeFrame(t, conn, "ex010101;")
	lower := mustReadFrame(t, conn)
	writeFrame(t, conn, "EX010101;")
	upper := mustReadFrame(t, conn)
	if lower != upper {
		t.Errorf("ex010101; -> %q but EX010101; -> %q — the command name's case must not matter", lower, upper)
	}
	if lower == "?;" {
		t.Errorf("EX010101; -> %q for both cases, so the equality above proves nothing", lower)
	}
}

// --- Set-shaped bodies: not modelled ---

// TestEXSetShaped_NotModelled is register entry 17's second half. "EX0101011;"
// is shaped like a real Set frame (address "010101" + a P4 digit "1"), and
// handleEX rejects any non-six-digit body uniformly, so it draws "?;" — and,
// critically, WITHOUT applying the write. This is a deliberate modelling gap (the
// manual documents a Set form), not a claim that a real FTdx10 refuses EX Set.
func TestEXSetShaped_NotModelled(t *testing.T) {
	r, conn := newTestRadio(t)
	writeFrame(t, conn, "EX0101011;")
	if got, want := mustReadFrame(t, conn), "?;"; got != want {
		t.Errorf("EX0101011; (set-shaped) -> %q, want %q", got, want)
	}
	if got, ok := r.EXState("010101"); !ok || got != "000" {
		t.Errorf("EXState(\"010101\") after a rejected set-shaped body = %q, %v, want \"000\", true (state unchanged)", got, ok)
	}
}

// --- EXDefaults(): structural self-checks ---

// TestEXDefaults_CountsPerGroup is this package's independent recount of the
// committed transcription B: 197 items, 99/30/57/11 across P1 01-04, and NO
// P1=05 or P1=06 group (the FT-710's chart has a P1=06 EXTENSION SETTING block
// of 90 items; the FTdx10's has none).
//
// The numbers are literals, read off the CSV's group boundaries, not derived
// from exGroups — a count computed from the table it is checking proves nothing.
// gen/main_test.go pins the same totals from the CSV side, and
// core/transport/ex_crosscheck_ftdx10_test.go binds them to the DIALECT's
// independently generated inventory; three derivations, one fact.
func TestEXDefaults_CountsPerGroup(t *testing.T) {
	d := EXDefaults()
	if len(d) != 197 {
		t.Fatalf("len(EXDefaults()) = %d, want 197", len(d))
	}
	counts := map[string]int{}
	for addr := range d {
		counts[addr[:2]]++
	}
	want := map[string]int{"01": 99, "02": 30, "03": 57, "04": 11}
	for p1, wantN := range want {
		if counts[p1] != wantN {
			t.Errorf("P1=%s count = %d, want %d", p1, counts[p1], wantN)
		}
	}
	for _, absent := range []string{"05", "06"} {
		if n, ok := counts[absent]; ok {
			t.Errorf("P1=%s count = %d, want the group ABSENT (the FTdx10 chart populates P1 01-04 only)", absent, n)
		}
	}
}

// TestEXGroups_EighteenDistinctSubgroups checks the generated table's own shape:
// 18 entries, no duplicate (P1,P2) key. A duplicated key would silently make one
// group's widths unreachable through buildEXDefaults' overwrite.
func TestEXGroups_EighteenDistinctSubgroups(t *testing.T) {
	if len(exGroups) != 18 {
		t.Fatalf("len(exGroups) = %d, want 18", len(exGroups))
	}
	seen := map[[2]string]bool{}
	for _, g := range exGroups {
		key := [2]string{g.p1, g.p2}
		if seen[key] {
			t.Errorf("duplicate (P1,P2) subgroup %v in exGroups", key)
		}
		seen[key] = true
		if len(g.widths) == 0 {
			t.Errorf("subgroup %v has no items", key)
		}
	}
	if len(seen) != 18 {
		t.Errorf("distinct (P1,P2) subgroups = %d, want 18", len(seen))
	}
}

// TestEXDefaults_AddressesAreContiguousFromP3One pins the property the compact
// widths string encodes: within every subgroup the P3 items run 01, 02, 03 … with
// no gaps, because the string's index IS the item index. gen/groupRows refuses to
// emit a gap; this is the same property asserted from the expanded map, so a
// hand-edit of the generated file that inserted or dropped a token is caught on
// this side too.
func TestEXDefaults_AddressesAreContiguousFromP3One(t *testing.T) {
	d := EXDefaults()
	perGroup := map[string]map[int]bool{}
	for addr := range d {
		if len(addr) != 6 {
			t.Errorf("address %q is not six digits", addr)
			continue
		}
		key := addr[:4]
		p3 := int(addr[4]-'0')*10 + int(addr[5]-'0')
		if perGroup[key] == nil {
			perGroup[key] = map[int]bool{}
		}
		perGroup[key][p3] = true
	}
	if len(perGroup) != 18 {
		t.Fatalf("EXDefaults() spans %d (P1,P2) subgroups, want 18", len(perGroup))
	}
	for key, items := range perGroup {
		for i := 1; i <= len(items); i++ {
			if !items[i] {
				t.Errorf("subgroup %s has %d items but no P3=%02d — the P3 run is not contiguous from 01", key, len(items), i)
			}
		}
	}
}

// TestEXDefaults_WidthShape pins what the fake ANSWERS with, per width class: a
// numeric item is all-'0' of its own width, and the one text item is 12 spaces.
// Both values are INVENTED — doc.go register entry 4 — and this test is where the
// invention is written down as behaviour rather than as prose.
func TestEXDefaults_WidthShape(t *testing.T) {
	d := EXDefaults()
	textCount := 0
	for addr, p4 := range d {
		switch len(p4) {
		case 12:
			if p4 != strings.Repeat(" ", 12) {
				t.Errorf("EXDefaults()[%q] = %q, want 12 spaces", addr, p4)
			}
			if addr != "040101" {
				t.Errorf("EXDefaults()[%q] is 12 bytes wide; the chart's only text item is 040101 (MY CALL.)", addr)
			}
			textCount++
		case 1, 2, 3, 4:
			for i := 0; i < len(p4); i++ {
				if p4[i] != '0' {
					t.Errorf("EXDefaults()[%q] = %q, want all-'0'", addr, p4)
					break
				}
			}
		default:
			t.Errorf("EXDefaults()[%q] = %q (len %d), want length 1-4 all-'0' or 12 spaces", addr, p4, len(p4))
		}
	}
	if textCount != 1 {
		t.Errorf("text entry count = %d, want 1 (MY CALL. at 040101 — where the FT-710's chart has six)", textCount)
	}
}

// TestEXDefaults_Independent: every call must return a FRESH map, so mutating one
// result can never reach another call's, nor a constructed Radio's own state.
// New seeds each Radio from EXDefaults(), so a shared map would make every fake
// in a test binary share one menu.
func TestEXDefaults_Independent(t *testing.T) {
	first := EXDefaults()
	first["010101"] = "999"
	delete(first, "010104")

	second := EXDefaults()
	if got := second["010101"]; got != "000" {
		t.Errorf("EXDefaults()[\"010101\"] on a fresh call = %q, want \"000\" (unaffected by mutating an earlier copy)", got)
	}
	if _, ok := second["010104"]; !ok {
		t.Error("EXDefaults() lost address 010104 after it was deleted from an earlier copy")
	}

	_, conn := newTestRadio(t)
	writeFrame(t, conn, "EX010101;")
	if got, want := mustReadFrame(t, conn), "EX010101000;"; got != want {
		t.Errorf("EX010101; after mutating an EXDefaults() copy -> %q, want %q", got, want)
	}
}

// --- WithEXSetting / WithEXUnavailable / EXState ---

func TestWithEXSetting_OverlayAndEXState(t *testing.T) {
	r, conn := newTestRadio(t, WithEXSetting("030108", "3"))

	got, ok := r.EXState("030108")
	if !ok || got != "3" {
		t.Fatalf("EXState(\"030108\") = %q, %v, want \"3\", true", got, ok)
	}

	writeFrame(t, conn, "EX030108;")
	if got, want := mustReadFrame(t, conn), "EX0301083;"; got != want {
		t.Errorf("EX030108; after WithEXSetting -> %q, want %q", got, want)
	}

	// The overlay is verbatim and unvalidated, and it does not disturb its
	// neighbours.
	if got, ok := r.EXState("030109"); !ok || got != "0" {
		t.Errorf("EXState(\"030109\") = %q, %v, want the untouched default \"0\", true", got, ok)
	}
}

// TestWithEXSetting_AcceptsAnAddressTheInventoryDoesNotHave pins the option's
// deliberate looseness: it does not consult exGroups, so a test can make an
// out-of-inventory address answerable without editing the projection of
// transcription B that the cross-check depends on.
func TestWithEXSetting_AcceptsAnAddressTheInventoryDoesNotHave(t *testing.T) {
	if _, ok := EXDefaults()["050101"]; ok {
		t.Fatal("test fixture error: 050101 is in the inventory, so this test proves nothing")
	}
	r, conn := newTestRadio(t, WithEXSetting("050101", "7"))
	if got, ok := r.EXState("050101"); !ok || got != "7" {
		t.Fatalf("EXState(\"050101\") = %q, %v, want \"7\", true", got, ok)
	}
	writeFrame(t, conn, "EX050101;")
	if got, want := mustReadFrame(t, conn), "EX0501017;"; got != want {
		t.Errorf("EX050101; after WithEXSetting on an out-of-inventory address -> %q, want %q", got, want)
	}
}

// TestWithEXUnavailable_ForcesRejectionForAKnownAddress: removing a map entry
// makes a KNOWN, otherwise-valid address answer exactly as an out-of-inventory
// one does. This is the seam a driver-level "this setting is unavailable" test
// needs, and it introduces no new assumed behaviour — it triggers the fake's
// existing documented "?;".
func TestWithEXUnavailable_ForcesRejectionForAKnownAddress(t *testing.T) {
	if _, ok := EXDefaults()["010101"]; !ok {
		t.Fatal("test fixture error: 010101 is not in the inventory, so this test proves nothing")
	}
	r, conn := newTestRadio(t, WithEXUnavailable("010101"))

	if p4, ok := r.EXState("010101"); ok {
		t.Errorf("EXState(\"010101\") = %q, true after WithEXUnavailable; want absent", p4)
	}
	writeFrame(t, conn, "EX010101;")
	if got, want := mustReadFrame(t, conn), "?;"; got != want {
		t.Errorf("EX010101; after WithEXUnavailable -> %q, want %q", got, want)
	}

	// Its neighbour still answers, so the option removed one entry and not the
	// table.
	writeFrame(t, conn, "EX010102;")
	if got, want := mustReadFrame(t, conn), "EX010102000;"; got != want {
		t.Errorf("EX010102; (neighbour of the removed address) -> %q, want %q", got, want)
	}
}

// TestWithEXOptions_ComposeInOrder: the options are overlays applied in the
// order given, so the LAST one wins — the semantics WithSlot already has, and
// the reason both option doc comments say "applied to whatever exSettings
// already holds".
func TestWithEXOptions_ComposeInOrder(t *testing.T) {
	r, _ := newTestRadio(t, WithEXSetting("010108", "1"), WithEXUnavailable("010108"))
	if p4, ok := r.EXState("010108"); ok {
		t.Errorf("EXState(\"010108\") = %q, true; want absent (WithEXUnavailable came last)", p4)
	}

	r2, conn2 := newTestRadio(t, WithEXUnavailable("010108"), WithEXSetting("010108", "1"))
	if got, ok := r2.EXState("010108"); !ok || got != "1" {
		t.Errorf("EXState(\"010108\") = %q, %v, want \"1\", true (WithEXSetting came last)", got, ok)
	}
	writeFrame(t, conn2, "EX010108;")
	if got, want := mustReadFrame(t, conn2), "EX0101081;"; got != want {
		t.Errorf("EX010108; -> %q, want %q", got, want)
	}
}

// TestEXState_ConcurrentWithReads polls EXState from one goroutine while the
// main test goroutine drives EX read exchanges through Port() — the EX
// equivalent of exercising SlotState concurrently with command processing (see
// the Radio doc comment: safe for concurrent use, run tests with -race). No
// *testing.T call is made from the background goroutine; its only job is to give
// -race something to catch if exSettings access were ever unguarded.
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
				_, _ = r.EXState("010101")
			}
		}
	}()

	for i := 0; i < 20; i++ {
		writeFrame(t, conn, "EX010101;")
		if got, want := mustReadFrame(t, conn), "EX010101000;"; got != want {
			t.Fatalf("exchange %d: EX010101; -> %q, want %q", i, got, want)
		}
	}
	close(stop)
	wg.Wait()
}
