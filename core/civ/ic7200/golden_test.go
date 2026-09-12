// SPDX-License-Identifier: GPL-3.0-or-later

package ic7200_test

import (
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7200"
)

// golden reads the frozen vectors (freeze_test.go SHA-pins the file).
func golden(t *testing.T) map[string][]byte {
	t.Helper()
	b, err := os.ReadFile("testdata/IC-7200-vectors.golden")
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

// TestGoldenFramesAndGate replays the three frozen vectors through the
// builder, the gate and — where the frame is a memory answer — the
// parser, so all four seams agree byte for byte.
func TestGoldenFramesAndGate(t *testing.T) {
	v := golden(t)
	if len(v) != 3 {
		t.Fatalf("the golden file carries %d vectors, want 3", len(v))
	}
	p := ic7200.Profile()

	id, err := p.BuildTransceiverIDRead()
	if err != nil {
		t.Fatal(err)
	}
	if string(id.Bytes()) != string(v["read-transceiver-id"]) || !p.AllowedCommand(id.Bytes()) {
		t.Fatalf("19 00 = % X, want golden % X (admitted: %v)", id.Bytes(), v["read-transceiver-id"], p.AllowedCommand(id.Bytes()))
	}

	read, err := p.BuildMemoryRead(civ.ChannelAddress{Channel: 1})
	if err != nil {
		t.Fatal(err)
	}
	if string(read.Bytes()) != string(v["read-record"]) || !p.AllowedCommand(read.Bytes()) {
		t.Fatalf("1A 00 read = % X, want golden % X (admitted: %v)", read.Bytes(), v["read-record"], p.AllowedCommand(read.Bytes()))
	}

	set, err := p.BuildMemorySet(legalRecord())
	if err != nil {
		t.Fatal(err)
	}
	if string(set.Bytes()) != string(v["set-record"]) || !p.AllowedCommand(set.Bytes()) {
		t.Fatalf("1A 00 set = % X, want golden % X (admitted: %v)", set.Bytes(), v["set-record"], p.AllowedCommand(set.Bytes()))
	}

	// The set frame, replayed as an ANSWER (from the radio, to the
	// controller): the codec builds no answers, so the envelope is
	// rewritten here from the set vector's own record bytes.
	answer := append([]byte{0xfe, 0xfe, 0xe0, 0x76, 0x1a, 0x00}, v["set-record"][6:len(v["set-record"])-1]...)
	answer = append(answer, 0xfd)
	addr, raw, err := p.MemoryAnswerRecord(answer)
	if err != nil {
		t.Fatalf("MemoryAnswerRecord: %v", err)
	}
	if addr != (civ.ChannelAddress{Channel: 1}) {
		t.Errorf("MemoryAnswerRecord address = %v, want channel 1", addr)
	}
	if len(raw) != ic7200.RecordOnlyLength {
		t.Errorf("MemoryAnswerRecord raw length = %d, want %d", len(raw), ic7200.RecordOnlyLength)
	}
	rec, err := p.ParseMemoryAnswer(answer)
	if err != nil {
		t.Fatalf("ParseMemoryAnswer: %v", err)
	}
	want := legalRecord()
	if rec.RXFreqHz != want.RXFreqHz || rec.TXFreqHz != want.TXFreqHz || rec.Mode != want.Mode || rec.Filter != want.Filter || rec.DataMode != want.DataMode {
		t.Errorf("ParseMemoryAnswer = %+v, want fields matching %+v", rec, want)
	}
}
