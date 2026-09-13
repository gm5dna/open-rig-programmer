// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft950

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseSlotForm(t *testing.T) {
	tests := []struct {
		slot string
		want slotKind
	}{
		{"000", slotMemory}, // this radio's own delta: MemoryLo is 000
		{"099", slotMemory},
		{"100", slotPMS},
		{"117", slotPMS},
		{"118", slotInvalid}, // answer-only none form, never a valid request
		{"999", slotInvalid},
		{"P1L", slotInvalid}, // the sibling token form this radio does not use
		{"1", slotInvalid},
		{"", slotInvalid},
	}
	for _, tt := range tests {
		if got := parseSlotForm(tt.slot); got != tt.want {
			t.Errorf("parseSlotForm(%q) = %v, want %v", tt.slot, got, tt.want)
		}
	}
}

func TestValidModeByte(t *testing.T) {
	for b := byte('1'); b <= '9'; b++ {
		if !validModeByte(b) {
			t.Errorf("validModeByte(%q) = false, want true", b)
		}
	}
	for _, b := range []byte("ABC") {
		if !validModeByte(b) {
			t.Errorf("validModeByte(%q) = false, want true", b)
		}
	}
	for _, b := range []byte("0DEF") {
		if validModeByte(b) {
			t.Errorf("validModeByte(%q) = true, want false — this radio's memory-record legend has no hole, no placeholder and stops at C (MD's own 13th value, D: AM-N, is not part of this record — doc.go)", b)
		}
	}
}

func TestValidCTCSSByte(t *testing.T) {
	for _, b := range []byte("012") {
		if !validCTCSSByte(b) {
			t.Errorf("validCTCSSByte(%q) = false, want true", b)
		}
	}
	for _, b := range []byte("34") {
		if validCTCSSByte(b) {
			t.Errorf("validCTCSSByte(%q) = true, want false — this radio's P8 is three-valued", b)
		}
	}
}

func TestValidToneDigits(t *testing.T) {
	tests := []struct {
		s    string
		want bool
	}{
		{"00", true},
		{"49", true},
		{"99", true}, // shape only — CN's separate 00-49 ceiling is not enforced here
		{"9", false},
		{"999", false},
		{"AB", false},
	}
	for _, tt := range tests {
		if got := validToneDigits(tt.s); got != tt.want {
			t.Errorf("validToneDigits(%q) = %v, want %v", tt.s, got, tt.want)
		}
	}
}

func mustBlock(t *testing.T, slot, freq string, clarSign byte, clarMag string, rx, tx bool, mode, kind, ctcss byte, tone string, shift byte) []byte {
	t.Helper()
	b := []byte(slot)
	b = append(b, freq...)
	b = append(b, clarSign)
	b = append(b, clarMag...)
	b = append(b, boolFlagByte(rx))
	b = append(b, boolFlagByte(tx))
	b = append(b, mode, kind, ctcss)
	b = append(b, tone...)
	b = append(b, shift)
	if len(b) != memBlockLen {
		t.Fatalf("mustBlock built %d bytes, want %d", len(b), memBlockLen)
	}
	return b
}

func TestParseMemoryBlock_Valid(t *testing.T) {
	block := mustBlock(t, "001", "07000000", '+', "0000", false, true, '3', mwSetKindFixed, '1', "26", '0')
	slot, s, ok := parseMemoryBlock(block)
	if !ok {
		t.Fatal("parseMemoryBlock rejected a well-formed block")
	}
	if slot != "001" {
		t.Errorf("slot = %q, want %q", slot, "001")
	}
	if s.Freq != "07000000" || s.ClarSign != '+' || s.ClarMag != "0000" || s.RXClar || !s.TXClar ||
		s.Mode != '3' || s.CTCSS != '1' || s.Tone != "26" || s.Shift != '0' {
		t.Errorf("parsed state = %+v, does not match the block", s)
	}
	// The stored Kind is always the ANSWER byte, never the Set's fixed
	// placeholder — doc.go's register entry AN ANSWER'S KIND BYTE IS ALWAYS
	// '1'.
	if s.Kind != kindMemory {
		t.Errorf("Kind = %q, want kindMemory %q", s.Kind, kindMemory)
	}
}

