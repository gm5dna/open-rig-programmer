// SPDX-License-Identifier: GPL-3.0-or-later

package clone

import (
	"context"
	"fmt"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// writeVerifyPairTimeout bounds the INTERNAL context Service.writePair
// derives for each write+verify pair (M3 Codex-review fix wave, Fix 7,
// adjudicated MEDIUM), once write_attempt is durably journaled: once a
// wire write is underway, a caller's ctx cancelling must not be able to
// abandon it unverified (see writePair's doc comment) — but Execute still
// needs SOME bound so a genuinely wedged transport cannot hang forever
// with caller cancellation deliberately unavailable for this window. This
// is deliberately generous — several times transport's own
// DefaultTimeout+DefaultErrorWindow for a single exchange, since a pair
// is up to four exchanges (MW, MT, then the read-back's MR, MT) plus
// their Settle pacing.
//
// PRE-HARDWARE ESTIMATE — like transport's own DefaultTimeout/
// DefaultErrorWindow/DefaultSettle, chosen without a physical FT-710 to
// measure against; to be reviewed at M5a alongside those.
const writeVerifyPairTimeout = 10 * time.Second

// ExecuteOptions carries the caller-side confirmations Execute requires
// beyond plan and confirmedDigest.
type ExecuteOptions struct {
	// FirmwareConfirmed is the firmware version a human has read off the
	// radio's front panel and confirmed, required (non-empty) the first
	// time this Service's session performs an Execute call (obligation
	// 10, the first-write interactive gate). Once accepted, later
	// Execute calls on the same Service do not need to repeat it. This
	// layer enforces presence only — obtaining the value from a human is
	// the CLI/GUI's job.
	FirmwareConfirmed string
}

// Report is what Execute returns: exactly what happened, and — if
// execution stopped early — why, and how far it got.
type Report struct {
	// FirmwareConfirmed is the firmware version string confirmed for this
	// Service's session (obligation 10's audit clause): the value this
	// Execute call's caller supplied via ExecuteOptions.FirmwareConfirmed
	// the FIRST time this Service performed an Execute, persisted for
	// every later Execute call on the same Service regardless of whether
	// THIS call supplied it again. Callers (CLI/GUI) should display and/or
	// persist this alongside the rest of the report — it is the durable
	// record of what a human confirmed they read off the radio's front
	// panel, not merely a gate that was satisfied and then forgotten.
	FirmwareConfirmed string
	// Written counts channels whose WriteChannel call completed without
	// error (the frame was sent and drew no rejection within its error
	// window — see driver.WriteResult's doc comment for exactly what
	// that does, and does not, guarantee at the wire level).
	Written int
	// Verified counts channels that were Written AND whose read-back
	// matched what was sent (obligation 7).
	Verified int
	// SkippedBlocked counts diff entries never attempted because they
	// were Blocked (obligation 6) — including every Erased entry, always
	// blocked under this project's v1 capability profiles.
	SkippedBlocked int
	// Unchanged counts diff entries that required no action.
	Unchanged int
	// Slots details every channel Execute acted on or skipped, in the
	// order it processed them.
	Slots []SlotResult
	// Aborted is true if Execute stopped before processing every delta
	// entry.
	Aborted bool
	// AbortReason explains Aborted; empty when Aborted is false.
	AbortReason string
	// JournalPath is where this execution's journal lines were appended
	// (obligation 8) — the same file PrepareSend began writing to.
	JournalPath string
}

// SlotResult is one entry in a Report.Slots: what Execute did (or decided
// not to do) for one slot.
type SlotResult struct {
	// Slot is the canonical wire-form slot identifier.
	Slot string
	// Action names what happened to this slot: "write" (written, and
	// verified unless this is the aborting entry), "skipped-blocked"
	// (Blocked in the diff, never attempted), "verify-read-drift"
	// (obligation 11 caught the radio's memory having changed since
	// PrepareSend, before any write was attempted), or "verify-read-error"
	// (obligation 11's re-read itself failed — a transport error, as
	// opposed to a clean read that disagreed).
	Action string
	// VerifyOK is true only for a "write" entry whose read-back matched.
	VerifyOK bool
	// Detail is a human-readable explanation: the block reason, the
	// mismatch description, or empty for a clean write.
	Detail string
}

// Action values a SlotResult may carry.
const (
	actionWrite           = "write"
	actionSkippedBlocked  = "skipped-blocked"
	actionVerifyReadDrift = "verify-read-drift"
	actionVerifyReadError = "verify-read-error"
)

// writableFieldsMismatch compares want (what was sent) against got (the
// read-back) on every field this project's write path can express.
// CTCSSTone and ScanSkip are deliberately excluded: both always read back
// FieldState Unknown by construction (driver.Session.ReadChannel's doc
// comment — the CAT protocol has no command to read either), so comparing
// them would manufacture a false mismatch on every single write.
func writableFieldsMismatch(want, got codeplug.ChannelData) []spec.Field {
	var bad []spec.Field
	if want.FreqHz != got.FreqHz {
		bad = append(bad, spec.FieldFrequency)
	}
	if want.Mode != got.Mode {
		bad = append(bad, spec.FieldMode)
	}
	if want.ClarHz != got.ClarHz || want.RxClar != got.RxClar || want.TxClar != got.TxClar {
		bad = append(bad, spec.FieldClarifier)
	}
	if want.CTCSS != got.CTCSS {
		bad = append(bad, spec.FieldCTCSSState)
	}
	if want.Shift != got.Shift {
		bad = append(bad, spec.FieldShift)
	}
	if want.Tag != got.Tag {
		bad = append(bad, spec.FieldTag)
	}
	if want.TagDisplay != got.TagDisplay {
		bad = append(bad, spec.FieldTagDisplay)
	}
	return bad
}

// verifyWriteResult compares a completed write's read-back (verify)
// against what was sent (ch) for slot, and returns the resulting
// *VerifyMismatchError (nil for a clean match) alongside the SlotResult
// Execute's delta-write loop should record either way (obligation 7).
//
// Factored out of the delta-write loop itself so this one decision — has
// the write actually landed as sent? — is unit-testable against
// constructed codeplug.Channel values, without a live driver.Session:
// see TestVerifyWriteResult for the populated-vs-different-values and
// populated-vs-empty cases this resolves.
func verifyWriteResult(slot string, ch, verify codeplug.Channel) (*VerifyMismatchError, SlotResult) {
	var mismatchErr *VerifyMismatchError
	if verify.Empty() {
		mismatchErr = &VerifyMismatchError{Slot: slot}
	} else if bad := writableFieldsMismatch(*ch.Data, *verify.Data); len(bad) > 0 {
		mismatchErr = &VerifyMismatchError{Slot: slot, Fields: bad}
	}
	if mismatchErr != nil {
		return mismatchErr, SlotResult{Slot: slot, Action: actionWrite, VerifyOK: false, Detail: mismatchErr.Error()}
	}
	return nil, SlotResult{Slot: slot, Action: actionWrite, VerifyOK: true}
}

// indexChannels returns channels keyed by Slot, for the delta-write loop's
// per-slot lookups against a plan's private copies.
func indexChannels(channels []codeplug.Channel) map[string]codeplug.Channel {
	m := make(map[string]codeplug.Channel, len(channels))
	for _, ch := range channels {
		m[ch.Slot] = ch
	}
	return m
}

// baselineChannelEqual reports whether a fresh read (fresh) still agrees
// with what the plan's baseline recorded for the same slot (before) — the
// verify-read phase's drift check (obligation 11). Both sides are "as read
// from the radio" data (CTCSSTone/ScanSkip always Unknown on both), so
// unlike writableFieldsMismatch a plain equality check is safe here:
// nothing asymmetric is being compared.
func baselineChannelEqual(fresh codeplug.Channel, before *codeplug.ChannelData) bool {
	if before == nil {
		return fresh.Empty()
	}
	if fresh.Empty() {
		return false
	}
	return *fresh.Data == *before
}

// errString returns err.Error(), or "" for a nil err — keeps journal field
// maps concise.
func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// Execute runs plan's delta writes against this Service's session.
//
// Order (fixed — see doc.go): refusal checks for obligations 3 (dual-digest
// recheck + re-Validate against the session's CURRENT capabilities), 4
// (session identity binding), 5 (confirmation binding), and 10 (first-write
// firmware gate), each a distinct typed error, nothing written; then the
// delta-write loop itself, once per unblocked Added/Modified entry
// (obligation 6): obligation 11's verify-read check, obligation 7's
// write-then-verify, and obligation 8's journaling, all run PER SLOT, in
// that order, immediately one after another (M3 Codex-review fix wave,
// Fix 3 — see doc.go's updated obligation 11 text for why this is no
// longer a separate up-front phase covering every slot before any of them
// are written).
//
// Cancellation (ctx) is honoured BETWEEN slots only — checked at the top
// of each iteration of the delta-write loop — but never WITHIN one slot's
// own verify-read/write/verify sequence: a write without its verify is an
// unknown state, and abandoning it mid-pair would leave the radio's
// memory and this package's bookkeeping unable to agree on what actually
// happened.
//
// Execute returns (nil, err) for every refusal — nothing was written. Once
// the delta-write loop (or the verify-read phase immediately before it)
// has started touching the radio, an abort (a verify mismatch, a write
// rejection, a transport failure, or a cancelled ctx) returns a NON-NIL
// partial *Report alongside a non-nil *AbortedError: both matter, since
// the report's Slots/Written/Verified counts carry forensic detail the
// error string alone cannot.
func (s *Service) Execute(ctx context.Context, plan *SendPlan, confirmedDigest string, opts ExecuteOptions) (*Report, error) {
	// Fix 2 (adjudicated MEDIUM): acquire this Service's operation lock for
	// Execute's WHOLE run — refusing a concurrent ReadAll/PrepareSend/
	// Execute call with a typed *BusyError rather than interleaving their
	// wire traffic. See Service.acquireOp's doc comment.
	if err := s.acquireOp("Execute"); err != nil {
		return nil, err
	}
	defer s.releaseOp()

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("clone: Execute: %w", err)
	}

	journal := s.openJournal(plan.snapshotPath)

	// Obligation 3: dual-digest recheck against the plan's own private
	// copies (a self-consistency assertion — see StaleBaselineError's doc
	// comment), then re-Validate the candidate against the session's
	// CURRENT effective capabilities.
	if got := codeplug.Digest(plan.baseline); got != plan.baselineDigest {
		return nil, &StaleBaselineError{Reason: "plan's private baseline digest no longer matches its recorded digest"}
	}
	if got := codeplug.Digest(plan.candidate); got != plan.candidateDigest {
		return nil, &CandidateChangedError{Reason: "plan's private candidate digest no longer matches its recorded digest"}
	}
	caps := s.sess.Capabilities()
	candidateCP := &codeplug.Codeplug{
		Schema:   codeplug.CurrentSchema,
		Radio:    plan.candidateRadio,
		Channels: plan.candidate,
	}
	if issues := codeplug.Validate(candidateCP, caps); codeplug.HasErrors(issues) {
		return nil, &ValidationFailedError{Issues: issues}
	}

	// Obligation 4: session identity binding. Generation catches a
	// reconnect that happens to answer with the SAME Identity content
	// (obligation 4's stated rationale); the Identity comparison is
	// belt-and-braces alongside it.
	if plan.generation != s.generation || plan.identity != s.sess.Identity() {
		return nil, &SessionChangedError{Reason: "this plan was prepared against a different session (a reconnect, or a different Service instance)"}
	}

	// Obligation 5: confirmation binding (Fix 1 — plan-scoped, not merely
	// baseline-scoped; see SendPlan.ConfirmationDigest's doc comment).
	if want := plan.ConfirmationDigest(); confirmedDigest != want {
		return nil, &ConfirmationMismatchError{Want: want, Got: confirmedDigest}
	}

	// Obligation 10: first-write interactive gate. The gate state itself
	// (firmwareConfirmed/firmwareVersion) is held once per Service, as
	// before — but the confirmed value must now actually flow somewhere
	// useful (the audit clause this closes): journaled once, the first
	// time it is accepted, and returned in EVERY Report from here on, not
	// left to sit in a write-only field nothing ever reads back.
	s.mu.Lock()
	firstConfirmation := false
	if !s.firmwareConfirmed {
		if opts.FirmwareConfirmed == "" {
			s.mu.Unlock()
			return nil, ErrFirmwareUnconfirmed
		}
		s.firmwareConfirmed = true
		s.firmwareVersion = opts.FirmwareConfirmed
		firstConfirmation = true
	}
	firmwareVersion := s.firmwareVersion
	s.mu.Unlock()

	// Emitted BEFORE the delta loop (and before the verify-read phase
	// too) — this is a record of a HUMAN'S confirmation, not a radio
	// interaction, so it belongs beside "prepare" in the journal's
	// timeline, not interleaved with per-slot write/verify lines.
	// Best-effort (journalAppend, not appendDeltaJournal): the gate state
	// this event records already lives safely in memory (s.firmwareConfirmed
	// above), so a durability hiccup here is worth a diagnostic, not a
	// refusal — see journalAppend's doc comment.
	if firstConfirmation {
		s.journalAppend(journal, "firmware_confirmed", map[string]any{"version": firmwareVersion})
	}

	report := &Report{JournalPath: journal.Path(), FirmwareConfirmed: firmwareVersion}

	// Partition the diff (obligation 6): Blocked entries (including every
	// Erased entry, always Blocked under this project's v1 capability
	// profiles) are skipped and counted; Unchanged entries need no
	// action; unblocked Added/Modified entries become the delta list.
	var deltas []codeplug.DiffEntry
	for _, e := range plan.diff.Entries {
		if e.Blocked {
			report.SkippedBlocked++
			report.Slots = append(report.Slots, SlotResult{Slot: e.Slot, Action: actionSkippedBlocked, Detail: e.BlockReason})
			continue
		}
		switch e.Kind {
		case codeplug.DiffAdded, codeplug.DiffModified:
			deltas = append(deltas, e)
		case codeplug.DiffUnchanged:
			report.Unchanged++
		case codeplug.DiffErased:
			// Defence in depth: every current (v1) capability profile
			// makes FieldErase never write-Supported, so codeplug.Diff's
			// own erase gate always marks a DiffErased entry Blocked —
			// this branch is unreachable today. If that ever changed
			// without this package changing too, an erase would still
			// never be written here: obligation 6 blocks erase in v1
			// unconditionally, not merely by relying on the capability
			// gate above.
			report.SkippedBlocked++
			report.Slots = append(report.Slots, SlotResult{Slot: e.Slot, Action: actionSkippedBlocked, Detail: "erase is not sent in v1"})
		}
	}

	if len(deltas) == 0 {
		s.journalAppend(journal, "completion", map[string]any{
			"written": 0, "verified": 0,
			"skipped_blocked": report.SkippedBlocked, "unchanged": report.Unchanged,
		})
		return report, nil
	}

	candidateBySlot := indexChannels(plan.candidate)

	// Obligation 12: snapshot the radio's current memory selection BEFORE
	// the first write below — MW moves it to the written slot
	// (HW-CONFIRMED 2026-07-13, M5b write trials, docs/hardware-notes.md)
	// — and best-effort restore it on the way out, however the loop below
	// ends (success, abort, or a cancelled ctx): see
	// snapshotMemorySelection/restoreMemorySelection (memory_selector.go)
	// for why this is entirely best-effort, never a refusal or an abort
	// cause. The defer covers every return path below this point,
	// including every s.abort(...) call inside the loop and writePair.
	if snap := s.snapshotMemorySelection(ctx, journal); snap != "" {
		defer s.restoreMemorySelection(journal, snap)
	}

	// Delta-write loop (obligations 6, 7, 8, 11). Fix 3 (M3 Codex-review
	// fix wave): each slot's verify-read (obligation 11) runs IMMEDIATELY
	// before that SAME slot's own write_attempt journal line — not as a
	// separate up-front phase covering every to-be-written slot before
	// ANY of them are written. This closes the drift-to-write window down
	// to as small as physically possible: under the old up-front-phase
	// design, a slot's baseline was re-checked, but then the radio's
	// memory could still drift AGAIN while every OTHER delta's verify-read
	// ran, before that slot's own write finally happened. See
	// TestExecute_VerifyReadPerSlot_FirstWrittenBeforeSecondDrifts for the
	// test that pins this down: draining a LATER slot's drift must not
	// prevent an EARLIER, undrifted slot from having already completed.
	for i, e := range deltas {
		if err := ctx.Err(); err != nil {
			return s.abort(report, journal, "", fmt.Sprintf("context cancelled before slot %q: %v", e.Slot, err), err)
		}

		// Obligation 11's per-slot check.
		fresh, err := s.sess.ReadChannel(ctx, e.Slot)
		if err != nil {
			report.Slots = append(report.Slots, SlotResult{Slot: e.Slot, Action: actionVerifyReadError, Detail: err.Error()})
			return s.abort(report, journal, e.Slot, fmt.Sprintf("verify-read: %v", err), err)
		}
		if !baselineChannelEqual(fresh, e.Before) {
			stale := &StaleBaselineError{Slot: e.Slot, Reason: "radio's current content no longer matches what PrepareSend read"}
			report.Slots = append(report.Slots, SlotResult{Slot: e.Slot, Action: actionVerifyReadDrift, Detail: stale.Error()})
			return s.abort(report, journal, e.Slot, stale.Error(), stale)
		}
		s.progress("verify-read", i+1, len(deltas), e.Slot)

		ch := candidateBySlot[e.Slot]
		if _, err := s.writePair(journal, report, i, len(deltas), e, ch); err != nil {
			return report, err
		}
	}

	s.journalAppend(journal, "completion", map[string]any{
		"written": report.Written, "verified": report.Verified,
		"skipped_blocked": report.SkippedBlocked, "unchanged": report.Unchanged,
	})

	return report, nil
}

