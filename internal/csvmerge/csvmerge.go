// SPDX-License-Identifier: GPL-3.0-or-later

// Package csvmerge holds the pure CSV/CHIRP merge policy shared by
// cmd/rigprog's "import" subcommand and app/'s ImportCSV/ImportCHIRP
// bound methods (task-15 brief §2) — extracted from
// cmd/rigprog/import.go's mergeCSV/mergeCHIRP so the GUI reuses exactly
// the same merge semantics the CLI already proved, rather than a second,
// independently-drifting copy.
//
// This is deliberately NOT core/codeplug: it encodes CLI/GUI PRODUCT
// policy (how an imported CSV/CHIRP file should be reconciled onto an
// existing codeplug — full-replace vs sparse merge, which mismatches
// refuse wholesale) rather than radio/codeplug model semantics. Nothing
// here talks to a session, a driver, or the wire.
package csvmerge

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
)

// channelSlotSet returns the set of every Slot in channels.
func channelSlotSet(channels []codeplug.Channel) map[string]bool {
	set := make(map[string]bool, len(channels))
	for _, ch := range channels {
		set[ch.Slot] = true
	}
	return set
}

// joinOrNone renders items as a comma-joined list, or "none" when empty —
// used so a missing/extra slot list reads clearly either way.
func joinOrNone(items []string) string {
	if len(items) == 0 {
		return "none"
	}
	return strings.Join(items, ", ")
}

// InventoryMismatchError is MergeCSV's refusal error: imported's slot
// inventory does not match base's exactly. Both slices are wire-form
// slots, sorted. A caller wanting cmd/rigprog's own original wording
// (naming its --csv/--into flags) should errors.As against this rather
// than relying on Error()'s generic text — see cmd/rigprog/import.go's
// mergeCSV alias.
type InventoryMismatchError struct {
	// Missing lists slots present in base but absent from imported.
	Missing []string
	// Extra lists slots present in imported but absent from base.
	Extra []string
}

// Error implements the error interface with GENERIC wording (this
// package is shared by both a CLI and a GUI caller, neither of which
// necessarily has "--csv"/"--into" flags) — see InventoryMismatchError's
// doc comment for how a caller wanting different wording should use the
// struct fields instead.
func (e *InventoryMismatchError) Error() string {
	return fmt.Sprintf("imported CSV slot inventory differs from the target's inventory (missing: %s; extra: %s)", joinOrNone(e.Missing), joinOrNone(e.Extra))
}

// MergeCSV replaces base's Channels wholesale with imported (task-13
// brief §2's --csv rule): rigprog's own CSV is a lossless, full-inventory
// round-trip format, so the two slot inventories must match EXACTLY —
// any difference means the CSV came from a different radio/region, and
// is refused rather than guessed at (a non-nil *InventoryMismatchError).
// base is left completely unchanged on refusal.
func MergeCSV(base *codeplug.Codeplug, imported []codeplug.Channel) error {
	baseSlots := channelSlotSet(base.Channels)
	importedSlots := channelSlotSet(imported)

	var missing, extra []string
	for slot := range baseSlots {
		if !importedSlots[slot] {
			missing = append(missing, slot)
		}
	}
	for slot := range importedSlots {
		if !baseSlots[slot] {
			extra = append(extra, slot)
		}
	}
	if len(missing) > 0 || len(extra) > 0 {
		sort.Strings(missing)
		sort.Strings(extra)
		return &InventoryMismatchError{Missing: missing, Extra: extra}
	}

	base.Channels = imported
	return nil
}

// UnknownSlotsError is MergeCHIRP's refusal error when imported names a
// slot base's inventory does not contain at all. Slots are wire-form,
// sorted. See InventoryMismatchError's doc comment for why this carries
// structured fields rather than baking in CLI-specific wording.
type UnknownSlotsError struct {
	Slots []string
}

// Error implements the error interface with GENERIC wording — see
// UnknownSlotsError's doc comment.
func (e *UnknownSlotsError) Error() string {
	return fmt.Sprintf("CHIRP row(s) target slot(s) not in the target's inventory: %s", strings.Join(e.Slots, ", "))
}

// DuplicateSlotsError is MergeCHIRP's refusal error when imported
// contains more than one row for the same slot (Fix 5, adjudicated
// MEDIUM, Codex M4 #5). Slots are already in DISPLAY form
// (codeplug.DisplaySlot), sorted.
type DuplicateSlotsError struct {
	Slots []string
}

// Error implements the error interface.
func (e *DuplicateSlotsError) Error() string {
	return fmt.Sprintf("CHIRP import has more than one row for slot(s): %s", strings.Join(e.Slots, ", "))
}

// MergeCHIRP merges imported (sparse: a CHIRP import only ever produces
// one row per slot it could map at all — task-13 brief §2) onto base BY
// SLOT: every base slot NOT touched by imported keeps its current
// contents unchanged. Every imported slot must already exist in base's
// inventory; if any do not, the merge is refused wholesale (a non-nil
// *UnknownSlotsError, naming every offending slot) rather than partially
// applied.
//
// Also refuses wholesale — before mutating base at all — if imported
// itself contains more than one row for the same slot (Fix 5, adjudicated
// MEDIUM, Codex M4 #5): csvio.ImportCHIRP does not deduplicate by
// Location, so two CHIRP rows can legitimately both map to the same
// slot. Applying them in order (last-wins) would silently discard the
// earlier row's data — an ambiguous sparse merge this function refuses
// outright instead (a non-nil *DuplicateSlotsError, naming every
// duplicated slot) so the caller can fix the source CSV.
func MergeCHIRP(base *codeplug.Codeplug, imported []codeplug.Channel) error {
	baseIndex := make(map[string]int, len(base.Channels))
	for i, ch := range base.Channels {
		baseIndex[ch.Slot] = i
	}

	var unknown []string
	seen := make(map[string]int, len(imported))
	var duplicated []string
	for _, ch := range imported {
		if _, ok := baseIndex[ch.Slot]; !ok {
			unknown = append(unknown, ch.Slot)
		}
		seen[ch.Slot]++
		if seen[ch.Slot] == 2 {
			duplicated = append(duplicated, ch.Slot)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return &UnknownSlotsError{Slots: unknown}
	}
	if len(duplicated) > 0 {
		sort.Strings(duplicated)
		names := make([]string, len(duplicated))
		for i, slot := range duplicated {
			names[i] = codeplug.DisplaySlot(slot)
		}
		return &DuplicateSlotsError{Slots: names}
	}

	for _, ch := range imported {
		base.Channels[baseIndex[ch.Slot]] = ch
	}
	return nil
}
