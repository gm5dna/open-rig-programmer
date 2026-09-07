// SPDX-License-Identifier: GPL-3.0-or-later

package ft891

import (
	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"
)

// params is this radio's configuration of the bodies the Yaesu NEWCAT
// drivers share (core/driver/internal/yaesu): what differs between these
// radios lives here, and the body lives there once.
var params = yaesu.Params{
	Name:              "ft891",
	Model:             modelName,
	Dialect:           catDialect,
	CATID:             catID,
	DescriptorVersion: settingsDescriptorVersion,
	// This manual charts its menu by MENU NUMBER ALONE — no second
	// level, and no printed P1-P2-P3 form — so each P1 is one menu
	// holding one group, and an item displays as its wire address (the
	// nil Display default).
	Group: yaesu.GroupByP1,

	// The combined MT record encodes three CTCSS states, in the manual's
	// own legend order — which is the order the refusal text names them
	// in.
	CTCSS: []yaesu.CTCSSName{
		{Name: "OFF", State: cat.CTCSSOff},
		{Name: "ENC-DEC", State: cat.CTCSSEncDec},
		{Name: "ENC", State: cat.CTCSSEnc},
	},
	EraseReason: "this radio's CAT command set contains no erase command at all (layout 111-147), so this codec cannot express an erase, and FieldErase is not write-Supported",
	// This radio's P11 (byte 28) is a LIVE TAG flag with no "leave it
	// alone" encoding, so a write may only ever send a Known value; the
	// format verbs take the state found and the state required.
	RefuseTagDisplayUnknown: "tag display FieldState is %q, not %q; this radio's P11 (byte 28) is a LIVE TAG flag with no \"leave it alone\" encoding, so only a Known value is ever sent",
	RefuseTxClar:            "this radio's P5 (byte 21) legend prints \"0: (Fixed)\" on every block that carries the field grid, so it has no TX clarifier flag to set; the offset and the RX flag are writable and only the TX half is refused, because all three are one spec.Field",
	BuildMT: func(d cat.Dialect, m cat.MemoryData, tag string, display bool) (cat.Command, error) {
		return d.BuildMTSetCombinedDisplay(m, tag, display)
	},
}
