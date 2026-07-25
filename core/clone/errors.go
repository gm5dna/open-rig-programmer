// SPDX-License-Identifier: GPL-3.0-or-later

package clone

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// ErrStaleBaseline is the sentinel a caller should compare against (via
// errors.Is) when a baseline no longer matches what Execute is about to act
// on. This covers TWO distinct checks, both refusals before any write:
//
//   - the dual-digest recheck (obligation 3): Digest recomputed over the
//     plan's own private baseline copy no longer matches the digest stored
//     in the plan at PrepareSend time — a self-consistency assertion that
//     should be unreachable outside a bug, since the plan's baseline is
//     never mutated after PrepareSend;
//   - the verify-read phase (obligation 11): a fresh read of a
//     to-be-written slot, taken immediately before the delta-write loop,
//     disagrees with what the plan's baseline recorded for that slot —
//     someone or something changed the radio's memory between PrepareSend
//     and Execute. StaleBaselineError.Slot names the drifted slot for this
//     case; it is empty for the whole-plan digest recheck.
//
// The error actually returned is always a *StaleBaselineError.
var ErrStaleBaseline = fmt.Errorf("clone: baseline no longer matches what Execute is about to act on")

// StaleBaselineError reports why Execute refused: see ErrStaleBaseline for
// the two cases this covers.
type StaleBaselineError struct {
	// Slot is the specific slot a verify-read phase found drifted, or ""
	// for a whole-plan digest mismatch.
	Slot string
	// Reason is a human-readable explanation.
	Reason string
}

// Error implements the error interface.
func (e *StaleBaselineError) Error() string {
	if e.Slot == "" {
		return fmt.Sprintf("clone: stale baseline: %s", e.Reason)
	}
	return fmt.Sprintf("clone: stale baseline at slot %q: %s", e.Slot, e.Reason)
}

// Unwrap lets errors.Is(err, ErrStaleBaseline) match.
func (e *StaleBaselineError) Unwrap() error { return ErrStaleBaseline }

// ErrCandidateChanged is the sentinel a caller should compare against (via
// errors.Is) when the dual-digest recheck (obligation 3) finds that Digest
// recomputed over the plan's own private CANDIDATE copy no longer matches
// the digest stored in the plan at PrepareSend time. Like ErrStaleBaseline's
// digest half, this is a self-consistency assertion that should be
// unreachable outside a bug: the plan's candidate is never mutated after
// PrepareSend. The error actually returned is a *CandidateChangedError.
var ErrCandidateChanged = fmt.Errorf("clone: candidate no longer matches the plan it was captured into")

// CandidateChangedError reports the digest mismatch ErrCandidateChanged
// names.
type CandidateChangedError struct {
	// Reason is a human-readable explanation.
	Reason string
}

// Error implements the error interface.
func (e *CandidateChangedError) Error() string {
	return fmt.Sprintf("clone: candidate changed: %s", e.Reason)
}

// Unwrap lets errors.Is(err, ErrCandidateChanged) match.
func (e *CandidateChangedError) Unwrap() error { return ErrCandidateChanged }

// ErrSessionChanged is the sentinel a caller should compare against (via
// errors.Is) when Execute's session identity binding check (obligation 4)
// finds that the plan was not prepared against the session this Service is
// currently bound to — either the probed Identity differs, or the Service's
// own generation counter (assigned once, at NewService time) differs. The
// generation half exists precisely because content/identity alone cannot
// detect a reconnect to a DIFFERENT physical radio that happens to answer
// with the same CAT ID/port/USB serial as before: a plan is only ever valid
// for the exact Service (and therefore session) instance that built it.
// The error actually returned is a *SessionChangedError.
var ErrSessionChanged = fmt.Errorf("clone: session has changed since this plan was prepared")

// SessionChangedError reports the mismatch ErrSessionChanged names.
type SessionChangedError struct {
	// Reason is a human-readable explanation.
	Reason string
}

// Error implements the error interface.
func (e *SessionChangedError) Error() string {
	return fmt.Sprintf("clone: session changed: %s", e.Reason)
}

