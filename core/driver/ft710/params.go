// SPDX-License-Identifier: GPL-3.0-or-later

package ft710

import "github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"

// params is this radio's configuration of the bodies the Yaesu NEWCAT
// drivers share (core/driver/internal/yaesu): what differs between these
// radios lives here, and the body lives there once.
//
// The FT-710 shares the settings surface only. Its memory write is two
// frames (MW then MT) where every other Yaesu here writes one, so
// write.go stays this package's own.
var params = yaesu.Params{
	Name:              "ft710",
	Model:             modelName,
	Dialect:           catDialect,
	CATID:             catID,
	DescriptorVersion: settingsDescriptorVersion,
	// The manual charts this menu as P1 menus of P2 groups (the shared
	// nil default), and prints each item's position as the "P1-P2-P3"
	// triple this Display renders.
	Display: yaesu.DisplayP1P2P3,
}
