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

	if ok, code := parseArgs(fs, args, "probe", printProbeUsage, stdout, stderr); !ok {
		return code
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "rigprog probe: unexpected argument %q\n", fs.Arg(0))
		printProbeUsage(stderr)
		return exitUsage
	}

	if !validateSessionArgs(stderr, "probe", *model, *port, *fake, printProbeUsage) {
		return exitUsage
	}

	sess, closeAll, err := openSession(ctx, *model, *port, *fake)
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
//
// When the driver could NAME the model it found (M9d-2, spec A5: the
// error carries a GotModel), the diagnostic names that radio instead of
// leaving the operator to decode a bare CAT ID; the wanted side is still
// the SELECTED model, never the error's own WantModel. Drivers that
// cannot name what they found leave GotModel empty and get the original
// ID-only wording, byte-for-byte.
func wrongRadioMessage(model string, wr *driver.WrongRadioError) string {
	if wr.GotModel != "" {
		return fmt.Sprintf("rigprog probe: wrong radio: radio identifies as %s (CAT ID %q); you selected %s — this port's radio does not identify as %s\n", wr.GotModel, wr.Got, model, model)
	}
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
// Region, the firmware answer and Diagnostics are read via the OPTIONAL
// driver.RegionReporter/driver.FirmwareAnswerReporter/
// driver.DiagnosticsReporter capabilities (core/driver/optional.go, task 37;
// the middle one added at Tier 6 task 18): sess's concrete type may implement
// none of them. Region's absence renders "Region:        -"; the firmware
// answer's absence and Diagnostics' absence OMIT their lines entirely, rather
// than a fabricated dash or zero — for the firmware answer that distinction
// is the whole point, since "this radio has no firmware query" and "the radio
// answered nothing" are different facts and only one of them is true of most
// registered models. Every FT-710 session implements Region and Diagnostics
// and not the firmware answer.
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

	// The radio's own firmware answer, VERBATIM and %q-quoted, for the
	// sessions whose concrete type has one to give (driver.
	// FirmwareAnswerReporter — every Kenwood row registered today, and
	// nothing else).
	// Absent capability, absent line: a fabricated "-" would read as "the
	// radio answered nothing" where the truth is "this radio has no such
	// question".
	//
	// QUOTED RATHER THAN BARE, and never compared against anything here.
	// This is the mitigation for the TS-590S's A13: that row refuses channel
	// writes on a firmware answer of 2.00 or later AND on one it cannot read
	// as a version at all, and in the second case the driver's own reading is
	// exactly what is in doubt — so the bytes have to reach the user
	// unedited, with any stray or non-printing character visible rather than
	// swallowed.
	if fr, ok := sess.(driver.FirmwareAnswerReporter); ok {
		fmt.Fprintf(stdout, "Firmware answer: %q\n", fr.FirmwareAnswer())
	}

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
