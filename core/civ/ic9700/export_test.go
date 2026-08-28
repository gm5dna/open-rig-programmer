// SPDX-License-Identifier: GPL-3.0-or-later

package ic9700

// Test-only aliases. The _test.go suffix keeps them out of every non-test
// build, so an external ic9700_test package may call
// ic9700.FixedTemplateForTest while the production API has no such
// symbol — and nothing here widens what this package exports.
//
// It lives HERE, beside the identifier it aliases: a package cannot alias
// another package's unexported symbol, so the driver's own export_test.go
// could not have carried this one.
var FixedTemplateForTest = fixedTemplate
