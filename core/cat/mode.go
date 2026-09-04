// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"fmt"
	"sort"
	"strings"
)

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
		return 0, newParseError([]byte{c}, "invalid mode code: want "+d.modeDomainText())
	}
	return m, nil
}

// modeDomainText renders THIS DIALECT'S OWN accepted mode bytes as the
// compact range list a refusal quotes.
//
// IT IS DERIVED, NOT WRITTEN DOWN, and that is the whole of the change. The
// refusal above read "want '0'-'9' or 'A'-'F'" on every dialect — the
// FT-710's contiguous nibble space, stated as though it were the protocol's.
// It is not: the FT-891's mode legend has a HOLE at 'A' and no 'E' or 'F',
// so that sentence is simply false there, and a user told to send a byte the
// radio rejects has been misdirected by this program rather than by the
// radio.
//
// The FT-710's own table is exactly '0'-'9' and 'A'-'F', which this renders
// as "'0'-'9' or 'A'-'F'" — the same string, byte for byte, which is why
// core/cat/testdata/parser-corpus.golden's three ParseMode rows do not move.
// That coincidence is the POINT rather than a lucky accident: a message
// derived from the dialect says the true thing on every radio and happens to
// say the old thing on this one.
//
// A dialect with no modes at all (only the zero Dialect, which NewDialect
// cannot produce) gets a sentence rather than an empty list, because "want "
// followed by nothing tells a reader nothing.
func (d Dialect) modeDomainText() string {
	keys := make([]int, 0, len(d.modeNames))
	for m := range d.modeNames {
		keys = append(keys, int(byte(m)))
	}
	if len(keys) == 0 {
		return "a mode this dialect declares, but it declares none"
	}
	sort.Ints(keys)

	// Collapse consecutive byte values into ranges, so a contiguous table
	// reads as a range and a table with holes shows them.
	var parts []string
	for i := 0; i < len(keys); {
		j := i
		for j+1 < len(keys) && keys[j+1] == keys[j]+1 {
			j++
		}
		lo, hi := rune(byte(keys[i])), rune(byte(keys[j]))
		if lo == hi {
			parts = append(parts, fmt.Sprintf("%q", lo))
		} else {
			parts = append(parts, fmt.Sprintf("%q-%q", lo, hi))
		}
		i = j + 1
	}
	switch len(parts) {
	case 1:
		return parts[0]
	case 2:
		return parts[0] + " or " + parts[1]
	default:
		return strings.Join(parts[:len(parts)-1], ", ") + " or " + parts[len(parts)-1]
	}
}

// Wire returns the single wire byte for m.
func (m Mode) Wire() byte {
	return byte(m)
}

// unknownModeName renders a Mode no dialect recognises. Shared by
// Dialect.ModeName and Mode.String (Task 56), which is what stops the two
// from drifting apart on the one string they must still agree about.
func unknownModeName(m Mode) string {
	return fmt.Sprintf("Mode(%#02x)", byte(m))
}

// String is a DIAGNOSTIC FALLBACK, not a rendering path: it reads the
// FT-710's mode table (the package-level modeNames) whatever radio the
// Mode came from, because a bare Mode carries no dialect and fmt.Stringer
// gives it nowhere to receive one. Use it for logs, test failure messages
// and %v of a Mode in isolation.
//
// ANYTHING USER-VISIBLE MUST GO THROUGH Dialect.ModeName INSTEAD. The
// driver does: core/driver/ft710's ReadChannel renders a channel's mode
// with s.dialect.ModeName(m.Mode), so what reaches a codeplug, the CLI
// listing and the GUI grid is the mode table of the radio that answered.
// The two agree exactly for the FT-710 — same table, same unknown-mode
// formatting via unknownModeName — so this distinction costs nothing today
// and is the whole point the moment a second dialect exists.
//
// Returns the reference table's display name for m (e.g. "LSB",
// "DATA-FM-N"), or "-" for ModeUnset. A Mode constructed by an invalid
// cast rather than ParseMode returns unknownModeName's placeholder.
func (m Mode) String() string {
	if name, ok := modeNames[m]; ok {
		return name
	}
	return unknownModeName(m)
}
