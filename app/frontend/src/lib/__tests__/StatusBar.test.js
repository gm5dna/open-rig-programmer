// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import { appState } from '../state/app.svelte.js'
import StatusBar from '../StatusBar.svelte'

function resetState() {
	appState.clearConnection()
	appState.setIssues([])
	appState.setDirty(false)
	// clearConnection deliberately leaves appVersion alone (it is not
	// connection-scoped), so reset it here or it leaks between tests.
	appState.setAppVersion(null)
}

beforeEach(() => {
	resetState()
})

describe('StatusBar', () => {
	it('shows "Saved" and no dirty emphasis when clean', () => {
		render(StatusBar)
		expect(screen.getByText('Saved')).toBeInTheDocument()
	})

	it('shows "Unsaved changes" once dirty', () => {
		appState.setDirty(true)
		render(StatusBar)
		expect(screen.getByText('Unsaved changes')).toBeInTheDocument()
	})

	it('shows the validation-error count from severity "error" issues only', () => {
		appState.setIssues([
			{ Slot: '001', Field: 'freq', Severity: 'error', Msg: 'bad' },
			{ Slot: '002', Field: 'tone', Severity: 'warning', Msg: 'hmm' },
		])
		render(StatusBar)
		expect(screen.getByText('1 validation issue')).toBeInTheDocument()
	})

	it('pluralises the issue count', () => {
		appState.setIssues([
			{ Slot: '001', Field: 'freq', Severity: 'error', Msg: 'bad' },
			{ Slot: '002', Field: 'freq', Severity: 'error', Msg: 'bad' },
		])
		render(StatusBar)
		expect(screen.getByText('2 validation issues')).toBeInTheDocument()
	})

	it('shows "Idle" when no transfer is active and there is no prior outcome', () => {
		render(StatusBar)
		expect(screen.getByText('Idle')).toBeInTheDocument()
	})

	it('renders phase + counter + slot while a transfer is active', () => {
		appState.beginTransfer('read')
		appState.applyProgress({ Phase: 'read', Done: 42, Total: 117, Slot: 'CH-042' })
		render(StatusBar)

		const label = screen.getByText((text) => text.includes('Reading') && text.includes('42/117'))
		expect(label).toBeInTheDocument()
		expect(screen.getByText('CH-042')).toBeInTheDocument()
		expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '36')
	})

	it('renders a friendly label for the "verify-read" phase', () => {
		appState.beginTransfer('send')
		appState.applyProgress({ Phase: 'verify-read', Done: 1, Total: 1, Slot: '001' })
		render(StatusBar)
		expect(screen.getByText((text) => text.includes('Verifying (pre-write)'))).toBeInTheDocument()
	})

	it('shows the last non-ok outcome once the transfer ends', () => {
		appState.beginTransfer('send')
		appState.applyTransferDone({ Kind: 'send', Outcome: 'cancelled', Report: null, Message: 'x' })
		render(StatusBar)
		expect(screen.getByText('Last Send: cancelled')).toBeInTheDocument()
	})

	it('does not linger on the last outcome once it was "ok" — shows Idle instead', () => {
		appState.beginTransfer('read')
		appState.applyTransferDone({ Kind: 'read', Outcome: 'ok', Report: null, Message: '' })
		render(StatusBar)
		expect(screen.getByText('Idle')).toBeInTheDocument()
	})

	describe('build version', () => {
		it('shows nothing until the version fetch has resolved', () => {
			render(StatusBar)
			expect(screen.queryByTestId('app-version')).not.toBeInTheDocument()
		})

		it('renders the backend-composed Display verbatim for a release build', () => {
			appState.setAppVersion({ Version: 'v1.0.0', Display: 'v1.0.0', IsRelease: true })
			render(StatusBar)

			const chip = screen.getByTestId('app-version')
			expect(chip).toHaveTextContent('v1.0.0')
			expect(chip).not.toHaveClass('version-label-unreleased')
		})

		it('renders the unreleased-build wording from Go, not composed here', () => {
			appState.setAppVersion({
				Version: 'dev',
				Display: 'dev (unreleased build)',
				IsRelease: false,
			})
			render(StatusBar)

			const chip = screen.getByTestId('app-version')
			expect(chip).toHaveTextContent('dev (unreleased build)')
			expect(chip).toHaveClass('version-label-unreleased')
		})

		it('does not invent wording — whatever Display says is what is shown', () => {
			// The frontend must not reconstruct the string from Version +
			// IsRelease: a future backend wording change lands here with no
			// JS edit at all.
			appState.setAppVersion({
				Version: 'v2.3.4',
				Display: 'v2.3.4 — release candidate',
				IsRelease: true,
			})
			render(StatusBar)
			expect(screen.getByTestId('app-version')).toHaveTextContent('v2.3.4 — release candidate')
		})
	})
})
