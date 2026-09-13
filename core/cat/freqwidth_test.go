// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"fmt"
	"math"
	"testing"
)

// TestMemoryFreqHz is the ONE conversion between the neutral model's
// uint64 frequency and this package's uint32 (design D4, item 7). It is
// tested directly because its whole purpose is a refusal that the four
// registered radios can never trigger: nothing else would ever exercise
// it, and an untested refusal is one nobody notices turning into a cast.
func TestMemoryFreqHz(t *testing.T) {
	for _, tt := range []struct {
		name    string
		in      uint64
		want    uint32
		wantErr bool
	}{
		{"zero", 0, 0, false},
		{"an ordinary HF frequency", 14_250_000, 14_250_000, false},
		{"the widest the 9-digit field holds", memFreqMax, memFreqMax, false},
		{"one hertz past the field", memFreqMax + 1, 0, true},
		{"the old uint32 ceiling is past it too", math.MaxUint32, 0, true},
		{"a value that would truncate into a plausible small one", uint64(1)<<32 | 14_250_000, 0, true},
		{"10 GHz, the IC-905's reach", 10_000_000_000, 0, true},
		{"the widest uint64", math.MaxUint64, 0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MemoryFreqHz(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("MemoryFreqHz(%d) = %d, nil; want an error", tt.in, got)
				}
				want := fmt.Sprintf("cat: frequency %d Hz is too large for this protocol's memory frame (maximum %d Hz)", tt.in, memFreqMax)
				if err.Error() != want {
					t.Errorf("MemoryFreqHz(%d) error = %q, want %q", tt.in, err.Error(), want)
				}
				if got != 0 {
					t.Errorf("MemoryFreqHz(%d) returned %d alongside its error, want 0", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("MemoryFreqHz(%d) error = %v, want nil", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("MemoryFreqHz(%d) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

// TestMemoryFreqHz_TruncationIsWhatItRefuses states the point of the
// function in one assertion: for every value it refuses, a bare cast
// would have produced a DIFFERENT, plausible frequency — which is
// exactly the silent corruption a fixed-width wire field invites.
func TestMemoryFreqHz_TruncationIsWhatItRefuses(t *testing.T) {
	in := uint64(1)<<32 | 14_250_000
	if _, err := MemoryFreqHz(in); err == nil {
		t.Fatal("MemoryFreqHz did not refuse a value that truncates into range")
	}
	if uint64(uint32(in)) == in {
		t.Fatal("the fixture no longer truncates; pick a value that does")
	}
	if uint32(in) != 14_250_000 {
		t.Fatalf("uint32(%d) = %d, want the fixture to truncate to a plausible 14.25 MHz", in, uint32(in))
	}
}

// TestValidateSetFields_FreqHzBoundedByDialectsOwnDigitWidth is Codex
// close-review finding P1: BuildMWSet must refuse a FreqHz needing more
// digits than THIS DIALECT'S OWN P2 field (d.memoryFreqDigits), not just
// the registered family's fixed 9. Before this fix an 8-digit dialect
// (the ft2000/ftdx9000 family, MemoryFreqDigits 8) handed a 9-digit value
// had it pass validateSetFields — which only ever checked the package's
// 9-digit memFreqMax — and then overflow encodeMemoryFields' fixed-width
// "%0*d" write into the clarifier sign byte that follows, corrupting the
// frame rather than refusing it.
func TestValidateSetFields_FreqHzBoundedByDialectsOwnDigitWidth(t *testing.T) {
	cfg := validBaselineConfig()
	cfg.MemoryFrameLen, cfg.MemoryFreqDigits = 27, 8 // ft2000-shaped
	d, err := NewDialect(cfg)
	if err != nil {
		t.Fatalf("NewDialect(8-digit peer): %v", err)
	}
	slot, err := d.MemorySlot(10)
	if err != nil {
		t.Fatalf("fixture MemorySlot: %v", err)
	}
	m := MemoryData{
		Slot:  slot,
		Mode:  Mode('2'),
		Kind:  d.MWWriteKind(),
		CTCSS: CTCSSOff,
		Shift: ShiftSimplex,
	}

	// The widest value the 8-digit field can hold, 99,999,999: must build.
	m.FreqHz = 99_999_999
	if _, err := d.BuildMWSet(m); err != nil {
		t.Errorf("BuildMWSet(FreqHz=99999999) under an 8-digit dialect: %v, want it accepted (the field's own widest value)", err)
	}

	// One past it — needs 9 digits, which this dialect's P2 field does not
	// have. Must be REFUSED, never silently truncated or overflowed into
	// the byte that follows.
	m.FreqHz = 100_000_000
	cmd, err := d.BuildMWSet(m)
	if err == nil {
		t.Fatalf("BuildMWSet(FreqHz=100000000) under an 8-digit dialect succeeded, emitting %q — this value needs 9 digits, which this dialect's P2 field does not have", cmd.Bytes())
	}
	if !cmd.IsZero() {
		t.Error("BuildMWSet returned a non-zero Command alongside its refusal")
	}
}
