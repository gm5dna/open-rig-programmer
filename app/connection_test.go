// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// TestConnectDemo_Lifecycle exercises the REAL ConnectDemo/Disconnect
// path (via internal/wiring.OpenFakeSessionFor — task-15 brief §2's "app:
// unit tests against fakeradio via internal/wiring"). Cheap: ConnectDemo
// only opens+probes (no full ReadAll).
//
// Model/Region assertions pin task 41 (M9a-5)'s migration off the
// Task-39 compatibility wrappers: ConnectionInfo.Model now comes from
// sess.Capabilities().Model rather than the wiring.Model alias (deleted
// by task 44, once unreferenced), and Region from a driver.RegionReporter
// type assertion rather than a direct *ft710.Session.Region() call —
// both must still report exactly what they did before the migration.
func TestConnectDemo_Lifecycle(t *testing.T) {
	a, _ := newTestApp(t)

	info, err := a.ConnectDemo("")
	if err != nil {
		t.Fatalf("ConnectDemo: unexpected error: %v", err)
	}
	if !info.Demo {
		t.Error("ConnectDemo: ConnectionInfo.Demo = false, want true")
	}
	if info.CATID != "0800" {
		t.Errorf("ConnectDemo: CATID = %q, want %q", info.CATID, "0800")
	}
	if info.Model != wiring.DefaultModel {
		t.Errorf("ConnectDemo: Model = %q, want %q", info.Model, wiring.DefaultModel)
	}
	// The default fakeradio image (ImageUK) is HW-CONFIRMED 2026-07-13 to
	// carry no 5xx bank at all — deriveRegion's "no-60m" case.
	if info.Region != "no-60m" {
		t.Errorf("ConnectDemo: Region = %q, want %q", info.Region, "no-60m")
	}

	if _, err := a.ConnectDemo(""); !errors.Is(err, ErrAlreadyConnected) {
		t.Errorf("ConnectDemo while connected: err = %v, want ErrAlreadyConnected", err)
	}

	if err := a.Disconnect(); err != nil {
		t.Fatalf("Disconnect: unexpected error: %v", err)
	}
	if err := a.Disconnect(); !errors.Is(err, ErrNotConnected) {
		t.Errorf("Disconnect while not connected: err = %v, want ErrNotConnected", err)
	}

	// Reconnecting after a clean disconnect must work.
	if _, err := a.ConnectDemo(""); err != nil {
		t.Fatalf("ConnectDemo after Disconnect: unexpected error: %v", err)
	}
}

// TestDisconnect_KeepsWorkingCopy confirms (Fix 1, adjudicated HIGH,
// Codex M6 #1 — "Verify Go's Disconnect keeps working (it does —
// confirm, don't change)") that Disconnect never touches
// working/dirty/workingPath: only a.conn and a.currentPlan are cleared.
// The frontend-side half of Fix 1 (appState.disconnectConnection) relies
// entirely on this Go contract already holding.
func TestDisconnect_KeepsWorkingCopy(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.ConnectDemo(""); err != nil {
		t.Fatalf("ConnectDemo: %v", err)
	}
	if _, err := a.ReadRadio(); err != nil {
		t.Fatalf("ReadRadio: %v", err)
	}
	// An edit made while connected.
	view, err := a.GetCodeplug()
	if err != nil {
		t.Fatalf("GetCodeplug: %v", err)
	}
	edited := view.Channels[0]
	if edited.Data == nil {
		t.Fatal("test setup: slot 0's Data is nil, want a populated channel to edit")
	}
	edited.Data.Tag = "DISCONNECT-TEST"
	if _, err := a.UpdateChannel(edited); err != nil {
		t.Fatalf("UpdateChannel: %v", err)
	}
	if !a.IsDirty() {
		t.Fatal("test setup: not dirty after an edit — the rest of this test would be vacuous")
	}

	if err := a.Disconnect(); err != nil {
		t.Fatalf("Disconnect: unexpected error: %v", err)
	}

	if !a.IsDirty() {
		t.Error("IsDirty() after Disconnect = false, want true — the working copy must survive a disconnect")
	}
	got, err := a.GetCodeplug()
	if err != nil {
		t.Fatalf("GetCodeplug after Disconnect: unexpected error: %v", err)
	}
	if got.Channels[0].Data == nil || got.Channels[0].Data.Tag != "DISCONNECT-TEST" {
		t.Errorf("GetCodeplug after Disconnect: slot 0 = %+v, want the edit still present", got.Channels[0])
	}
	// Save must still be possible (SaveFile refuses only on ErrNothingLoaded/
	// busy — neither applies here).
	if err := a.SaveFile(filepath.Join(t.TempDir(), "still-saveable.json")); err != nil {
		t.Errorf("SaveFile after Disconnect: unexpected error: %v (Save must still be possible)", err)
	}
}

