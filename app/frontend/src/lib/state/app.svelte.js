// SPDX-License-Identifier: GPL-3.0-or-later

// Runes-based application state — a Svelte 5 ".svelte.js" module using
// $state at class-field scope ("universal reactivity"). One singleton
// instance, `appState`, is exported below; every component reads it
// reactively and every mutation goes through the methods on this class.
//
// This module owns DATA only. It never imports wailsjs and never talks
// to Go — src/lib/bridge/bindings.js is the only thing that calls these
// mutators, in response to a bound-method result or a transfer:progress/
// transfer:done event. Components should treat this file as read-mostly:
// call bindings.js functions to make things happen, read `appState`'s
// fields/getters to render.
//
// Task 17/18 contract notes:
//   - `canSend` composes the brief's four pieces (connected, baseline
//     fresh, no blocking issues, !dirtyTransferConflicts) PLUS "no
//     transfer currently active" (implied by "canSend" — you can't start
//     a second send mid-transfer). Each piece is also exposed on its own
//     (`connected`, `baselineFresh`, `blockingIssues`,
//     `dirtyTransferConflicts`) for a future UI that wants to explain
//     *why* Send is disabled.
//   - `dirtyTransferConflicts` is a placeholder: always false until
//     Task 18 defines the real rule (what counts as a conflict between a
//     local edit and a stale/in-flight transfer). It already participates
//     in `canSend` so nothing downstream needs to change when Task 18
//     starts setting it for real.
//   - `transfer.active` is NOT driven solely by transfer:progress/
//     transfer:done — see bindings.js's module doc comment for why
//     ConfirmSend is a special case (it starts a background transfer and
//     returns immediately; ReadRadio/DiffAgainstRadio/PrepareSend block
//     until their own call settles).

/** @typedef {import('../../../wailsjs/go/models').main.PortEntry} PortEntry */
/** @typedef {import('../../../wailsjs/go/models').main.ConnectionInfo} ConnectionInfo */
/** @typedef {import('../../../wailsjs/go/models').main.CodeplugView} CodeplugView */
/** @typedef {import('../../../wailsjs/go/models').main.IssueView} IssueView */
/** @typedef {import('../../../wailsjs/go/models').main.UISpecView} UISpecView */
/** @typedef {import('../../../wailsjs/go/models').main.SettingsSpecView} SettingsSpecView */
/** @typedef {import('../../../wailsjs/go/models').main.SettingsView} SettingsView */
/** @typedef {import('../../../wailsjs/go/models').main.VersionView} VersionView */
/** @typedef {import('../../../wailsjs/go/models').codeplug.Channel} Channel */

/**
 * transfer:progress's payload (main App's ProgressEvent — no models.ts
 * class exists for it, see task-15-report.md's "Events — exact contract").
 * Task 36 (M8b-6): gains TargetKind/TargetID/TargetDisplay — task 35's
 * progressCallback now populates these for EVERY progress event, channel
 * ones included (TargetDisplay == Slot there); a settings read (Phase
 * "read-settings") is the first event that ever sets TargetKind
 * "setting".
 * @typedef {Object} ProgressPayload
 * @property {string} Phase
 * @property {number} Done
 * @property {number} Total
 * @property {string} Slot
 * @property {string} TargetKind
 * @property {string} TargetID
 * @property {string} TargetDisplay
 */

/**
 * One entry of a ReportView's Slots (main.SlotResultView) — hand-declared
 * like ReportView below, since neither type appears in any bound-method
 * signature, so Wails never generates a models.ts class for either (see
 * task-15-report.md's "Events — exact contract").
 * @typedef {Object} SlotResultView
 * @property {string} Slot
 * @property {string} SlotDisplay
 * @property {string} Action
 * @property {boolean} VerifyOK
 * @property {string} Detail
 */

