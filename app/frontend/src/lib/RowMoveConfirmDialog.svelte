<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
<script>
	// Confirmation for the ↑/↓ row-action's EMPTY-neighbour case (task-24
	// brief): swapping with a populated neighbour is instant (both channels
	// survive, fully sendable — no data loss possible, so no confirmation
	// is needed), but moving onto an EMPTY neighbour empties the source in
	// the file — the same file-level "erase" as delete/drag-move (no CAT
	// erase exists: the radio keeps the old contents until front-panel
	// deletion). Same wording as MoveCopyDialog's own "Move here" caveat,
	// reused near-verbatim rather than invented afresh. Pure presentation,
	// like DeleteConfirmDialog — ChannelGrid owns what happens on confirm.
	import Modal from './Modal.svelte'

	/** @type {{
	 *   sourceDisplay: string,
	 *   targetDisplay: string,
	 *   onconfirm: () => void,
	 *   oncancel: () => void,
	 * }} */
	let { sourceDisplay, targetDisplay, onconfirm, oncancel } = $props()
</script>

<Modal labelledBy="row-move-confirm-title" onclose={oncancel}>
	<div class="modal-header">
		<h2 class="modal-title" id="row-move-confirm-title">Move {sourceDisplay} to {targetDisplay}?</h2>
		<p class="modal-subtitle">
			{targetDisplay} is empty — {sourceDisplay} becomes empty in this file. The radio keeps its old
			contents until front-panel deletion.
		</p>
	</div>
	<div class="modal-footer">
		<button type="button" class="modal-btn" onclick={oncancel}>Cancel</button>
		<button type="button" class="modal-btn modal-btn-danger" onclick={onconfirm}>Move here</button>
	</div>
</Modal>
