// SPDX-License-Identifier: GPL-3.0-or-later

package ft991a

import "github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"

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
	CATID:             catID,
	DescriptorVersion: settingsDescriptorVersion,
	// This manual prints ONE FLAT MENU LIST with no chart hierarchy at
	// all, so the descriptor holds a single menu and group rather than a
	// partition the manual does not have, and an item displays as its
	// wire address (the nil Display default).
	Group: yaesu.GroupFlat,
}
