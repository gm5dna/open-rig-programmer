// SPDX-License-Identifier: GPL-3.0-or-later

package fakeradio

import "testing"

// --- parseSlotForm ---

func TestParseSlotForm(t *testing.T) {
	tests := []struct {
		name string
		slot string
		want slotKind
	}{
		{"memory low edge", "001", slotMemory},
		{"memory high edge", "099", slotMemory},
		{"memory mid", "007", slotMemory},
		{"vfo/answer-only 000", "000", slotVFO000},
		{"PMS P1L", "P1L", slotPMS},
		{"PMS P9U", "P9U", slotPMS},
		{"PMS P5L", "P5L", slotPMS},
		{"60m low", "500", slot60m},
		{"60m mid", "501", slot60m},
		{"60m high", "599", slot60m},
		{"EMG", "EMG", slotEMG},

		// Rejection cases from the protocol reference's golden-vector table.
		{"reject: 100 (out of memory range, not 5xx)", "100", slotInvalid},
		{"reject: P0L (PMS pair digit 0 invalid)", "P0L", slotInvalid},
		{"reject: P9X (bad L/U suffix)", "P9X", slotInvalid},
		{"reject: EM (wrong length)", "EM", slotInvalid},
		{"reject: empty", "", slotInvalid},
		{"reject: 600 (not memory, not 5xx)", "600", slotInvalid},
		{"reject: 999", "999", slotInvalid},
		{"reject: lowercase pms suffix", "p1l", slotInvalid},
		{"reject: 4 chars", "0001", slotInvalid},
		{"reject: non-numeric memory-shaped", "0a1", slotInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseSlotForm(tt.slot); got != tt.want {
				t.Errorf("parseSlotForm(%q) = %v, want %v", tt.slot, got, tt.want)
			}
		})
	}
}

func TestMwAllowedSlot(t *testing.T) {
	allowed := map[slotKind]bool{
		slotMemory:  true,
		slotPMS:     true,
		slot60m:     false,
		slotEMG:     false,
		slotVFO000:  false,
		slotInvalid: false,
	}
	for kind, want := range allowed {
		if got := mwAllowedSlot(kind); got != want {
			t.Errorf("mwAllowedSlot(%v) = %v, want %v", kind, got, want)
		}
	}
}

func TestMrReadableSlot(t *testing.T) {
	readable := map[slotKind]bool{
		slotMemory:  true,
		slotPMS:     true,
		slot60m:     true,
		slotEMG:     true,
		slotVFO000:  false,
		slotInvalid: false,
	}
	for kind, want := range readable {
		if got := mrReadableSlot(kind); got != want {
			t.Errorf("mrReadableSlot(%v) = %v, want %v", kind, got, want)
		}
	}
}

// --- validTag (SAFETY-CRITICAL) ---
//
// The reference's rejection-case example "X;TX1;" (a tag attempting
// command injection via an embedded ';') is tested here, directly
// against validTag, rather than end-to-end through Port(): a ';' inside
// an incoming byte stream is consumed by the ACCUMULATOR as a frame
// terminator before parser.go's command handling ever sees it, so
// "MT0011X;TX1;" arrives at handleFrame as two separate frames
// ("MT0011X;" and "TX1;"), never as one MT command whose tag contains a
// literal ';'. validTag's ';' rejection is still real defence in depth
// (in case that framing invariant is ever weakened), and is exactly what
// this test pins.

func TestValidTag(t *testing.T) {
	tests := []struct {
		name string
		tag  []byte
		want bool
	}{
		{"empty tag ok", []byte(""), true},
		{"12 ascii chars ok", []byte("CALLING FREQ"), true},
		{"3 chars ok", []byte("40M"), true},
		{"boundary 0x20 (space) ok", []byte{0x20}, true},
		{"boundary 0x7E (~) ok", []byte{0x7E}, true},

		{"reject: contains ;", []byte("X;TX1;"), false},
		{"reject: contains \\r", []byte("X\rY"), false},
		{"reject: contains \\n", []byte("X\nY"), false},
		{"reject: control byte 0x00", []byte{0x00}, false},
		{"reject: control byte 0x1F", []byte{0x1F}, false},
		{"reject: byte 0x7F (DEL)", []byte{0x7F}, false},
		{"reject: byte 0x80 (non-ASCII)", []byte{0x80}, false},
		{"reject: 13 ascii chars (byte-length 13)", []byte("ABCDEFGHIJKLM"), false},
		{
			// A 12-RUNE string containing a multi-byte rune whose BYTE
			// length exceeds 12: "CALLING FREQ" with the last char
			// replaced by 'é' (U+00E9, 2 UTF-8 bytes) is 12 runes but 13
			// bytes. Also independently caught by the charset check,
			// since UTF-8 continuation/lead bytes for any multi-byte
			// rune are always >= 0x80 (outside 0x20-0x7E) — belt and
			// braces, both checks must reject it.
			"reject: 12 runes, 13 bytes (multi-byte rune)",
			[]byte("CALLING FREÉ"), // 11 ASCII + 1 two-byte rune = 12 runes, 13 bytes
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validTag(tt.tag); got != tt.want {
				t.Errorf("validTag(%q) = %v, want %v", tt.tag, got, tt.want)
			}
		})
	}
}

