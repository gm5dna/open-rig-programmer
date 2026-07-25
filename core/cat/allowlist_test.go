// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"strings"
	"testing"
)

// --- AllowedCommand: adversarial rejection table ---

func TestAllowedCommand_RejectTable(t *testing.T) {
	tests := []struct {
		name  string
		frame []byte
	}{
		{"non-allowlisted command TX1;", []byte("TX1;")},
		{"non-allowlisted command FA014250000;", []byte("FA014250000;")},
		{"double frame MW...;MW...;", []byte("MW005014250000+000000210000;MW005014250000+000000210000;")},
		{"embedded ';' not at the end", []byte("MT0011X;Y;")},
		{"empty", []byte("")},
		{"nil", nil},
		{"no terminator", []byte("MC099")},
		{"only terminator", []byte(";")},
		{"allowlisted prefix but two terminators", []byte("AI0;;")},
		{"allowlisted prefix, terminator not at end", []byte("AI0;X")},
		{"lower case allowlisted-looking prefix", []byte("id0800;")},
		{"allowlisted prefix, embedded terminator mid-body then real one", []byte("MW;005014250000+000000210000;")},

		// --- strict grammar: bare 2-byte-prefix + trailing ';' is no
		// longer sufficient (Task 3 Fix 1) ---
		{"MW; alone: prefix allowlisted but nowhere near the 28-byte Set shape", []byte("MW;")},
		{"AI2; garbage state digit", []byte("AI2;")},
		{"AIX; garbage state byte", []byte("AIX;")},
		{"MR garbage slot, right length", []byte("MRXYZ;")},
		{"MR wrong length", []byte("MRXX;")},
		{"MW far too short for the 28-byte Set shape", []byte("MWGARBAGE;")},
		{"MT set with invalid slot", []byte("MTXYZ1TAG;")},
		{"MC garbage slot, right length", []byte("MCXYZ;")},

		// --- MW write-direction policy re-checked from raw wire bytes,
		// not just field shape (Task 3 Fix 1: "share the field-validation
		// logic ... rather than duplicating it") ---
		{
			"MW to slot 501: shape valid but Writable() fails (same slot rejected by BuildMWSet)",
			[]byte("MW501014250000+000000210000;"),
		},
		{
			"MW Kind/slot mismatch: memory slot 005 with PMS kind digit '5' (BuildMWSet would also reject this)",
			[]byte("MW005014250000+000000250000;"),
		},

		// --- MT: charset/policy re-checked byte-for-byte, not just
		// "one trailing ';'" (Task 3 Fix 1 problem statement) ---
		{
			"MT set with a hidden NUL after the display byte: single trailing ';' but rejected on tag charset, not injection via ';'",
			[]byte("MT0011A\x00TX1;"),
		},
		{
			"MT set to slot 501: policy-rejected exactly as BuildMTSet rejects it",
			[]byte("MT501140M;"),
		},

		// --- EX: reads only (Task 28 / M8a-3). Set and Answer share an
		// identical wire shape with Read's prefix but a longer body
		// (manual lines ~630-637) and are rejected outright. Written as a
		// phase restriction; made permanent for v1.x by the M8d
		// menu-write no-go (25/07/2026, docs/menu-write-decision.md). MT's
		// set/answer exception is the precedent any future reopening would
		// follow — see validEXRead's doc comment. ---
		{
			"EX010101000; answer/set-shaped (P4=\"000\", AF TREBLE GAIN's width) -- rejected outright, cf. MT's set/answer-shaped ACCEPT entry; shipped policy after the M8d no-go",
			[]byte("EX010101000;"),
		},
		{"EX050101; well-formed read shape but P1==05 anomaly address is not a Table 2 member", []byte("EX050101;")},
		{"EXAABBCC; non-digit address bytes", []byte("EXAABBCC;")},
		{"ex010101; lower case command prefix", []byte("ex010101;")},
		{"EX010101;EX010102; two frames concatenated", []byte("EX010101;EX010102;")},
		{"EX01010; 8 bytes -- one short of exReadLen (address field truncated)", []byte("EX01010;")},
		{
			"EX0101011; 10 bytes -- EXACTLY the minimal EX Set/Answer shape (exAnswerMinLen), one over exReadLen",
			[]byte("EX0101011;"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if AllowedCommand(tc.frame) {
				t.Errorf("AllowedCommand(%q) = true, want false", tc.frame)
			}
		})
	}
}

