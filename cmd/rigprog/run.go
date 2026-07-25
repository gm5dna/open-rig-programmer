// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"io"
)

// run is rigprog's testable entry point: main is a thin wrapper that
// calls run with the real process args/stdin/stdout/stderr and exits
// with its return code (task-11 brief §2). Every subcommand dispatches
// from here, so black-box tests (the compiled binary) and in-process
// tests exercise identically the same logic.
//
// stdin flows through to write (its confirmation and firmware prompts —
// task 14); every other subcommand ignores it.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return exitUsage
	}

	cmd, rest := args[0], args[1:]

	switch {
	case cmd == "help" || cmd == "-h" || cmd == "--help":
		printUsage(stdout)
		return exitSuccess
	case cmd == "version" || cmd == "-v" || cmd == "--version":
		return cmdVersion(rest, stdout, stderr)
	case cmd == "ports":
		return cmdPorts(rest, stdout, stderr)
	case cmd == "probe":
		ctx, stop := newInterruptContext()
		defer stop()
		return cmdProbe(ctx, rest, stdout, stderr)
	case cmd == "read":
		ctx, stop := newInterruptContext()
		defer stop()
		return cmdRead(ctx, rest, stdout, stderr)
	case cmd == "write":
		ctx, stop := newInterruptContext()
		defer stop()
		return cmdWrite(ctx, rest, stdin, stdout, stderr)
	case cmd == "diff":
		ctx, stop := newInterruptContext()
		defer stop()
		return cmdDiff(ctx, rest, stdout, stderr)
	case cmd == "export":
		return cmdExport(rest, stdout, stderr)
	case cmd == "import":
		return cmdImport(rest, stdout, stderr)
	case cmd == "settings":
		return cmdSettings(rest, stdout, stderr)
	case notImplemented[cmd]:
		return cmdNotImplemented(cmd, stderr)
	default:
		fmt.Fprintf(stderr, "rigprog: unknown subcommand %q\n\n", cmd)
		printUsage(stderr)
		return exitUsage
	}
}
