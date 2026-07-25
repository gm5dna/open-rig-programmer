// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/clone"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/internal/radiotext"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// errTrackingWriter wraps w, forwarding every Write call unchanged but
// recording the FIRST error any of them returns, and short-circuiting
// every write after that to a no-op returning the same cached error (so
// a broken pipe never spews repeated failing syscalls). This lets a
// rendering function built from many individual fmt.Fprint*/fmt.Fprintf
// calls — each, by Go's usual io.Writer idiom, ignoring its OWN returned
// error inline — be checked ONCE at the end via its err field. That is
// what Fix 1 (adjudicated HIGH, Codex M4 #1) needs: a write-flow
// renderer or prompt that could not actually reach its writer must gate
// Execute, not silently let a broken stdout/stderr fall through to it.
type errTrackingWriter struct {
	w   io.Writer
	err error
}

func (e *errTrackingWriter) Write(p []byte) (int, error) {
	if e.err != nil {
		return 0, e.err
	}
	n, err := e.w.Write(p)
	if err != nil {
		e.err = err
	}
	return n, err
}

// countSendable returns how many of diff's entries are actually
// send-worthy: Added or Modified, and NOT Blocked (task-14 brief §1 step
// 5). This is the exact quantity step 5 checks against zero ("nothing to
// send") and the exact N step 6's confirmation prompt names.
//
// countSendable == 0 is ambiguous by itself between two very different
// situations (task-25 brief, the reported "i don't seem to be able to
// send deletes to the radio" defect): the working copy genuinely matches
// the radio (diff.Blocked == 0 too), or every pending change IS blocked
// — most often a channel delete, which becomes a DiffErased entry Diff
// always marks Blocked (no CAT erase exists — see hasBlockedErase's doc
// comment). runWrite below distinguishes the two rather than reporting
// "Nothing to send." for both.
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

// hasBlockedErase reports whether entries contains at least one Blocked
// DiffErased entry — writeNothingSendableReport's gate for appending the
// model's radiotext.Text.EraseProcedure: a field-gate or Inert block
// (e.g. "bank ... is read-only", "clarifier changes are ignored") has no
// front-panel workaround to offer, but a blocked erase always does.
func hasBlockedErase(entries []codeplug.DiffEntry) bool {
	for _, e := range entries {
		if e.Blocked && e.Kind == codeplug.DiffErased {
			return true
		}
	}
	return false
}

// writeNothingSendableReport renders the "blocked-only" nothing-to-send
// case (task-25 brief, adjudicated remedy for the reported "i don't seem
// to be able to send deletes to the radio" defect): countSendable == 0
// NOT because the working copy matches the radio, but because every
// pending change is Blocked. The plan's full diff (including this same
// per-entry BlockReason) was already rendered above by writePlanSummary
// — this repeats just the blocked slots, reasons, and (for an erase) the
// front-panel procedure, right next to the final verdict, so the caller
// never has to scroll back up to see WHY nothing was sendable. Unlike
// the plain "Nothing to send." case, this NEVER claims the working copy
// matches the radio — it does not; the pending edits are simply
// unsendable, not honoured.
//
// The front-panel erase procedure is model's own radiotext.Text.
// EraseProcedure (task 40: was a package-level FT-710-only const before
// this task; internal/radiotext.For's FT-710 entry is byte-identical to
// the retired const's text). Silently omitted — never a fabricated
// generic sentence — for a model radiotext has no entry for.
func writeNothingSendableReport(w io.Writer, model string, diff codeplug.DiffResult) {
	fmt.Fprintln(w, "\nNothing was sent: every pending change is blocked.")
	fmt.Fprintln(w, "The working copy still differs from the radio — your edits are still in FILE, but none of them can be written to this radio:")
	for _, e := range diff.Entries {
		if !e.Blocked {
			continue
		}
		fmt.Fprintf(w, "  %s (%s): %s\n", codeplug.DisplaySlot(e.Slot), e.Kind, e.BlockReason)
	}
	if hasBlockedErase(diff.Entries) {
		if text, ok := radiotext.For(model); ok {
			fmt.Fprintln(w)
			fmt.Fprintln(w, text.EraseProcedure)
		}
	}
}

