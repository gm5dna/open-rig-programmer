// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx9000

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

	res, err := sess.WriteChannel(testCtx(t), populatedChannel("001"))
	if err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	if len(res.Steps) != 1 || res.Steps[0].Command != "MW" || !res.Steps[0].Sent || !res.Steps[0].Confirmed {
		t.Errorf("WriteResult = %+v, want one confirmed MW step", res)
	}
}

func TestWriteChannel_RefusedOnUnverifiedProfile(t *testing.T) {
	_, sess := openSession(t, RealHardware, slotImage{})

	_, err := sess.WriteChannel(testCtx(t), populatedChannel("001"))
	var refused *driver.WriteRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("WriteChannel on the all-Unverified profile = %v, want *driver.WriteRefusedError", err)
	}
}

func TestWriteChannel_RefusedEmptyChannel(t *testing.T) {
	_, sess := openSession(t, Simulated, slotImage{})

	_, err := sess.WriteChannel(testCtx(t), codeplug.Channel{Slot: "001"})
	var refused *driver.WriteRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("WriteChannel of an empty channel = %v, want *driver.WriteRefusedError", err)
	}
	if len(refused.Fields) != 1 || refused.Fields[0] != spec.FieldErase {
		t.Errorf("refused.Fields = %v, want [FieldErase]", refused.Fields)
	}
}

func TestWriteChannel_RefusedNonKnownCTCSSTone(t *testing.T) {
	_, sess := openSession(t, Simulated, slotImage{})

	ch := populatedChannel("001")
	ch.Data.CTCSSTone = codeplug.ToneField{}

	_, err := sess.WriteChannel(testCtx(t), ch)
	var refused *driver.WriteRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("WriteChannel with a non-Known CTCSSTone = %v, want *driver.WriteRefusedError", err)
	}
	if len(refused.Fields) != 1 || refused.Fields[0] != spec.FieldCTCSSTone {
		t.Errorf("refused.Fields = %v, want [FieldCTCSSTone]", refused.Fields)
	}
}

func TestWriteChannel_RejectedByRadio(t *testing.T) {
	_, sess := openSession(t, Simulated, slotImage{rejectWrites: true})

	_, err := sess.WriteChannel(testCtx(t), populatedChannel("001"))
	if err == nil {
		t.Fatal("WriteChannel succeeded against a peer that rejects every MW, want an error")
	}
}
