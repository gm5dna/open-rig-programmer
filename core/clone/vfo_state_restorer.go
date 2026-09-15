// SPDX-License-Identifier: GPL-3.0-or-later

package clone

import (
	"context"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
)

// VFOStateRestorer is MemorySelector's sibling for a family whose write
// choreography must overwrite VFO-A's live content to commit a channel
// (v1.9.0 binary-CAT write model, §Write model: the FT-920/FT-890/
// FT-900/FT-1000MP write path is always VFO→memory — there is no
// per-channel "write record N" verb on any of them). Like
// MemorySelector, this is deliberately NOT added to driver.Session
// itself: it is an OPTIONAL capability a session's CONCRETE type may
// implement, reached from Execute via a plain type assertion
// (s.sess.(VFOStateRestorer)), so driver.Session's seam stays unchanged
// for every driver whose radio has no VFO to disturb (the FT-710
// included).
//
// The failure policy is the opposite of MemorySelector's, and that is
// the whole reason this is a separate interface rather than an addition
// to MemorySelector: a MemorySelector snapshot failure is best-effort —
// the write proceeds anyway, because all that is at risk is a current-
// memory POINTER. Here, steps 1-6 of the write choreography overwrite
// VFO-A's actual CONTENT, so a SnapshotVFOState error means ABORT before
// building any frame — there is no safe "proceed without a snapshot"
// path when proceeding means knowingly clobbering data this package
// never captured.
type VFOStateRestorer interface {
	// SnapshotVFOState reads VFO-A's current content and the operator's
	// active-VFO selection (A or B), before anything overwrites either.
	// An error means no usable snapshot is available — the caller MUST
	// abort before the first mutating frame, never proceed and guess.
	SnapshotVFOState(ctx context.Context) (VFOSnapshot, error)
	// RestoreVFOState re-selects snapshot.ActiveVFO and re-sends
	// snapshot.Content to VFO-A, undoing what the write choreography's
	// steps 1-6 did to it. Called only after a successful
	// SnapshotVFOState; a restore failure is a journal warning, never an
	// abort — see restoreVFOState's doc comment.
	RestoreVFOState(ctx context.Context, snapshot VFOSnapshot) error
}

// VFOSnapshot is what SnapshotVFOState captures and RestoreVFOState
// replays: VFO-A's content (reusing codeplug.ChannelData — no new
// field-carrying type needed) plus which VFO the operator had active, a
// selection ChannelData itself has no field for.
type VFOSnapshot struct {
	// Content is VFO-A's live content at snapshot time.
	Content codeplug.ChannelData
	// ActiveVFO is "A" or "B" — the write choreography's step 1 always
	// selects VFO-A, so restoring Content alone could leave A selected
	// when the operator had B active.
	ActiveVFO string
}

// snapshotVFOState snapshots this Service's session's VFO-A state via the
// VFOStateRestorer type assertion, for restoreVFOState to replay
// afterwards. Returns ok=false, err=nil when the session's concrete type
// does not implement VFOStateRestorer at all (nothing to snapshot or
// restore — every driver registered before this family). Unlike
// snapshotMemorySelection (memory_selector.go), a non-nil err here is
// FATAL: the caller (Execute) must abort before the delta-write loop
// starts, not proceed with an unknown VFO state.
func (s *Service) snapshotVFOState(ctx context.Context, journal journalAppender) (VFOSnapshot, bool, error) {
	r, ok := s.sess.(VFOStateRestorer)
	if !ok {
		return VFOSnapshot{}, false, nil
	}
	snap, err := r.SnapshotVFOState(ctx)
	if err != nil {
		s.journalAppend(journal, "vfo_snapshot", map[string]any{"ok": false, "error": errString(err)})
		return VFOSnapshot{}, false, err
	}
	s.journalAppend(journal, "vfo_snapshot", map[string]any{"ok": true, "active_vfo": snap.ActiveVFO})
	return snap, true, nil
}

// restoreVFOState best-effort restores snapshot — the VFO state
// snapshotVFOState captured — via the same VFOStateRestorer type
// assertion. Mirrors restoreMemorySelection's shape and rationale
// exactly: an internal, caller-independent, bounded context (Execute's
// own ctx may already be cancelled by the time this deferred cleanup
// runs, but the courtesy restore still deserves a fair, bounded try), and
// a restore failure is a journal warning, never a cause Execute's
// returned error is replaced with — by the time this runs, every channel
// write this Execute call attempted has already been through its own
// write-then-verify (obligation 7); restoring VFO-A is a courtesy on top
// of that, not a safety gate.
func (s *Service) restoreVFOState(journal journalAppender, snapshot VFOSnapshot) {
	r, ok := s.sess.(VFOStateRestorer)
	if !ok {
		return // unreachable in practice — snapshotVFOState already required this to succeed
	}
	ctx, cancel := context.WithTimeout(context.Background(), writeVerifyPairTimeout)
	defer cancel()
	err := r.RestoreVFOState(ctx, snapshot)
	s.journalAppend(journal, "vfo_restore", map[string]any{"ok": err == nil, "active_vfo": snapshot.ActiveVFO, "error": errString(err)})
}