/**
 * A completed (or aborted) send's report, as attached to transfer:done's
 * Report field — hand-declared, see SlotResultView's doc comment above.
 * @typedef {Object} ReportView
 * @property {string} FirmwareConfirmed
 * @property {number} Written
 * @property {number} Verified
 * @property {number} SkippedBlocked
 * @property {number} Unchanged
 * @property {SlotResultView[]} Slots
 * @property {boolean} Aborted
 * @property {string} AbortReason
 * @property {string} JournalPath
 * @property {string} SnapshotPath
 */

/**
 * transfer:done's payload (main App's TransferDoneEvent — likewise no
 * models.ts class).
 * @typedef {Object} TransferDonePayload
 * @property {string} Kind
 * @property {string} Outcome
 * @property {ReportView | null} [Report]
 * @property {string} Message
 */

/**
 * @typedef {Object} TransferProgress
 * @property {string} phase
 * @property {number} done
 * @property {number} total
 * @property {string} slot
 * @property {string} targetKind
 * @property {string} targetId
 * @property {string} targetDisplay
 */

/**
 * @typedef {Object} TransferState
 * @property {boolean} active
 * @property {string | null} kind
 * @property {TransferProgress} progress
 * @property {TransferDonePayload | null} lastOutcome
 */

/** `kind` distinguishes a genuine rejection (red, AlertStrip's original
 * styling) from an informational toast (task 18: "friendly toast" for
 * NothingToSend, "success toast with path" for Export CSV) — both are
 * dismissible inline strips, never a native alert/confirm/prompt; `kind`
 * only changes the colouring. Defaults to 'error' so every pre-task-18
 * caller (bindings.js's reportError) needs no change.
 * @typedef {{id: number, message: string, kind: 'error' | 'info'}} Alert */

const EMPTY_PROGRESS = Object.freeze({
	phase: '',
	done: 0,
	total: 0,
	slot: '',
	targetKind: '',
	targetId: '',
	targetDisplay: '',
})

let nextAlertId = 1

class AppState {
	/** Current connection, or null when disconnected.
	 * @type {ConnectionInfo | null} */
	connection = $state(null)
	/** True while a Connect/ConnectDemo call is in flight. */
	connecting = $state(false)

	/** Ports as last returned by ListPorts.
	 * @type {PortEntry[]} */
	ports = $state([])
	/** True while a ListPorts call is in flight. */
	portsLoading = $state(false)

	/** Every radio model this build can open a session against, as last
	 * returned by GetSupportedModels (internal/wiring.SupportedModels' own
	 * sorted order — never re-sorted or filtered here). Task 13 (M9d): the
	 * model picker's list. Empty until the first refreshSupportedModels()
	 * resolves, and empty again if that call ever fails — the picker then
	 * offers only its default entry, which still connects (see
	 * `selectedModel`). Not connection-scoped: the set of models a build
	 * supports cannot change while it runs, so neither clearConnection nor
	 * disconnectConnection touches it.
	 * @type {string[]} */
	supportedModels = $state([])

	/** Which radio the user picked in the connection bar, forwarded by
	 * bindings.js's connect()/connectDemo() as their model argument. Task 13
	 * (M9d). The empty-string default is load-bearing: '' means
	 * internal/wiring.DefaultModel to Go's own Connect/ConnectDemo, so an
	 * untouched picker connects EXACTLY as the app did before this field
	 * existed. Any other value must be one of `supportedModels` — Go
	 * refuses an unknown model before opening anything (app/connection.go's
	 * connectModel), and the picker only ever offers what
	 * GetSupportedModels listed, so it cannot produce one.
	 *
	 * The user's own choice, not connection state: deliberately survives
	 * clearConnection/disconnectConnection, so reconnecting after a
	 * disconnect keeps the radio they chose. Carries no capability meaning
	 * whatsoever — the unverified-write consent decision is keyed on the
	 * CONNECTED model, never on this pending choice.
	 * @type {string} */
	selectedModel = $state('')

