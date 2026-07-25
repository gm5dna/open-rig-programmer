// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import "fmt"

// Mode is the CAT P6 mode nibble: a single upper-case ASCII character
// ('0'-'9', 'A'-'F') identifying the operating mode of a memory channel or
// VFO. The underlying byte value IS the wire byte, so Mode(c) for any valid
// c round-trips through Wire() unchanged.
//
// Reference: "Mode nibble (P6) — single ASCII char, emit upper case".
type Mode byte

// Mode constants for the 15 named modes in the reference table, plus
// ModeUnset for the '0' = "-" row documented immediately below that table.
//
// ModeUnset only ever appears in reads of odd radio states. Builders
// elsewhere in this codec must reject ModeUnset when constructing Set
// frames; parsers must accept it.
const (
	ModeUnset Mode = '0' // "-" (unset)

	ModeLSB   Mode = '1' // LSB
	ModeUSB   Mode = '2' // USB
	ModeCWU   Mode = '3' // CW-U
	ModeFM    Mode = '4' // FM
	ModeAM    Mode = '5' // AM
	ModeRTTYL Mode = '6' // RTTY-L
	ModeCWL   Mode = '7' // CW-L
	ModeDATAL Mode = '8' // DATA-L
	ModeRTTYU Mode = '9' // RTTY-U

	ModeDATAFM  Mode = 'A' // DATA-FM
	ModeFMN     Mode = 'B' // FM-N
	ModeDATAU   Mode = 'C' // DATA-U
	ModeAMN     Mode = 'D' // AM-N
	ModePSK     Mode = 'E' // PSK
	ModeDATAFMN Mode = 'F' // DATA-FM-N
)

// modeNames maps every valid Mode to its display name from the reference
// table (and the '0' = "-" row below it).
var modeNames = map[Mode]string{
	ModeUnset:   "-",
	ModeLSB:     "LSB",
	ModeUSB:     "USB",
	ModeCWU:     "CW-U",
	ModeFM:      "FM",
	ModeAM:      "AM",
	ModeRTTYL:   "RTTY-L",
	ModeCWL:     "CW-L",
	ModeDATAL:   "DATA-L",
	ModeRTTYU:   "RTTY-U",
	ModeDATAFM:  "DATA-FM",
	ModeFMN:     "FM-N",
	ModeDATAU:   "DATA-U",
	ModeAMN:     "AM-N",
	ModePSK:     "PSK",
	ModeDATAFMN: "DATA-FM-N",
}

// ParseMode parses a single P6 wire byte into a Mode. It accepts exactly
// '0'-'9' and 'A'-'F' (upper case only — that is all the radio ever emits).
// '0' parses successfully to ModeUnset; callers building a Set frame must
// separately reject ModeUnset if it is not a valid value to send.
//
// Anything else, including lower-case hex digits, is rejected with a
// *ParseError.
func ParseMode(c byte) (Mode, error) {
	switch {
	case c == '0':
		return ModeUnset, nil
	case c >= '1' && c <= '9':
		return Mode(c), nil
	case c >= 'A' && c <= 'F':
		return Mode(c), nil
	default:
		return 0, newParseError([]byte{c}, "invalid mode code: want '0'-'9' or 'A'-'F'")
	}
}

// Wire returns the single wire byte for m.
func (m Mode) Wire() byte {
	return byte(m)
}

// String returns the reference table's display name for m (e.g. "LSB",
// "DATA-FM-N"), or "-" for ModeUnset. Modes constructed by an invalid cast
// rather than ParseMode return a diagnostic placeholder.
func (m Mode) String() string {
	if name, ok := modeNames[m]; ok {
		return name
	}
	return fmt.Sprintf("Mode(%#02x)", byte(m))
}
