// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/clone"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
)

// --- countSendable ---

// TestCountSendable pins task-14 brief §1 step 5's definition: "Added +
// Modified entries that are not Blocked" — Unchanged/Erased entries, and
// any Blocked entry regardless of Kind, never count.
func TestCountSendable(t *testing.T) {
	diff := codeplug.DiffResult{
		Entries: []codeplug.DiffEntry{
			{Slot: "001", Kind: codeplug.DiffAdded},
			{Slot: "002", Kind: codeplug.DiffModified},
			{Slot: "003", Kind: codeplug.DiffModified, Blocked: true, BlockReason: "unwritable field"},
			{Slot: "004", Kind: codeplug.DiffUnchanged},
			{Slot: "005", Kind: codeplug.DiffErased, Blocked: true},
		},
	}
	if got, want := countSendable(diff), 2; got != want {
		t.Errorf("countSendable = %d, want %d", got, want)
	}
}

func TestCountSendable_Zero(t *testing.T) {
	diff := codeplug.DiffResult{Entries: []codeplug.DiffEntry{
		{Slot: "001", Kind: codeplug.DiffUnchanged},
		{Slot: "002", Kind: codeplug.DiffAdded, Blocked: true},
	}}
	if got := countSendable(diff); got != 0 {
		t.Errorf("countSendable = %d, want 0", got)
	}
}

// --- hasBlockedErase / writeNothingSendableReport (task-25 brief) ---

