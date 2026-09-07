// SPDX-License-Identifier: GPL-3.0-or-later

package ic7100

// WithSiblingRecordLengths supplies the tier-integration attribution table.
// It changes only diagnostic attribution; it never changes accepted
// lengths. It has no production caller today — the IC-7100 matrix declares
// no registered sibling, and tier integration owns any later cross-model
// attribution — so it lives here, in the test binary only, until that
// integration gives it one. probe_test.go and e2e_test.go's synthetic
// tables are what currently exercise the branch it feeds.
func WithSiblingRecordLengths(lengths SiblingLengths) Option {
	return func(d *ic7100Driver) {
		d.siblingLengths = make(SiblingLengths, len(lengths))
		for length, model := range lengths {
			d.siblingLengths[length] = model
		}
	}
}
