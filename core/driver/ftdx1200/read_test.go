// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx1200

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
)

func TestReadChannel_Populated(t *testing.T) {
	_, s := openSession(t, Simulated, slotImage{
		mrAnswers: map[string]string{"001": populatedAnswer("001")},
	})
	ch, err := s.ReadChannel(testCtx(t), "001")
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
	if ch.Data.CTCSSTone.State != codeplug.Unavailable {
		t.Errorf("CTCSSTone.State = %v, want Unavailable — matrix §1.3: P9 is fixed on this radio's read side too", ch.Data.CTCSSTone.State)
	}
	if ch.Data.Tag != "" || ch.Data.TagDisplay.State != codeplug.Unavailable {
		t.Errorf("Tag/TagDisplay = %q/%v, want \"\"/Unavailable — NoTag", ch.Data.Tag, ch.Data.TagDisplay.State)
	}
}

func TestReadChannel_EmptySlotIsNotAnError(t *testing.T) {
	_, s := openSession(t, Simulated, slotImage{})
	ch, err := s.ReadChannel(testCtx(t), "002")
	if err != nil {
		t.Fatalf("ReadChannel of an unpopulated slot: %v", err)
	}
	if ch.Data != nil {
		t.Error("ReadChannel returned non-nil Data for an empty slot")
	}
	if ch.Slot != "002" {
		t.Errorf("Slot = %q, want \"002\"", ch.Slot)
	}
}

func TestReadChannel_AnswerMismatchRefuses(t *testing.T) {
	f := mrAnswerFields{
		slot: "099", freq: "14250000", clarSign: '+', clarMag: "0000",
		rxClar: '0', txClar: '0', mode: '2', kind: '1', ctcss: '0',
		tone: "00", shift: '0',
	}.frame()
	_, s := openSession(t, Simulated, slotImage{mrAnswers: map[string]string{"001": f}})

	if _, err := s.ReadChannel(testCtx(t), "001"); err == nil {
		t.Fatal("ReadChannel accepted an answer naming a different slot, want *AnswerMismatchError")
	}
}
