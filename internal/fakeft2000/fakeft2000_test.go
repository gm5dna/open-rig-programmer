// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft2000

import (
	"bytes"
	"testing"
	"time"
)

func TestNew_DefaultsToFT2000(t *testing.T) {
	r := New()
	defer r.Close()
	if r.Model() != "FT-2000" {
		t.Errorf("Model() = %q, want %q", r.Model(), "FT-2000")
	}
	if r.CATID() != "0251" {
		t.Errorf("CATID() = %q, want %q", r.CATID(), "0251")
	}
}

func TestNew_WithModelNameFT2000D(t *testing.T) {
	r := New(WithModelName("FT-2000D"))
	defer r.Close()
	if r.Model() != "FT-2000D" || r.CATID() != "0252" {
		t.Errorf("Model/CATID = %q/%q, want FT-2000D/0252", r.Model(), r.CATID())
	}
}

func TestPort_RoundTrip(t *testing.T) {
	r := New()
	defer r.Close()
	port := r.Port()
	if _, err := port.Write([]byte("ID;")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	buf := make([]byte, 64)
	n, err := port.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got, want := buf[:n], []byte("ID0251;"); !bytes.Equal(got, want) {
		t.Errorf("reply over the port = %q, want %q", got, want)
	}
}

func TestClose_IsPromptDespiteAPendingLatency(t *testing.T) {
	r := New(WithLatency(time.Hour))
	port := r.Port()
	if _, err := port.Write([]byte("ID;")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	done := make(chan struct{})
	go func() {
		r.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not return promptly despite a pending latency wait")
	}
}

func TestWithSlot_Overlay(t *testing.T) {
	r := New(WithSlot("050", MemState{
		Freq: "21200000", ClarSign: '+', ClarMag: "0000",
		Mode: '3', Kind: kindMemory, CTCSS: '0', Tone: "00", Shift: '0',
	}))
	defer r.Close()
	s, ok := r.SlotState("050")
	if !ok || s.Freq != "21200000" {
		t.Errorf("WithSlot did not overlay channel 050: %+v, ok=%v", s, ok)
	}
	// The rest of the default image is untouched.
	if _, ok := r.SlotState("001"); !ok {
		t.Error("WithSlot dropped the default image's channel 001")
	}
}

func TestWithFactoryImage_ReplacesTheWholeMap(t *testing.T) {
	empty := func() map[string]MemState { return map[string]MemState{} }
	r := New(WithFactoryImage(empty))
	defer r.Close()
	if _, ok := r.SlotState("001"); ok {
		t.Error("WithFactoryImage did not replace the default image")
	}
}

func TestTwoRadiosDoNotShareState(t *testing.T) {
	r1 := New()
	defer r1.Close()
	r2 := New()
	defer r2.Close()

	block := mustBlock(t, "010", "07100000", '+', "0000", false, false, '1', mwSetKindFixed, '0', "00", '0')
	send(r1, "MW"+string(block)+";")

	if _, ok := r2.SlotState("010"); ok {
		t.Error("a write to r1 is visible on r2 — the two Radios share mutable state")
	}
}
