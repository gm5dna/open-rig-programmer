// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/svelte'
import { appState } from '../state/app.svelte.js'
import ConnectionBar from '../ConnectionBar.svelte'

vi.mock('../bridge/bindings.js', () => ({
	listPorts: vi.fn().mockResolvedValue([]),
	refreshSupportedModels: vi.fn().mockResolvedValue([]),
	connect: vi.fn().mockResolvedValue(undefined),
	connectDemo: vi.fn().mockResolvedValue(undefined),
	disconnect: vi.fn().mockResolvedValue(undefined),
	// Task 14 (M9d): the consent dialogue ConnectionBar now mounts imports
	// these two from the same module.
	applyUnverifiedWriteConsent: vi.fn().mockResolvedValue(undefined),
	refreshUnverifiedConsents: vi.fn().mockResolvedValue([]),
}))

import { listPorts, refreshSupportedModels, connect, connectDemo } from '../bridge/bindings.js'

function resetState() {
	appState.clearConnection()
	appState.setPorts([])
	appState.setPortsLoading(false)
	appState.setConnecting(false)
	appState.setSupportedModels([])
	appState.setSelectedModel('')
	appState.setUISpec(null)
	appState.setUnverifiedConsentPrompt(null)
	appState.setUnverifiedConsents([])
	appState.closeUnverifiedGrants()
	appState.setSendDialogOpen(false)
	appState.alerts = []
}

beforeEach(() => {
	resetState()
	vi.clearAllMocks()
	listPorts.mockResolvedValue([])
	refreshSupportedModels.mockResolvedValue([])
	connect.mockResolvedValue(undefined)
	connectDemo.mockResolvedValue(undefined)
})

describe('ConnectionBar', () => {
	it('calls listPorts once on mount', () => {
		render(ConnectionBar)
		expect(listPorts).toHaveBeenCalledTimes(1)
	})

	it('renders every port from appState as a select option', () => {
		appState.setPorts([
			{ Path: '/dev/tty.usbserial-A', Description: 'Silicon Labs CP210x', Score: 10, Hints: ['likely'] },
			{ Path: '/dev/tty.usbserial-B', Description: '', Score: 1, Hints: [] },
		])
		render(ConnectionBar)

		expect(screen.getByRole('option', { name: /\/dev\/tty\.usbserial-A.*Silicon Labs CP210x.*likely/ })).toBeInTheDocument()
		expect(screen.getByRole('option', { name: '/dev/tty.usbserial-B' })).toBeInTheDocument()
	})

	it('the demo control is a separate, clearly-labelled button — not a dropdown option', () => {
		appState.setPorts([{ Path: '/dev/tty.usbserial-A', Description: 'FTDI', Score: 5, Hints: [] }])
		render(ConnectionBar)

		const demoButton = screen.getByRole('button', { name: 'Demo (simulated radio)' })
		expect(demoButton).toBeInTheDocument()

		// Not present as one of the port <option>s.
		const options = screen.getAllByRole('option').map((o) => o.textContent)
		expect(options.some((text) => text?.toLowerCase().includes('demo'))).toBe(false)
	})

	it('clicking Connect calls the connect binding with the selected port', async () => {
		appState.setPorts([{ Path: '/dev/tty.usbserial-A', Description: 'FTDI', Score: 5, Hints: [] }])
		render(ConnectionBar)

		const select = screen.getByLabelText('Port')
		await fireEvent.change(select, { target: { value: '/dev/tty.usbserial-A' } })

		const connectButton = screen.getByRole('button', { name: 'Connect' })
		expect(connectButton).not.toBeDisabled()
		await fireEvent.click(connectButton)

		expect(connect).toHaveBeenCalledWith('/dev/tty.usbserial-A')
	})

	it('clicking Demo calls the connectDemo binding', async () => {
		render(ConnectionBar)
		await fireEvent.click(screen.getByRole('button', { name: 'Demo (simulated radio)' }))
		expect(connectDemo).toHaveBeenCalledTimes(1)
	})

	it('Connect is disabled until a port is selected', () => {
		appState.setPorts([{ Path: '/dev/tty.usbserial-A', Description: 'FTDI', Score: 5, Hints: [] }])
		render(ConnectionBar)
		expect(screen.getByRole('button', { name: 'Connect' })).toBeDisabled()
	})

	it('renders a ConnectionInfo badge once connected, distinguishing a live connection', () => {
		appState.setConnection({ Model: 'FT-710', CATID: '0800', Port: '/dev/tty.usbserial-A', USBSerial: '', Region: '', Demo: false })
		render(ConnectionBar)

		expect(screen.getByText(/FT-710/)).toBeInTheDocument()
		expect(screen.getByText(/ID 0800/)).toBeInTheDocument()
		expect(screen.getByText(/\/dev\/tty\.usbserial-A/)).toBeInTheDocument()
		expect(screen.queryByText('DEMO')).not.toBeInTheDocument()
	})

	it('renders a visibly distinct DEMO badge for a demo connection', () => {
		appState.setConnection({ Model: 'FT-710', CATID: '0800', Port: 'fake', USBSerial: 'SIM0001', Region: '', Demo: true })
		render(ConnectionBar)

		expect(screen.getByText('DEMO')).toBeInTheDocument()
	})

	it('disables port selection, Connect and Demo once connected, and shows Disconnect instead', () => {
		appState.setConnection({ Model: 'FT-710', CATID: '0800', Port: '/dev/tty.usbserial-A', USBSerial: '', Region: '', Demo: false })
		render(ConnectionBar)

		expect(screen.getByLabelText('Port')).toBeDisabled()
		expect(screen.getByRole('button', { name: 'Demo (simulated radio)' })).toBeDisabled()
		expect(screen.getByRole('button', { name: 'Disconnect' })).toBeInTheDocument()
		expect(screen.queryByRole('button', { name: 'Connect' })).not.toBeInTheDocument()
	})

	it('Disconnect is disabled while a transfer is active (Codex M6 #2, adjudicated HIGH, remedy 2d — mirrors Go now refusing Disconnect during the reservation)', () => {
		appState.setConnection({ Model: 'FT-710', CATID: '0800', Port: '/dev/tty.usbserial-A', USBSerial: '', Region: '', Demo: false })
		appState.beginTransfer('read')
		render(ConnectionBar)

		expect(screen.getByRole('button', { name: 'Disconnect' })).toBeDisabled()
	})
})

