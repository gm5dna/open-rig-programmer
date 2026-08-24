// SPDX-License-Identifier: GPL-3.0-or-later

package ic905

// Model is the display label and the CI-V profile's own name. PDF p.1,
// cover panel, model mark IC-905.
const Model = "IC-905"

// RadioAddress is the transceiver's default CI-V address. PDF p.3
// (folio 2), "About the data format", "Controller (PC) to IC-905"
// frame, cell (2), printed AC above the label "Transceiver's default
// address". MANUAL-EVIDENCED (matrix section 3.4).
//
// It is a USER SETTING on this radio, and this tier ships no
// --civ-address flag (spec D3.3): an IC-905 moved off AC is
// unreachable, and the probe times out rather than mis-identifying
// anything.
const RadioAddress = byte(0xAC)

// nameCharset is every byte PDF p.20 (folio 19), "Codes for character
// entries", prints for the memory name field, plus 0x20.
//
// 0x20 IS ASSUMED. Neither printed table contains a space, and the
// field is sixteen characters fixed, so a shorter name needs a
// character the tables do not print. 0x20 is the only value this
// document ever prints for a space, in three tables that all govern
// OTHER fields (PDF pp.18, 21 and 24). Register: D5 entry 3. Lift:
// ic905-R-16.
const nameCharset = "" +
	"0123456789" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
	"abcdefghijklmnopqrstuvwxyz" +
	"!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~" +
	" "
