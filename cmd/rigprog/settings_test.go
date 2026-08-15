// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/clone"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/csvio"
	"github.com/gm5dna/open-rig-programmer/internal/fakeradio"
	"github.com/gm5dna/open-rig-programmer/internal/userconfig"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// TestCmdSettings_MissingFileArgument / UnknownFlag / TrailingFlagForm
// pin the flags-first grammar (task-34 brief, export's own precedent):
// missing FILE, an unrecognised flag, and a flag placed AFTER FILE (which
// stdlib flag.Parse stops reading at) are all exit 2 — fast, no file
// ever touched.
func TestCmdSettings_MissingFileArgument(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdSettings([]string{"--csv", "out.csv"}, &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdSettings(no FILE) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
}

func TestCmdSettings_UnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdSettings([]string{"--bogus", "somefile.json"}, &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdSettings(--bogus) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
}

func TestCmdSettings_TrailingFlagForm(t *testing.T) {
	// "settings FILE --csv OUT": stdlib flag parsing stops at the first
	// positional (FILE), so --csv/OUT become two unexpected extra
	// arguments — exactly export's own pinned grammar.
	var stdout, stderr bytes.Buffer
	got := cmdSettings([]string{"somefile.json", "--csv", "out.csv"}, &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdSettings(FILE --csv OUT) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
}

// TestCmdSettings_UnknownModel pins task 40's --model validation for
// settings: an unrecognised model exits 2 (usage), naming FT-710, before
// FILE is even loaded.
func TestCmdSettings_UnknownModel(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdSettings([]string{"--model", unknownModelSentinel, "/nonexistent/rigprog-test.json"}, &stdout, &stderr)
	if got != exitUsage {
		t.Fatalf("cmdSettings(--model %s) = %d, want exitUsage (%d); stderr=%q", unknownModelSentinel, got, exitUsage, stderr.String())
	}
	if !strings.Contains(stderr.String(), unknownModelSentinel) || !strings.Contains(stderr.String(), "FT-710") {
		t.Errorf("cmdSettings(--model %s) stderr = %q, want it to name both the rejected and supported model", unknownModelSentinel, stderr.String())
	}
}

// TestCmdSettings_FakeExplicitModel_ByteIdentical pins task 40's headline
// requirement: "--model FT-710" (the default, spelled out explicitly) is
// byte-identical to no --model flag at all.
func TestCmdSettings_FakeExplicitModel_ByteIdentical(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "unrecognised.json")
	cp := &codeplug.Codeplug{
		Schema: codeplug.CurrentSchema,
		Menus: &codeplug.MenuSnapshot{
			Descriptor: "ft710-ex@1",
			Entries: []codeplug.MenuEntry{
				{ID: "010101", Value: "042", State: codeplug.MenuKnown},
			},
		},
	}
	if err := codeplug.Save(path, cp); err != nil {
		t.Fatalf("Save fixture: %v", err)
	}

	var wantStdout, wantStderr bytes.Buffer
	wantCode := cmdSettings([]string{path}, &wantStdout, &wantStderr)

	var gotStdout, gotStderr bytes.Buffer
	gotCode := cmdSettings([]string{"--model", "FT-710", path}, &gotStdout, &gotStderr)

	if gotCode != wantCode {
		t.Errorf("exit code = %d, want %d (flag-absent)", gotCode, wantCode)
	}
	if gotStdout.String() != wantStdout.String() {
		t.Errorf("stdout = %q, want byte-identical to flag-absent %q", gotStdout.String(), wantStdout.String())
	}
	if gotStderr.String() != wantStderr.String() {
		t.Errorf("stderr = %q, want byte-identical to flag-absent %q", gotStderr.String(), wantStderr.String())
	}
}

// TestCmdSettings_Help pins "rigprog settings -h".
func TestCmdSettings_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdSettings([]string{"-h"}, &stdout, &stderr)
	if got != exitSuccess {
		t.Errorf("cmdSettings([-h]) = %d, want exitSuccess (%d)", got, exitSuccess)
	}
	if stdout.Len() == 0 {
		t.Error("cmdSettings([-h]): stdout is empty, want usage text")
	}
}

