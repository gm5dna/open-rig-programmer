// SPDX-License-Identifier: GPL-3.0-or-later

// MHz↔Hz conversion for the frequency column (task-17 brief §2). This is
// FORMATTING, not validation — range/step checking is Go's job
// (core/codeplug.Validate). Both directions use string/integer maths
// only, never floating point, so integer Hz round-trip exactly:
// mhzToHz(hzToMHz(hz)) === hz for every integer hz, with no drift on
// values like 7.074000 MHz → 7074000 Hz.

/**
 * Formats integer Hz as an MHz string with exactly six decimal places
 * (single-Hz resolution — the radio's own storable precision).
 * Returns '' for anything that is not a non-negative integer.
 * @param {number} hz
 * @returns {string}
 */
export function hzToMHz(hz) {
	if (typeof hz !== 'number' || !Number.isInteger(hz) || hz < 0) return ''
	const digits = String(hz).padStart(7, '0')
	return `${digits.slice(0, -6)}.${digits.slice(-6)}`
}

// Accepts: optional whole part, optional '.' + up to six fractional
// digits — at least one digit somewhere. Nothing else (no sign, no
// grouping, no units): stricter is safer for a value that ends up on a
// radio, and the paste/editor paths alert on a null rather than guess.
const MHZ_PATTERN = /^(\d*)(?:\.(\d{0,6}))?$/

/**
 * Parses an MHz string ("7.074", "0.030", ".5", "14") to exact integer
 * Hz, or null if the text is not a plain non-negative MHz value with at
 * most six fractional digits (sub-Hz precision cannot round-trip, so it
 * is rejected rather than silently truncated).
 * @param {string} text
 * @returns {number | null}
 */
export function mhzToHz(text) {
	if (typeof text !== 'string') return null
	const match = MHZ_PATTERN.exec(text.trim())
	if (!match) return null
	const whole = match[1] ?? ''
	const frac = match[2] ?? ''
	if (whole === '' && frac === '') return null
	const fracPadded = frac.padEnd(6, '0')
	return Number(whole || '0') * 1_000_000 + Number(fracPadded)
}
