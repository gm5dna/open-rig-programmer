// SPDX-License-Identifier: GPL-3.0-or-later

package ic7700_test

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7700"
)

// This file compares this dialect's codec against the hand-derived wire
// frames in testdata/IC-7700-vectors.golden.
//
// PROVENANCE, STATED PLAINLY: unlike the earlier-landed models in this
// tree (ic7610, ic7300, ic7851), whose golden vectors came from an
// independent QUARANTINED reading, this wave's single-implementer brief
// does not fund a second blind leg. These vectors were derived by hand
// from the reviewed capability matrix's own citations
// (docs/superpowers/icom-matrices/ic7700-capability-matrix.md §3.11,
// §3.15) — the same evidence profile.go was built from — and are
// deliberately built from all-zero-or-trivial field values (frequency 0,
// mode LSB, filter FIL1, tone OFF) so that every byte's expected value
// follows from arithmetic a reader can check by eye, without needing to
// re-derive packed-BCD digit math this wave has no second reading to
// arbitrate against. A mismatch here is still a STOP, not a fix-the-vector
// exercise.
//
// Hardware status: UNVERIFIED. No IC-7700 has ever answered a frame.

const goldenFile = "IC-7700-vectors.golden"

const (
	vecReadRecord = "read-record"         // 9 bytes  - BuildMemoryRead(1)
	vecReadID     = "read-transceiver-id" // 7 bytes  - BuildTransceiverIDRead()
	vecSetRecord  = "set-record-minimal"  // 48 bytes - BuildMemorySet(goldenRecord())
)

func loadGoldenVectors(t *testing.T) map[string][]byte {
	t.Helper()
	path := filepath.Join("testdata", goldenFile)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	text := strings.TrimSuffix(string(raw), "\n")
	lines := strings.Split(text, "\n")
	if len(lines) != 3 {
		t.Fatalf("%s carries %d lines, want exactly 3", path, len(lines))
	}
	out := make(map[string][]byte, 3)
	for i, line := range lines {
		name, hexes, ok := strings.Cut(line, "\t")
		if !ok {
			t.Fatalf("%s line %d is not <name><TAB><hex bytes>: %q", path, i+1, line)
		}
		if _, dup := out[name]; dup {
			t.Fatalf("%s names %q twice", path, name)
		}
		var frame []byte
		for _, tok := range strings.Split(hexes, " ") {
			b, err := hex.DecodeString(tok)
			if err != nil || len(tok) != 2 {
				t.Fatalf("%s line %d: token %q: %v", path, i+1, tok, err)
			}
			frame = append(frame, b[0])
		}
		out[name] = frame
	}
	return out
}

// goldenRecord is the set-record-minimal vector's field content, stated as
// literals rather than parsed out of the frame (a test that only rebuilt
// what it just parsed would prove the codec self-consistent and nothing
// else).
func goldenRecord() civ.MemoryRecord {
	return civ.MemoryRecord{
		Address:      civ.ChannelAddress{Channel: 1},
		RXFreqHz:     civ.Available[uint64](0),
		TXFreqHz:     civ.Available[uint64](0), // idx15-19, mirrored RX per this wave's write policy — this is a civ-level literal, not a driver decision
		Mode:         civ.Available("LSB"),
		Filter:       civ.Available("FIL1"),
		ToneMode:     civ.Available("OFF"),
		ToneTXDeciHz: civ.Available[uint64](0),
		ToneRXDeciHz: civ.Available[uint64](0),
		Name:         civ.Available("TESTCH"),
	}
}

func TestGolden_ReadRecord(t *testing.T) {
	vecs := loadGoldenVectors(t)
	cmd, err := ic7700.Profile().BuildMemoryRead(civ.ChannelAddress{Channel: 1})
	if err != nil {
		t.Fatalf("BuildMemoryRead: %v", err)
	}
	got := cmd.Bytes()
	want := vecs[vecReadRecord]
	if hex.EncodeToString(got) != hex.EncodeToString(want) {
		t.Errorf("BuildMemoryRead(1) = % X, want % X", got, want)
	}
}

