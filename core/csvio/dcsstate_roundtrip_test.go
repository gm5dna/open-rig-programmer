// SPDX-License-Identifier: GPL-3.0-or-later

package csvio

import (
	"bytes"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
)

// --- FT-991A Stage 0 (S0.4): a five-state CTCSS vocabulary ---
//
// The NATIVE schema moves the CTCSS state as an opaque string (export.go's
// ctcss column, import.go's cell("ctcss")), so a vocabulary with two DCS
// members needs no change here. This pin is what says so: the states go out
// and come back byte for byte, and nothing in this package normalises them
// against the family three.
//
// The CHIRP direction is deliberately not touched. csvio's CHIRP surface is
// import-only and resolves states BY SEMANTICS, asking only for
// ToneOff/ToneEncode/ToneEncodeDecode, so a DCS state can never be SELECTED
// by an import — which is the property TestImportCHIRP_* already hold.
func TestNativeCSV_RoundTripsADCSState(t *testing.T) {
	channels := []codeplug.Channel{
		{Slot: "001", Data: &codeplug.ChannelData{
			FreqHz: 145500000, Mode: "FM", CTCSS: "DCS-ENC-DEC", Shift: "SIMPLEX",
		}},
		{Slot: "002", Data: &codeplug.ChannelData{
			FreqHz: 145600000, Mode: "FM", CTCSS: "DCS-ENC", Shift: "SIMPLEX",
		}},
		{Slot: "003", Data: &codeplug.ChannelData{
			FreqHz: 145700000, Mode: "FM", CTCSS: "ENC-DEC", Shift: "SIMPLEX",
		}},
	}

	var buf bytes.Buffer
	if err := Export(&buf, channels); err != nil {
		t.Fatalf("Export: %v", err)
	}
	back, err := Import(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(back) != len(channels) {
		t.Fatalf("Import returned %d channels, want %d", len(back), len(channels))
	}
	for i := range channels {
		if back[i].Data == nil {
			t.Fatalf("channel %q came back empty", channels[i].Slot)
		}
		if got, want := back[i].Data.CTCSS, channels[i].Data.CTCSS; got != want {
			t.Errorf("channel %q ctcss = %q, want %q — the native schema carries the state as an opaque string", channels[i].Slot, got, want)
		}
	}
}
