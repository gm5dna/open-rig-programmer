// SPDX-License-Identifier: GPL-3.0-or-later

package ft2000

import (
	"errors"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

func populatedChannel(slot string) codeplug.Channel {
	return codeplug.Channel{
		Slot: slot,
		Data: &codeplug.ChannelData{
			FreqHz: 14_250_000,
			Mode:   "USB",
			CTCSS:  "OFF",
			Shift:  "SIMPLEX",
		},
	}
}

func TestWriteChannel_SimulatedAccepted(t *testing.T) {
	p, s := openSession(t, testModels[0], Simulated, slotImage{})

	res, err := s.WriteChannel(testCtx(t), populatedChannel("001"))
	if err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	if len(res.Steps) != 1 || res.Steps[0].Command != "MW" || !res.Steps[0].Sent || !res.Steps[0].Confirmed {
		t.Errorf("Steps = %+v, want exactly one confirmed MW step — no MT to sequence beside it (matrix §0)", res.Steps)
	}
	transcript := p.Transcript()
	if len(transcript) != 3 { // AI0;, ID;, MW...
		t.Fatalf("transcript = %v, want 3 frames (AI0;, ID;, MW Set)", transcript)
	}
	if !strings.HasPrefix(transcript[2], "MW001") {
		t.Errorf("last frame = %q, want an MW Set for slot 001", transcript[2])
	}
}

func TestWriteChannel_RealHardwareRefusedBeforeAnyWireTraffic(t *testing.T) {
	p, s := openSession(t, testModels[0], RealHardware, slotImage{})

	_, err := s.WriteChannel(testCtx(t), populatedChannel("001"))
	if err == nil {
		t.Fatal("WriteChannel succeeded under RealHardware, want a refusal — writeTrialsComplete is false")
	}
	var refused *driver.WriteRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("error = %v, want *driver.WriteRefusedError", err)
	}
	transcript := p.Transcript()
	if len(transcript) != 2 { // AI0;, ID; only — the refusal precedes any MW
		t.Errorf("transcript = %v, want exactly [AI0;, ID;] — no wire traffic before a refusal", transcript)
	}
}

func TestWriteChannel_EmptyChannelIsRefusedAsErase(t *testing.T) {
	_, s := openSession(t, testModels[0], Simulated, slotImage{})

	_, err := s.WriteChannel(testCtx(t), codeplug.Channel{Slot: "001"})
	if err == nil {
		t.Fatal("WriteChannel succeeded for an empty channel, want an erase refusal")
	}
	var refused *driver.WriteRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("error = %v, want *driver.WriteRefusedError", err)
	}
	if len(refused.Fields) != 1 || refused.Fields[0] != spec.FieldErase {
		t.Errorf("Fields = %v, want [FieldErase]", refused.Fields)
	}
}

func TestWriteChannel_RejectedByRadio(t *testing.T) {
	_, s := openSession(t, testModels[0], Simulated, slotImage{rejectSets: true})

	res, err := s.WriteChannel(testCtx(t), populatedChannel("001"))
	if err == nil {
		t.Fatal("WriteChannel succeeded despite the scripted radio rejecting every Set, want an error")
	}
	if len(res.Steps) != 1 || !res.Steps[0].Sent || res.Steps[0].Confirmed {
		t.Errorf("Steps = %+v, want Sent true / Confirmed false (the frame went out and was rejected)", res.Steps)
	}
}

func TestWriteChannel_KnownTagIsNeverRequested(t *testing.T) {
	// A caller-supplied Tag is not among requestedFields at all (NoTag —
	// this family has no wire route for it), so it must never itself
	// cause a refusal: only the five mapped fields (and CTCSSTone) are
	// judged.
	_, s := openSession(t, testModels[0], Simulated, slotImage{})
	ch := populatedChannel("001")
	ch.Data.Tag = "IGNOREDONWRITE"

	if _, err := s.WriteChannel(testCtx(t), ch); err != nil {
		t.Fatalf("WriteChannel with a non-empty Tag: %v — Tag must be silently ignored, not refused, on this NoTag radio", err)
	}
}

func TestWriteChannel_KnownCTCSSToneWritesTheIndex(t *testing.T) {
	p, s := openSession(t, testModels[0], Simulated, slotImage{})
	ch := populatedChannel("001")
	caps := s.Capabilities()
	ch.Data.CTCSSTone = codeplug.ToneField{State: codeplug.Known, Value: caps.CTCSSTones[26]} // 159.8 Hz

	if _, err := s.WriteChannel(testCtx(t), ch); err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	transcript := p.Transcript()
	last := transcript[len(transcript)-1]
	// P9 sits at this dialect's offset 23-24 within the frame (matrix
	// §1.1's shifted positions); the frame includes the two-byte "MW"
	// prefix, so within the string that is index 23:25.
	if got := last[23:25]; got != "26" {
		t.Errorf("P9 field = %q, want \"26\" (the tone index), full frame %q", got, last)
	}
}

func TestWriteChannel_UnknownToneNotAdmittedByRadio(t *testing.T) {
	_, s := openSession(t, testModels[0], Simulated, slotImage{})
	ch := populatedChannel("001")
	ch.Data.CTCSSTone = codeplug.ToneField{State: codeplug.Known, Value: 12345} // not on the 50-entry chart

	if _, err := s.WriteChannel(testCtx(t), ch); err == nil {
		t.Fatal("WriteChannel succeeded with a tone outside the 50-entry chart, want a refusal")
	}
}