func TestValidTag_ByteLengthNotRuneCount(t *testing.T) {
	tag := []byte("CALLING FREÉ")
	if len(tag) != 13 {
		t.Fatalf("test fixture error: expected 13 bytes, got %d", len(tag))
	}
	if validTag(tag) {
		t.Error("validTag accepted a tag whose byte length exceeds 12, even though its rune count is 12")
	}
}

// --- validClarMagDigits ---

func TestValidClarMagDigits(t *testing.T) {
	tests := []struct {
		name string
		mag  string
		want bool
	}{
		{"zero", "0000", true},
		{"mid step", "0120", true},
		{"max valid (9990)", "9990", true},
		{"reject: 9995 not a 10Hz step", "9995", false},
		{"reject: 9999 not a 10Hz step", "9999", false},
		{"reject: too short", "999", false},
		{"reject: too long", "09990", false},
		{"reject: non-digit", "12a0", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validClarMagDigits(tt.mag); got != tt.want {
				t.Errorf("validClarMagDigits(%q) = %v, want %v", tt.mag, got, tt.want)
			}
		})
	}
}

// --- encodeFreqDigits (semantic builder) ---

func TestEncodeFreqDigits(t *testing.T) {
	got, err := encodeFreqDigits(7_000_000)
	if err != nil {
		t.Fatalf("encodeFreqDigits(7_000_000): unexpected error: %v", err)
	}
	if got != "007000000" {
		t.Errorf("encodeFreqDigits(7_000_000) = %q, want %q", got, "007000000")
	}

	// Boundary: exactly 9 digits' worth is fine.
	got, err = encodeFreqDigits(999_999_999)
	if err != nil {
		t.Fatalf("encodeFreqDigits(999_999_999): unexpected error: %v", err)
	}
	if got != "999999999" {
		t.Errorf("encodeFreqDigits(999_999_999) = %q, want %q", got, "999999999")
	}
}

func TestEncodeFreqDigits_RejectsMoreThanNineDigits(t *testing.T) {
	_, err := encodeFreqDigits(1_000_000_000)
	if err == nil {
		t.Fatal("encodeFreqDigits(1_000_000_000): want error (needs 10 digits), got nil")
	}
}

// --- encodeClarifier (semantic builder) ---

func TestEncodeClarifier(t *testing.T) {
	sign, mag, err := encodeClarifier('-', 120)
	if err != nil {
		t.Fatalf("encodeClarifier('-', 120): unexpected error: %v", err)
	}
	if sign != '-' || mag != "0120" {
		t.Errorf("encodeClarifier('-', 120) = (%q, %q), want ('-', \"0120\")", sign, mag)
	}
}

func TestEncodeClarifier_Rejections(t *testing.T) {
	tests := []struct {
		name  string
		sign  byte
		magHz uint16
	}{
		{"reject: bad sign", 'x', 0},
		{"reject: 9995 not a 10Hz step", '+', 9995},
		{"reject: magnitude exceeds 9990 (was documented as +-10000)", '+', 9999},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := encodeClarifier(tt.sign, tt.magHz); err == nil {
				t.Errorf("encodeClarifier(%q, %d): want error, got nil", tt.sign, tt.magHz)
			}
		})
	}
}

func TestEncodeClarifier_RejectsOutOfRangeMagnitude(t *testing.T) {
	// uint16 can represent 10000 directly (unlike the 4-digit wire
	// field, which cannot) — encodeClarifier must still refuse it, since
	// the manual caps the field at 9990.
	if _, _, err := encodeClarifier('+', 10000); err == nil {
		t.Error("encodeClarifier('+', 10000): want error (exceeds 9990), got nil")
	}
}

