// SPDX-License-Identifier: GPL-3.0-or-later

// Bridge module tests: mock the wailsjs GLOBALS (window.go.*, window.runtime.*)
// that the generated wailsjs/go/main/App.js and wailsjs/runtime files call
// into — this exercises the real generated wrapper code, not a stand-in
// for it, matching how the actual Wails webview bridge is shaped (task-16
// brief §4: "mock wailsjs globals; assert subscriptions and fan-out").

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { appState } from '../../state/app.svelte.js'
import * as bindings from '../bindings.js'

function resetState() {
	appState.clearConnection()
	appState.setPorts([])
	appState.setPortsLoading(false)
	appState.setConnecting(false)
	appState.setUISpec(null)
	appState.setSettingsSpec(null)
	appState.setSettings(null)
	appState.setActiveView('channels')
	appState.setAppVersion(null)
	appState.setSupportedModels([])
	appState.setSelectedModel('')
	// Task 14 (M9d): the consent surface's own state — not connection-scoped
	// (see each field's doc comment in app.svelte.js), so reset explicitly.
	appState.setUnverifiedConsentPrompt(null)
	appState.setUnverifiedConsents([])
	appState.closeUnverifiedGrants()
	appState.setSendDialogOpen(false)
	appState.alerts = []
}

/** What the GetSupportedModels stub returns — this build really does
 * register four models (M9d-2), but these tests assert the list is passed
 * through untouched, so the exact membership is the stub's business. */
const SUPPORTED_MODELS = ['FT-710', 'FTDX101D', 'FTDX101MP', 'FTdx10']

/** A minimal, synthetic UISpecView for the GetUISpec stub. */
const UI_SPEC = {
	Live: false,
	Banks: [{ ID: 'MEM', Label: 'Memories', ReadOnly: false, Slots: [{ Slot: '001', Display: 'M-01' }] }],
	Modes: ['LSB', 'USB'],
	ShiftOptions: ['SIMPLEX', 'PLUS', 'MINUS'],
	CTCSSStateOptions: ['OFF', 'ENC-DEC', 'ENC'],
	Tones: [{ Decihertz: 885, Display: '88.5 Hz' }],
	TagMaxBytes: 12,
	ClarMaxHz: 9990,
	ClarStepHz: 10,
}

/** A minimal, synthetic SettingsSpecView for the GetSettingsSpec stub
 * (task 36, M8b-6) — deliberately invented labels/IDs, same discipline as
 * SettingsViewer.test.js's own fixture. */
const SETTINGS_SPEC = {
	Live: false,
	DescriptorVersion: 'synthetic-v1',
	Menus: [{ ID: 'M1', Label: 'MENU ALPHA', Groups: [] }],
}

/** A synthetic VersionView for the GetAppVersion stub. Deliberately
 * NOT the real build's version: these tests assert the value is passed
 * through untouched, which a fixture equal to the real one could not
 * distinguish from the frontend inventing it. */
const VERSION_VIEW = {
	Version: 'v9.9.9',
	Display: 'v9.9.9',
	IsRelease: true,
}

/** A minimal SettingsView for the GetSettings/ReadSettingsRadio stubs. */
const SETTINGS_VIEW = {
	HasSnapshot: true,
	Descriptor: 'synthetic-v1',
	Complete: true,
	HasLegacy: false,
	Entries: [{ ID: '990101', Value: 'ON', State: 'known' }],
}

/** One consent-eligible radio's state, as GetUnverifiedWriteConsent
 * returns it (task 14, M9d). The Warning is the BACKEND's text — these
 * tests only ever pass it through, never re-word it. */
const CONSENT_VIEW = {
	Model: 'FTdx10',
	NeedsConsent: true,
	Granted: false,
	Recorded: false,
	Warning: 'This project has never written to a real FTdx10. …',
}

/** ListUnverifiedWriteConsents' shape: every supported model, including
 * the hardware-verified ones (NeedsConsent false), which are LISTED rather
 * than filtered out. */
const CONSENT_ROWS = [
	{ Model: 'FT-710', NeedsConsent: false, Granted: false, Recorded: false, Warning: '' },
	CONSENT_VIEW,
]

/** A real connection to a consent-eligible radio with nothing recorded —
 * the one shape that raises the arming dialogue. */
const NEEDS_CONSENT_INFO = {
	Model: 'FTdx10', CATID: '0761', Port: '/dev/tty.usb', USBSerial: '', Region: '', Demo: false,
	NeedsUnverifiedConsent: true, UnverifiedConsentRecorded: false,
}

beforeEach(() => {
	resetState()
	// Fresh stub surface for every bound method used below.
	window.go = {
		main: {
			App: {
				ListPorts: vi.fn(),
				Connect: vi.fn(),
				ConnectDemo: vi.fn(),
				Disconnect: vi.fn(),
				ReadRadio: vi.fn(),
				ConfirmSend: vi.fn(),
				LoadFile: vi.fn(),
				SaveFileAs: vi.fn(),
				GetUISpec: vi.fn().mockResolvedValue(UI_SPEC),
				UpdateChannel: vi.fn(),
				UpdateChannels: vi.fn(),
				Validate: vi.fn().mockResolvedValue({ Issues: [], Advisory: true }),
				ImportCSV: vi.fn(),
				ImportCHIRP: vi.fn(),
				GetCodeplug: vi.fn(),
				GetSettingsSpec: vi.fn().mockResolvedValue(SETTINGS_SPEC),
				GetSettings: vi.fn().mockResolvedValue(SETTINGS_VIEW),
				ReadSettingsRadio: vi.fn(),
				SaveFile: vi.fn(),
				// Fix 1/Fix 4 (Codex M8b #1/#4): the save + settings-read
				// wrappers read the authoritative post-op dirty state back from
				// Go. Default clean; individual tests override.
				IsDirty: vi.fn().mockResolvedValue(false),
				GetAppVersion: vi.fn().mockResolvedValue(VERSION_VIEW),
				GetSupportedModels: vi.fn().mockResolvedValue(SUPPORTED_MODELS),
				// Task 14 (M9d): the consent surface (task 11's bound methods,
				// bindings regenerated in ced078d).
				GetUnverifiedWriteConsent: vi.fn().mockResolvedValue(CONSENT_VIEW),
				SetUnverifiedWriteConsent: vi.fn().mockResolvedValue(undefined),
				ListUnverifiedWriteConsents: vi.fn().mockResolvedValue(CONSENT_ROWS),
			},
		},
	}
	window.runtime = window.runtime ?? {}
	window.runtime.EventsOnMultiple = vi.fn(() => () => {})
	window.runtime.WindowSetTitle = vi.fn()
})

describe('setWindowTitle (task 18)', () => {
	it('calls the wails runtime WindowSetTitle with the given title', () => {
		bindings.setWindowTitle('Open Rig Programmer — codeplug.json*')
		expect(window.runtime.WindowSetTitle).toHaveBeenCalledWith('Open Rig Programmer — codeplug.json*')
	})
})

describe('initTransferEvents', () => {
	it('subscribes to transfer:progress and transfer:done, fanning out to appState', () => {
		bindings.initTransferEvents()
		bindings.initTransferEvents() // idempotent — must not double-subscribe

		const calls = window.runtime.EventsOnMultiple.mock.calls
		const progressCall = calls.find((c) => c[0] === 'transfer:progress')
		const doneCall = calls.find((c) => c[0] === 'transfer:done')
		expect(progressCall).toBeTruthy()
		expect(doneCall).toBeTruthy()
		// Exactly one of each — the second initTransferEvents() call was a no-op.
		expect(calls.filter((c) => c[0] === 'transfer:progress')).toHaveLength(1)
		expect(calls.filter((c) => c[0] === 'transfer:done')).toHaveLength(1)

		// Fan-out: invoking the captured callbacks updates appState directly.
		progressCall[1]({ Phase: 'write', Done: 3, Total: 9, Slot: 'CH-009' })
		expect(appState.transfer.active).toBe(true)
		expect(appState.transfer.progress).toEqual({
			phase: 'write',
			done: 3,
			total: 9,
			slot: 'CH-009',
			targetKind: '',
			targetId: '',
			targetDisplay: '',
		})

		doneCall[1]({ Kind: 'send', Outcome: 'ok', Report: null, Message: '' })
		expect(appState.transfer.active).toBe(false)
		expect(appState.transfer.lastOutcome).toEqual({ Kind: 'send', Outcome: 'ok', Report: null, Message: '' })
	})
})

