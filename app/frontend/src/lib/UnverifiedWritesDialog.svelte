<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
<script>
	// The unverified-write consent dialogue (task 14, M9d), in TWO MODES —
	// one component rather than two, because they are the same question
	// asked twice over the same data (an UnverifiedWriteConsentView), with
	// the same guards, the same bridge call and the same inline-error
	// handling; splitting them would duplicate all four to vary a heading
	// and a row count:
	//
	//   - 'arm'    — the question asked ONCE per radio, raised by
	//                bindings.js after a real connect to a consent-eligible
	//                radio with no decision recorded. Two answers, both of
	//                which record a decision: "Enable unverified writes"
	//                and "Not now" (a decline is a decision, so it is never
	//                re-asked).
	//   - 'manage' — the always-reachable grants panel: every model this
	//                build supports, hardware-verified ones included (shown
	//                "n/a", toggle disabled — there is no unverified write
	//                on those for a consent to unlock), each eligible one
	//                grantable or revocable at any time.
	//
	// The BODY of the arming mode is Go's own `Warning`, rendered verbatim.
	// Nothing here re-words a hardware claim: which radios this project has
	// written to, and what a read-back does and does not protect, is stated
	// in exactly one place (internal/radiotext), so it can only ever be
	// wrong in one place. The ONE piece of frontend prose above it (the
	// arming subtitle) is a pointer to that paragraph, deliberately making
	// no claim the paragraph does not already make; if the warning's own
	// wording ever changes, that subtitle is the line to re-read.
	//
	// The reconnect a change may require is NOT orchestrated here — see
	// bindings.js's applyUnverifiedWriteConsent, which disconnects first and
	// rejects (persisting nothing) if the session refuses to close. This
	// dialogue reads that contract off the promise: a rejection leaves it
	// open with the reason inline, a resolution closes it.
	import Modal from './Modal.svelte'
	import { appState } from './state/app.svelte.js'
	import { applyUnverifiedWriteConsent, refreshUnverifiedConsents } from './bridge/bindings.js'
	import { describeError } from './errorText.js'

	/** @typedef {import('../../wailsjs/go/models').main.UnverifiedWriteConsentView} UnverifiedWriteConsentView */

	/** @type {{ mode: 'arm' | 'manage' }} */
	let { mode } = $props()

	/** The model whose decision is currently in flight, or '' when idle —
	 * a consent change can take a disconnect/reconnect round trip, so every
	 * control is held while one is running. */
	let pending = $state('')
	/** WHICH answer is in flight, so only the button the user actually
	 * pressed relabels itself: `pending` alone put "Enabling…" on the
	 * primary button even when the user had pressed "Not now". */
	let pendingOn = $state(false)
	/** A refusal, shown INLINE: this modal's backdrop can sit above the
	 * alert strip, so the strip alone cannot be relied on (the same reason
	 * SendFlowDialog shows its pre-flight rejections inline). */
	let error = $state('')

	const prompt = $derived(appState.unverifiedConsentPrompt)
	const blockedReason = $derived(appState.consentChangeBlockedReason)
	const locked = $derived(!appState.canChangeUnverifiedConsent || pending !== '')

	// The store is shared with the CLI, so it can change behind a running
	// app: the panel always reads it afresh rather than trusting whatever
	// the last refresh left behind. Never throws (bindings.js).
	$effect(() => {
		if (mode === 'manage') void refreshUnverifiedConsents()
	})

	/** Records one decision. `toggle` is the checkbox that triggered it, when
	 * a checkbox did: the grants panel's boxes are rendered from state
	 * (`checked={row.Granted}`) but are NOT bound to it, so a browser's own
	 * click has already moved the DOM property by the time this runs. On a
	 * REJECTION nothing was persisted, `row.Granted` is therefore unchanged,
	 * and Svelte's update skips a DOM write it sees no reason for — leaving
	 * a ticked "unverified writes" box for a radio with no recorded grant,
	 * contradicting the state text beside it. So the property is restored by
	 * hand. `!on` IS the pre-click value by construction (the caller passes
	 * `!row.Granted`).
	 * @param {string} model
	 * @param {boolean} on
	 * @param {HTMLInputElement} [toggle] */
	async function record(model, on, toggle) {
		if (locked) return
		error = ''
		pending = model
		pendingOn = on
		try {
			// `sessionUnconsented` in 'arm' mode is knowledge this call site
			// holds BY CONSTRUCTION, not a guess: the arming dialogue is raised
			// only for a connection whose UnverifiedConsentRecorded is false
			// (bindings.js's raiseConsentPromptIfDue, via
			// appState.unverifiedConsentDue), so the session behind it was
			// opened with no consent to spend. The bridge uses it for one
			// thing — letting a DECLINE persist without re-opening a session
			// that is already declining every unverified write. The grants
			// panel knows nothing of the kind and says nothing.
			await applyUnverifiedWriteConsent(model, on, { sessionUnconsented: mode === 'arm' })
			// Resolved => the decision IS recorded (see the bridge's contract).
			if (mode === 'arm') appState.setUnverifiedConsentPrompt(null)
		} catch (err) {
			// Rejected => nothing was persisted; stay open, say why, and put
			// the checkbox back where the store still says it belongs.
			error = describeError(err)
			if (toggle) toggle.checked = !on
		} finally {
			pending = ''
		}
	}

	/** @param {UnverifiedWriteConsentView} row */
	function stateText(row) {
		if (!row.NeedsConsent) return 'n/a — this radio’s writes are hardware-verified'
		if (row.Granted) return 'Enabled'
		return row.Recorded ? 'Not enabled' : 'Never asked'
	}

	function close() {
		appState.closeUnverifiedGrants()
	}