// Unwrap lets errors.Is(err, ErrSessionChanged) match.
func (e *SessionChangedError) Unwrap() error { return ErrSessionChanged }

// ErrConfirmationMismatch is the sentinel a caller should compare against
// (via errors.Is) when Execute's confirmation binding check (obligation 5)
// finds that the confirmedDigest argument does not equal the plan's own
// ConfirmationDigest() — the caller is confirming a diff the user was
// never shown (or a stale one), OR (Fix 1, adjudicated HIGH) is confirming
// a DIFFERENT plan that happens to share this plan's BaselineDigest (two
// plans PrepareSend built from the same baseline read but different
// candidates always share BaselineDigest — see ConfirmationDigest's doc
// comment for why the check is no longer against BaselineDigest alone).
// The error actually returned is a *ConfirmationMismatchError.
var ErrConfirmationMismatch = fmt.Errorf("clone: confirmed digest does not match the plan the user reviewed")

// ConfirmationMismatchError reports the mismatch ErrConfirmationMismatch
// names.
type ConfirmationMismatchError struct {
	// Want is the plan's own ConfirmationDigest().
	Want string
	// Got is the confirmedDigest the caller actually supplied.
	Got string
}

// Error implements the error interface.
func (e *ConfirmationMismatchError) Error() string {
	return fmt.Sprintf("clone: confirmation digest %q does not match the plan's baseline digest %q — the caller must confirm exactly what the user was shown", e.Got, e.Want)
}

// Unwrap lets errors.Is(err, ErrConfirmationMismatch) match.
func (e *ConfirmationMismatchError) Unwrap() error { return ErrConfirmationMismatch }

// ErrValidationFailed is the sentinel a caller should compare against (via
// errors.Is) when PrepareSend's codeplug.Validate call (or Execute's
// re-validation at the execute boundary, obligation 3) finds at least one
// SeverityError Issue. The error actually returned is a
// *ValidationFailedError carrying every Issue found (errors AND warnings),
// so a caller can display the full picture, not just the blocking ones.
var ErrValidationFailed = fmt.Errorf("clone: candidate codeplug failed validation")

// ValidationFailedError carries the Issues ErrValidationFailed names.
type ValidationFailedError struct {
	// Issues is every Issue codeplug.Validate found, errors and warnings
	// alike.
	Issues []codeplug.Issue
}

// Error implements the error interface: it summarises the error-severity
// issues (warnings are for display, not for this summary — see
// codeplug.HasErrors).
func (e *ValidationFailedError) Error() string {
	var msgs []string
	for _, i := range e.Issues {
		if i.Severity == codeplug.SeverityError {
			msgs = append(msgs, i.Msg)
		}
	}
	return fmt.Sprintf("clone: candidate codeplug failed validation: %s", strings.Join(msgs, "; "))
}

// Unwrap lets errors.Is(err, ErrValidationFailed) match.
func (e *ValidationFailedError) Unwrap() error { return ErrValidationFailed }

// ErrFirmwareUnconfirmed is the sentinel a caller should compare against
// (via errors.Is) when Execute's first-write interactive gate (obligation
// 10) finds that this Service has never had a firmware version confirmed
// for its session, and ExecuteOptions.FirmwareConfirmed is empty. This
// layer enforces presence only — obtaining the confirmation from a human
// reading it off the radio's front panel is the CLI/GUI's job.
var ErrFirmwareUnconfirmed = fmt.Errorf("clone: firmware version not confirmed for this session's first write")

// ErrVerifyMismatch is the sentinel a caller should compare against (via
// errors.Is) when a per-channel write's read-back verify (obligation 7)
// disagrees with what was written, on at least one WRITABLE field
// (CTCSSTone/ScanSkip are deliberately excluded — they read back Unknown by
// construction, see driver.Session.ReadChannel). The error actually
// returned is a *VerifyMismatchError, and it is always wrapped inside an
// *AbortedError (see ErrAborted): a verify mismatch always stops the whole
// Execute run.
var ErrVerifyMismatch = fmt.Errorf("clone: write verify read-back did not match what was written")

