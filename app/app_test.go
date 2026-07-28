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
// naming a model internal/wiring does not (yet) register — e.g. a second
// radio's dialect exists in core/cat but no driver/registry entry has
// landed yet (see .superpowers/sdd/HANDOFF-m9c.md's still-open "FTdx10
// slice"), or a hand-edited/corrupt file — must fall back to
// wiring.DefaultModel rather than erroring: refuse-before-corrupt means
// this falls back to a KNOWN-safe baseline, never propagates the lookup
// failure into a zero/garbage Capabilities.
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
// FT-710's. internal/wiring registers only "FT-710" today (no second
// driver exists yet — see HANDOFF-m9c.md), so this substitutes
// capsForModel (see its own doc comment, app.go) to exercise the
// resolution against a model name of the test's choosing, restoring the
// real function via t.Cleanup.
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
