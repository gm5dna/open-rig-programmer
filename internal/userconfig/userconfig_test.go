// SPDX-License-Identifier: GPL-3.0-or-later

package userconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// settingsPath returns a path inside a fresh temp directory, one level
// below it, so every test also exercises SetUnverifiedWrites' parent
// directory creation.
func settingsPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "rigprog", "settings.json")
}

// readRawMap decodes the file at path as a generic JSON object. Tests use
// it to inspect what actually landed on disk rather than what Load chose
// to expose.
func readRawMap(t *testing.T, path string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("file at %s is not a JSON object: %v\n%s", path, err, b)
	}
	return m
}

// rawUnverifiedWrites returns the decoded "unverifiedWrites" object from
// the file at path, so a test can assert on the RECORDED bytes (an
// explicit false must be present, not merely absent).
func rawUnverifiedWrites(t *testing.T, path string) map[string]any {
	t.Helper()
	m := readRawMap(t, path)
	raw, ok := m["unverifiedWrites"]
	if !ok {
		t.Fatalf("file at %s has no \"unverifiedWrites\" key: %v", path, m)
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("file at %s: \"unverifiedWrites\" is %T, want object", path, raw)
	}
	return obj
}

// TestDefaultPath pins the location the CLI and the GUI must agree on:
// <os.UserConfigDir()>/rigprog/settings.json.
func TestDefaultPath(t *testing.T) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		t.Skipf("os.UserConfigDir unavailable in this environment: %v", err)
	}
	want := filepath.Join(cfgDir, "rigprog", "settings.json")

	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error: %v", err)
	}
	if got != want {
		t.Errorf("DefaultPath() = %q, want %q", got, want)
	}
}

// TestLoad_MissingFileIsEmptyNotAnError pins the first-run case: no file
// yet is not a fault, and the zero Settings reports every slug absent.
func TestLoad_MissingFileIsEmptyNotAnError(t *testing.T) {
	path := settingsPath(t)

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load of a missing file returned an error: %v", err)
	}
	granted, recorded := s.UnverifiedWritesFor("ftdx10")
	if granted || recorded {
		t.Errorf("UnverifiedWritesFor(%q) on a missing file = (%v, %v), want (false, false)", "ftdx10", granted, recorded)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("Load created something at %s (stat err = %v) — Load must never write", path, err)
	}
}

// TestUnverifiedWritesFor_AbsentSlug pins absent as distinguishable from
// declined: a slug nobody has been asked about reads (false, false) even
// when OTHER slugs are recorded.
func TestUnverifiedWritesFor_AbsentSlug(t *testing.T) {
	path := settingsPath(t)
	if err := SetUnverifiedWrites(path, "ftdx10", true); err != nil {
		t.Fatalf("SetUnverifiedWrites: %v", err)
	}

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	granted, recorded := s.UnverifiedWritesFor("ftdx101d")
	if granted || recorded {
		t.Errorf("UnverifiedWritesFor(%q) = (%v, %v), want (false, false) — an unasked model must not read as declined", "ftdx101d", granted, recorded)
	}
}

// TestSetUnverifiedWrites_GrantRoundTrips pins the granted case.
func TestSetUnverifiedWrites_GrantRoundTrips(t *testing.T) {
	path := settingsPath(t)

	if err := SetUnverifiedWrites(path, "ftdx10", true); err != nil {
		t.Fatalf("SetUnverifiedWrites: %v", err)
	}
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	granted, recorded := s.UnverifiedWritesFor("ftdx10")
	if !granted || !recorded {
		t.Errorf("UnverifiedWritesFor(%q) after a grant = (%v, %v), want (true, true)", "ftdx10", granted, recorded)
	}
	if got, ok := rawUnverifiedWrites(t, path)["ftdx10"]; !ok || got != true {
		t.Errorf("on disk, unverifiedWrites[%q] = %v (present %v), want true", "ftdx10", got, ok)
	}
}

// TestSetUnverifiedWrites_DeclineIsRecordedNotDeleted is the heart of the
// task: a decline is a DECISION. It must persist as an explicit false so
// the caller can tell "said no" from "never asked" and stop re-prompting.
func TestSetUnverifiedWrites_DeclineIsRecordedNotDeleted(t *testing.T) {
	path := settingsPath(t)

	if err := SetUnverifiedWrites(path, "ftdx10", false); err != nil {
		t.Fatalf("SetUnverifiedWrites: %v", err)
	}
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	granted, recorded := s.UnverifiedWritesFor("ftdx10")
	if granted {
		t.Errorf("UnverifiedWritesFor(%q) granted = true after a decline", "ftdx10")
	}
	if !recorded {
		t.Errorf("UnverifiedWritesFor(%q) recorded = false after a decline — the decline was not persisted", "ftdx10")
	}
	raw := rawUnverifiedWrites(t, path)
	got, ok := raw["ftdx10"]
	if !ok {
		t.Fatalf("on disk, unverifiedWrites has no %q key after a decline: %v — a stored false, not a deletion", "ftdx10", raw)
	}
	if got != false {
		t.Errorf("on disk, unverifiedWrites[%q] = %v, want false", "ftdx10", got)
	}
}

