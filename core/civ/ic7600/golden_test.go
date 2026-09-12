// SPDX-License-Identifier: GPL-3.0-or-later

package ic7600_test

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7600"
)

// This file is the mechanical byte-compare of this dialect's codec against
// the four hand-derived wire frames in testdata/ic7600-vectors.golden - see
// testdata/ic7600-golden-provenance.md for how they were built (the matrix,
// not a blind quarantine leg) - and the positive proof of what the outbound
// gate REFUSES.
//
// THIS FILE MAY NOT MODIFY ANY VECTOR, and no failure here is ever fixed by
// editing one. A golden-vs-codec mismatch is a STOP for orchestrator
// arbitration AGAINST THE PDF.
//
// # Hardware status
//
// UNVERIFIED, for all four vectors. Not one has been sent to, or captured
// from, a real IC-7600. Green here means the codec agrees with the matrix's
// own reading of the manual, not that any radio accepts these bytes.

const goldenFile = "ic7600-vectors.golden"

const (
	vecReadRecord    = "read-record"                // 9 bytes  - BuildMemoryRead(1)
	vecSetRecord     = "set-record-name-with-space" // 34 bytes - BuildMemorySet(goldenRecord())
	vecReadID        = "read-transceiver-id"        // 7 bytes  - BuildTransceiverIDRead()
	vecManualExample = "manual-example-14"          // 14 bytes - DOCUMENTARY ONLY, and REFUSED
)

// loadGoldenVectors parses the file STRICTLY: exactly four lines, exactly
// the four names below, no blank lines, every token exactly two hex digits.
func loadGoldenVectors(t *testing.T) map[string][]byte {
	t.Helper()
	path := filepath.Join(testdataDir, goldenFile)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	text := strings.TrimSuffix(string(raw), "\n")
	lines := strings.Split(text, "\n")
	if len(lines) != 4 {
		t.Fatalf("%s carries %d lines, want exactly 4", path, len(lines))
	}
	out := make(map[string][]byte, 4)
	for i, line := range lines {
		name, hexes, ok := strings.Cut(line, "\t")
		if !ok {
			t.Fatalf("%s line %d is not <name><TAB><hex bytes>: %q", path, i+1, line)
		}
		if name == "" {
			t.Fatalf("%s line %d has an empty name", path, i+1)
		}
		if _, dup := out[name]; dup {
			t.Fatalf("%s names %q twice", path, name)
		}
		var frame []byte
		for _, tok := range strings.Split(hexes, " ") {
			if len(tok) != 2 {
				t.Fatalf("%s line %d: token %q is not exactly two hex digits", path, i+1, tok)
			}
			b, err := hex.DecodeString(tok)
			if err != nil {
				t.Fatalf("%s line %d: token %q: %v", path, i+1, tok, err)
			}
			frame = append(frame, b[0])
		}
		out[name] = frame
	}
	for name, wantLen := range map[string]int{
		vecReadRecord: 9, vecSetRecord: 34, vecReadID: 7, vecManualExample: 14,
	} {
		got, ok := out[name]
		if !ok {
			t.Fatalf("%s carries no vector named %q", path, name)
		}
		if len(got) != wantLen {
			t.Fatalf("%s: vector %q is %d bytes, want %d", path, name, len(got), wantLen)
		}
	}
	return out
}

// requireFrame compares a built command with a golden vector and prints
// both sides, both lengths and the first differing wire position on
// failure.
func requireFrame(t *testing.T, what string, got, want []byte) {
	t.Helper()
	if string(got) == string(want) {
		return
	}
	t.Errorf("%s DISAGREES WITH THE GOLDEN VECTOR - a STOP for arbitration against the PDF.\n"+
		"  built  (%d bytes): % X\n"+
		"  golden (%d bytes): % X\n"+
		"  first differing wire position (1-indexed): %s",
		what, len(got), got, len(want), want, firstDiff(got, want))
}

