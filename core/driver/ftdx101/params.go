// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx101

import "github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"

// modelName is the name the two siblings share where a message means the
// PACKAGE's radio rather than one variant of it — the settings surface,
// whose EX inventory is identical across the pair.
const modelName = "FTdx101"

// params is this PACKAGE's configuration of the bodies the Yaesu NEWCAT
// drivers share (core/driver/internal/yaesu). It carries what both
// siblings agree on; what they differ in — dialect, CAT ID, model name —
// is on modelD and modelMP, which are Params values of their own.
var params = yaesu.Params{
	Name:              "ftdx101",
	Model:             modelName,
	Dialect:           modelD.dialect,
	DescriptorVersion: settingsDescriptorVersion,
	// The manual charts this menu as P1 menus of P2 groups (the shared
	// nil default), and prints each item's position as the "P1-P2-P3"
	// triple this Display renders.
	Display: yaesu.DisplayP1P2P3,
}
