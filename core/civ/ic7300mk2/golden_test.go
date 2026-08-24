// SPDX-License-Identifier: GPL-3.0-or-later

package ic7300mk2_test

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7300mk2"
)

// A NOTE THE REVIEWER MUST CHECK BY EYE.
//
// This file calls Profile().BuildMemorySet. internal/guards' Rule 4
// (TestCIVWritePathReachableOnlyThroughDriver) fences that builder to
// core/civ, core/civ/civtest and core/driver/**, and its comment says a
// call site in a core/civ/<model> package "is precisely the regression this
// fence exists to refuse". The guard's walker SKIPS _test.go files, so a
// test call site is invisible to it — the same latitude
// core/cat/ftdx10/golden_test.go takes.
//
// NO NON-TEST FILE IN THIS PACKAGE MAY REFERENCE BuildMemorySet, and
// because the guard cannot see the difference, the reviewer looks:
//
//	grep -rn "BuildMemorySet" core/civ/ic7300mk2 --include='*.go' | grep -v '_test.go'
//
// must print nothing.

// goldenVector is one line of testdata/ic7300mk2-vectors.golden.
type goldenVector struct {
	name  string
	frame []byte
}

// loadGoldenVectors parses the frozen vector file STRICTLY: one TAB, no
// CR, no trimming of the frame field.
func loadGoldenVectors(t *testing.T) map[string]goldenVector {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(evidenceDir, "ic7300mk2-vectors.golden"))
	if err != nil {
		t.Fatalf("reading the golden vectors: %v", err)
	}
	if bytes.ContainsRune(raw, '\r') {
		t.Fatal("the golden vector file contains a CR — it is frozen evidence and a line-ending change is a finding, not a fix")
	}
	out := map[string]goldenVector{}
	for i, line := range strings.Split(string(raw), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 2 {
			t.Fatalf("golden vector line %d splits into %d TAB-separated fields, want exactly 2", i+1, len(parts))
		}
		name, frame := parts[0], parts[1]
		if _, dup := out[name]; dup {
			t.Fatalf("golden vector %q appears twice", name)
		}
		var b []byte
		for _, tok := range strings.Split(frame, " ") {
			if len(tok) != 2 {
				t.Fatalf("golden vector %q has the byte token %q, want exactly two hex digits", name, tok)
			}
			v, err := hex.DecodeString(tok)
			if err != nil {
				t.Fatalf("golden vector %q has the byte token %q: %v", name, tok, err)
			}
			b = append(b, v[0])
		}
		out[name] = goldenVector{name: name, frame: b}
	}
	return out
}

