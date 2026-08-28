// SPDX-License-Identifier: GPL-3.0-or-later

package ic905_test

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ/ic905"
)

// The band rule, and why it needs no band field.
//
// Ten packed-BCD digits — the five-byte frequency field the
// memory-content diagram draws — reach at most 9,999,999,999 Hz. The
// 10G band starts at 10,000,000,000 Hz (PDF p.20, folio 19, "Band
// stacking register", "(1): Frequency band codes", row
// "06 | 10G | 10000.000000 ~ 10500.000000"; the same table is printed
// again at PDF p.30, folio 29 — matrix Erratum 9). No band is documented
// between 5850 MHz and 10 GHz. So over the documented storable set,
// "does not fit ten digits" and "is in band 06" are the SAME predicate,
// and the record carrying no band field (matrix Erratum 8) costs
// nothing.
//
// ASSUMED, both halves: that the memory record widens at all, and the
// width it takes. Register: D5 model-specific entry for the 905
// (6-byte frequency >= 10 GHz, and its second record length). Lift:
// ic905-R-06.
func TestRecordLengthForFrequency(t *testing.T) {
	for _, tc := range []struct {
		name string
		hz   uint64
		want int
	}{
		{"144 MHz band floor", 144_000_000, ic905.RecordLengthShort},
		{"the golden 68-byte vector's 144.5 MHz", 144_500_000, ic905.RecordLengthShort},
		{"5600 band, well inside ten digits", 5_760_000_000, ic905.RecordLengthShort},
		{"the last frequency ten BCD digits reach", 9_999_999_999, ic905.RecordLengthShort},
		{"the 10G band floor", 10_000_000_000, ic905.RecordLengthWide},
		{"the golden 69-byte vector's 10.25 GHz", 10_250_000_000, ic905.RecordLengthWide},
		{"the documented ceiling", 10_500_000_000, ic905.RecordLengthWide},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ic905.RecordLengthForFrequency(tc.hz); got != tc.want {
				t.Errorf("RecordLengthForFrequency(%d) = %d, want %d", tc.hz, got, tc.want)
			}
			wide := tc.want == ic905.RecordLengthWide
			if got := ic905.NeedsWideFrequency(tc.hz); got != wide {
				t.Errorf("NeedsWideFrequency(%d) = %v, want %v", tc.hz, got, wide)
			}
		})
	}
}