// TestSetUnverifiedWrites_RevocationIsRecorded pins the same rule reached
// from the other side: grant, then revoke. The key must survive as false,
// not vanish back to "never asked".
func TestSetUnverifiedWrites_RevocationIsRecorded(t *testing.T) {
	path := settingsPath(t)

	if err := SetUnverifiedWrites(path, "ftdx10", true); err != nil {
		t.Fatalf("SetUnverifiedWrites(grant): %v", err)
	}
	if err := SetUnverifiedWrites(path, "ftdx10", false); err != nil {
		t.Fatalf("SetUnverifiedWrites(revoke): %v", err)
	}

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	granted, recorded := s.UnverifiedWritesFor("ftdx10")
	if granted || !recorded {
		t.Errorf("UnverifiedWritesFor(%q) after grant-then-revoke = (%v, %v), want (false, true)", "ftdx10", granted, recorded)
	}
	if got, ok := rawUnverifiedWrites(t, path)["ftdx10"]; !ok || got != false {
		t.Errorf("on disk, unverifiedWrites[%q] = %v (present %v), want false", "ftdx10", got, ok)
	}
}

// TestSetUnverifiedWrites_RegrantAfterDecline pins that a recorded
// decline is not a dead end — the user can change their mind back.
func TestSetUnverifiedWrites_RegrantAfterDecline(t *testing.T) {
	path := settingsPath(t)

	for _, on := range []bool{false, true} {
		if err := SetUnverifiedWrites(path, "ftdx10", on); err != nil {
			t.Fatalf("SetUnverifiedWrites(%v): %v", on, err)
		}
	}

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	granted, recorded := s.UnverifiedWritesFor("ftdx10")
	if !granted || !recorded {
		t.Errorf("UnverifiedWritesFor(%q) after decline-then-grant = (%v, %v), want (true, true)", "ftdx10", granted, recorded)
	}
}

// TestSetUnverifiedWrites_SlugsAreIndependent pins per-model isolation:
// three models, three different states, none disturbing the others.
func TestSetUnverifiedWrites_SlugsAreIndependent(t *testing.T) {
	path := settingsPath(t)

	if err := SetUnverifiedWrites(path, "ftdx10", true); err != nil {
		t.Fatalf("SetUnverifiedWrites(ftdx10): %v", err)
	}
	if err := SetUnverifiedWrites(path, "ftdx101d", false); err != nil {
		t.Fatalf("SetUnverifiedWrites(ftdx101d): %v", err)
	}

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, tc := range []struct {
		slug                      string
		wantGranted, wantRecorded bool
	}{
		{"ftdx10", true, true},
		{"ftdx101d", false, true},
		{"ft710", false, false},
	} {
		granted, recorded := s.UnverifiedWritesFor(tc.slug)
		if granted != tc.wantGranted || recorded != tc.wantRecorded {
			t.Errorf("UnverifiedWritesFor(%q) = (%v, %v), want (%v, %v)", tc.slug, granted, recorded, tc.wantGranted, tc.wantRecorded)
		}
	}
}

