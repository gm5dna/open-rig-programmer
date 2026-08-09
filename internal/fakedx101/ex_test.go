// SPDX-License-Identifier: GPL-3.0-or-later

package fakedx101

import (
	"strings"
	"sync"
	"testing"
)

// This file's exchange tests recompute every expected reply byte string as a
// LITERAL, from the EX grammar and from transcription B's own digits column —
// never by calling this package's own builders, and never from the generated
// table. The addresses and widths below were read off the committed CSV row by
// row; that is the same independence discipline the rest of this package's tests
// keep (see fakedx101_test.go's header), and it is what makes a mistake in the
// projection visible here rather than agreed with.
//
// The B rows these literals come from (line numbers count the artefact's own
// '#' provenance block, as an editor does):
//
//	010101  AGC FAST DELAY  digits 4   (line 44)
//	010102  AGC MID DELAY   digits 4   (line 45)
//	010104  LCUT FREQ       digits 2   (line 47)
//	010105  LCUT SLOP       digits 1   (line 48)
//	010109  SSB OUT LEVEL   digits 3   (line 52)
//	040101  MY CALL.        digits 12  (line 223 — the chart's ONE text item)
//
// EVERY TEST BELOW RUNS AGAINST BOTH MODELS where the answer could conceivably
// differ, because this package exists partly to make "the D and the MP differ in
// the ID answer and nowhere else" testable, and the menu surface is the largest
// place that claim could quietly fail (ex.go, "one inventory, two radios").

// exModels is the pair of constructors every model-independent EX claim is run
// against. A test that used only NewD would leave the MP's menu surface
// completely unexercised, which is exactly the gap the shared inventory could
// hide in.
var exModels = []struct {
	name  string
	build ctor
}{
	{"FTDX101D", NewD},
	{"FTDX101MP", NewMP},
}

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
		{"1-digit width default (LCUT SLOP)", "EX010105;", "EX0101050;"},
		{"2-digit width default (LCUT FREQ)", "EX010104;", "EX01010400;"},
		{"3-digit width default (SSB OUT LEVEL)", "EX010109;", "EX010109000;"},
		{"4-digit width default (AGC FAST DELAY)", "EX010101;", "EX0101010000;"},
		{"text default, 12 spaces (MY CALL.)", "EX040101;", textWant},
	}
	for _, m := range exModels {
		t.Run(m.name, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					_, conn := newTestRadioFrom(t, m.build)
					writeFrame(t, conn, tt.req)
					if got := mustReadFrame(t, conn); got != tt.want {
						t.Errorf("%s -> %q, want %q", tt.req, got, tt.want)
					}
				})
			}
		})
	}
}

// --- Reads: valid-shape but unknown address ---

