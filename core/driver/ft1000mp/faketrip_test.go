// SPDX-License-Identifier: GPL-3.0-or-later

package ft1000mp

import (
	"context"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/internal/fakeft1000mp"
)

// This file is Phase 5's own driver-against-fake round-trip suite
// (plan.md §Phase 5, Codex #11 v2 change): internal/fakeft1000mp (Phase
// 4) was built quarantined from this package's own driver code, so
// pairing the two together is this phase's job. Per Stuart's
// 15/09/2026 override, Store/Enter SHIPS as Unverified/consent-gated —
// not Unsupported — so a Simulated session (already all-Supported) can
// write here with no consent option needed, exactly as every other
// registered model's own fake round trip does.
func TestRoundTrip_FT1000MPAgainstFake(t *testing.T) {
	radio := fakeft1000mp.New()
	defer radio.Close()

	sess, err := New(Simulated).Open(context.Background(), radio.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer sess.Close()

	for _, slot := range append(append(memSlots(), pSlots()...), qmbSlots()...) {
		if _, err := sess.ReadChannel(context.Background(), slot); err != nil {
			t.Fatalf("ReadChannel(%q): %v", slot, err)
		}
	}

	want := codeplug.Channel{Slot: "5", Data: &codeplug.ChannelData{
		FreqHz: 14_250_000,
		Mode:   "USB",
		Shift:  "SIMPLEX",
	}}
	if _, err := sess.WriteChannel(context.Background(), want); err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	got, err := sess.ReadChannel(context.Background(), "5")
	if err != nil {
		t.Fatalf("ReadChannel after write: %v", err)
	}
	if got.Data == nil {
		t.Fatal("ReadChannel after write returned a blank channel")
	}
	if got.Data.FreqHz != want.Data.FreqHz || got.Data.Mode != want.Data.Mode || got.Data.Shift != want.Data.Shift {
		t.Errorf("read-back = %+v, want FreqHz/Mode/Shift from %+v", got.Data, want.Data)
	}

	// Store's channel argument is not the one under test here (the
	// package's own write_test.go pins it byte-for-byte) — this test's
	// job is only that a write against a LIVE fake radio genuinely
	// changes what a later read sees, and that every slot is at least
	// readable once, end to end. This radio's untouched record decodes
	// as FreqHz 0/LSB, not a codeplug.Channel with nil Data — there is
	// no "populated" flag byte for ReadChannel to test (matrix §1.2) —
	// so the leak check is on FreqHz alone, not Data being nil.
	other, err := sess.ReadChannel(context.Background(), "6")
	if err != nil {
		t.Fatalf("ReadChannel(6): %v", err)
	}
	if other.Data != nil && other.Data.FreqHz == want.Data.FreqHz {
		t.Errorf("ReadChannel(6) FreqHz = %d, want 0: the write above must not leak into a sibling slot", other.Data.FreqHz)
	}
}