// TestHasBlockedErase pins the exact gate for appending
// eraseFrontPanelProcedure: only a Blocked DiffErased entry counts — an
// unblocked erase, or a Blocked entry of any other Kind, does not.
func TestHasBlockedErase(t *testing.T) {
	cases := []struct {
		name    string
		entries []codeplug.DiffEntry
		want    bool
	}{
		{
			name: "blocked erase present",
			entries: []codeplug.DiffEntry{
				{Slot: "001", Kind: codeplug.DiffModified},
				{Slot: "501", Kind: codeplug.DiffErased, Blocked: true, BlockReason: "erase not supported on this radio"},
			},
			want: true,
		},
		{
			name: "erase present but not blocked",
			entries: []codeplug.DiffEntry{
				{Slot: "501", Kind: codeplug.DiffErased, Blocked: false},
			},
			want: false,
		},
		{
			name: "blocked, but not an erase",
			entries: []codeplug.DiffEntry{
				{Slot: "501", Kind: codeplug.DiffModified, Blocked: true, BlockReason: "bank 60M is read-only"},
			},
			want: false,
		},
		{
			name:    "no entries",
			entries: nil,
			want:    false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasBlockedErase(tc.entries); got != tc.want {
				t.Errorf("hasBlockedErase() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestWriteNothingSendableReport pins the blocked-only nothing-to-send
// rendering (task-25 brief, adjudicated remedy for the reported "i don't
// seem to be able to send deletes to the radio" defect): every Blocked
// entry's slot, kind, and reason, PLUS the front-panel erase procedure
// when a blocked erase is among them, and NEVER the plain "Nothing to
// send." wording (that message falsely claims parity with the radio).
func TestWriteNothingSendableReport(t *testing.T) {
	diff := codeplug.DiffResult{
		Entries: []codeplug.DiffEntry{
			{Slot: "001", Kind: codeplug.DiffUnchanged},
			{Slot: "501", Kind: codeplug.DiffErased, Blocked: true, BlockReason: "erase not supported on this radio"},
		},
		Blocked: 1, Unchanged: 1, Erased: 1,
	}
	var buf bytes.Buffer
	writeNothingSendableReport(&buf, "FT-710", diff)
	out := buf.String()

	for _, want := range []string{
		"5-01",                              // DisplaySlot form, not the wire slot
		"erased",                            // the Kind
		"erase not supported on this radio", // the BlockReason
		"[V/M]", "[ERASE]",                  // the front-panel procedure
		"still differs from the radio", // never claims parity
	} {
		if !strings.Contains(out, want) {
			t.Errorf("writeNothingSendableReport output = %q, want it to contain %q", out, want)
		}
	}
	if strings.Contains(out, "Nothing to send.") {
		t.Errorf("writeNothingSendableReport output = %q, want it NOT to contain the plain \"Nothing to send.\" (false parity claim)", out)
	}
}

// TestWriteNothingSendableReport_NoEraseProcedureWithoutBlockedErase pins
// the procedure's own gate: a blocked-only plan with no erase among the
// blocked entries must not print the erase-specific front-panel line at
// all (it would be actionable advice for the wrong problem).
func TestWriteNothingSendableReport_NoEraseProcedureWithoutBlockedErase(t *testing.T) {
	diff := codeplug.DiffResult{
		Entries: []codeplug.DiffEntry{
			{Slot: "501", Kind: codeplug.DiffModified, Blocked: true, BlockReason: "bank 60M is read-only"},
		},
		Blocked: 1,
	}
	var buf bytes.Buffer
	writeNothingSendableReport(&buf, "FT-710", diff)
	out := buf.String()
	if !strings.Contains(out, "bank 60M is read-only") {
		t.Errorf("writeNothingSendableReport output = %q, want the BlockReason", out)
	}
	if strings.Contains(out, "[V/M]") || strings.Contains(out, "[ERASE]") {
		t.Errorf("writeNothingSendableReport output = %q, want NO front-panel erase procedure (no blocked erase entry)", out)
	}
}

// TestWriteNothingSendableReport_UnknownModelOmitsProcedure pins task 40's
// radiotext migration: a model radiotext.For has no entry for (never
// reachable in production — validateModel already refuses an unknown
// --model before any session opens) still renders the blocked-slot list
// in full, but silently omits the front-panel erase procedure rather
// than fabricating generic advisory copy for a model this build knows
// nothing about.
func TestWriteNothingSendableReport_UnknownModelOmitsProcedure(t *testing.T) {
	diff := codeplug.DiffResult{
		Entries: []codeplug.DiffEntry{
			{Slot: "501", Kind: codeplug.DiffErased, Blocked: true, BlockReason: "erase not supported on this radio"},
		},
		Blocked: 1, Erased: 1,
	}
	var buf bytes.Buffer
	writeNothingSendableReport(&buf, "FTdx10", diff)
	out := buf.String()
	if !strings.Contains(out, "erase not supported on this radio") {
		t.Errorf("writeNothingSendableReport(FTdx10) output = %q, want the BlockReason", out)
	}
	if strings.Contains(out, "[V/M]") || strings.Contains(out, "[ERASE]") {
		t.Errorf("writeNothingSendableReport(FTdx10) output = %q, want NO front-panel erase procedure (no radiotext entry for this model)", out)
	}
}

// --- writePlanSummary ---

// TestWritePlanSummary pins task-14 brief §1 step 4: the diff (reusing
// Task 12's writeDiffReport — no duplicated rendering logic), the
// snapshot path, and the truncated baseline digest labelled as the
// plan's baseline.
func TestWritePlanSummary(t *testing.T) {
	diff := codeplug.DiffResult{
		Entries:   []codeplug.DiffEntry{{Slot: "001", Kind: codeplug.DiffUnchanged}},
		Unchanged: 1,
	}
	var buf bytes.Buffer
	if err := writePlanSummary(&buf, diff, "/snaps/snapshot-1.orp.json", "0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatalf("writePlanSummary: unexpected error: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"No changes.", // writeDiffReport reused, not duplicated
		"/snaps/snapshot-1.orp.json",
		"0123456789ab (truncated, plan's baseline)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("writePlanSummary output = %q, want it to contain %q", out, want)
		}
	}
	if strings.Contains(out, "0123456789abcdef0123456789abcdef") {
		t.Errorf("writePlanSummary output = %q, want the FULL digest not to appear (only the truncated form)", out)
	}
}

// --- writeExecuteSummary ---

// TestWriteExecuteSummary pins task-14 brief §1 step 8's success
// summary: Written/Verified/SkippedBlocked/Unchanged, journal path,
// snapshot path.
func TestWriteExecuteSummary(t *testing.T) {
	report := &clone.Report{
		Written: 3, Verified: 3, SkippedBlocked: 1, Unchanged: 113,
		JournalPath: "/snaps/snapshot-1.jsonl",
	}
	var buf bytes.Buffer
	writeExecuteSummary(&buf, report, "/snaps/snapshot-1.orp.json")
	out := buf.String()

	for _, want := range []string{"3", "113", "/snaps/snapshot-1.jsonl", "/snaps/snapshot-1.orp.json"} {
		if !strings.Contains(out, want) {
			t.Errorf("writeExecuteSummary output = %q, want it to contain %q", out, want)
		}
	}
}

// --- writeSlotTable / writeAbortReport ---

// TestWriteAbortReport pins task-14 brief §1 step 8's abort rendering:
// slot, reason, the per-slot table (action + verify status + detail),
// journal path, snapshot path, and — the plan honesty rule — recovery
// guidance that says "snapshot", never "backup".
func TestWriteAbortReport(t *testing.T) {
	abortErr := &clone.AbortedError{
		Slot: "001", Reason: "clone: slot \"001\": read-back disagrees on tag",
		Cause: &clone.VerifyMismatchError{Slot: "001"},
	}
	report := &clone.Report{
		Written: 1, Verified: 0,
		Slots: []clone.SlotResult{
			{Slot: "001", Action: "write", VerifyOK: false, Detail: "mismatch"},
			{Slot: "002", Action: "skipped-blocked", Detail: "unwritable field"},
		},
		Aborted: true, AbortReason: abortErr.Reason,
		JournalPath: "/snaps/run.jsonl",
	}
	var buf bytes.Buffer
	writeAbortReport(&buf, abortErr, report, "/snaps/run.orp.json")
	out := buf.String()

	for _, want := range []string{
		"M-01", // codeplug.DisplaySlot("001")
		"M-02",
		"/snaps/run.jsonl",
		"/snaps/run.orp.json",
		"snapshot", // plan honesty rule
		"write",
		"skipped-blocked",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("writeAbortReport output = %q, want it to contain %q", out, want)
		}
	}
	if strings.Contains(strings.ToLower(out), "backup") {
		t.Errorf("writeAbortReport output = %q, want it NEVER to say \"backup\" (plan honesty rule)", out)
	}
}

// TestWriteAbortReport_CancelledPair pins step 9: a cancel-abort (Cause
// wraps context.Canceled) must say the in-flight write+verify pair
// completed and point at the journal.
func TestWriteAbortReport_CancelledPair(t *testing.T) {
	abortErr := &clone.AbortedError{Reason: "context cancelled before slot \"010\"", Cause: context.Canceled}
	report := &clone.Report{Written: 1, Verified: 1, JournalPath: "/snaps/run.jsonl"}

	var buf bytes.Buffer
	writeAbortReport(&buf, abortErr, report, "/snaps/run.orp.json")
	out := buf.String()

	if !strings.Contains(out, "completed") {
		t.Errorf("writeAbortReport (cancelled) = %q, want it to say the in-flight pair completed", out)
	}
	if !strings.Contains(out, "/snaps/run.jsonl") {
		t.Errorf("writeAbortReport (cancelled) = %q, want it to point at the journal", out)
	}
}

// TestWriteAbortReport_NoSlot pins the "abort happened between slots"
// case (AbortedError.Slot == "").
func TestWriteAbortReport_NoSlot(t *testing.T) {
	abortErr := &clone.AbortedError{Reason: "context cancelled before slot \"010\": context canceled", Cause: context.Canceled}
	report := &clone.Report{JournalPath: "/snaps/run.jsonl"}
	var buf bytes.Buffer
	writeAbortReport(&buf, abortErr, report, "/snaps/run.orp.json")
	if buf.Len() == 0 {
		t.Fatal("writeAbortReport wrote nothing")
	}
}

// --- refusalMessage / handleExecuteOutcome ---

// TestRefusalMessage_Mapping unit-tests task-14 brief §1 step 8's
// refusal-cause -> plain-language mapping directly against constructed
// typed errors — no live flow needed to reach each one.
func TestRefusalMessage_Mapping(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"stale baseline", &clone.StaleBaselineError{Reason: "x"}, "the radio's contents changed after the plan was prepared — re-run write"},
		{"session changed", &clone.SessionChangedError{Reason: "x"}, "re-run write"},
		{"candidate changed", &clone.CandidateChangedError{Reason: "x"}, "re-run write"},
		{"confirmation mismatch", &clone.ConfirmationMismatchError{Want: "a", Got: "b"}, "re-run write"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := refusalMessage(tc.err)
			if !strings.Contains(got, tc.want) {
				t.Errorf("refusalMessage(%T) = %q, want it to contain %q", tc.err, got, tc.want)
			}
		})
	}
}

