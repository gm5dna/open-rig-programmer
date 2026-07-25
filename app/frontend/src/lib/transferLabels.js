// SPDX-License-Identifier: GPL-3.0-or-later

// Shared display labels for transfer:progress's Phase and a transfer's
// Kind ('read'|'diff'|'prepare'|'send'|'settings') — factored out of
// StatusBar.svelte (task 16) so the task-18 send dialogue's own live
// progress view (which must show the same phase wording inside the modal,
// not just the status bar underneath it) does not duplicate the mapping.
//
// Task 36 (M8b-6) adds 'settings'/'read-settings': a settings read has
// exactly one phase throughout (there is no separate verify step), so
// 'read-settings' reuses the same short "Reading" wording the channel
// read's own 'read' phase already carries — StatusBar's line reads
// "Reading settings: Reading 3/9 <item>", the same {kindLabel}: {phaseLabel}
// composition every other kind already uses.

/** @type {Record<string, string>} */
const PHASE_LABELS = {
	read: 'Reading',
	'verify-read': 'Verifying (pre-write)',
	write: 'Writing',
	verify: 'Verifying',
	'read-settings': 'Reading',
}

/** @param {string} phase @returns {string} */
export function phaseLabel(phase) {
	if (!phase) return ''
	return PHASE_LABELS[phase] ?? phase.charAt(0).toUpperCase() + phase.slice(1)
}

/** @type {Record<string, string>} */
const KIND_LABELS = { read: 'Read', diff: 'Compare', prepare: 'Prepare', send: 'Send', settings: 'Reading settings' }

/** @param {string | null | undefined} kind @returns {string} */
export function kindLabel(kind) {
	return (kind && KIND_LABELS[kind]) ?? kind ?? ''
}