// --- mode validators ---

func TestValidModeBuildByte(t *testing.T) {
	for _, m := range []byte("123456789ABCDEF") {
		if !validModeBuildByte(m) {
			t.Errorf("validModeBuildByte(%q) = false, want true", m)
		}
	}
	rejects := []byte{'0', 'G', 'a', ' ', 0}
	for _, m := range rejects {
		if validModeBuildByte(m) {
			t.Errorf("validModeBuildByte(%q) = true, want false", m)
		}
	}
}

func TestValidModeWireByte_AcceptsUnsetPlaceholder(t *testing.T) {
	// Reference mode table footer: "'0' = '-' (unset) ... parser accepts
	// it" — unlike validModeBuildByte, the wire-level (lenient) validator
	// must accept '0'.
	if !validModeWireByte('0') {
		t.Error("validModeWireByte('0') = false, want true (parser must accept the unset placeholder)")
	}
	if validModeWireByte('G') {
		t.Error("validModeWireByte('G') = true, want false")
	}
}

// --- buildMRAnswer: golden vectors G4 and G6, recomputed here as plain
// literals from the reference table — NOT by calling any other fakeradio
// builder. ---

func TestBuildMRAnswer_GoldenVectors(t *testing.T) {
	tests := []struct {
		name  string
		slot  string
		state MemState
		want  string
	}{
		{
			// G4: MR001007000000+000000110000; — M-01, 7.000000 MHz,
			// clar +0000 off/off, LSB(1), Memory(1), CTCSS off, simplex.
			name: "G4",
			slot: "001",
			state: MemState{
				Freq: "007000000", ClarSign: '+', ClarMag: "0000",
				RXClar: false, TXClar: false, Mode: '1', Kind: '1', CTCSS: '0', Shift: '0',
			},
			want: "MR001007000000+000000110000;",
		},
		{
			// G6: MRP1L001810000+000000150000; — PMS P1L, 1.810000 MHz,
			// LSB, kind PMS(5).
			name: "G6",
			slot: "P1L",
			state: MemState{
				Freq: "001810000", ClarSign: '+', ClarMag: "0000",
				RXClar: false, TXClar: false, Mode: '1', Kind: '5', CTCSS: '0', Shift: '0',
			},
			want: "MRP1L001810000+000000150000;",
		},
		{
			// Every field != default, mirroring G7's MW frame but as an
			// MR answer: M-99, 52.354000 MHz, clar -0120 rx-on/tx-off,
			// FM, kind Memory, CTCSS ENC/DEC, minus shift.
			name: "all non-default fields",
			slot: "099",
			state: MemState{
				Freq: "052354000", ClarSign: '-', ClarMag: "0120",
				RXClar: true, TXClar: false, Mode: '4', Kind: '1', CTCSS: '1', Shift: '2',
			},
			want: "MR099052354000-012010411002;",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(buildMRAnswer(tt.slot, tt.state))
			if got != tt.want {
				t.Errorf("buildMRAnswer(%q, ...) = %q, want %q", tt.slot, got, tt.want)
			}
			if len(got) != 28 {
				t.Errorf("buildMRAnswer(%q, ...) length = %d, want 28", tt.slot, len(got))
			}
		})
	}
}

// --- buildMTReply: golden vectors G8, G9 (as Set/Answer layout). ---

func TestBuildMTReply_GoldenVectors(t *testing.T) {
	tests := []struct {
		name    string
		slot    string
		display bool
		tag     string
		want    string
	}{
		{"G8", "001", true, "CALLING FREQ", "MT0011CALLING FREQ;"},
		{"G9", "005", false, "40M", "MT005040M;"},
		{"empty tag", "002", false, "", "MT0020;"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(buildMTReply(tt.slot, tt.display, tt.tag))
			if got != tt.want {
				t.Errorf("buildMTReply(%q, %v, %q) = %q, want %q", tt.slot, tt.display, tt.tag, got, tt.want)
			}
		})
	}
}

// --- buildMCAnswer / buildIDAnswer / buildAIAnswer ---

