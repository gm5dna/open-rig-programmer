// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/internal/buildinfo"
)

// TestGetAppVersion_ReportsTheBuildStamp pins the mapping from
// buildinfo to the view. `go test` is never stamped, so this run
// exercises the unreleased-build branch — the branch a broken release
// artefact would also take.
func TestGetAppVersion_ReportsTheBuildStamp(t *testing.T) {
	a := &App{}
	got := a.GetAppVersion()

	if got.Version != buildinfo.Version() {
		t.Errorf("GetAppVersion().Version = %q, want %q", got.Version, buildinfo.Version())
	}
	if got.IsRelease != buildinfo.IsRelease() {
		t.Errorf("GetAppVersion().IsRelease = %v, want %v", got.IsRelease, buildinfo.IsRelease())
	}
	if got.IsRelease {
		t.Fatal("test binary reports IsRelease = true; these tests assume an unstamped build")
	}
	if !strings.Contains(got.Display, "unreleased build") {
		t.Errorf("GetAppVersion().Display = %q, want it to say \"unreleased build\" for an unstamped build", got.Display)
	}
	if !strings.HasPrefix(got.Display, got.Version) {
		t.Errorf("GetAppVersion().Display = %q, want it to start with the version %q", got.Display, got.Version)
	}
}

// TestGetAppVersion_NeedsNoSession is the property that lets the status
// bar render a version before the user has connected anything: the
// method reads no connection state, so it answers on a zero App.
func TestGetAppVersion_NeedsNoSession(t *testing.T) {
	zero := &App{}
	if got := zero.GetAppVersion(); got.Version == "" || got.Display == "" {
		t.Errorf("GetAppVersion() on a zero App = %+v, want both fields populated", got)
	}
}

// TestGetAppVersion_IsStable pins idempotence: the status bar fetches it
// once at startup and never refreshes, so a value that could change
// between calls would be a bug.
func TestGetAppVersion_IsStable(t *testing.T) {
	a := &App{}
	first, second := a.GetAppVersion(), a.GetAppVersion()
	if first != second {
		t.Errorf("GetAppVersion() returned %+v then %+v, want identical values", first, second)
	}
}
