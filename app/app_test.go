// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/clone"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// testAppCtxTimeout bounds every test App's ctx — generous, since a full
// MEM+PMS ReadAll over fakeradio genuinely takes a few real seconds (see
// core/clone/helpers_test.go's identical reasoning).
const testAppCtxTimeout = 60 * time.Second

// newTestApp returns an *App wired for unit testing: emit records into a
// slice a test can inspect (never touches the real Wails runtime, which
// is unavailable under plain `go test`), dialogs default to a fake that
// always reports "user cancelled" unless a test overrides it, and ctx is
// a real (timeout-bounded) context.Context — a.ctx is nil until Wails'
// own startup() runs it, which never happens under `go test`.
func newTestApp(t *testing.T) (*App, *eventRecorder) {
	t.Helper()
	a := NewApp()
	rec := &eventRecorder{}
	a.emit = rec.record
	a.dialogs = &fakeDialogs{}
	ctx, cancel := context.WithTimeout(context.Background(), testAppCtxTimeout)
	t.Cleanup(cancel)
	a.ctx = ctx
	return a, rec
}

type recordedEvent struct {
	name string
	data any
}

// eventRecorder is a thread-safe recorder for a.emit — ConfirmSend's
// transfer goroutine calls record concurrently with the test's own
// goroutine polling named/all (see waitForEvent), so this must be safe
// under `go test -race`.
type eventRecorder struct {
	mu     sync.Mutex
	events []recordedEvent
}

func (r *eventRecorder) record(event string, data any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, recordedEvent{name: event, data: data})
}

func (r *eventRecorder) named(event string) []recordedEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []recordedEvent
	for _, e := range r.events {
		if e.name == event {
			out = append(out, e)
		}
	}
	return out
}

func (r *eventRecorder) all() []recordedEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]recordedEvent(nil), r.events...)
}

// fakeDialogs is the dialogAPI test double (task-15 brief §2: "Dialogs:
// small interface field ... tests inject fakes"). Zero value cancels
// every dialog (empty path, nil error) — the same signal a real Wails
// dialog gives when the user dismisses it.
type fakeDialogs struct {
	openFilePath string
	openFileErr  error
	saveFilePath string
	saveFileErr  error
	messages     []string
}

func (f *fakeDialogs) OpenFile(_ wailsruntime.OpenDialogOptions) (string, error) {
	return f.openFilePath, f.openFileErr
}

func (f *fakeDialogs) SaveFile(_ wailsruntime.SaveDialogOptions) (string, error) {
	return f.saveFilePath, f.saveFileErr
}

func (f *fakeDialogs) Message(opts wailsruntime.MessageDialogOptions) (string, error) {
	f.messages = append(f.messages, opts.Message)
	return "", nil
}

func TestNewApp_InitialState(t *testing.T) {
	a, _ := newTestApp(t)
	if a.IsDirty() {
		t.Error("IsDirty() on a fresh App = true, want false")
	}
	if _, err := a.GetCodeplug(); !errors.Is(err, ErrNothingLoaded) {
		t.Errorf("GetCodeplug() on a fresh App: err = %v, want ErrNothingLoaded", err)
	}
}

func TestCurrentCaps_DisconnectedIsAdvisory(t *testing.T) {
	_, advisory := currentCaps(nil, nil)
	if !advisory {
		t.Error("currentCaps(nil, nil): advisory = false, want true (disconnected caps are always advisory)")
	}
}

// defaultModelCaps is wiring.StaticCapabilities(wiring.DefaultModel) — the
// FT-710's own static baseline — fetched fresh so every comparison below
// is against the real registry rather than a hand-copied literal.
func defaultModelCaps(t *testing.T) spec.Capabilities {
	t.Helper()
	caps, err := wiring.StaticCapabilities(wiring.DefaultModel)
	if err != nil {
		t.Fatalf("wiring.StaticCapabilities(%q): unexpected error: %v", wiring.DefaultModel, err)
	}
	return caps
}

