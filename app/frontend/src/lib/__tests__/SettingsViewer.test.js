// SPDX-License-Identifier: GPL-3.0-or-later

// SettingsViewer component tests (task-36 brief): mocked bridge, real
// appState singleton. The spec fixture is deliberately SYNTHETIC (menu/
// group/item labels and IDs invented for this test, never a real FT-710
// name) — the whole point of this suite is proving the component renders
// nothing but what the spec/settings views hand it, with ZERO protocol
// facts hardcoded in the component itself.

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/svelte'
import { appState } from '../state/app.svelte.js'
import SettingsViewer from '../SettingsViewer.svelte'

vi.mock('../bridge/bindings.js', () => ({
	readSettingsRadio: vi.fn().mockResolvedValue({ HasSnapshot: true, Descriptor: '', Complete: true, HasLegacy: false, Entries: [] }),
}))

import { readSettingsRadio } from '../bridge/bindings.js'

const readSettingsRadioMock = vi.mocked(readSettingsRadio)

/** A wholly synthetic SettingsSpecView — none of these labels/IDs are
 * real FT-710 menu/EX vocabulary (task-36 brief: "the vitest fixture is
 * deliberately synthetic to prove it"). */
const SETTINGS_SPEC = {
	Live: false,
	DescriptorVersion: 'synthetic-v1',
	Menus: [
		{
			ID: 'M1',
			Label: 'MENU ALPHA',
			Groups: [
				{
					ID: 'G1',
					Label: 'GROUP BETA',
					Items: [
						{ ID: '990101', Label: 'SYNTH ITEM ONE', Display: '1-01' },
						{ ID: '990102', Label: 'SYNTH ITEM TWO', Display: '1-02' },
					],
				},
			],
		},
		{
			ID: 'M2',
			Label: 'MENU GAMMA',
			Groups: [
				{
					ID: 'G2',
					Label: 'GROUP DELTA',
					Items: [{ ID: '990201', Label: 'SYNTH ITEM THREE', Display: '2-01' }],
				},
			],
		},
	],
}

/** @param {object} [overrides] */
function settingsFixture(overrides = {}) {
	return {
		HasSnapshot: true,
		Descriptor: 'synthetic-v1',
		Complete: true,
		HasLegacy: false,
		Entries: [
			{ ID: '990101', Value: 'ON', State: 'known' },
			{ ID: '990102', Value: '', State: 'unavailable' },
			{ ID: '990201', Value: '', State: 'unsupported' },
		],
		...overrides,
	}
}

function codeplugFixture() {
	return { Schema: 1, Generator: 'test', Radio: {}, Channels: [], WorkingPath: '', Dirty: false, BaselineStale: false }
}

function connect() {
	appState.setConnection({ Model: 'FT-710', CATID: '0800', Port: '/dev/tty.usb', USBSerial: '', Region: '', Demo: false })
}

function resetState() {
	appState.clearConnection()
	appState.setSettingsSpec(null)
	appState.setSettings(null)
	appState.setActiveView('channels')
	appState.alerts = []
}

beforeEach(() => {
	resetState()
	vi.clearAllMocks()
	readSettingsRadioMock.mockResolvedValue({ HasSnapshot: true, Descriptor: '', Complete: true, HasLegacy: false, Entries: [] })
})

describe('rendering from spec + entries only', () => {
	it('renders menu tabs, subgroup headings and item rows (Display/Label/Value) from the synthetic spec', () => {
		appState.setSettingsSpec(SETTINGS_SPEC)
		appState.setCodeplug(codeplugFixture())
		appState.setSettings(settingsFixture())
		render(SettingsViewer)

		expect(screen.getByRole('tablist', { name: 'Settings menus' })).toBeInTheDocument()
		expect(screen.getByRole('tab', { name: 'MENU ALPHA' })).toBeInTheDocument()
		expect(screen.getByRole('tab', { name: 'MENU GAMMA' })).toBeInTheDocument()
		expect(screen.getByText('GROUP BETA')).toBeInTheDocument()
		expect(screen.getByText('SYNTH ITEM ONE')).toBeInTheDocument()
		expect(screen.getByText('1-01')).toBeInTheDocument()
		expect(screen.getByText('ON')).toBeInTheDocument()
	})

	it('renders nothing menu-specific without a spec', () => {
		appState.setSettingsSpec(null)
		appState.setCodeplug(codeplugFixture())
		appState.setSettings(settingsFixture())
		render(SettingsViewer)

		expect(screen.queryByRole('tablist')).not.toBeInTheDocument()
		expect(screen.queryByText('MENU ALPHA')).not.toBeInTheDocument()
		expect(screen.queryByText('SYNTH ITEM ONE')).not.toBeInTheDocument()
	})
})

