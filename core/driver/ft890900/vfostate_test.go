// SPDX-License-Identifier: GPL-3.0-or-later

package ft890900

import (
	"context"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/bincat"
	"github.com/gm5dna/open-rig-programmer/core/clone"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
)

// buildSubRecordBytes assembles a bare 9-byte VFO/Memory Data Record (no
// leading Memory Status Flags byte) — the shape U=UBothVFOs' 18-byte
// reply carries twice.
func buildSubRecordBytes(freqTensOfHz uint32, clarHz int16, mode, tone, flags byte) [9]byte {
	var sub [9]byte
	sub[1] = byte(freqTensOfHz >> 16)
	sub[2] = byte(freqTensOfHz >> 8)
	sub[3] = byte(freqTensOfHz)
	sub[4] = byte(uint16(clarHz) >> 8)
	sub[5] = byte(uint16(clarHz))
	sub[6] = mode
	sub[7] = tone
	sub[8] = flags
	return sub
}

func bothVFOsFrame() [5]byte { return [5]byte{bincat.UBothVFOs, 0, 0, 0, bincat.OpStatusUpdate} }
func currentOpFrame() [5]byte {
	return [5]byte{bincat.UOperatingData, 0, 0, 0, bincat.OpStatusUpdate}
}

func TestSnapshotVFOState_CurrentOpMatchesVFOA_ReportsActiveA(t *testing.T) {
	p := newScriptedPort(t)
	sess := openFakeSimulatedFT890(t, p)
	restorer := sess.(clone.VFOStateRestorer)

	vfoA := buildSubRecordBytes(710_000, 0, bincat.ModeLSB, 3, 0)
	vfoB := buildSubRecordBytes(1_000_000, 0, bincat.ModeUSB, 7, 0)
	p.setAnswer(bothVFOsFrame(), append(append([]byte{}, vfoA[:]...), vfoB[:]...))
	current := make([]byte, 19)
	copy(current[1:10], vfoA[:])
	p.setAnswer(currentOpFrame(), current)

	snap, err := restorer.SnapshotVFOState(context.Background())
	if err != nil {
		t.Fatalf("SnapshotVFOState: %v", err)
	}
	if snap.ActiveVFO != "A" {
		t.Errorf("ActiveVFO = %q, want \"A\"", snap.ActiveVFO)
	}
	if snap.Content.FreqHz != 7_100_000 || snap.Content.Mode != "LSB" {
		t.Errorf("Content = %+v, want FreqHz=7100000 Mode=LSB", snap.Content)
	}
}

func TestSnapshotVFOState_CurrentOpMatchesVFOB_ReportsActiveB(t *testing.T) {
	p := newScriptedPort(t)
	sess := openFakeSimulatedFT890(t, p)
	restorer := sess.(clone.VFOStateRestorer)

	vfoA := buildSubRecordBytes(710_000, 0, bincat.ModeLSB, 3, 0)
	vfoB := buildSubRecordBytes(1_000_000, 0, bincat.ModeUSB, 7, 0)
	p.setAnswer(bothVFOsFrame(), append(append([]byte{}, vfoA[:]...), vfoB[:]...))
	current := make([]byte, 19)
	copy(current[1:10], vfoB[:])
	p.setAnswer(currentOpFrame(), current)

	snap, err := restorer.SnapshotVFOState(context.Background())
	if err != nil {
		t.Fatalf("SnapshotVFOState: %v", err)
	}
	if snap.ActiveVFO != "B" {
		t.Errorf("ActiveVFO = %q, want \"B\" (current-op matched VFO-B's content, not VFO-A's)", snap.ActiveVFO)
	}
}

func TestRestoreVFOState_ReplaysContentAndReselectsB(t *testing.T) {
	p := newScriptedPort(t)
	sess := openFakeSimulatedFT890(t, p)
	restorer := sess.(clone.VFOStateRestorer)

	snap := clone.VFOSnapshot{
		Content: codeplug.ChannelData{
			FreqHz: 14_250_000, Mode: "USB", Shift: "SIMPLEX",
			CTCSSTone: codeplug.ToneField{State: codeplug.Known, Value: standardTones[2]},
			OffsetHz:  codeplug.FreqField{State: codeplug.Unknown},
		},
		ActiveVFO: "B",
	}
	if err := restorer.RestoreVFOState(context.Background(), snap); err != nil {
		t.Fatalf("RestoreVFOState: %v", err)
	}

	got := p.Transcript()
	wantOpcodes := []byte{bincat.OpABSelect, bincat.OpSetFreq, bincat.OpSetMode, bincat.OpClarifier, bincat.OpShift, bincat.OpTone, bincat.OpABSelect}
	if len(got) != len(wantOpcodes) {
		t.Fatalf("frames sent = %v, want %d frames with opcodes %v", got, len(wantOpcodes), wantOpcodes)
	}
	for i, op := range wantOpcodes {
		if got[i][4] != op {
			t.Errorf("frame %d opcode = %#02x, want %#02x", i, got[i][4], op)
		}
	}
	// The final frame re-selects VFO-B (V=1).
	if got[len(got)-1][0] != 1 {
		t.Errorf("final A/B-select arg = %d, want 1 (VFO-B)", got[len(got)-1][0])
	}
}
