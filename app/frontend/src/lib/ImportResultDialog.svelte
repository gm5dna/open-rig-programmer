<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
<script>
	// Import CSV/CHIRP result dialogue (task-18 brief §2): on refusal
	// (blocking loss / unknown slots / duplicates / inventory mismatch)
	// shows the report with NOTHING merged — Go guarantees that; this
	// renders it honestly, never implying a partial merge happened. On a
	// successful merge with non-blocking CHIRP loss entries, shows them
	// in this same dismissible panel (CSV's LossEntries is always empty —
	// core/csvio's contract — so that branch only ever fires for CHIRP in
	// practice, but the component does not special-case the format).
	import Modal from './Modal.svelte'

	/** @typedef {import('../../wailsjs/go/models').main.ImportResultView} ImportResultView */

	/** @type {{ format: 'CSV' | 'CHIRP', result: ImportResultView, onclose: () => void }} */
	let { format, result, onclose } = $props()

	const refused = $derived(!result.Merged)
	const cause = $derived(result.ParseError || result.RefusalReason || '')
</script>

<Modal labelledBy="import-result-title" onclose={onclose}>
	<div class="modal-header">
		<h2 class="modal-title" id="import-result-title">
			{refused ? `${format} import refused` : `${format} import complete`}
		</h2>
		<p class="modal-subtitle">
			{result.Path}
			{#if refused}
				— nothing was merged.
			{:else}
				— merged into the working copy.
			{/if}
		</p>
	</div>
	<div class="modal-body">
		{#if refused && cause}
			<p>{cause}</p>
		{/if}
		{#if result.LossEntries?.length}
			<div>
				<p class="modal-subtitle">
					{refused
						? 'The loss entries below are why the import was refused:'
						: 'The following fields could not be carried over exactly and were adjusted or dropped:'}
				</p>
				<table class="report-table">
					<thead>
						<tr>
							<th scope="col">Line</th>
							<th scope="col">Column</th>
							<th scope="col">Value</th>
							<th scope="col">Action</th>
							<th scope="col">Detail</th>
						</tr>
					</thead>
					<tbody>
						{#each result.LossEntries as entry (entry.Line + entry.Column + entry.Value)}
							<tr class:blocking-row={entry.Blocking}>
								<td>{entry.Line}</td>
								<td>{entry.Column}</td>
								<td>{entry.Value}</td>
								<td>{entry.Action}{entry.Blocking ? ' (blocking)' : ''}</td>
								<td>{entry.Detail}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}
	</div>
	<div class="modal-footer">
		<button type="button" class="modal-btn modal-btn-primary" onclick={onclose}>Dismiss</button>
	</div>
</Modal>

<style>
	.blocking-row td {
		color: var(--colour-danger);
	}
</style>
