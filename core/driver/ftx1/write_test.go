// SPDX-License-Identifier: GPL-3.0-or-later

package ftx1

import (
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
)

func populatedChannel(slot string) codeplug.Channel {
	return codeplug.Channel{
		Slot: slot,
		Data: &codeplug.ChannelData{
			FreqHz: 14_250_000,
			Mode:   "USB",
			CTCSS:  "OFF",
			Shift:  "SIMPLEX",
			Tag:    "HOME",
		},
	}
}

func TestWriteChannel_Succeeds(t *testing.T) {
	slot := mustMemorySlot(t, 1)
	p, s := openSession(t, Simulated, slotImage{})
	res, err := s.WriteChannel(testCtx(t), populatedChannel(slot.Wire()))
	if err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	if len(res.Steps) != 2 {
		t.Fatalf("WriteResult.Steps = %+v, want exactly 2 (MW, MT)", res.Steps)
	}
	if res.Steps[0].Command != "MW" || !res.Steps[0].Sent || !res.Steps[0].Confirmed {
		t.Errorf("Steps[0] = %+v, want a Sent+Confirmed MW step", res.Steps[0])
	}
	if res.Steps[1].Command != "MT" || !res.Steps[1].Sent || !res.Steps[1].Confirmed {
		t.Errorf("Steps[1] = %+v, want a Sent+Confirmed MT step", res.Steps[1])
	}

	transcript := p.Transcript()
	if len(transcript) != 4 { // AI0, ID, MW, MT
		t.Fatalf("transcript = %v, want [AI0;, ID;, MW...;, MT...;]", transcript)
	}
	if !strings.HasPrefix(transcript[2], "MW"+slot.Wire()) {
		t.Errorf("MW frame = %q, want it to start %q", transcript[2], "MW"+slot.Wire())
	}
	if !strings.HasPrefix(transcript[3], "MT"+slot.Wire()) {
		t.Errorf("MT frame = %q, want it to start %q", transcript[3], "MT"+slot.Wire())
	}
	if got := len(transcript[3]); got != mtSetFrameLen {
		t.Errorf("MT frame length = %d, want %d (no display byte)", got, mtSetFrameLen)
	}
}

func TestWriteChannel_EmptyChannelRefusesErase(t *testing.T) {
	slot := mustMemorySlot(t, 1)
	_, s := openSession(t, Simulated, slotImage{})
	if _, err := s.WriteChannel(testCtx(t), codeplug.Channel{Slot: slot.Wire()}); err == nil {
		t.Fatal("WriteChannel of an empty channel succeeded, want a refusal — no erase command is documented")
	}
}

func TestWriteChannel_MWRejectedByRadio(t *testing.T) {
	slot := mustMemorySlot(t, 1)
	_, s := openSession(t, Simulated, slotImage{rejectMW: true})
	res, err := s.WriteChannel(testCtx(t), populatedChannel(slot.Wire()))
	if err == nil {
		t.Fatal("WriteChannel succeeded against a radio that rejects every MW Set, want an error")
	}
	if len(res.Steps) != 2 || !res.Steps[0].Sent || res.Steps[0].Confirmed {
		t.Errorf("Steps[0] = %+v, want Sent but not Confirmed (rejected)", res.Steps[0])
	}
	if res.Steps[1].Sent {
		t.Error("Steps[1] (MT) reports Sent after MW was rejected — the sequence must stop")
	}
}

func TestWriteChannel_MTRejectedByRadio(t *testing.T) {
	slot := mustMemorySlot(t, 1)
	_, s := openSession(t, Simulated, slotImage{rejectMT: true})
	res, err := s.WriteChannel(testCtx(t), populatedChannel(slot.Wire()))
	if err == nil {
		t.Fatal("WriteChannel succeeded against a radio that rejects every MT Set, want an error")
	}
	if !res.Steps[0].Sent || !res.Steps[0].Confirmed {
		t.Errorf("Steps[0] (MW) = %+v, want Sent+Confirmed — it must have gone through before MT was attempted", res.Steps[0])
	}
	if !res.Steps[1].Sent || res.Steps[1].Confirmed {
		t.Errorf("Steps[1] (MT) = %+v, want Sent but not Confirmed (rejected)", res.Steps[1])
	}
}

