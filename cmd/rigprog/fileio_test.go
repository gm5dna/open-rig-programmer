// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// TestCheckOverwrite_NoExistingFile pins checkOverwrite's baseline case:
// a path that does not exist yet is never refused, --force or not.
func TestCheckOverwrite_NoExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.json")

	for _, force := range []bool{false, true} {
		refused, err := checkOverwrite(path, force)
		if err != nil {
			t.Fatalf("checkOverwrite(nonexistent, force=%v): unexpected error: %v", force, err)
		}
		if refused {
			t.Errorf("checkOverwrite(nonexistent, force=%v) = refused, want not refused", force)
		}
	}
}

// TestCheckOverwrite_ExistingFile pins the refuse-overwrite rule shared by
// every subcommand that writes an output file (task-12 brief §1, reused
// verbatim by task-13's export/import): an existing path is refused
// unless force is true.
func TestCheckOverwrite_ExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.json")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatalf("seeding existing file: %v", err)
	}

	refused, err := checkOverwrite(path, false)
	if err != nil {
		t.Fatalf("checkOverwrite(existing, force=false): unexpected error: %v", err)
	}
	if !refused {
		t.Error("checkOverwrite(existing, force=false) = not refused, want refused")
	}

	refused, err = checkOverwrite(path, true)
	if err != nil {
		t.Fatalf("checkOverwrite(existing, force=true): unexpected error: %v", err)
	}
	if refused {
		t.Error("checkOverwrite(existing, force=true) = refused, want not refused")
	}
}

// TestLoadCodeplugStrict_Success pins the success path: no stderr output,
// the loaded Codeplug returned, exitSuccess.
func TestLoadCodeplugStrict_Success(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.json")
	fixture := &codeplug.Codeplug{Schema: codeplug.CurrentSchema}
	if err := codeplug.Save(path, fixture); err != nil {
		t.Fatalf("Save fixture: %v", err)
	}

	var stderr bytes.Buffer
	cp, code := loadCodeplugStrict(&stderr, "test", "", path)
	if cp == nil {
		t.Fatalf("loadCodeplugStrict: cp = nil, want a loaded Codeplug; stderr=%q", stderr.String())
	}
	if code != exitSuccess {
		t.Errorf("loadCodeplugStrict: code = %d, want exitSuccess (%d)", code, exitSuccess)
	}
	if stderr.Len() != 0 {
		t.Errorf("loadCodeplugStrict: stderr = %q, want empty on success", stderr.String())
	}
}

// TestLoadCodeplugStrict_NonexistentFile pins the plain load-failure path:
// nil Codeplug, exitError, a message naming cmdName and path.
func TestLoadCodeplugStrict_NonexistentFile(t *testing.T) {
	var stderr bytes.Buffer
	cp, code := loadCodeplugStrict(&stderr, "export", "", "/nonexistent/path/rigprog-test.json")
	if cp != nil {
		t.Error("loadCodeplugStrict(nonexistent): cp != nil, want nil")
	}
	if code != exitError {
		t.Errorf("loadCodeplugStrict(nonexistent): code = %d, want exitError (%d)", code, exitError)
	}
	if !strings.Contains(stderr.String(), "rigprog export:") {
		t.Errorf("loadCodeplugStrict(nonexistent) stderr = %q, want it to be prefixed with the given cmdName", stderr.String())
	}
}

// TestLoadCodeplugStrict_SchemaTooNew pins the distinct schema-too-new
// message (task-12 brief §2, shared here by export/import).
func TestLoadCodeplugStrict_SchemaTooNew(t *testing.T) {
	path := filepath.Join(t.TempDir(), "too-new.json")
	tooNew := &codeplug.Codeplug{Schema: codeplug.CurrentSchema + 1}
	if err := codeplug.Save(path, tooNew); err != nil {
		t.Fatalf("Save fixture: %v", err)
	}

	var stderr bytes.Buffer
	cp, code := loadCodeplugStrict(&stderr, "import", "--into", path)
	if cp != nil {
		t.Error("loadCodeplugStrict(schema too new): cp != nil, want nil")
	}
	if code != exitError {
		t.Errorf("loadCodeplugStrict(schema too new): code = %d, want exitError (%d)", code, exitError)
	}
	if !strings.Contains(stderr.String(), "newer") {
		t.Errorf("loadCodeplugStrict(schema too new) stderr = %q, want it to mention the file is newer than supported", stderr.String())
	}
	if !strings.Contains(stderr.String(), "--into") {
		t.Errorf("loadCodeplugStrict(schema too new) stderr = %q, want it to mention the --into label", stderr.String())
	}
}