// TestCmdSettings_LoadNonexistentFile pins a plain FILE-load failure,
// via the shared loadCodeplugStrict helper.
func TestCmdSettings_LoadNonexistentFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdSettings([]string{"/nonexistent/path/rigprog-test.json"}, &stdout, &stderr)
	if got != exitError {
		t.Errorf("cmdSettings(nonexistent FILE) = %d, want exitError (%d); stderr=%q", got, exitError, stderr.String())
	}
}

// TestCmdSettings_NoSnapshot pins the plain "no settings snapshot" error
// (task-34 brief): a codeplug.Codeplug with Menus == nil (exactly what a
// default "rigprog read", without --settings, produces) is refused, exit
// 1, stderr mentioning --settings.
func TestCmdSettings_NoSnapshot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "no-menus.json")
	cp := &codeplug.Codeplug{Schema: codeplug.CurrentSchema, Channels: []codeplug.Channel{{Slot: "001"}}}
	if err := codeplug.Save(path, cp); err != nil {
		t.Fatalf("Save fixture: %v", err)
	}

	var stdout, stderr bytes.Buffer
	got := cmdSettings([]string{path}, &stdout, &stderr)
	if got != exitError {
		t.Errorf("cmdSettings(no Menus) = %d, want exitError (%d); stderr=%q", got, exitError, stderr.String())
	}
	if !strings.Contains(stderr.String(), "no settings snapshot") {
		t.Errorf("cmdSettings(no Menus) stderr = %q, want it to mention \"no settings snapshot\"", stderr.String())
	}
	if !strings.Contains(stderr.String(), "--settings") {
		t.Errorf("cmdSettings(no Menus) stderr = %q, want it to mention --settings", stderr.String())
	}
}

// TestCmdSettings_LegacyOnlyNotice pins the distinct legacy-only error
// (task-34 brief): a migrated v1-opaque file (Menus non-nil, zero
// entries, Legacy present — exactly core/codeplug/file.go's loadV1
// migration shape) is refused, exit 1, with stderr distinguishing this
// from the plain no-snapshot case: it says preserved legacy menu data is
// present but not renderable, and still points at --settings.
func TestCmdSettings_LegacyOnlyNotice(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "legacy-only.json")
	cp := &codeplug.Codeplug{
		Schema: codeplug.CurrentSchema,
		Menus:  &codeplug.MenuSnapshot{Legacy: json.RawMessage(`{"anything":"preserved verbatim"}`)},
	}
	if err := codeplug.Save(path, cp); err != nil {
		t.Fatalf("Save fixture: %v", err)
	}

	var stdout, stderr bytes.Buffer
	got := cmdSettings([]string{path}, &stdout, &stderr)
	if got != exitError {
		t.Errorf("cmdSettings(legacy-only) = %d, want exitError (%d); stderr=%q", got, exitError, stderr.String())
	}
	stderrStr := stderr.String()
	if !strings.Contains(stderrStr, "preserved legacy menu data is present but not renderable") {
		t.Errorf("cmdSettings(legacy-only) stderr = %q, want it to say preserved legacy menu data is present but not renderable", stderrStr)
	}
	if !strings.Contains(stderrStr, "--settings") {
		t.Errorf("cmdSettings(legacy-only) stderr = %q, want it to suggest re-reading with --settings", stderrStr)
	}
	if strings.Contains(stderrStr, "no settings snapshot") {
		t.Errorf("cmdSettings(legacy-only) stderr = %q, want it NOT to use the plain no-snapshot wording (distinct case)", stderrStr)
	}
}

