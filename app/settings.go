// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/clone"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// currentSettingsDescriptor mirrors currentCaps' (app.go) connected/
// disconnected split, for the settings surface: the connected session's
// own driver.SettingsReader descriptor when the concrete session
// implements that OPTIONAL capability (see driver.SettingsReader's doc
// comment — the identical optional-interface reasoning
// core/clone/memory_selector.go's MemorySelector already established),
// Live true; otherwise (disconnected, or a future driver whose session
// lacks the capability — every ft710.Session implements it
// unconditionally today, so that branch is currently only reachable
// offline) the static wiring.StaticSettingsDescriptor(wiring.DefaultModel)
// baseline, Live false. Callers must hold a.mu (mirrors currentCaps' own
// contract: this reads only its conn argument, never a itself).
//
// The ok/error results StaticSettingsDescriptor returns are both
// discarded here — see currentCaps' doc comment (app.go) for why that is
// deliberate for wiring.DefaultModel specifically: a future model
// entirely lacking a settings surface would fall back to the zero
// SettingsDescriptor, exactly as a genuinely absent capability already
// does.
func currentSettingsDescriptor(conn *connectionState) (d driver.SettingsDescriptor, live bool) {
	if conn != nil {
		if reader, ok := conn.session.(driver.SettingsReader); ok {
			return reader.SettingsDescriptor(), true
		}
	}
	descriptor, _, _ := wiring.StaticSettingsDescriptor(wiring.DefaultModel)
	return descriptor, false
}

// settingsDisplayMap builds a setting-ID -> descriptor Display lookup from
// d, walking menus->groups->items exactly once — see progressCallback's
// doc comment (send.go) for why ReadSettingsRadio builds this once per
// read operation rather than letting the progress closure re-scan the
// descriptor for every event.
func settingsDisplayMap(d driver.SettingsDescriptor) map[string]string {
	out := make(map[string]string)
	for _, m := range d.Menus {
		for _, g := range m.Groups {
			for _, it := range g.Items {
				out[it.ID] = it.Display
			}
		}
	}
	return out
}

// GetSettingsSpec returns the radio's menu/EX settings STRUCTURE — menus,
// groups, and items, never a value (see GetSettings/ReadSettingsRadio for
// that) — so the frontend settings viewer can render its tree without
// hardcoding any FT-710 protocol fact: mirrors GetUISpec's own structure/
// discipline (uispec.go), every field mapped straight from
// driver.SettingsDescriptor via descriptorToSpecView (convert.go), nothing
// invented here. See currentSettingsDescriptor's doc comment for exactly
// which descriptor is used and when. Works offline (never errors): the
// static ft710.SettingsDescriptor() baseline is always available even with
// no session.
func (a *App) GetSettingsSpec() (SettingsSpecView, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	d, live := currentSettingsDescriptor(a.conn)
	return descriptorToSpecView(d, live), nil
}

// GetSettings returns the CONTENT of the working copy's menu/EX settings
// snapshot (task 33's codeplug.MenuSnapshot, task 31's schema-2 field) —
// the values a previous ReadSettingsRadio (or a loaded file) captured, NOT
// the live radio (see ReadSettingsRadio for that). ErrNothingLoaded-style
// refusal when there is no working copy at all (ReadRadio/LoadFile has
// never populated one) — matching GetCodeplug's own convention.
// SettingsView.HasSnapshot is false when the working copy has never had
// its settings read (a.working.Menus == nil): settings acquisition is
// opt-in, so a channels-only working copy is an entirely ordinary state,
// not an error.
func (a *App) GetSettings() (SettingsView, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.working == nil {
		return SettingsView{}, ErrNothingLoaded
	}
	return menuSnapshotToView(a.working.Menus), nil
}

// classifySettingsOutcome maps a ReadSettings error to transfer:done's
// Outcome/Message for Kind "settings" — the settings-read counterpart of
// send.go's classifyReadDiffOutcome, with one addition:
// clone.ErrSettingsUnsupported is a CONTENT-level refusal (the connected
// session's driver exposes no settings surface at all — see
// driver.SettingsReader's doc comment) rather than an operational
// failure, so it maps to "refused" with friendly wording (see
// friendlyErr), never "error".
func classifySettingsOutcome(err error) (outcome, message string) {
	if err == nil {
		return "ok", ""
	}
	if isCancelled(err) {
		return "cancelled", "cancelled"
	}
	if errors.Is(err, clone.ErrSettingsUnsupported) {
		return "refused", friendlyErr(err).Error()
	}
	var busy *clone.BusyError
	if errors.As(err, &busy) {
		return "error", fmt.Sprintf("another operation is running (%s)", busy.InProgress)
	}
	return "error", err.Error()
}

