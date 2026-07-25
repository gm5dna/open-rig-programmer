// SPDX-License-Identifier: GPL-3.0-or-later

// MHz↔Hz formatting (task-17 brief §2): "MHz↔Hz conversion is
// formatting, do it carefully and test it: exact integer Hz round-trip,
// no floating-point drift on values like 7.074000 MHz → 7074000 Hz".
// Both directions are string/integer maths — no floating point anywhere.

import { describe, it, expect } from 'vitest'
import { hzToMHz, mhzToHz } from '../freq.js'

describe('hzToMHz', () => {
	it('formats integer Hz as MHz with exactly six decimal places', () => {
		expect(hzToMHz(7074000)).toBe('7.074000')
		expect(hzToMHz(14074000)).toBe('14.074000')
		expect(hzToMHz(145500000)).toBe('145.500000')
	})

	it('handles sub-MHz frequencies with a leading zero (30 kHz = radio minimum)', () => {
		expect(hzToMHz(30000)).toBe('0.030000')
		expect(hzToMHz(999999)).toBe('0.999999')
	})

	it('handles single-Hz precision without drift', () => {
		expect(hzToMHz(7074001)).toBe('7.074001')
		expect(hzToMHz(74999990)).toBe('74.999990')
	})

	it('formats zero as 0.000000', () => {
		expect(hzToMHz(0)).toBe('0.000000')
	})

	it('returns an empty string for non-finite or non-integer input', () => {
		expect(hzToMHz(NaN)).toBe('')
		expect(hzToMHz(Infinity)).toBe('')
		expect(hzToMHz(7.5)).toBe('')
		// @ts-expect-error deliberate bad input
		expect(hzToMHz(null)).toBe('')
		// @ts-expect-error deliberate bad input
		expect(hzToMHz(undefined)).toBe('')
	})
})

describe('mhzToHz', () => {
	it('parses plain MHz values to exact integer Hz', () => {
		expect(mhzToHz('7.074')).toBe(7074000)
		expect(mhzToHz('7.074000')).toBe(7074000)
		expect(mhzToHz('14.074')).toBe(14074000)
	})

	it('parses whole-number MHz', () => {
		expect(mhzToHz('7')).toBe(7000000)
		expect(mhzToHz('50')).toBe(50000000)
	})

	it('parses fraction-only input (sub-MHz)', () => {
		expect(mhzToHz('.030')).toBe(30000)
		expect(mhzToHz('0.030')).toBe(30000)
	})

	it('pads short fractions rather than misreading them', () => {
		expect(mhzToHz('7.1')).toBe(7100000)
		expect(mhzToHz('7.07')).toBe(7070000)
	})

	it('tolerates surrounding whitespace', () => {
		expect(mhzToHz('  7.074  ')).toBe(7074000)
	})

	it('accepts exactly six fractional digits (single-Hz precision)', () => {
		expect(mhzToHz('7.074001')).toBe(7074001)
	})

	it('rejects sub-Hz precision (more than six fractional digits)', () => {
		expect(mhzToHz('7.0740001')).toBeNull()
	})

	it('rejects non-numeric, negative, empty and malformed input', () => {
		expect(mhzToHz('')).toBeNull()
		expect(mhzToHz('   ')).toBeNull()
		expect(mhzToHz('abc')).toBeNull()
		expect(mhzToHz('-7.074')).toBeNull()
		expect(mhzToHz('7,074')).toBeNull()
		expect(mhzToHz('7.0.74')).toBeNull()
		expect(mhzToHz('.')).toBeNull()
		expect(mhzToHz('7 MHz')).toBeNull()
	})

	it('round-trips exactly: mhzToHz(hzToMHz(hz)) === hz across representative values', () => {
		const samples = [30000, 999999, 1000000, 7074000, 7074001, 14074000, 21200500, 50313000, 74999990]
		for (const hz of samples) {
			expect(mhzToHz(hzToMHz(hz))).toBe(hz)
		}
	})

	it('round-trips the other way: hzToMHz(mhzToHz(text)) preserves six-decimal form', () => {
		expect(hzToMHz(/** @type {number} */ (mhzToHz('7.074000')))).toBe('7.074000')
		expect(hzToMHz(/** @type {number} */ (mhzToHz('0.030000')))).toBe('0.030000')
	})
})
