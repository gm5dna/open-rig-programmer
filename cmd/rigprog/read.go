// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/gm5dna/open-rig-programmer/core/clone"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/internal/buildinfo"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// cliGeneratorPrefix is the stable prefix a CLI-set Codeplug.Generator
// always starts with (task-12 brief §1). What follows it is the build's
// version — "dev" for any build the release pipeline did not stamp, so a
// file written by a development build says so.
const cliGeneratorPrefix = "rigprog/"

// cliGeneratorID is what applyDefaultGenerator fills Codeplug.Generator
// with, when it is empty. See applyDefaultGenerator's doc comment: as of
// this task, clone.Service.ReadAll always sets Generator itself (to
// "open-rig-programmer/core/clone", never empty — core/clone/service.go's
// generatorID), so in practice this value is never observed in a file
// this command writes; it exists to honour task-12 brief §1's "if and
// only if the service leaves it empty" rule defensively, in case that
// changes. The offline "import" path, by contrast, sets it directly and
// IS observed (import.go).
//
// A var rather than a const because buildinfo.Version() is only known at
// link time; it is assigned once at init and never written again.
var cliGeneratorID = cliGeneratorPrefix + buildinfo.Version()

// applyDefaultGenerator sets cp.Generator to cliGeneratorID if and only
// if it is currently empty (task-12 brief §1) — it must never overwrite
// a Generator the read already populated. See cliGeneratorID's doc
// comment for why this branch is not reachable via any wiring this
// package currently offers.
func applyDefaultGenerator(cp *codeplug.Codeplug) {
	if cp.Generator == "" {
		cp.Generator = cliGeneratorID
	}
}

// countPopulated returns how many of channels are populated (see
// codeplug.Channel.Empty).
func countPopulated(channels []codeplug.Channel) int {
	n := 0
	for _, ch := range channels {
		if !ch.Empty() {
			n++
		}
	}
	return n
}

// truncateDigest returns digest's first 12 hex characters — task-12
// brief §1's "short baseline digest (first 12 hex chars is fine — label
// it truncated)" — or digest unchanged if it is already 12 characters or
// shorter.
func truncateDigest(digest string) string {
	const n = 12
	if len(digest) <= n {
		return digest
	}
	return digest[:n]
}

// writeReadSummary writes cmdRead's stdout success summary (task-12
// brief §1): slots read, populated count, region, truncated baseline
// digest, output path.
func writeReadSummary(w io.Writer, cp *codeplug.Codeplug, outPath string) {
	fmt.Fprintf(w, "Slots read:      %d\n", len(cp.Channels))
	fmt.Fprintf(w, "Populated:       %d\n", countPopulated(cp.Channels))
	fmt.Fprintf(w, "Region:          %s\n", displayOrDash(cp.Radio.Region))
	fmt.Fprintf(w, "Baseline digest: %s (truncated)\n", truncateDigest(cp.Radio.BaselineDigest))
	fmt.Fprintf(w, "Output:          %s\n", outPath)
}

// writeSettingsReadSummary writes cmdRead's --settings stdout summary
// (task-34 brief): how many settings items came back known, and how many
// were unavailable (the radio's own "?;" rejection at read time — a
// recorded fact, not a failure — see clone.ReadSettings' doc comment).
// snap.Entries only ever holds MenuKnown/MenuUnavailable states here — a
// fresh ReadSettings result never produces MenuUnsupported (that state
// exists solely for MergeMenuSnapshots' carry-forward job) — so these two
// counts always sum to len(snap.Entries).
func writeSettingsReadSummary(w io.Writer, snap *codeplug.MenuSnapshot) {
	known, unavailable := 0, 0
	for _, e := range snap.Entries {
		switch e.State {
		case codeplug.MenuKnown:
			known++
		case codeplug.MenuUnavailable:
			unavailable++
		}
	}
	fmt.Fprintf(w, "Settings read:        %d\n", known)
	fmt.Fprintf(w, "Settings unavailable: %d\n", unavailable)
}

