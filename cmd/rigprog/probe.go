// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/internal/radiotext"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// cmdProbe implements "rigprog probe" (task-11 brief §5): open a session
// against either a real port (--port PATH) or the in-process simulated
// radio (--fake), then report the radio's identity and inventory. The ID
// probe itself happens inside Driver.Open; a wrong-radio answer is
// reported clearly and distinctly from other failures.
func cmdProbe(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("probe", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // this function owns all usage/error output.
	port := fs.String("port", "", "real serial port device path")
	fake := fs.Bool("fake", false, "use the in-process simulated radio")
	model := fs.String("model", wiring.DefaultModel, "radio model to target")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printProbeUsage(stdout)
			return exitSuccess
		}
		fmt.Fprintf(stderr, "rigprog probe: %v\n", err)
		printProbeUsage(stderr)
		return exitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "rigprog probe: unexpected argument %q\n", fs.Arg(0))
		printProbeUsage(stderr)
		return exitUsage
	}

	if !validateModel(stderr, "probe", *model, printProbeUsage) {
		return exitUsage
	}

	havePort := *port != ""
	if havePort == *fake { // both true, or both false
		fmt.Fprintln(stderr, "rigprog probe: exactly one of --port or --fake is required")
		printProbeUsage(stderr)
		return exitUsage
	}

	var (
		sess     driver.Session
		closeAll func() error
		err      error
	)
	if *fake {
		sess, closeAll, err = openFakeSession(ctx, *model)
	} else {
		sess, closeAll, err = openRealSession(ctx, *model, *port)
	}
	if err != nil {
		var wrongRadio *driver.WrongRadioError
		if errors.As(err, &wrongRadio) {
			fmt.Fprint(stderr, wrongRadioMessage(*model, wrongRadio))
			return exitError
		}
		fmt.Fprintf(stderr, "rigprog probe: %v\n", err)
		return exitError
	}
	defer func() { _ = closeAll() }()

	writeProbeReport(stdout, stderr, *model, sess)
	return exitSuccess
}

// wrongRadioMessage renders cmdProbe's wrong-radio diagnostic: an Open
// call's ID; probe answered with a different radio's CAT ID than model
// (the SELECTED --model, task 40 brief) expected — never a hardcoded
// "FT-710", since a caller who asked for a specific model should be told
// what they asked for, not what this build's only current driver happens
// to be.
func wrongRadioMessage(model string, wr *driver.WrongRadioError) string {
	return fmt.Sprintf("rigprog probe: wrong radio: got CAT ID %q, want %q — this port's radio does not identify as %s\n", wr.Got, wr.Want, model)
}

// writeProbeReport writes rigprog probe's human-readable report for an
// already-open, already-probed session to stdout — model, CAT ID, port,
// USB serial, region, and 60 m/EMG inventory summary — and a wire-health
// warning to stderr if the session's diagnostics report a nonzero
// UnexpectedFrames count (task-11 brief §5). Split out from cmdProbe so
// it can be exercised in-process against a hand-built session (e.g. one
// using ImageUS) without going through flag parsing or a wiring
// constructor that always picks the default fakeradio image.
//
// Region and Diagnostics are read via the OPTIONAL driver.RegionReporter/
// driver.DiagnosticsReporter capabilities (core/driver/optional.go, task
// 37): sess's concrete type may not implement either — a future second
// driver may implement neither. Region's absence renders "Region:        -";
// Diagnostics' absence omits the "Unexpected frames" line (and its
// stderr warning) entirely, rather than a fabricated zero. Every FT-710
// session implements both today, so this task changes nothing observable
// for FT-710.
func writeProbeReport(stdout, stderr io.Writer, model string, sess driver.Session) {
	id := sess.Identity()
	caps := sess.Capabilities()

	fmt.Fprintf(stdout, "Model:         %s\n", model)
	fmt.Fprintf(stdout, "CAT ID:        %s\n", id.CATID)
	fmt.Fprintf(stdout, "Port:          %s\n", id.Port)
	fmt.Fprintf(stdout, "USB serial:    %s\n", displayOrDash(id.USBSerial))

	region := "-"
	if rr, ok := sess.(driver.RegionReporter); ok {
		region = rr.Region()
	}
	fmt.Fprintf(stdout, "Region:        %s\n", region)

	count60m := 0
	if b, ok := caps.Bank(spec.Bank60m); ok {
		count60m = len(b.Slots)
	}
	_, hasEMG := caps.Bank(spec.BankEMG)
	fmt.Fprintf(stdout, "60 m channels: %d\n", count60m)
	fmt.Fprintf(stdout, "EMG channel:   %s\n", yesNo(hasEMG))

	if dr, ok := sess.(driver.DiagnosticsReporter); ok {
		diag := dr.Diagnostics()
		fmt.Fprintf(stdout, "Unexpected frames: %d\n", diag.UnexpectedFrames)
		if diag.UnexpectedFrames > 0 {
			fmt.Fprintf(stderr, "rigprog probe: warning: %d unexpected frame(s) seen on this session — check for wire contention or a misbehaving cable\n", diag.UnexpectedFrames)
		}
	}

	fmt.Fprintln(stdout)
	if text, ok := radiotext.For(model); ok {
		fmt.Fprintln(stdout, text.ProbeFirmwareNote)
	}
}

// displayOrDash returns s, or "-" if s is empty — for optional identity
// fields (e.g. USB serial on a non-USB port) that should never render as
// a blank line.
func displayOrDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// yesNo renders a bool as "yes"/"no" for human-readable report lines.
func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
