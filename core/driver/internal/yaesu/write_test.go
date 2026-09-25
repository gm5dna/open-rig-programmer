// SPDX-License-Identifier: GPL-3.0-or-later

package yaesu

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	catft991a "github.com/gm5dna/open-rig-programmer/core/cat/ft991a"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// TestBuildMWCommand_ZeroMaxFreqHzIsNoBound pins the fleet convention
// "MaxFreqHz 0 = no bound" (codeplug/validate.go, ic905/write.go) for this
// package's own frequency-range check: a caps value that never sets
// MaxFreqHz (ftx1's own caps.go, "no bound" by its own comment) must not
// refuse an otherwise-valid write. All seven drivers migrated onto this
// body set MaxFreqHz non-zero, so they are unaffected by this branch.
func TestBuildMWCommand_ZeroMaxFreqHzIsNoBound(t *testing.T) {
	d := catft991a.Dialect()
	slot, err := d.MemorySlot(1)
	if err != nil {
		t.Fatalf("MemorySlot(1): %v", err)
	}
	p := &MRParams{
		Name:        "test",
		MRAnswerLen: 27,
		CTCSS:       CTCSSVocab3,
		WriteKind:   func(cat.Dialect) byte { return d.MWWriteKind() },
	}
	ch := codeplug.Channel{
		Slot: slot.Wire(),
		Data: &codeplug.ChannelData{
			FreqHz: 14_250_000,
			Mode:   "USB",
			CTCSS:  "OFF",
			Shift:  "SIMPLEX",
		},
	}
	if _, err := BuildMWCommand(d, spec.Capabilities{}, p, ch); err != nil {
		t.Fatalf("BuildMWCommand with caps.MaxFreqHz==0 refused a valid frequency, want no bound: %v", err)
	}
}

// TestToneIndexRoundTrip pins ToneForIndex/IndexForTone as exact inverses
// over the whole 50-entry chart (formerly duplicated per-driver in
// ft450d/ft950/ftdx9000; one copy here covers all three's shared body).
func TestToneIndexRoundTrip(t *testing.T) {
	for i := 0; i < 50; i++ {
		tone, ok := ToneForIndex(uint8(i))
		if !ok {
			t.Fatalf("ToneForIndex(%d) refused, want ok", i)
		}
		idx, ok := IndexForTone(tone)
		if !ok || idx != uint8(i) {
			t.Errorf("IndexForTone(ToneForIndex(%d)) = %d, %v, want %d, true", i, idx, ok, i)
		}
	}
	if _, ok := ToneForIndex(50); ok {
		t.Error("ToneForIndex(50) succeeded, want refused (chart is 0-49)")
	}
}
