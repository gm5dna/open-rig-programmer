// SPDX-License-Identifier: GPL-3.0-or-later

package ic7300_test

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7300"
)

// A NOTE THE REVIEWER MUST CHECK BY EYE.
//
// This file calls Profile().BuildMemorySet. internal/guards' Rule 4
// (TestCIVWritePathReachableOnlyThroughDriver) fences that builder to
// core/civ, core/civ/civtest and core/driver/**, and its comment says a
// call site in a core/civ/<model> package "is precisely the regression this
// fence exists to refuse". The guard's walker SKIPS _test.go files
// (internal/guards/importgraph_test.go's parseRepo), so a test call site is
// invisible to it — the same latitude core/cat/ftdx10/golden_test.go takes
// when it calls d.BuildMWSet.
//
// NO NON-TEST FILE IN THIS PACKAGE MAY REFERENCE BuildMemorySet, and
// because the guard cannot see the difference, the reviewer looks:
//
//	grep -rn "BuildMemorySet" core/civ/ic7300 --include='*.go' | grep -v '_test.go'
//
// must print nothing.

// goldenVector is one line of testdata/ic7300-vectors.golden.
type goldenVector struct {
	name  string
	frame []byte
}

// loadGoldenVectors parses the frozen vector file STRICTLY.
//
// One TAB, never two; no CR anywhere; no trimming of the frame field. The
// strictness is the point: a vector file whose whitespace had drifted
// would otherwise be silently re-read as a different frame, and these
// frames are the only bytes in this package that anybody wrote down as
// bytes rather than derived from a diagram.
func loadGoldenVectors(t *testing.T) map[string]goldenVector {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(evidenceDir, "ic7300-vectors.golden"))
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