// TestCmdSettings_RendersUnsupportedEntries pins the "Unrecognised
// settings" section (task-34 brief): a hand-built file with one entry
// matching a real descriptor item (010101, "AF TREBLE GAIN" — see
// core/clone/settings_test.go's spotAddr) renders normally, grouped under
// its menu/group; a second entry whose ID ("999999") is not a Table 2
// member at all is listed separately.
func TestCmdSettings_RendersUnsupportedEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "unrecognised.json")
	cp := &codeplug.Codeplug{
		Schema: codeplug.CurrentSchema,
		Menus: &codeplug.MenuSnapshot{
			Descriptor: "ft710-ex@1",
			Entries: []codeplug.MenuEntry{
				{ID: "010101", Value: "042", State: codeplug.MenuKnown},
				{ID: "999999", Value: "7", State: codeplug.MenuUnsupported},
			},
		},
	}
	if err := codeplug.Save(path, cp); err != nil {
		t.Fatalf("Save fixture: %v", err)
	}

	var stdout, stderr bytes.Buffer
	got := cmdSettings([]string{path}, &stdout, &stderr)
	if got != exitSuccess {
		t.Fatalf("cmdSettings(unrecognised entry) = %d, want exitSuccess (%d); stderr=%q", got, exitSuccess, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"RADIO SETTING",  // menu label
		"MODE SSB",       // group label
		"AF TREBLE GAIN", // recognised item's label
		"042",            // recognised item's value
		"Unrecognised settings",
		"999999", // the unrecognised entry's raw ID
	} {
		if !strings.Contains(out, want) {
			t.Errorf("cmdSettings(unrecognised entry) stdout = %q, want it to contain %q", out, want)
		}
	}
}

// buildSettingsSnapshotFile saves a minimal codeplug.Codeplug carrying
// only the given menu entries, for tests that only care about
// cmdSettings' rendering/export logic, not a real radio read.
func buildSettingsSnapshotFile(t *testing.T, path string, entries []codeplug.MenuEntry) {
	t.Helper()
	cp := &codeplug.Codeplug{
		Schema: codeplug.CurrentSchema,
		Menus:  &codeplug.MenuSnapshot{Descriptor: "ft710-ex@1", Entries: entries},
	}
	if err := codeplug.Save(path, cp); err != nil {
		t.Fatalf("Save fixture: %v", err)
	}
}