func TestBuildMCAnswer(t *testing.T) {
	// Derived from G11 ("MC099;" — a Set frame) applied to the Answer
	// layout ("MC" + current + ";"), per the reference's "MC" section.
	if got, want := string(buildMCAnswer("099")), "MC099;"; got != want {
		t.Errorf("buildMCAnswer(%q) = %q, want %q", "099", got, want)
	}
	if got, want := string(buildMCAnswer("000")), "MC000;"; got != want {
		t.Errorf("buildMCAnswer(%q) = %q, want %q", "000", got, want)
	}
}

func TestBuildIDAnswer(t *testing.T) {
	// G1: ID; -> ID0800;
	if got, want := string(buildIDAnswer()), "ID0800;"; got != want {
		t.Errorf("buildIDAnswer() = %q, want %q", got, want)
	}
	if len(buildIDAnswer()) != 7 {
		t.Errorf("buildIDAnswer() length = %d, want 7", len(buildIDAnswer()))
	}
}

func TestBuildAIAnswer(t *testing.T) {
	if got, want := string(buildAIAnswer('0')), "AI0;"; got != want {
		t.Errorf("buildAIAnswer('0') = %q, want %q", got, want)
	}
	if got, want := string(buildAIAnswer('1')), "AI1;"; got != want {
		t.Errorf("buildAIAnswer('1') = %q, want %q", got, want)
	}
}

// --- reassembler ---

func TestReassembler_SingleChunkMultipleFrames(t *testing.T) {
	a := newReassembler(64)
	events := a.push([]byte("ID0800;AI0;?;"))
	want := []string{"ID0800;", "AI0;", "?;"}
	assertFrames(t, events, want)
}

func TestReassembler_SplitAcrossChunks(t *testing.T) {
	a := newReassembler(64)
	var events []accEvent
	for _, chunk := range []string{"MR", "001", ";"} {
		events = append(events, a.push([]byte(chunk))...)
	}
	assertFrames(t, events, []string{"MR001;"})
}

func TestReassembler_ByteAtATime(t *testing.T) {
	a := newReassembler(64)
	var events []accEvent
	for _, b := range []byte("MW005;") {
		events = append(events, a.push([]byte{b})...)
	}
	assertFrames(t, events, []string{"MW005;"})
}

func TestReassembler_OverflowThenResync(t *testing.T) {
	a := newReassembler(8)
	// 9 non-terminator bytes, exceeding max=8, followed by junk up to a
	// ';', then a legitimate short frame.
	events := a.push([]byte("123456789garbage;ID;"))
	if len(events) != 2 {
		t.Fatalf("events = %+v, want 2 events (overflow, then ID; frame)", events)
	}
	if !events[0].overflow || events[0].frame != nil {
		t.Errorf("events[0] = %+v, want overflow event with nil frame", events[0])
	}
	if events[1].overflow || string(events[1].frame) != "ID;" {
		t.Errorf("events[1] = %+v, want frame \"ID;\"", events[1])
	}
}

func TestReassembler_OverflowExactlyAtBoundaryStillWorks(t *testing.T) {
	a := newReassembler(8)
	// Exactly 8 bytes then terminator: must NOT overflow (not "more
	// than" max).
	events := a.push([]byte("12345678;"))
	assertFrames(t, events, []string{"12345678;"})
}

func TestReassembler_OverflowSplitAcrossPushCalls(t *testing.T) {
	a := newReassembler(4)
	var events []accEvent
	events = append(events, a.push([]byte("123"))...)
	events = append(events, a.push([]byte("45;"))...) // now 5 bytes accumulated before ';' — over max=4
	if len(events) != 1 || !events[0].overflow {
		t.Fatalf("events = %+v, want a single overflow event", events)
	}
	// Resync completes at the ';' already consumed above; the next push
	// should parse normally (frame kept well within max=4 so it isn't
	// itself an overflow).
	events = a.push([]byte("ID;"))
	assertFrames(t, events, []string{"ID;"})
}

func assertFrames(t *testing.T, events []accEvent, want []string) {
	t.Helper()
	if len(events) != len(want) {
		t.Fatalf("got %d events %+v, want %d frames %v", len(events), events, len(want), want)
	}
	for i, w := range want {
		if events[i].overflow {
			t.Errorf("events[%d] is an overflow event, want frame %q", i, w)
			continue
		}
		if string(events[i].frame) != w {
			t.Errorf("events[%d] = %q, want %q", i, events[i].frame, w)
		}
	}
}
