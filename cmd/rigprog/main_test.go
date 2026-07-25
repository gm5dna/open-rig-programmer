// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// rigprogBinPath is the path to the rigprog binary TestMain compiles
// once, shared by every black-box test in this package.
var rigprogBinPath string

// TestMain compiles cmd/rigprog exactly once, into a shared temporary
// directory, before running this package's tests (task-11 brief §6):
// the project's first CLI black-box harness — later tasks' read/write/
// diff/export/import black-box tests reuse runBinary/binResult
// unchanged.
func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "rigprog-bin-*")
	if err != nil {
		panic("cmd/rigprog: TestMain: MkdirTemp: " + err.Error())
	}

	binName := "rigprog"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	rigprogBinPath = filepath.Join(tmpDir, binName)

	build := exec.Command("go", "build", "-o", rigprogBinPath, ".")
	var buildOutput bytes.Buffer
	build.Stdout = &buildOutput
	build.Stderr = &buildOutput
	if err := build.Run(); err != nil {
		panic("cmd/rigprog: TestMain: building rigprog: " + err.Error() + "\n" + buildOutput.String())
	}

	code := m.Run()
	os.RemoveAll(tmpDir)
	os.Exit(code)
}

// binResult is one black-box invocation's captured outcome.
type binResult struct {
	stdout, stderr string
	exitCode       int
}

// runBinary execs the TestMain-built rigprog binary with args and stdin,
// returning its captured stdout, stderr, and exit code. An ordinary
// nonzero exit is NOT itself a test failure — asserting specific exit
// codes is exactly what callers of this helper do; only a failure to
// even start the process, or an abnormal exit (e.g. a signal), fails the
// test directly.
func runBinary(t *testing.T, stdin string, args ...string) binResult {
	t.Helper()
	return runBinaryStdin(t, strings.NewReader(stdin), args...)
}

// runBinaryStdin is runBinary's underlying implementation, taking an
// arbitrary io.Reader for stdin rather than a string — see
// TestBlackbox_Write_DevNullStdin_NonInteractive (blackbox_test.go,
// Fix 4, adjudicated MEDIUM, Codex M4 #4), which needs the child
// process's os.Stdin to be a REAL /dev/null (a genuine character
// device), not the pipe exec.Cmd sets up for a plain io.Reader — the
// whole point of that test is exercising isStdinTTY's actual
// ModeCharDevice quirk on a real character device, something
// strings.NewReader's pipe-backed stdin can never present as.
func runBinaryStdin(t *testing.T, stdin io.Reader, args ...string) binResult {
	t.Helper()
	cmd := exec.Command(rigprogBinPath, args...)
	cmd.Stdin = stdin
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	exitCode := 0
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("running rigprog %v: %v", args, err)
		}
	}
	return binResult{stdout: stdout.String(), stderr: stderr.String(), exitCode: exitCode}
}
