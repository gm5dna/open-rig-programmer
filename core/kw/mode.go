// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import "fmt"

// Mode is the memory record's P5 byte: a single ASCII digit naming the
// channel's operating mode. The underlying byte IS the wire byte, so
// Mode(c) round-trips through Wire() unchanged.
//
// THE MEMORY MODE VOCABULARY IS MD'S. Neither book prints a legend against
// MR/MW P5; both say "refer to the MD command" (590:1544-1545, 480:959), so
// the ten nibbles below are MD's own (590:1353-1363, 480:843-854).
type Mode byte

// The ten MD nibbles both books print, at the values they print them.
//
// TWO OF THE TEN NAME NO MODE. ModeNone is "None (setting failure)" on the
// 590 pair (590:1353) and "No mode (Not used for the TS-480)" on the 480
// (480:843); ModeTune is "None (setting failure)" (590:1362) and "Tune (Not
// used for the TS-480)" (480:853). They are declared here because they are
// bytes this codec must recognise on the wire — ModeNone is the empty
// channel's own mode byte on the 590 pair (A18a) — and they are refused by
// NewLayout as legend entries and by the MW builder as record content
// (A18b). They are not interchangeable: the 480 gives them different
// meanings, which is why A18b's lift trials both.
const (
	ModeNone Mode = '0'
	ModeLSB  Mode = '1'
	ModeUSB  Mode = '2'
	ModeCW   Mode = '3'
	ModeFM   Mode = '4'
	ModeAM   Mode = '5'
	ModeFSK  Mode = '6'
	ModeCWR  Mode = '7'
	ModeTune Mode = '8'
	ModeFSKR Mode = '9'
)

// Wire returns m's single wire byte.
func (m Mode) Wire() byte { return byte(m) }

// String renders m for logs and test failures: the nibble in the form the
// books print it, never a display name.
//
// IT IS DELIBERATELY NOT A NAME. A display name is per-LAYOUT — the 480
// prints "CWR" and "FSR" where the 590 pair print "CW-R" and "FSK-R"
// (erratum E12, doc.go) — and a bare Mode carries no layout, so a String
// that returned a name would have to pick one radio's book and would then
// be wrong on the other. Layout.ModeName is the naming path.
func (m Mode) String() string { return fmt.Sprintf("Mode(%q)", byte(m)) }

// namesAMode reports whether m is one of the eight nibbles that name a mode
// a channel can be in — every documented nibble except ModeNone and
// ModeTune.
func (m Mode) namesAMode() bool {
	switch m {
	case ModeLSB, ModeUSB, ModeCW, ModeFM, ModeAM, ModeFSK, ModeCWR, ModeFSKR:
		return true
	default:
		return false
	}
}

// documentedNibble reports whether m is one of the ten bytes MD prints —
// including the two that name no mode.
func (m Mode) documentedNibble() bool {
	return m.namesAMode() || m == ModeNone || m == ModeTune
}

// ModeName returns the display name THIS LAYOUT publishes for m, and
// whether it publishes one at all.
//
// The names are the programme's own consistent spellings on every row. The
// 480's book prints "CWR (CW Reverse)" at 480:852 and "FSR (FSK Reverse)"
// at 480:854 where the 590 pair print "CW-R" and "FSK-R" (590:1361,
// 590:1363); the printed forms are recorded as erratum E12 in doc.go, and
// what a layout publishes is the project's spelling, exactly as the FT-891's
// mode legend was treated.
//
// A zero Layout publishes no name for anything.
func (l Layout) ModeName(m Mode) (string, bool) {
	name, ok := l.modeNames[m]
	return name, ok
}

// ParseMode resolves a P5 wire byte against THIS LAYOUT'S OWN legend,
// returning the mode and whether the layout knows it.
//
// MEMBERSHIP, AGAINST THE RECEIVER — never a hardcoded byte range. The two
// legends happen to cover the same eight nibbles, and a range check would
// pass every test in this tree while the per-layout seam was fiction.
//
// ModeNone and ModeTune are NOT in any legend and so are refused here. The
// record parser does not route the empty channel's mode byte through this
// method for exactly that reason: A18a's rule is "P4-P15 all zero", tested
// before any field is interpreted.
func (l Layout) ParseMode(c byte) (Mode, bool) {
	m := Mode(c)
	if _, ok := l.modeNames[m]; !ok {
		return 0, false
	}
	return m, true
}
