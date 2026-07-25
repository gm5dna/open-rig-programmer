<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
<script>
	// Action toolbar (task-16 brief, task-18 §1/§2): buttons wired to
	// bindings — Read Radio, Open/Save, Send to Radio, Import/Export —
	// plus the dirty-guard and send-flow/import-result dialogues those
	// actions open. ActionBar owns which dialogue (if any) is open; each
	// dialogue component is presentation + callbacks only.
	import { appState } from './state/app.svelte.js'
	import {
		readRadio,
		loadFile,
		saveFile,
		saveFileAs,
		prepareSend,
		importCSV,
		importCHIRP,
		exportCSV,
	} from './bridge/bindings.js'
	import ToolButton from './ToolButton.svelte'
	import DirtyConfirmDialog from './DirtyConfirmDialog.svelte'
	import SendFlowDialog from './SendFlowDialog.svelte'
	import ImportResultDialog from './ImportResultDialog.svelte'

	/** @typedef {import('../../wailsjs/go/models').main.SendPlanView} SendPlanView */
	/** @typedef {import('../../wailsjs/go/models').main.ImportResultView} ImportResultView */

	const hasCodeplug = $derived(appState.codeplug !== null)
	const transferBusy = $derived(appState.transfer.active)

	let savePending = $state(false)

	/** Pending dirty-guard action, or null. @type {{ action: 'open' | 'read' } | null} */
	let dirtyGuard = $state(null)
	let dirtyGuardSaving = $state(false)

	/** The active send review/transfer dialogue's plan, or null.
	 * @type {SendPlanView | null} */
	let sendPlan = $state(null)

	/** The active import result dialogue, or null.
	 * @type {{ format: 'CSV' | 'CHIRP', result: ImportResultView } | null} */
	let importResult = $state(null)

	// --- Read Radio / Open, with the dirty guard --------------------------

	async function doReadRadio() {
		try {
			await readRadio()
		} catch {
			// Alert strip already carries the message.
		}
	}

	async function doOpen() {
		try {
			await loadFile()
		} catch {
			// Alert strip already carries the message.
		}
	}

	async function handleReadRadio() {
		if (appState.dirty) {
			dirtyGuard = { action: 'read' }
			return
		}
		await doReadRadio()
	}

	async function handleOpen() {
		if (appState.dirty) {
			dirtyGuard = { action: 'open' }
			return
		}
		await doOpen()
	}

	function dirtyGuardCancel() {
		dirtyGuard = null
	}

	async function dirtyGuardDiscard() {
		const action = dirtyGuard?.action
		dirtyGuard = null
		if (action === 'open') await doOpen()
		else if (action === 'read') await doReadRadio()
	}

	async function dirtyGuardSaveFirst() {
		const action = dirtyGuard?.action
		dirtyGuardSaving = true
		try {
			const path = appState.codeplug?.WorkingPath
			if (path) {
				await saveFile(path)
			} else {
				const chosen = await saveFileAs()
				if (!chosen) return // user cancelled the save dialog — abandon the guarded action too
			}
		} catch {
			return // alert strip already carries the message — stay on the guard
		} finally {
			dirtyGuardSaving = false
		}
		dirtyGuard = null
		if (action === 'open') await doOpen()
		else if (action === 'read') await doReadRadio()
	}

	// --- Save / Save As -----------------------------------------------------

	// "Save" reuses the working path once one exists; falls back to the
	// save dialog (same as Save As) the first time, so it is never a dead
	// button just because nothing has been saved yet.
	async function handleSave() {
		savePending = true
		try {
			const path = appState.codeplug?.WorkingPath
			if (path) {
				await saveFile(path)
			} else {
				await saveFileAs()
			}
		} catch {
			// Alert strip already carries the message.
		} finally {
			savePending = false
		}
	}

	async function handleSaveAs() {
		savePending = true
		try {
			await saveFileAs()
		} catch {
			// Alert strip already carries the message.
		} finally {
			savePending = false
		}
	}

	// --- Send to Radio --------------------------------------------------

	async function handleSend() {
		try {
			const plan = await prepareSend()
			// task-25 brief (adjudicated remedy for the reported "i don't
			// seem to be able to send deletes to the radio" defect):
			// NothingToSend alone is ambiguous between "the working copy
			// genuinely matches the radio" and "every pending change is
			// Blocked" (in practice, a channel delete — the radio has no
			// CAT erase command). Only the FIRST case is the toast's "the
			// working copy already matches the radio" claim; the second is
			// NOT true parity, so it must open the review dialogue instead
			// — SendFlowDialog renders an honest informational state for
			// it (blocked reasons + the front-panel erase procedure, no
			// Confirm affordance).
			const blockedOnly = plan.NothingToSend && (plan.Diff?.Counts?.Blocked ?? 0) > 0
			if (plan.NothingToSend && !blockedOnly) {
				appState.pushAlert('Nothing to send — the working copy already matches the radio.', 'info')
				return
			}
			sendPlan = plan
		} catch {
			// Alert strip already carries the message.
		}
	}

	function closeSendDialog() {
		sendPlan = null
	}

	// --- Import / Export --------------------------------------------------

	async function handleImportCSV() {
		try {
			const result = await importCSV()
			if (result.Cancelled) return
			if (!result.Merged || result.LossEntries?.length) {
				importResult = { format: 'CSV', result }
			} else {
				appState.pushAlert(`Imported ${result.Path}`, 'info')
			}
		} catch {
			// Alert strip already carries the message.
		}
	}

	async function handleImportCHIRP() {
		try {
			const result = await importCHIRP()
			if (result.Cancelled) return
			if (!result.Merged || result.LossEntries?.length) {
				importResult = { format: 'CHIRP', result }
			} else {
				appState.pushAlert(`Imported ${result.Path}`, 'info')
			}
		} catch {
			// Alert strip already carries the message.
		}
	}

	function closeImportDialog() {
		importResult = null
	}

	async function handleExportCSV() {
		try {
			const path = await exportCSV()
			if (path) appState.pushAlert(`Exported to ${path}`, 'info')
		} catch {
			// Alert strip already carries the message.
		}
	}