	/** Current codeplug view, or null before any ReadRadio/LoadFile/
	 * GetCodeplug has succeeded.
	 * @type {CodeplugView | null} */
	codeplug = $state(null)
	/** Mirrors codeplug?.Dirty once a codeplug is loaded; also updated by
	 * edit/import/save calls that return their own Dirty flag. */
	dirty = $state(false)

	/** Most recent validation/edit issues.
	 * @type {IssueView[]} */
	issues = $state([])

	/** Everything the channel grid needs that only Go knows (GetUISpec —
	 * task 17): bank tabs/writability, slot display strings, edit
	 * vocabularies, tag/clarifier limits. Null until the first
	 * refreshUISpec() resolves (bindings.js fetches it at app start and
	 * again after connect/disconnect/read/load/import, since a live
	 * session's discovered banks change it). NOT reset by
	 * clearConnection: an offline spec is still meaningful — the
	 * disconnect flow refreshes it instead.
	 * @type {UISpecView | null} */
	uiSpec = $state(null)

	/** Which build of the app this is (GetAppVersion): `Version` bare,
	 * `Display` ready to render, `IsRelease` false for a build the
	 * release pipeline did not stamp. Fetched once at app start and
	 * never refreshed — it cannot change while the app is running — and
	 * so, unlike every other spec here, NOT touched by connect,
	 * disconnect or clearConnection. Null until that first fetch
	 * resolves; the status bar renders nothing until then rather than
	 * flashing a placeholder version.
	 * @type {VersionView | null} */
	appVersion = $state(null)

	/** Task 36 (M8b-6): which top-level content view is showing —
	 * 'channels' (ChannelGrid) or 'settings' (SettingsViewer). Pure
	 * frontend state: Go is never consulted (there is no bound method for
	 * it), and switching loses no data in either view — both read
	 * straight from this same appState instance. Defaults to 'channels'
	 * so the app opens exactly as it did before this task existed.
	 * @type {'channels' | 'settings'} */
	activeView = $state('channels')

	/** Everything the settings viewer needs that only Go knows
	 * (GetSettingsSpec — task 36): menu/group/item STRUCTURE, never a
	 * value (see `settings` below for that). Works offline like uiSpec;
	 * `Live` is true only once connected. Refreshed by bindings.js after
	 * connect/connectDemo/disconnect ONLY — unlike uiSpec, the settings
	 * menu structure does not depend on which working copy is loaded, so
	 * readRadio/loadFile/importers do not refresh it. NOT reset by
	 * clearConnection, mirroring uiSpec's own "meaningful offline"
	 * rationale — the disconnect flow refreshes it instead.
	 * @type {SettingsSpecView | null} */
	settingsSpec = $state(null)

	/** The working copy's own settings CONTENT (GetSettings — task 36):
	 * the values a previous ReadSettingsRadio (or a loaded file) carried
	 * in, never the live radio (see readSettingsRadio in bindings.js for
	 * that). Refreshed by bindings.js wherever the working copy itself is
	 * replaced (readRadio/loadFile/a successful import) — those bound
	 * methods return no Settings field of their own, so a follow-up
	 * GetSettings call is the only way to learn it. Survives disconnect
	 * (Fix-1 style, mirrors `codeplug`): this is working-copy content, not
	 * connection state, so disconnectConnection() must not strand it.
	 * @type {SettingsView | null} */
	settings = $state(null)

	/** Task 18 placeholder — see module doc comment above. */
	dirtyTransferConflicts = $state(false)

	/** In-flight transfer state, spanning ReadRadio/DiffAgainstRadio/
	 * PrepareSend/ConfirmSend/ReadSettingsRadio. `kind` is one of 'read'|
	 * 'diff'|'prepare'|'send'|'settings'|null — 'prepare' is a
	 * bindings.js-local addition (PrepareSend has no Kind of its own in
	 * transfer:done, since it never emits one); 'settings' mirrors
	 * ReadSettingsRadio's own transfer:done Kind (task 36, M8b-6).
	 * `progress` mirrors the last transfer:progress payload (Phase/Done/
	 * Total/Slot/TargetKind/TargetID/TargetDisplay, lower-cased).
	 * `lastOutcome` is the last transfer:done payload verbatim (Kind/
	 * Outcome/Report/Message), or null.
	 * @type {TransferState} */
	transfer = $state({
		active: false,
		kind: null,
		progress: { ...EMPTY_PROGRESS },
		lastOutcome: null,
	})

