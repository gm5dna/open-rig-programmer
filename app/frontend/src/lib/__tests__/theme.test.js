// SPDX-License-Identifier: GPL-3.0-or-later

// initTheme (v1.8.0 Phase 5): sets <html data-theme> from the persisted
// 'theme' localStorage key, and clears it for 'system' (or nothing
// stored) so style.css's prefers-color-scheme media query decides.

import { describe, it, expect, beforeEach } from 'vitest'
import { initTheme } from '../theme.js'

beforeEach(() => {
	window.localStorage.clear()
	document.documentElement.removeAttribute('data-theme')
})

describe('initTheme', () => {
	it('sets data-theme from a stored light/dark choice', () => {
		window.localStorage.setItem('theme', 'dark')
		initTheme()
		expect(document.documentElement.getAttribute('data-theme')).toBe('dark')

		window.localStorage.setItem('theme', 'light')
		initTheme()
		expect(document.documentElement.getAttribute('data-theme')).toBe('light')
	})

	it('clears data-theme for System (stored or absent)', () => {
		document.documentElement.setAttribute('data-theme', 'dark')
		window.localStorage.setItem('theme', 'system')
		initTheme()
		expect(document.documentElement.hasAttribute('data-theme')).toBe(false)

		document.documentElement.setAttribute('data-theme', 'light')
		window.localStorage.clear()
		initTheme()
		expect(document.documentElement.hasAttribute('data-theme')).toBe(false)
	})
})
