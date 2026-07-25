// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, beforeEach } from 'vitest'
import { appState } from '../app.svelte.js'

/** Resets every field back to its module-load default. appState is a
 * singleton (same instance across every test in this file), so each test
 * needs a clean slate — going through the class's own setters/mutators
 * where they exist, matching how production code would reset them. */
function resetState() {
	appState.clearConnection()
	appState.setPorts([])
	appState.setPortsLoading(false)
	appState.setConnecting(false)
	appState.setUISpec(null)
	appState.setSettingsSpec(null)
	appState.setSettings(null)
	appState.setActiveView('channels')
	appState.alerts = []
}

beforeEach(() => {
	resetState()
})

describe('connection state transitions', () => {
	it('starts disconnected', () => {
		expect(appState.connected).toBe(false)
		expect(appState.isDemo).toBe(false)
		expect(appState.connection).toBeNull()
	})

	it('setConnection makes connected true and exposes the connection', () => {
		appState.setConnection({ Model: 'FT-710', CATID: '0800', Port: 'COM3', USBSerial: '', Region: '', Demo: false })
		expect(appState.connected).toBe(true)
		expect(appState.isDemo).toBe(false)
		expect(appState.connection?.Model).toBe('FT-710')
	})

	it('isDemo reflects ConnectionInfo.Demo', () => {
		appState.setConnection({ Model: 'FT-710', CATID: '0800', Port: 'fake', USBSerial: 'SIM0001', Region: '', Demo: true })
		expect(appState.isDemo).toBe(true)
	})

	it('clearConnection resets connection, codeplug, dirty, issues and transfer', () => {
		appState.setConnection({ Model: 'FT-710', CATID: '0800', Port: 'COM3', USBSerial: '', Region: '', Demo: false })
		appState.setCodeplug({ Schema: 1, Generator: 'x', Radio: {}, Channels: [], WorkingPath: '', Dirty: true, BaselineStale: false })
		appState.setIssues([{ Slot: '001', Field: 'freq', Severity: 'error', Msg: 'bad' }])
		appState.dirtyTransferConflicts = true
		appState.beginTransfer('read')

		appState.clearConnection()

		expect(appState.connection).toBeNull()
		expect(appState.codeplug).toBeNull()
		expect(appState.dirty).toBe(false)
		expect(appState.issues).toEqual([])
		expect(appState.dirtyTransferConflicts).toBe(false)
		expect(appState.transfer.active).toBe(false)
		expect(appState.transfer.kind).toBeNull()
	})

	it('disconnectConnection (Fix 1, Codex M6 #1, adjudicated HIGH) clears ONLY connection-scoped state — codeplug/dirty/issues/workingPath persist', () => {
		appState.setConnection({ Model: 'FT-710', CATID: '0800', Port: 'COM3', USBSerial: '', Region: '', Demo: false })
		appState.setCodeplug({
			Schema: 1, Generator: 'x', Radio: {},
			Channels: [{ slot: '001', data: { freq_hz: 7100000 } }],
			WorkingPath: '/tmp/edited.json', Dirty: true, BaselineStale: false,
		})
		appState.setIssues([{ Slot: '001', Field: 'freq', Severity: 'error', Msg: 'bad' }])
		appState.dirtyTransferConflicts = true
		appState.beginTransfer('read')

		appState.disconnectConnection()

		// Connection-scoped state IS cleared.
		expect(appState.connection).toBeNull()
		expect(appState.dirtyTransferConflicts).toBe(false)
		expect(appState.transfer.active).toBe(false)
		expect(appState.transfer.kind).toBeNull()

		// Unsaved work must never be stranded by a disconnect — Go's own
		// Disconnect keeps the working copy; the frontend must not diverge
		// from that by wiping its view of it.
		expect(appState.codeplug).not.toBeNull()
		expect(appState.codeplug.WorkingPath).toBe('/tmp/edited.json')
		expect(appState.codeplug.Channels[0].data.freq_hz).toBe(7100000)
		expect(appState.dirty).toBe(true)
		expect(appState.issues).toEqual([{ Slot: '001', Field: 'freq', Severity: 'error', Msg: 'bad' }])

		// The baseline can no longer be asserted fresh once disconnected
		// from the radio that would need re-reading to refresh it.
		expect(appState.codeplug.BaselineStale).toBe(true)
	})
})

