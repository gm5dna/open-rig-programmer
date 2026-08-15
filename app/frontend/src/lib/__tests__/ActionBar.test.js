// SPDX-License-Identifier: GPL-3.0-or-later

// ActionBar tests (task-18 brief §3): the Send-button gating matrix +
// tooltip, the dirty-guard flows for Open and Read Radio, and the
// Import CSV/CHIRP/Export CSV wiring (success toast vs. the loss/refusal
// panel). SendFlowDialog's own internals are covered by
// SendFlowDialog.test.js — here we only check ActionBar opens it with
// the right plan and that Confirm is never reachable without one.

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/svelte'
import { appState } from '../state/app.svelte.js'
import ActionBar from '../ActionBar.svelte'

vi.mock('../bridge/bindings.js', () => ({
	readRadio: vi.fn().mockResolvedValue({}),
	loadFile: vi.fn().mockResolvedValue({}),
	saveFile: vi.fn().mockResolvedValue(undefined),
	saveFileAs: vi.fn().mockResolvedValue('/tmp/saved.json'),
	prepareSend: vi.fn(),
	confirmSend: vi.fn(),
	cancelTransfer: vi.fn(),
	importCSV: vi.fn(),
	importCHIRP: vi.fn(),
	exportCSV: vi.fn(),
}))

import {
	readRadio,
	loadFile,
	saveFile,
	saveFileAs,
	prepareSend,
	importCSV,
	importCHIRP,
	exportCSV,
} from '../bridge/bindings.js'

const readRadioMock = vi.mocked(readRadio)
const loadFileMock = vi.mocked(loadFile)
const saveFileMock = vi.mocked(saveFile)
const saveFileAsMock = vi.mocked(saveFileAs)
const prepareSendMock = vi.mocked(prepareSend)
const importCSVMock = vi.mocked(importCSV)
const importCHIRPMock = vi.mocked(importCHIRP)
const exportCSVMock = vi.mocked(exportCSV)

const CODEPLUG = { Schema: 1, Generator: 'x', Radio: {}, Channels: [], WorkingPath: '', Dirty: false, BaselineStale: false }

function connectAndLoad() {
	appState.setConnection({ Model: 'FT-710', CATID: '0800', Port: 'COM3', USBSerial: '', Region: '', Demo: false })
	appState.setCodeplug(CODEPLUG)
}

beforeEach(() => {
	appState.clearConnection()
	appState.alerts = []
	appState.setIssues([])
	vi.clearAllMocks()
	readRadioMock.mockResolvedValue({})
	loadFileMock.mockResolvedValue({})
	saveFileMock.mockResolvedValue(undefined)
	saveFileAsMock.mockResolvedValue('/tmp/saved.json')
})

function sendButton() {
	return screen.getByRole('button', { name: 'Send to Radio' })
}

describe('Send button gating matrix', () => {
	it('disabled, "Connect to a radio first", when not connected', () => {
		render(ActionBar)
		expect(sendButton()).toBeDisabled()
		expect(sendButton().closest('span')).toHaveAttribute('title', 'Connect to a radio first')
	})

	it('disabled, explains a missing codeplug, once connected with nothing loaded', () => {
		appState.setConnection({ Model: 'FT-710', CATID: '0800', Port: 'COM3', USBSerial: '', Region: '', Demo: false })
		render(ActionBar)
		expect(sendButton()).toBeDisabled()
		expect(sendButton().closest('span')?.title).toMatch(/read the radio|open a codeplug/i)
	})

	it('disabled, explains blocking issues, when connected+loaded but a severity "error" issue exists', () => {
		connectAndLoad()
		appState.setIssues([{ Slot: '001', Field: 'freq', Severity: 'error', Msg: 'bad' }])
		render(ActionBar)
		expect(sendButton()).toBeDisabled()
		expect(sendButton().closest('span')?.title).toMatch(/validation error/i)
	})

	it('disabled while a transfer is active', () => {
		connectAndLoad()
		appState.beginTransfer('read')
		render(ActionBar)
		expect(sendButton()).toBeDisabled()
		expect(sendButton().closest('span')?.title).toMatch(/transfer/i)
	})

	it('enabled, no tooltip, when connected + loaded + baseline fresh + no blocking issues + idle', () => {
		connectAndLoad()
		render(ActionBar)
		expect(sendButton()).not.toBeDisabled()
		expect(sendButton().title).toBe('')
	})

	it('enabled even with a warning-only issue', () => {
		connectAndLoad()
		appState.setIssues([{ Slot: '001', Field: 'tone', Severity: 'warning', Msg: 'hmm' }])
		render(ActionBar)
		expect(sendButton()).not.toBeDisabled()
	})
})

