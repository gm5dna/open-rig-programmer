// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/svelte'
import { appState } from '../state/app.svelte.js'
import ConnectionBar from '../ConnectionBar.svelte'

vi.mock('../bridge/bindings.js', () => ({
	listPorts: vi.fn().mockResolvedValue([]),
	connect: vi.fn().mockResolvedValue(undefined),
	connectDemo: vi.fn().mockResolvedValue(undefined),
	disconnect: vi.fn().mockResolvedValue(undefined),
}))

import { listPorts, connect, connectDemo } from '../bridge/bindings.js'

function resetState() {
	appState.clearConnection()
	appState.setPorts([])
	appState.setPortsLoading(false)
	appState.setConnecting(false)
	appState.alerts = []
}

beforeEach(() => {
	resetState()
	vi.clearAllMocks()
	listPorts.mockResolvedValue([])
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
