// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"sync"

	"github.com/gm5dna/open-rig-programmer/core/clone"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// connectionState holds everything Connect/ConnectDemo build and every
// radio-touching bound method needs, or nil when not connected
// (task-15 brief §2's original shape: "connection: nil | {session
// *ft710.Session, closer, svc *clone.Service, demo bool}" — task 41 (M9a-5,
// the GUI-backend neutralisation) widens session to the model-neutral
// driver.Session interface; every FT-710-specific capability a caller
// needs is reached via an optional-interface type assertion against the
// concrete value a driver's Open returned, e.g. connection.go's
// driver.RegionReporter check for ConnectionInfo.Region).
type connectionState struct {
	session driver.Session
	closer  func() error
	svc     *clone.Service
	demo    bool
}

// transferState tracks ConfirmSend's async transfer (task-15 brief §2:
// "transfer state: idle | running (with cancel func)").
type transferState struct {
	running bool
	cancel  context.CancelFunc
}

// dialogAPI is the small interface (task-15 brief §2's "Runtime seams")
// every bound method uses instead of calling the Wails runtime dialog
// functions directly, so a unit test can inject a fake without a real
// window (see app_test.go's fakeDialogs). runtimeDialogs (below) is the
// production implementation.
type dialogAPI interface {
	OpenFile(opts wailsruntime.OpenDialogOptions) (string, error)
	SaveFile(opts wailsruntime.SaveDialogOptions) (string, error)
	Message(opts wailsruntime.MessageDialogOptions) (string, error)
}

// runtimeDialogs is dialogAPI's production implementation: thin
// wrappers over the Wails runtime dialog functions. ctxFn reads a.ctx
// AT CALL TIME (not captured at construction) — NewApp builds this
// before App.startup has ever run, so a.ctx is not set yet when
// runtimeDialogs itself is constructed.
type runtimeDialogs struct {
	ctxFn func() context.Context
}

func (d runtimeDialogs) OpenFile(opts wailsruntime.OpenDialogOptions) (string, error) {
	return wailsruntime.OpenFileDialog(d.ctxFn(), opts)
}

func (d runtimeDialogs) SaveFile(opts wailsruntime.SaveDialogOptions) (string, error) {
	return wailsruntime.SaveFileDialog(d.ctxFn(), opts)
}

func (d runtimeDialogs) Message(opts wailsruntime.MessageDialogOptions) (string, error) {
	return wailsruntime.MessageDialog(d.ctxFn(), opts)
}

// App is the sole bound struct (task-15 brief §2): every GUI<->core
// interaction goes through it. It owns clone/driver/ft710 (via
// internal/wiring)/codeplug/csvio/spec only through their public APIs —
// no GUI knowledge leaks below this package.
//
// mu guards every field below it. It is held only for short,
// non-blocking critical sections — never across a svc call (ReadAll/
// PrepareSend/Execute), a dialog, or file I/O — so CancelTransfer and
// read-only accessors (IsDirty, GetCodeplug) never block behind a
// multi-second radio operation. Mutual exclusion of the radio
// operations themselves (ReadAll/PrepareSend/Execute) is enforced by
// clone.Service's OWN try-lock (core/clone/service.go's acquireOp) —
// this App surfaces a resulting *clone.BusyError as a friendly error
// (see friendlyErr) rather than re-implementing that lock here. transfer
// additionally gates the few operations the brief calls out explicitly
// (Disconnect, LoadFile, ConfirmSend) that must refuse outright while a
// send is in flight, since those do not themselves go through svc.
type App struct {
	ctx context.Context

	mu sync.Mutex

	conn *connectionState

	// baseline is the last full radio read (ReadRadio); nil until then.
	baseline *codeplug.Codeplug
	// working is the editable copy; nil until ReadRadio or LoadFile.
	working *codeplug.Codeplug
	// workingPath is the last save/load path; "" if none (task-15 brief
	// §2). ReadRadio clears it: a fresh read is not the same artefact as
	// whatever was last loaded/saved, and silently letting SaveFile(path)
	// overwrite an unrelated file with new radio content would be
	// dangerous.
	workingPath string
	dirty       bool
	// workingRev is a monotonically-increasing revision counter bumped
	// under a.mu at EVERY working-copy content mutation (ReadRadio,
	// loadFilePath, UpdateChannel(s), ImportCSV/CHIRP, ReadSettingsRadio —
	// see bumpWorkingRevLocked's callers). Fix 4 (Codex M8b #4): SaveFile
	// snapshots this alongside its deep copy under a.mu, writes OUTSIDE the
	// lock, then clears dirty ONLY if workingRev still matches — so a
	// mutation that lands mid-save (e.g. ReadSettingsRadio, which is a
	// reservation holder Save does not exclude against) cannot leave the
	// newer working copy marked clean behind a stale on-disk snapshot.
	workingRev uint64
	// baselineStale is true once a send has touched the radio since the
	// last ReadRadio — see CodeplugView.BaselineStale's doc comment.
	baselineStale bool

	currentPlan *clone.SendPlan

	transfer transferState

	// opBusy names the App-level operation currently holding Fix 2's
	// exclusive-operation reservation (Codex M6 #2, adjudicated HIGH:
	// "the controller does not reserve App-wide operations") — "" when
	// free, else one of "ReadRadio"/"DiffAgainstRadio"/"PrepareSend"/
	// "ReadSettingsRadio" (task 35), the four long-running methods that
	// read (or, for PrepareSend/DiffAgainstRadio, used to read outside mu)
	// `working`. transfer.running is a SEPARATE, pre-existing reservation
	// (ConfirmSend's own) that composes with this one rather than
	// double-booking — see checkNotBusyLocked/reserveOpLocked in
	// reservation.go.
	opBusy string

	// settingsDisplay maps a setting ID to its descriptor Display string
	// (task 35): built fresh by ReadSettingsRadio, under mu, once at the
	// START of each call, and cleared again once conn.svc.ReadSettings
	// returns — see progressCallback's doc comment (send.go) for why its
	// read-settings branch consults this map rather than re-scanning the
	// descriptor for every one of up to 296 progress events. nil except
	// for the duration of one ReadSettingsRadio call's own svc.ReadSettings
	// call.
	settingsDisplay map[string]string

	// emit sends one event to the frontend (task-15 brief §2's Events
	// seam). Always non-nil after NewApp; tests overwrite it with a
	// recorder (see app_test.go's eventRecorder).
	emit func(event string, data any)
	// dialogs is the Dialogs seam (task-15 brief §2); always non-nil
	// after NewApp.
	dialogs dialogAPI
}