describe('menu tablist keyboard (own fresh markup — behavioural parity with ChannelGrid’s bank tablist)', () => {
	beforeEach(() => {
		appState.setSettingsSpec(SETTINGS_SPEC)
		appState.setCodeplug(codeplugFixture())
		appState.setSettings(settingsFixture())
	})

	it('completes the tab pattern: aria-controls, roving tabindex, arrow-key wrap, Home/End, aria-selected moves', async () => {
		render(SettingsViewer)
		const alpha = screen.getByRole('tab', { name: 'MENU ALPHA' })
		const gamma = screen.getByRole('tab', { name: 'MENU GAMMA' })
		expect(alpha).toHaveAttribute('aria-selected', 'true')
		expect(alpha).toHaveAttribute('tabindex', '0')
		expect(gamma).toHaveAttribute('aria-selected', 'false')
		expect(gamma).toHaveAttribute('tabindex', '-1')
		expect(alpha).toHaveAttribute('aria-controls', 'settings-menu-panel')
		expect(gamma).toHaveAttribute('aria-controls', 'settings-menu-panel')

		await fireEvent.keyDown(alpha, { key: 'ArrowRight' })
		expect(screen.getByRole('tab', { name: 'MENU GAMMA' })).toHaveAttribute('aria-selected', 'true')
		expect(screen.getByRole('tab', { name: 'MENU ALPHA' })).toHaveAttribute('tabindex', '-1')
		const panel = screen.getByRole('tabpanel')
		expect(panel).toHaveAttribute('id', 'settings-menu-panel')
		expect(panel).toHaveAttribute('aria-labelledby', 'settings-menu-tab-M2')

		// Wraps forward past the last tab back to the first.
		await fireEvent.keyDown(screen.getByRole('tab', { name: 'MENU GAMMA' }), { key: 'ArrowRight' })
		expect(screen.getByRole('tab', { name: 'MENU ALPHA' })).toHaveAttribute('aria-selected', 'true')

		// Wraps backward past the first tab to the last.
		await fireEvent.keyDown(screen.getByRole('tab', { name: 'MENU ALPHA' }), { key: 'ArrowLeft' })
		expect(screen.getByRole('tab', { name: 'MENU GAMMA' })).toHaveAttribute('aria-selected', 'true')

		await fireEvent.keyDown(screen.getByRole('tab', { name: 'MENU GAMMA' }), { key: 'Home' })
		expect(screen.getByRole('tab', { name: 'MENU ALPHA' })).toHaveAttribute('aria-selected', 'true')

		await fireEvent.keyDown(screen.getByRole('tab', { name: 'MENU ALPHA' }), { key: 'End' })
		expect(screen.getByRole('tab', { name: 'MENU GAMMA' })).toHaveAttribute('aria-selected', 'true')
	})

	it('exactly one tab carries tabindex 0 at any time', async () => {
		render(SettingsViewer)
		let zeroTabs = screen.getAllByRole('tab').filter((t) => t.getAttribute('tabindex') === '0')
		expect(zeroTabs).toHaveLength(1)

		await fireEvent.click(screen.getByRole('tab', { name: 'MENU GAMMA' }))
		zeroTabs = screen.getAllByRole('tab').filter((t) => t.getAttribute('tabindex') === '0')
		expect(zeroTabs).toHaveLength(1)
		expect(zeroTabs[0]).toHaveAccessibleName('MENU GAMMA')
	})

	it('clicking a tab switches the panel content', async () => {
		render(SettingsViewer)
		expect(screen.getByText('SYNTH ITEM ONE')).toBeInTheDocument()
		expect(screen.queryByText('SYNTH ITEM THREE')).not.toBeInTheDocument()

		await fireEvent.click(screen.getByRole('tab', { name: 'MENU GAMMA' }))
		expect(screen.getByText('SYNTH ITEM THREE')).toBeInTheDocument()
		expect(screen.queryByText('SYNTH ITEM ONE')).not.toBeInTheDocument()
	})
})

