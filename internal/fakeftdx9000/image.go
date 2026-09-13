// SPDX-License-Identifier: GPL-3.0-or-later

package fakeftdx9000

import "fmt"

// Image is a factory image: a function returning a freshly populated set of
// memory slots. EACH CALL MUST RETURN AN INDEPENDENT MAP, so multiple *Radio
// instances never share mutable slot state. Same contract as every sibling
// fake's Image.
type Image func() map[string]MemState

var _ Image = DefaultImage

// Mode nibble codes used by the fixtures below (matrix §1.5).
const (
	modeLSB = '1'
	modeUSB = '2'
)

// encodeFreqDigits converts hz to the 8-digit ASCII P2 field, refusing values
// that would need more than 8 digits — ONE NARROWER than every registered
// Yaesu sibling's 9 (matrix §2). It does NOT enforce this radio's
// MinFreqHz/MaxFreqHz range: that range is the driver's own assumed step from
// FA/FB (matrix §1.12/§1.13), a different question from what this fake,
// modelling the radio's wire grammar, accepts.
func encodeFreqDigits(hz uint64) (string, error) {
	if hz > 99_999_999 {
		return "", fmt.Errorf("fakeftdx9000: frequency %d Hz needs more than %d digits", hz, freqDigits)
	}
	return fmt.Sprintf("%0*d", freqDigits, hz), nil
}

// defaultState builds one unremarkable populated slot: the given frequency
// and mode, no clarifier offset, both clarifier flags off, CTCSS off, tone
// index 00, simplex, the answer kind.
//
// It panics on an invalid argument, deliberately: every call is a
// compile-time-known fixture constant in this package, so a bad one is a
// programming error in a test fixture.
func defaultState(freqHz uint64, mode byte) MemState {
	freq, err := encodeFreqDigits(freqHz)
	if err != nil {
		panic(err)
	}
	if !validModeByte(mode) {
		panic("fakeftdx9000: invalid mode byte in a fixture constant")
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
// half ('L' or 'U'), from the MC legend's own line: "100: P1L 101: P1U ...
// 116: P9L 117: P9U" (matrix §1.4). So pmsSlot(1, 'L') is "100" and
// pmsSlot(9, 'U') is "117" — DECIMAL CHANNEL NUMBERS continuing the memory
// range, never the token form "P1L".
func pmsSlot(pair int, half byte) string {
	n := pmsLo + (pair-1)*2
	if half == 'U' {
		n++
	}
	return fmt.Sprintf("%03d", n)
}

// DefaultImage is the image New uses when no WithFactoryImage option is
// given: two memory channels and the first and last PMS pairs, and nothing
// else — the same minimal-by-design shape every sibling fake's default image
// follows (doc.go's register entry THE DEFAULT IMAGE'S CONTENT IS INVENTED): a
// plain channel, populated so the fleet's read-every-registered-model pins
// are non-vacuous, and both ends of the PMS numbering so the seven pairs
// between them stay empty for the empty-slot and create-on-Set pins to name.
//
// THE CONTENT IS INVENTED. No FTdx9000's factory memory contents have been
// read by this project; these are placeholders.
func DefaultImage() map[string]MemState {
	slots := map[string]MemState{
		// 7.000000 MHz LSB — a placeholder, as internal/fakeft991a's own
		// equivalent channel is.
		"001": defaultState(7_000_000, modeLSB),
		// 14.250000 MHz USB.
		"002": defaultState(14_250_000, modeUSB),
	}

	type band struct {
		pair               int // 1-9
		lowerHz, upperHz   uint64
		lowerMod, upperMod byte
	}
	// Plausible IARU Region 1 amateur band edges — placeholders, not sourced
	// from any programmed radio. FIRST and LAST pairs only, deliberately: see
	// this function's doc comment.
	bands := []band{
		{1, 1_810_000, 2_000_000, modeLSB, modeLSB},   // P1L/P1U = 100/101, 160m
		{9, 28_000_000, 29_700_000, modeUSB, modeUSB}, // P9L/P9U = 116/117, 10m
	}
	for _, b := range bands {
		slots[pmsSlot(b.pair, 'L')] = defaultState(b.lowerHz, b.lowerMod)
		slots[pmsSlot(b.pair, 'U')] = defaultState(b.upperHz, b.upperMod)
	}
	return slots
}
