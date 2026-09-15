// SPDX-License-Identifier: GPL-3.0-or-later

package ft890900

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/internal/fakeft890"
	"github.com/gm5dna/open-rig-programmer/internal/fakeft900"
)

// This file is Phase 5's own driver-against-fake round-trip suite
// (plan.md §Phase 5, Codex #11 v2 change): internal/fakeft890 and
// internal/fakeft900 (Phase 4) were built quarantined from this
// package's own driver code, so pairing the two together — and the
// crossed wrong-radio checks the family's shared identity probe makes
// possible — is this phase's job, not Phase 3's or Phase 4's.

func slotName(n int) string { return fmt.Sprintf("%03d", n) }

func TestRoundTrip_FT890AgainstFakeFT890(t *testing.T) {
	radio := fakeft890.New()
	defer radio.Close()

	sess, err := NewFT890(Simulated).Open(context.Background(), radio.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer sess.Close()

	for n := 1; n <= ft890TrueSlotCount; n++ {
		if _, err := sess.ReadChannel(context.Background(), slotName(n)); err != nil {
			t.Fatalf("ReadChannel(%q): %v", slotName(n), err)
		}
	}

	want := codeplug.Channel{Slot: "010", Data: &codeplug.ChannelData{
		FreqHz:    7_123_000,
		Mode:      "LSB",
		Shift:     "SIMPLEX",
		CTCSSTone: codeplug.ToneField{State: codeplug.Known, Value: standardTones[3]},
		OffsetHz:  codeplug.FreqField{State: codeplug.Unavailable},
	}}
	if _, err := sess.WriteChannel(context.Background(), want); err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	got, err := sess.ReadChannel(context.Background(), "010")
	if err != nil {
		t.Fatalf("ReadChannel after write: %v", err)
	}
	if got.Data == nil {
		t.Fatal("ReadChannel after write returned a blank channel")
	}
	if got.Data.FreqHz != want.Data.FreqHz || got.Data.Mode != want.Data.Mode || got.Data.Shift != want.Data.Shift {
		t.Errorf("read-back = %+v, want FreqHz/Mode/Shift from %+v", got.Data, want.Data)
	}
	if got.Data.CTCSSTone != want.Data.CTCSSTone {
		t.Errorf("read-back CTCSSTone = %+v, want %+v", got.Data.CTCSSTone, want.Data.CTCSSTone)
	}
}

func TestRoundTrip_FT900AgainstFakeFT900(t *testing.T) {
	radio := fakeft900.New()
	defer radio.Close()

	sess, err := NewFT900(Simulated).Open(context.Background(), radio.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer sess.Close()

	for n := 1; n <= ft900TrueSlotCount; n++ {
		if _, err := sess.ReadChannel(context.Background(), slotName(n)); err != nil {
			t.Fatalf("ReadChannel(%q): %v", slotName(n), err)
		}
	}

	want := codeplug.Channel{Slot: "050", Data: &codeplug.ChannelData{
		FreqHz:   14_300_000,
		Mode:     "FM",
		Shift:    "MINUS",
		OffsetHz: codeplug.FreqField{State: codeplug.Known, Value: 600},
	}}
	if _, err := sess.WriteChannel(context.Background(), want); err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	got, err := sess.ReadChannel(context.Background(), "050")
	if err != nil {
		t.Fatalf("ReadChannel after write: %v", err)
	}
	if got.Data == nil {
		t.Fatal("ReadChannel after write returned a blank channel")
	}
	if got.Data.FreqHz != want.Data.FreqHz || got.Data.Mode != want.Data.Mode || got.Data.Shift != want.Data.Shift {
		t.Errorf("read-back = %+v, want FreqHz/Mode/Shift from %+v", got.Data, want.Data)
	}
}

// TestCrossedWrongRadio_FT890AgainstFakeFT900 opens the FT-890 driver
// against a live fakeft900.Radio: channel 33 is inside FT-900's true
// 1-100 range, so the fake answers Open's boundary probe — which the
// FT-890 driver (AnswersBoundaryProbe false) reads as proof it is
// talking to the other radio.
func TestCrossedWrongRadio_FT890AgainstFakeFT900(t *testing.T) {
	radio := fakeft900.New()
	defer radio.Close()

	_, err := NewFT890(RealHardware).Open(context.Background(), radio.Port(), driver.Identity{})
	if !errors.Is(err, driver.ErrWrongRadio) {
		t.Fatalf("Open error = %v, want errors.Is(_, driver.ErrWrongRadio)", err)
	}
}

// TestCrossedWrongRadio_FT900AgainstFakeFT890 is the reverse: channel 33
// is outside FT-890's true 1-32 range, so the fake stays silent — which
// the FT-900 driver (AnswersBoundaryProbe true) reads as proof it is
// talking to the other radio.
func TestCrossedWrongRadio_FT900AgainstFakeFT890(t *testing.T) {
	radio := fakeft890.New()
	defer radio.Close()

	_, err := NewFT900(RealHardware).Open(context.Background(), radio.Port(), driver.Identity{})
	if !errors.Is(err, driver.ErrWrongRadio) {
		t.Fatalf("Open error = %v, want errors.Is(_, driver.ErrWrongRadio)", err)
	}
}
