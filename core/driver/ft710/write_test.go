// SPDX-License-Identifier: GPL-3.0-or-later

package ft710

import (
	"errors"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/internal/fakeradio"
)

// writableChannel returns a fully-writable populated channel for slot:
// every FieldState-carrying field Unknown (nothing inexpressible
// requested), everything else set to plain, codec-expressible values.
func writableChannel(slot string) codeplug.Channel {
	return codeplug.Channel{
		Slot: slot,
		Data: &codeplug.ChannelData{
			FreqHz:     14_250_000,
			Mode:       "USB",
			ClarHz:     0,
			CTCSS:      "OFF",
			CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
			Shift:      "SIMPLEX",
			Tag:        "TEST",
			TagDisplay: true,
			ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
		},
	}
}

// TestWriteChannel_HappyPath (Simulated profile): MW then MT are sent and
// unrejected, and the fake's stored slot state shows the exact bytes the
// wire carried — memory slot AND PMS slot both with Kind '1'. HW-CONFIRMED
// 2026-07-13 (M5b write trials, docs/hardware-notes.md): this test
// formerly expected Kind '5' for a PMS write (the ASSUMED pairing) —
// hardware-refuted (a live PMS write carrying Kind '5' was REJECTED; see
// TestReadChannel_HWDerived_M5b_PMSKindLeniency and
// internal/fakeradio's TestHWDerived_MW_KindMatrix_M5b).
func TestWriteChannel_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		slot     string
		wantKind byte
	}{
		{"memory slot -> Kind '1'", "010", '1'},
		{"PMS slot -> Kind '1' (HW-CONFIRMED, was wrongly '5')", "P2U", '1'},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			radio, sess := openSession(t, Simulated)

			res, err := sess.WriteChannel(testCtx(t), writableChannel(tt.slot))
			if err != nil {
				t.Fatalf("WriteChannel: unexpected error: %v", err)
			}
			want := driver.WriteResult{MWSent: true, MWConfirmed: true, MTSent: true, MTConfirmed: true}
			if res != want {
				t.Errorf("WriteResult = %+v, want %+v", res, want)
			}

			state, ok := radio.SlotState(tt.slot)
			if !ok {
				t.Fatalf("SlotState(%q): no state stored after WriteChannel", tt.slot)
			}
			if !state.Populated {
				t.Error("slot not marked Populated after MW")
			}
			if state.Freq != "014250000" {
				t.Errorf("stored Freq = %q, want \"014250000\"", state.Freq)
			}
			if state.Mode != '2' {
				t.Errorf("stored Mode = %q, want '2' (USB)", state.Mode)
			}
			if state.Kind != tt.wantKind {
				t.Errorf("stored Kind = %q, want %q (HW-CONFIRMED Kind-on-write pairing)", state.Kind, tt.wantKind)
			}
			if state.ClarSign != '+' || state.ClarMag != "0000" {
				t.Errorf("stored clarifier = %c%s, want +0000", state.ClarSign, state.ClarMag)
			}
			if state.CTCSS != '0' || state.Shift != '0' {
				t.Errorf("stored CTCSS/Shift = %q/%q, want '0'/'0'", state.CTCSS, state.Shift)
			}
			if state.Tag != "TEST" || !state.TagDisplay {
				t.Errorf("stored Tag/TagDisplay = %q/%v, want \"TEST\"/true", state.Tag, state.TagDisplay)
			}
		})
	}
}

// TestWriteChannel_InertClarifierTransmitted (M5b): the driver's
// defence-in-depth gate treats the clarifier's spec.Inert Write support
// as acceptable-to-TRANSMIT — a channel carrying a non-zero clarifier is
// NOT refused here, whatever its value: the radio provably ignores the
// field (HW-CONFIRMED 2026-07-13), and this layer lacks the baseline
// needed to tell a changed value from an unchanged one — that half of
// the Inert rule lives in codeplug.Diff (see spec.Inert). fakeradio
// mirrors the radio: the transmitted clarifier is accepted and stored as
// zeros.
func TestWriteChannel_InertClarifierTransmitted(t *testing.T) {
	radio, sess := openSession(t, Simulated)

	ch := writableChannel("010")
	ch.Data.ClarHz = 100
	ch.Data.RxClar = true

	res, err := sess.WriteChannel(testCtx(t), ch)
	if err != nil {
		t.Fatalf("WriteChannel (non-zero Inert clarifier): unexpected error: %v", err)
	}
	want := driver.WriteResult{MWSent: true, MWConfirmed: true, MTSent: true, MTConfirmed: true}
	if res != want {
		t.Errorf("WriteResult = %+v, want %+v", res, want)
	}

	state, ok := radio.SlotState("010")
	if !ok || !state.Populated {
		t.Fatalf("SlotState(\"010\") = %+v, ok=%v, want populated after the write", state, ok)
	}
	if state.ClarSign != '+' || state.ClarMag != "0000" || state.RXClar || state.TXClar {
		t.Errorf("stored clarifier = %c%s rx=%v tx=%v, want +0000 false false (radio ignores the transmitted clarifier)", state.ClarSign, state.ClarMag, state.RXClar, state.TXClar)
	}
}

