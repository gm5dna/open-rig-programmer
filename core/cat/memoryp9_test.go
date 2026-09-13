// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"strings"
	"testing"
)

// This file is the v1.8.0 dx3000 lift's own seam, MemoryP9Policy's third
// value: cat.P9ToneIndexReadOnly, byte pair 25-26 of the shared memory
// field block. A new file rather than an appendix to memdata_test.go, for
// memoryp5_test.go's own reason (that file's header comment) — and because
// memdata_test.go carries a frozen literal-order golden
// (TestEvidenceLiterals_OrderedRecordsSurvive) this axis must not perturb.

// p9ReadOnlyDialect declares MemoryP9 P9ToneIndexReadOnly: the FTdx3000's
// asymmetric P9 (dialectconfig.go's MemoryP9Policy doc comment) — live
// tone-table index on read, printed-fixed "00" refused-if-nonzero on
// write. The one axis this fixture varies is P9, mirroring p5FixedDialect's
// shape for its own axis.
var p9ReadOnlyDialect = mustFixtureDialect(DialectConfig{
	CATID:     "0462",
	ModeNames: map[Mode]string{ModeUnset: "-", ModeUSB: "USB"},
	Slots: SlotSpace{
		MemoryLo: 1, MemoryHi: 99,
		PMSPairs:     9,
		PMSForm:      PMSFormNumeric,
		PMSNumericLo: 100,
		MCSelects:    MCSelectsAll,
	},
	EXAddressForm: EXAddressSingle,
	MT: MTPolicy{
		Form: MTFormShort, ReadSlots: MTReadsReadable,
		TagMaxBytes: 1, ClearTagByte: ' ',
	},
	Clarifier:        ClarifierPolicy{StepHz: 10, MaxAbsHz: 9990},
	MemoryP5:         P5TxClar,
	MemoryP9:         P9ToneIndexReadOnly, // THE AXIS UNDER TEST
	ToneStates:       ToneStatesCTCSS,
	MWWriteKind:      KindVFO,
	MemoryFrameLen:   27,
	MemoryFreqDigits: 8,
})

// TestMemoryP9_ReadOnlyIndex is the third MemoryP9Policy value's own
// table: live on read, fixed-and-refuse-nonzero on write (ftdx3000 matrix
// §1.3; v1.8.0 phase 2 dx3000 driver brief).
func TestMemoryP9_ReadOnlyIndex(t *testing.T) {
	slot, err := p9ReadOnlyDialect.MemorySlot(7)
	if err != nil {
		t.Fatalf("fixture broken: MemorySlot(7): %v", err)
	}
	base := MemoryData{
		Slot: slot, FreqHz: 14_250_000, Mode: ModeUSB,
		Kind: p9ReadOnlyDialect.MWWriteKind(), CTCSS: CTCSSOff, Shift: ShiftSimplex,
	}

	t.Run("write encodes fixed 00", func(t *testing.T) {
		cmd, err := p9ReadOnlyDialect.BuildMWSet(base)
		if err != nil {
			t.Fatalf("BuildMWSet with ToneIndex 0 = %v, want accepted (P9 is schema on write)", err)
		}
		if got := string(cmd.Bytes()[p9ReadOnlyDialect.memP9Off() : p9ReadOnlyDialect.memP9Off()+2]); got != "00" {
			t.Errorf("BuildMWSet wrote P9 %q, want \"00\"", got)
		}
	})

	t.Run("write refuses a nonzero tone", func(t *testing.T) {
		m := base
		m.ToneIndex = 5
		cmd, err := p9ReadOnlyDialect.BuildMWSet(m)
		if err == nil {
			t.Fatalf("BuildMWSet with ToneIndex 5 succeeded, emitting %q — the FTdx3000's MW P9 is printed-fixed \"00\", there is no write-side tone index to set", cmd.Bytes())
		}
		for _, want := range []string{"P9", P9ToneIndexReadOnly.String()} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("refusal %q does not mention %q", err, want)
			}
		}
	})

	t.Run("read decodes a live index", func(t *testing.T) {
		clean, err := p9ReadOnlyDialect.BuildMWSet(base)
		if err != nil {
			t.Fatalf("BuildMWSet: %v", err)
		}
		frame := append([]byte(nil), clean.Bytes()...)
		frame[0], frame[1] = 'M', 'R'
		copy(frame[p9ReadOnlyDialect.memP9Off():], "26")

		got, err := p9ReadOnlyDialect.ParseMRAnswer(frame)
		if err != nil {
			t.Fatalf("ParseMRAnswer(%q) = %v, want a live tone index to decode", frame, err)
		}
		if got.ToneIndex != 26 {
			t.Errorf("ParseMRAnswer(%q).ToneIndex = %d, want 26 — MR's P9 is live under P9ToneIndexReadOnly even though MW's is fixed", frame, got.ToneIndex)
		}
	})
}
