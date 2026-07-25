// SPDX-License-Identifier: GPL-3.0-or-later

// Renders any binding-rejection value (string, Error, or something else
// entirely — Wails' own rejection shape is not part of the documented
// contract) as one readable line. Shared by bridge/bindings.js (for the
// alert strip) and any dialogue that needs the same text INLINE, because
// a modal dialogue can sit visually above the alert strip and must not
// rely on it alone to explain a synchronous rejection (task-18: the send
// dialogue's confirm-digest-mismatch/no-active-plan pre-flight failures).
// Deliberately has no wailsjs import, unlike bindings.js.

/**
 * @param {unknown} err
 * @returns {string}
 */
export function describeError(err) {
	if (err instanceof Error) return err.message
	if (typeof err === 'string') return err
	if (err && typeof err === 'object' && typeof (/** @type {{message?:unknown}} */ (err)).message === 'string') {
		return /** @type {{message:string}} */ (err).message
	}
	return String(err ?? 'an unexpected error occurred')
}
