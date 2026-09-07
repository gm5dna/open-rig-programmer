// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx10

import "github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"

// params is this radio's configuration of the bodies the Yaesu NEWCAT
// drivers share (core/driver/internal/yaesu): what differs between these
// radios lives here, and the body lives there once.
var params = yaesu.Params{
	Name:              "ftdx10",
	Model:             modelName,
	Dialect:           catDialect,
	CATID:             catID,
	DescriptorVersion: settingsDescriptorVersion,
	// The manual charts this menu as P1 menus of P2 groups (the shared
	// nil default), and prints each item's position as the "P1-P2-P3"
	// triple this Display renders.
	Display: yaesu.DisplayP1P2P3,
}
