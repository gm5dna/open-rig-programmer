// SPDX-License-Identifier: GPL-3.0-or-later

// Task 14 (M9d): the consent dialogue's two modes.
//
//   - 'arm'    — the question asked once, after a real connect to a
//                consent-eligible radio with no decision recorded. Its body
//                is the BACKEND's Warning, verbatim: the frontend states no
//                hardware claim of its own, so there is exactly one place
//                that sentence can be wrong.
//   - 'manage' — the always-reachable grants panel: every supported model,
//                hardware-verified ones included (shown "n/a", toggle
//                disabled), each eligible one revocable at any time.
//
// The reconnect orchestration a toggle may trigger lives in the BRIDGE and
// is pinned by bindings.test.js — here we only assert which call this
// dialogue makes, and what it does with the answer.

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/svelte'
import { appState } from '../state/app.svelte.js'
import UnverifiedWritesDialog from '../UnverifiedWritesDialog.svelte'

vi.mock('../bridge/bindings.js', () => ({
	applyUnverifiedWriteConsent: vi.fn().mockResolvedValue(undefined),
	refreshUnverifiedConsents: vi.fn().mockResolvedValue([]),
}))

import { applyUnverifiedWriteConsent, refreshUnverifiedConsents } from '../bridge/bindings.js'

const applyMock = vi.mocked(applyUnverifiedWriteConsent)
const refreshMock = vi.mocked(refreshUnverifiedConsents)

/** internal/radiotext.UnverifiedWriteWarningTemplate's real shape, filled
 * for one radio — the text Go serves and this dialogue must render
 * unaltered. */
const WARNING =
	"This project has never written to a real FTdx10. Enabling unverified writes sends memory-write commands that are documented in the manufacturer's CAT reference and exercised against a simulator, but have not been proven on real hardware. Every write is read back and compared, and stops on any mismatch — but a misinterpreted frame could corrupt the targeted memory channel. You can revoke this at any time."

const PROMPT = { Model: 'FTdx10', NeedsConsent: true, Granted: false, Recorded: false, Warning: WARNING }

const ROWS = [
	{ Model: 'FT-710', NeedsConsent: false, Granted: false, Recorded: false, Warning: '' },
	{ Model: 'FTDX101D', NeedsConsent: true, Granted: true, Recorded: true, Warning: 'w' },
	{ Model: 'FTdx10', NeedsConsent: true, Granted: false, Recorded: false, Warning: WARNING },
]

beforeEach(() => {
	appState.clearConnection()
	appState.setUISpec(null)
	appState.setUnverifiedConsentPrompt(null)
	appState.setUnverifiedConsents([])
	appState.closeUnverifiedGrants()
	appState.setSendDialogOpen(false)
	appState.alerts = []
	vi.clearAllMocks()
	applyMock.mockResolvedValue(undefined)
	refreshMock.mockResolvedValue([])
})

describe('arming mode', () => {
	it("renders the backend's warning verbatim — no frontend re-wording of a hardware claim", () => {
		appState.setUnverifiedConsentPrompt(PROMPT)
		render(UnverifiedWritesDialog, { mode: 'arm' })

		const body = screen.getByTestId('unverified-warning')
		expect(body.textContent).toBe(WARNING)
	})

	it('names the radio in its title', () => {
		appState.setUnverifiedConsentPrompt(PROMPT)
		render(UnverifiedWritesDialog, { mode: 'arm' })
		expect(screen.getByRole('heading').textContent).toContain('FTdx10')
	})

	it('"Enable unverified writes" records a grant and closes the dialogue', async () => {
		appState.setUnverifiedConsentPrompt(PROMPT)
		render(UnverifiedWritesDialog, { mode: 'arm' })

		await fireEvent.click(screen.getByRole('button', { name: 'Enable unverified writes' }))

		// The third argument is what this dialogue KNOWS by construction: it
		// is raised only for a connection with no decision recorded, so its
		// session was opened unconsented. The bridge no longer infers that
		// from the UI spec (final review, Codex BLOCKER).
		expect(applyMock).toHaveBeenCalledWith('FTdx10', true, { sessionUnconsented: true })
		expect(appState.unverifiedConsentPrompt).toBeNull()
	})

	it('"Not now" records a DECLINE (a decision, so it is never re-asked) and closes', async () => {
		appState.setUnverifiedConsentPrompt(PROMPT)
		render(UnverifiedWritesDialog, { mode: 'arm' })

		await fireEvent.click(screen.getByRole('button', { name: 'Not now' }))

		expect(applyMock).toHaveBeenCalledWith('FTdx10', false, { sessionUnconsented: true })
		expect(appState.unverifiedConsentPrompt).toBeNull()
	})

	it('a refusal (nothing persisted) leaves the dialogue OPEN, with the reason shown inline', async () => {
		appState.setUnverifiedConsentPrompt(PROMPT)
		applyMock.mockRejectedValue('app: a transfer is running')
		render(UnverifiedWritesDialog, { mode: 'arm' })

		await fireEvent.click(screen.getByRole('button', { name: 'Enable unverified writes' }))

		expect(appState.unverifiedConsentPrompt).toEqual(PROMPT)
		expect(screen.getByText(/a transfer is running/)).toBeInTheDocument()
	})

	it('both answers are disabled while a transfer is running', () => {
		appState.setUnverifiedConsentPrompt(PROMPT)
		appState.beginTransfer('read')
		render(UnverifiedWritesDialog, { mode: 'arm' })

		expect(screen.getByRole('button', { name: 'Enable unverified writes' })).toBeDisabled()
		expect(screen.getByRole('button', { name: 'Not now' })).toBeDisabled()
	})

	it('both answers are disabled while a send dialogue is open', () => {
		appState.setUnverifiedConsentPrompt(PROMPT)
		appState.setSendDialogOpen(true)
		render(UnverifiedWritesDialog, { mode: 'arm' })

		expect(screen.getByRole('button', { name: 'Enable unverified writes' })).toBeDisabled()
	})
})