func TestGolden_ReadTransceiverID(t *testing.T) {
	vecs := loadGoldenVectors(t)
	cmd, err := ic7700.Profile().BuildTransceiverIDRead()
	if err != nil {
		t.Fatalf("BuildTransceiverIDRead: %v", err)
	}
	got := cmd.Bytes()
	want := vecs[vecReadID]
	if hex.EncodeToString(got) != hex.EncodeToString(want) {
		t.Errorf("BuildTransceiverIDRead() = % X, want % X", got, want)
	}
}

func TestGolden_SetRecord(t *testing.T) {
	vecs := loadGoldenVectors(t)
	cmd, err := ic7700.Profile().BuildMemorySet(goldenRecord())
	if err != nil {
		t.Fatalf("BuildMemorySet: %v", err)
	}
	got := cmd.Bytes()
	want := vecs[vecSetRecord]
	if hex.EncodeToString(got) != hex.EncodeToString(want) {
		t.Errorf("BuildMemorySet(goldenRecord()) =\n% X, want\n% X", got, want)
	}
}

// TestGolden_AnswerRoundTrips builds the ANSWER-shaped frame (radio ->
// controller: to=E0, from=74) from the same 39-byte record body the set
// vector carries, and confirms ParseMemoryAnswer decodes it back to the
// same field values goldenRecord() states.
func TestGolden_AnswerRoundTrips(t *testing.T) {
	vecs := loadGoldenVectors(t)
	setFrame := vecs[vecSetRecord]
	// setFrame: FE FE 74 E0 1A 00 <ch-hi> <ch-lo> <39 record bytes> FD
	// answerFrame swaps to/from: FE FE E0 74 1A 00 <ch-hi> <ch-lo> <same
	// 39 bytes> FD.
	if len(setFrame) != 48 {
		t.Fatalf("set-record-minimal vector is %d bytes, want 48", len(setFrame))
	}
	answer := append([]byte{}, setFrame...)
	answer[2], answer[3] = setFrame[3], setFrame[2]

	rec, err := ic7700.Profile().ParseMemoryAnswer(answer)
	if err != nil {
		t.Fatalf("ParseMemoryAnswer: %v", err)
	}
	want := goldenRecord()
	if rec.Mode != want.Mode {
		t.Errorf("Mode = %v, want %v", rec.Mode, want.Mode)
	}
	if rec.Filter != want.Filter {
		t.Errorf("Filter = %v, want %v", rec.Filter, want.Filter)
	}
	if rec.ToneMode != want.ToneMode {
		t.Errorf("ToneMode = %v, want %v", rec.ToneMode, want.ToneMode)
	}
	if rec.Name != want.Name {
		t.Errorf("Name = %v, want %v", rec.Name, want.Name)
	}
	if rec.RXFreqHz != want.RXFreqHz {
		t.Errorf("RXFreqHz = %v, want %v", rec.RXFreqHz, want.RXFreqHz)
	}
	if rec.TXFreqHz != want.TXFreqHz {
		t.Errorf("TXFreqHz = %v, want %v (idx15-19, the TX-duplicate block's own frequency span)", rec.TXFreqHz, want.TXFreqHz)
	}
}

// TestGolden_AllFFRecordFailsToParse pins that an all-FF answer fails to
// decode rather than being silently read as an empty channel: 0xFF is in
// none of this profile's mapped enums (mode, filter, tone_mode), so the
// first enum byte the decoder reaches refuses with a parse error. This
// records the question raised at matrix §3.8(b) rather than answering it —
// see doc.go.
func TestGolden_AllFFRecordFailsToParse(t *testing.T) {
	answer := []byte{0xFE, 0xFE, 0xE0, 0x74, 0x1A, 0x00, 0x00, 0x01}
	for i := 0; i < ic7700.RecordOnlyLength; i++ {
		answer = append(answer, 0xFF)
	}
	answer = append(answer, 0xFD)
	if _, err := ic7700.Profile().ParseMemoryAnswer(answer); err == nil {
		t.Fatal("ParseMemoryAnswer(all-FF record) = nil error, want a parse error")
	}
}