// TestGoldenVectorsReplay replays the G leg's four frames through this
// package's builders, its parsers, its accumulator and its gate.
//
// FOUR, not three: this model's G leg found a worked example frame on the
// page and wrote it down. It is a NEGATIVE vector — see below.
func TestGoldenVectorsReplay(t *testing.T) {
	p := ic7300mk2.Profile()
	vectors := loadGoldenVectors(t)

	// ---- 1. Exactly four vectors, by name. ---------------------------
	wantNames := []string{"read-record", "set-record-name-with-space", "read-transceiver-id", "manual-example-1"}
	if len(vectors) != len(wantNames) {
		t.Fatalf("the golden file carries %d vectors, want %d", len(vectors), len(wantNames))
	}
	for _, n := range wantNames {
		if _, ok := vectors[n]; !ok {
			t.Fatalf("the golden file has no vector named %q", n)
		}
	}

	// ---- 2. read-transceiver-id. -------------------------------------
	idCmd, err := p.BuildTransceiverIDRead()
	if err != nil {
		t.Fatalf("BuildTransceiverIDRead: %v", err)
	}
	wantID := []byte{0xFE, 0xFE, 0xB6, 0xE0, 0x19, 0x00, 0xFD}
	if !bytes.Equal(idCmd.Bytes(), wantID) {
		t.Errorf("BuildTransceiverIDRead() = % X, want % X", idCmd.Bytes(), wantID)
	}
	if !bytes.Equal(idCmd.Bytes(), vectors["read-transceiver-id"].frame) {
		t.Errorf("BuildTransceiverIDRead() = % X, but the golden vector is % X", idCmd.Bytes(), vectors["read-transceiver-id"].frame)
	}
	if !p.AllowedCommand(idCmd.Bytes()) {
		t.Errorf("the gate refused the identity read % X", idCmd.Bytes())
	}

	// ---- 3. read-record. ---------------------------------------------
	addr := civ.ChannelAddress{Group: 0, Channel: 1}
	readCmd, err := p.BuildMemoryRead(addr)
	if err != nil {
		t.Fatalf("BuildMemoryRead(%v): %v", addr, err)
	}
	wantRead := []byte{0xFE, 0xFE, 0xB6, 0xE0, 0x1A, 0x00, 0x00, 0x01, 0xFD}
	if !bytes.Equal(readCmd.Bytes(), wantRead) {
		t.Errorf("BuildMemoryRead(%v) = % X, want % X", addr, readCmd.Bytes(), wantRead)
	}
	if !bytes.Equal(readCmd.Bytes(), vectors["read-record"].frame) {
		t.Errorf("BuildMemoryRead(%v) = % X, but the golden vector is % X", addr, readCmd.Bytes(), vectors["read-record"].frame)
	}
	if !p.AllowedCommand(readCmd.Bytes()) {
		t.Errorf("the gate refused the memory read % X", readCmd.Bytes())
	}

	// ---- 4. set-record-name-with-space, field by field. --------------
	setFrame := vectors["set-record-name-with-space"].frame
	if len(setFrame) != 54 {
		t.Fatalf("the set vector is %d bytes, want 54 (7 frame bytes + 2 address bytes + 45 record bytes)", len(setFrame))
	}
	recordBytes := setFrame[8 : len(setFrame)-1]
	if len(recordBytes) != 45 {
		t.Fatalf("the set vector's record is %d bytes, want 45", len(recordBytes))
	}

	want := civ.MemoryRecord{
		Address:      addr,
		RXFreqHz:     civ.Available[uint64](14_100_000),
		TXFreqHz:     civ.Available[uint64](14_100_000),
		ToneTXDeciHz: civ.Available[uint64](885),
		ToneRXDeciHz: civ.Available[uint64](1000),
		Mode:         civ.Available("USB"),
		Filter:       civ.Available("FIL1"),
		DataMode:     civ.Available("OFF"),
		ToneMode:     civ.Available("OFF"),
		Name:         civ.Available("TESTING NAME0123"),
		Select:       civ.Available("OFF"),
	}

	answer := make([]byte, 0, len(setFrame))
	answer = append(answer, 0xFE, 0xFE, 0xE0, 0xB6, 0x1A, 0x00)
	answer = append(answer, setFrame[6], setFrame[7])
	answer = append(answer, recordBytes...)
	answer = append(answer, 0xFD)

	got, err := p.ParseMemoryAnswer(answer)
	if err != nil {
		t.Fatalf("ParseMemoryAnswer over the golden record: %v", err)
	}
	if got.Address != want.Address {
		t.Errorf("Address = %v, want %v", got.Address, want.Address)
	}
	for _, f := range []struct {
		name      string
		got, want civ.Optional[uint64]
	}{
		{"RXFreqHz", got.RXFreqHz, want.RXFreqHz},
		{"TXFreqHz", got.TXFreqHz, want.TXFreqHz},
		{"ToneTXDeciHz", got.ToneTXDeciHz, want.ToneTXDeciHz},
		{"ToneRXDeciHz", got.ToneRXDeciHz, want.ToneRXDeciHz},
		{"OffsetHz", got.OffsetHz, want.OffsetHz},
		{"DTCSCode", got.DTCSCode, want.DTCSCode},
	} {
		if f.got != f.want {
			t.Errorf("%s = %v, want %v", f.name, f.got, f.want)
		}
	}
	for _, f := range []struct {
		name      string
		got, want civ.Optional[string]
	}{
		{"Mode", got.Mode, want.Mode},
		{"Filter", got.Filter, want.Filter},
		{"DataMode", got.DataMode, want.DataMode},
		{"ToneMode", got.ToneMode, want.ToneMode},
		{"Name", got.Name, want.Name},
		{"Select", got.Select, want.Select},
		{"Duplex", got.Duplex, want.Duplex},
		{"DTCSPolarity", got.DTCSPolarity, want.DTCSPolarity},
	} {
		if f.got != f.want {
			t.Errorf("%s = %v, want %v", f.name, f.got, f.want)
		}
	}
	if got != want {
		t.Errorf("ParseMemoryAnswer = %+v, want %+v", got, want)
	}

	// The two tone fields differ from each other on this vector (885 and
	// 1000), which is deliberate: a vector whose repeater tone and tone
	// squelch agreed could not tell the two three-byte spans apart.
	if want.ToneTXDeciHz == want.ToneRXDeciHz {
		t.Error("this vector's two tone values are equal — it would then not distinguish ⑫ ~ ⑭ from ⑮ ~ ⑰")
	}

	// The SPLIT half of ③, asserted SEPARATELY, so E6's write-time
	// comparison has a golden witness.
	layout, ok := p.LayoutFor(45)
	if !ok {
		t.Fatal("LayoutFor(45) missing")
	}
	if recordBytes[0]&0xF0 != layout.Fixed[0]&0xF0 {
		t.Errorf("the golden record's byte 0 high nibble is %#x but the layout's template says %#x — this vector is a Split-OFF channel and E6's unmapped-region comparison must hold on it", recordBytes[0]>>4, layout.Fixed[0]>>4)
	}

	name, _ := got.Name.Get()
	if len(name) != 16 {
		t.Errorf("Name is %q, %d bytes — the field is sixteen bytes wide and this vector fills it", name, len(name))
	}
	if name[7] != 0x20 {
		t.Errorf("Name[7] = %#02x, want 0x20 — the space at index 7 is the point of this vector, and the space byte is D5 entry 3's space half, ASSUMED (lift MK2-R9)", name[7])
	}

	gotAddr, gotRaw, err := p.MemoryAnswerRecord(answer)
	if err != nil {
		t.Fatalf("MemoryAnswerRecord: %v", err)
	}
	if gotAddr != addr {
		t.Errorf("MemoryAnswerRecord address = %v, want %v", gotAddr, addr)
	}
	if !bytes.Equal(gotRaw, recordBytes) {
		t.Errorf("MemoryAnswerRecord returned % X, want the golden record % X", gotRaw, recordBytes)
	}

	setCmd, err := p.BuildMemorySet(want)
	if err != nil {
		t.Fatalf("BuildMemorySet: %v", err)
	}
	if !bytes.Equal(setCmd.Bytes(), setFrame) {
		t.Errorf("BuildMemorySet = % X,\nthe golden frame is % X", setCmd.Bytes(), setFrame)
	}
	if !p.AllowedCommand(setFrame) {
		t.Errorf("the gate refused the golden set frame % X", setFrame)
	}

	// ---- 5. manual-example-1, THE NEGATIVE VECTOR. -------------------
	//
	// It is the power-ON command with its documented FE preamble padding
	// (PDF p.16, §3.16 A8): fifteen FE bytes, then B6 E0 18 01 FD. It is
	// the only vector in either model's set that the DOCUMENT ITSELF
	// prints as a complete frame, and NOTHING IN THIS TIER MAY BUILD OR
	// SEND IT. 18 01 on a shared bus is exactly the frame that must never
	// be constructible.
	manualExample1 := vectors["manual-example-1"].frame
	if len(manualExample1) != 20 {
		t.Fatalf("manual-example-1 is %d bytes, want 20 (fifteen FE, then B6 E0 18 01 FD)", len(manualExample1))
	}
	if p.AllowedCommand(manualExample1) {
		t.Errorf("the gate ADMITTED manual-example-1 (% X) — this vector exists to prove the gate refuses a frame the manual itself prints, and 18 01 is not one of the three grammars this tier ships", manualExample1)
	}

	// The 15 leading FEs are PADDING, not noise, and the frame is
	// addressed to the radio, so a controller-side accumulator counts it
	// Unexpected and returns nothing. Both halves are the point: D1's
	// padding tolerance holds, and the address filter is what keeps a
	// controller from ever matching a frame meant for the radio.
	acc := p.NewAccumulator()
	frames, err := acc.Push(manualExample1)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if len(frames) != 0 {
		t.Errorf("Push returned %d frames, want 0 — this vector is addressed to B6 (the radio), and a controller-side accumulator never returns it", len(frames))
	}
	st := acc.Stats()
	if st.Unexpected != 1 {
		t.Errorf("Stats().Unexpected = %d, want 1 — the frame completed and was filtered by address, not discarded as rubbish", st.Unexpected)
	}
	if st.NoiseBytes != 0 {
		t.Errorf("Stats().NoiseBytes = %d, want 0 — the 15 leading FEs are documented preamble padding (PDF p.16, §3.16 A8), which D1 requires the accumulator to tolerate rather than count as noise", st.NoiseBytes)
	}
	if st.Truncated != 0 {
		t.Errorf("Stats().Truncated = %d, want 0", st.Truncated)
	}

	// ---- 6. What the gate must REFUSE. -------------------------------
	refuse := []struct {
		what  string
		frame []byte
	}{
		{
			"the documented clear form (1A 00 with ③ = FF)",
			[]byte{0xFE, 0xFE, 0xB6, 0xE0, 0x1A, 0x00, 0x00, 0x01, 0xFF, 0xFD},
		},
		{
			"the 0B memory-clear command",
			[]byte{0xFE, 0xFE, 0xB6, 0xE0, 0x0B, 0xFD},
		},
		{
			"the 18 00 power-off command",
			[]byte{0xFE, 0xFE, 0xB6, 0xE0, 0x18, 0x00, 0xFD},
		},
		{
			"a 1A 05 menu write (here the USB (B) Function item)",
			[]byte{0xFE, 0xFE, 0xB6, 0xE0, 0x1A, 0x05, 0x00, 0x94, 0x01, 0xFD},
		},
		{
			"the identity ANSWER replayed as a command",
			[]byte{0xFE, 0xFE, 0xB6, 0xE0, 0x19, 0x00, 0xB6, 0xFD},
		},
		{
			"a clear aimed at P1, which this radio cannot clear at all",
			[]byte{0xFE, 0xFE, 0xB6, 0xE0, 0x1A, 0x00, 0x01, 0x00, 0xFF, 0xFD},
		},
	}
	// The same set frame, re-addressed to the IC-7300.
	foreign := append([]byte(nil), setFrame...)
	foreign[2] = 0x94
	refuse = append(refuse, struct {
		what  string
		frame []byte
	}{"the golden set frame re-addressed to 94 (the IC-7300)", foreign})

	// Erratum 2's one deliberate width does not arise here either: this
	// profile is single-length, so the gate and the builder coincide.
	if got := len(p.RecordLengths()); got != 1 {
		t.Errorf("RecordLengths() has %d entries, want 1 — this profile is single-length, which is why Erratum 2's deliberate width does not apply", got)
	}
	shortSet := make([]byte, 0, 48)
	shortSet = append(shortSet, 0xFE, 0xFE, 0xB6, 0xE0, 0x1A, 0x00, 0x00, 0x01)
	shortSet = append(shortSet, make([]byte, 39)...)
	shortSet = append(shortSet, 0xFD)
	refuse = append(refuse, struct {
		what  string
		frame []byte
	}{"a set carrying 39 record bytes (the IC-7300's length)", shortSet})

	for _, bad := range []byte{0x06, 0x09} {
		mutated := append([]byte(nil), setFrame...)
		mutated[8+6] = bad
		refuse = append(refuse, struct {
			what  string
			frame []byte
		}{"the set frame with mode byte " + hex.EncodeToString([]byte{bad}), mutated})
		both := append([]byte(nil), mutated...)
		both[8+20] = bad
		refuse = append(refuse, struct {
			what  string
			frame []byte
		}{"the set frame with BOTH mode bytes " + hex.EncodeToString([]byte{bad}), both})
	}

	for _, r := range refuse {
		if p.AllowedCommand(r.frame) {
			t.Errorf("the gate ADMITTED %s: % X", r.what, r.frame)
		}
	}
}
