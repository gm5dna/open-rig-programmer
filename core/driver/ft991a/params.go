// SPDX-License-Identifier: GPL-3.0-or-later

package ft991a

import (
	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"
)

// params is this radio's configuration of the bodies the Yaesu NEWCAT
// drivers share (core/driver/internal/yaesu): what differs between these
// radios lives here, and the body lives there once.
//
// ReadGap is left nil here and armed only by settings_test.go's
// concurrency pin.
var params = yaesu.Params{
	Name:              "ft991a",
	Model:             modelName,
	Dialect:           catDialect,
	DescriptorVersion: settingsDescriptorVersion,
	// This manual prints ONE FLAT MENU LIST with no chart hierarchy at
	// all, so the descriptor holds a single menu and group rather than a
	// partition the manual does not have, and an item displays as its
	// wire address (the nil Display default).
	Group: yaesu.GroupFlat,

	// The combined MT record encodes five CTCSS states — the three the
	// other radios here carry plus two DCS pairs — in the manual's own
	// legend order, which is the order the refusal text names them in.
	CTCSS: []yaesu.CTCSSName{
		{Name: "OFF", State: cat.CTCSSOff},
		{Name: "ENC-DEC", State: cat.CTCSSEncDec},
		{Name: "ENC", State: cat.CTCSSEnc},
		{Name: "DCS-ENC-DEC", State: cat.CTCSSDCSEncDec},
		{Name: "DCS-ENC", State: cat.CTCSSDCSEnc},
	},
	EraseReason: "this radio's CAT command set contains no erase command at all (layout 123-196), so this codec cannot express an erase, and FieldErase is not write-Supported",
	// cat.ModeUnset is the parser's placeholder for a mode byte this
	// dialect reads but cannot name; it must never reach a Set frame.
	RefuseModeUnset: "mode %q is the parse-only placeholder cat.ModeUnset and must not be ModeUnset in a Set frame",
	// The only radio here whose storable range is narrower than the
	// codec's, so the only one that checks the frequency against its own
	// declared capabilities.
	CheckFreqRange: true,
	BuildMT: func(d cat.Dialect, m cat.MemoryData, tag string, _ bool) (cat.Command, error) {
		return d.BuildMTSetCombined(m, tag)
	},

	// NoProbe, left at the zero value: this manual declares no 5xx or
	// EMG slot, so Open has nothing to discover and sends no sweep.
}