describe('progress event updates', () => {
	it('applyProgress marks the transfer active and mirrors the payload (lower-cased)', () => {
		appState.applyProgress({ Phase: 'write', Done: 4, Total: 12, Slot: 'CH-014' })
		expect(appState.transfer.active).toBe(true)
		expect(appState.transfer.progress).toEqual({
			phase: 'write',
			done: 4,
			total: 12,
			slot: 'CH-014',
			targetKind: '',
			targetId: '',
			targetDisplay: '',
		})
	})

	it('applyProgress stores TargetKind/TargetID/TargetDisplay (task 36, M8b-6 — every progress event carries the target triple now)', () => {
		appState.applyProgress({
			Phase: 'read-settings',
			Done: 3,
			Total: 9,
			Slot: '',
			TargetKind: 'setting',
			TargetID: '990101',
			TargetDisplay: '1-01',
		})
		expect(appState.transfer.progress.targetKind).toBe('setting')
		expect(appState.transfer.progress.targetId).toBe('990101')
		expect(appState.transfer.progress.targetDisplay).toBe('1-01')
	})

	it('beginTransfer resets progress to zero and sets kind', () => {
		appState.applyProgress({ Phase: 'write', Done: 4, Total: 12, Slot: 'CH-014' })
		appState.beginTransfer('send')
		expect(appState.transfer.active).toBe(true)
		expect(appState.transfer.kind).toBe('send')
		expect(appState.transfer.progress).toEqual({
			phase: '',
			done: 0,
			total: 0,
			slot: '',
			targetKind: '',
			targetId: '',
			targetDisplay: '',
		})
	})

	it('endTransfer clears active without touching kind or lastOutcome', () => {
		appState.beginTransfer('prepare')
		appState.applyProgress({ Phase: 'read', Done: 1, Total: 1, Slot: '001' })
		appState.endTransfer()
		expect(appState.transfer.active).toBe(false)
		expect(appState.transfer.kind).toBe('prepare')
	})

	it('applyTransferDone clears active, records kind/outcome/report', () => {
		appState.beginTransfer('read')
		appState.applyTransferDone({ Kind: 'read', Outcome: 'ok', Report: null, Message: '' })
		expect(appState.transfer.active).toBe(false)
		expect(appState.transfer.kind).toBe('read')
		expect(appState.transfer.lastOutcome).toEqual({ Kind: 'read', Outcome: 'ok', Report: null, Message: '' })
	})

	it('applyTransferDone falls back to the existing kind if the payload omits one', () => {
		appState.beginTransfer('send')
		appState.applyTransferDone({ Outcome: 'ok', Report: null, Message: '' })
		expect(appState.transfer.kind).toBe('send')
	})
})

describe('canSend pieces', () => {
	function makeReadyState() {
		appState.setConnection({ Model: 'FT-710', CATID: '0800', Port: 'COM3', USBSerial: '', Region: '', Demo: false })
		appState.setCodeplug({ Schema: 1, Generator: 'x', Radio: {}, Channels: [], WorkingPath: '', Dirty: false, BaselineStale: false })
	}

	it('is true when connected, baseline fresh, no blocking issues, no conflicts, no active transfer', () => {
		makeReadyState()
		expect(appState.canSend).toBe(true)
	})

	it('is false when not connected', () => {
		makeReadyState()
		appState.setConnection(null)
		expect(appState.canSend).toBe(false)
	})

	it('is false when the baseline is stale', () => {
		makeReadyState()
		appState.setCodeplug({ Schema: 1, Generator: 'x', Radio: {}, Channels: [], WorkingPath: '', Dirty: false, BaselineStale: true })
		expect(appState.baselineFresh).toBe(false)
		expect(appState.canSend).toBe(false)
	})

	it('is false when no codeplug has ever loaded (baselineFresh is false, not unknown)', () => {
		appState.setConnection({ Model: 'FT-710', CATID: '0800', Port: 'COM3', USBSerial: '', Region: '', Demo: false })
		expect(appState.baselineFresh).toBe(false)
		expect(appState.canSend).toBe(false)
	})

	it('is false when there is a blocking (severity "error") issue', () => {
		makeReadyState()
		appState.setIssues([{ Slot: '001', Field: 'freq', Severity: 'error', Msg: 'bad' }])
		expect(appState.blockingIssues).toHaveLength(1)
		expect(appState.canSend).toBe(false)
	})

	it('is true when issues are only severity "warning"', () => {
		makeReadyState()
		appState.setIssues([{ Slot: '001', Field: 'tone', Severity: 'warning', Msg: 'hmm' }])
		expect(appState.blockingIssues).toHaveLength(0)
		expect(appState.canSend).toBe(true)
	})

	it('is false when dirtyTransferConflicts is set', () => {
		makeReadyState()
		appState.dirtyTransferConflicts = true
		expect(appState.canSend).toBe(false)
	})

	it('is false while a transfer is active', () => {
		makeReadyState()
		appState.beginTransfer('send')
		expect(appState.canSend).toBe(false)
	})
})

