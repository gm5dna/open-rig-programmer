// SPDX-License-Identifier: GPL-3.0-or-later

package ic7851_test

import (
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7851"
)

// THE GOLDEN LEG IS REPLAYED THROUGH ALL FOUR SEAMS, NOT ONE.
//
// A vector that only ever meets a BUILDER proves the builder agrees with
// itself. Each frame below therefore goes through the builder, through
// AllowedCommand, and — where it is a record — back in through
// MemoryAnswerRecord (raw bytes) and ParseMemoryAnswer (neutral fields),
// and the bytes must be identical at every step.
//
// The vectors and their per-byte provenance are frozen
// (freeze_test.go). The two files are INDEPENDENT renderings of the same
// three frames — one a byte string, one a byte-by-byte anchor table — and
// TestGoldenVectorsAreDerivableFromTheirOwnProvenance rebuilds each frame
// from the second and requires the first.

func golden(t *testing.T) map[string][]byte {
	t.Helper()
	b, err := os.ReadFile("testdata/IC-7851-vectors.golden")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]byte{}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		v, err := hex.DecodeString(strings.Join(parts[1:], ""))
		if err != nil {
			t.Fatalf("golden %s: %v", parts[0], err)
		}
		out[parts[0]] = v
	}
	return out
}

// answerFrame wraps a record in the 1A 00 ANSWER envelope: from the radio,
// to the controller. Written out here from the frame shape the golden
// command vectors themselves show, reversed — the codec builds no answers,
// so a test that wants one has to say what one looks like.
func answerFrame(addr, record []byte) []byte {
	f := []byte{0xfe, 0xfe, 0xe0, 0x8e, 0x1a, 0x00}
	f = append(f, addr...)
	f = append(f, record...)
	return append(f, 0xfd)
}

// TestGoldenFramesAndGate replays the three frozen vectors.
func TestGoldenFramesAndGate(t *testing.T) {
	v := golden(t)
	if len(v) != 3 {
		t.Fatalf("the golden file carries %d vectors, want 3", len(v))
	}
	p := ic7851.Profile()

	id, err := p.BuildTransceiverIDRead()
	if err != nil {
		t.Fatal(err)
	}
	if string(id.Bytes()) != string(v["read-transceiver-id"]) || !p.AllowedCommand(id.Bytes()) {
		t.Fatalf("19 00 = % X, want golden % X (admitted: %v)", id.Bytes(), v["read-transceiver-id"], p.AllowedCommand(id.Bytes()))
	}

	read, err := p.BuildMemoryRead(civ.ChannelAddress{Channel: 1})
	if err != nil {
		t.Fatal(err)
	}
	if string(read.Bytes()) != string(v["read-record"]) || !p.AllowedCommand(read.Bytes()) {
		t.Fatalf("1A 00 read = % X, want golden % X (admitted: %v)", read.Bytes(), v["read-record"], p.AllowedCommand(read.Bytes()))
	}

	// THE SET, AND THE SPACE ASSUMPTION IT CARRIES. The name is
	// "ALPHA BETA": ten characters with a space in the middle, whose code
	// 0x20 is printed on PDF p.260 but NOT in either character-code table
	// on p.261 that the memory-name field points at (register entry
	// ic7851-name-digit-space-codes; disagreement E-3, resolved
	// conservatively as ASCII by the matrix's stated split grade).
	set, err := p.BuildMemorySet(goldenSetRecord())
	if err != nil {
		t.Fatal(err)
	}
	if string(set.Bytes()) != string(v["set-record-name-with-space"]) {
		t.Fatalf("set frame = % X, want golden % X", set.Bytes(), v["set-record-name-with-space"])
	}
	if !p.AllowedCommand(set.Bytes()) {
		t.Fatal("the golden set frame is refused by its own gate")
	}

	// AND BACK IN AGAIN. The same record, in an answer envelope, must
	// parse to the same neutral fields and hand back the same raw bytes.
	record := v["set-record-name-with-space"][8 : len(v["set-record-name-with-space"])-1]
	frame := answerFrame([]byte{0x00, 0x01}, record)
	gotAddr, gotRaw, err := p.MemoryAnswerRecord(frame)
	if err != nil {
		t.Fatalf("MemoryAnswerRecord on the golden record: %v", err)
	}
	if gotAddr != (civ.ChannelAddress{Channel: 1}) {
		t.Errorf("MemoryAnswerRecord addressed %s, want channel 1", gotAddr)
	}
	if string(gotRaw) != string(record) {
		t.Errorf("MemoryAnswerRecord = % X, want the golden record % X", gotRaw, record)
	}
	rec, err := p.ParseMemoryAnswer(frame)
	if err != nil {
		t.Fatalf("ParseMemoryAnswer on the golden record: %v", err)
	}
	if rec != goldenSetRecord() {
		t.Errorf("ParseMemoryAnswer = %+v, want the record the golden frame was built from %+v", rec, goldenSetRecord())
	}

	// EVERY FRAME THIS PACKAGE PRODUCES USES ONLY THE ADMITTED ADDRESSES
	// AND THE TWO ADMITTED COMMANDS.
	for name, frame := range v {
		if frame[2] != 0x8e || frame[3] != 0xe0 {
			t.Errorf("%s is addressed %#02x from %#02x, want 8E from E0", name, frame[2], frame[3])
		}
		cn, sc := frame[4], frame[5]
		if !(cn == 0x19 && sc == 0x00) && !(cn == 0x1a && sc == 0x00) {
			t.Errorf("%s carries command %#02x %#02x; this tier ships 19 00 and 1A 00 and nothing else", name, cn, sc)
		}
	}

	// THE REFUSALS. Each is a form the document PRINTS and this tier does
	// not ship: the two erase shapes, the two memory-bank commands, and
	// every 1A 05 menu write.
	for _, tc := range []struct {
		name  string
		frame []byte
	}{
		{"top-level 09 (memory clear group)", []byte{0xfe, 0xfe, 0x8e, 0xe0, 0x09, 0xfd}},
		{"top-level 0A", []byte{0xfe, 0xfe, 0x8e, 0xe0, 0x0a, 0xfd}},
		{"top-level 0B (memory clear)", []byte{0xfe, 0xfe, 0x8e, 0xe0, 0x0b, 0xfd}},
		{"1A 05 (any menu write)", []byte{0xfe, 0xfe, 0x8e, 0xe0, 0x1a, 0x05, 0xfd}},
		{"the 1A 00 + FF clear form", []byte{0xfe, 0xfe, 0x8e, 0xe0, 0x1a, 0x00, 0x00, 0x01, 0xff, 0xfd}},
		{"a 19 00 ANSWER replayed outbound", []byte{0xfe, 0xfe, 0x8e, 0xe0, 0x19, 0x00, 0x8e, 0xfd}},
		{"a set addressed to another station", func() []byte {
			f := append([]byte(nil), set.Bytes()...)
			f[2] = 0x94
			return f
		}()},
	} {
		if p.AllowedCommand(tc.frame) {
			t.Errorf("the gate admitted %s: % X", tc.name, tc.frame)
		}
	}
}

