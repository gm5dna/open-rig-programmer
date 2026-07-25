// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestCmdPorts_FakeRejected pins task-11 brief §4: --fake is NOT accepted
// by ports.
func TestCmdPorts_FakeRejected(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdPorts([]string{"--fake"}, &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdPorts([--fake]) = %d, want exitUsage (%d)", got, exitUsage)
	}
	if !strings.Contains(stderr.String(), "--fake") {
		t.Errorf("stderr = %q, want it to mention --fake", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty on a usage error", stdout.String())
	}
}

// TestCmdPorts_UnexpectedArg pins the "contradictory/unexpected args ->
// exitUsage" half of the exit-code table for ports.
func TestCmdPorts_UnexpectedArg(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdPorts([]string{"bogus"}, &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdPorts([bogus]) = %d, want exitUsage (%d)", got, exitUsage)
	}
}

// TestCmdPorts_Help pins the "rigprog ports -h" half of task-11 brief §1:
// explicit help goes to stdout, exit 0.
func TestCmdPorts_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdPorts([]string{"-h"}, &stdout, &stderr)
	if got != exitSuccess {
		t.Errorf("cmdPorts([-h]) = %d, want exitSuccess (%d)", got, exitSuccess)
	}
	if stdout.Len() == 0 {
		t.Error("cmdPorts([-h]): stdout is empty, want usage text")
	}
	if stderr.Len() != 0 {
		t.Errorf("cmdPorts([-h]): stderr = %q, want empty for explicit help", stderr.String())
	}
}

// TestCmdPorts_RealDiscovery drives the actual transport.Discover() call
// against whatever this machine's OS reports. Environment-dependent (no
// FT-710 need be attached), so it asserts only the documented contract —
// exit 0, and either the ranked table's header or the friendly
// zero-candidates message — never specific devices (task-11 brief §6).
func TestCmdPorts_RealDiscovery(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdPorts(nil, &stdout, &stderr)
	if got != exitSuccess {
		t.Fatalf("cmdPorts(nil) = %d, want exitSuccess (%d); stderr=%q", got, exitSuccess, stderr.String())
	}
	out := stdout.String()
	hasHeader := strings.Contains(out, "PATH")
	hasNoCandidates := strings.Contains(out, "no serial ports found")
	if !hasHeader && !hasNoCandidates {
		t.Errorf("cmdPorts(nil): stdout = %q, want either the ranked table's header or the no-candidates message", out)
	}
}
