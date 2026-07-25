// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import "testing"

// TestFieldConstantsDistinct enumerates the ten Field constants this
// package defines and checks each has a non-empty, distinct underlying
// string. Erase is deliberately included: it is modelled as a Field (not a
// separate mechanism) so that the same FieldSupport machinery gates it.
func TestFieldConstantsDistinct(t *testing.T) {
	fields := []Field{
		FieldFrequency,
		FieldMode,
		FieldClarifier,
		FieldCTCSSState,
		FieldCTCSSTone,
		FieldShift,
		FieldTag,
		FieldTagDisplay,
		FieldScanSkip,
		FieldErase,
	}
	const wantCount = 10
	if len(fields) != wantCount {
		t.Fatalf("listed %d Field constants, want %d", len(fields), wantCount)
	}
	seen := make(map[Field]bool, len(fields))
	for _, f := range fields {
		if f == "" {
			t.Error("Field constant has empty underlying string")
		}
		if seen[f] {
			t.Errorf("duplicate Field value %q", f)
		}
		seen[f] = true
	}
}
