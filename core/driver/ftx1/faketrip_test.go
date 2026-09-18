// SPDX-License-Identifier: GPL-3.0-or-later

package ftx1

import (
	"context"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/internal/fakeftx1"
)

// This file is the milestone's own registration-phase (F4) driver-against-
// fake round trip: internal/fakeftx1 (F3) was built quarantined from this
// package's own driver code (its own doc.go says so; its register_test.go
// checks only its own ASSUMED register's completeness, not this package),
// so pairing the two together is this phase's job — the ft1000mp/
// ft890900 faketrip_test.go shape, not ftdx1200's scripted-responder one,
// because F4's own acceptance criteria name a read across every bank AND
// both arms of the consent gate against a LIVE fake radio, which only a
// real fake session can prove end to end.

// TestRoundTrip_FTX1AgainstFake reads across every bank — memory, PMS,
// the 5 MHz band and EMGCH (MR and MT together, Session's own doc
// comment) — on a Simulated session, then proves a write is visible on a
// later read. DefaultImage tags every populated slot (its own doc
// comment): a successful MR followed by a rejected MT is a genuine
// ReadChannel error on this driver (read.go's own doc comment), not an
// empty-tag signal, so the default image leaves nothing half-populated.
func TestRoundTrip_FTX1AgainstFake(t *testing.T) {
	radio := fakeftx1.New()
	defer radio.Close()

	sess, err := New(Simulated).Open(context.Background(), radio.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer sess.Close()

	for _, slot := range []string{"00001", "P-01L", "50001", "EMGCH"} {
		if _, err := sess.ReadChannel(context.Background(), slot); err != nil {
			t.Fatalf("ReadChannel(%q): %v", slot, err)
		}
	}

	want := codeplug.Channel{Slot: "00003", Data: &codeplug.ChannelData{
		FreqHz: 14_250_000,
		Mode:   "USB",
		Shift:  "SIMPLEX",
		CTCSS:  "ENC",
		Tag:    "TESTCHAN",
	}}
	if _, err := sess.WriteChannel(context.Background(), want); err != nil {
		t.Fatalf("WriteChannel (Simulated): %v", err)
	}
	got, err := sess.ReadChannel(context.Background(), "00003")
	if err != nil {
		t.Fatalf("ReadChannel after write: %v", err)
	}
	if got.Data == nil {
		t.Fatal("ReadChannel after write returned a blank channel")
	}
	if got.Data.FreqHz != want.Data.FreqHz || got.Data.Mode != want.Data.Mode || got.Data.Shift != want.Data.Shift || got.Data.CTCSS != want.Data.CTCSS || got.Data.Tag != want.Data.Tag {
		t.Errorf("read-back = %+v, want FreqHz/Mode/Shift/CTCSS/Tag from %+v", got.Data, want.Data)
	}

	// A sibling slot must not see the write above.
	other, err := sess.ReadChannel(context.Background(), "00001")
	if err != nil {
		t.Fatalf("ReadChannel(00001): %v", err)
	}
	if other.Data != nil && other.Data.FreqHz == want.Data.FreqHz && other.Data.Tag == want.Data.Tag {
		t.Errorf("ReadChannel(00001) = %+v, want the seed image unchanged: the write above must not leak into a sibling slot", other.Data)
	}
}

// TestRoundTrip_FTX1AgainstFake_ConsentGate is the write half of F4's own
// acceptance criteria: a RealHardware session WITHOUT
// WithConsentedUnverifiedWrites refuses every write before any wire
// traffic reaches the radio (write.go's own doc comment — "Refusal comes
// FIRST, before ANY wire traffic"), so the refusal is proved against a
// radio that never has to answer anything; WITH the option, against a
// live fake radio, the write succeeds and reads back identical. Two
// separate fake radios, deliberately: reusing one port across two Open
// calls would race two transport.Engines over the same underlying
// connection, which the refused arm's own zero wire traffic does not
// need and the consented arm must not risk.
func TestRoundTrip_FTX1AgainstFake_ConsentGate(t *testing.T) {
	plainRadio := fakeftx1.New()
	defer plainRadio.Close()
	plain, err := New(RealHardware).Open(context.Background(), plainRadio.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open (no consent): %v", err)
	}
	defer plain.Close()

	ch := codeplug.Channel{Slot: "00004", Data: &codeplug.ChannelData{
		FreqHz: 7_100_000,
		Mode:   "LSB",
		Shift:  "SIMPLEX",
		CTCSS:  "OFF",
		Tag:    "NOCONSENT",
	}}
	if _, err := plain.WriteChannel(context.Background(), ch); err == nil {
		t.Fatal("WriteChannel without consent succeeded, want refused before any wire traffic")
	}

	radio := fakeftx1.New()
	defer radio.Close()
	consented, err := New(RealHardware, WithConsentedUnverifiedWrites()).Open(context.Background(), radio.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open (consented): %v", err)
	}
	defer consented.Close()

	if _, err := consented.WriteChannel(context.Background(), ch); err != nil {
		t.Fatalf("WriteChannel with consent: %v", err)
	}
	got, err := consented.ReadChannel(context.Background(), "00004")
	if err != nil {
		t.Fatalf("ReadChannel after consented write: %v", err)
	}
	if got.Data == nil {
		t.Fatal("ReadChannel after consented write returned a blank channel")
	}
	if got.Data.FreqHz != ch.Data.FreqHz || got.Data.Mode != ch.Data.Mode || got.Data.Shift != ch.Data.Shift || got.Data.CTCSS != ch.Data.CTCSS || got.Data.Tag != ch.Data.Tag {
		t.Errorf("read-back = %+v, want FreqHz/Mode/Shift/CTCSS/Tag from %+v", got.Data, ch.Data)
	}
}
