// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"io"
)

// notImplemented lists subcommands the dispatch table accepts but does
// not yet implement (task-11 brief §1): a later task replaces each entry
// with a real implementation. Keeping this as a single set/map — rather
// than a switch arm per command — is what keeps run's dispatch table
// trivially extensible: adding a real implementation is deleting one
// entry here and adding one case in run, never restructuring dispatch
// itself. task-12 removed "read" and "diff"; task-13 removed "export"
// and "import"; task-14 removed "write" (write.go) — every dispatch-table
// entry now has a real implementation. Left as an empty map, ready for
// a future subcommand, rather than deleted outright.
var notImplemented = map[string]bool{}

// cmdNotImplemented reports that cmd is accepted by the dispatch table
// but not yet implemented: "not implemented" to stderr, exit 1 (the
// error class — this is not a usage mistake, the subcommand name is
// valid).
func cmdNotImplemented(cmd string, stderr io.Writer) int {
	fmt.Fprintf(stderr, "rigprog %s: not implemented\n", cmd)
	return exitError
}
