// SPDX-License-Identifier: GPL-3.0-or-later

package ft890900

import (
	"context"
	"fmt"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/internal/fakeft890"
)

// This file is Phase 5's own driver-against-fake round-trip suite
// (plan.md §Phase 5, Codex #11 v2 change): internal/fakeft890 (Phase 4)
// was built quarantined from this package's own driver code, so pairing
// the two together is this phase's job, not Phase 3's or Phase 4's.

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
