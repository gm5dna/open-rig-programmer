// SPDX-License-Identifier: GPL-3.0-or-later

// ChannelGrid component tests (task-17 brief §3): mocked bridge, real
// appState singleton. Fixtures are synthetic (MYCALL convention).

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/svelte'
import { appState } from '../state/app.svelte.js'
import ChannelGrid from '../ChannelGrid.svelte'

vi.mock('../bridge/bindings.js', () => ({
	updateChannel: vi.fn().mockResolvedValue({ Issues: [], Dirty: true }),
	updateChannels: vi.fn().mockResolvedValue({ Issues: [], Dirty: true }),
}))

import { updateChannel, updateChannels } from '../bridge/bindings.js'

const updateChannelMock = vi.mocked(updateChannel)
const updateChannelsMock = vi.mocked(updateChannels)

const UI_SPEC = {
	Live: false,
	Banks: [
		{
			ID: 'MEM',
			Label: 'Memories',
			ReadOnly: false,
			Slots: [
				{ Slot: '001', Display: 'M-01' },
				{ Slot: '002', Display: 'M-02' },
				{ Slot: '003', Display: 'M-03' },
			],
		},
		{
			ID: 'PMS',
			Label: 'Scan limits (PMS)',
			ReadOnly: false,
			Slots: [
				{ Slot: 'P1L', Display: 'P1L' },
				{ Slot: 'P1U', Display: 'P1U' },
			],
		},
		{
			ID: '60M',
			Label: '60 m channels',
			ReadOnly: true,
			Slots: [
				{ Slot: '501', Display: '5-01' },
				{ Slot: '502', Display: '5-02' },
			],
		},
	],
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
	// Task 42 (M9a-6): the radiotext-sourced prose fields — byte-identical
	// to internal/radiotext.go's ft710Text, so the component tests below
	// keep asserting the real, end-to-end FT-710 wording even though the
	// component itself no longer hardcodes any of it.
	ToneScanSkipNote: "Tone and Scan Skip aren't carried by the FT-710's CAT protocol — set them on the radio.",
	// m42a: the grid-legend's second sentence, previously hardcoded in
	// ChannelGrid.svelte itself — now served the same way ToneScanSkipNote
	// is, so this fixture is the only place either sentence is written out.
	ToneScanSkipVerification:
		"Preservation across a rewrite is hardware-verified for Tone; Scan Skip preservation is not yet verified (see each cell's tooltip).",
	EraseDialogNote:
		'The FT-710 has no CAT erase command. To delete a channel on the radio: press and hold [V/M] to open the memory channel list, select the channel, then touch [ERASE].',
	PreservationTooltips: {
		Tone: 'not readable over CAT — preserved when writing (hardware-verified 13/07/2026)',
		ScanSkip: 'not readable over CAT — preservation when writing is unverified (never probed)',
	},
	FirmwarePlaceholder: 'e.g. V01-10',
}

function data(freqHz, mode, extra = {}) {
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
		tag_display: true,
		scan_skip: { state: 'unknown' },
		...extra,
	}
}

function codeplugFixture() {
	return {
		Schema: 1,
		Generator: 'test',
		Radio: { model: 'FT-710', cat_id: '0800', read_at: null, region: 'GB' },
		Channels: [
			{ slot: '001', data: data(7074000, 'USB') },
			{ slot: '002', data: null }, // empty slot
			{ slot: '003', data: data(14074000, 'FM', { ctcss_tone: { state: 'known', value: 885 }, scan_skip: { state: 'known', value: false } }) },
			{ slot: 'P1L', data: data(7000000, 'LSB') },
			{ slot: 'P1U', data: data(7200000, 'LSB') },
			{ slot: '501', data: data(5330500, 'USB') },
			{ slot: '502', data: data(5346500, 'USB') },
		],
		WorkingPath: '',
		Dirty: false,
		BaselineStale: false,
	}
}

beforeEach(() => {
	appState.clearConnection()
	appState.setUISpec(UI_SPEC)
	appState.setCodeplug(codeplugFixture())
	appState.setIssues([])
	appState.alerts = []
	vi.clearAllMocks()
	updateChannelMock.mockResolvedValue({ Issues: [], Dirty: true })
	updateChannelsMock.mockResolvedValue({ Issues: [], Dirty: true })
})

/** @param {HTMLElement} container @param {number} row @param {number} col */
function cell(container, row, col) {
	const el = container.querySelector(`td[data-row="${row}"][data-col="${col}"]`)
	if (!(el instanceof HTMLElement)) throw new Error(`no cell ${row},${col}`)
	return el
}

const FREQ = 1
const TAG = 8
const TAG_DISPLAY = 9
const TONE = 6
const SKIP = 7

describe('bank tabs', () => {
	it('renders one tab per UISpec bank inside a labelled tablist', () => {
		render(ChannelGrid)
		expect(screen.getByRole('tablist', { name: 'Memory banks' })).toBeInTheDocument()
		expect(screen.getByRole('tab', { name: /Memories/ })).toBeInTheDocument()
		expect(screen.getByRole('tab', { name: /Scan limits \(PMS\)/ })).toBeInTheDocument()
		expect(screen.getByRole('tab', { name: /60 m channels/ })).toBeInTheDocument()
	})

	it('shows a per-bank issue-count chip only when non-zero', () => {
		appState.setIssues([
			{ Slot: '001', Field: 'frequency', Severity: 'error', Msg: 'bad freq' },
			{ Slot: '001', Field: 'tag', Severity: 'warning', Msg: 'odd tag' },
			{ Slot: '501', Field: 'mode', Severity: 'warning', Msg: 'odd mode' },
		])
		render(ChannelGrid)
		expect(screen.getByRole('tab', { name: /Memories 2 issues/ })).toBeInTheDocument()
		expect(screen.getByRole('tab', { name: /60 m channels.*1 issue/ })).toBeInTheDocument()
		expect(screen.getByRole('tab', { name: /Scan limits/ }).textContent).not.toMatch(/\d/)
	})

	it('completes the tab pattern: aria-controls, roving tabindex, arrow-key selection', async () => {
		render(ChannelGrid)
		const mem = screen.getByRole('tab', { name: /Memories/ })
		const pms = screen.getByRole('tab', { name: /Scan limits/ })
		expect(mem).toHaveAttribute('aria-selected', 'true')
		expect(mem).toHaveAttribute('aria-controls', 'channel-grid-panel')
		expect(mem).toHaveAttribute('tabindex', '0')
		expect(pms).toHaveAttribute('tabindex', '-1')

		await fireEvent.keyDown(mem, { key: 'ArrowRight' })
		expect(screen.getByRole('tab', { name: /Scan limits/ })).toHaveAttribute('aria-selected', 'true')
		const panel = screen.getByRole('tabpanel')
		expect(panel).toHaveAttribute('aria-labelledby', 'bank-tab-PMS')
	})

	it('switching tab shows that bank’s rows and resets the focused cell', async () => {
		const { container } = render(ChannelGrid)
		// Move focus away from the first cell first (fireEvent so Svelte's
		// flush is awaited before asserting on the roving tabindex).
		const c = cell(container, 1, FREQ)
		c.focus()
		await fireEvent.focus(c)
		expect(c).toHaveAttribute('tabindex', '0')

		await fireEvent.click(screen.getByRole('tab', { name: /Scan limits/ }))
		expect(screen.getByText('P1L')).toBeInTheDocument()
		expect(screen.queryByText('M-01')).not.toBeInTheDocument()
		// Bank-tab reset: the roving tabindex is back on the first cell.
		expect(cell(container, 0, 0)).toHaveAttribute('tabindex', '0')
	})
})

