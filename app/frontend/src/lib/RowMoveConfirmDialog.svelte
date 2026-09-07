<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
<script>
	// Confirmation for the ↑/↓ row-action's EMPTY-neighbour case (task-24
	// brief): swapping with a populated neighbour is instant (both channels
	// survive, fully sendable — no data loss possible, so no confirmation
	// is needed), but moving onto an EMPTY neighbour empties the source in
	// the file — the same file-level "erase" as delete/drag-move (no CAT
	// erase exists: the radio keeps the old contents until front-panel
	// deletion). Same wording as MoveCopyDialog's own "Move here" caveat,
	// reused near-verbatim rather than invented afresh. ChannelGrid owns
	// what happens on confirm.
	import ConfirmDialog from './ConfirmDialog.svelte'

	/** @type {{
	 *   sourceDisplay: string,
	 *   targetDisplay: string,
	 *   onconfirm: () => void,
	 *   oncancel: () => void,
	 * }} */
	let { sourceDisplay, targetDisplay, onconfirm, oncancel } = $props()
</script>

<ConfirmDialog
	labelledBy="row-move-confirm-title"
	title="Move {sourceDisplay} to {targetDisplay}?"
	body="{targetDisplay} is empty — {sourceDisplay} becomes empty in this file. The radio keeps its old contents until front-panel deletion."
	confirmLabel="Move here"
	{onconfirm}
	{oncancel}
/>
