// SPDX-License-Identifier: GPL-3.0-or-later

// Pure module tests (task-22 brief §2/§4, test-FIRST): the drag gesture
// state machine (hover/leave/drop/cancel/self-drop/locked-target
// transitions), the copy/swap/move UpdateChannels payload builder, and
// the keyboard-path slot-entry resolver. No DOM, no component, no
// bridge — mirrors nav.js/paste.js's own test shape.

import { describe, it, expect } from 'vitest'
import {
	initialDragState,
	beginDrag,
	hoverDrag,
	leaveDrag,
	dropDrag,
	cancelDrag,
	buildDragOpChannels,
	resolveTargetSlot,
	resolveNeighbourSlot,
} from '../dragDrop.js'

describe('initialDragState', () => {
	it('starts idle', () => {
		expect(initialDragState()).toEqual({ sourceSlot: null, overSlot: null })
	})
})

describe('beginDrag', () => {
	it('starts a drag from a populated, unlocked row', () => {
		const result = beginDrag(initialDragState(), { slot: '001', locked: false, populated: true })
		expect(result).toEqual({ ok: true, state: { sourceSlot: '001', overSlot: null } })
	})

	it('refuses an empty slot (no populated-row source)', () => {
		const state = initialDragState()
		const result = beginDrag(state, { slot: '002', locked: false, populated: false })
		expect(result.ok).toBe(false)
		expect(result.state).toBe(state)
	})

	it('refuses a locked (read-only) bank', () => {
		const state = initialDragState()
		const result = beginDrag(state, { slot: '501', locked: true, populated: true })
		expect(result.ok).toBe(false)
		expect(result.state).toBe(state)
	})
})

describe('hoverDrag', () => {
	it('is a no-op when no drag is in progress', () => {
		const state = initialDragState()
		expect(hoverDrag(state, { slot: '002', locked: false })).toBe(state)
	})

	it('sets overSlot when hovering a valid candidate target', () => {
		const dragging = { sourceSlot: '001', overSlot: null }
		expect(hoverDrag(dragging, { slot: '002', locked: false })).toEqual({ sourceSlot: '001', overSlot: '002' })
	})

	it('clears overSlot (no highlight) when hovering the source row itself', () => {
		const dragging = { sourceSlot: '001', overSlot: '002' }
		expect(hoverDrag(dragging, { slot: '001', locked: false })).toEqual({ sourceSlot: '001', overSlot: null })
	})

	it('clears overSlot (no highlight) when hovering a locked target', () => {
		const dragging = { sourceSlot: '001', overSlot: '002' }
		expect(hoverDrag(dragging, { slot: '501', locked: true })).toEqual({ sourceSlot: '001', overSlot: null })
	})

	it('moving between two valid targets updates overSlot each time', () => {
		let state = { sourceSlot: '001', overSlot: null }
		state = hoverDrag(state, { slot: '002', locked: false })
		expect(state.overSlot).toBe('002')
		state = hoverDrag(state, { slot: '003', locked: false })
		expect(state.overSlot).toBe('003')
	})
})

describe('leaveDrag', () => {
	it('clears overSlot when leaving the CURRENT hover target', () => {
		const dragging = { sourceSlot: '001', overSlot: '002' }
		expect(leaveDrag(dragging, '002')).toEqual({ sourceSlot: '001', overSlot: null })
	})

	it('does nothing when leaving a row that is not the current overSlot (adjacent-row enter/leave firing order)', () => {
		// A dragenter on the NEW row can fire (setting overSlot to '003')
		// before the dragleave on the OLD row ('002') arrives — that late
		// dragleave must not clear the already-current '003' highlight.
		const dragging = { sourceSlot: '001', overSlot: '003' }
		const result = leaveDrag(dragging, '002')
		expect(result).toBe(dragging)
	})
})