// TestAllowedCommand_RejectsGoldenAnswerFrames: AllowedCommand's job is to
// gate what may be WRITTEN to the radio. Every one of these golden vectors
// is a frame this package can PARSE as an inbound ANSWER, but NONE of them
// is a legal outbound command — ID has no "ID0800;" Set form, and MR has
// no Set/Answer-shaped Set form at all (only a 6-byte Read). Before this
// fix, AllowedCommand's 2-byte-prefix-plus-one-trailing-';' check accepted
// all of these (see the superseded TestAllowedCommand_AcceptsAllowlistedSingleFrames
// entries in git history).
func TestAllowedCommand_RejectsGoldenAnswerFrames(t *testing.T) {
	tests := []struct {
		name  string
		frame string
	}{
		{"G1 answer: ID0800;", "ID0800;"},
		{"G4: MR answer for M-01", "MR001007000000+000000110000;"},
		{"G6: MR answer for PMS P1L", "MRP1L001810000+000000150000;"},
		{"G7 shared layout: MR answer for M-99", "MR099052354000-012010411002;"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if AllowedCommand([]byte(tc.frame)) {
				t.Errorf("AllowedCommand(%q) = true, want false (inbound-only answer frame)", tc.frame)
			}
		})
	}
}

func TestAllowedCommand_AcceptsAllowlistedSingleFrames(t *testing.T) {
	tests := []struct {
		name  string
		frame []byte
	}{
		{"ID;", []byte("ID;")},
		{"AI;  (read form; no builder emits it yet, but it is legal grammar)", []byte("AI;")},
		{"AI0;", []byte("AI0;")},
		{"AI1;", []byte("AI1;")},
		{"MR007;", []byte("MR007;")},
		{"MR read for PMS slot", []byte("MRP1L;")},
		{"MR read for 5xx slot", []byte("MR501;")},
		{"MR read for EMG slot", []byte("MREMG;")},
		{"MW set G5 (this IS the Set frame, not an answer -- MW has no answer form)", []byte("MW005014250000+000000210000;")},
		{"MW set G7 (every field non-default)", []byte("MW099052354000-012010411002;")},
		{"MT001;", []byte("MT001;")},
		{"MT read for 5xx slot", []byte("MT501;")},
		{"MT set/answer-shaped G8 (Set and Answer share this exact wire form)", []byte("MT0011CALLING FREQ;")},
		{"MC;", []byte("MC;")},
		{"MC099;", []byte("MC099;")},
		{"MC set for 5xx slot", []byte("MC501;")},
		{"MC set for EMG slot", []byte("MCEMG;")},
		{"EX010101; EX read for (01,01,01) AF TREBLE GAIN", []byte("EX010101;")},
		{"EX060518; EX read for (06,05,18) RPTT SELECT", []byte("EX060518;")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if !AllowedCommand(tc.frame) {
				t.Errorf("AllowedCommand(%q) = false, want true", tc.frame)
			}
		})
	}
}

