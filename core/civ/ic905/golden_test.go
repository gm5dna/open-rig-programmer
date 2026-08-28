// SPDX-License-Identifier: GPL-3.0-or-later

package ic905_test

// THIS FILE MAY NOT MODIFY ANY VECTOR, and no failure here is fixed by
// editing one. A golden-versus-codec mismatch is a STOP for orchestrator
// arbitration against the PDF.
//
// WHY THE EXPECTATIONS ARE HARDCODED RATHER THAN PARSED OUT OF THE
// FRAME. A test that parsed a frame and rebuilt it would prove the codec
// is self-consistent and nothing else — an encoder and decoder sharing
// one wrong offset round-trip perfectly. Every expectation below is a
// LITERAL read by hand off the vector's own field map in
// ic905-golden-assumptions.csv, and each carries the printed index it
// was read from.
//
// WHAT THE VECTORS ARE. testdata/ic905-vectors.golden holds four
// name<TAB>hex-frame lines, derived by a quarantined agent from renders
// alone, with ic905-golden-assumptions.csv recording every byte run's
// provenance (78 rows: 43 manual_derived, 16 structural, 15
// manual_documented, 4 inherited_assumed) and ic905-golden-provenance.md
// recording the two assumptions — A1, the read request's four address
// bytes, and A2, the space 0x20 in the name — and the one STOP, the
// frequency field's width.

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic905"
)

// goldenVectorOrder returns the vector names in FILE order.
//
// requireVectorNames pins the count AND the order, so a vector removed
// or reordered fails here rather than silently reducing coverage.
func goldenVectorOrder(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(evidenceDir, "ic905-vectors.golden"))
	if err != nil {
		t.Fatalf("reading the golden vectors: %v", err)
	}
	var names []string
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		name, _, ok := strings.Cut(line, "\t")
		if !ok {
			t.Fatalf("golden vector line %q has no tab separator", line)
		}
		names = append(names, name)
	}
	return names
}

func requireVectorNames(t *testing.T) {
	t.Helper()
	want := []string{
		"read-record",
		"set-record-name-with-space-68",
		"set-record-name-with-space-69",
		"read-transceiver-id",
	}
	if got := goldenVectorOrder(t); !slices.Equal(got, want) {
		t.Fatalf("the golden vector file holds %v, want %v — a vector removed or reordered reduces coverage in silence", got, want)
	}
}

// goldenRecordValues is the neutral record BOTH set vectors carry, read
// by hand off ic905-golden-assumptions.csv, each value with the printed
// index it came from. The frequency is the caller's, because it is the
// only field the two vectors disagree about.
func goldenRecordValues(freqHz uint64) civ.MemoryRecord {
	return civ.MemoryRecord{
		Address:      civ.ChannelAddress{Group: 0, Channel: 1}, // (1),(2) group 00 and (3),(4) channel 01
		RXFreqHz:     civ.Available(freqHz),                    // (6)~(10)
		Mode:         civ.Available("FM"),                      // (11), printed 05:FM
		Filter:       civ.Available("FIL1"),                    // (12), printed 01:FIL1
		DataMode:     civ.Available("OFF"),                     // (13), printed 00: Data mode OFF
		Duplex:       civ.Available("OFF"),                     // (14) left nibble, 0=Duplex OFF
		ToneMode:     civ.Available("OFF"),                     // (14) right nibble, 0=OFF
		ToneTXDeciHz: civ.Available[uint64](885),               // (16)~(18), 088.5 Hz
		ToneRXDeciHz: civ.Available[uint64](885),               // (19)~(21), 088.5 Hz
		DTCSPolarity: civ.Available("NN"),                      // (22), transmit Normal / receive Normal
		DTCSCode:     civ.Available[uint64](23),                // (23),(24), code 023
		OffsetHz:     civ.Available[uint64](0),                 // (26)~(28), 000.000 MHz
		Name:         civ.Available("HIGHLAND BASE905"),        // 53~68
	}
}

// 1. read-transceiver-id. The assumptions CSV records NO ASSUMED BYTE in
// this vector: the command row's Data cell is printed empty (PDF p.6,
// folio 5, "19*1 | 00 | | Read the transceiver ID").
func TestGolden_TransceiverIDRead(t *testing.T) {
	requireVectorNames(t)
	want := loadGoldenVectors(t)["read-transceiver-id"]
	if len(want) != 7 {
		t.Fatalf("read-transceiver-id is %d bytes, want 7", len(want))
	}
	cmd, err := ic905.Profile().BuildTransceiverIDRead()
	if err != nil {
		t.Fatalf("BuildTransceiverIDRead: %v", err)
	}
	requireFrame(t, "read-transceiver-id", cmd.Bytes(), want)
}