// TestListPorts_NoError pins that ListPorts at least runs cleanly
// end-to-end (transport.Discover) — its content is host-dependent, so
// nothing about WHICH ports it finds is asserted.
func TestListPorts_NoError(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.ListPorts(); err != nil {
		t.Errorf("ListPorts: unexpected error: %v", err)
	}
}

// TestGetSupportedModels_ContainsDefaultModel pins the new (task 41,
// M9a-5) bound method: registry-driven (internal/wiring.SupportedModels),
// so it must at least contain wiring.DefaultModel, sorted (matching
// SupportedModels' own guarantee).
func TestGetSupportedModels_ContainsDefaultModel(t *testing.T) {
	a, _ := newTestApp(t)
	got := a.GetSupportedModels()
	if !reflect.DeepEqual(got, wiring.SupportedModels()) {
		t.Errorf("GetSupportedModels() = %v, want %v (wiring.SupportedModels())", got, wiring.SupportedModels())
	}
	found := false
	for _, m := range got {
		if m == wiring.DefaultModel {
			found = true
		}
	}
	if !found {
		t.Errorf("GetSupportedModels() = %v, want it to contain wiring.DefaultModel %q", got, wiring.DefaultModel)
	}
}

// TestConnect_EmptyModelIsTheDefaultModel pins Connect/ConnectDemo's new
// model parameter's empty case (M9c-5 E4): "" means wiring.DefaultModel,
// so every existing caller — including the frontend's two bridge sites,
// which pass the empty string explicitly — connects exactly as it did
// before the parameter existed. Naming the default model explicitly must
// reach the same place.
func TestConnect_EmptyModelIsTheDefaultModel(t *testing.T) {
	for _, model := range []string{"", wiring.DefaultModel} {
		t.Run("model="+model, func(t *testing.T) {
			a, _ := newTestApp(t)
			info, err := a.ConnectDemo(model)
			if err != nil {
				t.Fatalf("ConnectDemo(%q): unexpected error: %v", model, err)
			}
			t.Cleanup(func() { _ = a.Disconnect() })
			if info.Model != wiring.DefaultModel {
				t.Errorf("ConnectDemo(%q): Model = %q, want %q", model, info.Model, wiring.DefaultModel)
			}
		})
	}
	if got, err := connectModel(""); err != nil || got != wiring.DefaultModel {
		t.Errorf("connectModel(\"\") = %q, %v; want %q, nil", got, err, wiring.DefaultModel)
	}
}

// TestConnect_UnknownModelRefused pins the validation half of the model
// parameter (M9c-5 E4): a model naming no registered driver is refused
// with the CLI's own error shape — a *wiring.UnknownModelError carrying
// the offending name and the full supported list, whose Error() text
// therefore names every model the caller could have asked for — and
// refused BEFORE any side effect, so nothing is created and no session is
// left behind.
func TestConnect_UnknownModelRefused(t *testing.T) {
	const bogus = "NoSuchRadioModel"
	calls := []struct {
		name string
		call func(*App) (ConnectionInfo, error)
	}{
		{"Connect", func(a *App) (ConnectionInfo, error) {
			return a.Connect("/dev/nonexistent-rigprog-test-port", bogus)
		}},
		{"ConnectDemo", func(a *App) (ConnectionInfo, error) { return a.ConnectDemo(bogus) }},
	}
	for _, tc := range calls {
		t.Run(tc.name, func(t *testing.T) {
			a, _ := newTestApp(t)
			if _, err := tc.call(a); err == nil {
				t.Fatalf("%s(%q): err = nil, want a *wiring.UnknownModelError", tc.name, bogus)
			} else {
				var unknown *wiring.UnknownModelError
				if !errors.As(err, &unknown) {
					t.Fatalf("%s(%q): err = %v, want a *wiring.UnknownModelError", tc.name, bogus, err)
				}
				if unknown.Model != bogus {
					t.Errorf("%s(%q): UnknownModelError.Model = %q, want %q", tc.name, bogus, unknown.Model, bogus)
				}
				if !reflect.DeepEqual(unknown.Supported, wiring.SupportedModels()) {
					t.Errorf("%s(%q): UnknownModelError.Supported = %v, want wiring.SupportedModels() %v", tc.name, bogus, unknown.Supported, wiring.SupportedModels())
				}
			}
			a.mu.Lock()
			conn := a.conn
			a.mu.Unlock()
			if conn != nil {
				t.Errorf("%s(%q): a.conn is set, want nil (the refusal must leave no session behind)", tc.name, bogus)
			}
		})
	}
}

