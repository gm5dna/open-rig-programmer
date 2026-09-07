// SPDX-License-Identifier: GPL-3.0-or-later

package ic905

// WithSiblingRecordLengths supplies the SiblingLengths table this driver
// attributes a foreign record length against. It has no production caller
// today — Wave 4 wires the registry's own table in the same commit that
// registers the tier's models and runs the cross-model distinctness check
// — so it lives here, in the test binary only, until that commit gives it
// one. TestProbe_TheSiblingTableIsEmptyInWaveThree and the synthetic
// tables in probe_test.go / e2e_test.go are what currently exercise the
// branch it feeds.
func WithSiblingRecordLengths(l SiblingLengths) Option {
	return func(d *ic905Driver) {
		d.siblingLengths = make(SiblingLengths, len(l))
		for n, model := range l {
			d.siblingLengths[n] = model
		}
	}
}