	/** Dismissible inline alerts (binding rejections etc). Never a native
	 * alert/confirm/prompt — see AlertStrip.svelte.
	 * @type {Alert[]} */
	alerts = $state([])

	get connected() {
		return this.connection !== null
	}

	get isDemo() {
		return this.connection?.Demo === true
	}

	/** True once a codeplug is loaded AND the App does not consider its
	 * baseline stale (main.CodeplugView.BaselineStale). False (not just
	 * "unknown") before any codeplug has been loaded at all. */
	get baselineFresh() {
		return this.codeplug !== null && this.codeplug.BaselineStale === false
	}

	/** Issues severe enough to block a send (core/codeplug.SeverityError,
	 * serialised as the string "error" — see IssueView.Severity). */
	get blockingIssues() {
		return this.issues.filter((issue) => issue.Severity === 'error')
	}

	/** True when the current `issues` were computed against the static
	 * offline capability baseline rather than a connected session's own.
	 *
	 * Fix 5 (adjudicated MED, Codex M6 #5): STORED alongside `issues`
	 * (from ValidationView.Advisory) whenever a Validate pass actually
	 * runs, rather than inferred from `connected` after the fact — the
	 * old getter (`!this.connected`) flipped to "authoritative" the
	 * INSTANT Connect resolved, even though `issues` itself was still
	 * whatever had last been computed offline (or nothing at all, for
	 * ReadRadio/LoadFile, which never populated issues before this fix).
	 * bindings.js re-runs Validate on every event that changes the
	 * working copy or the capability source (connect, disconnect,
	 * LoadFile, import, ReadRadio) and calls setIssuesAdvisory alongside
	 * setIssues so the two never drift apart. Defaults to true (advisory)
	 * — the same value the old getter gave at boot, before any
	 * connection exists. */
	issuesAdvisory = $state(true)

	get canSend() {
		return (
			this.connected &&
			this.baselineFresh &&
			this.blockingIssues.length === 0 &&
			!this.dirtyTransferConflicts &&
			!this.transfer.active
		)
	}

	/** Plain-language reason canSend is false, checked in the same order
	 * canSend's own pieces are listed above (task 18: "Tooltip explains
	 * the unmet condition otherwise") — '' when canSend is true, so a
	 * caller can use this directly as a conditional tooltip string. Only
	 * one reason is ever shown, even if several pieces are unmet, since a
	 * single sentence reads better than a list on a button tooltip. */
	get sendBlockedReason() {
		if (!this.connected) return 'Connect to a radio first'
		if (this.transfer.active) return 'A transfer is already running'
		if (this.codeplug === null) return 'Read the radio or open a codeplug first'
		if (!this.baselineFresh) return 'The baseline may be stale — read the radio again before sending'
		if (this.blockingIssues.length > 0) {
			const n = this.blockingIssues.length
			return `${n} validation ${n === 1 ? 'error needs' : 'errors need'} to be fixed first`
		}
		if (this.dirtyTransferConflicts) return 'Resolve the conflicting local changes first'
		return ''
	}

	/** Task 36 (M8b-6): the settings viewer's own Read button gate —
	 * ReadSettingsRadio (app/settings.go) requires both a connection AND
	 * an already-loaded working copy, and (like every other radio
	 * transfer) refuses while another transfer is running. Deliberately
	 * simpler than canSend: no baseline-freshness or blocking-issue
	 * pieces, since those are channel-baseline concepts settings play no
	 * part in. */
	get canReadSettings() {
		return this.connected && this.codeplug !== null && !this.transfer.active
	}