describe('Send flow wiring', () => {
	it('click calls prepareSend and opens the review dialogue with the returned plan', async () => {
		connectAndLoad()
		const plan = {
			Diff: { Added: [], Modified: [], Erased: [], Counts: { Added: 0, Modified: 0, Erased: 0, Blocked: 0, Unchanged: 10 } },
			SnapshotPath: '/tmp/snap.json', BaselineDigestShort: 'abc123', ConfirmationDigest: 'tok', NothingToSend: false, FirmwareRequired: false,
		}
		prepareSendMock.mockResolvedValue(plan)
		render(ActionBar)
		await fireEvent.click(sendButton())
		expect(prepareSendMock).toHaveBeenCalledTimes(1)
		expect(await screen.findByText('Review before sending')).toBeInTheDocument()
	})

	it('NothingToSend shows a friendly info toast, no dialogue', async () => {
		connectAndLoad()
		prepareSendMock.mockResolvedValue({
			Diff: { Added: [], Modified: [], Erased: [], Counts: { Added: 0, Modified: 0, Erased: 0, Blocked: 0, Unchanged: 10 } },
			SnapshotPath: '/tmp/snap.json', BaselineDigestShort: 'abc123', ConfirmationDigest: 'tok', NothingToSend: true, FirmwareRequired: false,
		})
		render(ActionBar)
		await fireEvent.click(sendButton())
		expect(screen.queryByText('Review before sending')).not.toBeInTheDocument()
		expect(appState.alerts).toHaveLength(1)
		expect(appState.alerts[0].kind).toBe('info')
		expect(appState.alerts[0].message).toMatch(/nothing to send/i)
	})

	it('NothingToSend with blocked entries (e.g. a pending delete) opens the review dialogue informationally instead of the toast (task-25 brief: not a genuine parity case)', async () => {
		connectAndLoad()
		prepareSendMock.mockResolvedValue({
			Diff: {
				Added: [], Modified: [],
				Erased: [
					{ Slot: '010', SlotDisplay: 'M-10', Bank: 'MEM', Kind: 'erased', Before: null, Blocked: true, BlockReason: 'erase not supported on this radio' },
				],
				Counts: { Added: 0, Modified: 0, Erased: 1, Blocked: 1, Unchanged: 9 },
			},
			SnapshotPath: '/tmp/snap.json', BaselineDigestShort: 'abc123', ConfirmationDigest: 'tok', NothingToSend: true, FirmwareRequired: false,
		})
		render(ActionBar)
		await fireEvent.click(sendButton())
		expect(await screen.findByText('Review before sending')).toBeInTheDocument()
		expect(appState.alerts).toHaveLength(0)
		expect(screen.getByText(/erase not supported on this radio/)).toBeInTheDocument()
	})
})