describe('rows and cells', () => {
	it('renders one row per slot of the active bank, with display slots and formatted frequency', () => {
		const { container } = render(ChannelGrid)
		expect(screen.getByText('M-01')).toBeInTheDocument()
		expect(screen.getByText('M-02')).toBeInTheDocument()
		expect(cell(container, 0, FREQ).textContent).toBe('7.074000')
		expect(cell(container, 0, TAG).textContent).toBe('MYCALL')
	})

	it('dims empty slots with an "empty" affordance', () => {
		const { container } = render(ChannelGrid)
		const emptyRow = cell(container, 1, 0).closest('tr')
		expect(emptyRow?.classList.contains('row-empty')).toBe(true)
		expect(cell(container, 1, FREQ).textContent).toBe('empty')
	})

	it('keeps the slot cell text exactly the display slot — the row-action icons live in their own trailing column (layout fix)', () => {
		const { container } = render(ChannelGrid)
		// A populated row whose actions ARE available: the slot cell must
		// still contain nothing but the display slot (the earlier absolute
		// overlay inside this cell could overlap/truncate the label —
		// caught by Stuart's live session).
		expect(cell(container, 0, 0).textContent?.trim()).toBe('M-01')

		const deleteBtn = screen.getByRole('button', { name: 'Delete M-01' })
		const actionsCell = deleteBtn.closest('td')
		expect(actionsCell?.classList.contains('col-actions')).toBe(true)
		expect(actionsCell).not.toBe(cell(container, 0, 0))
		// The trailing column is real table structure: one extra header cell
		// and one extra cell per row beyond the ten data COLUMNS.
		expect(container.querySelectorAll('thead th')).toHaveLength(11)
		expect(cell(container, 0, 0).closest('tr')?.querySelectorAll('td')).toHaveLength(11)
	})

	it('greys unreadable Tone/Scan skip with per-column preservation tooltips (M5b: tone preservation hardware-verified, skip never probed)', () => {
		const { container } = render(ChannelGrid)
		const tone = cell(container, 0, TONE)
		expect(tone.classList.contains('cell-preserved')).toBe(true)
		expect(tone.getAttribute('title')).toBe(
			'not readable over CAT — preserved when writing (hardware-verified 13/07/2026)'
		)
		const skip = cell(container, 0, SKIP)
		expect(skip.classList.contains('cell-preserved')).toBe(true)
		expect(skip.getAttribute('title')).toBe(
			'not readable over CAT — preservation when writing is unverified (never probed)'
		)
		// Row 2 (slot 003) carries a Known tone from a file: not greyed.
		expect(cell(container, 2, TONE).classList.contains('cell-preserved')).toBe(false)
		expect(cell(container, 2, TONE).textContent).toBe('88.5 Hz')
	})

	it('task 42: an empty served PreservationTooltips renders no tooltip text, not a hardcoded fallback', () => {
		appState.setUISpec({ ...UI_SPEC, PreservationTooltips: { Tone: '', ScanSkip: '' } })
		const { container } = render(ChannelGrid)
		expect(cell(container, 0, TONE).getAttribute('title')).toBe('')
		expect(cell(container, 0, SKIP).getAttribute('title')).toBe('')
	})
})

describe('keyboard navigation', () => {
	it('arrows move the roving focus between cells', async () => {
		const { container } = render(ChannelGrid)
		const start = cell(container, 0, FREQ)
		start.focus()
		await fireEvent.keyDown(start, { key: 'ArrowDown' })
		expect(cell(container, 1, FREQ)).toHaveAttribute('tabindex', '0')
		expect(document.activeElement).toBe(cell(container, 1, FREQ))
	})
})

describe('edit queue — out-of-order regression (Fix 3, adjudicated MED, Codex M6 #3)', () => {
	it('two rapid edits to different fields of the same slot both survive, even with deliberately delayed/out-of-order mock responses', async () => {
		/** @type {(() => void)[]} */
		const releases = []
		updateChannelMock.mockImplementation(
			(channel) =>
				new Promise((resolve) => {
					// Mirrors the real bridge (bindings.js's updateChannel):
					// mirror the sent channel into appState.codeplug on
					// success — this test mocks bindings.js entirely, so it
					// must reproduce that side effect for a second queued
					// edit's "freshest data" read to see anything real.
					releases.push(() => {
						appState.applyChannelEdits([channel])
						resolve({ Issues: [], Dirty: true })
					})
				})
		)

		const { container } = render(ChannelGrid)

		// Commit a Frequency edit on slot 001 (7.074000 -> 7.100000).
		const freqCell = cell(container, 0, FREQ)
		freqCell.focus()
		await fireEvent.keyDown(freqCell, { key: 'Enter' })
		let input = screen.getByRole('textbox')
		await fireEvent.input(input, { target: { value: '7.1' } })
		await fireEvent.keyDown(input, { key: 'Enter' })

		// BEFORE that first commit resolves, commit a Tag edit on the SAME
		// slot (MYCALL -> NEWCALL).
		const tagCell = cell(container, 0, TAG)
		tagCell.focus()
		await fireEvent.keyDown(tagCell, { key: 'Enter' })
		input = screen.getByRole('textbox')
		await fireEvent.input(input, { target: { value: 'NEWCALL' } })
		await fireEvent.keyDown(input, { key: 'Enter' })

		// The queue must serialise: only one Go call in flight at a time.
		expect(updateChannelMock).toHaveBeenCalledTimes(1)

		// Resolve the first (frequency) call.
		releases[0]()
		await Promise.resolve()
		await Promise.resolve()

		// The SECOND call must now have been dispatched, materialised from
		// the FRESHEST data at ITS OWN dequeue time — carrying BOTH the
		// frequency edit (already applied by the first) and the tag edit.
		// The pre-fix fire-and-forget shape built this payload from the
		// STALE pre-edit-1 base instead, losing the frequency change here.
		expect(updateChannelMock).toHaveBeenCalledTimes(2)
		const secondPayload = updateChannelMock.mock.calls[1][0]
		expect(secondPayload.data.freq_hz).toBe(7100000)
		expect(secondPayload.data.tag).toBe('NEWCALL')

		releases[1]()
		await Promise.resolve()

		expect(appState.codeplug.Channels[0].data.freq_hz).toBe(7100000)
		expect(appState.codeplug.Channels[0].data.tag).toBe('NEWCALL')
	})
})

