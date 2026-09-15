// SPDX-License-Identifier: GPL-3.0-or-later

package bincat

import (
	"bytes"
	"errors"
	"testing"
)

// TestEncodeBCD_FT1000MPWorkedExample pins the ONE write-side BCD fixture
// this package keeps from the deferred FT-920/consented FT-1000MP evidence
// (plan.md Phase 1 v2 change, Codex #3): "Set Main VFO-A to 14.25000 MHz"
// (FT-1000MP manual, CONSTRUCTING AND SENDING CAT COMMANDS, p.86-87) is
// byte-for-byte identical to FT-920's own p.88 example. Neither radio ships
// a Profile this milestone; this is a round-trip codec fixture only.
func TestEncodeBCD_FT1000MPWorkedExample(t *testing.T) {
	const freqTensOfHz = 1425000 // 14.25000 MHz, in units of 10 Hz
	want := []byte{0x00, 0x50, 0x42, 0x01}

	got, err := EncodeBCD(freqTensOfHz, 4)
	if err != nil {
		t.Fatalf("EncodeBCD: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("EncodeBCD(%d, 4) = % x, want % x", freqTensOfHz, got, want)
	}

	back, err := DecodeBCD(got)
	if err != nil {
		t.Fatalf("DecodeBCD: %v", err)
	}
	if back != freqTensOfHz {
		t.Fatalf("DecodeBCD(% x) = %d, want %d", got, back, freqTensOfHz)
	}
}

func TestBCDRoundTripTable(t *testing.T) {
	for _, v := range []uint64{0, 1, 99, 100, 12345678, 99999999} {
		enc, err := EncodeBCD(v, 4)
		if err != nil {
			t.Fatalf("EncodeBCD(%d, 4): %v", v, err)
		}
		dec, err := DecodeBCD(enc)
		if err != nil {
			t.Fatalf("DecodeBCD(% x): %v", enc, err)
		}
		if dec != v {
			t.Fatalf("round trip of %d gave %d (via % x)", v, dec, enc)
		}
	}
}

func TestEncodeBCDOverflow(t *testing.T) {
	if _, err := EncodeBCD(100, 1); !errors.Is(err, ErrBCD) {
		t.Fatalf("EncodeBCD(100, 1) error = %v, want ErrBCD", err)
	}
}

func TestEncodeBCDBadWidth(t *testing.T) {
	for _, n := range []int{0, -1, maxBCDBytes + 1} {
		if _, err := EncodeBCD(0, n); !errors.Is(err, ErrBCD) {
			t.Errorf("EncodeBCD(0, %d) error = %v, want ErrBCD", n, err)
		}
	}
}

func TestDecodeBCDInvalidNibble(t *testing.T) {
	if _, err := DecodeBCD([]byte{0xFA}); !errors.Is(err, ErrBCD) {
		t.Fatalf("DecodeBCD([0xFA]) error = %v, want ErrBCD", err)
	}
}

func TestDecodeBCDBadWidth(t *testing.T) {
	if _, err := DecodeBCD(nil); !errors.Is(err, ErrBCD) {
		t.Fatalf("DecodeBCD(nil) error = %v, want ErrBCD", err)
	}
}
