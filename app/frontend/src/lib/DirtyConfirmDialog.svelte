<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
<script>
	// Dirty guard for Open and Read Radio while the working copy has
	// unsaved edits (task-18 brief §2: "dirty guard on Open and on Read
	// Radio while dirty (in-app confirm dialogue: discard / cancel;
	// 'save first' convenience button if cheap)"). Pure presentation +
	// three callbacks — ActionBar owns which action (Open/Read Radio) is
	// pending and what happens on each choice.
	import Modal from './Modal.svelte'

	/** @type {{
	 *   action: 'open' | 'read',
	 *   saving?: boolean,
	 *   oncancel: () => void,
	 *   ondiscard: () => void,
	 *   onsavefirst: () => void,
	 * }} */
	let { action, saving = false, oncancel, ondiscard, onsavefirst } = $props()

	const actionSentence = $derived(
		action === 'open' ? 'Opening a different codeplug' : 'Reading the radio'
	)
</script>

<Modal labelledBy="dirty-guard-title" onclose={oncancel}>
	<div class="modal-header">
		<h2 class="modal-title" id="dirty-guard-title">Unsaved changes</h2>
		<p class="modal-subtitle">
			You have unsaved changes. {actionSentence} will replace the working copy — those changes will
			be lost unless you save first.
		</p>
	</div>
	<div class="modal-footer">
		<button type="button" class="modal-btn" onclick={oncancel}>Cancel</button>
		<button type="button" class="modal-btn modal-btn-danger" onclick={ondiscard}>Discard changes</button>
		<button type="button" class="modal-btn modal-btn-primary" onclick={onsavefirst} disabled={saving}>
			{saving ? 'Saving…' : 'Save first'}
		</button>
	</div>
</Modal>