describe('editing', () => {
	it('Enter opens the frequency editor prefilled; Enter commits the FULL edited channel', async () => {
		const { container } = render(ChannelGrid)
		const freqCell = cell(container, 0, FREQ)
		freqCell.focus()
		await fireEvent.keyDown(freqCell, { key: 'Enter' })

		const input = screen.getByRole('textbox', { name: 'Frequency (MHz), M-01' })
		expect(input).toHaveValue('7.074000')

		await fireEvent.input(input, { target: { value: '7.2' } })
		await fireEvent.keyDown(input, { key: 'Enter' })

		expect(updateChannel).toHaveBeenCalledTimes(1)
		expect(updateChannel).toHaveBeenCalledWith({
			slot: '001',
			data: {
				freq_hz: 7200000,
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
			},
		})
	})

	it('commits exactly once and the cell re-renders when the bridge mirrors the edit back', async () => {
		// Mirror the real bridge: a successful update rewrites the codeplug
		// view. In a real browser (verified live via a dev harness), the
		// committing Enter would — without stopPropagation in the editor
		// keydown — bubble to the cell handler, reopen the editor with the
		// PRE-edit value and re-commit the old data on the follow-up blur.
		// happy-dom does not reproduce that exact event/focus interleaving,
		// so this test pins the required END state (one call, updated
		// display, no lingering editor) rather than the failure mechanism.
		updateChannelMock.mockImplementation(async (ch) => {
			appState.applyChannelEdits([ch])
			return { Issues: [], Dirty: true }
		})
		const { container } = render(ChannelGrid)
		const freqCell = cell(container, 0, FREQ)
		freqCell.focus()
		await fireEvent.keyDown(freqCell, { key: 'Enter' })
		const input = screen.getByRole('textbox', { name: 'Frequency (MHz), M-01' })
		await fireEvent.input(input, { target: { value: '7.2' } })
		await fireEvent.keyDown(input, { key: 'Enter' })
		await Promise.resolve() // let the mocked bridge promise settle

		expect(updateChannel).toHaveBeenCalledTimes(1)
		expect(cell(container, 0, FREQ).textContent).toBe('7.200000')
		expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
	})

	it('Escape reverts without committing', async () => {
		const { container } = render(ChannelGrid)
		const freqCell = cell(container, 0, FREQ)
		freqCell.focus()
		await fireEvent.keyDown(freqCell, { key: 'Enter' })
		const input = screen.getByRole('textbox', { name: 'Frequency (MHz), M-01' })
		await fireEvent.input(input, { target: { value: '9.9' } })
		await fireEvent.keyDown(input, { key: 'Escape' })

		expect(updateChannel).not.toHaveBeenCalled()
		expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
		expect(cell(container, 0, FREQ).textContent).toBe('7.074000')
	})

	it('an unchanged commit is skipped (no call, no dirty)', async () => {
		const { container } = render(ChannelGrid)
		const freqCell = cell(container, 0, FREQ)
		freqCell.focus()
		await fireEvent.keyDown(freqCell, { key: 'Enter' })
		await fireEvent.keyDown(screen.getByRole('textbox'), { key: 'Enter' })
		expect(updateChannel).not.toHaveBeenCalled()
	})

	it('typing into an empty slot’s frequency cell opens the editor prefilled and populates it (Added)', async () => {
		const { container } = render(ChannelGrid)
		const emptyFreq = cell(container, 1, FREQ)
		emptyFreq.focus()
		await fireEvent.keyDown(emptyFreq, { key: '7' })

		const input = screen.getByRole('textbox', { name: 'Frequency (MHz), M-02' })
		expect(input).toHaveValue('7')

		await fireEvent.input(input, { target: { value: '7.2' } })
		await fireEvent.keyDown(input, { key: 'Enter' })

		// A new channel from UISpec defaults: neutral vocabulary entries,
		// tone/skip unknown (preserve whatever the radio has).
		expect(updateChannel).toHaveBeenCalledWith({
			slot: '002',
			data: {
				freq_hz: 7200000,
				mode: 'LSB',
				clar_hz: 0,
				rx_clar: false,
				tx_clar: false,
				ctcss: 'OFF',
				ctcss_tone: { state: 'unknown' },
				shift: 'SIMPLEX',
				tag: '',
				tag_display: false,
				scan_skip: { state: 'unknown' },
			},
		})
	})

	it('Enter on a toggle cell commits immediately (Tag display)', async () => {
		const { container } = render(ChannelGrid)
		const toggle = cell(container, 0, TAG_DISPLAY)
		toggle.focus()
		await fireEvent.keyDown(toggle, { key: 'Enter' })

		expect(updateChannel).toHaveBeenCalledTimes(1)
		const arg = updateChannelMock.mock.calls[0][0]
		expect(arg.slot).toBe('001')
		expect(arg.data.tag_display).toBe(false) // was true in the fixture
	})

	it('the clarifier editor takes its min/max/step from the UISpec and commits value + Rx/Tx as one edit', async () => {
		const CLAR = 3
		const { container } = render(ChannelGrid)
		const clarCell = cell(container, 0, CLAR)
		clarCell.focus()
		await fireEvent.keyDown(clarCell, { key: 'Enter' })

		const input = screen.getByRole('spinbutton', { name: 'Clarifier hertz, M-01' })
		expect(input).toHaveAttribute('min', '-9990')
		expect(input).toHaveAttribute('max', '9990')
		expect(input).toHaveAttribute('step', '10')

		await fireEvent.input(input, { target: { value: '-120' } })
		const [rx] = screen.getAllByRole('checkbox')
		await fireEvent.click(rx)
		await fireEvent.keyDown(input, { key: 'Enter' })

		expect(updateChannel).toHaveBeenCalledTimes(1)
		const arg = updateChannelMock.mock.calls[0][0]
		expect(arg.data.clar_hz).toBe(-120)
		expect(arg.data.rx_clar).toBe(true)
		expect(arg.data.tx_clar).toBe(false)
	})

	it('the tag editor shows a live byte counter', async () => {
		const { container } = render(ChannelGrid)
		const tagCell = cell(container, 0, TAG)
		tagCell.focus()
		await fireEvent.keyDown(tagCell, { key: 'Enter' })

		expect(screen.getByText('6/12')).toBeInTheDocument() // MYCALL

		const input = screen.getByRole('textbox', { name: 'Tag, M-01' })
		await fireEvent.input(input, { target: { value: 'MYCALL LONG 40' } })
		const counter = screen.getByText('14/12')
		expect(counter).toBeInTheDocument()
		expect(counter.classList.contains('over')).toBe(true)
	})
})

