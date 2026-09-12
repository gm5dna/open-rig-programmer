// SPDX-License-Identifier: GPL-3.0-or-later

package ts2000

import (
	"context"
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
)

// populatedChannel is a channel every requestedFieldRules entry this row
// grades can be asked to write.
func populatedChannel(slot string) codeplug.Channel {
	return codeplug.Channel{
		Slot: slot,
		Data: &codeplug.ChannelData{
			FreqHz: 145500000, Mode: "FM",
			Tag:      "REPEATER",
			ScanSkip: codeplug.BoolField{State: codeplug.Known, Value: false},
			Duplex:   codeplug.StringField{State: codeplug.Known, Value: "1"},
			OffsetHz: codeplug.FreqField{State: codeplug.Known, Value: 600000},
			ToneMode: codeplug.StringField{State: codeplug.Known, Value: "CTCSS"},
			ToneTx:   codeplug.ToneField{State: codeplug.Known, Value: 915},
			ToneRx:   codeplug.ToneField{State: codeplug.Known, Value: 915},
		},
	}
}

// TestWriteChannel_AlwaysRefused pins that no profile builds an MW frame:
// three raw bytes (P11, P14, P15) have no honest value this milestone can
// supply, on RealHardware (unconsented, refused by the capability gate)
// and on Simulated (which passes that gate and meets the standing
// registerP14 refusal — the control that the pin is not vacuous).
func TestWriteChannel_AlwaysRefused(t *testing.T) {
	for _, tc := range []struct {
		name  string
		opts  []Option
		wantP RefusalOrGateKind
	}{
		{"RealHardware unconsented", nil, gateRefusal},
		{"RealHardware consented", []Option{WithConsentedUnverifiedWrites()}, registerRefusal},
		{"Simulated", []Option{WithSimulatedProfile()}, registerRefusal},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sess, p := openSession(t, NewTS2000, "019", radioImage{}, tc.opts...)
			res, err := sess.WriteChannel(context.Background(), populatedChannel("072"))
			if err == nil {
				t.Fatal("WriteChannel succeeded; every channel write on this row must be refused")
			}
			if res.Steps == nil || len(res.Steps) != 0 {
				t.Errorf("WriteResult.Steps = %v, want an explicitly empty (non-nil) slice", res.Steps)
			}
			if !errors.Is(err, driver.ErrWriteRefused) {
				t.Errorf("err = %v, want errors.Is(_, driver.ErrWriteRefused)", err)
			}
			var re *RefusalError
			gotRegister := errors.As(err, &re)
			if tc.wantP == registerRefusal && !gotRegister {
				t.Errorf("err = %v, want a *RefusalError (the standing registerP14 refusal, not the capability gate)", err)
			}
			if tc.wantP == gateRefusal && gotRegister {
				t.Errorf("err = %v, want the bare capability-gate refusal, not *RefusalError — an unconsented RealHardware session must be refused BEFORE registerP14 is ever reached", err)
			}
			for _, f := range p.Transcript() {
				if f != "AI0;" && f != "ID;" && f != "TY;" {
					t.Errorf("a frame was sent for a write: %q — no MW of any kind may ever leave this package", f)
				}
			}
		})
	}
}

// RefusalOrGateKind distinguishes the two refusal shapes WriteChannel can
// produce, so a test can assert WHICH rung answered rather than merely
// that one did (core/driver/ts480's own P7 H2 reasoning).
type RefusalOrGateKind int

const (
	gateRefusal RefusalOrGateKind = iota
	registerRefusal
)

// TestWriteChannel_EmptyChannelIsRefusedAsErase pins the erase rung, ahead
// of the field checks that would otherwise dereference a nil Data.
func TestWriteChannel_EmptyChannelIsRefusedAsErase(t *testing.T) {
	sess, _ := openSession(t, NewTS2000, "019", radioImage{}, WithSimulatedProfile())
	_, err := sess.WriteChannel(context.Background(), codeplug.Channel{Slot: "072"})
	var wre *driver.WriteRefusedError
	if !errors.As(err, &wre) {
		t.Fatalf("WriteChannel(empty): %v, want *driver.WriteRefusedError", err)
	}
	if len(wre.Fields) != 1 || wre.Fields[0] != "erase" {
		t.Errorf("WriteRefusedError.Fields = %v, want [erase]", wre.Fields)
	}
}

// TestWriteChannel_UnknownSlotIsRefusedBeforeAnyFieldCheck pins slot
// membership as the first rung, ahead of the empty-channel check.
func TestWriteChannel_UnknownSlotIsRefusedBeforeAnyFieldCheck(t *testing.T) {
	sess, _ := openSession(t, NewTS2000, "019", radioImage{}, WithSimulatedProfile())
	_, err := sess.WriteChannel(context.Background(), populatedChannel("999"))
	var use *UnknownSlotError
	if !errors.As(err, &use) {
		t.Fatalf("WriteChannel(999): %v, want *UnknownSlotError", err)
	}
}