</script>

{#if mode === 'arm' && prompt}
	<!-- Not closable: the question has two answers, and BOTH record a
	     decision. An Escape that recorded nothing would simply re-ask at
	     the next connection, which is the nagging this dialogue exists to
	     avoid. It cannot strand the user either: the modal backdrop covers
	     the whole app, so no transfer and no send dialogue can start behind
	     it — the guards that would hold both answers disabled are
	     unreachable while it is open. -->
	<Modal labelledBy="unverified-writes-title" closable={false}>
		<div class="modal-header">
			<h2 class="modal-title" id="unverified-writes-title">Unverified writes — {prompt.Model}</h2>
			<p class="modal-subtitle">This project has not proven writing to this radio on real hardware.</p>
		</div>
		<div class="modal-body">
			<p class="warning-body" data-testid="unverified-warning">{prompt.Warning}</p>
			{#if blockedReason}
				<p class="blocked-note" data-testid="consent-blocked-reason">{blockedReason}</p>
			{/if}
			{#if error}
				<p class="consent-error">{error}</p>
			{/if}
		</div>
		<div class="modal-footer">
			<button type="button" class="modal-btn" disabled={locked} onclick={() => record(prompt.Model, false)}>
				Not now
			</button>
			<button
				type="button"
				class="modal-btn modal-btn-primary"
				disabled={locked}
				onclick={() => record(prompt.Model, true)}
			>
				{pending && pendingOn ? 'Enabling…' : 'Enable unverified writes'}
			</button>
		</div>
	</Modal>
{:else if mode === 'manage'}
	<Modal labelledBy="unverified-writes-title" onclose={close}>
		<div class="modal-header">
			<h2 class="modal-title" id="unverified-writes-title">Unverified writes</h2>
			<p class="modal-subtitle">
				Enabling unverified writes applies from the next connection to that radio. Changing it for the
				radio you are connected to re-opens the session; your working copy is kept, and its baseline is
				marked stale until you read the radio again.
			</p>
		</div>
		<div class="modal-body">
			{#if blockedReason}
				<p class="blocked-note" data-testid="consent-blocked-reason">{blockedReason}</p>
			{/if}
			{#if error}
				<p class="consent-error">{error}</p>
			{/if}
			<ul class="grant-list">
				{#each appState.unverifiedConsents as row (row.Model)}
					<li class="grant-row">
						<span class="grant-model">{row.Model}</span>
						<span class="grant-state" data-testid={`consent-state-${row.Model}`}>{stateText(row)}</span>
						<input
							type="checkbox"
							class="grant-toggle"
							aria-label={`Unverified writes for ${row.Model}`}
							checked={row.Granted}
							disabled={!row.NeedsConsent || locked}
							title={!row.NeedsConsent
								? 'This radio’s writes are hardware-verified — there is nothing to enable'
								: blockedReason}
							onchange={(e) => record(row.Model, !row.Granted, e.currentTarget)}
						/>
					</li>
				{/each}
			</ul>
		</div>
		<div class="modal-footer">
			<button type="button" class="modal-btn modal-btn-primary" onclick={close}>Close</button>
		</div>
	</Modal>
{/if}

<style>
	/* The arming warning is the one paragraph in this app that must be read
	 * before it is dismissed — amber-keyed like every other caution in the
	 * interface (AlertStrip's idiom: a tinted ground and a left rule). */
	.warning-body {
		padding: var(--space-3);
		background: var(--colour-warn-bg);
		border-left: 3px solid var(--colour-warn);
		border-radius: var(--radius-sm);
	}

	.blocked-note {
		color: var(--colour-text-dim);
	}

	.consent-error {
		color: var(--colour-danger);
	}

	.grant-list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
	}

	.grant-row {
		display: grid;
		grid-template-columns: 8rem 1fr auto;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-2) 0;
		border-bottom: 1px solid var(--colour-hairline-soft);
	}

	.grant-model {
		font-family: var(--font-mono);
		color: var(--colour-text);
	}

	.grant-state {
		color: var(--colour-text-dim);
	}

	.grant-toggle:disabled {
		opacity: 0.45;
		cursor: not-allowed;
	}
</style>