describe('read-only banks', () => {
	async function showLockedBank() {
		const rendered = render(ChannelGrid)
		await fireEvent.click(screen.getByRole('tab', { name: /60 m channels/ }))
		return rendered
	}

	it('locks every row: lock glyph with the read-only tooltip, no editors on Enter or double-click', async () => {
		const { container } = await showLockedBank()
		const locks = screen.getAllByRole('img', { name: 'read-only' })
		expect(locks.length).toBe(2) // one per row

		const freqCell = cell(container, 0, FREQ)
		expect(freqCell.textContent).toBe('5.330500')
		freqCell.focus()
		await fireEvent.keyDown(freqCell, { key: 'Enter' })
		expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
		await fireEvent.dblClick(freqCell)
		expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
		expect(screen.queryByRole('combobox')).not.toBeInTheDocument()
		expect(updateChannel).not.toHaveBeenCalled()
	})

	it('marks the grid aria-readonly', async () => {
		await showLockedBank()
		expect(screen.getByRole('grid')).toHaveAttribute('aria-readonly', 'true')
	})
})

describe('issue decoration', () => {
	it('outlines an error cell red (connected) with the message as tooltip, and marks the row', () => {
		appState.setConnection({ Model: 'FT-710', CATID: '0800', Port: '/dev/tty.usb', USBSerial: '', Region: '', Demo: false })
		// Fix 5 (adjudicated MED, Codex M6 #5): issuesAdvisory is now STORED
		// (from a real Validate pass), not derived from `connected` — a
		// connected caller (bindings.js's revalidateQuiet) sets it
		// explicitly; this test drives appState directly, so it must too.
		appState.setIssuesAdvisory(false)
		appState.setIssues([{ Slot: '001', Field: 'frequency', Severity: 'error', Msg: 'slot "001": bad frequency' }])
		const { container } = render(ChannelGrid)

		const freqCell = cell(container, 0, FREQ)
		expect(freqCell.classList.contains('cell-issue-error')).toBe(true)
		expect(freqCell.getAttribute('title')).toContain('bad frequency')
		expect(freqCell.closest('tr')?.classList.contains('row-issue-error')).toBe(true)
	})

	it('outlines a warning amber', () => {
		appState.setConnection({ Model: 'FT-710', CATID: '0800', Port: '/dev/tty.usb', USBSerial: '', Region: '', Demo: false })
		appState.setIssuesAdvisory(false)
		appState.setIssues([{ Slot: '001', Field: 'tag', Severity: 'warning', Msg: 'slot "001": odd tag' }])
		const { container } = render(ChannelGrid)
		expect(cell(container, 0, TAG).classList.contains('cell-issue-warn')).toBe(true)
	})

	it('renders even errors amber with the advisory note while disconnected', () => {
		// beforeEach leaves the state disconnected — advisory mode.
		appState.setIssues([{ Slot: '001', Field: 'frequency', Severity: 'error', Msg: 'slot "001": bad frequency' }])
		const { container } = render(ChannelGrid)

		const freqCell = cell(container, 0, FREQ)
		expect(freqCell.classList.contains('cell-issue-warn')).toBe(true)
		expect(freqCell.classList.contains('cell-issue-error')).toBe(false)
		expect(freqCell.getAttribute('title')).toContain('advisory')
	})
})

