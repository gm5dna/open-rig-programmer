// SPDX-License-Identifier: GPL-3.0-or-later

package codeplug

import (
	"encoding/json"
	"fmt"
)

// MenuEntryState classifies one menu/EX entry within a MenuSnapshot.
type MenuEntryState string

const (
	// MenuKnown means the read succeeded; Value carries the raw canonical
	// P4 the radio answered.
	MenuKnown MenuEntryState = "known"
	// MenuUnavailable means the radio rejected the read ("?;") at snapshot
	// time; Value is empty (there is no value to record).
	MenuUnavailable MenuEntryState = "unavailable"
	// MenuUnsupported means this ID is not in the reading build's
	// descriptor. The entry is PRESERVED, never dropped — its Value is
	// carried verbatim so a newer build that does understand the ID loses
	// nothing — but it is carried data, not freshly read data, so an empty
	// Value is allowed.
	MenuUnsupported MenuEntryState = "unsupported"
)

// MenuEntry is one menu/EX setting captured in a MenuSnapshot.
type MenuEntry struct {
	// ID is the stable 6-digit menu identifier, P1P2P3.
	ID string `json:"id"`
	// Value is the raw canonical P4 value (see MenuEntryState for when it
	// is populated).
	Value string `json:"value,omitempty"`
	// State classifies this entry — see MenuEntryState.
	State MenuEntryState `json:"state"`
}

// MenuSnapshot is a captured set of a radio's menu/EX settings, plus the
// provenance needed to interpret it. It is the schema-2 successor to the
// opaque v1.1 "menus" reservation: every subtree is schema-controlled
// EXCEPT Legacy, which preserves a migrated v1 opaque payload verbatim.
type MenuSnapshot struct {
	// Descriptor names the menu-descriptor build this snapshot was read
	// against, e.g. "ft710-ex@1", for provenance.
	Descriptor string `json:"descriptor"`
	// Complete is true only when every entry the descriptor defines was
	// read successfully: no Unavailable and no Unsupported entries (see
	// Validate).
	Complete bool `json:"complete"`
	// Entries holds the captured menu settings.
	Entries []MenuEntry `json:"entries"`
	// Legacy is a verbatim-preserved v1 opaque "menus" payload — the ONLY
	// schema-uncontrolled subtree in schema 2. It is nil for a natively
	// written snapshot that never migrated a v1 file.
	Legacy json.RawMessage `json:"legacy,omitempty"`
}

// Clone returns a nil-safe deep copy of m: a fresh Entries slice (each
// MenuEntry holds only strings, so copying the slice is a full deep copy)
// and a fresh Legacy byte slice. Mutating the clone can never reach the
// original.
func (m *MenuSnapshot) Clone() *MenuSnapshot {
	if m == nil {
		return nil
	}
	out := *m
	if m.Entries != nil {
		out.Entries = make([]MenuEntry, len(m.Entries))
		copy(out.Entries, m.Entries)
	}
	if m.Legacy != nil {
		out.Legacy = append(json.RawMessage(nil), m.Legacy...)
	}
	return &out
}

// isSettingIDWidth reports whether id is exactly FOUR or exactly SIX ASCII
// digits.
//
// A menu setting ID is a radio's EX address rendered as wire digits, so its
// width belongs to the RADIO and not to this package. Both widths the
// project's dialects express are admitted: six for a (P1,P2,P3) MENU Number
// — the FT-710, FTdx10 and FTdx101, core/cat's EXAddressTriple — and four
// for a (P1,P2) one, core/cat's EXAddressPair. This is a validator rule
// only: no serialised field changed and the schema did not move.
//
// FIVE STAYS REFUSED, and that is the whole reason this is two exact widths
// rather than a 4..6 range. The rule exists to catch a mis-shaped ID before
// it is written to a file or put to a radio, and a range would admit
// precisely the truncated six-digit address it was written to catch.
func isSettingIDWidth(id string) bool {
	if len(id) != 4 && len(id) != 6 {
		return false
	}
	for i := 0; i < len(id); i++ {
		if id[i] < '0' || id[i] > '9' {
			return false
		}
	}
	return true
}