// TestAllowedCommand_PropertyEveryBuilderOutput: for EVERY builder in this
// package, its output must satisfy AllowedCommand. This is the property
// the brief asks for: AllowedCommand is the transport layer's last-line
// defence before writing to the wire, so nothing this package itself would
// ever try to send may be rejected by it.
func TestAllowedCommand_PropertyEveryBuilderOutput(t *testing.T) {
	memSlot := mustMemorySlot(t, 1)
	pmsSlot := mustPMSSlot(t, 1, false)
	sixtyM, err := SixtyMSlot(1)
	if err != nil {
		t.Fatal(err)
	}

	var outputs []Command

	outputs = append(outputs, BuildIDRead())
	outputs = append(outputs, BuildAISet(false))
	outputs = append(outputs, BuildAISet(true))

	if out, err := BuildMRRead(memSlot); err == nil {
		outputs = append(outputs, out)
	} else {
		t.Fatalf("BuildMRRead: %v", err)
	}
	if out, err := BuildMRRead(sixtyM); err == nil {
		outputs = append(outputs, out)
	} else {
		t.Fatalf("BuildMRRead(5xx): %v", err)
	}

	mw := MemoryData{
		Slot: memSlot, FreqHz: 14_250_000, Mode: ModeUSB, Kind: KindMemory,
		CTCSS: CTCSSOff, Shift: ShiftSimplex,
	}
	if out, err := BuildMWSet(mw); err == nil {
		outputs = append(outputs, out)
	} else {
		t.Fatalf("BuildMWSet: %v", err)
	}
	mwPMS := mw
	mwPMS.Slot = pmsSlot
	mwPMS.Kind = KindMemory // HW-CONFIRMED 2026-07-13: PMS writes carry KindMemory, not KindPMS
	if out, err := BuildMWSet(mwPMS); err == nil {
		outputs = append(outputs, out)
	} else {
		t.Fatalf("BuildMWSet(PMS): %v", err)
	}

	if out, err := BuildMTSet(memSlot, true, "CALLING FREQ"); err == nil {
		outputs = append(outputs, out)
	} else {
		t.Fatalf("BuildMTSet: %v", err)
	}
	if out, err := BuildMTSet(memSlot, false, ""); err == nil {
		outputs = append(outputs, out)
	} else {
		t.Fatalf("BuildMTSet(empty tag): %v", err)
	}
	if out, err := BuildMTRead(memSlot); err == nil {
		outputs = append(outputs, out)
	} else {
		t.Fatalf("BuildMTRead: %v", err)
	}
	if out, err := BuildMTRead(sixtyM); err == nil {
		outputs = append(outputs, out)
	} else {
		t.Fatalf("BuildMTRead(5xx): %v", err)
	}

	if out, err := BuildMCSet(memSlot); err == nil {
		outputs = append(outputs, out)
	} else {
		t.Fatalf("BuildMCSet: %v", err)
	}
	if out, err := BuildMCSet(sixtyM); err == nil {
		outputs = append(outputs, out)
	} else {
		t.Fatalf("BuildMCSet(5xx): %v", err)
	}
	outputs = append(outputs, BuildMCRead())

	if out, err := BuildEXRead(EXAddress{P1: 1, P2: 1, P3: 1}); err == nil {
		outputs = append(outputs, out)
	} else {
		t.Fatalf("BuildEXRead((01,01,01)): %v", err)
	}
	if out, err := BuildEXRead(EXAddress{P1: 6, P2: 5, P3: 18}); err == nil {
		outputs = append(outputs, out)
	} else {
		t.Fatalf("BuildEXRead((06,05,18)): %v", err)
	}

	for _, out := range outputs {
		if !AllowedCommand(out.Bytes()) {
			t.Errorf("AllowedCommand(%q) = false, want true (builder output)", out.Bytes())
		}
	}
}

// TestAllowedCommand_EXPropertyAll296Reads: every one of the 296 Table 2
// inventory addresses' BuildEXRead output must satisfy AllowedCommand — the
// EX analogue of TestAllowedCommand_PropertyEveryBuilderOutput, run at full
// inventory scale so no single address is missed.
func TestAllowedCommand_EXPropertyAll296Reads(t *testing.T) {
	items := EXItems()
	for _, it := range items {
		cmd, err := BuildEXRead(it.Addr)
		if err != nil {
			t.Fatalf("BuildEXRead(%v): unexpected error: %v", it.Addr, err)
		}
		if !AllowedCommand(cmd.Bytes()) {
			t.Errorf("AllowedCommand(%q) = false, want true (EX read for %v)", cmd.Bytes(), it.Addr)
		}
	}
	if len(items) != 296 {
		t.Fatalf("test fixture bug: EXItems() returned %d items, want 296", len(items))
	}
}

// TestAllowedCommand_EXAnswersRejectedOutboundAll296 proves the EX
// restriction (validEXRead's doc comment, allowlist.go) at inventory
// scale: for every one of the 296 Table 2 items, the EX Set/Answer-shaped
// frame -- "EX" + address + a width-correct P4 body (all '0' for numeric
// items, 12 spaces for the six Text items, per exinventory.go's
// EXItem.Digits/Text docs) + ";" -- is rejected outbound, even though
// ParseEXAnswer would happily parse it as a well-formed inbound answer.
//
// This was written as a phase-scoped invariant (M8a-M8d) expecting a
// later milestone to rewrite it. The M8d menu-write decision
// (25/07/2026) went the other way: the menu surface is read-only for
// v1.x (docs/menu-write-decision.md), so this test now pins shipped
// policy. Any future reopening would follow MT's set/answer exception --
// see validEXRead's doc comment and the MT set/answer-shaped ACCEPT
// entry in TestAllowedCommand_AcceptsAllowlistedSingleFrames.
func TestAllowedCommand_EXAnswersRejectedOutboundAll296(t *testing.T) {
	items := EXItems()
	for _, it := range items {
		var p4 string
		if it.Text {
			p4 = strings.Repeat(" ", it.Digits)
		} else {
			p4 = strings.Repeat("0", it.Digits)
		}
		frame := "EX" + it.Addr.Wire() + p4 + ";"
		if AllowedCommand([]byte(frame)) {
			t.Errorf("AllowedCommand(%q) = true, want false (EX Set/Answer-shaped, phase-scoped rejection) for %v", frame, it.Addr)
		}
	}
	if len(items) != 296 {
		t.Fatalf("test fixture bug: EXItems() returned %d items, want 296", len(items))
	}
}
