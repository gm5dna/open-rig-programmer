// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft450d

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
		{"001", slotMemory},
		{"500", slotMemory},
		{"501", slotPMS},
		{"504", slotPMS},
		{"000", slotInvalid}, // answer-only none form, never a valid request
		{"505", slotInvalid}, // 60m/Alaska: not in this dialect at all
		{"510", slotInvalid},
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

func TestWritableSlot_ExcludesPMS(t *testing.T) {
	if !writableSlot(slotMemory) {
		t.Error("writableSlot(slotMemory) = false, want true")
	}
	if writableSlot(slotPMS) {
		t.Error("writableSlot(slotPMS) = true, want false — doc.go's register entry PMS SLOTS (501-504) REFUSE EVERY MW SET")
	}
	if writableSlot(slotInvalid) {
		t.Error("writableSlot(slotInvalid) = true, want false")
	}
}

func TestReadableSlot_CoversBothBanks(t *testing.T) {
	if !readableSlot(slotMemory) || !readableSlot(slotPMS) {
		t.Error("readableSlot must accept both slotMemory and slotPMS — matrix §1.1's read range 001-504")
	}
	if readableSlot(slotInvalid) {
		t.Error("readableSlot(slotInvalid) = true, want false")
	}
}

func TestValidModeByte(t *testing.T) {
	for b := byte('1'); b <= '9'; b++ {
		if !validModeByte(b) {
			t.Errorf("validModeByte(%q) = false, want true", b)
		}
	}
	for _, b := range []byte("BC") {
		if !validModeByte(b) {
			t.Errorf("validModeByte(%q) = false, want true", b)
		}
	}
	for _, b := range []byte("0ADEF") {
		if validModeByte(b) {
			t.Errorf("validModeByte(%q) = true, want false — this radio's mode legend has a clean hole at 'A' and stops at 'C' (doc.go)", b)
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
	// placeholder — doc.go's register entry AN ANSWER'S KIND BYTE IS
	// ALWAYS '1'.
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
		{"mode is the clean hole 'A'", func(b []byte) { b[blkMode] = 'A' }},
		{"mode is 'D' (absent entirely)", func(b []byte) { b[blkMode] = 'D' }},
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

// send is a small test helper: run one frame through a Radio's handler
// and return the reply, exercising handleFrame directly rather than
// round tripping bytes through the pipe.
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
	block := mustBlock(t, "001", "03500000", '-', "0000", false, false, '1', mwSetKindFixed, '0', "00", '0')
	if reply := send(r, "MW"+string(block)+";"); reply != nil {
		t.Fatalf("MW Set to a populated channel replied %q, want silence", reply)
	}
	s, _ := r.SlotState("001")
	if s.Freq != "03500000" {
		t.Errorf("MW did not overwrite channel 001: got Freq %q", s.Freq)
	}
}

func TestHandleMW_MalformedSetIsRejected(t *testing.T) {
	r := New()
	defer r.Close()
	if reply := send(r, "MW001;"); !bytes.Equal(reply, rejection) {
		t.Errorf("short MW body: reply = %q, want %q", reply, rejection)
	}
}

// TestHandleMW_RefusesEveryPMSSlot is the wave's own reason to exist:
// doc.go's register entry PMS SLOTS (501-504) REFUSE EVERY MW SET —
// the SAFE SHAPE ruling, not a manual restriction. A WELL-FORMED block
// addressed to a PMS channel must still be refused, and the refusal must
// change nothing.
func TestHandleMW_RefusesEveryPMSSlot(t *testing.T) {
	r := New()
	defer r.Close()
	for _, slot := range []string{"501", "502", "503", "504"} {
		block := mustBlock(t, slot, "05330000", '+', "0000", false, false, '3', mwSetKindFixed, '0', "00", '0')
		before, hadBefore := r.SlotState(slot)
		reply := send(r, "MW"+string(block)+";")
		if !bytes.Equal(reply, rejection) {
			t.Errorf("MW to PMS slot %s: reply = %q, want %q", slot, reply, rejection)
		}
		after, hasAfter := r.SlotState(slot)
		if hadBefore != hasAfter || (hadBefore && before != after) {
			t.Errorf("a refused MW to PMS slot %s changed stored state: before=%+v(%v) after=%+v(%v)", slot, before, hadBefore, after, hasAfter)
		}
	}
}

func TestHandleMR_ReadsAPopulatedMemoryChannel(t *testing.T) {
	r := New()
	defer r.Close()
	reply := send(r, "MR001;")
	if len(reply) != 2+memBlockLen+1 {
		t.Fatalf("MR001 answer length = %d, want %d", len(reply), 2+memBlockLen+1)
	}
	if !bytes.HasPrefix(reply, []byte("MR001")) {
		t.Errorf("MR001 answer = %q, does not open with the slot", reply)
	}
}

// TestHandleMR_ReadsAPopulatedPMSSlot confirms MR still ANSWERS a PMS
// slot — the read half of the split write ceiling (matrix §3/§4): only
// MW is refused, never MR.
func TestHandleMR_ReadsAPopulatedPMSSlot(t *testing.T) {
	r := New(WithSlot("501", MemState{
		Freq: "05330000", ClarSign: '+', ClarMag: "0000",
		Mode: '3', Kind: kindMemory, CTCSS: '0', Tone: "00", Shift: '0',
	}))
	defer r.Close()
	reply := send(r, "MR501;")
	if !bytes.HasPrefix(reply, []byte("MR501")) {
		t.Errorf("MR501 answer = %q, want it to open with the slot (PMS is READ, not refused)", reply)
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
	for _, slot := range []string{"000", "505", "510", "999"} {
		if reply := send(r, "MR"+slot+";"); !bytes.Equal(reply, rejection) {
			t.Errorf("MR%s: reply = %q, want %q", slot, reply, rejection)
		}
	}
}

func TestHandleMR_HasNoSetDirection(t *testing.T) {
	r := New()
	defer r.Close()
	block := mustBlock(t, "001", "07000000", '+', "0000", false, false, '1', mwSetKindFixed, '0', "00", '0')
	if reply := send(r, "MR"+string(block)+";"); !bytes.Equal(reply, rejection) {
		t.Errorf("a 24-byte MR body (the MW Set shape): reply = %q, want %q — MR has no Set direction", reply, rejection)
	}
}

func TestHandleMC_SelectAndReadBack(t *testing.T) {
	r := New()
	defer r.Close()
	if reply := send(r, "MC001;"); reply != nil {
		t.Fatalf("MC-set replied %q, want silence", reply)
	}
	if got, want := r.CurrentChannel(), "001"; got != want {
		t.Errorf("CurrentChannel = %q, want %q", got, want)
	}
	if reply, want := send(r, "MC;"), []byte("MC001;"); !bytes.Equal(reply, want) {
		t.Errorf("MC read = %q, want %q", reply, want)
	}
}

// TestHandleMC_CanSelectAPMSSlot confirms MC selects across BOTH banks
// (matrix §1.1, MCSelectsAll) even though MW cannot write one.
func TestHandleMC_CanSelectAPMSSlot(t *testing.T) {
	r := New()
	defer r.Close()
	if reply := send(r, "MC501;"); reply != nil {
		t.Fatalf("MC-set of a populated PMS slot replied %q, want silence", reply)
	}
	if got, want := r.CurrentChannel(), "501"; got != want {
		t.Errorf("CurrentChannel = %q, want %q", got, want)
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
	if reply, want := send(r, "MC;"), []byte("MC000;"); !bytes.Equal(reply, want) {
		t.Errorf("MC read before any Set = %q, want %q", reply, want)
	}
}

func TestHandleAI_DefaultsToOffAndSetIsSilent(t *testing.T) {
	r := New()
	defer r.Close()
	if reply, want := send(r, "AI;"), []byte("AI0;"); !bytes.Equal(reply, want) {
		t.Errorf("AI read at construction = %q, want %q (OFF by default)", reply, want)
	}
	if reply := send(r, "AI1;"); reply != nil {
		t.Fatalf("AI Set replied %q, want silence (fire-and-forget)", reply)
	}
	if reply, want := send(r, "AI;"), []byte("AI1;"); !bytes.Equal(reply, want) {
		t.Errorf("AI read after Set = %q, want %q", reply, want)
	}
}

func TestHandleAI_MalformedSetIsRejected(t *testing.T) {
	r := New()
	defer r.Close()
	if reply := send(r, "AI2;"); !bytes.Equal(reply, rejection) {
		t.Errorf("AI2 (outside the {0,1} legend): reply = %q, want %q", reply, rejection)
	}
}

// TestEngineInitSequence_AIThenID reproduces core/transport.Engine.Init's
// critical-path sequence: AI0; sent as a Set and expected to be answered
// with silence, then ID; read normally.
func TestEngineInitSequence_AIThenID(t *testing.T) {
	r := New()
	defer r.Close()
	if reply := send(r, "AI0;"); reply != nil {
		t.Fatalf("AI0; (Engine.Init's own Set) replied %q, want silence", reply)
	}
	if reply, want := send(r, "ID;"), []byte("ID0244;"); !bytes.Equal(reply, want) {
		t.Errorf("ID; after AI0; = %q, want %q", reply, want)
	}
}

func TestHandleID_IsFixed(t *testing.T) {
	r := New()
	defer r.Close()
	if reply, want := send(r, "ID;"), []byte("ID0244;"); !bytes.Equal(reply, want) {
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
	// Bytes up to and including the next ';' are discarded; framing
	// resumes after it.
	events = acc.push([]byte("garbage;ID;"))
	if len(events) != 1 || events[0].overflow || string(events[0].frame) != "ID;" {
		t.Fatalf("events after resync = %+v, want one frame \"ID;\"", events)
	}
}
