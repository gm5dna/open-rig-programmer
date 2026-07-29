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

// currentModel is THE model resolver for this package (M9c-5 E4): the one
// answer to "which radio is this?" that every model-keyed lookup here —
// capabilities, snapshot directory, bank synthesis, prose, settings
// descriptor — is required to consume, so no site re-derives it and
// drifts from the others. It returns a model NAME (the driver.Registry
// key internal/wiring's tables are keyed by), never capabilities.
//
// The chain, in order:
//
//  1. The connected session's own model. A live session IS the radio, so
//     its Capabilities().Model is authoritative — and
//     driver.Registry.Register refuses any driver whose Model() and
//     Capabilities().Model disagree, so this is exactly the registry key
//     the session was opened under.
//  2. Otherwise the working copy's own Radio.Model
//     (core/codeplug/radioinfo.go), but ONLY when capsForModel recognises
//     it. This is Fix B1's rule (Codex fix-B review, HIGH: "offline
//     import transforms data against the WRONG model's capabilities, and
//     the result can later pass the send gate"), generalised from
//     currentCaps to every site: an offline import or edit against a
//     working copy that is NOT an FT-710 must be transformed/validated
//     against THAT radio's own vocabulary, never silently against the
//     FT-710's.
//  3. Otherwise wiring.DefaultModel (the FT-710) — when working is nil,
//     its Radio.Model is "", or that model names no registered driver (a
//     future dialect-only model with no driver yet — see
//     .superpowers/sdd/HANDOFF-m9c.md's still-open "FTdx10 slice" — or a
//     hand-edited/corrupt file). Refuse-before-corrupt: an unresolvable
//     model degrades to a KNOWN-safe baseline rather than being handed on
//     to lookups that would all fail on it at once. wiring.DefaultModel is
//     a hardcoded, always-registered model name (internal/wiring's own
//     TestStaticCapabilities_FT710EqualsDriver pins this), so this tail
//     cannot itself fail to resolve.
//
// Step 2's recognition check is the whole reason this is a resolver
// rather than a field read (M9c-5 E4's design note): handing a RAW,
// unrecognised Radio.Model to wiring.SynthesiseDiscoveredBanks would make
// it report ok == false and SILENTLY DROP a legacy file's discovered
// 60m/EMG channels out of the grid — data loaded but invisible, the exact
// failure synthesiseDiscoveredBanks exists to prevent.
func currentModel(conn *connectionState, working *codeplug.Codeplug) string {
	if conn != nil {
		return conn.session.Capabilities().Model
	}
	if working != nil && working.Radio.Model != "" {
		if _, err := capsForModel(working.Radio.Model); err == nil {
			return working.Radio.Model
		}
	}
	return wiring.DefaultModel
}

// currentCaps returns the capabilities Validate/UpdateChannel(s)/import
// should validate/transform against, and whether that result is merely
// advisory (task-15 brief §2's Validate bullet): the connected session's
// OWN effective capabilities (authoritative — includes discovered
// regional banks) when conn is non-nil, otherwise the static offline
// baseline of currentModel's resolved model, advisory: true — it lacks
// discovered regional banks, matching the CLI's offline import
// adjudication.
//
// The disconnected model resolution is currentModel's, not a second copy
// of it (M9c-5 E4): see that function for the fallback chain and why an
// unrecognised working-copy model degrades to wiring.DefaultModel. The
// connected branch deliberately does NOT go through currentModel —
// currentModel would only name the model, and this function needs the
// session's own EFFECTIVE capabilities (discovered 60m/EMG inventory
// included), which no static per-model lookup can reproduce.
//
// FT-710 byte-identity: a working copy whose Radio.Model is "FT-710"
// (wiring.DefaultModel's own value), or is empty, or is nil, resolves via
// the EXACT SAME capsForModel(wiring.DefaultModel) call this function
// made unconditionally before Fix B1 — see
// TestCurrentCaps_DisconnectedFT710WorkingIsByteIdentical.
//
// capsForModel's error is discarded rather than propagated: currentModel
// has already established that the model it returns is either recognised
// or wiring.DefaultModel, every caller of currentCaps treats the
// disconnected baseline as advisory rather than a hard failure, and this
// function's two-result signature predates task 41 (M9a-5) and is
// unchanged by it.
func currentCaps(conn *connectionState, working *codeplug.Codeplug) (spec.Capabilities, bool) {
	if conn != nil {
		return conn.session.Capabilities(), false
	}
	caps, _ := capsForModel(currentModel(conn, working))
	return caps, true
}

// capsForModel indirects wiring.StaticCapabilities — called only by
// currentModel (the recognition check) and currentCaps (the value it
// returns) — so this package's own tests can exercise the working-copy-
// model resolution above against a model name wiring itself does not
// register (no second real driver exists yet: internal/wiring's
// realDrivers table lists only "FT-710" today — see
// .superpowers/sdd/HANDOFF-m9c.md). Reassigned ONLY by tests (e.g.
// TestCurrentCaps_DisconnectedUsesWorkingCopyModel), restored via
// t.Cleanup; production code must never reassign it. Mirrors
// internal/wiring's own FakeSessionOpts seam (internal/wiring/fake.go)
// for the identical reason: process-global, unsynchronised state, safe
// only for tests that do not call t.Parallel().
var capsForModel = wiring.StaticCapabilities

// supportedModels indirects wiring.SupportedModels — connectModel's
// (connection.go) only caller — for exactly the same reason capsForModel
// exists, and under exactly the same rules: internal/wiring registers one
// model today, so the ONLY way to prove that Connect/ConnectDemo's
// validated model parameter is genuinely THREADED into the three
// model-keyed wiring calls the connect path makes — rather than each
// still naming wiring.DefaultModel — is to let a test admit a second
// model name at the validation gate and then observe where it arrives
// (see TestConnect_ResolvedModelThreadsIntoWiring). Reassigned ONLY by
// tests, restored via t.Cleanup; production code must never reassign it.
var supportedModels = wiring.SupportedModels