describe('grants panel (manage mode)', () => {
	it('refreshes the listing when it opens — the store is shared with the CLI, so it can change behind the app', () => {
		render(UnverifiedWritesDialog, { mode: 'manage' })
		expect(refreshMock).toHaveBeenCalledTimes(1)
	})

	it('lists every supported model, hardware-verified ones included', () => {
		appState.setUnverifiedConsents(ROWS)
		render(UnverifiedWritesDialog, { mode: 'manage' })

		expect(screen.getByText('FT-710')).toBeInTheDocument()
		expect(screen.getByText('FTDX101D')).toBeInTheDocument()
		expect(screen.getByText('FTdx10')).toBeInTheDocument()
	})

	it('shows a hardware-verified radio as n/a, with its toggle disabled — there is no unverified write for a consent to unlock', () => {
		appState.setUnverifiedConsents(ROWS)
		render(UnverifiedWritesDialog, { mode: 'manage' })

		expect(screen.getByTestId('consent-state-FT-710').textContent).toContain('n/a')
		expect(screen.getByLabelText('Unverified writes for FT-710')).toBeDisabled()
	})

	it('an eligible radio’s toggle reflects the recorded grant', () => {
		appState.setUnverifiedConsents(ROWS)
		render(UnverifiedWritesDialog, { mode: 'manage' })

		expect(screen.getByLabelText('Unverified writes for FTDX101D')).toBeChecked()
		expect(screen.getByLabelText('Unverified writes for FTdx10')).not.toBeChecked()
	})

	it('granting an eligible radio calls the bridge with true', async () => {
		appState.setUnverifiedConsents(ROWS)
		render(UnverifiedWritesDialog, { mode: 'manage' })

		await fireEvent.click(screen.getByLabelText('Unverified writes for FTdx10'))

		expect(applyMock).toHaveBeenCalledWith('FTdx10', true, { sessionUnconsented: false })
	})

	it('revoking a granted radio calls the bridge with false, claiming NO knowledge of the live session', async () => {
		appState.setUnverifiedConsents(ROWS)
		render(UnverifiedWritesDialog, { mode: 'manage' })

		await fireEvent.click(screen.getByLabelText('Unverified writes for FTDX101D'))

		// sessionUnconsented FALSE from the grants panel, always: this panel
		// is reachable at any time, for any radio, and knows nothing about
		// how the live session (if there is one) was constructed. A revocation
		// from here therefore always re-opens a matching live session.
		expect(applyMock).toHaveBeenCalledWith('FTDX101D', false, { sessionUnconsented: false })
	})

	it('every toggle is disabled while a transfer is running, and says why', () => {
		appState.setUnverifiedConsents(ROWS)
		appState.beginTransfer('send')
		render(UnverifiedWritesDialog, { mode: 'manage' })

		expect(screen.getByLabelText('Unverified writes for FTdx10')).toBeDisabled()
		expect(screen.getByTestId('consent-blocked-reason').textContent).toMatch(/transfer/i)
	})

	it('every toggle is disabled while a send dialogue is open', () => {
		appState.setUnverifiedConsents(ROWS)
		appState.setSendDialogOpen(true)
		render(UnverifiedWritesDialog, { mode: 'manage' })

		expect(screen.getByLabelText('Unverified writes for FTdx10')).toBeDisabled()
	})

	it('a refused change is shown inline and leaves the panel open, with the toggle back where the store says it belongs', async () => {
		appState.setUnverifiedConsents(ROWS)
		applyMock.mockRejectedValue('app: a transfer is running')
		appState.openUnverifiedGrants()
		render(UnverifiedWritesDialog, { mode: 'manage' })

		await fireEvent.click(screen.getByLabelText('Unverified writes for FTdx10'))

		expect(screen.getByText(/a transfer is running/)).toBeInTheDocument()
		expect(appState.unverifiedGrantsOpen).toBe(true)
		// The click moved the DOM property before the refusal came back, and
		// nothing in the store changed — so the box must be put back rather
		// than left claiming a grant that was never recorded, contradicting
		// its own row's state text.
		expect(screen.getByLabelText('Unverified writes for FTdx10')).not.toBeChecked()
		expect(screen.getByTestId('consent-state-FTdx10').textContent).toBe('Never asked')
	})

	it('a refused REVOCATION likewise leaves the granted radio’s box still ticked', async () => {
		appState.setUnverifiedConsents(ROWS)
		applyMock.mockRejectedValue('userconfig: settings.json is corrupt')
		render(UnverifiedWritesDialog, { mode: 'manage' })

		await fireEvent.click(screen.getByLabelText('Unverified writes for FTDX101D'))

		expect(screen.getByLabelText('Unverified writes for FTDX101D')).toBeChecked()
		expect(screen.getByTestId('consent-state-FTDX101D').textContent).toBe('Enabled')
	})

	it('Close dismisses the panel', async () => {
		appState.openUnverifiedGrants()
		render(UnverifiedWritesDialog, { mode: 'manage' })

		await fireEvent.click(screen.getByRole('button', { name: 'Close' }))

		expect(appState.unverifiedGrantsOpen).toBe(false)
	})
})