// TestResolveSnapshotDir_Override pins task-12 brief §1: --snapshot-dir,
// when given, overrides the default outright. Moved here from
// read_test.go by task 14, since write.go now uses resolveSnapshotDir
// too (see fileio.go's doc comment on the function itself) — behaviour
// unchanged. Threaded with wiring.DefaultModel by task-7 (D9) so it
// keeps pinning the same override-passthrough property now that
// resolveSnapshotDir is model-keyed — see
// TestResolveSnapshotDir_OtherModelGetsSubdir for the non-default-model
// override case.
func TestResolveSnapshotDir_Override(t *testing.T) {
	got, err := resolveSnapshotDir("/tmp/some/override", wiring.DefaultModel)
	if err != nil {
		t.Fatalf("resolveSnapshotDir(override): unexpected error: %v", err)
	}
	if got != "/tmp/some/override" {
		t.Errorf("resolveSnapshotDir(override) = %q, want %q", got, "/tmp/some/override")
	}
}

// TestResolveSnapshotDir_Default pins the default: <UserConfigDir>/rigprog/snapshots.
// Threaded with wiring.DefaultModel by task-7 (D9): DefaultModel stays
// at this base directory unchanged, so every snapshot written before
// per-model subdirectories existed is still found.
func TestResolveSnapshotDir_Default(t *testing.T) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		t.Skipf("os.UserConfigDir unavailable in this environment: %v", err)
	}
	got, err := resolveSnapshotDir("", wiring.DefaultModel)
	if err != nil {
		t.Fatalf("resolveSnapshotDir(\"\"): unexpected error: %v", err)
	}
	want := filepath.Join(cfgDir, "rigprog", "snapshots")
	if got != want {
		t.Errorf("resolveSnapshotDir(\"\") = %q, want %q", got, want)
	}
}

// TestResolveSnapshotDir_DefaultModelStaysAtRoot mirrors
// internal/wiring's own test of the same name: pins task-7's (D9) most
// important property, that DefaultModel resolves to the base directory
// unchanged, byte-identical to the pre-task-7 behaviour, so every
// snapshot written before per-model subdirectories existed is still
// found.
func TestResolveSnapshotDir_DefaultModelStaysAtRoot(t *testing.T) {
	got, err := resolveSnapshotDir("/tmp/snaps", wiring.DefaultModel)
	if err != nil {
		t.Fatalf("resolveSnapshotDir: %v", err)
	}
	if got != "/tmp/snaps" {
		t.Errorf("resolveSnapshotDir(override, DefaultModel) = %q, want %q unchanged — existing FT-710 snapshots must keep working", got, "/tmp/snaps")
	}
}

// TestResolveSnapshotDir_OtherModelGetsSubdir mirrors internal/wiring's
// own test of the same name: pins task-7's (D9) collision-avoidance
// rule, that any model other than DefaultModel gets its own
// <base>/<model-slug>/ subdirectory — applied to an explicit override
// too, since two models sharing one named directory is exactly the
// collision this rule exists to prevent.
func TestResolveSnapshotDir_OtherModelGetsSubdir(t *testing.T) {
	got, err := resolveSnapshotDir("/tmp/snaps", "FTdx10")
	if err != nil {
		t.Fatalf("resolveSnapshotDir: %v", err)
	}
	if want := "/tmp/snaps/ftdx10"; got != want {
		t.Errorf("resolveSnapshotDir(override, %q) = %q, want %q", "FTdx10", got, want)
	}
}

// TestResolveSnapshotDir_EmptySlugIsError mirrors internal/wiring's own
// test of the same name: pins fix-round-1's finding, that a
// non-wiring.DefaultModel name which slugs to "" must not silently fall
// back to the base directory. filepath.Join drops empty elements, so
// filepath.Join(base, "") == base — without this guard such a model
// would collapse into exactly wiring.DefaultModel's own directory,
// precisely the collision this task exists to prevent, with no error
// raised.
func TestResolveSnapshotDir_EmptySlugIsError(t *testing.T) {
	for _, model := range []string{"", "---", "!!!", ".", ".."} {
		if got, err := resolveSnapshotDir("/tmp/snaps", model); err == nil {
			t.Errorf("resolveSnapshotDir(override, %q) = %q, <nil error>, want an error (empty slug must not silently collapse into DefaultModel's directory)", model, got)
		}
	}
}

// --- saveCodeplugNoClobber / openCSVCommit (Fix 3, adjudicated MEDIUM,
// Codex M4 #3): checkOverwrite is Stat-then-act — a TOCTOU race against
// whatever a long radio read spends its time on. These two commit
// helpers enforce no-clobber atomically at the moment of the actual
// filesystem commit, not merely at an earlier Stat. Both tests below
// simulate the race directly: create the destination AFTER the point
// where an early checkOverwrite call would already have passed.