describe('channel delete (task 22)', () => {
	it('shows the delete affordance only for populated, unlocked rows', async () => {
		render(ChannelGrid)
		expect(screen.getByRole('button', { name: 'Delete M-01' })).toBeInTheDocument()
		expect(screen.queryByRole('button', { name: 'Delete M-02' })).not.toBeInTheDocument() // M-02 is empty

		await fireEvent.click(screen.getByRole('tab', { name: /60 m channels/ }))
		expect(screen.queryByRole('button', { name: /Delete/ })).not.toBeInTheDocument() // read-only bank
	})

	it('clicking the ✕ opens a confirm modal with the honest erase caveat; confirming sends an empty channel', async () => {
		render(ChannelGrid)
		await fireEvent.click(screen.getByRole('button', { name: 'Delete M-01' }))

		expect(screen.getByText('Clear M-01 in this file?')).toBeInTheDocument()
		expect(screen.getByText(/radio keeps the channel until you delete it from the front panel/)).toBeInTheDocument()
		expect(screen.getByText(/erased \(unsupported\)/)).toBeInTheDocument()
		// task-25 brief: the front-panel erase procedure is surfaced right
		// here too, since this is the moment the user forms the
		// expectation (FT-710 operation manual, p.62). Task 42: this is now
		// UI_SPEC.EraseDialogNote rendered verbatim (plain prose, no
		// <strong> markup — see radiotext.go's own doc comment on why the
		// component-carried markup was dropped in the migration).
		expect(screen.getByText(/no CAT erase command/)).toBeInTheDocument()
		expect(screen.getByText(/\[V\/M\]/)).toBeInTheDocument()
		expect(screen.getByText(/\[ERASE\]/)).toBeInTheDocument()

		await fireEvent.click(screen.getByRole('button', { name: 'Clear M-01' }))

		expect(updateChannel).toHaveBeenCalledTimes(1)
		expect(updateChannel).toHaveBeenCalledWith({ slot: '001', data: null })
		expect(screen.queryByText('Clear M-01 in this file?')).not.toBeInTheDocument()
	})

	it('task 42: an empty served EraseDialogNote skips the erase-procedure paragraph entirely (no hardcoded fallback)', async () => {
		appState.setUISpec({ ...UI_SPEC, EraseDialogNote: '' })
		render(ChannelGrid)
		await fireEvent.click(screen.getByRole('button', { name: 'Delete M-01' }))
		expect(screen.getByText('Clear M-01 in this file?')).toBeInTheDocument()
		expect(screen.getByText(/radio keeps the channel until you delete it from the front panel/)).toBeInTheDocument()
		expect(screen.queryByText(/no CAT erase command/)).not.toBeInTheDocument()
	})

	it('Cancel closes the modal without committing', async () => {
		render(ChannelGrid)
		await fireEvent.click(screen.getByRole('button', { name: 'Delete M-01' }))
		await fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))

		expect(updateChannel).not.toHaveBeenCalled()
		expect(screen.queryByText('Clear M-01 in this file?')).not.toBeInTheDocument()
	})

	it('Delete/Backspace on a focused populated row (outside an open editor) opens the same confirm', async () => {
		const { container } = render(ChannelGrid)
		const freqCell = cell(container, 0, FREQ)
		freqCell.focus()
		await fireEvent.keyDown(freqCell, { key: 'Delete' })
		expect(screen.getByText('Clear M-01 in this file?')).toBeInTheDocument()
		await fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))

		const tagCell = cell(container, 0, TAG)
		tagCell.focus()
		await fireEvent.keyDown(tagCell, { key: 'Backspace' })
		expect(screen.getByText('Clear M-01 in this file?')).toBeInTheDocument()
	})

	it('Delete does nothing on an empty row or inside a locked bank', async () => {
		const { container } = render(ChannelGrid)
		const emptyCell = cell(container, 1, FREQ) // M-02, empty
		emptyCell.focus()
		await fireEvent.keyDown(emptyCell, { key: 'Delete' })
		expect(screen.queryByRole('dialog')).not.toBeInTheDocument()

		await fireEvent.click(screen.getByRole('tab', { name: /60 m channels/ }))
		const lockedCell = cell(container, 0, FREQ)
		lockedCell.focus()
		await fireEvent.keyDown(lockedCell, { key: 'Delete' })
		expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
		expect(updateChannel).not.toHaveBeenCalled()
	})

	it('Delete/Backspace while an editor is open is handled by the editor, not the delete flow', async () => {
		const { container } = render(ChannelGrid)
		const tagCell = cell(container, 0, TAG)
		tagCell.focus()
		await fireEvent.keyDown(tagCell, { key: 'Enter' }) // opens the tag editor
		const input = screen.getByRole('textbox', { name: 'Tag, M-01' })
		await fireEvent.keyDown(input, { key: 'Backspace' }) // ordinary text editing inside the input
		expect(screen.queryByText('Clear M-01 in this file?')).not.toBeInTheDocument()
	})

	it('deleting M-01 (RequiredSlots) sends the empty channel and the grid shows the resulting blocking issue', async () => {
		appState.setConnection({ Model: 'FT-710', CATID: '0800', Port: '/dev/tty.usb', USBSerial: '', Region: '', Demo: false })
		appState.setIssuesAdvisory(false)
		updateChannelMock.mockImplementation(async (ch) => {
			appState.applyChannelEdits([ch])
			const issues = [{ Slot: '001', Field: '', Severity: 'error', Msg: 'required slot "001" must not be empty' }]
			appState.setIssues(issues)
			return { Issues: issues, Dirty: true }
		})
		const { container } = render(ChannelGrid)
		await fireEvent.click(screen.getByRole('button', { name: 'Delete M-01' }))
		await fireEvent.click(screen.getByRole('button', { name: 'Clear M-01' }))
		await Promise.resolve()
		await Promise.resolve()

		expect(updateChannel).toHaveBeenCalledWith({ slot: '001', data: null })
		expect(cell(container, 0, FREQ).textContent).toBe('empty')
		expect(cell(container, 0, 0).closest('tr')?.classList.contains('row-issue-error')).toBe(true)
		// The now-empty M-01 row loses its own delete affordance.
		expect(screen.queryByRole('button', { name: 'Delete M-01' })).not.toBeInTheDocument()
	})
})