describe('dropDrag', () => {
	it('succeeds for a genuine cross-slot drop on an unlocked target', () => {
		const dragging = { sourceSlot: '001', overSlot: '002' }
		const result = dropDrag(dragging, { slot: '002', locked: false })
		expect(result.ok).toBe(true)
		expect(result.source).toBe('001')
		expect(result.target).toBe('002')
		expect(result.state).toEqual({ sourceSlot: null, overSlot: null })
	})

	it('refuses (no-op) dropping on the source row itself', () => {
		const dragging = { sourceSlot: '001', overSlot: null }
		const result = dropDrag(dragging, { slot: '001', locked: false })
		expect(result.ok).toBe(false)
		expect(result.reason).toBe('self')
		expect(result.state).toEqual(initialDragState())
	})

	it('refuses dropping on a locked (read-only) target', () => {
		const dragging = { sourceSlot: '001', overSlot: '501' }
		const result = dropDrag(dragging, { slot: '501', locked: true })
		expect(result.ok).toBe(false)
		expect(result.reason).toBe('locked')
		expect(result.state).toEqual(initialDragState())
	})

	it('refuses a drop when no drag was in progress', () => {
		const result = dropDrag(initialDragState(), { slot: '002', locked: false })
		expect(result.ok).toBe(false)
		expect(result.reason).toBe('no-drag')
	})
})

describe('cancelDrag', () => {
	it('always resolves to idle', () => {
		expect(cancelDrag({ sourceSlot: '001', overSlot: '002' })).toEqual(initialDragState())
	})
})

describe('buildDragOpChannels', () => {
	const SOURCE_DATA = {
		freq_hz: 7074000,
		mode: 'USB',
		clar_hz: 0,
		rx_clar: false,
		tx_clar: false,
		ctcss: 'OFF',
		ctcss_tone: { state: 'unknown' },
		shift: 'SIMPLEX',
		tag: 'MYCALL',
		tag_display: true,
		scan_skip: { state: 'unknown' },
	}
	const TARGET_DATA = {
		freq_hz: 14074000,
		mode: 'FM',
		clar_hz: 10,
		rx_clar: true,
		tx_clar: false,
		ctcss: 'ENC',
		ctcss_tone: { state: 'known', value: 885 },
		shift: 'PLUS',
		tag: 'OTHER',
		tag_display: false,
		scan_skip: { state: 'known', value: true },
	}

	it('copy: target becomes a copy of source; source is re-sent unchanged (both slots in one batch)', () => {
		const channels = buildDragOpChannels('copy', '001', SOURCE_DATA, '002', TARGET_DATA)
		expect(channels).toEqual([
			{ slot: '001', data: SOURCE_DATA },
			{ slot: '002', data: SOURCE_DATA },
		])
	})

	it('swap: source and target exchange contents — neither channel is lost', () => {
		const channels = buildDragOpChannels('swap', '001', SOURCE_DATA, '002', TARGET_DATA)
		expect(channels).toEqual([
			{ slot: '001', data: TARGET_DATA },
			{ slot: '002', data: SOURCE_DATA },
		])
	})

	it('move: target becomes source; source becomes empty (Data: null) — the same file-level erase as delete', () => {
		const channels = buildDragOpChannels('move', '001', SOURCE_DATA, '002', TARGET_DATA)
		expect(channels).toEqual([
			{ slot: '001', data: null },
			{ slot: '002', data: SOURCE_DATA },
		])
	})

	it('copy into an empty target still sends both slots', () => {
		const channels = buildDragOpChannels('copy', '001', SOURCE_DATA, '002', null)
		expect(channels).toEqual([
			{ slot: '001', data: SOURCE_DATA },
			{ slot: '002', data: SOURCE_DATA },
		])
	})

	it('normalises optional FieldState keys the same way columns.js’s cloneData does (no bare value on a non-known state)', () => {
		const sparse = { ...SOURCE_DATA, ctcss_tone: { state: 'unknown', value: 0 } }
		const channels = buildDragOpChannels('move', '001', sparse, '002', TARGET_DATA)
		expect(channels[1].data.ctcss_tone).toEqual({ state: 'unknown' })
	})
})