describe('value/state rendering', () => {
	beforeEach(() => {
		appState.setSettingsSpec(SETTINGS_SPEC)
		appState.setCodeplug(codeplugFixture())
	})

	it('shows a known value plain, with no state badge', () => {
		appState.setSettings(settingsFixture())
		render(SettingsViewer)
		const row = screen.getByText('SYNTH ITEM ONE').closest('tr')
		expect(row?.textContent).toContain('ON')
		expect(row?.textContent).not.toMatch(/unavailable|unsupported|not read/)
	})

	it('badges an unavailable entry with accessible (visible) text', () => {
		appState.setSettings(settingsFixture())
		render(SettingsViewer)
		const row = screen.getByText('SYNTH ITEM TWO').closest('tr')
		expect(row?.textContent).toMatch(/unavailable/)
	})

	it('badges an unsupported entry, in its own menu tab', async () => {
		appState.setSettings(settingsFixture())
		render(SettingsViewer)
		await fireEvent.click(screen.getByRole('tab', { name: 'MENU GAMMA' }))
		const row = screen.getByText('SYNTH ITEM THREE').closest('tr')
		expect(row?.textContent).toMatch(/unsupported/)
	})

	it('renders an "Unrecognised settings" section for entries whose ID is not in the spec', () => {
		appState.setSettings(
			settingsFixture({
				Entries: [
					{ ID: '990101', Value: 'ON', State: 'known' },
					{ ID: '999999', Value: 'weird value', State: 'known' },
				],
			})
		)
		render(SettingsViewer)
		expect(screen.getByText('Unrecognised settings')).toBeInTheDocument()
		expect(screen.getByText('999999')).toBeInTheDocument()
		expect(screen.getByText('weird value')).toBeInTheDocument()
	})

	it('omits the "Unrecognised settings" section when every entry matches the spec', () => {
		appState.setSettings(settingsFixture())
		render(SettingsViewer)
		expect(screen.queryByText('Unrecognised settings')).not.toBeInTheDocument()
	})
})

describe('empty states', () => {
	beforeEach(() => {
		appState.setSettingsSpec(SETTINGS_SPEC)
	})

	it('shows guidance and no tablist when no codeplug is loaded', () => {
		render(SettingsViewer)
		expect(screen.getByText(/no codeplug loaded/i)).toBeInTheDocument()
		expect(screen.queryByRole('tablist', { name: 'Settings menus' })).not.toBeInTheDocument()
	})

	it('shows a "Read settings from radio" CTA once a codeplug is loaded but has no snapshot', () => {
		connect()
		appState.setCodeplug(codeplugFixture())
		render(SettingsViewer)
		expect(screen.getByText(/no settings/i)).toBeInTheDocument()
		expect(screen.getByRole('button', { name: 'Read settings from radio' })).not.toBeDisabled()
	})

	it('clicking the CTA calls the mocked readSettingsRadio', async () => {
		connect()
		appState.setCodeplug(codeplugFixture())
		render(SettingsViewer)
		await fireEvent.click(screen.getByRole('button', { name: 'Read settings from radio' }))
		expect(readSettingsRadioMock).toHaveBeenCalledTimes(1)
	})

	it('shows a legacy-data notice when HasLegacy is true', () => {
		connect()
		appState.setCodeplug(codeplugFixture())
		appState.setSettings(settingsFixture({ HasLegacy: true }))
		render(SettingsViewer)
		expect(screen.getByText(/preserved/i)).toBeInTheDocument()
		expect(screen.getByText(/legacy/i)).toBeInTheDocument()
	})
})

describe('read button — single-reason disabled tooltip (mirrors sendBlockedReason)', () => {
	beforeEach(() => {
		appState.setSettingsSpec(SETTINGS_SPEC)
	})

	function readButton() {
		return screen.getByRole('button', { name: 'Read settings from radio' })
	}

	it('disabled — disconnected', () => {
		appState.setCodeplug(codeplugFixture())
		render(SettingsViewer)
		expect(readButton()).toBeDisabled()
		expect(readButton().closest('span')?.title).toMatch(/connect/i)
	})

	it('disabled — nothing loaded', () => {
		connect()
		render(SettingsViewer)
		expect(readButton()).toBeDisabled()
		expect(readButton().closest('span')?.title).toMatch(/read the radio|open a codeplug/i)
	})

	it('disabled — transfer active', () => {
		connect()
		appState.setCodeplug(codeplugFixture())
		appState.beginTransfer('read')
		render(SettingsViewer)
		expect(readButton()).toBeDisabled()
		expect(readButton().closest('span')?.title).toMatch(/transfer/i)
	})

	it('enabled, no tooltip, when connected + loaded + idle', () => {
		connect()
		appState.setCodeplug(codeplugFixture())
		render(SettingsViewer)
		expect(readButton()).not.toBeDisabled()
		expect(readButton().title).toBe('')
	})
})
