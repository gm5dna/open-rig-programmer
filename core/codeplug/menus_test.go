// SPDX-License-Identifier: GPL-3.0-or-later

package codeplug

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

// TestMenuSnapshot_ValidateTable violates each consistency rule in
// isolation and asserts the matching typed error (and that a valid, and a
// nil, snapshot both pass).
func TestMenuSnapshot_ValidateTable(t *testing.T) {
	cases := []struct {
		name       string
		snap       *MenuSnapshot
		wantErr    bool
		wantEntry  bool // expect *MenuEntryError
		wantDupeID bool // expect *DuplicateMenuIDError
	}{
		{
			name: "valid mixed states",
			snap: &MenuSnapshot{Descriptor: "ft710-ex@1", Complete: false, Entries: []MenuEntry{
				{ID: "000101", Value: "3", State: MenuKnown},
				{ID: "000202", State: MenuUnavailable},
				{ID: "000303", Value: "7", State: MenuUnsupported},
			}},
			wantErr: false,
		},
		{name: "nil snapshot", snap: nil, wantErr: false},
		{
			name:    "known with empty value",
			snap:    &MenuSnapshot{Entries: []MenuEntry{{ID: "000101", State: MenuKnown}}},
			wantErr: true, wantEntry: true,
		},
		{
			name:    "unavailable with value",
			snap:    &MenuSnapshot{Entries: []MenuEntry{{ID: "000101", Value: "3", State: MenuUnavailable}}},
			wantErr: true, wantEntry: true,
		},
		{
			name:    "unsupported with empty value allowed",
			snap:    &MenuSnapshot{Entries: []MenuEntry{{ID: "000101", State: MenuUnsupported}}},
			wantErr: false,
		},
		{
			// Seven, not five: five became an admitted width with the
			// TS-890S/TS-990S grouped EX address, so the out-of-vocabulary
			// case this row exists for had to move up one.
			name:    "7-digit id",
			snap:    &MenuSnapshot{Entries: []MenuEntry{{ID: "0001011", Value: "3", State: MenuKnown}}},
			wantErr: true, wantEntry: true,
		},
		{
			name:    "non-digit id",
			snap:    &MenuSnapshot{Entries: []MenuEntry{{ID: "0001A1", Value: "3", State: MenuKnown}}},
			wantErr: true, wantEntry: true,
		},
		{
			name:    "unknown state",
			snap:    &MenuSnapshot{Entries: []MenuEntry{{ID: "000101", Value: "3", State: "bogus"}}},
			wantErr: true, wantEntry: true,
		},
		{
			name: "duplicate id",
			snap: &MenuSnapshot{Entries: []MenuEntry{
				{ID: "000101", Value: "3", State: MenuKnown},
				{ID: "000101", Value: "5", State: MenuKnown},
			}},
			wantErr: true, wantDupeID: true,
		},
		{
			name: "complete with unavailable entry",
			snap: &MenuSnapshot{Complete: true, Entries: []MenuEntry{
				{ID: "000101", State: MenuUnavailable},
			}},
			wantErr: true, wantEntry: true,
		},
		{
			name: "complete with unsupported entry",
			snap: &MenuSnapshot{Complete: true, Entries: []MenuEntry{
				{ID: "000101", Value: "7", State: MenuUnsupported},
			}},
			wantErr: true, wantEntry: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.snap.Validate()
			if tc.wantErr && err == nil {
				t.Fatal("Validate() = nil, want error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
			if tc.wantEntry {
				var mee *MenuEntryError
				if !errors.As(err, &mee) {
					t.Errorf("errors.As(err, *MenuEntryError) = false (err = %v)", err)
				}
			}
			if tc.wantDupeID {
				var dme *DuplicateMenuIDError
				if !errors.As(err, &dme) {
					t.Errorf("errors.As(err, *DuplicateMenuIDError) = false (err = %v)", err)
				}
			}
		})
	}
}

// TestMergeMenuSnapshots covers the refresh-merge rule end to end.
func TestMergeMenuSnapshots(t *testing.T) {
	t.Run("nil old returns fresh unchanged", func(t *testing.T) {
		fresh := &MenuSnapshot{Descriptor: "fresh@2", Entries: []MenuEntry{{ID: "000101", Value: "9", State: MenuKnown}}}
		if got := MergeMenuSnapshots(nil, fresh); got != fresh {
			t.Errorf("MergeMenuSnapshots(nil, fresh) = %p, want fresh (%p) unchanged", got, fresh)
		}
	})

	t.Run("carries legacy, preserves absent ids as unsupported, fresh wins, forces complete false", func(t *testing.T) {
		old := &MenuSnapshot{
			Descriptor: "old@1",
			Complete:   true,
			Entries: []MenuEntry{
				{ID: "000101", Value: "3", State: MenuKnown}, // present in fresh -> fresh wins
				{ID: "000202", Value: "7", State: MenuKnown}, // absent from fresh -> carried
			},
			Legacy: json.RawMessage(`{"leg":1}`),
		}
		fresh := &MenuSnapshot{
			Descriptor: "fresh@2",
			Complete:   true,
			Entries:    []MenuEntry{{ID: "000101", Value: "9", State: MenuKnown}},
		}
		got := MergeMenuSnapshots(old, fresh)

		if got.Descriptor != "fresh@2" {
			t.Errorf("Descriptor = %q, want fresh@2", got.Descriptor)
		}
		if got.Complete {
			t.Error("Complete = true, want false (an unsupported entry was carried)")
		}
		var lb bytes.Buffer
		if err := json.Compact(&lb, got.Legacy); err != nil {
			t.Fatalf("json.Compact error = %v", err)
		}
		if lb.String() != `{"leg":1}` {
			t.Errorf("Legacy = %s, want old's {\"leg\":1} carried verbatim", lb.String())
		}
		want := []MenuEntry{
			{ID: "000101", Value: "9", State: MenuKnown},       // fresh wins
			{ID: "000202", Value: "7", State: MenuUnsupported}, // carried, value verbatim
		}
		if !reflect.DeepEqual(got.Entries, want) {
			t.Errorf("Entries = %+v, want %+v", got.Entries, want)
		}
	})

	t.Run("nothing carried keeps fresh complete", func(t *testing.T) {
		old := &MenuSnapshot{Entries: []MenuEntry{{ID: "000101", Value: "3", State: MenuKnown}}}
		fresh := &MenuSnapshot{
			Complete: true,
			Entries: []MenuEntry{
				{ID: "000101", Value: "9", State: MenuKnown},
				{ID: "000202", Value: "5", State: MenuKnown},
			},
		}
		got := MergeMenuSnapshots(old, fresh)
		if !got.Complete {
			t.Error("Complete = false, want true (nothing carried, fresh was complete)")
		}
		if len(got.Entries) != 2 {
			t.Errorf("len(Entries) = %d, want 2 (no carry)", len(got.Entries))
		}
	})
}

// TestMenuSnapshot_CloneIndependence: Clone is nil-safe and produces a
// fully independent copy — mutating the clone's Entries or Legacy never
// reaches the original.
func TestMenuSnapshot_CloneIndependence(t *testing.T) {
	if (*MenuSnapshot)(nil).Clone() != nil {
		t.Fatal("nil.Clone() != nil, want nil")
	}

	orig := &MenuSnapshot{
		Descriptor: "ft710-ex@1",
		Entries:    []MenuEntry{{ID: "000101", Value: "3", State: MenuKnown}},
		Legacy:     json.RawMessage(`{"leg":1}`),
	}
	clone := orig.Clone()
	clone.Entries[0].Value = "MUTATED"
	clone.Legacy[0] = 'X'

	if orig.Entries[0].Value != "3" {
		t.Errorf("orig.Entries[0].Value = %q, want unchanged \"3\"", orig.Entries[0].Value)
	}
	if orig.Legacy[0] != '{' {
		t.Errorf("orig.Legacy[0] = %q, want unchanged '{'", orig.Legacy[0])
	}
}

// nonEmptyMenus is a representative non-nil snapshot used by the
// ignore-menus pins to vary Menus without touching channels.
func nonEmptyMenus() *MenuSnapshot {
	return &MenuSnapshot{
		Descriptor: "ft710-ex@1",
		Entries:    []MenuEntry{{ID: "000101", Value: "3", State: MenuKnown}},
		Legacy:     []byte(`{"leg":1}`),
	}
}

// withMenus returns cp with its Menus set, for chaining.
func withMenus(cp *Codeplug, m *MenuSnapshot) *Codeplug {
	cp.Menus = m
	return cp
}

// TestDigest_IgnoresMenus: a codeplug's content digest (Digest over its
// channels) depends only on the channels — setting or clearing Menus never
// moves it. The positive control (a mutated channel DOES move the digest)
// guards against this pin passing merely because the digest is constant.
func TestDigest_IgnoresMenus(t *testing.T) {
	noMenus := withMenus(testBaselineCodeplug(), nil)
	withM := withMenus(testBaselineCodeplug(), nonEmptyMenus())

	if Digest(noMenus.Channels) != Digest(withM.Channels) {
		t.Error("Digest moved when only Menus differed; it must depend on channels alone")
	}

	// Positive control: the digest is genuinely live over channel content.
	mutated := testBaselineCodeplug()
	mutated.Channels[0].Data.FreqHz += 1000
	if Digest(mutated.Channels) == Digest(noMenus.Channels) {
		t.Fatal("Digest did not move when a channel changed — the pin above is vacuous")
	}
}

// TestDiff_IgnoresMenus: Diff takes the whole *Codeplug (so it COULD read
// Menus), yet varying only the file's Menus must leave the DiffResult
// byte-for-byte identical. The diff is deliberately non-trivial (a real
// Modified entry) so the pin has teeth.
func TestDiff_IgnoresMenus(t *testing.T) {
	caps := testCapabilities()
	base := testBaselineCodeplug()

	makeFile := func(m *MenuSnapshot) *Codeplug {
		f := testBaselineCodeplug()
		f.Channels[0].Data.Tag = "EDITED" // a genuine change vs base -> Modified
		f.Menus = m
		return f
	}

	da, err := Diff(base, makeFile(nil), caps)
	if err != nil {
		t.Fatalf("Diff(no menus) error = %v", err)
	}
	db, err := Diff(base, makeFile(nonEmptyMenus()), caps)
	if err != nil {
		t.Fatalf("Diff(with menus) error = %v", err)
	}
	if da.Modified == 0 {
		t.Fatal("test setup produced no Modified entries — the pin would be vacuous")
	}
	if !reflect.DeepEqual(da, db) {
		t.Error("DiffResult moved when only the file's Menus differed")
	}
}

// TestValidate_IgnoresMenus: Validate takes the whole *Codeplug but its
// Issues must not shift when only Menus varies. A deliberate channel-level
// error is present so the compared Issue lists are non-empty (live).
func TestValidate_IgnoresMenus(t *testing.T) {
	caps := testCapabilities()

	makeCP := func(m *MenuSnapshot) *Codeplug {
		cp := testBaselineCodeplug()
		cp.Channels[0].Data.Mode = "NOPE" // unsupported mode -> a real issue
		cp.Menus = m
		return cp
	}

	a := Validate(makeCP(nil), caps)
	b := Validate(makeCP(nonEmptyMenus()), caps)
	if len(a) == 0 {
		t.Fatal("test setup produced no issues — the pin would be vacuous")
	}
	if !reflect.DeepEqual(a, b) {
		t.Error("Validate issues moved when only Menus differed")
	}
}

// TestMenuSnapshotValidate_ThreeDigitIDs pins the width the TS-590S/SG and
// TS-480 needed, and that the widths either side of it still behave.
//
// The Kenwood TS-590S/SG and TS-480 address a menu by a three-digit MENU
// number, so isSettingIDWidth admits 3 as well as 4 and 6. Five was refused
// when this test was written and is admitted now — the TS-890S/TS-990S
// grouped EX address is five wire characters — so the admitted set runs
// from three to six and the row below asserts the acceptance rather than
// the refusal. See isSettingIDWidth's own rationale for what that costs and
// for what catches a truncated address in its place.
func TestMenuSnapshotValidate_ThreeDigitIDs(t *testing.T) {
	for _, tc := range []struct {
		name   string
		id     string
		wantOK bool
	}{
		{"three digits (the Kenwood MENU number)", "080", true},
		{"three digits, all zero", "000", true},
		{"four digits (the pair form) still accepted", "0801", true},
		{"six digits (the triple form) still accepted", "000101", true},
		{"five digits (the grouped form) now accepted", "00010", true},
		{"two digits", "08", false},
		{"seven digits", "0001011", false},
		{"three with a non-digit", "08X", false},
	} {
		snap := &MenuSnapshot{Entries: []MenuEntry{{ID: tc.id, Value: "3", State: MenuKnown}}}
		err := snap.Validate()
		if tc.wantOK && err != nil {
			t.Errorf("%s: Validate() on ID %q = %v, want accepted", tc.name, tc.id, err)
		}
		if !tc.wantOK && err == nil {
			t.Errorf("%s: Validate() accepted ID %q", tc.name, tc.id)
		}
	}
}

// TestMenuSnapshotValidate_KenwoodShapedSnapshot validates a whole snapshot
// shaped the way a Kenwood settings read will build one: every ID three
// digits, the three entry states mixed, Complete false. It is the
// snapshot-level counterpart of the width table above — the per-entry rules
// beside the width (uniqueness, the per-state Value rules, the Complete
// rule) must all still fire on a three-digit ID, exactly as
// TestMenuSnapshotValidate_FourDigitIDsGoThroughEveryOtherRule asserts for
// the pair form.
func TestMenuSnapshotValidate_KenwoodShapedSnapshot(t *testing.T) {
	snap := &MenuSnapshot{
		Descriptor: "ts590sg-ex@1",
		Entries: []MenuEntry{
			{ID: "000", Value: "1", State: MenuKnown},
			{ID: "021", State: MenuUnavailable},
			{ID: "087", Value: "2", State: MenuUnsupported},
		},
	}
	if err := snap.Validate(); err != nil {
		t.Errorf("Validate() on a Kenwood-shaped snapshot = %v, want accepted", err)
	}

	dup := &MenuSnapshot{Entries: []MenuEntry{
		{ID: "080", Value: "3", State: MenuKnown},
		{ID: "080", Value: "5", State: MenuKnown},
	}}
	var de *DuplicateMenuIDError
	if err := dup.Validate(); !errors.As(err, &de) {
		t.Errorf("Validate() on duplicate three-digit IDs = %v, want *DuplicateMenuIDError", err)
	}

	empty := &MenuSnapshot{Entries: []MenuEntry{{ID: "080", State: MenuKnown}}}
	if err := empty.Validate(); err == nil {
		t.Error("Validate() accepted a Known three-digit entry with an empty value")
	}

	complete := &MenuSnapshot{Complete: true, Entries: []MenuEntry{{ID: "080", State: MenuUnavailable}}}
	if err := complete.Validate(); err == nil {
		t.Error("Validate() accepted a Complete snapshot carrying an Unavailable three-digit entry")
	}
}

// TestMenuSnapshotValidate_KenwoodGroupedSnapshot is the pin that carries
// the five-digit decision: a whole snapshot shaped the way a TS-890S or
// TS-990S settings read will build one — every ID the grouped EX address
// P1 P2P2 P3P3 rendered as five wire characters, the three entry states
// mixed, Complete false.
//
// WHAT ADMITTING FIVE COSTS, pinned here rather than left in prose alone:
// the admitted set is now every width from three to six, so a (P1,P2,P3)
// Yaesu address that lost one digit on its way to an ID validates where it
// used to be refused. isSettingIDWidth's doc comment records why that is
// accepted and what catches such an address instead (inventory
// membership). The last row below is what this rule still reaches — a
// seven-digit shape no radio in the fleet addresses.
func TestMenuSnapshotValidate_KenwoodGroupedSnapshot(t *testing.T) {
	snap := &MenuSnapshot{
		Descriptor: "ts890s-ex@1",
		Entries: []MenuEntry{
			{ID: "00000", Value: "1", State: MenuKnown},
			{ID: "10203", State: MenuUnavailable},
			{ID: "90909", Value: "2", State: MenuUnsupported},
		},
	}
	if err := snap.Validate(); err != nil {
		t.Errorf("Validate() on a Kenwood-grouped snapshot = %v, want accepted", err)
	}

	dup := &MenuSnapshot{Entries: []MenuEntry{
		{ID: "10203", Value: "3", State: MenuKnown},
		{ID: "10203", Value: "5", State: MenuKnown},
	}}
	var de *DuplicateMenuIDError
	if err := dup.Validate(); !errors.As(err, &de) {
		t.Errorf("Validate() on duplicate five-digit IDs = %v, want *DuplicateMenuIDError", err)
	}

	// errors.As alone does not discriminate here: a *MenuEntryError is also
	// what a reverted (pre-widening) isSettingIDWidth would return for this
	// five-digit ID, for the width reason rather than this one — checking
	// Reason is what pins "refused for THIS rule", as the width table above
	// already pins the width rule itself.
	empty := &MenuSnapshot{Entries: []MenuEntry{{ID: "10203", State: MenuKnown}}}
	var eme *MenuEntryError
	wantEmptyReason := "a known entry must have a non-empty value"
	if err := empty.Validate(); !errors.As(err, &eme) || eme.Reason != wantEmptyReason {
		t.Errorf("Validate() on a Known five-digit entry with an empty value = %v, want *MenuEntryError{Reason: %q}", err, wantEmptyReason)
	}

	complete := &MenuSnapshot{Complete: true, Entries: []MenuEntry{{ID: "10203", State: MenuUnavailable}}}
	var cme *MenuEntryError
	wantCompleteReason := "a complete snapshot must not contain an unavailable entry"
	if err := complete.Validate(); !errors.As(err, &cme) || cme.Reason != wantCompleteReason {
		t.Errorf("Validate() on a Complete snapshot carrying an Unavailable five-digit entry = %v, want *MenuEntryError{Reason: %q}", err, wantCompleteReason)
	}

	wide := &MenuSnapshot{Entries: []MenuEntry{{ID: "1020304", Value: "1", State: MenuKnown}}}
	var mee *MenuEntryError
	if err := wide.Validate(); !errors.As(err, &mee) {
		t.Errorf("Validate() on a seven-digit ID = %v, want *MenuEntryError — the widening must not have become 'any width'", err)
	}
}
