// SPDX-License-Identifier: GPL-3.0-or-later

// App shell / view-switch tests (task 36, M8b-6): the whole tree mounts
// here, so the bindings.js mock must cover every named export any
// child component (ConnectionBar, ActionBar -> SendFlowDialog, ChannelGrid,
// SettingsViewer) imports — see each component's own `from
// './bridge/bindings.js'` import list.

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/svelte'
import { appState } from '../lib/state/app.svelte.js'
import App from '../App.svelte'

vi.mock('../lib/bridge/bindings.js', () => ({
	initTransferEvents: vi.fn(),
	refreshUISpec: vi.fn().mockResolvedValue(null),
	refreshSettingsSpec: vi.fn().mockResolvedValue(null),
	refreshAppVersion: vi.fn().mockResolvedValue(null),
	setWindowTitle: vi.fn(),
	listPorts: vi.fn().mockResolvedValue([]),
	refreshSupportedModels: vi.fn().mockResolvedValue([]),
	connect: vi.fn(),
	connectDemo: vi.fn(),
	disconnect: vi.fn(),
	readRadio: vi.fn(),
	loadFile: vi.fn(),
	saveFile: vi.fn(),
	saveFileAs: vi.fn(),
	prepareSend: vi.fn(),
	importCSV: vi.fn(),
	importCHIRP: vi.fn(),
	exportCSV: vi.fn(),
	confirmSend: vi.fn(),
	cancelTransfer: vi.fn(),
	updateChannel: vi.fn(),
	updateChannels: vi.fn(),
	readSettingsRadio: vi.fn(),
}))

const SETTINGS_SPEC = {
	Live: false,
	DescriptorVersion: 'synthetic-v1',
	Menus: [{ ID: 'M1', Label: 'MENU ALPHA', Groups: [] }],
}

function codeplugFixture() {
	return { Schema: 1, Generator: 'test', Radio: {}, Channels: [], WorkingPath: '', Dirty: false, BaselineStale: false }
}

const UI_SPEC = {
	Live: false,
	Banks: [{ ID: 'MEM', Label: 'Memories', ReadOnly: false, Slots: [] }],
	Modes: [],
	ShiftOptions: [],
	CTCSSStateOptions: [],
	Tones: [],
	TagMaxBytes: 12,
	ClarMaxHz: 9990,
	ClarStepHz: 10,
}

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

describe('Channels|Settings view switch', () => {
	it('opens on Channels by default, rendering ChannelGrid', () => {
		appState.setUISpec(UI_SPEC)
		appState.setCodeplug(codeplugFixture())
		render(App)
		expect(screen.getByRole('tab', { name: 'Channels' })).toHaveAttribute('aria-selected', 'true')
		expect(screen.getByRole('tab', { name: 'Settings' })).toHaveAttribute('aria-selected', 'false')
		expect(screen.getByRole('grid')).toBeInTheDocument() // ChannelGrid's own table
	})

	it('switching to Settings renders SettingsViewer; back to Channels renders ChannelGrid, appState intact both ways', async () => {
		appState.setUISpec(UI_SPEC)
		appState.setSettingsSpec(SETTINGS_SPEC)
		appState.setCodeplug(codeplugFixture())
		appState.setSettings({ HasSnapshot: true, Descriptor: '', Complete: true, HasLegacy: false, Entries: [] })
		render(App)

		await fireEvent.click(screen.getByRole('tab', { name: 'Settings' }))
		expect(appState.activeView).toBe('settings')
		expect(screen.queryByRole('grid')).not.toBeInTheDocument()
		expect(screen.getByRole('tab', { name: 'MENU ALPHA' })).toBeInTheDocument() // SettingsViewer's own menu tab

		await fireEvent.click(screen.getByRole('tab', { name: 'Channels' }))
		expect(appState.activeView).toBe('channels')
		expect(screen.getByRole('grid')).toBeInTheDocument()
		expect(screen.queryByRole('tab', { name: 'MENU ALPHA' })).not.toBeInTheDocument()

		// Neither switch touched the underlying data.
		expect(appState.uiSpec).toEqual(UI_SPEC)
		expect(appState.settingsSpec).toEqual(SETTINGS_SPEC)
		expect(appState.codeplug).not.toBeNull()
		expect(appState.settings?.HasSnapshot).toBe(true)
	})

	it('keyboard: ArrowRight moves the view tab, roving tabindex follows', async () => {
		appState.setUISpec(UI_SPEC)
		render(App)
		const channelsTab = screen.getByRole('tab', { name: 'Channels' })
		expect(channelsTab).toHaveAttribute('tabindex', '0')

		await fireEvent.keyDown(channelsTab, { key: 'ArrowRight' })
		expect(screen.getByRole('tab', { name: 'Settings' })).toHaveAttribute('aria-selected', 'true')
		expect(screen.getByRole('tab', { name: 'Settings' })).toHaveAttribute('tabindex', '0')
		expect(screen.getByRole('tab', { name: 'Channels' })).toHaveAttribute('tabindex', '-1')
	})
})
