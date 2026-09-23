// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx5000

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
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

// TestWriteChannel_TagDisplayKnownRefused: a Known TagDisplay is requested
// (RequestConditionalTagFields) — unlike an Unavailable/Unknown one, which
// TestWriteChannel_Accepted proves is never requested at all — and this
// radio has no display flag to write, so the capability walk refuses it.
func TestWriteChannel_TagDisplayKnownRefused(t *testing.T) {
	_, sess := openSession(t, Simulated, slotImage{})

	data := populatedChannel(14_250_000)
	data.TagDisplay = codeplug.BoolField{State: codeplug.Known, Value: true}
	_, err := sess.WriteChannel(testCtx(t), codeplug.Channel{Slot: "010", Data: data})
	if err == nil {
		t.Fatal("WriteChannel with TagDisplay Known = nil, want a refusal — this radio's record has no display flag")
	}
}

// TestWriteChannel_ScanSkipKnownRefused: a Known ScanSkip is requested
// (RequestConditionalTagFields) — unlike an Unavailable one, which
// TestWriteChannel_Accepted proves is never requested at all — and this
// radio has no scan-skip byte to write, so the capability walk refuses it.
func TestWriteChannel_ScanSkipKnownRefused(t *testing.T) {
	_, sess := openSession(t, Simulated, slotImage{})

	data := populatedChannel(14_250_000)
	data.ScanSkip = codeplug.BoolField{State: codeplug.Known, Value: true}
	_, err := sess.WriteChannel(testCtx(t), codeplug.Channel{Slot: "010", Data: data})
	if err == nil {
		t.Fatal("WriteChannel with ScanSkip Known = nil, want a refusal — this radio's record has no scan-skip byte")
	}
}

// TestBuildMWCommand_TagRefused pins ExplicitTagRefusal's own wording:
// buildMWCommand refuses a non-empty Tag itself, ahead of the value-level
// checks that follow it, before the caps-driven "not write-Supported"
// refusal (which fires first via WriteChannel today, since FieldTag is the
// zero FieldSupport) ever gets a chance to. Called directly via the Session
// method, bypassing WriteChannel's capability walk, the same way
// TestBuildMWCommand_ToneRoundTrips already does. Renamed from
// TestBuildWriteCommand_TagRefused: the pre-migration package-level
// buildWriteCommand(dialect, ch) became the Session method
// s.buildMWCommand(ch) the shared core migration uses, the same shape
// every other migrated sibling's equivalent test already keeps
// (TestBuildMWCommand_UnknownModeRefuses, ftdx1200/ftdx3000). The
// assertions are unchanged from the pin commit.
func TestBuildMWCommand_TagRefused(t *testing.T) {
	_, s := openSession(t, Simulated, slotImage{})
	data := populatedChannel(14_250_000)
	data.Tag = "GB3TEST"
	_, err := s.buildMWCommand(codeplug.Channel{Slot: "010", Data: data})

	var refused *driver.WriteRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("buildMWCommand with a non-empty Tag = %v, want *driver.WriteRefusedError", err)
	}
	if len(refused.Fields) != 1 || refused.Fields[0] != spec.FieldTag {
		t.Errorf("refused.Fields = %v, want [FieldTag]", refused.Fields)
	}
	const want = "this radio's CAT command set has no channel-name/tag route at all (matrix §2/§3): a non-empty tag cannot be written"
	if refused.Reason != want {
		t.Errorf("refused.Reason = %q, want %q", refused.Reason, want)
	}
}

// TestBuildMWCommand_TagDisplayKnownRefused pins ExplicitTagRefusal's
// TagDisplay wording, the same way TestBuildMWCommand_TagRefused pins its
// Tag wording. Renamed from TestBuildWriteCommand_TagDisplayKnownRefused
// (see that renaming's own comment above); assertions unchanged.
func TestBuildMWCommand_TagDisplayKnownRefused(t *testing.T) {
	_, s := openSession(t, Simulated, slotImage{})
	data := populatedChannel(14_250_000)
	data.TagDisplay = codeplug.BoolField{State: codeplug.Known, Value: true}
	_, err := s.buildMWCommand(codeplug.Channel{Slot: "010", Data: data})

	var refused *driver.WriteRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("buildMWCommand with TagDisplay Known = %v, want *driver.WriteRefusedError", err)
	}
	if len(refused.Fields) != 1 || refused.Fields[0] != spec.FieldTagDisplay {
		t.Errorf("refused.Fields = %v, want [FieldTagDisplay]", refused.Fields)
	}
	const want = "this radio's 27-byte record has no display flag (matrix §2): a Known tag_display cannot be written"
	if refused.Reason != want {
		t.Errorf("refused.Reason = %q, want %q", refused.Reason, want)
	}
}

// TestWriteChannel_KindByteHardcodedVFO pins Q2: the MW Set frame's P7 kind
// byte is always cat.KindVFO. Offset 21 is this dialect's own memKindOff
// (P1 3 digits + P2 8 digits + P3 sign/4-digit mag + P4 + P5 + P6, matching
// the P9 offset TestBuildMWCommand_ToneRoundTrips already addresses at
// frame[23:25], two bytes further on past P8).
func TestWriteChannel_KindByteHardcodedVFO(t *testing.T) {
	p, sess := openSession(t, Simulated, slotImage{})

	_, err := sess.WriteChannel(testCtx(t), codeplug.Channel{Slot: "010", Data: populatedChannel(14_250_000)})
	if err != nil {
		t.Fatalf("WriteChannel = %v, want nil", err)
	}

	transcript := p.Transcript()
	mw := transcript[len(transcript)-1]
	if got, want := mw[21], byte(cat.KindVFO); got != want {
		t.Errorf("MW frame P7 (Kind) byte = %q, want %q (cat.KindVFO)", got, want)
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

// TestBuildMWCommand_ToneRoundTrips is a pure-function check: every
// standard chart tone builds an MW frame carrying its own two-digit index.
// Renamed from TestBuildWriteCommand_ToneRoundTrips (see
// TestBuildMWCommand_TagRefused's comment); assertions unchanged.
func TestBuildMWCommand_ToneRoundTrips(t *testing.T) {
	_, s := openSession(t, Simulated, slotImage{})
	tones := spec.StandardCTCSSTones()
	for i, tone := range tones {
		data := populatedChannel(14_250_000)
		data.CTCSSTone = codeplug.ToneField{State: codeplug.Known, Value: tone}
		cmd, err := s.buildMWCommand(codeplug.Channel{Slot: "010", Data: data})
		if err != nil {
			t.Fatalf("buildMWCommand tone index %d = %v, want nil", i, err)
		}
		frame := string(cmd.Bytes())
		want := fmt.Sprintf("%02d", i)
		if got := frame[23:25]; got != want {
			t.Errorf("index %d: P9 = %q, want %q", i, got, want)
		}
	}
}