describe('dirty guard — Open / Read Radio', () => {
	it('Open with no unsaved changes calls loadFile directly, no dialogue', async () => {
		render(ActionBar)
		await fireEvent.click(screen.getByRole('button', { name: 'Open…' }))
		expect(loadFileMock).toHaveBeenCalledTimes(1)
		expect(screen.queryByText('Unsaved changes')).not.toBeInTheDocument()
	})

	it('Open while dirty shows the guard; Cancel leaves loadFile uncalled', async () => {
		appState.setDirty(true)
		render(ActionBar)
		await fireEvent.click(screen.getByRole('button', { name: 'Open…' }))
		expect(await screen.findByText('Unsaved changes')).toBeInTheDocument()
		expect(loadFileMock).not.toHaveBeenCalled()
		await fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
		expect(screen.queryByText('Unsaved changes')).not.toBeInTheDocument()
		expect(loadFileMock).not.toHaveBeenCalled()
	})

	it('Open while dirty, Discard changes proceeds with loadFile', async () => {
		appState.setDirty(true)
		render(ActionBar)
		await fireEvent.click(screen.getByRole('button', { name: 'Open…' }))
		await fireEvent.click(screen.getByRole('button', { name: 'Discard changes' }))
		expect(loadFileMock).toHaveBeenCalledTimes(1)
	})

	it('Open while dirty, Save first saves via the existing working path then loads', async () => {
		appState.setCodeplug({ ...CODEPLUG, WorkingPath: '/tmp/existing.json', Dirty: true })
		render(ActionBar)
		await fireEvent.click(screen.getByRole('button', { name: 'Open…' }))
		await fireEvent.click(screen.getByRole('button', { name: 'Save first' }))
		expect(saveFileMock).toHaveBeenCalledWith('/tmp/existing.json')
		expect(loadFileMock).toHaveBeenCalledTimes(1)
	})

	it('Read Radio while dirty shows the guard, customised for reading', async () => {
		connectAndLoad()
		appState.setDirty(true)
		render(ActionBar)
		await fireEvent.click(screen.getByRole('button', { name: 'Read Radio' }))
		expect(await screen.findByText(/Reading the radio/)).toBeInTheDocument()
		expect(readRadioMock).not.toHaveBeenCalled()
		await fireEvent.click(screen.getByRole('button', { name: 'Discard changes' }))
		expect(readRadioMock).toHaveBeenCalledTimes(1)
	})

	it('Save first with no working path yet falls back to the save dialog', async () => {
		appState.setCodeplug({ ...CODEPLUG, Dirty: true })
		saveFileAsMock.mockResolvedValue('/tmp/new.json')
		render(ActionBar)
		await fireEvent.click(screen.getByRole('button', { name: 'Open…' }))
		await fireEvent.click(screen.getByRole('button', { name: 'Save first' }))
		expect(saveFileAsMock).toHaveBeenCalledTimes(1)
		expect(loadFileMock).toHaveBeenCalledTimes(1)
	})

	it('Save first: cancelling the save dialog abandons the guarded action too', async () => {
		appState.setCodeplug({ ...CODEPLUG, Dirty: true })
		saveFileAsMock.mockResolvedValue('') // user cancelled
		render(ActionBar)
		await fireEvent.click(screen.getByRole('button', { name: 'Open…' }))
		await fireEvent.click(screen.getByRole('button', { name: 'Save first' }))
		expect(loadFileMock).not.toHaveBeenCalled()
	})
})

describe('Import CSV / CHIRP', () => {
	it('a clean success (no losses) shows a toast, no dialogue', async () => {
		connectAndLoad()
		importCSVMock.mockResolvedValue({ Path: '/tmp/in.csv', Merged: true, LossEntries: [], Issues: [], Dirty: true })
		render(ActionBar)
		await fireEvent.click(screen.getByRole('button', { name: 'Import CSV…' }))
		expect(screen.queryByText(/import complete/i)).not.toBeInTheDocument()
		expect(appState.alerts[0].message).toMatch(/imported/i)
		expect(appState.alerts[0].kind).toBe('info')
	})

	it('a refused import (nothing merged) shows the report honestly', async () => {
		connectAndLoad()
		importCSVMock.mockResolvedValue({ Path: '/tmp/in.csv', Merged: false, RefusalReason: 'inventory mismatch: missing 001; extra 999', LossEntries: [] })
		render(ActionBar)
		await fireEvent.click(screen.getByRole('button', { name: 'Import CSV…' }))
		expect(await screen.findByText('CSV import refused')).toBeInTheDocument()
		expect(screen.getByText(/inventory mismatch/)).toBeInTheDocument()
	})

	it('a CHIRP success with non-blocking loss entries shows the dismissible panel', async () => {
		connectAndLoad()
		importCHIRPMock.mockResolvedValue({
			Path: '/tmp/in.csv', Merged: true, Dirty: true, Issues: [],
			LossEntries: [{ Line: 4, Column: 'Tone', Value: '99.9', Action: 'dropped', Detail: 'unsupported tone', Blocking: false }],
		})
		render(ActionBar)
		await fireEvent.click(screen.getByRole('button', { name: 'Import CHIRP…' }))
		expect(await screen.findByText('CHIRP import complete')).toBeInTheDocument()
		expect(screen.getByText('unsupported tone')).toBeInTheDocument()
		await fireEvent.click(screen.getByRole('button', { name: 'Dismiss' }))
		expect(screen.queryByText('CHIRP import complete')).not.toBeInTheDocument()
	})

	it('a cancelled dialog does nothing', async () => {
		connectAndLoad()
		importCSVMock.mockResolvedValue({ Cancelled: true })
		render(ActionBar)
		await fireEvent.click(screen.getByRole('button', { name: 'Import CSV…' }))
		expect(appState.alerts).toHaveLength(0)
		expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
	})

	it('Import buttons are disabled with a tooltip until a codeplug is loaded', () => {
		render(ActionBar)
		const btn = screen.getByRole('button', { name: 'Import CSV…' })
		expect(btn).toBeDisabled()
		expect(btn.closest('span')).toHaveAttribute('title', 'Open or read a codeplug first')
	})
})