// writePair runs one delta entry's write+verify pair (obligations 6, 7,
// 8) — write_attempt journal, WriteChannel, write_result journal,
// read-back ReadChannel, verify_result journal — mutating report in
// place. i/total are this delta's 0-based index and the total delta
// count, for progress reporting.
//
// Fix 7 (adjudicated MEDIUM): once write_attempt is durably journaled
// below, WriteChannel and its paired read-back ReadChannel run under an
// INTERNAL context derived from context.Background() (writeVerifyPairTimeout),
// deliberately NOT the caller's Execute ctx. A write without its verify is
// an unknown state — the radio may hold new data this package's
// bookkeeping never confirmed — so once the wire write is underway, a
// caller's ctx cancelling (the user closed the app, a deadline unrelated
// to this specific pair expired) must not be able to abandon it
// unverified. The caller's ctx is still honoured, exactly as before, but
// only BETWEEN pairs — checked at the top of Execute's per-slot loop,
// before writePair is ever called for the NEXT slot.
//
// Returns (report, nil) to continue Execute's loop with the next slot, or
// (report, err) with err a non-nil *AbortedError (already built via
// s.abort) meaning Execute must return immediately with exactly that
// pair — callers should `return report, err` (or, since report is always
// the SAME pointer passed in, equivalently discard the first return and
// use their own report variable).
func (s *Service) writePair(journal journalAppender, report *Report, i, total int, e codeplug.DiffEntry, ch codeplug.Channel) (*Report, error) {
	// write_attempt is the intent-to-write line: FAIL-SAFE per the
	// ratified policy (doc.go). If it cannot be persisted, refuse BEFORE
	// calling WriteChannel — at this instant nothing has touched the
	// radio for this slot, so the refusal is free (no SlotResult is
	// recorded: nothing happened TO record).
	if jfe := s.appendDeltaJournal(journal, "write_attempt", e.Slot, map[string]any{"slot": e.Slot}); jfe != nil {
		return s.abort(report, journal, e.Slot, jfe.Error(), jfe)
	}

	// Fix 7: an internal, caller-independent context for the wire write
	// and its paired verify below — see this method's doc comment.
	pairCtx, cancel := context.WithTimeout(context.Background(), writeVerifyPairTimeout)
	defer cancel()

	res, err := s.sess.WriteChannel(pairCtx, ch)
	// write_result is recorded AFTER the wire write attempt, whatever its
	// outcome — a journal failure here cannot un-write what WriteChannel
	// already did, so it aborts all FURTHER writes (standard abort
	// machinery) rather than refusing for free.
	jfe := s.appendDeltaJournal(journal, "write_result", e.Slot, map[string]any{
		"slot": e.Slot, "mw_sent": res.MWSent, "mw_confirmed": res.MWConfirmed,
		"mt_sent": res.MTSent, "mt_confirmed": res.MTConfirmed,
		"error": errString(err),
	})
	if jfe != nil {
		if err == nil {
			// The wire write itself succeeded — it counts, even though
			// the journal failed to record it.
			report.Written++
			report.Slots = append(report.Slots, SlotResult{Slot: e.Slot, Action: actionWrite, VerifyOK: false, Detail: jfe.Error()})
		} else {
			report.Slots = append(report.Slots, SlotResult{Slot: e.Slot, Action: actionWrite, VerifyOK: false, Detail: err.Error()})
		}
		// Fix 1 (Codex M5b fix wave, adjudicated HIGH): WriteChannel has
		// been invoked — wire contact is possible (indeed, here it
		// definitely happened: err is nil) — so ALWAYS attempt a
		// same-slot readback before aborting, preserving jfe as the
		// abort's cause untouched. See postFailureReadback's doc comment.
		s.postFailureReadback(pairCtx, journal, e.Slot)
		return s.abort(report, journal, e.Slot, jfe.Error(), jfe)
	}
	if err != nil {
		report.Slots = append(report.Slots, SlotResult{Slot: e.Slot, Action: actionWrite, VerifyOK: false, Detail: err.Error()})
		// Fix 1: WriteChannel itself failed or was rejected — MW may
		// still have landed before MT failed/was rejected (exactly the
		// scenario this fix exists for). Always attempt the readback
		// before aborting with the ORIGINAL WriteChannel error preserved
		// as the cause.
		s.postFailureReadback(pairCtx, journal, e.Slot)
		return s.abort(report, journal, e.Slot, fmt.Sprintf("write rejected or failed: %v", err), err)
	}
	report.Written++
	s.progress("write", i+1, total, e.Slot)

	verify, err := s.sess.ReadChannel(pairCtx, e.Slot)
	if err != nil {
		report.Slots = append(report.Slots, SlotResult{Slot: e.Slot, Action: actionWrite, VerifyOK: false, Detail: err.Error()})
		if jfe := s.appendDeltaJournal(journal, "verify_result", e.Slot, map[string]any{"slot": e.Slot, "ok": false, "error": errString(err)}); jfe != nil {
			return s.abort(report, journal, e.Slot, jfe.Error(), jfe)
		}
		return s.abort(report, journal, e.Slot, fmt.Sprintf("verify read-back failed: %v", err), err)
	}

	// obligation 7's write-then-verify comparison — see
	// verifyWriteResult's doc comment for why this is factored out.
	mismatchErr, slotResult := verifyWriteResult(e.Slot, ch, verify)
	verifyDetail := ""
	if mismatchErr != nil {
		verifyDetail = mismatchErr.Error()
	}
	// verify_result, like write_result, is recorded AFTER a completed
	// wire action (the paired verify ReadChannel) — a journal failure
	// here likewise aborts all FURTHER writes rather than refusing for
	// free, but this slot's true outcome (a clean verify, or a genuine
	// mismatch) is still counted and reported accurately; only the run
	// stops early.
	if jfe := s.appendDeltaJournal(journal, "verify_result", e.Slot, map[string]any{
		"slot": e.Slot, "ok": mismatchErr == nil, "error": verifyDetail,
	}); jfe != nil {
		report.Slots = append(report.Slots, slotResult)
		if mismatchErr == nil {
			report.Verified++
		}
		return s.abort(report, journal, e.Slot, jfe.Error(), jfe)
	}

	if mismatchErr != nil {
		report.Slots = append(report.Slots, slotResult)
		return s.abort(report, journal, e.Slot, mismatchErr.Error(), mismatchErr)
	}

	report.Verified++
	report.Slots = append(report.Slots, slotResult)
	s.progress("verify", i+1, total, e.Slot)
	return report, nil
}

