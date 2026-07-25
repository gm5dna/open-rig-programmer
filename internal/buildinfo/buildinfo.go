// SPDX-License-Identifier: GPL-3.0-or-later

// Package buildinfo carries the release version stamped into a build.
//
// It exists because a released binary must be able to say which release
// it is: a bug report that says "it did X" is far less useful than one
// that says "v1.0.0 did X". Before this package the project had no
// build-time version stamping at all — cmd/rigprog's codeplug Generator
// string was the literal placeholder "rigprog/dev" with a comment saying
// so.
//
// The value is injected at link time by the release workflow:
//
//	go build -ldflags "-X github.com/gm5dna/open-rig-programmer/internal/buildinfo.version=v1.0.0"
//
// An ordinary `go build`, `go run` or `go test` leaves it unset, and
// Version() reports DevVersion. That fallback is deliberate: an
// unstamped build is a development build and should say so rather than
// claim a release it is not.
package buildinfo

import (
	"runtime"
	"strings"
)

// DevVersion is what Version() reports for a build with no stamped
// version — every `go build`, `go run` and `go test`, and any packaged
// build whose ldflags were dropped. Tests across the repo depend on this
// exact string, which is what makes an accidentally-unstamped release
// artefact visible rather than silent.
const DevVersion = "dev"

// version is set at link time (see the package doc comment). It is
// deliberately unexported: nothing in the program may assign to it, so
// the only way a build reports a release version is for the build itself
// to have stamped one.
var version string

// Version returns the stamped release version (e.g. "v1.0.0"), or
// DevVersion if this build was not stamped. Leading and trailing space
// is trimmed so a mis-quoted ldflags value cannot produce a version with
// invisible padding; a value that is entirely space is treated as unset.
func Version() string {
	if v := strings.TrimSpace(version); v != "" {
		return v
	}
	return DevVersion
}

// IsRelease reports whether this build carries a stamped version. It is
// the honest form of "is this a release build?" — useful anywhere the
// answer changes what a user should be told.
func IsRelease() bool { return Version() != DevVersion }

// Platform returns the GOOS/GOARCH this binary was built for, in the
// "darwin/arm64" form Go itself uses. Reported alongside the version
// because "which build" is half of "which version": a universal macOS
// binary runs as one arch or the other, and knowing which matters when
// diagnosing a serial-port problem.
func Platform() string { return runtime.GOOS + "/" + runtime.GOARCH }

// GoVersion returns the Go toolchain version this binary was built with.
func GoVersion() string { return runtime.Version() }
