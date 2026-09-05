// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/internal/radiotext"
	"github.com/gm5dna/open-rig-programmer/internal/userconfig"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// tempUserConfig points this package's userConfigPath seam (consent.go)
// at a file under a fresh t.TempDir(), restoring the previous value on
// cleanup, and returns that path. EVERY test that can reach the consent
// store uses it — including the connect-path tests, since connect() now
// reads the store on its real branch: the production location is the REAL
// user's settings file, and no test may read, still less write, the
// settings of whoever runs "go test".
//
// The file itself is not created. An absent settings file is a valid,
// meaningful state (userconfig.Load returns the zero Settings for it) and
// it is the state a first-run user is in.
//
// Mirrors cmd/rigprog/settings_test.go's helper of the same name, over
// that command's own copy of the same seam.
func tempUserConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "settings.json")
	prev := userConfigPath
	userConfigPath = func() (string, error) { return path, nil }
	t.Cleanup(func() { userConfigPath = prev })
	return path
}

// eligibleModel is the consent-eligible model these tests grant and
// revoke against: a radio whose writes this project has never proved on
// hardware, so a recorded consent can actually open a write gate on it.
// wiring.DefaultModel (the FT-710) is its opposite and appears below as
// the refusal case — the one registered radio whose writes are
// hardware-verified.
const eligibleModel = wiring.FTdx10Model

// fixedCapsSession is a minimal driver.Session stub that reports one
// fixed capability set and nothing else. It exists because two things
// this task must prove are statements about CAPABILITIES alone — the
// amber flag GetUISpec derives from a live session's own labels, and the
// ConnectionInfo a consented connect returns — and app/ may not import a
// concrete driver package to obtain a real consented session (the
// M9a neutral-core discipline, pinned repo-wide by
// internal/guards/composition_imports_test.go). The consented capability
// set is instead built here by core/spec's OWN transform, which is the
// same one a driver applies.
//
// Every other Session method is unreachable from the paths under test and
// panics if ever called, so a wiring mistake in a test fails loudly rather
// than silently returning a zero value. Mirrors importexport_test.go's
// changingCapsSession in shape and intent.
type fixedCapsSession struct {
	caps spec.Capabilities
	id   driver.Identity
}

func (s fixedCapsSession) Capabilities() spec.Capabilities { return s.caps }

func (s fixedCapsSession) Identity() driver.Identity { return s.id }

func (s fixedCapsSession) ReadChannel(context.Context, string) (codeplug.Channel, error) {
	panic("fixedCapsSession: ReadChannel is not reachable from the consent paths")
}

func (s fixedCapsSession) WriteChannel(context.Context, codeplug.Channel) (driver.WriteResult, error) {
	panic("fixedCapsSession: WriteChannel is not reachable from the consent paths")
}

func (s fixedCapsSession) Close() error { return nil }

// realSessionRecorder records every call this package's
// openRealSessionWith seam (connection.go) receives, and answers each with
// a fixedCapsSession carrying the model's STATIC capabilities.
//
// It is the one place a test can observe what consent does to a real
// connect. Consent reaches a SESSION and nothing else: it transforms the
// capability set a real-hardware driver's Open assembles, so observing it
// means opening a real-profile session, which means a serial port —
// internal/wiring's own openSerial seam exists for exactly that but is
// package-private there. This package's call into
// wiring.OpenRealSessionWith is the nearest point it can name.
//
// What is proved here is only the part this package can prove: that the
// user's RECORDED DECISION is read and handed to wiring. That
// wiring spends it correctly, and that a consented session's capabilities
// really do carry spec.ConsentedUnverified, are internal/wiring's own
// proofs (TestOpenRealSessionWith_ConsentedSessionCaps) and are not
// restated here.
type realSessionRecorder struct {
	mu    sync.Mutex
	calls []realSessionCall
}

type realSessionCall struct {
	model    string
	portPath string
	opts     wiring.SessionOptions
}

