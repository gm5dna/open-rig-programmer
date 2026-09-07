<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
<script>
	// Dirty guard for Open and Read Radio while the working copy has
	// unsaved edits (task-18 brief §2: "dirty guard on Open and on Read
	// Radio while dirty (in-app confirm dialogue: discard / cancel;
	// 'save first' convenience button if cheap)"). ActionBar owns which
	// action (Open/Read Radio) is pending and what happens on each choice;
	// this is the one ConfirmDialog use with a third (extra) button.
	import ConfirmDialog from './ConfirmDialog.svelte'

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

<ConfirmDialog
	labelledBy="dirty-guard-title"
	title="Unsaved changes"
	body="You have unsaved changes. {actionSentence} will replace the working copy — those changes will be lost unless you save first."
	confirmLabel="Discard changes"
	onconfirm={ondiscard}
	{oncancel}
	extraLabel={saving ? 'Saving…' : 'Save first'}
	onextra={onsavefirst}
	extraDisabled={saving}
/>
