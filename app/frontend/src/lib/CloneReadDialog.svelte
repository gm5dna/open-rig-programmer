<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
<script>
	// Clone-mode GUI read (Phase 4b of the clone-mode READ milestone) —
	// the GUI equivalent of `rigprog read --clone` (cmd/rigprog/cloneread.go).
	// A modal driven by its own small state machine, mirroring
	// SendFlowDialog's morph-through-phases shape:
	//
	//   pick (model + port) -> arming (spinner) -> prompt (only once
	//   "clone:armed" has fired) -> receiving (progress) -> done | refused
	//
	// This dialogue's own model dropdown is fed ONLY by getCloneModels() —
	// a SEPARATE list from the ordinary connection bar's model picker,
	// never merged with it (internal/wiring's disjoint-registry proof) —
	// and READ-ONLY by construction: there is no send/write button
	// anywhere in this file, on any phase (spec.md's clone-mode ruling:
	// Write is Unsupported for every registered clone model).
	import Modal from './Modal.svelte'
	import { appState } from './state/app.svelte.js'
	import {
		getCloneModels,
		armCloneRead,
		cancelCloneRead,
		receiveCloneImage,
		onCloneArmed,
		listPorts,
	} from './bridge/bindings.js'
	import { describeError } from './errorText.js'

	/** @type {{ onClose: () => void }} */
	let { onClose } = $props()

	/** @type {'pick' | 'arming' | 'prompt' | 'receiving' | 'done' | 'refused'} */
	let phase = $state('pick')

	/** @type {string[]} */
	let models = $state([])
	let model = $state('')
	let portPath = $state('')
	let armError = $state('')
	let refusalMessage = $state('')
	let armed = $state(false)

	let unsubscribeArmed = () => {}

	$effect(() => {
		listPorts()
		getCloneModels().then((list) => {
			models = list ?? []
			if (!model && models.length) model = models[0]
		})
		unsubscribeArmed = onCloneArmed(() => {
			armed = true
			phase = 'prompt'
		})
		return () => unsubscribeArmed()
	})

	const progress = $derived(appState.transfer.progress)
	const outcome = $derived(appState.transfer.lastOutcome)

	/** @param {Event & {currentTarget: HTMLSelectElement}} e */
	function onModelChange(e) {
		model = e.currentTarget.value ?? ''
	}

	/** @param {Event & {currentTarget: HTMLSelectElement}} e */
	function onPortChange(e) {
		portPath = e.currentTarget.value ?? ''
	}

	async function handleArm() {
		if (!model || !portPath) return
		armError = ''
		armed = false
		phase = 'arming'
		try {
			await armCloneRead(portPath, model)
			// "clone:armed" (onCloneArmed above) is what actually advances
			// to 'prompt' — spec.md §Read model point 1: arm-before-prompt
			// is a definite ordering, so this phase transition is driven by
			// the EVENT, never by armCloneRead's own promise settling.
		} catch (err) {
			armError = describeError(err)
			phase = 'pick'
		}
	}

	async function handleReceive() {
		phase = 'receiving'
		refusalMessage = ''
		try {
			await receiveCloneImage()
			phase = 'done'
		} catch (err) {
			refusalMessage = describeError(err)
			phase = 'refused'
		}
	}

	/** Closing at pick/arming/prompt abandons an armed read (if any) so the
	 * App-level busy reservation is released rather than stuck for the
	 * rest of the session (app/clone.go's CancelCloneRead). */
	async function handleClose() {
		if (phase === 'arming' || phase === 'prompt') {
			try {
				await cancelCloneRead()
			} catch {
				// Nothing to release, or already gone — either way, close.
			}
		}
		onClose()
	}

	function handleTryAgain() {
		phase = 'pick'
		armError = ''
		refusalMessage = ''
	}
</script>