// TestSaveCodeplugNoClobber_RefusesRaceWinner pins the codeplug-JSON
// commit path (read --out, import --out): force=false, and the
// destination is created (with known, distinguishable content) only
// just before the commit call — simulating another process winning a
// race checkOverwrite's earlier Stat could not have caught. The commit
// must refuse (errDestExists) and must NOT touch the existing file's
// bytes.
func TestSaveCodeplugNoClobber_RefusesRaceWinner(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")
	original := []byte("i-was-here-first")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatalf("seeding race-winner file: %v", err)
	}

	cp := &codeplug.Codeplug{Schema: codeplug.CurrentSchema}
	err := saveCodeplugNoClobber(path, cp, false)
	if !errors.Is(err, errDestExists) {
		t.Fatalf("saveCodeplugNoClobber(race winner, force=false) = %v, want errDestExists", err)
	}

	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("reading %s after refused commit: %v", path, readErr)
	}
	if string(got) != string(original) {
		t.Errorf("saveCodeplugNoClobber(race winner): file contents = %q, want untouched original %q", got, original)
	}

	// No stray private temp file left behind in dir.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", dir, err)
	}
	if len(entries) != 1 {
		t.Errorf("saveCodeplugNoClobber(race winner): dir has %d entries, want exactly 1 (out.json only, temp file cleaned up): %v", len(entries), entries)
	}
}

// TestSaveCodeplugNoClobber_NoExistingFile pins the ordinary success
// path: no destination present, force=false, the commit succeeds and
// the saved file loads back via codeplug.Load.
func TestSaveCodeplugNoClobber_NoExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")
	cp := &codeplug.Codeplug{Schema: codeplug.CurrentSchema}

	if err := saveCodeplugNoClobber(path, cp, false); err != nil {
		t.Fatalf("saveCodeplugNoClobber(no existing file): unexpected error: %v", err)
	}
	loaded, err := codeplug.Load(path)
	if err != nil {
		t.Fatalf("codeplug.Load(%s): %v", path, err)
	}
	if loaded.Schema != codeplug.CurrentSchema {
		t.Errorf("saveCodeplugNoClobber: loaded.Schema = %d, want %d", loaded.Schema, codeplug.CurrentSchema)
	}
}

// TestSaveCodeplugNoClobber_ForceOverwrites pins the consented-overwrite
// case: force=true commits over an existing file unconditionally, via
// codeplug.Save's own rename semantics directly (no temp-link dance
// needed once the caller has consented).
func TestSaveCodeplugNoClobber_ForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")
	if err := os.WriteFile(path, []byte("stale"), 0o600); err != nil {
		t.Fatalf("seeding existing file: %v", err)
	}

	cp := &codeplug.Codeplug{Schema: codeplug.CurrentSchema, Generator: "force-test"}
	if err := saveCodeplugNoClobber(path, cp, true); err != nil {
		t.Fatalf("saveCodeplugNoClobber(force=true): unexpected error: %v", err)
	}
	loaded, err := codeplug.Load(path)
	if err != nil {
		t.Fatalf("codeplug.Load(%s): %v", path, err)
	}
	if loaded.Generator != "force-test" {
		t.Errorf("saveCodeplugNoClobber(force=true): loaded.Generator = %q, want %q (the new content, overwritten)", loaded.Generator, "force-test")
	}
}

// TestOpenCSVCommit_RefusesRaceWinner mirrors
// TestSaveCodeplugNoClobber_RefusesRaceWinner for export --csv's commit
// path: O_EXCL refuses atomically when the destination already exists,
// and never touches its bytes.
func TestOpenCSVCommit_RefusesRaceWinner(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.csv")
	original := []byte("i-was-here-first")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatalf("seeding race-winner file: %v", err)
	}

	f, err := openCSVCommit(path, false)
	if !errors.Is(err, errDestExists) {
		t.Fatalf("openCSVCommit(race winner, force=false) = (%v, %v), want (nil, errDestExists)", f, err)
	}
	if f != nil {
		t.Errorf("openCSVCommit(race winner, force=false): f = %v, want nil", f)
	}

	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("reading %s after refused commit: %v", path, readErr)
	}
	if string(got) != string(original) {
		t.Errorf("openCSVCommit(race winner): file contents = %q, want untouched original %q", got, original)
	}
}

// TestOpenCSVCommit_NoExistingFile pins the ordinary success path.
func TestOpenCSVCommit_NoExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.csv")

	f, err := openCSVCommit(path, false)
	if err != nil {
		t.Fatalf("openCSVCommit(no existing file): unexpected error: %v", err)
	}
	defer f.Close()
	if _, err := f.WriteString("hello\n"); err != nil {
		t.Fatalf("writing via openCSVCommit's file: %v", err)
	}
}

// TestOpenCSVCommit_ForceTruncates pins force=true: an existing file's
// content is discarded (O_TRUNC), the consented-overwrite case.
func TestOpenCSVCommit_ForceTruncates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.csv")
	if err := os.WriteFile(path, []byte("this-should-be-gone-entirely"), 0o600); err != nil {
		t.Fatalf("seeding existing file: %v", err)
	}

	f, err := openCSVCommit(path, true)
	if err != nil {
		t.Fatalf("openCSVCommit(force=true): unexpected error: %v", err)
	}
	if _, err := f.WriteString("new\n"); err != nil {
		t.Fatalf("writing via openCSVCommit's file: %v", err)
	}
	f.Close()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if string(got) != "new\n" {
		t.Errorf("openCSVCommit(force=true): file contents = %q, want %q (old content truncated away)", got, "new\n")
	}
}
