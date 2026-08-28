// SPDX-License-Identifier: GPL-3.0-or-later

package ic705_test

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic705"
)

// The G leg derived three frames by hand from the rendered pages, with a
// per-byte assumption register beside them. This file replays all three
// through the codec.
//
// THE EXPECTATIONS BELOW ARE LITERALS, read by hand off the vector and off
// the manual's own field map, each carrying the record offset it was read
// from. A test that parsed a frame and rebuilt it would prove the codec
// self-consistent and nothing else — a decoder and an encoder sharing one
// wrong offset round-trip perfectly.
//
// A golden-vs-codec mismatch on the read or the ID vector is a STOP for
// orchestrator arbitration AGAINST THE PDF. No vector is ever edited.
//
// HARDWARE STATUS: UNVERIFIED, all three. Not one has been sent to, or
// captured from, a real IC-705. Green here means the codec agrees with the
// manual as four agents read it, not that any radio accepts these bytes.

const goldenFile = "testdata/vectors.golden"

// loadGoldenVectors reads the vector file into name → frame.
//
// The format is one "name<TAB>hex bytes" record per line. The hex is taken
// after the single tab and split on whitespace; the parser refuses a line
// that is not exactly one tab, and refuses a CR, so a file that acquired
// either would fail loudly rather than be read approximately.
func loadGoldenVectors(t *testing.T) map[string][]byte {
	t.Helper()
	raw := readFile(t, goldenFile)
	out := map[string][]byte{}
	for i, line := range strings.Split(raw, "\n") {
		if line == "" {
			continue
		}
		if strings.Contains(line, "\r") {
			t.Fatalf("%s line %d carries a CR — the vector file is LF-only and a rewritten line ending would change the frozen bytes", goldenFile, i+1)
		}
		name, hexBytes, ok := strings.Cut(line, "\t")
		if !ok || strings.Contains(hexBytes, "\t") {
			t.Fatalf("%s line %d is not exactly one name, one tab and one frame: %q", goldenFile, i+1, line)
		}
		frame, err := hex.DecodeString(strings.ReplaceAll(hexBytes, " ", ""))
		if err != nil {
			t.Fatalf("%s line %d: %v", goldenFile, i+1, err)
		}
		if _, dup := out[name]; dup {
			t.Fatalf("%s line %d: vector %q appears twice", goldenFile, i+1, name)
		}
		out[name] = frame
	}
	if len(out) != 3 {
		t.Fatalf("%s holds %d vectors, want 3 (read-record, set-record-name-with-space, read-transceiver-id)", goldenFile, len(out))
	}
	return out
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := readFileBytes(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(b)
}

func vector(t *testing.T, name string) []byte {
	t.Helper()
	v, ok := loadGoldenVectors(t)[name]
	if !ok {
		t.Fatalf("%s has no vector named %q", goldenFile, name)
	}
	return v
}

func TestGoldenReadRecordIsWhatTheBuilderEmits(t *testing.T) {
	// The vector's own register calls this A1 — the undocumented read
	// form. BUILDING it is how the package commits to that assumption in
	// ONE place: if the form is wrong, it is wrong here and in the driver
	// together, not differently in each.
	want := vector(t, "read-record")
	cmd, err := ic705.Profile().BuildMemoryRead(civ.ChannelAddress{Group: 0, Channel: 12})
	if err != nil {
		t.Fatalf("BuildMemoryRead: %v", err)
	}
	if got := cmd.Bytes(); !bytes.Equal(got, want) {
		t.Errorf("read-record: builder emits % X, the vector holds % X", got, want)
	}
	if len(want) != 11 {
		t.Errorf("the read-record vector is %d bytes, want 11", len(want))
	}
}

func TestGoldenTransceiverIDIsWhatTheBuilderEmits(t *testing.T) {
	want := vector(t, "read-transceiver-id")
	cmd, err := ic705.Profile().BuildTransceiverIDRead()
	if err != nil {
		t.Fatalf("BuildTransceiverIDRead: %v", err)
	}
	if got := cmd.Bytes(); !bytes.Equal(got, want) {
		t.Errorf("read-transceiver-id: builder emits % X, the vector holds % X", got, want)
	}
	if want := []byte{0xFE, 0xFE, 0xA4, 0xE0, 0x19, 0x00, 0xFD}; !bytes.Equal(vector(t, "read-transceiver-id"), want) {
		t.Errorf("the read-transceiver-id vector is % X, want % X", vector(t, "read-transceiver-id"), want)
	}
}

// setRecord strips the set vector down to its 111-byte record: drop the
// six leading framing/command bytes and the trailing FD to reach the
// 115-byte data area, then drop the four address bytes.
func setRecord(t *testing.T) []byte {
	t.Helper()
	frame := vector(t, "set-record-name-with-space")
	if len(frame) != 122 {
		t.Fatalf("the set vector is %d bytes, want 122 = 7 framing/command + 4 address + 111 record", len(frame))
	}
	data := frame[6 : len(frame)-1]
	if len(data) != 115 {
		t.Fatalf("the set vector's data area is %d bytes, want 115", len(data))
	}
	rec := data[4:]
	if len(rec) != 111 {
		t.Fatalf("the set vector's record is %d bytes, want 111", len(rec))
	}
	return rec
}

func TestGoldenSetVectorDecodesFieldByField(t *testing.T) {
	rec := setRecord(t)

	// Read BY HAND off the vector, each with its record offset.
	byHand := []struct {
		off   int
		want  []byte
		means string
	}{
		{0, []byte{0x00}, "split OFF / select OFF"},
		{1, []byte{0x00, 0x00, 0x50, 0x45, 0x01}, "145.500000 MHz, packed BCD little-endian"},
		{6, []byte{0x05}, "FM"},
		{7, []byte{0x01}, "FIL1"},
		{8, []byte{0x00}, "data mode OFF"},
		{9, []byte{0x11}, "DUP- (high nibble) / TONE (low nibble)"},
		{10, []byte{0x00}, "digital squelch OFF, second nibble the printed literal 0"},
		{11, []byte{0x00, 0x08, 0x85}, "88.5 Hz, tone_tx under O-5"},
		{14, []byte{0x00, 0x08, 0x85}, "88.5 Hz, tone_rx under O-5"},
		{17, []byte{0x00}, "DTCS polarity NN"},
		{18, []byte{0x00, 0x23}, "DTCS 023"},
		{20, []byte{0x00}, "DV code squelch"},
		{21, []byte{0x00, 0x60, 0x00}, "600 kHz offset, 100 Hz units little-endian"},
		{24, []byte{0x43, 0x51, 0x43, 0x51, 0x43, 0x51, 0x20, 0x20}, `UR call sign "CQCQCQ  "`},
		{32, bytes.Repeat([]byte{0x20}, 8), "R1 call sign, blank"},
		{40, bytes.Repeat([]byte{0x20}, 8), "R2 call sign, blank"},
		{48, []byte{0x00, 0x00, 0x50, 0x45, 0x01}, "TX frequency (matrix erratum 1: positions 53-57)"},
		{95, []byte("MY REPEATER CH01"), "the sixteen-byte name"},
	}
	for _, c := range byHand {
		got := rec[c.off : c.off+len(c.want)]
		if !bytes.Equal(got, c.want) {
			t.Errorf("record offset %d (%s) = % X, want % X", c.off, c.means, got, c.want)
		}
	}

	// The NOTE panel's own claim, checked rather than quoted: the
	// duplicated block repeats offsets 6..47 at 53..94, byte for byte.
	if !bytes.Equal(rec[53:95], rec[6:48]) {
		t.Errorf("record offsets 53..94 are % X, but offsets 6..47 are % X — the manual's NOTE panel says the block carries the same data", rec[53:95], rec[6:48])
	}

	// Now the ANSWER direction through the codec. The two address bytes
	// swap; nothing else about the frame changes.
	rrec, err := ic705.Profile().ParseMemoryAnswer(answerForm(t, vector(t, "set-record-name-with-space")))
	if err != nil {
		t.Fatalf("ParseMemoryAnswer over the answer form of the set vector: %v", err)
	}
	if rrec.Address != (civ.ChannelAddress{Group: 0, Channel: 12}) {
		t.Errorf("address = %+v, want {Group:0 Channel:12}", rrec.Address)
	}
	wantNum := []struct {
		name string
		got  civ.Optional[uint64]
		want uint64
	}{
		{"RXFreqHz", rrec.RXFreqHz, 145500000},
		{"TXFreqHz", rrec.TXFreqHz, 145500000},
		{"OffsetHz", rrec.OffsetHz, 600000},
		{"ToneTXDeciHz", rrec.ToneTXDeciHz, 885},
		{"ToneRXDeciHz", rrec.ToneRXDeciHz, 885},
		{"DTCSCode", rrec.DTCSCode, 23},
	}
	for _, c := range wantNum {
		v, ok := c.got.Get()
		if !ok {
			t.Errorf("%s is unavailable, want %d", c.name, c.want)
			continue
		}
		if v != c.want {
			t.Errorf("%s = %d, want %d", c.name, v, c.want)
		}
	}
	wantText := []struct {
		name string
		got  civ.Optional[string]
		want string
	}{
		{"Mode", rrec.Mode, "FM"},
		{"Filter", rrec.Filter, "FIL1"},
		{"DataMode", rrec.DataMode, "OFF"},
		{"Duplex", rrec.Duplex, "DUP-"},
		{"ToneMode", rrec.ToneMode, "TONE"},
		{"DTCSPolarity", rrec.DTCSPolarity, "NN"},
		{"Name", rrec.Name, "MY REPEATER CH01"},
	}
	for _, c := range wantText {
		v, ok := c.got.Get()
		if !ok {
			t.Errorf("%s is unavailable, want %q", c.name, c.want)
			continue
		}
		if v != c.want {
			t.Errorf("%s = %q, want %q", c.name, v, c.want)
		}
	}
	// AND NO SELECT FIELD. Record offset 0 is unmapped (O-6), so the
	// parser reports nothing for it and the RAW byte is what the driver's
	// preservation check reads. A Select value here would mean the ★n
	// nibble had been mapped after all, which R6 forbids.
	if !rrec.Select.Unavailable() {
		t.Errorf("Select = %v, want unavailable — record offset 0 is unmapped and nothing may report a value for it", rrec.Select)
	}
}

// answerForm turns a controller-to-radio frame into the radio's answer by
// swapping the two address bytes, and nothing else.
func answerForm(t *testing.T, frame []byte) []byte {
	t.Helper()
	if len(frame) < 5 {
		t.Fatalf("frame % X is too short to have address bytes", frame)
	}
	out := append([]byte(nil), frame...)
	out[2], out[3] = out[3], out[2]
	return out
}

func TestGoldenSetVectorIsRefusedByThisTiersGate(t *testing.T) {
	// NOT A DEFECT, and the reason is the point (O-2/E6). The vector
	// carries CQCQCQ in the UR call sign — an area no neutral field
	// claims and this tier's template fixes at 0x20 — so the gate's
	// re-encode cannot reproduce it and BuildMemorySet cannot produce it.
	//
	// The vector remains the manual's own evidence for every field's
	// POSITION, which Tasks 4-6 consume. What it is not is a frame this
	// tier can send. If an opaque carrier ever lands, this test inverts,
	// and this comment is the record of why it was ever this way round.
	frame := vector(t, "set-record-name-with-space")
	if ic705.Profile().AllowedCommand(frame) {
		t.Error("the gate admitted a record whose DV routing this tier cannot preserve")
	}
}

// blankCallSigns returns the set vector with its three call-sign areas —
// and their copies inside the duplicated TX block — set to 0x20.
func blankCallSigns(t *testing.T, frame []byte) []byte {
	t.Helper()
	out := append([]byte(nil), frame...)
	const recStart = 6 + 4 // framing/command, then the four address bytes
	for _, r := range [][2]int{{24, 47}, {71, 94}} {
		for off := r[0]; off <= r[1]; off++ {
			out[recStart+off] = 0x20
		}
	}
	return out
}

func TestGoldenSetVectorWithBlankCallSignsIsAdmitted(t *testing.T) {
	blank := blankCallSigns(t, vector(t, "set-record-name-with-space"))
	if bytes.Equal(blank, vector(t, "set-record-name-with-space")) {
		t.Fatal("blanking the call signs changed nothing — this test would then be the refusal test over again")
	}
	p := ic705.Profile()
	if !p.AllowedCommand(blank) {
		t.Fatalf("the gate refused the blanked vector % X — this is the write path the tier actually ships", blank)
	}

	// And the builder REPRODUCES it, byte for byte, from the record the
	// parser reads back. This is the manual's own record travelling the
	// whole round trip the driver will make of it.
	rec, err := p.ParseMemoryAnswer(answerForm(t, blank))
	if err != nil {
		t.Fatalf("ParseMemoryAnswer over the blanked vector: %v", err)
	}
	cmd, err := p.BuildMemorySet(rec)
	if err != nil {
		t.Fatalf("BuildMemorySet: %v", err)
	}
	if got := cmd.Bytes(); !bytes.Equal(got, blank) {
		t.Errorf("BuildMemorySet emits\n  % X\nbut the blanked vector is\n  % X", got, blank)
	}
}

func TestGoldenAssumptionsCoverEveryVectorByte(t *testing.T) {
	vectors := loadGoldenVectors(t)

	// half is one nibble of one byte of one vector: the coverage unit,
	// because two of the register's runs claim a single byte one nibble
	// at a time.
	type half struct {
		vector string
		byte   int
		high   bool
	}
	claimed := map[half]string{}
	statuses := map[string]int{}
	rows := 0

	for i, rec := range readCSV(t, "testdata/golden-assumptions.csv") {
		name := rec[0]
		frame, ok := vectors[name]
		if !ok {
			t.Errorf("golden-assumptions.csv row %d claims bytes of vector %q, which %s does not contain", i+1, name, goldenFile)
			continue
		}
		rows++
		first := atoi(t, "golden-assumptions.csv", i, rec[1])
		last := atoi(t, "golden-assumptions.csv", i, rec[3])
		status := rec[7]
		statuses[status]++

		if first < 1 || last < first || last > len(frame) {
			t.Errorf("row %d claims bytes %d..%d of a %d-byte vector %q", i+1, first, last, len(frame), name)
			continue
		}

		// A nibble run names first_nibble/last_nibble as 1 or 2; a
		// whole-byte run prints '-' in both.
		halves := []bool{true, false}
		if rec[2] != "-" || rec[4] != "-" {
			n1, err1 := strconv.Atoi(rec[2])
			n2, err2 := strconv.Atoi(rec[4])
			if err1 != nil || err2 != nil || n1 != n2 || (n1 != 1 && n1 != 2) {
				t.Errorf("row %d has nibbles %q..%q — a nibble run names one nibble of one byte", i+1, rec[2], rec[4])
				continue
			}
			halves = []bool{n1 == 1}
		}

		for b := first; b <= last; b++ {
			for _, h := range halves {
				k := half{name, b, h}
				if prev, dup := claimed[k]; dup {
					t.Errorf("vector %q byte %d's %s nibble is claimed twice — by %q and again at row %d", name, b, nibbleName(h), prev, i+1)
				}
				claimed[k] = fmt.Sprintf("row %d (%s)", i+1, status)
			}
		}

		// The register's own bytes_hex must be the vector's bytes. A run
		// that claims a byte but records a different value would be a
		// register describing a frame nobody has.
		wantHex := strings.Fields(rec[5])
		if len(halves) == 2 {
			if len(wantHex) != last-first+1 {
				t.Errorf("row %d claims bytes %d..%d but records %d hex values", i+1, first, last, len(wantHex))
				continue
			}
		}
		for j, hx := range wantHex {
			b := first + j
			if b > last {
				break
			}
			v, err := strconv.ParseUint(hx, 16, 8)
			if err != nil {
				t.Errorf("row %d: %q is not a hex byte", i+1, hx)
				continue
			}
			if got := frame[b-1]; got != byte(v) {
				t.Errorf("row %d records %s at vector %q byte %d, but the vector holds %02X", i+1, hx, name, b, got)
			}
		}

		if status == "inherited_assumed" && !strings.Contains(rec[10], "Assumption register A") {
			t.Errorf("row %d is inherited_assumed but names no assumption register entry — an inherited assumption with no register entry is an assumption nobody can lift", i+1)
		}
	}

	if rows == 0 {
		t.Fatal("read zero assumption rows — the loader or its filter is broken")
	}
	if statuses["inherited_assumed"] == 0 {
		t.Error("no run carries status inherited_assumed — the register no longer names an assumption, which for these three frames cannot be right")
	}

	for name, frame := range vectors {
		for b := 1; b <= len(frame); b++ {
			for _, h := range []bool{true, false} {
				if _, ok := claimed[half{name, b, h}]; !ok {
					t.Errorf("vector %q byte %d's %s nibble is claimed by NO run — a vector byte the register does not account for is a gap in the evidence, not a pass", name, b, nibbleName(h))
				}
			}
		}
	}
}

// readFileBytes is os.ReadFile, kept behind a helper so that every
// artefact in this package is read the same way.
func readFileBytes(path string) ([]byte, error) { return os.ReadFile(filepath.FromSlash(path)) }
