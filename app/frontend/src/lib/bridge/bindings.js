// SPDX-License-Identifier: GPL-3.0-or-later

// The ONLY module that imports wailsjs. Every Go-bound call and every
// runtime event subscription goes through here, wrapped so components
// (and their tests) never touch `../../../wailsjs/*` directly. Every
// wrapper also updates src/lib/state/app.svelte.js's `appState` so
// components can read the result reactively instead of threading return
// values around by hand.
//
// transfer:progress/transfer:done fan-out: `initTransferEvents()` (called
// once, from the app root) subscribes to both events and forwards each
// payload straight to `appState.applyProgress`/`applyTransferDone` — see
// task-15-report.md's "Events — exact contract" section for the payload
// shapes (ProgressEvent{Phase,Done,Total,Slot,TargetKind,TargetID,
// TargetDisplay — the target triple added by task 35/36}, TransferDoneEvent
// {Kind,Outcome,Report,Message}), both PascalCase, no models.ts class
// (Wails only generates one for bound-method-signature types).
//
// `transfer.active` bookkeeping is NOT uniform across every call — this
// is the one subtlety Task 17/18 need to know before adding more actions
// here:
//   - readRadio/diffAgainstRadio/prepareSend/readSettingsRadio: the bound
//     call itself blocks until the whole operation is over, so a plain
//     try/finally around the call is correct: it starts the transfer, and
//     clears `active` the moment the call settles either way. PrepareSend
//     in particular never gets a transfer:done event at all (see
//     task-15-report.md), so its `finally` is the ONLY thing that ever
//     clears it.
//   - confirmSend: ConfirmSend reserves the transfer slot on the Go side
//     and returns almost immediately, handing the real work to a
//     goroutine that finishes later. A `finally` here would wrongly clear
//     `active` the instant the call returns, while the radio is still
//     being written. So confirmSend only clears `active` itself on a
//     SYNCHRONOUS rejection (a pre-flight refusal — nothing was started,
//     no transfer:done is ever coming for that path); on success, only
//     the eventual `transfer:done` (Kind "send") event clears it.
//
// Cancelled dialogs: LoadFile's documented cancel contract is a
// zero-value CodeplugView with a nil error (no `Cancelled` flag exists on
// that type, unlike ImportResultView) — `loadFile()` below treats a
// missing/empty `Channels` as "the dialog was cancelled, nothing
// changed" and deliberately does NOT overwrite `appState.codeplug` with
// that zero value. SaveFileAs/ExportCSV signal cancellation with an empty
// string path; callers should treat a falsy return the same way.

import * as App from '../../../wailsjs/go/main/App.js'
import { EventsOn, WindowSetTitle } from '../../../wailsjs/runtime'
import { appState } from '../state/app.svelte.js'
import { describeError } from '../errorText.js'

/**
 * @param {unknown} err
 * @param {string} context
 */
function reportError(err, context) {
	appState.pushAlert(`${context}: ${describeError(err)}`)
}

let transferEventsInitialised = false

/**
 * Fetches GetUISpec into appState.uiSpec — everything the channel grid
 * needs that only Go knows (bank tabs/writability, slot display strings,
 * edit vocabularies, tag/clarifier limits). Deliberately NEVER throws,
 * unlike every other wrapper here: it runs as an enrichment step inside
 * connect/disconnect/read/load/import (a live session's discovered
 * banks change the spec), and a failed spec refresh must not turn a
 * succeeded primary call into a rejection — the alert strip still
 * carries the message.
 * @returns {Promise<import('../../../wailsjs/go/models').main.UISpecView | null>}
 */
export async function refreshUISpec() {
	try {
		const spec = await App.GetUISpec()
		appState.setUISpec(spec)
		return spec
	} catch (err) {
		reportError(err, 'loading the grid layout')
		return null
	}
}

