// SPDX-License-Identifier: GPL-3.0-or-later

package ft950

import (
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

func populatedChannel(slot string) codeplug.Channel {
	return codeplug.Channel{
		Slot: slot,
		Data: &codeplug.ChannelData{
			FreqHz:    14250000,
			Mode:      "USB",
			CTCSS:     "OFF",
			CTCSSTone: codeplug.ToneField{State: codeplug.Known, Value: spec.StandardCTCSSTones()[0]},
			Shift:     "SIMPLEX",
		},
	}
}

func TestWriteChannel_Success(t *testing.T) {
	_, sess := openSession(t, Simulated, slotImage{})

	res, err := sess.WriteChannel(testCtx(t), populatedChannel("000"))
	if err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	if len(res.Steps) != 1 || res.Steps[0].Command != "MW" || !res.Steps[0].Sent || !res.Steps[0].Confirmed {
		t.Errorf("WriteResult = %+v, want one confirmed MW step", res)
	}
}

func TestWriteChannel_RefusedOnUnverifiedProfile(t *testing.T) {
	_, sess := openSession(t, RealHardware, slotImage{})

	_, err := sess.WriteChannel(testCtx(t), populatedChannel("000"))
	var refused *driver.WriteRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("WriteChannel on the all-Unverified profile = %v, want *driver.WriteRefusedError", err)
	}
}

func TestWriteChannel_RefusedEmptyChannel(t *testing.T) {
	_, sess := openSession(t, Simulated, slotImage{})

	_, err := sess.WriteChannel(testCtx(t), codeplug.Channel{Slot: "000"})
	var refused *driver.WriteRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("WriteChannel of an empty channel = %v, want *driver.WriteRefusedError", err)
	}
	if len(refused.Fields) != 1 || refused.Fields[0] != spec.FieldErase {
		t.Errorf("refused.Fields = %v, want [FieldErase]", refused.Fields)
	}
}

// TestWriteChannel_NonKnownCTCSSToneDefaultsToIndexZero pins this driver's
// own choice (doc.go entry 1, the ft2000 shape, not the FTdx9000 one): a
// write that never mentions a tone is NOT refused, and rides the wire as
// tone index 0.
func TestWriteChannel_NonKnownCTCSSToneDefaultsToIndexZero(t *testing.T) {
	p, sess := openSession(t, Simulated, slotImage{})

	ch := populatedChannel("000")
	ch.Data.CTCSSTone = codeplug.ToneField{}

	res, err := sess.WriteChannel(testCtx(t), ch)
	if err != nil {
		t.Fatalf("WriteChannel with a non-Known CTCSSTone = %v, want success (defaults to tone index 0)", err)
	}
	if len(res.Steps) != 1 || !res.Steps[0].Confirmed {
		t.Errorf("WriteResult = %+v, want one confirmed MW step", res)
	}
	got := p.Transcript()
	if len(got) == 0 {
		t.Fatal("no frames recorded")
	}
	mw := got[len(got)-1]
	if len(mw) < 26 || mw[23:25] != "00" {
		t.Errorf("MW frame = %q, want P9 (positions 24-25) to read \"00\"", mw)
	}
}

func TestWriteChannel_RejectedByRadio(t *testing.T) {
	_, sess := openSession(t, Simulated, slotImage{rejectWrites: true})

	_, err := sess.WriteChannel(testCtx(t), populatedChannel("000"))
	if err == nil {
		t.Fatal("WriteChannel succeeded against a peer that rejects every MW, want an error")
	}
}