func TestParseMemoryBlock_RejectsBadFields(t *testing.T) {
	valid := mustBlock(t, "001", "07000000", '+', "0000", false, false, '3', mwSetKindFixed, '1', "00", '0')
	tests := []struct {
		name   string
		mutate func([]byte)
	}{
		{"non-digit frequency", func(b []byte) { b[blkFreqStart] = 'X' }},
		{"bad clarifier sign", func(b []byte) { b[blkClarSign] = '*' }},
		{"bad clarifier magnitude", func(b []byte) { b[blkClarMagStart] = 'X' }},
		{"bad RX clarifier flag", func(b []byte) { b[blkRXClar] = '2' }},
		{"bad TX clarifier flag", func(b []byte) { b[blkTXClar] = '2' }},
		{"mode outside the legend", func(b []byte) { b[blkMode] = '0' }},
		{"kind not the fixed Set byte", func(b []byte) { b[blkKind] = '1' }},
		{"CTCSS outside the legend", func(b []byte) { b[blkCTCSS] = '3' }},
		{"tone not two digits", func(b []byte) { b[blkToneStart] = 'X' }},
		{"shift outside the legend", func(b []byte) { b[blkShift] = '3' }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := append([]byte(nil), valid...)
			tt.mutate(b)
			if _, _, ok := parseMemoryBlock(b); ok {
				t.Errorf("parseMemoryBlock accepted a block with %s", tt.name)
			}
		})
	}
}

func TestParseMemoryBlock_WrongLength(t *testing.T) {
	if _, _, ok := parseMemoryBlock([]byte("too short")); ok {
		t.Error("parseMemoryBlock accepted a block of the wrong length")
	}
}

// send is a small test helper: run one frame through a Radio's handler and
// return the reply, exercising handleFrame directly rather than round
// tripping bytes through the pipe — parser_test.go's job is the wire
// grammar, not the transport.
func send(r *Radio, frame string) []byte {
	return r.handleFrame([]byte(frame))
}

func TestHandleMW_AcceptedSetIsSilentAndStores(t *testing.T) {
	r := New()
	defer r.Close()
	block := mustBlock(t, "005", "14200000", '+', "0000", false, false, '2', mwSetKindFixed, '0', "00", '0')
	reply := send(r, "MW"+string(block)+";")
	if reply != nil {
		t.Fatalf("accepted MW Set replied %q, want silence", reply)
	}
	s, ok := r.SlotState("005")
	if !ok {
		t.Fatal("MW Set did not create the channel")
	}
	if s.Freq != "14200000" || s.Mode != '2' {
		t.Errorf("stored state = %+v, does not match the Set", s)
	}
	if r.CurrentChannel() != slotNoneWire {
		t.Errorf("CurrentChannel = %q, want %q — a Set must not move the selection", r.CurrentChannel(), slotNoneWire)
	}
}

func TestHandleMW_OverwritesAnExistingChannel(t *testing.T) {
	r := New()
	defer r.Close()
	block := mustBlock(t, "000", "03500000", '-', "0000", false, false, '1', mwSetKindFixed, '0', "00", '0')
	if reply := send(r, "MW"+string(block)+";"); reply != nil {
		t.Fatalf("MW Set to a populated channel replied %q, want silence", reply)
	}
	s, _ := r.SlotState("000")
	if s.Freq != "03500000" {
		t.Errorf("MW did not overwrite channel 000: got Freq %q", s.Freq)
	}
}

func TestHandleMW_MalformedSetIsRejected(t *testing.T) {
	r := New()
	defer r.Close()
	if reply := send(r, "MW001;"); !bytes.Equal(reply, rejection) {
		t.Errorf("short MW body: reply = %q, want %q", reply, rejection)
	}
}

func TestHandleMR_ReadsAPopulatedChannel(t *testing.T) {
	r := New()
	defer r.Close()
	reply := send(r, "MR000;")
	if len(reply) != 2+memBlockLen+1 {
		t.Fatalf("MR000 answer length = %d, want %d", len(reply), 2+memBlockLen+1)
	}
	if !bytes.HasPrefix(reply, []byte("MR000")) {
		t.Errorf("MR000 answer = %q, does not open with the slot", reply)
	}
}

func TestHandleMR_EmptySlotAnswersRejection(t *testing.T) {
	r := New()
	defer r.Close()
	if reply := send(r, "MR050;"); !bytes.Equal(reply, rejection) {
		t.Errorf("MR of an empty slot: reply = %q, want %q", reply, rejection)
	}
}

