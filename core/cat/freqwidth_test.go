// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"errors"
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
				var ftw *FreqTooWideError
				if !errors.As(err, &ftw) {
					t.Fatalf("MemoryFreqHz(%d) error = %v, want a *FreqTooWideError", tt.in, err)
				}
				if ftw.FreqHz != tt.in {
					t.Errorf("FreqTooWideError.FreqHz = %d, want %d", ftw.FreqHz, tt.in)
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
