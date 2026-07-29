// SPDX-License-Identifier: GPL-3.0-or-later

// Column model tests: definitions, per-cell editability, display
// formatting and per-column paste-cell parsing. All vocabulary
// (modes/shift/CTCSS-state/tones) and limits come from the UISpec
// fixture — never hardcoded in the module under test (task-17 brief,
// controller amendment).

import { describe, it, expect } from 'vitest'
import { COLUMNS, isCellEditable, displayValue, newChannelData, cloneData, parsePasteCell } from '../columns.js'

/** Synthetic UISpec (MYCALL-fixture convention — no real off-air data). */
const uiSpec = {
	Live: false,
	Banks: [],
	Modes: ['LSB', 'USB', 'CW-U', 'FM'],
	ShiftOptions: ['SIMPLEX', 'PLUS', 'MINUS'],
	CTCSSStateOptions: ['OFF', 'ENC-DEC', 'ENC'],
	Tones: [
		{ Decihertz: 670, Display: '67.0 Hz' },
		{ Decihertz: 885, Display: '88.5 Hz' },
	],
	TagMaxBytes: 12,
	ClarMaxHz: 9990,
	ClarStepHz: 10,
}

/** A populated channel's data, tone/skip unreadable over CAT (the state
 * every radio read produces for them — see core/driver/ft710/read.go).
 * Tag display is a BoolField too (M9c-5 E1), but the CAT protocol DOES
 * read it, so a read leaves it Known. */
function readData() {
	return {
		freq_hz: 7074000,
		mode: 'USB',
		clar_hz: 0,
		rx_clar: false,
		tx_clar: false,
		ctcss: 'OFF',
		ctcss_tone: { state: 'unknown' },
		shift: 'SIMPLEX',
		tag: 'MYCALL',
		tag_display: { state: 'known', value: true },
		scan_skip: { state: 'unknown' },
	}
}

/** The same channel as a file could carry it: tone/skip Known. */
function fileData() {
	const d = readData()
	d.ctcss_tone = { state: 'known', value: 885 }
	d.scan_skip = { state: 'known', value: true }
	return d
}

/** readData with tag_display in the given BoolField state — the four
 * cases this column now has to survive: Known-true, Known-false,
 * Unknown (a CHIRP import) and Unavailable (a radio whose frame carries
 * no display flag). @param {object} tagDisplay */
function withTagDisplay(tagDisplay) {
	const d = readData()
	d.tag_display = tagDisplay
	return d
}

const col = (id) => {
	const found = COLUMNS.find((c) => c.id === id)
	if (!found) throw new Error(`no column ${id}`)
	return found
}

describe('COLUMNS', () => {
	it('lists the ten brief-mandated columns in order', () => {
		expect(COLUMNS.map((c) => c.id)).toEqual([
			'slot', 'freq', 'mode', 'clar', 'shift', 'ctcss', 'tone', 'skip', 'tag', 'tagDisplay',
		])
	})

	it('keys every editable column to its IssueView field name', () => {
		expect(col('slot').field).toBeNull()
		expect(col('freq').field).toBe('frequency')
		expect(col('clar').field).toBe('clarifier')
		expect(col('ctcss').field).toBe('ctcss_state')
		expect(col('tone').field).toBe('ctcss_tone')
		expect(col('skip').field).toBe('scan_skip')
		expect(col('tagDisplay').field).toBe('tag_display')
	})
})

describe('isCellEditable', () => {
	it('the Slot column is never editable', () => {
		expect(isCellEditable(col('slot'), readData())).toBe(false)
		expect(isCellEditable(col('slot'), null)).toBe(false)
	})

	it('an empty slot is editable ONLY via Frequency (populating it)', () => {
		expect(isCellEditable(col('freq'), null)).toBe(true)
		for (const id of ['mode', 'clar', 'shift', 'ctcss', 'tone', 'skip', 'tag', 'tagDisplay']) {
			expect(isCellEditable(col(id), null), id).toBe(false)
		}
	})

	it('Tone/Scan skip are editable only when their FieldState is known', () => {
		expect(isCellEditable(col('tone'), readData())).toBe(false)
		expect(isCellEditable(col('skip'), readData())).toBe(false)
		expect(isCellEditable(col('tone'), fileData())).toBe(true)
		expect(isCellEditable(col('skip'), fileData())).toBe(true)
	})

	it('Tag display is editable only when its FieldState is known (all four states)', () => {
		expect(isCellEditable(col('tagDisplay'), withTagDisplay({ state: 'known', value: true }))).toBe(true)
		expect(isCellEditable(col('tagDisplay'), withTagDisplay({ state: 'known', value: false }))).toBe(true)
		expect(isCellEditable(col('tagDisplay'), withTagDisplay({ state: 'unknown' }))).toBe(false)
		expect(isCellEditable(col('tagDisplay'), withTagDisplay({ state: 'unavailable' }))).toBe(false)
	})

	it('every other column is editable on a populated channel', () => {
		for (const id of ['freq', 'mode', 'clar', 'shift', 'ctcss', 'tag']) {
			expect(isCellEditable(col(id), readData()), id).toBe(true)
		}
	})
})

