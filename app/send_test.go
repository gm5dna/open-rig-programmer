// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/clone"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/internal/fakeradio"
	"github.com/gm5dna/open-rig-programmer/internal/radiotext"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// containsCI reports whether s contains substr, case-insensitively.
func containsCI(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// connectDirect wires a's connectionState directly against sess (built
// via openTestSimSession), bypassing ConnectDemo/internal/wiring so a
// test can inject a custom clone.Progress (defaults to
// a.progressCallback(), the production wiring, when nil). Pre-task-41
// this also gave a test a *fakeradio.Radio handle to assert on
// afterwards; app/ no longer imports core/driver/ft710 at all (even in
// test files — task 41, M9a-5), so a test needing to inspect post-write
// state now reads it back through sess itself (driver.Session.
// ReadChannel), same as send_test.go's own SlotState replacements below.
func connectDirect(t *testing.T, a *App, sess driver.Session, progress clone.Progress) {
	t.Helper()
	if progress == nil {
		progress = a.progressCallback()
	}
	svc := clone.NewService(sess, clone.SnapshotStore{Dir: t.TempDir()}, clone.WithProgress(progress))
	a.mu.Lock()
	a.conn = &connectionState{session: sess, closer: sess.Close, svc: svc, demo: true}
	a.mu.Unlock()
}

// prepareTwoDeltaCandidate builds a working codeplug matching sess's
// minimalFactoryImage baseline exactly, except two edited slots — "001"
// (MEM) and "P1L" (PMS) — each at a new frequency, so PrepareSend
// produces exactly two unblocked Modified deltas without needing a real
// ReadRadio first. Requires sess to have been opened with "P1L" overlaid
// populated (fakeradio.WithSlot("P1L", pmsModifiableSeed)) — the default
// factory image leaves PMS empty (Fix 3), which would make this a
// PMS-ADDED delta instead of Modified.
func prepareTwoDeltaCandidate(sess driver.Session) *codeplug.Codeplug {
	edits := map[string]*codeplug.ChannelData{
		"001": writableChannel("001", 7_100_000, "").Data,
		"P1L": writableChannel("P1L", 14_050_000, "").Data,
	}
	return matchingCandidateFile(sess.Capabilities(), minimalFactoryPopulated(), edits)
}

// TestPrepareSend_ConfirmSend_HappyPath drives the full send lifecycle
// against a real fakeradio session (task-15 brief §5): PrepareSend,
// ConfirmSend's synchronous digest-mismatch pre-check, then a correct
// ConfirmSend, waiting for the async transfer:done event, and asserting
// the fakeradio image actually changed.
func TestPrepareSend_ConfirmSend_HappyPath(t *testing.T) {
	a, rec := newTestApp(t)
	sess := openTestSimSession(t, fakeradio.WithSlot("P1L", pmsModifiableSeed))
	connectDirect(t, a, sess, nil)

	a.mu.Lock()
	a.working = prepareTwoDeltaCandidate(sess)
	a.mu.Unlock()

	planView, err := a.PrepareSend()
	if err != nil {
		t.Fatalf("PrepareSend: unexpected error: %v", err)
	}
	if planView.NothingToSend {
		t.Error("PrepareSend: NothingToSend = true, want false (two edited slots)")
	}
	if !planView.FirmwareRequired {
		t.Error("PrepareSend: FirmwareRequired = false, want true (fresh session, no firmware confirmed yet)")
	}
	// Fix 6 (adjudicated LOW, Codex M6 #6): the FT-710 V01-10 threshold
	// prose lives in Go, not JS — SendPlanView must carry it whenever
	// FirmwareRequired is true.
	if !strings.Contains(planView.FirmwareGuidance, "V01-10") {
		t.Errorf("PrepareSend: FirmwareGuidance = %q, want it to mention V01-10", planView.FirmwareGuidance)
	}
	if planView.ConfirmationDigest == "" || planView.SnapshotPath == "" {
		t.Errorf("PrepareSend: ConfirmationDigest/SnapshotPath empty: %+v", planView)
	}
	if len(planView.Diff.Modified) != 2 {
		t.Errorf("PrepareSend: Diff.Modified = %+v, want 2 entries", planView.Diff.Modified)
	}

	// Digest-mismatch pre-check: refused synchronously, no transfer
	// started, no events emitted, the plan stays active.
	err = a.ConfirmSend("not-the-right-digest", "V01-10")
	var mismatch *DigestMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("ConfirmSend(wrong digest): err = %v, want *DigestMismatchError", err)
	}
	if a.transferRunning() {
		t.Error("ConfirmSend(wrong digest): transfer started, want it refused before starting")
	}
	a.mu.Lock()
	stillActive := a.currentPlan != nil
	a.mu.Unlock()
	if !stillActive {
		t.Error("ConfirmSend(wrong digest): currentPlan cleared, want it to remain active for a retry")
	}
	if len(rec.named("transfer:done")) != 0 {
		t.Error("ConfirmSend(wrong digest): transfer:done emitted, want none")
	}

	if err := a.ConfirmSend(planView.ConfirmationDigest, "V01-10"); err != nil {
		t.Fatalf("ConfirmSend: unexpected error: %v", err)
	}

	ev := waitForTransferDone(t, rec, "send", 10*time.Second)
	if ev.Outcome != "ok" {
		t.Fatalf("transfer:done Outcome = %q, want %q (message: %s)", ev.Outcome, "ok", ev.Message)
	}
	if ev.Report == nil {
		t.Fatal("transfer:done Report = nil, want a populated report")
	}
	if ev.Report.Written != 2 || ev.Report.Verified != 2 {
		t.Errorf("transfer:done Report Written/Verified = %d/%d, want 2/2", ev.Report.Written, ev.Report.Verified)
	}
	if ev.Report.SnapshotPath == "" || ev.Report.JournalPath == "" {
		t.Errorf("transfer:done Report paths empty: %+v", ev.Report)
	}

	// The radio itself must reflect the write — read back through the
	// session driver-neutrally (task 41, M9a-5: no *fakeradio.Radio handle
	// is available here any more — see connectDirect's doc comment).
	for _, tt := range []struct {
		slot   string
		wantHz uint32
	}{
		{"001", 7_100_000},
		{"P1L", 14_050_000},
	} {
		ch, err := sess.ReadChannel(a.ctx, tt.slot)
		if err != nil {
			t.Errorf("ReadChannel(%s) after send: unexpected error: %v", tt.slot, err)
			continue
		}
		if ch.Data == nil || ch.Data.FreqHz != tt.wantHz {
			t.Errorf("ReadChannel(%s) after send = %+v, want FreqHz=%d", tt.slot, ch.Data, tt.wantHz)
		}
	}

	// Post-transfer App state: plan cleared, baseline marked stale,
	// firmware confirmation persisted for future PrepareSend predictions.
	a.mu.Lock()
	planCleared := a.currentPlan == nil
	a.mu.Unlock()
	if !planCleared {
		t.Error("currentPlan not cleared after ConfirmSend completed")
	}
	view, err := a.GetCodeplug()
	if err != nil {
		t.Fatalf("GetCodeplug: %v", err)
	}
	if !view.BaselineStale {
		t.Error("CodeplugView.BaselineStale = false after a successful send, want true")
	}
	if view.Radio.FirmwareConfirmed != "V01-10" {
		t.Errorf("CodeplugView.Radio.FirmwareConfirmed = %q, want %q", view.Radio.FirmwareConfirmed, "V01-10")
	}

	if err := a.ConfirmSend(planView.ConfirmationDigest, "V01-10"); !errors.Is(err, ErrNoActivePlan) {
		t.Errorf("ConfirmSend after completion: err = %v, want ErrNoActivePlan", err)
	}
}