describe('uiSpec (task 17)', () => {
	it('starts null and stores whatever setUISpec is given', () => {
		expect(appState.uiSpec).toBeNull()
		const spec = { Live: false, Banks: [], Modes: ['LSB'], ShiftOptions: [], CTCSSStateOptions: [], Tones: [], TagMaxBytes: 12, ClarMaxHz: 9990, ClarStepHz: 10 }
		appState.setUISpec(spec)
		expect(appState.uiSpec).toEqual(spec)
		appState.setUISpec(null)
		expect(appState.uiSpec).toBeNull()
	})
})

describe('activeView / settingsSpec / settings (task 36, M8b-6)', () => {
	it('activeView defaults to "channels"; setActiveView stores either value', () => {
		expect(appState.activeView).toBe('channels')
		appState.setActiveView('settings')
		expect(appState.activeView).toBe('settings')
		appState.setActiveView('channels')
		expect(appState.activeView).toBe('channels')
	})

	it('settingsSpec starts null and stores whatever setSettingsSpec is given', () => {
		expect(appState.settingsSpec).toBeNull()
		const spec = { Live: false, DescriptorVersion: 'v1', Menus: [{ ID: 'M1', Label: 'MENU ALPHA', Groups: [] }] }
		appState.setSettingsSpec(spec)
		expect(appState.settingsSpec).toEqual(spec)
		appState.setSettingsSpec(null)
		expect(appState.settingsSpec).toBeNull()
	})

	it('settings starts null and stores whatever setSettings is given', () => {
		expect(appState.settings).toBeNull()
		const settings = { HasSnapshot: true, Descriptor: 'v1', Complete: true, HasLegacy: false, Entries: [{ ID: '990101', Value: 'ON', State: 'known' }] }
		appState.setSettings(settings)
		expect(appState.settings).toEqual(settings)
	})

	it('clearConnection nulls settings (same working-copy content as codeplug) but leaves settingsSpec alone (mirrors uiSpec — meaningful offline)', () => {
		const spec = { Live: false, DescriptorVersion: 'v1', Menus: [] }
		appState.setSettingsSpec(spec)
		appState.setSettings({ HasSnapshot: true, Descriptor: 'v1', Complete: true, HasLegacy: false, Entries: [] })

		appState.clearConnection()

		expect(appState.settings).toBeNull()
		expect(appState.settingsSpec).toEqual(spec)
	})

	it('disconnectConnection leaves BOTH settings and settingsSpec untouched — bindings.js refreshes settingsSpec separately, exactly like uiSpec', () => {
		appState.setConnection({ Model: 'FT-710', CATID: '0800', Port: 'COM3', USBSerial: '', Region: '', Demo: false })
		const spec = { Live: true, DescriptorVersion: 'v1', Menus: [] }
		appState.setSettingsSpec(spec)
		const settings = { HasSnapshot: true, Descriptor: 'v1', Complete: true, HasLegacy: false, Entries: [{ ID: '990101', Value: 'ON', State: 'known' }] }
		appState.setSettings(settings)

		appState.disconnectConnection()

		expect(appState.settings).toEqual(settings)
		expect(appState.settingsSpec).toEqual(spec)
	})
})

describe('canReadSettings / readSettingsBlockedReason (task 36, M8b-6)', () => {
	function makeReadyState() {
		appState.setConnection({ Model: 'FT-710', CATID: '0800', Port: 'COM3', USBSerial: '', Region: '', Demo: false })
		appState.setCodeplug({ Schema: 1, Generator: 'x', Radio: {}, Channels: [], WorkingPath: '', Dirty: false, BaselineStale: false })
	}

	it('is true, with an empty reason, when connected + loaded + idle', () => {
		makeReadyState()
		expect(appState.canReadSettings).toBe(true)
		expect(appState.readSettingsBlockedReason).toBe('')
	})

	it('explains a missing connection first', () => {
		expect(appState.canReadSettings).toBe(false)
		expect(appState.readSettingsBlockedReason).toMatch(/connect/i)
	})

	it('explains an active transfer even though connected + loaded', () => {
		makeReadyState()
		appState.beginTransfer('read')
		expect(appState.canReadSettings).toBe(false)
		expect(appState.readSettingsBlockedReason).toMatch(/transfer/i)
	})

	it('explains a missing codeplug once connected', () => {
		appState.setConnection({ Model: 'FT-710', CATID: '0800', Port: 'COM3', USBSerial: '', Region: '', Demo: false })
		expect(appState.canReadSettings).toBe(false)
		expect(appState.readSettingsBlockedReason).toMatch(/read the radio|open a codeplug/i)
	})
})

