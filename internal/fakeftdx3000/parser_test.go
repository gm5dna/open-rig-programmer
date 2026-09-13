// SPDX-License-Identifier: GPL-3.0-or-later

package fakeftdx3000

import (
	"bytes"
	"testing"
)

// wireRoundTrip writes req to r's port and returns whatever comes back,
// or nil if nothing arrives within the read (the caller must know
// whether a reply is expected).
func wireExchange(t *testing.T, r *Radio, req string) []byte {
	t.Helper()
	port := r.Port()
	if _, err := port.Write([]byte(req)); err != nil {
		t.Fatalf("Write %q: %v", req, err)
	}
	buf := make([]byte, 64)
	n, err := port.Read(buf)
	if err != nil {
		t.Fatalf("Read after %q: %v", req, err)
	}
	return buf[:n]
}

func TestReassembler_SplitAcrossWrites(t *testing.T) {
	a := newReassembler(256)
	if evs := a.push([]byte("I")); len(evs) != 0 {
		t.Fatalf("push(%q) produced %d events, want 0", "I", len(evs))
	}
	evs := a.push([]byte("D;"))
	if len(evs) != 1 || string(evs[0].frame) != "ID;" {
		t.Fatalf("push(%q) = %+v, want one frame \"ID;\"", "D;", evs)
	}
}

func TestReassembler_OverflowThenResync(t *testing.T) {
	a := newReassembler(4)
	evs := a.push([]byte("TOOLONG"))
	if len(evs) != 1 || !evs[0].overflow {
		t.Fatalf("push(overlong) = %+v, want one overflow event", evs)
	}
	// Bytes up to and including the next ';' are discarded; framing
	// resumes cleanly after it.
	evs = a.push([]byte("garbage;ID;"))
	if len(evs) != 1 || string(evs[0].frame) != "ID;" {
		t.Fatalf("push after overflow = %+v, want one frame \"ID;\"", evs)
	}
}

func TestHandleFrame_UnknownCommandRejected(t *testing.T) {
	r := New()
	defer r.Close()
	if got := wireExchange(t, r, "ZZ;"); !bytes.Equal(got, rejection) {
		t.Errorf("ZZ; = %q, want %q", got, rejection)
	}
}

func TestHandleFrame_CommandNameCaseInsensitive(t *testing.T) {
	r := New()
	defer r.Close()
	if got := wireExchange(t, r, "id;"); !bytes.Equal(got, []byte("ID0462;")) {
		t.Errorf("id; = %q, want %q", got, "ID0462;")
	}
}

func TestMR_EmptySlotRejected(t *testing.T) {
	r := New()
	defer r.Close()
	if got := wireExchange(t, r, "MR050;"); !bytes.Equal(got, rejection) {
		t.Errorf("MR050; (unpopulated) = %q, want %q", got, rejection)
	}
}

func TestMR_OutOfDomainSlotRejected(t *testing.T) {
	r := New()
	defer r.Close()
	if got := wireExchange(t, r, "MR118;"); !bytes.Equal(got, rejection) {
		t.Errorf("MR118; (outside 001-117) = %q, want %q", got, rejection)
	}
}

func TestMR_Channel000Rejected(t *testing.T) {
	// MC's own legend admits "000" as a Regular Memory Channel, but
	// MW/MR's own P1 cells both print "(001 117)" — the erratum this
	// fake follows MW/MR on (doc.go register entry 2, matrix §2.4).
	r := New()
	defer r.Close()
	if got := wireExchange(t, r, "MR000;"); !bytes.Equal(got, rejection) {
		t.Errorf("MR000; = %q, want %q", got, rejection)
	}
}

// --- MW/MR round trip and the P9 asymmetry ---

func memBlockBytes(slot, freq string, clarSign byte, clarMag string, rx, tx bool, mode byte, ctcss byte, tone string, shift byte) []byte {
	var b []byte
	b = append(b, slot...)
	b = append(b, freq...)
	b = append(b, clarSign)
	b = append(b, clarMag...)
	b = append(b, boolFlagByte(rx))
	b = append(b, boolFlagByte(tx))
	b = append(b, mode)
	b = append(b, mwSetKindFixed)
	b = append(b, ctcss)
	b = append(b, tone...)
	b = append(b, shift)
	return b
}

func TestMW_AcceptsToneZeroZero_ThenMRReadsItBack(t *testing.T) {
	r := New()
	defer r.Close()

	body := memBlockBytes("050", "07123456", '+', "0000", false, false, modeUSB, '0', "00", '0')
	frame := append([]byte("MW"), body...)
	frame = append(frame, ';')

	port := r.Port()
	if _, err := port.Write(frame); err != nil {
		t.Fatalf("Write MW: %v", err)
	}
	// Fire-and-forget: prove acceptance via a subsequent MR, not a reply
	// to the MW itself.
	got := wireExchange(t, r, "MR050;")
	// The Answer's Kind byte is always kindMemory ('1'), never the Set's
	// fixed placeholder ('0') — doc.go's register entry AN ANSWER'S KIND
	// BYTE IS ALWAYS '1'.
	wantBody := append([]byte(nil), body...)
	wantBody[blkKind] = kindMemory
	want := append(append([]byte("MR"), wantBody...), ';')
	if !bytes.Equal(got, want) {
		t.Errorf("MR050; after MW = %q, want %q", got, want)
	}
}