// Validate enforces the MenuSnapshot consistency rules, returning the
// first violation as a typed error (*MenuEntryError or
// *DuplicateMenuIDError). It is nil-safe: a nil snapshot is valid (it
// simply carries no menu data). Load and Save both call this, so the exact
// same rules gate a file on the way in and on the way out:
//
//   - a Known entry must have a non-empty Value; an Unavailable entry must
//     have an empty Value; an Unsupported entry's Value is preserved
//     verbatim and may be empty;
//   - every ID is exactly four or exactly six ASCII digits — the two EX
//     address widths, see isSettingIDWidth — and no ID repeats;
//   - a Complete snapshot contains no Unavailable and no Unsupported
//     entries (those two states are precisely the ways a read was NOT
//     complete).
func (m *MenuSnapshot) Validate() error {
	if m == nil {
		return nil
	}
	seen := make(map[string]bool, len(m.Entries))
	for i, e := range m.Entries {
		if !isSettingIDWidth(e.ID) {
			return &MenuEntryError{Index: i, ID: e.ID, Reason: "id must be exactly 4 or 6 ASCII digits"}
		}
		if seen[e.ID] {
			return &DuplicateMenuIDError{ID: e.ID}
		}
		seen[e.ID] = true

		switch e.State {
		case MenuKnown:
			if e.Value == "" {
				return &MenuEntryError{Index: i, ID: e.ID, Reason: "a known entry must have a non-empty value"}
			}
		case MenuUnavailable:
			if e.Value != "" {
				return &MenuEntryError{Index: i, ID: e.ID, Reason: "an unavailable entry must have an empty value"}
			}
			if m.Complete {
				return &MenuEntryError{Index: i, ID: e.ID, Reason: "a complete snapshot must not contain an unavailable entry"}
			}
		case MenuUnsupported:
			if m.Complete {
				return &MenuEntryError{Index: i, ID: e.ID, Reason: "a complete snapshot must not contain an unsupported entry"}
			}
		default:
			return &MenuEntryError{Index: i, ID: e.ID, Reason: fmt.Sprintf("unknown state %q", e.State)}
		}
	}
	return nil
}

// MergeMenuSnapshots produces the snapshot a refresh should store: fresh's
// entries as-is, plus every old entry whose ID is absent from fresh
// carried forward as MenuUnsupported with its Value preserved (so a menu
// the current descriptor no longer knows about is never silently lost).
// old.Legacy is carried verbatim. Descriptor and Complete come from fresh,
// except Complete is forced false whenever any Unsupported entry was
// carried (a snapshot carrying unread, descriptor-unknown data is by
// definition not complete). A nil old returns fresh unchanged.
//
// The result is independently allocated: mutating it never reaches old or
// fresh.
func MergeMenuSnapshots(old, fresh *MenuSnapshot) *MenuSnapshot {
	if old == nil {
		return fresh
	}

	out := &MenuSnapshot{
		Descriptor: fresh.Descriptor,
		Complete:   fresh.Complete,
	}
	if old.Legacy != nil {
		out.Legacy = append(json.RawMessage(nil), old.Legacy...)
	}

	freshIDs := make(map[string]bool, len(fresh.Entries))
	for _, e := range fresh.Entries {
		freshIDs[e.ID] = true
	}

	entries := make([]MenuEntry, 0, len(fresh.Entries)+len(old.Entries))
	entries = append(entries, fresh.Entries...)

	carried := false
	for _, e := range old.Entries {
		if freshIDs[e.ID] {
			continue // fresh wins for any ID present in both.
		}
		entries = append(entries, MenuEntry{ID: e.ID, Value: e.Value, State: MenuUnsupported})
		carried = true
	}
	out.Entries = entries
	if carried {
		out.Complete = false
	}
	return out
}

// DuplicateMenuIDError reports that a MenuSnapshot repeats a menu entry ID.
// encoding/json cannot catch this (two entries are distinct array
// elements, not duplicate object keys), so Validate does.
type DuplicateMenuIDError struct{ ID string }

// Error implements the error interface.
func (e *DuplicateMenuIDError) Error() string {
	return fmt.Sprintf("codeplug: duplicate menu entry id %q", e.ID)
}

// MenuEntryError reports that one MenuEntry violates a consistency rule.
type MenuEntryError struct {
	// Index is the entry's position in MenuSnapshot.Entries.
	Index int
	// ID is the offending entry's ID (as read, even if that ID is itself
	// the problem).
	ID string
	// Reason is a human-readable description of the violated rule.
	Reason string
}

// Error implements the error interface.
func (e *MenuEntryError) Error() string {
	return fmt.Sprintf("codeplug: menu entry %d (id %q): %s", e.Index, e.ID, e.Reason)
}
