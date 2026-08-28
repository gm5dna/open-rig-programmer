// SPDX-License-Identifier: GPL-3.0-or-later

package ic905

// RecordLengthShort and RecordLengthWide are this model's two
// RECORD-ONLY lengths — the payload after the four channel-address
// bytes, which is what civ.Profile's layouts and BuildMemorySet's
// <record> argument denote (spec Erratum 1). Their data-area
// equivalents on the wire are 68 and 69.
const (
	// RecordLengthShort is the shape PDF p.19 (folio 18) draws: five
	// frequency bytes at printed indices (6)...(10), with (11) the next
	// printed index. MANUAL-EVIDENCED, addend by addend (matrix
	// section 3.11 Condition A).
	RecordLengthShort = 64
	// RecordLengthWide is the same record with the frequency field
	// widened to six bytes for the 10 GHz band. ASSUMED: the
	// memory-content diagram prints ONE shape, and the 5-/6-byte
	// conditional is printed against four OTHER command lists, none of
	// which includes 1A 00 (matrix section 3.11 Condition B, Erratum 1).
	// Register: D5 model-specific entry for the 905. Lift: ic905-R-06.
	RecordLengthWide = 65
)

// MaxNarrowFrequencyHz is the largest frequency ten packed-BCD digits
// can express, and therefore the largest this model's five-byte
// frequency field can carry.
const MaxNarrowFrequencyHz = uint64(9_999_999_999)

// NeedsWideFrequency reports whether hz requires the six-byte
// frequency form.
//
// The predicate is arithmetic, not a band lookup, and it is exactly
// equivalent to "is in band 06" over the documented storable set: the
// 10G band starts at 10,000,000,000 Hz, one hertz past what ten digits
// reach, and no band is documented between 5850 MHz and 10 GHz. That
// equivalence is why the record carrying no band field (matrix
// Erratum 8) does not leave the write path guessing.
func NeedsWideFrequency(hz uint64) bool { return hz > MaxNarrowFrequencyHz }

// RecordLengthForFrequency is the record length hz would need.
//
// It is NOT what BuildMemorySet emits — civ.ProfileConfig.BuildLength
// is static, and this profile declares RecordLengthShort (see
// profile_test.go's TestBuildLengthIsTheShapeTheDiagramDraws for the
// argument; it lives there because it needs a built Profile). It exists
// so that core/driver/ic905 can REFUSE a wide write before the wire with
// a named reason, and so that the one-line change ic905-R-06 would
// authorise is already written down.
func RecordLengthForFrequency(hz uint64) int {
	if NeedsWideFrequency(hz) {
		return RecordLengthWide
	}
	return RecordLengthShort
}
