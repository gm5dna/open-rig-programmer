// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx5000

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// populatedChannel is a plain MEM channel with a Known tone, as a read of
// this radio always leaves one — the shape a write must be able to round-
// trip.
func populatedChannel(freqHz uint64) *codeplug.ChannelData {
	return &codeplug.ChannelData{
		FreqHz:     freqHz,
		Mode:       "USB",
		CTCSS:      "OFF",
		CTCSSTone:  codeplug.ToneField{State: codeplug.Known, Value: 670},
		Shift:      "SIMPLEX",
		TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
		ScanSkip:   codeplug.BoolField{State: codeplug.Unavailable},
	}
}

// TestWriteChannel_Accepted: a well-formed write of a Simulated (writable)
// session's MEM slot sends exactly one MW frame and reports it
// sent/confirmed.
func TestWriteChannel_Accepted(t *testing.T) {
	p, sess := openSession(t, Simulated, slotImage{})

	res, err := sess.WriteChannel(testCtx(t), codeplug.Channel{Slot: "010", Data: populatedChannel(14_250_000)})
	if err != nil {
		t.Fatalf("WriteChannel = %v, want nil", err)
	}
	if len(res.Steps) != 1 || res.Steps[0].Command != "MW" || !res.Steps[0].Sent || !res.Steps[0].Confirmed {
		t.Fatalf("WriteResult = %+v, want one confirmed MW step", res)
	}

	transcript := p.Transcript()
	last := transcript[len(transcript)-1]
	if !strings.HasPrefix(last, "MW010") {
		t.Errorf("last frame = %q, want an MW Set for slot 010", last)
	}
	if len(last) != mwSetFrameLen {
		t.Errorf("MW frame is %d bytes, want %d", len(last), mwSetFrameLen)
	}
}

// TestWriteChannel_RejectedByRadio: a "?;" answer to the MW Set surfaces
// as an error naming the rejection.
func TestWriteChannel_RejectedByRadio(t *testing.T) {
	_, sess := openSession(t, Simulated, slotImage{rejectSets: true})

	_, err := sess.WriteChannel(testCtx(t), codeplug.Channel{Slot: "010", Data: populatedChannel(14_250_000)})
	if err == nil {
		t.Fatal("WriteChannel = nil, want an error: the radio rejected the Set")
	}
}

// TestWriteChannel_RefusedOnUnverifiedProfile: a RealHardware (unconsented)
// session refuses every write before a frame is built.
func TestWriteChannel_RefusedOnUnverifiedProfile(t *testing.T) {
	p, sess := openSession(t, RealHardware, slotImage{})

	_, err := sess.WriteChannel(testCtx(t), codeplug.Channel{Slot: "010", Data: populatedChannel(14_250_000)})
	if err == nil {
		t.Fatal("WriteChannel on an unconsented RealHardware session = nil, want a refusal")
	}
	for _, f := range p.Transcript()[2:] {
		t.Errorf("unexpected wire frame %q — an Unverified write must be refused before any frame is built", f)
	}
}

// TestWriteChannel_Erase: an empty channel (Data nil) is refused as an
// erase this codec cannot express.
func TestWriteChannel_Erase(t *testing.T) {
	_, sess := openSession(t, Simulated, slotImage{})

	_, err := sess.WriteChannel(testCtx(t), codeplug.Channel{Slot: "010", Data: nil})
	if err == nil {
		t.Fatal("WriteChannel of an empty channel = nil, want a refusal: this radio has no erase command")
	}
}

// TestWriteChannel_TagRefused: a non-empty tag is refused rather than
// silently dropped — this radio has no tag route at all.
func TestWriteChannel_TagRefused(t *testing.T) {
	_, sess := openSession(t, Simulated, slotImage{})

	data := populatedChannel(14_250_000)
	data.Tag = "GB3TEST"
	_, err := sess.WriteChannel(testCtx(t), codeplug.Channel{Slot: "010", Data: data})
	if err == nil {
		t.Fatal("WriteChannel with a non-empty Tag = nil, want a refusal")
	}
}

// TestWriteChannel_CTCSSToneMustBeKnown: this radio's P9 is always on the
// wire, so a write with no Known tone is refused rather than silently
// writing a fabricated index.
func TestWriteChannel_CTCSSToneMustBeKnown(t *testing.T) {
	_, sess := openSession(t, Simulated, slotImage{})

	data := populatedChannel(14_250_000)
	data.CTCSSTone = codeplug.ToneField{State: codeplug.Unknown}
	_, err := sess.WriteChannel(testCtx(t), codeplug.Channel{Slot: "010", Data: data})
	if err == nil {
		t.Fatal("WriteChannel with CTCSSTone Unknown = nil, want a refusal naming FieldCTCSSTone")
	}
}

// TestBuildWriteCommand_ToneRoundTrips is a pure-function check: every
// standard chart tone builds an MW frame carrying its own two-digit index.
func TestBuildWriteCommand_ToneRoundTrips(t *testing.T) {
	tones := spec.StandardCTCSSTones()
	for i, tone := range tones {
		data := populatedChannel(14_250_000)
		data.CTCSSTone = codeplug.ToneField{State: codeplug.Known, Value: tone}
		cmd, err := buildWriteCommand(dialect, codeplug.Channel{Slot: "010", Data: data})
		if err != nil {
			t.Fatalf("buildWriteCommand tone index %d = %v, want nil", i, err)
		}
		frame := string(cmd.Bytes())
		want := fmt.Sprintf("%02d", i)
		if got := frame[23:25]; got != want {
			t.Errorf("index %d: P9 = %q, want %q", i, got, want)
		}
	}
}
