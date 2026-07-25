// SPDX-License-Identifier: GPL-3.0-or-later

// Modal.svelte tests (task-18 brief: "modal dialogues are in-app
// components with focus management — trap focus in the modal, Escape
// closes where safe"). SendFlowDialog.test.js already covers the
// never-mid-transfer Escape rule via a real dialogue; this file pins the
// generic mechanics directly: initial focus, Tab/Shift-Tab wrapping
// within the dialog only, backdrop click, and focus restoration on close.

import { describe, it, expect, vi } from 'vitest'
import { render, fireEvent } from '@testing-library/svelte'
import { createRawSnippet } from 'svelte'
import Modal from '../Modal.svelte'

/** A snippet rendering three buttons, so the trap has something to cycle
 * through. @returns {import('svelte').Snippet} */
function threeButtons() {
	return createRawSnippet(() => ({
		render: () => '<div><button id="a">A</button><button id="b">B</button><button id="c">C</button></div>',
	}))
}

/** @param {ParentNode} root @param {string} selector @returns {HTMLElement} */
function focusEl(root, selector) {
	const el = root.querySelector(selector)
	if (!(el instanceof HTMLElement)) throw new Error(`no element for ${selector}`)
	el.focus()
	return el
}

describe('Modal focus trap', () => {
	it('focuses the first focusable element on mount', () => {
		render(Modal, { labelledBy: 'x', children: threeButtons() })
		expect(document.activeElement?.id).toBe('a')
	})

	it('Tab from the last element wraps to the first (never escapes the dialog)', async () => {
		const { container } = render(Modal, { labelledBy: 'x', children: threeButtons() })
		const dialog = container.querySelector('[role="dialog"]')
		focusEl(container, '#c')
		await fireEvent.keyDown(dialog, { key: 'Tab' })
		expect(document.activeElement?.id).toBe('a')
	})

	it('Shift-Tab from the first element wraps to the last', async () => {
		const { container } = render(Modal, { labelledBy: 'x', children: threeButtons() })
		const dialog = container.querySelector('[role="dialog"]')
		focusEl(container, '#a')
		await fireEvent.keyDown(dialog, { key: 'Tab', shiftKey: true })
		expect(document.activeElement?.id).toBe('c')
	})

	it('closable backdrop click calls onclose', async () => {
		const onclose = vi.fn()
		const { container } = render(Modal, { labelledBy: 'x', onclose, children: threeButtons() })
		await fireEvent.click(container.querySelector('.modal-backdrop'))
		expect(onclose).toHaveBeenCalledTimes(1)
	})

	it('a click on the panel itself does not close (only the backdrop does)', async () => {
		const onclose = vi.fn()
		const { container } = render(Modal, { labelledBy: 'x', onclose, children: threeButtons() })
		await fireEvent.click(container.querySelector('.modal-panel'))
		expect(onclose).not.toHaveBeenCalled()
	})

	it('closable=false suppresses both Escape and the backdrop click', async () => {
		const onclose = vi.fn()
		const { container } = render(Modal, { labelledBy: 'x', closable: false, onclose, children: threeButtons() })
		const dialog = container.querySelector('[role="dialog"]')
		await fireEvent.keyDown(dialog, { key: 'Escape' })
		await fireEvent.click(container.querySelector('.modal-backdrop'))
		expect(onclose).not.toHaveBeenCalled()
	})

	it('restores focus to the previously-focused element once unmounted', async () => {
		const trigger = document.createElement('button')
		trigger.id = 'trigger'
		document.body.appendChild(trigger)
		trigger.focus()
		expect(document.activeElement?.id).toBe('trigger')

		const { unmount } = render(Modal, { labelledBy: 'x', children: threeButtons() })
		expect(document.activeElement?.id).toBe('a')

		unmount()
		expect(document.activeElement?.id).toBe('trigger')
		trigger.remove()
	})
})
