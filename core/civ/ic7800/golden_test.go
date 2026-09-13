// SPDX-License-Identifier: GPL-3.0-or-later

package ic7800_test

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7800"
)

// This file is the mechanical byte-compare of this dialect's codec against
// the four hand-derived wire frames in testdata/ic7800-vectors.golden, and
// the positive proof of what the outbound gate REFUSES.
//
// # What the vectors are, and what this file may do with them
//
// Derived by hand from the matrix (docs/superpowers/icom-matrices/
// ic7800-capability-matrix.md) and the IC-7800 Instruction Manual Section
// 14 (PDF pp.200-217), a SINGLE evidence source — unlike the IC-7610
// lineage's four independently-blind legs, this model's matrix was
// authored by one reading of one document (matrix §0), and this package's
// testdata follows suit rather than fabricating an independence it does
// not have. Provenance for the set-record vector is itemised in
// testdata/ic7800-golden-provenance.md.
//
// THIS FILE MAY NOT MODIFY ANY VECTOR, and no failure here is ever fixed by
// editing one. A golden-vs-codec mismatch is a STOP for orchestrator
// arbitration AGAINST THE PDF - either the hand derivation or the codec
// misreads the manual - which is why every comparison prints both sides, both
// lengths and the first differing wire position: the failure output is the
// arbitration's input.
//
// # Why the expectations are literals rather than parsed out of the frame
//
// A test that parsed a frame and rebuilt it would prove the codec
// self-consistent and nothing else: a decoder and an encoder sharing one
// wrong offset would round-trip perfectly. goldenRecord() below therefore
// states the vector's fields as LITERALS read by hand off the provenance
// notes, each carrying the frame byte position it came from.
//
// # Hardware status
//
// UNVERIFIED, for all four vectors. Not one has been sent to, or captured
// from, a real IC-7800. Green here means the codec agrees with the manual as
// one agent read it, not that any radio accepts these bytes.

// goldenRecord is the neutral record the set-record-name-with-space vector
// encodes, stated as LITERALS read by hand, each carrying the frame byte
// position it came from (see testdata/ic7800-golden-provenance.md).
//
// Select and DataMode are deliberately ABSENT (the zero Optional, i.e.
// Unavailable): ruling E6 leaves both regions unmapped on this model, so
// the encoder writes the Fixed template's zeros there.
func goldenRecord() civ.MemoryRecord {
	return civ.MemoryRecord{
		Address:      civ.ChannelAddress{Channel: 1},    // frame bytes 7-8: 00 01
		RXFreqHz:     civ.Available[uint64](14_250_000), // frame bytes 10-14: 00 00 25 14 00
		Mode:         civ.Available("USB"),              // frame byte 15: 01
		Filter:       civ.Available("FIL1"),             // frame byte 16: 01
		ToneMode:     civ.Available("TONE"),             // frame byte 17 low nibble: 1
		ToneTXDeciHz: civ.Available[uint64](885),        // frame bytes 18-20: 00 08 85
		ToneRXDeciHz: civ.Available[uint64](1000),       // frame bytes 21-23: 00 10 00
		Name:         civ.Available("HOME QTH01"),       // frame bytes 24-33
	}
}

// goldenFile is the four-line vector file, relative to this package.
const goldenFile = "ic7800-vectors.golden"

// The four vector names, and what may be done with each.
const (
	vecReadRecord    = "read-record"                // 9 bytes  - BuildMemoryRead(1)
	vecSetRecord     = "set-record-name-with-space" // 34 bytes - BuildMemorySet(goldenRecord())
	vecReadID        = "read-transceiver-id"        // 7 bytes  - BuildTransceiverIDRead()
	vecManualExample = "manual-example-14"          // 14 bytes - DOCUMENTARY ONLY, and REFUSED
)

// loadGoldenVectors parses the file STRICTLY: exactly four lines, exactly the
// four names below, no blank lines, every token exactly two hex digits. A
// permissive parser here would let a mangled vector pass as evidence.
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