// postFailureReadback attempts a best-effort same-slot read-back after a
// write-path failure where wire contact is possible: WriteChannel itself
// returned an error (a rejection or a transport failure, whether or not
// MW already landed), or its write_result journal line failed to persist
// after WriteChannel otherwise succeeded (Codex M5b fix wave, Fix 1,
// adjudicated HIGH). WriteChannel can successfully confirm MW and then
// fail/reject MT: the radio's memory has already changed even though the
// overall write is being reported — and, until now, ABORTED — as a
// failure, with nothing recording what the radio actually holds. Called
// from BOTH of writePair's pre-verify abort points, under the same
// pairCtx the write itself ran under (still independent of the caller's
// ctx — see writeVerifyPairTimeout's doc comment), immediately before
// s.abort.
//
// This is diagnostic, not corrective: it never returns anything, and its
// own outcome (or a failure to even complete the read) is only ever
// journaled via the best-effort journalAppend (event
// "post_failure_readback") — never allowed to influence, replace, or
// wrap the ORIGINAL failure that is writePair's actual abort cause. A
// second failure here (the readback itself erroring, or ITS journal line
// failing to append) is recorded and otherwise ignored; the run was
// already aborting regardless.
func (s *Service) postFailureReadback(ctx context.Context, journal journalAppender, slot string) {
	verify, err := s.sess.ReadChannel(ctx, slot)
	fields := map[string]any{"slot": slot, "ok": err == nil, "error": errString(err)}
	if err == nil {
		fields["empty"] = verify.Empty()
		if !verify.Empty() {
			fields["freq_hz"] = verify.Data.FreqHz
			fields["tag"] = verify.Data.Tag
		}
	}
	s.journalAppend(journal, "post_failure_readback", fields)
}

// abort finalises report as an aborted execution: sets Aborted/AbortReason,
// journals the abort event, and returns the report alongside a non-nil
// *AbortedError wrapping cause — see ErrAborted's doc comment for why both
// a report and an error are returned together for this one case.
func (s *Service) abort(report *Report, journal journalAppender, slot, reason string, cause error) (*Report, error) {
	report.Aborted = true
	report.AbortReason = reason
	s.journalAppend(journal, "abort", map[string]any{"slot": slot, "reason": reason})
	return report, &AbortedError{Slot: slot, Reason: reason, Cause: cause}
}