// TestGoldenVectorsReplay replays the G leg's three frames through this
// package's builders, its parsers and its gate.
//
// The IC-7300's G leg wrote no manual-example-<n> vector: its provenance
// says the document prints no worked example frame, which the matrix's
// Erratum 3 establishes is WRONG ON THE PAGE — PDF p.166's footnote *3
// prints one, for 18 01. The quarantined artefact is deliberately NOT
// edited; the disposition is record-and-continue, ratified by the
// orchestrator on 24/08/2026. The consequence for this test is that this
// model has three vectors where its sibling has four, and the missing
// fourth is the negative one, so the gate's refusals below are asserted
// against frames this test constructs rather than against a vector.
func TestGoldenVectorsReplay(t *testing.T) {
	p := ic7300.Profile()
	vectors := loadGoldenVectors(t)

	// ---- 1. Exactly three vectors, by name. --------------------------
	wantNames := []string{"read-record", "set-record-name-with-space", "read-transceiver-id"}
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
	wantID := []byte{0xFE, 0xFE, 0x94, 0xE0, 0x19, 0x00, 0xFD}
	if !bytes.Equal(idCmd.Bytes(), wantID) {
		t.Errorf("BuildTransceiverIDRead() = % X, want % X", idCmd.Bytes(), wantID)
	}
	if !bytes.Equal(idCmd.Bytes(), vectors["read-transceiver-id"].frame) {
		t.Errorf("BuildTransceiverIDRead() = % X, but the golden vector is % X", idCmd.Bytes(), vectors["read-transceiver-id"].frame)
	}
	if !p.AllowedCommand(idCmd.Bytes()) {
		t.Errorf("the gate refused the identity read % X — a builder and a gate that disagree mean this profile cannot send a command it believes is valid", idCmd.Bytes())
	}

	// ---- 3. read-record. ---------------------------------------------
	addr := civ.ChannelAddress{Group: 0, Channel: 1}
	readCmd, err := p.BuildMemoryRead(addr)
	if err != nil {
		t.Fatalf("BuildMemoryRead(%v): %v", addr, err)
	}
	wantRead := []byte{0xFE, 0xFE, 0x94, 0xE0, 0x1A, 0x00, 0x00, 0x01, 0xFD}
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
	// FE FE to from cn sc <2 address bytes> <39 record bytes> FD.
	if len(setFrame) != 48 {
		t.Fatalf("the set vector is %d bytes, want 48 (7 frame bytes + 2 address bytes + 39 record bytes)", len(setFrame))
	}
	recordBytes := setFrame[8 : len(setFrame)-1]
	if len(recordBytes) != 39 {
		t.Fatalf("the set vector's record is %d bytes, want 39", len(recordBytes))
	}

	want := civ.MemoryRecord{
		Address:      addr,
		RXFreqHz:     civ.Available[uint64](14_250_000),
		TXFreqHz:     civ.Available[uint64](14_250_000),
		ToneTXDeciHz: civ.Available[uint64](885),
		ToneRXDeciHz: civ.Available[uint64](885),
		Mode:         civ.Available("USB"),
		Filter:       civ.Available("FIL1"),
		DataMode:     civ.Available("OFF"),
		ToneMode:     civ.Available("OFF"),
		Name:         civ.Available("TEST CHAN1"),
		Select:       civ.Available("OFF"),
	}

	// The record parsed out of the golden ANSWER shape, asserted ONE FIELD
	// AT A TIME so a failure names the field rather than printing two
	// structs and leaving the reader to diff them.
	answer := make([]byte, 0, len(setFrame))
	answer = append(answer, 0xFE, 0xFE, 0xE0, 0x94, 0x1A, 0x00)
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
	// civ.MemoryRecord is comparable, and the round-trip property this
	// package rests on is stated as equality, so it is stated here too
	// after the field-by-field pass has already named anything wrong.
	if got != want {
		t.Errorf("ParseMemoryAnswer = %+v, want %+v", got, want)
	}

	// The SPLIT half of ③, asserted SEPARATELY from every mapped field,
	// so the E6 contract has a golden witness: the vector's unmapped
	// nibble equals the layout's template, which is exactly the comparison
	// each driver makes before it writes.
	layout, ok := p.LayoutFor(39)
	if !ok {
		t.Fatal("LayoutFor(39) missing")
	}
	if recordBytes[0]&0xF0 != layout.Fixed[0]&0xF0 {
		t.Errorf("the golden record's byte 0 high nibble is %#x but the layout's template says %#x — this vector is a Split-OFF channel and E6's unmapped-region comparison must hold on it", recordBytes[0]>>4, layout.Fixed[0]>>4)
	}

	// The name is what this vector is FOR.
	name, _ := got.Name.Get()
	if len(name) != 10 {
		t.Errorf("Name is %q, %d bytes — the field is ten bytes wide and this vector fills it", name, len(name))
	}
	if name[4] != 0x20 {
		t.Errorf("Name[4] = %#02x, want 0x20 — the space at index 4 is the whole point of this vector, and the space byte is D5 entry 3's space half, ASSUMED (lift ic7300-name-space)", name[4])
	}

	// MemoryAnswerRecord hands back the raw bytes, unread.
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

	// And the build direction: the record above, re-encoded, must be the
	// golden frame BYTE FOR BYTE. A mismatch here is a STOP — the vectors
	// are never regenerated.
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

	// ---- 6. What the gate must REFUSE. -------------------------------
	refuse := []struct {
		what  string
		frame []byte
	}{
		{
			"the documented clear form (1A 00 with ③ = FF)",
			[]byte{0xFE, 0xFE, 0x94, 0xE0, 0x1A, 0x00, 0x00, 0x01, 0xFF, 0xFD},
		},
		{
			"the 0B memory-clear command",
			[]byte{0xFE, 0xFE, 0x94, 0xE0, 0x0B, 0xFD},
		},
		{
			"the 18 00 power-off command",
			[]byte{0xFE, 0xFE, 0x94, 0xE0, 0x18, 0x00, 0xFD},
		},
		{
			"a 1A 05 menu write",
			[]byte{0xFE, 0xFE, 0x94, 0xE0, 0x1A, 0x05, 0x00, 0x91, 0xFD},
		},
		{
			"the identity ANSWER replayed as a command",
			[]byte{0xFE, 0xFE, 0x94, 0xE0, 0x19, 0x00, 0x94, 0xFD},
		},
	}
	// The same set frame, re-addressed to the IC-7300MK2. A profile gates
	// for the radio it describes and for no other.
	foreign := append([]byte(nil), setFrame...)
	foreign[2] = 0xB6
	refuse = append(refuse, struct {
		what  string
		frame []byte
	}{"the golden set frame re-addressed to B6 (the IC-7300MK2)", foreign})

	// A set at the SIBLING's record length. Erratum 2's one deliberate
	// width — a gate admitting every DECLARED length while the builder
	// emits one — does not arise on this pair: both profiles are
	// single-length, so AllowedCommand and BuildMemorySet coincide here.
	if got := len(p.RecordLengths()); got != 1 {
		t.Errorf("RecordLengths() has %d entries, want 1 — this profile is single-length, which is why Erratum 2's deliberate width does not arise", got)
	}
	longSet := make([]byte, 0, 54)
	longSet = append(longSet, 0xFE, 0xFE, 0x94, 0xE0, 0x1A, 0x00, 0x00, 0x01)
	longSet = append(longSet, make([]byte, 45)...)
	longSet = append(longSet, 0xFD)
	refuse = append(refuse, struct {
		what  string
		frame []byte
	}{"a set carrying 45 record bytes (the IC-7300MK2's length)", longSet})

	// A record byte carrying a value the layout does not define. The mode
	// byte is walked through 06 — printed NOWHERE on PDF p.167, the two
	// spare cells struck through — and 09, past the printed column
	// altogether.
	for _, bad := range []byte{0x06, 0x09} {
		mutated := append([]byte(nil), setFrame...)
		mutated[8+6] = bad
		refuse = append(refuse, struct {
			what  string
			frame []byte
		}{"the set frame with mode byte " + hex.EncodeToString([]byte{bad}), mutated})
		// And with BOTH copies changed, so the refusal cannot be credited
		// to the duplicated-span disagreement check alone: an undefined
		// enum value must be refused on its own account.
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