// writePlanSummary renders a PrepareSend plan's review material to w
// (task-14 brief §1 step 4): the diff — reusing diff.go's writeDiffReport
// verbatim, never duplicating its rendering logic — followed by the
// snapshot path and the truncated baseline digest, explicitly labelled as
// the plan's baseline (as opposed to any later, re-read digest). Takes
// the diff/paths as plain values rather than a *clone.SendPlan itself:
// SendPlan is deliberately opaque outside core/clone (accessors only), so
// this keeps the renderer unit-testable with synthetic data, the same
// principle diff.go's writeDiffReport already follows for codeplug.DiffResult.
//
// Returns the first write error encountered, if any (Fix 1, adjudicated
// HIGH, Codex M4 #1): this is pre-Execute rendering, so a caller that
// cannot actually deliver the plan to the user must abort rather than
// let Execute run against an unseen plan.
func writePlanSummary(w io.Writer, diff codeplug.DiffResult, snapshotPath, baselineDigest string) error {
	tw := &errTrackingWriter{w: w}
	_ = writeDiffReport(tw, diff) // tw itself already tracks this; see its doc comment.
	fmt.Fprintln(tw)
	fmt.Fprintf(tw, "Snapshot: %s\n", snapshotPath)
	fmt.Fprintf(tw, "Baseline digest: %s (truncated, plan's baseline)\n", truncateDigest(baselineDigest))
	return tw.err
}

// writeExecuteSummary renders a successful Execute's summary to w
// (task-14 brief §1 step 8): Written, Verified, SkippedBlocked, Unchanged,
// journal path, snapshot path. snapshotPath comes from the plan
// (Report itself does not carry it — only the journal path does).
func writeExecuteSummary(w io.Writer, report *clone.Report, snapshotPath string) {
	fmt.Fprintf(w, "Written:        %d\n", report.Written)
	fmt.Fprintf(w, "Verified:       %d\n", report.Verified)
	fmt.Fprintf(w, "SkippedBlocked: %d\n", report.SkippedBlocked)
	fmt.Fprintf(w, "Unchanged:      %d\n", report.Unchanged)
	fmt.Fprintf(w, "Journal:        %s\n", report.JournalPath)
	fmt.Fprintf(w, "Snapshot:       %s\n", snapshotPath)
}

// writeSlotTable renders report's per-slot results (task-14 brief §1 step
// 8's abort rendering: "the per-slot results table — action + verify
// status + detail per slot"). Action/VerifyOK/Detail are documented,
// stable string contracts on clone.SlotResult/clone.Report (see
// core/clone/execute.go's doc comments) even though the underlying
// constants are unexported there.
func writeSlotTable(w io.Writer, slots []clone.SlotResult) {
	fmt.Fprintln(w, "Slot results:")
	for _, s := range slots {
		verify := "-"
		if s.Action == "write" {
			if s.VerifyOK {
				verify = "verified"
			} else {
				verify = "FAILED"
			}
		}
		line := fmt.Sprintf("  %s: %s [%s]", codeplug.DisplaySlot(s.Slot), s.Action, verify)
		if s.Detail != "" {
			line += " — " + s.Detail
		}
		fmt.Fprintln(w, line)
	}
}

// writeRecoveryGuidance returns the abort recovery message (task-14 brief
// §1 step 8's plan honesty rule): the saved file is always called a
// SNAPSHOT, never a "backup" — restore limits via CAT are unproven until
// hardware trials, and "backup" implies a completeness guarantee this
// project cannot yet make.
func writeRecoveryGuidance(snapshotPath string) string {
	return fmt.Sprintf(
		"Recovery: the radio has been partially written and is left in that partial state.\n"+
			"The snapshot at %s records what this radio held when the plan was\n"+
			"prepared — re-running \"rigprog write\" with that snapshot file as FILE can\n"+
			"re-send any still-writable slots. Erased and other hidden fields cannot be\n"+
			"restored via CAT (unverified until hardware trials).",
		snapshotPath,
	)
}

