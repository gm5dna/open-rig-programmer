// SPDX-License-Identifier: GPL-3.0-or-later

package fakeftdx1200

import (
	"bytes"
	"testing"
	"time"
)

func TestNew_DefaultsToFFT1Fitted(t *testing.T) {
	r := New()
	defer r.Close()
	if r.Model() != "FTdx1200" {
		t.Errorf("Model() = %q, want %q", r.Model(), "FTdx1200")
	}
	if r.CATID() != "0582" {
		t.Errorf("CATID() = %q, want %q", r.CATID(), "0582")
	}
}

// roundTrip writes req to the fake's port and returns whatever it replies.
// Only for a request that actually gets a reply (a Read, ID, or a refused
// Set) — a SUCCESSFUL Set is fire-and-forget silence (handleEvent) and must
// use writeOnly instead, or this blocks forever.
func roundTrip(t *testing.T, r *Radio, req string) []byte {
	t.Helper()
	writeOnly(t, r, req)
	buf := make([]byte, 64)
	n, err := r.Port().Read(buf)
	if err != nil {
		t.Fatalf("Read after %q: %v", req, err)
	}
	return buf[:n]
}

// writeOnly writes req to the fake's port without reading a reply — the
// right call for a Set this fake is expected to accept silently.
func writeOnly(t *testing.T, r *Radio, req string) {
	t.Helper()
	if _, err := r.Port().Write([]byte(req)); err != nil {
		t.Fatalf("Write(%q): %v", req, err)
	}
}

func TestPort_ID_RoundTrip(t *testing.T) {
	r := New()
	defer r.Close()
	if got, want := roundTrip(t, r, "ID;"), []byte("ID0582;"); !bytes.Equal(got, want) {
		t.Errorf("ID; = %q, want %q", got, want)
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

// mwFrame builds a syntactically valid 27-byte MW Set frame for slot,
// carrying an unremarkable field block with the given mode byte, so tests
// can probe one axis (mode, tone, kind, slot) at a time.
func mwFrame(slot string, mode byte) string {
	return "MW" + slot + "07000000" + "+0000" + "0" + "0" + string(mode) + "0" + "0" + "00" + "0" + ";"
}

func TestMW_MR_RoundTrip(t *testing.T) {
	r := New()
	defer r.Close()
	writeOnly(t, r, mwFrame("050", modeUSB))
	want := []byte("MR" + "050" + "07000000" + "+0000" + "0" + "0" + string(modeUSB) + string(kindMemory) + "0" + "00" + "0" + ";")
	if got := roundTrip(t, r, "MR050;"); !bytes.Equal(got, want) {
		t.Errorf("MR050; after MW = %q, want %q", got, want)
	}
}

func TestMW_ModeHoleAtA_Refused(t *testing.T) {
	r := New()
	defer r.Close()
	if got, want := roundTrip(t, r, mwFrame("050", 'A')), rejection; !bytes.Equal(got, want) {
		t.Errorf("MW with Mode 'A' (the printed hole, matrix §1.2) = %q, want %q", got, want)
	}
	// And 'A' was never stored: a read of the same slot still says "?;"
	// (nothing was ever written there).
	if got, want := roundTrip(t, r, "MR050;"), rejection; !bytes.Equal(got, want) {
		t.Errorf("MR050; after a refused MW = %q, want %q (slot must remain empty)", got, want)
	}
}

func TestMW_ToneMustBeLiteral00(t *testing.T) {
	r := New()
	defer r.Close()
	bad := "MW" + "051" + "07000000" + "+0000" + "0" + "0" + string(modeUSB) + "0" + "0" + "05" + "0" + ";"
	if got, want := roundTrip(t, r, bad), rejection; !bytes.Equal(got, want) {
		t.Errorf("MW with Tone %q (not the fixed \"00\", matrix §1.3) = %q, want %q", "05", got, want)
	}
}

func TestMR_ToneAlwaysReadsFixed00(t *testing.T) {
	r := New()
	defer r.Close()
	got := roundTrip(t, r, "MR001;") // "001" is populated by DefaultImage
	if len(got) != 2+memBlockLen+1 {
		t.Fatalf("MR001; reply length = %d, want %d", len(got), 2+memBlockLen+1)
	}
	tone := got[2+blkToneStart : 2+blkToneEnd]
	if !bytes.Equal(tone, []byte(toneFixedBytes)) {
		t.Errorf("MR001; tone field = %q, want %q (matrix §1.3, printed-fixed on read)", tone, toneFixedBytes)
	}
}

func TestMW_KindByteMustBeFixed0(t *testing.T) {
	r := New()
	defer r.Close()
	bad := "MW" + "052" + "07000000" + "+0000" + "0" + "0" + string(modeUSB) + "1" /* not fixed '0' */ + "0" + "00" + "0" + ";"
	if got, want := roundTrip(t, r, bad), rejection; !bytes.Equal(got, want) {
		t.Errorf("MW with Kind byte '1' (not MW's own fixed '0', matrix §1.4) = %q, want %q", got, want)
	}
}

func TestMR_Slot000Refused(t *testing.T) {
	r := New()
	defer r.Close()
	if got, want := roundTrip(t, r, "MR000;"), rejection; !bytes.Equal(got, want) {
		t.Errorf("MR000; = %q, want %q (channel \"000\" is out of scope, doc.go)", got, want)
	}
}

func TestMR_PMSSlotRoundTrip(t *testing.T) {
	r := New()
	defer r.Close()
	got := roundTrip(t, r, "MR100;") // populated by DefaultImage (pmsSlot(1, 'L'))
	if len(got) != 2+memBlockLen+1 {
		t.Fatalf("MR100; reply length = %d, want %d: %q", len(got), 2+memBlockLen+1, got)
	}
}

func TestWithSlot_Overlay(t *testing.T) {
	r := New(WithSlot("060", MemState{
		Freq: "21200000", ClarSign: '+', ClarMag: "0000",
		Mode: '3', Kind: kindMemory, CTCSS: '0', Shift: '0',
	}))
	defer r.Close()
	s, ok := r.SlotState("060")
	if !ok || s.Freq != "21200000" {
		t.Errorf("WithSlot did not overlay channel 060: %+v, ok=%v", s, ok)
	}
	if _, ok := r.SlotState("001"); !ok {
		t.Error("WithSlot dropped the default image's channel 001")
	}
}

func TestAI_AcceptedAndNeverPushedUnsolicited(t *testing.T) {
	r := New()
	defer r.Close()
	writeOnly(t, r, "AI1;")
	if got, want := roundTrip(t, r, "AI;"), []byte("AI1;"); !bytes.Equal(got, want) {
		t.Errorf("AI; = %q, want %q", got, want)
	}
}

func TestUnknownCommand_Rejected(t *testing.T) {
	r := New()
	defer r.Close()
	if got, want := roundTrip(t, r, "MC001;"), rejection; !bytes.Equal(got, want) {
		t.Errorf("MC001; (not implemented, doc.go register entry MC (CURRENT CHANNEL SELECTION) IS NOT IMPLEMENTED) = %q, want %q", got, want)
	}
}
