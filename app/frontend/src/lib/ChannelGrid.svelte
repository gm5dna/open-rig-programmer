<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
<script>
	// The channel grid (task 17) — a hand-rolled, keyboard-first editable
	// table over the working codeplug. Replaces GridPlaceholder.
	//
	// Data flow: bank tabs, per-slot display strings, read-only locking
	// and every edit vocabulary come from appState.uiSpec (GetUISpec —
	// nothing protocol-specific is hardcoded here); channel data comes
	// from appState.codeplug; issue decoration from appState.issues. All
	// edits go through the bridge (updateChannel/updateChannels), whose
	// EditResult refreshes issues/dirty and mirrors the edit back into
	// the codeplug view — this component never mutates state directly.
	//
	// Public component API (for Task 18): no props, no exports — the
	// grid is wholly driven by appState and the bridge. Mount it, keep
	// uiSpec/codeplug populated, and it renders; that is the entire
	// contract.
	import { tick } from 'svelte'
	import { appState } from './state/app.svelte.js'
	import { updateChannel, updateChannels } from './bridge/bindings.js'
	import {
		columnsFor,
		isCellEditable,
		displayValue,
		newChannelData,
		cloneData,
		parsePasteCell,
		tierColumnFor,
	} from './grid/columns.js'
	import { hzToMHz, mhzToHz } from './grid/freq.js'
	import { initialFocus, moveFocus, clampFocus } from './grid/nav.js'
	import { parseBlock, mapPasteToChannels } from './grid/paste.js'
	import { createEditQueue } from './grid/editQueue.js'
	import {
		initialDragState,
		beginDrag,
		hoverDrag,
		leaveDrag,
		dropDrag,
		cancelDrag,
		buildDragOpChannels,
		resolveNeighbourSlot,
	} from './grid/dragDrop.js'
	import DeleteConfirmDialog from './DeleteConfirmDialog.svelte'
	import MoveCopyDialog from './MoveCopyDialog.svelte'
	import RowMoveConfirmDialog from './RowMoveConfirmDialog.svelte'

	/** @typedef {import('../../wailsjs/go/models').codeplug.Channel} Channel */
	/** @typedef {import('../../wailsjs/go/models').codeplug.ChannelData} ChannelData */
	/** @typedef {import('../../wailsjs/go/models').main.BankView} BankView */
	/** @typedef {import('../../wailsjs/go/models').main.IssueView} IssueView */
	/** The models.ts classes carry a convertValues helper method; the
	 * objects this component builds (and everything Wails actually
	 * transports) are plain JSON, so edits are typed structurally.
	 * @typedef {Omit<ChannelData, 'convertValues'>} ChannelDataShape */

	const READ_ONLY_TOOLTIP = 'read-only over CAT — the radio rejects writes to this bank'
	const ADVISORY_NOTE = 'advisory — validated offline against the static capability baseline'

	/** The preserved-cell tooltip for a column ('tone' or 'skip' — the only
	 * two cellPreserved can flag). Task 42 (M9a-6): served from GetUISpec's
	 * PreservationTooltips (internal/radiotext.Text.PreservationTooltips —
	 * see that package's doc comment for the M5b write-trial provenance of
	 * the exact wording: tone preservation across a rewrite is hardware-
	 * verified, scan-skip preservation was never probed) rather than
	 * hardcoded here. Empty string (spec not yet loaded, or a future model
	 * with no radiotext entry) renders no tooltip — no hardcoded fallback
	 * prose is reintroduced. @param {{id: string}} column */
	function preservedTooltip(column) {
		const tooltips = appState.uiSpec?.PreservationTooltips
		return column.id === 'tone' ? (tooltips?.Tone ?? '') : (tooltips?.ScanSkip ?? '')
	}

	const encoder = new TextEncoder()

	// --- banks and rows -------------------------------------------------

	const banks = $derived(appState.uiSpec?.Banks ?? [])
	/** @type {string | null} */
	let activeBankId = $state(null)
	const activeBank = $derived(banks.find((b) => b.ID === activeBankId) ?? banks[0] ?? null)
	const bankLocked = $derived(activeBank?.ReadOnly === true)
	const slots = $derived(activeBank?.Slots ?? [])
	// The columns THIS bank renders: always the ten in COLUMNS, plus any
	// field the Icom model extensions added that this bank's capabilities say the
	// radio actually has (columnsFor / BankView.Fields). For every model
	// registered today, D8's seven remain hidden because their bank field
	// lists do not name them.
	const columns = $derived(columnsFor(activeBank))

	/** @type {Map<string, Channel>} */
	const channelBySlot = $derived(
		new Map((appState.codeplug?.Channels ?? []).map((ch) => [ch.slot, ch]))
	)

	/** @param {number} rowIdx @returns {ChannelData | null} */
	function dataAt(rowIdx) {
		const sv = slots[rowIdx]
		if (!sv) return null
		return channelBySlot.get(sv.Slot)?.data ?? null
	}

	// --- issue decoration -------------------------------------------------

	/** @type {Map<string, IssueView[]>} */
	const issuesBySlot = $derived.by(() => {
		/** @type {Map<string, IssueView[]>} */
		const m = new Map()
		for (const issue of appState.issues) {
			const list = m.get(issue.Slot)
			if (list) list.push(issue)
			else m.set(issue.Slot, [issue])
		}
		return m
	})

	/** @param {BankView} bank */
	function bankIssueCount(bank) {
		let n = 0
		for (const sv of bank.Slots ?? []) n += (issuesBySlot.get(sv.Slot) ?? []).length
		return n
	}

	/** @param {BankView} bank */
	function bankHasError(bank) {
		return (bank.Slots ?? []).some((sv) =>
			(issuesBySlot.get(sv.Slot) ?? []).some((i) => i.Severity === 'error')
		)
	}

	/** Issues decorating one cell (slot + the column's field key).
	 * @param {string} slot @param {ReturnType<typeof columnsFor>[number]} column */
	function cellIssues(slot, column) {
		if (!column.field) return []
		return (issuesBySlot.get(slot) ?? []).filter((i) => i.Field === column.field)
	}

	/** 'error' | 'warn' | null. Advisory-mode (disconnected) issues always
	 * render amber: offline validation is advisory, not authoritative.
	 * @param {IssueView[]} issues */
	function severityClass(issues) {
		if (issues.length === 0) return null
		if (!appState.issuesAdvisory && issues.some((i) => i.Severity === 'error')) return 'error'
		return 'warn'
	}

	/** @param {IssueView[]} issues */
	function issueTitle(issues) {
		if (issues.length === 0) return null
		const msgs = issues.map((i) => i.Msg).join('\n')
		return appState.issuesAdvisory ? `${msgs}\n(${ADVISORY_NOTE})` : msgs
	}

	// --- focus ------------------------------------------------------------

	let focus = $state(initialFocus())
	const dims = $derived({ rows: slots.length, cols: columns.length })

	// Keep the remembered position renderable when the active bank or the
	// loaded data shrinks. Settles immediately: once in range, the clamp
	// returns the same coordinates and nothing is written.
	$effect(() => {
		const clamped = clampFocus(focus, dims)
		if (clamped.row !== focus.row || clamped.col !== focus.col) focus = clamped
	})

	/** @type {HTMLTableElement | undefined} */
	let tableEl = $state(undefined)

	/** @param {{row: number, col: number}} pos */
	async function focusCell(pos) {
		focus = pos
		await tick()
		const cell = /** @type {HTMLElement | null} */ (
			tableEl?.querySelector(`td[data-row="${pos.row}"][data-col="${pos.col}"]`) ?? null
		)
		cell?.focus()
	}

	/** @param {number} rowIdx @param {number} colIdx */
	function setFocusPos(rowIdx, colIdx) {
		if (focus.row !== rowIdx || focus.col !== colIdx) focus = { row: rowIdx, col: colIdx }
	}

	// --- bank tabs (complete tablist pattern — carried Task 16 review
	// obligation: aria-controls + arrow-key navigation, roving tabindex) ---

	/** @param {string} id */
	function selectBank(id) {
		// Guards on the DERIVED active bank, not the raw `activeBankId`
		// field: before any explicit selection, activeBankId is still
		// null while activeBank already falls back to banks[0] (so the
		// first tab renders pre-highlighted). Guarding on activeBankId
		// alone missed that case — clicking the already-active first tab
		// passed the check (null !== banks[0].ID) and reset focus back to
		// 0,0, discarding wherever the user had navigated to within that
		// bank (task-17 review finding).
		if (activeBank?.ID === id) return
		activeBankId = id
		editing = null
		focus = initialFocus() // bank-tab reset (nav.js contract)
	}

	/** @param {KeyboardEvent} e @param {number} index */
	function onTabKeydown(e, index) {
		let target = null
		if (e.key === 'ArrowRight') target = (index + 1) % banks.length
		else if (e.key === 'ArrowLeft') target = (index - 1 + banks.length) % banks.length
		else if (e.key === 'Home') target = 0
		else if (e.key === 'End') target = banks.length - 1
		if (target === null || banks.length === 0) return
		e.preventDefault()
		selectBank(banks[target].ID)
		document.getElementById(`bank-tab-${banks[target].ID}`)?.focus()
	}

	// --- editing ----------------------------------------------------------

	/** @type {{row: number, col: number, seed: string | null} | null} */
	let editing = $state(null)
	let textDraft = $state('')
	let clarDraft = $state({ hz: '0', rx: false, tx: false })

	const tagBytes = $derived(encoder.encode(textDraft).length)
	const tagMaxBytes = $derived(appState.uiSpec?.TagMaxBytes ?? 0)

	/** Svelte action: focus a freshly-rendered editor. Seeded editors get
	 * the caret at the end (the seed is the first typed character);
	 * Enter-opened editors select-all so typing replaces.
	 * @param {HTMLInputElement | HTMLSelectElement} node
	 * @param {{selectAll: boolean}} opts */
	function autofocus(node, opts) {
		node.focus()
		if (node instanceof HTMLInputElement && node.type === 'text') {
			try {
				if (opts.selectAll) node.select()
				else node.setSelectionRange(node.value.length, node.value.length)
			} catch {
				// happy-dom / non-text inputs: focus alone is enough
			}
		}
	}

	// --- tier columns: which editor each KIND opens -----------------------
	//
	// The seventeen tier-added columns (grid/columns.js's TIER_COLUMNS) had
	// no editor at all until this task: a double-click opened the select
	// below with no options in it and cancelled on blur, so paste and CSV
	// import were the only routes to answering such a cell. The editor is
	// chosen by the column's KIND, and each kind reuses a path that already
	// existed rather than growing a new one:
	//   freq / int / text → the free-text editor, committed through the
	//     PASTE parser (grid/columns.js's parsePasteCell) so an edit and a
	//     paste of the same text produce the identical field object.
	//   tone → the same UISpec-served tone list the CTCSS Tone column uses.
	//   bool → the same one-keystroke toggle Tag display uses.
	//
	// NO SELECT FOR THE TEXT KINDS (duplex, tone_mode, dtcs_polarity,
	// filter, tuning_step, preamp, antenna), deliberately, and the reason
	// is a GAP rather than an absence: each of the seven has a declared
	// per-radio vocabulary in core/spec/capabilities.go (DuplexOptions,
	// ToneModes, DTCSPolarities, Filters, TuningSteps, PreampOptions,
	// AntennaOptions), but GetUISpec serves NONE of them to the frontend —
	// app/types.go's UISpecView carries only Modes, ShiftOptions,
	// CTCSSStateOptions and Tones. So a pick-list here could only offer a
	// vocabulary this file invented, which is exactly what columns.js's own
	// header forbids. Free text passes the entry to Go unjudged, precisely
	// as a paste of the same string does, and Go's Validate answers it.
	// Serving those seven lists is the change that would earn a select
	// here; until one is served, this stays free text.

	/** The TierColumn whose editor is the free-text one — the freq, int and
	 * text kinds. null for a non-tier column, and for the tone and bool
	 * kinds, which have their own paths above.
	 * @param {ReturnType<typeof columnsFor>[number]} column */
	function tierTextColumn(column) {
		const tier = tierColumnFor(column)
		if (!tier || tier.kind === 'tone' || tier.kind === 'bool') return null
		return tier
	}

	/** Columns a single keystroke TOGGLES rather than opening an editor
	 * for: Tag display, Scan skip and every tier column of the bool kind.
	 * @param {ReturnType<typeof columnsFor>[number]} column */
	function isToggleColumn(column) {
		return column.id === 'tagDisplay' || column.id === 'skip' || tierColumnFor(column)?.kind === 'bool'
	}

	/** The {state, value} object a tier column edits, or undefined.
	 * @param {{key: string}} tier @param {ChannelData | null} data */
	function tierField(tier, data) {
		return data ? /** @type {Record<string, any>} */ (data)[tier.key] : undefined
	}

	/** The text a tier editor OPENS with: the value the cell already holds,
	 * in the same spelling parsePasteCell reads back, so committing an
	 * untouched cell is the no-op the unchanged-commit skip expects. A cell
	 * with no Known value opens EMPTY — never the em dash displayValue
	 * renders for it, which is a "no claim made" mark and not a value
	 * anyone could edit.
	 *
	 * A KNOWN field with no `value` key is Known ZERO, and opens on zero:
	 * FieldState.Value is `json:"value,omitempty"`
	 * (core/codeplug/fieldstate.go), so Go drops the key for every zero it
	 * sends, and an attenuator answered 0 dB or an offset answered 0 Hz
	 * arrives as {"state":"known"} alone. Opening such a cell EMPTY would
	 * make it unre-committable, an emptied editor being the no-op below —
	 * pinned by the two "a Known ZERO opens the … editor on zero" tests, and
	 * displayValue normalises the same way.
	 * @param {{key: string, kind: string}} tier @param {ChannelData | null} data */
	function tierEditText(tier, data) {
		const f = tierField(tier, data)
		if (f?.state !== 'known') return ''
		if (tier.kind === 'freq') return hzToMHz(typeof f.value === 'number' ? f.value : 0)
		if (tier.kind === 'int') return String(typeof f.value === 'number' ? f.value : 0)
		return f.value == null ? '' : String(f.value)
	}

	/** @param {number} rowIdx @param {number} colIdx @param {string | null} seed */
	function openEditor(rowIdx, colIdx, seed = null) {
		if (bankLocked || !appState.uiSpec) return
		const column = columns[colIdx]
		const data = dataAt(rowIdx)
		if (!isCellEditable(column, data)) return
		if (isToggleColumn(column)) {
			toggleCell(rowIdx, column)
			return
		}
		const tierText = tierTextColumn(column)
		if (tierText) textDraft = seed ?? tierEditText(tierText, data)
		else if (column.id === 'freq') textDraft = seed ?? (data ? hzToMHz(data.freq_hz) : '')
		else if (column.id === 'tag') textDraft = seed ?? data?.tag ?? ''
		else if (column.id === 'clar') {
			clarDraft = { hz: seed ?? String(data?.clar_hz ?? 0), rx: data?.rx_clar ?? false, tx: data?.tx_clar ?? false }
		}
		editing = { row: rowIdx, col: colIdx, seed }
	}

	function cancelEditor() {
		if (!editing) return
		editing = null
		void focusCell(focus)
	}

	/** The FIFO edit queue (Fix 3, adjudicated MED, Codex M6 #3): grid
	 * commits used to be fire-and-forget with a WHOLE ChannelData built
	 * eagerly at commit time, so two rapid edits to different fields of
	 * one slot could both be based on the same pre-edit data — whichever
	 * Go call's response settled last silently discarded the other's
	 * field. The queue instead stores each edit as a (slot, transform)
	 * pair and materialises the outgoing channel from the FRESHEST
	 * appState codeplug only once it is actually dequeued, awaiting each
	 * updateChannel call before starting the next. getData reads
	 * channelBySlot directly (not a snapshot) so every dequeue sees
	 * whatever the previous, already-awaited edit just committed. */
	const editQueue = createEditQueue({
		getData: (slot) => channelBySlot.get(slot)?.data ?? null,
		commit: (channel) => updateChannel(/** @type {Channel} */ (/** @type {unknown} */ (channel))),
		onError: () => {
			// Alert strip already carries the message (bindings.js's
			// updateChannel already reports+rethrows on rejection) — the
			// queue itself just keeps draining whatever else is queued.
		},
	})

	/** Queues one edit. apply receives the slot's FRESHEST ChannelData
	 * (read only once this edit is actually dequeued — see editQueue.js)
	 * and must return the FULL next ChannelData to send, or `null` for a
	 * delete (task 22: the working copy's Channel becomes {Slot, Data:
	 * null} — the same file-level "erase" every UpdateChannel caller
	 * already accepts structurally, see app/codeplug.go's applyEditsLocked).
	 * Still "fire-and-forget" from the caller's own point of view — the
	 * bridge already alerts on rejection and refreshes issues/dirty/
	 * codeplug on success — only the queue changes WHAT gets materialised
	 * and WHEN.
	 * @param {string} slot @param {(data: ChannelData | null) => ChannelDataShape | null} apply */
	function submitEdit(slot, apply) {
		editing = null
		void focusCell(focus)
		editQueue.enqueue(slot, apply)
	}

	/** Commit the text editor (Frequency or Tag). @param {number} rowIdx @param {number} colIdx */
	function commitTextEditor(rowIdx, colIdx) {
		if (!editing) return
		const column = columns[colIdx]
		const sv = slots[rowIdx]
		const data = dataAt(rowIdx)
		const spec = appState.uiSpec
		if (!sv || !spec) return cancelEditor()

		if (column.id === 'freq') {
			const text = textDraft.trim()
			if (text === '') return cancelEditor() // erase is out of scope — no-op
			const hz = mhzToHz(text)
			if (hz === null) {
				appState.pushAlert(`"${text}" is not a frequency in MHz — the edit was not applied`)
				return cancelEditor()
			}
			if (data && hz === data.freq_hz) return cancelEditor() // unchanged
			// The Added row's tag-display default is the ACTIVE BANK's
			// (BankView.TagDisplayDefault — M9c-5 review W1), so the bank goes
			// with the spec: sv came from activeBank's own Slots, so it is
			// non-null here.
			const bank = /** @type {BankView} */ (activeBank)
			submitEdit(sv.Slot, (fresh) => (fresh ? { ...cloneData(fresh), freq_hz: hz } : newChannelData(spec, bank, hz)))
			return
		}
		// Tag: exact text (unclamped — Go validates bytes/charset).
		if (!data) return cancelEditor()
		if (textDraft === (data.tag ?? '')) return cancelEditor()
		submitEdit(sv.Slot, (fresh) => ({ ...cloneData(fresh ?? data), tag: textDraft }))
	}

	/** Commit a tier column's free-text editor (the freq, int and text
	 * kinds) THROUGH THE PASTE PARSER: parsePasteCell is the one place a
	 * typed string becomes a tier field, so an edit and a paste of the same
	 * text cannot diverge — pinned by "a text-kind cell is FREE TEXT",
	 * which compares this commit's field against parsePasteCell's own
	 * answer for the same string.
	 *
	 * An unreadable entry is REFUSED exactly as the Frequency editor
	 * refuses one: the parser's own reason as an alert, and no commit — the
	 * cell keeps whatever state it had. An emptied editor is the same
	 * no-op the Frequency editor makes of one; there is no route back to
	 * unanswered here, and inventing an "erase" for a tier field is out of
	 * scope for this task.
	 * @param {number} rowIdx @param {number} colIdx */
	function commitTierTextEditor(rowIdx, colIdx) {
		if (!editing) return
		const column = columns[colIdx]
		const tier = tierTextColumn(column)
		const sv = slots[rowIdx]
		const data = dataAt(rowIdx)
		const spec = appState.uiSpec
		if (!sv || !data || !tier || !spec) return cancelEditor()

		const text = textDraft.trim()
		if (text === '') return cancelEditor()
		const parsed = parsePasteCell(column, text, spec)
		if (!parsed.ok) {
			appState.pushAlert(`${parsed.reason} — the edit was not applied`)
			return cancelEditor()
		}
		const next = /** @type {Record<string, any>} */ (parsed.patch)[tier.key]
		const current = tierField(tier, data)
		if (current?.state === 'known' && current.value === next.value) return cancelEditor() // unchanged
		submitEdit(sv.Slot, (fresh) => ({ ...cloneData(fresh ?? data), [tier.key]: next }))
	}

	/** Commit a select editor (Mode/Shift/CTCSS/Tone, and a tier column of
	 * the tone kind).
	 * @param {number} rowIdx @param {number} colIdx @param {string} value */
	function commitSelectEditor(rowIdx, colIdx, value) {
		if (!editing) return
		const column = columns[colIdx]
		const sv = slots[rowIdx]
		const data = dataAt(rowIdx)
		if (!sv || !data) return cancelEditor()
		const tier = tierColumnFor(column)
		if (tier?.kind === 'tone') {
			// '' is the head-of-list placeholder an UNANSWERED tone cell
			// opens on (see the markup): committing it answers nothing, so
			// blurring away from an untouched cell cannot manufacture
			// whichever tone happens to be first in the radio's own list.
			// The pre-tier Tone column needs no such entry — it is editable
			// only from 'known', so it always opens on a real value.
			if (value === '') return cancelEditor()
			const decihertz = Number(value)
			const current = tierField(tier, data)
			if (current?.state === 'known' && current.value === decihertz) return cancelEditor()
			submitEdit(sv.Slot, (fresh) => ({
				...cloneData(fresh ?? data),
				[tier.key]: { state: 'known', value: decihertz },
			}))
			return
		}
		switch (column.id) {
			case 'mode':
				if (value === data.mode) return cancelEditor()
				submitEdit(sv.Slot, (fresh) => ({ ...cloneData(fresh ?? data), mode: value }))
				return
			case 'shift':
				if (value === data.shift) return cancelEditor()
				submitEdit(sv.Slot, (fresh) => ({ ...cloneData(fresh ?? data), shift: value }))
				return
			case 'ctcss':
				if (value === data.ctcss) return cancelEditor()
				submitEdit(sv.Slot, (fresh) => ({ ...cloneData(fresh ?? data), ctcss: value }))
				return
			case 'tone': {
				const decihertz = Number(value)
				if (data.ctcss_tone?.state === 'known' && decihertz === data.ctcss_tone.value) return cancelEditor()
				submitEdit(sv.Slot, (fresh) => ({
					...cloneData(fresh ?? data),
					ctcss_tone: /** @type {ChannelData['ctcss_tone']} */ ({ state: 'known', value: decihertz }),
				}))
				return
			}
			default:
				cancelEditor()
		}
	}

	/** Commit the clarifier editor (value + Rx/Tx toggles as one edit).
	 * @param {number} rowIdx */
	function commitClarEditor(rowIdx) {
		if (!editing) return
		const sv = slots[rowIdx]
		const data = dataAt(rowIdx)
		if (!sv || !data) return cancelEditor()
		const text = String(clarDraft.hz).trim()
		if (!/^[+-]?\d+$/.test(text)) {
			appState.pushAlert(`"${text}" is not a whole number of hertz for the clarifier — the edit was not applied`)
			return cancelEditor()
		}
		const hz = Number(text)
		if (hz === (data.clar_hz ?? 0) && clarDraft.rx === (data.rx_clar ?? false) && clarDraft.tx === (data.tx_clar ?? false)) {
			return cancelEditor()
		}
		const { rx, tx } = clarDraft
		submitEdit(sv.Slot, (fresh) => ({ ...cloneData(fresh ?? data), clar_hz: hz, rx_clar: rx, tx_clar: tx }))
	}

	/** Toggle a boolean cell (Tag display or Scan skip) as an immediate
	 * one-keystroke commit — no editor needed.
	 *
	 * SCAN SKIP flips only from Known. A non-Known one is not toggled: the
	 * toggle would have to INVENT the state it flips from, and its
	 * 'unknown' means "preserve whatever the radio has"
	 * (core/codeplug/fieldstate.go) — which on every radio that can
	 * currently produce an 'unknown' scan skip is a field the caps declare
	 * UNREACHABLE (spec.FieldSupport.Unreachable: Unsupported in both
	 * directions), so there is nothing for the user to decide and no
	 * honest value to decide it to. The bar is the STATE, not a
	 * hard-coded belief about scan skip: a radio that can genuinely store
	 * one imports Known (core/csvio's chirpScanSkip, M9d-2 task 8) and
	 * this toggle serves it unchanged.
	 *
	 * TAG DISPLAY flips from Known, and from UNKNOWN it ROUTES TO KNOWN
	 * (M9c-6 D5b): the first press commits {known, false} — Display OFF —
	 * and it flips normally from there, so the cell walks Unknown → Off →
	 * On. The invention rule is named and it is single: OFF is the same
	 * conservative, wire-neutral value newChannelData already justifies
	 * for a blank row (grid/columns.js), chosen for the same reason —
	 * where the flag exists it is a MANDATORY wire field, so SOME value
	 * will be sent, and the one that changes least is the one to invent.
	 * This is a decision the user is being ASKED for (an Unknown tag
	 * display blocks its channel at plan time until answered), which is
	 * exactly what makes inventing a starting point legitimate here and
	 * not for scan skip.
	 *
	 * UNAVAILABLE is never toggled, either field: the radio's frame has no
	 * such flag, so there is no question outstanding and any value would
	 * be a fiction. isCellEditable refuses it before openEditor reaches
	 * here; these guards are the second line of that defence.
	 *
	 * A TIER COLUMN OF THE BOOL KIND (data_mode, tuning_step_enabled,
	 * ip_plus) follows TAG DISPLAY's rule, not scan skip's, because
	 * isCellEditable already says it does: such a column renders only where
	 * the bank's capabilities say the radio HAS the field, so an unresolved
	 * value there is a question the user may legitimately answer — and its
	 * 'unknown' and its absent zero state are that same question left
	 * unanswered. The first activation therefore commits Off and it flips
	 * from there, walking the cell unanswered → Off → On. Off is exactly
	 * what a paste of "off" into the column already produces
	 * (parsePasteCell), which is what makes it a starting point rather than
	 * an invention: the two routes to answering the cell agree. The
	 * admission is ASKED of isCellEditable rather than restated here, so
	 * this toggle can never admit a cell the grid refuses to open —
	 * 'unavailable' included.
	 * @param {number} rowIdx @param {ReturnType<typeof columnsFor>[number]} column */
	function toggleCell(rowIdx, column) {
		const sv = slots[rowIdx]
		const data = dataAt(rowIdx)
		if (!sv || !data) return
		const tier = tierColumnFor(column)
		if (tier?.kind === 'bool') {
			if (!isCellEditable(column, data)) return
			const current = tierField(tier, data)
			const next = current?.state === 'known' ? !current.value : false
			submitEdit(sv.Slot, (fresh) => ({
				...cloneData(fresh ?? data),
				[tier.key]: { state: 'known', value: next },
			}))
			return
		}
		const tagDisplayState = data.tag_display?.state
		if (column.id === 'tagDisplay' && (tagDisplayState === 'known' || tagDisplayState === 'unknown')) {
			// Unknown's first press lands on false; Known flips.
			const nextTagDisplay = tagDisplayState === 'unknown' ? false : !data.tag_display.value
			submitEdit(sv.Slot, (fresh) => ({
				...cloneData(fresh ?? data),
				tag_display: /** @type {ChannelData['tag_display']} */ ({ state: 'known', value: nextTagDisplay }),
			}))
		} else if (column.id === 'skip' && data.scan_skip?.state === 'known') {
			const nextSkip = !data.scan_skip.value
			submitEdit(sv.Slot, (fresh) => ({
				...cloneData(fresh ?? data),
				scan_skip: /** @type {ChannelData['scan_skip']} */ ({ state: 'known', value: nextSkip }),
			}))
		}
	}

	// --- delete (task-22 brief §1) -----------------------------------------
	//
	// File-level only — there is no CAT erase (docs/hardware-notes.md's
	// "No CAT erase", HW-CONFIRMED 2026-07-13): confirming replaces the
	// slot's Channel with {Slot, Data: null} via the SAME edit queue every
	// other commit uses (Task 15's applyEditsLocked already accepts and
	// replaces-by-slot for an empty channel — no Go change needed, verified
	// by reading app/codeplug.go directly). Deleting a RequiredSlots slot
	// (M-01) or half a populated PMS pair is not refused here: Go's own
	// Validate pass runs after the edit and its Issues render exactly as
	// any other issue (the row's slot-cell decoration) — the modal copy
	// never promises otherwise.

	/** The slot pending delete confirmation, or null. @type {string | null} */
	let deleteConfirmSlot = $state(null)
	let deleteConfirmDisplay = $state('')

	/** Whether row rowIdx may show/trigger ANY of this task's per-row
	 * affordances (delete, drag-start, keyboard copy/move): populated and
	 * the active bank is not read-only — locked banks show none of them
	 * (brief §1/§2: "read-only banks take no part in drag for v1").
	 * @param {number} rowIdx */
	function canActOnRow(rowIdx) {
		return !bankLocked && dataAt(rowIdx) !== null
	}

	/** @param {number} rowIdx */
	function requestDelete(rowIdx) {
		if (!canActOnRow(rowIdx)) return
		const sv = slots[rowIdx]
		if (!sv) return
		deleteConfirmSlot = sv.Slot
		deleteConfirmDisplay = sv.Display
	}

	function confirmDelete() {
		const slot = deleteConfirmSlot
		deleteConfirmSlot = null
		if (!slot) return
		void focusCell(focus)
		submitEdit(slot, () => null)
	}

	function cancelDelete() {
		deleteConfirmSlot = null
		void focusCell(focus)
	}

	// --- drag copy/swap/move (task-22 brief §2) -----------------------------
	//
	// Pointer path: drag a populated row, drop on another row in the SAME
	// bank tab (cross-tab is structurally impossible here anyway — only the
	// active bank's rows are ever rendered, so there is nothing else to drop
	// on). The gesture itself is the pure grid/dragDrop.js state machine;
	// this component only translates DOM drag events into its calls and
	// opens the resulting popover. Dragging is disabled while ANY cell is
	// being edited (editing !== null) — deliberately global, not just the
	// dragged row's own editor, to keep the interaction unambiguous rather
	// than reasoning about which specific row-vs-editor combinations are
	// "safe".

	let dragState = $state(initialDragState())

	/** @param {number} rowIdx */
	function rowDraggable(rowIdx) {
		return editing === null && canActOnRow(rowIdx) // same populated+unlocked gate as delete/copy-move
	}

	/** @param {DragEvent} e @param {string} slot @param {number} rowIdx */
	function onRowDragStart(e, slot, rowIdx) {
		const result = beginDrag(dragState, { slot, locked: bankLocked, populated: dataAt(rowIdx) !== null })
		if (!result.ok) {
			e.preventDefault()
			return
		}
		dragState = result.state
		if (e.dataTransfer) {
			e.dataTransfer.effectAllowed = 'move'
			e.dataTransfer.setData('text/plain', slot) // not relied upon — dragState carries the source
		}
	}

	/** @param {DragEvent} e @param {string} slot */
	function onRowDragEnter(e, slot) {
		if (!dragState.sourceSlot) return
		e.preventDefault()
	}

	/** @param {DragEvent} e @param {string} slot */
	function onRowDragOver(e, slot) {
		if (!dragState.sourceSlot) return
		e.preventDefault() // required to allow a drop at all
		dragState = hoverDrag(dragState, { slot, locked: bankLocked })
	}

	/** @param {string} slot */
	function onRowDragLeave(slot) {
		dragState = leaveDrag(dragState, slot)
	}

	/** @param {DragEvent} e @param {string} slot */
	function onRowDrop(e, slot) {
		e.preventDefault()
		const result = dropDrag(dragState, { slot, locked: bankLocked })
		dragState = result.state
		if (result.ok) openMoveCopyPopover(/** @type {string} */ (result.source), /** @type {string} */ (result.target))
	}

	function onRowDragEnd() {
		dragState = cancelDrag()
	}

	/** @type {{sourceSlot: string, sourceDisplay: string, targetSlot: string | null} | null} */
	let moveCopyPopover = $state(null)

	/** @param {string} sourceSlot @param {string} targetSlot */
	function openMoveCopyPopover(sourceSlot, targetSlot) {
		const sourceSv = (activeBank?.Slots ?? []).find((sv) => sv.Slot === sourceSlot)
		moveCopyPopover = { sourceSlot, sourceDisplay: sourceSv?.Display ?? sourceSlot, targetSlot }
	}

	/** The per-row "Copy/Move to…" keyboard/a11y equivalent (brief §2): same
	 * popover, opened WITHOUT a resolved target — the dialog itself shows
	 * the slot-entry field. @param {number} rowIdx */
	function openMoveCopyForRow(rowIdx) {
		if (!canActOnRow(rowIdx)) return // same populated+unlocked gate as delete
		const sv = slots[rowIdx]
		if (!sv) return
		moveCopyPopover = { sourceSlot: sv.Slot, sourceDisplay: sv.Display, targetSlot: null }
	}

	function closeMoveCopyPopover() {
		moveCopyPopover = null
		void focusCell(focus)
	}

	/** @param {'copy'|'swap'|'move'} action @param {string} targetSlot */
	function chooseMoveCopy(action, targetSlot) {
		const popover = moveCopyPopover
		moveCopyPopover = null
		void focusCell(focus)
		if (!popover) return
		// Freshest data at confirm time (channelBySlot is derived from the
		// live appState.codeplug, not a snapshot taken when the drag began).
		const sourceData = channelBySlot.get(popover.sourceSlot)?.data ?? null
		if (!sourceData) return // the source row was emptied from under us — nothing to do
		const targetData = channelBySlot.get(targetSlot)?.data ?? null
		const channels = buildDragOpChannels(action, popover.sourceSlot, sourceData, targetSlot, targetData)
		updateChannels(/** @type {Channel[]} */ (channels)).catch(() => {
			// Alert strip already carries the message (bindings.js's
			// updateChannels already reports+rethrows on rejection).
		})
	}

	// --- ↑/↓ row-reorder (task-24) ------------------------------------------
	//
	// Replaces the old per-row → button (which opened MoveCopyDialog) with a
	// pair of neighbour-move buttons for the common case Stuart actually
	// wanted after using the grid on real hardware: walking a channel up or
	// down the CURRENT bank's own visible order, one slot at a time. The
	// dialogue/drag machinery is UNCHANGED and stays reachable via Ctrl/Cmd+M
	// — it still covers long-distance moves; this is purely a short-hop
	// shortcut for the adjacent case. Semantics come straight from
	// grid/dragDrop.js's resolveNeighbourSlot + the same buildDragOpChannels
	// the drag/keyboard-copy paths already use:
	//   - populated neighbour → SWAP: both channels survive, fully sendable,
	//     no data can be lost — committed instantly, no confirmation.
	//   - empty neighbour → MOVE: the source becomes empty in the file (the
	//     same file-level erase as delete/drag-move — no CAT erase exists).
	//     Shown via RowMoveConfirmDialog first, same honest wording as
	//     MoveCopyDialog's own "Move here" caveat.

	/** @type {{sourceSlot: string, sourceDisplay: string, targetSlot: string, targetDisplay: string, targetRowIdx: number} | null} */
	let rowMoveConfirm = $state(null)

	/** @param {number} rowIdx @param {'up'|'down'} direction */
	function requestNeighbourMove(rowIdx, direction) {
		if (!canActOnRow(rowIdx) || !activeBank) return
		const sv = slots[rowIdx]
		if (!sv) return
		const result = resolveNeighbourSlot(activeBank, sv.Slot, direction, (neighbourSlot) =>
			(channelBySlot.get(neighbourSlot)?.data ?? null) !== null
		)
		if (!result.ok) return
		const targetRowIdx = direction === 'up' ? rowIdx - 1 : rowIdx + 1
		if (result.action === 'swap') {
			commitNeighbourMove(sv.Slot, result.targetSlot, 'swap', targetRowIdx)
			return
		}
		const targetSv = slots[targetRowIdx]
		rowMoveConfirm = {
			sourceSlot: sv.Slot,
			sourceDisplay: sv.Display,
			targetSlot: result.targetSlot,
			targetDisplay: targetSv?.Display ?? result.targetSlot,
			targetRowIdx,
		}
	}

	/** Commits the resolved neighbour swap/move and keeps focus (and the
	 * roving-tabindex "current cell") ON the channel that just moved, so
	 * repeated ↑/↓ activations walk it further without re-acquiring the
	 * row each time.
	 * @param {string} sourceSlot @param {string} targetSlot @param {'swap'|'move'} action @param {number} targetRowIdx */
	function commitNeighbourMove(sourceSlot, targetSlot, action, targetRowIdx) {
		const sourceData = channelBySlot.get(sourceSlot)?.data ?? null
		if (!sourceData) return // the source row was emptied from under us — nothing to do
		const targetData = channelBySlot.get(targetSlot)?.data ?? null
		const channels = buildDragOpChannels(action, sourceSlot, sourceData, targetSlot, targetData)
		updateChannels(/** @type {Channel[]} */ (channels)).catch(() => {
			// Alert strip already carries the message.
		})
		focus = { row: targetRowIdx, col: focus.col }
		void focusCell(focus)
	}

	function confirmNeighbourMove() {
		const pending = rowMoveConfirm
		rowMoveConfirm = null
		if (!pending) return
		commitNeighbourMove(pending.sourceSlot, pending.targetSlot, 'move', pending.targetRowIdx)
	}

	function cancelNeighbourMove() {
		rowMoveConfirm = null
		void focusCell(focus)
	}

	/** Whether row rowIdx's ↑ ('up') or ↓ ('down') action is at the bank's
	 * own boundary (first slot has no "up", last has no "down") — disabled,
	 * not hidden, so the pair's layout stays stable per row.
	 * @param {number} rowIdx @param {'up'|'down'} direction */
	function neighbourAtBoundary(rowIdx, direction) {
		return direction === 'up' ? rowIdx === 0 : rowIdx === slots.length - 1
	}

	/** @param {number} rowIdx @param {'up'|'down'} direction */
	function neighbourAriaLabel(rowIdx, direction) {
		const sv = slots[rowIdx]
		if (!sv) return ''
		const neighbourSv = slots[direction === 'up' ? rowIdx - 1 : rowIdx + 1]
		return neighbourSv ? `Move ${sv.Display} ${direction} to ${neighbourSv.Display}` : `Move ${sv.Display} ${direction}`
	}

	// --- grid keyboard handling -------------------------------------------

	/** Pre-tier columns whose editor is a text input (typing a character
	 * opens it prefilled). */
	const TEXT_COLUMNS = new Set(['freq', 'tag', 'clar'])

	/** Whether typing a printable character opens this column's editor
	 * seeded with it: the three above, and every tier column that takes the
	 * free-text editor.
	 * @param {ReturnType<typeof columnsFor>[number]} column */
	function opensOnTyping(column) {
		return TEXT_COLUMNS.has(column.id) || tierTextColumn(column) !== null
	}

	/** @param {KeyboardEvent} e @param {number} rowIdx @param {number} colIdx */
	function onCellKeydown(e, rowIdx, colIdx) {
		if (editing) return // the editor owns the keyboard while open
		// Alt+ArrowUp/ArrowDown (task-24 brief §5): the keyboard equivalent of
		// the per-row ↑/↓ actions — checked BEFORE moveFocus below, since
		// ArrowUp/ArrowDown are otherwise plain cell-navigation keys there
		// regardless of any held modifier. Always swallowed (never falls
		// through to ordinary navigation) so Alt+Arrow is unambiguously this
		// binding, even on a row where canActOnRow refuses the actual move.
		if (e.altKey && (e.key === 'ArrowUp' || e.key === 'ArrowDown')) {
			e.preventDefault()
			if (canActOnRow(rowIdx)) requestNeighbourMove(rowIdx, e.key === 'ArrowUp' ? 'up' : 'down')
			return
		}
		const moved = moveFocus(focus, e.key, dims, e.shiftKey)
		if (moved) {
			e.preventDefault()
			void focusCell(moved)
			return
		}
		if (e.key === 'Tab') return // leaving the grid — let the browser take it
		// Delete/Backspace (task-22 brief §1): the keyboard equivalent of the
		// per-row ✕ affordance, outside an open editor (already excluded by
		// the guard above) — a no-op on an empty row or a locked bank
		// (canActOnRow), so it never conflicts with an in-cell edit.
		if ((e.key === 'Delete' || e.key === 'Backspace') && canActOnRow(rowIdx)) {
			e.preventDefault()
			requestDelete(rowIdx)
			return
		}
		// Ctrl/Cmd+M (task-22 brief §2): the keyboard equivalent of the
		// per-row "Copy/Move to…" affordance — deliberately NOT a bare
		// letter key, so it can never collide with typing a character into
		// a text column to open its editor (that path explicitly excludes
		// ctrlKey/metaKey below).
		if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'm' && canActOnRow(rowIdx)) {
			e.preventDefault()
			openMoveCopyForRow(rowIdx)
			return
		}
		const column = columns[colIdx]
		if (e.key === 'Enter' || (e.key === ' ' && isToggleColumn(column))) {
			e.preventDefault()
			openEditor(rowIdx, colIdx)
			return
		}
		if (e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey && opensOnTyping(column)) {
			e.preventDefault()
			openEditor(rowIdx, colIdx, e.key)
		}
	}

	/** Shared Enter/Escape handling for editor elements. Always stops
	 * propagation: commit/cancel null `editing` synchronously, so letting
	 * the same keydown bubble on to the cell handler would REOPEN the
	 * editor with stale data mid-event and the follow-up blur would
	 * re-commit the old value (caught live against the real bridge —
	 * the mocked component test alone masked it via the unchanged-commit
	 * skip; pinned by a regression test since).
	 * @param {KeyboardEvent} e @param {() => void} commit */
	function onEditorKeydown(e, commit) {
		e.stopPropagation()
		if (e.key === 'Enter') {
			e.preventDefault()
			commit()
		} else if (e.key === 'Escape') {
			e.preventDefault()
			cancelEditor()
		}
	}

	/** The clarifier editor is a composite (input + two toggles): commit
	 * when focus leaves the whole group, not each part.
	 * @param {FocusEvent} e @param {number} rowIdx */
	function onClarFocusOut(e, rowIdx) {
		const group = /** @type {HTMLElement} */ (e.currentTarget)
		const next = /** @type {Node | null} */ (e.relatedTarget)
		if (next && group.contains(next)) return
		commitClarEditor(rowIdx)
	}

	// --- paste --------------------------------------------------------------

	/** @param {ClipboardEvent} e */
	function onPaste(e) {
		if (editing) return // native paste into the open editor
		const spec = appState.uiSpec
		if (!activeBank || !spec) return
		e.preventDefault()
		const text = e.clipboardData?.getData('text') ?? ''
		const rows = parseBlock(text)
		if (rows.length === 0) return
		const result = mapPasteToChannels(rows, {
			startRow: focus.row,
			startCol: focus.col,
			bank: activeBank,
			channelBySlot,
			uiSpec: spec,
		})
		if (!result.ok) {
			appState.pushAlert(`paste rejected: ${result.reason}`)
			return
		}
		if (result.channels.length === 0) return
		updateChannels(result.channels).catch(() => {
			// Alert strip already carries the message.
		})
	}

	// --- small display helpers ----------------------------------------------

	/** Split a six-decimal MHz string for the readout treatment (integer
	 * part bright, fraction dimmer). @param {string} mhz */
	function freqParts(mhz) {
		const dot = mhz.indexOf('.')
		return dot < 0 ? [mhz, ''] : [mhz.slice(0, dot), mhz.slice(dot)]
	}

	/** @param {ReturnType<typeof columnsFor>[number]} column @param {ChannelData | null} data */
	function cellPreserved(column, data) {
		if (data == null) return false
		if (column.id === 'tone') return data.ctcss_tone?.state !== 'known'
		if (column.id === 'skip') return data.scan_skip?.state !== 'known'
		return false
	}
