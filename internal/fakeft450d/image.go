// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft450d

import "fmt"

// Image is a factory image: a function returning a freshly populated set
// of memory slots. Each call must return an independent map, so that
// multiple *Radio instances never share mutable slot state
// (TestDefaultImage_EachCallIsIndependent pins it for DefaultImage).
type Image func() map[string]MemState

// The compile-time proof that this package's own image satisfies the
// contract callers are typed against.
var _ Image = DefaultImage

// Mode nibble codes used by the fixtures below, from the eleven-member
// legend (layout:794-797, state.go's Mode doc).
const (
	modeLSB = '1'
	modeUSB = '2'
)

// encodeFreqDigits converts hz to the 8-digit ASCII P2 field, refusing
// values that would need more than 8 digits (matrix §1.1).
func encodeFreqDigits(hz uint64) (string, error) {
	if hz > 99_999_999 {
		return "", fmt.Errorf("fakeft450d: frequency %d Hz needs more than 8 digits", hz)
	}
	return fmt.Sprintf("%08d", hz), nil
}

// validModeByte reports whether m is one of the eleven mode nibbles this
// radio's legend admits — a CLEAN HOLE at 'A', no 'D'/'E'/'F' member
// (doc.go, matrix §1.2).
func validModeByte(m byte) bool {
	switch {
	case m >= '1' && m <= '9':
		return true
	case m == 'B' || m == 'C':
		return true
	}
	return false
}

// defaultState builds one unremarkable populated slot: the given
// frequency and mode, no clarifier offset, both clarifier flags off,
// CTCSS off, tone index "00", simplex, the answer Kind.
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
		panic("fakeft450d: invalid mode byte in a fixture constant")
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

// pmsSlot returns a PMS slot's 3-byte wire form for a pair number (1-2)
// and a half ('L' or 'U') — MC's own legend, "501: P1L Channel 502: P1U
// Channel 503: P2L Channel 504: P2U Channel" (layout:716-719): DIRECT
// DECIMAL CHANNEL NUMBERS continuing the memory range, so pmsSlot(1, 'L')
// is "501" and pmsSlot(2, 'U') is "504".
func pmsSlot(pair int, half byte) string {
	n := pmsLo + (pair-1)*2
	if half == 'U' {
		n++
	}
	return fmt.Sprintf("%03d", n)
}

// DefaultImage is the image New uses when no WithFactoryImage option is
// given: two memory channels and one PMS pair — the first, 501/502 — and
// nothing else.
//
// MINIMAL BY DESIGN, mirroring fakeft950's own DefaultImage decision: at
// least one memory channel populated so fleet-wide read-every-registered-
// model pins are non-vacuous against this radio, at least one PMS pair
// populated so a test can exercise MR against it (and confirm MW still
// refuses it — doc.go's register entry 6), and nothing outside 001-504
// because this radio has none.
//
// THE PMS PAIR IS POPULATED DIRECTLY IN THIS MAP, NEVER VIA A SCRIPTED
// MW: that command refuses PMS slots outright (register entry 6), so this
// is the one route by which this fake's own PMS state can ever be
// non-default.
//
// doc.go's register entry THE DEFAULT IMAGE'S CONTENT IS INVENTED. No
// FT-450D has ever had its factory memory contents read by this project.
func DefaultImage() map[string]MemState {
	return map[string]MemState{
		// 7.000000 MHz LSB — the same placeholder frequency
		// fakeft950's own DefaultImage uses for its first channel.
		"001": defaultState(7_000_000, modeLSB),
		// 14.250000 MHz USB.
		"002": defaultState(14_250_000, modeUSB),
		// P1L/P1U, a plausible IARU Region 1 160 m band edge — a
		// placeholder, not sourced from any programmed radio.
		pmsSlot(1, 'L'): defaultState(1_810_000, modeLSB),
		pmsSlot(1, 'U'): defaultState(2_000_000, modeLSB),
	}
}