// install points openRealSessionWith at r for the duration of the test.
func (r *realSessionRecorder) install(t *testing.T) {
	t.Helper()
	prev := openRealSessionWith
	openRealSessionWith = func(ctx context.Context, model, portPath string, opts wiring.SessionOptions) (driver.Session, func() error, error) {
		r.mu.Lock()
		r.calls = append(r.calls, realSessionCall{model: model, portPath: portPath, opts: opts})
		r.mu.Unlock()

		caps, err := capsForModel(model)
		if err != nil {
			return nil, nil, err
		}
		sess := fixedCapsSession{caps: caps, id: driver.Identity{CATID: caps.CATID, Port: portPath}}
		return sess, sess.Close, nil
	}
	t.Cleanup(func() { openRealSessionWith = prev })
}

// only returns the single recorded call, failing the test unless there was
// exactly one.
func (r *realSessionRecorder) only(t *testing.T) realSessionCall {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.calls) != 1 {
		t.Fatalf("openRealSessionWith calls = %d, want exactly 1: %+v", len(r.calls), r.calls)
	}
	return r.calls[0]
}

// containSnapshotDir points a test's snapshot directory at a temporary
// HOME: connect() creates that directory for real, under
// os.UserConfigDir(), and no test may write into the developer's own
// config directory. The consent store is NOT covered by this — it has its
// own seam (tempUserConfig), which every caller of this helper also uses.
//
// All four variables are set because os.UserConfigDir has THREE branches
// (Go 1.25 os/file.go): darwin reads HOME directly (appending
// "/Library/Application Support"), windows reads %AppData%, and every
// other unix reads XDG_CONFIG_HOME, falling back to HOME. So on this
// developer's own macOS, HOME is the line doing the work — it is NOT
// redundant with XDG_CONFIG_HOME here, and dropping it as "covered
// already" would silently restore the bug this helper exists to prevent.
// AppData is set because no amount of HOME setting redirects it on
// Windows, so without that line this test would create a real directory
// in the runner's own roaming profile. LocalAppData is set alongside it,
// belt-and-braces, so no per-user Windows location can leak outside the
// temporary HOME. Go looks environment variables up case-insensitively on
// Windows, so the mixed-case spellings that platform uses are the ones
// written here.
func containSnapshotDir(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("AppData", home)
	t.Setenv("LocalAppData", home)
}

// TestGetUnverifiedWriteConsent_TheThreeStates pins the distinction the
// whole store exists for (userconfig's "a decline is a decision"): never
// asked, asked and declined, and granted are THREE states, not two, and
// GetUnverifiedWriteConsent reports which one a model is in. A GUI that
// could not tell "never asked" from "declined" would either nag a user who
// has already said no at every session, or silently stop offering a
// decision they never took.
func TestGetUnverifiedWriteConsent_TheThreeStates(t *testing.T) {
	a, _ := newTestApp(t)
	path := tempUserConfig(t)

	got, err := a.GetUnverifiedWriteConsent(eligibleModel)
	if err != nil {
		t.Fatalf("GetUnverifiedWriteConsent(%q): unexpected error: %v", eligibleModel, err)
	}
	if got.Model != eligibleModel {
		t.Errorf("Model = %q, want %q", got.Model, eligibleModel)
	}
	if !got.NeedsConsent {
		t.Errorf("NeedsConsent = false, want true — %s's writes are not hardware-verified", eligibleModel)
	}
	if got.Granted || got.Recorded {
		t.Errorf("never asked: Granted = %v, Recorded = %v; want false, false", got.Granted, got.Recorded)
	}
	if got.Warning == "" {
		t.Error("Warning = \"\", want the arming dialogue's body — a consent-eligible model must carry the text the user is asked to accept")
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Errorf("settings file %s exists (err = %v) after a READ, want it still absent", path, statErr)
	}

	if err := a.SetUnverifiedWriteConsent(eligibleModel, false); err != nil {
		t.Fatalf("SetUnverifiedWriteConsent(%q, false): unexpected error: %v", eligibleModel, err)
	}
	got, err = a.GetUnverifiedWriteConsent(eligibleModel)
	if err != nil {
		t.Fatalf("GetUnverifiedWriteConsent after a decline: unexpected error: %v", err)
	}
	if got.Granted {
		t.Error("declined: Granted = true, want false")
	}
	if !got.Recorded {
		t.Error("declined: Recorded = false, want true — a decline is a decision, and it must survive so the user is not asked again")
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Errorf("settings file %s after a recorded decline: %v; want it written", path, statErr)
	}

	if err := a.SetUnverifiedWriteConsent(eligibleModel, true); err != nil {
		t.Fatalf("SetUnverifiedWriteConsent(%q, true): unexpected error: %v", eligibleModel, err)
	}
	got, err = a.GetUnverifiedWriteConsent(eligibleModel)
	if err != nil {
		t.Fatalf("GetUnverifiedWriteConsent after a grant: unexpected error: %v", err)
	}
	if !got.Granted || !got.Recorded {
		t.Errorf("granted: Granted = %v, Recorded = %v; want true, true", got.Granted, got.Recorded)
	}
	if !got.NeedsConsent {
		t.Error("granted: NeedsConsent = false, want true — NeedsConsent describes the RADIO, not the answer, so granting must not clear it")
	}
}

