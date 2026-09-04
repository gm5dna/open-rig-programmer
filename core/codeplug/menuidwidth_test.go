// SPDX-License-Identifier: GPL-3.0-or-later

package codeplug

import (
	"errors"
	"strings"
	"testing"
)

// TestMenuSnapshotValidate_SettingIDWidths pins the setting-ID shape rule
// across both EX address forms and around each edge.
//
// A menu setting ID is a radio's EX address rendered as wire digits, so its
// width is the radio's, not this package's: six for the FT-710, FTdx10 and
// FTdx101 (a (P1,P2,P3) triple), four for a radio whose MENU Number is a
// (P1,P2) pair. Five is neither, and stays refused — the point of the rule
// is that a mis-shaped ID is caught before it is written to a file or put
// to a radio, and widening it to "four to six" would admit exactly the
// truncation it exists to catch.
func TestMenuSnapshotValidate_SettingIDWidths(t *testing.T) {
	for _, tc := range []struct {
		name   string
		id     string
		wantOK bool
	}{
		{"six digits (the triple form)", "000101", true},
		{"four digits (the pair form)", "0801", true},
		{"five digits is neither form", "00010", false},
		{"three digits", "080", false},
		{"seven digits", "0001011", false},
		{"empty", "", false},
		{"four with a non-digit", "08X1", false},
		{"six with a non-digit", "0001A1", false},
	} {
		snap := &MenuSnapshot{Entries: []MenuEntry{{ID: tc.id, Value: "3", State: MenuKnown}}}
		err := snap.Validate()
		if tc.wantOK && err != nil {
			t.Errorf("%s: Validate() on ID %q = %v, want accepted", tc.name, tc.id, err)
		}
		if !tc.wantOK {
			if err == nil {
				t.Errorf("%s: Validate() accepted ID %q", tc.name, tc.id)
				continue
			}
			var mee *MenuEntryError
			if !errors.As(err, &mee) {
				t.Errorf("%s: Validate() = %v, want a *MenuEntryError", tc.name, err)
				continue
			}
			if !strings.Contains(mee.Reason, "4 or 6 ASCII digits") {
				t.Errorf("%s: MenuEntryError.Reason = %q, want it to name both widths", tc.name, mee.Reason)
			}
			if mee.ID != tc.id {
				t.Errorf("%s: MenuEntryError.ID = %q, want %q — the refusal must name the offending ID", tc.name, mee.ID, tc.id)
			}
		}
	}
}

// TestMenuSnapshotValidate_FourDigitIDsGoThroughEveryOtherRule checks that
// admitting the narrower width did not open a hole in the rules beside it:
// uniqueness, the per-state value rules and the Complete rule must all still
// fire on a four-digit ID.
func TestMenuSnapshotValidate_FourDigitIDsGoThroughEveryOtherRule(t *testing.T) {
	dup := &MenuSnapshot{Entries: []MenuEntry{
		{ID: "0801", Value: "3", State: MenuKnown},
		{ID: "0801", Value: "5", State: MenuKnown},
	}}
	var de *DuplicateMenuIDError
	if err := dup.Validate(); !errors.As(err, &de) {
		t.Errorf("Validate() on duplicate four-digit IDs = %v, want *DuplicateMenuIDError", err)
	}
	empty := &MenuSnapshot{Entries: []MenuEntry{{ID: "0801", State: MenuKnown}}}
	if err := empty.Validate(); err == nil {
		t.Error("Validate() accepted a Known four-digit entry with an empty value")
	}
	complete := &MenuSnapshot{Complete: true, Entries: []MenuEntry{{ID: "0801", State: MenuUnavailable}}}
	if err := complete.Validate(); err == nil {
		t.Error("Validate() accepted a Complete snapshot carrying an Unavailable four-digit entry")
	}
}
