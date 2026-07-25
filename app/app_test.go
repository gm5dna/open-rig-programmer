// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/clone"
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
	_, advisory := currentCaps(nil)
	if !advisory {
		t.Error("currentCaps(nil): advisory = false, want true (disconnected caps are always advisory)")
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
