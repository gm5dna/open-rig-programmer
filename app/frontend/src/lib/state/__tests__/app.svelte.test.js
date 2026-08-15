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
	appState.setSupportedModels([])
	appState.setSelectedModel('')
	// Task 14 (M9d): the consent surface's own state — deliberately NOT
	// connection-scoped (see each field's doc comment), so, exactly like
	// uiSpec and the model picker's fields, a test that wants it clean
	// resets it here rather than relying on clearConnection.
	appState.setUnverifiedConsentPrompt(null)
	appState.setUnverifiedConsents([])
	appState.closeUnverifiedGrants()
	appState.setSendDialogOpen(false)
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

describe('model picker state (task 13, M9d — the GUI can finally name a radio)', () => {
	it('selectedModel defaults to "" — the empty string means wiring.DefaultModel, so an untouched picker connects exactly as the app did before it existed', () => {
		expect(appState.selectedModel).toBe('')
	})

	it('setSelectedModel stores the user\'s choice, and "" puts it back to the default', () => {
		appState.setSelectedModel('FTdx10')
		expect(appState.selectedModel).toBe('FTdx10')
		appState.setSelectedModel('')
		expect(appState.selectedModel).toBe('')
	})

	it('setSelectedModel coerces a null choice to "" rather than storing it — the connect path takes a string', () => {
		appState.setSelectedModel(null)
		expect(appState.selectedModel).toBe('')
	})

	it('supportedModels starts empty and stores whatever GetSupportedModels returned, in that order', () => {
		expect(appState.supportedModels).toEqual([])
		appState.setSupportedModels(['FT-710', 'FTDX101D', 'FTDX101MP', 'FTdx10'])
		expect(appState.supportedModels).toEqual(['FT-710', 'FTDX101D', 'FTDX101MP', 'FTdx10'])
	})

	it('setSupportedModels(null) leaves an empty list, never null — the picker always iterates an array', () => {
		appState.setSupportedModels(['FT-710'])
		appState.setSupportedModels(null)
		expect(appState.supportedModels).toEqual([])
	})

	it('a chosen model survives clearConnection — it is the picker\'s own choice, not connection-scoped state', () => {
		appState.setSelectedModel('FTdx10')
		appState.setSupportedModels(['FT-710', 'FTdx10'])
		appState.clearConnection()
		expect(appState.selectedModel).toBe('FTdx10')
		expect(appState.supportedModels).toEqual(['FT-710', 'FTdx10'])
	})

	it('a chosen model survives disconnectConnection too — reconnecting must offer the radio the user picked', () => {
		appState.setSelectedModel('FTdx10')
		appState.setSupportedModels(['FT-710', 'FTdx10'])
		appState.setConnection({ Model: 'FTdx10', CATID: '0761', Port: '/dev/tty.usb', USBSerial: '', Region: '', Demo: false })
		appState.disconnectConnection()
		expect(appState.connection).toBeNull()
		expect(appState.selectedModel).toBe('FTdx10')
		expect(appState.supportedModels).toEqual(['FT-710', 'FTdx10'])
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

// --- Task 14 (M9d): the unverified-write consent surface -----------------

/** A real connection to a consent-eligible radio with no decision recorded
 * — the one shape that must raise the arming dialogue. */
const NEEDS_CONSENT = {
	Model: 'FTdx10', CATID: '0761', Port: 'COM3', USBSerial: '', Region: '', Demo: false,
	NeedsUnverifiedConsent: true, UnverifiedConsentRecorded: false,
}

/** A minimal UISpecView carrying only the field the amber badge derives
 * from — the point of the badge's tests is that NOTHING else feeds it. */
/** @param {boolean} consented */
function specWithAmber(consented) {
	return { Live: true, Banks: [], Modes: [], ShiftOptions: [], CTCSSStateOptions: [], Tones: [], TagMaxBytes: 12, ClarMaxHz: 9990, ClarStepHz: 10, UnverifiedWritesConsented: consented }
}

describe('unverifiedConsentDue — when the arming dialogue is owed (task 14)', () => {
	it('is true after a real connect to a consent-eligible radio with no decision recorded', () => {
		appState.setConnection(NEEDS_CONSENT)
		expect(appState.unverifiedConsentDue).toBe(true)
	})

	it('is false once a decision is recorded — a DECLINE is a decision, so it is not re-asked', () => {
		appState.setConnection({ ...NEEDS_CONSENT, UnverifiedConsentRecorded: true })
		expect(appState.unverifiedConsentDue).toBe(false)
	})

	it('is false for a recorded GRANT too (the session is already armed — nothing to ask)', () => {
		appState.setConnection({ ...NEEDS_CONSENT, UnverifiedConsentRecorded: true })
		appState.setUISpec(specWithAmber(true))
		expect(appState.unverifiedConsentDue).toBe(false)
	})

	it('is false for a demo session, even one whose model would need consent on real hardware', () => {
		appState.setConnection({ ...NEEDS_CONSENT, Demo: true })
		expect(appState.unverifiedConsentDue).toBe(false)
	})

	it('is false for a radio whose writes are hardware-verified', () => {
		appState.setConnection({ ...NEEDS_CONSENT, Model: 'FT-710', NeedsUnverifiedConsent: false })
		expect(appState.unverifiedConsentDue).toBe(false)
	})

	it('is false while disconnected', () => {
		expect(appState.unverifiedConsentDue).toBe(false)
	})
})

describe('unverifiedWritesArmed — the amber indicator (task 14)', () => {
	it('derives from uiSpec.UnverifiedWritesConsented and nothing else', () => {
		appState.setUISpec(specWithAmber(true))
		expect(appState.unverifiedWritesArmed).toBe(true)
		appState.setUISpec(specWithAmber(false))
		expect(appState.unverifiedWritesArmed).toBe(false)
	})

	it('is false with no spec loaded at all', () => {
		expect(appState.unverifiedWritesArmed).toBe(false)
	})

	it('does NOT follow the connection or a recorded grant — only the live capability label', () => {
		// A recorded grant on a connected, consent-eligible radio, but a spec
		// whose capabilities carry no ConsentedUnverified: the session is not
		// armed, so the badge must stay dark.
		appState.setConnection({ ...NEEDS_CONSENT, UnverifiedConsentRecorded: true })
		appState.setUnverifiedConsents([{ Model: 'FTdx10', NeedsConsent: true, Granted: true, Recorded: true, Warning: 'w' }])
		appState.setUISpec(specWithAmber(false))
		expect(appState.unverifiedWritesArmed).toBe(false)
	})
})

describe('canChangeUnverifiedConsent — the busy guards (task 14)', () => {
	it('is true when idle', () => {
		expect(appState.canChangeUnverifiedConsent).toBe(true)
		expect(appState.consentChangeBlockedReason).toBe('')
	})

	it('is false while a transfer is running', () => {
		appState.beginTransfer('read')
		expect(appState.canChangeUnverifiedConsent).toBe(false)
		expect(appState.consentChangeBlockedReason).toMatch(/transfer/i)
	})

	it('is false while a send dialogue is open', () => {
		appState.setSendDialogOpen(true)
		expect(appState.canChangeUnverifiedConsent).toBe(false)
		expect(appState.consentChangeBlockedReason).toMatch(/send/i)
	})

	it('is false while a connect attempt is in flight', () => {
		appState.setConnecting(true)
		expect(appState.canChangeUnverifiedConsent).toBe(false)
		expect(appState.consentChangeBlockedReason).toMatch(/connect/i)
	})
})

describe('consent panel + prompt bookkeeping (task 14)', () => {
	it('setUnverifiedConsentPrompt stores and clears the arming dialogue', () => {
		const view = { Model: 'FTdx10', NeedsConsent: true, Granted: false, Recorded: false, Warning: 'w' }
		appState.setUnverifiedConsentPrompt(view)
		expect(appState.unverifiedConsentPrompt).toEqual(view)
		appState.setUnverifiedConsentPrompt(null)
		expect(appState.unverifiedConsentPrompt).toBeNull()
	})

	it('setUnverifiedConsents stores the rows verbatim; null/undefined becomes an empty list', () => {
		const rows = [
			{ Model: 'FT-710', NeedsConsent: false, Granted: false, Recorded: false, Warning: '' },
			{ Model: 'FTdx10', NeedsConsent: true, Granted: true, Recorded: true, Warning: 'w' },
		]
		appState.setUnverifiedConsents(rows)
		expect(appState.unverifiedConsents).toEqual(rows)
		appState.setUnverifiedConsents(null)
		expect(appState.unverifiedConsents).toEqual([])
	})

	it('open/closeUnverifiedGrants toggle the always-reachable panel', () => {
		expect(appState.unverifiedGrantsOpen).toBe(false)
		appState.openUnverifiedGrants()
		expect(appState.unverifiedGrantsOpen).toBe(true)
		appState.closeUnverifiedGrants()
		expect(appState.unverifiedGrantsOpen).toBe(false)
	})

	it('invalidatePreparedPlan bumps preparedPlanEpoch — the signal a prepared plan is stale', () => {
		const before = appState.preparedPlanEpoch
		appState.invalidatePreparedPlan()
		expect(appState.preparedPlanEpoch).toBe(before + 1)
		appState.invalidatePreparedPlan()
		expect(appState.preparedPlanEpoch).toBe(before + 2)
	})
})
