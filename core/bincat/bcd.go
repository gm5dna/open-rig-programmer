// SPDX-License-Identifier: GPL-3.0-or-later

package bincat

import (
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/civ"
)

// maxBCDBytes bounds EncodeBCD/DecodeBCD's field width. The widest field
// this family documents is SetFreq's 4 argument bytes; the ceiling is left
// generous rather than tight because DecodeBCD accumulates into a uint64,
// which nine bytes (18 digits) cannot overflow.
const maxBCDBytes = 9

// EncodeBCD packs v into n bytes of packed BCD, LEAST SIGNIFICANT decimal
// pair in byte 0. This is the ONE byte order every multi-byte field in
// this family uses for its write-side numeric arguments — SetFreq's own
// worked example is reproduced by it byte for byte (spec.md §Frame
// grammar and the FT-1000MP manual's own CONSTRUCTING AND SENDING CAT
// COMMANDS example: 14.25000 MHz, 1,425,000 tens-of-Hz, n=4, gives
// 00 50 42 01 — see bcd_test.go's fixture, which reproduces it directly).
// Unlike core/civ's mixed frequency/tone byte-order convention, there is
// no per-field axis to carry here: every documented multi-byte field in
// this family agrees on this one order.
func EncodeBCD(v uint64, n int) ([]byte, error) {
	if n < 1 || n > maxBCDBytes {
		return nil, fmt.Errorf("%w: field width %d is outside 1..%d", ErrBCD, n, maxBCDBytes)
	}
	out := make([]byte, n)
	rest := v
	for i := 0; i < n; i++ {
		pair := rest % 100
		rest /= 100
		out[i] = byte(pair/10)<<4 | byte(pair%10)
	}
	if rest != 0 {
		return nil, fmt.Errorf("%w: value %d needs more than the %d bytes (%d digits) of its field", ErrBCD, v, n, 2*n)
	}
	return out, nil
}

// DecodeBCD reverses EncodeBCD: b[0] holds the least significant decimal
// pair. Each byte is validated through civ.DecodeBCD2 — the single-byte
// packed-BCD decode this package reuses rather than reimplementing — so a
// nibble above 9 refuses the whole field instead of silently misreading
// it.
func DecodeBCD(b []byte) (uint64, error) {
	if len(b) < 1 || len(b) > maxBCDBytes {
		return 0, fmt.Errorf("%w: field of %d bytes is outside 1..%d", ErrBCD, len(b), maxBCDBytes)
	}
	var out uint64
	for i := len(b) - 1; i >= 0; i-- {
		pair, err := civ.DecodeBCD2(b[i])
		if err != nil {
			return 0, fmt.Errorf("%w (at byte %d of a %d-byte field): %v", ErrBCD, i, len(b), err)
		}
		out = out*100 + uint64(pair)
	}
	return out, nil
}
