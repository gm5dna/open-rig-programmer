// SPDX-License-Identifier: GPL-3.0-or-later

package ft890900

import (
	"context"
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/bincat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
)

// wantFrame renders the expected wire bytes for one opcode/args pair,
// matching bincat.BuildFrame's own argument-then-opcode order.
func wantFrame(opcode byte, args [4]byte) [5]byte {
	return [5]byte{args[0], args[1], args[2], args[3], opcode}
}

func openFakeSimulatedFT890(t *testing.T, p *scriptedPort) driver.Session {
	t.Helper()
	p.setAnswer(statusUpdateFrame(probeChannel1), blankRecord())
	sess, err := NewFT890(Simulated).Open(context.Background(), p.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	p.ResetTranscript() // isolate the caller's own frames from Open's two identity probes
	return sess
}

func TestWriteChannel_SimplexChannel_SendsSevenFramesInOrder(t *testing.T) {
	p := newScriptedPort(t)
	sess := openFakeSimulatedFT890(t, p)

	ch := codeplug.Channel{Slot: "001", Data: &codeplug.ChannelData{
		FreqHz:    14_250_000,
		Mode:      "USB",
		Shift:     "SIMPLEX",
		CTCSSTone: codeplug.ToneField{State: codeplug.Known, Value: standardTones[5]},
		OffsetHz:  codeplug.FreqField{State: codeplug.Unavailable},
	}}

	res, err := sess.WriteChannel(context.Background(), ch)
	if err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	wantSteps := []string{"A/B", "SetFreq", "SetMode", "Clar", "Shift", "Tone", "Store"}
	if len(res.Steps) != len(wantSteps) {
		t.Fatalf("Steps = %v, want %d steps %v", res.Steps, len(wantSteps), wantSteps)
	}
	for i, s := range res.Steps {
		if s.Command != wantSteps[i] || !s.Sent || !s.Confirmed {
			t.Errorf("Steps[%d] = %+v, want {%q true true}", i, s, wantSteps[i])
		}
	}

	bcdFreq, _ := bincat.EncodeBCD(1_425_000, 4)
	want := []([5]byte){
		wantFrame(bincat.OpABSelect, [4]byte{0, 0, 0, 0}),
		wantFrame(bincat.OpSetFreq, [4]byte{bcdFreq[0], bcdFreq[1], bcdFreq[2], bcdFreq[3]}),
		wantFrame(bincat.OpSetMode, [4]byte{bincat.ModeUSB, 0, 0, 0}),
		wantFrame(bincat.OpClarifier, clarifierOffArgs),
		wantFrame(bincat.OpShift, [4]byte{0, 0, 0, 0}), // SIMPLEX
		wantFrame(bincat.OpTone, [4]byte{5, 0, 0, 0}),
		wantFrame(bincat.OpStore, [4]byte{1, 0, 0, 0}),
	}
	got := p.Transcript()
	if len(got) != len(want) {
		t.Fatalf("frames sent = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("frame %d = % x, want % x", i, got[i], want[i])
		}
	}
}

func TestWriteChannel_NonSimplexChannel_SendsOffsetFrame(t *testing.T) {
	p := newScriptedPort(t)
	sess := openFakeSimulatedFT890(t, p)

	ch := codeplug.Channel{Slot: "002", Data: &codeplug.ChannelData{
		FreqHz:   14_300_000,
		Mode:     "FM",
		Shift:    "MINUS",
		OffsetHz: codeplug.FreqField{State: codeplug.Known, Value: 600},
	}}
	res, err := sess.WriteChannel(context.Background(), ch)
	if err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	wantSteps := []string{"A/B", "SetFreq", "SetMode", "Clar", "Shift", "Offset", "Tone", "Store"}
	if len(res.Steps) != len(wantSteps) {
		t.Fatalf("Steps = %v, want 8 steps %v", res.Steps, wantSteps)
	}
	for i, s := range res.Steps {
		if s.Command != wantSteps[i] {
			t.Errorf("Steps[%d].Command = %q, want %q", i, s.Command, wantSteps[i])
		}
	}
	got := p.Transcript()
	wantOffsetFrame := wantFrame(bincat.OpOffset, [4]byte{0, 0x00, 0x06, 0x00}) // EncodeBCD(600,3)
	wantShiftFrame := wantFrame(bincat.OpShift, [4]byte{1, 0, 0, 0})            // MINUS
	if got[4] != wantShiftFrame {
		t.Errorf("Shift frame = % x, want % x", got[4], wantShiftFrame)
	}
	if got[5] != wantOffsetFrame {
		t.Errorf("Offset frame = % x, want % x", got[5], wantOffsetFrame)
	}
}

func TestWriteChannel_RefusesErase(t *testing.T) {
	p := newScriptedPort(t)
	sess := openFakeSimulatedFT890(t, p)

	_, err := sess.WriteChannel(context.Background(), codeplug.Channel{Slot: "001", Data: nil})
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel(erase) error = %v, want errors.Is(_, driver.ErrWriteRefused)", err)
	}
}

func TestWriteChannel_RefusesARequestedClarifier(t *testing.T) {
	p := newScriptedPort(t)
	sess := openFakeSimulatedFT890(t, p)

	ch := codeplug.Channel{Slot: "001", Data: &codeplug.ChannelData{
		FreqHz: 14_250_000, Mode: "USB", Shift: "SIMPLEX", ClarHz: 200,
	}}
	_, err := sess.WriteChannel(context.Background(), ch)
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel(clarifier requested) error = %v, want errors.Is(_, driver.ErrWriteRefused): opcode 09H's byte layout is undocumented", err)
	}
}

func TestWriteChannel_RefusesNonSimplexWithoutAKnownOffset(t *testing.T) {
	p := newScriptedPort(t)
	sess := openFakeSimulatedFT890(t, p)

	ch := codeplug.Channel{Slot: "001", Data: &codeplug.ChannelData{
		FreqHz: 14_250_000, Mode: "USB", Shift: "PLUS",
	}}
	_, err := sess.WriteChannel(context.Background(), ch)
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel(non-simplex, no offset) error = %v, want errors.Is(_, driver.ErrWriteRefused)", err)
	}
}

func TestWriteChannel_RealHardwareWithoutConsent_Refused(t *testing.T) {
	p := newScriptedPort(t)
	sess := openFakeFT890(t, p) // RealHardware, no consent

	ch := codeplug.Channel{Slot: "001", Data: &codeplug.ChannelData{
		FreqHz: 14_250_000, Mode: "USB", Shift: "SIMPLEX",
	}}
	_, err := sess.WriteChannel(context.Background(), ch)
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel on an unconsented RealHardware session error = %v, want errors.Is(_, driver.ErrWriteRefused)", err)
	}
}