func TestMW_RefusesNonzeroTone(t *testing.T) {
	// The headline wrinkle (matrix §1.3, doc.go register entry TONE
	// INDEX (P9)): MW's own block prints "P9: 0: (Fixed)" — a live
	// two-digit tone is a READ-side fact only, never an accepted Set.
	r := New()
	defer r.Close()

	body := memBlockBytes("051", "07000000", '+', "0000", false, false, modeUSB, '0', "05", '0')
	frame := append([]byte("MW"), body...)
	frame = append(frame, ';')

	if got := wireExchange(t, r, string(frame)); !bytes.Equal(got, rejection) {
		t.Errorf("MW with nonzero P9 = %q, want %q", got, rejection)
	}
	// And the slot must not have been created.
	if got := wireExchange(t, r, "MR051;"); !bytes.Equal(got, rejection) {
		t.Errorf("MR051; after a refused MW = %q, want %q (slot must stay empty)", got, rejection)
	}
}

func TestMR_ReadsALiveNonzeroToneFromAFixture(t *testing.T) {
	// A stored slot's tone can only be nonzero via WithSlot — never via
	// an accepted MW — and MR must answer it live, unmodified.
	fixture := MemState{
		Freq: "07000000", ClarSign: '+', ClarMag: "0000",
		Mode: modeUSB, Kind: kindMemory, CTCSS: '1', Tone: "23", Shift: '0',
	}
	r := New(WithSlot("052", fixture))
	defer r.Close()

	got := wireExchange(t, r, "MR052;")
	if !bytes.Contains(got, []byte("23")) {
		t.Errorf("MR052; = %q, want it to carry the live tone index \"23\"", got)
	}

	// Now try to write channel 052 back with that SAME nonzero tone: it
	// must be refused, and the fixture's live tone must survive
	// untouched.
	body := memBlockBytes("052", "07000000", '+', "0000", false, false, modeUSB, '1', "23", '0')
	frame := append(append([]byte("MW"), body...), ';')
	if got := wireExchange(t, r, string(frame)); !bytes.Equal(got, rejection) {
		t.Errorf("MW re-asserting the fixture's own nonzero tone = %q, want %q", got, rejection)
	}
	if got := wireExchange(t, r, "MR052;"); !bytes.Contains(got, []byte("23")) {
		t.Errorf("MR052; after the refused MW = %q, want the fixture's tone \"23\" untouched", got)
	}
}

// --- MC ---

func TestMC_ReadBeforeAnySet(t *testing.T) {
	r := New()
	defer r.Close()
	if got := wireExchange(t, r, "MC;"); !bytes.Equal(got, []byte("MC000;")) {
		t.Errorf("MC; before any set = %q, want %q", got, "MC000;")
	}
}

func TestMC_SetChannel000Refused(t *testing.T) {
	// MC's own legend prints 000 as a nameable channel, but this fake
	// follows MW/MR's narrower span (doc.go register entry 2).
	r := New()
	defer r.Close()
	if got := wireExchange(t, r, "MC000;"); !bytes.Equal(got, rejection) {
		t.Errorf("MC000; (set) = %q, want %q", got, rejection)
	}
}

func TestMC_SetPopulatedChannelThenRead(t *testing.T) {
	r := New()
	defer r.Close()
	// "001" is populated by DefaultImage.
	port := r.Port()
	if _, err := port.Write([]byte("MC001;")); err != nil {
		t.Fatalf("Write MC001;: %v", err)
	}
	if got := wireExchange(t, r, "MC;"); !bytes.Equal(got, []byte("MC001;")) {
		t.Errorf("MC; after MC001; = %q, want %q", got, "MC001;")
	}
}

func TestMC_SetEmptyChannelRejectedAndDoesNotMoveSelection(t *testing.T) {
	r := New()
	defer r.Close()
	if got := wireExchange(t, r, "MC099;"); !bytes.Equal(got, rejection) {
		t.Errorf("MC099; (unpopulated) = %q, want %q", got, rejection)
	}
	if got := wireExchange(t, r, "MC;"); !bytes.Equal(got, []byte("MC000;")) {
		t.Errorf("MC; after a refused MC-set = %q, want the sentinel %q unchanged", got, "MC000;")
	}
}

// --- Field validators ---

func TestValidCTCSSByte(t *testing.T) {
	for _, b := range []byte("012") {
		if !validCTCSSByte(b) {
			t.Errorf("validCTCSSByte(%q) = false, want true", b)
		}
	}
	if validCTCSSByte('3') {
		t.Error("validCTCSSByte('3') = true, want false — only 0-2 are printed (matrix §2.9)")
	}
}

func TestValidShiftByte(t *testing.T) {
	for _, b := range []byte("012") {
		if !validShiftByte(b) {
			t.Errorf("validShiftByte(%q) = false, want true", b)
		}
	}
	if validShiftByte('3') {
		t.Error("validShiftByte('3') = true, want false")
	}
}

func TestParseSlotForm(t *testing.T) {
	tests := []struct {
		slot string
		want slotKind
	}{
		{"001", slotMemory},
		{"099", slotMemory},
		{"100", slotPMS},
		{"117", slotPMS},
		{"000", slotInvalid},
		{"118", slotInvalid},
		{"0AB", slotInvalid},
	}
	for _, tt := range tests {
		if got := parseSlotForm(tt.slot); got != tt.want {
			t.Errorf("parseSlotForm(%q) = %v, want %v", tt.slot, got, tt.want)
		}
	}
}
