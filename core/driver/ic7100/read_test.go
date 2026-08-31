// SPDX-License-Identifier: GPL-3.0-or-later

package ic7100

import (
	"context"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/clone"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
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

// TestReadChannel_FreshReadSurvivesSaveLoad pins the D8 fresh-read rule on
// this driver (drivertest.AssertFreshReadSaveLoad): a freshly read
// occupied channel reports the seven receiver fields Unavailable, and
// survives a save/load round trip byte-for-byte.
func TestReadChannel_FreshReadSurvivesSaveLoad(t *testing.T) {
	p := newRespondingPort(t, withRecord(1, 1, occupiedRecord(t)))
	s := openTestSession(t, p)
	ch, err := s.ReadChannel(context.Background(), "A-001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	drivertest.AssertFreshReadSaveLoad(t, ch, codeplug.Load)
}

// fullMemoryRecord is every field channelData reads, all present, so a
// table test can knock one out at a time and see only that field move.
func fullMemoryRecord() civ.MemoryRecord {
	return civ.MemoryRecord{
		Address:      civ.ChannelAddress{Group: 1, Channel: 1},
		RXFreqHz:     civ.Available(uint64(145_500_000)),
		TXFreqHz:     civ.Available(uint64(145_500_000)),
		OffsetHz:     civ.Available(uint64(600_000)),
		ToneTXDeciHz: civ.Available(uint64(885)),
		ToneRXDeciHz: civ.Available(uint64(885)),
		DTCSCode:     civ.Available(uint64(23)),
		Duplex:       civ.Available("OFF"),
		Mode:         civ.Available("FM"),
		Filter:       civ.Available("FIL1"),
		DataMode:     civ.Available("OFF"),
		ToneMode:     civ.Available("OFF"),
		DTCSPolarity: civ.Available("NN"),
		Name:         civ.Available("HOME BASE"),
		Select:       civ.Available("OFF"),
	}
}

// TestChannelData_AbsentTxFreqAndOffsetAreUnavailable pins the correct
// non-Known FreqField state for TxFreqHz/OffsetHz when civ.Optional
// reports the record does not carry them.
//
// numberOf discards Optional's presence flag, so an absent TXFreqHz or
// OffsetHz used to read as Known 0 — indistinguishable from a genuine
// zero-Hz value. FreqHz (RXFreqHz) is left as the bare numberOf
// conversion: it is a plain uint64 by a known upstream defect that a
// separate design task owns, not this one.
func TestChannelData_AbsentTxFreqAndOffsetAreUnavailable(t *testing.T) {
	p := newRespondingPort(t)
	s := openTestSession(t, p)
	for _, tc := range []struct {
		name  string
		clear func(*civ.MemoryRecord)
		get   func(codeplug.ChannelData) codeplug.FreqField
	}{
		{"TxFreqHz", func(r *civ.MemoryRecord) { r.TXFreqHz = civ.Optional[uint64]{} }, func(d codeplug.ChannelData) codeplug.FreqField { return d.TxFreqHz }},
		{"OffsetHz", func(r *civ.MemoryRecord) { r.OffsetHz = civ.Optional[uint64]{} }, func(d codeplug.ChannelData) codeplug.FreqField { return d.OffsetHz }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := fullMemoryRecord()
			tc.clear(&rec)
			got := tc.get(s.channelData(rec))
			if got != (codeplug.FreqField{State: codeplug.Unavailable}) {
				t.Errorf("%s = %+v, want Unavailable — the record does not carry it", tc.name, got)
			}
		})
	}
}