describe('ConnectionBar model picker (task 13, M9d)', () => {
	it('fetches the supported-model list once on mount — the list is Go\'s (GetSupportedModels), never a hard-coded one here', () => {
		render(ConnectionBar)
		expect(refreshSupportedModels).toHaveBeenCalledTimes(1)
	})

	it('renders the Radio select as exactly Default plus one option per supported model, in the order Go gave them', () => {
		appState.setSupportedModels(['FT-710', 'FTDX101D', 'FTdx10'])
		render(ConnectionBar)

		// Scoped to the model select (the port picker has options of its
		// own), and an exact ordered comparison rather than a containment
		// one: count and order both matter, so a hard-coded list in the
		// markup — the very thing this picker exists to remove — could not
		// pass this.
		const select = /** @type {HTMLSelectElement} */ (screen.getByLabelText('Radio'))
		expect([...select.options].map((o) => o.textContent?.trim())).toEqual([
			'Default',
			'FT-710',
			'FTDX101D',
			'FTdx10',
		])
	})

	it('starts on the default option, whose value is "" — an untouched picker connects exactly as before it existed', () => {
		appState.setSupportedModels(['FT-710', 'FTdx10'])
		render(ConnectionBar)

		const select = /** @type {HTMLSelectElement} */ (screen.getByLabelText('Radio'))
		expect(select.value).toBe('')
		expect(appState.selectedModel).toBe('')
	})

	it('choosing a radio stores it in appState.selectedModel, which is what the bridge forwards', async () => {
		appState.setSupportedModels(['FT-710', 'FTdx10'])
		appState.setPorts([{ Path: '/dev/tty.usbserial-A', Description: 'FTDI', Score: 5, Hints: [] }])
		render(ConnectionBar)

		await fireEvent.change(screen.getByLabelText('Radio'), { target: { value: 'FTdx10' } })
		expect(appState.selectedModel).toBe('FTdx10')

		await fireEvent.change(screen.getByLabelText('Port'), { target: { value: '/dev/tty.usbserial-A' } })
		await fireEvent.click(screen.getByRole('button', { name: 'Connect' }))

		// connect() takes only the port: the chosen model rides in appState,
		// so the demo path picks up the same choice with no second argument.
		expect(connect).toHaveBeenCalledWith('/dev/tty.usbserial-A')
	})

	it('the choice also reaches the demo path — clicking Demo after picking a radio leaves the choice in place for connectDemo to forward', async () => {
		appState.setSupportedModels(['FT-710', 'FTdx10'])
		render(ConnectionBar)

		await fireEvent.change(screen.getByLabelText('Radio'), { target: { value: 'FTdx10' } })
		await fireEvent.click(screen.getByRole('button', { name: 'Demo (simulated radio)' }))

		expect(connectDemo).toHaveBeenCalledTimes(1)
		expect(appState.selectedModel).toBe('FTdx10')
	})

	it('the radio picker is disabled once connected — the model is fixed for the life of a session', () => {
		appState.setSupportedModels(['FT-710', 'FTdx10'])
		appState.setConnection({ Model: 'FT-710', CATID: '0800', Port: '/dev/tty.usbserial-A', USBSerial: '', Region: '', Demo: false })
		render(ConnectionBar)

		expect(screen.getByLabelText('Radio')).toBeDisabled()
	})

	it('the radio picker is disabled while a connect attempt is in flight', () => {
		appState.setSupportedModels(['FT-710', 'FTdx10'])
		appState.setConnecting(true)
		render(ConnectionBar)

		expect(screen.getByLabelText('Radio')).toBeDisabled()
	})
})