describe('issuesAdvisory (task 17; storage model superseded by Fix 5, adjudicated MED, Codex M6 #5)', () => {
	it('defaults to true (advisory) before any Validate pass has ever run', () => {
		expect(appState.issuesAdvisory).toBe(true)
	})

	it('setIssuesAdvisory stores the flag verbatim — it is no longer DERIVED from `connected` (Fix 5: connecting alone must not silently relabel stale offline issues as authoritative; only an actual Validate pass may)', () => {
		appState.setConnection({ Model: 'FT-710', CATID: '0800', Port: 'COM3', USBSerial: '', Region: '', Demo: false })
		// Connecting alone does NOT flip it — a genuine caller (bindings.js's
		// revalidateQuiet) must do that from a real ValidationView.
		expect(appState.issuesAdvisory).toBe(true)

		appState.setIssuesAdvisory(false)
		expect(appState.issuesAdvisory).toBe(false)
	})

	it('clearConnection resets it back to true (advisory)', () => {
		appState.setIssuesAdvisory(false)
		appState.clearConnection()
		expect(appState.issuesAdvisory).toBe(true)
	})
})

describe('applyChannelEdits (task 17)', () => {
	const channels = () => [
		{ slot: '001', data: { freq_hz: 7074000, mode: 'USB' } },
		{ slot: '002', data: null },
		{ slot: '003', data: { freq_hz: 14074000, mode: 'USB' } },
	]

	it('replaces exactly the matching slots, wholesale (mirroring Go applyEditsLocked)', () => {
		appState.setCodeplug({ Schema: 1, Generator: 'x', Radio: {}, Channels: channels(), WorkingPath: '', Dirty: false, BaselineStale: false })

		appState.applyChannelEdits([
			{ slot: '002', data: { freq_hz: 7100000, mode: 'LSB' } },
		])

		const bySlot = new Map(appState.codeplug.Channels.map((ch) => [ch.slot, ch]))
		expect(bySlot.get('002')?.data).toEqual({ freq_hz: 7100000, mode: 'LSB' })
		expect(bySlot.get('001')?.data).toEqual({ freq_hz: 7074000, mode: 'USB' }) // untouched
		expect(appState.codeplug.Channels).toHaveLength(3) // never adds/removes slots
	})

	it('applies a whole batch at once', () => {
		appState.setCodeplug({ Schema: 1, Generator: 'x', Radio: {}, Channels: channels(), WorkingPath: '', Dirty: false, BaselineStale: false })

		appState.applyChannelEdits([
			{ slot: '001', data: { freq_hz: 7000000, mode: 'CW-U' } },
			{ slot: '003', data: { freq_hz: 21074000, mode: 'USB' } },
		])

		const bySlot = new Map(appState.codeplug.Channels.map((ch) => [ch.slot, ch]))
		expect(bySlot.get('001')?.data?.freq_hz).toBe(7000000)
		expect(bySlot.get('003')?.data?.freq_hz).toBe(21074000)
	})

	it('is a no-op when no codeplug is loaded', () => {
		expect(appState.codeplug).toBeNull()
		appState.applyChannelEdits([{ slot: '001', data: { freq_hz: 7000000, mode: 'USB' } }])
		expect(appState.codeplug).toBeNull()
	})
})

