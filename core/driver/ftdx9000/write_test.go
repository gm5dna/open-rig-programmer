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

// TestWriteChannel_IcomTierFieldKnown pins today's write-side blind spot
// (spec-v2 finding 9, adjudication BLOCKER 2/Q8): requestedFields is a
// fixed 6-field list that never asks after any of the 17 Icom-tier
// fields, so a channel that sets one (e.g. FieldDataMode) Known is
// ACCEPTED — the value is silently dropped from the MW frame, not
// refused, unlike every migrated sibling driver's TierRequestedFields
// loop. The MR core migration (plan Step 4) must reproduce this exactly
// via MRParams.SkipTierFields; this test is the backstop that would
// catch SkipTierFields wired wrong (see step4b.md for the
// mutation-sensitivity check confirming that).
//
// FieldDuplex — the field the retry brief names as the example Icom-tier
// field — does NOT serve this purpose: driver.CheckFieldStates judges
// every Known StringField against this radio's own vocabulary
// (caps.DuplexOptions), which is nil/empty for every in-scope Yaesu
// radio, so a Known Duplex is refused there, before requestedFields is
// ever consulted, on both the unmigrated driver and any migrated one —
// the test would pass unchanged whichever way SkipTierFields were wired.
// FieldDataMode is a BoolField, whose Valid() has no vocabulary to fail
// closed against (codeplug/fieldstate.go's own documented distinction),
// so it reaches requestedFields and is the one this test uses instead.
func TestWriteChannel_IcomTierFieldKnown(t *testing.T) {
	basePort, baseSess := openSession(t, Simulated, slotImage{})
	if _, err := baseSess.WriteChannel(testCtx(t), populatedChannel("001")); err != nil {
		t.Fatalf("baseline WriteChannel: %v", err)
	}
	baseFrames := basePort.Transcript()
	baseMW := baseFrames[len(baseFrames)-1]

	tieredPort, tieredSess := openSession(t, Simulated, slotImage{})
	tiered := populatedChannel("001")
	tiered.Data.DataMode = codeplug.BoolField{State: codeplug.Known, Value: true}

	res, err := tieredSess.WriteChannel(testCtx(t), tiered)
	if err != nil {
		t.Fatalf("WriteChannel with a Known Icom-tier field (DataMode) = %v, want acceptance: this radio's 27-byte record has no room for it, and it is not one of requestedFields' fixed six", err)
	}
	if len(res.Steps) != 1 || res.Steps[0].Command != "MW" || !res.Steps[0].Sent || !res.Steps[0].Confirmed {
		t.Errorf("WriteResult = %+v, want one confirmed MW step", res)
	}
	tieredFrames := tieredPort.Transcript()
	tieredMW := tieredFrames[len(tieredFrames)-1]
	if tieredMW != baseMW {
		t.Errorf("MW frame with DataMode Known = %q, want identical to the DataMode-absent baseline %q — a tier field this record cannot carry must be silently dropped, not encoded", tieredMW, baseMW)
	}
}

// TestWriteChannel_FixedSixFieldStillRefused is
// TestWriteChannel_IcomTierFieldKnown's control: an invalid value in the
// fixed six (Mode) is still refused, proving the Icom-tier acceptance
// above is specific to a field requestedFields never asks after, not a
// hole that swallows every checked field.
func TestWriteChannel_FixedSixFieldStillRefused(t *testing.T) {
	_, sess := openSession(t, Simulated, slotImage{})
	ch := populatedChannel("001")
	ch.Data.Mode = "BOGUS"

	_, err := sess.WriteChannel(testCtx(t), ch)
	var refused *driver.WriteRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("WriteChannel with an unrecognised Mode = %v, want *driver.WriteRefusedError", err)
	}
	if len(refused.Fields) != 1 || refused.Fields[0] != spec.FieldMode {
		t.Errorf("refused.Fields = %v, want [FieldMode]", refused.Fields)
	}
}
