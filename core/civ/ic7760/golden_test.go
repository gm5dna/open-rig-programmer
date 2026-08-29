// SPDX-License-Identifier: GPL-3.0-or-later

package ic7760_test

import (
	"bytes"
	"encoding/hex"
	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7760"
	"os"
	"strings"
	"testing"
)

func goldenVectors(t *testing.T) map[string][]byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/IC-7760-vectors.golden")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]byte{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			t.Fatalf("malformed golden line %q", line)
		}
		fields := strings.Fields(parts[1])
		b := make([]byte, len(fields))
		for i, field := range fields {
			decoded, err := hex.DecodeString(field)
			if err != nil || len(decoded) != 1 {
				t.Fatalf("golden %s token %q: %v", parts[0], field, err)
			}
			b[i] = decoded[0]
		}
		out[parts[0]] = b
	}
	return out
}

func goldenRecord() civ.MemoryRecord {
	return civ.MemoryRecord{Address: civ.ChannelAddress{Channel: 1}, RXFreqHz: civ.Available[uint64](14_100_000), Mode: civ.Available("USB"), Filter: civ.Available("FIL1"), ToneMode: civ.Available("OFF"), ToneTXDeciHz: civ.Available[uint64](885), ToneRXDeciHz: civ.Available[uint64](885), Name: civ.Available("ALPHA BETA")}
}

func TestGolden(t *testing.T) {
	v := goldenVectors(t)
	p := ic7760.Profile()
	rd, err := p.BuildMemoryRead(civ.ChannelAddress{Channel: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(rd.Bytes(), v["read-record"]) {
		t.Errorf("read = % X, want % X", rd.Bytes(), v["read-record"])
	}
	set, err := p.BuildMemorySet(goldenRecord())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(set.Bytes(), v["set-record-name-with-space"]) {
		t.Errorf("set = % X, want % X", set.Bytes(), v["set-record-name-with-space"])
	}
	answer := append([]byte(nil), set.Bytes()...)
	answer[2], answer[3] = answer[3], answer[2]
	got, err := p.ParseMemoryAnswer(answer)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != goldenRecord().Name || got.RXFreqHz != goldenRecord().RXFreqHz {
		t.Errorf("golden round trip = %+v", got)
	}
	id, err := p.BuildTransceiverIDRead()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(id.Bytes(), v["read-transceiver-id"]) {
		t.Errorf("id = % X, want % X", id.Bytes(), v["read-transceiver-id"])
	}
	for _, token := range []byte{0x00, 0x7a, 0xff} {
		frame := []byte{0xfe, 0xfe, 0xe0, 0xb2, 0x19, 0x00, token, 0xfd}
		parsed, err := p.ParseTransceiverID(frame)
		if err != nil || parsed != hex.EncodeToString([]byte{token}) {
			t.Errorf("ID token %#02x parsed %q, err %v", token, parsed, err)
		}
	}
	if _, err := p.ParseMemoryAnswer(append(answer[:len(answer)-2], 0xfd)); err == nil {
		t.Error("wrong record length accepted")
	}
	if !p.AllowedCommand(v["read-transceiver-id"]) {
		t.Error("documented diagnostic ID read was refused")
	}
}
