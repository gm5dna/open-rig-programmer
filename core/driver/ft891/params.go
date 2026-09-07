// SPDX-License-Identifier: GPL-3.0-or-later

package ft891

import "github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"

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
}