	/** Plain-language reason canReadSettings is false — mirrors
	 * sendBlockedReason's single-reason style (see its doc comment): '' when
	 * canReadSettings is true, checked in the same order canReadSettings'
	 * own pieces are listed above. */
	get readSettingsBlockedReason() {
		if (!this.connected) return 'Connect to a radio first'
		if (this.transfer.active) return 'A transfer is already running'
		if (this.codeplug === null) return 'Read the radio or open a codeplug first'
		return ''
	}

	// --- mutators — called from bindings.js (and, for dirtyTransferConflicts,
	// eventually Task 18's edit-tracking code) ---

	/** @param {boolean} loading */
	setPortsLoading(loading) {
		this.portsLoading = loading
	}

	/** @param {PortEntry[]} ports */
	setPorts(ports) {
		this.ports = ports ?? []
	}

	/** @param {boolean} connecting */
	setConnecting(connecting) {
		this.connecting = connecting
	}

	/** Task 13 (M9d) — the model picker's list, exactly as
	 * GetSupportedModels returned it. Null/undefined becomes an empty
	 * array (mirrors setPorts): the picker always iterates an array.
	 * @param {string[] | null} models */
	setSupportedModels(models) {
		this.supportedModels = models ?? []
	}

	/** Task 13 (M9d) — the user's chosen radio; '' means the default model
	 * (see `selectedModel`'s own doc comment). A null/undefined choice
	 * (a `<select>` with no value) becomes '' rather than being stored:
	 * the connect path takes a string.
	 * @param {string | null} model */
	setSelectedModel(model) {
		this.selectedModel = model ?? ''
	}

	/** @param {ConnectionInfo | null} info */
	setConnection(info) {
		this.connection = info
	}

	/** Resets everything scoped to one connection: connection info, the
	 * loaded codeplug, dirty/issues, and any in-flight transfer bookkeeping
	 * (Disconnect refuses while a transfer is genuinely running, so this
	 * is a clean reset, not a mid-transfer interruption).
	 *
	 * NOT used by Disconnect any more (see disconnectConnection below,
	 * Fix 1, Codex M6 #1, adjudicated HIGH) — kept as the general
	 * full-state-reset utility every test file's `beforeEach` already
	 * relies on it for.
	 *
	 * Task 36 (M8b-6): `settings` is nulled alongside `codeplug` — it is
	 * the SAME working copy's content, so a full reset must clear both
	 * together. `settingsSpec` is deliberately left alone, mirroring
	 * `uiSpec`'s own "meaningful offline, not connection-scoped" treatment
	 * (see that field's doc comment) — test files that need it null call
	 * setSettingsSpec(null) themselves, exactly as they already do for
	 * uiSpec. `activeView` is untouched too: it is not connection- or
	 * working-copy-scoped at all.
	 *
	 * Task 13 (M9d): neither is the model picker's state —
	 * `supportedModels` is a property of the BUILD, and `selectedModel` is
	 * the user's own pending choice, which must survive so a reconnect
	 * offers the radio they picked. Test files that want either reset call
	 * its setter themselves, exactly as they already do for uiSpec. */
	clearConnection() {
		this.connection = null
		this.codeplug = null
		this.settings = null
		this.dirty = false
		this.issues = []
		this.issuesAdvisory = true
		this.dirtyTransferConflicts = false
		this.transfer = { active: false, kind: null, progress: { ...EMPTY_PROGRESS }, lastOutcome: null }
	}