// TestConnect_ResolvedModelThreadsIntoWiring is the connect cluster's
// threading pin (M9c-5 E4): the ONE model the connect path resolves must
// reach all THREE of its model-keyed wiring calls —
// wiring.ResolveSnapshotDir, wiring.OpenFakeSessionFor and
// wiring.OpenRealSessionFor — rather than each naming wiring.DefaultModel
// for itself, which is what a snapshot directory belonging to a different
// radio than the session writing into it would mean.
//
// The evidence is arranged so that a model that did NOT arrive cannot
// produce it, which is why testModel is an UNREGISTRABLE name and not the
// really-registered FTdx10 (M9c-6): a model wiring accepts would succeed at
// every site, leaving "the model arrived" indistinguishable from "the site
// used the default". testModel is instead admitted at app/'s validation
// gate ONLY (the supportedModels seam):
// wiring itself still refuses it, and each of the two session
// constructors reports that refusal as a *wiring.UnknownModelError
// NAMING testModel — an error only reachable by having been handed
// testModel. The snapshot directory is the third site's evidence: it is
// testModel's own per-model subdirectory (wiring.ResolveSnapshotDir's
// slug rule), it is asserted absent first, and connect creates it only
// AFTER validation has passed — so its existence proves both that
// validation admitted the model and that ResolveSnapshotDir received it
// rather than wiring.DefaultModel (whose directory is the un-slugged base
// one).
//
// THE REAL PATH'S EVIDENCE MOVED at the consent milestone, and this note
// is here so nobody reads more into the assertion below than it now
// proves. connect's real branch resolves the user's recorded consent
// first (consent.go's consentFor), and that begins with the SAME shared
// eligibility predicate — so testModel is refused there, by
// wiring.NeedsUnverifiedConsent, before wiring.OpenRealSessionWith is
// reached. The evidence CLASS is unchanged (internal/wiring's own typed
// error, naming testModel, reachable only by having been handed it), but
// the site that raises it is one step earlier. That the resolved model
// reaches the session constructor ITSELF is now pinned directly, and more
// strongly, by consent_test.go's
// TestConnect_RecordedConsentReachesTheSession, which records the model,
// the port and the options openRealSessionWith was called with.
func TestConnect_ResolvedModelThreadsIntoWiring(t *testing.T) {
	// The real branch consults the consent store. testModel is refused
	// before it gets there (see above), but the seam is pointed at a
	// temporary file regardless, so no future reordering of connect can
	// make this test read the settings of whoever runs "go test".
	tempUserConfig(t)

	// connect() creates the snapshot directory for real, under
	// os.UserConfigDir(); contain it in a temp HOME so this test writes
	// nothing into the developer's own config directory.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))

	orig := supportedModels
	supportedModels = func() []string { return append(orig(), testModel) }
	t.Cleanup(func() { supportedModels = orig })

	wantDir, err := wiring.ResolveSnapshotDir("", testModel)
	if err != nil {
		t.Fatalf("wiring.ResolveSnapshotDir(\"\", %q): unexpected error: %v", testModel, err)
	}
	if _, err := os.Stat(wantDir); !os.IsNotExist(err) {
		t.Fatalf("test setup: %s already exists (err = %v) — the assertion below would pass vacuously", wantDir, err)
	}

	a, _ := newTestApp(t)

	// The demo path: OpenFakeSessionFor's own lookup is what refuses.
	_, err = a.ConnectDemo(testModel)
	assertUnknownModel(t, "ConnectDemo", err)

	if _, statErr := os.Stat(wantDir); statErr != nil {
		t.Errorf("snapshot directory %s: %v; want it created — ResolveSnapshotDir did not receive the resolved model", wantDir, statErr)
	}

	// The real path: OpenRealSessionFor's model lookup refuses before any
	// port is touched, so this needs no hardware and opens nothing.
	_, err = a.Connect("/dev/nonexistent-rigprog-test-port", testModel)
	assertUnknownModel(t, "Connect", err)
}