// TestWriteChannel_RealHardwareRefusesWithoutConsent: every write is
// Unverified on the all-Unverified fail-safe, so nothing is writable
// until the caller opts in.
func TestWriteChannel_RealHardwareRefusesWithoutConsent(t *testing.T) {
	slot := mustMemorySlot(t, 1)
	_, s := openSession(t, RealHardware, slotImage{})
	if _, err := s.WriteChannel(testCtx(t), populatedChannel(slot.Wire())); err == nil {
		t.Fatal("WriteChannel succeeded on an unconsented RealHardware session, want a refusal")
	}
}

func TestWriteChannel_RealHardwareConsentedSucceeds(t *testing.T) {
	slot := mustMemorySlot(t, 1)
	_, s := openSession(t, RealHardware, slotImage{}, WithConsentedUnverifiedWrites())
	res, err := s.WriteChannel(testCtx(t), populatedChannel(slot.Wire()))
	if err != nil {
		t.Fatalf("WriteChannel on a consented RealHardware session: %v", err)
	}
	if len(res.Steps) != 2 || !res.Steps[0].Confirmed || !res.Steps[1].Confirmed {
		t.Errorf("WriteResult = %+v, want both steps Confirmed", res.Steps)
	}
}

// TestWriteChannel_5MHzBankRefusesEvenConsented: the 5 MHz/EMGCH banks
// are write-Unsupported structurally (caps.go), which consent cannot
// lift — cat.Dialect.writableSlot excludes them from MW regardless.
func TestWriteChannel_5MHzBankRefusesEvenConsented(t *testing.T) {
	slot, err := dialect.SixtyMSlot(1)
	if err != nil {
		t.Fatalf("SixtyMSlot(1): %v", err)
	}
	_, s := openSession(t, RealHardware, slotImage{}, WithConsentedUnverifiedWrites())
	if _, err := s.WriteChannel(testCtx(t), populatedChannel(slot.Wire())); err == nil {
		t.Fatal("WriteChannel succeeded against the 5 MHz bank even under consent, want a refusal")
	}
}

func TestBuildMWMTCommands_UnknownModeRefuses(t *testing.T) {
	slot := mustMemorySlot(t, 1)
	_, s := openSession(t, Simulated, slotImage{})
	ch := populatedChannel(slot.Wire())
	ch.Data.Mode = "NOT-A-MODE"
	if _, _, err := s.buildMWMTCommands(ch); err == nil {
		t.Fatal("buildMWMTCommands accepted an unknown mode name")
	}
}

// TestBuildMWMTCommands_SixthToneState covers the FTX-1's own P8 byte '5'
// ("REV TONE") on the write side, mirroring read_test.go's read-side pin.
func TestBuildMWMTCommands_SixthToneState(t *testing.T) {
	slot := mustMemorySlot(t, 1)
	_, s := openSession(t, Simulated, slotImage{})
	ch := populatedChannel(slot.Wire())
	ch.Data.CTCSS = "REV-TONE"
	mwCmd, _, err := s.buildMWMTCommands(ch)
	if err != nil {
		t.Fatalf("buildMWMTCommands: %v", err)
	}
	// The CTCSS (P8) byte sits 5 bytes before the frame's end on every
	// registered dialect and on this one alike (dialecttest's own
	// ctcssOffsetFor: frameLen-5) — P8, P9(2), shift(1), ';'(1).
	frame := mwCmd.Bytes()
	if got := frame[len(frame)-5]; got != '5' {
		t.Errorf("MW frame %q carries CTCSS byte %q at offset %d, want '5'", frame, got, len(frame)-5)
	}
}