// TestGetUnverifiedWriteConsent_WarningNamesTheRadio pins that the
// Warning served to the frontend is internal/radiotext's shared template
// with THIS model's name substituted — not a sentence app/ writes for
// itself, and not another radio's wording.
func TestGetUnverifiedWriteConsent_WarningNamesTheRadio(t *testing.T) {
	a, _ := newTestApp(t)
	tempUserConfig(t)

	got, err := a.GetUnverifiedWriteConsent(eligibleModel)
	if err != nil {
		t.Fatalf("GetUnverifiedWriteConsent(%q): unexpected error: %v", eligibleModel, err)
	}
	want := fmt.Sprintf(radiotext.UnverifiedWriteWarningTemplate, eligibleModel)
	if got.Warning != want {
		t.Errorf("Warning =\n%q\nwant\n%q", got.Warning, want)
	}
	if !strings.Contains(got.Warning, eligibleModel) {
		t.Errorf("Warning = %q, want it to name %s", got.Warning, eligibleModel)
	}
}

// TestUnverifiedWriteConsent_HardwareVerifiedModelRefused pins the
// eligibility gate on BOTH bound methods, using the same shared predicate
// the CLI uses (wiring.NeedsUnverifiedConsent): the FT-710's writes are
// hardware-verified, so it has nothing to consent to. Get reports that
// plainly (NeedsConsent false, no warning); Set refuses outright, because
// recording a decision about nothing would be a lie the user could later
// act on.
//
// The refusal happens BEFORE the store is touched, which the absent
// settings file at the end is the evidence for: a refused grant must not
// create, alter or even read the user's settings.
func TestUnverifiedWriteConsent_HardwareVerifiedModelRefused(t *testing.T) {
	a, _ := newTestApp(t)
	path := tempUserConfig(t)

	got, err := a.GetUnverifiedWriteConsent(wiring.DefaultModel)
	if err != nil {
		t.Fatalf("GetUnverifiedWriteConsent(%q): unexpected error: %v", wiring.DefaultModel, err)
	}
	if got.NeedsConsent {
		t.Errorf("NeedsConsent = true for %s, want false — its writes are hardware-verified", wiring.DefaultModel)
	}
	if got.Granted || got.Recorded {
		t.Errorf("%s: Granted = %v, Recorded = %v; want false, false — there is no decision to hold", wiring.DefaultModel, got.Granted, got.Recorded)
	}
	if got.Warning != "" {
		t.Errorf("Warning = %q for %s, want \"\" — there is no unverified write here to warn about", got.Warning, wiring.DefaultModel)
	}

	for _, on := range []bool{true, false} {
		err := a.SetUnverifiedWriteConsent(wiring.DefaultModel, on)
		if err == nil {
			t.Fatalf("SetUnverifiedWriteConsent(%q, %v): err = nil, want a refusal", wiring.DefaultModel, on)
		}
		for _, want := range []string{wiring.DefaultModel, "hardware-verified"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("SetUnverifiedWriteConsent(%q, %v): err = %q, want it to mention %q", wiring.DefaultModel, on, err, want)
			}
		}
	}

	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Errorf("settings file %s exists (err = %v), want it never created — every refusal above precedes the store", path, statErr)
	}
}

