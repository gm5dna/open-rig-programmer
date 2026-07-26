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

// ParseMode parses a single P6 wire byte into a Mode under THIS dialect's
// mode set. Membership is decided by the dialect's own table (ValidMode),
// never by a hardcoded byte range: a radio whose mode set differs must
// reject the bytes it does not know, and accept only its own. For FT710
// that table holds exactly '0'-'9' and 'A'-'F' (upper case only — that is
// all the radio ever emits), so this accepts exactly what the former
// package-level range check accepted.
//
// '0' parses successfully to ModeUnset (Mode's underlying byte IS the wire
// byte, so Mode('0') and ModeUnset are the same value); callers building a
// Set frame must separately reject ModeUnset if it is not a valid value to
// send.
//
// Anything the dialect does not know — including lower-case hex digits
// under FT710 — is rejected with a *ParseError. A zero Dialect knows no
// modes at all and therefore rejects every byte.
func (d Dialect) ParseMode(c byte) (Mode, error) {
	m := Mode(c)
	if !d.ValidMode(m) {
		return 0, newParseError([]byte{c}, "invalid mode code: want '0'-'9' or 'A'-'F'")
	}
	return m, nil
}

// ParseMode parses a single P6 wire byte into a Mode. It accepts exactly
// '0'-'9' and 'A'-'F' (upper case only — that is all the radio ever emits).
// '0' parses successfully to ModeUnset; callers building a Set frame must
// separately reject ModeUnset if it is not a valid value to send.
//
// Anything else, including lower-case hex digits, is rejected with a
// *ParseError.
//
// Migration scaffold: delegates to FT710; removed in Task 55.
func ParseMode(c byte) (Mode, error) {
	return FT710.ParseMode(c)
}

// Wire returns the single wire byte for m.
func (m Mode) Wire() byte {
	return byte(m)
}

// unknownModeName renders a Mode no dialect recognises. Used by
// Dialect.ModeName today; will be shared with Mode.String from Task 56,
// which is what stops the two from drifting apart. Mode.String keeps its
// own inline, byte-identical formatting until then — Task 56's to change,
// not this one's; see the M9b plan.
func unknownModeName(m Mode) string {
	return fmt.Sprintf("Mode(%#02x)", byte(m))
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
