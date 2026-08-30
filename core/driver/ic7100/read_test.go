// SPDX-License-Identifier: GPL-3.0-or-later

package ic7100

import (
	"context"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/clone"
)

func TestReadAllWalksExactlyDenseBanksAThroughE(t *testing.T) {
	p := newRespondingPort(t, withRecord(1, 1, occupiedRecord(t)))
	s := openTestSession(t, p)
	service := clone.NewService(s, clone.SnapshotStore{})
	cp, err := service.ReadAll(context.Background())
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(cp.Channels) != 495 || cp.Channels[0].Slot != "A-001" || cp.Channels[494].Slot != "E-099" {
		t.Fatalf("channels = %d, first/last %q/%q", len(cp.Channels), cp.Channels[0].Slot, cp.Channels[494].Slot)
	}
	if cp.Channels[0].Data == nil || cp.Channels[0].Data.FreqHz != 145_500_000 || cp.Channels[0].Data.Tag != "HOME BASE" {
		t.Errorf("A-001 = %+v, want occupied golden", cp.Channels[0])
	}
	for _, f := range p.frames() {
		if len(f) == 10 && f[4] == 0x1A && f[5] == 0x00 {
			bank, channel := decodeBCD(f[6]), decodeBCD(f[7])*100+decodeBCD(f[8])
			if bank < 1 || bank > 5 || channel < 1 || channel > 99 {
				t.Errorf("ReadAll sent out-of-scope address % X", f[6:9])
			}
		}
	}
}

func TestReadChannelTreatsFAAndAllFFAsSeparateEmptyForms(t *testing.T) {
	ff := make([]byte, 111)
	for i := range ff {
		ff[i] = 0xFF
	}
	p := newRespondingPort(t, withRecord(1, 2, ff), withRecord(1, 1, occupiedRecord(t)))
	s := openTestSession(t, p)
	for _, slot := range []string{"A-002", "A-003"} {
		ch, err := s.ReadChannel(context.Background(), slot)
		if err != nil || ch.Data != nil {
			t.Errorf("ReadChannel(%s) = %+v, %v; want empty", slot, ch, err)
		}
	}
}

func TestReadChannelRefusesSpecialsBeforeTraffic(t *testing.T) {
	p := newRespondingPort(t, withRecord(1, 1, occupiedRecord(t)))
	s := openTestSession(t, p)
	before := len(p.frames())
	for _, slot := range []string{"A-000", "A-100", "A-106", "F-001", "01-001"} {
		if _, err := s.ReadChannel(context.Background(), slot); err == nil {
			t.Errorf("ReadChannel(%q) succeeded", slot)
		}
	}
	if got := len(p.frames()); got != before {
		t.Errorf("invalid reads sent %d frames", got-before)
	}
}
