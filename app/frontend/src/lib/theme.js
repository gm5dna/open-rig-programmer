// SPDX-License-Identifier: GPL-3.0-or-later

// Light/dark/system theme (v1.8.0 Phase 5). 'light'/'dark' set
// <html data-theme="…">, which style.css's :root[data-theme="light"] and
// :root[data-theme="dark"] key off directly. 'system' (or anything
// missing/unrecognised) clears the attribute so style.css's
// prefers-color-scheme media query decides instead — that's what makes
// System follow the OS live, with no listener needed here.

const STORAGE_KEY = 'theme'

/** @param {string} theme */
export function applyTheme(theme) {
	if (theme === 'light' || theme === 'dark') {
		document.documentElement.setAttribute('data-theme', theme)
	} else {
		document.documentElement.removeAttribute('data-theme')
	}
}

/** @returns {string} */
export function getStoredTheme() {
	try {
		return window.localStorage.getItem(STORAGE_KEY) ?? 'system'
	} catch {
		return 'system'
	}
}

/** @param {string} theme */
export function setTheme(theme) {
	try {
		window.localStorage.setItem(STORAGE_KEY, theme)
	} catch {
		// Private browsing / storage disabled: still applies for this
		// session, just doesn't persist.
	}
	applyTheme(theme)
}

/** Call once, as early as possible (top of main.js), so <html> carries
 * the right data-theme before Svelte's first paint. */
export function initTheme() {
	applyTheme(getStoredTheme())
}
