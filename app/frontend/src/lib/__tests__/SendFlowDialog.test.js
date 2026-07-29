// SPDX-License-Identifier: GPL-3.0-or-later

// Send dialogue / transfer-view tests (task-18 brief §3): a fixture
// SendPlanView (incl. blocked reasons + a firmware-required variant),
// confirm passing the exact token from THIS fixture, and every outcome
// rendering (ok/aborted/refused/cancelled) — asserting "snapshot" wording
// is present and "backup" is never present anywhere in the dialogue,
// per the project's test-pinned convention.

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/svelte'
import { appState } from '../state/app.svelte.js'
import SendFlowDialog from '../SendFlowDialog.svelte'

vi.mock('../bridge/bindings.js', () => ({
	confirmSend: vi.fn(),
	cancelTransfer: vi.fn(),
	readRadio: vi.fn().mockResolvedValue({}),
}))

import { confirmSend, cancelTransfer, readRadio } from '../bridge/bindings.js'

const confirmSendMock = vi.mocked(confirmSend)
const cancelTransferMock = vi.mocked(cancelTransfer)
const readRadioMock = vi.mocked(readRadio)

/** Task 42 (M9a-6): the two radiotext-sourced prose fields this dialogue
 * consumes — byte-identical to internal/radiotext.go's ft710Text, so the
 * tests below keep asserting the real, end-to-end FT-710 wording even
 * though the dialogue itself no longer hardcodes any of it. */
const UI_SPEC = {
	EraseDialogNote:
		'The FT-710 has no CAT erase command. To delete a channel on the radio: press and hold [V/M] to open the memory channel list, select the channel, then touch [ERASE].',
	FirmwarePlaceholder: 'e.g. V01-10',
}

/** @param {object} extra */
function channelData(freqHz, mode, extra = {}) {
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
		tag_display: { state: 'known', value: true },
		scan_skip: { state: 'unknown' },
		...extra,
	}
}

/** A fixture SendPlanView with all four review groups populated: one
 * unblocked Added, one unblocked + one BLOCKED Modified, one unblocked
 * Erased. @param {object} overrides */
function planFixture(overrides = {}) {
	return {
		Diff: {
			Added: [
				{ Slot: '004', SlotDisplay: 'M-04', Bank: 'MEM', Kind: 'added', After: channelData(7074000, 'USB'), Blocked: false, BlockReason: '' },
			],
			Modified: [
				{
					Slot: '001', SlotDisplay: 'M-01', Bank: 'MEM', Kind: 'modified',
					Before: channelData(7000000, 'LSB'), After: channelData(7100000, 'USB'),
					Blocked: false, BlockReason: '',
				},
				{
					Slot: '501', SlotDisplay: '5-01', Bank: '60M', Kind: 'modified',
					Before: channelData(5330500, 'USB'), After: channelData(5346500, 'USB'),
					Blocked: true, BlockReason: 'bank 60M is read-only',
				},
			],
			Erased: [
				{ Slot: '010', SlotDisplay: 'M-10', Bank: 'MEM', Kind: 'erased', Before: channelData(14074000, 'USB'), Blocked: false, BlockReason: '' },
			],
			Counts: { Added: 1, Modified: 2, Erased: 1, Blocked: 1, Unchanged: 95 },
		},
		SnapshotPath: '/tmp/snapshots/2026-07-12T090000.json',
		BaselineDigestShort: 'abc123def456',
		ConfirmationDigest: 'confirm-digest-xyz-001',
		NothingToSend: false,
		FirmwareRequired: false,
		FirmwareGuidance: '',
		...overrides,
	}
}

beforeEach(() => {
	appState.clearConnection()
	appState.setUISpec(UI_SPEC)
	appState.alerts = []
	vi.clearAllMocks()
	confirmSendMock.mockResolvedValue(undefined)
	cancelTransferMock.mockResolvedValue(undefined)
	readRadioMock.mockResolvedValue({})
})