</script>

<div class="grid-region">
	{#if appState.codeplug === null}
		<div class="grid-empty">
			<p class="grid-empty-title">No codeplug loaded</p>
			<p class="grid-empty-hint">Read Radio or open a saved codeplug to see channels here.</p>
		</div>
	{:else if appState.uiSpec === null || activeBank === null}
		<div class="grid-empty">
			<p class="grid-empty-title">Grid layout unavailable</p>
			<p class="grid-empty-hint">The radio's bank layout could not be loaded — see the alert above, then reconnect or reload.</p>
		</div>
	{:else}
		<div class="bank-tabs" role="tablist" aria-label="Memory banks">
			{#each banks as bank, i (bank.ID)}
				{@const count = bankIssueCount(bank)}
				<button
					type="button"
					role="tab"
					id={`bank-tab-${bank.ID}`}
					aria-selected={activeBank.ID === bank.ID}
					aria-controls="channel-grid-panel"
					tabindex={activeBank.ID === bank.ID ? 0 : -1}
					class="bank-tab"
					class:active={activeBank.ID === bank.ID}
					onclick={() => selectBank(bank.ID)}
					onkeydown={(e) => onTabKeydown(e, i)}
				>
					{bank.Label}
					{#if bank.ReadOnly}
						<svg class="lock-glyph" viewBox="0 0 12 12" aria-hidden="true"><path d="M3.5 5V3.5a2.5 2.5 0 0 1 5 0V5h.75c.41 0 .75.34.75.75v4.5c0 .41-.34.75-.75.75h-6.5A.75.75 0 0 1 2 10.25v-4.5C2 5.34 2.34 5 2.75 5Zm1.25 0h2.5V3.5a1.25 1.25 0 0 0-2.5 0Z" fill="currentColor"/></svg>
					{/if}
					{#if count > 0}
						<span
							class="issue-chip"
							class:issue-chip-error={!appState.issuesAdvisory && bankHasError(bank)}
							aria-label={`${count} ${count === 1 ? 'issue' : 'issues'}`}
						>{count}</span>
					{/if}
				</button>
			{/each}
		</div>

		<div
			class="grid-scroll"
			id="channel-grid-panel"
			role="tabpanel"
			aria-labelledby={`bank-tab-${activeBank.ID}`}
			onpaste={onPaste}
		>
			<table class="grid" role="grid" aria-label={`${activeBank.Label} channels`} aria-readonly={bankLocked || undefined} bind:this={tableEl}>
				<thead>
					<tr>
						{#each columns as column (column.id)}
							<th scope="col" class={`col-${column.id}`}>{column.label}</th>
						{/each}
						<!-- Trailing per-row actions column (task-22 layout fix): a
						     dedicated slim column so the action icons never overlap
						     the slot label or any data cell. NOT part of the column list —
						     nav.js's dims, paste's column mapping and the roving
						     tabindex all stay column-list-sized; this column carries no
						     data-row/data-col and takes no part in cell focus. -->
						<th scope="col" class="col-actions"><span class="visually-hidden">Row actions</span></th>
					</tr>
				</thead>
				<tbody>
					{#each slots as sv, r (sv.Slot)}
						{@const data = channelBySlot.get(sv.Slot)?.data ?? null}
						{@const rowIssues = issuesBySlot.get(sv.Slot) ?? []}
						{@const rowSeverity = severityClass(rowIssues)}
						{@const draggableRow = rowDraggable(r)}
						<tr
							class:row-empty={data === null}
							class:row-locked={bankLocked}
							class:row-issue-error={rowSeverity === 'error'}
							class:row-issue-warn={rowSeverity === 'warn'}
							class:row-drag-over={dragState.overSlot === sv.Slot}
							draggable={draggableRow}
							ondragstart={(e) => onRowDragStart(e, sv.Slot, r)}
							ondragenter={(e) => onRowDragEnter(e, sv.Slot)}
							ondragover={(e) => onRowDragOver(e, sv.Slot)}
							ondragleave={() => onRowDragLeave(sv.Slot)}
							ondrop={(e) => onRowDrop(e, sv.Slot)}
							ondragend={onRowDragEnd}
						>
							{#each columns as column, c (column.id)}
								{@const issues = cellIssues(sv.Slot, column)}
								{@const cellSeverity = severityClass(issues)}
								{@const preserved = cellPreserved(column, data)}
								{@const tier = tierColumnFor(column)}
								{@const tierText = tierTextColumn(column)}
								<td
									class={`col-${column.id}`}
									class:cell-issue-error={cellSeverity === 'error'}
									class:cell-issue-warn={cellSeverity === 'warn'}
									class:cell-preserved={preserved}
									data-row={r}
									data-col={c}
									tabindex={focus.row === r && focus.col === c ? 0 : -1}
									title={issueTitle(issues) ?? (preserved ? preservedTooltip(column) : undefined)}
									onfocus={() => setFocusPos(r, c)}
									onkeydown={(e) => onCellKeydown(e, r, c)}
									ondblclick={() => openEditor(r, c)}
								>
									{#if column.id === 'slot'}
										{#if bankLocked}
											<svg class="lock-glyph" viewBox="0 0 12 12" role="img" aria-label="read-only"><title>{READ_ONLY_TOOLTIP}</title><path d="M3.5 5V3.5a2.5 2.5 0 0 1 5 0V5h.75c.41 0 .75.34.75.75v4.5c0 .41-.34.75-.75.75h-6.5A.75.75 0 0 1 2 10.25v-4.5C2 5.34 2.34 5 2.75 5Zm1.25 0h2.5V3.5a1.25 1.25 0 0 0-2.5 0Z" fill="currentColor"/></svg>
										{/if}
										{sv.Display}
									{:else if editing && editing.row === r && editing.col === c}
										{#if column.id === 'freq' || column.id === 'tag'}
											<span class="editor-wrap">
												<input
													type="text"
													class="cell-editor"
													aria-label={`${column.label}, ${sv.Display}`}
													bind:value={textDraft}
													use:autofocus={{ selectAll: editing.seed === null }}
													onkeydown={(e) => onEditorKeydown(e, () => commitTextEditor(r, c))}
													onblur={() => commitTextEditor(r, c)}
												/>
												{#if column.id === 'tag'}
													<span class="byte-counter" class:over={tagBytes > tagMaxBytes}>{tagBytes}/{tagMaxBytes}</span>
												{/if}
											</span>
										{:else if tierText}
											<!-- A tier column of the freq, int or text kind: the same
											     plain text editor Frequency uses, committed through the
											     PASTE parser (commitTierTextEditor). Free text is
											     deliberate for the text kinds — see the "which editor
											     each KIND opens" comment above for the seven
											     vocabularies core/spec declares and GetUISpec does not
											     serve. -->
											<span class="editor-wrap">
												<input
													type="text"
													class="cell-editor"
													aria-label={`${column.label}, ${sv.Display}`}
													bind:value={textDraft}
													use:autofocus={{ selectAll: editing.seed === null }}
													onkeydown={(e) => onEditorKeydown(e, () => commitTierTextEditor(r, c))}
													onblur={() => commitTierTextEditor(r, c)}
												/>
											</span>
										{:else if column.id === 'clar'}
											<span class="editor-wrap clar-editor" onfocusout={(e) => onClarFocusOut(e, r)}>
												<!-- min/max/step from the UISpec (ClarMaxHz/ClarStepHz) so
												     arrow keys step by the radio's own increment; still no
												     local validation — Go remains authoritative on commit. -->
												<input
													type="number"
													class="cell-editor cell-editor-clar"
													aria-label={`Clarifier hertz, ${sv.Display}`}
													inputmode="numeric"
													min={-appState.uiSpec.ClarMaxHz}
													max={appState.uiSpec.ClarMaxHz}
													step={appState.uiSpec.ClarStepHz}
													bind:value={clarDraft.hz}
													use:autofocus={{ selectAll: editing.seed === null }}
													onkeydown={(e) => onEditorKeydown(e, () => commitClarEditor(r))}
												/>
												<label class="clar-toggle"><input type="checkbox" bind:checked={clarDraft.rx} /> Rx</label>
												<label class="clar-toggle"><input type="checkbox" bind:checked={clarDraft.tx} /> Tx</label>
											</span>
										{:else}
											<select
												class="cell-editor"
												aria-label={`${column.label}, ${sv.Display}`}
												use:autofocus={{ selectAll: false }}
												onkeydown={(e) => onEditorKeydown(e, () => commitSelectEditor(r, c, /** @type {HTMLSelectElement} */ (e.currentTarget).value))}
												onblur={(e) => commitSelectEditor(r, c, /** @type {HTMLSelectElement} */ (e.currentTarget).value)}
											>
												{#if column.id === 'mode'}
													{#each appState.uiSpec.Modes ?? [] as mode (mode)}
														<option value={mode} selected={mode === data?.mode}>{mode}</option>
													{/each}
												{:else if column.id === 'shift'}
													{#each appState.uiSpec.ShiftOptions ?? [] as shift (shift)}
														<option value={shift} selected={shift === data?.shift}>{shift}</option>
													{/each}
												{:else if column.id === 'ctcss'}
													{#each appState.uiSpec.CTCSSStateOptions ?? [] as state (state)}
														<option value={state} selected={state === data?.ctcss}>{state}</option>
													{/each}
												{:else if column.id === 'tone'}
													{#each appState.uiSpec.Tones ?? [] as tone (tone.Decihertz)}
														<option value={String(tone.Decihertz)} selected={data?.ctcss_tone?.state === 'known' && tone.Decihertz === data.ctcss_tone.value}>{tone.Display}</option>
													{/each}
												{:else if tier && tier.kind === 'tone'}
													{@const current = tierField(tier, data)}
													{#if current?.state !== 'known'}
														<!-- The head-of-list "nothing chosen yet" entry, shown
														     only while the cell is unanswered: without it, opening
														     such a cell and blurring away would commit whichever
														     tone the radio happens to list first. It is the em dash
														     the cell itself displays, and commitSelectEditor makes
														     a no-op of it. -->
														<option value="" selected>—</option>
													{/if}
													{#each appState.uiSpec.Tones ?? [] as tone (tone.Decihertz)}
														<option value={String(tone.Decihertz)} selected={current?.state === 'known' && tone.Decihertz === current.value}>{tone.Display}</option>
													{/each}
												{/if}
											</select>
										{/if}
									{:else if column.id === 'freq'}
										{#if data === null}
											<span class="empty-affordance">empty</span>
										{:else}
											{@const parts = freqParts(displayValue(column, data, appState.uiSpec))}
											<span class="freq-readout"><span>{parts[0]}</span><span class="freq-frac">{parts[1]}</span></span>
										{/if}
									{:else if column.id === 'clar' && data !== null}
										{displayValue(column, data, appState.uiSpec)}
										{#if data.rx_clar}<span class="clar-chip">Rx</span>{/if}
										{#if data.tx_clar}<span class="clar-chip">Tx</span>{/if}
									{:else}
										{displayValue(column, data, appState.uiSpec)}
									{/if}
								</td>
							{/each}
							<td class="col-actions">
								{#if canActOnRow(r)}
									<!-- Per-row action area (task-22 brief §1, task-24 brief §1/§2/§3):
									     pointer/hover-only discoverability affordance in its own
									     trailing column (layout fix: the absolute overlay inside the
									     slot cell could overlap/truncate the slot label — caught live
									     by Stuart). tabindex="-1" deliberately keeps the buttons OUT of
									     the grid's own roving-tabindex Tab sequence (Tab is reserved for
									     leaving the grid, per nav.js's contract); the REAL keyboard-
									     equivalent paths are Delete/Backspace, Alt+ArrowUp/Down and
									     Ctrl/Cmd+M on the row's own focused cell (onCellKeydown, above).
									     The old per-row → button (which opened MoveCopyDialog) is
									     replaced here by the ↑/↓ pair (task-24) — the dialogue/drag path
									     for long-distance moves stays reachable via Ctrl/Cmd+M only. -->
									<span class="row-actions">
										<button
											type="button"
											class="row-action row-action-up"
											tabindex="-1"
											disabled={neighbourAtBoundary(r, 'up')}
											aria-label={neighbourAriaLabel(r, 'up')}
											title={neighbourAriaLabel(r, 'up')}
											onclick={(e) => {
												e.stopPropagation()
												requestNeighbourMove(r, 'up')
											}}
										>
											<svg viewBox="0 0 12 12" aria-hidden="true" fill="none"><path d="M2.5 7.5 6 3.5 9.5 7.5" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/></svg>
										</button>
										<button
											type="button"
											class="row-action row-action-down"
											tabindex="-1"
											disabled={neighbourAtBoundary(r, 'down')}
											aria-label={neighbourAriaLabel(r, 'down')}
											title={neighbourAriaLabel(r, 'down')}
											onclick={(e) => {
												e.stopPropagation()
												requestNeighbourMove(r, 'down')
											}}
										>
											<svg viewBox="0 0 12 12" aria-hidden="true" fill="none"><path d="M2.5 4.5 6 8.5 9.5 4.5" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/></svg>
										</button>
										<button
											type="button"
											class="row-action row-action-delete"
											tabindex="-1"
											aria-label={`Delete ${sv.Display}`}
											title={`Delete ${sv.Display}`}
											onclick={(e) => {
												e.stopPropagation()
												requestDelete(r)
											}}
										>
											<svg viewBox="0 0 12 12" aria-hidden="true"><path d="M3 3l6 6M9 3l-6 6" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/></svg>
										</button>
									</span>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<p class="grid-legend" title="See docs/hardware-notes.md § M5b write trials">
			{appState.uiSpec.GridLegendNote}
			{appState.uiSpec.ToneScanSkipVerification}
		</p>
	{/if}

	{#if deleteConfirmSlot}
		<DeleteConfirmDialog
			slotDisplay={deleteConfirmDisplay}
			eraseNote={appState.uiSpec?.EraseDialogNote ?? ''}
			onconfirm={confirmDelete}
			oncancel={cancelDelete}
		/>
	{/if}

	{#if moveCopyPopover && activeBank}
		<MoveCopyDialog
			bank={activeBank}
			{channelBySlot}
			sourceSlot={moveCopyPopover.sourceSlot}
			sourceDisplay={moveCopyPopover.sourceDisplay}
			targetSlot={moveCopyPopover.targetSlot}
			onchoose={chooseMoveCopy}
			oncancel={closeMoveCopyPopover}
		/>
	{/if}

	{#if rowMoveConfirm}
		<RowMoveConfirmDialog
			sourceDisplay={rowMoveConfirm.sourceDisplay}
			targetDisplay={rowMoveConfirm.targetDisplay}
			onconfirm={confirmNeighbourMove}
			oncancel={cancelNeighbourMove}
		/>
	{/if}
</div>

<style>
	.grid-region {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-height: 0;
	}

	/* --- empty states --- */

	.grid-empty {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: var(--space-1);
		margin: var(--space-4);
		background: var(--colour-panel-sunken);
		border: 1px dashed var(--colour-hairline);
		border-radius: var(--radius-md);
		text-align: center;
		padding: var(--space-5);
	}

	.grid-empty-title {
		margin: 0;
		font-size: 14px;
		font-family: var(--font-mono);
		color: var(--colour-text);
	}

	.grid-empty-hint {
		margin: 0;
		font-size: 12.5px;
		color: var(--colour-text-dim);
	}

	/* --- bank tabs --- */

	.bank-tabs {
		display: flex;
		gap: var(--space-1);
		padding: var(--space-2) var(--space-4) 0;
		background: var(--colour-bg);
	}

	.bank-tab {
		display: inline-flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-4);
		border: 1px solid var(--colour-hairline);
		border-bottom: none;
		border-radius: var(--radius-sm) var(--radius-sm) 0 0;
		background: var(--colour-panel-sunken);
		color: var(--colour-text-dim);
		font-size: 12.5px;
	}

	.bank-tab.active {
		color: var(--colour-text);
		box-shadow: inset 0 2px 0 var(--colour-accent);
	}

	.issue-chip {
		font-family: var(--font-mono);
		font-size: 10.5px;
		line-height: 1;
		padding: 2px 6px;
		border-radius: 8px;
		background: var(--colour-warn-bg);
		color: var(--colour-warn);
	}

	.issue-chip-error {
		background: var(--colour-danger-bg);
		color: var(--colour-danger);
	}

	.lock-glyph {
		width: 11px;
		height: 11px;
		flex-shrink: 0;
		color: var(--colour-text-faint);
		vertical-align: -1px;
	}

	/* --- the grid --- */

	.grid-scroll {
		flex: 1;
		overflow: auto;
		margin: 0 var(--space-4) var(--space-4);
		border: 1px solid var(--colour-hairline);
		border-radius: 0 var(--radius-sm) var(--radius-sm) var(--radius-sm);
		background: var(--colour-panel-sunken);
	}

	table.grid {
		width: 100%;
		border-collapse: collapse;
		font-size: 12.5px;
	}

	thead th {
		position: sticky;
		top: 0;
		z-index: 1;
		background: var(--colour-panel);
		color: var(--colour-text-dim);
		font-weight: 500;
		font-size: 10.5px;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		text-align: left;
		padding: var(--space-2) var(--space-2);
		border-bottom: 1px solid var(--colour-hairline);
		white-space: nowrap;
	}

	td {
		padding: 3px var(--space-2);
		border-bottom: 1px solid var(--colour-hairline-soft);
		white-space: nowrap;
		cursor: default;
	}

	/* The active-cell cursor: always visible (mouse or keyboard), like a
	 * rig's own cursor — plus the issue outline stays independent. */
	td:focus {
		outline: none;
		box-shadow: inset 0 0 0 2px var(--colour-accent-strong);
	}

	/* --- column treatments --- */

	.col-slot {
		font-family: var(--font-mono);
		color: var(--colour-text-dim);
		width: 4.5rem;
	}

	.col-freq {
		font-family: var(--font-mono);
		text-align: right;
		width: 7.5rem;
	}

	.freq-frac {
		color: var(--colour-text-dim);
	}

	.col-clar,
	.col-tag {
		font-family: var(--font-mono);
	}

	.clar-chip {
		font-size: 10px;
		font-family: var(--font-ui);
		color: var(--colour-text-dim);
		border: 1px solid var(--colour-hairline);
		border-radius: var(--radius-sm);
		padding: 0 3px;
		margin-left: 3px;
		vertical-align: 1px;
	}

	/* --- row/cell states --- */

	.row-empty td {
		color: var(--colour-text-faint);
	}

	.empty-affordance {
		font-style: italic;
		color: var(--colour-text-faint);
	}

	.row-locked td {
		color: var(--colour-text-dim);
	}

	.cell-preserved {
		color: var(--colour-text-faint);
	}

	/* Issue decoration: outline (not box-shadow) so it coexists with the
	 * focus cursor; the row marker is a bar on the slot cell. */
	.cell-issue-error {
		outline: 1px solid var(--colour-danger);
		outline-offset: -1px;
		background: var(--colour-danger-bg);
	}

	.cell-issue-warn {
		outline: 1px solid var(--colour-warn);
		outline-offset: -1px;
		background: var(--colour-warn-bg);
	}

	.row-issue-error td.col-slot {
		box-shadow: inset 3px 0 0 var(--colour-danger);
	}

	.row-issue-warn td.col-slot {
		box-shadow: inset 3px 0 0 var(--colour-warn);
	}

	/* --- editors --- */

	.editor-wrap {
		display: inline-flex;
		align-items: center;
		gap: var(--space-1);
		width: 100%;
	}

	.cell-editor {
		font-family: inherit;
		font-size: inherit;
		width: 100%;
		min-width: 3rem;
		background: var(--colour-bg);
		color: var(--colour-text);
		border: 1px solid var(--colour-accent);
		border-radius: var(--radius-sm);
		padding: 1px 4px;
	}

	.cell-editor-clar {
		width: 4.5rem;
	}

	.clar-editor {
		white-space: nowrap;
	}

	.clar-toggle {
		font-family: var(--font-ui);
		font-size: 11px;
		color: var(--colour-text-dim);
		display: inline-flex;
		align-items: center;
		gap: 2px;
	}

	.byte-counter {
		font-family: var(--font-mono);
		font-size: 10.5px;
		color: var(--colour-text-dim);
		flex-shrink: 0;
	}

	.byte-counter.over {
		color: var(--colour-danger);
	}

	/* --- per-row actions (task 22, widened task 24 for the ↑/↓ pair):
	 * backlit-switch metaphor — invisible until the row is hovered or holds
	 * focus, like a panel switch lit only when it's the one under your
	 * hand. Live in their OWN slim trailing column (layout fix: the earlier
	 * absolute overlay inside the slot cell could overlap/truncate the slot
	 * label — caught by Stuart's live session), so the slot label and every
	 * data column stay untouched at any window width. Static opacity toggle
	 * only, no motion beyond it — the project-wide prefers-reduced-motion
	 * rule already zeroes the transition duration when reduced motion is
	 * requested. Widened from 2.5rem (✕/→ pair) to fit three 15px buttons
	 * (↑/↓/✕) plus their gaps without ever squeezing into the slot cell —
	 * the slot-cell-purity test (ChannelGrid.test.js) pins the slot cell's
	 * own textContent, not this column's width, but the width is kept
	 * generous precisely so that guarantee never comes under pressure. */
	.col-actions {
		width: 4.5rem;
		min-width: 4.5rem;
		text-align: right;
	}

	th.col-actions .visually-hidden {
		position: absolute;
		width: 1px;
		height: 1px;
		overflow: hidden;
		clip-path: inset(50%);
		white-space: nowrap;
	}

	.row-actions {
		display: inline-flex;
		gap: 2px;
		vertical-align: middle;
		opacity: 0;
		pointer-events: none;
		transition: opacity var(--transition-fast);
	}

	tr:hover .row-actions,
	tr:focus-within .row-actions {
		opacity: 1;
		pointer-events: auto;
	}

	.row-action {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 15px;
		height: 15px;
		padding: 0;
		border-radius: 3px;
		color: var(--colour-text-dim);
	}

	.row-action svg {
		width: 9px;
		height: 9px;
	}

	.row-action-delete:hover {
		background: var(--colour-danger-bg);
		color: var(--colour-danger);
	}

	.row-action-up:hover,
	.row-action-down:hover {
		background: var(--colour-panel-raised);
		color: var(--colour-accent);
	}

	/* First/last-slot boundary (task 24): disabled, not hidden, so the
	 * ↑/↓/✕ trio's layout never shifts row-to-row. */
	.row-action:disabled {
		opacity: 0.3;
		cursor: not-allowed;
	}

	.row-action:disabled:hover {
		background: none;
		color: var(--colour-text-dim);
	}

	/* --- drag copy/swap/move (task 22) --- */

	tr[draggable='true'] {
		cursor: grab;
	}

	.row-drag-over td {
		outline: 1px dashed var(--colour-accent-strong);
		outline-offset: -1px;
	}

	/* --- legend (task 22 §3: Tone/Scan-skip discoverability) --- */

	.grid-legend {
		margin: 0 var(--space-4) var(--space-4);
		font-size: 11.5px;
		line-height: 1.5;
		color: var(--colour-text-faint);
	}
</style>