// TestHandleExecuteOutcome_Success pins the err==nil path: exit 0, the
// success summary on stdout, nothing on stderr.
func TestHandleExecuteOutcome_Success(t *testing.T) {
	report := &clone.Report{Written: 2, Verified: 2, JournalPath: "/j"}
	var stdout, stderr bytes.Buffer
	got := handleExecuteOutcome(&stdout, &stderr, report, nil, "/s")
	if got != exitSuccess {
		t.Errorf("handleExecuteOutcome(success) = %d, want exitSuccess (%d)", got, exitSuccess)
	}
	if stdout.Len() == 0 {
		t.Error("handleExecuteOutcome(success): stdout is empty, want the success summary")
	}
	if stderr.Len() != 0 {
		t.Errorf("handleExecuteOutcome(success): stderr = %q, want empty", stderr.String())
	}
}

// TestHandleExecuteOutcome_Success_SkippedBlockedStillZero pins task-14
// brief §1 step 8's explicit rule: "SkippedBlocked > 0 is still exit 0
// — the blocked entries were displayed before confirmation (informed
// consent)". handleExecuteOutcome never inspects SkippedBlocked at all
// (only err == nil gates the success path), but this test pins the
// observable behaviour directly rather than relying on that
// implementation detail.
func TestHandleExecuteOutcome_Success_SkippedBlockedStillZero(t *testing.T) {
	report := &clone.Report{Written: 1, Verified: 1, SkippedBlocked: 3, JournalPath: "/j"}
	var stdout, stderr bytes.Buffer
	got := handleExecuteOutcome(&stdout, &stderr, report, nil, "/s")
	if got != exitSuccess {
		t.Errorf("handleExecuteOutcome(success, SkippedBlocked=3) = %d, want exitSuccess (%d)", got, exitSuccess)
	}
	if !strings.Contains(stdout.String(), "SkippedBlocked: 3") {
		t.Errorf("handleExecuteOutcome(success, SkippedBlocked=3): stdout = %q, want it to show the count", stdout.String())
	}
}