/**
 * Fetches GetSettingsSpec into appState.settingsSpec — the settings menu/
 * group/item STRUCTURE the settings viewer renders its tablist and tables
 * from (never a value — see getSettings/readSettingsRadio for that).
 * Mirrors refreshUISpec exactly (task 36, M8b-6): works offline, and is
 * deliberately NEVER-throws, since it runs as an enrichment step inside
 * connect/connectDemo/disconnect (Live flips with the connection) and a
 * failed spec refresh must not turn a succeeded primary call into a
 * rejection — the alert strip still carries the message. Unlike
 * refreshUISpec, NOT hooked into readRadio/loadFile/importers — the
 * settings menu STRUCTURE does not depend on which working copy is
 * loaded, only on the connection (see appState.settingsSpec's own doc
 * comment).
 * @returns {Promise<import('../../../wailsjs/go/models').main.SettingsSpecView | null>}
 */
export async function refreshSettingsSpec() {
	try {
		const spec = await App.GetSettingsSpec()
		appState.setSettingsSpec(spec)
		return spec
	} catch (err) {
		reportError(err, 'loading the settings layout')
		return null
	}
}

/**
 * Fetches GetAppVersion into appState.appVersion — which build this is,
 * with the display string composed in Go (see VersionView's doc comment
 * for why the wording is not assembled here). Called once at app start
 * and never again: the version cannot change while the app runs.
 *
 * NEVER throws, for the same reason refreshUISpec does not: it is an
 * enrichment step at startup, and failing to learn the version must not
 * stop the app opening. On failure appState.appVersion stays null and
 * the status bar simply shows no version chip.
 * @returns {Promise<import('../../../wailsjs/go/models').main.VersionView | null>}
 */
export async function refreshAppVersion() {
	try {
		const version = await App.GetAppVersion()
		appState.setAppVersion(version)
		return version
	} catch (err) {
		reportError(err, 'reading the app version')
		return null
	}
}

/** Subscribes to transfer:progress/transfer:done exactly once (idempotent
 * — safe to call from every mount). Call from the app root's $effect. */
export function initTransferEvents() {
	if (transferEventsInitialised) return
	transferEventsInitialised = true
	EventsOn('transfer:progress', (payload) => appState.applyProgress(payload))
	EventsOn('transfer:done', (payload) => appState.applyTransferDone(payload))
}

/** Sets the native window title (task-18: "Open Rig Programmer —
 * <filename>[*]"). Wails' own runtime already exposes WindowSetTitle —
 * no Go binding gap, so this is the one place that call lives, matching
 * bindings.js's role as the only wailsjs importer. The caller (App.svelte)
 * computes the display string reactively from appState; this module only
 * makes the runtime call.
 * @param {string} title */
export function setWindowTitle(title) {
	WindowSetTitle(title)
}

export async function listPorts() {
	appState.setPortsLoading(true)
	try {
		const ports = await App.ListPorts()
		appState.setPorts(ports)
		return ports
	} catch (err) {
		reportError(err, 'listing ports')
		throw err
	} finally {
		appState.setPortsLoading(false)
	}
}

/** M9c-5 (E4): App.Connect/App.ConnectDemo now take a model name, and the
 * empty string means the default model (internal/wiring.DefaultModel) —
 * the exact behaviour these two call sites had before the parameter
 * existed. It is passed EXPLICITLY, with no compat wrapper on the Go
 * side, so the day a model picker lands the only change here is
 * forwarding the user's choice instead of ''. There is no model-selection
 * surface yet: appState carries no chosen model to forward.
 * @param {string} portPath */
export async function connect(portPath) {
	appState.setConnecting(true)
	try {
		const info = await App.Connect(portPath, '')
		appState.setConnection(info)
		await refreshUISpec()
		await refreshSettingsSpec() // task 36: Live flips true now connected
		await revalidateQuiet() // Fix 5: caps just became authoritative
		return info
	} catch (err) {
		reportError(err, 'connecting')
		throw err
	} finally {
		appState.setConnecting(false)
	}
}