describe('drag copy/swap/move (task 22)', () => {
	/** happy-dom has no working DragEvent/DataTransfer — a plain Event
	 * shim carries everything the handlers actually read (nothing:
	 * dataTransfer access is optional-chained throughout). Mirrors the
	 * existing paste describe block's ClipboardEvent shim above. */
	async function drag(target, type) {
		const event = new Event(type, { bubbles: true, cancelable: true })
		await fireEvent(target, event)
	}

	it('shows the row-action affordances only for populated, unlocked rows (same gate as delete)', async () => {
		render(ChannelGrid)
		// M-01: populated — ↑ (boundary, first slot of the bank) and ↓ (to
		// the empty M-02) both render, ↑ disabled.
		expect(screen.getByRole('button', { name: 'Move M-01 up' })).toBeDisabled()
		expect(screen.getByRole('button', { name: 'Move M-01 down to M-02' })).not.toBeDisabled()
		// M-02: empty — no row actions at all.
		expect(screen.queryByRole('button', { name: /Move M-02/ })).not.toBeInTheDocument()
		await fireEvent.click(screen.getByRole('tab', { name: /60 m channels/ }))
		expect(screen.queryByRole('button', { name: /^Move /i })).not.toBeInTheDocument()
	})

	it('drag M-01 onto M-03 shows a drag-over highlight on the target row while hovering, cleared on leave', async () => {
		const { container } = render(ChannelGrid)
		const sourceRow = cell(container, 0, FREQ).closest('tr')
		const targetRow = cell(container, 2, FREQ).closest('tr')
		await drag(sourceRow, 'dragstart')
		await drag(targetRow, 'dragover')
		expect(targetRow?.classList.contains('row-drag-over')).toBe(true)
		await drag(targetRow, 'dragleave')
		expect(targetRow?.classList.contains('row-drag-over')).toBe(false)
	})

	it('dropping on the source row itself is a no-op: no popover, no call', async () => {
		const { container } = render(ChannelGrid)
		const row = cell(container, 0, FREQ).closest('tr')
		await drag(row, 'dragstart')
		await drag(row, 'dragover')
		await drag(row, 'drop')
		expect(screen.queryByText(/Copy or move M-01/)).not.toBeInTheDocument()
		expect(updateChannels).not.toHaveBeenCalled()
	})

	it('drop opens the popover; Copy here sends BOTH slots (source unchanged, target becomes a copy)', async () => {
		const { container } = render(ChannelGrid)
		const sourceRow = cell(container, 0, FREQ).closest('tr') // M-01, populated
		const targetRow = cell(container, 1, FREQ).closest('tr') // M-02, empty
		await drag(sourceRow, 'dragstart')
		await drag(targetRow, 'dragover')
		await drag(targetRow, 'drop')

		expect(screen.getByText('Copy or move M-01')).toBeInTheDocument()
		expect(screen.queryByRole('button', { name: 'Swap' })).not.toBeInTheDocument() // target empty: swap not offered
		await fireEvent.click(screen.getByRole('button', { name: /^Copy here/ }))

		expect(updateChannels).toHaveBeenCalledTimes(1)
		const batch = updateChannelsMock.mock.calls[0][0]
		expect(batch[0].slot).toBe('001')
		expect(batch[1].slot).toBe('002')
		expect(batch[0].data.freq_hz).toBe(7074000) // source unchanged
		expect(batch[1].data.freq_hz).toBe(7074000) // target: a copy of source
		expect(batch[1].data.tag).toBe('MYCALL')
	})

	it('Swap exchanges both channels intact — neither is lost', async () => {
		const { container } = render(ChannelGrid)
		const sourceRow = cell(container, 0, FREQ).closest('tr') // M-01: 7.074000 USB
		const targetRow = cell(container, 2, FREQ).closest('tr') // M-03: 14.074000 FM
		await drag(sourceRow, 'dragstart')
		await drag(targetRow, 'dragover')
		await drag(targetRow, 'drop')
		await fireEvent.click(screen.getByRole('button', { name: 'Swap' }))

		expect(updateChannels).toHaveBeenCalledTimes(1)
		const batch = updateChannelsMock.mock.calls[0][0]
		expect(batch[0].slot).toBe('001')
		expect(batch[0].data.freq_hz).toBe(14074000)
		expect(batch[0].data.mode).toBe('FM')
		expect(batch[1].slot).toBe('003')
		expect(batch[1].data.freq_hz).toBe(7074000)
		expect(batch[1].data.mode).toBe('USB')
	})

	it('Move here empties the source (Data: null) and moves its contents to the target', async () => {
		const { container } = render(ChannelGrid)
		const sourceRow = cell(container, 0, FREQ).closest('tr')
		const targetRow = cell(container, 2, FREQ).closest('tr')
		await drag(sourceRow, 'dragstart')
		await drag(targetRow, 'dragover')
		await drag(targetRow, 'drop')
		await fireEvent.click(screen.getByRole('button', { name: 'Move here' }))

		expect(updateChannels).toHaveBeenCalledTimes(1)
		const batch = updateChannelsMock.mock.calls[0][0]
		expect(batch[0]).toEqual({ slot: '001', data: null })
		expect(batch[1].slot).toBe('003')
		expect(batch[1].data.freq_hz).toBe(7074000)
		expect(batch[1].data.mode).toBe('USB')
	})

	it('Cancel closes the popover without committing', async () => {
		const { container } = render(ChannelGrid)
		const sourceRow = cell(container, 0, FREQ).closest('tr')
		const targetRow = cell(container, 2, FREQ).closest('tr')
		await drag(sourceRow, 'dragstart')
		await drag(targetRow, 'dragover')
		await drag(targetRow, 'drop')
		await fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))

		expect(updateChannels).not.toHaveBeenCalled()
		expect(screen.queryByText(/Copy or move M-01/)).not.toBeInTheDocument()
	})

	describe('keyboard path (Ctrl/Cmd+M — the ONLY path to the Copy/Swap/Move dialogue since task 24 replaced the → mouse button with ↑/↓; still covers long-distance moves)', () => {
		it('opens the popover with a slot-entry field; an invalid entry disables the actions; a valid populated target offers Copy/Swap/Move', async () => {
			const { container } = render(ChannelGrid)
			const freqCell = cell(container, 0, FREQ) // M-01
			freqCell.focus()
			await fireEvent.keyDown(freqCell, { key: 'm', ctrlKey: true })

			expect(screen.getByText('Copy or move M-01')).toBeInTheDocument()
			const input = screen.getByLabelText('Target slot')

			await fireEvent.input(input, { target: { value: 'M-99' } })
			expect(screen.getByText(/is not a slot/)).toBeInTheDocument()
			expect(screen.queryByRole('button', { name: /^Copy here/ })).not.toBeInTheDocument()

			await fireEvent.input(input, { target: { value: 'M-03' } }) // slot 003, populated
			expect(screen.getByRole('button', { name: /^Copy here/ })).toBeInTheDocument()
			expect(screen.getByRole('button', { name: 'Swap' })).toBeInTheDocument()
			expect(screen.getByRole('button', { name: 'Move here' })).toBeInTheDocument()

			await fireEvent.click(screen.getByRole('button', { name: 'Swap' }))
			expect(updateChannels).toHaveBeenCalledTimes(1)
			const batch = updateChannelsMock.mock.calls[0][0]
			expect(batch.map((c) => c.slot)).toEqual(['001', '003'])
		})

		it('refuses the source slot itself as a target', async () => {
			const { container } = render(ChannelGrid)
			const freqCell = cell(container, 0, FREQ)
			freqCell.focus()
			await fireEvent.keyDown(freqCell, { key: 'm', ctrlKey: true })
			const input = screen.getByLabelText('Target slot')
			await fireEvent.input(input, { target: { value: 'M-01' } })
			expect(screen.getByText(/must be different/)).toBeInTheDocument()
		})

		it('is refused (no popover) on an empty row or inside a locked bank', async () => {
			const { container } = render(ChannelGrid)
			const emptyCell = cell(container, 1, FREQ)
			emptyCell.focus()
			await fireEvent.keyDown(emptyCell, { key: 'm', ctrlKey: true })
			expect(screen.queryByRole('dialog')).not.toBeInTheDocument()

			await fireEvent.click(screen.getByRole('tab', { name: /60 m channels/ }))
			const lockedCell = cell(container, 0, FREQ)
			lockedCell.focus()
			await fireEvent.keyDown(lockedCell, { key: 'm', ctrlKey: true })
			expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
		})
	})
})