describe('displayValue', () => {
	it('formats a populated channel', () => {
		const d = fileData()
		expect(displayValue(col('freq'), d, uiSpec)).toBe('7.074000')
		expect(displayValue(col('mode'), d, uiSpec)).toBe('USB')
		expect(displayValue(col('clar'), d, uiSpec)).toBe('0')
		expect(displayValue(col('shift'), d, uiSpec)).toBe('SIMPLEX')
		expect(displayValue(col('ctcss'), d, uiSpec)).toBe('OFF')
		expect(displayValue(col('tone'), d, uiSpec)).toBe('88.5 Hz')
		expect(displayValue(col('skip'), d, uiSpec)).toBe('On')
		expect(displayValue(col('tag'), d, uiSpec)).toBe('MYCALL')
		expect(displayValue(col('tagDisplay'), d, uiSpec)).toBe('On')
	})

	it('signs a non-zero clarifier', () => {
		const d = readData()
		d.clar_hz = 120
		expect(displayValue(col('clar'), d, uiSpec)).toBe('+120')
		d.clar_hz = -9990
		expect(displayValue(col('clar'), d, uiSpec)).toBe('-9990')
	})

	it('shows an em dash for unreadable tone/skip (state unknown or unavailable)', () => {
		const d = readData()
		expect(displayValue(col('tone'), d, uiSpec)).toBe('—')
		expect(displayValue(col('skip'), d, uiSpec)).toBe('—')
		d.ctcss_tone = { state: 'unavailable' }
		expect(displayValue(col('tone'), d, uiSpec)).toBe('—')
	})

	it('renders Tag display On/Off only when Known, an em dash otherwise (all four states)', () => {
		const show = (tagDisplay) => displayValue(col('tagDisplay'), withTagDisplay(tagDisplay), uiSpec)
		expect(show({ state: 'known', value: true })).toBe('On')
		expect(show({ state: 'known', value: false })).toBe('Off')
		// A Known-false BoolField arrives from Go with `value` omitted
		// (json:"value,omitempty") — that is Off, not unknown.
		expect(show({ state: 'known' })).toBe('Off')
		expect(show({ state: 'unknown' })).toBe('—')
		expect(show({ state: 'unavailable' })).toBe('—')
	})

	it('falls back to decihertz maths for a known tone missing from the table', () => {
		const d = readData()
		d.ctcss_tone = { state: 'known', value: 693 }
		expect(displayValue(col('tone'), d, uiSpec)).toBe('69.3 Hz')
	})

	it('renders empty strings for an empty slot', () => {
		for (const c of COLUMNS.filter((c) => c.id !== 'slot')) {
			expect(displayValue(c, null, uiSpec), c.id).toBe('')
		}
	})
})

describe('newChannelData', () => {
	it('builds Added-channel defaults entirely from the UISpec (no JS literals)', () => {
		const d = newChannelData(uiSpec, 7100000)
		expect(d).toEqual({
			freq_hz: 7100000,
			mode: uiSpec.Modes[0],
			clar_hz: 0,
			rx_clar: false,
			tx_clar: false,
			ctcss: uiSpec.CTCSSStateOptions[0],
			ctcss_tone: { state: 'unknown' },
			shift: uiSpec.ShiftOptions[0],
			tag: '',
			// A mandatory wire field: an Added row is Known-off, never
			// 'unknown' (which the send plan would block).
			tag_display: { state: 'known', value: false },
			scan_skip: { state: 'unknown' },
		})
	})
})

