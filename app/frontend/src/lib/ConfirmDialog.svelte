<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
<script>
	// Shared shape behind DeleteConfirmDialog/RowMoveConfirmDialog/
	// DirtyConfirmDialog (ponytail audit 2026-09-06, finding 44): all three
	// were Modal + a title + one or two paragraphs + Cancel and a
	// destructive confirm button, differing only in copy. `extraLabel` is
	// the one addition beyond that shared shape — DirtyConfirmDialog's
	// "Save first" convenience button (task-18 brief) — rendered only when
	// given, so the other two call sites stay a plain title/body/
	// confirmLabel use.
	import Modal from './Modal.svelte'

	/** @type {{
	 *   labelledBy: string,
	 *   title: string,
	 *   body: string,
	 *   note?: string,
	 *   confirmLabel: string,
	 *   onconfirm: () => void,
	 *   oncancel: () => void,
	 *   danger?: boolean,
	 *   extraLabel?: string,
	 *   onextra?: () => void,
	 *   extraDisabled?: boolean,
	 * }} */
	let {
		labelledBy,
		title,
		body,
		note = '',
		confirmLabel,
		onconfirm,
		oncancel,
		danger = true,
		extraLabel = '',
		onextra = () => {},
		extraDisabled = false,
	} = $props()
</script>

<Modal {labelledBy} onclose={oncancel}>
	<div class="modal-header">
		<h2 class="modal-title" id={labelledBy}>{title}</h2>
		<p class="modal-subtitle">{body}</p>
		{#if note}
			<p class="modal-subtitle">{note}</p>
		{/if}
	</div>
	<div class="modal-footer">
		<button type="button" class="modal-btn" onclick={oncancel}>Cancel</button>
		<button type="button" class="modal-btn" class:modal-btn-danger={danger} onclick={onconfirm}>
			{confirmLabel}
		</button>
		{#if extraLabel}
			<button type="button" class="modal-btn modal-btn-primary" onclick={onextra} disabled={extraDisabled}>
				{extraLabel}
			</button>
		{/if}
	</div>
</Modal>