export async function connectDemo() {
	appState.setConnecting(true)
	try {
		const info = await App.ConnectDemo('') // see connect(): '' is the default model
		appState.setConnection(info)
		await refreshUISpec()
		await refreshSettingsSpec() // task 36: Live flips true now connected
		await revalidateQuiet() // Fix 5: caps just became authoritative
		return info
	} catch (err) {
		reportError(err, 'starting demo mode')
		throw err
	} finally {
		appState.setConnecting(false)
	}
}

/** Fix 1 (adjudicated HIGH, Codex M6 #1): clears ONLY connection-scoped
 * state (appState.disconnectConnection — see its doc comment) rather
 * than the old clearConnection(), which used to wipe the visible
 * codeplug/dirty flag/issues even though Go's own App.Disconnect leaves
 * the dirty working copy untouched — stranding unsaved work behind a
 * wrongly-disabled Save button. uiSpec is refreshed back to the offline
 * baseline as before (task 36: so is settingsSpec — Live back to false). */
export async function disconnect() {
	try {
		await App.Disconnect()
		appState.disconnectConnection()
		await refreshUISpec() // back to the offline baseline spec
		await refreshSettingsSpec() // task 36: same, for the settings spec
		await revalidateQuiet() // Fix 5: caps just reverted to advisory
	} catch (err) {
		reportError(err, 'disconnecting')
		throw err
	}
}

export async function readRadio() {
	appState.beginTransfer('read')
	try {
		const view = await App.ReadRadio()
		appState.setCodeplug(view)
		await refreshUISpec()
		await refreshSettingsQuiet() // task 36: the new working copy's settings content
		await revalidateQuiet() // Fix 5: CodeplugView carries no Issues
		return view
	} catch (err) {
		reportError(err, 'reading radio')
		throw err
	} finally {
		appState.endTransfer()
	}
}

export async function getCodeplug() {
	try {
		const view = await App.GetCodeplug()
		appState.setCodeplug(view)
		await refreshUISpec()
		return view
	} catch (err) {
		reportError(err, 'loading codeplug')
		throw err
	}
}

/** Fetches GetSettings into appState.settings — the working copy's OWN
 * settings content (never the live radio — see readSettingsRadio for
 * that). Ordinary throw-and-report shape, unlike the internal
 * refreshSettingsQuiet() helper this same module uses to hook readRadio/
 * loadFile/importers (task 36, M8b-6): this export exists for any direct
 * caller that wants to await it and handle its own failure, mirroring
 * getCodeplug's identical role alongside refreshCodeplugQuiet. */
export async function getSettings() {
	try {
		const view = await App.GetSettings()
		appState.setSettings(view)
		return view
	} catch (err) {
		reportError(err, 'loading settings')
		throw err
	}
}

/** @param {import('../../../wailsjs/go/models').codeplug.Channel} channel */
export async function updateChannel(channel) {
	try {
		const result = await App.UpdateChannel(channel)
		appState.applyChannelEdits([channel])
		appState.setIssues(result.Issues)
		appState.setDirty(result.Dirty)
		return result
	} catch (err) {
		reportError(err, 'updating channel')
		throw err
	}
}

/** @param {import('../../../wailsjs/go/models').codeplug.Channel[]} channels */
export async function updateChannels(channels) {
	try {
		const result = await App.UpdateChannels(channels)
		appState.applyChannelEdits(channels)
		appState.setIssues(result.Issues)
		appState.setDirty(result.Dirty)
		return result
	} catch (err) {
		reportError(err, 'updating channels')
		throw err
	}
}

export async function validate() {
	try {
		const result = await App.Validate()
		appState.setIssues(result.Issues)
		appState.setIssuesAdvisory(result.Advisory)
		return result
	} catch (err) {
		reportError(err, 'validating')
		throw err
	}
}

/** Fix 4 (adjudicated MED, Codex M6 #4): refreshes appState.codeplug
 * from Go's own authoritative GetCodeplug — used after a successful
 * import merge, whose ImportResultView carries Issues/Dirty but NOT the
 * merged Channels themselves, so the grid was previously left showing
 * pre-import data until the next unrelated refresh. Deliberately NEVER
 * throws, like refreshUISpec()/revalidateQuiet() — a failed refresh must
 * not turn a succeeded merge into a rejection. */
