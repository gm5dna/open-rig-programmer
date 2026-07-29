// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/clone"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/internal/radiotext"
)

// isCancelled reports whether err is (or wraps) a context cancellation —
// the app-local equivalent of cmd/rigprog/signal.go's isCancelled,
// restated here since that one is unexported in a package this one must
// not import.
func isCancelled(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// progressCallback returns the clone.Progress closure wired into every
// Service this App constructs (Connect/ConnectDemo): one transfer:progress
// event per callback. clone.Progress's fourth parameter (name) carries
// EITHER a channel slot (every phase ReadAll/PrepareSend/Execute use) OR,
// for phase "read-settings" (clone.Service.ReadSettings, task 33), a
// 6-digit setting ID — see that phase string's own doc comment
// (core/clone/settings.go). This callback tells the two apart by phase and
// populates ProgressEvent's fields accordingly (see that type's own doc
// comment, types.go, for exactly which fields each branch sets):
//
//   - any OTHER phase (a channel event): Slot stays exactly as it always
//     was (codeplug.DisplaySlot(name), JS compatibility), TargetKind
//     "channel", TargetID the WIRE-form slot (name, unchanged), and
//     TargetDisplay the SAME DisplaySlot value Slot carries.
//   - phase "read-settings" (a settings event): Slot is left empty (there
//     is no slot), TargetKind "setting", TargetID the setting ID (name,
//     unchanged), and TargetDisplay looked up from a.settingsDisplay — a
//     map ReadSettingsRadio builds ONCE, at the start of its own call
//     (see that method's doc comment, settings.go), rather than this
//     closure re-scanning the whole settings descriptor for every one of
//     up to 296 progress events a single read produces.
func (a *App) progressCallback() clone.Progress {
	return func(phase string, done, total int, name string) {
		if phase == "read-settings" {
			a.mu.Lock()
			display := a.settingsDisplay[name]
			a.mu.Unlock()
			a.emit("transfer:progress", ProgressEvent{
				Phase: phase, Done: done, Total: total,
				TargetKind: "setting", TargetID: name, TargetDisplay: display,
			})
			return
		}
		displaySlot := codeplug.DisplaySlot(name)
		a.emit("transfer:progress", ProgressEvent{
			Phase: phase, Done: done, Total: total,
			Slot:          displaySlot,
			TargetKind:    "channel",
			TargetID:      name,
			TargetDisplay: displaySlot,
		})
	}
}

// emitDone emits transfer:done exactly once for one ReadRadio/
// DiffAgainstRadio/ReadSettingsRadio call or one ConfirmSend transfer
// (task-15 brief §2; ReadSettingsRadio added by task 35).
func (a *App) emitDone(kind, outcome string, report *ReportView, message string) {
	a.emit("transfer:done", TransferDoneEvent{Kind: kind, Outcome: outcome, Report: report, Message: message})
}

// classifyReadDiffOutcome maps a ReadAll/Diff-path error to the "ok"/
// "error"/"cancelled" subset of transfer:done's Outcome values that can
// actually arise there (no write ever happens, so "aborted"/"refused"
// never apply) — reusing the CLASSIFICATION intent of cmd/rigprog's own
// isCancelled-first, then-everything-else handling in read.go/diff.go,
// not its exit codes.
func classifyReadDiffOutcome(err error) (outcome, message string) {
	if err == nil {
		return "ok", ""
	}
	if isCancelled(err) {
		return "cancelled", "cancelled"
	}
	var busy *clone.BusyError
	if errors.As(err, &busy) {
		return "error", fmt.Sprintf("another operation is running (%s)", busy.InProgress)
	}
	return "error", err.Error()
}

// isExecuteRefusalSentinel mirrors cmd/rigprog/write.go's function of
// the same name (restated here — that one is unexported in a package
// this one must not import): Execute's documented pre-write refusal
// sentinels, PLUS *clone.ValidationFailedError (Execute's own
// re-validation at the execute boundary) — the CLI keeps that one in a
// separate "exitBlocked" bucket, but transfer:done's Outcome enum has no
// separate "blocked" value (task-15 brief §2 lists exactly ok/aborted/
// refused/error/cancelled), so it is grouped here with the other
// pre-write, nothing-was-sent refusals.
func isExecuteRefusalSentinel(err error) bool {
	return errors.Is(err, clone.ErrStaleBaseline) ||
		errors.Is(err, clone.ErrSessionChanged) ||
		errors.Is(err, clone.ErrCandidateChanged) ||
		errors.Is(err, clone.ErrConfirmationMismatch) ||
		errors.Is(err, clone.ErrFirmwareUnconfirmed) ||
		errors.Is(err, clone.ErrValidationFailed)
}

// refusalMessage maps one of isExecuteRefusalSentinel's errors to a
// plain-language line, mirroring cmd/rigprog/write.go's refusalMessage
// (GUI wording, not CLI's "re-run write").
func refusalMessage(err error) string {
	switch {
	case errors.Is(err, clone.ErrStaleBaseline):
		return "refused: the radio's contents changed after the plan was prepared — read the radio and prepare send again"
	case errors.Is(err, clone.ErrSessionChanged):
		return "refused: the radio session changed since the plan was prepared (a reconnect?) — prepare send again"
	case errors.Is(err, clone.ErrCandidateChanged):
		return "refused: an internal consistency check failed (candidate changed) — prepare send again"
	case errors.Is(err, clone.ErrConfirmationMismatch):
		return "refused: the confirmation did not match the plan that was reviewed — prepare send again"
	case errors.Is(err, clone.ErrFirmwareUnconfirmed):
		return "refused: firmware version not confirmed for this session's first send"
	case errors.Is(err, clone.ErrValidationFailed):
		return "refused: candidate codeplug failed validation at send time — validate and prepare send again"
	default:
		return err.Error()
	}
}

// displaySlotOrDash returns codeplug.DisplaySlot(slot), or "-" for an
// empty slot (an abort that happened BETWEEN slots, not at one).
func displaySlotOrDash(slot string) string {
	if slot == "" {
		return "-"
	}
	return codeplug.DisplaySlot(slot)
}

// classifyExecuteOutcome maps ConfirmSend's Execute result to
// transfer:done's Outcome/Message, reusing the CLASSIFICATION intent of
// cmd/rigprog/write.go's handleExecuteOutcome (not its exit codes — see
// that function's doc comment for the CLI's own error-class-by-class
// reasoning, which this mirrors):
//
//   - nil: "ok".
//   - *clone.AbortedError wrapping a context cancellation (CancelTransfer
//     was honoured at the next verified channel boundary — see
//     CancelTransfer's doc comment): "cancelled". The radio may already
//     hold a partial write; report is still attached.
//   - *clone.AbortedError for any other cause (verify mismatch, write
//     rejection, journal failure): "aborted". Messaging always says
//     "snapshot", never "backup" (task-15 brief's hard constraint).
//   - a bare cancelled error (ctx was already cancelled before Execute's
//     delta loop even started — no AbortedError, no report): "cancelled".
//   - *clone.BusyError: "error" (an operational failure, not a content
//     refusal — matches the CLI's own classification).
//   - isExecuteRefusalSentinel(err): "refused".
//   - anything else: "error".
func classifyExecuteOutcome(err error) (outcome, message string) {
	if err == nil {
		return "ok", ""
	}

	var aborted *clone.AbortedError
	if errors.As(err, &aborted) {
		if isCancelled(aborted.Cause) {
			return "cancelled", fmt.Sprintf(
				"transfer cancelled at slot %s: the in-flight write+verify pair completed before the cancellation was honoured — see the journal for exactly what happened, and the snapshot for the radio's contents before this send",
				displaySlotOrDash(aborted.Slot))
		}
		return "aborted", fmt.Sprintf(
			"transfer aborted at slot %s: %s — the radio was partially written; the snapshot records its contents before this send, and the journal records exactly what happened",
			displaySlotOrDash(aborted.Slot), aborted.Reason)
	}

	if isCancelled(err) {
		return "cancelled", "transfer cancelled before any write began"
	}

	var busy *clone.BusyError
	if errors.As(err, &busy) {
		return "error", fmt.Sprintf("another operation is running (%s)", busy.InProgress)
	}

	if isExecuteRefusalSentinel(err) {
		return "refused", refusalMessage(err)
	}

	return "error", err.Error()
}

// firmwareGuidance returns the radio-specific advisory PrepareSend attaches
// to SendPlanView.FirmwareGuidance whenever firmwareRequiredLocked predicts
// true — moved out of the frontend (Fix 6, adjudicated LOW, Codex M6 #6:
// "no FT-710 protocol facts in the frontend") so the V01-10 threshold and
// CAT's lack of a firmware-version query live in exactly one place, next
// to firmwareRequiredLocked's own FT-710 knowledge below, rather than
// duplicated as JS prose the frontend previously hardcoded. Restates
// core/clone's firmware-gate intent (see clone.ErrFirmwareUnconfirmed and
// ExecuteOptions.FirmwareConfirmed's doc comments) in user-facing wording —
// core/clone itself deliberately carries no display strings.
//
// Task 41 (M9a-5, the GUI-backend neutralisation) sources this from
// internal/radiotext rather than a hardcoded const — the same served
// value, byte-identical to the old const, for the FT-710.
//
// M9c-5 (E4) keys it off model — currentModel's resolved answer, taken
// under a.mu by the caller — instead of wiring.DefaultModel, and HONOURS
// radiotext.For's ok: a model with no radiotext entry yields "" (the
// frontend then shows no advisory at all), never the FT-710's own
// firmware sentence attributed to some other radio. The same silence-on-
// false rule cmd/rigprog's prose sites follow.
func firmwareGuidance(model string) string {
	text, ok := radiotext.For(model)
	if !ok {
		return ""
	}
	return text.FirmwareGuidance
}

// firmwareRequiredLocked is PrepareSend's best-effort prediction of
// whether Execute will need ExecuteOptions.FirmwareConfirmed — see
// SendPlanView.FirmwareRequired's doc comment for why this cannot be
// authoritative. Callers must hold a.mu.
func (a *App) firmwareRequiredLocked() bool {
	workingConfirmed := ""
	if a.working != nil {
		workingConfirmed = a.working.Radio.FirmwareConfirmed
	}
	baselineConfirmed := ""
	if a.baseline != nil {
		baselineConfirmed = a.baseline.Radio.FirmwareConfirmed
	}
	return workingConfirmed == "" && baselineConfirmed == ""
}

// PrepareSend builds a send plan (svc.PrepareSend against a DEEP COPY of
// working) and stores it as the active plan for a subsequent ConfirmSend.
// Synchronous: it does not emit transfer:done (only ReadRadio/
// DiffAgainstRadio/ConfirmSend's async transfer do — see
// TransferDoneEvent.Kind's doc comment), but DOES emit transfer:progress
// during PrepareSend's own internal fresh read.
//
// Fix 2 (adjudicated HIGH, Codex M6 #2): the old shape captured
// a.working's own pointer under a quick lock and passed it, still live,
// to svc.PrepareSend OUTSIDE mu for the whole call — concurrent with
// UpdateChannel(s) mutating that same struct under mu, that was a
// genuine unsynchronized read/write (and, semantically, a torn plan even
// without the race detector's help). Now takes an independent deep copy
// under mu before releasing it. ALSO reserves the App-level exclusive-
// operation slot (a.opBusy) for its whole duration, refusing
// UpdateChannel(s)/SaveFile*/LoadFile*/Import*/Export*/Disconnect/
// another radio op with a typed busy error meanwhile — belt and braces
// with the deep copy (either alone would fix the race; both together
// also close the lost-edit/torn-plan window a race-clean but unreserved
// PrepareSend would still leave open). The pre-existing explicit
// transfer.running check is kept EXACTLY as before (still ErrTransferRunning,
// checked before the new reservation) — see reserveOpLocked's doc
// comment for why this method alone (unlike ReadRadio/DiffAgainstRadio/
// ReadSettingsRadio) has always special-cased a running send this way.
func (a *App) PrepareSend() (SendPlanView, error) {
	a.mu.Lock()
	conn := a.conn
	if conn == nil {
		a.mu.Unlock()
		return SendPlanView{}, ErrNotConnected
	}
	if a.working == nil {
		a.mu.Unlock()
		return SendPlanView{}, ErrNothingLoaded
	}
	if a.transfer.running {
		a.mu.Unlock()
		return SendPlanView{}, ErrTransferRunning
	}
	if err := a.reserveOpLocked("PrepareSend"); err != nil {
		a.mu.Unlock()
		return SendPlanView{}, err
	}
	workingCopy := deepCopyCodeplug(a.working)
	a.mu.Unlock()
	defer a.releaseOp()

	plan, err := conn.svc.PrepareSend(a.ctx, workingCopy)
	if err != nil {
		return SendPlanView{}, fmt.Errorf("app: preparing send: %w", friendlyErr(err))
	}

	a.mu.Lock()
	a.currentPlan = plan
	firmwareRequired := a.firmwareRequiredLocked()
	// Resolved under a.mu (currentModel reads a.conn/a.working), used
	// outside it: the string it returns is a value, not a live view of
	// App state.
	model := currentModel(a.conn, a.working)
	a.mu.Unlock()

	guidance := ""
	if firmwareRequired {
		guidance = firmwareGuidance(model)
	}

	diff := plan.Diff()
	return SendPlanView{
		Diff:                buildDiffSummary(diff),
		SnapshotPath:        plan.SnapshotPath(),
		BaselineDigestShort: truncateDigest(plan.BaselineDigest()),
		ConfirmationDigest:  plan.ConfirmationDigest(),
		NothingToSend:       countSendable(diff) == 0,
		FirmwareRequired:    firmwareRequired,
		FirmwareGuidance:    guidance,
	}, nil
}

// truncateDigest returns digest's first 12 hex characters (matching
// cmd/rigprog/read.go's truncateDigest — restated here, unexported
// there), or digest unchanged if already that short or shorter.
func truncateDigest(digest string) string {
	const n = 12
	if len(digest) <= n {
		return digest
	}
	return digest[:n]
}

// countSendable mirrors cmd/rigprog/write.go's countSendable (restated
// here, unexported there): how many of diff's entries are Added or
// Modified and NOT Blocked.
func countSendable(diff codeplug.DiffResult) int {
	n := 0
	for _, e := range diff.Entries {
		if e.Blocked {
			continue
		}
		if e.Kind == codeplug.DiffAdded || e.Kind == codeplug.DiffModified {
			n++
		}
	}
	return n
}

// ConfirmSend starts the transfer ASYNCHRONOUSLY. It reserves the
// transfer slot (a.transfer.running=true) in the SAME critical section
// as the running check, so two concurrent ConfirmSend calls can never
// both proceed past it — a prior shape checked running, released the
// lock to compare the confirmation digest, and only then re-locked to
// set running=true, letting both callers pass the check before either
// reserved the slot; the loser's Execute would fail fast against
// clone's own try-lock with a *BusyError, but its completion block
// would then unconditionally clear a.transfer/a.currentPlan out from
// under the still-running winner (see task-15 fix-wave report, Fix 1).
// With the reservation atomic, a loser is refused with
// ErrTransferRunning immediately and never spawns a goroutine at all.
//
// Because the reservation now comes first, every pre-flight check that
// follows — conn/plan present, and the confirmation digest matching the
// active plan (failing fast, before ever touching the radio: Execute
// would refuse identically, but this avoids spawning a doomed
// goroutine) — releases the reservation again on failure. Only once
// every check has passed does it run Execute in a goroutine with a
// cancellable context derived from a.ctx, storing that cancel func as
// the transfer state CancelTransfer uses. Returns immediately once the
// goroutine is started; the eventual outcome arrives via transfer:done
// (Kind "send").
func (a *App) ConfirmSend(confirmationDigest, firmware string) error {
	a.mu.Lock()
	if a.transfer.running {
		a.mu.Unlock()
		return ErrTransferRunning
	}
	// Fix 2 (adjudicated HIGH, Codex M6 #2): ConfirmSend is one of the
	// four operations the remedy names as reserving/respecting the
	// App-level exclusive-operation slot ("ReadRadio, DiffAgainstRadio,
	// PrepareSend, and ConfirmSend reserve exclusively" — that adjudicated
	// text predates task 35, which added a fifth holder, ReadSettingsRadio,
	// to the same reservation; the quote is left verbatim as the
	// historical record) — refused if a concurrently-running ReadRadio/
	// DiffAgainstRadio/PrepareSend/ReadSettingsRadio holds a.opBusy, so a
	// NEW PrepareSend can never race a ConfirmSend still acting on an
	// OLDER plan.
	if a.opBusy != "" {
		busy := &OperationBusyError{InProgress: a.opBusy}
		a.mu.Unlock()
		return busy
	}
	conn := a.conn
	plan := a.currentPlan
	a.transfer = transferState{running: true}
	a.mu.Unlock()

	release := func() {
		a.mu.Lock()
		a.transfer = transferState{}
		a.mu.Unlock()
	}

	if conn == nil {
		release()
		return ErrNotConnected
	}
	if plan == nil {
		release()
		return ErrNoActivePlan
	}
	if want := plan.ConfirmationDigest(); confirmationDigest != want {
		release()
		return &DigestMismatchError{Want: want, Got: confirmationDigest}
	}

	ctx, cancel := context.WithCancel(a.ctx)
	a.mu.Lock()
	a.transfer = transferState{running: true, cancel: cancel}
	a.mu.Unlock()

	snapshotPath := plan.SnapshotPath()
	go func() {
		report, err := conn.svc.Execute(ctx, plan, confirmationDigest, clone.ExecuteOptions{FirmwareConfirmed: firmware})
		cancel()

		outcome, message := classifyExecuteOutcome(err)
		var reportView *ReportView
		if report != nil {
			rv := reportToView(report, snapshotPath)
			reportView = &rv
		}

		a.mu.Lock()
		a.transfer = transferState{}
		a.currentPlan = nil
		if report != nil {
			// The radio was touched (fully or partially) — the App's
			// cached baseline may no longer reflect the radio's true
			// state. It is deliberately NOT recomputed here (Execute's
			// candidate is not necessarily what the radio now holds on
			// an abort) — see CodeplugView.BaselineStale's doc comment.
			a.baselineStale = true
		}
		if report != nil && report.FirmwareConfirmed != "" {
			if a.working != nil {
				a.working.Radio.FirmwareConfirmed = report.FirmwareConfirmed
				a.bumpWorkingRevLocked() // serialised content changed: a concurrent Save must not clear dirty against a stale snapshot
			}
			if a.baseline != nil {
				a.baseline.Radio.FirmwareConfirmed = report.FirmwareConfirmed
			}
		}
		a.mu.Unlock()

		a.emitDone("send", outcome, reportView, message)
	}()

	return nil
}

// CancelTransfer cancels the running transfer's context. clone honours
// cancellation only BETWEEN verified channel boundaries (core/clone/
// execute.go: ctx is checked at the top of each delta-write loop
// iteration, never within one slot's own verify-read/write/verify
// sequence) — so a cancelled transfer's in-flight slot still completes
// its write+verify pair before Execute stops; the eventual transfer:done
// event's Outcome will be "cancelled" for that case, or "aborted" if a
// different cause won the race. Returns ErrNoTransferRunning if nothing
// is running.
func (a *App) CancelTransfer() error {
	a.mu.Lock()
	cancel := a.transfer.cancel
	running := a.transfer.running
	a.mu.Unlock()

	if !running || cancel == nil {
		return ErrNoTransferRunning
	}
	cancel()
	return nil
}
