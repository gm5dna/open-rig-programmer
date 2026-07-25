// SPDX-License-Identifier: GPL-3.0-or-later

package clone

import "context"

// MemorySelector is an OPTIONAL capability a driver.Session's CONCRETE
// type may implement: query and recall the radio's current memory
// selection. Controller design decision (task 20 brief): this is
// deliberately NOT added to driver.Session itself — core/driver/ft710.Session
// implements it as a concrete-type addition (CurrentMemory, RecallMemory,
// both built on the existing MC codec), so driver.Session's seam stays
// unchanged for every OTHER driver, and Execute below reaches it via a
// plain type assertion (s.sess.(MemorySelector)) rather than a new,
// mandatory interface method every future driver would have to implement
// even if its radio has no concept of a "current selection" at all.
// fakeradio-driven tests exercise this through the REAL driver, never a
// mock, because the concrete *ft710.Session IS what Execute's tests
// construct (see core/clone/helpers_test.go's openSimSession).
//
// Obligation 12 (see doc.go): HW-CONFIRMED 2026-07-13 (M5b write trials,
// docs/hardware-notes.md) — an MW write moves the radio's selection to
// the written slot, hands-off. A delta-write loop that writes several
// slots therefore drags the radio's operating selection through every
// one of them; Execute snapshots the selection before the loop and
// best-effort restores it afterwards, via this interface.
type MemorySelector interface {
	// CurrentMemory reports the radio's current memory selection as a
	// canonical wire-form slot. An error (including an unparseable/empty
	// answer) means no usable snapshot is available — the caller must
	// skip the restore step, never guess a recall target.
	CurrentMemory(ctx context.Context) (string, error)
	// RecallMemory issues a recall (MC-set) for slot.
	RecallMemory(ctx context.Context, slot string) error
}

// snapshotMemorySelection best-effort snapshots this Service's session's
// current memory selection (obligation 12), for restoreMemorySelection to
// recall afterwards. Returns "" when there is nothing to restore later:
// the session's concrete driver.Session does not implement MemorySelector
// at all, a transport failure occurred, or the answer was
// unparseable/empty (the "000" VFO-state hypothesis remains UNTESTED —
// core/cat/mc.go's doc comment). None of these are fatal to Execute —
// only to the restore step that would otherwise follow — so a journal
// note records why (best-effort: a durability hiccup on THIS line is
// logged, never surfaced as an Execute failure) and Execute proceeds with
// its delta-write loop regardless.
func (s *Service) snapshotMemorySelection(ctx context.Context, journal journalAppender) string {
	sel, ok := s.sess.(MemorySelector)
	if !ok {
		return ""
	}
	slot, err := sel.CurrentMemory(ctx)
	if err != nil || slot == "" {
		s.journalAppend(journal, "mc_snapshot", map[string]any{"ok": false, "error": errString(err)})
		return ""
	}
	s.journalAppend(journal, "mc_snapshot", map[string]any{"ok": true, "slot": slot})
	return slot
}

// restoreMemorySelection best-effort recalls slot — the snapshot
// snapshotMemorySelection took — via the same MemorySelector type
// assertion. A recall failure is a journal warning, never an abort
// (obligation 12): by the time this runs, every channel write this
// Execute call attempted has already been through its own
// write-then-verify (obligation 7); the operating-selection restore is a
// courtesy on top of that, not a safety gate. Uses an internal,
// caller-independent context (mirroring writePair's Fix 7 rationale,
// execute.go): Execute's own ctx may already be cancelled or expired by
// the time this best-effort cleanup runs, but the courtesy recall still
// deserves a fair, bounded try.
func (s *Service) restoreMemorySelection(journal journalAppender, slot string) {
	sel, ok := s.sess.(MemorySelector)
	if !ok {
		return // unreachable in practice — snapshotMemorySelection already required this to return a non-empty slot
	}
	ctx, cancel := context.WithTimeout(context.Background(), writeVerifyPairTimeout)
	defer cancel()
	err := sel.RecallMemory(ctx, slot)
	s.journalAppend(journal, "mc_restore", map[string]any{"ok": err == nil, "slot": slot, "error": errString(err)})
}
