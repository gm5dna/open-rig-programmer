// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"io"

	"github.com/gm5dna/open-rig-programmer/internal/buildinfo"
)

// versionUsageText is "rigprog version"'s own usage text.
const versionUsageText = `rigprog version — report this build's version.

Usage:
  rigprog version

Prints the release version on the first line and the build's Go
toolchain and platform on the second, then exits 0. "rigprog -v" and
"rigprog --version" are the same command.

The first line is stable and script-readable: the second field is the
version, so "rigprog version | head -1 | cut -d' ' -f2" yields it. A
build that was not produced by the release pipeline reports "` +
	buildinfo.DevVersion + `" and says so in as many words — quote that line
verbatim in a bug report rather than guessing which release you have.
`

// printVersionUsage writes "rigprog version"'s usage text to w.
func printVersionUsage(w io.Writer) {
	fmt.Fprint(w, versionUsageText)
}

// cmdVersion implements "rigprog version" (and the -v/--version
// aliases). It takes no flags and touches no radio, no file and no
// network, so it cannot fail: the only non-zero exit it can produce is
// exitUsage for an argument it does not understand.
//
// The unreleased-build suffix is deliberate. A user reading "rigprog
// dev" off a binary they downloaded from a release page has found a
// broken artefact — the release workflow's ldflags did not reach the
// build — and the phrasing should make that obvious to them and to
// whoever reads their bug report, rather than looking like a version
// number in its own right.
func cmdVersion(args []string, stdout, stderr io.Writer) int {
	for _, arg := range args {
		switch arg {
		case "-h", "--help":
			printVersionUsage(stdout)
			return exitSuccess
		default:
			fmt.Fprintf(stderr, "rigprog version: unexpected argument %q\n\n", arg)
			printVersionUsage(stderr)
			return exitUsage
		}
	}

	suffix := ""
	if !buildinfo.IsRelease() {
		suffix = " (unreleased build)"
	}
	fmt.Fprintf(stdout, "rigprog %s%s\n", buildinfo.Version(), suffix)
	fmt.Fprintf(stdout, "%s %s\n", buildinfo.GoVersion(), buildinfo.Platform())
	return exitSuccess
}
