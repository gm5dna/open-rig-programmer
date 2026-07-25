// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// cmdPorts implements "rigprog ports" (task-11 brief §4): it calls
// transport.Discover() and prints a ranked table of candidate serial
// ports. It never accepts --fake — ports enumerates real OS-visible
// devices only; "rigprog probe --fake" is how the simulator is
// exercised.
func cmdPorts(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ports", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // this function owns all usage/error output.
	fake := fs.Bool("fake", false, "not accepted by ports; use \"rigprog probe --fake\" instead")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printPortsUsage(stdout)
			return exitSuccess
		}
		fmt.Fprintf(stderr, "rigprog ports: %v\n", err)
		printPortsUsage(stderr)
		return exitUsage
	}
	if *fake {
		fmt.Fprintln(stderr, "rigprog ports: --fake is not accepted (ports enumerates real serial devices only; use \"rigprog probe --fake\" to exercise the simulator)")
		return exitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "rigprog ports: unexpected argument %q\n", fs.Arg(0))
		printPortsUsage(stderr)
		return exitUsage
	}

	infos, err := transport.Discover()
	if err != nil {
		fmt.Fprintf(stderr, "rigprog ports: %v\n", err)
		return exitError
	}
	if len(infos) == 0 {
		fmt.Fprintln(stdout, "no serial ports found")
		return exitSuccess
	}

	tw := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "PATH\tSCORE\tDESCRIPTION\tVID:PID\tHINTS")
	for _, p := range infos {
		desc := p.Description
		if desc == "" {
			desc = "-"
		}
		vidpid := "-"
		if p.VID != "" || p.PID != "" {
			vidpid = p.VID + ":" + p.PID
		}
		hints := strings.Join(p.Hints, "; ")
		if hints == "" {
			hints = "-"
		}
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\n", p.Path, p.Score, desc, vidpid, hints)
	}
	_ = tw.Flush()

	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, `Ranking is a heuristic; run "rigprog probe --port <path>" for the definitive active ID check.`)
	return exitSuccess
}
