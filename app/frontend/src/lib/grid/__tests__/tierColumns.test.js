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
 * reachable, so BankView.Fields is empty. This is the shape all four
 * Yaesu models produce, on every bank — and the contrast the IC-7610
 * fixture below is measured against. */
const ft710MemBank = {
	ID: 'MEM',
	Label: 'Memories',
	ReadOnly: false,
	Slots: [{ Slot: '001', Display: 'M-01' }],
	TagDisplayDefault: { state: 'known', value: false },
	Fields: [],
}

/** The REGISTERED IC-7610's MEM bank, exactly as GetUISpec serves it —
 * copied verbatim from the Go literal that Go test
 * TestGetUISpec_IC7610MEMBank_IsTheJSGridFixture (app/uispec_test.go)
 * pins against the real registered radio, so this fixture cannot drift
 * away from what the frontend is really handed. Update it from that
 * test's failure message, never by hand.
 *
 * This is the first fixture here that is a claim about a real radio: the
 * IC-7610's 1A 00 memory record maps four of the ten tier-added fields
 * (tone_mode, tone_tx, tone_rx, filter) and none of the other six, and
 * its record carries no display flag at all — hence the 'unavailable'
 * tag-display default. It replaces a HYPOTHETICAL bank this file used
 * while no Icom driver existed. */
const ic7610MemBank = {
	ID: 'MEM',
	Label: 'Memories',
	ReadOnly: false,
	Slots: [{ Slot: '001', Display: 'M-01' }],
	TagDisplayDefault: { state: 'unavailable' },
	Fields: ['tone_mode', 'tone_tx', 'tone_rx', 'filter'],
}

const receiverBank = {
	...ic7610MemBank,
	Fields: [
		'tuning_step_enabled',
		'tuning_step',
		'program_tuning_step',
		'attenuator',
		'preamp',
		'antenna',
		'ip_plus',
	],
}

const uiSpec = {
	Modes: ['USB', 'FM'],
	ShiftOptions: ['SIMPLEX', 'PLUS', 'MINUS'],
	CTCSSStateOptions: ['OFF', 'ENC-DEC', 'ENC'],
	Tones: [{ Decihertz: 885, Display: '88.5 Hz' }],
}

