// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx5000

import (
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
)

// TestReadChannel_Populated reads a populated MEM slot and checks every
// mapped field, including CTCSSTone — the one field this radio maps that
// no other registered Yaesu sibling does.
func TestReadChannel_Populated(t *testing.T) {
	img := slotImage{mrAnswers: map[string]string{"010": populatedAnswer("010")}}
	_, sess := openSession(t, Simulated, img)

	ch, err := sess.ReadChannel(testCtx(t), "010")
	if err != nil {
		t.Fatalf("ReadChannel(\"010\") = %v, want nil", err)
	}
	if ch.Data == nil {
		t.Fatal("ReadChannel returned an empty channel, want the populated one")
	}
	d := *ch.Data
	if d.FreqHz != 14_250_000 {
		t.Errorf("FreqHz = %d, want 14250000", d.FreqHz)
	}
	if d.Mode != "USB" {
		t.Errorf("Mode = %q, want \"USB\" (wire '2')", d.Mode)
	}
	if d.ClarHz != -150 || !d.RxClar || d.TxClar {
		t.Errorf("ClarHz/RxClar/TxClar = %d/%v/%v, want -150/true/false", d.ClarHz, d.RxClar, d.TxClar)
	}
	if d.CTCSS != "ENC-DEC" {
		t.Errorf("CTCSS = %q, want \"ENC-DEC\" (wire '1')", d.CTCSS)
	}
	if d.CTCSSTone.State != codeplug.Known || d.CTCSSTone.Value != 797 {
		t.Errorf("CTCSSTone = %+v, want Known 79.7 Hz (chart index 05)", d.CTCSSTone)
	}
	if d.Shift != "PLUS" {
		t.Errorf("Shift = %q, want \"PLUS\" (wire '1')", d.Shift)
	}
	if d.Tag != "" {
		t.Errorf("Tag = %q, want \"\" — this radio has no tag route", d.Tag)
	}
	if d.TagDisplay.State != codeplug.Unavailable {
		t.Errorf("TagDisplay.State = %v, want Unavailable", d.TagDisplay.State)
	}
	if d.ScanSkip.State != codeplug.Unavailable {
		t.Errorf("ScanSkip.State = %v, want Unavailable", d.ScanSkip.State)
	}
	if d.TxFreqHz.State != codeplug.Unavailable || d.IPPlus.State != codeplug.Unavailable {
		t.Errorf("an Icom-tier field is not Unavailable: TxFreqHz=%v IPPlus=%v", d.TxFreqHz.State, d.IPPlus.State)
	}
}

// TestReadChannel_EmptySlot: a "?;" rejection maps to an empty channel,
// not an error.
func TestReadChannel_EmptySlot(t *testing.T) {
	_, sess := openSession(t, Simulated, slotImage{})

	ch, err := sess.ReadChannel(testCtx(t), "050")
	if err != nil {
		t.Fatalf("ReadChannel(\"050\") = %v, want nil (an unanswered slot is an empty channel)", err)
	}
	if ch.Data != nil {
		t.Errorf("ch.Data = %+v, want nil for an empty slot", ch.Data)
	}
	if ch.Slot != "050" {
		t.Errorf("ch.Slot = %q, want \"050\"", ch.Slot)
	}
}

// TestReadChannel_AnswerMismatch: an answer naming a different slot is a
// typed refusal, never silently accepted.
func TestReadChannel_AnswerMismatch(t *testing.T) {
	img := slotImage{mrAnswers: map[string]string{"010": populatedAnswer("011")}}
	_, sess := openSession(t, Simulated, img)

	_, err := sess.ReadChannel(testCtx(t), "010")
	var mismatch *AnswerMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("ReadChannel = %v, want a *AnswerMismatchError", err)
	}
	if mismatch.Requested != "010" || mismatch.Answered != "011" {
		t.Errorf("mismatch = %+v, want Requested \"010\" Answered \"011\"", mismatch)
	}
}

// TestReadChannel_InvalidSlot is refused before any wire traffic.
func TestReadChannel_InvalidSlot(t *testing.T) {
	p, sess := openSession(t, Simulated, slotImage{})

	if _, err := sess.ReadChannel(testCtx(t), "999"); err == nil {
		t.Fatal("ReadChannel(\"999\") = nil error, want a refusal — 999 is outside this radio's 001-117 slot space")
	}
	for _, f := range p.Transcript()[2:] {
		t.Errorf("unexpected wire frame %q for an invalid slot that should never reach the wire", f)
	}
}