// TestEXRead_UnknownAddress covers doc.go register entry 17's first half: a
// syntactically valid six-digit address this fake holds no entry for answers
// "?;", the protocol's single unattributed NAK. ASSUMED on both models — no
// FTDX101D and no FTDX101MP has ever been asked anything, and on these radios
// the rejection convention ITSELF is unattested besides (register entry 16).
//
// Membership comes from the chart's own rows, via the generated inventory, so
// 05xxxx answers "?;" because nothing was transcribed for it. LIKE the FTdx10's,
// this chart carries a P1 anomaly: the EX grammar block states "P1 : 01 - 05"
// (layout 700) while Table 2 populates 01-04 only, recorded UNRESOLVED in
// core/cat/ftdx101/doc.go because — unlike the FT-710's — it cannot be put to
// hardware. So the first case below is DOUBLY unknown: out of the chart's
// inventory, and inside the grammar block's declared range. This test pins what
// the FAKE does, which is answer "?;" because it holds no entry; it is not
// evidence about either bound.
func TestEXRead_UnknownAddress(t *testing.T) {
	tests := []struct{ name, req string }{
		{"no P1=05 group in the chart, though the grammar block declares P1 to 05", "EX050101;"},
		{"no P1=06 group either (the FT-710 has one; this chart does not)", "EX060101;"},
		{"P3 beyond its group's item count (01/01 has 14)", "EX010115;"},
		{"P3 beyond its group's item count (04/03 has 2)", "EX040303;"},
		{"P3 beyond a one-item group (02/03 has 1)", "EX020302;"},
		{"P2 beyond its P1's subgroup count (01 has 07)", "EX010801;"},
		{"all zeros", "EX000000;"},
	}
	for _, m := range exModels {
		t.Run(m.name, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					_, conn := newTestRadioFrom(t, m.build)
					writeFrame(t, conn, tt.req)
					if got, want := mustReadFrame(t, conn), "?;"; got != want {
						t.Errorf("%s -> %q, want %q", tt.req, got, want)
					}
				})
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

// TestEXRead_LowerCaseCommandAccepted pins the DIVERGENCE from
// internal/fakedx10, which refuses a lower-case command name. It is a MANUAL
// FACT for these radios and not an inherited leniency: "A command consists of 2
// alphabetical characters. You may use either lower or upper case characters."
// (layout 204-205). EX is checked because it is the newest command arm, and the
// one a later tightening would most plausibly forget.
//
// The FIELD half stays case-sensitive (register entry 12), but an EX body is all
// digits, so this command has no field case to test — which is itself worth
// stating, since its absence here is not an omission.
func TestEXRead_LowerCaseCommandAccepted(t *testing.T) {
	for _, m := range exModels {
		t.Run(m.name, func(t *testing.T) {
			_, conn := newTestRadioFrom(t, m.build)
			writeFrame(t, conn, "ex010101;")
			if got, want := mustReadFrame(t, conn), "EX0101010000;"; got != want {
				t.Errorf("ex010101; (lower-case command) -> %q, want %q (layout 204-205)", got, want)
			}
		})
	}
}

// --- Set-shaped bodies: not modelled ---

// TestEXSetShaped_NotModelled is register entry 17's second half. "EX0101011;"
// is shaped like a real Set frame (address "010101" + a P4 digit "1"), and
// handleEX rejects any non-six-digit body uniformly, so it draws "?;" — and,
// critically, WITHOUT applying the write. This is a deliberate modelling gap (the
// manual documents a Set form), not a claim that a real FTdx101 refuses EX Set;
// the menu surface is READ-ONLY for v1.x by the M8d decision of 25/07/2026.
func TestEXSetShaped_NotModelled(t *testing.T) {
	r, conn := newTestRadio(t)
	writeFrame(t, conn, "EX0101011;")
	if got, want := mustReadFrame(t, conn), "?;"; got != want {
		t.Errorf("EX0101011; (set-shaped) -> %q, want %q", got, want)
	}
	if got, ok := r.EXState("010101"); !ok || got != "0000" {
		t.Errorf("EXState(\"010101\") after a rejected set-shaped body = %q, %v, want \"0000\", true (state unchanged)", got, ok)
	}
}

// --- EXDefaults(): structural self-checks ---

// TestEXDefaults_CountsPerGroup is this package's independent recount of the
// committed transcription B: 193 items, 86/31/62/14 across P1 01-04, and NO
// P1=05 or P1=06 group (the FT-710's chart has a P1=06 EXTENSION SETTING block
// of 90 items; this one has none).
//
// The numbers are literals, read off the CSV's group boundaries, not derived
// from exGroups — a count computed from the table it is checking proves nothing.
// gen/main_test.go pins the same totals from the CSV side, and
// core/transport/ex_crosscheck_ftdx101_test.go binds them to the DIALECT's
// independently generated inventory; three derivations, one fact.
func TestEXDefaults_CountsPerGroup(t *testing.T) {
	d := EXDefaults()
	if len(d) != 193 {
		t.Fatalf("len(EXDefaults()) = %d, want 193", len(d))
	}
	counts := map[string]int{}
	for addr := range d {
		counts[addr[:2]]++
	}
	want := map[string]int{"01": 86, "02": 31, "03": 62, "04": 14}
	for p1, wantN := range want {
		if counts[p1] != wantN {
			t.Errorf("P1=%s count = %d, want %d", p1, counts[p1], wantN)
		}
	}
	for _, absent := range []string{"05", "06"} {
		if n, ok := counts[absent]; ok {
			t.Errorf("P1=%s count = %d, want the group ABSENT (the FTdx101 chart populates P1 01-04 only)", absent, n)
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
// newRadio seeds each Radio from EXDefaults(), so a shared map would make every
// fake in a test binary share one menu — and, worse here than in the
// single-model fakes, would make a D and an MP built in the same test share one.
func TestEXDefaults_Independent(t *testing.T) {
	first := EXDefaults()
	first["010101"] = "9999"
	delete(first, "010102")

	second := EXDefaults()
	if got := second["010101"]; got != "0000" {
		t.Errorf("EXDefaults()[\"010101\"] on a fresh call = %q, want \"0000\" (unaffected by mutating an earlier copy)", got)
	}
	if _, ok := second["010102"]; !ok {
		t.Error("EXDefaults() lost address 010102 after it was deleted from an earlier copy")
	}

	_, conn := newTestRadio(t)
	writeFrame(t, conn, "EX010101;")
	if got, want := mustReadFrame(t, conn), "EX0101010000;"; got != want {
		t.Errorf("EX010101; after mutating an EXDefaults() copy -> %q, want %q", got, want)
	}
}

// TestEXSettings_AreNotSharedBetweenRadios is the same independence seen from the
// other end, and it is the one this package needs that its single-model siblings
// do not: overlaying one model's menu must not reach the other's. A shared map
// would make WithEXUnavailable on a D silently disable the same item on every MP
// in the binary.
func TestEXSettings_AreNotSharedBetweenRadios(t *testing.T) {
	_, connD := newTestRadioFrom(t, NewD, WithEXUnavailable("010101"), WithEXSetting("010104", "99"))
	_, connMP := newTestRadioFrom(t, NewMP)

	if got, want := exchange(t, connD, "EX010101;"), "?;"; got != want {
		t.Errorf("D EX010101; -> %q, want %q", got, want)
	}
	if got, want := exchange(t, connMP, "EX010101;"), "EX0101010000;"; got != want {
		t.Errorf("MP EX010101; -> %q, want %q — the D's WithEXUnavailable reached the MP's menu", got, want)
	}
	if got, want := exchange(t, connMP, "EX010104;"), "EX01010400;"; got != want {
		t.Errorf("MP EX010104; -> %q, want %q — the D's WithEXSetting reached the MP's menu", got, want)
	}
}

// --- WithEXSetting / WithEXUnavailable / EXState ---

func TestWithEXSetting_OverlayAndEXState(t *testing.T) {
	r, conn := newTestRadio(t, WithEXSetting("010105", "3"))

	got, ok := r.EXState("010105")
	if !ok || got != "3" {
		t.Fatalf("EXState(\"010105\") = %q, %v, want \"3\", true", got, ok)
	}

	writeFrame(t, conn, "EX010105;")
	if got, want := mustReadFrame(t, conn), "EX0101053;"; got != want {
		t.Errorf("EX010105; after WithEXSetting -> %q, want %q", got, want)
	}

	// The overlay is verbatim and unvalidated, and it does not disturb its
	// neighbours.
	if got, ok := r.EXState("010104"); !ok || got != "00" {
		t.Errorf("EXState(\"010104\") = %q, %v, want the untouched default \"00\", true", got, ok)
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
	if got, want := mustReadFrame(t, conn), "EX0101020000;"; got != want {
		t.Errorf("EX010102; (neighbour of the removed address) -> %q, want %q", got, want)
	}
}

// TestWithEXOptions_ComposeInOrder: the options are overlays applied in the
// order given, so the LAST one wins — the semantics WithSlot already has, and
// the reason both option doc comments say "applied to whatever exSettings
// already holds".
func TestWithEXOptions_ComposeInOrder(t *testing.T) {
	r, _ := newTestRadio(t, WithEXSetting("010105", "1"), WithEXUnavailable("010105"))
	if p4, ok := r.EXState("010105"); ok {
		t.Errorf("EXState(\"010105\") = %q, true; want absent (WithEXUnavailable came last)", p4)
	}

	r2, conn2 := newTestRadio(t, WithEXUnavailable("010105"), WithEXSetting("010105", "1"))
	if got, ok := r2.EXState("010105"); !ok || got != "1" {
		t.Errorf("EXState(\"010105\") = %q, %v, want \"1\", true (WithEXSetting came last)", got, ok)
	}
	writeFrame(t, conn2, "EX010105;")
	if got, want := mustReadFrame(t, conn2), "EX0101051;"; got != want {
		t.Errorf("EX010105; -> %q, want %q", got, want)
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
		if got, want := mustReadFrame(t, conn), "EX0101010000;"; got != want {
			t.Fatalf("exchange %d: EX010101; -> %q, want %q", i, got, want)
		}
	}
	close(stop)
	wg.Wait()
}

// TestEXAnswersAreIdenticalOnBothModels sweeps the WHOLE inventory at a D and at
// an MP and requires byte-identical replies. It is the package-local half of the
// cross-check's ID-divergence leg
// (core/transport/ex_crosscheck_ftdx101_test.go), and it is here as well as
// there because the two ask different questions: that one binds the fake to the
// DIALECT, this one binds the fake's two constructors to each other with no
// dialect involved at all.
//
// The addresses are taken from EXDefaults() rather than from a literal list —
// deliberately, because the point is COVERAGE of the whole surface, and the
// counts that surface must have are pinned by the literal-bearing tests above.
func TestEXAnswersAreIdenticalOnBothModels(t *testing.T) {
	addrs := EXDefaults()
	if len(addrs) == 0 {
		t.Fatal("EXDefaults() is empty — this sweep would pass vacuously")
	}

	_, connD := newTestRadioFrom(t, NewD)
	_, connMP := newTestRadioFrom(t, NewMP)

	for addr := range addrs {
		send := "EX" + addr + ";"
		gotD := exchange(t, connD, send)
		gotMP := exchange(t, connMP, send)
		if gotD != gotMP {
			t.Errorf("%q: D answered %q, MP answered %q — the two models share one MENU chart and must answer identically", send, gotD, gotMP)
		}
		if gotD == "?;" {
			t.Errorf("%q: both models refused an address EXDefaults() holds", send)
		}
	}
}