// goldenSetRecord is the record the golden set vector was built from,
// stated in NEUTRAL terms so the vector's meaning is legible beside its
// bytes: 14.250000 MHz, USB, FIL1, tone OFF, 88.5 Hz repeater tone,
// 100.0 Hz tone squelch, name "ALPHA BETA".
func goldenSetRecord() civ.MemoryRecord {
	return civ.MemoryRecord{
		Address:  civ.ChannelAddress{Channel: 1},
		RXFreqHz: civ.Available(uint64(14250000)),
		Mode:     civ.Available("USB"), Filter: civ.Available("FIL1"),
		ToneMode:     civ.Available("OFF"),
		ToneTXDeciHz: civ.Available(uint64(885)), ToneRXDeciHz: civ.Available(uint64(1000)),
		Name: civ.Available("ALPHA BETA"),
	}
}

// TestGoldenVectorsAreDerivableFromTheirOwnProvenance rebuilds each frozen
// frame from the frozen per-byte anchor table and requires the two to
// agree byte for byte.
//
// THE TWO FILES ARE INDEPENDENT RENDERINGS. IC-7851-vectors.golden is a
// byte string; IC-7851-golden-assumptions.csv is a row per byte or nibble,
// each with its status, its PDF page and the printed anchor it was read
// off. A defect in either is invisible while only one is consumed, which
// is what this test closes. Every row must be used and every byte of every
// frame must be covered exactly once.
func TestGoldenVectorsAreDerivableFromTheirOwnProvenance(t *testing.T) {
	v := golden(t)
	a := readLeg(t, "golden-assumptions", "IC-7851-golden-assumptions.csv")

	built := map[string]map[int]byte{}
	statuses := map[string]int{}
	for _, r := range a.rows {
		r.used = true
		name := r.fields["vector_name"]
		if _, ok := v[name]; !ok {
			t.Fatalf("row %d names the vector %q, which the golden file does not carry", r.num, name)
		}
		switch st := r.fields["status"]; st {
		case "inherited_assumed", "manual_documented", "manual_derived":
			statuses[st]++
		default:
			t.Errorf("row %d carries status %q, which is not one of the leg's three grades", r.num, st)
		}
		// A MANUAL grade must cite a page; an inherited assumption must
		// name its register entry instead and cites none.
		hasPage := r.fields["pdf_page"] != ""
		if wantPage := r.fields["status"] != "inherited_assumed"; hasPage != wantPage {
			t.Errorf("row %d is %s and %s a PDF page", r.num, r.fields["status"], map[bool]string{true: "cites", false: "cites no"}[hasPage])
		}

		first := atoi(t, r.fields["first_byte"])
		last := atoi(t, r.fields["last_byte"])
		vals := parseHexList(t, r.fields["bytes_hex"])
		if built[name] == nil {
			built[name] = map[int]byte{}
		}

		if n1, n2 := r.fields["first_nibble"], r.fields["last_nibble"]; n1 != "-" || n2 != "-" {
			// A NIBBLE ROW. Both ends must name the same nibble of the
			// same byte, and the value must fit in four bits.
			if first != last || n1 != n2 {
				t.Fatalf("row %d spans bytes %d..%d nibbles %s..%s; a nibble row is one nibble of one byte", r.num, first, last, n1, n2)
			}
			if len(vals) != 1 || vals[0] > 0x0f {
				t.Fatalf("row %d gives the nibble value %q", r.num, r.fields["bytes_hex"])
			}
			switch n1 {
			case "1": // the LEFT half, which is the high nibble
				built[name][first] |= vals[0] << 4
			case "2":
				built[name][first] |= vals[0]
			default:
				t.Fatalf("row %d names nibble %q of a two-nibble byte", r.num, n1)
			}
			continue
		}

		if got, want := len(vals), last-first+1; got != want {
			t.Fatalf("row %d gives %d bytes for positions %d..%d", r.num, got, first, last)
		}
		for i, b := range vals {
			if _, dup := built[name][first+i]; dup {
				t.Fatalf("row %d re-states byte %d of %s", r.num, first+i, name)
			}
			built[name][first+i] = b
		}
	}

	for name, want := range v {
		got, ok := built[name]
		if !ok {
			t.Errorf("no provenance row covers the vector %q", name)
			continue
		}
		if len(got) != len(want) {
			t.Errorf("%s: the provenance covers %d byte positions and the vector is %d bytes long", name, len(got), len(want))
		}
		for i, b := range want {
			g, covered := got[i+1] // the CSV's positions are 1-based
			if !covered {
				t.Errorf("%s: byte %d (%#02x) has no provenance row", name, i+1, b)
				continue
			}
			if g != b {
				t.Errorf("%s: the provenance derives %#02x at byte %d and the vector carries %#02x", name, g, i+1, b)
			}
		}
	}

	// THE GRADES ARE NOT ALL ONE THING, and a leg that had quietly become
	// so would be a leg that had stopped reading the page.
	for _, st := range []string{"inherited_assumed", "manual_documented", "manual_derived"} {
		if statuses[st] == 0 {
			t.Errorf("no provenance row is graded %s", st)
		}
	}
	a.requireAllConsumed(t)
}