describe('row reorder ↑/↓ (task 24 — replaces the old per-row → button)', () => {
	it('aria-labels name the row and, when not at a boundary, the neighbour it will move to', () => {
		render(ChannelGrid)
		// MEM bank: M-01 is the bank's own first slot (no "up" neighbour);
		// its "down" neighbour is the empty M-02.
		expect(screen.getByRole('button', { name: 'Move M-01 up' })).toBeInTheDocument()
		expect(screen.getByRole('button', { name: 'Move M-01 down to M-02' })).toBeInTheDocument()
		// M-03 is MEM's own last slot (no "down" neighbour); its "up"
		// neighbour is the empty M-02.
		expect(screen.getByRole('button', { name: 'Move M-03 up to M-02' })).toBeInTheDocument()
		expect(screen.getByRole('button', { name: 'Move M-03 down' })).toBeInTheDocument()
	})

	it('disables ↑ on the bank’s own first slot and ↓ on its last, leaving both enabled in between', () => {
		render(ChannelGrid)
		expect(screen.getByRole('button', { name: 'Move M-01 up' })).toBeDisabled()
		expect(screen.getByRole('button', { name: 'Move M-01 down to M-02' })).not.toBeDisabled()
		expect(screen.getByRole('button', { name: 'Move M-03 up to M-02' })).not.toBeDisabled()
		expect(screen.getByRole('button', { name: 'Move M-03 down' })).toBeDisabled()
	})

	it('a populated neighbour is a SWAP: commits instantly with no confirmation, exchanging both channels intact', async () => {
		render(ChannelGrid)
		await fireEvent.click(screen.getByRole('tab', { name: /Scan limits/ })) // PMS: P1L and P1U both populated
		await fireEvent.click(screen.getByRole('button', { name: 'Move P1L down to P1U' }))

		expect(screen.queryByRole('dialog')).not.toBeInTheDocument() // instant — no confirmation
		expect(updateChannels).toHaveBeenCalledTimes(1)
		const batch = updateChannelsMock.mock.calls[0][0]
		expect(batch).toEqual([
			{ slot: 'P1L', data: expect.objectContaining({ freq_hz: 7200000 }) }, // P1U's data, exchanged in
			{ slot: 'P1U', data: expect.objectContaining({ freq_hz: 7000000 }) }, // P1L's data, exchanged in
		])
	})

	it('an empty neighbour is a MOVE: shows a confirm dialogue with the keeps-old-contents caveat before committing', async () => {
		render(ChannelGrid)
		await fireEvent.click(screen.getByRole('button', { name: 'Move M-01 down to M-02' }))

		expect(screen.getByText('Move M-01 to M-02?')).toBeInTheDocument()
		expect(screen.getByText(/radio keeps its old contents until front-panel deletion/)).toBeInTheDocument()
		expect(updateChannels).not.toHaveBeenCalled()

		await fireEvent.click(screen.getByRole('button', { name: 'Move here' }))
		expect(updateChannels).toHaveBeenCalledTimes(1)
		const batch = updateChannelsMock.mock.calls[0][0]
		expect(batch[0]).toEqual({ slot: '001', data: null })
		expect(batch[1].slot).toBe('002')
		expect(batch[1].data.freq_hz).toBe(7074000)
	})

	it('Cancel on the empty-neighbour confirm commits nothing and closes the dialogue', async () => {
		render(ChannelGrid)
		await fireEvent.click(screen.getByRole('button', { name: 'Move M-01 down to M-02' }))
		await fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))

		expect(updateChannels).not.toHaveBeenCalled()
		expect(screen.queryByText('Move M-01 to M-02?')).not.toBeInTheDocument()
	})

	it('Alt+ArrowDown on a focused row performs the same SWAP as the ↓ button', async () => {
		const { container } = render(ChannelGrid)
		await fireEvent.click(screen.getByRole('tab', { name: /Scan limits/ }))
		const p1l = cell(container, 0, FREQ)
		p1l.focus()
		await fireEvent.keyDown(p1l, { key: 'ArrowDown', altKey: true })

		expect(updateChannels).toHaveBeenCalledTimes(1)
		const batch = updateChannelsMock.mock.calls[0][0]
		expect(batch[0].slot).toBe('P1L')
		expect(batch[1].slot).toBe('P1U')
	})

	it('plain ArrowDown (no Alt) is unaffected — ordinary cell navigation, no commit', async () => {
		const { container } = render(ChannelGrid)
		const start = cell(container, 0, FREQ)
		start.focus()
		await fireEvent.keyDown(start, { key: 'ArrowDown' })

		expect(updateChannels).not.toHaveBeenCalled()
		expect(cell(container, 1, FREQ)).toHaveAttribute('tabindex', '0')
	})

	it('two consecutive Alt+ArrowUp activations move the channel two slots, focus following it each time without re-acquiring the row', async () => {
		// M-02 populated too for this test only, so both hops are instant
		// swaps — the point here is the neighbour walk + focus tracking,
		// not the move-confirm path (covered above).
		const cp = codeplugFixture()
		cp.Channels[1] = { slot: '002', data: data(3563000, 'LSB', { tag: 'THIRD' }) }
		appState.setCodeplug(cp)
		updateChannelsMock.mockImplementation(async (channels) => {
			appState.applyChannelEdits(channels)
			return { Issues: [], Dirty: true }
		})

		const { container } = render(ChannelGrid)
		const start = cell(container, 2, FREQ) // M-03: 14.074000 FM
		start.focus()
		await fireEvent.keyDown(start, { key: 'ArrowUp', altKey: true })

		expect(updateChannels).toHaveBeenCalledTimes(1)
		// The channel that started at M-03 now lives at M-02…
		expect(cell(container, 1, FREQ).textContent).toBe('14.074000')
		// …and focus (the roving tabindex) followed it there — the SAME
		// now-focused element can drive the next Alt+ArrowUp without the
		// test (or a real user) re-acquiring the row.
		expect(cell(container, 1, FREQ)).toHaveAttribute('tabindex', '0')
		expect(document.activeElement).toBe(cell(container, 1, FREQ))

		await fireEvent.keyDown(/** @type {HTMLElement} */ (document.activeElement), { key: 'ArrowUp', altKey: true })

		expect(updateChannels).toHaveBeenCalledTimes(2)
		expect(cell(container, 0, FREQ).textContent).toBe('14.074000')
		expect(cell(container, 0, FREQ)).toHaveAttribute('tabindex', '0')
		expect(document.activeElement).toBe(cell(container, 0, FREQ))
	})

	it('is refused (no button, no commit) on an empty row or inside a locked bank', async () => {
		const { container } = render(ChannelGrid)
		expect(screen.queryByRole('button', { name: /Move M-02/ })).not.toBeInTheDocument()

		const emptyCell = cell(container, 1, FREQ)
		emptyCell.focus()
		await fireEvent.keyDown(emptyCell, { key: 'ArrowUp', altKey: true })
		expect(updateChannels).not.toHaveBeenCalled()

		await fireEvent.click(screen.getByRole('tab', { name: /60 m channels/ }))
		expect(screen.queryByRole('button', { name: /Move 5-01/ })).not.toBeInTheDocument()
		const lockedCell = cell(container, 0, FREQ)
		lockedCell.focus()
		await fireEvent.keyDown(lockedCell, { key: 'ArrowDown', altKey: true })
		expect(updateChannels).not.toHaveBeenCalled()
	})
})

