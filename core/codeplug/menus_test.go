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
			name:    "5-digit id",
			snap:    &MenuSnapshot{Entries: []MenuEntry{{ID: "00010", Value: "3", State: MenuKnown}}},
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
