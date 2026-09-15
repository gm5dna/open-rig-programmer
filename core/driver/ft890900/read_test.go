// SPDX-License-Identifier: GPL-3.0-or-later

package ft890900

import (
	"context"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/bincat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
)

// buildRecordBytes assembles a 19-byte VFO/Memory Data Record matching
// profile.go's offsets, for test fixtures only.
func buildRecordBytes(blanked bool, freqTensOfHz uint32, clarHz int16, mode, tone, flags byte) []byte {
	rec := make([]byte, 19)
	if blanked {
		rec[0] = 0x80
	}
	rec[freqOffset] = byte(freqTensOfHz >> 16)
	rec[freqOffset+1] = byte(freqTensOfHz >> 8)
	rec[freqOffset+2] = byte(freqTensOfHz)
	rec[clarOffset] = byte(uint16(clarHz) >> 8)
	rec[clarOffset+1] = byte(uint16(clarHz))
	rec[modeOffset] = mode
	rec[toneOffset] = tone
	rec[flagsOffset] = flags
	return rec
}

func openFakeFT890(t *testing.T, p *scriptedPort) driver.Session {
	t.Helper()
	p.setAnswer(statusUpdateFrame(probeChannel1), blankRecord())
	sess, err := NewFT890(RealHardware).Open(context.Background(), p.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	p.ResetTranscript() // isolate the caller's own frames from Open's two identity probes
	return sess
}

func TestReadChannel_DecodesAPopulatedRecord(t *testing.T) {
	p := newScriptedPort(t)
	sess := openFakeFT890(t, p)

	rec := buildRecordBytes(false, 1_425_000, 0, bincat.ModeUSB, 0x05, 0)
	p.setAnswer(statusUpdateFrame(5), rec)

	ch, err := sess.ReadChannel(context.Background(), "005")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if ch.Empty() {
		t.Fatal("ReadChannel returned an empty channel, want populated")
	}
	if ch.Data.FreqHz != 14_250_000 {
		t.Errorf("FreqHz = %d, want 14250000", ch.Data.FreqHz)
	}
	if ch.Data.Mode != "USB" {
		t.Errorf("Mode = %q, want USB", ch.Data.Mode)
	}
	if ch.Data.Shift != "SIMPLEX" {
		t.Errorf("Shift = %q, want SIMPLEX", ch.Data.Shift)
	}
	if ch.Data.CTCSSTone.State != codeplug.Known || ch.Data.CTCSSTone.Value != standardTones[5] {
		t.Errorf("CTCSSTone = %+v, want Known %v", ch.Data.CTCSSTone, standardTones[5])
	}
	if ch.Data.OffsetHz.State != codeplug.Unavailable {
		t.Errorf("OffsetHz.State = %v, want Unavailable (no wire route reads it back)", ch.Data.OffsetHz.State)
	}
}

func TestReadChannel_BlankedRecordIsEmptyChannel(t *testing.T) {
	p := newScriptedPort(t)
	sess := openFakeFT890(t, p)

	p.setAnswer(statusUpdateFrame(9), buildRecordBytes(true, 1_000_000, 0, bincat.ModeFM, 0, 0))

	ch, err := sess.ReadChannel(context.Background(), "009")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if !ch.Empty() {
		t.Errorf("ReadChannel = %+v, want an empty (Blanked) channel", ch)
	}
}

func TestReadChannel_RefusesASlotOutsideTheTrueRange(t *testing.T) {
	p := newScriptedPort(t)
	sess := openFakeFT890(t, p)

	if _, err := sess.ReadChannel(context.Background(), "033"); err == nil {
		t.Error("ReadChannel(\"033\") on an FT-890 (true range 001-032) succeeded, want an error")
	}
	// No wire traffic at all: the refusal is local.
	if got := len(p.Transcript()); got != 0 {
		t.Errorf("frames sent = %d, want 0 (the refusal is local, before any wire traffic)", got)
	}
}