// TestSetUnverifiedWrites_PreservesUnknownKeys is the forward-compatibility
// pin: a settings file written by a NEWER build carries keys this build has
// never heard of, and a consent decision taken here must not destroy them.
// Hence the raw-map merge rather than a Settings round-trip.
//
// The fixture deliberately includes numeric literals that a
// map[string]any merge — the OTHER natural implementation, and the one
// the decoded-value assertions below cannot tell apart from the
// json.RawMessage merge — would silently mangle, because decoding to
// float64 and re-encoding is lossy:
//
//	12345678901234567890123 → 1.2345678901234568e+22 (precision gone)
//	1.0                     → 1                      (a float becomes an int)
//
// Those two are asserted on the file's BYTES, not on decoded values, so
// the RawMessage choice is load-bearing rather than incidental. Nothing
// here parses those numbers: their whole point is that this build must
// carry them through without understanding them.
func TestSetUnverifiedWrites_PreservesUnknownKeys(t *testing.T) {
	path := settingsPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	original := []byte(`{
  "unverifiedWrites": {"ftdx10": true},
  "theme": "dark",
  "windowGeometry": {"w": 1280, "h": 800},
  "recentFiles": ["a.json", "b.json"],
  "bignum": 12345678901234567890123,
  "float": 1.0
}`)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := SetUnverifiedWrites(path, "ftdx101d", false); err != nil {
		t.Fatalf("SetUnverifiedWrites: %v", err)
	}

	got := readRawMap(t, path)
	for key, want := range map[string]any{
		"theme":          "dark",
		"windowGeometry": map[string]any{"w": float64(1280), "h": float64(800)},
		"recentFiles":    []any{"a.json", "b.json"},
	} {
		if !reflect.DeepEqual(got[key], want) {
			t.Errorf("unknown key %q after a Set = %#v, want %#v — a newer build's settings were destroyed", key, got[key], want)
		}
	}

	// The byte-level half: the literals must survive VERBATIM. A
	// map[string]any merge passes every assertion above and fails these.
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	for _, literal := range []string{
		`"bignum": 12345678901234567890123`,
		`"float": 1.0`,
	} {
		if !strings.Contains(string(b), literal) {
			t.Errorf("the file no longer contains the literal %s after a Set — an unknown value was decoded and re-encoded lossily rather than carried through verbatim.\nfile:\n%s", literal, b)
		}
	}

	// ...and the consent map itself merged rather than replaced.
	raw := rawUnverifiedWrites(t, path)
	if raw["ftdx10"] != true {
		t.Errorf("on disk, unverifiedWrites[%q] = %v, want true (the pre-existing entry)", "ftdx10", raw["ftdx10"])
	}
	if v, ok := raw["ftdx101d"]; !ok || v != false {
		t.Errorf("on disk, unverifiedWrites[%q] = %v (present %v), want false (the new entry)", "ftdx101d", v, ok)
	}
}

// corruptFiles are the shapes that must NEVER be silently reset: a
// truncated file, a non-object top level, and a well-formed object whose
// unverifiedWrites value — or one entry of it — is the wrong type.
//
// The two NULL rows are the ones encoding/json would otherwise wave
// through, and each is a silent LIE about a user's decisions rather than a
// parse failure (final review, Codex MEDIUM). Decoding into
// map[string]bool, `"unverifiedWrites": null` yields a nil map — read back
// as "no decisions recorded at all", so every radio is asked again — and a
// null ENTRY is a no-op into a bool, leaving the zero value, so a grant
// that was corrupted to null reads back as a recorded DECLINE. Neither
// shape can be written by this package; both are corruption, and this
// package's contract for corruption is to refuse and say so.
var corruptFiles = map[string]string{
	"truncated":              `{"unverifiedWrites": {"ftdx10": tru`,
	"not an object":          `["ftdx10"]`,
	"wrong type for the map": `{"unverifiedWrites": "yes please"}`,
	"wrong type for a value": `{"unverifiedWrites": {"ftdx10": "yes"}}`,
	"null map":               `{"unverifiedWrites": null}`,
	"null value":             `{"unverifiedWrites": {"ftdx10": null}}`,
	"empty file":             ``,
}

// TestLoad_CorruptFileErrors pins that a corrupt file is reported, with
// the path and hand-repair guidance, and never overwritten by Load.
func TestLoad_CorruptFileErrors(t *testing.T) {
	for name, body := range corruptFiles {
		t.Run(name, func(t *testing.T) {
			path := settingsPath(t)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatalf("MkdirAll: %v", err)
			}
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}

			_, err := Load(path)
			if err == nil {
				t.Fatalf("Load of a corrupt file returned nil error — a silent reset is exactly what must not happen")
			}
			assertCorruptErrorText(t, err, path)
			assertBytesUntouched(t, path, body)
		})
	}
}

// TestSetUnverifiedWrites_CorruptFileErrorsWithoutClobbering pins the
// same for the write path: Set cannot repair the file, so it must refuse
// and leave the user's bytes exactly as they were.
func TestSetUnverifiedWrites_CorruptFileErrorsWithoutClobbering(t *testing.T) {
	for name, body := range corruptFiles {
		t.Run(name, func(t *testing.T) {
			path := settingsPath(t)
			dir := filepath.Dir(path)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatalf("MkdirAll: %v", err)
			}
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}

			err := SetUnverifiedWrites(path, "ftdx10", true)
			if err == nil {
				t.Fatalf("SetUnverifiedWrites over a corrupt file returned nil error — it must refuse rather than clobber")
			}
			assertCorruptErrorText(t, err, path)
			assertBytesUntouched(t, path, body)
			assertNoStrayFiles(t, dir, filepath.Base(path))
		})
	}
}