// --- Task 14 (M9d): the consent affordances -----------------------------

/** A UISpecView carrying only the field the amber indicator reads. The
 * badge derives from the live session's capability label and NOTHING else
 * — never from the connection, never from the settings store. */
/** @param {boolean} consented */
function specArmed(consented) {
	return { Live: true, Banks: [], Modes: [], ShiftOptions: [], CTCSSStateOptions: [], Tones: [], TagMaxBytes: 12, ClarMaxHz: 9990, ClarStepHz: 10, UnverifiedWritesConsented: consented }
}

describe('ConnectionBar unverified-write consent affordances (task 14, M9d)', () => {
	it('offers "Unverified writes…" even while disconnected — the grants panel is always reachable', async () => {
		render(ConnectionBar)

		const button = screen.getByRole('button', { name: 'Unverified writes…' })
		expect(button).not.toBeDisabled()

		await fireEvent.click(button)
		expect(appState.unverifiedGrantsOpen).toBe(true)
	})

	it('shows no amber indicator when the live spec reports no consented writes', () => {
		appState.setConnection({ Model: 'FTdx10', CATID: '0761', Port: 'COM3', USBSerial: '', Region: '', Demo: false, NeedsUnverifiedConsent: true, UnverifiedConsentRecorded: true })
		appState.setUISpec(specArmed(false))
		render(ConnectionBar)

		expect(screen.queryByRole('button', { name: /unverified writes enabled/i })).not.toBeInTheDocument()
	})

	it('shows the amber indicator when the live spec DOES carry consented writes, and it opens the same panel', async () => {
		appState.setUISpec(specArmed(true))
		render(ConnectionBar)

		const badge = screen.getByRole('button', { name: /unverified writes enabled/i })
		await fireEvent.click(badge)
		expect(appState.unverifiedGrantsOpen).toBe(true)
	})

	it('mounts the arming dialogue when one is due, and the grants panel when it is open', async () => {
		appState.setUnverifiedConsentPrompt({ Model: 'FTdx10', NeedsConsent: true, Granted: false, Recorded: false, Warning: 'the backend warning' })
		render(ConnectionBar)

		expect(screen.getByText('the backend warning')).toBeInTheDocument()
		expect(screen.getByRole('button', { name: 'Enable unverified writes' })).toBeInTheDocument()
	})
})

// UISpecView.Transmit (this task): radio-level ANATOMY —
// core/spec.Capabilities.Transmit, served as "has_transmitter",
// "receive_only" or "" — was sent to the frontend and read by nothing
// here. The radio badge is where this app already states the connected
// radio's identity, so it is where the one fact about that radio's
// anatomy belongs. It is NOT used to hide columns: which columns a bank
// renders is BankView.Fields' contract alone (grid/columns.js's
// columnsFor), and that must not gain a second source.
describe('ConnectionBar receiver label (UISpecView.Transmit)', () => {
	const connected = { Model: 'IC-R8600', CATID: '96', Port: '/dev/tty.usbserial-A', USBSerial: '', Region: '', Demo: false }

	it('labels a receive_only radio "Receive only" beside its model', () => {
		appState.setConnection(connected)
		appState.setUISpec({ Transmit: 'receive_only', Banks: [] })
		render(ConnectionBar)
		expect(screen.getByText(/Receive only/)).toBeInTheDocument()
	})

	it('says nothing for a transceiver, and nothing for an unspecified Transmit', () => {
		appState.setConnection({ ...connected, Model: 'FT-710', CATID: '0800' })
		appState.setUISpec({ Transmit: 'has_transmitter', Banks: [] })
		const { unmount } = render(ConnectionBar)
		expect(screen.queryByText(/Receive only/)).not.toBeInTheDocument()
		unmount()

		// The zero value: a capability set that never stated its anatomy
		// says nothing rather than guessing either way.
		appState.setUISpec({ Transmit: '', Banks: [] })
		render(ConnectionBar)
		expect(screen.queryByText(/Receive only/)).not.toBeInTheDocument()
	})

	it('says nothing while nothing is connected — the label describes the radio on the cable', () => {
		appState.setUISpec({ Transmit: 'receive_only', Banks: [] })
		render(ConnectionBar)
		expect(screen.getByText('Not connected')).toBeInTheDocument()
		expect(screen.queryByText(/Receive only/)).not.toBeInTheDocument()
	})
})