// firstDiff reports the 1-indexed position of the first byte at which a and
// b differ, or a length-mismatch message if one is a prefix of the other.
func firstDiff(a, b []byte) string {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return strconv.Itoa(i + 1)
		}
	}
	if len(a) != len(b) {
		return "none within the shorter frame - lengths differ"
	}
	return "none"
}

// requireFrame compares a built command with a golden vector and prints both
// sides, both lengths and the first differing wire position on failure.
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

// TestGolden_ReadRecord binds the read builder to the G leg's nine-byte
// vector.
//
// BYTES 7-8 ARE THE LEG'S SINGLE inherited_assumed RUN: the document prints
// no 1A 00 read request at all, so what green here means is that the builder
// agrees with the ASSUMPTION - D5 entry 1, register entry lifted by matrix
// R1's capture ic7800-read-request-ch01 - and not that any radio does.
func TestGolden_ReadRecord(t *testing.T) {
	want := loadGoldenVectors(t)[vecReadRecord]
	cmd, err := ic7800.Profile().BuildMemoryRead(civ.ChannelAddress{Channel: 1})
	if err != nil {
		t.Fatalf("BuildMemoryRead(1): %v", err)
	}
	requireFrame(t, "BuildMemoryRead(channel 1)", cmd.Bytes(), want)
}

// TestGolden_SetRecord binds the set builder to the 34-byte vector.
func TestGolden_SetRecord(t *testing.T) {
	want := loadGoldenVectors(t)[vecSetRecord]
	cmd, err := ic7800.Profile().BuildMemorySet(goldenRecord())
	if err != nil {
		t.Fatalf("BuildMemorySet(goldenRecord()): %v", err)
	}
	requireFrame(t, "BuildMemorySet(goldenRecord())", cmd.Bytes(), want)
}

// TestGolden_SetRecordParsesBack turns the same vector into an
// ANSWER-direction frame and requires the codec's READING of those bytes to
// equal the derivation's stated intent. Binding both directions to one
// vector is what stops a shared wrong offset round-tripping perfectly.
func TestGolden_SetRecordParsesBack(t *testing.T) {
	vec := loadGoldenVectors(t)[vecSetRecord]
	answer := make([]byte, len(vec))
	copy(answer, vec)
	answer[2], answer[3] = answer[3], answer[2] // swap to/from: FE FE E0 98 1A 00 ...

	rec, err := ic7800.Profile().ParseMemoryAnswer(answer)
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
		t.Errorf("Select = %v, want Unavailable - ruling E6 leaves record byte 0 (printed 'e')'s low nibble unmapped, and an unmapped region is not decoded", rec.Select)
	}
	if !rec.DataMode.Unavailable() {
		t.Errorf("DataMode = %v, want Unavailable - ruling E6 leaves record byte 8 (printed '!1')'s high nibble unmapped", rec.DataMode)
	}
}

