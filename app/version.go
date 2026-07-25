// SPDX-License-Identifier: GPL-3.0-or-later

package main

import "github.com/gm5dna/open-rig-programmer/internal/buildinfo"

// VersionView is GetAppVersion's return value: which build of Open Rig
// Programmer the user is running.
//
// Display carries the fully-formed string the UI renders, rather than
// leaving the frontend to compose one from Version and IsRelease. That
// follows the project's standing "no facts assembled in JS" discipline
// (the Codex M6 finding that produced internal/radiotext): the wording
// that tells a user their build is not a release is a statement this
// program makes, and it is made in one place, here, where a test can
// pin it.
type VersionView struct {
	// Version is the bare version string — "v1.0.0" for a release build,
	// buildinfo.DevVersion ("dev") for anything the release pipeline did
	// not stamp. Machine-facing: use this, not Display, for comparisons.
	Version string
	// Display is what to show a human: Version for a release build, and
	// Version plus an explicit unreleased-build note otherwise.
	Display string
	// IsRelease is false for an unstamped build. Exposed so the UI can
	// decorate an unreleased build differently (a muted chip rather than
	// a plain one) without parsing Display.
	IsRelease bool
}

// GetAppVersion reports this build's version to the frontend. Bound
// method (Wails): the status bar shows it, so a user filing a bug report
// can read their version off the window instead of hunting through
// Finder's Get Info for the bundle version.
//
// Takes no session and touches no radio: it answers identically whether
// connected, disconnected or in demo mode, and cannot fail.
func (a *App) GetAppVersion() VersionView {
	v := buildinfo.Version()
	display := v
	if !buildinfo.IsRelease() {
		display = v + " (unreleased build)"
	}
	return VersionView{
		Version:   v,
		Display:   display,
		IsRelease: buildinfo.IsRelease(),
	}
}
