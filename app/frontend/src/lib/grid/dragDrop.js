// SPDX-License-Identifier: GPL-3.0-or-later

// Pure logic for task 22's drag copy/swap/move (file-level, within one
// bank tab — see docs/hardware-notes.md's "M5b write trials" for why a
// Move/Delete carries the same honest erase caveat: there is no CAT
// erase, so the radio keeps a slot's old contents until front-panel
// deletion). Three independent pure concerns, kept in one module because
// they only ever get used together by ChannelGrid's drag/keyboard-copy
// flow:
//
//   1. The drag GESTURE state machine (beginDrag/hoverDrag/leaveDrag/
//      dropDrag/cancelDrag) — no DOM, the component translates
//      dragstart/dragover/dragleave/drop/dragend into these calls, exactly
//      as nav.js does for keyboard events.
//   2. buildDragOpChannels — the exact UpdateChannels batch for a
//      confirmed copy/swap/move, shared by BOTH the pointer (drag+drop)
//      and keyboard ("Copy/Move to…") paths, since both resolve to the
//      same (action, source, target) triple before committing.
//   3. resolveTargetSlot — validates a typed slot-entry (the keyboard
//      path's "e.g. M-50" field) against the CURRENT bank's own slots
//      only (cross-tab move/copy is out of scope for v1 — brief's scope
//      guard).
//
// Task 24 adds a fourth, equally pure concern:
//
//   4. resolveNeighbourSlot — the ↑/↓ row-action's neighbour resolution:
//      which slot is "previous"/"next" in the CURRENT bank's own visible
//      Slots order, first/last-slot boundaries, and whether the resulting
//      commit is a SWAP (populated neighbour: both channels survive,
//      instant, no confirmation needed) or a MOVE (empty neighbour: the
//      source becomes empty in the file — the caller must confirm before
//      committing, same erase caveat as drag-move/delete). It never
//      touches ChannelData itself — the caller still builds the actual
//      commit via buildDragOpChannels once it knows the action.

import { cloneData } from './columns.js'

/** @typedef {import('../../../wailsjs/go/models').codeplug.Channel} Channel */
/** @typedef {import('../../../wailsjs/go/models').codeplug.ChannelData} ChannelData */
/** @typedef {import('../../../wailsjs/go/models').main.BankView} BankView */

/** @typedef {{sourceSlot: string | null, overSlot: string | null}} DragState */

/** The idle state — no drag in progress. @returns {DragState} */
export function initialDragState() {
	return { sourceSlot: null, overSlot: null }
}

/**
 * Begins a drag from a row (the component's ondragstart). Refuses (ok:
 * false, state UNCHANGED — same reference) a locked bank or an empty
 * slot — brief §2: "drag a populated row"; read-only banks "take no part
 * in drag for v1". The component should e.preventDefault() the native
 * dragstart when ok is false, so no ghost drag begins at all.
 * @param {DragState} state
 * @param {{slot: string, locked: boolean, populated: boolean}} row
 * @returns {{ok: boolean, state: DragState}}
 */
export function beginDrag(state, { slot, locked, populated }) {
	if (locked || !populated) return { ok: false, state }
	return { ok: true, state: { sourceSlot: slot, overSlot: null } }
}

/**
 * Hovering a candidate target row while dragging (ondragover). A no-op
 * (returns the SAME state) when no drag is in progress. A locked bank or
 * the source row itself is never a valid target: overSlot is explicitly
 * cleared (no stale highlight) rather than left showing the last valid
 * hover.
 * @param {DragState} state
 * @param {{slot: string, locked: boolean}} target
 * @returns {DragState}
 */
export function hoverDrag(state, { slot, locked }) {
	if (!state.sourceSlot) return state
	if (locked || slot === state.sourceSlot) {
		return state.overSlot === null ? state : { ...state, overSlot: null }
	}
	return state.overSlot === slot ? state : { ...state, overSlot: slot }
}

/**
 * Leaving a candidate target row (ondragleave). Only clears the highlight
 * if `slot` IS the current overSlot: an adjacent row's dragenter commonly
 * fires before the old row's dragleave in real browsers, and a late
 * dragleave for a row that is no longer current must not clear the
 * NEW (already-current) hover.
 * @param {DragState} state
 * @param {string} slot
 * @returns {DragState}
 */
export function leaveDrag(state, slot) {
	if (state.overSlot !== slot) return state
	return { ...state, overSlot: null }
}

/**
 * Drops onto a row (ondrop). Always resolves to idle — the drag gesture
 * is over either way. A locked bank, no drag in progress, or dropping on
 * the source row itself (brief §2: "Drop on self = no-op") all refuse
 * (ok: false) without raising an error; only a genuine cross-slot drop on
 * an unlocked target succeeds.
 * @param {DragState} state
 * @param {{slot: string, locked: boolean}} target
 * @returns {{ok: boolean, reason?: 'no-drag'|'locked'|'self', source?: string, target?: string, state: DragState}}
 */
export function dropDrag(state, { slot, locked }) {
	const idle = initialDragState()
	if (!state.sourceSlot) return { ok: false, reason: 'no-drag', state: idle }
	if (locked) return { ok: false, reason: 'locked', state: idle }
	if (slot === state.sourceSlot) return { ok: false, reason: 'self', state: idle }
	return { ok: true, source: state.sourceSlot, target: slot, state: idle }
}

/**
 * Cancels a drag in progress (ondragend firing without a drop having
 * happened, e.g. dropped outside the grid, or Escape) — always idle.
 * @returns {DragState}
 */
export function cancelDrag() {
	return initialDragState()
}