// TestEveryDeclaredEnumValueSurvivesTheRoundTrip is the replay F3 asks
// for: every mode, every filter and every tone mode, through the builder,
// the gate, the raw-record parser and the answer parser.
//
// THE VOCABULARIES COME FROM THE B TRANSCRIPTION, not from the profile
// being tested: reading the codes off the artefact is what makes this a
// replay of the evidence rather than of the code.
func TestEveryDeclaredEnumValueSurvivesTheRoundTrip(t *testing.T) {
	b := readLeg(t, "transcription-b", "IC-7851-transcription-b.csv")
	var modes, filters, toneModes map[byte]string
	for _, r := range b.rows {
		if r.fields["diagram_id"] != "D1" {
			continue
		}
		lo, _, err := parseIndex(r.fields["field_index"])
		if err != nil {
			t.Fatal(err)
		}
		switch lo {
		case 9:
			modes, filters = splitModeAndFilter(t, r.fields["values_verbatim"])
		case 11:
			toneModes = parseEnum(t, strings.Split(r.fields["values_verbatim"], "|")[1])
		}
	}
	if len(modes) != 10 || len(filters) != 3 || len(toneModes) != 3 {
		t.Fatalf("the transcription gives %d modes, %d filters and %d tone modes; the page prints 10, 3 and 3", len(modes), len(filters), len(toneModes))
	}

	p := ic7851.Profile()
	for _, m := range sortedCodes(modes) {
		for _, f := range sortedCodes(filters) {
			for _, tm := range sortedCodes(toneModes) {
				name := fmt.Sprintf("%s/%s/%s", modes[m], filters[f], toneModes[tm])
				t.Run(name, func(t *testing.T) {
					rec := goldenSetRecord()
					rec.Mode = civ.Available(modes[m])
					rec.Filter = civ.Available(filters[f])
					rec.ToneMode = civ.Available(toneModes[tm])
					replay(t, p, rec, map[int]byte{6: m, 7: f, 8: tm})
				})
			}
		}
	}
}