// TestHandleExecuteOutcome_Refusals table-drives every pre-write refusal
// sentinel to exit 4, message on stderr.
func TestHandleExecuteOutcome_Refusals(t *testing.T) {
	for _, err := range []error{
		&clone.StaleBaselineError{Reason: "x"},
		&clone.SessionChangedError{Reason: "x"},
		&clone.CandidateChangedError{Reason: "x"},
		&clone.ConfirmationMismatchError{Want: "a", Got: "b"},
	} {
		t.Run(errorTypeName(err), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			got := handleExecuteOutcome(&stdout, &stderr, nil, err, "/s")
			if got != exitRefused {
				t.Errorf("handleExecuteOutcome(%T) = %d, want exitRefused (%d)", err, got, exitRefused)
			}
			if stdout.Len() != 0 {
				t.Errorf("handleExecuteOutcome(%T): stdout = %q, want empty", err, stdout.String())
			}
			if stderr.Len() == 0 {
				t.Errorf("handleExecuteOutcome(%T): stderr is empty, want a refusal message", err)
			}
		})
	}
}

// TestHandleExecuteOutcome_FirmwareUnconfirmed pins the defensive case
// Fix 2 (adjudicated MEDIUM, Codex M4 #2) names explicitly:
// ErrFirmwareUnconfirmed reaching handleExecuteOutcome at all (the
// ordinary path handles it earlier, in executeAndReport, via the
// firmware prompt/retry) is still one of the documented refusal
// sentinels — exit 4, not the generic exit 1 bucket.
func TestHandleExecuteOutcome_FirmwareUnconfirmed(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := handleExecuteOutcome(&stdout, &stderr, nil, clone.ErrFirmwareUnconfirmed, "/s")
	if got != exitRefused {
		t.Errorf("handleExecuteOutcome(ErrFirmwareUnconfirmed) = %d, want exitRefused (%d)", got, exitRefused)
	}
	if stderr.Len() == 0 {
		t.Error("handleExecuteOutcome(ErrFirmwareUnconfirmed): stderr is empty, want a refusal message")
	}
}

