// SPDX-License-Identifier: GPL-3.0-or-later

package ic9700_test

import (
	"bytes"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic9700"
)

// THE FROZEN VECTORS, REPLAYED — every one the artefact carries.
//
// Leg G produced four records in testdata/ic9700-vectors.golden:
//
//	read-record                 10 B  FE FE A2 E0 1A 00 01 00 01 FD
//	set-record-name-with-space 121 B  the full record, name INVERNESS GB3CFR
//	read-transceiver-id          7 B  FE FE A2 E0 19 00 FD
//	manual-example-1             8 B  FE FE FE A2 E0 18 01 FD
//
// ON THE COUNT. The tier's plan of record says "G produces 3 vectors";
// that was G's PRODUCTION BRIEF, not a cap on what may be replayed, and a
// frozen quarantined artefact is never edited to reach a number. The
// fourth vector is additional coverage, not a deviation, and
// TestEveryGoldenVectorIsExercised is what keeps every vector replayed if
// a later freeze carries more.
//
// MANUAL-EXAMPLE-1 IS NOT A BUILDER VECTOR AND MUST NEVER BECOME ONE. It
// is transcribed from PDF p.13 footnote *4's worked example box; this tier
// never sends 18 01. It earns its place twice — as the accumulator's
// preamble-padding case, and as a gate-refusal case — and both are pinned
// below.
//
// A GOLDEN-VERSUS-CODEC MISMATCH IS A STOP. No test here may modify a
// vector: the disagreement is reported byte by byte and arbitrated against
// the PDF.

// goldenFrame returns one frozen vector's bytes by name.
func goldenFrame(t *testing.T, name string) []byte {
	t.Helper()
	for _, v := range goldenVectors(t) {
		if v.name == name {
			return v.frame
		}
	}
	t.Fatalf("the frozen vector file has no %q", name)
	return nil
}

// goldenNames returns every vector's name, in file order.
func goldenNames(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, v := range goldenVectors(t) {
		out = append(out, v.name)
	}
	return out
}

// swapAddresses turns a controller-to-radio frame into the radio-to-
// controller frame with the same body, by exchanging the `to` and `from`
// bytes.
//
// It finds them AFTER the preamble run rather than at fixed indices,
// because manual-example-1 carries three preamble bytes and a fixed index
// would swap a preamble byte into an address.
func swapAddresses(frame []byte) []byte {
	out := append([]byte(nil), frame...)
	i := 0
	for i < len(out) && out[i] == civ.PreambleByte {
		i++
	}
	if i+1 >= len(out) {
		return out
	}
	out[i], out[i+1] = out[i+1], out[i]
	return out
}

// ic9700GoldenRecord is leg G's byte walk, as a neutral record: all
// fourteen mapped field ids of the set-record vector.
func ic9700GoldenRecord() civ.MemoryRecord {
	return civ.MemoryRecord{
		Address:      civ.ChannelAddress{Group: 1, Channel: 1},
		Select:       civ.Available("OFF"),
		RXFreqHz:     civ.Available(uint64(145_500_000)),
		Mode:         civ.Available("FM"),
		Filter:       civ.Available("FIL1"),
		DataMode:     civ.Available("OFF"),
		Duplex:       civ.Available("OFF"),
		ToneMode:     civ.Available("OFF"),
		ToneTXDeciHz: civ.Available(uint64(885)),
		ToneRXDeciHz: civ.Available(uint64(885)),
		DTCSPolarity: civ.Available("NN"),
		DTCSCode:     civ.Available(uint64(23)),
		OffsetHz:     civ.Available(uint64(600_000)),
		TXFreqHz:     civ.Available(uint64(145_500_000)),
		Name:         civ.Available("INVERNESS GB3CFR"),
	}
}

func TestGoldenReadRecordIsWhatTheBuilderBuilds(t *testing.T) {
	p := ic9700.Profile()
	// Group carries the WIRE index under E4, so band 01 is Group 1.
	cmd, err := p.BuildMemoryRead(civ.ChannelAddress{Group: 1, Channel: 1})
	if err != nil {
		t.Fatalf("BuildMemoryRead: %v", err)
	}
	if got, want := cmd.Bytes(), goldenFrame(t, "read-record"); !bytes.Equal(got, want) {
		t.Fatalf("frame = % X\nwant   % X", got, want)
	}
	if !p.AllowedCommand(cmd.Bytes()) {
		t.Error("the gate refuses this package's own read request")
	}
}

func TestGoldenSetRecordIsWhatTheBuilderBuilds(t *testing.T) {
	p := ic9700.Profile()
	rec := ic9700GoldenRecord() // every one of the 14 mapped ids, per the G walk
	cmd, err := p.BuildMemorySet(rec)
	if err != nil {
		t.Fatalf("BuildMemorySet: %v", err)
	}
	want := goldenFrame(t, "set-record-name-with-space")
	if got := cmd.Bytes(); !bytes.Equal(got, want) {
		t.Fatalf("frame = % X\nwant   % X", got, want)
	}
	if got, want := len(want), 121; got != want {
		t.Errorf("set frame is %d bytes, want %d", got, want)
	}
	// FE FE <to> <from> 1A 00 … FD is seven bytes of envelope; what is
	// left is the address plus the record, i.e. the data area.
	if got, want := len(want)-7, ic9700.DataAreaLength; got != want {
		t.Errorf("data area is %d bytes, want %d", got, want)
	}
	if !p.AllowedCommand(cmd.Bytes()) {
		t.Error("the gate refuses this package's own set frame")
	}
}

