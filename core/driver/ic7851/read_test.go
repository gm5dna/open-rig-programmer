// SPDX-License-Identifier: GPL-3.0-or-later

package ic7851

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic7851 "github.com/gm5dna/open-rig-programmer/core/civ/ic7851"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// TestFixedDigitBytesAreRefusedOnRead is the READ half of F1.
//
// Excluding ⑧, ⑫ and ⑮ from their spans is what stops the BUILDER and the
// GATE putting a digit in a printed-fixed byte, but it also means the
// record parser no longer looks at those bytes at all. A radio answering
// with a digit in one of them would therefore be read as a DIFFERENT and
// entirely plausible value — 145.5 MHz arriving as 45.5 MHz — and written
// back with the byte quietly zeroed. Neither is an outcome a caller could
// distinguish from success, so the record is refused instead.
func TestFixedDigitBytesAreRefusedOnRead(t *testing.T) {
	if err := fixedDigitsDiffer(make([]byte, civic7851.RecordOnlyLength)); err != nil {
		t.Fatalf("an all-zero record was refused: %v", err)
	}
	for _, tc := range []struct {
		offset int
		name   string
	}{
		{civic7851.FreqFixedOffset, "⑧"},
		{civic7851.ToneTXFixedOffset, "⑫"},
		{civic7851.ToneRXFixedOffset, "⑮"},
	} {
		for _, v := range []byte{0x01, 0x10, 0x99} {
			raw := make([]byte, civic7851.RecordOnlyLength)
			raw[tc.offset] = v
			err := fixedDigitsDiffer(raw)
			var e *FixedDigitError
			if !errors.As(err, &e) {
				t.Errorf("record byte %d = %#02x (printed %s) gave %v, want *FixedDigitError", tc.offset, v, tc.name, err)
				continue
			}
			if e.Offset != tc.offset || e.Got != v {
				t.Errorf("FixedDigitError = %+v, want offset %d got %#02x", e, tc.offset, v)
			}
		}
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
	if recordIsAbsent(make([]byte, 25)) {
		t.Fatal("zero record treated as empty")
	}
	ff := make([]byte, 25)
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

// recordAtFrequency uses the CI-V builder so the test record exercises the
// same frequency encoding the driver parses, then removes the set envelope.
func recordAtFrequency(t *testing.T, hz uint64) []byte {
	t.Helper()
	cmd, err := civic7851.Profile().BuildMemorySet(civ.MemoryRecord{
		Address:      civ.ChannelAddress{Channel: 42},
		RXFreqHz:     civ.Available(hz),
		Mode:         civ.Available("USB"),
		Filter:       civ.Available("FIL1"),
		ToneMode:     civ.Available("TONE"),
		ToneTXDeciHz: civ.Available[uint64](885),
		ToneRXDeciHz: civ.Available[uint64](1000),
		Name:         civ.Available("FREQ LIMIT"),
	})
	if err != nil {
		t.Fatalf("BuildMemorySet(%d Hz): %v", hz, err)
	}
	frame := cmd.Bytes()
	return frame[8 : len(frame)-1]
}

// TestReadChannel_RefusesFrequencyOutsideRadioDomain pins both bounds the
// write-side domain rung applies, including the ceiling+1 regression.
func TestReadChannel_RefusesFrequencyOutsideRadioDomain(t *testing.T) {
	for _, hz := range []uint64{MinRadioFreqHz - 1, MaxRadioFreqHz + 1} {
		t.Run(fmt.Sprintf("%d Hz", hz), func(t *testing.T) {
			r := newFake(t)
			r.SetSlot("001", e2eSeed(0).record(t))
			r.SetSlot("042", recordAtFrequency(t, hz))
			s, _ := openFake(t, r, New7851)
			ch, err := s.ReadChannel(t.Context(), "042")
			var domain *OutOfDomainError
			if !errors.As(err, &domain) {
				t.Fatalf("ReadChannel = (%+v, %v), want *OutOfDomainError", ch, err)
			}
			if domain.Field != spec.FieldFrequency || domain.Value != hz || domain.Min != MinRadioFreqHz || domain.Max != MaxRadioFreqHz {
				t.Errorf("OutOfDomainError = %+v, want {frequency, %d, %d, %d}", domain, hz, uint64(MinRadioFreqHz), uint64(MaxRadioFreqHz))
			}
			if !errors.Is(err, ErrOutOfDomain) {
				t.Errorf("errors.Is(%v, ErrOutOfDomain) = false", err)
			}
			if msg := err.Error(); !strings.Contains(msg, fmt.Sprint(hz)) || !strings.Contains(msg, fmt.Sprint(MaxRadioFreqHz)) {
				t.Errorf("error %q does not render measured frequency %d and ceiling %d", msg, hz, uint64(MaxRadioFreqHz))
			}
			if !ch.Empty() {
				t.Errorf("refused read returned a populated channel: %+v", ch)
			}
		})
	}
}

// TestReadChannel_AcceptsFrequencyAtCeiling pins the strict > comparison:
// MaxRadioFreqHz is the largest valid value, not the first invalid one.
func TestReadChannel_AcceptsFrequencyAtCeiling(t *testing.T) {
	r := newFake(t)
	r.SetSlot("001", e2eSeed(0).record(t))
	r.SetSlot("042", recordAtFrequency(t, MaxRadioFreqHz))
	s, _ := openFake(t, r, New7851)
	ch, err := s.ReadChannel(t.Context(), "042")
	if err != nil {
		t.Fatalf("ReadChannel at %d Hz: %v", uint64(MaxRadioFreqHz), err)
	}
	if ch.Empty() || ch.Data.FreqHz != MaxRadioFreqHz {
		t.Errorf("ReadChannel at ceiling = %+v, want Known frequency %d", ch, uint64(MaxRadioFreqHz))
	}
}

// TestReadChannel_FreshReadSurvivesSaveLoad pins the D8 fresh-read rule on
// this driver (drivertest.AssertFreshReadSaveLoad): a freshly read
// occupied channel reports the seven receiver fields Unavailable, answers
// all seventeen tier fields rather than leaving one Absent, and survives a
// save/load round trip byte-for-byte.
//
// BOTH ROWS, like every E2E case in this package: New7851 and New7850
// share one implementation, one profile and one capability set, so the
// second invocation asserts nothing new about this package — it is what
// makes the REGISTERED IC-7850 a model the fleet's per-driver arm actually
// exercises. internal/wiring's
// TestOpenFakeSessionFor_EveryRegisteredModel_ReadsEveryDefaultSlot
// delegates the ten Icom models with empty default fake images to exactly
// this call, and until this loop the IC-7850 was named there without
// anything running for it.
func TestReadChannel_FreshReadSurvivesSaveLoad(t *testing.T) {
	for _, c := range constructors {
		t.Run(c.model, func(t *testing.T) {
			r := newFake(t)
			f := e2eSeed(0)
			r.SetSlot("001", f.record(t))
			s, _ := openFake(t, r, c.make)
			ch, err := s.ReadChannel(t.Context(), "001")
			if err != nil {
				t.Fatalf("ReadChannel: %v", err)
			}
			drivertest.AssertFreshReadSaveLoad(t, ch, s.Capabilities(), codeplug.Load)
		})
	}
}
