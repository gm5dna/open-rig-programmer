// SPDX-License-Identifier: GPL-3.0-or-later

// Paste tests (task-17 brief §2: "unit-test it hard — quoted cells,
// embedded tabs, trailing newline, short rows"): parseBlock's TSV/CSV
// grammar, then mapPasteToChannels' mapping of a parsed block onto the
// visible columns starting at the focused cell — the pure core behind
// the grid's single UpdateChannels call.

import { describe, it, expect } from 'vitest'
import { parseBlock, mapPasteToChannels } from '../paste.js'
import { COLUMNS } from '../columns.js'

describe('parseBlock — delimiter detection', () => {
	it('splits on tabs when the text contains a tab (spreadsheet clipboard is TSV)', () => {
		expect(parseBlock('a\tb\nc\td')).toEqual([['a', 'b'], ['c', 'd']])
	})

	it('splits on commas otherwise', () => {
		expect(parseBlock('a,b\nc,d')).toEqual([['a', 'b'], ['c', 'd']])
	})

	it('a single cell parses as one row of one column', () => {
		expect(parseBlock('7.074')).toEqual([['7.074']])
	})
})

describe('parseBlock — quoting', () => {
	it('unwraps quoted cells', () => {
		expect(parseBlock('"a",b')).toEqual([['a', 'b']])
	})

	it('keeps a delimiter inside quotes as literal text', () => {
		expect(parseBlock('"a,b",c')).toEqual([['a,b', 'c']])
	})

	it('keeps an embedded tab inside a quoted TSV cell', () => {
		// The block itself is TSV (unquoted tab present); the quoted cell's
		// tab is data, not a delimiter.
		expect(parseBlock('"a\tb"\tc')).toEqual([['a\tb', 'c']])
	})

	it('keeps an embedded newline inside quotes as cell content', () => {
		expect(parseBlock('"a\nb",c\nd,e')).toEqual([['a\nb', 'c'], ['d', 'e']])
	})

	it('unescapes doubled quotes inside a quoted cell', () => {
		expect(parseBlock('"say ""hi""",x')).toEqual([['say "hi"', 'x']])
	})
})

describe('parseBlock — row shapes', () => {
	it('drops exactly one trailing newline (the standard clipboard tail) without inventing an empty row', () => {
		expect(parseBlock('a,b\n')).toEqual([['a', 'b']])
		expect(parseBlock('a\tb\r\n')).toEqual([['a', 'b']])
	})

	it('keeps a deliberate blank line as an empty row (only the final trailing newline is special)', () => {
		expect(parseBlock('a\n\nb')).toEqual([['a'], [''], ['b']])
	})

	it('preserves short rows as-is — no padding', () => {
		expect(parseBlock('a,b,c\nd\ne,f')).toEqual([['a', 'b', 'c'], ['d'], ['e', 'f']])
	})

	it('handles CRLF line endings', () => {
		expect(parseBlock('a,b\r\nc,d')).toEqual([['a', 'b'], ['c', 'd']])
	})

	it('returns no rows for empty or whitespace-free empty input', () => {
		expect(parseBlock('')).toEqual([])
		expect(parseBlock('\n')).toEqual([])
	})
})

// --- mapPasteToChannels ------------------------------------------------

/** Synthetic fixtures (MYCALL convention). */
const uiSpec = {
	Live: false,
	Banks: [],
	Modes: ['LSB', 'USB', 'FM'],
	ShiftOptions: ['SIMPLEX', 'PLUS', 'MINUS'],
	CTCSSStateOptions: ['OFF', 'ENC-DEC', 'ENC'],
	Tones: [{ Decihertz: 885, Display: '88.5 Hz' }],
	TagMaxBytes: 12,
	ClarMaxHz: 9990,
	ClarStepHz: 10,
}

const memBank = {
	ID: 'MEM',
	Label: 'Memories',
	ReadOnly: false,
	Slots: [
		{ Slot: '001', Display: 'M-01' },
		{ Slot: '002', Display: 'M-02' },
		{ Slot: '003', Display: 'M-03' },
	],
	// The FT-710 shape: the memory frame carries a display flag, so an
	// Added row states it and states it off (app/uispec.go's
	// bankTagDisplayDefault).
	TagDisplayDefault: { state: 'known', value: false },
}

/** memBank on a radio whose memory frame has NO display flag: every row
 * in it, added or existing, is Unavailable for tag_display. */
const noFlagBank = { ...memBank, TagDisplayDefault: { state: 'unavailable' } }

const lockedBank = {
	ID: '60M',
	Label: '60 m channels',
	ReadOnly: true,
	Slots: [{ Slot: '501', Display: '5-01' }],
	TagDisplayDefault: { state: 'known', value: false },
}

function populatedData(freqHz, mode) {
	return {
		freq_hz: freqHz,
		mode,
		clar_hz: 0,
		rx_clar: false,
		tx_clar: false,
		ctcss: 'OFF',
		ctcss_tone: { state: 'unknown' },
		shift: 'SIMPLEX',
		tag: 'MYCALL',
		tag_display: { state: 'known', value: false },
		scan_skip: { state: 'unknown' },
	}
}