// assertUnknownModel fails unless err is a *wiring.UnknownModelError
// naming testModel — the evidence that internal/wiring, not app/'s own
// validation gate, refused it, and therefore that testModel reached the
// wiring call at all.
func assertUnknownModel(t *testing.T, call string, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s(%q): err = nil, want internal/wiring's own *wiring.UnknownModelError", call, testModel)
	}
	var unknown *wiring.UnknownModelError
	if !errors.As(err, &unknown) {
		t.Fatalf("%s(%q): err = %v, want a *wiring.UnknownModelError", call, testModel, err)
	}
	if unknown.Model != testModel {
		t.Errorf("%s(%q): UnknownModelError.Model = %q, want %q — the model did not reach the wiring call", call, testModel, unknown.Model, testModel)
	}
}

// TestReadRadio_NotConnected pins the connection-required guard shared
// by every radio-touching bound method.
func TestReadRadio_NotConnected(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.ReadRadio(); !errors.Is(err, ErrNotConnected) {
		t.Errorf("ReadRadio while not connected: err = %v, want ErrNotConnected", err)
	}
}

// TestDiffAgainstRadio_NotConnected pins the same guard for
// DiffAgainstRadio.
func TestDiffAgainstRadio_NotConnected(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.DiffAgainstRadio(); !errors.Is(err, ErrNotConnected) {
		t.Errorf("DiffAgainstRadio while not connected: err = %v, want ErrNotConnected", err)
	}
}

// TestPrepareSend_NotConnected pins the same guard for PrepareSend.
func TestPrepareSend_NotConnected(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.PrepareSend(); !errors.Is(err, ErrNotConnected) {
		t.Errorf("PrepareSend while not connected: err = %v, want ErrNotConnected", err)
	}
}

// TestConfirmSend_NoActivePlan pins ConfirmSend's synchronous
// "no plan" pre-check.
func TestConfirmSend_NoActivePlan(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.ConnectDemo(""); err != nil {
		t.Fatalf("ConnectDemo: %v", err)
	}
	if err := a.ConfirmSend("anything", ""); !errors.Is(err, ErrNoActivePlan) {
		t.Errorf("ConfirmSend with no plan: err = %v, want ErrNoActivePlan", err)
	}
}

// TestCancelTransfer_NothingRunning pins CancelTransfer's guard when
// idle.
func TestCancelTransfer_NothingRunning(t *testing.T) {
	a, _ := newTestApp(t)
	if err := a.CancelTransfer(); !errors.Is(err, ErrNoTransferRunning) {
		t.Errorf("CancelTransfer while idle: err = %v, want ErrNoTransferRunning", err)
	}
}

// TestConnectDemo_ConsentFieldsFalseAndStoreUnread pins that the demo
// branch of connect is untouched by consent: ConnectDemo returns both
// consent fields false, whatever radio it simulates, and never reads the
// settings store at all — proved by pointing the store at a file this
// build cannot parse and watching the connection succeed anyway.
//
// False is the honest value here rather than a copy of the real branch's:
// wiring.OpenFakeSessionFor takes no SessionOptions, so a simulator
// session spends no consent and needs none. A GUI that prompted for
// consent on a demo connection would be asking a user to authorise a write
// to a radio that does not exist.
func TestConnectDemo_ConsentFieldsFalseAndStoreUnread(t *testing.T) {
	for _, model := range []string{wiring.DefaultModel, wiring.FTdx10Model} {
		t.Run(model, func(t *testing.T) {
			a, _ := newTestApp(t)
			path := tempUserConfig(t)
			if err := os.WriteFile(path, []byte("{ this is not JSON"), 0o600); err != nil {
				t.Fatalf("writing the corrupt fixture: %v", err)
			}

			info, err := a.ConnectDemo(model)
			if err != nil {
				t.Fatalf("ConnectDemo(%q) over an unreadable store: unexpected error: %v — the demo branch must not consult it", model, err)
			}
			t.Cleanup(func() { _ = a.Disconnect() })
			if info.NeedsUnverifiedConsent || info.UnverifiedConsentRecorded {
				t.Errorf("ConnectDemo(%q): NeedsUnverifiedConsent = %v, UnverifiedConsentRecorded = %v; want false, false",
					model, info.NeedsUnverifiedConsent, info.UnverifiedConsentRecorded)
			}
		})
	}
}