describe('columnsFor', () => {
	// THE pin the Icom tier's frontend work exists to satisfy: on a Yaesu
	// model the grid's column set is the ten it has always had, in the
	// order it has always had them — unchanged by the arrival of a radio
	// that does have tier fields. If a tier column ever leaks into a
	// Yaesu grid, this fails with the offending id named.
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
		const ids = columnsFor(ic7610MemBank).map((c) => c.id)
		expect(ids.slice(0, COLUMNS.length)).toEqual(COLUMNS.map((c) => c.id))
		// TIER_COLUMNS order, not the order Fields happens to list: the
		// IC-7610's four are positions 4, 5, 6 and 9 of the ten, so this
		// pins both the filtering and the ordering.
		expect(ids.slice(COLUMNS.length)).toEqual(['toneMode', 'toneTx', 'toneRx', 'filter'])
	})

	it('never invents a column for a field the bank did not name', () => {
		const ids = columnsFor(ic7610MemBank).map((c) => c.id)
		for (const id of ['txFreq', 'duplex', 'offset', 'dtcsCode', 'dtcsPolarity', 'dataMode']) {
			expect(ids, id).not.toContain(id)
		}
	})

	it('pairs every tier column with the spec.Field of the same name', () => {
		for (const column of TIER_COLUMNS) {
			expect(column.field).toBe(column.key)
		}
	})

	it('shows each receiver column only when the bank lists its field', () => {
		expect(columnsFor(receiverBank).slice(COLUMNS.length).map((c) => c.id)).toEqual([
			'tuningStepEnabled',
			'tuningStep',
			'programTuningStep',
			'attenuator',
			'preamp',
			'antenna',
			'ipPlus',
		])
		for (const id of ['tuningStepEnabled', 'tuningStep', 'programTuningStep', 'attenuator', 'preamp', 'antenna', 'ipPlus']) {
			expect(columnsFor(ic7610MemBank).map((c) => c.id)).not.toContain(id)
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

	it('is editable from known, unknown or absent — only unavailable stays refused (this task)', () => {
		expect(isCellEditable(duplex, { duplex: { state: 'known', value: 'OFF' } })).toBe(true)
		expect(isCellEditable(duplex, { duplex: { state: 'unknown' } })).toBe(true)
		// Absent: the key is simply missing from the channel — a Yaesu
		// channel with no tier fields at all, or a pre-tier file. The
		// column only renders where columnsFor says the radio HAS the
		// field, so a missing key here is a question the user may
		// legitimately answer, the same as 'unknown' is.
		expect(isCellEditable(duplex, { freq_hz: 14250000, mode: 'USB' })).toBe(true)
		expect(isCellEditable(duplex, {})).toBe(true)
		expect(isCellEditable(duplex, { duplex: { state: 'unavailable' } })).toBe(false)
		expect(isCellEditable(duplex, { duplex: { state: '' } })).toBe(false)
		// A wholly-empty slot (data null) is not "absent" — it is the
		// same "no populated channel" case every other column refuses
		// (isCellEditable's Slot/empty-slot rule); only Frequency opens
		// on it.
		expect(isCellEditable(duplex, null)).toBe(false)
	})

	it('leaves the non-tier columns unaffected by the absent rule', () => {
		// col('tag') is a plain COLUMNS entry (default case: editable iff
		// data != null) — a missing key on the data object is meaningless
		// to it, so this pins that the tier-only 'absent' branch has not
		// leaked into the switch below it.
		const tag = COLUMNS.find((c) => c.id === 'tag')
		expect(isCellEditable(tag, { freq_hz: 14250000, mode: 'USB' })).toBe(true)
		expect(isCellEditable(tag, null)).toBe(false)
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

describe('the edit flow on an absent tier cell', () => {
	// ChannelGrid.svelte's own commit handlers (commitSelectEditor,
	// commitTextEditor) all reduce to the same shape: gate with
	// isCellEditable, then `{ ...cloneData(fresh ?? data), [key]: value }`
	// — cloneData supplies the "fresh" channel, the column's own parser
	// supplies the value. No tier column is wired into that Svelte
	// template yet (grep confirms no tier id or `column.kind` appears
	// there), so this composes the same three exported, already-tested
	// primitives ChannelGrid would call, to pin the same three-part
	// behaviour the brief asks for: open, commit, cancel.
	const duplex = TIER_COLUMNS.find((c) => c.id === 'duplex')
	/** @type {Record<string, any>} */
	const absentChannel = { freq_hz: 14250000, mode: 'USB' } // no 'duplex' key: absent

	it('opens: an absent cell is editable and displays as an empty editor, same as unknown', () => {
		expect(isCellEditable(duplex, absentChannel)).toBe(true)
		expect(displayValue(duplex, absentChannel, uiSpec)).toBe('—')
	})

	it('commits: a value typed into an absent cell yields Known, same as committing over unknown does', () => {
		const parsed = parsePasteCell(duplex, 'DUP+', uiSpec)
		expect(parsed.ok).toBe(true)
		const committed = { ...cloneData(absentChannel), ...(parsed.ok ? parsed.patch : {}) }
		expect(committed.duplex).toEqual({ state: 'known', value: 'DUP+' })

		// The same commit starting from 'unknown' lands on the identical
		// shape — the two starting states are indistinguishable once
		// answered, exactly as tag_display's Known-off already is.
		const fromUnknown = { ...cloneData({ ...absentChannel, duplex: { state: 'unknown' } }), ...(parsed.ok ? parsed.patch : {}) }
		expect(fromUnknown.duplex).toEqual(committed.duplex)
	})

	it('cancels: no commit call leaves the field absent — cloneData never invents a key the source lacks', () => {
		// cancelEditor (ChannelGrid.svelte) sets editing = null and calls
		// nothing that touches data — so the only claim worth pinning here
		// is the one cloneData already makes doubly sure of: a clone taken
		// before any commit still omits the key entirely (already pinned
		// generally by "omits a tier key the incoming data does not
		// carry" above; this ties it to the specific absent-editing case).
		const clone = cloneData(absentChannel)
		expect(clone).not.toHaveProperty('duplex')
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
		const data = newChannelData(uiSpec, ic7610MemBank, 145500000)
		for (const key of ic7610MemBank.Fields) {
			expect(data[key], key).toEqual({ state: 'unknown' })
		}
		// And no key at all for a field this bank does not reach: absent,
		// never invented, which is what the Go side then resolves against
		// the radio's own capabilities (codeplug.NormaliseTierFields).
		for (const column of TIER_COLUMNS) {
			if (ic7610MemBank.Fields.includes(column.key)) continue
			expect(data, column.key).not.toHaveProperty(column.key)
		}
	})

	// The BANKLESS add, and the Go-side half it hands the work to (Wave 4
	// task R2, deviation (c)). A row added with no bank object at all —
	// there is nothing to ask about reachability — must omit every one of
	// the ten tier keys rather than default them: an omitted key decodes
	// in Go to the zero FieldState, Absent, which says "nobody has spoken
	// about this field", and ON THE NEXT LOAD of the saved file
	// codeplug.NormaliseTierFields resolves that to Unavailable for a
	// field the radio does not have while leaving it Absent for one it
	// does. Not before then: the in-memory add goes through
	// applyEditsLocked, which runs no such pass — the row stays exactly as
	// this factory built it until the file is saved and loaded again.
	// Manufacturing an 'unknown' here would destroy the distinction before
	// Go ever gets the chance.
	it('omits every tier key for a bankless add, whatever shape the missing bank takes', () => {
		for (const bank of [undefined, null, {}, { ID: 'MEM', Label: 'Memories', ReadOnly: false, Slots: [] }]) {
			const data = newChannelData(uiSpec, bank, 145500000)
			for (const column of TIER_COLUMNS) {
				expect(data, `${column.key} for bank ${JSON.stringify(bank)}`).not.toHaveProperty(column.key)
			}
			// The pre-tier keys are all still there: a bankless add is a
			// full channel, missing only what nobody can answer.
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
		}
	})
})