// VerifyMismatchError reports which slot and fields ErrVerifyMismatch
// names. Fields is empty when the mismatch was "read back as an empty slot
// after a populated write" rather than a specific field disagreeing.
type VerifyMismatchError struct {
	// Slot is the canonical wire-form slot that failed verification.
	Slot string
	// Fields lists the spec.Fields whose read-back value disagreed with
	// what was written. Empty when the read-back came back as an empty
	// (erased) slot instead of the populated channel just written.
	Fields []spec.Field
}

// Error implements the error interface.
func (e *VerifyMismatchError) Error() string {
	if len(e.Fields) == 0 {
		return fmt.Sprintf("clone: slot %q: read-back shows an empty slot after a populated write", e.Slot)
	}
	names := make([]string, len(e.Fields))
	for i, f := range e.Fields {
		names[i] = string(f)
	}
	return fmt.Sprintf("clone: slot %q: read-back disagrees on %s", e.Slot, strings.Join(names, ", "))
}

// Unwrap lets errors.Is(err, ErrVerifyMismatch) match.
func (e *VerifyMismatchError) Unwrap() error { return ErrVerifyMismatch }

// ErrJournalFailed is the sentinel a caller should compare against (via
// errors.Is) when a durable journal append itself fails (obligation 8's
// ratified fail-safe policy — see doc.go's "Journal durability policy"
// section, recorded verbatim there). Which of the two outcomes below
// happens depends on WHEN the failure occurs relative to the wire:
//
//   - the write_attempt line (recorded immediately before Execute calls
//     WriteChannel for a slot) fails to persist: Execute refuses before
//     ever calling WriteChannel — nothing has touched the radio for that
//     slot, so the refusal is free;
//   - a write_result or verify_result line (recorded AFTER WriteChannel,
//     or after the paired verify ReadChannel, has already run) fails to
//     persist: the wire action already happened and cannot be un-done, so
//     Execute counts it and aborts all FURTHER writes via the same
//     abort machinery a write rejection or verify mismatch uses.
//
// PrepareSend's own "prepare" journal line is likewise fail-safe: a
// failure there means PrepareSend refuses to hand back a plan at all.
//
// The error actually returned is always a *JournalFailedError wrapping
// the underlying append/fsync error; inside Execute (never PrepareSend)
// it is additionally always wrapped inside an *AbortedError (see
// ErrAborted), the same as any other abort cause.
var ErrJournalFailed = fmt.Errorf("clone: journal append failed")

// JournalFailedError reports which journal event failed to persist, at
// which slot (when applicable), and why.
type JournalFailedError struct {
	// Event is the journal event name that failed to append, e.g.
	// "write_attempt", "write_result", "verify_result", or "prepare".
	Event string
	// Slot is the slot being processed when the failure happened, or ""
	// for PrepareSend's "prepare" line (which is not slot-scoped).
	Slot string
	// Cause is the underlying error Journal.Append returned.
	Cause error
}

// Error implements the error interface.
func (e *JournalFailedError) Error() string {
	if e.Slot == "" {
		return fmt.Sprintf("clone: journal append %q failed: %v", e.Event, e.Cause)
	}
	return fmt.Sprintf("clone: journal append %q failed at slot %q: %v", e.Event, e.Slot, e.Cause)
}

// Unwrap lets errors.Is(err, ErrJournalFailed) match, AND lets
// errors.Is/errors.As reach Cause itself — mirrors AbortedError.Unwrap's
// multi-error form, for the same reason.
func (e *JournalFailedError) Unwrap() []error {
	if e.Cause == nil {
		return []error{ErrJournalFailed}
	}
	return []error{ErrJournalFailed, e.Cause}
}

