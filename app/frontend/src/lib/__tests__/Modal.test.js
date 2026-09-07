// SPDX-License-Identifier: GPL-3.0-or-later

// Modal.svelte tests (task-18 brief: "modal dialogues are in-app
// components with focus management — trap focus in the modal, Escape
// closes where safe"). SendFlowDialog.test.js already covers the
// never-mid-transfer Escape rule via a real dialogue; this file pins the
// generic mechanics directly: initial focus, backdrop click, and focus
// restoration on close. Tab/Shift-Tab containment within the dialog is
// native <dialog>/showModal() behaviour (ponytail audit 2026-09-06,
// finding 44) — a real browser guarantee, not something a synthetic
// keydown can exercise here, so it is no longer pinned in this suite.

import { describe, it, expect, vi } from 'vitest'
import { render, fireEvent } from '@testing-library/svelte'
import { createRawSnippet } from 'svelte'
import Modal from '../Modal.svelte'

/** A snippet rendering three buttons, so initial-focus has something to
 * land on. @returns {import('svelte').Snippet} */
function threeButtons() {
	return createRawSnippet(() => ({
		render: () => '<div><button id="a">A</button><button id="b">B</button><button id="c">C</button></div>',
	}))
}

describe('Modal focus trap', () => {
	it('focuses the first focusable element on mount', () => {
		render(Modal, { labelledBy: 'x', children: threeButtons() })
		expect(document.activeElement?.id).toBe('a')
	})

	// A native <dialog>'s ::backdrop is not a hit-testable descendant: a
	// genuine backdrop click's event target IS the dialog element itself
	// (Modal.svelte's own onBackdropClick relies on exactly this), so
	// firing the click on the dialog directly is what a backdrop click
	// looks like here; firing it on a descendant is what a click on the
	// dialog's own rendered content looks like.
	it('closable backdrop click calls onclose', async () => {
		const onclose = vi.fn()
		const { container } = render(Modal, { labelledBy: 'x', onclose, children: threeButtons() })
		await fireEvent.click(container.querySelector('[role="dialog"]'))
		expect(onclose).toHaveBeenCalledTimes(1)
	})

	it('a click on the dialog\'s own content does not close (only the backdrop does)', async () => {
		const onclose = vi.fn()
		const { container } = render(Modal, { labelledBy: 'x', onclose, children: threeButtons() })
		await fireEvent.click(container.querySelector('#a'))
		expect(onclose).not.toHaveBeenCalled()
	})

	it('closable=false suppresses both Escape and the backdrop click', async () => {
		const onclose = vi.fn()
		const { container } = render(Modal, { labelledBy: 'x', closable: false, onclose, children: threeButtons() })
		const dialog = container.querySelector('[role="dialog"]')
		await fireEvent.keyDown(dialog, { key: 'Escape' })
		await fireEvent.click(dialog)
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
