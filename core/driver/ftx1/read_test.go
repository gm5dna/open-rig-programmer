// SPDX-License-Identifier: GPL-3.0-or-later

package ftx1

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
)

func TestReadChannel_Populated(t *testing.T) {
	slot := mustMemorySlot(t, 1)
	_, s := openSession(t, Simulated, slotImage{
		mrAnswers: map[string]string{slot.Wire(): populatedMRAnswer(t, slot)},
		mtAnswers: map[string]string{slot.Wire(): populatedMTAnswer(t, slot, "HOME")},
	})
	ch, err := s.ReadChannel(testCtx(t), slot.Wire())
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if ch.Data == nil {
		t.Fatal("ReadChannel returned a nil Data for a populated channel")
	}
	if ch.Data.FreqHz != 14_250_000 {
		t.Errorf("FreqHz = %d, want 14250000", ch.Data.FreqHz)
	}
	if ch.Data.Mode != "USB" {
		t.Errorf("Mode = %q, want \"USB\"", ch.Data.Mode)
	}
	if ch.Data.CTCSS != "OFF" {
		t.Errorf("CTCSS = %q, want \"OFF\"", ch.Data.CTCSS)
	}
	if ch.Data.Shift != "SIMPLEX" {
		t.Errorf("Shift = %q, want \"SIMPLEX\"", ch.Data.Shift)
	}
	if ch.Data.Tag != "HOME" {
		t.Errorf("Tag = %q, want \"HOME\"", ch.Data.Tag)
	}
	if ch.Data.TagDisplay.State != codeplug.Unavailable {
		t.Errorf("TagDisplay.State = %v, want Unavailable — MTFormShortNoDisplay carries no display byte", ch.Data.TagDisplay.State)
	}
	if ch.Data.CTCSSTone.State != codeplug.Unknown {
		t.Errorf("CTCSSTone.State = %v, want Unknown", ch.Data.CTCSSTone.State)
	}
	if ch.Data.ScanSkip.State != codeplug.Unknown {
		t.Errorf("ScanSkip.State = %v, want Unknown", ch.Data.ScanSkip.State)
	}
}

func TestReadChannel_EmptySlotIsNotAnError(t *testing.T) {
	slot := mustMemorySlot(t, 2)
	_, s := openSession(t, Simulated, slotImage{})
	ch, err := s.ReadChannel(testCtx(t), slot.Wire())
	if err != nil {
		t.Fatalf("ReadChannel of an unpopulated slot: %v", err)
	}
	if ch.Data != nil {
		t.Error("ReadChannel returned non-nil Data for an empty slot")
	}
	if ch.Slot != slot.Wire() {
		t.Errorf("Slot = %q, want %q", ch.Slot, slot.Wire())
	}
}

// TestReadChannel_SixthToneState covers the FTX-1's own P8 byte '5' ("REV
// TONE"), which no other registered dialect's domain carries — the
// clearest proof that ctcssNames' six-entry map (read.go) is actually
// wired through the read path, not just declared.
func TestReadChannel_SixthToneState(t *testing.T) {
	slot := mustMemorySlot(t, 3)
	cmd, err := dialect.BuildMWSet(cat.MemoryData{
		Slot: slot, FreqHz: 14250000, Mode: cat.ModeUSB,
		Kind: dialect.MWWriteKind(), CTCSS: cat.CTCSSState('5'), Shift: cat.ShiftSimplex,
	})
	if err != nil {
		t.Fatalf("BuildMWSet: %v", err)
	}
	frame := append([]byte(nil), cmd.Bytes()...)
	frame[0], frame[1] = 'M', 'R'

	_, s := openSession(t, Simulated, slotImage{
		mrAnswers: map[string]string{slot.Wire(): string(frame)},
		mtAnswers: map[string]string{slot.Wire(): populatedMTAnswer(t, slot, "")},
	})
	ch, err := s.ReadChannel(testCtx(t), slot.Wire())
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if ch.Data.CTCSS != "REV-TONE" {
		t.Errorf("CTCSS = %q, want \"REV-TONE\"", ch.Data.CTCSS)
	}
}

func TestReadChannel_MRAnswerMismatchRefuses(t *testing.T) {
	slot1 := mustMemorySlot(t, 1)
	slot2 := mustMemorySlot(t, 2)
	_, s := openSession(t, Simulated, slotImage{
		mrAnswers: map[string]string{slot1.Wire(): populatedMRAnswer(t, slot2)},
	})
	if _, err := s.ReadChannel(testCtx(t), slot1.Wire()); err == nil {
		t.Fatal("ReadChannel accepted an MR answer naming a different slot, want *AnswerMismatchError")
	}
}

func TestReadChannel_MTAnswerMismatchRefuses(t *testing.T) {
	slot1 := mustMemorySlot(t, 1)
	slot2 := mustMemorySlot(t, 2)
	_, s := openSession(t, Simulated, slotImage{
		mrAnswers: map[string]string{slot1.Wire(): populatedMRAnswer(t, slot1)},
		mtAnswers: map[string]string{slot1.Wire(): populatedMTAnswer(t, slot2, "X")},
	})
	if _, err := s.ReadChannel(testCtx(t), slot1.Wire()); err == nil {
		t.Fatal("ReadChannel accepted an MT answer naming a different slot, want *AnswerMismatchError")
	}
}

func TestReadChannel_InvalidSlotRefuses(t *testing.T) {
	_, s := openSession(t, Simulated, slotImage{})
	if _, err := s.ReadChannel(testCtx(t), "not-a-slot"); err == nil {
		t.Fatal("ReadChannel accepted an invalid slot string, want a refusal")
	}
}
