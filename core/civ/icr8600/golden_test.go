// SPDX-License-Identifier: GPL-3.0-or-later

package icr8600_test

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/icr8600"
)

type r8600GoldenVector struct {
	name  string
	frame []byte
}

var r8600GoldenOrder = []string{
	"read-record",
	"set-record-name-with-space-48",
	"set-record-name-with-space-50",
	"set-record-name-with-space-52",
	"set-record-name-with-space-54",
	"set-record-name-with-space-55-fm",
	"set-record-name-with-space-55-dcr",
	"set-record-name-with-space-56",
	"read-transceiver-id",
	"manual-example-1",
}

func TestGoldenVectorsReplay(t *testing.T) {
	vectors := loadR8600GoldenVectors(t)
	p := icr8600.Profile()

	id, err := p.BuildTransceiverIDRead()
	if err != nil {
		t.Fatalf("BuildTransceiverIDRead: %v", err)
	}
	requireR8600GoldenFrame(t, "read-transceiver-id", id.Bytes(), vectors["read-transceiver-id"].frame)
	if !p.AllowedCommand(id.Bytes()) {
		t.Errorf("gate refused golden identity read % X", id.Bytes())
	}

	addr := civ.ChannelAddress{Group: 0, Channel: 1}
	read, err := p.BuildMemoryRead(addr)
	if err != nil {
		t.Fatalf("BuildMemoryRead(%v): %v", addr, err)
	}
	requireR8600GoldenFrame(t, "read-record", read.Bytes(), vectors["read-record"].frame)
	if !p.AllowedCommand(read.Bytes()) {
		t.Errorf("gate refused golden memory read % X", read.Bytes())
	}

	setVectors := []struct {
		name string
		mode string
	}{
		{"set-record-name-with-space-48", "AM"},
		{"set-record-name-with-space-50", "D-STAR"},
		{"set-record-name-with-space-52", "P25"},
		{"set-record-name-with-space-54", "NXDN-VN"},
		{"set-record-name-with-space-55-fm", "FM"},
		{"set-record-name-with-space-55-dcr", "DCR"},
		{"set-record-name-with-space-56", "dPMR"},
	}
	for _, tc := range setVectors {
		t.Run(tc.name, func(t *testing.T) {
			want := vectors[tc.name].frame
			rec := r8600GoldenRecord(tc.mode)
			cmd, err := p.BuildMemorySet(rec)
			if err != nil {
				t.Fatalf("BuildMemorySet(%s): %v", tc.mode, err)
			}
			requireR8600GoldenFrame(t, tc.name, cmd.Bytes(), want)
			if !p.AllowedCommand(want) {
				t.Errorf("gate refused golden set % X", want)
			}

			recordLength := p.BuildRecordLengthFor(tc.mode)
			dataAreaLength := 4 + recordLength
			fullFrameLength := 7 + dataAreaLength
			if len(want) != fullFrameLength {
				t.Errorf("full frame length = %d, want 7 overhead + %d data-area = %d", len(want), dataAreaLength, fullFrameLength)
			}
			if got := len(want) - 7; got != dataAreaLength {
				t.Errorf("data-area length = %d, want %d", got, dataAreaLength)
			}
			if tc.mode == "FM" || tc.mode == "DCR" {
				if recordLength != 44 || dataAreaLength != 48 || fullFrameLength != 55 {
					t.Errorf("G %s accounting = record/data/frame %d/%d/%d, want 44/48/55", tc.mode, recordLength, dataAreaLength, fullFrameLength)
				}
			}

			answer := append([]byte(nil), want...)
			answer[2], answer[3] = answer[3], answer[2]
			gotAddr, raw, err := p.MemoryAnswerRecord(answer)
			if err != nil {
				t.Fatalf("MemoryAnswerRecord: %v", err)
			}
			if gotAddr != addr {
				t.Errorf("raw address = %v, want %v", gotAddr, addr)
			}
			wantRaw := want[10 : len(want)-1]
			if len(raw) != recordLength || !bytes.Equal(raw, wantRaw) {
				t.Errorf("raw record (%d) = % X, want golden (%d) % X", len(raw), raw, len(wantRaw), wantRaw)
			}

			decoded, err := p.ParseMemoryAnswer(answer)
			if err != nil {
				t.Fatalf("ParseMemoryAnswer: %v", err)
			}
			if decoded != rec {
				t.Errorf("decoded record = %+v, want literal golden intent %+v", decoded, rec)
			}
			rebuilt, err := p.BuildMemorySet(decoded)
			if err != nil {
				t.Fatalf("re-encode decoded record: %v", err)
			}
			requireR8600GoldenFrame(t, tc.name+" decode/re-encode", rebuilt.Bytes(), want)
		})
	}

	// The manual's only worked example is preamble-padding evidence for
	// command 18 01. There is no builder for it and it is outside the three
	// admitted grammars, so replaying it means proving the gate refuses it.
	manual := vectors["manual-example-1"].frame
	if p.AllowedCommand(manual) {
		t.Errorf("gate admitted documentary manual example % X", manual)
	}
}