// TestUnverifiedWriteConsent_UnknownModelRefused pins that both bound
// methods validate the model through the same shared predicate, and so
// report an unrecognised one with internal/wiring's own typed error, whose
// text names every model the caller could have asked for.
func TestUnverifiedWriteConsent_UnknownModelRefused(t *testing.T) {
	const bogus = "NoSuchRadioModel"
	a, _ := newTestApp(t)
	path := tempUserConfig(t)

	if _, err := a.GetUnverifiedWriteConsent(bogus); err == nil {
		t.Errorf("GetUnverifiedWriteConsent(%q): err = nil, want a *wiring.UnknownModelError", bogus)
	} else {
		assertUnknownModelNamed(t, "GetUnverifiedWriteConsent", bogus, err)
	}
	if err := a.SetUnverifiedWriteConsent(bogus, true); err == nil {
		t.Errorf("SetUnverifiedWriteConsent(%q, true): err = nil, want a *wiring.UnknownModelError", bogus)
	} else {
		assertUnknownModelNamed(t, "SetUnverifiedWriteConsent", bogus, err)
	}

	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Errorf("settings file %s exists (err = %v), want it never created — an unnameable radio must not reach the store", path, statErr)
	}
}

// assertUnknownModelNamed fails unless err is a *wiring.UnknownModelError
// naming model.
func assertUnknownModelNamed(t *testing.T, call, model string, err error) {
	t.Helper()
	var unknown *wiring.UnknownModelError
	if !errors.As(err, &unknown) {
		t.Fatalf("%s(%q): err = %v, want a *wiring.UnknownModelError", call, model, err)
	}
	if unknown.Model != model {
		t.Errorf("%s(%q): UnknownModelError.Model = %q, want %q", call, model, unknown.Model, model)
	}
}

// TestListUnverifiedWriteConsents_EveryModelOnce pins the grants panel's
// data source: one row per wiring.SupportedModels() entry, in that
// function's own order, so a management surface built on it can never
// offer — or omit — a model the connect path knows about. The FT-710's
// row reports NeedsConsent false rather than a state a user could be
// tempted to change, and a recorded grant shows up on ITS OWN row and no
// other.
func TestListUnverifiedWriteConsents_EveryModelOnce(t *testing.T) {
	a, _ := newTestApp(t)
	tempUserConfig(t)

	rows, err := a.ListUnverifiedWriteConsents()
	if err != nil {
		t.Fatalf("ListUnverifiedWriteConsents: unexpected error: %v", err)
	}
	models := wiring.SupportedModels()
	if len(rows) != len(models) {
		t.Fatalf("rows = %d, want %d (one per supported model): %+v", len(rows), len(models), rows)
	}
	for i, row := range rows {
		if row.Model != models[i] {
			t.Errorf("row %d Model = %q, want %q (wiring.SupportedModels' own order)", i, row.Model, models[i])
		}
		if row.Model == wiring.DefaultModel {
			if row.NeedsConsent {
				t.Errorf("%s row: NeedsConsent = true, want false", row.Model)
			}
			if row.Warning != "" {
				t.Errorf("%s row: Warning = %q, want \"\"", row.Model, row.Warning)
			}
			continue
		}
		if !row.NeedsConsent {
			t.Errorf("%s row: NeedsConsent = false, want true", row.Model)
		}
		if !strings.Contains(row.Warning, row.Model) {
			t.Errorf("%s row: Warning = %q, want it to name that model", row.Model, row.Warning)
		}
		if row.Granted || row.Recorded {
			t.Errorf("%s row before any decision: Granted = %v, Recorded = %v; want false, false", row.Model, row.Granted, row.Recorded)
		}
	}

	if err := a.SetUnverifiedWriteConsent(eligibleModel, true); err != nil {
		t.Fatalf("SetUnverifiedWriteConsent(%q, true): unexpected error: %v", eligibleModel, err)
	}
	rows, err = a.ListUnverifiedWriteConsents()
	if err != nil {
		t.Fatalf("ListUnverifiedWriteConsents after a grant: unexpected error: %v", err)
	}
	for _, row := range rows {
		wantGranted := row.Model == eligibleModel
		if row.Granted != wantGranted || row.Recorded != wantGranted {
			t.Errorf("%s row after granting %s: Granted = %v, Recorded = %v; want %v, %v — a decision must land on one model's row only",
				row.Model, eligibleModel, row.Granted, row.Recorded, wantGranted, wantGranted)
		}
	}
}