describe('listPorts', () => {
	it('calls App.ListPorts and stores the result, toggling portsLoading', async () => {
		const ports = [{ Path: '/dev/tty.usb', Description: 'FTDI', Score: 10, Hints: ['likely'] }]
		window.go.main.App.ListPorts.mockResolvedValue(ports)

		const promise = bindings.listPorts()
		expect(appState.portsLoading).toBe(true)
		const result = await promise

		expect(window.go.main.App.ListPorts).toHaveBeenCalledTimes(1)
		expect(result).toEqual(ports)
		expect(appState.ports).toEqual(ports)
		expect(appState.portsLoading).toBe(false)
	})

	it('pushes an alert and rethrows on rejection, still clearing portsLoading', async () => {
		window.go.main.App.ListPorts.mockRejectedValue('scan failed')

		await expect(bindings.listPorts()).rejects.toBe('scan failed')
		expect(appState.portsLoading).toBe(false)
		expect(appState.alerts).toHaveLength(1)
		expect(appState.alerts[0].message).toContain('listing ports')
		expect(appState.alerts[0].message).toContain('scan failed')
	})
})

describe('connect / connectDemo / disconnect', () => {
	it('connect calls App.Connect(portPath, model) and stores the ConnectionInfo', async () => {
		const info = { Model: 'FT-710', CATID: '0800', Port: '/dev/tty.usb', USBSerial: '', Region: '', Demo: false }
		window.go.main.App.Connect.mockResolvedValue(info)

		await bindings.connect('/dev/tty.usb')

		// M9c-5 (E4): the model is passed EXPLICITLY, and '' means the
		// default model — no compat wrapper hides the new parameter.
		expect(window.go.main.App.Connect).toHaveBeenCalledWith('/dev/tty.usb', '')
		expect(appState.connection).toEqual(info)
		expect(appState.connecting).toBe(false)
	})

	it("connectDemo calls App.ConnectDemo('') — the default model", async () => {
		const info = { Model: 'FT-710', CATID: '0800', Port: 'fake', USBSerial: 'SIM0001', Region: '', Demo: true }
		window.go.main.App.ConnectDemo.mockResolvedValue(info)

		await bindings.connectDemo()

		expect(window.go.main.App.ConnectDemo).toHaveBeenCalledWith('')
		expect(appState.connection?.Demo).toBe(true)
	})

	// Task 13 (M9d): the model picker landed, so these two call sites now
	// forward appState.selectedModel. An untouched picker is '' — pinned by
	// the two tests above, which are exactly today's behaviour.
	it('connect forwards the picked model from appState.selectedModel', async () => {
		appState.setSelectedModel('FTdx10')
		window.go.main.App.Connect.mockResolvedValue({ Model: 'FTdx10', CATID: '0761', Port: '/dev/tty.usb', USBSerial: '', Region: '', Demo: false })

		await bindings.connect('/dev/tty.usb')

		expect(window.go.main.App.Connect).toHaveBeenCalledWith('/dev/tty.usb', 'FTdx10')
	})

	it('connectDemo forwards the picked model too — the demo path opens that model\'s own simulator', async () => {
		appState.setSelectedModel('FTDX101MP')
		window.go.main.App.ConnectDemo.mockResolvedValue({ Model: 'FTDX101MP', CATID: '0681', Port: 'fake', USBSerial: 'SIM0001', Region: '', Demo: true })

		await bindings.connectDemo()

		expect(window.go.main.App.ConnectDemo).toHaveBeenCalledWith('FTDX101MP')
	})

	it('disconnect clears the connection on success', async () => {
		appState.setConnection({ Model: 'FT-710', CATID: '0800', Port: '/dev/tty.usb', USBSerial: '', Region: '', Demo: false })
		window.go.main.App.Disconnect.mockResolvedValue(undefined)

		await bindings.disconnect()

		expect(appState.connection).toBeNull()
	})

	it('disconnect (Fix 1, Codex M6 #1, adjudicated HIGH) preserves the loaded codeplug, dirty flag and workingPath — a disconnect must never strand unsaved work', async () => {
		appState.setConnection({ Model: 'FT-710', CATID: '0800', Port: '/dev/tty.usb', USBSerial: '', Region: '', Demo: false })
		appState.setCodeplug({
			Schema: 1, Generator: 'x', Radio: {},
			Channels: [{ slot: '001', data: { freq_hz: 7100000 } }],
			WorkingPath: '/tmp/edited.json', Dirty: false, BaselineStale: false,
		})
		// An edit made while connected, mirrored into appState the way
		// updateChannel's own wrapper does — Go's working copy is dirty.
		appState.setDirty(true)
		window.go.main.App.Disconnect.mockResolvedValue(undefined)

		await bindings.disconnect()

		expect(appState.connection).toBeNull()
		expect(appState.codeplug).not.toBeNull()
		expect(appState.codeplug.WorkingPath).toBe('/tmp/edited.json')
		expect(appState.codeplug.Channels[0].data.freq_hz).toBe(7100000)
		expect(appState.dirty).toBe(true)
	})

	it('disconnect leaves state untouched and alerts on rejection', async () => {
		appState.setConnection({ Model: 'FT-710', CATID: '0800', Port: '/dev/tty.usb', USBSerial: '', Region: '', Demo: false })
		window.go.main.App.Disconnect.mockRejectedValue(new Error('busy'))

		await expect(bindings.disconnect()).rejects.toThrow('busy')
		expect(appState.connection).not.toBeNull()
		expect(appState.alerts[0].message).toBe('disconnecting: busy')
	})
})

describe('readRadio', () => {
	it('marks the transfer active immediately, then settles it once the call resolves', async () => {
		const view = { Schema: 1, Generator: 'x', Radio: {}, Channels: [], WorkingPath: '', Dirty: false, BaselineStale: false }
		let resolveCall
		window.go.main.App.ReadRadio.mockReturnValue(
			new Promise((resolve) => {
				resolveCall = resolve
			})
		)

		const promise = bindings.readRadio()
		expect(appState.transfer.active).toBe(true)
		expect(appState.transfer.kind).toBe('read')

		resolveCall(view)
		await promise

		expect(appState.codeplug).toEqual(view)
		expect(appState.transfer.active).toBe(false)
	})

	it('still clears active and alerts when App.ReadRadio rejects', async () => {
		window.go.main.App.ReadRadio.mockRejectedValue('app: not connected')

		await expect(bindings.readRadio()).rejects.toBe('app: not connected')
		expect(appState.transfer.active).toBe(false)
		expect(appState.alerts[0].message).toContain('reading radio')
	})
})

describe('confirmSend — the asymmetric active-clearing case', () => {
	it('does NOT clear active on a successful call — only transfer:done does that', async () => {
		window.go.main.App.ConfirmSend.mockResolvedValue(undefined)

		await bindings.confirmSend('digest-abc', 'v1.23')

		expect(window.go.main.App.ConfirmSend).toHaveBeenCalledWith('digest-abc', 'v1.23')
		expect(appState.transfer.kind).toBe('send')
		// The transfer is still "running" from the frontend's point of view —
		// ConfirmSend just started a background goroutine.
		expect(appState.transfer.active).toBe(true)

		// Only the eventual transfer:done event ends it.
		appState.applyTransferDone({ Kind: 'send', Outcome: 'ok', Report: null, Message: '' })
		expect(appState.transfer.active).toBe(false)
	})

	it('DOES clear active on a synchronous pre-flight rejection (nothing was started)', async () => {
		window.go.main.App.ConfirmSend.mockRejectedValue('app: no active plan')

		await expect(bindings.confirmSend('digest-abc', 'v1.23')).rejects.toBe('app: no active plan')
		expect(appState.transfer.active).toBe(false)
		expect(appState.alerts[0].message).toContain('sending to radio')
	})
})

