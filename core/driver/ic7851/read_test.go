// SPDX-License-Identifier: GPL-3.0-or-later

package ic7851

import (
	"errors"
	"testing"

	civic7851 "github.com/gm5dna/open-rig-programmer/core/civ/ic7851"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
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

// TestReadChannel_FreshReadSurvivesSaveLoad pins the D8 fresh-read rule on
// this driver (drivertest.AssertFreshReadSaveLoad): a freshly read
// occupied channel reports the seven receiver fields Unavailable, and
// survives a save/load round trip byte-for-byte.
func TestReadChannel_FreshReadSurvivesSaveLoad(t *testing.T) {
	r := newFake(t)
	f := e2eSeed(0)
	r.SetSlot("001", f.record(t))
	s, _ := openFake(t, r, New7851)
	ch, err := s.ReadChannel(t.Context(), "001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	drivertest.AssertFreshReadSaveLoad(t, ch, s.Capabilities(), codeplug.Load)
}