describe('review phase', () => {
	it('renders the counts line, snapshot path and baseline digest', () => {
		render(SendFlowDialog, { plan: planFixture(), onClose: vi.fn(), onPrepareAgain: vi.fn() })
		expect(screen.getByText(/A snapshot of the radio's current state has been saved first/)).toBeInTheDocument()
		expect(screen.getByText('/tmp/snapshots/2026-07-12T090000.json')).toBeInTheDocument()
		expect(screen.getByText(/abc123def456/)).toBeInTheDocument()
		expect(screen.getByText('1', { exact: true, selector: '.count-chip-added strong' })).toBeInTheDocument()
	})

	it('groups entries into Added / Modified / Erased / Blocked, excluding blocked entries from their own kind group', () => {
		render(SendFlowDialog, { plan: planFixture(), onClose: vi.fn(), onPrepareAgain: vi.fn() })
		expect(screen.getByText('Added (1)')).toBeInTheDocument()
		expect(screen.getByText('Modified (1)')).toBeInTheDocument() // the blocked one excluded
		expect(screen.getByText('Erased (1)')).toBeInTheDocument()
		expect(screen.getByText('Blocked (1)')).toBeInTheDocument()
		expect(screen.getByText(/bank 60M is read-only/)).toBeInTheDocument()
		// The blocked slot (5-01) appears only in the Blocked group.
		const blockedSection = screen.getByText('Blocked (1)').closest('section')
		expect(within(blockedSection).getByText(/5-01/)).toBeInTheDocument()
	})

	it("renders the Inert-clarifier BlockReason verbatim from Go (M5b: a CHANGED clarifier is blocked — the radio ignores the transmitted value — while the clarifier column itself stays editable offline)", () => {
		const plan = planFixture()
		plan.Diff.Modified.push({
			Slot: '002', SlotDisplay: 'M-02', Bank: 'MEM', Kind: 'modified',
			Before: channelData(7000000, 'LSB'),
			After: channelData(7000000, 'LSB', { clar_hz: 100 }),
			Blocked: true, BlockReason: 'clarifier changes are ignored by the radio and cannot be sent',
		})
		plan.Diff.Counts = { Added: 1, Modified: 3, Erased: 1, Blocked: 2, Unchanged: 94 }
		render(SendFlowDialog, { plan, onClose: vi.fn(), onPrepareAgain: vi.fn() })
		expect(screen.getByText('Blocked (2)')).toBeInTheDocument()
		// The Go-composed reason renders verbatim (Codex M6 #6: no
		// protocol prose composed in JS — this string comes from
		// codeplug.Diff's inertBlockReason).
		expect(screen.getByText(/clarifier changes are ignored by the radio and cannot be sent/)).toBeInTheDocument()
		const blockedSection = screen.getByText('Blocked (2)').closest('section')
		expect(within(blockedSection).getByText(/M-02/)).toBeInTheDocument()
	})

	it('has no firmware field when FirmwareRequired is false', () => {
		render(SendFlowDialog, { plan: planFixture(), onClose: vi.fn(), onPrepareAgain: vi.fn() })
		expect(screen.queryByLabelText('Confirmed firmware version')).not.toBeInTheDocument()
	})

	it('firmware-required variant: renders FirmwareGuidance from the plan VERBATIM (Codex M6 #6, LOW: no FT-710 protocol facts in JS — the dialogue must not compose its own V01-10 sentence); Confirm disabled until filled', async () => {
		const guidance = 'TEST-ONLY firmware guidance text: needs V01-10 or later, no CAT query exists for it.'
		render(SendFlowDialog, { plan: planFixture({ FirmwareRequired: true, FirmwareGuidance: guidance }), onClose: vi.fn(), onPrepareAgain: vi.fn() })
		const input = screen.getByLabelText('Confirmed firmware version')
		expect(screen.getByText(guidance)).toBeInTheDocument()
		// Task 42: the input's placeholder is UI_SPEC.FirmwarePlaceholder
		// (GetUISpec's FirmwarePlaceholder), not a hardcoded "e.g. V01-10".
		expect(input).toHaveAttribute('placeholder', 'e.g. V01-10')
		const confirmBtn = screen.getByRole('button', { name: 'Confirm send' })
		expect(confirmBtn).toBeDisabled()

		await fireEvent.input(input, { target: { value: 'V01-10' } })
		expect(confirmBtn).not.toBeDisabled()
	})

	it('task 42: an empty served FirmwarePlaceholder leaves the input with no placeholder (no hardcoded fallback)', () => {
		appState.setUISpec({ ...UI_SPEC, FirmwarePlaceholder: '' })
		render(SendFlowDialog, { plan: planFixture({ FirmwareRequired: true, FirmwareGuidance: 'x' }), onClose: vi.fn(), onPrepareAgain: vi.fn() })
		expect(screen.getByLabelText('Confirmed firmware version')).toHaveAttribute('placeholder', '')
	})

	it('Confirm passes the exact confirmation digest from THIS plan, plus the firmware field', async () => {
		const plan = planFixture({ FirmwareRequired: true, ConfirmationDigest: 'this-exact-token' })
		render(SendFlowDialog, { plan, onClose: vi.fn(), onPrepareAgain: vi.fn() })
		await fireEvent.input(screen.getByLabelText('Confirmed firmware version'), { target: { value: 'V01-10' } })
		await fireEvent.click(screen.getByRole('button', { name: 'Confirm send' }))
		expect(confirmSendMock).toHaveBeenCalledWith('this-exact-token', 'V01-10')
	})

	it('Cancel closes the dialogue and never calls confirmSend (the plan is simply dropped)', async () => {
		const onClose = vi.fn()
		render(SendFlowDialog, { plan: planFixture(), onClose, onPrepareAgain: vi.fn() })
		await fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
		expect(onClose).toHaveBeenCalledTimes(1)
		expect(confirmSendMock).not.toHaveBeenCalled()
	})

	it('a truly-empty plan (NothingToSend, no blocked entries) shows the plain parity message with Cancel/Confirm still present', () => {
		const plan = planFixture({
			NothingToSend: true,
			Diff: {
				Added: [], Modified: [], Erased: [],
				Counts: { Added: 0, Modified: 0, Erased: 0, Blocked: 0, Unchanged: 100 },
			},
		})
		render(SendFlowDialog, { plan, onClose: vi.fn(), onPrepareAgain: vi.fn() })
		expect(screen.getByText(/working copy already matches the radio/)).toBeInTheDocument()
		expect(screen.getByRole('button', { name: 'Confirm send' })).toBeDisabled()
		expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument()
	})

	it('shows the synchronous rejection inline (digest mismatch etc.) without leaving review phase', async () => {
		confirmSendMock.mockRejectedValue(new Error('app: confirmation digest mismatch'))
		render(SendFlowDialog, { plan: planFixture(), onClose: vi.fn(), onPrepareAgain: vi.fn() })
		await fireEvent.click(screen.getByRole('button', { name: 'Confirm send' }))
		expect(await screen.findByText(/confirmation digest mismatch/)).toBeInTheDocument()
		// Still in review — the Confirm button is still here (not swallowed
		// into a phase change).
		expect(screen.getByRole('button', { name: 'Confirm send' })).toBeInTheDocument()
	})

	it('Escape closes the dialogue while reviewing (closable)', async () => {
		const onClose = vi.fn()
		const { container } = render(SendFlowDialog, { plan: planFixture(), onClose, onPrepareAgain: vi.fn() })
		const dialog = container.querySelector('[role="dialog"]')
		await fireEvent.keyDown(dialog, { key: 'Escape' })
		expect(onClose).toHaveBeenCalledTimes(1)
	})
})

// task-25 brief (adjudicated remedy for the reported "i don't seem to be
// able to send deletes to the radio" defect): a blocked-only plan
// (NothingToSend true BECAUSE every pending change is Blocked, not
// because the working copy matches the radio) must render an honest
// informational state — the blocked reasons, the counts line, and a
// single Close affordance, NEVER a disabled-but-present Confirm button
// (which would wrongly imply sending might become possible).
describe('blocked-only informational state (task-25 brief)', () => {
	it('has no Confirm affordance, only Close — closing calls onClose and never confirmSend', async () => {
		const onClose = vi.fn()
		render(SendFlowDialog, { plan: planFixture({ NothingToSend: true }), onClose, onPrepareAgain: vi.fn() })

		expect(screen.getByText(/None of the pending changes can be sent/)).toBeInTheDocument()
		expect(screen.getByText(/bank 60M is read-only/)).toBeInTheDocument()
		expect(screen.queryByRole('button', { name: 'Confirm send' })).not.toBeInTheDocument()
		expect(screen.queryByRole('button', { name: 'Cancel' })).not.toBeInTheDocument()

		await fireEvent.click(screen.getByRole('button', { name: 'Close' }))
		expect(onClose).toHaveBeenCalledTimes(1)
		expect(confirmSendMock).not.toHaveBeenCalled()
	})

	it('shows the front-panel erase procedure when the blocked entry is an unsupported erase', () => {
		const plan = planFixture({
			NothingToSend: true,
			Diff: {
				Added: [], Modified: [],
				Erased: [
					{
						Slot: '010', SlotDisplay: 'M-10', Bank: 'MEM', Kind: 'erased',
						Before: channelData(14074000, 'USB'), Blocked: true, BlockReason: 'erase not supported on this radio',
					},
				],
				Counts: { Added: 0, Modified: 0, Erased: 1, Blocked: 1, Unchanged: 99 },
			},
		})
		render(SendFlowDialog, { plan, onClose: vi.fn(), onPrepareAgain: vi.fn() })
		expect(screen.getByText(/erase not supported on this radio/)).toBeInTheDocument()
		// Task 42: UI_SPEC.EraseDialogNote rendered verbatim (plain prose, no
		// <strong> markup — see radiotext.go's own doc comment on why the
		// component-carried markup was dropped in the migration).
		expect(screen.getByText(/no CAT erase command/)).toBeInTheDocument()
		expect(screen.getByText(/\[V\/M\]/)).toBeInTheDocument()
		expect(screen.getByText(/\[ERASE\]/)).toBeInTheDocument()
		expect(screen.queryByRole('button', { name: 'Confirm send' })).not.toBeInTheDocument()
	})

	it('does NOT show the erase procedure when the only blocked entry is not an erase', () => {
		render(SendFlowDialog, { plan: planFixture({ NothingToSend: true }), onClose: vi.fn(), onPrepareAgain: vi.fn() })
		expect(screen.queryByText(/no CAT erase command/)).not.toBeInTheDocument()
	})

	it('task 42: an empty served EraseDialogNote skips the erase-procedure paragraph even for a blocked erase (no hardcoded fallback)', () => {
		appState.setUISpec({ ...UI_SPEC, EraseDialogNote: '' })
		const plan = planFixture({
			NothingToSend: true,
			Diff: {
				Added: [], Modified: [],
				Erased: [
					{
						Slot: '010', SlotDisplay: 'M-10', Bank: 'MEM', Kind: 'erased',
						Before: channelData(14074000, 'USB'), Blocked: true, BlockReason: 'erase not supported on this radio',
					},
				],
				Counts: { Added: 0, Modified: 0, Erased: 1, Blocked: 1, Unchanged: 99 },
			},
		})
		render(SendFlowDialog, { plan, onClose: vi.fn(), onPrepareAgain: vi.fn() })
		expect(screen.getByText(/erase not supported on this radio/)).toBeInTheDocument()
		expect(screen.queryByText(/no CAT erase command/)).not.toBeInTheDocument()
	})

	it('a mixed plan (some sendable, some blocked) is unchanged: dialogue opens normally, Confirm present, blocked entries listed', () => {
		render(SendFlowDialog, { plan: planFixture(), onClose: vi.fn(), onPrepareAgain: vi.fn() })
		expect(screen.getByRole('button', { name: 'Confirm send' })).toBeInTheDocument()
		expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument()
		expect(screen.queryByRole('button', { name: 'Close' })).not.toBeInTheDocument()
		expect(screen.getByText(/bank 60M is read-only/)).toBeInTheDocument()
		expect(screen.queryByText(/None of the pending changes can be sent/)).not.toBeInTheDocument()
	})
})

describe('transferring phase', () => {
	async function enterTransferring(plan = planFixture()) {
		const onClose = vi.fn()
		const utils = render(SendFlowDialog, { plan, onClose, onPrepareAgain: vi.fn() })
		await fireEvent.click(screen.getByRole('button', { name: 'Confirm send' }))
		return { onClose, ...utils }
	}

	it('shows live phase/counter/slot progress from appState.transfer', async () => {
		await enterTransferring()
		appState.applyProgress({ Phase: 'verify-read', Done: 2, Total: 10, Slot: 'M-03' })
		expect(await screen.findByText(/Verifying \(pre-write\)/)).toBeInTheDocument()
		expect(screen.getByText(/2\/10/)).toBeInTheDocument()
		expect(screen.getByText('M-03')).toBeInTheDocument()
	})

	it('Cancel transfer calls cancelTransfer and shows the boundary caption', async () => {
		await enterTransferring()
		expect(screen.getByText('Stops at the next verified channel boundary.')).toBeInTheDocument()
		await fireEvent.click(screen.getByRole('button', { name: 'Cancel transfer' }))
		expect(cancelTransferMock).toHaveBeenCalledTimes(1)
	})

	it('Escape does NOT close while transferring (never mid-transfer)', async () => {
		const { onClose, container } = await enterTransferring()
		const dialog = container.querySelector('[role="dialog"]')
		await fireEvent.keyDown(dialog, { key: 'Escape' })
		expect(onClose).not.toHaveBeenCalled()
	})

	it('moves to the result phase once transfer:done (Kind "send") arrives', async () => {
		await enterTransferring()
		appState.applyTransferDone({ Kind: 'send', Outcome: 'ok', Report: { Written: 1, Verified: 1, SkippedBlocked: 0, Unchanged: 116, Slots: [], JournalPath: '/j', SnapshotPath: '/s' }, Message: '' })
		expect(await screen.findByText('Send complete')).toBeInTheDocument()
	})
})

describe('result phase — outcome renderings', () => {
	/** @param {object} outcome */
	async function toResult(outcome, plan = planFixture()) {
		const onClose = vi.fn()
		const onPrepareAgain = vi.fn()
		const utils = render(SendFlowDialog, { plan, onClose, onPrepareAgain })
		await fireEvent.click(screen.getByRole('button', { name: 'Confirm send' }))
		appState.applyTransferDone(outcome)
		await screen.findByRole('heading', { level: 2 })
		return { onClose, onPrepareAgain, ...utils }
	}

	it('ok: shows the verify report counts, paths, and the re-read prompt; "snapshot" present, "backup" absent', async () => {
		const { container } = await toResult({
			Kind: 'send', Outcome: 'ok',
			Report: { Written: 3, Verified: 3, SkippedBlocked: 1, Unchanged: 112, Slots: [], JournalPath: '/tmp/journal.log', SnapshotPath: '/tmp/snap.json' },
			Message: '',
		})
		expect(screen.getByText('Send complete')).toBeInTheDocument()
		expect(screen.getByText(/Re-read the radio to refresh the baseline/)).toBeInTheDocument()
		expect(screen.getByText(/\/tmp\/journal\.log/)).toBeInTheDocument()
		expect(screen.getByText(/\/tmp\/snap\.json/)).toBeInTheDocument()
		expect(container.textContent.toLowerCase()).not.toContain('backup')
	})

	it('ok: "Read radio now" calls readRadio and closes', async () => {
		const { onClose } = await toResult({
			Kind: 'send', Outcome: 'ok',
			Report: { Written: 1, Verified: 1, SkippedBlocked: 0, Unchanged: 116, Slots: [], JournalPath: '/j', SnapshotPath: '/s' },
			Message: '',
		})
		await fireEvent.click(screen.getByRole('button', { name: 'Read radio now' }))
		expect(readRadioMock).toHaveBeenCalledTimes(1)
		expect(onClose).toHaveBeenCalledTimes(1)
	})

	it('aborted: shows the reason, the per-slot table, paths, and recovery guidance saying "snapshot" (never "backup") and that erase/hidden fields cannot be restored via CAT', async () => {
		const { container } = await toResult({
			Kind: 'send', Outcome: 'aborted',
			Report: {
				Written: 1, Verified: 1, SkippedBlocked: 0, Unchanged: 0,
				Slots: [{ Slot: '001', SlotDisplay: 'M-01', Action: 'write', VerifyOK: true, Detail: 'ok' }],
				Aborted: true, AbortReason: 'verify mismatch', JournalPath: '/tmp/journal.log', SnapshotPath: '/tmp/snap.json',
			},
			Message: 'transfer aborted at slot M-02: verify mismatch — the radio was partially written; the snapshot records its contents before this send, and the journal records exactly what happened',
		})
		expect(screen.getByText('Send aborted')).toBeInTheDocument()
		expect(screen.getByText(/verify mismatch/)).toBeInTheDocument()
		expect(screen.getByText('M-01')).toBeInTheDocument() // per-slot table row
		expect(screen.getByText(/\/tmp\/journal\.log/)).toBeInTheDocument()
		expect(screen.getByText(/\/tmp\/snap\.json/)).toBeInTheDocument()
		expect(screen.getByText(/cannot be restored to the radio over CAT/)).toBeInTheDocument()
		expect(screen.getByText(/front panel/)).toBeInTheDocument()
		const text = container.textContent.toLowerCase()
		expect(text).toContain('snapshot')
		expect(text).not.toContain('backup')
	})

	it('refused: shows the plain-language cause and a "Prepare again" affordance', async () => {
		const { onClose, onPrepareAgain } = await toResult({
			Kind: 'send', Outcome: 'refused', Report: null,
			Message: 'refused: the radio\'s contents changed after the plan was prepared — read the radio and prepare send again',
		})
		expect(screen.getByText('Send refused')).toBeInTheDocument()
		expect(screen.getByText(/read the radio and prepare send again/)).toBeInTheDocument()
		await fireEvent.click(screen.getByRole('button', { name: 'Prepare again' }))
		expect(onClose).toHaveBeenCalledTimes(1)
		expect(onPrepareAgain).toHaveBeenCalledTimes(1)
	})

	it('cancelled with a Report (partial write): shows the boundary explanation, counts, paths, and the SAME unrestorable-field warning the aborted branch shows (Codex M6 elevated minor i)', async () => {
		const { container } = await toResult({
			Kind: 'send', Outcome: 'cancelled',
			Report: { Written: 1, Verified: 1, SkippedBlocked: 0, Unchanged: 0, Slots: [], JournalPath: '/tmp/journal.log', SnapshotPath: '/tmp/snap.json' },
			Message: 'transfer cancelled at slot M-01: the in-flight write+verify pair completed before the cancellation was honoured',
		})
		expect(screen.getByText('Send cancelled')).toBeInTheDocument()
		expect(screen.getByText(/cancellation was honoured/)).toBeInTheDocument()
		expect(screen.getByText(/\/tmp\/journal\.log/)).toBeInTheDocument()
		expect(screen.getByText(/\/tmp\/snap\.json/)).toBeInTheDocument()
		expect(screen.getByText(/cannot be restored to the radio over CAT/)).toBeInTheDocument()
		expect(screen.getByText(/front panel/)).toBeInTheDocument()
		const text = container.textContent.toLowerCase()
		expect(text).toContain('snapshot')
		expect(text).not.toContain('backup')
	})

	it('cancelled with no Report (nothing written yet): shows just the explanation, no report table', async () => {
		const { container } = await toResult({
			Kind: 'send', Outcome: 'cancelled', Report: null,
			Message: 'transfer cancelled before any write began',
		})
		expect(screen.getByText('Send cancelled')).toBeInTheDocument()
		expect(screen.getByText('transfer cancelled before any write began')).toBeInTheDocument()
		expect(container.querySelector('.report-table')).not.toBeInTheDocument()
	})
})

describe('regression: a second send must not render the previous outcome (Critical review finding)', () => {
	/** A previous send's outcome, distinguishable from the new one below by
	 * its paths — this must NEVER be visible once the second dialogue's
	 * Confirm has been clicked. */
	const PREVIOUS_OUTCOME = {
		Kind: 'send', Outcome: 'ok',
		Report: {
			Written: 9, Verified: 9, SkippedBlocked: 0, Unchanged: 1, Slots: [],
			JournalPath: '/tmp/PREVIOUS-journal.log', SnapshotPath: '/tmp/PREVIOUS-snap.json',
		},
		Message: '',
	}

	const NEW_OUTCOME = {
		Kind: 'send', Outcome: 'ok',
		Report: {
			Written: 2, Verified: 2, SkippedBlocked: 0, Unchanged: 8, Slots: [],
			JournalPath: '/tmp/NEW-journal.log', SnapshotPath: '/tmp/NEW-snap.json',
		},
		Message: '',
	}

	it('stays in transferring (not closable, Cancel-transfer shown, previous outcome absent) until THIS send completes, then shows the NEW outcome', async () => {
		// Seed appState as it would sit after a prior send completed earlier
		// in the same session — the exact condition beginTransfer must
		// neutralise and the phase effect must not be fooled by.
		appState.applyTransferDone(PREVIOUS_OUTCOME)
		expect(appState.transfer.lastOutcome).toEqual(PREVIOUS_OUTCOME)

		// The mock mirrors the REAL confirmSend (bindings.js): it calls the
		// state module's own beginTransfer('send') synchronously and does
		// NOT resolve `active`/`lastOutcome` itself — only a later
		// transfer:done (applyTransferDone) does that.
		confirmSendMock.mockImplementation(async () => {
			appState.beginTransfer('send')
		})

		const onClose = vi.fn()
		const { container } = render(SendFlowDialog, { plan: planFixture(), onClose, onPrepareAgain: vi.fn() })
		await fireEvent.click(screen.getByRole('button', { name: 'Confirm send' }))

		// Must be sitting in 'transferring', not skipping straight to
		// 'result' on the strength of the stale lastOutcome.
		expect(await screen.findByText('Sending to radio…')).toBeInTheDocument()
		expect(screen.getByRole('button', { name: 'Cancel transfer' })).toBeInTheDocument()
		expect(screen.queryByText('Send complete')).not.toBeInTheDocument()
		expect(screen.queryByText(/PREVIOUS-journal\.log/)).not.toBeInTheDocument()
		expect(screen.queryByText(/PREVIOUS-snap\.json/)).not.toBeInTheDocument()

		// Not closable mid-transfer, even with a stale lastOutcome sitting
		// in state.
		const dialog = container.querySelector('[role="dialog"]')
		await fireEvent.keyDown(dialog, { key: 'Escape' })
		expect(onClose).not.toHaveBeenCalled()

		// THIS send's own transfer:done arrives.
		appState.applyTransferDone(NEW_OUTCOME)

		expect(await screen.findByText('Send complete')).toBeInTheDocument()
		expect(screen.getByText(/NEW-journal\.log/)).toBeInTheDocument()
		expect(screen.getByText(/NEW-snap\.json/)).toBeInTheDocument()
		expect(screen.queryByText(/PREVIOUS-journal\.log/)).not.toBeInTheDocument()
		expect(screen.queryByText(/PREVIOUS-snap\.json/)).not.toBeInTheDocument()
	})
})