func TestHandleMR_OutOfDomainSlotAnswersRejection(t *testing.T) {
	r := New()
	defer r.Close()
	for _, slot := range []string{"118", "999"} {
		if reply := send(r, "MR"+slot+";"); !bytes.Equal(reply, rejection) {
			t.Errorf("MR%s: reply = %q, want %q", slot, reply, rejection)
		}
	}
}

func TestHandleMR_HasNoSetDirection(t *testing.T) {
	r := New()
	defer r.Close()
	block := mustBlock(t, "000", "07000000", '+', "0000", false, false, '1', mwSetKindFixed, '0', "00", '0')
	if reply := send(r, "MR"+string(block)+";"); !bytes.Equal(reply, rejection) {
		t.Errorf("a 24-byte MR body (the MW Set shape): reply = %q, want %q — MR has no Set direction", reply, rejection)
	}
}

func TestHandleMC_SelectAndReadBack(t *testing.T) {
	r := New()
	defer r.Close()
	if reply := send(r, "MC000;"); reply != nil {
		t.Fatalf("MC-set replied %q, want silence", reply)
	}
	if got, want := r.CurrentChannel(), "000"; got != want {
		t.Errorf("CurrentChannel = %q, want %q", got, want)
	}
	if reply, want := send(r, "MC;"), []byte("MC000;"); !bytes.Equal(reply, want) {
		t.Errorf("MC read = %q, want %q", reply, want)
	}
}

func TestHandleMC_SelectingAnEmptySlotIsRejected(t *testing.T) {
	r := New()
	defer r.Close()
	if reply := send(r, "MC050;"); !bytes.Equal(reply, rejection) {
		t.Errorf("MC-set of an empty slot: reply = %q, want %q", reply, rejection)
	}
	if r.CurrentChannel() != slotNoneWire {
		t.Errorf("a rejected MC-set moved the selection to %q", r.CurrentChannel())
	}
}

func TestHandleMC_ReadBeforeAnySetAnswersTheNoneForm(t *testing.T) {
	r := New()
	defer r.Close()
	if reply, want := send(r, "MC;"), []byte("MC118;"); !bytes.Equal(reply, want) {
		t.Errorf("MC read before any Set = %q, want %q", reply, want)
	}
}

func TestHandleID_IsFixed(t *testing.T) {
	r := New()
	defer r.Close()
	if reply, want := send(r, "ID;"), []byte("ID0310;"); !bytes.Equal(reply, want) {
		t.Errorf("ID answer = %q, want %q", reply, want)
	}
}

func TestHandleID_HasNoSetDirection(t *testing.T) {
	r := New()
	defer r.Close()
	if reply := send(r, "IDsomething;"); !bytes.Equal(reply, rejection) {
		t.Errorf("an ID body: reply = %q, want %q", reply, rejection)
	}
}

func TestCommandNamesAreAcceptedInEitherCase(t *testing.T) {
	r := New()
	defer r.Close()
	upper := send(r, "ID;")
	lower := send(r, "id;")
	mixed := send(r, "Id;")
	if !bytes.Equal(upper, lower) || !bytes.Equal(upper, mixed) {
		t.Errorf("case-folding mismatch: ID=%q id=%q Id=%q", upper, lower, mixed)
	}
}

func TestUnknownCommandIsRejected(t *testing.T) {
	r := New()
	defer r.Close()
	if reply := send(r, "ZZ;"); !bytes.Equal(reply, rejection) {
		t.Errorf("unknown command: reply = %q, want %q", reply, rejection)
	}
}

func TestAccumulatorOverflowRejectsOnceAndResyncs(t *testing.T) {
	acc := newReassembler(8)
	events := acc.push([]byte(strings.Repeat("X", 20)))
	if len(events) != 1 || !events[0].overflow {
		t.Fatalf("events = %+v, want exactly one overflow", events)
	}
	// Bytes up to and including the next ';' are discarded; framing resumes
	// after it.
	events = acc.push([]byte("garbage;ID;"))
	if len(events) != 1 || events[0].overflow || string(events[0].frame) != "ID;" {
		t.Fatalf("events after resync = %+v, want one frame \"ID;\"", events)
	}
}
