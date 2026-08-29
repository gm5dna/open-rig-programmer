// SPDX-License-Identifier: GPL-3.0-or-later

package ic7851_test

import (
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7851"
)

func golden(t *testing.T) map[string][]byte {
	t.Helper()
	b, err := os.ReadFile("testdata/IC-7851-vectors.golden")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]byte{}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		v, err := hex.DecodeString(strings.Join(parts[1:], ""))
		if err != nil {
			t.Fatalf("golden %s: %v", parts[0], err)
		}
		out[parts[0]] = v
	}
	return out
}

func TestGoldenFramesAndGate(t *testing.T) {
	v := golden(t)
	p := ic7851.Profile()
	id, err := p.BuildTransceiverIDRead()
	if err != nil {
		t.Fatal(err)
	}
	if string(id.Bytes()) != string(v["read-transceiver-id"]) || !p.AllowedCommand(id.Bytes()) {
		t.Fatal("19 00 golden frame mismatch or refused")
	}
	read, err := p.BuildMemoryRead(civ.ChannelAddress{Channel: 1})
	if err != nil {
		t.Fatal(err)
	}
	if string(read.Bytes()) != string(v["read-record"]) || !p.AllowedCommand(read.Bytes()) {
		t.Fatal("1A 00 read golden frame mismatch or refused")
	}
	set, err := p.BuildMemorySet(civ.MemoryRecord{Address: civ.ChannelAddress{Channel: 1}, RXFreqHz: civ.Available(uint64(14250000)), Mode: civ.Available("USB"), Filter: civ.Available("FIL1"), ToneMode: civ.Available("OFF"), ToneTXDeciHz: civ.Available(uint64(885)), ToneRXDeciHz: civ.Available(uint64(1000)), Name: civ.Available("ALPHA BETA")})
	if err != nil {
		t.Fatal(err)
	}
	if string(set.Bytes()) != string(v["set-record-name-with-space"]) {
		t.Errorf("set frame = % X, want golden % X", set.Bytes(), v["set-record-name-with-space"])
	}
	if !p.AllowedCommand(set.Bytes()) {
		t.Error("golden set frame refused by its own gate")
	}
	for _, frame := range [][]byte{
		{0xfe, 0xfe, 0x8e, 0xe0, 0x09, 0xfd},
		{0xfe, 0xfe, 0x8e, 0xe0, 0x0a, 0xfd},
		{0xfe, 0xfe, 0x8e, 0xe0, 0x0b, 0xfd},
		{0xfe, 0xfe, 0x8e, 0xe0, 0x1a, 0x05, 0xfd},
		{0xfe, 0xfe, 0x8e, 0xe0, 0x1a, 0x00, 0x00, 0x01, 0xff, 0xfd},
	} {
		if p.AllowedCommand(frame) {
			t.Errorf("refused command was admitted: % X", frame)
		}
	}
}
