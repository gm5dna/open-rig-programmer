// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it } from 'vitest'
import {
	COLUMNS,
	TIER_COLUMNS,
	cloneData,
	columnsFor,
	displayValue,
	isCellEditable,
	newChannelData,
	parsePasteCell,
} from '../columns.js'

/** The FT-710's MEM bank as GetUISpec serves it today: no tier field is
 * reachable, so BankView.Fields is empty. This is the shape every one of
 * the four registered models produces, on every bank. */
const ft710MemBank = {
	ID: 'MEM',
	Label: 'Memories',
	ReadOnly: false,
	Slots: [{ Slot: '001', Display: 'M-01' }],
	TagDisplayDefault: { state: 'known', value: false },
	Fields: [],
}

/** A hypothetical bank on a radio that DOES have some of the tier's
 * fields. No such driver exists; this is a fixture for the conditional
 * rendering rule, not a claim about any radio. */
const icomMemBank = {
	ID: 'MEM',
	Label: 'Memories',
	ReadOnly: false,
	Slots: [{ Slot: 'G01-001', Display: 'G01-001' }],
	TagDisplayDefault: { state: 'unavailable' },
	Fields: ['duplex', 'offset', 'tone_mode', 'tone_tx', 'data_mode'],
}

const uiSpec = {
	Modes: ['USB', 'FM'],
	ShiftOptions: ['SIMPLEX', 'PLUS', 'MINUS'],
	CTCSSStateOptions: ['OFF', 'ENC-DEC', 'ENC'],
	Tones: [{ Decihertz: 885, Display: '88.5 Hz' }],
}

describe('columnsFor', () => {
	// THE pin the Icom tier's frontend work exists to satisfy: with
	// today's four models the grid's column set is the ten it has always
	// had, in the order it has always had them. If a tier column ever
	// leaks into a Yaesu grid, this fails with the offending id named.
	it('serves the unchanged ten columns for an FT-710 bank', () => {
		expect(columnsFor(ft710MemBank)).toEqual(COLUMNS)
		expect(columnsFor(ft710MemBank).map((c) => c.id)).toEqual([
			'slot',
			'freq',
			'mode',
			'clar',
			'shift',
			'ctcss',
			'tone',
			'skip',
			'tag',
			'tagDisplay',
		])
	})

	it('serves the unchanged ten for a bank with no Fields at all', () => {
		expect(columnsFor({ ...ft710MemBank, Fields: undefined })).toEqual(COLUMNS)
		expect(columnsFor(null)).toEqual(COLUMNS)
		expect(columnsFor(undefined)).toEqual(COLUMNS)
	})

	it('appends only the tier columns the bank actually reaches, in TIER_COLUMNS order', () => {
		const ids = columnsFor(icomMemBank).map((c) => c.id)
		expect(ids.slice(0, COLUMNS.length)).toEqual(COLUMNS.map((c) => c.id))
		expect(ids.slice(COLUMNS.length)).toEqual([
			'duplex',
			'offset',
			'toneMode',
			'toneTx',
			'dataMode',
		])
	})

	it('never invents a column for a field the bank did not name', () => {
		const ids = columnsFor(icomMemBank).map((c) => c.id)
		expect(ids).not.toContain('txFreq')
		expect(ids).not.toContain('dtcsCode')
		expect(ids).not.toContain('filter')
	})

	it('pairs every tier column with the spec.Field of the same name', () => {
		for (const column of TIER_COLUMNS) {
			expect(column.field).toBe(column.key)
		}
	})
})

