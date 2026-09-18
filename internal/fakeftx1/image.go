// SPDX-License-Identifier: GPL-3.0-or-later

package fakeftx1

import "fmt"

// Image is a factory image: a function returning a freshly populated pair
// of maps — MR/MW's memory-block state and MT's tag state, kept
// independent (doc.go register entry 5). Each call must return
// independent maps, so that multiple *Radio instances never share
// mutable state (TestDefaultImage_EachCallIsIndependent pins it for
// DefaultImage).
type Image func() (slots map[string]MemState, tags map[string]string)

// The compile-time proof that this package's own image satisfies the
// contract callers are typed against.
var _ Image = DefaultImage

// Mode nibble codes used by the fixtures below (spec.md §5, state.go's
// Mode doc).
const (
	modeLSB = '1'
	modeUSB = '2'
)

// Kind bytes used by the fixtures below (spec.md §3.1 row P7).
const (
	kindMemory = '1'
	kindPMS    = '5'
)

// encodeFreqDigits converts hz to the 9-digit ASCII P2 field, refusing
// values that would need more than 9 digits (spec.md §3.1).
func encodeFreqDigits(hz uint64) (string, error) {
	if hz > 999_999_999 {
		return "", fmt.Errorf("fakeftx1: frequency %d Hz needs more than 9 digits", hz)
	}
	return fmt.Sprintf("%09d", hz), nil
}

// defaultState builds one unremarkable populated slot: the given
// frequency, mode and kind, no clarifier offset, both clarifier flags
// off, tone OFF, simplex.
//
// It panics on an invalid argument, deliberately: every call is a
// compile-time-known fixture constant in this package, so a bad one is a
// programming error in a fixture, not a runtime condition a caller could
// act on.
func defaultState(freqHz uint64, mode, kind byte) MemState {
	freq, err := encodeFreqDigits(freqHz)
	if err != nil {
		panic(err)
	}
	if !validModeByte(mode) {
		panic("fakeftx1: invalid mode byte in a fixture constant")
	}
	if !validKindByte(kind) {
		panic("fakeftx1: invalid kind byte in a fixture constant")
	}
	return MemState{
		Freq:     freq,
		ClarSign: '+',
		ClarMag:  "0000",
		RXClar:   false,
		TXClar:   false,
		Mode:     mode,
		Kind:     kind,
		Tone:     '0',
		Shift:    '0',
	}
}

// padTag right-pads s with spaces to the fixed 12-byte MT tag width
// (doc.go register entry 8; f1-report.md item 3, TagFill). Panics if s is
// already longer than that — a fixture bug, not a runtime condition.
func padTag(s string) string {
	if len(s) > tagWireLen {
		panic("fakeftx1: tag fixture longer than the 12-byte wire field")
	}
	for len(s) < tagWireLen {
		s += " "
	}
	return s
}

// DefaultImage is the image New uses when no WithFactoryImage option is
// given: a few channels across EVERY bank FTX-1's slot space has (spec.md
// §4) — two plain memory channels, one PMS pair (both halves), one 5 MHz
// channel, and the emergency channel — plus tags on a couple of them, so
// fleet-wide read-every-registered-model pins and this milestone's BI
// legs are non-vacuous against every bank.
//
// THE CONTENT IS INVENTED — doc.go's register entry THE DEFAULT IMAGE'S
// CONTENT IS INVENTED. No FTX-1 has ever been read by this project.
func DefaultImage() (map[string]MemState, map[string]string) {
	slots := map[string]MemState{
		// 7.000000 MHz LSB — the same placeholder frequency
		// internal/fakeradio's and internal/fakeftdx3000's own
		// DefaultImages use for their first channel.
		"00001": defaultState(7_000_000, modeLSB, kindMemory),
		// 14.250000 MHz USB.
		"00002": defaultState(14_250_000, modeUSB, kindMemory),
		// PMS pair 1, both halves — a plausible IARU Region 1 160 m band
		// edge, invented, not sourced from any programmed radio.
		pmsWire(1, 'L'): defaultState(1_810_000, modeLSB, kindPMS),
		pmsWire(1, 'U'): defaultState(2_000_000, modeLSB, kindPMS),
		// 5.3305 MHz, the lowest of the UK 5 MHz channels — invented
		// placeholder, kind ASSUMED Memory (doc.go register entry 3: no
		// dedicated P7 legend value exists for this bank).
		"50001": defaultState(5_330_500, modeUSB, kindMemory),
		// EMGCH — the fixed Alaska-style emergency channel form (spec.md
		// §4), kind ASSUMED Memory for the same reason as the 5 MHz
		// channel above.
		emgWire: defaultState(5_167_500, modeUSB, kindMemory),
	}
	tags := map[string]string{
		"00001":         padTag("HOME"),
		pmsWire(1, 'L'): padTag("160M-EDGE"),
	}
	return slots, tags
}