// TestNameDigitAndSpaceCodesRoundTrip exercises the two name characters
// the referenced code tables do NOT print.
//
// PDF p.261's "Character codes— Letters" and "— Symbols" tables give a hex
// code for every letter and symbol and NONE for the digits or the space,
// while the same page's per-command table says of 1A 00 "Memory name /
// All characters are usable." and PDF p.185 says the repertoire includes
// "numerals … and spaces". The matrix resolves the gap conservatively as
// ASCII, and register entry ic7851-name-digit-space-codes carries the
// single lift: name M-CH02 "A 1" at the front panel and read back bytes
// ⑱⑲⑳.
//
// THE ASSUMPTION IS PINNED AS BYTES so the lift has something exact to
// contradict.
func TestNameDigitAndSpaceCodesRoundTrip(t *testing.T) {
	p := ic7851.Profile()
	rec := goldenSetRecord()
	rec.Name = civ.Available("A 1")
	want := map[int]byte{
		15: 'A',  // 0x41, printed on p.261
		16: 0x20, // the space: ASSUMED
		17: '1',  // 0x31: ASSUMED
		18: 0x20, // and the pad, which is the same byte (ic7851-name-pad-byte)
	}
	replay(t, p, rec, want)

	// A NAME ENDING IN THE PAD BYTE DOES NOT ROUND-TRIP, and the profile
	// says so rather than hiding it: padding erases the data-versus-fill
	// distinction on the wire. Recorded here as a consequence of the
	// 0x20 pad rather than left for a caller to discover.
	rec.Name = civ.Available("AB ")
	cmd, err := p.BuildMemorySet(rec)
	if err != nil {
		t.Fatal(err)
	}
	back, err := p.ParseMemoryAnswer(answerFrame([]byte{0x00, 0x01}, cmd.Bytes()[8:len(cmd.Bytes())-1]))
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := back.Name.Get(); got != "AB" {
		t.Errorf("a name ending in the pad byte read back as %q, want the trimmed %q — if this ever changes, the pad-byte assumption has changed with it", got, "AB")
	}
}

// replay builds one record, checks the named record bytes, runs the gate,
// and parses the same bytes back through both answer parsers.
func replay(t *testing.T, p civ.Profile, rec civ.MemoryRecord, wantBytes map[int]byte) {
	t.Helper()
	cmd, err := p.BuildMemorySet(rec)
	if err != nil {
		t.Fatalf("BuildMemorySet: %v", err)
	}
	frame := cmd.Bytes()
	record := frame[8 : len(frame)-1]
	if len(record) != ic7851.RecordOnlyLength {
		t.Fatalf("the built record is %d bytes, want %d", len(record), ic7851.RecordOnlyLength)
	}
	for off, want := range wantBytes {
		if record[off] != want {
			t.Errorf("record byte %d = %#02x, want %#02x", off, record[off], want)
		}
	}
	if !p.AllowedCommand(frame) {
		t.Fatalf("the gate refused a frame this profile's own builder produced: % X", frame)
	}
	answer := answerFrame([]byte{0x00, 0x01}, record)
	_, raw, err := p.MemoryAnswerRecord(answer)
	if err != nil {
		t.Fatalf("MemoryAnswerRecord: %v", err)
	}
	if string(raw) != string(record) {
		t.Errorf("MemoryAnswerRecord = % X, want % X", raw, record)
	}
	back, err := p.ParseMemoryAnswer(answer)
	if err != nil {
		t.Fatalf("ParseMemoryAnswer: %v", err)
	}
	if back != rec {
		t.Errorf("ParseMemoryAnswer = %+v, want %+v", back, rec)
	}
	// AND THE PARSED RECORD REBUILDS THE SAME FRAME. This is the same
	// identity AllowedCommand's re-encode leg relies on, asserted here on
	// the parser's own output rather than on the gate's.
	again, err := p.BuildMemorySet(back)
	if err != nil {
		t.Fatalf("rebuilding the parsed record: %v", err)
	}
	if string(again.Bytes()) != string(frame) {
		t.Errorf("the parsed record rebuilds % X, want % X", again.Bytes(), frame)
	}
}

func sortedCodes(m map[byte]string) []byte {
	out := make([]byte, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func atoi(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		t.Fatalf("%q is not a number: %v", s, err)
	}
	return n
}

func parseHexList(t *testing.T, s string) []byte {
	t.Helper()
	var out []byte
	for _, f := range strings.Fields(s) {
		v, err := strconv.ParseUint(f, 16, 8)
		if err != nil {
			t.Fatalf("%q is not a hexadecimal byte: %v", f, err)
		}
		out = append(out, byte(v))
	}
	if len(out) == 0 {
		t.Fatalf("%q carries no bytes", s)
	}
	return out
}
