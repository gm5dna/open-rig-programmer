// SPDX-License-Identifier: GPL-3.0-or-later

package buildinfo

import (
	"runtime"
	"strings"
	"testing"
)

// TestVersion_UnstampedBuildReportsDev pins the fallback. `go test`
// never stamps a version, so this is the state every test run — and the
// state a packaged build must NOT be in. If a release artefact ever
// reports "dev", the ldflags were dropped.
func TestVersion_UnstampedBuildReportsDev(t *testing.T) {
	if got := Version(); got != DevVersion {
		t.Errorf("Version() in an unstamped test build = %q, want %q", got, DevVersion)
	}
	if IsRelease() {
		t.Error("IsRelease() = true in an unstamped test build, want false")
	}
}

// TestVersion_StampedValueIsReported exercises the link-time path
// without a linker: version is package-level state, so setting it here
// is exactly what -X does.
func TestVersion_StampedValueIsReported(t *testing.T) {
	restore := version
	t.Cleanup(func() { version = restore })

	version = "v1.0.0"
	if got := Version(); got != "v1.0.0" {
		t.Errorf("Version() = %q, want %q", got, "v1.0.0")
	}
	if !IsRelease() {
		t.Error("IsRelease() = false for a stamped build, want true")
	}
}

// TestVersion_WhitespaceOnlyStampIsTreatedAsUnset covers the mis-quoted
// ldflags case: `-X ...version= ` (or a value that shell-mangles to
// spaces) must not produce a build claiming to be release " ".
func TestVersion_WhitespaceOnlyStampIsTreatedAsUnset(t *testing.T) {
	restore := version
	t.Cleanup(func() { version = restore })

	for _, v := range []string{" ", "\t", "\n", "  \t\n "} {
		version = v
		if got := Version(); got != DevVersion {
			t.Errorf("Version() with version=%q = %q, want %q", v, got, DevVersion)
		}
		if IsRelease() {
			t.Errorf("IsRelease() = true with version=%q, want false", v)
		}
	}
}

// TestVersion_StampIsTrimmed keeps a padded value from reaching output,
// where it would render as "rigprog  v1.0.0 " in a Generator string or a
// status bar.
func TestVersion_StampIsTrimmed(t *testing.T) {
	restore := version
	t.Cleanup(func() { version = restore })

	version = "  v1.0.0\n"
	if got := Version(); got != "v1.0.0" {
		t.Errorf("Version() = %q, want the trimmed %q", got, "v1.0.0")
	}
}

// TestPlatform_MatchesRuntime pins the format callers render.
func TestPlatform_MatchesRuntime(t *testing.T) {
	want := runtime.GOOS + "/" + runtime.GOARCH
	if got := Platform(); got != want {
		t.Errorf("Platform() = %q, want %q", got, want)
	}
	if !strings.Contains(Platform(), "/") {
		t.Errorf("Platform() = %q, want a GOOS/GOARCH pair", Platform())
	}
}

// TestGoVersion_NonEmpty is a smoke check: runtime.Version() is never
// empty, and a caller printing it should never print a blank field.
func TestGoVersion_NonEmpty(t *testing.T) {
	if GoVersion() == "" {
		t.Error("GoVersion() = \"\", want the toolchain version")
	}
}
