// SPDX-License-Identifier: GPL-3.0-or-later

// Pure paste logic (task-17 brief §2): parseBlock turns a clipboard
// string into rows×cells (TSV or CSV, quoted cells, embedded
// tabs/newlines, trailing newline, short rows); mapPasteToChannels maps
// a parsed block onto the visible columns starting at the focused cell
// and builds the affected channels for ONE UpdateChannels call — or
// rejects the WHOLE paste (nothing partially applied; Go's own
// UpdateChannels refuses whole batches the same way).
//
// Rejection/acceptance policy, documented for the component:
//   - A read-only bank, the Slot column, a Tone/Scan-skip cell whose
//     FieldState is not 'known', an unparseable cell, or a row that
//     targets an EMPTY slot without a parseable frequency ⇒ the whole
//     paste is rejected with a single readable reason.
//   - Rows past the end of the bank and columns past the last visible
//     column are clipped; empty cells leave that field unchanged.
//   - Parsing is formatting, not validation: vocabulary and ranges stay
//     with Go (a pasted mode of "XYZ" maps fine here and comes back as
//     a validation issue).

import { COLUMNS, cloneData, isCellEditable, newChannelData, parsePasteCell } from './columns.js'

/** @typedef {import('../../../wailsjs/go/models').codeplug.Channel} Channel */
/** @typedef {import('../../../wailsjs/go/models').main.BankView} BankView */
/** @typedef {import('../../../wailsjs/go/models').main.UISpecView} UISpecView */

/**
 * Parses a clipboard block into rows of cells.
 *
 * Grammar: tab-delimited when the text contains any tab (spreadsheet
 * clipboards are TSV), else comma-delimited. A cell starting with '"'
 * is quoted: delimiters/newlines inside are data, '""' is a literal
 * quote. Exactly one trailing newline (the standard clipboard tail) is
 * dropped; interior blank lines are kept as empty rows; short rows are
 * preserved as-is.
 * @param {string} text
 * @returns {string[][]}
 */
export function parseBlock(text) {
	if (!text) return []
	const delim = text.includes('\t') ? '\t' : ','

	let s = text
	if (s.endsWith('\r\n')) s = s.slice(0, -2)
	else if (s.endsWith('\n') || s.endsWith('\r')) s = s.slice(0, -1)
	if (s === '') return []

	/** @type {string[][]} */
	const rows = []
	/** @type {string[]} */
	let row = []
	let cell = ''
	let inQuotes = false

	const endCell = () => {
		row.push(cell)
		cell = ''
	}
	const endRow = () => {
		endCell()
		rows.push(row)
		row = []
	}

	for (let i = 0; i < s.length; i++) {
		const ch = s[i]
		if (inQuotes) {
			if (ch === '"') {
				if (s[i + 1] === '"') {
					cell += '"'
					i++
				} else {
					inQuotes = false
				}
			} else {
				cell += ch
			}
			continue
		}
		if (ch === '"' && cell === '') {
			inQuotes = true
		} else if (ch === delim) {
			endCell()
		} else if (ch === '\n') {
			endRow()
		} else if (ch === '\r') {
			endRow()
			if (s[i + 1] === '\n') i++
		} else {
			cell += ch
		}
	}
	endRow()
	return rows
}

/**
 * @typedef {Object} PasteContext
 * @property {number} startRow   focused cell's row within the bank
 * @property {number} startCol   focused cell's column index (COLUMNS order)
 * @property {BankView} bank     the active bank (ReadOnly + Slots)
 * @property {Map<string, Channel>} channelBySlot   working-copy channels keyed by slot
 * @property {UISpecView} uiSpec
 */

/**
 * Maps a parsed block onto the grid, building the affected channels.
 * See the module doc comment for the full policy.
 * @param {string[][]} rows
 * @param {PasteContext} context
 * @returns {{ok: true, channels: Channel[]} | {ok: false, reason: string}}
 */
export function mapPasteToChannels(rows, { startRow, startCol, bank, channelBySlot, uiSpec }) {
	if (bank.ReadOnly) {
		return { ok: false, reason: `the ${bank.Label} bank is read-only over CAT — nothing was pasted` }
	}
	const slots = bank.Slots ?? []
	/** @type {Channel[]} */
	const channels = []

	for (let r = 0; r < rows.length && startRow + r < slots.length; r++) {
		const slotView = slots[startRow + r]
		const existingData = channelBySlot.get(slotView.Slot)?.data ?? null

		/** @type {{column: typeof COLUMNS[number], patch: object}[]} */
		const patches = []
		/** @type {number | null} */
		let pastedFreqHz = null

		for (let c = 0; c < rows[r].length && startCol + c < COLUMNS.length; c++) {
			const text = rows[r][c]
			if (text === '') continue // empty cell: leave this field unchanged
			const column = COLUMNS[startCol + c]
			if (column.id === 'slot') {
				return { ok: false, reason: 'the Slot column cannot be pasted into' }
			}
			if ((column.id === 'tone' || column.id === 'skip') && !isCellEditable(column, existingData)) {
				return {
					ok: false,
					reason: `${slotView.Display}: the ${column.label} column is not settable over CAT for this channel — nothing was pasted`,
				}
			}
			const parsed = parsePasteCell(column, text, uiSpec)
			if (!parsed.ok) {
				return { ok: false, reason: `${slotView.Display}, ${column.label}: ${parsed.reason}` }
			}
			if (column.id === 'freq') {
				pastedFreqHz = /** @type {number} */ (parsed.patch.freq_hz)
			}
			patches.push({ column, patch: parsed.patch })
		}

		if (patches.length === 0) continue

		let data
		if (existingData == null) {
			if (pastedFreqHz === null) {
				return {
					ok: false,
					reason: `${slotView.Display} is empty — a paste must include a frequency to populate it`,
				}
			}
			data = newChannelData(uiSpec, pastedFreqHz)
		} else {
			data = cloneData(existingData)
		}
		for (const { patch } of patches) {
			Object.assign(data, patch)
		}
		channels.push(/** @type {Channel} */ ({ slot: slotView.Slot, data }))
	}

	return { ok: true, channels }
}
