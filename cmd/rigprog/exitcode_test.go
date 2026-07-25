// SPDX-License-Identifier: GPL-3.0-or-later

package main

import "testing"

// TestExitCodes pins the exit-code table (task-11 brief §2) as a stable,
// script-facing contract: these numbers must never be renumbered or
// repurposed once released.
func TestExitCodes(t *testing.T) {
	cases := []struct {
		name string
		got  int
		want int
	}{
		{"success", exitSuccess, 0},
		{"error", exitError, 1},
		{"usage", exitUsage, 2},
		{"blocked", exitBlocked, 3},
		{"refused", exitRefused, 4},
		{"aborted", exitAborted, 5},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, c.got, c.want)
		}
	}
}