// TestGolden_ReadRecord binds the read builder to the nine-byte vector.
//
// BYTES 7-8 ARE THE inherited_assumed RUN: the document prints no 1A 00
// read request at all, so what green here means is that the builder
// agrees with the ASSUMPTION - D5 entry 1 (R1) - and not that any radio
// does.
func TestGolden_ReadRecord(t *testing.T) {
	want := loadGoldenVectors(t)[vecReadRecord]
	cmd, err := ic7600.Profile().BuildMemoryRead(civ.ChannelAddress{Channel: 1})
	if err != nil {
		t.Fatalf("BuildMemoryRead(1): %v", err)
	}
	requireFrame(t, "BuildMemoryRead(channel 1)", cmd.Bytes(), want)
}

// TestGolden_SetRecord binds the set builder to the 34-byte vector.
func TestGolden_SetRecord(t *testing.T) {
	want := loadGoldenVectors(t)[vecSetRecord]
	cmd, err := ic7600.Profile().BuildMemorySet(goldenRecord())
	if err != nil {
		t.Fatalf("BuildMemorySet(goldenRecord()): %v", err)
	}
	requireFrame(t, "BuildMemorySet(goldenRecord())", cmd.Bytes(), want)
}

// TestGolden_SetRecordParsesBack turns the same vector into an
// ANSWER-direction frame and requires the codec's READING of those bytes
// to equal the derivation's stated intent.
func TestGolden_SetRecordParsesBack(t *testing.T) {
	vec := loadGoldenVectors(t)[vecSetRecord]
	answer := make([]byte, len(vec))
	copy(answer, vec)
	answer[2], answer[3] = answer[3], answer[2] // swap to/from: FE FE E0 7A 1A 00 ...

	rec, err := ic7600.Profile().ParseMemoryAnswer(answer)
	if err != nil {
		t.Fatalf("ParseMemoryAnswer of the golden set vector: %v", err)
	}
	want := goldenRecord()
	if rec.Address != want.Address {
		t.Errorf("Address = %v, want %v", rec.Address, want.Address)
	}
	if rec.RXFreqHz != want.RXFreqHz {
		t.Errorf("RXFreqHz = %v, want %v", rec.RXFreqHz, want.RXFreqHz)
	}
	if rec.Mode != want.Mode {
		t.Errorf("Mode = %v, want %v", rec.Mode, want.Mode)
	}
	if rec.Filter != want.Filter {
		t.Errorf("Filter = %v, want %v", rec.Filter, want.Filter)
	}
	if rec.ToneMode != want.ToneMode {
		t.Errorf("ToneMode = %v, want %v", rec.ToneMode, want.ToneMode)
	}
	if rec.ToneTXDeciHz != want.ToneTXDeciHz {
		t.Errorf("ToneTXDeciHz = %v, want %v", rec.ToneTXDeciHz, want.ToneTXDeciHz)
	}
	if rec.ToneRXDeciHz != want.ToneRXDeciHz {
		t.Errorf("ToneRXDeciHz = %v, want %v", rec.ToneRXDeciHz, want.ToneRXDeciHz)
	}
	if rec.Name != want.Name {
		t.Errorf("Name = %v, want %v", rec.Name, want.Name)
	}
	if !rec.Select.Unavailable() {
		t.Errorf("Select = %v, want Unavailable - ruling E6 leaves printed (3) unmapped, and an unmapped region is not decoded", rec.Select)
	}
	if !rec.DataMode.Unavailable() {
		t.Errorf("DataMode = %v, want Unavailable - ruling E6 leaves printed (11)'s high nibble unmapped", rec.DataMode)
	}
}