describe('resolveTargetSlot', () => {
	const BANK = {
		ID: 'MEM',
		Label: 'Memories',
		ReadOnly: false,
		Slots: [
			{ Slot: '001', Display: 'M-01' },
			{ Slot: '050', Display: 'M-50' },
		],
	}
	const LOCKED_BANK = { ...BANK, ID: '60M', Label: '60 m channels', ReadOnly: true }

	it('resolves a display-form entry case-insensitively and trimmed', () => {
		expect(resolveTargetSlot(BANK, '  m-50  ', '001')).toEqual({ ok: true, slot: '050' })
	})

	it('resolves the raw wire-form slot too', () => {
		expect(resolveTargetSlot(BANK, '050', '001')).toEqual({ ok: true, slot: '050' })
	})

	it('refuses an empty entry', () => {
		const result = resolveTargetSlot(BANK, '', '001')
		expect(result.ok).toBe(false)
	})

	it('refuses a slot not in this bank', () => {
		const result = resolveTargetSlot(BANK, 'M-99', '001')
		expect(result.ok).toBe(false)
		expect(result.reason).toContain('M-99')
	})

	it('refuses the source slot itself (self-target, same rule as a self-drop)', () => {
		const result = resolveTargetSlot(BANK, 'M-01', '001')
		expect(result.ok).toBe(false)
	})

	it('refuses outright when the bank is read-only', () => {
		const result = resolveTargetSlot(LOCKED_BANK, 'M-01', '999')
		expect(result.ok).toBe(false)
		expect(result.reason).toContain('read-only')
	})
})

describe('resolveNeighbourSlot', () => {
	// task-24: the ↑/↓ row-action neighbour resolver. Populated/empty is
	// injected via a predicate rather than real channel data — this module
	// never touches ChannelData directly (buildDragOpChannels already owns
	// that), it only decides WHICH slot is the neighbour and whether the
	// resulting commit is a SWAP (populated neighbour) or a MOVE (empty
	// neighbour).
	const BANK = {
		ID: 'MEM',
		Label: 'Memories',
		ReadOnly: false,
		Slots: [
			{ Slot: '001', Display: 'M-01' },
			{ Slot: '002', Display: 'M-02' },
			{ Slot: '003', Display: 'M-03' },
		],
	}
	const LOCKED_BANK = { ...BANK, ID: '60M', Label: '60 m channels', ReadOnly: true }

	const ALL_POPULATED = () => true
	const ALL_EMPTY = () => false

	it('resolves the previous slot for "up" as a SWAP when it is populated', () => {
		expect(resolveNeighbourSlot(BANK, '002', 'up', ALL_POPULATED)).toEqual({
			ok: true,
			targetSlot: '001',
			action: 'swap',
		})
	})

	it('resolves the previous slot for "up" as a MOVE when it is empty', () => {
		expect(resolveNeighbourSlot(BANK, '002', 'up', ALL_EMPTY)).toEqual({
			ok: true,
			targetSlot: '001',
			action: 'move',
		})
	})

	it('resolves the next slot for "down" the same way', () => {
		expect(resolveNeighbourSlot(BANK, '002', 'down', ALL_POPULATED)).toEqual({
			ok: true,
			targetSlot: '003',
			action: 'swap',
		})
		expect(resolveNeighbourSlot(BANK, '002', 'down', ALL_EMPTY)).toEqual({
			ok: true,
			targetSlot: '003',
			action: 'move',
		})
	})

	it('only asks the predicate about the resolved NEIGHBOUR slot, not the source', () => {
		const asked = []
		resolveNeighbourSlot(BANK, '002', 'up', (slot) => {
			asked.push(slot)
			return true
		})
		expect(asked).toEqual(['001'])
	})

	it('refuses "up" from the first slot of the bank (boundary)', () => {
		const result = resolveNeighbourSlot(BANK, '001', 'up', ALL_POPULATED)
		expect(result).toEqual({ ok: false, reason: 'boundary' })
	})

	it('refuses "down" from the last slot of the bank (boundary)', () => {
		const result = resolveNeighbourSlot(BANK, '003', 'down', ALL_POPULATED)
		expect(result).toEqual({ ok: false, reason: 'boundary' })
	})

	it('refuses outright when the bank is read-only, without consulting the predicate', () => {
		let called = false
		const result = resolveNeighbourSlot(LOCKED_BANK, '002', 'up', () => {
			called = true
			return true
		})
		expect(result).toEqual({ ok: false, reason: 'locked' })
		expect(called).toBe(false)
	})

	it('refuses a slot that is not in this bank at all', () => {
		const result = resolveNeighbourSlot(BANK, '999', 'up', ALL_POPULATED)
		expect(result).toEqual({ ok: false, reason: 'not-found' })
	})
})