// TestCmdSettings_CSVExport pins the CSV export shape (task-34 brief):
// exact header, one row per rendered entry (recognised + unrecognised),
// "Rows written"/"Output" summary lines.
func TestCmdSettings_CSVExport(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snap.json")
	buildSettingsSnapshotFile(t, path, []codeplug.MenuEntry{
		{ID: "010101", Value: "042", State: codeplug.MenuKnown},
		{ID: "010102", State: codeplug.MenuUnavailable},
		{ID: "999999", Value: "7", State: codeplug.MenuUnsupported},
	})
	csvOut := filepath.Join(dir, "snap.csv")

	var stdout, stderr bytes.Buffer
	got := cmdSettings([]string{"--csv", csvOut, path}, &stdout, &stderr)
	if got != exitSuccess {
		t.Fatalf("cmdSettings(--csv) = %d, want exitSuccess (%d); stderr=%q", got, exitSuccess, stderr.String())
	}
	if !strings.Contains(stdout.String(), "3") {
		t.Errorf("cmdSettings(--csv) stdout = %q, want it to mention the row count (3)", stdout.String())
	}

	f, err := os.Open(csvOut)
	if err != nil {
		t.Fatalf("opening exported CSV: %v", err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("parsing exported CSV: %v", err)
	}
	if len(rows) != 4 { // header + 3 entries
		t.Fatalf("CSV rows = %d, want 4 (header + 3 entries)", len(rows))
	}
	wantHeader := []string{"id", "menu", "group", "label", "state", "value"}
	for i, col := range wantHeader {
		if rows[0][i] != col {
			t.Errorf("CSV header[%d] = %q, want %q", i, rows[0][i], col)
		}
	}
}

// TestCmdSettings_CSVRefuseOverwrite pins the shared refuse-overwrite
// rule (checkOverwrite, fileio.go): an existing --csv without --force
// refuses before FILE is even loaded, exit 1, file left untouched.
func TestCmdSettings_CSVRefuseOverwrite(t *testing.T) {
	dir := t.TempDir()
	csvOut := filepath.Join(dir, "existing.csv")
	const sentinel = "not a csv file"
	if err := os.WriteFile(csvOut, []byte(sentinel), 0o600); err != nil {
		t.Fatalf("seeding existing --csv file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	got := cmdSettings([]string{"--csv", csvOut, "/nonexistent/rigprog-test.json"}, &stdout, &stderr)
	if got != exitError {
		t.Errorf("cmdSettings(existing --csv, no --force) = %d, want exitError (%d); stderr=%q", got, exitError, stderr.String())
	}
	contents, err := os.ReadFile(csvOut)
	if err != nil {
		t.Fatalf("reading %s after refused settings export: %v", csvOut, err)
	}
	if string(contents) != sentinel {
		t.Error("cmdSettings(existing --csv, no --force) overwrote the file as a side effect, want it untouched")
	}
}

// TestSettingsCSV_FormulaEscaping pins task-34's CSV formula-injection
// guard: dangerous prefixes ('=', '+', '-', '@') in the "value" and
// "label" columns are escaped (a leading apostrophe) in the emitted CSV
// — the same csvio.EscapeCell rule "rigprog export" already uses.
//
// The "value" case goes through the REAL wire pipeline end to end:
// fakeradio.WithEXSetting seeds a dangerous P4 payload at a known
// address, clone.Service.ReadSettings reads it for real (a settings-only
// read — no channel ReadAll, keeping this test's radio-invocation budget
// modest), and the resulting file is fed through cmdSettings --csv. No
// real FT-710 menu label is dangerous-prefixed (Table 2 has none), so the
// "label" case instead proves the SAME code path (writeSettingsCSV) by
// calling it directly with a hand-built row — there is no other way to
// get a dangerous label without inventing one.
//
// The session is opened via internal/wiring's FakeSessionOpts +
// OpenFakeSessionFor (task 40: this file no longer imports
// core/driver/ft710 directly) — the SAME production code path
// openFakeSession itself uses, just with fakeradio.WithEXSetting layered
// on top for this one call.
func TestSettingsCSV_FormulaEscaping(t *testing.T) {
	t.Run("value, via a real settings read", func(t *testing.T) {
		const addr = "010101" // AF TREBLE GAIN
		const dangerous = "=SUM(A1)"

		prevOpts := wiring.FakeSessionOpts
		wiring.FakeSessionOpts = []fakeradio.Option{fakeradio.WithEXSetting(addr, dangerous)}
		t.Cleanup(func() { wiring.FakeSessionOpts = prevOpts })

		sess, closeAll, err := wiring.OpenFakeSessionFor(testCtx(t), wiring.DefaultModel)
		if err != nil {
			t.Fatalf("OpenFakeSessionFor: %v", err)
		}
		t.Cleanup(func() { _ = closeAll() })

		svc := clone.NewService(sess, clone.SnapshotStore{})
		snap, err := svc.ReadSettings(testCtx(t))
		if err != nil {
			t.Fatalf("ReadSettings: %v", err)
		}

		dir := t.TempDir()
		path := filepath.Join(dir, "dangerous-value.json")
		cp := &codeplug.Codeplug{Schema: codeplug.CurrentSchema, Menus: codeplug.MergeMenuSnapshots(nil, snap)}
		if err := codeplug.Save(path, cp); err != nil {
			t.Fatalf("Save: %v", err)
		}
		csvOut := filepath.Join(dir, "dangerous-value.csv")

		var stdout, stderr bytes.Buffer
		got := cmdSettings([]string{"--csv", csvOut, path}, &stdout, &stderr)
		if got != exitSuccess {
			t.Fatalf("cmdSettings(--csv) = %d, want exitSuccess (%d); stderr=%q", got, exitSuccess, stderr.String())
		}

		f, err := os.Open(csvOut)
		if err != nil {
			t.Fatalf("opening exported CSV: %v", err)
		}
		defer f.Close()
		rows, err := csv.NewReader(f).ReadAll()
		if err != nil {
			t.Fatalf("parsing exported CSV: %v", err)
		}
		found := false
		for _, row := range rows[1:] {
			if row[0] == addr {
				found = true
				want := csvio.EscapeCell(dangerous)
				if row[5] != want { // value column
					t.Errorf("value column for %s = %q, want %q (escaped)", addr, row[5], want)
				}
			}
		}
		if !found {
			t.Fatalf("no CSV row found for address %s", addr)
		}
	})

	t.Run("label, via writeSettingsCSV directly", func(t *testing.T) {
		var buf bytes.Buffer
		rows := []settingsRow{{id: "010101", display: "01-01-01", menu: "RADIO SETTING", group: "MODE SSB", label: "@DANGEROUS LABEL", state: "known", value: "1"}}
		if err := writeSettingsCSV(&buf, rows, nil); err != nil {
			t.Fatalf("writeSettingsCSV: %v", err)
		}
		parsed, err := csv.NewReader(&buf).ReadAll()
		if err != nil {
			t.Fatalf("parsing CSV: %v", err)
		}
		want := csvio.EscapeCell("@DANGEROUS LABEL")
		if parsed[1][3] != want { // label column
			t.Errorf("label column = %q, want %q (escaped)", parsed[1][3], want)
		}
	})
}

// tempUserConfig points this package's userConfigPath seam at a file
// under a fresh t.TempDir(), restoring the previous value on cleanup,
// and returns that path. Every test below that can reach the consent
// store uses it: the store's production location is the REAL user's
// settings file, and a test that grants consent must never be able to
// write to the machine it runs on.
//
// The file itself is not created — an absent settings file is a valid,
// meaningful state (userconfig.Load returns the zero Settings for it),
// and it is the state a first-run user is in.
func tempUserConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "settings.json")
	prev := userConfigPath
	userConfigPath = func() (string, error) { return path, nil }
	t.Cleanup(func() { userConfigPath = prev })
	return path
}

// TestCmdSettings_UnverifiedWrites_List pins the listing: one row per
// wiring.SupportedModels() entry, consent-eligible models showing off/on,
// and the FT-710 — the one registered radio whose writes are
// hardware-verified — showing "n/a (hardware-verified)" rather than a
// state a user could be tempted to change.
func TestCmdSettings_UnverifiedWrites_List(t *testing.T) {
	tempUserConfig(t)

	var stdout, stderr bytes.Buffer
	got := cmdSettings([]string{"unverified-writes"}, &stdout, &stderr)
	if got != exitSuccess {
		t.Fatalf("settings unverified-writes = %d, want exitSuccess (%d); stderr=%q", got, exitSuccess, stderr.String())
	}
	out := stdout.String()
	for _, model := range wiring.SupportedModels() {
		row := consentRow(t, out, model)
		want := "off"
		if model == wiring.DefaultModel {
			want = "n/a (hardware-verified)"
		}
		if !strings.HasSuffix(row, want) {
			t.Errorf("listing row for %q = %q, want it to end %q", model, row, want)
		}
	}
}

// consentRow returns the one line of a "settings unverified-writes"
// listing whose first whitespace-separated field is model, with its
// trailing padding trimmed — so a test asserts on THAT model's state
// rather than on a substring that any row (or a hint line) could satisfy.
func consentRow(t *testing.T, listing, model string) string {
	t.Helper()
	for _, line := range strings.Split(listing, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == model {
			return strings.TrimRight(line, " \t")
		}
	}
	t.Fatalf("listing = %q, want a row for model %q", listing, model)
	return ""
}

// TestCmdSettings_UnverifiedWrites_GrantRevokeRoundTrip pins the whole
// set/revoke round trip, including the two things a user could not
// otherwise check: the confirmation copy is the honest one (it names what
// the grant actually means), and a REVOKED grant is stored as an explicit
// false rather than a deleted key — the recorded-decline semantics the
// store was built for (a user who has said no is not asked again).
func TestCmdSettings_UnverifiedWrites_GrantRevokeRoundTrip(t *testing.T) {
	path := tempUserConfig(t)

	var stdout, stderr bytes.Buffer
	got := cmdSettings([]string{"unverified-writes", "FTdx10", "on"}, &stdout, &stderr)
	if got != exitSuccess {
		t.Fatalf("settings unverified-writes FTdx10 on = %d, want exitSuccess (%d); stderr=%q", got, exitSuccess, stderr.String())
	}
	const grantCopy = "unverified writes for FTdx10 (ftdx10): on — this project has never written to a real FTdx10; every write is read back and compared"
	if !strings.Contains(stdout.String(), grantCopy) {
		t.Errorf("grant stdout = %q, want it to contain %q", stdout.String(), grantCopy)
	}

	if granted := rawConsent(t, path)["ftdx10"]; granted != true {
		t.Errorf("stored consent for \"ftdx10\" = %v, want true", granted)
	}

	stdout.Reset()
	stderr.Reset()
	if got := cmdSettings([]string{"unverified-writes"}, &stdout, &stderr); got != exitSuccess {
		t.Fatalf("settings unverified-writes (after grant) = %d, want exitSuccess (%d); stderr=%q", got, exitSuccess, stderr.String())
	}
	if row := consentRow(t, stdout.String(), "FTdx10"); !strings.HasSuffix(row, "on") {
		t.Errorf("listing after grant: FTdx10 row = %q, want it to end \"on\"", row)
	}

	stdout.Reset()
	stderr.Reset()
	if got := cmdSettings([]string{"unverified-writes", "FTdx10", "off"}, &stdout, &stderr); got != exitSuccess {
		t.Fatalf("settings unverified-writes FTdx10 off = %d, want exitSuccess (%d); stderr=%q", got, exitSuccess, stderr.String())
	}
	if !strings.Contains(stdout.String(), "unverified writes for FTdx10 (ftdx10): off") {
		t.Errorf("revoke stdout = %q, want it to state the new state plainly", stdout.String())
	}

	// A DECLINE IS A DECISION: the revoked entry is an explicit false in
	// the file, not an absent key (which would mean "never asked").
	stored := rawConsent(t, path)
	granted, recorded := stored["ftdx10"]
	if !recorded {
		t.Fatalf("stored consent = %v, want the revoked model still recorded (an explicit false, not a deleted key)", stored)
	}
	if granted {
		t.Errorf("stored consent for \"ftdx10\" = %v, want false", granted)
	}

	stdout.Reset()
	stderr.Reset()
	if got := cmdSettings([]string{"unverified-writes"}, &stdout, &stderr); got != exitSuccess {
		t.Fatalf("settings unverified-writes (after revoke) = %d, want exitSuccess (%d); stderr=%q", got, exitSuccess, stderr.String())
	}
	if row := consentRow(t, stdout.String(), "FTdx10"); !strings.HasSuffix(row, "off") {
		t.Errorf("listing after revoke: FTdx10 row = %q, want it to end \"off\"", row)
	}
}

// rawConsent decodes the settings file at path and returns its
// unverifiedWrites map, read as raw JSON rather than through
// internal/userconfig — this is a pin on what is actually ON DISK, so it
// must not be able to pass by agreeing with the same accessor the
// command wrote through.
func rawConsent(t *testing.T, path string) map[string]bool {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the settings file %s: %v", path, err)
	}
	var file struct {
		UnverifiedWrites map[string]bool `json:"unverifiedWrites"`
	}
	if err := json.Unmarshal(b, &file); err != nil {
		t.Fatalf("decoding the settings file %s (%q): %v", path, string(b), err)
	}
	return file.UnverifiedWrites
}

