<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
<script>
	// The send-to-radio dialogue (task-18 brief §1): one dialogue that
	// MORPHS through three phases rather than three separate modals —
	// 'review' (the SendPlanView PrepareSend returned), 'transferring'
	// (live progress once Confirm is clicked), 'result' (the eventual
	// transfer:done outcome: ok/aborted/refused/cancelled). A fresh
	// instance is mounted per PrepareSend (ActionBar keyed on `plan`), so
	// `plan.ConfirmationDigest` is always THIS dialogue's own token —
	// never cached across dialogues, per the brief's hard constraint.
	//
	// Not closable (Escape/backdrop) while phase is 'transferring' — the
	// brief's "never mid-transfer" — Modal.svelte enforces that from the
	// `closable` prop below.
	import Modal from './Modal.svelte'
	import { appState } from './state/app.svelte.js'
	import { confirmSend, cancelTransfer, readRadio } from './bridge/bindings.js'
	import { phaseLabel, kindLabel } from './transferLabels.js'
	import { hzToMHz } from './grid/freq.js'
	import { describeError } from './errorText.js'

	/** @typedef {import('../../wailsjs/go/models').main.SendPlanView} SendPlanView */
	/** @typedef {import('../../wailsjs/go/models').codeplug.ChannelData} ChannelData */

	/** @type {{ plan: SendPlanView, onClose: () => void, onPrepareAgain: () => void }} */
	let { plan, onClose, onPrepareAgain } = $props()

	/** @type {'review' | 'transferring' | 'result'} */
	let phase = $state('review')
	let firmware = $state('')
	let confirming = $state(false)
	let confirmError = $state('')
	let cancelling = $state(false)

	const diff = $derived(plan.Diff)
	const counts = $derived(diff.Counts)

	/** Every group filters OUT blocked entries — a blocked change is shown
	 * exactly once, in the Blocked group, with its reason, rather than
	 * appearing twice (task-18: "Added / Modified / Erased-unsupported /
	 * Blocked-with-reasons" as four distinct groups; DiffEntryView.Blocked
	 * is a cross-cutting flag on any Kind, not its own Kind — core/
	 * codeplug/diff.go's DiffResult.Blocked doc comment). */
	const addedRows = $derived((diff.Added ?? []).filter((e) => !e.Blocked))
	const modifiedRows = $derived((diff.Modified ?? []).filter((e) => !e.Blocked))
	const erasedRows = $derived((diff.Erased ?? []).filter((e) => !e.Blocked))
	const blockedRows = $derived(
		[...(diff.Added ?? []), ...(diff.Modified ?? []), ...(diff.Erased ?? [])].filter((e) => e.Blocked)
	)

	/** task-25 brief (adjudicated remedy for the reported "i don't seem to
	 * be able to send deletes to the radio" defect): NothingToSend is true
	 * for two very different plans — the working copy genuinely matches
	 * the radio, or every pending change is Blocked (in practice, a
	 * channel delete — the radio has no CAT erase command). ActionBar only
	 * ever opens THIS dialogue for the second case now (the first stays a
	 * toast) — but blockedOnly is derived here too, defensively, so the
	 * dialogue is honest about its own state regardless of how it was
	 * reached. */
	const blockedOnly = $derived(plan.NothingToSend && counts.Blocked > 0)
	const blockedEraseRows = $derived(blockedRows.filter((e) => e.Kind === 'erased'))

	/** @param {ChannelData | null | undefined} data */
	function freqText(data) {
		if (!data) return '(empty)'
		const mode = data.mode ? ` ${data.mode}` : ''
		return `${hzToMHz(data.freq_hz)} MHz${mode}`
	}

	const firmwareMissing = $derived(plan.FirmwareRequired && firmware.trim() === '')

	async function handleConfirm() {
		if (firmwareMissing || confirming) return
		confirmError = ''
		confirming = true
		try {
			await confirmSend(plan.ConfirmationDigest, firmware.trim())
			phase = 'transferring'
		} catch (err) {
			// A synchronous pre-flight rejection (digest mismatch / no
			// active plan / not connected) — the alert strip also carries
			// it, but the strip can sit BEHIND this modal's backdrop, so
			// the dialogue shows it inline too.
			confirmError = describeError(err)
		} finally {
			confirming = false
		}
	}

	async function handleCancelTransfer() {
		cancelling = true
		try {
			await cancelTransfer()
		} catch {
			// Alert strip carries it; the transfer is still running either
			// way — nothing else to do from here.
		}
	}

	async function handleReadRadioNow() {
		try {
			await readRadio()
		} catch {
			// Alert strip already carries the message.
		}
		onClose()
	}

	function handlePrepareAgain() {
		onClose()
		onPrepareAgain()
	}

	// Transition 'transferring' -> 'result' the moment THIS dialogue's own
	// transfer settles (Kind "send" — ReadRadio/DiffAgainstRadio events
	// never arrive while Send's own transfer is running, since the Send
	// button is disabled whenever any transfer is active). Gated on
	// `!appState.transfer.active` as well as Kind "send" — defence in depth
	// alongside beginTransfer clearing lastOutcome on confirm: only
	// applyTransferDone ever clears `active`, so a done that arrives while
	// `active` is still true can never be THIS confirm's own outcome
	// (task-18 review, Critical: a second send must not render the first
	// send's leftover outcome the instant Confirm is clicked).
	$effect(() => {
		const outcome = appState.transfer.lastOutcome
		if (phase === 'transferring' && !appState.transfer.active && outcome?.Kind === 'send') {
			phase = 'result'
		} else if (phase === 'result' && !outcome) {
			// lastOutcome can go null out from under a mounted result screen
			// (e.g. a later beginTransfer, or a disconnect) — there is
			// nothing left to show, so close rather than render an empty
			// result phase (task-18 review, Minor).
			onClose()
		}
	})

	const progress = $derived(appState.transfer.progress)
	const progressPct = $derived(progress.total > 0 ? Math.round((progress.done / progress.total) * 100) : 0)
	const outcome = $derived(appState.transfer.lastOutcome)