// TestUnverifiedWriteConsent_CorruptStoreSpeaksUserconfigsOwnWords pins
// that a settings file this build cannot read reaches the FRONTEND with
// internal/userconfig's own text, verbatim: it names the file and tells
// the user to repair it by hand, and a generic "could not read settings"
// would throw both away. Every consent surface — the two readers, the
// writer and the connect path — must fail the same way, since a listing
// that quietly showed every model "off" would be telling a user their
// recorded decisions were gone.
func TestUnverifiedWriteConsent_CorruptStoreSpeaksUserconfigsOwnWords(t *testing.T) {
	containSnapshotDir(t)
	a, _ := newTestApp(t)
	path := tempUserConfig(t)
	if err := os.WriteFile(path, []byte("{ this is not JSON"), 0o600); err != nil {
		t.Fatalf("writing the corrupt fixture: %v", err)
	}
	_, loadErr := userconfig.Load(path)
	if loadErr == nil {
		t.Fatal("test setup: userconfig.Load accepted the corrupt fixture — every assertion below would be vacuous")
	}
	want := loadErr.Error()

	// The connect path must not even reach the seam: a store it cannot
	// read has no safe default, so nothing is opened.
	var rec realSessionRecorder
	rec.install(t)

	checks := []struct {
		name string
		err  error
	}{
		{"GetUnverifiedWriteConsent", func() error {
			_, err := a.GetUnverifiedWriteConsent(eligibleModel)
			return err
		}()},
		{"ListUnverifiedWriteConsents", func() error {
			_, err := a.ListUnverifiedWriteConsents()
			return err
		}()},
		{"SetUnverifiedWriteConsent", a.SetUnverifiedWriteConsent(eligibleModel, true)},
		{"Connect", func() error {
			_, err := a.Connect("/dev/nonexistent-rigprog-test-port", eligibleModel)
			return err
		}()},
	}
	for _, c := range checks {
		if c.err == nil {
			t.Errorf("%s over a corrupt store: err = nil, want internal/userconfig's own refusal", c.name)
			continue
		}
		if c.err.Error() != want {
			t.Errorf("%s over a corrupt store:\nerr  = %q\nwant = %q (userconfig's own text, verbatim)", c.name, c.err, want)
		}
	}

	rec.mu.Lock()
	calls := len(rec.calls)
	rec.mu.Unlock()
	if calls != 0 {
		t.Errorf("openRealSessionWith called %d times over a corrupt store, want 0 — nothing may be opened on a guess", calls)
	}
	a.mu.Lock()
	conn := a.conn
	a.mu.Unlock()
	if conn != nil {
		t.Error("a.conn is set after a refused Connect, want nil")
	}

	// The corrupt file is left exactly as it was — userconfig's promise,
	// restated here because it is app/ that now calls it.
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("re-reading the corrupt fixture: %v", err)
	}
	if string(b) != "{ this is not JSON" {
		t.Errorf("the corrupt settings file was rewritten: %q", b)
	}
}