/** channels: 001 populated, 002 EMPTY, 003 populated. */
function channelBySlot() {
	return new Map([
		['001', { slot: '001', data: populatedData(7074000, 'USB') }],
		['002', { slot: '002', data: null }],
		['003', { slot: '003', data: populatedData(14074000, 'USB') }],
	])
}

const FREQ_COL = COLUMNS.findIndex((c) => c.id === 'freq')
const MODE_COL = COLUMNS.findIndex((c) => c.id === 'mode')
const TONE_COL = COLUMNS.findIndex((c) => c.id === 'tone')
const TAG_COL = COLUMNS.findIndex((c) => c.id === 'tag')
const TAG_DISPLAY_COL = COLUMNS.findIndex((c) => c.id === 'tagDisplay')

function ctx(overrides = {}) {
	return {
		startRow: 0,
		startCol: FREQ_COL,
		bank: memBank,
		channelBySlot: channelBySlot(),
		uiSpec,
		...overrides,
	}
}

/** One paste row starting at FREQ_COL carrying the given per-column texts,
 * with every other cell empty (mapPasteToChannels skips those). Spelling
 * the sparse row out by hand would be nine mostly-empty cells whose
 * alignment is the whole point.
 * @param {Record<string, string>} cells   keyed by COLUMNS id
 * @returns {string[][]}
 */
function rowFrom(cells) {
	const row = []
	for (let c = FREQ_COL; c < COLUMNS.length; c++) row.push(cells[COLUMNS[c].id] ?? '')
	return [row]
}

describe('mapPasteToChannels — happy paths', () => {
	it('maps a rows×cols block onto the visible columns from the focused cell', () => {
		const rows = parseBlock('7.1\tFM\n14.2\tLSB')
		const result = mapPasteToChannels(rows, ctx())
		expect(result.ok).toBe(true)
		expect(result.channels).toHaveLength(2)
		const [first, second] = result.channels
		expect(first.slot).toBe('001')
		expect(first.data.freq_hz).toBe(7100000)
		expect(first.data.mode).toBe('FM')
		expect(first.data.tag).toBe('MYCALL') // untouched fields preserved
		// Row 2 lands on EMPTY slot 002: the pasted frequency populates it
		// (Added) with UISpec defaults for everything not pasted.
		expect(second.slot).toBe('002')
		expect(second.data.freq_hz).toBe(14200000)
		expect(second.data.mode).toBe('LSB')
		expect(second.data.ctcss).toBe('OFF')
		expect(second.data.shift).toBe('SIMPLEX')
	})

	it('skips empty cells, leaving those fields unchanged', () => {
		const rows = parseBlock('\tFM') // empty freq cell, mode FM
		const result = mapPasteToChannels(rows, ctx())
		expect(result.ok).toBe(true)
		expect(result.channels).toHaveLength(1)
		expect(result.channels[0].data.freq_hz).toBe(7074000) // untouched
		expect(result.channels[0].data.mode).toBe('FM')
	})

	it('clips rows past the end of the bank', () => {
		const rows = parseBlock('7.1\n7.2\n7.3\n7.4') // 4 rows from row 2 of 3
		const result = mapPasteToChannels(rows, ctx({ startRow: 1 }))
		expect(result.ok).toBe(true)
		expect(result.channels.map((c) => c.slot)).toEqual(['002', '003'])
	})

	it('clips columns past the last visible column', () => {
		// Start at Tag (col 8): tag, tagDisplay land; a third column clips.
		const rows = parseBlock('CALL 40M\ton\tOVERFLOW')
		const result = mapPasteToChannels(rows, ctx({ startCol: TAG_COL }))
		expect(result.ok).toBe(true)
		expect(result.channels[0].data.tag).toBe('CALL 40M')
		expect(result.channels[0].data.tag_display).toEqual({ state: 'known', value: true })
	})

	it('a fully clipped or all-empty paste yields ok with zero channels (no-op)', () => {
		expect(mapPasteToChannels([], ctx())).toEqual({ ok: true, channels: [] })
		const rows = parseBlock('7.1') // one row starting past the last bank row
		expect(mapPasteToChannels(rows, ctx({ startRow: 3 }))).toEqual({ ok: true, channels: [] })
	})
})

