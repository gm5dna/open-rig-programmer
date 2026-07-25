// SPDX-License-Identifier: GPL-3.0-or-later

// Grid keyboard-nav state machine (task-17 brief §3): pure focused-cell
// transitions + bank-tab reset. The component maps DOM events onto these
// functions; everything here is plain data in, plain data out.

import { describe, it, expect } from 'vitest'
import { initialFocus, moveFocus, clampFocus } from '../nav.js'

const DIMS = { rows: 5, cols: 10 }

describe('initialFocus', () => {
	it('starts at the first cell — also the bank-tab reset position', () => {
		expect(initialFocus()).toEqual({ row: 0, col: 0 })
	})
})

describe('moveFocus — arrows', () => {
	it('moves one cell in each direction', () => {
		const pos = { row: 2, col: 4 }
		expect(moveFocus(pos, 'ArrowUp', DIMS)).toEqual({ row: 1, col: 4 })
		expect(moveFocus(pos, 'ArrowDown', DIMS)).toEqual({ row: 3, col: 4 })
		expect(moveFocus(pos, 'ArrowLeft', DIMS)).toEqual({ row: 2, col: 3 })
		expect(moveFocus(pos, 'ArrowRight', DIMS)).toEqual({ row: 2, col: 5 })
	})

	it('clamps at every edge instead of wrapping', () => {
		expect(moveFocus({ row: 0, col: 0 }, 'ArrowUp', DIMS)).toEqual({ row: 0, col: 0 })
		expect(moveFocus({ row: 0, col: 0 }, 'ArrowLeft', DIMS)).toEqual({ row: 0, col: 0 })
		expect(moveFocus({ row: 4, col: 9 }, 'ArrowDown', DIMS)).toEqual({ row: 4, col: 9 })
		expect(moveFocus({ row: 4, col: 9 }, 'ArrowRight', DIMS)).toEqual({ row: 4, col: 9 })
	})
})

describe('moveFocus — Home/End within the row', () => {
	it('Home jumps to the first column, End to the last, same row', () => {
		expect(moveFocus({ row: 3, col: 5 }, 'Home', DIMS)).toEqual({ row: 3, col: 0 })
		expect(moveFocus({ row: 3, col: 5 }, 'End', DIMS)).toEqual({ row: 3, col: 9 })
	})
})

describe('moveFocus — Tab/Shift-Tab across columns', () => {
	it('Tab moves right one column', () => {
		expect(moveFocus({ row: 1, col: 3 }, 'Tab', DIMS)).toEqual({ row: 1, col: 4 })
	})

	it('Tab wraps from the last column onto the next row', () => {
		expect(moveFocus({ row: 1, col: 9 }, 'Tab', DIMS)).toEqual({ row: 2, col: 0 })
	})

	it('Tab at the very last cell returns null — focus may leave the grid (no keyboard trap)', () => {
		expect(moveFocus({ row: 4, col: 9 }, 'Tab', DIMS)).toBeNull()
	})

	it('Shift-Tab mirrors: left, wrapping back onto the previous row, null at the first cell', () => {
		expect(moveFocus({ row: 1, col: 3 }, 'Tab', DIMS, true)).toEqual({ row: 1, col: 2 })
		expect(moveFocus({ row: 2, col: 0 }, 'Tab', DIMS, true)).toEqual({ row: 1, col: 9 })
		expect(moveFocus({ row: 0, col: 0 }, 'Tab', DIMS, true)).toBeNull()
	})
})

describe('moveFocus — unhandled keys', () => {
	it('returns null so the component leaves the event alone', () => {
		expect(moveFocus({ row: 1, col: 1 }, 'a', DIMS)).toBeNull()
		expect(moveFocus({ row: 1, col: 1 }, 'Enter', DIMS)).toBeNull()
		expect(moveFocus({ row: 1, col: 1 }, 'PageDown', DIMS)).toBeNull()
	})
})

describe('moveFocus — degenerate grids', () => {
	it('never yields a position inside an empty grid', () => {
		expect(moveFocus({ row: 0, col: 0 }, 'ArrowDown', { rows: 0, cols: 10 })).toBeNull()
	})
})

describe('clampFocus — bank switches and shrinking data', () => {
	it('keeps an in-range position unchanged', () => {
		expect(clampFocus({ row: 2, col: 4 }, DIMS)).toEqual({ row: 2, col: 4 })
	})

	it('clamps a position beyond the new dimensions (smaller bank)', () => {
		expect(clampFocus({ row: 90, col: 4 }, DIMS)).toEqual({ row: 4, col: 4 })
		expect(clampFocus({ row: 2, col: 12 }, DIMS)).toEqual({ row: 2, col: 9 })
	})

	it('collapses to the initial cell for an empty grid', () => {
		expect(clampFocus({ row: 3, col: 3 }, { rows: 0, cols: 10 })).toEqual({ row: 0, col: 0 })
	})
})