describe('loadFile — cancelled-dialog handling', () => {
	it('returns null and leaves appState.codeplug untouched on the documented zero-value cancel', async () => {
		appState.setCodeplug({ Schema: 1, Generator: 'x', Radio: {}, Channels: [{ slot: '001' }], WorkingPath: '/tmp/a.json', Dirty: false, BaselineStale: false })
		window.go.main.App.LoadFile.mockResolvedValue({ Schema: 0, Generator: '', Radio: {}, Channels: null, WorkingPath: '', Dirty: false, BaselineStale: false })

		const result = await bindings.loadFile()

		expect(result).toBeNull()
		expect(appState.codeplug?.WorkingPath).toBe('/tmp/a.json')
	})

	it('stores the codeplug on a real load', async () => {
		const view = { Schema: 1, Generator: 'x', Radio: {}, Channels: [{ slot: '001' }], WorkingPath: '/tmp/b.json', Dirty: false, BaselineStale: false }
		window.go.main.App.LoadFile.mockResolvedValue(view)

		const result = await bindings.loadFile()

		expect(result).toEqual(view)
		expect(appState.codeplug).toEqual(view)
	})
})

describe('refreshUISpec (task 17)', () => {
	it('stores the spec in appState and returns it', async () => {
		const spec = await bindings.refreshUISpec()
		expect(window.go.main.App.GetUISpec).toHaveBeenCalledTimes(1)
		expect(spec).toEqual(UI_SPEC)
		expect(appState.uiSpec).toEqual(UI_SPEC)
	})

	it('alerts but does NOT throw on rejection — a spec refresh must never break its caller', async () => {
		window.go.main.App.GetUISpec.mockRejectedValue('boom')
		const spec = await bindings.refreshUISpec()
		expect(spec).toBeNull()
		expect(appState.uiSpec).toBeNull()
		expect(appState.alerts[0].message).toContain('loading the grid layout')
	})
})

describe('refreshAppVersion (v1.0.0 release tail)', () => {
	it('stores the version in appState and returns it, unmodified', async () => {
		const version = await bindings.refreshAppVersion()
		expect(window.go.main.App.GetAppVersion).toHaveBeenCalledTimes(1)
		expect(version).toEqual(VERSION_VIEW)
		expect(appState.appVersion).toEqual(VERSION_VIEW)
	})

	it('alerts but does NOT throw on rejection — failing to learn the version must not stop the app opening', async () => {
		window.go.main.App.GetAppVersion.mockRejectedValue('boom')
		const version = await bindings.refreshAppVersion()
		expect(version).toBeNull()
		expect(appState.appVersion).toBeNull()
		expect(appState.alerts[0].message).toContain('reading the app version')
	})

	it('is not refreshed by connect or disconnect — the version cannot change while the app runs', async () => {
		await bindings.refreshAppVersion()
		expect(window.go.main.App.GetAppVersion).toHaveBeenCalledTimes(1)

		window.go.main.App.Connect.mockResolvedValue({ Model: 'FT-710', CATID: '0800', Port: '/dev/tty.usb', USBSerial: '', Region: '', Demo: false })
		await bindings.connect('/dev/tty.usb')
		window.go.main.App.Disconnect.mockResolvedValue(undefined)
		await bindings.disconnect()

		expect(window.go.main.App.GetAppVersion).toHaveBeenCalledTimes(1)
		expect(appState.appVersion).toEqual(VERSION_VIEW)
	})
})

describe('refreshSupportedModels (task 13, M9d — the model picker\'s list)', () => {
	it('stores GetSupportedModels\' list in appState and returns it, in Go\'s own order', async () => {
		const models = await bindings.refreshSupportedModels()
		expect(window.go.main.App.GetSupportedModels).toHaveBeenCalledTimes(1)
		expect(models).toEqual(SUPPORTED_MODELS)
		expect(appState.supportedModels).toEqual(SUPPORTED_MODELS)
	})

	it('alerts but does NOT throw on rejection — a picker with no list still connects as the default model', async () => {
		window.go.main.App.GetSupportedModels.mockRejectedValue('boom')
		const models = await bindings.refreshSupportedModels()
		expect(models).toBeNull()
		expect(appState.supportedModels).toEqual([])
		expect(appState.alerts[0].message).toContain('listing supported radios')
	})
})

describe('UISpec refresh triggers (task 17: banks change with the session/working copy)', () => {
	it('connect refreshes the spec after storing the connection', async () => {
		window.go.main.App.Connect.mockResolvedValue({ Model: 'FT-710', CATID: '0800', Port: '/dev/tty.usb', USBSerial: '', Region: '', Demo: false })
		await bindings.connect('/dev/tty.usb')
		expect(window.go.main.App.GetUISpec).toHaveBeenCalledTimes(1)
		expect(appState.uiSpec).toEqual(UI_SPEC)
	})

	it('connectDemo refreshes the spec', async () => {
		window.go.main.App.ConnectDemo.mockResolvedValue({ Model: 'FT-710', CATID: '0800', Port: 'fake', USBSerial: 'SIM0001', Region: '', Demo: true })
		await bindings.connectDemo()
		expect(window.go.main.App.GetUISpec).toHaveBeenCalledTimes(1)
	})

	it('disconnect refreshes the spec (back to the offline baseline)', async () => {
		appState.setConnection({ Model: 'FT-710', CATID: '0800', Port: '/dev/tty.usb', USBSerial: '', Region: '', Demo: false })
		window.go.main.App.Disconnect.mockResolvedValue(undefined)
		await bindings.disconnect()
		expect(window.go.main.App.GetUISpec).toHaveBeenCalledTimes(1)
	})

	it('a failed connect does NOT refresh the spec', async () => {
		window.go.main.App.Connect.mockRejectedValue('no such port')
		await expect(bindings.connect('/dev/bad')).rejects.toBe('no such port')
		expect(window.go.main.App.GetUISpec).not.toHaveBeenCalled()
	})

	it('readRadio refreshes the spec after storing the codeplug', async () => {
		const view = { Schema: 1, Generator: 'x', Radio: {}, Channels: [{ slot: '001' }], WorkingPath: '', Dirty: false, BaselineStale: false }
		window.go.main.App.ReadRadio.mockResolvedValue(view)
		await bindings.readRadio()
		expect(window.go.main.App.GetUISpec).toHaveBeenCalledTimes(1)
	})

	it('loadFile refreshes the spec on a real load but not on a cancelled dialog', async () => {
		window.go.main.App.LoadFile.mockResolvedValue({ Schema: 0, Generator: '', Radio: {}, Channels: null, WorkingPath: '', Dirty: false, BaselineStale: false })
		await bindings.loadFile()
		expect(window.go.main.App.GetUISpec).not.toHaveBeenCalled()

		window.go.main.App.LoadFile.mockResolvedValue({ Schema: 1, Generator: 'x', Radio: {}, Channels: [{ slot: '001' }], WorkingPath: '/tmp/b.json', Dirty: false, BaselineStale: false })
		await bindings.loadFile()
		expect(window.go.main.App.GetUISpec).toHaveBeenCalledTimes(1)
	})
})

