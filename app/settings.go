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
// Live true; otherwise (disconnected, or a connected session whose driver
// does not implement driver.SettingsReader — EVERY registered model's
// session implements it unconditionally today, the FT-710's, the FTdx10's
// and both FTDX101s', so that branch is currently only
// reachable offline; the claim is deliberately about the registered SET
// rather than about the FT-710 alone, since it is the set that decides
// whether a connected session can reach it) the static baseline of the model currentModel
// resolves, Live false. Callers must hold a.mu (mirrors currentCaps' own
// contract: this reads only its arguments, never a itself).
//
// THE DRIFT IS CLOSED HERE (M9c-5 E4; .superpowers/sdd/HANDOFF-m9c.md's
// precondition 10, "app/ consumer drift"). This fallback used to
// name wiring.DefaultModel literally, which had a consequence the comment
// on it recorded honestly as a defect: because the FT-710's driver
// implements driver.StaticSettingsProvider, that call ALWAYS succeeded, so
// a connected session for some other, future model whose driver does not
// implement driver.SettingsReader silently received the FT-710's OWN
// settings tree — a menu structure belonging to a different radio,
// rendered as if it were this one's. Resolving through currentModel
// instead, both results are now honoured for real:
//   - a non-nil error (model names no registered driver) and
//   - ok == false (a registered model whose driver has no settings
//     surface at all)
//
// each yield the ZERO descriptor with live false — an empty settings tree,
// which the frontend renders as "no settings", rather than another
// radio's. For wiring.DefaultModel the resolved call is byte-identical to
// the old one.
func currentSettingsDescriptor(conn *connectionState, working *codeplug.Codeplug) (d driver.SettingsDescriptor, live bool) {
	if conn != nil {
		if reader, ok := conn.session.(driver.SettingsReader); ok {
			return reader.SettingsDescriptor(), true
		}
	}
	descriptor, ok, err := wiring.StaticSettingsDescriptor(currentModel(conn, working))
	if err != nil || !ok {
		return driver.SettingsDescriptor{}, false
	}
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
	d, live := currentSettingsDescriptor(a.conn, a.working)
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
// (TargetKind "setting", TargetID the four- or six-digit setting ID, TargetDisplay
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
	d, _ := currentSettingsDescriptor(conn, a.working)
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