// 2. read-record. ITS FOUR ADDRESS BYTES ARE ASSUMED (G's assumption A1;
// D5 entry 1; lift ic905-R-01): the document prints no read form for
// 1A 00, and the read was cut after field (4) because the CLEAR form —
// this document's only demonstration that a 1A 00 frame may stop early —
// stops there too.
func TestGolden_MemoryReadRequest(t *testing.T) {
	requireVectorNames(t)
	want := loadGoldenVectors(t)["read-record"]
	if len(want) != 11 {
		t.Fatalf("read-record is %d bytes, want 11", len(want))
	}
	cmd, err := ic905.Profile().BuildMemoryRead(civ.ChannelAddress{Group: 0, Channel: 1})
	if err != nil {
		t.Fatalf("BuildMemoryRead: %v", err)
	}
	requireFrame(t, "read-record", cmd.Bytes(), want)
}

// 3. set-record-name-with-space-68 — 75 bytes, record 64. This is the
// vector the BUILD direction is pinned against, and it is the reason
// BuildLength is 64.
//
// The name's ninth byte is the ASSUMED 0x20 (G's assumption A2; D5
// entry 3; lift ic905-R-16): the one byte in this record that no printed
// table governing the name field supplies.
func TestGolden_MemorySetIsTheSixtyEightByteVector(t *testing.T) {
	requireVectorNames(t)
	want := loadGoldenVectors(t)["set-record-name-with-space-68"]
	if len(want) != 75 {
		t.Fatalf("set-record-name-with-space-68 is %d bytes, want 75 (7 + 4 address + 64 record)", len(want))
	}
	cmd, err := ic905.Profile().BuildMemorySet(goldenRecordValues(144_500_000))
	if err != nil {
		t.Fatalf("BuildMemorySet: %v", err)
	}
	requireFrame(t, "set-record-name-with-space-68", cmd.Bytes(), want)
}

// 4. set-record-name-with-space-69 — 76 bytes, record 65. BuildMemorySet
// CANNOT produce it, and this test says so explicitly rather than
// omitting the vector.
func TestGolden_TheWideVectorIsAdmittedParsedAndRefusedABuilder(t *testing.T) {
	requireVectorNames(t)
	p := ic905.Profile()
	vector := loadGoldenVectors(t)["set-record-name-with-space-69"]
	if len(vector) != 76 {
		t.Fatalf("set-record-name-with-space-69 is %d bytes, want 76 (7 + 4 address + 65 record)", len(vector))
	}

	// (a) The gate ADMITS it. Spec Erratum 2's one deliberate width,
	//     pinned here on the real model that motivated it.
	if !p.AllowedCommand(vector) {
		t.Errorf("AllowedCommand(the 65-byte set) = false — the gate must admit a set at EITHER declared length (spec Erratum 2)")
	}

	// (b) Turned round into an ANSWER, it parses — the 10 GHz uint64
	//     round trip through the dialect.
	answer := slices.Clone(vector)
	answer[2], answer[3] = answer[3], answer[2] // to/from swapped: an answer runs radio -> controller
	rec, err := p.ParseMemoryAnswer(answer)
	if err != nil {
		t.Fatalf("ParseMemoryAnswer(the 65-byte answer): %v", err)
	}
	assertNumeric(t, "rx_frequency", rec.RXFreqHz, 10_250_000_000)
	assertNumeric(t, "tone_tx", rec.ToneTXDeciHz, 885)
	assertNumeric(t, "tone_rx", rec.ToneRXDeciHz, 885)
	assertNumeric(t, "dtcs_code", rec.DTCSCode, 23)
	assertNumeric(t, "offset", rec.OffsetHz, 0)
	assertText(t, "mode", rec.Mode, "FM")
	assertText(t, "filter", rec.Filter, "FIL1")
	assertText(t, "dtcs_polarity", rec.DTCSPolarity, "NN")
	assertText(t, "name", rec.Name, "HIGHLAND BASE905")

	// (c) And BUILDING it is REFUSED, naming the field. FAILING CLOSED
	//     IS THE BEHAVIOUR, NOT AN ACCIDENT: BuildLength is 64, so
	//     10,250,000,000 Hz does not fit the five-byte frequency field,
	//     and nothing ASSUMED reaches a radio.
	cmd, err := p.BuildMemorySet(goldenRecordValues(10_250_000_000))
	if err == nil {
		t.Fatalf("BuildMemorySet(10.25 GHz) built % X — it must refuse, not truncate", cmd.Bytes())
	}
	if !strings.Contains(err.Error(), string(civ.FieldRXFrequency)) {
		t.Errorf("BuildMemorySet(10.25 GHz) error = %q, want it to name %q", err, civ.FieldRXFrequency)
	}
}

// requireFrame compares a built frame with a golden one and, on a
// mismatch, prints both sides, both lengths and the FIRST DIFFERING WIRE
// POSITION — the failure output is the arbitration's input.
func requireFrame(t *testing.T, name string, got, want []byte) {
	t.Helper()
	if bytes.Equal(got, want) {
		return
	}
	at := -1
	for i := 0; i < min(len(got), len(want)); i++ {
		if got[i] != want[i] {
			at = i
			break
		}
	}
	if at < 0 {
		at = min(len(got), len(want))
	}
	t.Fatalf("%s does not match the golden vector — STOP, and arbitrate against the PDF:\n  built  (%d bytes) % X\n  golden (%d bytes) % X\n  first difference at wire position %d (1-based %d)",
		name, len(got), got, len(want), want, at, at+1)
}