// TestConnect_RecordedConsentReachesTheSession is this task's connect-path
// pin: the decision the user recorded is read at connect time and spent on
// the session, and the ConnectionInfo the frontend gets back says both
// whether this radio needs a decision at all and whether one has been
// taken — the two facts an "ask me" prompt has to be gated on.
//
// Whether consent is IN FORCE is deliberately not among them: that is read
// from the session's own capabilities (GetUISpec's
// UnverifiedWritesConsented), so the interface cannot show consent that
// the running session does not have.
func TestConnect_RecordedConsentReachesTheSession(t *testing.T) {
	cases := []struct {
		name         string
		record       func(*testing.T, *App)
		wantConsent  bool
		wantRecorded bool
	}{
		{
			name:   "never asked",
			record: func(*testing.T, *App) {},
		},
		{
			name: "declined",
			record: func(t *testing.T, a *App) {
				if err := a.SetUnverifiedWriteConsent(eligibleModel, false); err != nil {
					t.Fatalf("SetUnverifiedWriteConsent(false): %v", err)
				}
			},
			wantRecorded: true,
		},
		{
			name: "granted",
			record: func(t *testing.T, a *App) {
				if err := a.SetUnverifiedWriteConsent(eligibleModel, true); err != nil {
					t.Fatalf("SetUnverifiedWriteConsent(true): %v", err)
				}
			},
			wantConsent:  true,
			wantRecorded: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			containSnapshotDir(t)
			a, _ := newTestApp(t)
			tempUserConfig(t)
			tc.record(t, a)

			var rec realSessionRecorder
			rec.install(t)

			const port = "/dev/nonexistent-rigprog-test-port"
			info, err := a.Connect(port, eligibleModel)
			if err != nil {
				t.Fatalf("Connect(%q, %q): unexpected error: %v", port, eligibleModel, err)
			}
			t.Cleanup(func() { _ = a.Disconnect() })

			call := rec.only(t)
			if call.model != eligibleModel {
				t.Errorf("openRealSessionWith model = %q, want %q", call.model, eligibleModel)
			}
			if call.portPath != port {
				t.Errorf("openRealSessionWith portPath = %q, want %q", call.portPath, port)
			}
			if call.opts.ConsentUnverifiedWrites != tc.wantConsent {
				t.Errorf("SessionOptions.ConsentUnverifiedWrites = %v, want %v — the recorded decision did not reach the session",
					call.opts.ConsentUnverifiedWrites, tc.wantConsent)
			}

			if !info.NeedsUnverifiedConsent {
				t.Errorf("ConnectionInfo.NeedsUnverifiedConsent = false, want true for %s", eligibleModel)
			}
			if info.UnverifiedConsentRecorded != tc.wantRecorded {
				t.Errorf("ConnectionInfo.UnverifiedConsentRecorded = %v, want %v", info.UnverifiedConsentRecorded, tc.wantRecorded)
			}
		})
	}
}

// TestConnect_HardwareVerifiedModelSpendsNoConsent pins the other half of
// the connect path: a radio whose writes are hardware-verified reports
// NeedsUnverifiedConsent false, and a grant sitting in the store under its
// slug — which only a hand-edited file, or a build in which that radio's
// writes were still unverified, could have put there — is NOT spent. The
// consent option would be a no-op on such a session anyway (core/spec's
// transform changes nothing in a set with no write-side Unverified), so
// this costs nothing and keeps a stale key from ever reading as a live
// grant.
func TestConnect_HardwareVerifiedModelSpendsNoConsent(t *testing.T) {
	containSnapshotDir(t)
	a, _ := newTestApp(t)
	path := tempUserConfig(t)
	if err := userconfig.SetUnverifiedWrites(path, wiring.ModelSlug(wiring.DefaultModel), true); err != nil {
		t.Fatalf("seeding a stale grant: %v", err)
	}

	var rec realSessionRecorder
	rec.install(t)

	info, err := a.Connect("/dev/nonexistent-rigprog-test-port", wiring.DefaultModel)
	if err != nil {
		t.Fatalf("Connect(%q): unexpected error: %v", wiring.DefaultModel, err)
	}
	t.Cleanup(func() { _ = a.Disconnect() })

	if got := rec.only(t).opts.ConsentUnverifiedWrites; got {
		t.Errorf("SessionOptions.ConsentUnverifiedWrites = true for %s, want false — a stale key must not be spent", wiring.DefaultModel)
	}
	if info.NeedsUnverifiedConsent {
		t.Errorf("ConnectionInfo.NeedsUnverifiedConsent = true for %s, want false", wiring.DefaultModel)
	}
	if info.UnverifiedConsentRecorded {
		t.Errorf("ConnectionInfo.UnverifiedConsentRecorded = true for %s, want false — there is no decision to have taken", wiring.DefaultModel)
	}

	// The stale key itself is left alone: this path reads the store, it
	// does not tidy it.
	settings, err := userconfig.Load(path)
	if err != nil {
		t.Fatalf("re-reading the store: %v", err)
	}
	if granted, recorded := settings.UnverifiedWritesFor(wiring.ModelSlug(wiring.DefaultModel)); !granted || !recorded {
		t.Errorf("the seeded key is now granted=%v recorded=%v, want true, true — connect must not rewrite the store", granted, recorded)
	}
}