// TestCurrentCaps_DisconnectedNilWorkingIsDefaultModel pins the
// pre-existing (and still-required) behaviour: with no working copy at
// all, disconnected resolution is exactly wiring.DefaultModel's static
// baseline.
func TestCurrentCaps_DisconnectedNilWorkingIsDefaultModel(t *testing.T) {
	got, advisory := currentCaps(nil, nil)
	if !advisory {
		t.Error("currentCaps(nil, nil): advisory = false, want true")
	}
	if !reflect.DeepEqual(got, defaultModelCaps(t)) {
		t.Errorf("currentCaps(nil, nil) = %+v, want the default model's static caps", got)
	}
}

// TestCurrentCaps_DisconnectedFT710WorkingIsByteIdentical is the FT-710
// byte-identity requirement (Codex fix-B review, Fix B1): a working copy
// whose Radio.Model is exactly "FT-710" (wiring.DefaultModel's own value)
// must resolve EXACTLY as the nil-working case above.
func TestCurrentCaps_DisconnectedFT710WorkingIsByteIdentical(t *testing.T) {
	working := &codeplug.Codeplug{Radio: codeplug.RadioInfo{Model: "FT-710"}}
	got, advisory := currentCaps(nil, working)
	if !advisory {
		t.Error("currentCaps(nil, FT-710 working): advisory = false, want true")
	}
	if !reflect.DeepEqual(got, defaultModelCaps(t)) {
		t.Errorf("currentCaps(nil, FT-710 working) = %+v, want byte-identical to the default model's static caps", got)
	}
}

// TestCurrentCaps_DisconnectedEmptyModelFallsBack: a working copy present
// but with Radio.Model left "" (e.g. a hand-built Codeplug that never set
// RadioInfo) must fall back to wiring.DefaultModel, not fail or panic.
func TestCurrentCaps_DisconnectedEmptyModelFallsBack(t *testing.T) {
	working := &codeplug.Codeplug{Radio: codeplug.RadioInfo{}}
	got, advisory := currentCaps(nil, working)
	if !advisory {
		t.Error("currentCaps(nil, empty-model working): advisory = false, want true")
	}
	if !reflect.DeepEqual(got, defaultModelCaps(t)) {
		t.Errorf("currentCaps(nil, empty-model working) = %+v, want the default model's static caps", got)
	}
}

// TestCurrentCaps_DisconnectedUnregisteredModelFallsBack: a working copy
// naming a model internal/wiring does not register — a radio whose dialect
// exists in core/cat but whose driver has not landed, a file written by
// another programme for a radio this build has never carried, or a
// hand-edited/corrupt file — must fall back to wiring.DefaultModel rather
// than erroring: refuse-before-corrupt means this falls back to a
// KNOWN-safe baseline, never propagates the lookup failure into a
// zero/garbage Capabilities.
//
// The fixture name is deliberately not any real radio's. It used to be
// possible to describe this case as "the FTdx10, whose driver has not
// landed yet"; since M9c-6 that model is REGISTERED and resolves happily,
// so only a name no driver will ever answer to keeps this test about the
// fallback rather than about a scheduling accident.
func TestCurrentCaps_DisconnectedUnregisteredModelFallsBack(t *testing.T) {
	working := &codeplug.Codeplug{Radio: codeplug.RadioInfo{Model: "NoSuchRadioModel"}}
	got, advisory := currentCaps(nil, working)
	if !advisory {
		t.Error("currentCaps(nil, unregistered-model working): advisory = false, want true")
	}
	if !reflect.DeepEqual(got, defaultModelCaps(t)) {
		t.Errorf("currentCaps(nil, unregistered-model working) = %+v, want the default model's static caps (fallback)", got)
	}
}

