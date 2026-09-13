// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft950

import "fmt"

// Image is a factory image: a function returning a freshly populated set of
// memory slots. Each call must return an independent map, so that multiple
// *Radio instances never share mutable slot state (TestDefaultImage_
// EachCallIsIndependent pins it for DefaultImage).
type Image func() map[string]MemState

// The compile-time proof that this package's own image satisfies the
// contract callers are typed against.
var _ Image = DefaultImage

// Mode nibble codes used by the fixtures below, from the twelve-member legend
// (layout:857/898, state.go's Mode doc).
const (
	modeLSB = '1'
	modeUSB = '2'
)

// encodeFreqDigits converts hz to the 8-digit ASCII P2 field, refusing values
// that would need more than 8 digits (matrix §1.2: this radio's P2 is one
// digit narrower than every other registered Yaesu dialect's).
func encodeFreqDigits(hz uint64) (string, error) {
	if hz > 99_999_999 {
		return "", fmt.Errorf("fakeft950: frequency %d Hz needs more than 8 digits", hz)
	}
	return fmt.Sprintf("%08d", hz), nil
}

// validModeByte reports whether m is one of the twelve mode nibbles this
// radio's legend admits — no placeholder, no hole (doc.go).
func validModeByte(m byte) bool {
	switch {
	case m >= '1' && m <= '9':
		return true
	case m >= 'A' && m <= 'C':
		return true
	}
	return false
}

// defaultState builds one unremarkable populated slot: the given frequency
// and mode, no clarifier offset, both clarifier flags off, CTCSS off, tone
// index "00", simplex, the answer Kind.
//
// It panics on an invalid argument, deliberately: every call is a
// compile-time-known fixture constant in this package, so a bad one is a
// programming error in a fixture, not a runtime condition a caller could act
// on.
func defaultState(freqHz uint64, mode byte) MemState {
	freq, err := encodeFreqDigits(freqHz)
	if err != nil {
		panic(err)
	}
	if !validModeByte(mode) {
		panic("fakeft950: invalid mode byte in a fixture constant")
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

// pmsSlot returns a PMS slot's 3-byte wire form for a pair number (1-9) and a
// half ('L' or 'U') — MC's own legend, "100: P1L 101: P1U ~ 116: P9L 117: P9U"
// (layout:795-802): DIRECT DECIMAL CHANNEL NUMBERS, so pmsSlot(1, 'L') is
// "100" and pmsSlot(9, 'U') is "117".
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
// outside 000-117 because this radio has none.
//
// THE FIRST CHANNEL IS "000", NOT "001" — this radio's own MemoryLo delta
// (doc.go, matrix §1.5): the lowest regular channel is a real, addressable
// slot here.
//
// THE CONTENT IS INVENTED (doc.go's register entry THE DEFAULT IMAGE'S
// CONTENT IS INVENTED). No FT-950 has ever had its factory memory contents
// read by this project.
func DefaultImage() map[string]MemState {
	return map[string]MemState{
		// 7.000000 MHz LSB — the same placeholder frequency
		// fakeft2000's own DefaultImage uses for its first channel.
		"000": defaultState(7_000_000, modeLSB),
		// 14.250000 MHz USB.
		"001": defaultState(14_250_000, modeUSB),
		// P1L/P1U, a plausible IARU Region 1 160 m band edge — a placeholder,
		// not sourced from any programmed radio.
		pmsSlot(1, 'L'): defaultState(1_810_000, modeLSB),
		pmsSlot(1, 'U'): defaultState(2_000_000, modeLSB),
	}
}