</script>

<div class="action-bar" role="toolbar" aria-label="Actions">
	<div class="tool-group">
		<span class="tool-group-label">Radio</span>
		<ToolButton
			label="Read Radio"
			onclick={handleReadRadio}
			disabled={!appState.connected || transferBusy}
			tooltip={appState.connected ? '' : 'Connect to a radio first'}
		/>
		<ToolButton
			label="Send to Radio"
			onclick={handleSend}
			disabled={!appState.canSend}
			tooltip={appState.canSend ? '' : appState.sendBlockedReason}
		/>
	</div>

	<div class="tool-group">
		<span class="tool-group-label">File</span>
		<ToolButton label="Open…" onclick={handleOpen} disabled={transferBusy} />
		<ToolButton
			label="Save"
			onclick={handleSave}
			disabled={!hasCodeplug || savePending || transferBusy}
			tooltip={!hasCodeplug ? 'Open or read a codeplug first' : transferBusy ? 'A transfer is already running' : ''}
		/>
		<ToolButton
			label="Save As…"
			onclick={handleSaveAs}
			disabled={!hasCodeplug || savePending || transferBusy}
			tooltip={!hasCodeplug ? 'Open or read a codeplug first' : transferBusy ? 'A transfer is already running' : ''}
		/>
	</div>

	<div class="tool-group">
		<span class="tool-group-label">Data</span>
		<ToolButton
			label="Import CSV…"
			onclick={handleImportCSV}
			disabled={!hasCodeplug || transferBusy}
			tooltip={!hasCodeplug ? 'Open or read a codeplug first' : transferBusy ? 'A transfer is already running' : ''}
		/>
		<ToolButton
			label="Import CHIRP…"
			onclick={handleImportCHIRP}
			disabled={!hasCodeplug || transferBusy}
			tooltip={!hasCodeplug ? 'Open or read a codeplug first' : transferBusy ? 'A transfer is already running' : ''}
		/>
		<ToolButton
			label="Export CSV…"
			onclick={handleExportCSV}
			disabled={!hasCodeplug || transferBusy}
			tooltip={!hasCodeplug ? 'Open or read a codeplug first' : transferBusy ? 'A transfer is already running' : ''}
		/>
	</div>
</div>

{#if dirtyGuard}
	<DirtyConfirmDialog
		action={dirtyGuard.action}
		saving={dirtyGuardSaving}
		oncancel={dirtyGuardCancel}
		ondiscard={dirtyGuardDiscard}
		onsavefirst={dirtyGuardSaveFirst}
	/>
{/if}

{#if sendPlan}
	{#key sendPlan}
		<SendFlowDialog plan={sendPlan} onClose={closeSendDialog} onPrepareAgain={handleSend} />
	{/key}
{/if}

{#if importResult}
	<ImportResultDialog format={importResult.format} result={importResult.result} onclose={closeImportDialog} />
{/if}

<style>
	.action-bar {
		display: flex;
		align-items: center;
		gap: var(--space-5);
		padding: var(--space-2) var(--space-4);
		background: var(--colour-panel);
		border-bottom: 1px solid var(--colour-hairline);
		flex-wrap: wrap;
	}

	.tool-group {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}

	.tool-group-label {
		font-size: 10.5px;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: var(--colour-text-faint);
		margin-right: var(--space-1);
	}
</style>