// NewApp creates a new App application struct, wiring emit/dialogs to
// their production (Wails runtime) implementations. Both read a.ctx at
// CALL time via closures, since ctx is not set until startup runs.
func NewApp() *App {
	a := &App{}
	a.emit = func(event string, data any) {
		wailsruntime.EventsEmit(a.ctx, event, data)
	}
	a.dialogs = runtimeDialogs{ctxFn: func() context.Context { return a.ctx }}
	return a
}

// startup is called when the app starts. The context is saved so bound
// methods (via a.ctx) and emit/dialogs (via their ctxFn closures above)
// can call Wails runtime functions.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// OnBeforeClose implements the options.App.OnBeforeClose hook (wired in
// main.go): it prevents the window closing while a transfer is running
// (task-15 brief §2), showing a MessageDialog explanation first. Wails
// exempts this method from JS binding by function identity (matching it
// against options.App.OnBeforeClose) — it is NOT part of this package's
// bound-method surface despite being exported.
func (a *App) OnBeforeClose(ctx context.Context) bool {
	if !a.transferRunning() {
		return false
	}
	_, _ = a.dialogs.Message(wailsruntime.MessageDialogOptions{
		Type:    wailsruntime.WarningDialog,
		Title:   "Transfer in progress",
		Message: "A transfer to the radio is still running. Wait for it to finish, or cancel it, before closing.",
	})
	return true
}

// transferRunning reports whether a send transfer is currently running.
func (a *App) transferRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.transfer.running
}

// IsDirty reports whether the working copy has unsaved edits (task-15
// brief §2's SaveFile bullet: "provide IsDirty() bool" so the frontend
// can decide whether to ask before a destructive LoadFile). Fix 4 (Codex
// M8b #4) makes this the authoritative post-save dirty state the frontend
// save wrappers read back: SaveFile may leave it TRUE when a mutation
// landed mid-save, so the wrappers must reflect it rather than force false.
func (a *App) IsDirty() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.dirty
}

// bumpWorkingRevLocked increments the working-copy revision counter (see
// workingRev's doc comment). Callers must hold a.mu, and MUST call it at
// every site that mutates the working copy's content, so a Save in flight
// can detect a concurrent mutation and refuse to clear dirty over it.
func (a *App) bumpWorkingRevLocked() {
	a.workingRev++
}

// currentCaps returns the capabilities Validate/UpdateChannel(s) should
// validate against, and whether that result is merely advisory (task-15
// brief §2's Validate bullet): the connected session's OWN effective
// capabilities (authoritative — includes discovered regional banks) when
// conn is non-nil, otherwise the static offline baseline
// (wiring.StaticCapabilities(wiring.DefaultModel), advisory: true — it
// lacks discovered regional banks, matching the CLI's offline import
// adjudication).
//
// wiring.DefaultModel is a hardcoded, always-registered model name
// (internal/wiring's own TestStaticCapabilities_FT710EqualsDriver pins
// this) — the lookup cannot fail for it in practice, so the error is
// discarded here rather than propagated: every caller of currentCaps
// already treats the disconnected baseline as advisory, never a hard
// failure, and this function's two-result signature predates task 41
// (M9a-5) and is unchanged by it.
func currentCaps(conn *connectionState) (spec.Capabilities, bool) {
	if conn != nil {
		return conn.session.Capabilities(), false
	}
	caps, _ := wiring.StaticCapabilities(wiring.DefaultModel)
	return caps, true
}