describe('validation lifecycle (Fix 5, adjudicated MED, Codex M6 #5)', () => {
	it('connect re-runs Validate and stores connected-authoritative issues (closes elevated minor b)', async () => {
		// A file was loaded offline (advisory) BEFORE connecting — the
		// realistic scenario: the user opens a codeplug, sees advisory
		// issues, then connects to a live radio.
		appState.setCodeplug({ Schema: 1, Generator: 'x', Radio: {}, Channels: [{ slot: '001', data: null }], WorkingPath: '/tmp/a.json', Dirty: false, BaselineStale: false })
		appState.setIssues([])
		appState.setIssuesAdvisory(true)

		window.go.main.App.Connect.mockResolvedValue({ Model: 'FT-710', CATID: '0800', Port: '/dev/tty.usb', USBSerial: '', Region: '', Demo: false })
		const authoritativeIssues = [{ Slot: '501', Field: '', Severity: 'error', Msg: 'not part of any bank this radio supports' }]
		window.go.main.App.Validate.mockResolvedValue({ Issues: authoritativeIssues, Advisory: false })

		await bindings.connect('/dev/tty.usb')

		expect(window.go.main.App.Validate).toHaveBeenCalledTimes(1)
		expect(appState.issues).toEqual(authoritativeIssues)
		expect(appState.issuesAdvisory).toBe(false)
	})

	it('connect does NOT call Validate when nothing is loaded yet (no spurious ErrNothingLoaded alert)', async () => {
		window.go.main.App.Connect.mockResolvedValue({ Model: 'FT-710', CATID: '0800', Port: '/dev/tty.usb', USBSerial: '', Region: '', Demo: false })
		await bindings.connect('/dev/tty.usb')
		expect(window.go.main.App.Validate).not.toHaveBeenCalled()
		expect(appState.alerts).toHaveLength(0)
	})

	it('disconnect re-runs Validate, reverting to advisory', async () => {
		appState.setConnection({ Model: 'FT-710', CATID: '0800', Port: '/dev/tty.usb', USBSerial: '', Region: '', Demo: false })
		appState.setCodeplug({ Schema: 1, Generator: 'x', Radio: {}, Channels: [{ slot: '001', data: null }], WorkingPath: '/tmp/a.json', Dirty: false, BaselineStale: false })
		appState.setIssuesAdvisory(false)
		window.go.main.App.Disconnect.mockResolvedValue(undefined)
		const advisoryIssues = [{ Slot: '001', Field: 'freq', Severity: 'warning', Msg: 'advisory only' }]
		window.go.main.App.Validate.mockResolvedValue({ Issues: advisoryIssues, Advisory: true })

		await bindings.disconnect()

		expect(window.go.main.App.Validate).toHaveBeenCalledTimes(1)
		expect(appState.issues).toEqual(advisoryIssues)
		expect(appState.issuesAdvisory).toBe(true)
	})

	it('loadFile shows issues immediately (loaded-invalid-file case)', async () => {
		const view = { Schema: 1, Generator: 'x', Radio: {}, Channels: [{ slot: '001', data: null }], WorkingPath: '/tmp/bad.json', Dirty: false, BaselineStale: false }
		window.go.main.App.LoadFile.mockResolvedValue(view)
		const invalidFileIssues = [{ Slot: '001', Field: 'freq', Severity: 'error', Msg: 'frequency out of range' }]
		window.go.main.App.Validate.mockResolvedValue({ Issues: invalidFileIssues, Advisory: true })

		await bindings.loadFile()

		expect(window.go.main.App.Validate).toHaveBeenCalledTimes(1)
		expect(appState.issues).toEqual(invalidFileIssues)
		expect(appState.issuesAdvisory).toBe(true)
	})

	it('readRadio re-runs Validate', async () => {
		const view = { Schema: 1, Generator: 'x', Radio: {}, Channels: [{ slot: '001', data: null }], WorkingPath: '', Dirty: false, BaselineStale: false }
		window.go.main.App.ReadRadio.mockResolvedValue(view)
		const issues = [{ Slot: '001', Field: 'freq', Severity: 'error', Msg: 'bad' }]
		window.go.main.App.Validate.mockResolvedValue({ Issues: issues, Advisory: false })

		await bindings.readRadio()

		expect(window.go.main.App.Validate).toHaveBeenCalledTimes(1)
		expect(appState.issues).toEqual(issues)
		expect(appState.issuesAdvisory).toBe(false)
	})

	it('a successful import re-runs Validate to refresh issues+advisory together', async () => {
		appState.setCodeplug({ Schema: 1, Generator: 'x', Radio: {}, Channels: [{ slot: '001', data: null }], WorkingPath: '', Dirty: false, BaselineStale: false })
		window.go.main.App.ImportCSV.mockResolvedValue({ Path: '/tmp/in.csv', Merged: true, Issues: [], Dirty: true })
		window.go.main.App.GetCodeplug.mockResolvedValue({ Schema: 1, Generator: 'x', Radio: {}, Channels: [{ slot: '001', data: null }], WorkingPath: '', Dirty: true, BaselineStale: false })
		const issues = [{ Slot: '001', Field: 'freq', Severity: 'warning', Msg: 'hmm' }]
		window.go.main.App.Validate.mockResolvedValue({ Issues: issues, Advisory: true })

		await bindings.importCSV()

		expect(window.go.main.App.Validate).toHaveBeenCalledTimes(1)
		expect(appState.issues).toEqual(issues)
	})
})

describe('view sync after import/Save As (Fix 4, adjudicated MED, Codex M6 #4)', () => {
	it('a successful CSV import refreshes appState.codeplug.Channels with the merged result (call GetCodeplug after merge)', async () => {
		appState.setCodeplug({ Schema: 1, Generator: 'x', Radio: {}, Channels: [{ slot: '001', data: { freq_hz: 7000000, mode: 'LSB' } }], WorkingPath: '', Dirty: false, BaselineStale: false })
		window.go.main.App.ImportCSV.mockResolvedValue({ Path: '/tmp/in.csv', Merged: true, Issues: [], Dirty: true })
		const mergedView = {
			Schema: 1, Generator: 'x', Radio: {},
			Channels: [{ slot: '001', data: { freq_hz: 7100000, mode: 'USB' } }],
			WorkingPath: '', Dirty: true, BaselineStale: false,
		}
		window.go.main.App.GetCodeplug.mockResolvedValue(mergedView)

		await bindings.importCSV()

		expect(window.go.main.App.GetCodeplug).toHaveBeenCalledTimes(1)
		expect(appState.codeplug.Channels[0].data.freq_hz).toBe(7100000)
		expect(appState.dirty).toBe(true)
	})

	it('a successful CHIRP import refreshes appState.codeplug.Channels too', async () => {
		appState.setCodeplug({ Schema: 1, Generator: 'x', Radio: {}, Channels: [{ slot: '001', data: null }], WorkingPath: '', Dirty: false, BaselineStale: false })
		window.go.main.App.ImportCHIRP.mockResolvedValue({ Path: '/tmp/in.csv', Merged: true, Issues: [], LossEntries: [], Dirty: true })
		const mergedView = {
			Schema: 1, Generator: 'x', Radio: {},
			Channels: [{ slot: '001', data: { freq_hz: 14250000, mode: 'USB' } }],
			WorkingPath: '', Dirty: true, BaselineStale: false,
		}
		window.go.main.App.GetCodeplug.mockResolvedValue(mergedView)

		await bindings.importCHIRP()

		expect(window.go.main.App.GetCodeplug).toHaveBeenCalledTimes(1)
		expect(appState.codeplug.Channels[0].data.freq_hz).toBe(14250000)
	})

	it('a cancelled/refused import does NOT call GetCodeplug (nothing changed)', async () => {
		appState.setCodeplug({ Schema: 1, Generator: 'x', Radio: {}, Channels: [{ slot: '001', data: null }], WorkingPath: '', Dirty: false, BaselineStale: false })
		window.go.main.App.ImportCSV.mockResolvedValue({ Path: '/tmp/in.csv', Merged: false, RefusalReason: 'inventory mismatch' })

		await bindings.importCSV()

		expect(window.go.main.App.GetCodeplug).not.toHaveBeenCalled()
	})

	it('Save As updates appState.codeplug.WorkingPath so the title bar reflects the new filename', async () => {
		appState.setCodeplug({ Schema: 1, Generator: 'x', Radio: {}, Channels: [], WorkingPath: '', Dirty: true, BaselineStale: false })
		window.go.main.App.SaveFileAs.mockResolvedValue('/tmp/renamed.json')

		const path = await bindings.saveFileAs()

		expect(path).toBe('/tmp/renamed.json')
		expect(appState.codeplug.WorkingPath).toBe('/tmp/renamed.json')
		expect(appState.dirty).toBe(false)
	})

	it('Save As leaves WorkingPath untouched when the user cancels the dialog', async () => {
		appState.setCodeplug({ Schema: 1, Generator: 'x', Radio: {}, Channels: [], WorkingPath: '/tmp/original.json', Dirty: true, BaselineStale: false })
		window.go.main.App.SaveFileAs.mockResolvedValue('')

		await bindings.saveFileAs()

		expect(appState.codeplug.WorkingPath).toBe('/tmp/original.json')
	})
})