describe('busy gating while a transfer is active (Codex M6 #2, adjudicated HIGH, remedy 2d)', () => {
	it('Save, Save As, Import CSV/CHIRP, and Export CSV are all disabled while a transfer is active — mirroring the Go-side reservation refusal', () => {
		connectAndLoad()
		appState.beginTransfer('read')
		render(ActionBar)
		expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
		expect(screen.getByRole('button', { name: 'Save As…' })).toBeDisabled()
		expect(screen.getByRole('button', { name: 'Import CSV…' })).toBeDisabled()
		expect(screen.getByRole('button', { name: 'Import CHIRP…' })).toBeDisabled()
		expect(screen.getByRole('button', { name: 'Export CSV…' })).toBeDisabled()
	})
})

describe('Export CSV', () => {
	it('success shows a toast with the path', async () => {
		connectAndLoad()
		exportCSVMock.mockResolvedValue('/tmp/out.csv')
		render(ActionBar)
		await fireEvent.click(screen.getByRole('button', { name: 'Export CSV…' }))
		expect(appState.alerts[0].message).toBe('Exported to /tmp/out.csv')
		expect(appState.alerts[0].kind).toBe('info')
	})

	it('a cancelled dialog (empty path) shows no toast', async () => {
		connectAndLoad()
		exportCSVMock.mockResolvedValue('')
		render(ActionBar)
		await fireEvent.click(screen.getByRole('button', { name: 'Export CSV…' }))
		expect(appState.alerts).toHaveLength(0)
	})
})

// --- Task 14 (M9d): the prepared plan vs a consent reconnect -------------

describe('prepared plan and the consent surface', () => {
	const PLAN = {
		Diff: { Added: [], Modified: [], Erased: [], Counts: { Added: 0, Modified: 0, Erased: 0, Blocked: 0, Unchanged: 10 } },
		SnapshotPath: '/tmp/snap.json', BaselineDigestShort: 'abc123', ConfirmationDigest: 'tok', NothingToSend: false, FirmwareRequired: false,
	}

	async function openSendDialogue() {
		connectAndLoad()
		prepareSendMock.mockResolvedValue(PLAN)
		render(ActionBar)
		await fireEvent.click(sendButton())
		expect(await screen.findByText('Review before sending')).toBeInTheDocument()
	}

	it('tells the rest of the app while a send dialogue is open — the consent surface refuses to change anything meanwhile', async () => {
		await openSendDialogue()
		expect(appState.sendDialogOpen).toBe(true)
		expect(appState.canChangeUnverifiedConsent).toBe(false)

		await fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
		expect(appState.sendDialogOpen).toBe(false)
	})

	it('drops its prepared plan when one is invalidated — a consent reconnect took the backend’s own plan with it', async () => {
		await openSendDialogue()

		appState.invalidatePreparedPlan()
		await screen.findByRole('button', { name: 'Send to Radio' })

		expect(screen.queryByText('Review before sending')).not.toBeInTheDocument()
		expect(appState.sendDialogOpen).toBe(false)
	})
})
