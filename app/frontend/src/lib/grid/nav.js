// SPDX-License-Identifier: GPL-3.0-or-later

// Pure keyboard-navigation state machine for the channel grid (task-17
// brief §3): focused-cell transitions and the bank-tab reset. No DOM —
// the component translates keydown events into these calls and moves
// real focus itself.
//
// Contract: moveFocus returns the NEW position for a handled key, or
// null when the key is not a navigation key (the component must then
// leave the event alone) or when navigation would leave the grid
// (Tab/Shift-Tab past either end — deliberately NOT clamped, so the
// grid never becomes a keyboard trap: the component lets the browser
// move focus out instead).

/** @typedef {{row: number, col: number}} CellPos */
/** @typedef {{rows: number, cols: number}} GridDims */

/**
 * The first cell — also the reset position after a bank-tab switch.
 * @returns {CellPos}
 */
export function initialFocus() {
	return { row: 0, col: 0 }
}

/**
 * Applies one navigation key to a focused-cell position.
 * @param {CellPos} pos
 * @param {string} key   KeyboardEvent.key ('Tab' covers Shift-Tab via shiftKey)
 * @param {GridDims} dims
 * @param {boolean} [shiftKey]
 * @returns {CellPos | null} the new position, or null (unhandled / leaves the grid)
 */
export function moveFocus(pos, key, dims, shiftKey = false) {
	if (dims.rows <= 0 || dims.cols <= 0) return null
	const lastRow = dims.rows - 1
	const lastCol = dims.cols - 1

	switch (key) {
		case 'ArrowUp':
			return { row: Math.max(0, pos.row - 1), col: pos.col }
		case 'ArrowDown':
			return { row: Math.min(lastRow, pos.row + 1), col: pos.col }
		case 'ArrowLeft':
			return { row: pos.row, col: Math.max(0, pos.col - 1) }
		case 'ArrowRight':
			return { row: pos.row, col: Math.min(lastCol, pos.col + 1) }
		case 'Home':
			return { row: pos.row, col: 0 }
		case 'End':
			return { row: pos.row, col: lastCol }
		case 'Tab': {
			if (shiftKey) {
				if (pos.col > 0) return { row: pos.row, col: pos.col - 1 }
				if (pos.row > 0) return { row: pos.row - 1, col: lastCol }
				return null // first cell: let focus leave the grid backwards
			}
			if (pos.col < lastCol) return { row: pos.row, col: pos.col + 1 }
			if (pos.row < lastRow) return { row: pos.row + 1, col: 0 }
			return null // last cell: let focus leave the grid forwards
		}
		default:
			return null
	}
}

/**
 * Clamps a remembered position into new dimensions (bank switched to a
 * smaller bank, data reloaded). An empty grid collapses to the initial
 * cell so the position is always renderable once rows exist again.
 * @param {CellPos} pos
 * @param {GridDims} dims
 * @returns {CellPos}
 */
export function clampFocus(pos, dims) {
	if (dims.rows <= 0 || dims.cols <= 0) return initialFocus()
	return {
		row: Math.min(Math.max(0, pos.row), dims.rows - 1),
		col: Math.min(Math.max(0, pos.col), dims.cols - 1),
	}
}
