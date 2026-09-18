// SPDX-License-Identifier: GPL-3.0-or-later

// Extends vitest's `expect` with jest-dom's DOM matchers (toBeDisabled,
// toBeInTheDocument, etc) for component tests. @testing-library/svelte's
// own vite plugin (see vite.config.js) adds its OWN setup file (auto
// cleanup) after this one — both run.
import '@testing-library/jest-dom/vitest'

// happy-dom implements <dialog>.showModal()/.close() (and the 'close'
// event .close() fires), but not the native behaviour where a REAL
// Escape key press while a dialog is open via showModal() fires a
// cancelable 'cancel' event first. A real browser would not honour that
// native behaviour for Testing Library's fireEvent.keyDown either (it
// dispatches an untrusted event), so this listens for the same
// synthetic keydown and reproduces exactly what a trusted Escape press
// does — letting Modal.svelte's own oncancel handler (the one thing
// production relies on) run identically under test.
// Node 22+'s own experimental global Web Storage shadows happy-dom's
// working implementation (it wins the property, then throws/warns and
// returns undefined without --localstorage-file) — breaks any test that
// touches window.localStorage regardless of what it's testing. Swap in a
// plain in-memory Storage stand-in whenever the real thing isn't usable.
try {
	window.localStorage.setItem('__vitest_probe__', '1')
	window.localStorage.removeItem('__vitest_probe__')
} catch {
	/** @type {Map<string, string>} */
	const store = new Map()
	Object.defineProperty(window, 'localStorage', {
		configurable: true,
		value: {
			getItem: (/** @type {string} */ k) => (store.has(k) ? store.get(k) : null),
			setItem: (/** @type {string} */ k, /** @type {unknown} */ v) => void store.set(k, String(v)),
			removeItem: (/** @type {string} */ k) => void store.delete(k),
			clear: () => store.clear(),
		},
	})
}

document.addEventListener('keydown', (e) => {
	if (e.key !== 'Escape') return
	const dialog = document.querySelector('dialog[open]')
	if (!dialog) return
	const cancelEvent = new Event('cancel', { cancelable: true })
	dialog.dispatchEvent(cancelEvent)
	if (!cancelEvent.defaultPrevented) /** @type {HTMLDialogElement} */ (dialog).close()
})