// TestCmdSettings_UnverifiedWrites_HardwareVerifiedRefused pins the
// refusal that keeps the command honest: the FT-710's writes are
// hardware-verified, so there is no unverified write for a consent to
// unlock, and BOTH "on" and "off" are refused as usage errors rather
// than recorded as a decision about nothing. The store must not even be
// created.
func TestCmdSettings_UnverifiedWrites_HardwareVerifiedRefused(t *testing.T) {
	for _, state := range []string{"on", "off"} {
		t.Run(state, func(t *testing.T) {
			path := tempUserConfig(t)

			var stdout, stderr bytes.Buffer
			got := cmdSettings([]string{"unverified-writes", "FT-710", state}, &stdout, &stderr)
			if got != exitUsage {
				t.Fatalf("settings unverified-writes FT-710 %s = %d, want exitUsage (%d); stderr=%q", state, got, exitUsage, stderr.String())
			}
			out := stderr.String()
			for _, want := range []string{"FT-710", "unverified write", "hardware-verified"} {
				if !strings.Contains(out, want) {
					t.Errorf("refusal stderr = %q, want it to contain %q", out, want)
				}
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Errorf("a refused grant created (or touched) the settings file %s, want it left absent", path)
			}
		})
	}
}

// TestCmdSettings_UnverifiedWrites_NearMissModelRefused regression-pins
// the near miss: "FTDX10" is the manual's spelling, NOT this project's
// registry key, and it must keep failing validateModel's ordinary
// unknown-model check — naming the models this build does support — the
// way it does everywhere else in this command. The one place that
// spelling appears in this repository's CLI tests is here, as the thing
// being refused.
func TestCmdSettings_UnverifiedWrites_NearMissModelRefused(t *testing.T) {
	path := tempUserConfig(t)

	var stdout, stderr bytes.Buffer
	got := cmdSettings([]string{"unverified-writes", "FTDX10", "on"}, &stdout, &stderr)
	if got != exitUsage {
		t.Fatalf("settings unverified-writes FTDX10 on = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
	out := stderr.String()
	if !strings.Contains(out, "FTDX10") || !strings.Contains(out, "FTdx10") {
		t.Errorf("refusal stderr = %q, want it to name both the rejected spelling and the supported one", out)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("a refused grant created (or touched) the settings file %s, want it left absent", path)
	}
}

// TestCmdSettings_UnverifiedWrites_BadStateWord pins the state word: it
// is "on" or "off" and nothing else — never a silently-ignored third
// value, and never a guess at which of the two a user meant.
func TestCmdSettings_UnverifiedWrites_BadStateWord(t *testing.T) {
	path := tempUserConfig(t)

	var stdout, stderr bytes.Buffer
	got := cmdSettings([]string{"unverified-writes", "FTdx10", "maybe"}, &stdout, &stderr)
	if got != exitUsage {
		t.Fatalf("settings unverified-writes FTdx10 maybe = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
	if !strings.Contains(stderr.String(), "on") || !strings.Contains(stderr.String(), "off") {
		t.Errorf("refusal stderr = %q, want it to name the two accepted state words", stderr.String())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("a refused set created (or touched) the settings file %s, want it left absent", path)
	}
}

// TestCmdSettings_UnverifiedWrites_BadArity pins the sub-mode's arity: 0
// further arguments lists, 2 set, and anything else is a usage error —
// in particular a bare model name, which could otherwise be read as
// either "show me this model" or "turn this model on".
func TestCmdSettings_UnverifiedWrites_BadArity(t *testing.T) {
	for _, args := range [][]string{
		{"unverified-writes", "FTdx10"},
		{"unverified-writes", "FTdx10", "on", "please"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			tempUserConfig(t)
			var stdout, stderr bytes.Buffer
			got := cmdSettings(args, &stdout, &stderr)
			if got != exitUsage {
				t.Errorf("cmdSettings(%v) = %d, want exitUsage (%d); stderr=%q", args, got, exitUsage, stderr.String())
			}
		})
	}
}

// TestCmdSettings_UnverifiedWrites_FlagsRefused pins that the sub-mode
// takes no flags: "settings --csv OUT unverified-writes" asks for a CSV
// export of a consent listing, which this command does not do, and it is
// refused rather than silently ignored.
func TestCmdSettings_UnverifiedWrites_FlagsRefused(t *testing.T) {
	tempUserConfig(t)
	var stdout, stderr bytes.Buffer
	got := cmdSettings([]string{"--csv", "out.csv", "unverified-writes"}, &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdSettings(--csv OUT unverified-writes) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
}

// corruptStore writes bytes that are not a settings file this build can
// read to path, and returns it.
func corruptStore(t *testing.T, path string) string {
	t.Helper()
	if err := os.WriteFile(path, []byte("{not json at all"), 0o600); err != nil {
		t.Fatalf("seeding a corrupt settings file at %s: %v", path, err)
	}
	return path
}

// TestCmdSettings_UnverifiedWrites_CorruptStore pins the carried Task-7
// review note: a settings file this build cannot read is surfaced with
// internal/userconfig's OWN error text — which names the path and tells
// the user to delete or repair it by hand — never a generic "could not
// read settings". Both the listing and a set are affected, and neither
// touches the file.
func TestCmdSettings_UnverifiedWrites_CorruptStore(t *testing.T) {
	for _, args := range [][]string{
		{"unverified-writes"},
		{"unverified-writes", "FTdx10", "on"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			path := corruptStore(t, tempUserConfig(t))
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading the seeded corrupt file: %v", err)
			}

			var stdout, stderr bytes.Buffer
			got := cmdSettings(args, &stdout, &stderr)
			if got == exitSuccess {
				t.Fatalf("cmdSettings(%v) with a corrupt store = exitSuccess, want a failure; stdout=%q", args, stdout.String())
			}
			out := stderr.String()
			for _, want := range []string{path, "repair the file by hand"} {
				if !strings.Contains(out, want) {
					t.Errorf("corrupt-store stderr = %q, want it to contain %q (userconfig's own wording, verbatim)", out, want)
				}
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading the corrupt file back: %v", err)
			}
			if string(after) != string(before) {
				t.Errorf("the corrupt settings file was modified: before=%q after=%q", before, after)
			}
		})
	}
}

// TestSessionOptionsFor pins the one function every real-hardware
// session in this command now goes through: what the store says becomes
// what wiring is asked for, and a store this build cannot read is an
// ERROR rather than either default — a corrupt file must never be
// silently read as "unconsented" (which would refuse writes the user had
// authorised) or as "consented" (which would authorise writes they had
// not).
func TestSessionOptionsFor(t *testing.T) {
	t.Run("absent store is zero options", func(t *testing.T) {
		tempUserConfig(t)
		opts, err := sessionOptionsFor(wiring.FTdx10Model)
		if err != nil {
			t.Fatalf("sessionOptionsFor: unexpected error: %v", err)
		}
		if (opts != wiring.SessionOptions{}) {
			t.Errorf("sessionOptionsFor(absent store) = %+v, want the zero SessionOptions", opts)
		}
	})

	t.Run("granted", func(t *testing.T) {
		path := tempUserConfig(t)
		if err := userconfig.SetUnverifiedWrites(path, "ftdx10", true); err != nil {
			t.Fatalf("seeding a grant: %v", err)
		}
		opts, err := sessionOptionsFor(wiring.FTdx10Model)
		if err != nil {
			t.Fatalf("sessionOptionsFor: unexpected error: %v", err)
		}
		if !opts.ConsentUnverifiedWrites {
			t.Error("sessionOptionsFor(granted).ConsentUnverifiedWrites = false, want true")
		}
	})

	t.Run("another model's grant does not carry", func(t *testing.T) {
		path := tempUserConfig(t)
		if err := userconfig.SetUnverifiedWrites(path, "ftdx10", true); err != nil {
			t.Fatalf("seeding a grant: %v", err)
		}
		opts, err := sessionOptionsFor(wiring.FTdx101DModel)
		if err != nil {
			t.Fatalf("sessionOptionsFor: unexpected error: %v", err)
		}
		if opts.ConsentUnverifiedWrites {
			t.Error("sessionOptionsFor(FTdx101D) = consented, want the FTdx10's grant to stay the FTdx10's")
		}
	})

	t.Run("revoked", func(t *testing.T) {
		path := tempUserConfig(t)
		if err := userconfig.SetUnverifiedWrites(path, "ftdx10", false); err != nil {
			t.Fatalf("seeding a decline: %v", err)
		}
		opts, err := sessionOptionsFor(wiring.FTdx10Model)
		if err != nil {
			t.Fatalf("sessionOptionsFor: unexpected error: %v", err)
		}
		if opts.ConsentUnverifiedWrites {
			t.Error("sessionOptionsFor(declined).ConsentUnverifiedWrites = true, want false")
		}
	})

	t.Run("corrupt store is an error", func(t *testing.T) {
		path := corruptStore(t, tempUserConfig(t))
		opts, err := sessionOptionsFor(wiring.FTdx10Model)
		if err == nil {
			t.Fatalf("sessionOptionsFor(corrupt store) = %+v, <nil error>, want an error", opts)
		}
		if !strings.Contains(err.Error(), path) {
			t.Errorf("sessionOptionsFor(corrupt store) error = %q, want userconfig's own text naming %q", err, path)
		}
		if opts.ConsentUnverifiedWrites {
			t.Error("sessionOptionsFor(corrupt store) returned consent alongside its error")
		}
	})
}