// TestHandleExecuteOutcome_ValidationFailed pins Fix 2 (adjudicated
// MEDIUM, Codex M4 #2): Execute's own re-validation (obligation 3) can
// return a *clone.ValidationFailedError, exactly like PrepareSend's —
// this must map to exit 3 (writeValidationIssues on stderr), not the
// blanket exit 4 the pre-fix code gave every non-AbortedError.
func TestHandleExecuteOutcome_ValidationFailed(t *testing.T) {
	err := &clone.ValidationFailedError{Issues: []codeplug.Issue{
		{Slot: "001", Field: "frequency", Severity: codeplug.SeverityError, Msg: "frequency must be greater than 0 Hz"},
	}}
	var stdout, stderr bytes.Buffer
	got := handleExecuteOutcome(&stdout, &stderr, nil, err, "/s")
	if got != exitBlocked {
		t.Errorf("handleExecuteOutcome(ValidationFailedError) = %d, want exitBlocked (%d)", got, exitBlocked)
	}
	if !strings.Contains(stderr.String(), "M-01") || !strings.Contains(stderr.String(), "frequency") {
		t.Errorf("handleExecuteOutcome(ValidationFailedError) stderr = %q, want the issue rendered", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("handleExecuteOutcome(ValidationFailedError): stdout = %q, want empty", stdout.String())
	}
}

// TestHandleExecuteOutcome_Cancelled pins Fix 2: Execute's pre-operation
// ctx.Err() check wraps context.Canceled/DeadlineExceeded in a plain
// fmt.Errorf (core/clone/execute.go), not an AbortedError — isCancelled
// must catch it and map to exit 1 with the same "cancelled" wording
// every other radio-touching subcommand uses, not exit 4.
func TestHandleExecuteOutcome_Cancelled(t *testing.T) {
	err := fmt.Errorf("clone: Execute: %w", context.Canceled)
	var stdout, stderr bytes.Buffer
	got := handleExecuteOutcome(&stdout, &stderr, nil, err, "/s")
	if got != exitError {
		t.Errorf("handleExecuteOutcome(cancelled) = %d, want exitError (%d)", got, exitError)
	}
	if !strings.Contains(stderr.String(), "cancelled") {
		t.Errorf("handleExecuteOutcome(cancelled) stderr = %q, want it to say \"cancelled\"", stderr.String())
	}
}

// TestHandleExecuteOutcome_Busy pins Fix 2: a *clone.BusyError (a
// concurrent ReadAll/PrepareSend/Execute call refused this Service's
// try-lock) is an operational failure, not a pre-write content refusal
// — exit 1, not exit 4.
func TestHandleExecuteOutcome_Busy(t *testing.T) {
	err := &clone.BusyError{InProgress: "ReadAll"}
	var stdout, stderr bytes.Buffer
	got := handleExecuteOutcome(&stdout, &stderr, nil, err, "/s")
	if got != exitError {
		t.Errorf("handleExecuteOutcome(BusyError) = %d, want exitError (%d)", got, exitError)
	}
	if !strings.Contains(stderr.String(), "ReadAll") {
		t.Errorf("handleExecuteOutcome(BusyError) stderr = %q, want it to name the in-progress operation", stderr.String())
	}
}

// TestHandleExecuteOutcome_Unrecognised pins Fix 2's final bucket: any
// error that is not the AbortedError/ValidationFailedError/cancellation/
// BusyError/refusal-sentinel case is a generic exit 1, NOT exit 4 (the
// pre-fix blanket mapping this whole fix replaces).
func TestHandleExecuteOutcome_Unrecognised(t *testing.T) {
	err := errors.New("some unexpected transport error")
	var stdout, stderr bytes.Buffer
	got := handleExecuteOutcome(&stdout, &stderr, nil, err, "/s")
	if got != exitError {
		t.Errorf("handleExecuteOutcome(unrecognised) = %d, want exitError (%d)", got, exitError)
	}
	if !strings.Contains(stderr.String(), "some unexpected transport error") {
		t.Errorf("handleExecuteOutcome(unrecognised) stderr = %q, want the error message", stderr.String())
	}
}

// errorTypeName is a small test helper naming err's concrete type for
// subtest names.
func errorTypeName(err error) string {
	switch err.(type) {
	case *clone.StaleBaselineError:
		return "StaleBaseline"
	case *clone.SessionChangedError:
		return "SessionChanged"
	case *clone.CandidateChangedError:
		return "CandidateChanged"
	case *clone.ConfirmationMismatchError:
		return "ConfirmationMismatch"
	default:
		return "unknown"
	}
}

// TestHandleExecuteOutcome_Aborted pins the *clone.AbortedError path:
// exit 5, delegating to writeAbortReport on stderr, using report even
// though err is also non-nil.
func TestHandleExecuteOutcome_Aborted(t *testing.T) {
	abortErr := &clone.AbortedError{Slot: "001", Reason: "boom"}
	report := &clone.Report{Aborted: true, AbortReason: "boom", JournalPath: "/j"}
	var stdout, stderr bytes.Buffer
	got := handleExecuteOutcome(&stdout, &stderr, report, abortErr, "/s")
	if got != exitAborted {
		t.Errorf("handleExecuteOutcome(aborted) = %d, want exitAborted (%d)", got, exitAborted)
	}
	if !strings.Contains(stderr.String(), "M-01") {
		t.Errorf("handleExecuteOutcome(aborted) stderr = %q, want the slot rendered", stderr.String())
	}
}

// --- writeValidationIssues ---

// TestWriteValidationIssues pins task-14 brief §1 step 3's validation
// rendering: every Issue (slot, field, severity, message), in order.
func TestWriteValidationIssues(t *testing.T) {
	issues := []codeplug.Issue{
		{Slot: "001", Field: "frequency", Severity: codeplug.SeverityError, Msg: "frequency must be greater than 0 Hz"},
		{Slot: "", Field: "", Severity: codeplug.SeverityWarning, Msg: "codeplug-level note"},
	}
	var buf bytes.Buffer
	writeValidationIssues(&buf, issues)
	out := buf.String()
	for _, want := range []string{"M-01", "frequency", "error", "frequency must be greater than 0 Hz", "warning", "codeplug-level note"} {
		if !strings.Contains(out, want) {
			t.Errorf("writeValidationIssues output = %q, want it to contain %q", out, want)
		}
	}
}

// --- isStdinTTY ---

// TestIsStdinTTY_NonFile pins the non-*os.File case (bytes/strings
// readers, exactly what run()'s in-process tests and black-box piped
// stdin both use) as non-TTY.
func TestIsStdinTTY_NonFile(t *testing.T) {
	if isStdinTTY(strings.NewReader("yes\n")) {
		t.Error("isStdinTTY(strings.Reader) = true, want false")
	}
	if isStdinTTY(bytes.NewReader(nil)) {
		t.Error("isStdinTTY(bytes.Reader) = true, want false")
	}
}

// TestIsStdinTTY_RegularFile pins the *os.File-but-not-a-character-device
// case (a plain regular file) as non-TTY — an *os.File alone is not
// sufficient, only a character device counts.
func TestIsStdinTTY_RegularFile(t *testing.T) {
	tmp, err := os.CreateTemp(t.TempDir(), "rigprog-tty-test-*")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer tmp.Close()
	if isStdinTTY(tmp) {
		t.Error("isStdinTTY(regular file) = true, want false")
	}
}

// TestIsStdinTTY_DevNullQuirk_ResolveConfirmationFallsBackToNonInteractive
// documents WHY Fix 4 (adjudicated MEDIUM, Codex M4 #4) exists, and pins
// its actual remedy: /dev/null IS itself a character device on Unix, so
// isStdinTTY misclassifies it as an interactive terminal — but
// resolveConfirmation, given a reader that immediately hits EOF (exactly
// what reading from /dev/null does), still falls back to the correct
// non-interactive refusal (exitUsage) rather than trusting isStdinTTY's
// (wrong, here) verdict and treating an EOF read as a declined
// confirmation (which the pre-fix code did, at exitRefused/4 — see
// write_test.go's git history for the case this replaces).
func TestIsStdinTTY_DevNullQuirk_ResolveConfirmationFallsBackToNonInteractive(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Skipf("cannot open %s in this environment: %v", os.DevNull, err)
	}
	defer f.Close()
	if !isStdinTTY(f) {
		t.Skip("isStdinTTY(/dev/null) = false in this environment — the quirk this test documents does not reproduce here (the EOF-based fallback is still exercised by TestResolveConfirmation's own EOF case)")
	}

	var stdout, stderr bytes.Buffer
	in := bufio.NewReader(f)
	proceed, code := resolveConfirmation(in, &stdout, &stderr, true /* isTTY, per the quirk above */, false, 1)
	if proceed {
		t.Error("resolveConfirmation(/dev/null quirk) proceed = true, want false")
	}
	if code != exitUsage {
		t.Errorf("resolveConfirmation(/dev/null quirk) code = %d, want exitUsage (%d) — an EOF read must be treated as non-interactive, not a declined confirmation", code, exitUsage)
	}
	if !strings.Contains(stderr.String(), "--yes") {
		t.Errorf("resolveConfirmation(/dev/null quirk) stderr = %q, want it to mention --yes", stderr.String())
	}
}