	/** Disconnect's OWN state clearing (Fix 1, adjudicated HIGH, Codex M6
	 * #1: the old clearConnection() above wiped the visible codeplug,
	 * dirty flag and issues on every disconnect, while Go's own
	 * App.Disconnect keeps the dirty working copy untouched — Save then
	 * appeared wrongly disabled with the edit stranded, invisible, in
	 * Go's memory). Resets ONLY connection-scoped state: connection info,
	 * the baseline-fresh claim (no longer assertable once disconnected
	 * from the radio that would need re-reading to refresh it — mirrors
	 * applyTransferDone's identical BaselineStale=true mutation after a
	 * send), and any in-flight transfer bookkeeping (Disconnect refuses
	 * while a transfer is genuinely running, so this is a clean reset,
	 * not a mid-transfer interruption).
	 *
	 * Deliberately does NOT touch codeplug/dirty/issues/workingPath.
	 * issues/issuesAdvisory are refreshed separately, to ADVISORY, by
	 * Fix 5's revalidate() (bindings.js's disconnect(), called
	 * immediately after this).
	 *
	 * Task 36 (M8b-6): for the same reason, does NOT touch `settings`
	 * either — its CONTENT survives a disconnect exactly like `codeplug`
	 * does (it is that same working copy's data). `settingsSpec` is also
	 * left alone here — mirrors `uiSpec`'s own treatment: bindings.js's
	 * disconnect() calls refreshSettingsSpec() immediately after this, the
	 * same way it already calls refreshUISpec(), to bring it back to the
	 * offline (`Live` false) baseline. */
	disconnectConnection() {
		this.connection = null
		if (this.codeplug !== null) this.codeplug.BaselineStale = true
		this.dirtyTransferConflicts = false
		this.transfer = { active: false, kind: null, progress: { ...EMPTY_PROGRESS }, lastOutcome: null }
	}

	/** @param {CodeplugView | null} view */
	setCodeplug(view) {
		this.codeplug = view
		this.dirty = view?.Dirty ?? false
	}

	/** @param {IssueView[]} issues */
	setIssues(issues) {
		this.issues = issues ?? []
	}

	/** @param {boolean} advisory */
	setIssuesAdvisory(advisory) {
		this.issuesAdvisory = advisory
	}

	/** @param {UISpecView | null} spec */
	setUISpec(spec) {
		this.uiSpec = spec
	}

	/** @param {VersionView | null} version */
	setAppVersion(version) {
		this.appVersion = version
	}

	/** Task 36 (M8b-6) — the Channels|Settings view switch. Pure frontend
	 * state; see `activeView`'s own doc comment.
	 * @param {'channels' | 'settings'} view */
	setActiveView(view) {
		this.activeView = view
	}

	/** @param {SettingsSpecView | null} spec */
	setSettingsSpec(spec) {
		this.settingsSpec = spec
	}

	/** @param {SettingsView | null} settings */
	setSettings(settings) {
		this.settings = settings
	}

	/** Mirrors a successful UpdateChannel/UpdateChannels into the local
	 * codeplug view: Go's applyEditsLocked assigns each sent channel
	 * WHOLESALE into the working copy (and refuses the whole batch
	 * otherwise), so replacing the same slots with the same values here
	 * is exactly faithful — no re-fetch round trip needed. Slots are
	 * only ever replaced, never added or removed.
	 * @param {Channel[]} channels */
	applyChannelEdits(channels) {
		if (this.codeplug === null) return
		const bySlot = new Map(channels.map((ch) => [ch.slot, ch]))
		this.codeplug.Channels = this.codeplug.Channels.map((ch) => bySlot.get(ch.slot) ?? ch)
	}

	/** @param {boolean} dirty */
	setDirty(dirty) {
		this.dirty = dirty
	}

	/** Updates the loaded codeplug's WorkingPath (Fix 4, adjudicated MED,
	 * Codex M6 #4) — used after a successful Save As, whose own return
	 * value is just the chosen path; without this, appState.codeplug.
	 * WorkingPath stayed stale, so the title bar kept showing "Untitled"
	 * (App.svelte derives the title from it) and a subsequent plain Save
	 * wrongly reopened the Save As dialogue (ActionBar's handleSave
	 * branches on WorkingPath being non-empty). A no-op if nothing is
	 * loaded (should not happen — SaveFileAs itself requires a working
	 * copy — but mirrors applyChannelEdits' same defensive null check).
	 * @param {string} path */
	setWorkingPath(path) {
		if (this.codeplug !== null) this.codeplug.WorkingPath = path
	}