// TestGolden_TransceiverID binds the probe's request to the seven-byte
// vector, and then asserts NOTHING ABOUT WHICH TOKEN comes back - D5
// entry 7 (R7): the 19 00 reply value is undocumented on all six wave
// models.
func TestGolden_TransceiverID(t *testing.T) {
	p := ic7600.Profile()
	want := loadGoldenVectors(t)[vecReadID]
	cmd, err := p.BuildTransceiverIDRead()
	if err != nil {
		t.Fatalf("BuildTransceiverIDRead: %v", err)
	}
	requireFrame(t, "BuildTransceiverIDRead()", cmd.Bytes(), want)

	for _, token := range []byte{0x7A, 0x00, 0x98} {
		frame := []byte{0xFE, 0xFE, 0xE0, 0x7A, 0x19, 0x00, token, 0xFD}
		got, err := p.ParseTransceiverID(frame)
		if err != nil {
			t.Errorf("ParseTransceiverID(% X): %v", frame, err)
			continue
		}
		if want := hex.EncodeToString([]byte{token}); got != want {
			t.Errorf("ParseTransceiverID(% X) = %q, want %q - the token is recorded verbatim, never interpreted", frame, got, want)
		}
	}
}

// TestGolden_ManualExampleIsRefused is the positive proof that the fourth
// vector is DOCUMENTARY ONLY - see testdata/ic7600-golden-provenance.md:
// this vector is inherited from the IC-7610 exemplar's own vector of the
// same name (the general Icom CI-V multi-preamble wake convention), not
// independently re-confirmed against this radio's own document.
func TestGolden_ManualExampleIsRefused(t *testing.T) {
	p := ic7600.Profile()
	padded := loadGoldenVectors(t)[vecManualExample]
	if p.AllowedCommand(padded) {
		t.Errorf("AllowedCommand admitted the worked example % X; this tier never sends 18 01", padded)
	}
	unpadded := []byte{0xFE, 0xFE, 0x7A, 0xE0, 0x18, 0x01, 0xFD}
	if p.AllowedCommand(unpadded) {
		t.Errorf("AllowedCommand admitted the unpadded power-ON frame % X", unpadded)
	}
}

// TestGate_RefusesEverythingButTheThreeGrammars is the gate's negative
// half. The gate admits 19 00, a valid 1A 00 read and a re-validated 1A 00
// set, and each row below names the reason it is not one of those three.
func TestGate_RefusesEverythingButTheThreeGrammars(t *testing.T) {
	p := ic7600.Profile()
	set := loadGoldenVectors(t)[vecSetRecord]
	const prefix = 6 + ic7600.AddressBytes

	mutate := func(offset int, value byte) []byte {
		out := make([]byte, len(set))
		copy(out, set)
		out[prefix+offset] = value
		return out
	}
	resize := func(n int) []byte {
		out := make([]byte, 0, prefix+n+1)
		out = append(out, set[:prefix]...)
		body := make([]byte, n)
		copy(body, set[prefix:len(set)-1])
		out = append(out, body...)
		return append(out, 0xFD)
	}

	for _, tc := range []struct {
		name  string
		frame []byte
	}{
		{"the 1A 00 clear form (PDF p.178's three-line list)",
			[]byte{0xFE, 0xFE, 0x7A, 0xE0, 0x1A, 0x00, 0x00, 0x01, 0xFF, 0xFD}},
		{"command 0B, Memory clear (PDF p.169)",
			[]byte{0xFE, 0xFE, 0x7A, 0xE0, 0x0B, 0xFD}},
		{"a transceive SET - NO RADIO MUTATION AT INIT, EVER",
			[]byte{0xFE, 0xFE, 0x7A, 0xE0, 0x1A, 0x05, 0x01, 0x12, 0x01, 0xFD}},
		{"a 1A 05 read of the USB echo item - any 1A 05 at all",
			[]byte{0xFE, 0xFE, 0x7A, 0xE0, 0x1A, 0x05, 0x01, 0x16, 0xFD}},
		{"a set at 24 record bytes, a length this profile does not declare", resize(24)},
		{"a set at 26 record bytes, a length this profile does not declare", resize(26)},
		{"a set whose (9) is 0x06, a mode code printed nowhere", mutate(6, 0x06)},
		{"a set whose (10) is 0x00, which the filter column does not print", mutate(7, 0x00)},
		{"a set whose record byte 0 is 0x02, an E6-unmapped SELECT marker", mutate(0, 0x02)},
		{"a set whose record byte 8 is 0x21, an E6-unmapped data mode", mutate(8, 0x21)},
		{"a frame addressed to 0x94 instead of 0x7A",
			append([]byte{0xFE, 0xFE, 0x94}, set[3:]...)},
		{"a frame from a controller other than 0xE0",
			append([]byte{0xFE, 0xFE, 0x7A, 0xE1}, set[4:]...)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if p.AllowedCommand(tc.frame) {
				t.Errorf("AllowedCommand ADMITTED %s:\n  % X", tc.name, tc.frame)
			}
		})
	}

	read, err := p.BuildMemoryRead(civ.ChannelAddress{Channel: 1})
	if err != nil {
		t.Fatalf("BuildMemoryRead(1): %v", err)
	}
	id, err := p.BuildTransceiverIDRead()
	if err != nil {
		t.Fatalf("BuildTransceiverIDRead: %v", err)
	}
	for name, frame := range map[string][]byte{
		"the 1A 00 read":          read.Bytes(),
		"the 1A 00 set":           set,
		"the 19 00 identity read": id.Bytes(),
	} {
		if !p.AllowedCommand(frame) {
			t.Errorf("AllowedCommand REFUSED %s, which is one of the three grammars:\n  % X", name, frame)
		}
	}
}

