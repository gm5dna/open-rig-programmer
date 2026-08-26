// SPDX-License-Identifier: GPL-3.0-or-later

package ic9700

// Test-only aliases for this package's unexported slot pair. The _test.go
// suffix keeps them out of every non-test build, so an external
// ic9700_test package may call ic9700.SlotAddress while the production API
// has no such symbol — and nothing here widens what this package exports.
//
// THERE ARE TWO export_test.go FILES IN THIS TIER, one per package, and
// they are not interchangeable: fixedTemplate belongs to core/civ/ic9700
// and its alias lives in THAT package's own export_test.go, because a
// package cannot alias another package's unexported identifier.
var (
	SlotAddress = slotAddress
	AddressSlot = addressSlot
)