// writeAbortReport renders a *clone.AbortedError's full picture to w
// (task-14 brief §1 step 8): the slot and reason, the per-slot results
// table, the journal path, the snapshot path, and recovery guidance. When
// the abort's Cause is (or wraps) a context cancellation, adds step 9's
// required note: the in-flight write+verify pair completed before the
// cancellation was honoured, so the journal is the record of exactly what
// happened.
func writeAbortReport(w io.Writer, abortErr *clone.AbortedError, report *clone.Report, snapshotPath string) {
	slot := "-"
	if abortErr.Slot != "" {
		slot = codeplug.DisplaySlot(abortErr.Slot)
	}
	fmt.Fprintf(w, "rigprog write: ABORTED at slot %s: %s\n", slot, abortErr.Reason)
	if isCancelled(abortErr) {
		fmt.Fprintln(w, "  the in-flight write+verify pair completed before the cancellation was honoured; see the journal for exactly what was written.")
	}
	if report != nil {
		writeSlotTable(w, report.Slots)
		fmt.Fprintf(w, "Journal:  %s\n", report.JournalPath)
	}
	fmt.Fprintf(w, "Snapshot: %s\n", snapshotPath)
	fmt.Fprintln(w, writeRecoveryGuidance(snapshotPath))
}

// refusalMessage maps a pre-write refusal error (task-14 brief §1 step
// 8's ErrStaleBaseline/ErrSessionChanged/ErrCandidateChanged/
// ErrConfirmationMismatch, plus — Fix 2, adjudicated MEDIUM, Codex M4
// #2 — the defensive ErrFirmwareUnconfirmed case) to a plain-language
// line. The stale-baseline wording matches the brief's own example
// verbatim. Falls back to err.Error() for anything else (defensive only
// — handleExecuteOutcome only calls this once every other error class
// has already been ruled out, so in practice this always sees one of
// the five).
func refusalMessage(err error) string {
	switch {
	case errors.Is(err, clone.ErrStaleBaseline):
		return "refused: the radio's contents changed after the plan was prepared — re-run write"
	case errors.Is(err, clone.ErrSessionChanged):
		return "refused: the radio session changed since the plan was prepared (a reconnect?) — re-run write"
	case errors.Is(err, clone.ErrCandidateChanged):
		return "refused: an internal consistency check failed (candidate changed) — re-run write"
	case errors.Is(err, clone.ErrConfirmationMismatch):
		return "refused: the confirmation did not match the plan that was reviewed — re-run write"
	case errors.Is(err, clone.ErrFirmwareUnconfirmed):
		return "refused: firmware version not confirmed for this session's first write"
	default:
		return err.Error()
	}
}

// isExecuteRefusalSentinel reports whether err is one of Execute's
// documented pre-write refusal sentinels (Fix 2, adjudicated MEDIUM,
// Codex M4 #2): the dual-digest/session/confirmation checks (obligations
// 3-5), plus ErrFirmwareUnconfirmed for the defensive case where it
// reaches handleExecuteOutcome at all — the ordinary path handles it
// earlier, in executeAndReport, via the firmware prompt/retry.
func isExecuteRefusalSentinel(err error) bool {
	return errors.Is(err, clone.ErrStaleBaseline) ||
		errors.Is(err, clone.ErrSessionChanged) ||
		errors.Is(err, clone.ErrCandidateChanged) ||
		errors.Is(err, clone.ErrConfirmationMismatch) ||
		errors.Is(err, clone.ErrFirmwareUnconfirmed)
}

// writeValidationIssues renders every Issue PrepareSend's
// *clone.ValidationFailedError carried (task-14 brief §1 step 3: "print
// every Issue — slot, field, severity, message"), in that field order,
// in codeplug.Validate's own returned order. Deliberately distinct from
// import.go's writeIssues: that one's "offline validation notes —
// authoritative validation happens at write time" framing is specific to
// import's advisory, pre-radio check — here, THIS is write time, so the
// framing must not say otherwise.
func writeValidationIssues(w io.Writer, issues []codeplug.Issue) {
	fmt.Fprintln(w, "rigprog write: candidate codeplug failed validation:")
	for _, is := range issues {
		slot := "-"
		if is.Slot != "" {
			slot = codeplug.DisplaySlot(is.Slot)
		}
		field := string(is.Field)
		if field == "" {
			field = "-"
		}
		fmt.Fprintf(w, "  slot %s, field %s, %s: %s\n", slot, field, is.Severity, is.Msg)
	}
}

