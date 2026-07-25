// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/internal/buildinfo"
)

// TestRun_VersionAndAliases covers every spelling that must work. A user
// reaching for -v or --version and getting "unknown subcommand" is the
// failure this pins.
func TestRun_VersionAndAliases(t *testing.T) {
	for _, arg := range []string{"version", "-v", "--version"} {
		var stdout, stderr bytes.Buffer
		code := run([]string{arg}, nil, &stdout, &stderr)

		if code != exitSuccess {
			t.Errorf("run(%q) = %d, want %d (stderr: %q)", arg, code, exitSuccess, stderr.String())
		}
		if stderr.Len() != 0 {
			t.Errorf("run(%q) wrote to stderr: %q, want nothing", arg, stderr.String())
		}
		if !strings.HasPrefix(stdout.String(), "rigprog ") {
			t.Errorf("run(%q) stdout = %q, want it to start with %q", arg, stdout.String(), "rigprog ")
		}
	}
}

// TestCmdVersion_FirstLineIsScriptReadable pins the documented contract
// in versionUsageText: field 2 of line 1 is the version, so
// "rigprog version | head -1 | cut -d' ' -f2" works.
func TestCmdVersion_FirstLineIsScriptReadable(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := cmdVersion(nil, &stdout, &stderr); code != exitSuccess {
		t.Fatalf("cmdVersion() = %d, want %d", code, exitSuccess)
	}

	lines := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("cmdVersion() printed %d lines (%q), want exactly 2", len(lines), stdout.String())
	}

	fields := strings.Fields(lines[0])
	if len(fields) < 2 {
		t.Fatalf("cmdVersion() line 1 = %q, want at least 2 space-separated fields", lines[0])
	}
	if fields[0] != "rigprog" {
		t.Errorf("cmdVersion() line 1 field 1 = %q, want %q", fields[0], "rigprog")
	}
	if fields[1] != buildinfo.Version() {
		t.Errorf("cmdVersion() line 1 field 2 = %q, want buildinfo.Version() = %q", fields[1], buildinfo.Version())
	}

	// Line 2 is the build environment: toolchain then GOOS/GOARCH.
	if !strings.Contains(lines[1], buildinfo.Platform()) {
		t.Errorf("cmdVersion() line 2 = %q, want it to contain the platform %q", lines[1], buildinfo.Platform())
	}
	if !strings.Contains(lines[1], buildinfo.GoVersion()) {
		t.Errorf("cmdVersion() line 2 = %q, want it to contain the Go version %q", lines[1], buildinfo.GoVersion())
	}
}

// TestCmdVersion_UnstampedBuildSaysSo is the one that catches a broken
// release artefact. `go test` is never stamped, so this run must carry
// the warning; a release build must not, and the same code path decides
// both.
func TestCmdVersion_UnstampedBuildSaysSo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmdVersion(nil, &stdout, &stderr)

	out := stdout.String()
	if buildinfo.IsRelease() {
		t.Fatal("test binary reports IsRelease() = true; these tests assume an unstamped build")
	}
	if !strings.Contains(out, "unreleased build") {
		t.Errorf("cmdVersion() on an unstamped build = %q, want it to say \"unreleased build\"", out)
	}
	if !strings.Contains(out, buildinfo.DevVersion) {
		t.Errorf("cmdVersion() on an unstamped build = %q, want it to report %q", out, buildinfo.DevVersion)
	}
}

// TestCmdVersion_HelpAndBadArgs pins the two argument paths.
func TestCmdVersion_HelpAndBadArgs(t *testing.T) {
	t.Run("help exits 0 to stdout", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		if code := cmdVersion([]string{"-h"}, &stdout, &stderr); code != exitSuccess {
			t.Errorf("cmdVersion(-h) = %d, want %d", code, exitSuccess)
		}
		if !strings.Contains(stdout.String(), "rigprog version") {
			t.Errorf("cmdVersion(-h) stdout = %q, want the usage text", stdout.String())
		}
		if stderr.Len() != 0 {
			t.Errorf("cmdVersion(-h) wrote to stderr: %q", stderr.String())
		}
	})

	t.Run("unexpected argument exits 2 to stderr", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		if code := cmdVersion([]string{"--json"}, &stdout, &stderr); code != exitUsage {
			t.Errorf("cmdVersion(--json) = %d, want %d", code, exitUsage)
		}
		if !strings.Contains(stderr.String(), "--json") {
			t.Errorf("cmdVersion(--json) stderr = %q, want it to name the offending argument", stderr.String())
		}
		if stdout.Len() != 0 {
			t.Errorf("cmdVersion(--json) wrote to stdout: %q, want nothing", stdout.String())
		}
	})
}

// TestUsage_ListsVersionCommand keeps the top-level command list honest:
// a command a user cannot discover from "rigprog help" may as well not
// exist.
func TestUsage_ListsVersionCommand(t *testing.T) {
	var buf bytes.Buffer
	printUsage(&buf)
	if !strings.Contains(buf.String(), "version") {
		t.Errorf("printUsage output = %q, want it to list the version command", buf.String())
	}
}

// TestCliGeneratorID_CarriesTheBuildVersion pins the link between the
// build stamp and what lands in a written file's Generator field. An
// imported codeplug records which rigprog wrote it; that is only useful
// if it tracks the actual build.
func TestCliGeneratorID_CarriesTheBuildVersion(t *testing.T) {
	want := "rigprog/" + buildinfo.Version()
	if cliGeneratorID != want {
		t.Errorf("cliGeneratorID = %q, want %q", cliGeneratorID, want)
	}
	if !strings.HasPrefix(cliGeneratorID, cliGeneratorPrefix) {
		t.Errorf("cliGeneratorID = %q, want the stable prefix %q", cliGeneratorID, cliGeneratorPrefix)
	}
}
