// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/clone"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// cmdDiff implements "rigprog diff" (task-12 brief §2): load a codeplug
// file, read a fresh baseline from a real or simulated radio, and report
// how the file differs from that baseline. Read-only and side-effect-free
// — it never snapshots, journals, or calls PrepareSend.
func cmdDiff(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // this function owns all usage/error output.
	port := fs.String("port", "", "real serial port device path")
	fake := fs.Bool("fake", false, "use the in-process simulated radio")
	model := fs.String("model", wiring.DefaultModel, "radio model to target")

	if ok, code := parseArgs(fs, args, "diff", printDiffUsage, stdout, stderr); !ok {
		return code
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "rigprog diff: exactly one FILE argument is required")
		printDiffUsage(stderr)
		return exitUsage
	}
	file := fs.Arg(0)

	if !validateSessionArgs(stderr, "diff", *model, *port, *fake, printDiffUsage) {
		return exitUsage
	}

	// loadCodeplugStrict (fileio.go) is the one shared place strict-Load
	// error rendering lives, including the distinct schema-too-new
	// message — task-13's export/import reuse it verbatim for their own
	// FILE/--into loads.
	candidate, code := loadCodeplugStrict(stderr, "diff", "", file)
	if candidate == nil {
		return code
	}
	// Same gap as the baseline check below, mirrored: a candidate FILE
	// saved from a partial ReadAll has the same unreturned-slot hole as a
	// partial baseline, and Diff's inventory check cannot tell it from an
	// ordinary unchanged/added slot either. Checked before openSession so
	// a bad file is refused without touching the radio.
	if n := len(candidate.Radio.FailedSlots); n > 0 {
		fmt.Fprintf(stderr, "rigprog diff: %s is a partial read (%d slot(s) failed); re-read the radio and try again\n", file, n)
		return exitError
	}

	sess, closeAll, err := openSession(ctx, *model, *port, *fake)
	if err != nil {
		if isCancelled(err) {
			fmt.Fprintln(stderr, "rigprog diff: cancelled")
			return exitError
		}
		fmt.Fprintf(stderr, "rigprog diff: %v\n", err)
		return exitError
	}
	defer func() { _ = closeAll() }()

	// diff never snapshots or journals (task-12 brief §2): the
	// SnapshotStore clone.NewService requires is unused here, so its Dir
	// is left empty rather than resolved/created like read's.
	svc := clone.NewService(sess, clone.SnapshotStore{}, clone.WithProgress(progressPrinter(stderr)))

	baseline, err := svc.ReadAll(ctx)
	if err != nil {
		if isCancelled(err) {
			fmt.Fprintln(stderr, "rigprog diff: cancelled")
			return exitError
		}
		fmt.Fprintf(stderr, "rigprog diff: %v\n", err)
		return exitError
	}

	// A partial baseline hits the exact same checkInventory gap PrepareSend
	// refuses against (core/clone/plan.go): a slot ReadAll never returned
	// at all is indistinguishable from an ordinary unchanged/added slot to
	// Diff's inventory check. diff has no exitBlocked/exitRefused concept
	// of its own, so this is exitError like every other failure branch here.
	if n := len(baseline.Radio.FailedSlots); n > 0 {
		fmt.Fprintf(stderr, "rigprog diff: baseline read is partial (%d slot(s) failed); re-read the radio and try again\n", n)
		return exitError
	}

	result, err := codeplug.Diff(baseline, candidate, sess.Capabilities())
	if err != nil {
		fmt.Fprintf(stderr, "rigprog diff: file and radio have different slot inventories: %v\n", err)
		return exitError
	}

	// Best-effort: diff is read-only and always exits 0 once the diff
	// itself was computed — there is no Execute here for a failed render
	// to wrongly gate (contrast write.go's writePlanSummary, which is
	// exactly why writeDiffReport now returns an error at all).
	_ = writeDiffReport(stdout, result)
	return exitSuccess
}

// filterEntries returns the subset of entries whose Kind == kind, in
// their original (baseline slot) order.
func filterEntries(entries []codeplug.DiffEntry, kind codeplug.DiffKind) []codeplug.DiffEntry {
	var out []codeplug.DiffEntry
	for _, e := range entries {
		if e.Kind == kind {
			out = append(out, e)
		}
	}
	return out
}

