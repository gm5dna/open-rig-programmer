// SPDX-License-Identifier: GPL-3.0-or-later

package ic7700

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic7700 "github.com/gm5dna/open-rig-programmer/core/civ/ic7700"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// buildRecord uses the CI-V builder so a test record exercises the same
// encoding the driver later parses, then strips the set envelope (the
// leading FE FE 74 E0 1A 00 <ch-hi> <ch-lo> and the trailing FD) down to
// the RecordOnlyLength body a scriptedPort answer carries.
func buildRecord(t *testing.T, rec civ.MemoryRecord) []byte {
	t.Helper()
	cmd, err := civic7700.Profile().BuildMemorySet(rec)
	if err != nil {
		t.Fatalf("BuildMemorySet: %v", err)
	}
	frame := cmd.Bytes()
	return frame[8 : len(frame)-1]
}

// seedRecord is a simple, fully-populated record for channel ch — every
// mapped field Known, nothing exotic.
func seedRecord(t *testing.T, ch int) []byte {
	t.Helper()
	return buildRecord(t, civ.MemoryRecord{
		Address:      civ.ChannelAddress{Channel: ch},
		RXFreqHz:     civ.Available[uint64](14_250_000),
		TXFreqHz:     civ.Available[uint64](14_250_000),
		Mode:         civ.Available("USB"),
		Filter:       civ.Available("FIL1"),
		ToneMode:     civ.Available("TONE"),
		ToneTXDeciHz: civ.Available[uint64](885),
		ToneRXDeciHz: civ.Available[uint64](1000),
		Name:         civ.Available("HOME"),
	})
}

