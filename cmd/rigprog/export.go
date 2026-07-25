// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/gm5dna/open-rig-programmer/core/csvio"
)

// cmdExport implements "rigprog export" (task-13 brief §1): load a
// codeplug file and write it out as rigprog's own CSV schema. OFFLINE —
// unlike read/diff, no --port/--fake flags exist here at all: this
// function never opens a radio session, so there is nothing to accept or
// reject beyond what flag.Parse itself does with an unrecognised flag
// (exitUsage, same as any other unknown flag).
func cmdExport(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // this function owns all usage/error output.
	csvOut := fs.String("csv", "", "output CSV file path (required)")
	force := fs.Bool("force", false, "overwrite --csv if it already exists")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printExportUsage(stdout)
			return exitSuccess
		}
		fmt.Fprintf(stderr, "rigprog export: %v\n", err)
		printExportUsage(stderr)
		return exitUsage
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "rigprog export: exactly one FILE argument is required")
		printExportUsage(stderr)
		return exitUsage
	}
	file := fs.Arg(0)
	if *csvOut == "" {
		fmt.Fprintln(stderr, "rigprog export: --csv is required")
		printExportUsage(stderr)
		return exitUsage
	}

	// Refuse-overwrite is checked before FILE is even loaded — same rule,
	// same shared helper, as read --out (task-12 brief §1, fileio.go).
	refused, err := checkOverwrite(*csvOut, *force)
	if err != nil {
		fmt.Fprintf(stderr, "rigprog export: checking %s: %v\n", *csvOut, err)
		return exitError
	}
	if refused {
		fmt.Fprintf(stderr, "rigprog export: %s already exists; use --force to overwrite\n", *csvOut)
		return exitError
	}

	cp, code := loadCodeplugStrict(stderr, "export", "", file)
	if cp == nil {
		return code
	}

	// Fix 3 (adjudicated MEDIUM, Codex M4 #3): no-clobber enforced AT THE
	// COMMIT (openCSVCommit's O_EXCL when !force), not just via the
	// checkOverwrite Stat above — that earlier check is a fast-fail
	// optimisation only, and cannot close the race window a long radio
	// read could otherwise leave open (export itself is offline and
	// fast, but openCSVCommit is the one place this project's shared
	// commit discipline lives, so export uses it too).
	f, err := openCSVCommit(*csvOut, *force)
	if err != nil {
		if errors.Is(err, errDestExists) {
			fmt.Fprintf(stderr, "rigprog export: %s already exists; use --force to overwrite\n", *csvOut)
			return exitError
		}
		fmt.Fprintf(stderr, "rigprog export: creating %s: %v\n", *csvOut, err)
		return exitError
	}
	// Formula-injection escaping (a leading '=', '+', '-', '@' gets a
	// leading apostrophe unless it's a plain signed integer) lives
	// entirely inside csvio.Export — task-13 brief §1 is explicit this
	// command must not add a second layer of it.
	if err := csvio.Export(f, cp.Channels); err != nil {
		f.Close()
		fmt.Fprintf(stderr, "rigprog export: writing %s: %v\n", *csvOut, err)
		return exitError
	}
	if err := f.Close(); err != nil {
		fmt.Fprintf(stderr, "rigprog export: closing %s: %v\n", *csvOut, err)
		return exitError
	}

	// One row per slot, including empty slots — Export's own contract
	// (core/csvio/export.go), so "rows written" is simply the channel
	// count, not a "populated" count.
	fmt.Fprintf(stdout, "Rows written: %d\n", len(cp.Channels))
	fmt.Fprintf(stdout, "Output:       %s\n", *csvOut)
	return exitSuccess
}
