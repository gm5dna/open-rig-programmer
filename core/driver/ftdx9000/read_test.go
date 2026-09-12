// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx9000

import (
	"fmt"
	"testing"
)

// mrAnswerFrame builds a well-formed 27-byte MR answer (matrix §2) for a
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
	frame := mrAnswerFrame("001", 14250000, '+', 0, false, false, '2', '0', '0', 0, '0')
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