func TestGoldenSetRecordRoundTripsThroughTheParser(t *testing.T) {
	p := ic9700.Profile()
	answer := swapAddresses(goldenFrame(t, "set-record-name-with-space"))
	addr, raw, err := p.MemoryAnswerRecord(answer)
	if err != nil {
		t.Fatalf("MemoryAnswerRecord: %v", err)
	}
	if got, want := len(raw), ic9700.RecordLength; got != want {
		t.Fatalf("record is %d bytes, want %d — the fingerprint measures the RECORD, not the %d-byte data area",
			got, want, ic9700.DataAreaLength)
	}
	if addr.Group != 1 || addr.Channel != 1 {
		t.Errorf("address = %v, want band 1 channel 1", addr)
	}
	rec, err := p.ParseMemoryAnswer(answer)
	if err != nil {
		t.Fatalf("ParseMemoryAnswer: %v", err)
	}
	if rec != ic9700GoldenRecord() {
		t.Errorf("round trip changed the record:\n got %+v\nwant %+v", rec, ic9700GoldenRecord())
	}
	if name, _ := rec.Name.Get(); name != "INVERNESS GB3CFR" {
		t.Errorf("name = %q, want %q — the embedded space is ASSUMED (D5 entry 3, lift W4)", name, "INVERNESS GB3CFR")
	}
}

func TestGoldenTransceiverIDRead(t *testing.T) {
	p := ic9700.Profile()
	cmd, err := p.BuildTransceiverIDRead()
	if err != nil {
		t.Fatalf("BuildTransceiverIDRead: %v", err)
	}
	if got, want := cmd.Bytes(), goldenFrame(t, "read-transceiver-id"); !bytes.Equal(got, want) {
		t.Fatalf("frame = % X, want % X", got, want)
	}
}

func TestManualExampleIsNormalisedByTheAccumulatorThenRefusedByTheGate(t *testing.T) {
	// Feeding the gate the RAW three-FE example and calling the refusal a
	// grammar assertion does not work: civ.WellFormed requires exactly two
	// leading FE bytes, so the raw vector is refused on FRAMING before any
	// grammar is consulted. Both halves are pinned separately here — the
	// accumulator does the normalising, and the GATE is then asked about
	// the normalised frame, which is the only form that tests the grammar
	// at all.
	p := ic9700.Profile()
	raw := goldenFrame(t, "manual-example-1") // FE FE FE A2 E0 18 01 FD

	// (a) the raw vector is not well formed, and that alone is not the
	//     claim we care about — assert it so the reason is on the record.
	if p.AllowedCommand(raw) {
		t.Fatal("the gate admitted the raw three-FE example")
	}

	// (b) the accumulator tolerates the extra preamble byte and hands
	//     back a normalised frame. This radio can see up to 119 repeated
	//     FE bytes from other software on a shared REMOTE bus, and PDF
	//     p.13 footnote *4 draws five.
	acc := civ.NewFrameAccumulator(p.MaxFrame(), p.ControllerAddress())
	frames, err := acc.Push(swapAddresses(raw)) // addressed to the controller, or the filter drops it
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("got %d frames from the padded example, want 1", len(frames))
	}
	if got, want := frames[0], swapAddresses(raw)[1:]; !bytes.Equal(got, want) {
		t.Fatalf("normalised frame = % X, want % X (one FE stripped)", got, want)
	}

	// (c) THE GRAMMAR ASSERTION. The normalised, well-formed `18 01`
	//     frame in the controller-to-radio direction is refused because
	//     the gate admits only 19 00, 1A 00 read and 1A 00 set — nothing
	//     in this tier may send a power-on command.
	normalised := raw[1:] // FE FE A2 E0 18 01 FD — well formed, wrong command
	if !civ.WellFormed(normalised) {
		t.Fatal("the normalised example is not well formed; the grammar assertion below would be vacuous")
	}
	if p.AllowedCommand(normalised) {
		t.Error("the gate admitted a well-formed `18 01`; the tier admits three grammars and no more")
	}
}

func TestEveryGoldenVectorIsExercised(t *testing.T) {
	// A vector nothing replays is a vector nothing checks.
	want := map[string]bool{
		"read-record": true, "set-record-name-with-space": true,
		"read-transceiver-id": true, "manual-example-1": true,
	}
	got := goldenNames(t)
	if len(got) != len(want) {
		t.Fatalf("testdata/ic9700-vectors.golden has %d vectors, this test replays %d", len(got), len(want))
	}
	for _, n := range got {
		if !want[n] {
			t.Errorf("vector %q is in the file and in no test", n)
		}
	}
}