func assertCorruptErrorText(t *testing.T, err error, path string) {
	t.Helper()
	msg := err.Error()
	if !strings.Contains(msg, path) {
		t.Errorf("error text does not name the file: %q (want it to contain %q)", msg, path)
	}
	if !strings.Contains(msg, "delete or repair the file by hand") {
		t.Errorf("error text lacks the hand-repair guidance: %q", msg)
	}
}

func assertBytesUntouched(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s back: %v", path, err)
	}
	if string(got) != want {
		t.Errorf("the original file was modified.\n got: %q\nwant: %q", got, want)
	}
}

// assertNoStrayFiles pins that the atomic-replace dance leaves no
// temporary file behind, on the success path or the refusal path.
func assertNoStrayFiles(t *testing.T, dir string, want ...string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading dir %s: %v", dir, err)
	}
	var got []string
	for _, e := range entries {
		got = append(got, e.Name())
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("directory %s contains %v, want exactly %v — a temp file was left behind", dir, got, want)
	}
}

// TestSetUnverifiedWrites_CreatesParentAndLeavesNoTempFiles pins the
// first-run mechanics: the parent directory is created on demand and the
// only thing left in it is the settings file itself.
func TestSetUnverifiedWrites_CreatesParentAndLeavesNoTempFiles(t *testing.T) {
	path := settingsPath(t)
	dir := filepath.Dir(path)

	for _, on := range []bool{true, false, true} {
		if err := SetUnverifiedWrites(path, "ftdx10", on); err != nil {
			t.Fatalf("SetUnverifiedWrites(%v): %v", on, err)
		}
		assertNoStrayFiles(t, dir, filepath.Base(path))
	}
}

// TestSetUnverifiedWrites_FileMode pins the owner-only mode: a consent
// record names the operator's radios, so 0600 rather than 0644.
func TestSetUnverifiedWrites_FileMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits are not meaningful on Windows")
	}
	path := settingsPath(t)
	if err := SetUnverifiedWrites(path, "ftdx10", true); err != nil {
		t.Fatalf("SetUnverifiedWrites: %v", err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if got := fi.Mode().Perm(); got != 0o600 {
		t.Errorf("settings file mode = %#o, want 0600", got)
	}
}

// TestSetUnverifiedWrites_OutputIsHandEditable pins that the file we ask
// a user to "repair by hand" is actually readable by hand: indented, with
// a trailing newline.
func TestSetUnverifiedWrites_OutputIsHandEditable(t *testing.T) {
	path := settingsPath(t)
	if err := SetUnverifiedWrites(path, "ftdx10", true); err != nil {
		t.Fatalf("SetUnverifiedWrites: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.HasSuffix(string(b), "\n") {
		t.Errorf("settings file does not end in a newline: %q", b)
	}
	if !strings.Contains(string(b), "\n  ") {
		t.Errorf("settings file is not indented: %q", b)
	}
}

// TestLoad_EmptyJSONObject pins the benign-but-not-missing case: a file
// containing "{}" is valid and simply records nothing.
func TestLoad_EmptyJSONObject(t *testing.T) {
	path := settingsPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load of {} returned an error: %v", err)
	}
	if granted, recorded := s.UnverifiedWritesFor("ftdx10"); granted || recorded {
		t.Errorf("UnverifiedWritesFor(%q) = (%v, %v), want (false, false)", "ftdx10", granted, recorded)
	}
}

// TestUnverifiedWritesFor_ZeroValueSettings pins that the zero Settings —
// what Load returns for a missing file, and what a caller may construct
// directly — is safe to query without a nil-map panic.
func TestUnverifiedWritesFor_ZeroValueSettings(t *testing.T) {
	var s Settings
	if granted, recorded := s.UnverifiedWritesFor("ftdx10"); granted || recorded {
		t.Errorf("zero Settings UnverifiedWritesFor(%q) = (%v, %v), want (false, false)", "ftdx10", granted, recorded)
	}
}

// TestSlugsAreOpaque pins the deliberate absence of any model knowledge in
// this package: whatever string the caller uses as a key round-trips, and
// nothing here validates it against internal/wiring's model names.
func TestSlugsAreOpaque(t *testing.T) {
	path := settingsPath(t)
	for _, slug := range []string{"", "not-a-real-radio", "FTdx10", "ft-710/x", "  spaces  "} {
		if err := SetUnverifiedWrites(path, slug, true); err != nil {
			t.Fatalf("SetUnverifiedWrites(%q): %v", slug, err)
		}
	}
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, slug := range []string{"", "not-a-real-radio", "FTdx10", "ft-710/x", "  spaces  "} {
		if granted, recorded := s.UnverifiedWritesFor(slug); !granted || !recorded {
			t.Errorf("UnverifiedWritesFor(%q) = (%v, %v), want (true, true) — slugs are opaque here", slug, granted, recorded)
		}
	}
}
