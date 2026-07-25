// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import "testing"

// TestDiscover_RunsWithoutError exercises the real OS enumerator (not the
// pure rankPorts logic — see discover_test.go for that). It cannot assert
// anything about WHICH ports are present, since that depends entirely on
// what's plugged into the machine running the test; it only proves
// Discover's plumbing (enumerator call -> rankPorts -> return) doesn't
// error or panic on a machine with zero relevant devices attached, which is
// the expected state of any CI runner.
func TestDiscover_RunsWithoutError(t *testing.T) {
	got, err := Discover()
	if err != nil {
		t.Fatalf("Discover: unexpected error: %v", err)
	}
	if got == nil {
		t.Log("Discover returned a nil slice (zero ports found) — acceptable, len(nil) == 0")
	}
}
