// SPDX-License-Identifier: GPL-3.0-or-later

package clone

import (
	"context"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
)

// ReadSettings reads every item in the session's settings descriptor and
// assembles the result into a fresh *codeplug.MenuSnapshot — the
// settings-surface counterpart to ReadAll (task 33/M8b-3), with three
// differences ReadAll does not have:
//
//   - OPT-IN: not every session exposes a settings surface at all (see
//     driver.SettingsReader's doc comment, and MemorySelector for the
//     identical optional-interface precedent). A session whose concrete
//     driver.Session type does not implement driver.SettingsReader
//     refuses with a *SettingsUnsupportedError wrapping
//     ErrSettingsUnsupported — BEFORE any wire traffic.
//   - PARTIAL-TOLERANT: an individual item the radio currently rejects
//     ("?;") is recorded as MenuUnavailable and the read continues — a
//     rejection is data about this radio's current state, not a failure
//     of the read itself. Only a genuine ReadSetting error (a transport
//     failure, a malformed answer) aborts the whole call.
//   - CANCELLABLE mid-run: ctx is checked between every item (not just
//     once at the start), same as ReadAll checks it between every slot.
//
// Descriptor validation: the descriptor driver.SettingsReader.
// SettingsDescriptor returns is ALREADY a defensive copy (see
// SettingsDescriptor.Clone) — ReadSettings fetches it once, holds that
// local copy for the whole call, and validates it (descriptor.Validate())
// BEFORE issuing a single ReadSetting call. A malformed descriptor (e.g. a
// driver bug producing a duplicate item ID) is refused with zero wire
// exchanges, exactly like the capability-absence case above.
//
// Per item: ReadSetting(ctx, id). Its returned SettingValue.ID is
// cross-checked against the id just requested — a driver returning a
// value for the WRONG item is a bug this layer refuses to paper over by
// silently accepting it, so a mismatch aborts the whole call (naming both
// IDs) rather than being recorded as that item's answer. A SettingKnown
// result becomes a MenuEntry{ID, Value: Raw, State: MenuKnown}; a
// SettingUnavailable result becomes MenuEntry{ID, State: MenuUnavailable}
// (empty Value) and the loop continues — see PARTIAL-TOLERANT above. Any
// OTHER ReadSetting error aborts immediately, wrapped as
// "clone: ReadSettings: setting %q: %w" (failures are failures, never
// papered over the way a "?;" rejection is).
//
// Like ReadAll, this is entirely read-only: no journal line, no
// SnapshotStore write. Progress is reported once per item, phase
// "read-settings", 1-based done against the total item count across every
// menu/group in the descriptor, the neutral setting ID as the name
// parameter.
//
// acquireOp("ReadSettings") is held for the call's WHOLE duration —
// mutual exclusion with ReadAll/PrepareSend/Execute, exactly as those
// three already exclude each other (see Service.acquireOp). This package
// never lets a settings read interleave its own wire traffic with a
// channel read/write, or vice versa.
//
// On ANY error (unsupported, malformed descriptor, a cancelled/expired
// ctx, a driver ID mismatch, or a genuine ReadSetting failure) the
// returned snapshot is always nil — there is no such thing as a "partial
// snapshot on error": a partial snapshot is only ever returned on
// SUCCESS, when every item was attempted and some came back
// MenuUnavailable (see Complete below).
func (s *Service) ReadSettings(ctx context.Context) (*codeplug.MenuSnapshot, error) {
	if err := s.acquireOp("ReadSettings"); err != nil {
		return nil, err
	}
	defer s.releaseOp()

	reader, ok := s.sess.(driver.SettingsReader)
	if !ok {
		return nil, &SettingsUnsupportedError{Model: s.sess.Capabilities().Model}
	}

	// A defensive copy already (SettingsDescriptor's own contract) — held
	// locally for the whole call, never re-fetched per item.
	descriptor := reader.SettingsDescriptor()
	if err := descriptor.Validate(); err != nil {
		return nil, fmt.Errorf("clone: ReadSettings: invalid settings descriptor: %w", err)
	}

	// SettingsDescriptor.Validate accepts any non-empty, unique opaque item
	// ID, but a MenuSnapshot requires every ID to be exactly 4 or exactly 6
	// ASCII digits — the two EX address widths (see MenuSnapshot.Validate).
	// Without this preflight a custom SettingsReader minting a mis-shaped ID
	// would pass descriptor validation, be READ against the wire, and fail
	// only AFTER every read when the built snapshot is validated. Probe the
	// snapshot rule up front by validating an all-MenuUnsupported snapshot
	// built from the descriptor (Complete stays false, so the only
	// per-entry rules that fire are the ID shape and ID uniqueness) — a bad
	// ID is then refused with zero wire exchanges, exactly like the
	// capability-absence and malformed-descriptor cases above.
	total := 0
	probe := &codeplug.MenuSnapshot{}
	for _, menu := range descriptor.Menus {
		for _, group := range menu.Groups {
			total += len(group.Items)
			for _, item := range group.Items {
				probe.Entries = append(probe.Entries, codeplug.MenuEntry{ID: item.ID, State: codeplug.MenuUnsupported})
			}
		}
	}
	if err := probe.Validate(); err != nil {
		return nil, fmt.Errorf("clone: ReadSettings: settings descriptor item ID is not a valid snapshot ID: %w", err)
	}

	entries := make([]codeplug.MenuEntry, 0, total)
	unavailable := 0
	done := 0
	for _, menu := range descriptor.Menus {
		for _, group := range menu.Groups {
			for _, item := range group.Items {
				if err := ctx.Err(); err != nil {
					return nil, fmt.Errorf("clone: ReadSettings: %w", err)
				}

				val, err := reader.ReadSetting(ctx, item.ID)
				if err != nil {
					return nil, fmt.Errorf("clone: ReadSettings: setting %q: %w", item.ID, err)
				}
				if val.ID != item.ID {
					return nil, fmt.Errorf("clone: ReadSettings: requested setting %q but the driver returned a value for %q — refusing to map an answer onto the wrong setting", item.ID, val.ID)
				}

				entry := codeplug.MenuEntry{ID: item.ID}
				switch val.State {
				case driver.SettingKnown:
					entry.Value = val.Raw
					entry.State = codeplug.MenuKnown
				case driver.SettingUnavailable:
					entry.State = codeplug.MenuUnavailable
					unavailable++
				default:
					// Defensive: driver.SettingsReader's contract only ever
					// produces these two states for a real driver; a third
					// value can only come from a misbehaving implementation
					// — never guess which of the two it meant.
					return nil, fmt.Errorf("clone: ReadSettings: setting %q: unrecognised SettingReadState %v", item.ID, val.State)
				}
				entries = append(entries, entry)

				done++
				s.progress("read-settings", done, total, item.ID)
			}
		}
	}

	// Complete means "every item the descriptor defines was read
	// successfully" — i.e. every item is PRESENT (one MenuEntry per
	// descriptor item, guaranteed by the triple loop above: it appends
	// exactly once per Item, in order, and returns early on any error, so
	// len(entries) == total always holds here) AND Known (no
	// MenuUnavailable entry was recorded). With presence already
	// guaranteed by construction, that reduces to the single check below:
	// zero unavailable entries. MenuUnsupported is NEVER produced by a
	// read — the loop above only ever writes MenuKnown or MenuUnavailable
	// — it exists solely for MergeMenuSnapshots' job of carrying an OLD
	// entry forward when a newer descriptor no longer defines its ID.
	snapshot := &codeplug.MenuSnapshot{
		Descriptor: descriptor.Version,
		Complete:   unavailable == 0,
		Entries:    entries,
	}

	// Belt-and-braces: the snapshot just built must itself satisfy the
	// same consistency rules Load/Save enforce on every codeplug file.
	if err := snapshot.Validate(); err != nil {
		return nil, fmt.Errorf("clone: ReadSettings: internal error: built snapshot failed Validate: %w", err)
	}

	return snapshot, nil
}
