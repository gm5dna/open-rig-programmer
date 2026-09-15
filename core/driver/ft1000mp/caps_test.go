// SPDX-License-Identifier: GPL-3.0-or-later

package ft1000mp

import "testing"

// TestRecordModeBase_MatchesOpcodeFamilyFormula verifies recordModeBase's
// hand-written table against the formula its own doc comment claims: for
// a 0CH wire code >= 2, the record's 3-bit family index is
// 2+(code-2)/2 (integer division); codes 0 and 1 (LSB/USB) map to
// themselves.
func TestRecordModeBase_MatchesOpcodeFamilyFormula(t *testing.T) {
	familyIndex := func(code byte) byte {
		if code < 2 {
			return code
		}
		return 2 + (code-2)/2
	}
	for code, name := range modeNames {
		idx := familyIndex(code)
		wantBase, ok := recordModeBase[idx]
		if !ok {
			t.Fatalf("code %#02x (%s): family index %d has no recordModeBase entry", code, name, idx)
		}
		if _, ok := modeNames[wantBase]; !ok {
			t.Fatalf("recordModeBase[%d] = %#02x is not a modeNames key", idx, wantBase)
		}
	}
	// And the reverse: every recordModeBase value is itself the base
	// member of its own family (its own formula fixes to itself).
	for idx, base := range recordModeBase {
		if familyIndex(base) != idx {
			t.Errorf("recordModeBase[%d] = %#02x, but familyIndex(%#02x) = %d", idx, base, base, familyIndex(base))
		}
	}
}

func TestCapabilities_Validate(t *testing.T) {
	if err := CapabilitiesUnverified().Validate(); err != nil {
		t.Errorf("CapabilitiesUnverified().Validate(): %v", err)
	}
	if err := CapabilitiesSimulated().Validate(); err != nil {
		t.Errorf("CapabilitiesSimulated().Validate(): %v", err)
	}
}
