<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
<script>
	// Channel-delete confirmation (task-22 brief §1). File-level only: no
	// CAT erase exists (docs/hardware-notes.md's "No CAT erase" — HW-
	// CONFIRMED 2026-07-13, re-probed non-confounded the same day), so the
	// honest copy never promises the radio itself is touched — only that
	// the working COPY becomes empty, and that sending it later will show
	// up as an unsupported erase until the channel is cleared from the
	// front panel. The front-panel procedure itself (task-25 brief: this
	// is the moment the user forms the expectation, so it belongs here too,
	// not just in the send-review dialogue) is the `eraseNote` prop — task
	// 42 (M9a-6): served from GetUISpec's EraseDialogNote (internal/
	// radiotext.Text.EraseDialogNote, sourced from the FT-710 operation
	// manual p.62 — see that package's doc comment), passed down by
	// ChannelGrid rather than hardcoded here. Pure presentation, like
	// DirtyConfirmDialog — ChannelGrid owns what happens on confirm.
	import Modal from './Modal.svelte'

	/** @type {{
	 *   slotDisplay: string,
	 *   eraseNote: string,
	 *   onconfirm: () => void,
	 *   oncancel: () => void,
	 * }} */
	let { slotDisplay, eraseNote, onconfirm, oncancel } = $props()
</script>

<Modal labelledBy="delete-confirm-title" onclose={oncancel}>
	<div class="modal-header">
		<h2 class="modal-title" id="delete-confirm-title">Clear {slotDisplay} in this file?</h2>
		<p class="modal-subtitle">
			The radio keeps the channel until you delete it from the front panel — sending marks it
			"erased (unsupported)".
		</p>
		{#if eraseNote}
			<p class="modal-subtitle">{eraseNote}</p>
		{/if}
	</div>
	<div class="modal-footer">
		<button type="button" class="modal-btn" onclick={oncancel}>Cancel</button>
		<button type="button" class="modal-btn modal-btn-danger" onclick={onconfirm}>Clear {slotDisplay}</button>
	</div>
</Modal>