// TestGolden_TransceiverID binds the probe's request to the seven-byte
// vector, and then asserts NOTHING ABOUT WHICH TOKEN comes back.
//
// D5 entry 7 records the 19 00 reply VALUE as undocumented on all six models
// in this tier. The probe therefore requires an ADDRESS-MATCHED reply and
// records the token as a diagnostic; it never matches it. Matrix lift R7's
// capture ic7800-id-1900 is what would give this radio's token a value to be
// compared against, and until it is taken a test that expected a particular
// token would be asserting a fact nobody has.
func TestGolden_TransceiverID(t *testing.T) {
	p := ic7800.Profile()
	want := loadGoldenVectors(t)[vecReadID]
	cmd, err := p.BuildTransceiverIDRead()
	if err != nil {
		t.Fatalf("BuildTransceiverIDRead: %v", err)
	}
	requireFrame(t, "BuildTransceiverIDRead()", cmd.Bytes(), want)

	for _, token := range []byte{0x6A, 0x00, 0x7A} {
		frame := []byte{0xFE, 0xFE, 0xE0, 0x6A, 0x19, 0x00, token, 0xFD}
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
// vector is DOCUMENTARY ONLY: a padded power-ON command (18 01), the
// family-wide preamble-padding convention CI-V radios use before the radio
// is known to be listening. This tier sends no such command: core/civ has
// no builder for it, AllowedCommand has no branch that could admit it, and
// the vector is committed because it is evidence ABOUT THE PREAMBLE, not
// because anything may replay it.
func TestGolden_ManualExampleIsRefused(t *testing.T) {
	p := ic7800.Profile()
	padded := loadGoldenVectors(t)[vecManualExample]
	if p.AllowedCommand(padded) {
		t.Errorf("AllowedCommand admitted the manual's worked example % X; this tier never sends 18 01", padded)
	}
	unpadded := []byte{0xFE, 0xFE, 0x6A, 0xE0, 0x18, 0x01, 0xFD}
	if p.AllowedCommand(unpadded) {
		t.Errorf("AllowedCommand admitted the unpadded power-ON frame % X", unpadded)
	}
}

// TestGate_RefusesEverythingButTheThreeGrammars is the gate's negative half.
// The gate admits 19 00, a valid 1A 00 read and a re-validated 1A 00 set, and
// each row below names the reason it is not one of those three.
func TestGate_RefusesEverythingButTheThreeGrammars(t *testing.T) {
	p := ic7800.Profile()
	set := loadGoldenVectors(t)[vecSetRecord]
	const prefix = 6 + ic7800.AddressBytes

	// mutate returns a copy of the golden set frame with one record byte
	// changed, so every row below differs from an ADMITTED frame in exactly
	// the way its name says.
	mutate := func(offset int, value byte) []byte {
		out := make([]byte, len(set))
		copy(out, set)
		out[prefix+offset] = value
		return out
	}
	// resize returns a set frame carrying n record bytes.
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
		{"the 1A 00 clear form (PDF p.211's channel+FF note)",
			[]byte{0xFE, 0xFE, 0x6A, 0xE0, 0x1A, 0x00, 0x00, 0x01, 0xFF, 0xFD}},
		{"command 0B, Memory clear (PDF p.201)",
			[]byte{0xFE, 0xFE, 0x6A, 0xE0, 0x0B, 0xFD}},
		{"a transceive SET - NO RADIO MUTATION AT INIT, EVER",
			[]byte{0xFE, 0xFE, 0x6A, 0xE0, 0x1A, 0x05, 0x01, 0x12, 0x01, 0xFD}},
		{"a 1A 05 read of the USB echo item - any 1A 05 at all",
			[]byte{0xFE, 0xFE, 0x6A, 0xE0, 0x1A, 0x05, 0x01, 0x16, 0xFD}},
		{"a set at 24 record bytes, a length this profile does not declare", resize(24)},
		{"a set at 26 record bytes, a length this profile does not declare", resize(26)},
		{"a set whose mode byte is 0x06, a mode code printed nowhere", mutate(6, 0x06)},
		{"a set whose filter byte is 0x00, which the filter column does not print", mutate(7, 0x00)},
		{"a set whose record byte 0 is 0x02, an E6-unmapped SELECT marker", mutate(0, 0x02)},
		{"a set whose record byte 8 is 0x21, an E6-unmapped data mode", mutate(8, 0x21)},
		{"a frame addressed to 0x94 instead of 0x6A",
			append([]byte{0xFE, 0xFE, 0x94}, set[3:]...)},
		{"a frame from a controller other than 0xE0",
			append([]byte{0xFE, 0xFE, 0x6A, 0xE1}, set[4:]...)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if p.AllowedCommand(tc.frame) {
				t.Errorf("AllowedCommand ADMITTED %s:\n  % X", tc.name, tc.frame)
			}
		})
	}

	// The gate is only worth testing if it still admits the three grammars.
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

// TestGate_SingleLengthProfileHasNoWidth pins that Erratum 2's one deliberate
// gate width does not appear on this model.
//
// The erratum admits a set at any length a MULTI-length profile declares,
// which on the IC-905 is genuinely wider than what its builder emits. THE
// IC-7800 IS SINGLE-LENGTH, so the admitted set-lengths and the builder's own
// length coincide exactly, and this test is where that coincidence is made
// mechanical rather than assumed. NO CROSS-MODEL CLAIM IS MADE HERE: whether
// 25 tells this radio apart from its siblings is a tier-level Wave-4 check.
func TestGate_SingleLengthProfileHasNoWidth(t *testing.T) {
	p := ic7800.Profile()
	lengths := p.RecordLengths()
	if len(lengths) != 1 || lengths[0] != ic7800.RecordOnlyLength {
		t.Fatalf("RecordLengths() = %v, want [%d]", lengths, ic7800.RecordOnlyLength)
	}
	if got := p.BuildRecordLength(); got != lengths[0] {
		t.Errorf("BuildRecordLength() = %d, want %d - on a single-length profile the two cannot differ", got, lengths[0])
	}

	set := loadGoldenVectors(t)[vecSetRecord]
	const prefix = 6 + ic7800.AddressBytes
	for n := 0; n <= 40; n++ {
		frame := make([]byte, 0, prefix+n+1)
		frame = append(frame, set[:prefix]...)
		body := make([]byte, n)
		copy(body, set[prefix:len(set)-1])
		frame = append(frame, body...)
		frame = append(frame, 0xFD)

		// n == 0 is not a short set at all: a 1A 00 carrying the address
		// and no data IS the read grammar, and the gate admits it as one.
		// Every other length below 25 and every length above it is a set at
		// a width this profile does not declare.
		admitted := p.AllowedCommand(frame)
		want := n == ic7800.RecordOnlyLength || n == 0
		if admitted != want {
			what := "a set carrying"
			if n == 0 {
				what = "the READ grammar, i.e. a 1A 00 carrying"
			}
			t.Errorf("AllowedCommand(%s %d record bytes) = %v, want %v - the admitted SET-length set is exactly {%d}, and the only other 1A 00 the gate admits is the zero-data read",
				what, n, admitted, want, ic7800.RecordOnlyLength)
		}
	}
}

// TestGolden_AllFFRecordFailsToParse records D5 entry 2(b) rather than
// deciding it.
//
// An answer whose 25 record bytes are all 0xFF fails with a parse error
// naming an offset, because 0xF is in none of the three mapped enums and is
// not a BCD digit. THIS PACKAGE DOES NOT CLAIM THAT AN ALL-FF RECORD MEANS
// EMPTY: matrix lift R2b's capture ic7800-ff-record-ch50 is what would settle
// it, and the driver's own empty-slot recognition keys on
// transport.ErrRejected (an FA consumed by the framing), never on a frame
// that reached this parser.
func TestGolden_AllFFRecordFailsToParse(t *testing.T) {
	frame := []byte{0xFE, 0xFE, 0xE0, 0x6A, 0x1A, 0x00, 0x00, 0x01}
	for i := 0; i < ic7800.RecordOnlyLength; i++ {
		frame = append(frame, 0xFF)
	}
	frame = append(frame, 0xFD)

	rec, err := ic7800.Profile().ParseMemoryAnswer(frame)
	if err == nil {
		t.Fatalf("ParseMemoryAnswer accepted an all-FF record and produced %v; 0xF is in none of the three mapped enums", rec)
	}
	// The error must say WHERE the record stopped making sense - which field
	// and which byte of it - or a reader meeting it has nothing to arbitrate
	// against the page. It stops at the FIRST mapped span, rx_frequency at
	// record offset 1, because 0xFF is not a packed-BCD pair.
	msg := err.Error()
	if !strings.Contains(msg, string(civ.FieldRXFrequency)) || !strings.Contains(msg, "byte") {
		t.Errorf("the parse error names neither the offending field nor a byte position, so it cannot tell a reader WHERE the record stopped making sense:\n  %v", err)
	}
}