describe('updateChannel / updateChannels (task 17)', () => {
	const channel = { slot: '001', data: { freq_hz: 7100000, mode: 'USB' } }

	it('updateChannel stores Issues/Dirty and mirrors the edit into the codeplug view', async () => {
		appState.setCodeplug({ Schema: 1, Generator: 'x', Radio: {}, Channels: [{ slot: '001', data: { freq_hz: 7074000, mode: 'LSB' } }], WorkingPath: '', Dirty: false, BaselineStale: false })
		const result = { Issues: [{ Slot: '001', Field: 'frequency', Severity: 'warning', Msg: 'hmm' }], Dirty: true }
		window.go.main.App.UpdateChannel.mockResolvedValue(result)

		const got = await bindings.updateChannel(channel)

		expect(window.go.main.App.UpdateChannel).toHaveBeenCalledWith(channel)
		expect(got).toEqual(result)
		expect(appState.issues).toEqual(result.Issues)
		expect(appState.dirty).toBe(true)
		expect(appState.codeplug.Channels[0].data.freq_hz).toBe(7100000)
	})

	it('updateChannels mirrors the whole batch', async () => {
		appState.setCodeplug({
			Schema: 1, Generator: 'x', Radio: {},
			Channels: [{ slot: '001', data: null }, { slot: '002', data: null }],
			WorkingPath: '', Dirty: false, BaselineStale: false,
		})
		window.go.main.App.UpdateChannels.mockResolvedValue({ Issues: [], Dirty: true })
		const batch = [
			{ slot: '001', data: { freq_hz: 7000000, mode: 'LSB' } },
			{ slot: '002', data: { freq_hz: 7100000, mode: 'USB' } },
		]

		await bindings.updateChannels(batch)

		expect(window.go.main.App.UpdateChannels).toHaveBeenCalledWith(batch)
		expect(appState.codeplug.Channels[0].data.freq_hz).toBe(7000000)
		expect(appState.codeplug.Channels[1].data.freq_hz).toBe(7100000)
	})

	it('a rejected updateChannel alerts, rethrows and leaves the codeplug untouched', async () => {
		appState.setCodeplug({ Schema: 1, Generator: 'x', Radio: {}, Channels: [{ slot: '001', data: null }], WorkingPath: '', Dirty: false, BaselineStale: false })
		window.go.main.App.UpdateChannel.mockRejectedValue('app: unknown slot')

		await expect(bindings.updateChannel(channel)).rejects.toBe('app: unknown slot')
		expect(appState.codeplug.Channels[0].data).toBeNull()
		expect(appState.alerts[0].message).toContain('updating channel')
	})
})

describe('saveFileAs — cancelled-dialog handling', () => {
	it('does not clear dirty when the user cancels (empty path)', async () => {
		appState.setDirty(true)
		window.go.main.App.SaveFileAs.mockResolvedValue('')

		const path = await bindings.saveFileAs()

		expect(path).toBe('')
		expect(appState.dirty).toBe(true)
	})

	it('clears dirty once a real path comes back', async () => {
		appState.setDirty(true)
		window.go.main.App.SaveFileAs.mockResolvedValue('/tmp/c.json')

		const path = await bindings.saveFileAs()

		expect(path).toBe('/tmp/c.json')
		expect(appState.dirty).toBe(false)
	})
})

describe('refreshSettingsSpec (task 36, M8b-6)', () => {
	it('stores the spec in appState and returns it', async () => {
		const spec = await bindings.refreshSettingsSpec()
		expect(window.go.main.App.GetSettingsSpec).toHaveBeenCalledTimes(1)
		expect(spec).toEqual(SETTINGS_SPEC)
		expect(appState.settingsSpec).toEqual(SETTINGS_SPEC)
	})

	it('alerts but does NOT throw on rejection — a spec refresh must never break its caller', async () => {
		window.go.main.App.GetSettingsSpec.mockRejectedValue('boom')
		const spec = await bindings.refreshSettingsSpec()
		expect(spec).toBeNull()
		expect(appState.settingsSpec).toBeNull()
		expect(appState.alerts[0].message).toContain('loading the settings layout')
	})
})

describe('settingsSpec refresh triggers (task 36: Live flips with the connection, connect/connectDemo/disconnect ONLY — unlike uiSpec, readRadio/loadFile/importers do NOT refresh it)', () => {
	it('connect refreshes the settings spec after storing the connection', async () => {
		window.go.main.App.Connect.mockResolvedValue({ Model: 'FT-710', CATID: '0800', Port: '/dev/tty.usb', USBSerial: '', Region: '', Demo: false })
		await bindings.connect('/dev/tty.usb')
		expect(window.go.main.App.GetSettingsSpec).toHaveBeenCalledTimes(1)
		expect(appState.settingsSpec).toEqual(SETTINGS_SPEC)
	})

	it('connectDemo refreshes the settings spec', async () => {
		window.go.main.App.ConnectDemo.mockResolvedValue({ Model: 'FT-710', CATID: '0800', Port: 'fake', USBSerial: 'SIM0001', Region: '', Demo: true })
		await bindings.connectDemo()
		expect(window.go.main.App.GetSettingsSpec).toHaveBeenCalledTimes(1)
	})

	it('disconnect refreshes the settings spec (back to the offline baseline)', async () => {
		appState.setConnection({ Model: 'FT-710', CATID: '0800', Port: '/dev/tty.usb', USBSerial: '', Region: '', Demo: false })
		window.go.main.App.Disconnect.mockResolvedValue(undefined)
		await bindings.disconnect()
		expect(window.go.main.App.GetSettingsSpec).toHaveBeenCalledTimes(1)
	})

	it('a failed connect does NOT refresh the settings spec', async () => {
		window.go.main.App.Connect.mockRejectedValue('no such port')
		await expect(bindings.connect('/dev/bad')).rejects.toBe('no such port')
		expect(window.go.main.App.GetSettingsSpec).not.toHaveBeenCalled()
	})

	it('readRadio does NOT refresh the settings spec (only its content)', async () => {
		const view = { Schema: 1, Generator: 'x', Radio: {}, Channels: [{ slot: '001' }], WorkingPath: '', Dirty: false, BaselineStale: false }
		window.go.main.App.ReadRadio.mockResolvedValue(view)
		await bindings.readRadio()
		expect(window.go.main.App.GetSettingsSpec).not.toHaveBeenCalled()
	})
})

describe('getSettings (task 36, M8b-6)', () => {
	it('stores the settings view in appState and returns it', async () => {
		const view = await bindings.getSettings()
		expect(window.go.main.App.GetSettings).toHaveBeenCalledTimes(1)
		expect(view).toEqual(SETTINGS_VIEW)
		expect(appState.settings).toEqual(SETTINGS_VIEW)
	})

	it('alerts and rethrows on rejection (ordinary throw-and-report shape, unlike refreshSettingsSpec)', async () => {
		window.go.main.App.GetSettings.mockRejectedValue('app: nothing loaded')
		await expect(bindings.getSettings()).rejects.toBe('app: nothing loaded')
		expect(appState.alerts[0].message).toContain('loading settings')
	})
})