	/** Marks a transfer as starting. `kind` is 'read'|'diff'|'prepare'|
	 * 'send'. Resets progress to zero so a stale reading from a previous
	 * transfer never flashes before the first real progress event. Also
	 * clears `lastOutcome` — a new operation supersedes whatever the
	 * previous one left behind, so a consumer keyed on lastOutcome (e.g.
	 * SendFlowDialog's transferring -> result transition) can never mistake
	 * a stale outcome for this transfer's own (task-18 review: a second
	 * send in the same session must not flash the previous send's result
	 * before this one has actually finished). StatusBar/other lastOutcome
	 * readers already tolerate null (that's the state before the very first
	 * transfer of a session completes).
	 * @param {string} kind */
	beginTransfer(kind) {
		this.transfer.active = true
		this.transfer.kind = kind
		this.transfer.progress = { ...EMPTY_PROGRESS }
		this.transfer.lastOutcome = null
	}

	/** Clears the active flag without touching kind/lastOutcome — used
	 * when a call settles with no transfer:done coming (ReadRadio/
	 * DiffAgainstRadio/PrepareSend on completion; ConfirmSend only on a
	 * synchronous pre-flight rejection, since a successful ConfirmSend
	 * call hands off to the eventual transfer:done event instead). */
	endTransfer() {
		this.transfer.active = false
	}

	/** transfer:progress payload handler (Phase/Done/Total/Slot/TargetKind/
	 * TargetID/TargetDisplay, PascalCase as emitted — see task-15-report.md's
	 * ProgressEvent shape, extended by task 35/36 with the target triple:
	 * every progress event carries it now, channel ones included
	 * (TargetDisplay == Slot there) — see ProgressPayload's own doc
	 * comment above.
	 * @param {ProgressPayload} payload */
	applyProgress(payload) {
		this.transfer.active = true
		this.transfer.progress = {
			phase: payload?.Phase ?? '',
			done: payload?.Done ?? 0,
			total: payload?.Total ?? 0,
			slot: payload?.Slot ?? '',
			targetKind: payload?.TargetKind ?? '',
			targetId: payload?.TargetID ?? '',
			targetDisplay: payload?.TargetDisplay ?? '',
		}
	}

	/** transfer:done payload handler (Kind/Outcome/Report/Message). A
	 * "send" transfer that actually touched the radio (Report non-null —
	 * the same nil-Report rule PrepareSend/Execute use for "nothing was
	 * sent") marks the loaded codeplug's baseline stale, mirroring the Go
	 * side's own a.baselineStale=true (app/send.go's ConfirmSend
	 * goroutine) — Go's flag lives on a struct field the frontend never
	 * re-fetches on its own, so canSend/baselineFresh would otherwise
	 * stay wrongly "fresh" until the next GetCodeplug/ReadRadio. Read
	 * transfers never touch this (a fresh ReadRadio sets its own
	 * BaselineStale via the CodeplugView it returns).
	 * @param {TransferDonePayload} payload */
	applyTransferDone(payload) {
		this.transfer.active = false
		this.transfer.kind = payload?.Kind ?? this.transfer.kind
		this.transfer.lastOutcome = payload ?? null
		if (payload?.Kind === 'send' && payload.Report && this.codeplug !== null) {
			this.codeplug.BaselineStale = true
		}
	}

	/** Pushes a dismissible alert (see AlertStrip.svelte) and returns its
	 * id, so a caller could dismiss it programmatically if ever needed.
	 * @param {string} message
	 * @param {'error' | 'info'} [kind]
	 * @returns {number} */
	pushAlert(message, kind = 'error') {
		const id = nextAlertId++
		this.alerts = [...this.alerts, { id, message, kind }]
		return id
	}

	/** @param {number} id */
	dismissAlert(id) {
		this.alerts = this.alerts.filter((alert) => alert.id !== id)
	}
}

export const appState = new AppState()