// --- readLine ---

func TestReadLine(t *testing.T) {
	cases := []struct {
		name     string
		in, want string
		wantEOF  bool
	}{
		{"lf", "yes\n", "yes", false},
		{"crlf", "yes\r\n", "yes", false},
		{"no trailing newline (EOF)", "yes", "yes", true},
		{"empty", "", "", true},
		{"empty line", "\n", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, eof := readLine(bufio.NewReader(strings.NewReader(tc.in)))
			if got != tc.want {
				t.Errorf("readLine(%q) line = %q, want %q", tc.in, got, tc.want)
			}
			if eof != tc.wantEOF {
				t.Errorf("readLine(%q) eof = %v, want %v", tc.in, eof, tc.wantEOF)
			}
		})
	}
}

// --- resolveConfirmation ---

// TestResolveConfirmation covers the confirmation gate (task-14 brief §1
// step 6) at the function level with an injected isTTY flag. THIS is the
// decline-coverage this package relies on (Fix 7, adjudicated LOW, Codex
// M4 #7): the "tty, declined/wrong case/empty line" cases below are the
// ONLY place an isTTY==true caller who types an answer other than
// exactly "yes" is exercised. No black-box test can drive that same
// branch — every runBinary/runBinaryStdin invocation's stdin is either a
// pipe or a real /dev/null, and isStdinTTY correctly (or, for
// /dev/null, via the Fix 4 EOF fallback) treats both as non-interactive,
// so a black-box "write" without --yes always takes the non-interactive
// branch instead (see blackbox_test.go's "piped stdin without yes"/
// "real /dev/null stdin without yes" subtests — their own doc comments
// disclaim proving decline, deferring to this test). The "tty, EOF" case
// pins Fix 4 (adjudicated MEDIUM, Codex M4 #4) itself: an EOF read on an
// (allegedly) interactive stdin is reclassified as non-interactive,
// exitUsage — exactly the case a genuine "yes"/"no" typed decline above
// must NOT be confused with.
func TestResolveConfirmation(t *testing.T) {
	cases := []struct {
		name        string
		isTTY, yes  bool
		stdin       string
		wantProceed bool
		wantCode    int
		wantStderr  string
	}{
		{"yes flag short-circuits even non-tty", false, true, "", true, exitSuccess, ""},
		{"non-tty without --yes is refused", false, false, "", false, exitUsage, "--yes"},
		{"tty, exact yes, proceeds", true, false, "yes\n", true, exitSuccess, ""},
		{"tty, declined, cancelled", true, false, "no\n", false, exitRefused, "cancelled, nothing sent"},
		{"tty, wrong case, declined", true, false, "YES\n", false, exitRefused, "cancelled, nothing sent"},
		{"tty, empty line, declined", true, false, "\n", false, exitRefused, "cancelled, nothing sent"},
		{"tty, EOF (no input at all), reclassified non-interactive", true, false, "", false, exitUsage, "--yes"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			in := bufio.NewReader(strings.NewReader(tc.stdin))
			proceed, code := resolveConfirmation(in, &stdout, &stderr, tc.isTTY, tc.yes, 3)
			if proceed != tc.wantProceed {
				t.Errorf("resolveConfirmation(%+v) proceed = %v, want %v", tc, proceed, tc.wantProceed)
			}
			if !proceed && code != tc.wantCode {
				t.Errorf("resolveConfirmation(%+v) code = %d, want %d", tc, code, tc.wantCode)
			}
			if tc.wantStderr != "" && !strings.Contains(stderr.String(), tc.wantStderr) {
				t.Errorf("resolveConfirmation(%+v) stderr = %q, want it to contain %q", tc, stderr.String(), tc.wantStderr)
			}
		})
	}
}