describe('tier cells', () => {
	const duplex = TIER_COLUMNS.find((c) => c.id === 'duplex')
	const offset = TIER_COLUMNS.find((c) => c.id === 'offset')
	const toneTx = TIER_COLUMNS.find((c) => c.id === 'toneTx')
	const dataMode = TIER_COLUMNS.find((c) => c.id === 'dataMode')

	it('renders a Known value and an em dash for every other state', () => {
		expect(displayValue(duplex, { duplex: { state: 'known', value: 'DUP-' } }, uiSpec)).toBe('DUP-')
		expect(displayValue(offset, { offset: { state: 'known', value: 600000 } }, uiSpec)).toBe('0.600000')
		expect(displayValue(toneTx, { tone_tx: { state: 'known', value: 885 } }, uiSpec)).toBe('88.5 Hz')
		expect(displayValue(dataMode, { data_mode: { state: 'known', value: true } }, uiSpec)).toBe('On')

		for (const state of ['unknown', 'unavailable', '']) {
			expect(displayValue(duplex, { duplex: { state } }, uiSpec)).toBe('—')
		}
		expect(displayValue(duplex, {}, uiSpec)).toBe('—')
	})

	it('is editable only from known or unknown, exactly as tag display is', () => {
		expect(isCellEditable(duplex, { duplex: { state: 'known', value: 'OFF' } })).toBe(true)
		expect(isCellEditable(duplex, { duplex: { state: 'unknown' } })).toBe(true)
		expect(isCellEditable(duplex, { duplex: { state: 'unavailable' } })).toBe(false)
		expect(isCellEditable(duplex, { duplex: { state: '' } })).toBe(false)
		expect(isCellEditable(duplex, null)).toBe(false)
	})

	it('parses a pasted cell per kind, and refuses what it cannot read', () => {
		expect(parsePasteCell(duplex, ' DUP+ ', uiSpec)).toEqual({
			ok: true,
			patch: { duplex: { state: 'known', value: 'DUP+' } },
		})
		expect(parsePasteCell(offset, '0.600000', uiSpec)).toEqual({
			ok: true,
			patch: { offset: { state: 'known', value: 600000 } },
		})
		expect(parsePasteCell(toneTx, '88.5 Hz', uiSpec)).toEqual({
			ok: true,
			patch: { tone_tx: { state: 'known', value: 885 } },
		})
		expect(parsePasteCell(dataMode, 'on', uiSpec)).toEqual({
			ok: true,
			patch: { data_mode: { state: 'known', value: true } },
		})
		expect(parsePasteCell(offset, 'not a frequency', uiSpec).ok).toBe(false)
		expect(parsePasteCell(dataMode, 'maybe', uiSpec).ok).toBe(false)
	})
})

describe('cloneData and newChannelData', () => {
	// The load-bearing one: a Yaesu channel arrives with all ten tier
	// fields 'unavailable', and a clone must carry them through
	// untouched. Inventing an 'unknown' fallback would turn an untouched
	// channel into a modified one the moment any cell was edited, because
	// Go compares ChannelData whole.
	it('carries the tier fields through a clone verbatim', () => {
		const data = {
			freq_hz: 14250000,
			mode: 'USB',
			clar_hz: 0,
			rx_clar: false,
			tx_clar: false,
			ctcss: 'OFF',
			ctcss_tone: { state: 'unknown' },
			shift: 'SIMPLEX',
			tag: 'CALLING',
			tag_display: { state: 'known', value: false },
			scan_skip: { state: 'unknown' },
			tx_frequency: { state: 'unavailable' },
			duplex: { state: 'unavailable' },
			offset: { state: 'unavailable' },
			tone_mode: { state: 'unavailable' },
			tone_tx: { state: 'unavailable' },
			tone_rx: { state: 'unavailable' },
			dtcs_code: { state: 'unavailable' },
			dtcs_polarity: { state: 'unavailable' },
			filter: { state: 'unavailable' },
			data_mode: { state: 'unavailable' },
		}
		expect(cloneData(data)).toEqual(data)
	})

	it('omits a tier key the incoming data does not carry', () => {
		const clone = cloneData({
			freq_hz: 14250000,
			mode: 'USB',
			ctcss: 'OFF',
			ctcss_tone: { state: 'unknown' },
			shift: 'SIMPLEX',
			tag_display: { state: 'known', value: false },
			scan_skip: { state: 'unknown' },
		})
		for (const column of TIER_COLUMNS) {
			expect(clone).not.toHaveProperty(column.key)
		}
	})

	it('gives a new FT-710 row exactly the keys it always had', () => {
		const data = newChannelData(uiSpec, ft710MemBank, 14250000)
		expect(Object.keys(data)).toEqual([
			'freq_hz',
			'mode',
			'clar_hz',
			'rx_clar',
			'tx_clar',
			'ctcss',
			'ctcss_tone',
			'shift',
			'tag',
			'tag_display',
			'scan_skip',
		])
	})

	it('starts a new row unknown for every tier field the bank reaches', () => {
		const data = newChannelData(uiSpec, icomMemBank, 145500000)
		expect(data.duplex).toEqual({ state: 'unknown' })
		expect(data.offset).toEqual({ state: 'unknown' })
		expect(data.tone_mode).toEqual({ state: 'unknown' })
		expect(data.data_mode).toEqual({ state: 'unknown' })
		// And nothing for a field this bank does not reach.
		expect(data).not.toHaveProperty('filter')
		expect(data).not.toHaveProperty('dtcs_code')
	})
})