// isStdinTTY reports whether r is connected to an interactive terminal:
// the established, dependency-free stdlib idiom (an *os.File whose mode
// has os.ModeCharDevice set). Both run()'s in-process tests
// (bytes.Reader/strings.Reader) and black-box piped stdin correctly read
// as non-TTY here (Fix 4, adjudicated MEDIUM, Codex M4 #4, additionally
// documents /dev/null's own quirk: it IS itself a character device, so
// THIS function alone misclassifies it as interactive too — the prompts
// built on isStdinTTY correct for that at the EOF layer instead, see
// resolveConfirmation). Since no test in this package can make THIS
// function itself observe isTTY==true without a real terminal, the
// interactive-decline path (an isTTY==true caller who types "no") is
// covered exclusively at the function level, by injecting isTTY
// directly — see TestResolveConfirmation's "tty, declined, ..." cases
// (write_test.go). No black-box test proves that wiring end-to-end
// (Fix 7, adjudicated LOW, Codex M4 #7): black-box stdin is always
// either a pipe or /dev/null, never a real terminal, so every black-box
// "write" invocation without --yes takes the non-interactive branch,
// never the interactive prompt at all.
func isStdinTTY(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// readLine reads one line from in and returns it with any trailing \r\n
// or \n stripped, plus whether the read hit EOF before a newline was
// found — bufio.Reader.ReadString's own io.EOF, whichever of "no bytes
// at all" or "some bytes but no trailing newline" produced it (line
// still holds whatever WAS read either way, matching ReadString's own
// contract). Callers share a single *bufio.Reader across every prompt in
// one run — see runWrite — so an over-read by one prompt's Read call
// never strands a later prompt's answer in a discarded buffer.
//
// eof is what Fix 4 (adjudicated MEDIUM, Codex M4 #4) uses:
// isStdinTTY's os.ModeCharDevice test misclassifies /dev/null (itself a
// character device on Unix) as an interactive terminal, so `</dev/null`
// reaches a prompt built on this function. An EOF here means there was
// no answer to read at all, regardless of what isTTY claimed — see
// resolveConfirmation/promptFirmwareVersion for how that gets
// reclassified as non-interactive rather than a typed decline.
func readLine(in *bufio.Reader) (line string, eof bool) {
	s, err := in.ReadString('\n')
	return strings.TrimRight(s, "\r\n"), errors.Is(err, io.EOF)
}

// nonInteractiveConfirmationGuidance is resolveConfirmation's message
// for every case where this run cannot be confirmed interactively —
// isTTY said so outright, or (Fix 4, adjudicated MEDIUM, Codex M4 #4) an
// EOF read revealed isTTY was wrong. Printing the identical line either
// way is deliberate: a caller reading stderr should not be able to tell
// the two cases apart, since both mean exactly the same thing — pass
// --yes.
const nonInteractiveConfirmationGuidance = "rigprog write: refusing to prompt on non-interactive stdin; pass --yes to send without a confirmation prompt"

// resolveConfirmation implements task-14 brief §1 step 6's confirmation
// gate: --yes proceeds outright; a TTY gets the exact prompt the brief
// specifies and must answer exactly "yes" or the send is cancelled;
// anything else (no --yes, not a TTY, or — Fix 4 — an EOF read on an
// allegedly-interactive stdin) is refused as a usage error telling the
// caller to pass --yes. isTTY is an injected flag (see isStdinTTY)
// precisely so this function's own tests can drive the interactive
// prompt/decline path without a real terminal — the layer this task's
// report names as covering that path.
//
// A write error while rendering EITHER the non-TTY guidance or the
// interactive prompt itself (Fix 1, adjudicated HIGH, Codex M4 #1) is
// reported as exitError (1), not exitUsage/exitRefused: the caller never
// actually saw what it was being asked to confirm, so proceeding — or
// even claiming a definite refusal reason — would be dishonest.
func resolveConfirmation(in *bufio.Reader, stdout, stderr io.Writer, isTTY, yesFlag bool, n int) (proceed bool, code int) {
	if yesFlag {
		return true, exitSuccess
	}
	tw := &errTrackingWriter{w: stderr}
	if !isTTY {
		fmt.Fprintln(tw, nonInteractiveConfirmationGuidance)
		if tw.err != nil {
			return false, exitError
		}
		return false, exitUsage
	}
	fmt.Fprintf(tw, "Send %d change(s) to the radio? Type \"yes\": ", n)
	if tw.err != nil {
		return false, exitError
	}
	answer, eof := readLine(in)
	if eof {
		// Fix 4 (adjudicated MEDIUM, Codex M4 #4): isStdinTTY said this
		// looked interactive, but the read just hit EOF with no answer at
		// all — /dev/null's classic quirk. Reclassify as non-interactive
		// (exitUsage, the SAME guidance the !isTTY branch above prints)
		// rather than treating a silent EOF as a typed decline.
		fmt.Fprintln(tw, nonInteractiveConfirmationGuidance)
		if tw.err != nil {
			return false, exitError
		}
		return false, exitUsage
	}
	if answer != "yes" {
		fmt.Fprintln(tw, "cancelled, nothing sent")
		return false, exitRefused
	}
	return true, exitSuccess
}

// writeFirmwareNonInteractiveGuidance is promptFirmwareVersion's message
// for every case where this run cannot obtain a firmware confirmation
// interactively — isTTY said so outright, or (Fix 4, adjudicated MEDIUM,
// Codex M4 #4) an EOF read revealed isTTY was wrong. Printing the
// identical text either way is deliberate, mirroring
// nonInteractiveConfirmationGuidance's own reasoning above.
func writeFirmwareNonInteractiveGuidance(w io.Writer) {
	fmt.Fprintln(w, "rigprog write: refused: firmware version not confirmed for this session's first write")
	fmt.Fprintln(w, "  memory CAT requires firmware >= V01-10; there is no CAT query for it —")
	fmt.Fprintln(w, "  check the radio's front panel (or SD-card version screen), then re-run with --firmware VER")
}

// promptFirmwareVersion implements task-14 brief §1 step 7's first-write
// firmware gate follow-up: on a TTY, explains why a firmware version is
// needed (memory CAT requires firmware >= V01-10; there is no CAT query
// for it; it is shown on the radio's front panel or SD-card version
// screen) and reads a non-empty version string; off a TTY, refuses and
// tells the caller to pass --firmware. isTTY is injected for the same
// testability reason as resolveConfirmation's.
//
// The third return, err, is non-nil only when writing the guidance/
// prompt text itself failed (Fix 1, adjudicated HIGH, Codex M4 #1) — as
// opposed to ok==false, which means the prompt was rendered fine but
// refused/declined/empty/EOF. executeAndReport treats a non-nil err as
// an abort distinct from an ordinary refusal: nothing further should be
// attempted, since the caller never actually saw the prompt.
//
// An EOF read on an allegedly-interactive stdin (Fix 4, adjudicated
// MEDIUM, Codex M4 #4 — the same /dev/null quirk resolveConfirmation's
// doc comment explains) is reclassified as non-interactive: ok==false
// with writeFirmwareNonInteractiveGuidance's text, NOT the unrelated
// "no firmware version entered" wording an empty typed LINE gets. Either
// way ok==false maps to exitRefused (4) in executeAndReport — unlike the
// confirmation prompt's EOF case, the firmware prompt keeps its
// documented non-interactive exit code.
func promptFirmwareVersion(in *bufio.Reader, isTTY bool, stdout, stderr io.Writer) (version string, ok bool, err error) {
	tw := &errTrackingWriter{w: stderr}
	if !isTTY {
		writeFirmwareNonInteractiveGuidance(tw)
		return "", false, tw.err
	}
	fmt.Fprintln(tw, "Memory CAT requires firmware >= V01-10. There is no CAT query for the firmware version —")
	fmt.Fprintln(tw, "check the radio's front panel (or SD-card version screen).")
	fmt.Fprint(tw, "Firmware version: ")
	if tw.err != nil {
		return "", false, tw.err
	}
	line, eof := readLine(in)
	if eof {
		writeFirmwareNonInteractiveGuidance(tw)
		return "", false, tw.err
	}
	v := strings.TrimSpace(line)
	if v == "" {
		fmt.Fprintln(tw, "rigprog write: refused: no firmware version entered")
		return "", false, nil
	}
	return v, true, nil
}

// handleExecuteOutcome implements task-14 brief §1 step 8's outcome
// handling, EXPLICITLY classified by error type (Fix 2, adjudicated
// MEDIUM, Codex M4 #2 — replacing the earlier "everything non-Aborted is
// exit 4" blanket mapping, which was wrong for at least three of
// Execute's actual error classes):
//
//   - err == nil: success, the summary on stdout, exitSuccess.
//   - *clone.AbortedError (the radio HAS been touched — report is used,
//     never discarded): the full abort picture on stderr, exitAborted.
//   - *clone.ValidationFailedError (Execute's own re-validation at the
//     execute boundary, obligation 3, can produce this exactly like
//     PrepareSend's): every Issue on stderr, exitBlocked — matching
//     reportPrepareSendFailure's own PrepareSend-time mapping.
//   - a cancelled ctx (isCancelled — Execute wraps context.Canceled/
//     DeadlineExceeded in a plain error, not an AbortedError, when the
//     cancellation is observed BEFORE any write starts): the same
//     "cancelled" wording every other radio-touching subcommand uses,
//     exitError.
//   - *clone.BusyError (a concurrent ReadAll/PrepareSend/Execute call
//     refused this Service's try-lock): an operational failure, not a
//     content refusal, exitError.
//   - one of the documented pre-write refusal sentinels
//     (isExecuteRefusalSentinel): a plain-language line via
//     refusalMessage, exitRefused — the ONLY class this function still
//     maps to exit 4.
//   - anything else (an unexpected/unrecognised error): a generic
//     message, exitError — NOT exitRefused, since this project's exit 4
//     is reserved for the documented refusal sentinels alone.
//
// snapshotPath comes from the plan, not the (possibly nil, on a
// pre-write refusal) report.
func handleExecuteOutcome(stdout, stderr io.Writer, report *clone.Report, err error, snapshotPath string) int {
	if err == nil {
		writeExecuteSummary(stdout, report, snapshotPath)
		return exitSuccess
	}

	var aborted *clone.AbortedError
	if errors.As(err, &aborted) {
		writeAbortReport(stderr, aborted, report, snapshotPath)
		return exitAborted
	}

	var vfe *clone.ValidationFailedError
	if errors.As(err, &vfe) {
		writeValidationIssues(stderr, vfe.Issues)
		return exitBlocked
	}

	if isCancelled(err) {
		fmt.Fprintln(stderr, "rigprog write: cancelled")
		return exitError
	}

	var busy *clone.BusyError
	if errors.As(err, &busy) {
		fmt.Fprintf(stderr, "rigprog write: %v\n", err)
		return exitError
	}

	if isExecuteRefusalSentinel(err) {
		fmt.Fprintf(stderr, "rigprog write: %s\n", refusalMessage(err))
		return exitRefused
	}

	fmt.Fprintf(stderr, "rigprog write: %v\n", err)
	return exitError
}

// reportPrepareSendFailure implements task-14 brief §1 step 3's
// PrepareSend error handling: a cancelled ctx prints "cancelled"
// (matching every other radio-touching subcommand); a
// *clone.ValidationFailedError prints every Issue and returns exitBlocked
// (3); anything else (journal/snapshot failures, ErrBusy, a plain
// transport error) is exitError (1), explicitly noting nothing was sent.
func reportPrepareSendFailure(stderr io.Writer, err error) int {
	if isCancelled(err) {
		fmt.Fprintln(stderr, "rigprog write: cancelled")
		return exitError
	}
	var vfe *clone.ValidationFailedError
	if errors.As(err, &vfe) {
		writeValidationIssues(stderr, vfe.Issues)
		return exitBlocked
	}
	fmt.Fprintf(stderr, "rigprog write: preparing send: %v (nothing was sent)\n", err)
	return exitError
}

// executeAndReport implements task-14 brief §1 steps 7-8: calls Execute,
// and if it refuses specifically with ErrFirmwareUnconfirmed (the
// first-write interactive gate), obtains a firmware version via
// promptFirmwareVersion and re-Executes exactly once with it — "Execute
// is refusal-safe to retry here: the firmware refusal happens before any
// write reaches the radio" (task-14 brief §1 step 7). Either way, the
// final (report, err) pair is handed to handleExecuteOutcome.
//
// If promptFirmwareVersion itself could not render (Fix 1, adjudicated
// HIGH, Codex M4 #1), this aborts with exitError BEFORE the retry
// Execute call that would actually reach the radio — no write has
// happened yet at this point either way (the first Execute call refused
// with ErrFirmwareUnconfirmed before touching the radio at all).
func executeAndReport(ctx context.Context, svc *clone.Service, plan *clone.SendPlan, firmwareFlag string, isTTY bool, in *bufio.Reader, stdout, stderr io.Writer) int {
	digest := plan.ConfirmationDigest()
	report, err := svc.Execute(ctx, plan, digest, clone.ExecuteOptions{FirmwareConfirmed: firmwareFlag})
	if errors.Is(err, clone.ErrFirmwareUnconfirmed) {
		version, ok, werr := promptFirmwareVersion(in, isTTY, stdout, stderr)
		if werr != nil {
			fmt.Fprintf(stderr, "rigprog write: rendering firmware prompt: %v (nothing was sent)\n", werr)
			return exitError
		}
		if !ok {
			return exitRefused
		}
		report, err = svc.Execute(ctx, plan, digest, clone.ExecuteOptions{FirmwareConfirmed: version})
	}
	return handleExecuteOutcome(stdout, stderr, report, err, plan.SnapshotPath())
}

// runWrite implements task-14 brief §1 steps 3-9 against an
// already-open session: build the Service (snapshot store + progress to
// stderr), PrepareSend, render the plan, short-circuit when nothing is
// sendable — exitSuccess for the genuine "matches the radio" case,
// exitBlocked when the short-circuit is reached only because every
// pending change is Blocked (task-25 brief; see writeNothingSendableReport) —
// gate on confirmation, Execute (with the firmware retry), and report the
// outcome. Split out from cmdWrite (which owns flag parsing
// and session construction, exactly like every other radio-touching
// subcommand in this package) so a test can drive this flow directly
// against a session it constructs itself — see write_inprocess_test.go
// for why: this is what lets fault/drift/round-trip in-process tests
// hold their own reference to the session (and, via it, the underlying
// fakeradio.Radio) without needing a new production-only test seam.
// Takes the driver.Session interface, not a concrete session type —
// nothing in this function needs driver-specific accessors
// (Region/Diagnostics), and the interface is what a test's own
// ft710.New(...).Open(...) call already returns, with no type assertion
// needed. model (task 40) is the session's own radio model, threaded
// through to writeNothingSendableReport's radiotext.For lookup — the
// caller (cmdWrite) already validated it against wiring.SupportedModels()
// before opening sess.
func runWrite(ctx context.Context, model string, sess driver.Session, snapshotDir string, file *codeplug.Codeplug, yesFlag bool, firmwareFlag string, isTTY bool, stdin io.Reader, stdout, stderr io.Writer) int {
	svc := clone.NewService(sess, clone.SnapshotStore{Dir: snapshotDir}, clone.WithProgress(progressPrinter(stderr)))

	plan, err := svc.PrepareSend(ctx, file)
	if err != nil {
		return reportPrepareSendFailure(stderr, err)
	}

	diff := plan.Diff()

	// Fix 1 (adjudicated HIGH, Codex M4 #1): a failure to actually render
	// the plan gates Execute outright — with --yes, resolveConfirmation
	// below never writes anything at all, so THIS is the one place that
	// bug (a broken stdout letting Execute run against a plan the caller
	// never saw) can be caught.
	if err := writePlanSummary(stdout, diff, plan.SnapshotPath(), plan.BaselineDigest()); err != nil {
		fmt.Fprintf(stderr, "rigprog write: rendering plan summary: %v (nothing was sent)\n", err)
		return exitError
	}

	sendable := countSendable(diff)
	if sendable == 0 {
		// task-25 brief (adjudicated remedy for the reported "i don't seem
		// to be able to send deletes to the radio" defect): countSendable
		// == 0 alone does NOT mean the working copy matches the radio — a
		// blocked-only plan (in practice, a channel delete: no CAT erase
		// exists) reaches here too, and must never be reported as if it
		// were the same "nothing pending at all" case. Only diff.Blocked
		// == 0 (genuinely nothing — every entry Unchanged) gets the plain
		// success message; a blocked-only plan is exitBlocked (3), naming
		// every blocked slot and reason again, plus the front-panel erase
		// procedure when relevant — a fixed, script-visible signal that
		// nothing was actually sent, not a false parity claim.
		if diff.Blocked == 0 {
			fmt.Fprintln(stdout, "\nNothing to send.")
			return exitSuccess
		}
		writeNothingSendableReport(stdout, model, diff)
		return exitBlocked
	}

	in := bufio.NewReader(stdin)
	proceed, code := resolveConfirmation(in, stdout, stderr, isTTY, yesFlag, sendable)
	if !proceed {
		if code == exitError {
			fmt.Fprintln(stderr, "rigprog write: nothing was sent")
		}
		return code
	}

	return executeAndReport(ctx, svc, plan, firmwareFlag, isTTY, in, stdout, stderr)
}

// cmdWrite implements "rigprog write" (task-14 brief §1): load FILE
// strictly, open a session, and hand off to runWrite for the actual
// PrepareSend/confirm/Execute flow. Mirrors cmdRead/cmdDiff's own
// flag-parsing and session-opening shape exactly (--port XOR --fake,
// resolveSnapshotDir + MkdirAll before opening a session so a cheap
// failure there never wastes a multi-second ReadAll).
func cmdWrite(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("write", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // this function owns all usage/error output.
	port := fs.String("port", "", "real serial port device path")
	fake := fs.Bool("fake", false, "use the in-process simulated radio")
	yes := fs.Bool("yes", false, "skip the interactive confirmation prompt (required for non-interactive runs)")
	firmware := fs.String("firmware", "", "confirmed radio firmware version (see the radio's front panel or SD-card version screen)")
	model := fs.String("model", wiring.DefaultModel, "radio model to target")
	snapshotDirFlag := fs.String("snapshot-dir", "", "snapshot/journal directory (default: <UserConfigDir>/rigprog/snapshots)")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printWriteUsage(stdout)
			return exitSuccess
		}
		fmt.Fprintf(stderr, "rigprog write: %v\n", err)
		printWriteUsage(stderr)
		return exitUsage
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "rigprog write: exactly one FILE argument is required")
		printWriteUsage(stderr)
		return exitUsage
	}
	filePath := fs.Arg(0)

	if !validateModel(stderr, "write", *model, printWriteUsage) {
		return exitUsage
	}

	havePort := *port != ""
	if havePort == *fake { // both true, or both false
		fmt.Fprintln(stderr, "rigprog write: exactly one of --port or --fake is required")
		printWriteUsage(stderr)
		return exitUsage
	}

	// Step 1: load FILE strictly (task-12-style typed-error reporting) —
	// before anything radio-touching, exactly like diff's candidate load.
	file, code := loadCodeplugStrict(stderr, "write", "", filePath)
	if file == nil {
		return code
	}

	snapshotDir, err := resolveSnapshotDir(*snapshotDirFlag)
	if err != nil {
		fmt.Fprintf(stderr, "rigprog write: %v\n", err)
		return exitError
	}
	if err := os.MkdirAll(snapshotDir, 0o700); err != nil {
		fmt.Fprintf(stderr, "rigprog write: creating snapshot directory %s: %v\n", snapshotDir, err)
		return exitError
	}

	// Step 2: open session; the Service (with the snapshot store and
	// progress wired to stderr) is built inside runWrite.
	var (
		sess     driver.Session
		closeAll func() error
	)
	if *fake {
		sess, closeAll, err = openFakeSession(ctx, *model)
	} else {
		sess, closeAll, err = openRealSession(ctx, *model, *port)
	}
	if err != nil {
		if isCancelled(err) {
			fmt.Fprintln(stderr, "rigprog write: cancelled")
			return exitError
		}
		fmt.Fprintf(stderr, "rigprog write: %v\n", err)
		return exitError
	}
	defer func() { _ = closeAll() }()

	return runWrite(ctx, *model, sess, snapshotDir, file, *yes, *firmware, isStdinTTY(stdin), stdin, stdout, stderr)
}