</script>

<Modal labelledBy="send-flow-title" closable={phase !== 'transferring'} onclose={onClose}>
	{#if phase === 'review'}
		<div class="modal-header">
			<h2 class="modal-title" id="send-flow-title">Review before sending</h2>
			<p class="modal-subtitle">
				A snapshot of the radio's current state has been saved first, at
				<span class="path-line">{plan.SnapshotPath}</span>. Baseline digest {plan.BaselineDigestShort}…
			</p>
		</div>
		<div class="modal-body">
			<div class="counts-line">
				<span class="count-chip count-chip-added"><strong>{counts.Added}</strong> to add</span>
				<span class="count-chip count-chip-modified"><strong>{counts.Modified}</strong> to modify</span>
				<span class="count-chip"><strong>{counts.Erased}</strong> to erase</span>
				<span class="count-chip count-chip-blocked"><strong>{counts.Blocked}</strong> blocked</span>
			</div>

			{#if plan.NothingToSend}
				{#if blockedOnly}
					<p class="blocked-only-notice">
						None of the pending changes can be sent to this radio — see the reasons below. Your
						edits are still saved in the working copy; the radio itself is unchanged.
					</p>
				{:else}
					<p>Nothing here can be sent — the working copy already matches the radio.</p>
				{/if}
			{/if}

			{#if addedRows.length}
				<section>
					<h3 class="group-title">Added ({addedRows.length})</h3>
					<ul class="entry-list">
						{#each addedRows as e (e.Slot)}
							<li><span class="entry-slot">{e.SlotDisplay}</span> → {freqText(e.After)}</li>
						{/each}
					</ul>
				</section>
			{/if}

			{#if modifiedRows.length}
				<section>
					<h3 class="group-title">Modified ({modifiedRows.length})</h3>
					<ul class="entry-list">
						{#each modifiedRows as e (e.Slot)}
							<li><span class="entry-slot">{e.SlotDisplay}</span> {freqText(e.Before)} → {freqText(e.After)}</li>
						{/each}
					</ul>
				</section>
			{/if}

			{#if erasedRows.length}
				<section>
					<h3 class="group-title">Erased ({erasedRows.length})</h3>
					<ul class="entry-list">
						{#each erasedRows as e (e.Slot)}
							<li><span class="entry-slot">{e.SlotDisplay}</span> {freqText(e.Before)} → erase</li>
						{/each}
					</ul>
				</section>
			{/if}

			{#if blockedRows.length}
				<section>
					<h3 class="group-title group-title-blocked">Blocked ({blockedRows.length})</h3>
					<ul class="entry-list">
						{#each blockedRows as e (e.Slot)}
							<li><span class="entry-slot">{e.SlotDisplay}</span> ({e.Kind}) — {e.BlockReason}</li>
						{/each}
					</ul>
					{#if blockedEraseRows.length && appState.uiSpec?.EraseDialogNote}
						<p class="erase-procedure">{appState.uiSpec.EraseDialogNote}</p>
					{/if}
				</section>
			{/if}

			{#if plan.FirmwareRequired}
				<div class="field-group">
					<label for="firmware-input">Confirmed firmware version</label>
					<input id="firmware-input" type="text" bind:value={firmware} placeholder={appState.uiSpec?.FirmwarePlaceholder ?? ''} />
					<p class="modal-subtitle">{plan.FirmwareGuidance}</p>
				</div>
			{/if}

			{#if confirmError}
				<p class="confirm-error">{confirmError}</p>
			{/if}
		</div>
		<div class="modal-footer">
			{#if blockedOnly}
				<!-- task-25 brief: a blocked-only plan is purely informational
				     — nothing here CAN be sent, so a disabled Confirm button
				     (implying it might become enabled) would be dishonest.
				     Close is the only affordance. -->
				<button type="button" class="modal-btn modal-btn-primary" onclick={onClose}>Close</button>
			{:else}
				<button type="button" class="modal-btn" onclick={onClose}>Cancel</button>
				<button
					type="button"
					class="modal-btn modal-btn-primary"
					disabled={plan.NothingToSend || firmwareMissing || confirming}
					onclick={handleConfirm}
				>
					{confirming ? 'Confirming…' : 'Confirm send'}
				</button>
			{/if}
		</div>
	{:else if phase === 'transferring'}
		<div class="modal-header">
			<h2 class="modal-title" id="send-flow-title">Sending to radio…</h2>
		</div>
		<div class="modal-body">
			<p class="transfer-line">
				{kindLabel(appState.transfer.kind)}: {phaseLabel(progress.phase)}
				{#if progress.total > 0}{progress.done}/{progress.total}{/if}
				{#if progress.slot}<span class="entry-slot">{progress.slot}</span>{/if}
			</p>
			<span class="progress-track" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow={progressPct}>
				<span class="progress-fill" style:width={`${progressPct}%`}></span>
			</span>
		</div>
		<div class="modal-footer">
			<span class="modal-footer-note">Stops at the next verified channel boundary.</span>
			<button type="button" class="modal-btn modal-btn-danger" disabled={cancelling} onclick={handleCancelTransfer}>
				{cancelling ? 'Cancelling…' : 'Cancel transfer'}
			</button>
		</div>
	{:else if outcome}
		<div class="modal-header">
			<h2 class="modal-title" id="send-flow-title">
				{#if outcome.Outcome === 'ok'}Send complete
				{:else if outcome.Outcome === 'aborted'}Send aborted
				{:else if outcome.Outcome === 'refused'}Send refused
				{:else if outcome.Outcome === 'cancelled'}Send cancelled
				{:else}Send failed
				{/if}
			</h2>
		</div>
		<div class="modal-body">
			{#if outcome.Outcome === 'ok'}
				{@const r = outcome.Report}
				{#if r}
					<div class="counts-line">
						<span class="count-chip count-chip-added"><strong>{r.Written}</strong> written</span>
						<span class="count-chip"><strong>{r.Verified}</strong> verified</span>
						<span class="count-chip"><strong>{r.SkippedBlocked}</strong> skipped (blocked)</span>
						<span class="count-chip"><strong>{r.Unchanged}</strong> unchanged</span>
					</div>
					<p class="path-line">Snapshot: {r.SnapshotPath}</p>
					<p class="path-line">Journal: {r.JournalPath}</p>
				{/if}
				<p>The send succeeded. Re-read the radio to refresh the baseline before comparing or sending again.</p>
			{:else if outcome.Outcome === 'aborted'}
				{@const r = outcome.Report}
				<p>{outcome.Message}</p>
				{#if r}
					<table class="report-table">
						<thead>
							<tr><th scope="col">Slot</th><th scope="col">Action</th><th scope="col">Verified</th><th scope="col">Detail</th></tr>
						</thead>
						<tbody>
							{#each r.Slots as s (s.Slot)}
								<tr><td>{s.SlotDisplay}</td><td>{s.Action}</td><td>{s.VerifyOK ? 'yes' : 'no'}</td><td>{s.Detail}</td></tr>
							{/each}
						</tbody>
					</table>
					<p class="path-line">Snapshot: {r.SnapshotPath}</p>
					<p class="path-line">Journal: {r.JournalPath}</p>
					<p class="recovery-note">
						A snapshot of the radio's contents immediately before this send is saved at the path
						above, alongside a journal of exactly what happened. If you need to recover, load the
						snapshot to see what the radio held before this send — but note that erased or hidden
						fields cannot be restored to the radio over CAT; recovering those requires manual entry
						from the front panel.
					</p>
				{/if}
			{:else if outcome.Outcome === 'refused'}
				<p>{outcome.Message}</p>
			{:else if outcome.Outcome === 'cancelled'}
				{@const r = outcome.Report}
				<p>{outcome.Message}</p>
				{#if r}
					<div class="counts-line">
						<span class="count-chip count-chip-added"><strong>{r.Written}</strong> written</span>
						<span class="count-chip"><strong>{r.Verified}</strong> verified</span>
					</div>
					<p class="path-line">Snapshot: {r.SnapshotPath}</p>
					<p class="path-line">Journal: {r.JournalPath}</p>
					<p class="recovery-note">
						A snapshot of the radio's contents immediately before this send is saved at the path
						above, alongside a journal of exactly what happened. If you need to recover, load the
						snapshot to see what the radio held before this send — but note that erased or hidden
						fields cannot be restored to the radio over CAT; recovering those requires manual entry
						from the front panel.
					</p>
				{/if}
			{:else}
				<p>{outcome.Message || 'Something went wrong sending to the radio.'}</p>
			{/if}
		</div>
		<div class="modal-footer">
			{#if outcome.Outcome !== 'ok' && outcome.Report}
				<button type="button" class="modal-btn" onclick={handleReadRadioNow}>Read radio now</button>
			{/if}
			{#if outcome.Outcome === 'ok'}
				<button type="button" class="modal-btn modal-btn-primary" onclick={handleReadRadioNow}>Read radio now</button>
				<button type="button" class="modal-btn" onclick={onClose}>Close</button>
			{:else}
				<button type="button" class="modal-btn modal-btn-primary" onclick={handlePrepareAgain}>Prepare again</button>
				<button type="button" class="modal-btn" onclick={onClose}>Close</button>
			{/if}
		</div>
	{/if}
</Modal>

<style>
	.counts-line {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-2);
	}

	.group-title {
		margin: 0 0 var(--space-1);
		font-size: 11px;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: var(--colour-text-dim);
	}

	.group-title-blocked {
		color: var(--colour-danger);
	}

	.blocked-only-notice {
		font-weight: 600;
	}

	.erase-procedure {
		margin-top: var(--space-2);
		color: var(--colour-text-dim);
		font-size: 12px;
	}

	.entry-list {
		list-style: none;
		margin: 0;
		padding: 0;
		font-size: 12.5px;
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.entry-list li {
		padding: 2px 0;
	}

	.entry-slot {
		font-family: var(--font-mono);
		color: var(--colour-accent-strong);
		margin-right: var(--space-1);
	}

	.field-group {
		display: flex;
		flex-direction: column;
		gap: var(--space-1);
	}

	.field-group label {
		font-size: 11.5px;
		font-weight: 600;
	}

	.field-group input {
		background: var(--colour-panel-sunken);
		border: 1px solid var(--colour-hairline);
		border-radius: var(--radius-sm);
		padding: var(--space-2) var(--space-3);
		font-family: var(--font-mono);
		max-width: 12rem;
	}

	.confirm-error {
		color: var(--colour-danger);
	}

	.transfer-line {
		font-family: var(--font-mono);
		font-size: 13px;
	}

	.progress-track {
		display: block;
		width: 100%;
		height: 8px;
		border-radius: 4px;
		background: var(--colour-panel-sunken);
		overflow: hidden;
	}

	.progress-fill {
		display: block;
		height: 100%;
		background: var(--colour-accent);
		transition: width var(--transition-fast);
	}

	@media (prefers-reduced-motion: reduce) {
		.progress-fill {
			transition: none;
		}
	}

	.recovery-note {
		color: var(--colour-text-dim);
		font-size: 12px;
	}
</style>