// ReadSettingsRadio performs an OPT-IN acquisition of the connected
// radio's menu/EX settings (clone.Service.ReadSettings, task 33), merging
// the result into the working copy's existing MenuSnapshot rather than
// replacing it outright — see codeplug.MergeMenuSnapshots' doc comment:
// this is what preserves a carried Legacy (v1-migrated) payload, and any
// entry whose ID this build's descriptor no longer recognises, across a
// refresh (MERGE, NEVER ASSIGN). Requires BOTH a connection AND an
// already-loaded working copy (typed refusals: ErrNotConnected /
// ErrNothingLoaded) — this project's GUI flow is read-channels-then-
// settings, so a channels-less codeplug is never a sensible artefact to
// attach settings to.
//
// Reserves the App-level exclusive-operation slot (a.opBusy) for its whole
// duration via reserveOpLocked("ReadSettingsRadio") — the same Fix 2
// reservation ReadRadio/DiffAgainstRadio/PrepareSend already take (see
// reservation.go): refused with a typed *OperationBusyError while any of
// those three (or another ReadSettingsRadio) holds it, and itself refuses
// a concurrently-attempted holder of any of the others. Like ReadRadio/
// DiffAgainstRadio (and unlike PrepareSend/ConfirmSend), it does NOT
// explicitly check a.transfer.running — a ReadSettingsRadio call made
// during a running send instead collides with clone.Service's OWN op
// lock, surfacing a friendlyErr-wrapped *clone.BusyError, exactly as
// ReadRadio/DiffAgainstRadio already do (see reserveOpLocked's doc
// comment).
//
// Before releasing the reservation lock, builds a.settingsDisplay (see
// that field's doc comment, app.go) from the SAME descriptor
// conn.svc.ReadSettings is about to read against, so progressCallback's
// read-settings branch (send.go) never re-scans the descriptor per event;
// cleared again (either outcome) once conn.svc.ReadSettings returns.
//
// On success: a.working.Menus is set to the MERGE result (never simply
// assigned the fresh snapshot), dirty is set true (this DID change the
// working copy), and baseline/baselineStale are deliberately left
// untouched — settings are not part of the channel baseline ReadRadio/
// PrepareSend/Execute reason about. Emits transfer:progress per setting
// (TargetKind "setting", TargetID the 6-digit setting ID, TargetDisplay
// from the descriptor) and exactly one transfer:done (Kind "settings"):
// Outcome "ok" on success, "refused" (with friendly wording) if the
// connected session's driver exposes no settings surface at all
// (clone.ErrSettingsUnsupported — see currentSettingsDescriptor's doc
// comment for why that can only happen with a future, non-FT-710 driver
// today), "cancelled" if ctx was cancelled mid-run, "error" otherwise.
// Report is always nil for a "settings" event — a settings read never
// produces a clone.Report at all (that type is Execute's own).
func (a *App) ReadSettingsRadio() (SettingsView, error) {
	a.mu.Lock()
	conn := a.conn
	if conn == nil {
		a.mu.Unlock()
		return SettingsView{}, ErrNotConnected
	}
	if a.working == nil {
		a.mu.Unlock()
		return SettingsView{}, ErrNothingLoaded
	}
	if err := a.reserveOpLocked("ReadSettingsRadio"); err != nil {
		a.mu.Unlock()
		return SettingsView{}, err
	}
	d, _ := currentSettingsDescriptor(conn)
	a.settingsDisplay = settingsDisplayMap(d)
	a.mu.Unlock()
	defer a.releaseOp()

	fresh, err := conn.svc.ReadSettings(a.ctx)
	if err != nil {
		a.mu.Lock()
		a.settingsDisplay = nil
		a.mu.Unlock()
		outcome, message := classifySettingsOutcome(err)
		a.emitDone("settings", outcome, nil, message)
		return SettingsView{}, fmt.Errorf("app: reading settings: %w", friendlyErr(err))
	}

	a.mu.Lock()
	a.working.Menus = codeplug.MergeMenuSnapshots(a.working.Menus, fresh)
	a.bumpWorkingRevLocked() // Fix 4: working-copy settings merged (the mid-save mutation this counter exists to catch)
	a.dirty = true
	a.settingsDisplay = nil
	view := menuSnapshotToView(a.working.Menus)
	a.mu.Unlock()

	a.emitDone("settings", "ok", nil, "")
	return view, nil
}
