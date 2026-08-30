// SPDX-License-Identifier: GPL-3.0-or-later

package ic7760_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7760"
	"os"
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
	if want := goldenRecord(); got != want {
		t.Errorf("golden parse did not compare field-for-field:\n got %+v\nwant %+v", got, want)
	}
	reencoded, err := p.BuildMemorySet(got)
	if err != nil {
		t.Fatalf("re-encode parsed golden: %v", err)
	}
	if !bytes.Equal(reencoded.Bytes(), v["set-record-name-with-space"]) {
		t.Errorf("re-encoded parsed golden = % X, want byte-identical % X", reencoded.Bytes(), v["set-record-name-with-space"])
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
	if _, err := p.ParseMemoryAnswer(append(answer[:len(answer)-2], 0xfd)); !errors.Is(err, civ.ErrRecordLength) {
		t.Errorf("wrong record length error = %v, want ErrRecordLength", err)
	}
	if !p.AllowedCommand(v["read-transceiver-id"]) {
		t.Error("documented diagnostic ID read was refused")
	}
}

func TestGoldenNegativeVectors(t *testing.T) {
	p := ic7760.Profile()
	set := append([]byte(nil), goldenVectors(t)["set-record-name-with-space"]...)
	answer := append([]byte(nil), set...)
	answer[2], answer[3] = answer[3], answer[2]

	t.Run("wrong mode enum", func(t *testing.T) {
		bad := append([]byte(nil), answer...)
		bad[14] = 0x06 // record offset 6; 06 is absent from the printed mode enum.
		if _, err := p.ParseMemoryAnswer(bad); err == nil || !strings.Contains(err.Error(), "mode") {
			t.Errorf("ParseMemoryAnswer error = %v, want mode-enum refusal", err)
		}
	})
	t.Run("wrong data enum", func(t *testing.T) {
		bad := append([]byte(nil), set...)
		bad[16] = 0x40 // record offset 8 high nibble; the printed domain ends at 3.
		if p.AllowedCommand(bad) {
			t.Error("gate admitted data-mode nibble 4")
		}
	})
	t.Run("unsupported fields", func(t *testing.T) {
		for name, mutate := range map[string]func(*civ.MemoryRecord){
			"TX frequency": func(r *civ.MemoryRecord) { r.TXFreqHz = civ.Available[uint64](14_200_000) },
			"data mode":    func(r *civ.MemoryRecord) { r.DataMode = civ.Available("DATA 1") },
			"select":       func(r *civ.MemoryRecord) { r.Select = civ.Available("★1") },
		} {
			t.Run(name, func(t *testing.T) {
				rec := goldenRecord()
				mutate(&rec)
				if cmd, err := p.BuildMemorySet(rec); err == nil || !cmd.IsZero() {
					t.Errorf("BuildMemorySet = % X, %v; want zero command and refusal", cmd.Bytes(), err)
				}
			})
		}
	})
	t.Run("clear forms", func(t *testing.T) {
		forms := [][]byte{
			{0xFE, 0xFE, 0xB2, 0xE0, 0x0B, 0xFD},
			{0xFE, 0xFE, 0xB2, 0xE0, 0x1A, 0x00, 0x00, 0x01, 0xFF, 0xFD},
		}
		for _, frame := range forms {
			if p.AllowedCommand(frame) {
				t.Errorf("gate admitted clear frame % X", frame)
			}
		}
	})
	t.Run("malformed names", func(t *testing.T) {
		for _, name := range []string{"ELEVENCHARS", "BAD\nNAME"} {
			rec := goldenRecord()
			rec.Name = civ.Available(name)
			if _, err := p.BuildMemorySet(rec); err == nil {
				t.Errorf("BuildMemorySet accepted malformed name %q", name)
			}
		}
		bad := append([]byte(nil), answer...)
		bad[23] = 0x00 // record offset 15, first name byte.
		if _, err := p.ParseMemoryAnswer(bad); err == nil || !strings.Contains(err.Error(), "name") {
			t.Errorf("ParseMemoryAnswer error = %v, want malformed-name refusal", err)
		}
	})

	// The frozen vector itself is the final negative-vector control: it must
	// remain accepted exactly as written after all mutations above.
	if !p.AllowedCommand(set) {
		t.Error("the unchanged frozen set vector was refused")
	}
	// 22 = the tier's 15 plus Erratum 6's seven receiver fields
	// (tuning step enable/code, programmable step, attenuator, preamp,
	// antenna, IP+), which this profile leaves unmapped: the whole-struct
	// golden comparison above still covers them, as an unexpectedly
	// decoded value would break equality against the zero Optionals.
	if got := goldenRecord(); reflect.ValueOf(got).NumField() != 22 {
		t.Fatalf("MemoryRecord field count = %d, want 22; update the full-field golden comparison for new fields", reflect.ValueOf(got).NumField())
	}
}