describe('settings-content refresh triggers (task 36: readRadio/loadFile/a successful import replace the working copy, so each refreshes appState.settings too — none of their own results carries it)', () => {
	it('readRadio refreshes settings content after storing the codeplug', async () => {
		const view = { Schema: 1, Generator: 'x', Radio: {}, Channels: [{ slot: '001' }], WorkingPath: '', Dirty: false, BaselineStale: false }
		window.go.main.App.ReadRadio.mockResolvedValue(view)
		await bindings.readRadio()
		expect(window.go.main.App.GetSettings).toHaveBeenCalledTimes(1)
		expect(appState.settings).toEqual(SETTINGS_VIEW)
	})

	it('loadFile refreshes settings content on a real load but not on a cancelled dialog', async () => {
		window.go.main.App.LoadFile.mockResolvedValue({ Schema: 0, Generator: '', Radio: {}, Channels: null, WorkingPath: '', Dirty: false, BaselineStale: false })
		await bindings.loadFile()
		expect(window.go.main.App.GetSettings).not.toHaveBeenCalled()

		window.go.main.App.LoadFile.mockResolvedValue({ Schema: 1, Generator: 'x', Radio: {}, Channels: [{ slot: '001' }], WorkingPath: '/tmp/b.json', Dirty: false, BaselineStale: false })
		await bindings.loadFile()
		expect(window.go.main.App.GetSettings).toHaveBeenCalledTimes(1)
	})

	it('a merged CSV import refreshes settings content', async () => {
		appState.setCodeplug({ Schema: 1, Generator: 'x', Radio: {}, Channels: [{ slot: '001', data: null }], WorkingPath: '', Dirty: false, BaselineStale: false })
		window.go.main.App.ImportCSV.mockResolvedValue({ Path: '/tmp/in.csv', Merged: true, Issues: [], Dirty: true })
		window.go.main.App.GetCodeplug.mockResolvedValue({ Schema: 1, Generator: 'x', Radio: {}, Channels: [{ slot: '001', data: null }], WorkingPath: '', Dirty: true, BaselineStale: false })

		await bindings.importCSV()

		expect(window.go.main.App.GetSettings).toHaveBeenCalledTimes(1)
	})

	it('a merged CHIRP import refreshes settings content', async () => {
		appState.setCodeplug({ Schema: 1, Generator: 'x', Radio: {}, Channels: [{ slot: '001', data: null }], WorkingPath: '', Dirty: false, BaselineStale: false })
		window.go.main.App.ImportCHIRP.mockResolvedValue({ Path: '/tmp/in.csv', Merged: true, Issues: [], LossEntries: [], Dirty: true })
		window.go.main.App.GetCodeplug.mockResolvedValue({ Schema: 1, Generator: 'x', Radio: {}, Channels: [{ slot: '001', data: null }], WorkingPath: '', Dirty: true, BaselineStale: false })

		await bindings.importCHIRP()

		expect(window.go.main.App.GetSettings).toHaveBeenCalledTimes(1)
	})

	it('a cancelled/refused import does NOT refresh settings content', async () => {
		appState.setCodeplug({ Schema: 1, Generator: 'x', Radio: {}, Channels: [{ slot: '001', data: null }], WorkingPath: '', Dirty: false, BaselineStale: false })
		window.go.main.App.ImportCSV.mockResolvedValue({ Path: '/tmp/in.csv', Merged: false, RefusalReason: 'inventory mismatch' })

		await bindings.importCSV()

		expect(window.go.main.App.GetSettings).not.toHaveBeenCalled()
	})

	it('a failed settings-content refresh alerts but does not turn a succeeded readRadio into a rejection (quiet, like refreshUISpec)', async () => {
		const view = { Schema: 1, Generator: 'x', Radio: {}, Channels: [{ slot: '001' }], WorkingPath: '', Dirty: false, BaselineStale: false }
		window.go.main.App.ReadRadio.mockResolvedValue(view)
		window.go.main.App.GetSettings.mockRejectedValue('boom')

		const result = await bindings.readRadio()

		expect(result).toEqual(view)
		expect(appState.settings).toBeNull()
		expect(appState.alerts.some((a) => a.message.includes('loading settings'))).toBe(true)
	})
})

describe('readSettingsRadio (task 36, M8b-6)', () => {
	it('marks the transfer active with kind "settings", stores the returned view, then settles', async () => {
		let resolveCall
		window.go.main.App.ReadSettingsRadio.mockReturnValue(
			new Promise((resolve) => {
				resolveCall = resolve
			})
		)

		const promise = bindings.readSettingsRadio()
		expect(appState.transfer.active).toBe(true)
		expect(appState.transfer.kind).toBe('settings')

		resolveCall(SETTINGS_VIEW)
		await promise

		expect(appState.settings).toEqual(SETTINGS_VIEW)
		expect(appState.transfer.active).toBe(false)
	})

	it('still clears active and alerts when App.ReadSettingsRadio rejects', async () => {
		window.go.main.App.ReadSettingsRadio.mockRejectedValue('app: not connected')

		await expect(bindings.readSettingsRadio()).rejects.toBe('app: not connected')
		expect(appState.transfer.active).toBe(false)
		expect(appState.alerts[0].message).toContain('reading settings')
	})

	it('syncs dirty from Go (IsDirty) so the unsaved-changes guard activates on a clean codeplug (Fix 1, Codex M8b #1)', async () => {
		// The exact regression: a clean codeplug is open, a settings read
		// merges new data (Go marks the working copy dirty). Without this
		// sync the ActionBar Open/Read guard would see dirty=false and could
		// silently discard the freshly-read settings.
		appState.setDirty(false)
		window.go.main.App.ReadSettingsRadio.mockResolvedValue(SETTINGS_VIEW)
		window.go.main.App.IsDirty.mockResolvedValue(true)

		await bindings.readSettingsRadio()

		expect(window.go.main.App.IsDirty).toHaveBeenCalledTimes(1)
		expect(appState.dirty).toBe(true)
	})
})

describe('save wrappers honour Go\'s true post-save dirty (Fix 4, Codex M8b #4)', () => {
	it('saveFile leaves dirty TRUE when a mutation landed mid-save (IsDirty reports dirty)', async () => {
		appState.setDirty(true)
		window.go.main.App.SaveFile.mockResolvedValue(undefined)
		window.go.main.App.IsDirty.mockResolvedValue(true)

		await bindings.saveFile('/tmp/x.json')

		expect(window.go.main.App.IsDirty).toHaveBeenCalledTimes(1)
		expect(appState.dirty).toBe(true) // NOT forced false — honours Go's real state
	})

	it('saveFile clears dirty when Go reports clean (the ordinary, unraced case)', async () => {
		appState.setDirty(true)
		window.go.main.App.SaveFile.mockResolvedValue(undefined)
		window.go.main.App.IsDirty.mockResolvedValue(false)

		await bindings.saveFile('/tmp/x.json')

		expect(appState.dirty).toBe(false)
	})

	it('saveFileAs leaves dirty TRUE when Go still reports dirty after the save', async () => {
		appState.setCodeplug({ Schema: 1, Generator: 'x', Radio: {}, Channels: [], WorkingPath: '', Dirty: true, BaselineStale: false })
		appState.setDirty(true)
		window.go.main.App.SaveFileAs.mockResolvedValue('/tmp/renamed.json')
		window.go.main.App.IsDirty.mockResolvedValue(true)

		const path = await bindings.saveFileAs()

		expect(path).toBe('/tmp/renamed.json')
		expect(appState.codeplug.WorkingPath).toBe('/tmp/renamed.json')
		expect(appState.dirty).toBe(true)
	})
})

// --- Task 14 (M9d): the unverified-write consent surface -----------------
//
// The orchestration under test lives HERE, in the bridge, not in a
// component and not in app.svelte.js (which owns data only): a consent
// change that affects the CONNECTED model must Disconnect FIRST, persist
// only once that succeeded, and re-open the session on the same port and
// the same model. These tests pin that ORDER, and pin that a refused
// disconnect persists nothing at all.