// openScripted opens a Session against a scriptedPort answering img, under
// the Simulated profile (writable, no consent needed) unless opts say
// otherwise.
func openScripted(t *testing.T, img radioImage, opts ...Option) (*Session, *scriptedPort) {
	t.Helper()
	p := newScriptedPort(t, img)
	d := New(Simulated, opts...)
	sess, err := d.Open(t.Context(), p.Port(), driver.Identity{Port: "/dev/scripted"})
	if err != nil {
		t.Fatalf("Open: %v\ntranscript:\n  %s", err, hexFrames(p.Transcript()))
	}
	s, ok := sess.(*Session)
	if !ok {
		t.Fatalf("Open returned %T, want *Session", sess)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, p
}

// occupiedImage answers the identity probe and carries one seed record at
// channel 1, so Open's fingerprint search settles quickly.
func occupiedImage(t *testing.T) radioImage {
	return radioImage{
		idToken: []byte{0x01, 0x74},
		records: map[int][]byte{1: seedRecord(t, 1)},
	}
}

func TestSlotMapping(t *testing.T) {
	for _, tc := range []struct {
		slot    string
		channel int
	}{{"001", 1}, {"099", 99}, {"P1", 100}, {"P2", 101}} {
		a, _, err := slotToAddress(tc.slot)
		if err != nil || a.Channel != tc.channel || a.Group != 0 {
			t.Fatalf("%s -> %#v, %v", tc.slot, a, err)
		}
	}
	for _, slot := range []string{"000", "100", "101", "CALL", "G01-001"} {
		if _, _, err := slotToAddress(slot); err == nil {
			t.Errorf("slot %q unexpectedly accepted", slot)
		}
	}
}

func TestAllFFIsEmpty(t *testing.T) {
	if recordIsAbsent(make([]byte, civic7700.RecordOnlyLength)) {
		t.Fatal("zero record treated as empty")
	}
	ff := make([]byte, civic7700.RecordOnlyLength)
	for i := range ff {
		ff[i] = 0xff
	}
	if !recordIsAbsent(ff) {
		t.Fatal("all-FF record not treated as empty")
	}
	if recordIsAbsent(nil) {
		t.Fatal("nil record treated as empty")
	}
}

// TestReadChannel_RefusesFrequencyOutsideRadioDomain pins both bounds the
// domain rung applies, including the ceiling+1 regression.
func TestReadChannel_RefusesFrequencyOutsideRadioDomain(t *testing.T) {
	for _, hz := range []uint64{MaxFreqHz + 1} {
		t.Run(fmt.Sprintf("%d Hz", hz), func(t *testing.T) {
			img := occupiedImage(t)
			img.records[42] = buildRecord(t, civ.MemoryRecord{
				Address: civ.ChannelAddress{Channel: 42}, RXFreqHz: civ.Available(hz), TXFreqHz: civ.Available(hz),
				Mode: civ.Available("USB"), Filter: civ.Available("FIL1"), ToneMode: civ.Available("OFF"),
				ToneTXDeciHz: civ.Available[uint64](885), ToneRXDeciHz: civ.Available[uint64](1000), Name: civ.Available("X"),
			})
			s, _ := openScripted(t, img)
			ch, err := s.ReadChannel(t.Context(), "042")
			var domain *OutOfDomainError
			if !errors.As(err, &domain) {
				t.Fatalf("ReadChannel = (%+v, %v), want *OutOfDomainError", ch, err)
			}
			if domain.Field != spec.FieldFrequency || domain.Value != hz || domain.Min != MinFreqHz || domain.Max != MaxFreqHz {
				t.Errorf("OutOfDomainError = %+v, want {frequency, %d, %d, %d}", domain, hz, uint64(MinFreqHz), uint64(MaxFreqHz))
			}
			if !errors.Is(err, ErrOutOfDomain) {
				t.Errorf("errors.Is(%v, ErrOutOfDomain) = false", err)
			}
			if msg := err.Error(); !strings.Contains(msg, fmt.Sprint(hz)) || !strings.Contains(msg, fmt.Sprint(MaxFreqHz)) {
				t.Errorf("error %q does not render measured frequency %d and ceiling %d", msg, hz, uint64(MaxFreqHz))
			}
			if !ch.Empty() {
				t.Errorf("refused read returned a populated channel: %+v", ch)
			}
		})
	}
}

// TestReadChannel_AcceptsFrequencyAtCeiling pins the strict > comparison.
func TestReadChannel_AcceptsFrequencyAtCeiling(t *testing.T) {
	img := occupiedImage(t)
	img.records[42] = buildRecord(t, civ.MemoryRecord{
		Address: civ.ChannelAddress{Channel: 42}, RXFreqHz: civ.Available[uint64](MaxFreqHz), TXFreqHz: civ.Available[uint64](MaxFreqHz),
		Mode: civ.Available("USB"), Filter: civ.Available("FIL1"), ToneMode: civ.Available("OFF"),
		ToneTXDeciHz: civ.Available[uint64](0), ToneRXDeciHz: civ.Available[uint64](0), Name: civ.Available("X"),
	})
	s, _ := openScripted(t, img)
	ch, err := s.ReadChannel(t.Context(), "042")
	if err != nil {
		t.Fatalf("ReadChannel at %d Hz: %v", uint64(MaxFreqHz), err)
	}
	if ch.Empty() || ch.Data.FreqHz != MaxFreqHz {
		t.Errorf("ReadChannel at ceiling = %+v, want Known frequency %d", ch, uint64(MaxFreqHz))
	}
}

// TestReadChannel_TxFrequencyAlwaysKnown pins that idx15-19 (the
// TX-duplicate block's own frequency span) is ALWAYS decoded Known: the
// manual states the block is "still necessary" even with Split OFF
// (matrix §1 row 4), so unlike every field the record has no span for,
// this one is never Unavailable.
func TestReadChannel_TxFrequencyAlwaysKnown(t *testing.T) {
	s, _ := openScripted(t, occupiedImage(t))
	ch, err := s.ReadChannel(t.Context(), "001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if ch.Data.TxFreqHz.State != codeplug.Known || ch.Data.TxFreqHz.Value != 14_250_000 {
		t.Errorf("TxFreqHz = %+v, want Known 14250000", ch.Data.TxFreqHz)
	}
}

// TestReadChannel_FreshReadSurvivesSaveLoad pins the fresh-read rule: a
// freshly read occupied channel survives a save/load round trip
// field-for-field.
func TestReadChannel_FreshReadSurvivesSaveLoad(t *testing.T) {
	s, _ := openScripted(t, occupiedImage(t))
	ch, err := s.ReadChannel(t.Context(), "001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	drivertest.AssertFreshReadSaveLoad(t, ch, s.Capabilities(), codeplug.Load)
}
