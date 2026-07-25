<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
<script>
	// The drag-drop / keyboard "Copy or move" popover (task-22 brief §2).
	// Opened two ways, both landing here with the SAME three choices:
	//   - a drop on another row already carries a resolved targetSlot;
	//   - the per-row "Copy/Move to…" keyboard path opens with
	//     targetSlot === null and shows a slot-entry field first
	//     (validated against the CURRENT bank's own slots — resolveTargetSlot,
	//     grid/dragDrop.js — cross-tab is out of scope for v1).
	// Swap is only offered when the target is ALREADY populated: swapping
	// with an empty slot would silently erase the source exactly like Move
	// while claiming "no erase involved" (this component's own honesty
	// rule — see dragDrop.js's module comment on why swap must not lose
	// either channel). Pure presentation: ChannelGrid resolves the fresh
	// channel data and commits via updateChannels on onchoose.
	import Modal from './Modal.svelte'
	import { resolveTargetSlot } from './grid/dragDrop.js'

	/** @typedef {import('../../wailsjs/go/models').main.BankView} BankView */
	/** @typedef {import('../../wailsjs/go/models').codeplug.Channel} Channel */

	/** @type {{
	 *   bank: BankView,
	 *   channelBySlot: Map<string, Channel>,
	 *   sourceSlot: string,
	 *   sourceDisplay: string,
	 *   targetSlot: string | null,
	 *   onchoose: (action: 'copy'|'swap'|'move', targetSlot: string) => void,
	 *   oncancel: () => void,
	 * }} */
	let { bank, channelBySlot, sourceSlot, sourceDisplay, targetSlot, onchoose, oncancel } = $props()

	const needsEntry = $derived(targetSlot === null)
	let targetText = $state('')
	let entryError = $state('')
	/** @type {string | null} */
	let typedTarget = $state(null)

	function onEntryInput() {
		if (targetText.trim() === '') {
			typedTarget = null
			entryError = ''
			return
		}
		const result = resolveTargetSlot(bank, targetText, sourceSlot)
		if (!result.ok) {
			typedTarget = null
			entryError = result.reason
			return
		}
		entryError = ''
		typedTarget = result.slot
	}

	const resolvedTarget = $derived(targetSlot ?? typedTarget)
	const targetDisplay = $derived(
		resolvedTarget ? ((bank.Slots ?? []).find((sv) => sv.Slot === resolvedTarget)?.Display ?? resolvedTarget) : ''
	)
	const targetHasData = $derived(resolvedTarget ? channelBySlot.get(resolvedTarget)?.data != null : false)
</script>

<Modal labelledBy="move-copy-title" onclose={oncancel}>
	<div class="modal-header">
		<h2 class="modal-title" id="move-copy-title">Copy or move {sourceDisplay}</h2>
		{#if resolvedTarget}
			<p class="modal-subtitle">
				{sourceDisplay} → {targetDisplay}{targetHasData ? ` (currently holds a channel)` : ' (currently empty)'}
			</p>
		{/if}
	</div>
	<div class="modal-body">
		{#if needsEntry}
			<label class="field-label" for="move-copy-target-entry">Target slot</label>
			<input
				id="move-copy-target-entry"
				class="modal-input"
				type="text"
				placeholder="e.g. M-50"
				autocomplete="off"
				bind:value={targetText}
				oninput={onEntryInput}
			/>
			{#if entryError}<p class="field-error">{entryError}</p>{/if}
		{/if}
		{#if resolvedTarget}
			<p class="modal-subtitle">
				Move here: {sourceDisplay} becomes empty in this file — the radio keeps its old contents until
				front-panel deletion.
			</p>
		{/if}
	</div>
	<div class="modal-footer">
		<button type="button" class="modal-btn" onclick={oncancel}>Cancel</button>
		{#if resolvedTarget}
			<button type="button" class="modal-btn" onclick={() => onchoose('copy', /** @type {string} */ (resolvedTarget))}>
				Copy here{targetHasData ? ' (overwrites it)' : ''}
			</button>
			{#if targetHasData}
				<button type="button" class="modal-btn" onclick={() => onchoose('swap', /** @type {string} */ (resolvedTarget))}>
					Swap
				</button>
			{/if}
			<button
				type="button"
				class="modal-btn modal-btn-danger"
				onclick={() => onchoose('move', /** @type {string} */ (resolvedTarget))}
			>
				Move here
			</button>
		{/if}
	</div>
</Modal>

<style>
	.field-label {
		display: block;
		font-size: 11.5px;
		color: var(--colour-text-dim);
		margin-bottom: var(--space-1);
	}

	.modal-input {
		width: 100%;
		background: var(--colour-bg);
		color: var(--colour-text);
		border: 1px solid var(--colour-hairline);
		border-radius: var(--radius-sm);
		padding: var(--space-2);
		font-family: var(--font-mono);
	}

	.modal-input:focus {
		outline: none;
		border-color: var(--colour-accent);
	}

	.field-error {
		margin: var(--space-1) 0 0;
		font-size: 11.5px;
		color: var(--colour-danger);
	}
</style>