// TestGate_SingleLengthProfileHasNoWidth pins that Erratum 2's one
// deliberate gate width does not appear on this model: it is
// SINGLE-LENGTH, so the admitted set-lengths and the builder's own length
// coincide exactly. NO CROSS-MODEL CLAIM IS MADE HERE (matrix S3.12(iii)).
func TestGate_SingleLengthProfileHasNoWidth(t *testing.T) {
	p := ic7600.Profile()
	lengths := p.RecordLengths()
	if len(lengths) != 1 || lengths[0] != ic7600.RecordOnlyLength {
		t.Fatalf("RecordLengths() = %v, want [%d]", lengths, ic7600.RecordOnlyLength)
	}
	if got := p.BuildRecordLength(); got != lengths[0] {
		t.Errorf("BuildRecordLength() = %d, want %d", got, lengths[0])
	}

	set := loadGoldenVectors(t)[vecSetRecord]
	const prefix = 6 + ic7600.AddressBytes
	for n := 0; n <= 40; n++ {
		frame := make([]byte, 0, prefix+n+1)
		frame = append(frame, set[:prefix]...)
		body := make([]byte, n)
		copy(body, set[prefix:len(set)-1])
		frame = append(frame, body...)
		frame = append(frame, 0xFD)

		admitted := p.AllowedCommand(frame)
		want := n == ic7600.RecordOnlyLength || n == 0
		if admitted != want {
			what := "a set carrying"
			if n == 0 {
				what = "the READ grammar, i.e. a 1A 00 carrying"
			}
			t.Errorf("AllowedCommand(%s %d record bytes) = %v, want %v - the admitted SET-length set is exactly {%d}, and the only other 1A 00 the gate admits is the zero-data read",
				what, n, admitted, want, ic7600.RecordOnlyLength)
		}
	}
}

// TestGolden_AllFFRecordFailsToParse records D5 entry 2(b) rather than
// deciding it: an answer whose 25 record bytes are all 0xFF fails with a
// parse error naming an offset. THIS PACKAGE DOES NOT CLAIM THAT AN ALL-FF
// RECORD MEANS EMPTY.
func TestGolden_AllFFRecordFailsToParse(t *testing.T) {
	frame := []byte{0xFE, 0xFE, 0xE0, 0x7A, 0x1A, 0x00, 0x00, 0x01}
	for i := 0; i < ic7600.RecordOnlyLength; i++ {
		frame = append(frame, 0xFF)
	}
	frame = append(frame, 0xFD)

	rec, err := ic7600.Profile().ParseMemoryAnswer(frame)
	if err == nil {
		t.Fatalf("ParseMemoryAnswer accepted an all-FF record and produced %v", rec)
	}
	msg := err.Error()
	if !strings.Contains(msg, string(civ.FieldRXFrequency)) || !strings.Contains(msg, "byte") {
		t.Errorf("the parse error names neither the offending field nor a byte position:\n  %v", err)
	}
}