func TestGoldenAllFFRecordsRemainRawBeforeLayoutSelection(t *testing.T) {
	p := icr8600.Profile()
	addr := civ.ChannelAddress{Group: 0, Channel: 1}
	for _, length := range p.RecordLengths() {
		t.Run(strconv.Itoa(length), func(t *testing.T) {
			frame := []byte{0xFE, 0xFE, 0xE0, 0x96, 0x1A, 0x00, 0x00, 0x00, 0x00, 0x01}
			frame = append(frame, bytes.Repeat([]byte{0xFF}, length)...)
			frame = append(frame, 0xFD)

			gotAddr, raw, err := p.MemoryAnswerRecord(frame)
			if err != nil {
				t.Fatalf("MemoryAnswerRecord(%d all-FF bytes): %v", length, err)
			}
			if gotAddr != addr || len(raw) != length || !bytes.Equal(raw, bytes.Repeat([]byte{0xFF}, length)) {
				t.Errorf("raw split = address %v record % X, want %v and %d untouched FF bytes", gotAddr, raw, addr, length)
			}
			// The raw split above is the pre-fingerprint hook: no mode layout
			// has yet been selected. Interpreting the bytes must fail closed,
			// because FF is an undeclared mode discriminator, rather than turn
			// the assumed icr8600-empty-reply-ff representation into a
			// channel fingerprint.
			if _, err := p.LayoutForRecord(raw); err == nil || !strings.Contains(err.Error(), "undeclared mode") {
				t.Errorf("LayoutForRecord(%d all-FF bytes) error = %v, want undeclared-mode refusal", length, err)
			}
			if rec, err := p.ParseMemoryAnswer(frame); err == nil {
				t.Errorf("ParseMemoryAnswer accepted all-FF record as %+v", rec)
			}
		})
	}
}

func r8600GoldenRecord(mode string) civ.MemoryRecord {
	rec := commonRecord(mode)
	if mode == "FM" {
		rec.ToneMode = civ.Available("TSQL")
		rec.ToneRXDeciHz = civ.Available(uint64(885))
		rec.DTCSPolarity = civ.Available("Normal")
		rec.DTCSCode = civ.Available(uint64(23))
	}
	return rec
}

func loadR8600GoldenVectors(t *testing.T) map[string]r8600GoldenVector {
	t.Helper()
	path := filepath.Join("testdata", "IC-R8600-vectors.golden")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if bytes.ContainsRune(raw, '\r') {
		t.Fatalf("%s contains a CR; frozen vector whitespace is evidence", path)
	}
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(lines) != len(r8600GoldenOrder) {
		t.Fatalf("%s has %d vectors, want %d", path, len(lines), len(r8600GoldenOrder))
	}
	gotOrder := make([]string, 0, len(lines))
	vectors := make(map[string]r8600GoldenVector, len(lines))
	for lineNumber, line := range lines {
		parts := strings.Split(line, "\t")
		if len(parts) != 2 {
			t.Fatalf("%s line %d has %d TAB-separated fields, want two", path, lineNumber+1, len(parts))
		}
		name := parts[0]
		if _, exists := vectors[name]; exists {
			t.Fatalf("%s names %q twice", path, name)
		}
		var frame []byte
		for _, token := range strings.Split(parts[1], " ") {
			if len(token) != 2 {
				t.Fatalf("%s vector %q token %q is not exactly two hex digits", path, name, token)
			}
			value, err := hex.DecodeString(token)
			if err != nil {
				t.Fatalf("%s vector %q token %q: %v", path, name, token, err)
			}
			frame = append(frame, value[0])
		}
		gotOrder = append(gotOrder, name)
		vectors[name] = r8600GoldenVector{name: name, frame: frame}
	}
	if !reflect.DeepEqual(gotOrder, r8600GoldenOrder) {
		t.Fatalf("golden vector order = %v, want %v", gotOrder, r8600GoldenOrder)
	}
	return vectors
}

func requireR8600GoldenFrame(t *testing.T, name string, got, want []byte) {
	t.Helper()
	if bytes.Equal(got, want) {
		return
	}
	first := min(len(got), len(want))
	for i := 0; i < min(len(got), len(want)); i++ {
		if got[i] != want[i] {
			first = i
			break
		}
	}
	t.Fatalf("%s disagrees with frozen G — STOP and arbitrate against the PDF:\n  built  (%d) % X\n  golden (%d) % X\n  first difference: byte %d (one-based %d)",
		name, len(got), got, len(want), want, first, first+1)
}
