// SPDX-License-Identifier: GPL-3.0-or-later

package ft450d

import (
	"errors"
	"fmt"
	"testing"
)

// mrAnswerFrame builds a well-formed 27-byte MR answer (matrix §1.1) for a
// test peer to serve.
func mrAnswerFrame(slot string, freqHz uint32, clarSign byte, clarMag int, rxClar, txClar bool, mode byte, kind byte, ctcss byte, toneIdx int, shift byte) string {
	b := func(v bool) byte {
		if v {
			return '1'
		}
		return '0'
	}
	return fmt.Sprintf("MR%s%08d%c%04d%c%c%c%c%c%02d%c;",
		slot, freqHz, clarSign, clarMag, b(rxClar), b(txClar), mode, kind, ctcss, toneIdx, shift)
}

func TestReadChannel_Populated(t *testing.T) {
	frame := mrAnswerFrame("001", 14250000, '+', 0, false, false, '2', '1', '0', 0, '0')
	_, sess := openSession(t, Simulated, slotImage{mrAnswers: map[string]string{"001": frame}})

	ch, err := sess.ReadChannel(testCtx(t), "001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if ch.Data == nil {
		t.Fatal("ReadChannel returned an empty channel, want populated")
	}
	if ch.Data.FreqHz != 14250000 {
		t.Errorf("FreqHz = %d, want 14250000", ch.Data.FreqHz)
	}
	if ch.Data.Mode != "USB" {
		t.Errorf("Mode = %q, want %q", ch.Data.Mode, "USB")
	}
	if ch.Data.CTCSSTone.Value.Hz() != 67.0 {
		t.Errorf("CTCSSTone.Value = %v, want tone index 0's chart entry (67.0 Hz)", ch.Data.CTCSSTone.Value)
	}
	if ch.Data.Tag != "" {
		t.Errorf("Tag = %q, want \"\" (NoTag)", ch.Data.Tag)
	}
}

// TestReadChannel_PMSSlot pins that a PMS slot reads through the exact same
// MR path as a MEM slot — the write split (caps.go's pmsFields) is a
// WriteChannel-only distinction.
func TestReadChannel_PMSSlot(t *testing.T) {
	frame := mrAnswerFrame("501", 5000000, '+', 0, false, false, '4', '1', '0', 0, '0')
	_, sess := openSession(t, Simulated, slotImage{mrAnswers: map[string]string{"501": frame}})

	ch, err := sess.ReadChannel(testCtx(t), "501")
	if err != nil {
		t.Fatalf("ReadChannel(\"501\"): %v", err)
	}
	if ch.Data == nil || ch.Data.FreqHz != 5000000 {
		t.Errorf("ReadChannel(\"501\") = %+v, want a populated 5000000 Hz channel", ch)
	}
}

func TestReadChannel_EmptySlotIsRejected(t *testing.T) {
	_, sess := openSession(t, Simulated, slotImage{})

	ch, err := sess.ReadChannel(testCtx(t), "050")
	if err != nil {
		t.Fatalf("ReadChannel of an unpopulated slot: %v", err)
	}
	if ch.Data != nil {
		t.Errorf("ReadChannel of a slot the peer rejects returned populated data, want an empty channel")
	}
}

func TestReadChannel_KindMismatch(t *testing.T) {
	// '2' is neither KindVFO ('0') nor KindMemory ('1') — this manual's own
	// read-side legend (layout:796) prints only those two.
	frame := mrAnswerFrame("001", 14250000, '+', 0, false, false, '2', '2', '0', 0, '0')
	_, sess := openSession(t, Simulated, slotImage{mrAnswers: map[string]string{"001": frame}})

	_, err := sess.ReadChannel(testCtx(t), "001")
	var mismatch *KindMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("ReadChannel error = %v, want *KindMismatchError", err)
	}
}