// TestConfirmSend_CancelMidTransfer_AndBusyExclusion drives a
// deterministic mid-transfer cancellation: a custom clone.Progress hook
// calls CancelTransfer (and, before that, probes the busy/transfer-
// running guards) from INSIDE Execute's own delta-write loop, at the
// verify-read callback for the FIRST of two delta slots — guaranteeing
// (task-15 brief §5's "boundary cancel", "assert what clone actually
// does"): slot 1 completes (verify-read for it already passed the
// ctx-check boundary before the hook runs), slot 2 is never attempted
// (ctx.Err() is checked at the TOP of the loop, before slot 2's own
// verify-read). No sleeps/timing races: the hook runs synchronously on
// Execute's own goroutine, so calling ReadRadio/DiffAgainstRadio/
// LoadFile/Disconnect from inside it deterministically observes svc's
// op lock held (clone.ErrBusy) and a.transfer.running == true.
func TestConfirmSend_CancelMidTransfer_AndBusyExclusion(t *testing.T) {
	a, rec := newTestApp(t)
	sess := openTestSimSession(t, fakeradio.WithSlot("P1L", pmsModifiableSeed))

	var (
		hookFired     bool
		busyErrs      []error
		loadFileErr   error
		disconnectErr error
	)
	hook := func(phase string, done, total int, slot string) {
		a.emit("transfer:progress", ProgressEvent{Phase: phase, Done: done, Total: total, Slot: codeplug.DisplaySlot(slot)})
		if hookFired || phase != "verify-read" || done != 1 {
			return
		}
		hookFired = true

		// Busy exclusion: svc's own op lock is held for the whole
		// Execute call — a reentrant ReadRadio/DiffAgainstRadio on the
		// SAME Service must be refused with a friendly *clone.BusyError
		// wrap, not deadlock or silently interleave.
		if _, err := a.ReadRadio(); err == nil {
			busyErrs = append(busyErrs, fmt.Errorf("ReadRadio during transfer: got nil error, want a busy error"))
		} else if !errors.Is(err, clone.ErrBusy) {
			busyErrs = append(busyErrs, fmt.Errorf("ReadRadio during transfer: err = %v, want errors.Is(_, clone.ErrBusy)", err))
		}
		if _, err := a.DiffAgainstRadio(); err == nil {
			busyErrs = append(busyErrs, fmt.Errorf("DiffAgainstRadio during transfer: got nil error, want a busy error"))
		} else if !errors.Is(err, clone.ErrBusy) {
			busyErrs = append(busyErrs, fmt.Errorf("DiffAgainstRadio during transfer: err = %v, want errors.Is(_, clone.ErrBusy)", err))
		}

		// App-level transfer-running guards (task-15 brief §2): these do
		// NOT go through svc, so they need a.transfer.running itself,
		// which ConfirmSend set before spawning this goroutine.
		_, loadFileErr = a.LoadFile()
		disconnectErr = a.Disconnect()

		if err := a.CancelTransfer(); err != nil {
			busyErrs = append(busyErrs, fmt.Errorf("CancelTransfer from progress hook: %v", err))
		}
	}
	connectDirect(t, a, sess, hook)

	a.mu.Lock()
	a.working = prepareTwoDeltaCandidate(sess)
	a.mu.Unlock()

	planView, err := a.PrepareSend()
	if err != nil {
		t.Fatalf("PrepareSend: unexpected error: %v", err)
	}
	if len(planView.Diff.Modified) != 2 {
		t.Fatalf("PrepareSend: Diff.Modified = %+v, want 2 entries (test needs two deltas for a between-slots cancel)", planView.Diff.Modified)
	}

	if err := a.ConfirmSend(planView.ConfirmationDigest, "V01-10"); err != nil {
		t.Fatalf("ConfirmSend: unexpected error: %v", err)
	}

	ev := waitForTransferDone(t, rec, "send", 10*time.Second)

	if !hookFired {
		t.Fatal("progress hook never fired at verify-read done==1 — test setup is broken, every assertion above would be vacuous")
	}
	for _, e := range busyErrs {
		t.Error(e)
	}
	if !errors.Is(loadFileErr, ErrTransferRunning) {
		t.Errorf("LoadFile during transfer: err = %v, want ErrTransferRunning", loadFileErr)
	}
	if !errors.Is(disconnectErr, ErrTransferRunning) {
		t.Errorf("Disconnect during transfer: err = %v, want ErrTransferRunning", disconnectErr)
	}

	// Assert what clone actually does (task-15 brief §5): a cancellation
	// observed between slots, after the in-flight slot's write+verify
	// pair already completed, surfaces as a *clone.AbortedError wrapping
	// context.Canceled — classified "cancelled", not "aborted" (see
	// classifyExecuteOutcome's doc comment).
	if ev.Outcome != "cancelled" {
		t.Fatalf("transfer:done Outcome = %q, want %q (message: %s)", ev.Outcome, "cancelled", ev.Message)
	}
	if ev.Report == nil {
		t.Fatal("transfer:done Report = nil, want the partial report (one slot completed)")
	}
	if ev.Report.Written != 1 || ev.Report.Verified != 1 {
		t.Errorf("transfer:done Report Written/Verified = %d/%d, want 1/1 (exactly one slot completed before the cancel boundary)", ev.Report.Written, ev.Report.Verified)
	}
	if !ev.Report.Aborted {
		t.Error("transfer:done Report.Aborted = false, want true")
	}
	if len(ev.Report.Slots) != 1 {
		t.Fatalf("transfer:done Report.Slots = %+v, want exactly one entry (the second delta was never attempted)", ev.Report.Slots)
	}
	completedSlot := ev.Report.Slots[0].Slot
	otherSlot := "P1L"
	if completedSlot == "P1L" {
		otherSlot = "001"
	}

	// The completed slot must show the new value, read back through the
	// session driver-neutrally (task 41, M9a-5 — see connectDirect's doc
	// comment); the untouched one must still show its ORIGINAL (pre-edit)
	// value.
	completedCh, err := sess.ReadChannel(a.ctx, completedSlot)
	wantHz := map[string]uint32{"001": 7_100_000, "P1L": 14_050_000}[completedSlot]
	if err != nil || completedCh.Data == nil || completedCh.Data.FreqHz != wantHz {
		t.Errorf("ReadChannel(%s) (the completed slot) = %+v (err=%v), want FreqHz=%d", completedSlot, completedCh.Data, err, wantHz)
	}
	otherCh, err := sess.ReadChannel(a.ctx, otherSlot)
	origHz := map[string]uint32{"001": 7_000_000, "P1L": 14_000_000}[otherSlot]
	if err != nil || otherCh.Data == nil || otherCh.Data.FreqHz != origHz {
		t.Errorf("ReadChannel(%s) (untouched — never attempted before the cancel) = %+v (err=%v), want FreqHz=%d", otherSlot, otherCh.Data, err, origHz)
	}

	// Message must say "snapshot", never "backup" (task-15's hard
	// constraint).
	if !containsCI(ev.Message, "snapshot") {
		t.Errorf("transfer:done Message = %q, want it to mention 'snapshot'", ev.Message)
	}
	if containsCI(ev.Message, "backup") {
		t.Errorf("transfer:done Message = %q, must never say 'backup'", ev.Message)
	}

	a.mu.Lock()
	planCleared := a.currentPlan == nil
	transferIdle := !a.transfer.running
	a.mu.Unlock()
	if !planCleared {
		t.Error("currentPlan not cleared after a cancelled ConfirmSend")
	}
	if !transferIdle {
		t.Error("transfer.running still true after transfer:done fired")
	}
}