describe('legend (task 22 §3: Tone/Scan-skip discoverability)', () => {
	it('renders a one-line note below the grid, distinguishing Tone (verified) from Scan Skip (unverified)', () => {
		render(ChannelGrid)
		expect(screen.getByText(/aren't carried by the FT-710's CAT protocol/)).toBeInTheDocument()
		expect(screen.getByText(/hardware-verified for Tone/)).toBeInTheDocument()
		expect(screen.getByText(/Scan Skip preservation is/)).toBeInTheDocument()
	})

	// m42a: pins the FULL rendered legend, whitespace-normalised the way a
	// browser collapses it visually, against the exact two-sentence string
	// the pre-move component rendered — the regression this task must not
	// break: moving the second sentence from a Svelte literal to a served
	// field must produce byte-identical rendered prose.
	it('m42a: renders both legend sentences with a single space between them, matching the pre-move rendering exactly', () => {
		const { container } = render(ChannelGrid)
		const legend = container.querySelector('.grid-legend')
		const normalised = legend.textContent.replace(/\s+/g, ' ').trim()
		expect(normalised).toBe(
			"Tone and Scan Skip aren't carried by the FT-710's CAT protocol — set them on the radio. Preservation across a rewrite is hardware-verified for Tone; Scan Skip preservation is not yet verified (see each cell's tooltip)."
		)
	})

	it('task 42: an empty served ToneScanSkipNote leaves ToneScanSkipVerification rendering alone, with no hardcoded fallback for either sentence', () => {
		appState.setUISpec({ ...UI_SPEC, ToneScanSkipNote: '' })
		render(ChannelGrid)
		expect(screen.queryByText(/aren't carried by the FT-710's CAT protocol/)).not.toBeInTheDocument()
		expect(screen.getByText(/hardware-verified for Tone/)).toBeInTheDocument()
	})

	it('m42a: an empty served ToneScanSkipVerification leaves ToneScanSkipNote rendering alone, with no hardcoded fallback for either sentence', () => {
		appState.setUISpec({ ...UI_SPEC, ToneScanSkipVerification: '' })
		render(ChannelGrid)
		expect(screen.getByText(/aren't carried by the FT-710's CAT protocol/)).toBeInTheDocument()
		expect(screen.queryByText(/hardware-verified for Tone/)).not.toBeInTheDocument()
	})
})

describe('paste', () => {
	/** @param {HTMLElement} target @param {string} text */
	async function paste(target, text) {
		const event = new Event('paste', { bubbles: true, cancelable: true })
		// happy-dom has no ClipboardEvent constructor with clipboardData;
		// an expando property carries the same shape the handler reads.
		// @ts-expect-error deliberate test shim
		event.clipboardData = { getData: () => text }
		await fireEvent(target, event)
	}

	it('maps a TSV block from the focused cell into ONE updateChannels call', async () => {
		const { container } = render(ChannelGrid)
		const start = cell(container, 0, FREQ)
		start.focus()
		await paste(start, '7.1\tFM\n14.2\tLSB\n')

		expect(updateChannels).toHaveBeenCalledTimes(1)
		const batch = updateChannelsMock.mock.calls[0][0]
		expect(batch).toHaveLength(2)
		expect(batch[0].slot).toBe('001')
		expect(batch[0].data.freq_hz).toBe(7100000)
		expect(batch[0].data.mode).toBe('FM')
		expect(batch[0].data.tag).toBe('MYCALL') // untouched field preserved
		expect(batch[1].slot).toBe('002') // empty slot populated by the pasted frequency
		expect(batch[1].data.freq_hz).toBe(14200000)
		expect(batch[1].data.mode).toBe('LSB')
	})

	it('rejects a paste into a read-only bank with an inline alert and no call', async () => {
		const { container } = render(ChannelGrid)
		await fireEvent.click(screen.getByRole('tab', { name: /60 m channels/ }))
		const start = cell(container, 0, FREQ)
		start.focus()
		await paste(start, '5.4')

		expect(updateChannels).not.toHaveBeenCalled()
		expect(appState.alerts).toHaveLength(1)
		expect(appState.alerts[0].message).toContain('read-only')
	})
})

describe('empty states', () => {
	it('shows the no-codeplug message before anything is loaded', () => {
		appState.setCodeplug(null)
		render(ChannelGrid)
		expect(screen.getByText('No codeplug loaded')).toBeInTheDocument()
		expect(screen.queryByRole('grid')).not.toBeInTheDocument()
	})

	it('explains a missing UISpec instead of rendering a broken grid', () => {
		appState.setUISpec(null)
		render(ChannelGrid)
		expect(screen.getByText('Grid layout unavailable')).toBeInTheDocument()
	})
})