/**
 * Builds the exact UpdateChannels batch for one confirmed copy, swap, or
 * move (task-22 brief §2) — ALWAYS both slots, in [source, target] order,
 * so the edit queue and Go's own single validation pass see the whole
 * operation as ONE atomic edit (brief: "All three commit through ONE
 * UpdateChannels call (both slots in one batch)"). Every value that goes
 * into the outgoing data is passed through columns.js's cloneData first —
 * the same FieldState write-rule normalisation (core/codeplug/
 * fieldstate.go's Valid: a non-'known' ToneField/BoolField must carry no
 * bare `value`) every other commit path in ChannelGrid already applies —
 * so a value the user never touched can never fail Go's validation.
 *
 *   - copy: target becomes a normalised copy of source's data; source is
 *     RE-SENT unchanged rather than omitted, deliberately — interpretation
 *     flagged in the report: this keeps one code path across all three
 *     actions and lets Go's single Validate pass see the true resulting
 *     state of BOTH slots together, at the cost of one harmless
 *     re-assignment of already-correct data.
 *   - swap: source and target exchange data outright — neither is lost.
 *   - move: target becomes source's data; source becomes empty
 *     (Channel.Data === null) — the same file-level "erase" delete.js
 *     uses (there is no CAT erase; the radio keeps the old contents until
 *     front-panel deletion — docs/hardware-notes.md).
 *
 * @param {'copy'|'swap'|'move'} action
 * @param {string} sourceSlot
 * @param {ChannelData} sourceData    always non-null: only a populated row can be dragged (beginDrag's own gate)
 * @param {string} targetSlot
 * @param {ChannelData | null} targetData
 * @returns {Channel[]}
 */
export function buildDragOpChannels(action, sourceSlot, sourceData, targetSlot, targetData) {
	const sourceCopy = /** @type {ChannelData} */ (cloneData(sourceData))
	switch (action) {
		case 'copy':
			return /** @type {Channel[]} */ ([
				{ slot: sourceSlot, data: sourceCopy },
				{ slot: targetSlot, data: sourceCopy },
			])
		case 'swap':
			return /** @type {Channel[]} */ ([
				{ slot: sourceSlot, data: targetData ? cloneData(targetData) : null },
				{ slot: targetSlot, data: sourceCopy },
			])
		case 'move':
			return /** @type {Channel[]} */ ([
				{ slot: sourceSlot, data: null },
				{ slot: targetSlot, data: sourceCopy },
			])
		default:
			throw new Error(`buildDragOpChannels: unknown action "${action}"`)
	}
}

/**
 * Resolves a typed slot-entry (the keyboard "Copy/Move to…" path's
 * display-form field, e.g. "M-50") against the CURRENT bank's own slots
 * only — matched case-insensitively/trimmed against either the SlotView's
 * Display string or its raw wire-form Slot. Refuses a read-only bank
 * outright (mirrors drag's own "read-only banks take no part" rule), an
 * empty entry, an unmatched entry, and the source slot itself (self-
 * target, the same rule as a self-drop).
 * @param {BankView} bank
 * @param {string} text
 * @param {string} excludeSlot
 * @returns {{ok: true, slot: string} | {ok: false, reason: string}}
 */
export function resolveTargetSlot(bank, text, excludeSlot) {
	if (bank.ReadOnly) {
		return { ok: false, reason: `the ${bank.Label} bank is read-only over CAT — nothing can be moved or copied here` }
	}
	const trimmed = text.trim()
	if (trimmed === '') return { ok: false, reason: 'enter a target slot, e.g. "M-50"' }
	const needle = trimmed.toLowerCase()
	const match = (bank.Slots ?? []).find(
		(sv) => sv.Display.toLowerCase() === needle || sv.Slot.toLowerCase() === needle
	)
	if (!match) return { ok: false, reason: `"${trimmed}" is not a slot in the ${bank.Label} bank` }
	if (match.Slot === excludeSlot) return { ok: false, reason: 'the source and target must be different slots' }
	return { ok: true, slot: match.Slot }
}

/**
 * Resolves the ↑/↓ row-action's neighbour (task-24 brief): the slot
 * immediately before ('up') or after ('down') `slot` in the CURRENT
 * bank's own visible Slots order. Refuses a read-only bank outright
 * (mirrors drag's own "read-only banks take no part" rule) WITHOUT
 * consulting `isPopulated` — a locked bank never reaches the point of
 * caring whether the neighbour holds a channel. A missing `slot` (not in
 * this bank at all) and either boundary (no previous slot for the
 * bank's own first row, no next slot for its last) both refuse.
 *
 * `isPopulated` is asked ONLY about the resolved neighbour slot, once
 * that slot is known — this module never touches ChannelData itself
 * (buildDragOpChannels already owns building the actual commit); the
 * caller decides the predicate however it likes (e.g. a channelBySlot
 * lookup).
 * @param {BankView} bank
 * @param {string} slot
 * @param {'up'|'down'} direction
 * @param {(neighbourSlot: string) => boolean} isPopulated
 * @returns {{ok: true, targetSlot: string, action: 'swap'|'move'} | {ok: false, reason: 'boundary'|'locked'|'not-found'}}
 */
export function resolveNeighbourSlot(bank, slot, direction, isPopulated) {
	if (bank.ReadOnly) return { ok: false, reason: 'locked' }
	const slots = bank.Slots ?? []
	const idx = slots.findIndex((sv) => sv.Slot === slot)
	if (idx === -1) return { ok: false, reason: 'not-found' }
	const targetIdx = direction === 'up' ? idx - 1 : idx + 1
	if (targetIdx < 0 || targetIdx >= slots.length) return { ok: false, reason: 'boundary' }
	const targetSlot = slots[targetIdx].Slot
	return { ok: true, targetSlot, action: isPopulated(targetSlot) ? 'swap' : 'move' }
}