<Modal labelledBy="clone-read-title" closable={phase !== 'receiving'} onclose={handleClose}>
	{#if phase === 'pick'}
		<div class="modal-header">
			<h2 class="modal-title" id="clone-read-title">Clone-mode read</h2>
			<p class="modal-subtitle">
				Whole-image read for radios with no CAT memory protocol — the FT-817/857/897 families.
				This is a read-only transfer: nothing is ever written back to the radio.
			</p>
		</div>
		<div class="modal-body">
			<label class="field-label" for="clone-model">Model</label>
			<select id="clone-model" value={model} onchange={onModelChange}>
				{#each models as m (m)}
					<option value={m}>{m}</option>
				{/each}
			</select>
			<p class="field-note">
				The image cannot confirm this against a same-length sibling (e.g. FT-857 vs FT-857D) —
				this is what you select, not what the transfer proves.
			</p>

			<label class="field-label" for="clone-port">Port</label>
			<select id="clone-port" value={portPath} onchange={onPortChange}>
				<option value="">Choose a port…</option>
				{#each appState.ports as p (p.Path)}
					<option value={p.Path}>{p.Description ? `${p.Path} — ${p.Description}` : p.Path}</option>
				{/each}
			</select>

			{#if armError}
				<p class="confirm-error">{armError}</p>
			{/if}
		</div>
		<div class="modal-footer">
			<button type="button" class="modal-btn" onclick={handleClose}>Cancel</button>
			<button type="button" class="modal-btn modal-btn-primary" disabled={!model || !portPath} onclick={handleArm}>
				Arm
			</button>
		</div>
	{:else if phase === 'arming'}
		<div class="modal-header">
			<h2 class="modal-title" id="clone-read-title">Arming…</h2>
		</div>
		<div class="modal-body">
			<p>Opening the port and starting the deadline clocks.</p>
		</div>
		<div class="modal-footer">
			<button type="button" class="modal-btn" onclick={handleClose}>Cancel</button>
		</div>
	{:else if phase === 'prompt'}
		<div class="modal-header">
			<h2 class="modal-title" id="clone-read-title">Ready to receive</h2>
		</div>
		<div class="modal-body">
			<p class="prompt-line">Put the radio into clone-send mode now, then press SEND.</p>
			<p class="field-note">
				Model as selected: {model}. This is operator-asserted, not wire-confirmed — the image
				cannot distinguish this model from a same-length sibling.
			</p>
		</div>
		<div class="modal-footer">
			<button type="button" class="modal-btn" onclick={handleClose}>Cancel</button>
			<button type="button" class="modal-btn modal-btn-primary" onclick={handleReceive}>
				Receiving…
			</button>
		</div>
	{:else if phase === 'receiving'}
		<div class="modal-header">
			<h2 class="modal-title" id="clone-read-title">Receiving…</h2>
		</div>
		<div class="modal-body">
			<p class="transfer-line">{progress.phase || 'clone-receive'}</p>
			<span class="progress-track" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow="0">
				<span class="progress-fill indeterminate"></span>
			</span>
		</div>
		<div class="modal-footer">
			<span class="modal-footer-note">Waiting for the whole image — this can take a while.</span>
		</div>
	{:else if phase === 'done'}
		<div class="modal-header">
			<h2 class="modal-title" id="clone-read-title">Clone-mode read complete</h2>
		</div>
		<div class="modal-body">
			<p>
				The image has been loaded as the working copy for <strong>{model}</strong> (operator-asserted,
				not wire-confirmed). Save it to keep a copy on disk.
			</p>
		</div>
		<div class="modal-footer">
			<button type="button" class="modal-btn modal-btn-primary" onclick={onClose}>Close</button>
		</div>
	{:else if phase === 'refused'}
		<div class="modal-header">
			<h2 class="modal-title" id="clone-read-title">Read refused</h2>
		</div>
		<div class="modal-body">
			<p>{refusalMessage || outcome?.Message || 'The clone-mode image could not be read.'}</p>
		</div>
		<div class="modal-footer">
			<button type="button" class="modal-btn modal-btn-primary" onclick={handleTryAgain}>Try again</button>
			<button type="button" class="modal-btn" onclick={onClose}>Close</button>
		</div>
	{/if}
</Modal>

<style>
	.field-label {
		display: block;
		margin-top: var(--space-2);
		margin-bottom: var(--space-1);
		font-size: 11px;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: var(--colour-text-dim);
	}

	.field-note {
		margin-top: var(--space-1);
		color: var(--colour-text-dim);
		font-size: 12px;
	}

	.prompt-line {
		font-weight: 600;
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

	.progress-fill.indeterminate {
		display: block;
		height: 100%;
		width: 40%;
		background: var(--colour-accent);
		animation: clone-indeterminate 1.2s ease-in-out infinite;
	}

	@keyframes clone-indeterminate {
		0% {
			margin-left: 0%;
		}
		50% {
			margin-left: 60%;
		}
		100% {
			margin-left: 0%;
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.progress-fill.indeterminate {
			animation: none;
		}
	}
</style>