async function refreshCodeplugQuiet() {
	try {
		const view = await App.GetCodeplug()
		appState.setCodeplug(view)
	} catch (err) {
		reportError(err, 'loading codeplug')
	}
}

/** Task 36 (M8b-6) counterpart to refreshCodeplugQuiet: readRadio/loadFile/
 * a successful import all replace the working copy, but none of their own
 * results carries settings content (GetSettings is the only way to learn
 * it) — so each hooks this afterward to keep appState.settings in step.
 * Deliberately NEVER throws, like refreshCodeplugQuiet/revalidateQuiet — a
 * failed settings-content refresh must not turn a succeeded primary
 * action into a rejection; the alert strip still carries the message. */
async function refreshSettingsQuiet() {
	try {
		const view = await App.GetSettings()
		appState.setSettings(view)
	} catch (err) {
		reportError(err, 'loading settings')
	}
}

/** Fix 5 (adjudicated MED, Codex M6 #5): re-runs authoritative Validate
 * whenever the working copy or the capability source changes — connect/
 * connectDemo, disconnect, LoadFile, a successful import, ReadRadio (see
 * each call site below) — storing Issues AND Advisory together
 * (ValidationView carries both) so the frontend never has to re-derive
 * advisory styling from connection state after the fact. A no-op when
 * nothing is loaded yet (ordinary connect/disconnect with no codeplug
 * open must not surface Validate's own ErrNothingLoaded as a spurious
 * alert). Deliberately NEVER throws, like refreshUISpec() — a failed
 * revalidation must not turn a succeeded primary action into a
 * rejection; the alert strip still carries the message. */
async function revalidateQuiet() {
	if (appState.codeplug === null) return
	try {
		const result = await App.Validate()
		appState.setIssues(result.Issues)
		appState.setIssuesAdvisory(result.Advisory)
	} catch (err) {
		reportError(err, 'validating')
	}
}

export async function diffAgainstRadio() {
	appState.beginTransfer('diff')
	try {
		return await App.DiffAgainstRadio()
	} catch (err) {
		reportError(err, 'comparing with radio')
		throw err
	} finally {
		appState.endTransfer()
	}
}

export async function prepareSend() {
	appState.beginTransfer('prepare')
	try {
		return await App.PrepareSend()
	} catch (err) {
		reportError(err, 'preparing send')
		throw err
	} finally {
		appState.endTransfer()
	}
}

/** Task 36 (M8b-6): opt-in acquisition of the connected radio's settings
 * (app/settings.go's ReadSettingsRadio) — requires both a connection AND
 * an already-loaded working copy (Go's own typed refusals), reserves the
 * App-level exclusive-operation slot for its whole duration. The bound
 * call itself blocks until the whole read is over (like readRadio/
 * diffAgainstRadio/prepareSend above, unlike confirmSend — see this
 * module's doc comment), so a plain try/finally around it is correct:
 * begins the transfer, and clears `active` the moment the call settles
 * either way. Stores the returned SettingsView (the merged working copy)
 * straight into appState.settings — no separate refresh call needed, the
 * same way readRadio's own CodeplugView return needs no follow-up
 * getCodeplug(). */
export async function readSettingsRadio() {
	appState.beginTransfer('settings')
	try {
		const view = await App.ReadSettingsRadio()
		appState.setSettings(view)
		// Fix 1 (adjudicated HIGH, Codex M8b #1): Go's ReadSettingsRadio sets
		// dirty=true (the merged settings ARE an unsaved change), but this
		// wrapper never mirrored it — so the unsaved-changes guards (ActionBar
		// Open/Read, the status bar and title) saw a clean state and could
		// silently discard a freshly-read settings snapshot. Sync the true
		// dirty state back from Go via the existing IsDirty binding.
		appState.setDirty(await App.IsDirty())
		return view
	} catch (err) {
		reportError(err, 'reading settings')
		throw err
	} finally {
		appState.endTransfer()
	}
}