// ErrBusy is the sentinel a caller should compare against (via errors.Is)
// when ReadAll, PrepareSend, or Execute is called while this Service is
// already running ANOTHER of those three operations (M3 Codex-review fix
// wave, Fix 2, adjudicated MEDIUM). A Service enforces logical-operation
// serialisation with a try-lock (Service.acquireOp): a concurrent caller
// is refused outright, immediately, rather than being queued behind the
// in-progress operation or — worse — allowed to interleave its own wire
// traffic with it. The error actually returned is a *BusyError.
var ErrBusy = fmt.Errorf("clone: this Service is already running another operation")

// BusyError reports which operation was already in progress when a
// concurrent caller was refused.
type BusyError struct {
	// InProgress names the operation already running: "ReadAll",
	// "PrepareSend", or "Execute".
	InProgress string
}

// Error implements the error interface.
func (e *BusyError) Error() string {
	return fmt.Sprintf("clone: busy: %s is already in progress on this Service", e.InProgress)
}

// Unwrap lets errors.Is(err, ErrBusy) match.
func (e *BusyError) Unwrap() error { return ErrBusy }

// ErrSettingsUnsupported is the sentinel a caller should compare against
// (via errors.Is) when ReadSettings finds that this Service's session does
// not implement the optional driver.SettingsReader capability — the exact
// same optional-interface reasoning as MemorySelector (see
// core/clone/memory_selector.go's doc comment): not every driver's radio
// has a settings surface this project has characterised (or one at all),
// so ReadSettings discovers it with a plain type assertion rather than
// requiring every driver.Session implementation to satisfy it. The error
// actually returned is a *SettingsUnsupportedError naming the model.
var ErrSettingsUnsupported = errors.New("clone: this session's driver does not expose settings")

// SettingsUnsupportedError reports which model's session
// ErrSettingsUnsupported names.
type SettingsUnsupportedError struct {
	// Model is this session's Capabilities().Model.
	Model string
}

// Error implements the error interface.
func (e *SettingsUnsupportedError) Error() string {
	return fmt.Sprintf("clone: ReadSettings: %q's driver does not expose a settings surface", e.Model)
}

// Unwrap lets errors.Is(err, ErrSettingsUnsupported) match.
func (e *SettingsUnsupportedError) Unwrap() error { return ErrSettingsUnsupported }

// ErrAborted is the sentinel a caller should compare against (via
// errors.Is) when Execute stopped before completing every delta write: a
// verify mismatch, a write rejection, a transport timeout, or a context
// cancellation observed between channel operations. The error actually
// returned is always an *AbortedError, alongside a non-nil *Report the
// caller should still inspect for exactly how far execution got — Execute
// deliberately returns BOTH a report and an error in this one case, because
// the partial progress has forensic value the error string alone cannot
// carry.
var ErrAborted = fmt.Errorf("clone: execute aborted before completing every write")

// AbortedError reports why Execute aborted and, when known, which slot was
// being processed at the time.
type AbortedError struct {
	// Slot is the slot being processed when the abort happened, or "" when
	// the abort happened between slots (e.g. a cancelled context observed
	// before the next slot's write began).
	Slot string
	// Reason is a human-readable explanation.
	Reason string
	// Cause is the underlying error that triggered the abort, when there is
	// one distinguishable from Reason alone (e.g. a *VerifyMismatchError, a
	// write rejection, or context.Canceled/context.DeadlineExceeded). May
	// be nil.
	Cause error
}

// Error implements the error interface.
func (e *AbortedError) Error() string {
	if e.Slot == "" {
		return fmt.Sprintf("clone: aborted: %s", e.Reason)
	}
	return fmt.Sprintf("clone: aborted at slot %q: %s", e.Slot, e.Reason)
}

// Unwrap lets errors.Is(err, ErrAborted) match (always), AND lets
// errors.Is/errors.As reach Cause itself (e.g. errors.Is(err,
// ErrVerifyMismatch), or errors.Is(err, context.Canceled)) — both are part
// of "what this error means", so both are exposed via the multi-error
// Unwrap form.
func (e *AbortedError) Unwrap() []error {
	if e.Cause == nil {
		return []error{ErrAborted}
	}
	return []error{ErrAborted, e.Cause}
}
