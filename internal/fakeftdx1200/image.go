// SPDX-License-Identifier: GPL-3.0-or-later

package fakeftdx1200

import "fmt"

// Image is a factory image: a function returning a freshly populated set of
// memory slots. Each call must return an independent map, so that multiple
// *Radio instances never share mutable slot state.
type Image func() map[string]MemState

// The compile-time proof that this package's own image satisfies the
// contract callers are typed against.
var _ Image = DefaultImage

// Mode nibble codes used by the fixtures below, from the eleven-member
// legend with a hole at 'A' (matrix §1.2, state.go's Mode doc).
const (
	modeLSB = '1'
	modeUSB = '2'
)

// encodeFreqDigits converts hz to the 8-digit ASCII P2 field, refusing
// values that would need more than 8 digits (matrix §1.1).
func encodeFreqDigits(hz uint64) (string, error) {
	if hz > 99_999_999 {
		return "", fmt.Errorf("fakeftdx1200: frequency %d Hz needs more than 8 digits", hz)
	}
	return fmt.Sprintf("%08d", hz), nil
}

// validModeByte reports whether m is one of the eleven mode nibbles this
// radio's legend admits: '1'-'9', 'B', 'C'. 'A' is a genuine hole — printed
// "----" on every command that carries this field (matrix §1.2) — and is
// therefore never valid, and there is no 'D' on this radio at all.
func validModeByte(m byte) bool {
	switch {
	case m >= '1' && m <= '9':
		return true
	case m == 'B' || m == 'C':
		return true
	}
	return false
}

// defaultState builds one unremarkable populated slot: the given frequency
// and mode, no clarifier offset, both clarifier flags off, CTCSS off,
// simplex, the answer Kind. There is no Tone parameter: P9 is
// printed-fixed "00" on every command that carries it (matrix §1.3,
// doc.go's register entry P9 (TONE) IS PRINTED-FIXED) and is never stored.
//
// It panics on an invalid argument, deliberately: every call is a
// compile-time-known fixture constant in this package, so a bad one is a
// programming error in a fixture, not a runtime condition a caller could
// act on.
func defaultState(freqHz uint64, mode byte) MemState {
	freq, err := encodeFreqDigits(freqHz)
	if err != nil {
		panic(err)
	}
	if !validModeByte(mode) {
		panic("fakeftdx1200: invalid mode byte in a fixture constant")
	}
	return MemState{
		Freq:     freq,
		ClarSign: '+',
		ClarMag:  "0000",
		RXClar:   false,
		TXClar:   false,
		Mode:     mode,
		Kind:     kindMemory,
		CTCSS:    '0',
		Shift:    '0',
	}
}

// pmsSlot returns a PMS slot's 3-byte wire form for a pair number (1-9) and
// a half ('L' or 'U') — the MC legend's own decomposition (matrix §2.4):
// "100: P-1L / 101: P-1U / ~ / 116: P-9L / 117: P-9U", direct decimal
// channel numbers, so pmsSlot(1, 'L') is "100" and pmsSlot(9, 'U') is "117".
func pmsSlot(pair int, half byte) string {
	n := pmsLo + (pair-1)*2
	if half == 'U' {
		n++
	}
	return fmt.Sprintf("%03d", n)
}

// DefaultImage is the image New uses when no WithFactoryImage option is
// given: two memory channels and one PMS pair — the first, 100/101 — and
// nothing else.
//
// MINIMAL BY DESIGN, mirroring fakeft2000's own DefaultImage decision: at
// least one memory channel populated so fleet-wide read-every-registered-
// model pins are non-vacuous against this radio, at least one PMS pair
// populated so the direct-decimal PMS numbering is exercised, and nothing
// outside 001-117 because this radio has none (doc.go's register entry
// CHANNEL "000" IS OUT OF SCOPE).
//
// THE CONTENT IS INVENTED (doc.go's register entry THE DEFAULT IMAGE'S
// CONTENT IS INVENTED). No FTdx1200's
// factory memory contents have been read by this project.
func DefaultImage() map[string]MemState {
	return map[string]MemState{
		// 7.000000 MHz LSB — the same placeholder frequency fakeft2000's
		// own DefaultImage uses for its first channel.
		"001": defaultState(7_000_000, modeLSB),
		// 14.250000 MHz USB.
		"002": defaultState(14_250_000, modeUSB),
		// P-1L/P-1U, a plausible IARU Region 1 160 m band edge — a
		// placeholder, not sourced from any programmed radio.
		pmsSlot(1, 'L'): defaultState(1_810_000, modeLSB),
		pmsSlot(1, 'U'): defaultState(2_000_000, modeLSB),
	}
}