/** The connection the reconnect produces: same radio, same port, and now a
 * recorded decision (so no second arming dialogue is raised). */
const RECONNECTED_INFO = { ...NEEDS_CONSENT_INFO, UnverifiedConsentRecorded: true }

/** A UISpecView whose capabilities carry (or do not carry) the consented
 * write label — the ONLY thing the amber indicator and the
 * "does this change affect the live session?" decision read. */
/** @param {boolean} consented */
function specArmed(consented) {
	return { ...UI_SPEC, UnverifiedWritesConsented: consented }
}

describe('connect — raising the arming dialogue (task 14)', () => {
	it('raises the prompt after a real connect that needs consent with nothing recorded, with the BACKEND-served warning verbatim', async () => {
		window.go.main.App.Connect.mockResolvedValue(NEEDS_CONSENT_INFO)

		await bindings.connect('/dev/tty.usb')

		expect(window.go.main.App.GetUnverifiedWriteConsent).toHaveBeenCalledWith('FTdx10')
		expect(appState.unverifiedConsentPrompt).toEqual(CONSENT_VIEW)
		expect(appState.unverifiedConsentPrompt?.Warning).toBe(CONSENT_VIEW.Warning)
	})

	it('raises nothing when a decision is already recorded — a decline is a decision', async () => {
		window.go.main.App.Connect.mockResolvedValue({ ...NEEDS_CONSENT_INFO, UnverifiedConsentRecorded: true })

		await bindings.connect('/dev/tty.usb')

		expect(window.go.main.App.GetUnverifiedWriteConsent).not.toHaveBeenCalled()
		expect(appState.unverifiedConsentPrompt).toBeNull()
	})

	it('raises nothing for a recorded grant either', async () => {
		window.go.main.App.Connect.mockResolvedValue({ ...NEEDS_CONSENT_INFO, UnverifiedConsentRecorded: true })
		window.go.main.App.GetUISpec.mockResolvedValue(specArmed(true))

		await bindings.connect('/dev/tty.usb')

		expect(appState.unverifiedConsentPrompt).toBeNull()
	})

	it('never raises it on the demo path, even for a model that would need consent on real hardware', async () => {
		window.go.main.App.ConnectDemo.mockResolvedValue({ ...NEEDS_CONSENT_INFO, Demo: true })

		await bindings.connectDemo()

		expect(window.go.main.App.GetUnverifiedWriteConsent).not.toHaveBeenCalled()
		expect(appState.unverifiedConsentPrompt).toBeNull()
	})

	it('a failed warning fetch alerts and leaves no prompt — the connection itself still succeeds', async () => {
		window.go.main.App.Connect.mockResolvedValue(NEEDS_CONSENT_INFO)
		window.go.main.App.GetUnverifiedWriteConsent.mockRejectedValue('settings file unreadable')

		const info = await bindings.connect('/dev/tty.usb')

		expect(info).toEqual(NEEDS_CONSENT_INFO)
		expect(appState.connection).toEqual(NEEDS_CONSENT_INFO)
		expect(appState.unverifiedConsentPrompt).toBeNull()
		expect(appState.alerts.some((a) => a.message.includes('settings file unreadable'))).toBe(true)
	})
})