describe('alerts', () => {
	it('pushAlert appends and returns an id; dismissAlert removes it', () => {
		const id = appState.pushAlert('reading radio: boom')
		expect(appState.alerts).toHaveLength(1)
		expect(appState.alerts[0]).toEqual({ id, message: 'reading radio: boom', kind: 'error' })

		appState.dismissAlert(id)
		expect(appState.alerts).toHaveLength(0)
	})

	it('assigns distinct ids to successive alerts', () => {
		const a = appState.pushAlert('one')
		const b = appState.pushAlert('two')
		expect(a).not.toBe(b)
		expect(appState.alerts).toHaveLength(2)
	})

	it('defaults to kind "error"; a caller can push an informational toast instead (task 18)', () => {
		appState.pushAlert('nothing to send')
		appState.pushAlert('exported to /tmp/out.csv', 'info')
		expect(appState.alerts[0].kind).toBe('error')
		expect(appState.alerts[1].kind).toBe('info')
	})
})

describe('sendBlockedReason (task 18)', () => {
	function makeReadyState() {
		appState.setConnection({ Model: 'FT-710', CATID: '0800', Port: 'COM3', USBSerial: '', Region: '', Demo: false })
		appState.setCodeplug({ Schema: 1, Generator: 'x', Radio: {}, Channels: [], WorkingPath: '', Dirty: false, BaselineStale: false })
	}

	it('is empty when canSend is true', () => {
		makeReadyState()
		expect(appState.canSend).toBe(true)
		expect(appState.sendBlockedReason).toBe('')
	})

	it('explains a missing connection first', () => {
		expect(appState.sendBlockedReason).toMatch(/connect/i)
	})

	it('explains an active transfer even though connected', () => {
		makeReadyState()
		appState.beginTransfer('send')
		expect(appState.sendBlockedReason).toMatch(/transfer/i)
	})

	it('explains a missing codeplug once connected', () => {
		appState.setConnection({ Model: 'FT-710', CATID: '0800', Port: 'COM3', USBSerial: '', Region: '', Demo: false })
		expect(appState.sendBlockedReason).toMatch(/read the radio|open a codeplug/i)
	})

	it('explains a stale baseline', () => {
		makeReadyState()
		appState.setCodeplug({ Schema: 1, Generator: 'x', Radio: {}, Channels: [], WorkingPath: '', Dirty: false, BaselineStale: true })
		expect(appState.sendBlockedReason).toMatch(/stale/i)
	})

	it('explains blocking issues, pluralised correctly', () => {
		makeReadyState()
		appState.setIssues([{ Slot: '001', Field: 'freq', Severity: 'error', Msg: 'bad' }])
		expect(appState.sendBlockedReason).toBe('1 validation error needs to be fixed first')

		appState.setIssues([
			{ Slot: '001', Field: 'freq', Severity: 'error', Msg: 'bad' },
			{ Slot: '002', Field: 'freq', Severity: 'error', Msg: 'bad' },
		])
		expect(appState.sendBlockedReason).toBe('2 validation errors need to be fixed first')
	})
})

describe('applyTransferDone marks the baseline stale after a send (task 18)', () => {
	it('sets codeplug.BaselineStale when a send transfer:done carries a Report', () => {
		appState.setCodeplug({ Schema: 1, Generator: 'x', Radio: {}, Channels: [], WorkingPath: '', Dirty: false, BaselineStale: false })
		appState.beginTransfer('send')
		appState.applyTransferDone({ Kind: 'send', Outcome: 'ok', Report: { Written: 1 }, Message: '' })
		expect(appState.codeplug.BaselineStale).toBe(true)
	})

	it('leaves BaselineStale alone for a refusal with no Report', () => {
		appState.setCodeplug({ Schema: 1, Generator: 'x', Radio: {}, Channels: [], WorkingPath: '', Dirty: false, BaselineStale: false })
		appState.beginTransfer('send')
		appState.applyTransferDone({ Kind: 'send', Outcome: 'refused', Report: null, Message: 'refused: stale baseline' })
		expect(appState.codeplug.BaselineStale).toBe(false)
	})

	it('leaves BaselineStale alone for a read/diff transfer', () => {
		appState.setCodeplug({ Schema: 1, Generator: 'x', Radio: {}, Channels: [], WorkingPath: '', Dirty: false, BaselineStale: false })
		appState.beginTransfer('diff')
		appState.applyTransferDone({ Kind: 'diff', Outcome: 'ok', Report: null, Message: '' })
		expect(appState.codeplug.BaselineStale).toBe(false)
	})

	it('is a no-op when no codeplug is loaded', () => {
		appState.beginTransfer('send')
		expect(() => appState.applyTransferDone({ Kind: 'send', Outcome: 'ok', Report: { Written: 1 }, Message: '' })).not.toThrow()
		expect(appState.codeplug).toBeNull()
	})
})