// --- promptFirmwareVersion ---

func TestPromptFirmwareVersion_NonTTY(t *testing.T) {
	var stdout, stderr bytes.Buffer
	in := bufio.NewReader(strings.NewReader(""))
	version, ok, err := promptFirmwareVersion(in, false, &stdout, &stderr)
	if ok {
		t.Error("promptFirmwareVersion(non-tty) ok = true, want false")
	}
	if err != nil {
		t.Errorf("promptFirmwareVersion(non-tty) err = %v, want nil", err)
	}
	if version != "" {
		t.Errorf("promptFirmwareVersion(non-tty) version = %q, want empty", version)
	}
	if !strings.Contains(stderr.String(), "--firmware") {
		t.Errorf("promptFirmwareVersion(non-tty) stderr = %q, want it to mention --firmware", stderr.String())
	}
}

func TestPromptFirmwareVersion_TTY(t *testing.T) {
	var stdout, stderr bytes.Buffer
	in := bufio.NewReader(strings.NewReader("V01-10\n"))
	version, ok, err := promptFirmwareVersion(in, true, &stdout, &stderr)
	if !ok || version != "V01-10" {
		t.Errorf("promptFirmwareVersion(tty, \"V01-10\") = (%q, %v), want (\"V01-10\", true)", version, ok)
	}
	if err != nil {
		t.Errorf("promptFirmwareVersion(tty) err = %v, want nil", err)
	}
	if stderr.Len() == 0 {
		t.Error("promptFirmwareVersion(tty): stderr is empty, want the front-panel/SD-card guidance")
	}
}

func TestPromptFirmwareVersion_TTY_Empty(t *testing.T) {
	var stdout, stderr bytes.Buffer
	in := bufio.NewReader(strings.NewReader("\n"))
	version, ok, err := promptFirmwareVersion(in, true, &stdout, &stderr)
	if ok {
		t.Error("promptFirmwareVersion(tty, empty) ok = true, want false")
	}
	if err != nil {
		t.Errorf("promptFirmwareVersion(tty, empty) err = %v, want nil", err)
	}
	if version != "" {
		t.Errorf("promptFirmwareVersion(tty, empty) version = %q, want empty", version)
	}
}

// TestPromptFirmwareVersion_TTY_EOF pins Fix 4 (adjudicated MEDIUM,
// Codex M4 #4): an EOF read (no input at all — distinct from an empty
// LINE, which TestPromptFirmwareVersion_TTY_Empty above covers) is
// reclassified as non-interactive and gets the SAME guidance the
// non-TTY path prints (mentioning --firmware), rather than the
// unhelpful "no firmware version entered" wording. ok=false either way
// — the firmware prompt keeps its documented non-interactive exit code
// (4), unlike the confirmation prompt's EOF case (exit 2).
func TestPromptFirmwareVersion_TTY_EOF(t *testing.T) {
	var stdout, stderr bytes.Buffer
	in := bufio.NewReader(strings.NewReader(""))
	version, ok, err := promptFirmwareVersion(in, true, &stdout, &stderr)
	if ok {
		t.Error("promptFirmwareVersion(tty, EOF) ok = true, want false")
	}
	if err != nil {
		t.Errorf("promptFirmwareVersion(tty, EOF) err = %v, want nil", err)
	}
	if version != "" {
		t.Errorf("promptFirmwareVersion(tty, EOF) version = %q, want empty", version)
	}
	if !strings.Contains(stderr.String(), "--firmware") {
		t.Errorf("promptFirmwareVersion(tty, EOF) stderr = %q, want the same guidance as the non-TTY path (mentioning --firmware)", stderr.String())
	}
}

// TestPromptFirmwareVersion_WriteFailure pins Fix 1 (adjudicated HIGH,
// Codex M4 #1): a writer that cannot even render the firmware guidance
// text must be reported via the new err return, distinct from an
// ordinary ok==false refusal — executeAndReport treats the two
// differently (exitError vs exitRefused).
func TestPromptFirmwareVersion_WriteFailure(t *testing.T) {
	var stdout bytes.Buffer
	stderr := &failingWriter{err: errors.New("simulated broken stderr")}
	in := bufio.NewReader(strings.NewReader("V01-10\n"))
	version, ok, err := promptFirmwareVersion(in, true, &stdout, stderr)
	if ok {
		t.Error("promptFirmwareVersion(failing writer) ok = true, want false")
	}
	if version != "" {
		t.Errorf("promptFirmwareVersion(failing writer) version = %q, want empty", version)
	}
	if err == nil {
		t.Error("promptFirmwareVersion(failing writer) err = nil, want the writer's error")
	}
}