// TestCurrentCaps_DisconnectedUsesWorkingCopyModel is Fix B1's positive
// case: given a model wiring.StaticCapabilities WOULD resolve, currentCaps
// must use THAT model's own capabilities, not silently substitute the
// FT-710's. It substitutes capsForModel (see its own doc comment, app.go)
// to exercise the resolution against a model name of the test's choosing,
// restoring the real function via t.Cleanup — a fixture model rather than
// the really-registered FTdx10 because the fixture's capabilities are the
// test's to choose (TagLen 42 here), so "these caps came from the
// working-copy model" is observable without depending on any real radio's
// values, which would change the day that radio's own facts changed.
func TestCurrentCaps_DisconnectedUsesWorkingCopyModel(t *testing.T) {
	fakeCaps := spec.Capabilities{Model: "TESTMODEL", CATID: "9999", TagLen: 42}
	orig := capsForModel
	capsForModel = func(model string) (spec.Capabilities, error) {
		if model == "TESTMODEL" {
			return fakeCaps, nil
		}
		return orig(model)
	}
	t.Cleanup(func() { capsForModel = orig })

	working := &codeplug.Codeplug{Radio: codeplug.RadioInfo{Model: "TESTMODEL"}}
	got, advisory := currentCaps(nil, working)
	if !advisory {
		t.Error("currentCaps(nil, TESTMODEL working): advisory = false, want true")
	}
	if !reflect.DeepEqual(got, fakeCaps) {
		t.Errorf("currentCaps(nil, TESTMODEL working) = %+v, want the working copy's OWN model's caps %+v", got, fakeCaps)
	}
}

// TestCurrentCaps_ConnectedIgnoresWorkingCopyModel: connected resolution
// must keep returning the session's own capabilities exactly as before
// this fix, regardless of what the working copy's Radio.Model says (a
// stale/mismatched working copy while connected is Validate's job to
// flag, not currentCaps' job to second-guess the live session over).
func TestCurrentCaps_ConnectedIgnoresWorkingCopyModel(t *testing.T) {
	sess := openTestSimSession(t)
	conn := &connectionState{session: sess}
	working := &codeplug.Codeplug{Radio: codeplug.RadioInfo{Model: "SomeOtherModel"}}

	got, advisory := currentCaps(conn, working)
	if advisory {
		t.Error("currentCaps(conn, working): advisory = true, want false (connected is authoritative)")
	}
	if !reflect.DeepEqual(got, sess.Capabilities()) {
		t.Errorf("currentCaps(conn, working) = %+v, want the connected session's own Capabilities()", got)
	}
}

// testModel is the model name every M9c-5 E4 threading test injects: a
// name internal/wiring does not register, so that "the resolved model
// reached this call site" is observable as an outcome that DIFFERS from
// the wiring.DefaultModel one, rather than being indistinguishable from
// the hardcoded behaviour these tests exist to prove is gone.
const testModel = "TESTMODEL"

// recogniseTestModel makes capsForModel (app.go's seam) recognise
// testModel, returning caps of the test's own, and restores the real
// function afterwards. internal/wiring registers two models since M9c-6,
// but neither can serve here (see capsForModel's doc comment): these tests
// need a model wiring itself REFUSES, so that "the resolved model reached
// this call site" shows up as an outcome no default-model path could
// produce.
func recogniseTestModel(t *testing.T) spec.Capabilities {
	t.Helper()
	caps := spec.Capabilities{Model: testModel, CATID: "9999", TagLen: 42}
	recogniseModelCaps(t, caps)
	return caps
}

// recogniseModelCaps is recogniseTestModel's caller-supplied-capabilities
// sibling: the same capsForModel substitution, but for a test that needs
// testModel to describe a SHAPE the FT-710 does not have — a bank whose
// FieldTagDisplay is Unsupported in both directions, say. Splitting it out
// keeps one substitution implementation rather than two that could drift
// on which model name they answer to or whether they restore the original.
func recogniseModelCaps(t *testing.T, caps spec.Capabilities) {
	t.Helper()
	orig := capsForModel
	capsForModel = func(model string) (spec.Capabilities, error) {
		if model == testModel {
			return caps, nil
		}
		return orig(model)
	}
	t.Cleanup(func() { capsForModel = orig })
}