// TestWriteChannel_DelayedRejection: the MW frame draws a "?;" that
// arrives DELAYED but still inside the transport's error window
// (fakeradio FaultDelayedRejection). WriteChannel must report MWSent
// (the frame went out) but NOT MWConfirmed, must not send MT at all, and
// must surface the rejection as a typed error.
//
// Exchange arithmetic (minimalImage pins it): Open produces AI0;=1,
// ID;=2, MR501;=3 (rejected: no 60m), MREMG;=4 (rejected: no EMG) — so
// WriteChannel's MW frame is exchange 5.
func TestWriteChannel_DelayedRejection(t *testing.T) {
	_, sess := openSession(t, Simulated,
		fakeradio.WithFactoryImage(minimalImage),
		fakeradio.WithFault(fakeradio.FaultDelayedRejection(5, 50*time.Millisecond)),
	)

	res, err := sess.WriteChannel(testCtx(t), writableChannel("010"))
	if !errors.Is(err, cat.ErrRejected) {
		t.Fatalf("WriteChannel = %v, want errors.Is match against cat.ErrRejected", err)
	}
	want := driver.WriteResult{MWSent: true, MWConfirmed: false, MTSent: false, MTConfirmed: false}
	if res != want {
		t.Errorf("WriteResult = %+v, want %+v (MW sent but rejected; MT never attempted)", res, want)
	}
}

// TestWriteChannel_RefusedBeforeWire is the defence-in-depth test: every
// refusal below must happen BEFORE any wire traffic at all — zero Write
// calls reach the port — and carry driver.ErrWriteRefused.
func TestWriteChannel_RefusedBeforeWire(t *testing.T) {
	knownTone := writableChannel("010")
	knownTone.Data.CTCSSTone = codeplug.ToneField{State: codeplug.Known, Value: 670}

	knownSkip := writableChannel("010")
	knownSkip.Data.ScanSkip = codeplug.BoolField{State: codeplug.Known, Value: true}

	tests := []struct {
		name    string
		profile Profile
		ch      codeplug.Channel
	}{
		// The codec cannot express a tone or scan-skip write: a Known
		// value must be refused, never silently dropped.
		{"Known CTCSS tone (Simulated)", Simulated, knownTone},
		{"Known scan skip (Simulated)", Simulated, knownSkip},
		// The write gate still functions on the RealHardware profile
		// post-M5b-flip: a plain writable channel is now legitimately
		// accepted there (see TestWriteTrialsComplete_FlippedTrue_M5b),
		// so the honest safety assertion — replacing the old "any channel
		// on the Unverified profile is refused" pin — is that a channel
		// requesting something the verified profile does NOT support is
		// still refused before any wire traffic, exactly as on Simulated.
		{"Known CTCSS tone (RealHardware, post-flip)", RealHardware, knownTone},
		{"Known scan skip (RealHardware, post-flip)", RealHardware, knownSkip},
		// Erase (empty channel) is never expressible by this codec.
		{"erase via empty channel", Simulated, codeplug.Channel{Slot: "010"}},
		// Discovered banks are read-only.
		{"60m slot", Simulated, writableChannel("501")},
		// A slot outside every bank cannot be gated per-field, so it is
		// refused outright.
		{"slot in no bank", Simulated, writableChannel("000")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cp, sess := openCountingSession(t, tt.profile)

			baseline := cp.writes.Load()
			_, err := sess.WriteChannel(testCtx(t), tt.ch)
			if !errors.Is(err, driver.ErrWriteRefused) {
				t.Fatalf("WriteChannel = %v, want errors.Is match against driver.ErrWriteRefused", err)
			}
			if got := cp.writes.Load(); got != baseline {
				t.Errorf("refused WriteChannel produced %d wire writes, want 0 (refusal must precede ALL wire traffic)", got-baseline)
			}
		})
	}
}

// TestWriteChannel_RefusalNamesFields: the Known-tone refusal must name
// the offending field, so the clone service (and a human reading a log)
// can see exactly why.
func TestWriteChannel_RefusalNamesFields(t *testing.T) {
	_, sess := openSession(t, Simulated)

	ch := writableChannel("010")
	ch.Data.CTCSSTone = codeplug.ToneField{State: codeplug.Known, Value: 670}

	_, err := sess.WriteChannel(testCtx(t), ch)
	var wre *driver.WriteRefusedError
	if !errors.As(err, &wre) {
		t.Fatalf("WriteChannel = %v, want a *driver.WriteRefusedError", err)
	}
	found := false
	for _, f := range wre.Fields {
		if f == "ctcss_tone" {
			found = true
		}
	}
	if !found {
		t.Errorf("WriteRefusedError.Fields = %v, want ctcss_tone named", wre.Fields)
	}
	if wre.Slot != "010" {
		t.Errorf("WriteRefusedError.Slot = %q, want \"010\"", wre.Slot)
	}
}