// TestConfirmSend_ConcurrentCalls_ReserveAtomically reproduces the
// ConfirmSend double-start TOCTOU (task-15 fix-wave review, Fix 1): the
// pre-fix implementation checks a.transfer.running under a.mu, RELEASES
// the lock to compare the confirmation digest, then re-locks and sets
// running=true unconditionally — so two (or more) concurrent
// ConfirmSend calls can all observe running==false before any of them
// reserves the slot. Every one that does then spawns its own Execute
// goroutine; clone.Service's own try-lock (acquireOp) lets only one
// actually touch the radio, and every "loser" fails fast with a
// *clone.BusyError — but the loser's completion block unconditionally
// clears a.transfer/a.currentPlan, corrupting the still-running
// winner's state (OnBeforeClose stops preventing close, Disconnect
// stops refusing) while a real send is still in flight.
//
// n concurrent calls (not just two) are launched together off a closed
// start gate — no sleeps-as-synchronisation — to make the race
// reliable: SHA-256'ing the confirmation digest between the check and
// the reservation (pre-fix) is easily slow enough, relative to a
// mutex lock/unlock, for most or all of them to pass the check before
// the first one re-locks. The winning transfer is then held open
// deterministically via the same mid-transfer progress-hook technique
// TestConfirmSend_CancelMidTransfer_AndBusyExclusion uses, so the
// state-guard assertions below observe a transfer that is GENUINELY
// still in flight, not a race against its own completion.
func TestConfirmSend_ConcurrentCalls_ReserveAtomically(t *testing.T) {
	a, rec := newTestApp(t)
	sess := openTestSimSession(t)

	winnerBlocked := make(chan struct{})
	releaseWinner := make(chan struct{})
	var hookOnce sync.Once
	hook := func(phase string, done, total int, slot string) {
		a.emit("transfer:progress", ProgressEvent{Phase: phase, Done: done, Total: total, Slot: codeplug.DisplaySlot(slot)})
		if phase != "verify-read" || done != 1 {
			return
		}
		hookOnce.Do(func() {
			close(winnerBlocked)
			<-releaseWinner
		})
	}
	connectDirect(t, a, sess, hook)

	a.mu.Lock()
	a.working = prepareTwoDeltaCandidate(sess)
	a.mu.Unlock()

	planView, err := a.PrepareSend()
	if err != nil {
		t.Fatalf("PrepareSend: unexpected error: %v", err)
	}

	const n = 16
	start := make(chan struct{})
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			errs[i] = a.ConfirmSend(planView.ConfirmationDigest, "V01-10")
		}(i)
	}
	close(start)
	wg.Wait()

	nilCount := 0
	for _, e := range errs {
		if e == nil {
			nilCount++
			continue
		}
		if !errors.Is(e, ErrTransferRunning) {
			t.Errorf("ConfirmSend concurrent call: err = %v, want nil or ErrTransferRunning", e)
		}
	}
	if nilCount != 1 {
		t.Errorf("ConfirmSend: %d of %d concurrent calls were accepted (nil error), want exactly 1 (atomic reservation)", nilCount, n)
	}

	select {
	case <-winnerBlocked:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the winning transfer to reach the mid-transfer hook")
	}

	// The one accepted transfer is genuinely still in flight (blocked in
	// the hook, holding clone's op lock) — every state guard must still
	// see it as busy, never cleared by a loser's completion block.
	if !a.transferRunning() {
		t.Error("a.transferRunning() = false while the winning transfer is still blocked mid-transfer — a loser's completion block cleared it")
	}
	if !a.OnBeforeClose(context.Background()) {
		t.Error("OnBeforeClose = false while a transfer is still in flight, want true (prevent close)")
	}

	close(releaseWinner)
	ev := waitForTransferDone(t, rec, "send", 10*time.Second)
	if ev.Outcome != "ok" {
		t.Errorf("transfer:done Outcome = %q, want %q (message: %s)", ev.Outcome, "ok", ev.Message)
	}

	// Tightened double-emit coverage (task-15 fix-wave review): exactly
	// one transfer:done of Kind "send" must ever be emitted for this one
	// accepted transfer, however many losers raced for the slot.
	sendDoneCount := 0
	for _, e := range rec.named("transfer:done") {
		if tv, ok := e.data.(TransferDoneEvent); ok && tv.Kind == "send" {
			sendDoneCount++
		}
	}
	if sendDoneCount != 1 {
		t.Errorf("transfer:done Kind=send count = %d, want exactly 1", sendDoneCount)
	}
}

// TestFirmwareGuidance_FollowsResolvedModel is the send cluster's
// threading pin (M9c-5 E4): the firmware advisory PrepareSend attaches is
// keyed off the resolved model, and radiotext.For's ok is honoured — a
// model with no entry yields "" (no advisory shown at all), never the
// FT-710's own firmware sentence attributed to a different radio.
func TestFirmwareGuidance_FollowsResolvedModel(t *testing.T) {
	want, ok := radiotext.For(wiring.DefaultModel)
	if !ok || want.FirmwareGuidance == "" {
		t.Fatalf("test setup: radiotext.For(%q) ok=%v with empty FirmwareGuidance — the contrast below would be vacuous", wiring.DefaultModel, ok)
	}
	if got := firmwareGuidance(wiring.DefaultModel); got != want.FirmwareGuidance {
		t.Errorf("firmwareGuidance(%q) = %q, want %q", wiring.DefaultModel, got, want.FirmwareGuidance)
	}
	if got := firmwareGuidance("NoSuchRadioModel"); got != "" {
		t.Errorf("firmwareGuidance(unknown model) = %q, want \"\" (silence, never another radio's advisory)", got)
	}
}
