// SPDX-License-Identifier: GPL-3.0-or-later

package ft2000

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
)

func TestReadChannel_Populated(t *testing.T) {
	_, s := openSession(t, testModels[0], Simulated, slotImage{
		mrAnswers: map[string]string{"001": populatedAnswer("001")},
	})

	ch, err := s.ReadChannel(testCtx(t), "001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if ch.Data == nil {
		t.Fatal("Data is nil, want a populated channel")
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
	// CTCSSTone comes back KNOWN (matrix §1.3) — unlike every registered
	// sibling, this radio's P9 is a live tone index.
	if ch.Data.CTCSSTone.State != codeplug.Known {
		t.Errorf("CTCSSTone.State = %q, want Known", ch.Data.CTCSSTone.State)
	}
	if ch.Data.CTCSSTone.Value != 670 { // index 00 = 67.0 Hz = decihertz 670
		t.Errorf("CTCSSTone.Value = %v, want 670 (67.0 Hz, tone index 00)", ch.Data.CTCSSTone.Value)
	}
	if ch.Data.TagDisplay.State != codeplug.Unavailable {
		t.Errorf("TagDisplay.State = %q, want Unavailable — NoTag", ch.Data.TagDisplay.State)
	}
	if ch.Data.Tag != "" {
		t.Errorf("Tag = %q, want \"\" — NoTag", ch.Data.Tag)
	}
	if ch.Data.ScanSkip.State != codeplug.Unknown {
		t.Errorf("ScanSkip.State = %q, want Unknown — unreadable via CAT", ch.Data.ScanSkip.State)
	}
}

func TestReadChannel_EmptySlotIsNotAnError(t *testing.T) {
	_, s := openSession(t, testModels[0], Simulated, slotImage{}) // no mrAnswers at all -> every read "?;"

	ch, err := s.ReadChannel(testCtx(t), "002")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if ch.Data != nil {
		t.Errorf("Data = %+v, want nil (empty channel)", ch.Data)
	}
	if ch.Slot != "002" {
		t.Errorf("Slot = %q, want \"002\"", ch.Slot)
	}
}

func TestReadChannel_LiveToneIndex(t *testing.T) {
	f := mrAnswerFields{
		slot: "003", freq: "14250000",
		clarSign: '+', clarMag: "0000", rxClar: '0', txClar: '0',
		mode: '2', kind: '1', ctcss: '1', tone: "26", shift: '0',
	}.frame()
	_, s := openSession(t, testModels[0], Simulated, slotImage{
		mrAnswers: map[string]string{"003": f},
	})

	ch, err := s.ReadChannel(testCtx(t), "003")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if ch.Data.CTCSSTone.Value != 1598 { // chart index 26 = 159.8 Hz (matrix §1.3 spot-check)
		t.Errorf("CTCSSTone.Value = %v, want 1598 (159.8 Hz, tone index 26)", ch.Data.CTCSSTone.Value)
	}
}

func TestReadChannel_AnswerMismatchIsRefused(t *testing.T) {
	_, s := openSession(t, testModels[0], Simulated, slotImage{
		mrAnswers: map[string]string{"001": populatedAnswer("099")}, // answers the WRONG slot
	})

	_, err := s.ReadChannel(testCtx(t), "001")
	if err == nil {
		t.Fatal("ReadChannel succeeded despite a wrong-slot answer, want a refusal")
	}
}

func TestReadChannel_InvalidSlotIsRefused(t *testing.T) {
	_, s := openSession(t, testModels[0], Simulated, slotImage{})

	if _, err := s.ReadChannel(testCtx(t), "not-a-slot"); err == nil {
		t.Fatal("ReadChannel succeeded for a malformed slot, want a refusal")
	}
}
