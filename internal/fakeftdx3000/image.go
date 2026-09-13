// SPDX-License-Identifier: GPL-3.0-or-later

package fakeftdx3000

import "fmt"

// Image is a factory image: a function returning a freshly populated set
// of memory slots. Each call must return an independent map, so that
// multiple *Radio instances never share mutable slot state
// (TestDefaultImage_EachCallIsIndependent pins it for DefaultImage).
type Image func() map[string]MemState

// The compile-time proof that this package's own image satisfies the
// contract callers are typed against.
var _ Image = DefaultImage

// Mode nibble codes used by the fixtures below, from the twelve-member
// legend (layout:950-952, state.go's Mode doc).
const (
	modeLSB = '1'
	modeUSB = '2'
)

// encodeFreqDigits converts hz to the 8-digit ASCII P2 field, refusing
// values that would need more than 8 digits (matrix §1.1).
func encodeFreqDigits(hz uint64) (string, error) {
	if hz > 99_999_999 {
		return "", fmt.Errorf("fakeftdx3000: frequency %d Hz needs more than 8 digits", hz)
	}
	return fmt.Sprintf("%08d", hz), nil
}

// validModeByte reports whether m is one of the twelve mode nibbles the
// MW/MR legend admits — no 'D' (AM-N) member, matrix §1.2.
func validModeByte(m byte) bool {
	switch {
	case m >= '1' && m <= '9':
		return true
	case m >= 'A' && m <= 'C':
		return true
	}
	return false
}

// defaultState builds one unremarkable populated slot: the given
// frequency and mode, no clarifier offset, both clarifier flags off,
// CTCSS off, tone index "00" (the only value an accepted Set can ever
// leave, doc.go register entry 3), simplex, the answer Kind.
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
		panic("fakeftdx3000: invalid mode byte in a fixture constant")
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
		Tone:     "00",
		Shift:    '0',
	}
}

// pmsSlot returns a PMS slot's 3-byte wire form for a pair number (1-9)
// and a half ('L' or 'U') — MC's own legend, "100: P-1L 101: P-1U ~
// 116: P-9L 117: P-9U" (layout:879-883): DIRECT DECIMAL CHANNEL NUMBERS,
// so pmsSlot(1, 'L') is "100" and pmsSlot(9, 'U') is "117".
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
// MINIMAL BY DESIGN, mirroring internal/fakeft2000's own DefaultImage
// decision: at least one memory channel populated so fleet-wide
// read-every-registered-model pins are non-vacuous against this radio, at
// least one PMS pair populated, and nothing outside 001-117 because this
// radio has none.
//
// Channel "002" carries a NONZERO tone index — this radio's own novelty
// (doc.go register entry 3): the only way this fake's P9 can ever answer
// live is through a fixture or a WithSlot option, since an accepted MW
// Set always leaves "00". Exercising that path from New()'s own default
// image, rather than only from a test-constructed option, is deliberate:
// the fleet-wide "read every registered model" pins (internal/wiring and
// similar) drive DefaultImage() alone, and would otherwise never touch
// this radio's headline wrinkle at all.
//
// THE CONTENT IS INVENTED (doc.go's register entry THE DEFAULT IMAGE'S
// CONTENT IS INVENTED). No FTDX3000 has been read by this project.
func DefaultImage() map[string]MemState {
	ch002 := defaultState(14_250_000, modeUSB)
	ch002.CTCSS = '1'
	ch002.Tone = "05"
	return map[string]MemState{
		// 7.000000 MHz LSB — the same placeholder frequency
		// internal/fakeft2000's own DefaultImage uses for its first
		// channel.
		"001": defaultState(7_000_000, modeLSB),
		// 14.250000 MHz USB, CTCSS ENC/DEC, tone index 05 — invented, to
		// exercise the live-read/fixed-write P9 asymmetry (see above).
		"002": ch002,
		// P-1L/P-1U, a plausible IARU Region 1 160 m band edge — a
		// placeholder, not sourced from any programmed radio.
		pmsSlot(1, 'L'): defaultState(1_810_000, modeLSB),
		pmsSlot(1, 'U'): defaultState(2_000_000, modeLSB),
	}
}