describe('cloneData', () => {
	it('deep-copies, never sharing the nested field-state objects', () => {
		const original = fileData()
		const copy = cloneData(original)
		expect(copy).toEqual(original)
		expect(copy).not.toBe(original)
		expect(copy.ctcss_tone).not.toBe(original.ctcss_tone)
		expect(copy.scan_skip).not.toBe(original.scan_skip)
		expect(copy.tag_display).not.toBe(original.tag_display)
	})

	it('carries every Tag display state through unchanged (all four states)', () => {
		for (const tagDisplay of [
			{ state: 'known', value: true },
			{ state: 'known', value: false },
			{ state: 'unknown' },
			{ state: 'unavailable' },
		]) {
			const copy = cloneData(withTagDisplay(tagDisplay))
			expect(copy.tag_display, tagDisplay.state).toEqual(tagDisplay)
		}
	})

	it('normalises absent optional fields to explicit zero values, omitting non-Known field-state values', () => {
		const sparse = { freq_hz: 7074000, mode: 'USB', ctcss: 'OFF', ctcss_tone: { state: 'unknown' }, shift: 'SIMPLEX', scan_skip: { state: 'unknown' } }
		const copy = cloneData(sparse)
		expect(copy.clar_hz).toBe(0)
		expect(copy.rx_clar).toBe(false)
		expect(copy.tx_clar).toBe(false)
		expect(copy.tag).toBe('')
		// Go always emits tag_display now; an absent one can only be a
		// hand-built shape, and the clone refuses to invent a value for it
		// exactly as it does for tone/scan skip.
		expect(copy.tag_display).toEqual({ state: 'unknown' })
		// codeplug.ToneField/BoolField.Valid: a non-Known state must carry a
		// ZERO value — the key must therefore stay absent, not default in.
		expect('value' in copy.ctcss_tone).toBe(false)
		expect('value' in copy.scan_skip).toBe(false)
	})
})

describe('parsePasteCell', () => {
	it('frequency: MHz text to integer Hz', () => {
		expect(parsePasteCell(col('freq'), '7.074', uiSpec)).toEqual({ ok: true, patch: { freq_hz: 7074000 } })
	})

	it('frequency: rejects unparseable text with a readable reason', () => {
		const r = parsePasteCell(col('freq'), 'seven', uiSpec)
		expect(r.ok).toBe(false)
		expect(r.reason).toContain('frequency')
	})

	it('mode/shift/CTCSS pass through trimmed — Go validates vocabulary', () => {
		expect(parsePasteCell(col('mode'), ' FM ', uiSpec)).toEqual({ ok: true, patch: { mode: 'FM' } })
		expect(parsePasteCell(col('shift'), 'PLUS', uiSpec)).toEqual({ ok: true, patch: { shift: 'PLUS' } })
		expect(parsePasteCell(col('ctcss'), 'ENC', uiSpec)).toEqual({ ok: true, patch: { ctcss: 'ENC' } })
	})

	it('clarifier: signed integer Hz only', () => {
		expect(parsePasteCell(col('clar'), '-120', uiSpec)).toEqual({ ok: true, patch: { clar_hz: -120 } })
		expect(parsePasteCell(col('clar'), '+120', uiSpec)).toEqual({ ok: true, patch: { clar_hz: 120 } })
		expect(parsePasteCell(col('clar'), '1.5', uiSpec).ok).toBe(false)
	})

	it('tone: Hz text (with or without the unit) to a Known decihertz ToneField', () => {
		expect(parsePasteCell(col('tone'), '88.5', uiSpec)).toEqual({ ok: true, patch: { ctcss_tone: { state: 'known', value: 885 } } })
		expect(parsePasteCell(col('tone'), '67.0 Hz', uiSpec)).toEqual({ ok: true, patch: { ctcss_tone: { state: 'known', value: 670 } } })
		expect(parsePasteCell(col('tone'), 'high', uiSpec).ok).toBe(false)
	})

	it('booleans: on/off vocabulary for Scan skip and Tag display', () => {
		expect(parsePasteCell(col('skip'), 'on', uiSpec)).toEqual({ ok: true, patch: { scan_skip: { state: 'known', value: true } } })
		expect(parsePasteCell(col('skip'), 'No', uiSpec)).toEqual({ ok: true, patch: { scan_skip: { state: 'known', value: false } } })
		// Tag display parses to a KNOWN BoolField: pasting on/off is how a
		// user makes an Unknown display honest (paste.js refuses only
		// tone/skip cells, so this route stays open where the cell editor
		// is closed).
		expect(parsePasteCell(col('tagDisplay'), 'TRUE', uiSpec)).toEqual({ ok: true, patch: { tag_display: { state: 'known', value: true } } })
		expect(parsePasteCell(col('tagDisplay'), '0', uiSpec)).toEqual({ ok: true, patch: { tag_display: { state: 'known', value: false } } })
		expect(parsePasteCell(col('tagDisplay'), 'maybe', uiSpec).ok).toBe(false)
		expect(parsePasteCell(col('skip'), 'maybe', uiSpec).ok).toBe(false)
	})

	it('tag: exact text, untrimmed and unclamped — Go validates length/charset', () => {
		expect(parsePasteCell(col('tag'), ' MYCALL 40M X ', uiSpec)).toEqual({ ok: true, patch: { tag: ' MYCALL 40M X ' } })
	})

	it('slot: never pasteable', () => {
		expect(parsePasteCell(col('slot'), '001', uiSpec).ok).toBe(false)
	})
})