describe('mapPasteToChannels — rejections (whole paste, nothing applied)', () => {
	it('rejects any paste into a read-only bank', () => {
		const rows = parseBlock('5.3305')
		const result = mapPasteToChannels(rows, ctx({ bank: lockedBank, channelBySlot: new Map([['501', { slot: '501', data: populatedData(5330500, 'USB') }]]) }))
		expect(result.ok).toBe(false)
		expect(result.reason).toContain('read-only')
	})

	it('rejects a paste that touches the Slot column', () => {
		const rows = parseBlock('001\t7.1')
		const result = mapPasteToChannels(rows, ctx({ startCol: 0 }))
		expect(result.ok).toBe(false)
		expect(result.reason).toContain('Slot')
	})

	it('rejects a paste touching a Tone cell that is not editable (state unknown)', () => {
		const rows = parseBlock('88.5')
		const result = mapPasteToChannels(rows, ctx({ startCol: TONE_COL }))
		expect(result.ok).toBe(false)
		expect(result.reason).toContain('Tone')
	})

	it('accepts a Tone cell where the state is known', () => {
		const bySlot = channelBySlot()
		bySlot.get('001').data.ctcss_tone = { state: 'known', value: 670 }
		const rows = parseBlock('88.5')
		const result = mapPasteToChannels(rows, ctx({ startCol: TONE_COL, channelBySlot: bySlot }))
		expect(result.ok).toBe(true)
		expect(result.channels[0].data.ctcss_tone).toEqual({ state: 'known', value: 885 })
	})

	it('accepts a Tag display cell whose state is UNKNOWN — the deliberate asymmetry with tone/skip', () => {
		// M9c-5 E1: an Unknown tag display blocks its channel at plan time
		// ("set On or Off before sending"), so a paste must be able to SET
		// it — the bulk mitigation the design records for a column
		// isCellEditable leaves uneditable. Tone/skip, which the CAT
		// protocol cannot write at all when unknown, stay refused.
		const bySlot = channelBySlot()
		bySlot.get('001').data.tag_display = { state: 'unknown' }
		const rows = parseBlock('on')
		const result = mapPasteToChannels(rows, ctx({ startCol: TAG_DISPLAY_COL, channelBySlot: bySlot }))
		expect(result.ok).toBe(true)
		expect(result.channels[0].data.tag_display).toEqual({ state: 'known', value: true })
	})

	it('accepts a Tag display paste that POPULATES an empty slot in a bank whose radio carries the flag', () => {
		// The new row takes tag_display from the bank default (Known-off),
		// and the pasted 'on' then overrides it.
		const rows = rowFrom({ freq: '7.1', tagDisplay: 'on' })
		const result = mapPasteToChannels(rows, ctx({ startRow: 1 }))
		expect(result.ok).toBe(true)
		expect(result.channels[0].slot).toBe('002') // was EMPTY
		expect(result.channels[0].data.tag_display).toEqual({ state: 'known', value: true })
	})

	it('REFUSES a Tag display paste into a populated row whose state is UNAVAILABLE', () => {
		// {state:'unavailable'} means this radio's memory frame has no
		// display flag at all (core/codeplug/fieldstate.go). Unknown→Known
		// is a user decision the wire can carry; Unavailable→Known would
		// manufacture a value for a field that does not exist, and the whole
		// paste is refused rather than silently inventing one.
		const bySlot = channelBySlot()
		bySlot.get('001').data.tag_display = { state: 'unavailable' }
		const rows = parseBlock('on')
		const result = mapPasteToChannels(rows, ctx({ startCol: TAG_DISPLAY_COL, channelBySlot: bySlot, bank: noFlagBank }))
		expect(result.ok).toBe(false)
		expect(result.reason).toContain('M-01')
		expect(result.reason).toContain('Tag display')
		expect(result.reason).toContain('nothing was pasted')
	})

	it('REFUSES a Tag display paste that would populate an EMPTY slot in a bank with no display flag', () => {
		// The new row would take its tag_display from the bank default —
		// Unavailable — so the same refusal applies before any row is built.
		const rows = rowFrom({ freq: '7.1', tagDisplay: 'on' })
		const result = mapPasteToChannels(rows, ctx({ startRow: 1, bank: noFlagBank }))
		expect(result.ok).toBe(false)
		expect(result.reason).toContain('M-02')
		expect(result.reason).toContain('Tag display')
	})

	it('leaves a Tag display cell untouched when the paste does not reach that column', () => {
		// The refusal is per CELL PASTED, never a blanket ban on pasting
		// into a bank whose flag is Unavailable: a frequency-only paste onto
		// an Unavailable row still succeeds, carrying the state through.
		const bySlot = channelBySlot()
		bySlot.get('001').data.tag_display = { state: 'unavailable' }
		const rows = parseBlock('7.1')
		const result = mapPasteToChannels(rows, ctx({ channelBySlot: bySlot, bank: noFlagBank }))
		expect(result.ok).toBe(true)
		expect(result.channels[0].data.tag_display).toEqual({ state: 'unavailable' })
	})

	it('rejects an unparseable cell, naming the position', () => {
		const rows = parseBlock('7.1\n7.banana')
		const result = mapPasteToChannels(rows, ctx())
		expect(result.ok).toBe(false)
		expect(result.reason).toContain('M-02')
	})

	it('rejects a row that targets an empty slot without a parseable frequency', () => {
		const rows = parseBlock('FM') // mode only, onto empty slot 002
		const result = mapPasteToChannels(rows, ctx({ startRow: 1, startCol: MODE_COL }))
		expect(result.ok).toBe(false)
		expect(result.reason).toContain('M-02')
		expect(result.reason.toLowerCase()).toContain('empty')
	})
})
