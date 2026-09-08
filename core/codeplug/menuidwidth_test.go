// SPDX-License-Identifier: GPL-3.0-or-later

package codeplug

import (
	"errors"
	"strings"
	"testing"
)

// TestMenuSnapshotValidate_SettingIDWidths pins the setting-ID shape rule
// across every EX address form and around each edge.
//
// A menu setting ID is a radio's EX address rendered as wire digits, so its
// width is the radio's, not this package's: six for the FT-710, FTdx10 and
// FTdx101 (a (P1,P2,P3) triple), four for a radio whose MENU Number is a
// (P1,P2) pair, three for a Kenwood MENU number, and five for the
// TS-890S/TS-990S grouped EX address (P1 P2P2 P3P3). Only two and
// seven-or-more are left outside, so what this rule now refuses is a shape
// no radio in the fleet addresses at all. The fourth width and what
// admitting it costs are pinned by
// TestMenuSnapshotValidate_ThreeDigitIDs and
// TestMenuSnapshotValidate_KenwoodGroupedSnapshot (menus_test.go).
func TestMenuSnapshotValidate_SettingIDWidths(t *testing.T) {
	for _, tc := range []struct {
		name   string
		id     string
		wantOK bool
	}{
		{"six digits (the triple form)", "000101", true},
		{"four digits (the pair form)", "0801", true},
		{"five digits (the Kenwood grouped form)", "00010", true},
		{"three digits (the Kenwood form)", "080", true},
		{"two digits", "08", false},
		{"seven digits", "0001011", false},
		{"empty", "", false},
		{"four with a non-digit", "08X1", false},
		{"five with a non-digit", "000X0", false},
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
			if !strings.Contains(mee.Reason, "3, 4, 5 or 6 ASCII digits") {
				t.Errorf("%s: MenuEntryError.Reason = %q, want it to name all four widths", tc.name, mee.Reason)
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
