// SPDX-License-Identifier: GPL-3.0-or-later

// CloneReadDialog tests (Phase 4b): the pick -> arming -> prompt ->
// receiving -> done|refused state machine, the armed-before-prompt rule
// (spec.md §Read model point 1 — 'prompt' must never render before
// "clone:armed" has fired), each refusal error's own message, and the
// no-write-affordance assertion the milestone plan requires: this
// dialogue renders no send/write control in any state.

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte'
import { appState } from '../state/app.svelte.js'
import CloneReadDialog from '../CloneReadDialog.svelte'

/** @type {(payload: {Model: string}) => void} */
let armedHandler = () => {}

vi.mock('../bridge/bindings.js', () => ({
	getCloneModels: vi.fn(),
	armCloneRead: vi.fn(),
	cancelCloneRead: vi.fn(),
	receiveCloneImage: vi.fn(),
	onCloneArmed: vi.fn((handler) => {
		armedHandler = handler
		return vi.fn() // unsubscribe
	}),
	listPorts: vi.fn().mockResolvedValue([]),
}))

import { getCloneModels, armCloneRead, cancelCloneRead, receiveCloneImage } from '../bridge/bindings.js'

const getCloneModelsMock = vi.mocked(getCloneModels)
const armCloneReadMock = vi.mocked(armCloneRead)
const cancelCloneReadMock = vi.mocked(cancelCloneRead)
const receiveCloneImageMock = vi.mocked(receiveCloneImage)

beforeEach(() => {
	appState.clearConnection()
	appState.ports = [{ Path: '/dev/ttyUSB0', Description: 'USB serial', Score: 1, Hints: [] }]
	vi.clearAllMocks()
	getCloneModelsMock.mockResolvedValue(['FT-817', 'FT-817ND'])
	armCloneReadMock.mockResolvedValue(undefined)
	cancelCloneReadMock.mockResolvedValue(undefined)
	receiveCloneImageMock.mockResolvedValue({})
})

/** Selects the model and port pickers and clicks Arm. */
async function armFromPickPhase() {
	await waitFor(() => expect(screen.getByRole('option', { name: 'FT-817' })).toBeInTheDocument())
	await fireEvent.change(screen.getByLabelText('Port'), { target: { value: '/dev/ttyUSB0' } })
	await fireEvent.click(screen.getByRole('button', { name: 'Arm' }))
}

describe('state machine', () => {
	it('starts on pick, with Arm disabled until a port is chosen', async () => {
		render(CloneReadDialog, { onClose: vi.fn() })
		expect(screen.getByText('Clone-mode read')).toBeInTheDocument()
		await waitFor(() => expect(screen.getByRole('option', { name: 'FT-817' })).toBeInTheDocument())
		expect(screen.getByRole('button', { name: 'Arm' })).toBeDisabled()
	})

	it('does not show the prompt phase until "clone:armed" fires — armed-before-prompt', async () => {
		// armCloneRead resolves but never calls armedHandler in this case,
		// simulating the real ordering where the event, not the promise, is
		// what the dialogue actually keys off.
		let resolveArm
		armCloneReadMock.mockReturnValue(new Promise((r) => (resolveArm = r)))

		render(CloneReadDialog, { onClose: vi.fn() })
		await armFromPickPhase()

		expect(screen.getByText('Arming…')).toBeInTheDocument()
		expect(screen.queryByText('Ready to receive')).not.toBeInTheDocument()

		resolveArm(undefined)
		await waitFor(() => expect(screen.getByText('Arming…')).toBeInTheDocument())
		// Still arming: the promise settled, but no armed event has fired.
		expect(screen.queryByText('Ready to receive')).not.toBeInTheDocument()

		armedHandler({ Model: 'FT-817' })
		await waitFor(() => expect(screen.getByText('Ready to receive')).toBeInTheDocument())
	})

	it('goes pick -> arming -> prompt (on the armed event) -> receiving -> done on success', async () => {
		render(CloneReadDialog, { onClose: vi.fn() })
		await armFromPickPhase()
		armedHandler({ Model: 'FT-817' })
		await waitFor(() => expect(screen.getByText('Ready to receive')).toBeInTheDocument())

		await fireEvent.click(screen.getByRole('button', { name: 'Receiving…' }))
		await waitFor(() => expect(screen.getByText('Clone-mode read complete')).toBeInTheDocument())
		expect(receiveCloneImageMock).toHaveBeenCalledOnce()
	})

	it('renders each refusal error and returns to pick on Try again', async () => {
		receiveCloneImageMock.mockRejectedValue(new Error('clonewire: image length matches no candidate profile'))

		render(CloneReadDialog, { onClose: vi.fn() })
		await armFromPickPhase()
		armedHandler({ Model: 'FT-817' })
		await waitFor(() => expect(screen.getByText('Ready to receive')).toBeInTheDocument())
		await fireEvent.click(screen.getByRole('button', { name: 'Receiving…' }))

		await waitFor(() => expect(screen.getByText('Read refused')).toBeInTheDocument())
		expect(screen.getByText(/matches no candidate profile/)).toBeInTheDocument()

		await fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
		expect(screen.getByText('Clone-mode read')).toBeInTheDocument()
	})

	it('cancels an armed-but-unreceived read on close, releasing the reservation', async () => {
		const onClose = vi.fn()
		render(CloneReadDialog, { onClose })
		await armFromPickPhase()
		armedHandler({ Model: 'FT-817' })
		await waitFor(() => expect(screen.getByText('Ready to receive')).toBeInTheDocument())

		await fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
		expect(cancelCloneReadMock).toHaveBeenCalledOnce()
		expect(onClose).toHaveBeenCalledOnce()
	})
})

describe('no write/send affordance', () => {
	it('renders no send/write control in any state', async () => {
		render(CloneReadDialog, { onClose: vi.fn() })
		const assertNoWriteControl = () => {
			for (const el of screen.queryAllByRole('button')) {
				expect(el.textContent ?? '').not.toMatch(/send|write/i)
			}
		}

		assertNoWriteControl() // pick
		await armFromPickPhase()
		assertNoWriteControl() // arming

		armedHandler({ Model: 'FT-817' })
		await waitFor(() => expect(screen.getByText('Ready to receive')).toBeInTheDocument())
		assertNoWriteControl() // prompt

		await fireEvent.click(screen.getByRole('button', { name: 'Receiving…' }))
		assertNoWriteControl() // receiving

		await waitFor(() => expect(screen.getByText('Clone-mode read complete')).toBeInTheDocument())
		assertNoWriteControl() // done
	})
})
