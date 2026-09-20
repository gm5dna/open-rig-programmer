// SPDX-License-Identifier: GPL-3.0-or-later

package codeplug

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
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
	// ID is the stable menu identifier: exactly 3, 4, 5 or 6 ASCII digits —
	// the EX address in its dialect's wire form (P1P2P3 under
	// EXAddressTriple, P1P2 under EXAddressPair, bare P1 under
	// EXAddressSingle — the FT-991A; cat.Dialect.EXWire renders each of
	// those three Yaesu forms — a bare Kenwood MENU number under core/kw,
	// or the grouped P1 P2P2 P3P3 EX address the TS-890S and TS-990S use
	// (core/kw/ma, landing at Stage 1).
	// Not always six: the S0-close review's LOW-4 finding was this comment
	// still promising P1P2P3 unconditionally after the FT-891's narrower
	// wire form was added, and the Kenwood line has since narrowed it twice
	// more. See isSettingIDWidth, which is the only place the width is
	// judged.
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
	out.Entries = slices.Clone(m.Entries)
	out.Legacy = slices.Clone(m.Legacy)
	return &out
}

// isSettingIDWidth reports whether id is exactly THREE, exactly FOUR,
// exactly FIVE or exactly SIX ASCII digits.
//
// A menu setting ID is a radio's EX address rendered as wire digits, so its
// width belongs to the RADIO and not to this package. Every width the
// project's dialects express is admitted: six for a (P1,P2,P3) MENU Number
// — the FT-710, FTdx10 and FTdx101, core/cat's EXAddressTriple — four for a
// (P1,P2) one, core/cat's EXAddressPair — three for either a bare P1
// under core/cat's EXAddressSingle (the FT-991A) or a Kenwood MENU number,
// which core/kw addresses as its own bare three-digit field — and five for
// the grouped P1 P2P2 P3P3 EX address of the TS-890S and TS-990S, which
// will be rendered as one five-character field (core/kw/ma, landing at
// Stage 1). This is a validator rule only: no serialised field changed and
// the schema did not move.
// TestMenuSnapshotValidate_SettingIDWidths and
// TestMenuSnapshotValidate_ThreeDigitIDs pin every admitted width and the
// edges either side of the set.
//
// FIVE IS NOW ADMITTED, AND THAT SPENDS THE ARGUMENT THIS PARAGRAPH USED TO
// MAKE. It said the widths were named one by one rather than as a 3..6
// range precisely so that five — the shape a (P1,P2,P3) address that lost
// one digit takes — stayed refused. The TS-890S and TS-990S address a
// setting by a five-character EX field, so admitting five is what "the
// width belongs to the radio" now requires, and the admitted set runs
// unbroken from three to six: a range in all but name. The truncation
// guard this rule was written to be is therefore VESTIGIAL, and the cost is
// published here rather than argued away.
//
// WHAT STILL CATCHES A TRUNCATED ADDRESS IS INVENTORY MEMBERSHIP, but at a
// DIFFERENT LAYER and LATER than this rule ever ran. This package is
// radio-neutral and holds no inventory; membership is consulted by the
// DRIVER, when an ID is parsed back into an address
// (cat.Dialect.ParseEXAddress, core/cat/exinventory.go:186), pinned per
// radio by TestSession_ReadSetting_ErrorTyping (Yaesu; zero frames, both
// the four- and now the five-digit rows) and
// TestReadSetting_AnUnknownIDIsRefusedBeforeAnyFrame (Kenwood). So the
// catch moves down a layer and later: a MenuSnapshot — a codeplug file on
// load, or clone's before-wire preflight — carrying a truncated five-digit
// Yaesu address now validates clean, and nothing refuses it until the
// radio is asked for that setting. That is where the check belonged in any
// case: neither of the two books this widening serves prints a contiguous
// EX domain to bound an address against, so a width rule could only ever
// have been a proxy for membership — and a poor one, since the widths of
// two radios in the same fleet may coincide.
//
// WHAT ADMITTING THREE COST, kept because it is the same ledger and the
// three-digit entry is still true: it re-opened the truncation failure mode
// one width down, and twice. A (P1,P2) Pair address truncated from four
// digits to three validates, and so does a (P1,P2,P3) Triple address
// truncated to three. Neither validated before.
//
// The mitigation is partial, and is recorded as partial. A Kenwood setting
// ID is never DERIVED from a Yaesu one — the inventories come from
// different generated files, different internal/extable profiles and
// different radios — so no CROSS-FAMILY path produces a truncated address
// that this widened validator would then accept. That argument does not
// reach the within-family case: a driver that renders one of its own
// radio's addresses into the wrong width is a formatting bug inside a
// single inventory, and this rule no longer catches any of those. What it
// reaches now is two-or-fewer and seven-or-more digits, and a non-digit at
// any width.
func isSettingIDWidth(id string) bool {
	if n := len(id); n < 3 || n > 6 {
		return false
	}
	return strings.IndexFunc(id, func(r rune) bool { return r < '0' || r > '9' }) < 0
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
//   - every ID is exactly 3, 4, 5 or 6 ASCII digits — the four EX address
//     widths, see isSettingIDWidth — and no ID repeats;
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
			return &MenuEntryError{Index: i, ID: e.ID, Reason: "id must be exactly 3, 4, 5 or 6 ASCII digits"}
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
		out.Legacy = slices.Clone(old.Legacy)
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
