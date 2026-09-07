// SPDX-License-Identifier: GPL-3.0-or-later

// Shared roving-tablist keyboard handler for this app's three tablists
// (App.svelte's Channels|Settings view switch, SettingsViewer's menu
// tabs, ChannelGrid's bank tabs) — the standard ARIA tablist pattern:
// wrapping Left/Right and jump-to-edge Home/End, with no separate
// "focus without selecting" state (moving IS selecting here).
//
/**
 * @param {KeyboardEvent} e
 * @param {number} index - the currently-focused tab's index
 * @param {number} length - how many tabs there are
 * @param {(target: number) => void} select - called with the target
 *   index once a navigation key resolves one; the caller does the
 *   actual selection and DOM focus() (each has its own id scheme).
 */
export function tabKeydown(e, index, length, select) {
	let target = null
	if (e.key === 'ArrowRight') target = (index + 1) % length
	else if (e.key === 'ArrowLeft') target = (index - 1 + length) % length
	else if (e.key === 'Home') target = 0
	else if (e.key === 'End') target = length - 1
	if (target === null || length === 0) return
	e.preventDefault()
	select(target)
}
