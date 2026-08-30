// SPDX-License-Identifier: GPL-3.0-or-later

package ic7100

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
)

type goldenVector struct {
	name  string
	frame []byte
}

func goldenVectors(t *testing.T) []goldenVector {
	t.Helper()
	path := filepath.Join("testdata", "IC-7100-vectors.golden")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var vectors []goldenVector
	for lineNo, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		name, encoded, ok := strings.Cut(line, "\t")
		if !ok {
			t.Fatalf("%s line %d has no tab separator", path, lineNo+1)
		}
		frame, err := hex.DecodeString(strings.ReplaceAll(strings.TrimSpace(encoded), " ", ""))
		if err != nil {
			t.Fatalf("%s line %d (%s): %v", path, lineNo+1, name, err)
		}
		vectors = append(vectors, goldenVector{name: name, frame: frame})
	}
	return vectors
}

func goldenFrame(t *testing.T, name string) []byte {
	t.Helper()
	for _, vector := range goldenVectors(t) {
		if vector.name == name {
			return append([]byte(nil), vector.frame...)
		}
	}
	t.Fatalf("frozen vector %q is missing", name)
	return nil
}

func goldenNames(t *testing.T) []string {
	t.Helper()
	var names []string
	for _, vector := range goldenVectors(t) {
		names = append(names, vector.name)
	}
	return names
}

func swapGoldenAddresses(frame []byte) []byte {
	out := append([]byte(nil), frame...)
	i := 0
	for i < len(out) && out[i] == civ.PreambleByte {
		i++
	}
	if i+1 < len(out) {
		out[i], out[i+1] = out[i+1], out[i]
	}
	return out
}

func TestGoldenReadRecord(t *testing.T) {
	cmd, err := Profile().BuildMemoryRead(civ.ChannelAddress{Group: 1, Channel: 1})
	if err != nil {
		t.Fatalf("BuildMemoryRead: %v", err)
	}
	// ASSUMED: ic7100-read-request-form; lift: send this exact request and
	// record the reply form. The vector pins the E0 controller spelling.
	if got, want := cmd.Bytes(), goldenFrame(t, "read-record"); !bytes.Equal(got, want) {
		t.Fatalf("read frame = % X\nwant       = % X", got, want)
	}
	if got := cmd.Bytes()[3]; got != 0xe0 {
		t.Errorf("controller byte = %#02x, want E0", got)
	}
}

func TestGoldenTransceiverIDRead(t *testing.T) {
	cmd, err := Profile().BuildTransceiverIDRead()
	if err != nil {
		t.Fatalf("BuildTransceiverIDRead: %v", err)
	}
	if got, want := cmd.Bytes(), goldenFrame(t, "read-transceiver-id"); !bytes.Equal(got, want) {
		t.Fatalf("ID frame = % X, want % X", got, want)
	}
}

func TestGoldenSetRecord(t *testing.T) {
	cmd, err := Profile().BuildMemorySet(knownRecord())
	if err != nil {
		t.Fatalf("BuildMemorySet: %v", err)
	}
	want := goldenFrame(t, "set-record-name-with-space")
	if got := cmd.Bytes(); !bytes.Equal(got, want) {
		t.Fatalf("set frame = % X\nwant      = % X", got, want)
	}
	if got := len(want); got != 121 {
		t.Errorf("set frame length = %d, want 121", got)
	}
	if got := len(want) - 7; got != DataAreaLength {
		t.Errorf("data area length = %d, want %d", got, DataAreaLength)
	}
	dataArea := want[6 : len(want)-1]
	if !bytes.Equal(dataArea[4:51], dataArea[51:98]) {
		t.Error("golden RX data-area bytes 5–51 differ from TX bytes 52–98")
	}
	if got := dataArea[98:]; !bytes.Equal(got, []byte("HOME BASE       ")) {
		t.Errorf("16-byte name = %q, want HOME BASE plus seven spaces", got)
	}
}

func TestGoldenSetRecordParsers(t *testing.T) {
	answer := swapGoldenAddresses(goldenFrame(t, "set-record-name-with-space"))
	addr, raw, err := Profile().MemoryAnswerRecord(answer)
	if err != nil {
		t.Fatalf("MemoryAnswerRecord: %v", err)
	}
	// ASSUMED: ic7100-record-length; lift: read one occupied channel and
	// count record bytes. Raw parsing happens before typed decoding so the
	// future ic7100-all-ff-record empty decision remains possible.
	if len(raw) != RecordLength {
		t.Fatalf("raw record length = %d, want %d", len(raw), RecordLength)
	}
	if addr != (civ.ChannelAddress{Group: 1, Channel: 1}) {
		t.Errorf("address = %+v, want bank 1 channel 1", addr)
	}
	allFF := true
	for _, b := range raw {
		allFF = allFF && b == 0xff
	}
	if allFF {
		t.Fatal("occupied golden unexpectedly has an all-FF record")
	}
	rec, err := Profile().ParseMemoryAnswer(answer)
	if err != nil {
		t.Fatalf("ParseMemoryAnswer: %v", err)
	}
	if rec != knownRecord() {
		t.Errorf("parsed record differs:\n got %+v\nwant %+v", rec, knownRecord())
	}
}

func TestGoldenManualExamplePreservesSkippedIndexSix(t *testing.T) {
	v := goldenFrame(t, "manual-example-1")
	i := 0
	for i < len(v) && v[i] == civ.PreambleByte {
		i++
	}
	if i != 9 {
		t.Fatalf("manual example has %d leading FE bytes, want 9", i)
	}
	// The rendered example labels 18 as index (4), 01 as (5), then FD as
	// (7): printed index (6) is skipped, so no invented data byte belongs.
	if got, want := v[i:], []byte{0x88, 0xe0, 0x18, 0x01, 0xfd}; !bytes.Equal(got, want) {
		t.Errorf("manual example tail = % X, want % X", got, want)
	}
	normalised := v[i-2:]
	if !civ.WellFormed(normalised) {
		t.Fatalf("normalised manual example is not well formed: % X", normalised)
	}
	if Profile().AllowedCommand(normalised) {
		t.Error("gate admitted the manual's power-on example")
	}
}

func TestGoldenEveryVectorReplayed(t *testing.T) {
	want := map[string]bool{
		"read-record": true, "set-record-name-with-space": true,
		"read-transceiver-id": true, "manual-example-1": true,
	}
	for _, name := range goldenNames(t) {
		if !want[name] {
			t.Errorf("frozen vector %q has no replay", name)
		}
		delete(want, name)
	}
	for name := range want {
		t.Errorf("expected frozen vector %q is missing", name)
	}
}