// writeDiffReport renders a DiffResult to w: grouped Added/Modified/
// Erased sections (only non-empty ones, or "No changes." when there are
// none), then a count line including Unchanged (task-12 brief §2 — a
// stable, script-parseable line: "Added N, Modified N, Erased N, Blocked
// N, Unchanged N").
//
// Returns the first write error w returned, if any (Fix 1, adjudicated
// HIGH, Codex M4 #1) — writePlanSummary (write.go) is the caller that
// actually needs this to gate Execute; cmdDiff below stays best-effort
// (diff has no Execute to protect, and always exits 0 once the diff
// itself was computed).
func writeDiffReport(w io.Writer, result codeplug.DiffResult) error {
	tw := &errTrackingWriter{w: w}
	added := filterEntries(result.Entries, codeplug.DiffAdded)
	modified := filterEntries(result.Entries, codeplug.DiffModified)
	erased := filterEntries(result.Entries, codeplug.DiffErased)

	if len(added) == 0 && len(modified) == 0 && len(erased) == 0 {
		fmt.Fprintln(tw, "No changes.")
	} else {
		if len(added) > 0 {
			fmt.Fprintln(tw, "Added:")
			for _, e := range added {
				writeAddedEntry(tw, e)
			}
		}
		if len(modified) > 0 {
			fmt.Fprintln(tw, "Modified:")
			for _, e := range modified {
				writeModifiedEntry(tw, e)
			}
		}
		if len(erased) > 0 {
			fmt.Fprintln(tw, "Erased:")
			for _, e := range erased {
				writeErasedEntry(tw, e)
			}
		}
	}

	fmt.Fprintln(tw)
	fmt.Fprintf(tw, "Added %d, Modified %d, Erased %d, Blocked %d, Unchanged %d\n",
		result.Added, result.Modified, result.Erased, result.Blocked, result.Unchanged)
	return tw.err
}

// writeAddedEntry renders one Added DiffEntry: display slot plus the new
// channel's frequency (Hz), mode, and quoted tag (task-12 brief §2's
// terse field set).
func writeAddedEntry(w io.Writer, e codeplug.DiffEntry) {
	fmt.Fprintf(w, "  %s: freq %d Hz, mode %s, tag %q\n", codeplug.DisplaySlot(e.Slot), e.After.FreqHz, e.After.Mode, e.After.Tag)
	writeBlockedAnnotation(w, e)
}

// writeModifiedEntry renders one Modified DiffEntry: display slot plus a
// terse before->after for whichever of frequency (Hz), mode and quoted tag
// ACTUALLY CHANGED.
//
// ONLY THE CHANGED FIELDS (v1.5.x follow-up (j)). This line used to print all
// three unconditionally, so a one-field edit read "freq 7030000→7030000 Hz,
// mode CW-U→CW-U, tag ""→"WINTEST"" — two thirds of it restating values that
// did not move, in the one place a reviewer looks to see what did.
//
// The no-named-field fallback is not defensive padding: Diff's equality is
// over the WHOLE ChannelData (its Equality doc), so a slot whose tone, shift
// or field state moved is Modified with all three of these fields equal, and
// a bare "M-01:" would claim nothing.
// TestWriteDiffReport_ModifiedNamesOnlyChangedFields pins all three shapes.
func writeModifiedEntry(w io.Writer, e codeplug.DiffEntry) {
	var parts []string
	if e.Before.FreqHz != e.After.FreqHz {
		parts = append(parts, fmt.Sprintf("freq %d→%d Hz", e.Before.FreqHz, e.After.FreqHz))
	}
	if e.Before.Mode != e.After.Mode {
		parts = append(parts, fmt.Sprintf("mode %s→%s", e.Before.Mode, e.After.Mode))
	}
	if e.Before.Tag != e.After.Tag {
		parts = append(parts, fmt.Sprintf("tag %q→%q", e.Before.Tag, e.After.Tag))
	}
	if len(parts) == 0 {
		parts = append(parts, "changed in a field this summary does not print")
	}
	fmt.Fprintf(w, "  %s: %s\n", codeplug.DisplaySlot(e.Slot), strings.Join(parts, ", "))
	writeBlockedAnnotation(w, e)
}

// writeErasedEntry renders one Erased DiffEntry: display slot, plus the
// honesty marking when Blocked — a Blocked erase will NOT actually
// happen; the slot keeps its current contents (task-12 brief §2). M5b
// made this marking permanent rather than provisional: NO CAT erase
// exists (HW-CONFIRMED 2026-07-13, docs/hardware-notes.md), so every
// erase entry is Blocked, always.
func writeErasedEntry(w io.Writer, e codeplug.DiffEntry) {
	fmt.Fprintf(w, "  %s: erased\n", codeplug.DisplaySlot(e.Slot))
	if e.Blocked {
		fmt.Fprintln(w, "    UNSUPPORTED — slot will keep its current contents")
	}
	writeBlockedAnnotation(w, e)
}

// writeBlockedAnnotation writes e's BlockReason, indented, when e is
// Blocked — task-12 brief §2: "Blocked entries always show BlockReason."
func writeBlockedAnnotation(w io.Writer, e codeplug.DiffEntry) {
	if e.Blocked {
		fmt.Fprintf(w, "    BLOCKED: %s\n", e.BlockReason)
	}
}