describe('applyUnverifiedWriteConsent — the bridge-owned reconnect (task 14)', () => {
	/** Records the ORDER the bound methods are called in. */
	function recordCallOrder() {
		/** @type {string[]} */
		const order = []
		const App = window.go.main.App
		App.Disconnect.mockImplementation(async () => {
			order.push('Disconnect')
		})
		App.SetUnverifiedWriteConsent.mockImplementation(async () => {
			order.push('SetUnverifiedWriteConsent')
		})
		App.Connect.mockImplementation(async () => {
			order.push('Connect')
			return RECONNECTED_INFO
		})
		return order
	}

	function connectedUnarmed() {
		appState.setConnection(NEEDS_CONSENT_INFO)
		appState.setUISpec(specArmed(false))
	}

	it('accepting for the CONNECTED model disconnects FIRST, then persists, then re-opens the same port and model', async () => {
		connectedUnarmed()
		const order = recordCallOrder()

		await bindings.applyUnverifiedWriteConsent('FTdx10', true)

		expect(order).toEqual(['Disconnect', 'SetUnverifiedWriteConsent', 'Connect'])
		expect(window.go.main.App.SetUnverifiedWriteConsent).toHaveBeenCalledWith('FTdx10', true)
		// The SAME port and the SAME model — never appState.selectedModel,
		// whose '' would re-open this build's default radio instead.
		expect(appState.selectedModel).toBe('')
		expect(window.go.main.App.Connect).toHaveBeenCalledWith('/dev/tty.usb', 'FTdx10')
		expect(appState.connection).toEqual(RECONNECTED_INFO)
	})

	it('a REFUSED disconnect (busy / a transfer in flight) persists NOTHING and never reconnects — and surfaces the refusal', async () => {
		connectedUnarmed()
		window.go.main.App.Disconnect.mockRejectedValue('app: a transfer is running')

		await expect(bindings.applyUnverifiedWriteConsent('FTdx10', true)).rejects.toBe('app: a transfer is running')

		expect(window.go.main.App.SetUnverifiedWriteConsent).not.toHaveBeenCalled()
		expect(window.go.main.App.Connect).not.toHaveBeenCalled()
		// Still connected, still unarmed — nothing moved.
		expect(appState.connection).toEqual(NEEDS_CONSENT_INFO)
		expect(appState.alerts.some((a) => a.message.includes('a transfer is running'))).toBe(true)
	})

	it('a failed STORE write rejects too (nothing persisted) and does not re-open the session on a decision that was never recorded', async () => {
		connectedUnarmed()
		window.go.main.App.SetUnverifiedWriteConsent.mockRejectedValue('userconfig: settings.json is corrupt')

		await expect(bindings.applyUnverifiedWriteConsent('FTdx10', true)).rejects.toBe('userconfig: settings.json is corrupt')

		expect(window.go.main.App.Disconnect).toHaveBeenCalledTimes(1)
		expect(window.go.main.App.Connect).not.toHaveBeenCalled()
		expect(appState.alerts.some((a) => a.message.includes('settings.json is corrupt'))).toBe(true)
	})

	it('the ARMING dialogue’s DECLINE persists false and does NOT reconnect — its session was opened unconsented by construction', async () => {
		connectedUnarmed()

		await bindings.applyUnverifiedWriteConsent('FTdx10', false, { sessionUnconsented: true })

		expect(window.go.main.App.SetUnverifiedWriteConsent).toHaveBeenCalledWith('FTdx10', false)
		expect(window.go.main.App.Disconnect).not.toHaveBeenCalled()
		expect(window.go.main.App.Connect).not.toHaveBeenCalled()
		expect(appState.connection).toEqual(NEEDS_CONSENT_INFO)
	})

	it('the ARMING dialogue’s GRANT still orchestrates — an unconsented session is exactly the one a grant has to re-open', async () => {
		connectedUnarmed()
		const order = recordCallOrder()

		await bindings.applyUnverifiedWriteConsent('FTdx10', true, { sessionUnconsented: true })

		expect(order).toEqual(['Disconnect', 'SetUnverifiedWriteConsent', 'Connect'])
		expect(window.go.main.App.Connect).toHaveBeenCalledWith('/dev/tty.usb', 'FTdx10')
	})

	it('REVOKING an armed connected session runs the whole orchestration (disconnect → persist false → reconnect)', async () => {
		appState.setConnection({ ...NEEDS_CONSENT_INFO, UnverifiedConsentRecorded: true })
		appState.setUISpec(specArmed(true))
		const order = recordCallOrder()

		await bindings.applyUnverifiedWriteConsent('FTdx10', false)

		expect(order).toEqual(['Disconnect', 'SetUnverifiedWriteConsent', 'Connect'])
		expect(window.go.main.App.SetUnverifiedWriteConsent).toHaveBeenCalledWith('FTdx10', false)
	})

	// --- Final review, Codex BLOCKER: the uiSpec oracle is GONE from this
	// decision. A toggle aimed at the CONNECTED model on a REAL session
	// always orchestrates, because the frontend cannot know what the live
	// session was constructed with — and the failure that mattered was
	// silent: a spec fetch that failed (or had not caught up) read
	// "unconsented", which sent a REVOCATION of an armed session down the
	// direct-persist path, leaving an immutable live session still consented
	// and still writable while the store said the grant was gone. The only
	// price of the new rule is an occasional unnecessary reconnect, which is
	// visible, recoverable, and was accepted at adjudication.

	it('REVOKING the connected model with NO uiSpec at all (a failed spec fetch) still runs the whole orchestration', async () => {
		appState.setConnection({ ...NEEDS_CONSENT_INFO, UnverifiedConsentRecorded: true })
		// No setUISpec: this is exactly the state a failed GetUISpec leaves
		// behind, and the state whose false-y read used to skip the reconnect.
		expect(appState.uiSpec).toBeNull()
		const order = recordCallOrder()

		await bindings.applyUnverifiedWriteConsent('FTdx10', false)

		expect(order).toEqual(['Disconnect', 'SetUnverifiedWriteConsent', 'Connect'])
		expect(window.go.main.App.SetUnverifiedWriteConsent).toHaveBeenCalledWith('FTdx10', false)
		expect(window.go.main.App.Connect).toHaveBeenCalledWith('/dev/tty.usb', 'FTdx10')
	})

	it('REVOKING a GRANTED session whose uiSpec is STALE-FALSE still runs the whole orchestration', async () => {
		// The store says granted (Recorded, and the session was opened with
		// the grant), but the spec in hand says otherwise. The old oracle
		// believed the spec and persisted directly; the live session stayed
		// writable.
		appState.setConnection({ ...NEEDS_CONSENT_INFO, UnverifiedConsentRecorded: true })
		appState.setUISpec(specArmed(false))
		const order = recordCallOrder()

		await bindings.applyUnverifiedWriteConsent('FTdx10', false)

		expect(order).toEqual(['Disconnect', 'SetUnverifiedWriteConsent', 'Connect'])
		expect(window.go.main.App.Connect).toHaveBeenCalledWith('/dev/tty.usb', 'FTdx10')
	})

	it('GRANTING a session whose uiSpec already reads consented reconnects anyway — the accepted cost of not trusting the spec', async () => {
		appState.setConnection({ ...NEEDS_CONSENT_INFO, UnverifiedConsentRecorded: true })
		appState.setUISpec(specArmed(true))
		const order = recordCallOrder()

		await bindings.applyUnverifiedWriteConsent('FTdx10', true)

		expect(order).toEqual(['Disconnect', 'SetUnverifiedWriteConsent', 'Connect'])
	})

	it('a REVOCATION from the grants panel (no arming-dialogue knowledge) orchestrates even when the spec reads unconsented', async () => {
		connectedUnarmed()
		const order = recordCallOrder()

		await bindings.applyUnverifiedWriteConsent('FTdx10', false)

		expect(order).toEqual(['Disconnect', 'SetUnverifiedWriteConsent', 'Connect'])
	})

	it('persists directly, with no reconnect, while DISCONNECTED', async () => {
		await bindings.applyUnverifiedWriteConsent('FTdx10', true)

		expect(window.go.main.App.SetUnverifiedWriteConsent).toHaveBeenCalledWith('FTdx10', true)
		expect(window.go.main.App.Disconnect).not.toHaveBeenCalled()
		expect(window.go.main.App.Connect).not.toHaveBeenCalled()
	})

	it('persists directly for a model OTHER than the connected one', async () => {
		connectedUnarmed()

		await bindings.applyUnverifiedWriteConsent('FTDX101MP', true)

		expect(window.go.main.App.SetUnverifiedWriteConsent).toHaveBeenCalledWith('FTDX101MP', true)
		expect(window.go.main.App.Disconnect).not.toHaveBeenCalled()
		expect(window.go.main.App.Connect).not.toHaveBeenCalled()
	})

	it('persists directly for a DEMO session on the same model — a demo never spends consent, so there is nothing to re-open', async () => {
		appState.setConnection({ ...NEEDS_CONSENT_INFO, Demo: true })
		appState.setUISpec(specArmed(false))

		await bindings.applyUnverifiedWriteConsent('FTdx10', true)

		expect(window.go.main.App.SetUnverifiedWriteConsent).toHaveBeenCalledWith('FTdx10', true)
		expect(window.go.main.App.Disconnect).not.toHaveBeenCalled()
		expect(window.go.main.App.Connect).not.toHaveBeenCalled()
	})

	it('refreshes the grants list after every recorded decision', async () => {
		await bindings.applyUnverifiedWriteConsent('FTdx10', true)

		expect(window.go.main.App.ListUnverifiedWriteConsents).toHaveBeenCalled()
		expect(appState.unverifiedConsents).toEqual(CONSENT_ROWS)
	})

	it('the working copy SURVIVES the consent reconnect, with the baseline marked stale — the existing disconnect contract', async () => {
		connectedUnarmed()
		appState.setCodeplug({
			Schema: 1, Generator: 'x', Radio: {},
			Channels: [{ slot: '001', data: { freq_hz: 7100000 } }],
			WorkingPath: '/tmp/edited.json', Dirty: false, BaselineStale: false,
		})
		appState.setDirty(true)
		recordCallOrder()

		await bindings.applyUnverifiedWriteConsent('FTdx10', true)

		expect(appState.codeplug).not.toBeNull()
		expect(appState.codeplug.WorkingPath).toBe('/tmp/edited.json')
		expect(appState.codeplug.Channels[0].data.freq_hz).toBe(7100000)
		expect(appState.dirty).toBe(true)
		expect(appState.codeplug.BaselineStale).toBe(true)
	})

	it('invalidates any prepared send plan — the backend cleared its own at Disconnect', async () => {
		connectedUnarmed()
		recordCallOrder()
		const before = appState.preparedPlanEpoch

		await bindings.applyUnverifiedWriteConsent('FTdx10', true)

		expect(appState.preparedPlanEpoch).toBe(before + 1)
	})

	it('leaves a prepared plan alone when nothing was reconnected', async () => {
		const before = appState.preparedPlanEpoch

		await bindings.applyUnverifiedWriteConsent('FTdx10', true)

		expect(appState.preparedPlanEpoch).toBe(before)
	})

	it('a failed RECONNECT still resolves — the decision was recorded, and the alert strip carries the failure', async () => {
		connectedUnarmed()
		window.go.main.App.Connect.mockRejectedValue('port busy')

		await expect(bindings.applyUnverifiedWriteConsent('FTdx10', true)).resolves.toBeUndefined()

		expect(window.go.main.App.SetUnverifiedWriteConsent).toHaveBeenCalledWith('FTdx10', true)
		expect(appState.connection).toBeNull()
		expect(appState.alerts.some((a) => a.message.includes('port busy'))).toBe(true)
	})
})

describe('refreshUnverifiedConsents (task 14)', () => {
	it('stores every row ListUnverifiedWriteConsents returned, hardware-verified models included', async () => {
		const rows = await bindings.refreshUnverifiedConsents()

		expect(rows).toEqual(CONSENT_ROWS)
		expect(appState.unverifiedConsents).toEqual(CONSENT_ROWS)
		expect(appState.unverifiedConsents.some((r) => r.Model === 'FT-710' && r.NeedsConsent === false)).toBe(true)
	})

	it('alerts but does NOT throw on rejection — a listing failure must not break its caller', async () => {
		window.go.main.App.ListUnverifiedWriteConsents.mockRejectedValue('settings unreadable')

		const rows = await bindings.refreshUnverifiedConsents()

		expect(rows).toBeNull()
		expect(appState.unverifiedConsents).toEqual([])
		expect(appState.alerts.some((a) => a.message.includes('settings unreadable'))).toBe(true)
	})
})
