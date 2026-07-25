// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"io"

	"github.com/gm5dna/open-rig-programmer/core/clone"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
)

// formatProgressLine renders one clone.Progress callback invocation as a
// single human-readable line, e.g. "read 42/117 M-42\n" (task-12 brief
// §1's example). slot is rendered via codeplug.DisplaySlot — the sole
// place slot display mapping lives — so this line matches every other
// slot-facing report in this package.
func formatProgressLine(phase string, done, total int, slot string) string {
	return fmt.Sprintf("%s %d/%d %s\n", phase, done, total, codeplug.DisplaySlot(slot))
}

// progressPrinter returns a clone.Progress that writes formatProgressLine
// to w, one line per callback. Both cmdRead and cmdDiff pass this
// (wrapping stderr) to clone.WithProgress: progress is never part of a
// command's stdout result, only its stderr narration (task-12 brief §1's
// "progress rendered to stderr").
func progressPrinter(w io.Writer) clone.Progress {
	return func(phase string, done, total int, slot string) {
		fmt.Fprint(w, formatProgressLine(phase, done, total, slot))
	}
}