// TestCurrentModel_FallbackChain pins the ONE resolver's whole chain
// (M9c-5 E4) in the order it must be applied: the connected session
// first, then the working copy's model when it is RECOGNISED, then
// wiring.DefaultModel. Every model-keyed site in this package consumes
// this function, so this test is what stops those sites drifting apart.
func TestCurrentModel_FallbackChain(t *testing.T) {
	recogniseTestModel(t)
	sess := openTestSimSession(t)
	conn := &connectionState{session: sess}

	tests := []struct {
		name    string
		conn    *connectionState
		working *codeplug.Codeplug
		want    string
	}{
		{
			name: "no connection, no working copy -> the default model",
			want: wiring.DefaultModel,
		},
		{
			name:    "working copy with no model -> the default model",
			working: &codeplug.Codeplug{Radio: codeplug.RadioInfo{}},
			want:    wiring.DefaultModel,
		},
		{
			name:    "working copy naming an unregistered model -> the default model",
			working: &codeplug.Codeplug{Radio: codeplug.RadioInfo{Model: "NoSuchRadioModel"}},
			want:    wiring.DefaultModel,
		},
		{
			name:    "working copy naming a recognised model -> that model",
			working: &codeplug.Codeplug{Radio: codeplug.RadioInfo{Model: testModel}},
			want:    testModel,
		},
		{
			name:    "connected -> the session's own model, whatever the working copy says",
			conn:    conn,
			working: &codeplug.Codeplug{Radio: codeplug.RadioInfo{Model: testModel}},
			want:    sess.Capabilities().Model,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := currentModel(tc.conn, tc.working); got != tc.want {
				t.Errorf("currentModel() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestCurrentModel_ConnectedModelIsTheRegistryKey pins the assumption the
// connected branch rests on: a session's Capabilities().Model is the
// registry key internal/wiring's model-keyed tables use, so handing it
// straight to wiring.StaticSettingsDescriptor/SynthesiseDiscoveredBanks
// resolves rather than failing. driver.Registry.Register enforces
// Model() == Capabilities().Model; this pins that the value that reaches
// those lookups really is a supported model name.
func TestCurrentModel_ConnectedModelIsTheRegistryKey(t *testing.T) {
	sess := openTestSimSession(t)
	got := currentModel(&connectionState{session: sess}, nil)
	for _, m := range wiring.SupportedModels() {
		if m == got {
			return
		}
	}
	t.Errorf("currentModel(connected) = %q, want one of wiring.SupportedModels() %v", got, wiring.SupportedModels())
}

func TestFriendlyErr_WrapsBusyError(t *testing.T) {
	busy := &clone.BusyError{InProgress: "Execute"}
	got := friendlyErr(busy)
	if got == nil {
		t.Fatal("friendlyErr(busy): nil")
	}
	if !errors.Is(got, clone.ErrBusy) {
		t.Errorf("friendlyErr(busy) = %v, want errors.Is(_, clone.ErrBusy)", got)
	}
	if !strings.Contains(strings.ToLower(got.Error()), "another operation is running") {
		t.Errorf("friendlyErr(busy).Error() = %q, want it to mention 'another operation is running'", got.Error())
	}
}

func TestFriendlyErr_PassesThroughOtherErrors(t *testing.T) {
	plain := errors.New("boom")
	if got := friendlyErr(plain); got != plain {
		t.Errorf("friendlyErr(plain) = %v, want the same error unchanged", got)
	}
	if friendlyErr(nil) != nil {
		t.Error("friendlyErr(nil) != nil")
	}
}

func TestOnBeforeClose_PreventsWhileTransferRunning(t *testing.T) {
	a, rec := newTestApp(t)
	a.transfer = transferState{running: true, cancel: func() {}}
	if prevent := a.OnBeforeClose(context.Background()); !prevent {
		t.Error("OnBeforeClose while transfer running = false, want true (prevent close)")
	}
	if len(rec.events) != 0 {
		t.Errorf("OnBeforeClose should not emit events, got %v", rec.events)
	}
}

func TestOnBeforeClose_AllowsWhenIdle(t *testing.T) {
	a, _ := newTestApp(t)
	if prevent := a.OnBeforeClose(context.Background()); prevent {
		t.Error("OnBeforeClose while idle = true, want false (allow close)")
	}
}
