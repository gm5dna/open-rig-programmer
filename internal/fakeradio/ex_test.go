// SPDX-License-Identifier: GPL-3.0-or-later

package fakeradio

import (
	"strings"
	"sync"
	"testing"
)

// This file's exchange tests recompute every expected reply byte string as
// a literal, directly from the protocol reference's "EX" grammar (never by
// calling fakeradio's own builders) — same independence discipline as
// fakeradio_test.go's golden-exchange tests.

// --- Reads: known address, default state ---

func TestEXRead_KnownAddressDefault(t *testing.T) {
	// A text item's default is 12 spaces; its answer is 21 bytes:
	// "EX"(2) + addr(6) + P4(12) + ";"(1).
	textWant := "EX040101" + strings.Repeat(" ", 12) + ";"
	if len(textWant) != 21 {
		t.Fatalf("test fixture error: textWant %q has length %d, want 21", textWant, len(textWant))
	}

	tests := []struct {
		name string
		req  string
		want string
	}{
		{"1-digit width default", "EX030105;", "EX0301050;"},
		{"4-digit width default", "EX010104;", "EX0101040000;"},
		{"text default, 12 spaces", "EX040101;", textWant},
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

func TestEXRead_UnknownAddress(t *testing.T) {
	for _, req := range []string{"EX050101;", "EX010199;", "EX000000;"} {
		t.Run(req, func(t *testing.T) {
			_, conn := newTestRadio(t)
			writeFrame(t, conn, req)
			if got, want := mustReadFrame(t, conn), "?;"; got != want {
				t.Errorf("%s -> %q, want %q", req, got, want)
			}
		})
	}
}

// --- Malformed bodies ---

func TestEXRead_MalformedBody(t *testing.T) {
	tests := []struct{ name, req string }{
		{"5-digit body", "EX01010;"},
		{"7-digit body", "EX0101011;"},
		{"non-digit body", "EX01A101;"},
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

// TestEXRead_CaseInsensitiveCommandName mirrors the existing parser's
// convention (handleFrame's toUpperASCII on the two command-name bytes
// only): command NAMES are case-insensitive; field values are not. An EX
// read body is pure digits, so there is no field-value case question here
// — this test pins the command-name half of that convention for EX.
func TestEXRead_CaseInsensitiveCommandName(t *testing.T) {
	_, conn := newTestRadio(t)
	writeFrame(t, conn, "ex030105;")
	want := "EX0301050;"
	if got := mustReadFrame(t, conn); got != want {
		t.Errorf("ex030105; (lowercase cmd) -> %q, want %q", got, want)
	}
}

// --- Set-shaped bodies (task e) ---

// TestHandleEX_AcceptsSetForAnyInventoryAddressAtCharacterisedWidth pins
// the headline behaviour task (e) adds: once WithEXSetWidth gives an
// inventory address a characterised Set width, a well-formed Set-shaped
// body at exactly that width is accepted — no reply at all
// (fire-and-forget, mirroring handleMW's success case) — and the stored
// value changes, so the paired read returns it.
//
// "030105" (CAT-1 RATE) is deliberately used for the second case: that
// address is one of core/cat's permanently-denied ones
// (core/cat/exdenylist.go's exCATLinkDenied) — this fake has no way to
// know that (doc.go, THE HARD RULE: fakeradio never imports core/cat) and
// does not consult it even by analogy. Accepting the Set here is the
// proof: the fake models wire behaviour only, never core/cat's project
// policy.
func TestHandleEX_AcceptsSetForAnyInventoryAddressAtCharacterisedWidth(t *testing.T) {
	tests := []struct {
		name  string
		addr  string
		width int
		value string
	}{
		{"ordinary admitted address", "010101", 3, "007"},
		{"denied-in-core/cat address (fake does not consult it)", "030105", 1, "9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, conn := newTestRadio(t, WithEXSetWidth(tt.addr, tt.width))

			writeFrame(t, conn, "EX"+tt.addr+tt.value+";")
			assertNoReply(t, conn)

			if got, ok := r.EXState(tt.addr); !ok || got != tt.value {
				t.Errorf("EXState(%q) after accepted Set = %q, %v, want %q, true", tt.addr, got, ok, tt.value)
			}

			writeFrame(t, conn, "EX"+tt.addr+";")
			want := "EX" + tt.addr + tt.value + ";"
			if got := mustReadFrame(t, conn); got != want {
				t.Errorf("EX%s; after Set -> %q, want %q (the paired read must return the just-Set value)", tt.addr, got, want)
			}
		})
	}
}

// TestHandleEX_RefusesSetWrongWidth covers every way a Set-shaped body's
// payload can fail to match exSetWidths[addr]: too short, too long, and
// — the default state of every address today, since exSetWidths is empty
// until Session W runs (doc.go register item 24) — uncharacterised, no
// entry at all. Every case refuses "?;" with state unchanged.
func TestHandleEX_RefusesSetWrongWidth(t *testing.T) {
	tests := []struct {
		name    string
		opts    []Option
		wireSet string // full Set frame's body after "EX"
	}{
		{"uncharacterised: no exSetWidths entry at all", nil, "0301051"},
		{"too short for the characterised width", []Option{WithEXSetWidth("010101", 3)}, "01010107"},
		{"too long for the characterised width", []Option{WithEXSetWidth("010101", 3)}, "0101010007"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, conn := newTestRadio(t, tt.opts...)
			addr := tt.wireSet[:exAddrLen]
			before, beforeOK := r.EXState(addr)

			writeFrame(t, conn, "EX"+tt.wireSet+";")
			if got, want := mustReadFrame(t, conn), "?;"; got != want {
				t.Errorf("EX%s; -> %q, want %q", tt.wireSet, got, want)
			}
			if got, ok := r.EXState(addr); ok != beforeOK || got != before {
				t.Errorf("EXState(%q) after refused Set = %q, %v, want unchanged %q, %v", addr, got, ok, before, beforeOK)
			}
		})
	}
}

// TestHandleEX_RefusesSetOutOfInventory proves the inventory-membership
// check (the same one the Read form uses) runs, and wins, ahead of the
// width check: injecting a Set width for an out-of-inventory address
// (WithEXSetWidth sets it unconditionally, with no membership check of
// its own) still refuses "?;", because handleEX never reaches
// exSetWidths for an address r.exSettings does not have at all.
func TestHandleEX_RefusesSetOutOfInventory(t *testing.T) {
	const addr = "050101" // no P1=05 group in Table 2 — doc.go register item 23/25
	r, conn := newTestRadio(t, WithEXSetWidth(addr, 1))

	writeFrame(t, conn, "EX"+addr+"1;")
	if got, want := mustReadFrame(t, conn), "?;"; got != want {
		t.Errorf("EX%s1; -> %q, want %q", addr, got, want)
	}
	if _, ok := r.EXState(addr); ok {
		t.Errorf("EXState(%q) = ok, want absent (out-of-inventory address must never become answerable via Set)", addr)
	}
}

// --- EXDefaults(): structural self-checks ---

// TestEXDefaults_CountsPerGroup is this task's independent per-P1 recount
// (see the task-29 report for the manual line ranges each group derives
// from): 94/31/65/16/90, total 296. There is no P1=05 in Table 2 — see
// doc.go register, the P1=06-not-05 anomaly.
func TestEXDefaults_CountsPerGroup(t *testing.T) {
	d := EXDefaults()
	if len(d) != 296 {
		t.Fatalf("len(EXDefaults()) = %d, want 296", len(d))
	}
	counts := map[string]int{}
	for addr := range d {
		counts[addr[:2]]++
	}
	want := map[string]int{"01": 94, "02": 31, "03": 65, "04": 16, "06": 90}
	for p1, wantN := range want {
		if counts[p1] != wantN {
			t.Errorf("P1=%s count = %d, want %d", p1, counts[p1], wantN)
		}
	}
	if n, ok := counts["05"]; ok {
		t.Errorf("P1=05 count = %d, want absent (no P1=05 in Table 2 — see doc.go register)", n)
	}
}

func TestEXDefaults_GroupCount21(t *testing.T) {
	if len(exGroups) != 21 {
		t.Fatalf("len(exGroups) = %d, want 21", len(exGroups))
	}
	seen := map[[2]string]bool{}
	for _, g := range exGroups {
		key := [2]string{g.p1, g.p2}
		if seen[key] {
			t.Errorf("duplicate (P1,P2) group %v in exGroups", key)
		}
		seen[key] = true
	}
	if len(seen) != 21 {
		t.Errorf("distinct (P1,P2) groups = %d, want 21", len(seen))
	}
}

func TestEXDefaults_WidthShape(t *testing.T) {
	d := EXDefaults()
	textCount := 0
	for addr, p4 := range d {
		switch len(p4) {
		case 12:
			if p4 != strings.Repeat(" ", 12) {
				t.Errorf("EXDefaults()[%q] = %q, want 12 spaces", addr, p4)
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
	if textCount != 6 {
		t.Errorf("text entry count = %d, want 6 (one MY CALL + five PRESET NAME — see doc.go register)", textCount)
	}
}

// --- WithEXSetting / EXState ---

func TestWithEXSetting_OverlayAndEXState(t *testing.T) {
	r, conn := newTestRadio(t, WithEXSetting("030105", "3"))

	got, ok := r.EXState("030105")
	if !ok || got != "3" {
		t.Fatalf("EXState(\"030105\") = %q, %v, want \"3\", true", got, ok)
	}

	writeFrame(t, conn, "EX030105;")
	want := "EX0301053;"
	if got := mustReadFrame(t, conn); got != want {
		t.Errorf("EX030105; after WithEXSetting -> %q, want %q", got, want)
	}

	// EXDefaults() must return a FRESH copy every call: mutating one
	// returned map must never reach a constructed Radio's own state, nor
	// leak into a later EXDefaults() call.
	d := EXDefaults()
	d["030105"] = "9"
	if got, _ := r.EXState("030105"); got != "3" {
		t.Errorf("EXState(\"030105\") after mutating an EXDefaults() copy = %q, want unaffected \"3\"", got)
	}
	d2 := EXDefaults()
	if d2["030105"] != "0" {
		t.Errorf("EXDefaults()[\"030105\"] on a fresh call = %q, want default \"0\" (unaffected by the earlier mutation)", d2["030105"])
	}
}

// TestEXState_ConcurrentWithReads polls EXState from one goroutine while
// the main test goroutine drives EX read exchanges through Port() — the
// EX equivalent of exercising SlotState concurrently with command
// processing (see fakeradio.go's Radio doc comment: "safe for concurrent
// use ... run tests with -race"). No *testing.T call is made from the
// background goroutine (Fatalf et al. must run only on the test
// goroutine); the background goroutine's only job is to give -race
// something to catch if exSettings access were ever unguarded.
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
				r.EXState("030105")
			}
		}
	}()

	for i := 0; i < 100; i++ {
		writeFrame(t, conn, "EX030105;")
		if got := mustReadFrame(t, conn); got != "EX0301050;" {
			t.Fatalf("EX030105; (iteration %d) -> %q, want \"EX0301050;\"", i, got)
		}
	}
	close(stop)
	wg.Wait()
}

// --- M8c hardware override layer (task 47) ---

// TestEXRead_ToneFreqUsesHardwareOverride pins the M8c correction:
// 01 03 21 (RADIO SETTING -> MODE FM -> TONE FREQ) answered a THREE-byte
// P4 in the M8c sweeps ("EX010321012;", one UK radio, firmware V01-12 —
// docs/hardware-notes.md), not the two bytes Table 2's Digits column
// prints. exGroups keeps the manual's 2 — it is this package's
// independent transcription of that table, and core/transport's
// cross-check compares it against core/cat's independent transcription —
// so the observed width is supplied by exHardwareOverrides instead.
func TestEXRead_ToneFreqUsesHardwareOverride(t *testing.T) {
	_, conn := newTestRadio(t)
	writeFrame(t, conn, "EX010321;")
	got := mustReadFrame(t, conn)
	if want := len("EX010321") + 3 + len(";"); len(got) != want {
		t.Errorf("answer %q is %d bytes, want %d (address + 3-byte P4 + ';')", got, len(got), want)
	}
}

// TestEXRead_SignedItemsCarryAnExplicitSign pins the M8c observation that,
// in those sweeps of one radio, 26 addresses answered with an explicit
// leading '+'/'-' counted inside the manual's own width. The fake's VALUE is its own synthetic placeholder
// (register item 21); only the shape and width are hardware-derived.
func TestEXRead_SignedItemsCarryAnExplicitSign(t *testing.T) {
	for _, addr := range []string{"010101", "020103", "030318"} {
		t.Run(addr, func(t *testing.T) {
			_, conn := newTestRadio(t)
			writeFrame(t, conn, "EX"+addr+";")
			got := mustReadFrame(t, conn)
			p4 := got[len("EX")+exAddrLen : len(got)-1]
			if p4 == "" || (p4[0] != '+' && p4[0] != '-') {
				t.Errorf("EX%s answered P4 %q, want an explicit leading sign", addr, p4)
			}
			if len(p4) != 3 {
				t.Errorf("EX%s answered a %d-byte P4, want 3", addr, len(p4))
			}
		})
	}
}

// TestEXRuntimeDefaults_OverlaysWithoutDisturbingTheManualTable proves the
// two layers stay separate: EXDefaults() keeps reporting this package's
// manual transcription (which core/transport compares against core/cat's),
// while EXRuntimeDefaults() reports what the fake actually answers.
func TestEXRuntimeDefaults_OverlaysWithoutDisturbingTheManualTable(t *testing.T) {
	manual := EXDefaults()
	runtime := EXRuntimeDefaults()

	if len(manual) != len(runtime) {
		t.Fatalf("manual table has %d addresses, runtime table %d — the overlay must not add or remove any", len(manual), len(runtime))
	}
	if got := len(manual["010321"]); got != 2 {
		t.Errorf("EXDefaults()[010321] is %d bytes, want the manual's 2 — the overlay has leaked into the manual transcription", got)
	}
	if got := len(runtime["010321"]); got != 3 {
		t.Errorf("EXRuntimeDefaults()[010321] is %d bytes, want the hardware-observed 3", got)
	}

	differing := 0
	for addr, p4 := range manual {
		if runtime[addr] != p4 {
			differing++
		}
	}
	if want := len(exHardwareOverrides); differing != want {
		t.Errorf("%d addresses differ between the manual and runtime tables, want %d (one per override)", differing, want)
	}
}

// TestEXRuntimeDefaults_Independent proves each call returns its own map,
// matching EXDefaults' existing contract.
func TestEXRuntimeDefaults_Independent(t *testing.T) {
	first := EXRuntimeDefaults()
	first["010101"] = "MUTATED"
	if second := EXRuntimeDefaults(); second["010101"] == "MUTATED" {
		t.Error("mutating one EXRuntimeDefaults() result affected another call's result")
	}
}