/** See this module's doc comment: does NOT clear `active` on success —
 * only the eventual transfer:done (Kind "send") event does that.
 * @param {string} confirmationDigest
 * @param {string} firmware */
export async function confirmSend(confirmationDigest, firmware) {
	appState.beginTransfer('send')
	try {
		await App.ConfirmSend(confirmationDigest, firmware)
	} catch (err) {
		appState.endTransfer()
		reportError(err, 'sending to radio')
		throw err
	}
}

export async function cancelTransfer() {
	try {
		await App.CancelTransfer()
	} catch (err) {
		reportError(err, 'cancelling transfer')
		throw err
	}
}

export async function isDirty() {
	return App.IsDirty()
}

/** @param {string} path */
export async function saveFile(path) {
	try {
		await App.SaveFile(path)
		// Fix 4 (adjudicated MED, Codex M8b #4): reflect Go's TRUE post-save
		// dirty state rather than forcing false — SaveFile leaves dirty true
		// when a mutation (e.g. a settings read) landed mid-save, so the
		// stale-on-disk/clean-in-memory silent-loss window never opens.
		appState.setDirty(await App.IsDirty())
	} catch (err) {
		reportError(err, 'saving file')
		throw err
	}
}

/** Returns the chosen path, or "" if the user cancelled the save dialog
 * (App.SaveFileAs's documented cancel contract) — `appState.dirty` and
 * `appState.codeplug.WorkingPath` (Fix 4, adjudicated MED, Codex M6 #4:
 * so the title bar reflects the new filename and a subsequent Save does
 * not reopen this same dialogue) are only updated when a path actually
 * came back. */
export async function saveFileAs() {
	try {
		const path = await App.SaveFileAs()
		if (path) {
			// Fix 4 (Codex M8b #4): honour Go's true post-save dirty state
			// (may still be true if a mutation landed mid-save), not a forced
			// false — see saveFile's own comment.
			appState.setDirty(await App.IsDirty())
			appState.setWorkingPath(path)
		}
		return path
	} catch (err) {
		reportError(err, 'saving file')
		throw err
	}
}

/** Returns the loaded codeplug view, or null if the user cancelled the
 * open dialog — see this module's doc comment on LoadFile's zero-value
 * cancel contract. `appState.codeplug` is left untouched on cancel. */
export async function loadFile() {
	try {
		const view = await App.LoadFile()
		if (!view || !view.Channels || view.Channels.length === 0) {
			return null
		}
		appState.setCodeplug(view)
		await refreshUISpec()
		await refreshSettingsQuiet() // task 36: the new working copy's settings content
		await revalidateQuiet() // Fix 5: shows a loaded-invalid-file's issues immediately
		return view
	} catch (err) {
		reportError(err, 'opening file')
		throw err
	}
}

export async function importCSV() {
	try {
		const result = await App.ImportCSV()
		if (result.Merged) {
			await refreshCodeplugQuiet() // Fix 4: the grid must show the merged Channels
			await refreshUISpec()
			await refreshSettingsQuiet() // task 36: the merged working copy's settings content
			await revalidateQuiet() // Fix 5: refreshes Issues+Advisory together
		}
		return result
	} catch (err) {
		reportError(err, 'importing CSV')
		throw err
	}
}

export async function importCHIRP() {
	try {
		const result = await App.ImportCHIRP()
		if (result.Merged) {
			await refreshCodeplugQuiet() // Fix 4: the grid must show the merged Channels
			await refreshUISpec()
			await refreshSettingsQuiet() // task 36: the merged working copy's settings content
			await revalidateQuiet() // Fix 5: refreshes Issues+Advisory together
		}
		return result
	} catch (err) {
		reportError(err, 'importing CHIRP')
		throw err
	}
}

/** Returns the chosen path, or "" if the user cancelled the save dialog. */
export async function exportCSV() {
	try {
		return await App.ExportCSV()
	} catch (err) {
		reportError(err, 'exporting CSV')
		throw err
	}
}