// --- cmdWrite usage-error paths (fast, no radio touched — mirrors
// cmdDiff's own tests in diff_test.go) ---

func TestCmdWrite_NeitherPortNorFake(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdWrite(testCtx(t), []string{"somefile.json"}, strings.NewReader(""), &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdWrite(no --port/--fake) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
}

func TestCmdWrite_BothPortAndFake(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdWrite(testCtx(t), []string{"--port", "/dev/cu.fake", "--fake", "somefile.json"}, strings.NewReader(""), &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdWrite(both) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
}

func TestCmdWrite_MissingFileArgument(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdWrite(testCtx(t), []string{"--fake"}, strings.NewReader(""), &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdWrite(--fake, no FILE) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
}

func TestCmdWrite_TooManyArguments(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdWrite(testCtx(t), []string{"--fake", "a.json", "b.json"}, strings.NewReader(""), &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdWrite(--fake, 2 FILEs) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
}

// TestCmdWrite_UnknownModel pins task 40's --model validation for write:
// an unrecognised model exits 2 (usage), naming FT-710, before FILE is
// even loaded (a nonexistent path is used deliberately to prove
// rejection happens before any file I/O is attempted).
func TestCmdWrite_UnknownModel(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdWrite(testCtx(t), []string{"--fake", "--model", "FTdx10", "/nonexistent/rigprog-test.json"}, strings.NewReader(""), &stdout, &stderr)
	if got != exitUsage {
		t.Fatalf("cmdWrite(--model FTdx10) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
	if !strings.Contains(stderr.String(), "FTdx10") || !strings.Contains(stderr.String(), "FT-710") {
		t.Errorf("cmdWrite(--model FTdx10) stderr = %q, want it to name both the rejected and supported model", stderr.String())
	}
}

func TestCmdWrite_LoadNonexistentFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdWrite(testCtx(t), []string{"--fake", "/nonexistent/path/rigprog-test.json"}, strings.NewReader(""), &stdout, &stderr)
	if got != exitError {
		t.Errorf("cmdWrite(nonexistent file) = %d, want exitError (%d); stderr=%q", got, exitError, stderr.String())
	}
}

func TestCmdWrite_SchemaTooNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "too-new.json")
	tooNew := &codeplug.Codeplug{Schema: codeplug.CurrentSchema + 1}
	if err := codeplug.Save(path, tooNew); err != nil {
		t.Fatalf("Save fixture: %v", err)
	}

	var stdout, stderr bytes.Buffer
	got := cmdWrite(testCtx(t), []string{"--fake", path}, strings.NewReader(""), &stdout, &stderr)
	if got != exitError {
		t.Errorf("cmdWrite(schema too new) = %d, want exitError (%d); stderr=%q", got, exitError, stderr.String())
	}
	if !strings.Contains(stderr.String(), "newer") {
		t.Errorf("cmdWrite(schema too new) stderr = %q, want it to mention the file is newer than supported", stderr.String())
	}
}

// TestCmdWrite_CancelledBeforeStart mirrors TestCmdRead_CancelledBeforeStart/
// TestCmdDiff_CancelledBeforeStart: a context already cancelled before
// cmdWrite opens a session yields exit 1 and a "cancelled" message — the
// FILE load happens first (step 1), so this uses a minimal valid fixture.
func TestCmdWrite_CancelledBeforeStart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.json")
	fixture := &codeplug.Codeplug{Schema: codeplug.CurrentSchema}
	if err := codeplug.Save(path, fixture); err != nil {
		t.Fatalf("Save fixture: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var stdout, stderr bytes.Buffer
	got := cmdWrite(ctx, []string{"--fake", path}, strings.NewReader(""), &stdout, &stderr)
	if got != exitError {
		t.Errorf("cmdWrite(cancelled ctx) = %d, want exitError (%d); stderr=%q", got, exitError, stderr.String())
	}
	if !strings.Contains(stderr.String(), "cancelled") {
		t.Errorf("cmdWrite(cancelled ctx) stderr = %q, want it to say \"cancelled\"", stderr.String())
	}
}

func TestCmdWrite_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdWrite(testCtx(t), []string{"-h"}, strings.NewReader(""), &stdout, &stderr)
	if got != exitSuccess {
		t.Errorf("cmdWrite([-h]) = %d, want exitSuccess (%d)", got, exitSuccess)
	}
	if stdout.Len() == 0 {
		t.Error("cmdWrite([-h]): stdout is empty, want usage text")
	}
}