// cmdRead implements "rigprog read" (task-12 brief §1): read every slot
// from a real or simulated radio and save it as a codeplug file.
func cmdRead(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("read", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // this function owns all usage/error output.
	port := fs.String("port", "", "real serial port device path")
	fake := fs.Bool("fake", false, "use the in-process simulated radio")
	out := fs.String("out", "", "output codeplug file path (required)")
	settings := fs.Bool("settings", false, "also read the radio's menu/EX settings surface (opt-in)")
	model := fs.String("model", wiring.DefaultModel, "radio model to target")
	force := fs.Bool("force", false, "overwrite --out if it already exists")
	snapshotDirFlag := fs.String("snapshot-dir", "", "snapshot/journal directory (default: <UserConfigDir>/rigprog/snapshots)")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printReadUsage(stdout)
			return exitSuccess
		}
		fmt.Fprintf(stderr, "rigprog read: %v\n", err)
		printReadUsage(stderr)
		return exitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "rigprog read: unexpected argument %q\n", fs.Arg(0))
		printReadUsage(stderr)
		return exitUsage
	}

	if !validateModel(stderr, "read", *model, printReadUsage) {
		return exitUsage
	}

	havePort := *port != ""
	if havePort == *fake { // both true, or both false
		fmt.Fprintln(stderr, "rigprog read: exactly one of --port or --fake is required")
		printReadUsage(stderr)
		return exitUsage
	}
	if *out == "" {
		fmt.Fprintln(stderr, "rigprog read: --out is required")
		printReadUsage(stderr)
		return exitUsage
	}

	// Refuse-overwrite is checked BEFORE anything radio-touching: never
	// overwrite --out as a side effect of a read that later fails, and
	// never spend a multi-second ReadAll only to refuse at the very end
	// (task-12 brief §1). checkOverwrite (fileio.go) is the one shared
	// place this Stat dance lives — task-13's export/import reuse it
	// verbatim.
	refused, err := checkOverwrite(*out, *force)
	if err != nil {
		fmt.Fprintf(stderr, "rigprog read: checking %s: %v\n", *out, err)
		return exitError
	}
	if refused {
		fmt.Fprintf(stderr, "rigprog read: %s already exists; use --force to overwrite\n", *out)
		return exitError
	}

	snapshotDir, err := resolveSnapshotDir(*snapshotDirFlag)
	if err != nil {
		fmt.Fprintf(stderr, "rigprog read: %v\n", err)
		return exitError
	}
	if err := os.MkdirAll(snapshotDir, 0o700); err != nil {
		fmt.Fprintf(stderr, "rigprog read: creating snapshot directory %s: %v\n", snapshotDir, err)
		return exitError
	}

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
			fmt.Fprintln(stderr, "rigprog read: cancelled")
			return exitError
		}
		fmt.Fprintf(stderr, "rigprog read: %v\n", err)
		return exitError
	}
	defer func() { _ = closeAll() }()

	// clone.NewService requires a SnapshotStore even though ReadAll
	// itself never touches it (task-12 brief §1: "the store is wired now
	// so the service construction is uniform" — Task 14's write path
	// uses it for real).
	svc := clone.NewService(sess, clone.SnapshotStore{Dir: snapshotDir}, clone.WithProgress(progressPrinter(stderr)))

	cp, err := svc.ReadAll(ctx)
	if err != nil {
		if isCancelled(err) {
			fmt.Fprintln(stderr, "rigprog read: cancelled")
			return exitError
		}
		fmt.Fprintf(stderr, "rigprog read: %v\n", err)
		return exitError
	}

	applyDefaultGenerator(cp)

	// task-34 brief: --settings is opt-in and runs AFTER the channel
	// ReadAll above — the default (no flag) path above is completely
	// unchanged: zero settings/EX wire traffic, cp.Menus stays nil. A
	// settings-read failure here — after the channel read already
	// succeeded — aborts the WHOLE command without ever calling
	// saveCodeplugNoClobber below: never write half the artefact the user
	// asked for (a file claiming --settings was honoured when it was not).
	var settingsSnapshot *codeplug.MenuSnapshot
	if *settings {
		settingsSnapshot, err = svc.ReadSettings(ctx)
		if err != nil {
			if isCancelled(err) {
				fmt.Fprintln(stderr, "rigprog read: cancelled")
				return exitError
			}
			fmt.Fprintf(stderr, "rigprog read: %v\n", err)
			return exitError
		}
		cp.Menus = codeplug.MergeMenuSnapshots(nil, settingsSnapshot)
	}

	// Fix 3 (adjudicated MEDIUM, Codex M4 #3): no-clobber enforced AT THE
	// COMMIT (saveCodeplugNoClobber), not just via the checkOverwrite Stat
	// above — read's own ReadAll can run for several seconds, exactly the
	// window a race against the checkOverwrite call above could exploit.
	if err := saveCodeplugNoClobber(*out, cp, *force); err != nil {
		if errors.Is(err, errDestExists) {
			fmt.Fprintf(stderr, "rigprog read: %s already exists; use --force to overwrite\n", *out)
			return exitError
		}
		fmt.Fprintf(stderr, "rigprog read: saving %s: %v\n", *out, err)
		return exitError
	}

	writeReadSummary(stdout, cp, *out)
	if *settings {
		writeSettingsReadSummary(stdout, settingsSnapshot)
	}
	return exitSuccess
}
